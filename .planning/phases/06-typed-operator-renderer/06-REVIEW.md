---
phase: 06-typed-operator-renderer
reviewed: 2026-08-17T00:00:00Z
depth: deep
files_reviewed: 29
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
  - cmd/engram/flaggroup_test.go
  - cmd/engram/summarize_test.go
  - cmd/engram/testdata/catalog.golden
  - cmd/engram/testdata/help.golden
  - docs-site/src/content/docs/guides/cli.md
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 06: Code Review Report

**Reviewed:** 2026-08-17T00:00:00Z
**Depth:** deep
**Files Reviewed:** 29
**Status:** issues_found

## Summary

This is iteration 3 (final) of the fix/re-review loop on `feat/2026-08-12.01`. Per the orchestrator's
prior independent verification (treated as ground truth, not re-derived here): the `--all-scopes`
enforcement set is complete across all five commands that declare it (`spine-review
scan`/`verify`/`purge`/`consolidate`, `summarize-missing`), each now carries a real
`MarkFlagsMutuallyExclusive("scope", "all-scopes")` call, the new reverse-direction gate
(`TestEveryScopeAllScopesPairHasAFlagGroup`) is genuinely non-vacuous, and `task` (lint + all Go
packages + Python hook tests) is green.

This pass confirmed and characterized the one residual defect flagged for follow-up
(WR-04's half-applied fix, below), traced the golden-file regeneration from `1acf4a87` line-by-line
to confirm it changed exactly the two intended lines per file with no incidental drift, and read the
three newly-added/extended test files in full (`flaggroup_test.go`, `summarize_test.go`, and the
`shellQuote` round-trip addition in `spine_review_purge_test.go`). `shellQuote` itself
(`spine_review_purge.go:166-168`) is a correct, standard POSIX single-quote escape
(`'` → close-quote, escaped literal `'`, reopen-quote), and its round-trip test now covers all three
motivating shapes (space, embedded quote, embedded newline) by actually piping through `sh -c`
rather than string-matching the escaping scheme — no further defect found in that area.

One Warning remains open: `summarize-missing`'s `--all-scopes` usage string is the only one of the
five enforcement sites that does not state the exclusivity constraint in its own help text, even
though `1acf4a87` fixed the identical gap on `scan`/`verify`. One Info item records the missing
conformance-gate direction that would have caught this automatically.

## Warnings

### WR-01: `summarize-missing --all-scopes` usage text still doesn't state the exclusivity it enforces (WR-04 fix is 4-of-5, not 5-of-5)

**File:** `cmd/engram/summarize.go:116`
**Issue:**

`1acf4a87` ("WR-04 state --scope exclusivity in scan/verify --all-scopes usage text") fixed
`spine_review_scan.go:159` and `spine_review_verify.go:666` to append
`"; mutually exclusive with --scope"`, matching the phrasing already present on
`spine_review_purge.go:423`. `spine_review_consolidate.go:222-223` states the same constraint in a
different phrasing (`"(mutually exclusive with --scope)"`). `summarize.go:116` — added one commit
earlier, in `2fb63574` (WR-01 round 2), specifically to close this exact silent-narrowing hazard for
`summarize-missing` — never received the corresponding usage-text update:

```go
// cmd/engram/summarize.go:116 (current)
summarizeMissingCmd.Flags().BoolVar(&summarizeAllScopes, "all-scopes", false, "sweep every scope (required if --scope is omitted)")
```

vs. the fixed sibling sites, e.g.:

```go
// cmd/engram/spine_review_scan.go:159 (post-1acf4a87)
spineReviewScanCmd.Flags().BoolVar(&spineScanAllScopes, "all-scopes", false, "sweep every scope (required if --scope is omitted); mutually exclusive with --scope")
```

`summarize-missing`'s enforcement is real (`summarize.go:121` calls
`MarkFlagsMutuallyExclusive("scope", "all-scopes")`, and `TestSummarizeMissingScopeAndAllScopesRejected`
in `summarize_test.go:48` proves it rejects both at once) — this is a documentation/consistency defect,
not a behavioral one: `engram summarize-missing --help` is the one place among the five sibling
commands where an operator cannot learn the constraint without reading source or hitting the
rejection at runtime. This is the third occurrence of a half-applied N-site fix in this phase
(after the two rounds of WR-01 itself), which is itself worth noting as a pattern: N-site fixes in
this codebase have twice now shipped N-1.

