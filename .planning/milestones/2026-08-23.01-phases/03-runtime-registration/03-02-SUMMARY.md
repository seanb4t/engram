---
phase: 03-runtime-registration
plan: 02
subsystem: setup
tags: [mcp-registration, claude-code, os-exec, idempotency, tolerance, argv-security]

requires:
  - phase: 03-runtime-registration
    provides: "Action.Args/Action.Tolerant, Plan.Probe, Environment.Run seam, setup.Preview()/Apply() shared executor, codex.go reference shape (plan 03-01)"
provides:
  - "claude-code's Plan() authors a two-action tolerant-remove-then-fatal-add write sequence for every auth mode (D-08/D-09), making OutcomeAlreadyCorrect reachable for the one runtime whose mcp add refuses on an existing name"
  - "the shared executor (apply.go) surfaces every Tolerant action's Description on Result.Notes regardless of that action's own exit code — a general behavior, not a claude-code special case — so a failed row can carry the destructive-window warning without apply.go knowing claude-code by name"
  - "claude-code's bearer mode names ENGRAM_TOKEN via a ${...} shell-style variable reference (D-05/D-06), replacing bearerProvenance's path-provenance placeholder for this runtime"
  - "internal/surfaces/toolclass.go's setup row comment corrected to the live-verified per-runtime add-on-existing behavior"
  - "TestActionToleranceIsAuthoredNotPositional, proven (by a temporary, reverted experiment) to fail against a reintroduced positional tolerance rule"
affects: [03-04-generic-portable-config, 03-05-remaining-wave, 06-docs-site]

actuals:
  tokens: 11119
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Executor-level Notes accumulation keyed on Action.Tolerant alone (never on outcome or runtime identity): a tolerant action's Description is surfaced on Notes whether it succeeded or its own nonzero exit was tolerated, so a runtime's own authored warning text can ride a general executor mechanism to a failed row's diagnostics"
    - "Remove-then-add as the general shape for a runtime whose add cannot be made idempotent without a force flag or a forbidden config read — codex/opencode need none of it (their add already overwrites); this is per-runtime Plan() composition, not a change to the shared executor's rules"

key-files:
  created:
    - internal/setup/claudecode_test.go
  modified:
    - internal/setup/claudecode.go
    - internal/setup/apply.go
    - internal/setup/apply_test.go
    - internal/setup/plan.go
    - internal/setup/plan_test.go
    - internal/surfaces/toolclass.go
    - cmd/engram/setup_test.go

key-decisions:
  - "Task 1 checkpoint (resolved by the user, 2026-09-09): remove-then-add for claude-code's write sequence, explicitly accepting the destructive window a failed/interrupted add after a successful remove opens (engram cannot restore what it never read); Action.Tolerant kept as an authored-per-action bool rather than 03-RESEARCH.md's recommended action-position rule, because Plan.Actions is documented as growable by Phase 4's skills distribution and a positional rule would silently make a later, unrelated action tolerant; and the toolclass.go comment correction, leaving Class values untouched. Full reasoning is in 03-02-PLAN.md Task 1's checkpoint body — not re-litigated here."
  - "Rule 3 (blocking) deviation, found while closing out Task 3: plan.go's bearerProvenance lost its last caller once this plan and 03-03 both moved their runtimes off it, tripping golangci-lint's unused check and failing `task`. 03-02-PLAN.md's own Task 2 acceptance criterion forbids deleting the function (03-04's generic pseudo-runtime still needs it), so a justified //nolint:unused was added with an expanded doc comment explaining the dead-until-03-04 window, rather than deleting the function or leaving `task` red."
  - "Rule 1 (bug/pre-existing-test) deviation: three pre-existing tests (TestPlanBearerRedactsCredentialByProvenance, TestPlanBearerNeverReadsTokenFile, TestSetupBearerTokenFileRedactedInOutput) and one JSON-shape test (TestSetupPreviewJSONHasClaudeCodeCommand) pinned claude-code's now-retired single-action Command string and provenance-placeholder bearer form. Updated all four to assert the corrected two-action sequence and ENGRAM_TOKEN-naming form; TestPlanBearerEmptyTokenFileNamesEnvVar (which only made sense against the retired provenance path) was replaced by TestPlanClaudeCodeBearerIgnoresTokenFile, proving Options.TokenFile no longer affects claude-code's bearer output at all."

