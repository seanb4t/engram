---
phase: 05-operator-correctness
plan: 05
subsystem: docs
tags: [cli, docs-gate, cobra, operator-commands]

# Dependency graph
requires:
  - phase: 05-operator-correctness
    provides: consolidate_docs_test.go's cliGuideRelPath/nextHeadingPattern idiom, operator_output_test.go's operatorCommands()-derived gate pattern
provides:
  - "guides/cli.md §Operator commands list naming migrate, migrate status, migrate revert (linked to /guides/migrate/) and setup"
  - "TestCLIGuideOperatorCommandsListsEveryOperatorCommand, a docs gate deriving the required command set from operatorCommands() instead of a hand-typed list"
affects: [docs-site, cmd/engram]

# Actuals (#2632)
actuals:
  tokens: 2700
  tasks: 1
  commits: 1

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Docs-gate pattern: extract a markdown section with a pure function (extractOperatorCommandsList), derive the required set from live code (operatorCommands()), then diff with a second pure function (missingOperatorCommandMentions) — mirrors consolidate_docs_test.go's extractConsolidateSection idiom."

key-files:
  created: []
  modified:
    - cmd/engram/operator_output_test.go
    - docs-site/src/content/docs/guides/cli.md

key-decisions:
  - "D-06 implemented: migrate, migrate status, migrate revert added right after backfill-short-ids, with a parenthetical link to /guides/migrate/, consistent with how guides/migrate.md names them."
  - "setup added after the migrate-set-owner alias clause — the gate's own finding (operatorCommands() returns it), not something #503 named; an exclusion for it would have made the gate assert something false since setup does register the operator --output flag."

patterns-established: []

requirements-completed: [OPS-05]

coverage:
  - id: D1
    description: "guides/cli.md §Operator commands list names migrate, migrate status, migrate revert, and setup, linked to /guides/migrate/"
    requirement: OPS-05
    verification:
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestCLIGuideOperatorCommandsListsEveryOperatorCommand"
        status: pass
    human_judgment: false
  - id: D2
    description: "Docs gate derives the required command set from the live operatorCommands() cobra tree, never a hand-typed list, so a future operator command cannot be left off the guide unnoticed"
    requirement: OPS-05
    verification:
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestCLIGuideOperatorCommandsListsEveryOperatorCommand"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-09-24
status: complete
---

# Phase 5 Plan 5: Operator-Command Docs Gate Summary

**Added a docs gate (`TestCLIGuideOperatorCommandsListsEveryOperatorCommand`) deriving the required `--output`-accepting command set from `operatorCommands()`, then fixed `guides/cli.md` to name `migrate`, `migrate status`, `migrate revert`, and `setup` (#503, D-06).**

## Performance

- **Duration:** 20 min
- **Completed:** 2026-09-24T17:02:58Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- `cmd/engram/operator_output_test.go` gained two pure helpers (`extractOperatorCommandsList`, `missingOperatorCommandMentions`) and `TestCLIGuideOperatorCommandsListsEveryOperatorCommand`, which reads the live `operatorCommands()` tree and fails if `guides/cli.md`'s §Operator commands prose omits any command name.
- `guides/cli.md`'s §Operator commands list now names `migrate`, `migrate status`, and `migrate revert` (with a link to `/guides/migrate/`) and `setup`, alongside the previously-listed commands. Only the list paragraph changed — the table and every anchored rule region are untouched.

## Task Commits

1. **Task 1: Derived docs gate for the operator-command list, then list migrate, migrate status, migrate revert and setup (D-06, #503)** - `102c87f0` (docs)

_Note: single commit — the plan's rule `3p0zsqrhmb` calls for no committed positive-control subtest; RED was observed pre-commit and is quoted below, not committed as a separate test-only commit._

## Files Created/Modified
- `cmd/engram/operator_output_test.go` - Added the docs gate (`TestCLIGuideOperatorCommandsListsEveryOperatorCommand`) and its two pure helpers, plus `os`/`regexp` imports
- `docs-site/src/content/docs/guides/cli.md` - §Operator commands list paragraph rewritten to name `migrate`, `migrate status`, `migrate revert` (linked to `/guides/migrate/`) and `setup`

## RED evidence (rule `3p0zsqrhmb` — no committed positive control; quoted here instead)

Gate run against the pre-edit guide:

```
operator_output_test.go:860: ../../docs-site/src/content/docs/guides/cli.md "### Operator commands" list is missing: [migrate migrate revert migrate status setup] (add each as a backticked name)
--- FAIL: TestCLIGuideOperatorCommandsListsEveryOperatorCommand (0.00s)
```

Named exactly the four keys the plan predicted (`migrate`, `migrate revert`, `migrate status`, `setup`) — the planning-time command set was unchanged.

## Decisions Made
- D-06: list the three `migrate` commands consistent with `guides/migrate`'s own naming, and add a docs gate since none existed for this list.
- `setup` was added as the gate's own finding (not named by #503) — an exclusion for it in the gate would assert something false, since `operatorCommands()` returns it and it does register the operator `--output` flag via `addOperatorOutputFlag`.

## Deviations from Plan

None - plan executed exactly as written. One adjustment during acceptance-criteria verification: the initial `t.Fatal` message for the zero-applicability guard repeated the literal string `operatorCommands()`, which inflated the acceptance criterion's grep count from the required 1 to 2; reworded the message to describe the empty tree without repeating the call literal. This is a wording-only fix within Task 1, not a Rule 1-4 deviation (no behavior change, caught by the plan's own acceptance-criteria gate before commit).

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 5 (Operator Correctness) plan 05-05 is the last plan in the phase's plan list; `guides/cli.md`'s operator-command list is now mechanically gated against the live cobra tree.
- `go test ./cmd/engram/ -count=1`, `go test ./internal/surfaces/ -count=1`, `go test ./internal/keylinks/`, `golangci-lint run ./cmd/engram/...`, and `gofmt -l cmd/engram/operator_output_test.go` all pass/clean.
- `rumdl check docs-site/src/content/docs/guides/cli.md` reports the file is excluded by `.rumdl.toml` patterns (pre-existing exclude, not introduced by this plan) — no findings to report either way.

---
*Phase: 05-operator-correctness*
*Completed: 2026-09-24*

## Self-Check: PASSED

- FOUND: `cmd/engram/operator_output_test.go`
- FOUND: `docs-site/src/content/docs/guides/cli.md`
- FOUND: `.planning/phases/05-operator-correctness/05-05-SUMMARY.md`
- FOUND commit `102c87f0` in `git log --oneline --all`
