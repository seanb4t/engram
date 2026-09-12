---
phase: 03-runtime-registration
plan: 04
subsystem: setup
tags: [mcp-registration, generic-runtime, portable-config, opt-in-registry, argv-security]

requires:
  - phase: 03-runtime-registration
    provides: "Action.Args/Action.Tolerant, Plan.Probe, Environment.Run seam, setup.Preview()/Apply() shared executor (plan 03-01); the corrected per-runtime bearer conventions (plans 03-02, 03-03) that leave bearerProvenance with zero callers"
provides:
  - "Generic (internal/setup/generic.go): the opt-in pseudo-runtime that authors a portable, single-line {\"mcpServers\":{\"engram\":{...}}} JSON document instead of any CLI invocation — zero Actions, no Probe, no subprocess of any kind"
  - "optInOnlyRuntime (runtime.go): a structural, self-declared predicate a Runtime may implement to opt itself out of Select(nil)'s default (no-`--runtime`) selection — resolves D-14's stated purpose without a by-name check anywhere in runtime.go"
  - "Plan.Config (plan.go): the field carrying a Config-only runtime's whole deliverable, copied onto Result.Config by the shared executor unchanged"
  - "apply.go's general zero-Action-Plan handling (D-16): OutcomeWouldWrite in both the preview and --apply lane, before ever touching env.LookPath or env.Run — applies to any future Config-only runtime, not special-cased on this one's name"
  - "cmd/engram/setup.go: the preview lane (setupBuildRows) now carries plan.Config onto the report row, matching the apply lane's existing wiring"
affects: [03-05-remaining-wave, 06-docs-site]

actuals:
  tokens: 11936
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Structural opt-in-only predicate (optInOnlyRuntime) over a by-name exclusion — the same structural-predicates-over-enumerations shape cmd/engram/cmdwalk.go's operatorCommands() already uses — for a runtime that must be excluded from a default selection without runtime.go's no-special-casing-by-name constraint ever naming it"
    - "Zero-Action Plan as a general, safely-degrading executor state (apply.go): a Plan with no Actions and no Probe is not an error case — it is a Config-only deliverable, classified OutcomeWouldWrite unconditionally, with no LookPath/Run reached at all"

key-files:
  created:
    - internal/setup/generic.go
    - internal/setup/generic_test.go
  modified:
    - internal/setup/runtime.go
    - internal/setup/plan.go
    - internal/setup/apply.go
    - internal/setup/plan_test.go
    - internal/setup/detect_test.go
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/testdata/catalog.golden
    - cmd/engram/testdata/help.golden

key-decisions:
  - "D-14's literal phrasing (\"Detect() reports not-present unless explicitly named\") could not be implemented as written: Runtime.Detect(env Environment) bool carries no signal about whether the runtime was named on --runtime, and adding one would either change that method's signature for every registered runtime or require a by-name check outside generic.go — both forbidden. Resolved with optInOnlyRuntime, a structural predicate the runtime declares about itself, consumed once in Select's empty-names branch. D-14's stated PURPOSE (a bare invocation never claims presence for, or inflates the denominator with, a runtime that corresponds to no installed software) is achieved by construction; Detect() itself stays unconditionally true, its own doc comment stating plainly why that alone is not what prevents inclusion in the default set."
  - "Generic's bearer mode does NOT follow claude-code/opencode/codex's ENGRAM_TOKEN-naming convention when --token-file is supplied: it falls back to bearerProvenance's literal path-provenance placeholder instead. Generic has no CLI of its own that could resolve a \"${...}\" substitution token at connect time on an arbitrary third-party client, so it cannot promise the operator that such a reference will ever expand there; showing the file's own path is the honest alternative. This makes generic bearerProvenance's ONE remaining production caller (D-06), which forced removing the //nolint:unused directive plan 03-02 had added while the function was temporarily dead."
  - "The D-15 clause 'a bare `--output json` nests the portable config properly as an object for machine consumers' is NOT taken. A row field typed to marshal as a JSON object bypasses viewScalar's sanitizing branch (cmd/engram/operator_view.go) entirely and fails TestOperatorViewFixturesHaveNoUnsanitizedNesting by design — reopening that gap would put an unsanitized, operator-supplied --url into the text lane (the T-06-03 control the guard protects). D-15's other clause (minified single-line JSON in an ordinary row field) is honored intact: Config stays a Go string, and --output json emits it as a JSON STRING whose contents happen to be JSON, never a nested object. Recorded in setupRuntimeRow's own doc comment so a later 'consistency' edit fails the guard test rather than silently reopening the gap."

