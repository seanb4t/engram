# Decisions (ADRs) — merge-adrs companion set

Synthesized from 31 fine-grained companion ADRs, all **LOCKED** (precedence 0,
high confidence). These refine or implement scopes governed at a coarser grain by
the 25 baseline locks in `.planning/PROJECT.md`. Each entry preserves the decision
in intent with full source provenance. Cross-refs to out-of-set ids are dangling
(traceability only, see context.md); the only in-set edges are `8q3 → u9v` and
`m4s8 → ddiw` (no cycles).

---

## Discovery Tools & Trust

### DEC-0gy — Dedicated store_discovery/search_discovery tools, not overloaded store_memory
- source: docs/adr/engram-0gy-dedicated-store-discovery-search-discovery-tools-not-overloa.md
- status: LOCKED
- decision: Introduce dedicated `store_discovery`/`search_discovery` MCP tools that enforce
  discovery-specific invariants rather than overloading the curated
  `store_memory`/`search_memory` contract.
- scope: store_discovery, search_discovery, store_memory, search_memory, discovery records, citations, MCP tool signatures

### DEC-3l0 — Graceful decay over binary staleness for discovery trust
- source: docs/adr/engram-3l0-graceful-decay-over-binary-staleness-discovery-trust.md
- status: LOCKED
- decision: The server stores and surfaces citation pins and `created_at` as raw aging signals
  rather than computing a freshness verdict, leaving trust judgment to the repo-aware agent.
- scope: discovery records, search_discovery, citation pins, created_at, freshness/staleness model

---

## Memory Summary & Update Contract

### DEC-4y7p — Explicit-first memory summary with offline operator auto-fill
- source: docs/adr/engram-4y7p-explicit-first-memory-summary-offline-operator-auto-fill.md
- status: LOCKED
- decision: Submitters may author memory summaries at write time; missing ones are filled by an
  offline operator sweep (`engram summarize-missing`) via `FillSummary`, never on the write path.
- scope: memory summary field, summary_source, store_memory write path, engram summarize-missing, FillSummary, ENGRAM_SUMMARY_MODEL, recall

### DEC-ddiw — Reject update_memory on content change with unaddressed client summary
- source: docs/adr/engram-ddiw-reject-update-memory-content-change-unaddressed-client-summa.md
- status: LOCKED
- decision: Atomically reject an `update_memory` write when content changes but an existing
  client-authored summary is left unaddressed; auto-clear an existing auto summary instead.
- scope: update_memory, content field, summary field, client vs auto summary provenance, presence-signal (*string), FetchForUpdate owner gate

---

## Rules

### DEC-d386 — Session-start surfaces rules as a progressive-disclosure index
- source: docs/adr/engram-d386-session-start-surfaces-rules-as-progressive-disclosure-index.md
- status: LOCKED
- decision: The session-start hook renders a one-line-per-rule index via `list_rules` with full
  rule text fetched on demand through `get_memory`, keeping context cost flat regardless of rule count.
- scope: session-start-memory-recall hook, list_rules, rule:repo:* scope, rule:project:* scope, ENGRAM_PROJECT, get_memory, progressive-disclosure index
- refines: engram-iedk, engram-ambu (baseline rule locks)

### DEC-m4s8 — Reject malformed rule summaries; never silently normalize
- source: docs/adr/engram-m4s8-reject-malformed-rule-summaries-newline-oversize-cleared-nev.md
- status: LOCKED
- decision: `store_rule` and the `update_memory` rule-guard reject any rule summary containing a
  newline, exceeding 256 bytes, or being cleared, rather than silently normalizing it.
- scope: store_rule, update_memory rule-guard, validateRuleSummary, rule summary, rules index entry
- refines: DEC-ddiw (in-set), engram-ambu

---

## Temporal / Scheduled Recall

