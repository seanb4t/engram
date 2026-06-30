<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-1frj; do not edit manually; use `/adr update engram-1frj` -->

# Boundary id-set cursor with half-open date window for recall

**Date:** 2026-06-30
**Status:** Accepted
**Decision:** engram-1frj
**Deciders:** Sean

## Context

engram-lkm (2026-06-11) added offset pagination to ListMemories and explicitly deferred cursor pagination, justifying scanCap=1000 as sufficient for the single-operator use case. The MCP list_memory path needed deterministic O(limit)-per-page paging and date-windowed recall. Qdrant does not contractually guarantee stable ordering within an equal-created_at group, which makes naive cursor designs (resume by timestamp alone) prone to duplicates or skips when records share a timestamp — a real condition because bulk imports (e.g. the June-27 beads→engram migration) can produce same-second timestamps.

## Decision

Cursor is the default paging mode for the MCP list_memory path: an opaque base64url(JSON{c, seen}) token, resumed by order_by created_at desc with start_from=c, fetching limit+len(seen) records, dropping the seen ids, then taking limit. Offset pagination is retained for the operator-console UI (O(offset+limit), honest cost documented). The date window is half-open [created_after, created_before) using gte/lt, applied on all recall tools. MCP list_memory output gains a `next_cursor` field; because the tool already returns a structured {memories} object (not a bare array), this is an ADDITIVE change, not a breaking reshape. One encoder/decoder serves both the MCP cursor and the wire page_token.

## Rationale

- Dedup by id membership (not position) is the only correct approach when Qdrant's intra-group order is not contractually stable — position-based dedup would silently duplicate or skip records.
- The limit+len(seen) fetch rule is the critical invariant: without it, a timestamp group larger than limit exhausts the page with only already-seen ids and terminates traversal early.
- Half-open [created_after, created_before) lets contiguous day-windows tile without overlap and aligns with how agents express "everything before yesterday".
- created_after/created_before are named distinctly from not_before/not_after (the validity-window axis) to eliminate a correctness trap: the two axes are orthogonal filters and must not share naming.
- A single shared token encoder prevents two codecs diverging across the MCP and Connect surfaces.
- A new monotonic sequence field was rejected because it would force a data migration to stamp existing records, breaking the no-migration goal; the boundary id-set reuses the existing created_at datetime field.

## Alternatives Considered

- **Retain offset for MCP, extend engram-lkm (rejected):** no cursor token design and no output change, but O(offset+limit) per page, duplicates/skips under concurrent writes, and deep-paging cost grows unboundedly.
- **Qdrant point-ID continuation cursor / ScrollPoints.Offset (rejected):** O(limit) per page and no extra field, but ScrollPoints.Offset is a point-ID continuation, not a skip count; it does not compose with order_by created_at for equal-timestamp groups and is unstable across Qdrant node rebalances.
- **New monotonic sequence field (rejected):** trivially unique cursor key with no tie-break logic, but requires a data migration to stamp existing records and adds a data-model field — breaks the no-migration goal.
- **Boundary id-set cursor on created_at (chosen):** uses the existing datetime field (no migration), order-independent dedup by explicit id membership, one encoder shared by MCP cursor and wire page_token, O(limit) per page in normal operation; the cost is token/fetch size growing as O(seen) while traversing a large same-timestamp group (O(G²/limit) total for a group of cardinality G, bounded and only pathological under same-second bulk imports).

## Consequences

- Positive: MCP callers get deterministic O(limit)-per-page traversal of arbitrarily large scopes; date-windowed recall ("memories from June 27") is a first-class query, not a client-side post-filter; supersedes the deferred-cursor decision in engram-lkm and retires the scanCap ceiling for List.
- Negative: the cursor token carries a seen id-set that grows while traversing large same-timestamp groups; callers must treat the token as opaque and never construct one manually.
- Neutral: list_memory gains next_cursor additively (the tool already returns a structured {memories} object); offset pagination is retained for the operator-console UI with documented O(offset+limit) cost; nanosecond-precision timestamps for new writes are an optional additive mitigation, while existing second-precision records are handled correctly by the id-set without re-stamping.