requirements-completed: [REQ-register-generic-mcp]

coverage:
  - id: D1
    description: "engram setup --runtime generic --url <url> emits a portable MCP server configuration as a single-line minified JSON document keyed by mcpServers with an engram entry carrying type, url, and (bearer mode) a headers object"
    requirement: REQ-register-generic-mcp
    verification:
      - kind: unit
        ref: "internal/setup/generic_test.go#TestGenericConfig"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupGenericRowCarriesPortableConfig"
        status: pass
    human_judgment: false
  - id: D2
    description: "generic starts no process of any kind — Detect always true, but the shared executor never reaches env.LookPath or env.Run for a zero-Action Plan, in either the preview or --apply lane"
    verification:
      - kind: unit
        ref: "internal/setup/generic_test.go#TestGenericStartsNoProcess"
        status: pass
    human_judgment: false
  - id: D3
    description: "a bare engram setup (no --runtime) never lists or claims presence for generic, so the report's present/selected denominator is never inflated by a pseudo-runtime (D-14)"
    verification:
      - kind: unit
        ref: "internal/setup/plan_test.go#TestSelectDefaultSetExcludesOptInRuntimes"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupBareInvocationOmitsGeneric"
        status: pass
    human_judgment: false
  - id: D4
    description: "no literal credential value ever reaches generic's Config text, in either bearer sub-form (${ENGRAM_TOKEN} reference or bearerProvenance's path placeholder)"
    requirement: REQ-register-auth-modes
    verification:
      - kind: unit
        ref: "internal/setup/generic_test.go#TestGenericConfigCarriesNoSecret"
        status: pass
      - kind: unit
        ref: "internal/setup/plan_test.go#TestNoSecretInArgs"
        status: pass
    human_judgment: false
  - id: D5
    description: "generic and a failing native runtime selected together exit exitPartial (8), never exitSetupFailed (9) — internal/setup/exit.go's Classify table is untouched, and the resulting exit-code consequence is pinned by a test"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupGenericAndFailingRuntimeExitsPartial"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-09
status: complete
plan_head_before: 4f927f5e
commits: 3
---

# Phase 3 Plan 4: Generic pseudo-runtime and its portable MCP config Summary

**`engram setup --runtime generic --url <url>` now prints a minified, single-line `{"mcpServers":{"engram":{...}}}` document for pasting into any MCP client engram doesn't natively support — an opt-in registry member with zero Actions, no Probe, and no subprocess of any kind, structurally excluded from a bare invocation's default selection.**

## Performance

- **Duration:** 45 min
- **Started:** 2026-09-09T15:31:00Z (approximate — this plan's PLAN_START_TIME was not captured at spawn; derived from the surrounding session's STATE.md timestamps)
- **Completed:** 2026-09-09T16:19:00Z
- **Tasks:** 3
- **Files modified:** 11 (2 created, 9 modified)

