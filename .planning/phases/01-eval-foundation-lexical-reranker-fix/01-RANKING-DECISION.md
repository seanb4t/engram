# Phase 1 Live Retrieval-Eval Baseline: D-05 Ranking Decision

## Measurement

- Date: 2026-09-23T03:39:55Z
- Git SHA: `5bc6b9fa5f82ea744ac803a4c1366d88bc82b351`
- Embedder model: `google/gemini-embedding-2`
- Embedder dim: `3072`
- `ENGRAM_EMBED_QUERY_INSTRUCTION` set: no
- `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` set: no

## Table

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

## Rule verdict (D-05, applied by decideRanking)

D-05 decision: winner=lexical reason=best eligible paraphrase MRR eligible=vector-only, lexical, overlap-gate-t0.90, overlap-gate-t0.75, overlap-gate-t0.60, cosine-blend-a0.05, cosine-blend-a0.10, cosine-blend-a0.20, cosine-blend-a0.30

Rule winner: lexical

Eligible set: vector-only, lexical, overlap-gate-t0.90, overlap-gate-t0.75, overlap-gate-t0.60, cosine-blend-a0.05, cosine-blend-a0.10, cosine-blend-a0.20, cosine-blend-a0.30

Deciding clause: best eligible paraphrase MRR — `lexical` (0.817) beat every other eligible variant's paraphrase MRR, including `vector-only` (0.579) and the best cosine-blend grid point, `cosine-blend-a0.30` (0.792). No margin gate applied here: D-05's 0.05 margin only gates a *tuned* candidate against vector-only when vector-only would otherwise win outright; here `lexical` already has the outright-best eligible MRR, so the margin clause never triggers.

## Gates on today's shipped ranking

D-10 gate PASS: #261 target at rank 1 for both queries under the shipped ranking

D-10 gate PASS: shipped paraphrase MRR 0.817 >= vector-only 0.579

## Shipped matches

shipped (SearchReranked) matches: lexical

This names `lexical`, as expected before any ranking change — the eval's lexical port matches what ships today.

## Plan 01-06 branch

lexical

The rank step stays the lexical reranker. On this live measurement, `decideRanking` selected `lexical` as the best-eligible variant by paraphrase MRR (0.817), ahead of `vector-only` (0.579) and every cosine-blend/overlap-gate grid point measured. This is the opposite of the spike-004 finding recorded in `recall-rerank.md` (lexical MRR 0.656 vs vector-only MRR 0.922 on a 16-record single-domain fixture); the live 80–120-record multi-domain corpus and independently-authored blind queries produced a different outcome. No code change to `internal/store`'s ranking step is required for plan 01-06 under this branch.

## No-answer queries (D-12)

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
