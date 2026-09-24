---
phase: 03-curation-verdicts
reviewed: 2026-09-24T00:00:00Z
depth: standard
files_reviewed: 45
files_reviewed_list:
  - .gitignore
  - CLAUDE.md
  - Taskfile.yaml
  - cmd/engram/consolidate_docs_test.go
  - cmd/engram/operator_output_test.go
  - cmd/engram/operator_view.go
  - cmd/engram/operator_view_scan_test.go
  - cmd/engram/spine_review_consolidate.go
  - cmd/engram/spine_review_consolidate_test.go
  - cmd/engram/spine_review_consolidate_view.go
  - cmd/engram/sweep_scope.go
  - cmd/engram/sweep_scope_test.go
  - cmd/engram/testdata/catalog.golden
  - cmd/engram/testdata/help.golden
  - docs-site/src/content/docs/guides/cli.md
  - docs-site/src/content/docs/guides/configure.md
  - docs-site/src/content/docs/guides/upgrade.md
  - docs-site/src/content/docs/reference/tools.md
  - internal/config/config.go
  - internal/config/decisions_config_test.go
  - internal/config/decisions_docs_test.go
  - internal/config/registry.go
  - internal/config/validate.go
  - internal/curationeval/doc.go
  - internal/curationeval/eval_test.go
  - internal/curationeval/evaluate.go
  - internal/curationeval/gate.go
  - internal/curationeval/gate_test.go
  - internal/curationeval/localfile.go
  - internal/curationeval/localfile_test.go
  - internal/curationeval/metrics.go
  - internal/curationeval/metrics_test.go
  - internal/curationeval/pairs.go
  - internal/curationeval/pairs_test.go
  - internal/server/decider.go
  - internal/server/decider_test.go
  - internal/skills/data/curating-spine/SKILL.md
  - internal/store/boundedread.go
  - internal/store/verdictstate.go
  - internal/store/verdictstate_test.go
  - internal/surfaces/rules.go
  - internal/surfaces/toolclass.go
  - internal/verdict/verdict.go
  - internal/verdict/verdict_test.go
  - skill/engram/skills/curating-spine/SKILL.md
findings:
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-09-24T00:00:00Z
**Depth:** standard
**Files Reviewed:** 45
**Status:** issues_found

## Summary

This phase adds advisory relation-verdicts to `spine-review consolidate` (`internal/verdict`, the
`RecordStates`/bounded-read wiring in `internal/store`, `internal/server/decider.go`,
`cmd/engram/spine_review_consolidate*.go`) plus a gated, aggregate-only accuracy harness
(`internal/curationeval`), and closes the #508 gap that let `consolidate` run without `--scope`/
`--all-scopes`.

The implementation is unusually well-defended for the properties this review was asked to verify:

- **No mutation path.** `spineConsolidateStore` exposes only `NearDuplicates`/`RecordStates`
  (both read-only), proven structurally by `TestConsolidateStoreSurfaceIsReadOnly` via
  `reflect.TypeOf`. `Store.RecordStates` issues no write RPC on any path
  (`TestRecordStatesDoesNotMutate` snapshots point count + payload digest before/after).
- **Bounded reads.** `verdictStateView`/`verdictStateRecordCeiling` derive a provable per-record
  byte ceiling from `RecordCaps`, pinned by `TestVerdictStateViewCeiling` (82432 bytes at
  defaults, `perRPCLimit` 25).
- **Disclosure ordering.** The stderr disclosure line is printed before `runVerdictPass` is
  called, and no memory content ever appears in stdout/stderr (only counts, classes, provider
  metadata) — `verdictHeadlineClause`/`verdictSummaryLine`/`verdictDisclosureLine` are all pure
  functions over counts.
- **Per-pair failure isolation.** `runVerdictPass` degrades a pair to `state_unavailable`/
  `timeout`/etc. without failing the command; `TestSpineReviewConsolidateFailurePolicy`,
  `TestSpineReviewConsolidateStateFetchErrorDegrades`, and
  `TestSpineReviewConsolidateDeadlineDuringVerdictPass` all assert exit 0.
- **Nested-value sanitization.** `sanitizeViewValue` is applied to every string the text lane
  renders regardless of nesting depth; `renderVerdictView` (the row-field renderer for
  `verdict`) returns raw text but `viewRow` sanitizes the whole returned string before it
  reaches output, and `TestOperatorViewFixturesHaveNoUnsanitizedNesting` proves this over every
  fixture (including two consolidate fixtures with verdict objects) by substituting a hostile
  control-character payload into every string leaf.
- **Threshold/probability parsing.** `config.ParseProbability` rejects NaN/Inf/out-of-range and
  is the single parser shared by `Config.Validate`, `internal/server.verdictSettings`, and
  `--verdict-threshold`, so the validated and enforced ranges cannot diverge; boundary behavior
  (`p == threshold` never flags, one ULP below always does) is pinned in
  `TestFromResultThresholdBoundary`.
