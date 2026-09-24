# Spike Wrap-Up Summary

**Date:** 2026-09-22
**Spikes processed:** 4
**Feature areas:** Decision transport, Curation verdicts, Recall reranking
**Skill output:** `./.claude/skills/spike-findings-engram/`

## Processed Spikes

| # | Name | Type | Verdict | Feature Area |
|---|------|------|---------|--------------|
| 001 | jev-openrouter-transport | standard | VALIDATED | Decision transport |
| 002 | jev-gateway-passthrough | standard | VALIDATED (gateway route added 2026-09-22) | Decision transport |
| 003 | jev-consolidate-verdicts | standard | VALIDATED | Curation verdicts |
| 004 | jev-rerank-eval | comparison | VALIDATED | Recall reranking |

## Key Findings

- **Transport:** Jev is reachable today at `https://openrouter.ai/api/alpha/decisions` with
  engram's existing OpenRouter key — p50 267 ms, one call per batch of questions, ≤ 255
  choices, 32k-token context, OpenRouter-wrapped errors classified by status. OpenRouter
  ships a Go SDK for the Decisions API. The LiteLLM gateway now serves it at
  `https://llm.fzymgc.house/openrouter/alpha/decisions` behind a per-key
  `allowed_passthrough_routes` grant (selfhosted-cluster PR #2227).
- **Curation:** on 39 real spine pairs, accuracy 0.84 and Brier 0.18; all 26 verdicts at
  p ≥ 0.9 were correct, every miss < 0.8. Misses are refinements and state changes read as
  `related` — an `updates` option is the next iteration. Strongest case for adoption.
- **Reranking:** on 16 paraphrase queries over the #261 corpus, Jev MRR 1.00 vs vector 0.92
  vs the shipped lexical reranker 0.66; Jev also gives an absolute no-answer signal (all
  0.02). Adds ~300–650 ms per search; needs a larger, independently written eval before a
  decision.
- **Side finding:** the shipped lexical reranker (`store.RerankHits`) degrades paraphrased
  queries versus plain vector order — worth a real retrieval-eval case regardless of Jev.
- **Side finding:** a stale spine record contradicts rule `rvmts69cz1` (it says not to pass
  `--reset-phase-numbers`).