### DEC-ufz — Soft-hide expired records at recall; opt-in prune-expired for storage reclaim
- source: docs/adr/engram-ufz-soft-hide-expired-records-at-recall-opt-prune-expired-storag.md
- status: LOCKED
- decision: Expired records are soft-hidden by the Qdrant recall gate and never auto-destroyed;
  storage is reclaimed only by the explicit, manually-run `engram prune-expired` operator command.
- scope: expired records, recall gate, Qdrant, engram prune-expired, list_scheduled, get_memory, pull-only architecture
- refines: engram-y1g (baseline temporal gate lock)

### DEC-c0m — Inject Store clock via WithClock option; keep public signatures stable
- source: docs/adr/engram-c0m-inject-store-clock-via-withclock-option-keep-public-signatur.md
- status: LOCKED
- decision: `Store` gains an unexported `now` func field defaulting to `time.Now`, injectable via
  a `WithClock` functional option, so `Search` and `List` read the clock internally without
  changing public signatures.
- scope: internal/store, Store.Search, Store.List, WithClock option, clock injection, recall gate, functional options
- refines: engram-y1g

---

## Config Validation

### DEC-wtw — Keep config.Load assembly-only; validate via a separate Config.Validate()
- source: docs/adr/engram-wtw-keep-config-load-assembly-only-validate-via-separate-config.md
- status: LOCKED
- decision: Keep `config.Load` assembly-only (never validating operator values) and check
  well-formedness via a separate pure `Config.Validate()` on the startup/store-construction path.
- scope: config.Load, Config.Validate, koanf pipeline, StoreFromEnvNoEnsure, EmbedderFromEnv, ENGRAM_ env layer
- cross-refs (dangling): engram-edv, engram-mbnw

### DEC-d24 — Validate data-plane fields only; listen_addr is a serve-local guard
- source: docs/adr/engram-d24-validate-data-plane-fields-only-listen-addr-is-serve-local-g.md
- status: LOCKED
- decision: `Config.Validate` checks only the five universal data-plane fields; `server.listen_addr`
  is validated by a serve-local guard so admin commands never trip on serve-specific config.
- scope: Config.Validate, qdrant.addr, qdrant.collection, embed.model, embed.dim, openai.base_url, server.listen_addr, runServe, admin commands
- refines: DEC-wtw

---

## Telemetry / Observability

### DEC-6gb — Instrument store/embed/auth with inline spans, not a decorator layer
- source: docs/adr/engram-6gb-instrument-store-embed-auth-inline-spans-not-decorator-layer.md
- status: LOCKED
- decision: Each public method in store, embed, and auth creates its own OpenTelemetry span inline
  via a package-level tracer, with per-operation metrics delegated to `internal/telemetry` helpers.
- scope: internal/store, internal/embed, internal/auth, internal/telemetry, OpenTelemetry spans, inline instrumentation
- refines: engram-dwi, engram-uxh (baseline telemetry locks)

### DEC-f7p — Instrument at three seams: HTTP, MCP method, and downstream clients
- source: docs/adr/engram-f7p-instrument-at-three-seams-http-mcp-method-and-downstream-cli.md
- status: LOCKED
- decision: Instrument all three request-path boundaries — HTTP (otelhttp + auth-failure
  middleware), the MCP method layer (`AddReceivingMiddleware`), and downstream embedder/Qdrant
  clients — since a single wrap would miss whole signal classes.
- scope: HTTP layer, otelhttp, MCP method dispatch, AddReceivingMiddleware, embedder client, Qdrant gRPC, auth-failure middleware
- refines: engram-dwi, engram-uxh

### DEC-tdk — Instrument MCP tools via AddReceivingMiddleware, not per-handler
- source: docs/adr/engram-tdk-instrument-mcp-tools-via-addreceivingmiddleware-not-per-hand.md
- status: LOCKED
- decision: All MCP tool calls are instrumented from a single `srv.AddReceivingMiddleware` seam
  that records span, metrics, and structured log per `tools/call`, rather than annotating each
  of the 11 handlers.
