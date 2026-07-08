# Decisions (ADRs)

Synthesized from 25 ADRs, all **LOCKED** (precedence 0). Locked decisions cannot be
auto-overridden by any lower-precedence source. Each entry preserves the decision
verbatim in intent with full source provenance.

---

## Authorization & Isolation

### DEC-cgb — Enforce per-actor authorization in the store layer, not in handlers
- source: docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md
- status: LOCKED
- decision: Per-actor authorization is enforced inside `internal/store` via Qdrant read
  filters and owner-gate primitives, not in MCP handlers.
- scope: internal/store, Qdrant read filters, authorization, MCP tool handlers, owner isolation

### DEC-g37x — Use configurable OIDC claim as record owner (default: email)
- source: docs/adr/engram-g37x-use-configurable-oidc-claim-as-record-owner-default-email.md
- status: LOCKED
- decision: The record authz owner key is a configurable OIDC claim (default `email`, via
  `ENGRAM_OWNER_CLAIM`) so ownership survives IdP `sub` rotation.
- scope: owner authz key, ENGRAM_OWNER_CLAIM, OIDC claim resolution, migrate-remap-owner, session cookie sealing

### DEC-kyz — Sharing grants read but never write (read/write gate asymmetry)
- source: docs/adr/engram-kyz-sharing-grants-read-but-never-write-read-write-gate-asymmetr.md
- status: LOCKED
- decision: Access primitives are asymmetric — sharing grants read access only; owners
  retain exclusive write/delete/visibility control.
- scope: getReadable, getWritable, ownedOrAbsent, DeleteAll, shared visibility, id-addressed store ops, authz gates

### DEC-xa6 — Return 404 not-found for unauthorized id-addressed operations
- source: docs/adr/engram-xa6-return-404-not-found-unauthorized-id-addressed-operations.md
- status: LOCKED
- decision: All owner/visibility mismatches return the same not-found error as a missing id,
  preventing cross-actor existence leaks.
- scope: get_memory, update_memory, delete_memory, set_visibility, discovery overwrite, ownership authz, ErrNotFound

### DEC-12c — Represent authz Subject as a sealed Go interface
- source: docs/adr/engram-12c-represent-authz-subject-as-sealed-go-interface.md
- status: LOCKED
- decision: Authz caller identity is modeled as a sealed Go interface with default-deny
  exhaustive type switches in the store layer.
- scope: internal/store, authz Subject, anonymous/authenticated variants, store enforcement gates, Qdrant owner payload

---

## Recall Semantics

### DEC-ambu — Recall returns summary by default with full-content opt-in
- source: docs/adr/engram-ambu-recall-returns-summary-by-default-full-content-opt.md
- status: LOCKED
- decision: Search/list recall returns summary-shaped output by default with a `full=true`
  opt-in for full content; `get_memory` is unchanged.
- scope: search_memory, list_memory, get_memory, Connect SearchMemories/ListMemories, memory contract, web UI

### DEC-4xt7 — Tag-filtered recall: hard Qdrant filter, AND-default
- source: docs/adr/engram-4xt7-tag-filtered-recall-hard-qdrant-filter-and-default.md
- status: LOCKED
- decision: The optional `tags` filter on search_memory/list_memory is a hard AND
  (contains-all) Qdrant pre-filter composed onto the authz envelope.
- scope: search_memory, list_memory, Store.Search, Store.List, Qdrant tag filter, tags recall dimension

### DEC-y1g — Gate recall via Qdrant filter; leave get_memory ungated
- source: docs/adr/engram-y1g-gate-recall-via-qdrant-filter-leave-get-memory-ungated.md
- status: LOCKED
- decision: Temporal validity is enforced as Qdrant filter conditions on Search/List, while
  get_memory and by-id paths stay ungated for record management.
- scope: Qdrant filter, Search, List, get_memory, temporal validity gate, store layer

### DEC-1frj — Boundary id-set cursor with half-open date window for recall
- source: docs/adr/engram-1frj-boundary-id-set-cursor-half-open-date-window-recall.md
- status: LOCKED
- decision: Adopt an opaque boundary id-set cursor over `created_at` with a half-open date
  window for deterministic O(limit)-per-page recall paging.
