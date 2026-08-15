---
phase: 04-migration-cli-first-customer
reviewed: 2026-08-15T00:00:00Z
depth: deep
files_reviewed: 33
files_reviewed_list:
  - cmd/engram/backfill.go
  - cmd/engram/backfill_test.go
  - cmd/engram/cmdwalk_test.go
  - cmd/engram/destructive.go
  - cmd/engram/destructive_test.go
  - cmd/engram/migrate_family.go
  - cmd/engram/migrate_family_test.go
  - cmd/engram/operator_output_test.go
  - cmd/engram/operror.go
  - cmd/engram/operror_test.go
  - cmd/engram/root.go
  - cmd/engram/spine_review_purge_test.go
  - cmd/engram/testdata/catalog.golden
  - cmd/engram/testdata/help.golden
  - docs-site/src/content/docs/guides/cli.md
  - docs-site/src/content/docs/guides/upgrade.md
  - docs-site/src/content/docs/reference/errors.md
  - internal/migrate/additive.go
  - internal/migrate/additive_test.go
  - internal/migrate/migrate.go
  - internal/migrate/migrate_test.go
  - internal/migrate/registry.go
  - internal/migrate/registry_test.go
  - internal/migrate/step.go
  - internal/migrate/v1_step.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/migrate.go
  - internal/store/migrate_status.go
  - internal/store/migratebacklog.go
  - internal/store/revert.go
  - internal/store/store.go
  - internal/surfaces/rules.go
  - internal/surfaces/toolclass.go
findings:
  critical: 1
  warning: 4
  info: 1
  total: 6
status: issues
---

# Phase 04: Code Review Report (deep)

**Reviewed:** 2026-08-15
**Depth:** deep
**Files Reviewed:** 33 (per `files_reviewed_list`; several test/data files read for cross-reference only)
**Status:** issues_found

## Summary

This is a deep-depth pass on top of an already-CONFIRMED standard-depth review (see
`<already_established_do_not_re_derive>` in the review brief — all three prior findings,
WR-01/WR-02/IN-01, are re-confirmed by direct code reading below and carried forward
unchanged). `internal/migrate` is confirmed stdlib-only by direct import inspection of every
file in the package (`fmt`, `errors`, `slices`, `maps`, `sort`, `strings` only). The
`migrateFamilyStore` interface seam matches the concrete `*Store` method set exactly. The
`internal/surfaces` toolclass/rules registries carry rows for all five migrate-family CLI
surfaces (`migrate`, `migrate status`, `migrate revert`, `backfill-short-ids`, plus the
pre-existing `migrate-remap-owner`/`migrate-set-owner`), and `cmdwalk_test.go` /
`destructive_test.go` / `operator_output_test.go` all cross-check those registries against the
live cobra tree and the committed goldens with set-equality (both directions) — no drift found
there.

The one new class of defect this deep pass surfaces is specific to `migrate revert`'s
CLI-to-store call chain, which is structurally different from `migrate`'s (forward-sweep)
call chain in a way that reopens exactly the staleness problem the shared H5
preview-then-apply pattern exists to close. `migrate`'s apply path derives its preview
manifest and its apply call from the SAME outer function invocation (`migrateSweepApplyRun`),
so there is no possible staleness between what was rendered and what was acted on. `migrate
revert --apply`, by contrast, makes the CLI's OWN `st.PreviewRevert()` call, and Store's
`Revert()` performs its OWN, SEPARATE, internal re-preview — two independent RPC round trips
against a live backend, with the CLI rendering from the FIRST one and discarding the plan
`Store.Revert` actually acted on. That reopens both an error-handling gap (CR-01) and a
report-fidelity gap (WR-03), and compounds the already-known shared-timeout problem (WR-01)
into a three-pass-under-one-budget variant unique to revert (WR-04).

## Critical Issues

### CR-01: `migrate revert --apply`'s refusal contract silently breaks when Store.Revert's own internal preflight (not the CLI's) is what refuses

**File:** `cmd/engram/migrate_family.go:453-489` (`revertApplyRun`), cross-referenced against
`internal/store/revert.go:277-335` (`Revert`/`revertWithSteps`), `cmd/engram/operror.go:68-150`
(`classifyOperatorErr`), and `cmd/engram/root.go:89-114` (`Execute`/`exitCodeFromError`).

