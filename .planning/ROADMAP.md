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
Phases 13–21 (v0.10.x — Hardening & Write Lane) shipped 2026-07-16; full detail archived at
`milestones/v0.10.x-ROADMAP.md`. Success criteria are stated as observable truths that hold when
each phase completes.

**Active milestone — v0.11.x — Capture & Service Identity** (Phases 22–26, planned 2026-07-16):
makes programmatic capture correct/re-runnable and gives headless service principals a
first-class, isolated identity. Research (`.planning/research/{SUMMARY,ARCHITECTURE,CEDAR}.md`)
converged on a dependency-ordered build sequence — the Cedar authz foundation must land first (it
is the trust anchor every other phase's isolation depends on, and is a behavior-preserving
refactor of `internal/store`'s existing filter/gate functions, refining LOCKED `DEC-cgb` via a new
ADR rather than overriding it), then service-auth-chain + tenancy isolation (where the milestone's
#1 risk — a service principal silently resolving to `owner==""` — must be proven fail-closed as
the first test), then the capture trio in strict internal order (idempotency → supersession →
citations, since supersession reuses idempotency's re-Upsert mechanism), with category filter and
the chat/summarize base-URL split bundled as a low-risk, independent tail. Zero new store-layer
authz **primitive** and (except `cedar-go`) zero new dependencies are required — every feature
extends an existing seam. See Phase Details below for full per-phase rationale and decisions.

## Milestones

- ✅ **v0.8.x Baseline** — Phases 1–7 (shipped)
- ✅ **Connect Auth Hardening** — Phase 8 (shipped; R1–R4 verified 2026-07-08)
- ✅ **v0.9.x — Recall Quality** — Phases 9–12 (shipped 2026-07-10, PR #336): retrieval eval + ranking precision (#261), embedder query/document asymmetry (#305), async-on-write summaries (#320), per-memory usage signals (#317). Full detail archived at `milestones/v0.9.x-ROADMAP.md`.
- ✅ **v0.10.x — Hardening & Write Lane** — Phases 13–21 (shipped 2026-07-16): embedder reliability & options (#333/#332/#331/#334/#337, closes #261), Connect write lane + CSRF + stateless session rotation (#322/#323), correctness & polish tail, CI/maintenance hygiene. 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge only → #369). Full detail archived at `milestones/v0.10.x-ROADMAP.md`.
- 📋 **v0.11.x — Capture & Service Identity** — Phases 22–26 (planned 2026-07-16): Cedar authz foundation (#362/#373 trust anchor), service auth chain + tenancy isolation (#362/#373), idempotent capture (#340), supersession with history (#342), structured citations + category filter + chat base URL (#341/#374/#350).

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

### 📋 v0.11.x — Capture & Service Identity (Planned)

**Milestone Goal:** Make programmatic capture correct and re-runnable, and give headless service
principals a first-class, isolated identity — so agents can write memory mechanically and safely
into shared stores.

- [x] **Phase 22: Cedar Authz Foundation & Store Enforcement** - Cedar (cedar-go v1.8.0) PDP decides authorization over enumerable buckets; `internal/store` compiles decisions into the Qdrant filter — behavior-preserving refinement of DEC-cgb (completed 2026-07-17)
- [x] **Phase 23: Service Auth Chain & Tenancy Isolation** - Pluggable verifier chain (OIDC user → OIDC client-credentials → static token); a service principal never resolves to the anonymous bucket (completed 2026-07-17)
- [x] **Phase 24: Idempotent Capture** - `store_memory` accepts an idempotency key with strict, owner-scoped, race-safe replay-safety (completed 2026-07-18)
- [x] **Phase 25: Supersession with History** - A memory can supersede another via additive links; superseded records are soft-hidden from recall but stay fetchable by id (completed 2026-07-19)
- [ ] **Phase 26: Structured Citations, Category Filter & Chat Base URL** - Optional provenance on curated memories, MCP↔Connect category-filter parity, and a distinct chat/summarize base URL

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

### Phase 22: Cedar Authz Foundation & Store Enforcement

**Goal**: engram gains a real ABAC policy engine (`internal/authz`, cedar-go v1.8.0) that decides
authorization over a small enumerable set of buckets, and `internal/store` becomes the sole place
that decision is compiled into a Qdrant filter or gate check — a behavior-preserving refinement of
`DEC-cgb`, not a new authz primitive, and the trust anchor every later phase's isolation depends on.
**Depends on**: Nothing (first phase of v0.11.x)
**Requirements**: REQ-cedar-pdp-foundation, REQ-cedar-store-enforcement
**Decisions**: refines DEC-cgb (new ADR, working id DEC-cdr1 — "Cedar PDP decides the predicate;
the store enforces it as the Qdrant filter"); reaffirms DEC-xa6 (uniform not-found), DEC-kyz
(sharing grants read, never write), DEC-12c (`Subject` stays a sealed 2-variant sum — Cedar's
`Principal` is a thin entity wrapper over the existing owner string, not a parallel identity system)
**Success Criteria** (what must be TRUE):

1. Every existing recall/authz behavior is unchanged for human callers — the full pre-existing
   isolation/sharing test suite passes byte-for-byte after the refactor (behavior-preserving,
   per cedar-go's default-deny-on-error semantics matching `DEC-12c`'s discipline).

2. A `go:embed`-compiled default policy corpus (own-record read/write, shared-read, tenant-isolate,
   plus a defense-in-depth `forbid ... unless principal.owner != ""` policy) is provably correct via
   permanent regression tests evaluated against the policy text itself — own-record allow,
   shared-read allow (never write/delete/share/schedule), cross-owner write deny, empty-owner deny.

3. `internal/store`'s bulk-recall filter-builder (Search/List) and its id-addressed gate functions
   (`getWritable`/`GetReadable`/`OwnedOrAbsent`) consult `internal/authz.Decide` for the
   authorization decision over enumerable buckets (own/shared/tenant) and translate it into the
   same Qdrant filter/gate shape they build today — Cedar decides, the store still enforces; no
   per-record Cedar evaluation on the hot recall path.

4. A Cedar `Deny` on a get/update/delete/share/schedule target maps to the exact same not-found
   error already used for a genuinely missing id (`DEC-xa6` preserved); `internal/authz`'s
   `Diagnostic` never leaks into a caller-facing error.

5. The `Principal`/`Memory` entity schema reserves `tenant`/`roles` attributes and hierarchy fields
   (`memberOfTypes`/`parents`) as present-but-unpopulated, so a later full tenant/group/role ABAC
   milestone can extend it with no breaking schema change.

**Note**: research flags the OIDC client-credentials owner-claim source (`client_id` vs `azp` vs a
custom claim) and the `shared`-visibility cross-tenant policy question as open items for Phase 23,
not this phase — this phase's policies encode exactly today's rules (own + global shared-read).
**Plans**: 3/3 plans executed

Plans:
**Wave 1**

- [x] 22-01-PLAN.md — internal/authz PDP foundation: cedar-go v1.8.0, four embedded policies, DecideBucket/DecideRecord API, D-08 policy-corpus regression suite (REQ-cedar-pdp-foundation)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 22-02-PLAN.md — store bulk-recall wiring: inject PDP, PDP-back ownerOrSharedCondition/ownerOnlyCondition via DecideBucket, behavior-preserving + per-bucket call-count tests (REQ-cedar-store-enforcement)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 22-03-PLAN.md — store id-addressed gate wiring (GetReadable/getWritable/OwnedOrAbsent via DecideRecord, Deny→ErrNotFound, no Diagnostic leak) + DEC-cdr1 ADR (REQ-cedar-store-enforcement)

### Phase 23: Service Auth Chain & Tenancy Isolation

**Goal**: Headless service principals — OIDC client-credentials or operator-provisioned static
tokens — authenticate through a pluggable, config-selectable verifier chain and are isolated to
their own owner bucket by default, never the anonymous bucket and never colliding with a human
owner. The milestone's #1 risk (a service principal silently resolving to `owner==""`) is proven
fail-closed as the first test of this phase.
**Depends on**: Phase 22 (Cedar foundation — the bucket-decision seam this phase's principals flow through)
**Requirements**: REQ-service-auth-chain, REQ-static-token-auth, REQ-service-owner-failclosed, REQ-service-principal-isolation
**Decisions**: reuses DEC-g37x's `namespacedOwner` injective encoding (no new owner-encoding
scheme); reaffirms DEC-12c's anti-pattern guard — no 3rd `Subject` variant for service principals
(a service principal resolves to the existing `authenticated{sub}` variant with a namespaced owner,
exactly like any other non-email claim already does)
**Success Criteria** (what must be TRUE):

1. A caller authenticates through a defined-order chain — OIDC user token, then OIDC
   client-credentials, then static provisioned token — and each mechanism resolves to the same
   `TokenInfo{Extra[owner]}`/`Subject` contract the store already gates on; which mechanisms are
   enabled is config-selectable via ENGRAM_ koanf.

2. An authenticated service principal (client-credentials or static token) whose configured owner
   claim cannot be resolved is REJECTED with an explicit fail-closed error, never silently mapped
   to the anonymous empty-owner bucket — this is the FIRST test proven in this phase, before any
   other service-auth behavior is considered done.

3. An operator can provision a static bearer token mapped to exactly one owner via ENGRAM_ config;
   the token is verified with a constant-time compare (`crypto/subtle`) and never appears in logs,
   spans, or error messages.

4. A headless service principal is isolated to its own owner bucket by default — it cannot read
   another human's or another service principal's private records, and does not collide with the
   anonymous bucket or a human owner.

5. The `shared`-visibility cross-tenant question (does a service principal's global `shared`-read
   grant cross tenant boundaries, or is it scoped) is resolved as an explicit, written, tested
   policy decision — not left implicit.

**Note**: this phase's tenancy isolation is largely a *verification* phase once the auth chain
exists — it proves namespaced-owner isolation against the store filters Phase 22 wired, per the
research-converged build order (`#362` requires `#373` to ship together or first).
**Plans**: 6/6 plans executed

Plans:
**Wave 1** *(parallel — zero file overlap; internal/auth is split across three files)*

- [x] 23-01-PLAN.md — internal/auth OIDC service lane: FIRST fail-closed empty-owner reject (SC2/D-08/D-10) + NewService per-lane audience (D-14) (REQ-service-owner-failclosed, REQ-service-auth-chain)
- [x] 23-02-PLAN.md — internal/auth static-token verifier: crypto/subtle constant-time compare, per-owner map, no-leak discipline (REQ-static-token-auth)
- [x] 23-03-PLAN.md — internal/auth chainVerifier combinator + JWT-shape discriminator: D-02 order, D-03 nil-guard, D-04 deny-by-default (REQ-service-auth-chain)
- [x] 23-04-PLAN.md — internal/config service_auth.* rows + ServiceAuthConfig + token→owner map parser + validation (REQ-service-auth-chain, REQ-static-token-auth)
- [x] 23-05-PLAN.md — internal/store service-principal isolation + permanent cross-tenant shared-read decision test (SC4/SC5/D-07/D-15/D-16) (REQ-service-principal-isolation)

**Wave 2** *(depends on 23-01..23-04)*

- [x] 23-06-PLAN.md — cmd/engram/serve.go withAuth chain wiring (the ONE call site) + chain-parity integration test + docs (config guide, auth reference, cross-tenant shared-read ADR) (REQ-service-auth-chain, REQ-service-principal-isolation)

### Phase 24: Idempotent Capture

**Goal**: `store_memory` is safely re-runnable — a repeat call with the same idempotency key and
owner returns the original record unchanged, a repeat with the same key but different content is
explicitly rejected, and concurrent retries never produce duplicate Qdrant records.
**Depends on**: Nothing (orthogonal to the auth/tenancy track; can run in parallel with Phases 22–23)
**Requirements**: REQ-idempotent-capture
**Decisions**: none yet — the same-key/different-content contract (reject vs. explicit upsert) is
locked as REQUIREMENTS.md's decision to reject; a deterministic point-ID derivation
(`uuid.NewSHA1` over `(owner, scope, key)`) replaces `uuid.NewString()` only when a key is supplied
**Success Criteria** (what must be TRUE):

1. A `store_memory` call with an idempotency key, repeated with identical content, returns the
   original record/result unchanged — no duplicate is created.

2. A repeat call with the same key but different content is rejected with an explicit, distinct
   mismatch error rather than silently overwriting or duplicating.

3. An idempotency key is scoped per-owner — two different owners can use the identical key value
   without colliding.

4. Concurrent repeat calls with the same key resolve to exactly one record in Qdrant (race-safe via
   deterministic-ID upsert, not a search-then-insert check).

5. Omitting the idempotency key preserves today's behavior exactly — a fresh record is created
   every time.

**Plans**: 2/2 plans executed

**Wave 1**

- [x] 24-01-PLAN.md — internal/store fingerprint payload field + ErrIdempotencyConflict sentinel + connectError row; internal/server idempotencyPointID + contentFingerprint pure helpers (D-02/D-03/D-04/D-06/D-10) (REQ-idempotent-capture)

**Wave 2** *(depends on 24-01)*

- [x] 24-02-PLAN.md — storeArgs.idempotency_key + tool descriptions + checkIdempotentReplay check-before-embed branch in store_memory/schedule_memory; SC1–SC5 tests incl. two-owner matrix + `-race` concurrency (D-08/D-09/D-11/D-12/D-13) (REQ-idempotent-capture)

### Phase 25: Supersession with History

**Goal**: A memory can supersede another via additive `supersedes`/`superseded_by` links —
correction is explicit and preserves history, never deleting or silently overwriting. Superseded
records are soft-hidden from recall but remain fetchable by id.
**Depends on**: Phase 24 (reuses idempotency's payload-only re-Upsert mechanism to stamp the
superseded record)
**Requirements**: REQ-supersession-links
**Decisions**: reuses DEC-y1g/DEC-ufz's soft-hide-at-recall-gate pattern (Search/List exclude,
`get_memory` stays ungated); follows the DEC-90w precedent of a dedicated tool/verb over
overloading `store_memory`; routes through the existing DEC-kyz write gate (`getWritable`/
`OwnedOrAbsent`), never a read grant
**Success Criteria** (what must be TRUE):

1. Storing a memory with a `supersedes` link to an existing record stamps that record's
   `superseded_by` link back, without deleting or overwriting its content.

2. A superseded record no longer appears in `search_memory`/`list_memory` results (soft-hidden at
   the recall gate) but remains fully fetchable via `get_memory`.

3. Superseding an existing record is only possible through the ownership write gate — a caller with
   only read/shared access cannot supersede another owner's record (no DEC-xa6/DEC-kyz regression).

4. Correction is explicit — no automatic superseding happens on any similarity threshold or
   write-through path; a single-hop supersession model rejects cycles at write time.

**Plans**: 2/2 plans complete
**Wave 1**

- [x] 25-01-PLAN.md — Store-layer supersede primitive: Memory link fields + codec, Store.Supersede (owner-gated, single-hop, vector-preserving back-stamp), recall-gate soft-hide, ErrAlreadySuperseded (Wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 25-02-PLAN.md — supersede_memory MCP tool: supersedeArgs + handler + registration, connectError exhaustiveness (Wave 2, depends on 25-01)

### Phase 26: Structured Citations, Category Filter & Chat Base URL

**Goal**: Three small, independent extensions of existing seams close out the milestone — curated
memories can optionally carry structured provenance, `search_memory`/`list_memory` gain
MCP↔Connect category-filter parity, and the chat/summarize client can target a base URL distinct
from the embedder's.
**Depends on**: Phase 25 (citations continues the capture trio's additive-payload pattern; category
filter and the chat base-URL split are independent additions bundled here for pacing, not blocked
by or blocking the capture/auth spine)
**Requirements**: REQ-memory-citations, REQ-category-filter, REQ-chat-base-url
**Decisions**: relaxes the discovery-only `payload()` citations write-gate to any category (`Kind`
stays discovery-specific) without violating DEC-2bv (single collection, no new authz surface);
mirrors DEC-4xt7's tag-filter hard-AND-pre-filter pattern for category filtering
**Success Criteria** (what must be TRUE):

1. A `memory`-category record can optionally be stored with structured citations (the existing
   discovery `Citation` shape, reused verbatim) — citations are never auto-populated.

2. `search_memory` and `list_memory` accept an optional `category` filter over the MCP surface,
   composing as a hard Qdrant pre-filter alongside the existing owner/scope/tags filters (applied
   before vector ranking on search), at parity with Connect's `ListMemories` (which already
   supports it).

3. An operator can configure `ENGRAM_OPENAI_CHAT_BASE_URL` distinct from the embedder's base URL;
   when unset, the summarizer falls back to the shared `ENGRAM_OPENAI_BASE_URL` with zero
   configuration-change behavior impact.

4. None of the three additions introduces new store-layer authz surface — citations and category
   filtering compose onto the existing authz-outer-`Must` invariant untouched.

**Plans**: 5/6 plans executed

**Wave 1**

- [x] 26-01-PLAN.md — Track B store layer: `store.SearchOptions` + `categoryMatchCondition`, `Search`/`SearchReranked` reshape across ~25 call sites, `coreSearchRequest.Categories`, OR/pre-ranking/SC4 store tests (Wave 1)

**Wave 2** *(blocked on Wave 1)*

- [x] 26-02-PLAN.md — Track B MCP surface: `categories` argument on `search_memory`/`list_memory` with explicit OR wording, closure wiring, empty/unknown/ordering edge tests (Wave 2, depends on 26-01)

**Wave 3** *(blocked on Wave 2; the two plans below are independent of each other)*

- [x] 26-03-PLAN.md — Track B Connect parity: `SearchMemoriesRequest.categories = 8` (one-way, checkpoint-gated), `task proto:gen` + committed `gen/`, handler wiring, MCP↔Connect parity test (Wave 3, depends on 26-02)
- [x] 26-04-PLAN.md — Track C chat base URL: `internal/openaiurl.Join` shared provider-shape join, `ENGRAM_OPENAI_CHAT_BASE_URL` config/registry/validate trio, `cmp.Or` at the summarizer construction site, Helm wiring (Wave 3, file-ownership serialization only)

**Wave 4** *(blocked on Wave 3)*

- [x] 26-05-PLAN.md — Track A citations: `payload()` gate split, `storeArgs.Citations` + the `toMemory` mapping, shared `validateCitations`, Connect compact-view fix, cross-write-path preservation suite (Wave 4, depends on 26-03 + 26-04)

**Wave 5** *(blocked on Wave 4)*

- [ ] 26-06-PLAN.md — Docs + skill: `curating-memory` citations guidance, memory-record/tools reference updates, `ENGRAM_OPENAI_CHAT_BASE_URL` configure guide (Wave 5, depends on 26-03 + 26-04 + 26-05)

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (v0.8.x, shipped) · 9 → 10 → 11 → 12 (v0.9.x, shipped 2026-07-10) · 13 → 14 (embedder track) · 15 → 16 → 17 → 18 → 19 (write-lane track, strict order) · 20 → 21 (independent) — v0.10.x shipped 2026-07-16 · 22 → 23 (Cedar foundation → service auth/tenancy, strict order) · 24 → 25 → 26 (capture trio + recall/config tail, strict order; 24 can start in parallel with 22–23) — v0.11.x planned 2026-07-16

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
| 22. Cedar Authz Foundation & Store Enforcement | v0.11.x | 2/2 | Complete | 2026-07-17 |
| 23. Service Auth Chain & Tenancy Isolation | v0.11.x | 4/4 | Complete | 2026-07-17 |
| 24. Idempotent Capture | v0.11.x | 2/2 | Complete | 2026-07-18 |
| 25. Supersession with History | v0.11.x | 0/1 | Complete   | 2026-07-19 |
| 26. Structured Citations, Category Filter & Chat Base URL | v0.11.x | 0/3 | In Progress|  |

**v0.9.x — Recall Quality: ✅ shipped 2026-07-10 (PR #336) · 6/6 requirements · audit PASSED.**
**v0.10.x — Hardening & Write Lane: ✅ shipped 2026-07-16 · 9 phases (13–21) · 19/20 requirements (REQ-ci-renovate-spa-drift's live self-heal observation deferred, post-merge → #369) · audit tech_debt (9/9 Nyquist, 0 blockers).** Full detail: `milestones/v0.10.x-ROADMAP.md`.
**v0.11.x — Capture & Service Identity: 📋 planned 2026-07-16 · 5 phases (22–26) · 11/11 requirements mapped, 0 unmapped.** Research: `.planning/research/{SUMMARY,ARCHITECTURE,CEDAR}.md`.
