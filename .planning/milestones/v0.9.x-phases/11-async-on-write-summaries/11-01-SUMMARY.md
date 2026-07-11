---
phase: 11-async-on-write-summaries
plan: 01
subsystem: config
tags: [koanf, opentelemetry, backoff, go-modules]

requires: []
provides:
  - "ENGRAM_SUMMARY_ON_WRITE / _WORKERS / _QUEUE_SIZE koanf-string config fields with Validate() rules"
  - "github.com/cenkalti/backoff/v5 promoted to a direct go.mod require (zero go.sum churn)"
  - "telemetry.SummaryQueueMetrics static instruments + telemetry.RegisterSummaryQueueDepth standalone gauge helper"
affects: [11-02-queue-core, 11-03-server-wiring]

tech-stack:
  added: []
  patterns:
    - "koanf-string / parse-at-use config fields (mirrors ui.enabled and embed.dim, not native bool/int koanf types)"
    - "SummaryQueueMetrics static-instrument constructor + separately-registered ObservableGauge, split by whether a live queue exists at call time"

key-files:
  created: []
  modified:
    - internal/config/registry.go
    - internal/config/config.go
    - internal/config/validate.go
    - internal/config/validate_test.go
    - internal/config/config_test.go
    - internal/telemetry/metrics.go
    - go.mod

key-decisions:
  - "ON_WRITE/WORKERS/QUEUE_SIZE Validate() checks run unconditionally (not gated by Summarize.Model != \"\"), since the fields carry safe defaults and the true D-01 AND-gate is a Wave-3 runtime concern"
  - "backoff/v5 promoted to direct via a go.mod-only hand-edit (moved between require() blocks), not go get/go mod tidy — nothing in this plan's own source imports it yet (Wave 2 does); go mod tidy would otherwise revert the indirect marker until 11-02 lands"

requirements-completed: [REQ-async-summaries]

coverage:
  - id: D1
    description: "ENGRAM_SUMMARY_ON_WRITE/_WORKERS/_QUEUE_SIZE registered with defaults and validated (bool/positive-int)"
    requirement: "REQ-async-summaries"
    verification:
      - kind: unit
        ref: "internal/config/validate_test.go#TestValidateFieldRules (summary_on_write_non-bool, summary_workers_*, summary_queue_size_*)"
        status: pass
      - kind: unit
        ref: "internal/config/validate_test.go#TestValidateHappyPath"
        status: pass
    human_judgment: false
  - id: D2
    description: "backoff/v5 is a direct go.mod require with zero go.sum churn"
    requirement: "REQ-async-summaries"
    verification:
      - kind: other
        ref: "go mod verify && git diff --stat go.sum (empty)"
        status: pass
    human_judgment: false
  - id: D3
    description: "telemetry.NewSummaryQueueMetrics(meter) builds static counters + fill histogram; telemetry.RegisterSummaryQueueDepth(meter, depth) is a separate gauge helper"
    requirement: "REQ-async-summaries"
    verification:
      - kind: other
        ref: "go build ./... && go vet ./internal/telemetry/..."
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-07-10
status: complete
---

# Phase 11 Plan 01: Config Knobs, backoff/v5 Promotion, Summary Queue Instruments Summary

**Three `ENGRAM_SUMMARY_*` config knobs with validation, `cenkalti/backoff/v5` promoted to a direct dependency at zero go.sum cost, and `telemetry.SummaryQueueMetrics` instruments ready for the Wave-2 async summary queue.**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-07-10T12:56:11Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- `ENGRAM_SUMMARY_ON_WRITE` (default `"false"`), `ENGRAM_SUMMARY_WORKERS` (default `"2"`), `ENGRAM_SUMMARY_QUEUE_SIZE` (default `"256"`) registered as koanf-string fields, validated unconditionally in `Config.Validate()` (bool-parseable / positive-integer)
- `github.com/cenkalti/backoff/v5` moved from the indirect require block to the direct block in `go.mod` — no network fetch, no `go.sum` hash change (already pinned at v5.0.3 via `otlploggrpc`)
- `telemetry.SummaryQueueMetrics` added: static `enqueued`/`dropped`/`failed`/`retried` `Int64Counter`s + a `fill.duration` `Float64Histogram`, with encapsulated ctx-taking record helpers (mirrors `ToolMetrics`)
- `telemetry.RegisterSummaryQueueDepth(meter, depth)` added as a standalone helper for the queue-depth `Int64ObservableGauge`, deliberately not owned by the static constructor since its `depth()` closure needs a live queue that only exists once Wave-3 `buildDepsFromEnv` constructs it

## Task Commits

