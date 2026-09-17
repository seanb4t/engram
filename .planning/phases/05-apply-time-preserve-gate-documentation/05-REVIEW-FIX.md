---
phase: 05-apply-time-preserve-gate-documentation
fixed_at: 2026-09-16T23:01:49Z
review_path: /Volumes/Code/github.com/seanb4t/engram/.planning/phases/05-apply-time-preserve-gate-documentation/05-REVIEW.md
iteration: 1
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 5: Code Review Fix Report

**Fixed at:** 2026-09-16T23:01:49Z
**Source review:** /Volumes/Code/github.com/seanb4t/engram/.planning/phases/05-apply-time-preserve-gate-documentation/05-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 1 (fix_scope: critical_warning — CR-*/BL-*/WR-* only; REVIEW.md carried 0 Critical, 1 Warning, 1 Info; IN-01 excluded by scope)
- Fixed: 1
- Skipped: 0

**Verification environment:** main checkout (no worktree — `workflow.use_worktrees` is honored elsewhere, but this run edited/committed directly in the existing branch checkout `feat/2026-09-13.01` per the orchestrator's explicit `project_constraints`, which pin this fixer to the main working tree).

## Fixed Issues

### WR-01: Preview's "not compared" `Drift` note bypasses `boundCapture`, inconsistent with the rest of this file's own bounding discipline

**Files modified:** `internal/setup/apply.go`, `internal/setup/apply_test.go`
**Commit:** `fd045e52`
**Applied fix:** In `execute()`'s `!mutate` (Preview) branch, `res.Drift = c.notCompared` is now `res.Drift = boundCapture(c.notCompared)`, matching every other rendered field in the file (`renderClassification`'s `Drift`/`Registered`/`Reason`, `describeFailure`'s `Reason`, `toleratedNote`'s `Notes`). This is the exact fix REVIEW.md proposed, applied at the single call site (not duplicated inside `classifyProbe`, since `c.notCompared` is only ever rendered from this one call site today).

Also added `TestPreviewNotComparedDriftStaysBounded` (`internal/setup/apply_test.go`), built on the existing `fakeEnvWithRun`/`scriptedRun` harness (rule `m45p2b4bp7` — no real CLI invoked), which drives `Preview` through a `codex` probe #1 seam error (`scriptedResult{Err: ...}`) carrying a 100KB error string, and asserts `res.Drift` stays within `maxCapturedBytes + len(truncationMarker)` and never contains the unbounded text. This closes the exact coverage gap REVIEW.md identified (`TestDriftFieldsStayBoundedAgainstOversizedProbeContent` only covers the `renderClassification` fields, not this "not compared" preview path).

**Verification performed:**
- Confirmed the new test goes RED against the pre-fix code (temporarily reverted the one-line fix via `sed`, re-ran `TestPreviewNotComparedDriftStaysBounded`, observed `len(res.Drift) = 100087, want <= 4174` and the unbounded-content assertion failing, then restored the fix) before committing.
- `go build ./internal/setup/...` — clean.
- `go vet ./internal/setup/...` — clean.
- `go test ./internal/setup/ -run '^(TestApplyPreservedIssuesZeroWrites|TestApplyPreservedNeverRunsClaudeCodeRemove|TestApplyAlreadyCorrectIssuesZeroWrites|TestDriftFieldsStayBoundedAgainstOversizedProbeContent|TestPreviewNotComparedDriftStaysBounded)$' -count=1 -v` — all PASS (`mutate == true` gate semantics — `preserved`/`already-correct` returning before `plan.Actions[0]` — confirmed untouched).
- `go test ./internal/setup/ ./cmd/engram/ -count=1 -shuffle=on` — PASS.
- `go test ./internal/keylinks/ -count=1` — PASS.
- `task lint` — all checks passed (golangci-lint, rumdl, yamlfmt, actionlint, ruff).
- `go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1` — PASS after committing (all 22 red-evidence patches, including the three under `.planning/phases/04-drift-detection-read-only/red-evidence/` that touch `internal/setup/apply.go`, still apply cleanly and stay RED against the post-fix tree; no patch regeneration was needed).

## Skipped Issues

None — the single in-scope finding (WR-01) was fixed. IN-01 (Info severity) was excluded by `fix_scope: critical_warning` and left for a separate pass.

---

_Fixed: 2026-09-16T23:01:49Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
