---
phase: 03-plugin-first-delivery
plan: 03
subsystem: setup
tags: [go, plugin-delivery, claude-code, codex, setup, report-composition]

requires:
  - phase: 03-plugin-first-delivery
    provides: "plan 03-01's PluginRuntime/PluginPreview/PluginApply lane and plan 03-02's DetectPresence, both composed into the report row here"
provides:
  - "The plugin facet (Plugin/PluginState/PluginInstalled/PluginTarget/PluginSource/PluginCommand/PluginNote) on setupRuntimeRow, folded through setup.AggregateOutcome alongside registration and skills"
  - "D-07/D-08/D-09 routing: a plugin-delivered runtime never reaches skills.Install; it gets a SkillsNative presence report instead"
  - "D-12 fallback: a plugin probe failure never fails the row — registration and the native skills copy proceed exactly as today"
  - "--help text describing plugin-first delivery, with the stale 'no separate plugin install is required' claim removed"
affects: [03-04-codex-plugin-manifest]

actuals:
  tokens: 14278
  tasks: 3
  commits: 2
  plan_head_before: 6c002e4eb43b1f137c70a44c13b1e37f38632e26

tech-stack:
  added: []
  patterns:
    - "Third facet composed in cmd/engram, never in internal/setup's shared executor — setupApplyPluginFacet mirrors setupApplySkillsFacet's own shape exactly (registration, then plugin, then skills, folded two-way twice)"
    - "Routing decided by CAPABILITY (PluginResult.Delivered()), never by install success — a plugin-capable runtime never receives the native copy in the same run even when its own install just failed"

key-files:
  created: []
  modified:
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/setup_delegation_test.go
    - cmd/engram/operator_view_setup_test.go
    - cmd/engram/testdata/help.golden

key-decisions:
  - "Fixed a pre-existing TestSetupClientID assertion (cmd/engram/setup_test.go) that iterated every captured Run call and required a bare preview's non-add call to equal exactly `claude mcp get engram` — the new read-only plugin list probe is a second, legitimate preview-lane call, so the assertion now also accepts `claude plugin list --json` (Rule 1: the assertion was incompatible with the new correct behavior, not a design change)."
  - "Task 3's literal acceptance-criteria bullet `go vet ./cmd/engram/ exits 0` does not hold, independent of this plan: cmd/engram/operator_view_test.go's TestOperatorViewDuplicateKeyAdjacency deliberately declares two struct fields with the same json tag (a `//nolint:govet` annotation respected by golangci-lint's own vet analyzer but not by bare `go vet`), confirmed unmodified by `git diff HEAD -- cmd/engram/operator_view_test.go` before and after this plan. `task lint:go` (the actual gate `task` runs) uses golangci-lint, which is green. Documented here rather than silently declared satisfied."

requirements-completed: [REQ-plugin-facet-reported, REQ-plugin-skips-skills-copy, REQ-plugin-capability-detection, REQ-plugin-install-or-update, REQ-plugin-three-way-state]

coverage:
  - id: D1
    description: "The plugin facet is composed onto setupRuntimeRow as its own seven flat-scalar fields, folded through setup.AggregateOutcome beside registration and skills — a wrote registration next to a failed plugin install stays visible on one row and reaches exitPartial"
    requirement: REQ-plugin-facet-reported
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupApplyJSONEmitsPluginFacet"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupPreviewShowsPluginArgv"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_delegation_test.go#TestSetupGeneratedInvocations"
        status: pass
    human_judgment: false
  - id: D2
    description: "A plugin-delivered runtime (Delivered() == true) never calls skills.Install and never writes an AGENTS.md index block; it reports what already sits at the native destination via skills.DetectPresence (path, count, symlink-vs-copy), never removing anything"
    requirement: REQ-plugin-skips-skills-copy
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites"
        status: pass
    human_judgment: false
  - id: D3
    description: "A plugin capability-probe failure (nonzero exit or seam timeout) never fails the row: registration and the native skills copy proceed exactly as today, with the reason reported on plugin_note and plugin_state=unavailable"
    requirement: REQ-plugin-capability-detection
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupPluginUnavailableFallsBackToNative/exit-nonzero"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupPluginUnavailableFallsBackToNative/timeout"
        status: pass
    human_judgment: false
  - id: D4
    description: "`engram setup --help` describes plugin-first delivery (marketplace add/install/update/current, mutual exclusion, report-never-remove) with no destination path segment, and the stale 'no separate plugin install is required' claim is gone; help.golden regenerated in the same commit"
    requirement: REQ-plugin-facet-reported
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHelpNamesPluginDelivery"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHelpNamesSkillsInstallation"
        status: pass
      - kind: unit
        ref: "cmd/engram/golden_test.go#TestHelpGolden"
        status: pass
    human_judgment: false
  - id: D5
    description: "Full repo gate (task, task license:check, empty go.mod/go.sum diff, internal/keylinks, golden drift check, setupgen drift check) stays green"
    verification:
      - kind: other
        ref: "task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -count=1 && go run ./internal/surfacesgen --check-setup"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-09-15
