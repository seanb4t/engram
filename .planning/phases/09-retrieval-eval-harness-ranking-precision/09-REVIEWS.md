---
phase: 9
reviewers: [codex]
reviewed_at: 2026-07-09T19:00:59Z
plans_reviewed: [09-01-PLAN.md, 09-02-PLAN.md, 09-03-PLAN.md]
---

# Cross-AI Plan Review — Phase 9

> Reviewer: **Codex** (`codex-cli 0.143.0`, default model) — source-grounded (opened repo files, cited `file:line`).

## Codex Review

**Overall Assessment**

The phase strategy is sound: eval first, document the already-shipped score, then gate ranking changes on measured recall. The biggest risks are in execution details: Plan 09-01’s proposed `TestMain` can accidentally put Docker/Qdrant on the required CI path even when the eval is “skipped”; Plan 09-03’s hard “raw score separation” assertion may conflict with a post-search heuristic reranker; and the ranking fix only wires MCP `search_memory`, leaving Connect search unchanged.

## 09-01-PLAN.md

**Summary**

Good eval-first plan, and it matches existing repository patterns, but it needs two corrections before execution: gate Qdrant startup in `TestMain` before starting Docker, and make the #261 fixture large enough to fail meaningfully against default `k=8`.

**Strengths**

- Correctly mirrors the existing env-gated eval pattern: `TestSummaryFidelity` skips unless `ENGRAM_SUMMARY_EVAL=1` in [internal/summarize/fidelity_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/summarize/fidelity_test.go:38).
- Correctly identifies the production search path: MCP `searchMemory` defaults `k` to 8, calls `EmbedQuery`, then `Store.Search` in [internal/server/tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:704).
- Correctly uses the existing Qdrant testcontainer precedent from [internal/store/store_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store_test.go:48).
- Correctly keeps fixtures inline; I found no existing `testdata/` tree.

**Concerns**

- **HIGH:** Copying `store_test.go`/`server` `TestMain` literally means Qdrant starts before `TestRetrievalEval` can call `t.Skip`. CI runs `go test ./...` in [ci.yaml](/Volumes/Code/github.com/seanb4t/engram/.github/workflows/ci.yaml:32), and existing `TestMain` starts testcontainers at [internal/server/tools_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:125). The new eval package must check `ENGRAM_RETRIEVAL_EVAL` inside `TestMain` before Docker startup.
- **HIGH:** Acceptance only requires ≥2 distractors, but default `k` is 8 in [internal/server/tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:705). With fewer than 8 distractors, “target within default k” is nearly trivial and may not catch #261.
- **MEDIUM:** A new `internal/retrievaleval` package cannot call unexported `deps`, `searchArgs`, or `searchMemory`; these are package-private in [internal/server/tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:33) and [internal/server/tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:704). It can only mimic the handler unless moved into `package server` or a shared exported helper is added.
- **MEDIUM:** Prod embedder parity is incomplete if the eval only reads query instruction. Production also wires query params, document params, and document instruction in [internal/server/tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:215), with env registry entries in [internal/config/registry.go](/Volumes/Code/github.com/seanb4t/engram/internal/config/registry.go:32).

**Suggestions**

- Add an early `if os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1" { os.Exit(m.Run()) }` branch in the eval package `TestMain`.
- Require at least `defaultK + 1` sticky neighbors, preferably 12-20, and require the baseline run to record the pre-fix miss or poor target rank.
- Either move the eval into `internal/server` as `package server`, or extract a shared search helper so the eval and MCP handler cannot drift.
- Use the same config parsing as production, including `ENGRAM_EMBED_QUERY_PARAMS`, `ENGRAM_EMBED_DOCUMENT_PARAMS`, and `ENGRAM_EMBED_DOCUMENT_INSTRUCTION`.

**Risk Assessment: MEDIUM**

The concept is low-risk, but the CI/TestMain issue and weak fixture floor could undermine the whole eval gate.

## 09-02-PLAN.md

**Summary**

This is the strongest plan. The “score already shipped always-on” claim is true in the current code, and the proposed doc-only change is the right scope.

**Strengths**

