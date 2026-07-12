---
phase: 17-wired-write-handlers-full-crud-schedule
plan: 02
subsystem: api
tags: [go, qdrant, connect-rpc, mcp, authz]

# Dependency graph
requires:
  - phase: 17-01
    provides: ordered-list ClaimIdentity, versioned session cookie, comma-list owner-claim config
provides:
  - "internal/server/store_iface.go: memStore interface (deps.st retyped from *store.Store to it)"
  - "store.Store.UpdatePayload: targeted, vector-preserving payload-only update (visibility/summary/summary_source/access_count/last_accessed_at), with provenance-clear via a distinct DeletePayload op"
  - "internal/server/identity.go: caller struct, callerFromTokenInfo (single choke point for both auth lanes), mutationResult, errRuleImmutable"
  - "every write deps.* method (storeMemory/scheduleMemory/storeDiscovery/updateMemory/deleteMemory/setVisibility) plus storeRule/listRules take an explicit caller"
  - "updateArgs.Content is *string with vector-vs-payload routing (deps.updateMemory)"
  - "parseWindow/validateRuleSummary rejections wrapped with store.ErrInvalidArgument"
affects: [17-03, 17-04, 17-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Store function-var seam (mintCandidate-style) for injecting a partial-failure path against a concrete *qdrant.Client with no interface seam (deletePayloadKeys)"
    - "caller{Subj, Actor} explicit-argument identity threading, replacing internal ctx-derived subjectFromContext/actorFromContext on the write lane"

key-files:
  created:
    - internal/server/store_iface.go
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/server/identity.go
    - internal/server/identity_test.go
    - internal/server/tools.go
    - internal/server/rules.go
    - internal/server/tools_test.go
    - internal/server/rules_test.go
    - internal/server/connectapi_test.go
    - internal/server/summaryqueue_test.go
    - internal/server/embed_wiring_test.go
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts

key-decisions:
  - "UpdatePayload persists via a TARGETED SetPayload (visibility/summary/summary_source/access_count/last_accessed_at) plus a separate targeted DeletePayload for summary_model/summary_egress_at — never a whole-payload OverwritePayload(payload(cur)), which a stale FetchForUpdate snapshot with no CAS could use to revert a concurrent content update while the new vector survives (content/vector desync)"
  - "The two-op SetPayload+DeletePayload sequence is accepted as non-atomic: a DeletePayload failure after SetPayload commits leaves stale provenance metadata only, never content/vector corruption; documented in the store method's godoc and covered by an injected-failure test via a deletePayloadKeys function-var seam"
  - "callerFromTokenInfo is the single choke point for both auth lanes; Actor falls back to the resolved owner when TokenInfo.UserID is empty (Connect cookie lane never sets UserID)"
  - "errRuleImmutable is a NEW sentinel; errStaleSummary is REUSED unchanged (not redeclared, closing round-2 review BLOCKER 2)"
  - "storeFill/buildUsageQueue stay on concrete *store.Store (memStore does not declare FillSummary/IncrementAccess); a new testDepsWithStore test helper returns both *deps and the concrete store for the three call sites that need it"

patterns-established:
  - "Payload-only store writes use a function-var seam (mirroring mintCandidate) to make Qdrant partial-failure paths testable without a broader client-interface refactor"

requirements-completed: [REQ-connect-write-authz-parity]

coverage:
  - id: D1
    description: "Payload-only vector-preserving store update (UpdatePayload) preserves the vector, bumps AccessCount/LastAccessedAt, and clears stale auto-summary provenance by key deletion"
    requirement: REQ-connect-write-authz-parity
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestUpdatePayloadPreservesVectorBumpsUsageAndClearsProvenance"
        status: pass
    human_judgment: false
  - id: D2
    description: "UpdatePayload's two-op non-atomicity is documented and covered by an injected DeletePayload-failure test (primary mutation committed, provenance stays stale)"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestUpdatePayloadInjectedDeletePayloadFailure"
        status: pass
    human_judgment: false
  - id: D3
    description: "memStore interface (including DeleteAll + ListScopes) exists; deps.st retyped; *store.Store satisfies it via compile assertion"
    verification:
      - kind: unit
        ref: "go build ./... (var _ memStore = (*store.Store)(nil))"
        status: pass
    human_judgment: false
  - id: D4
    description: "callerFromTokenInfo resolves Subj + Actor for both auth lanes, with Actor falling back to owner when UserID is empty; nil TokenInfo yields the anonymous caller"
    requirement: REQ-connect-write-authz-parity
    verification:
      - kind: unit
        ref: "internal/server/identity_test.go#TestCallerFromTokenInfoActorFallsBackToOwner"
        status: pass
      - kind: unit
        ref: "internal/server/identity_test.go#TestCallerFromTokenInfoUserIDWins"
        status: pass
      - kind: unit
        ref: "internal/server/identity_test.go#TestCallerFromTokenInfoAnonymousAndFailClosed"
        status: pass
    human_judgment: false
  - id: D5
    description: "Every write deps.* method (6 write + storeRule/listRules) takes an explicit caller; the full MCP write suite still passes"
    requirement: REQ-connect-write-authz-parity
    verification:
      - kind: unit
        ref: "go test ./internal/server/..."
        status: pass
    human_judgment: false
  - id: D6
    description: "updateArgs.Content is *string; nil content with unchanged tags routes to the payload-only method (no re-embed); deps.updateMemory/setVisibility return a typed mutationResult sourced from the fetched record"
    requirement: REQ-connect-write-authz-parity
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestUpdateMemoryReturnsMutationResult"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestSetVisibilityReturnsMutationResult"
        status: pass
    human_judgment: false
  - id: D7
    description: "Rule-immutability rejections are errors.Is-matchable via errRuleImmutable; parseWindow/validateRuleSummary rejections are errors.Is-matchable via store.ErrInvalidArgument"
    verification:
      - kind: unit
        ref: "internal/server/rules_test.go#TestUpdateMemoryRuleGuard"
        status: pass
      - kind: unit
        ref: "internal/server/rules_test.go#TestUpdateMemoryRuleGuardRejectsUnshare"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestScheduleMemoryValidation"
        status: pass
    human_judgment: false
  - id: D8
    description: "proto tags-only doc comment corrected (tags-only DOES re-embed); gen/go + gen/ts regenerated and drift-free"
    verification:
      - kind: other
        ref: "task proto:gen (re-run twice, sha256 stable)"
        status: pass
    human_judgment: false

# Metrics
duration: 25min
completed: 2026-07-12
status: complete
---

# Phase 17 Plan 02: Write-Lane Foundations Summary

**A payload-only vector-preserving store update, a narrow memStore interface, and a single caller-identity seam now thread through every write handler — the four prerequisites 17-04's Connect handler wiring needs to compile.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-07-12
- **Tasks:** 3 completed
- **Files modified:** 14 (1 created, 13 modified)

## Accomplishments

- Added `store.Store.UpdatePayload` — a targeted `SetPayload` (visibility/summary/summary_source/access_count/last_accessed_at) plus a separate targeted `DeletePayload` for stale auto-summary provenance keys, so a shared/summary-only update skips the re-embed entirely and the existing vector survives untouched. Verified via a raw-Qdrant `WithVectors(true)` before/after comparison and a raw-payload key-absence check (not just the decoded struct).
- Added `internal/server/store_iface.go`'s `memStore` interface (including `DeleteAll`/`ListScopes`, whose omission was the round-1 review's consensus BLOCKER) and retyped `deps.st` to it — a pure interface carve verified by `var _ memStore = (*store.Store)(nil)`.
- Added the `caller{Subj, Actor}` identity seam (`callerFromTokenInfo`, `callerFromContext`, `callerFromConnectContext`) as the single choke point both auth lanes build a caller through; `Actor` falls back to the resolved owner when `TokenInfo.UserID` is empty (the Connect cookie lane never sets it).
- Threaded `caller` through all six write `deps.*` methods plus `storeRule`/`listRules`, retyped `updateArgs.Content` to `*string` with vector-vs-payload routing in `deps.updateMemory`, and made `updateMemory`/`setVisibility` return a typed `mutationResult`.
- Wrapped rule-immutability rejections with the new `errRuleImmutable` sentinel and `parseWindow`/`validateRuleSummary` rejections with the existing `store.ErrInvalidArgument`, both `errors.Is`-matchable for 17-04's Connect error mapper.
- Corrected the proto's tags-only doc comment (a tags-only update DOES re-embed, since tags are folded into the embedded document) and regenerated `gen/go` + `gen/ts` (verified drift-free by running `task proto:gen` twice and comparing sha256 hashes).

