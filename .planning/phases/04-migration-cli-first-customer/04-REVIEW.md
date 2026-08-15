---
phase: 04-migration-cli-first-customer
reviewed: 2026-08-15T15:24:03Z
depth: standard
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
  - cmd/engram/operror_test.go
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
  - internal/store/migrate_converge_test.go
  - internal/store/migrate_status.go
  - internal/store/migrate_status_test.go
  - internal/store/migrate_test.go
  - internal/store/migratebacklog.go
  - internal/store/revert.go
  - internal/store/revert_test.go
  - internal/store/schemaversion_recallgate_test.go
  - internal/store/schemaversion_stamp_gate_test.go
  - internal/store/store.go
  - internal/surfaces/rules.go
  - internal/surfaces/toolclass.go
  - skill/engram/skills/curating-memory/SKILL.md
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 04: Code Review Report

**Reviewed:** 2026-08-15T15:24:03Z
**Depth:** standard
**Files Reviewed:** 33 (list above; several are unchanged-but-relevant context files pulled in via `files_to_read`)
**Status:** issues_found

## Summary

This phase adds a schema-version migration mechanism (`internal/migrate`), its
store-layer driver (`Store.Migrate`/`Store.MigrateStatus`/`Store.PreviewRevert`/
`Store.Revert`), and the `engram migrate` / `migrate status` / `migrate revert`
CLI family, replacing the old standalone `BackfillShortIDs`.

I traced every filter construction the task explicitly flagged as historically
dangerous in this repo (`backlogFilter`, `aboveTargetFilter`, the `MigrateStatus`
Facet/Count triad) and found each one correctly guards against the
Range-on-absent-key false-empty-backlog trap: `backlogFilter` nests its
`Range`/`IsEmpty` pair inside a `Must`-wrapped `Should` (never a bare top-level
`Should`), and `aboveTargetFilter` correctly omits an `IsEmpty` arm (an absent
key decodes to v0, which is never above a non-negative target). I traced every
Qdrant write in the sweep and revert paths and confirmed they are all delta-only
(`SetPayload`/`DeletePayload` built from `AddedKeys`/`RemovedKeys` against the
step chain's own before/after, never a whole-payload overwrite) — the
payload-clobbering risk the task flagged as high-priority is not present. The
whole-range revert preflight (`PreviewRevert`) genuinely precedes any write and
uses the one exhaustive paginated iterator (`scrollAllPoints`) rather than a
hand-rolled loop, so a partial-range revert cannot occur. The manifest-limited
apply's preview/apply intersection (Spared/Appeared) is derived correctly, and
the mid-sweep no-lock convergence property is proven against a live concurrent
writer with a deterministic wire-level trigger (not a sleep).

I did not find a BLOCKER: no data-loss path, no silently-empty backlog filter,
no payload clobbering, and no idempotence/re-entrancy violation in the code as
written. I found two WARNING-level robustness/documentation gaps and one INFO
item, detailed below.

## Warnings

### WR-01: `--apply`'s shared timeout budget covers two full-backlog passes, undocumented

**File:** `cmd/engram/migrate_family.go:187-213` (`migrateSweepApplyRun`), also reached via `cmd/engram/backfill.go:40-42` (`backfillApplyRun`)
**Issue:** CONFIRMED by reading the code and cross-checked against
`cmd/engram/migrate_family_test.go`'s `TestMigrateFamilyPreviewAndApply`
(`"migrate --apply calls Migrate twice: DryRun then Manifest"`).

`migrateSweepApplyRun` installs the `--timeout` deadline ONCE:

```go
ctx, cancel := migrateWithTimeout(ctx, timeout)
defer cancel()

previewRes, err := st.Migrate(ctx, store.MigrateOptions{DryRun: true})
...
res, err := st.Migrate(ctx, store.MigrateOptions{Manifest: previewManifest})
```

and both `Store.Migrate` calls share that single context. The first call is a
full-backlog `DryRun` projection that, per `internal/store/migrate.go`'s own
doc comment, "issues materially more Qdrant traffic than an ordinary read-only
dry run" because it mints (and Count-probes) a real candidate short_id for
every record that needs one — work that is then thrown away, since the second
call mints its own short_ids from a fresh, empty `seen` set. So a single
`--apply` invocation's `--timeout` (default `5m`) must cover: one full-backlog
mint-heavy preview pass, PLUS one full-backlog apply pass that repeats the same
per-record minting work. On a backlog large enough that a single pass would
comfortably fit in the configured timeout, the doubled cost can push the
combined run past the deadline, producing a hard failure (`context deadline
exceeded`) where an operator who read only the flag's `"max wall-clock (0
disables)"` usage string would expect the timeout to bound one mutating pass.

