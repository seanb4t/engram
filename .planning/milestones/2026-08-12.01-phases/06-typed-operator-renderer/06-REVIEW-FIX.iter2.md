---
phase: 06-typed-operator-renderer
fixed_at: 2026-08-17T19:45:00Z
review_path: .planning/phases/06-typed-operator-renderer/06-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 06: Code Review Fix Report

**Fixed at:** 2026-08-17T19:45:00Z
**Source review:** .planning/phases/06-typed-operator-renderer/06-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (3 warning, 2 info -- `fix_scope: all`)
- Fixed: 5
- Skipped: 0

**Verification environment:** all edits, builds, and test runs happened in an
isolated git worktree (`gsd-reviewfix/06-58556`, based on `feat/2026-08-12.01`
at `61f62f3d`), fast-forwarded into `feat/2026-08-12.01` and torn down at the
end of this run per the standard review-fix worktree protocol. `task` (lint +
Go tests across every package + the Python skill-hook suite) ran to
completion twice inside that worktree; the numbers below are reproducible
from `feat/2026-08-12.01` after this fast-forward.

## Fixed Issues

### WR-01: `spine-review scan` and `spine-review verify` silently ignored `--all-scopes` when `--scope` was also supplied

**Files modified:** `cmd/engram/spine_review_scan.go`, `cmd/engram/spine_review_verify.go`, `cmd/engram/spine_review_test.go`, `cmd/engram/spine_review_verify_test.go`, `docs-site/src/content/docs/guides/cli.md`
**Commit:** `093b4779`
**Applied fix:** Registered `cmd.MarkFlagsMutuallyExclusive("scope", "all-scopes")` in both leaves' `init()`, mirroring the sibling registrations already present in `spine_review_consolidate.go:231` and `spine_review_purge.go:418` (now `:437`). Added `TestSpineReviewScanScopeAndAllScopesRejected` and `TestSpineReviewVerifyScopeAndAllScopesRejected`, mirroring `TestSpineReviewConsolidateScopeAndAllScopesRejected` -- both prove the combination is rejected at `exitUsage` via cobra's own validation, before `RunE` (and therefore before the store is ever dialed) runs. Also updated `cli.md`'s `scan`/`verify` sections to state the mutual-exclusivity constraint explicitly, closing the doc gap the finding named alongside the code gap.

### WR-02: `sanitizeViewValue` (T-06-03's mitigation) is bypassed for nested-container values

**Files modified:** `cmd/engram/operator_view.go`, `cmd/engram/operator_output_test.go`
**Commit:** `36dd3542`
**Applied fix:** Took the cheap, non-behavioral branch the review explicitly offered: no change to `viewScalar`'s rendering behavior (the review itself confirms the path is unreachable by any current operator report struct). Added `TestOperatorViewFixturesHaveNoUnsanitizedNesting`, which walks the marshaled JSON of every fixture in `operatorViewFixtures()` (the real, live document set for every operator command) and fails loudly if any array element or row-level field is itself a JSON array/object -- the exact shape `viewScalar`'s kind switch does not sanitize. The test passes today (confirming the gap is genuinely unreachable) and was sanity-checked against a synthetic two-level-deep fixture (`[][]string`) to confirm it fails loudly as designed -- that synthetic probe was not committed, only used to validate the guard before commit. Also tightened `sanitizeViewValue`'s doc comment to state the narrower actual guarantee (only a JSON string value `viewScalar` recognizes by its own kind check) instead of the blanket "every value" framing WR-02 flagged, cross-referencing the new test.

### WR-03: `purgeRerunCommand` built an unquoted, unsafe-to-copy-paste re-run string

