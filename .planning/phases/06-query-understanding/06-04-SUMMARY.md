---
phase: 06-query-understanding
plan: 04
subsystem: search
tags: [config-validation, startup-disclosure, jev, query-understanding, slog]

# Dependency graph
requires:
  - phase: 06-query-understanding
    provides: "06-01: understandingEnabled/understandingTimeout resolvers, understandDecider, SearchConfig.Understanding/UnderstandingTimeout/UnderstandingAudit fields and registry rows"
provides:
  - "logUnderstandingEnabled (D-15): the startup Warn disclosing console query-text egress when understanding is on, naming source, host, model, timeout, api_key_source, disable_with"
  - "Config.Validate coverage for ENGRAM_SEARCH_UNDERSTANDING / _TIMEOUT / _AUDIT (D-01/D-01a/D-16), with an independent understandingOn copy of internal/server's resolution"
  - "understandingAudit / logUnderstandingAuditEnabled (D-16 config half): the audit flag's own resolver and its two-line startup disclosure, independent of the rerank audit flag"
affects: [06-05-audit-telemetry, 06-06-vendoring]

# Actuals (#2632)
actuals:
  tokens: 9105
  tasks: 3
  commits: 3
  plan_head_before: 036e25a293254dc05112f81a65703b799cda369c
  plan_head_after: 2242f937fe7f94fb8975ca5c51eb7ba846b4bafe

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Startup disclosure Warns are structural copies of logSearchRerankAuditEnabled/logSearchRankerEnabled: host-only via url.Parse(...).Host, api_key_source names the winning env var (never a value), never the secret itself"
    - "Config.Validate keeps an independent copy of internal/server's on/off resolution switch (understandingOn mirrors understandingEnabled), commented as kept in sync by hand rather than sharing code across the config/server package boundary"
    - "Feature-specific audit flags are structurally identical but read only their own SearchConfig field — never cross-consult a sibling audit flag"

key-files:
  created:
    - internal/config/understanding_config_test.go
  modified:
    - internal/server/decider.go
    - internal/server/decider_test.go
    - internal/server/tools.go
    - internal/server/tools_test.go
    - internal/config/validate.go
    - internal/config/validate_test.go
    - internal/config/config_test.go
    - internal/config/service_auth_test.go
    - internal/config/search_config_test.go

key-decisions:
  - "Tracer feedback gate (Task 1, #3299): HUMAN_VERIFY_MODE defaulted to end-of-phase and the tracer's <verify> carried only <automated> — re-ran the tracer verify (3 top-level PASS, 4 subtest PASS), confirmed green, and proceeded straight to Task 2 expansion with no checkpoint (row 3 of the precedence chain)."
  - "Every SearchConfig{} test literal (8 sites across 4 files) gained UnderstandingTimeout: \"2s\", UnderstandingAudit: \"false\" per rule s780vae1vr, since the new unconditional audit-boolean check makes any hand-built literal's empty-string zero value fail Validate()."

requirements-completed: []
# NLQ-01 and NLQ-04 are shared across sibling plans still in flight in this
# phase (06-06 for NLQ-01; 06-05 and 06-06 for NLQ-04) — the shared-ID gate
# (#2388) computed via `requirements.ready-ids` returned 0/2 ready, so
# neither is marked complete here. Both will flip once the last declaring
# plan finishes.

coverage:
  - id: D1
    description: "D-15 startup disclosure: every startup where query understanding is on (default-following provider, or explicit) Warns the endpoint host, source, model, timeout, api_key_source and the off switch, before serving; absent when off or when no provider is configured"
    requirement: "NLQ-01"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider"
        status: pass
      - kind: unit
        ref: "internal/server/decider_test.go#TestUnderstandingEnabledLogLine"
        status: pass
    human_judgment: false
  - id: D2
    description: "Config.Validate rejects a malformed ENGRAM_SEARCH_UNDERSTANDING* value, jev-without-provider, and a bad timeout while effectively on; a no-provider deployment validates byte-identically to before this plan"
    requirement: "NLQ-01"
    verification:
      - kind: unit
        ref: "internal/config/understanding_config_test.go#TestUnderstandingConfigValidate"
        status: pass
      - kind: unit
        ref: "internal/config/understanding_config_test.go#TestUnderstandingRegistryEntries"
        status: pass
    human_judgment: false
  - id: D3
    description: "The audit flag (D-16) is parsed independently of the rerank audit flag and discloses at startup whether it actually records anything"
    requirement: "NLQ-04"
    verification:
      - kind: unit
        ref: "internal/server/decider_test.go#TestUnderstandingAuditResolver"
        status: pass
      - kind: unit
        ref: "internal/server/decider_test.go#TestUnderstandingAuditEnabledLogLine"
        status: pass
      - kind: integration
        ref: "internal/server/tools_test.go#TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider/audit_while_off,audit_on"
        status: pass

duration: 55min
completed: 2026-09-28
status: complete
---

# Phase 6 Plan 4: Query Understanding Startup Disclosure & Validation Summary

**D-01's startup Warn (endpoint host, source, timeout, key-source), Config.Validate for the three ENGRAM_SEARCH_UNDERSTANDING* keys, and the independent audit-flag resolver, all proven against a counting decisions server.**

## Performance

