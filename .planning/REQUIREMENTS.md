# Requirements: engram

**Defined:** 2026-07-07 (retrospective baseline — v0.8.x shipped)
**Active milestone:** v0.9.x — Recall Quality (opened 2026-07-09; see `## v0.9.x Requirements` and Phases 9–12 in ROADMAP.md)
**Core Value:** Correctable recall precision — a coding agent gets back the RIGHT memory for its context, and wrong/stale memories can be corrected or superseded.

> **Retrospective note.** engram is already shipped. IDs are reused verbatim from the ingest
> intel (`.planning/intel/requirements.md`). Requirements marked `[x]` are **implemented and
> merged to main** (Phases 1–8). `REQ-connect-auth-posture` (R1–R4) was found **already shipped**
> during a 2026-07-08 reconciliation — the cookie/OIDC observe lane landed in PR #248/#266; the
> earlier "deferred / not started" note (ingested from the 2026-06-09 plan/spec, which described
> the interim anonymous state as current) was stale.

## v1 Requirements

Shipped in the v0.8.x baseline. Each maps to exactly one roadmap phase.

### Authorization & Isolation

- [x] **REQ-per-actor-isolation**: Authorization layer over engram — per-actor read isolation, write gating, opt-in sharing, a stable owner key, and record migration for pre-isolation records. *(realized-by DEC-cgb, DEC-kyz, DEC-xa6, DEC-y1g, DEC-g37x, DEC-12c)*
- [x] **REQ-typed-subject-authz**: Typed `Subject` sum replacing bare `sub string`, making anonymous/authenticated states and the fail-closed branch explicit with store-layer exhaustive default-deny. *(realized-by DEC-12c)*
- [x] **REQ-configurable-claim-owner**: Owner key moved from OIDC `sub` to a configurable identity claim (default `email`) plus a general owner-remap migration to survive IdP `sub` rotation. *(realized-by DEC-g37x)*

### Recall & Windowing

- [x] **REQ-scheduled-memories**: Time-gated recall via `not_before`/`not_after` temporal window fields stored as epoch-second Qdrant payloads; `store_memory` stays windowless. *(realized-by DEC-90w, DEC-y1g)*
- [x] **REQ-windowed-cursor-recall**: Server-side indexed filtering, `created_at` half-open range windows, and opaque cursor pagination across recall tools; scanCap/approximate retired. *(realized-by DEC-1frj, DEC-ef28)*
- [x] **REQ-auto-summary**: Recall-time summary shaping — explicit submitter-authored summaries with an operator-invoked cheap-model auto-summary fallback, `summary_source` provenance, and a `summarize` backfill CLI. *(realized-by DEC-ambu)*

### Memory Kinds & Handles

- [x] **REQ-discovery-memory-type**: Citation-backed, aging-aware "discovery" memory kind caching earned code understanding; recalled on demand, never at session start. *(realized-by DEC-2bv)*
- [x] **REQ-rule-memory-kind**: Normative `rule` memory category in a `rule:*` scope with session-start one-line index surfacing, `store_rule`/`list_rules` tools, and short_id support. *(realized-by DEC-iedk)*
- [x] **REQ-short-id-handle**: 10-char Crockford base32 `short_id` handle round-tripping through by-id tools alongside the primary UUID, resolved at the handler layer. *(realized-by DEC-zzq0, DEC-02ta)*

### Embedder

- [x] **REQ-asymmetric-embedder-params**: Provider-agnostic query-vs-document embedder request-body param maps plus a document-side text instruction for asymmetric/both-side-prefix models; reindex boundary respected. *(realized-by DEC-zyhq; connection vars renamed by DEC-378)*

### Config & Validation

- [x] **REQ-config-prefix-koanf**: `MEM_*` → `ENGRAM_*` rename, provider-neutral embedder keys, and a koanf-backed typed `internal/config` package consumed at every entrypoint. *(realized-by DEC-jgq, DEC-irq, DEC-378)*
- [x] **REQ-config-validation**: A single pure `Config.Validate()` asserting well-formedness of data-plane config, run loudly and early at serve/reindex/migrate/prune entrypoints. *(registry-driven)*

### Telemetry & Observability

- [x] **REQ-observability-telemetry**: Structured `slog` logging plus OpenTelemetry metrics and traces over OTLP, instrumented at HTTP, MCP, and downstream-client seams, with Helm knobs. *(realized-by DEC-dwi, DEC-uxh)*
- [x] **REQ-telemetry-seams**: OTel spans and domain-latency metrics across store, embed, and auth layers plus a complete OTel resource. *(realized-by DEC-dwi, DEC-uxh)*

### Web UI & Operator Console

