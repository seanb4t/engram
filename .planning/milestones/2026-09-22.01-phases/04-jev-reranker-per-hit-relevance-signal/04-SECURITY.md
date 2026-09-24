---
phase: "4"
slug: "jev-reranker-per-hit-relevance-signal"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-24"
---

# Phase 4 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| search path → Decisions provider | Only when `ENGRAM_SEARCH_RANKER=jev`; query + truncated candidate state, one no-retry call under a 2s budget | Memory text of already-authorized hits (high) |
| Provider response → rank step | Relevance map validated (finite, [0,1], covers every hit id) | Probabilities |
| Rank step → caller | Reorders an already owner/scope-filtered pool only | Authorized records + per-hit `relevance` |
| Helm values → server env | `memory.search.*` rendered only when ranker ≠ lexical | Config |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-04-01 | Info disclosure | search egress | high | mitigate | `searchRankHook` gated on `ranker == jev`; bounded state (600 chars/candidate, 28k-token guard) | closed |
| T-04-02 | Elevation / Info disclosure | rank hook | high | mitigate | `applyRelevance` requires coverage of the pool's own ids; foreign ids ignored; pool is authz-filtered | closed |
| T-04-03 | Tampering | provider answers | medium | mitigate | `FromResponse` + `applyRelevance` reject NaN/Inf/out-of-range | closed |
| T-04-04 | DoS | synchronous rerank | high | mitigate | `CandidateK` ≤ 100; dedicated 2s timeout; `WithNoRetry` (normalized to the worst-case severity filed across plans) | closed |
| T-04-05 | Info disclosure | fallback logs | low | mitigate | `logFallback` emits class word only | closed |
| T-04-06 | DoS | retry | medium | mitigate | Separate no-retry search client; consolidate client unchanged | closed |
| T-04-07 | Info disclosure | enablement log | low | mitigate | Host-only endpoint + key source, never key value | closed |
| T-04-08 | Tampering | config | low | mitigate | Ranker enum + jev-requires-provider + positive timeout validation | closed |
| T-04-09 | Tampering | response shape | medium | mitigate | Opt-in eval row excluded from D-05; parity tests assert no response-level flag | closed |
| T-04-10 | Tampering | relevance gaming | low | accept | See Accepted Risks | closed |
| T-04-11 | Repudiation (evidence) | eval | medium | mitigate | Opt-in exclusion; `fallbacks=0/26` recorded | closed |
| T-04-12 | Info disclosure | eval provenance | low | mitigate | Host-only `EndpointHost` | closed |
| T-04-13 | Info disclosure | eval egress | low | accept | See Accepted Risks | closed |
| T-04-14 | Info disclosure | committed eval log | high | mitigate | Credential-shape scan: 0 matches | closed |
| T-04-15 | Info disclosure | Helm default | high | mitigate | Default `ranker: lexical`; gate emits nothing by default | closed |
| T-04-16 | Tampering | Helm gate | low | mitigate | `chart:validate` pins jev-without-provider still renders (fails loudly server-side) | closed |
| T-04-17 | Info disclosure | Helm docs | medium | mitigate | values.yaml + deploy.md link the configure disclosure | closed |
| T-04-SC | Tampering (deps) | go.mod | low | accept | See Accepted Risks | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-04-01 | T-04-10 | Relevance gaming via shared records has the same exposure as lexical/vector ranking; no threshold acts on relevance; reorder-only blast radius | plan 04-04 threat model | 2026-09-24 |
| AR-04-02 | T-04-13 | Retrieval eval egresses only the committed synthetic corpus, only when an operator sets `ENGRAM_RETRIEVAL_EVAL` and a provider | plan 04-06 threat model | 2026-09-24 |
| AR-04-03 | T-04-SC | No new module dependency (empty go.mod/go.sum diff) | plan threat model | 2026-09-24 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-24 | 18 | 18 | 0 | gsd-security-auditor (L1, source-traced); accepted risks logged by orchestrator |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-24
