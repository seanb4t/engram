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
| `memory.openai.baseURL` | `ENGRAM_OPENAI_BASE_URL` | Embeddings endpoint URL (cluster must supply) |
| `memory.embed.model` | `ENGRAM_EMBED_MODEL` | Embed model name (default `ollama/bge-m3`) |
| `memory.embed.dim` | `ENGRAM_EMBED_DIM` | Vector dimension (default `1024`) |
| `memory.summarize.model` | `ENGRAM_SUMMARY_MODEL` | Auto-summary chat model, served by `memory.openai.baseURL` (empty disables auto-summary) |
| `memory.summarize.maxChars` | `ENGRAM_SUMMARY_MAX_CHARS` | Max generated-summary length (default `280`) |
| `memory.qdrant.collection` | `ENGRAM_QDRANT_COLLECTION` | Qdrant collection name (default `memory`) |
| `memory.oidc.issuer` | `ENGRAM_OIDC_ISSUER` | OIDC issuer URL; setting it enables bearer-token enforcement |
| `memory.oidc.audience` | `ENGRAM_OIDC_AUDIENCE` | Expected OIDC audience (optional) |
| `memory.oidc.resourceMetadata` | `ENGRAM_OIDC_RESOURCE_METADATA` | WWW-Authenticate resource metadata URL (optional) |

`ENGRAM_QDRANT_ADDR` is set automatically by the chart to the in-cluster Qdrant service address and does not need a Helm value.

The `ENGRAM_OPENAI_API_KEY` value comes from a Kubernetes Secret (`memory.openai.apiKeySecret`).

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
