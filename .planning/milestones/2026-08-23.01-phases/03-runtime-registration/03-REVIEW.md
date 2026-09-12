---
phase: 03-runtime-registration
reviewed: 2026-09-09T00:00:00Z
depth: standard
files_reviewed: 22
files_reviewed_list:
  - cmd/engram/setup.go
  - cmd/engram/setup_test.go
  - cmd/engram/testdata/catalog.golden
  - cmd/engram/testdata/help.golden
  - internal/keylinks/keylinks.go
  - internal/keylinks/keylinks_test.go
  - internal/setup/apply.go
  - internal/setup/apply_test.go
  - internal/setup/claudecode.go
  - internal/setup/claudecode_test.go
  - internal/setup/codex.go
  - internal/setup/detect_test.go
  - internal/setup/environment.go
  - internal/setup/generic.go
  - internal/setup/generic_test.go
  - internal/setup/opencode.go
  - internal/setup/opencode_test.go
  - internal/setup/plan.go
  - internal/setup/plan_test.go
  - internal/setup/quote.go
  - internal/setup/quote_test.go
  - internal/setup/runtime.go
  - internal/surfaces/toolclass.go
findings:
  critical: 1
  warning: 4
  info: 1
  total: 6
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-09-09
**Depth:** standard
**Files Reviewed:** 22
**Status:** issues_found

## Summary

Reviewed the runtime-registration implementation (`internal/setup`, `cmd/engram/setup.go`,
`internal/keylinks`, `internal/surfaces/toolclass.go`) against the design invariants stated
in the task brief: stdlib-only imports, argv-authored `Action.Command()`, no literal
credential in `Args`, no read of `--token-file`, no stderr-content branching, authored
(not positional) `Tolerant`, and the deliberate per-runtime asymmetry (codex single
overwrite, opencode no-remove/live-dialing probe, claude-code tolerant-remove-then-fatal-add,
generic zero-Action opt-in). All of these invariants hold under direct inspection and are
independently pinned by tests — I did not find a violation of any of them.

The one genuine correctness defect is in the shared executor (`internal/setup/apply.go`):
the empty-`Args` defensive check that guards `env.LookPath` only covers `plan.Actions[0]`,
not every action in the (documented-as-growable) `Actions` slice, so a later action with an
empty `Args` slice panics the whole `execute()` call — and nothing recovers from that panic
anywhere up the call chain, so one runtime's malformed `Plan` would crash the entire
`engram setup --apply` invocation, not just that runtime's own row. This directly
contradicts the executor's own stated goal ("one runtime's failure never prevents another's
attempt or its row from rendering") the moment `Plan.Actions` grows past one element in the
way the package's own doc comments say a later phase (skills distribution) will do.

The remaining findings are lower-severity robustness/defense-in-depth gaps: an unenforced
assumption that every action/probe in a `Plan` names the same binary as `Actions[0]`, and a
new (Phase 3) increase in the surface of the already-known/accepted issue #523 (raw
third-party CLI output now flows into the rendered report via `Registered`/`Notes` with only
control-character stripping, no shell-metacharacter quoting).

## Critical Issues

### CR-01: Panic on any non-first `Action` with an empty `Args` slice

**File:** `internal/setup/apply.go:217-320`
**Issue:** `execute()` validates that `Args` is non-empty only for `plan.Actions[0]`
(line 225: `if len(plan.Actions[0].Args) == 0 { ... OutcomeFailed ... }`), then resolves
`binary` from that first action's `Args[0]` (line 232). Every subsequent action is executed
unconditionally at line 286:

```go
for _, action := range plan.Actions {
    rr, runErr := runSeam(ctx, env, binary, action.Args[1:])
```

