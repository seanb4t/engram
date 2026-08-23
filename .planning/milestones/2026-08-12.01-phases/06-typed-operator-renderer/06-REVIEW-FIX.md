---
phase: 06-typed-operator-renderer
fixed_at: 2026-08-17T20:20:00Z
review_path: .planning/phases/06-typed-operator-renderer/06-REVIEW.md
iteration: 3
findings_in_scope: 10
fixed: 10
skipped: 0
status: all_fixed
---

# Phase 06: Code Review Fix Report (cumulative, iteration 3 — final)

**Fixed at:** 2026-08-17T20:20:00Z
**Source review:** .planning/phases/06-typed-operator-renderer/06-REVIEW.md
**Iteration:** 3 (final)

**Summary (cumulative across all three fix/re-review cycles):**
- Findings in scope: 10 (5 in iteration 1, 3 in iteration 2, 2 in iteration 3)
- Fixed: 10
- Skipped: 0

**Iteration 3 summary:** the re-review found that iteration 2's WR-01 fix
(closing the `summarize-missing` `MarkFlagsMutuallyExclusive` gap) had again
shipped 4-of-5, in a new dimension: the flag group itself was real, but
`summarize-missing`'s `--all-scopes` usage text — unlike the other four
sibling sites — never stated the exclusivity constraint in prose. This is
the third occurrence of this exact half-applied-N-site-fix pattern in this
phase. This iteration closes that one remaining site (WR-01) and, more
importantly, closes the conformance-gate hole that let it recur a third
time (IN-01): a new test direction now asserts that any command with a real
`--scope`/`--all-scopes` flag group also states the constraint in at least
one of the pair's Usage strings, proved non-vacuous by an explicit
red/green cycle.

**Verification environment:** all edits, builds, and test runs happened in
an isolated git worktree
(`gsd-reviewfix/06-1087`, based on `feat/2026-08-12.01` at `9379c616`),
fast-forwarded into `feat/2026-08-12.01` and torn down at the end of this
run per the standard review-fix worktree protocol. `task` (lint + Go tests
across every package + the Python skill-hook suite) ran to completion once
inside that worktree with zero failures, including `internal/store`'s
`TestRedEvidencePatchesAreLive` (153.1s) and every `cmd/engram` flag-group
test. No concurrent working-tree mutation occurred during that run — the
constraint that broke iteration 2's first attempt was respected throughout.
The numbers below are reproducible from `feat/2026-08-12.01` after this
fast-forward.

## Fixed Issues — Iteration 3 (this run, final)

### WR-01: `summarize-missing --all-scopes` usage text still doesn't state the exclusivity it enforces (round 3, closing the 4-of-5 gap for the third and last time)

**File:** `cmd/engram/summarize.go:116`
**Files modified:** `cmd/engram/summarize.go`, `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden`
**Commit:** `75e284db`
**Applied fix:** Appended `"; mutually exclusive with --scope"` to
`summarize.go:116`'s `--all-scopes` usage string, producing wording
byte-identical to the fixed `scan`/`verify` sites from `1acf4a87` (iteration
2), per the review's explicit instruction not to touch `consolidate`'s
differing parenthetical phrasing (left untouched, as scoped). Regenerated
`testdata/help.golden` and `testdata/catalog.golden` via `go test
./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -update -count=1` (the
same minimal regeneration path `1acf4a87` used, not the full `task
surfaces:gen`) and confirmed via `git diff` the resulting diff touched
exactly the two intended lines (one in each golden file) with no incidental
drift. `TestSummarizeMissingScopeAndAllScopesRejected`,
`TestHelpGolden`, and `TestCatalogGolden` all pass.

### IN-01: The "flag group exists but usage text is silent" direction has no conformance gate

