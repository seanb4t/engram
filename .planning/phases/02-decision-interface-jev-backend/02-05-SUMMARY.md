---
phase: 02-decision-interface-jev-backend
plan: 05
subsystem: api
tags: [decide, jev, config, wiring, otel, slog]

# Dependency graph
requires:
  - phase: 02-decision-interface-jev-backend
    provides: 02-03's deciderFromConfig gate and 02-04's widened Decider (DecideMany, valid noul requests for concurrency testing)
provides:
  - internal/server/decider.go's five decisions* resolvers (timeout, max_timeout, drain_bytes, drain_timeout, concurrency) and logDeciderEnabled
  - deciderFromConfig's jev branch now threading all six jev.Option knobs, including WithConcurrency (D-10)
  - deps.decider field, built once in buildDepsFromEnv through the single gated seam
  - DEC-01 proofs: TestDeciderFromConfigProviderUnset, TestBuildDepsFromEnvRejectsUnknownProvider, TestBuildDepsFromEnvConstructsDecider
affects: [phase 3 operator command (reaches deciderFromConfig), phase 4 search_memory reranker (first consumer of deps.decider)]

# Actuals (#2632)
actuals:
  tokens: 5784
  tasks: 2
  commits: 2
plan_head_before: 54a4a9b879af83b7591d362d9f4722d048f56861

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "decisions* resolvers mirror the summary*/embed* resolver shape exactly: struct-literal-set-before-loop honored-zero for drain bytes/timeout, post-loop non-positive-rejected fallback for max_timeout/concurrency"
    - "logDeciderEnabled: url.Parse + u.Host-only extraction to strip userinfo/path/query from an enablement log line, never the secret value itself — only which env var supplied it"

key-files:
  created: []
  modified:
    - internal/server/decider.go
    - internal/server/decider_test.go
    - internal/server/tools.go
    - internal/server/tools_test.go

key-decisions:
  - "deps.decider is wired into buildDepsFromEnv after embedder identity is computed, per the plan's explicit RESEARCH A4 resolution — the decider goes into deps now (unused field this phase) rather than staying a standalone unwired constructor, so every existing buildDepsFromEnv integration test already exercises the off path"
  - "TestBuildDepsFromEnvRejectsUnknownProvider needs no new code path to fail before any store dial: Config.Validate already rejects an unknown ENGRAM_DECISIONS_PROVIDER unconditionally (internal/config/validate.go:286-287), so loadAndValidate's existing early-return already satisfies the 'before any store dial' requirement"
  - "logDeciderEnabled fires only when dec != nil in buildDepsFromEnv (not unconditionally) — an unset provider produces neither a constructed decider nor a log line, keeping the off-by-default path silent as well as inert"

patterns-established: []

requirements-completed: [DEC-01, DEC-03]

coverage:
  - id: D1
    description: "deciderFromConfig's empty-provider branch stays the first case and constructs nothing; three back-to-back calls with a fully-populated Decisions config all return a nil Decider and nil error, with zero requests reaching a configured base URL"
    requirement: "DEC-01"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestDeciderFromConfigProviderUnset"
        status: pass
    human_judgment: false
  - id: D2
    description: "An unknown ENGRAM_DECISIONS_PROVIDER value fails deciderFromConfig and buildDepsFromEnv, naming the env var, before any store dial"
    requirement: "DEC-01"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestDeciderFromConfigUnknownProvider"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestBuildDepsFromEnvRejectsUnknownProvider"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every ENGRAM_DECISIONS_* knob (timeout, max_timeout, drain_bytes, drain_timeout, concurrency) reaches the jev client through the real wiring: a 150ms timeout bounds a call to a 2s-sleeping endpoint, and a concurrency of 2 caps DecideMany's peak in-flight requests at exactly 2 over 6 requests"
    requirement: "DEC-03"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestDeciderFromConfigAppliesOptions/timeout"
        status: pass
      - kind: unit
        ref: "internal/server/decider_test.go#TestDeciderFromConfigAppliesOptions/concurrency"
        status: pass
    human_judgment: false
  - id: D4
    description: "The five decisions* resolvers mirror the summary*/embed* fallback shape exactly: unparseable/empty input falls back to its documented default; drain bytes and drain timeout honor an explicit 0; max_timeout and concurrency reject any non-positive value"
    requirement: "DEC-03"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestDeciderResolverDefaults"
        status: pass
    human_judgment: false
  - id: D5
    description: "The registry's ENGRAM_DECISIONS_MODEL default equals jev.DefaultModel, pinned by a test so the two literals cannot drift"
    requirement: "DEC-03"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestDecisionsModelDefaultMatchesJev"
        status: pass
    human_judgment: false
  - id: D6
    description: "Enabling decisions logs one 'typed decisions enabled' Info line naming provider, model, endpoint_host (host only — never userinfo, path or query) and api_key_source (ENGRAM_DECISIONS_API_KEY, ENGRAM_OPENAI_API_KEY, or none); the key value never appears in the log output"
    requirement: "DEC-01"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestDeciderEnabledLogLine"
        status: pass
    human_judgment: false
  - id: D7
    description: "deps.decider is built exactly once in buildDepsFromEnv through deciderFromConfig, nil when the provider is unset and non-nil when it is jev, and constructing an enabled decider makes zero outbound calls at startup (no probe or health request)"
    requirement: "DEC-01"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestBuildDepsFromEnvLoadsConfigOnce (d.decider == nil assertion)"
        status: pass
      - kind: unit
        ref: "internal/server/tools_test.go#TestBuildDepsFromEnvConstructsDecider"
        status: pass
    human_judgment: false

