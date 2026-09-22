---
phase: 01-test-harness-fixture-helper
plan: 03
subsystem: testing
tags: [qdrant, grpc, storetest, testcontainers, ast-gate]

requires:
  - phase: 01-01
    provides: "internal/store/storetest package: RecvLimit, QdrantImage, Run/terminate lifecycle, RequireQdrant/SkipOrFailNoQdrant, Addr/EnvAddr/ContainerBooted, AssertSharedAddressHonored, Dial"
provides:
  - "internal/server, internal/e2e, internal/retrievaleval TestMains delegate Qdrant container lifecycle to storetest.Run (retrievaleval via storetest.Run(m, storetest.IgnoreRequireQdrant()))"
  - "The five remaining cross-package dial sites (testDepsWithStore, dialWarnPendingMigrationsTestClient, dialRawQdrantClient, spineReviewQdrantClient, newTestcontainerStore) on storetest.Dial(t, storetest.RecvLimit)"
affects: ["01-04", "01-05"]

actuals:
  tokens: 10159
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Cross-package TestMain delegation: os.Exit(storetest.Run(m)) or os.Exit(storetest.Run(m, storetest.IgnoreRequireQdrant())), with package-specific quirks (e2e's local binary build, retrievaleval's opt-in gate) staying local, wrapping storetest rather than reimplementing it"
    - "storetest.Addr()/storetest.SkipOrFailNoQdrant(t) replace the old package-local address var + skip-or-fail helper pair at every call site that only needs the resolved address, not a dialed client"

key-files:
  modified:
    - internal/server/tools_test.go
    - internal/server/schemaversion_wire_test.go
    - internal/e2e/harness_test.go
    - internal/e2e/spine_review_test.go
    - internal/e2e/console_browser_test.go
    - internal/retrievaleval/retrieval_eval_test.go

key-decisions:
  - "internal/retrievaleval keeps ENGRAM_RETRIEVAL_EVAL as TestMain's first statement, then delegates via storetest.Run(m, storetest.IgnoreRequireQdrant()) — preserves its pre-phase behavior of never parsing ENGRAM_REQUIRE_QDRANT (a missing Qdrant with the eval gate set still only skips), recorded per RESEARCH.md Pitfall 6 and this plan's orchestrator guidance."
  - "internal/e2e keeps its early storetest.RequireQdrant() parse and local `go build` of the engram binary BEFORE delegating to storetest.Run(m) — both orthogonal to storetest and preserved verbatim in ordering; e2e newly inherits storetest's post-boot empty-address fail-closed check, a deliberate normalization toward the store/server harness."
  - "The dead deferred `os.RemoveAll(tmp)` in e2e's old TestMain (unreachable — every path used os.Exit, which skips defers) is dropped rather than carried forward; cleanup now happens via an explicit `_ = os.RemoveAll(tmp)` immediately before each os.Exit."

requirements-completed: [REQ-test-client-parity]

