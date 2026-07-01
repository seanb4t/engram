---
title: Embedding query instructions
description: Tune recall quality with ENGRAM_EMBED_QUERY_INSTRUCTION — when to set a query-side instruction, and the exact value for Qwen3, BGE, E5, nomic, OpenAI, Google, Cohere, Voyage, and Jina embedders.
---

Many retrieval embedders are **asymmetric**: they expect the search *query* to be
wrapped with a task instruction while the stored *document* is embedded raw.
Sending both sides through the identical raw path — which engram did before
`v0.9` — leaves recall quality on the table and makes ranking sensitive to how a
query is phrased.

`ENGRAM_EMBED_QUERY_INSTRUCTION` fixes this for text-prefix embedders. It changes
only the **query** embedding, so documents are untouched and **no reindex is
needed** to adopt it.

## How the knob behaves

`ENGRAM_EMBED_QUERY_INSTRUCTION` is applied by `EmbedQuery` (used by
`search_memory` and `search_discovery`); the store/update/reindex paths always
embed documents raw. Three modes, chosen by the value:

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

### Both-side prefix models — not yet supported

These require a prefix on **both** the query and the document (and a query-only
prefix is *worse* than none). The document side needs a reindex, which this knob
does not do — leave it empty until that lands (`engram-wd89.1`).

| Model | Query / document prefix |
| --- | --- |
| intfloat/e5-\* (e5-base/large, multilingual-e5-large) | `query: ` / `passage: ` |
| nomic-embed-text-v1 / v1.5 | `search_query: ` / `search_document: ` |

### Cloud models — asymmetry is an API parameter, not a text prefix

Google, Cohere, Voyage, and Jina do not take the instruction as text — they take
a **request field** (`task_type` / `input_type` / `task`) that engram's
text-prefix knob cannot set. **Leave `ENGRAM_EMBED_QUERY_INSTRUCTION` empty** and
inject the retrieval parameter at your OpenAI-compatible gateway (e.g. LiteLLM
per-call `input_type`), which is the only place that field can be added today.
Native passthrough is tracked in `engram-wd89.1`.

| Provider / model | Request field — query vs document |
| --- | --- |
| Google Gemini / Vertex (gemini-embedding-001, text-embedding-004/005, text-multilingual-embedding-002) | `task_type=RETRIEVAL_QUERY` / `RETRIEVAL_DOCUMENT` |
| Cohere embed v3 (embed-english-v3.0, embed-multilingual-v3.0) | `input_type=search_query` / `search_document` (**required**) |
| Voyage (voyage-3, voyage-3-lite, voyage-large-2-instruct) | `input_type=query` / `document` (optional) |
| Jina embeddings v3 | `task=retrieval.query` / `retrieval.passage` |

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
