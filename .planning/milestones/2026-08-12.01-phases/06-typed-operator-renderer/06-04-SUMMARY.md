---
phase: 06-typed-operator-renderer
plan: 04
subsystem: cli
tags: [cobra, encoding-json, operator-renderer, cli-output, migrate]

requires:
  - phase: 06-typed-operator-renderer
    provides: "renderOperatorView/viewFields/assertViewIdentity mechanism (06-01), proven on prune-expired"
provides:
  - "migrate, migrate revert, migrate status, and backfill-short-ids all converted to the one-serialization-plus-a-view mechanism"
  - "migrateStatusReportDoc: the tier's last hand-declared-vs-embedded-store-type gap closed — migrate status no longer serializes store.MigrateStatusResult directly"
  - "migrateViewFixtures / TestMigrateViewIdentity: ten document variants across four commandKeys under the shared identity gate"
affects: [06-07-PLAN.md]

actuals:
  tokens: 3702
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "R1 headline trim can require deleting a per-item enumeration loop from an existing xxxSummary, not just confirming an already-single-line function (migrate status was the first group to actually exercise this, vs. 06-01's prune-expired where R1 was a no-op)"
    - "A hand-declared CLI doc struct reproduces an embedded store-result type's keys/tags/order exactly for the pre-existing fields, then appends any R2 gap-closure key last — never interleaved"

key-files:
  created:
    - cmd/engram/operator_view_migrate_test.go
  modified:
    - cmd/engram/migrate_family.go
    - cmd/engram/migrate_family_test.go

key-decisions:
  - "migrateStatusReportDoc's five pre-existing fields are declared in the exact order/tags of store.MigrateStatusResult, with CurrentVersion/current_version appended last — preserves json byte-for-byte compatibility on every pre-existing key while adding exactly the one fact (R2) statusSummary already stated but no key carried"
  - "statusSummary's per-future-bucket `v%d=%d` loop is deleted per R1's literal instruction, not partially retained — the future-version detail now lives entirely in the `future` field the identity-gated view renders as its own row; kept only the single non-enumerating prose clause R1/D-04 call for"
  - "backfill-short-ids's preview-variant fixture is built directly from migrateReportDoc (not a backfill-specific converter), matching the alias's own delegation onto migrateSweepPreviewRun — documented in both the fixture file and migrateSweepPreviewRun's own doc comment"

patterns-established:
  - "When R1's headline trim deletes a fact the pre-existing (soon-to-be-retired) TestOperatorOutputParity hardcodes, that subtest is EXPECTED to fail until 06-07 (wave 3, depends on every wave-2 conversion plan) deletes it — not a defect to work around inside a wave-2 plan, which is prohibited from touching that file"

requirements-completed: [REQ-operator-renderer-typed]

coverage:
  - id: D1
    description: "migrate and migrate revert (and the backfill-short-ids alias riding the same sweep functions) render one text line per JSON key, with json documents byte-unchanged"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyPreviewAndApply"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyRevertRefusals"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyRevertReversible"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_migrate_test.go#TestMigrateViewIdentity/migrate"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_migrate_test.go#TestMigrateViewIdentity/migrate_revert"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_migrate_test.go#TestMigrateViewIdentity/backfill-short-ids"
        status: pass
    human_judgment: false
  - id: D2
    description: "migrate status serializes a hand-declared migrateStatusReportDoc (not store.MigrateStatusResult passed through), preserving all five pre-existing keys/tags/order and adding current_version"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyStatusReportDocNeverMarshalsNull"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyStatusReportDocKeyOrder"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_migrate_test.go#TestMigrateViewIdentity/migrate_status"
        status: pass
    human_judgment: false
  - id: D3
    description: "The tier's last store-type passthrough is gone: rg -c 'store\\.MigrateStatusResult' cmd/engram/migrate_family.go counts only parameter positions, never a return or renderOperator argument"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "manual rg verification (see Deviations/verification section below)"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-08-17
status: complete
---

# Phase 6 Plan 4: Typed Operator Renderer — migrate family conversion Summary

**Converted `migrate`, `migrate status`, `migrate revert`, and the `backfill-short-ids` alias to the one-serialization-plus-a-view mechanism, closing the tier's last store-type passthrough with a new hand-declared `migrateStatusReportDoc`.**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-08-17T10:30:00Z (approx.)
- **Completed:** 2026-08-17T10:46:41Z
- **Tasks:** 3
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments

