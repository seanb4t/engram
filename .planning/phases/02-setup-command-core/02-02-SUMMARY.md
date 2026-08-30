---
phase: 02-setup-command-core
plan: 02
subsystem: cli
tags: [exit-codes, cobra, cli, setup, catalog]

requires:
  - phase: 02-setup-command-core
    plan: 01
    provides: "cmd/engram/setup.go's preview/apply skeleton, internal/setup's Runtime/Plan/Outcome/Result types, and the setupBuildRows/setupPlanDoc report-building pipeline this plan wires classification into"
provides:
  - "internal/setup.Classify: pure ExitClass classification (ExitTotalSuccess/ExitPartial/ExitTotalFailure) over a []Result, exhaustively tested"
  - "cmd/engram exit codes 8 (exitPartial) and 9 (exitSetupFailed), published, allowlisted, and golden-pinned across all five taxonomy sites"
  - "engram setup --apply renders every runtime's own outcome then exits via setup.Classify (setupExitCode mapping)"
affects: [phase-3-runtime-registration]

actuals:
  tokens: 7400
  tasks: 3
  commits: 5

tech-stack:
  added: []
  patterns:
    - "internal/setup.Classify mirrors verifyFailOnErr's shape: a pure classification function over an already-built report, returning a typed decision the caller maps to a process exit code — no I/O, no cmd/engram vocabulary, leaf-purity preserved"
    - "Render-then-classify sequencing in setupApplyRun: renderOperator runs unconditionally before any error is returned, so a nonzero exit never erases the per-runtime record (T-02-06)"
    - "Exhaustiveness pinned by a generated cross-product (every Outcome tuple up to length 3) with the expectation derived from three stated predicates, not transcribed — resists silent drift if a sixth Outcome is ever added"

key-files:
  created:
    - internal/setup/exit.go
    - internal/setup/exit_test.go
  modified:
    - cmd/engram/client_common.go
    - cmd/engram/catalog.go
    - cmd/engram/catalog_test.go
    - cmd/engram/exitcode_baseline_test.go
    - cmd/engram/testdata/catalog.golden
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/operator_view_setup_test.go

key-decisions:
  - "Every present (attempted) runtime under --apply is marked OutcomeFailed with a Reason naming setup.ErrApplyNotImplemented, regardless of whether Plan() itself would have succeeded — Apply() genuinely does not exist until Phase 3, so distinguishing 'would have succeeded' from 'would have failed on an unsupported auth mode' this phase would assert a capability the binary does not have."
  - "exitPartial (8) has no reachable producer through engram setup this phase: since every attempted runtime fails identically under the Apply stub, only ExitTotalSuccess (nothing attempted) and ExitTotalFailure (something attempted) are reachable through the CLI. The full 3-way table is proven instead by exit.go's pure, exhaustive unit tests — recorded in nonConnectProducedCodes and in setupApplyRun's own doc comment so a later reader does not read the gap as an oversight."

requirements-completed:
  - REQ-setup-partial-failure-legible

