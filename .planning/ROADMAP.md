# Roadmap: engram

## Overview

This is a **retrospective / as-built roadmap**. engram is already shipped (current release line
**v0.8.x**). Phases 1–7 group the already-completed work by synthesis area — Authorization &
Isolation, Recall Semantics, Memory Kinds & Tools, Embedder, Config & Transport, Telemetry &
Observability, and Web UI / Docs Site / Distribution. All 25 ADR-locked decisions and 23 of 24
routed requirements are implemented and merged to main. Phase 8 is a **deferred forward stub**
capturing the one known follow-up (Connect observe-lane auth hardening, R1–R4) so future
`/gsd-new-milestone` and `/gsd-plan-phase` runs have a clean anchor. Success criteria are stated
as observable truths that hold in the shipped baseline (or, for Phase 8, targets when undertaken).

## Milestones

- ✅ **v0.8.x Baseline** — Phases 1–7 (shipped)
- 📋 **Connect Auth Hardening** — Phase 8 (deferred, R1–R4)

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): milestone work
- Decimal phases (2.1, 2.2): urgent insertions (marked INSERTED)

- [x] **Phase 1: Authorization & Isolation** - Per-actor read isolation, write gating, opt-in sharing, configurable owner key
- [x] **Phase 2: Recall Semantics** - Summary-by-default, tag/temporal gating, windowed cursor paging, payload indexes
- [x] **Phase 3: Memory Kinds & Tools** - Discovery + rule kinds, schedule tools, short_id handle
- [x] **Phase 4: Embedder** - Protocol-named connection vars + asymmetric query/document param passthrough
- [x] **Phase 5: Config & Transport** - ENGRAM_ koanf config, Config.Validate, fatal legacy guard, explicit MCP path
- [x] **Phase 6: Telemetry & Observability** - slog + OTel over OTLP at every seam, never blocking startup
- [x] **Phase 7: Web UI, Docs Site & Distribution** - Operator console SPA, docs site, brand system, bundled client plugin
- [ ] **Phase 8: Connect Observe-Lane Auth Hardening** - DEFERRED — replace interim anonymous mount with cookie/OIDC (R1–R4)

## Phase Details

### Phase 1: Authorization & Isolation
**Goal**: A coding agent's memories are isolated per actor, so each agent's recall stays clean and only the owner can mutate its own records.
**Depends on**: Nothing (first phase)
**Requirements**: REQ-per-actor-isolation, REQ-typed-subject-authz, REQ-configurable-claim-owner
**Decisions**: DEC-cgb, DEC-g37x, DEC-kyz, DEC-xa6, DEC-12c
**Success Criteria** (what is TRUE):
  1. An authenticated actor sees and mutates only its own records; another actor's private records are invisible.
  2. `shared` records are readable by any authenticated caller but writable only by their owner (read/write asymmetry).
  3. Unauthorized id-addressed operations return the same not-found error as a missing id — no cross-actor existence leak.
  4. The record owner key is a configurable OIDC claim (default `email`) that survives IdP `sub` rotation, with `migrate-remap-owner` for re-stamping and pre-isolation backfill.
  5. Anonymous (auth-disabled) callers map to a single empty-owner bucket and cannot read other actors' `shared` records.
**Status**: Complete (v0.8.x)
**Plans**: N/A (retrospective — shipped before GSD planning)

### Phase 2: Recall Semantics
**Goal**: Recall returns relevant, current, token-efficient memories with precise filtering and deterministic paging.
**Depends on**: Phase 1
**Requirements**: REQ-scheduled-memories, REQ-windowed-cursor-recall, REQ-auto-summary
**Decisions**: DEC-ambu, DEC-4xt7, DEC-y1g, DEC-1frj, DEC-ef28
**Success Criteria** (what is TRUE):
  1. `search_memory`/`list_memory` return summary-shaped output by default, with `full=true` opting into full content; `get_memory` always returns full and carries `summary_source` provenance.
  2. An optional `tags` filter hard-ANDs (contains-all) as a Qdrant pre-filter applied before vector ranking.
  3. Temporal validity (`not_before`/`not_after`) gates recall as Qdrant filter conditions while `get_memory` and by-id paths stay ungated.
  4. Recall accepts half-open `created_after`/`created_before` windows and `list_memory` paginates via an opaque boundary cursor returning `{memories, next_cursor}`.
  5. `owner`/`scope`/`created_at` are Qdrant payload-indexed for exact server-side Count and range filtering (scanCap/approximate retired).
