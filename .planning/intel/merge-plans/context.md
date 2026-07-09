<!--
SPDX-License-Identifier: Apache-2.0
-->
# Context Intel — Implementation Plans (DOC set)

24 engram implementation/execution plans, all classified DOC (precedence 3 — lowest).
These are the HOW behind features whose WHAT/decisions are already owned by higher-precedence
baseline ADRs + specs + Pass-A companion ADRs. Role here: traceability context only. A DOC
cannot override any decision/requirement/constraint, so no normative content is extracted from them.

Each entry: title + one-line summary + source path + the baseline phase / DEC / REQ it implements.
Grouped by the baseline roadmap phase (1–7, plus the deferred Phase 8) each plan realizes.

---

## Phase 1 — Authorization & Isolation

Implements: REQ-per-actor-isolation, REQ-typed-subject-authz, REQ-configurable-claim-owner
(DEC-cgb, DEC-g37x, DEC-kyz, DEC-xa6, DEC-12c)

### Per-actor Memory Isolation — Implementation Plan

Adds an engram-wide store authz layer so each actor reads/mutates only its own memories, with opt-in per-record sharing.

- source: docs/superpowers/plans/2026-06-06-per-actor-memory-isolation.md
- implements: Phase 1 / DEC-cgb, DEC-kyz, DEC-xa6 / REQ-per-actor-isolation
- design spec: docs/superpowers/specs/2026-06-06-per-actor-memory-isolation-design.md

### Typed `Subject` authz-core refactor — Implementation Plan

Replaces the bare `sub string` caller-identity param with a sealed `store.Subject` sum type (Anonymous | Authenticated) that fails closed by construction.

- source: docs/superpowers/plans/2026-06-08-typed-subject-authz-core.md
- implements: Phase 1 / DEC-12c / REQ-typed-subject-authz
- design spec: (none — refactor plan; refs ADR engram-cgb, engram-12c)

### Configurable-claim owner + general owner remap — Implementation Plan

Moves the per-record authz key from OIDC `sub` to a configurable identity claim (default email, fail-closed) and adds a `migrate-remap-owner` CLI verb.

- source: docs/superpowers/plans/2026-06-29-configurable-claim-owner.md
- implements: Phase 1 / DEC-g37x / REQ-configurable-claim-owner
- design spec: docs/superpowers/specs/2026-06-29-configurable-claim-owner-design.md

---

## Phase 2 — Recall Semantics

Implements: REQ-scheduled-memories, REQ-windowed-cursor-recall, REQ-auto-summary
(DEC-ambu, DEC-4xt7, DEC-y1g, DEC-1frj, DEC-ef28; schedule tool surface DEC-90w presented in Phase 3)

### Scheduled / Future Memories — Implementation Plan

Adds a temporal validity window (not_before deferred reveal + not_after expiry) gated purely at recall, with schedule_memory/list_scheduled tools and a prune-expired CLI.

- source: docs/superpowers/plans/2026-06-12-scheduled-memories.md
- implements: Phase 2 / DEC-y1g, DEC-90w / REQ-scheduled-memories
- design spec: docs/superpowers/specs/2026-06-12-scheduled-memories-design.md

### Server-Side Windowed + Cursor Recall — Implementation Plan

Adds Qdrant payload indexes and rebuilds recall to server-side date-window filtering plus cursor paging on all recall tools, retiring the in-memory scanCap/approximate slice.

- source: docs/superpowers/plans/2026-06-29-windowed-cursor-recall.md
- implements: Phase 2 / DEC-1frj, DEC-ef28 / REQ-windowed-cursor-recall
- design spec: docs/superpowers/specs/2026-06-29-windowed-cursor-recall-design.md

### Auto-Summary for Curated Memories — Implementation Plan

Cuts recall-time token cost by returning a short summary in place of full content, with caller-authored summaries and an operator-invoked cheap-model summarize-missing sweep.

- source: docs/superpowers/plans/2026-06-25-auto-summary-curated-memories.md
- implements: Phase 2 / DEC-ambu, DEC-4xt7 / REQ-auto-summary
- design spec: docs/superpowers/specs/2026-06-25-auto-summary-curated-memories-design.md

---

## Phase 3 — Memory Kinds & Tools

Implements: REQ-discovery-memory-type, REQ-rule-memory-kind, REQ-short-id-handle
(DEC-2bv, DEC-90w, DEC-iedk, DEC-zzq0, DEC-02ta)

### Discovery Memory Type — Implementation Plan

Adds a citation-backed, aging-aware discovery memory type as a 5th category with store_discovery/search_discovery tools and a discovering capture skill.

- source: docs/superpowers/plans/2026-06-05-discovery-memory-type.md
- implements: Phase 3 / DEC-2bv / REQ-discovery-memory-type
- design spec: docs/superpowers/specs/2026-06-05-discovery-memory-type-design.md

