---
phase: 05-operator-correctness
plan: 04
subsystem: database
tags: [migrate, testing, qdrant, convergence]

# Dependency graph
requires:
  - phase: 05-operator-correctness (plan 01-03, sequentially prior)
    provides: no direct dependency — this plan is self-contained (depends_on: [])
provides:
  - "TestMigrateBelowCursorInsertConverges: proves the migrate sweep's documented
    convergence contract (D-05) for a record inserted mid-sweep below the sweep's
    already-advanced in-pass cursor"
  - "midSweepHook.onScroll records the triggering scroll's Offset uuid via a new
    triggerCursorID() accessor, reusable by future mid-sweep tests"
affects: []

# Actuals (#2632)
actuals:
  tokens: 3930
  tasks: 1
  commits: 1

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "midSweepHook now records the triggering request's Offset uuid before
      firing its callback, so a test can assert an insert landed below the
      sweep's in-pass cursor, not merely 'after' or 'before' the seeded set"

key-files:
  created: []
  modified:
    - internal/store/migrate_converge_test.go

key-decisions:
  - "D-05 implemented exactly as scoped: test-only coverage of the below-cursor
    mid-sweep insert case; internal/store/migrate.go is unchanged (verified via
    git diff --quiet HEAD after the hand-check mutation was reverted)"
  - "midSweepHook.onScroll widened to accept the *qdrant.ScrollPoints request
    (additive change) so the recorded trigger cursor can be asserted against —
    TestMigrateConvergesWithoutLock is unaffected and still passes unchanged"
  - "#501's persisted-cursor RED-patch restore items (03-04-red-1-persisted-cursor.patch,
    its redEvidenceDirs entry) confirmed obsolete: the red-evidence harness was
    removed repo-wide in c1afd6c1 under rule 3p0zsqrhmb; internal/store/redevidence_harness_test.go
    no longer exists. Non-vacuity is shown instead by the in-test cursor assertion
    plus a hand-applied, never-committed mutation run (see below)"

requirements-completed: [OPS-04]

coverage:
  - id: D1
    description: "TestMigrateBelowCursorInsertConverges covers a mid-sweep insert whose id sorts below the migrate sweep's advanced in-pass cursor, asserting the documented convergence contract"
    requirement: "OPS-04"
    verification:
      - kind: integration
        ref: "internal/store/migrate_converge_test.go#TestMigrateBelowCursorInsertConverges"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_converge_test.go#TestMigrateConvergesWithoutLock (regression, still passes after the shared hook change)"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-09-24
status: complete
---

# Phase 5 Plan 4: Below-cursor migrate-sweep convergence coverage Summary

**Added `TestMigrateBelowCursorInsertConverges`, the only migrate-sweep case no existing test exercised: a record inserted mid-sweep whose id sorts below the sweep's already-advanced in-pass cursor, proving D-05's documented convergence contract via a live Qdrant-backed race.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-24T16:50Z (approx)
- **Completed:** 2026-09-24T16:56:27Z
- **Tasks:** 1 completed
- **Files modified:** 1 (`internal/store/migrate_converge_test.go`)

## Accomplishments

