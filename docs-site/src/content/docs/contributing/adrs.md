---
title: Architecture Decision Records
description: Index of engram Architecture Decision Records, linking to the canonical source in the repository.
---

Architecture Decision Records (ADRs) capture significant design choices made during engram's development. They are produced via the `/adr` and `capture-adrs` workflow — the source of truth is `bd decision` records, and the files in `docs/adr/` are rendered from those records (do not edit them manually).

The list below links directly to the canonical ADR files in the repository. Do not copy or embed ADR bodies here — the files carry a "do not edit manually" banner and are regenerated on re-render.

## ADRs

- [Dedicated store\_discovery/search\_discovery tools, not overloaded store\_memory](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-0gy-dedicated-store-discovery-search-discovery-tools-not-overloa.md)
- [SvelteKit adapter-static SPA vendored via go:embed, SSR dropped](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-0lu-sveltekit-adapter-static-spa-vendored-via-go-embed-ssr-dropp.md)
- [Represent authz Subject as a sealed Go interface](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-12c-represent-authz-subject-as-sealed-go-interface.md)
- [Deploy docs-site via an in-repo GitHub Actions wrangler workflow](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-1w7-deploy-docs-site-via-repo-github-actions-wrangler-workflow.md)
- [Trust sealed cookie sub until session TTL; defer per-request IdP refresh](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-1xv-trust-sealed-cookie-sub-until-session-ttl-defer-per-request.md)
- [Discovery is a 5th category in the single Memory collection](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-2bv-discovery-is-5th-category-single-memory-collection.md)
- [Graceful decay over binary staleness for discovery trust](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-3l0-graceful-decay-over-binary-staleness-discovery-trust.md)
- [engram plugin ships no bundled MCP server; /engram-setup is the sole registration path](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md)
- [Adopt ConnectRPC and protobuf/buf for the web UI API](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-8xe-adopt-connectrpc-and-protobuf-buf-web-ui-api.md)
- [Embed the BFF in the engram Go binary, not a Node runtime](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-bgj-embed-bff-engram-go-binary-not-node-runtime.md)
- [Enforce per-actor authorization in the store layer, not in handlers](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md)
- [Export telemetry via OTLP only; omit a Prometheus scrape endpoint](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-dwi-export-telemetry-via-otlp-only-omit-prometheus-scrape-endpoi.md)
- [shadcn-svelte (on bits-ui) + Tailwind v4 as the component layer, re-themed](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-e38-shadcn-svelte-bits-ui-tailwind-v4-as-component-layer-re-them.md)
- [Instrument at three seams: HTTP, MCP method, and downstream clients](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-f7p-instrument-at-three-seams-http-mcp-method-and-downstream-cli.md)
- [Use the stable OIDC sub as the authorization key in a new owner field](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-hvg-use-stable-oidc-sub-as-authorization-key-new-owner-field.md)
- [Sharing grants read but never write (read/write gate asymmetry)](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-kyz-sharing-grants-read-but-never-write-read-write-gate-asymmetr.md)
- [Instrument MCP tools via AddReceivingMiddleware, not per-handler](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-tdk-instrument-mcp-tools-via-addreceivingmiddleware-not-per-hand.md)
- [Deploy docs-site via Workers Static Assets without an SSR adapter](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-ttb-deploy-docs-site-via-workers-static-assets-without-ssr-adapt.md)
- [Host the docs site inside the engram monorepo at docs-site/](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-u5h-host-docs-site-inside-engram-monorepo-at-docs-site.md)
- [Stateless encrypted-cookie session, no server-side store](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-u9v-stateless-encrypted-cookie-session-no-server-side-store.md)
- [Telemetry is never a hard server startup dependency](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-uxh-telemetry-is-never-hard-server-startup-dependency.md)
- [Return 404 not-found for unauthorized id-addressed operations](https://github.com/seanb4t/engram/blob/main/docs/adr/engram-xa6-return-404-not-found-unauthorized-id-addressed-operations.md)

## Generated index

The file [`docs/adr/README.md`](https://github.com/seanb4t/engram/blob/main/docs/adr/README.md) in the repository is the bd-generated ADR index — it is not an ADR itself.
