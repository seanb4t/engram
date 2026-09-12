---
phase: 02-setup-command-core
plan: 01
subsystem: cli
tags: [cobra, cli, mcp, setup, claude-code, codex, opencode, koanf]

requires:
  - phase: 01-version-homebrew-distribution
    provides: "engram version --json and the release/distribution scaffolding this milestone builds toward"
provides:
  - "internal/setup: Runtime interface, Plan/Action/Outcome/Result types, injectable Environment seam, package-level Runtimes registry with real Detect()/Plan() for claude-code, codex, and opencode"
  - "engram setup: a real, preview-by-default operator command registered through registerDestructive, reporting one row per selected runtime with its exact mcp-add invocation"
  - "internal/config.ValidateSetupAuth and setup.url/setup.auth registry rows"
  - "internal/surfaces toolclass row for setup (Destructive:true, Idempotent:true)"
affects: [phase-3-runtime-registration, phase-5-generated-equivalence-proof]

actuals:
  tokens: 108000
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "internal/setup mirrors internal/migrate's package-level-literal registry discipline (Runtimes is a var literal, never built in init())"
    - "Injectable Environment struct-of-func-fields seam (LookPath/Getenv/HomeDir), same class as cliNow/citationFileReader"
    - "Credential redaction by provenance (Bearer <from PATH>), never by value or fixed mask"

key-files:
  created:
    - internal/setup/plan.go
    - internal/setup/environment.go
    - internal/setup/runtime.go
    - internal/setup/claudecode.go
    - internal/setup/codex.go
    - internal/setup/opencode.go
    - internal/setup/leafpurity_test.go
    - internal/setup/detect_test.go
    - internal/setup/plan_test.go
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/operator_view_setup_test.go
  modified:
    - internal/config/registry.go
    - internal/config/config.go
    - internal/config/client_validate.go
    - internal/config/client_validate_test.go
    - internal/surfaces/toolclass.go
    - cmd/engram/destructive_test.go
    - cmd/engram/cmdwalk_test.go
    - cmd/engram/operator_output_test.go
    - cmd/engram/clienttest_test.go
    - cmd/engram/golden_test.go
    - cmd/engram/testdata/catalog.golden
    - cmd/engram/testdata/help.golden

key-decisions:
  - "All twelve (runtime, auth mode) cells authored as either an exact invocation or an explicit ErrAuthModeUnsupported result (opencode x oauth-client is the one permanent unsupported cell) — D-09."
  - "--runtime deliberately NOT routed through internal/config's registry; its ENGRAM_RUNTIME default is read directly via os.Getenv at init() time (Pitfall 3: pflag's StringSliceValue.String() returns a bracketed display form the changed-flag overlay cannot round-trip)."
  - "Bearer credential redacted by provenance (Bearer <from PATH>, or <from ENGRAM_TOKEN> when --token-file is empty) — Plan() never opens, stats, or reads the token file at any layer."
  - "Codex's bearer mode names ENGRAM_TOKEN via its own --bearer-token-env-var flag rather than a provenance placeholder — the live-verified codex mcp add surface carries no --header equivalent, so this is a structurally different (and simpler) redaction: no path or value ever appears in the command at all."

requirements-completed:
  - REQ-setup-detects-runtimes
  - REQ-setup-previews-by-default
  - REQ-setup-non-interactive
  - REQ-setup-correct-by-reading

coverage:
  - id: D1
    description: "engram setup detects claude-code, codex, and opencode via exec.LookPath only, never a config-directory stat"
    requirement: "REQ-setup-detects-runtimes"
    verification:
      - kind: unit
        ref: "internal/setup/detect_test.go#TestDetectEveryRegisteredRuntime"
        status: pass
      - kind: unit
        ref: "internal/setup/detect_test.go#TestDetectIgnoresConfigDirectory"
        status: pass
    human_judgment: false
  - id: D2
    description: "A bare engram setup (no --apply) previews every detected runtime's exact mcp-add invocation and writes nothing"
    requirement: "REQ-setup-previews-by-default"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupPreviewExecutesNoRuntimeCLI"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupPreviewJSONHasClaudeCodeCommand"
        status: pass
    human_judgment: false
  - id: D3
    description: "engram setup runs to completion with no TTY, --runtime/--auth accept explicit values, and --output json emits a parseable document"
    requirement: "REQ-setup-non-interactive"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupAllThreeRuntimesPresentEmitsThreeRows"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupRejectsUnknownRuntime"
        status: pass
    human_judgment: false
  - id: D4
    description: "engram setup --help teaches every targetable runtime, the four auth modes, and what --apply does, without running the command and interpreting a failure"
    requirement: "REQ-setup-correct-by-reading"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupHelpNamesEveryRuntimeAndAuthMode"
        status: pass
      - kind: automated_ui
        ref: "go run ./cmd/engram setup --help (live, recorded below)"
        status: pass
    human_judgment: false
  - id: D5
    description: "The bearer credential is previewed by provenance (Bearer <from PATH>) and Plan() never opens, stats, or reads the token file"
    verification:
      - kind: unit
        ref: "internal/setup/plan_test.go#TestPlanBearerNeverReadsTokenFile"
        status: pass
      - kind: unit
        ref: "internal/setup/plan_test.go#TestPlanBearerRedactsCredentialByProvenance"
        status: pass
    human_judgment: false