- **#508 scope-guard ordering.** `requireSweepScope` is the first statement in `RunE`, proven by
  `TestSpineReviewConsolidateScopeGuardPrecedesEverything`, which substitutes
  `spineConsolidateStoreFromEnv` with a function that fails the test if called — the guard fires
  even before `--output` validation, so no Qdrant dial is possible before the scope check.
- **curationeval gating / no verbatim content.** `resolveEvalGate` never touches
  `internal/config`/production validation, so an unrelated malformed `ENGRAM_*` var can't
  break it; `formatReport` receives `[]prediction` (gold label + `verdict.Verdict`) only, never
  pair text, and every line is aggregate counts/rates prefixed `CURATION-EVAL | `.

Two issues are worth fixing, neither blocking: an operator-facing disclosure inaccuracy about
what "summary" vs "content" truncation actually means (the two are truncated together, not
independently, and this can silently swallow the summary), and a minor duplication of the
relation-name string literals outside `internal/verdict`.

## Warnings

### WR-01: Disclosure text overstates what gets sent — "summary" is not actually sent in full

**File:** `cmd/engram/spine_review_consolidate.go:136-137, 468`
**File:** `docs-site/src/content/docs/guides/cli.md:290-292`
**File:** `docs-site/src/content/docs/guides/configure.md:186-193`

**Issue:** Every operator-facing surface that discloses what a verdict request sends says, in
effect, "both records' summary [in full] plus up to `ENGRAM_DECISIONS_VERDICT_STATE_CHARS`
characters of content":

```go
// cmd/engram/spine_review_consolidate.go:468
"each request carrying both records' summary and up to %d characters of content; --no-verdicts skips this",
```

But `internal/verdict.State` (the function that actually builds what's sent) truncates the
**combined** `summary + "\n\n" + content` string to `maxChars` runes — it does not send the full
summary plus a separate N-character content budget:

```go
// internal/verdict/verdict.go:83-92
func State(summary, content string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = DefaultStateChars
	}
	full := content
	if summary != "" {
		full = summary + "\n\n" + content
	}
	return truncateRunes(full, maxChars)
}
```

Under the default caps (`ENGRAM_MEMORY_MAX_SUMMARY_BYTES=512`,
`ENGRAM_DECISIONS_VERDICT_STATE_CHARS=1500`) this rarely bites, but both caps are operator-
configurable, and `ENGRAM_MEMORY_MAX_SUMMARY_BYTES=0` is a documented, supported "disable the
summary bound" setting that falls back to `ENGRAM_MEMORY_MAX_CONTENT_BYTES` (64 KiB) as the
summary's effective ceiling (`internal/store/boundedread.go`'s `summaryTerm`). In that
configuration — or simply with a summary near or over `VerdictStateChars` — `State` can
silently truncate the summary itself and send *zero* characters of content, while every
disclosure surface an operator reads before opting in tells them the full summary is always
included. This directly undercuts the informed-consent purpose of the disclosure line (D-10):
an operator judging whether it's acceptable to send "both records' summary" to a third party is
being told something the code does not actually guarantee.

Note `docs-site/src/content/docs/guides/configure.md:214` (the `ENGRAM_DECISIONS_VERDICT_STATE_CHARS`
table row) already states this correctly — "How many characters of each record (its summary,
then the head of its content)" — so the file is internally inconsistent with its own prose
section immediately above it.

**Fix:** Reword every disclosure surface to match `internal/verdict.State`'s actual contract,
e.g.:

```go
"each request carrying, per record, up to %d characters total of its summary followed by its content; --no-verdicts skips this",
```

and the equivalent wording in the CLI `Long` help text, `docs-site/guides/cli.md`, and
`docs-site/guides/configure.md`'s "What leaves your deployment" prose (leave the already-correct
table row at `configure.md:214` as the reference wording).

**Status:** Fixed in 0d15d0b5

## Info

### IN-01: Relation names duplicated as string literals outside `internal/verdict`

**File:** `cmd/engram/spine_review_consolidate_view.go:29-42`

**Issue:** `probabilityFor` switches on the five D-05 relation names as bare string literals
(`"duplicate"`, `"contradicts"`, `"updates"`, `"related"`, `"unrelated"`) instead of the
exported constants `verdict.Duplicate`/`verdict.Contradicts`/`verdict.Updates`/`verdict.Related`/
`verdict.Unrelated` that `internal/verdict/verdict.go` declares specifically so the vocabulary
has one source of truth. This file's sibling, `cmd/engram/spine_review_consolidate.go`, already
imports `internal/verdict` and uses those constants throughout. A future rename or sixth relation
added only via the constants would silently desync this switch from the rest of the codebase
(the text lane would just render an empty `p=` for the new/renamed relation rather than failing
loudly).

**Fix:** Import `github.com/seanb4t/engram/internal/verdict` in
`spine_review_consolidate_view.go` and switch on `verdict.Duplicate` etc. instead of string
literals.

---

_Reviewed: 2026-09-24T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
