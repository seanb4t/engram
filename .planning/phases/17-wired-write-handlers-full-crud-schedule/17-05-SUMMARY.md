---
phase: 17-wired-write-handlers-full-crud-schedule
plan: 05
subsystem: testing
tags: [connect-rpc, mcp, authz-parity, qdrant, ci, go-ast]

requires:
  - phase: 17-wired-write-handlers-full-crud-schedule
    provides: "17-04's six wired Connect write RPCs + rewired read handlers (all thin adapters onto deps.*)"
provides:
  - "TestWriteParity: one spy-backed row per write RPC proving MCP<->Connect delegation parity (SC2/D-10, REQ-connect-write-authz-parity acceptance gate)"
  - "A source/AST sub-test proving each Connect write handler names its deps.* method (closes the storeMemory/scheduleMemory shared-store-trace ambiguity)"
  - "TestCrossOwnerRewrap: split short_id/direct-UUID cross-owner leak tables for UpdateMemory/DeleteMemory/SetVisibility (SC4/D-11)"
  - "requireQdrant() fail-closed ENGRAM_REQUIRE_QDRANT gate wired into TestMain/testDeps + the CI test job (round-6 MED — no more silent-skip authz gate)"
affects: [phase-18-session-rotation]

tech-stack:
  added: []
  patterns:
    - "Spy-based delegation parity table (spyStore below deps, per-lane independent fixtures) cloning TestRerankParityMCPAndConnect's dual-closure shape"
    - "Source/AST delegation assertion via go/parser + runtime.Caller (not relative os.ReadFile) to prove a handler names its deps.* method"
    - "Single env-parse choke point (requireQdrant) driving a fail-closed CI gate instead of a silent local-skip default"

key-files:
  created:
    - internal/server/connectapi_write_parity_test.go
    - internal/server/connectapi_crossowner_test.go
  modified:
    - internal/server/tools_test.go
    - .github/workflows/ci.yaml

key-decisions:
  - "assertCodeParity maps the direct MCP-lane domain error through the production connectError(ctx, err) before connect.CodeOf, comparing it against connect.CodeOf(handlerErr) on the Connect lane — never a hand-rolled oracle (round-3 MED-6)"
  - "traceKey (Method+Owner only) is used for CREATE rows (fresh UUID per lane by design); assertSameStoreTraceExact (full Method+Owner+Args) is used for by-id rows (both lanes act on the same pre-seeded id)"
  - "StoreMemory asserts a non-empty, lane-appropriate Memory.Actor per lane (MCP: bearer TokenInfo.UserID; Connect: resolved-owner fallback), never cross-lane byte equality — a false invariant for a non-email owner (round-4 MED)"
  - "requireQdrant() is the sole ENGRAM_REQUIRE_QDRANT read/parse point; a malformed value returns a non-nil error rather than coercing to false, so a CI typo cannot silently re-enable skipping (round-8 LOW)"

requirements-completed: [REQ-connect-write-authz-parity]

coverage:
  - id: D1
    description: "Per-RPC MCP<->Connect spy delegation parity for all six write RPCs (StoreMemory, StoreDiscovery, ScheduleMemory, UpdateMemory x3, DeleteMemory, SetVisibility), each row on independently seeded per-lane fixtures"
    requirement: "REQ-connect-write-authz-parity"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_write_parity_test.go#TestWriteParity"
        status: pass
    human_judgment: false
  - id: D2
    description: "Source/AST sub-test proving each Connect write handler body invokes its named deps.* method (delegation proof distinct from the spy's store-trace proof)"
    requirement: "REQ-connect-write-authz-parity"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_write_parity_test.go#TestWriteParity/source_delegates_to_named_deps_methods"
        status: pass
    human_judgment: false
  - id: D3
    description: "Split short_id/direct-UUID cross-owner leak tables for the three by-id write RPCs (no resolved-UUID leak on short_id input; supplied UUID echoed on direct-UUID input)"
    verification:
      - kind: integration
        ref: "internal/server/connectapi_crossowner_test.go#TestCrossOwnerRewrap"
        status: pass
    human_judgment: false
  - id: D4
    description: "Fail-closed ENGRAM_REQUIRE_QDRANT gate: TestMain/testDeps fail startup instead of skipping under the flag, CI test job sets it, requireQdrant() rejects malformed values"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestRequireQdrant"
        status: pass
      - kind: integration
        ref: "ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/... -run TestConnectCookieLaneIsolation -v (no SKIP)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Idempotency-ban gate re-asserts green now that real write-RPC logic exists (SC5/D-12)"
    verification:
      - kind: other
        ref: "grep -rn idempotency_level proto/engram/v1/*.proto (no match); task proto:lint"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-07-12
