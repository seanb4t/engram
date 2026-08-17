---
phase: 06-typed-operator-renderer
plan: 03
subsystem: cli
tags: [cobra, encoding-json, operator-renderer, cli-output]

requires:
  - phase: 06-typed-operator-renderer/06-01
    provides: "the one-serialization-plus-a-view mechanism (renderOperatorView, viewFields, assertViewIdentity) and the §Conversion Rules R1-R5 recipe this plan applies"
provides:
  - "reindex, summarize-missing, migrate-set-owner and migrate-remap-owner converted to headline-plus-complete-table under --output text, with zero new per-report renderer code"
  - "reindexSummary and migrateSetOwnerSummary's doc comments corrected — the retired byte-stability claim replaced with D-04 headline-producer framing and the D-03 statement that wording may change in any release"
  - "flatViewFixtures / TestFlatViewIdentity (cmd/engram/operator_view_flat_test.go) — this group's four reports under the shared identity gate, ready for plan 06-07's enumeration merge"
affects: [06-07-PLAN.md]

actuals:
  tokens: 1744
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "R1 headline trim is a no-op confirmation, not an edit, for every xxxSummary function that was already a single-line-per-variant pure formatter before this phase — recorded explicitly per report rather than left implicit"
    - "A report may build its document as an inline struct literal at the renderOperator call site (migrate-set-owner) rather than through a named converter function; under the view mechanism this is legitimate per R5 (no converter registry exists to be missing from), and the call site carries a one-line comment pointing at where its shape is fixture-gated"

key-files:
  created:
    - cmd/engram/operator_view_flat_test.go
  modified:
    - cmd/engram/reindex.go
    - cmd/engram/summarize.go
    - cmd/engram/migrate.go

key-decisions:
  - "R2 gap check run and recorded per report rather than assumed from R1's no-op status: reindexSummary's WouldUpsert/Unchanged/Skipped/Scanned/Upserted/target/dim are all present as reindexOutputDoc keys (the cutover variant's ENGRAM_QDRANT_COLLECTION= value is target again, derivable, not a gap); summarizeSummary's Filled/Scanned/Skipped/Failed are all present as summarizeOutputDoc keys; migrateSetOwnerSummary's owner/stamped are present as migrateSetOwnerReportDoc keys; migrateRemapSummary's n/owner are present as would_remap-or-remapped/owner (its --apply mention is flag prose, not a value). No key was added anywhere in this plan."
  - "migrateRemapSummary's doc comment was left untouched — it carried no byte-stability claim to correct, unlike reindexSummary and migrateSetOwnerSummary, and the plan's action text did not ask for an unconditional D-04 addendum there the way it did for summarizeSummary"

patterns-established:
  - "Doc-comment correction pattern for a retired byte-stability claim: state the function is a headline producer per D-04 (one line, non-exhaustive prose above a complete field table), state the field table carries every value, and state the sentence's wording may change in any release because --output json is the contract (D-03) — while preserving the pre-existing 'pure (no I/O)' and 'renderOperator supplies the trailing newline' clauses verbatim, since those remain true and load-bearing"

requirements-completed: [REQ-operator-renderer-typed]

coverage:
  - id: D1
    description: "reindex and summarize-missing render as headline-plus-complete-table under --output text with json documents byte-unchanged"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/reindex_test.go#TestReindexTextModeUnchanged"
        status: pass
      - kind: unit
        ref: "cmd/engram/reindex_test.go#TestReindexReportDocCarriesEverySummaryFact"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_flat_test.go#TestFlatViewIdentity/reindex"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_flat_test.go#TestFlatViewIdentity/summarize-missing"
        status: pass
    human_judgment: false
  - id: D2
    description: "migrate-set-owner and migrate-remap-owner render as headline-plus-complete-table under --output text with json documents byte-unchanged, and migrate-set-owner's inline struct-literal call site is preserved rather than refactored"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_test.go#TestMigrateRemapDryRunJSONDistinguishesPreviewFromApplied"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_flat_test.go#TestFlatViewIdentity/migrate-set-owner"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_flat_test.go#TestFlatViewIdentity/migrate-remap-owner"
        status: pass
    human_judgment: false
  - id: D3
    description: "All four reports' every sentence variant (3 reindex, 2 summarize, 1 set-owner, 2 remap) is exercised under the shared, non-vacuous identity gate and ready for plan 06-07's enumeration merge"
    requirement: "REQ-operator-renderer-typed"
    verification:
      - kind: unit
        ref: "cmd/engram/operator_view_flat_test.go#TestFlatViewIdentity"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-17
