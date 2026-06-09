<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-8xe; do not edit manually; use `/adr update engram-8xe` -->

# Adopt ConnectRPC and protobuf/buf for the web UI API

**Date:** 2026-06-09
**Status:** Accepted
**Decision:** engram-8xe
**Deciders:** Sean

## Context

engram has been MCP-only; CLAUDE.md explicitly listed protobuf/buf under 'Not used here' when engram adopted the holomush cobra conventions. Adding a human-facing CRUD console introduces a second consumer class — a browser UI — where a typed, evolvable Go<->TypeScript contract across a growing RPC surface (observe -> correct -> author) has qualitatively different value from the text-transport MCP core. No existing ADR covers this convention.

## Decision

Adopt ConnectRPC (connect-go server, connect-es TypeScript client) with a protobuf schema and the buf toolchain for the web-UI API, consciously reversing and SCOPING the CLAUDE.md 'Not used here: protobuf/buf' convention: buf/protobuf is used ONLY for the web-UI ConnectRPC API; the MCP core, store, auth, and CLI remain protobuf-free.

## Rationale

- One proto schema generates both sides, eliminating Go<->TS contract drift as the API grows across three phases.\n- connect-go natively speaks Connect and gRPC-Web — no Envoy/grpc-web proxy required.\n- The prior 'no protobuf' call was made when engram was MCP-only; a browser UI is a structurally different consumer class that earns the toolchain cost.\n- Node and buf stay strictly dev-time; go build / goreleaser / the release runner need neither — generated stubs are committed and CI drift-checked.

## Alternatives Considered

**REST/JSON hand-written handlers** — no new toolchain and consistent with current style, but no type safety across the Go<->TS boundary, schema drift surfaces at runtime, and the contract must be hand-maintained as the surface grows across three phases. Rejected.

## Consequences

Positive: end-to-end type safety (contract violations are compile errors); a single source of truth (.proto) for a surface growing across three phases; no proxy layer (browser calls the engram binary directly over fetch). Negative: buf toolchain + codegen added as dev deps; generated artifacts committed to the repo; new CI buf lint/breaking/gen-drift jobs. Neutral: CLAUDE.md 'Not used here' line scoped to the exception; MCP core/store/auth/CLI unaffected.
