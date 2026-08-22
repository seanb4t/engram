# Phase 6: Typed Operator Renderer - Pattern Map

**Mapped:** 2026-08-16
**Files analyzed:** 18 (1 new mechanism file + 1 new test file + 15 report-conversion files + 1 modified core file)
**Analogs found:** 18 / 18 (this is an internal refactor — every file's best analog is either a sibling
report in the same tier or a same-shape registry/gate elsewhere in the repo)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `cmd/engram/fieldset.go` (NEW) | model/utility (self-validating registry+renderer type) | transform | `internal/surfaces/rules.go` (`ConditionalRule` + `ValidateRules`) | role-match, strongest of the two candidates |
| `cmd/engram/fieldset_test.go` (NEW) | test | transform | `internal/surfaces/conformance_test.go` (`TestZeroApplicableSurfacesFailsGate`) | role-match |
| `cmd/engram/operator_output.go` (MODIFIED — `renderOperator` signature) | utility/renderer | request-response | itself (pre-image), no external analog needed | exact (self) |
| `cmd/engram/operator_output_test.go` (MODIFIED — retire parity test, add registry gate) | test | request-response | `internal/server/connectapi_parity_test.go` (`assertDecodeBackCoversAllFields`) + `cmd/engram/cmdwalk.go` (`operatorCommands`) | exact for the both-directions idiom |
| `cmd/engram/prune.go` (2 reports: preview, apply) | CLI report builder | CRUD (flat) | Group A (flat single-sentence) — see below | exact (group self-analog) |
| `cmd/engram/summarize.go` (1 report) | CLI report builder | CRUD (flat) | Group A | exact |
| `cmd/engram/migrate.go` — `migrate-remap-owner` (1 report) | CLI report builder | CRUD (flat) | Group A | exact |
| `cmd/engram/migrate.go` — `migrate-set-owner` (1 report, inline-literal call site) | CLI report builder | CRUD (flat) | Group A, but see Pitfall 5 (own call-site shape) | role-match, one caveat |
| `cmd/engram/backfill.go` (delegates, no own report) | CLI report builder (thin alias) | CRUD (flat) | `migrate.go`'s sweep reports (shared function) | exact (literally shares the builder) |
| `cmd/engram/reindex.go` (1 report, 3 sentence variants) | CLI report builder | CRUD (flat, multi-variant) | Group A, closest single peer is `migrate_family.go`'s `migrate` (also 2-3 variants) | role-match |
| `cmd/engram/migrate_family.go` — `migrate` (2 variants) | CLI report builder | CRUD (flat, multi-variant) | Group A | exact |
| `cmd/engram/migrate_family.go` — `migrate status` | CLI report builder | CRUD (flat, but source type is `store.MigrateStatusResult` passed through, not hand-declared) | **Its own exception — no clean analog.** Closest structural peer for the join style is `spine_review_scan.go`'s `spineScanSummary` (inline, non-newline joins) — see Group B/Pitfall 3. | partial — treat separately |
| `cmd/engram/migrate_family.go` — `migrate revert` (3 shapes: refusal×2, preview, applied) | CLI report builder | CRUD (flat, multi-variant with a distinct refusal shape) | Group A, refusal sub-shape has no other analog in this tier | role-match |
| `cmd/engram/spine_review_scan.go` (1 report, 1 dynamic list) | CLI report builder | CRUD (flat + one trailing list) | Group A for the flat part; Group B (`archiveSummary`) for the list-append pattern | role-match |
| `cmd/engram/spine_review_archive.go` (archive + restore, shared builder) | CLI report builder | CRUD (nested, D-05/D-06) | Group B (nested reports) — this is the **canonical D-06 example** (`archiveResultDoc.ID`) | exact (canonical) |
| `cmd/engram/spine_review_purge.go` (preview + apply, 3 independent lists on apply) | CLI report builder | CRUD (nested, multiple independent lists) | Group B | exact |
| `cmd/engram/spine_review_consolidate.go` (1 report, 1 list + conditional line) | CLI report builder | CRUD (nested, D-06 replacement-not-drop) | Group B | exact |
| `cmd/engram/spine_review_verify.go` (1 report, 3 independent lists) | CLI report builder | CRUD (nested, multiple independent lists) | Group B, structurally closest to `purgeAppliedSummary`'s three-list shape | exact |

