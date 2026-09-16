---
phase: 05-apply-time-preserve-gate-documentation
reviewed: 2026-09-16T00:00:00Z
depth: standard
files_reviewed: 17
files_reviewed_list:
  - cmd/engram/agent_setup_docs_test.go
  - cmd/engram/install_docs_test.go
  - cmd/engram/plugin_docs_test.go
  - cmd/engram/setup_test.go
  - cmd/engram/setup.go
  - cmd/engram/testdata/help.golden
  - docs-site/src/content/docs/guides/agent-setup.md
  - docs-site/src/content/docs/guides/install.md
  - docs-site/src/content/docs/guides/plugin.md
  - internal/setup/apply_test.go
  - internal/setup/apply.go
  - internal/setup/claudecode_test.go
  - internal/setup/claudecode.go
  - internal/setup/codex.go
  - internal/setup/drift_test.go
  - internal/setup/drift.go
  - internal/setup/plan.go
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 5: Code Review Report

**Reviewed:** 2026-09-16T00:00:00Z
**Depth:** standard
**Files Reviewed:** 17
**Status:** issues_found

## Summary

Reviewed the apply-time preserve gate implementation (`internal/setup/apply.go`,
`drift.go`, `claudecode.go`, `codex.go`, `plan.go`), the CLI wiring
(`cmd/engram/setup.go`), the three docs-gate test files, the generated
`help.golden`, and the three updated guides, at standard depth, cross-checked
against `05-CONTEXT.md`'s D-01..D-07 and diffed line-for-line against the
pre-phase revision to isolate what this phase actually changed.

The security-critical property held up under adversarial tracing: in
`execute()`'s `mutate == true` path, `classifyProbe` is computed once from
probe1 and shared by both lanes; a `compared` classification of
`OutcomeAlreadyCorrect` or `OutcomePreserved` returns before the write-action
loop's first iteration (`apply.go:486-492`), so `claudeCodeRemoveAction`
(`plan.Actions[0]` for every Claude Code auth mode) is never dispatched on
those rows. An ambiguous/scanner-less/seam-error classification falls through
unchanged to the write loop (ambiguity resolves to `wrote`, never skips a
write). The plugin lane runs independently of the registration outcome
(`setupApplyPluginFacet` is called unconditionally for a present runtime),
confirmed live by `TestSetupApplyPreservedRuntimeSkipsRegistrationWrite`. The
D-02 post-write re-observe rebuilds `Registered` via
`Observe`/`renderObservation`, never `displayCapture`, and unconditionally
sets `Outcome = OutcomeWrote` regardless of what the re-observe finds. The
D-03 OAuth re-login note (`claudeCodeOAuthReLoginNote`) is set in
`claudecode.go`'s `Observe` purely by observed shape (`auth == AuthNone`),
but is only ever surfaced onto `Result.Notes` by `apply.go`'s
`renderClassification` when `c.drift.Outcome == OutcomeWouldWrite` — verified
against `TestOAuthReLoginConsequence`'s bearer/foreign/preserved/
already-correct/ambiguous/codex negative cases, all of which assert `Notes ==
""`. `ManualRemediation` is authored per-runtime in `claudecode.go`/`codex.go`
(D-05) and only appended by the shared executor when non-empty. The redaction
discipline (`REQ-drift-redaction`) holds for every new field: `Registered`,
`Drift`, `Reason` (which now also carries `ManualRemediation`) are run through
`boundCapture` in `renderClassification`, and the post-write re-observe path
never uses `displayCapture` for a compared runtime. `internal/setup` remains
stdlib-only across every file in scope (verified by import scan). No test in
this phase's diff invokes a real `claude`/`codex`/`opencode` binary or touches
`$HOME` — every scripted test drives `fakeEnvWithRun`/`scriptedRun` or
`fakeSetupEnvWithRun`/`scriptedSetupRun`/`fakePluginRun` fakes (rule
`m45p2b4bp7` honored). `agent-setup.md`'s `already-correct` row no longer
claims "does not guarantee that no write commands ran," and all three guides'
"Unreleased as of v0.16.1" notices are truthful against
`git describe --tags --abbrev=0` (`v0.16.1`).

