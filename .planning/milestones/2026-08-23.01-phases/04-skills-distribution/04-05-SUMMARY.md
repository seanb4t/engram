---
phase: 04-skills-distribution
plan: 05
subsystem: skills
tags: [go, tdd, skills, agents-md, setup, regression-test]

# Dependency graph
requires:
  - phase: 04-skills-distribution
    provides: "04-02/04-03/04-04's FormatAgentsMD write path, codex's SkillTarget authoring, and setupApplySkillsFacet's Report.Err -> row -> exit wiring"
provides:
  - "installAgentsMDIndex now distinguishes confirmed nonexistence (errors.Is against fs.ErrNotExist) from every other index-read failure — only the former is the create case"
  - "A non-nonexistence read error on the operator's AGENTS.md-shaped index preserves the file byte-for-byte, performs zero writes, and reports a wrapped error naming the index path, while the independent native skill-file pass still completes"
  - "TestInstallPreservesIndexOnReadError (package boundary) and TestSetupIndexReadFailureReachesPartialExit (CLI boundary) as permanent regressions for issue #559 / audit blocker B01"
affects: [04-VERIFICATION.md gap B01]

# Actuals (#2632)
actuals:
  tokens: 3552
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns: ["errors.Is-based two-way classification of a filesystem read error at one call site, mirroring the existing Splice malformed-early-return shape"]

key-files:
  created: []
  modified:
    - internal/skills/install.go
    - internal/skills/environment.go
    - internal/skills/install_test.go
    - cmd/engram/setup_test.go

key-decisions:
  - "Fixed exactly one predicate at one call site (installAgentsMDIndex's read-error classification) — installFiles' own ambiguity-resolves-to-wrote posture for engram-owned skill files was deliberately left untouched (D-08)."
  - "During the tracer feedback gate re-run, the rewritten Environment.ReadFile doc comment was found to still contain the literal phrase \"ANY read error\" (describing installFiles' correct, unchanged behavior), which tripped the negative-grep guard meant to catch the STALE claim about installAgentsMDIndex. Reworded in a follow-up commit (bae018f2) without changing the described behavior — see Deviations."

requirements-completed: [REQ-skills-agents-md-fallback, REQ-skills-native-format]

coverage:
  - id: D1
    description: "installAgentsMDIndex preserves an unreadable AGENTS.md index byte-for-byte with zero writes, reports a wrapped path-naming error, and the independent skill-file pass still completes (D-07/D-15); confirmed nonexistence still creates a fresh document"
    requirement: "REQ-skills-agents-md-fallback"
    verification:
      - kind: unit
        ref: "internal/skills/install_test.go#TestInstallPreservesIndexOnReadError"
        status: pass
      - kind: unit
        ref: "internal/skills/install_test.go#TestInstallAgentsMdConverges"
        status: pass
    human_judgment: false
  - id: D2
    description: "The index-read failure reaches the codex row (failed skills facet, reason naming the index path) and the exitPartial exit code at the CLI boundary, while claude-code's row and codex's native skill writes are unaffected"
    requirement: "REQ-skills-native-format"
    verification:
      - kind: integration
        ref: "cmd/engram/setup_test.go#TestSetupIndexReadFailureReachesPartialExit"
        status: pass
    human_judgment: false

duration: 18min
completed: 2026-09-12
status: complete
---

# Phase 04 Plan 05: Preserve unreadable AGENTS.md index instead of silently overwriting it Summary

