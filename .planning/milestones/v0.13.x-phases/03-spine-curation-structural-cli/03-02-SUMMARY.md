---
phase: 03-spine-curation-structural-cli
plan: 02
subsystem: cli
tags: [cobra, output-format, exit-codes, operator-cli]

# Dependency graph
requires:
  - phase: 03-spine-curation-structural-cli
    provides: "plan 03-01's cmdwalk.go (walkCommands/commandWalkSkip/commandKey), operator_output.go (addOperatorOutputFlag/operatorOutputFormat/renderOperator), config.ValidateOutputFormat, and spine-review scan's own already-wired --output"
provides:
  - "cmd/engram/cmdwalk.go: operatorCommands() — the concrete structural predicate defining operator-tier membership (non-nil RunE, no client 'server' flag, minus a named {serve, version} exclusion set)"
  - "--output json|text with TTY auto-detection on all SIX pre-existing operator commands (reindex, prune-expired, summarize-missing, backfill-short-ids, migrate-remap-owner, migrate-set-owner), matching spine-review scan's contract"
  - "Pure text-formatter + json-doc-builder pairs for every operator command (reindexReportDoc, pruneReportDoc, summarizeReportDoc, backfillReportDoc, migrateRemapDoc, migrateSetOwnerReportDoc), each provably carrying every fact its text sentence states"
  - "TestTimeoutGroupMatrix: the published three-group --timeout matrix (reject-zero-client, zero-disables, reject-zero-operator) pinned behaviourally, with a live-tree set-equality gate"
  - "Operator-tier --output documentation in the CLI guide; the existing three-group --timeout table extended (not duplicated) with spine-review scan"
affects: [03-03, 03-04, 03-05, 03-06, 03-07]

# Actuals (#2632)
actuals:
  tokens: 17291
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Every operator command resolves --output through the SAME operatorOutputFormat/renderOperator pair from cmd/engram/operator_output.go — no command reaches outputFormatFromConfig directly (grep-gated: 0 hits outside operator_output.go/client_common.go)"
    - "Pure *Summary(...) string text formatters paired 1:1 with a pure *ReportDoc(...) json-doc builder, both driven from the SAME result value, so the phase's parity test can assert every text fact also appears as a json field"
    - "operatorCommands() as a concrete structural predicate (RunE != nil, no 'server' flag, minus a named exclusion set) replacing the previous informal 'whatever walkCommands classifies as operator' language"
    - "--dry-run vs applied distinction expressed as an explicit boolean plus SEPARATE count fields in json, never inferred from a '[dry-run]' text prefix"

key-files:
  created: []
  modified:
    - cmd/engram/reindex.go
    - cmd/engram/reindex_test.go
    - cmd/engram/prune.go
    - cmd/engram/prune_test.go
    - cmd/engram/migrate.go
    - cmd/engram/migrate_test.go
    - cmd/engram/summarize.go
    - cmd/engram/summarize_test.go
    - cmd/engram/backfill.go
    - cmd/engram/backfill_test.go
    - cmd/engram/cmdwalk.go
    - cmd/engram/cmdwalk_test.go
    - cmd/engram/operator_output_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - internal/config/client_validate_test.go
    - docs-site/src/content/docs/guides/cli.md

