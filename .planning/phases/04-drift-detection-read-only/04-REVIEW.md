---
phase: 04-drift-detection-read-only
reviewed: 2026-09-15T00:00:00Z
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
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-09-15T00:00:00Z
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Reviewed Phase 4's read-only drift-detection surface: `internal/setup`'s new `Compare`/`Observe`/`DriftRuntime` machinery (`drift.go`, `claudecode.go`, `codex.go`), the `apply.go` executor's new `!mutate` branch, `exit.go`/`aggregate.go`'s `OutcomePreserved` placement, and `cmd/engram/setup.go`'s row plumbing, plus the associated tests and docs.

The central security property (REQ-drift-redaction: an observed header/credential *value* must never reach any rendered field) holds by construction and is exercised thoroughly by `TestRedactionUnconditional` and its CLI-boundary mirror `TestSetupJSONNeverLeaksProbeLiteral` — I traced every place a `rawHeader.Value` or `doc.Transport.BearerTokenEnvVar`/`Authorization` line's raw text is used and confirmed each is compared-and-discarded inside the observing runtime's own `Observe` call frame, never copied onto `Observation`, `Drift`, or `Result`. D-09 (ambiguity → would-write), D-04 (`OutcomePreserved` as a non-failed `Classify` attempt), D-11 (claude-code's total parse — unrecognized lines become `preserved`, never silently dropped), and the "mutate branch of `execute()` is unchanged" claim all check out against the diff (`git diff 97c83d4^..HEAD -- internal/setup/apply.go` shows the mutate branch untouched). No test in the reviewed files invokes a real `claude`/`codex`/`opencode` binary or touches the real `$HOME` (rule m45p2b4bp7) — confirmed by grep across all reviewed `_test.go` files.

Two robustness gaps remain in the new `!mutate`/`DriftRuntime` path, both regressions relative to conventions this same package already established elsewhere (bounded captures via `boundCapture`/`maxCapturedBytes`, and the "each unrecognized field reported exactly once" discipline).

## Warnings

### WR-01: `Result.Registered`/`Drift`/`Reason` bypass the package's own capture-size bound in the new drift-compare path

**File:** `internal/setup/apply.go:326-359` (the `!mutate` branch of `execute`), `internal/setup/drift.go:342-460` (`renderObservation`, `Compare`)

**Issue:** `apply.go`'s own doc comment for `maxCapturedBytes` (apply.go:28-37) states the invariant this phase's new code silently drops: *"maxCapturedBytes bounds any third-party stdout/stderr capture placed on a rendered Result field (**Reason, Notes, Registered**) ... the mitigation for the 'third-party stdout becomes report content' threat ... a runtime that floods stdout could otherwise flood the operator's terminal or the --output json lane."* Before this phase, Preview's `Result.Registered` was built via `displayCapture(probe1.Stdout + probe1.Stderr)` (bounded to 4096 bytes, git diff confirms this was the pre-Phase-4 code). As of this phase, for any runtime implementing `DriftRuntime`, `Registered` is instead built via `renderObservation(obs)` (drift.go:342-359), and `Drift`/`Reason` are built via `strings.Join(d.Details, "; ")` / `strings.Join(d.Preserved, "; ") + "; " + obs.WholeEntryNote` (apply.go:354-357) — **none of these three assignments ever calls `boundCapture` or `displayCapture`**. `Compare`'s `Details` lines embed observed header *names* (drift.go:427,430,433 — taken verbatim from `h.Name`, itself sourced from `rawHeader.Name`, an untrusted string parsed straight out of `claude mcp get`'s stdout or `codex mcp get --json`'s `http_headers`/`env_http_headers` map keys) and the observed URL (`displayURL(obs.URL)`, drift.go:400) with no length cap at all. Every sibling capture path in this same package (Phase 2/3's `Reason`/`Notes` via `describeFailure`/`toleratedNote`, and Phase 3's `PluginResult.Installed`/`Source` via `boundCapture`, per `plugin.go:246,281` and `plugin_test.go:886-902`) is deliberately bounded against exactly this threat; this phase's new path is not, and `cmd/engram/setup.go`'s row copies these fields verbatim (`setupRuntimeRowFromResult`, setup.go:799-800) into the rendered report, where `sanitizeViewValue` (operator_view.go:223-234) strips only C0/DEL controls and applies no length bound either. A malicious or compromised MCP endpoint that claude-code's probe dials (the guide itself states "that read dials the configured URL", agent-setup.md:37-38), or a corrupted/adversarial `codex mcp get --json` response, can therefore inject an unbounded-length header name or URL into `Result.Drift`/`Result.Registered`/`Result.Reason`, flooding the operator's terminal or `--output json` output — precisely the threat `maxCapturedBytes` exists to close, now reopened for this one new path.

**Fix:** Route the observed-name and observed-URL components of `Compare`'s `Details`/`Preserved` lines, and `renderObservation`'s header-name/URL rendering, through `boundCapture` (or a shared bound with `unrecognizedLabelBound`'s 40-byte discipline, which codex.go's `Unrecognized` entries already use) before they are joined into `Details`/`Registered`. Minimally, apply `boundCapture` to the final `res.Drift`/`res.Registered`/`res.Reason` strings in `apply.go`'s `!mutate` branch, mirroring the mutate lane's own `res.Registered = displayCapture(probe2.Stdout + probe2.Stderr)` (apply.go:420) one code path below.

### WR-02: Codex's type-mismatch fallback double-reports a known field as unrecognized

**File:** `internal/setup/codex.go:416-430` (the `json.Unmarshal` error switch) and `internal/setup/codex.go:476-484` (the unconditional field-rules block)

**Issue:** When `codex mcp get --json` returns a document where a well-known field has the wrong JSON type (e.g. `"enabled": "true"` instead of a JSON boolean, or `"transport": {"type": 1, ...}`), `json.Unmarshal` records a `*json.UnmarshalTypeError` but — per Go's documented behavior — continues decoding the rest of the object, leaving the mismatched field at its Go zero value. `Observe`'s error-handling switch (codex.go:424-427) appends `typeErr.Field` (e.g. `"enabled"` or `"transport.type"`) to `unrecognized` for this case. Execution then falls through (no early return) to the unconditional field-rules block below, which independently re-derives the SAME finding from the now-zero-valued field: `if doc.Enabled == nil || !*doc.Enabled { unrecognized = append(unrecognized, "enabled") }` (codex.go:476-478) and `if doc.Transport.Type != "streamable_http" { unrecognized = append(unrecognized, "transport.type") }` (codex.go:482-484) both fire again, because a decode failure leaves `doc.Enabled == nil` / `doc.Transport.Type == ""`. The result is `Observation.Unrecognized` — and therefore `Result.Drift`/`Result.Reason` — containing the same label twice (e.g. `"unrecognized-content: enabled, enabled"` instead of `"unrecognized-content: enabled"`), degrading the operator-facing report's legibility. This is distinct from the "Totality gate" fixed-token guard (codex.go:466-471), which correctly checks `len(unrecognized) == 0` before adding its own fallback token — the two field-rule checks at codex.go:476 and 482 have no equivalent guard against the typeErr branch having already reported the same field. No test in `codex_test.go` exercises a type-mismatched `enabled` or `transport.type` value, so this path is unverified.

**Fix:** Either (a) `return Observation{}, false` immediately whenever `typeErr.Field` names one of the fields the later field-rules block independently tests (`enabled`, `transport.type`), since a type-mismatched core field is at least as unframeable as the existing `wrong-name`/`empty` cases that already return `false`; or (b) skip the corresponding field-rule check when `typeErr.Field` already recorded that same field; or (c) deduplicate `unrecognized` once, before it is bounded/quoted, e.g. via `slices.Compact` on a sorted copy.

## Info

### IN-01: Codex's dead header-comparison branch renders the wrong reference form

**File:** `internal/setup/codex.go:524-527`

**Issue:** `codexRuntime.Observe` builds the "planned" side of its header comparison as `plannedHeader{Name: h.Name, Value: h.EnvVar}` — the bare environment-variable name — whereas `claudeCodeRuntime.Observe`'s equivalent (claudecode.go:510-512) renders `"${" + h.EnvVar + "}"`, matching what that runtime's write path actually authors. `codexRuntime.Plan` unconditionally declines any `opts.Headers` (codex.go:94-102, `ErrHeaderUnsupported`), so `opts.Headers` is always empty on every reachable call and this branch never executes today — the doc comment says as much (codex.go:404-408). If a future change relaxes that restriction (e.g. codex adds a real header flag) without also fixing this rendering, `Compare`'s `HeaderMissing`/`HeaderDiffers` detail lines would show the wrong "would write" value (a bare name rather than whatever syntax codex's hypothetical header flag actually expects).

**Fix:** No functional change needed while the restriction holds; consider a comment at codex.go:524 making explicit that this rendering is a placeholder never exercised, so a future contributor who lifts the `ErrHeaderUnsupported` guard is prompted to revisit it.

---

_Reviewed: 2026-09-15T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
