---
phase: 17
slug: wired-write-handlers-full-crud-schedule
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-12
---

# Phase 17 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Validation Architecture lives in `17-RESEARCH.md`; the planner lifts per-task
> checks from it into each PLAN.md's `must_haves`.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` |
| **Config file** | none — `Taskfile.yaml` drives `task test`; golangci via `.golangci.yaml` |
| **Quick run command** | `go test ./internal/server/... ./internal/auth/... ./internal/webauth/...` |
| **Full suite command** | `task test` (and `task lint:go` — run explicitly per phase-15 executor blind-spot gotcha) |
| **Estimated runtime** | ~30–90 seconds (Qdrant-backed store tests skip without a live Qdrant; fake-store parity tests run in-process) |

---

## Sampling Rate

- **After every task commit:** Run the quick command scoped to the touched package.
- **After every plan wave:** Run `task test` then `task lint:go`.
- **Before `/gsd-verify-work`:** Full suite + `task lint:go` must be green.
- **Max feedback latency:** 90 seconds.

---

## Per-Task Verification Map

> Filled at Wave 0 / execution once PLAN.md task IDs exist. Derived from
> `17-RESEARCH.md` § Validation Architecture — the MCP/Connect parity table,
> the cross-owner existence-leak table, and the NO_SIDE_EFFECTS CI re-assertion.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 17-01-01 | 01 | 1 | REQ-connect-write-authz-parity | T-17-01 / — | fake store substitutable; nil-store test no longer panics | unit | `go test ./internal/server/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Fake `store` seam (narrow interface) so MCP/Connect parity tests can substitute a non-Qdrant store — prerequisite for the D-10 parity table.
- [ ] Shared parity-scenario table fixture (à la `TestRerankParityMCPAndConnect`) driving both lanes.

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Browser network-tab existence-leak check (DEC-xa6) | REQ-connect-write-authz-parity | Confirms no resolved-UUID leak reaches a real browser | Covered by automated cross-owner table test per by-id RPC; manual spot-check optional |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
