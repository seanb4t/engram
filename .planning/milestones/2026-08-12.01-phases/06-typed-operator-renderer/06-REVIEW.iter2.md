---
phase: 06-typed-operator-renderer
reviewed: 2026-08-17T00:00:00Z
depth: deep
files_reviewed: 28
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
  warning: 3
  info: 2
  total: 5
status: issues_found
---

# Phase 06: Code Review Report

**Reviewed:** 2026-08-17T00:00:00Z
**Depth:** deep
**Files Reviewed:** 28
**Status:** issues_found

## Summary

This phase converts every operator report to the "one serialization plus a rendered view"
architecture: `renderOperator`/`viewFields`/`renderOperatorView` (operator_output.go,
operator_view.go) walk the exact bytes `json.Marshal(doc)` produced, so `--output text` and
`--output json` derive from a single source by construction. That core mechanism holds up under
a deep read: `assertViewIdentity` genuinely cross-checks two independently-authored key walks
(`jsonTopLevelKeys` vs `viewFields`), the retired parity gate's non-vacuity is honestly recorded
rather than silently dropped, `TestOperatorViewFixturesCoverEveryOperatorCommand` derives its
expectation from the live `operatorCommands()` tree (not a hand-typed list) and is proven
falsifiable by `TestSetDiffDetectsDivergence`, and `TestOperatorDocsAreHandDeclared` closes off
the field-promotion escape hatch. Error handling in the migrate/revert/purge/archive families is
unusually careful about partial-progress reporting (CR-06, WR-03, WR-05, H5/H6 are all covered by
dedicated behavioral tests) and I could not find a regression in any of that previously-reviewed
territory.

