---
phase: 04-skills-distribution
plan: 04
subsystem: distribution
tags: [go-embed, skills, setup-command, cobra-help, golden-tests]

requires:
  - phase: 04-skills-distribution
    plan: 01
    provides: "internal/skills' Install/Inventory, the injectable Environment seam, and internal/setup's SkillFormat/SkillTarget/SkillsOutcome/AggregateOutcome model this plan's generic wiring and end-to-end test both build on"
  - phase: 04-skills-distribution
    plan: 03
    provides: "codex and opencode's own SkillTarget authoring — the last two native/agents-md destinations this plan's four-runtime end-to-end test exercises alongside generic"
provides:
  - "generic.go: every Plan() authors the explicit no-destination SkillFormatNone value, carrying the skills payload in its --output json deliverable without ever reaching a filesystem write"
  - "cmd/engram/setup.go: setupSkillsTarget widened to (skills.Target, error) — no runtime is un-wired anymore, so an unrecognized format is a failed row, never a silent skip; setupApplySkillsFacet explicitly skips the install call for the no-destination format"
  - "cmd/engram/setup.go: setupLongDescription states --apply's widened effect (skills install alongside registration, in-binary source) with no per-runtime destination path; setupPreviewSummary names both effects"
  - "cmd/engram/testdata/help.golden: the one deliberate golden movement this phase makes, confined to the engram setup section's added prose; catalog.golden is untouched"
  - "TestSetupReportCoversEveryRuntimeShape: the phase's closing gate — one table-driven test proving the composition, aggregation, row shape, and exit classification together across all four runtime shapes in both lanes"
affects: []

actuals:
  tokens: 9018
  tasks: 3
  commits: 3
  plan_head_before: c03ba948c5a5ad1fd3926dd8e0a5f5c62a10ab68

tech-stack:
  added: []
  patterns:
    - "setupSkillsTarget's exhaustive switch returns an error (never a silent bool skip) once every registered runtime authors an explicit SkillFormat — retiring the 04-01-era 'not wired yet' escape hatch now that nothing is un-wired"
    - "A structural, registry-derived test fixture (setupE2ERecorder) records every process invocation and every skills filesystem write, so 'no real runtime, no real home' is asserted against the recording rather than by construction alone"

key-files:
  created: []
  modified:
    - internal/setup/generic.go
    - internal/setup/generic_test.go
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/testdata/help.golden

key-decisions:
  - "setupSkillsTarget's signature widened from (skills.Target, bool) to (skills.Target, error): the bool's `ok=false` arm used to mean 'this runtime has not been wired for skills yet' (04-01/04-02/04-03, while codex/opencode/generic still carried the Go zero-value SkillTarget). As of this plan every registered runtime (claude-code, codex, opencode, generic) authors one of the three explicit SkillFormat values, so that legitimate skip case no longer exists — an unrecognized format is now unambiguously an authoring bug, surfaced as a failed row naming the unknown value rather than silently continuing with an unset skills facet."
  - "setupApplySkillsFacet explicitly skips the `skills.Install` call when the resolved target's format is the no-destination value, rather than relying on Install's own internal FormatNone no-op. Task 1's own prohibition ('never let generic reach the install path') reads as a statement about the CALL SITE, not merely the observable outcome — belt-and-braces against a future refactor of Install accidentally giving FormatNone a real side effect."
  - "TestSetupReportCoversEveryRuntimeShape's expected aggregated outcomes come from a small literal fold table (setupOutcomeFoldTable) populated by hand from the facet pairs this test's own fakes are scripted to produce — never a call to setup.AggregateOutcome — per the plan's own explicit prohibition on a test computing its own expectation from the function under test."

requirements-completed: [REQ-skills-embedded-in-binary, REQ-skills-agents-md-fallback, REQ-skills-native-format]

