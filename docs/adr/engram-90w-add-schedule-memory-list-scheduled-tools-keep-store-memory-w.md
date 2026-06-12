<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-90w; do not edit manually; use `/adr update engram-90w` -->

# Add schedule_memory/list_scheduled tools; keep store_memory windowless

**Date:** 2026-06-12
**Status:** Accepted
**Decision:** engram-90w
**Deciders:** Sean Brandt

## Context

store_memory's contract explicitly forbids timestamps as content and is built around curated, unscheduled records. Adding not_before/not_after to it would require category-conditional validation (both fields optional, at least one required only when scheduling), muddying a deliberately clean signature. The discovery tools established the precedent of dedicated tools for structurally different record shapes.

## Decision

schedule_memory and list_scheduled are separate MCP tools with dedicated signatures; store_memory is not modified and retains its windowless contract.

## Rationale

- store_memory's design intent is 'explicit, zero-junk, correctable, no auto-extraction' — temporal bounds are scheduling metadata, not memory content.\n- A shared tool cannot enforce 'at least one bound required' without category-conditional validation absent from the windowless case.\n- Consistent with the discovery-tools precedent: structurally different record shapes get dedicated tools.\n- schedule_memory's signature fully describes a valid scheduled call with no optional-conditional ambiguity.

## Alternatives Considered

- **Dedicated schedule_memory + list_scheduled tools (chosen):** preserves store_memory's clean contract; dedicated signature fully describes a valid scheduled call; validation is unconditional. Costs two more tools on the MCP surface.\n- **Add not_before/not_after as optional params to store_memory (rejected):** fewer tools, but violates the 'no timestamps' contract and forces confusing conditional validation onto the curated-four signature.

## Consequences

Positive: store_memory's contract stays stable and well-understood; schedule_memory validation is unconditional and self-documenting; discovery category is explicitly rejected at the schedule_memory boundary. Negative: larger MCP tool surface — agents must choose between store_memory and schedule_memory. Neutral: get/update/delete operate by id and serve all record kinds unchanged.
