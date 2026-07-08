# Requirements

Derived from 25 SPEC/design docs (precedence 1). These supply the WHAT / context /
requirements behind the locked decisions in `decisions.md`. Where a SPEC is the design doc
behind a locked ADR, the ADR wins on the locked decision and the requirement here captures the
functional intent and acceptance surface. No PRDs were present in this ingest set; no competing
acceptance variants exist.

Each entry: `REQ-{slug}` · source · description · acceptance surface (from scope) · related decision.

---

## Authorization & Isolation

### REQ-per-actor-isolation — Per-actor memory isolation (authentication → authorization)
- source: docs/superpowers/specs/2026-06-06-per-actor-memory-isolation-design.md
- description: Add an authorization layer over engram: per-actor read isolation, write gating,
  opt-in sharing, a stable owner key, and record migration for pre-isolation records.
- acceptance: memory store enforces owner scoping; OIDC sub/owner field on records;
  visibility/sharing model; Qdrant query filter; record migration path.
- realized-by: DEC-cgb, DEC-kyz, DEC-xa6, DEC-y1g, DEC-g37x, DEC-12c

### REQ-typed-subject-authz — Typed Subject for the authz core
- source: docs/superpowers/specs/2026-06-08-typed-subject-authz-core-design.md
- description: Replace the bare `sub string` caller-identity with a typed `Subject` sum so
  anonymous/authenticated states and the fail-closed branch are explicit.
- acceptance: Subject type in authz core; ownerFromContext resolution; OIDC sub → owner
  payload; store-layer enforcement is exhaustive/default-deny.
- realized-by: DEC-12c
- cross_ref: ADR engram-hvg (out-of-set), ADR engram-kyz

### REQ-configurable-claim-owner — Configurable-claim owner + general owner remap
- source: docs/superpowers/specs/2026-06-29-configurable-claim-owner-design.md
- description: Move the authz owner key from OIDC `sub` to a configurable identity claim
  (default `email`) and add a general owner-remap migration to survive IdP `sub` rotation.
- acceptance: owner authz key configurable; OIDC claim resolution; owner-remap migration;
  anonymous bucket preserved.
- realized-by: DEC-g37x
- note: SPEC states it supersedes out-of-set ADR engram-hvg; in-set locked DEC-g37x encodes
  the same decision — they agree.

### REQ-connect-auth-posture — Connect API auth posture (interim disposition + deferred R1–R4)
- source: docs/superpowers/specs/2026-06-09-connect-auth-posture-addendum.md
- description: Record the honest disposition of the interim anonymous Connect API mount and
  the deferred auth requirements R1–R4 for the cookie/OIDC observe lane.
- acceptance: interim anonymous owner bucket documented; deferred R1–R4 tracked for the
  cookie/OIDC observe lane; store isolation applies.
- status-note: DEFERRED requirements — carry forward into roadmap as future auth work.

---

## Recall & Windowing

### REQ-scheduled-memories — Scheduled / future memories (temporal validity window)
- source: docs/superpowers/specs/2026-06-12-scheduled-memories-design.md
- description: Time-gated memory recall via `not_before`/`not_after` temporal window fields
  stored as epoch-second Qdrant payloads.
- acceptance: not_before deferred reveal; not_after expiry; Qdrant filter gate on
  search_memory/list_memory; store_memory stays windowless.
- realized-by: DEC-90w, DEC-y1g

### REQ-windowed-cursor-recall — Server-side windowed + cursor recall
- source: docs/superpowers/specs/2026-06-29-windowed-cursor-recall-design.md
- description: Server-side indexed filtering, `created_at` range windows, and cursor
  pagination across engram recall tools.
- acceptance: list_memory/search_memory/list_scheduled accept created_after/created_before
  (half-open) and cursor; Qdrant payload indexes; scanCap/approximate retired.
- realized-by: DEC-1frj, DEC-ef28
- cross_ref: ADR engram-lkm (out-of-set)

### REQ-auto-summary — Auto-summary for curated memories (recall-time token reduction)
- source: docs/superpowers/specs/2026-06-25-auto-summary-curated-memories-design.md
- description: Recall-time summary shaping — explicit submitter-authored summaries with an
  operator-invoked cheap-model auto-summary fallback to reduce recall token cost.
