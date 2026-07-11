---
phase: 11-async-on-write-summaries
plan: 02
subsystem: async-worker
tags: [go-concurrency, backoff-v5, otel-metrics, worker-pool]

requires:
  - phase: 11-01
    provides: "ENGRAM_SUMMARY_ON_WRITE/_WORKERS/_QUEUE_SIZE config knobs; backoff/v5 direct dep; telemetry.SummaryQueueMetrics static instruments + RegisterSummaryQueueDepth"
provides:
  - "internal/server/summaryqueue.go: bounded, nil-safe, non-blocking summaryQueue worker pool (tryEnqueue/Start/Shutdown/Wait/depth)"
  - "storeFill production fill-builder: Get -> FillSummary -> LogSummaryEgress, backoff.Permanent on not-found"
  - "internal/store.LogSummaryEgress: shared content-free egress audit helper, factored out of SummarizeMissing"
affects: [11-03-server-wiring]

tech-stack:
  added: []
  patterns:
    - "nil-safe *summaryQueue receiver methods (disabled = nil = no-op) instead of an interface + no-op implementation"
    - "second sync.WaitGroup (inFlight) as a deterministic drain seam for tests, decoupled from the worker-lifecycle WaitGroup"
    - "per-attempt context.WithTimeout wrapping each backoff.Retry operation call, independent of the retry library's own elapsed-time accounting"

key-files:
  created:
    - internal/server/summaryqueue.go
    - internal/server/summaryqueue_test.go
  modified:
    - internal/store/summarize.go

key-decisions:
  - "LogSummaryEgress is called for all three terminal fill outcomes in the async worker path (filled/skipped/failed) per the plan's explicit behavior spec, whereas SummarizeMissing (which pre-filters eligibility itself before calling FillSummary) only ever reaches filled/failed — both paths share the identical helper and message string so audit consumers see one consistent egress log regardless of which gateway produced it."
  - "WithMaxElapsedTime uses a fixed 20s package constant (matching the RESEARCH Pattern 2 citation), decoupled from the injected attemptTimeout, so tests with a tiny attemptTimeout aren't at risk of the elapsed-time ceiling firing before WithMaxTries does — WithMaxTries(3) is the primary bound, MaxElapsedTime is a safety backstop."
  - "storeFill and depth() are exercised by two new Qdrant-gated integration tests (skip-gated via the existing testDeps helper) plus a Qdrant-free depth test, rather than left dead until Wave 3 wires them into buildDepsFromEnv/deps — this was necessary to keep this plan's own `task lint` (unused-code check) green without disabling the linter or deferring lint enforcement to Wave 3."

requirements-completed: [REQ-async-summaries]

coverage:
  - id: D1
    description: "tryEnqueue never blocks the write path — non-blocking select/default drop-and-count on a full queue, nil-safe when the queue is disabled"
    requirement: "REQ-async-summaries"
    verification:
      - kind: unit
        ref: "internal/server/summaryqueue_test.go#TestSummaryQueueNeverBlocksWrite"
        status: pass
      - kind: unit
        ref: "internal/server/summaryqueue_test.go#TestSummaryQueueDropsWhenFull"
        status: pass
    human_judgment: false
  - id: D2
    description: "Worker drains ids through Store.FillSummary under bounded backoff/v5 retry (WithMaxTries/WithMaxElapsedTime/WithNotify), with a per-attempt context.WithTimeout cutting a hung fill instead of burning the full retry budget (Codex finding #2)"
    requirement: "REQ-async-summaries"
    verification:
      - kind: unit
        ref: "internal/server/summaryqueue_test.go#TestSummaryQueueRetryGivesUp"
        status: pass
      - kind: unit
        ref: "internal/server/summaryqueue_test.go#TestSummaryQueueHungFillIsInterrupted (-race)"
        status: pass
    human_judgment: false
  - id: D3
    description: "A panicking fill is recovered at the worker level, counts failed, keeps the pool alive, and never wedges the deterministic Wait() drain seam (Codex finding #4)"
    requirement: "REQ-async-summaries"
    verification:
      - kind: unit
        ref: "internal/server/summaryqueue_test.go#TestSummaryQueuePanicDoesNotWedgeWait (-race)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Shutdown drains best-effort bounded by the caller's context and never hangs past its deadline"
    requirement: "REQ-async-summaries"
    verification:
      - kind: unit
        ref: "internal/server/summaryqueue_test.go#TestSummaryQueueShutdownDrainsWithinBudget (-race)"
        status: pass
    human_judgment: false
  - id: D5
    description: "store.LogSummaryEgress shared helper: SummarizeMissing refactored to call it (behavior-preserving), and the async worker's storeFill calls the same helper for filled/skipped/failed outcomes (Codex finding #3, T-11-06)"
    requirement: "REQ-async-summaries"
    verification:
      - kind: unit
        ref: "internal/store/summarize_test.go#TestSummarizeMissingEmitsEgressAuditLog"
        status: pass
      - kind: integration
        ref: "internal/server/summaryqueue_test.go#TestStoreFillFillsEligibleRecord"
        status: pass
      - kind: integration
        ref: "internal/server/summaryqueue_test.go#TestStoreFillReturnsPermanentOnNotFound"
        status: pass
    human_judgment: false
  - id: D6
    description: "Happy-path fill and queue-depth sampler both behave correctly"
    requirement: "REQ-async-summaries"
    verification:
      - kind: unit
        ref: "internal/server/summaryqueue_test.go#TestSummaryQueueFillSuccess"
        status: pass
      - kind: unit
        ref: "internal/server/summaryqueue_test.go#TestSummaryQueueDepthReflectsChannelOccupancy"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-07-10
