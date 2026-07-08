<!-- refreshed: 2026-07-08 -->
# Architecture

**Analysis Date:** 2026-07-08

## System Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                       Clients                                │
├──────────────────┬──────────────────┬───────────────────────┤
│  Coding agent    │  Web console SPA │   Connect API client  │
│  (MCP over HTTP) │  (Svelte, `ui/`) │  (connect-go/-es gen)  │
└────────┬─────────┴────────┬─────────┴──────────┬────────────┘
         │ bearer JWT       │ session cookie     │
         ▼                  ▼                     ▼
┌─────────────────────────────────────────────────────────────┐
│           HTTP mux (single http.Server)                      │
│           `cmd/engram/serve.go` (runServe)                   │
│  /mcp → MCP transport   /auth/* → login   /ui/ → SPA         │
│  /engram.v1.EngramService/* → Connect read API              │
└──────┬───────────────────────┬──────────────────┬───────────┘
       │                       │                   │
       ▼                       ▼                   ▼
┌──────────────┐   ┌───────────────────┐  ┌───────────────────┐
│ auth verify  │   │ MCP tool handlers │  │ Connect handlers  │
│ `internal/   │   │ `internal/server/ │  │ `internal/server/ │
│  auth`,      │   │  tools.go`,       │  │  connectapi.go`   │
│ `webauth`    │   │  rules.go`        │  │                   │
└──────┬───────┘   └─────────┬─────────┘  └─────────┬─────────┘
       │  Subject            │ deps{st,em,now}      │
       └─────────────────────┼──────────────────────┘
                             ▼
              ┌───────────────────────────────┐
              │   store (`internal/store`)    │
              │   authz + payload mapping      │
              └───────┬───────────────┬───────┘
                      │               │
                      ▼               ▼
              ┌──────────────┐  ┌──────────────┐
              │ embed client │  │   Qdrant     │
              │ `internal/   │  │ (vector DB,  │
              │  embed`      │  │  gRPC :6334) │
              └──────┬───────┘  └──────────────┘
                     ▼
        OpenAI-compatible /v1/embeddings
        (Ollama / vLLM / LiteLLM gateway)
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| CLI / entrypoint | cobra command tree; wires serve + operator commands | `cmd/engram/root.go`, `cmd/engram/main.go` |
| Serve wiring | Builds telemetry, mux, MCP server, auth lanes, HTTP server | `cmd/engram/serve.go` |
| MCP tool handlers | Registers + serves the memory/discovery/rule tools | `internal/server/tools.go`, `internal/server/rules.go` |
| Connect read API | Implements generated `EngramServiceHandler` (read-only) | `internal/server/connectapi.go` |
| Identity resolution | Maps verified token/session to `store.Subject` | `internal/server/identity.go` |
| Store | Qdrant-backed persistence, authz gate, recall gating | `internal/store/store.go`, `internal/store/subject.go` |
| Embedder | OpenAI-compatible embeddings client (asymmetric) | `internal/embed/embed.go` |
| Bearer auth | OIDC JWKS/issuer/expiry verification for MCP lane | `internal/auth/auth.go` |
| Web auth | OIDC auth-code login, sealed session cookie for SPA | `internal/webauth/*.go` |
| Config | koanf env-first loader + field registry | `internal/config/config.go`, `internal/config/registry.go` |
| Telemetry | OTel logger/traces/metrics setup + tool metrics | `internal/telemetry/*.go` |
| Short IDs | Crockford base32 10-char handle minting | `internal/shortid/shortid.go` |
| Summarize | Auto-digest generation for recall summaries | `internal/summarize/summarize.go` |

## Pattern Overview

**Overall:** Layered hexagonal-ish server. A thin cobra CLI composes a single Go
process that mounts three HTTP protocols (MCP, Connect, web login/SPA) over one
`http.ServeMux`, all funneling into a shared authorization-aware store backed by
Qdrant and an external embeddings endpoint.

**Key Characteristics:**
- Single binary, single process, single listener; protocols multiplexed by path.
- Env-first config via a single field registry (`registry.go`) — no config file, no viper.
- Identity is never client-asserted; a verified `Subject` sum type drives authz.
- Generated code (`gen/`) from protobuf is committed and drift-checked in CI.
- Observability is pervasive: every layer owns an OTel tracer.

## Layers

**CLI / Command layer:**
- Purpose: Parse flags, load config, dispatch to serve or operator commands.
- Location: `cmd/engram/`
- Contains: cobra commands (`serve`, `version`, `reindex`, `migrate`, `prune`, `summarize`, `backfill`).
- Depends on: `internal/config`, `internal/server`, `internal/telemetry`, `internal/webauth`, `internal/auth`.
- Used by: process `main` (`main.go`).

**Transport / Handler layer:**
- Purpose: Register MCP tools, serve Connect read API, serve web-auth endpoints.
- Location: `internal/server/`, `internal/webauth/`
- Contains: tool registration, request shaping, identity extraction, Connect service impl.
- Depends on: `internal/store`, `internal/embed`, `internal/summarize`, `internal/auth`.
- Used by: `cmd/engram/serve.go`.

**Domain / Store layer:**
- Purpose: Persist and query memories with per-actor authorization and recall gating.
- Location: `internal/store/`
- Contains: `Store`, `Subject` sum type, cursor paging, payload mapping, subject filters.
- Depends on: Qdrant client, `internal/shortid`, `internal/telemetry`.
- Used by: server + Connect handlers, operator commands.

**Adapter layer:**
- Purpose: External I/O — embeddings endpoint and Qdrant gRPC.
- Location: `internal/embed/`, Qdrant client (`github.com/qdrant/go-client`).
- Depends on: `net/http`, gRPC.
- Used by: store and server wiring.

## Data Flow

### store_memory (MCP write path)

1. Request arrives at the MCP transport mounted at `/mcp` (`cmd/engram/serve.go:176` mountMCPRoutes).
2. `withAuth` validates the bearer JWT and resolves the owner-claim (`cmd/engram/serve.go:237`, `internal/auth/auth.go`).
3. Tool handler for `store_memory` runs, deriving a `store.Subject` from the verified token (`internal/server/tools.go:925`, `internal/server/identity.go`).
4. Handler calls the embedder to vectorize content (`internal/embed/embed.go` Embed).
5. Store stamps `actor`/`owner`, mints a `short_id`, and upserts the point into Qdrant (`internal/store/store.go`, `internal/shortid/shortid.go`).
6. Result (id + short_id) is returned to the caller.

### search_memory / list_memory (recall path)

1. Request hits MCP tool handler (`internal/server/tools.go:937`/`:943`).
2. Query text is embedded via `EmbedQuery` (asymmetric, optional instruction prefix) (`internal/embed/embed.go`).
3. Store applies the exhaustive `Subject` type-switch authz filter plus the recall gate (scheduled/expired windows) and optional tag/time filters (`internal/store/store.go`).
4. Compact summaries returned by default; `full=true` returns content; `get_memory` bypasses the recall gate.

### Web console read (Connect API)

1. SPA calls `/engram.v1.EngramService/*` with the sealed session cookie.
2. `webauth.Resolver.Resolve` maps the cookie to an identity (`internal/webauth/resolver.go`), wired as `connectResolve` (`cmd/engram/serve.go:138`).
3. Connect handler (`internal/server/connectapi.go`) queries the same store, read-only, and shapes proto responses.

**State Management:**
- All durable state lives in Qdrant; the process is stateless apart from OIDC verifier caches and sealed session/flow cookies (`internal/webauth/session.go`).

## Key Abstractions

**Subject (sealed sum type):**
- Purpose: Verified caller identity that drives authorization.
- Examples: `internal/store/subject.go`
- Pattern: Unexported variants (`anonymous`, `authenticated`); zero value is nil to fail closed. Read filters and write gates use an exhaustive type switch with a default-deny arm, never `Owner()`.

**deps (handler context):**
- Purpose: Bundles the store, embedder interface, clock, and recall truncation cap for tool handlers.
- Examples: `internal/server/tools.go` (`type deps struct`).
- Pattern: Constructor-injected dependencies; tests override `configLoad`, `now`.

**Field registry (config):**
- Purpose: Single source of truth for every `ENGRAM_` var, its default, legacy name, and flag.
- Examples: `internal/config/registry.go`.
- Pattern: Table-driven — defaults layer, env layer, and changed-flags overlay all derive from the same slice.

**SummarySource (contract-locked string):**
- Purpose: Records provenance of a memory's summary across proto/MCP/Qdrant boundaries.
- Examples: `internal/store/store.go`.

## Entry Points

**`engram serve`:**
- Location: `cmd/engram/serve.go` (runServe).
- Triggers: operator/Helm start.
- Responsibilities: build telemetry, mux, MCP server + tools, auth lanes, Connect handler, and HTTP server.

**Operator commands:**
- Location: `cmd/engram/reindex.go`, `migrate.go`, `prune.go`, `summarize.go`, `backfill.go`.
- Triggers: manual operator invocation.
- Responsibilities: embedder migration, owner remap, expiry prune, summary backfill, short-id backfill.

## Architectural Constraints

- **Threading:** Standard Go `net/http` server — one goroutine per request; store and embedder must be concurrency-safe (stateless clients).
- **Global state:** Package-level OTel tracers per package (`var tracer = otel.Tracer(...)`); `configLoad` seam in `internal/server/tools.go`. No mutable domain singletons.
- **Circular imports:** None observed; dependency direction is CLI → server → store → adapters.
- **Identity trust:** Clients never assert identity. Only IdP-signed JWT (MCP lane) or sealed session cookie (web lane) establishes a `Subject`. Nil `Subject` fails closed.
- **Generated code:** `gen/` is committed and CI-checked for drift against `proto/`; never hand-edit.

## Anti-Patterns

### Reading identity from client-supplied fields

**What happens:** Trusting an `actor`/`owner` value sent in a request body.
**Why it's wrong:** Lets any caller impersonate another and bypass per-actor isolation.
**Do this instead:** Derive `Subject` from the verified token/session only; `actor`/`owner` are server-set (`internal/store/subject.go`, `internal/server/identity.go`).

### Enforcing authz via `Subject.Owner()`

**What happens:** Using `Owner()` string comparison as the access gate.
**Why it's wrong:** `Owner()` is a persistence accessor, not enforcement; it can't express the anonymous-vs-authenticated distinction and silently mis-grants on nil.
**Do this instead:** Use the exhaustive type switch with a default-deny arm (`internal/store/store.go`, documented in `subject.go`).

### Adding a new `ENGRAM_` var ad hoc

**What happens:** Reading `os.Getenv` directly in a command or handler.
**Why it's wrong:** Bypasses defaults, legacy-guard, and flag-overlay wiring; drifts the single source of truth.
**Do this instead:** Add a `field` entry in `internal/config/registry.go`.

## Error Handling

**Strategy:** Sentinel errors classified at the store boundary, mapped to transport codes at the Connect/MCP edge.

**Patterns:**
- `store.ErrNotFound` deliberately conflates "absent" and "not visible" so ownership never leaks (`internal/store/store.go`).
- `store.ErrInvalidArgument` distinguishes malformed client requests from infra failures; Connect maps it to `CodeInvalidArgument`, everything else to `CodeInternal`.
- Fail-fast config validation before any client is built (`loadAndValidate` in `internal/server/tools.go`).
- Wrapped errors with `fmt.Errorf("...: %w", err)` throughout.

## Cross-Cutting Concerns

**Logging:** `log/slog` with an OTel bridge; configured in `internal/telemetry/logger.go`.
**Validation:** `Config.Validate()` (`internal/config/validate.go`); serve-local listen-addr guard in `serve.go`.
**Authentication:** Two lanes — bearer JWT (`internal/auth`) for MCP, OIDC auth-code + sealed cookie (`internal/webauth`) for the SPA/Connect API. No issuer configured disables validation (logged loudly).

---

*Architecture analysis: 2026-07-08*