requirements-completed: [REQ-register-claude-code, REQ-setup-idempotent, REQ-register-auth-modes]

coverage:
  - id: D1
    description: "engram setup --apply --runtime claude-code converges: wrote on a first run, already-correct on an identical second run, via the tolerant-remove-then-fatal-add sequence"
    requirement: REQ-setup-idempotent
    verification:
      - kind: unit
        ref: "internal/setup/apply_test.go#TestApplyConvergesClaudeCode"
        status: pass
      - kind: unit
        ref: "internal/setup/apply_test.go#TestApplyToleratesClearSlotFailure"
        status: pass
      - kind: unit
        ref: "internal/setup/apply_test.go#TestApplyFailsWhenRegistrationActionFails"
        status: pass
    human_judgment: false
  - id: D2
    description: "claude-code's Plan() authors the checkpoint-approved two-action sequence for every auth mode (oauth, oauth-client, bearer, none), and the bearer form names ENGRAM_TOKEN via a ${...} reference with no credential value or token-file path reaching argv"
    requirement: REQ-register-claude-code
    verification:
      - kind: unit
        ref: "internal/setup/claudecode_test.go#TestClaudeCodePlan"
        status: pass
      - kind: unit
        ref: "internal/setup/claudecode_test.go#TestClaudeCodeBearerHeaderIsAnEnvVarReference"
        status: pass
      - kind: unit
        ref: "internal/setup/plan_test.go#TestNoSecretInArgs"
        status: pass
    human_judgment: false
  - id: D3
    description: "an action's failure tolerance is read only from its own authored Action.Tolerant field, never from its position in Plan.Actions — verified to actually catch a reintroduced positional rule, not merely pass against the current implementation"
    requirement: REQ-register-auth-modes
    verification:
      - kind: unit
        ref: "internal/setup/apply_test.go#TestActionToleranceIsAuthoredNotPositional"
        status: pass
    human_judgment: false
  - id: D4
    description: "internal/surfaces/toolclass.go's setup row comment states the real, live-verified per-runtime add-on-existing behavior rather than the falsified uniform-overwrite claim, while leaving the Class{...} literal untouched"
    verification:
      - kind: other
        ref: "git diff internal/surfaces/toolclass.go shows comment-only changes (no Class{...} literal change)"
        status: pass
    human_judgment: true
    rationale: "The Class literal being untouched is mechanically checked; the comment's prose accuracy (whether it correctly states each runtime's live-verified behavior) is a judgment call no test can make."

duration: 15min
completed: 2026-09-09
status: complete
---

# Phase 3 Plan 2: Claude Code registration — the tolerant-remove-then-fatal-add correction Summary

**`claude mcp add`'s refuse-on-existing behavior (no force flag, live-verified at both `--scope project` and `--scope user`) is corrected with a tolerant `claude mcp remove` clearing the slot before a fatal `claude mcp add`, making `OutcomeAlreadyCorrect` reachable for claude-code for the first time, with the destructive window explicitly accepted and surfaced via a general (not claude-code-specific) executor Notes mechanism.**

## Performance

