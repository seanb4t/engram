# Phase 6: Typed Operator Renderer - Context

**Gathered:** 2026-08-16 (re-discussed — supersedes the 2026-08-16 first pass)
**Status:** Ready for planning

> **This document replaces an earlier CONTEXT.md for the same phase.** The first pass locked a
> bespoke `{key}` / `[...]` template language as the mechanism (old D-01 through D-07). Four
> cross-AI review cycles surfaced that the template was the phase's largest new artifact and the
> source of most of its risk. Sean re-scoped the mechanism on 2026-08-16; the phase BOUNDARY is
> unchanged. Durable record: `6z129d6v3x`. The nine PLAN.md files on disk build the superseded
> design and must be replanned.

<domain>
## Phase Boundary

This phase replaces `renderOperator(cmd, format, text string, doc any)` — which takes the one-line
text sentence and the JSON document as two *unrelated* arguments — so that both `--output text` and
`--output json` derive from a single value.

The mechanism is **one serialization, plus a view**: the hand-declared report struct with its `json`
tags is the only serialization, and the text lane is a rendered view produced by walking that same
struct. There is no second format, no template language, and therefore nothing to keep in sync.

The deliverable is the mechanism plus the migration of every operator report to it. Today that is 15
operator commands, each carrying a bespoke `xxxSummary(...) string` (an `fmt.Sprintf` prose sentence)
and an independent `xxxDoc` struct, coupled only by convention.

**In scope:** the view renderer; conversion of all 15 operator reports; retirement of the superseded
hand-built parity row table.

**Out of scope:** the six new record-state fields themselves (2026-08-12.01 Phase 5/7 own those).
This phase exists so that adding them later touches exactly one struct per report. No new operator
command, no change to `--output` flag registration (`addOperatorOutputFlag`), no change to the client
tier's renderer.

</domain>

<decisions>
## Implementation Decisions

### Serialization Model

- **D-01:** There is **one serialization: JSON**, produced by `encoding/json` over the report's
  hand-declared struct. The text lane is a **rendered view** of that same struct — not a second
  format, not parseable, with no stability contract beyond "it shows every field".

  This supersedes the first pass's D-01 (ordered `[]Field` + text template), D-03 (every field
  referenced by a `{key}` placeholder) and D-04 (byte-identical sentences) together. YAML and TOON
  were both considered and rejected: each adds a dependency against the project's zero-new-Go-deps
  record, and — the deciding reason — each *looks* parseable, so someone parses it and the project
  owes escaping and stability semantics it never designed. Two machine-readable surfaces to keep in
  sync is the original text/json divergence defect wearing a hat.
  — **Reversibility:** costly — undo touches all 15 report renderers and every `renderOperator` call
  site, but nothing outside `cmd/engram` and no published wire contract.

### Enforcement Locus

- **D-02:** Identity holds **by construction, trivially**: nothing renders text except a walk over
  the same value `encoding/json` marshals. There is no second call site, therefore no coverage rule
  to enforce, no `validateFieldSet`, no placeholder parser, and no construction-time gate.

  A direct consequence, and a reversal of the first pass's D-02: **`doc any` becomes safe and stays.**
  The old design spent significant machinery making `doc any` impossible, because a free-form document
  could disagree with a free-form sentence. With one value feeding both lanes there is nothing for it
  to disagree with. `renderOperator` keeps a single document argument.
  — **Reversibility:** reversible — the signature is internal to `cmd/engram` (package `main`).

### Text Lane Status

- **D-03:** `--output text` is **explicitly not a stable interface**; `--output json` is the contract.
  This must be stated in the `--output` flag help and in the docs-site operator reference — it is what
  makes the text lane safe to evolve, and it is the mechanism by which the first pass's D-04
  (byte-identity) dissolves rather than being traded away. Sentences may change freely, forever.

  Consistent with the correct-by-reading principle (`4aksmneehh`): the interface states its own
  stability guarantee rather than leaving a caller to discover it by breakage.
  — **Reversibility:** one-way — once published as unstable and evolved, prior text output cannot be
  restored as a contract; any consumer that depended on it has already been told not to.

### The Headline

