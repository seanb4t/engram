---
title: Quickstart
description: Get engram running in minutes — Qdrant, embedder, Docker, and your first memory.
---

Get the MCP server running locally in a few minutes.

## Prerequisites

- **Qdrant** — a running Qdrant instance (gRPC port `6334`). The quickest path is Docker:
  ```sh
  docker run -d -p 6334:6334 qdrant/qdrant
  ```
- **Embeddings endpoint** — an OpenAI-compatible embeddings endpoint. Options:
  - [LiteLLM](https://docs.litellm.ai/) in front of any model
  - OpenAI API directly (set `MEM_LITELLM_URL=https://api.openai.com/v1` and `MEM_EMBED_MODEL=text-embedding-3-small`)
- **OIDC issuer** (optional) — if you want bearer-token enforcement, an OIDC issuer URL. Without one, the server accepts all requests (logged loudly).

## Run with Docker

Pull and run the latest image from GHCR:

```sh
docker run -d \
  --name engram \
  -p 8080:8080 \
  -e MEM_QDRANT_ADDR=host.docker.internal:6334 \
  -e MEM_LITELLM_URL=http://host.docker.internal:4000 \
  -e MEM_EMBED_MODEL=ollama/bge-m3 \
  ghcr.io/seanb4t/engram:latest
```

(`host.docker.internal` resolves on macOS and Windows; Linux users need `--add-host host.docker.internal:host-gateway` or replace with the host IP.)

The MCP endpoint is served at the **root** — `http://localhost:8080` — with no path prefix.

Key environment variables (see [Configure](/guides/configure/) for the full list):

| Variable | What it does |
|----------|-------------|
| `MEM_QDRANT_ADDR` | Qdrant gRPC address (`host:port`); default `localhost:6334` |
| `MEM_LITELLM_URL` | Embeddings endpoint (OpenAI-compatible); default `http://localhost:4000` |
| `MEM_EMBED_MODEL` | Model name forwarded to the endpoint; default `ollama/bge-m3` |

## Register with Claude Code

Once the server is running, add it to Claude Code with `/engram-setup`. See the [Claude Code Plugin guide](/guides/plugin/) for details.

## Store and recall your first memory

With the server registered, use `store_memory` to persist a fact and `search_memory` to retrieve it. See the [Tools reference](/reference/tools/) for full parameter docs.

```
store_memory — persist a decision, convention, preference, or gotcha
search_memory — semantic search over stored memories
```
