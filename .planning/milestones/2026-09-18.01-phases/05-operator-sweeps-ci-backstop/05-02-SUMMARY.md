---
phase: 05-operator-sweeps-ci-backstop
plan: 02
subsystem: database
tags: [qdrant, bounded-reads, revert, byte-budget]

# Dependency graph
requires:
  - phase: 05-operator-sweeps-ci-backstop
    provides: "scrollAllPoints(ctx, collection, filter, view, fn); fullView/summaryView/scanView/citationsView/nearDuplicateIdentityView/keysView; four spine sweeps already migrated (plan 05-01)"
provides:
  - "previewRevertWithSteps (engram migrate revert's preflight) migrated onto schemaVersionOnlyView/schemaVersionOnlyRecordCeiling — the narrowest projection in the phase (one field, schema_version)"
  - "internal/store/revertpreview_oversized_test.go: a real-Qdrant regression proving the revert preflight completes over an oversized above-target range on both fixture shapes"
  - "unbudgetedView (the count-only readView constructor) deleted from internal/store entirely — production AND test callers, in one commit"
affects: [05-03, 05-04, 05-05, 05-06]

# Actuals (#2632)
actuals:
  tokens: 3470
  tasks: 2
  commits: 2
plan_head_before: 93931844e2b880b95fef27cec4cd9cc2f9920743

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "schemaVersionOnlyView (D-04): a one-field readView for a preflight that reads exactly one payload key, keyed on the shared schemaVersionKey constant so selector and reader can never drift apart"
    - "A zero-value readView{} is still representable and still means 'no byte-derived ceiling' after unbudgetedView's deletion — only the convenience constructor is gone, not the concept sweepLimit/budgeted() branch on"

key-files:
  created:
    - internal/store/revertpreview_oversized_test.go
  modified:
    - internal/store/boundedread.go
    - internal/store/revert.go
    - internal/store/spine.go
    - internal/store/export_test.go
    - internal/store/boundedread_test.go
    - internal/store/spine_test.go
    - internal/store/orderedpage_oversized_test.go

key-decisions:
  - "The revert preflight's refusal against the production migrate.Registry (single v0->v1 Irreversible step) is an Irreversible-chain refusal, never an Unsupported-version one: the reverse chain from the seeded records' schema version IS reachable (StepsFrom finds it), it simply declines to run backward. The test asserts plan.Irreversible[0].To equals the seeded schema version and plan.Unsupported is empty, rather than the plan text's looser 'reports ... as the unsupported one' phrasing."
  - "The orderedpage_oversized_test.go 'unbudgeted view' table row was KEPT (retargeted to store.ReadView{}), not deleted: scrollOrderedPage's own argument validation independently rejects any view with no byte-derived ceiling, a standing production invariant unrelated to whether unbudgetedView the constructor exists."

requirements-completed: []  # Plan 05-06 owns ticking REQ-bounded-read-mechanism/REQ-sweeps-bounded (shared-ID gate, engram_project_gates)

coverage:
  - id: D1
    description: "previewRevertWithSteps (engram migrate revert's preflight) migrated onto schemaVersionOnlyView, proven over an above-target range whose 256-record page would have overflowed the named receive limit on both fixture shapes, reaching the identical refusal verdict as before"
    requirement: "REQ-sweeps-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestRevertPreviewBoundedOverGRPCLimit/few-large"
        status: pass
      - kind: integration
        ref: "internal/store#TestRevertPreviewBoundedOverGRPCLimit/many-small"
        status: pass
      - kind: integration
        ref: "internal/store (in-place revert characterization suite: TestMigrateRevertStepsFromArgOrder, TestRevertRefusalErrorSingleEnvelope, TestMigrateRevertIrreversibleRangeRefusesWhole, TestMigrateRevertFixtureInjectionConverges, TestMigrateRevertPerRecordChainSelection, TestMigrateRevertMultiPageUnsupportedPreflight, TestMigrateRevertPartialFailureReconciliation, TestMigrateRevertMidLoopRefusalIsTypedAndCatchable, TestMigrateRevertMidLoopUnsupportedRefusalIsTypedAndCatchable)"
        status: pass
    human_judgment: false
  - id: D2
    description: "unbudgetedView (the count-only readView constructor) and its exported test shim UnbudgetedView deleted from internal/store, with all five former callers (previewRevertWithSteps, and the four test-file callers the inventory's production-scoped closing check cannot see) retargeted in the same two commits — no commit in this phase leaves the package unbuildable"
    requirement: "REQ-bounded-read-mechanism"
    verification:
      - kind: unit
        ref: "whole-package grep: rg -o -e 'nbudgetedView' internal/store | wc -l == 0"
        status: pass
      - kind: unit
        ref: "inventory closing check (b): rg -o 's[.]scrollAllPoints[(].*unbudgetedView[(]' internal/store --glob '!*_test.go' | wc -l == 0"
        status: pass
      - kind: integration
        ref: "internal/store full suite (go test ./internal/store/ -count=1, ENGRAM_REQUIRE_QDRANT=1): ok, 869.847s, zero failures including TestRedEvidencePatchesAreLive's 42 registered phase 1-4 patches"
        status: pass
    human_judgment: false

