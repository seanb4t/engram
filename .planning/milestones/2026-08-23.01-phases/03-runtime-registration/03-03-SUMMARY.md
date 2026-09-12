---
phase: 03-runtime-registration
plan: 03
subsystem: setup
tags: [mcp-registration, opencode, os-exec, argv-security, cli-drift]

requires:
  - phase: 03-runtime-registration
    provides: "Action.Args/Action.Tolerant, Plan.Probe, Environment.Run seam, setup.Preview()/Apply() shared executor, codex.go reference shape (plan 03-01)"
provides:
  - "opencode.go converted to Args/Probe (D-01/D-09), with the shipped colon-space bearer-header syntax bug fixed to opencode's required KEY=VALUE form"
  - "opencode's bearer header names ENGRAM_TOKEN through opencode's own {env:...} substitution token — no credential value, no token-file path (D-05/D-06)"
  - "opencode's doc comment recording both third-party constraints: the polluted, per-server-networked mcp list probe, and the upstream {env:...} substitution reliability gap (anomalyco/opencode#5299)"
  - "TestApplyOpenCodeConvergence pinning D-08's convergence behavior for opencode's uniquely polluted probe"
affects: [03-04-generic-portable-config, 03-05-remaining-wave, 06-docs-site]

actuals:
  tokens: 6238
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Runtime-native substitution token for bearer credentials (D-05/D-06) — a second, independently-verified instance of the pattern claude-code's {env:...}/${...} form and codex's --bearer-token-env-var flag already established: never a provenance placeholder for a runtime whose own CLI resolves the variable at connect time"

key-files:
  created:
    - internal/setup/opencode_test.go
  modified:
    - internal/setup/opencode.go
    - internal/setup/plan_test.go
    - cmd/engram/setup_test.go

key-decisions:
  - "Two pre-existing tests (internal/setup/plan_test.go's TestPlanBearerRedactsCredentialByProvenance and TestPlanBearerNeverReadsTokenFile, cmd/engram/setup_test.go's TestSetupBearerTokenFileRedactedInOutput) pinned the OLD provenance-placeholder form for opencode's bearer output. Updated all three to assert opencode's corrected ENGRAM_TOKEN-naming form (D-05/D-06) while leaving claude-code's still-provenance-based assertions untouched — that runtime's own conversion is plan 03-02's scope, not this plan's. This is a direct, foreseeable consequence of the header-syntax fix this plan exists to make, not scope creep."
  - "The doc-comment work Task 2 calls for (probe asymmetry, {env:...} reliability gap) was written directly into opencode.go's Plan() doc comment as part of Task 1's rewrite, since both fixes touch the exact same comment block and splitting them across two commits would have meant reverting/re-editing the same paragraph twice. Task 2's own commit is therefore test-only (TestApplyOpenCodeConvergence), consistent with 03-01's own precedent for a coherent single-pass doc edit."

requirements-completed: [REQ-register-opencode, REQ-setup-idempotent, REQ-register-auth-modes]

coverage:
  - id: D1
    description: "opencode's Plan() authors a single, non-tolerant Action for oauth/none/bearer, with the corrected KEY=VALUE header form and an ENGRAM_TOKEN-naming bearer value never carrying a credential or token-file path"
    requirement: REQ-register-auth-modes
    verification:
      - kind: unit
        ref: "internal/setup/opencode_test.go#TestOpenCodePlan"
        status: pass
      - kind: unit
        ref: "internal/setup/opencode_test.go#TestOpenCodeBearerHeaderSyntax"
        status: pass
      - kind: unit
        ref: "internal/setup/opencode_test.go#TestOpenCodeBearerHeaderCarriesNoSecret"
        status: pass
    human_judgment: false
  - id: D2
    description: "opencode's oauth-client mode states plainly it is unsupported, wrapping ErrAuthModeUnsupported and naming both the runtime and the mode"
    requirement: REQ-register-auth-modes
    verification:
      - kind: unit
        ref: "internal/setup/opencode_test.go#TestOpenCodePlan/oauth-client"
        status: pass
    human_judgment: false
  - id: D3
    description: "opencode's convergence classification degrades in the safe direction under its polluted, every-server, network-dialing mcp list probe: identical captures classify already-correct, captures differing only in an unrelated server's status never do"
    requirement: REQ-setup-idempotent
    verification:
      - kind: unit
        ref: "internal/setup/opencode_test.go#TestApplyOpenCodeConvergence"
        status: pass
    human_judgment: false

duration: 33min
completed: 2026-09-09
status: complete
---

# Phase 3 Plan 3: opencode registration — header fix and convergence pinning Summary

**`engram setup --apply --runtime opencode --auth bearer` now authors the `Authorization=Bearer {env:ENGRAM_TOKEN}` KEY=VALUE argv opencode's real CLI accepts, replacing the confirmed-broken colon-space form, with a regression test that fails if the old form is ever reintroduced.**

## Performance

