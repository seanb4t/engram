---
title: Jev reranker production telemetry — two-tier design (#618)
date: 2026-09-25
context: >-
  Surfaced from /gsd-explore after 12h of v0.20.0 running with
  ENGRAM_SEARCH_RANKER=jev on the cluster. ClickStack could report the decide
  span's cost/latency/status (23 calls, 0 fallbacks, $0.0053, +300ms p50 on
  tool/search_memory) but nothing about whether Jev changed any result order
  or whether the change was right. Issue #618 states the problem.
tags: [jev, rerank, telemetry, otel, span-attributes, audit-log, search_memory, cost-justification]
---

# Jev reranker production telemetry — two-tier design (#618)

## Purpose (decided 2026-09-25, Sean)

The data must support a **cost/complexity justification with real numbers** —
keep Jev on, turn it off, or tune it — not a dashboard toggle. Aggregate
counts alone cannot say whether a reorder was *right*; that needs individual
searches reconstructed and graded offline. Hence two tiers, both selected.

## Tier 1 — always-on bounded span attributes

Stamped on the **ambient** span via `trace.SpanFromContext(ctx)` (no new span):
`tool/search_memory` / `tool/search_discovery` for MCP, the RPC span for
Connect. First precedent of `internal/store` writing to a span it did not
open; chosen over a child span because every question is per-search and the
JOIN against `decide` was the painful part of the 12h read.

Seam: `store.applyRankHook` / `RankWithHook` (`internal/store/rerank.go`) —
the only place holding the lexical order, the Jev order, and the hook
outcome together. Displacement is measured within the **caller's top-k**
(what the caller saw), after truncation.

| attribute | type | meaning |
|---|---|---|
| `engram.rerank.outcome` | string | `applied` / `fallback` / `skipped` (empty pool); **absent** when the ranker is off |
| `engram.rerank.fallback_class` | string | stamped by `relevance.Hook` (`errClass`) — it owns the classification; store never imports relevance |
| `engram.rerank.top1_changed` | bool | first hit differs from lexical first hit |
| `engram.rerank.moved` | int | positions in top-k whose id differs from lexical |
| `engram.rerank.promoted` | int | ids in top-k that were beyond k in lexical order — the only thing the CandidateK over-fetch buys |
| `engram.rerank.relevance_max` | float | max relevance in the returned set; "no answer" is a query-time threshold, never baked in |

No query text, ids, or content — same rule the `decide` span and
`logFallback` already honor. No new metric initially: ClickStack counts
spans; `decide` already carries cost/latency.

## Tier 2 — opt-in audit capture (time-boxed)

`ENGRAM_SEARCH_RERANK_AUDIT=true`, default off, operator-owned. One
structured log line per reranked search:

- `query` (text), `owner`, `k`, `outcome`
- per candidate over the full CandidateK pool: `id`, `before_rank`,
  `after_rank` (1-based; "before" is lexical order for `search_memory`,
  vector order for `search_discovery` — hence not "lexical_rank"),
  `cosine` (Qdrant score), `relevance` (absent on fallback)
- **ids only, never content or summaries** — grading fetches content via
  `get_memory`. Decided over "plus summaries" (privacy footprint, 100-candidate
  lines) and over "top-k ids only" (loses relevance and the beyond-k pool, so
  you cannot see what Jev did *not* promote).

This is a deliberate exception to the "never the query" telemetry rule, which
is why it sits behind a flag with a loud startup log (the auth-disabled
pattern). Lands in ClickStack Logs; joins to Tier 1 by `trace_id`. No
sampling — volume is ~2 searches/hour today.

## Out of scope here

Grading itself (human or stronger-model judge over a captured sample) is a
later step — see seed `jev-cost-verdict-from-audit-sample`. The capture
format must make it trivial; the code change does not do it.

## Evidence base at decision time

- `04-EVAL-JEV.md` (2026-09-24): jev paraphrase MRR 0.883 vs lexical 0.817
  vs vector 0.579; no-answer relevance 0.01–0.03; 0/26 fallbacks at 2s.
- Cluster 2026-09-25 01:28Z–13:30Z: 23 `decide` spans, 23 `ok`, avg 271ms
  p95 405ms, 26.7 candidates avg, $0.0053 total (~$0.00023/search);
  `tool/search_memory` p50 406→706ms, p95 778→1019ms.