# Metrics
duration: 62min
completed: 2026-09-20
status: complete
---

# Phase 5 Plan 2: Revert Preflight Bounded, unbudgetedView Deleted Entirely

**Moved `engram migrate revert`'s preflight onto a new one-field `schemaVersionOnlyView` projection and deleted the count-only `unbudgetedView` constructor from `internal/store` — production and all four test-file callers in one commit — closing D-01's inventory checklist for this phase.**

## Performance

- **Duration:** 62 min
- **Started:** 2026-09-20T15:35:00Z (approx)
- **Completed:** 2026-09-20T16:37:00Z (approx)
- **Tasks:** 2
- **Files modified:** 8 (1 created, 7 modified)

## Accomplishments

- Added `schemaVersionOnlyView()`/`schemaVersionOnlyRecordCeiling` (D-04) — the narrowest projection in the whole phase: one small integer field (`schema_version`), keyed on the shared `schemaVersionKey` constant so the selector can never drift from the field `versionOf` actually reads.
- Swapped `previewRevertWithSteps`' view argument from `unbudgetedView(qdrant.NewWithPayload(true))` to `schemaVersionOnlyView()` — filter, callback body, memoisation and error handling stayed byte-identical.
- Added `internal/store/revertpreview_oversized_test.go`: a real-Qdrant regression (`TestRevertPreviewBoundedOverGRPCLimit`, both fixture shapes) proving the preflight completes over an above-target range whose 256-record page would have overflowed `storetest.RecvLimit`, reaching the identical whole-range refusal verdict against the production `migrate.Registry`.
- Deleted `unbudgetedView` (the count-only view constructor) and its exported `export_test.go` shim `UnbudgetedView` entirely, in one commit that also retargeted the three test-file callers the inventory's production-scoped closing check (b) cannot see: `TestSweepLimit` (now constructs `readView{}` directly), `snapshotCollection` (now uses `s.fullView()`), and `scrollOrderedPage`'s argument-validation table row (now uses `store.ReadView{}`).
- Every non-test `scrollAllPoints` caller in `internal/store` now passes a budgeted view; the count-only escape hatch is gone.

## Task Commits

Each task was committed atomically:

