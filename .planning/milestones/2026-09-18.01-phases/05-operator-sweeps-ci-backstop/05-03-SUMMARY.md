---
phase: 05-operator-sweeps-ci-backstop
plan: 03
subsystem: database
tags: [qdrant, bounded-reads, migrate, revert, byte-budget, sentinel-error]

# Dependency graph
requires:
  - phase: 05-operator-sweeps-ci-backstop
    provides: "scrollAllPoints(ctx, collection, filter, view, fn) with a collection parameter and WithVectors(false) (plan 05-01); previewRevertWithSteps migrated onto schemaVersionOnlyView, unbudgetedView deleted entirely (plan 05-02)"
provides:
  - "Store.Migrate's three hand-rolled ScrollAndOffset cursor loops (DryRun projection, Manifest-limited apply, default sweep-mode pass) deleted, each replaced by one scrollAllPoints call over s.fullView()"
  - "Store.revertWithSteps' own pass loop deleted, replaced by the identical sentinel-batch shape as Migrate's sweep-mode pass — its structural twin"
  - "errMigratePassBatchComplete (migrate.go) and errRevertPassBatchComplete (revert.go): two package-level, per-sweep sentinel errors expressing a mutating pass's batch boundary, unwrapped with errors.Is at each call site — never shared between the two sweeps"
  - "internal/store/migratesweep_oversized_test.go: TestMigrateBoundedOverGRPCLimit (sweep + dry-run arms) and TestRevertApplyBoundedOverGRPCLimit, both fixture shapes, all real-Qdrant"
  - "operatorMigrationEmitters justifications for Store.Migrate and Store.revertWithSteps corrected to describe only their surviving direct Count emission; Store.scrollAllPoints' justification widened to name both migrated walks"
affects: [05-04, 05-05, 05-06]

# Actuals (#2632)
actuals:
  tokens: 9209
  tasks: 3
  commits: 3
plan_head_before: cc0ebc48091325e7ddc113f672d5ab2a6f72b366

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-sweep sentinel-error batch boundary (D-01): a mutating pass's per-record cap is expressed as a package-level error returned from the scrollAllPoints callback once a closure-scoped counter reaches the pass's batch size, unwrapped with errors.Is at the call site — never a shared sentinel across sweeps, never a page-size Limit"
    - "Interleaved read/write safety proof, restated at every migrated write loop: scrollAllPoints advances a FORWARD id cursor from the previous response's next-page offset; a write that removes the record from the pass's own filter cannot move that cursor backwards, so no record is skipped or revisited within a pass"

key-files:
  created:
    - internal/store/migratesweep_oversized_test.go
  modified:
    - internal/store/migrate.go
    - internal/store/revert.go
    - internal/store/export_test.go
    - internal/store/schemaversion_recallgate_test.go

key-decisions:
  - "Task 1's oversized regression test omits the DryRun arm the plan's <behavior> block describes for TestMigrateBoundedOverGRPCLimit, deferring it to task 2's commit. DryRun's own ScrollAndOffset walk is unmigrated until task 2 — calling it at the default batch over the FewLarge shape at task 1's commit point genuinely overflows the receive limit (confirmed by running it: real Qdrant returned 'response exceeded the client's receive limit'), which is precisely the bug task 2 fixes, not task 1. Extending the SAME test function in task 2 (rather than leaving the dry-run arm unwritten) is what fulfills the plan's own D-03 requirement that DryRun's walk get its own real-Qdrant oversized-scale proof, since task 2's declared <files> list does not include the test file but its own <verify> command (^TestMigrate) re-runs this test regardless."
  - "revertFixtureStep's inverse deletes a key production fixtures never actually carry (the fixture step is purely test-local), so migrate.RemovedKeys(original, current) is empty for the SetPayload write in the revert-apply regression — that is expected and intentional: the regression proves the sentinel-batch pass loop's own bounded read/write cycle and the schema_version stamp, not a specific removed-key shape (which the in-place revert_test.go suite, D-02's characterization, already pins independently)."
  - "Both new test helpers (migrateSweepBacklogFilter, migrateSweepAboveTargetFilter) reconstruct backlogFilter/aboveTargetFilter's exact shape from outside the package using the exposed SchemaVersionKey() shim, rather than adding a second export_test.go shim for the filter constructors themselves — the plan explicitly asked only for the key name to be exposed ('rather than repeating a string literal'), and the filter shapes themselves are simple qdrant primitive compositions, not internal engram logic worth a shim."