status: complete
---

# Phase 11 Plan 02: Summary Queue Worker-Pool Core Summary

**`internal/server/summaryqueue.go` — a bounded, nil-safe, non-blocking worker pool that drains record ids through the existing `Store.FillSummary` seam under `backoff/v5` bounded retry with a per-attempt timeout and panic-safe draining, plus a shared `store.LogSummaryEgress` audit helper reused by the summarize-missing sweep.**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-07-10T13:10:52Z
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments
- `summaryQueue` type: bounded `ch chan string`, nil-safe receiver methods (`tryEnqueue`/`Start`/`Shutdown`/`Wait`/`depth`) so a disabled queue (`ENGRAM_SUMMARY_ON_WRITE=false`) needs no call-site branching
- `tryEnqueue` uses a `select`/`default` non-blocking send — a full queue drops and counts, never blocks the write path (SC#2)
- `worker`/`process` drain ids through `backoff.Retry` (`WithMaxTries(3)`, `WithMaxElapsedTime(20s)`, a custom `ExponentialBackOff` with `MaxInterval=2s` instead of the library's 60s default, `WithNotify` incrementing `retried`), wrapping **each** attempt in its own `context.WithTimeout(workerCtx, attemptTimeout)` so a hung summarizer call is cut instead of burning the retry budget (Codex finding #2)
- Panic safety: `defer q.itemDone()` + a worker-level `recover()` turn a panicking fill into a `failed` increment with balanced in-flight accounting — the pool keeps draining and the deterministic `Wait()` seam never wedges (Codex finding #4)
- `storeFill(st, summarize, model, maxChars)` is the production fill-builder: `Get` → `FillSummary` → `store.LogSummaryEgress` (outcome `filled`/`skipped`/`failed`); a not-found `Get` error is wrapped via `backoff.Permanent` so it is never retried (RESEARCH Pitfall 4)
- `store.LogSummaryEgress` factored out of `SummarizeMissing`'s inline `auditAttrs` closure into an exported, shared helper — `SummarizeMissing` now calls it directly (behavior-preserving refactor, verified by the existing `TestSummarizeMissingEmitsEgressAuditLog`), and `storeFill` calls the identical helper so the sweep and the async worker cannot drift (Codex finding #3, T-11-06)
- `Shutdown(ctx)` closes the channel and drains via a `WaitGroup`+`select` bounded by `ctx`, per the RESEARCH Pattern-3 corrected ordering (caller guarantees no in-flight HTTP handler before calling it)
- `depth() int64` samples live channel occupancy for the Wave-3 `telemetry.RegisterSummaryQueueDepth` gauge callback
- Nine deterministic/integration tests, all `-race` clean where required, zero `time.Sleep` in the test file

## Task Commits

1. **Task 1: Implement the summaryQueue worker pool (bounded, non-blocking, bounded-retry, drainable)** - `add49df` (feat)
2. **Task 2: Deterministic queue tests — never-blocks, drop-and-count, retry-gives-up, drain-within-budget** - `266c154` (test)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified
- `internal/server/summaryqueue.go` - `summaryQueue` type, `newSummaryQueue`/`tryEnqueue`/`Start`/`worker`/`process`/`itemDone`/`Shutdown`/`Wait`/`depth`, `storeFill` production fill-builder, `newRetryBackOff` helper
- `internal/server/summaryqueue_test.go` - 9 tests: the 7 spec'd deterministic behaviors plus 2 supplementary tests (`depth()` occupancy, `storeFill` integration) added to keep `storeFill`/`depth` exercised for lint purposes ahead of Wave 3's wiring
- `internal/store/summarize.go` - new exported `LogSummaryEgress(ctx, m, model, outcome, err)`; `SummarizeMissing` refactored to call it instead of an inline closure (fields/message unchanged)

## Decisions Made
- `LogSummaryEgress` is called for all three terminal outcomes (`filled`/`skipped`/`failed`) in the async worker's `storeFill`, per the plan's explicit "each terminal fill outcome emits ... egress audit" behavior spec — `SummarizeMissing` itself never reaches a `skipped` call site (it pre-filters eligibility before calling `FillSummary`), so its call sites stay `filled`/`failed` only, matching pre-refactor behavior exactly.
- `WithMaxElapsedTime` is a fixed 20s constant, not scaled to the injected `attemptTimeout`, so a tiny test `attemptTimeout` can't cause the elapsed-time ceiling to fire before `WithMaxTries` does — `WithMaxTries(3)` is the primary bound.
- Added two Qdrant-gated integration tests (`TestStoreFillFillsEligibleRecord`, `TestStoreFillReturnsPermanentOnNotFound`, skip-gated via the existing `testDeps` helper) plus a Qdrant-free `TestSummaryQueueDepthReflectsChannelOccupancy`, beyond the plan's 7 spec'd behaviors — necessary to keep `storeFill`/`depth()` non-dead-code ahead of Wave 3's wiring into `buildDepsFromEnv`/`deps`, since `task lint`'s `unused` checker otherwise flags both symbols (see Deviations).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added integration/unit test coverage for `storeFill`/`depth()` to keep `task lint` clean**
- **Found during:** Task 2, after implementing the 7 spec'd tests
- **Issue:** `storeFill` (the production fill-builder) and `depth()` (the queue-depth sampler) are only ever *called* by Wave 3's `buildDepsFromEnv`/gauge-registration wiring, which is explicitly out of scope for this plan. With only the 7 behavior-spec'd tests (which exercise the queue purely via the injected fake `fill` closure), `golangci-lint`'s `unused` check flagged both symbols as dead code, failing `task lint` — a plan-level verification requirement (`<verification>`: "task lint clean").
- **Fix:** Added `TestSummaryQueueDepthReflectsChannelOccupancy` (Qdrant-free) and two Qdrant-gated integration tests, `TestStoreFillFillsEligibleRecord` / `TestStoreFillReturnsPermanentOnNotFound` (skip-gated via the existing `testDeps(t)` helper, same posture as the rest of the package's integration tests), exercising both symbols directly.
- **Files modified:** `internal/server/summaryqueue_test.go`
- **Verification:** `task lint:go` → `0 issues`; `go test ./internal/server/... -run 'TestSummaryQueue|TestStoreFill' -v` all pass
- **Committed in:** `266c154` (Task 2 commit)

**2. [Rule 1 - Bug] Removed a literal "time.Sleep" substring from a test-file comment**
- **Found during:** Task 2, verifying the plan's `grep -c 'time.Sleep' ... reports 0` acceptance criterion
- **Issue:** An explanatory comment on the `shutdownWithinBudget` test helper used the phrase "time.Sleep-free way to bound test cleanup", which the literal grep-based acceptance check would have flagged as a false positive (the check greps the raw text, not code semantics).
- **Fix:** Reworded the comment to "sleep-free way to bound test cleanup deterministically" — no functional change.
- **Files modified:** `internal/server/summaryqueue_test.go`
- **Verification:** `grep -c 'time.Sleep' internal/server/summaryqueue_test.go` → `0`
- **Committed in:** `266c154` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking/test-coverage, 1 bug/false-positive text match)
**Impact on plan:** Both fixes stay within `internal/server/summaryqueue_test.go` — no scope creep into files outside this plan's `files_modified`. The added tests are strictly additive coverage for symbols the plan itself defines; no behavior changed.

## Issues Encountered
None beyond the deviations above.

## User Setup Required
None - no external service configuration required. `storeFill`/`depth()` remain unwired (no `ENGRAM_SUMMARY_ON_WRITE` behavior change) until Wave 3 (`11-03`) wires the queue into `deps`/`buildDepsFromEnv`/`Register`.

## Next Phase Readiness
- Wave 3 (`11-03`, server wiring) can now: construct `newSummaryQueue(workers, queueSize, summaryTimeout(cfg), telemetry.NewSummaryQueueMetrics(meter), storeFill(st, summarize, model, maxChars))` in `buildDepsFromEnv`; call `telemetry.RegisterSummaryQueueDepth(meter, q.depth)`; call `q.Start(ctx)` at server startup; call `deps.summaryQueue.tryEnqueue(m.ID)` from `storeMemory`'s Upsert-success tail; and call `q.Shutdown(ctx)` from `Register`'s returned shutdown func, strictly AFTER `httpSrv.Shutdown(ctx)` has returned (Pattern 3 ordering — enforced by this plan's `Shutdown` implementation, not re-verified until Wave 3 wires the caller).
- No blockers.

---
*Phase: 11-async-on-write-summaries*
*Completed: 2026-07-10*

## Self-Check: PASSED

All modified/created files (`internal/server/summaryqueue.go`, `internal/server/summaryqueue_test.go`, `internal/store/summarize.go`) and this SUMMARY.md confirmed present on disk. Both task commits (`add49df`, `266c154`) confirmed in `git log`.
