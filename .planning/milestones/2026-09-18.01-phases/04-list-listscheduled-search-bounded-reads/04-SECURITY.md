---
phase: "4"
slug: "list-listscheduled-search-bounded-reads"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-20"
---

# Phase 4 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

The register was authored at plan time across all eight plans' `<threat_model>` blocks
(`register_authored_at_plan_time: true`), so this audit verified that each declared
mitigation is present in the shipped implementation rather than scanning for new threats.
ASVS L1 is the configured level; the high-severity filter-narrowing and rejection-ordering
threats were verified at L2 depth because they are this phase's real security surface.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| MCP / Connect client → `internal/server` | The wire boundary. Every recall call arrives with a caller identity resolved upstream; count knobs (`limit`, `k`) are caller-controlled input | Scope names, count knobs, filter args; memory content and summaries on the way out |
| `internal/server` → `internal/store` | The authz boundary. Per CLAUDE.md, authorization is enforced in `internal/store` via Qdrant read filters and owner gates — never in a handler | `Subject`, the typed options struct carrying the caller's projection choice |
| `internal/store` → Qdrant (gRPC) | The capacity boundary this milestone exists for. A response exceeding the 4 MiB receive cap must fail named, never as an opaque `internal` | Filtered point ids, payloads projected by view |

New crossings this phase: the two-phase search fetch re-crosses the store→Qdrant boundary a
second time per query (`has_id` batch), and the deep-offset prefix walk crosses it once per
prefix page with a keys-only projection. Both re-apply the caller's filter unchanged.

---

## Threat Register