- [x] **REQ-web-ui-console**: Authenticated operator web console over the memory store — Svelte SPA + Go BFF over ConnectRPC, OIDC auth, v1 read-only observe lane, discovery search. *(realized-by DEC-8xe, DEC-0lu, DEC-bj6)*
- [x] **REQ-operator-console-spa**: Implementation-ready v1 read-only SvelteKit operator console consuming the Connect `EngramService` API; adapter-static + `go:embed` serving. *(realized-by DEC-0lu, DEC-8xe)*
- [x] **REQ-operator-console-redesign**: Migrate the SvelteKit UI onto shadcn-svelte components and semantic theme tokens (svelte-query, redesigned AppShell/ScopeRail/MemoryList/MemoryDetail/SearchPalette).
- [x] **REQ-memory-display-ux**: Surface real `summary`/`summary_source` fields and redesign the memory display with safe markdown rendering. *(realized-by DEC-ambu)*
- [x] **REQ-ui-test-unification**: Replace jsdom/happy-dom emulators with vitest 4 browser mode (real Chromium) for unified UI/sanitizer testing; CI test gate green.

### Docs Site & Brand

- [x] **REQ-docs-site**: Static Astro Starlight documentation site deployed to Cloudflare Workers as engram's canonical docs home. *(realized-by DEC-ttb)*
- [x] **REQ-docs-landing-redesign**: Redesign the Astro/Starlight docs landing page into a navigational product hub.
- [x] **REQ-brand-identity**: Unified engram brand visual system (logo/wordmark/favicon, neural violet accent #6E56CF) applied across the operator console SPA and the docs site.

### Packaging

- [x] **REQ-relocate-memory-curator**: Relocate the memory-curator client plugin into the engram repo under a bundled skill/engram layout with a marketplace entry, MCP server id rebrand, and SessionStart/PostToolUse hooks.

## Connect Auth Posture (R1–R4) — shipped

### Connect Auth Posture (R1–R4)

- [x] **REQ-connect-auth-posture**: Replace the interim anonymous Connect API mount with full cookie/OIDC observe-lane authentication (**R1–R4**), so the read API serves real per-actor identities. **Shipped** in PR #248 (webauth lane) + PR #266 (owner-claim hardening); R1–R4 verified green on main 2026-07-08 (`internal/webauth/*`, `mountConnect` gating, `TestConnect*` isolation/obs/CORS tests). Maps to Phase 8. *(source: docs/superpowers/specs/2026-06-09-connect-auth-posture-addendum.md)*

### Deferred follow-ups (v0.10.x candidates — not routed to a v0.9.x phase)

Genuine remaining work beyond R1–R4; carried to a later milestone (not part of v0.9.x Recall Quality):

- Connect **write-lane** RPCs (`StoreMemory`/`StoreDiscovery`) + CSRF-token hardening (plan §"Out of scope") — GitHub #322.
- Session **refresh-token rotation** / re-seal on access-token expiry (v1 trusts the sealed cookie's `sub` until the session TTL) — GitHub #323.

## v0.9.x Requirements — Recall Quality

**Milestone opened:** 2026-07-09 (`/gsd-new-milestone`). **Theme:** make recall measurably
trustworthy as the store grows, so `search-before-store` stops producing duplicates. Scoped
from the promoted backlog (GitHub milestone `v0.9.x`); the consolidation/write-lane remainder is
deferred to v0.10.x. Each requirement maps to exactly one phase (9–12).

### Retrieval Ranking & Evaluation (Phase 9)

- [x] **REQ-retrieval-eval**: A reproducible retrieval-quality evaluation harness — a labeled query→expected-record dataset (including the #261 miss as a regression fixture), `recall@k` / MRR metrics, runnable via a `task eval:retrieval` target — so ranking and embedding changes are measured, not guessed. *(GitHub #261; foundational for Phases 9–11)*
- [x] **REQ-search-similarity-scores**: `search_memory` optionally returns a per-result similarity score so callers/agents can gauge how close a near-miss was (and so the eval harness can assert score separation). *(GitHub #261)*
- [x] **REQ-ranking-precision**: Eliminate phrasing-sensitive ranking and "sticky topical neighbor" crowding so a near-verbatim restatement of a record surfaces that record within default `k` — via hybrid dense+lexical (BM25) fusion and/or a higher-`k`+rerank strategy, **selected by the REQ-retrieval-eval numbers**. *(GitHub #261)*

### Embedder Query/Document Asymmetry (Phase 10)

- [x] **REQ-embedder-native-params**: Query-vs-document embedding asymmetry via the embedder's **native** mechanism, extending the shipped text-prefix `REQ-asymmetric-embedder-params`: (1) per-call API-field passthrough (`ENGRAM_EMBED_QUERY_PARAM` / `ENGRAM_EMBED_DOCUMENT_PARAM`, e.g. `input_type=search_query`) for cloud embedders (Cohere/Google/Voyage/Jina); (2) a document-side prefix knob (`ENGRAM_EMBED_DOCUMENT_INSTRUCTION`) for both-side-prefix models (E5/nomic) applied at store **and** reindex, respecting the reindex boundary. Documented per-model in `guides/embedding-instructions`. *(GitHub #305; extends DEC-zyhq — VERIFIED ALREADY SHIPPED under Phase 4 on 2026-07-10; #305 closed; evidence in phases/10-.../10-VERIFICATION.md)*

### Recall Surface Completeness (Phase 11)

- [x] **REQ-async-summaries**: Async-on-write summary fill — after a successful `store_memory` upsert, enqueue the record id; an in-process worker pool drains it via the idempotent, vector-preserving `Store.FillSummary`, so new summary-less records gain summaries without an operator sweep and **without** putting the summarizer on the synchronous write path (a gateway outage must never fail `store_memory`). Broad enablement gated on the summary-fidelity eval (`task eval:summary`). *(GitHub #320; builds on DEC-ambu / the shipped `summarize-missing` CLI)*

### Curation Telemetry (Phase 12)

- [x] **REQ-usage-signals**: Strong per-record usage signals to inform curation — increment counters **only** on `get_memory` fetch-by-id and `update_memory` (never search/list result-set membership); hybrid storage (recall ids on OTLP spans → ClickStack for analytics; a payload `access_count` on get/update for MCP-visible curation tools); server-set operational metadata that **never silently affects ranking** (usage-weighted recall would be its own deliberate future decision). *(GitHub #317; design-first)*

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Auto-extraction of memories | Core invariant — capture is explicit and user-blessed; keeps recall zero-junk |
| Prometheus `/metrics` scrape endpoint | Telemetry is OTLP-gRPC only (DEC-dwi) |
| SSR (console or docs) | Adapter-static SPA + static docs only (DEC-0lu, DEC-ttb) |
| viper / config files / `MEM_*` vars | ENGRAM_ koanf only; `MEM_*` is a fatal startup guard (DEC-jgq, DEC-irq) |
| Separate Qdrant collection per memory kind | Discovery/rule/scheduled live in the single Memory collection (DEC-2bv) |
| Client-config generalization + `/engram-setup` | Superseded by out-of-set ADR engram-50b; historical context only |
| Database migrations, cocogitto | Not used in this project |
| Usage-weighted recall (ranking by `access_count`) | Out of scope for v0.9.x — usage signals (REQ-usage-signals) are curation metadata only; letting them affect ranking is a separate deliberate decision |
| Connect write-lane + session rotation | Deferred to v0.10.x (GitHub #322/#323); v0.9.x is recall-quality-only |

## Traceability

Which phase covers which requirement. Retrospective — completed requirements are `Complete`.

| Requirement | Phase | Status |
|-------------|-------|--------|
| REQ-per-actor-isolation | Phase 1 | Complete |
| REQ-typed-subject-authz | Phase 1 | Complete |
| REQ-configurable-claim-owner | Phase 1 | Complete |
| REQ-scheduled-memories | Phase 2 | Complete |
| REQ-windowed-cursor-recall | Phase 2 | Complete |
| REQ-auto-summary | Phase 2 | Complete |
| REQ-discovery-memory-type | Phase 3 | Complete |
| REQ-rule-memory-kind | Phase 3 | Complete |
| REQ-short-id-handle | Phase 3 | Complete |
| REQ-asymmetric-embedder-params | Phase 4 | Complete |
| REQ-config-prefix-koanf | Phase 5 | Complete |
| REQ-config-validation | Phase 5 | Complete |
| REQ-observability-telemetry | Phase 6 | Complete |
| REQ-telemetry-seams | Phase 6 | Complete |
| REQ-web-ui-console | Phase 7 | Complete |
| REQ-operator-console-spa | Phase 7 | Complete |
| REQ-operator-console-redesign | Phase 7 | Complete |
| REQ-memory-display-ux | Phase 7 | Complete |
| REQ-ui-test-unification | Phase 7 | Complete |
| REQ-docs-site | Phase 7 | Complete |
| REQ-docs-landing-redesign | Phase 7 | Complete |
| REQ-brand-identity | Phase 7 | Complete |
| REQ-relocate-memory-curator | Phase 7 | Complete |
| REQ-connect-auth-posture | Phase 8 | Complete |
| REQ-retrieval-eval | Phase 9 | Planned (v0.9.x) |
| REQ-search-similarity-scores | Phase 9 | Planned (v0.9.x) |
| REQ-ranking-precision | Phase 9 | Planned (v0.9.x) |
| REQ-embedder-native-params | Phase 10 | Complete (already shipped) |
| REQ-async-summaries | Phase 11 | Complete (v0.9.x) |
| REQ-usage-signals | Phase 12 | Complete (v0.9.x) |

**Coverage:**

- Routed requirements: 30 total (24 shipped v0.8.x + 6 planned v0.9.x)
- Mapped to phases: 30 ✓
- Unmapped: 0 ✓
- Complete: 24 (all v0.8.x requirements shipped)
- Planned (v0.9.x Recall Quality): 6 across Phases 9–12
- Deferred (v0.10.x): Connect write-lane + CSRF (#322), session refresh rotation (#323), and the consolidation tail (short_id polish, embed refactors, summarize CronJob, proto fidelity) — unrouted.

---

*Requirements defined: 2026-07-07*
*Last updated: 2026-07-09 — opened milestone v0.9.x (Recall Quality); routed 6 requirements to Phases 9–12*
