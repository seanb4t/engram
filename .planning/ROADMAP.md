# Roadmap: engram

## Overview

This is a **retrospective / as-built roadmap**. engram is already shipped (current release line
**v0.8.x**). Phases 1–7 group the already-completed work by synthesis area — Authorization &
Isolation, Recall Semantics, Memory Kinds & Tools, Embedder, Config & Transport, Telemetry &
Observability, and Web UI / Docs Site / Distribution. All 56 ADR-locked decisions (25 core + 31
companion refinements, folded 2026-07-08) and all 24 routed requirements are implemented and
merged to main. Per-phase implementation plans are cross-referenced in
`.planning/intel/merge-plans/context.md`. Phase 8 (Connect observe-lane auth hardening, R1–R4)
was **found already shipped** during a 2026-07-08 reconciliation — the cookie/OIDC lane landed
opportunistically inside PR #248 and was hardened in PR #266, before this retrospective baseline
was authored; the earlier "deferred stub" framing (ingested from the 2026-06-09 plan/spec, which
described the interim anonymous state as current) was stale. Success criteria are stated as
observable truths that hold in the shipped baseline.

## Milestones

- ✅ **v0.8.x Baseline** — Phases 1–7 (shipped)
- ✅ **Connect Auth Hardening** — Phase 8 (shipped; R1–R4 verified 2026-07-08)
- 🚧 **v0.9.x — Recall Quality** — Phases 9–12 (opened 2026-07-09): retrieval eval + ranking precision (#261), embedder query/document asymmetry (#305), async-on-write summaries (#320), per-memory usage signals (#317)

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
- [x] **Phase 8: Connect Observe-Lane Auth Hardening** - Cookie/OIDC observe lane replaces the interim anonymous mount (R1–R4); shipped in PR #248/#266

### v0.9.x — Recall Quality

- [x] **Phase 9: Retrieval Eval Harness & Ranking Precision** - Labeled retrieval eval (recall@k/MRR), similarity scores in `search_memory`, hybrid/rerank to kill phrasing-sensitivity — chosen by the eval numbers (completed 2026-07-10)
- [x] **Phase 10: Asymmetric Query/Document Embeddings** - Native API-param passthrough (cloud) + document-side prefix (E5/nomic) for query≠document embeds — found ALREADY SHIPPED under Phase 4 (verified 2026-07-10; #305 closed; no plans built)
- [ ] **Phase 11: Async-on-Write Summaries** - In-process worker drains `FillSummary` after upsert, off the synchronous write path; eval-gated
- [ ] **Phase 12: Per-Memory Usage Signals** - Strong-signal counters (get/update) via hybrid OTLP + payload `access_count`; never affects ranking

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

### Phase 9: Retrieval Eval Harness & Ranking Precision

**Goal**: A near-verbatim restatement of a record reliably surfaces that record within default `k`, and recall changes are measurable — so `search-before-store` stops producing duplicates (the #261 failure).
**Depends on**: Phase 2 (recall semantics), Phase 6 (telemetry — eval reuses OTLP seams)
**Requirements**: REQ-retrieval-eval, REQ-search-similarity-scores, REQ-ranking-precision
**Success Criteria** (what is TRUE):

1. A reproducible retrieval eval (`task eval:retrieval`) runs a labeled query→expected-record dataset — including the #261 miss as a regression fixture — and reports `recall@k` / MRR; CI or a make target can detect regressions.
2. `search_memory` can return a per-result similarity score (opt-in), and the eval asserts score separation between the target record and its sticky topical neighbors.
3. Phrasing-sensitive misses are eliminated: Query A/B from #261 (near-verbatim restatements) surface Record T within default `k`, via hybrid dense+lexical fusion and/or higher-`k`+rerank — the approach chosen by the eval numbers, not assumed.

**Status**: Planned (v0.9.x)
**Plans**: 3/3 plans complete
**Wave 1**

- [x] 09-01-PLAN.md — Retrieval eval harness (`task eval:retrieval`, env-gated Go test, #261 regression fixture, recall@k/MRR) [Wave 1]
- [x] 09-02-PLAN.md — Document the always-on `search_memory` similarity score (tools.go Description, CLAUDE.md, docs-site); record D-04 supersession [Wave 1]

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 09-03-PLAN.md — Eval-gated ranking precision: D-06 heuristic rerank first; D-07 hybrid / D-08 cross-encoder as conditional escalations chosen by the eval numbers [Wave 2]

### Phase 10: Asymmetric Query/Document Embeddings

**Goal**: Query and document embeddings can diverge via the embedder's native mechanism, improving retrieval for asymmetric models — measured against the Phase 9 harness.
**Depends on**: Phase 9 (eval harness to prove the gain), Phase 4 (shipped embedder text-prefix knobs)
**Requirements**: REQ-embedder-native-params
**Success Criteria** (what is TRUE):

1. Cloud embedders receive a distinct native API field per call — `ENGRAM_EMBED_QUERY_PARAM` on `EmbedQuery`, `ENGRAM_EMBED_DOCUMENT_PARAM` on `Embed` (e.g. Cohere `input_type=search_query|search_document`, Google `task_type`, Voyage/Jina equivalents).
2. Both-side-prefix models (E5/nomic) get a `ENGRAM_EMBED_DOCUMENT_INSTRUCTION` document-side prefix applied at store **and** reindex, honoring the reindex boundary (a doc-prefix change requires a reindex).
3. Behavior is documented per-model in `guides/embedding-instructions`, and the Phase 9 eval shows a non-regression (ideally improvement) for at least one asymmetric model configuration.

**Status**: Complete — ALREADY SHIPPED (verified 2026-07-10; see `10-VERIFICATION.md`)
**Plans**: None built. `/gsd-discuss-phase 10` baseline verification found `REQ-embedder-native-params` fully implemented under Phase 4: `embed.go` `WithQueryParams`/`WithDocumentParams`/`WithDocumentInstruction` (native `input_type`/`task_type` map passthrough + E5/nomic doc prefix), config `ENGRAM_EMBED_QUERY_PARAMS`/`_DOCUMENT_PARAMS`/`_DOCUMENT_INSTRUCTION` (validated, wired in `embedderFromConfig`), tests, and per-model docs. GitHub #305 closed as already-shipped. Optional follow-up: eval demonstration of retrieval benefit for an asymmetric config (rides on #334).

### Phase 11: Async-on-Write Summaries

**Goal**: New summary-less records gain summaries automatically, without an operator sweep and without risking the write path.
**Depends on**: Phase 2 (auto-summary / `FillSummary` seam)
**Requirements**: REQ-async-summaries
**Success Criteria** (what is TRUE):

1. After a successful `store_memory` upsert, the record id is enqueued and an in-process worker pool drains it via the idempotent, vector-preserving `Store.FillSummary`.
2. The summarizer is never on the synchronous write path — a gateway/embedder outage degrades to "no summary yet" and **never** fails `store_memory`.
3. Broad auto-enablement is gated behind the summary-fidelity eval (`task eval:summary`); the queue is bounded and observable (depth/latency on OTLP).

**Status**: Planned (v0.9.x)
**Plans**: 1/3 plans executed

**Wave 1**

- [x] 11-01-PLAN.md — Foundation: `ENGRAM_SUMMARY_ON_WRITE`/`_WORKERS`/`_QUEUE_SIZE` config knobs + `Validate()`, `cenkalti/backoff/v5` direct promotion, `telemetry.SummaryQueueMetrics` instruments [Wave 1]

**Wave 2** *(blocked on Wave 1)*

- [ ] 11-02-PLAN.md — Queue core: `internal/server/summaryqueue.go` bounded non-blocking worker pool (drop-and-count, bounded backoff retry, best-effort drain) + deterministic Qdrant-free tests [Wave 2]

**Wave 3** *(blocked on Wave 2)*

- [ ] 11-03-PLAN.md — Wiring + lifecycle + docs: enqueue in `store_memory`/`schedule_memory` behind the D-01 AND-gate, `Register` drain returned + drained after `httpSrv.Shutdown`, depth gauge wired, docs/CLAUDE.md [Wave 3]

### Phase 12: Per-Memory Usage Signals

**Goal**: Track strong per-record usage to inform curation, as operational metadata that never silently changes what recall returns.
**Depends on**: Phase 6 (OTLP → ClickStack), Phase 3 (by-id tools)
**Requirements**: REQ-usage-signals
**Success Criteria** (what is TRUE):

1. Counters increment **only** on `get_memory` fetch-by-id and `update_memory` — never on search/list result-set membership (avoids noisy, write-amplifying, racy updates).
2. Storage is hybrid: recall ids ride OTLP spans → ClickStack for analytics (zero storage change), and a payload `access_count` is maintained on get/update for MCP-visible curation tools.
3. Usage counters are server-set and **never** silently affect ranking; any usage-weighted recall remains an explicit, out-of-scope future decision.

**Status**: Planned (v0.9.x) — design-first
**Plans**: TBD — `/gsd-plan-phase 12`. Source: GitHub #317.

## Progress

**Execution Order:** 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 (shipped) · 9 → 10 → 11 → 12 (v0.9.x, planned)

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
| 9. Retrieval Eval & Ranking Precision | v0.9.x | 3/3 | Complete    | 2026-07-10 |
| 10. Asymmetric Query/Document Embeddings | v0.9.x | 0/1 | Planned | — |
| 11. Async-on-Write Summaries | v0.9.x | 1/3 | In Progress|  |
| 12. Per-Memory Usage Signals | v0.9.x | 0/1 | Planned | — |
