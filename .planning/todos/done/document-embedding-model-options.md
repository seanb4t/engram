> **Deferred at v0.9.x milestone close (2026-07-10) → filed as GitHub #337:** https://github.com/seanb4t/engram/issues/337

---
title: Document embedding model options in docs-site + Helm chart
date: 2026-07-09
priority: medium
tags: [docs, helm-chart, embedding, openrouter, gemini, reindex]
---

# Document embedding model options in docs-site + Helm chart

## Why

Operators currently have no guidance on which embedding model to run or how to
switch. Prod hit repeated brownouts on `qwen3-embedding-8b` via OpenRouter, and
choosing an alternative (Gemini, local, etc.) silently requires matching the
Qdrant vector dim + a reindex. Document the supported options and their exact
config so a switch is a recipe, not a research project.

See `.planning/notes/embedding-model-options.md` for the landscape + findings.

## Deliverables

### 1. docs-site guide (`guides/embedding-models` or similar)

- Overview: engram's embedder is OpenAI-compatible (`ENGRAM_EMBED_*`), so any
  OpenAI-compatible endpoint works — OpenRouter, Google (OpenAI-compat), OpenAI,
  or a local server (TEI / Infinity / Ollama / vLLM).
- **Separate the two axes** the note calls out:
  - *Which model* → recall quality (link Phase 9 eval once it lands).
  - *Reliability* → direct-to-provider vs OpenRouter, fallbacks.
- Per-model recipe table, each with:
  - base URL, `ENGRAM_EMBED_MODEL` (correct namespaced slug),
  - **vector dim → Qdrant vector size** (the part that bites),
  - query instruction / prefix if the model needs one
    (`ENGRAM_EMBED_QUERY_INSTRUCTION`),
  - **mandatory `engram reindex` step** when changing model/dim
    (cross-link the existing `guides/reindex`).
- Reliability section: OpenRouter provider routing / `allow_fallbacks`, the
  `529 Provider Overloaded` failure mode, and "prefer direct-to-provider for
  a stable prod default."
- Privacy note: routing memory content through OpenRouter vs local embedding.

### 2. Helm chart (`charts/engram/`)

- Add commented `values.yaml` examples for the embedder block covering at least:
  reliability-first hosted (Gemini direct), OpenRouter multi-model, and local
  (TEI/Ollama). Each example pairs model + dim + any query instruction.
- Ensure the vector-dim value is surfaced/parameterized so a model change and
  the Qdrant vector size stay in sync (verify current chart wiring).

## Notes / gotchas

- **Verify exact native dims** at implementation (table in the note has some
  marked "verify") — Qdrant vector size must match exactly.
- Confirm whether engram's embedder passes extra OpenRouter body params
  (`provider`, `task_type`) — see the resilience seed; may constrain what the
  docs can promise until that code lands.
- Keep model slugs namespaced (`openai/…`, `qwen/…`, `google/…`).
