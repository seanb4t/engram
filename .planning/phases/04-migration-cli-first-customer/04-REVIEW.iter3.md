---
status: issues
files_reviewed: 42
depth: deep
iteration: 2
findings:
  critical: 0
  warning: 1
  info: 0
  total: 1
---

# Phase 04: Code Review Report — Iteration 2 (deep, --auto fix loop)

**Reviewed:** 2026-08-15
**Depth:** deep
**Iteration:** 2
**Files Reviewed:** 42 (per config `files` list)
**Status:** issues (1 new WARNING; all 6 iteration-1 findings resolved)

## Summary

This pass re-reviewed the three iteration-1 fix commits (`43a77129`, `de7fcbb9`,
`e8a909a9`) against the diff range `09329293..HEAD`, plus a fresh trace of every
return path out of `Store.PreviewRevert` / `Store.Revert` / `revertWithSteps` and
`revertApplyRun` looking specifically for refusal-shaped conditions that could
still surface as an untyped error after the CR-01 fix.

All six iteration-1 findings are RESOLVED as fixed (see "Previous findings"
below), confirmed by direct code reading and by running the cited tests plus the
full `help.golden`/`catalog.golden` suite locally (`go test ./cmd/engram/...`,
`go test ./internal/store/... -run 'TestRevertRefusalError|TestMigrateRevert'`,
`go build ./...`) — all green, matching the orchestrator's independently-verified
`task` run.

One new WARNING surfaced from tracing the CR-01 fix's actual boundary: the typed
`*RevertRefusedError` only covers `Store.Revert`'s own **preflight** refusal
(the `!plan.Reversible` return before the write loop starts). A structurally
identical refusal condition can still occur **after** that preflight passes,
inside the write-convergence loop itself, and it surfaces as a plain
`fmt.Errorf` that `revertApplyRun`'s `errors.As(err, &refused)` does not catch —
falling through to `classifyOperatorErr`'s generic exit-1 passthrough with a
message that does not follow the `field=`/`hint=` envelope grammar. This is not
introduced by the iteration-1 fix (the code at this site is untouched by the
diff) and does not risk data loss, but it means CR-01's stated goal — "no
refusal-shaped condition surfaces as an untyped error out of this call chain" —
is only fully met for the branch that was tested. Detailed below.

## Warnings

### WR-05: A refusal-shaped condition discovered mid-write-loop (after `Store.Revert`'s own preflight already passed) still surfaces as an untyped, non-`field=`/`hint=` error that `revertApplyRun`'s `errors.As` cannot catch