requirements-completed: []  # Plan 05-06 owns ticking REQ-bounded-read-mechanism/REQ-sweeps-bounded (shared-ID gate, engram_project_gates)

coverage:
  - id: D1
    description: "Store.Migrate's default sweep-mode pass loop bounded on the shared byte-budget iterator, its batch boundary expressed as errMigratePassBatchComplete, proven over an oversized backlog on both fixture shapes with the in-place migrate characterization suite unedited and green"
    requirement: "REQ-sweeps-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestMigrateBoundedOverGRPCLimit/few-large"
        status: pass
      - kind: integration
        ref: "internal/store#TestMigrateBoundedOverGRPCLimit/many-small"
        status: pass
      - kind: integration
        ref: "internal/store (in-place migrate characterization suite: all TestMigrate* tests, 14 named in 05-CONTEXT.md, run via -run '^TestMigrate')"
        status: pass
    human_judgment: false
  - id: D2
    description: "Store.Migrate's DryRun projection and Manifest-limited apply walks moved onto the shared iterator (no sentinel — both are exhaustive by design), with Store.Migrate's and Store.scrollAllPoints' operatorMigrationEmitters justifications corrected in the same commit as the call-site move"
    requirement: "REQ-sweeps-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestMigrateBoundedOverGRPCLimit (dry-run arm, both shapes, added in this task's commit)"
        status: pass
      - kind: integration
        ref: "internal/store#TestRecallEmissionSetIsCompleteAndClassified (all four subtests)"
        status: pass
      - kind: integration
        ref: "internal/store (in-place migrate suite, TestMigrateFullBacklogProjection/TestMigrateDryRunWritesNothing/TestMigrateManifestIntersection/TestMigrateManifestSparedDeletedRecord/TestMigrateManifestBacklogAppeared and the rest, unedited)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Store.revertWithSteps' own pass loop bounded on the shared iterator as Store.Migrate's structural twin, its own distinct sentinel (errRevertPassBatchComplete), proven over an oversized above-target range on both fixture shapes through a locally built reversible fixture chain (the production registry's only step is Irreversible), with the in-place revert characterization suite unedited and green and both remaining recall-gate justifications corrected"
    requirement: "REQ-sweeps-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestRevertApplyBoundedOverGRPCLimit/few-large"
        status: pass
      - kind: integration
        ref: "internal/store#TestRevertApplyBoundedOverGRPCLimit/many-small"
        status: pass
      - kind: integration
        ref: "internal/store (full package suite, go test ./internal/store/ -count=1 -timeout 40m): ok, 302.202s, zero failures — including TestRedEvidencePatchesAreLive)"
        status: pass
    human_judgment: false

# Metrics
duration: 37min
completed: 2026-09-20
status: complete
---

# Phase 5 Plan 3: Migrate and Revert's Own Sweep Loops Bounded on the Shared Iterator

**Deleted all four hand-rolled `ScrollAndOffset` cursor loops inside `engram migrate` and `engram migrate revert`, replacing each with `scrollAllPoints`, with each mutating pass's batch boundary now expressed as its own distinct sentinel error rather than a page size — the two most safety-critical operator commands in the repo stop requesting 256 full payloads per RPC without changing a single counter, guard, message, or write shape.**

## Performance