**Status**: Complete (v0.8.x)
**Plans**: N/A (retrospective)
**Note**: The `schedule_memory`/`list_scheduled` tool surface (DEC-90w) that sets these windows is presented in Phase 3.

### Phase 3: Memory Kinds & Tools
**Goal**: Beyond plain memories, agents capture discoveries and rules as distinct kinds and address any record by a stable short handle.
**Depends on**: Phase 1
**Requirements**: REQ-discovery-memory-type, REQ-rule-memory-kind, REQ-short-id-handle
**Decisions**: DEC-2bv, DEC-90w, DEC-iedk, DEC-zzq0, DEC-02ta
**Success Criteria** (what is TRUE):
  1. Discovery is a 5th category in the single Memory collection carrying `kind` (map|fact), aging-pinned citations, and a summary — recalled on demand via `store_discovery`/`search_discovery`, never at session start.
  2. Rules are a normative, always-shared kind surfaced at session start as a one-line progressive-disclosure index; `set_visibility` is rejected for rules.
  3. `schedule_memory`/`list_scheduled` expose temporal windows as dedicated tools without adding window params to `store_memory`.
  4. Every record carries a server-minted 10-char Crockford base32 `short_id`, accepted anywhere an id is accepted and resolved to the UUID at the handler layer via `Store.ResolvePointID`.
**Status**: Complete (v0.8.x)
**Plans**: N/A (retrospective)

### Phase 4: Embedder
**Goal**: Embedding works against OpenAI-compatible gateways and asymmetric/cloud models via protocol-named connection vars and query/document param passthrough.
**Depends on**: Nothing (foundational subsystem)
**Requirements**: REQ-asymmetric-embedder-params
**Decisions**: DEC-378, DEC-zyhq
**Success Criteria** (what is TRUE):
  1. Embedder connection vars are protocol-named (`ENGRAM_OPENAI_BASE_URL`/`ENGRAM_OPENAI_API_KEY`), naming the wire protocol not the vendor.
  2. Query vs document embedding params are provider-agnostic JSON maps merged into the `/v1/embeddings` body, supporting `input_type`/`task`/`task_type` passthrough and a document-side text instruction.
  3. Changing embedder params respects the reindex boundary — the `engram reindex` migration path exists for embedder changes.
**Status**: Complete (v0.8.x)
**Plans**: N/A (retrospective)

### Phase 5: Config & Transport
**Goal**: All configuration is unified under `ENGRAM_` via koanf with early validation and a fatal legacy guard; the MCP transport mounts at an explicit configurable path.
**Depends on**: Phase 4 (embedder vars fold into the unified registry)
**Requirements**: REQ-config-prefix-koanf, REQ-config-validation
**Decisions**: DEC-jgq, DEC-irq, DEC-bj6
**Success Criteria** (what is TRUE):
  1. All config loads via koanf from a single `ENGRAM_` field registry — no scattered getenv, no viper, `MEM_` prefix retired.
  2. Retired `MEM_*` vars trigger a fatal `config.CheckLegacy` startup guard with guidance — no silent fallback or dual-read shim.
  3. `Config.Validate()` runs early and loudly at the serve/reindex/migrate/prune entrypoints.
  4. The MCP StreamableHTTP transport mounts at an explicit configurable path (default `/mcp`); the console takes root when the UI is enabled.
**Status**: Complete (v0.8.x)
**Plans**: N/A (retrospective)

