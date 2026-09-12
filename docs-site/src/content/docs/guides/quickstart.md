---
title: Quickstart
description: Get engram running in minutes — Qdrant, embedder, Docker, and your first memory.
---

Get the MCP server running locally in a few minutes.

**Already have a server endpoint?** Go to [Install](/guides/install/) for the
binary, then [Agent Setup](/guides/agent-setup/) to connect your agent. Check the
setup availability notice there: v0.15.1 lacks `setup`, so that route currently
requires a source build. For standalone Claude Code registration, see the
[plugin guide](/guides/plugin/). Continue below if you need to provision a server.

## Prerequisites

- **Qdrant** — a running Qdrant instance (gRPC port `6334`). The quickest path is Docker:

  ```sh
  docker network create engram-local
  docker run -d --name engram-qdrant --network engram-local qdrant/qdrant
  ```

  This creates a network shared by the two containers. Qdrant is reachable
  by engram without publishing its port on the host.

- **Embeddings endpoint** — an OpenAI-compatible embeddings endpoint. Options:
  - [LiteLLM](https://docs.litellm.ai/) in front of any model
  - OpenAI API directly (set `ENGRAM_OPENAI_BASE_URL=https://api.openai.com/v1` and `ENGRAM_EMBED_MODEL=text-embedding-3-small`)
- **OIDC issuer** (optional) — if you want bearer-token enforcement, an OIDC issuer URL. Without one, the server accepts all requests (logged loudly).

## Run with Docker

Pull and run the latest image from GHCR:

```sh
docker run -d \
  --name engram \
  --network engram-local \
  -p 127.0.0.1:8080:8080 \
  -e ENGRAM_QDRANT_ADDR=engram-qdrant:6334 \
  -e ENGRAM_OPENAI_BASE_URL=http://host.docker.internal:4000 \
  -e ENGRAM_EMBED_MODEL=ollama/bge-m3 \
  ghcr.io/seanb4t/engram:latest
```

(`host.docker.internal` resolves on macOS and Windows; Linux users need `--add-host host.docker.internal:host-gateway` or replace with the host IP.)

This example binds engram to host loopback for local use. For access from other
machines, configure authentication and a protected deployment first; see
[Configure](/guides/configure/).

The MCP endpoint is served at **`http://localhost:8080/mcp`** by default. Set `ENGRAM_MCP_PATH=/` to restore the pre-0.7 behavior where the transport answered at the bare root.

Key environment variables (see [Configure](/guides/configure/) for the full list):

| Variable | What it does |
|----------|-------------|
| `ENGRAM_QDRANT_ADDR` | Qdrant gRPC address (`host:port`); default `localhost:6334` |
| `ENGRAM_OPENAI_BASE_URL` | Embeddings endpoint (OpenAI-compatible); default `http://localhost:4000` |
| `ENGRAM_EMBED_MODEL` | Model name forwarded to the endpoint; default `ollama/bge-m3` |

## Connect your agent

Once the server is running, [install the binary](/guides/install/) and follow
[Agent Setup](/guides/agent-setup/) for runtime selection, authentication, and a
preview before applying changes. Setup currently requires the unreleased source
build described in Install. The [Claude Code Plugin guide](/guides/plugin/)
also covers standalone registration when no binary is installed.

## Store and recall your first memory

With the server registered, use `store_memory` to persist a fact and `search_memory` to retrieve it. See the [Tools reference](/reference/tools/) for full parameter docs.

```text
store_memory — persist a decision, convention, preference, or gotcha
search_memory — semantic search over stored memories
```