coverage:
  - id: D1
    description: "generic's Plan() authors the explicit no-destination skill format on every returned Plan, carrying the curation skills in its --output json deliverable exactly as it already carries the portable server config, with its aggregated outcome pinned to would-write in both lanes and the install call skipped entirely so it can never reach the filesystem."
    requirement: "REQ-skills-embedded-in-binary"
    verification:
      - kind: unit
        ref: "internal/setup/generic_test.go#TestGenericSkillTargetHasNoDestination"
        status: pass
      - kind: unit
        ref: "internal/setup/generic_test.go#TestGenericSkillsOutcomeIsWouldWriteInBothLanes"
        status: pass
      - kind: integration
        ref: "cmd/engram/setup_test.go#TestSetupReportCoversEveryRuntimeShape"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram setup --help states, in its own long description, that --apply installs the curation skills alongside registering the MCP server and that they ship inside the binary — naming no per-runtime destination path — while setupCmd's short description and flag set, and the command catalog golden, stay byte-identical."
    requirement: "REQ-setup-correct-by-reading"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHelpNamesSkillsInstallation"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestDestructiveCommandsExactFlagSet"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHelpNamesEveryRuntimeAndAuthMode"
        status: pass
    human_judgment: false
  - id: D3
    description: "One invocation covering claude-code, codex, opencode, and generic renders one row each with per-facet fields and exactly one aggregated outcome per row, proven across a bare preview, an explicit caller-ordered selection, a partial-failure apply, a total-failure apply, and an all-absent selection — with structural proof that no real runtime binary is executed and no real home directory is written."
    requirement: "REQ-skills-native-format"
    verification:
      - kind: integration
        ref: "cmd/engram/setup_test.go#TestSetupReportCoversEveryRuntimeShape"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-09-10
status: complete
---

# Phase 4 Plan 4: Generic's Skills Payload, the Truthful Help Text, and the Closing Gate Summary

**`generic` now carries the five curation skills in its `--output json` deliverable with no filesystem write, `engram setup --help` tells an operator that `--apply` installs skills alongside registering the server, and one table-driven test proves the aggregated report across all four runtime shapes in both lanes — closing Phase 4.**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-09-10T15:10:00Z
- **Completed:** 2026-09-10T15:35:00Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- `internal/setup/generic.go`: `Plan()` authors `Skills: SkillTarget{Format: SkillFormatNone}` on its one return path, with a doc comment recording D-11's reasoning — generic has no machine of its own, so it derives no `HomeDir()`-based destination and never reaches the filesystem, but still carries the full skill content, a per-skill digest, and a total byte count in its report row exactly like every native runtime.
- `cmd/engram/setup.go`: `setupSkillsTarget` widened from `(skills.Target, bool)` to `(skills.Target, error)` now that every registered runtime authors an explicit `SkillFormat` — an unrecognized value is a failed row naming the unknown format, never a silent skip. `setupApplySkillsFacet` explicitly skips the `skills.Install` call for the no-destination format.
- `cmd/engram/setup.go`: `setupLongDescription` gains a paragraph stating `--apply`'s widened effect (skills install alongside MCP registration, in-binary source, two separate row facets under one aggregated outcome) while naming no per-runtime destination path; `setupPreviewSummary`'s trailing clause now names both effects. `setupCmd.Short` is untouched, with a comment recording why.
- `cmd/engram/testdata/help.golden`: regenerated via the explicit `go test ./cmd/engram -update` invocation; the phase-wide diff (against the pre-phase baseline `788d7127`) is confined to exactly one added paragraph inside the `## engram setup` section. `catalog.golden` and `internal/setup/exit.go`/`exit_test.go` are byte-identical to their pre-phase state.
- `cmd/engram/setup_test.go`: `TestSetupHelpNamesSkillsInstallation` derives its forbidden destination-path segments from each registered runtime's own `Plan()` rather than a hardcoded literal. `TestSetupReportCoversEveryRuntimeShape` — the phase's closing gate — drives the real command through five scenarios (bare preview, explicit four-runtime caller-ordered selection, partial-failure apply, total-failure apply, all-absent selection) against a purpose-built `setupE2ERecorder` that structurally proves no real runtime binary is ever invoked and no real home directory is ever written.

## Task Commits

Each task was committed atomically:

1. **Task 1: `generic` carries the skills in its deliverable, and still writes nothing** — `e9f4dd48` (feat, tdd)
2. **Task 2: `--help` and the headlines state the command's full effect** — `2f54013e` (docs)
3. **Task 3: One invocation, four runtime shapes — the aggregated report proven end to end** — `8c980450` (test, tdd)

**Plan metadata:** committed alongside this SUMMARY.

_Note: Tasks 1 and 3 carried `tdd="true"`; see TDD Gate Compliance below._

## Files Created/Modified