1. **Task 1: Add ENGRAM_SUMMARY_ON_WRITE / _WORKERS / _QUEUE_SIZE config knobs + Validate() rules** - `bad2ffb` (feat)
2. **Task 2: Promote cenkalti/backoff/v5 to direct + add telemetry.SummaryQueueMetrics instruments** - `d884aff` (feat)

**Plan metadata:** (this commit, docs: complete plan)

## Files Created/Modified
- `internal/config/registry.go` - three new `summarize.*` registry entries with string defaults
- `internal/config/config.go` - `SummarizeConfig.OnWrite/Workers/QueueSize` koanf fields
- `internal/config/validate.go` - unconditional bool/positive-int `Validate()` checks for the three new fields
- `internal/config/validate_test.go` - `validConfig()`/`summarizeEnabled()` builders updated for the new unconditional checks; new table-driven ON_WRITE/WORKERS/QUEUE_SIZE cases
- `internal/config/config_test.go` - `TestValidateIgnoresSummaryWhenDisabled` updated with valid `OnWrite`/`Workers`/`QueueSize` so it still asserts nil error
- `internal/telemetry/metrics.go` - `SummaryQueueMetrics` type + `NewSummaryQueueMetrics` constructor + record helpers + standalone `RegisterSummaryQueueDepth`
- `go.mod` - `github.com/cenkalti/backoff/v5` moved to the direct require block

## Decisions Made
- Made the three new `Validate()` checks unconditional rather than nesting them inside the existing `if c.Summarize.Model != ""` block, per the plan's explicit instruction (D-01/D-07) — the runtime "model set AND on_write true" AND-gate is a Wave-3 (`buildDepsFromEnv`) concern, not a `Validate()` concern.
- Promoted `backoff/v5` via a direct `go.mod` text edit (moving the require line between the two `require()` blocks) rather than `go get`/`go mod tidy`, because no `.go` file in this plan imports it yet — that import lands in Wave 2 (`11-02`, `summaryqueue.go`). Running `go mod tidy` now would have reverted the indirect marker until that import exists. `go mod verify` confirms no supply-chain drift and `git diff --stat go.sum` is empty.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test builders broken by new unconditional Validate() checks**
- **Found during:** Task 1
- **Issue:** Making the ON_WRITE/WORKERS/QUEUE_SIZE checks unconditional (as the plan specifies) meant `TestValidateIgnoresSummaryWhenDisabled` (in `internal/config/config_test.go`, not listed in the plan's `files_modified`) would start failing — it constructs a `SummarizeConfig{Model: "", MaxChars: "garbage"}` literal that leaves the three new fields at their zero value `""`, which fails `strconv.ParseBool("")`/`strconv.ParseUint("")`.
- **Fix:** Added `OnWrite: "false", Workers: "2", QueueSize: "256"` to that literal so the test still asserts its original intent (disabled summarize is tolerant of garbage `MaxChars`) without being defeated by the new unconditional checks.
- **Files modified:** `internal/config/config_test.go`
- **Verification:** `go test ./internal/config/...` passes (all cases, including `TestValidateIgnoresSummaryWhenDisabled`)
- **Committed in:** `bad2ffb` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix, test-only)
**Impact on plan:** Necessary correction to keep existing test intent valid under the plan's own unconditional-check design. No scope creep — no new files, no files outside the config package.

## Issues Encountered
- `task lint` (the full `default` gate) fails on pre-existing `rumdl` markdown-lint findings across `.planning/phases/11-async-on-write-summaries/*.md` (11-CONTEXT.md, 11-01/02/03-PLAN.md, 11-PATTERNS.md, 11-RESEARCH.md). None of these files were touched by this plan's tasks and `git log` confirms they predate this execution (introduced in earlier `docs(11):` commits on this branch) — out of scope per the SCOPE BOUNDARY rule. `task lint:go` (golangci-lint) and `task license:check` are both clean; `task test` (go + python) is green.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Wave 2 (`11-02`, queue core) can now import `backoff/v5` as a direct dependency, read `ENGRAM_SUMMARY_WORKERS`/`_QUEUE_SIZE` for pool/channel sizing, and call the `SummaryQueueMetrics` record helpers.
- Wave 3 (`11-03`, server wiring) can call `telemetry.NewSummaryQueueMetrics(meter)` in `serve.go` and `telemetry.RegisterSummaryQueueDepth(meter, depth)` in `buildDepsFromEnv` once the live queue exists.
- No blockers.

---
*Phase: 11-async-on-write-summaries*
*Completed: 2026-07-10*

## Self-Check: PASSED

All modified files (internal/config/registry.go, config.go, validate.go, validate_test.go, config_test.go, internal/telemetry/metrics.go, go.mod) and the SUMMARY.md itself confirmed present on disk. Both task commits (`bad2ffb`, `d884aff`) confirmed in `git log`.
