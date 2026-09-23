---
spike: 004
idea: jev-typed-decisions
name: jev-rerank-eval
type: comparison
validates: "Given the #261 corpus plus low-overlap paraphrase queries, when candidates are ordered by vector cosine, by engram's lexical reranker, and by Jev relevance scores, then Jev's ranking quality justifies its added latency"
verdict: VALIDATED
related: [001]
tags: [jev, rerank, retrieval-eval, gh-261, lexical-reranker, recall]
---

# Spike 004: rerank quality — vector vs lexical vs Jev

## What This Validates

Given the permanent #261 corpus (Record T plus 15 sticky topical neighbours, copied from
`internal/retrievaleval/fixtures.go`), when each query's candidates are ordered three ways:

- **(a) vector** — cosine similarity over the production embedder, `google/gemini-embedding-2`
  at 3072 dimensions, using `embed.EmbedQuery` for queries and `embed.Embed` for documents;
- **(b) lexical** — engram's shipped reranker, `store.RerankHits`, run over the vector order;
- **(c) Jev** — one Decisions request per query with one Noul question per candidate, "does
  record cNN directly answer the query?", sorted by P(yes) with ties kept in vector order;

then Jev's recall@k and MRR show whether a Jev reranker would earn its added latency.

The pieces are real: the production embedder and the real `store.RerankHits`. The Qdrant
search is replaced by an in-memory cosine ranking over 16 records, because every candidate
set here is the whole corpus: `candidateK` never goes below 32.

## How to Run

```sh
cd .planning/spikes/004-jev-rerank-eval
go run main.go          # ranking table + metrics; writes results.json
go run probe_edges.go   # no-answer and multi-answer queries
```

## Investigation Trail

1. **#261 alone cannot tell the rerankers apart.** Its two queries are near-verbatim
   restatements of Record T, so every ranker already scores MRR 1.0. It stays as the
   regression set.
2. **Added 16 paraphrase queries**, one per record, avoiding each record's tool names
   where possible, for example "How is the Kubernetes packaging validated?" for the
   `helm lint` record. **Bias warning:** they were written by someone who could see the
   targets. Each query has exactly one intended answer.
3. Ran all three rankers.
4. **Follow-ups**, because a perfect score calls for suspicion: (i) how wide the margin is
   between the intended record and the top result, (ii) queries that no record answers,
   (iii) broad queries that several records answer.

## Results

**VALIDATED**, with strong caveats about the fixture.

| Set | Ranker | R@1 | R@3 | MRR |
|---|---|---|---|---|
| gh261 (n=2) | vector / lexical / Jev | 1.00 | 1.00 | 1.000 |
| paraphrase (n=16) | vector | 0.88 | 0.94 | 0.922 |
| paraphrase (n=16) | **lexical (shipped)** | **0.50** | 0.81 | **0.656** |
| paraphrase (n=16) | **Jev** | **1.00** | 1.00 | **1.000** |

- **Jev ranks the intended record first on every query.** Its P(intended) never drops below 0.85.
- **Jev gives an absolute "nothing relevant" signal.** On three questions no record answers
  (Qdrant key rotation, the default embedding model, the release approver), every candidate
  scored **0.02**. Cosine similarity cannot say this: it always ranks something first. For
  recall this is arguably worth more than the reordering, because it lets `search_memory`
  mark an empty or irrelevant result set.
- **Broad queries get graded scores, not certainty.** "Which checks can fail CI?" put four
  records at 0.58–0.80. "What does `task lint` cover?" came out mixed, with the best
  answers at 0.49–0.66, and Record T itself was not in the top 5 for that phrasing.
  Reranking works best when a query has one answer.
- **Latency:** p50 312 ms, p90 643 ms, max 888 ms per query for 16 short candidates; the
  full run cost $0.0013. Production candidate sets are 32–100 records of up to 64 KiB, so
  expect more input tokens, higher latency, and a hit on the 32k-token context limit for
  large records. The state would need per-candidate truncation, or summaries in place of
  content.

**Side finding, independent of Jev: the shipped lexical reranker makes paraphrased
queries worse.** It drops R@1 from 0.88 (plain vector order) to 0.50. For para-03 it
pushed the intended record from rank 1 to rank 10, for para-10 from 4 to 10, and for
para-13 from 1 to 8. It was tuned for the #261 near-verbatim failure, and it promotes any
candidate that shares words with the query, such as `task`, `lint` and `fmt`. This
fixture is small and was written with the targets in view, so it is a signal, not a
verdict. It is still worth a real retrieval-eval case, because users usually recall by
paraphrase.

Caveats:

- 16 short, synthetic records in one tooling domain.
- Queries were written with the targets in view.
- Only one intended answer per query.

A credible rerank decision needs a larger, independently written query set over real
spine records.
