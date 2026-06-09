<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-0lu; do not edit manually; use `/adr update engram-0lu` -->

# SvelteKit adapter-static SPA vendored via go:embed, SSR dropped

**Date:** 2026-06-09
**Status:** Accepted
**Decision:** engram-0lu
**Deciders:** Sean

## Context

The frontend framework choice determines the binary's embedding strategy, the BFF's responsibilities, and the client/server data-fetching model across all three phases. Alternatives considered: SvelteKit with SSR enabled, and Astro with Svelte islands.

## Decision

Build the frontend with SvelteKit adapter-static as a client-routed SPA (via a fallback page), vendored into the engram binary via go:embed; SSR and SvelteKit's server half are dropped entirely.

## Rationale

- A static bundle is the only frontend model compatible with go:embed in the Go binary — SSR requires a Node process at serve time.\n- The console is an authenticated internal tool with no SEO, no public surface, and stateful users; SSR's first-paint and crawlability benefits do not apply.\n- A client-side connect-query layer is a more natural fit for live-updating memory search than server-side form actions.\n- Astro's islands model is content-first and increasingly awkward for a CRUD console as interactivity grows.

## Alternatives Considered

**SvelteKit with SSR enabled** — best ergonomics + first paint, but requires a Node runtime at serve time, conflicting with the Go-binary-only deployment; SSR benefits are irrelevant for an authed internal console. Rejected. **Astro + Svelte islands** — also vendorable, but its content-first islands model fights a stateful CRUD console as interactivity grows. Rejected.

## Consequences

Positive: go build / goreleaser need no Node at runtime (the SPA is a committed, embedded static artifact); the typed connect-es client makes API contract drift a frontend compile error. Negative: SvelteKit load/form-actions ergonomics are unavailable (client-side data fetching only); built SPA assets are committed and require a vendored-asset CI drift check. Neutral: task ui:build regenerates the committed SPA artifacts.
