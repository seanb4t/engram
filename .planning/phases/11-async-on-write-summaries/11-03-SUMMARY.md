---
phase: 11-async-on-write-summaries
plan: 03
subsystem: server
tags: [go-concurrency, otel-metrics, worker-pool, shutdown-ordering]

requires:
  - phase: 11-01
    provides: "ENGRAM_SUMMARY_ON_WRITE/_WORKERS/_QUEUE_SIZE config knobs; telemetry.SummaryQueueMetrics static instruments + RegisterSummaryQueueDepth"
  - phase: 11-02
    provides: "internal/server/summaryqueue.go: bounded, nil-safe, non-blocking summaryQueue worker pool (tryEnqueue/Start/Shutdown/Wait/depth); storeFill production fill-builder; store.LogSummaryEgress"
provides:
  - "deps.summaryQueue: nil-safe field, constructed only under the D-01 two-switch AND-gate (buildSummaryQueue)"
  - "store_memory/schedule_memory enqueue-after-Upsert; store_discovery/store_rule negative space (D-06)"
  - "Register(s, mux, tm, sqm, resolve) (shutdown func(context.Context), err error) — grown signature, drain closure"
  - "serve.go shutdown ordering: drainSummaries invoked strictly after httpSrv.Shutdown returns (D-08)"
  - "depth gauge registered from buildDepsFromEnv via telemetry.RegisterSummaryQueueDepth (D-09)"
  - "docs-site/guides/configure.md + CLAUDE.md: async-on-write knobs, two-step opt-in, degradation contract"
affects: []

tech-stack:
  added: []
  patterns:
    - "runtime AND-gate lives in a dedicated buildSummaryQueue helper (called from buildDepsFromEnv), separate from Config.Validate()'s unconditional parseability checks"
    - "Register grows to return a caller-invoked shutdown closure rather than performing lifecycle teardown itself, mirroring the single-caller-single-owner shape already used for tm/mux"
    - "test-only summaryQueue injection via a testDepsWithSummaryQueue helper that wraps testDeps, keeping every existing testDeps call site unchanged"

key-files:
  created: []
  modified:
    - internal/server/tools.go
    - internal/server/tools_test.go
    - cmd/engram/serve.go
    - docs-site/src/content/docs/guides/configure.md
    - CLAUDE.md

key-decisions:
  - "buildSummaryQueue treats an unparseable ENGRAM_SUMMARY_ON_WRITE the same as false (disabled), logging nothing extra beyond the existing Validate()-time warning path — the AND-gate's job is enable/disable, not re-diagnosing a value Validate() already checked at startup"
  - "buildDepsFromEnv takes sqm *telemetry.SummaryQueueMetrics as an explicit parameter (threaded from Register) rather than constructing its own, so serve.go's single meter/instrument set is the only source of the static instruments — mirrors how tm flows into instrumentTools"
  - "The three new tool.go doc-comment/test additions stay literally free of the string 'tryEnqueue' outside the two real call sites, per the plan's literal grep-based acceptance check (grep -c == 2)"

requirements-completed: [REQ-async-summaries]

coverage:
  - id: D1
    description: "The async worker is constructed and started ONLY when BOTH ENGRAM_SUMMARY_MODEL is set AND ENGRAM_SUMMARY_ON_WRITE parses true; otherwise deps.summaryQueue is nil (D-01)"
    requirement: "REQ-async-summaries"
    verification:
      - kind: other
        ref: "internal/server/tools.go buildSummaryQueue: two early-return nil branches (Model==\"\" ; ParseBool err or !onWrite); go build/vet clean"
        status: pass
    human_judgment: false
  - id: D2
    description: "store_memory and schedule_memory enqueue the record id AFTER a successful Upsert; store_discovery and store_rule NEVER enqueue (D-06)"
    requirement: "REQ-async-summaries"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreMemoryEnqueuesOnSuccess"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestDiscoveryAndRuleNeverEnqueue"
        status: pass
    human_judgment: false
  - id: D3
    description: "A successful store_memory returns unconditionally even when the summarizer hangs/fails (SC#2)"
    requirement: "REQ-async-summaries"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestStoreMemoryReturnsWhenSummarizerHangs (-race)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The pool drains strictly AFTER httpSrv.Shutdown(ctx) returns, never in parallel (D-08)"
    requirement: "REQ-async-summaries"
    verification:
      - kind: other
        ref: "cmd/engram/serve.go: shutdownErr := httpSrv.Shutdown(shutdownCtx) precedes drainSummaries(shutdownCtx) textually and sequentially in the same case block"
        status: pass
    human_judgment: false
  - id: D5
    description: "Docs describe the new knobs, the two-step opt-in, the manual task eval:summary gate (not CI), and the degradation contract (D-02, SC#3)"
    requirement: "REQ-async-summaries"
    verification:
      - kind: other
        ref: "docs-site/src/content/docs/guides/configure.md §Async-on-write summaries; CLAUDE.md Memory contract addition"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-07-10
status: complete
---

