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
| `ENGRAM_OPENAI_CHAT_BASE_URL` | — | _(empty)_ | Base URL for the chat/summarize lane only — the embedder always uses `ENGRAM_OPENAI_BASE_URL` regardless of this setting. Empty means the summarizer **inherits** `ENGRAM_OPENAI_BASE_URL`. Validated only when set: a malformed or non-HTTP(S) value fails startup; empty is always valid. See [Auto-summary](#auto-summary) below for the URL-shape rule and the per-lane credential behavior. |
| `ENGRAM_OPENAI_CHAT_API_KEY` | — | _(empty)_ | API key for the chat/summarize lane only — the embedder always uses `ENGRAM_OPENAI_API_KEY` regardless of this setting. Empty means the summarizer **inherits** `ENGRAM_OPENAI_API_KEY`. Never validated at startup: unlike a base URL, an API key has no verifiable shape and empty is meaningful (inherit), so a wrong key fails at the provider rather than at boot. See [Auto-summary](#auto-summary) below for the per-lane credential behavior. |

Source: `internal/config` (registry) + `internal/server/tools.go` (`embedderFromConfig`).

## Memory

| Environment variable | Default | Description |
|----------------------|---------|-------------|
| `ENGRAM_MEMORY_MAX_SUMMARY_BYTES` | `512` | Max byte length of a memory `summary` on `store_memory`/`schedule_memory`/`supersede_memory`/`update_memory`. A caller-supplied summary over this bound is rejected (`field=summary hint=too_long`) rather than silently truncated. `0` disables the bound. |

This is separate from `ENGRAM_SUMMARY_MAX_CHARS` below: this bound is enforced at write
time against a **caller-authored** summary; `ENGRAM_SUMMARY_MAX_CHARS` caps the length of a
**server-generated** one.

Source: `internal/config` (registry) + `internal/server/tools.go` (`maxMemorySummaryBytes`, `validateStoreArgs`/`validateUpdateArgs`).

## Auto-summary

When `ENGRAM_SUMMARY_MODEL` is set, the server digests memories that lack a
summary using that chat model. **By default** it is served by the same
OpenAI-compatible endpoint as the embedder (`ENGRAM_OPENAI_BASE_URL` +
`ENGRAM_OPENAI_API_KEY`) — but the chat/summarize lane can be pointed at a
**different** gateway by setting `ENGRAM_OPENAI_CHAT_BASE_URL`, independent of
the embedder. This unblocks a common split deployment: a local embedder (no
egress, no per-token cost) paired with a hosted chat model for summary
quality — e.g. embeddings at `http://localhost:4000` (a local TEI/Ollama/vLLM
server) with summaries at `https://api.openai.com/v1`. Empty
`ENGRAM_SUMMARY_MODEL` disables auto-summary entirely, and recall returns only
client-authored summaries.

**Each lane can carry its own API key.** `ENGRAM_OPENAI_API_KEY` is always the
embedder's credential. The chat/summarize lane can carry its own via
`ENGRAM_OPENAI_CHAT_API_KEY`; leaving it empty means the chat lane
**inherits** the embedder's key and sends it to whatever
`ENGRAM_OPENAI_CHAT_BASE_URL` resolves to. That inherit-by-default behavior is
safe for the local-embedder-plus-hosted-chat split above, because local
embedding servers (Ollama, TEI, vLLM) simply ignore an `Authorization` header
they don't expect. It is worth knowing about *before* you point the chat lane
at a hosted gateway, though: leaving `ENGRAM_OPENAI_CHAT_API_KEY` unset while
setting `ENGRAM_OPENAI_CHAT_BASE_URL` sends your embedding API key to that
gateway too. Set `ENGRAM_OPENAI_CHAT_API_KEY` to opt out and give the chat
lane a credential of its own.

In a Helm deployment, set `memory.summarize.chatApiKeySecret` to render
`ENGRAM_OPENAI_CHAT_API_KEY` into the pod spec as a `secretKeyRef` (unset
omits it, matching the inherit-by-default behavior above).

**URL shape matters.** Supply the provider's full OpenAI-compatible root,
including its `/v1` suffix (or `/v1beta/openai` for Gemini-compatible
gateways), and engram appends the chat-completions path directly — e.g.
`ENGRAM_OPENAI_CHAT_BASE_URL=https://api.openai.com/v1` resolves to
`https://api.openai.com/v1/chat/completions`. Supply a bare host with no
`/v1`-shaped suffix and engram appends `/v1/chat/completions` itself — e.g.
`ENGRAM_OPENAI_CHAT_BASE_URL=http://litellm.internal:4000` resolves to
`http://litellm.internal:4000/v1/chat/completions`. Getting this wrong
(appending your own `/v1/chat/completions` onto a URL that already ends in
`/v1`) is the most likely first failure when configuring this variable.

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_SUMMARY_MODEL` | — | _(empty)_ | Chat model for auto-summary; empty disables auto-summary |
| `ENGRAM_SUMMARY_MAX_CHARS` | — | `280` | Max generated-summary length (also the recall-truncation cap) |

(`ENGRAM_OPENAI_CHAT_BASE_URL` and `ENGRAM_OPENAI_CHAT_API_KEY` are documented
in [Embedder](#embedder) above, alongside `ENGRAM_OPENAI_BASE_URL` and
`ENGRAM_OPENAI_API_KEY` — this section explains the per-lane credential
behavior and the effect the base URL has on the chat/summarize lane
specifically.)

In a Helm deployment, set `memory.summarize.chatBaseURL` to render this
variable into the pod spec (unset omits it, matching the inherit-by-default
behavior above).

Source: `internal/config` (registry) + `internal/server/tools.go` (`summarizerFromConfig`) + `internal/openaiurl` (the shape-aware endpoint join).

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

Source: `cmd/engram/serve.go` (`init()` flag registration and `buildAuthChain`, which composes the chain and wraps it in `auth.EnforceExpiry` so token expiry is enforced on the composed chain rather than only inside the MCP bearer wrapper).

### Service principals (machine-to-machine)

A headless service principal (CI runner, batch job, another backend service) authenticates over a third lane composed alongside the human OIDC lane above — each mechanism activates independently, based on its own config being present. Env-only (secret-bearing, no `--flag` equivalents):

| Environment variable | Default | Description |
|----------------------|---------|-------------|
| `ENGRAM_SERVICE_AUTH_OIDC_ISSUER` | _(empty — lane off)_ | Client-credentials OIDC issuer URL (may reuse `ENGRAM_OIDC_ISSUER`'s IdP or be distinct) |
| `ENGRAM_SERVICE_AUTH_OIDC_AUDIENCE` | _(empty)_ | Expected `aud` claim for the service lane, independent of `ENGRAM_OIDC_AUDIENCE` |
| `ENGRAM_SERVICE_AUTH_OWNER_CLAIMS` | `client_id,azp` | Ordered claim list the service lane resolves an owner from — never defaults to `email` |
| `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` | _(empty — lane off)_ | Comma-separated `owner=token` pairs, e.g. `ci=tok-abc,batch=tok-def` |

A deployment with none of these set is unchanged: only the human OIDC lane (or no auth at all) is active. Static tokens have no revocation list — rotating means editing `ENGRAM_SERVICE_AUTH_STATIC_TOKENS` and restarting; see `reference/auth.md` (Service principals) for the fail-closed empty-owner guarantee, the no-revocation kill-switch, and the [global cross-tenant `shared`-read decision](/reference/auth/#cross-tenant-shared-reads) (`docs/adr/engram-svct-service-tenant-global-shared-read.md`).

Source: `internal/config` (`ServiceAuthConfig`, `service_auth.*` registry rows) + `cmd/engram/serve.go` (`buildAuthChain`).

### Headless Connect lane

| Environment variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `ENGRAM_CONNECT_HEADLESS` | `--connect-headless` | `false` | Mounts the ConnectRPC lane on a deployment with the web UI disabled |

`ENGRAM_CONNECT_HEADLESS` defaults **off** and is independent of every `ENGRAM_UI_*` and
`ENGRAM_SERVICE_AUTH_*` variable — a deployment with no Connect surface today gains none on
upgrade, including one that already has service-auth configured; configuring an auth lane never
mounts Connect by itself.

Connect is mounted when the web UI is enabled **or** this flag is set. With either (or both), one
lane serves both credential types: a well-formed `Authorization: Bearer` credential authenticates
against the same verifier chain the MCP lane uses, and everything else falls through to the
session-cookie lane.

Setting `ENGRAM_CONNECT_HEADLESS` with no auth lane configured (no `ENGRAM_OIDC_ISSUER` and no
`ENGRAM_SERVICE_AUTH_*`) **refuses to start** — mounting would expose every write RPC
unauthenticated into the anonymous empty-owner bucket. Configure at least one auth lane first:
either `ENGRAM_OIDC_ISSUER` (the human OIDC lane) or `ENGRAM_SERVICE_AUTH_*` (client-credentials
OIDC or static tokens, see [Service principals](#service-principals-machine-to-machine) above).

A bearer-authenticated Connect caller is exempt from the `X-CSRF-Token` double-submit check that
cookie-authenticated browser callers must satisfy; the exemption is decided by which lane verified
the request, never by which headers the caller sent.

Source: `internal/config` (the `connect.headless` registry row) + `cmd/engram/serve.go`
(`connectHeadlessGuard`, `connectResolverFor`) + `internal/server/connectbearer.go`.

## Logging

| Environment variable | Default | Description |
|---------------------|---------|-------------|
| `ENGRAM_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `ENGRAM_LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `ENGRAM_LOG_STDOUT` | `true` | Write logs to stdout; set `false` to suppress (requires OTLP endpoint) |

Source: `internal/telemetry/config.go` (`ConfigFromEnv`).

**Authorization decision diagnostics.** At `debug` level, every authorization decision
(both allow and deny) emits one `"authz decision (bucket)"` or `"authz decision
(record)"` line carrying `allow`, `action`, the satisfied Cedar `policy_ids`, and
`policy_error_count`; bucket-scoped decisions also carry `bucket` (`own`/`shared`).
Volume is bounded: at most two lines per bulk recall call (one per bucket probed) and
one per id-addressed operation (get/update/delete/set_visibility) — never one line per
result row.

It does **not** carry a full Cedar expression trace, any policy error **message** text,
or the caller's `owner`/`scope` values — those are deliberately excluded so a decision
line is always safe to leave in a log pipeline. Raise `ENGRAM_LOG_LEVEL=debug` to see
these lines; they do not appear at `info` or above.

## Observability (OpenTelemetry)

OTLP export is enabled when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (standard OpenTelemetry env var, not `ENGRAM_*`). When it is empty, providers are no-ops.

The Helm chart exposes `observability.otlpEndpoint`, `observability.otlpHeaders`, and related values that the chart maps to the appropriate environment variables for the pod.

## Precedence

```
flag (--oidc-issuer) > environment variable (ENGRAM_OIDC_ISSUER) > built-in default
```

No viper, no config file. The Helm chart sets `ENGRAM_*` variables in the pod spec; use `--set` or a `valuesObject` override for cluster-specific values.
