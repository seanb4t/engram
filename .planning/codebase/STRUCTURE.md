# Codebase Structure

**Analysis Date:** 2026-07-08

## Directory Layout

```text
engram/
├── cmd/engram/          # cobra CLI: root, serve, version + operator commands
├── internal/            # all private application packages
│   ├── server/          # MCP tool registration + Connect read API handlers
│   ├── store/           # Qdrant-backed memory store + authz Subject
│   ├── embed/           # OpenAI-compatible embeddings client
│   ├── auth/            # OIDC bearer-token verifier (MCP lane)
│   ├── webauth/         # OIDC auth-code login + session cookie (web lane)
│   ├── config/          # koanf env-first loader + field registry
│   ├── summarize/       # auto-digest summary generation
│   ├── telemetry/       # OTel logger/traces/metrics setup
│   └── shortid/         # Crockford base32 short-id minting
├── proto/engram/v1/     # protobuf schema (EngramService v1 read API)
├── gen/                 # committed buf-generated code
│   ├── go/              # connect-go stubs
│   └── ts/              # protobuf-es types
├── ui/                  # Svelte/SvelteKit web console SPA
│   └── src/lib/         # client, queries, components, scope helpers
├── charts/engram/       # Helm chart (server + Qdrant)
├── docs-site/           # Astro Starlight documentation site
├── docs/                # ADRs (docs/adr) + superpowers specs
├── skill/engram/        # Claude plugin/skill assets
├── brand/               # brand/logo assets
├── Taskfile.yaml        # task runner (lint + test + proto + chart + license)
├── buf.yaml/buf.gen.yaml# protobuf lint + codegen config
├── go.mod / go.sum      # Go module (module github.com/seanb4t/engram)
└── CLAUDE.md            # AI routing + memory contract (AGENTS.md symlink)
```

## Directory Purposes

**`cmd/engram/`:**

- Purpose: Process entrypoint and command tree; thin — no business logic.
- Contains: one file per command plus co-located `_test.go`.
- Key files: `main.go`, `root.go`, `serve.go`, `reindex.go`, `migrate.go`, `prune.go`, `summarize.go`, `backfill.go`, `version.go`, `mcproute.go`, `uiconfig.go`, `httplog.go`.

**`internal/server/`:**

- Purpose: MCP tool handlers + Connect read-API implementation.
- Contains: tool registration, identity resolution, instrumentation.
- Key files: `tools.go` (Register + memory/discovery tools), `rules.go` (rule tools), `connectapi.go`, `connectauth.go`, `connectobs.go`, `identity.go`, `summary.go`, `instrument.go`.

**`internal/store/`:**

- Purpose: Qdrant persistence, authorization, recall gating, paging.
- Key files: `store.go`, `subject.go`, `cursor.go`, `summarize.go`.

**`internal/config/`:**

- Purpose: Single source of truth for `ENGRAM_` configuration.
- Key files: `registry.go` (field table), `config.go` (structs + Load), `validate.go`, `legacy.go`, `embedparams.go`.

**`internal/webauth/`:**

- Purpose: Web-console login lane and session sealing.
- Key files: `handlers.go`, `oidc.go`, `session.go`, `resolver.go`, `static.go`.

**`ui/`:**

- Purpose: Read-only web console consuming the Connect API.
- Key files: `src/lib/client.ts`, `src/lib/queries.ts`, `src/routes/+page.svelte`, `src/lib/components/*.svelte`, `src/lib/gen/engram_pb.ts`.

## Key File Locations

**Entry Points:**

- `cmd/engram/main.go`: process `main` → `Execute()`.
- `cmd/engram/serve.go`: `runServe` server bootstrap.

**Configuration:**

- `internal/config/registry.go`: every env var / default / flag.
- `Taskfile.yaml`: build, lint, test, proto, chart, license tasks.
- `buf.yaml`, `buf.gen.yaml`: protobuf lint + codegen.

**Core Logic:**

- `internal/server/tools.go`: MCP tool contract.
- `internal/store/store.go`: persistence + authz.

**Testing:**

- Co-located `*_test.go` beside each Go source file.
- `ui/src/**/*.test.ts` and `*.browser.test.ts` for the SPA.

## Naming Conventions

**Files:**

- Go: `snake_case.go` per feature; tests as `<name>_test.go` co-located.
- TS/Svelte: `camelCase.ts` for libs, `PascalCase.svelte` for components, SvelteKit `+page`/`+layout` route files.

**Directories:**

- Go packages are single-word lowercase (`store`, `embed`, `webauth`).
- Generated protobuf under `gen/go/engram/v1` and `gen/ts/engram/v1` mirroring `proto/engram/v1`.

**Config keys:**

- Env vars: `ENGRAM_<AREA>_<NAME>`; koanf keys: `area.name` (`registry.go`).

## Where to Add New Code

**New MCP tool:**

- Register in `internal/server/tools.go` (or `rules.go` for rule-kind tools) via `mcp.AddTool`.
- Business logic → a method on `deps` or a `internal/store` function.
- Tests: `internal/server/tools_test.go`.

**New store capability:**

- Implementation: `internal/store/store.go` (respect the `Subject` authz switch).
- Tests: `internal/store/store_test.go`.

**New config value:**

- Add a `field` entry to `internal/config/registry.go`; add a flag in `cmd/engram/serve.go` init if flag-overridable.

**New operator command:**

- Add `cmd/engram/<verb>.go`, register on the root command in `root.go`.

**New Connect API method:**

- Edit `proto/engram/v1/engram.proto`, run `task proto:gen`, implement in `internal/server/connectapi.go`.

**New UI view:**

- Route: `ui/src/routes/`; component: `ui/src/lib/components/`; data: `ui/src/lib/queries.ts`.

## Special Directories

**`gen/`:**

- Purpose: buf-generated connect-go + protobuf-es code.
- Generated: Yes (`task proto:gen`). Committed: Yes. CI-checked for drift.

**`charts/engram/`:**

- Purpose: Helm chart shipping server + Qdrant.
- Generated: No. Committed: Yes. `Chart.yaml` version synced by release-please.

**`docs-site/`:**

- Purpose: Astro Starlight docs. Committed: Yes.

**`skill/engram/`:**

- Purpose: Claude plugin assets; `plugin.json` version synced by release-please.

---

*Structure analysis: 2026-07-08*
