---
phase: 11
slug: async-on-write-summaries
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-09
validated: 2026-07-10
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Reconciled against the 3 finalized plans (11-01/02/03) after the plan-checker passed Nyquist Dimension 8 (2026-07-09).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go standard testing; table-driven, `-race` on async tests) |
| **Config file** | none — `Taskfile.yaml` orchestrates; qdrant testcontainer already vendored (`internal/store` tests). Async tests are Qdrant-free via the injectable `fill func(ctx, id) error` seam |
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

> Task IDs are positional (`11-{plan}-{NN}`). Async-path tests are deterministic — they assert via the `Wait`/`Idle` drain seam, never `time.Sleep` (acceptance criteria grep for zero `time.Sleep`).

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 11-01-01 | 01 | 1 | REQ-async-summaries | T-11-01 | invalid `ENGRAM_SUMMARY_*` rejected at startup; D-01 AND-gate is parseable, worker absent unless both switches set | unit | `go test ./internal/config/... -run 'TestValidate\|TestSummary' -v` | ✅ | ✅ green |
| 11-01-02 | 01 | 1 | REQ-async-summaries | T-11-01 / T-11-SC | queue-depth gauge + fill-latency histogram + enqueued/dropped/failed/retried counters register on the shared meter; `backoff/v5` promoted to direct, `go mod verify` clean | build/vet | `go build ./... && go vet ./internal/telemetry/... && go mod verify` | ✅ | ✅ green |
| 11-02-01 | 02 | 2 | REQ-async-summaries | T-11-01 / T-11-02 / T-11-04 | bounded channel; non-blocking `tryEnqueue` (`select`/`default`); `WaitGroup`+ctx drain; bounded `backoff.WithMaxTries` retry (no infinite loop) | build/vet | `go build ./internal/server/... && go vet ./internal/server/...` | ✅ | ✅ green |
| 11-02-02 | 02 | 2 | REQ-async-summaries | T-11-01/02/03/04 | never-blocks-write, drops-when-full (drop+count), retry-gives-up (bounded wall time), shutdown-drains-within-budget — all via `Wait` seam, no `time.Sleep` | unit (`-race`) | `go test ./internal/server/... -run 'TestSummaryQueue' -race -v` | ✅ | ✅ green |
| 11-03-01 | 03 | 3 | REQ-async-summaries | T-11-02 / T-11-05 | enqueue fired only post-Upsert on `store_memory`/`schedule_memory`, behind `ENGRAM_SUMMARY_ON_WRITE && ENGRAM_SUMMARY_MODEL`; `store_discovery`/`store_rule` excluded (D-06) | build/vet | `go build ./internal/server/... && go vet ./internal/server/...` | ✅ | ✅ green |
| 11-03-02 | 03 | 3 | REQ-async-summaries | T-11-04 | worker pool started at serve; drain invoked STRICTLY after `httpSrv.Shutdown(shutdownCtx)` returns (no send-on-closed-channel) | build/vet | `go build ./... && go vet ./cmd/engram/... ./internal/server/... ./internal/telemetry/...` | ✅ | ✅ green |
| 11-03-03 | 03 | 3 | REQ-async-summaries | T-11-02 / T-11-05 | `store_memory` returns even when the summarizer hangs; discovery/rule never enqueue; docs describe the knobs + the manual (not-CI) `task eval:summary` gate | unit (`-race`) + docs | `go test ./internal/server/... -run 'TestStoreMemory\|TestDiscoveryAndRuleNeverEnqueue' -race -v` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky · `File Exists` flips ✅ as each new file lands during execution.*

---

## Wave 0 Requirements

- Existing infrastructure covers all phase requirements — `go test` + the qdrant testcontainer harness (`internal/store`) are already present. The async worker adds a **`Wait`/`Idle` drain seam** (an exported completion signal on `summaryQueue`) so async fill is awaited deterministically without `time.Sleep`, and an injectable `fill func(ctx, id) error` closure so the drop/never-block/retry/drain tests run **without Qdrant**.

*New test files created during execution (not Wave 0 scaffolding): `internal/server/summaryqueue_test.go` (queue behaviors) and handler-level enqueue tests in `internal/server/tools_test.go`.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Summary-fidelity gate (`task eval:summary`) | REQ-async-summaries | D-02: MANUAL operator gate, **not** CI — calls a live LLM gateway (cost + non-determinism); gates broad default-on of `ENGRAM_SUMMARY_ON_WRITE` | Operator runs `task eval:summary` (`ENGRAM_SUMMARY_EVAL=1 go test ./internal/summarize/ -run TestSummaryFidelity -v`) against a configured `ENGRAM_SUMMARY_MODEL`; judge summary quality before recommending default-on |
| Live async fill vs. prod gateway | REQ-async-summaries | Prod qwen3-embedding-8b sees 529 brownouts + hardcoded 30s embed timeout — real-gateway behavior can't run in hermetic CI | Operator sets `ENGRAM_SUMMARY_ON_WRITE=true` + `ENGRAM_SUMMARY_MODEL`, stores a record, confirms the summary is filled out-of-band and the store call returned immediately |

*Structural/behavioral tests (write-path-never-blocks, drop-and-count, retry-gives-up, shutdown drain, discovery/rule exclusion) are all CI-automated — only the live-LLM fidelity gate is manual.*

---

## Validation Audit 2026-07-10

Post-execution reconcile (State A) via `/gsd-validate-phase 11`. Each task's plan-time
automated command was re-run against the shipped code — all green, no gaps, no
auditor/test-generation needed.

| Metric | Count |
|--------|-------|
| Tasks in map | 7 |
| Covered (green automated) | 7 |
| Partial | 0 |
| Missing (gaps) | 0 |
| Manual-only (by design) | 2 (`task eval:summary` fidelity gate + live-gateway async fill) |

Notes:
- Verified commands: `go test ./internal/config/...` (config/summarize validation), `go build ./... && go vet ./internal/telemetry/... && go mod verify` (telemetry + backoff-direct), `go test ./internal/server/... -run TestSummaryQueue -race` (queue behaviors), `go test ./internal/server/... -run 'TestStoreMemory…|TestDiscoveryAndRuleNeverEnqueue' -race` (wiring + negative-space), `go test ./internal/store/... -run 'TestSummarizeMissingEmitsEgressAuditLog…'` (T-11-06 egress audit).
- **Coverage strengthened during execution:** the code-review fixes added two `-race` regressions under task 11-02-02 — `TestSummaryQueueEnqueueAfterShutdownIsDroppedNotPanic` (CR-01: late enqueue after channel close is dropped, not a panic) and `TestSummaryQueueRetryBudgetAccommodatesFullTryCount` (WR-02: retry wall-time budget cannot pre-empt the try count).
- Manual-only items are by-design (D-02: `eval:summary` is a MANUAL operator gate, not CI; live-gateway async fill can't run hermetically) — not coverage gaps.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify (all 7 tasks carry an automated verify)
- [x] Wave 0 covers all MISSING references (none — existing infra + the new drain/fill seams)
- [x] No watch-mode flags
- [x] Feedback latency < 5s (quick run)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** verified 2026-07-10 (re-reconciled post-execution — all 7 tasks green, 0 gaps; see Validation Audit above). Plan-time approval: 2026-07-09 (plan-checker Nyquist Dimension-8 pass).