Two related facts, confirmed directly rather than assumed:

- **(a) Gate coverage for this direction:** `TestEveryDeclaredExclusivityHasAFlagGroup`
  (`flaggroup_test.go:424`) only fires when a flag's *Usage string* already claims exclusivity
  (`flagsClaimedMutuallyExclusive` returns early on `len(peers) == 0` at `flaggroup_test.go:430`) —
  it cannot catch "usage text is silent about a constraint that IS enforced" because it never
  inspects the flag-group annotation unless the prose already mentions it.
  `TestEveryScopeAllScopesPairHasAFlagGroup` (`flaggroup_test.go:472`) checks the opposite fact — that
  a declared `--scope`/`--all-scopes` pair has a backing `MarkFlagsMutuallyExclusive` group — but
  never inspects `Usage` text at all. **Neither existing gate, nor any combination of the two, would
  fail on this exact defect.** This direction (a real flag group exists but no flag's Usage states it)
  is currently ungated.
- **(b) Golden-file impact:** confirmed both `testdata/help.golden:407` and
  `testdata/catalog.golden:966` currently pin the *unfixed* wording
  (`"sweep every scope (required if --scope is omitted)"`, no exclusivity clause) — i.e. the goldens
  are self-consistent with the present bug, not stale. Fixing `summarize.go:116`'s usage string
  requires regenerating both golden files (as `1acf4a87` already did correctly for the `scan`/`verify`
  sites — that diff changed exactly the two affected lines in each golden file with no incidental
  drift, confirmed via `git show 1acf4a87 -- cmd/engram/testdata/`).
- **(c) Target wording:** four sites currently disagree on phrasing —
  `scan`/`verify`/`purge` use `"; mutually exclusive with --scope"` (3 of 4), `consolidate` uses
  `"(mutually exclusive with --scope)"` (1 of 4). The semicolon form is both the majority and the
  better fit for `summarize-missing` specifically: `summarize.go:116`'s existing prefix
  (`"sweep every scope (required if --scope is omitted)"`) is verbatim identical to `scan.go`'s and
  `verify.go`'s pre-fix prefix, so appending the identical suffix they now carry
  (`"; mutually exclusive with --scope"`) produces byte-identical wording to two of the four sibling
  sites, not just a fix. Normalizing `consolidate.go`'s parenthetical phrasing to match is a
  reasonable follow-up for full five-way consistency but is a separate, lower-priority cosmetic
  change — out of scope for closing this specific gap.

**Fix:**
```go
// cmd/engram/summarize.go:116
summarizeMissingCmd.Flags().BoolVar(&summarizeAllScopes, "all-scopes", false, "sweep every scope (required if --scope is omitted); mutually exclusive with --scope")
```
Then regenerate `testdata/help.golden` and `testdata/catalog.golden` (the two `summarize-missing`
`all-scopes` usage lines: `help.golden:407`, `catalog.golden:966`) via this package's golden
regeneration path, and confirm `TestSummarizeMissingScopeAndAllScopesRejected` and the catalog/help
golden tests still pass.

## Info

### IN-01: The "flag group exists but usage text is silent" direction has no conformance gate

**File:** `cmd/engram/flaggroup_test.go` (new tests: `TestEveryDeclaredExclusivityHasAFlagGroup` at
line 424, `TestEveryScopeAllScopesPairHasAFlagGroup` at line 472)
**Issue:** As detailed in WR-01(a) above, this phase added two gates covering two directions of a
three-part invariant (usage claims exclusivity → group exists; scope+all-scopes pair exists → group
exists) but not the third (group exists → usage states it). That third direction is exactly what let
WR-01's own fix ship 4-of-5 in `1acf4a87`. Without a gate, a future flag-group addition elsewhere in
the tree can silently repeat this same half-applied-documentation pattern a fourth time.
**Fix:** Extend `TestEveryScopeAllScopesPairHasAFlagGroup` (or add a sibling test) so that, for every
command where `declaredGroupCoversPair(scope, "scope", "all-scopes")` is true, at least one of the two
flags' `Usage` strings is asserted to contain `"mutually exclusive"` (reusing
`flagsClaimedMutuallyExclusive`/its substring check already present in this file). This closes the
loop the way `TestEveryDeclaredExclusivityHasAFlagGroup` closes the other one, without introducing a
new hand-maintained command list — same `walkCommands(rootCmd, commandWalkSkip)` traversal already in
use.

---

_Reviewed: 2026-08-17T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
