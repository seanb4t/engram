<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-tdk; do not edit manually; use `/adr update engram-tdk` -->

# Instrument MCP tools via AddReceivingMiddleware, not per-handler

**Date:** 2026-06-07
**Status:** Accepted
**Decision:** engram-tdk
**Deciders:** Sean Brandt

## Context

engram exposes 11 memory/discovery tools registered uniformly via mcp.AddTool. Observability signals (tool name, caller actor/owner, latency, outcome) must be captured for every tool, including ones added later. The go-sdk Server provides AddReceivingMiddleware, a single seam wrapping every MethodHandler.

## Decision

Instrument all tool calls from one srv.AddReceivingMiddleware. For method=="tools/call" it type-asserts the request to *mcp.CallToolRequest, reads .Params.Name, and records a span, metrics, and a structured log line carrying actor/owner/outcome/dur_ms.

## Rationale

- A single seam future-proofs instrumentation: any new tool is covered with zero marginal code.\n- Handler signatures are unchanged, so existing tests pass untouched.\n- Caller identity (actor/owner) is extracted once at the boundary, not duplicated across 11 handlers.\n- AddReceivingMiddleware availability confirmed during design grounding (bead engram-ew7, deepwiki).

## Alternatives Considered

**Per-handler annotation** (rejected): explicit and per-tool customizable, but requires updating 11 sites, adds boilerplate to every future tool, and duplicates actor/owner extraction.

## Consequences

Positive: consistent spans/logs across all tools at no per-tool cost; downstream embedder/Qdrant spans parent automatically via context propagation. Negative: non-tool methods (initialize, tools/list) need an explicit method-string filter to avoid cardinality noise; the *mcp.CallToolRequest type assertion couples to a go-sdk type. Neutral: non-tool methods still traced at debug level.
