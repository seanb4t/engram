---
phase: 03-plugin-first-delivery
reviewed: 2026-09-15T00:15:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - internal/setup/plugin.go
  - internal/setup/plugin_test.go
  - .planning/phases/03-plugin-first-delivery/red-evidence/03-01-plugin-version-compare.patch
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
status: issues_found
---

# Phase 03: Code Review Report (iteration 2 — re-review of auto-fix)

**Reviewed:** 2026-09-15T00:15:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found (Info only — no Critical/Warning findings)

## Summary

Re-reviewed the exact four-commit delta (`21d9ada3..HEAD`) produced by the iteration-1 auto-fix:
WR-02 (bound third-party plugin strings), the WR-02 red-evidence patch regeneration, WR-01 (honor
`Action.Tolerant` in the plugin lane), and WR-03 (coverage-only test for Codex's
remove-succeeds/add-fails window). `git diff --stat 21d9ada3..HEAD` touches exactly the three
files named in this iteration's scope — nothing else changed.

**WR-02 (bounding):** every third-party-derived string that reaches a rendered `PluginResult`
field is now routed through the unmodified `boundCapture` (`apply.go`): `res.Installed`
(`plugin.go:246`), `res.Source` (`plugin.go:281`), and both of `classifyPluginVersion`'s
"not a release version"/"dev build" notes and its "newer than binary" note (`plugin.go:432,437,446`).
Traced every other place a third-party string can reach a `Result`/row field
(`describeFailure`/`describeSeamError`/`toleratedNote` for `res.Reason` and the marketplace-failure
`res.Note` path, and `cmd/engram/setup.go`'s `setupApplyPluginFacet`, which copies `p.Installed`/
`p.Source`/`p.Note` straight onto row fields with no separate bounding of its own) — no missed path
found; `plugin.go` is the single choke point, matching the existing `apply.go` discipline exactly.
`maxCapturedBytes` is untouched (still 4096) and no second bounding helper was introduced —
`joinPluginNote` is a note-*joining* helper, not a bounding one, and is orthogonal to `boundCapture`.

**WR-01 (Tolerant):** `executePlugin`'s write loop's new `switch` (`plugin.go:306-328`) matches
`apply.go`'s `execute()` case order and semantics exactly: a seam error is fatal regardless of
`Tolerant`; a `Tolerant` action's nonzero exit is joined onto `Note` via `joinPluginNote`/
`toleratedNote` (the same `toleratedNote` `apply.go` uses) and the sequence continues; a
non-`Tolerant` nonzero exit still fails the row immediately; a `Tolerant` action's `Description` is
also surfaced on success, mirroring `execute()`'s own full behavior. `joinPluginNote` composes
correctly (empty-existing passthrough, otherwise `"; "`-joined) and is exercised twice per the
new loop without ever dropping or duplicating an existing note.

**Test non-vacuousness:** `TestPluginResultFieldsAreBoundCaptured` scripts a version string and a
marketplace source string both `maxCapturedBytes+2000` bytes, and asserts (for `Installed`,
`Source`, and `Note`) that the result is valid UTF-8, carries `truncationMarker`, and is
substantially shorter than the untruncated input — a genuine truncation assertion, not a mere
presence check; it would fail if the WR-02 fix were reverted. `TestExecutePluginHonorsTolerant`
uses a synthetic `fakeTolerantPluginRuntime` whose Tolerant "clear" step is scripted to fail and
whose non-tolerant "install" step is scripted to succeed, and asserts `OutcomeWrote`, the
tolerated-failure text on `Note`, and that both actions actually ran in order — this would fail
under the pre-fix behavior (which failed the row on the tolerated exit and never ran "install").
The new WR-03 subtest (`outdated-add-fails-after-remove-succeeds`) pins `OutcomeFailed` naming
`add` (never `remove`) with exactly two actions run — verified this is coverage of pre-existing,
already-correct behavior (no code change accompanies it), consistent with the orchestrator's
scoping of WR-03 as coverage-only.

**Red-evidence patch:** the regenerated `03-01-plugin-version-compare.patch` is a single-line
mutation (`classifyPluginVersion`'s `default:` arm: `PluginCurrent` → `PluginOutdated`) that applies
cleanly at HEAD and, when applied, drives its mapped target (`TestPluginVersionCompare`, scoped via
the harness's `-run "^TestPluginVersionCompare$"`) red — confirmed by running
`go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1` directly, which passed
and logged `confirmed RED: 03-01-plugin-version-compare.patch applied -> TestPluginVersionCompare
failed as expected`, with the tree cleanly restored afterward.

**Gates run directly:** `go test ./internal/setup/ -count=1` (PASS), `go test ./cmd/engram -count=1`
(PASS, single run only), `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1`
(PASS, all five phase-03 patches — including this iteration's regenerated one — confirmed live-RED,
phases 01/02 entries unaffected).

**No new defect** was introduced by these four commits. `WR-03`'s scoping (coverage, no rollback/
retry mechanism) and `IN-01`'s deferral (bare symlinked native skills dir with zero matching
entries rendering as `"none"`) are per the orchestrator's recorded decisions and are not
re-litigated here.

## Info

### IN-01: Misleading rationale in a new test comment (does not affect test correctness)

**File:** `internal/setup/plugin_test.go:894-897`
**Issue:** `TestPluginResultFieldsAreBoundCaptured`'s comment for `longVersion` says the digit run
is "too long for `strconv.ParseUint` (range error) so `parseVersionCore` reports `!ok`". In fact
`parseVersionCore` never reaches `strconv.ParseUint` for this input: `pluginVersionCorePattern`
(`^(0|[1-9][0-9]*)[.](0|[1-9][0-9]*)[.](0|[1-9][0-9]*)$`) requires two literal dots, and
`longVersion` is a pure digit run with none, so `FindStringSubmatch` returns `nil` and the function
returns `ok == false` at the regex stage, before any `ParseUint` call. The test's actual behavior
and assertions are correct — it genuinely exercises the "not a release version" arm and its
`boundCapture`-wrapped note — this is a comment-accuracy nit only, not a functional or coverage gap.
**Fix:** Reword the comment, e.g.: "A pure-digit run well over `maxCapturedBytes`: `+ pluginVersionCorePattern`+
requires two literal dots this string doesn't have, so `parseVersionCore`'s regex never matches and
it reports `!ok`, landing `classifyPluginVersion` in its 'not a release version' arm — exactly the
arm whose Note interpolates the untrusted `installed` string (plugin.go WR-02 fix)."

---

_Reviewed: 2026-09-15T00:15:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