duration: ~4h (including a mid-session pause for a 1Password signing-agent outage)
completed: 2026-08-30
status: complete
---

# Phase 2 Plan 1: Setup Command Core Summary

`engram setup` lands as a real, preview-by-default cobra command backed by the new
`internal/setup` package, detecting claude-code, codex, and opencode via `exec.LookPath` and
authoring the exact `mcp add` invocation (all twelve runtime x auth-mode cells) each would issue —
changing nothing on disk this phase.

## Performance

- **Duration:** ~4h wall-clock (active implementation was closer to ~2h; the remainder was a
  1Password SSH-signing-agent outage that blocked every `git commit` mid-session — see Deviations)
- **Tasks:** 3/3 completed
- **Files modified:** 12 created, 12 modified (24 total)

## Accomplishments

- `internal/setup`: `Runtime` interface, five-value `Outcome` enum (`OutcomeNotPresent` a real
  value, never a zero/absence per D-07), injectable `Environment` seam, and a package-level
  `Runtimes` literal with real `Detect()`/`Plan()` for all three v1 runtimes.
- `engram setup`: routed through `registerDestructive`, rendering one typed report doc via
  `renderOperator` — text and json cannot drift by construction. `--apply` returns
  `setup.ErrApplyNotImplemented` (Phase 3 stub, D-09) after rendering the identical preview.
- All twelve (runtime, auth-mode) cells authored: `oauth`/`oauth-client`/`bearer`/`none` on
  claude-code and codex, plus `oauth`/`bearer`/`none` on opencode (its `oauth-client` cell is a
  permanent, explicit `ErrAuthModeUnsupported` — no client-id flag exists on the live-verified
  surface). Bearer credentials redacted by provenance, never by value.
