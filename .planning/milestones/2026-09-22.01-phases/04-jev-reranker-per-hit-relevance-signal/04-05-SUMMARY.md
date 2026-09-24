---
phase: 04-jev-reranker-per-hit-relevance-signal
plan: 05
subsystem: search
tags: [jev, reranker, relevance, discovery, connect, mcp, d-05, d-06, d-07]

# Dependency graph
requires:
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 01
    provides: store.RankHook, applyRankHook/applyRelevance, store.Memory.Relevance *float64, the D-03 fallback contract this plan's discovery path reuses verbatim
  - phase: 04-jev-reranker-per-hit-relevance-signal
    plan: 03
    provides: ENGRAM_SEARCH_RANKER-gated deps.rankHook construction (unedited by this plan; deps.searchDiscovery now branches on the same field deps.searchMemory already reads)
provides:
  - store.(*Store).SearchDiscoveryReranked — the discovery-side opt-in Jev path, base order = discovery's shipped vector order (no lexical step)
  - memStore.SearchDiscoveryReranked, spyStore.SearchDiscoveryReranked, and the hook-gated branch in deps.searchDiscovery
  - relevance documented on both MCP tool descriptions (search_memory, search_discovery) and in reference/tools.md, reference/memory-record.md
affects: [04-06/04-07/04-08 (docs, security review, Helm values)]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
# Computed excluding gen/, ui/src/lib/gen/, internal/webauth/static (regenerated/vendored,
# not hand-authored).
actuals:
  tokens: 7916
  tasks: 3
  commits: 3
plan_head_before: 8c38fc0c9497c6e0aad7cd52d5b0f2b12bf76bc7

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Discovery reranking reuses applyRankHook/applyRelevance verbatim but skips the lexical step entirely — SearchDiscoveryReranked hands the hook SearchDiscovery's own vector order (CandidateK(k) over-fetch), never RerankHits' output, matching D-07's discretion that discoveries have no lexical rank step to preserve on fallback."
    - "Second interface method, mirrored fake: memStore.SearchDiscoveryReranked and spyStore.SearchDiscoveryReranked were added as a sibling pair to the existing SearchDiscovery method, exactly as SearchReranked/spyStore.SearchReranked were added in an earlier plan — the compile-time `var _ memStore = (*store.Store)(nil)` assertion needed no changes."

key-files:
  created:
    - internal/store/discovery_rerank_test.go
    - internal/server/discovery_rerank_test.go
  modified:
    - internal/store/store.go
    - internal/server/store_iface.go
    - internal/server/fakestore_test.go
    - internal/server/tools.go
    - docs-site/src/content/docs/reference/tools.md
    - docs-site/src/content/docs/reference/memory-record.md

key-decisions:
  - "SearchDiscoveryReranked applies rejectOverMaximum(\"k\", k) against the caller's own k (not CandidateK(k)) before over-fetching — mirrors the D-10 backstop reasoning documented on Store.Search, since CandidateK's internal cap would otherwise let an absurd k (e.g. 50000) silently pass through as a smaller, wrong-looking request instead of a named rejection."
  - "The discovery fixture vectors in internal/server/discovery_rerank_test.go are chosen relative to testDepsWithStore's fakeEmbedder, which embeds every query string to the SAME fixed vector {0.1, 0.2, 0.3} regardless of text — vectors were picked for strictly decreasing cosine similarity to that fixed vector so the vector order (and therefore the fallback order) is deterministic and known ahead of time, not toward the literal query text."

requirements-completed: [RANK-03, RANK-04]