## Task Commits

Each task was committed atomically:

1. **Task 1: Payload-only store update + memStore interface** - `2291197` (feat)
2. **Task 2: caller seam + mutationResult + typed sentinels** - `fd0c0e7` (feat)
3. **Task 3: Thread caller through write methods + vector/payload routing** - `ae8eed1` (feat)

_Note: Task 1's and Task 3's changes to `internal/server/tools.go`/`tools_test.go` were implemented together in a single working session (Task 3's caller-threading depends directly on Task 1's `deps.st` retype in the same functions) and were split across the two commits by file, not by hunk — the `deps.st` field retype, `testDepsWithStore` helper, and the `buildUsageQueue` test call-site fix landed in Task 1's commit; everything else in `tools.go`/`tools_test.go` landed in Task 3's commit. `internal/server/summaryqueue_test.go` (a Task 1 file per the plan frontmatter) is committed with Task 3 because its `storeFill(st, ...)` call site depends on `testDepsWithStore`, which lives in `tools_test.go`._

**Plan metadata:** (this commit, pending)

## Files Created/Modified

- `internal/store/store.go` - `Store.UpdatePayload` (targeted payload-only write, provenance clear via `DeletePayload`), `deletePayloadKeys` function-var test seam
- `internal/store/store_test.go` - vector-preservation, usage-bump, provenance-clear, and injected-failure tests for `UpdatePayload`
- `internal/server/store_iface.go` (new) - the `memStore` interface + compile assertion
- `internal/server/identity.go` - `caller`, `callerFromTokenInfo`, `callerFromContext`, `callerFromConnectContext`, `mutationResult`, `errRuleImmutable`
- `internal/server/identity_test.go` - caller-resolution unit tests (Actor fallback, UserID-wins, anonymous, fail-closed, Connect fail-closed)
- `internal/server/tools.go` - `deps.st` retyped to `memStore`; caller threaded through all six write methods; `updateArgs.Content *string` + vector/payload routing; `mutationResult` returns; `parseWindow` wrapped with `store.ErrInvalidArgument`; MCP tool registrations build a `caller` once via `callerFromContext`
- `internal/server/rules.go` - `storeRule`/`listRules` take `caller`; `validateRuleSummary` wrapped with `store.ErrInvalidArgument`
- `internal/server/tools_test.go` - `callerFor`/`strp` test helpers; `testDepsWithStore`; every write-method call site updated to the new signatures; new `mutationResult`/`store.ErrInvalidArgument` assertion tests
- `internal/server/rules_test.go` - every write-method call site updated; `errRuleImmutable`/`store.ErrInvalidArgument` assertions added to the existing rule-guard tests
- `internal/server/connectapi_test.go` - `storeMemory` call sites updated to the new signature
- `internal/server/summaryqueue_test.go` - `storeFill` call sites re-pointed at the concrete store via `testDepsWithStore`
- `internal/server/embed_wiring_test.go` - `TestStoreMemoryEmbedsContentPlusTags` updated with an explicit anonymous caller
- `proto/engram/v1/engram.proto` - `UpdateMemoryRequest` doc comment corrected (tags-only re-embeds)
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts` - regenerated (doc-comment-only diff)

## Decisions Made

- `UpdatePayload` uses a TARGETED two-op `SetPayload`+`DeletePayload` (not a whole-payload `OverwritePayload`) per the plan's round-7 finding: a whole-payload write from a stale `FetchForUpdate` snapshot with no compare-and-swap can revert a concurrent content update while the new vector survives, causing durable content/vector desync.
- The real `*qdrant.Client` has no interface seam, so the injected-failure test for the two-op non-atomicity uses a `deletePayloadKeys` function-var field on `Store` (mirroring the existing `mintCandidate` test-override pattern) rather than a broader client-interface refactor.
- `updateArgs.Content` stays a single struct field (no separate MCP-only wire type): it changes to `*string` with no `omitempty` tag, so the MCP jsonschema still treats it as required and the MCP tool-registration handler passes the decoded `updateArgs` straight through — MCP's full-replace behavior is unchanged.
- The rule-immutable and stale-summary error messages are preserved byte-for-byte by wrapping with `%w` mid-string (e.g. `fmt.Errorf("%w — delete the rule instead of making it private", errRuleImmutable)`), so existing `strings.Contains` assertions keep passing alongside the new `errors.Is` assertions.

## Deviations from Plan

None — plan executed exactly as written. The task-commit-boundary note above (Task 1/Task 3 file-split in `tools.go`/`tools_test.go`/`summaryqueue_test.go`) is a process note about how the atomic commits were split, not a functional deviation: all three commits build and pass tests in sequence, and the final state matches every acceptance criterion in the plan.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

17-03 (protoconv response mapping) and 17-04 (Connect handler wiring, including the `connectError` mapper) can now consume: the `memStore` interface (for a Connect-lane fake), `mutationResult` (for populating `UpdateMemoryResponse`/`SetVisibilityResponse` without a re-fetch), and the typed sentinels (`errRuleImmutable`, `errStaleSummary`, `store.ErrInvalidArgument`) for `errors.Is`-based error-code mapping. No blockers.

---
*Phase: 17-wired-write-handlers-full-crud-schedule*
*Completed: 2026-07-12*

## Self-Check: PASSED

- FOUND: internal/server/store_iface.go
- FOUND: .planning/phases/17-wired-write-handlers-full-crud-schedule/17-02-SUMMARY.md
- FOUND commit: 2291197 (feat: payload-only store update + memStore interface)
- FOUND commit: fd0c0e7 (feat: caller seam + mutationResult + typed sentinels)
- FOUND commit: ae8eed1 (feat: thread caller through write methods)
