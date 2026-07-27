---
phase: 24-idempotent-capture
plan: 02
subsystem: api
tags: [go, qdrant, idempotency, mcp, concurrency]

# Dependency graph
requires:
  - phase: 24-idempotent-capture (Plan 01)
    provides: "idempotencyPointID(owner, scope, key), contentFingerprint(storeArgs), store.ErrIdempotencyConflict, Memory.IdempotencyFingerprint payload codec"
provides:
  - "storeArgs.IdempotencyKey optional arg, promoted to scheduleArgs via Go field embedding (D-13)"
  - "deps.checkIdempotentReplay — shared check-before-embed helper wired into storeMemory and scheduleMemory before Embed"
  - "Live keyed replay-safety on store_memory/schedule_memory: SC1-SC5 proven end-to-end against a real Qdrant"
affects: [25-supersession-links]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Check-before-embed keyed branch (mirrors storeDiscovery's resolve->Get->decide->embed shape) at the shared storeArgs seam, minus OwnedOrAbsent since owner is baked into the point-ID hash (D-09)"
    - "First `go test -race` invocation in this repo (SC4 concurrency test) — new CI-invocation surface"

key-files:
  created: []
  modified:
    - internal/server/tools.go
    - internal/server/tools_test.go

key-decisions:
  - "checkIdempotentReplay signature: (ctx, owner string, a storeArgs) (replay bool, id, shortID, pointID string, err error) — matches the plan's artifact table"
  - "scheduleMemory calls checkIdempotentReplay(ctx, owner, a.storeArgs) AFTER parseWindow (cheap pure validation stays first) but still BEFORE Embed"
  - "SC5 test uses testDeps(t) (which delegates to testDepsWithStore) rather than testDepsWithStore(t) directly — functionally identical self-skip behavior, matches the sibling TestStoreMemoryMintsAndReturnsShortID convention immediately above it"
  - "SC4 concurrency test resolves the caller once via callerFor(ctx, t) before spawning goroutines (never calls t.Fatalf from inside a goroutine) to avoid a race on *testing.T itself"

requirements-completed: [REQ-idempotent-capture]

coverage:
  - id: D1
    description: "storeArgs gains an optional idempotency_key arg, documented on both store_memory and schedule_memory tool descriptions (D-11); omitting it preserves today's fresh-uuid-every-time behavior byte-for-byte (SC5)"
    requirement: "REQ-idempotent-capture"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreMemoryNoKeyAlwaysFresh"
        status: pass
    human_judgment: false
  - id: D2
    description: "A keyed store_memory replay with identical content returns the ORIGINAL (id, short_id) with zero side-effects (no second Embed, no duplicate Qdrant point)"
    requirement: "REQ-idempotent-capture"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreMemoryIdempotentReplayReturnsOriginal"
        status: pass
    human_judgment: false
  - id: D3
    description: "Same key + different content is rejected with store.ErrIdempotencyConflict (errors.Is true), never a silent overwrite, and the original record is left unchanged"
    requirement: "REQ-idempotent-capture"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreMemoryIdempotentReplayRejectsMismatch"
        status: pass
    human_judgment: false
  - id: D4
    description: "The idempotency key is owner-scoped: two owners reusing the identical key value get two independent, cross-invisible records (Pitfall 2 matrix)"
    requirement: "REQ-idempotent-capture"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreMemoryIdempotentKeyScopedPerOwner"
        status: pass
    human_judgment: false
  - id: D5
    description: "N concurrent identical keyed store_memory calls resolve to exactly one Qdrant point (no-duplicate invariant), proven under go test -race"
    requirement: "REQ-idempotent-capture"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreMemoryIdempotentConcurrentIdenticalOnePoint (go test -race)"
        status: pass
    human_judgment: false
  - id: D6
    description: "schedule_memory replay with the same key + identical content but a CHANGED not_before/not_after window returns the original record with its ORIGINAL window unchanged (schedule window excluded from the D-07 fingerprint)"
    requirement: "REQ-idempotent-capture"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestScheduleMemoryIdempotentIgnoresWindowChange"
        status: pass
    human_judgment: false

duration: 9min
completed: 2026-07-18
status: complete
---

# Phase 24 Plan 02: Idempotent Capture — Handler Wiring Summary

**Wired Plan 01's pure primitives into `store_memory`/`schedule_memory` via a shared `checkIdempotentReplay` check-before-embed helper — keyed replay is now zero-side-effect on match, rejects with `store.ErrIdempotencyConflict` on mismatch, and converges concurrent identical retries to exactly one Qdrant point under `-race`.**

## Performance

- **Duration:** 9 min
- **Started:** 2026-07-18T19:44:00Z
- **Completed:** 2026-07-18T19:53:00Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments
- `storeArgs.IdempotencyKey` added once (Go field promotion carries it to `scheduleArgs`, D-13); both `store_memory` and `schedule_memory` tool descriptions now document the match/reject/omit contract (D-11)
- `deps.checkIdempotentReplay(ctx, owner, storeArgs)` — a shared check-before-embed helper mirroring `storeDiscovery`'s resolve→Get→decide→embed shape, minus the `OwnedOrAbsent` gate (owner is baked into the deterministic point-ID hash, D-09)
- Both `storeMemory` and `scheduleMemory` branch through the helper BEFORE `d.em.Embed`: replay match returns the original `(id, short_id)` immediately; absent falls through with the resolved `pointID` threaded into `m.ID` (never recomputed) and `m.IdempotencyFingerprint` stamped; mismatch rejects before embedding
- All five success criteria plus the schedule-window decision proven end-to-end against a real Qdrant (testcontainers): SC1 (zero-side-effect replay), SC2 (mismatch reject), SC3 (two-owner matrix), SC4 (20 concurrent identical calls → 1 point, under `go test -race` — first `-race` invocation in this repo), SC5 (keyless unchanged)
- Keyless path is byte-for-byte untouched: `toMemory` still mints a fresh `uuid.NewString()` every time

## Task Commits

Each task was committed atomically:

1. **Task 1: Add idempotency_key arg, document the contract, pin SC5** - `8f879f12` (feat)
2. **Task 2: checkIdempotentReplay + keyed branch (SC1, SC2)** - `303acf97` (test, RED) → `7384b891` (feat, GREEN)
3. **Task 3: SC3 two-owner matrix, SC4 -race concurrency, schedule-window decision** - `5bd29a59` (test)

**Plan metadata:** pending (this commit)

_Note: Task 2 is TDD — RED (`303acf97`, tests fail against the unimplemented keyed branch) then GREEN (`7384b891`)._

## Files Created/Modified
- `internal/server/tools.go` - `storeArgs.IdempotencyKey` field, `checkIdempotentReplay` helper, keyed branch in `storeMemory`/`scheduleMemory`, updated `store_memory`/`schedule_memory` tool descriptions
- `internal/server/tools_test.go` - SC1–SC5 tests, cross-owner matrix, `-race` concurrency test, schedule-window decision test

## Decisions Made
- `checkIdempotentReplay` returns `(replay bool, id, shortID, pointID string, err error)` — the exact tuple shape from the plan's artifact table, so the deterministic pointID is threaded into `m.ID` on the create path rather than recomputed independently (RESEARCH Pattern 2 anti-pattern avoided).
- `scheduleMemory` runs `parseWindow` (pure argument validation) before `checkIdempotentReplay`, but the idempotency check still runs strictly before `Embed` — satisfying the plan's ordering requirement without reordering unrelated validation.
- SC5's `TestStoreMemoryNoKeyAlwaysFresh` uses `testDeps(t)` (which itself delegates to `testDepsWithStore(t)` and discards the concrete store) rather than calling `testDepsWithStore(t)` directly, since the test doesn't need the raw `*store.Store` — this matches the immediately-adjacent `TestStoreMemoryMintsAndReturnsShortID` convention and preserves the same Qdrant self-skip behavior the plan asked for.
- SC4's concurrency test resolves the `caller` once via `callerFor(ctx, t)` before fanning out goroutines (rather than calling it inside each goroutine as the summarizer-hang precedent does for its single goroutine) to avoid concurrent `t.Fatalf` calls racing on `*testing.T`.

## Deviations from Plan

None - plan executed exactly as written. All three tasks, all five success criteria, and the schedule-window decision test landed as specified; no Rule 1-4 auto-fixes were needed.

## Issues Encountered

None. Docker/testcontainers was available locally, so every integration-tier SC test (SC1–SC5, the two-owner matrix, and the `-race` concurrency test) ran for real against an ephemeral Qdrant `v1.18.2` container rather than self-skipping.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `go build ./...`, `go vet ./internal/server/...`, `golangci-lint run ./internal/server/... ./internal/store/...`, `gofmt -l` (clean), and `task license:check` are all green.
- `go test ./...` is green; `go test -race ./internal/server/...` (full package, not just the new tests) is green — the SC4 `-race` test is a durable addition to the package's normal test run, not a special CI-only invocation.
- `task lint:yaml` (`yamlfmt -lint .`) fails, but this is pre-existing environment drift on files this plan never touched (`.github/workflows/ci.yaml`, `Taskfile.yaml`) — out of scope per the deviation-rules scope boundary (only auto-fix issues directly caused by this plan's changes). Not fixed here; flagged for a separate cleanup if it recurs in CI.
- Phase 25 (supersession-links) reuses this plan's re-`Upsert`-at-deterministic-ID mechanism (owner/scope-aware point resolution, payload-only server-set stamp discipline) as its foundation.
- No blockers.

---
*Phase: 24-idempotent-capture*
*Completed: 2026-07-18*

## Self-Check: PASSED

All created/modified files found on disk; all commit hashes (8f879f12, 303acf97, 7384b891, 5bd29a59) found in git log.
