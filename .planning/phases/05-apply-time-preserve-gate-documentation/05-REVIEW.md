---
phase: 05-apply-time-preserve-gate-documentation
reviewed: 2026-09-16T23:15:00Z
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
  warning: 0
  info: 1
  total: 1
status: clean
---

# Phase 5: Code Review Report

**Reviewed:** 2026-09-16T23:15:00Z
**Depth:** standard
**Files Reviewed:** 17
**Status:** clean

## Summary

Re-review (iteration 2) after fix commit `fd045e52`, which addressed WR-01
from the prior review (`05-REVIEW.iter2.md`). Verified the fix directly
against the diff (`git diff 89be5f13..HEAD -- internal/setup/`) and against
the full current state of `internal/setup/apply.go`, not just the fixer's
report.

**WR-01 closed, correctly.** `execute()`'s `!mutate` (Preview) branch now
reads `res.Drift = boundCapture(c.notCompared)` (`apply.go:464`), matching
every other rendered field in the file that carries potentially
attacker/OS-influenced content (`renderClassification`'s
Drift/Registered/Reason at `apply.go:280-282`, `describeFailure`'s Reason,
`toleratedNote`'s Notes). This is the single call site that renders
`c.notCompared` (`classifyProbe` never renders it directly), so no
duplicate fix site was needed. The new test,
`TestPreviewNotComparedDriftStaysBounded`, is a real regression guard, not a
tautology: it drives a scripted `codex` probe #1 seam error carrying a
100KB error string through the real `Preview` seam (`fakeEnvWithRun`/
`scriptedRun`, rule `m45p2b4bp7` honored — no real CLI invoked), then
asserts both a byte-length ceiling (`maxCapturedBytes + len(truncationMarker)
+ 64`) and the literal absence of the unbounded 100KB payload in
`res.Drift`. Confirmed it fails against the pre-fix code by reverting the
one-line change locally and re-running — matches the fixer's own claimed
verification.

**The security-critical `mutate == true` gate is untouched by this fix**,
confirmed by direct re-read of `execute()` (`apply.go:372-600`): a
`c.compared` classification of `OutcomeAlreadyCorrect` or `OutcomePreserved`
still returns at `apply.go:490-493`, strictly before the write-action loop's
first iteration — so `claudeCodeRemoveAction` (`plan.Actions[0]` for every
Claude Code auth mode) still never runs on a preserved or already-correct
row. `TestApplyPreservedNeverRunsClaudeCodeRemove`,
`TestApplyPreservedIssuesZeroWrites`, and
`TestApplyAlreadyCorrectIssuesZeroWrites` all pass. `go build`/`go vet` on
`./internal/setup/...` and `./cmd/engram/...` are clean, and
`go test ./internal/setup/... ./cmd/engram/...` passes in full (a separate,
pre-existing `go vet` finding in `cmd/engram/operator_view_test.go` — a
duplicate `json` struct tag — is outside this phase's file scope and
unrelated to the fix; not reported here).

IN-01 (`strings.Replace` fixture derivations at `apply_test.go:73-74, 346,
607-608, etc.` with no landed-substitution check) was out of the fixer's
scope (`fix_scope: critical_warning`) and stands unchanged on re-read — still
Info, carried forward without escalation per this iteration's instructions.

No new issues were introduced by the fix commit, and a fresh scan of the
full phase scope (all 17 files) surfaced nothing beyond what iteration 1
already found and this iteration's fix already resolved.

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
intended no-bearer one) rather than failing loudly. This phase's own newer
tests (`TestDriftFieldsStayBoundedAgainstOversizedProbeContent`,
`TestPreviewNotComparedDriftStaysBounded`) already guard against exactly
this failure mode with explicit landed-substitution assertions, showing the
pattern is known; it just was not applied to the older `Replace` call sites
this phase touches.
**Fix:** Add the same landed-substitution assertion used in the newer
tests, e.g.:
```go
noBearer := strings.Replace(codexGetEngramBearer,
    `"bearer_token_env_var":"ENGRAM_TOKEN"`, `"bearer_token_env_var":null`, 1)
if noBearer == codexGetEngramBearer {
    t.Fatal("fixture setup failed: bearer_token_env_var replacement did not land")
}
```

---

_Reviewed: 2026-09-16T23:15:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
