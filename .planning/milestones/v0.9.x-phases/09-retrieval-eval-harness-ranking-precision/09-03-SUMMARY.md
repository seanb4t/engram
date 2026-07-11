---
phase: 09-retrieval-eval-harness-ranking-precision
plan: 03
subsystem: api
tags: [go, qdrant, rerank, lexical-overlap, mcp, connect, retrieval-eval]

requires:
  - phase: 09-01
    provides: "internal/retrievaleval harness + #261 fixture + prod-parity embed/search measured path"
  - phase: 09-02
    provides: "search_memory score field + reference/tools.md score-field docs"
provides:
  - "internal/store/rerank.go: dependency-free lexical-overlap reranker (D-06) over []store.Memory"
  - "store.SearchReranked: the single shared over-fetch+rerank+truncate helper both recall surfaces call"
  - "MCP deps.searchMemory AND Connect engramAPI.SearchMemories both route through the shared reranked path"
  - "hard RANK-based #261 acceptance bar in the eval (permanent green regression guard)"
  - "the eval-gated ranking decision: accept-d06 (D-07 hybrid / D-08 cross-encoder evaluated and NOT needed)"
affects: [10-asymmetric-embeddings, 12-usage-signals]

tech-stack:
  added: []
  patterns:
    - "One shared exported store helper (SearchReranked) is the sole ranking path for MCP + Connect + the eval — no surface can drift from the shipped rerank behavior"
    - "candidateK = min(max(k*4, 32), 100): bounded over-fetch, never Limit==k, never unbounded"
    - "Rerank runs strictly AFTER ownerScopeFilter — reorders an already-authorized []Memory, never widens visibility"
    - "internal/store stays narrow: RerankHits/SearchReranked take plain inputs (query string, vec []float32, k) and import neither internal/embed nor internal/server"
    - "tied-raw-score corpus (fakeEmbedder fixed vector) isolates the lexical rerank as the deterministic differentiator in the two-surface parity test"

key-files:
  created:
    - internal/store/rerank.go
    - internal/store/rerank_test.go
  modified:
    - internal/store/store.go
    - internal/server/tools.go
    - internal/server/connectapi.go
    - internal/server/connectapi_test.go
    - internal/retrievaleval/retrieval_eval_test.go
    - internal/retrievaleval/fixtures.go
    - docs-site/src/content/docs/reference/tools.md

key-decisions:
  - "accept-d06: the D-06 heuristic reranker closes REQ-ranking-precision on the lightest lever (no schema change, no reindex, no new dependency) across BOTH recall surfaces via the shared Store.SearchReranked helper"
  - "Hybrid (D-07) and cross-encoder (D-08) were evaluated and are NOT needed — live recall@8=1.00 / MRR=1.000; escalating would contradict D-05 (numbers decide). Kept documented as conditional-only."
  - "D-06 is a pure lexical-overlap term-set-intersection boost tie-broken by raw Score then ID; candidateK=min(max(k*4,32),100); SearchReranked rejects k<=0 (callers pass already-defaulted k: MCP 8, Connect 20)"
  - "D-03 NARROWED: #261 acceptance is RANK-within-k (hard t.Errorf); raw-score separation is a t.Logf DIAGNOSTIC only — a lexical reranker can promote T's rank without raising its raw Qdrant score above every neighbor, and Score stays raw first-stage dense similarity"

patterns-established:
  - "Single shared ranking path for all recall surfaces + the eval (review finding 2/5)"
  - "Behavior-changing shared helpers are proven by real two-surface behavioral tests, not a grep (round-2 finding 3)"

requirements-completed: [REQ-ranking-precision]

