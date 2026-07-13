<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Architecture Decision Records

Each ADR is backed by a `bd` decision record (the bead id prefixes the
filename). ADRs are generated and indexed from their backing `bd` decision
records; edit the bead, then re-render — do not hand-edit the rendered files.
Beads was retired 2026-07-08, so this pipeline is now dead: ADRs dated after
that point are hand-authored Markdown instead, with no backing bead and no
render step.

<!-- BEGIN INDEX -->

| ADR | Date | Status | Title |
|-----|------|--------|-------|
| [engram-slr8](engram-slr8-stateless-sliding-session-reseal.md) | 2026-07-13 | Accepted | Stateless sliding-expiry session re-seal |
| [engram-iedk](engram-iedk-rules-are-always-shared-server-set-immutable-visibility-set.md) | 2026-07-06 | Accepted | Rules are always-shared with server-set immutable visibility; set_visibility rejects rules |
| [engram-d386](engram-d386-session-start-surfaces-rules-as-progressive-disclosure-index.md) | 2026-07-06 | Accepted | Session-start surfaces rules as a progressive-disclosure index, not full-content injection |
| [engram-m4s8](engram-m4s8-reject-malformed-rule-summaries-newline-oversize-cleared-nev.md) | 2026-07-06 | Accepted | Reject malformed rule summaries (newline/oversize/cleared); never silently normalize |
| [engram-zzq0](engram-zzq0-encode-short-id-as-10-char-crockford-base32.md) | 2026-07-06 | Accepted | Encode short_id as 10-char Crockford base32 |
| [engram-02ta](engram-02ta-resolve-short-id-at-handler-layer-not-inside-store-methods.md) | 2026-07-06 | Accepted | Resolve short_id at the handler layer, not inside store methods |
| [engram-zyhq](engram-zyhq-generic-param-map-passthrough-over-embedder-profiles-asymmet.md) | 2026-07-01 | Accepted | Generic param-map passthrough over embedder profiles for asymmetric/cloud embedders |
| [engram-ef28](engram-ef28-index-owner-scope-created-at-as-qdrant-payload-indexes.md) | 2026-06-30 | Accepted | Index owner/scope/created_at as Qdrant payload indexes |
| [engram-1frj](engram-1frj-boundary-id-set-cursor-half-open-date-window-recall.md) | 2026-06-30 | Accepted | Boundary id-set cursor with half-open date window for recall |
| [engram-g37x](engram-g37x-use-configurable-oidc-claim-as-record-owner-default-email.md) | 2026-06-29 | Accepted | Use configurable OIDC claim as record owner (default: email) |
| [engram-1h3k](engram-1h3k-adopt-two-tier-vitest-config-node-real-chromium-browser.md) | 2026-06-28 | Accepted | Adopt two-tier vitest config: node + real-Chromium browser |
| [engram-om5b](engram-om5b-node-test-tier-environment-node-drop-happy-dom.md) | 2026-06-28 | Accepted | Node test tier on environment:node, drop happy-dom |
| [engram-3nas](engram-3nas-render-user-memory-content-via-marked-dompurify-allowlist.md) | 2026-06-27 | Accepted | Render user memory content via marked + DOMPurify allowlist |
| [engram-4y7p](engram-4y7p-explicit-first-memory-summary-offline-operator-auto-fill.md) | 2026-06-26 | Accepted | Explicit-first memory summary with offline operator auto-fill |
| [engram-ambu](engram-ambu-recall-returns-summary-by-default-full-content-opt.md) | 2026-06-26 | Accepted | Recall returns summary by default with full-content opt-in |
| [engram-ddiw](engram-ddiw-reject-update-memory-content-change-unaddressed-client-summa.md) | 2026-06-26 | Accepted | Reject update_memory on content change with unaddressed client summary |
| [engram-wtw](engram-wtw-keep-config-load-assembly-only-validate-via-separate-config.md) | 2026-06-14 | Accepted | Keep config.Load assembly-only; validate via a separate Config.Validate() |
| [engram-d24](engram-d24-validate-data-plane-fields-only-listen-addr-is-serve-local-g.md) | 2026-06-14 | Accepted | Validate data-plane fields only; listen_addr is a serve-local guard |
| [engram-jgq](engram-jgq-unify-config-under-engram-prefix-via-koanf-internal-config.md) | 2026-06-14 | Accepted | Unify config under ENGRAM_ prefix via koanf internal/config |
| [engram-378](engram-378-name-embedder-connection-vars-by-protocol-not-implementation.md) | 2026-06-14 | Accepted | Name embedder connection vars by protocol, not implementation |
| [engram-irq](engram-irq-breaking-config-renames-ship-fatal-legacy-env-startup-guard.md) | 2026-06-14 | Accepted | Breaking config renames ship with a fatal legacy-env startup guard |
| [engram-no3](engram-no3-ship-engram-wordmark-as-outlined-svg-paths-not-webfont.md) | 2026-06-13 | Accepted | Ship engram wordmark as outlined SVG paths, not a webfont |
| [engram-4ag](engram-4ag-gate-dashboard-category-breakdown-bar-listscopes-api-extensi.md) | 2026-06-13 | Accepted | Gate dashboard category-breakdown bar on a listScopes API extension |
| [engram-lzz](engram-lzz-adopt-shadcn-semantic-tokens-retire-bespoke-eg-cat-layer.md) | 2026-06-12 | Accepted | Adopt shadcn semantic tokens; retire bespoke eg-*/--cat-* layer |
| [engram-y1g](engram-y1g-gate-recall-via-qdrant-filter-leave-get-memory-ungated.md) | 2026-06-12 | Accepted | Gate recall via Qdrant filter; leave get_memory ungated |
| [engram-90w](engram-90w-add-schedule-memory-list-scheduled-tools-keep-store-memory-w.md) | 2026-06-12 | Accepted | Add schedule_memory/list_scheduled tools; keep store_memory windowless |
| [engram-ufz](engram-ufz-soft-hide-expired-records-at-recall-opt-prune-expired-storag.md) | 2026-06-12 | Accepted | Soft-hide expired records at recall; opt-in prune-expired for storage reclaim |
| [engram-c0m](engram-c0m-inject-store-clock-via-withclock-option-keep-public-signatur.md) | 2026-06-12 | Accepted | Inject Store clock via WithClock option; keep public signatures stable |
| [engram-6gb](engram-6gb-instrument-store-embed-auth-inline-spans-not-decorator-layer.md) | 2026-06-11 | Accepted | Instrument store/embed/auth with inline spans, not a decorator layer |
| [engram-wot](engram-wot-spans-carry-engram-owner-only-exclude-actor-and-email-as-pii.md) | 2026-06-11 | Accepted | Spans carry engram.owner only; exclude actor and email as PII |
| [engram-7qd](engram-7qd-reuse-otel-standard-env-vars-sampler-and-export-interval-add.md) | 2026-06-11 | Accepted | Reuse OTel-standard env vars for sampler and export interval; add no MEM_* equivalents |
| [engram-9tj](engram-9tj-inject-k8s-resource-attributes-via-chart-downward-api-not-go.md) | 2026-06-11 | Accepted | Inject k8s resource attributes via chart Downward API, not Go SDK detectors |
| [engram-2xl](engram-2xl-use-tanstack-svelte-query-as-spa-data-layer.md) | 2026-06-11 | Accepted | Use @tanstack/svelte-query as the SPA data layer |
| [engram-c4y](engram-c4y-drive-spa-shell-state-via-url-parameters.md) | 2026-06-11 | Accepted | Drive SPA shell state via URL parameters |
| [engram-vxk](engram-vxk-spa-fallback-static-handler-serve-index-html-client-routes.md) | 2026-06-11 | Accepted | SPA-fallback static handler: serve index.html for client routes |
| [engram-lkm](engram-lkm-listmemories-gains-offset-pagination-and-server-side-filters.md) | 2026-06-11 | Superseded by engram-1frj | ListMemories gains offset pagination and server-side filters |
| [engram-8q3](engram-8q3-session-cookie-seals-only-sub-expiry-no-oidc-tokens-stored-c.md) | 2026-06-10 | Accepted | Session cookie seals only sub+expiry; no OIDC tokens stored client-side |
| [engram-u5h](engram-u5h-host-docs-site-inside-engram-monorepo-at-docs-site.md) | 2026-06-10 | Accepted | Host the docs site inside the engram monorepo at docs-site/ |
| [engram-ttb](engram-ttb-deploy-docs-site-via-workers-static-assets-without-ssr-adapt.md) | 2026-06-10 | Accepted | Deploy docs-site via Workers Static Assets without an SSR adapter |
| [engram-1xv](engram-1xv-trust-sealed-cookie-sub-until-session-ttl-defer-per-request.md) | 2026-06-10 | Superseded by engram-8q3 | Trust sealed cookie sub until session TTL; defer per-request IdP refresh |
| [engram-1w7](engram-1w7-deploy-docs-site-via-repo-github-actions-wrangler-workflow.md) | 2026-06-10 | Accepted | Deploy docs-site via an in-repo GitHub Actions wrangler workflow |
| [engram-u9v](engram-u9v-stateless-encrypted-cookie-session-no-server-side-store.md) | 2026-06-09 | Accepted | Stateless encrypted-cookie session, no server-side store |
| [engram-e38](engram-e38-shadcn-svelte-bits-ui-tailwind-v4-as-component-layer-re-them.md) | 2026-06-09 | Superseded by engram-lzz | shadcn-svelte (on bits-ui) + Tailwind v4 as the component layer, re-themed |
| [engram-bgj](engram-bgj-embed-bff-engram-go-binary-not-node-runtime.md) | 2026-06-09 | Accepted | Embed the BFF in the engram Go binary, not a Node runtime |
| [engram-8xe](engram-8xe-adopt-connectrpc-and-protobuf-buf-web-ui-api.md) | 2026-06-09 | Accepted | Adopt ConnectRPC and protobuf/buf for the web UI API |
| [engram-0lu](engram-0lu-sveltekit-adapter-static-spa-vendored-via-go-embed-ssr-dropp.md) | 2026-06-09 | Accepted | SvelteKit adapter-static SPA vendored via go:embed, SSR dropped |
| [engram-12c](engram-12c-represent-authz-subject-as-sealed-go-interface.md) | 2026-06-08 | Accepted | Represent authz Subject as a sealed Go interface |
| [engram-uxh](engram-uxh-telemetry-is-never-hard-server-startup-dependency.md) | 2026-06-07 | Accepted | Telemetry is never a hard server startup dependency |
| [engram-tdk](engram-tdk-instrument-mcp-tools-via-addreceivingmiddleware-not-per-hand.md) | 2026-06-07 | Accepted | Instrument MCP tools via AddReceivingMiddleware, not per-handler |
| [engram-f7p](engram-f7p-instrument-at-three-seams-http-mcp-method-and-downstream-cli.md) | 2026-06-07 | Accepted | Instrument at three seams: HTTP, MCP method, and downstream clients |
| [engram-dwi](engram-dwi-export-telemetry-via-otlp-only-omit-prometheus-scrape-endpoi.md) | 2026-06-07 | Accepted | Export telemetry via OTLP only; omit a Prometheus scrape endpoint |
| [engram-xa6](engram-xa6-return-404-not-found-unauthorized-id-addressed-operations.md) | 2026-06-06 | Accepted | Return 404 not-found for unauthorized id-addressed operations |
| [engram-kyz](engram-kyz-sharing-grants-read-but-never-write-read-write-gate-asymmetr.md) | 2026-06-06 | Accepted | Sharing grants read but never write (read/write gate asymmetry) |
| [engram-hvg](engram-hvg-use-stable-oidc-sub-as-authorization-key-new-owner-field.md) | 2026-06-06 | Superseded by engram-g37x | Use the stable OIDC sub as the authorization key in a new owner field |
| [engram-cgb](engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md) | 2026-06-06 | Accepted | Enforce per-actor authorization in the store layer, not in handlers |
| [engram-3l0](engram-3l0-graceful-decay-over-binary-staleness-discovery-trust.md) | 2026-06-06 | Accepted | Graceful decay over binary staleness for discovery trust |
| [engram-2bv](engram-2bv-discovery-is-5th-category-single-memory-collection.md) | 2026-06-06 | Accepted | Discovery is a 5th category in the single Memory collection |
| [engram-0gy](engram-0gy-dedicated-store-discovery-search-discovery-tools-not-overloa.md) | 2026-06-06 | Accepted | Dedicated store_discovery/search_discovery tools, not overloaded store_memory |
| [engram-50b](engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md) | 2026-06-03 | Accepted | engram plugin ships no bundled MCP server; /engram-setup is the sole registration path |

<!-- END INDEX -->
