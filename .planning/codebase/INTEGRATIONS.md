# External Integrations

**Analysis Date:** 2026-07-08

## APIs & External Services

**Embeddings (OpenAI-compatible):**
- Any `/v1/embeddings` endpoint - Ollama, vLLM, or a LiteLLM gateway (`internal/embed/embed.go`).
  - Client: custom `net/http` client, Bearer auth.
  - Config: `ENGRAM_OPENAI_BASE_URL` (default `http://localhost:4000`), `ENGRAM_OPENAI_API_KEY`, `ENGRAM_EMBED_MODEL` (default `ollama/bge-m3`), `ENGRAM_EMBED_DIM` (default `1024`).

**Summarization (OpenAI-compatible chat):**
- `/v1/chat/completions` on the same gateway (`internal/summarize/`) - compresses a memory into a one-line recall summary.
  - Config: `ENGRAM_SUMMARY_MODEL` (empty disables), `ENGRAM_SUMMARY_MAX_CHARS` (280), `ENGRAM_SUMMARY_MAX_TOKENS` (1024), `ENGRAM_SUMMARY_TIMEOUT` (30s). Reuses `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`.

## Data Storage

**Vector Database:**
- Qdrant (`internal/store/`)
  - Client: `github.com/qdrant/go-client` over gRPC.
  - Connection: `ENGRAM_QDRANT_ADDR` (default `localhost:6334`), collection `ENGRAM_QDRANT_COLLECTION` (default `mem_eval`).
  - Chart ships a bundled Qdrant (`qdrant/qdrant:v1.18.2`) with a PVC.

**File Storage:**
- Qdrant snapshots optionally written to S3-compatible object storage (chart `values.yaml`: `pathPrefix: memory/qdrant-snapshots`). No app-level file storage otherwise.

**Caching:**
- None.

## Authentication & Identity

**Auth Provider:**
- OIDC (any issuer) via `github.com/coreos/go-oidc/v3` + go-sdk auth middleware (`internal/auth/auth.go`).
  - Bearer-token enforcement: JWKS signature + issuer + expiry, optional audience.
  - Config: `ENGRAM_OIDC_ISSUER` (enables enforcement; empty = disabled, logged loudly), `ENGRAM_OIDC_AUDIENCE`.
  - Verified identity → memory `actor`; `ENGRAM_OWNER_CLAIM` (default `email`) → `owner` authz key.

**Web UI OAuth:**
- Browser SPA auth-code flow (`internal/webauth/`, `ui/`) via `golang.org/x/oauth2` + go-oidc.
  - Config: `ENGRAM_UI_ENABLED`, `ENGRAM_UI_ISSUER`, `ENGRAM_UI_CLIENT_ID`/secret (OIDC keys), `ENGRAM_UI_REDIRECT_URL`, `ENGRAM_UI_COOKIE_KEY` (session cookie signing).
  - Resource metadata: `ENGRAM_OIDC_RESOURCE_METADATA` (RFC 9728 protected-resource metadata for MCP clients).

## Monitoring & Observability

**Telemetry:**
- OpenTelemetry (`internal/telemetry/`) - traces, metrics, and logs exported via OTLP/gRPC.
  - Config: native `OTEL_EXPORTER_OTLP_ENDPOINT` (empty disables all export); service name/version from build.
  - Connect/gRPC/net-http auto-instrumentation; slog bridged to OTel via `otelslog`.

**Logs:**
- Structured slog (`ENGRAM_LOG_LEVEL` default `info`, `ENGRAM_LOG_FORMAT` default `json`, `ENGRAM_LOG_STDOUT` default `true`).

## CI/CD & Deployment

**Hosting:**
- Container image `ghcr.io/seanb4t/engram` (GHCR); OCI Helm chart `oci://ghcr.io/seanb4t/charts` (`charts/engram/`, server + Qdrant).
- Docs site deployed to Cloudflare (`wrangler`, `docs-site/`).

**CI Pipeline:**
- GitHub Actions (`.github/`) - lint, test, buf drift check, license-eye, semantic-PR-title validation.
- Releases: release-please → tag/GitHub Release; goreleaser ships binary + image; `task chart:push` ships the OCI chart.

## Environment Configuration

**Required env vars (production):**
- `ENGRAM_QDRANT_ADDR`, `ENGRAM_QDRANT_COLLECTION`, `ENGRAM_OPENAI_BASE_URL` (+ `ENGRAM_OPENAI_API_KEY`), `ENGRAM_EMBED_MODEL`, `ENGRAM_EMBED_DIM`. `ENGRAM_OIDC_ISSUER` required to enforce auth.

**Secrets location:**
- Supplied via environment (Helm values / K8s secrets). No secrets committed. `ENGRAM_OPENAI_API_KEY`, `ENGRAM_OIDC_CLIENT_SECRET`, `ENGRAM_UI_CLIENT_SECRET`, `ENGRAM_UI_COOKIE_KEY` are the sensitive vars.

## Webhooks & Callbacks

**Incoming:**
- MCP tool endpoint (`ENGRAM_MCP_PATH` on `ENGRAM_LISTEN_ADDR`, default `:8080`).
- OIDC OAuth redirect callback for the web UI (`ENGRAM_UI_REDIRECT_URL`).
- Connect read API (`EngramService` v1) served from `gen/go/` stubs.

**Outgoing:**
- Embeddings + chat-completion calls to the OpenAI-compatible gateway; JWKS/OIDC discovery fetches to the issuer; OTLP export to the collector. No other outbound webhooks.

---

*Integration audit: 2026-07-08*