- `Memory.Score` exists and is documented as Qdrant similarity, search-only, in [internal/store/store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:136).
- Qdrant scores are copied into memories in [internal/store/store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:578).
- Compact recall preserves `Score` in [internal/server/summary.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/summary.go:89).
- Connect maps `Score` onto the proto response in [internal/server/connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:32).
- The tool description currently omits score in [internal/server/tools.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools.go:937), and docs reference only says “Returns a list” in [docs-site/src/content/docs/reference/tools.md](/Volumes/Code/github.com/seanb4t/engram/docs-site/src/content/docs/reference/tools.md:85).
- The output-schema caveat is correct: MCP SDK omits output schema when `Out` is `any` in [/Users/sean/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.6.1/mcp/server.go](/Users/sean/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.6.1/mcp/server.go:495).

**Concerns**

- **LOW:** “present on ranked hits only” should be worded carefully because `omitempty` will omit an actual zero score in compact JSON. The field is populated in Go, but JSON visibility depends on non-zero value.

**Suggestions**

- Document it as “search results carry `score` when non-zero in JSON; unranked list/get results have zero/omitted score.”
- Add or update a small doc/test assertion if there is already a schema/prose registration test nearby; current `TestToolArgSchemasDoNotPanic` covers AddTool shape in [internal/server/tools_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/tools_test.go:34), not description content.

**Risk Assessment: LOW**

It is documentation-only and matches current behavior.

## 09-03-PLAN.md

**Summary**

The eval-gated escalation structure is good, but the concrete D-06 wiring has two important gaps: it bypasses Connect search, and its raw-score separation acceptance may be impossible for a heuristic reranker that changes order without changing Qdrant similarity scores.

**Strengths**

- Good constraint that usage signals must not affect ranking; store search currently filters by owner/scope/tags before returning results in [internal/store/store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:562).
- Good choice to start with a hermetic reranker unit test before hybrid/cross-encoder work.
- Conditional D-07/D-08 avoids speculative schema and gateway work.

**Concerns**

- **HIGH:** The hard assertion “Record T’s score is strictly above every sticky neighbor” conflicts with D-06 reranking. `Score` is raw Qdrant similarity copied from `ScoredPoint.Score` in [internal/store/store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:582). A lexical post-reranker can promote T while its raw vector score remains below a sticky neighbor. Failing the plan on that would reject a valid D-06 ranking fix or tempt changing score semantics.
- **HIGH:** Connect search is not covered. Connect `SearchMemories` embeds and calls `Store.Search` directly in [internal/server/connectapi.go](/Volumes/Code/github.com/seanb4t/engram/internal/server/connectapi.go:153), so a reranker wired only into MCP `searchMemory` leaves another recall surface unfixed.
- **MEDIUM:** The over-fetch strategy is underspecified. “limit ≥ k” can become no effective over-fetch, while an unbounded factor can amplify latency for large caller-supplied `k`; current `Store.Search` passes `Limit: k` directly to Qdrant in [internal/store/store.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/store.go:568).
- **MEDIUM:** If reranking changes result order, score descending is no longer guaranteed. Any eval assertion that assumes non-increasing raw scores after rerank will be wrong.

**Suggestions**

- Treat raw Qdrant score separation as diagnostic for D-06. If strict raw-score separation is truly required, then D-06 may be insufficient and should trigger D-07/D-08 rather than redefining `score`.
- Put reranking behind a shared helper used by MCP and Connect, or move it into a store/search service that receives both query text and vector.
- Define candidate over-fetch explicitly, e.g. `candidateK = min(max(k*4, 32), 100)`, then evaluate that latency/quality tradeoff.
- Make the fixture strong enough to prove promotion: target initially outside top 8 or below sticky neighbors, then inside top 8 after accepted ranking.

**Risk Assessment: MEDIUM-HIGH**

The escalation discipline is good, but the acceptance criteria need tightening to avoid either false failure or partial product behavior.

---

## Consensus Summary

Single external reviewer (**Codex**, `codex-cli 0.143.0`), so this is a synthesis of Codex's
source-grounded findings rather than a multi-reviewer consensus. Codex opened the referenced
files and cited `file:line` evidence, so these findings are weighted as verified, not
impressionistic.

### Verified TRUE (de-risks the plans — no action needed)

- **09-02's core claim holds:** the similarity `score` is already shipped always-on end-to-end —
  `store.go:136/578` (`Memory.Score` ← `ScoredPoint.Score`) → `summary.go:89` (compact recall
  preserves it) → `connectapi.go:32` (Connect maps it). The tool `Description` (`tools.go:937`)
  and docs (`reference/tools.md:85`) genuinely omit it. Doc-only scope confirmed correct.