### `rule` Memory Kind — Implementation Plan

Adds a normative, user-blessed, always-shared `rule` memory category enumerable as a complete set and surfaced at session start as a progressive-disclosure one-line index.

- source: docs/superpowers/plans/2026-07-06-rule-memory-kind.md
- implements: Phase 3 / DEC-iedk / REQ-rule-memory-kind
- design spec: (refs short-id design spec; ADR engram-iedk, engram-0gy)
- intra-set ref: docs/superpowers/plans/2026-07-06-short-id-handle.md (traceability edge, acyclic)

### short_id Handle — Implementation Plan

Gives every memory a 10-char Crockford base32 short_id handle that round-trips through every by-id tool, minted server-side with a backfill command for legacy records.

- source: docs/superpowers/plans/2026-07-06-short-id-handle.md
- implements: Phase 3 / DEC-zzq0, DEC-02ta / REQ-short-id-handle
- design spec: docs/superpowers/specs/2026-07-06-short-id-handle-design.md

---

## Phase 4 — Embedder

Implements: REQ-asymmetric-embedder-params (DEC-378, DEC-zyhq)

### Asymmetric / cloud embedder param passthrough — Implementation Plan

Lets engram embed queries and documents asymmetrically via provider-agnostic request-body params and a document-side text instruction, without changing default behavior.

- source: docs/superpowers/plans/2026-07-01-asymmetric-cloud-embedder-params.md
- implements: Phase 4 / DEC-zyhq / REQ-asymmetric-embedder-params
- design spec: docs/superpowers/specs/2026-07-01-asymmetric-cloud-embedder-params-design.md

---

## Phase 5 — Config & Transport

Implements: REQ-config-prefix-koanf, REQ-config-validation (DEC-jgq, DEC-irq, DEC-bj6)

### ENGRAM_ config prefix, provider-neutral embedder, koanf internal/config — Implementation Plan

Renames every MEM_* env var to ENGRAM_* (LiteLLM to OPENAI) behind a new koanf-backed internal/config package with a field registry and a fatal startup guard for retired vars.

- source: docs/superpowers/plans/2026-06-14-engram-config-prefix-koanf.md
- implements: Phase 5 / DEC-jgq, DEC-irq / REQ-config-prefix-koanf
- design spec: docs/superpowers/specs/2026-06-14-engram-config-prefix-koanf-design.md

### Centralized config validation (`Config.Validate`) — Implementation Plan

Adds a pure `Config.Validate()` method asserting the data-plane config is well-formed, wired into the store-build choke point plus a serve-local listen_addr guard, so malformed config fails loudly at startup.

- source: docs/superpowers/plans/2026-06-14-config-validation.md
- implements: Phase 5 / DEC-bj6 / REQ-config-validation
- design spec: docs/superpowers/specs/2026-06-14-config-validation-design.md

---

## Phase 6 — Telemetry & Observability

Implements: REQ-observability-telemetry, REQ-telemetry-seams (DEC-dwi, DEC-uxh)

### Observability: structured logging + OpenTelemetry — Implementation Plan

Adds structured slog logging plus OTel metrics and traces exported over OTLP, instrumented at the HTTP/auth, MCP tool-call, and downstream-client seams.

- source: docs/superpowers/plans/2026-06-07-observability-logging-telemetry.md
- implements: Phase 6 / DEC-dwi / REQ-observability-telemetry
- design spec: docs/superpowers/specs/2026-06-07-observability-logging-telemetry-design.md

### Telemetry & Metrics At Every Seam — Implementation Plan

Instruments the store, embed, and auth layers with inline OTel spans and per-operation latency metrics, completes the OTel resource attribute set, and exposes sampler/interval + k8s knobs in the Helm chart.

- source: docs/superpowers/plans/2026-06-11-telemetry-at-every-seam.md
- implements: Phase 6 / DEC-uxh / REQ-telemetry-seams
- design spec: docs/superpowers/specs/2026-06-11-telemetry-at-every-seam-design.md

---

## Phase 7 — Web UI, Docs Site & Distribution

Implements: REQ-web-ui-console, REQ-operator-console-spa, REQ-operator-console-redesign,
REQ-memory-display-ux, REQ-ui-test-unification, REQ-docs-site, REQ-docs-landing-redesign,
REQ-brand-identity, REQ-relocate-memory-curator (DEC-8xe, DEC-0lu, DEC-ttb)

### engram web UI — v1 backend API foundation — Implementation Plan

Stands up engram's ConnectRPC read API (EngramService v1) on the binary, authorized by the same Subject/owner model as the MCP tools, fully tested via Go handler tests.

