<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-y1g; do not edit manually; use `/adr update engram-y1g` -->

# Gate recall via Qdrant filter; leave get_memory ungated

**Date:** 2026-06-12
**Status:** Accepted
**Decision:** engram-y1g
**Deciders:** Sean Brandt

## Context

engram's Search returns Qdrant top-k with no Go post-filter, so any temporal gate must be a server-side Qdrant condition or it silently shrinks results below k. By-id operations (get_memory / FetchForUpdate) are the only path an owner has to manage a pending or expired record surfaced via list_scheduled. The gate must therefore be asymmetric: recall paths are gated, by-id is not.

## Decision

Temporal validity is enforced as two composable Must conditions in the Qdrant filter on Search and List; get_memory and all by-id paths remain ungated so owners can manage hidden records.

## Rationale

- Search is Qdrant top-k with no Go post-filter; a post-filter silently under-returns.\n- The gate composes orthogonally with the existing ownerOrSharedCondition via extra Must conditions, without rewriting the authz envelope.\n- By-id ungating is the management escape hatch: list_scheduled surfaces the ID, then get/update/delete operate on the record.\n- NewIsEmpty ensures zero behavior change for every existing unwindowed record.

## Alternatives Considered

- **Server-side Qdrant filter on recall paths only (chosen):** prevents under-returning on top-k; backward-compatible via NewIsEmpty; no over-fetch into Go memory. Asymmetric contract needs documenting; fields unindexed (full scan, acceptable today).\n- **Post-filter in Go after fetch (rejected):** silently shrinks Search below requested k and over-fetches into server memory.\n- **Gate everywhere including get_memory (rejected):** removes the only management path for pending/expired records — owner cannot inspect or update what they cannot fetch.

## Consequences

Positive: full backward compatibility (pre-feature records match via NewIsEmpty); the gate lives in the store layer so no future handler can bypass it; no Go-memory over-fetch. Negative: asymmetric recall-vs-by-id contract must be understood by contributors; payload fields unindexed (full scan at scale). Neutral: a payload index can be added later without changing the contract.
