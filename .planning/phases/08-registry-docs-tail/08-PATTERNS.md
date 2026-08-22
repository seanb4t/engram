# Phase 8: Registry & Docs Tail - Pattern Map

**Mapped:** 2026-08-21
**Files analyzed:** 10 (3 modified call sites + 1 rule literal + 1 generator entry + 3 anchor targets + 2 new docs pages + CLAUDE.md + reference page extension)
**Analogs found:** 10 / 10

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/surfaces/rules.go` (new `RuleSweepScopeOrAllScopesRequired` const + literal) | config/registry | request-response (validation rule) | `RulePurgeFilterRequiresScope` (same file, lines 150-162 doc comment, 257-265 literal) | exact |
| `cmd/engram/summarize.go:39` | controller (cobra RunE guard) | request-response | `requirePurgeFilterScope`, `cmd/engram/spine_review_purge.go:79-91` | exact |
| `cmd/engram/spine_review_scan.go:44` | controller (cobra RunE guard) | request-response | same | exact |
| `cmd/engram/spine_review_verify.go:623` | controller (cobra RunE guard) | request-response | same | exact |
| `internal/surfacesgen/main.go` (`ruleTargets` entry) | config/generator-map | batch/transform | `RulePurgeFilterRequiresScope` entry, `internal/surfacesgen/main.go:127-135` | exact |
| Anchor pair in `docs-site/src/content/docs/reference/tools.md` | component (prose anchor) | transform (generated fill) | existing `purge-filter-requires-scope` anchor, `tools.md:128` | exact (filled analog; empty form inferred) |
| Anchor pair in `docs-site/src/content/docs/guides/cli.md` | component (prose anchor) | transform | existing `purge-filter-requires-scope` anchor, `cli.md:309` | exact (filled analog; empty form inferred) |
| Anchor pair in `skill/engram/skills/curating-memory/SKILL.md` | component (prose anchor) | transform | existing `purge-filter-requires-scope` anchor, `SKILL.md:388` | exact (filled analog; empty form inferred) |
| `docs-site/src/content/docs/guides/migrate.md` (new) | route/doc page | request-response (operator guide) | `docs-site/src/content/docs/guides/reindex.md` | exact |
| `docs-site/src/content/docs/reference/memory-record.md` (extend `## Field reference` + new sections) | component (reference doc) | transform | `### Supersession` / `### Archiving` sections, same file lines 37-84 | exact |
| Rejection test for `summarize.go`'s scope-or-all-scopes guard | test | request-response | `TestSpineReviewScanRequiresScopeOrAllScopes`, `cmd/engram/spine_review_test.go:47-59` | exact |
| `CLAUDE.md` (Not-used-here line, `cmd/engram/` row, Memory contract) | config/routing-doc | transform | itself, no external analog needed — self-consistent edit | n/a |

## Pattern Assignments

### `internal/surfaces/rules.go` (registry, request-response)

**Analog:** `RulePurgeFilterRequiresScope`, `internal/surfaces/rules.go:150-162` (doc comment) and `257-265` (literal)

**Doc comment + const pattern** (lines 150-162):
```go
// RulePurgeFilterRequiresScope is the ID of the rule requiring an explicit
// --scope (or --all-scopes) whenever `spine-review purge`'s free-form
// filter path (category/tags/older-than supplied with no structural class
// selected -- store.PurgeFilterPathActive) is engaged (D-10, 03-07-PLAN.md).
// A structural-class-only run is exempt: a class is a derivation the
// operator merely parameterizes, never a free-form judgment. Fields lists
// all four field names the filter path is built from, so ApplicableSurfaces
// resolves this rule onto exactly the surfaces that expose scope AND
// category AND tags AND older-than together -- measured against the live
// tree, that is `spine-review purge` alone on the cobra-usage lane, plus
// docs-site/reference/tools.md, docs-site/guides/cli.md, and
// curating-memory/SKILL.md on the prose lanes (03-07-PLAN.md Task 3).
const RulePurgeFilterRequiresScope = "purge-filter-requires-scope"
```

**Struct literal pattern** (lines 257-265):
```go
{
    ID:       RulePurgeFilterRequiresScope,
    Sentence: "the free-form filter path requires an explicit --scope or --all-scopes: category or tags always engage it, and older-than engages it when no class is selected",
    Fields:   []string{"scope", "category", "tags", "older-than"},
    Hint:     "conditional_required",
    // TagForm deliberately left empty: no MCP arg struct carries a
    // "class"/purge-shaped field set at all -- this rule is CLI-only,
    // same reasoning as RuleDestructiveRequiresApply/RuleVerifyFailOnValues.
    declared: true,
},
```

