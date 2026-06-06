<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-cgb; do not edit manually; use `/adr update engram-cgb` -->

# Enforce per-actor authorization in the store layer, not in handlers

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** engram-cgb
**Deciders:** Sean Brandt

## Context

engram authenticated callers via OIDC but had no authorization layer: once a token validated, callers had full read/write access to every record regardless of who created it (bugs engram-ir1, engram-2kw). The enforcement point had to be chosen before any code could be written.

## Decision

Authorization is enforced entirely within internal/store/store.go via Qdrant-level read filters (ownerScopeFilter / the owner-or-shared subclause) and id-path owner-gate primitives (getReadable / getWritable / ownedOrAbsent). MCP tool handlers pass the caller's `sub` through but contain no policy logic.

## Rationale

- Prevents any future handler or internal caller from accidentally bypassing isolation.\n- The Qdrant filter is applied at query time, not after fetching — avoids loading other owners' vectors into server memory.\n- Single invariant point: all read paths compose the owner filter; all write paths go through getWritable or ownedOrAbsent.

## Alternatives Considered

**Enforce in MCP tool handlers** — rejected: every new handler or internal caller must remember to apply the filter; a missed call-site silently grants full access, and post-filtering in a handler over-fetches other owners' vectors into memory.\n**Enforce in the store layer (chosen)** — isolation becomes an invariant of the data layer; no code path can return or mutate another owner's record.

## Consequences

**Positive:** new tools cannot silently skip authz (they must pass sub to store calls); cross_spine discovery search stays safe because the owner/shared subclause is always present even when the scope condition is dropped.\n**Negative:** store method signatures become authz-aware, mixing persistence and security concerns; auth-disabled mode relies on the subtle sub=="" matches owner=="" invariant.\n**Neutral:** raw Get() stays an internal-only primitive beneath the owner-aware wrappers.