- scope: MCP tools, AddReceivingMiddleware, tools/call, CallToolRequest, span/metrics/logging, caller actor/owner extraction
- refines: engram-dwi, engram-uxh; cross-refs (dangling): engram-ew7

### DEC-wot — Spans carry engram.owner only; exclude actor and email as PII
- source: docs/adr/engram-wot-spans-carry-engram-owner-only-exclude-actor-and-email-as-pii.md
- status: LOCKED
- decision: Span attributes carry only the opaque OIDC `sub` (`engram.owner`) and never `actor`,
  keeping email/username PII off trace backends and confined to trace-correlated structured log lines.
- scope: span attributes, engram.owner, actor, PII/data-minimization, trace backends, structured logs, OIDC sub
- refines: engram-dwi, engram-uxh

### DEC-7qd — Reuse OTel-standard env vars for sampler and export interval; no MEM_* equivalents
- source: docs/adr/engram-7qd-reuse-otel-standard-env-vars-sampler-and-export-interval-add.md
- status: LOCKED
- decision: Configure the OTel sampler and metric export interval exclusively via OTel-standard
  env vars templated in the Helm chart, with no `MEM_*` counterparts and no Go code change.
- scope: OTEL_TRACES_SAMPLER, OTEL_METRIC_EXPORT_INTERVAL, sampler, metric export interval, Helm chart observability block, config namespace convention
- refines: engram-dwi

### DEC-9tj — Inject k8s resource attributes via chart Downward API, not Go SDK detectors
- source: docs/adr/engram-9tj-inject-k8s-resource-attributes-via-chart-downward-api-not-go.md
- status: LOCKED
- decision: Kubernetes OTel resource attributes are injected by the Helm chart via the Downward
  API into `OTEL_RESOURCE_ATTRIBUTES`, keeping the binary deployment-agnostic with no k8s SDK detector.
- scope: OpenTelemetry resource attributes, Helm chart, Downward API, OTEL_RESOURCE_ATTRIBUTES, resource.WithFromEnv, Kubernetes deployment
- refines: engram-dwi

---

## BFF / Session / Auth (operator console)

### DEC-bgj — Embed the BFF in the engram Go binary, not a Node runtime
- source: docs/adr/engram-bgj-embed-bff-engram-go-binary-not-node-runtime.md
- status: LOCKED
- decision: Implement the backend-for-frontend (OIDC login/callback, session seal/unseal, static
  serving) inside the engram Go binary rather than a separate Node/SvelteKit runtime.
- scope: BFF, engram Go binary, OIDC login/callback, session seal/unseal, static asset serving, go-oidc, x/oauth2, Helm chart
- refines: engram-0lu, engram-8xe (baseline SPA/console locks)

### DEC-u9v — Stateless encrypted-cookie session, no server-side store
- source: docs/adr/engram-u9v-stateless-encrypted-cookie-session-no-server-side-store.md
- status: LOCKED
- decision: After OIDC login engram seals `{access, refresh, sub}` into an httpOnly, SameSite,
  AES-GCM encrypted cookie with no server-side session store.
- scope: OIDC session, encrypted cookie, AES-GCM, Connect interceptor, access token refresh, MEM_UI_COOKIE_KEY, BFF
- refines: engram-g37x (baseline session-cookie lock)

### DEC-8q3 — Session cookie seals only sub+expiry; no OIDC tokens stored client-side
- source: docs/adr/engram-8q3-session-cookie-seals-only-sub-expiry-no-oidc-tokens-stored-c.md
- status: LOCKED
- decision: The session cookie seals only `{sub, expiry}`; no OIDC access or refresh token is
  stored client-side, deferring token custody to the server-side write phase.
