---
phase: 9
slug: retrieval-eval-harness-ranking-precision
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-10
---

# Phase 9 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Verified from the 09-03 PLAN threat model at L1 (ASVS L1, block_on=high). All
> threats authored at plan time; register origin `register_authored_at_plan_time: true`.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| MCP + Connect search request → shared in-process reranker | Both `deps.searchMemory` and `engramAPI.SearchMemories` route through `Store.SearchReranked`, which runs the SAME owner/scope-constrained query and reranks strictly AFTER results return — reorders already-authorized `[]Memory`, never widens visibility. | Caller's own owner-scoped memory records (in-process only) |
| (conditional D-07) reindex → live Qdrant collection | NOT IMPLEMENTED this phase. Would be an operator-run backfill into a new target collection + cutover. | n/a (not built) |
| (conditional D-08) reranker → external `/v1/rerank` gateway | NOT IMPLEMENTED this phase. Would send candidate content to an external endpoint. | n/a (not built) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-09-03-01 | Elevation of Privilege / Information Disclosure | shared D-06 reranker ordering (MCP + Connect) | high | mitigate | `Store.SearchReranked` (store.go:607) takes `scope, subj Subject` and reranks strictly AFTER the owner/scope-constrained query; over-fetch (`candidateK`) raises the candidate `Limit` within the SAME owner/scope filter, never re-queries or widens scope. Verified by `TestRerankParityMCPAndConnect` → `no_cross_owner_leak_through_reranked_path` (connectapi_test.go:478): actor-B sees 0 of actor-A's private records through the reranked path on BOTH MCP and Connect. | closed |
| T-09-03-02 | Tampering (scope integrity) | ranking inputs | medium | mitigate | `internal/store/rerank.go` consumes only content/lexical/vector signals; no usage-signal field is read (grep-verified: no usage/useCount/lastUsed/accessCount refs). Preserves the Phase 12 boundary (usage signals must never affect ranking). | closed |
| T-09-03-03 | Denial of Service | required CI `test` job | medium | mitigate | D-06 reranker unit tests are hermetic and run in the required `test` job; the eval stays env-gated behind `ENGRAM_RETRIEVAL_EVAL` (TestMain short-circuits before Docker). No new required status check, no skipped-required-workflow deadlock (protect-main 8 checks unchanged). | closed |
| T-09-03-SC | Tampering | supply chain | high | mitigate | D-06 adds NO dependency (stdlib only). `git diff origin/main...HEAD -- go.mod` shows no new `require` lines. No rerank/tokenizer library introduced. | closed |
| T-09-03-D07 | Tampering / Denial of Service | (conditional) reindex + schema change | medium | mitigate | Not applicable — D-07 not implemented (eval accepted D-06). No code path exists. | closed (n/a) |
| T-09-03-D08 | Information Disclosure | (conditional) cross-encoder gateway call | high | mitigate | Not applicable — D-08 not implemented (eval accepted D-06). No candidate content leaves the process. | closed (n/a) |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|

No accepted risks — every threat is mitigated in code and verified.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-10 | 6 | 6 | 0 | gsd-secure-phase (L1 grep classification; short-circuit — all mitigations verified in code + tests) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log (none)
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-10
