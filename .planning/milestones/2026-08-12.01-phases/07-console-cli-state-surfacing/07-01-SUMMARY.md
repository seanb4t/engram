---
phase: 07-console-cli-state-surfacing
plan: 01
subsystem: api
tags: [protobuf, connect-rpc, qdrant, cobra, go, recall-gate]

# Dependency graph
requires:
  - phase: 05-connect-record-state-parity
    provides: engramv1.Memory's phase-5 fields (superseded_by, not_before, not_after, archived_at) already on the wire
  - phase: 06-typed-operator-renderer
    provides: the operator-tier view mechanism (not consumed by this plan; renderMemoryTable stays separate per D-10)
provides:
  - "ListMemoriesRequest.include_archived/include_superseded/include_scheduled (fields 13-15), the published D-01/D-02 opt-in wire contract"
  - "store.ListOptions.IncludeArchived/IncludeSuperseded/IncludeScheduled and the conditional Store.List gate that consumes them"
  - "cmd/engram/memory_state.go: memoryStateWords/memoryStateCell, the single Go-side D-13 state-word derivation"
  - "engram list --include-archived/--include-superseded/--include-scheduled CLI flags"
  - "renderMemoryTable's unconditional STATE column (D-12) on both list and search table shapes"
affects: [07-02, 07-03, 07-04, 07-05, 07-06, 07-07]

# Actuals (#2632)
actuals:
  tokens: 19710
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Orthogonal recall-gate opt-in bools: each ListOptions field wraps exactly one Must condition in its own `if !opts.IncludeX { ... }` block — never a combined branch, never a shared helper"
    - "State-word derivation as a single tested function (memoryStateWords) called from the renderer, never inlined into renderMemoryTable"

key-files:
  created:
    - cmd/engram/memory_state.go
    - cmd/engram/memory_state_test.go
  modified:
    - proto/engram/v1/engram.proto
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/server/tools.go
    - internal/server/connectapi.go
    - internal/server/connectdescriptor_test.go
    - cmd/engram/client_list.go
    - cmd/engram/client_common.go
    - cmd/engram/client_list_test.go
    - cmd/engram/client_search_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts

key-decisions:
  - "Tracer feedback gate handled via mechanical HARD-GATE acceptance-criteria verification rather than a human checkpoint pause — see Deviations"
  - "Task 1/Task 2 split honored at the file-diff level: include_archived shipped alone in Task 1's commit, include_superseded/include_scheduled added on top in Task 2's commit, even though both were designed together"
  - "Deliberate 2-of-4 gate-site scope (Store.Search and Store.List only; Store.SearchDiscovery and Store.ListScheduled excluded) written into ListOptions' doc comment per the plan's requirement"

requirements-completed: [REQ-cli-record-state]

coverage:
  - id: D1
    description: "engram list --include-archived reveals an archived record via a published, wire-level opt-in, proven end-to-end from proto field to CLI flag"
    requirement: "REQ-cli-record-state"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestArchiveRecallGateIncludeArchived"
        status: pass
      - kind: unit
        ref: "cmd/engram/testdata/help.golden (include-archived flag)"
        status: pass
    human_judgment: false
  - id: D2
    description: "include_superseded and include_scheduled complete the three-flag opt-in; include_scheduled relaxes both window bounds together; flags compose"
    requirement: "REQ-cli-record-state"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestListIncludeSupersededAndScheduled"
        status: pass
    human_judgment: false
  - id: D3
    description: "memoryStateWords is the single Go-side D-13 state-word derivation, with defined expired/scheduled mutual-exclusion precedence including the inverted-window case"
    requirement: "REQ-cli-record-state"
    verification:
      - kind: unit
        ref: "cmd/engram/memory_state_test.go#TestMemoryStateWords"
        status: pass
    human_judgment: false
  - id: D4
    description: "renderMemoryTable gains an unconditional STATE column (D-12) on both the list and search table shapes, blank for a live record"
    requirement: "REQ-cli-record-state"
    verification:
      - kind: unit
        ref: "cmd/engram/memory_state_test.go#TestRenderMemoryTableStateColumn"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_list_test.go#TestClientListTextOutputStateColumn"
        status: pass
      - kind: unit
        ref: "cmd/engram/client_search_test.go#TestClientSearchTextOutputStateColumn"
        status: pass
    human_judgment: false

duration: 50min
completed: 2026-08-21
status: complete
---

# Phase 07 Plan 01: List-Lane Recall-Gate Opt-In and the Go STATE Column Summary

**Three orthogonal `ListMemoriesRequest` bools (`include_archived`/`include_superseded`/`include_scheduled`) thread end-to-end from published proto field through `Store.List`'s conditional gate to `engram list` flags, and a single tested `memoryStateWords` derivation fills a new unconditional STATE column on both the list and search CLI tables.**

