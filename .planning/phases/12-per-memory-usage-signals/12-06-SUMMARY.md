---
phase: 12-per-memory-usage-signals
plan: 06
subsystem: api
tags: [go, mcp, connect-rpc, qdrant, usage-signals, curation]

# Dependency graph
requires:
  - phase: 12-per-memory-usage-signals (plan 01)
    provides: "store.Memory.AccessCount/LastAccessedAt payload fields, store.Update free bump, store.IncrementAccess"
  - phase: 12-per-memory-usage-signals (plan 02)
    provides: "config.UsageConfig.Signals / ENGRAM_USAGE_SIGNALS registry+validate wiring"
  - phase: 12-per-memory-usage-signals (plan 03)
    provides: "engramv1.Memory proto fields 19/20 (access_count, last_accessed_at) + gen/ regen"
  - phase: 12-per-memory-usage-signals (plan 05)
    provides: "usageQueue async worker-pool engine (newUsageQueue/tryEnqueue/Start/Shutdown/Wait/depth)"
provides:
  - "deps.usageQueue field + buildUsageQueue(cfg, st, uqm) config-gated construction (ENGRAM_USAGE_SIGNALS, default true)"
  - "get_memory (MCP) and Connect GetMemory tryEnqueue(pid) call sites, success-only, call-and-ignore (D-01)"
  - "recallView.AccessCount/LastAccessedAt + toRecallView population (D-07 compact-view exposure)"
  - "memoryToProto AccessCount/LastAccessedAt mapping onto proto fields 19/20"
  - "serve.go UsageQueueMetrics construction + composed summary+usage shutdown drain"
  - "D-02 negative-space test proving search/list/list_scheduled never enqueue"
affects: [phase-12-followups, curation-tooling, clickstack-dashboards]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Config-gated async worker construction: buildUsageQueue mirrors buildSummaryQueue's point-of-use strconv.ParseBool gate, returning nil (nil-safe no-op) when disabled."
    - "Call-and-ignore async enqueue at the handler boundary, success-branch-only (never inside reused store primitives)."
    - "Hand-written view allow-lists (recallView, memoryToProto) require explicit field mirroring — adding a store.Memory field alone never surfaces on compact/wire shapes."
    - "Composed shutdown closure: Register returns a single func draining multiple independent async queues, each nil-safe."

key-files:
  created: []
  modified:
    - internal/server/tools.go
    - internal/server/connectapi.go
    - internal/server/summary.go
    - cmd/engram/serve.go
    - internal/server/tools_test.go
    - internal/server/summary_test.go
    - .planning/phases/12-per-memory-usage-signals/deferred-items.md

key-decisions:
  - "Fixed worker pool (workers=2, queueSize=256) for buildUsageQueue — no new env-configurable knobs this phase, per plan's planner discretion."
  - "No usage-queue depth OTel gauge added — plan marked it optional/discretion and D-09 doesn't require it."
  - "serve.go's drainSummaries local renamed to drain since it now composes both queues' shutdown."

patterns-established:
  - "usageQueueRecorder + testDepsWithUsageQueue test seam (tools_test.go) mirrors testDepsWithSummaryQueue for injecting a call-recording fill into a live queue."

requirements-completed: [REQ-usage-signals]

coverage:
  - id: D1
    description: "get_memory (MCP) and Connect GetMemory enqueue exactly the fetched id on success only, never on a denied/ErrNotFound get (D-01)"
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestGetMemoryEnqueuesUsageSignalOnSuccessOnly"
        status: pass
    human_judgment: false
  - id: D2
    description: "search_memory/list_memory/list_scheduled never enqueue a usage-signal counter write, even when the result set includes an already-counted record (D-02 hard invariant)"
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestSearchListMemoryDoNotEnqueueUsageSignal"
        status: pass
    human_judgment: false
  - id: D3
    description: "update_memory raises access_count by exactly 1 via the free store.Update bump; no separate async enqueue (D-04)"
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestUpdateMemoryIncrementsAccessCountOnceNoAsyncEnqueue"
        status: pass
    human_judgment: false
  - id: D4
    description: "recallView/toRecallView (compact list/search shape) and memoryToProto (Connect wire) surface access_count/last_accessed_at read-only (D-07)"
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "internal/server/summary_test.go#TestToRecallViewSurfacesUsageSignals"
        status: pass
      - kind: unit
        ref: "internal/server/connectapi_test.go#TestMemoryToProto"
        status: pass
    human_judgment: false
  - id: D5
    description: "ENGRAM_USAGE_SIGNALS gates the payload write (buildUsageQueue nil on false/unparseable); a nil/disabled queue still returns the record via getMemory with zero counter writes (D-09)"
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestBuildUsageQueueConfigGate"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-07-10
status: complete
---

# Phase 12 Plan 06: Integration Capstone Summary

**Wired the config-gated async usageQueue into get_memory/Connect GetMemory (call-and-ignore, success-only), exposed access_count/last_accessed_at read-only on the compact recall view and Connect wire, composed the queue's shutdown drain into serve.go, and proved the D-02 negative-space invariant end-to-end.**

## Performance

- **Duration:** ~12 min (13:32–13:43 UTC-4, from Plan 05's last commit to this plan's last test commit)
- **Tasks:** 3/3 completed
- **Files modified:** 7 (tools.go, connectapi.go, summary.go, serve.go, tools_test.go, summary_test.go, deferred-items.md)

