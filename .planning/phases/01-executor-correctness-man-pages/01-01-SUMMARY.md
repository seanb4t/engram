---
phase: 01-executor-correctness-man-pages
plan: 01
subsystem: infra
tags: [os-exec, context, subprocess, setup, tdd]

# Dependency graph
requires: []
provides:
  - "osRun (internal/setup/environment.go) reports ctx.Err() before unwrapping *exec.ExitError, so a deadline-killed or cancelled subprocess never misreports as a clean exit -1 (GitHub #560)"
  - "runSeam (internal/setup/apply.go) names a DeadlineExceeded seam error as 'timed out after 20s: context deadline exceeded' in the operator-facing row"
affects: [setup, cli, distribution]

# Actuals (#2632)
actuals:
  tokens: 3077
  tasks: 3
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "os.Args[0] re-exec idiom (mirrors internal/store/store_test.go:280-303) for giving a subprocess-exec seam a real child process to kill without invoking a third-party CLI"
    - "runSeam as the single wrap point for a bounded Environment.Run call, matching describeSeamError's existing sibling pattern"

key-files:
  created:
    - internal/setup/environment_test.go
  modified:
    - internal/setup/environment.go
    - internal/setup/apply.go
    - internal/setup/apply_test.go

key-decisions:
  - "D-10: osRun's ctx.Err() case returns the bare sentinel (covers both DeadlineExceeded and Canceled) rather than narrowing to a deadline-only check"
  - "D-11: the 'timed out after 20s' wording lives in runSeam, the sole owner of execTimeout, not in osRun or describeSeamError"
  - "D-12: the RunResult returned alongside a ctx error is the zero value — partial stdout/stderr captured before the kill is discarded, never surfaced"
  - "Renamed the helper's blocking-mode string from the plan's illustrative \"sleep\" to \"hang\" — Task 1's own acceptance criteria forbid the literal substring \"sleep\" appearing anywhere in environment_test.go (to prove no shell utility is invoked), which the plan's <behavior> prose used as its mode name; the semantic behavior (write a stdout line, then block past the deadline) is unchanged"

patterns-established:
  - "A subprocess-exec seam consults ctx.Err() as the FIRST classification check, before errors.As unwrapping — os/exec guarantees ctx.Done() closes strictly before CommandContext's kill fires, so no extra synchronization is needed"

requirements-completed: [REQ-osrun-deadline-error]

coverage:
  - id: D1
    description: "osRun returns the zero RunResult plus the bare ctx.Err() sentinel for a deadline-killed or cancelled real child process, and still returns an exit code with a nil error for a live-context nonzero exit"
    requirement: "REQ-osrun-deadline-error"
    verification:
      - kind: unit
        ref: "internal/setup/environment_test.go#TestOsRunReportsContextDeadlineExceeded"
        status: pass
      - kind: unit
        ref: "internal/setup/environment_test.go#TestOsRunReportsContextCanceled"
        status: pass
      - kind: unit
        ref: "internal/setup/environment_test.go#TestOsRunNonzeroExitStaysNilError"
        status: pass
    human_judgment: false
  - id: D2
    description: "runSeam names a timed-out seam error as 'timed out after 20s: context deadline exceeded' in the operator row, and leaves a cancelled seam error unwrapped as 'context canceled'"
    requirement: "REQ-osrun-deadline-error"
    verification:
      - kind: unit
        ref: "internal/setup/apply_test.go#TestDriftReportedLegibly/probe-seam-deadline-exceeded-names-timeout"
        status: pass
      - kind: unit
        ref: "internal/setup/apply_test.go#TestDriftReportedLegibly/probe-seam-canceled-passes-through-unwrapped"
        status: pass
    human_judgment: false

duration: 13min
completed: 2026-09-13
status: complete
---

# Phase 1 Plan 1: Executor Correctness — osRun Deadline Classification Summary

**`osRun` now reports `ctx.Err()` before unwrapping `*exec.ExitError`, so a deadline-killed or cancelled runtime CLI subprocess surfaces as a named timeout/cancellation instead of a misleading clean `exited -1`, and `runSeam` renders that timeout with its 20s duration in the operator-facing row.**

## Performance