1. **Task 1: The revert preflight bounded on a one-field projection, proven over an oversized above-target range (tracer)** — `af0aee9e` (refactor)
2. **Task 2 (owns the deletion trap): The zero-ceiling view constructor removed, with all four test-file callers retargeted in the same change** — `e9afaee6` (refactor!)

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/store/boundedread.go` — added `schemaVersionOnlyRecordCeiling`/`schemaVersionOnlyView()`; deleted `unbudgetedView`.
- `internal/store/revert.go` — `previewRevertWithSteps`' view argument changed to `schemaVersionOnlyView()`.
- `internal/store/revertpreview_oversized_test.go` (new) — the D-03 real-Qdrant regression for the revert preflight.
- `internal/store/spine.go` — `scrollAllPoints`' doc comment rewritten to describe an unbudgeted view by its `maxRecordBytes == 0` property rather than by the deleted constructor's name.
- `internal/store/export_test.go` — deleted the `UnbudgetedView` shim.
- `internal/store/boundedread_test.go` — `TestSweepLimit`'s first assertion retargeted to `sweepLimit(readView{})`.
- `internal/store/spine_test.go` — `snapshotCollection`'s view retargeted to `s.fullView()`.
- `internal/store/orderedpage_oversized_test.go` — the `"unbudgeted view"` table row retargeted to `store.ReadView{}`, renamed `"no byte ceiling"`.

## Decisions Made

- **Irreversible, not Unsupported, is the refusal path this test proves.** Every seeded fixture record carries `migrate.CurrentVersion` (1), and the production `migrate.Registry` has exactly one step, `0->1`, declared `Irreversible`. Reverting to 0 finds a REACHABLE reverse chain (`StepsFrom` succeeds) that is simply declared irreversible — so `plan.Irreversible` gets one entry (`From=0, To=1`) and `plan.Unsupported` stays empty. This exactly mirrors the existing in-place characterization test `TestMigrateRevertIrreversibleRangeRefusesWhole`. The plan's own `<behavior>` prose ("reports the seeded records' schema version as the unsupported one") is read here as informal language for "the seeded schema version surfaces in the refusal plan" (via `Irreversible[0].To`), not a literal claim that `RevertPlan.Unsupported` is populated — the test asserts the actual, correct mechanism and documents this explicitly (deviation-adjacent clarification, not a fix, since the underlying behavior — Store.Revert reaching the identical whole-range refusal it always reached — is unchanged from before this plan).
- **The `orderedpage_oversized_test.go` "unbudgeted view" row survives, retargeted rather than deleted.** Reading `scrollOrderedPage`'s own validation (`if !view.budgeted() { return ..., ErrInvalidArgument }`) confirms this rejection is a standing, permanent production invariant — completely independent of whether a constructor named `unbudgetedView`/`UnbudgetedView` exists. `store.ReadView{}` (the zero value) is directly constructible from `package store_test` without any new export shim, since Go permits an empty composite literal of a type with unexported fields from outside its package. Row renamed `"no byte ceiling"` to describe what it now literally constructs.
- **`schemaVersionOnlyRecordCeiling` set to 64 bytes** — smaller than `keysRecordCeiling` (256), matching the plan's requirement that a single small integer is cheaper to encode than an RFC3339 string; both are documented fixed allowances (not derived from `RecordCaps`), correctable at runtime by `scrollAllPoints`' own batch-of-1 fallback (D-07) if ever undersized.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] My own retargeting comments briefly reintroduced the string "unbudgetedView"/"UnbudgetedView", tripping Task 2's own whole-package-grep verify command**

- **Found during:** Task 2, running the plan's first automated `<verify>` (`rg -o -e 'nbudgetedView' internal/store | wc -l` must print `0`)
- **Issue:** Two of my own explanatory comments (in `spine_test.go`'s `snapshotCollection` and `orderedpage_oversized_test.go`'s retargeted table row) named the deleted constructor by identifier for clarity, which the verify command's substring grep — deliberately scoped to catch exactly this class of leftover reference, including in doc comments — correctly flagged.
- **Fix:** Reworded both comments to describe the removed constructor by its role ("the removed count-only view constructor", "Phase 5's count-only view constructor") instead of its literal identifier.
- **Files modified:** `internal/store/spine_test.go`, `internal/store/orderedpage_oversized_test.go`
- **Verification:** `rg -o -e 'nbudgetedView' internal/store | wc -l` prints `0` after the reword; `go build ./...` and `go vet ./internal/store/...` both clean.
- **Committed in:** `e9afaee6` (Task 2 commit — caught and fixed before committing, never landed in a bad state)

---

**Total deviations:** 1 auto-fixed (1 bug, caught by the plan's own verify gate before commit)
**Impact on plan:** No scope creep; the fix is purely wording in my own newly-authored comments, never touching production behavior.

## Issues Encountered

- **False-alarm red-evidence failures traced to a dirty working tree, not a regression.** Running the plan's full-suite verify command (`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -v`) WHILE Task 2's edits were still uncommitted produced four `--- FAIL:` lines under `TestRedEvidencePatchesAreLive`, all against Phase 3 patches touching `boundedread.go`/`spine.go` (`03-02-ceiling-drops-citations.patch`, `03-02-sweep-count-not-byte-derived.patch`, `03-02-sweep-fallback-removed.patch`, `03-02-sweep-swallows-single-overflow.patch`). Investigation (reading the harness's own dirty-tree guard, `redevidence_harness_test.go:317`) confirmed each failure's actual message was `refusing to apply <patch>: files it touches are already dirty` — the harness's own tree-safety check correctly refusing to run against uncommitted source, not evidence of a broken patch. After committing Task 2, all four patches individually re-verified `confirmed RED` against their mapped target tests on the clean tree, and a full extended-timeout run (`go test -timeout 25m`, 869.847s) confirmed the ENTIRE `internal/store` suite — all 42 registered phase 1-4 red-evidence patches plus every other test — passes with **zero** failures.
- **The plan's literal full-suite `<verify>` command (expecting exactly one `--- FAIL:` line, matching `TestRedEvidencePatchesAreLive`) does not match today's actual, correct repo state.** `redEvidenceDirs` has no entry for phase 5 yet (plan 05-06's job), and its empty-map guard only fires when the WHOLE map is empty — which it is not, since phases 1-4 are already registered. There is therefore no structural reason for `TestRedEvidencePatchesAreLive` to be red at this point in the phase, and the extended-timeout run above measured it fully green. Read the `engram_project_gates` framing ("the single expected RED... until plan 05-06") as describing plan 05-06's own future-tense target state, not this plan's actual current one. Documented here rather than silently forcing a mismatch; no code change follows from this, since the actual, better (fully green) outcome supersedes the plan's assumption without requiring one.
- **Ambient system load (`uptime` reported load averages ~102-114) made the plan's own default-timeout full-suite verify command (`go test ./internal/store/ -count=1`, no explicit `-timeout`) exceed `go test`'s 10-minute default and panic mid-run on a clean tree**, roughly 5x slower per-patch than the historical baseline (04-08-SUMMARY.md: 191.96s for 42 patches; this environment: 9m41s for just the first 25). This is an environment/machine-load condition, not a code regression — re-run with an explicit `-timeout 25m` completed cleanly at 869.847s with zero failures. No Docker container leak was found (`docker ps -a` showed only 2 unrelated containers); the slowdown is attributed to concurrent load on the shared machine at test time. This is exactly the class of finding the phase's own `T-05-06-08`/`T-04-08-05`/`T-03-06-03` threat entries anticipate (the pre-authorized "runtime contingency" belongs to plan 05-06, not this plan) — no fix applied here, since the underlying test result (fully green) is unaffected once given adequate wall-clock time.
- **The plan's own acceptance criterion "`rg -o -e 'scrollAllPoints[(]ctx, s[.]collection,' internal/store | wc -l` prints `6`, unchanged from plan 05-01" is stale.** The actual, historical count both before and after this plan is **7**, not 6 — confirmed by checking out `internal/store/{revert,spine,export_test,spine_test}.go` at the commit immediately preceding this plan (`93931844`) and re-running the same grep, which also printed 7. Plan 05-01's own SUMMARY documents this exact discrepancy as its own Rule 3 deviation (`export_test.go`'s `ScrollAllPoints` shim, "This seventh call site was not named in the plan's `<interfaces>` section"). The invariant this criterion actually protects — the call-site count is unchanged by this plan's edits — holds (7 before, 7 after); only the plan's hard-coded expected number (6) is stale, inherited from the same undercount plan 05-01 already flagged. No code change follows.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `previewRevertWithSteps` (`revert.go`) is now the LAST of the phase's five `unbudgetedView` callers to migrate, and the constructor is now fully deleted from `internal/store` — production and test. The inventory's D-01 checklist for this phase's `scrollAllPoints`/`unbudgetedView` callers is closed.
- Plans 05-03/05-04/05-05 (the four own-loop sweeps: `Store.Migrate`'s three loops, `Store.SummarizeMissing`, `Store.revertWithSteps`'s pass loop, `Store.Reindex`) are unaffected by this plan's changes and remain their own, separate migrations onto `scrollAllPoints`.
- Plan 05-06 still owns: authoring and registering this phase's own red-evidence patches (`redEvidenceDirs`), the 64 MiB `MaxCallRecvMsgSize` backstop (D-05/D-06), closing #497 (D-07), and ticking `REQ-bounded-read-mechanism`/`REQ-sweeps-bounded` once every declaring plan in this phase has a SUMMARY (shared-ID gate #2388) — this plan's `requirements-completed` is deliberately empty for that reason.
- `requirements-completed` deliberately left empty: `REQ-bounded-read-mechanism` and `REQ-sweeps-bounded` are shared across this phase's plans and plan 05-06 owns ticking them once every declaring plan has a SUMMARY.

## Self-Check: PASSED

- `[ -f internal/store/revertpreview_oversized_test.go ]` → FOUND
- `git log --oneline --all | grep -q af0aee9e` → FOUND
- `git log --oneline --all | grep -q e9afaee6` → FOUND
- All plan-level `<acceptance_criteria>` re-verified passing (see per-task grep/build/test output above), with two stale-number clarifications documented under Issues Encountered (the `scrollAllPoints(ctx, s.collection,` count is 7, not the plan's stated 6, unchanged from plan 05-01) rather than silently forced to match.
- Plan-level `<verification>` block re-run: `TestRevertPreviewBoundedOverGRPCLimit` — 2/2 subtests PASS; inventory closing check (b) prints `0`; whole-package `nbudgetedView` grep prints `0`; full suite (`go test ./internal/store/ -count=1`, extended `-timeout 25m` to accommodate today's ambient system load) — `ok`, 869.847s, **zero** failures (better than the plan's expected "one red"; see Issues Encountered); `task lint` clean; `task license:check` clean; `git diff --exit-code HEAD -- go.mod go.sum` clean.

---

*Phase: 05-operator-sweeps-ci-backstop*
*Completed: 2026-09-20*
