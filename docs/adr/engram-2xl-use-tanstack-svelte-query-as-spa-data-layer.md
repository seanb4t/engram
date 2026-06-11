<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-2xl; do not edit manually; use `/adr update engram-2xl` -->

# Use @tanstack/svelte-query as the SPA data layer

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** engram-2xl
**Deciders:** Sean

## Context

The operator-console SPA calls five ConnectRPC read endpoints with loading/error/empty states, stale-time caching, and stable query keys that compose with pagination + filter params. SvelteKit load() is server-side and unavailable in an adapter-static SSR-off SPA (engram-0lu); hand-rolled stores would re-implement dedup/cache/invalidation.

## Decision

Adopt @tanstack/svelte-query as the sole async data layer. Every RPC call is a query with a stable composite key (e.g. [rpc, scope, filters, limit, offset]); loading/error/empty handled declaratively; staleTime tuned per query. The connect-es client stays a thin singleton that svelte-query wraps.

## Rationale

- load() is unavailable with adapter-static + SSR=false (engram-0lu); client-side fetching is the only option.\n- Query keys including scope/filters/limit/offset make cache invalidation on filter/page change automatic.\n- Built-in states eliminate per-component state machines across five views.\n- staleTime avoids redundant refetches on back-navigation.\n- Scales as the surface grows observe->correct->author.

## Alternatives Considered

**Hand-rolled Svelte stores** — no dep but re-implements dedup/loading/stale/invalidation; grows complex. Rejected.\n**SvelteKit load()** — first-class ergonomics but server-side only; structurally incompatible with adapter-static SSR-off. Rejected.

## Consequences

Positive: declarative states without boilerplate; stable cache; instant cached back-navigation. Negative: ~15kB bundle; query-key shape must stay consistent (drift = silent cache miss). Neutral: connect-es client remains a thin singleton.
