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
| [engram-tdk](engram-tdk-instrument-mcp-tools-via-addreceivingmiddleware-not-per-hand.md) | 2026-06-07 | Accepted | Instrument MCP tools via AddReceivingMiddleware, not per-handler |
| [engram-dwi](engram-dwi-export-telemetry-via-otlp-only-omit-prometheus-scrape-endpoi.md) | 2026-06-07 | Accepted | Export telemetry via OTLP only; omit a Prometheus scrape endpoint |
| [engram-f7p](engram-f7p-instrument-at-three-seams-http-mcp-method-and-downstream-cli.md) | 2026-06-07 | Accepted | Instrument at three seams: HTTP, MCP method, and downstream clients |
| [engram-uxh](engram-uxh-telemetry-is-never-hard-server-startup-dependency.md) | 2026-06-07 | Accepted | Telemetry is never a hard server startup dependency |
| [engram-cgb](engram-cgb-enforce-per-actor-authorization-store-layer-not-handlers.md) | 2026-06-06 | Accepted | Enforce per-actor authorization in the store layer, not in handlers |
| [engram-hvg](engram-hvg-use-stable-oidc-sub-as-authorization-key-new-owner-field.md) | 2026-06-06 | Accepted | Use the stable OIDC sub as the authorization key in a new owner field |
| [engram-kyz](engram-kyz-sharing-grants-read-but-never-write-read-write-gate-asymmetr.md) | 2026-06-06 | Accepted | Sharing grants read but never write (read/write gate asymmetry) |
| [engram-xa6](engram-xa6-return-404-not-found-unauthorized-id-addressed-operations.md) | 2026-06-06 | Accepted | Return 404 not-found for unauthorized id-addressed operations |
| [engram-0gy](engram-0gy-dedicated-store-discovery-search-discovery-tools-not-overloa.md) | 2026-06-06 | Accepted | Dedicated store_discovery/search_discovery tools, not overloaded store_memory |
| [engram-2bv](engram-2bv-discovery-is-5th-category-single-memory-collection.md) | 2026-06-06 | Accepted | Discovery is a 5th category in the single Memory collection |
| [engram-3l0](engram-3l0-graceful-decay-over-binary-staleness-discovery-trust.md) | 2026-06-06 | Accepted | Graceful decay over binary staleness for discovery trust |
| [engram-50b](engram-50b-engram-plugin-ships-no-bundled-mcp-server-engram-setup-is-so.md) | 2026-06-03 | Accepted | engram plugin ships no bundled MCP server; /engram-setup is the sole registration path |

<!-- END INDEX -->
