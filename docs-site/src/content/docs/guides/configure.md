---
title: Configure
description: Full reference for engram's MEM_* environment variables and their defaults.
---

engram uses **env-first configuration** with no viper: every setting is an environment variable (`MEM_*`). Only a subset of settings expose a `--flag` on `engram serve` — specifically the server listen address and the OIDC settings; Qdrant, embedder, and logging variables are env-only. Where a flag exists, it takes precedence over the environment variable.

## Server

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `MEM_LISTEN_ADDR` | `--listen-addr` | `:8080` | TCP address the HTTP server binds to |

Source: `cmd/engram/serve.go` (`f.StringVar(&listenAddr, "listen-addr", server.EnvOr("MEM_LISTEN_ADDR", ":8080"), ...)`)

## Qdrant (vector store)

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `MEM_QDRANT_ADDR` | — | `localhost:6334` | Qdrant gRPC address (`host:port`) |
| `MEM_QDRANT_COLLECTION` | — | `mem_eval` | Qdrant collection name for memories (binary default; the Helm chart sets this to `memory` — see [Deploy](/guides/deploy/)) |
| `MEM_EMBED_DIM` | — | `1024` | Vector dimension; must match the embedding model |

Source: `internal/server/tools.go` (`StoreFromEnv`).

## Embedder

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `MEM_LITELLM_URL` | — | `http://localhost:4000` | OpenAI-compatible embeddings endpoint (LiteLLM or OpenAI) |
| `MEM_LITELLM_KEY` | — | _(empty)_ | API key for the embeddings endpoint |
| `MEM_EMBED_MODEL` | — | `ollama/bge-m3` | Model name forwarded to the endpoint |

Source: `internal/server/tools.go` (`buildDepsFromEnv`).

## OIDC / Auth

Setting `MEM_OIDC_ISSUER` enables bearer-token enforcement (JWKS signature + issuer + expiry validation). Without it, all requests are accepted and a loud warning is logged.

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `MEM_OIDC_ISSUER` | `--oidc-issuer` | _(empty)_ | OIDC issuer URL; setting it enables bearer-token enforcement |
| `MEM_OIDC_AUDIENCE` | `--oidc-audience` | _(empty)_ | Expected `aud` claim (optional; omit to skip audience check) |
| `MEM_OIDC_RESOURCE_METADATA` | `--oidc-resource-metadata` | _(empty)_ | `WWW-Authenticate` resource metadata URL returned in 401 responses (optional) |

Source: `cmd/engram/serve.go` (`init()` flag registration and `withAuth`).

## Logging

| Environment variable | Default | Description |
|---------------------|---------|-------------|
| `MEM_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `MEM_LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `MEM_LOG_STDOUT` | `true` | Write logs to stdout; set `false` to suppress (requires OTLP endpoint) |

Source: `internal/telemetry/config.go` (`ConfigFromEnv`).

## Observability (OpenTelemetry)

OTLP export is enabled when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (standard OpenTelemetry env var, not `MEM_*`). When it is empty, providers are no-ops.

The Helm chart exposes `observability.otlpEndpoint`, `observability.otlpHeaders`, and related values that the chart maps to the appropriate environment variables for the pod.

## Precedence

```
flag (--oidc-issuer) > environment variable (MEM_OIDC_ISSUER) > built-in default
```

No viper, no config file. The Helm chart sets `MEM_*` variables in the pod spec; use `--set` or a `valuesObject` override for cluster-specific values.
