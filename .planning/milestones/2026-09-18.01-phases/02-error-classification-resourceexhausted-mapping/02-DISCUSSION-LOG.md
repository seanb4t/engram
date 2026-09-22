# Phase 2: Error Classification & ResourceExhausted Mapping - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-19
**Phase:** 02-error-classification-resourceexhausted-mapping
**Areas discussed:** Where classification happens, Wire contract (hint + field), CLI exit code, MCP mapper shape & scope

---

## Where classification happens

| Option | Description | Selected |
|--------|-------------|----------|
| Client interceptor | Unary interceptor in store.NewQdrantClient base options; covers every RPC; keeps status.FromError working | ✓ |
| Helper at each call site | classifyQdrantErr at each store method; the per-site pattern that drifted #583→#585 | |
| Both | Interceptor plus explicit wraps | |

| Option | Description | Selected |
|--------|-------------|----------|
| Sentinel + detail type | `ErrResponseTooLarge` for errors.Is + a type with method/observed/limit for logs only | ✓ |
| Bare sentinel | Just the sentinel, wrapping the original error | |

## Wire contract

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed `response` token | `field=response`; mapper decoupled from tool argument shapes | ✓ |
| The size-shaping request fields | e.g. `field=limit,full`; couples the mapper to every tool's args | |

| Option | Description | Selected |
|--------|-------------|----------|
| too_large | too_long/too_many family; 11th HintCode | ✓ |
| response_too_large | Self-describing; redundant beside field=response | |
| resource_exhausted | Mirrors transport code name | |

## CLI exit code

| Option | Description | Selected |
|--------|-------------|----------|
| New dedicated code 10 | `exitTooLarge = 10`; one code per failure class | ✓ |
| Reuse 2 (usage) | Misleading when one huge record is the cause | |
| Keep 1, document it | Indistinguishable from other unclassified failures | |

## MCP mapper shape & scope

| Option | Description | Selected |
|--------|-------------|----------|
| Receiving middleware | mcp.Middleware using CallToolResult.GetError(); instrumentTools precedent; no closure edits | ✓ |
| Registration decorator | Wrap each typed handler via a local addTool helper; 15 call sites | |

| Option | Description | Selected |
|--------|-------------|----------|
| This sentinel only | Map ErrResponseTooLarge; everything else unchanged; lane inconsistency deferred | ✓ |
| Also scrub unclassified errors | Mirror connectError's default arm on MCP; widens scope | |

## Claude's Discretion

- Identifiers and file placement
- Interceptor / middleware ordering (log-once constraint)
- Regression-test design (end-to-end overflow via storetest; classifier unit test)
- Phase 2 red-evidence patches

## Deferred Ideas

- MCP lane scrubbing of unclassified errors (lane consistency with Connect)
- Defensive stream interceptor