duration: ~11min
completed: 2026-09-23
status: complete
---

# Phase 2 Plan 5: Wire the Decider into Startup — Every Knob, Off by Default Summary

**Every `ENGRAM_DECISIONS_*` knob (timeout, max_timeout, drain bytes/timeout, concurrency) now reaches the jev client through `deciderFromConfig`, `deps.decider` is built once in `buildDepsFromEnv` through that single gated seam, and enabling the feature logs one line naming the key's source — never its value.**

## Performance

- **Duration:** ~11 min (commit-span: 2026-09-23T11:38:51-04:00 – 2026-09-23T11:41:28-04:00)
- **Tasks:** 2 completed
- **Files modified:** 4 (0 created, 4 modified)

## Accomplishments

- `internal/server/decider.go`: five `decisions*` resolvers (`decisionsTimeout`, `decisionsMaxTimeout`, `decisionsDrainBytes`, `decisionsDrainTimeout`, `decisionsConcurrency`) mirroring the `summary*`/`embed*` resolver shape exactly — struct-literal-set-before-loop honored-zero for the drain pair, post-loop non-positive-rejected fallback for `max_timeout`/`concurrency`
- `deciderFromConfig`'s `jev` branch now passes all six `jev.Option` knobs (`WithHTTPTransport`, `WithTimeout`, `WithMaxTimeout`, `WithDrainBytes`, `WithDrainTimeout`, `WithConcurrency`) — the empty-provider case stays the first branch, unchanged
- `logDeciderEnabled`: logs `provider`, `model`, `endpoint_host` (the base URL's host only, via `url.Parse` + `u.Host`) and `api_key_source` (`ENGRAM_DECISIONS_API_KEY` / `ENGRAM_OPENAI_API_KEY` / `none`) — proven by test to never leak the base URL's userinfo/path/query or the key's value
- `deps.decider decide.Decider` field added (doc-commented per D-01: nil unless the provider is set, no caller added this phase, Phase 4's reranker is the first consumer); `buildDepsFromEnv` builds it once through `deciderFromConfig` and logs enablement only when a decider was actually constructed
- Seven `internal/server` decider tests plus three `deps`-level tests, all RED-verified (`go vet` compile failures against the pre-task contract) then GREEN, race-clean, `golangci-lint` clean

## Task Commits

Both tasks followed RED → GREEN (implementation added after the test file failed to compile), each as a single commit combining the RED tests and the GREEN implementation, per this plan's TDD instruction:

1. **Task 1: Thread every `ENGRAM_DECISIONS_*` knob into the jev decider; log enablement without the key** — `27968de1` (feat)
2. **Task 2: Build `deps.decider` at startup through the one gated seam** — `344ba149` (feat)

**Plan metadata:** committed separately after this SUMMARY (see below).

## Files Created/Modified

- `internal/server/decider.go` — five `decisions*` resolvers, `logDeciderEnabled`, `deciderFromConfig`'s jev branch now passes all six options
- `internal/server/decider_test.go` — `TestDeciderFromConfigProviderUnset`, `TestDeciderFromConfigUnknownProvider`, `TestDeciderFromConfigAppliesOptions` (timeout, concurrency subtests), `TestDeciderResolverDefaults`, `TestDecisionsModelDefaultMatchesJev`, `TestDeciderEnabledLogLine`
- `internal/server/tools.go` — `deps.decider` field, `buildDepsFromEnv`'s `deciderFromConfig` call plus conditional `logDeciderEnabled`
- `internal/server/tools_test.go` — `TestBuildDepsFromEnvRejectsUnknownProvider` (new), `TestBuildDepsFromEnvConstructsDecider` (new), `TestBuildDepsFromEnvLoadsConfigOnce` (extended with the `d.decider == nil` assertion and `ENGRAM_DECISIONS_PROVIDER` isolation)

## Decisions Made

- `deps.decider` is wired into `buildDepsFromEnv` now, as an unused-this-phase field, per the plan's explicit RESEARCH A4 resolution — not left as a standalone unwired constructor. Every existing `buildDepsFromEnv` integration test (`TestBuildDepsFromEnvLoadsConfigOnce`) already exercises the off path as a result.
- `TestBuildDepsFromEnvRejectsUnknownProvider` needed no new validation code: `Config.Validate` already rejects an unknown `ENGRAM_DECISIONS_PROVIDER` unconditionally (`internal/config/validate.go:286-287`), so `loadAndValidate`'s existing early return in `buildDepsFromEnv` already satisfies "before any store dial" — confirmed by direct read before writing the test, not assumed.
- `logDeciderEnabled` is called only when `dec != nil` (not unconditionally) inside `buildDepsFromEnv`, so an unset provider stays silent as well as inert — no log line, no decider, no request.

## Deviations from Plan

None — plan executed exactly as written. Both tasks' `<verify>` and `<acceptance_criteria>` blocks pass as specified. One note on the plan's own verify scripts: the `rg -o -e '^--- PASS: Test(...)$'` patterns in both tasks' `<automated>` verify commands anchor with a trailing `$`, but `go test -v` PASS lines carry a duration suffix (e.g. `--- PASS: TestDeciderFromConfigProviderUnset (0.00s)`), so the literal regex matches zero lines regardless of pass/fail — the same class of verify-script bug plan 02-04's SUMMARY documented. Not treated as a fresh deviation (PLAN.md untouched); all named tests were independently confirmed PASSing by direct inspection of `go test -v` output (see verification below).

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. `ENGRAM_DECISIONS_PROVIDER` remains off by default; this plan adds no new registry rows or Helm/docs surface (those landed in 02-03/02-06).

## Verification

- `go test ./internal/server/ -run '^(TestDeciderFromConfig|TestDeciderResolverDefaults|TestDecisionsModelDefaultMatchesJev|TestDeciderEnabledLogLine|TestDeciderTracerEndToEnd)' -count=1 -race -v`: all 7 named top-level tests PASS.
- `go test ./internal/server/ -run '^TestBuildDepsFromEnvRejectsUnknownProvider$' -count=1 -v`: PASS.
- `go test ./internal/server/ -run '^TestBuildDepsFromEnv(LoadsConfigOnce|ConstructsDecider)$' -count=1 -v`: both PASS (Qdrant-gated tests **ran** locally against the Testcontainers-managed Qdrant instance; neither SKIPped).
- `go test ./internal/server/ -count=1`: full package green (no regressions).
- `golangci-lint run ./internal/server/...`: 0 issues.
- Acceptance-criteria greps: `case "":` is `deciderFromConfig`'s first branch; exactly 6 distinct `jev.With*` options passed; `logDeciderEnabled` carries exactly 4 attribute keys; `decider: dec,` and `deciderFromConfig(cfg)` each appear exactly once in `tools.go`; zero `.decider.Decide` callers added anywhere in `internal/server` or `cmd`; `d.decider != nil` assertion present in `tools_test.go`.
- `go build ./...` and `go vet ./...`: clean (the one pre-existing `go vet` finding in `cmd/engram/operator_view_test.go` is unrelated to this plan's files and untouched by this plan).
- `go.mod`/`go.sum`: untouched — no new dependency added (context_note's `reject-hand-write` constraint honored).

## Next Phase Readiness

- `deps.decider` now exists as a startup-built field, off unless `ENGRAM_DECISIONS_PROVIDER` is set, reachable via the single `deciderFromConfig` seam Phase 3's operator command and Phase 4's `search_memory` reranker will both call into.
- Every `ENGRAM_DECISIONS_*` config knob is proven to take effect end to end (not just passed syntactically): the timeout bounds a real stalled call, and the concurrency setting caps `DecideMany`'s real peak in-flight count.
- Requirements DEC-01 and DEC-03 are the last plans declaring those IDs in this phase's plan set alongside 02-03/02-06/02-08 — `requirements mark-complete` is invoked below; if the shared-ID gate still reports not-ready (e.g. 02-08 has not yet completed for DEC-03), REQUIREMENTS.md will remain Pending until that plan finishes, per the existing #2388 gate.
- No blockers. Plan 02-07 (retry/classification) is declared order-independent from this plan per the dispatch context note.

---
*Phase: 02-decision-interface-jev-backend*
*Plan: 05*
*Completed: 2026-09-23*

## Self-Check: PASSED

All modified files (`internal/server/decider.go`, `internal/server/decider_test.go`,
`internal/server/tools.go`, `internal/server/tools_test.go`) confirmed present
on disk. Both task commit hashes (`27968de1`, `344ba149`) confirmed present in
`git log --oneline --all`.
