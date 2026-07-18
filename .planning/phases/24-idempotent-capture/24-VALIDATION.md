---
phase: 24
slug: idempotent-capture
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-18
---

# Phase 24 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib `testing`) |
| **Config file** | none — `Taskfile.yaml` (`task test`) |
| **Quick run command** | `go test ./internal/server/... ./internal/store/...` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~60–120 seconds (store tests exercise a real Qdrant per `internal/store` bench/integration convention) |

---

## Sampling Rate

- **After every task commit:** Run `{quick run command}`
- **After every plan wave:** Run `{full suite command}`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 120 seconds

---

## Per-Task Verification Map

> Seeded scaffold — the planner populates one row per task, mapping each phase task to a
> success criterion (SC1–SC5) and threat ref. Reference SC coverage:
> SC1 replay→same record · SC2 mismatch→distinct error · SC3 two-owners-same-key matrix ·
> SC4 concurrent-identical→one point (`-race`) · SC5 no-key→fresh-every-time.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| {N}-01-01 | 01 | 1 | REQ-idempotent-capture | T-24-01 / — | {expected secure behavior or "N/A"} | unit | `{command}` | ✅ / ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] {test file(s)} — stubs for REQ-idempotent-capture
- [ ] {shared fixtures if needed}

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| {behavior} | REQ-idempotent-capture | {reason} | {steps} |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
