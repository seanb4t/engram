---
phase: 12
slug: per-memory-usage-signals
status: approved
nyquist_compliant: true
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

> Finalized against the 6 committed plans (12-01..12-06, commit 38ae1cff). Anchored to the
> research's Validation Architecture seams: injectable clock (`store.WithClock`, store.go~181),
> a Wait/Idle drain seam on the async incrementer (mirrors `summaryqueue.go`), and Qdrant-free
> negative-space tests. Each row's command is package-scoped for quick per-task feedback; the
> terminal gate is `task` (lint + test) in 12-06.

| Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | Status |
|------|------|-------------|------------|-----------------|-----------|-------------------|--------|
| 12-01 | 1 | REQ-usage-signals | T-12-01/02 | `access_count`/`last_accessed_at` round-trip through `payload()`/`fromPayload()` losslessly (uint64 → Qdrant IntegerValue); `IncrementAccess` via `SetPayload`; free `Update` bump; **`RerankHits` invariant under `access_count`** (D-08) | unit | `go test ./internal/store/...` | ⬜ pending |
| 12-02 | 1 | REQ-usage-signals | — | `ENGRAM_USAGE_SIGNALS` parses (default on); `UsageQueueMetrics` instruments construct | unit | `go test ./internal/config/... ./internal/telemetry/...` | ⬜ pending |
| 12-03 | 1 | REQ-usage-signals | T-12-SC | proto `Memory` fields 19/20 additive; `gen/` regenerated & committed (CI drift-check reproduced) | build/gen | `task proto:lint && task proto:gen && git diff --exit-code -- gen/` | ⬜ pending |
| 12-04 | 2 | REQ-usage-signals | — | `engram.recall.ids` (bounded slice) + `engram.recall.count` on `store.Search`/`List`/`Get` spans; **not** `instrumentTools` | unit | `go test ./internal/store/...` | ⬜ pending |
| 12-05 | 2 | REQ-usage-signals | T-12-04/05 | async incrementer: drop-on-full, no retry, no `get_memory` error under Qdrant outage; CR-01 closed-guard; deterministic via Wait/Idle drain seam (Qdrant-free) | unit | `go test ./internal/server/...` | ⬜ pending |
| 12-06 | 3 | REQ-usage-signals | T-12-11 | get + Connect `GetMemory` enqueue (call-and-ignore); D-07 `recallView`/`memoryToProto` exposure; **D-02 negative-space: zero enqueues from search/list/list_scheduled** | unit + e2e | `go test ./internal/server/...` then `task` | ⬜ pending |

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

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (confirmed by plan-checker across 12-01..12-06)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (no MISSING refs — existing `go test` infra + `store.WithClock` suffice)
- [x] No watch-mode flags
- [x] Feedback latency < 120s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-07-10 (plan-checker VERIFICATION PASSED; Dimension 8 satisfied in substance by the finalized plans)