- **Duration:** 15 min (this continuation session; the Task 1 checkpoint's human-review pause is not counted)
- **Started:** 2026-09-09T11:35:00Z
- **Completed:** 2026-09-09T11:49:03Z
- **Tasks:** 3 (Task 1 checkpoint resolved by the user before this continuation; Tasks 2-3 executed here)
- **Files modified:** 8 (1 created, 7 modified)

## Accomplishments
- claude-code's `Plan()` now authors two `Action`s for every auth mode (`oauth`, `none`, `oauth-client`, `bearer`): a blanket-tolerant `claude mcp remove engram --scope user` followed by the fatal `claude mcp add`, plus a `Plan.Probe` of `claude mcp get engram` (D-09) — none of which existed before this plan.
- claude-code's bearer mode now names `ENGRAM_TOKEN` via a `${ENGRAM_TOKEN}` shell-style variable reference in its `--header` value (D-05/D-06), live-verified end-to-end in `03-RESEARCH.md` against claude 2.1.265, replacing `bearerProvenance`'s path-placeholder form for this runtime.
- The shared executor (`apply.go`) now surfaces every `Tolerant` action's `Description` on `Result.Notes` regardless of that action's own exit code — a general mechanism any runtime's `Plan()` can use, not a claude-code special case — which is how an operator learns from a *failed* row that a preceding tolerant action already cleared the prior registration.
- `internal/surfaces/toolclass.go`'s `setup` row comment corrected from a falsified "all three CLIs overwrite" claim to the live-verified per-runtime reality, with the `Class{...}` literal itself untouched (confirmed by `git diff`).
- `TestActionToleranceIsAuthoredNotPositional` added and demonstrated — by a temporary, reverted patch to `execute()` implementing the rejected positional rule — to actually go RED against a reintroduced position-derived tolerance rule, not merely pass against the current implementation.

## Task Commits

Each task was committed atomically:

1. **Task 1: DECISION checkpoint** — no commit (checkpoint task; resolved by the user's three-part answer, carried forward into Tasks 2-3 per this plan's `<checkpoint_resolution>` handoff)
2. **Task 2: claude-code's two-action write sequence, probe, and env-var-reference bearer form** — `9093de58` (test, RED) then `a461510e` (feat, GREEN)
3. **Task 3: correct the falsified classification premise and pin tolerance as authored, not positional** — `7e30ae76` (test)

**Plan metadata:** committed separately per this workflow's final-commit step.

_Note: Task 2 was `tdd="true"` and followed the canonical RED→GREEN sequence — `9093de58` was verified to fail against the pre-existing single-action claudecode.go before `a461510e` landed the implementation. Task 3 was not TDD-flagged and lands as a single commit, but its own test (TestActionToleranceIsAuthoredNotPositional) was independently verified to fail against a reintroduced positional rule via a temporary, reverted experiment (see Deviations)._

## Files Created/Modified
- `internal/setup/claudecode.go` — `Plan()` rewritten to the two-action `Args`/`Tolerant`/`Probe` model; bearer header fixed to `${ENGRAM_TOKEN}`
- `internal/setup/claudecode_test.go` — `TestClaudeCodePlan`, `TestClaudeCodeBearerHeaderIsAnEnvVarReference`
- `internal/setup/apply.go` — `toleratedNote` helper; executor now records every tolerant action's `Description` on `Notes`
- `internal/setup/apply_test.go` — `TestApplyConvergesClaudeCode`, `TestApplyToleratesClearSlotFailure`, `TestApplyFailsWhenRegistrationActionFails`, `TestActionToleranceIsAuthoredNotPositional`
- `internal/setup/plan.go` — `Result.Notes` doc comment expanded; `bearerProvenance` doc comment updated and given a justified `//nolint:unused`
- `internal/setup/plan_test.go` — `TestPlanBearerRedactsCredentialByProvenance` and `TestPlanBearerNeverReadsTokenFile` updated to the corrected claude-code form; `TestPlanBearerEmptyTokenFileNamesEnvVar` replaced by `TestPlanClaudeCodeBearerIgnoresTokenFile`
- `internal/surfaces/toolclass.go` — `setup` row comment corrected to the real per-runtime behavior; `Class{...}` unchanged
- `cmd/engram/setup_test.go` — `TestSetupPreviewJSONHasClaudeCodeCommand` and `TestSetupBearerTokenFileRedactedInOutput` updated to the two-action Command string and ENGRAM_TOKEN-naming form

## Decisions Made

See `key-decisions` in frontmatter for the Task 1 checkpoint's three-part resolution (remove-then-add with the destructive window explicitly accepted; `Action.Tolerant` authored-not-positional; the toolclass.go comment correction) and the two deviations below.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Four pre-existing tests pinned claude-code's now-retired single-action Command and provenance bearer form**
- **Found during:** Task 2, running the plan's own `<verify>` command set (`go test ./internal/setup/... ./cmd/engram/... -count=1`)
- **Issue:** `internal/setup/plan_test.go`'s `TestPlanBearerRedactsCredentialByProvenance`, `TestPlanBearerNeverReadsTokenFile`, and `TestPlanBearerEmptyTokenFileNamesEnvVar`, plus `cmd/engram/setup_test.go`'s `TestSetupPreviewJSONHasClaudeCodeCommand` and `TestSetupBearerTokenFileRedactedInOutput`, all asserted claude-code's retired single-action `Command` string and/or its `bearerProvenance`-based `"Bearer <from PATH>"` form — exactly what this plan's D-05/D-06/D-08 fixes retire
- **Fix:** Updated `TestPlanBearerRedactsCredentialByProvenance` and `TestPlanBearerNeverReadsTokenFile` to loop over all `Runtimes` (including claude-code) asserting the corrected ENGRAM_TOKEN-naming form; replaced `TestPlanBearerEmptyTokenFileNamesEnvVar` with `TestPlanClaudeCodeBearerIgnoresTokenFile` (TokenFile no longer affects claude-code's bearer output at all); updated the two `cmd/engram/setup_test.go` tests to the corrected Command string
- **Files modified:** `internal/setup/plan_test.go`, `cmd/engram/setup_test.go`
- **Verification:** `go test ./internal/setup/... ./cmd/engram/... -count=1` green; `task` green
- **Committed in:** `a461510e` (Task 2 GREEN commit)

**2. [Rule 3 - Blocking] `bearerProvenance` lost its last caller and tripped `task`'s unused-code lint**
- **Found during:** Task 3, running `task` per its own `<verify>`
- **Issue:** Once this plan and 03-03 both moved their runtimes off `bearerProvenance` onto their own env-var-reference forms, the function had zero callers anywhere in the module, and golangci-lint's default-enabled `unused` linter failed `task`. 03-02-PLAN.md's own Task 2 acceptance criterion explicitly forbids deleting the function (`rg -c 'func bearerProvenance' internal/setup/plan.go` must return `1`) because the generic pseudo-runtime (03-04) still needs it
- **Fix:** Added a justified `//nolint:unused` directly above the function with an expanded doc comment explaining the dead-until-03-04 window and pointing back to the acceptance criterion that forbids deletion
- **Files modified:** `internal/setup/plan.go`
- **Verification:** `task lint` and `task` both green
- **Committed in:** `7e30ae76` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 Rule 1 — a direct, foreseeable consequence of this plan's own header-syntax and write-sequence fixes; 1 Rule 3 — a blocking lint failure with no non-destructive fix available other than the justified ignore directive). **Impact:** No behavior outside claude-code's own registration sequence and bearer rendering was touched; codex and opencode are unaffected.

## Issues Encountered

None beyond the two deviations above. No real `claude`/`codex`/`opencode` binary was invoked at any point in this plan's tests — every exec goes through the scripted `Environment.Run` fake (rule `m45p2b4bp7`).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- claude-code is now a fully converged runtime alongside codex (03-01) and opencode (03-03): `Args`/`Probe`/`Tolerant` all authored, convergence pinned including the destructive-window failure path.
- `03-04` (generic pseudo-runtime) is unaffected by this plan's scope but is the intended next caller of `bearerProvenance`, which this plan deliberately preserved (with a justified `//nolint:unused`) rather than deleted.
- All three registered runtimes (`claude-code`, `codex`, `opencode`) now reach `OutcomeAlreadyCorrect` on a converged second `--apply` run — REQ-setup-idempotent's success criterion 5 is satisfied across the full registry.
- No blockers.

---
*Phase: 03-runtime-registration*
*Completed: 2026-09-09*

## Self-Check: PASSED

- FOUND: internal/setup/claudecode_test.go
- FOUND: internal/setup/claudecode.go
- FOUND: internal/setup/apply.go
- FOUND: internal/setup/apply_test.go
- FOUND: internal/setup/plan.go
- FOUND: internal/setup/plan_test.go
- FOUND: internal/surfaces/toolclass.go
- FOUND: cmd/engram/setup_test.go
- FOUND: .planning/phases/03-runtime-registration/03-02-SUMMARY.md
- FOUND commit: 9093de58 (test(03-02): pin claude-code's tolerant-remove-then-fatal-add sequence (RED))
- FOUND commit: a461510e (feat(03-02): claude-code tolerant-remove-then-fatal-add sequence (GREEN))
- FOUND commit: 7e30ae76 (test(03-02): correct toolclass.go's falsified overwrite comment; pin authored-not-positional tolerance)
- FOUND: go test ./internal/setup/... ./internal/surfaces/... ./cmd/engram/... -count=1 green
- FOUND: task (lint + full suite) green
- FOUND: go test ./internal/keylinks/... green