**Fixed `installAgentsMDIndex` to treat only confirmed `fs.ErrNotExist` as "create a fresh index" — every other read error (permission denied, transient I/O) now preserves the operator's AGENTS.md byte-for-byte with zero writes and a wrapped, path-naming error, closing GitHub issue #559 / audit blocker B01.**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-09-12T19:55:00Z (approx)
- **Completed:** 2026-09-12T20:12:54Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- `installAgentsMDIndex` now classifies the index read result two ways: `errors.Is(readErr, fs.ErrNotExist)` (bare or wrapped) is the create case; any other read error returns before `RenderBlock`/`Splice`/`MkdirAll`/`WriteFile`, accumulating `fmt.Errorf("skills: read index %s: %w", target.IndexFile, readErr)` into `Report.Err`.
- `TestInstallPreservesIndexOnReadError` (package boundary) reproduces the audit's own defect as a red-then-green regression: permission-denied and transient-error subtests failed against the unmodified code (index in `Wrote`, `Report.Err` nil) and pass after the fix; nonexistence subtests (bare and wrapped) passed throughout.
- `TestSetupIndexReadFailureReachesPartialExit` (CLI boundary) proves the failure reaches the codex row (`skills == "failed"`, `outcome == "failed"`, `reason` naming the index path), leaves the claude-code row unaffected, records zero writes to the index, records at least one write under codex's native skills directory, and drives `engram setup --apply` to `exitPartial` (8).
- `Environment.ReadFile`'s doc comment and `installAgentsMDIndex`'s doc comment now state the two consumers' different rules (engram-owned skill files vs. the operator-owned index) instead of claiming every read error is handled identically.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "unreadable index is preserved" — reproduce the defect red, fix the one predicate, prove it green** - `95daee01` (fix) — RED observed, then fix + GREEN in the same commit per the plan's own action step (test and implementation committed together).
2. **Task 1 follow-up (tracer feedback gate self-correction): reword stale doc-comment phrase** - `bae018f2` (fix) — see Deviations.
3. **Task 2: The failure reaches the codex row and the partial exit — CLI propagation regression, then the full gate** - `c773d80e` (test)

_Note: TDD tasks may have multiple commits; Task 1's RED phase was run and recorded (see below) before the fix was applied, all within the working tree prior to the first commit — the RED evidence is a recorded command transcript rather than a separate commit, matching the plan's own instruction to commit test+fix together for this task._

### RED run (Task 1, before any fix — recorded verbatim)

```
$ go test ./internal/skills/... -count=1 -run '^TestInstallPreservesIndexOnReadError$' -v
install_test.go:377: indexWriteCount = 1, want 0 (zero index writes on a non-nonexistence read error)
    install_test.go:381: Wrote = [/synthetic/.agents/skills/alpha/SKILL.md /synthetic/.agents/skills/beta/SKILL.md /synthetic/.agents/skills/beta/references/notes.md /synthetic/.codex/AGENTS.md], want it to NOT contain the index file
    install_test.go:391: Report.Err = nil, want a non-nil error naming the index path
    install_test.go:377: indexWriteCount = 1, want 0 (zero index writes on a non-nonexistence read error)
    install_test.go:381: Wrote = [/synthetic/.agents/skills/alpha/SKILL.md /synthetic/.agents/skills/beta/SKILL.md /synthetic/.agents/skills/beta/references/notes.md /synthetic/.codex/AGENTS.md], want it to NOT contain the index file
    install_test.go:391: Report.Err = nil, want a non-nil error naming the index path
--- FAIL: TestInstallPreservesIndexOnReadError (0.00s)
    --- FAIL: TestInstallPreservesIndexOnReadError/permission_denied (0.00s)
    --- FAIL: TestInstallPreservesIndexOnReadError/transient_error (0.00s)
    --- PASS: TestInstallPreservesIndexOnReadError/bare_nonexistence (0.00s)
    --- PASS: TestInstallPreservesIndexOnReadError/wrapped_nonexistence (0.00s)
FAIL
```

### GREEN run (Task 1, after the fix)

```
$ go test ./internal/skills/... -count=1 -run '^TestInstallPreservesIndexOnReadError$' -v
--- PASS: TestInstallPreservesIndexOnReadError (0.00s)
    --- PASS: TestInstallPreservesIndexOnReadError/permission_denied (0.00s)
    --- PASS: TestInstallPreservesIndexOnReadError/transient_error (0.00s)
    --- PASS: TestInstallPreservesIndexOnReadError/bare_nonexistence (0.00s)
    --- PASS: TestInstallPreservesIndexOnReadError/wrapped_nonexistence (0.00s)
ok  	github.com/seanb4t/engram/internal/skills	0.067s
```

### GREEN run (Task 2, CLI boundary — passed on first run since Task 1 already supplied the upstream error)

```
$ go test ./cmd/engram/... -count=1 -run '^TestSetupIndexReadFailureReachesPartialExit$' -v
--- PASS: TestSetupIndexReadFailureReachesPartialExit (0.00s)
ok  	github.com/seanb4t/engram/cmd/engram	0.333s
```