## Pattern Assignments

### Group A: Flat single-sentence reports (mechanical conversions)

**Members:** `prune.go` (`prunePreviewSummary`/`pruneSummary`), `summarize.go` (`summarizeSummary`),
`migrate.go` (`migrateRemapSummary`, `migrateSetOwnerSummary`), `reindex.go` (`reindexSummary`, 3
variants), `migrate_family.go` `migrate` (`migrateSummary`, 2 variants) and `migrate revert`
(`revertSummary`, 3 shapes incl. refusal).

**Shape:** one `xxxSummary(...) string` (pure `fmt.Sprintf`, no I/O) + one `xxxDoc` struct with plain
JSON tags, called together at exactly one (or two-per-mode) `renderOperator` site. These are their own
best analogs for each other — convert `prune.go` first as the worked D-03 example (RESEARCH.md already
supplies the full before/after), then treat every other Group A member as a repetition of the same
shape.

**Current shape** [VERIFIED: cmd/engram/reindex.go:87-129]:
```go
text := reindexSummary(res, reindexTarget, dim, reindexDryRun, reindexResume)
return renderOperator(cmd, format, text, reindexReportDoc(res, reindexTarget, dim, reindexDryRun, reindexResume))

func reindexSummary(res store.ReindexResult, target string, dim uint64, dryRun, resume bool) string {
    if dryRun {
        if resume { return fmt.Sprintf("dry-run --resume: %d would be re-embedded, ...", ...) }
        return fmt.Sprintf("dry-run: %d record(s) would be re-embedded into %q at dim %d", ...)
    }
    return fmt.Sprintf("re-embedded %d/%d record(s) into %q at dim %d ...", ...)
}
type reindexOutputDoc struct {
    DryRun bool `json:"dry_run"`
    Resume bool `json:"resume"`
    ...
}
```

**Conversion target** — one `xxxReport(...) FieldSet` per sentence VARIANT (not one struct serving all
variants — see Anti-Pattern below), replacing both the summary function and the doc struct. See
RESEARCH.md "Worked conversion" (prune.go) for the full byte-level example; the same shape applies to
every Group A member, just with a different variant count (1 for `summarize`/`migrateRemapSummary`, 3
for `reindexSummary`/`revertSummary`).

**`migrate-set-owner`'s caveat (Pitfall 5):** unlike every other Group A member, its doc is built as an
inline struct literal at the call site, not via a converter function:
```go
renderOperator(cmd, format, migrateSetOwnerSummary(migrateOwner, n), migrateSetOwnerReportDoc{Owner: migrateOwner, Stamped: n})
```
[VERIFIED: cmd/engram/migrate.go:64]. Its `FieldSet` builder must be a new named function like its
siblings — do not just inline-literal a `FieldSet{}` at the call site, since that reintroduces the
"skipped by a converter-function-name grep" hazard the planner should enumerate from the 19
`renderOperator` call sites, never from an `xxxDoc(...)` grep.

**`backfill.go`'s caveat:** it has NO own report — it delegates to `migrateSweepPreviewRun`/
`migrateSweepApplyRun`, sharing `migrate`'s report builder outright [VERIFIED: cmd/engram/backfill.go
doc comment "thin delegating alias"]. No separate FieldSet builder needed; converting `migrate`'s
builder converts `backfill-short-ids` for free.

---

### Group B: Nested (two-level) reports — D-05/D-06 exercised, four distinct join shapes

