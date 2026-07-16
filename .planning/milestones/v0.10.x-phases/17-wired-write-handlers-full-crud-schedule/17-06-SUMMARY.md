---
phase: 17-wired-write-handlers-full-crud-schedule
plan: 06
subsystem: api
tags: [go, mcp, connect-rpc, read-path, refactor]

# Dependency graph
requires:
  - phase: 17-02
    provides: caller struct, callerFromContext/callerFromConnectContext, memStore interface, mutationResult
provides:
  - "coreListRequest/coreListResult/coreSearchRequest: transport-neutral typed read contract, a SUPERSET of the MCP and Connect read lanes"
  - "deps.listMemory/searchMemory returning typed []store.Memory (no []any), caller-threaded, no internal Limit/K default"
  - "deps.getMemory/listScheduled/searchDiscovery caller-threaded (return types unchanged)"
  - "MCP list_memory/search_memory tool closures owning shapeRecall shaping and the MCP-lane defaults (limit=20, k=8, CursorMode=true)"
affects: [17-04, 17-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Typed core read contract as a superset convergence point: each transport applies its own defaults/parsing at its boundary, never inside the shared deps method"
    - "Per-lane default discipline (no internal Limit/K default in the shared core) mirrored from the write-lane caller-threading pattern in 17-02"

key-files:
  created: []
  modified:
    - internal/server/tools.go
    - internal/server/tools_test.go
    - internal/server/connectapi_test.go
    - internal/server/embed_wiring_test.go

key-decisions:
  - "coreListRequest/coreListResult/coreSearchRequest defined as a superset of both lanes (offset, categories, visibility, exact total, cursor/cursor_mode, tags, created window) so 17-04's Connect read rewire drops nothing"
  - "No internal Limit/K default lives in deps.listMemory/searchMemory; each lane applies its own (MCP 20/8 in the tool closures, Connect leaves limit=0 as 'all' and applies k=20 in 17-04) — same discipline as the write-lane caller pattern"
  - "deps.searchDiscovery intentionally retains its internal k=8 default (MCP lane only), with a retention comment, since the Connect SearchDiscoveries adapter (17-04) pre-applies k=20"
  - "MCP recall shaping (shapeRecall -> []any) moved out of deps.listMemory/searchMemory into the MCP list_memory/search_memory tool closures; the list closure explicitly sets CursorMode: true to preserve today's unconditional MCP cursor-mode pagination"
  - "TestListMemoryRejectsBadWindow relocated to assert parseRFC3339 directly (the exact call the MCP closures make) since the typed core's CreatedAfter/CreatedBefore are time.Time and cannot carry an invalid string"

requirements-completed: [REQ-connect-write-authz-parity]

coverage:
  - id: D1
    description: "Typed core read contract (coreListRequest/coreListResult/coreSearchRequest) exists as a superset of both transport lanes"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestListMemorySupersetOffsetAndCursorModes"
        status: pass
    human_judgment: false
  - id: D2
    description: "deps.listMemory/searchMemory return typed []store.Memory (no []any), caller-threaded, no internal default"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestSearchListMemoryTagsHandler"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestRerankParityMCPAndConnect"
        status: pass
    human_judgment: false
  - id: D3
    description: "MCP list_memory closure preserves unconditional cursor-mode pagination (tokenless first page issues a non-empty next_cursor)"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestListMemoryReturnsNextCursorField"
        status: pass
    human_judgment: false
  - id: D4
    description: "Offset mode and cursor mode are mutually exclusive on the shared path; offset asserts exact Total + empty NextToken, cursor asserts non-empty NextToken"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestListMemorySupersetOffsetAndCursorModes"
        status: pass
    human_judgment: false

duration: 27min
completed: 2026-07-12
status: complete
---

# Phase 17 Plan 06: Typed core read contract (D-07 read convergence) Summary

**Transport-neutral coreListRequest/coreListResult/coreSearchRequest superset contract; deps.listMemory/searchMemory now return typed []store.Memory (no []any), caller-threaded like the write lane, with MCP recall shaping and per-lane defaults moved into the MCP tool closures.**

## Performance

- **Duration:** 27 min
- **Started:** 2026-07-12T23:04:00Z (approx.)
- **Completed:** 2026-07-12T23:31:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Defined `coreListRequest`/`coreListResult`/`coreSearchRequest` as a strongly-typed superset of both the MCP and Connect read contracts (offset, categories, visibility, exact total, cursor/cursor_mode, tags, created window) — the neutral convergence point 17-04 rewires the Connect read handlers onto.
- Refactored `deps.listMemory`/`deps.searchMemory` to return typed `[]store.Memory` (list also returns exact `Total` + `NextToken`) instead of MCP-shaped `[]any`, threading an explicit `caller` instead of reading `subjectFromContext(ctx)` internally.
- Threaded `caller` through `getMemory`, `listScheduled`, and `searchDiscovery` too (their return types are unchanged).
- Removed the internal `Limit==0 -> 20` and `K==0 -> 8` defaults from the shared list/search core (round-4 finding-7 discipline); the MCP `list_memory`/`search_memory` tool closures now apply the MCP-lane defaults (limit=20, k=8) and the closures parse `created_after`/`created_before` themselves via `parseRFC3339` before building the core request.
- The `list_memory` closure explicitly sets `CursorMode: true` on its `coreListRequest`, preserving today's unconditional MCP cursor-mode pagination (the neutral core no longer hard-codes it).
- `deps.searchDiscovery` intentionally retains its internal `k=8` default (MCP lane only) with a retention comment, since the Connect `SearchDiscoveries` adapter (17-04) pre-applies `k=20`.
- Updated every direct read-method call site across `tools_test.go`, `connectapi_test.go`, and `embed_wiring_test.go` to the new caller-threaded, typed-core signatures.
- Added `TestListMemorySupersetOffsetAndCursorModes`, split into two subtests matching the store's mutually-exclusive offset/cursor semantics: offset mode asserts an exact `Total` distinct from the page length with an empty `NextToken`; cursor mode asserts a non-empty `NextToken` on a full first page that resumes to the next page.

## Task Commits

Each task was committed atomically:

1. **Task 1: Define the typed core read contract + refactor deps read methods** - `0d334ed3` (feat)
2. **Task 2: Update read-path test call sites + superset regression coverage** - `dfbc815a` (test)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/server/tools.go` - typed core read request/result types; `deps.listMemory`/`searchMemory`/`getMemory`/`listScheduled`/`searchDiscovery` refactored; MCP `list_memory`/`search_memory`/`list_scheduled`/`get_memory`/`search_discovery` tool closures updated to build caller + core requests and own recall shaping/defaults
- `internal/server/tools_test.go` - all direct read-method call sites updated to the new signatures; `TestListMemoryRejectsBadWindow` relocated to the `parseRFC3339` boundary; new `TestListMemorySupersetOffsetAndCursorModes` regression test
- `internal/server/connectapi_test.go` - `TestRerankParityMCPAndConnect`'s `mcpIDs` closure updated to the typed-core signature, dropping the `recallView` type assertion
- `internal/server/embed_wiring_test.go` - `TestSearchMemoryUsesEmbedQuery`'s search-path call site updated to `(ctx, caller, coreSearchRequest)` with an explicit anonymous caller

## Decisions Made
- Same per-lane-default discipline as the 17-02 write-lane caller pattern: the shared core carries zero implicit defaults, each transport adapter supplies its own before calling in. This keeps MCP's k=8/limit=20/CursorMode=true and Connect's k=20/limit=0="all" both correct on one shared path.
- `TestListMemoryRejectsBadWindow`'s intent (bad-window rejection) is preserved by asserting `parseRFC3339("nope")` directly rather than building a full MCP client/server round-trip harness through `Register`+`mcp.NewInMemoryTransports` — the MCP tool closures are unexported and only reachable via the wire protocol, and `parseRFC3339` is the exact boundary function both the MCP list_memory/search_memory closures and the Connect handlers call. This is a proportionate substitute for a full protocol round-trip test given no existing test infra registers closures independently of `Register`'s env-based `buildDepsFromEnv`.

## Deviations from Plan

None - plan executed exactly as written, including all round-3/4/5/6/8 review refinements baked into the plan text (CursorMode: true preservation, offset/cursor split, time.Time core fields, k=8 retention comment, embed_wiring_test.go search-path update, TestListMemoryRejectsBadWindow relocation).

## Issues Encountered

None. `go build ./...`, `go vet ./...`, `go test ./internal/server/...` (full suite, live Qdrant via testcontainers), and `task lint:go` are all green. Task 1's commit intentionally leaves the `internal/server` test package non-compiling until Task 2's commit lands (both edit the same package; this mirrors the plan's own note that the two tasks are inseparable at compile time).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- The typed core read contract is ready for 17-04 to rewire the Connect `ListMemories`/`SearchMemories` handlers onto without dropping any Connect field.
- `go build ./...`, `go vet ./...`, full `internal/server` test suite, and `task lint:go` are all green on this branch.

---
*Phase: 17-wired-write-handlers-full-crud-schedule*
*Completed: 2026-07-12*

## Self-Check: PASSED