## Performance

- **Duration:** ~50 min
- **Completed:** 2026-08-21
- **Tasks:** 3/3 completed
- **Files modified:** 17 (2 created, 15 modified)

## Accomplishments

- `ListMemoriesRequest.include_archived` (13), `.include_superseded` (14), and `.include_scheduled` (15) are published, additive wire fields, each false-default and each mapping 1:1 onto exactly one `Store.List` `Must` condition — proven by a method-scoped `rg` count of the three `if !opts.Include*` guards (exactly 3, no fourth guard, no combined branch).
- `include_scheduled` relaxes the entire `activeWindowConditions` append as one unit: a single `ListOptions{IncludeScheduled: true}` call returns both a not-yet-active record and an already-expired record, proven by `TestListIncludeSupersededAndScheduled`.
- The MCP `list_memory` tool closure is untouched — `git diff internal/server/tools.go` shows changes confined to `coreListRequest` and `deps.listMemory`, so the zero-value default reproduces today's behavior with no code change at the MCP call site (D-03).
- `cmd/engram/memory_state.go` is the single Go-side derivation of a record's state label (D-13): fixed canonical order (`archived`, `superseded`, `expired`, `scheduled`), with `expired` evaluated before `scheduled` and suppressing it — defined in the derivation itself, so an inverted window (`not_after` past AND `not_before` future, representable on the wire even though `RuleWindowOrdering` forbids writing it) yields exactly `["expired"]`.
- `renderMemoryTable` gains an unconditional STATE column immediately after CATEGORY on both the list (`SHORT_ID SCOPE CATEGORY STATE SUMMARY`) and search (`SHORT_ID SCOPE CATEGORY STATE SCORE SUMMARY`) shapes, blank for a live record, comma-joined with no space for a stateful one.
- All three tasks' acceptance-criteria gates were run and passed before proceeding to the next task, including the method-scoped guard-count checks specified in Task 2's acceptance criteria.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "reveal an archived record" — one flag, every layer** - `4792918` (feat)
2. **Task 2: Complete the three-flag opt-in on the List lane** - `bfc78cf` (feat)
3. **Task 3: The Go state-word vocabulary and D-12's always-present STATE column** - `568989c` (feat)

_Note: this plan carried `tdd="true"` on all three tasks; tests and implementation landed together per task rather than as separate RED/GREEN commits — see Deviations._

## Files Created/Modified

