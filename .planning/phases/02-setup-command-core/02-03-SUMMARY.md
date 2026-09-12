---
phase: 02-setup-command-core
plan: 03
subsystem: cli
tags: [koanf, config, cobra, setup, exit-codes]

# Dependency graph
requires:
  - phase: 02-setup-command-core (plans 01-02)
    provides: "the setup command, internal/setup's Runtime/Plan/Options types, and the setup.url/setup.auth registry rows (previously dead code)"
provides:
  - "a live ENGRAM_URL/ENGRAM_AUTH environment lane for engram setup, resolved through config.Load(cmd.Flags()) exactly like every other client/operator command"
  - "a required-URL usage-error guard (exitUsage) replacing a silently malformed would-write command"
  - "first-occurrence deduplication in internal/setup.Select for repeated --runtime names"
affects: [phase-3-apply-implementation, phase-5-generated-equivalence-proof]

# Actuals (#2632)
actuals:
  tokens: 4486
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "registry-backed cobra flag with no backing Go var (mirrors addClientFlags): setupCmd.Flags().String(...) + config.Load(cmd.Flags()) is the single resolution path"

key-files:
  created: []
  modified:
    - cmd/engram/setup.go
    - cmd/engram/setup_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - internal/setup/runtime.go
    - internal/setup/plan_test.go

key-decisions:
  - "Kept the full registry enrollment (SetupConfig, setup.url, setup.auth, both (default: ENGRAM_*) help suffixes) per D-04 — did not implement 02-VERIFICATION.md's rejected delete-the-claims alternative"
  - "Required-URL guard is a runtime usageErrorf inside setupPlanDoc, not cobra's MarkFlagsRequired — mirrors clientFromFlags' --server-or-ENGRAM_SERVER_URL guard and avoids reintroducing the plain-fmt.Errorf-bypasses-cliError defect D-03 already rejected"
  - "Select's --runtime dedup silently skips a repeat rather than erroring — a duplicate is unambiguous about intent, unlike an unknown name"

patterns-established:
  - "A cobra command's env-backed flags read exclusively through config.Load(cmd.Flags()) inside the function that consumes them — never a second os.Getenv call alongside the registry path"

requirements-completed: [REQ-setup-correct-by-reading]

coverage:
  - id: D1
    description: "ENGRAM_URL / ENGRAM_AUTH reach the previewed command for every present runtime with no matching flag (CR-01)"
    requirement: "REQ-setup-correct-by-reading"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupURLFromEnvReachesCommand"
        status: pass
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupAuthFromEnvSelectsBearerForm"
        status: pass
      - kind: integration
        ref: "ENGRAM_URL=... ENGRAM_AUTH=bearer go run ./cmd/engram setup --output json (manual repro of the original CR-01 failure)"
        status: pass
    human_judgment: false
  - id: D2
    description: "An absent URL from both lanes exits usage error (2) instead of a malformed would-write report (WR-01)"
    requirement: "REQ-setup-correct-by-reading"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupMissingURLIsUsageError"
        status: pass
      - kind: integration
        ref: "go run ./cmd/engram setup --output json (ENGRAM_URL unset) exits 2, names --url and ENGRAM_URL"
        status: pass
    human_judgment: false
  - id: D3
    description: "--url and --auth flags beat ENGRAM_URL/ENGRAM_AUTH when both are set"
    requirement: "REQ-setup-correct-by-reading"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupFlagBeatsEnvForURL"
        status: pass
    human_judgment: false
  - id: D4
    description: "A URL crossing the environment lane reaches the emitted command byte-for-byte (D-02): no appended/stripped slash, no decode, no re-encode"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupEnvURLPassedVerbatim"
        status: pass
    human_judgment: false
  - id: D5
    description: "Two identical preview runs with the env lane set are byte-identical on both the json and text lanes"
    verification:
      - kind: unit
        ref: "cmd/engram/setup_test.go#TestSetupEnvLanePreviewDeterministic"
        status: pass
    human_judgment: false
  - id: D6
    description: "A repeated --runtime name (WR-02) produces exactly one report row, preserving first-occurrence order; a repeated unknown name still errors"
    verification:
      - kind: unit
        ref: "internal/setup/plan_test.go#TestSelectDedupesRepeatedNames"
        status: pass
      - kind: integration
        ref: "go run ./cmd/engram setup --runtime claude-code,claude-code --output json emits a runtimes array of length 1"
        status: pass
    human_judgment: false