- **Duration:** 33 min
- **Started:** 2026-09-09T02:15:00Z
- **Completed:** 2026-09-09T02:48:00Z
- **Tasks:** 2
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments
- Fixed the live, confirmed bug in `internal/setup/opencode.go`'s bearer-mode header: `Authorization: Bearer ...` (HTTP-header-string form, rejected outright by opencode's real CLI) is now `Authorization=Bearer {env:ENGRAM_TOKEN}` (opencode's required `KEY=VALUE` form, live-verified in 03-RESEARCH.md to write and round-trip correctly).
- Converted `opencode.go`'s `Plan()` to the `Args`/`Probe` model (D-01/D-09): every auth mode authors a single, non-tolerant `Action`, and every `Plan` carries `Probe = {"opencode", "mcp", "list"}` — the only read verb opencode's CLI surface exposes.
- Recorded both of opencode's real third-party limitations directly in `opencode.go`'s doc comment: the probe asymmetry (no `--json`, every-registered-server listing, per-server network dial) and the upstream `anomalyco/opencode#5299` `{env:...}` substitution reliability gap, including the operator-visible failure signature an unresolved credential would produce.
- Added `TestApplyOpenCodeConvergence`, pinning D-08's ambiguity-resolves-to-wrote invariant specifically against opencode's polluted probe: an unrelated server's status-glyph flip between two reads must classify `wrote`, never `already-correct`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix opencode's header syntax and convert it to the Args/Probe model** — `8e074f07` (test, RED) then `9f8e3c22` (feat, GREEN)
2. **Task 2: Pin opencode's convergence behavior and record its two third-party constraints** — `586eee0d` (test — doc-comment work landed in Task 1's commit; see Decisions)

**Plan metadata:** committed separately per this workflow's final-commit step.

_Note: Task 1 was `tdd="true"` and followed the canonical RED→GREEN sequence (`8e074f07` → `9f8e3c22`); Task 2 was not TDD-flagged and lands as a single test-only commit._

## Files Created/Modified
- `internal/setup/opencode.go` — `Plan()` rewritten to `Args`/`Probe`, bearer header fixed to `KEY=VALUE`, doc comment records both third-party constraints
- `internal/setup/opencode_test.go` — `TestOpenCodePlan`, `TestOpenCodeBearerHeaderSyntax`, `TestOpenCodeBearerHeaderCarriesNoSecret`, `TestApplyOpenCodeConvergence`
- `internal/setup/plan_test.go` — `TestPlanBearerRedactsCredentialByProvenance` and `TestPlanBearerNeverReadsTokenFile` updated to assert opencode's corrected ENGRAM_TOKEN-naming form instead of the retired provenance placeholder
- `cmd/engram/setup_test.go` — `TestSetupBearerTokenFileRedactedInOutput` updated to assert per-runtime redaction forms (claude-code's provenance placeholder vs. opencode's ENGRAM_TOKEN naming)

## Decisions Made
See `key-decisions` in frontmatter for full reasoning on the two non-obvious calls this plan made: updating three pre-existing tests that pinned opencode's old (broken) provenance form, and folding Task 2's doc-comment work into Task 1's commit.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Three pre-existing tests pinned opencode's now-retired provenance-placeholder bearer form**
- **Found during:** Task 1, running the plan's own `<verify>` command set (`go test ./internal/setup/... ./cmd/engram/... -count=1`)
- **Issue:** `internal/setup/plan_test.go`'s `TestPlanBearerRedactsCredentialByProvenance` and `TestPlanBearerNeverReadsTokenFile`, and `cmd/engram/setup_test.go`'s `TestSetupBearerTokenFileRedactedInOutput`, all asserted opencode's bearer output contained the `Bearer <from PATH>` provenance string — the exact form this plan's D-05/D-06 fix retires for opencode in favor of naming `ENGRAM_TOKEN` via opencode's own `{env:...}` substitution token
- **Fix:** Updated all three tests to assert opencode's corrected form (names `ENGRAM_TOKEN`, never the token-file path or a provenance placeholder) while leaving every claude-code assertion in the same tests untouched, since claude-code's own D-05/D-06 conversion is plan 03-02's scope
- **Files modified:** `internal/setup/plan_test.go`, `cmd/engram/setup_test.go`
- **Verification:** `go test ./internal/setup/... ./cmd/engram/... -count=1` green; `go test ./... -count=1` green; `task` green
- **Committed in:** `9f8e3c22` (Task 1 GREEN commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — a direct, foreseeable consequence of this plan's own header-syntax fix, not scope creep). **Impact:** No behavior outside opencode's bearer-mode credential rendering was touched; claude-code's still-pending conversion (plan 03-02) is unaffected.

## Issues Encountered

None — no real `opencode`/`codex`/`claude` binary was invoked at any point; every exec in the new and updated tests goes through the scripted `Environment.Run` fake (rule `m45p2b4bp7`, confirmed by `rg -n 'exec\.' internal/setup/opencode_test.go` finding no match).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- opencode is now a fully converged Wave-2/3 runtime alongside codex (03-01): `Args`/`Probe`/`Tolerant` all authored, convergence pinned under its own probe's real constraints.
- `03-04` (generic portable config) and `03-05` (remaining wave) are unaffected by this plan's scope — neither touches `opencode.go`.
- Phase 6's docs-site work should surface both documented opencode caveats (the polluted probe and the `{env:...}` reliability gap) in operator-facing prose, per this plan's own doc-comment cross-reference.
- No blockers.

---
*Phase: 03-runtime-registration*
*Completed: 2026-09-09*

## Self-Check: PASSED

- FOUND: internal/setup/opencode.go
- FOUND: internal/setup/opencode_test.go
- FOUND: internal/setup/plan_test.go
- FOUND: cmd/engram/setup_test.go
- FOUND commit: 8e074f07 (test(03-03): pin corrected opencode Args/Probe shape and bearer header regression)
- FOUND commit: 9f8e3c22 (feat(03-03): fix opencode bearer header syntax and convert to Args/Probe)
- FOUND commit: 586eee0d (test(03-03): pin opencode convergence behavior under its polluted probe)
- FOUND: go test ./... -count=1 green
- FOUND: task (lint + full suite) green