New literal: `ID: RuleSweepScopeOrAllScopesRequired`, `Fields: []string{"scope", "all-scopes"}`, `Hint: "conditional_required"`, `TagForm` left empty for the identical reason (no MCP arg struct exposes `all_scopes`). Append to the `rules` slice — do not insert.

---

### `cmd/engram/{summarize,spine_review_scan,spine_review_verify}.go` (controller, request-response)

**Analog:** `requirePurgeFilterScope`, `cmd/engram/spine_review_purge.go:79-91`

**Verbatim template:**
```go
// store.PurgeFilterPathActive) is engaged. A structural-class-only run
// (even one that supplies --older-than as that class's own window
// override) is allowed without either, since a class is a derivation
// rather than an operator judgment (D-10). Composes the registered
// surfaces.RulePurgeFilterRequiresScope rule's Sentence into the rejection
// -- never a bare, unregistered usage check (RESEARCH.md Pitfall 2).
func requirePurgeFilterScope(opts store.PurgeOptions) error {
	if opts.Scope != "" || opts.AllScopes {
		return nil
	}
	if !store.PurgeFilterPathActive(opts) {
		return nil
	}
	rule, ok := surfaces.RuleByID(surfaces.RulePurgeFilterRequiresScope)
	if !ok {
		panic("spine-review purge: surfaces.RulePurgeFilterRequiresScope is not registered in internal/surfaces/rules.go")
	}
	return usageErrorf("%s", rule.Sentence)
}
```

Apply at each of the three sites: replace the bare
`usageErrorf("--scope <scope> or --all-scopes is required")` with a
`surfaces.RuleByID(surfaces.RuleSweepScopeOrAllScopesRequired)` lookup + panic-on-missing guard +
`usageErrorf("%s", rule.Sentence)`. `spine_review_verify.go` already imports `internal/surfaces`
(confirmed at `spine_review_verify.go:22`, used for `RuleVerifyFailOnValues`); `summarize.go` and
`spine_review_scan.go` need the import added.

---

### `internal/surfacesgen/main.go` (`ruleTargets` entry) — config/generator-map, batch

**Analog:** `RulePurgeFilterRequiresScope` entry, `internal/surfacesgen/main.go:127-135`

```go
// RulePurgeFilterRequiresScope: no proto anchor -- "class"/"older-than"
// (purge's own flags) are not proto fields on any message. Anchored on
// all THREE prose surfaces the live tree measures as exposing
// scope+category+tags+older-than together (03-07-PLAN.md Task 3): the
// CLI guide's own purge subsection, the tools reference (where the
// filter vocabulary is published), and curating-memory's SKILL.md
// (where an agent learns the deletion contract).
surfaces.RulePurgeFilterRequiresScope: {
    {path: "docs-site/src/content/docs/guides/cli.md", kind: kindMarkdown},
    {path: "docs-site/src/content/docs/reference/tools.md", kind: kindMarkdown},
    {path: "skill/engram/skills/curating-memory/SKILL.md", kind: kindMarkdown},
},
```