# Metrics
duration: 25min
completed: 2026-08-30
status: complete
---

# Phase 2 Plan 3: Wire the ENGRAM_URL/ENGRAM_AUTH env lane through config.Load Summary

**`engram setup`'s `--url`/`--auth` now resolve through `config.Load(cmd.Flags())` like every other command, making the shipped `(default: ENGRAM_URL)`/`(default: ENGRAM_AUTH)` help claims true, and an absent URL now fails at exit 2 instead of silently emitting a broken command.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-08-30T14:44:00Z
- **Completed:** 2026-08-30T15:09:00Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Closed CR-01: `--url`/`--auth` are registry-backed flags with no backing Go var (mirrors `addClientFlags`); `setupPlanDoc` calls `config.Load(cmd.Flags())` as its first step and reads `cfg.Setup.URL`/`cfg.Setup.Auth`
- Closed WR-01: an empty `cfg.Setup.URL` (from either lane, or an explicit `--url ""`) returns a `usageErrorf` naming both `--url` and `ENGRAM_URL`, evaluated after auth validation and runtime selection so existing `--auth basic` / `--runtime nope` error messages are unchanged
- Closed WR-02: `internal/setup.Select` deduplicates repeated `--runtime` names to first occurrence, so `--runtime claude-code,claude-code` yields one report row instead of two
- Regenerated `cmd/engram/testdata/help.golden` and `cmd/engram/testdata/catalog.golden` via `task surfaces:gen` — `--url`'s usage string gained a `(required)` marker; both `(default: ENGRAM_*)` suffixes and both registry rows survive intact (scope fence honored)
- Added six new tests for the environment lane (reach, bearer-form selection, flag-beats-env, missing-URL usage error x3 sub-cases, verbatim byte passthrough, preview determinism) and one for `Select`'s dedup

## Task Commits

Each task was committed atomically, with RED (tests written and confirmed failing against the pre-fix implementation) verified before GREEN (implementation making them pass) within the same commit:

1. **Task 1: Wire ENGRAM_URL/ENGRAM_AUTH end to end through config.Load, and make an absent URL a usage error** - `d24ce988` (feat)
2. **Task 2: Dedupe repeated --runtime names in internal/setup.Select (WR-02)** - `69f55622` (fix)

