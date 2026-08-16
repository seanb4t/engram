# Phase 6: Typed Operator Renderer - Context

**Gathered:** 2026-08-16
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase replaces `renderOperator(cmd, format, text string, doc any)` — which takes the
one-line text sentence and the JSON document as two *unrelated* arguments — with a single
ordered field-set declaration per operator report, from which BOTH `--output text` and
`--output json` are derived.

The deliverable is the mechanism plus the migration of every operator report to it. Today
that is 15 operator commands, each carrying a bespoke `xxxSummary(...) string` (an
`fmt.Sprintf` prose sentence) and an independent `xxxDoc` struct, coupled only by
convention.

**In scope:** the field-set type and renderer; conversion of all 15 operator reports;
retirement of the superseded hand-built parity row table.

**Out of scope:** the six new record-state fields themselves (2026-08-12.01 Phase 5/7 own
those). This phase exists so that adding them later touches exactly one declaration per
report. No new operator command, no change to `--output` flag registration
(`addOperatorOutputFlag`), no change to the client tier's renderer.

</domain>

<decisions>
## Implementation Decisions

### Declaration Shape

- **D-01:** Each operator report is declared as an **ordered `[]Field` value** carrying a
  text template plus the fields, rather than reflecting over the existing `xxxDoc` struct
  tags or wrapping them in a generic `Report[T]`. Both lanes walk the same ordered slice:
  text renders the template, json is built from the fields in declaration order. No
  reflection, no template engine added to a CLI that has neither today. Accepted cost:
  each of the 15 reports is rewritten as a builder returning a field set, and the
  standalone `xxxDoc` structs lose their role as the JSON shape declaration.

  Illustrative shape (from the discussion, not a locked API):

  ```go
  func pruneReport(deleted uint64, before time.Time) FieldSet {
      return FieldSet{
          Text: "pruned ~{deleted} expired record(s) (not_after < {before}; {best_effort})",
          Fields: []Field{
              {Key: "preview", Val: false},
              {Key: "deleted", Val: deleted},
              {Key: "before", Val: before},
              {Key: "best_effort", Val: true},
          },
      }
  }
  ```

  — **Reversibility:** costly — undo touches all 15 report builders and every call site of
  `renderOperator`, but nothing outside `cmd/engram` and no published wire contract.

### Enforcement Locus

- **D-02:** The widening guarantee is enforced at **compile time**: `renderOperator` stops
  accepting a free-form `doc any`. The only way to call it is with a field set, so a JSON
  document carrying more state than its sentence states becomes *unconstructible* rather
  than merely detectable. This is the literal reading of the phase goal's "enforced by
  construction, not merely detected by test".

  A consequence to plan for deliberately: `doc any` is what lets the current 15 call sites
  pass heterogeneous structs. Removing it means every call site converts in the same
  change as the signature, or behind a temporary second entry point — the planner should
  pick one and say which; a long-lived two-signature period would reintroduce exactly the
  uncoupled path this phase deletes.
  — **Reversibility:** costly — the signature is internal to `cmd/engram` (package `main`),
  so undo is mechanical, but it is a 15-call-site change in both directions.

### Coverage Rule (what makes the identity claim non-vacuous)

- **D-03:** **Every field must be referenced by a `{key}` placeholder in the text
  template.** Prose coverage does not count, and there is no `Silent` escape hatch. A field
  present in the field set but absent from the template is a construction failure.

  This is the decision that keeps the guarantee real. The rejected alternative — "a
  placeholder OR a declared prose substring" — would have let boolean fields like
  `pruneOutputDoc.BestEffort` and `.Preview` stay covered by the words "best-effort count"
  alone, which is prose asserting a claim rather than a structure enforcing it. The
  `Silent` variant was rejected outright because every future field can take the escape
  hatch, which reopens the widening hole this phase exists to close.

  Direct consequence the planner must handle: fields whose current text presence is prose
  must become **value renderers that emit the same bytes**. `best_effort` renders as the
  literal `best-effort count` when true and as the empty string when false — the sentence
  is byte-identical, but the field is now structurally referenced.
  — **Reversibility:** reversible — loosening the rule later is additive; tightening it
  after the fact would not be, which is why it is locked strict now.

### Sentence Fidelity

- **D-04:** All 15 existing text sentences are preserved **byte-identical**, character for
  character. This is the strong reading of Success Criterion 2's "regression-free", and it
  keeps the existing pinned-sentence tests untouched so they act as an *independent*
  regression gate on this refactor rather than being rewritten alongside it. No named
  exception list — if a sentence cannot be reproduced, that is a finding about the
  mechanism, not a licence to normalize the prose.

  Note the sentences are genuinely bespoke and some are multi-line (`archiveSummary`
  emits a header line plus one line per row, joined by `\n` and right-trimmed). The field
  set must be expressive enough for that; it is not a `key: value` renderer.
  — **Reversibility:** reversible — the constraint is a choice about this refactor, not a
  structure that outlives it.

