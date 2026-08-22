---
status: issues
files_reviewed: 14
depth: deep
iteration: 3
findings:
  critical: 1
  warning: 1
  info: 0
  total: 2
---

# Phase 04: Code Review Report (Iteration 3 — Deep, post-WR-05)

**Reviewed:** 2026-08-15
**Depth:** deep
**Files Reviewed:** 14
**Status:** issues_found

## Summary

This iteration reviews commit `876dabdf` (the WR-05 fix: typing the two mid-loop
refusal-shaped return sites in `revertWithSteps`'s write-convergence loop as
`*RevertRefusedError`) and traces its blast radius across every consumer of
`RevertPlan`/`RevertRefusedError` in the phase's file set.

The fix itself is sound: `revert.go`'s two new return sites correctly synthesize a
single-record `RevertPlan`, the `Reversible` field's zero value matches the derived
invariant for both synthesized shapes, `TestRevertRefusalErrorSingleEnvelope`'s
one-envelope contract is undisturbed, the new `countSideEffectInjector` test helper is
per-test/per-connection with no process-wide or shared-state leakage, and
`errors.md`'s new paragraph (lines 149-160) accurately documents the `Candidates: 1`
synthesis and the mid-loop trigger.

However, tracing the fix's blast radius into its only functional consumer outside
`revert.go` — `cmd/engram/migrate_family.go`'s `revertApplyRun` — surfaces one CRITICAL
reporting-fidelity defect that 876dabdf's semantic change opens up in code it did not
touch, plus one WARNING documentation gap tied to the same root cause. Both are new
findings this iteration; neither is a re-litigation of CR-01/WR-01..05/IN-01, all of
which remain resolved (see Previous Findings below).

## Critical Issues

### CR-06: `revertApplyRun` discards real partial-progress counts on a mid-loop refusal, reporting a destructive mutation that occurred as if it never happened

**File:** `cmd/engram/migrate_family.go:504-516`

**Issue:**

```go
res, err := st.Revert(ctx, to)
if err != nil {
    var refused *store.RevertRefusedError
    if errors.As(err, &refused) {
        if rerr := renderOperator(cmd, format,
            revertSummary(refused.Plan, false, store.RevertResult{}),
            revertReportDoc(refused.Plan, false, store.RevertResult{})); rerr != nil {
            return rerr
        }
        return usageErrorf("%s", store.RevertRefusalError(refused.Plan))
    }
    return classifyOperatorErr(err)
}
```

`st.Revert` returns `(res, err)`. `res` here is the *real* `RevertResult` Go returned —
it is captured by `:=` and then never read in the `errors.As` branch. Both render calls
pass a freshly-constructed `store.RevertResult{}` (all fields zero) instead.

Before 876dabdf this was harmless: `*RevertRefusedError` could only originate from
`revertWithSteps`'s **top-of-function** preflight (revert.go:373-381), which runs before
the write-convergence loop starts anything, so at that return site `res.Reverted`,
`res.Failed`, `res.Passes`, and `res.Backlog` genuinely were all still zero — `res` and
`store.RevertResult{}` were equivalent, and passing the literal instead of the real
variable was a no-op simplification.

876dabdf breaks that equivalence by adding two new `*RevertRefusedError` return sites
**inside** the write-convergence loop (revert.go:473-478, revert.go:497-502). By the time
either of those sites fires, the loop may already have completed one or more full passes
and written `res.Reverted++` for every record that converged successfully before the
offending (racer) record was reached — in the same batch or an earlier pass. `res` at
that point is the true, non-zero record of what the collection just had done to it.
`revertApplyRun` throws that value away and renders as if `applied: false`,
`reverted: 0`, `failed: 0`, `backlog: 0` — even though N records were durably re-stamped
to the target version moments earlier in the same invocation.

**Concrete failure scenario:**

1. Operator runs `engram migrate revert --to 0 --apply` against a backlog of, say, 500
   above-target records (batch size is `migrateBatch = 256`, `internal/store/migrate.go:22`).
2. Pass 1: 256 records revert cleanly; `res.Reverted = 256`, `res.Passes = 1`.
3. Between pass 1 and pass 2, a concurrent `engram migrate --apply` lands one new record at
   an unsupported/irreversible version (the exact race REVIEWS.md WR-05 traces).
4. Pass 2's `ScrollAndOffset` returns that record; `revertStepsFrom` or `migrate.Inverse`
   fails on it; `revertWithSteps` returns `res{Reverted: 256, Passes: 2, ...}` wrapped with
   `&RevertRefusedError{Plan: <synthesized single-record plan>}`.
5. `revertApplyRun` catches this, discards `res`, and emits (in `--output json`):
   `{"to":0,"applied":false,"reversible":false,"candidates":1,"reverted":0,"failed":0,
   "passes":0,"backlog":0,"refusal":"field=record_version hint=unsupported: ..."}`.
6. The operator's audit trail claims zero records were touched. In truth 256 records were
   already permanently re-stamped to the target schema version. An operator relying on this
   report to decide whether the collection needs reconciliation, or whether it is safe to
   re-run from a clean baseline, is misled about the collection's actual state.