- scope: list_memory, MCP recall paging, cursor pagination, date-window recall, Connect ListMemories API, Qdrant ordering

### DEC-ef28 — Index owner/scope/created_at as Qdrant payload indexes
- source: docs/adr/engram-ef28-index-owner-scope-created-at-as-qdrant-payload-indexes.md
- status: LOCKED
- decision: Create keyword and datetime Qdrant payload indexes on owner/scope/created_at,
  retiring scanCap and the approximate flag for exact server-side Count and filtering.
- scope: Qdrant payload indexes, List/ListScheduled/ListScopes, ensureCollection, owner authz filtering, created_at date-range queries

---

## Memory Kinds & Tools

### DEC-2bv — Discovery is a 5th category in the single Memory collection
- source: docs/adr/engram-2bv-discovery-is-5th-category-single-memory-collection.md
- status: LOCKED
- decision: Discovery is added as a 5th category on the existing Memory record in one Qdrant
  collection rather than a separate collection.
- scope: discovery category, Memory record, Qdrant collection, internal/store/store.go, query-time isolation filter

### DEC-90w — Add schedule_memory/list_scheduled tools; keep store_memory windowless
- source: docs/adr/engram-90w-add-schedule-memory-list-scheduled-tools-keep-store-memory-w.md
- status: LOCKED
- decision: Add dedicated `schedule_memory` and `list_scheduled` MCP tools rather than adding
  temporal window params to `store_memory`.
- scope: schedule_memory, list_scheduled, store_memory, MCP tool surface, temporal validity windows

### DEC-iedk — Rules are always-shared with server-set immutable visibility; set_visibility rejects rules
- source: docs/adr/engram-iedk-rules-are-always-shared-server-set-immutable-visibility-set.md
- status: LOCKED
- decision: Rule-category memories are server-set to `shared` and immutable; the
  `set_visibility` handler rejects any call targeting a rule.
- scope: rule memory kind, set_visibility MCP handler, visibility, shared-read grant, GetReadable
- relates-to: DEC-kyz (cross_ref)

### DEC-zzq0 — Encode short_id as 10-char Crockford base32
- source: docs/adr/engram-zzq0-encode-short-id-as-10-char-crockford-base32.md
- status: LOCKED
- decision: Encode memory record `short_id` as a 10-char lowercase Crockford base32 token
  instead of ULID or Sqids.
- scope: short_id, memory records, Crockford base32 encoding, Qdrant lookup

### DEC-02ta — Resolve short_id at the handler layer, not inside store methods
- source: docs/adr/engram-02ta-resolve-short-id-at-handler-layer-not-inside-store-methods.md
- status: LOCKED
- decision: Resolve `short_id` to UUID via a shared `Store.ResolvePointID` method called from
  each by-id handler rather than inside store methods.
- scope: short_id resolution, Store.ResolvePointID, MCP by-id tools, Connect GetMemory RPC, ownership gates

---

## Embedder

### DEC-378 — Name embedder connection vars by protocol, not implementation
- source: docs/adr/engram-378-name-embedder-connection-vars-by-protocol-not-implementation.md
- status: LOCKED
- decision: Rename embedder connection env vars from `MEM_LITELLM_*` to
  `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`, naming the wire protocol not the vendor.
- scope: embedder, environment variables, ENGRAM_OPENAI_BASE_URL, ENGRAM_OPENAI_API_KEY, OpenAI-compatible /v1/embeddings API, embed.New

### DEC-zyhq — Generic param-map passthrough over embedder profiles for asymmetric/cloud embedders
- source: docs/adr/engram-zyhq-generic-param-map-passthrough-over-embedder-profiles-asymmet.md
- status: LOCKED
- decision: Expose query/document embedding params as raw provider-agnostic JSON maps
  (`ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS`) merged into the /v1/embeddings
  body, instead of per-provider profiles.
- scope: embedder, embed.Client, ENGRAM_EMBED_QUERY_PARAMS, ENGRAM_EMBED_DOCUMENT_PARAMS, /v1/embeddings request body, cloud/gateway embedders