- **D-04:** Each report keeps **one hand-written prose headline line** above the field table, declared
  non-exhaustive (e.g. `spine purge applied: 4 deleted, 1 spared, 0 appeared`). It preserves the
  at-a-glance summary and the explanatory nuance that field names cannot carry — `1 spared (ineligible
  since preview)` teaches something `spared_count 1` does not.

  The headline is **additive prose over a complete table**: because the table below it renders every
  field unconditionally, the headline can add emphasis but can never hide a field. That is the
  property that makes a hand-written, ungated surface acceptable here — it is bounded to one line and
  is structurally incapable of causing the widening this phase exists to prevent.
  — **Reversibility:** reversible — dropping headlines later is a deletion.

### Field Labels

- **D-05:** Top-level fields render with a **humanized label** (`older_than` → `Older than`). Nested
  row fields render with **raw keys inline** (`id=01H8… outcome=ok`).

  This asymmetry is **deliberate, not an oversight** — top-level output is read, dense row output is
  scanned — and is recorded here explicitly so a later reader does not "fix" it into consistency.

- **D-06:** The identity gate **MUST NOT derive its expected labels by calling the same humanizer the
  renderer calls.** Both sides would then move together and a humanizer bug would be invisible — the
  precise shape of `01mdq5qq9j`, where a partition identity was invariant under the very mutation it
  appeared to guard.

  Decompose into two independent checks: (a) the identity gate asserts **one rendered line per JSON
  key** — correspondence and set equality, not label text; (b) a separate table-driven unit test pins
  the humanizer on fixed input→output pairs.
  — **Reversibility:** reversible — but getting it wrong reintroduces a vacuous gate, so it is locked.

### Nested Rendering

- **D-07:** The four two-level reports (`archive`, `purge`, `consolidate`, `verify`) render **one line
  per row with inline `key=value` fields**, indented under the field's label. Compact, closest to
  today's output, and legible for a long list.

  Accepted cost: this is a second rendering rule (nested values do not render like top-level ones),
  and very wide rows will wrap. The alternative — an indented sub-block per row — was rejected as too
  verbose for a 50-row purge.
  — **Reversibility:** reversible — local to the nested-value branch of the renderer.

### Testing

- **D-08:** Text output is pinned **structurally only — no text goldens.** The gate asserts the
  identity property (every JSON key has a corresponding rendered line) and nothing about exact
  formatting, so tweaking the renderer breaks nothing.

  This is the honest consequence of D-03: golden files on an explicitly-unstable lane would create the
  illusion of a contract and a maintenance surface for output nobody may depend on. JSON goldens are
  unaffected — that lane *is* the contract and should be pinned normally.
  — **Reversibility:** reversible — adding goldens later is additive.

### Fate of the Existing Gate

- **D-09:** `TestOperatorOutputParity` and its 15 hand-built `operatorParityRows()` are **retired** —
  now because they are obsolete rather than because a construction guarantee supersedes them. There is
  no longer a text/json divergence for a parity test to detect.

  Two facts about the retired test to record in the SUMMARY rather than lose (durable record
  `b3wd4wwwda`): its `facts` strings were hand-listed, which is the "test over hand-built rows"
  Success Criterion 1 explicitly rejects; and it was **one-directional** — it asserted every declared
  text fact appears in the JSON, never that the JSON fails to widen past the text.

  Its one genuinely good property is worth carrying forward: it gated its row set against
  `operatorCommands()` in **both directions**. Whatever enumerates reports for the new identity gate
  must keep that both-ways set equality, derived from the cobra tree and never hand-listed.
  — **Reversibility:** reversible — the deleted test is recoverable from git.

### Claude's Discretion

- Whether the view renderer reflects over the struct or dispatches through a small interface each doc
  implements. Reflection gives declaration-order field iteration for free and zero per-report work;
  an interface is explicit but is 15 implementations. Research and planning decide.
- The humanizer's exact rule (underscore→space plus leading capital is the obvious default) and how
  acronyms and initialisms are handled.
- Column alignment mechanics — `text/tabwriter` is the stdlib fit, but the choice is the planner's.
- How the headline is threaded: whether the existing 15 `xxxSummary` functions are trimmed down to
  headline producers or replaced outright.
