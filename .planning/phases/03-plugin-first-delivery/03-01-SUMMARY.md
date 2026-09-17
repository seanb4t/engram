---
phase: 03-plugin-first-delivery
plan: 01
subsystem: setup
tags: [go, plugin-delivery, claude-code, codex, setup]

requires:
  - phase: 02-custom-auth-headers
    provides: internal/setup's Runtime/Plan/Action/Environment/runSeam registration lane this plugin lane runs parallel to
provides:
  - "PluginState (absent/outdated/current/unavailable) and the PluginRuntime optional interface in internal/setup/plugin.go"
  - "PluginPreview/PluginApply/executePlugin: one plugin list --json probe decides capability and state, plugin marketplace list decides whether a marketplace add is authored, and a stdlib-only SemVer-core comparator classifies outdated/current"
  - "claude-code and codex both implement PluginRuntime in their own files, authoring the exact D-04/D-05/D-06 argv (marketplace add, install/add, update or remove-then-add)"
affects: [03-02-skills-routing, 03-03-report-integration, 03-04-codex-plugin-manifest]

actuals:
  tokens: 16453
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Plugin lane is a PARALLEL execution path to registration (execute/apply.go), not nested in it — a semantic version comparison cannot be expressed by execute()'s blind byte-compare"
    - "Optional-interface idiom (PluginRuntime) mirrors runtime.go's optInOnlyRuntime: type-asserted once at the lane's entry point, never a by-name branch"
    - "AUTHORED-HERE invariant: every plugin argv element is a fixed literal authored in the runtime's own file (claudecode.go/codex.go), never centrally"

key-files:
  created:
    - internal/setup/plugin.go
    - internal/setup/plugin_test.go
  modified:
    - internal/setup/claudecode.go
    - internal/setup/codex.go
    - .planning/phases/02-custom-auth-headers/red-evidence/02-01-codex-header-decline.patch

key-decisions:
  - "Regenerated the stale 02-01-codex-header-decline.patch in place rather than reporting it as a separate follow-up: the staleness was a direct, mechanical consequence of this plan's own codex.go edit (import block shift, and `strings` now used elsewhere), and TestRedEvidencePatchesAreLive is a hard phase gate — fixing it inline kept the gate green without touching redevidence_harness_test.go itself."
  - "Corrected a stale gsd-plan-head-before-03-01 commit ledger left over from an aborted 2026-09-08 attempt at this same plan slot; the true pre-plan base is f58f7a46 (this session's own last docs commit), verified via git rev-list --count against both bases."

requirements-completed: [REQ-plugin-capability-detection, REQ-plugin-install-or-update, REQ-plugin-three-way-state]

coverage:
  - id: D1
    description: "PluginState (absent/outdated/current/unavailable), PluginRuntime interface, PluginResult, PluginPreview/PluginApply/executePlugin, and the stdlib SemVer-core comparator"
    requirement: REQ-plugin-three-way-state
    verification:
      - kind: unit
        ref: "internal/setup/plugin_test.go#TestPluginVersionCompare"
        status: pass
      - kind: unit
        ref: "internal/setup/plugin_test.go#TestSetupPackageIsStdlibOnlyLeaf"
        status: pass
    human_judgment: false
  - id: D2
    description: "claude-code implements PluginRuntime: one plugin list --json probe decides capability/state, marketplace list decides whether marketplace add is authored, argv carries -y and --scope user"
    requirement: REQ-plugin-capability-detection
    verification:
      - kind: unit
        ref: "internal/setup/plugin_test.go#TestPluginPlan/claude-code"
        status: pass
      - kind: unit
        ref: "internal/setup/plugin_test.go#TestPluginCapabilityProbeFailureFallsBackToNative/claude-code"
        status: pass
    human_judgment: false
  - id: D3
    description: "codex implements PluginRuntime: installed[] matched by name+marketplaceName, two-column marketplace parse, HTTPS Git URL marketplace add, remove-then-add update (D-02, no native update verb)"
    requirement: REQ-plugin-install-or-update
    verification:
      - kind: unit
        ref: "internal/setup/plugin_test.go#TestPluginPlan/codex"
        status: pass
      - kind: unit
        ref: "internal/setup/plugin_test.go#TestPluginCapabilityProbeFailureFallsBackToNative/codex"
        status: pass
      - kind: unit
        ref: "internal/setup/plugin_test.go#TestPluginRuntimeIsOptional"
        status: pass
    human_judgment: false
  - id: D4
    description: "Full repo gate (task, task license:check, empty go.mod/go.sum diff, keylinks satisfiability, shuffled internal/setup run) stays green, including the pre-existing TestRedEvidencePatchesAreLive harness"
    verification:
      - kind: other
        ref: "task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1 && go test ./internal/setup/ -count=1 -shuffle=on"
        status: pass
    human_judgment: false

