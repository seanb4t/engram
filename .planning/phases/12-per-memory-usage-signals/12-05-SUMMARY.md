---
phase: 12-per-memory-usage-signals
plan: 05
subsystem: api
tags: [go, concurrency, worker-pool, otel, telemetry]

# Dependency graph
requires:
  - phase: 12-per-memory-usage-signals
    provides: "12-02's telemetry.UsageQueueMetrics (Enqueued/Dropped/Failed counters) this queue reports to"
provides:
  - "usageQueue type + newUsageQueue constructor: bounded, non-blocking, nil-safe async worker pool"
  - "tryEnqueue/Start/worker/process/itemDone/Shutdown/Wait/depth methods reusing the Phase 11 CR-01 shutdown-safety kernel"
affects: [12-per-memory-usage-signals plan 06 (get_memory/Connect GetMemory wiring + serve.go lifecycle)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D-10 single-attempt async worker: same CR-01 RWMutex closed-guard + inFlight reserve-before-send kernel as summaryQueue, with all backoff/retry machinery stripped — a lost bump is acceptable, never worth stalling a worker over."

key-files:
  created:
    - internal/server/usagequeue.go
    - internal/server/usagequeue_test.go
  modified: []

key-decisions:
  - "Field/method names mirror summaryQueue exactly (fill, itemDone, depth) for maximum reviewer pattern-recognition, per PATTERNS.md's 'exact (minus retry/backoff)' analog assignment."
  - "No attemptTimeout/maxElapsed parameter on newUsageQueue — there is no retry budget to derive since D-10 mandates a single fill attempt per id."

patterns-established:
  - "Any future async get-path or write-path worker pool should copy this file (or summaryqueue.go) rather than hand-rolling shutdown safety — the CR-01 send-on-closed-channel panic was already found and fixed once."

requirements-completed: [REQ-usage-signals]

coverage:
  - id: D1
    description: "usageQueue is bounded, non-blocking, and never panics on send-to-closed-channel during shutdown (CR-01 kernel: RWMutex closed guard + inFlight reserve-before-send)"
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "internal/server/usagequeue_test.go#TestUsageQueueNeverBlocksWrite"
        status: pass
      - kind: unit
        ref: "internal/server/usagequeue_test.go#TestUsageQueueDropsWhenFull"
        status: pass
      - kind: unit
        ref: "internal/server/usagequeue_test.go#TestUsageQueueEnqueueAfterShutdownIsDroppedNotPanic"
        status: pass
    human_judgment: false
  - id: D2
    description: "process() calls fill exactly once per id with no retry loop; failures are logged and counted (Failed), not retried"
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "internal/server/usagequeue_test.go#TestUsageQueueSingleAttemptNoRetry"
        status: pass
    human_judgment: false
  - id: D3
    description: "Shutdown drains in-flight work bounded by ctx, is idempotent, and a panicking fill never wedges Wait() (deterministic drain seam, no time.Sleep in tests)"
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "internal/server/usagequeue_test.go#TestUsageQueueShutdownDrainsWithinBudget"
        status: pass
      - kind: unit
        ref: "internal/server/usagequeue_test.go#TestUsageQueuePanicDoesNotWedgeWait"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-07-10
status: complete
---

# Phase 12 Plan 05: Async Usage-Signal Incrementer Engine Summary

**Bounded, non-blocking usageQueue worker pool (CR-01 shutdown-safety kernel, D-10 single-attempt no-retry) with Qdrant-free deterministic tests, ready for Plan 06 wiring into get_memory**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-07-10T13:20:00Z (approx)
- **Completed:** 2026-07-10T13:32:00Z
- **Tasks:** 2
- **Files modified:** 2 (both new)

## Accomplishments
- Built `internal/server/usagequeue.go`: `usageQueue` type reusing the Phase 11 `summaryQueue`'s CR-01 shutdown-safety kernel verbatim in shape — `mu sync.RWMutex`/`closed bool` guard, `inFlight sync.WaitGroup` reserve-before-send, `tryEnqueue`/`Start`/`worker`/`Shutdown`/`Wait`/`depth`.
- `process(ctx, id)` is a single-attempt rewrite (D-10): no `backoff.Retry`, no `maxElapsed`, no `Retried` counter — one call to the injected `fill func(ctx, id) error`, with a `recover()` guard so a panicking fill counts as `Failed` and never wedges the pool.
- `internal/server/usagequeue_test.go`: 9 Qdrant-free tests mirroring `summaryqueue_test.go`'s deterministic (`Wait()`-based, no `time.Sleep`) patterns — never-blocks-under-hung-fill, drop-on-full, single-attempt-no-retry, panic-does-not-wedge-Wait, bounded-and-idempotent Shutdown, post-Shutdown drop-not-panic, depth-reflects-occupancy, fill-success happy path, and nil-receiver no-op.
- `go build`, `go vet`, and `golangci-lint run ./internal/server/...` all clean; full `-race` test run green.

## Task Commits

Each task was committed atomically:

1. **Task 1: usagequeue.go — CR-01 kernel minus retry** - `3dea452` (feat)
2. **Task 2: usagequeue_test.go — Qdrant-free shutdown-safety + drop-on-full** - `8f2771e` (test)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/server/usagequeue.go` - `usageQueue`/`newUsageQueue` + `tryEnqueue`/`Start`/`worker`/`process`/`itemDone`/`Shutdown`/`Wait`/`depth`; no `cenkalti/backoff` import
- `internal/server/usagequeue_test.go` - 9 deterministic tests reusing `counterSum` from `summaryqueue_test.go` (same package)

## Decisions Made
- No `attemptTimeout`/`maxElapsed` constructor parameter — D-10 rules out a retry budget entirely, so there is nothing to derive it from (unlike `newSummaryQueue`).
- Kept struct/method naming identical to `summaryQueue` (`fill`, `itemDone`, `depth`) rather than renaming to `increment`, matching PATTERNS.md's literal analog guidance and minimizing reviewer cognitive load against the shipped precedent.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. This plan builds the engine only; no lifecycle wiring into `serve.go`/`tools.go` (that is Plan 06).

## Next Phase Readiness

- `usageQueue` is ready for Plan 06 to wire: `newUsageQueue(workers, queueSize, metrics, func(ctx, id) error { return st.IncrementAccess(ctx, id) })`, `Start`/`Shutdown` lifecycle mirroring `summaryQueue`'s `serve.go` call sites, and `tryEnqueue(pid)` call-and-ignore at `getMemory` (tools.go) and Connect `GetMemory` (connectapi.go).
- No new external packages added; the plan deliberately avoided the `cenkalti/backoff` import used by `summaryQueue`.

---
*Phase: 12-per-memory-usage-signals*
*Completed: 2026-07-10*

## Self-Check: PASSED
- FOUND: internal/server/usagequeue.go
- FOUND: internal/server/usagequeue_test.go
- FOUND: .planning/phases/12-per-memory-usage-signals/12-05-SUMMARY.md
- FOUND commit: 3dea452
- FOUND commit: 8f2771e
