---
phase: 12
slug: per-memory-usage-signals
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-10
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib + testcontainers-go for Qdrant-backed tests) |
| **Config file** | none — Go modules; `Taskfile.yaml` targets |
| **Quick run command** | `go test ./internal/store/... ./internal/server/... ./internal/config/...` |
| **Full suite command** | `task test` (lint + test via `task`) |
| **Estimated runtime** | ~30–120 s (Qdrant testcontainer spins up once per package that needs it; Qdrant-free unit tests are sub-second) |

---

## Sampling Rate

- **After every task commit:** Run the package-scoped `go test ./internal/<pkg>/...` for the touched package
- **After every plan wave:** Run the full suite command
- **Before `/gsd-verify-work`:** `task` (lint + test) must be green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

> Populated once PLAN.md tasks exist. Anchored to the research's Validation Architecture seams:
> injectable clock (`store.WithClock`, store.go~181), a Wait/Idle drain seam on the async
> incrementer (mirrors `summaryqueue.go`), and Qdrant-free negative-space tests.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 12-01-01 | 01 | 1 | REQ-usage-signals | T-12-* / — | `access_count`/`last_accessed_at` round-trip through `payload()`/`fromPayload()` losslessly (uint64 → Qdrant IntegerValue) | unit | `go test ./internal/store/...` | ❌ W0 | ⬜ pending |
| 12-0x-xx | — | — | REQ-usage-signals | — | get-by-id + update increment; **search/list/list_scheduled do NOT** (negative-space) | unit | `go test ./internal/server/...` | ❌ W0 | ⬜ pending |
| 12-0x-xx | — | — | REQ-usage-signals | — | reranker output invariant under `access_count` (D-08 ranking isolation) | unit | `go test ./internal/store/...` | ❌ W0 | ⬜ pending |
| 12-0x-xx | — | — | REQ-usage-signals | — | async get-path incrementer: drop-on-full, no `get_memory` error under Qdrant outage; deterministic via Wait/Idle drain seam | unit | `go test ./internal/server/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Test files for the new `IncrementAccess` store method + async incrementer (Qdrant-free, injectable-fill seam)
- [ ] Negative-space test asserting counters never fire on search/list membership
- [ ] Ranking-isolation test asserting `SearchReranked` ignores `access_count`

*Existing infrastructure (`go test`, store_test.go Qdrant testcontainer, `store.WithClock`) covers the rest — no new framework install.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Recall ids surface on OTLP spans → ClickStack (D-06) | REQ-usage-signals | Requires a live OTLP collector / ClickStack; the unit layer asserts the span attributes are SET, not that ClickStack ingests them | Run server with `ENGRAM_OTLP_ENDPOINT` set; issue `search_memory`; confirm `engram.recall.ids`/`engram.recall.count` on the recall span in ClickStack |

*The span-attribute emission itself is unit-testable via an in-memory span recorder; only the end-to-end ClickStack analytics view is manual.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
