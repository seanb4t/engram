---
phase: 20-correctness-polish
plan: 03
subsystem: database
tags: [go, qdrant, error-handling, store]

requires:
  - phase: 20-correctness-polish
    provides: "Phase 20 context/research/patterns for the six-issue correctness batch"
provides:
  - "Bounded MintShortID collision-retry loop (16 real Qdrant Count() checks)"
  - "ErrShortIDExhausted sentinel, errors.Is-checkable, follows ErrAmbiguousShortID idiom"
affects: [store, server-tools, server-rules]

tech-stack:
  added: []
  patterns:
    - "Sentinel error via package-level errors.New + fmt.Errorf(%w) wrap, reused from ErrAmbiguousShortID"
    - "Bounded-attempt loop where in-batch dedup (seen-map) hits do not consume the real-check budget"

key-files:
  created: []
  modified:
    - internal/store/store.go
    - internal/store/store_test.go

key-decisions:
  - "Cap set to 16 real Qdrant Count() checks per D-04 (extra headroom over the ~8 already astronomically safe in 32^10 Crockford base32 space)"
  - "seen-map dup hits decrement the loop counter (attempts--) so they never consume the real-collision-check budget, per D-05"
  - "Exhaustion rides the existing telemetry deferred wrapper (RecordStoreOp + span.RecordError) with no new metric, per D-06"

patterns-established:
  - "Bounded retry-until-unique loops in this codebase count only real backend checks against the cap, not in-memory dedup skips"

requirements-completed:
  - REQ-shortid-mint-cap

coverage:
  - id: D1
    description: "MintShortID returns errors.Is-checkable ErrShortIDExhausted after exactly 16 real Qdrant Count() collision checks instead of looping forever"
    requirement: "REQ-shortid-mint-cap"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestMintShortIDExhaustsAfterCap"
        status: pass
    human_judgment: false
  - id: D2
    description: "seen-map dedup hits (in-batch dedup for batch-mint callers like BackfillShortIDs) do not consume the 16-attempt real-check budget"
    requirement: "REQ-shortid-mint-cap"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestMintShortIDSeenMapDoesNotConsumeBudget"
        status: pass
    human_judgment: false
  - id: D3
    description: "Existing single-collision-then-fresh-candidate behavior (TestMintShortIDRetriesOnCollision) is unchanged by the bounded rewrite"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestMintShortIDRetriesOnCollision"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-07-16
status: complete
---

# Phase 20 Plan 03: Store/ShortID Summary

**MintShortID now gives up with an errors.Is-checkable ErrShortIDExhausted after 16 real Qdrant collision checks instead of retrying forever, and seen-map dedup hits are free (don't count against the cap).**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-07-16T00:00:55Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Bounded `MintShortID`'s previously-unbounded `for {}` retry loop at `maxMintAttempts = 16` real (Qdrant `Count()`-checked) collision attempts (D-04)
- Added `ErrShortIDExhausted` sentinel beside the existing `ErrAmbiguousShortID` (store.go:56-56ish), wrapped with `%w` and `errors.Is`-checkable
- Preserved the D-05 invariant: `seen`-map in-batch dedup hits (`continue` branch) decrement the loop counter so they never consume the real-check budget — batch callers like `BackfillShortIDs` are unaffected by the cap under normal load
- Exhaustion rides the existing telemetry deferred wrapper (`RecordStoreOp` + `span.RecordError`) untouched — no new metric (D-06)
- Added `TestMintShortIDExhaustsAfterCap` and `TestMintShortIDSeenMapDoesNotConsumeBudget`, mirroring the existing `TestMintShortIDRetriesOnCollision`

## Task Commits

1. **Task 1: Bound MintShortID at 16 real collision checks and add ErrShortIDExhausted (#308)** - `8b75d7b0` (fix)

**Plan metadata:** (this commit, pending)

## Files Created/Modified
- `internal/store/store.go` - Added `ErrShortIDExhausted` sentinel + `maxMintAttempts = 16` const; rewrote `MintShortID`'s unbounded loop as a bounded `for attempts := 0; attempts < maxMintAttempts; attempts++` loop with `attempts--` on seen-map dup hits
- `internal/store/store_test.go` - Added `TestMintShortIDExhaustsAfterCap` (forces every candidate to collide, asserts `errors.Is(err, ErrShortIDExhausted)` and exactly 16 real `Count()` calls) and `TestMintShortIDSeenMapDoesNotConsumeBudget` (pre-populates `seen` with 3 dup candidates, asserts exhaustion still happens at exactly 16 real checks)

## Decisions Made
- Followed the plan's exact `attempts--` idiom (rather than a separate `checked` counter) for the D-05 seen-map exemption — matches 20-RESEARCH.md's/20-PATTERNS.md's documented code shape verbatim.
- No production code change needed beyond `store.go` — callers (`internal/server/tools.go:657/693/758`, `internal/server/rules.go:134`) already propagate `MintShortID`'s error unchanged; verified via `rg`, no diff required there.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None. `go build ./...`, `go vet ./internal/store/...`, `golangci-lint run ./internal/store/...`, and `go test ./internal/store/...` (full package, including live-Qdrant testcontainer tests) all pass clean. Ran verification directly (`go test`/`golangci-lint`) per the plan's note that the repo's default `task` target is blocked by a pre-existing `.rumdl.toml` gap deferred to Phase 21 — not a code failure in this plan's scope.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- #308 closed: `MintShortID` is now bounded and its exhaustion path is fully tested.
- No blockers for the remaining Phase 20 plans (proto/discovery, embed cleanups, Helm CronJob) — this plan is store/shortid-scoped and file-disjoint from the others.

---
*Phase: 20-correctness-polish*
*Completed: 2026-07-16*
