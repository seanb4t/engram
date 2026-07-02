---
title: Embedding query instructions
description: Tune recall quality with ENGRAM_EMBED_QUERY_INSTRUCTION/ENGRAM_EMBED_DOCUMENT_INSTRUCTION and ENGRAM_EMBED_QUERY_PARAMS/ENGRAM_EMBED_DOCUMENT_PARAMS — when to set each, and the exact value for Qwen3, BGE, E5, nomic, OpenAI, Google, Cohere, Voyage, and Jina embedders.
---

Many retrieval embedders are **asymmetric**: they expect the search *query* to be
wrapped with a task instruction while the stored *document* is embedded raw.
Sending both sides through the identical raw path — which engram did before
`v0.9` — leaves recall quality on the table and makes ranking sensitive to how a
query is phrased.

`ENGRAM_EMBED_QUERY_INSTRUCTION` fixes this for text-prefix embedders. It changes
only the **query** embedding, so documents are untouched and **no reindex is
needed** to adopt it. For models that need a prefix on both sides, and for cloud
providers whose asymmetry is an API parameter rather than a text prefix,
`ENGRAM_EMBED_DOCUMENT_INSTRUCTION` and `ENGRAM_EMBED_QUERY_PARAMS`/
`ENGRAM_EMBED_DOCUMENT_PARAMS` cover the rest — see the sections below.

## Hot vs reindex-gated

- **Hot (no reindex):** `ENGRAM_EMBED_QUERY_INSTRUCTION`, `ENGRAM_EMBED_QUERY_PARAMS` — they change only the query vector at search time.
- **Reindex-gated:** `ENGRAM_EMBED_DOCUMENT_INSTRUCTION`, `ENGRAM_EMBED_DOCUMENT_PARAMS` (and tags-in-vector) — they change the stored document vector, so existing records need `engram reindex` + a collection cutover (see [Reindex](/guides/reindex/)).

## How the knob behaves

`ENGRAM_EMBED_QUERY_INSTRUCTION` is applied by `EmbedQuery` (used by
`search_memory` and `search_discovery`); it never affects the store/update/reindex
paths, which embed documents per `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` (raw by
default — see the "both-side prefix" section below). Three modes, chosen by the
`ENGRAM_EMBED_QUERY_INSTRUCTION` value:

- **Empty (default):** the query is embedded raw — symmetric behavior, unchanged
  from before. Correct for symmetric models.
- **Contains `{query}`:** the value is used verbatim as a template with `{query}`
  replaced by the search text. Use this for plain-prefix models.
- **Otherwise:** the value is wrapped in the Qwen3-family template
  `Instruct: <value>\nQuery: <text>`. Use this for instruction-tuned embedders.

## Which value for which model

engram calls an OpenAI-compatible `/v1/embeddings` endpoint, so the model is
whatever your `ENGRAM_EMBED_MODEL` / gateway resolves to. Pick the row that
matches it.

### Instruction-tuned (query instruction, documents raw) — supported

These embed the query with a task instruction and the document raw — exactly what
this knob does.

| Model family | `ENGRAM_EMBED_QUERY_INSTRUCTION` |
| --- | --- |
| Qwen3-Embedding (`0.6B` / `4B` / `8B`) | `Given a web search query, retrieve relevant passages that answer the query` |
| gte-Qwen2-\*-instruct, e5-mistral-7b-instruct, SFR-Embedding-2 | same task string (same `Instruct:/Query:` template) |
| BAAI bge-\*-en-v1.5 (`large` / `base` / `small`) | `Represent this sentence for searching relevant passages: {query}` |

The Qwen3 rows omit `{query}`, so the value is wrapped as
`Instruct: …\nQuery: <text>`. The bge-v1.5 row includes `{query}`, so it becomes
a plain prefix. Tailor the Qwen3 task string to your corpus if you like — write
it in English (the models were trained that way); it typically moves recall 1–5%.

### Symmetric (no instruction) — leave empty

These are trained without a query instruction; adding one *hurts*. Leave
`ENGRAM_EMBED_QUERY_INSTRUCTION` unset.

| Model | Note |
| --- | --- |
| BAAI bge-m3 | dense retrieval needs no instruction (its lexical/sparse output is unused over `/v1/embeddings`) |
| OpenAI text-embedding-3-small / -large, text-embedding-ada-002 | symmetric |
| mistral-embed | symmetric |
| Amazon Titan Text Embeddings v2 | symmetric |

### Both-side prefix models — both sides required

These require a prefix on **both** the query and the document (and a query-only
prefix is *worse* than none). Set `ENGRAM_EMBED_QUERY_INSTRUCTION` (hot — takes
effect immediately) and `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` (changes stored
vectors — set it before indexing, or run `engram reindex` afterward; see
[Reindex](/guides/reindex/)).

| Model | `ENGRAM_EMBED_QUERY_INSTRUCTION` (hot) | `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` (needs reindex) |
| --- | --- | --- |
| intfloat/e5-\* (e5-base/large, multilingual-e5-large) | `query: {query}` | `passage: {document}` |
| nomic-embed-text-v1 / v1.5 | `search_query: {query}` | `search_document: {document}` |

### Cloud models — asymmetry is an API parameter, not a text prefix

Google, Cohere, Voyage, and Jina take the query/document asymmetry as a
**request field** (`task_type` / `input_type` / `task`), not a text prefix.
`ENGRAM_EMBED_QUERY_PARAMS` and `ENGRAM_EMBED_DOCUMENT_PARAMS` set this natively:
each is a JSON object merged into the `/v1/embeddings` request body (query params
into `EmbedQuery` calls, document params into `Embed` calls); the reserved keys
`model` and `input` are rejected since engram sets those authoritatively.
Alternatively, you can still inject the field at your OpenAI-compatible gateway
(e.g. a LiteLLM per-call mapping) if you'd rather not set it in engram.

| Provider / model | `ENGRAM_EMBED_QUERY_PARAMS` / `ENGRAM_EMBED_DOCUMENT_PARAMS` |
| --- | --- |
| Cohere embed v3 | `{"input_type":"search_query"}` / `{"input_type":"search_document"}` (**required**) |
| Voyage (voyage-3, voyage-3-lite, voyage-large-2-instruct) | `{"input_type":"query"}` / `{"input_type":"document"}` (optional) |
| OpenRouter | forwards whichever field name/value the backend model expects — see its row above |
| Jina embeddings v3 | `{"task":"retrieval.query"}` / `{"task":"retrieval.passage"}` |
| Google Gemini / Vertex | `{"task_type":"RETRIEVAL_QUERY"}` / `{"task_type":"RETRIEVAL_DOCUMENT"}` |

The gateway forwards these fields to the provider (OpenRouter accepts
`input_type` natively; LiteLLM maps provider params per model). The **document**
side changes stored vectors, so set it before indexing or run a reindex.

## Tags are part of the document vector

From `v0.9`, a record's `tags` are folded into the text engram embeds
(`content` + the tag list), so curated keywords contribute to recall rather than
only acting as an AND pre-filter. This changes the **document** vector, so —
unlike the query instruction — it applies to existing records only after you
re-embed the corpus:

```sh
engram reindex --target <new-collection>
```

Then repoint `ENGRAM_QDRANT_COLLECTION` at the new collection. See
[Reindex](/guides/reindex/) for the full cutover flow.

## Seeing the ranking

`search_memory` results now carry the Qdrant similarity `score` (higher is
closer; omitted on unranked `list_memory` results), so you can see how close a
near-miss ranked when tuning the instruction.
