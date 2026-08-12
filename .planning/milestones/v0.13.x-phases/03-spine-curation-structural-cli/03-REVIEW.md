---
phase: 03-spine-curation-structural-cli
reviewed: 2026-08-07T21:09:27Z
depth: standard
files_reviewed: 35
files_reviewed_list:
  - cmd/engram/spine_review.go
  - cmd/engram/spine_review_scan.go
  - cmd/engram/spine_review_verify.go
  - cmd/engram/spine_review_consolidate.go
  - cmd/engram/spine_review_archive.go
  - cmd/engram/spine_review_purge.go
  - cmd/engram/cmdwalk.go
  - cmd/engram/destructive.go
  - cmd/engram/operator_output.go
  - cmd/engram/catalog.go
  - cmd/engram/client_common.go
  - cmd/engram/prune.go
  - cmd/engram/migrate.go
  - cmd/engram/reindex.go
  - cmd/engram/summarize.go
  - cmd/engram/backfill.go
  - internal/store/spine.go
  - internal/store/store.go
  - internal/surfaces/rules.go
  - internal/surfaces/toolclass.go
  - internal/config/client_validate.go
  - internal/server/tools.go
  - cmd/engram/spine_review_test.go
  - cmd/engram/spine_review_archive_test.go
  - cmd/engram/spine_review_consolidate_test.go
  - cmd/engram/spine_review_purge_test.go
  - cmd/engram/spine_review_verify_test.go
  - cmd/engram/destructive_test.go
  - cmd/engram/operator_output_test.go
  - internal/store/spine_test.go
  - internal/store/spine_forgery_test.go
  - internal/store/store_test.go
  - internal/e2e/spine_review_test.go
  - docs-site/src/content/docs/guides/cli.md
  - docs-site/src/content/docs/reference/memory-record.md
findings:
  critical: 0
  warning: 2
  info: 0
  total: 2
status: issues_found
---

# Phase 3: Code Review Report

**Reviewed:** 2026-08-07T21:09:27Z
**Depth:** standard
**Files Reviewed:** 35
**Status:** issues_found

## Summary

This review targeted the eleven load-bearing invariants named in the review brief rather than
re-litigating already-converged surface issues. Each was traced through the actual merged code,
not assumed from doc comments:

- **Subject-less collection-wide sweeps** — confirmed. `ScanSpine`, `EnumerateCitations`,
  `NearDuplicates`, `derivePurgeEligible`/`PreviewPurge`/`ApplyPurge` take no `Subject` and never
  compose `Search`/`List`/`SearchDiscovery`/`ListScheduled`.
- **Pagination** — confirmed. `scrollAllPoints` (`internal/store/spine.go:46`) is the only
  paginated whole-spine loop; every other `client.Scroll(` call site in `store.go` is a
  pre-existing, explicitly-`Limit`-bounded single page for a Subject-gated `List`/`ListScheduled`
  read, not a second copy-pasted whole-spine sweep.
- **`PurgeManifest` unforgeability** — confirmed by direct reflection test
  (`spine_forgery_test.go`) plus manual trace: `ApplyPurge` rejects `!manifest.IsVerified()` before
  touching `s.client` (`spine.go:1318`), proven by a nil-client `Store` in the forgery test.
- **Intersection-only delete** — confirmed and directly unit-tested
  (`TestPurgeIntersectionSparesIneligibleReportsAppeared`).
- **Derived, not declared** classification, and **`registerDestructive` owns `RunE`** — confirmed;
  no injectable classification seam exists, and `applyRequested` reads the flag's value, never
  `Changed` (tested at `destructive_test.go:189`).
- **Archive orthogonality** — confirmed: `archived_at` IsEmpty is a sibling condition at all four
  recall sites (`Search`, `SearchDiscovery`, `List` via `listFilter`, `ListScheduled`) plus the
  expiry filter, never folded into `activeWindowConditions`.
- **Concurrency** — confirmed: `Archive`/`Restore` take `s.locker.Lock(ctx, id)` and `Update`'s
  in-lock re-read carries `ArchivedAt` forward, matching the CR-04 supersession fix exactly.
- **Path safety in `verify`** — confirmed: `resolveContainedRef` rejects absolute refs, resolves
  symlinks via `filepath.EvalSymlinks`, and checks containment via `filepath.Rel` (never a lexical
  prefix check), including the "file missing" case via `deepestExistingAncestor`.
- **Error routing** — confirmed: every new `RunE` routes through `classifyOperatorErr`/
  `classifyOperatorErrConstruction`/`usageErrorf`; no bare `os.Exit` or unclassified `fmt.Errorf`
  found on any new path.
- **`--timeout` divergence** — confirmed unchanged: client tier still rejects `0`
  (`internal/config/client_validate.go:33`), the operator tier (including all six new
  `spine-review` leaves) still treats `0` as unbounded, and `migrate-remap-owner`/
  `migrate-set-owner` still reject `0` explicitly.

Two issues surfaced that are real, evidenced divergences rather than nits — both are documentation/
contract-vs-implementation mismatches, not data-loss or security bugs (both fail toward stricter or
more confusing behavior, not toward silent data exposure).

## Warnings

### WR-01: `purge`'s filter-path scope gate is stricter than the rule sentence and CLI guide document

**File:** `internal/store/spine.go:910-912` (`PurgeFilterPathActive`), `internal/surfaces/rules.go:254`
(`RulePurgeFilterRequiresScope.Sentence`), `docs-site/src/content/docs/guides/cli.md:281-283`

