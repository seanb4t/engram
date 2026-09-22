---
phase: 01-test-harness-fixture-helper
plan: 02
subsystem: testing
tags: [qdrant, grpc, otelgrpc, store, ast-gate]

requires:
  - phase: 01-01
    provides: "store.NewQdrantClient(host, port, opts...) — the single Qdrant client constructor for production and every test"
provides:
  - "dialTestClient(t, opts...) — the single in-package Qdrant test-dial primitive, building through NewQdrantClient"
  - "TestNewQdrantClientAppliesBaseAndCallerOptions — REQ-test-client-parity's production-span-plus-caller-interceptor parity proof"
  - "All six in-package internal/store dial sites (dialTestClient and its five interceptor-wrapping siblings) converged onto NewQdrantClient"
affects: ["01-03", "01-04", "01-05"]

actuals:
  tokens: 3729
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "One-line delegation: an interceptor-wrapping dial helper reduces to `return dialTestClient(t, grpc.WithUnaryInterceptor(...))`, with dialTestClient owning address/skip/fail-closed/dial logic exclusively"
    - "Observable-span-plus-interceptor parity proof: a client's dial options are proven by what the client DOES (a recorded span, a fired interceptor), never by inspecting grpc/qdrant internals (rule m45p2b4bp7)"

key-files:
  created:
    - internal/store/qdrantclient_test.go
  modified:
    - internal/store/store_test.go
    - internal/store/migrate_status_test.go
    - internal/store/revert_test.go
    - internal/store/migrate_converge_test.go
    - internal/store/schemaversion_recallgate_test.go
    - internal/store/migrate_faultinject_test.go

key-decisions:
  - "dialTestClient keeps its address/skip/fail-closed logic byte-identical (plan 01-04 relocates the harness); only the client construction line changed, from a bare qdrant.NewClient to NewQdrantClient(host, port, opts...)."
  - "Each of the five interceptor helpers' doc comment was rewritten to state it now delegates to dialTestClient for address/skip/dial ownership, replacing the old 'identical dial/skip/parse boilerplate' framing that no longer describes the code."
  - "net and strconv imports dropped from all five interceptor-helper files once their inlined dial logic was removed — verified via rg that neither package is referenced anywhere else in each file before removing."

requirements-completed: [REQ-test-client-parity]

coverage:
  - id: D1
    description: "dialTestClient (and therefore every in-package test client) builds through NewQdrantClient, not a bare qdrant.NewClient"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "internal/store#TestNewQdrantClientAppliesBaseAndCallerOptions"
        status: pass
      - kind: integration
        ref: "internal/store — ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -v (only TestRedEvidencePatchesAreLive fails)"
        status: pass
    human_judgment: false
  - id: D2
    description: "A test client built through dialTestClient with a caller grpc.WithUnaryInterceptor both fires that interceptor AND records the production otelgrpc client span for the same RPC"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "internal/store#TestNewQdrantClientAppliesBaseAndCallerOptions"
        status: pass
    human_judgment: false
  - id: D3
    description: "The five interceptor-wrapping dial helpers each reduce to a one-line delegation to dialTestClient, with every interceptor-driven test family (facet counts, count side effects, mid-sweep hooks, recall-filter capture, SetPayload fault injection) still passing against real Qdrant"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: integration
        ref: "internal/store — ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -count=1 -v (top-level failures = exactly TestRedEvidencePatchesAreLive)"
        status: pass
    human_judgment: false

duration: 10min
completed: 2026-09-18
status: complete
---

# Phase 1 Plan 2: In-Package Test Clients Converge on the Shared Constructor Summary

**All six `internal/store` in-package Qdrant test dial sites now build through `NewQdrantClient`, proven by a new parity test that observes a caller interceptor firing alongside the production otelgrpc client span on the same RPC.**

## Performance

- **Duration:** 10 min
- **Started:** 2026-09-18T20:04:37-04:00
- **Completed:** 2026-09-18T20:14:06-04:00
- **Tasks:** 2 completed
- **Files modified:** 6 (1 created, 5 modified)

## Accomplishments

- `dialTestClient(t *testing.T, opts ...grpc.DialOption) *qdrant.Client` — the single in-package dialing primitive — now builds every in-package client through `NewQdrantClient(host, port, opts...)` instead of a bare `qdrant.NewClient(&qdrant.Config{...})`, keeping its address/skip/fail-closed logic untouched.
- New `internal/store/qdrantclient_test.go` with `TestNewQdrantClientAppliesBaseAndCallerOptions`: dials through `dialTestClient` with a counting `grpc.UnaryClientInterceptor`, calls `HealthCheck`, and asserts BOTH that the interceptor fired (caller option took effect) AND that a client-kind `/HealthCheck` span was recorded on the package's process-wide span recorder (production's otelgrpc base option took effect on the same client). Observed RED against the plain-constructor variant first (interceptor count passed, span assertion failed with "no client-kind HealthCheck span recorded"), then GREEN after switching to `NewQdrantClient`.
- The five interceptor-wrapping dial siblings — `dialFacetInterceptingTestClient`, `dialCountSideEffectTestClient`, `dialMidSweepTestClient`, `dialCapturingTestClient`, `dialFaultInjectingTestClient` — each collapsed from ~25 lines of inlined address/skip/parse/dial boilerplate to one line: `return dialTestClient(t, grpc.WithUnaryInterceptor(<their interceptor>))`. Every interceptor and every calling test stayed untouched.
- Removed the now-unused `net`/`strconv` imports from all five files whose only use was the deleted inline dial logic.

