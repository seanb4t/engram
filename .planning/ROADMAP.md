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
- 🚧 **v0.10.x — Hardening & Write Lane** — Phases 13–21 (planned 2026-07-10): embedder reliability & options (#333/#332/#331/#334/#337, closes #261), Connect write lane + CSRF + stateless session rotation (#322/#323), correctness & polish tail, CI/maintenance hygiene. See `.planning/REQUIREMENTS.md` and `.planning/research/SUMMARY.md`.

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

<details open>
<summary>🚧 v0.10.x — Hardening & Write Lane (Phases 13–21) — IN PROGRESS</summary>

- [x] **Phase 13: Embedder Reliability Foundation** - Configurable HTTP timeout (re-derived backoff budget) + base-URL `/v1` join fix across every provider shape + embedder-config-identity payload stamp (completed 2026-07-11)
- [x] **Phase 14: Embedder Model Options & Eval** - Direct Gemini embeddings (eval-verified task_type behavior) + #261 prod-parity re-confirm on qwen3 + docs-site/Helm model recipes (completed 2026-07-11)
- [x] **Phase 15: Additive Proto + Stub Write Handlers** - Six new write RPCs (additive-only, buf-generated), CI lint gate against `idempotency_level`, safe `CodeUnimplemented` stubs (completed 2026-07-11)
- [x] **Phase 16: CSRF Interceptor** - Origin/Sec-Fetch-Site primary defense + session-bound double-submit token on every write RPC; read lane untouched (completed 2026-07-12)
- [x] **Phase 17: Wired Write Handlers (Full CRUD + Schedule)** - deps.* subject/actor refactor + all six write RPCs delegating to the shared MCP business-logic layer, MCP/Connect parity-tested (completed 2026-07-13)
- [ ] **Phase 18: Stateless Session Rotation** - Sliding-expiry cookie re-seal on every authenticated request, new ADR for the no-revocation trade-off, no server-side state
- [ ] **Phase 19: Console Write UX** - Create/edit/delete/re-share/schedule from the operator console over the write lane, with CSRF + silent re-seal retry
- [ ] **Phase 20: Correctness & Polish** - Discovery proto fidelity, MintShortID collision cap, embed param-key/body-build cleanup, discovery short_id schema, summarize-missing CronJob
- [ ] **Phase 21: CI / Maintenance Hygiene** - Renovate vendored-SPA drift fix, Phase-11 review residuals, `.rumdl.toml` `.planning` exclude

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

### Phase 13: Embedder Reliability Foundation

**Goal**: The embedder client survives provider brownouts and joins base URLs correctly across every documented provider shape, with each record traceable to the embedder config that produced it.
**Depends on**: Nothing (fully isolated — zero import-graph overlap with the write-lane track; ships first as low-risk throughput)
**Requirements**: REQ-embed-timeout, REQ-embed-baseurl-join, REQ-embed-config-identity
**Success Criteria** (what is TRUE):

1. Operators can set `ENGRAM_EMBED_TIMEOUT` (koanf, validated) to override the previously-hardcoded 30s HTTP client timeout, and the async summary-queue's backoff budget is re-derived from the new value — no stale `30 * time.Second` literal survives in `summaryqueue.go`.
2. `ENGRAM_OPENAI_BASE_URL` values shaped like OpenRouter (trailing `/v1`), OpenAI (no trailing `/v1`), a trailing-slash variant, and the Gemini `/v1beta/openai` shape all resolve to the correct embeddings path — proven by a provider-shape table test, not just the one reported OpenRouter case.
3. Every newly stored record carries an embedder-config-identity hash (model + dim + relevant params) in its payload, so a future reindex-boundary audit has the data it needs to detect mixed-embedding-space records.
4. A slow or unreachable embedder fails within the configured timeout instead of hanging the calling MCP tool call (`store_memory`/`update_memory`/`search_memory`) indefinitely.

**Status**: Not started
**Plans**: 3/3 plans complete
**Wave 1**

- [x] 13-01-PLAN.md — Embed client hardening: ENGRAM_EMBED_TIMEOUT + shape-aware base-URL join/override (SC1/SC2/SC4)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 13-02-PLAN.md — Embedder-config-identity helper + payload codec + clean write-site stamping (SC3, 4 sites + rule)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 13-03-PLAN.md — Reindex identity stamping: verbatim-payload landmine + StoreAndEmbedderFromEnvNoEnsure signature ripple (SC3, 5th site)

### Phase 14: Embedder Model Options & Eval

**Goal**: Operators can point engram at Gemini's embeddings API and trust that the documented model recipes (OpenRouter/Gemini/OpenAI/local) actually deliver working asymmetric query/document embeddings, with the last v0.9.x eval follow-up closed.
**Depends on**: Phase 13 (the timeout knob makes eval runs reliable)
**Requirements**: REQ-embed-gemini-direct, REQ-embed-prod-parity-eval, REQ-embed-model-docs
**Success Criteria** (what is TRUE):

1. engram can embed queries and documents against Google's Gemini embeddings API with the wire shape verified against live docs, and `task_type`/dimension behavior is confirmed — not assumed — by a `task eval:retrieval` run asserting the shipped Gemini config's query and document vectors actually differ (silent `task_type` no-op is a recall regression with no error to catch it, per PITFALLS.md Pitfall 12 — this is a correctness gate, not a docs note).
2. The #261 regression fixture re-confirms recall@8 parity on the prod-parity `qwen3-embedding-8b`@4096 config with `ENGRAM_EMBED_QUERY_INSTRUCTION`, closing GitHub #261 and #334 for good.
3. The docs site and Helm `values.yaml` document each supported embedding model (OpenRouter/Gemini/OpenAI/local TEI-Ollama-vLLM), pairing base URL + model + vector dim + query instruction, with every model/dim change explicitly called out as requiring `engram reindex` (cross-linking `guides/reindex`).

**Status**: Not started
**Plans**: 3/3 plans complete

**Wave 1** *(parallel — no file overlap)*

- [x] 14-01-PLAN.md — Gemini differ-case eval fixture: skip-gated `TestRetrievalEval_AsymmetryDiffer` (named so `task eval:retrieval` reaches it) proving instruction-prefix asymmetry takes effect (Pitfall-12 gate, REQ-embed-gemini-direct)
- [x] 14-02-PLAN.md — Model & recipe docs: new `guides/embedding-models.md` + Helm `values.yaml` commented recipes + cross-links/reindex callouts (REQ-embed-model-docs)

**Wave 2** *(blocked on Wave 1)*

- [x] 14-03-PLAN.md — Model-id lock checkpoint + live eval runs (Gemini differ + qwen3@4096 recall@8) + committed `14-EVAL-EVIDENCE.md` (REQ-embed-gemini-direct, REQ-embed-prod-parity-eval; closes #261/#334)

### Phase 15: Additive Proto + Stub Write Handlers

**Goal**: The Connect wire contract for all six write RPCs exists, is additive-only, and is provably impossible to reach over an unauthenticated GET — before any business logic is wired behind it.
**Depends on**: Nothing (independent of the embedder track; establishes the write-lane's wire contract before any handler logic exists)
**Requirements**: REQ-connect-write-rpcs
**Success Criteria** (what is TRUE):

1. `EngramService` exposes `StoreMemory`, `StoreDiscovery`, `UpdateMemory`, `DeleteMemory`, `SetVisibility`, `ScheduleMemory` as additive proto RPCs (no field renumbering), with `gen/go` and `gen/ts` regenerated and CI's `buf` drift check green.
2. A CI lint/grep gate fails the build if any RPC in `engram.proto` carries `idempotency_level = NO_SIDE_EFFECTS` — asserted for all six new write RPCs (this option would make a mutating RPC GET-reachable and CSRF-exploitable, PITFALLS.md Pitfall 2).
3. Calling any of the six write RPCs today returns `CodeUnimplemented` (the safe default via the embedded `Unimplemented...Handler`), not a panic or 500; a raw HTTP GET against any write RPC's path returns non-2xx.
4. The five existing read RPCs are unaffected — identical wire format and behavior, verified by a regression test.

**Status**: Not started
**Plans**: 4/4 plans complete

**Wave 1** *(parallel — no file overlap)*

- [x] 15-01-PLAN.md — Additive proto contract (6 write RPCs + messages + Visibility enum + FieldMask/Timestamp + buf.validate) + protovalidate BSR dep + `buf.lock` + gen/ regenerate (SC1)
- [x] 15-02-PLAN.md — Idempotency-level ban gate: `task proto:lint` grep-ban + mirrored CI `buf` job step (SC2)

**Wave 2** *(blocked on 15-01)*

- [x] 15-03-PLAN.md — Hand-rolled protovalidate interceptor (`connectvalidate.go`) + `mountConnect` wiring, auth-before-validate order (D-08/D-10)

**Wave 3** *(blocked on 15-01 + 15-03)*

- [x] 15-04-PLAN.md — Descriptor regression test (D-12, SC2/SC4) + full negative-path matrix across all six write RPCs (D-11, SC3)

### Phase 16: CSRF Interceptor

**Goal**: The write lane's primary defense against cross-site request forgery exists in the Connect interceptor chain before any write RPC does real work.
**Depends on**: Phase 15 (needs the write-procedure names to gate on)
**Requirements**: REQ-connect-csrf
**Success Criteria** (what is TRUE):

1. Every state-changing Connect RPC rejects a request whose Origin/Sec-Fetch-Site indicates a cross-origin caller, via Go 1.26 stdlib `net/http.CrossOriginProtection` as the primary defense.
2. A session-bound double-submit CSRF token (HMAC over session identity, non-HttpOnly cookie, echoed as a request header) is required and validated on every write RPC before any handler logic runs; the token is never a bare random value and never checked without reference to the resolved `Subject`.
3. The five existing read RPCs are provably unaffected — no CSRF header is required for them, verified by a regression test enumerating each read RPC against the CSRF interceptor's write-only allowlist.
4. `TestConnectNoCORSHeaders` (or equivalent) remains green — no `Access-Control-Allow-Origin` is ever emitted from the Connect mux — preserved as a permanent CI gate, since same-origin (not `SameSite` alone) is the load-bearing CSRF mitigation.

**Status**: Planned
**Plans**: 3/3 plans complete
**Wave 1**

- [x] 16-01-PLAN.md — webauth CSRF signer: HKDF sub-key of ui.cookie_key + HMAC-over-Owner double-submit signer (D-08)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 16-02-PLAN.md — Connect write-only CSRF token interceptor (D-02/D-03/D-05/D-07) + Register/mountConnect wiring + D-06/SC2/SC3 regression matrix

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 16-03-PLAN.md — CrossOriginProtection whole-server wrap + Connect-shaped deny handler (D-04/D-07) + engram_csrf cookie minting (SC1)

**Flag for /gsd-secure-phase.**

### Phase 17: Wired Write Handlers (Full CRUD + Schedule)

**Goal**: A caller on the Connect write lane can create, update, delete, re-share, and schedule memories/discoveries with exactly the same authorization and business-logic guarantees as the MCP lane, because both lanes run through the identical code path.
**Depends on**: Phase 15 (proto contract), Phase 16 (CSRF gate already in place before any handler does real work)
**Requirements**: REQ-connect-write-authz-parity
**Success Criteria** (what is TRUE):

1. `deps.storeMemory`/`updateMemory`/`deleteMemory`/`setVisibility`/`scheduleMemory`/`storeDiscovery` accept an explicit `subj store.Subject, actor string` — no ctx-derived resolution internally — and the MCP tool call sites are updated to pass them explicitly with unchanged MCP behavior (verified: existing MCP test suite stays green).
2. **Invariant**: every Connect write handler is a thin proto/args adapter that calls the same `deps.*` method the MCP tool calls — never `store.*` directly — proven by an MCP/Connect parity test per RPC asserting identical rejections (rule un-share attempt, stale-summary conflict, cross-owner id) on both lanes.
3. A caller can `StoreMemory`/`StoreDiscovery`/`UpdateMemory`/`DeleteMemory`/`SetVisibility`/`ScheduleMemory` over Connect and see the effect reflected in subsequent reads; a rule record remains immutable/un-shareable over Connect exactly as over MCP (DEC-iedk preserved on both lanes).
4. Every by-id write RPC re-wraps a `store.ErrNotFound` with the caller's original input (short_id or UUID as supplied), never the resolved UUID — verified by a cross-owner table test per RPC — so no existence leak (DEC-xa6) reopens via a browser-visible network tab.
5. **Invariant**: no write RPC carries `idempotency_level = NO_SIDE_EFFECTS` — re-asserted by the Phase 15 CI gate now that real logic exists behind these RPCs.

**Status**: Planned (revised after cross-AI review round 2 — 6 plans, 4 waves)
**Plans**: 6/6 plans complete
**Wave 1**

- [x] 17-01-PLAN.md — Ordered owner-claim list + HARDENED injective namespace encoding (D-04/D-05/D-06) [wave 1]
- [x] 17-02-PLAN.md — Store payload-only update + memStore interface (incl DeleteAll/ListScopes) + caller seam + write-lane single-path + by-id results + sentinels (D-01/D-02/D-03/D-10) [wave 1]

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 17-03-PLAN.md — protoconv conversion layer + RFC3339Nano + result mapping + exact-mapping tests (D-09) [wave 2]
- [x] 17-06-PLAN.md — Read-lane transport-neutral typed core convergence (D-07 hardened) [wave 2]

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 17-04-PLAN.md — connectError mapper + scripted-spy fake + six write handlers + read-lane rewire + negative-matrix fix (D-07/D-10/D-11) [wave 3]

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 17-05-PLAN.md — Per-RPC spy parity table + split cross-owner leak tables + idempotency-ban re-assert (D-10/D-11/D-12) [wave 4]

**Flag for /gsd-secure-phase.**

### Phase 18: Stateless Session Rotation

**Goal**: An operator's authenticated session stays alive across a long working session without ever dropping an in-flight write, without introducing any server-side session state.
**Depends on**: Phase 17 (rotation's value is keeping write-capable sessions alive; sequenced after the write lane exists so this review doesn't conflate "does the write lane work" with "is the cookie security model still sound")
**Requirements**: REQ-session-rotation
**Success Criteria** (what is TRUE):

1. Every authenticated Connect request (read or write) re-seals the `{owner, expiry}` cookie with a fresh, forward-only expiry once the session crosses a documented re-seal threshold — with no new server-side state (honors DEC-u9v).
2. A new ADR documents what "rotation" means under statelessness and records the explicit no-revocation limitation (a stolen sealed cookie is valid for at most the session TTL; the only kill-switch is rotating `ENGRAM_SESSION_KEY`) rather than letting the trade-off drift past DEC-u9v silently.
3. Concurrent requests carrying the same near-expiry cookie all produce forward-monotonic expiries (`nowUTC().Add(sessionTTL)`, not a delta from the old value) — no re-seal race silently shortens a session.
4. Hard expiry stays strict and fail-closed; a documented, bounded clock-skew budget applies only to the rotation-threshold comparison, never to the hard expiry check itself.

**Status**: Planned
**Plans**: 3/3 plans complete

**Wave 1** *(parallel — no file overlap)*

- [x] 18-01-PLAN.md — webauth reseal core: Handler.Reseal (absolute forward-only expiry past ½-TTL+skew threshold, dual-cookie D-08) + headerOnlyWriter shim + resealThreshold/resealSkew constants; SC3 forward-monotonic concurrency + SC4 hard-expiry guard
- [x] 18-02-PLAN.md — SC2 ADR engram-slr8 (rotation-under-statelessness, no-revocation limitation, ENGRAM_UI_COOKIE_KEY kill-switch, hard-expiry vs threshold-skew split) + docs/adr/README.md index/prose

**Wave 2** *(blocked on 18-01)*

- [x] 18-03-PLAN.md — Connect reseal interceptor (innermost, best-effort, read AND write) + mountConnect/Register/serve.go DI ripple (webHandler.Reseal), interceptor-contract tests

**Flag for /gsd-secure-phase — mandatory** (changes the security posture of the whole cookie-auth model).

### Phase 19: Console Write UX

**Goal**: An operator can create, edit, delete, re-share, and schedule memories/discoveries directly from the console, with write failures handled gracefully rather than losing their input.
**Depends on**: Phase 17 (write lane must exist), Phase 16 (CSRF token pattern), Phase 18 (rotation UX stable)
**Requirements**: REQ-console-write-ux
**Success Criteria** (what is TRUE):

1. An operator can create, edit, delete, change visibility, and schedule a memory or discovery from the console UI, backed by the Connect write lane.
2. The console attaches the CSRF token to every write request automatically, mirroring the server-side double-submit pattern.
3. A write that fails because the session needed rotation is silently retried once through a re-seal; if that also fails, the operator is prompted to re-authenticate without losing their in-flight input.

**Status**: Not started
**Plans**: TBD
**UI hint**: yes

### Phase 20: Correctness & Polish

**Goal**: A cluster of independent correctness gaps identified during v0.9.x code review are closed, each removing a specific silent-failure or drift risk.
**Depends on**: Nothing (independent of the write-lane and embedder tracks; can be scheduled flexibly)
**Requirements**: REQ-discovery-proto-fidelity, REQ-shortid-mint-cap, REQ-embed-param-key-sharing, REQ-embed-body-build-collapse, REQ-discovery-shortid-schema, REQ-summarize-cronjob
**Success Criteria** (what is TRUE):

1. `SearchDiscoveries` returns `kind`, `citations`, and `summary` over the Connect wire instead of silently dropping them.
2. `MintShortID` gives up with an explicit exhaustion error after a bounded number of collision-retry attempts instead of looping indefinitely.
3. `config.ParseEmbedParams` and `embedReq`'s wire contract share a single reserved-param-key list, so they cannot silently desync.
4. `embed.Client.embed()` builds its request body via a single map-based code path (no separate struct-marshal vs. map-merge branches).
5. `storeDiscoveryArgs.ID`'s jsonschema advertises `short_id` support, matching the skill docs.
6. The Helm chart ships `engram summarize-missing` as a `batch/v1` CronJob reusing the Deployment's image/env plumbing via a shared `_helpers.tpl` block.

**Status**: Not started
**Plans**: TBD

### Phase 21: CI / Maintenance Hygiene

**Goal**: The CI pipeline and planning tooling stop generating false-positive red builds so real signal isn't drowned out by noise.
**Depends on**: Nothing (independent; can land any time)
**Requirements**: REQ-ci-renovate-spa-drift, REQ-p11-review-residuals, REQ-lint-planning-exclude
**Success Criteria** (what is TRUE):

1. A Renovate bump to the vendored SPA no longer reddens `main` — an in-repo self-healing fallback replaces the inert `postUpgradeTasks` rule.
2. The Phase-11 async-summary code-review residuals are resolved: WR-03 (`Wait` misuse), IN-01 (duplicate depth-gauge registration), IN-02 (test hermeticity).
3. `task lint:markdown` passes with `.planning/**` excluded from `.rumdl.toml`, while shipped Markdown outside `.planning/` is still linted (the systemic 331-failure planning-doc noise is gone).

**Status**: Not started
**Plans**: TBD

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (v0.8.x, shipped) · 9 → 10 → 11 → 12 (v0.9.x, shipped 2026-07-10) · 13 → 14 (embedder track, independent) · 15 → 16 → 17 → 18 → 19 (write-lane track, strict order) · 20 → 21 (independent, flexible scheduling) — v0.10.x in progress

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
| 18. Stateless Session Rotation | v0.10.x | 3/3 | Complete   | 2026-07-13 |
| 19. Console Write UX | v0.10.x | 0/1 | Not started | - |
| 20. Correctness & Polish | v0.10.x | 0/6 | Not started | - |
| 21. CI / Maintenance Hygiene | v0.10.x | 0/3 | Not started | - |

**v0.9.x — Recall Quality: ✅ shipped 2026-07-10 (PR #336) · 6/6 requirements · audit PASSED.**
**v0.10.x — Hardening & Write Lane: roadmap created 2026-07-10 · 9 phases (13–21) · 0/20 requirements complete.** Next: `/gsd-plan-phase 13`.
