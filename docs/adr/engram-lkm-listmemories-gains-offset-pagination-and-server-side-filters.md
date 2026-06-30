<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-lkm; do not edit manually; use `/adr update engram-lkm` -->

# ListMemories gains offset pagination and server-side filters

**Date:** 2026-06-11
**Status:** Superseded by engram-1frj
**Decision:** engram-lkm
**Deciders:** Sean

## Context

The v1 Observe pane needs paginated listing with left-rail category/visibility filters. Today ListMemoriesRequest is {scope, limit} only; no offset, no filters, and the response has no total. The footer needs an accurate 'N of total' that reflects active filters. Choices: where to filter (client vs server) and which pagination model (offset vs cursor).

## Decision

Extend ListMemoriesRequest with uint64 offset, repeated string categories, string visibility; extend ListMemoriesResponse with uint64 total and bool approximate. The store applies filters server-side, returns the pre-truncation matched count, clamps to an empty page when offset>=total. Additive fields keep buf breaking green and leave the MCP path unaffected.

## Rationale

- Client-side filtering produces inaccurate totals when combined with server pagination — server-side filtering is the only honest-footer model.\n- Offset pagination is sufficient within the existing scanCap=1000 ceiling for the single-operator use case.\n- Additive proto fields are backward-compatible; the MCP/CLI ListMemories path is unaffected.\n- The approximate flag honestly signals the scanCap boundary.

## Alternatives Considered

**Client-side filtering of server pages** — no backend change, but 'showing 3 of 50' lies (50 was unfiltered); accurate totals need fetching everything. Rejected.\n**Cursor pagination** — stable under concurrent writes, but needs cursor tokens, prev-history, no page jumping, more store complexity. Deferred.

## Consequences

Positive: footer count accurate for any filter; non-breaking proto (MCP/CLI unaffected); store stays scroll-and-slice (no cursor state). Negative: offset can duplicate/skip records if data changes between page fetches; approximate needs a UI affordance. Neutral: cursor pagination + free-text server filters deferred.

## References

- Superseded by: engram-1frj