duration: 34min
completed: 2026-09-15
status: complete
---

# Phase 3 Plan 1: Plugin Delivery Lane Summary

Parallel plugin-delivery lane in `internal/setup` — one JSON probe classifies absent/outdated/current for both claude-code and codex, and a stdlib-only SemVer comparator decides updates without ever churning a current or dev-build install.

## Performance

- **Duration:** 34 min
- **Started:** 2026-09-14T21:40:00Z (approx.)
- **Completed:** 2026-09-14T22:14:00Z (approx.)
- **Tasks:** 3
- **Files modified:** 5 (2 created, 3 modified; 1 of the 3 modified files — the red-evidence patch — is outside this plan's `files_modified`, see Deviations)

## Accomplishments

- `PluginState` (absent/outdated/current/unavailable), the `PluginRuntime` optional interface, `PluginResult`, `PluginPreview`/`PluginApply`/`executePlugin`, and a stdlib-only SemVer-core comparator (`classifyPluginVersion`) all landed in the new `internal/setup/plugin.go` — a lane parallel to registration's `execute()`, never nested inside it.
- claude-code implements `PluginRuntime`: one `plugin list --json` probe decides capability and state (matched by `id`), `plugin marketplace list` decides whether a `marketplace add` is authored, and every write action carries `-y`/`--scope user`.
- codex implements `PluginRuntime`: `installed[]` matched by BOTH `name` and `marketplaceName`, a two-column marketplace table read coarsely, an HTTPS Git URL marketplace add, and remove-then-add for "outdated" since codex has no native update verb (D-02).
- opencode and generic deliberately stay outside the plugin lane (`TestPluginRuntimeIsOptional`): zero `Run` calls, `Attempted == false`.
- All 3 tasks' full repo gate (`task`, `task license:check`, empty `go.mod`/`go.sum` diff, `internal/keylinks`, shuffled `internal/setup`) is green.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end plugin lane for claude-code** - `bfaebf14` (feat)
2. **Task 2: Codex implements PluginRuntime** - `7d184cde` (feat)
3. **Task 3 gate fixups (golangci-lint findings)** - `30ed9238` (chore)

**Deviation fix (see below):** `f6f419a8` (fix) — regenerated a stale phase-02 red-evidence patch.

**Plan metadata:** committed separately after this SUMMARY.

_Both Task 1 and Task 2 were TDD (`tdd="true"`): tests were written first and observed to fail to compile (RED — `PluginState`/`PluginRuntime`/`PluginPreview` undefined for Task 1; `Codex` not satisfying `PluginRuntime` for Task 2) before the production code was added (GREEN). See "TDD Gate Compliance" below._

## Files Created/Modified

- `internal/setup/plugin.go` - PluginState, PluginRuntime, PluginResult, PluginPreview/PluginApply/executePlugin, classifyPluginVersion + stdlib SemVer-core comparator
- `internal/setup/plugin_test.go` - TestPluginVersionCompare, TestPluginPlan (claude-code + codex), TestPluginCapabilityProbeFailureFallsBackToNative (claude-code + codex), TestPluginRuntimeIsOptional
- `internal/setup/claudecode.go` - claude-code's PluginRuntime implementation (probes, JSON parse, coarse marketplace parse, marketplace-add/install/update actions)
- `internal/setup/codex.go` - codex's PluginRuntime implementation (probes, installed[] parse, two-column marketplace parse, marketplace-add/add/remove actions)
- `.planning/phases/02-custom-auth-headers/red-evidence/02-01-codex-header-decline.patch` - regenerated to match codex.go's new import block (see Deviations)

## Decisions Made

- Followed the plan's literal argv/behavior specification verbatim (exact strings for `Command`, `Reason`, and `Note` were pinned by the plan itself and verified by acceptance-criteria `rg` greps).
- Regenerated the stale red-evidence patch in place (Rule 1: auto-fix bug) rather than deferring it, since `TestRedEvidencePatchesAreLive` is a hard gate this plan's own Task 3 runs, and the staleness was a direct, mechanical consequence of Task 2's `codex.go` edit.
- Corrected a stale commit-ledger sentinel (`gsd-plan-head-before-03-01`) left over from an earlier, aborted 2026-09-08 attempt at this plan slot — verified the true base against `git log`/`git reflog` before trusting the measured commit count.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Two golangci-lint findings from the full `task` gate**
- **Found during:** Task 3 (`task` run)
- **Issue:** `strconv.ParseUint` result named `min` shadowed the builtin (`revive`); a test-local `[][]string` literal-then-append pattern triggered `prealloc`.
- **Fix:** Renamed `min` to `mnr` in `classifyPluginVersion`'s helper `parseVersionCore`; preallocated `wantApplyCalls` with `make([][]string, 0, 2+len(c.wantApplyExtraArgs))`.
- **Files modified:** `internal/setup/plugin.go`, `internal/setup/plugin_test.go`
- **Verification:** `task lint:go` exits 0 with "0 issues."; full test suite re-run green.
- **Committed in:** `30ed9238`

**2. [Rule 1 - Bug] Stale phase-02 red-evidence patch broke `TestRedEvidencePatchesAreLive`**
- **Found during:** Task 3 (`task` run)
- **Issue:** `02-01-codex-header-decline.patch`'s import-block hunk (removing the `strings` import) no longer applies: Task 2 added `encoding/json` to `codex.go`'s import block (shifting context) and Task 2's `ParseMarketplaceList` now uses `strings.Split`/`Fields`/`Join`, so `strings` can no longer be removed at all when reproducing the RED state.
- **Fix:** Regenerated the patch to remove only the `opts.Headers`-decline block inside `codex-code`'s `Plan()`, leaving the import block untouched. Manually re-verified, before trusting `task`'s own run: (1) `git apply --check` succeeds, (2) with the patch applied, `TestCodexDeclinesHeaders` genuinely FAILS (5 of 6 subtests fail, `err = <nil>, want errors.Is(err, ErrHeaderUnsupported)`), (3) `git apply -R` restores the tree exactly (`git diff --exit-code` clean).
- **Files modified:** `.planning/phases/02-custom-auth-headers/red-evidence/02-01-codex-header-decline.patch` (outside this plan's `files_modified` — the plan's own `<verification>` "touches only the four files in files_modified" check does not hold for this reason, recorded here rather than silently passed over)
- **Verification:** `task` (full lint+test, including `internal/store`'s `TestRedEvidencePatchesAreLive`) exits 0.
- **Committed in:** `f6f419a8`

---

**Total deviations:** 2 auto-fixed (1 lint/Rule 1, 1 stale-fixture/Rule 1).
**Impact on plan:** Both fixes were necessary to keep the full repo gate green; neither touched `internal/store/redevidence_harness_test.go` itself (the hard constraint) or any of this plan's own production logic beyond a cosmetic rename.

## TDD Gate Compliance

| Task | RED observed | GREEN | REFACTOR | Notes |
|------|---------------|-------|----------|-------|
| Task 1 | Yes — `plugin_test.go` in place, `plugin.go` absent: `go build ./internal/setup/...` failed naming `PluginState`/`PluginRuntime`/`PluginPreview` undefined | Yes — full Task 1 verify command green after `plugin.go`+`claudecode.go` additions | N/A (no refactor commit needed) | Single feat commit `bfaebf14` per the plan's own commit-scope instruction (test+prod authored together, not split into separate `test(...)`/`feat(...)` commits — the plan's `<action>` specified one commit) |
| Task 2 | Yes — codex subtests added, `codex.go` unmodified: `TestPluginPlan/codex/*` failed (`Codex does not implement PluginRuntime`, `runPluginCase` `t.Fatalf`) | Yes — full Task 2 verify command green after `codex.go` additions | N/A | Single feat commit `7d184cde`, same rationale |

## Issues Encountered

None beyond the two auto-fixed deviations above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `PluginRuntime`, `PluginPreview`/`PluginApply`, and `PluginResult.Delivered()` are ready for plan 03-02 (skills routing) and 03-03 (report integration / `setupApplyPluginFacet`) to consume.
- `internal/setup` remains a verified stdlib-only leaf (`TestSetupPackageIsStdlibOnlyLeaf` green); `go.mod`/`go.sum` unchanged.
- No blockers for 03-02/03-03/03-04.

## Self-Check: PASSED

- `internal/setup/plugin.go` and `internal/setup/plugin_test.go` exist on disk: confirmed via `[ -f ]`.
- Commits `bfaebf14`, `7d184cde`, `30ed9238`, `f6f419a8` all found via `git log --oneline --all`.
- All plan-level `<acceptance_criteria>` re-run and PASS (see task-by-task verification above and the `rg` greps run during execution).
- Plan-level `<verification>` commands re-run and PASS: `go test ./internal/setup/ -run '^(TestPluginVersionCompare|TestPluginPlan|TestPluginCapabilityProbeFailureFallsBackToNative|TestPluginRuntimeIsOptional)$' -count=1 -v`, `rg -n -e 'LookPath' internal/setup/plugin.go` (empty), `rg -n -F 'golang.org/x/mod' internal/setup/*.go` (empty), `task && task license:check && git diff --exit-code -- go.mod go.sum && go test ./internal/keylinks/ -count=1` (all exit 0).

---
*Phase: 03-plugin-first-delivery*
*Completed: 2026-09-15*