- **Duration:** 13 min
- **Started:** 2026-09-13T17:47:00Z
- **Completed:** 2026-09-13T18:00:00Z
- **Tasks:** 3
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments
- `osRun`'s classification switch checks `ctx.Err() != nil` FIRST, returning the zero `RunResult{}` plus the bare context error (D-10/D-12) — proven against a REAL re-exec'd child process for both deadline-expiry and parent-cancellation, with a negative control proving a live-context nonzero exit is unaffected
- `runSeam` wraps only `context.DeadlineExceeded` as `timed out after %s: %w` (D-11), so the operator's row reads `<runtime>: <argv>: timed out after 20s: context deadline exceeded`; a cancellation passes through unwrapped as `<runtime>: <argv>: context canceled`
- Both fixes proven RED-then-GREEN: the two new `osRun` context tests failed with `err = <nil>` against unmodified `environment.go`, and the new `probe-seam-deadline-exceeded-names-timeout` subtest failed lacking the `timed out` prefix against unmodified `apply.go`, before each fix was applied

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end deadline classification — osRun returns ctx.Err() for a real deadline-killed child** - `0d0f561f` (fix)
2. **Task 2: runSeam names the timeout — the row reads "timed out after 20s: context deadline exceeded"** - `f35ed2de` (feat)
3. **Task 3: Plan gate — full `task`, license headers, zero go.mod drift, shuffle/flake check** - no commit (verification-only; no formatter or license fixer rewrote any of this plan's files)

**Plan metadata:** committed alongside this SUMMARY (see final commit).

_Note: both TDD tasks landed RED (test-only) and GREEN (fix) together in one commit each — the RED observation is recorded above and in each task's acceptance-criteria run, not split into a separate `test(...)` commit, matching this repo's own established precedent (03-01/03-05/03-06) for a one-case-addition fix._

## Files Created/Modified
- `internal/setup/environment.go` - `osRun`'s classification switch gains `case ctx.Err() != nil:` as its first arm, returning `RunResult{}, ctx.Err()`; doc comment rewritten to state the new contract and cite D-10/D-12/#560
- `internal/setup/environment_test.go` (new) - real-subprocess deadline/cancel/nonzero-exit tests for `osRun`, re-exec'ing the test binary itself via `os.Args[0]` (never a third-party CLI or `$HOME`)
- `internal/setup/apply.go` - `runSeam` wraps a `context.DeadlineExceeded` seam error with `timed out after %s: %w`; `"errors"` added to the import block; doc comment extended to name D-11
- `internal/setup/apply_test.go` - two new `TestDriftReportedLegibly` subtests pinning the exact timeout and cancellation row wording via the existing fake-`Environment` seam (no real 20s wait)

## Decisions Made
- Followed D-10/D-11/D-12 from `01-CONTEXT.md` exactly: bare `ctx.Err()` sentinel (not deadline-only), the timeout wording owned by `runSeam` (not `osRun` or `describeSeamError`), and the zero-value `RunResult` on any ctx error.
- Deviated from the plan's illustrative helper-mode string "sleep" to "hang" (see Deviations below) — a naming choice forced by the plan's own machine-checked acceptance criteria, with identical runtime semantics.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Helper mode string "sleep" conflicts with the task's own no-shell-utility acceptance-criteria grep**
- **Found during:** Task 1 (writing `environment_test.go` per `<behavior>`)
- **Issue:** The plan's `<behavior>` section names the blocking helper mode `"sleep"` (e.g. `t.Setenv(osRunHelperModeEnv, "sleep")`), but the SAME task's `<acceptance_criteria>` requires `rg -n -e 'exec\.Command\(|"sleep"|...' internal/setup/environment_test.go` to print nothing — a literal, machine-checked conflict, since a quoted Go string `"sleep"` used purely as a mode tag would still match that grep.
- **Fix:** Renamed the mode value to `"hang"` throughout (`osRunHelperModeEnv` values `"hang"`/`"exit3"`); the helper's actual behavior (write one line to stdout, then block 5s past the deadline) is unchanged from the plan's spec, and no doc comment or code references the literal substring `"sleep"`.
- **Files modified:** `internal/setup/environment_test.go`
- **Verification:** `rg -n -e 'exec\.Command\(|"sleep"|"claude"|"codex"|"opencode"|UserHomeDir' internal/setup/environment_test.go` prints nothing; both context tests still exercise the identical real-subprocess-kill behavior (verified RED-then-GREEN).
- **Committed in:** `0d0f561f` (Task 1 commit)

**2. [Rule 1 - Bug] `runSeam`'s extended doc comment initially referenced the literal string `context.Canceled`, conflicting with Task 2's own acceptance criteria**
- **Found during:** Task 2 acceptance-criteria verification
- **Issue:** `rg -n -F 'context.Canceled' internal/setup/apply.go` is required to print nothing ("Canceled is not special-cased in production code"), but the first draft of `runSeam`'s extended doc comment named `context.Canceled` in prose to explain why it is left unwrapped.
- **Fix:** Reworded the comment to describe "a plain cancellation ... the Canceled sentinel from the standard library's context package" without ever spelling the literal dotted identifier.
- **Files modified:** `internal/setup/apply.go`
- **Verification:** `rg -n -F 'context.Canceled' internal/setup/apply.go` prints nothing; `gofmt -l internal/setup/` clean; full package test still passes.
- **Committed in:** `f35ed2de` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 — literal wording adjustments forced by the plan's own machine-checked acceptance criteria; neither changes runtime behavior).
**Impact on plan:** No scope creep; both are cosmetic identifier choices made to satisfy the plan's own literal grep-based acceptance gates. All `must_haves.truths`, `artifacts`, and `key_links` from the plan frontmatter are satisfied as specified.

## Issues Encountered

**`task` (the repo's full lint+test gate, required by Task 3's `<verify>`) fails for reasons entirely outside this plan's `files_modified` scope (`internal/setup/*` only), confirmed pre-existing at the plan's starting commit (`9918f3af`) via a throwaway detached worktree, BEFORE any of this plan's changes:**

1. `internal/keylinks` `TestNoEscapedPatternsRepoWide` and `TestActiveMilestoneKeyLinksSatisfiable` fail against `.planning/phases/01-executor-correctness-man-pages/01-01-PLAN.md` and `01-02-PLAN.md`'s own `key_links.pattern` YAML fields (over-escaped regex illustrations) and a "0 plan files scanned" satisfiability gap. These are planning-artifact/tooling gates over files this plan must not hand-edit (`planning-artifacts` rule: never invent structure or hand-fix a tool-generated/tool-read file).
2. `internal/store` `TestRedEvidencePatchesAreLive` fails because `redEvidenceDirs` (in `internal/store/redevidence_harness_test.go`) is empty while phase `01-executor-correctness-man-pages` is active — this repo's established pattern (see prior-milestone commit `6ce098e0`, "register phase 01 red-evidence, unblocking CI") requires a phase that ships real RED evidence to register a `red-evidence/*.patch` directory + mapping. This plan DID ship real RED evidence (both TDD tasks observed genuine RED, recorded above and in each task's acceptance-criteria run), but authoring the patch files and touching `internal/store/redevidence_harness_test.go` is outside this plan's declared `files_modified` (`internal/setup/*` only) and outside the sequential-execution scope boundary given for this plan.