### Nesting

- **D-05:** A `Field`'s value **may itself be a `FieldSet` or `[]FieldSet`**, recursing the
  same coverage rule (D-03) one level down. The parent sentence covers the list through an
  aggregate field (a count); each element's own field set governs its own JSON object and
  its own rendered line.

  This is not hypothetical — four of the 15 reports are already two-level and already
  render per-row text today: `archiveReportDoc` (`[]archiveResultDoc`, rows rendered as
  `  requested=… id=… outcome=…`), the purge/restore pair, `consolidateReportDoc`
  (`[]consolidatePairDoc`), and `verifyReportDoc` (`[]verifyEntryDoc`). Flattening to
  aggregates-only was rejected because it changes the JSON shape of those four commands,
  breaking D-04. Treating the list as one opaque field was rejected because the guarantee
  would stop at the outer object and element fields could grow freely.
  — **Reversibility:** costly — the recursion shape is baked into the four two-level
  reports.

### Conditional Presence

- **D-06:** A conditionally-present field carries **one explicit `Present` predicate that
  both lanes read**. When absent, the field is omitted from the JSON object AND its
  placeholder together with its surrounding literal segment drops from the sentence — one
  decision, with no way for the two lanes to disagree.

  The live case is `archiveResultDoc.ID`: `json:"id,omitempty"` on the JSON side and a
  hand-written `if r.ID == ""` branch dropping `id=` on the text side. Both lanes already
  agree, but they agree via two independent conditionals — precisely the divergence this
  phase removes, one level down. Deriving presence from an optional template segment was
  rejected as putting the decision in the sentence rather than beside the value; always
  emitting both was rejected because it changes the JSON shape and the text line for
  failed-resolution rows, breaking D-04.

  ```go
  {Key: "id", Val: r.ID, Present: r.ID != "", Text: " id={id}"}
  ```
  — **Reversibility:** reversible — local to the field declaration.

### Fate of the Existing Gate

- **D-07:** `TestOperatorOutputParity` and its 15 hand-built `operatorParityRows()` are
  **retired**, not kept as a backstop and not rewritten as a derived both-ways gate. If
  field-set identity holds by construction (D-02 + D-03), the row table is dead weight
  that can rot — and hand-maintained evidence rotting undetected is a failure mode this
  milestone has already hit more than once in phases 2 through 4.

  Two facts about the retired test that the planner should record in the SUMMARY rather
  than lose: its `facts` strings were hand-listed, which is the "test over hand-built
  rows" that Success Criterion 1 explicitly rejects; and it was **one-directional** — it
  asserted every declared text fact appears in the JSON, and never that the JSON fails to
  widen past the text. The new mechanism must cover the direction the old test never did.
  — **Reversibility:** reversible — the deleted test is recoverable from git.

### Claude's Discretion

- The concrete Go API of `FieldSet` / `Field` (names, whether `Text` is a method or a
  struct field, how the `{key}` placeholder syntax is parsed and when parse failure is
  detected) is left to research and planning. D-01's snippet is illustrative.
- Whether placeholder-coverage failure surfaces at compile time, at `init()`, or at first
  render is a mechanism choice — but it must be reachable without executing every operator
  command against a live store.
- Multi-line and list-joining rendering mechanics for the four two-level reports.
- Migration order and batching across plans.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements

- `.planning/ROADMAP.md` §"Phase 6: Typed Operator Renderer" — goal, the three success
  criteria, and the dependency note (independent of other phases in this milestone; must
  complete before Phase 7).
- `.planning/REQUIREMENTS.md` — `REQ-operator-renderer-typed` (line 48), the sole
  requirement mapped to this phase; upstream issue #481.

### The code being replaced

- `cmd/engram/operator_output.go` — `renderOperator` (line 64), the two-argument rendering
  path this phase replaces; also `addOperatorOutputFlag` and `operatorOutputFormat`, which
  are NOT in scope and must keep working unchanged.
- `cmd/engram/operator_output_test.go` — `TestOperatorOutputParity` (line 347) and
  `operatorParityRows()` (line 138), retired by D-07; `TestRenderOperatorTextAndJSON`
  (line 84) pins the trailing-newline contract; `TestOperatorOutputEmpty` (line 411) pins
  the never-emit-bare-null contract; `TestOperatorOutputStream` (line 444) pins the
  write-to-cmd's-own-writer contract. The last three are behavioral contracts the new
  renderer must continue to satisfy.
