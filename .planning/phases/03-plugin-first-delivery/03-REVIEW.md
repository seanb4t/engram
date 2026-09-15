---
phase: 03-plugin-first-delivery
reviewed: 2026-09-14T23:50:00Z
depth: standard
files_reviewed: 15
files_reviewed_list:
  - internal/setup/plugin.go
  - internal/setup/plugin_test.go
  - internal/setup/claudecode.go
  - internal/setup/codex.go
  - internal/skills/presence.go
  - internal/skills/presence_test.go
  - internal/skills/environment.go
  - cmd/engram/setup.go
  - cmd/engram/setup_test.go
  - cmd/engram/setup_delegation_test.go
  - cmd/engram/operator_view_setup_test.go
  - internal/setupgen/setupgen.go
  - internal/setupgen/setupgen_test.go
  - internal/setupgen/manifest_test.go
  - internal/store/redevidence_harness_test.go
findings:
  critical: 0
  warning: 3
  info: 1
  total: 4
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-09-14T23:50:00Z
**Depth:** standard
**Files Reviewed:** 15 (plus release-please-config.json, skill/engram/.codex-plugin/plugin.json, skill/engram/commands/engram-setup.md, cmd/engram/testdata/help.golden, internal/surfacesgen/main_test.go inspected as diff-only)
**Status:** issues_found

## Summary

Reviewed Phase 3's plugin-first delivery lane end to end: the hand-rolled SemVer-core comparator
and D-01/D-03 classification (`internal/setup/plugin.go`), both runtimes' probe/action authoring
(`claudecode.go`, `codex.go`), the read-only presence reporter (`internal/skills/presence.go`), the
routing/aggregation logic in `cmd/engram/setup.go`, and the setupgen plugin-table generator. Ran the
full test suite (`internal/setup`, `internal/skills`, `internal/setupgen`, `cmd/engram`, and the
`internal/store` red-evidence harness with all five phase patches confirmed live-RED against their
named tests, phases 01/02 entries unmodified) plus `task lint`, all green.

The version state machine, the exact-match (never substring) JSON matching on both CLIs, the
never-write presence reporter, the plugin/native routing invariant (`p.Delivered()`, capability-gated
never success-gated), the plugin outcome fold into `exitPartial`, and the codex remove-then-add
sequencing all check out against their documented decisions (D-01–D-12) and are backed by passing
tests, including a dedicated negative-space test for zero native writes when plugin-delivered. No
argv ever carries a parsed/untrusted value — every action's `Args` is a package-level constant slice
authored in the owning runtime's file. No panics found on malformed/empty/huge JSON inputs.

Three warnings below are real gaps worth fixing before this ships further: an inconsistency in how
`Action.Tolerant` is honored between the two execution lanes, an unbounded-length third-party string
landing on rendered report fields (bypassing this package's own established bounding discipline), and
a documented destructive-window scenario (Codex: remove succeeds, add fails) with zero test coverage
in either the internal/setup or cmd/engram suite.

## Warnings

### WR-01: `executePlugin`'s action loop silently ignores `Action.Tolerant`

**File:** `internal/setup/plugin.go:287-299`
**Issue:** `Action.Tolerant` is a package-level, type-level contract (`plan.go:100-114`): "a nonzero
exit from THIS action is expected and does not fail the row." `apply.go`'s `execute()` (the
registration lane) honors it explicitly (`apply.go:335,344`). `executePlugin`'s own write loop,
however, never reads `action.Tolerant` at all — every action run through this lane is treated as
fatal on any nonzero exit or seam error, regardless of how it was authored:
```go
for _, action := range actions {
    rr, runErr := runSeam(ctx, env, binary, action.Args[1:])
    if runErr != nil {
        res.Outcome = OutcomeFailed
        ...
    }
    if rr.ExitCode != 0 {
        res.Outcome = OutcomeFailed
        ...
    }
}
```
Today this is behaviorally harmless because none of the four plugin actions
(`claudePluginMarketplaceAddAction`, `claudePluginInstallAction`, `claudePluginUpdateAction`,
`codexPluginMarketplaceAddAction`, `codexPluginAddAction`, `codexPluginRemoveAction`) sets
`Tolerant: true`. But `Action` is a single shared, exported type consumed by two different executors
with two different interpretations of the same field, with no doc comment on `PluginRuntime` or
`executePlugin` calling out the divergence. A future contributor extending either runtime's
`PluginActions` (e.g. adding a tolerant "clear the slot" step mirroring `claudeCodeRemoveAction`'s own
style) will get silent, wrong behavior — the action will fail the row on any tolerated nonzero exit —
with no compiler or test signal pointing at the cause.
**Fix:** Either honor `action.Tolerant` in `executePlugin`'s loop the same way `apply.go` does, or add
an explicit runtime assertion (and doc comment) that every plugin action must have `Tolerant == false`,
failing loudly if that invariant is ever violated, so the two lanes' semantics for the same field type
cannot silently diverge.

### WR-02: Untrusted plugin-CLI strings land on rendered fields unbounded, unlike every other captured string in this package