**Members:** `spine_review_archive.go` (`archiveSummary`, archive+restore share one builder),
`spine_review_purge.go` (`purgePreviewSummary`/`purgeAppliedSummary`), `spine_review_consolidate.go`
(`consolidateSummary`), `spine_review_verify.go` (`verifySummary`).

**These four are their own best analogs and NO single "list field" abstraction fits all four** — each
needs its own explicit join configuration. RESEARCH.md's Pattern 2 table is the authoritative
per-report join-shape ledger (row source, separator, row template, notes); reproduce it here as the
load-bearing fact for planning:

| Report | Rows source | Join | Notes |
|--------|-------------|------|-------|
| `archiveSummary` | one `[]store.ArchiveResult` | `"\n"` + `TrimRight` | canonical D-06 case: `id=` segment drops when `ID==""` |
| `purgePreviewSummary` | one `[]string` (eligible ids) | `"\n"` | list sandwiched between header + two FIXED notice-sentence constants + a re-run command line (5 template segments in one report, not just header+list) |
| `purgeAppliedSummary` | THREE separate `[]string` (deleted/spared/appeared) | `"\n"` per block | 3 independent nested lists, not one |
| `consolidateSummary` | one `[]store.DuplicatePair` | `"\n"` | preceded by a D-06 REPLACEMENT-not-drop conditional line (`min_score=%g` vs `min_score=(none...)`) |
| `verifySummary` | THREE separate `[]verifyEntry` (moved/broken/unverifiable) | `"\n"` per block | same three-independent-lists shape as purge-applied |
| `statusSummary` (migrate status — Group C, listed for contrast) | TWO lists | **inline, comma-joined on the SAME line**, not newline | the one non-newline join in the whole tier |

**Canonical D-06 excerpt** (archive) [VERIFIED: cmd/engram/spine_review_archive.go:36-40]:
```go
if r.ID == "" {
    ...  // "  requested=%s outcome=%s\n"
} else {
    ...  // "  requested=%s id=%s outcome=%s\n"
}
```
Convert this to `{Key: "id", Val: r.ID, Present: r.ID != "", Text: " id={id}"}` (D-06's span-drop form,
CONTEXT.md line 145) — the one field in the whole tier where the bracketed conditional-span mechanism is
strictly required rather than a `Render`-function replacement.

**REPLACEMENT-not-drop excerpt** (consolidate, contrast with archive) [VERIFIED:
cmd/engram/spine_review_consolidate.go:198-201]:
```go
if minScore != nil {
    ...  // "  min_score=%g\n"
} else {
    ...  // "  min_score=(none — no filter applied; negative-scoring pairs are reported)\n"
}
```
This is `Present`-independent from the TEXT side (something always renders) but `omitempty` on the JSON
side — a `Render` function returning the alternate-prose string, not a span drop. Do not conflate this
with the archive `ID` case; RESEARCH.md's Pattern 3 table is the full inventory distinguishing the two
shapes across all `omitempty` fields in the tier.

---

### Group C: `migrate status` — the exception, no clean analog (Pitfall 3)

**Current shape** [VERIFIED: cmd/engram/migrate_family.go:282-296]: `statusReportDoc` passes
`store.MigrateStatusResult` straight through — the ONLY report in the tier reusing a `store`-layer
type's own JSON tags directly rather than a hand-declared CLI-only struct (comment: "no separate
CLI-side type exists"). D-02 forbids this shortcut going forward — `renderOperator`'s only accepted
argument becomes `FieldSet`, so this report needs its own new hand-declared `FieldSet` builder like
every other report, losing the passthrough.

**Additional distinguishing features, all three present ONLY here:**
- D-05 nesting **twice** (`Buckets` always, `Future` conditionally) [VERIFIED:
  cmd/engram/migrate_family.go:303-316]
- The tier's ONE inline (non-newline, comma/space-joined on the same line) join style — everything
  else in Group B joins with `"\n"`
- `Future`'s entire clause (prefix text + every `v%d=%d` element) is itself D-06-conditional on
  `len(res.Future) > 0`

