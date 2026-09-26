---
phase: 02-recall-first-search
plan: 01
subsystem: api
tags: [recall-gate, connect, mcp, proto, qdrant]

# Dependency graph
requires: []
provides:
  - "RecallGateHidden proto message + recall_gate_hidden fields on SearchMemoriesResponse (5) and ListMemoriesResponse (8)"
  - "internal/server/hiddencount.go: countRecallHidden, recallGateFlags, (*deps).listRecallHidden, (*deps).searchRecallHidden, toProto, withRecallHidden"
  - "memStore.Search (plain vector order, never SearchReranked) for the recall-gate hidden-count comparison"
  - "coreListResult.Hidden and coreSearchResult{Memories,Hidden} shared by Connect and MCP lanes"
affects: [02-02, 02-04, 02-08, 02-09]

# Actuals (#2632)
actuals:
  tokens: 42538
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Recall-gate hidden count via a SECOND, unmodified Store.List/Store.Search call under the caller's own Subject with all three Include flags forced true — computed once in the shared deps core, read by both Connect and MCP lanes"
    - "Degrade-not-abort for a supplementary derived value: a comparison-call failure logs once at ERROR and returns nil (absent field), never fails the RPC and never fabricates zeros"

key-files:
  created:
    - internal/server/hiddencount.go
    - internal/server/hiddencount_test.go
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/server/tools.go
    - internal/server/connectapi.go
    - internal/server/store_iface.go
    - internal/server/fakestore_test.go
    - internal/server/tools_test.go
    - internal/server/connectapi_test.go
    - internal/server/connectapi_crossspine_test.go
    - internal/server/connectdescriptor_test.go

key-decisions:
  - "Task 1 checkpoint:decision resolved before dispatch: option-a — one message RecallGateHidden{total,archived,superseded,expired,scheduled}, fields recall_gate_hidden (SearchMemoriesResponse=5, ListMemoriesResponse=8), MCP key recall_gate_hidden (decided by Sean 2026-09-26)"
  - "Fixed pre-existing internal/server/connectdescriptor_test.go field-count pins for ListMemoriesResponse (7->8) and SearchMemoriesResponse (4->5), stale after this plan's additive proto fields (Rule 1 — directly caused by this plan, not out of scope)"

patterns-established:
  - "A derived read-only cross-cutting value (hidden count, coverage) is computed ONCE in the shared deps.* core and shared by both transports, never recomputed independently per lane"

requirements-completed: [ENTRY-03]

coverage:
  - id: D1
    description: "RecallGateHidden proto message published additively on both SearchMemoriesResponse (field 5) and ListMemoriesResponse (field 8); buf breaking stays green"
    requirement: "ENTRY-03"
    verification:
      - kind: integration
        ref: "internal/server/hiddencount_test.go#TestRecallHiddenListParity"
        status: pass
      - kind: other
        ref: "go tool buf breaking --against '.git#branch=main'"
        status: pass
    human_judgment: false
  - id: D2
    description: "List-lane hidden count: a second Store.List call at the same page window with the recall gate lifted, bucketed per state, identical on Connect ListMemories and MCP list_memory, including cross_spine and include_archived reclassification"
    verification:
      - kind: integration
        ref: "internal/server/hiddencount_test.go#TestRecallHiddenListParity"
        status: pass
    human_judgment: false
  - id: D3
    description: "Search-lane hidden count: a second plain Store.Search call (never SearchReranked) at the same k with the recall gate lifted, bucketed per state, identical on Connect SearchMemories and MCP search_memory, including cross_spine and include_superseded reclassification"
    verification:
      - kind: integration
        ref: "internal/server/hiddencount_test.go#TestRecallHiddenSearchParity"
        status: pass
    human_judgment: false
  - id: D4
    description: "A comparison-call failure degrades gracefully: the caller's own hits survive, recall_gate_hidden is absent (nil field / missing MCP key, never fabricated zeros), and the cause is logged exactly once server-side at ERROR and never reaches the wire"
    verification:
      - kind: integration
        ref: "internal/server/hiddencount_test.go#TestRecallHiddenDegradesOnComparisonFailure"
        status: pass
    human_judgment: false
  - id: D5
    description: "When the caller's own request already includes every recall-gated state, no comparison call is issued at all (cost control) and the count is present with every field zero"
    verification:
      - kind: integration
        ref: "internal/server/hiddencount_test.go#TestRecallHiddenSkipsComparisonWhenAllIncluded"
        status: pass
    human_judgment: false
  - id: D6
    description: "Pure per-state bucketing and precedence rules (expired suppresses scheduled; NotAfter exclusive, NotBefore inclusive-active; a record with several states counts once in each; a request's own Include flags gate which states are counted)"
    verification:
      - kind: unit
        ref: "internal/server/hiddencount_test.go#TestCountRecallHidden"
        status: pass
    human_judgment: false

duration: 38min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 1: Recall-Gate Hidden Count Summary

**SearchMemories/search_memory and ListMemories/list_memory now report a per-state (archived/superseded/expired/scheduled) count of records the recall gate hid, computed once via a second Store.Search/Store.List call and shared identically by Connect and MCP.**

## Performance

- **Duration:** 38 min
- **Started:** 2026-09-26T17:32:00Z
- **Completed:** 2026-09-26T18:10:28Z
- **Tasks:** 3 (1 checkpoint:decision resolved pre-dispatch, 2 executed)
- **Files modified:** 12 (2 created, 10 modified)

