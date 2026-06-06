<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-0gy; do not edit manually; use `/adr update engram-0gy` -->

# Dedicated store_discovery/search_discovery tools, not overloaded store_memory

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** engram-0gy
**Deciders:** Sean

## Context

Discovery records are structurally different from curated memories: they require
at least one citation, enforce a `kind` enum (`map` | `fact`), set
`source=agent-inferred` server-side, and support a `cross_spine` recall mode.
These invariants could be enforced either by adding parameters to the existing
`store_memory` / `search_memory` tools or by introducing dedicated tools.

## Decision

`store_memory` and `search_memory` are left untouched for the curated four
categories. Two new MCP tools — `store_discovery` and `search_discovery` — carry
dedicated signatures, pure validation helpers, and handlers that enforce the
discovery-specific invariants. `get_memory` / `update_memory` / `delete_memory`
operate by id and serve both record kinds unchanged.

## Rationale

- Citations are required for discoveries but meaningless for the curated four; a
  shared tool cannot enforce that without category-conditional validation.
- `source=agent-inferred` is server-set for every discovery with no client
  argument, whereas `store_memory` accepts a `source` — merging the tools
  creates a confusing conditional contract.
- Separate tools preserve the zero-junk discipline: each tool's signature fully
  describes a valid call for its record type.

## Alternatives Considered

**Overload `store_memory` / `search_memory` with discovery parameters
(rejected).** Fewer tools and leverages existing agent familiarity, but pollutes
the curated contract with discovery-only required fields (citations) that are
nonsensical for decision/preference/convention/gotcha, forces validation
branching inside one handler, and collides `cross_spine` semantics with the
curated scope model.

## Consequences

- **Positive:** curated tool contracts are frozen (no regression risk on the
  existing memory workflow); discovery validation lives in pure functions
  unit-tested independently of Qdrant.
- **Negative:** agents working with both record types must learn two tool pairs;
  README and CLAUDE.md tool tables grow by two entries.
- **Neutral:** by-id tools (`get`/`update`/`delete`) need no change to serve
  discovery records.
