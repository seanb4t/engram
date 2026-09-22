---
phase: "07"
slug: "bounded-provider-responses"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-22"
validated: "2026-09-22"
---

# Phase 07 — Validation Strategy

> Per-phase validation contract, reconstructed retroactively (validate-phase State B) from the
> PLAN/SUMMARY artifacts on 2026-09-22 — the verify:post hook did not seed or reconcile this file
> during execution (milestone audit tech-debt item).

Every test name below was resolved from the plans' `<automated>` blocks and run with `-v` on
2026-09-22; each row's match count is the number of `--- PASS` lines observed, so no row can be a
`no tests to run` false green.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (stdlib) + `net/http/httptest`; the shared slow-trickle handler lives in `internal/testhttp/trickle.go`. No live Qdrant, no network |
| **Config file** | none — `Taskfile.yaml` `test:*` tasks are the entry points |
| **Quick run command** | `go test ./internal/httpdrain/ ./internal/embed/ ./internal/summarize/ ./internal/config/ ./internal/server/ -count=1` |
| **Full suite command** | `task` (lint + whole-repo test) |
| **Actual runtime** | httpdrain 0.1 s · embed 0.3 s · summarize 0.2 s · config 0.1 s · server (`TestProviderBound*`) 1.0 s |

---

## Sampling Rate

- **After every task commit:** the quick run command above.
- **After every plan:** the quick run command plus `go test ./internal/keylinks/ -count=1`.
- **Phase gate:** `task` green.

---

## Per-Requirement Verification Map

| Requirement | Behavior proven | Type | Automated Command | Tests matched | Status |
|-------------|-----------------|------|-------------------|---------------|--------|
| REQ-provider-drain-bounded | The shared drain stops at its byte bound, closes the body at its time bound, and a zero bound skips the drain entirely | unit | `go test ./internal/httpdrain/ -run '^TestDrain(StopsAtByteBound\|ClosesBodyAtTimeBound\|ZeroSkipsEntirely)$' -count=1 -v` | 3 | ✅ green |
| REQ-provider-drain-bounded | Embed: large-but-fast body bounded by bytes (connection reuse observed); slow-trickle body bounded by time under `WithTimeout(0)`; zero options honored | integration | `go test ./internal/embed/ -run '^TestEmbedDrain(BoundedByBytes\|BoundedByTimeUnderZeroRequestTimeout\|OptionsHonorZero)$' -count=1 -v` | 3 | ✅ green |
| REQ-provider-drain-bounded | Summarize: the same three properties on the second lane | integration | `go test ./internal/summarize/ -run '^TestSummarizeDrain(BoundedByBytes\|BoundedByTimeUnderZeroRequestTimeout\|OptionsHonorZero)$' -count=1 -v` | 3 | ✅ green |
| REQ-provider-drain-bounded | A request timeout ceiling applies on both lanes, independent of `http.Client.Timeout` | unit | `go test ./internal/embed/ ./internal/summarize/ -run '^Test(Embed\|Summarize)TimeoutCeiling$' -count=1 -v` | 2 | ✅ green |
| REQ-provider-drain-bounded | The six `ENGRAM_{EMBED,SUMMARY}_{DRAIN_BYTES,DRAIN_TIMEOUT,MAX_TIMEOUT}` keys are registry-declared and an unbounded value is rejected | unit | `go test ./internal/config/ -run '^(TestProviderBoundRegistryEntries\|TestValidateProviderBounds)$' -count=1 -v` | 2 | ✅ green |
| REQ-provider-drain-bounded | The configured bounds actually reach both production clients (a dead helper cannot pass) | unit | `go test ./internal/server/ -run '^TestProviderBound(HelpersParseAndDefault\|OptionsWiredIntoBothClients)$' -count=1 -v` | 2 | ✅ green |
| REQ-provider-error-body-closed | A non-2xx provider response surfaces status + a bounded, truncated body snippet and is drained for connection reuse — embed | integration | `go test ./internal/embed/ -run '^TestEmbedNon2xx(IncludesStatusAndBody\|DrainsForReuse\|ErrorBodyTruncated)$' -count=1 -v` | 3 | ✅ green |
| REQ-provider-error-body-closed | Same, summarize | integration | `go test ./internal/summarize/ -run '^TestSummarizeNon200(IncludesStatusAndBody\|DrainsForReuse\|ErrorBodyTruncated)$' -count=1 -v` | 3 | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

Plan 07-05's red-evidence patches and the `TestRedEvidencePatchesAreLive` harness were removed in
`c1afd6c1` (rule `3p0zsqrhmb`: no tests for tests). They were never behavior coverage; the rows
above are.

---

## Manual-Only Verifications

None.

---

## Validation Sign-Off

- [x] Every requirement has an automated command
- [x] Every command matched a non-zero test count on the recorded run
- [x] No watch-mode flags
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-09-22 (retroactive, State B)

## Validation Audit 2026-09-22

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |
