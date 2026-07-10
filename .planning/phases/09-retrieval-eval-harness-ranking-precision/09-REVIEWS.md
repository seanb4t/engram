---
phase: 9
reviewers: [codex]
failed_reviewers: [gemini]
reviewed_at: 2026-07-09T20:56:47Z
review_round: 2
plans_reviewed: [09-01-PLAN.md, 09-02-PLAN.md, 09-03-PLAN.md]
---

# Cross-AI Plan Review — Phase 9 (Round 2)

> Round 2 reviews the plans as revised after round 1's Codex findings. Requested reviewers: `--codex --gemini`.
> **Codex** ran (source-grounded). **Gemini** failed — see note below.

## Codex Review

I verified the referenced source directly. No `.codegraph/` index exists in this checkout, so this is based on `rg`/line reads.

## 09-01 Summary
Good eval-first shape. The revised TestMain gate, fixture floor, and full embedder-parity intent address the prior review direction, but the plan still needs sharper implementation constraints around Qdrant testcontainer wiring and document embedding.

## 09-01 Strengths
- The env-gated eval mirrors existing precedent: `TestSummaryFidelity` skips unless `ENGRAM_SUMMARY_EVAL=1` in [fidelity_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/summarize/fidelity_test.go:38).
- The plan correctly requires gating before testcontainer startup. Existing Qdrant TestMain starts Docker at [store_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store_test.go:48), so putting `ENGRAM_RETRIEVAL_EVAL` first is the right fix for a new eval package.
- Full embedder parity is source-backed: `embedderFromConfig` is reached through `StoreAndEmbedderFromEnvNoEnsure` at [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:130), and `EmbedQuery` is the real search path at [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:720).

## 09-01 Concerns
- **MEDIUM:** `StoreAndEmbedderFromEnvNoEnsure` also constructs a Qdrant store from normal env config, not just an embedder. It loads config and calls `storeFromConfig` at [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:130), and `storeFromConfig` dials `cfg.Qdrant.Addr` at [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:76). If a developer runs the eval with prod-like env, this can point at the configured Qdrant rather than the testcontainer unless the plan requires `ENGRAM_QDRANT_ADDR=testQdrantAddr` before calling the builder, or avoids the builder for store construction entirely.
- **MEDIUM:** “Seed via the store’s document embedding path” is imprecise. `Store.Upsert` accepts a precomputed vector and does not call `EmbedText` ([store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:419)). The document embedding path is in the server handler: `Embed(ctx, store.EmbedText(...))` then `Upsert` at [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:507). Since `retrievaleval` cannot call unexported `storeMemory`, the plan should explicitly require `em.Embed(ctx, store.EmbedText(m.Content, m.Tags))`.
- **LOW:** “CI pays zero Docker/Qdrant cost” is overbroad. Existing `internal/store` and `internal/server` tests already start Qdrant testcontainers in `go test ./...` at [store_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store_test.go:48) and [tools_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:125). The accurate claim is “zero additional Docker cost from the eval package when gated off.”

## 09-01 Suggestions
- Add an acceptance criterion that the eval store is always created from `testQdrantAddr` with a unique collection, never from ambient `ENGRAM_QDRANT_ADDR`.
- Replace “store’s document embedding path” with the exact call sequence: `store.EmbedText` -> `em.Embed` -> `st.Upsert`.
- Treat “baseline starts failing” as expected evidence, not a hard precondition. If PR #262 already passes the fixture, the eval should record that honestly.

## 09-01 Risk Assessment
**MEDIUM.** The eval architecture is sound, but an ambiguous builder/store setup could accidentally measure or touch the wrong Qdrant.

## 09-02 Summary
This plan is appropriately narrow and source-backed. The score is already on the wire; docs are the right scope.

## 09-02 Strengths
- Score plumbing is real: `Memory.Score` is documented as Qdrant similarity at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:136), populated from `ScoredPoint.Score` at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:579), shaped into MCP compact recall at [summary.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/summary.go:89), and mapped to Connect at [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:32).
- The docs gap is real: `search_memory` docs currently just say “Returns a list” at [tools.md](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/reference/tools.md:100), while the embedding guide already mentions score at [embedding-instructions.md](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/guides/embedding-instructions.md:124).
- Avoiding output-schema work is correct. Current tool registration uses `(*mcp.CallToolResult, any, error)` at [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:937), matching the plan’s prose-only approach.

## 09-02 Concerns
- **MEDIUM:** If 09-03 lands reranking, `reference/tools.md` line 95 will become misleading because it says tag-filtered search is “ranked by vector similarity” ([tools.md](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/reference/tools.md:95)). After rerank, ordering may include lexical policy while `score` remains raw Qdrant similarity. 09-03 should include a docs follow-up.
- **LOW:** The `CLAUDE.md` memory contract currently says recall returns summaries but does not mention score at [CLAUDE.md](/Volumes/Code/github.com/seanb4t/engram/CLAUDE.md:59). The plan fixes this, but the wording should stay terse to avoid making the stable contract read like tuning docs.

## 09-02 Suggestions
- Add a 09-03 doc task or note now: “Final order may include reranking; `score` remains the raw vector similarity and may not be monotonic after rerank.”
- Keep `omitempty` wording exactly as planned; that matches `Score float32 json:"score,omitempty"` at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:139).

## 09-02 Risk Assessment
**LOW.** Documentation-only and well aligned with current code. Main risk is becoming stale immediately after 09-03.

## 09-03 Summary
The revised plan fixes the biggest prior issue: both MCP and Connect must share the ranking path. It correctly threads query text and vector into a shared helper. Remaining risks are behavioral test coverage, documentation after rerank, and a still-murky D-03 “score separation” reconciliation.

## 09-03 Strengths
- The shared-helper requirement is source-backed. MCP currently embeds and calls `Store.Search` at [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:720), while Connect separately embeds and calls `Store.Search` at [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:149). A shared `SearchReranked` helper is the right way to avoid drift.
- Query text is available at both call sites, so the proposed signature is feasible: MCP has `a.Query` at [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:704), Connect has `req.Msg.Query` at [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:144), and `Store.Search` already accepts the vector at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:544).
- Authz remains preservable if implemented as planned: `Store.Search` builds `ownerScopeFilter` before querying Qdrant at [store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:562), so over-fetching inside that same filter does not widen visibility.

## 09-03 Concerns
- **MEDIUM:** Grep-based acceptance is not enough for a behavior-changing shared helper. Existing Connect tests cover search isolation and tag filtering at [connectapi_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_test.go:118) and [connectapi_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi_test.go:331), but the plan should require tests proving MCP and Connect both preserve `k`, tags, created windows, and no private leaks through `SearchReranked`.
- **MEDIUM:** The D-03 reconciliation is internally inconsistent. The plan says raw-score separation is diagnostic, then says “score separation stays asserted/reported.” Since `Score` is raw Qdrant score ([store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:136)) and reranking can promote lower-score hits, rank-based acceptance is correct for #261, but the plan should explicitly supersede or narrow D-03 rather than claim a diagnostic is an assertion.
- **MEDIUM:** Docs need to change when reranking lands. `score` will still be raw Qdrant similarity, but result order may no longer be score-descending. Current docs say vector ranking at [tools.md](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/reference/tools.md:95), and 09-03 does not include docs files.
- **LOW:** The exported helper should define `k==0` behavior. Handlers currently default before search, MCP to 8 at [tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:704) and Connect to 20 at [connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:153), but an exported store helper should not silently overfetch then truncate to zero.
- **LOW:** Putting lexical ranking policy in `internal/store` is acceptable because it must reuse store filters, but it broadens store responsibility. Keep the API narrow and pure; avoid importing embed/server concepts into `store`.

## 09-03 Suggestions
- Add MCP and Connect integration tests that seed records and assert identical reranked IDs for the same query, plus no leakage across owners.
- Add a unit test for `candidateK` bounds, including small `k`, large `k`, and `k==0`.
- Add docs edits to 09-03 or make 09-02 depend on final ranking wording.
- Make the D-03 decision explicit: “Phase 9 acceptance is rank-based; raw score gap is diagnostic because score remains first-stage dense similarity.”

## 09-03 Risk Assessment
**MEDIUM.** The direction is right and prior high-risk gaps are mostly fixed, but this changes production ordering across two APIs. It needs stronger behavioral tests and clearer docs/decision language before execution.

---

## Gemini Review

**FAILED — not run.** `gemini-cli 0.46.0` returned `IneligibleTierError: UNSUPPORTED_CLIENT`:
the free "Gemini Code Assist for individuals" tier is no longer supported by this client;
Google directs users to migrate to the Antigravity suite (<https://antigravity.google>).
This is an account/product deprecation, not a transient failure — a flag cannot bypass it.
For a second independent reviewer, use `--agy` (Antigravity) or `--claude` / `--qwen`.

---

## Consensus Summary

**Round 2** of cross-AI review, on the plans as revised after round 1. Two reviewers were
requested (`--codex --gemini`); **only Codex ran** — Gemini failed with a hard
`IneligibleTierError` (`gemini-cli 0.46.0` on the free individual tier is no longer supported by
Google; it directs users to migrate to the Antigravity suite). This is an account/product
deprecation, not a transient error, so this round is effectively single-reviewer. To get a second
independent opinion, re-run with `--agy` (Antigravity, Google's named replacement) or `--claude` /
`--qwen`.

Codex again verified against source (`rg` + line reads; no `.codegraph/` index present) and cited
`file:line` evidence, so findings are weighted as verified.

### Round-1 HIGH fixes — CONFIRMED correct (not just present)

- **TestMain gate (09-01):** gating `ENGRAM_RETRIEVAL_EVAL` before testcontainer startup is the
  right fix; existing Qdrant TestMain starts Docker at `store_test.go:48`.
- **Shared helper covers both surfaces (09-03):** MCP (`tools.go:720`) and Connect
  (`connectapi.go:149`) both embed + call `Store.Search` today; a shared `SearchReranked` is the
  correct anti-drift move. The proposed signature is feasible — query text is available at both
  call sites (`tools.go:704` `a.Query`, `connectapi.go:144` `req.Msg.Query`) and `Store.Search`
  already takes the vector (`store.go:544`). Authz is preserved: over-fetch happens inside the
  existing `ownerScopeFilter` (`store.go:562`), so it does not widen visibility.
- **Rank-based #261 bar (09-03):** correct — `Score` is raw Qdrant similarity (`store.go:136`), and
  reranking can promote a lower-raw-score hit, so rank position is the right acceptance signal.
- **Always-on score (09-02):** re-confirmed end-to-end (`store.go:136/579` → `summary.go:89` →
  `connectapi.go:32`); docs gap real (`tools.md:100` "Returns a list"); prose-only correct
  (`Out=any` at `tools.go:937`).

### New / remaining concerns (priority order for a potential round-2 `--reviews`)

1. **[MEDIUM · 09-01] Eval could touch the WRONG Qdrant.** `StoreAndEmbedderFromEnvNoEnsure`
   (added for embedder parity) also builds a store from ambient env — it calls `storeFromConfig`
   which dials `cfg.Qdrant.Addr` (`tools.go:130`, `:76`). Running the eval with prod-like env could
   point it at the configured Qdrant instead of the testcontainer. **Fix:** require the eval to pin
   `ENGRAM_QDRANT_ADDR=<testQdrantAddr>` (unique collection) before calling the builder, or build
   the store from `testQdrantAddr` directly and use the builder for the embedder only. Add an
   acceptance criterion asserting the eval store is always the testcontainer, never ambient
   `ENGRAM_QDRANT_ADDR`.

2. **[MEDIUM · 09-01] "Document embedding path" is imprecise.** `Store.Upsert` takes a precomputed
   vector and does NOT call `EmbedText` (`store.go:419`); the real doc-embed path is the handler's
   `em.Embed(ctx, store.EmbedText(content, tags))` then `Upsert` (`tools.go:507`). Since
   `retrievaleval` can't call unexported `storeMemory`, the plan should spell out the exact seed
   sequence: `store.EmbedText` → `em.Embed` → `st.Upsert` (so tag-folding matches prod).

3. **[MEDIUM · 09-03] Grep acceptance is too weak for a behavior-changing shared helper.** "both
   files call `SearchReranked`" (grep) does not prove both behave correctly. Require MCP + Connect
   integration tests that seed records and assert identical reranked IDs for the same query, plus
   preserved `k` / tags / created-windows and no cross-owner leakage — existing
   `connectapi_test.go:118/331` are the precedent to extend.

4. **[MEDIUM · 09-03] D-03 language is internally inconsistent.** The plan says raw-score
   separation is "diagnostic" but also that "score separation stays asserted/reported." Pick one:
   make the rank bar the assertion and raw-score separation a `t.Logf` diagnostic, and **explicitly
   supersede/narrow D-03** in the plan text rather than have a diagnostic masquerade as an
   assertion.

5. **[MEDIUM · 09-02/09-03] Docs go stale when rerank lands.** `reference/tools.md:95` says
   tag-filtered search is "ranked by vector similarity" — after rerank, order may reflect lexical
   policy while `score` stays raw Qdrant similarity (no longer monotonic with order). 09-03 touches
   no docs files. **Fix:** add a docs task to 09-03 (or have 09-02 depend on final ranking wording):
   "Final order may include reranking; `score` remains first-stage dense similarity and may be
   non-monotonic after rerank."

6. **[LOW · 09-03] Define `k==0` for the exported helper.** Handlers default before search (MCP→8
   `tools.go:704`, Connect→20 `connectapi.go:153`); an exported `SearchReranked` should not silently
   over-fetch then truncate to zero. Specify its `k==0` contract.

7. **[LOW · 09-03] Keep `internal/store` narrow.** Housing lexical rerank in `store` is acceptable
   (it must reuse store filters) but broadens store's responsibility — keep the helper API pure;
   don't import `embed`/`server` concepts into `store`.

8. **[LOW · 09-01 wording] "zero Docker cost" is overbroad.** Other tests already start Qdrant
   testcontainers in `go test ./...` (`store_test.go:48`, `tools_test.go:125`). The accurate claim
   is **"zero *additional* Docker cost from the eval package when gated off."** Tighten the plan/
   threat wording.

### Divergent Views

None available — single successful reviewer (Gemini ineligible this round).

### Recommended Disposition

The round-1 HIGH fixes hold up. The remaining findings are mostly MEDIUM refinements, not
correctness blockers — concerns 1 and 3 are the most execution-relevant (wrong-Qdrant isolation;
real behavioral tests for the two-surface helper). Reasonable options:

- **Tighten now:** `/gsd-plan-phase 9 --reviews` once more to fold in 1–5 (env isolation, exact
  seed sequence, real MCP+Connect tests, D-03 supersession language, docs task). Cheap insurance
  before execution changes production ordering across two APIs.
- **Or proceed:** these are refinements an attentive executor can honor from this REVIEWS.md; 6–8
  are fine as executor guidance. If proceeding, run `/gsd-execute-phase 9` and treat this file as
  the executor's checklist.
