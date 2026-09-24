---
phase: 02-decision-interface-jev-backend
plan: 03
subsystem: api
tags: [decide, jev, openrouter, decisions-api, config, otel]

# Dependency graph
requires:
  - phase: 02-decision-interface-jev-backend
    provides: 02-02's D-06 resolution record (reject-hand-write) in 02-SDK-EVALUATION.md
provides:
  - internal/decide (Decider contract, noul question type only)
  - internal/decide/jev (hand-written Jev backend, New+Option client, decide span)
  - internal/server/decider.go (deciderFromConfig wiring seam)
  - internal/config DecisionsConfig struct, nine ENGRAM_DECISIONS_* registry rows, gated Validate block
affects: [02-04 (choice/score questions, DecideMany), 02-05 (deps wiring, key-source logging), 02-07 (error classification, response-too-large), 02-08 (DecideMany verification)]

# Actuals (#2632)
actuals:
  tokens: 10443
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "internal/decide: pure-interface contract package (Decider/Request/State/Question/Answer/Response/Usage), zero I/O, mirrors internal/embed's public-surface conventions"
    - "internal/decide/jev: New+Option client hand-written on net/http + encoding/json (D-06 reject-hand-write), following internal/embed/internal/summarize's struct-literal-drain-defaults / post-loop-ceiling-fallback shape"
    - "decide span + one debug slog line per call, both excluding State/Instructions/criteria/API key (D-13)"
    - "config registry: decisions.* rows off by default, presence-enables (D-01), provider enum checked unconditionally, remaining fields gated on provider==jev"

key-files:
  created:
    - internal/decide/decide.go
    - internal/decide/jev/jev.go
    - internal/server/decider.go
    - internal/server/decider_test.go
    - internal/config/decisions_config_test.go
  modified:
    - internal/config/config.go
    - internal/config/registry.go
    - internal/config/validate.go

key-decisions:
  - "Built on the D-06 reject-hand-write branch recorded in 02-SDK-EVALUATION.md: internal/decide/jev is net/http + encoding/json, no github.com/OpenRouterTeam/go-sdk or spyzhov/ajson added to go.mod (verified by the plan's third verify command)"
  - "decisions.timeout default is 10s (per this plan's explicit spec), not the 30s PATTERNS.md had drafted from RESEARCH — the PLAN.md task text is authoritative and jev.go's own defaultTimeout already matches it"
  - "concurrency (like max_timeout) uses the post-loop-fallback Option convention, not the struct-literal-default convention drainBytes/drainTimeout use — it has no honored-zero semantics"

patterns-established:
  - "Provider-neutral contract package (internal/decide) with zero I/O, backends in internal/decide/<name> — the shape future providers (an emulator, DEC-F2) will follow"

requirements-completed: [DEC-01, DEC-02, DEC-03, DEC-06]

coverage:
  - id: D1
    description: "A config.Config with Decisions.Provider=jev and Decisions.BaseURL flows through deciderFromConfig, decide.Decider and the jev backend to exactly {base}/alpha/decisions and back as a typed noul answer with usage"
    requirement: "DEC-02"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestDeciderTracerEndToEnd"
        status: pass
    human_judgment: false
  - id: D2
    description: "The API key resolves at the wiring seam via cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey) — empty Decisions.APIKey falls back to the OpenAI key, a set one wins"
    requirement: "DEC-03"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestDeciderTracerEndToEnd (Authorization=Bearer fallback-key) and its own_key_wins subtest"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every Decide call emits one decide span with the eight D-13 attributes (provider, model, questions, model_snapshot, input_tokens, output_tokens, cost_usd, status) and one debug slog line excluding sensitive fields"
    requirement: "DEC-06"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestDeciderTracerEndToEnd (span attribute assertions)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The nine ENGRAM_DECISIONS_* vars are registered exactly once each with no Legacy/Flag and the specified defaults, and round-trip through config.Load(nil); Config.Validate checks the provider enum unconditionally and every other decisions field only when provider=jev, with the base URL never falling back to ENGRAM_OPENAI_BASE_URL"
    requirement: "DEC-01"
    verification:
      - kind: unit
        ref: "internal/config/decisions_config_test.go#TestDecisionsRegistryEntries"
        status: pass
      - kind: unit
        ref: "internal/config/decisions_config_test.go#TestDecisionsValidate"
        status: pass
    human_judgment: false
  - id: D5
    description: "internal/decide/jev is hand-written on net/http + encoding/json per the D-06 reject-hand-write resolution — engram's go.mod carries no github.com/OpenRouterTeam/go-sdk dependency"
    requirement: "DEC-06"
    verification:
      - kind: other
        ref: "rg -o -F 'github.com/OpenRouterTeam/go-sdk' go.mod | wc -l (prints 0)"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-09-23
status: complete
---

# Phase 2 Plan 3: Decision Interface & Jev Backend — Tracer + Config Enablement Summary

**A `config.Config` with `Decisions.Provider=jev` flows end to end through `deciderFromConfig` and a hand-written Jev client to `{base}/alpha/decisions`, returning a typed noul answer with usage and a fully-attributed `decide` OTel span; the nine `ENGRAM_DECISIONS_*` vars now register and validate, off by default.**

