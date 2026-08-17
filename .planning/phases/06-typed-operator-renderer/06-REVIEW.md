---
status: issues-found
phase: 06-typed-operator-renderer
depth: standard
reviewed_files: 26
critical: 0
warning: 1
info: 2
---

# Phase 06: Typed Operator Renderer — Code Review Report

**Reviewed:** 2026-08-17T15:28:16Z
**Depth:** standard
**Files Reviewed:** 26 (excluding the two golden fixtures and the docs-site page, which were spot-diffed rather than fully re-derived)
**Status:** issues-found (no BLOCKERs; one WARNING, two INFO)

## Summary

This phase replaces the operator CLI's two-argument `renderOperator(cmd, format, text, doc)` with a
single-serialization model: `--output json` marshals a hand-declared report struct, and `--output
text` is a rendered view produced by walking the exact bytes that marshal produced. I verified the
phase's central, historically-fragile claims directly rather than trusting the SUMMARY.md files:

- **One serialization, verified by grep.** `rg -o 'json\.Marshal\(' cmd/engram/ --glob '!*_test.go'`
  returns exactly one hit: `cmd/engram/operator_view.go:46`. Every other JSON-mode call site in the
  tier (`operator_output.go`, `catalog.go`) uses `json.NewEncoder(...).Encode(doc)` against the same
  `doc any` value the text branch renders — there is no second path from a report value to text.
- **Marshaled bytes, not the struct.** `viewFields` calls `json.Marshal(doc)` once, then walks the
  result with `(*json.Decoder).Token`/`Decode`, so `omitempty`, `json:"-"`, embedded-field promotion,
  and numeric precision (no `float64` round-trip — `viewScalar`'s default case returns the raw JSON
  text verbatim) all inherit `encoding/json`'s own behavior rather than a reimplementation of it. I
  confirmed this is exercised, not merely asserted, via `TestOperatorViewOrdering`'s four subtests and
  `TestOperatorViewDuplicateKeyAdjacency`.
- **The identity gate is not vacuous.** `TestOrderedKeyDiffDetectsDivergence`,
  `TestCountTopLevelFieldLines`, and `TestSetDiffDetectsDivergence` are committed negative-case proofs
  that each of the gate's three moving parts can independently return non-empty/failing results, and
  the recorded mutation probe in `06-01-SUMMARY.md` shows `TestHumanizeKey` failing while
  `TestOperatorViewIdentity` stays green under a mutated `humanizeKey` — the exact decomposition D-06
  requires (label-text bugs are invisible to the identity gate by design; a dedicated table-driven test
  is the only thing that can catch them).
- **Both-directions coverage, derived not transcribed.** `TestOperatorViewFixturesCoverEveryOperatorCommand`
  computes its expectation from `commandKeySet(operatorCommands())` — the live cobra tree — and diffs it
  against the merged `operatorViewFixtures()` map in both directions, with a per-entry non-empty-slice
  check so a key registered with zero documents cannot satisfy the gate vacuously. I confirmed
  `backfill-short-ids`' preview variant (the gap the phase's own history names) has its own fixture
  entry in `operator_view_migrate_test.go`.
- **Additive-only JSON changes, verified against the actual struct diffs**, not just the doc comments:
  `spineScanReportDoc` gained `Scope` (no existing tag/order changed — `git diff` confirms only an
  insertion), `purgeReportDoc` gained `Rerun` with `omitempty` (present only on the preview doc, absent
  on applied — asserted directly in `spine_review_purge_test.go:239-254`), and the new
  `migrateStatusReportDoc` reproduces `store.MigrateStatusResult`'s five keys in their original order
  before appending `current_version` last (asserted by
  `TestMigrateFamilyStatusReportDocKeyOrder`, which uses `orderedKeyDiff` — not set membership).
- **Sanitization is applied consistently.** `sanitizeViewValue` maps every rune `< 0x20` and `0x7f` to
  a space and is the only path a decoded JSON *string* scalar takes (`viewScalar`); I traced every
  caller of `viewScalar` (top-level scalars, array elements, and `viewRow`'s nested `key=value`
  segments) and found no bypass. Raw (non-string) scalars — numbers, bools, `null` — are printed
  verbatim, which is safe because `encoding/json` itself always escapes control characters inside any
  nested string, at any depth, so a value can never reach the renderer as a literal control byte
  outside the one path that is sanitized.
- **Build, tests, lint, and license all pass** at HEAD (`go build ./...`, `go test ./cmd/engram/...
  -count=1`, `task lint`, `task license:check`) — I ran all four directly rather than trusting the
  plan's own acceptance criteria.