**Verified this plan's own contribution is fully clean:** `go test ./internal/setup/... -count=1` passes; `go vet ./internal/setup/` and `golangci-lint run ./internal/setup/...` report zero issues; `gofmt -l internal/setup/` is empty; `task license:check` passes (the new test file carries the SPDX header); `git diff --exit-code -- go.mod go.sum` is clean; `go test ./internal/setup/ -count=1 -shuffle=on` and `go test ./internal/setup/ -run '^TestOsRun' -count=5` both pass; `git status --porcelain -- internal/setup/` is empty after commits.

Both pre-existing gaps are recorded in `.planning/WINDOWS.md` (entries #8 and #9, kind `deviation`, phase `01`) for follow-up — most likely by whichever plan/step in this phase or a dedicated red-evidence-registration task closes them out, since they are phase-level (not plan-file-level) concerns.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `internal/setup`'s subprocess-exec seam now honors its own documented contract end-to-end; ready for `engram setup --apply` to correctly report a hung runtime CLI as a named timeout instead of a misleading clean failure (live post-merge observation only — never gated by a test, per rule `m45p2b4bp7`).
- Plan 01-02 (man pages) is independent and unblocked — no shared files with this plan.
- **Blocker for phase-level `task` green:** the two pre-existing gaps above (`internal/keylinks` plan-regex-escape gate, `internal/store` red-evidence registration) remain open in `.planning/WINDOWS.md` and will need a dedicated fix (likely touching `internal/store/redevidence_harness_test.go` + a new `red-evidence/` directory, and/or the plan-authoring tool that generated the `key_links.pattern` escaping) before phase 01's overall `task` gate is green.

---
*Phase: 01-executor-correctness-man-pages*
*Completed: 2026-09-13*

## Self-Check: PASSED
- `internal/setup/environment.go` — FOUND
- `internal/setup/environment_test.go` — FOUND
- `internal/setup/apply.go` — FOUND
- `internal/setup/apply_test.go` — FOUND
- Commit `0d0f561f` — FOUND in `git log --oneline --all`
- Commit `f35ed2de` — FOUND in `git log --oneline --all`
- All plan `<acceptance_criteria>` re-run and passing (Task 1: 8/8; Task 2: 8/8)
- Plan-level `<verification>` re-run: osRun 5x-count PASS, `TestDriftReportedLegibly` two new subtests PASS, `task license:check` PASS, `go.mod`/`go.sum` diff clean; `git diff --stat` between this plan's two commits touches exactly the four `files_modified` files; whole-repo `task` fails ONLY on pre-existing, out-of-scope gates (documented above and in WINDOWS.md)
