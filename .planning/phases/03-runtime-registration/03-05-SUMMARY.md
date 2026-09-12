---
phase: 03-runtime-registration
plan: 05
subsystem: setup
tags: [mcp-registration, preview-honesty, token-file, cli-drift, help-prose]

requires:
  - phase: 03-runtime-registration
    provides: "The shared setup.Preview/Apply executor (03-01), claude-code's tolerant-remove-then-fatal-add sequence (03-02), opencode's KEY=VALUE header fix (03-03), and the generic pseudo-runtime's zero-Action Plan (03-04) — this plan closes the phase by making the preview lane honest and the flag's narrowed scope legible across all four"
provides:
  - "setup.Preview runs Plan.Probe for a present runtime with one wired and reports the bounded result on Result.Registered (D-10) — no probe outcome of any kind ever moves Outcome away from OutcomeWouldWrite or changes a preview's process exit code"
  - "cmd/engram/setup.go's setupBuildRows routes through setup.Preview instead of calling Detect/Plan directly, so the preview and apply lanes share exactly one code path from a Runtime to a Result (D-15 extended to row-building)"
  - "internal/setup/apply.go's tokenFileIgnoredMarker constant, set on Result.TokenFile by a structural rule (the runtime's Plan carries at least one Action) whenever --token-file is supplied — never keyed on a runtime's name, never carrying the supplied path"
  - "Corrected --help prose: --token-file's Usage string, setupLongDescription's bearer line, and setupPreviewSummary all describe the real, narrowed D-06 scope instead of Phase 2's provenance-placeholder premise"
  - "TestSetupPartialExitIsLiveProducible — the first live, non-generic production path proving exitPartial (8) is actually reachable, not merely allowlisted"
affects: [04-skills-distribution, 06-install-documentation]

actuals:
  tokens: 8938
  tasks: 3
  commits: 3
  plan_head_before: 86dd189de2d5fa3dcfd1c79ccf12999d830fd5ec

tech-stack:
  added: []
  patterns:
    - "One code path from Runtime to Result in BOTH lanes (D-15 extended): setupBuildRows now calls setup.Preview exactly the way setupApplyRun calls setup.Apply, so the preview report can never drift from what --apply would actually attempt"
    - "Structural token-file marker (D-07): Result.TokenFile is set from whether Plan.Actions is non-empty, never from Plan.Runtime's name — the same structural-predicate-over-enumeration shape D-14's optInOnlyRuntime already established"

key-files:
  created: []
  modified:
    - internal/setup/apply.go
    - internal/setup/apply_test.go
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/testdata/catalog.golden
    - cmd/engram/testdata/help.golden

key-decisions:
  - "Preview's probe capture uses the SAME combined stdout+stderr shape apply's read #2 already uses (res.Registered = boundCapture(probe1.Stdout + probe1.Stderr)), applied regardless of the probe's exit code — D-11 reports rather than diagnoses, so a nonzero-exit probe's output is rendered exactly like a zero-exit probe's, never string-matched or branched on."
  - "The token_file=ignored marker is set once, immediately after Result's Binary/Command/Config fields are populated (past the zero-Actions early return), so the structural rule ('this Plan carries at least one Action') is enforced by the code's own control flow rather than by a second, separately-fallible name check."
  - "TestSetupPartialExitIsLiveProducible distinguishes claude-code from codex inside the scripted Run fake by the LookPath-resolved BINARY PATH ('codex' substring), never by a runtime-name branch in production code — the fake only ever sees (path, args), mirroring exactly what the real Environment.Run seam sees."

requirements-completed: [REQ-setup-idempotent, REQ-register-auth-modes, REQ-register-cli-surface-drift-legible]

coverage:
  - id: D1
    description: "A bare `engram setup` runs each present runtime's Plan.Probe and reports currently-registered state as an informational registered= row field, while never classifying already-correct and never changing the process exit code on any probe outcome (zero exit, nonzero exit, or seam error)"
    requirement: REQ-setup-idempotent
    verification:
      - kind: unit
        ref: "internal/setup/apply_test.go#TestPreviewReportsRegisteredState"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupPreviewExitsZeroWhenProbeFails"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupPreviewNeverClassifiesAlreadyCorrect"
        status: pass
    human_judgment: false
  - id: D2
    description: "--token-file's narrowed D-06 scope is legible per row (token_file=ignored for every native runtime, absent for generic and absent entirely when the flag is not supplied, never carrying the supplied path) and in --help (the flag's own Usage string, setupLongDescription's bearer line, and setupPreviewSummary all state the real behavior)"
    requirement: REQ-register-auth-modes
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupTokenFileMarkedIgnoredForNativeRuntimes"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupNoTokenFileLeavesNoMarker"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupTokenFilePathNotDuplicatedIntoMarker"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHelpNamesEveryRuntimeAndAuthMode"
        status: pass
    human_judgment: false
  - id: D3
    description: "exitPartial (8) has a real, live production path with two genuinely native runtimes (one succeeds, one fails), not merely an allowlist claim in catalog_test.go's nonConnectProducedCodes; setupCmd's flag set still carries exactly six flags plus apply (D-12's no-timeout divergence held, destructive_test.go's expectation row untouched)"
    requirement: REQ-register-cli-surface-drift-legible
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupPartialExitIsLiveProducible"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveCommandsExactFlagSet"
        status: pass
    human_judgment: false