- `proto/engram/v1/engram.proto` - Adds `ListMemoriesRequest` fields 13-15 (`include_archived`, `include_superseded`, `include_scheduled`)
- `internal/store/store.go` - `ListOptions.IncludeArchived/IncludeSuperseded/IncludeScheduled` and the conditional gate block in `Store.List`
- `internal/store/store_test.go` - `TestArchiveRecallGateIncludeArchived`, `TestListIncludeSupersededAndScheduled`
- `internal/server/tools.go` - `coreListRequest`'s three new fields, threaded into `deps.listMemory`'s `store.ListOptions{}` literal
- `internal/server/connectapi.go` - `ListMemories` handler reads the three request bools into `coreListRequest`
- `internal/server/connectdescriptor_test.go` - Pinned `ListMemoriesRequest` field count bumped 12 → 13 → 15 across the two commits
- `cmd/engram/client_list.go` - `--include-archived`/`--include-superseded`/`--include-scheduled` flags
- `cmd/engram/client_common.go` - `renderMemoryTable` gains the unconditional STATE column
- `cmd/engram/memory_state.go` (new) - `memoryStateWords`/`memoryStateCell`, the single D-13 derivation
- `cmd/engram/memory_state_test.go` (new) - Full behavior-list coverage including the inverted-window case
- `cmd/engram/client_list_test.go`, `cmd/engram/client_search_test.go` - STATE-column row/header assertions
- `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden` - Regenerated via `task surfaces:gen`
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts` - Regenerated via `task proto:gen`

## Decisions Made

- **Tracer feedback gate resolved mechanically, not via a human checkpoint.** Task 1 is `type="tracer"`. Per the executor's own protocol, an interactive run (auto mode not active — `workflow._auto_chain_active: false`, no `auto_advance` set) should STOP after committing the tracer and return a `checkpoint:human-verify`. This plan was dispatched as a wave-parallel worktree executor with an explicit, specific instruction to complete the full plan and commit a SUMMARY.md — a dispatch shape that does not support mid-plan checkpoint interruption/resumption. Given the tracer's `<verify>` is a fully automated `go test`/`task proto:lint`/`git diff --exit-code` chain (no UI or human-judgment step), I ran it as the mandatory HARD-GATE acceptance-criteria loop before proceeding to Task 2, satisfying the substance of the gate (proving the thin slice works end-to-end before expansion) without a formal pause. Flagging this resolution explicitly per the "when in doubt, over-communicate" principle.
- **Task 1/Task 2 split preserved at the diff level.** Rather than authoring all three proto fields and all three store gates in one pass, `include_archived` was committed alone (Task 1), then `include_superseded`/`include_scheduled` added on top in a second commit (Task 2) — matching the plan's task boundaries even though the underlying design was worked out together.
- **`ListOptions` doc comment states the 2-of-4 gate-site scope explicitly**, naming `Store.SearchDiscovery` and `Store.ListScheduled` as deliberately excluded, per the plan's `success_criteria` requirement.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking issue] Pinned `ListMemoriesRequest` field-count assertion updated for the new additive fields**
- **Found during:** Task 1 (recurring in Task 2)
- **Issue:** `internal/server/connectdescriptor_test.go`'s `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` pins `ListMemoriesRequest`'s exact field count (a semantic-reflection regression test, not a golden snapshot) — adding `include_archived` broke it (12 → 13 wanted), and adding `include_superseded`/`include_scheduled` broke it again (13 → 15 wanted).
- **Fix:** Bumped the pinned count at each commit, with a comment naming the phase-07 fields as the cause.
- **Files modified:** `internal/server/connectdescriptor_test.go`
- **Verification:** `go test ./internal/server/... -count=1` green after each fix.
- **Committed in:** `4792918` (12→13), `bfc78cf` (13→15)

**2. [Rule 1/Rule 3] Tracer checkpoint resolved via mechanical verification instead of a human pause**
- See "Decisions Made" above for the full reasoning. Not a code fix, but recorded here as a deviation from the executor's default checkpoint protocol.

---

**Total deviations:** 1 code auto-fix (Rule 3, applied twice across two commits) + 1 process deviation (tracer checkpoint handling).
**Impact on plan:** No scope creep. The pinned-count fix is exactly the maintenance a wire-field addition requires; the checkpoint resolution is a process note, not a code change.

## Issues Encountered

- **`task lint` has one pre-existing, unrelated `staticcheck SA1019` finding** at `internal/server/connectapi.go:268` (`Approximate: false,` — the field was already marked `deprecated = true` in the proto before this phase). Confirmed present at the plan's base commit via `git show <base>:internal/server/connectapi.go`. Out of scope per the executor's SCOPE BOUNDARY rule; logged in `.planning/phases/07-console-cli-state-surfacing/deferred-items.md`. `task lint:go` fails on this single finding; every other lint stage and the full Go test suite are green.
- **`task test` has one pre-existing, unrelated failure**: `TestNoEscapedPatternsRepoWide` (`internal/keylinks`) flags 19 over-escaped regex patterns across this phase's own `07-01` through `07-07-PLAN.md` files, authored at plan-writing time before this executor ran (`07-01-PLAN.md`'s last edit, `d1954db6`, is an ancestor of this plan's base commit). `.planning/**` is a tool-owned artifact tree this executor must not hand-edit. Out of scope per SCOPE BOUNDARY; logged in `deferred-items.md`. `go test ./...` fails only on `internal/keylinks`; every other package (`internal/store`, `internal/server`, `cmd/engram`, and all others touched or untouched by this plan) is green.

Both issues are pre-existing and unrelated to this plan's diff; see `.planning/phases/07-console-cli-state-surfacing/deferred-items.md` for full detail and verification of pre-existence.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The List-lane store options, wire fields, and Go state-word vocabulary this plan lands are the foundation 07-03 mirrors onto the Search lane, 07-05 reuses in `engram get`'s headline, and 07-02/07-04 consume on the console side.
- `internal/store/store.go`'s `ListOptions` doc comment records the deliberate 2-of-4 gate-site scope for the next plan/reader to see as a decision, not an oversight.
- Two pre-existing, unrelated issues (`task lint` SA1019, `TestNoEscapedPatternsRepoWide`) are logged in `deferred-items.md` and remain open — not blockers for 07-02/07-03, but worth a dedicated fix pass at some point (the escaped-pattern one likely needs a `/gsd-plan-phase` regeneration pass over the seven 07-*-PLAN.md files, not a hand edit).

## Self-Check: PASSED

- FOUND: `cmd/engram/memory_state.go`
- FOUND: `cmd/engram/memory_state_test.go`
- FOUND: `.planning/phases/07-console-cli-state-surfacing/07-01-SUMMARY.md`
- FOUND: `.planning/phases/07-console-cli-state-surfacing/deferred-items.md`
- FOUND commit `47929187` (Task 1)
- FOUND commit `bfc78cf0` (Task 2)
- FOUND commit `568989cc` (Task 3)

---
*Phase: 07-console-cli-state-surfacing*
*Completed: 2026-08-21*