**Status:** CONFIRMED (the mishandling code path is directly traceable; only the triggering
race window is probabilistic, not the defect itself).

**Issue:** `revertApplyRun` calls `st.PreviewRevert(ctx, to)` itself first (call A) and only
renders the deliberately-designed refusal document (`renderOperator` + `usageErrorf` carrying
`exitUsage`) when call A's own `plan.Reversible` is false:

```go
plan, err := st.PreviewRevert(ctx, to)      // call A
...
if !plan.Reversible {
    refusal := store.RevertRefusalError(plan)
    if rerr := renderOperator(cmd, format, revertSummary(plan, false, store.RevertResult{}),
        revertReportDoc(plan, false, store.RevertResult{})); rerr != nil {
        return rerr
    }
    return usageErrorf("%s", refusal)        // exitUsage (2), refusal rendered
}

res, err := st.Revert(ctx, to)              // call B
if err != nil {
    return classifyOperatorErr(err)          // <-- the gap
}
```

But `Store.Revert` (`revertWithSteps`, `internal/store/revert.go:307-335`) performs its OWN,
SEPARATE, second full-range preflight (`s.previewRevertWithSteps(ctx, to, steps)`,
`revert.go:324`) before doing anything else, and refuses independently if THAT preflight says
non-reversible:

```go
plan, perr := s.previewRevertWithSteps(ctx, to, steps)   // call B's own internal preflight
...
res.Plan = plan
if !plan.Reversible {
    err = RevertRefusalError(plan)          // a bare, unsentineled errors.New(...)
    return res, err
}
```

If the above-target range becomes irreversible or gains an unsupported version between call A
and call B (a concurrent writer landing a record in that narrow window — the two calls are
genuinely separate RPC round-trips against a live backend, with no lock held between them), call
B returns this bare `RevertRefusalError` as `err`. Back in `revertApplyRun`, that `err` falls
into the generic `classifyOperatorErr(err)` path. `RevertRefusalError` is `errors.New(...)` — no
sentinel, no `ExitCode()` method — so `classifyOperatorErr`'s own documented default (`operror.go:143-149`,
"a true passthrough... exit code 1 stays a real backstop") returns it unchanged. `Execute()`
(`root.go:89-94`) then does `fmt.Fprintln(os.Stderr, "Error:", err)` and `os.Exit(1)`.

Net effect, in this race window: (1) the exit code is a generic **1**, not the taxonomically
deliberate **exitUsage (2)** every other refusal path and every test in
`TestMigrateFamilyRevertRefusals` asserts; (2) **no JSON or text document is ever rendered** —
a `--output json` caller gets an unstructured `"Error: field=steps hint=irreversible: ..."`
line on stderr instead of the `revertOutputDoc{Refusal: ...}` envelope every other refusal path
produces; (3) `RevertResult.Plan` (`revert.go:329`, "the preflight verdict this run acted on")
is set but never read by any caller anywhere in the repo (`grep '\.Plan\b'` across
`cmd/engram` and `internal/store` finds exactly one write site and zero read sites) — the
information needed to render the TRUE refusal is computed and then thrown away.