status: complete
---

# Phase 17 Plan 05: Write-RPC parity, cross-owner leak split, fail-closed Qdrant gate Summary

**Spy-based per-RPC MCP<->Connect delegation parity table with a source/AST wrapper-name assertion, split short_id/UUID cross-owner leak tests, and a fail-closed ENGRAM_REQUIRE_QDRANT CI gate — closing REQ-connect-write-authz-parity and #322.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 2
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- `TestWriteParity` proves, per write RPC, that the direct MCP-lane `deps.*` call and the Connect handler call produce an identical store trace (spy, below `deps`) and map to the same Connect error code via the production `connectError` mapper — with each row seeded on its own independent spy fixture so a mutating direct-lane call never bleeds into the Connect-lane call.
- A `go/parser`/`runtime.Caller`-based source/AST sub-test proves each of the six Connect write handler bodies textually invokes its NAMED `deps.*` method, closing the gap the spy alone cannot close (storeMemory/scheduleMemory share `MintShortID`+`Upsert`, so a wrong-method handler could otherwise forge an identical store trace).
- `TestCrossOwnerRewrap` splits the by-id-RPC cross-owner leak assertion into two non-contradictory cases per RPC: short_id input (message excludes the resolved UUID) and direct-UUID input (message echoes exactly the supplied UUID).
- `requireQdrant() (bool, error)` is now the sole `ENGRAM_REQUIRE_QDRANT` parse point; `TestMain` fails startup (non-zero exit) instead of skipping when the flag is set and Qdrant is unavailable, `testDeps`/`testDepsWithStore` fail instead of skip via a shared `failOrSkipNoQdrant` helper, and the CI `test` job now sets the flag — a Docker/image-pull failure reddens CI instead of silently skipping the real-store authz gate.

## Task Commits

Each task was committed atomically:

1. **Task 1: Per-RPC MCP<->Connect spy delegation parity table (D-10)** - `438674a1` (test)
2. **Task 2: Split cross-owner leak tables (SC4/D-11) + real-store isolation gate + idempotency-ban re-assert (SC5/D-12)** - `36235a7f` (test)

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/server/connectapi_write_parity_test.go` - `TestWriteParity`: per-RPC spy delegation parity table + source/AST delegation sub-test
- `internal/server/connectapi_crossowner_test.go` - `TestCrossOwnerRewrap`: split short_id/direct-UUID cross-owner leak tables
- `internal/server/tools_test.go` - `requireQdrant()`, `failOrSkipNoQdrant()`, fail-closed `TestMain`/`testDepsWithStore`/`TestBuildDepsFromEnvLoadsConfigOnce`, `TestRequireQdrant`
- `.github/workflows/ci.yaml` - `ENGRAM_REQUIRE_QDRANT: "1"` on the `test` job's `go test ./...` step

## Decisions Made

- `assertCodeParity` maps the direct MCP-lane domain error through the production `connectError(ctx, err)` before `connect.CodeOf`, comparing against `connect.CodeOf(handlerErr)` on the Connect lane — apples-to-apples, never a hand-rolled test oracle (round-3 MED-6).
- Store-trace comparison uses two granularities: `assertSameStoreTrace` (Method+Owner only) for CREATE rows, where a fresh UUID is minted per lane by design; `assertSameStoreTraceExact` (full Method+Owner+Args) for by-id rows, where both lanes act on the same pre-seeded id and full call equality is the stronger, correct proof.
- StoreMemory's actor assertion is non-empty + lane-appropriate (MCP: bearer `TokenInfo.UserID`; Connect: resolved-owner fallback), never cross-lane byte equality — a false invariant for a non-email owner (round-4 MED).
- `requireQdrant()` returns a non-nil error on a malformed (non-empty, unparseable) `ENGRAM_REQUIRE_QDRANT` value rather than coercing it to `false`, so a CI typo cannot silently re-enable skipping and defeat the fail-closed gate (round-8 LOW).

## Deviations from Plan

None — plan executed exactly as written. All acceptance criteria and verification commands pass as specified.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 17 (wired-write-handlers-full-crud-schedule) is now fully executed: all six write RPCs are live, wired, authz-parity-proven, and the real-Qdrant isolation gate is fail-closed in CI.
- REQ-connect-write-authz-parity is closed; #322 is closable.
- Phase 18 (session rotation) can proceed — no blockers surfaced by this plan.

---
*Phase: 17-wired-write-handlers-full-crud-schedule*
*Completed: 2026-07-12*

## Self-Check: PASSED

All created/modified files and both task commit hashes verified present on disk / in git history.