coverage:
  - id: D1
    description: "Store.SearchDiscoveryReranked over-fetches CandidateK(k) discoveries through the existing authz-filtered SearchDiscovery call, hands the whole pool to the hook in SearchDiscovery's own vector order, truncates to k, and falls back to SearchDiscovery's exact shipped order on any hook failure — never widening visibility beyond what SearchDiscovery already returns"
    requirement: RANK-03
    verification:
      - kind: integration
        ref: "internal/store/discovery_rerank_test.go#TestSearchDiscoveryRerankedFallbackIsShippedOrder"
        status: pass
      - kind: integration
        ref: "internal/store/discovery_rerank_test.go#TestSearchDiscoveryRerankedSortsAndStamps"
        status: pass
      - kind: integration
        ref: "internal/store/discovery_rerank_test.go#TestSearchDiscoveryRerankedOwnerIsolation"
        status: pass
      - kind: unit
        ref: "internal/store/discovery_rerank_test.go#TestSearchDiscoveryRerankedRejectsZeroK"
        status: pass
    human_judgment: false
  - id: D2
    description: "deps.searchDiscovery branches on d.rankHook: nil (the default, ranker unset or lexical) calls SearchDiscovery exactly as before this plan (zero SearchDiscoveryReranked calls, MCP k=8 / Connect k=20 defaults unchanged), and a configured hook makes MCP and Connect agree on order and per-hit relevance, fall back to the identical vector order on hook error, and never surface another owner's private discovery through either lane"
    requirement: RANK-03
    verification:
      - kind: integration
        ref: "internal/server/discovery_rerank_test.go#TestSearchDiscoveryDefaultPathUnchanged"
        status: pass
      - kind: integration
        ref: "internal/server/discovery_rerank_test.go#TestSearchDiscoveryRelevanceBothLanes/both_lanes_carry_the_same_relevance"
        status: pass
      - kind: integration
        ref: "internal/server/discovery_rerank_test.go#TestSearchDiscoveryRelevanceBothLanes/hook_error_keeps_vector_order"
        status: pass
      - kind: integration
        ref: "internal/server/discovery_rerank_test.go#TestSearchDiscoveryRelevanceBothLanes/another_owner's_private_discovery_never_appears"
        status: pass
    human_judgment: false
  - id: D3
    description: "The relevance field is documented on every tool surface that can carry it: both MCP tool descriptions (search_memory, search_discovery) gain a sentence explaining the opt-in field, reference/tools.md documents its meaning/opt-in/absence/fallback for both tools, and memory-record.md lists it beside score as a query-time, never-stored field"
    requirement: RANK-04
    verification:
      - kind: unit
        ref: "internal/server (TestRegisterToolsEnumerable, TestSurfaceConformanceServerSide, TestToolAnnotationsBothDirections, TestSupersedeDocsMatchShippedContract, TestRecallMaximumIsStatedNumerically, TestErrorsDocHintCodesMatchArgErrorConstants)"
        status: pass
      - kind: unit
        ref: "internal/surfaces (full package)"
        status: pass
      - kind: other
        ref: "task lint:markdown"
        status: pass
      - kind: other
        ref: "task surfaces:gen (regenerating anchored surfaces produces no diff)"
        status: pass
    human_judgment: false

# Metrics
duration: 13min
completed: 2026-09-24
status: complete
---

# Phase 4 Plan 5: search_discovery Jev Reranker and Relevance Docs Summary

**`search_discovery` now has an opt-in Jev-reranked path (`SearchDiscoveryReranked`) that carries relevance on both MCP and Connect, with the default lexical/vector path proven byte-identical, and `relevance` is now documented on every tool surface that can carry it.**

## Performance