## Performance

- **Duration:** ~20 min
- **Tasks:** 2 completed
- **Files modified:** 8 (5 created, 3 modified)

## Accomplishments

- `internal/decide`: a pure, zero-I/O provider-neutral `Decider` contract (`Request`/`State`/`Question`/`Answer`/`Response`/`Usage`), noul question type only — choice/score land in plan 02-04
- `internal/decide/jev`: hand-written Jev backend (D-06 `reject-hand-write`) over `net/http` + `encoding/json`, `New`+`Option` client mirroring `internal/embed`/`internal/summarize`, pinned `DefaultModel = "typesafe/jev-1.13"`, a `decide` OTel span with all eight D-13 attributes plus one debug `slog` line, neither carrying `State`, `Instructions`, criteria, or the API key
- `internal/server/decider.go`: `deciderFromConfig` wiring seam — empty provider constructs nothing (D-01), `jev` builds the client with the key-only fallback (`cmp.Or(cfg.Decisions.APIKey, cfg.OpenAI.APIKey)`, D-03), any other value is a configuration error naming `ENGRAM_DECISIONS_PROVIDER`
- `internal/config`: `DecisionsConfig` struct, nine `ENGRAM_DECISIONS_*` registry rows (no Legacy, no Flag), and a `Config.Validate` block that checks the provider enum unconditionally and gates every other decisions field on `provider=="jev"` — a deployment that never sets the provider validates byte-identically to before this plan
- `TestDeciderTracerEndToEnd` (httptest `/api/alpha/decisions`, `tracetest.NewSpanRecorder`) and `TestDecisionsRegistryEntries`/`TestDecisionsValidate` (RED then GREEN) all pass

## Task Commits

Each task was committed atomically (Task 2 followed the RED → GREEN TDD cycle, no REFACTOR needed):

1. **Task 1: End to end — a configured jev decider answers one Noul question at {base}/alpha/decisions, with a decide span** — `948a282a` (feat)
2. **Task 2 RED: add failing tests for ENGRAM_DECISIONS_* registry and Validate** — `20e5bd04` (test)
3. **Task 2 GREEN: register ENGRAM_DECISIONS_* settings, off by default** — `1ffc83a9` (feat)

**Plan metadata:** committed separately after this SUMMARY (see below).

## Files Created/Modified

- `internal/decide/decide.go` — the `Decider` interface, `Request`/`State`/`Question`/`Answer`/`Response`/`Usage` types, noul question constructor
- `internal/decide/jev/jev.go` — the Jev backend: `Client`, `Option` set, `New`, `Decide`, wire structs, `decide` span
- `internal/server/decider.go` — `deciderFromConfig` gate
- `internal/server/decider_test.go` — `TestDeciderTracerEndToEnd` (plus `own key wins` subtest)
- `internal/config/config.go` — `Config.Decisions DecisionsConfig` field and the `DecisionsConfig` struct
- `internal/config/registry.go` — nine `decisions.*` rows
- `internal/config/validate.go` — the provider-gated `Decisions.*` validation block
- `internal/config/decisions_config_test.go` — `TestDecisionsRegistryEntries`, `TestDecisionsValidate`

## Decisions Made

- Built on the D-06 `reject-hand-write` branch recorded in `02-SDK-EVALUATION.md`; verified `go.mod` carries no `github.com/OpenRouterTeam/go-sdk` (the plan's third `<verify>` command).
- `decisions.timeout`'s registry default is `10s`, per this plan's explicit task text — supersedes `02-PATTERNS.md`'s earlier `30s` draft. `jev.go`'s own `defaultTimeout` constant already matched `10s`, so no cross-file drift.
- `concurrency` (like `max_timeout`) resolves via the post-loop-fallback `Option` convention in `jev.New`, not the struct-literal-default convention `drainBytes`/`drainTimeout` use — there is no "0 means default" escape hatch for it, matching the plan's explicit instruction.

## Deviations from Plan

None — plan executed exactly as written. Both tasks' `<verify>` and `<acceptance_criteria>` blocks pass as specified; no auto-fixes were needed.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required. `ENGRAM_DECISIONS_*` remains off by default until an operator sets `ENGRAM_DECISIONS_PROVIDER=jev` (Helm/docs wiring is plan 02-06).

## Next Phase Readiness

- The tracer slice (config → wiring seam → `decide.Decider` → Jev backend → span) is proven and ready for plan 02-04 to extend with choice/score questions and `DecideMany`.
- `deciderFromConfig` exists as a standalone constructor, not yet wired into `deps` — plan 02-05 integrates it into `buildDepsFromEnv` and adds config-driven timeout/drain/concurrency options plus key-source logging.
- No blockers.

---
*Phase: 02-decision-interface-jev-backend*
*Plan: 03*
*Completed: 2026-09-23*

## Self-Check: PASSED

All created files (`internal/decide/decide.go`, `internal/decide/jev/jev.go`,
`internal/server/decider.go`, `internal/server/decider_test.go`,
`internal/config/decisions_config_test.go`) confirmed present on disk. All
three task commit hashes (`948a282a`, `20e5bd04`, `1ffc83a9`) confirmed
present in git history.
