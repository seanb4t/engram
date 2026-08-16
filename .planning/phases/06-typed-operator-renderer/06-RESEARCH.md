# Phase 6: Typed Operator Renderer - Research

**Researched:** 2026-08-16
**Domain:** Internal Go CLI output-rendering mechanism (no external library; stdlib `encoding/json` +
hand-rolled text-template substitution)
**Confidence:** MEDIUM — the codebase-facts sections (current surface, doc shapes, sentence bytes) are
HIGH (read directly, this session, with line citations). The proposed `FieldSet`/`Field` mechanism
itself is ORIGINAL DESIGN SYNTHESIS with no external precedent to verify against — treat those
sections as a recommendation for the planner to interrogate, not a fact.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 (Declaration Shape):** Each operator report is an **ordered `[]Field` value** carrying a text
  template plus the fields — no reflection over `xxxDoc` struct tags, no generic `Report[T]` +
  template-engine wrapper. Both lanes walk the same ordered slice: text renders the template, json is
  built from the fields in declaration order.
- **D-02 (Enforcement Locus):** Enforced at **compile time** — `renderOperator` stops accepting a
  free-form `doc any`. The only way to call it is with a field set. Either every call site converts in
  the same change as the signature, or behind a temporary second entry point — planner must pick one.
- **D-03 (Coverage Rule):** **Every field must be referenced by a `{key}` placeholder in the text
  template.** No prose-only coverage, no `Silent` escape hatch. Boolean fields whose text presence is
  prose today (e.g. `best_effort`) must become value renderers that emit the identical bytes.
- **D-04 (Sentence Fidelity):** All 15 existing text sentences preserved **byte-identical**,
  character-for-character. No named exception list. Some sentences are multi-line (header + one line
  per row).
- **D-05 (Nesting):** A `Field`'s value **may itself be a `FieldSet` or `[]FieldSet`**, recursing the
  D-03 coverage rule one level down. Four reports are already two-level: `archiveReportDoc`,
  purge/restore, `consolidateReportDoc`, `verifyReportDoc`.
- **D-06 (Conditional Presence):** A conditionally-present field carries **one explicit `Present`
  predicate that both lanes read**. When absent: omitted from JSON AND its placeholder plus surrounding
  literal segment drops from the sentence. Example: `archiveResultDoc.ID` (`Present: r.ID != ""`).
- **D-07 (Fate of the Existing Gate):** `TestOperatorOutputParity` and `operatorParityRows()` are
  **retired**, not kept as backstop, not rewritten as a derived both-ways gate. Two properties to
  record: its `facts` were hand-listed (what SC1 rejects), and it was one-directional (text→json only,
  never proved json does NOT widen past text).

### Claude's Discretion

- The concrete Go API of `FieldSet`/`Field` (names, whether `Text` is a method or field, placeholder
  syntax, when parse failure is detected).
- Whether placeholder-coverage failure surfaces at compile time, `init()`, or first render — must be
  reachable without executing every operator command against a live store.
- Multi-line and list-joining rendering mechanics for the four two-level reports.
- Migration order and batching across plans.

### Deferred Ideas (OUT OF SCOPE)

- Applying the field-set mechanism to the client tier (`engram search`/`list`/`get` renderers in
  `client_common.go`) — same class of problem, different tier, not this phase's boundary.