coverage:
  - id: D1
    description: "internal/server, internal/e2e and internal/retrievaleval each delegate Qdrant container lifecycle to storetest.Run; none still declares its own ENGRAM_REQUIRE_QDRANT parser, container terminator, Qdrant image literal, skip-or-fail helper, or address/booted package variables"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "rg -n 'func (requireQdrant|failOrSkipNoQdrant|skipOrFailNoQdrant|terminateQdrant|TestRequireQdrant)[(]' internal/server/*_test.go internal/e2e/*_test.go internal/retrievaleval/*_test.go | wc -l -> 0"
        status: pass
      - kind: unit
        ref: "rg -n -w 'testQdrant(Addr|ContainerBooted)|qdrantImageTag' internal/server/*_test.go internal/e2e/*_test.go internal/retrievaleval/*_test.go | wc -l -> 0; rg -n 'qdrant/qdrant[:]v' (same scope) -> 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "The five cross-package dial sites obtain their client with storetest.Dial(t, storetest.RecvLimit); post-dial raw usage (SetPayload injection, codec bypass, DeleteCollection) is unchanged"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "rg -o 'storetest[.]Dial[(]t, storetest[.]RecvLimit[)]' internal/server/tools_test.go internal/server/schemaversion_wire_test.go -> 3; presence in spine_review_test.go and retrieval_eval_test.go confirmed by Read"
        status: pass
      - kind: integration
        ref: "ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ ./internal/e2e/ ./internal/retrievaleval/ -count=1"
        status: pass
    human_judgment: false
  - id: D3
    description: "internal/retrievaleval never parses ENGRAM_REQUIRE_QDRANT (IgnoreRequireQdrant applied); internal/e2e still fails closed on it before the build"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: integration
        ref: "internal/retrievaleval#TestSharedQdrantAddressHonored under ENGRAM_RETRIEVAL_EVAL=1 ENGRAM_REQUIRE_QDRANT=treu ENGRAM_QDRANT_TEST_ADDR=127.0.0.1:6334 -> PASS (parse ignored, shared path taken)"
        status: pass
      - kind: integration
        ref: "ENGRAM_REQUIRE_QDRANT=treu go test ./internal/e2e/ -run '^TestSharedQdrantAddressHonored$' -count=1 -> non-zero exit, \"invalid value \\\"treu\\\"\""
        status: pass
    human_judgment: false
  - id: D4
    description: "Every TestSharedQdrantAddressHonored delegates to storetest.AssertSharedAddressHonored; CI's shared-address count holds at 2 PASS + 1 SKIP across server/e2e/retrievaleval (the third PASS is internal/store's, plan 01-04)"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: integration
        ref: "ENGRAM_QDRANT_TEST_ADDR=127.0.0.1:6334 ENGRAM_REQUIRE_QDRANT=1 go test -run '^TestSharedQdrantAddressHonored$' -v -count=1 ./internal/server/... ./internal/e2e/... ./internal/retrievaleval/... -> PASS=2 SKIP=1"
        status: pass
    human_judgment: false
  - id: D5
    description: "Zero remaining qdrant.NewClient call sites (outside comments) in internal/server, internal/e2e, internal/retrievaleval test files"
    requirement: "REQ-test-client-parity"
    verification:
      - kind: unit
        ref: "rg -n 'qdrant[.]NewClient[(]' internal/server internal/e2e internal/retrievaleval | rg -v '^[^:]+:[0-9]+:\\s*//' | wc -l -> 0"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-09-19
status: complete
---

# Phase 1 Plan 3: internal/server, internal/e2e and internal/retrievaleval Converge on storetest Summary

**The three cross-package Qdrant-backed test suites now delegate container lifecycle to `storetest.Run` (retrievaleval via `IgnoreRequireQdrant`), and their five remaining `qdrant.NewClient` dial sites converge on `storetest.Dial`, closing REQ-test-client-parity's last non-in-package call sites.**

## Performance

- **Duration:** ~40 min
- **Completed:** 2026-09-19
- **Tasks:** 2 completed
- **Files modified:** 6

## Accomplishments