- Seven pinned interface gates backfilled in the same commits as the `toolclass.go` row
  (`TestMutatingCommandNamesMembership`, `TestDestructiveCommandsExactFlagSet`,
  `wantOperatorCommandKeys`, `operatorViewFixtures`/`operatorInvalidOutputArgs`,
  `resetClientFlags`'s StringSliceVar cleanup, `envDerivedFlagDefaults`), with
  `catalog.golden`/`help.golden` regenerated via `task surfaces:gen`.

## Task Commits

Each task was implemented, tested, and verified individually against its own `<verify>` and
`<acceptance_criteria>` before moving to the next. Due to a git-mechanics accident during the
resume after the 1Password outage (see Deviations, item 2), the actual commit boundaries do not
line up 1:1 with the plan's three tasks — the full, correct, fully-tested implementation of all
three tasks is present and green at the final commit below:

1. **Task 1 (end-to-end claude-code preview) — split across two commits:**
   - `e7719127` — pinned-gate test churn untouched by later tasks (`destructive_test.go`,
     `cmdwalk_test.go`, `operator_output_test.go`, `clienttest_test.go`, `golden_test.go`).
   - `1d2fe791` — the `internal/setup` package, `internal/config`/`internal/surfaces` rows, and
     `cmd/engram/setup.go` plus its own tests and goldens. **Note:** because of the git-mechanics
     issue below, this commit's working-tree state for the files it touches already includes
     Task 2's and Task 3's changes to those same files (three-runtime `Runtimes` literal, all
     four auth modes on claude-code, `bearerProvenance`, `setupCmd.Long`/`Example`, and the final
     goldens) — it is **not independently buildable in isolation** (missing `Codex`/`OpenCode`
     symbols until the next commit).
2. **Task 2 (codex + opencode registration) + Task 3 (all four auth modes, credential redaction,
   self-teaching `--help`)** — `67a6090d`: adds `internal/setup/codex.go`,
   `internal/setup/opencode.go`, and `internal/setup/plan_test.go` (the exhaustive 3x4
   runtime-by-auth-mode table). This is the commit that restores buildability; **HEAD at
   `67a6090d` is the complete, tested, plan-final state** — `go build ./...`, `go test ./...
   -count=1`, `task lint`, and `task license:check` all pass at this commit, and
   `task surfaces:gen` produces no further diff.

**Plan metadata:** committed separately immediately after this SUMMARY (worktree mode — STATE.md
and ROADMAP.md are the orchestrator's own job, excluded here).

## Files Created/Modified

- `internal/setup/plan.go` — `Outcome`/`Action`/`Plan`/`Result` types, `bearerProvenance` helper
- `internal/setup/environment.go` — injectable `Environment` seam, `OSEnvironment`
- `internal/setup/runtime.go` — `Runtime` interface, `Runtimes` registry, `Names()`, `Select()`
- `internal/setup/claudecode.go` — `ClaudeCode` Runtime, all four auth modes
- `internal/setup/codex.go` — `Codex` Runtime, all four auth modes
- `internal/setup/opencode.go` — `OpenCode` Runtime, three auth modes + permanent unsupported cell
- `internal/setup/leafpurity_test.go` — stdlib-only import gate (mirrors `internal/migrate`'s)
- `internal/setup/detect_test.go` — fake-`Environment`-driven detection tests
- `internal/setup/plan_test.go` — exhaustive 3x4 auth-mode table, URL passthrough, `Select()`
- `cmd/engram/setup.go` — the `setup` cobra command, report doc, preview/apply closures
- `cmd/engram/setup_test.go` — preview/apply/exit-code/redaction/help-golden tests
- `cmd/engram/operator_view_setup_test.go` — view-identity fixtures for every report shape
- `internal/config/registry.go` — `setup.url`/`setup.auth` rows
- `internal/config/config.go` — `SetupConfig` struct, `Config.Setup` field
- `internal/config/client_validate.go` — `ValidateSetupAuth`
- `internal/config/client_validate_test.go` — `TestValidateSetupAuth`
- `internal/surfaces/toolclass.go` — `setup` toolclass row (Destructive:true, Idempotent:true)
- `cmd/engram/destructive_test.go`, `cmdwalk_test.go`, `operator_output_test.go`,
  `clienttest_test.go`, `golden_test.go` — pinned-gate churn for the new command
- `cmd/engram/testdata/catalog.golden`, `help.golden` — regenerated via `task surfaces:gen`

## Decisions Made

See `key-decisions` in frontmatter. All decisions were pre-made in `02-CONTEXT.md` (D-01 through
D-16) and `02-PATTERNS.md`; this plan applied them rather than making new ones, with one
discretionary choice: `Plan`/`Action`/`Outcome`/`Result`'s exact Go shapes (string-backed `Outcome`
enum, field names) were left to the planner per CONTEXT.md's "Claude's Discretion" note, and are
recorded here as shipped.

## Deviations from Plan

### Process Deviations (not code defects)

**1. [Environment blocker] 1Password SSH-signing-agent hard failure, resolved by explicit user
authorization to bypass signing per-commit**
- **Found during:** attempting to commit Task 1 (after all three tasks were fully implemented,
  tested, and verified against the plan's own acceptance criteria).
- **Issue:** every `git commit` failed with `error: 1Password: failed to fill whole buffer` /
  `fatal: failed to write commit object` (one retry surfaced `1Password: agent returned an
  error`). Confirmed the failure was in the signing helper itself, not git or the working tree:
  invoking `op-ssh-sign -Y sign -n git -f ~/.ssh/seanb4t_ed25519.pub <file>` directly reproduced
  the identical error. Retried 8 times over several minutes with no change; not transient.
- **Fix:** returned a `checkpoint:human-action` to the orchestrator/user rather than bypassing
  signing unilaterally. The user explicitly authorized a per-commit bypass in-session (standing
  repo memory record `m614ggnzt9`: "Autonomous runs MAY bypass a locked 1Password signing agent
  with per-commit `git -c commit.gpgsign=false`; never flip persistent config; sign normally when
  1Password is available"). All three commits in this plan (`e7719127`, `1d2fe791`, `67a6090d`)
  and this SUMMARY's own commit were made with `git -c commit.gpgsign=false commit <pathspec>
  ...` — the per-invocation flag form only. No persistent git config was modified; `.gitconfig`
  and the repo's local config are untouched, so normal signing resumes automatically the next
  time 1Password is available.
- **Files modified:** none (process-only).
- **Verification:** `git config --local --list | grep sign` returns nothing (confirms no local
  override was written); `git log --show-signature` on these four commits will show them as
  unsigned, which is expected and by design for this bypass.
- **Impact:** none on code correctness; commits are unsigned, a fact visible in the commit
  history for anyone auditing it.

**2. [Rule 3 - blocking issue, self-corrected] Task-commit boundary collapsed due to
`git commit <pathspec>` capturing working-tree state, not staged-index state**
- **Found during:** committing Task 1 after resuming from the signing outage. Task 1's files had
  been deliberately staged (`git add`) in their Task-1-only form, with Task 2's and Task 3's
  further edits to the same files present but deliberately left UNSTAGED on top, so that three
  separate, independently-buildable commits could be made in sequence.
- **Issue:** `git commit <pathspec>...` (without `--only`) commits the pathspec-matched files'
  CURRENT WORKING-TREE content, not their staged INDEX content, when they differ. This is
  documented git behavior but not what I expected: it silently pulled Task 2's and Task 3's
  unstaged edits into what was intended to be a Task-1-only commit (`1d2fe791`), for every file
  that carried a layered edit (`internal/setup/{runtime.go,claudecode.go,plan.go,detect_test.go}`,
  `cmd/engram/{setup.go,setup_test.go,operator_view_setup_test.go}`, and the two golden files).
  Files with NO further edits (the five pinned-gate test files in `e7719127`) were unaffected.
- **Fix:** rather than rewrite history (forbidden — no `git commit --amend`/rebase without
  explicit request), added one further commit (`67a6090d`) containing the three files that were
  still genuinely new/untracked (`internal/setup/{codex.go,opencode.go,plan_test.go}`) — these
  supply the `Codex`/`OpenCode` symbols `runtime.go` (already committed in `1d2fe791`)
  references, restoring full buildability. Verified `go build ./...`, `go test ./... -count=1`,
  `task lint`, `task license:check`, and `task surfaces:gen` (no further diff) all pass at the
  final commit.
- **Files modified:** none beyond what was already planned — this only affected which commit
  each already-planned file's final content landed in.
- **Verification:** `git status --short` is clean at HEAD; full test suite green; see the
  Performance section above for exact commands.
- **Committed in:** `67a6090d` (the corrective commit) — see the "Task Commits" section above for
  the full accounting of which content landed in which commit.
- **Impact:** the intended one-commit-per-task granularity is not preserved (`1d2fe791`
  mixes Tasks 1–3's content for the files it touches, and is not independently buildable in
  isolation). All code is present, correct, and fully tested at the final commit
  (`67a6090d`); a future `git bisect` across this plan should treat `e7719127`..`67a6090d` as one
  unit rather than expect each commit to be independently green.

---

**Total deviations:** 2 (both process/git-mechanics, zero code defects). **Impact on plan:** none
on the shipped code's correctness — every acceptance criterion in `02-01-PLAN.md` is verified
against the final working tree (`67a6090d`), and the full test suite, lint, and license gates are
green there. The impact is limited to commit-history granularity and commit signatures, both
recorded above for auditability.

## Known Stubs

- **`engram setup --apply`** returns `setup.ErrApplyNotImplemented` and performs no mutation.
  This is an **intentional, plan-documented stub** (D-09 in `02-CONTEXT.md`): Phase 2 ships only
  `Detect()` and `Plan()`; Phase 3 ("Runtime Registration") is explicitly scoped to implement
  `Apply()` and actually execute the authored invocations. Not a defect — resolved by design in
  the next phase.

## Issues Encountered

See "Deviations from Plan" above — both items were genuinely encountered issues (an environment
outage and a git-mechanics surprise), fully resolved, and documented there rather than duplicated
here.

## User Setup Required

None — no external service configuration required. (The 1Password signing outage from Deviation 1
was a session-local git-signing issue, already resolved by the user in-session; no standing setup
action is needed going forward.)

## Next Phase Readiness

Phase 3 (Runtime Registration) can proceed: `internal/setup.Runtime`'s `Plan()` for all twelve
(runtime, auth-mode) cells is authored and stable — Phase 3's job is to execute these exact
strings via `os/exec`, never to re-derive them (D-09's explicit boundary). `ErrApplyNotImplemented`
is the single, typed signal Phase 3 replaces with a real `Apply()` implementation.