- `cmd/engram/cmdwalk.go` — `operatorCommands()` (line 101), the derived set that defines
  which commands are in scope. Derive the migration work-list from this, never from a
  hand-written list.

### The 15 reports to convert

- `cmd/engram/prune.go` — `prunePreviewSummary`/`pruneSummary` + `pruneOutputDoc`; the
  clearest case of prose-covered boolean fields (D-03).
- `cmd/engram/spine_review_archive.go` — `archiveSummary` (line 30) + `archiveReportDoc`/
  `archiveResultDoc`; the multi-line, nested, conditionally-present case (D-04/D-05/D-06).
- `cmd/engram/spine_review_purge.go`, `spine_review_consolidate.go`,
  `spine_review_verify.go`, `spine_review_scan.go` — the remaining spine-review leaves.
- `cmd/engram/reindex.go`, `summarize.go`, `migrate.go`, `migrate_family.go` — the
  operator commands proper.

### Project conventions

- `CLAUDE.md` §Conventions — CLI is cobra, config via `internal/config` (koanf), SPDX
  headers required on in-scope Go files (`task license:check`).
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/TESTING.md` — established
  patterns the new mechanism should match.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `operatorCommands()` (`cmd/engram/cmdwalk.go:101`) already derives the operator command
  set by walking the cobra tree with explicit exclusions. The existing parity test gates
  its row set against this **both directions** — a new command without a row fails, and a
  row without a command fails. That both-ways set-equality discipline is worth carrying
  into whatever replaces it, even though the row table itself is retired.
- The existing `xxxSummary(...) string` functions are already **pure, no-I/O** and already
  return a sentence with no trailing newline (`renderOperator` appends it). That purity is
  what makes converting them to field-set builders mechanical rather than invasive.
- `pruneOutputDoc`'s existing design already separates `Eligible` from `Deleted` as
  distinct fields rather than one mode-dependent field — the field set should preserve
  that, not collapse it.

### Established Patterns

- Every doc struct is **hand-declared**, never an embedded store result type
  (`archiveReportDoc`'s comment states this explicitly: "so this exclusion is enforced by
  the type itself"). The field set must preserve that property — record content must
  remain unreachable from an operator report by construction, not by remembering to omit it.
- JSON mode writes **exactly one document plus one trailing newline** via
  `json.Encoder.Encode`, to `cmd.OutOrStdout()` and never the process's real `os.Stdout`.
- `uint64` counters render as JSON numbers here (this is the CLI's own encoder, not
  protojson) — unlike the wire tier, where Phase 5's D-01 had to choose `uint32` because
  protojson renders `uint64` as a string.

### Integration Points

- Every `renderOperator(...)` call site in `cmd/engram/*.go` — 15 reports across
  `reindex.go`, `prune.go` (2 sites), `summarize.go`, `migrate.go` (2), `migrate_family.go`
  (3), and the six spine-review leaves.
- 2026-08-12.01 Phase 7 consumes the result: the six new record-state fields land in
  operator reports afterward, and Success Criterion 3 is precisely the promise that each
  one touches a single declaration.

</code_context>

<specifics>
## Specific Ideas

- The `best_effort` renderer is the worked example of D-03: it must emit the literal
  string `best-effort count` when true and the empty string when false, so that
  `pruneSummary`'s sentence comes out byte-identical while the field becomes structurally
  referenced rather than merely described in prose.
- `archiveResultDoc.ID` is the worked example of D-06: `Present: r.ID != ""` drops both the
  JSON key and the ` id={id}` segment together.
- The retired parity test's blind spot is worth naming explicitly in the phase SUMMARY: it
  only ever checked text→json, never json→text. The new mechanism closes the direction the
  old gate never covered.

</specifics>

<deferred>
## Deferred Ideas

- **Applying the field-set mechanism to the client tier** (`engram search`/`list`/`get`
  renderers in `client_common.go`). Same class of problem, different tier, and not in this
  phase's boundary. Worth considering after Phase 7 has exercised the mechanism.

### Reviewed Todos (not folded)

- **Research a versioned payload-migration mechanism**
  (`2026-08-10-research-versioned-payload-migration-mechanism.md`, matched at score 0.4) —
  not folded. The match is on weak generic keywords ("phase", "there"), and STATE.md
  already records this todo as scoped into 2026-08-12.01 Phases 2–4 (schema versioning
  foundation, migration registry/sweep, migration CLI), all of which are complete. It has
  no bearing on operator output rendering.

</deferred>

---

*Phase: 6-Typed Operator Renderer*
*Context gathered: 2026-08-16*
