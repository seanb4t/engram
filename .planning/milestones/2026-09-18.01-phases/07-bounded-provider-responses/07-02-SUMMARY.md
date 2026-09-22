---
phase: 07-bounded-provider-responses
plan: 02
subsystem: config
tags: [config, registry, validation, provider-bounds, koanf]

# Dependency graph
requires:
  - "07-01 (the option names and constructor defaults these keys must agree with)"
provides:
  - "Six ENGRAM_ registry keys: EMBED/SUMMARY × DRAIN_BYTES, DRAIN_TIMEOUT, MAX_TIMEOUT"
  - "EmbedConfig.DrainBytes / DrainTimeout / MaxTimeout struct fields with koanf tags"
  - "SummarizeConfig.DrainBytes / DrainTimeout / MaxTimeout struct fields with koanf tags"
  - "Startup validation for all six, each error naming its own environment variable"
  - "Corrected Timeout doc comments — zero no longer promises 'no timeout'"
affects: [07-04]

# Actuals (#2632)
actuals:
  tasks: 2
  commits: 2
plan_head_before: 48a338752b8b4a0f5ac1e0b1d0ae1ba0cb3f9d8e

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Reuse of ParseNonNegativeIntCap for a 'zero valid, negative rejected' byte bound — the exact shape ENGRAM_MEMORY_MAX_SUMMARY_BYTES already uses"
    - "Two deliberately different zero-semantics in one validation block: zero ACCEPTED for a drain bound (it means skip the drain), zero REJECTED for a timeout ceiling (a zero ceiling would reintroduce the unbounded request)"
    - "Summarize-lane bounds gated on Summarize.Model, mirroring summarize.timeout — an unset model means no summarizer is ever built, so the values are inert"

key-files:
  created:
    - internal/config/providerbounds_test.go
  modified:
    - internal/config/registry.go
    - internal/config/config.go
    - internal/config/validate.go
    - internal/config/validate_test.go
    - internal/config/config_test.go
    - internal/config/service_auth_test.go

key-decisions:
  - "Defaults chosen to match 07-01's constructor defaults rather than the plan's predictions: drain_bytes 262144 (256 KiB), drain_timeout 2s, max_timeout 10m. The 256 KiB drain default deliberately equals net/http's own maxPostCloseReadBytes safety-net threshold, which 07-01 discovered while writing the byte-axis regression."
  - "The two ceilings reject zero as well as negative (d <= 0), unlike the two drain bounds which accept zero. This asymmetry is intentional and documented at both sites: zero means 'skip the drain' for a drain bound (D-05), but a zero ceiling would silently restore the unbounded request this phase exists to remove (D-08). There is deliberately no configurable 'unbounded'."
  - "validate.go's embed-timeout comment previously cited 'D-08' meaning v0.10.x Phase 13's decision (zero = infinite). Rewritten to disambiguate: this phase's own D-08 is the ceiling knob, and zero now resolves to it rather than disabling the timeout."

requirements-completed: []  # REQ-provider-drain-bounded is shared with plans 07-01/07-03/07-04/07-05 — not marked complete until every declaring plan has a SUMMARY.

coverage:
  - id: D1
    description: "Six brand-new keys exist in the registry that is the single source of truth for every ENGRAM_ variable, each with a Default and neither a Legacy nor a Flag value"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/config/providerbounds_test.go#TestProviderBoundRegistryEntries"
        status: pass
    human_judgment: false
  - id: D2
    description: "Each of the six reaches its Config struct field when its environment variable is set, and reads back as its documented default when it is not"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/config/providerbounds_test.go#TestProviderBoundRegistryEntries"
        status: pass
    human_judgment: false
  - id: D3
    description: "A negative drain bound fails startup naming its own environment variable; zero passes, because zero is the supported skip-the-drain setting"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/config/providerbounds_test.go#TestValidateProviderBounds"
        status: pass
    human_judgment: false
  - id: D4
    description: "A non-positive request-timeout ceiling fails startup naming its own environment variable — there is deliberately no way to configure an unbounded ceiling"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "internal/config/providerbounds_test.go#TestValidateProviderBounds"
        status: pass
    human_judgment: false
  - id: D5
    description: "EmbedConfig.Timeout and SummarizeConfig.Timeout doc comments no longer promise that zero disables the timeout; both say it resolves to the ceiling instead"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "rg -n 'resolves to the MaxTimeout ceiling' internal/config/config.go"
        status: pass
    human_judgment: false
  - id: D6
    description: "Every Config literal in this package that reaches Validate carries the new always-enforced embed fields, so the package's own suite still passes"
    requirement: "REQ-provider-drain-bounded"
    verification:
      - kind: unit
        ref: "go test ./internal/config/ -count=1"
        status: pass
    human_judgment: false