Per the task brief: the known, already-filed gap where the operator text view's HEADLINE line
bypasses `sanitizeViewValue` (issue #505, threat T-06-03 only covers FIELD values) is still
present and still accurate as described — every headline producer in this diff (`archiveSummary`,
`consolidateSummary`, `verifySummary`, `spineScanSummary`, `migrateSummary`, `migrateRemapSummary`,
`migrateSetOwnerSummary`, `reindexSummary`, `summarizeSummary`, `statusSummary`, `revertSummary`,
`purgePreviewSummary`/`purgeAppliedSummary`) interpolates only CLI flag values, counts, or
version/plan structure — never stored record content — so the gap remains latent, not live. No
change in this diff makes #505 worse or better.

New findings below are either a genuine cross-file inconsistency (three of five
`--scope`/`--all-scopes` operator leaves enforce mutual exclusivity, two silently don't) or a
latent robustness gap in the shared rendering primitive that today's document set happens not to
trigger. Nothing rises to Critical: no crash, no data loss, no security bypass, no injection into
a currently-reachable code path.

## Warnings

### WR-01: `spine-review scan` and `spine-review verify` silently ignore `--all-scopes` when `--scope` is also supplied

**File:** `cmd/engram/spine_review_scan.go:43-49,61,156-163`, `cmd/engram/spine_review_verify.go:621-631,643,663-675`

**Issue:** `spine-review consolidate` and `spine-review purge` both register
`cmd.MarkFlagsMutuallyExclusive("scope", "all-scopes")` (spine_review_consolidate.go:231,
spine_review_purge.go:418), so supplying both flags together is rejected by cobra before RunE
ever runs. `spine-review scan` and `spine-review verify` — siblings that use the identical
"`--scope` or `--all-scopes` is required" bare `usageErrorf` guard (verify.go's own comment says
it "Mirrors spine-review scan's exact wording") — never register that mutual-exclusivity
constraint. Their RunE guard only rejects the "neither supplied" case:

```go
if spineScanScope == "" && !spineScanAllScopes {
    return usageErrorf("--scope <scope> or --all-scopes is required")
}
```

If an operator supplies `--scope s --all-scopes` together (e.g. a copy-paste error, or a script
that appends `--all-scopes` unconditionally), this check passes silently. Both leaves then build
their store options from `Scope` alone —
`store.SpineScanOptions{Scope: spineScanScope}` (scan.go:61) and
`store.SpineScanOptions{Scope: spineVerifyScope}` (verify.go:643) — with no `AllScopes` field in
the call at all. `--all-scopes` is therefore silently discarded: the operator gets a report
scoped to `s` while believing they asked for (and cobra accepted) a spine-wide sweep, and neither
the rendered headline (`spineScanSummary`/`verifySummary`) nor the JSON document ever discloses
that `--all-scopes` was requested-but-ignored — `spineScanDoc`'s `Scope` field just reads `"s"`
exactly as it would for a normal single-scope invocation. `cli.md` documents consolidate's
`--scope`/`--all-scopes` as explicitly mutually exclusive (line 226-229 of the guide) but never
makes that claim for scan/verify — the inconsistency is real in both code and docs, not a doc
typo. No test in `spine_review_test.go` or `spine_review_verify_test.go` exercises the
both-flags-supplied case for either command (only the "neither" case is pinned), so this gap is
unguarded.

**Fix:** Register the same guard scan/verify's siblings already use:

```go
spineReviewScanCmd.MarkFlagsMutuallyExclusive("scope", "all-scopes")
spineReviewVerifyCmd.MarkFlagsMutuallyExclusive("scope", "all-scopes")
```

placed in each command's `init()` alongside the existing flag registrations, plus a regression
test mirroring `TestSpineReviewConsolidateScopeAndAllScopesRejected`.

### WR-02: `sanitizeViewValue` (T-06-03's mitigation) is bypassed for nested-container values

**File:** `cmd/engram/operator_view.go:90-93` (array-element handling in `viewFields`),
`:133-152` (`viewScalar`), `:181-185` (`viewRow`)

**Issue:** `sanitizeViewValue` is documented as the mitigation for a stored value forging report
structure or emitting a terminal escape sequence (threat T-06-03), and the test suite
(`TestOperatorViewSanitizesControlCharacters`) proves it for a top-level scalar string field. The
guarantee is narrower than the doc comments claim, though: `viewScalar` only recognizes two
shapes explicitly — a JSON string (`raw[0] == '"'`, sanitized) and JSON `null` (rendered empty).
Every other byte sequence falls through to `return string(raw)` (operator_view.go:151),
**verbatim, unsanitized**. Two call sites feed values into `viewScalar` whose kind is never
checked first:

1. `viewFields`'s array branch (lines 83-93): for each array element, only `'{' ` (object) is
   special-cased via `viewRow`; anything else — including a *nested array* element (`elem[0] ==
   '['`) — falls into `viewScalar(elem)`, which returns the raw, un-sanitized JSON array text
   (e.g. `["a\x1b[31mb"]`) rather than a rendered/sanitized row.
2. `viewRow` (lines 181-185): every field inside a rendered row is passed to `viewScalar`
   unconditionally, with no kind check at all — a row-level field that is itself an object or
   array (two levels deep from the doc root) renders as raw, unsanitized JSON text glued into the
   `key=value` line instead of a proper nested rendering.

Today no operator report struct in this package has an array-of-arrays field or a row-level
object/array field (`rg '\[\]\[\]|map\[string\]' cmd/engram/*.go` over the non-test files finds
none), so this is currently unreachable — it does not undermine #505 or any live report. But it
is a real gap in a function whose own doc comment (`sanitizeViewValue`'s, operator_view.go:206-212)
claims a blanket guarantee ("a stored ... value can never forge an extra report line or emit a
terminal escape sequence"), and `TestOperatorDocsAreHandDeclared`/`TestOperatorViewFixturesCoverEveryOperatorCommand`
would not catch a future report author adding such a field — those gates check struct provenance
and per-command fixture coverage, not per-field shape safety.

**Fix:** Make `viewScalar` (or its caller) exhaustive over `valueKind`, and route array/object
values through the same array/row logic `viewFields` already has, e.g.:

```go
func viewScalar(raw json.RawMessage) string {
    switch valueKind(raw) {
    case '[', '{':
        // recurse into viewFields/viewRow-shaped rendering, or explicitly
        // render+sanitize a JSON-safe placeholder rather than raw bytes.
    }
    ...
}
```

or, more cheaply, add a fixture/test asserting `viewScalar`/`viewRow` never emit an un-sanitized
control character for a nested-container value, so a future two-level-deep report field fails the
identity gate rather than silently reintroducing the T-06-03 gap.

### WR-03: `purgeRerunCommand` builds an unquoted, unsafe-to-copy-paste re-run string

**File:** `cmd/engram/spine_review_purge.go:162-183`

**Issue:** `purgeRerunCommand` is explicitly designed to be "genuinely copy-pasteable" (its own
doc comment) and is surfaced both in the JSON document (`purgeReportDoc.Rerun`) and rendered in
the text view. It interpolates `--scope %s`, `--category %s`, and `--tags %s` via plain
`fmt.Fprintf` with no shell quoting:

```go
if opts.AllScopes {
    b.WriteString(" --all-scopes")
} else if opts.Scope != "" {
    fmt.Fprintf(&b, " --scope %s", opts.Scope)
}
if opts.Category != "" {
    fmt.Fprintf(&b, " --category %s", opts.Category)
}
for _, tag := range opts.Tags {
    fmt.Fprintf(&b, " --tags %s", tag)
}
```

A scope, category, or tag value containing a space or shell metacharacter (all are free-form
strings an operator can set on the write path) produces a rendered command that is not actually
the command that was run if copy-pasted into a shell — it silently splits into extra/different
arguments. This is not an injection into *this* process (the string is only rendered, never
executed by engram itself), but it does defeat the documented purpose of the field for exactly
the inputs most likely to need it.

**Fix:** Shell-quote each interpolated value (e.g. via a small `shellQuote` helper applying
single-quote escaping), or document the field's plain-value limitation explicitly if quoting is
out of scope for this phase.

## Info

### IN-01: `renderArchiveResults` discards already-collected per-id results on a mid-batch abort

**File:** `cmd/engram/spine_review_archive.go:97-127` (`spineArchiveOrRestore`),
`:134-147` (`renderArchiveResults`)

**Issue:** `spineArchiveOrRestore` returns `results` "exactly as far as it got" when a non-
not-found error aborts the loop partway through a multi-`--id` batch (e.g. ids 1-2 succeed, id 3
hits a transport error). `renderArchiveResults` then takes:

```go
if procErr != nil && len(results) < len(ids) {
    // Aborted partway through ... no complete report to render.
    return classifyOperatorErr(procErr)
}
```

which means the already-successful outcomes for ids 1-2 are never rendered to the operator at
all — not in text, not in JSON — even though they were genuinely processed and
`spineArchiveOrRestore` has them in hand. This appears to be a deliberate simplification (the
comment frames it as "no complete report to render," implying completeness is the bar for
rendering anything), but it does mean real, already-computed information is silently thrown away
on abort, which could matter for an operator triaging a partially-failed multi-id `archive`/
`restore` call. Worth confirming this is the intended operator experience rather than an
oversight, since every other partial-progress path added in this phase (migrate revert's CR-06,
purge's spared/appeared accounting) goes out of its way to preserve and report partial progress
rather than discard it.

**Fix (if intended, docs-only):** note the asymmetry explicitly in `spineArchiveOrRestore`'s doc
comment (it currently states the behavior but not why it differs from every other partial-progress
path in the same phase). **Fix (if unintended):** render the partial `results` slice (with
`Applied`-equivalent semantics honestly reflecting "processing was interrupted") before returning
the classified error, mirroring `revertApplyRun`'s render-then-return-error pattern.

### IN-02: `spine-review purge`'s `init()` swallows a missing-rule lookup that the same file treats as a fail-fast invariant elsewhere

**File:** `cmd/engram/spine_review_purge.go:88-92` (`requirePurgeFilterScope`) vs `:407`
(`init()`)

**Issue:** `requirePurgeFilterScope` in this same file panics loudly if
`surfaces.RulePurgeFilterRequiresScope` is missing from the registry:

```go
rule, ok := surfaces.RuleByID(surfaces.RulePurgeFilterRequiresScope)
if !ok {
    panic("spine-review purge: surfaces.RulePurgeFilterRequiresScope is not registered in internal/surfaces/rules.go")
}
```

but `init()`, composing the same rule's `Sentence` into three flag `--help` strings, discards the
`ok` result entirely:

```go
scopeRule, _ := surfaces.RuleByID(surfaces.RulePurgeFilterRequiresScope)
spineReviewPurgeCmd.Flags().StringVar(&spinePurgeCategory, "category", "",
    "free-form filter: only this category; "+scopeRule.Sentence)
```

If the rule were ever unregistered, `--help` for `--category`/`--tags`/`--older-than` would
silently degrade to a usage string ending in an empty sentence, rather than failing the binary at
startup the way `requirePurgeFilterScope` (and `spine_review_verify.go`'s equivalent `panic`
guards, lines 561-566 and 669-672) do for the identical class of registry gap. This exact
"swallow `ok`, degrade help text" pattern is also used pre-existing elsewhere in this package
(`client_list.go:102,112`, `client_search.go:90`), so it is an established, if inconsistent,
convention rather than a phase-06 novelty — flagging for awareness, not as a regression.

**Fix:** Either accept the existing convention (informational only), or make the `init()`-time
lookup panic like its sibling call three lines away, for consistency within the same file.

---

_Reviewed: 2026-08-17T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