# Phase 11 Plan 03: Async-on-Write Server Wiring Summary

**The three previously-inert Wave-1/2 building blocks (config knobs, worker-pool core, static instruments) are now live: `store_memory`/`schedule_memory` enqueue post-Upsert behind a runtime two-switch AND-gate, `serve.go` drains the pool strictly after HTTP shutdown, the depth gauge is on OTLP, and the feature is documented end to end.**

## Performance

- **Duration:** ~45 min
- **Completed:** 2026-07-10T13:23:08Z
- **Tasks:** 3
- **Files modified:** 5 (0 created)

## Accomplishments
- `deps.summaryQueue *summaryQueue` (nil-safe field) + `buildSummaryQueue(cfg, st, sqm) *summaryQueue`: constructs and starts the worker pool ONLY when `cfg.Summarize.Model != ""` AND `strconv.ParseBool(cfg.Summarize.OnWrite)` is true — the D-01 AND-gate's actual runtime home, distinct from `Config.Validate()`'s unconditional parseability checks from Plan 11-01
- `storeMemory`/`scheduleMemory` tails split so `tryEnqueue(m.ID)` fires exactly once, only after a confirmed-successful `Upsert` (`grep -c 'tryEnqueue' internal/server/tools.go` == 2); `storeDiscovery`/`storeRule` are untouched (D-06 negative space)
- `Register` grew to `func Register(s, mux, tm, sqm *telemetry.SummaryQueueMetrics, resolve) (shutdown func(context.Context), err error)` — the single production caller (`cmd/engram/serve.go:145`) now captures and invokes the drain closure
- `serve.go` builds the static `telemetry.NewSummaryQueueMetrics(meter)` instruments alongside the existing `tm`, threads `sqm` through `Register`, and invokes `drainSummaries(shutdownCtx)` strictly AFTER `httpSrv.Shutdown(shutdownCtx)` returns (sequential, same case block) — closes the send-on-closed-channel panic risk identified in RESEARCH Pitfall 1 (D-08)
- The D-09 queue-depth `ObservableGauge` is registered from inside `buildSummaryQueue` via `telemetry.RegisterSummaryQueueDepth(otel.Meter(...), q.depth)`, immediately after the queue is constructed and started — the only point in the codebase where `q.depth()` has a live queue to close over
- `testDepsWithSummaryQueue(t, workers, queueSize, fill)` test helper injects a live, test-controlled `*summaryQueue` onto `deps` without touching any existing `testDeps` call site
- Three new handler-level tests, all passing under `-race`: `TestStoreMemoryEnqueuesOnSuccess` (SC#1, deterministic via `Wait()`), `TestStoreMemoryReturnsWhenSummarizerHangs` (SC#2, prompt return with a permanently-blocked fill), `TestDiscoveryAndRuleNeverEnqueue` (D-06 negative space, atomic counter stays 0)
- `docs-site/src/content/docs/guides/configure.md` gained an "Async-on-write summaries" subsection: the three env vars, the two-step opt-in (`task eval:summary` as an explicitly manual, non-CI gate), bounded/non-blocking behavior, and the "no summary yet" degradation contract
- `CLAUDE.md`'s Memory contract section gained a concise note on async summary fill and the degradation-never-fails-a-write guarantee

## Task Commits

1. **Task 1: Wire enqueue into deps + handlers behind the D-01 AND-gate; grow Register's signature (D-01, D-06)** - `22aab609` (feat)
2. **Task 2: Wire lifecycle in serve.go — build static SummaryQueueMetrics, capture + invoke drain after httpSrv.Shutdown (D-08, D-09)** - `e4d6d87a` (feat)
3. **Task 3: Enqueue-on-success + write-path-degradation tests, and docs/CLAUDE.md for the new knobs (SC#1, SC#2, D-02)** - `9d75400f` (test)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified
- `internal/server/tools.go` - `deps.summaryQueue` field; `buildSummaryQueue` (D-01 AND-gate + gauge registration); `buildDepsFromEnv(sqm)` now threads `sqm` through; `storeMemory`/`scheduleMemory` tails split for post-Upsert `tryEnqueue`; `storeDiscovery` doc comment notes the D-06 exclusion; `Register` signature grown, returns `d.summaryQueue.Shutdown` as the drain closure
- `internal/server/tools_test.go` - `testDepsWithSummaryQueue` helper; three new tests (`TestStoreMemoryEnqueuesOnSuccess`, `TestStoreMemoryReturnsWhenSummarizerHangs`, `TestDiscoveryAndRuleNeverEnqueue`); two pre-existing call sites (`buildDepsFromEnv`, `Register`) updated for the new signatures; `sync`/`sync/atomic` imports added
- `cmd/engram/serve.go` - `sqm := telemetry.NewSummaryQueueMetrics(...)` built alongside `tm`; `Register` call captures `drainSummaries`; SIGTERM branch calls `httpSrv.Shutdown` then `drainSummaries(shutdownCtx)` sequentially
- `docs-site/src/content/docs/guides/configure.md` - new "Async-on-write summaries" subsection under Auto-summary
- `CLAUDE.md` - Memory contract note on async summary fill + degradation

## Decisions Made
- `buildSummaryQueue` treats an unparseable `ENGRAM_SUMMARY_ON_WRITE` identically to `false` (disabled) — `Config.Validate()` (Plan 11-01) already surfaces a startup-time warning/error for a malformed value, so the AND-gate itself stays a simple enable/disable decision without re-diagnosing.
- `buildDepsFromEnv` takes `sqm *telemetry.SummaryQueueMetrics` as an explicit parameter threaded in from `Register` (which gets it from `serve.go`), rather than constructing its own instance — keeps exactly one `SummaryQueueMetrics` per process, matching the existing `tm`/`instrumentTools` ownership shape.
- The `storeDiscovery` doc-comment intentionally avoids the literal substring `tryEnqueue` (says "enqueues for async summary fill" instead) so the plan's literal `grep -c 'tryEnqueue' internal/server/tools.go == 2` acceptance check isn't defeated by an explanatory comment.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated two additional `buildDepsFromEnv`/`Register` call sites in tools_test.go not listed in the plan's action text**
- **Found during:** Task 1, `go vet ./internal/server/...`
- **Issue:** `TestBuildDepsFromEnvLoadsConfigOnce` called `buildDepsFromEnv()` (no args) and `TestRegisterReturnsErrorOnStoreInitFailure` called `Register(s, mux, tm, nil)` (4 args) — both pre-existing call sites broken by the signature changes the plan's Task 1 `<action>` explicitly specifies.
- **Fix:** Updated to `buildDepsFromEnv(nil)` and `Register(s, mux, tm, nil, nil)` respectively — `nil` `sqm`/queue is exactly the disabled-path semantics both tests already exercise.
- **Files modified:** `internal/server/tools_test.go`
- **Verification:** `go vet ./internal/server/...` clean; `task test:go` green
- **Committed in:** `22aab609` (Task 1 commit)

**2. [Rule 1 - Bug] Reverted unrelated `task fmt` drift on 4 out-of-scope files**
- **Found during:** Task 3, running `task fmt` ahead of the `task` gate
- **Issue:** `task fmt` (dprint) reformatted `.claude/settings.json`, `docs-site/package.json`, `internal/webauth/static/_app/version.json`, and `ui/tsconfig.json` — pre-existing formatting drift unrelated to this plan's `files_modified`.
- **Fix:** `git checkout --` on all four files before committing; left the drift untouched (SCOPE BOUNDARY — pre-existing, not caused by this plan's tasks).
- **Files modified:** none (reverted, not committed)
- **Verification:** `git status --short` after checkout shows only this plan's intended files
- **Committed in:** not committed (reverted before staging)

---

**Total deviations:** 2 auto-fixed (1 blocking/compile-fix, 1 bug/scope-guard). Both are mechanical corrections required to keep the plan's own explicit instructions (signature changes) compiling and to avoid smuggling unrelated formatter drift into this plan's commits. No scope creep into files outside `files_modified`.

## Issues Encountered
`task` (the full default gate, including `lint:markdown`) fails on pre-existing `rumdl` findings across `.planning/phases/09-*` and `.planning/phases/11-*` markdown files (none touched by this plan's tasks; predate this execution per `git log`) — out of scope per the SCOPE BOUNDARY rule, consistent with 11-01's and 11-02's identical note. `task lint:go` (golangci-lint), `task license:check`, and `task test:go` (including `-race` runs of the three new tests and the full `internal/server` package) are all clean. `go build ./...` and `go mod tidy` (verified via `go.mod`/`go.sum` diff after tidy) show zero drift.

## User Setup Required
None for the code to compile and tests to pass. To actually exercise the async-on-write path in a live deployment: an operator sets `ENGRAM_SUMMARY_MODEL`, runs `task eval:summary` to judge fidelity for their model/data (manual, per-deployment, not CI), then sets `ENGRAM_SUMMARY_ON_WRITE=true`. `ENGRAM_SUMMARY_WORKERS`/`ENGRAM_SUMMARY_QUEUE_SIZE` are optional tuning knobs with sane defaults (2 workers, 256-deep queue).

## Next Phase Readiness
- Phase 11 (async-on-write-summaries) is now fully implemented across all three plans: config knobs + instruments (11-01), worker-pool core (11-02), and server wiring + docs (11-03, this plan).
- No blockers for the next phase (12: Per-Memory Usage Signals).

---
*Phase: 11-async-on-write-summaries*
*Completed: 2026-07-10*

## Self-Check: PASSED

All modified files (`internal/server/tools.go`, `internal/server/tools_test.go`, `cmd/engram/serve.go`, `docs-site/src/content/docs/guides/configure.md`, `CLAUDE.md`) and this SUMMARY.md confirmed present on disk. All three task commits (`22aab609`, `e4d6d87a`, `9d75400f`) confirmed in `git log`.