46 threats, all closed. Verified 2026-09-20 by `gsd-security-auditor` against the shipped tree.

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-04-01-01 | Information Disclosure | responsetoolarge.go | high | mitigate | `responseTooLargeDetail` carries no byte ceiling or upstream gRPC text (`responsetoolarge.go:37,45-47`); rename only | closed |
| T-04-01-02 | Information Disclosure | argerror.go | high | mitigate | `argErrf`/`HintOutOfRange` detail never interpolates the rejected value — `TestHintNeverEchoesValue/recall_count_over_maximum_no_echo` | closed |
| T-04-01-03 | Repudiation | red-evidence | medium | mitigate | Phase 2's errors-doc patch regenerated under the renamed hint; harness re-proves apply→RED→revert | closed |
| T-04-01-04 | Tampering | redevidence_harness_test.go | medium | mitigate | `defer revert()` + `t.Cleanup(revert)` with a porcelain-clean assertion (`:337-345`) | closed |
| T-04-01-SC | Tampering | supply chain | low | accept | No manifest change; `go.mod`/`go.sum` untouched | closed |
| T-04-02-01 | Information Disclosure | orderedpage.go / store.go | high | mitigate | `excludeSeen` narrows only (`orderedpage.go:79-91`); the same filter value `f` is threaded through walk and assembly (`store.go:1573,1660,1719,1731`) | closed |
| T-04-02-02 | Information Disclosure | store.go / orderedpage.go | high | mitigate | `Seen` bounded at `MaxRecallLimit` before any RPC (`store.go:1772-1774`, `orderedpage.go:132-134`) | closed |
| T-04-02-03 | Denial of Service | store.go | high | mitigate | `rejectOverMaximum` first, then an `Offset > math.MaxUint64-effectiveLimit` wrap guard, both before any RPC (`:1650,1708-1714`) | closed |
| T-04-02-04 | Tampering | store.go | medium | mitigate | `next_cursor` derived from `Exhausted` alone, never from a budget cut (`:1788-1791`) | closed |
| T-04-02-05 | Repudiation | overflow regressions | medium | mitigate | Retargeted to a single legacy-oversized record that still overflows via the batch-of-1 fallback | closed |
| T-04-02-SC | Tampering | supply chain | low | accept | No manifest change | closed |
| T-04-03-01 | Information Disclosure | store.go | high | mitigate | `walkOffsetPrefix` uses the identical `f`, differing only in the `keysView()` selector (`:1565-1584`) | closed |
| T-04-03-02 | Information Disclosure | store.go | high | mitigate | `ListScheduled` builds its filter once and passes it unchanged into `collectOrderedPages` (`:1883-1900`) | closed |
| T-04-03-03 | Denial of Service | store.go | medium | accept | Deep offset costs O(offset) tiny reads — the D-07 tradeoff, documented at `walkOffsetPrefix`; each prefix RPC is byte-bounded by `keysRecordCeiling` | closed |
| T-04-03-04 | Tampering | orderedpage.go | medium | mitigate | An over-bound `Seen` set is rejected, never silently truncated (`:132-134`) | closed |
| T-04-03-05 | Information Disclosure | boundedread.go | low | mitigate | `keysView()` is a single explicit `NewWithPayloadInclude("created_at")` — no payload leaks into the prefix walk (`:250-258`) | closed |
| T-04-03-SC | Tampering | supply chain | low | accept | No manifest change | closed |
| T-04-04-01 | Information Disclosure | searchfetch.go | high | mitigate | `includeIDs` wraps the caller's filter as a nested `Must` and adds only an id restriction (`:50-71`); cross-owner exclusion proven by test | closed |
| T-04-04-02 | Tampering | searchfetch.go | high | mitigate | Drop-on-disappear: a record that left visibility between the two phases is absent, never stale (`:73-124`; `TestSearchFetchSkipsEmptyBatch/drop-on-disappear`) | closed |
| T-04-04-03 | Denial of Service | store.go / rerank.go | medium | mitigate | `rejectOverMaximum("k", k)` is the first line of `Search`/`SearchDiscovery` (`:1159,:1326`); `candidateK` still clamps the reranker at 100 | closed |
| T-04-04-04 | Tampering | store.go | medium | mitigate | Results assembled by walking phase-one's own id order — never re-sorted (`:1224-1237`; `TestSearchPreservesRankOrder`) | closed |
| T-04-04-05 | Repudiation | searchfetch.go | medium | mitigate | Empty id batch returns as the first statement, before any RPC (`:104-107`) — this is also what keeps `recallInvocationRows`' exact counts stable | closed |
| T-04-04-SC | Tampering | supply chain | low | accept | No manifest change | closed |
| T-04-05-01 | Information Disclosure | searchfetch.go | high | mitigate | `backfillNoSummaryContent` passes the same `f` to `fetchPayloadsByID` (`:174-196`) | closed |
| T-04-05-02 | Information Disclosure | store / search lanes | high | mitigate | `TestNoSummaryContentBackfill` (List) and `TestSearchNoSummaryContentBackfill` (Search) — the latter closed WINDOWS #10 | closed |
| T-04-05-03 | Denial of Service | store.go | high | mitigate | `rejectOverMaximum` is the first statement of `List`/`ListScheduled` (`:1650,:1876`) and of both search entry points | closed |
| T-04-05-04 | Tampering | store.go | medium | mitigate | `listByCursor`'s silent clamp deleted; refusal centralized in `List`'s single guard (`:1756-1759`) | closed |
| T-04-05-05 | Denial of Service | searchfetch.go | low | mitigate | Early return when no record needs backfill, before any RPC (`:183-185`) | closed |
| T-04-05-SC | Tampering | supply chain | low | accept | No manifest change | closed |
| T-04-06-01 | Denial of Service | tools.go | high | mitigate | `rejectOverMaximumCount` is the first statement in all four shared core methods, ahead of scope resolution and `EmbedQuery` — `TestOutOfRangeRejectedBeforeAnyBackend` asserts zero embed and zero gRPC calls | closed |
| T-04-06-02 | Information Disclosure | tools.go | high | mitigate | Rejection message built from field name + the constant only (`:1643-1651`) | closed |
| T-04-06-03 | Information Disclosure | boundedread.go | medium | mitigate | `Full` selects only `readView.selector`; it never reaches `listFilter`/`ownerScopeFilter` | closed |
| T-04-06-04 | Elevation of Privilege | outofrange_test.go | medium | mitigate | The table drives all seven real entry points, not the shared core — a surface that forgot the guard would go red | closed |
| T-04-06-05 | Tampering | rules.go | medium | mitigate | `Full: a.Full` threaded into the rule listing's direct `Store.List` call (`:220-225`); pinned by a content assertion | closed |
| T-04-06-SC | Tampering | supply chain | low | accept | No manifest change | closed |
| T-04-07-01 | Repudiation | proto / CLI / docs | high | mitigate | The maximum stated numerically on every documented surface; `recallmaxdocs_test.go` gates it | closed |
| T-04-07-02 | Tampering | codegen | medium | mitigate | Generation-task-only workflow (`task proto:gen` / `surfaces:gen`); the verify step re-runs and diffs clean | closed |
| T-04-07-03 | Repudiation | upgrade.md | high | mitigate | BREAKING §16 entry with a who-should-act row (`upgrade.md:38,413-439`) | closed |
| T-04-07-04 | Tampering | PROJECT.md | medium | mitigate | Key Decisions table row only — no version-bearing heading added (`:921`) | closed |
| T-04-07-SC | Tampering | supply chain | low | accept | No manifest change; generation uses the already-vendored buf tool | closed |
| T-04-08-01 | Tampering | redevidence harness | medium | mitigate | Revert-on-failure shared mechanism re-proven across all 42 patches | closed |
| T-04-08-02 | Repudiation | red-evidence | high | mitigate | All 17 of this phase's patches present and registered; harness re-verifies each | closed |
| T-04-08-03 | Repudiation | REQUIREMENTS.md | high | mitigate | Exactly this phase's five requirements ticked; traceability rows read Complete | closed |
| T-04-08-04 | Tampering | cross-phase | high | mitigate | Git history shows only Phase 4's own files touched; Phase 2/3 directories unmodified by this phase's commits | closed |
| T-04-08-05 | Denial of Service | test runtime | medium | mitigate | The default-timeout run stayed within budget; the contingency never fired | closed |
| T-04-08-SC | Tampering | supply chain | low | accept | No manifest change | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-04-01 | T-04-03-03 | Deep offset costs O(offset) small reads. Accepted as the D-05/D-07 consequence of keeping offset paging unbounded in depth: every prefix RPC is byte-bounded by `keysView`/`keysRecordCeiling`, so the cost is latency, never overflow. The alternative — capping offset at the maximum — was considered in discuss-phase and rejected (it would have restricted a LOCKED ADR's UI paging mode). | user (discuss-phase, 2026-09-19) | 2026-09-20 |
| R-04-02 | all `T-04-NN-SC` | No supply-chain change this phase: zero new `go.mod`/`go.sum` entries, and codegen runs the already-vendored buf tool. | orchestrator | 2026-09-20 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-20 | 46 | 46 | 0 | gsd-security-auditor (verify-mitigations mode, ASVS L1 / L2 depth on the high-severity filter and ordering threats) |

**Related, verified during this audit but not new findings:**

- WINDOWS #10 and #11 were open at plan 04-08's commit time and are both `fixed` at HEAD (`44b36d9d`, `989c4593`). No outstanding gap against T-04-05-01/T-04-05-02.
- 04-REVIEW.md's IN-01 (the store-layer `rejectOverMaximum` backstop returns a bare error rather than the `field=`/`hint=` envelope) stands as deliberately deferred: unreachable in current behavior because every server entry point calls `rejectOverMaximumCount` first. Recorded, not re-raised.
- No unregistered threat flags: every SUMMARY's `## Threat Flags` section either declares no new surface or maps entirely onto that plan's own registered IDs.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
