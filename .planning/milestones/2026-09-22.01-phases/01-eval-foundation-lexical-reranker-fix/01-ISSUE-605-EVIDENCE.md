Shipped the D-05 winner: `store.RerankHits` (lexical reranking) stays `SearchReranked`'s single rank step, applied through the new `rankCandidates` seam — the pre-committed D-05 rule ("among variants that (a) keep #261's target at rank 1 for both #261 queries and (b) have paraphrase MRR ≥ vector-only MRR, ship the one with the best paraphrase MRR. A tuned variant (blend or gate) wins over vector-only only if it beats vector-only paraphrase MRR by a clear margin (≥ 0.05); otherwise the simpler variant wins. Ties go to the simplest.") selected `lexical` as the best-eligible variant by paraphrase MRR on the live blind multi-domain corpus, and the human checkpoint approved that winner.

**Embedder:** `google/gemini-embedding-2`, dim 3072.

## Before (pre-change baseline, `01-EVAL-BASELINE.log`)

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
| jev | Jev: disabled | — | — | — | — |
| shipped (SearchReranked) | [1 1] | true | 0.950 | 0.817 | — |

`D-05 decision: winner=lexical reason=best eligible paraphrase MRR eligible=vector-only, lexical, overlap-gate-t0.90, overlap-gate-t0.75, overlap-gate-t0.60, cosine-blend-a0.05, cosine-blend-a0.10, cosine-blend-a0.20, cosine-blend-a0.30`

`shipped (SearchReranked) matches: lexical`

## After (post-change re-run, `01-EVAL-SHIPPED.log`)

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
| jev | Jev: disabled | — | — | — | — |
| shipped (SearchReranked) | [1 1] | true | 0.950 | 0.817 | — |

`D-05 decision: winner=lexical reason=best eligible paraphrase MRR eligible=vector-only, lexical, overlap-gate-t0.90, overlap-gate-t0.75, overlap-gate-t0.60, cosine-blend-a0.05, cosine-blend-a0.10, cosine-blend-a0.20, cosine-blend-a0.30`

`shipped (SearchReranked) matches: lexical`

Both D-10 gates on the post-change run:

```
D-10 gate PASS: #261 target at rank 1 for both queries under the shipped ranking
D-10 gate PASS: shipped paraphrase MRR 0.817 >= vector-only 0.579
```

The re-run's `D-05 decision: winner=lexical` and `shipped (SearchReranked) matches: lexical` lines are byte-identical to the pre-change baseline — the ranking shipped, the D-05 rule's own re-application, and `01-RANKING-DECISION.md`'s `Approved winner: lexical` all agree.

## No-answer queries (D-12, post-change run)

These four queries have no correct target in the corpus and are excluded from recall@k/MRR — only the top hit and its score are logged, one line per enabled roster entry plus the shipped row:

