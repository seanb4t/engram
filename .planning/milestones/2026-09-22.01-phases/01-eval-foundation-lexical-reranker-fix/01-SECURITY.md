---
phase: "1"
slug: "eval-foundation-lexical-reranker-fix"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-23"
---

# Phase 1 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Eval → embedding gateway | The gated eval sends fixture text and queries to the third-party embedding API | Synthetic public-style text only (low) |
| Committed evidence → public repo / GitHub #605 | Eval logs, decision record, and evidence comment are published | Model name, dim, metrics; never credential values (high if leaked) |
| `SearchReranked` rank step → caller | Reordering happens after the owner/scope-filtered `Search` | Authorized memory records only |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-01-01 | Denial of service | `TestMain` eval gate | medium | mitigate | `resolveEvalGate` maps only `ENGRAM_RETRIEVAL_EVAL`, never calls `Validate()` (`internal/retrievaleval/gate.go`) | closed |
| T-01-02 | Repudiation | differ gate / gate parse | medium | mitigate | `cosineDistance` rejects NaN/Inf/degenerate input (`vector.go`); unit-tested | closed |
| T-01-03 | Information disclosure | `differProbe` to embedder | low | accept | Synthetic public tooling text (see Accepted Risks) | closed |
| T-01-04 | Information disclosure | `VectorOrder` / comparison rankers | low | mitigate | Pure functions; `SearchReranked` still calls authz-filtered `Search` once (`store.go`) | closed |
| T-01-05 | Tampering | usage counters in rank | low | mitigate | `TestVectorOrderIgnoresAccessCount`, `TestComparisonRankersIgnoreAccessCount` | closed |
| T-01-06 | Information disclosure | `paraphraseSeeds` | medium | mitigate | `TestParaphraseCorpusIntegrity/denylist` | closed |
| T-01-07 | Tampering | blind query authorship | medium | mitigate | Fresh-context labels-only author; `label-leak` subtest; provenance in `01-BLIND-QUERIES.md` | closed |
| T-01-08 | Repudiation | query provenance | low | mitigate | `01-BLIND-QUERIES.md` header (date, author type, prompt commit) | closed |
| T-01-09 | Tampering | ranking choice | medium | mitigate | `decideRanking` + `TestDecideRanking` pinned before live numbers | closed |
| T-01-10 | Repudiation | eval output | medium | mitigate | Distinct `harness:` prefix vs `D-10 gate FAILED:` in `retrieval_eval_test.go` | closed |
| T-01-11 | Information disclosure | paraphrase queries to embedder | low | accept | Synthetic fictional-project prose (see Accepted Risks) | closed |
| T-01-12 | Information disclosure | `01-EVAL-BASELINE.log` | high | mitigate | Key value verified absent (`rg -F`) | closed |
| T-01-13 | Tampering | decision checkpoint | medium | mitigate | `Approved winner` == `Rule winner` (`lexical`) in `01-RANKING-DECISION.md` | closed |
| T-01-14 | Spoofing | non-production embedder | low | mitigate | Model/dim recorded (`gemini-embedding-2`) in decision record | closed |
| T-01-15 | Information disclosure | `SearchReranked` ranking change | high | mitigate | Search call shape unchanged; `TestRerankParityMCPAndConnect/no_cross_owner_leak_through_reranked_path` | closed |
| T-01-16 | Information disclosure | `01-EVAL-SHIPPED.log`, `01-ISSUE-605-EVIDENCE.md` | high | mitigate | Key value verified absent (`rg -F`) | closed |
| T-01-17 | Elevation of privilege | `gh issue comment` | medium | mitigate | `Evidence comment authorized: yes` written from the user's checkpoint reply | closed |
| T-01-18 | Tampering | shipped ranker | medium | mitigate | `TestRankCandidatesIsTheD05Winner` pins the seam | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01-01 | T-01-03 | `differProbe` is synthetic public tooling text; unchanged posture from T-14-01 | plan 01-01 threat model | 2026-09-22 |
| AR-01-02 | T-01-11 | Paraphrase queries are synthetic fictional-project prose; corpus denylist constrains neighbours | plan 01-04 threat model | 2026-09-22 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-23 | 18 | 18 | 0 | orchestrator (L1 grep verification; auditor short-circuited per register_authored_at_plan_time + ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-23