- scope: session cookie, OIDC access/refresh token, sub claim, session TTL, Session type, read-only v1 lane, server-side token store
- refines: engram-g37x; cross-refs: engram-u9v (in-set), engram-1xv (dangling)
- note: Scopes the read-only v1 lane. DEC-u9v describes the eventual write-phase custody model
  (seals access+refresh); DEC-8q3 governs the current v1 read lane (seals only sub+expiry).
  Complementary, staged — not contradictory. See INGEST-CONFLICTS.md INFO.

---

## SPA / Operator Console Internals

### DEC-2xl — Use @tanstack/svelte-query as the SPA data layer
- source: docs/adr/engram-2xl-use-tanstack-svelte-query-as-spa-data-layer.md
- status: LOCKED
- decision: Adopt `@tanstack/svelte-query` as the operator-console SPA's sole async data layer,
  wrapping the connect-es client with stable composite query keys.
- scope: operator-console SPA, @tanstack/svelte-query, ConnectRPC read endpoints, connect-es client, query keys, caching, pagination/filters
- refines: engram-0lu

### DEC-c4y — Drive SPA shell state via URL parameters
- source: docs/adr/engram-c4y-drive-spa-shell-state-via-url-parameters.md
- status: LOCKED
- decision: Encode SPA shell state (scope, category/visibility filters, query, selected id) in
  the URL query string, making it the single source of truth with stateless panes.
- scope: SPA shell state, URL query parameters, Observe three-pane view, filters, svelte-query cache keys, SPA-fallback handler, deep-linking
- refines: engram-0lu

### DEC-3nas — Render user memory content via marked + DOMPurify allowlist
- source: docs/adr/engram-3nas-render-user-memory-content-via-marked-dompurify-allowlist.md
- status: LOCKED
- decision: Render user-authored memory content by piping it through `marked` then `DOMPurify`
  with a tight tag/attr allowlist and link-hardening hook as the sole `{@html}` entry point.
- scope: memory content rendering, marked, DOMPurify, ui/src/lib/markdown.ts, renderMarkdown, Svelte SPA XSS sanitization, {@html}
- refines: engram-0lu; cross-refs (dangling): engram-kyz

### DEC-vxk — SPA-fallback static handler: serve index.html for client routes
- source: docs/adr/engram-vxk-spa-fallback-static-handler-serve-index-html-client-routes.md
- status: LOCKED
- decision: Wrap `StaticHandler` with SPA-fallback logic that serves `index.html` (200) for
  extensionless `/ui/*` client routes, real files when present, and 404 otherwise.
- scope: StaticHandler, internal/webauth/static.go, SPA /ui/ serving, embedded FS, http.FileServer, index.html fallback, client-side routing
- refines: engram-0lu

### DEC-4ag — Gate dashboard category-breakdown bar on a listScopes API extension
- source: docs/adr/engram-4ag-gate-dashboard-category-breakdown-bar-listscopes-api-extensi.md
- status: LOCKED
- decision: Drop the dashboard category-breakdown bar and defer per-category counts until a future
  `listScopes` API extension provides real data.
- scope: operator-console dashboard, listScopes RPC, ScopeChip, category-breakdown bar, ui/src/routes/+page.svelte, per-category counts
- refines: engram-0lu, engram-8xe

### DEC-lzz — Adopt shadcn semantic tokens; retire bespoke eg-*/--cat-* layer
- source: docs/adr/engram-lzz-adopt-shadcn-semantic-tokens-retire-bespoke-eg-cat-layer.md
- status: LOCKED
- decision: Replace `app.css` with shadcn's standard semantic token set and remove bespoke `eg-*`
  utilities and `--cat-*` variables, retaining only four category accent hues within the shadcn system.
- scope: shadcn semantic tokens, app.css, eg-* Tailwind utilities, --cat-* CSS variables, category accent hues, Tailwind v4, light/dark theming
- refines: engram-0lu; cross-refs (dangling): engram-e38

