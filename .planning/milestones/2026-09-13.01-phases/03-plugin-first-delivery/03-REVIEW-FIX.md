---
phase: 03-plugin-first-delivery
fixed_at: 2026-09-15T04:06:20Z
review_path: .planning/phases/03-plugin-first-delivery/03-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 03: Code Review Fix Report

**Fixed at:** 2026-09-15T04:06:20Z
**Source review:** .planning/phases/03-plugin-first-delivery/03-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope (critical_warning): 3 (WR-01, WR-02, WR-03)
- Fixed: 3
- Skipped: 0
- Out of scope this pass (per orchestrator instruction): IN-01

**Isolation:** all edits and commits were made in a dedicated git worktree
(`.claude/worktrees/rf-03-24293-1789444346`, branch `gsd-reviewfix/03-24293`),
fast-forwarded onto `feat/2026-09-13.01` and torn down after this report was
written. All verification below (`task`, the red-evidence harness, the full
gate) ran inside that isolated worktree — a real git worktree with its own
`go.sum`-resolved module cache, not a hand-rolled directory, so `go test`/
`task` ran exactly as they would in the main checkout.

## Fixed Issues

### WR-02: Untrusted plugin-CLI strings land on rendered fields unbounded

**Files modified:** `internal/setup/plugin.go`, `internal/setup/plugin_test.go`
**Commit:** `753d5c50` (fix), `07c753f6` (red-evidence patch regeneration)
**Applied fix:** Routed the three untrusted plugin-CLI strings that reach
rendered `PluginResult` fields — `Installed` (`entry.Version` from
`plugin list --json`), `Source` (the observed marketplace "Source:" line),
and `classifyPluginVersion`'s note interpolation of `installed` — through
the package's existing `boundCapture` (`apply.go`), unchanged and reused
verbatim (no second bounding helper, `maxCapturedBytes` untouched).
Comparison logic (`parseVersionCore`) still sees the raw, untruncated
`installed` string; only values placed on rendered fields are bounded.

Added `TestPluginResultFieldsAreBoundCaptured`, mirroring
`TestApply/"captured-output-over-budget-truncated-on-rune-boundary"`
(`apply_test.go`): scripts an oversized version string and an oversized
marketplace source line, asserts `Installed`/`Source`/`Note` are valid
UTF-8, carry the truncation marker, and are bounded well below the
untruncated input length. Verified RED against the pre-fix code (reverted
`plugin.go` to HEAD, kept the test) before re-applying the fix and
confirming GREEN.

The red-evidence patch `03-01-plugin-version-compare.patch` targets the
exact lines this fix touches in `classifyPluginVersion`'s default arm and
no longer applied cleanly at the post-fix HEAD. Regenerated it (temporarily
reintroduced the "PluginCurrent -> PluginOutdated" regression on top of the
WR-02 fix, `git diff -U3`, reverted, then proved apply -> RED
(`TestPluginVersionCompare/greater-numeric-patch` fails) -> `git apply -R`
leaves the tree clean) and committed the regenerated patch as a follow-up
commit.

### WR-01: `executePlugin`'s action loop silently ignores `Action.Tolerant`

**Files modified:** `internal/setup/plugin.go`, `internal/setup/plugin_test.go`
**Commit:** `2357822b`
**Applied fix:** `executePlugin`'s write-action loop now honors
`Action.Tolerant` with the same semantics `apply.go`'s `execute()` uses: a
Tolerant action's nonzero exit is recorded onto `PluginResult.Note` (via a
new `joinPluginNote` helper wrapping `toleratedNote`, reused verbatim from
`apply.go`) and the sequence continues; a non-Tolerant nonzero exit, or a
seam error from any action, still fails the row immediately. A Tolerant
action's `Description` is also surfaced on success (exit 0), mirroring
`execute()`'s own full behavior. Updated `executePlugin`'s step-by-step doc
comment (step 10) to document the contract so a future contributor sees it.
No plugin action authored by `claudecode.go`/`codex.go` sets `Tolerant`
today, so behavior is unchanged for every current caller.

Added a synthetic `fakeTolerantPluginRuntime` (test-only — no real runtime
sets `Tolerant` on a plugin action) authoring a Tolerant "clear" step that
fails followed by a non-tolerant "install" step, and
`TestExecutePluginHonorsTolerant` asserting the sequence continues past the
tolerated failure, records it on `Note`, and reaches `OutcomeWrote`.
Verified RED against the pre-fix code (same revert/reapply method as
WR-02) before confirming GREEN.