- `migrateSummary` and `revertSummary` confirmed as R1-compliant single-line headline producers (no edit to returned text); R2 gap-check run and recorded — every fact either summary states is already a key in `migrateOutputDoc`/`revertOutputDoc`, so no new key was needed for those two. `backfill-short-ids` required zero source changes: its two closures are one-line adapters over the same sweep functions this task converted.
- `migrateStatusReportDoc` declared in `cmd/engram/migrate_family.go`: hand-declared, not an embedded `store.MigrateStatusResult` — the tier's last such passthrough is closed. Its first five fields reproduce `store.MigrateStatusResult`'s own keys/tags/order exactly; `CurrentVersion`/`current_version` is appended last per R2, because `statusSummary` already states this binary's own target version and no pre-existing key carried it.
- `statusSummary` trimmed per R1: the per-future-bucket `v%d=%d` enumeration loop deleted; the headline now states only that some records are at a newer version, without enumerating them — that detail now lives in the `future` field the view renders as its own row.
- `cmd/engram/operator_view_migrate_test.go` created: `migrateViewFixtures()` keyed by all four commandKeys (`migrate`, `migrate status`, `migrate revert`, `backfill-short-ids`), ten document variants total, each built by calling the report's real converter and reusing fixed values from the (soon-to-be-retired) `operatorParityRows` entries. `TestMigrateViewIdentity` runs the shared `assertViewIdentity` gate over all ten — all pass.

## Task Commits

Each task was committed atomically:

1. **Task 1: Convert `migrate` and `migrate revert` (and the `backfill-short-ids` alias)** — `6da30bc8` (docs)
2. **Task 2: Give `migrate status` a hand-declared CLI document** — `102e5c00` (feat)
3. **Task 3: Put the migrate family's four commands under the shared identity gate** — `03a4ff5e` (test)

**Plan metadata:** committed alongside this SUMMARY.

## Files Created/Modified

- `cmd/engram/migrate_family.go` — `migrateStatusReportDoc` (new struct); `statusReportDoc`'s signature changed to return it; `statusSummary` trimmed (future-bucket loop deleted); `migrateSummary`/`revertSummary`/`migrateSweepPreviewRun` doc comments extended per D-04/R2/R3 provenance recording
- `cmd/engram/migrate_family_test.go` — `TestMigrateFamilyStatusReportDocKeyOrder` added (pins the new struct's marshaled key order)
- `cmd/engram/operator_view_migrate_test.go` — migrate-family fixtures and identity-gate test (new)

`cmd/engram/backfill_test.go` was declared in `files_modified` but required no edit: it carried no exact-whole-stdout text comparisons to convert per R4 (already structural).

## Decisions Made

- Applied R1 to `migrateSummary`/`revertSummary` as a **confirmation** (both already returned exactly one line) rather than an edit — matches 06-01's precedent for `prune-expired`'s already-single-line summaries.
- R2 gap-check conclusion recorded in code doc comments and here: `migrateSummary` states `wouldMigrate`, `target`, `res.Migrated`, `res.Backlog`, `res.Failed`, `len(res.Spared)`, `len(res.Appeared)` — all present in `migrateOutputDoc`. `revertSummary` states the refusal string, `res.Reverted`, `res.Failed`, `plan.Candidates`, `plan.To`, `res.Backlog` — all present in `revertOutputDoc`. Its `engram migrate status` reconcile hint is prose naming a command, not a value. **No key was added for Task 1's two reports.**
- `statusSummary`'s R2 gap-check: it states `migrate.CurrentVersion` (the binary's own target) which had no corresponding key in `store.MigrateStatusResult` — this is the one fact added as `current_version` in Task 2, per the plan's explicit instruction.
- `migrateStatusReportDoc`'s field declaration order matches `store.MigrateStatusResult`'s exactly for the five pre-existing keys, with `CurrentVersion` appended last — never interleaved — so the json lane is unchanged for every pre-existing key and the new key is unambiguously additive.
- `backfill-short-ids`'s fixtures in Task 3 are built from `migrateReportDoc` (not a backfill-specific converter), matching the alias's real delegation onto `migrateSweepPreviewRun`/`migrateSweepApplyRun` — documented both in the fixture file's comments and in `migrateSweepPreviewRun`'s own doc comment (recording that the PREVIEW variant is the one a hand-registered list missed once, per 06-CONTEXT.md `<specifics>`).

## Deviations from Plan

### Auto-fixed Issues

None — no bugs, missing critical functionality, or blocking issues requiring Rule 1–3 auto-fixes were found during implementation.

### Documented, Unresolvable-Within-Scope Deviation

**1. [Deferred to 06-07] `TestOperatorOutputParity/migrate_status` fails after Task 2's R1 trim**