coverage:
  - id: D1
    description: "exitPartial=8 and exitSetupFailed=9 published, allowlisted, and golden-pinned across all five taxonomy sites (client_common.go const block, catalog.go doc.ExitCodes, catalog_test.go's wantExitCodes/nonConnectProducedCodes, exitcode_baseline_test.go, testdata/catalog.golden)"
    requirement: "REQ-setup-partial-failure-legible"
    verification:
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogListsEveryExitCode"
        status: pass
      - kind: unit
        ref: "cmd/engram/catalog_test.go#TestCatalogExitCodesMatchMapper"
        status: pass
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaselineClaims"
        status: pass
      - kind: unit
        ref: "cmd/engram/exitcode_baseline_test.go#TestExitCodeBaselineRowCount"
        status: pass
      - kind: unit
        ref: "cmd/engram/golden_test.go#TestCatalogGolden"
        status: pass
    human_judgment: false
  - id: D2
    description: "internal/setup.Classify is a pure, exhaustively-tested function classifying a []Result into ExitTotalSuccess/ExitPartial/ExitTotalFailure, with not-present runtimes never counted as an attempt (D-07) and a zero-valued Outcome treated as a failure rather than laundered into success"
    requirement: "REQ-setup-partial-failure-legible"
    verification:
      - kind: unit
        ref: "internal/setup/exit_test.go#TestClassifySingleOutcome"
        status: pass
      - kind: unit
        ref: "internal/setup/exit_test.go#TestClassifyBoundaryCases"
        status: pass
      - kind: unit
        ref: "internal/setup/exit_test.go#TestClassifyMixedCases"
        status: pass
      - kind: unit
        ref: "internal/setup/exit_test.go#TestClassifyRejectsZeroValueOutcome"
        status: pass
      - kind: unit
        ref: "internal/setup/exit_test.go#TestClassifyExhaustiveOutcomeCombinations"
        status: pass
    human_judgment: false
  - id: D3
    description: "engram setup --apply renders every selected runtime's own outcome via renderOperator BEFORE returning any error, then exits with a status (setupExitCode) that distinguishes total success from total failure this phase; setupPreview is untouched and still exits 0 for everything except a usage/config error (D-08)"
    requirement: "REQ-setup-partial-failure-legible"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupApplyAllAbsentExitsZero"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupApplyAtLeastOnePresentExitsSetupFailed"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupPreviewExitsZeroRegardlessOfPresence"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupApplyJSONEmitsPerRuntimeOutcome"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupExitCodes"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_view_setup_test.go#TestSetupViewIdentity"
        status: pass
    human_judgment: false
  - id: D4
    description: "Real-machine spot check: engram setup's reported presence of claude-code/codex/opencode matches reality, and engram setup --apply against real binaries produces the expected exit 9 with a full report"
    verification: []
    human_judgment: true
    rationale: "The plan's Task 3 <verify> designates this a <human-check> explicitly: asserting against actually-installed third-party binaries in the automated suite would violate rule m45p2b4bp7 (never test or red-gate third-party behavior). I ran `engram setup` and `engram setup --apply` on this dev machine as a sanity spot-check during execution (see Issues Encountered) and both matched expectations, but that is a developer's ad hoc check, not the designated human sign-off this item calls for."

duration: ~20 min
completed: 2026-08-30
status: complete
---

# Phase 2 Plan 2: Exit-Status Taxonomy Summary

`engram setup` now publishes a three-way exit-status taxonomy — 0 (total success), 8 (`exitPartial`), 9 (`exitSetupFailed`) — backed by `internal/setup.Classify`, a pure function proven over a generated, predicate-derived exhaustive table, with `--apply` rendering every runtime's own outcome before returning a typed exit status.

## Performance

- **Duration:** ~20 min
- **Tasks:** 3/3 completed
- **Files modified:** 2 created, 8 modified (10 total)

## Accomplishments