status: complete
---

# Phase 3 Plan 3: Plugin-First Report Composition Summary

The `engram setup` report row now carries a third facet — plugin delivery, folded through the same aggregation as registration and skills — so a `wrote` registration next to a `failed` plugin install stays visible at `exitPartial`, and a plugin-delivered runtime never receives the native skills copy.

## Performance

- **Duration:** 40 min (approx.)
- **Started:** 2026-09-15T02:15:00Z (approx.)
- **Completed:** 2026-09-15T02:55:00Z (approx.)
- **Tasks:** 3
- **Files modified:** 5 (setup.go, setup_test.go, setup_delegation_test.go, operator_view_setup_test.go, testdata/help.golden)

## Accomplishments

- `setupRuntimeRow` gained eight new flat-scalar fields (`Plugin`, `PluginState`, `PluginInstalled`, `PluginTarget`, `PluginSource`, `PluginCommand`, `PluginNote`, `SkillsNative`), all `omitempty`, rendered by the existing `renderOperator` pipeline with zero bespoke code — proven by the flat-scalar identity gate over four new view fixtures.
- `setupApplyPluginFacet` composes the plugin facet exactly the way `setupApplySkillsFacet` composes the skills facet: independently computed, folded via `setup.AggregateOutcome`, with a failed plugin install joining its reason onto `Reason` as `plugin: ...` alongside (never instead of) a registration failure.
- `setupRuntimeRowFromResult` now runs the plugin lane (`setup.PluginPreview`/`setup.PluginApply`) once per present runtime, reusing the registration lane's already-resolved `Result.Binary` — no second `LookPath` — and routes to either the native write path (`setupApplySkillsFacet`) or the new report-only path (`setupReportNativeSkills`) by `PluginResult.Delivered()` (capability, never install success).
- `setupReportNativeSkills`/`setupNativePresenceSummary` implement D-07/D-08/D-09: a plugin-delivered runtime's row never calls `skills.Install` (one call site remains, in the native path) and never writes `AGENTS.md`; it reports what already exists (path, count, symlink-vs-copy, index-block presence) via `skills.DetectPresence`, joined into a summary string ending "— remove manually to avoid duplicates" when anything is found, else "none".
- A plugin capability-probe failure (D-12) — nonzero exit or a `runSeam` timeout — never fails the row: `PluginResult.Outcome` stays empty so nothing folds into the aggregate, and registration plus the native skills copy proceed exactly as before this phase.
- `--help`'s "--apply also installs..." paragraph is rewritten to describe plugin-first delivery (marketplace add/install/update/current, mutual exclusion, report-never-remove), with the stale "no separate plugin install is required" sentence removed; `help.golden` regenerated in the same commit as its cause.
- Five new tests (`TestSetupApplyJSONEmitsPluginFacet`, `TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites`, `TestSetupPreviewShowsPluginArgv`, `TestSetupPluginUnavailableFallsBackToNative`, `TestSetupHelpNamesPluginDelivery`) plus an updated `TestSetupGeneratedInvocations` pin the whole facet end to end, entirely against scripted fakes — no real `claude`/`codex` verb, no real `$HOME` touch.
- Full repo gate (`task`, `task license:check`, empty `go.mod`/`go.sum` diff, `internal/keylinks`, golden drift, `internal/surfacesgen --check-setup`) is green.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end plugin facet in the report** - `1adda9a7` (feat)
2. **Task 2: Unavailable fallback proof, help paragraph, view fixtures, golden regen** - `c2059eec` (feat)
3. **Task 3: Plan gate** - no commit needed (verification-only; no formatter/license fixer rewrote anything)