status: complete
---

# Phase 6 Plan 3: Typed Operator Renderer — flat report group conversion Summary

**`reindex`, `summarize-missing`, `migrate-set-owner` and `migrate-remap-owner` converted to the one-serialization-plus-a-view mechanism with zero new per-report renderer code, their headline doc comments corrected for D-03's unstable-text guarantee, and all eight sentence-variant fixtures gated under the shared identity check.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-08-17T14:36:00Z (approx.)
- **Completed:** 2026-08-17T14:41:24Z
- **Tasks:** 3
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments

- Applied `06-01-PLAN.md §Conversion Rules` R1 to `reindexSummary`, `summarizeSummary`, `migrateSetOwnerSummary` and `migrateRemapSummary`: all four were already single-line-per-variant pure headline producers, so R1 was a confirmation in every case, not an edit to any returned string.
- Ran R2's gap check explicitly for all four reports and recorded the conclusion in the frontmatter above: no fact stated by any of the eight sentence variants is missing from its doc struct, so no JSON key was added anywhere in this plan.
- Corrected `reindexSummary`'s and `migrateSetOwnerSummary`'s doc comments, which each claimed a specific pre-conversion sentence "returns ... unchanged, character for character" as a stability guarantee. 06-CONTEXT.md D-03 declares the text lane explicitly not a stable interface, so that guarantee is now false; replaced with D-04 headline-producer framing (one line, non-exhaustive prose above a complete field table the view renders) plus the D-03 statement that wording may change in any release. Extended `summarizeSummary`'s comment with the same D-04/D-03 framing (it carried no comparable claim to correct); left `migrateRemapSummary`'s comment untouched (it carried no comparable claim and the plan did not ask for an unconditional addendum there).
- Preserved `migrate-set-owner`'s inline `migrateSetOwnerReportDoc{Owner: migrateOwner, Stamped: n}` struct literal at its `renderOperator` call site exactly as R5 requires — not refactored into a named converter — and added a one-line comment recording that this is deliberate and that the document shape is enumerated by `flatViewFixtures`, so a future reader grepping for converter functions does not conclude the report is unconverted.
- Created `cmd/engram/operator_view_flat_test.go`: `flatViewFixtures()` (four `commandKey` entries — `reindex` with 3 variants, `summarize-missing` with 2, `migrate-set-owner` with 1, `migrate-remap-owner` with 2 — 8 documents total) and `TestFlatViewIdentity`, which runs the shared `assertViewIdentity` gate (defined by plan 06-01) over every fixture. All 8 subtests pass.
- Left `reindexOutputDoc`, `summarizeOutputDoc`, `migrateSetOwnerReportDoc`, `migrateRemapReportDoc` and all four `renderOperator` call sites structurally untouched (R5); `cmd/engram/operator_output_test.go` and `cmd/engram/migrate_family_test.go` (plan 06-04's file) are both untouched, so this plan stayed parallel-safe with 06-04/06-05/06-06.

## Task Commits

Each task was committed atomically:

1. **Task 1: Convert `reindex` and `summarize-missing`** — `143645d2` (docs)
2. **Task 2: Convert `migrate-set-owner` and `migrate-remap-owner`** — `4ad50cfa` (docs)
3. **Task 3: Put this group's four reports under the shared identity gate** — `752ec304` (test)

**Plan metadata:** committed alongside this SUMMARY (see final commit in this plan's range).

## Files Created/Modified

- `cmd/engram/reindex.go` — `reindexSummary`'s doc comment corrected (D-03/D-04 framing); no other change
- `cmd/engram/summarize.go` — `summarizeSummary`'s doc comment extended with D-03/D-04 framing; no other change
- `cmd/engram/migrate.go` — `migrateSetOwnerSummary`'s doc comment corrected; a one-line comment added at the inline `migrateSetOwnerReportDoc{...}` call site recording the deliberate literal
- `cmd/engram/operator_view_flat_test.go` — `flatViewFixtures`, `TestFlatViewIdentity` (new)

## Decisions Made

- R2's gap check was run and its conclusion recorded explicitly for each of the four reports (see `key-decisions` in the frontmatter above) rather than left to be inferred from R1 being a no-op — the plan's `<output>` instruction required this visibility.
- `migrateRemapSummary`'s doc comment was left as-is: it carried no byte-stability claim comparable to `reindexSummary`'s or `migrateSetOwnerSummary`'s, and Task 2's action text asked for the correction only on `migrateSetOwnerSummary`.

## Deviations from Plan

None - plan executed exactly as written. No Rule 1-4 auto-fixes were needed: no bug, no missing critical functionality, and no blocking issue was found in any of the three tasks.

## Issues Encountered

None. `Edit` initially failed to match a larger multi-line `old_string` block in `cmd/engram/migrate.go` on the first attempt (likely a whitespace/context-window mismatch in the tool call, not a real file discrepancy — a narrower, single-line `old_string` matched immediately on retry); this had no effect on the committed result and required no deviation rule.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All four flat reports in this plan's scope are fully converted: headline producers confirmed structural-only, doc structs byte-unchanged, and every sentence variant gated under `assertViewIdentity` via `flatViewFixtures`.
- Plan 06-07 depends on this plan's `flatViewFixtures` function (along with the other conversion plans' group fixture functions and plan 06-01's `pruneViewFixtures`) to build the merged `operatorViewFixtures` map and the both-directions enumeration gate against `operatorCommands()`.
- `go test ./cmd/engram/...` and the full `task` (lint + test) gate both exit 0. `task license:check` exits 0 — the new file carries the required SPDX header.
- No blockers.

---
*Phase: 06-typed-operator-renderer*
*Completed: 2026-08-17*

## Self-Check: PASSED

- `cmd/engram/operator_view_flat_test.go` exists: FOUND
- `cmd/engram/reindex.go`, `cmd/engram/summarize.go`, `cmd/engram/migrate.go` all modified and present: FOUND
- Commit `143645d2` (Task 1): FOUND in `git log --oneline --all`
- Commit `4ad50cfa` (Task 2): FOUND in `git log --oneline --all`
- Commit `752ec304` (Task 3): FOUND in `git log --oneline --all`
- `go test ./cmd/engram/ -run 'TestReindex|TestSummarize'` exits 0: PASSED
- `go test ./cmd/engram/ -run 'TestMigrateSetOwner|TestMigrateRemap'` exits 0: PASSED
- `go test ./cmd/engram/ -run 'TestFlatViewIdentity' -v` prints 8 `--- PASS` subtest lines and exits 0: PASSED
- `go test ./cmd/engram/...` exits 0: PASSED
- `task` (lint + test, full module) exits 0: PASSED
- `task license:check` exits 0: PASSED
- `git diff cmd/engram/reindex.go cmd/engram/summarize.go cmd/engram/migrate.go` shows no change inside any `*Doc struct` block: PASSED
- `git diff --exit-code cmd/engram/operator_output_test.go` exits 0: PASSED
- `git diff --exit-code cmd/engram/migrate_family_test.go` exits 0: PASSED
- `rg -c 'character for character' cmd/engram/reindex.go cmd/engram/migrate.go` outputs 0 for both: PASSED
- All plan-level `<verification>` and `<success_criteria>` commands re-run and passing (see above)
