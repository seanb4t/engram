---
phase: 20-correctness-polish
plan: 02
subsystem: embed
tags: [go, embed, config, import-cycle, refactor]

# Dependency graph
requires:
  - phase: 20-correctness-polish
    provides: 20-01's proto/discovery-fidelity slice (no functional dependency; same phase, parallel plan)
provides:
  - Single map-based request-body build in embed.Client.embed() (embedReq struct removed)
  - Single shared reserved-param-key list (config.ReservedEmbedParamKeys) consumed by both ParseEmbedParams and embed.ReservedParamKeys
affects: [internal/embed, internal/config, any future embed-body or embed-params work]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared constant lives in the lower-layer package (internal/config) with the higher-layer package (internal/embed) aliasing it, when the 'natural' direction would create an import cycle via a transitive telemetry->config edge"

key-files:
  created: []
  modified:
    - internal/embed/embed.go
    - internal/config/embedparams.go
    - internal/config/embedparams_test.go

key-decisions:
  - "Reversed the plan's stated import direction: canonical ReservedEmbedParamKeys lives in internal/config (not internal/embed) because internal/embed already imports internal/telemetry, which imports internal/config — a config->embed edge as originally planned would cycle. internal/embed.ReservedParamKeys now aliases internal/config.ReservedEmbedParamKeys, preserving the single-source-of-truth intent without the cycle."

patterns-established:
  - "embed()'s outbound body is always built via one map[string]any path: operator params merge first, then model/input are set last (authoritative), then json.Marshal once."

requirements-completed: [REQ-embed-param-key-sharing, REQ-embed-body-build-collapse]

coverage:
  - id: D1
    description: "embed.Client.embed() builds its request body via a single map-based path (embedReq struct-marshal branch removed)"
    requirement: "REQ-embed-body-build-collapse"
    verification:
      - kind: unit
        ref: "internal/embed/embed_test.go#TestEmbedParamsMergedIntoBody (all subtests, including 'no params configured produces exactly model+input')"
        status: pass
    human_judgment: false
  - id: D2
    description: "config.ParseEmbedParams and embed's wire contract share one reserved-param-key list so they cannot silently desync"
    requirement: "REQ-embed-param-key-sharing"
    verification:
      - kind: unit
        ref: "internal/config/embedparams_test.go#TestParseEmbedParams (reserved-key subtests iterate ReservedEmbedParamKeys)"
        status: pass
    human_judgment: false

# Metrics
duration: 20min
completed: 2026-07-15
status: complete
---

# Phase 20 Plan 02: Embed Cleanups Summary

**Collapsed embed.Client.embed()'s two-path body build into one map-based path and unified the reserved-param-key list between internal/config and internal/embed via a config-owned canonical slice (direction reversed from plan to avoid a real import cycle).**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-07-15T23:55:00Z
- **Tasks:** 2
- **Files modified:** 3 (internal/embed/embed.go, internal/config/embedparams.go, internal/config/embedparams_test.go)

## Accomplishments
- `embed.Client.embed()` now builds its outbound request body via a single `map[string]any` path (empty params → exactly `{model, input}`; non-empty params → merged then `model`/`input` set last, authoritative). The unused `embedReq` struct was removed.
- `config.ParseEmbedParams`'s reserved-key reject loop and `internal/embed`'s wire contract now share one canonical list — `config.ReservedEmbedParamKeys`, aliased as `embed.ReservedParamKeys` — so the two lists cannot silently desync.

## Task Commits

Each task was committed atomically:

1. **Task 1: Collapse embed() to a single map path and export ReservedParamKeys** - `f748647b` (refactor)
2. **Task 2: Have config.ParseEmbedParams consume the shared reserved-key list** - `a6eae255` (refactor)

**Plan metadata:** (this commit)

## Files Created/Modified
- `internal/embed/embed.go` - single-path map-based `embed()` body build; `embedReq` struct removed; `ReservedParamKeys` now aliases `config.ReservedEmbedParamKeys`; new import of `internal/config`
- `internal/config/embedparams.go` - new exported `ReservedEmbedParamKeys` var (canonical list, doc comment explains the cycle-avoidance direction); `ParseEmbedParams`'s reject loop now ranges over it instead of an inline literal
- `internal/config/embedparams_test.go` - reserved-key subtests range over `ReservedEmbedParamKeys` instead of a re-derived local literal

## Decisions Made
- **Import direction reversed from the plan's literal instructions.** The plan specified `internal/config` importing `internal/embed.ReservedParamKeys`. Attempting that produced a real `go build` import-cycle error: `internal/embed` → `internal/telemetry` → `internal/config` → (planned) `internal/embed`. RESEARCH.md's claim of "no import cycle" was incorrect — it verified `internal/config` imports no `internal/*` packages *today* and `internal/embed` imports only `internal/telemetry`, but missed that `internal/telemetry` itself imports `internal/config`, closing the loop the moment `internal/config` gained any edge to `internal/embed`.
  - Resolution: declared the canonical `ReservedEmbedParamKeys` slice in `internal/config` (which has zero internal package dependents), and had `internal/embed.ReservedParamKeys` alias it (`embed` already transitively depends on `config` via `telemetry`, so this direction adds no new cyclical risk). This preserves the single-source-of-truth intent (both packages consume the exact same slice; desync is now structurally impossible) while satisfying `go build`.
  - This is covered under 20-CONTEXT.md's "Claude's Discretion" grant: "#304, #302, #303 are mechanical refactors ... implemented at Claude's discretion within the decisions above." No user-facing behavior changed — the reserved keys enforced (`model`, `input`) and the error strings are identical to before.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Reversed the config↔embed import direction to avoid a real import cycle**
- **Found during:** Task 2 (config.ParseEmbedParams consuming the shared list)
- **Issue:** The plan's literal design (`internal/config` imports `internal/embed.ReservedParamKeys`) fails `go build` with an import cycle: `cmd/engram → internal/auth → internal/telemetry → internal/config → internal/embed → internal/telemetry`. RESEARCH.md's "no import cycle" verification missed that `internal/telemetry` imports `internal/config`.
- **Fix:** Declared the canonical `ReservedEmbedParamKeys` in `internal/config` instead, and had `internal/embed.ReservedParamKeys` alias it (`var ReservedParamKeys = config.ReservedEmbedParamKeys`). `internal/embed` now imports `internal/config` directly — a safe direction since `internal/config` has no internal dependents and `internal/embed` already transitively depends on it via `internal/telemetry`.
- **Files modified:** internal/embed/embed.go, internal/config/embedparams.go, internal/config/embedparams_test.go
- **Verification:** `go build ./...` clean; `go test ./...` all green; `golangci-lint run ./...` 0 issues.
- **Committed in:** a6eae255 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking — Rule 3)
**Impact on plan:** Necessary correction to a factually incorrect RESEARCH claim; no user-facing or wire-contract behavior changed. No scope creep.

## Issues Encountered
- `go build ./...` failed after the plan's literal Task 2 action (config importing embed) with an import cycle through internal/telemetry. Resolved per the deviation above before proceeding — see Decisions Made.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- #304 and #302 closed. Both success criteria (ROADMAP SC3, SC4) met: single map-based embed() body build; one shared reserved-param-key list.
- No blockers for remaining Phase 20 plans (20-01 already complete; 20-03/20-04 if scheduled are independent subsystems per 20-CONTEXT.md D-10).

---
*Phase: 20-correctness-polish*
*Completed: 2026-07-15*