**No in-repo report shares this combination.** Do not batch this conversion alongside Group A's "simple"
reports (Pitfall 3's explicit warning) — budget it as its own task.

---

### `cmd/engram/fieldset.go` (NEW) — self-validating registry/descriptor type

**Analog 1 (primary): `internal/surfaces/rules.go`** — `ConditionalRule` + package-level `rules []ConditionalRule`
+ `ValidateRules()`/`validateRuleSet()`.

**Why this is the closer analog over `internal/config`'s field registry:** `ConditionalRule` is a
small, hand-declared, self-describing VALUE type (not a struct-tag-driven config binding) whose
correctness is proven by a completeness/uniqueness validator function walking a declared slice — the
exact shape `FieldSet`/`Field` needs (an ordered `[]Field`, each carrying a `Key` that must be
referenced elsewhere, validated by `validateFieldSet` walking the slice). `internal/config`'s registry
is env-var/flag binding metadata (koanf-driven), a different problem shape.

**Declaration + doc-comment convention to copy** [VERIFIED: internal/surfaces/rules.go:30-88]:
```go
type ConditionalRule struct {
    ID       string   // stable identifier, doubles as registry key
    Sentence string   // canonical statement, ASCII-only (enforced by validator)
    Fields   []string // ordered field/argument names this rule constrains
    ...
    declared bool // provenance marker — unexported so it cannot be forged
                  // by a composite literal written outside this package
}
```
`FieldSet`/`Field` should NOT necessarily copy the unforgeable-`declared`-bool trick (that defends
against off-registry construction, which is not FieldSet's risk — FieldSet's risk is placeholder/field
mismatch, not provenance) but SHOULD copy:
1. The **declared-slice → built-once-derived-map → exported-lookup-function** shape [VERIFIED:
   internal/surfaces/rules.go:271-291]:
   ```go
   var ruleByID = func() map[string]ConditionalRule {
       m := make(map[string]ConditionalRule, len(rules))
       for _, r := range rules { m[r.ID] = r }
       return m
   }()
   func RuleByID(id string) (ConditionalRule, bool) { r, ok := ruleByID[id]; return r, ok }
   ```
2. The **exported validator with a separated testable-over-arbitrary-input core**:
   `ValidateRules()` (calls the real registry) vs. `validateRuleSet(set []ConditionalRule) error` (takes
   any slice) [VERIFIED: internal/surfaces/rules.go:300-307] — apply the same split to
   `validateFieldSet(fs FieldSet) error`, so `fieldset_test.go` can construct throwaway `FieldSet` values
   without touching any of the 15 real report builders.
3. **Named, descriptive error construction** naming the offending ID/key, never a bare bool
   [VERIFIED: internal/surfaces/rules.go:310-335] — `fmt.Errorf("surfaces: rule %q has empty Fields",
   r.ID)` pattern → `fmt.Errorf("field-set coverage mismatch: %v", diff)` per RESEARCH.md's own sketch.

**Analog 2 (secondary, for the "how is completeness proven non-vacuously" half):
`internal/surfaces/conformance_test.go`'s `TestZeroApplicableSurfacesFailsGate`** — a SYNTHETIC
ghost value proving the real gate function (`assertRuleAppliesSomewhere`) actually fails when the
guarantee is violated, exercising the production code path directly rather than a parallel copy
[VERIFIED: internal/surfaces/conformance_test.go:267-278]:
```go
func TestZeroApplicableSurfacesFailsGate(t *testing.T) {
    ghostRule := ConditionalRule{ID: "test-ghost-zero-surfaces", Fields: []string{"ghost_field_zzz"}, ...}
    applicable := ApplicableSurfaces(ghostRule, map[Surface][]string{})
    if err := assertRuleAppliesSomewhere(ghostRule, applicable); err == nil {
        t.Fatal("... a rule applying nowhere must fail the gate, not pass")
    }
}
```
This is the direct template for RESEARCH.md's own proposed
`TestFieldSetCoverage_CatchesWidening` (mandatory mutation/red-proof test) — same shape, one field with
no placeholder instead of one field applying nowhere.

---

### `cmd/engram/operator_output_test.go` — retiring `TestOperatorOutputParity`, adding the bidirectional registry gate (D-07)

**Analog 1 (primary, in-file): `TestOperatorOutputParity` itself** — the exact both-directions
set-equality idiom to imitate for whatever replaces it, applied to `operatorCommands()`'s derived set
rather than a hand-listed table [VERIFIED: cmd/engram/operator_output_test.go:347-364]:
```go
func TestOperatorOutputParity(t *testing.T) {
    rows := operatorParityRows()
    rowNames := make(map[string]bool, len(rows))
    for _, r := range rows { rowNames[r.name] = true }
    wantNames := commandKeySet(operatorCommands())
    for name := range wantNames {
        if !rowNames[name] { t.Errorf("... missing a row for operator command %q", name) }
    }
    for name := range rowNames {
        if !wantNames[name] { t.Errorf("... has a row for %q, which is not in operatorCommands()", name) }
    }
    ...
}
```
The retiring test's documented BLIND SPOT (worth restating in the phase SUMMARY per CONTEXT.md's
"Specific Ideas"): it only ever checked `strings.Contains(row.text, fact)` AND
`containsString(values, fact)` — text→json containment, never proving json does NOT carry MORE than
text states. The new registry gate must close this direction, which is exactly what a
`validateFieldSet` walking `fs.Fields` symmetric-difference against parsed placeholders does (set
equality, not one-directional containment).

