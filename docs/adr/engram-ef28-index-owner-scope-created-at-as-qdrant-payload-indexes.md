<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-ef28; do not edit manually; use `/adr update engram-ef28` -->

# Index owner/scope/created_at as Qdrant payload indexes

**Date:** 2026-06-30
**Status:** Accepted
**Decision:** engram-ef28
**Deciders:** Sean

## Context

engram's List/ListScheduled/ListScopes scrolled up to scanCap=1000 points and filtered owner, scope, and created_at in-memory. This made authz filtering O(scanCap) regardless of result size, silently truncated any scope exceeding 1000 readable records (the `approximate` flag is the system admitting it cannot report a true total), and prevented server-side date-range queries. Qdrant supports keyword and datetime payload indexes that enable exact Count, server-side DatetimeRange filtering, and order_by — but only once the indexes exist. The stored RFC3339 created_at strings parse natively into a datetime index with no payload re-stamping.

## Decision

Create keyword payload indexes on `owner` (is_tenant=true) and `scope`, and a datetime payload index on `created_at`, idempotently on every boot inside `ensureCollection`. Retire scanCap and the `approximate` flag for List; an exact `Count(filter)` replaces them.

## Rationale

- scanCap=1000 is an arbitrary ceiling that makes the system dishonest about totals once any scope grows past it — the `approximate` flag is an admission of this structural gap.
- Authz correctness requires the owner filter to be an index lookup, not a scan bounded by a fixed cap that could drop matching records.
- Qdrant idempotent index creation (treat already-exists as success, any other error as fatal) means existing deployments self-heal on next boot with no operator action and no data migration.
- All three indexes are needed together: owner for tenant colocality, scope for the must-condition on every recall path, created_at for the date-window and order_by.

## Alternatives Considered

- **Keep scroll-to-cap, status quo (rejected):** no index management and simpler store code, but authz filter is O(scanCap) not O(result), it silently truncates past 1000, the approximate flag admits the system cannot report true totals, and no server-side date range is possible.
- **Full server-side with Qdrant payload indexes — Approach A (chosen):** exact Count, server-side order_by created_at desc, DatetimeRange filter, authz that scales with the index rather than scanCap, and retirement of the approximate flag — at the cost of idempotent index creation on every boot.

## Consequences

- Positive: List returns exact totals always (approximate permanently false, deprecated); recall scales with result cardinality rather than a fixed 1000-row scan; date-window queries and server-side order_by become possible with no data migration.
- Negative: ensureCollection must handle Qdrant index-creation errors explicitly (a non-idempotent error fails startup); index backfill on first boot after deploy adds latency proportional to collection size.
- Neutral: discovery-scope records in the same collection gain the indexes for free; pre-isolation records missing the owner key remain invisible, consistent with existing behavior.