- `TestMigrateBelowCursorInsertConverges` (four subtests) covers the below-cursor mid-sweep insert case (#501, OPS-04): an ordinary `Store.Upsert` below the cursor needs no sweep work (D-05), while a raw-injected below-target laggard below the cursor is picked up and migrated by a later pass's fresh re-derivation.
- `midSweepHook.onScroll` now takes the `*qdrant.ScrollPoints` request and records the triggering scroll's Offset uuid (the in-pass cursor at the moment the mid-sweep write fired) via a new `triggerCursorID()` accessor — additive only; `TestMigrateConvergesWithoutLock` is unchanged and still passes.
- Non-vacuity was hand-verified per the plan's step (4): the default sweep loop in `internal/store/migrate.go` was temporarily mutated to `return res, nil` right after the first pass's `scrollAllPoints` call (modeling a sweep that never re-derives), the laggard and convergence subtests FAILED as expected while the cursor subtest still PASSED, and the mutation was reverted — confirmed clean via `git diff --quiet HEAD -- internal/store/migrate.go`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Below-cursor mid-sweep insert coverage (D-05, #501)** - `39ef398f` (test)

**Plan metadata:** committed alongside this SUMMARY.

_Note: this was not a TDD plan — a single production-quality test commit._

## Files Created/Modified

- `internal/store/migrate_converge_test.go` — added `TestMigrateBelowCursorInsertConverges` (four subtests) and widened `midSweepHook.onScroll`/`midSweepInterceptor` to record and expose the triggering scroll's cursor.

## Decisions Made

See `key-decisions` in frontmatter. No deviations from the plan's stated decisions (D-05, D-02).

## Deviations from Plan

None - plan executed exactly as written.

## Non-vacuity hand check (step 4) — FAIL lines quoted

With `internal/store/migrate.go`'s default sweep loop temporarily mutated to `return res, nil` immediately after the first pass's `scrollAllPoints` call (never committed):

```
--- FAIL: TestMigrateBelowCursorInsertConverges (0.64s)
    --- PASS: TestMigrateBelowCursorInsertConverges/the_insert_landed_below_the_advanced_cursor (0.00s)
    --- PASS: TestMigrateBelowCursorInsertConverges/already-current_below-cursor_write_needs_no_sweep_work_(D-05) (0.00s)
    --- FAIL: TestMigrateBelowCursorInsertConverges/below-target_below-cursor_record_is_migrated_by_a_later_pass (0.00s)
    --- FAIL: TestMigrateBelowCursorInsertConverges/the_sweep_converged (0.00s)
```

Specific assertion failures recorded:

```
migrate_converge_test.go:479: laggard id c5200000-0000-0000-0000-000000000002 absent from the sweep's recorded SetPayload write-id set, want present
migrate_converge_test.go:508: res.Backlog = 6, want 0 (converged)
migrate_converge_test.go:511: migrateBacklogIDs(target=1) = [c5200000-0000-0000-0000-000000000002], want empty
migrate_converge_test.go:534: recorded write-id set mismatch:
      missing (expected, not observed): [c5200000-0000-0000-0000-000000000002]
      extra (observed, not expected): []
```

This confirms the test is non-vacuous: it genuinely detects a sweep that stops re-deriving after the first pass. After reverting the mutation, `git diff --quiet HEAD -- internal/store/migrate.go` exited 0 (clean working tree against HEAD, confirmed both immediately after the revert and again post-commit).

## Verification Results

- `env ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -run '^TestMigrate(BelowCursorInsertConverges|ConvergesWithoutLock)$' -v` — exit 0, both tests PASS, four `TestMigrateBelowCursorInsertConverges` subtests all PASS.
- `env ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -run '^TestMigrate'` (plan-level `<verification>`) — exit 0, `ok`.
- `golangci-lint run ./internal/store/...` — 0 issues.
- `gofmt -l internal/store/migrate_converge_test.go` — no output (clean).
- `go test ./internal/keylinks/` (per orchestrator instruction) — `ok`.

## Acceptance Criteria

- `rg -o -F 'func TestMigrateBelowCursorInsertConverges(' internal/store/migrate_converge_test.go | wc -l` → `1` ✅
- `rg -o -F 'GetOffset().GetUuid()' internal/store/migrate_converge_test.go | wc -l` → `1` ✅
- `rg -o -F 'spineScrollBatch' internal/store/migrate_converge_test.go | wc -l` → `4` (≥ 3 required) ✅
- `git log -1 --format=%B | rg -o -e 'Closes #501|c1afd6c1' | sort -u | wc -l` → `2` ✅
- SUMMARY quotes the step (4) mutation FAIL lines and records the clean working-tree-vs-HEAD check — see above ✅

## Known Stubs

None.

## Threat Flags

None — this plan added test coverage only; no new production surface.

## Self-Check: PASSED

- `[ -f internal/store/migrate_converge_test.go ]` → FOUND
- `git log --oneline --all | grep -q "39ef398f"` → FOUND