_Note: both tasks carry `tdd="true"`; RED was verified (tests failed against the pre-fix code) before writing the GREEN implementation, but each task landed as a single atomic commit rather than separate `test(...)`/`feat(...)` commits — the plan's frontmatter `type` is `execute`, not `tdd`, and `workflow.tdd_mode` is not enabled in this project's config, so the strict two-commit RED/GREEN gate (`references/tdd.md`'s `<gate_enforcement>`) does not apply here._

## Files Created/Modified
- `cmd/engram/setup.go` - deleted `setupURL`/`setupAuth` package vars; `--url`/`--auth` registered as registry-backed flags; `setupPlanDoc` now takes `cmd *cobra.Command`, calls `config.Load`, and enforces the required-URL guard
- `cmd/engram/setup_test.go` - six new tests for the environment lane and its edges; four existing tests updated to pass `--url` now that it's required
- `cmd/engram/testdata/help.golden` - `--url`'s usage line gained `(required)`
- `cmd/engram/testdata/catalog.golden` - same string change reflected in the machine-readable catalog (regenerated by `task surfaces:gen` alongside help.golden — not separately listed in the plan's `files_modified`, but an unavoidable, mechanical consequence of the one authorized generator)
- `internal/setup/runtime.go` - `Select` gained first-occurrence deduplication
- `internal/setup/plan_test.go` - `TestSelectDedupesRepeatedNames`

## Decisions Made
- Kept `SetupConfig`, both registry rows, and both `(default: ENGRAM_*)` help suffixes intact — the plan's scope fence rejected the delete-the-claims alternative from 02-VERIFICATION.md, and D-04 (full registry enrollment) stands.
- Enforced the required-URL guard via a runtime `usageErrorf` inside `setupPlanDoc`, not cobra's `MarkFlagsRequired` — cobra's mechanism raises a plain `fmt.Errorf` that bypasses `cliError`/`ExitCode()` (the exact defect D-03 already rejected `MarkFlagsMutuallyExclusive` for) and would also incorrectly demand `--url` even when `ENGRAM_URL` alone supplies the value.
- `Select`'s `--runtime` dedup silently skips a repeat rather than erroring: `--runtime a,a` and `--runtime a --runtime a` are both unambiguous about intent, unlike an unknown name, which is a genuinely different kind of wrongness and still errors even when repeated.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `cmd/engram/testdata/catalog.golden` also needed regeneration**
- **Found during:** Task 1 (`task surfaces:gen` regenerates both `help.golden` and `catalog.golden` from the same live cobra tree in one invocation)
- **Issue:** The plan's `files_modified` frontmatter lists only `cmd/engram/testdata/help.golden`, but `--url`'s changed usage string also appears in the machine-readable `catalog.golden`. Running the single authorized generator (`task surfaces:gen`, as the plan's own `<action>` instructs) necessarily touches both.
- **Fix:** Included `cmd/engram/testdata/catalog.golden` in Task 1's commit alongside `help.golden`. The diff is the identical one-line string change reflected in JSON form; no other content changed.
- **Files modified:** `cmd/engram/testdata/catalog.golden`
- **Verification:** `task surfaces:gen` re-run after the commit leaves the working tree clean (plan-level `<verification>` requirement); `go test ./... -count=1` and `task` both pass.
- **Committed in:** `d24ce988` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking — an unavoidable consequence of the one sanctioned generator command).
**Impact on plan:** No scope creep; the extra file is a byte-identical-mechanism regeneration of a file already in the phase's golden-pinning discipline, not new authored content.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `REQ-setup-correct-by-reading` is unblocked: every `engram setup --help` claim about `--url`/`--auth` is now true of the shipped binary.
- `Config.Setup.URL`/`Config.Setup.Auth` are live inputs; the `setup.url`/`setup.auth` registry rows are no longer dead code.
- Phase 3 (`Apply()` implementation) can proceed against a `setup.Options` that is now correctly sourced from either lane, with no residual malformed-command failure mode to work around.
- No blockers.

## Self-Check: PASSED

- `cmd/engram/setup.go` exists and contains no `setupURL`/`setupAuth` package vars — confirmed via `grep -n "^var ("`.
- `internal/setup/runtime.go` exists and `Select` contains the `seen` map dedup logic — confirmed via read.
- Commit `d24ce988` found in `git log --oneline --all`.
- Commit `69f55622` found in `git log --oneline --all`.
- All plan-level `<verification>` commands re-run and passed: `go test ./... -count=1` (0), `task` (0), `task license:check` (0), `task surfaces:gen` (clean tree), the `ENGRAM_URL`/`ENGRAM_AUTH` manual repro (exit 0, URL byte-for-byte in every row), the missing-URL manual repro (exit 2, names both spellings), `TestCatalogExitCodesMatchMapper` (pass, no gap/duplicate in exit codes 0-9).
- All plan-level `<success_criteria>` re-verified against the diff and golden files.

---
*Phase: 02-setup-command-core*
*Completed: 2026-08-30*
