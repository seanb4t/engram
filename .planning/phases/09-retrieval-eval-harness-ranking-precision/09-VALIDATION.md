---
phase: 9
slug: retrieval-eval-harness-ranking-precision
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-07-09
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (env-gated retrieval eval, mirrors the `eval:summary` precedent) |
| **Config file** | none — `Taskfile.yaml` `eval:retrieval` target added in Wave 0 |
| **Quick run command** | `go test ./internal/... -run <unit-under-change>` |
| **Full suite command** | `task test` (lint+test); gated eval: `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval` |
| **Estimated runtime** | unit ~seconds; gated eval requires a live Qdrant testcontainer + embedder gateway |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/<pkg>/...` for the touched package
- **After every plan wave:** Run `task test` (full lint+test)
- **Before `/gsd-verify-work`:** Full suite green + `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval` passes (recall@k / MRR at/above baseline, #261 fixture green)
- **Max feedback latency:** unit path < 60s (the gated eval is opt-in / integration-tier, not on the per-commit hot path)

---

## Per-Task Verification Map

> Populated by the planner (`must_haves` + `<acceptance_criteria>` per task) and the nyquist auditor once PLAN.md wave/task IDs exist.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| {N}-01-01 | 01 | 1 | REQ-retrieval-eval | T-9-01 / — | Eval harness runs and reports recall@k / MRR deterministically | integration | `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `Taskfile.yaml` — add `eval:retrieval` target (env-gated, mirrors `eval:summary`)
- [ ] retrieval-eval package + labeled dataset (incl. #261 Query A/B → Record T regression fixture)
- [ ] reuse `internal/store/store_test.go` Qdrant testcontainer provisioning (no new infra)

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| — | — | — | — |

*All phase behaviors target automated verification via the eval harness; the planner confirms no manual-only gaps remain.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 60s (unit path)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-07-09 (gsd-plan-checker: VERIFICATION PASSED — Dimension 8 green; `wave_0_complete` flips at execution)
