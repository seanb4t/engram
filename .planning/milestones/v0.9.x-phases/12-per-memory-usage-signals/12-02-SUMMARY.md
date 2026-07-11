---
phase: 12-per-memory-usage-signals
plan: 02
subsystem: infra
tags: [koanf, config, otel, telemetry, go]

# Dependency graph
requires:
  - phase: 12-per-memory-usage-signals (plan 01)
    provides: store-layer Memory.AccessCount/LastAccessedAt + IncrementAccess foundation
provides:
  - "ENGRAM_USAGE_SIGNALS koanf-registered config gate (usage.signals, default \"true\")"
  - "Config.Validate() parseability check for ENGRAM_USAGE_SIGNALS"
  - "telemetry.UsageQueueMetrics (Enqueued/Dropped/Failed, no retry)"
affects: [12-05 (usage queue engine), 12-06 (wiring in serve.go/buildDepsFromEnv)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "koanf field registry row + point-of-use strconv.ParseBool (never a native bool at Load())"
    - "static OTLP counter struct mirroring SummaryQueueMetrics, minus retry"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/registry.go
    - internal/config/validate.go
    - internal/config/validate_test.go
    - internal/config/config_test.go
    - internal/telemetry/metrics.go

key-decisions:
  - "usage.signals defaults to \"true\" (on), unlike summarize.on_write's \"false\" default — D-09: usage signals are local, non-egressing curation metadata."
  - "UsageQueueMetrics carries only enqueued/dropped/failed counters, no retried counter or latency histogram — D-10 mandates single-attempt, no retry."

patterns-established: []

requirements-completed: [REQ-usage-signals]

coverage:
  - id: D1
    description: "ENGRAM_USAGE_SIGNALS is a registered koanf field (usage.signals) defaulting to \"true\", validated as a boolean string, kept as a string (parsed at point-of-use, not at Load())."
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "internal/config/validate_test.go#TestValidateFieldRules/usage_signals_non-bool"
        status: pass
      - kind: unit
        ref: "internal/config/validate_test.go#TestValidateUsageSignalsFalseAccepted"
        status: pass
    human_judgment: false
  - id: D2
    description: "telemetry.UsageQueueMetrics exposes Enqueued/Dropped/Failed OTLP counters with no retry counter or histogram."
    requirement: "REQ-usage-signals"
    verification:
      - kind: unit
        ref: "go build ./internal/telemetry/... && go vet ./internal/telemetry/..."
        status: pass
    human_judgment: false

# Metrics
duration: 12min
completed: 2026-07-10
status: complete
---

# Phase 12 Plan 02: Config gate + telemetry counters Summary

**Added ENGRAM_USAGE_SIGNALS (koanf field, default on) and telemetry.UsageQueueMetrics (enqueued/dropped/failed, no retry) as standalone supporting infra for the async usage-signal engine built in later plans.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-10T17:00:00Z (approx)
- **Completed:** 2026-07-10T17:03:32Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- `config.UsageConfig{Signals string}` + `Config.Usage` field registered in the koanf struct tree
- `usage.signals` / `ENGRAM_USAGE_SIGNALS` registry row added with `Default: "true"` — the single source of truth for the env var, default value, and defaults-map assembly
- `Config.Validate()` unconditionally rejects a non-boolean `ENGRAM_USAGE_SIGNALS` value, mirroring the existing `ENGRAM_SUMMARY_ON_WRITE` check
- `telemetry.UsageQueueMetrics` added, mirroring `SummaryQueueMetrics`'s shape (constructor + `Enqueued`/`Dropped`/`Failed` methods) but deliberately omitting a retry counter and latency histogram per D-10

## Task Commits

Each task was committed atomically:

1. **Task 1: ENGRAM_USAGE_SIGNALS config field + registry + validate** - `d439dcd` (feat)
2. **Task 2: telemetry.UsageQueueMetrics OTLP counters** - `39373fe` (feat)

## Files Created/Modified

- `internal/config/config.go` - `UsageConfig` struct + `Config.Usage` field
- `internal/config/registry.go` - `usage.signals` / `ENGRAM_USAGE_SIGNALS` registry row (`Default: "true"`)
- `internal/config/validate.go` - unconditional `strconv.ParseBool(c.Usage.Signals)` check
- `internal/config/validate_test.go` - base `validConfig()` sets `Usage.Signals: "true"`; added non-bool table case and a dedicated `"false"`-accepted test
- `internal/config/config_test.go` - fixed a pre-existing inline `Config{}` literal (`TestValidateIgnoresSummaryWhenDisabled`) that didn't set `Usage.Signals`, which would otherwise fail `Validate()` after this change
- `internal/telemetry/metrics.go` - `UsageQueueMetrics` struct + `NewUsageQueueMetrics` constructor + `Enqueued`/`Dropped`/`Failed` methods

## Decisions Made

- Kept `Signals` as a string field (never a native bool at `Load()`), matching the `ui.enabled`/`summarize.on_write` precedent — the boolean parse happens at point-of-use in `buildUsageQueue` (Plan 06), not here.
- No queue/config wiring was done in this plan — this plan strictly adds the config field and the metrics type, per the plan's explicit scope boundary.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed pre-existing test broken by the new unconditional validate check**

- **Found during:** Task 1 (`go test ./internal/config/...` after adding the `ENGRAM_USAGE_SIGNALS` parseability check)
- **Issue:** `internal/config/config_test.go`'s `TestValidateIgnoresSummaryWhenDisabled` builds a `Config{}` literal directly (not via the shared `validConfig()` helper in `validate_test.go`) and didn't set `Usage.Signals`. The new unconditional `ENGRAM_USAGE_SIGNALS` parseability check then failed on the empty string.
- **Fix:** Added `Usage: config.UsageConfig{Signals: "true"}` to that literal, matching the same value used in the shared `validConfig()` builder.
- **Files modified:** `internal/config/config_test.go`
- **Verification:** `go test ./internal/config/... -count=1` green.
- **Committed in:** `d439dcd` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to keep the existing test suite green after adding the new unconditional validation rule. No scope creep — file was already implicitly in scope as a consumer of `Config.Validate()`.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `config.UsageConfig`/`ENGRAM_USAGE_SIGNALS` and `telemetry.UsageQueueMetrics` are ready contracts for Plan 05 (usage queue engine) and Plan 06 (wiring into `serve.go`/`buildDepsFromEnv`).
- No blockers: `go build ./...`, `go vet ./internal/telemetry/...`, `golangci-lint run ./...`, and `go test ./...` are all green.
- `task lint:markdown` reports pre-existing issues in unrelated `.planning/` docs (phases 09/10/11 and `12-01-SUMMARY.md`) — out of scope per the SCOPE BOUNDARY rule; not fixed here.

---
*Phase: 12-per-memory-usage-signals*
*Completed: 2026-07-10*

## Self-Check: PASSED
