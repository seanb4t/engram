---
phase: 01-executor-correctness-man-pages
reviewed: 2026-09-13T23:45:00Z
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
  info: 0
  total: 1
status: issues_found
---

# Phase 01: Code Review Report (iteration 3, final re-review)

**Reviewed:** 2026-09-13T23:45:00Z
**Depth:** standard
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Final re-review of the auto-fix commit `553e77b2`, the last of three fix
iterations. Verified via `git diff aedc6d54 HEAD` that the entire delta since
iteration 2 is two doc-comment-only edits, exactly as the fix report claims:

- `internal/setup/environment.go`: `osRun`'s comment above the (unchanged)
  classification `switch` now states the accepted residual explicitly — a
  genuine, non-killed nonzero exit racing the deadline is still reported as
  the ctx error rather than its real exit code — instead of asserting (as the
  iteration-2 doc did) that this case was already safely handled.
- `cmd/engram/man.go`: `manHeader()`'s comment no longer claims returning a
  fresh struct "avoids any aliasing surprise across callers"; it now states
  correctly that `Date` intentionally keeps aliasing the single
  package-level `manDate`, because that value is set once at init and never
  mutated.

No executable code changed in this delta (confirmed by reading the diff
hunks: only comment lines were touched in both files), so no new
control-flow defect can have been introduced by this commit.

**Prior findings, both closed:**
- Iteration-2 WR-01 (doc overclaimed the reordered switch was fully safe) —
  closed. The orchestrator's recorded decision is that the switch ordering
  is the intended, permanent design (the `ExitCode() == -1` disambiguation
  the iteration-2 review itself proposed as a fix is platform-dependent —
  Windows reports killed processes as exit code `1`, not `-1` — so it cannot
  reliably distinguish the two cases either). Per the task brief, this
  residual is not re-raised here.
- Iteration-1/2 IN-01 (`manHeader()`'s comment overstated what aliasing
  freshness protects against) — closed by the man.go edit above; the new
  wording accurately describes what is and is not freshened per call.

**Verification beyond static reading:**
- `go test ./internal/setup/ ./cmd/engram/ -count=1` — both `ok`.
- Re-read `internal/setup/apply.go`'s `execute()` in full against the new
  comment's specific claim (see WR-01 below) rather than accepting the
  claim at face value.

No security issues, hardcoded secrets, or injection vectors were found.

## Warnings

### WR-01: The new "accepted residual" comment understates its own blast radius — it is not true that the residual "only changes the reported Reason" at every call site

**File:** `internal/setup/environment.go:98-114` (doc comment only; `osRun`'s
code, `:115-137`, is unchanged from iteration 2 and is explicitly not
re-litigated here per the task brief).

**Issue:** The new comment claims:

> `runSeam` (apply.go) classifies both outcomes as `Outcome == OutcomeFailed`,
> so the residual only changes the reported Reason (deadline text vs.
> exit-code text)

This is true for exactly one of `runSeam`'s four call sites in
`internal/setup/apply.go`'s `execute()` — a **non-tolerant** write `Action`'s
nonzero exit, where both the raced (seam-error) and un-raced (nonzero-exit)
paths land on `OutcomeFailed` with only the `Reason` text differing
(`describeSeamError` vs. `describeFailure`). It is false at the other three
sites, where the residual changes the **Outcome itself**, not just prose:

1. **Probe #1 under `Apply` (`apply.go:318-322`).** A probe's genuine
   nonzero exit (no race) is *not* a failure at all — `probe1Err == nil`
   means execution falls through and proceeds to run the write actions
   normally, with `Outcome` eventually settling as `OutcomeWrote` or
   `OutcomeAlreadyCorrect`. But if that same genuine nonzero exit races the
   deadline, `probe1Err != nil` and the row is failed immediately:
   `res.Outcome = OutcomeFailed`. The residual turns a normal,
   non-failing run into a hard failure here — not a wording change.

