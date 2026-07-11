---
title: Embedding model recipes
description: Copy-paste ENGRAM_EMBED_* env recipes for OpenRouter, Gemini, OpenAI, and local TEI/Ollama/vLLM embedders, with the exact base URL, model id, dimension, and query instruction for each.
---

engram calls an OpenAI-compatible `/v1/embeddings` endpoint, so it can point at
any provider that speaks that shape. The recipes below are **equal operator
choices** — engram ships with a neutral local default (`ollama/bge-m3`) and
takes no position on which provider you should run in production. The
OpenRouter `qwen/qwen3-embedding-8b` and Gemini `gemini-embedding-2` configs
shown here double as this project's eval/CI reference configuration (used by
`task eval:retrieval`) — that is a testing convenience, not a recommendation.

Switching model, provider, or dimension changes the stored vector space. See
[Reindex](/guides/reindex/) before cutting over an existing deployment. For the
mechanics of query/document instruction and param tuning (once you've picked a
model), see [Embedding query instructions](/guides/embedding-instructions/).

## At a glance

| Provider | Model id | Base URL | Native dim | Query-side mechanism | Reindex on change |
| --- | --- | --- | --- | --- | --- |
| OpenRouter | `qwen/qwen3-embedding-8b` | `https://openrouter.ai/api/v1` | 4096 | `ENGRAM_EMBED_QUERY_INSTRUCTION` (Qwen3 instruction wrap; documents raw) | Yes — [Reindex](/guides/reindex/) |
| Gemini | `gemini-embedding-2` | `https://generativelanguage.googleapis.com/v1beta/openai` | 3072 | `ENGRAM_EMBED_QUERY_INSTRUCTION` + `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` (text-prefix — **not** `*_PARAMS`/`task_type`) | Yes — [Reindex](/guides/reindex/) |
| OpenAI | `text-embedding-3-large` (or `-small`) | `https://api.openai.com/v1` | 3072 (`-large`) / 1536 (`-small`) | none — symmetric | Yes — [Reindex](/guides/reindex/) |
| Local — TEI | `BAAI/bge-m3` | `http://<host>:8080/v1` | 1024 | none — symmetric | Yes — [Reindex](/guides/reindex/) |
| Local — Ollama | `bge-m3` | `http://<host>:11434/v1` | 1024 | none — symmetric | Yes — [Reindex](/guides/reindex/) |
| Local — vLLM | `BAAI/bge-m3` | `http://<host>:8000/v1` | 1024 | none — symmetric | Yes — [Reindex](/guides/reindex/) |

Any change to `ENGRAM_EMBED_MODEL`, `ENGRAM_EMBED_DIM`, or the provider itself
means the new vectors are not comparable to what's already stored — run
[`engram reindex`](/guides/reindex/) and cut over once you've verified the
target collection.

## OpenRouter — qwen/qwen3-embedding-8b

Also the eval/CI reference for `task eval:retrieval`'s recall@k run (D-08) —
using it here is a testing convenience, not a production recommendation.

```sh
export ENGRAM_OPENAI_BASE_URL='https://openrouter.ai/api/v1'
export ENGRAM_OPENAI_API_KEY='replace-with-your-key'
export ENGRAM_EMBED_MODEL='qwen/qwen3-embedding-8b'
export ENGRAM_EMBED_DIM=4096
export ENGRAM_EMBED_QUERY_INSTRUCTION='Given a web search query, retrieve relevant passages that answer the query'
```

Documents are embedded raw (no `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` needed).
Changing model or dimension away from this recipe requires
[`engram reindex`](/guides/reindex/).

## Gemini — gemini-embedding-2

Gemini's query/document asymmetry is a **text-prefix** instruction on the
OpenAI-compat `/v1/embeddings` endpoint engram calls — not the `task_type`
request field the native `embedContent` API supports. Use
`ENGRAM_EMBED_QUERY_INSTRUCTION` / `ENGRAM_EMBED_DOCUMENT_INSTRUCTION`, never
`ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS` (a `task_type` param
is a silent no-op through this endpoint).

