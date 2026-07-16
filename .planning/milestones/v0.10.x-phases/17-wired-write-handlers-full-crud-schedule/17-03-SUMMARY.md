---
phase: 17-wired-write-handlers-full-crud-schedule
plan: 03
subsystem: api
tags: [connect-rpc, protobuf, timestamppb, fieldmaskpb, rfc3339]

# Dependency graph
requires:
  - phase: 17-02
    provides: mutationResult{ID,ShortID}, updateArgs with *string Content / *bool Shared, errRuleImmutable, deps.storeMemory/scheduleMemory/storeDiscovery/updateMemory/deleteMemory/setVisibility signatures
provides:
  - internal/server/protoconv.go — the sole D-09 conversion layer: proto write-request -> internal *Args mappers for all six write RPCs, and mutationResult/(id,short_id) -> proto response mappers
  - windowBoundFloor/windowBoundCeil — outward-rounded RFC3339Nano scheduling-window formatting that survives the store's second-granular .Unix() flooring
affects: [17-04-wired-write-handlers, 17-05, 17-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D-09 conversion-layer seam: every write handler's proto<->args/response translation lives in one file (protoconv.go), keeping handlers thin (identity resolve -> protoconv -> one deps.* call -> protoconv -> response)"
    - "Mask-driven pointer population: a field maps from the proto message into an *Args pointer field ONLY when its path is present in update_mask.paths; absent -> nil (never the proto zero value)"
    - "Outward-rounding before lossy-precision persistence: round a sub-second bound OUTWARD (floor for a lower/reveal bound, ceil for an upper/expiry bound) before handing it to a second-granular downstream store, so precision loss never flips a valid future bound into the past"

key-files:
  created:
    - internal/server/protoconv.go
    - internal/server/protoconv_test.go
  modified: []

key-decisions:
  - "The Visibility enum<->shared bool mapping (visibilityToShared) is used ONLY by the SetVisibility path; UpdateMemory's shared field is a proto bool (engram.proto:168) mapped directly to *bool via &req.Shared, never through the enum mapper (round-8 MED, Codex)"
  - "windowBoundFloor/windowBoundCeil round scheduling bounds OUTWARD to whole seconds (not_before down via Truncate, not_after up via ceil) before RFC3339Nano formatting, so the store's .Unix() flooring on encode/decode is a no-op and a sub-second not_after cannot persist as immediately-expired (round-8 MED, Codex)"
  - "Result mappers (mutationResultToUpdateMemoryResponse etc.) consume 17-02's mutationResult/(id,short_id) tuples directly — no re-fetch inside protoconv or the future handler"

patterns-established:
  - "Table-driven exact-mapping tests (not round-trip) for every proto<->args/response conversion, mirroring connectapi_negative_test.go's style"

requirements-completed: [REQ-connect-write-authz-parity]

coverage:
  - id: D1
    description: "Visibility enum <-> shared bool mapping, scoped to the SetVisibility path only"
    requirement: "REQ-connect-write-authz-parity"
    verification:
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvVisibilityToShared"
        status: pass
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvSetVisibilityRequestToArgs"
        status: pass
    human_judgment: false
  - id: D2
    description: "UpdateMemoryRequest update_mask -> updateArgs mask-driven pointer mapping (nil-Content landmine 2; shared bool->*bool round-8 fix, including the shared=false presence case)"
    requirement: "REQ-connect-write-authz-parity"
    verification:
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvUpdateMemoryRequestToArgs"
        status: pass
    human_judgment: false
  - id: D3
    description: "Citation<->citationArg, StoreMemoryRequest/StoreDiscoveryRequest/ScheduleMemoryRequest -> args exact-field mapping"
    verification:
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvCitationToArg"
        status: pass
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvStoreMemoryRequestToArgs"
        status: pass
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvStoreDiscoveryRequestToArgs"
        status: pass
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvScheduleMemoryRequestToArgs"
        status: pass
    human_judgment: false
  - id: D4
    description: "Timestamp -> scheduling-window string with outward rounding (not_before floor / not_after ceil), including the round-8 near-future not_after surviving the store's second-granular .Unix() flooring"
    requirement: "REQ-connect-write-authz-parity"
    verification:
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvWindowBoundFloorsAndCeils"
        status: pass
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvNotAfterNearFutureSurvivesStoreFlooring"
        status: pass
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvWindowBoundOrderingPreserved"
        status: pass
    human_judgment: false
  - id: D5
    description: "mutationResult / (id, short_id) -> proto write response mapping"
    requirement: "REQ-connect-write-authz-parity"
    verification:
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvResultToResponse"
        status: pass
      - kind: unit
        ref: "internal/server/protoconv_test.go#TestProtoconvIDsToResponses"
        status: pass
    human_judgment: false

duration: 10min
completed: 2026-07-12
status: complete
---

# Phase 17 Plan 03: protoconv Conversion Layer Summary

**D-09 conversion layer (`internal/server/protoconv.go`) for all six write RPCs: mask-driven UpdateMemory mapping (landmine 2 nil-Content, round-8 bool-not-enum shared), outward-rounded RFC3339Nano scheduling-window formatting, and mutationResult/(id,short_id) -> response mappers, built RED->GREEN with exact-mapping table tests.**

## Performance

- **Duration:** ~10 min
- **Completed:** 2026-07-12T23:11:01Z
- **Tasks:** 2
- **Files modified:** 2 (both new)

## Accomplishments

- `protoconv.go` now owns every write-lane proto<->args and result->response conversion, so the six write handlers (17-04) can stay thin identity-resolve -> protoconv -> one `deps.*` call -> protoconv -> response adapters (SC2, D-09).
- The UpdateMemory `shared` bool/enum mismatch flagged in round-8 review is resolved structurally: `visibilityToShared` is referenced ONLY from `setVisibilityRequestToArgs`; `updateMemoryRequestToArgs` maps `&req.Shared` directly to `*bool`, proven by a dedicated `shared=false` presence test.
- Scheduling-window timestamps are rounded OUTWARD to whole seconds (`windowBoundFloor`/`windowBoundCeil`) before RFC3339Nano formatting, closing the round-5/round-8 silent-immediate-expiry edge: a `not_after` ~500ms in the future now survives the store's `.Unix()` flooring as strictly-future, verified by simulating that flooring in `TestProtoconvNotAfterNearFutureSurvivesStoreFlooring`.
- The `content` update_mask landmine (absent path -> nil pointer, never the proto zero value) is covered for both the tags-only and content+summary mask shapes.

## Task Commits

Each task was committed atomically (TDD RED -> GREEN):

1. **Task 1: protoconv exact-mapping tests (RED) incl RFC3339Nano + result mapping** - `dd2fad4f` (test)
2. **Task 2: Implement protoconv conversion layer (GREEN) with RFC3339Nano + result mapping** - `da0a0de2` (feat)

## TDD Gate Compliance

- RED gate: `dd2fad4f test(17-03): add failing protoconv exact-mapping tests` — confirmed the package failed to compile (`undefined: visibilityToShared` etc.) before `protoconv.go` existed.
- GREEN gate: `da0a0de2 feat(17-03): implement protoconv conversion layer (GREEN)` — `go test ./internal/server/... -run TestProtoconv` passes; no REFACTOR commit needed (implementation was clean on first pass).

## Files Created/Modified

- `internal/server/protoconv.go` - Visibility/citation/request/timestamp/result conversion functions (the D-09 layer)
- `internal/server/protoconv_test.go` - table-driven exact-mapping tests covering every conversion function, incl. outward-rounding and the round-8 presence-sensitive cases

## Decisions Made

- Kept `windowBoundFloor`/`windowBoundCeil` as two thin wrappers over a shared `formatWindowBound(ts, roundUp bool)` helper rather than one function with a rounding-direction parameter at every call site — matches the plan's "not_before DOWN / not_after UP" framing directly in the call names at `scheduleMemoryRequestToArgs`.
- Reworded one doc comment to avoid a literal `time.RFC3339` substring (plain-second layout) so the acceptance-criteria grep (`grep -c 'time.RFC3339\b' protoconv.go` == 0) stays a true negative on the plain-second formatter, not just the code path.

## Deviations from Plan

None - plan executed exactly as written. The extra exact-mapping tests for `storeMemoryRequestToArgs`/`storeDiscoveryRequestToArgs`/`scheduleMemoryRequestToArgs` (beyond the explicitly-listed behavior cases) are within the plan's stated scope ("the D-09 conversion layer (requests + results)... exact-mapping unit tests") — not a deviation, just fuller coverage of the same artifact.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 17-04 (wired write handlers) can now call `protoconv`'s request->args and result->response functions directly from the six Connect handler bodies; no further conversion logic should be inlined per-handler (D-09 prohibition).
- `windowBoundFloor`/`windowBoundCeil` feed `parseWindow` (tools.go:452) unchanged — 17-04 wires `scheduleMemoryRequestToArgs`'s output straight into the existing `deps.scheduleMemory` call.
- No blockers.

---
*Phase: 17-wired-write-handlers-full-crud-schedule*
*Completed: 2026-07-12*

## Self-Check: PASSED
- FOUND: internal/server/protoconv.go
- FOUND: internal/server/protoconv_test.go
- FOUND: dd2fad4f (test commit)
- FOUND: da0a0de2 (feat commit)