```
no-answer paraphrase-blind-multidomain/T21: shipped (SearchReranked) top=auth-03 score=0.578410
no-answer paraphrase-blind-multidomain/T21: vector-only top=auth-06 score=0.626070
no-answer paraphrase-blind-multidomain/T21: lexical top=auth-03 score=0.578410
no-answer paraphrase-blind-multidomain/T21: overlap-gate-t0.90 top=auth-06 score=0.626070
no-answer paraphrase-blind-multidomain/T21: overlap-gate-t0.75 top=auth-06 score=0.626070
no-answer paraphrase-blind-multidomain/T21: overlap-gate-t0.60 top=auth-06 score=0.626070
no-answer paraphrase-blind-multidomain/T21: cosine-blend-a0.05 top=auth-06 score=0.626070
no-answer paraphrase-blind-multidomain/T21: cosine-blend-a0.10 top=deploy-02 score=0.613434
no-answer paraphrase-blind-multidomain/T21: cosine-blend-a0.20 top=deploy-02 score=0.613434
no-answer paraphrase-blind-multidomain/T21: cosine-blend-a0.30 top=deploy-02 score=0.613434
no-answer paraphrase-blind-multidomain/T22: shipped (SearchReranked) top=datamodel-15 score=0.568204
no-answer paraphrase-blind-multidomain/T22: vector-only top=datamodel-14 score=0.601522
no-answer paraphrase-blind-multidomain/T22: lexical top=datamodel-15 score=0.568204
no-answer paraphrase-blind-multidomain/T22: overlap-gate-t0.90 top=datamodel-14 score=0.601522
no-answer paraphrase-blind-multidomain/T22: overlap-gate-t0.75 top=datamodel-14 score=0.601522
no-answer paraphrase-blind-multidomain/T22: overlap-gate-t0.60 top=datamodel-14 score=0.601522
no-answer paraphrase-blind-multidomain/T22: cosine-blend-a0.05 top=datamodel-14 score=0.601522
no-answer paraphrase-blind-multidomain/T22: cosine-blend-a0.10 top=datamodel-14 score=0.601522
no-answer paraphrase-blind-multidomain/T22: cosine-blend-a0.20 top=tooling-12 score=0.587373
no-answer paraphrase-blind-multidomain/T22: cosine-blend-a0.30 top=datamodel-15 score=0.568204
no-answer paraphrase-blind-multidomain/T23: shipped (SearchReranked) top=testing-13 score=0.548974
no-answer paraphrase-blind-multidomain/T23: vector-only top=datamodel-03 score=0.607509
no-answer paraphrase-blind-multidomain/T23: lexical top=testing-13 score=0.548974
no-answer paraphrase-blind-multidomain/T23: overlap-gate-t0.90 top=datamodel-03 score=0.607509
no-answer paraphrase-blind-multidomain/T23: overlap-gate-t0.75 top=datamodel-03 score=0.607509
no-answer paraphrase-blind-multidomain/T23: overlap-gate-t0.60 top=datamodel-03 score=0.607509
no-answer paraphrase-blind-multidomain/T23: cosine-blend-a0.05 top=datamodel-03 score=0.607509
no-answer paraphrase-blind-multidomain/T23: cosine-blend-a0.10 top=datamodel-03 score=0.607509
no-answer paraphrase-blind-multidomain/T23: cosine-blend-a0.20 top=datamodel-03 score=0.607509
no-answer paraphrase-blind-multidomain/T23: cosine-blend-a0.30 top=datamodel-03 score=0.607509
no-answer paraphrase-blind-multidomain/T24: shipped (SearchReranked) top=config-01 score=0.536800
no-answer paraphrase-blind-multidomain/T24: vector-only top=deploy-03 score=0.563062
no-answer paraphrase-blind-multidomain/T24: lexical top=config-01 score=0.536800
no-answer paraphrase-blind-multidomain/T24: overlap-gate-t0.90 top=deploy-03 score=0.563062
no-answer paraphrase-blind-multidomain/T24: overlap-gate-t0.75 top=deploy-03 score=0.563062
no-answer paraphrase-blind-multidomain/T24: overlap-gate-t0.60 top=deploy-03 score=0.563062
no-answer paraphrase-blind-multidomain/T24: cosine-blend-a0.05 top=deploy-01 score=0.557938
no-answer paraphrase-blind-multidomain/T24: cosine-blend-a0.10 top=deploy-01 score=0.557938
no-answer paraphrase-blind-multidomain/T24: cosine-blend-a0.20 top=config-01 score=0.536800
no-answer paraphrase-blind-multidomain/T24: cosine-blend-a0.30 top=config-01 score=0.536800
```

## Provenance and reproduction

The 24 paraphrase queries were written **blind** to their targets (D-01): a fresh, tool-less subagent authored them from topic labels only, with the target mapping done afterward in a separate, non-blind pass. This is what makes the paraphrase MRR gap between `vector-only` (0.579) and `lexical` (0.817) a real signal rather than an artifact of query authorship.

Reproduce with `task eval:retrieval` (needs a live Qdrant and the production embedding gateway).

## Caveats

1. This result reverses spike 004's finding: here, lexical MRR 0.817 beats vector-only MRR 0.579 on the live 80-120-record multi-domain corpus with independently-authored blind paraphrase queries. Spike 004 measured the opposite — lexical MRR 0.656 vs vector-only MRR 0.922 — on a 16-record single-domain target-aware fixture. The two runs are not comparable measurements of the same claim; the live result supersedes the spike finding for shipping purposes.
2. Lexical's paraphrase recall@8 is 0.950 vs vector-only's 1.000 — one target falls out of the default k=8 window under the lexical ranking that vector-only would have retrieved.
3. `TestRetrievalEval_AsymmetryDiffer` SKIPPED on this live run because the operator's embed config (`google/gemini-embedding-2`) is symmetric. EVAL-01's cosine asymmetry gate is therefore covered only by the hermetic unit tests in this run, not by a live asymmetric-embedder observation.

Closes the acceptance criteria: the retrieval eval reports recall@k/MRR for both the near-verbatim and paraphrase sets, the shipped ranking does not regress the paraphrase set against vector-only order (0.817 ≥ 0.579), and #261 stays at rank 1.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