**Issue:** The published rule sentence and the CLI guide both state the harder `--scope`/
`--all-scopes` requirement applies to "category, tags, or older-than **with no class selected**" —
i.e., selecting a `--class` should exempt `--category`/`--tags` from the harder gate, the same way
it exempts `--older-than`. The actual implementation only extends that exemption to `--older-than`:

```go
func PurgeFilterPathActive(opts PurgeOptions) bool {
	return opts.Category != "" || len(opts.Tags) > 0 || (opts.OlderThan != 0 && len(opts.Classes) == 0)
}
```

`opts.Category != ""` and `len(opts.Tags) > 0` are unconditional — they trigger the harder gate
(and `requirePurgeFilterScope`'s rejection) even when one or more `Classes` is also selected. Only
the `OlderThan` disjunct is conditioned on `len(opts.Classes) == 0`.

Concrete repro: `engram spine-review purge --class expired --category decision` (no `--scope`/
`--all-scopes`). Per the documented sentence ("...with no class selected...", implying a supplied
class exempts the whole filter-path trio), this should be accepted as a class-only-parameterized
run. In the actual binary it is rejected with a usage error (exit 2) demanding `--scope` or
`--all-scopes`, because `Category != ""` alone sets `PurgeFilterPathActive` true regardless of
`Classes`.

This exact combination (`Classes` non-empty **and** `Category`/`Tags` also set) is untested:
`spine_review_purge_test.go`'s `TestRequirePurgeFilterScope` table only exercises class-only,
class+older-than, and category/tags-with-no-class cases — never class+category together — so the
divergence between prose and code shipped unnoticed.

The failure direction is safe (the code is stricter than documented, never more permissive), but an
operator following the CLI guide or `--help` text literally will hit a confusing rejection the docs
told them not to expect.

**Fix:** Pick one behavior and make the other match it. Either (a) extend the `Classes`-present
exemption to `Category`/`Tags` in `PurgeFilterPathActive` to match the documented sentence:

```go
func PurgeFilterPathActive(opts PurgeOptions) bool {
	if len(opts.Classes) > 0 {
		return false
	}
	return opts.Category != "" || len(opts.Tags) > 0 || opts.OlderThan != 0
}
```

(if this is chosen, re-verify `derivePurgeEligible`'s own `filterPath` branch, which currently ORs
the free-form match into eligibility even when classes are also selected — that OR-in-additional-
eligibility behavior would need to move behind the same `len(opts.Classes) == 0` condition to stay
consistent), or (b) correct the rule sentence and CLI guide to state plainly that `--category`/
`--tags` always require `--scope`/`--all-scopes` regardless of `--class`, and add a
`TestRequirePurgeFilterScope` case pinning `{Classes: [...], Category: "x"}` → `wantErr: true` so
the actual behavior is locked in by a test rather than left to silently diverge from prose again.

### WR-02: `spine-review archive`/`restore`'s per-id report echoes an inconsistent id representation

**File:** `cmd/engram/spine_review_archive.go:92-116` (`spineArchiveOrRestore`)

**Issue:** `spineArchiveOrRestore` resolves each caller-supplied id (which may be a short_id) via
`st.ResolvePointID`, then calls `fn` (`Store.Archive`/`Store.Restore`) with the **resolved**
canonical UUID. `Store.Archive`/`Store.Restore` build their `ArchiveResult{ID: id, ...}` from that
resolved id, so a `changed`/`already` outcome's `id` field in the report is always the canonical
UUID — never what the operator typed. But the `not_found` path (when `ResolvePointID` itself fails)
appends `store.ArchiveResult{ID: raw, ...}` using the **original, unresolved** caller input:

```go
resolved, rerr := st.ResolvePointID(ctx, raw)
if rerr != nil {
    if !errors.Is(rerr, store.ErrNotFound) {
        return results, rerr
    }
    results = append(results, store.ArchiveResult{ID: raw, Outcome: store.ArchiveOutcomeNotFound})
    ...
    continue
}
res, rerr := fn(ctx, resolved)   // fn returns ArchiveResult{ID: resolved, ...}
```

Concrete repro: `engram spine-review archive --id <shortid-that-resolves> --id <typo-shortid>`
produces a report where the first row's `id` is a full UUID the operator never typed, and the
second row's `id` is the literal short_id string they typed. A caller scripting against the JSON
output and matching `results[].id` back to the `--id` values it supplied (rather than relying on
positional order, which the `ArchiveResult` type gives no field to distinguish from) will fail to
find a match for every successfully-resolved short_id. `ArchiveResult` carries no `ShortID` field
(unlike `CitationRecord`, which explicitly carries both `ID` and `ShortID` for exactly this
reason), so there is no way for a JSON consumer to recover the original short_id for a
`changed`/`already` row at all. This exact path (archiving via a short_id, mixed with a not-found
id in the same invocation) is untested — `spine_review_archive_test.go` only ever constructs
`store.ArchiveResult{ID: "id-1", ...}` fixtures directly, never through `spineArchiveOrRestore`
with real short_id resolution.

**Fix:** Either echo the caller's original (raw) id consistently across all three outcomes —
`spineArchiveOrRestore` already has `raw` in scope in the loop, so thread it through instead of
`resolved` when building the reported `ArchiveResult` — or add a `ShortID`/`InputID` field to
`ArchiveResult`/`archiveResultDoc` so both the canonical id and the caller's original token are
always present, matching `CitationRecord`'s existing (ID, ShortID) pattern:

```go
res, rerr := fn(ctx, resolved)
if rerr != nil && res.Outcome == "" {
    return results, rerr
}
res.ID = raw // report what the operator supplied, not the resolved canonical form
results = append(results, res)
```

---

_Reviewed: 2026-08-07T21:09:27Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
