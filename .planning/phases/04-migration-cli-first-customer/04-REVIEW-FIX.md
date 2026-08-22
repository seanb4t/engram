---
phase: 04-migration-cli-first-customer
depth: deep
iteration: 3
findings_in_scope: 9
fixed: 9
skipped: 0
status: all_fixed
---

# Phase 04 — Code Review Fix Report (deep, `--fix --auto --all`)

Three review/fix iterations. Every finding raised across all three is fixed and
committed; the final tree is green on `task` (golangci-lint on a clean cache + all
17 Go packages + Python suite), `go build ./...`, `go vet ./...`, and
`task license:check` (333 valid, 0 invalid).

## Outcome by finding

| ID | Iter | Severity | Status | Commit |
|----|------|----------|--------|--------|
| CR-01 | 1 | Critical | Fixed | `43a77129` |
| WR-03 | 1 | Warning | Fixed | `43a77129` |
| WR-02 | 1 | Warning | Fixed | `de7fcbb9` |
| WR-01 | 1 | Warning | Fixed (docs) | `e8a909a9` |
| WR-04 | 1 | Warning | Fixed (docs) | `e8a909a9` |
| IN-01 | 1 | Info | Fixed (docs) | `e8a909a9` |
| WR-05 | 2 | Warning | Fixed | `876dabdf` |
| CR-06 | 3 | Critical | Fixed | `b904c092` |
| WR-06 | 3 | Warning | Fixed (docs) | `f2e696e0` |

## Iteration 1 — six findings

**CR-01 / WR-03 (shared root cause), `43a77129`.** `migrate revert --apply` made two
independent CLI→store round trips: the CLI's own `PreviewRevert`, then
`Store.Revert`'s internal re-preview. Only the first could render the refusal
document and return `exitUsage` (2); a refusal caught by the second escaped as a
bare `errors.New`, producing exit 1 with no rendered document. The success path also
rendered from the stale outer `plan` rather than the plan the store acted on.

Fixed structurally rather than by duplicating the rendering block: a new exported
`store.RevertRefusedError{Plan}`, caught with `errors.As` in `revertApplyRun` and
rendered through the *same* path, plus rendering from `res.Plan`.

**WR-02, `de7fcbb9`.** `RevertRefusalError` joined up to two parts, so a range holding
both an irreversible-chain record and an unsupported-version record emitted **two**
`field=`/`hint=` envelopes in one string — breaking the one-envelope contract an
agent branches on. Now a four-branch switch emitting exactly one, leading with
`irreversible` (it cannot be resolved by migrating forward again) and folding the
unsupported detail into the same envelope's text.

**WR-01 / WR-04 / IN-01, `e8a909a9`.** Documentation only: the migrate family spends
one `--timeout` budget across two full-backlog passes. Recorded in `Long` help,
`guides/cli.md`, and `help.golden`.

## Iteration 2 — one finding

**WR-05, `876dabdf`.** CR-01's fix covered only `Store.Revert`'s top-level preflight.
Two structurally identical refusal sites *inside* the write-convergence loop still
returned bare `fmt.Errorf`, uncatchable by `errors.As`. Both now synthesize a
single-record `RevertPlan` and return `&RevertRefusedError{...}`. The comment claiming
the branch was "never expected to fire in production" was corrected — the review
traced a realizable trigger (a concurrent `migrate --apply` landing an above-target
record between the preflight and the write loop).

## Iteration 3 — two findings, including one this loop caused

**CR-06, `b904c092` — a regression introduced by the WR-05 fix.** `876dabdf` added new
*sources* of `*RevertRefusedError` without revisiting the consumer that had quietly
relied on the old invariant "this error implies nothing was written." `revertApplyRun`
rendered a zero-value `RevertResult`, and both `revertReportDoc` and `revertSummary`
early-return on `!plan.Reversible` before populating counters. So a run that reverted
256 records and then hit the race reported `reverted: 0` — telling an operator there
was nothing to reconcile when the collection was left partially reverted.

Both renderers now surface the real `res` on the refusal path. The refusal envelope is
byte-unchanged, and the text clause is appended only when there is progress to report,
so a top-level preflight refusal (all-zero `res`) renders exactly as before.

Neither of `876dabdf`'s two new tests could have caught this: both seed the racing
record as the very first record the loop sees, making `res.Reverted` zero by
construction.

**WR-06, `f2e696e0`.** `reference/errors.md` described the mid-loop refusal but not its
operational consequence. Now states that such a refusal can follow writes that already
landed, that the report carries real counters alongside `applied: false`, and that a
non-zero `reverted` means a partially reverted collection to reconcile with
`engram migrate status`.

## Prove-RED evidence

Every gate below was run against a deliberate violator, confirmed to fail, then
restored and confirmed to pass. Gates verified independently by the orchestrator, not
only self-reported:

| Gate | Violator | RED output |
|------|----------|------------|
| `TestRevertRefusalErrorSingleEnvelope` | restore the two-envelope format string | `emitted 2 field=/hint= envelope(s), want exactly 1` |
| `TestMigrateFamilyRevertApplySecondPreflightRefusal` | remove the `errors.As` branch | `exitCodeFromError = 1, want 2 (exitUsage)`, empty stdout |
| `TestMigrateRevertMidLoopRefusalIsTypedAndCatchable` | revert the `Inverse` branch to bare `fmt.Errorf` | `errors.As(err, &refused) = false, want true` |
| `TestMigrateFamilyRevertApplyRefusalReportsPartialProgress` (JSON) | restore the doc-builder early return | `doc.Reverted = 0, want 256` |
| `TestMigrateFamilyRevertApplyRefusalReportsPartialProgress` (text) | drop the summary progress clause | summary missing the 256-record disclosure |

Two earlier orchestrator experiments on `Store.PreviewRevert` (pre-existing code, not
changed by this loop) also held: a verdict ignoring `Unsupported` and a batch-scoped
scan both went RED, the latter on `plan.Candidates = 2, want 5`.

## Notes

- No existing test was weakened. `TestMutatingCommandNamesMembership` (exact set) and
  `TestRevertRefusalErrorSingleEnvelope` (exactly one envelope) pass unmodified.
- No data-loss defect was found in any iteration. CR-01, WR-05 and CR-06 are all
  diagnosability/contract defects: wrong exit code, unparseable error, or an
  under-reported result. Nothing wrote the wrong bytes.
- The `--auto` cap is 3 iterations. CR-06 was found by iteration 3's review and would
  normally have been left to a human, but it was a regression this loop itself
  introduced, so it was fixed rather than shipped.
- Per-iteration snapshots are preserved as `04-REVIEW.iter{2,3}.md` and
  `04-REVIEW-FIX.iter{2,3}.md`.