- Migration order and batching across plans.
- Whether the report-enumeration gate needs an AST derivation or whether reflection over a registry
  suffices — the earlier design needed AST only because constructor routing had to be policed, which
  D-02 removes.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope and requirements

- `.planning/ROADMAP.md` §"Phase 6: Typed Operator Renderer" — goal, the three success criteria, and
  the dependency note (independent of other phases in this milestone; must complete before Phase 7).
  **Check the SC1/SC3 wording** — both say "field set", which still reads correctly against a struct,
  but confirm rather than assume.
- `.planning/REQUIREMENTS.md` — `REQ-operator-renderer-typed` (line 48), the sole requirement mapped
  to this phase; upstream issue #481.

### Prior-pass artifacts (read for evidence, NOT for design)

- `.planning/phases/06-typed-operator-renderer/06-RESEARCH.md` — the **codebase baseline is still
  valid and valuable**: the 15-report enumeration, doc shapes, sentence variants and `file:line`
  citations were verified directly and rate HIGH confidence. Its *proposed mechanism* (the
  `FieldSet`/`Field` template) is superseded by D-01 — ignore that half.
- `.planning/phases/06-typed-operator-renderer/06-PATTERNS.md` — the three-group shape classification
  (9 flat / 4 two-level / `migrate status` alone) still holds and still drives sequencing.
- `.planning/phases/06-typed-operator-renderer/06-REVIEWS.md` — four review cycles against the
  superseded design. Most findings die with the template, but two survive as live facts about the
  repo: the red-evidence harness accepts build failure as RED (`366pjeht8e`), and
  `backfill-short-ids` has an unregistered **preview** variant at `cmd/engram/backfill.go:36`.

### The code being replaced

- `cmd/engram/operator_output.go` — `renderOperator` (line 64), the two-argument rendering path this
  phase replaces; also `addOperatorOutputFlag` and `operatorOutputFormat`, which are NOT in scope and
  must keep working unchanged.
- `cmd/engram/operator_output_test.go` — `TestOperatorOutputParity` (line 347) and
  `operatorParityRows()` (line 138), retired by D-09; `TestRenderOperatorTextAndJSON` (line 84) pins
  the trailing-newline contract; `TestOperatorOutputEmpty` (line 411) pins the never-emit-bare-null
  contract; `TestOperatorOutputStream` (line 444) pins the write-to-cmd's-own-writer contract. The
  last three are behavioral contracts the new renderer must continue to satisfy.
- `cmd/engram/cmdwalk.go` — `operatorCommands()` (line 101), the derived set that defines which
  commands are in scope. Derive the migration work-list from this, never from a hand-written list.

### The 15 reports to convert

- `cmd/engram/prune.go` — `prunePreviewSummary`/`pruneSummary` + `pruneOutputDoc`. Note the doc
  comment at lines 127-132: `Eligible` and `Deleted` are deliberately separate fields so the JSON
  shape is stable across preview/applied. Preserve that.
- `cmd/engram/spine_review_archive.go` — `archiveSummary` (line 30) + `archiveReportDoc`/
  `archiveResultDoc`; the multi-line nested case (D-07).
- `cmd/engram/spine_review_purge.go` — `purgeReportDoc` (line 194) carries the most fields never
  stated in its applied sentence; the preview sentence prints notices and a `re-run:` line that the
  applied sentence lacks (`:285` vs `:294-309`).
- `cmd/engram/spine_review_consolidate.go`, `spine_review_verify.go`, `spine_review_scan.go` — the
  remaining spine-review leaves. `spineScanReportDoc` (line 82) has **no** `scope` key although
  `spineScanSummary` renders the scan target; the view will surface that gap.
- `cmd/engram/reindex.go`, `summarize.go`, `migrate.go`, `migrate_family.go`, `backfill.go` — the
  operator commands proper. `migrate status` is the one report passing `store.MigrateStatusResult`
  directly rather than a hand-declared doc struct.

### Project conventions

- `CLAUDE.md` §Conventions — CLI is cobra, config via `internal/config` (koanf), SPDX headers required
  on in-scope Go files (`task license:check`).
