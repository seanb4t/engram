---
phase: 12-per-memory-usage-signals
plan: 01
subsystem: database
tags: [go, qdrant, store, telemetry, ranking]

# Dependency graph
requires:
  - phase: 09-retrieval-eval-harness-ranking-precision
    provides: "RerankHits / SearchReranked (rerank.go) — the pure ranking function this plan's negative-space test guards against usage-signal contamination"
  - phase: 11-async-on-write-summaries
    provides: "SetVisibility/SetPayload precedent and the store-method telemetry-span idiom IncrementAccess mirrors"
provides:
  - "store.Memory.AccessCount (uint64) and store.Memory.LastAccessedAt (time.Time) fields"
  - "payload()/fromPayload() round-trip for access_count/last_accessed_at (lossless via Qdrant Value_IntegerValue; legacy records read zero)"
  - "store.IncrementAccess(ctx, id) — vector-preserving SetPayload primitive, no re-authorization"
  - "Free update-path bump: store.Update increments AccessCount/LastAccessedAt on the already-fetched record before Upsert"
  - "D-08 negative-space test proving RerankHits output is invariant under AccessCount"
affects: [12-02, 12-03, 12-04, 12-05, 12-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Server-set-only field: AccessCount/LastAccessedAt have no client-writable tool argument, mirroring Actor/Owner"
    - "Vector-preserving partial-payload SetPayload write (IncrementAccess mirrors SetVisibility, minus getWritable)"
    - "Free piggyback bump: mutate the already-in-memory record before an existing Upsert instead of a second store call"

key-files:
  created:
    - internal/store/usage_test.go
  modified:
    - internal/store/store.go
    - internal/store/rerank_test.go

key-decisions:
  - "IncrementAccess deliberately skips getWritable/GetReadable — the handler-boundary caller (Plan 06) already gated ownership; a second internal Get here is a read-modify-write of the counter, not re-authorization (D-01/D-04)."
  - "No payload index added for access_count — it is never filtered/sorted/faceted this phase, keeping the D-08 boundary honest (no raw-Qdrant-filter bypass path for a future usage-weighted recall)."
  - "usage_test.go tests are split across two commits (Task 1 fields/round-trip/update-bump; Task 2 IncrementAccess) matching the plan's task boundaries, even though authored together."

patterns-established:
  - "Negative-space ranking-invariance test: construct two input sets identical except the guarded field, with non-monotonic (neither ascending nor descending in tiebreak-key order) values so the test catches a tiebreak violation in either sort direction."

requirements-completed: [REQ-usage-signals]

coverage:
  - id: D1
    description: "Memory.AccessCount/LastAccessedAt round-trip losslessly through payload()/fromPayload() via Qdrant Value_IntegerValue; legacy records missing the keys read zero with no backfill."
    requirement: "REQ-usage-signals"
    verification:
      - kind: integration
        ref: "internal/store/usage_test.go#TestUsageAccessCountRoundtrip"
        status: pass
      - kind: integration
        ref: "internal/store/usage_test.go#TestUsageLegacyRecordReadsZero"
        status: pass
    human_judgment: false
  - id: D2
    description: "store.IncrementAccess writes access_count+1/last_accessed_at via a vector-preserving SetPayload, without re-running the ownership gate."
    requirement: "REQ-usage-signals"
    verification:
      - kind: integration
        ref: "internal/store/usage_test.go#TestIncrementAccess"
        status: pass
    human_judgment: false
  - id: D3
    description: "store.Update bumps AccessCount/LastAccessedAt on the already-fetched record before Upsert — zero extra store round-trip."
    requirement: "REQ-usage-signals"
    verification:
      - kind: integration
        ref: "internal/store/usage_test.go#TestUpdateBumpsAccessCountByOne"
        status: pass
    human_judgment: false
  - id: D4
    description: "RerankHits/SearchReranked output is invariant under access_count — a genuine negative-space guard, not a tautology (D-08)."
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "internal/store/rerank_test.go#TestRerankHitsIgnoresAccessCount"
        status: pass
    human_judgment: false

# Metrics
duration: 8min
completed: 2026-07-10
status: complete
---

# Phase 12 Plan 01: Store-Layer Usage-Signal Foundation Summary

**store.Memory grew AccessCount/LastAccessedAt with a lossless Qdrant payload round-trip, a new IncrementAccess SetPayload primitive for the async get-path, a free update-path bump, and a verified D-08 ranking-invariance guard.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-07-10T16:50:50Z
- **Completed:** 2026-07-10T16:57:15Z
- **Tasks:** 3
- **Files modified:** 3 (1 created: `usage_test.go`; 2 modified: `store.go`, `rerank_test.go`)

## Accomplishments
- Added `Memory.AccessCount uint64` / `Memory.LastAccessedAt time.Time` and wired both through `payload()` (unconditional int write / conditional RFC3339 write, mirroring the `not_before`/`not_after` idiom) and `fromPayload()` (guarded `GetIntegerValue()`/`time.Parse` reads) — verified lossless via a raw-payload assertion that `access_count` round-trips as a Qdrant `IntegerValue`, not a `DoubleValue`.
- `store.Update` now bumps `cur.AccessCount++` / `cur.LastAccessedAt = s.now()` on the already-fetched record immediately before its existing `Upsert` — zero extra store round-trip (D-04).
- Added `store.IncrementAccess(ctx, id)`, the `SetVisibility`-shaped partial-payload primitive for the async get-path: reads the current counter via `s.Get`, writes `access_count+1`/`last_accessed_at` via a vector-preserving `SetPayload`, and deliberately never calls `getWritable`/`GetReadable` (grep-verified: zero occurrences in the function body) — the handler-boundary caller (Plan 06) already gated ownership.
- Added `TestRerankHitsIgnoresAccessCount`, a genuine negative-space guard for the D-08 hard invariant: two hit sets identical in every rank-relevant field except `AccessCount` (assigned non-monotonic values so neither an ascending nor descending accidental tiebreak would go undetected). Verified the test actually fails by temporarily wiring an `AccessCount` tiebreak into a scratch copy of `RerankHits`, confirming a real detection, then reverting `rerank.go` to its original (untouched) state.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Memory fields + payload round-trip + free update bump** - `9f9693d` (feat)
2. **Task 2: Add store.IncrementAccess partial-payload primitive** - `4a9c2a4` (feat)
3. **Task 3: D-08 negative-space reranker invariance test** - `1663bbe` (test)

_No TDD tasks in this plan; each commit is a single feat/test commit._

## Files Created/Modified
- `internal/store/store.go` - `Memory.AccessCount`/`LastAccessedAt` fields, `payload()`/`fromPayload()` wiring, `Update`'s free bump, new `IncrementAccess` method
- `internal/store/usage_test.go` (new) - round-trip, legacy-record-zero, update-bump, and `IncrementAccess` integration tests (Qdrant-backed via `testStore(t)`)
- `internal/store/rerank_test.go` - `TestRerankHitsIgnoresAccessCount` negative-space invariance test

## Decisions Made
- `IncrementAccess` intentionally skips ownership re-authorization (`getWritable`/`GetReadable`) — confirmed via `12-RESEARCH.md` Pattern 1 and the plan's D-01/D-04 boundary; the internal `Get` call is purely RMW plumbing for the counter value.
- No payload index registered for `access_count` (verified `ensureIndexes` unchanged) — keeps the field unfilterable/unsortable at the raw Qdrant layer, closing off a future workaround that could bypass the reranker's D-08 boundary.

## Deviations from Plan

None - plan executed exactly as written. The three tasks map 1:1 to the three commits; no Rule 1-4 auto-fixes were needed.

## Issues Encountered

Pre-existing markdown-lint failures (331 issues across 37 `.planning/` files, none touched by this plan) cause `task` (the combined lint+test gate) to fail at the `lint:markdown` step. `task lint:go` (0 issues) and `task test` (all Go + Python suites green, including the full `internal/store` package) both pass independently. Logged to `.planning/phases/12-per-memory-usage-signals/deferred-items.md` per the scope-boundary rule — not fixed here since it's unrelated to this plan's `internal/store/` changes.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

The store-layer foundation (`Memory` fields, payload round-trip, `IncrementAccess`, free update bump, D-08 guard) is in place for the remaining Wave 1+ plans in this phase: the async get-path incrementer (queue wiring), MCP/Connect handler-boundary counting calls, `recallView`/proto exposure, and the `ENGRAM_USAGE_SIGNALS` config gate. No blockers.

---
*Phase: 12-per-memory-usage-signals*
*Completed: 2026-07-10*
