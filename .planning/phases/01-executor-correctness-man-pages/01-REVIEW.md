---
phase: 01-executor-correctness-man-pages
reviewed: 2026-09-13T18:42:30Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - internal/setup/environment.go
  - internal/setup/environment_test.go
  - internal/setup/apply.go
  - internal/setup/apply_test.go
  - cmd/engram/man.go
  - cmd/engram/man_test.go
  - cmd/engram/releaseconfig_test.go
  - .goreleaser.yaml
  - internal/store/redevidence_harness_test.go
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-09-13T18:42:30Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Reviewed the osRun deadline-classification fix (`internal/setup/environment.go`,
`apply.go`), the new hidden `engram man` command and its cobra-tree
snapshot/restore mechanism (`cmd/engram/man.go`), the Homebrew cask
post-install/uninstall man-page hooks (`.goreleaser.yaml`) and their
ordering/shape gate (`releaseconfig_test.go`), and the phase's red-evidence
registration (`internal/store/redevidence_harness_test.go`).

Verification performed beyond static reading:
- Ran `go test ./internal/store/ -run TestRedEvidencePatchesAreLive -v`
  (full run, not `-short`) end to end: all four registered patches
  (`01-01-osrun-ctx-err-first.patch`, `01-01-runseam-timeout-wording.patch`,
  `01-02-man-header-pinned.patch`, `01-02-man-tree-restore.patch`) applied
  cleanly, drove their mapped target test RED as claimed, and reverted
  cleanly (`git status --porcelain` empty afterward). The phase's central
  claim — "TDD RED was proven against real source for these two
  regressions" — is independently confirmed, not merely asserted.
- Ran the full `internal/setup` and relevant `cmd/engram` test subsets
  (`TestOsRun*`, `TestDriftReportedLegibly`, `TestApplyConverges*`,
  `TestManPages*`, `TestManCmd*`, `TestManGeneration*`,
  `TestReleaseConfig*`): all pass.
- Confirmed by grep that `runSeam` is the only non-test call site of
  `Environment.Run` in `internal/setup`, and that no `cmd/engram` code
  bypasses `Apply`/`Preview` to call `env.Run` directly — the doc comment's
  "single place a deadline is named" claim holds structurally, not just by
  assertion.
- Manually traced `os/exec`'s `Cmd.Wait()`/`watchCtx` semantics against the
  new `ctx.Err() != nil` case in `osRun` to check for a race between a
  genuinely successful run and a concurrently-firing context deadline (see
  WR-01 below).

No security issues, hardcoded secrets, or injection vectors were found.
The only defect worth recording is a narrow, low-probability
misclassification race in the new `osRun` ordering, which is a correctness
edge case rather than something the test suite currently exercises.

## Warnings

### WR-01: `osRun`'s unconditional `ctx.Err()` check can discard a genuinely successful run under a boundary-timing race

**File:** `internal/setup/environment.go:110-112`
**Issue:** The new first `switch` case —

```go
case ctx.Err() != nil:
    return RunResult{}, ctx.Err()
```

— is evaluated regardless of whether `runErr` was `nil`. Per `os/exec`'s own
`Cmd.Wait()`/`watchCtx` implementation, when the child process exits
naturally (success or ordinary nonzero exit) essentially at the same
wall-clock instant the context's deadline timer independently fires, the
two are not synchronized against each other: `cmd.Run()` can legitimately
return `nil` (or a real `*exec.ExitError`) for a run whose child was never
actually killed, while `ctx.Err()` — checked microseconds later, purely by
wall-clock proximity to the deadline — has *also* just become non-nil.
Because this new case runs before `case runErr == nil`, that scenario is
folded into "never got an answer": `RunResult{}` and `ctx.Err()` are
returned, discarding the real (and valid) stdout/stderr/exit status.

For a runtime write action, this converts a real, successful registration
into an operator-visible `OutcomeFailed: timed out after 20s`, which is
strictly worse than the pre-fix behavior for this specific slice of cases
(the pre-fix bug misreported a *genuinely killed* child as a clean nonzero
exit; this introduces the mirror-image risk of misreporting a *genuinely
successful* child as killed). In practice the window is vanishingly small
given the fixed 20s `execTimeout` and the ~2s real-world completion times
the package's own comments cite, so this is not a blocker — but it is a
real, provable gap the new tests do not exercise (both
`TestOsRunReportsContextDeadlineExceeded`/`TestOsRunReportsContextCanceled`
only assert the case where the child is still genuinely running/blocked at
the deadline, never the boundary case where it finishes right at it).

**Fix:** Narrow the new case so it only fires when there was otherwise no
clean success to report, e.g. gate it on `runErr != nil`:

```go
switch {
case runErr == nil:
    return result, nil
case ctx.Err() != nil:
    return RunResult{}, ctx.Err()
case errors.As(runErr, &exitErr):
    ...
}
```

This still catches GitHub #560's case (a SIGKILLed child surfaces as a
non-nil `*exec.ExitError`, so `runErr != nil` holds), while no longer
risking discarding a run that `os/exec` itself already concluded was clean.
If the team's intent is specifically to also treat a "successful but the
deadline fired concurrently" run as ambiguous-favor-safety (mirroring
D-08's "ambiguity resolves to wrote, never already-correct" policy
elsewhere in this package), that should be stated explicitly in the doc
comment and covered by a dedicated test that forces the boundary race
(e.g. a helper that exits right as the deadline elapses), rather than left
implicit.

## Info

### IN-01: `manHeader()`'s doc comment overstates what "returning a new value each call" protects against

**File:** `cmd/engram/man.go:30-47`
**Issue:** The comment says returning a fresh `*doc.GenManHeader` per call
"avoids any aliasing surprise across callers," but `Date: &manDate` still
takes the address of the single package-level `manDate` variable on every
call — the returned struct is new, but its `Date` field always aliases the
same shared pointer. This is harmless today because `manDate` is never
mutated after package init, but the comment's stated rationale doesn't
match what's actually protected (the new-struct-per-call is protecting
against `doc.GenManTree`'s shallow-copy-per-file mutating some *other*
field of the header struct across files, not the `Date` field's aliasing).
**Fix:** Either take a local copy of the time value (`d := manDate; ...
Date: &d`) so the comment's claim is literally true, or reword the comment
to clarify that `Date`'s aliasing is intentionally shared (since the value
is immutable) and only the struct itself is freshened per call.

---

_Reviewed: 2026-09-13T18:42:30Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