---

## Config

### DEC-jgq — Unify config under ENGRAM_ prefix via koanf internal/config
- source: docs/adr/engram-jgq-unify-config-under-engram-prefix-via-koanf-internal-config.md
- status: LOCKED
- decision: Introduce `internal/config` (koanf v2) with a single field registry owning all
  `ENGRAM_` keys, retiring scattered EnvOr/getenv reads and the `MEM_` prefix.
- scope: internal/config, koanf, ENGRAM_ env vars, cmd/engram, server.EnvOr, CLI flags

### DEC-irq — Breaking config renames ship with a fatal legacy-env startup guard
- source: docs/adr/engram-irq-breaking-config-renames-ship-fatal-legacy-env-startup-guard.md
- status: LOCKED
- decision: Retired `MEM_*` env vars trigger a fatal registry-derived startup guard
  (`config.CheckLegacy`) rather than a silent fallback or dual-read shim.
- scope: config.CheckLegacy, field registry, MEM_* env vars, PersistentPreRunE, startup guard

### DEC-bj6 — MCP transport at explicit configurable path (MEM_MCP_PATH), console at root when UI enabled
- source: docs/adr/engram-bj6-mcp-transport-at-explicit-configurable-path-mem-mcp-path-con.md
- status: LOCKED
- decision: Mount the MCP StreamableHTTP transport at an explicit configurable path (default
  `/mcp`) instead of the root catch-all; the console takes root when the UI is enabled.
- scope: MCP transport, MEM_MCP_PATH, HTTP routing, web console/UI, Helm chart memory.mcpPath, mountMCPRoutes seam

---

## Telemetry

### DEC-dwi — Export telemetry via OTLP only; omit a Prometheus scrape endpoint
- source: docs/adr/engram-dwi-export-telemetry-via-otlp-only-omit-prometheus-scrape-endpoi.md
- status: LOCKED
- decision: Export metrics, traces, and logs exclusively over OTLP gRPC to a collector; add
  no Prometheus `/metrics` scrape endpoint.
- scope: telemetry, OTLP gRPC exporter, OpenTelemetry Collector, Prometheus /metrics endpoint, Grafana LGTM backend, Helm chart

### DEC-uxh — Telemetry is never a hard server startup dependency
- source: docs/adr/engram-uxh-telemetry-is-never-hard-server-startup-dependency.md
- status: LOCKED
- decision: A telemetry setup failure or missing OTLP endpoint yields no-op providers and
  never aborts engram server startup.
- scope: telemetry, OTLP exporter, server startup, Helm chart defaults, observability subsystem

---

## Web UI & Docs Site

### DEC-8xe — Adopt ConnectRPC and protobuf/buf for the web UI API
- source: docs/adr/engram-8xe-adopt-connectrpc-and-protobuf-buf-web-ui-api.md
- status: LOCKED
- decision: Adopt ConnectRPC with the protobuf/buf toolchain scoped to the web-UI API,
  reversing the prior "no protobuf" convention. MCP core stays as-is.
- scope: ConnectRPC, protobuf, buf toolchain, web-UI API, connect-go, connect-es, MCP core

### DEC-0lu — SvelteKit adapter-static SPA vendored via go:embed, SSR dropped
- source: docs/adr/engram-0lu-sveltekit-adapter-static-spa-vendored-via-go-embed-ssr-dropp.md
- status: LOCKED
- decision: Build the engram frontend as a SvelteKit adapter-static SPA vendored via
  `go:embed`, dropping SSR entirely.
- scope: frontend, SvelteKit, go:embed, SPA, BFF, engram binary, connect-es client

### DEC-ttb — Deploy docs-site via Workers Static Assets without an SSR adapter
- source: docs/adr/engram-ttb-deploy-docs-site-via-workers-static-assets-without-ssr-adapt.md
- status: LOCKED
- decision: Serve the Astro Starlight docs-site as static assets via a Cloudflare Workers
  assets binding, with no SSR adapter or worker script.
- scope: docs-site, Cloudflare Workers Static Assets, Astro Starlight, wrangler.jsonc, @astrojs/cloudflare adapter
