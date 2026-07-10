---
phase: 11
slug: async-on-write-summaries
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-09
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go standard testing; table-driven) |
| **Config file** | none — `Taskfile.yaml` orchestrates; qdrant testcontainer already vendored (`internal/store` tests) |
| **Quick run command** | `go test ./internal/server/... ./internal/config/...` |
| **Full suite command** | `task` (lint + test) |
| **Estimated runtime** | ~5s quick (pure-unit, no container) · ~60s full (`task`, includes qdrant testcontainer) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/server/... ./internal/config/...`
- **After every plan wave:** Run `task` (lint + test)
- **Before `/gsd-verify-work`:** Full suite must be green (`task` exits 0)
- **Max feedback latency:** ~5 seconds (quick run)

---

## Per-Task Verification Map

> Populated by the planner (each task's `<verify>`) and reconciled by `/gsd-validate-phase` once PLAN.md tasks exist. Async-path tests must be deterministic — assert via a drain/`Wait` test seam, never `time.Sleep`.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | REQ-async-summaries | T-11-xx / — | write path returns even when summarizer fails | unit | `go test ./internal/server/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- Existing infrastructure covers all phase requirements — `go test` + the qdrant testcontainer harness (`internal/store`) are already present; the async worker adds a **drain/`Wait` test seam** (an exported completion signal) so async fill can be awaited deterministically without `time.Sleep`.

*New test files (created during execution, not Wave 0 scaffolding): worker-pool/queue unit tests under `internal/server` (or the new queue package) covering write-path-never-blocks, drop-and-count under saturation, idempotent fill after async drain, and best-effort shutdown drain.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Summary-fidelity gate (`task eval:summary`) | REQ-async-summaries | D-02: MANUAL operator gate, **not** CI — calls a live LLM gateway (cost + non-determinism); gates broad auto-enablement of `ENGRAM_SUMMARY_ON_WRITE` | Operator runs `task eval:summary` against a configured `ENGRAM_SUMMARY_MODEL`; inspect summary quality before recommending default-on |
| Live async fill vs. prod gateway | REQ-async-summaries | Prod qwen3-embedding-8b sees 529 brownouts + hardcoded 30s embed timeout — real-gateway behavior can't run in hermetic CI | Operator sets `ENGRAM_SUMMARY_ON_WRITE=true` + `ENGRAM_SUMMARY_MODEL`, stores a record, confirms summary is filled out-of-band and the store call returned immediately |

*Structural/behavioral tests (write-path-never-blocks, drop-and-count, shutdown drain, idempotency) are all CI-automated — only the live-LLM fidelity gate is manual.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