## Accomplishments
- `internal/setup/generic.go`: `genericRuntime`/`Generic`, an unconditional `Detect()` (documented as deliberately meaningless on its own — see Decisions), and a `Plan()` that builds `{"mcpServers":{"engram":{"type":"http","url":...,"headers":{...}}}}` via `encoding/json.Marshal` on a small controlled struct (never string concatenation), authoring zero `Action`s and no `Probe`.
- `internal/setup/runtime.go`: the `optInOnlyRuntime` structural predicate; `Generic` appended last to `Runtimes`; `Select(nil)`'s empty-names branch now skips any runtime declaring itself opt-in-only — the explicit-names branch (`--runtime generic`) is unchanged.
- `internal/setup/plan.go`: `Plan.Config` field; `bearerProvenance`'s doc comment corrected (generic is its one remaining caller) and its now-stale `//nolint:unused` removed.
- `internal/setup/apply.go`: `execute()` classifies a zero-`Action` `Plan` `OutcomeWouldWrite` immediately, in both lanes, before touching `LookPath` or `Run` — a general rule, not keyed on any runtime's name.
- `cmd/engram/setup.go`: the preview lane (`setupBuildRows`) now carries `plan.Config` onto the report row (previously only the apply lane did); `setupRuntimeRow`'s doc comment records the D-15 clause deliberately not taken.
- Every registry-wide test in `internal/setup` (`TestPlanPassesURLVerbatim*`, `TestNoSecretInArgs`, `TestPlanAuthModes`, `TestPlanBearerRedactsCredentialByProvenance`, `TestDetectEveryRegisteredRuntime`, `TestDetectOnlyCodexPresent`, `TestSelectEmptyReturnsEveryRuntime`) made structural over `len(plan.Actions)`/`Plan.Config`/an empirically-observed "always detected" property, rather than skipping the new registry entry by name; `TestSelectDefaultSetExcludesOptInRuntimes` added, derived from the `optInOnlyRuntime` predicate itself so a second opt-in runtime can't make it vacuous.
- `cmd/engram/setup_test.go`: a `defaultRuntimeCount(t)` helper replacing `len(setup.Names())` in four bare-invocation row-count assertions (a bare `engram setup` now targets `Select(nil)`'s default set, which is one smaller than the full registry); three new tests (`TestSetupGenericRowCarriesPortableConfig`, `TestSetupBareInvocationOmitsGeneric`, `TestSetupGenericAndFailingRuntimeExitsPartial`).
- `cmd/engram/testdata/{help,catalog}.golden` regenerated via the repo's own `-update` step; diff confined to the `--runtime` value list and long description.

## Task Commits

Each task was committed atomically. Task 1 (`tdd="true"`) followed the canonical RED→GREEN sequence — see "TDD Gate Compliance" below for why the RED commit's scope extends beyond `generic.go`/`generic_test.go` alone.

1. **Task 1: The generic pseudo-runtime and the structural opt-in predicate** — `1413e6a6` (test, RED) then `1d15ee99` (feat, GREEN)
2. **Task 2: Render the portable config as a scalar row field** — folded into Task 1's RED/GREEN commits (see Deviations — the preview-lane `Config` wiring and the three new `cmd/engram` tests could not be isolated from Task 1's own RED state without leaving them permanently untested)
3. **Task 3: Registry-order fixture updates and golden regeneration** — the test-file updates (`plan_test.go`, `detect_test.go`) folded into Task 1's RED/GREEN commits for the same reason; the golden regeneration is its own commit, `145f68b1` (test)

**Plan metadata:** committed separately per this workflow's final-commit step.