If any action **after** the first one carries a `nil` or empty `Args` (e.g. a bug in a future
runtime's `Plan()`, or in a later phase's skills-distribution action — `plan.go`'s own doc
comment states `Plan.Actions` is "growable by a later phase" precisely for that reason),
`action.Args[1:]` panics with `slice bounds out of range [1:0]`. This is not defensive
code that degrades to `OutcomeFailed` — it crashes the Go process outright. Because
`setupApplyRun` (`cmd/engram/setup.go:334-369`) calls `setup.Apply` once per selected
runtime in a plain loop with no `recover()` anywhere in the call chain, a single
malformed `Plan` from ONE runtime takes down the entire multi-runtime `--apply` invocation,
losing every other runtime's already-computed row along with it — the exact failure mode
the executor's own doc comment (apply.go's package comment on step 7, and D-03's
errors.Join-style precedent) says must never happen.

Not reachable via any of the four shipped runtimes today (every one of them authors
uniformly non-empty `Args` for every action), so this is currently latent — but it is a real,
provable defect in the shared executor's defensive completeness, not a hypothetical: a
`Plan{Actions: []Action{{Args: []string{"x"}}, {Args: nil}}}` run through `Apply()` panics.

**Fix:** Validate every action's `Args`, not just the first, before entering the loop (or
inside it, before slicing):
```go
for i, action := range plan.Actions {
    if len(action.Args) == 0 {
        return Result{
            Runtime: name, Present: true, Outcome: OutcomeFailed,
            Reason: fmt.Sprintf("%s: Plan() authored action %d with no Args", name, i),
        }
    }
}
```
placed once, before the `env.LookPath` call, so both the pre-flight binary resolution and
the execution loop are covered by a single check over the whole slice.

## Warnings

### WR-01: Later actions' and Probe's binary identity is assumed, never verified

