---
phase: 11
slug: async-on-write-summaries
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on (high) severity
threats_open: 0
asvs_level: 1
created: 2026-07-10
---

# Phase 11 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Async-on-write summaries add an in-process worker pool that drains record ids
> through `Store.FillSummary` off the synchronous write path.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| HTTP client → MCP handler | `store_memory` / `schedule_memory` tool calls that (post-Upsert) enqueue a record id for async summary fill | Authenticated request; only the record id crosses into the queue |
| engram → summarizer gateway | New SECOND outbound egress path (async worker → `/v1/chat/completions`), parallel to the existing `summarize-missing` sweep | Already-authorized record content → gateway; audited content-free |
| Process config → runtime | `ENGRAM_SUMMARY_ON_WRITE` / `_WORKERS` / `_QUEUE_SIZE` (+ reused `_MODEL` / `_TIMEOUT`) parsed at point-of-use | Operator config only; no external input |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-11-01 | Denial of Service | Bounded enqueue channel (`ENGRAM_SUMMARY_QUEUE_SIZE`) | high | mitigate | Channel constructed with finite capacity (`make(chan string, queueSize)`, summaryqueue.go:99) from a `Validate()`-checked positive int; non-blocking drop-and-count on saturation (`default:` → `Dropped`, :133/:174). AND-gate leaves the queue nil unless opted in. Test: `TestSummaryQueueDropsWhenFull`. | closed |
| T-11-02 | Denial of Service | Write-path availability (`tryEnqueue` at `storeMemory`/`scheduleMemory`) | high | mitigate | Enqueue is a non-blocking `select`/`default` fired only post-Upsert; the handler returns unconditionally even with the summarizer down (SC#2). Tests: `TestSummaryQueueNeverBlocksWrite`, `TestStoreMemoryReturnsWhenSummarizerHangs`. | closed |
| T-11-03 | Denial of Service | Retry loop + hung summarizer attempt (529 brownout) | medium | mitigate | Per-attempt `context.WithTimeout(ctx, q.attemptTimeout)` (summaryqueue.go:228, reuses `ENGRAM_SUMMARY_TIMEOUT`) cuts a hung `/v1/chat/completions` call; `WithMaxTries` + `WithMaxElapsedTime` (:233/:234) bound the retry budget. **Strengthened** in this run: `maxElapsed` is now derived from `attemptTimeout` (WR-02 fix, `40ac40ca`) so the wall-time cap can no longer collapse the try count. Tests: `TestSummaryQueueRetryGivesUp`, `TestSummaryQueueHungFillIsInterrupted`. | closed |
| T-11-04 | Denial of Service (crash) | Shutdown ordering + worker panic (send-on-closed-channel) | high | mitigate | Drain runs STRICTLY after `httpSrv.Shutdown` returns, sequential (serve.go:216→:223); a panicking fill is recovered with balanced `defer itemDone()` (summaryqueue.go:211/:213). **Code review (CR-01) proved the plan's ordering-only mitigation insufficient** — `net/http.Server.Shutdown` does not force-terminate handlers on ctx-deadline, so a slow handler could still send after close. Closed robustly by guarding the close with `sync.RWMutex`+`closed` (RLock on send :156/:158, Lock on close :286–:292; fix `c2075697`). Tests: `TestSummaryQueueShutdownDrainsWithinBudget`, `TestSummaryQueuePanicDoesNotWedgeWait`, `TestSummaryQueueEnqueueAfterShutdownIsDroppedNotPanic` (all `-race`). | closed |
| T-11-05 | Information Disclosure / Elevation | Enqueue scope + worker re-fetch | low | accept | Only `store_memory`/`schedule_memory` enqueue (exactly 2 `tryEnqueue` sites, tools.go:580/:619); `store_discovery`/`store_rule` do NOT (negative-space test `TestDiscoveryAndRuleNeverEnqueue`, tools_test.go:503). The worker re-fetches via `store.Get` exactly as the existing `summarize-missing` sweep — no new external-input or authz surface; summary derives only from already-authorized content. | closed |
| T-11-06 | Repudiation | New async gateway egress audit trail | low | mitigate | The async worker (second egress path) emits the SAME content-free audit line as the sweep via the shared `store.LogSummaryEgress` helper (summarize.go:55) — id/scope/visibility/owner/content_len/model/outcome/err, never content. Called by BOTH `storeFill` (summaryqueue.go:130/:132/:138) and `SummarizeMissing` (summarize.go:177/:182), so the paths cannot drift. | closed |
| T-11-SC | Tampering (supply chain) | `cenkalti/backoff/v5` promotion | high | mitigate | Package Legitimacy Audit verdict OK (RESEARCH.md). Already resolved in go.sum at the recommended `v5.0.3` as indirect; promotion to direct (go.mod:8) is a go.mod-only edit — no network fetch, no go.sum hash change. Verified: `go mod verify` → "all modules verified"; `go mod tidy` stable. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `high` count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-11-01 | T-11-05 | The async worker introduces no new external input or authz surface: it enqueues only from already-authenticated `store_memory`/`schedule_memory` handlers and re-fetches via `store.Get` on the same path as the existing `summarize-missing` sweep. Summary derives solely from already-authorized content; v1 fence-token prompt-injection containment is unchanged. | Phase 11 threat model (plan-time), verified in secure-phase | 2026-07-10 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-10 | 7 | 7 | 0 | Claude (gsd-secure-phase, ASVS L1 grep-depth; register authored at plan time; informed by gsd-code-review CR-01) |

Notes:
- Register was authored at plan time (all 3 PLAN files carry `<threat_model>` blocks); verification confirmed each mitigation is present in the implementation (file:line + `-race` tests above).
- **T-11-04 finding of note:** the plan's original send-on-closed-channel mitigation relied only on the D-08 drain ordering, which the phase's code review (CR-01) proved necessary-but-insufficient against a `Shutdown`-deadline race. The mitigation was strengthened on-branch (`c2075697`) before this audit, so the threat is closed on the corrected implementation, not the plan's original claim.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-10