- **Duration:** ~13 min
- **Started:** 2026-09-24T11:02:47-04:00 (approximate — immediately after 04-04's completion)
- **Completed:** 2026-09-24T11:15:27-04:00
- **Tasks:** 3
- **Files modified:** 8 (2 new test files, 6 modified)

## Accomplishments

- `store.(*Store).SearchDiscoveryReranked`: over-fetches `CandidateK(k)` discoveries through the existing authz-filtered `SearchDiscovery` call, hands the whole pool to `applyRankHook` in `SearchDiscovery`'s own vector order (no lexical step — discoveries never had one), and truncates to k. Any hook failure (nil hook, error, or a rejected map) falls back to exactly `SearchDiscovery`'s own shipped order. `k == 0` is rejected with `ErrInvalidArgument`, mirroring `SearchReranked`'s own guard.
- `memStore` and `spyStore` both gained `SearchDiscoveryReranked`; the compile-time `var _ memStore = (*store.Store)(nil)` assertion required no other changes. `deps.searchDiscovery` now branches on `d.rankHook`: nil calls `SearchDiscovery` exactly as before this plan (byte-identical, proven by `TestSearchDiscoveryDefaultPathUnchanged`), and a configured hook routes through the new reranked path.
- `TestSearchDiscoveryRelevanceBothLanes` proves, over a real Qdrant, that MCP and Connect agree on order and per-hit relevance under a scripted hook, both fall back to the identical vector order with no relevance on a hook error, and neither lane (nor the hook itself) ever sees another owner's private discovery.
- Both MCP tool descriptions (`search_memory`, `search_discovery`) gained a sentence explaining `relevance`; `reference/tools.md` documents the field's opt-in (`ENGRAM_SEARCH_RANKER=jev`), meaning, absence rules, and fallback for both tools; `memory-record.md`'s field table gained a `Relevance` row beside `Score`, marked as a query-time, never-stored field.

## Task Commits

Each task was committed atomically:

1. **Task 1: Store.SearchDiscoveryReranked — over-fetch through the discovery filter, hook on vector order, shipped-order fallback** — `eb68f0e6` (feat, tdd)
2. **Task 2: deps.searchDiscovery branches on the hook — default path unchanged, relevance on both discovery lanes** — `a94a39f4` (feat, tdd)
3. **Task 3: Describe relevance on the MCP tools and in the reference docs** — `b673d0e3` (docs)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/store/store.go` — `Store.SearchDiscoveryReranked`
- `internal/store/discovery_rerank_test.go` (new) — fallback/sort/owner-isolation/zero-k tests
- `internal/server/store_iface.go` — `memStore.SearchDiscoveryReranked`
- `internal/server/fakestore_test.go` — `spyStore.SearchDiscoveryReranked`
- `internal/server/tools.go` — `deps.searchDiscovery`'s hook branch; both MCP description sentences
- `internal/server/discovery_rerank_test.go` (new) — default-path-unchanged and both-lanes-relevance tests
- `docs-site/src/content/docs/reference/tools.md` — relevance paragraphs for `search_memory` and `search_discovery`
- `docs-site/src/content/docs/reference/memory-record.md` — `Relevance` field row

## Decisions Made

See `key-decisions` in frontmatter. No architectural deviations from the plan.

## Deviations from Plan

None — plan executed exactly as written, including both flagged assumptions (a separate `SearchDiscoveryReranked` gated on the hook with base order = vector order, per D-07 discretion; and the `CandidateK(k)` over-fetch changing the Qdrant limit only on the jev path) as the plan's own explicit resolutions.

## Known Stubs

None.

## Threat Flags

None beyond the plan's own `<threat_model>` register (T-04-01, T-04-02, T-04-04), which this plan's tests directly exercise: `TestSearchDiscoveryDefaultPathUnchanged` (T-04-01, the hook-gated path never reached without a configured hook), `TestSearchDiscoveryRerankedOwnerIsolation` and the cross-owner subtest of `TestSearchDiscoveryRelevanceBothLanes` (T-04-02, the hook never sees or can surface another owner's private discovery).

## Self-Check: PASSED

- `internal/store/discovery_rerank_test.go` — FOUND
- `internal/server/discovery_rerank_test.go` — FOUND
- Commit `eb68f0e6` — FOUND (`git log --oneline --all | grep eb68f0e6`)
- Commit `a94a39f4` — FOUND (`git log --oneline --all | grep a94a39f4`)
- Commit `b673d0e3` — FOUND (`git log --oneline --all | grep b673d0e3`)
- `go build ./...` — PASS
- `go test ./internal/store/ ./internal/server/ -run 'Discovery|Discover' -count=1` — PASS
- `go test ./internal/store/ ./internal/server/ -count=1` (full suites) — PASS
- `task lint:markdown` — PASS (126 files, no issues)
- `golangci-lint run ./internal/store/... ./internal/server/...` — 0 issues
- `task license:check` — PASS (2292 files, 0 invalid)
- `task surfaces:gen` — no diff (anchored surfaces stable)
- `rg -o -e 'applyRankHook[(]ctx, query, hits, hook[)]' internal/store/store.go | wc -l` — 1
- `rg -o -e 'd[.]st[.]SearchDiscovery[(]ctx, scope, a[.]Kind, c[.]Subj, vec, a[.]K[)]' internal/server/tools.go | wc -l` — 1
- `rg -o -e 'd[.]st[.]SearchDiscoveryReranked[(]' internal/server/tools.go | wc -l` — 1
- `rg -o -e 'carries .relevance. [(]0 to 1' internal/server/tools.go | wc -l` — 2
- `rg -o -F 'relevance' docs-site/src/content/docs/reference/tools.md | wc -l` — 6
- `rg -o -F '| Relevance | \`relevance\` |' docs-site/src/content/docs/reference/memory-record.md | wc -l` — 1

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. `search_discovery` reranking activates only when `ENGRAM_SEARCH_RANKER=jev` is already configured per plan 04-03; this plan adds no new operator-facing configuration.

## Next Phase Readiness

Ready for 04-06/04-07/04-08 (docs, security review, Helm values). `search_discovery` and `search_memory` are now both fully wired through the Jev reranker with per-hit relevance surfaced on every tool surface (MCP, Connect, CLI). No further changes to this plan's files are expected from downstream plans in this phase.

---
*Phase: 04-jev-reranker-per-hit-relevance-signal*
*Completed: 2026-09-24*