### WR-03: No test exercises Codex's "remove succeeds, add fails" destructive window

**Files modified:** `internal/setup/plugin_test.go`
**Commit:** `a9cac954`
**Applied fix:** Coverage-only, no behavior change. Added
`outdated-add-fails-after-remove-succeeds` to `TestPluginPlan`'s codex
cases, mirroring the existing `outdated-remove-fails` case: `remove`
succeeds (unscripted, default `RunResult{ExitCode: 0}`), the following
`add` fails via `actionScript`, and the assertions pin the actual observed
outcome — `OutcomeFailed` with a `Reason` naming `add` (never `remove`),
and `wantApplyExtraArgs` stopping at exactly the two actions (remove, then
add) with no third action attempted. The behavior matched expectations on
the first run — no redesign was needed, consistent with the review's own
characterization of this as an accepted, documented risk rather than a
defect.

## Skipped Issues

### IN-01: `setupNativePresenceSummary` can silently drop a bare symlinked destination with zero matching skill entries

**File:** `cmd/engram/setup.go` (`setupNativePresenceSummary`)
**Reason:** skipped-by-scope — orchestrator instruction excluded IN-01 from
this pass (`fix_scope: critical_warning`). Reviewer's own note: a bare
symlinked native skills dir with zero matching entries renders as `"none"`,
dropping the symlink signal. Not classified as a defect (D-08's own wording
conditions the report on "if engram skills are present there"); left for a
future pass or an explicit acknowledgment/test per the review's own Fix
suggestion.

## Verification

- `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -v`
  (no `-short`): **PASS** (73.70s) — all five phase-03 patches confirmed
  live-RED against their mapped target tests
  (`TestPluginVersionCompare`, `TestPluginPlan` x2, `TestDetectPresence`,
  `TestSetupPluginDeliveredRuntimeAuthorsZeroNativeWrites`); phases 01/02
  entries unmodified and still confirmed live.
- `task`: **PASS** (lint clean across golangci-lint/yamlfmt/actionlint/
  rumdl/ruff; full `go test ./...` green, including the red-evidence
  harness inside `internal/store`).
- `task license:check`: **PASS** (1881 files checked, 0 invalid).
- `git diff --exit-code -- go.mod go.sum`: **clean** (no dependency drift;
  `internal/setup` stays a stdlib-only leaf, confirmed separately by
  `TestSetupPackageIsStdlibOnlyLeaf`).
- `go test ./internal/keylinks/ -count=1`: **PASS**.
- `go test ./internal/setup/ -count=1 -shuffle=on`: **PASS**.
- `go test ./cmd/engram -count=1`: **PASS** (never run with `-count>1`, per
  instruction).
- `go run ./internal/surfacesgen --check-setup`: **PASS** (no drift).

## Commits

1. `753d5c50` — `fix(setup): bound third-party plugin strings before they reach a row (WR-02)`
2. `07c753f6` — `fix(setup): regenerate stale red-evidence patch for WR-02's boundCapture`
3. `2357822b` — `fix(setup): honor Action.Tolerant in the plugin lane (WR-01)`
4. `a9cac954` — `test(setup): cover codex remove-then-add where add fails (WR-03)`

All four commits were made on the isolated worktree branch
(`gsd-reviewfix/03-24293`) and fast-forwarded onto `feat/2026-09-13.01` at
worktree teardown; `git log --oneline` on `feat/2026-09-13.01` shows all
four commits in this order, directly atop `21d9ada3` (docs(03): add code
review).

---

_Fixed: 2026-09-15T04:06:20Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_

## Iteration 2 (orchestrator-applied)

**Finding:** IN-01 (iteration 2, cosmetic) — `internal/setup/plugin_test.go`'s new
`TestPluginResultFieldsAreBoundCaptured` comment claimed the long version string fails via a
`strconv.ParseUint` range error; in fact `pluginVersionCorePattern` never matches it (no dots), so
`parseVersionCore` reports `!ok` at the regex stage and `ParseUint` is never reached. The test's
behavior and coverage were correct — only the stated rationale was wrong.

**Fix:** comment rewritten to name the real mechanism. No code change.

**Gates:** `gofmt` clean · `TestPluginResultFieldsAreBoundCaptured` PASS ·
`TestRedEvidencePatchesAreLive` PASS (all three phases).

_Applied by the /gsd-autonomous orchestrator after the --auto fix loop converged (iteration 2 was
otherwise 0 Critical / 0 Warning)._

