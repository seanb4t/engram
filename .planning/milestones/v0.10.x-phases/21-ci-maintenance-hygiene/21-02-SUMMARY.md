---
phase: 21-ci-maintenance-hygiene
plan: 02
subsystem: api
tags: [go, testing, goroutine-leak-fix, tdd, refactor]

requires:
  - phase: 11-async-on-write-summaries
    provides: summaryQueue/usageQueue worker-pool kernel and the storeMemory/scheduleMemory write-path enqueue call sites this plan refactors
provides:
  - "queue Wait() structurally unreachable from production code for both summaryQueue and usageQueue"
  - "deps.persistAndEnqueue shared helper replacing the duplicated MintShortID->Upsert->tryEnqueue block in storeMemory and scheduleMemory"
  - "hermetic TestBuildDepsFromEnvLoadsConfigOnce (no more ambient-env goroutine leak)"
affects: [internal/server]

tech-stack:
  added: []
  patterns:
    - "test-only export file convention (queue_export_test.go) for methods that must not exist in production builds"
    - "memStore-embedding wrapper struct that overrides a single method to inject a failure (upsertFailStore), for Qdrant-free error-path characterization tests"

key-files:
  created:
    - internal/server/queue_export_test.go
  modified:
    - internal/server/summaryqueue.go
    - internal/server/usagequeue.go
    - internal/server/tools.go
    - internal/server/tools_test.go

key-decisions:
  - "Wait() relocated to a single new file internal/server/queue_export_test.go (not inlined per-queue into summaryqueue_test.go/usagequeue_test.go) — groups both test-only escape hatches explicitly, establishing the repo's first export_test.go-style convention"
  - "persistAndEnqueue signature: func (d *deps) persistAndEnqueue(ctx context.Context, m store.Memory, vec []float32) (id, shortID string, err error) — exact shape from 21-PATTERNS.md, fits both call sites with zero adaptation"
  - "storeDiscovery/storeRule deliberately excluded from persistAndEnqueue (D-05 negative space) — each keeps its own independent MintShortID/logic; discoveries own their own summaries"

patterns-established:
  - "queue_export_test.go: test-only method relocation via the _test.go compiler-exclusion mechanism, no build tags"

requirements-completed: [REQ-p11-review-residuals]

coverage:
  - id: D1
    description: "Wait() cannot be called from production code for either summaryQueue or usageQueue — moved to a new test-only file, no non-test file defines it"
    requirement: "REQ-p11-review-residuals"
    verification:
      - kind: unit
        ref: "go build ./... (production build succeeds with no Wait method); rg --glob '!*_test.go' -n 'func \\(q \\*(summaryQueue|usageQueue)\\) Wait' internal/server/ (no match)"
        status: pass
      - kind: unit
        ref: "internal/server -run 'TestSummaryQueue|TestUsageQueue' (all existing queue tests pass unmodified)"
        status: pass
    human_judgment: false
  - id: D2
    description: "storeMemory and scheduleMemory no longer duplicate the MintShortID->Upsert->enqueue block; both delegate to deps.persistAndEnqueue, and a failing Upsert produces zero enqueues from either handler"
    requirement: "REQ-p11-review-residuals"
    verification:
      - kind: unit
        ref: "internal/server#TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure (confirmed passing BOTH before and after the extraction)"
        status: pass
      - kind: unit
        ref: "internal/server#TestStoreMemoryEnqueuesOnSuccess, #TestStoreMemoryReturnsWhenSummarizerHangs, #TestDiscoveryAndRuleNeverEnqueue (all pass unmodified)"
        status: pass
    human_judgment: false
  - id: D3
    description: "TestBuildDepsFromEnvLoadsConfigOnce is hermetic against ambient ENGRAM_SUMMARY_* env and starts no queue"
    requirement: "REQ-p11-review-residuals"
    verification:
      - kind: unit
        ref: "internal/server#TestBuildDepsFromEnvLoadsConfigOnce, run both with a clean env and with ENGRAM_SUMMARY_ON_WRITE=true ENGRAM_SUMMARY_MODEL=gpt-4o exported ambiently"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-07-16
status: complete
---

# Phase 21 Plan 02: Phase-11 async-summary review residuals Summary

**`Wait()` relocated to a test-only file for both queues, a shared `persistAndEnqueue` helper collapses the duplicated write-path tail, and a leaked-goroutine test is now hermetic — closing WR-03/IN-01/IN-02 from issue #335.**

## Performance

- **Duration:** ~15 min
- **Tasks:** 3
- **Files modified:** 5 (1 created, 4 modified)

## Accomplishments