- **Retirement is clean.** `rg -n 'operatorParityRows|TestOperatorOutputParity'` returns zero hits
  anywhere in `cmd/engram`; `backfill.go` (referenced but out of this phase's file list) is untouched,
  confirming the migrate-family conversion correctly reused `migrateSweepPreviewRun`/`migrateSweepApplyRun`
  rather than duplicating a report shape for the alias.

I found no BLOCKER-level defects. The one WARNING below is a genuine, verified gap in test coverage
for a code path that exists in shipped `operator_view.go` but is reached by none of the 15 real
operator reports and by no test in the diff.

## Warnings

### WR-01: `viewFields`'s bare-nested-object branch is unreached by any real report and untested

**File:** `cmd/engram/operator_view.go:95-100`
**Issue:** `viewFields` has three cases for a top-level field's JSON value: array (`case '['`), a bare
object (`case '{'`, which calls `viewRow` once and sets `field.Rows = []string{row}`), and scalar
(default). I checked every one of the 15 doc structs the phase converts or is about to hand off to
Phase 7 (`grep -n 'type .*Doc struct'` across `cmd/engram/*.go`, then read each: `migrateOutputDoc`,
`migrateStatusReportDoc`, `revertOutputDoc`, `migrateSetOwnerReportDoc`, `migrateRemapReportDoc`,
`spineScanReportDoc`, `pruneOutputDoc`, `purgeReportDoc`, `reindexOutputDoc`, `consolidateReportDoc`,
`archiveReportDoc`, `verifyReportDoc`, `summarizeOutputDoc`) — every container-valued field in every
one of them is a *slice* of a row struct (`case '['`), never a bare nested struct. `TestOperatorViewOrdering`'s
"embedded struct promotes fields at the embedding position" subtest is the only test with a nested
struct, and Go's anonymous-embedding promotion means that field's subkeys are flattened into the
*parent's* top-level object by `encoding/json` — it never produces a nested `case '{'` value either.
I confirmed with `rg -n 'json:"'` across every `operator_view*_test.go` file that no fixture struct
anywhere declares a plain (non-slice, non-embedded) nested-struct field.

This means the `case '{'` branch (5 lines, a real decode-and-render path) is dead in production and has
zero direct test coverage in this diff. Reading it, it very likely works correctly — it reuses the
otherwise well-exercised `viewRow` — but this repo has a specifically documented history of
untested/unreachable branches hiding defects (the phase's own `<specifics>` cites the
`backfill-short-ids` preview variant missed by a hand-written list), and "the code looks right" is
exactly the kind of unverified claim this review is instructed not to accept at face value. Nothing in
06-CONTEXT.md or 06-01-PLAN.md's `<specifics>`/D-07 actually calls for this shape (D-07 describes only
the array-of-rows shape for the four two-level reports), so this branch is speculative generality
beyond what any of the 15 conversions needed.

**Fix:** Either (a) add a small synthetic test mirroring `TestOperatorViewOrdering`'s throwaway-struct
style — a doc with one plain nested-struct field, asserting it renders as a single four-space row
under its own label and that `countTopLevelFieldLines` still counts it as exactly one top-level field
— or (b) if no report is ever expected to need this shape, delete the `case '{'` branch and let a
bare-object top-level value fall through to the scalar default (which would render the object's raw
JSON text, an explicit and honestly-labeled degradation) until a real report needs it, per the
project's "never speculative" bias. Given the mechanism is meant to generalize across future Phase 7
additions, option (a) is the safer choice.

## Info

### IN-01: Headline text bypasses `sanitizeViewValue` by construction, with no structural guard against future misuse

**File:** `cmd/engram/operator_view.go:246` (`renderOperatorView`'s `fmt.Fprintln(w, headline)`)
**Issue:** Every field *value* rendered from `doc` passes through `sanitizeViewValue` (via
`viewScalar`), but `headline` is written directly, unsanitized. I checked every headline producer in
the diff (`reindexSummary`, `pruneSummary`/`prunePreviewSummary`, `archiveSummary`, `purgeAppliedSummary`/
`purgePreviewSummary`, `consolidateSummary`, `verifySummary`, `spineScanSummary`, `statusSummary`,
`revertSummary`, `migrateSummary`, `migrateSetOwnerSummary`, `summarizeSummary`) and confirmed every
one interpolates only counts and CLI-flag values the invoking operator supplied on their own command
line (`--scope`, `--class`, `--target`, etc.), or static reason strings from the `migrate`/`store`
packages — never a stored record's `id`, `tag`, or free-form `reason` field pulled from Qdrant. So this
is currently safe, not a live vulnerability.
**Fix:** No code change needed today. Worth a one-line doc comment on `renderOperatorView` (or on the
`headline string` parameter of `renderOperator`) stating that the headline is NOT sanitized and must
never be built from stored/untrusted record content — only from CLI-flag values and counts the
operator already controls — so a future headline producer doesn't reintroduce the injection class
`sanitizeViewValue` exists to close (T-06-03).

### IN-02: `countTopLevelFieldLines`'s leading-two-spaces heuristic is unguarded against a headline that itself starts with two spaces

**File:** `cmd/engram/operator_view_test.go:98-113`
**Issue:** This test-only helper (not shipped in the binary) classifies a line as a top-level field
line purely by its first three characters (`"  " + non-space`). I confirmed no current headline
producer emits a string starting with two literal spaces (all begin with fixed prose like `"spine
..."`, `"stamped owner=..."`, `"dry-run: ..."`), including the one headline built from
`store.RevertRefusalError(plan).Error()`, which I traced to `internal/store/revert.go:169-183` and
confirmed always begins with `"revert cannot reach v..."`. So this is not currently exploitable, and
it lives in test code, not the shipped binary — low impact.
**Fix:** No action required now. If a future headline producer's format string could begin with
whitespace, this counting helper would silently misclassify it as a field line (or vice versa),
weakening the identity gate's own line-count assertion without any test failing to say so. A one-line
guard or comment noting the assumption in `countTopLevelFieldLines`'s doc comment would make the
dependency explicit for the next person who adds a headline producer.

---

_Reviewed: 2026-08-17T15:28:16Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