key-decisions:
  - "The catalog.golden-decode set-equality acceptance criterion (Task 2's acceptance list) and operatorCommands() itself both landed in the Task 3 commit, not split across Task 2/Task 3 file boundaries — operatorCommands() did not exist until Task 3, so a test asserting against it could not be authored earlier without forward-referencing an undefined symbol. The plan's per-task <files> lists reflect this: neither Task 2's nor Task 3's declared file set includes catalog_test.go, so the set-equality gate was added to cmdwalk_test.go (a Task 3 file) as TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs, satisfying the criterion's substance without inventing an undeclared file."
  - "operatorCommandExclusions is keyed by commandKey (not bare cobra Use), consistent with every other qualified-path lookup this phase introduced (03-01's commandKey/ClassForCommand convention) — serve/version are both top-level so the two forms coincide today, but this keeps the predicate correct if either is ever nested."
  - "version's Run (not RunE) already excludes it from operatorCommands() structurally; the named exclusion for 'version' is kept anyway so the exclusion set does not silently rely on that incidental fact surviving a future refactor to RunE."
  - "TestTimeoutGroupMatrix's zero-disables rows reuse the exact deadQdrant/deadServer + --timeout 2s-style unreachable-backend pattern exitcode_baseline_test.go already established, substituting --timeout 0 — proving 0 does NOT trip a usage error and instead reaches (and fails at) the same transport-unavailable path the existing baseline pins for a positive timeout."
  - "Filed GitHub issue #481 for the deferred typed-renderer refactor (Codex review #3's 'known limitation, recorded rather than redesigned') rather than attempting it in this plan — renderOperator(cmd, format, text string, doc any) still permits a json document structurally unrelated to its text sentence; TestOperatorOutputParity narrows this at test time but does not make it structurally unrepresentable."

requirements-completed: [REQ-operator-output-flag]

coverage:
  - id: D1
    description: "reindex and prune-expired accept --output json|text via the shared operator helpers, with --timeout left untouched"
    requirement: "REQ-operator-output-flag"
    verification:
      - kind: unit
        ref: "cmd/engram/reindex_test.go#TestReindexRejectsInvalidOutput"
        status: pass
      - kind: unit
        ref: "cmd/engram/prune_test.go#TestPruneOutputJSONHasBestEffortMarker"
        status: pass
    human_judgment: false
  - id: D2
    description: "migrate-set-owner, migrate-remap-owner, summarize-missing, and backfill-short-ids accept --output json|text; migrate-remap-owner's json carries the dry-run/applied distinction as separate fields"
    requirement: "REQ-operator-output-flag"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_test.go#TestMigrateRemapDryRunJSONDistinguishesPreviewFromApplied"
        status: pass
      - kind: unit
        ref: "cmd/engram/summarize_test.go#TestSummarizeReportDocSeparatesCounts"
        status: pass
      - kind: unit
        ref: "cmd/engram/backfill_test.go#TestBackfillRejectsInvalidOutput"
        status: pass
    human_judgment: false
  - id: D3
    description: "operatorCommands() is a concrete, both-directions-gated structural predicate for operator-tier membership; every text fact appears as a json field for every operator command; the published three-group --timeout matrix is pinned behaviourally and extended to spine-review scan"
    requirement: "REQ-operator-output-flag"
    verification:
      - kind: unit
        ref: "cmd/engram/cmdwalk_test.go#TestOperatorCommands"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestOperatorOutputParity"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestTimeoutGroupMatrix"
        status: pass
      - kind: unit
        ref: "cmd/engram/cmdwalk_test.go#TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs"
        status: pass
    human_judgment: false

duration: ~45min
completed: 2026-08-06
status: complete
---

# Phase 3 Plan 2: Operator-Tier `--output` Backfill Summary

**`--output json|text` backfilled onto all six pre-existing operator commands via the shared operator_output.go helpers, plus `operatorCommands()` as a concrete, gated structural predicate and a behaviourally-pinned three-group `--timeout` matrix.**

## Performance

- **Duration:** ~45 min
- **Completed:** 2026-08-06
- **Tasks:** 3
- **Files modified:** 17 (0 created, 17 modified)

## Accomplishments

