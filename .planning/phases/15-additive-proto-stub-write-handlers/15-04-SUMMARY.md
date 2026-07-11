---
phase: 15-additive-proto-stub-write-handlers
plan: 04
subsystem: api
tags: [protobuf, connect-rpc, protoreflect, protovalidate, go-testing]

# Dependency graph
requires:
  - phase: 15-01
    provides: the six additive write RPCs + buf.validate annotations in proto/engram/v1/engram.proto, regenerated gen/go + gen/ts
  - phase: 15-03
    provides: the protovalidate interceptor wired into mountConnect (order otel -> access-log -> subject/401 -> validate/400)
provides:
  - Descriptor-walking regression test pinning the 11-RPC EngramService shape, per-field wire-shape tables for the read lane + Memory/ScopeCount, and IDEMPOTENCY_UNKNOWN on every method
  - Full negative-path matrix proving exact Connect codes (Unimplemented/Unauthenticated/405/InvalidArgument) for all six write RPCs, including the UpdateMemory mask cells and category-allowlist cells
affects: [phase-17-wired-write-handlers, phase-16-csrf-interceptor]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Descriptor-walking regression test (protoreflect.FileDescriptor walk) as the semantic alternative to golden wire snapshots for proving RPC/message shape invariants survive codegen"
    - "Go generics (callWrite[Req, Resp]) to keep a heterogeneous six-RPC test table uniform despite each RPC having a distinct typed connect.Request/Response pair"

key-files:
  created:
    - internal/server/connectdescriptor_test.go
    - internal/server/connectapi_negative_test.go
  modified: []

key-decisions:
  - "Per-field pins (number/name/kind/cardinality/message-type) on Memory, ScopeCount, and all five read request/response messages — not just message names — per cross-AI review finding #6, so an accidental additive/renamed/retyped read field fails the descriptor test even though RPC count and message names are unchanged"
  - "GET-405 cells use the generated engramv1connect.EngramService*Procedure constants rather than hand-written path strings, per finding #6, so the test tracks codegen instead of drifting from it"

patterns-established:
  - "Table-driven write-RPC negative test with a generic callWrite[Req, Resp] helper — new write RPCs slot in as one more table entry with a valid/invalid payload pair"

requirements-completed: [REQ-connect-write-rpcs]

coverage:
  - id: D1
    description: "Descriptor test pins 11 RPCs (5 read + 6 write) by exact request/response type and IDEMPOTENCY_UNKNOWN on every method"
    requirement: "REQ-connect-write-rpcs"
    verification:
      - kind: unit
        ref: "internal/server/connectdescriptor_test.go#TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs"
        status: pass
    human_judgment: false
  - id: D2
    description: "Per-field wire-shape table (number/name/kind/cardinality/message-type) pinned for Memory, ScopeCount, and all five read request/response messages (SC4)"
    requirement: "REQ-connect-write-rpcs"
    verification:
      - kind: unit
        ref: "internal/server/connectdescriptor_test.go#TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs"
        status: pass
    human_judgment: false
  - id: D3
    description: "All six write RPCs return exactly CodeUnimplemented (authenticated+valid), CodeUnauthenticated (unauthenticated, even with invalid payload), HTTP 405 (raw GET via generated procedure constants), and CodeInvalidArgument (authenticated+invalid)"
    requirement: "REQ-connect-write-rpcs"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_negative_test.go#TestWriteRPCNegativeMatrix"
        status: pass
    human_judgment: false
  - id: D4
    description: "UpdateMemory update_mask cells (absent/empty-paths/unknown-path) and StoreMemory/ScheduleMemory category=\"rule\" cells each return CodeInvalidArgument"
    requirement: "REQ-connect-write-rpcs"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_negative_test.go#TestWriteRPCNegativeMatrix/UpdateMemory_mask_cells"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_negative_test.go#TestWriteRPCNegativeMatrix/category_allowlist_cells"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-11
status: complete
---

# Phase 15 Plan 04: Descriptor + Negative-Path Regression Tests Summary

**Two new Go tests turn Phase 15's SC2/SC3/SC4 success criteria into automated proof: a protoreflect descriptor walk pinning the 11-RPC shape and per-field wire tables, and a table-driven negative matrix asserting the exact Connect code for all six write RPCs across four request shapes.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-11T22:23:30Z
- **Completed:** 2026-07-11T22:35:00Z
- **Tasks:** 2 completed
- **Files modified:** 2 (both new)

## Accomplishments
- `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` reflects over `engramv1.File_engram_v1_engram_proto`, asserting exactly 11 RPCs, the six write RPCs' exact request/response message types, per-field tables (number/name/kind/cardinality/message-type) for Memory/ScopeCount plus every read request/response message, and `IDEMPOTENCY_UNKNOWN` on all 11 methods
- `TestWriteRPCNegativeMatrix` drives the real interceptor chain (otel -> access-log -> subject/401 -> validate/400) over `httptest.NewServer`, table-driven across the six write RPCs, asserting `CodeUnimplemented` / `CodeUnauthenticated` / HTTP 405 / `CodeInvalidArgument` for each
- Dedicated subtests prove the D-03 mask fix end-to-end (absent/empty-paths/unknown-path `update_mask` on `UpdateMemory`) and the category allowlist fix (`category:"rule"` on `StoreMemory` and `ScheduleMemory`)
- `task lint:go` (golangci-lint, revive included) is clean; `task license:check` passes; full `task test` (Go + Python) is green

## Task Commits

Each task was committed atomically:

1. **Task 1: Descriptor-walking regression test (D-12)** - `efba59b6` (test)
2. **Task 2: Full negative-path matrix test across all six write RPCs (D-11)** - `89369653` (test)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/server/connectdescriptor_test.go` - Descriptor-walk test + `assertFields` helper pinning per-field wire shapes
- `internal/server/connectapi_negative_test.go` - Negative-path matrix + generic `callWrite` helper, mask cells, category cells

## Decisions Made
- Per-field pins (not just message-name pins) for Memory/ScopeCount/read messages, addressing cross-AI review finding #6 (SC4 under-proved by names alone)
- GET-405 cells use generated `engramv1connect.EngramService*Procedure` constants instead of hardcoded path strings (finding #6, codex LOW)
- `callWrite[Req, Resp]` generic helper keeps the six-RPC table uniform despite each RPC having a distinct typed client method signature

## Deviations from Plan

None - plan executed exactly as written. Both test files matched the RESEARCH.md/PATTERNS.md-verified patterns on the first pass; no auto-fixes were needed.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase 15 (additive-proto-stub-write-handlers) is now complete: all four plans (proto+codegen, idempotency-ban CI gate, protovalidate interceptor, regression tests) are committed and green. Phase 17 (wired write handlers) can now implement the six stub RPCs' business logic behind this proven contract — the descriptor test and negative matrix will catch any regression in RPC shape, idempotency posture, or validation wiring as Phase 17 fills in handler bodies. Phase 16 (CSRF interceptor) slots into the same `connect.WithInterceptors` chain this plan's tests exercise.

---
*Phase: 15-additive-proto-stub-write-handlers*
*Completed: 2026-07-11*

## Self-Check: PASSED

All created files and commit hashes verified present on disk / in git log.
