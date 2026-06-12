<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-bj6; do not edit manually; use `/adr update engram-bj6` -->

# MCP transport at explicit configurable path (MEM_MCP_PATH) with console at root when UI enabled

**Date:** 2026-06-12
**Status:** Accepted
**Decision:** engram-bj6
**Deciders:** Sean Brandt

## Context

Until 0.6.x the MCP StreamableHTTP transport was the root catch-all (mux.Handle("/", mcpHandler)); the console was a sub-path under /ui/. With the web UI enabled this had three consequences: (1) a browser hitting the host root got a 401 "no bearer token" wall instead of the console; (2) the catch-all swallowed /, /favicon.ico and stray paths into the bearer gate as noisy 401s; (3) in gateway-fronted deployments the console hostname's root re-exposed the MCP transport, bypassing the gateway's policy/audit front door (still bearer-gated, so defense-in-depth, not a vuln). The go-sdk StreamableHTTPHandler dispatches on HTTP method + session-ID header and ignores the request path, so it answered at every path under the catch-all.

## Decision

Introduce MEM_MCP_PATH (flag --mcp-path, env-first). It defaults to /mcp in ALL modes (headless and UI-enabled) and must be an absolute path. The MCP transport is mounted at that exact path. When MCP is off the root, "/" is owned by a small handler: a browser GET of the bare root 302-redirects to /ui/ when the UI is enabled; every other request (non-GET, sub-paths, or any request when headless) returns 404 instead of the MCP bearer gate. The special value MEM_MCP_PATH=/ is an escape hatch that restores the legacy root catch-all (MCP answers every path) in either mode. Routing lives in a testable mountMCPRoutes(mux, mcpHandler, uiEnabled, mcpPath) seam; the Helm chart exposes memory.mcpPath (empty omits the env var).

## Rationale

An explicit /mcp endpoint lets gateway-fronted deployments keep MCP strictly behind the gateway path and make the console hostname a console only — no accidental second door to the transport. It also kills the 401 noise: stray paths and mis-targeted MCP clients get a clear 404 rather than a bearer-gate 401. A uniform /mcp default (rather than only-when-UI-enabled) gives one predictable, documented endpoint everywhere and a single escape hatch. Exact-path mounting is correct because the go-sdk handler ignores the path; a subtree (/mcp/) mount would trigger the ServeMux trailing-slash 301 that breaks MCP POSTs. The transport endpoint itself is never redirected: MCP is POST/SSE-based and clients do not follow redirects, so only browser GET / can be redirected — which is also why a headless /->/mcp redirect was rejected (it would break POST clients and there is no browser in headless to benefit). Connect API (/engram.v1.EngramService/), /auth/*, and /ui/ are explicit ServeMux patterns that win over "/" by longest-prefix, so the SPA's baseUrl:"/" Connect calls are unaffected.

## Alternatives Considered

(1) Keep headless as the root catch-all and only move MCP to /mcp when the UI is enabled — narrower breaking change, but two divergent routing models and /mcp would not be a stable documented endpoint in headless. Rejected for a uniform model. (2) Mount MCP at subtree /mcp/ — rejected: ServeMux 301-redirects POST /mcp to /mcp/, which MCP POST clients mishandle. (3) Redirect headless "/" -> /mcp — rejected: the MCP transport is POST/SSE and clients do not follow redirects, and headless has no browser, so the redirect helps nobody and breaks clients still POSTing to /. (4) Always also mount /mcp alongside the headless catch-all (no behavior change) — rejected as it leaves "/" re-exposing the transport, defeating purpose (3) of the change.

## Consequences

BREAKING for all installs: MCP previously answered at "/" (and every path); it now answers only at /mcp by default. UI-enabled and headless clients that POST to the host root must target /mcp, or set MEM_MCP_PATH=/ to restore the catch-all. This is signalled as a BREAKING CHANGE in the release (release-please / Conventional Commits). Gains: browsers landing on the host root get the console; MCP has a stable explicit endpoint; gateway deployments can scope the proxy to /mcp with no second transport door on the console hostname; bearer-gate 401 noise on stray paths is gone. The routing decision is unit-tested via mountMCPRoutes + resolveMCPPath; the Helm chart gained memory.mcpPath.
