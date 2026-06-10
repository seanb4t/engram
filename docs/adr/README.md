<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Architecture Decision Records

Each ADR is backed by a `bd` decision record (the bead id prefixes the
filename). ADRs are generated and indexed by the `dev-flow:capture-adrs`
workflow; edit the bead, then re-render — do not hand-edit the rendered files.

<!-- BEGIN INDEX -->

| ADR | Date | Status | Title |
|-----|------|--------|-------|
| [engram-u5h](engram-u5h-host-docs-site-inside-engram-monorepo-at-docs-site.md) | 2026-06-10 | Accepted | Host the docs site inside the engram monorepo at docs-site/ |
| [engram-ttb](engram-ttb-deploy-docs-site-via-workers-static-assets-without-ssr-adapt.md) | 2026-06-10 | Accepted | Deploy docs-site via Workers Static Assets without an SSR adapter |
| [engram-1xv](engram-1xv-trust-sealed-cookie-sub-until-session-ttl-defer-per-request.md) | 2026-06-10 | Accepted | Trust sealed cookie sub until session TTL; defer per-request IdP refresh |
| [engram-1w7](engram-1w7-deploy-docs-site-via-repo-github-actions-wrangler-workflow.md) | 2026-06-10 | Accepted | Deploy docs-site via an in-repo GitHub Actions wrangler workflow |
| [engram-u9v](engram-u9v-stateless-encrypted-cookie-session-no-server-side-store.md) | 2026-06-09 | Accepted | Stateless encrypted-cookie session, no server-side store |
| [engram-e38](engram-e38-shadcn-svelte-bits-ui-tailwind-v4-as-component-layer-re-them.md) | 2026-06-09 | Accepted | shadcn-svelte (on bits-ui) + Tailwind v4 as the component layer, re-themed |
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
| [engram-hvg](engram-hvg-use-stable-oidc-sub-as-authorization-key-new-owner-field.md) | 2026-06-06 | Accepted | Use the stable OIDC sub as the authorization key in a new owner field |
| [engram-cgb](engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md) | 2026-06-06 | Accepted | Enforce per-actor authorization in the store layer, not in handlers |
| [engram-3l0](engram-3l0-graceful-decay-over-binary-staleness-discovery-trust.md) | 2026-06-06 | Accepted | Graceful decay over binary staleness for discovery trust |
| [engram-2bv](engram-2bv-discovery-is-5th-category-single-memory-collection.md) | 2026-06-06 | Accepted | Discovery is a 5th category in the single Memory collection |
| [engram-0gy](engram-0gy-dedicated-store-discovery-search-discovery-tools-not-overloa.md) | 2026-06-06 | Accepted | Dedicated store_discovery/search_discovery tools, not overloaded store_memory |
| [engram-50b](engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md) | 2026-06-03 | Accepted | engram plugin ships no bundled MCP server; /engram-setup is the sole registration path |
<!-- END INDEX -->