- Exit codes 8 (`exitPartial`) and 9 (`exitSetupFailed`) declared in `client_common.go`'s const block, advertised in `catalog.go`'s `doc.ExitCodes`, allowlisted in `catalog_test.go`'s `nonConnectProducedCodes`, and golden-pinned via `task surfaces:gen` — all five taxonomy sites moved in the single Task 1 commit the plan required.
- `internal/setup.Classify(results []Result) ExitClass`: a pure, stdlib-only function (the package's leaf-purity gate still passes) classifying total success / partial / total failure, with `OutcomeNotPresent` never counted as an attempt (D-07) and a zero-valued `Outcome` explicitly treated as a failure rather than silently laundered into success. Pinned by a generated cross-product enumerating every tuple of the five `Outcome` values up to length 3, with the expected class derived from the three stated predicates rather than transcribed.
- `engram setup --apply` now renders its report via `renderOperator` UNCONDITIONALLY before computing `setup.Classify` and returning any error, so a nonzero exit can never erase the per-runtime record of what happened (T-02-06). Every attempted (present) runtime is marked `failed` with a reason naming `setup.ErrApplyNotImplemented`; a not-present runtime keeps its untouched `not-present` row.
- `setupExitCode(setup.ExitClass) int` is the three-arm mapping (`ExitTotalSuccess`→0, `ExitPartial`→8, `ExitTotalFailure`→9) with a documented, unreachable-while-`ExitClass`-has-three-values default arm.
- Confirmed live on this dev machine: bare `engram setup` reports all three runtimes present with their exact invocations; `engram setup --apply` exits 9 with the full three-row report still on stdout.

## Task Commits

Each task was implemented, tested, and verified individually against its own `<verify>` and `<acceptance_criteria>` before moving to the next.

1. **Task 1: Publish exitPartial=8 and exitSetupFailed=9 across all five taxonomy sites, in one commit** — `c676282d` (feat)
2. **Task 2: `internal/setup.Classify`** — TDD RED/GREEN:
   - `5d4dfa8e` (test) — failing tests for `Classify`/`ExitClass`
   - `a9d98fe2` (feat) — pure implementation, all tests green
3. **Task 3: Wire the classification into `engram setup`** — TDD RED/GREEN:
   - `03fee8e3` (test) — failing tests for the apply-lane exit behavior + `setupExitCode`
   - `19ce38aa` (feat) — `setupExitCode`, `setupResultsFromRows`, `setupApplySummary`, and the rewritten `setupApplyRun`

**Plan metadata:** committed separately immediately after this SUMMARY (worktree mode — STATE.md and ROADMAP.md are the orchestrator's own job, excluded here).

## Files Created/Modified

- `internal/setup/exit.go` — `ExitClass`, `ExitTotalSuccess`/`ExitPartial`/`ExitTotalFailure`, `Classify`
- `internal/setup/exit_test.go` — single-outcome, boundary, mixed, zero-value-guard, and generated-cross-product exhaustiveness tests
- `cmd/engram/client_common.go` — `exitPartial = 8`, `exitSetupFailed = 9` const additions
- `cmd/engram/catalog.go` — two new `doc.ExitCodes` entries
- `cmd/engram/catalog_test.go` — `wantExitCodes`/`nonConnectProducedCodes` extended
- `cmd/engram/exitcode_baseline_test.go` — `setup/bad-auth` introduced row, `wantRows` 38→39
- `cmd/engram/testdata/catalog.golden` — regenerated via `task surfaces:gen`
- `cmd/engram/setup.go` — `setupExitCode`, `setupApplyStubReason`, `setupResultsFromRows`, `setupApplySummary`, rewritten `setupApplyRun`
- `cmd/engram/setup_test.go` — apply-lane exit-code tests, preview-still-exits-0 tests, `setupExitCode` table test
- `cmd/engram/operator_view_setup_test.go` — added an apply-shaped mixed not-present/failed fixture

## Decisions Made

See `key-decisions` in frontmatter. Both were pre-scoped by `02-CONTEXT.md` (D-06, D-07, D-09) and `02-02-PLAN.md`'s own action text; the one discretionary call was exactly which reason string the apply-stub row carries and how `setupApplyRun`'s error message is worded, left to planner/executor discretion by the plan.

## Deviations from Plan

None — plan executed exactly as written. The test function names in `setup_test.go`/`exit_test.go` are the executor's own choice (the plan's `<behavior>` blocks describe cases, not literal test names), which is ordinary implementation latitude, not a deviation.

## Issues Encountered

- `go test ./... -count=1` intermittently failed across ~20 `internal/store` tests with `http2: frame too large` errors from a testcontainers-managed Qdrant instance (`CollectionExists() failed: ... connection error`). This is a transient Docker/testcontainers resource-contention issue on this machine, unrelated to any file this plan touched (`internal/store` is out of this plan's scope). Re-running `go test ./internal/store/...` in isolation, and then the full `go test ./... -count=1` again, both passed cleanly on retry — confirmed not caused by this plan's changes.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

Phase 3 (Runtime Registration) can proceed: `internal/setup.Classify` is stable and exhaustively proven, and `cmd/engram/setup.go`'s render-then-classify sequencing is in place and ready for Phase 3's real `Apply()` to populate genuine `already-correct`/`wrote` outcomes — at which point `exitPartial` (currently unreachable through the CLI, proven only by `exit.go`'s pure unit tests) gains its first live producer. No changes to `internal/setup.Result`, `Outcome`, or `Classify`'s signature are anticipated; Phase 3 replaces `setupApplyRun`'s stub-marking loop with real per-runtime `Apply()` calls feeding the same `setup.Result` shape.

## Self-Check: PASSED

- `internal/setup/exit.go` exists: FOUND
- `internal/setup/exit_test.go` exists: FOUND
- Commit `c676282d` found in `git log --oneline --all`
- Commit `5d4dfa8e` found in `git log --oneline --all`
- Commit `a9d98fe2` found in `git log --oneline --all`
- Commit `03fee8e3` found in `git log --oneline --all`
- Commit `19ce38aa` found in `git log --oneline --all`
- All acceptance criteria for Tasks 1–3 re-verified: PASS
- Plan-level `<verification>`: `go test ./... -count=1` exits 0 (PASS, after confirming the transient `internal/store` flake above was not caused by this plan); `task` (lint + test) exits 0 (PASS); `task license:check` exits 0 (PASS); `task surfaces:gen` leaves the working tree clean (PASS); bare `engram` invocation advertises exit codes 0–9 with no gaps and no duplicates (PASS, confirmed via live binary output)

---
*Phase: 02-setup-command-core*
*Completed: 2026-08-30*
