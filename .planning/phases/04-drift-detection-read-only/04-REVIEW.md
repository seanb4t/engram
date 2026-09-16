---
phase: 04-drift-detection-read-only
reviewed: 2026-09-16T01:30:00Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - cmd/engram/agent_setup_docs_test.go
  - cmd/engram/operator_view_setup_test.go
  - cmd/engram/setup_test.go
  - cmd/engram/setup.go
  - cmd/engram/testdata/help.golden
  - docs-site/src/content/docs/guides/agent-setup.md
  - internal/setup/aggregate_test.go
  - internal/setup/aggregate.go
  - internal/setup/apply_test.go
  - internal/setup/apply.go
  - internal/setup/claudecode_test.go
  - internal/setup/claudecode.go
  - internal/setup/codex_test.go
  - internal/setup/codex.go
  - internal/setup/drift_test.go
  - internal/setup/drift.go
  - internal/setup/exit_test.go
  - internal/setup/exit.go
  - internal/setup/plan.go
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
status: clean
---

# Phase 04: Code Review Report (re-review, iteration 2)

**Reviewed:** 2026-09-16T01:30:00Z
**Depth:** standard
**Files Reviewed:** 19
**Status:** clean

## Summary

This is a re-review after the fix pass for `04-REVIEW.iter2.md`'s two Warning findings (WR-01, WR-02). Both fixes were verified independently, not just trusted from the fixer's report.

**WR-01** (`internal/setup/apply.go:326-374`): confirmed via `git diff 4dfec584..HEAD -- internal/setup/apply.go` that `res.Drift`, `res.Registered`, and `res.Reason` are now each routed through `boundCapture` immediately after being built from `Compare`/`renderObservation`, closing the "unbounded observed header name/URL floods the operator's terminal or `--output json`" gap. The D-03 key-link statement `res.Registered = renderObservation(obs)` still exists verbatim as its own statement, with `boundCapture` applied as a separate step after it — `go test ./internal/keylinks/...` passes, confirming the key-link gate is satisfied. `git diff ff5a6f94 HEAD -- internal/setup/apply.go` confirms the `mutate == true` branch is untouched by the whole phase (nothing after the `if !mutate { ... }` block differs). The new `TestDriftFieldsStayBoundedAgainstOversizedProbeContent` (a scripted, non-real-binary probe carrying a 100KB header name and a 100KB URL) passes and correctly demonstrates the bound holds even under `OutcomePreserved`, which populates all three fields at once.

**WR-02** (`internal/setup/codex.go:413-508`): confirmed the fix adds `typeErrField` to record which single field (if any) the `*json.UnmarshalTypeError` branch already reported, and gates the two later field-rules checks (`enabled`, `transport.type`) so they don't re-derive the same finding from the now-zero-valued field. Traced the struct definitions (`codexRegistrationDoc`/`codexRegistrationTransport`, codex.go:317-341): `enabled` (`*bool`) and `transport.type` (`string`) are the *only* two fields capable of raising an `UnmarshalTypeError` while also being independently tested in the field-rules block — every other field-rules-checked field (`disabled_reason`, `transport.http_headers_helper`, `enabled_tools`, `disabled_tools`, `startup_timeout_sec`, `tool_timeout_sec`) is typed `json.RawMessage`, which never produces a type error regardless of the JSON shape supplied. So the two-field guard is complete, not a partial fix that happens to pass its own tests. The new table-driven subtests (`enabled-type-mismatch`, `transport-type-type-mismatch`) pass and assert exactly one entry in `Unrecognized`.

Ran `go build ./...`, `go test ./internal/setup/... ./cmd/engram/... -count=1`, and `go test ./internal/keylinks/... -count=1` independently — all pass. Confirmed rule m45p2b4bp7 compliance by grep across every `_test.go` file in scope: no `exec.Command`/`LookPath` call against a real `claude`/`codex`/`opencode` binary, and every `$HOME` mention is a comment explaining why the test uses a fake environment instead of touching the real one. No debug artifacts (`console.log`, `TODO`/`FIXME`/`HACK`, empty catch equivalents) found in the non-test scope files. `cmd/engram/setup.go`'s row plumbing (`setupRuntimeRowFromResult` and its JSON sibling) copies `Reason`/`Registered`/`Drift` verbatim from `Result` — now safe given both fields are bounded upstream in `apply.go`.

No new issues were introduced by the fix commits (`e303086f`, `ed9092ab`, `e3a7c82f`) — they touched only `internal/setup/apply.go`, `internal/setup/apply_test.go`, `internal/setup/codex.go`, `internal/setup/codex_test.go`. IN-01 (below) was out of the fixer's scope (Info-tier) and is carried forward unescalated, as instructed.

## Info

### IN-01: Codex's dead header-comparison branch renders the wrong reference form (carried forward, unchanged)

**File:** `internal/setup/codex.go:534` (line moved from 524-527 in the prior review due to intervening WR-02 comment insertions; same statement)
**Issue:** `codexRuntime.Observe` builds the "planned" side of its header comparison as `plannedHeader{Name: h.Name, Value: h.EnvVar}` — the bare environment-variable name — whereas `claudeCodeRuntime.Observe`'s equivalent (`claudecode.go:510-512`) renders `"${" + h.EnvVar + "}"`, matching what that runtime's write path actually authors. `codexRuntime.Plan` unconditionally declines any `opts.Headers` (codex.go:94-102, `ErrHeaderUnsupported`), so `opts.Headers` is always empty on every reachable call and this branch never executes today. If a future change relaxes that restriction without also fixing this rendering, `Compare`'s `HeaderMissing`/`HeaderDiffers` detail lines would show the wrong "would write" value.
**Fix:** No functional change needed while the restriction holds; consider a comment at codex.go:534 making explicit that this rendering is a placeholder never exercised, so a future contributor who lifts the `ErrHeaderUnsupported` guard is prompted to revisit it.

---

_Reviewed: 2026-09-16T01:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
