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

Source: `internal/server/tools.go` (`StoreAndEmbedderFromEnvNoEnsure`).

## Embedder

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_OPENAI_BASE_URL` | — | `http://localhost:4000` | OpenAI-compatible embeddings endpoint — point it at any backend that speaks the OpenAI `/v1/embeddings` API (e.g. Ollama, vLLM, TEI, LiteLLM, OpenAI) |
| `ENGRAM_OPENAI_API_KEY` | — | _(empty)_ | API key for the embeddings endpoint |
| `ENGRAM_EMBED_MODEL` | — | `ollama/bge-m3` | Model name forwarded to the endpoint |

Source: `internal/config` (registry) + `internal/server/tools.go` (`embedderFromConfig`).

## Auto-summary

When `ENGRAM_SUMMARY_MODEL` is set, the server digests memories that lack a summary using that chat model, served by the **same** OpenAI-compatible endpoint as the embedder (`ENGRAM_OPENAI_BASE_URL` + `ENGRAM_OPENAI_API_KEY`). Empty disables auto-summary, and recall returns only client-authored summaries.

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_SUMMARY_MODEL` | — | _(empty)_ | Chat model for auto-summary, served by `ENGRAM_OPENAI_BASE_URL`; empty disables auto-summary |
| `ENGRAM_SUMMARY_MAX_CHARS` | — | `280` | Max generated-summary length (also the recall-truncation cap) |

Source: `internal/config` (registry) + `internal/server/tools.go` (`summarizerFromConfig`).

### Async-on-write summaries

By default, `store_memory`/`schedule_memory` records without a client-authored `summary` stay unsummarized until the next `engram summarize-missing` sweep. Setting `ENGRAM_SUMMARY_ON_WRITE=true` enables a bounded in-process worker pool that fills the summary asynchronously right after each successful write — a memory typically gets a summary within seconds instead of waiting for the next sweep.

**Two-step opt-in.** Enabling this is a two-step, per-deployment decision, not a single flag flip:

1. Set `ENGRAM_SUMMARY_MODEL` and run `task eval:summary` (`ENGRAM_SUMMARY_EVAL=1 go test ./internal/summarize/ -run TestSummaryFidelity -v`) to judge whether your configured model preserves caveats/negations well enough for your data. This is a **manual, per-deployment gate** — it is intentionally **not** run in CI, since it needs a live gateway + model and its verdict is judgment, not a pass/fail regression test.
2. Once you're satisfied with fidelity, set `ENGRAM_SUMMARY_ON_WRITE=true` to turn the worker pool on.

Both switches must be true at once for the pool to start (`ENGRAM_SUMMARY_MODEL` non-empty AND `ENGRAM_SUMMARY_ON_WRITE` parsing true) — setting `ENGRAM_SUMMARY_MODEL` alone only enables the `summarize-missing` sweep, not the async worker.

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_SUMMARY_ON_WRITE` | — | `false` | Enables the async-on-write summary worker pool (requires `ENGRAM_SUMMARY_MODEL` also set) |
| `ENGRAM_SUMMARY_WORKERS` | — | `2` | Worker goroutine pool size draining the enqueue channel |
| `ENGRAM_SUMMARY_QUEUE_SIZE` | — | `256` | Bounded enqueue channel capacity |

**Bounded and non-blocking.** The enqueue channel is bounded (`ENGRAM_SUMMARY_QUEUE_SIZE`); a full queue drops the id and logs a warning instead of blocking the write — the next `engram summarize-missing` sweep reclaims any dropped or in-flight-at-shutdown records as a backstop. `store_memory`/`schedule_memory` always return success once the record is persisted, even when the summarizer is down, slow, or the queue is full: summarization is never on the synchronous write path.

**Degradation.** If the summary gateway is unreachable or erroring, the write still succeeds and the record simply has no summary yet ("no summary yet") until a later fill (retry, or the next `summarize-missing` sweep) succeeds. Recall never fails because of a missing summary — it falls back to truncated content.

Source: `internal/config` (registry) + `internal/server/tools.go` (`buildSummaryQueue`, the D-01 AND-gate) + `internal/server/summaryqueue.go` (worker pool).

## OIDC / Auth

Setting `ENGRAM_OIDC_ISSUER` enables bearer-token enforcement (JWKS signature + issuer + expiry validation). Without it, all requests are accepted and a loud warning is logged.

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_OIDC_ISSUER` | `--oidc-issuer` | _(empty)_ | OIDC issuer URL; setting it enables bearer-token enforcement |
| `ENGRAM_OIDC_AUDIENCE` | `--oidc-audience` | _(empty)_ | Expected `aud` claim (optional; omit to skip audience check) |
| `ENGRAM_OIDC_RESOURCE_METADATA` | `--oidc-resource-metadata` | _(empty)_ | `WWW-Authenticate` resource metadata URL returned in 401 responses (optional) |
| `ENGRAM_OWNER_CLAIM` | `--owner-claim` | `email` | OIDC claim whose value becomes the record `owner` (authz key); fail-closed if absent; requires `email_verified` when `email` |

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