**File:** `internal/setup/apply.go:181-184, 232, 262, 330`
**Issue:** `plan.go`'s doc comment states "`Probe[0]` is the bare binary name, resolved
exactly like an Action's `Args[0]` at exec time." In practice, `execute()` never reads
`Probe[0]` (or any action's `Args[0]` beyond the first) for resolution purposes at all — the
single `binary` value resolved from `plan.Actions[0].Args[0]` (line 232) is silently reused
for `Plan.Probe` (lines 262, 330) and for every subsequent `Action` in the loop (line 286).
Every shipped runtime happens to name the same bare binary in every action and its probe, so
this is safe today, but nothing enforces the invariant: a future runtime whose `Probe` or a
later `Action` names a *different* binary than `Actions[0]` would silently execute the wrong
resolved path under the previous runtime's identity, with no error and no test to catch it.
**Fix:** Either assert `action.Args[0] == plan.Actions[0].Args[0]` (and `plan.Probe[0] ==`
the same) for every element before executing, returning `OutcomeFailed` on mismatch, or
resolve each action's own `Args[0]` independently via `env.LookPath` rather than reusing one
cached `binary` value.

### WR-02: `Registered`/`Notes` route raw third-party CLI output into the operator report unquoted, widening issue #523's surface

**File:** `internal/setup/apply.go:273, 296, 336`; `internal/setup/plan.go:184-198`
**Issue:** Phase 3 introduces two new `Result` fields carrying **raw, third-party-controlled**
text into the rendered operator report: `Registered` (bounded `Plan.Probe` stdout+stderr,
D-10) and `Notes` (a tolerant action's own captured stderr, via `toleratedNote`). Both flow
through the same `viewRow`/`sanitizeViewValue` text-rendering path as every other field
(`cmd/engram/operator_view.go`), which strips only C0 control characters and DEL — it applies
no shell-metacharacter quoting. `Command`, by contrast, IS quoted (`quoteArgs`/`quoteWord` in
`quote.go`) precisely because it is meant to be copy-pasted and run. `Registered`/`Notes` are
not meant to be executed, but they render on the same human-readable line an operator may
select and paste into a shell (the exact vector issue #523 names). Before this phase, the
only unquoted-but-rendered values were operator-supplied (`--url`, flag values); this phase
adds arbitrary bytes actually generated by a third-party CLI (`claude mcp get`, `opencode mcp
list`, a tolerant action's stderr) to that same unescaped rendering path — e.g. a
compromised/misbehaving runtime CLI that echoes back attacker-influenced content (a crafted
MCP server name reflected in `mcp list`'s table, say) would render byte-for-byte, including
any shell metacharacters, in the preview text.
**Fix:** Not a request to re-open the accepted #523 gap fix (out of scope here), but worth a
tracked follow-up: either extend `sanitizeViewValue` to escape/flag shell metacharacters for
fields explicitly sourced from third-party process output, or document (next to `Registered`/
`Notes` in `plan.go`) that these fields are informational-only and must never be copy-pasted
as-is — the same caveat `Command`'s own doc comment implicitly makes moot by being quoted.

### WR-03: Destructive-window recovery guidance lives only in `Notes`, never in `Reason`

**File:** `internal/setup/claudecode.go:51-67`; `internal/setup/apply.go:284-320`
**Issue:** When claude-code's tolerant `mcp remove` succeeds and the following fatal `mcp add`
then fails (the accepted destructive-window case, `TestApplyFailsWhenRegistrationActionFails`),
the operator-facing `Reason` field is built purely mechanically by `describeFailure()` —
runtime name, failing action's argv, exit code, stderr — and never mentions that the prior
slot was actually cleared or that re-running `--apply` is the recovery path. That guidance
exists only as `claudeCodeRemoveAction.Description`, which the executor appends to `Notes`
(not `Reason`) whenever the tolerant action ran, regardless of its own outcome. This is a
deliberate, documented choice (apply.go's comment on step 7, claudecode.go's own comment), so
it is not a violation of an explicit test — but it means any consumer that surfaces `Reason`
alone (a log line, an alert, a truncated CLI summary) drops the one piece of information that
actually matters after this specific failure: that the operator now has NO claude-code
registration and must re-run `--apply`.
**Fix:** Consider folding the tolerant predecessor's `Description` (when the failing action is
non-tolerant and a prior tolerant action succeeded) directly into `Reason`, not only `Notes` —
or, at minimum, note in `Result.Reason`'s own doc comment that a consumer must never render
`Reason` without also rendering `Notes` for a claude-code row.

### WR-04: `describeSeamError`'s error text is never bounded, unlike `describeFailure`'s stderr

**File:** `internal/setup/apply.go:142-149`
**Issue:** `describeFailure` bounds captured stderr via `boundCapture` (max 4096 bytes) before
building `Reason`. `describeSeamError`, used for the structurally analogous "process never
produced an exit status" case, formats `err` directly with `%v` and applies no bound at all.
In practice Go's own exec-seam errors are short (`"exec: start failure"`, context deadline
text), so this is low risk today, but it is an inconsistency in an otherwise carefully
bounded reporting path (D-11's "every captured string is bounded" intent), and a future
`Environment.Run` implementation (or a wrapped error with a long chain) could produce an
unbounded `Reason`.
**Fix:** Wrap `err.Error()` through `boundCapture` in `describeSeamError`, mirroring
`describeFailure`.

## Info

### IN-01: `setupApplyRun` recomputes the failed-row count already implied by `Classify`

**File:** `cmd/engram/setup.go:354-368`
**Issue:** After `setup.Classify(setupResultsFromRows(rows))` has already examined every
row's outcome, `setupApplyRun` re-walks `rows` a second time solely to count
`OutcomeFailed` for the error message. Minor duplication, not a correctness issue (the two
counts cannot disagree given the current, single call site), but a small maintenance
liability if `Classify`'s semantics or the failed-outcome set ever changes.
**Fix:** Either have `Classify` (or a small sibling helper in `internal/setup`) return the
failed count alongside the `ExitClass`, or leave a comment at the loop noting it must be kept
in sync with `Classify`'s own notion of "failed."

---

_Reviewed: 2026-09-09_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