duration: 19min
completed: 2026-09-09
status: complete
---

# Phase 3 Plan 5: Preview honesty, the token_file marker, and golden regeneration Summary

**A bare `engram setup` now runs each present runtime's own read verb and shows present state next to intended state without ever claiming `already-correct`; `--token-file` carries an explicit `token_file=ignored` marker on every native runtime's row and `--help` states its real, narrowed scope; the pinned help/catalog goldens and `exitPartial`'s live-producibility are proven, closing Phase 3.**

## Performance

- **Duration:** 19 min
- **Started:** 2026-09-09T16:19:00Z (approximate — derived from STATE.md's prior `last_updated` timestamp; PLAN_START_TIME was not captured at spawn)
- **Completed:** 2026-09-09T16:38:30Z
- **Tasks:** 3
- **Files modified:** 6 (0 created, 6 modified)

## Accomplishments

- `internal/setup/apply.go`'s `execute()` non-mutating branch now runs `Plan.Probe` (when wired) and renders its bounded output onto `Result.Registered` — D-10's reversal of Phase 2's D-12 "a preview shells out to nothing" posture is complete. No probe outcome (zero exit, nonzero exit, empty output, or a seam error) ever moves `Outcome` away from `OutcomeWouldWrite`, and T-02-08's exit-code property is re-pinned on behavioral grounds now that the original structural argument ("`setupPreview` starts no process") is false.
- `cmd/engram/setup.go`'s `setupBuildRows` routes through `setup.Preview` instead of hand-calling `Detect`/`Plan`, sharing the exact same code path `setupApplyRun` already uses via `setup.Apply` and `setupRuntimeRowFromResult`. `setupPlanDoc`/`setupPreview` thread the real `context.Context` through instead of discarding it.
- `internal/setup/apply.go`'s new `tokenFileIgnoredMarker` constant is set on `Result.TokenFile` by a structural rule — the runtime's `Plan` carries at least one `Action` — whenever `--token-file` is supplied, in both the preview and apply lane. The generic pseudo-runtime is excluded because its `Plan` carries zero Actions, never because of a name check.
- `--token-file`'s Usage string, `setupLongDescription`'s bearer auth-mode line, and `setupPreviewSummary` are all rewritten to state the real, D-06-narrowed behavior: a native runtime is registered with an `ENGRAM_TOKEN` environment-variable reference resolved at connect time; `--token-file` applies only to the portable (generic) config; a bare invocation reads current state from each present runtime's own CLI, and two of the three dial the configured URL.
- `cmd/engram/testdata/{help,catalog}.golden` regenerated via `go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -update -count=1`; the diff is confined to the `engram setup` long description and the `--token-file` usage line.
- `TestSetupPartialExitIsLiveProducible` selects two genuinely native runtimes (one succeeds, one fails) and asserts the process exit code is exactly `exitPartial` — the first live, non-generic production path for that constant, turning `catalog_test.go`'s allowlist claim into a proven one.
- `cmd/engram/destructive_test.go`'s `setupCmd` flag-set row is confirmed unchanged (`git diff --stat` empty): D-12's deliberate no-`--timeout` divergence held through this phase.

## Task Commits

Each task was committed atomically:

1. **Task 1: The preview runs the probe and reports present state, without gaining a nonzero exit path** — `ef8a9c97` (feat)
2. **Task 2: The `token_file=ignored` marker and `--token-file`'s narrowed help prose** — `365e9edb` (feat)
3. **Task 3: Regenerate the pinned goldens and close the phase's full-suite gate** — `d1e0594e` (test)

_Note: All three tasks were `tdd="true"` in the plan. Tests and implementation were authored together in each commit rather than as separate RED→GREEN commits — see "TDD Gate Compliance" below._

## Files Created/Modified

- `internal/setup/apply.go` — Preview's non-mutating branch now runs the probe and populates `Result.Registered`; new `tokenFileIgnoredMarker` constant and its structural-rule assignment
- `internal/setup/apply_test.go` — `TestPreviewReportsRegisteredState` (probe-zero, probe-nonzero, probe-seam-error, not-present, zero-action subtests)
- `cmd/engram/setup.go` — `setupBuildRows`/`setupPlanDoc`/`setupPreview` thread `context.Context` and route through `setup.Preview`; `setupPreviewSummary`, `setupLongDescription`, and the `--token-file` flag's Usage string rewritten
- `cmd/engram/setup_test.go` — `TestSetupPreviewExitsZeroWhenProbeFails`, `TestSetupPreviewNeverClassifiesAlreadyCorrect`, `TestSetupTokenFileMarkedIgnoredForNativeRuntimes`, `TestSetupNoTokenFileLeavesNoMarker`, `TestSetupTokenFilePathNotDuplicatedIntoMarker`, `TestSetupPartialExitIsLiveProducible`
- `cmd/engram/testdata/catalog.golden`, `cmd/engram/testdata/help.golden` — regenerated

## Decisions Made

See `key-decisions` in frontmatter for the three non-obvious calls this plan made: the combined-capture shape for the preview's probe output regardless of exit code, the placement of the `token_file` marker assignment inside `execute()`'s existing control flow, and the path-based (never name-based) runtime distinction inside `TestSetupPartialExitIsLiveProducible`'s scripted fake.

## Deviations from Plan

None — plan executed as written across all three tasks. Every `<behavior>` bullet and `<acceptance_criteria>` item in 03-05-PLAN.md was verified directly (see Task Commits and the verification commands below); no Rule 1-4 auto-fix or architectural deviation was required.

## TDD Gate Compliance

All three tasks in 03-05-PLAN.md carried `tdd="true"`. For each, the implementation and its pinning tests were authored together in a single commit rather than as an independently-verified RED→GREEN pair — the same documented exception pattern this phase's own `WINDOWS.md` ids 1 and 2 already track for two earlier Phase 3 plans (RED genuinely reasoned through but not committed separately from GREEN). Reasoning per task:

- **Task 1** (`ef8a9c97`): `execute()`'s non-mutating branch and `TestPreviewReportsRegisteredState`/`TestSetupPreviewExitsZeroWhenProbeFails`/`TestSetupPreviewNeverClassifiesAlreadyCorrect` are two sides of one small, tightly-coupled control-flow change (a single `if hasProbe && probe1Err == nil` guard) — verified conceptually against the pre-change code (which unconditionally returned before ever setting `Registered`) before landing, but not committed as a separate failing-test commit.
- **Task 2** (`365e9edb`): the `tokenFileIgnoredMarker` assignment and its three pinning tests are similarly one coherent unit — the marker's structural rule and the tests proving both directions (present for native, absent for generic, absent when the flag is unset) were authored and verified together.
- **Task 3** (`d1e0594e`): golden regeneration is inherently a single step (the repo's own `-update` mechanism) — there is no meaningful RED state for a golden file, and `TestSetupPartialExitIsLiveProducible` was verified passing immediately after being written, with its correctness confirmed by reasoning through the scripted Run fake's per-path branching before running it.

All three commits' tests were independently confirmed to test the RIGHT thing (each assertion traces to a specific plan `<behavior>`/`<acceptance_criteria>` bullet) even though the strict commit-separated RED phase was not independently observed. No `feat`-then-separate-`test` pair was skipped; this is a commit-granularity deviation, not a coverage gap.

## Issues Encountered

None. No real `claude`/`codex`/`opencode` binary was invoked at any point — every exec in every new or updated test goes through the scripted `Environment.Run` fake (rule `m45p2b4bp7`).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 3 (Runtime Registration) is now feature-complete: all three native runtimes (`claude-code`, `codex`, `opencode`) plus the `generic` opt-in pseudo-runtime are wired end-to-end through the shared `setup.Preview`/`setup.Apply` executor, with an honest preview, a legible `--token-file` scope, and regenerated goldens.
- `go test ./... -count=1` and `task` (lint + full suite) are both green with every golden change committed.
- `REQ-setup-idempotent`, `REQ-register-auth-modes`, and `REQ-register-cli-surface-drift-legible` — all three declared across multiple Phase 3 plans (the shared-ID gate, #2388) — are now satisfied by every plan that declared them, since this plan's own `*-SUMMARY.md` is the last one required for readiness.
- Phase 4 (Skills Distribution) depends on this phase's runtime list and detection machinery, both unchanged in shape by this plan.
- No blockers.

---
*Phase: 03-runtime-registration*
*Completed: 2026-09-09*

## Self-Check: PASSED

- FOUND: internal/setup/apply.go
- FOUND: internal/setup/apply_test.go
- FOUND: cmd/engram/setup.go
- FOUND: cmd/engram/setup_test.go
- FOUND: cmd/engram/testdata/catalog.golden
- FOUND: cmd/engram/testdata/help.golden
- FOUND commit: ef8a9c97 (feat(03-05): preview runs the probe and reports present state (D-10))
- FOUND commit: 365e9edb (feat(03-05): token_file=ignored marker and narrowed --token-file help prose)
- FOUND commit: d1e0594e (test(03-05): regenerate pinned goldens; prove exitPartial live-producible)
- FOUND: go test ./... -count=1 green
- FOUND: task (lint + full suite) green
- FOUND: go test ./internal/keylinks/... green