- `.planning/codebase/CONVENTIONS.md`, `.planning/codebase/TESTING.md` — established patterns the new
  mechanism should match.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- `operatorCommands()` (`cmd/engram/cmdwalk.go:101`) already derives the operator command set by
  walking the cobra tree with explicit exclusions, and the existing parity test gates its row set
  against it **both directions**. Carry that discipline forward (D-09).
- The existing `xxxSummary(...) string` functions are already **pure, no-I/O** and return a sentence
  with no trailing newline. That purity makes trimming them to headline producers mechanical.
- `text/tabwriter` (stdlib) is the idiomatic fit for aligned column output — no dependency needed.
- `pruneOutputDoc`'s separation of `Eligible` from `Deleted` is a deliberate stable-shape design, not
  an accident; the conversion must not collapse it.

### Established Patterns

- Every doc struct is **hand-declared**, never an embedded store result type (`archiveReportDoc`'s
  comment states this explicitly: "so this exclusion is enforced by the type itself"). This property
  becomes MORE important under D-01, because the struct is now the sole serialization — record content
  must remain unreachable by construction.
- JSON mode writes **exactly one document plus one trailing newline** via `json.Encoder.Encode`, to
  `cmd.OutOrStdout()` and never the process's real `os.Stdout`.
- `uint64` counters render as JSON numbers here (the CLI's own encoder, not protojson) — unlike the
  wire tier, where Phase 5's D-01 chose `uint32` because protojson renders `uint64` as a string.

### Integration Points

- Every `renderOperator(...)` call site in `cmd/engram/*.go` — 15 reports (19 call sites) across
  `reindex.go`, `prune.go` (2), `summarize.go`, `migrate.go` (2), `migrate_family.go` (3),
  `backfill.go`, and the six spine-review leaves.
- 2026-08-12.01 Phase 7 consumes the result: the six new record-state fields land in operator reports
  afterward, and Success Criterion 3 is precisely the promise that each one touches a single struct.

</code_context>

<specifics>
## Specific Ideas

- The worked example of the target text shape, from the discussion:

  ```
  spine purge applied: 4 deleted, 1 spared, 0 appeared

    Applied          true
    Classes          orphan
    Scope            repo:x
    Category         note
    Older than       720h
    Eligible count   5
    Deleted count    4
    Deleted
      id=01H8… outcome=ok
      id=01H9… outcome=skipped
  ```

  Headline is hand-written prose (D-04); top-level labels humanized (D-05); nested rows inline with
  raw keys (D-05, D-07); no colons, no quotes, no brackets — so it reads as a report and not as a
  document anyone should parse (D-01/D-03).

- `spineScanReportDoc` gaining a `scope` key is expected and welcome: the sentence already renders the
  scan target, and under one serialization the view surfaces the omission rather than hiding it.

- The `backfill-short-ids` **preview** variant (`cmd/engram/backfill.go:36`) was missed by the first
  pass's hand-registered variant list and found only by deriving the universe from source. Whatever
  enumerates reports must derive, not transcribe.

</specifics>

<deferred>
## Deferred Ideas

- **Applying the view mechanism to the client tier** (`engram search`/`list`/`get` renderers in
  `client_common.go`). Same class of problem, different tier, outside this phase's boundary. Worth
  considering after Phase 7 has exercised the mechanism.
- **Hardening the red-evidence harness** so a patch that merely breaks compilation cannot count as RED
  (`internal/store/redevidence_harness_test.go:303-316`; durable record `366pjeht8e`). Real and worth
  fixing, but it is a test-infrastructure defect this phase inherits rather than causes — it was
  in-scope only because the superseded design leaned on that harness for two patches. File it rather
  than smuggle it in.

### Reviewed Todos (not folded)

- **Research a versioned payload-migration mechanism**
  (`2026-08-10-research-versioned-payload-migration-mechanism.md`, matched at score 0.4) — not folded.
  The match is on weak generic keywords, and STATE.md already records this todo as scoped into
  2026-08-12.01 Phases 2–4, all complete. No bearing on operator output rendering.

</deferred>

---

*Phase: 6-Typed Operator Renderer*
*Context gathered: 2026-08-16 (re-discussed)*