- `internal/server`'s `TestMain` collapses to `os.Exit(storetest.Run(m))`; `TestSharedQdrantAddressHonored` delegates to `storetest.AssertSharedAddressHonored(t)`. Deleted: `testQdrantAddr`/`testQdrantContainerBooted` vars, `requireQdrant`, `failOrSkipNoQdrant`, `terminateQdrant`, `TestRequireQdrant`, and the inline `"qdrant/qdrant:v1.19.1"` literal.
- Three server dial sites (`testDepsWithStore`, `dialWarnPendingMigrationsTestClient`, `dialRawQdrantClient`) converge on `storetest.Dial(t, storetest.RecvLimit)`; `TestBuildDepsFromEnvLoadsConfigOnce` reads its address from `storetest.Addr()` with a `storetest.SkipOrFailNoQdrant(t)` guard.
- `internal/e2e`'s `TestMain` keeps its early `storetest.RequireQdrant()` parse and local `go build` of the `engram` binary, then delegates to `storetest.Run(m)`; the previously-dead deferred `os.RemoveAll(tmp)` (unreachable under every `os.Exit` path) is dropped for an explicit call immediately before each exit. `startServer` and `spineReviewQdrantClient`/`newSpineReviewStore`/`pruneEnv` all read from `storetest.Addr()`/`storetest.Dial`/`storetest.SkipOrFailNoQdrant`. `console_browser_test.go`'s two comments naming the deleted e2e helpers now point at their `storetest` equivalents.
- `internal/retrievaleval`'s `TestMain` keeps the `ENGRAM_RETRIEVAL_EVAL` opt-in gate as its literal first statement, then delegates to `storetest.Run(m, storetest.IgnoreRequireQdrant())` — preserving its pre-phase behavior of never consulting `ENGRAM_REQUIRE_QDRANT`. `newTestcontainerStore` narrows to `(t testing.TB, dim uint64) *store.Store`, dialing via `storetest.Dial(t, storetest.RecvLimit)`.
- Deleted the package-local `qdrantImageTag` constant and `testQdrantAddr`/`testQdrantContainerBooted` vars from `retrievaleval`; `TestSharedQdrantAddressHonored` keeps its `ENGRAM_RETRIEVAL_EVAL` skip first, then delegates to `storetest.AssertSharedAddressHonored(t)`.
- Verified live: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ ./internal/e2e/ ./internal/retrievaleval/ -count=1` passes; the shared-address count across these three packages is exactly `PASS=2 SKIP=1`; `ENGRAM_REQUIRE_QDRANT=treu` still fails closed for server and e2e while retrievaleval ignores it via the shared path.

## Task Commits

Each task was committed atomically:

1. **Task 1: internal/server end to end on storetest: TestMain, three dial sites, shared-address test** - `e7572f90` (test)
2. **Task 2: internal/e2e and internal/retrievaleval on storetest, each preserving its own harness quirks** - `283e80eb` (test)

**Plan metadata:** commit pending (this SUMMARY + STATE/ROADMAP)

## Files Created/Modified

- `internal/server/tools_test.go` — `TestMain`/`TestSharedQdrantAddressHonored` delegate to storetest; `testDepsWithStore`/`dialWarnPendingMigrationsTestClient` dial via `storetest.Dial`; `TestBuildDepsFromEnvLoadsConfigOnce` reads `storetest.Addr()`; deleted harness duplicates.
- `internal/server/schemaversion_wire_test.go` — `dialRawQdrantClient` delegates to `storetest.Dial`.
- `internal/e2e/harness_test.go` — `TestMain` keeps the early parse + local binary build, delegates lifecycle to `storetest.Run`; `startServer` reads `storetest.Addr()`; deleted harness duplicates.
- `internal/e2e/spine_review_test.go` — `spineReviewQdrantClient` delegates to `storetest.Dial`; every address check reads `storetest.Addr()`.
- `internal/e2e/console_browser_test.go` — two comments reworded to name `storetest.RequireQdrant`/`storetest.SkipOrFailNoQdrant` instead of the deleted e2e helpers.
- `internal/retrievaleval/retrieval_eval_test.go` — `TestMain` keeps the opt-in gate, delegates via `storetest.Run(m, storetest.IgnoreRequireQdrant())`; `newTestcontainerStore` narrows and dials via `storetest.Dial`; deleted harness duplicates.

## Decisions Made

- retrievaleval's `IgnoreRequireQdrant` option is exercised exactly as designed: a `treu` value for `ENGRAM_REQUIRE_QDRANT` is silently ignored by this package (verified live), while the same value still fails closed for server and e2e — proving the divergence is intentional and gate-enforced, not accidental drift.
- e2e's dead deferred cleanup (`defer func() { _ = os.RemoveAll(tmp) }()`, unreachable because every original exit path used `os.Exit`) was dropped per the plan's explicit instruction rather than preserved as inert code.

## Deviations from Plan

None — plan executed exactly as written.

**Total deviations:** 0. **Impact:** None.

## Issues Encountered

None. A local Qdrant container was started manually (`docker run ... qdrant/qdrant:v1.19.1` on `127.0.0.1:6333/6334`) to exercise the `ENGRAM_QDRANT_TEST_ADDR` shared-instance path during verification, mirroring CI's `services.qdrant` container; it was stopped after verification completed.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Five of eleven test `qdrant.NewClient` call sites converged in this plan (six converged in 01-02); all eleven are now on `store.NewQdrantClient` (directly or via `storetest.Dial`).
- `internal/store` stays green except the single expected `TestRedEvidencePatchesAreLive` red (verified unchanged, not touched by this plan); plan 01-05's Task 3 closes it.
- `go test ./internal/keylinks/ -count=1` passes (checked below).
- No blockers or concerns for the next plan (01-04, `internal/store`'s own harness migration).

---
*Phase: 01-test-harness-fixture-helper*
*Completed: 2026-09-19*

## Self-Check: PASSED

All 6 modified files verified present on disk. Both task commits (`e7572f90`, `283e80eb`) verified present in `git log --oneline --all`. Every acceptance criterion for both tasks re-run and confirmed passing (zero remaining harness duplicates, zero remaining `qdrant.NewClient` sites outside comments, `storetest.Dial(t, storetest.RecvLimit)` count = 3 in server files, `os.Exit(storetest.Run(m))` present exactly once in `tools_test.go`, fail-closed `treu` parsing preserved for server and e2e, `storetest.Run(m, storetest.IgnoreRequireQdrant())` present exactly once in retrievaleval with the eval gate preceding it, `go vet`/`golangci-lint` clean for all three packages, `task lint` passes repo-wide, `git diff --exit-code main...HEAD -- go.mod go.sum` exits 0, and the plan-level shared-address count is `PASS=2 SKIP=1`).