**File:** `internal/store/revert.go:453-457` and `:460-469` (inside `revertWithSteps`'s
write loop), cross-referenced against `cmd/engram/migrate_family.go:504-516`
(`revertApplyRun`'s `errors.As(err, &refused)` handling).

**Status:** PLAUSIBLE (code-traced with a concrete, realizable trigger scenario;
not reproduced against a live Qdrant with a timed concurrent writer).

**Issue:** `revertWithSteps` runs `previewRevertWithSteps` exactly once (line 367,
"call B"), decides `plan.Reversible`, and — since the CR-01 fix — returns a typed
`*RevertRefusedError` if that single preflight already sees an irreversible step
or unsupported version. But if `plan.Reversible == true`, execution proceeds into
a write-convergence loop that repeats `Count` + `ScrollAndOffset` on every pass
until the backlog reaches zero (lines 390-441) — additional, independent RPC
round trips against a live backend, run *after* the preflight that established
reversibility. For each scrolled record, the loop re-derives its own chain via
`revertStepsFrom(steps, fromV, to)` (line 453) and, for each step in that chain,
looks up its inverse via `migrate.Inverse(step.Reversibility())` (line 461).

Two return sites in this loop are refusal-shaped but untyped:

```go
chain, cherr := revertStepsFrom(steps, fromV, to)
if cherr != nil {
    err = fmt.Errorf("revert: point %s: %w", id, cherr)   // no unsupported-version envelope
    return res, err
}
...
inverse, ok := migrate.Inverse(step.Reversibility())
if !ok {
    // "never expected to fire in production" — but see trigger below
    err = fmt.Errorf("revert: point %s: step (From=%d To=%d) has no inverse despite passing the whole-range preflight", id, step.From(), step.To())
    return res, err
}
```

Concrete trigger: with the shipped registry (single step, v0→v1, declared
`Irreversible`), a `migrate revert --to 0 --apply` invocation's `plan.Reversible`
can only be `true` when the preflight observes **zero** above-target records at
that moment (any v1 record makes the range irreversible by construction). If a
concurrent `engram migrate --apply` lands a NEW v1 record in the window between
`revertWithSteps`'s own preflight (call B) finishing and the write loop's first
`Count`/`ScrollAndOffset` (or any subsequent pass — the loop runs until backlog
is 0, so the window reopens every pass), that record enters the write loop
directly: `revertStepsFrom` still finds a *chain* for it (chain existence and
reversibility are different checks — `StepsFrom` doesn't fail on an irreversible
step), so `cherr` is nil, but `migrate.Inverse(step.Reversibility())` then
returns `ok == false` for the v0↔v1 step, hitting the second branch above with
the "never expected to fire in production" comment despite this being a directly
reachable production race, not a theoretical invariant violation.

Either branch returns a bare, unsentineled `error`. In `revertApplyRun`:

```go
res, err := st.Revert(ctx, to)
if err != nil {
    var refused *store.RevertRefusedError
    if errors.As(err, &refused) {   // does NOT match either branch above
        ...
    }
    return classifyOperatorErr(err) // generic exit-1 passthrough, no field=/hint= text
}
```

`errors.As` only matches `*store.RevertRefusedError`, which neither loop branch
returns, so both fall through to `classifyOperatorErr`'s documented default
("a true passthrough... exit code 1 stays a real backstop for a genuinely
unclassified Go error" — `cmd/engram/operror.go:61-63`). The result: exit code
**1** instead of the taxonomically-intended `exitUsage` (2) every other revert
refusal path uses, no `revertOutputDoc{Refusal: ...}` JSON envelope rendered at
all (an `--output json` caller gets nothing but an unstructured stderr line via
`Execute()`), and message text that does not follow the `field=`/`hint=` grammar
`docs-site/reference/errors.md` documents for every other revert refusal.

No data loss: no write for the offending record is attempted before either
branch returns (the chain/inverse lookup happens before any `DeletePayload`/
`SetPayload` call), and any records already reverted in earlier passes or
earlier in the same pass remain correctly reverted. This is a report-fidelity
and exit-code-taxonomy gap, not a correctness-of-mutation gap — hence WARNING,
not BLOCKER. It is untested: `grep -rn "no inverse despite passing" --type go`
shows the string appears only at its own definition site, in no test.

**Fix:** Give these two loop-internal branches the same typed treatment CR-01
gave the top-level preflight refusal — e.g. re-run (or synthesize) a
single-record `RevertPlan`-shaped refusal and return
`&store.RevertRefusedError{Plan: ...}` from both sites instead of a bare
`fmt.Errorf`, so `revertApplyRun`'s existing `errors.As` handling (already
correct for the top-level case) catches these too without any CLI-side change.
Minimally, rename the misleading "never expected to fire in production" comment
at line 463-466 since this review demonstrates a realizable trigger via a
concurrent `migrate --apply` racing a `migrate revert --apply` on the same
above-target range.

## Previous findings (iteration 1 → iteration 2 disposition)

- **CR-01** (`Store.Revert`'s own second preflight refusal surfaces untyped,
  wrong exit code, no rendered doc): **RESOLVED** for the exact branch it
  described (the `!plan.Reversible` return at `revertWithSteps`'s preflight
  gate, revert.go:373-382). Verified: `RevertRefusedError{Plan: plan}` is now
  returned there; `revertApplyRun` does `errors.As(err, &refused)` and renders
  from `refused.Plan`; `TestMigrateFamilyRevertApplySecondPreflightRefusal`
  passes and was independently confirmed RED-then-green by the orchestrator.
  **However**, see new finding WR-05 above: the identical class of untyped
  refusal is still reachable from two *different* return sites one layer
  deeper (inside the write-convergence loop, after the preflight this fix
  covers has already passed) — CR-01's fix closes the branch it targeted
  completely but does not close the adjacent, structurally identical gap.
- **WR-01** (`--apply`'s shared timeout budget covers two full-backlog passes,
  undocumented): **RESOLVED**. `migrateCmd.Long` now states "--apply performs
  a full fresh backlog scan before writing, in addition to the write pass
  itself — size --timeout for both passes together (same applies to
  backfill-short-ids, which delegates here)." Verified against
  `help.golden`'s diff and confirmed the golden test passes.
- **WR-02** (`RevertRefusalError` emits two concatenated `field=`/`hint=`
  envelopes on a mixed irreversible+unsupported range): **RESOLVED**. The
  function is now a 4-branch switch emitting exactly one envelope, leading
  with `field=steps hint=irreversible` and folding the unsupported detail in
  via "; additionally, ...". Verified by reading the switch and running
  `TestRevertRefusalErrorSingleEnvelope`, which asserts `strings.Count(msg,
  "field=") == 1` (a real occurrence count, not a `-c` line-count trap) —
  green. `docs-site/reference/errors.md` was updated to state the
  one-envelope contract and matches the actual precedence (irreversible
  leads) exactly.
- **WR-03** (revert's success-path report rendered from the CLI's stale
  first-preview `plan` instead of `res.Plan`): **RESOLVED**. `revertApplyRun`'s
  success tail now calls `renderOperator(cmd, format, revertSummary(res.Plan,
  true, res), revertReportDoc(res.Plan, true, res))`. Verified by reading the
  line and by `TestMigrateFamilyRevertReversible`'s updated fixture, which
  seeds `store.RevertResult{..., Plan: plan}` and would fail with a mismatched
  `Candidates` field if the CLI reverted to reading the stale outer `plan`.
- **WR-04** (revert `--apply`'s single timeout covers two full preflight scans
  plus the write-convergence loop, undocumented): **RESOLVED**.
  `migrateRevertCmd.Long` now states "--apply budgets TWO full read-only
  whole-range scans (this preflight, then an identical one Store.Revert
  repeats internally before it is allowed to write) plus the
  write-convergence loop itself under one --timeout -- size accordingly."
  Verified against `help.golden` and `docs-site/guides/cli.md`'s new
  `migrate revert --apply` paragraph, which states the same budget accurately.
- **IN-01** (`migrateCmd.Long` omits the double-pass cost note): **RESOLVED**
  as part of the same WR-01/WR-04 doc commit — both `migrate` and `migrate
  revert` `Long` strings and the CLI guide now carry the note.

---

_Reviewed: 2026-08-15_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
_Iteration: 2_
