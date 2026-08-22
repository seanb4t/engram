---
phase: 06-typed-operator-renderer
reviewed: 2026-08-17T00:00:00Z
depth: deep
files_reviewed: 27
files_reviewed_list:
  - cmd/engram/migrate.go
  - cmd/engram/migrate_family.go
  - cmd/engram/migrate_family_test.go
  - cmd/engram/operator_output.go
  - cmd/engram/operator_output_test.go
  - cmd/engram/operator_view.go
  - cmd/engram/operator_view_archive_purge_test.go
  - cmd/engram/operator_view_flat_test.go
  - cmd/engram/operator_view_migrate_test.go
  - cmd/engram/operator_view_scan_test.go
  - cmd/engram/operator_view_test.go
  - cmd/engram/prune_test.go
  - cmd/engram/reindex.go
  - cmd/engram/reindex_test.go
  - cmd/engram/spine_review_archive.go
  - cmd/engram/spine_review_archive_test.go
  - cmd/engram/spine_review_consolidate.go
  - cmd/engram/spine_review_consolidate_test.go
  - cmd/engram/spine_review_purge.go
  - cmd/engram/spine_review_purge_test.go
  - cmd/engram/spine_review_scan.go
  - cmd/engram/spine_review_test.go
  - cmd/engram/spine_review_verify.go
  - cmd/engram/spine_review_verify_test.go
  - cmd/engram/summarize.go
  - cmd/engram/testdata/catalog.golden
  - cmd/engram/testdata/help.golden
  - docs-site/src/content/docs/guides/cli.md
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 06: Code Review Report (iteration 2)

**Reviewed:** 2026-08-17
**Depth:** deep
**Files Reviewed:** 27 (+2 doc/golden data files)
**Status:** issues_found

## Summary

This is a re-review of iteration 1's five fixes (WR-01, WR-02, WR-03, IN-01, IN-02) on
`feat/2026-08-12.01`. Four of the five fixes are real, complete, and verified non-vacuous.
**One fix (WR-01) is incomplete**: the mutual-exclusivity constraint between `--scope` and
`--all-scopes` was applied to `spine-review scan`, `spine-review verify`, `spine-review purge`,
and `spine-review consolidate`, but **not** to `summarize-missing` (`cmd/engram/summarize.go`),
which registers the identical pair of flags with the identical silent-narrowing hazard. This
was empirically confirmed: `engram summarize-missing --scope x --all-scopes` is accepted by
cobra's flag validation and reaches `RunE` unrejected — the exact bug class WR-01 was meant to
close everywhere. This is exactly the recurring "half-applied fix" pattern this repo's own
process history flags, and no existing regression test (including the new
`TestEveryDeclaredExclusivityHasAFlagGroup` conformance gate) can catch it, because that gate
only checks the reverse direction (usage text claiming exclusivity with no enforcing flag group)
— `summarize-missing`'s `--all-scopes` usage text makes no such claim, so the gate is silent on
this site by construction.

Verification results for the other four fixes:
- **WR-02** (`TestOperatorViewFixturesHaveNoUnsanitizedNesting`): non-vacuous. It runs over
  `operatorViewFixtures()`, which is itself gated (`TestOperatorViewFixturesCoverEveryOperatorCommand`)
  to cover every live operator command derived from the cobra tree, not a hand-picked subset —
  a future doc field crossing the two-level-nesting boundary would fail this test loudly.
- **WR-03** (`shellQuote` + its two new tests): the escaping logic is correct for the empty
  string, embedded single quotes, and embedded shell metacharacters (`$`, backticks — single
  quotes suppress all of these). Both new tests (`TestShellQuoteRoundTripsThroughARealShell`,
  `TestPurgeRerunCommandQuotesFreeFormValues`) genuinely execute a real `sh` and would fail if
  `shellQuote` were reverted to a no-op — confirmed non-vacuous by tracing what `sh` would do
  with an unquoted value containing a space (`printf` reuses its format string over the extra
  positional arguments, producing a different string than the original). One coverage gap: the
  task's own worked example set (empty string, embedded quote, **embedded newline**) is not
  fully exercised — see IN-01 below.
- **IN-01**: docs-only, confirmed no behavior change.
- **IN-02**: confirmed present — `spine-review purge`'s `init()` now panics on a missing
  `RulePurgeFilterRequiresScope` registry entry, mirroring `requirePurgeFilterScope`'s own panic.

No golden file (`catalog.golden`, `help.golden`) or fixture pins `purgeReportDoc.Rerun` text, so
`shellQuote`'s introduction did not require (and did not receive) any golden-file update — verified
by grep; consistent.

## Warnings

### WR-01 (round 2): `summarize-missing` never got the `--scope`/`--all-scopes` mutual-exclusivity fix

**File:** `cmd/engram/summarize.go:113-121` (missing call; compare `cmd/engram/spine_review_scan.go:162`,
`cmd/engram/spine_review_verify.go:674`, `cmd/engram/spine_review_purge.go:445`,
`cmd/engram/spine_review_consolidate.go:231`, all of which now call `MarkFlagsMutuallyExclusive("scope", "all-scopes")`)