2. **A `Tolerant` write `Action`'s nonzero exit (`apply.go:326-336`).**
   `case runErr != nil` is checked before `case action.Tolerant`, and it
   does not consult `action.Tolerant` at all. A genuine nonzero exit on a
   tolerant action (no race) is appended to `Notes` and the sequence
   continues — never `OutcomeFailed`. If that same exit races the deadline,
   it is misclassified as a seam error and unconditionally fails the row
   via `case runErr != nil`, bypassing the tolerance the action's author
   deliberately authored (`claudecode.go`'s tolerant `mcp remove` is exactly
   this shape in production). Again, an `Outcome` change, not a `Reason`
   change — and in this case a materially worse one, since it defeats the
   entire purpose of `Action.Tolerant`.

3. **Probe #2 (`apply.go:370-382`).** A raced probe-2 seam error resolves to
   `OutcomeWrote` ("ambiguity resolves to wrote, never to already-correct" —
   neither `OutcomeFailed` as the comment claims). A genuine, un-raced
   nonzero-exit probe-2 instead proceeds to the byte-compare against
   probe-1's capture and could land on `OutcomeAlreadyCorrect` if the two
   captures happen to match. So here too the residual can change `Outcome`
   (`AlreadyCorrect` -> `Wrote`), and neither branch is `OutcomeFailed`,
   contradicting the comment's specific wording a second, independent way.

The comment's blanket claim ("classifies both outcomes as
`Outcome == OutcomeFailed`... only changes the reported Reason") is
therefore accurate for only 1 of 4 call sites and actively misleading for
the other 3, including the `Tolerant`-action case, which is a normal,
expected code path in this codebase (every `claude-code` `Apply` run
exercises it). A maintainer relying on this comment's stated scope would
reasonably (and wrongly) conclude the residual is a cosmetic wording
difference everywhere it can occur.

This is a documentation-accuracy issue only — the underlying switch
ordering is out of scope per the task brief's orchestrator decision, and no
executable code changed in this delta. It is a Warning because the residual
itself was already accepted as tolerable risk; what's being flagged is that
the written rationale for accepting it does not match the code it describes,
which will mislead the next person who reads it (e.g., when deciding whether
a future change to `execute()`'s tolerant-action handling is safe).

**Fix:** Narrow the comment's claim to the one call site where it actually
holds, and name the other three explicitly rather than generalizing from the
first. For example, replace the "runSeam... classifies both outcomes..."
sentence with something like:

```go
// Accepted residual (WR-01 iteration 2, 01-REVIEW.md): a genuine, non-killed
// nonzero exit that lands at essentially the same instant the deadline
// independently fires is still reported as the ctx error rather than its
// real exit code — osRun's ctx.Err() read is a separate, unsynchronized
// check from what cmd.Run() internally decided, so this ordering cannot
// distinguish "killed by us" from "exited on its own, right at the
// boundary." This is deliberate, not an oversight, but its impact varies by
// call site in apply.go's execute(): for a non-tolerant write Action's
// nonzero exit, both the raced and un-raced paths already resolve to
// OutcomeFailed, so the residual only changes the reported Reason text. At
// the other three runSeam call sites — probe #1 under Apply, a Tolerant
// Action's nonzero exit, and probe #2 — the residual changes the
// classified Outcome itself (a probe or tolerated failure can be promoted
// to a hard OutcomeFailed, or an already-correct probe-2 comparison can be
// forced to OutcomeWrote), not merely its wording. The deadline reason is
// still judged the more actionable text for an operator to see in the
// non-tolerant-write case; the other three cases carry a real, if narrow,
// availability cost that this comment does not paper over.
```

Whether to also change behavior (e.g., have the write-action loop consult
`action.Tolerant` even on a seam error, so a tolerant action's raced failure
is tolerated rather than promoted) is a separate design decision outside
this iteration's scope; this finding is only about making the comment's
claim match the code.

---

_Reviewed: 2026-09-13T23:45:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