- **`Out=any` → no output schema** verified against the go-sdk (`server.go:495`); documenting via
  `Description` prose (not a jsonschema) is the right channel.
- **Inline fixtures** are correct — no existing `testdata/` tree.
- **Eval-first / eval-gated escalation discipline** is sound; conditional D-07/D-08 correctly
  avoids speculative schema + gateway work.

### Agreed Concerns (priority order for `--reviews` replanning)

1. **[HIGH · 09-01] `TestMain` boots Qdrant before `t.Skip` can fire.** Copying the
   `store_test.go` / `tools_test.go:125` `TestMain` literally starts a testcontainer
   unconditionally; CI runs `go test ./...` (`ci.yaml:32`), so the required `test` job pays the
   Docker/Qdrant cost even when the eval is "skipped." **This partially undermines the
   zero-CI-cost claim.** Fix: `if os.Getenv("ENGRAM_RETRIEVAL_EVAL") != "1" { os.Exit(m.Run()) }`
   at the top of the eval package's `TestMain`, before any Docker startup.

2. **[HIGH · 09-03] Connect recall surface left unfixed.** Connect `SearchMemories`
   (`connectapi.go:153`) embeds and calls `Store.Search` directly, so a reranker wired only into
   MCP `searchMemory` (`tools.go:704`) fixes one of two recall paths. Wire D-06 behind a shared
   helper used by **both** MCP and Connect (this also resolves concern #5).

3. **[HIGH · 09-03] D-03 score-separation assertion conflicts with D-06 reranking.** `Score` is
   raw Qdrant similarity (`store.go:582`); a lexical post-reranker can promote Record T while its
   raw vector score stays below a sticky neighbor. A hard "T's raw score strictly above every
   neighbor" gate would reject a valid D-06 fix or tempt redefining `score` semantics. Treat raw
   score separation as **diagnostic**; if strict separation is truly required, that is evidence
   D-06 is insufficient → escalate to D-07/D-08, per the phase's own eval-gated logic. Also: once
   rerank changes order, results are no longer score-descending — drop any assertion assuming
   non-increasing raw scores.

4. **[HIGH · 09-01 / 09-03] Fixture floor too weak to catch #261.** Default `k` is 8
   (`tools.go:705`); requiring only ≥2 distractors makes "target within default k" nearly trivial.
   Require ≥ `defaultK + 1` (ideally 12–20) sticky neighbors, and have the baseline run record the
   pre-fix miss / poor target rank so the fixture proves a real promotion.

5. **[MEDIUM · 09-01] Package boundary.** A new `internal/retrievaleval` package cannot reach the
   unexported `deps` / `searchArgs` / `searchMemory` (`tools.go:33/704`). Either make the eval
   `package server`, or extract a shared exported search helper (dovetails with concern #2 — one
   helper serves MCP handler, Connect, and eval, preventing drift).

6. **[MEDIUM · 09-01] Prod embedder parity.** Production wires query params, document params, and
   document instruction (`tools.go:215`, `registry.go:32`), not just the query instruction. For a
   faithful baseline the eval should mirror `ENGRAM_EMBED_QUERY_PARAMS`,
   `ENGRAM_EMBED_DOCUMENT_PARAMS`, `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` — else it measures a
   different embedding than prod. (Note: query/doc asymmetry is Phase 10 #305 territory; keep the
   eval config-faithful without pulling Phase-10 scope forward.)

7. **[MEDIUM · 09-03] Over-fetch underspecified.** "limit ≥ k" can collapse to no over-fetch,
   while an unbounded factor amplifies latency for large caller `k` (`Store.Search` passes
   `Limit: k` straight to Qdrant, `store.go:568`). Define it explicitly, e.g.
   `candidateK = min(max(k*4, 32), 100)`.

- **09-02 (LOW):** word the score doc as "search results carry `score` when non-zero in JSON;
  unranked list/get results have zero/omitted score" (`omitempty` omits an actual zero).

### Divergent Views

None — single reviewer.

### Recommended Disposition

Concerns 1–4 are worth a targeted replan before execution — 1 and 2 are correctness/coverage gaps,
3 is a real acceptance-criteria conflict, 4 sharpens the regression fixture that is the whole point
of the phase. Run: `/gsd-plan-phase 9 --reviews`. Concerns 5–7 can be handled at plan-revision or
left as executor guidance in the affected tasks' `<read_first>` / `<action>`.