### DEC-no3 — Ship engram wordmark as outlined SVG paths, not a webfont
- source: docs/adr/engram-no3-ship-engram-wordmark-as-outlined-svg-paths-not-webfont.md
- status: LOCKED
- decision: The engram wordmark ships as outlined SVG paths embedded in the lockup asset and
  inlined via raw import, avoiding any webfont fetch, FOUT, and network dependency.
- scope: engram wordmark, brand lockup SVG, brand/engram-lockup.svg, ui/src/lib/assets, BrandMark.svelte, SvelteKit SPA, Astro/Starlight docs site
- refines: engram-0lu

---

## Test Tiers

### DEC-1h3k — Adopt two-tier vitest config: node + real-Chromium browser
- source: docs/adr/engram-1h3k-adopt-two-tier-vitest-config-node-real-chromium-browser.md
- status: LOCKED
- decision: Adopt a two-project vitest 4 config splitting pure-logic tests onto a node tier and
  DOM-touching tests onto a real-Chromium browser tier.
- scope: ui/vite.config.ts, vitest, DOMPurify sanitizer, bits-ui components, Playwright/Chromium browser mode, vitest-browser-svelte, node/browser test tiers
- refines: vitest-browser direction; cross-refs (dangling): engram-s2ao

### DEC-om5b — Node test tier on environment:node, drop happy-dom
- source: docs/adr/engram-om5b-node-test-tier-environment-node-drop-happy-dom.md
- status: LOCKED
- decision: Run the vitest node project on `environment:'node'` and drop happy-dom, falling back
  to retaining it only if a concrete logic test breaks on a missing DOM global.
- scope: vitest node project, environment:'node', happy-dom, logic tests, app.css node:fs test, vitest-setup.ts localStorage stub, mode-watcher
- refines: DEC-1h3k (the two-tier vitest decision)

---

## Docs Site

### DEC-u5h — Host the docs site inside the engram monorepo at docs-site/
- source: docs/adr/engram-u5h-host-docs-site-inside-engram-monorepo-at-docs-site.md
- status: LOCKED
- decision: Place the Astro Starlight docs site at `docs-site/` within the engram monorepo with
  tooling exemptions, rather than in a separate dedicated repository.
- scope: docs-site/, Astro Starlight, engram monorepo, tooling exemptions, .licenserc.yaml, .rumdl.toml, .yamlfmt, CI gates
- refines: engram-ttb (baseline docs-site lock)

### DEC-1w7 — Deploy docs-site via an in-repo GitHub Actions wrangler workflow
- source: docs/adr/engram-1w7-deploy-docs-site-via-repo-github-actions-wrangler-workflow.md
- status: LOCKED
- decision: Deploy the docs-site through a dedicated non-required GitHub Actions workflow using
  wrangler-action, keeping deploy config version-controlled and off the protect-main required checks.
- scope: docs-site, .github/workflows/docs-site.yaml, GitHub Actions, Cloudflare Workers, wrangler-action, protect-main ruleset, ci.yaml
- refines: engram-ttb

---

## Plugin Packaging

### DEC-50b — engram plugin ships no bundled MCP server; /engram-setup is the sole registration path
- source: docs/adr/engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md
- status: LOCKED
- decision: Remove the bundled `.mcp.json` from the engram plugin, making `/engram-setup` the
  single canonical MCP-server registration path via a user-scope `claude mcp add` command.
- scope: skill/engram plugin, .mcp.json, /engram-setup, plugin.json mcpServers, MCP server registration, claude mcp add
- cross-refs (dangling): 2026-06-02-generalize-engram-client-config, PR #16, v0.3.0
- note: NOT in conflict with the Phase-7 "bundled skill/engram plugin" baseline. The plugin
  remains vendored/bundled in-repo; 50b only removes the auto-registering `.mcp.json` config so
  that registration is done explicitly by `/engram-setup`. Different axes. See INGEST-CONFLICTS.md INFO.