```sh
export ENGRAM_OPENAI_BASE_URL='https://generativelanguage.googleapis.com/v1beta/openai'
export ENGRAM_OPENAI_API_KEY='replace-with-your-key'
export ENGRAM_EMBED_MODEL='gemini-embedding-2'
export ENGRAM_EMBED_DIM=3072
export ENGRAM_EMBED_QUERY_INSTRUCTION='task: search result | query: {query}'
export ENGRAM_EMBED_DOCUMENT_INSTRUCTION='title: none | text: {document}'
```

`ENGRAM_EMBED_DOCUMENT_INSTRUCTION` changes the stored document vector, so set
it before indexing or run [`engram reindex`](/guides/reindex/) afterward. This
is also the differ-case reference config for
`TestRetrievalEval_AsymmetryDiffer` — see "Running the retrieval eval" below.

## OpenAI — text-embedding-3-large

```sh
export ENGRAM_OPENAI_BASE_URL='https://api.openai.com/v1'
export ENGRAM_OPENAI_API_KEY='replace-with-your-key'
export ENGRAM_EMBED_MODEL='text-embedding-3-large'
export ENGRAM_EMBED_DIM=3072
export ENGRAM_EMBED_QUERY_INSTRUCTION=''
```

`text-embedding-3-large`/`-small` are symmetric — leave the query instruction
empty. Switching to `-small` (dim 1536) or back requires
[`engram reindex`](/guides/reindex/).

## Local — TEI (Text Embeddings Inference)

```sh
export ENGRAM_OPENAI_BASE_URL='http://<host>:8080/v1'
export ENGRAM_OPENAI_API_KEY=''
export ENGRAM_EMBED_MODEL='BAAI/bge-m3'
export ENGRAM_EMBED_DIM=1024
export ENGRAM_EMBED_QUERY_INSTRUCTION=''
```

TEI's gateway typically needs no API key — the empty quoted value above is
intentional, not a placeholder to fill in. `bge-m3` is symmetric, so the query
instruction stays empty. This is the same model/dim as the chart's shipped
default (`ollama/bge-m3`@1024) served through TEI instead of Ollama.

## Local — Ollama

```sh
export ENGRAM_OPENAI_BASE_URL='http://<host>:11434/v1'
export ENGRAM_OPENAI_API_KEY=''
export ENGRAM_EMBED_MODEL='bge-m3'
export ENGRAM_EMBED_DIM=1024
export ENGRAM_EMBED_QUERY_INSTRUCTION=''
```

This mirrors the chart's neutral default (`model: "ollama/bge-m3"`, `dim:
1024`). No API key is required for a local Ollama instance.

## Local — vLLM

```sh
export ENGRAM_OPENAI_BASE_URL='http://<host>:8000/v1'
export ENGRAM_OPENAI_API_KEY=''
export ENGRAM_EMBED_MODEL='BAAI/bge-m3'
export ENGRAM_EMBED_DIM=1024
export ENGRAM_EMBED_QUERY_INSTRUCTION=''
```

Same model/dim as the TEI recipe, served through vLLM's OpenAI-compatible
endpoint. Changing the served model requires
[`engram reindex`](/guides/reindex/).

## Running the retrieval eval (optional)

`task eval:retrieval` needs a live Qdrant (Docker, or `ENGRAM_QDRANT_TEST_ADDR`
pointed at a running instance) plus a live embedding gateway. It runs
`go test ./internal/retrievaleval/ -run TestRetrievalEval -v`, and that `-run`
regex substring-matches both the `#261` recall@k fixture and
`TestRetrievalEval_AsymmetryDiffer`, so a single invocation exercises both.

**Full eval (qwen3@4096 recall + Gemini differ), using the OpenRouter recipe
above:**

```sh
export ENGRAM_RETRIEVAL_EVAL=1
task eval:retrieval
```

**Gemini-only differ run**, using the Gemini recipe above, skipping the #261
recall case — target the differ test directly:

```sh
export ENGRAM_RETRIEVAL_EVAL=1
go test ./internal/retrievaleval/ -run TestRetrievalEval_AsymmetryDiffer -v
```

This targeted form still needs Docker (or `ENGRAM_QDRANT_TEST_ADDR`): the
package's `TestMain` starts Qdrant whenever `ENGRAM_RETRIEVAL_EVAL=1` is set,
regardless of which test the `-run` filter selects.