**Analog 2: `cmd/engram/cmdwalk.go`'s `operatorCommands()`** — the derived (never hand-listed) work-list
source [VERIFIED: cmd/engram/cmdwalk.go:101-116]. Both the retiring test's row-set AND the new
registry's coverage set must derive their command universe from this function, never a fresh hand-typed
list — this is the single mechanical anchor tying "which 15/19 call sites must convert" to a live,
self-updating source.

**Analog 3 (set-equality-by-symmetric-difference, not just presence/absence): `internal/server/connectapi_parity_test.go`'s `assertDecodeBackCoversAllFields`** — proves a compared-field
list is BOTH duplicate-free AND exactly equal (via `slices.Sort` + `slices.Equal`) to a reflection-derived
"what fields actually exist" universe, rather than a hand-typed table [VERIFIED:
internal/server/connectapi_parity_test.go:344-361]:
```go
func assertDecodeBackCoversAllFields(t *testing.T, compared []string) {
    t.Helper()
    got := slices.Clone(compared)
    slices.Sort(got)
    for i := 1; i < len(got); i++ {
        if got[i] == got[i-1] {
            t.Fatalf("assertDecodeBackCoversAllFields: %q was compared more than once", got[i])
        }
    }
    want := storeJSONVisibleFields(reflect.TypeOf(store.Memory{}))
    slices.Sort(want)
    if !slices.Equal(got, want) {
        t.Fatalf("decode-back comparator coverage mismatch: compared %v, want %v", got, want)
    }
}
```
This is the strongest available in-repo precedent for `validateFieldSet`'s own symmetric-difference
check (`declared` field-key set vs. `referenced` placeholder-key set) — copy the "dedupe-then-sort-then-
`slices.Equal`, name every mismatch" discipline, not the reflection-over-a-struct mechanism (D-01
explicitly forbids `reflect` inside `fieldset.go` itself; this analog's `reflect` usage is scoped to
the OTHER tier's test file and is not something `fieldset.go` or `fieldset_test.go` should imitate for
the render/validate path — only the assertion SHAPE transfers).

**No `internal/store/redevidence_harness_test.go` analog needed:** that harness's `TestRedEvidencePatchesAreLive`
proves an evidence-gate mechanism goes RED under a live source-mutation probe, a different concern
(source-code-mutation-detection, not field/placeholder set equality) — not a load-bearing precedent
for this phase; superseded by the `TestZeroApplicableSurfacesFailsGate` / mutation-provable-RED template
already covered above.