- source: docs/superpowers/plans/2026-06-09-engram-web-ui-v1-backend.md
- implements: Phase 7 / DEC-8xe, DEC-0lu / REQ-web-ui-console
- design spec: docs/superpowers/specs/2026-06-09-engram-web-ui-design.md

### engram operator-console SPA (v1 observe) — Implementation Plan

Ships a read-only SvelteKit operator console over the Connect EngramService API after extending ListMemories with offset pagination and server-side filters.

- source: docs/superpowers/plans/2026-06-10-engram-operator-console-spa.md
- implements: Phase 7 / DEC-0lu / REQ-operator-console-spa
- design spec: docs/superpowers/specs/2026-06-10-engram-operator-console-spa-design.md

### Operator Console Redesign — Implementation Plan

Rebuilds the operator console (ui/) as a modern responsive shadcn-svelte app, fixing rendering bugs and migrating off the custom eg-*/--cat-* layer without backend changes.

- source: docs/superpowers/plans/2026-06-12-operator-console-redesign.md
- implements: Phase 7 / DEC-0lu / REQ-operator-console-redesign
- design spec: (none — front-end-only redesign plan)

### Memory Display + Auto-Summary UX — Implementation Plan

Surfaces the server's auto-summary in the web console with summary-forward rows, a segmented Summary/Content/Meta detail panel, and sanitized markdown rendering.

- source: docs/superpowers/plans/2026-06-26-memory-display-summary-ux.md
- implements: Phase 7 / DEC-0lu / REQ-memory-display-ux
- design spec: docs/superpowers/specs/2026-06-26-memory-display-summary-ux-design.md

### Vitest 4 Browser-Mode UI Test Unification — Implementation Plan

Replaces the per-file vitest-environment split with a node tier plus a real-Chromium browser tier, migrating sanitizer + component tests to vitest-browser-svelte.

- source: docs/superpowers/plans/2026-06-27-vitest-browser-mode-ui-test-unification.md
- implements: Phase 7 / DEC-0lu / REQ-ui-test-unification
- design spec: docs/superpowers/specs/2026-06-27-vitest-browser-mode-ui-test-unification-design.md

### engram Documentation Site — Implementation Plan

Stands up an Astro Starlight docs site under docs-site/, deployed as static output to Cloudflare Workers Static Assets via a dedicated GitHub Actions workflow.

- source: docs/superpowers/plans/2026-06-09-docs-site-astro-starlight-cloudflare.md
- implements: Phase 7 / DEC-ttb / REQ-docs-site
- design spec: docs/superpowers/specs/2026-06-09-docs-site-astro-starlight-cloudflare-design.md

### engram docs-site landing-page redesign — Implementation Plan

Replaces the dead-end splash homepage with a custom StarlightPage landing that keeps the sidebar, routes via clickable cards, and reads as a product entrypoint.

- source: docs/superpowers/plans/2026-06-13-docs-site-landing-redesign.md
- implements: Phase 7 / DEC-ttb / REQ-docs-landing-redesign
- design spec: (none — landing-page redesign plan)

### engram Brand System — Implementation Plan

Applies the locked engram brand identity (memory-trace 'e' mark + neural-violet #6E56CF) across the operator console and docs site, and upstyles the console.

- source: docs/superpowers/plans/2026-06-13-engram-brand-identity.md
- implements: Phase 7 / DEC-ttb / REQ-brand-identity
- design spec: docs/superpowers/specs/2026-06-13-engram-brand-identity-design.md

### Relocate memory-curator into engram — Implementation Plan

Moves the memory-curator client plugin from fzymgc-house-skills into engram as a bundled skill-plugin under skill/engram/, rebranded to engram, then removes it from the old repo.

- source: docs/superpowers/plans/2026-06-02-relocate-memory-curator-into-engram.md
- implements: Phase 7 / (distribution) / REQ-relocate-memory-curator
- design spec: docs/superpowers/specs/2026-06-02-relocate-memory-curator-into-engram-design.md

---

## Phase 8 — Connect Observe-Lane Auth Hardening (DEFERRED)

Implements: REQ-connect-auth-posture (DEFERRED — R1–R4; no locked decisions — forward work)

### engram web UI — cookie/OIDC auth lane (Go BFF) — Implementation Plan

Makes the Connect EngramService a cookie-authenticated, headless-by-default surface via an OIDC login lane, sealed session cookie, cookie-to-Subject resolver, mount-gating, and observability parity.

- source: docs/superpowers/plans/2026-06-09-engram-web-ui-cookie-oidc-auth-lane.md
- implements: Phase 8 (deferred) / REQ-connect-auth-posture (R1–R4)
- design specs: docs/superpowers/specs/2026-06-09-engram-web-ui-design.md,
  docs/superpowers/specs/2026-06-09-connect-auth-posture-addendum.md