- **WR-03 (D-04):** `Wait()` no longer exists in any file `go build` compiles, for both `summaryQueue` and `usageQueue` — moved verbatim into a new `internal/server/queue_export_test.go` (`package server`, in-package, no build tag needed). All 10 existing `_test.go` call sites resolve unchanged.
- **IN-01 (D-05):** Extracted `func (d *deps) persistAndEnqueue(ctx context.Context, m store.Memory, vec []float32) (id, shortID string, err error)` into `internal/server/tools.go`. `storeMemory` and `scheduleMemory` now both delegate to it instead of duplicating the `MintShortID -> Upsert -> tryEnqueue` sequence. `storeDiscovery`/`storeRule` remain untouched — they never enqueue.
- **New regression test:** `TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure` in `internal/server/tools_test.go`, built on a new `upsertFailStore` wrapper (embeds `memStore`, overrides only `Upsert` to return an injected error — `spyStore.Upsert` has no error-injection hook). Covers both `storeMemory` and `scheduleMemory` failing on `Upsert` and asserts zero enqueues via the `Wait()` drain seam (no `time.Sleep`). **Confirmed passing against the un-refactored, still-duplicated code before the extraction, and re-confirmed passing after** — the characterization discipline the plan required.
- **IN-02 (D-06):** Added `t.Setenv("ENGRAM_SUMMARY_MODEL", "")` and `t.Setenv("ENGRAM_SUMMARY_ON_WRITE", "")` to `TestBuildDepsFromEnvLoadsConfigOnce`, matching `config_test.go`'s `TestLoadDefaults` empty-value-preserves-default pattern. Verified manually with `ENGRAM_SUMMARY_ON_WRITE=true ENGRAM_SUMMARY_MODEL=gpt-4o` exported in the shell — the test still passes and starts no summary queue.

## Task Commits

Each task was committed atomically:

1. **Task 1: Make Wait() unreachable from production for both queues (D-04/WR-03)** - `7c16e56b` (refactor)
2. **Task 2: Extract persistAndEnqueue and pin its ordering invariant (D-05/IN-01)** - `01a27606` (refactor)
3. **Task 3: Make TestBuildDepsFromEnvLoadsConfigOnce hermetic (D-06/IN-02)** - `298c4239` (fix)

## Files Created/Modified

- `internal/server/queue_export_test.go` - NEW: relocated `summaryQueue.Wait()` and `usageQueue.Wait()`, test-only, SPDX header
- `internal/server/summaryqueue.go` - `Wait()` method removed
- `internal/server/usagequeue.go` - `Wait()` method removed
- `internal/server/tools.go` - new `deps.persistAndEnqueue` helper; `storeMemory`/`scheduleMemory` collapsed to call it
- `internal/server/tools_test.go` - new `upsertFailStore` type + `TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure`; `TestBuildDepsFromEnvLoadsConfigOnce` gained two `t.Setenv("")` calls

## Decisions Made

- **`Wait()` relocation mechanism:** a single new shared file `internal/server/queue_export_test.go` rather than inlining each `Wait()` into its own `*_test.go` file — groups the "test-only escape hatch" convention explicitly and establishes it for the package (no prior `export_test.go` convention existed).
- **`persistAndEnqueue` final signature:** `func (d *deps) persistAndEnqueue(ctx context.Context, m store.Memory, vec []float32) (id, shortID string, err error)` — matches 21-PATTERNS.md's suggested shape exactly; both call sites needed zero adaptation since `m` and `vec` are already fully built by the time each handler reaches the shared tail.
- **Characterization test confirmation:** `TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure` was run and passed against the un-refactored, still-duplicated `storeMemory`/`scheduleMemory` code (`go test ./internal/server/... -run TestPersistAndEnqueueSkipsEnqueueOnUpsertFailure -v -count=1` → PASS, before Step 2's extraction was applied), then re-run and confirmed passing after the extraction. This satisfies the plan's fail-fast characterization requirement — the invariant was never unguarded during the refactor.

## Deviations from Plan

None — plan executed exactly as written. All three tasks matched 21-PATTERNS.md's analogs with no adaptation needed.

## Issues Encountered

**Pre-existing, out-of-scope `task lint:yaml` failure (not fixed, logged to `deferred-items.md`):** the plan's final verification step ("Once Plan 01 has landed: `task` full default gate green") surfaced that `task` (full default gate) fails at `lint:yaml` (`yamlfmt -lint .` flags `Taskfile.yaml`'s formatting). Confirmed byte-identical `Taskfile.yaml` at the pre-plan commit (`bb3edccb`, end of Plan 01) — this predates and is unrelated to any of 21-02's changes (`internal/server/` only). `task lint:go`, `task license:check`, `gofmt -l .`, and `task test:go` are all clean. Per the executor's scope-boundary rule, this was logged to `.planning/phases/21-ci-maintenance-hygiene/deferred-items.md` rather than fixed inline — it needs its own issue.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- REQ-p11-review-residuals (issue #335) fully closed: WR-03, IN-01, IN-02 all resolved with `usageQueue` covered alongside `summaryQueue` per D-00c.
- No behavior change observable to any client: no proto/wire/public-API/config change.
- Plan 21-03 (Renovate self-heal, #301) is independent of this plan's file set and can proceed.
- The pre-existing `Taskfile.yaml` yamlfmt failure (see Issues Encountered) should be filed as its own issue — it currently blocks a fully green `task` default gate, independent of Phase 21's three plans.

---
*Phase: 21-ci-maintenance-hygiene*
*Completed: 2026-07-16*

## Self-Check: PASSED

All created/modified files confirmed present on disk; all 4 commits (7c16e56b, 01a27606, 298c4239, d8dd87ab) confirmed in `git log`.