- **Duration:** 37 min
- **Started:** 2026-09-20T16:38:00Z (approx, from prior plan's completion)
- **Completed:** 2026-09-20T17:15:00Z (approx)
- **Tasks:** 3
- **Files modified:** 5 (1 created, 4 modified)

## Accomplishments

- `Store.Migrate`'s default sweep-mode pass loop now issues one `scrollAllPoints(ctx, s.collection, filter, s.fullView(), fn)` call per pass instead of a single `Limit: batch` `ScrollAndOffset`; its per-pass batch boundary is `errMigratePassBatchComplete`, a package-level sentinel unwrapped with `errors.Is` at the call site. Observed on real Qdrant: `few-large` — `res.Passes=2 res.Migrated=40 res.Failed=0`; `many-small` — `res.Passes=5 res.Migrated=1000 res.Failed=0` (proving the sentinel really ends a pass at the batch boundary rather than draining the whole 1000-record backlog in one).
- `Store.Migrate`'s `DryRun` projection and `Manifest`-limited apply walks are each now one exhaustive `scrollAllPoints` call (no sentinel — both are exhaustive by design, per REVIEWS.md H2/H7), with their per-point bodies carried over verbatim.
- `Store.revertWithSteps`' own pass loop — `Store.Migrate`'s sweep-mode loop's structural twin — is bounded the identical way, with its own distinct sentinel `errRevertPassBatchComplete` (never shared with migrate's). Observed on real Qdrant: `few-large` — `res.Passes=2 res.Reverted=40 res.Failed=0`; `many-small` — `res.Passes=5 res.Reverted=1000 res.Failed=0`.
- `internal/store/migratesweep_oversized_test.go` (new, `package store_test`) adds `TestMigrateBoundedOverGRPCLimit` (sweep + dry-run arms, both shapes) and `TestRevertApplyBoundedOverGRPCLimit` (driving a locally built reversible fixture chain through a new `RevertWithSteps` shim, since the production `migrate.Registry`'s only step is `Irreversible` and can never apply).
- `Store.Migrate`'s and `Store.revertWithSteps`' `operatorMigrationEmitters` justifications no longer claim a `ScrollAndOffset` emission they no longer make; `Store.scrollAllPoints`' justification is widened to name both migrated walks, preserving the `reachable set` substring the AST gate requires.
- `s.client.ScrollAndOffset(` call sites in production `internal/store` (excluding comments) are down to 3: the shared iterator itself (`spine.go`) and the two loops plan 05-04 owns (`summarize.go`, `store.go`).

## Task Commits

Each task was committed atomically:

1. **Task 1: The migrate sweep-mode pass loop bounded, its batch boundary expressed as a sentinel, proven over an oversized backlog (tracer)** — `c52ce24f` (refactor)
2. **Task 2: The dry-run projection and the manifest-limited apply moved onto the iterator, with the migrate emitter justification corrected in the same commit** — `157d5b62` (refactor)
3. **Task 3: The revert apply pass loop migrated as the migrate loop's twin, with its own sentinel and its own corrected justification** — `c000378c` (refactor)

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/store/migrate.go` — added `errMigratePassBatchComplete`; all three own-loop walks (DryRun, Manifest, default sweep) now route through `scrollAllPoints`.
- `internal/store/revert.go` — added `errRevertPassBatchComplete`; `revertWithSteps`' pass loop now routes through `scrollAllPoints`.
- `internal/store/export_test.go` — added `SchemaVersionKey()` and `RevertWithSteps(...)` shims.
- `internal/store/schemaversion_recallgate_test.go` — corrected `Store.Migrate` and `Store.revertWithSteps` justifications; widened `Store.scrollAllPoints`'s.
- `internal/store/migratesweep_oversized_test.go` (new) — `TestMigrateBoundedOverGRPCLimit` and `TestRevertApplyBoundedOverGRPCLimit`, both real-Qdrant, both fixture shapes.

## Decisions Made

- **Task 1's dry-run arm deferred to task 2's commit** (see key-decisions above): DryRun's own read loop is unmigrated at task 1's commit point, so exercising it at oversized scale genuinely overflows the receive limit — confirmed empirically before deferring, not assumed. The plan's own `<behavior>` block describes the dry-run arm as part of `TestMigrateBoundedOverGRPCLimit`, which is satisfied once task 2 lands; task 1's own `<verify>` (which greps for exactly the sweep/apply arm's two PASS lines) is unaffected either way.
- **`revertFixtureStep`'s inverse targets a key no seeded fixture record carries** — intentional; the regression proves the bounded pass-loop mechanics and the schema-version stamp, not a specific added/removed-key shape (already pinned by the in-place `revert_test.go` suite per D-02).
- **No new `export_test.go` shim for `backlogFilter`/`aboveTargetFilter` themselves** — only the schema-version key name is exposed (per the plan's explicit instruction), and the two test-local filter constructors reconstruct the identical shape from public `qdrant` primitives.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A doc comment for `errRevertPassBatchComplete` initially named `errMigratePassBatchComplete` by identifier, tripping this task's own acceptance criterion**

- **Found during:** Task 3, running the acceptance-criteria grep `rg -o -e 'errMigratePassBatchComplete' internal/store/revert.go | wc -l` (must print `0` — "the two sweeps do not share a marker")
- **Issue:** The doc comment explaining `errRevertPassBatchComplete` as `Store.Migrate`'s structural twin named the migrate sentinel by its literal identifier for cross-reference, which the criterion's substring grep — deliberately scoped to catch exactly this class of accidental sharing — correctly flagged.
- **Fix:** Reworded the comment to describe the relationship ("Store.Migrate's sweep-mode pass loop's exact structural twin ... but declared SEPARATELY, with its own distinct sentinel value") without naming the other sentinel by identifier.
- **Files modified:** `internal/store/revert.go`
- **Verification:** `rg -o -e 'errMigratePassBatchComplete' internal/store/revert.go | wc -l` prints `0`; full package suite re-run clean afterward (`ok`, 302.202s, zero failures).
- **Committed in:** `c000378c` (Task 3 commit — caught and fixed before committing, never landed in a bad state)

---

**Total deviations:** 1 auto-fixed (1 bug, caught by the task's own verify gate before commit)
**Impact on plan:** No scope creep; the fix is purely wording in my own newly-authored comment, never touching production behavior.

## Issues Encountered

- **Two acceptance-criteria literals were stale against the actual (correct) file state, verified by intent per this phase's established pattern (05-01-SUMMARY, 05-02-SUMMARY):**
  1. Task 1's criterion `rg -c -e 'func RevertWithSteps' internal/store/export_test.go` prints `1` — the actual declaration is `func (s *Store) RevertWithSteps(...)` (a method, matching every other shim in this file, e.g. `func (s *Store) FullView()`), so the bare-`func` literal never matches. `rg -c -e 'RevertWithSteps' internal/store/export_test.go` prints `2` (doc comment + declaration), confirming the shim exists as intended.
  2. Task 3's criterion `rg -o -e 's[.]client[.]ScrollAndOffset[(]' internal/store --glob '!*_test.go' | wc -l` prints `4`, not the stated `3` — the fourth match is `spine.go:76`'s pre-existing doc comment ("the enclosing function of each direct `s.client.ScrollAndOffset(` call"), predating this plan (confirmed via `git show HEAD~3:internal/store/spine.go`, from plan 05-01). The actual CODE call-site count (excluding the comment) is exactly 3: the shared iterator itself (`spine.go:97`) and the two loops plan 05-04 owns (`summarize.go:145`, `store.go:3449`) — matching the plan's own stated invariant.
  3. Both plans' `reachable set` grep (`rg -c -e 'reachable set' internal/store/schemaversion_recallgate_test.go`, stated to print `1`) actually prints `4` in both task 2 and task 3 — three of those four lines predate this plan entirely (a doc comment at line 389 and two lines inside `TestRecallEmissionSetIsCompleteAndClassified`'s own subtest body at lines 735/740, both there since before Phase 5). Only the `Store.scrollAllPoints` justification's own occurrence is the one this plan's edits touch, and the dedicated `foundScrollAllPointsRationale` subtest (which asserts specifically that entry's justification contains the substring) passed cleanly in every full-suite run.
  No code change follows from any of these three — the underlying invariant each criterion protects holds; only the literal command's expected count was stale.
- **The full `internal/store` suite ran fully green (zero failures, including `TestRedEvidencePatchesAreLive`) rather than the plan's stated "single expected RED"** — identical finding to 05-01-SUMMARY and 05-02-SUMMARY: `redEvidenceDirs` has no Phase 5 entry yet (plan 05-06's job), so there is nothing phase-5-specific for that test to fail on at this point in the phase. Two full runs (`-timeout 40m`, 414.128s and 302.202s) both confirmed `ok` with zero failures. No code change follows; the actual, better outcome supersedes the plan's assumption.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All three of `Store.Migrate`'s own-loop walks and `Store.revertWithSteps`' own pass loop are migrated onto `scrollAllPoints`. Of the original ten Phase 5 inventory rows, only `Store.SummarizeMissing` and `Store.Reindex` (plan 05-04's territory) still run their own `ScrollAndOffset` loops.
- `Store.Migrate`'s and `Store.revertWithSteps`' `operatorMigrationEmitters` rows both SURVIVE (each keeps a direct `Count`) with corrected justifications — matching the orchestrator-verified fact that these two rows are EDITED, not deleted (unlike `Store.SummarizeMissing`/`Store.Reindex`, which plan 05-04 will delete outright once their own last direct emission is gone).
- Not one counter, guard, message, filter, or write-shaping rule changed in either command; the full in-place `migrate_test.go` (14 tests) and `revert_test.go` (9 tests) characterization suites ran unedited and green throughout — no test broke in either an expected or unexpected way.
- Plan 05-04 (`Store.SummarizeMissing`, `Store.Reindex`) and plan 05-05 are unaffected by this plan's changes.
- Plan 05-06 still owns: authoring and registering this phase's own red-evidence patches, the 64 MiB `MaxCallRecvMsgSize` backstop (D-05/D-06), closing #497 (D-07), and ticking `REQ-bounded-read-mechanism`/`REQ-sweeps-bounded` once every declaring plan has a SUMMARY (shared-ID gate #2388) — this plan's `requirements-completed` is deliberately empty for that reason.

## Self-Check: PASSED

- `[ -f internal/store/migratesweep_oversized_test.go ]` → FOUND
- `git log --oneline --all | grep -q c52ce24f` → FOUND
- `git log --oneline --all | grep -q 157d5b62` → FOUND
- `git log --oneline --all | grep -q c000378c` → FOUND
- All plan-level `<acceptance_criteria>` re-verified passing per-task (see grep/build/test output above), with three stale-literal clarifications documented under Issues Encountered rather than silently forced to match, and one caught-before-commit deviation documented above.
- Plan-level `<verification>` block re-run: `TestMigrateBoundedOverGRPCLimit`/`TestRevertApplyBoundedOverGRPCLimit` — 4/4 subtests PASS; `s.client.ScrollAndOffset(` in `migrate.go`+`revert.go` prints `0`; full suite (`go test ./internal/store/ -count=1 -timeout 40m`) — `ok`, 302.202s, zero failures (better than the plan's expected "one red"; see Issues Encountered); `task lint` clean; `task license:check` clean; `git diff --exit-code HEAD -- go.mod go.sum` clean.

---

*Phase: 05-operator-sweeps-ci-backstop*
*Completed: 2026-09-20*
