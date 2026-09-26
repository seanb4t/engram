---
phase: "01"
slug: "store-prerequisites"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-26"
---

# Phase 01 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| caller Subject → internal/store (ArchiveAs/RestoreAs) | Phase 3 RPC/MCP handlers pass a verified Subject and a caller-chosen id; the store is the only place ownership is decided (DEC-cgb) | verified identity, record id |
| internal/store → Cedar PDP | getWritable consults the embedded policy corpus per record with the new archive action | owner claim, action, resource visibility |
| caller Subject + scope → internal/store (ListTags) | Phase 3 passes a verified Subject, a caller-chosen scope and limit | verified identity, scope, limit |
| internal/store → Qdrant Facet | aggregate tag counts computed server-side under the filter the store sends | tag names and counts (may reveal record existence) |
| caller Subject + anchor id + k → internal/store (RelatedMemories) | Phase 3 passes a verified Subject, a caller-chosen id and k | verified identity, record id, k |
| internal/store → Qdrant Query / Count / Facet / Scroll | candidate sets and tag frequencies computed server-side under the filter the store sends | record ids, summaries, tags, citations |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-01-01 | Elevation of Privilege | Store.ArchiveAs / Store.RestoreAs | high | mitigate | getWritable(ctx, id, subj, authz.ActionArchive) inside the store (spine.go:858, :969); TestArchiveAsOwnerGate / TestRestoreAsOwnerGate | closed |
| T-01-02 | Information Disclosure | ArchiveAs/RestoreAs not-found path | medium | mitigate | Uniform ErrNotFound + ArchiveOutcomeNotFound for missing and readable-but-unowned ids (store.go:2202-2215); TestArchiveAsFailsClosed | closed |
| T-01-03 | Elevation of Privilege | nil Subject | high | mitigate | principalParams fails closed, no operator sentinel (store.go:57-66); TestArchiveAsFailsClosed/nil_Subject | closed |
| T-01-04 | Tampering | check-then-write window | medium | mitigate | getWritable and the payload write run under one s.locker.Lock(ctx, id) (spine.go:852-866, 963-977); TestArchiveAsRestoreAsConcurrentSerialize | closed |
| T-01-05 | Elevation of Privilege | Cedar action vocabulary | medium | mitigate | Distinct ActionArchive (authz.go:32); shared_read.cedar permits read only; TestPolicyCorpus_SharedReadOnly covers ActionArchive | closed |
| T-01-06 | Repudiation | gated archive attribution | low | mitigate | Spans carry engram.owner and engram.archive.outcome (spine.go:839-850, 950-961) | closed |
| T-01-07 | Information Disclosure | ListTags / facetTags | high | mitigate | recallVisibleFilter (ownerScopeFilter first) is the Facet request's Filter, never a post-filter (listtags.go:41-65); TestListTagsReadFilter, TestListTagsUnderConcurrentWrites | closed |
| T-01-08 | Information Disclosure | recall-hidden records | medium | mitigate | recallVisibleFilter appends the three recall-gate conditions unconditionally (listtags.go:43-45); TestListTagsRecallVisibleOnly | closed |
| T-01-09 | Denial of Service | Facet size | low | mitigate | rejectOverMaximum("limit") before any RPC, default 100, Facet asks limit+1 (listtags.go:64, 120-122) | closed |
| T-01-10 | Tampering | truncation | low | mitigate | more derived from the limit+1 over-ask (listtags.go:85-88); TestListTagsLimitAndMore | closed |
| T-01-11 | Elevation of Privilege | recall-gate blind spot | medium | mitigate | Facet in recallEmissionMethods; ListTags/RelatedMemories in recallEntryPointSeeds (schemaversion_recallgate_test.go:344, 376-377); TestRecallEmissionSetIsCompleteAndClassified, TestSchemaVersionNeverGatesRecall | closed |
| T-01-12 | Information Disclosure | relatedVectorEdges / assembleRelated | high | mitigate | Vector Query under edgeFilter(recallVisibleFilter); fetchPayloadsByID composes the same filter (relatedmemories.go:569-576, 632; searchfetch.go:104-139); TestRelatedMemoriesVectorEdge, TestRelatedMemoriesCandidateVanishesBeforeFetch | closed |
| T-01-13 | Information Disclosure | supersession walk | high | mitigate | GetReadable per hop; unreadable member ends its branch (relatedmemories.go:491, 516); TestRelatedMemoriesSupersessionChain | closed |
| T-01-14 | Information Disclosure | count / truncation side channel | medium | mitigate | Candidates only from filtered sub-queries; fetch-miss dropped silently; Truncated counts admitted entries only (relatedmemories.go:662-671); TestRelatedMemoriesBounds | closed |
| T-01-15 | Information Disclosure | anchor resolution | medium | mitigate | Anchor via GetReadable, all failures fold to ErrNotFound (relatedmemories.go:733); TestRelatedMemoriesAnchorAccess | closed |
| T-01-16 | Denial of Service | fan-out | low | mitigate | rejectOverMaximum("k"); supersession depth 8 / 16 members; total ceiling 64; one vector Query; TestRelatedMemoriesSupersessionCaps, TestRelatedMemoriesBounds | closed |
| T-01-17 | Information Disclosure | tag weights (n, df) | medium | mitigate | n Count, facetTags df, and df fallback Count all carry recallVisibleFilter (relatedmemories.go:239-273); TestRelatedMemoriesTagAndCitationEdgesFollowGates | closed |
| T-01-18 | Information Disclosure | tag / citation probes | high | mitigate | Every probe Scroll uses edgeFilter over recallVisibleFilter (relatedmemories.go:304, 409-411); TestRelatedMemoriesTagAndCitationEdgesFollowGates | closed |
| T-01-19 | Tampering | citation matching | low | mitigate | kind+ref matched inside one nested citations object (relatedmemories.go:409-411); TestRelatedMemoriesCitationEdge | closed |
| T-01-20 | Denial of Service | probe fan-out | low | mitigate | Tag/citation probe caps 16, ids only; per-type caps 8; compile-time headroom guard (relatedmemories.go:368); TestRelatedMemoriesTagEdgeCap | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

No accepted risks.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-26 | 20 | 20 | 0 | gsd-security-auditor (cited tests run live against Qdrant) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-26
