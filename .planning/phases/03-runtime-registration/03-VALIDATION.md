---
phase: "3"
slug: "runtime-registration"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-08"
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
| {N}-01-01 | 01 | 1 | REQ-{XX} | T-{N}-01 / — | {expected secure behavior or "N/A"} | unit | `{command}` | ✅ / ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Populated by `/gsd-validate-phase` once PLAN.md task IDs exist. Requirement→test mapping is pre-derived in `03-RESEARCH.md` § Validation Architecture.*

---

## Wave 0 Requirements

- [ ] A fake `Environment.Run` seam and its test helper (mirroring `fakeEnv` in `detect_test.go`) — the shared prerequisite every other Wave 0 test in this phase depends on.
- [ ] `TestApplyConverges` — two consecutive `--apply` runs, asserting `already-correct` on the second for **every** runtime including claude-code (REQ-setup-idempotent; the only shape that catches Pitfall 1).
- [ ] `TestClaudeCodePlan` — the 2-action `remove`→`add` argv sequence per auth mode (REQ-register-claude-code).
- [ ] `TestOpenCodeBearerHeaderSyntax` — regression test for the confirmed live `KEY=VALUE` bug; must assert `=` present and `: ` absent, not merely a URL substring (REQ-register-opencode).
- [ ] `TestGenericConfig` — valid minified JSON matching the `mcpServers` shape (REQ-register-generic-mcp).
- [ ] `TestNoSecretInArgs` — scans every `Action.Args` for the literal token value across every auth mode × runtime (REQ-register-auth-modes).
- [ ] `TestDriftReportedLegibly` — nonzero exit + stderr from the fake yields an `OutcomeFailed` row naming runtime, argv, and stderr verbatim (REQ-register-cli-surface-drift-legible).

*`TestCodexPlan` extends the existing `plan_test.go` (Command-string form) to the Args form — partial infrastructure exists.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real `claude`/`codex`/`opencode` CLI accepts the authored argv | REQ-register-claude-code, REQ-register-codex, REQ-register-opencode | Requires the three third-party CLIs installed at specific versions; not reproducible in CI | Install claude 2.1.265+, codex-cli 0.153.4+, opencode 1.18.20+; run `engram setup --apply` twice; confirm second run reports "already correct" per runtime |
| Runtime resolves `${VAR}` / `{env:VAR}` header references to the real secret at connect time | REQ-register-auth-modes | Verification requires a live MCP connection and a capturing HTTP endpoint | Point engram at a throwaway HTTP server, register with a bearer env-var reference, connect, assert the server observed the resolved token |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