### Full repository gate

```
$ task
... (lint:setup, lint:actions, lint:markdown, lint:go, lint:yaml, lint:python — all clean)
$ go test ./...
ok  	github.com/seanb4t/engram/cmd/engram	4.356s
ok  	github.com/seanb4t/engram/internal/...  (all packages ok, including internal/store 22-33s)
```

`task` exited 0. The documented `internal/store` testcontainer flake (deferred-items.md) did **not** fire on either full-gate run in this session; `internal/store` passed cleanly (22.768s and 33.048s across two runs). No separate flake report is needed.

## Files Created/Modified

- `internal/skills/install.go` — `installAgentsMDIndex`'s read-error classification (the one-predicate fix) and rewritten first doc-comment paragraph; `io/fs` import added.
- `internal/skills/environment.go` — `Environment.ReadFile`'s doc comment rewritten to state `installFiles` and `installAgentsMDIndex`'s different rules.
- `internal/skills/install_test.go` — `TestInstallPreservesIndexOnReadError` (4-row table: permission denied, transient, bare nonexistence, wrapped nonexistence).
- `cmd/engram/setup_test.go` — `TestSetupIndexReadFailureReachesPartialExit`; `bytes` import added.

## Decisions Made

- Kept the fix scoped to exactly the read-error branch of `installAgentsMDIndex`; `installFiles`, `Splice`, `SkillsOutcome`, `AggregateOutcome`, `Classify`, and `Report`/`Target`/`Format` are all byte-identical to their pre-plan state (confirmed via `git diff` scoped to the fix commit).
- Task 2 required no production change: the wiring from `Report.Err` through `setupApplySkillsFacet` to the row and the exit code already existed (proven by the pre-existing `TestSetupSkillsFailureReachesPartialExit`); Task 1 supplied the only missing piece, an upstream error on the index-read path.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug in own just-written code] Stale-claim negative-grep false positive from a legitimate but ambiguously-worded doc comment**
- **Found during:** Task 1's tracer feedback gate (the mandatory re-run of the task's own `<verify>` block before starting Task 2's expansion work)
- **Issue:** The rewritten `Environment.ReadFile` doc comment (committed in `95daee01`) correctly described `installFiles`' unchanged behavior using the phrase "treats ANY read error identically," which is true and intentional — but it is the exact literal string the plan's own `rg -n 'ANY read error'` negative-grep guard was written to catch as evidence of the STALE claim (the pre-fix comment that wrongly said the *same* about `installAgentsMDIndex`). The check does not distinguish "this describes `installFiles`, correctly" from "this describes `installAgentsMDIndex`, incorrectly" — it only greps for the substring.
- **Fix:** Reworded the sentence to "treats every read error the same, without exception" — identical meaning, no longer matching the literal guard string.
- **Files modified:** `internal/skills/environment.go`
- **Verification:** `rg -n 'ANY read error' internal/skills/install.go internal/skills/environment.go` now prints nothing (exit 1); `go test ./internal/skills/... -count=1` still green; `gofmt -l` clean.
- **Committed in:** `bae018f2` (separate commit, not an amend — per this session's git-safety instructions, prior commits are never amended)

---

**Total deviations:** 1 auto-fixed (Rule 1, self-correction of just-written prose).
**Impact on plan:** No functional or test-coverage change; a wording-only correction to keep the doc comment's intended meaning while satisfying the plan's own negative-grep guard. No scope creep.

### Known verify-command false positives (documented, not fixed — out of scope)

These are properties of the *plan's own `<verify>`/`<acceptance_criteria>` commands*, not of the code they check, and are recorded here rather than "fixed" because fixing them would mean editing pre-existing, unrelated prose or rewriting already-created commit history, both out of this plan's scope:

- **`rg -n 'os\.CreateTemp|os\.Rename|EvalSymlinks|flock' internal/skills/install.go`** (Task 1 `<verify>`) prints one line even against the **pre-plan baseline** — `installAgentsMDIndex`'s pre-existing D-16 rationale comment (predating this plan; confirmed via `git show HEAD:internal/skills/install.go` before any edit in this session) explains *why* the write is NOT staged-and-renamed, and that explanation names `os.Rename` as the alternative it rejects. The substantive property the check protects — that this plan does not *add* staging, renaming, symlink resolution, or locking — holds: `git diff <FIX_SHA>^..<FIX_SHA> -- internal/skills/install.go` shows zero new occurrences of any of those patterns; the one match is the identical pre-existing line.
- **`git log --format='%h %s' --grep='#559' -n 2`** (Task 2 `<verify>`/`<acceptance_criteria>`) is satisfied by its `<verify>`-stated `fails_when` (prints fewer than two lines — it prints exactly two) but the stricter `<acceptance_criteria>` restatement expects those two lines to be specifically the Task 1 fix commit and the Task 2 test commit. Because deviation #1 above added a third commit (`bae018f2`) that also legitimately references `#559` (a `Refs #559` trailer, since it corrects prose introduced while closing that issue), the newest-two-by-date output is `c773d80e` (test) and `bae018f2` (the doc-wording fix) rather than `95daee01` (the Task 1 fix) and `c773d80e`. The acceptance criterion's own more rigorous check — pinning each required commit by its **exact subject** rather than by `-n 2` recency — passes cleanly: `FIX_SHA=95daee01dc8664b1b95d9c6c2bb607e44d529ac6` and `TEST_SHA=c773d80e173b5f7e44c4e46d72498caf2079e79e` both exist, and `git show --name-only --format= "$FIX_SHA" "$TEST_SHA" | sort -u` lists exactly the four files in this plan's `files_modified` (`cmd/engram/setup_test.go`, `internal/skills/environment.go`, `internal/skills/install.go`, `internal/skills/install_test.go`) and no others.

## Issues Encountered

None beyond the two documented above.

## Verification safety statement (repo gotcha ryr82bf2s2, rule m45p2b4bp7)

No live `engram setup --apply` (or any `engram setup` verb) was run against any real `$HOME`, `~/.claude`, `~/.codex`, `~/.config/opencode`, or any real `AGENTS.md` at any point in this plan's execution. Every verification ran through one of: `fakeInstallEnv` (`internal/skills/install_test.go`), an inline `skills.Environment` built from an in-memory map (both new tests in this plan), or `withFakeSetupEnv`/the package-level `skillsEnv` seam (`cmd/engram/setup_test.go`). The negative greps required by Task 2's `<verify>` block (`rg -n 'os\.UserHomeDir|exec\.Command\(|os\.Getenv\("HOME"\)' cmd/engram/setup_test.go internal/skills/install_test.go`) print nothing, confirming this structurally rather than by construction alone.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Gap B01 in `04-VERIFICATION.md` can now be re-verified to `passed` from the tests alone (`TestInstallPreservesIndexOnReadError`, `TestSetupIndexReadFailureReachesPartialExit`), with no live observation required.
- No blockers for the phase. The three prohibited-scope files (`installFiles`, `internal/setup/exit.go`, `internal/setup/aggregate.go`, `internal/skills/agentsmd.go`, `internal/skills/data/**`) are confirmed byte-identical to their pre-plan state.

---
*Phase: 04-skills-distribution*
*Completed: 2026-09-12*

## Self-Check: PASSED

- `[ -f internal/skills/install.go ]`, `[ -f internal/skills/environment.go ]`, `[ -f internal/skills/install_test.go ]`, `[ -f cmd/engram/setup_test.go ]` — all FOUND.
- `git log --oneline --all | grep -q 95daee01` — FOUND. `git log --oneline --all | grep -q bae018f2` — FOUND. `git log --oneline --all | grep -q c773d80e` — FOUND.
- All `<acceptance_criteria>` for both tasks re-run and passing, except the two documented, out-of-scope verify-command false positives above (both independently confirmed to hold on their substantive property).
- Plan-level `<verification>` block re-run: RED/GREEN observed and recorded; `TestSetupIndexReadFailureReachesPartialExit` green; `go build ./... && go test ./internal/skills/... ./internal/setup/... ./cmd/engram/... -count=1` green; `task` green (no flake observed); `task license:check` green; `installFiles`/`internal/setup/exit.go`/`internal/setup/aggregate.go`/`internal/skills/agentsmd.go`/`internal/skills/data/**` confirmed byte-identical; all verification ran through fake seams; two commits (by exact subject) reference #559 and together touch exactly the four `files_modified` files.