## Accomplishments

- Published `RecallGateHidden` proto message + `recall_gate_hidden` fields on `SearchMemoriesResponse` (5) and `ListMemoriesResponse` (8), regenerated `gen/go`, `gen/ts`, and `ui/src/lib/gen` — `buf breaking` stays green (D-01)
- List lane: `(*deps).listRecallHidden` runs a second `Store.List` call with the caller's own Subject and all three Include flags forced true, bucketed per state, wired through `coreListResult.Hidden` into both Connect `ListMemories` and MCP `list_memory` (D-02, D-03)
- Search lane: added `memStore.Search` (plain vector order), `(*deps).searchRecallHidden` runs the comparison via `Store.Search` — never `Store.SearchReranked` — at the same k, wired through `coreSearchResult.Hidden` into both Connect `SearchMemories` and MCP `search_memory` (D-02, D-03)
- Degrade-not-abort: a comparison failure logs once at ERROR server-side and yields an absent field/key, never a fabricated zero, never failing the caller's own request
- Skip-when-all-included: zero extra store calls when the caller's own request already relaxes every recall-gate state
- MCP input schemas unchanged — no `include_archived`/`include_superseded`/`include_scheduled` argument added to `searchArgs`/`listArgs` (D-03, decision `fenpnam8ah` preserved)

## Task Commits

Each task was committed atomically:

1. **Task 2 (tracer): List lane end to end** — `15649a96` (feat)
2. **Task 3: Search lane, memStore.Search, degrade and skip** — `c917df49` (feat)

_Task 1 was a `checkpoint:decision` resolved by the user before dispatch (see Decisions Made) and required no commit of its own._

_Note: Task 3 carried `tdd="true"`; per the plan's own action ordering, the interface/production changes (steps 1-5) were written first, then the three behavior tests (step 6) were written into `hiddencount_test.go` and run — all five tests (including the two from Task 2) passed on the first run, so no production-code fixes were needed._

## Files Created/Modified

- `internal/server/hiddencount.go` - `recallHidden`/`recallGateFlags`/`countRecallHidden`, `(*deps).listRecallHidden`, `(*deps).searchRecallHidden`, `toProto`, `withRecallHidden`
- `internal/server/hiddencount_test.go` - `TestCountRecallHidden` (pure, table-driven) plus four Qdrant/spy-backed parity, degrade, and skip tests
- `proto/engram/v1/engram.proto` - `RecallGateHidden` message; `recall_gate_hidden` fields on both responses
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts` - regenerated
- `internal/server/tools.go` - `coreListResult.Hidden`, `coreSearchResult`, `deps.listMemory`/`deps.searchMemory` wiring, both MCP closures
- `internal/server/connectapi.go` - `ListMemories`/`SearchMemories` populate `RecallGateHidden` unconditionally
- `internal/server/store_iface.go` - `memStore.Search`
- `internal/server/fakestore_test.go` - `spyStore.Search`
- `internal/server/tools_test.go` - `hitsOf` helper; existing `d.searchMemory` call sites rewrapped
- `internal/server/connectapi_test.go`, `internal/server/connectapi_crossspine_test.go` - existing `searchMemory` call sites rewrapped
- `internal/server/connectdescriptor_test.go` - field-count pins bumped for the two additive response fields

## Decisions Made

- Task 1 checkpoint:decision resolved before dispatch: **option-a** — one message `RecallGateHidden { uint64 total = 1; uint64 archived = 2; uint64 superseded = 3; uint64 expired = 4; uint64 scheduled = 5; }`, field `recall_gate_hidden` (`SearchMemoriesResponse` field 5, `ListMemoriesResponse` field 8), MCP key `recall_gate_hidden`. `total` counts distinct hidden records; per-state fields count records carrying that state (may sum to more than `total`); the field is ABSENT when the comparison failed, PRESENT with zeros when nothing was hidden. Decided by Sean 2026-09-26.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed stale field-count pins in `connectdescriptor_test.go`**
- **Found during:** Task 3's plan-level verify (`go test ./internal/server/ -count=1`)
- **Issue:** `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` hardcodes exact field counts for `ListMemoriesResponse` (was 7) and `SearchMemoriesResponse` (was 4); Task 2's additive `recall_gate_hidden` field bumped both to 8 and 5 respectively, failing the pinned assertion.
- **Fix:** Bumped both `assertFields` calls to the new counts and added the `recall_gate_hidden` field pin (kind `MessageKind`, `msgType: "engram.v1.RecallGateHidden"`) to each, following the file's own convention of a dated comment documenting each additive bump.
- **Files modified:** internal/server/connectdescriptor_test.go
- **Verification:** `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -count=1` passes (was previously the only failure)
- **Committed in:** c917df49 (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — pre-existing pinned test made stale by this plan's own additive change)
**Impact on plan:** Necessary to keep the plan-level verification green; no scope creep — the fix only updates field counts this plan's own proto edit changed.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The recall-gate hidden count is live on Connect and MCP for both search and list; plan 02-02 (CLI footer, CLAUDE.md, docs-site) can build on `resp.Msg.GetRecallGateHidden()` / MCP's `recall_gate_hidden` key directly.
- The console's results header (`N hidden by recall gate`, plans 02-04/02-08/02-09) has a real field to read.
- No blockers.

---
*Phase: 02-recall-first-search*
*Completed: 2026-09-26*
