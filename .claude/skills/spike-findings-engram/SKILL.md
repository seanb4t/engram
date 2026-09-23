---
name: spike-findings-engram
description: Implementation blueprint from engram spike experiments — Jev / System One typed decisions (transport via OpenRouter Decisions API, spine-review consolidate verdicts, recall reranking). Requirements, proven patterns, constraints, and gotchas. Load before building a decision client, advisory curation verdicts, or any search_memory reranking change.
---

<context>
## Project: engram

**jev-typed-decisions** — Use TypeSafe's Jev (System One typed-decision model) behind a
provider-neutral, advisory-only decision interface in engram, for spine-curation verdicts,
recall reranking, and write-time hints. Context: `.planning/notes/jev-system-one-decisions.md`,
`.planning/seeds/typed-decision-provider.md`.

Spike sessions wrapped: 2026-09-22
</context>

<requirements>
## Requirements (jev-typed-decisions)

- Provider-neutral interface using System One vocabulary (state + Choice/Score/Noul →
  probabilities); Jev is one backend, a chat-LLM emulator another.
- Off by default; advisory only — verdicts are surfaced, never acted on (reranking allowed).
- Transport goes through `/api/alpha/decisions`; engram's chat client cannot reach Jev.
- The decision client gets its own base-URL setting; it must not assume the chat/embeddings
  gateway serves Decisions.
- Spike fixtures built from real memory content are gitignored (public repo); commit only
  code, viewers and aggregate results.
</requirements>

<findings_index>
## Feature Areas

| Area | Reference | Key Finding |
|------|-----------|-------------|
| Decision transport | references/decision-transport.md | Works today via OpenRouter `/api/alpha/decisions` with engram's key (p50 ~270 ms); OpenRouter ships a Go SDK; LiteLLM gateway support in progress |
| Curation verdicts | references/curation-verdicts.md | 0.84 accuracy on real spine pairs; every verdict at p ≥ 0.9 correct; misses fall to `related` |
| Recall reranking | references/recall-rerank.md | Jev MRR 1.00 vs vector 0.92 vs shipped lexical 0.66 on paraphrases; lexical reranker hurts paraphrase recall |

## Source Files

Original spike source files are preserved in `sources/` for complete reference. Fixture
files containing verbatim memory content were never copied (local-only, gitignored).
</findings_index>

<metadata>
## Processed Spikes

- 001-jev-openrouter-transport
- 002-jev-gateway-passthrough
- 003-jev-consolidate-verdicts
- 004-jev-rerank-eval
</metadata>