**File:** `cmd/engram/flaggroup_test.go`
**Files modified:** `cmd/engram/flaggroup_test.go`
**Commit:** `1bffd28d`
**Applied fix:** Extended `TestEveryScopeAllScopesPairHasAFlagGroup` (rather
than adding a sibling test, since it already walks the exact command set
this check needs) with the third invariant direction: for every command
where `declaredGroupCoversPair(scope, "scope", "all-scopes")` is true, at
least one of `--scope`'s or `--all-scopes`'s `Usage` strings must contain
the phrase "mutually exclusive", reusing the existing
`flagsClaimedMutuallyExclusive` substring-matching machinery already in the
file rather than writing a new matcher. The command set is still derived by
walking the live cobra tree (`walkCommands(rootCmd, commandWalkSkip)`) —
never a hand-maintained list. Confirmed the matcher tolerates both phrasings
live in the tree without forcing any wording change at `consolidate`: its
`"(mutually exclusive with --scope)"` parenthetical and the four other
sites' `"; mutually exclusive with --scope"` form both satisfy the plain
substring check, and `consolidate` was left untouched.

**Proved the new gate is non-vacuous (per task instructions):** temporarily
reverted WR-01's usage-string change in `summarize.go` (restoring `"sweep
every scope (required if --scope is omitted)"` with no exclusivity clause),
ran `TestEveryScopeAllScopesPairHasAFlagGroup` — it failed exactly as
expected, with the subtest `TestEveryScopeAllScopesPairHasAFlagGroup/summarize-missing`
reporting: `summarize-missing declares a real --scope/--all-scopes flag
group, but neither flag's Usage text states the constraint (no "mutually
exclusive" phrase found) — an operator running --help alone cannot learn
this`. All four other sites (`spine-review scan/verify/purge/consolidate`)
continued to pass in that same run, confirming the gate isolates the exact
defective site rather than failing broadly. Restored the WR-01 fix and
reconfirmed the full test green again (`git diff --stat` showed only
`flaggroup_test.go` changed at that point — the restore was byte-exact).
This observation is reported honestly per the task instructions, not just
claimed: the gate is genuinely non-vacuous, not decorative.

## Fixed Issues — Iteration 2 (prior run, preserved for cumulative record)

### WR-01 (round 2): `summarize-missing` never got the `--scope`/`--all-scopes` mutual-exclusivity fix

**Files modified:** `cmd/engram/summarize.go`, `cmd/engram/summarize_test.go`, `cmd/engram/flaggroup_test.go`, `docs-site/src/content/docs/guides/cli.md`
**Commit:** `2fb63574`
**Applied fix:** Derived the complete set of commands registering both `--scope` and `--all-scopes` from the live cobra registrations. Only `summarize-missing` lacked `MarkFlagsMutuallyExclusive("scope", "all-scopes")`; added it in `init()`. Added `TestSummarizeMissingScopeAndAllScopesRejected`. Added the reverse-direction gate `TestEveryScopeAllScopesPairHasAFlagGroup` (pair-declared ⇒ group-exists), proved non-vacuous by explicit red/green cycle at the time. **Note (superseded by iteration 3):** the usage-text prose for `summarize-missing` was still missed at this point — see WR-01 (round 3) above.

### WR-04: `spine-review scan`/`verify`'s `--all-scopes` usage strings don't state the constraint

**Files modified:** `cmd/engram/spine_review_scan.go`, `cmd/engram/spine_review_verify.go`, `cmd/engram/testdata/help.golden`, `cmd/engram/testdata/catalog.golden`
**Commit:** `1acf4a87`
**Applied fix:** Appended `"; mutually exclusive with --scope"` to both `--all-scopes` usage strings. Regenerated goldens; diff confirmed scoped to exactly the two changed lines.

### IN-03: `TestShellQuoteRoundTripsThroughARealShell` doesn't exercise an embedded newline

**Files modified:** `cmd/engram/spine_review_purge_test.go`
**Commit:** `9379c616`
**Applied fix:** Added `"line one\nline two"` to the test's `cases` table. Confirmed `shellQuote`'s POSIX single-quote wrapping already preserves an embedded newline verbatim — a genuine test-coverage gap closed, not a live bug.