No data is at risk (`Store.Revert` returns before `aboveTargetFilter`/the write loop on this
path — zero records touched either way), but the documented, tested contract this file's own
comments repeatedly cite ("REVIEWS.md C5-H4... A REFUSED --apply MUST EXIT NON-ZERO... exitUsage
is the taxonomically correct code") silently degrades to an unstructured, wrongly-coded failure
the moment the SECOND preflight (not the first) is what catches the refusal. `migrate_family_test.go`'s
`TestMigrateFamilyRevertRefusals` only exercises the case where the fake's `previewRevertFn`
(standing in for call A) returns the refusal — there is no test where `revertFn` (standing in
for call B) returns a `RevertRefusalError`-shaped error while `previewRevertFn` reported
reversible, so this gap is untested as well as unhandled.

**Fix:** Give `Store.Revert`'s own refusal a first-class, classifiable shape (e.g. a typed
`*RevertRefusalErr` wrapping the plan, or a sentinel `store.ErrRevertRefused` that
`RevertRefusalError`'s text wraps), and handle it explicitly in `revertApplyRun` the same way
the call-A branch does — render the refusal document from the plan the error actually carries
(not the stale outer `plan`) and return `usageErrorf("%s", refusal)`:

```go
res, err := st.Revert(ctx, to)
if err != nil {
    var refused *store.RevertRefusedError // new typed error carrying the plan
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
Add a `migrate_family_test.go` case where the fake's `revertFn` returns this error while
`previewRevertFn` reports `Reversible: true`, asserting `exitUsage` and a non-empty rendered
`Refusal` field.

## Warnings

### WR-01: `migrate --apply` / `backfill-short-ids --apply` share ONE `--timeout` across TWO full-backlog `Store.Migrate` calls

**File:** `cmd/engram/migrate_family.go:187-213` (`migrateSweepApplyRun`), `cmd/engram/backfill.go:40-42`.

**Status:** CONFIRMED (carried forward from the prior standard-depth review; re-verified by
direct code reading — `ctx, cancel := migrateWithTimeout(ctx, timeout)` at line 196 wraps both
the `DryRun:true` preview call at line 199 and the manifest-limited apply call at line 205,
under one caller-supplied deadline).

**Issue:** An operator sizing `--timeout` around "how long my apply should take" is actually
budgeting for a full-backlog `DryRun` projection (which additionally performs a `MintShortID`
collision-probe `Count` per eligible record, per `MigrateOptions.DryRun`'s own doc comment)
PLUS the actual write pass, with no way to allocate time between the two. On a large backlog
this can starve the write pass of budget it would otherwise have had, or fail before any write
even begins. Undocumented in `migrateCmd.Long` or the CLI guide.

**Fix:** Either (a) document the two-pass cost explicitly in `migrateCmd.Long` /
`backfillShortIDsCmd.Short` and in `docs-site/src/content/docs/guides/cli.md`'s migrate-family
section, or (b) split the budget (e.g. reserve a fraction of `timeout` for the preview pass via
a shorter derived context, falling back to the full budget for the apply pass). Documentation
is the minimal fix; budget-splitting is the more complete one.

### WR-02: `RevertRefusalError` emits TWO `field=`/`hint=` envelopes joined by `"; "` on a mixed irreversible+unsupported range

**File:** `internal/store/revert.go:160-181` (`RevertRefusalError`).

**Status:** CONFIRMED (carried forward from the prior standard-depth review; re-verified —
when `len(plan.Irreversible) > 0` AND `len(plan.Unsupported) > 0`, `parts` accumulates one
`field=steps hint=irreversible: ...` string and one `field=record_version hint=unsupported: ...`
string, and the final `strings.Join(parts, "; ")` concatenates both into a single returned
error, violating the one-envelope-per-rejection contract documented in
`docs-site/src/content/docs/reference/errors.md`).

**Fix:** Pick one envelope to lead with (irreversible steps first, since a truly irreversible
step is the harder blocker to fix) and fold the other condition's detail into that envelope's
`Detail` text as an additional clause, or extend the envelope grammar to support a documented
multi-field/multi-hint form and update `errors.md` to match. Either way, the fix must land in
`RevertRefusalError` itself so the CLI and any future caller inherit it for free — do not
special-case this in `cmd/engram`.

### WR-03 (new): `migrate revert --apply`'s rendered report uses the CLI's stale first-preview plan, never the plan `Store.Revert` actually acted on

**File:** `cmd/engram/migrate_family.go:484-488` (the success tail of `revertApplyRun`),
cross-referenced against `internal/store/revert.go:56-62` (`RevertResult.Plan`'s doc comment:
"the preflight verdict this run acted on") and `revert.go:329` (`res.Plan = plan`, the ONLY
write site of that field in the repo).

**Status:** CONFIRMED (traced both the write site and the absence of any read site via
`grep '\.Plan\b'` across `cmd/engram` and `internal/store` — zero read sites).

**Issue:** On the reversible/success path:

```go
res, err := st.Revert(ctx, to)     // res.Plan is Store.Revert's OWN fresh (2nd) preflight verdict
...
return renderOperator(cmd, format, revertSummary(plan, true, res), revertReportDoc(plan, true, res))
```

both `revertSummary` and `revertReportDoc` are called with `plan` — the CLI's OWN first
`st.PreviewRevert()` call's result — not `res.Plan`, the fresher verdict `Store.Revert` itself
derived and actually walked against. If the above-target backlog changes between the CLI's
call to `PreviewRevert` and its call to `Revert` (two separate RPC round trips against a live
backend, exactly the same race window CR-01 depends on, just without crossing the
reversible/irreversible boundary), the rendered `Candidates` field can diverge from the
`Reverted`/`Failed`/`Backlog` fields actually observed — e.g. `Reverted` could legitimately
exceed the reported `Candidates` if more above-target records appeared between the two calls,
producing an internally inconsistent report an operator or script has no way to explain from
the document alone. This is exactly the staleness class `migrate`'s (forward) H5
preview-then-apply pattern avoids by construction (both calls happen inside ONE outer function
invocation, `migrateSweepApplyRun`, using the SAME manifest) — `migrate revert`'s
two-separate-CLI-calls shape reopens it.

**Fix:** Render from `res.Plan` (fresh) instead of the outer `plan` (stale) on the success
path:

```go
return renderOperator(cmd, format, revertSummary(res.Plan, true, res), revertReportDoc(res.Plan, true, res))
```
The outer `plan` from the CLI's own `PreviewRevert` call remains useful only for the
refusal-at-call-A branch above it, where it is the only verdict available at that point.

### WR-04 (new): `migrate revert --apply`'s single `--timeout` budget covers TWO full-range preflight scans plus the write-convergence loop — worse than WR-01's already-documented double-pass, and undocumented

**File:** `cmd/engram/migrate_family.go:453-489` (`revertApplyRun`), cross-referenced against
`internal/store/revert.go:307-335` (`revertWithSteps` calling `previewRevertWithSteps` again
internally before its write loop) and `internal/store/revert.go:203-222`
(`previewRevertWithSteps`'s own doc comment, confirming it is an EXHAUSTIVE
`scrollAllPoints` pass over the entire above-target range, not a single page).

**Status:** CONFIRMED.

**Issue:** `revertApplyRun` installs one deadline (`migrateWithTimeout(ctx, migrateRevertTimeout)`,
line 466) that must cover, in sequence: (1) the CLI's own `st.PreviewRevert(ctx, to)` call — an
exhaustive `scrollAllPoints` pass over the whole above-target range; (2) `st.Revert(ctx, to)`,
which internally repeats that IDENTICAL exhaustive pass a second time
(`previewRevertWithSteps` inside `revertWithSteps`) before it is even allowed to start the
actual reverse-walk write loop; (3) the write-convergence loop itself, which re-derives a fresh
`Count` and re-scrolls on every pass until the backlog reaches zero. This is strictly worse
than WR-01's already-flagged migrate case (which shares a budget across exactly two calls, one
preview and one apply) — revert shares one budget across two full read-only scans of
potentially the same large range PLUS an arbitrary number of write-convergence passes. Neither
`migrateRevertCmd.Long` nor `docs-site/src/content/docs/guides/cli.md`'s timeout-groups table
mentions this cost; an operator sizing `--timeout` for "one reverse walk of my backlog" is
actually budgeting for at least two full read-only passes over that same range before any
inverse is even applied.

**Fix:** Document the two-preflight-plus-convergence cost in `migrateRevertCmd.Long` and in the
CLI guide's `migrate revert` section, alongside WR-01's migrate note (both can be documented in
the same guide edit, since they are the same underlying pattern applied to two commands). A
structural fix (skipping the CLI's own redundant call-A preflight and rendering the preview
directly from what `Store.Revert` would compute, or exposing a preview-only variant that
doesn't duplicate `Store.Revert`'s own internal check) is a larger change and not required to
close this finding — documentation is the minimal correct fix, matching WR-01's disposition.

## Info

### IN-01: `migrateCmd.Long` omits the double-pass cost

**File:** `cmd/engram/migrate_family.go:237-247` (`migrateCmd`'s `Long` field).

**Status:** CONFIRMED (carried forward from the prior standard-depth review). The `Long` text
describes the intersection semantics (spared/appeared) accurately but never states that
`--apply` performs two full-backlog passes (a fresh `DryRun` preview, then the manifest-limited
apply), which is the operational fact WR-01 concerns.

**Fix:** Add one sentence to `migrateCmd.Long` noting the two-pass cost, e.g. "`--apply`
performs a full fresh backlog scan before writing, in addition to the write pass itself — size
`--timeout` accordingly." Same fix applies to `backfillShortIDsCmd.Short`/help text by
inheritance since it delegates to the same run funcs.

---

_Reviewed: 2026-08-15_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
