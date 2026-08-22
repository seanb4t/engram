---
phase: 08-registry-docs-tail
reviewed: 2026-08-21T23:59:00Z
depth: standard
files_reviewed: 16
files_reviewed_list:
  - CLAUDE.md
  - cmd/engram/spine_review_scan.go
  - cmd/engram/spine_review_verify.go
  - cmd/engram/summarize.go
  - cmd/engram/summarize_test.go
  - cmd/engram/sweep_scope.go
  - cmd/engram/sweep_scope_test.go
  - cmd/engram/testdata/catalog.golden
  - cmd/engram/testdata/help.golden
  - docs-site/src/content/docs/guides/migrate.md
  - docs-site/src/content/docs/guides/upgrade.md
  - docs-site/src/content/docs/reference/memory-record.md
  - docs-site/src/content/docs/reference/tools.md
  - internal/surfaces/normalize_test.go
  - internal/surfaces/rules.go
  - internal/surfacesgen/main.go
findings:
  critical: 0
  warning: 6
  info: 7
  total: 13
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-08-21
**Depth:** standard
**Files Reviewed:** 16
**Status:** issues_found

## Summary

This is a re-review of the same 16-file phase scope after two gap-closure
plans (08-05, 08-06) landed on top of the prior review (08-REVIEW.md,
2026-08-21). Only three files actually changed since that review:
`CLAUDE.md` (08-05), `internal/surfaces/rules.go` (08-06, doc-comment-only),
and `cmd/engram/sweep_scope_test.go` (08-06). The other 13 files are
byte-identical to what the prior review already assessed
(`git log -- <files>` confirms no commits touch them between the prior
review and `HEAD`).

**What I verified as fixed.** Four of the prior review's findings are
closed, and I did not take the SUMMARYs' word for it — I re-derived each
claim from source:

- **WR-02** (rules.go's false present-tense error-envelope claim): the
  retired sentence ("it drives field=scope attribution") is gone; the
  rewritten comment states the rule is CLI-only, names the sole
  enforcement site (`requireSweepScope`, bare `usageErrorf`), and states
  the field=/hint= envelope is inert today. I confirmed independently that
  `internal/server` has zero references to
  `surfaces.RuleSweepScopeOrAllScopesRequired` (`rg` across
  `internal/server/*.go`), and that the comment's non-vacuity control claim
  ("control rule: 3, not 0") is accurate — `RuleScopeRequiredUnlessCrossSpine`
  is referenced exactly 3 times in `internal/server/tools.go`. The two
  `spine.go` line citations the rewritten comment adds (`:384-387` for
  `NearDuplicates`, `:991` for `derivePurgeEligible`) both resolve to the
  claimed functions.
- **WR-03** (no durable zero-occurrence gate): `TestNoHandRolledSweepScopeGuards`
  now exists in `cmd/engram/sweep_scope_test.go`, runs inside `go test
  ./...`, and derives its comparison set from `walkCommands(rootCmd,
  commandWalkSkip)` — never a hand-listed set. I did not just read the
  code; I mutation-tested it myself in both directions rather than trusting
  the plan SUMMARY's own RED-output transcript: (1) removing
  `summarize-missing` from `enforcingSweepLeaves` reproduces the "extra"
  failure (`exposes --scope and --all-scopes but is in neither ... set`);
  (2) adding a nonexistent classified key reproduces the "missing" (stale)
  failure (`... but not found in the live command tree`). Both failed as
  expected; the working tree was restored clean afterward
  (`git status --porcelain` empty). The gate is genuinely non-vacuous, not
  merely observed-passing.
- **WR-07** (CLAUDE.md's archived-state paragraph named only 2 of 4 recall
  surfaces): now reads `search_memory`/`list_memory`/`search_discovery`/
  `list_scheduled` — all four, matching the adjacent Supersession paragraph
  and the four live `qdrant.NewIsEmpty("archived_at")` gate sites in
  `internal/store/store.go`.
- **WR-09** (CLAUDE.md's Layout row omitted that `backfill-short-ids` is
  deprecated): now reads `` `backfill-short-ids` (deprecated, use
  `migrate`) ``, consistent with `backfillShortIDsCmd.Deprecated` in
  `cmd/engram/backfill.go:58`.

`go test ./cmd/engram/... ./internal/surfaces/...` (targeted at the
sweep-scope and rule-resolution tests) is green; `go build ./...` succeeds;
the working tree is clean apart from this report.

**What is still open.** Per the review brief, findings already recorded
against unchanged files are not re-derived here, but they remain real and
are carried forward by reference rather than dropped silently, since this
file supersedes the prior 08-REVIEW.md as the phase's review record:

- **WR-01** (`cmd/engram/spine_review_scan.go:152`,
  `spine_review_verify.go:665`, `summarize.go:116`): the `--all-scopes`
  Usage string on all three enforcing leaves still states the "required if
  `--scope` is omitted" constraint twice — once hand-typed, once via the
  composed `Sentence`. Unchanged since the prior review.
- **WR-04** (`internal/surfaces/normalize_test.go`): the `SurfaceFields`
  narrowing for the new rule still has no package-level unit test proving
  the `{scope, all-scopes}`-without-`dry-run` exclusion; it is proven only
  by the CLI-package whitelist. Unchanged.
- **WR-05** (`internal/surfacesgen/main.go:123-136`,
  `docs-site/.../reference/tools.md:528,536`): the prose-surface gate for
  this rule still hangs on the incidental `dry-run` mention surviving in
  `tools.md`/`cli.md`; no direct applicability assertion exists. Unchanged.
- **WR-06** (`docs-site/.../guides/cli.md`): still documents mutual
  exclusivity for the three sweep leaves without documenting that
  omitting both flags is a hard rejection (as opposed to `consolidate`'s
  neighboring "supplying neither sweeps a well-defined empty result").
  Unchanged (this file is not in the 16-file scope for this pass, but the
  cross-reference from `rules.go`'s comment still applies).
- **WR-08** (`CLAUDE.md:15`): `migrate-remap-owner (alias:
  migrate-set-owner, deprecated)` still uses "alias" loosely for a command
  with an incompatible flag set. Per this review's `<known_false_positives>`
  brief, the underlying deprecated-supersession relationship IS correct and
  pinned (`TestMigrateSetOwnerEquivalentToRemapOwnerMissing`) — this is
  purely a wording-precision note, downgraded to Info here since two prior
  reviewers already over-called it as a functional defect and 08-VERIFICATION.md
  classified it info-level-only.
- **WR-10** (`cmd/engram/summarize_test.go:40-56`): the doc comment still
  describes a "bare usageErrorf guard" that 08-01 already replaced, and the
  test is still a strictly weaker duplicate of
  `TestSweepLeavesRejectMissingScopeIdentically/summarize-missing`.
  Unchanged.
- **IN-01 through IN-07**: unchanged; see prior 08-REVIEW.md for detail
  (consolidate's false-clean report cemented as an invariant; the
  non-enforcement assertion only inspects one flag; no per-rule pin test
  for the new rule; `CLAUDE.md`'s `migrate` phrasing parallels a
  parent-only command; `tools.md`'s required-ness callout was demoted from
  bold to prose; `migrate.md` doesn't call `backfill-short-ids` deprecated;
  `tools.md` lists `list_scheduled` among tools that "hide" scheduled
  records, which is backwards for that one state).

No new defect was introduced by the three changed files. No BLOCKER-class
defect exists in this phase's scope.

## Warnings

### WR-01 (carried forward, unchanged): The rule is stated twice on every `--all-scopes` flag

**File:** `cmd/engram/spine_review_scan.go:152`, `cmd/engram/spine_review_verify.go:665`, `cmd/engram/summarize.go:116`

**Issue:** Each Usage string still reads `"sweep every scope (required if
--scope is omitted); mutually exclusive with --scope; " +
sweepScopeRule().Sentence`, restating the same constraint twice — once
hand-typed, once from the registry. See prior 08-REVIEW.md WR-01 for full
detail; unchanged by this gap-closure wave.

**Fix:** Drop the hand-typed "required if --scope is omitted" clause,
keeping only the composed Sentence; regenerate goldens.

### WR-04 (carried forward, unchanged): The `SurfaceFields` narrowing has no unit-level negative proof

**File:** `internal/surfaces/normalize_test.go:92-98,107`

**Issue:** No test in `internal/surfaces` mirrors
`TestDiscoveryNotSchedulableExcludesCategoryOnlySurfaces` for this rule —
the narrowing that excludes `spine-review consolidate`/`purge` (which
expose `scope`+`all-scopes` without `dry-run`) is proven only by the
CLI-package hand-maintained whitelist (`enforcingSweepLeaves`/
`nonEnforcingSweepLeaves` in `cmd/engram/sweep_scope_test.go`), not at the
package level where the narrowing is declared.

**Fix:** Add `TestSweepScopeRuleExcludesScopeAndAllScopesOnlySurfaces`
mirroring the discovery test — see prior 08-REVIEW.md WR-04 for the full
suggested implementation.

### WR-05 (carried forward, unchanged): The prose-surface gate for this rule hangs on one incidental docs table row

**File:** `internal/surfacesgen/main.go:123-136`, `docs-site/src/content/docs/reference/tools.md:528,536`

**Issue:** `SurfaceDocsSite` resolves applicable for this rule only because
`dry-run`/`dry_run` happens to appear in `tools.md`'s summarize-missing
flag table. If that mention is ever reworded away, the check for this rule
silently stops running with the suite still green.

**Fix:** Assert applicability directly rather than deriving the gate's
existence from a `strings.Contains` hit in prose — see prior 08-REVIEW.md
WR-05.

### WR-06 (carried forward, unchanged): `guides/cli.md` documents mutual exclusivity but not required-ness for the three sweep leaves

**File:** `docs-site/src/content/docs/guides/cli.md:161-164,179-181,206-216` (vs. `:229-236`)

**Issue:** The operator guide states only "mutually exclusive" for
`summarize-missing`/`spine-review scan`/`spine-review verify`, while the
neighboring `spine-review consolidate` entry explicitly says supplying
neither is a supported (if narrow) mode. Read together, this misleads an
operator into believing omitting both flags is safe on all four; on three
of them it is a hard exit-2 rejection.

**Fix:** Add an anchored region (or shared paragraph) documenting the
required-ness for the three enforcing leaves; wire it through
`ruleTargets` in `internal/surfacesgen/main.go` so `task surfaces:gen`
keeps it in sync — see prior 08-REVIEW.md WR-06.

### WR-10 (carried forward, unchanged): `summarize_test.go`'s test is a weaker duplicate with a stale comment

**File:** `cmd/engram/summarize_test.go:40-56`

**Issue:** The doc comment claims to pin "the bare usageErrorf guard...
before the guard is converted onto the registry" — that guard no longer
exists (08-01 already converted it in this same phase). The test itself is
strictly subsumed by
`TestSweepLeavesRejectMissingScopeIdentically/summarize-missing`, which
asserts the same thing plus the message; this one asserts only the exit
code.

**Fix:** Delete the test, or rewrite the comment and strengthen the
assertion to include the Sentence — see prior 08-REVIEW.md WR-10.

## Info

### IN-08: `migrate-remap-owner`'s "alias" wording remains imprecise (downgraded from WR-08)

**File:** `CLAUDE.md:15`

**Issue:** `migrate-remap-owner (alias: migrate-set-owner, deprecated)`
uses "alias" for a command with an incompatible flag set
(`migrate-remap-owner`: `apply, from, from-anon, from-missing, output,
timeout, to`; `migrate-set-owner`: `output, owner, timeout` — no `--apply`,
no `--from*`, no `--to`, per `testdata/catalog.golden`). The
deprecated-supersession relationship itself IS correct and pinned
(`TestMigrateSetOwnerEquivalentToRemapOwnerMissing` in
`internal/store/store_test.go`); 08-VERIFICATION.md classifies this
wording as info-level-only, and this review's own brief flags it as a
documented false positive when read as a functional defect. Retained here
only as a wording-precision note, since an agent that reads "alias" and
constructs `engram migrate-set-owner --from-missing --to <owner>` will hit
an unknown-flag error.

**Fix:** `` `migrate-remap-owner` (supersedes the deprecated
`migrate-set-owner`, a separate command with its own `--owner` flag, not a
flag-compatible alias); ``

### IN-01 through IN-07 (carried forward, unchanged)

**Files:** `cmd/engram/sweep_scope_test.go`, `internal/surfaces/rules_test.go`, `CLAUDE.md`, `docs-site/src/content/docs/reference/tools.md`, `docs-site/src/content/docs/guides/migrate.md`

**Issue (summary):** See prior 08-REVIEW.md for full detail on each:
`spine-review consolidate`'s false-clean report is now cemented as an
enforced test invariant with no pointer to the underlying footgun (IN-01);
the non-enforcement assertion in `sweep_scope_test.go` only inspects the
`--all-scopes` flag's Usage, not every flag on the command (IN-02); no
per-rule pin test exists for `RuleSweepScopeOrAllScopesRequired` unlike its
closest siblings (IN-03); `CLAUDE.md`'s `` `migrate` (`status`, `revert`) ``
phrasing parallels the parent-only `spine-review` shape even though
`migrate` is itself a runnable verb (IN-04); `tools.md` demoted a bold
required-ness callout to mid-sentence prose (IN-05); `guides/migrate.md`
never states `backfill-short-ids` is deprecated (IN-06); `tools.md` lists
`list_scheduled` among tools that "hide" scheduled records, which is
backwards for the `scheduled` state specifically (IN-07). None of these
seven items changed in this gap-closure wave.

**Fix:** See prior 08-REVIEW.md IN-01 through IN-07 for concrete
suggestions on each.

---

_Reviewed: 2026-08-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