- Backfilled `--output json|text` with TTY auto-detection onto `reindex` and `prune-expired` (Task 1), then `migrate-set-owner`, `migrate-remap-owner`, `summarize-missing`, and `backfill-short-ids` (Task 2) — every one of the six pre-existing operator commands now routes through the SAME `operatorOutputFormat`/`renderOperator` pair `spine-review scan` established in plan 03-01, with zero second validation or rendering path anywhere in the tier.
- Extracted a pure text-formatter + json-doc-builder pair per command (`pruneSummary`/`pruneReportDoc`, `migrateSetOwnerSummary`/`migrateSetOwnerReportDoc`, `migrateRemapSummary`/`migrateRemapDoc`, `backfillSummary`/`backfillReportDoc`, plus `reindexReportDoc` and `summarizeReportDoc` alongside the pre-existing `reindexSummary`/`summarizeSummary`), each driven from the SAME result value so every fact the text sentence states is provably present as a json field.
- `migrate-remap-owner`'s json document carries the dry-run vs applied distinction as an explicit boolean plus SEPARATE `would_remap`/`remapped` count fields — never inferred from the `[dry-run]` text prefix (T-03-10's mitigation).
- Defined `operatorCommands()` in `cmd/engram/cmdwalk.go`: the concrete structural predicate for operator-tier membership (non-nil `RunE`, no client `server` flag, minus a named `{serve, version}` exclusion set), replacing the previous informal "whatever `walkCommands` classifies as operator" language (review #19). Gated both directions by `TestOperatorCommands` and by a catalog.golden-decoded set-equality test.
- Added `TestOperatorOutputParity` (json/text field-for-fact parity across all seven operator-tier commands, row-set gated against `operatorCommands()`), `TestOperatorOutputEncoding` (non-ASCII + double-quote round trip), `TestOperatorOutputEmpty` (no `null` anywhere in an empty-result document), `TestOperatorOutputStream` (json on stdout only, warnings on stderr), and `TestEveryOperatorCommandRejectsInvalidOutput` (behavioural sweep proving `--output yaml` exits usage for every command in `operatorCommands()`).
- Rewrote the `--timeout` regression as `TestTimeoutGroupMatrix` against the ACTUAL published three-group matrix (`reject-zero-client` = search/list/store, `zero-disables` = reindex/prune-expired/summarize-missing/backfill-short-ids/spine-review scan, `reject-zero-operator` = migrate-remap-owner/migrate-set-owner), asserted BEHAVIOURALLY (against a dead Qdrant/server) rather than by help-text wording, with a live-tree set-equality gate over every `--timeout`-bearing command.
- Extended (not duplicated) the CLI guide's existing three-group `--timeout` table with `spine-review scan` in the `zero-disables` row, and documented the operator-tier `--output` contract under the existing "Output contract" heading.

## Task Commits

1. **Task 1: Wire the shared operator output helpers into `reindex` and `prune-expired`** — `53a07af4` (feat)
2. **Task 2: `--output` on the four remaining operator commands** — `56c38dcf` (feat)
3. **Task 3: Output-contract parity tests, the operator-membership predicate, the true three-group `--timeout` matrix, and the CLI guide section** — `0bf57515` (test)

**Plan metadata:** _(pending — final `docs(03-02)` commit follows this SUMMARY)_

## Files Created/Modified

- `cmd/engram/reindex.go` — `--output` flag, `reindexReportDoc`/builder, `renderOperator` call
- `cmd/engram/reindex_test.go` — invalid-output, json-doc-fact, text-mode-unchanged tests
- `cmd/engram/prune.go` — `--output` flag, extracted `pruneSummary`, `pruneReportDoc`/builder
- `cmd/engram/prune_test.go` — invalid-output, best-effort-marker, text-mode-unchanged tests
- `cmd/engram/migrate.go` — `--output` on both commands; `migrateSetOwnerSummary`/`ReportDoc`, `migrateRemapSummary`/`Doc` (dry-run vs applied as separate fields)
- `cmd/engram/migrate_test.go` — invalid-output, dry-run-vs-applied, summary-unchanged tests
- `cmd/engram/summarize.go` — `--output` flag, `summarizeReportDoc`/builder reusing the existing `summarizeSummary`
- `cmd/engram/summarize_test.go` — invalid-output, separate-counts tests
- `cmd/engram/backfill.go` — `--output` flag, extracted `backfillSummary`, `backfillReportDoc`/builder
- `cmd/engram/backfill_test.go` — invalid-output, summary-unchanged tests
- `cmd/engram/cmdwalk.go` — `operatorCommands()`, `operatorCommandExclusions`
- `cmd/engram/cmdwalk_test.go` — `TestOperatorCommands`, `TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs`
- `cmd/engram/operator_output_test.go` — `TestOperatorOutputFormat`, `TestRenderOperatorTextAndJSON` (Task 1), parity/encoding/empty/stream tests, `TestTimeoutGroupMatrix`, `TestEveryOperatorCommandRejectsInvalidOutput` (Task 3)
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` — regenerated (Tasks 1 and 2 only; no flag changes in Task 3)
- `internal/config/client_validate_test.go` — `TestValidateOutputFormat`
- `docs-site/src/content/docs/guides/cli.md` — operator-tier `--output` subsection; extended three-group `--timeout` table

## Decisions Made

See `key-decisions` in frontmatter.

## Deviations from Plan

None — plan executed as written, with one clarified ordering note (see key-decisions): the catalog.golden set-equality acceptance criterion listed under Task 2 was implemented as part of Task 3's commit (`TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs`, in `cmdwalk_test.go`), because it depends on `operatorCommands()`, which Task 3 defines. Neither task's declared `<files>` list included `catalog_test.go`, so the gate was added to a Task-3-declared file rather than to an undeclared one. No Rule 1/2/3 auto-fixes were needed beyond this; no architectural (Rule 4) questions arose.

## Required Evidence (per this plan's critical execution constraints)

### 1. `--timeout` divergence — three-group matrix, pinned behaviourally

`TestTimeoutGroupMatrix` (cmd/engram/operator_output_test.go) asserts, per group, that `--timeout 0` behaves as the CLI guide's published table states:

- `reject-zero-client` (search, list, store): `--timeout 0` → `exitUsage`.
- `zero-disables` (reindex, prune-expired, summarize-missing, backfill-short-ids, spine-review scan): `--timeout 0` → NOT `exitUsage` (proceeds to dial and fails `exitUnavailable` against a dead Qdrant/server).
- `reject-zero-operator` (migrate-remap-owner, migrate-set-owner): `--timeout 0` → `exitUsage`.

A second assertion checks the union of the three groups equals every `--timeout`-bearing command in the live cobra tree (both directions).

**MUTATION CHECK, not RED-first** (per this plan's critical constraint #3 — the test and the three groups did not exist before this task, so the mis-grouping failure state cannot arise naturally in task order): temporarily moved `migrate-remap-owner` from `reject-zero-operator` into `zero-disables`, ran the test, and observed:

```
operator_output_test.go:491: migrate-remap-owner --timeout 0: exitCodeFromError = 2 (exitUsage), want anything else (zero-disables group; err=--timeout must be greater than 0 -- a timeout of 0 is not treated as unbounded)
```

Reverted immediately after observing the failure; `diff` against the pre-mutation file confirmed byte-identical restoration.

### 2. Six operator commands, not five

All six live operator commands — `reindex`, `prune-expired`, `summarize-missing`, `backfill-short-ids`, `migrate-remap-owner`, and `migrate-set-owner` (deprecated but live, present in `root.Commands()` and both goldens) — now accept `--output`. Confirmed via `TestOperatorCommands` and `TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs`: `operatorCommands()` returns exactly `{reindex, prune-expired, summarize-missing, backfill-short-ids, migrate-remap-owner, migrate-set-owner, spine-review scan}` (7 — six operator commands plus plan 03-01's `spine-review scan`), and `catalog.golden`'s `--output`-bearing command set equals that set unioned with `{search, list, store}` (10 total).

### 3. Golden regeneration

Both goldens regenerated after Task 1 (reindex, prune-expired gain `--output`) and after Task 2 (the remaining four gain `--output`); Task 3 introduced no new flags, so no further regeneration was needed. `go test ./cmd/engram/... -run 'TestHelpGolden|TestCatalogGolden' -count=1` passes at every checkpoint.

### 4. Shuffle seeds

- Task 2's own verify command (`TestMigrate|TestSummarize|TestBackfill|TestHelpGolden|TestCatalogGolden`) passed under `-shuffle=1`, `-shuffle=2`, `-shuffle=3`, plus three full-package `go test ./cmd/engram/... -count=1 -shuffle=on` runs.
- Task 3's full-package run (`go test ./cmd/engram/... -count=1`) passed under `-shuffle=1`, `-shuffle=7`, `-shuffle=13`.

### 5. `--timeout` diff-scoped non-interference

`git diff -- cmd/engram/reindex.go cmd/engram/prune.go | rg -o '^[-+].*"timeout"' | wc -l` → `0` (Task 1).
`git diff -- cmd/engram/migrate.go | rg -o '^[-+].*must be > 0' | wc -l` → `0` (Task 3) — the migrate commands' zero-rejection contract is untouched.

### 6. `outputFormatFromConfig` direct-call gate

`rg -v '^\s*//' cmd/engram/spine_review_scan.go cmd/engram/reindex.go cmd/engram/prune.go cmd/engram/migrate.go cmd/engram/summarize.go cmd/engram/backfill.go | rg -o 'outputFormatFromConfig\(' | wc -l` → `0`. `spine_review_scan.go` already resolved `--output` through `operatorOutputFormat` from plan 03-01 — no retrofit needed. Backed behaviourally by `TestEveryOperatorCommandRejectsInvalidOutput`, which drives `--output yaml` against every command in `operatorCommands()` and asserts `exitUsage` for each.

### 7. `go clean -testcache && task`

Ran `go clean -testcache` then `task` (lint + `go test ./...`, including `internal/e2e` — shells out to the built binary — and `internal/store` — testcontainers-backed Qdrant): all green.

### 8. Key-links verification

`gsd-tools verify key-links .planning/phases/03-spine-curation-structural-cli/03-02-PLAN.md` → `{"all_verified": true, "verified": 3, "pending": 0, "total": 3}`.

### 9. Follow-up issue for the deferred typed-renderer refactor

Filed **GitHub issue #481** ("operator tier: typed per-result renderer to make json/text widening structurally unrepresentable"), recording Codex review #3's known limitation: `renderOperator(cmd, format, text string, doc any)` still permits a json document structurally unrelated to its text sentence. `TestOperatorOutputParity` narrows this risk at test time (every text fact must appear as a json field, gated against `operatorCommands()`) but does not make widening structurally unrepresentable — that redesign is deferred, not attempted, per this plan's explicit scope note.

## Known Stubs

None. Every json-doc builder is wired to the real result values from `internal/store`'s existing operator methods; no placeholder or hardcoded-empty rendering exists anywhere in this plan's diff.

## Threat Flags

None beyond what the plan's own `<threat_model>` already anticipated (T-03-09, T-03-10, T-03-11) — all three are mitigated as designed:

- T-03-09 (information disclosure via json widening): every operator command's json struct is hand-declared and driven from the same result value as its text sentence; `TestOperatorOutputParity` asserts field-for-fact equivalence.
- T-03-10 (tampering of a downstream automated decision via `migrate-remap-owner --dry-run`): the dry-run/applied distinction is an explicit boolean plus separate count fields, proven by `TestMigrateRemapDryRunJSONDistinguishesPreviewFromApplied`.
- T-03-11 (input validation on `--output`): one exported `config.ValidateOutputFormat` shared by both tiers; `TestEveryOperatorCommandRejectsInvalidOutput` proves no second, unvalidated rejection site exists.

## Issues Encountered

None beyond the task-boundary clarification recorded in § Deviations from Plan.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All six pre-existing operator commands plus `spine-review scan` now share one `--output` validation path, one renderer, and a concrete, gated definition of operator-tier membership (`operatorCommands()`) — plans 03-03 through 03-07 can extend the tier (new `spine-review` leaves) by adding a row to `TestOperatorOutputParity` and assigning a `--timeout` group, both enforced by existing gates rather than by convention.
- GitHub issue #481 tracks the deferred typed-renderer refactor as a structural-hardening follow-up, not a blocker.
- No blockers for plan 03-03.

## Self-Check: PASSED

All key files (cmd/engram/reindex.go, prune.go, migrate.go, summarize.go, backfill.go, cmdwalk.go, operator_output_test.go, docs-site/.../cli.md) confirmed present on disk. All three task commit hashes (53a07af4, 56c38dcf, 0bf57515) confirmed present in `git log --oneline --all`.

---
*Phase: 03-spine-curation-structural-cli*
*Completed: 2026-08-06*
