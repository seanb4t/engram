---
phase: "3"
slug: "runtime-registration"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-08"
validated: "2026-09-12"
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go stdlib `testing`) |
| **Config file** | none — no test framework config beyond `go.mod` |
| **Quick run command** | `go test ./internal/setup/... ./cmd/engram/... -run TestSetup -count=1` |
| **Full suite command** | `task` (lint + `go test ./...`) |
| **Estimated runtime** | ~30 seconds (quick) / ~180 seconds (full) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/setup/... ./cmd/engram/... -run TestSetup -count=1`
- **After every plan wave:** Run `task`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 3-01-01 | 01 | 1 | REQ-register-codex | — | N/A | unit | `go test ./internal/setup/ -run 'TestApplyConvergesCodex' -count=1` | ✅ | ✅ green |
| 3-01-02 | 01 | 1 | REQ-register-cli-surface-drift-legible | — | Nonzero exit + stderr yields a failed row naming runtime/argv/stderr | unit | `go test ./internal/setup/ -run 'TestDriftReportedLegibly' -count=1` | ✅ | ✅ green |
| 3-01-03 | 01 | 1 | REQ-register-auth-modes | T-3-01 | No secret literal in any `Action.Args`; oauth/none author identical argv | unit | `go test ./internal/setup/ -run 'TestQuoteWord\|TestNoSecretInArgs\|TestOAuthAndNoneAuthorIdenticalArgs' -count=1` | ✅ | ✅ green |
| 3-02-01 | 02 | 2 | REQ-setup-idempotent | — | Second `--apply` reports already-correct; tolerant clear-slot failure; fatal add failure | unit | `go test ./internal/setup/ -run 'TestApplyConvergesClaudeCode\|TestApplyToleratesClearSlotFailure\|TestApplyFailsWhenRegistrationActionFails' -count=1` | ✅ | ✅ green |
| 3-02-02 | 02 | 2 | REQ-register-claude-code | T-3-01 | Bearer header is an env-var reference, never a value | unit | `go test ./internal/setup/ -run 'TestClaudeCodePlan\|TestClaudeCodeBearerHeaderIsAnEnvVarReference\|TestNoSecretInArgs' -count=1` | ✅ | ✅ green |
| 3-02-03 | 02 | 2 | REQ-register-auth-modes | — | Action tolerance is authored, not positional | unit | `go test ./internal/setup/ -run 'TestActionToleranceIsAuthoredNotPositional' -count=1` | ✅ | ✅ green |
| 3-03-01 | 03 | 2 | REQ-register-auth-modes | T-3-01 | opencode `KEY=VALUE` header syntax regression; header carries no secret; oauth-client rejected explicitly | unit | `go test ./internal/setup/ -run 'TestOpenCodePlan\|TestOpenCodeBearerHeaderSyntax\|TestOpenCodeBearerHeaderCarriesNoSecret' -count=1` | ✅ | ✅ green |
| 3-03-02 | 03 | 2 | REQ-register-opencode / REQ-setup-idempotent | — | Safe-direction-only convergence (live `mcp list` dials servers) | unit | `go test ./internal/setup/ -run 'TestApplyOpenCodeConvergence' -count=1` | ✅ | ✅ green |
| 3-04-01 | 04 | 3 | REQ-register-generic-mcp | — | Portable `mcpServers` JSON; starts no process | unit+integration | `go test ./internal/setup/ ./cmd/engram/ -run 'TestGenericConfig\|TestGenericStartsNoProcess\|TestSetupGenericRowCarriesPortableConfig' -count=1` | ✅ | ✅ green |
| 3-04-02 | 04 | 3 | REQ-register-generic-mcp | — | Generic is opt-in; excluded from the bare default set | unit+integration | `go test ./internal/setup/ ./cmd/engram/ -run 'TestSelectDefaultSetExcludesOptInRuntimes\|TestSetupBareInvocationOmitsGeneric' -count=1` | ✅ | ✅ green |
| 3-04-03 | 04 | 3 | REQ-register-auth-modes | T-3-01 | Generic config carries no secret; partial exit with a failing sibling | unit+integration | `go test ./internal/setup/ ./cmd/engram/ -run 'TestGenericConfigCarriesNoSecret\|TestNoSecretInArgs\|TestSetupGenericAndFailingRuntimeExitsPartial' -count=1` | ✅ | ✅ green |
| 3-05-01 | 05 | 4 | REQ-setup-idempotent | — | Preview reports registered state; never classifies already-correct; probe failure is exit 0 in preview | unit+integration | `go test ./internal/setup/ ./cmd/engram/ -run 'TestPreviewReportsRegisteredState\|TestSetupPreviewExitsZeroWhenProbeFails\|TestSetupPreviewNeverClassifiesAlreadyCorrect' -count=1` | ✅ | ✅ green |
| 3-05-02 | 05 | 4 | REQ-register-auth-modes | — | `token_file` marked ignored for native runtimes; path never duplicated into the marker | integration | `go test ./cmd/engram/ -run 'TestSetupTokenFileMarkedIgnoredForNativeRuntimes\|TestSetupNoTokenFileLeavesNoMarker\|TestSetupTokenFilePathNotDuplicatedIntoMarker\|TestSetupHelpNamesEveryRuntimeAndAuthMode' -count=1` | ✅ | ✅ green |
| 3-05-03 | 05 | 4 | REQ-register-cli-surface-drift-legible | — | Partial exit is live-producible; destructive flag set pinned | integration | `go test ./cmd/engram/ -run 'TestSetupPartialExitIsLiveProducible\|TestDestructiveCommandsExactFlagSet' -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Populated by `/gsd-validate-phase` on 2026-09-12 from 03-01..03-05 PLAN/SUMMARY coverage blocks. Note: 03-05-SUMMARY cites `TestDestructiveCommandsExactFlagSet` under `cmd/engram/setup_test.go`; it lives in `cmd/engram/destructive_test.go` (same package, same `-run` reach).*

