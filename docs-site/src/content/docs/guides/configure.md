---
title: Configure
description: Full reference for engram's ENGRAM_* environment variables and their defaults.
---

engram uses **env-first configuration** with no viper: every setting is an environment variable (`ENGRAM_*`). A subset of settings also expose a `--flag` on `engram serve` — the server settings (listen address, MCP path), the OIDC/auth settings, and the web-UI lane settings (see the per-section Flag columns below); Qdrant, embedder, and logging variables are env-only. Where a flag exists, it takes precedence over the environment variable.

## Server

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_LISTEN_ADDR` | `--listen-addr` | `:8080` | TCP address the HTTP server binds to |
| `ENGRAM_MCP_PATH` | `--mcp-path` | `/mcp` | Path the MCP transport mounts at. `/` restores the legacy root catch-all (where the transport answered at every path). When the web UI is enabled and this is `/mcp`, the host root serves the console. |

Source: `cmd/engram/serve.go` (flag registration via `internal/config`)

## Qdrant (vector store)

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_QDRANT_ADDR` | — | `localhost:6334` | Qdrant gRPC address (`host:port`) |
| `ENGRAM_QDRANT_COLLECTION` | — | `mem_eval` | Qdrant collection name for memories (binary default; the Helm chart sets this to `memory` — see [Deploy](/guides/deploy/)) |
| `ENGRAM_EMBED_DIM` | — | `1024` | Vector dimension; must match the embedding model |

Source: `internal/server/tools.go` (`StoreFromEnvNoEnsure`).

## Embedder

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_OPENAI_BASE_URL` | — | `http://localhost:4000` | OpenAI-compatible embeddings endpoint — point it at any backend that speaks the OpenAI `/v1/embeddings` API (e.g. Ollama, vLLM, TEI, LiteLLM, OpenAI) |
| `ENGRAM_OPENAI_API_KEY` | — | _(empty)_ | API key for the embeddings endpoint |
| `ENGRAM_EMBED_MODEL` | — | `ollama/bge-m3` | Model name forwarded to the endpoint |

Source: `internal/config` (registry) + `internal/server/tools.go` (EmbedderFromEnv).

## OIDC / Auth

Setting `ENGRAM_OIDC_ISSUER` enables bearer-token enforcement (JWKS signature + issuer + expiry validation). Without it, all requests are accepted and a loud warning is logged.

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_OIDC_ISSUER` | `--oidc-issuer` | _(empty)_ | OIDC issuer URL; setting it enables bearer-token enforcement |
| `ENGRAM_OIDC_AUDIENCE` | `--oidc-audience` | _(empty)_ | Expected `aud` claim (optional; omit to skip audience check) |
| `ENGRAM_OIDC_RESOURCE_METADATA` | `--oidc-resource-metadata` | _(empty)_ | `WWW-Authenticate` resource metadata URL returned in 401 responses (optional) |

Source: `cmd/engram/serve.go` (`init()` flag registration and `withAuth`).

## Logging

| Environment variable | Default | Description |
|---------------------|---------|-------------|
| `ENGRAM_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `ENGRAM_LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `ENGRAM_LOG_STDOUT` | `true` | Write logs to stdout; set `false` to suppress (requires OTLP endpoint) |

Source: `internal/telemetry/config.go` (`ConfigFromEnv`).

## Observability (OpenTelemetry)

OTLP export is enabled when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (standard OpenTelemetry env var, not `ENGRAM_*`). When it is empty, providers are no-ops.

The Helm chart exposes `observability.otlpEndpoint`, `observability.otlpHeaders`, and related values that the chart maps to the appropriate environment variables for the pod.

## Precedence

```
flag (--oidc-issuer) > environment variable (ENGRAM_OIDC_ISSUER) > built-in default
```

No viper, no config file. The Helm chart sets `ENGRAM_*` variables in the pod spec; use `--set` or a `valuesObject` override for cluster-specific values.