### Phase 6: Telemetry & Observability
**Goal**: The server is observable via structured logs plus OTel metrics/traces over OTLP, instrumented at every seam, without ever blocking startup.
**Depends on**: Phase 5
**Requirements**: REQ-observability-telemetry, REQ-telemetry-seams
**Decisions**: DEC-dwi, DEC-uxh
**Success Criteria** (what is TRUE):
  1. Structured `slog` logging plus OTel metrics and traces export exclusively over OTLP gRPC — no Prometheus `/metrics` scrape endpoint.
  2. Spans and domain-latency metrics instrument the store, embed, auth, HTTP, and MCP seams with a complete OTel resource.
  3. A telemetry setup failure or a missing OTLP endpoint yields no-op providers and never aborts server startup.
  4. The Helm chart exposes telemetry knobs (endpoint, toggles).
**Status**: Complete (v0.8.x)
**Plans**: N/A (retrospective)

### Phase 7: Web UI, Docs Site & Distribution
**Goal**: Operators observe the store through an authenticated web console, learn the product via a docs site under a unified brand, and consume engram as a bundled client plugin.
**Depends on**: Phase 2 (recall/Connect read API), Phase 5 (MCP/console routing)
**Requirements**: REQ-web-ui-console, REQ-operator-console-spa, REQ-operator-console-redesign, REQ-memory-display-ux, REQ-ui-test-unification, REQ-docs-site, REQ-docs-landing-redesign, REQ-brand-identity, REQ-relocate-memory-curator
**Decisions**: DEC-8xe, DEC-0lu, DEC-ttb
**Success Criteria** (what is TRUE):
  1. An authenticated read-only operator console (SvelteKit adapter-static SPA vendored via `go:embed`) reads the store over the Connect `EngramService` v1 read API, showing real `summary`/`summary_source` with safe markdown rendering.
  2. The console is a shadcn-svelte redesign on semantic theme tokens with svelte-query data flow (AppShell/ScopeRail/MemoryList/MemoryDetail/SearchPalette).
  3. UI and sanitizer tests run under vitest 4 browser mode (real Chromium via Playwright), retiring jsdom/happy-dom, with the CI test gate green.
  4. A static Astro Starlight docs site deploys to Cloudflare Workers with a redesigned landing hub, and the engram brand system (neural violet #6E56CF) is applied across console and docs.
  5. The memory-curator client plugin is relocated into the repo as the bundled `skill/engram` plugin with a marketplace entry and SessionStart/PostToolUse hooks.
**Status**: Complete (v0.8.x)
**Plans**: N/A (retrospective)
**UI hint**: yes

### Phase 8: Connect Observe-Lane Auth Hardening
**Goal**: Replace the interim anonymous Connect API mount with full cookie/OIDC observe-lane authentication so the read API serves real per-actor identities.
**Depends on**: Phase 7
**Requirements**: REQ-connect-auth-posture (DEFERRED — R1–R4)
**Decisions**: (none locked — forward work; interim disposition documented in the connect-auth-posture addendum)
**Success Criteria** (target, when undertaken):
  1. The Connect observe lane authenticates callers via cookie/OIDC instead of mounting anonymously into the single empty-owner bucket.
  2. Deferred requirements R1–R4 are satisfied before the observe lane is exposed to real identities.
  3. Store isolation applies to Connect-lane callers by their resolved owner, consistent with the MCP lane.
**Status**: Deferred — not started (future milestone)
**Plans**: TBD

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → (8 deferred)

| Phase | Milestone | Requirements | Status | Completed |
|-------|-----------|--------------|--------|-----------|
| 1. Authorization & Isolation | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| 2. Recall Semantics | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| 3. Memory Kinds & Tools | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| 4. Embedder | v0.8.x | 1/1 | Complete | shipped (v0.8.x) |
| 5. Config & Transport | v0.8.x | 2/2 | Complete | shipped (v0.8.x) |
| 6. Telemetry & Observability | v0.8.x | 2/2 | Complete | shipped (v0.8.x) |
| 7. Web UI, Docs Site & Distribution | v0.8.x | 9/9 | Complete | shipped (v0.8.x) |
| 8. Connect Auth Hardening | (future) | 0/1 | Deferred | - |
