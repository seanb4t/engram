# Recall reranking (search_memory)

## Requirements

- Provider-neutral decision interface (see `decision-transport.md`); Jev is one backend.
- Off by default. Reranking is inside the advisory contract (reordering is not a mutation),
  but it adds latency to every search, so it must be an explicit opt-in.
- Any ranking change is gated on `task eval:retrieval` (recall@k / MRR), not on anecdotes.

## How to Build It

1. Keep the vector fetch as-is (`SearchReranked`'s `candidateK` over-fetch, 32–100 hits).
2. Send ONE Decisions request per search: `state = {query, c00: <content>, c01: ..., ...}`,
   one Noul question per candidate:
   - instructions: "Does memory record cNN directly answer the query?"
   - criteria: true = "The record directly answers what the query asks."; false = "The record
     is about something else, or only shares vocabulary with the query."
3. Sort by P(yes), **stable**, so ties keep vector order; truncate to k.
4. Expose the per-hit relevance probability next to the existing cosine `score` — it is an
   absolute signal ("nothing here answers this": all ≈ 0.02) that cosine cannot give. A
   future `search_memory` could flag a no-relevant-results response from it.
5. Budget the state: 32k-token context across ALL candidates. With 32–100 candidates of up
   to 64 KiB each, send summaries (or per-candidate truncation), not full content.
6. Before building, add a paraphrase case to `internal/retrievaleval` written by someone who
   does not see the targets, over real spine records — the spike fixture is too small and
   target-aware to decide on.

## What to Avoid

- Do not trust the shipped lexical reranker (`store.RerankHits`) on paraphrased queries: in
  the spike it dropped R@1 from 0.88 (plain vector) to 0.50 and pushed intended records to
  rank 8–10, because it promotes any candidate sharing tool words (`task`, `lint`, `fmt`).
  It was tuned for #261's near-verbatim failure. Confirm with a real eval case, then decide
  whether it should be demoted or gated — independent of Jev.
- Do not expect certainty on broad queries: "what does `task lint` cover?" produced mixed
  0.29–0.66 scores and missed Record T in the top 5. Reranking helps most when a query has
  one answer.
- Do not put Jev on the synchronous path without a timeout fallback to vector order.

## Constraints

| Measure (#261 corpus, 16 records) | Vector | Lexical (shipped) | Jev |
|---|---|---|---|
| gh261 near-verbatim (n=2) MRR | 1.000 | 1.000 | 1.000 |
| paraphrase (n=16) R@1 | 0.88 | 0.50 | 1.00 |
| paraphrase (n=16) MRR | 0.922 | 0.656 | 1.000 |

- Jev P(intended) ≥ 0.85 on every paraphrase query; no-answer queries → all candidates 0.02.
- Added latency (16 short candidates): p50 312 ms · p90 643 ms · max 888 ms. Expect more
  with production-size candidate sets.
- Embedder used: `google/gemini-embedding-2` @ 3072 via `embed.EmbedQuery` / `embed.Embed`.
- Fixture caveats: synthetic single-domain records, queries written with targets in view,
  one intended answer per query.

## Origin

Synthesized from spikes: 004 (transport facts from 001)
Source files available in: sources/004-jev-rerank-eval/