## Fixed Issues — Iteration 1 (prior run, preserved for cumulative record)

### WR-01: `spine-review scan` and `spine-review verify` silently ignored `--all-scopes` when `--scope` was also supplied

**Files modified:** `cmd/engram/spine_review_scan.go`, `cmd/engram/spine_review_verify.go`, `cmd/engram/spine_review_test.go`, `cmd/engram/spine_review_verify_test.go`, `docs-site/src/content/docs/guides/cli.md`
**Commit:** `093b4779`
**Applied fix:** Registered `cmd.MarkFlagsMutuallyExclusive("scope", "all-scopes")` in both leaves' `init()`. Added `TestSpineReviewScanScopeAndAllScopesRejected` and `TestSpineReviewVerifyScopeAndAllScopesRejected`. **Note (superseded across two later rounds):** this fix covered only 4 of the 5 sites sharing the identical defect shape — `summarize-missing` was missed and required both a round-2 behavioral fix (`2fb63574`) and a round-3 usage-text fix (`75e284db`) to fully close; see above.

### WR-02: `sanitizeViewValue` (T-06-03's mitigation) is bypassed for nested-container values

**Files modified:** `cmd/engram/operator_view.go`, `cmd/engram/operator_output_test.go`
**Commit:** `36dd3542`
**Applied fix:** No behavioral change to `viewScalar` (confirmed the gap is unreachable by any current operator report struct). Added `TestOperatorViewFixturesHaveNoUnsanitizedNesting`.

### WR-03: `purgeRerunCommand` built an unquoted, unsafe-to-copy-paste re-run string

**Files modified:** `cmd/engram/spine_review_purge.go`, `cmd/engram/spine_review_purge_test.go`
**Commit:** `263b7bb3`
**Applied fix:** Added a `shellQuote` helper (POSIX single-quote escaping) and applied it to `--scope`, `--category`, and `--tags` interpolation. Added `TestShellQuoteRoundTripsThroughARealShell` and `TestPurgeRerunCommandQuotesFreeFormValues`.

### IN-01 (iteration 1): `spineArchiveOrRestore`/`renderArchiveResults` discard already-collected per-id results on a mid-batch abort

**Files modified:** `cmd/engram/spine_review_archive.go`
**Commit:** `815d2612`
**Applied fix:** Docs-only per instructions — extended doc comments to state the asymmetry explicitly and why. (Note: this is a distinct finding from iteration 3's IN-01 above — REVIEW.md finding IDs restart each review iteration and are not globally unique across iterations.)

### IN-02: `spine-review purge`'s `init()` swallowed a missing-rule lookup that the same file treats as a fail-fast invariant elsewhere

**Files modified:** `cmd/engram/spine_review_purge.go`
**Commit:** `73ecfebc`
**Applied fix:** The `init()`-time `surfaces.RuleByID(surfaces.RulePurgeFilterRequiresScope)` lookup now panics on a missing rule, mirroring `requirePurgeFilterScope`'s own panic.

## Skipped Issues

None — all 10 findings across all three iterations were fixed.

## Notes on this fix session (iteration 3)

- Every fix in this iteration was proved either by a passing existing
  regression test class it mirrors, a new regression test, or (for the new
  conformance gate) an explicit red/green cycle recorded above — no finding
  was applied "blind."
- The golden-file regeneration path used
  (`go test ./cmd/engram -run 'TestHelpGolden|TestCatalogGolden' -update
  -count=1`) matches the minimal path `1acf4a87` used in iteration 2, not
  the broader `task surfaces:gen` (which also chains `proto:gen`) — kept
  the diff scoped to exactly the two intended lines, per the task
  instructions.
- No working-tree mutation (`git checkout --`, `git restore`) occurred
  while any test run was in flight in this session.

---

_Fixed: 2026-08-17T20:20:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 3 (final)_