**Files modified:** `cmd/engram/spine_review_purge.go`, `cmd/engram/spine_review_purge_test.go`
**Commit:** `263b7bb3`
**Applied fix:** Added a `shellQuote` helper (POSIX single-quote escaping: wrap in `'...'`, replace an embedded quote with the close-escape-reopen idiom) and applied it to `--scope`, `--category`, and `--tags` interpolation in `purgeRerunCommand` (`--class` and `--older-than` are left unquoted, since both are drawn from closed, engram-controlled vocabularies that never contain shell metacharacters). Added `TestShellQuoteRoundTripsThroughARealShell` (a value with a space, a value with a single quote, an already-quote-heavy value, and the empty string, each verified by piping the quoted output through a real `sh -c` invocation) and `TestPurgeRerunCommandQuotesFreeFormValues` (re-parses a full rendered rerun command through `sh`'s own `set --` word-splitting and asserts the resulting argv matches the original `PurgeOptions` exactly). Confirmed no golden fixture (`cmd/engram/testdata/*.golden`) or other test pins the unquoted `Rerun` string, so no other fixture needed updating.
_Note: while writing the `shellQuote` doc comment, discovered Go's current `gofmt` rewrites adjacent `''` in comments into a Unicode right-double-quote (a "smart quotes" comment-formatting behavior in this toolchain). Reworded the comment to avoid two adjacent quote characters rather than fight the formatter; flagging here since it is a generically surprising gofmt behavior worth knowing about for any future comment discussing shell quoting._

### IN-01: `spineArchiveOrRestore`/`renderArchiveResults` discard already-collected per-id results on a mid-batch abort

**Files modified:** `cmd/engram/spine_review_archive.go`
**Commit:** `815d2612`
**Applied fix:** Took the conservative, docs-only branch per instructions -- no behavior change. Extended `spineArchiveOrRestore`'s doc comment to state explicitly that the function itself preserves partial results on abort, but `renderArchiveResults` (one layer up) discards them when `len(results) < len(ids)`, and to name why: a deliberate asymmetry with every other partial-progress path this phase added (migrate revert's CR-06, purge's spared/appeared accounting), because a partial archive/restore batch was judged to have no natural "in-progress" framing the way a manifest-driven purge/migrate does. Also cross-referenced this explanation from the discard site itself in `renderArchiveResults`, and named the concrete render-then-return-error change (mirroring `revertApplyRun`) that would apply if this judgment is later reversed.

### IN-02: `spine-review purge`'s `init()` swallowed a missing-rule lookup that the same file treats as a fail-fast invariant elsewhere

**Files modified:** `cmd/engram/spine_review_purge.go`
**Commit:** `73ecfebc`
**Applied fix:** Per instructions, fixed the inconsistency within `spine_review_purge.go` only (did not sweep the pre-existing `ok`-swallowing pattern across `client_list.go`/`client_search.go`, which are out of this review's scope). The `init()`-time `surfaces.RuleByID(surfaces.RulePurgeFilterRequiresScope)` lookup used to compose three `--help` strings now panics on a missing rule, with the same message shape as `requirePurgeFilterScope`'s existing panic three lines above it in the same file -- both call sites now fail the binary at startup for the identical class of registry gap, rather than one panicking and the other silently degrading three `--help` strings to an empty trailing sentence.

## Skipped Issues

None -- all 5 in-scope findings were fixed.

## Notes on the fix session itself

- **Transient background-test interference (self-corrected, no functional impact):** while running the mandatory `task` gate as a background process, an earlier interactive `git status`/`git checkout -- internal/migrate/additive.go` (issued to investigate what looked like an unexpected uncommitted diff, unrelated to any phase-06 file) was in fact interference with `internal/store`'s `TestRedEvidencePatchesAreLive` harness, which legitimately patches `internal/migrate/additive.go` in place mid-test and reverts it afterward. The `git checkout` interrupted that patch/revert cycle and caused several `internal/migrate`/`internal/store` tests to fail on that one run. This was **not** caused by, and did not touch, any file this review-fix session modified (`cmd/engram/*`, `docs-site/src/content/docs/guides/cli.md`). A clean, uninterrupted `task` run afterward passed with no failures (`internal/store ok 106.652s`, including the red-evidence suite), confirming the earlier failures were self-inflicted noise from investigating the transient diff, not a real regression. No source file was left in a modified state by this incident -- `git status` was clean before every commit in this session.

---

_Fixed: 2026-08-17T19:45:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