**Plan metadata:** committed separately after this SUMMARY.

_Both Task 1 and Task 2 were TDD (`tdd="true"`): tests were written first and observed to fail to compile (RED — `setupVersion`/`Plugin`-family fields undefined for Task 1; `TestSetupHelpNamesPluginDelivery` failing against HEAD's Long text and `TestHelpGolden` failing with a diff before `-update` for Task 2) before the production code was added (GREEN). See "TDD Gate Compliance" below._

## Files Created/Modified

- `cmd/engram/setup.go` - `setupVersion` seam, eight new row fields, `setupApplyPluginFacet`, `setupNativePresenceSummary`, `setupReportNativeSkills`, widened `setupRuntimeRowFromResult`, rewritten plugin-first help paragraph
- `cmd/engram/setup_test.go` - plugin test scaffolding (`fakeSkillsEnvWithEntries`, `withFakeSetupVersion`, `fakePluginRun`, `recording`, `counterBase`, fixture literals) and five new tests
- `cmd/engram/setup_delegation_test.go` - `TestSetupGeneratedInvocations` updated to expect each `PluginRuntime`'s list probe after its registration sequence in both lanes
- `cmd/engram/operator_view_setup_test.go` - four plugin-facet view fixtures
- `cmd/engram/testdata/help.golden` - regenerated with the plugin-first paragraph

## Decisions Made

- Followed the plan's literal field names, function shapes, and exact argv/string assertions verbatim (pinned by the plan itself and verified by acceptance-criteria `rg` greps).
- See "Deviations from Plan" below for the two auto-fixes required to keep the full test suite and stated acceptance criteria consistent with the new, correct behavior.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Pre-existing `TestSetupClientID` preview-call assertion incompatible with the new plugin probe**
- **Found during:** Task 1 (full `go test ./cmd/engram -count=1` run)
- **Issue:** `TestSetupClientID`'s first subtest iterates every captured `env.Run` call in the preview lane and requires any non-`add` call to equal exactly `["claude","mcp","get","engram"]`. The new plugin capability probe (`claude plugin list --json`) is a second, legitimate read-only preview-lane call this test had no way to know about.
- **Fix:** Extended the assertion to also accept `["claude","plugin","list","--json"]`, with a comment explaining why.
- **Files modified:** `cmd/engram/setup_test.go`
- **Verification:** `go test ./cmd/engram -count=1` exits 0 (whole package).
- **Committed in:** `1adda9a7` (Task 1 commit)

**2. [Rule 1 - Bug] Task 3's own acceptance-criteria bullet (`go vet ./cmd/engram/ exits 0`) does not hold, independent of this plan**
- **Found during:** Task 1 (verifying acceptance criteria)
- **Issue:** `cmd/engram/operator_view_test.go`'s `TestOperatorViewDuplicateKeyAdjacency` deliberately declares two struct fields sharing the same `json` tag (an intentional edge-probe fixture, annotated `//nolint:govet` for golangci-lint's own vet analyzer). Bare `go vet` still flags it (`structtag` check has no bare-vet nolint mechanism), independent of anything in this plan — confirmed via `git diff HEAD -- cmd/engram/operator_view_test.go` showing no changes from this plan.
- **Fix:** None — out of scope (a pre-existing condition in a file this plan never touches; fixing the deliberate test fixture would defeat its purpose). `task lint:go` (the actual gate `task` runs, and which Task 3's `<verify>` actually invokes) uses golangci-lint, which respects the `nolint` directive and is green.
- **Files modified:** none.
- **Verification:** `task` (the real Task 3 gate) exits 0; `golangci-lint run ./cmd/engram/...` reports "0 issues."
- **Committed in:** N/A — documentation only, recorded here per the executor's deviation-tracking discipline.

---

**Total deviations:** 2 auto-fixed/documented (2 Rule 1).
**Impact on plan:** The first fix was necessary to keep the whole `cmd/engram` package green, as the plan's own acceptance criteria require. The second is a documentation-only note about a pre-existing, out-of-scope condition — no code change, no scope creep.

## Issues Encountered

None beyond the two items documented above.

## TDD Gate Compliance

| Task | RED observed | GREEN | REFACTOR | Notes |
|------|---------------|-------|----------|-------|
| Task 1 | Yes — with the test edits in place and `setup.go` reverted to HEAD, `go test ./cmd/engram -count=1` failed to compile, naming `setupVersion` and every `Plugin*`/`SkillsNative` field undefined (verified explicitly by temporarily checking out HEAD's `setup.go`, confirming the compile failure, then restoring the modified file) | Yes — full Task 1 verify command green after `setup.go`'s production edits | N/A (no refactor commit needed) | Single feat commit `1adda9a7` per the plan's own commit-scope instruction |
| Task 2 | Yes — `TestSetupHelpNamesPluginDelivery` failed against HEAD's Long text (missing "plugin"/"marketplace"/etc. vocabulary); `TestHelpGolden` failed with a diff after the Long edit and before `-update` | Yes — both pass after the production edit and golden regeneration | N/A | Single feat commit `c2059eec`, same rationale |

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The plugin facet, D-07/D-08/D-09 routing, and D-12 fallback are all live in `engram setup`'s report — plan 03-04 (Codex plugin manifest, `setupgen`, `/engram-setup` regeneration) can proceed independently; this plan never touched `skill/engram/.codex-plugin/plugin.json`, `release-please-config.json`, `internal/setupgen/*`, or `skill/engram/commands/engram-setup.md`.
- All five of this phase's `REQ-plugin-*` requirements are now marked complete in `REQUIREMENTS.md` (`REQ-plugin-skips-skills-copy`'s shared-ID gate with plan 03-02 cleared now that both declaring plans have summaries).
- `internal/setup`/`internal/skills` remain untouched by this plan (composition lives entirely in `cmd/engram`); `go.mod`/`go.sum` unchanged.
- No blockers for 03-04.

## Self-Check: PASSED

- `cmd/engram/setup.go`, `cmd/engram/setup_test.go`, `cmd/engram/setup_delegation_test.go`, `cmd/engram/operator_view_setup_test.go`, `cmd/engram/testdata/help.golden` all exist on disk with the described content: confirmed via direct inspection.
- Commits `1adda9a7` and `c2059eec` both found via `git log --oneline --all`.
- All plan-level `<acceptance_criteria>` for all three tasks re-run and PASS (every `rg` grep and `go test` invocation shown above).
- Plan-level `<verification>` commands re-run and PASS: the nine-test `-run` regex (`TestSetupApplyJSONEmitsPluginFacet|TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites|TestSetupPreviewShowsPluginArgv|TestSetupPluginUnavailableFallsBackToNative|TestSetupHelpNamesPluginDelivery|TestSetupGeneratedInvocations|TestOperatorViewFixturesHaveNoUnsanitizedNesting|TestHelpGolden|TestCatalogGolden`) shows `--- PASS` for every test and subtest; `rg -c -F 'skills.Install(' cmd/engram/setup.go` prints `1`; `rg -n -F 'separate plugin install is required' cmd/engram/setup.go` prints nothing; `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go run ./internal/surfacesgen --check-setup` all exit 0; `git diff --stat` from the plan's base commit touches exactly the five files listed above under `cmd/engram/` (of the plan's six declared `files_modified`; `catalog.golden` did not move since its `Short` text is unchanged).

---
*Phase: 03-plugin-first-delivery*
*Completed: 2026-09-15*
