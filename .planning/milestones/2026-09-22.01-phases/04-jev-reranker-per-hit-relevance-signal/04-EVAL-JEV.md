# Phase 4 Live Jev Retrieval Eval Results (D-02, RANK-03/04/05)

## Provenance

- Date: 2026-09-24T15:44:08Z
- Git SHA: `48924c3dc63929e9e9fee918b85d8e23fb0ece9b`
- Endpoint host: `openrouter.ai` (parsed from `ENGRAM_DECISIONS_BASE_URL`, host only — no userinfo, path or query)
- Configured decisions model: `typesafe/jev-1.13` (registry default — `ENGRAM_DECISIONS_MODEL` unset)
- Rerank timeout: `2s` (production default — `ENGRAM_SEARCH_RERANK_TIMEOUT` unset)
- Embed model: `google/gemini-embedding-2` (dim 3072)
- Command: `ENGRAM_DECISIONS_PROVIDER=jev ENGRAM_DECISIONS_BASE_URL=https://openrouter.ai/api ENGRAM_SEARCH_RANKER=jev task eval:retrieval` — one run, no local (`ENGRAM_RETRIEVAL_EVAL_*`) corpus override in this environment. The decisions API key was not set explicitly; it fell back to the exported `ENGRAM_OPENAI_API_KEY` per D-03 (`cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey)`).

Only one run was needed: the default-timeout run's `JEV-EVAL | fallbacks=0/26` line shows zero fallbacks, so the plan's long-timeout re-run condition (`N > 0`) was not triggered — no `04-EVAL-JEV-LONG-TIMEOUT.log` was created.

## Variant table (verbatim)

```
JEV-EVAL | enabled=true model=typesafe/jev-1.13 endpoint_host=openrouter.ai rerank_timeout=2s

| variant | #261 ranks | #261 all rank 1 | paraphrase recall@8 | paraphrase MRR | D-05 eligible |
|---|---|---|---|---|---|
| vector-only | [1 1] | true | 1.000 | 0.579 | yes |
| lexical | [1 1] | true | 0.950 | 0.817 | yes |
| overlap-gate-t0.90 | [1 1] | true | 1.000 | 0.579 | yes |
| overlap-gate-t0.75 | [1 1] | true | 1.000 | 0.579 | yes |
| overlap-gate-t0.60 | [1 1] | true | 1.000 | 0.579 | yes |
| cosine-blend-a0.05 | [1 1] | true | 1.000 | 0.628 | yes |
| cosine-blend-a0.10 | [1 1] | true | 1.000 | 0.631 | yes |
| cosine-blend-a0.20 | [1 1] | true | 1.000 | 0.704 | yes |
| cosine-blend-a0.30 | [1 1] | true | 1.000 | 0.792 | yes |
| jev | [1 1] | true | 1.000 | 0.883 | opt-in |
| shipped (SearchReranked) | [1 1] | true | 0.950 | 0.817 | — |
D-05 decision: winner=lexical reason=best eligible paraphrase MRR eligible=vector-only, lexical, overlap-gate-t0.90, overlap-gate-t0.75, overlap-gate-t0.60, cosine-blend-a0.05, cosine-blend-a0.10, cosine-blend-a0.20, cosine-blend-a0.30
D-10 gate PASS: #261 target at rank 1 for both queries under the shipped ranking
D-10 gate PASS: shipped paraphrase MRR 0.817 >= vector-only 0.579
shipped (SearchReranked) matches: lexical
JEV-EVAL | fallbacks=0/26
```

## No-answer relevance (verbatim)

```
no-answer paraphrase-blind-multidomain/T21: jev relevance=[0.030 0.020 0.020 0.020 0.020 0.020 0.020 0.020]
no-answer paraphrase-blind-multidomain/T22: jev relevance=[0.010 0.010 0.010 0.010 0.010 0.010 0.010 0.010]
no-answer paraphrase-blind-multidomain/T23: jev relevance=[0.030 0.020 0.020 0.020 0.020 0.020 0.020 0.020]
no-answer paraphrase-blind-multidomain/T24: jev relevance=[0.020 0.020 0.010 0.010 0.010 0.010 0.010 0.010]
```

## Reading

Jev (`jev` row, opt-in, structurally excluded from the D-05 winner decision per plan 04-06) scored the strongest numbers on this corpus: paraphrase recall@8 1.000 and paraphrase MRR 0.883, ahead of lexical (0.950 / 0.817, today's D-05 winner) and vector-only (1.000 / 0.579). On the #261 sticky-neighbor-crowding regression fixture, Jev also placed the target at rank 1 for both queries — matching every other eligible variant and the shipped ranking, so it introduces no regression there. Jev's per-hit no-answer relevance values across all four no-answer paraphrase queries (T21-T24) sit in the 0.010-0.030 range, close to the earlier decision-provider spike's ~0.02 reference for "this candidate does not answer the query" — consistent evidence that the per-hit relevance signal correctly identifies non-answers even when it is the top-ranked candidate by embedding similarity alone. `JEV-EVAL | fallbacks=0/26` — zero fallbacks to the underlying ranking at the production 2s rerank-timeout default, so these numbers already reflect Jev operating within its shipped production budget; no confounding from a tighter timeout, and no second (long-timeout) run was needed to get an unconfounded read.

Per locked decision D-02, these numbers are recorded next to lexical and vector-only and are not acted upon: no constant, question wording, corpus entry, or timeout default was changed after seeing them, and the Jev ranker ships as opt-in only (`ENGRAM_SEARCH_RANKER=jev`) — the default ranker stays lexical regardless of Jev's stronger measured numbers here. ROADMAP Phase 4 success criterion 1's "shipped only once the eval numbers justify it" is superseded by locked decision D-02: Jev ships as an opt-in reranker unconditionally, and this file's role is measurement-and-record, never a ship gate.