coverage:
  - id: D1
    description: "Dependency-free D-06 lexical-overlap reranker (RerankHits) deterministically promotes a high-lexical-overlap candidate above topical-but-distinct neighbors, over already-fetched []Memory, stdlib-only"
    requirement: REQ-ranking-precision
    verification:
      - kind: unit
        ref: "internal/store/rerank_test.go#TestRerankHitsPromotesLexicalOverlap"
        status: pass
      - kind: unit
        ref: "internal/store/rerank_test.go#TestRerankHitsDeterministic"
        status: pass
    human_judgment: false
  - id: D2
    description: "Shared SearchReranked helper: candidateK=min(max(k*4,32),100) bounds (floor 32, cap 100) and k<=0 rejected with an error"
    requirement: REQ-ranking-precision
    verification:
      - kind: unit
        ref: "internal/store/rerank_test.go#TestCandidateK"
        status: pass
      - kind: unit
        ref: "internal/store/rerank_test.go#TestSearchRerankedRejectsZeroK"
        status: pass
    human_judgment: false
  - id: D3
    description: "BOTH MCP deps.searchMemory and Connect engramAPI.SearchMemories route through the shared SearchReranked helper and return identical reranked ID order, honor k/tags/created-window, and leak no cross-owner private records through the reranked path"
    requirement: REQ-ranking-precision
    verification:
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestRerankParityMCPAndConnect (identical_reranked_order, honors_k, honors_tags_and_filter, honors_created_window, no_cross_owner_leak_through_reranked_path)"
        status: pass
    human_judgment: false
  - id: D4
    description: "#261 fixture is a hard RANK-based acceptance bar (Record T within default k for Query A and B); raw-score separation is a diagnostic only; the eval's measured path calls the SAME shared SearchReranked helper"
    requirement: REQ-ranking-precision
    verification:
      - kind: integration
        ref: "ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval (live: gemini-embedding-2 @3072 via OpenRouter) — query-a rank 1/8, query-b rank 1/8, recall@8=1.00, MRR=1.000"
        status: pass
      - kind: unit
        ref: "go test ./internal/retrievaleval/... (gate unset) — clean skip, exits 0"
        status: pass
    human_judgment: false
  - id: D5
    description: "search_memory reference doc carries the rerank-order caveat (score may be non-monotonic after rerank) without overwriting 09-02's score-field docs"
    requirement: REQ-ranking-precision
    verification:
      - kind: other
        ref: "docs-site/src/content/docs/reference/tools.md search_memory Returns section — rerank-order caveat present, score-field docs preserved"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-07-09
status: complete
---

# Phase 9 Plan 03: Ranking Precision (D-06 Reranker) Summary

**A dependency-free lexical-overlap reranker (D-06) wired behind one shared `store.SearchReranked` helper on BOTH the MCP and Connect recall paths, proven by a live retrieval eval that surfaces the #261 target record at rank 1/8 for both near-verbatim queries (recall@8=1.00, MRR=1.000) — REQ-ranking-precision closed on the lightest lever, no schema change, no reindex, no new dependency.**

## Performance

- **Duration:** ~35 min (Tasks 1-2 execution + checkpoint resolution)
- **Started:** 2026-07-09T19:55:00Z (approx)
- **Completed:** 2026-07-09T20:05:00Z (approx; checkpoint decision applied)
- **Tasks:** 3/3 (Task 1 auto, Task 2 auto, Task 3 blocking checkpoint:decision — resolved accept-d06)
- **Files modified:** 8 (2 created, 6 modified)

## Accomplishments

- **D-06 reranker (`internal/store/rerank.go`):** a PURE, stdlib-only (`strings`/`sort`) reranker. `RerankHits(query, hits, k)` tokenizes query and candidate content+tags identically, up-weights candidates by lexical-overlap count, and tie-breaks deterministically on raw `Score` then `ID`. No embedder, handler, or MCP/Connect concept leaks into `internal/store` (round-2 finding 7).
- **Shared over-fetch helper (`Store.SearchReranked`):** computes `candidateK = min(max(k*4, 32), 100)` (review finding 7 — never `Limit==k`, never unbounded), runs the existing owner/scope-filtered `Search` at candidateK, applies `RerankHits`, and truncates to `k`. Rejects `k<=0` with `ErrInvalidArgument` (round-2 finding 6) — callers pass the already-defaulted effective `k` (MCP 8, Connect 20). Reranking runs strictly AFTER `ownerScopeFilter`, never widening visibility.
- **Both recall surfaces wired (review finding 2):** MCP `deps.searchMemory` (tools.go) and Connect `engramAPI.SearchMemories` (connectapi.go) both call `SearchReranked` instead of raw `Store.Search` — no recall surface left unreranked.
- **Real two-surface behavioral tests (round-2 finding 3, not a grep):** `TestRerankParityMCPAndConnect` seeds a #261-shaped corpus with `fakeEmbedder`'s fixed vector (all raw Qdrant scores tie → the lexical rerank is the sole differentiator) and asserts MCP and Connect return the IDENTICAL reranked ID order, promote the high-overlap record to rank 1 on both, honor `k`/`tags` AND-filter/`created_after` window identically, and leak no cross-owner private records through the reranked path.
- **Hard RANK-based #261 acceptance bar:** the eval's measured path now calls the SAME `SearchReranked` helper; the #261 fixture flips from report-only to a hard `t.Errorf` bar (Record T within default `k` by position for Query A AND B). Raw-score separation is a `t.Logf` diagnostic only (D-03 narrowed). The score-descending assertion was dropped from the reranked set (rerank changes order).
- **Docs caveat (round-2 finding 5):** `reference/tools.md` search_memory now notes "Final order may include reranking; `score` remains first-stage dense similarity and may be non-monotonic after rerank" — 09-02's score-field docs preserved.
- **Eval-gated decision = accept-d06:** the live eval clears the #261 rank bar; D-07/D-08 evaluated and not needed.