This is not a data-loss risk — a `context.DeadlineExceeded` mid-apply fails
loudly, and un-migrated records simply stay in the backlog for the next run
(partial-write safety is intact, confirmed via `TestMigrateHonorsCancel` and
the delta-write design). It is a robustness/documentation gap: neither
`cli.md`'s "Request timeout" table nor `migrateCmd.Long`'s description
mentions that `--apply`'s effective budget covers two full sweeps, and no test
in `migrate_family_test.go` asserts the apply pass still has usable budget
after a slow preview.

**Fix:** Document the doubled cost in `migrateCmd.Long` and in
`docs-site/guides/cli.md`'s migrate section (e.g. "`--timeout` bounds the
WHOLE `--apply` invocation, including the internal re-preview pass, not just
the write"), and/or consider splitting the budget (e.g. half the timeout for
each internal call) so a configured `--timeout` bounds each pass rather than
their sum.

### WR-02: `RevertRefusalError` can emit two concatenated `field=`/`hint=` envelopes in one error, untested

**File:** `internal/store/revert.go:160-181` (`RevertRefusalError`)
**Issue:** CONFIRMED by reading the code; the combined case is not covered by
`internal/store/revert_test.go` or `cmd/engram/migrate_family_test.go`'s
`TestMigrateFamilyRevertRefusals` (which only exercises the irreversible-only
and unsupported-only cases as two SEPARATE table rows, never together).

`previewRevertWithSteps` derives `plan.Irreversible` (from records whose
reverse chain IS reachable but traverses a step declared irreversible) and
`plan.Unsupported` (from records whose reverse chain is NOT reachable at all)
independently, over the same above-target scroll. Nothing prevents a
whole-range revert from containing both kinds of record simultaneously (e.g.
some records at v1 with an irreversible v0->v1 step in their chain, and other
records at a stray future v42 with no chain at all). In that case
`RevertRefusalError` builds `parts` with BOTH clauses and joins them with
`"; "`:

```go
if len(plan.Irreversible) > 0 {
    parts = append(parts, fmt.Sprintf("field=steps hint=irreversible: ..."))
}
if len(plan.Unsupported) > 0 {
    parts = append(parts, fmt.Sprintf("field=record_version hint=unsupported: ..."))
}
return errors.New(strings.Join(parts, "; "))
```

The result is a single `error`/`revertOutputDoc.Refusal` string carrying TWO
`field=<name> hint=<code>: <text>` envelopes back to back. `docs-site/
reference/errors.md`'s "Operator-tier hint codes" table documents each code
individually and doesn't state whether they can co-occur in one refusal, and
the project's error-envelope contract (CLAUDE.md: "a rejected call names the
failing field and a machine-stable hint code in one envelope") is stated in
the singular. A caller (script or agent) that pattern-matches the FIRST
`field=`/`hint=` pair in the string and stops would silently miss the second
clause naming the unsupported-version records — not a data-loss risk (the
whole operation is still correctly refused either way, so no write happens),
but a plausible source of an operator misdiagnosing a refusal as
"irreversible-only" and reaching for the wrong remediation.

**Fix:** Add a test case that seeds both an irreversible-chain record and an
unsupported-version record in the same above-target range and asserts the
combined-refusal shape is intentional (e.g. that both `field=` markers are
present and a machine parser is expected to scan for all matches, not just
the first). If the intent is genuinely "one envelope per rejection," consider
picking a single reported class deterministically (e.g. unsupported takes
precedence) instead of concatenating.

## Info

### IN-01: `migrate` toolclass row omits any mention of the double-preview cost in its `Long` description

**File:** `internal/surfaces/toolclass.go:242-256`, `cmd/engram/migrate_family.go:237-247` (`migrateCmd.Long`)
**Issue:** CONFIRMED by reading both; related to WR-01 but purely a
documentation completeness note, not a code defect. `migrateCmd.Long` explains
the preview/apply-intersection contract (Spared/Appeared) well but never
mentions that `--apply` internally performs a full second projection pass
before writing, which is the root cause of WR-01's timeout-sharing surprise.
**Fix:** Fold a one-sentence note into the same `Long` string when WR-01 is
addressed, so `engram migrate --help` and the upgrade guide stay in sync with
whatever fix is chosen there.

---

_Reviewed: 2026-08-15T15:24:03Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
