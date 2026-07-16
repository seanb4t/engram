# Roadmap: engram

## Overview

This is a **retrospective / as-built roadmap** for v0.8.x/v0.9.x, extended by GSD-tracked
milestones going forward. Phases 1–7 group the already-completed v0.8.x work by synthesis area —
Authorization & Isolation, Recall Semantics, Memory Kinds & Tools, Embedder, Config & Transport,
Telemetry & Observability, and Web UI / Docs Site / Distribution. All 56 ADR-locked decisions (25
core + 31 companion refinements, folded 2026-07-08) and all 24 routed v0.8.x requirements are
implemented and merged to main. Per-phase implementation plans are cross-referenced in
`.planning/intel/merge-plans/context.md`. Phase 8 (Connect observe-lane auth hardening, R1–R4) was
**found already shipped** during a 2026-07-08 reconciliation — the cookie/OIDC lane landed
opportunistically inside PR #248 and was hardened in PR #266, before this retrospective baseline
was authored; the earlier "deferred stub" framing (ingested from the 2026-06-09 plan/spec, which
described the interim anonymous state as current) was stale. Phases 9–12 (v0.9.x — Recall
Quality) shipped 2026-07-10 (PR #336); full detail archived at `milestones/v0.9.x-ROADMAP.md`.
Success criteria are stated as observable truths that hold when each phase completes.

**Active milestone — v0.10.x — Hardening & Write Lane** (Phases 13–21, planned 2026-07-10): makes
engram production-solid and writable over Connect. Research (`.planning/research/SUMMARY.md`,
`ARCHITECTURE.md`, `PITFALLS.md`) converged on a dependency-ordered build sequence — embedder
reliability first (fully isolated), then the write-lane track in strict order (additive proto +
stub handlers → CSRF interceptor → deps.* refactor + wired handlers → session rotation → console
UX), with correctness/polish and CI hygiene running independently of both tracks. See Phase
Details below for the full per-phase rationale, decisions, and pitfall cross-references.

## Milestones

- ✅ **v0.8.x Baseline** — Phases 1–7 (shipped)
- ✅ **Connect Auth Hardening** — Phase 8 (shipped; R1–R4 verified 2026-07-08)
- ✅ **v0.9.x — Recall Quality** — Phases 9–12 (shipped 2026-07-10, PR #336): retrieval eval + ranking precision (#261), embedder query/document asymmetry (#305), async-on-write summaries (#320), per-memory usage signals (#317). Full detail archived at `milestones/v0.9.x-ROADMAP.md`.
- ✅ **v0.10.x — Hardening & Write Lane** — Phases 13–21 (shipped 2026-07-16): embedder reliability & options (#333/#332/#331/#334/#337, closes #261), Connect write lane + CSRF + stateless session rotation (#322/#323), correctness & polish tail, CI/maintenance hygiene. 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge only → #369). Full detail archived at `milestones/v0.10.x-ROADMAP.md`.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): milestone work
- Decimal phases (2.1, 2.2): urgent insertions (marked INSERTED)

<details>
<summary>✅ v0.8.x Baseline (Phases 1–7) — SHIPPED</summary>

- [x] **Phase 1: Authorization & Isolation** - Per-actor read isolation, write gating, opt-in sharing, configurable owner key
- [x] **Phase 2: Recall Semantics** - Summary-by-default, tag/temporal gating, windowed cursor paging, payload indexes
- [x] **Phase 3: Memory Kinds & Tools** - Discovery + rule kinds, schedule tools, short_id handle
- [x] **Phase 4: Embedder** - Protocol-named connection vars + asymmetric query/document param passthrough
- [x] **Phase 5: Config & Transport** - ENGRAM_ koanf config, Config.Validate, fatal legacy guard, explicit MCP path
- [x] **Phase 6: Telemetry & Observability** - slog + OTel over OTLP at every seam, never blocking startup
- [x] **Phase 7: Web UI, Docs Site & Distribution** - Operator console SPA, docs site, brand system, bundled client plugin

</details>

<details>
<summary>✅ Connect Auth Hardening (Phase 8) — SHIPPED (PR #248/#266)</summary>

- [x] **Phase 8: Connect Observe-Lane Auth Hardening** - Cookie/OIDC observe lane replaces the interim anonymous mount (R1–R4); shipped in PR #248/#266

</details>

<details>
<summary>✅ v0.9.x — Recall Quality (Phases 9–12) — SHIPPED 2026-07-10 (PR #336)</summary>

Full detail archived at `milestones/v0.9.x-ROADMAP.md`.

- [x] **Phase 9: Retrieval Eval Harness & Ranking Precision** - Labeled retrieval eval (recall@k/MRR), always-on similarity scores in `search_memory`, dependency-free reranker to kill phrasing-sensitivity — chosen by the eval numbers (completed 2026-07-10)
- [x] **Phase 10: Asymmetric Query/Document Embeddings** - Native API-param passthrough (cloud) + document-side prefix (E5/nomic) for query≠document embeds — found ALREADY SHIPPED under Phase 4 (verified 2026-07-10; #305 closed; no plans built)
- [x] **Phase 11: Async-on-Write Summaries** - In-process worker drains `FillSummary` after upsert, off the synchronous write path; eval-gated (completed 2026-07-10)
- [x] **Phase 12: Per-Memory Usage Signals** - Strong-signal counters (get/update) via hybrid OTLP + payload `access_count`; never affects ranking (completed 2026-07-10)

</details>

<details>
<summary>✅ v0.10.x — Hardening & Write Lane (Phases 13–21) — SHIPPED 2026-07-16</summary>

Full detail archived at `milestones/v0.10.x-ROADMAP.md`.

- [x] **Phase 13: Embedder Reliability Foundation** - Configurable HTTP timeout (re-derived backoff budget) + base-URL `/v1` join fix across every provider shape + embedder-config-identity payload stamp (completed 2026-07-11)
- [x] **Phase 14: Embedder Model Options & Eval** - Direct Gemini embeddings (eval-verified task_type behavior) + #261 prod-parity re-confirm on qwen3 + docs-site/Helm model recipes (completed 2026-07-11)
- [x] **Phase 15: Additive Proto + Stub Write Handlers** - Six new write RPCs (additive-only, buf-generated), CI lint gate against `idempotency_level`, safe `CodeUnimplemented` stubs (completed 2026-07-11)
- [x] **Phase 16: CSRF Interceptor** - Origin/Sec-Fetch-Site primary defense + session-bound double-submit token on every write RPC; read lane untouched (completed 2026-07-12)
- [x] **Phase 17: Wired Write Handlers (Full CRUD + Schedule)** - deps.* subject/actor refactor + all six write RPCs delegating to the shared MCP business-logic layer, MCP/Connect parity-tested (completed 2026-07-13)
- [x] **Phase 18: Stateless Session Rotation** - Sliding-expiry cookie re-seal on every authenticated request, new ADR for the no-revocation trade-off, no server-side state (completed 2026-07-13)
- [x] **Phase 19: Console Write UX** - Create/edit/delete/re-share/schedule from the operator console over the write lane, with CSRF + a silent opportunistic auth-race retry (completed 2026-07-15; live browser E2E UAT deferred → #366)
- [x] **Phase 20: Correctness & Polish** - Discovery proto fidelity, MintShortID collision cap, embed param-key/body-build cleanup, discovery short_id schema, summarize-missing CronJob (completed 2026-07-16)
- [x] **Phase 21: CI / Maintenance Hygiene** - Renovate vendored-SPA self-heal, Phase-11 review residuals, `.rumdl.toml` `.planning` exclude (completed 2026-07-16; #301 live self-heal observation deferred, post-merge only → #369)

</details>

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
**Requirements**: REQ-connect-auth-posture (R1–R4 — satisfied)
**Decisions**: interim disposition + acceptance criteria R1–R4 (+R1a) documented in the connect-auth-posture addendum
**Success Criteria** (what is TRUE):

1. The Connect observe lane authenticates callers via cookie/OIDC (sealed AES-GCM session → verified `sub`) instead of mounting anonymously; when the UI is disabled (headless default) the Connect handler is not mounted at all (R1: `mountConnect` returns nil-not-mounted for a nil resolver).
2. R1–R4 are satisfied: mount-gating (R1), cookie→Subject as the sole authz entry with no anonymous fallthrough (R2), observability parity via `otelconnect` + access-log interceptors (R3), and same-origin posture with no permissive CORS (R4).
3. Store isolation applies to Connect-lane callers by their resolved owner, consistent with the MCP lane (verified: `TestConnectCrossActorIsolation`, `TestConnectCookieLaneIsolation`).

**Status**: Complete — shipped in PR #248 (webauth lane) + PR #266 (owner-claim hardening); R1–R4 verified green on main 2026-07-08. Config folded into the Phase-5 `ENGRAM_UI_*` koanf registry (not the plan's original `MEM_*`).
**Plans**: N/A (shipped before GSD planning). Reference implementation plan: `docs/superpowers/plans/2026-06-09-engram-web-ui-cookie-oidc-auth-lane.md`; design/acceptance criteria: `docs/superpowers/specs/2026-06-09-connect-auth-posture-addendum.md`. Out-of-scope follow-ups remain (Connect write-lane RPCs + CSRF hardening; session refresh-token rotation) — candidates for a future milestone.

### Phases 9–12 (v0.9.x — Recall Quality) — ✅ SHIPPED 2026-07-10

Full phase details (goals, success criteria, plans, decisions, tech debt) are archived at
[`milestones/v0.9.x-ROADMAP.md`](milestones/v0.9.x-ROADMAP.md). Requirements outcomes at
[`milestones/v0.9.x-REQUIREMENTS.md`](milestones/v0.9.x-REQUIREMENTS.md). Audit (PASSED) at
[`milestones/v0.9.x-MILESTONE-AUDIT.md`](milestones/v0.9.x-MILESTONE-AUDIT.md).

- Phase 9 — Retrieval Eval Harness & Ranking Precision (3 plans): eval harness + always-on similarity score + dependency-free reranker (#261; recall@8=1.00)
- Phase 10 — Asymmetric Query/Document Embeddings: already shipped under Phase 4 (#305 closed; no plans)
- Phase 11 — Async-on-Write Summaries (3 plans): bounded worker pool off the write path (#320)
- Phase 12 — Per-Memory Usage Signals (6 plans): get/update counters, hybrid OTLP+payload, never affects ranking (#317)

### Phases 13–21 (v0.10.x — Hardening & Write Lane) — ✅ SHIPPED 2026-07-16

Full phase details (goals, success criteria, plans, decisions, tech debt) are archived at
[`milestones/v0.10.x-ROADMAP.md`](milestones/v0.10.x-ROADMAP.md). Requirements outcomes at
[`milestones/v0.10.x-REQUIREMENTS.md`](milestones/v0.10.x-REQUIREMENTS.md). Audit (tech_debt — 19/20 requirements, 1 deferred) at
[`milestones/v0.10.x-MILESTONE-AUDIT.md`](milestones/v0.10.x-MILESTONE-AUDIT.md).

- Phase 13 — Embedder Reliability Foundation (3 plans): configurable timeout + base-URL join fix + embedder-config-identity stamp (#333/#332)
- Phase 14 — Embedder Model Options & Eval (3 plans): direct Gemini + prod-parity re-confirm + model recipes (#331/#334/#337, closes #261)
- Phase 15 — Additive Proto + Stub Write Handlers (4 plans): 6 additive write RPCs, idempotency-annotation CI gate (#322)
- Phase 16 — CSRF Interceptor (3 plans): Origin/Sec-Fetch-Site + session-bound double-submit token (#322)
- Phase 17 — Wired Write Handlers (6 plans): deps.* refactor, MCP↔Connect authz parity (#322)
- Phase 18 — Stateless Session Rotation (3 plans): sliding-expiry cookie re-seal, no server state (#323)
- Phase 19 — Console Write UX (6 plans): create/edit/delete/re-share/schedule over the write lane, CSRF + auth-race retry (live browser E2E UAT deferred → #366)
- Phase 20 — Correctness & Polish (4 plans): discovery proto fidelity, MintShortID cap, embed cleanups, summarize CronJob (#307/#308/#304/#302/#303/#269)
- Phase 21 — CI / Maintenance Hygiene (3 plans): rumdl `.planning` exclude, phase-11 residuals (#335), Renovate self-heal (#301 — live observation deferred, post-merge only → #369)


## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (v0.8.x, shipped) · 9 → 10 → 11 → 12 (v0.9.x, shipped 2026-07-10) · 13 → 14 (embedder track) · 15 → 16 → 17 → 18 → 19 (write-lane track, strict order) · 20 → 21 (independent) — v0.10.x shipped 2026-07-16

| Phase | Milestone | Requirements | Status | Completed |
|-------|-----------|--------------|--------|-----------|
| 1. Authorization & Isolation | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| 2. Recall Semantics | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| 3. Memory Kinds & Tools | v0.8.x | 3/3 | Complete | shipped (v0.8.x) |
| 4. Embedder | v0.8.x | 1/1 | Complete | shipped (v0.8.x) |
| 5. Config & Transport | v0.8.x | 2/2 | Complete | shipped (v0.8.x) |
| 6. Telemetry & Observability | v0.8.x | 2/2 | Complete | shipped (v0.8.x) |
| 7. Web UI, Docs Site & Distribution | v0.8.x | 9/9 | Complete | shipped (v0.8.x) |
| 8. Connect Auth Hardening | v0.8.x | 1/1 | Complete | shipped (PR #248/#266) |
| 9. Retrieval Eval & Ranking Precision | v0.9.x | 3/3 | Complete | 2026-07-10 (PR #336) |
| 10. Asymmetric Query/Document Embeddings | v0.9.x | 1/1 | Complete (already shipped) | 2026-07-10 (#305) |
| 11. Async-on-Write Summaries | v0.9.x | 1/1 | Complete | 2026-07-10 (PR #336) |
| 12. Per-Memory Usage Signals | v0.9.x | 1/1 | Complete | 2026-07-10 (PR #336) |
| 13. Embedder Reliability Foundation | v0.10.x | 3/3 | Complete    | 2026-07-11 |
| 14. Embedder Model Options & Eval | v0.10.x | 3/3 | Complete    | 2026-07-11 |
| 15. Additive Proto + Stub Write Handlers | v0.10.x | 4/4 | Complete    | 2026-07-11 |
| 16. CSRF Interceptor | v0.10.x | 3/3 | Complete    | 2026-07-12 |
| 17. Wired Write Handlers (Full CRUD + Schedule) | v0.10.x | 6/6 | Complete    | 2026-07-13 |
| 18. Stateless Session Rotation | v0.10.x | 3/3 | Complete    | 2026-07-13 |
| 19. Console Write UX | v0.10.x | 6/6 | Complete   | 2026-07-15 |
| 20. Correctness & Polish | v0.10.x | 4/4 | Complete    | 2026-07-16 |
| 21. CI / Maintenance Hygiene | v0.10.x | 3/3 | Complete   | 2026-07-16 |

**v0.9.x — Recall Quality: ✅ shipped 2026-07-10 (PR #336) · 6/6 requirements · audit PASSED.**
**v0.10.x — Hardening & Write Lane: ✅ shipped 2026-07-16 · 9 phases (13–21) · 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge → #369) · audit tech_debt (9/9 Nyquist, 0 blockers).** Full detail: `milestones/v0.10.x-ROADMAP.md`.