## Accomplishments

- `deps.usageQueue` + `buildUsageQueue(cfg, st, uqm)` construct and start a config-gated (ENGRAM_USAGE_SIGNALS, default true) fixed worker pool (workers=2, queueSize=256) injecting `st.IncrementAccess` as the fill; false/unparseable returns nil (nil-safe no-op).
- `getMemory` (MCP) and Connect `GetMemory` fire `tryEnqueue(pid)` call-and-ignore, strictly after a successful `GetReadable` — a denied/`ErrNotFound` get never increments (D-01).
- No `tryEnqueue` call exists in `searchMemory`/`listMemory`/`listScheduled` or their Connect equivalents (D-02) — proven by a dedicated negative-space test.
- `recallView`/`toRecallView` (summary.go) and `memoryToProto` (connectapi.go) explicitly carry and populate `AccessCount`/`LastAccessedAt`, closing the hand-written allow-list gap the RESEARCH flagged.
- `serve.go` constructs `UsageQueueMetrics` alongside `SummaryQueueMetrics`; `Register`'s returned shutdown closure now drains both queues (nil-safe), invoked strictly after `httpSrv.Shutdown` returns.
- End-to-end tests pin D-01 (success-only enqueue), D-02 (zero enqueues from search/list/list_scheduled), D-04 (update bumps once, no async enqueue), D-07 (compact view exposure), and D-09 (config gate + nil-safety).

## Task Commits

Each task was committed atomically:

1. **Task 1: deps.usageQueue + buildUsageQueue gate + lifecycle** - `2a70a1c5` (feat)
2. **Task 2: D-01 counting call sites + D-07 read-only exposure** - `4a43b9d2` (feat)
3. **Task 3: End-to-end negative-space + gate tests** - `cecbca38` (test)

_All three tasks compiled/vetted/tested green individually; no separate plan-metadata commit needed beyond this SUMMARY's own commit._

## Files Created/Modified

- `internal/server/tools.go` - `deps.usageQueue` field, `buildUsageQueue`, `buildDepsFromEnv`/`Register` threading `uqm`, `getMemory` tryEnqueue on success, composed shutdown closure
- `internal/server/connectapi.go` - Connect `GetMemory` tryEnqueue on success, `memoryToProto` AccessCount/LastAccessedAt mapping
- `internal/server/summary.go` - `recallView` struct + `toRecallView` populate AccessCount/LastAccessedAt
- `cmd/engram/serve.go` - `UsageQueueMetrics` construction, threaded into `Register`, composed drain after `httpSrv.Shutdown`
- `internal/server/tools_test.go` - `usageQueueRecorder`/`testDepsWithUsageQueue` test seam; D-01/D-02/D-04/D-09 tests; updated `Register`/`buildDepsFromEnv` call sites for the new `uqm` param
- `internal/server/summary_test.go` - `TestToRecallViewSurfacesUsageSignals` (D-07 regression guard)
- `.planning/phases/12-per-memory-usage-signals/deferred-items.md` - logged pre-existing `internal/store/store.go` golangci-lint findings (from 12-04) confirmed out of this plan's scope

## Decisions Made

- Used a fixed worker pool (2 workers, 256 queue depth) for `buildUsageQueue` rather than adding new `ENGRAM_USAGE_*` worker/queue-size env vars — the plan explicitly left this to planner discretion and a lightweight best-effort counter bump doesn't need per-deployment tuning knobs.
- Skipped an OTel usage-queue depth gauge (mirroring `summaryQueue.depth`'s `RegisterSummaryQueueDepth`) — optional per the plan, and D-09's acceptance criteria don't require it; `usageQueue.depth()` remains available if a future phase wants it.
- Renamed `serve.go`'s `drainSummaries` local to `drain` since it now composes both queues' shutdown, per the plan's "if desired" suggestion.

## Deviations from Plan

None — plan executed exactly as written. All three tasks' acceptance criteria were met without requiring architectural changes, bug fixes, or missing-functionality additions beyond what the plan specified.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. `ENGRAM_USAGE_SIGNALS` defaults to `true`; operators wanting zero read-path counter writes can set it to `false`.

## Next Phase Readiness

- Phase 12's full arc (store primitive → config gate → async engine → recall-span analytics → this integration capstone) is complete: `get_memory`/`update_memory` count strong-signal touches, `search_memory`/`list_memory`/`list_scheduled` never do, the counter is exposed read-only everywhere it should be, and the payload write is config-gated and shutdown-safe.
- `task lint:go`/`go vet`/`go build`/`go test ./... -count=1` are all green for every file this plan touched (`internal/server/...`, `cmd/engram/...` golangci-lint: 0 issues).
- Two out-of-scope, pre-existing gaps remain logged in `deferred-items.md` for a future cleanup pass: (1) `task lint:markdown` fails on ~331 pre-existing `.planning/` rumdl issues (logged in 12-01, unrelated to any phase-12 code), and (2) two `revive` findings in `internal/store/store.go` (from Plan 12-04's `recallIDs`/`Search`, confirmed pre-existing via `git stash`, outside this plan's `files_modified`).
- No blockers for closing out Phase 12.

---
*Phase: 12-per-memory-usage-signals*
*Completed: 2026-07-10*
