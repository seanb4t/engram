---
phase: 17-wired-write-handlers-full-crud-schedule
plan: 04
subsystem: api
tags: [connect-rpc, connectrpc, grpc-error-mapping, memstore, testing]

# Dependency graph
requires:
  - phase: 17-02
    provides: memStore interface carve, caller type, mutationResult, errRuleImmutable/errStaleSummary sentinels
  - phase: 17-03
    provides: protoconv request/response conversion layer for the six write RPCs
  - phase: 17-06
    provides: typed core read contract (coreListRequest/coreListResult/coreSearchRequest, deps.listMemory/searchMemory)
provides:
  - Single production connectError(ctx, err) mapper (typed sentinels -> Connect codes)
  - Six wired Connect write RPCs (StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory)
  - Connect read handlers (ListMemories, SearchMemories, GetMemory, SearchDiscoveries) rewired onto the typed core deps.*
  - Scripted-spy memStore fake (spyStore) for store-free handler tests
affects: [17-05, 18-session-rotation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single production error mapper (connectError) matching typed sentinels via errors.Is, never strings"
    - "Scripted-spy fake (spyStore) recording method+owner+args instead of reimplementing full store authz"
    - "Thin Connect write adapter: resolve caller -> protoconv -> deps.* -> protoconv -> connectError"

key-files:
  created:
    - internal/server/connecterror.go
    - internal/server/connecterror_test.go
    - internal/server/fakestore_test.go
  modified:
    - internal/server/connectapi.go
    - internal/server/connectapi_test.go
    - internal/server/connectapi_negative_test.go
    - internal/server/connectcsrf_test.go

key-decisions:
  - "connectError takes ctx as first param so the CodeInternal branch logs via slog.ErrorContext with request-scoped fields, and returns a generic non-leaking message"
  - "No CodeAborted arm in connectError — no distinct concurrency/edit-conflict sentinel exists today (round-5 LOW); add one only if such a sentinel is later introduced"
  - "spyStore mirrors ResolvePointID's real fast-path semantics (well-formed UUID returns without an existence check; non-UUID falls through to a short-id map lookup) so negative-matrix by-id cells against a scripted-absent id resolve to store.ErrNotFound the same way production does"
  - "TestWriteRPCNegativeMatrix's authenticated-valid cells: the three create RPCs assert err == nil (success); the three by-id RPCs assert CodeNotFound (id \"some-id\" is not a UUID and unseen by the spy) — not a uniform CodeOf(err) check"
  - "TestConnectCSRFTokenMatrix's happy-path cell asserts err == nil directly, not connect.CodeOf(nil) — CodeOf(nil) is CodeUnknown, not a success code (round-6 LOW)"
  - "SearchDiscoveries maps an empty Connect scope to CrossSpine=true (preserving any non-empty caller Scope) so an empty scope still spans ALL discovery scopes, matching the pre-rewire Store.SearchDiscovery contract"
  - "GetMemory's handler-level usage-enqueue call is removed; deps.getMemory is now the sole enqueue point (AccessCount +1 exactly, not +2)"
  - "ListMemories passes Connect limit=0 through unchanged as \"all\" (no re-introduced default) and parses created_after/before at the Connect boundary so malformed values return CodeInvalidArgument, never CodeInternal"

requirements-completed: [REQ-connect-write-authz-parity]

coverage:
  - id: D1
    description: "Single production connectError(ctx, err) mapper: typed sentinels (ErrNotFound, ErrInvalidArgument, errRuleImmutable, errStaleSummary, ErrAmbiguousShortID, context.Canceled, context.DeadlineExceeded) map to precise Connect codes; unknown errors map to CodeInternal with a generic non-leaking message"
    requirement: "REQ-connect-write-authz-parity"
    verification:
      - kind: unit
        ref: "internal/server/connecterror_test.go#TestConnectError"
        status: pass
    human_judgment: false
  - id: D2
    description: "Scripted-spy memStore (spyStore) backed by a non-nil embedder, recording method+owner+args, implementing the full memStore interface"
    verification:
      - kind: unit
        ref: "internal/server/fakestore_test.go#TestSpyStoreRecordsMethodAndSubject"
        status: pass
      - kind: unit
        ref: "internal/server/fakestore_test.go#TestSpyDepsStoreMemoryReachesEmbedder"
        status: pass
    human_judgment: false
  - id: D3
    description: "Six Connect write RPCs (StoreMemory, StoreDiscovery, UpdateMemory, DeleteMemory, SetVisibility, ScheduleMemory) wired as thin deps.* adapters; landmine 1 defused (negative matrix no longer expects CodeUnimplemented)"
    requirement: "REQ-connect-write-authz-parity"
    verification:
      - kind: integration
        ref: "internal/server/connectapi_negative_test.go#TestWriteRPCNegativeMatrix"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_test.go#TestConnectCSRFTokenMatrix"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestConnectStoreMemoryThenReadBack"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestConnectUpdateMemoryResponseCarriesCanonicalID"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestConnectSetVisibilityResponseCarriesCanonicalID"
        status: pass
    human_judgment: false
  - id: D4
    description: "Connect read handlers (ListMemories, SearchMemories, GetMemory, SearchDiscoveries) rewired onto the 17-06 typed core deps.* with no dropped field and k=20 preserved; ListScopes stays the documented D-07 exception"
    requirement: "REQ-connect-write-authz-parity"
    verification:
      - kind: integration
        ref: "internal/server/connectapi_cookie_test.go#TestConnectCookieLaneIsolation"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestRerankParityMCPAndConnect"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestConnectSearchDiscoveriesDefaultsK20"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestConnectSearchDiscoveriesEmptyScopeSpansAll"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestConnectGetMemoryEnqueuesUsageSignalExactlyOnce"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestConnectListMemoriesLimitZeroReturnsAll"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_test.go#TestListMemoriesRejectsBadCreatedBefore"
        status: pass
    human_judgment: false

duration: 17min
completed: 2026-07-12
status: complete
---

# Phase 17 Plan 04: Wired Write Handlers + Read-Lane Rewire Summary

**Six Connect write RPCs wired as thin deps.* adapters behind a single typed-sentinel connectError mapper, plus the four Connect read handlers rewired off direct store access onto the 17-06 typed core — closing Pitfall 1 for both writes and reads.**

## Performance

- **Duration:** 17 min
- **Started:** 2026-07-12T23:37:21Z
- **Completed:** 2026-07-12T23:54:22Z
- **Tasks:** 3
- **Files modified:** 7 (3 created, 4 modified)

## Accomplishments
- A single production `connectError(ctx, err)` mapper matches typed sentinels (`store.ErrNotFound`, `store.ErrInvalidArgument`, `errRuleImmutable`, `errStaleSummary`, `store.ErrAmbiguousShortID`, `context.Canceled`, `context.DeadlineExceeded`) to precise Connect codes; everything else maps to `CodeInternal` with a generic, non-leaking message logged via `slog.ErrorContext`.
- `internal/server/fakestore_test.go` adds a scripted-spy `memStore` (`spyStore`) backed by a non-nil embedder, so the six write RPCs can be exercised end-to-end without a live Qdrant while recording method+owner+args for the upcoming 17-05 delegation-parity test.
- `engramAPI` gains six real write methods (`StoreMemory`, `StoreDiscovery`, `UpdateMemory`, `DeleteMemory`, `SetVisibility`, `ScheduleMemory`), each a thin adapter: resolve caller → protoconv → the same `deps.*` method the MCP tool calls → protoconv → `connectError`. No handler touches `a.d.st.*` directly and none re-implements ownership comparison.
- `ListMemories`, `SearchMemories`, `GetMemory`, and `SearchDiscoveries` are rewired onto the 17-06 typed core (`deps.listMemory`/`deps.searchMemory`/`deps.getMemory`/`deps.searchDiscovery`), preserving every Connect field, the k=20 default on both search lanes, and the `limit=0` = "all" semantics; `ListScopes` remains the one documented D-07 exception.
- Landmine 1 fully defused: `TestWriteRPCNegativeMatrix` and `TestConnectCSRFTokenMatrix` construct spy-backed (non-nil store + embedder) deps and assert the real wired outcome instead of `CodeUnimplemented`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Production connectError mapper + scripted-spy fake + nil-store fixes** - `d15a6423` (feat)
2. **Task 2: Six Connect write handler methods** - `535292b9` (feat)
3. **Task 3: Rewire Connect read handlers onto the typed core** - `e94fc700` (feat)

**Plan metadata:** (this commit, appended after SUMMARY)

## Files Created/Modified
- `internal/server/connecterror.go` - the single production `connectError(ctx, err)` mapper
- `internal/server/connecterror_test.go` - `TestConnectError` table-test covering every mapping arm
- `internal/server/fakestore_test.go` - `spyStore` scripted-spy `memStore` + `newSpyDeps` helper
- `internal/server/connectapi.go` - six write handlers + four read handlers rewired onto `deps.*`
- `internal/server/connectapi_test.go` - SC3 create-then-read-back test, mutationResult canonical-id tests, and the five Task 3 read-lane regression tests
- `internal/server/connectapi_negative_test.go` - spy-backed deps + real per-case authenticated-valid outcomes
- `internal/server/connectcsrf_test.go` - spy-backed deps + happy-path cell flipped off `CodeUnimplemented`

## Decisions Made
- `connectError` takes `ctx` as its first parameter so the `CodeInternal` branch can log request-scoped fields via `slog.ErrorContext`, while returning a generic message to the client (no internal leak).
- No `CodeAborted` arm — no distinct concurrency/edit-conflict sentinel exists in the codebase today; one will be added only if such a sentinel is introduced later.
- `spyStore.ResolvePointID` mirrors the real store's fast-path behavior (a well-formed UUID resolves without an existence check; anything else falls through to a short-id lookup) so the negative-matrix's `"some-id"` scripted-absent cells resolve to `store.ErrNotFound` exactly as production would.
- `TestWriteRPCNegativeMatrix`'s authenticated-valid assertions are no longer a single uniform code check: the three create RPCs assert `err == nil` (success against the empty spy store), the three by-id RPCs assert `CodeNotFound` (the scripted id was never seeded).
- `TestConnectCSRFTokenMatrix`'s happy-path cell asserts `err == nil` directly rather than comparing against `connect.CodeOf(nil)`, since `CodeOf` returns `CodeUnknown` for a nil error.
- `SearchDiscoveries` maps an empty Connect request scope to `CrossSpine: true` (preserving any non-empty caller scope) to match the pre-rewire "empty scope = all discovery scopes" contract, since the shared `deps.searchDiscovery` otherwise rejects an empty scope.
- `GetMemory`'s former handler-level usage-enqueue call is removed; `deps.getMemory` is now the sole enqueue point, so `AccessCount` increments exactly once per Connect get.
- `ListMemories` passes Connect `limit=0` through unchanged (still means "all") and parses `created_after`/`created_before` at the Connect boundary so a malformed value returns `CodeInvalidArgument` directly, never reaching the typed core to be misclassified as `CodeInternal`.

## Deviations from Plan

None - plan executed exactly as written, including all round-1 through round-8 cross-AI review fixes baked into the plan text (connectError signature/mapping table, spy fake + non-nil embedder, CSRF-matrix re-pointing, SearchDiscoveries k=20 + empty-scope mapping, GetMemory exactly-once enqueue, ListMemories limit=0 passthrough, boundary-parsed created window).

## Issues Encountered

- Two doc-comment wordings (the six-write-handler adapter comment mentioning `a.d.st.*`, and per-handler comments mentioning "EmbedQuery"/"usageQueue.tryEnqueue" as the literal names of the removed calls) tripped the plan's own grep verification gates (`grep -nE 'a\.d\.st\.'`, `grep -c 'EmbedQuery'`, `grep -c 'usageQueue.tryEnqueue'` all needed to return 0/ListScopes-only). Reworded the comments to describe the removed calls without repeating their literal identifiers; re-verified all three grep gates return the exact required output.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 05 (delegation-parity test, MCP vs Connect hitting the identical `deps.*` method for the same subject) can now build directly on the `spyStore` call-recording infrastructure from this plan.
- The full `internal/server` package test suite is green (`go test ./internal/server/...`, `task lint:go` exits 0); `go build ./...` and `go test ./...` are clean across the repo.
- No blockers for Plan 05 or Plan 06 (already complete) — the write lane and read lane both terminate on the shared `deps.*`/`connectError` surface.

---
*Phase: 17-wired-write-handlers-full-crud-schedule*
*Completed: 2026-07-12*

## Self-Check: PASSED

All created/modified files and all three task commit hashes verified present on disk / in `git log`.