## Files Created/Modified
- `internal/setup/generic.go` — `Generic`/`genericRuntime`: `Name`, `OptInOnly`, `Detect`, `Plan` (builds the portable config document)
- `internal/setup/generic_test.go` — `TestGenericConfig`, `TestGenericStartsNoProcess`, `TestGenericConfigCarriesNoSecret`
- `internal/setup/runtime.go` — `optInOnlyRuntime` interface, `Runtimes` gains `Generic`, `Select`'s default-set exclusion
- `internal/setup/plan.go` — `Plan.Config` field; `bearerProvenance` doc comment corrected, stale `nolint` removed
- `internal/setup/apply.go` — `execute()`'s general zero-Action-Plan handling (D-16)
- `internal/setup/plan_test.go` — `planCarriesURL` helper; `TestPlanPassesURLVerbatim*`, `TestNoSecretInArgs`, `TestPlanAuthModes`, `TestPlanBearerRedactsCredentialByProvenance`, `TestSelectEmptyReturnsEveryRuntime` made structural; `TestSelectDefaultSetExcludesOptInRuntimes` added
- `internal/setup/detect_test.go` — `TestDetectEveryRegisteredRuntime`, `TestDetectOnlyCodexPresent` made structural over an "always detected" property rather than a hardcoded runtime-name comparison
- `cmd/engram/setup.go` — `setupBuildRows` carries `plan.Config`; `setupRuntimeRow` doc comment records the D-15 deviation
- `cmd/engram/setup_test.go` — `defaultRuntimeCount(t)` helper; `TestSetupGenericRowCarriesPortableConfig`, `TestSetupBareInvocationOmitsGeneric`, `TestSetupGenericAndFailingRuntimeExitsPartial`
- `cmd/engram/testdata/catalog.golden`, `cmd/engram/testdata/help.golden` — regenerated

## Decisions Made

See `key-decisions` in frontmatter for the two decisions this plan's own `<action>` text explicitly called for (D-14's structural resolution; the bearer-mode fallback to `bearerProvenance` when `--token-file` is supplied) and the D-15 clause deviation (Config stays a string, never a nested JSON object).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Every registry-wide test in `internal/setup` broke the moment `Generic` joined `Runtimes`**
- **Found during:** Task 1, running the plan's own `<verify>` command set
- **Issue:** `TestPlanPassesURLVerbatimGatewayRoute`, `TestPlanPassesURLVerbatimRootMounted`, `TestNoSecretInArgs`, `TestOAuthAndNoneAuthorIdenticalArgs`, `TestPlanAuthModes`, `TestPlanBearerRedactsCredentialByProvenance`, `TestSelectEmptyReturnsEveryRuntime` (all `internal/setup/plan_test.go`) and `TestDetectEveryRegisteredRuntime`, `TestDetectOnlyCodexPresent` (`internal/setup/detect_test.go`) all loop over `Runtimes` with assumptions a zero-`Action`, unconditionally-`Detect`ed registry member falsifies — none of these files are in this plan's `files_modified` frontmatter list, and `detect_test.go` is not named anywhere in the plan text at all
- **Fix:** Made every assertion structural — over `len(plan.Actions)`/`Plan.Config` presence, or an empirically-observed "Detect() is unconditionally true" property — rather than skipping the new entry by name (forbidden by this plan's own acceptance criteria and by `runtime.go`'s no-special-casing constraint)
- **Files modified:** `internal/setup/plan_test.go`, `internal/setup/detect_test.go`
- **Verification:** `go test ./internal/setup/... -count=1` green
- **Committed in:** `1413e6a6` (RED) / `1d15ee99` (GREEN)