**File:** `internal/setup/plugin.go:234,264,377-399`; `cmd/engram/setup.go:219-231`
**Issue:** `apply.go` establishes and documents (`apply.go:31-38`) a specific defense —
`maxCapturedBytes`/`boundCapture`/`displayCapture` — precisely because "a runtime that floods stdout
could otherwise flood the operator's terminal or the --output json lane," and applies it to every
third-party capture that reaches a rendered field (`Result.Registered`, `Reason`, `Notes` — see
`apply.go:158,181,183,313,376`). The plugin lane introduces three new rendered fields sourced directly
from the same class of untrusted `plugin list --json` / `plugin marketplace list` output, but none of
them goes through that bounding:
- `internal/setup/plugin.go:234`: `res.Installed = version` — `version` is `entry.Version` parsed
  straight out of third-party JSON (`claudecode.go:273`, `codex.go:233`), unbounded.
- `internal/setup/plugin.go:264`: `marketplacePresent, res.Source = pr.ParseMarketplaceList(...)` —
  the observed "Source:" line, unbounded.
- `internal/setup/plugin.go:384,389,398`: `classifyPluginVersion`'s notes interpolate the untrusted
  `installed` string directly via `fmt.Sprintf`, unbounded.

These flow unchanged into `row.PluginInstalled`, `row.PluginSource`, and `row.PluginNote`
(`cmd/engram/setup.go:224,226,228`). `renderOperatorView`'s `sanitizeViewValue` strips control
characters generically (mitigating terminal-escape injection), but applies no length cap, so a
compromised or simply buggy plugin CLI / marketplace (D-05 explicitly accepts a foreign,
operator-uncontrolled "engram" marketplace "as-is") returning a very long version or source string
floods the operator's terminal and bloats the `--output json` payload — exactly the failure mode
`maxCapturedBytes` exists to prevent everywhere else in this package.
**Fix:** Route `version`, `installed` (in the classification notes), and the marketplace `source`
through `boundCapture` (or an equivalent bound) before they reach `PluginResult`, mirroring how every
other third-party capture in this package is treated.

### WR-03: No test exercises Codex's "remove succeeds, add fails" destructive window

**File:** `internal/setup/codex.go:180-191`; `internal/setup/plugin_test.go` (codex cases,
lines 454-582); `cmd/engram/setup_test.go`
**Issue:** `codexPluginRemoveAction`'s own doc comment names the exact risk D-02's remove-then-add
sequencing accepts: "If the following add fails after this action succeeds, no codex plugin remains
until --apply is re-run." This is a real, documented, user-visible failure mode unique to Codex (Claude
Code's single `plugin update` action has no analogous window). The test suite covers `remove` failing
(`plugin_test.go:504-520`, `outdated-remove-fails`) and `add` failing from the `absent` state
(`apply-install-fails` — claude only), but no test — in `internal/setup/plugin_test.go` or
`cmd/engram/setup_test.go` — drives the `PluginOutdated` codex case where `remove` exits 0 and the
subsequent `add` exits nonzero (or seam-errors). Given this is the one scenario the code's own
comments single out as an accepted risk, it should be the one scenario a regression test pins: that
the row still correctly reports `OutcomeFailed` with a `Reason` naming `add` (not `remove`), that no
third action is attempted, and that `Command` still reflects both actions (so the preview/apply text an
operator sees still matches what would be re-run).
**Fix:** Add a `pluginCase` (mirroring `outdated-remove-fails`) scripting `plugin remove
engram@engram --json` to succeed and `plugin add engram@engram --json` to fail, asserting
`OutcomeFailed`, the `add`-naming `Reason`, and that `wantApplyExtraArgs` stops after `add` (i.e. the
loop ran exactly the two actions, in order, with the second one failing).

## Info

### IN-01: `setupNativePresenceSummary` can silently drop a bare symlinked destination with zero matching skill entries

**File:** `cmd/engram/setup.go` (`setupNativePresenceSummary`); `internal/skills/presence.go:57-59`
**Issue:** `DetectPresence` sets `Presence.Symlink = true` whenever `target.Dir` **itself** is a
symlink, independent of how many of the current skill inventory's entries are found beneath it
(`presence.go:57-59`). `setupNativePresenceSummary`, however, only emits the symlink-aware "N skills
present … (symlink)" line when `p.Skills > 0`; if `Dir` is a symlink but zero of the currently-shipped
skill names are found under it (e.g. the target's contents have drifted from the binary's current
`skills.Inventory()` — plausible for exactly the "hand-made symlink into the marketplace clone"
scenario 03-CONTEXT.md's own live facts describe), the function reports `"none"`, and the fact that
`Dir` is itself a symlink pointing somewhere is silently lost from the report. This is a narrow edge
case (D-08's own wording conditions the report on "if engram skills are present there"), so it is not
classified as a defect, but it is worth a one-line acknowledgment or an explicit test pinning the
current (silent) behavior as intentional, since it is not obviously so from the code alone.
**Fix:** Either extend `setupNativePresenceSummary` to note a bare `Dir`-is-symlink-with-zero-matches
case, or add a code comment (and a `TestDetectPresence`/`setupNativePresenceSummary` test) explaining
why this is deliberately out of scope.

---

_Reviewed: 2026-09-14T23:50:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