- acceptance: summary field on records; summary_source provenance; Connect
  SearchMemories/ListMemories return summaries; operator `summarize` CLI for backfill.
- realized-by: DEC-ambu

---

## Memory Kinds & Handles

### REQ-discovery-memory-type — Discovery memory type (citation-backed, aging-aware)
- source: docs/superpowers/specs/2026-06-05-discovery-memory-type-design.md
- description: A citation-backed, aging-aware "discovery" memory type caching earned code
  understanding in engram.
- acceptance: discovery memory type on the Qdrant memory store; MCP tools; citations with
  aging pins; recalled on demand, never at session start.
- realized-by: DEC-2bv

### REQ-rule-memory-kind — Rule memory kind (normative, always-indexed ground truth)
- source: docs/superpowers/specs/2026-07-06-rule-memory-kind-design.md
- description: A normative `rule` memory category with guaranteed session-start surfacing and
  discrete enumeration.
- acceptance: rule category on Memory record; `rule:*` scope; session-start hook surfaces a
  one-line index; store_rule/list_rules tools; short_id support.
- realized-by: DEC-iedk

### REQ-short-id-handle — short_id handle for engram memories
- source: docs/superpowers/specs/2026-07-06-short-id-handle-design.md
- description: A 10-character Crockford base32 `short_id` handle that round-trips through
  by-id memory tools alongside the primary UUID.
- acceptance: short_id accepted by get/update/delete/set_visibility; Qdrant point-id lookup;
  short_id surfaced in recall output.
- realized-by: DEC-zzq0, DEC-02ta

---

## Embedder

### REQ-asymmetric-embedder-params — Asymmetric / cloud embedder param passthrough (query vs document)
- source: docs/superpowers/specs/2026-07-01-asymmetric-cloud-embedder-params-design.md
- description: Provider-agnostic query-vs-document embedder request-body params plus a
  document-side text instruction for asymmetric and both-side-prefix models.
- acceptance: query/document param maps merged into request body; input_type/task/task_type
  field passthrough; reindex boundary respected.
- realized-by: DEC-zyhq
- cross_ref: guides/reindex

---

## Config & Validation

### REQ-config-prefix-koanf — ENGRAM_ prefix, provider-neutral embedder, koanf internal/config
- source: docs/superpowers/specs/2026-06-14-engram-config-prefix-koanf-design.md
- description: Rename `MEM_*` env vars to `ENGRAM_*`, provider-neutral embedder keys, and a
  koanf-backed typed `internal/config` package.
- acceptance: ENGRAM_ env vars via koanf; provider-neutral embedder config; cmd/engram/serve
  and internal/telemetry/config consume typed config.
- realized-by: DEC-jgq, DEC-irq, DEC-378

### REQ-config-validation — Centralized config validation (Config.Validate)
- source: docs/superpowers/specs/2026-06-14-config-validation-design.md
- description: A single pure `Config.Validate()` asserting well-formedness of data-plane
  config fields, run loudly and early at every entrypoint.
- acceptance: qdrant.addr/embed.dim and related fields validated; invoked at
  serve/reindex/migrate/prune entrypoints; registry-driven.

---

## Telemetry & Observability

### REQ-observability-telemetry — Structured logging + OpenTelemetry (logs/metrics/traces)
- source: docs/superpowers/specs/2026-06-07-observability-logging-telemetry-design.md
- description: Structured `slog` logging plus OpenTelemetry metrics and traces exported over
  OTLP, instrumented at HTTP, MCP, and downstream-client seams.
- acceptance: slog structured logging; OTel metrics/traces over OTLP; MCP server
  instrumentation; Helm chart knobs.
- realized-by: DEC-dwi, DEC-uxh

### REQ-telemetry-seams — Telemetry & metrics at every seam (instrumentation depth)
- source: docs/superpowers/specs/2026-06-11-telemetry-at-every-seam-design.md
- description: OpenTelemetry spans and domain-latency metrics across store, embed, and auth
  layers plus a complete OTel resource and Helm knobs.
- acceptance: spans/metrics in internal/store, internal/embed, internal/auth; full OTel
  resource; charts/engram knobs.
- realized-by: DEC-dwi, DEC-uxh

---

