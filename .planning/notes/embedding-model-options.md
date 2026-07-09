---
title: Embedding model options + OpenRouter reliability findings
date: 2026-07-09
context: >-
  Surfaced from /gsd-explore. Trigger: repeated brownouts/timeouts on
  qwen3-embedding-8b via OpenRouter in engram prod. Goal was to survey
  alternative embedders (esp. via OpenRouter) and decide what to document.
tags: [embedding, openrouter, gemini, reliability, qdrant, reindex, phase-9, phase-10]
---

# Embedding model options + OpenRouter reliability findings

## TL;DR

- **OpenRouter now has a native embeddings endpoint** (`POST
  https://openrouter.ai/api/v1/embeddings`, OpenAI-compatible). This is newer
  than prior assumption — embeddings used to be chat/completion-only.
- The **Qwen brownouts are an upstream-provider problem, not a model-quality
  problem.** `qwen3-embedding-8b` is a $0.01/M model served by budget upstreams
  that overload (`529 Provider Overloaded`). Fix is provider routing / fallback
  or a single-reliable-provider model — not necessarily a different embedder.
- **"Which model" (recall) and "how to make it reliable" (routing) are separate
  axes.** The docs must treat them separately.
- **Dim is the load-bearing config.** Qdrant here is a fixed-size single dense
  vector (4096 today). Each model has a different native dim, so every switch is
  an `engram reindex`.

## Models available through OpenRouter (as of 2026-07-09)

Slugs are namespaced `provider/model`. Prices are per 1M tokens.

| Slug | Provider | Ctx | $/M | Native dim (verify) | Notes |
|---|---|---|---|---|---|
| `perplexity/pplx-embed-v1-0.6b` | Perplexity | 32K | $0.004 | (verify) | cheapest listed |
| `qwen/qwen3-embedding-8b` | Qwen | 32K | $0.01 | 4096 | **current prod**; Matryoshka |
| `baai/bge-m3` | BAAI | 8K | $0.01 | 1024 | other current model |
| `qwen/qwen3-embedding-4b` | Qwen | 33K | $0.02 | 2560 | cheaper Qwen3 |
| `openai/text-embedding-3-small` | OpenAI | 8K | $0.02 | 1536 | Matryoshka via `dimensions` |
| `openai/text-embedding-3-large` | OpenAI | 8K | $0.13 | 3072 | strong baseline, Matryoshka |
| `mistral/mistral-embed-2312` | Mistral | 8K | $0.10 | 1024 | |
| `google/gemini-embedding-001` | Google | 20K | $0.15 | 3072 (768/1536/3072) | native `task_type` asymmetry |
| `google/gemini-embedding-2` | Google | 8K | $0.20 | 128–3072 (Matryoshka) | pick your dim |
| `nvidia/llama-nemotron-embed-vl-1b-v2:free` | NVIDIA | 131K | free | ~2048 (verify) | multimodal (text+image) |

(Programmatic list: `GET /api/v1/embeddings/models`. Browse:
`https://openrouter.ai/models?fmt=cards&output_modalities=embeddings`.)

## Reliability findings

- **Root cause of Qwen brownouts:** cheap model → budget upstream providers →
  overload. OpenRouter documents the exact remedy on the embeddings endpoint:
  provider routing —
  `{"provider": {"order": ["..."], "allow_fallbacks": true, "data_collection": "deny"}}`.
- **engram gap:** engram's embedder is a plain OpenAI-compatible client and
  almost certainly does **not** send a `provider` block today (confirm against
  `internal/embed/`). So to use OpenRouter fallbacks we'd need a code change, or
  switch to a model whose upstream is a single reliable provider.
- **Gemini for reliability — validated, with a twist:** Gemini routes to Google
  (single high-availability provider), sidestepping multi-provider overload.
  But for *reliability specifically*, pointing engram **directly at Google's
  OpenAI-compat endpoint** beats going through OpenRouter (one less hop /
  dependency). Direct-to-Google also unlocks Gemini's native asymmetry
  (`task_type: RETRIEVAL_QUERY` vs `RETRIEVAL_DOCUMENT`), which OpenRouter's
  OpenAI-shaped body does **not** expose — this is exactly the Phase 10 (#305)
  asymmetric-params question.
- **NVIDIA `:free` — unfit for prod.** OpenRouter free variants are hard-capped
  at **20 req/min and ~50 req/day** and can be pulled anytime. A memory server
  embeds on every store *and* every search, so 50/day is unusable. Also a 1B
  multimodal model (weaker pure-text retrieval than Qwen3-8B / Gemini). Fine as
  an eval/experiment candidate only.

## engram-specific implications

1. **Every model swap = `engram reindex`** (dim change on the single dense
   Qdrant vector). Matryoshka models (text-embedding-3, Gemini, Qwen3) let you
   *choose* the dim — a lever for index size vs. recall.
2. **Asymmetry conventions differ per model** and collide with Phase 10 (#305):
   Qwen3 wants an `Instruct:`-style query prompt (engram already ships
   `ENGRAM_EMBED_QUERY_INSTRUCTION`); Gemini wants a native `task_type` param;
   text-embedding-3 has no instruction concept.
3. **Privacy:** routing self-hosted memory content through OpenRouter proxies it
   to a downstream provider. `bge-m3` and the Qwen3 models can also run locally
   (TEI / Infinity / Ollama / vLLM) — same OpenAI-compatible interface, no
   per-token cost, nothing leaves the box.

## Provisional recommendation (not yet decided)

- **Reliability-first prod default:** `google/gemini-embedding-001` — ideally
  **direct to Google** (not via OpenRouter) for the SLA + native `task_type`.
  Cost (~$0.15/M) is negligible on a memory server's modest embedding volume.
- **Keep OpenRouter as the multi-model swap surface** for eval/experimentation
  and cheaper options.
- **Decide empirically** via the Phase 9 eval harness (recall@k / MRR on
  engram's own data) rather than MTEB rank.

## Sources

- https://openrouter.ai/docs/api/reference/embeddings
- https://openrouter.ai/collections/embedding-models
- https://openrouter.ai/docs/api/reference/limits (free-tier caps)