## Live Eval Results (verbatim — the accept-d06 evidence)

- **Embedder:** `google/gemini-embedding-2` via OpenRouter (base `https://openrouter.ai/api` → `/v1/embeddings`), dim 3072. Command: `ENGRAM_RETRIEVAL_EVAL=1 go test ./internal/retrievaleval/ -run TestRetrievalEval` against a live Qdrant testcontainer.
- **gh261-sticky-neighbor-crowding / query-a:** Record-T rank = **1/8** (hard rank bar PASS); `score(T)=0.878703`, best-distractor=`0.784238`, gap=**+0.094465** (diagnostic).
- **gh261-sticky-neighbor-crowding / query-b:** Record-T rank = **1/8** (hard rank bar PASS); `score(T)=0.952102`, best-distractor=`0.817654`, gap=**+0.134448** (diagnostic).
- **recall@8 = 1.00, MRR = 1.000** (post-D-06, both queries).
- **Cross-check:** an earlier run with `google/gemini-embedding-001` @3072 also passed — query-a/b both rank 1/8, recall@8=1.00, MRR=1.000. Consistent.

## Task Commits

Each task was committed atomically:

1. **Task 1: D-06 reranker + shared over-fetch helper wired into MCP and Connect, two-surface behavioral tests, docs caveat** - `ef1829c` (feat)
2. **Task 2: Route the eval through SearchReranked, hard RANK-based #261 bar** - `f07a642` (feat)
3. **Task 3: Eval-gated ranking decision (blocking checkpoint)** - resolved **accept-d06** (decision recorded, no code change)

**Progress bookkeeping:** `152e28d` (docs: record Task 1+2 progress + pending checkpoint)

_No TDD tasks in this plan._

## Files Created/Modified

- `internal/store/rerank.go` (created) - PURE `RerankHits` reranker + `candidateK` helper + `tokenize`/`lexicalOverlap`; stdlib-only, no embed/server imports
- `internal/store/rerank_test.go` (created) - hermetic unit tests: lexical-overlap promotion, determinism, truncation, candidateK bounds (32 floor / 100 cap), k<=0 rejection
- `internal/store/store.go` - added `Store.SearchReranked` (over-fetch candidateK → RerankHits → truncate; k<=0 → ErrInvalidArgument)
- `internal/server/tools.go` - `deps.searchMemory` calls `SearchReranked` (default k=8 applied first)
- `internal/server/connectapi.go` - `engramAPI.SearchMemories` calls the SAME `SearchReranked` (default k=20 applied first)
- `internal/server/connectapi_test.go` - `TestRerankParityMCPAndConnect` (identical order, k/tags/window parity, no cross-owner leak); added `slices` import
- `internal/retrievaleval/retrieval_eval_test.go` - measured path uses `SearchReranked`; #261 flipped to hard rank `t.Errorf`; dropped post-rerank score-descending assertion; raw-score gap is `t.Logf` diagnostic
- `internal/retrievaleval/fixtures.go` - #261 fixture doc comment updated to describe the hard rank bar
- `docs-site/src/content/docs/reference/tools.md` - search_memory rerank-order caveat (score non-monotonic after rerank)

## Decisions Made