## Web UI & Operator Console

### REQ-web-ui-console — engram web UI operator console (Svelte SPA + Go BFF + ConnectRPC)
- source: docs/superpowers/specs/2026-06-09-engram-web-ui-design.md
- description: An authenticated operator web console over engram's memory store, delivered in
  phases with a v1 read-only observe lane.
- acceptance: Svelte SPA + Go BFF over ConnectRPC; OIDC auth; read RPCs; discovery search;
  phased delivery.
- realized-by: DEC-8xe, DEC-0lu, DEC-bj6

### REQ-operator-console-spa — Operator-console SPA v1 (observe)
- source: docs/superpowers/specs/2026-06-10-engram-operator-console-spa-design.md
- description: Implementation-ready v1 read-only SvelteKit operator console consuming the
  Connect EngramService API.
- acceptance: SvelteKit adapter-static; EngramService Connect API; cookie/OIDC auth; offset
  pagination; go:embed static serving.
- realized-by: DEC-0lu, DEC-8xe

### REQ-operator-console-redesign — Operator console holistic shadcn-forward redesign
- source: docs/superpowers/specs/2026-06-12-operator-console-redesign-design.md
- description: Migrate the SvelteKit UI onto shadcn-svelte components and standard semantic
  theme tokens.
- acceptance: shadcn-svelte components; semantic theme tokens; svelte-query; Connect API;
  redesigned AppShell/ScopeRail/MemoryList/MemoryDetail/SearchPalette.

### REQ-memory-display-ux — Surface auto-summary + redesign memory display (web console)
- source: docs/superpowers/specs/2026-06-26-memory-display-summary-ux-design.md
- description: Surface real `summary`/`summary_source` fields and redesign the memory display
  in the web console with safe markdown rendering.
- acceptance: summary + summary_source provenance shown; MemoryRow/MemoryDetail redesign;
  safe markdown rendering.
- realized-by: DEC-ambu

### REQ-ui-test-unification — Unify UI test DOM via vitest 4 browser mode
- source: docs/superpowers/specs/2026-06-27-vitest-browser-mode-ui-test-unification-design.md
- description: Replace jsdom/happy-dom emulators with vitest 4 browser mode (real Chromium)
  for unified UI/sanitizer testing.
- acceptance: vitest 4 browser mode on Chromium/Playwright; DOMPurify sanitizer + bits-ui
  under real DOM; jsdom/happy-dom retired; CI test gate green.
- status-note: SPEC self-declares Status DRAFT; supersedes out-of-set decision engram-cv92.

---

## Docs Site & Brand

### REQ-docs-site — Documentation site (Astro Starlight on Cloudflare Workers)
- source: docs/superpowers/specs/2026-06-09-docs-site-astro-starlight-cloudflare-design.md
- description: A static Astro Starlight documentation site deployed to Cloudflare Workers as
  engram's canonical docs home.
- acceptance: Astro Starlight; Cloudflare Workers deploy; GitHub Actions deploy; pnpm;
  documentation content.
- realized-by: DEC-ttb

### REQ-docs-landing-redesign — Docs-site landing-page redesign
- source: docs/superpowers/specs/2026-06-13-docs-site-landing-redesign-design.md
- description: Redesign the Astro/Starlight docs landing page into a navigational product hub.
- acceptance: Starlight landing page (index.mdx); astro.config.mjs nav/sidebar; brand tokens.

### REQ-brand-identity — engram brand system (console + docs)
- source: docs/superpowers/specs/2026-06-13-engram-brand-identity-design.md
- description: A unified engram brand visual system applied across the operator console SPA
  and the Astro/Starlight docs site.
- acceptance: logo/wordmark/favicon; neural violet accent #6E56CF; applied across ui/,
  docs-site/, internal/webauth/static/.

---

## Packaging

### REQ-relocate-memory-curator — Relocate the memory-curator plugin into engram
- source: docs/superpowers/specs/2026-06-02-relocate-memory-curator-into-engram-design.md
- description: Relocate the memory-curator client plugin into the engram repo, rebranding it
  to engram under a bundled skill-plugin layout.
- acceptance: skill/engram bundle; plugin.json marketplace entry; MCP server id rebrand;
  SessionStart/PostToolUse hooks.
