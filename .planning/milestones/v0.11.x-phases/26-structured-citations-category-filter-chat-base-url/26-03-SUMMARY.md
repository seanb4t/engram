---
phase: 26-structured-citations-category-filter-chat-base-url
plan: 03
subsystem: api
tags: [protobuf, buf, connect-rpc, category-filter, parity]

# Dependency graph
requires:
  - phase: 26-02
    provides: coreSearchRequest.Categories / coreListRequest.Categories already threaded into the MCP search_memory/list_memory closures and the shared store core
provides:
  - "SearchMemoriesRequest.categories = 8 (additive proto field, no buf.validate allowlist)"
  - "Connect SearchMemories handler forwards req.Msg.Categories into the shared coreSearchRequest"
  - "MCP<->Connect category-filter parity proven by test for both search and list"
affects: [console, connect-clients, future-proto-field-additions]

# Tech tracking
tech-stack:
  added: []
  patterns: ["additive proto field + regenerate-and-diff-check as the standard drift-free codegen workflow"]

key-files:
  created: []
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/server/connectapi.go
    - internal/server/connectapi_test.go
    - internal/server/connectdescriptor_test.go

key-decisions:
  - "D-10 checkpoint (one-way field-number commitment) resolved by the human user as ship-field-8 — a genuine interactive decision, not an auto-approval, made after being told explicitly the commitment is irreversible and that D-10 in 26-CONTEXT.md was auto-generated rather than human-authored"
  - "D-11 confirmed: the new categories field carries NO buf.validate annotation, matching ListMemoriesRequest.categories (field 4) and deliberately not copying StoreMemoryRequest.category's write-domain in: allowlist"

patterns-established:
  - "Regenerate-then-diff-check (`task proto:gen && git diff --exit-code -- gen/ ui/src/lib/gen/`) run AFTER the commit, not before, to prove the committed trees are byte-identical to a fresh regeneration"

requirements-completed: [REQ-category-filter]

coverage:
  - id: D1
    description: "Connect SearchMemories accepts an additive categories filter field (SearchMemoriesRequest.categories = 8), closing the MCP<->Connect parity gap in the search direction"
    requirement: "REQ-category-filter"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestMCPConnectCategoryFilterParity"
        status: pass
    human_judgment: false
  - id: D2
    description: "The new field carries no write-domain buf.validate allowlist, so discovery/rule category values are accepted at the proto boundary"
    requirement: "REQ-category-filter"
    verification:
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestConnectSearchUnknownCategory"
        status: pass
    human_judgment: false
  - id: D3
    description: "Regenerated gen/go, gen/ts, and ui/src/lib/gen trees committed alongside the schema change, drift-free"
    verification:
      - kind: other
        ref: "task proto:gen && git diff --exit-code -- gen/ ui/src/lib/gen/ (exit 0)"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-25
status: complete
---

# Phase 26 Plan 03: Connect SearchMemories Category Filter Summary

**Added `repeated string categories = 8` to `SearchMemoriesRequest` (D-10, human-approved as a one-way field-number commitment), wired it into the Connect handler, and proved MCP<->Connect parity with a same-filter-same-order test — closing the search-side half of SC2's category-filter parity gap.**

## Performance

- **Duration:** 12 min
- **Tasks:** 2 completed (checkpoint pre-resolved by human, not re-presented)
- **Files modified:** 7 (proto, 3 generated trees, Connect handler, descriptor test, parity test)

## Accomplishments
- `SearchMemoriesRequest.categories = 8` shipped additive, no `buf.validate` annotation (D-11)
- `gen/go`, `gen/ts`, and `ui/src/lib/gen` regenerated and committed in the same commit as the `.proto` edit; confirmed byte-identical to a fresh regeneration post-commit
- Connect `SearchMemories` handler forwards `req.Msg.Categories` into the shared `coreSearchRequest`, mirroring `ListMemories`' existing `Categories` wiring exactly
- `connectdescriptor_test.go`'s pinned `SearchMemoriesRequest` field count raised 7 → 8
- `TestMCPConnectCategoryFilterParity` proves identical ordered id lists between MCP and Connect for both the search pair and the list pair
- `TestConnectSearchUnknownCategory` proves `discovery` passes `protovalidate.Validator` (no `InvalidArgument`) and an unmatched category returns an empty list with a nil error

## Task Commits

Each task was committed atomically:

1. **Checkpoint (D-10 decision)** — resolved interactively by the human user as `ship-field-8` in the orchestrator, prior to this executor being spawned. Not re-presented; treated as complete per the executor's launch instructions.
2. **Task 1: Add the additive proto field, regenerate, and wire the Connect handler (D-10/D-11)** - `f6b02fc8` (feat)
3. **Task 2: Prove MCP and Connect return the same set for the same category filter** - `a63b12f0` (test)

**Plan metadata:** commit follows this SUMMARY

_Note: Task 2 is a pure proof test against Task 1's already-complete implementation — there is no separate RED/GREEN split at the plan level since the behavior under test was implemented in Task 1, not driven into existence by this test._

## Files Created/Modified
- `proto/engram/v1/engram.proto` - `SearchMemoriesRequest.categories = 8`, no `buf.validate` annotation, trailing comment matching sibling style
- `gen/go/engram/v1/engram.pb.go` - regenerated (new `Categories []string` field + getter)
- `gen/ts/engram/v1/engram_pb.ts` - regenerated
- `ui/src/lib/gen/engram/v1/engram_pb.ts` - re-vendored copy of the TS codegen
- `internal/server/connectapi.go` - `SearchMemories` handler's `coreSearchRequest` literal gains `Categories: req.Msg.Categories`
- `internal/server/connectdescriptor_test.go` - `SearchMemoriesRequest` pinned field count 7 → 8
- `internal/server/connectapi_test.go` - `TestMCPConnectCategoryFilterParity`, `TestConnectSearchUnknownCategory`

## Decisions Made
- **D-10 (human-resolved, pre-executor):** ship `categories = 8` now rather than defer Connect search-side parity to a later milestone. The user was told explicitly this is the phase's only one-way commitment (a published field number can never be reused/removed) and that D-10 as written in `26-CONTEXT.md` was auto-generated by `/gsd-discuss-phase --auto`, making this their first genuine review. They selected `ship-field-8`.
- **D-11 (confirmed, not re-litigated):** no `buf.validate` allowlist on the new field — `discovery` and `rule` are legitimate filter targets that `StoreMemoryRequest.category`'s write-domain allowlist would incorrectly exclude.

## Deviations from Plan

None - plan executed exactly as written. The plan's `<verify>` automated command for Task 1 referenced `-run TestConnectDescriptor`, which does not match any function name in this repo (the actual descriptor test is `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs`); this executor ran the correctly-named test instead of the plan's literal string and confirmed it passes with the field count now pinned at 8. This is a stale test-name reference in the plan text, not a code deviation, so no auto-fix rule applies — noted here for plan-authoring accuracy only.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- SC2's "MCP<->Connect category-filter parity" is now true in both directions: MCP `search_memory`/`list_memory` (26-02) and Connect `SearchMemories`/`ListMemories` (26-01, 26-03) all filter by category with identical OR semantics and no write-domain allowlist leakage.
- Field 8 is now spent on `SearchMemoriesRequest.categories` — future additions to this message start at field 9.
- No blockers for remaining Phase 26 plans (26-05, 26-06 not yet executed per STATE.md plan count).

---
*Phase: 26-structured-citations-category-filter-chat-base-url*
*Completed: 2026-07-25*

## Self-Check: PASSED