**Issue:** `summarizeMissingCmd` registers both `--scope` (line 115) and `--all-scopes` (line 116)
with the identical "one covers the gap the other doesn't" shape as scan/verify/purge/consolidate,
but never calls `MarkFlagsMutuallyExclusive("scope", "all-scopes")`. Confirmed reachable: an
invocation supplying both flags together passes cobra's flag validation and reaches `RunE`
unrejected (verified empirically — `summarize-missing --scope x --all-scopes --output yaml`
fails on the `--output` validator, not on flag-group validation, proving the pair was silently
accepted). Downstream, `internal/store/summarize.go`'s `SummarizeMissing` only ever reads
`opts.Scope` to build the Qdrant filter (`internal/store/summarize.go:135-137`) — `opts.AllScopes`
is never consulted — so supplying both flags silently narrows the sweep to `--scope`'s value
while an operator who intended `--all-scopes` to win (or who left a stray `--scope` in their
shell history/wrapper script) gets a scope-restricted sweep with no warning, and no way to tell
from the output that `--all-scopes` was silently discarded. This is the exact defect WR-01
fixed at the other four call sites; this fifth site was missed.

No existing test catches this: `flaggroup_test.go`'s `TestEveryDeclaredExclusivityHasAFlagGroup`
only fires when a flag's own Usage text contains the phrase "mutually exclusive" without a
backing flag group — `summarize-missing --all-scopes`'s usage string
(`"sweep every scope (required if --scope is omitted)"`) makes no such claim, so this gate is
silent on this specific gap by construction, not by oversight in the gate itself.

**Fix:**
```go
// cmd/engram/summarize.go, in init(), immediately after the --timeout registration:
summarizeMissingCmd.Flags().DurationVar(&summarizeTimeout, "timeout", 30*time.Minute, "max wall-clock for the sweep (0 disables); also cancellable via Ctrl-C")
summarizeMissingCmd.MarkFlagsMutuallyExclusive("scope", "all-scopes")
rootCmd.AddCommand(summarizeMissingCmd)
```
Add a regression test mirroring `TestSpineReviewScanScopeAndAllScopesRejected` /
`TestSpineReviewVerifyScopeAndAllScopesRejected` (`spine_review_test.go:61-76`,
`spine_review_verify_test.go:537-552`) for `summarize-missing`, and update
`docs-site/src/content/docs/guides/cli.md`'s `summarize-missing` section to state the same
`--scope`/`--all-scopes` mutual exclusivity the scan/verify sections now state.

### WR-04: `spine-review scan`/`verify --all-scopes` usage text still doesn't say "mutually exclusive with --scope"

**File:** `cmd/engram/spine_review_scan.go:159`, `cmd/engram/spine_review_verify.go:666`

**Issue:** Commit `093b4779` added the `MarkFlagsMutuallyExclusive` call to both leaves but left
their `--all-scopes` usage strings unchanged (`"sweep every scope (required if --scope is
omitted)"`), unlike `spine-review purge`/`consolidate`, whose `--all-scopes` usage strings
explicitly state `"(mutually exclusive with --scope)"` / `"; mutually exclusive with --scope"`.
This is a low-severity documentation inconsistency (an operator running `--help` on scan/verify
does not learn about the constraint from the flag's own usage line, only from `cli.md`), and it
is exactly the shape `TestEveryDeclaredExclusivityHasAFlagGroup` cannot flag in this direction
(it only fails when usage text claims a constraint the code doesn't enforce, never the reverse).

**Fix:** Append `"; mutually exclusive with --scope"` to both usage strings, matching purge/
consolidate's wording, e.g.:
```go
spineReviewScanCmd.Flags().BoolVar(&spineScanAllScopes, "all-scopes", false, "sweep every scope (required if --scope is omitted); mutually exclusive with --scope")
```

## Info

### IN-03: `TestShellQuoteRoundTripsThroughARealShell` doesn't exercise an embedded newline

**File:** `cmd/engram/spine_review_purge_test.go:294-323`

**Issue:** The task that motivated this iteration's re-review explicitly names embedded
newlines as one of the three shapes `shellQuote` must handle correctly (alongside the empty
string and an embedded single quote). The current `cases` table (`"plain"`, `"has a space"`,
`"has'a'quote"`, `"O'Brien's scope"`, `""`) covers the first two but has no case containing
`\n`. The implementation is in fact correct for a newline (POSIX single-quoting preserves a
literal newline verbatim — it does not act as a command terminator inside `'...'`), so this is
a test-coverage gap rather than a live bug, but it's worth closing so a future regression in
this exact dimension isn't silently missed.

**Fix:**
```go
cases := []string{
    "plain",
    "has a space",
    "has'a'quote",
    "O'Brien's scope",
    "",
    "line one\nline two",
}
```

---

_Reviewed: 2026-08-17_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