completed: 2026-09-21
status: complete
---

# Phase 07 Plan 02: Provider-Bound Configuration Keys Summary

**Six new `ENGRAM_` keys — per-client drain byte bounds, drain timeouts, and request-timeout ceilings — declared in the one registry that is the single source of truth, each validated at startup under its own name.**

## Task Commits

1. **Task 1: Declare the six keys and their struct fields** - `46893194` (feat)
2. **Task 2: Validate the six at startup** - `07895080` (feat)

_No TDD gate applies to this plan (`type: execute`)._

## Accomplishments

- `internal/config/registry.go` gains six entries: `embed.drain_bytes` / `embed.drain_timeout` / `embed.max_timeout` and the `summarize.*` mirror, defaults `262144` / `2s` / `10m`. None carries a `Legacy` (nothing is being retired) or a `Flag` (a provider-tuning value is never typed at a prompt).
- `EmbedConfig` and `SummarizeConfig` gain the matching `DrainBytes`/`DrainTimeout`/`MaxTimeout` fields with `koanf` tags.
- `Validate` rejects a negative drain bound and a non-positive ceiling, each error naming its own environment variable. Zero passes for the drain bounds only.
- The summarize trio is gated on `Summarize.Model`, exactly as `summarize.timeout` already is.
- Both `Timeout` doc comments and `validate.go`'s embed-timeout comment corrected: zero no longer promises "no timeout (infinite)".

## Files Created/Modified

- `internal/config/registry.go` - the six new keys with their defaults
- `internal/config/config.go` - six struct fields, two corrected `Timeout` doc comments
- `internal/config/validate.go` - six validation blocks, corrected embed-timeout comment
- `internal/config/providerbounds_test.go` - **new**; `TestProviderBoundRegistryEntries` (registry shape, env→field, defaults) and `TestValidateProviderBounds` (the zero/negative table, asserting each error names its own variable)
- `internal/config/validate_test.go`, `internal/config/config_test.go`, `internal/config/service_auth_test.go` - existing `Config` literals extended with the now always-enforced embed fields

## Recorded test evidence

- `go test ./internal/config/ -count=1` → `ok` (0.098s)
- `go build ./...` → exit 0
- `task lint` → exit 0 (golangci-lint 126 files, yamlfmt, actionlint, rumdl, ruff all clean)
- `task fmt:check` → exit 0
- `task license:check` → 2194 files checked, **invalid: 0**

## Deviations from Plan

### Auto-fixed Issues

None.

**Total deviations:** 0 auto-fixed.

## Issues Encountered

- **This plan was closed out by the orchestrator, not by its own executor.** The executor committed task 1 (`46893194`), completed task 2's edits on disk, then backgrounded its verification gate (`go test ./...`) and ended its turn awaiting a notification that could never arrive — the known stall mode in gotcha `9f0qav7xja`. The orchestrator stopped the agent, killed the two orphaned `go test` processes, verified the work, and committed task 2 as `07895080`. **No work was lost or redone**; the uncommitted diff was reviewed in full and matched the plan's must-haves before it was committed.
- **A red-evidence patch was left applied** by the killed test harness: `internal/store/spine.go` held the `EnumerateCitations` mutation (bounded `citationsView()` swapped for an unbounded `WithPayload(true)`) because `TestRedEvidencePatchesAreLive` was interrupted between apply and revert. Reverted with `git checkout --` before committing. **Anyone killing a `go test ./...` in this repo must check `git status` for a stranded patch** — the harness reverts in a defer that a SIGKILL skips.
- The blocker 07-01 raised (`07-02-PLAN.md:53`'s escaped `koanf:` key-link pattern) was resolved by the orchestrator in `48a33875` before this plan started; `internal/keylinks` is green.

## User Setup Required

None — all six keys have working defaults.

## Next Phase Readiness

- Plan 07-03 (summarize lane onto `httpdrain`) is unblocked; the `summarize.*` keys it will be wired to already exist and validate.
- Plan 07-04 consumes all six of these keys — it must parse them with the **same exported parser** `Validate` uses (`ParseNonNegativeIntCap` for the byte bounds) so the validated range and the enforced range cannot diverge.