- **Found during:** Task 2 verification, and confirmed persisting through Task 3's full-suite acceptance check.
- **Issue:** Task 2's action instructs deleting `statusSummary`'s per-future-bucket `v%d=%d` enumeration loop (R1), because that detail now belongs in the view's `future` field rows. The pre-existing `TestOperatorOutputParity` in `cmd/engram/operator_output_test.go` (a file this plan is explicitly prohibited from editing, per R3's enforced verification `git diff --exit-code cmd/engram/operator_output_test.go`) hardcodes a `facts` list for the `"migrate status"` row including `"2"` — the version number of the single future bucket in its fixed test value — which was previously only reachable through that now-deleted enumeration clause. With the clause removed, the headline text no longer contains `"2"`, and the subtest fails: `row "migrate status": text "..." does not contain declared fact "2" (row is malformed)`.
- **Why this is not a Rule 1/2/3 auto-fix:** Every path to satisfying this test requires either (a) editing `cmd/engram/operator_output_test.go`, which is explicitly prohibited by this plan with a machine-verified acceptance criterion, or (b) not fully applying Task 2's explicit R1 instruction (re-adding some form of per-bucket enumeration to the headline), which would contradict the task's own `<action>` text and the phase's D-04 "headline is non-exhaustive, detail belongs in the table" design intent. Rule 4 (architectural decision requiring a stop) does not fit either, because the resolution is already decided at the phase-planning level: `.planning/phases/06-typed-operator-renderer/06-07-PLAN.md` (wave 3) explicitly `depends_on: ["06-02", "06-03", "06-04", "06-05", "06-06"]` — every wave-2 conversion plan, including this one — and its own stated purpose is to delete `TestOperatorOutputParity`/`operatorParityRows` entirely (D-09: "obsolete because there is no longer a text/json divergence for a parity test to detect"). The dependency graph confirms this is a designed sequencing point, not a gap this plan can or should close.
- **Verification of scope:** `go test ./... ` across the entire module fails on exactly this one subtest (`TestOperatorOutputParity/migrate_status`) and no other — confirmed via `go test ./... 2>&1 | grep -E '^(ok|FAIL|---)'`, showing every other package `ok` and only `cmd/engram`'s `TestOperatorOutputParity` failing. `task lint` and `task license:check` are both clean. All of this plan's own tests (`TestMigrateFamily*`, `TestBackfill*`, `TestMigrateViewIdentity`) pass.
- **Files that would need to change to resolve this (out of scope for this plan):** `cmd/engram/operator_output_test.go` — reserved for 06-07.
- **Resolution:** Defer to `06-07-PLAN.md`, which deletes `TestOperatorOutputParity` as its stated purpose. Recorded in the phase's cross-plan defect ledger (see below).

---

**Total deviations:** 0 auto-fixed; 1 documented cross-plan-sequencing deviation, resolved by a dependent downstream plan already in the phase's plan graph.
**Impact on plan:** No scope creep, no workaround that would compromise D-01/D-04's design intent. The one test failure is fully attributable to an explicitly-scheduled, already-planned-for retirement in 06-07 and does not indicate any defect in this plan's own converted code or new tests.

## Verification of "no store-type passthrough" prohibition

```
$ rg -n 'func statusReportDoc' cmd/engram/migrate_family.go
func statusReportDoc(res store.MigrateStatusResult) migrateStatusReportDoc {

$ rg -c 'store\.MigrateStatusResult' cmd/engram/migrate_family.go
2   # both are parameter-position uses (migrateFamilyStore interface method,
    # statusReportDoc's own parameter) — never a return type or a
    # renderOperator argument.
```

`rg -A 10 'type migrateStatusReportDoc struct' cmd/engram/migrate_family.go` shows six tagged fields and no bare embedded `store.` type.

## Issues Encountered

See "Documented, Unresolvable-Within-Scope Deviation" above — the one `TestOperatorOutputParity` subtest failure, deferred to 06-07 by design.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All four migrate-family commands (`migrate`, `migrate status`, `migrate revert`, `backfill-short-ids`) are converted to the one-serialization-plus-a-view mechanism; their json documents are unchanged except `migrate status`'s single additive `current_version` key.
- The tier's last store-type passthrough (`migrate status` serializing `store.MigrateStatusResult` directly) is closed — every operator report now serializes a hand-declared CLI-side struct.
- `06-07-PLAN.md` (wave 3) can proceed once all of `06-02`, `06-03`, `06-05`, `06-06` also land: it merges every group's fixture function, adds the both-directions enumeration gate against `operatorCommands()`, and deletes `TestOperatorOutputParity`/`operatorParityRows` — which will resolve this plan's one documented deviation.
- No other blockers.

---
*Phase: 06-typed-operator-renderer*
*Completed: 2026-08-17*

## Self-Check: PASSED

- `cmd/engram/operator_view_migrate_test.go` exists: FOUND
- `cmd/engram/migrate_family.go`, `cmd/engram/migrate_family_test.go` modified and present: FOUND
- Commit `6da30bc8` (Task 1): FOUND in `git log --oneline --all`
- Commit `102e5c00` (Task 2): FOUND in `git log --oneline --all`
- Commit `03a4ff5e` (Task 3): FOUND in `git log --oneline --all`
- `go test ./cmd/engram/ -run 'TestMigrateFamily|TestBackfill' -v` exits 0: PASSED
- `go test ./cmd/engram/ -run 'TestMigrateViewIdentity' -v` exits 0, 10 `--- PASS` subtests: PASSED
- `task lint` exits 0: PASSED
- `task license:check` exits 0: PASSED
- `go test ./cmd/engram/...` and `go test ./...`: PASSED except `TestOperatorOutputParity/migrate_status` (documented above, deferred to 06-07)
- `git diff --exit-code cmd/engram/operator_output_test.go` exits 0: PASSED
- `git diff --exit-code cmd/engram/backfill.go` exits 0: PASSED
- No file outside declared `files_modified` was touched (verified via `git diff --stat` against the plan's base commit): PASSED