- **Duration:** 55 min
- **Started:** 2026-09-28T10:09:00Z
- **Completed:** 2026-09-28T11:04:00Z
- **Tasks:** 3 completed
- **Files modified:** 9 (1 created, 8 modified)

## Accomplishments

- `logUnderstandingEnabled` Warns at every startup where console query text will leave the deployment, naming the endpoint host, whether it's on by default or explicit, the model, the per-query timeout, which env var supplied the API key, and the disable switch — proven for default, explicit, off and no-provider through `buildDepsFromEnv` against a counting `httptest` decisions server (zero requests reach it in every case).
- `Config.Validate` now rejects a malformed `ENGRAM_SEARCH_UNDERSTANDING` value (exact-literal enum, no case-folding or trimming), `jev` without a configured provider (naming both variables), and a bad `ENGRAM_SEARCH_UNDERSTANDING_TIMEOUT` whenever understanding is *effectively* on — explicit `jev`, or unset with the provider set to `jev` — while a no-provider deployment validates byte-identically to before this plan. All eight `SearchConfig{}` test literals across four files were updated to carry the two new fields.
- `understandingAudit`/`logUnderstandingAuditEnabled` give the audit flag its own resolver and startup disclosure, completely independent of the existing rerank-audit flag in both directions — an operator who sets the audit flag while understanding is off is told at startup that nothing is actually recorded.

## Task Commits

Each task was committed atomically:

1. **Task 1: From the environment to the startup disclosure (tracer)** - `675df917` (feat)
2. **Task 2: Config.Validate for D-01, D-01a and D-16** - `e5b4b147` (feat)
3. **Task 3: The audit flag's resolver and startup Warn** - `2242f937` (feat)

_This plan's metadata is committed separately per worktree-mode convention (STATE.md/ROADMAP.md excluded; orchestrator updates those centrally after the wave)._

## RED Evidence (per-task, uncommitted, reverted before the real commit)

- **Task 1:** Temporarily removed the `if udec != nil { logUnderstandingEnabled(...) }` block from `buildDepsFromEnv` (`internal/server/tools.go`). Re-ran the target tests: `TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider/default` and `/explicit` failed on "got 0 'search understanding enabled' records, want exactly 1" (the intentional, assertion-level RED for the target behavior — `off`/`no_provider` correctly still passed). Restored, re-ran GREEN, then committed.
- **Task 2:** The new `understanding_config_test.go` was written before touching `Config.Validate`; running it (and the full `internal/config` suite) went RED across every SearchConfig-literal-using test with `ENGRAM_SEARCH_UNDERSTANDING_AUDIT "": must be a boolean` errors the moment the unconditional audit check was added, confirming the RED→literal-fix dependency the plan calls out (`s780vae1vr`). Fixed by adding `Config.Validate`'s new block and updating all eight literals; re-ran GREEN.
- **Task 3:** Temporarily made `understandingAudit` an unconditional `return false` and removed the `if understandingAudit(cfg) { logUnderstandingAuditEnabled(...) }` wiring from `buildDepsFromEnv`. Re-ran the target tests: `TestUnderstandingAuditResolver/warns_only_on_a_non-empty_non-boolean_value` and both `TestBuildDepsFromEnvUnderstandingDefaultFollowsProvider/audit_while_off` and `/audit_on` failed on the expected assertions. Restored, re-ran GREEN, then committed.

## Files Created/Modified

- `internal/config/understanding_config_test.go` - New: registry-entry and full validation-table tests for the three understanding keys
- `internal/config/validate.go` - Adds the ENGRAM_SEARCH_UNDERSTANDING enum/jev-provider/timeout/audit checks
- `internal/config/validate_test.go`, `internal/config/config_test.go`, `internal/config/service_auth_test.go`, `internal/config/search_config_test.go` - All `SearchConfig{}` literals updated with the two new fields
- `internal/server/decider.go` - Adds `logUnderstandingEnabled`, `understandingAudit`, `logUnderstandingAuditEnabled`
- `internal/server/tools.go` - `buildDepsFromEnv` wires both new disclosures
- `internal/server/decider_test.go`, `internal/server/tools_test.go` - Test coverage for all three deliverables

## Decisions Made

- Tracer feedback gate resolved without a checkpoint (row 3 of the #3299 precedence chain: interactive, `end-of-phase` default, automated-only `<verify>`) — re-ran the tracer's verify, it passed, execution proceeded straight into Task 2's expansion.
- Applied rule `s780vae1vr` mechanically: every hand-built `SearchConfig{}` literal (8 sites, 4 files) updated in the same change that added the unconditional audit-boolean check, rather than leaving any literal to fail later.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. A pre-existing, unrelated `go vet` finding (`cmd/engram/operator_view_test.go:441`, duplicate JSON tag in a deliberate test fixture) is out of scope for this plan's files and already tracked as acknowledged in STATE.md.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- D-15's disclosure and D-01/D-01a/D-16's validation are both fully live; plan 06-05 (audit telemetry) can now rely on `understandingAudit`/`logUnderstandingAuditEnabled` existing and being independently wired.
- NLQ-01 and NLQ-04 remain open in REQUIREMENTS.md pending plans 06-05/06-06 (shared-ID gate, #2388) — no action needed here, they will close automatically once those plans' SUMMARYs land.

---
*Phase: 06-query-understanding*
*Completed: 2026-09-28*