**2. [Rule 3 - Blocking] `bearerProvenance`'s `//nolint:unused` directive itself became unused**
- **Found during:** final `task` run, after Task 3
- **Issue:** Once `generic.go` calls `bearerProvenance` again, golangci-lint's `nolintlint` linter flags the now-superfluous `//nolint:unused` directive plan 03-02 had added while the function was temporarily dead
- **Fix:** Removed the directive and rewrote the doc comment to name generic as the current caller
- **Files modified:** `internal/setup/plan.go`
- **Verification:** `task` (lint + full suite) green
- **Committed in:** `1413e6a6` (the comment/nolint edit landed as supporting infrastructure in the RED commit, ahead of the GREEN commit that gives the function a real caller again — harmless, since Go does not flag an unused *package-level function* at compile time, only golangci-lint's `unused` linter does, and that check only runs in the final `task` pass)

**3. [Rule 1 - Bug] Four bare-invocation row-count assertions in `cmd/engram/setup_test.go` assumed `len(setup.Names())` equals the default selection size**
- **Found during:** Task 2/3 verification
- **Issue:** `TestSetupPreviewExecutesNoRuntimeCLI`, `TestSetupApplyAllAbsentExitsZero`, `TestSetupApplyAtLeastOnePresentExitsSetupFailed`, and `TestSetupApplyJSONEmitsPerRuntimeOutcome` all compared a bare invocation's row count against `len(setup.Names())` — correct before this plan (every registered runtime WAS the default set) but wrong now that `Names()` reports 4 while `Select(nil)` resolves to 3
- **Fix:** Added `defaultRuntimeCount(t)` (derives the expectation from `setup.Select(nil)` itself) and replaced all four call sites
- **Files modified:** `cmd/engram/setup_test.go`
- **Verification:** `go test ./cmd/engram/... -count=1` green
- **Committed in:** `1413e6a6` (RED) / `1d15ee99` (GREEN)

---

**Total deviations:** 3 auto-fixed (2 Rule 1 — direct, foreseeable consequences of registering a fourth, structurally-different runtime; 1 Rule 3 — a blocking lint failure with no destructive fix available other than removing the now-superfluous directive). **Impact:** All three were necessary consequences of this plan's own registry change; no runtime's registered behavior outside `generic` itself was touched.

## TDD Gate Compliance

Task 1 (`tdd="true"`) followed the canonical RED→GREEN sequence: `1413e6a6` committed `internal/setup/generic_test.go` (and the plan_test.go/detect_test.go/setup_test.go structural fixes and new tests it forced, per Deviations above) against a **stubbed** `generic.go`'s `Plan()` (`return Plan{Runtime: "generic"}, nil` — no config document built at all). Confirmed RED: `go test ./internal/setup/... ./cmd/engram/... -count=1` failed on `TestGenericConfig`, `TestGenericStartsNoProcess`, `TestPlanPassesURLVerbatimGatewayRoute`, `TestPlanPassesURLVerbatimRootMounted`, `TestPlanAuthModes`, `TestPlanBearerRedactsCredentialByProvenance`, and `TestSetupGenericRowCarriesPortableConfig` — each failing for the expected reason (missing/empty `Config`), nothing else. `1d15ee99` then restored the real `Plan()` implementation (the `encoding/json.Marshal`-built document); confirmed GREEN: the same command is fully green. No REFACTOR commit followed — nothing needed reshaping after GREEN.

Task 2 and Task 3 are `type="auto"` (not `tdd`), but their own test-file changes were mechanically forced by Task 1's registry change before either task could be verified in isolation, so they landed inside Task 1's RED/GREEN pair rather than as independent commits — recorded as a deviation from the plan's literal three-commit shape above, not from the RED→GREEN discipline itself, which Task 1's own scope satisfies in full.

## Issues Encountered

None beyond the deviations above. No real `claude`/`codex`/`opencode` binary or any other subprocess was invoked by any test in this plan — `generic` itself is proven to reach neither `env.LookPath` nor `env.Run` at all (`TestGenericStartsNoProcess`), and every other affected test already drove a scripted fake (rule `m45p2b4bp7`).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All four registered runtimes (`claude-code`, `codex`, `opencode`, `generic`) are now wired end-to-end through the shared executor and the operator report; `generic` is the one whose entire deliverable is `Plan.Config` rather than an executed write.
- `REQ-register-generic-mcp` is complete. `REQ-register-auth-modes` and `REQ-setup-idempotent` remain `Pending` in REQUIREMENTS.md — both are declared by 03-05 as well (the shared-ID gate, #2388), and `requirements.ready-ids` confirmed only `REQ-register-generic-mcp` was ready to mark from this plan.
- `03-05` (preview-side probe reporting, the `token_file` marker, help prose, golden regeneration for the remaining surface) is the last plan in this phase's Wave 4.
- No blockers.

---
*Phase: 03-runtime-registration*
*Completed: 2026-09-09*

## Self-Check: PASSED