- `internal/setup/generic.go` — `Plan()` authors the no-destination `SkillTarget`; doc comment records D-11.
- `internal/setup/generic_test.go` — `TestGenericSkillTargetHasNoDestination`, `TestGenericSkillsOutcomeIsWouldWriteInBothLanes`.
- `cmd/engram/setup.go` — `setupSkillsTarget` returns `(skills.Target, error)`; `setupApplySkillsFacet` skips `Install` for the no-destination format; `setupLongDescription`/`setupPreviewSummary` state the widened effect; `setupCmd.Short` gains an explanatory comment.
- `cmd/engram/setup_test.go` — `TestSetupHelpNamesSkillsInstallation`, `TestSetupReportCoversEveryRuntimeShape`, `setupE2ERecorder`, `setupOutcomeFoldTable`.
- `cmd/engram/testdata/help.golden` — regenerated; the `## engram setup` section gains the new paragraph.

## Decisions Made

See `key-decisions` in the frontmatter — most notably widening `setupSkillsTarget` to return an error now that no runtime is left un-wired, and explicitly skipping the install call for the no-destination format as belt-and-braces beyond `Install`'s own no-op.

## Deviations from Plan

None — plan executed exactly as written. Every acceptance criterion and `<verify>` command passed on its first run except a single missing `fmt` import in the new test file, caught immediately by `go vet` and fixed inline before any test ran (not a behavioral deviation).

## TDD Gate Compliance

`workflow.tdd_mode` is not set in `.planning/config.json` (defaults false), so the strict plan-level gate (`gsd_run check tdd-red-evidence`) was not invoked. The RED→GREEN discipline was still followed where the plan's own action text made a clean RED phase possible, and disclosed honestly where it did not:

- **Task 1 — no clean RED phase; disclosed, not hidden.** `internal/setup/generic.go`'s `Skills` field addition and `internal/setup/generic_test.go`'s two new tests were written together: the tests describe exactly the one-line change the plan's action text specifies (`Skills: SkillTarget{Format: SkillFormatNone}`), with no intermediate stub state to target as RED — `SkillFormatNone`, `SkillTarget`, and `SkillsOutcome` already existed from 04-01. Both tests passed on their first run, verified live before the implementation-plus-test commit.
- **Task 3 — no clean RED phase; disclosed, not hidden.** `TestSetupReportCoversEveryRuntimeShape` exercises code paths (`setupSkillsTarget`, `setupApplySkillsFacet`, the shared executor) that were already correct after Task 1 and prior waves — the test's purpose is END-TO-END PROOF across all four runtime shapes at once, not driving new production code into existence. All five subtests passed on their first run.
- No REFACTOR-phase test regression occurred in either task.

## Issues Encountered

None.

## Known Stubs

None.

## Threat Flags

None beyond what the plan's own `<threat_model>` already registers (T-04-04, T-04-14, T-04-15, T-04-16, T-04-SC) — every one of those five is directly covered by this plan's own tests, cited in the coverage block above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Phase 4 (Skills Distribution) is complete: all four plans have landed, `REQ-skills-embedded-in-binary`, `REQ-skills-agents-md-fallback`, and `REQ-skills-native-format` are all ready to mark complete (confirmed via `gsd_run query requirements.ready-ids`, `3/3 ready`).
- The full verification suite is green: `go build ./... && go test ./... -count=1`, `task` (lint + full Go/Python suite), and `go test ./internal/keylinks/... -count=1` all pass. `internal/skills/data` is clean relative to its vendored source (`git status --porcelain internal/skills/data` empty).
- Pinned surfaces held for the whole phase: `cmd/engram/testdata/catalog.golden` and `internal/setup/exit.go`/`exit_test.go` are byte-identical to the pre-phase baseline (`788d7127`); `help.golden`'s only phase-wide movement is this plan's one added paragraph in the `## engram setup` section.
- No open blockers. `04-VALIDATION.md`'s Manual-Only Verifications table still has one pending half-row (Claude Code / opencode frontmatter tolerance for `metadata.engram-summary`, noted in 04-03-SUMMARY.md) — informational only, does not gate this phase's completion.

## Self-Check: PASSED

- All 5 key files (modified) verified present on disk with `[ -f ]`.
- All 3 commits (`e9f4dd48`, `2f54013e`, `8c980450`) verified present in `git log --oneline --all`.
- Re-ran `go build ./... && go test ./... -count=1` — all green.
- Re-ran `task` (lint + full suite) — green.
- Re-ran `git diff --stat 788d7127 HEAD -- cmd/engram/testdata/catalog.golden internal/setup/exit.go internal/setup/exit_test.go` — empty (pinned files untouched for the whole phase).
- Re-ran `git diff 788d7127 HEAD -- cmd/engram/testdata/help.golden` — confined to one added paragraph in the `## engram setup` section.
- `git status --porcelain internal/skills/data` — empty.

---
*Phase: 04-skills-distribution*
*Completed: 2026-09-10*