## Task Commits

Each task was committed atomically:

1. **Task 1: In-package tests dial through the shared constructor, proven by a production-span plus caller-interceptor parity test** - `8523cb75` (test)
2. **Task 2: The five interceptor dial helpers delegate to dialTestClient, and every interceptor-driven test family still passes** - `5c2a30ca` (test)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP)

## Files Created/Modified

- `internal/store/qdrantclient_test.go` - `TestNewQdrantClientAppliesBaseAndCallerOptions`, the REQ-test-client-parity direct proof.
- `internal/store/store_test.go` - `dialTestClient` gains `opts ...grpc.DialOption`, builds through `NewQdrantClient`; adds the `google.golang.org/grpc` import.
- `internal/store/migrate_status_test.go` - `dialFacetInterceptingTestClient` reduced to a `dialTestClient` delegation; drops `net`/`strconv`.
- `internal/store/revert_test.go` - `dialCountSideEffectTestClient` reduced to a `dialTestClient` delegation; drops `net`/`strconv`.
- `internal/store/migrate_converge_test.go` - `dialMidSweepTestClient` reduced to a `dialTestClient` delegation; drops `net`/`strconv`.
- `internal/store/schemaversion_recallgate_test.go` - `dialCapturingTestClient` reduced to a `dialTestClient` delegation; drops `net`/`strconv`.
- `internal/store/migrate_faultinject_test.go` - `dialFaultInjectingTestClient` reduced to a `dialTestClient` delegation; drops `net`/`strconv`.

## Decisions Made

- Kept `dialTestClient`'s address resolution, skip, and fail-closed logic exactly as-is per the plan's explicit instruction — plan 01-04 owns relocating the harness itself; this plan only converges the client-construction line.
- Rewrote each of the five helpers' doc comments to describe the new delegation shape rather than leave stale "identical dial/skip/parse boilerplate" prose describing code that no longer exists in that function.
- Verified via `rg` that no other code in each of the five files referenced `net.` or `strconv.` before dropping those imports, rather than assuming.

## Deviations from Plan

None — plan executed exactly as written. The RED/GREEN sequence in Task 1 was an explicitly required observation, not a deviation: the plain-constructor variant of `dialTestClient` was built and run first (interceptor count passed, span assertion failed as predicted), then switched to `NewQdrantClient` for GREEN.

**Total deviations:** 0. **Impact:** None — plan executed as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Six of eleven test `qdrant.NewClient` call sites now converge on `NewQdrantClient` (the six in-package `internal/store` sites); the remaining five cross-package sites (`internal/server`, `internal/e2e`, `internal/retrievaleval`) are plan 01-03's scope, and plan 01-03's own coupling justification (its `TestEveryStoreConstructionRoutesThroughSeam` / `TestCollectionPrefixesAreDisjoint` read this plan's files as source text) is unaffected — every `newTestStore` seam and `testCollectionPrefix` constant this plan touched stayed unchanged.
- `internal/store` stays green except the single expected `TestRedEvidencePatchesAreLive` red, unchanged from plan 01-01's baseline; plan 01-05's Task 3 closes it.
- `go test ./internal/keylinks/ -count=1` passes.
- No blockers or concerns for the next plan.

---
*Phase: 01-test-harness-fixture-helper*
*Completed: 2026-09-18*

## Self-Check: PASSED

Both created/modified files verified present on disk (`internal/store/qdrantclient_test.go` new; the five interceptor-helper files and `store_test.go` modified). Both task commits (`8523cb75`, `5c2a30ca`) verified present in `git log --oneline --all`. Acceptance criteria for both tasks re-run and confirmed passing: `dialTestClient` signature and construction line match exactly (1 line each), SPDX header matches `store.go`, `SpanKindClient`/`withSpanRecorder(t)` present in the new test file, five one-line delegations present, zero remaining `qdrant.NewClient(` call sites in `internal/store/*_test.go`, zero remaining `testQdrantAddr` references in the five converged files, `go vet ./internal/store/` exits 0, `golangci-lint run ./internal/store/...` reports 0 issues, `task lint` passes repo-wide, `git diff --exit-code main...HEAD -- go.mod go.sum` exits 0, and `go test ./internal/keylinks/ -count=1` passes.
