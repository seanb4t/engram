---
title: Deploy
description: Deploy engram via Helm (recommended) or Docker.
---

## Helm (recommended)

The `charts/engram` chart deploys the engram server and a Qdrant instance together.

### Install

```sh
helm install engram oci://ghcr.io/seanb4t/charts/engram \
  --namespace agent-memory \
  --create-namespace \
  --set memory.openai.baseURL=http://litellm.litellm.svc.cluster.local:4000 \
  --set memory.oidc.issuer=https://auth.example.com
```

### Key values

The chart sets `ENGRAM_*` environment variables from these Helm values. Supply the cluster-specific ones via `--set` or a `valuesObject`:

| Helm value | `ENGRAM_*` variable set | Description |
|-----------|---------------------|-------------|
| `memory.listenAddr` | `ENGRAM_LISTEN_ADDR` | Listen address (default `:8080`) |
| `memory.mcpPath` | `ENGRAM_MCP_PATH` | MCP transport path (empty → `/mcp`; `/` restores the legacy root catch-all) |
| `memory.mcpResourceUrl` | `ENGRAM_MCP_RESOURCE_URL` | Public URL clients reach the MCP endpoint on (empty → derived per request) |
| `memory.openai.baseURL` | `ENGRAM_OPENAI_BASE_URL` | Embeddings endpoint URL (cluster must supply) |
| `memory.embed.model` | `ENGRAM_EMBED_MODEL` | Embed model name (default `ollama/bge-m3`) |
| `memory.embed.dim` | `ENGRAM_EMBED_DIM` | Vector dimension (default `1024`) |
| `memory.summarize.model` | `ENGRAM_SUMMARY_MODEL` | Auto-summary chat model, served by `memory.openai.baseURL` (empty disables auto-summary) |
| `memory.summarize.maxChars` | `ENGRAM_SUMMARY_MAX_CHARS` | Max generated-summary length (default `280`) |
| `memory.qdrant.collection` | `ENGRAM_QDRANT_COLLECTION` | Qdrant collection name (default `memory`) |
| `memory.oidc.issuer` | `ENGRAM_OIDC_ISSUER` | OIDC issuer URL; setting it enables bearer-token enforcement |
| `memory.oidc.audience` | `ENGRAM_OIDC_AUDIENCE` | Expected OIDC audience (optional) |
| `memory.oidc.resourceMetadata` | `ENGRAM_OIDC_RESOURCE_METADATA` | WWW-Authenticate resource metadata URL (optional) |
| `memory.decisions.provider` | `ENGRAM_DECISIONS_PROVIDER` | Typed-decision provider; empty (default) disables it, `jev` enables it |
| `memory.decisions.baseURL` | `ENGRAM_DECISIONS_BASE_URL` | Decisions API base URL; required when `provider` is set, never falls back to `memory.openai.baseURL` |
| `memory.decisions.model` | `ENGRAM_DECISIONS_MODEL` | Decision model (default `typesafe/jev-1.13`, pinned) |
| `memory.decisions.timeout` | `ENGRAM_DECISIONS_TIMEOUT` | Per-request decision call timeout (empty → binary default `10s`) |
| `memory.decisions.concurrency` | `ENGRAM_DECISIONS_CONCURRENCY` | Cap on concurrent decision calls per batch (empty → binary default `4`) |
| `memory.search.ranker` | `ENGRAM_SEARCH_RANKER` | Search ranker; `lexical` (default; renders no variable) or `jev` (opt-in reranking of `search_memory` and `search_discovery` by the decision provider, adding a per-hit `relevance`; requires `memory.decisions.provider`) |
| `memory.search.rerankTimeout` | `ENGRAM_SEARCH_RERANK_TIMEOUT` | Per-search rerank call timeout, one attempt and no retry (empty → binary default `2s`) |
| `memory.search.rerankAudit` | `ENGRAM_SEARCH_RERANK_AUDIT` | `"true"` logs every reranked search's query text and candidate ids (never content) for offline grading; empty renders no variable (off). Only rendered with `ranker: jev` |

`ENGRAM_QDRANT_ADDR` is set automatically by the chart to the in-cluster Qdrant service address and does not need a Helm value.

The `ENGRAM_OPENAI_API_KEY` value comes from a Kubernetes Secret (`memory.openai.apiKeySecret`).

`ENGRAM_DECISIONS_API_KEY` comes from `memory.decisions.apiKeySecret`; left
empty, the server inherits the `ENGRAM_OPENAI_API_KEY` secret. Every
`memory.decisions.*` variable renders only when `memory.decisions.provider`
is set.

Both `memory.search.*` variables render only when `memory.search.ranker` is
set to a value other than `lexical`, so a default install is unchanged.
`jev` needs `memory.decisions.provider` with its base URL and key, and
without it the server refuses to start and names both variables. While
`jev` is on, every search sends the query and candidate record text to the
decisions provider, and a failed or slow call falls back to lexical order
without failing the search — read [Search reranking (Jev)](/guides/configure/#search-reranking-jev)
before enabling it.

For the full environment variable reference, see [Configure](/guides/configure/).

### Upgrade

```sh
helm upgrade engram oci://ghcr.io/seanb4t/charts/engram \
  --namespace agent-memory \
  --reuse-values
```

## Docker

For local or non-Kubernetes deployments, use the image from GHCR:

```sh
docker run -d \
  --name engram \
  -p 8080:8080 \
  -e ENGRAM_QDRANT_ADDR=host.docker.internal:6334 \
  -e ENGRAM_OPENAI_BASE_URL=http://host.docker.internal:4000 \
  -e ENGRAM_EMBED_MODEL=ollama/bge-m3 \
  -e ENGRAM_OIDC_ISSUER=https://auth.example.com \
  ghcr.io/seanb4t/engram:latest
```

**No volumes needed** — all state lives in Qdrant. Run a separate Qdrant container (see [Quickstart](/guides/quickstart/)) and point `ENGRAM_QDRANT_ADDR` at it.

The MCP endpoint is served at `http://host:8080/mcp` by default. Set `ENGRAM_MCP_PATH=/` to restore the pre-0.7 behavior where the transport answered at the bare root.