Two minor issues found, both low-severity and outside the security-critical
path; see below.

## Warnings

### WR-01: Preview's "not compared" `Drift` note bypasses `boundCapture`, inconsistent with the rest of this file's own bounding discipline

**File:** `internal/setup/apply.go:454-461`
**Issue:** In the `!mutate` (Preview) branch, when `!c.compared`, the code
does `res.Drift = c.notCompared; return res` with no `boundCapture` call. Every
other rendered field in this file that can carry untrusted or variable-length
content (`Drift`/`Registered`/`Reason` in `renderClassification`, `Reason` in
`describeFailure`, `Notes` via `toleratedNote`, `Registered` via
`displayCapture` in the ambiguous apply path) is explicitly bounded to
`maxCapturedBytes`. `c.notCompared` is built by `classifyProbe` from
`notComparedNote(name, quoteArgs(plan.Probe)+": "+probe1Err.Error())` when
probe1 hits a seam error — `probe1Err.Error()` is not a probe-body capture
(so this is not a `REQ-drift-redaction` violation), but it is also not
bounded, unlike every sibling field this same commit's own doc comments
(`WR-01` references throughout `apply.go`) commit to capping. This is
pre-existing behavior carried through the Phase 5 refactor unchanged (the
pre-refactor code had the identical gap), but the refactor was an opportunity
to close it and the file's own `TestDriftFieldsStayBoundedAgainstOversizedProbeContent`
test does not cover this specific "not compared" preview path, so a
regression here (e.g. a future OS/exec error that embeds a long or
attacker-influenced PATH) would go undetected.
**Fix:**
```go
if !c.compared {
    res.Drift = boundCapture(c.notCompared)
    return res
}
```
Apply the same treatment inside `classifyProbe` (or at the single call site
above) so every "not compared" rendering path — preview and any future apply
consumer — is bounded uniformly with the rest of the file.

## Info

### IN-01: `TestApplyConvergesCodex`/`TestApplyConvergesClaudeCode` subtests build fixtures with `strings.Replace`, which silently no-ops when the anchor string drifts

**File:** `internal/setup/apply_test.go:73-74`, `346`, `607-608`
**Issue:** Several tests derive a "no bearer" or "old URL" fixture via
`strings.Replace(codexGetEngramBearer, `"bearer_token_env_var":"ENGRAM_TOKEN"`, ..., 1)`
without checking the replacement count or verifying the substitution actually
landed. If `codexGetEngramBearer`'s literal text ever changes (e.g. its own
fixture drifts in an unrelated future edit) `strings.Replace` returns the
original string unchanged with no error, and the test would then silently
exercise the wrong fixture (a bearer-shaped registration instead of the
intended no-bearer one) rather than failing loudly. This phase's own new test
(`TestDriftFieldsStayBoundedAgainstOversizedProbeContent`) already guards
against exactly this failure mode with an explicit
`if !strings.Contains(stdout, hugeURL) || !strings.Contains(stdout, hugeName) { t.Fatal(...) }`
check, showing the pattern is known; it just was not applied to the older
`Replace` call sites this phase touches.
**Fix:** Add the same landed-substitution assertion used in
`TestDriftFieldsStayBoundedAgainstOversizedProbeContent`, e.g.:
```go
noBearer := strings.Replace(codexGetEngramBearer,
    `"bearer_token_env_var":"ENGRAM_TOKEN"`, `"bearer_token_env_var":null`, 1)
if noBearer == codexGetEngramBearer {
    t.Fatal("fixture setup failed: bearer_token_env_var replacement did not land")
}
```

---

_Reviewed: 2026-09-16T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
