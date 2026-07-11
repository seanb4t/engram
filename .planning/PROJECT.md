# engram

## What This Is

engram is a self-hosted, correctable, OAuth-secured memory MCP server for coding agents,
backed by Qdrant. It exposes an explicit, zero-junk memory contract (store / schedule /
search / list / get / update / delete plus discovery and rule kinds) over MCP, with a
ConnectRPC read API, a SvelteKit operator console, and an Astro Starlight docs site. It is
distributed as a container image and a Helm chart (server + Qdrant) for Kubernetes.

This is a **retrospective baseline** extended by GSD-tracked milestones: engram is already
shipped. Every locked decision and every routed requirement below is **implemented and merged
to main**. This document records the as-built state so future milestones build on an accurate
foundation.

**Latest milestone — v0.9.x — Recall Quality — ✅ SHIPPED 2026-07-10** (PR #336; opened
2026-07-09): made recall measurably trustworthy as the store grows, so `search-before-store`
stops producing duplicates. Four phases (9–12): retrieval eval + ranking precision (#261),
embedder query/document asymmetry (#305, already shipped under Phase 4), async-on-write
summaries (#320), and per-memory usage signals (#317). 6/6 requirements satisfied; audit
PASSED. Full detail in `.planning/milestones/v0.9.x-ROADMAP.md`.

**Active milestone — v0.10.x — Hardening & Write Lane** (opened 2026-07-10): harden the
operational surfaces that bit us in v0.9.x (embedder reliability), add the deferred Connect
**write lane** with proper auth (CSRF + session rotation), and pay down the promoted-but-unbuilt
correctness/CI backlog — so engram is production-solid and writable over Connect. See
`.planning/ROADMAP.md` and `.planning/REQUIREMENTS.md`.

## Current Milestone: v0.10.x — Hardening & Write Lane

**Goal:** Make engram production-solid and writable over Connect — fix the embedder-reliability
gaps surfaced by the v0.9.x eval brownouts, ship the deferred Connect write lane with CSRF +
session-rotation hardening, and clear the correctness/CI backlog.

**Target features:**
- **Embedder reliability & options** — configurable HTTP timeout (#333), base-URL `/v1` join fix (#332), Gemini direct (#331), prod-parity #261 re-confirm on qwen3 (#334, closes #261), embedding-model docs + Helm recipes (#337).
- **Connect write lane + auth hardening** — StoreMemory/StoreDiscovery write RPCs + CSRF (#322), session refresh-token rotation (#323). Security-sensitive centerpiece (threat-modeled).
- **Correctness & polish tail** — SearchDiscoveries proto fidelity (#307), MintShortID bounded cap (#308), embed/discovery polish (#302/#303/#304), summarize-missing CronJob in Helm (#269), from-beads refactor cluster.
- **CI / maintenance hygiene** — Renovate vendored-SPA drift (#301), Phase-11 review residuals (#335), `.rumdl.toml` `.planning` exclude.

## Core Value

**Correctable recall precision** — a coding agent gets back the RIGHT memory for its context,
and wrong or stale memories can be corrected or superseded, so recall stays trustworthy as the
store grows. Everything ladders up to this: relevant/correct/current recall, explicit
zero-junk capture (no auto-extraction), and per-actor isolation that keeps each agent's recall
clean.

## Requirements

### Validated

Shipped and relied upon. Baseline IDs/phase mapping and the full v0.9.x requirement
text are archived in `.planning/milestones/v0.9.x-REQUIREMENTS.md` (which embeds the full
pre-close `REQUIREMENTS.md` snapshot).

**v0.8.x baseline (Phases 1–7):**

- ✓ **Authorization & Isolation** — per-actor isolation, typed Subject authz, configurable-claim owner (Phase 1)
- ✓ **Recall Semantics** — scheduled/windowed recall, cursor paging, summary-by-default (Phase 2)
- ✓ **Memory Kinds & Tools** — discovery kind, rule kind, short_id handle, schedule tools (Phase 3)
- ✓ **Embedder** — protocol-named vars, asymmetric query/document param passthrough (Phase 4)
- ✓ **Config & Transport** — ENGRAM_ koanf config, Config.Validate, fatal legacy guard, MCP path (Phase 5)
- ✓ **Telemetry & Observability** — slog + OTel over OTLP at every seam, non-blocking (Phase 6)
- ✓ **Web UI, Docs Site & Distribution** — operator console SPA, docs site, brand, bundled client plugin (Phase 7)
- ✓ **Connect Observe-Lane Auth Hardening** — cookie/OIDC observe lane (R1–R4) (Phase 8; PR #248/#266)

**v0.9.x — Recall Quality (Phases 9–12; shipped 2026-07-10, PR #336):**

- ✓ **REQ-retrieval-eval** — reproducible retrieval eval (`task eval:retrieval`, #261 fixture, recall@k/MRR) — v0.9.x
- ✓ **REQ-search-similarity-scores** — always-on per-result similarity score in `search_memory` — v0.9.x
- ✓ **REQ-ranking-precision** — dependency-free reranker kills phrasing-sensitivity (recall@8=1.00 on #261) — v0.9.x
- ✓ **REQ-embedder-native-params** — native query/document param passthrough + doc-side prefix — v0.9.x (already shipped under Phase 4)
- ✓ **REQ-async-summaries** — async-on-write summary fill off the write path, eval-gated — v0.9.x
- ✓ **REQ-usage-signals** — per-record usage counters (get/update), hybrid OTLP+payload, never affects ranking — v0.9.x

### Active

**v0.10.x — Hardening & Write Lane** (being scoped; full text in `.planning/REQUIREMENTS.md`):

- Embedder reliability & options (#333/#332/#331/#334/#337; closes #261)
- Connect write lane + auth hardening (#322 write RPCs + CSRF, #323 session rotation)
- Correctness & polish tail (#307/#308/#302/#303/#304/#269 + from-beads refactors)
- CI / maintenance hygiene (#301/#335, `.rumdl.toml` `.planning` exclude)

### Deferred (v0.10.x candidates — not in v0.9.x)

- [ ] **Connect write-lane RPCs** — `StoreMemory`/`StoreDiscovery` over Connect + CSRF-token hardening (GitHub #322)
- [ ] **Session refresh-token rotation** — re-seal on access-token expiry (v1 trusts the sealed cookie's `sub` until the session TTL) (GitHub #323)
- [ ] **Consolidation tail** — short_id polish (cluster C), embed refactors (#302/#304), summarize CronJob (#269), SearchDiscoveries proto fidelity (#307)

> **REQ-connect-auth-posture (R1–R4)** was found **already shipped** (PR #248/#266) and reconciled to Phase 8 = Complete on 2026-07-08 — no longer active.

### Out of Scope

- **Auto-extraction of memories** — explicit, user-blessed capture is a core design invariant; automatic memory harvesting is deliberately excluded to keep recall zero-junk.
- **Prometheus `/metrics` scrape endpoint** — telemetry is OTLP-gRPC only (DEC-dwi).
- **Server-side rendering (SSR)** — the operator console is an adapter-static SPA and the docs site is static-only (DEC-0lu, DEC-ttb).
- **viper / config files / `MEM_*` env vars** — config is ENGRAM_-prefixed koanf only; `MEM_*` is a fatal startup guard (DEC-jgq, DEC-irq).
- **Database migrations, cocogitto** — not used in this project.
- **Separate Qdrant collections per memory kind** — discovery/rule/scheduled all live in the single Memory collection (DEC-2bv).

## Context

- **Ecosystem:** Go 1.26 static binary (`CGO_ENABLED=0`, distroless), Qdrant gRPC vector store, OpenAI-compatible embeddings/chat gateway. UI/docs built with pnpm + Node (not in the server image).
- **Surfaces:** MCP tool server (primary, StreamableHTTP at `/mcp`), ConnectRPC `EngramService` v1 read API, SvelteKit adapter-static operator console vendored via `go:embed`, Astro Starlight docs site on Cloudflare Workers.
- **Identity:** OIDC bearer tokens on the MCP lane become the memory `actor`; the authz `owner` key is a configurable claim (default `email`). No issuer → single anonymous empty-owner bucket.
- **VCS/build:** git (branch + PR; never push to `main` directly); `task` runner; buf-generated `gen/` tree committed and CI-checked; release-please-driven releases (binary + image via goreleaser, OCI Helm chart).
- **Connect observe lane:** authenticated via the cookie/OIDC lane (sealed session → verified `sub`); mounted only when the UI is enabled, headless by default (R1–R4 shipped in PR #248/#266, reconciled 2026-07-08). The MCP lane's no-issuer anonymous empty-owner bucket is unaffected.
- **Recall quality (v0.9.x, shipped 2026-07-10):** a labeled retrieval eval harness (`task eval:retrieval`, `internal/retrievaleval`) with a permanent #261 regression fixture; an always-on `search_memory` similarity score; a stdlib-only lexical-overlap reranker shared via `store.SearchReranked` (MCP + Connect); async-on-write summary fill via a bounded worker pool (`internal/server/summaryqueue.go`) off the write path; and per-record usage signals (`access_count`/`last_accessed_at`, `usagequeue.go`) that never affect ranking. Two reusable Go kernels: CR-01 shutdown-safety (RWMutex+closed) and `*time.Time` for optional timestamps.

## Constraints

- **Tech stack**: Go + Qdrant + MCP go-sdk + koanf + connect-go + go-oidc — Established, ADR-locked; the memory contract and authz model depend on them.
- **Security**: Authorization is enforced in `internal/store` (Qdrant read filters + owner gates), never in handlers — Prevents handler-level authz drift; store is the single default-deny chokepoint (DEC-cgb, DEC-12c).
- **Security**: Unauthorized id-addressed ops return the same not-found as a missing id — Prevents cross-actor existence leaks (DEC-xa6).
- **Compatibility**: `short_id` is 10-char Crockford base32, accepted anywhere an id is — Stable public handle; legacy records backfilled via `engram backfill-short-ids` (DEC-zzq0, DEC-02ta).
- **Observability**: Telemetry is OTLP-gRPC only and never a hard startup dependency — No Prometheus scrape; missing collector yields no-op providers (DEC-dwi, DEC-uxh).
- **Config**: Single ENGRAM_ field registry via koanf; retired `MEM_*` vars are a fatal guard — No silent fallback or dual-read shim (DEC-jgq, DEC-irq).
- **Frontend**: Operator console is adapter-static + `go:embed`; docs site is static-only — SSR dropped end-to-end (DEC-0lu, DEC-ttb).
- **Testing**: UI/sanitizer tests run under vitest 4 browser mode (real Chromium) — jsdom/happy-dom retired so DOMPurify + bits-ui exercise a real DOM.

## Decisions

All 56 ADRs are **LOCKED** (precedence 0) and cannot be auto-overridden by any lower-precedence
source: **25 core** decisions (headline choices, below, grouped by delivering phase) plus **31
companion refinements** (folded 2026-07-08 — finer-grained decisions that each refine a core lock;
see the *Companion / Refinement Decisions* subsection). Source of record: `docs/adr/engram-*.md`.
Implementation plans behind each phase are cross-referenced in
`.planning/intel/merge-plans/context.md`.

<decisions>

### Phase 1 — Authorization & Isolation

#### DEC-cgb — Enforce per-actor authorization in the store layer, not in handlers — [LOCKED]

- **Source:** docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md
- **Decision:** Per-actor authorization is enforced inside `internal/store` via Qdrant read filters and owner-gate primitives, not in MCP handlers.
- **Scope:** internal/store, Qdrant read filters, authorization, MCP tool handlers, owner isolation

#### DEC-g37x — Use configurable OIDC claim as record owner (default: email) — [LOCKED]

- **Source:** docs/adr/engram-g37x-use-configurable-oidc-claim-as-record-owner-default-email.md
- **Decision:** The record authz owner key is a configurable OIDC claim (default `email`, via `ENGRAM_OWNER_CLAIM`) so ownership survives IdP `sub` rotation.
- **Scope:** owner authz key, ENGRAM_OWNER_CLAIM, OIDC claim resolution, migrate-remap-owner, session cookie sealing

#### DEC-kyz — Sharing grants read but never write (read/write gate asymmetry) — [LOCKED]

- **Source:** docs/adr/engram-kyz-sharing-grants-read-but-never-write-read-write-gate-asymmetr.md
- **Decision:** Access primitives are asymmetric — sharing grants read access only; owners retain exclusive write/delete/visibility control.
- **Scope:** getReadable, getWritable, ownedOrAbsent, DeleteAll, shared visibility, id-addressed store ops, authz gates

#### DEC-xa6 — Return 404 not-found for unauthorized id-addressed operations — [LOCKED]

- **Source:** docs/adr/engram-xa6-return-404-not-found-unauthorized-id-addressed-operations.md
- **Decision:** All owner/visibility mismatches return the same not-found error as a missing id, preventing cross-actor existence leaks.
- **Scope:** get_memory, update_memory, delete_memory, set_visibility, discovery overwrite, ownership authz, ErrNotFound

#### DEC-12c — Represent authz Subject as a sealed Go interface — [LOCKED]

- **Source:** docs/adr/engram-12c-represent-authz-subject-as-sealed-go-interface.md
- **Decision:** Authz caller identity is modeled as a sealed Go interface with default-deny exhaustive type switches in the store layer.
- **Scope:** internal/store, authz Subject, anonymous/authenticated variants, store enforcement gates, Qdrant owner payload

### Phase 2 — Recall Semantics

#### DEC-ambu — Recall returns summary by default with full-content opt-in — [LOCKED]

- **Source:** docs/adr/engram-ambu-recall-returns-summary-by-default-full-content-opt.md
- **Decision:** Search/list recall returns summary-shaped output by default with a `full=true` opt-in for full content; `get_memory` is unchanged.
- **Scope:** search_memory, list_memory, get_memory, Connect SearchMemories/ListMemories, memory contract, web UI

#### DEC-4xt7 — Tag-filtered recall: hard Qdrant filter, AND-default — [LOCKED]

- **Source:** docs/adr/engram-4xt7-tag-filtered-recall-hard-qdrant-filter-and-default.md
- **Decision:** The optional `tags` filter on search_memory/list_memory is a hard AND (contains-all) Qdrant pre-filter composed onto the authz envelope.
- **Scope:** search_memory, list_memory, Store.Search, Store.List, Qdrant tag filter, tags recall dimension

#### DEC-y1g — Gate recall via Qdrant filter; leave get_memory ungated — [LOCKED]

- **Source:** docs/adr/engram-y1g-gate-recall-via-qdrant-filter-leave-get-memory-ungated.md
- **Decision:** Temporal validity is enforced as Qdrant filter conditions on Search/List, while get_memory and by-id paths stay ungated for record management.
- **Scope:** Qdrant filter, Search, List, get_memory, temporal validity gate, store layer

#### DEC-1frj — Boundary id-set cursor with half-open date window for recall — [LOCKED]

- **Source:** docs/adr/engram-1frj-boundary-id-set-cursor-half-open-date-window-recall.md
- **Decision:** Adopt an opaque boundary id-set cursor over `created_at` with a half-open date window for deterministic O(limit)-per-page recall paging.
- **Scope:** list_memory, MCP recall paging, cursor pagination, date-window recall, Connect ListMemories API, Qdrant ordering

#### DEC-ef28 — Index owner/scope/created_at as Qdrant payload indexes — [LOCKED]

- **Source:** docs/adr/engram-ef28-index-owner-scope-created-at-as-qdrant-payload-indexes.md
- **Decision:** Create keyword and datetime Qdrant payload indexes on owner/scope/created_at, retiring scanCap and the approximate flag for exact server-side Count and filtering.
- **Scope:** Qdrant payload indexes, List/ListScheduled/ListScopes, ensureCollection, owner authz filtering, created_at date-range queries

### Phase 3 — Memory Kinds & Tools

#### DEC-2bv — Discovery is a 5th category in the single Memory collection — [LOCKED]

- **Source:** docs/adr/engram-2bv-discovery-is-5th-category-single-memory-collection.md
- **Decision:** Discovery is added as a 5th category on the existing Memory record in one Qdrant collection rather than a separate collection.
- **Scope:** discovery category, Memory record, Qdrant collection, internal/store/store.go, query-time isolation filter

#### DEC-90w — Add schedule_memory/list_scheduled tools; keep store_memory windowless — [LOCKED]

- **Source:** docs/adr/engram-90w-add-schedule-memory-list-scheduled-tools-keep-store-memory-w.md
- **Decision:** Add dedicated `schedule_memory` and `list_scheduled` MCP tools rather than adding temporal window params to `store_memory`.
- **Scope:** schedule_memory, list_scheduled, store_memory, MCP tool surface, temporal validity windows

#### DEC-iedk — Rules are always-shared with server-set immutable visibility; set_visibility rejects rules — [LOCKED]

- **Source:** docs/adr/engram-iedk-rules-are-always-shared-server-set-immutable-visibility-set.md
- **Decision:** Rule-category memories are server-set to `shared` and immutable; the `set_visibility` handler rejects any call targeting a rule.
- **Scope:** rule memory kind, set_visibility MCP handler, visibility, shared-read grant, GetReadable
- **Relates to:** DEC-kyz

#### DEC-zzq0 — Encode short_id as 10-char Crockford base32 — [LOCKED]

- **Source:** docs/adr/engram-zzq0-encode-short-id-as-10-char-crockford-base32.md
- **Decision:** Encode memory record `short_id` as a 10-char lowercase Crockford base32 token instead of ULID or Sqids.
- **Scope:** short_id, memory records, Crockford base32 encoding, Qdrant lookup

#### DEC-02ta — Resolve short_id at the handler layer, not inside store methods — [LOCKED]

- **Source:** docs/adr/engram-02ta-resolve-short-id-at-handler-layer-not-inside-store-methods.md
- **Decision:** Resolve `short_id` to UUID via a shared `Store.ResolvePointID` method called from each by-id handler rather than inside store methods.
- **Scope:** short_id resolution, Store.ResolvePointID, MCP by-id tools, Connect GetMemory RPC, ownership gates

### Phase 4 — Embedder

#### DEC-378 — Name embedder connection vars by protocol, not implementation — [LOCKED]

- **Source:** docs/adr/engram-378-name-embedder-connection-vars-by-protocol-not-implementation.md
- **Decision:** Rename embedder connection env vars from `MEM_LITELLM_*` to `ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`, naming the wire protocol not the vendor.
- **Scope:** embedder, environment variables, ENGRAM_OPENAI_BASE_URL, ENGRAM_OPENAI_API_KEY, OpenAI-compatible /v1/embeddings API, embed.New
- **Note:** Also realizes the embedder half of REQ-config-prefix-koanf (Phase 5).

#### DEC-zyhq — Generic param-map passthrough over embedder profiles for asymmetric/cloud embedders — [LOCKED]

- **Source:** docs/adr/engram-zyhq-generic-param-map-passthrough-over-embedder-profiles-asymmet.md
- **Decision:** Expose query/document embedding params as raw provider-agnostic JSON maps (`ENGRAM_EMBED_QUERY_PARAMS`/`ENGRAM_EMBED_DOCUMENT_PARAMS`) merged into the /v1/embeddings body, instead of per-provider profiles.
- **Scope:** embedder, embed.Client, ENGRAM_EMBED_QUERY_PARAMS, ENGRAM_EMBED_DOCUMENT_PARAMS, /v1/embeddings request body, cloud/gateway embedders

### Phase 5 — Config & Transport

#### DEC-jgq — Unify config under ENGRAM_ prefix via koanf internal/config — [LOCKED]

- **Source:** docs/adr/engram-jgq-unify-config-under-engram-prefix-via-koanf-internal-config.md
- **Decision:** Introduce `internal/config` (koanf v2) with a single field registry owning all `ENGRAM_` keys, retiring scattered EnvOr/getenv reads and the `MEM_` prefix.
- **Scope:** internal/config, koanf, ENGRAM_ env vars, cmd/engram, server.EnvOr, CLI flags

#### DEC-irq — Breaking config renames ship with a fatal legacy-env startup guard — [LOCKED]

- **Source:** docs/adr/engram-irq-breaking-config-renames-ship-fatal-legacy-env-startup-guard.md
- **Decision:** Retired `MEM_*` env vars trigger a fatal registry-derived startup guard (`config.CheckLegacy`) rather than a silent fallback or dual-read shim.
- **Scope:** config.CheckLegacy, field registry, MEM_* env vars, PersistentPreRunE, startup guard

#### DEC-bj6 — MCP transport at explicit configurable path; console at root when UI enabled — [LOCKED]

- **Source:** docs/adr/engram-bj6-mcp-transport-at-explicit-configurable-path-mem-mcp-path-con.md
- **Decision:** Mount the MCP StreamableHTTP transport at an explicit configurable path (default `/mcp`) instead of the root catch-all; the console takes root when the UI is enabled.
- **Scope:** MCP transport, MEM_MCP_PATH, HTTP routing, web console/UI, Helm chart memory.mcpPath, mountMCPRoutes seam

### Phase 6 — Telemetry & Observability

#### DEC-dwi — Export telemetry via OTLP only; omit a Prometheus scrape endpoint — [LOCKED]

- **Source:** docs/adr/engram-dwi-export-telemetry-via-otlp-only-omit-prometheus-scrape-endpoi.md
- **Decision:** Export metrics, traces, and logs exclusively over OTLP gRPC to a collector; add no Prometheus `/metrics` scrape endpoint.
- **Scope:** telemetry, OTLP gRPC exporter, OpenTelemetry Collector, Prometheus /metrics endpoint, Grafana LGTM backend, Helm chart

#### DEC-uxh — Telemetry is never a hard server startup dependency — [LOCKED]

- **Source:** docs/adr/engram-uxh-telemetry-is-never-hard-server-startup-dependency.md
- **Decision:** A telemetry setup failure or missing OTLP endpoint yields no-op providers and never aborts engram server startup.
- **Scope:** telemetry, OTLP exporter, server startup, Helm chart defaults, observability subsystem

### Phase 7 — Web UI, Docs Site & Distribution

#### DEC-8xe — Adopt ConnectRPC and protobuf/buf for the web UI API — [LOCKED]

- **Source:** docs/adr/engram-8xe-adopt-connectrpc-and-protobuf-buf-web-ui-api.md
- **Decision:** Adopt ConnectRPC with the protobuf/buf toolchain scoped to the web-UI API, reversing the prior "no protobuf" convention. MCP core stays as-is.
- **Scope:** ConnectRPC, protobuf, buf toolchain, web-UI API, connect-go, connect-es, MCP core

#### DEC-0lu — SvelteKit adapter-static SPA vendored via go:embed, SSR dropped — [LOCKED]

- **Source:** docs/adr/engram-0lu-sveltekit-adapter-static-spa-vendored-via-go-embed-ssr-dropp.md
- **Decision:** Build the engram frontend as a SvelteKit adapter-static SPA vendored via `go:embed`, dropping SSR entirely.
- **Scope:** frontend, SvelteKit, go:embed, SPA, BFF, engram binary, connect-es client

#### DEC-ttb — Deploy docs-site via Workers Static Assets without an SSR adapter — [LOCKED]

- **Source:** docs/adr/engram-ttb-deploy-docs-site-via-workers-static-assets-without-ssr-adapt.md
- **Decision:** Serve the Astro Starlight docs-site as static assets via a Cloudflare Workers assets binding, with no SSR adapter or worker script.
- **Scope:** docs-site, Cloudflare Workers Static Assets, Astro Starlight, wrangler.jsonc, @astrojs/cloudflare adapter

### Companion / Refinement Decisions (31 fine-grained ADRs — folded 2026-07-08)

All **LOCKED** (precedence 0). Each refines or implements a scope already governed at a coarser
grain by a core decision above — they add no new product scope. Full text lives in the ADR source
and `.planning/intel/merge-adrs/decisions.md`; the `refines →` note names the core lock elaborated.

**Phase 2 — Recall Semantics**

- **DEC-4y7p** — Explicit-first memory summary; missing ones filled by an offline `engram summarize-missing` sweep, never on the write path. *(refines DEC-ambu)* — `docs/adr/engram-4y7p-explicit-first-memory-summary-offline-operator-auto-fill.md`
- **DEC-ddiw** — Reject `update_memory` on content change when a client-authored summary is left unaddressed; auto-clear an auto summary instead. *(refines DEC-ambu)* — `docs/adr/engram-ddiw-reject-update-memory-content-change-unaddressed-client-summa.md`
- **DEC-ufz** — Soft-hide expired records at the recall gate; reclaim storage only via explicit `engram prune-expired`. *(refines DEC-y1g)* — `docs/adr/engram-ufz-soft-hide-expired-records-at-recall-opt-prune-expired-storag.md`
- **DEC-c0m** — Inject the `Store` clock via a `WithClock` functional option; keep public signatures stable. *(refines DEC-y1g)* — `docs/adr/engram-c0m-inject-store-clock-via-withclock-option-keep-public-signatur.md`

**Phase 3 — Memory Kinds & Tools**

- **DEC-0gy** — Dedicated `store_discovery`/`search_discovery` tools rather than overloading `store_memory`. *(refines DEC-2bv)* — `docs/adr/engram-0gy-dedicated-store-discovery-search-discovery-tools-not-overloa.md`
- **DEC-3l0** — Surface raw citation-pin/`created_at` aging signals for discovery trust; no server-computed freshness verdict. *(refines DEC-2bv)* — `docs/adr/engram-3l0-graceful-decay-over-binary-staleness-discovery-trust.md`
- **DEC-d386** — Session-start surfaces rules as a one-line-per-rule progressive-disclosure index via `list_rules`. *(refines DEC-iedk/DEC-ambu)* — `docs/adr/engram-d386-session-start-surfaces-rules-as-progressive-disclosure-index.md`
- **DEC-m4s8** — Reject malformed rule summaries (newline / >256 B / cleared); never silently normalize. *(refines DEC-iedk, DEC-ddiw)* — `docs/adr/engram-m4s8-reject-malformed-rule-summaries-newline-oversize-cleared-nev.md`

**Phase 5 — Config & Transport**

- **DEC-wtw** — Keep `config.Load` assembly-only; check well-formedness via a separate pure `Config.Validate()`. *(refines DEC-jgq)* — `docs/adr/engram-wtw-keep-config-load-assembly-only-validate-via-separate-config.md`
- **DEC-d24** — `Config.Validate` checks only the five data-plane fields; `listen_addr` is a serve-local guard. *(refines DEC-wtw)* — `docs/adr/engram-d24-validate-data-plane-fields-only-listen-addr-is-serve-local-g.md`

**Phase 6 — Telemetry & Observability**

- **DEC-6gb** — Instrument store/embed/auth with inline OTel spans, not a decorator layer. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-6gb-instrument-store-embed-auth-inline-spans-not-decorator-layer.md`
- **DEC-f7p** — Instrument three seams: HTTP, MCP method (`AddReceivingMiddleware`), downstream clients. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-f7p-instrument-at-three-seams-http-mcp-method-and-downstream-cli.md`
- **DEC-tdk** — Instrument MCP tools from one `AddReceivingMiddleware` seam, not per-handler. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-tdk-instrument-mcp-tools-via-addreceivingmiddleware-not-per-hand.md`
- **DEC-wot** — Spans carry `engram.owner` (opaque `sub`) only; exclude actor/email as PII. *(refines DEC-dwi/DEC-uxh)* — `docs/adr/engram-wot-spans-carry-engram-owner-only-exclude-actor-and-email-as-pii.md`
- **DEC-7qd** — Configure sampler + export interval via OTel-standard env vars; no `MEM_*` counterparts. *(refines DEC-dwi)* — `docs/adr/engram-7qd-reuse-otel-standard-env-vars-sampler-and-export-interval-add.md`
- **DEC-9tj** — Inject k8s resource attributes via the Helm chart Downward API, not a Go SDK detector. *(refines DEC-dwi)* — `docs/adr/engram-9tj-inject-k8s-resource-attributes-via-chart-downward-api-not-go.md`

**Phase 7 — Web UI, Docs Site & Distribution**

- **DEC-bgj** — Embed the BFF (OIDC login/callback, session, static serving) in the engram Go binary, not a Node runtime. *(refines DEC-0lu/DEC-8xe)* — `docs/adr/engram-bgj-embed-bff-engram-go-binary-not-node-runtime.md`
- **DEC-8q3** — Operator-console session cookie seals only `{sub, expiry}`; no OIDC tokens client-side (read-only v1 lane). *(refines DEC-g37x)* — `docs/adr/engram-8q3-session-cookie-seals-only-sub-expiry-no-oidc-tokens-stored-c.md`
- **DEC-u9v** — Stateless AES-GCM encrypted-cookie session, no server-side store (eventual write-phase custody). *(refines DEC-g37x)* — `docs/adr/engram-u9v-stateless-encrypted-cookie-session-no-server-side-store.md`
- **DEC-2xl** — Use `@tanstack/svelte-query` as the SPA's sole async data layer over connect-es. *(refines DEC-0lu)* — `docs/adr/engram-2xl-use-tanstack-svelte-query-as-spa-data-layer.md`
- **DEC-c4y** — Drive SPA shell state (scope/filters/query/selection) via URL query params. *(refines DEC-0lu)* — `docs/adr/engram-c4y-drive-spa-shell-state-via-url-parameters.md`
- **DEC-3nas** — Render user memory content via `marked` + DOMPurify allowlist as the sole `{@html}` entry point. *(refines DEC-0lu)* — `docs/adr/engram-3nas-render-user-memory-content-via-marked-dompurify-allowlist.md`
- **DEC-vxk** — SPA-fallback static handler serves `index.html` for extensionless `/ui/*` client routes. *(refines DEC-0lu)* — `docs/adr/engram-vxk-spa-fallback-static-handler-serve-index-html-client-routes.md`
- **DEC-4ag** — Drop the dashboard category-breakdown bar until a `listScopes` API extension provides real counts. *(refines DEC-0lu/DEC-8xe)* — `docs/adr/engram-4ag-gate-dashboard-category-breakdown-bar-listscopes-api-extensi.md`
- **DEC-lzz** — Adopt shadcn semantic tokens; retire the bespoke `eg-*`/`--cat-*` layer. *(refines DEC-0lu)* — `docs/adr/engram-lzz-adopt-shadcn-semantic-tokens-retire-bespoke-eg-cat-layer.md`
- **DEC-no3** — Ship the engram wordmark as inlined outlined SVG paths, not a webfont. *(refines DEC-0lu)* — `docs/adr/engram-no3-ship-engram-wordmark-as-outlined-svg-paths-not-webfont.md`
- **DEC-1h3k** — Two-tier vitest config: node tier + real-Chromium browser tier. *(refines the UI test-unification direction)* — `docs/adr/engram-1h3k-adopt-two-tier-vitest-config-node-real-chromium-browser.md`
- **DEC-om5b** — Run the node test tier on `environment:'node'`; drop happy-dom. *(refines DEC-1h3k)* — `docs/adr/engram-om5b-node-test-tier-environment-node-drop-happy-dom.md`
- **DEC-u5h** — Host the Astro Starlight docs site in-monorepo at `docs-site/` with tooling exemptions. *(refines DEC-ttb)* — `docs/adr/engram-u5h-host-docs-site-inside-engram-monorepo-at-docs-site.md`
- **DEC-1w7** — Deploy docs-site via a dedicated non-required GitHub Actions wrangler workflow. *(refines DEC-ttb)* — `docs/adr/engram-1w7-deploy-docs-site-via-repo-github-actions-wrangler-workflow.md`
- **DEC-50b** — engram plugin ships no bundled MCP server; `/engram-setup` is the sole registration path (plugin stays bundled). *(refines DEC-8xe distribution)* — `docs/adr/engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md`

</decisions>

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Authz enforced in store layer, not handlers (DEC-cgb, DEC-12c) | Single default-deny chokepoint; no handler-level authz drift | ✓ Good — shipped |
| Configurable-claim owner key, default email (DEC-g37x) | Ownership survives IdP `sub` rotation | ✓ Good — shipped |
| 404-uniform not-found for unauthorized id ops (DEC-xa6) | Prevents cross-actor existence leaks | ✓ Good — shipped |
| Summary-by-default recall with full opt-in (DEC-ambu) | Cuts recall token cost while preserving correctable full content | ✓ Good — shipped |
| Discovery/rule as extra categories in one collection (DEC-2bv, DEC-iedk) | Avoids collection sprawl; one authz/recall path | ✓ Good — shipped |
| 10-char Crockford base32 short_id (DEC-zzq0, DEC-02ta) | Stable, human-usable handle over ULID/Sqids | ✓ Good — shipped |
| ENGRAM_ koanf config + fatal legacy guard (DEC-jgq, DEC-irq) | One config surface; no silent fallback | ✓ Good — shipped |
| OTLP-only, non-blocking telemetry (DEC-dwi, DEC-uxh) | Observability without a hard startup dependency | ✓ Good — shipped |
| ConnectRPC + adapter-static SPA + static docs (DEC-8xe, DEC-0lu, DEC-ttb) | Read API + embeddable console without SSR complexity | ✓ Good — shipped |
| Cookie/OIDC Connect observe lane (R1–R4) | Mount-gated, cookie-only authz, obs parity, same-origin | ✓ Good — shipped (PR #248/#266) |
| Fold 31 companion ADRs + 24 plans into baseline (2026-07-08) | Complete the decision record; the original 50-doc bootstrap capped out | ✓ Good — merged, 0 conflicts |
| Eval-chosen ranking lever — stdlib lexical reranker over hybrid/cross-encoder (D-06, v0.9.x) | The #261 regression eval cleared the bar on the lightest lever; no new dep, no reindex | ✓ Good — recall@8=1.00; D-07/D-08 not needed |
| Always-on `search_memory` similarity score (D-04 supersession, v0.9.x) | Better DX than an opt-in flag; eval can assert score separation | ✓ Good — shipped |
| Async-on-write summaries via bounded worker pool off the write path (D-01/D-08, v0.9.x) | A gateway outage must never fail `store_memory`; drain-after-shutdown under CR-01 kernel | ✓ Good — shipped (#320); residuals #335 |
| Usage signals never affect ranking (D-08 invariant, v0.9.x) | Curation metadata, not a ranking input; usage-weighted recall is a separate future decision | ✓ Good — negative-space test enforced |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---

*Last updated: 2026-07-10 — opened milestone v0.10.x — Hardening & Write Lane (`/gsd-new-milestone`). Prior: 2026-07-10 shipped + archived v0.9.x — Recall Quality (PR #336/#338); 2026-07-08 folded 31 companion ADRs + 24 plans into the baseline; 2026-07-07 retrospective baseline ingest (v0.8.x shipped).*