This directly contradicts the phase's own documented invariant for this type
(`revert.go:53-55`): *"Backlog is a fresh exact Count after the walk — truth, never
inferred from the counters."* Post-876dabdf, the refusal path no longer honors that
promise for `Reverted`/`Failed`/`Passes`/`Backlog`.

**Fix:** Render from the real `res`, not a zero-value literal, in the `errors.As` branch:

```go
res, err := st.Revert(ctx, to)
if err != nil {
    var refused *store.RevertRefusedError
    if errors.As(err, &refused) {
        if rerr := renderOperator(cmd, format,
            revertSummary(refused.Plan, false, res),
            revertReportDoc(refused.Plan, false, res)); rerr != nil {
            return rerr
        }
        return usageErrorf("%s", store.RevertRefusalError(refused.Plan))
    }
    return classifyOperatorErr(err)
}
```

Note `revertReportDoc`/`revertSummary` currently zero out `Reverted`/`Failed`/`Passes`/
`Backlog` whenever `!plan.Reversible` regardless of the `res` argument (`migrate_family.go:355-358`,
`374-377`), so passing the real `res` through is not sufficient by itself — those two
functions also need a code path (e.g. an explicit "partial" branch, gated on
`res.Passes > 0 || res.Reverted > 0`) that surfaces the real counts on a mid-loop refusal
while preserving today's all-zero rendering for the true top-level-preflight refusal (where
`res` really is zero and the current text/JSON shape is correct and already covered by
existing tests). A new test exercising `revertFn` returning a non-zero `RevertResult`
alongside `*RevertRefusedError` (unlike the existing
`TestMigrateFamilyRevertApplySecondPreflightRefusal`, which only exercises the
all-zero-`res` case) is needed to pin the fixed behavior — the current test suite does not
exercise this scenario at either layer: `revert_test.go`'s two new mid-loop tests
deliberately seed the racer as the very first record an empty collection's write loop ever
sees (so `res.Reverted` is 0 in both, by construction — see revert_test.go:588-651, 662-716),
and `migrate_family_test.go`'s existing refusal test only ever constructs a
zero-valued `RevertResult` alongside the refusal.

**CONFIRMED** — traced end-to-end through `revert.go`'s new return sites and
`migrate_family.go`'s sole consumer; not merely plausible. Verified via `git show 876dabdf`
that `migrate_family.go` was not touched by the fix, so this discard predates 876dabdf but
was inert until 876dabdf changed the invariant it silently relied on.

## Warnings

### WR-06: `errors.md`'s hint-code table still reads as "always refused before any write," which is no longer complete for the mid-loop case

**File:** `docs-site/src/content/docs/reference/errors.md:139`

**Issue:** The table row for `irreversible` says *"the whole operation is refused before
any write."* That remains true for the specific triggering record (the racer's own write is
indeed never attempted — `revert.go` confirms zero writes for that record), and the later
paragraph (lines 149-160) correctly qualifies that a mid-loop refusal is possible and
reports `Candidates: 1` rather than the whole range's count. But neither the table row nor
the qualifying paragraph tells the reader that, unlike the top-level-preflight refusal, a
mid-loop refusal can follow real, already-committed writes to *other* records earlier in the
same `--apply` invocation. Combined with CR-06 above, an operator reading only the docs (not
the source) has no way to learn that a mid-loop refusal's `reverted`/`backlog` fields in the
JSON report may currently under-report what the collection actually experienced.

**Fix:** Once CR-06 is fixed to report real partial-progress counts, extend the paragraph at
lines 149-160 with one sentence noting that a mid-loop refusal can follow real writes to
earlier records in the same run, and that the report's `reverted`/`failed`/`backlog` fields
(once fixed) reflect that partial progress rather than reading as zero.

**PLAUSIBLE** (doc-completeness gap, not independently a behavioral defect) — becomes
concretely misleading only in combination with CR-06; downgrading to WARNING because the
underlying per-record safety claim ("no write attempted for the racer itself") is accurate
as stated.

## Previous Findings — Resolution Status

| ID | Resolved |
|---|---|
| CR-01 | RESOLVED (iteration 1, commits 43a77129/de7fcbb9/e8a909a9) — confirmed by iteration-2 review and orchestrator prove-RED |
| WR-01 | RESOLVED (iteration 1) |
| WR-02 | RESOLVED (iteration 1) |
| WR-03 | RESOLVED (iteration 1) |
| WR-04 | RESOLVED (iteration 1) |
| IN-01 | RESOLVED (iteration 1) |
| WR-05 | RESOLVED (iteration 2 finding, fixed in 876dabdf) — the mid-loop refusal-typing fix itself is correct and well-tested against the real production registry; this iteration's CR-06/WR-06 are new findings surfaced by tracing WR-05's fix into its CLI-side consumer, not a re-litigation of WR-05 |

Not re-derived per instructions: `task` green on a clean lint cache, `go build ./...` clean,
and the orchestrator's own prove-RED experiment on `TestMigrateRevertMidLoopRefusalIsTypedAndCatchable`.

---

_Reviewed: 2026-08-15_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