---

## Wave 0 Requirements

- [x] A fake `Environment.Run` seam and its test helper (mirroring `fakeEnv` in `detect_test.go`) — the shared prerequisite every other Wave 0 test in this phase depends on.
- [x] `TestApplyConverges*` — two consecutive `--apply` runs, asserting `already-correct` on the second for **every** runtime including claude-code (REQ-setup-idempotent; landed as `TestApplyConvergesCodex`, `TestApplyConvergesClaudeCode`, `TestApplyOpenCodeConvergence`).
- [x] `TestClaudeCodePlan` — the 2-action `remove`→`add` argv sequence per auth mode (REQ-register-claude-code).
- [x] `TestOpenCodeBearerHeaderSyntax` — regression test for the confirmed live `KEY=VALUE` bug (REQ-register-opencode).
- [x] `TestGenericConfig` — valid minified JSON matching the `mcpServers` shape (REQ-register-generic-mcp).
- [x] `TestNoSecretInArgs` — scans every `Action.Args` for the literal token value across every auth mode × runtime (REQ-register-auth-modes).
- [x] `TestDriftReportedLegibly` — nonzero exit + stderr from the fake yields an `OutcomeFailed` row naming runtime, argv, and stderr verbatim (REQ-register-cli-surface-drift-legible).

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real `claude`/`codex`/`opencode` CLI accepts the authored argv | REQ-register-claude-code, REQ-register-codex, REQ-register-opencode | Requires the three third-party CLIs installed at specific versions; not reproducible in CI | Install claude 2.1.265+, codex-cli 0.153.4+, opencode 1.18.20+; run `engram setup --apply` twice; confirm second run reports "already correct" per runtime |
| Runtime resolves `${VAR}` / `{env:VAR}` header references to the real secret at connect time | REQ-register-auth-modes | Verification requires a live MCP connection and a capturing HTTP endpoint | Point engram at a throwaway HTTP server, register with a bearer env-var reference, connect, assert the server observed the resolved token |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-09-12 by `/gsd-validate-phase 3`

---

## Validation Audit 2026-09-12

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All 6 Phase 3 requirements are COVERED by committed tests; every SUMMARY-referenced test function exists and `go test ./internal/setup/ ./cmd/engram/ -count=1` is green at `e9cf19dd`. The two Manual-Only rows are retained: both require live third-party CLIs or a live MCP connection, which rule `m45p2b4bp7` keeps out of automated tests; the live `--apply` round trips are recorded in `03-UAT.md` (3/3). Open warning W01 (#560, `osRun` deadline classification) is a milestone-audit tech-debt item, not a coverage gap for any Phase 3 requirement.