New entry (per RESEARCH.md's `ApplicableSurfaces Resolution` derivation): identical 3-file shape
(`cli.md`, `tools.md`, `curating-memory/SKILL.md`), keyed on `surfaces.RuleSweepScopeOrAllScopesRequired`.
Add an explanatory comment (mirroring the pattern above) noting no proto anchor exists because
`all_scopes` is not a proto field anywhere, and noting the `SurfaceSkill` resolution is a substring
artifact per RESEARCH.md — place the anchor at a substantive extension point (near
`curating-memory/SKILL.md:382`'s existing `spine-review purge` mention), not `discovering/SKILL.md`.

Map key type note: existing entries key by the exported string const (`surfaces.RulePurgeFilterRequiresScope`
etc.), not by a separate ID literal — follow that exactly.

---

### Anchor pairs — `tools.md`, `cli.md`, `SKILL.md` (component, transform)

**No genuinely empty anchor pair exists anywhere in the tree today** (verified: every one of the 8
registered rules' anchors is already filled). The filled form of the closest analog
(`purge-filter-requires-scope`) is shown below in each host file; **the empty form the executor must
hand-write is inferred** by removing the body text between the two HTML comments, leaving the
comments themselves and their surrounding prose untouched:

**`skill/engram/skills/curating-memory/SKILL.md:388`** (filled):
```
<!-- engram:rule:start purge-filter-requires-scope -->the free-form filter path requires an explicit --scope or --all-scopes: category or tags always engage it, and older-than engages it when no class is selected<!-- engram:rule:end purge-filter-requires-scope -->.
```
Inferred empty form to hand-write for the new rule (note the trailing period sits **outside** the
closing anchor comment, per RESEARCH.md's "Generator Pipeline" step 5):
```
<!-- engram:rule:start sweep-scope-or-all-scopes-required --><!-- engram:rule:end sweep-scope-or-all-scopes-required -->.
```

**`docs-site/src/content/docs/reference/tools.md:128`** (filled):
```
<!-- engram:rule:start purge-filter-requires-scope -->the free-form filter path requires an explicit --scope or --all-scopes: category or tags always engage it, and older-than engages it when no class is selected<!-- engram:rule:end purge-filter-requires-scope -->.
```

**`docs-site/src/content/docs/guides/cli.md:309`** (filled):
```
<!-- engram:rule:start purge-filter-requires-scope -->the free-form filter path requires an explicit --scope or --all-scopes: category or tags always engage it, and older-than engages it when no class is selected<!-- engram:rule:end purge-filter-requires-scope -->.
```

Placement guidance from RESEARCH.md: in `cli.md` and `tools.md`, anchor it where the three sweep
commands' `--scope`/`--all-scopes` requirement is already substantively discussed (`cli.md:161-249`,
`tools.md`'s `summarize-missing` CLI usage block ~512-520). In `curating-memory/SKILL.md`, extend the
existing `spine-review purge --apply` mention (lines 380-388) to also name the sibling sweep
commands (`scan`/`verify`/`summarize-missing`), giving the anchor a genuine sentence home rather than
an orphaned one — **not** `discovering/SKILL.md`, which never mentions any sweep command.

`WriteRegion` (`internal/surfaces/anchor.go`) errors if the anchor pair does not pre-exist — write
the empty pair by hand FIRST, add the `ruleTargets` entry, THEN run `task surfaces:gen` (never bare
`go run ./internal/surfacesgen`) to fill the body.

---

### `docs-site/src/content/docs/guides/migrate.md` (new) — route/doc page

**Analog:** `docs-site/src/content/docs/guides/reindex.md`

**Frontmatter pattern** (lines 1-3):
```
---
title: Reindex (embedder migration)
description: Migrate memories to an embedder with a different vector dimension by re-embedding into a fresh Qdrant collection with engram reindex — including --source, --resume, and the cutover flow.
---
```
New page: `title: Migrate (schema-version migrations)` (or similar), `description:` naming
`engram migrate` / `migrate status` / `migrate revert` and the schema-version mechanism explicitly,
excluding `migrate-remap-owner`/`summarize-missing`/`reindex` per D-04.

**Section skeleton** (heading list, `reindex.md`):
```
## The migration flow
## Flags
## Output
## Resuming an interrupted run
## Repairing a pre-patch resume
## See also
```
Mirror this rhythm; exact headings are discretion (e.g. `## The migration flow` → `## Flags` →
`## Output` → recovery/edge-case section(s) covering `migrate revert`'s irreversible-range refusal →
`## See also`, linking back to `guides/upgrade.md:314-340`'s schema_version release note).

No sidebar registration needed — Starlight's `autogenerate: { directory: 'guides' }`
(`docs-site/astro.config.mjs:28-35`) picks up any new file under `guides/` automatically.

---

### `docs-site/src/content/docs/reference/memory-record.md` (extend) — component, transform

**Analog:** `### Supersession` / `### Archiving`, same file (lines 37-45 / 47-84 in the read excerpt
above; renumber against live line count when editing)

**Section shape to mirror:**
```
### Supersession

`supersede_memory` corrects one or more records without losing history. It stores
a single new, correcting record carrying a `supersedes` link to every target, and
stamps `superseded_by` onto each target — both additive; a target's content, tags,
and vector are untouched. ...
```
```
### Archiving

`engram spine-review archive --id <id>` explicitly retires a record: it stamps
`archived_at`, an entirely **new, orthogonal** key — distinct from both `not_after`
expiry and `superseded_by` supersession. ...
```

Each section: short lead paragraph naming the mechanism/verb, then behavioral detail, then a
cross-reference link. New sections needed: `schema_version` row in `## Field reference` (the table
currently has zero rows for `not_before`, `not_after`, or `schema_version` — confirmed by
RESEARCH.md), plus prose for the derived `expired`/`scheduled` words and their asymmetric boundary
rule (D-03), sourced from `internal/store/store.go`'s `activeWindowConditions` — **never re-derived
from this page's own existing prose**.

**Known staleness to fix in the same edit** (RESEARCH.md, not named by CONTEXT.md by line): the
`### Archiving` section's closing paragraph currently reads:
> "...but is **not present on the Connect lane**: `proto/engram/v1/engram.proto`'s `Memory` message
> does not carry `superseded_by`, `supersedes`, `not_before`, or `not_after` either... See
> [GitHub issue #482]... for the tracked follow-up to add all five fields to the Connect lane
> together."

This is false as of this milestone — `proto/engram/v1/engram.proto`'s `Memory` message now carries
all six fields (23-28) per `REQ-connect-record-state-parity` (#482, shipped). Correct this paragraph
in the same pass as the `schema_version` addition.

---

### Rejection test for `summarize.go`'s guard (test, request-response)

**Analog:** `TestSpineReviewScanRequiresScopeOrAllScopes`, `cmd/engram/spine_review_test.go:47-59`

```go
// TestSpineReviewScanRequiresScopeOrAllScopes pins the bare usageErrorf
// guard (mirroring summarize.go's exact wording) that rejects a scan
// invocation naming neither --scope nor --all-scopes.
func TestSpineReviewScanRequiresScopeOrAllScopes(t *testing.T) {
	resetClientFlags(t)
	_, _, err := runClient(t, "spine-review", "scan")
	if err == nil {
		t.Fatal("expected an error when neither --scope nor --all-scopes is supplied, got nil")
	}
	if got := exitCodeFromError(err); got != exitUsage {
		t.Errorf("exitCodeFromError(err) = %d, want %d (exitUsage)", got, exitUsage)
	}
}
```

Sibling analog, identical shape: `TestSpineReviewVerifyRequiresScopeOrAllScopes`,
`cmd/engram/spine_review_verify_test.go:523-535`. Both assert only `err != nil` and
`exitCodeFromError(err) == exitUsage` — no string-content assertion, so safe against any Sentence
wording change.

`cmd/engram/summarize_test.go` has **no equivalent test today** (verified by RESEARCH.md reading the
whole file) — add `TestSummarizeMissingRequiresScopeOrAllScopes` mirroring this shape exactly, using
`runClient(t, "summarize-missing")` (or the correct summarize verb invocation) with no `--scope`/
`--all-scopes` flags.

---

## Shared Patterns

### Rule-lookup-and-panic guard
**Source:** `cmd/engram/spine_review_purge.go:79-91` (`requirePurgeFilterScope`)
**Apply to:** All three sweep call-site conversions (`summarize.go`, `spine_review_scan.go`,
`spine_review_verify.go`). The panic message names the calling command and the missing rule's
registry const — copy that convention (`"<command>: surfaces.<RuleConst> is not registered in
internal/surfaces/rules.go"`).

### Anchor-pair-precedes-generator-map ordering
**Source:** `internal/surfaces/anchor.go`'s `WriteRegion` behavior (errors if anchor missing) +
`internal/surfacesgen/main.go`'s `ruleTargets` map
**Apply to:** All three docs/skill anchor edits. Order is fixed: (1) hand-write empty anchor pair in
each target file, (2) add `ruleTargets` entry, (3) `task surfaces:gen` (never bare `go run
./internal/surfacesgen`) fills bodies AND regenerates `cmd/engram/testdata/{help,catalog}.golden` if
any CLI `Usage` string changed.

### Zero-occurrence acceptance gate, not a fixed count
**Source:** CONTEXT.md D-01 / Specifics section
**Apply to:** The plan's acceptance gate for the sweep-guard conversion. Gate on
`rg -n "scope <scope> or --all-scopes is required" cmd/engram/ | wc -l` returning **zero**, never a
hardcoded "3 sites converted" assertion — a fourth hand-rolled site must fail the gate.

## No Analog Found

None — every file in scope has a direct, verbatim in-tree analog (the anchor-pair "empty form" is
the one inferred artifact, flagged explicitly above and in every host-file entry).

## Metadata

**Analog search scope:** `internal/surfaces/`, `internal/surfacesgen/`, `cmd/engram/`,
`docs-site/src/content/docs/{reference,guides}/`, `skill/engram/skills/curating-memory/`,
`.github/workflows/docs-site.yaml`
**Files scanned:** ~14 (all directly read via targeted `sed -n` ranges, no full-file loads)
**Pattern extraction date:** 2026-08-21
