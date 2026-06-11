<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-vxk; do not edit manually; use `/adr update engram-vxk` -->

# SPA-fallback static handler: serve index.html for client routes

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** engram-vxk
**Deciders:** Sean

## Context

The engram binary serves the SPA under /ui/ from an embedded FS via http.FileServer. A client-routed adapter-static SPA needs any /ui/* path with no real asset to return index.html (200), not 404, so the client router can handle it. The current StaticHandler (internal/webauth/static.go) does not.

## Decision

Wrap StaticHandler with SPA-fallback logic: if the embedded FS has the requested file, serve it; if the path has no file extension (a client route), serve index.html with 200; else 404. A unit test covers the fallback-vs-404 boundary.

## Rationale

- Client-routed SPAs require an index.html fallback for refresh + deep links — a structural requirement of adapter-static + client routing (engram-0lu).\n- The file-extension boundary is explicit and testable.\n- 404 for genuinely missing assets preserves correct behavior.\n- The Go binary is the only server; no nginx/CDN fallback layer exists.

## Alternatives Considered

**Hash-based routing (#/observe)** — any static server works, but hash URLs aren't server-shareable, non-standard, incompatible with SvelteKit default routing. Rejected.\n**Leave http.FileServer as-is** — refresh on /ui/observe 404s; deep links broken; incompatible with URL-driven state. Rejected.

## Consequences

Positive: refresh + deep links resolve correctly; URL-driven state is only viable with this in place. Negative: the extension heuristic can misclassify extensionless asset paths (rare for Vite output). Neutral: change confined to static.go + its test.