## Shared Patterns

### `renderOperator`'s I/O contract — unchanged, still the single rendering boundary

**Source:** `cmd/engram/operator_output.go:59-71`
**Apply to:** every converted report's call site (all 19)
```go
func renderOperator(cmd *cobra.Command, format outputFormat, fs FieldSet) error {
    if format == formatText {
        _, err := fmt.Fprintln(cmd.OutOrStdout(), fs.renderText())
        return err
    }
    enc := json.NewEncoder(cmd.OutOrStdout())
    return enc.Encode(fs)
}
```
Only the parameter list changes (`text string, doc any` → `fs FieldSet`, per D-02/RESEARCH.md's Code
Examples section); `fmt.Fprintln`'s trailing-newline behavior and `json.NewEncoder(...).Encode`'s
single-document-plus-newline behavior are NOT in scope to change — `TestRenderOperatorTextAndJSON`,
`TestOperatorOutputEmpty`, and `TestOperatorOutputStream` (cmd/engram/operator_output_test.go:84-119,
411-473) pin these independently of the doc/text coupling and must keep passing unmodified.

### `addOperatorOutputFlag`/`operatorOutputFormat` — explicitly out of scope

**Source:** `cmd/engram/operator_output.go:24-57`
**Apply to:** nothing in this phase — CONTEXT.md's Phase Boundary explicitly excludes any change to
`--output` flag registration. Listed here only so no plan accidentally touches it while editing the
adjacent `renderOperator` signature in the same file.

### Ordered-JSON-without-a-map — `FieldSet.MarshalJSON` must walk `[]Field` directly

**No in-repo analog exists for a custom ordered `MarshalJSON`** (RESEARCH.md's Don't Hand-Roll table
flags this as the one piece of genuinely novel mechanism in the phase: `map[string]any` sorts keys
alphabetically on marshal, silently reordering every document). Reuse `json.Marshal(value)` PER SCALAR
field (never a hand-rolled string-escaping routine) and hand-write only the structural bytes (`{`, `:`,
`,`, `}`) — this keeps `TestOperatorOutputEncoding`'s UTF-8/quote-escaping fidelity guarantee
[VERIFIED: cmd/engram/operator_output_test.go:389-404] intact without re-deriving `encoding/json`'s
escaping rules by hand.

## No Analog Found

| File/Concern | Role | Data Flow | Reason |
|---|---|---|---|
| `FieldSet.MarshalJSON`'s ordered-object-from-slice encoder | utility | transform | No existing type in the repo hand-writes ordered JSON structural bytes around per-field `json.Marshal` calls — this piece is genuine original synthesis (RESEARCH.md's own verdict); build it directly against `TestOperatorOutputEncoding`'s existing fixture as the regression check. |
| The `{key}`/`[...]`-span text-template parser (`parseFieldSetText`) | utility | transform | No template-engine or placeholder-parsing precedent exists anywhere in this stdlib-only codebase (D-01 explicitly forbids introducing `text/template`). RESEARCH.md's own Code Examples section is the only available sketch; treat it as a starting point, not a verified pattern. |
| `migrate status`'s conversion (Group C) | CLI report builder | CRUD (nested×2 + inline join) | No sibling report combines double-nesting, D-06 conditional presence, AND a non-newline join — see Group C above. |

## Metadata

**Analog search scope:** `cmd/engram/*.go` (all 15 report files + `operator_output.go`/`_test.go` +
`cmdwalk.go`), `internal/surfaces/*.go` (rules + conformance test), `internal/config/config.go` +
`service_auth_test.go`, `internal/server/connectapi_parity_test.go`, `internal/store/redevidence_harness_test.go`
**Files scanned:** ~24 (18 read/grepped this session; 6 more located and ruled out via targeted grep)
**Pattern extraction date:** 2026-08-16