- **accept-d06** — the eval RANK bar is met (recall@8=1.00, MRR=1.000; T rank 1/8 for both queries). REQ-ranking-precision closed by the lightest lever on both recall surfaces via the shared helper. The #261 fixture is a permanent GREEN rank-based regression guard.
- **D-07 (hybrid) and D-08 (cross-encoder) evaluated and NOT needed** — escalating with a perfect recall@8/MRR would contradict D-05 (numbers decide). Both remain documented as conditional-only in the plan's `<conditional_escalation>`.
- **D-06 mechanism:** pure lexical-overlap term-set-intersection boost, tie-broken by raw `Score` then `ID`; `candidateK=min(max(k*4,32),100)`; `SearchReranked` rejects `k<=0`.
- **D-03 narrowed:** #261 acceptance is RANK-within-k (hard); raw-score separation is a diagnostic only (a lexical reranker can promote T's rank without raising its raw Qdrant score above every neighbor; `Score` stays raw first-stage dense similarity).

## Deviations from Plan

None - plan executed exactly as written. Task 1 and Task 2 followed their actions, acceptance criteria, and prohibitions without needing an auto-fix; Task 3's checkpoint decision was resolved by the coordinator/user (accept-d06), not decided by the executor.

One in-flight lint touch-up during Task 1 (not a plan deviation): golangci-lint's `staticcheck QF1001` flagged a De Morgan simplification in `tokenize`'s rune predicate; applied the equivalent non-negated form. No behavior change.

## Issues Encountered

- **Live eval could not run in the original executor session** — no embedder gateway credentials were configured (`ENGRAM_OPENAI_*`/`ENGRAM_EMBED_*` unset), so `ENGRAM_RETRIEVAL_EVAL=1 task eval:retrieval` failed at the seed-embed step (`connection refused` to `localhost:4000`). Task 1/2 code was landed with hermetic + testcontainer proof, and the plan paused at the Task 3 checkpoint with an honest "live eval deferred" note. The coordinator subsequently ran the live eval (see below) and cleared the bar.
- **Prod-parity caveat (NOT closed):** prod is `qwen3-embedding-8b` @4096 (D-05a); the live run used `gemini-embedding-2` @3072 because qwen3-via-OpenRouter was browning out (30s+/call vs. the hardcoded 30s client timeout at `embed.go:77`, so the eval couldn't complete). `ENGRAM_EMBED_QUERY_INSTRUCTION` was also unset, so PR#262 query-side asymmetry was not exercised. The D-06 mechanism itself is proven embedder-independently by `TestRerankParityMCPAndConnect` (tied-score corpus → only the lexical rerank can promote T); the live eval proves the end-to-end pipeline clears the #261 bar on a real recent embedder, but does NOT by itself isolate whether gemini alone would rank T=1 unaided. **Follow-up:** an operator should run the `qwen3-embedding-8b` @4096 parity eval (with the PR#262 query instruction) once OpenRouter recovers, to confirm the bar on the actual prod embedder.

## User Setup Required

None for this plan to merge — the reranker and its unit/behavioral tests run in CI's required `test` job without a gateway. The live retrieval eval remains env-gated (`ENGRAM_RETRIEVAL_EVAL=1` + `ENGRAM_OPENAI_*`/`ENGRAM_EMBED_*` + Docker), unchanged from Plan 01.

## Follow-Ups Filed

Operational papercuts surfaced while running the live eval (candidates; already captured in engram memory `7469a3d0`):

1. **Prod-embedder parity eval** — run `qwen3-embedding-8b` @4096 with `ENGRAM_EMBED_QUERY_INSTRUCTION` set once OpenRouter recovers, to confirm the #261 rank bar on the actual prod embedder (not just gemini @3072).
2. **`embed.go:191` base-URL joining** — appends `/v1/embeddings` to `ENGRAM_OPENAI_BASE_URL`; for OpenRouter the base must be `https://openrouter.ai/api` (not `.../api/v1`, which 404s). Consider documenting or normalizing.
3. **Non-configurable 30s embed client timeout** (`embed.go:77`) — a slow provider (qwen3 brownout) can't complete; candidate `ENGRAM_EMBED_TIMEOUT` + OpenRouter provider routing.

## Next Phase Readiness

- REQ-ranking-precision is closed for v0.9.x; the #261 fixture is a permanent green regression guard on the shipped path.
- Phase 10 (asymmetric query/document embeddings) can build on the shared reranked search path unchanged — the reranker is embedder-agnostic and runs strictly after owner-scoped retrieval.
- One deferred confirmation (prod-embedder parity eval) is documented as a follow-up, not a blocker.

---
*Phase: 09-retrieval-eval-harness-ranking-precision*
*Completed: 2026-07-09*

## Self-Check: PASSED

All created/modified files exist on disk; task commits `ef1829c` and `f07a642` are present in git history.