- Todo `2026-08-10-research-versioned-payload-migration-mechanism.md` — reviewed, not folded, already
  scoped into completed Phases 2-4.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-operator-renderer-typed | Operator command output derives text and json from ONE shared ordered field set, so a json document cannot widen past what its text sentence states. Field-set identity holds by construction rather than by a test over hand-built rows (#481). | Full baseline surface enumerated below (all 15 call sites, doc shapes, sentence variants); concrete `FieldSet`/`Field` design proposal with coverage-checking mechanism (non-vacuous, both-directions, mutation-provable); migration sequencing that keeps coverage non-zero throughout (D-07 answer). |

</phase_requirements>

## Summary

This phase has no external dependency to research — it is a from-scratch internal mechanism inside
`cmd/engram` (package `main`), Go 1.26.3, stdlib only. The real research work is **reading the existing
surface precisely enough that the planner does not rediscover its edge cases mid-implementation**: 15
operator commands, 19 `renderOperator` call sites (some commands have preview/apply/refusal branches
sharing one report-builder pair), 14 distinct hand-declared JSON doc types (one report — `migrate
status` — is a structural exception, passing a `store`-layer type directly), and at least 4 reports
with genuinely bespoke multi-line, per-row, or conditionally-branching sentence construction that a
naive single-template design will not reproduce byte-identical.

The three hardest technical problems the planner must solve, in order of risk:

1. **Conditional-span text rendering (D-06).** A `{key}` placeholder that simply substitutes to `""`
   when absent leaves stray literal text behind (` id=` with nothing after it) — `archiveResultDoc.ID`
   proves this is a live case today, not a hypothetical. The template syntax needs a second construct
   beyond bare `{key}` — an optional *span* that drops the placeholder AND its surrounding literal
   together. This is genuine mechanism design, not implementation detail.
2. **List-joining semantics (D-05).** The four two-level reports do not share one joining algorithm:
   `archiveSummary` joins per-row lines with `\n` and a fixed prefix; `verifySummary` runs THREE
   independent per-tier loops (moved/broken/unverifiable), never one list; `purgePreviewSummary`
   interleaves a fixed multi-sentence block between the header and the per-id loop.
   `consolidateSummary` has a conditional single line (`min_score=...`) before its per-pair loop. A
   single "list field" abstraction will not fit all four without an explicit per-field join
   configuration.
3. **JSON-shape preservation is a SEPARATE constraint from D-04's text-byte-identity, and today's
   fields do not use `omitempty` consistently.** Several reports (`migrateRemapReportDoc`,
   `migrateOutputDoc`, `revertOutputDoc`) always emit BOTH halves of a mode-dependent pair (e.g.
   `would_remap` AND `remapped`, one always zero) with no `omitempty` — only `archiveResultDoc.ID`,
   `consolidateReportDoc.MinScore`, and four fields of `purgeReportDoc` use `omitempty` today. If a
   report's `Present` predicate is applied by "which mode is this" rather than by "does today's struct
   tag say omitempty", the refactor will silently change JSON shape — a regression SC2 forbids even
   though D-04 only pins the text side.

**Primary recommendation:** Build the `FieldSet`/`Field` type plus its coverage-checking and rendering
functions as a small, fully-unit-tested, self-contained addition FIRST (nothing calls it yet, zero
regression risk), including a proof that the coverage checker can go RED. Then convert the 15 reports
one at a time, keeping `TestOperatorOutputParity` alive and unmodified until every report has
converted, retiring it only as the LAST step once a new bidirectional registry test covers the full
`operatorCommands()` set. For every field that carries a mode-dependent value today, default its
`Field.Present` to match the CURRENT `omitempty` tag exactly (not to "does this mode apply") — this is
the single highest-risk regression in the whole phase.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Operator report construction (counts, ids, options) | Backend / CLI command layer (`cmd/engram` `RunE` bodies) | `internal/store` (source data) | Unchanged by this phase — report BUILDERS already receive already-computed store results; only how those results become text/JSON changes. |
| Text/JSON rendering identity | CLI command layer (`cmd/engram`, package `main`) | — | Wholly internal to `cmd/engram`; D-02 confirms the signature is internal to package `main`, so no cross-tier contract changes. |
| `--output` flag parsing / TTY detection | CLI command layer (`operator_output.go`) | — | Explicitly OUT OF SCOPE (`addOperatorOutputFlag`, `operatorOutputFormat` unchanged per CONTEXT.md). |
| Coverage-checking (field-set identity proof) | CLI command layer, build/test-time | — | A `go test`/`init()`-time gate, not a runtime request-path component — never reachable from a live store dial. |

## Standard Stack

No external packages are introduced by this phase. Everything needed already exists in Go's standard
library and is already imported in `cmd/engram`:

| Package | Purpose | Why standard here |
|---------|---------|--------------------|
| `encoding/json` | JSON-mode encoding (already `renderOperator`'s encoder: `json.NewEncoder(cmd.OutOrStdout()).Encode(doc)`, `cmd/engram/operator_output.go:69-70`) | [VERIFIED: cmd/engram/operator_output.go:69-70] Already the sole JSON encoder for this tier; no reason to add a third-party encoder. |
| `strings` | Placeholder substitution, per-row joining (`strings.Builder`, `strings.TrimRight`, `strings.NewReplacer`) | Already used throughout the 15 report builders (e.g. `spine_review_archive.go:31` `var b strings.Builder`). |
| `fmt` | Scalar formatting parity with existing `fmt.Sprintf` sentences | Already the formatter for every existing `xxxSummary` function. |
| `time` | `time.Time` field rendering (RFC3339 formatting already used, e.g. `pruneSummary`, `before.Format(time.RFC3339)`) | Already imported and used identically across reports. |

**Explicitly rejected by D-01, do not introduce:**
- `reflect` — D-01 rejects "reflecting over the existing `xxxDoc` struct tags" as a design axis; this
  extends to using `reflect` internally for `Field.Val`'s type dispatch. Use an explicit Go type switch
  over `any` instead (`switch v := val.(type) { case string: ...; case uint64: ...; case time.Time:
  ...; case FieldSet: ...; case []FieldSet: ... }`).
- `text/template` (or any templating engine) — D-01 explicitly states "no template engine added to a
  CLI that has neither today."

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled `{key}` + span placeholder parser | Go stdlib `regexp` for placeholder extraction | `regexp` is stdlib (not a new dependency) and is reasonable for the discovery/coverage-check pass (extracting placeholder names from `Text`), but recommend a hand-rolled single-pass scanner for the RENDER path itself (performance and precise control over the conditional-span drop semantics that a generic regex substitution cannot express cleanly). |
| Ordered JSON via `FieldSet.MarshalJSON` | A generic `map[string]any` passed to `json.Marshal` | REJECTED — Go's `encoding/json` does not preserve `map[string]any` key order (Go maps have randomized iteration order and `encoding/json` sorts map keys alphabetically on marshal), which would silently reorder every JSON document. `MarshalJSON` must walk the ordered `[]Field` slice directly. |

**Installation:** None — no new dependency, no `go get`/`go install` step for this phase.

**Version verification:** N/A — no package added or upgraded.

## Package Legitimacy Audit

**Not applicable.** This phase adds no external package to `go.mod`. The entire mechanism is Go
stdlib (`encoding/json`, `strings`, `fmt`, `time`; optionally `regexp` for placeholder discovery only).

**Packages removed due to [SLOP] verdict:** none — no packages evaluated, none installed.
**Packages flagged as suspicious [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
                     ┌─────────────────────────────────────────┐
                     │   Operator RunE handler (e.g. prune.go)  │
                     │   dials store, computes result values    │
                     └───────────────────┬───────────────────────┘
                                          │ result values (counts, ids, times, options)
                                          ▼
                     ┌─────────────────────────────────────────┐
                     │  Report builder (e.g. pruneReport(...))  │
                     │  returns ONE FieldSet{ Text, Fields }    │
                     │  — the ONLY declaration site per report  │
                     └───────────────────┬───────────────────────┘
                                          │ FieldSet value
                                          ▼
                     ┌─────────────────────────────────────────┐
                     │      renderOperator(cmd, format, fs)     │
                     │  format == text        format == json    │
                     └──────────┬────────────────────┬──────────┘
                                │                     │
                 ┌──────────────▼───────┐   ┌─────────▼─────────────┐
                 │ walk fs.Fields, sub-  │   │ FieldSet.MarshalJSON: │
                 │ stitute {key}/[span]  │   │ walk fs.Fields in     │
                 │ into fs.Text          │   │ declaration order,    │
                 │ (Present gates both   │   │ respecting Present,   │
                 │  the value AND the    │   │ recursing into nested │
                 │  surrounding literal) │   │ FieldSet/[]FieldSet   │
                 └──────────┬────────────┘   └─────────┬─────────────┘
                            │                            │
                            ▼                            ▼
                     fmt.Fprintln(stdout)          json.Encoder.Encode(stdout)

   ── build/test time, NOT in the request path ──────────────────────────────
   validateFieldSet(fs) — asserts SET EQUALITY between {Field.Key present in
   fs at construction} and {placeholder keys referenced in fs.Text}, recursing
   into nested FieldSet/[]FieldSet. Wired into a registry test gated
   bidirectionally against operatorCommands() (cobra-tree-derived), mirroring
   the retired TestOperatorOutputParity's both-ways row-set discipline.
```

### Recommended Project Structure

No new package or directory — this stays inside `cmd/engram` (package `main`), matching D-02's note
that the signature is internal to package `main`. Recommend one new file for the shared mechanism,
co-located with `operator_output.go`:

```
cmd/engram/
├── operator_output.go        # unchanged surface (addOperatorOutputFlag, operatorOutputFormat);
│                              # renderOperator's signature changes here (D-02)
├── operator_output_test.go   # TestOperatorOutputParity retired LAST (D-07); the 3 behavioral
│                              # contract tests (TestRenderOperatorTextAndJSON, TestOperatorOutputEmpty,
│                              # TestOperatorOutputStream) MUST keep passing unmodified
├── fieldset.go                # NEW: FieldSet, Field types; validateFieldSet; MarshalJSON; text render
├── fieldset_test.go            # NEW: coverage-checker unit tests INCLUDING the red-proof mutation test
├── prune.go, reindex.go, ...  # each of the 15 report builders converts to return FieldSet
```

### Pattern 1: FieldSet / Field — the shared declaration type

**What:** One ordered, explicit slice of named values plus the sentence template that must reference
every one of them.
**When to use:** Every operator report, replacing the current `xxxSummary(...) string` +
`xxxDoc`/`xxxReportDoc` struct pair.
**Example (proposed, NOT verified against any external source — original synthesis for this phase):**

```go
// Field is one named, typed value in an operator report: the single
// declaration point for how it appears in JSON (Key) and in text (via a
// {Key} placeholder, or a [ ... {Key} ... ] conditional span when Present
// can be false).
type Field struct {
	Key string // JSON key AND placeholder name; unique within a FieldSet.
	Val any    // string | bool | uint64 | int | float32 | time.Time | FieldSet | []FieldSet

	// Present, when non-nil and false, omits Val from JSON (the key is
	// dropped entirely, mirroring today's omitempty fields) AND drops the
	// placeholder together with its enclosing [ ] span from the rendered
	// text (D-06) — one predicate, both lanes read it. nil means always
	// present (the common case).
	Present *bool

	// Render overrides how Val appears in TEXT ONLY — required whenever the
	// byte-exact prose form differs from the natural Go value (e.g.
	// best_effort's bool -> "best-effort count" / "" mapping, D-03's worked
	// example). If nil, a built-in formatter is used (fmt.Sprint for
	// scalars, RFC3339 for time.Time).
	Render func(v any) string
}

// FieldSet is one operator report: a sentence template plus the ordered
// fields it must fully account for. Text uses two placeholder forms:
//   - bare {key}      — unconditional interpolation
//   - [literal{key}literal] — a conditional span; included as a whole
//     (literal text AND the interpolated value) only when that field's
//     Present is true or nil; dropped as a whole otherwise.
// Every Field.Key must appear in exactly one of these forms in Text
// (D-03); every placeholder in Text must name a real Field.Key. A field
// with a non-nil Present MUST use the bracketed span form, never a bare
// {key} — validateFieldSet rejects the mismatch.
type FieldSet struct {
	Text   string
	Fields []Field
}
```

**Worked conversion — `prunePreviewDoc`/`prunePreviewSummary` (the D-03 worked example):**

Current code [VERIFIED: cmd/engram/prune.go:109-115,127-145]:
```go
func prunePreviewSummary(eligible uint64, before time.Time) string {
	return fmt.Sprintf("preview: %d expired record(s) eligible for deletion (not_after < %s; best-effort count); re-run with --apply to delete",
		eligible, before.Format(time.RFC3339))
}
type pruneOutputDoc struct {
	Preview    bool      `json:"preview"`
	Eligible   uint64    `json:"eligible"`
	Deleted    uint64    `json:"deleted"`
	Before     time.Time `json:"before"`
	BestEffort bool      `json:"best_effort"`
}
```

Proposed conversion (illustrative — the exact bracket syntax is a discretion item, not locked):
```go
func prunePreviewReport(eligible uint64, before time.Time) FieldSet {
	return FieldSet{
		Text: "preview: {eligible} expired record(s) eligible for deletion (not_after < {before}; {best_effort}); re-run with --apply to delete",
		Fields: []Field{
			{Key: "preview", Val: true},
			{Key: "eligible", Val: eligible},
			{Key: "deleted", Val: uint64(0)},
			{Key: "before", Val: before, Render: func(v any) string { return v.(time.Time).Format(time.RFC3339) }},
			{Key: "best_effort", Val: true, Render: func(v any) string {
				if v.(bool) {
					return "best-effort count"
				}
				return ""
			}},
		},
	}
}
```

Note `deleted` has no natural text presence at all in this sentence (it is always 0 on the preview
path, unrepresentable in prose) — D-03 requires a placeholder for EVERY field, so it must appear
literally as `{deleted}` somewhere, OR (more likely correct) `deleted` should not be declared as a
Field on the PREVIEW variant at all — the applied variant (`pruneReportDoc`) is a DIFFERENT FieldSet
with its own Text, not a shared struct with unused fields. This is the resolution: **preview and
applied are two distinct FieldSet-returning functions (matching today's two distinct doc-converter
functions `prunePreviewDoc`/`pruneReportDoc`), not one shared struct type with mode-dependent
population.** Carrying today's `pruneOutputDoc` struct's habit of "one struct, all fields populated
differently per mode" forward into FieldSet would immediately violate D-03 for whichever field is
inapplicable in a given mode.

### Pattern 2: Nested FieldSet for multi-row reports (D-05)

**What:** A `Field.Val` of type `[]FieldSet`, each element carrying its OWN `Text` (the per-row line
template) and `Fields`.
**When to use:** The four two-level reports — `archiveReportDoc`/`archiveResultDoc`,
`purgeReportDoc`'s id lists, `consolidateReportDoc`/`consolidatePairDoc`, `verifyReportDoc`/
`verifyEntryDoc`.
**Critical finding — one list abstraction does not fit all four; each needs its own join
configuration:**

| Report | Rows come from | Join separator | Row template (verbatim) | Notes |
|--------|-----------------|-----------------|--------------------------|-------|
| `archiveSummary` | one `[]store.ArchiveResult` | `"\n"`, `strings.TrimRight` at the end | `"  requested=%s id=%s outcome=%s"` OR (D-06) `"  requested=%s outcome=%s"` when `ID==""` | [VERIFIED: cmd/engram/spine_review_archive.go:30-43] — one list, one row shape, D-06 governs the `id=` segment. |
| `purgePreviewSummary` | one `[]string` (eligible ids) | `"\n"` | `"  id=%s"` | [VERIFIED: cmd/engram/spine_review_purge.go:276-287] — the row loop is sandwiched between the header line and TWO fixed full-sentence notice lines (`purgeSameRunLimitationNotice`, `purgeIntersectionScopingNotice`) plus a re-run command line — five distinct template segments in one FieldSet, not just "header + list". |
| `purgeAppliedSummary` | THREE separate `[]string` (`res.Deleted`, `res.Spared`, `res.Appeared`) | `"\n"` per block | `"  deleted id=%s"`, `"  spared id=%s"`, `"  appeared id=%s (not purged; re-run to include)"` | [VERIFIED: cmd/engram/spine_review_purge.go:294-309] — THREE independent nested lists in one report, each with its own row template — not one list field. |
| `consolidateSummary` | one `[]store.DuplicatePair` | `"\n"` | `"  score=%g a=%s (short_id=%s scope=%q) b=%s (short_id=%s scope=%q)"` | [VERIFIED: cmd/engram/spine_review_consolidate.go:190-207] — preceded by a CONDITIONAL single line (`min_score=%g` vs `min_score=(none...)`) that is itself a D-06 Present-gated field, not a list. |
| `verifySummary` | THREE separate `[]verifyEntry` (`report.Moved`, `.Broken`, `.Unverifiable`) | `"\n"` per block | `"  moved record=%s short_id=%s ref=%q: %s"`, `"  broken ..."`, `"  unverifiable ..."` | [VERIFIED: cmd/engram/spine_review_verify.go:528-542] — same three-independent-lists shape as purge-applied. |
| `spineScanSummary` | one `[]spineScanBreakdownDoc`-shaped source (`res.ByScopeCategory`) | `"\n"` | `"  scope=%q category=%q count=%d"` | [VERIFIED: cmd/engram/spine_review_scan.go:131-145] — appended after THREE fixed lines of scalar counts, not immediately after the header. |
| `statusSummary` | TWO lists (`res.Buckets` always, `res.Future` only when non-empty) | inline `", %d at v%d"` / `" v%d=%d"` — **NOT newline-joined**, appended to the SAME line as the header | [VERIFIED: cmd/engram/migrate_family.go:303-316] — this is the one report whose "list" renders inline on one line, not one line per row. `Future`'s whole clause (the `"; %d record(s) at a version newer..."` prefix plus every `v%d=%d` element) is itself D-06-conditional on `len(res.Future) > 0`. |

**Recommendation:** Do not attempt a single generic "list field with one join style" — give each
`[]FieldSet`-valued `Field` its own explicit join configuration (a `Sep string` on the parent field, or
equivalently let the nested elements' own `Text` include their own leading `"\n  "` and rely on
`strings.Join`/simple concatenation at the parent). `statusSummary`'s inline (non-newline) join is the
strongest argument against a single hardcoded "one row per newline" default — make the separator an
explicit per-field property, not an implicit constant.

### Pattern 3: Conditional presence with surrounding-literal drop (D-06)

**What:** `Present *bool` (or `Present func() bool`) on a `Field`, read by BOTH lanes, dropping the
placeholder AND its adjacent literal segment together — not merely substituting empty string.
**When to use:** Every field with `omitempty` in its current struct tag.

**Full inventory of today's `omitempty`/conditional fields** [VERIFIED: line-cited struct definitions
below — these are the ONLY fields in the 15 reports carrying `omitempty` or structurally-conditional
JSON presence read this session]:

| Field | Struct | Tag (verbatim) | Text-side conditional behavior (verbatim from source) |
|-------|--------|------------------|----------------------------------------------------------|
| `ID` | `archiveResultDoc` | `` `json:"id,omitempty"` `` [VERIFIED: cmd/engram/spine_review_archive.go:54] | `if r.ID == "" { ... "  requested=%s outcome=%s\n" ... } else { ... "  requested=%s id=%s outcome=%s\n" }` [VERIFIED: cmd/engram/spine_review_archive.go:36-40] |
| `MinScore` | `consolidateReportDoc` | `` `json:"min_score,omitempty"` `` [VERIFIED: cmd/engram/spine_review_consolidate.go:159] | `if minScore != nil { "  min_score=%g\n" } else { "  min_score=(none — no filter applied; negative-scoring pairs are reported)\n" }` [VERIFIED: cmd/engram/spine_review_consolidate.go:198-201] — note: this is NOT a drop, it is a REPLACEMENT with alternate prose — a different D-06 shape than "drop the whole segment." |
| `Scope` | `purgeReportDoc` | `` `json:"scope,omitempty"` `` [VERIFIED: cmd/engram/spine_review_purge.go:197] | rendered via `purgeScopeText(opts)` which returns `"(none)"` literal, never omitted from text — another REPLACEMENT-not-drop shape. |
| `Category` | `purgeReportDoc` | `` `json:"category,omitempty"` `` [VERIFIED: cmd/engram/spine_review_purge.go:199] | not directly interpolated into `purgePreviewSummary`'s header line at all — category is absent from the rendered TEXT entirely today. |
| `Tags` | `purgeReportDoc` | `` `json:"tags,omitempty"` `` [VERIFIED: cmd/engram/spine_review_purge.go:200] | same — absent from rendered text today. |
| `OlderThan` | `purgeReportDoc` | `` `json:"older_than,omitempty"` `` [VERIFIED: cmd/engram/spine_review_purge.go:201] | same — absent from rendered text today. |
| `Refusal` | `revertOutputDoc` | `` `json:"refusal,omitempty"` `` [VERIFIED: cmd/engram/migrate_family.go:342] | populated only on the refusal path; `revertSummary`'s refusal branch is a WHOLLY DIFFERENT sentence shape (not a segment drop from the applied/preview sentence) [VERIFIED: cmd/engram/migrate_family.go:386-398]. |

**This inventory reveals D-06's canonical example (`archiveResultDoc.ID`) is the ONLY case in the
current codebase that is actually a clean "drop the placeholder and its literal" shape.** Every other
`omitempty` field today is either (a) always rendered in text via alternate prose (never dropped:
`MinScore`, `Scope`), (b) never rendered in text at all regardless of JSON presence (`Category`, `Tags`,
`OlderThan`), or (c) belongs to an entirely different sentence variant, not a segment drop (`Refusal`).
**Recommendation:** implement the D-06 bracketed-span mechanism for the one case that needs it
(`archiveResultDoc.ID`-shaped fields), and for the other `omitempty` fields, keep the field's JSON
`Present` predicate matching the CURRENT `omitempty` semantics exactly, while its Text placeholder
either (i) always renders via a `Render` function that already returns the correct alternate-prose
string regardless of presence (`MinScore`, `Scope` pattern), or (ii) is a field that is genuinely never
referenced in Text at all — which is **only legal if that field is dropped from the report's Fields
slice on the text-affecting variant**, since D-03 forbids an un-referenced field. `Category`/`Tags`/
`OlderThan` are the sharpest case: they exist in JSON today but never in text — under D-03 as locked,
this is now a coverage FAILURE that the phase must resolve, not merely preserve. The planner has two
lawful choices: (a) add a placeholder for them (changing the text sentence — forbidden by D-04's
no-exception-list), or (b) the phase's actual answer per D-03's own text ("if a sentence cannot be
reproduced, that is a finding about the mechanism") is that these three fields must gain a text
placeholder, and per D-04 the OPERATOR asked for the CURRENT sentence to be preserved bytes... **this
is a genuine conflict between D-03 (universal coverage) and D-04 (byte-identical text) for these three
specific fields, and it is not resolvable by mechanism design alone — flag it explicitly for the
planner/executor to surface as a checkpoint**, since the two locked decisions cannot both hold for
`Category`/`Tags`/`OlderThan` as currently rendered. (Likely resolution: these three fields ARE
referenced in text already, just not in the header line — re-check `purgeRerunCommand`, which embeds
`--category`, `--tags`, `--older-than` into the printed re-run command line, itself part of
`purgePreviewSummary`'s rendered text [VERIFIED: cmd/engram/spine_review_purge.go:157-183,
`purgeRerunCommand`'s flag-echoing logic, and cmd/engram/spine_review_purge.go:285 `re-run: %s`]. If
the FieldSet's placeholder for these fields interpolates through the re-run-command renderer rather
than a bare value, D-03 and D-04 both hold. This is a real out: flag it as the answer, but the planner
must verify it holds for all three before treating this pitfall as closed.)

### Anti-Patterns to Avoid

- **One shared struct type across preview/applied/refusal modes with mode-dependent field
  population.** This is TODAY's pattern (`pruneOutputDoc`, `migrateRemapReportDoc`, `migrateOutputDoc`,
  `revertOutputDoc` all do this) and it is precisely what makes D-03 hard to satisfy — a field that is
  meaningless in one mode still needs SOME text representation under strict per-field coverage. Prefer
  one `FieldSet`-returning function per sentence VARIANT (mirroring today's `prunePreviewDoc` vs
  `pruneReportDoc` being separate functions already), not one struct serving all variants.
- **Using `reflect` anywhere in the mechanism** — explicitly rejected by D-01; use type switches over
  `any`.
- **Hand-rolled JSON string escaping** inside `FieldSet.MarshalJSON` — see Don't Hand-Roll below.
- **Substituting a bare `{key}` to `""` for a Present-gated field** — leaves the surrounding literal
  behind (a real regression class, see D-06 discussion above); the span-drop must remove both together.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON string/value escaping inside `FieldSet.MarshalJSON` | A manual `"` `\` escaping routine for string values | `json.Marshal(value)` per scalar, called individually for each `Field.Val` and concatenated into the object buffer with hand-written `{`, `,`, `:`, `}` structural bytes only | `encoding/json`'s escaping (control chars, non-ASCII, `<`/`>`/`&` HTML-escaping behavior) is exactly what `TestOperatorOutputEncoding` already pins for `spineScanBreakdownDoc.Scope` [VERIFIED: cmd/engram/operator_output_test.go:389-404] — reusing `json.Marshal` per value guarantees identical escaping without re-deriving it. |
| Ordered-key JSON object construction | A `map[string]any` passed to `json.Marshal` | A custom `MarshalJSON` that walks `[]Field` in declaration order, writing structural bytes directly | Go's `encoding/json` does not preserve map iteration order for `map[string]any` — it marshals map keys in SORTED order, which would silently reorder every JSON document relative to today's struct-field-declaration order. |
| Placeholder syntax parsing | A regex-heavy or ad-hoc string-splitting parser reinvented per call site | ONE shared parser function (e.g. `parseFieldSetText(text string) (placeholders []placeholderRef, err error)`) called once by both the coverage checker and the two render lanes | A duplicated parser between "what does the coverage checker see" and "what does the text renderer actually substitute" is exactly the two-independent-conditionals shape D-06 exists to eliminate — one parse, shared by every consumer. |

**Key insight:** the entire risk surface in this phase is "the coverage-checker's notion of a field
disagreeing with the renderer's notion of a field." Every other engineering decision (which stdlib
functions to call) is low-risk; keeping exactly ONE parse/walk of `FieldSet.Text` shared by the
JSON encoder, the text renderer, AND the coverage validator is the single design choice that prevents
that class of bug by construction.

## Current Operator-Tier Surface (Regression Baseline for SC2)

All 15 rows below are read directly from source this session; each cell with a `[VERIFIED: path:line]`
tag was opened and quoted this session. This table is the authoritative "what must not change" ledger
for SC2 — the planner's task-level acceptance criteria should reference specific rows here.

| # | `commandKey` | `renderOperator` call site(s) | Doc type(s) | Text builder(s) | Sentence variant count |
|---|--------------|-------------------------------|-------------|-------------------|--------------------------|
| 1 | `reindex` | `reindex.go:88` | `reindexOutputDoc` (9 fields) [VERIFIED: cmd/engram/reindex.go:119-129] | `reindexSummary` | 3 (dry-run, dry-run+resume, applied) [VERIFIED: cmd/engram/reindex.go:98-112] |
| 2 | `prune-expired` | `prune.go:84` (preview), `prune.go:106` (apply) | `pruneOutputDoc` (5 fields) [VERIFIED: cmd/engram/prune.go:133-139] | `prunePreviewSummary`, `pruneSummary` | 2 |
| 3 | `summarize-missing` | `summarize.go:72` | `summarizeOutputDoc` (5 fields) [VERIFIED: cmd/engram/summarize.go:91-97] | `summarizeSummary` | 2 (dry-run/live) — note `Failed` has NO text presence in dry-run mode [VERIFIED: cmd/engram/summarize.go:78-85] |
| 4 | `backfill-short-ids` | delegates to `migrateSweepPreviewRun`/`migrateSweepApplyRun` (shared with `migrate`) [VERIFIED: cmd/engram/backfill.go:19-58 doc comment "thin delegating alias"] | shares `migrateOutputDoc` | shares `migrateSummary` | shares `migrate`'s 2 |
| 5 | `migrate-remap-owner` | `migrate.go:187` | `migrateRemapReportDoc` (4 fields, `WouldRemap`/`Remapped` BOTH always-serialized, no omitempty) [VERIFIED: cmd/engram/migrate.go:216-221] | `migrateRemapSummary` | 2 |
| 6 | `migrate-set-owner` | `migrate.go:64` | `migrateSetOwnerReportDoc` (2 fields, constructed as an INLINE struct literal — no builder function, unlike every other report) [VERIFIED: cmd/engram/migrate.go:64,78-81] | `migrateSetOwnerSummary` | 1 |
| 7 | `spine-review scan` | `spine_review_scan.go:66` | `spineScanReportDoc` (12 fields, 1 nested list `[]spineScanBreakdownDoc`) [VERIFIED: cmd/engram/spine_review_scan.go:82-98] | `spineScanSummary` | 1, + dynamic-length breakdown loop |
| 8 | `spine-review verify` | `spine_review_verify.go:659` | `verifyReportDoc` (7 fields, 3 nested lists sharing `verifyEntryDoc`) [VERIFIED: cmd/engram/spine_review_verify.go:492-501] | `verifySummary` | 1, + 3 independent dynamic-length loops [VERIFIED: cmd/engram/spine_review_verify.go:528-542] |
| 9 | `spine-review consolidate` | `spine_review_consolidate.go:130` | `consolidateReportDoc` (7 fields, 1 pointer field `MinScore`, 1 nested list) [VERIFIED: cmd/engram/spine_review_consolidate.go:155-163] | `consolidateSummary` | 1, + conditional min_score line + dynamic loop |
| 10 | `spine-review archive` | `spine_review_archive.go:145` (shared with restore, via `renderArchiveResults`) | `archiveReportDoc` (2 fields, nested `archiveResultDoc` with D-06 conditional `ID`) [VERIFIED: cmd/engram/spine_review_archive.go:49-66] | `archiveSummary` | 1 shape, `verb` param varies header only, per-row conditional |
| 11 | `spine-review restore` | same call site as #10, `verb="restore"` | same as #10 | same as #10 | same |
| 12 | `spine-review purge` | `spine_review_purge.go:336` (preview), `:377` (apply) | `purgeReportDoc` (16 fields — the largest doc in this tier) [VERIFIED: cmd/engram/spine_review_purge.go:194-215] | `purgePreviewSummary`, `purgeAppliedSummary` | 2, each with dynamic id-list loops + 2 shared full-sentence notice constants embedded verbatim |
| 13 | `migrate` | `migrate_family.go:174` (preview), `:211` (apply) | `migrateOutputDoc` (9 fields) [VERIFIED: cmd/engram/migrate_family.go:86-96] | `migrateSummary` | 2 |
| 14 | `migrate status` | `migrate_family.go:278` | **`store.MigrateStatusResult` passed DIRECTLY** — the sole exception to "hand-declared CLI-only doc struct" in this entire tier [VERIFIED: cmd/engram/migrate_family.go:282-296, doc comment: "no separate CLI-side type exists"]; `statusReportDoc` only null-safes two slice fields | `statusSummary` | 1, + 2 dynamic-length loops (`Buckets` always, `Future` D-06-conditional on `len(res.Future) > 0`), INLINE (non-newline) join [VERIFIED: cmd/engram/migrate_family.go:303-316] |
| 15 | `migrate revert` | `migrate_family.go:455` (preview), `:517` & `:539` (two distinct refusal render sites), `:548` (applied) | `revertOutputDoc` (9 fields incl. conditional `Refusal` string) [VERIFIED: cmd/engram/migrate_family.go:333-343] | `revertSummary` | 3 shapes: refusal (2 sub-variants — bare vs. with-already-landed-progress), preview, applied [VERIFIED: cmd/engram/migrate_family.go:386-405] |

**Totals:** 15 `commandKey` entries (confirmed against `operatorCommands()`'s live-tree derivation via
`TestOperatorOutputParity`'s existing bidirectional check [VERIFIED: cmd/engram/operator_output_test.go:344-364]),
19 non-test `renderOperator` call sites, 14 distinct hand-declared doc structs (one report reuses a
`store`-layer type directly), 4 reports requiring D-05 nesting, at least 1 confirmed clean D-06 case
(`archiveResultDoc.ID`) plus several `omitempty` fields whose text-side behavior is NOT a clean drop
(see Pattern 3 above).

**Three behavioral contracts that MUST keep passing unmodified through the whole migration**
[VERIFIED: cmd/engram/operator_output_test.go — quoted]:
- `TestRenderOperatorTextAndJSON` — text mode writes exactly `"hello\n"`; json mode writes exactly one
  `json.Unmarshal`-parseable document with exactly one trailing newline [cmd/engram/operator_output_test.go:84-119].
- `TestOperatorOutputEmpty` — every report's zero-value doc marshals with **zero occurrences of the
  substring `"null"`** anywhere in the JSON [cmd/engram/operator_output_test.go:411-437, assertion at 432: `if strings.Contains(string(b), "null")`].
- `TestOperatorOutputStream` — json mode writes to `cmd.OutOrStdout()` ONLY; a stderr warning must never
  leak into stdout [cmd/engram/operator_output_test.go:444-473].

These three are INDEPENDENT of `renderOperator`'s internal doc/text coupling — they test the
function's own I/O contract — so converting the signature does not obsolete them; they are the
regression gate that survives D-07's retirement of `TestOperatorOutputParity`.

## Common Pitfalls

### Pitfall 1: JSON shape drift from "which mode applies" instead of "which field had `omitempty`"

**What goes wrong:** A report author sets `Field.Present` to true only for the fields meaningful in the
CURRENT mode (e.g. only `Remapped` when `!dryRun`, only `WouldRemap` when `dryRun`), inadvertently
omitting a field from JSON that today is ALWAYS present (with value 0) because the struct has no
`omitempty` tag.
**Why it happens:** D-06's `Present` predicate is genuinely useful for hiding fields, and "hide the
field that doesn't apply to this mode" reads as the natural use — but today's `migrateRemapReportDoc`,
`migrateOutputDoc`, and `revertOutputDoc` (non-refusal fields) all emit BOTH halves of a mode-dependent
pair unconditionally [VERIFIED: cmd/engram/migrate.go:216-221 no `omitempty` tags on `WouldRemap`/
`Remapped`; cmd/engram/migrate_family.go:86-96 no `omitempty` on any of `WouldMigrate`/`Migrated`/
`Failed`/`Passes`/`Backlog`/`Spared`/`Appeared`].
**How to avoid:** `Present` must default to `true` (always-present) for every field UNLESS today's
struct tag says `omitempty` — treat the Pitfall 3 inventory table above as the exhaustive allowlist of
fields eligible for `Present: false`.
**Warning signs:** A converted report's JSON output for one mode has FEWER top-level keys than today's
struct would marshal for the same inputs — diff `json.Marshal` output field-by-field (key set, not
just scalar-value containment like the retired test did) against a fixture built from the OLD doc
struct before deleting the old struct.

### Pitfall 2: D-03/D-04 conflict for fields with no direct-header text presence (`purgeReportDoc`'s `Category`/`Tags`/`OlderThan`)

**What goes wrong:** These three fields exist in JSON today but are never interpolated into
`purgePreviewSummary`'s header line — only into the embedded re-run command string
(`purgeRerunCommand`). A naive reading of D-03 ("every field needs a `{key}` placeholder") could lead
to inserting a new bare mention of them into the header, which would violate D-04's byte-identical
requirement (no exception list).
**Why it happens:** The coverage rule and the byte-identity rule were designed against the SIMPLE case
(most fields have a direct, single textual mention); these three fields' only textual representation
today is INDIRECT, through a helper function's output that itself becomes part of the rendered text.
**How to avoid:** Treat `purgeRerunCommand`'s output as itself the value a `Field`'s `Render` function
produces (or as a further-nested structure), so the placeholder for `--category`/`--tags`/
`--older-than` resolves through that same command-echoing logic rather than a bare scalar substitution
— this keeps both D-03 (referenced) and D-04 (byte-identical) satisfied simultaneously. Verify this
resolution against the ACTUAL rendered bytes before considering the pitfall closed; it is a proposed
resolution, not a confirmed one.
**Warning signs:** A diff of `purgePreviewSummary`'s output before/after conversion showing ANY byte
change in the header or re-run-command line.

### Pitfall 3: `migrate status`'s exception to the hand-declared-doc-struct pattern

**What goes wrong:** `statusReportDoc` currently passes `store.MigrateStatusResult` straight through
(only null-safing two slices), reusing the `store` package's own JSON tags directly
[VERIFIED: cmd/engram/migrate_family.go:282-296, comment: "store.MigrateStatusResult's own json tags
... are already the exact contract this surface needs, so no separate CLI-side type exists"]. D-02
requires `renderOperator`'s ONLY accepted argument to be a `FieldSet` — there is no path for a raw
`store`-layer type to reach it anymore, so this report MUST gain its own hand-declared `FieldSet`
builder like every other report, losing the "reuse the store type's tags" shortcut it has today.
**Why it happens:** This report is structurally different from the other 14 today; a mechanical
find-and-replace across "every `renderOperator` call site" will not surface this one's extra work
(writing a NEW field-by-field mapping from `store.MigrateStatusResult` into `Field`s, including its
two dynamic-length bucket lists) unless the planner reads this specific file.
**How to avoid:** Budget `migrate status`'s conversion as strictly harder than the other single-shape
reports — it needs BOTH the D-05 nesting mechanism (twice — `Buckets` and `Future`) AND D-06 conditional
presence (`Future`'s entire clause) AND the inline (non-newline) join style unique to this report.
**Warning signs:** Any plan that batches `migrate status` into a "simple, low-risk" wave alongside
single-shape reports like `migrate-set-owner`.

### Pitfall 4: coverage checker parses a DIFFERENT notion of "placeholder" than the renderer

**What goes wrong:** The coverage-validation pass (build/test-time) and the actual text-render pass
(runtime) each independently re-parse `FieldSet.Text` for `{key}`/`[...]` occurrences. If their parsing
logic drifts (e.g. the validator treats a lone `{` inside a literal segment as a placeholder start but
the renderer treats it as literal text, or vice versa), the coverage guarantee is unsound — exactly the
"two independent conditionals" failure mode D-06 exists to eliminate, reintroduced one layer up.
**Why it happens:** It is tempting to write the coverage check as a quick standalone regex pass and the
renderer as separate hand-written substitution code, since they are used at different times (test vs.
runtime).
**How to avoid:** One shared parse function, called by both. See Don't Hand-Roll above.
**Warning signs:** A field that the validator accepts as "covered" but that renders incorrectly (or
vice versa) — this is exactly the kind of gap a mutation-based red-proof test (Validation Architecture
below) is designed to catch, so this pitfall doubles as the reason that test is mandatory, not optional
extra credit.

### Pitfall 5: `migrateSetOwnerReportDoc`'s inline-literal construction habit

**What goes wrong:** Fourteen of the fifteen reports build their doc via a dedicated converter
function (`pruneReportDoc(...)`, `archiveDoc(...)`, etc.); `migrate-set-owner` instead constructs its
doc as an inline struct literal directly at the `renderOperator` call site
[VERIFIED: cmd/engram/migrate.go:64 `renderOperator(cmd, format, migrateSetOwnerSummary(migrateOwner,
n), migrateSetOwnerReportDoc{Owner: migrateOwner, Stamped: n})`]. A mechanical "find every `xxxDoc(...)`
converter and rename it to return `FieldSet`" refactor script will silently miss this one call site.
**How to avoid:** Enumerate conversions from the `renderOperator` call-site grep (19 sites), not from a
converter-function-name grep.

## Code Examples

### `renderOperator`'s new signature (D-02)

```go
// renderOperator writes cmd's final result through the ONE rendering path
// every operator command and spine-review leaf shares. The ONLY way to
// call it is with a FieldSet — there is no longer a free-form doc any
// parameter, so a json document cannot widen past what fs.Text states
// (D-02). Both branches walk fs — text substitutes {key}/[span] into
// fs.Text; json builds the object from fs.Fields in declaration order via
// fs.MarshalJSON (called implicitly by enc.Encode).
func renderOperator(cmd *cobra.Command, format outputFormat, fs FieldSet) error {
	if format == formatText {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), fs.renderText())
		return err
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	return enc.Encode(fs)
}
```

Every one of the 19 call sites converts from `renderOperator(cmd, format, xxxSummary(...),
xxxDoc(...))` to `renderOperator(cmd, format, xxxReport(...))` where `xxxReport` is the ONE new builder
function replacing both the old summary function and the old doc-converter function.

### Coverage validator sketch (build/test-time, never in the request path)

```go
// validateFieldSet asserts SET EQUALITY (never partition/count identity —
// this project's recorded gotcha 01mdq5qq9j) between the field keys fs
// declares as Present at these particular values, and the placeholder keys
// fs.Text actually references — recursing into nested FieldSet/[]FieldSet
// values one level at a time. Returns a descriptive error naming every
// mismatched key, never a bare bool, so a failing case is diagnosable from
// the test output alone.
func validateFieldSet(fs FieldSet) error {
	declared := map[string]bool{}
	for _, f := range fs.Fields {
		if f.Present == nil || *f.Present {
			declared[f.Key] = true
		}
	}
	referenced, err := parseFieldSetPlaceholders(fs.Text) // the ONE shared parser (see Don't Hand-Roll)
	if err != nil {
		return err
	}
	if diff := setSymmetricDifference(declared, referenced); len(diff) > 0 {
		return fmt.Errorf("field-set coverage mismatch: %v", diff)
	}
	for _, f := range fs.Fields {
		switch v := f.Val.(type) {
		case FieldSet:
			if err := validateFieldSet(v); err != nil {
				return fmt.Errorf("field %q: %w", f.Key, err)
			}
		case []FieldSet:
			for i, elem := range v {
				if err := validateFieldSet(elem); err != nil {
					return fmt.Errorf("field %q[%d]: %w", f.Key, i, err)
				}
			}
		}
	}
	return nil
}
```

### The mutation-provable RED test (mandatory per this project's gate discipline)

```go
// TestFieldSetCoverage_CatchesWidening proves validateFieldSet CAN fail —
// a field with no placeholder is exactly the widening shape SC1 exists to
// make unconstructible; this test exercises the checker directly rather
// than through all 15 reports, so the mechanism's own capability to go RED
// is proven independent of any specific report's conversion state.
func TestFieldSetCoverage_CatchesWidening(t *testing.T) {
	fs := FieldSet{
		Text:   "count={count}",
		Fields: []Field{{Key: "count", Val: 3}, {Key: "secret", Val: "leaked"}},
	}
	if err := validateFieldSet(fs); err == nil {
		t.Fatal("validateFieldSet accepted a field set with an un-referenced field; the widening guarantee is not enforced")
	}
}
```

## State of the Art

Not applicable in the conventional sense (no external library version history to track) — this is a
first-time internal mechanism. The relevant "before/after" is entirely within this repository:

| Old Approach | Current (this phase's) Approach | When Changed | Impact |
|--------------|------------------------------------|---------------|--------|
| `renderOperator(cmd, format, text string, doc any)` — text and json coupled only by convention [VERIFIED: cmd/engram/operator_output.go:64] | `renderOperator(cmd, format, fs FieldSet)` — text and json both derived from one value | This phase | The core deliverable; SC1's "by construction" claim. |
| `TestOperatorOutputParity` — hand-listed `facts` strings, one-directional (text→json only) [VERIFIED: cmd/engram/operator_output_test.go:121-287,337-382] | Retired (D-07); replaced by `validateFieldSet` + a bidirectional registry test against `operatorCommands()` | This phase | Closes the direction the old test never covered (json cannot widen past text). |

**Deprecated/outdated:** `operatorParityRows()` and `jsonScalarValues`'s fact-containment-checking
approach — both retired per D-07, but the retirement must be the LAST step of the migration (see
Validation Architecture below), not a Wave 0 action.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The bracketed-span `[...]` syntax (vs. some other conditional-marker convention) is the right mechanism for D-06's "drop placeholder + surrounding literal together" requirement | Pattern 3 / Code Examples | LOW-MEDIUM — this is explicitly a Claude's-Discretion item per CONTEXT.md; if the planner picks a different syntax the coverage-checker/renderer pairing still needs the SAME one-shared-parser property, which is the load-bearing recommendation, not the specific bracket characters. |
| A2 | `purgeRerunCommand`'s embedding of `--category`/`--tags`/`--older-than` resolves the apparent D-03/D-04 conflict for those three fields (Pitfall 2) | Pitfall 2 | MEDIUM — if this resolution does not hold byte-for-byte on inspection, the planner needs a different answer (e.g. treating the whole re-run-command line as one opaque pre-rendered field, or escalating to the user as a genuine decision conflict) before implementation, not after. |
| A3 | `Field.Present` should default its true/false split to mirror EXISTING `omitempty` tags exactly, not "which mode applies" | Pitfall 1 | HIGH if wrong — this is the single highest-probability SC2 regression in the whole phase; every mode-dependent-pair report (`migrateRemapReportDoc`, `migrateOutputDoc`, `revertOutputDoc`) is affected. |
| A4 | The `[]FieldSet` join separator/style must be an explicit PER-FIELD property (not a single implicit constant), because `statusSummary`'s inline (non-newline) join genuinely differs from the six newline-joined reports | Pattern 2 | MEDIUM — if the planner instead hardcodes one join style, `migrate status`'s conversion will produce a byte-different sentence, a direct D-04 violation. |

**If this table is empty:** N/A — the table is populated; every entry above is genuine design
synthesis (no external precedent exists for this mechanism) rather than a verified fact, so the
planner should treat A1-A4 as items to confirm or explicitly re-decide during planning, not as settled.

## Open Questions

1. **Does SC2's "regression-free" extend to exact JSON byte/field-set identity, or only to the
   observable facts a consumer would parse out?**
   - What we know: D-04 explicitly pins TEXT to byte-identical, with no exception list. Nothing in
     CONTEXT.md makes the equivalent claim for JSON.
   - What's unclear: whether a report emitting FEWER top-level JSON keys than today (e.g. genuinely
     hiding an always-zero field under D-06) counts as a regression under SC2, even though no existing
     test would catch it (the retired `TestOperatorOutputParity` only checked scalar VALUE containment,
     never key-set equality; `TestOperatorOutputEmpty` only checks absence of the string `"null"`).
   - Recommendation: treat JSON key-set preservation as REQUIRED (Pitfall 1's guidance) — it is the
     conservative reading and costs nothing extra, whereas guessing "JSON shape may drift a little" and
     being wrong is a silent regression no current test catches.

2. **Is the D-03/D-04 tension for `purgeReportDoc`'s `Category`/`Tags`/`OlderThan` genuinely resolved
   by routing their placeholder through `purgeRerunCommand`'s existing text, or does it need a
   `checkpoint:human-verify` / explicit re-decision during planning?**
   - What we know: `purgeRerunCommand` already embeds these three values into the SAME rendered text
     block `purgePreviewSummary` produces.
   - What's unclear: whether making that embedding the field's canonical "text presence" for coverage
     purposes is a legitimate reading of D-03 (which says "referenced by a `{key}` placeholder in the
     text template") when the reference is indirect (inside a helper-produced string), or whether D-03
     was intended to mean a DIRECT placeholder for every field.
   - Recommendation: surface explicitly at plan time; do not silently resolve it inside a task's
     implementation notes.

3. **Should the coverage-checking registry (mapping `commandKey` → `FieldSet`-builder-invocation
   fixture) live as a NEW hand-authored table (like the retired `operatorParityRows()`, but without
   `facts` strings), or can it be derived some other way?**
   - What we know: there is no way to auto-discover "which Go function builds this command's report"
     without either a registry or (rejected) reflection-based struct-tag discovery.
   - What's unclear: whether the planner considers a hand-authored `map[string]func() FieldSet`
     registry, gated bidirectionally against `operatorCommands()`, an acceptable continuation of "no
     hand-listed facts" (the specific thing D-07 says was the problem) — the registry lists WHICH
     constructor to call, never a fact string, so it should be, but this is worth confirming explicitly
     given the project's stated history of "vacuous gate" recurrences (records tm0s0h3wgy, bqpfcnrnjs,
     fqznw5nc1g).
   - Recommendation: proceed with the registry-plus-bidirectional-set-equality design; it is the
     directly analogous, already-accepted pattern the retired test itself used for ITS row-set
     (CONTEXT.md explicitly calls that "worth carrying into whatever replaces it").

## Environment Availability

**Skipped.** This phase has no external tool, service, runtime, or CLI dependency beyond the Go
toolchain already required to build this repository (`go 1.26.3`, confirmed via `go.mod`). No new
package, database, or network dependency is introduced.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (`go test`) — no assertion library [VERIFIED: .planning/codebase/TESTING.md:9-14] |
| Config file | none — `go.mod` alone (`go 1.26.3`) |
| Quick run command | `go test ./cmd/engram/... -run TestFieldSet -v` (once the new test file exists) |
| Full suite command | `task test` (runs `task lint` then `test:go`+`test:python`) or `go test ./...` [VERIFIED: .planning/codebase/TESTING.md:16-27] |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-operator-renderer-typed (SC1: identity by construction) | `validateFieldSet` rejects an un-referenced field (widening made unconstructible) | unit | `go test ./cmd/engram/... -run TestFieldSetCoverage_CatchesWidening -v` | ❌ Wave 0 — new file `cmd/engram/fieldset_test.go` |
| REQ-operator-renderer-typed (SC1: universe is derived, not hand-listed) | Registry of report-constructors is bidirectionally set-equal to `operatorCommands()` | unit | `go test ./cmd/engram/... -run TestFieldSetRegistryCoversEveryOperatorCommand -v` | ❌ Wave 0 — new test, mirrors retired `TestOperatorOutputParity`'s row-set discipline [VERIFIED: cmd/engram/operator_output_test.go:344-364] |
| REQ-operator-renderer-typed (SC2: text byte-identical) | Every existing pinned-sentence assertion continues to pass unmodified | unit | `go test ./cmd/engram/... -run 'TestPrune|TestReindex|TestSummarize|TestSpineReview|TestMigrate|TestBackfill' -v` (existing per-report tests; exact `-run` pattern must be re-resolved against `go test -list` at plan time per this project's recorded false-green trap, record `bsbsvn4hbc`) | ✅ existing (per-file `_test.go` siblings already present for every report) |
| REQ-operator-renderer-typed (SC2: renderOperator's own I/O contract unchanged) | `TestRenderOperatorTextAndJSON`, `TestOperatorOutputEmpty`, `TestOperatorOutputStream` keep passing against the new signature | unit | `go test ./cmd/engram/... -run 'TestRenderOperatorTextAndJSON|TestOperatorOutputEmpty|TestOperatorOutputStream' -v` | ✅ existing [VERIFIED: cmd/engram/operator_output_test.go:84,406,444] |
| REQ-operator-renderer-typed (SC2: end-to-end behavioral parity) | `TestEveryOperatorCommandRejectsInvalidOutput`, `TestTimeoutGroupMatrix` keep passing (exercise `renderOperator`'s new signature transitively through every live command) | integration (drives the real cobra tree) | `go test ./cmd/engram/... -run 'TestEveryOperatorCommandRejectsInvalidOutput|TestTimeoutGroupMatrix' -v` | ✅ existing [VERIFIED: cmd/engram/operator_output_test.go:627,656] |
| REQ-operator-renderer-typed (SC3: one-declaration addition) | Adding a new field touches exactly one `FieldSet`-builder function | manual-only | N/A — this is a property about FUTURE ergonomics (Phase 7's six fields), not a runtime-checkable invariant within THIS phase | N/A |

### Sampling Rate

- **Per task commit:** the relevant per-report `_test.go` file's `-run` subset (e.g. converting
  `prune.go` → `go test ./cmd/engram/... -run TestPrune -v`), plus
  `TestFieldSetCoverage_CatchesWidening` once it exists (cheap, always run — it is the mechanism's own
  self-test).
- **Per wave merge:** `go test ./cmd/engram/... -v` (the whole package — this tier has no live-Qdrant
  dependency for its unit-level report builders per the existing "pure, no I/O" discipline
  [VERIFIED: CONTEXT.md code_context section: "the existing `xxxSummary(...) string` functions are
  already pure, no-I/O"]).
- **Phase gate:** `task test` green (lint + full suite) before `/gsd-verify-work`, plus the SC3
  manual-only verification recorded explicitly in VALIDATION.md with its justification (a property
  about future ergonomics cannot be a CI gate within this phase).

### Wave 0 Gaps

- [ ] `cmd/engram/fieldset.go` — the `FieldSet`/`Field` types, `validateFieldSet`, the shared
      placeholder parser, `FieldSet.MarshalJSON`, and the text-render walk. Zero regression risk to
      author first (nothing calls it yet).
- [ ] `cmd/engram/fieldset_test.go` — unit tests for the above, INCLUDING
      `TestFieldSetCoverage_CatchesWidening` (the mandatory red-proof mutation test) and a companion
      test proving a MISSING placeholder for a declared field is also caught (the symmetric direction).
- [ ] A new bidirectional registry test (e.g. `TestFieldSetRegistryCoversEveryOperatorCommand` in
      `operator_output_test.go` or a new file) — MUST NOT be authored to REPLACE
      `TestOperatorOutputParity` until every one of the 15 reports has converted (D-07's retirement is
      the LAST step, not a Wave 0 action — see Pitfall discussion under "Fate of the Existing Gate" in
      Summary).

*(Framework and per-report test files already exist; the only genuine Wave 0 gap is the new
mechanism's own test file, which the phase authors as its first deliverable.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | No new auth surface — `renderOperator`'s callers are already-authenticated/authorized operator commands; this phase changes only how their ALREADY-COMPUTED results are formatted. |
| V3 Session Management | No | Not touched. |
| V4 Access Control | No | Not touched — no new field, flag, or code path bypasses `internal/store`'s existing authorization gates; this phase is entirely downstream of authorization decisions already made. |
| V5 Input Validation | Marginal | The `{key}`/`[...]` placeholder templates are DEVELOPER-AUTHORED Go string literals (not external/user input) — there is no attacker-controlled input reaching the FieldSet parser at runtime. The "input validation" that matters here is the BUILD/TEST-TIME coverage check (`validateFieldSet`), which is a correctness gate, not a security boundary. |
| V6 Cryptography | No | Not touched. |

### Known Threat Patterns for this stack

None identified as applicable. This phase is an internal CLI output-rendering refactor with no new
network-reachable input, no new persisted data, no new authentication or authorization path, and no
new external dependency. The one property worth naming explicitly, even though it is not a
conventional STRIDE threat: a coverage-checker/renderer PARSING DIVERGENCE (Pitfall 4 above) is a
correctness bug that could theoretically let an operator-facing JSON document carry a value never
described in its accompanying text sentence — which is precisely the property SC1 exists to make
impossible. This is the phase's actual "security-adjacent" concern (information-disclosure-shaped, in
that a json consumer could see more than a text-reading operator believes is being reported), and it is
addressed entirely by the mutation-provable RED test in Validation Architecture, not by any ASVS
control.

## Sources

### Primary (HIGH confidence — read directly this session, file:line cited throughout)

- `cmd/engram/operator_output.go` — `renderOperator`, `addOperatorOutputFlag`, `operatorOutputFormat`
- `cmd/engram/operator_output_test.go` — `TestOperatorOutputParity`, `operatorParityRows`,
  `TestRenderOperatorTextAndJSON`, `TestOperatorOutputEmpty`, `TestOperatorOutputStream`,
  `TestEveryOperatorCommandRejectsInvalidOutput`, `TestTimeoutGroupMatrix`
- `cmd/engram/cmdwalk.go` — `operatorCommands`, `walkCommands`, `commandKey`, `operatorCommandExclusions`
- `cmd/engram/prune.go`, `reindex.go`, `summarize.go`, `migrate.go`, `migrate_family.go`,
  `spine_review_archive.go`, `spine_review_purge.go`, `spine_review_consolidate.go`,
  `spine_review_verify.go`, `spine_review_scan.go`, `backfill.go` — all 15 report builders and their
  doc/summary functions read in full
- `.planning/phases/06-typed-operator-renderer/06-CONTEXT.md`,
  `06-DISCUSSION-LOG.md` — locked decisions and rejected alternatives
- `.planning/REQUIREMENTS.md`, `.planning/STATE.md` — phase scope and milestone context
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/TESTING.md` — established project patterns
- `go.mod` — Go 1.26.3 confirmed

### Secondary (MEDIUM confidence)

None — no external documentation was consulted for this phase; there is no library to look up. All
non-codebase claims in this document are explicitly flagged `[ASSUMED]`/design-synthesis in the
Assumptions Log rather than presented as sourced fact.

### Tertiary (LOW confidence)

None.

## Metadata

**Confidence breakdown:**
- Current-surface baseline (the 15-row table, doc shapes, sentence variants): HIGH — every cell read
  directly this session with line citations.
- Proposed `FieldSet`/`Field` mechanism: MEDIUM — internally consistent with all 7 locked decisions and
  grounded in the actual sentence/doc shapes read, but is original synthesis with no external
  precedent; treat as a strong starting proposal for the planner, not a locked design.
- Pitfalls (JSON-shape drift, D-03/D-04 tension, `migrate status` exception): HIGH for the "what is true
  today" half (all line-cited), MEDIUM for "how to resolve it" (recommendations, not verified fixes).

**Research date:** 2026-08-16
**Valid until:** Until this phase's own PLAN.md/execution changes the files this research is based on
— the mechanism is entirely internal to a single milestone's single phase; there is no upstream
version-drift risk (no external library to go stale).
