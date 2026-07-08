# Technology Stack

**Analysis Date:** 2026-07-08

## Languages

**Primary:**
- Go 1.26.3 - Server, CLI, MCP tool handlers, store/embed/auth/config (all of `cmd/` and `internal/`). Declared in `go.mod`.

**Secondary:**
- TypeScript 6.0.3 + Svelte 5 - Web-auth SPA in `ui/` (SvelteKit static adapter, vendored into `internal/webauth/static`).
- Python - Skill-plugin hook tests only (`skill/engram/hooks/tests`, run via `uv run --with pytest pytest`).
- Protobuf - `EngramService` v1 read API in `proto/engram/v1/`, codegen via buf.

## Runtime

**Environment:**
- Go 1.26 (build image `golang:1.26`, runtime image `gcr.io/distroless/static-debian12:nonroot`). Static binary, `CGO_ENABLED=0`.
- Node/pnpm for building the UI and docs site (not present in the shipped server image).

**Package Manager:**
- Go modules - Lockfile: `go.sum` (present).
- pnpm 11.10.0 - UI (`ui/`) and docs (`docs-site/`); `pnpm-lock.yaml` present, `--frozen-lockfile` in CI.
- uv - Python hook test deps (ad hoc via `uv run --with pytest`).

## Frameworks

**Core:**
- `github.com/modelcontextprotocol/go-sdk` v1.6.1 - MCP server + tool registration (`internal/server/`).
- `github.com/spf13/cobra` v1.10.2 / `pflag` v1.0.10 - CLI (`cmd/engram/`).
- `github.com/knadh/koanf/v2` v2.3.5 (+ `providers/env/v2`, `providers/confmap`) - Config loader (`internal/config/`), env-first, no viper.
- `connectrpc.com/connect` v1.20.0 - Connect-go read API stubs (`gen/go/`).
- `github.com/qdrant/go-client` v1.18.3 - Qdrant vector DB client (`internal/store/`).
- `github.com/coreos/go-oidc/v3` v3.19.0 + `golang.org/x/oauth2` v0.36.0 - OIDC verification / OAuth (`internal/auth/`, `internal/webauth/`).

**UI framework:**
- SvelteKit 2.69 + Svelte 5.56, `@sveltejs/adapter-static`, Vite 8, TailwindCSS 4, shadcn-svelte, bits-ui.
- `@connectrpc/connect` + `@connectrpc/connect-web` + `@bufbuild/protobuf` - Connect client to the read API (`gen/ts/`).
- `@tanstack/svelte-query`, `marked`, `dompurify` (rendering), `mode-watcher`.

**Docs framework:**
- Astro 7 + `@astrojs/starlight` (`docs-site/`), deployed via `wrangler` (Cloudflare).

**Testing:**
- Go standard `testing` + `github.com/stretchr/testify` v1.11.1.
- `github.com/testcontainers/testcontainers-go/modules/qdrant` v0.43.0 - Integration tests against ephemeral Qdrant.
- Vitest 4 + Playwright + `vitest-browser-svelte` - UI tests (`ui/`).
- pytest - Python hook tests.

**Build/Dev:**
- Task (`Taskfile.yaml`) - lint/test/build/proto/chart entrypoints; `task` = lint + test.
- `github.com/bufbuild/buf` (Go tool directive) - protobuf lint/gen; committed `gen/` tree checked for drift.
- goreleaser - binary + image release (`.goreleaser.yaml`).
- golangci-lint (`.golangci.yaml`), yamlfmt, actionlint, rumdl, dprint - lint/format.

## Key Dependencies

**Critical:**
- `github.com/qdrant/go-client` - Vector store backing all memory persistence and recall.
- `github.com/modelcontextprotocol/go-sdk` - Defines the MCP tool contract agents call.
- `github.com/coreos/go-oidc/v3` - Bearer-token identity → memory `actor`/`owner` authz.
- `github.com/knadh/koanf/v2` - Single config surface for all `ENGRAM_` vars.

**Infrastructure / Observability:**
- `go.opentelemetry.io/otel` v1.44.0 (+ SDK, OTLP gRPC exporters for trace/metric/log, `otelslog` bridge) - Telemetry (`internal/telemetry/`).
- `connectrpc.com/otelconnect` v0.9.0 + gRPC/net-http otel instrumentation.
- `google.golang.org/grpc` v1.82.0, `google.golang.org/protobuf` v1.36.11.
- `github.com/google/uuid` v1.6.0 - Point IDs.

## Configuration

**Environment:**
- Env-first via `ENGRAM_` prefix, `--flag` overrides, koanf-loaded (`internal/config/registry.go` is the single source of truth). No viper, no config file.
- Retired `MEM_*` vars are mapped to `ENGRAM_*` and rejected with guidance (`CheckLegacy`).
- Key vars: `ENGRAM_QDRANT_ADDR` (default `localhost:6334`), `ENGRAM_QDRANT_COLLECTION` (default `mem_eval`), `ENGRAM_EMBED_MODEL` (default `ollama/bge-m3`), `ENGRAM_EMBED_DIM` (default `1024`), `ENGRAM_OPENAI_BASE_URL` (default `http://localhost:4000`), `ENGRAM_OPENAI_API_KEY`, `ENGRAM_OIDC_ISSUER`, `ENGRAM_OWNER_CLAIM` (default `email`), `ENGRAM_SUMMARY_MODEL`, `ENGRAM_LISTEN_ADDR` (default `:8080`), `ENGRAM_LOG_LEVEL`/`ENGRAM_LOG_FORMAT`. OTLP export via native `OTEL_EXPORTER_OTLP_ENDPOINT`.

**Build:**
- `Dockerfile` (local/standalone multi-stage), `Dockerfile.goreleaser` (CI path), `.goreleaser.yaml`, `buf.yaml` / `buf.gen.yaml`, `charts/engram/` (Helm).

## Platform Requirements

**Development:**
- Go 1.26+, Task, buf, Docker (testcontainers for Qdrant integration tests), pnpm + Node (UI/docs), uv (Python hook tests).

**Production:**
- Container image `ghcr.io/seanb4t/engram` on distroless; deployed via Helm chart `oci://ghcr.io/seanb4t/charts` (server + Qdrant). Requires reachable Qdrant (gRPC :6334) and an OpenAI-compatible embeddings/chat gateway.

---

*Stack analysis: 2026-07-08*
