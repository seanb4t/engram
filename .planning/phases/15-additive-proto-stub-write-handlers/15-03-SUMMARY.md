---
phase: 15-additive-proto-stub-write-handlers
plan: 03
subsystem: api
tags: [connect-rpc, protovalidate, interceptor, go]

# Dependency graph
requires:
  - phase: 15-additive-proto-stub-write-handlers (plan 01)
    provides: buf.validate constraints on the six write RPC request messages, generated Go/TS stubs, buf.build/go/protovalidate as an indirect go.mod dependency
provides:
  - newConnectValidateInterceptor — a hand-rolled Connect unary interceptor that validates every request message against its buf.validate constraints
  - mountConnect wiring: protovalidate.New() constructed once at startup, interceptor added as the innermost (last) WithInterceptors argument, after the subject interceptor
  - buf.build/go/protovalidate promoted from indirect to direct in go.mod (first runtime importer)
affects: [15-04, 17-deps-refactor-wired-handlers]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Hand-rolled Connect interceptor over buf.build/go/protovalidate instead of the unstable connectrpc.com/validate module"
    - "Interceptor ordering: otel -> access-log -> subject (401) -> validate (400) -> handler"

key-files:
  created:
    - internal/server/connectvalidate.go
    - internal/server/connectvalidate_test.go
  modified:
    - internal/server/connectapi.go
    - go.mod

key-decisions:
  - "Auth runs before validation (D-10): the validate interceptor is the LAST WithInterceptors argument, after newConnectSubjectInterceptor, so an unauthenticated caller gets CodeUnauthenticated and never sees field-level validation detail"
  - "The interceptor validates every unary RPC (reads included), not only the six writes — a deliberate no-op for today's unconstrained read messages that future-proofs any read-side buf.validate annotation"
  - "go mod tidy in this plan (not Plan 01) promotes buf.build/go/protovalidate from // indirect to a direct require, since this plan's connectvalidate.go is the first code to import the runtime package; no new external Go module was introduced"

requirements-completed: [REQ-connect-write-rpcs]

coverage:
  - id: D1
    description: "Hand-rolled protovalidate interceptor rejects invalid write-RPC payloads with CodeInvalidArgument and passes valid ones through"
    requirement: "REQ-connect-write-rpcs"
    verification:
      - kind: unit
        ref: "internal/server/connectvalidate_test.go#TestConnectValidateInterceptor/invalid_message_returns_invalid_argument"
        status: pass
      - kind: unit
        ref: "internal/server/connectvalidate_test.go#TestConnectValidateInterceptor/valid_message_passes_through"
        status: pass
    human_judgment: false
  - id: D2
    description: "Non-validation errors from the validator map to CodeInternal (covered via a fake validator, since a real validator over generated constraints cannot reach this branch)"
    verification:
      - kind: unit
        ref: "internal/server/connectvalidate_test.go#TestConnectValidateInterceptor/non_validation_error_maps_to_internal"
        status: pass
    human_judgment: false
  - id: D3
    description: "Non-proto request payloads pass through the interceptor defensively without panicking or erroring"
    verification:
      - kind: unit
        ref: "internal/server/connectvalidate_test.go#TestConnectValidateInterceptor/non_proto_request_passes_through_defensively"
        status: pass
    human_judgment: false
  - id: D4
    description: "mountConnect wires the validate interceptor as the innermost interceptor, after the subject interceptor, preserving the nil-resolver R1 early return, and the existing Connect test suite stays green"
    verification:
      - kind: unit
        ref: "go test ./internal/server/... -run 'TestConnect' -v (full suite, including TestMountConnectSkipsWhenResolverNil, TestMountConnectMountsWhenResolverPresent, TestConnectCrossActorIsolation, TestConnectCookieLaneIsolation)"
        status: pass
    human_judgment: false
  - id: D5
    description: "buf.build/go/protovalidate is promoted from indirect to direct in go.mod via go mod tidy, with no new external Go module introduced"
    verification:
      - kind: other
        ref: "git diff go.mod (single-line reclassification only, confirmed no go.sum change and no connectrpc.com/validate entry)"
        status: pass
    human_judgment: false

patterns-established:
  - "internal/server/connectvalidate.go: interceptor-factory shape mirroring newConnectSubjectInterceptor (connectauth.go) — a func(...) connect.UnaryInterceptorFunc closing over next connect.UnaryFunc"

# Metrics
duration: 12min
completed: 2026-07-11
status: complete
---

# Phase 15 Plan 3: Connect Validate Interceptor Summary

**Hand-rolled protovalidate Connect interceptor enforcing buf.validate constraints at request time, wired innermost (after auth) in mountConnect, promoting the protovalidate runtime to a direct go.mod dependency**

## Performance

- **Duration:** 12 min
- **Completed:** 2026-07-11T22:22:00Z
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments
- `newConnectValidateInterceptor` validates every unary RPC request message against its `buf.validate` constraints, mapping `*protovalidate.ValidationError` to `CodeInvalidArgument` and any other error to `CodeInternal`, with non-proto payloads passing through defensively
- `mountConnect` constructs a single `protovalidate.Validator` at startup (after the nil-resolver R1 guard) and adds the interceptor as the last `WithInterceptors` argument, guaranteeing auth (401) always runs before validation (400) per D-10
- `buf.build/go/protovalidate` promoted from `// indirect` to a direct `go.mod` require — this plan's `connectvalidate.go` is the first code in the module to import the runtime package; `connectrpc.com/validate` was deliberately not added

## Task Commits

Each task was committed atomically:

1. **Task 1: Create the hand-rolled protovalidate interceptor with unit tests + promote the runtime dep** - `d1166ccd` (feat)
2. **Task 2: Wire the validate interceptor into mountConnect (auth before validate)** - `a8e0f6da` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/server/connectvalidate.go` - `newConnectValidateInterceptor` factory validating request messages against buf.validate constraints
- `internal/server/connectvalidate_test.go` - Four-case unit test suite: valid (real validator), invalid (real validator), non-validation error (fake validator, CodeInternal), non-proto passthrough
- `internal/server/connectapi.go` - `mountConnect` constructs `protovalidate.New()` and wires the interceptor as the innermost `WithInterceptors` argument
- `go.mod` - `buf.build/go/protovalidate` reclassified from indirect to direct (via `go mod tidy`)

## Decisions Made
- Kept the interceptor scope-agnostic (validates all unary RPCs, not just the six writes) per the plan's explicit truth and Codex LOW disposition — today a no-op for reads, automatically enforced for any future read-side annotation
- Used a fake `protovalidate.Validator` in tests to reach the otherwise-unreachable `CodeInternal` branch, while keeping a real `protovalidate.New()` case to prove the Plan 01 generated annotations actually load and enforce

## Deviations from Plan

None - plan executed exactly as written. `go mod tidy` produced exactly the expected single-line reclassification (`buf.build/go/protovalidate` indirect → direct) with no go.sum changes and no new external module.

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- The validate interceptor is live in the Connect chain; Plan 04's negative-matrix tests (unauthenticated + invalid → `CodeUnauthenticated`, authenticated + invalid → `CodeInvalidArgument`) can now assert against real enforcement rather than a stub.
- `task license:check`, `go vet ./internal/server/...`, and the full `TestConnect*` suite are green; no blockers for Plan 04.

---
*Phase: 15-additive-proto-stub-write-handlers*
*Completed: 2026-07-11*
