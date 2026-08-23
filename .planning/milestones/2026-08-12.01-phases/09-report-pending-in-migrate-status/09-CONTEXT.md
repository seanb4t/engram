# Phase 9: Report pending in migrate status - Context

**Gathered:** 2026-08-22
**Status:** Ready for planning

<domain>
## Phase Boundary

`engram migrate status` — the offline operator verb — reports `pending`, the value milestone
`2026-08-12.01` declared canonical, and `docs-site/src/content/docs/guides/migrate.md` stops
describing a CLI derivation that does not exist.

This phase closes audit items **W2** and **W3** from `.planning/2026-08-12.01-MILESTONE-AUDIT.md`.
The audit's own finding is that these are one defect seen from two sides: the CLI does not compute
`pending`, and the docs claim it does. Closing W2 closes W3 as a side effect; closing W3 alone
would mean editing the doc to admit the gap rather than removing it.

This is **debt closure, not new scope**. `REQ-migrate-status-histogram` and `REQ-docs-record-state`
are both already satisfied and verified; nothing here re-opens them. No new runtime capability, no
new flag, no new RPC, no proto change, no MCP tool-schema change.

**In scope:**

- Adding a `pending` field to `migrateStatusReportDoc` (`cmd/engram/migrate_family.go:306-313`),
  populated from the single existing `store.MigrateStatusResult.Pending()` definition
  (`internal/store/migrate_status.go:76`).
- Adding a pending clause to `statusSummary` (`cmd/engram/migrate_family.go:348-358`), from the
  same method.
- Correcting the `pending` row of the `engram migrate status` / `engram migration-status` field
  table at `docs-site/src/content/docs/guides/migrate.md:279`.
- Extending the existing key-order gate (`TestMigrateFamilyStatusReportDocKeyOrder`,
  `cmd/engram/migrate_family_test.go:422`) and adding a re-derivation-proof test.

**Out of scope:**

- **W1** — no full-stack E2E for `engram migrate`. Parked as ROADMAP Phase 999.2 (BACKLOG).
- **W4** — CLAUDE.md's "every surface" record-state claim overstating the MCP lane. Parked as
  ROADMAP Phase 999.3 (BACKLOG).
- **Cross-cutting** — `schema_version` typed three ways in one proto file. Parked as ROADMAP
  Phase 999.4 (BACKLOG).
- **Pre-existing dprint drift** (4 files, unenforced by CI, not milestone-introduced) and the
  untracked phase-06 review artifacts. Neither is this phase's defect.
- **Any change to `MigrateStatusResult.Pending()` itself.** It is correct, tested
  (`internal/store/migrate_status_test.go:44,52`), and this phase adds a *consumer*, never a
  second definition.
- **Any change to `engram migration-status`** (`cmd/engram/client_migration_status.go`). Its text
  lane protojson-marshals the whole `MigrateStatusResponse` with `EmitDefaultValues`, so it already
  renders `pending` (field 7) in both `text` and `json`. It is not the surface W2 names.
- **Any change to the Connect RPC, the MCP startup advisory
  (`internal/server/tools.go:519`), or the client footer
  (`cmd/engram/client_common.go:376`).** All three already derive pending from `Pending()`.

</domain>

<decisions>
## Implementation Decisions

### Report struct shape

- **D-01:** `Pending uint64` with tag `json:"pending"` is **appended last**, after
  `CurrentVersion` — not inserted next to `Absent` where it would read more naturally.

  `migrateStatusReportDoc`'s own doc comment states the invariant it holds: *"Its first five
  fields reproduce `store.MigrateStatusResult`'s own keys, tags, and order exactly (buckets,
  absent, future, future_total, total) so the json lane is unchanged for every pre-existing key."*
  `CurrentVersion` was appended rather than interleaved for exactly this reason (06-01-PLAN.md
  §Conversion Rules R2). Appending `pending` keeps that sentence literally true and keeps every
  pre-existing json key at its pre-existing index for any consumer that reads positionally.

  Struct order is also *render* order: `viewFields` (`cmd/engram/operator_view.go:45`) walks the
  marshalled object's keys in document order, so appending puts `Pending` last in the text field
  table too. That is the right place for it — it is a value **derived from** the counts above it,
  and reading it after `absent`/`buckets` is how an operator checks the arithmetic.

  Rejected: inserting after `absent` for semantic grouping. It breaks the stated invariant, reorders
  the json document, and buys nothing the field table's label does not already convey.
  — **Reversibility:** costly — `pending` becomes a published key in the `--output json` contract of
  an operator verb; removing or renaming it later breaks any script that adopted it.

- **D-02:** The value is `res.Pending()`, called once in `statusReportDoc`. **Never** a CLI-side
  re-derivation, not even an inlined copy of the same three-line loop.

  This is the literal wording of the phase goal ("via the single existing
  `store.MigrateStatusResult.Pending()` definition ... never a re-derivation") and it is the
  method's stated purpose: its doc comment says it exists so that `warnPendingMigrations`, the CLI
  advisory footer, and the console banner *"all derive 'pending' from this one method instead of
  each recomputing the same loop — collapsing ... the half-applied N-site-invariant defect class
  this repo repeatedly hits."* This phase adds the fourth consumer. Adding a fifth *definition*
  would recreate precisely the defect the method closed.

  The arithmetic is non-obvious enough that a re-derivation would plausibly be wrong: `Pending()`
  is `Absent + sum(bucket.Count where bucket.Version < CurrentVersion)`, and **`Future` is
  deliberately excluded**. A naive `Absent + sum(all buckets)` or `Total - current-version bucket`
  both produce different numbers on any collection holding future records.
  — **Reversibility:** reversible — a single call site.

### Text headline

- **D-03:** `statusSummary` gains a pending clause, emitted **unconditionally** and positioned
  **after the per-bucket enumeration and before the future clause**.

  The phase goal names the text summary explicitly ("add `pending` to the CLI report struct **and**
  text summary"), so a struct-only fix does not close W2.

  *Unconditional*, because `total` and `absent` are already unconditional and `pending` is the
  value the milestone declared canonical — a clause that vanishes at zero makes "no pending clause"
  ambiguous between "zero pending" and "this binary predates the fix". Only the `future` clause is
  conditional, and that is because a future population is an anomaly rather than a routine reading.

  *Before the future clause*, because pending is derived from `absent` + the buckets the sentence
  has just enumerated, while `future` is the one population `Pending()` deliberately excludes.
  Ordering it `total → absent → buckets → pending → future` keeps every "behind this binary" fact
  contiguous and leaves the "ahead of this binary" fact last, where it cannot be misread as
  contributing to the number beside it.

  Wording is the planner's to choose within D-04's standing rule (`cmd/engram/migrate_family.go`
  §`statusSummary`): the headline is *additive prose over the field table*, `--output json` is the
  contract, and the sentence's wording may change in any release. It must not enumerate — the
  per-future-version enumeration was deliberately moved out of this sentence into the field table.
  — **Reversibility:** reversible — D-04 states this sentence is explicitly not a contract.

### Documentation correction

- **D-04:** The `pending` row at `guides/migrate.md:279` is rewritten to state that **all three
  surfaces carry the field**, sourced from one definition. It is not merely softened.

  The row currently reads: *"**Connect lane only** (`MigrateStatusResponse` field 7) — ... No CLI
  report struct carries this field; the CLI's own text and json output derive the equivalent number
  from `absent` and `buckets` directly."* Two claims, both false after D-01/D-03: "Connect lane
  only" becomes wrong, and the derivation claim was **never** true — `statusSummary` did none of
  that arithmetic and the operator did it by hand. That second half is W3 standing alone.

  The replacement states: `pending` is `absent` plus every bucket strictly below `current_version`,
  with `future` excluded; it is reported by `engram migrate status` (both `text` and `json`), by
  `engram migration-status`, and by the Connect `MigrateStatusResponse` (field 7); and every one of
  them reads the same server-side `MigrateStatusResult.Pending()`.

  The row keeps its place in the shared `engram migrate status` / `engram migration-status` table —
  which is now accurate for the first time, since the table's heading always covered both verbs
  while its `pending` row claimed only one had the field. The paragraph immediately below it (the
  protojson `uint64`-as-string note) remains true and is untouched: it describes an encoding
  difference, not a field-presence difference.
  — **Reversibility:** reversible — prose only.

### Acceptance gate shape

- **D-05:** The re-derivation prohibition (D-02) is proven **behaviourally with a discriminating
  fixture**, never by grepping for `Pending()`.

  A presence grep for `res.Pending()` is a claim, not evidence — durable record `v2rbxwg2r8`. The
  gate instead builds a `store.MigrateStatusResult` whose `Pending()` **differs from every
  plausible naive re-derivation**: a non-zero `Absent`, at least one bucket strictly below
  `CurrentVersion`, at least one bucket **at** `CurrentVersion`, and a non-empty `Future`. Then it
  asserts `statusReportDoc(res).Pending == res.Pending()`. Under that fixture,
  `Absent + sum(all buckets)`, `Absent + sum(buckets) + FutureTotal`, and
  `Total - currentVersionBucket` each yield a *different* number, so a copied-loop implementation
  goes RED instead of coincidentally passing.

- **D-06:** `TestMigrateFamilyStatusReportDocKeyOrder` (`cmd/engram/migrate_family_test.go:422`) is
  extended by appending `"pending"` to its `want` slice — it is the existing, correct home for this
  assertion and it is already ordered-set-shaped via `orderedKeyDiff`.

  **Verified at discuss time:** `orderedKeyDiff` (`cmd/engram/operator_view_test.go:73-91`) is
  already exact — it errors on length mismatch *and* on every positional difference, so appending
  `"pending"` to `want` makes the gate fail both if the key is missing and if it lands anywhere but
  last. No second parallel test is needed, and the test must NOT be relaxed to a subset check.

- **D-07:** The W3 doc gate asserts **zero** occurrences, anchored on an **inflection-free**
  substring, and ships with a positive control.

  This is a direct carry-forward of the audit's own recorded near-miss (durable record
  `6jj4dn31f8`): `rg 'derives the equivalent'` returned nothing and read as "W3 fixed", but the doc
  says *"derive the equivalent"* — plural subject, uninflected verb — and the sentence was intact.
  Anchor on `the equivalent number from`, which carries no verb and cannot inflect. Also assert
  zero occurrences of `Connect lane only` in that file. Pair both with a control that injects the
  string into a scratch copy and watches the gate fire RED; a gate observed only passing is not
  evidence.

- **D-08:** The `operatorViewFixtures()` `"migrate status"` entries
  (`cmd/engram/operator_view_migrate_test.go:59-66`) are re-checked, not assumed. They construct
  `store.MigrateStatusResult` values and pass them through `statusReportDoc`, so they pick up the
  new field automatically — but any test that compares rendered output against a literal expected
  block will need the new row. Locate those before editing, not after.

  `TestOperatorOutputEmpty` gains **no** `"migrate status"` entry: its own comment records that
  `statusReportDoc`'s null-safety is gated directly by
  `TestMigrateFamilyStatusReportDocNeverMarshalsNull` (REVIEWS.md C6-L4), and `uint64` cannot
  marshal to `null` regardless.

### Claude's Discretion

- Exact clause wording and punctuation in `statusSummary` (D-04's standing rule makes this
  explicitly non-contractual).
- Exact prose of the rewritten `pending` doc row, provided it states the arithmetic, names the
  `future` exclusion, and names all three reporting surfaces.
- Plan decomposition. The code change and the doc change are one logical unit and are small enough
  to be a single plan; splitting them is permitted but the doc gate (D-07) must not run before the
  code change lands, or it will pass against prose that is momentarily true and code that is not.
- Whether to also link the doc row to `reference/memory-record.md`'s `schema_version` section.

### Reviewed Todos (not folded)

- **`.planning/todos/pending/2026-08-10-research-versioned-payload-migration-mechanism.md`**
  (area `database`, match score 0.60). **Not folded — already retired by Phase 8.**

  `--auto` folds matches at score >= 0.40, but this match is keyword-driven (`phase`, `cmd`,
  `engram`, `migrate`, `internal`) and the todo is already locked as folded-and-retired in
  `.planning/phases/08-registry-docs-tail/08-CONTEXT.md` §`Folded Todos`, which records it as *"the
  origin of this milestone"*, answered by Phases 2–4 and closed by Phase 8's CLAUDE.md correction.
  Folding it a second time would re-open settled scope into a debt-closure phase.

  **Action for the planner or a follow-up:** the file is still sitting in `.planning/todos/pending/`
  despite Phase 8 retiring it. Moving it to `.planning/todos/done/` is a one-line housekeeping fix,
  but it is bookkeeping and not part of this phase's W2/W3 deliverable — do it only if it can be
  a separate commit.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### The audit items being closed

- `.planning/2026-08-12.01-MILESTONE-AUDIT.md:22` — W2 verbatim, naming
  `migrate_family.go:306-313` and `:348-358` as the two omission sites.
- `.planning/2026-08-12.01-MILESTONE-AUDIT.md:25` — W3 verbatim.
- `.planning/2026-08-12.01-MILESTONE-AUDIT.md:130-134` — the recorded near-miss false negatives,
  including the `derives`/`derive` inflection miss that D-07 exists to prevent recurring.
- `.planning/2026-08-12.01-MILESTONE-AUDIT.md:152-154` — the audit's own statement that W2 and W3
  are one defect and that closing W2 closes W3.
- `.planning/ROADMAP.md` §`### Phase 9: Report pending in migrate status` (~583) — goal, the two
  already-satisfied requirements, and the Phase 8 dependency.

### The single definition (never re-derive)

- `internal/store/migrate_status.go:64-84` §`MigrateStatusResult.Pending()` — **the authority.**
  Read the doc comment in full: it states the arithmetic, states that `Future` is deliberately
  excluded and why, and states the N-site-invariant defect class it was created to close.
- `internal/store/migrate_status.go:44-62` §`MigrateStatusResult` — the five source fields and the
  reason `Total` is a fresh exact count rather than a derived sum.
- `internal/store/migrate_status_test.go:44,52` — the existing `Pending()` unit assertions
  (populated → 8, zero-valued → 0).

### The edit surface

- `cmd/engram/migrate_family.go:291-313` §`migrateStatusReportDoc` — read the doc comment before
  editing; it states the field-order invariant D-01 preserves and why this struct is hand-declared
  rather than embedding `store.MigrateStatusResult`.
- `cmd/engram/migrate_family.go:315-337` §`statusReportDoc` — the converter; the nil-to-empty-slice
  normalization must survive the edit.
- `cmd/engram/migrate_family.go:339-358` §`statusSummary` — read the doc comment: D-04's rule that
  this sentence is additive prose, that `--output json` is the contract, and that enumeration
  belongs in the field table, not here.
- `docs-site/src/content/docs/guides/migrate.md:270-284` — the shared
  `engram migrate status` / `engram migration-status` field table; the `pending` row is line 279 and
  the protojson `uint64`-as-string paragraph immediately follows it.

### Existing consumers to leave alone (proof the single definition already works)

- `internal/server/connectapi.go:212` — `Pending: status.Pending()` on the Connect response.
- `internal/server/tools.go:500-525` §`warnPendingMigrations` — the startup advisory; its comment at
  `:510-512` records the H3 corrected predicate.
- `cmd/engram/client_common.go:363-383` §`migrationFooterCounts` — the client advisory footer,
  reading `resp.Msg.GetPending()` off the Connect response.
- `cmd/engram/client_migration_status.go:41-59` — why `engram migration-status` already reports
  `pending` in both lanes: protojson with `EmitDefaultValues`, rendered through the shared
  `renderOperatorView`.

### The rendering mechanism (why one struct field covers two lanes)

- `cmd/engram/operator_output.go:83-89` §`renderOperator` — text goes to `renderOperatorView`,
  json to `json.Encoder`; both from the same doc value.
- `cmd/engram/operator_view.go:45-80` §`viewFields` — marshals the doc and walks keys **in document
  order**, labelling via `humanizeKey`. This is why struct order is render order.

### Tests and gates that move

- `cmd/engram/migrate_family_test.go:420-438` §`TestMigrateFamilyStatusReportDocKeyOrder` — D-06's
  target. Check `orderedKeyDiff`'s exactness before extending `want`.
- `cmd/engram/migrate_family_test.go:400-419`
  §`TestMigrateFamilyStatusReportDocNeverMarshalsNull` — must keep passing unchanged.
- `cmd/engram/operator_view_migrate_test.go:57-66` — the `"migrate status"` view fixtures (D-08).
- `cmd/engram/operator_output_test.go:326-360` §`TestOperatorDocsAreHandDeclared` — the tier-wide
  gate that `migrateStatusReportDoc` stays a hand-declared local struct. A `uint64` field does not
  trip it; embedding the store type would.
- `cmd/engram/operator_output_test.go:495-525` §`TestOperatorOutputEmpty` — read the inline comment
  explaining why `"migrate status"` is deliberately absent from this map (D-08).

### Durable-memory hazards that apply directly to this phase's gates

- engram record `6jj4dn31f8` — the `derives`/`derive` inflection miss on *this exact sentence*, plus
  `cmd | tail; echo $?` reporting tail's status. Both were hit in the audit that produced W2/W3.
- engram record `v2rbxwg2r8` — a gate written to close a vacuous-gate finding is the highest-risk
  gate in the phase; self-test every gate against a constructed defect.
- engram record `6ey7knaz8v` — gate on **zero remaining occurrences**, never on "N converted".

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets

- **`MigrateStatusResult.Pending()` already exists and already has three production consumers.**
  This phase writes no arithmetic — it adds a fourth call site to a method whose entire stated
  purpose is being the only place that arithmetic lives.
- **The operator renderer is reflection-driven off json tags.** `renderOperator` →
  `renderOperatorView` → `viewFields` marshals the doc and walks the resulting object, so one
  struct field with a json tag lands in *both* `--output json` and the `text` field table. The
  label comes from `humanizeKey("pending")`. No second rendering edit is needed.
- **The only hand-written text surface is `statusSummary`'s single sentence** — which is why D-03
  is a separate decision from D-01 rather than falling out of it.
- **`migrationStatusCmd` is the proof the fix is correct.** `engram migration-status` protojson-
  marshals the whole response with `EmitDefaultValues` and renders it through the same
  `renderOperatorView`, so it has always shown `pending`. The operator-tier verb is the outlier,
  exactly as W2 says.

### Established Patterns

- **Hand-declared report docs, never embedded store types.** `TestOperatorDocsAreHandDeclared`
  enforces this tier-wide so record content is unreachable by construction (threat T-06-01). Add a
  scalar field; do not reach for embedding to "stay in sync".
- **Append, never interleave.** `CurrentVersion` was appended for the documented reason that the
  first five keys mirror the store type's own order. D-01 follows the same rule for the same reason.
- **The headline sentence is explicitly not a contract; `--output json` is.** D-04's rule, stated in
  `statusSummary`'s own doc comment.
- **`[]` never `null`** for slice-valued json fields (REVIEWS.md C5-L8). Not at risk here —
  `pending` is a `uint64` — but the normalization in `statusReportDoc` must survive the edit.

### Integration Points

- `statusReportDoc(res)` and `statusSummary(res)` are both called from one place:
  `migrateStatusCmd.RunE`'s `renderOperator(cmd, format, statusSummary(res), statusReportDoc(res))`
  (`cmd/engram/migrate_family.go:288`). Both edits converge on that single call.
- `docs-site` builds from `docs-site/src/content/docs/**`. The change is a table-row edit inside an
  existing page — no new page, no sidebar/nav registration, no anchored-region regeneration
  (`guides/migrate.md:279` is not inside an `engram:rule:*` anchor).

### Known hazards carried into this phase

- **`.planning/**` files must not receive an SPDX header** — GSD requires `---` frontmatter on line
  1, and a header above it makes a passed VERIFICATION.md read as `missing` (engram rule
  `2rjnv8sc9a`). `docs-site/**` is likewise excluded by `.licenserc.yaml`. Go files **do** need the
  header, but every file this phase touches already has one.
- **Do not shape an acceptance gate as `cmd 2>&1 | tail -N; echo $?`** — that reports `tail`'s exit
  status, not the command's, and passes unconditionally. Redirect to a file and test `$?`, or use
  `PIPESTATUS` (note: not portable to the user's `fish` shell — prefer the file redirect).
- **Agent-visible command output is lossy.** `=== RUN` lines are stripped from rendered Bash stdout
  (engram record `t4aq8704ss`). Never infer a toolchain behaviour from what appears in tool output;
  redirect to a file and read the file.
- **`.planning/codebase/TESTING.md` is stale.** It states *"No `testdata/` directories or golden
  files in the Go tree"*; `cmd/engram/testdata/{catalog,help}.golden` have existed since v0.13.x.
  Neither golden is affected by this phase (no flag or command-surface change), but do not treat
  that map as current.
- **No `ui/` changes are expected.** If any plan touches `ui/`, it must run `task ui:build` and
  commit `internal/webauth/static` — CI catches the drift only at PR time.

</code_context>

<specifics>
## Specific Ideas

- The discriminating fixture for D-05 must carry a bucket **strictly below** `migrate.CurrentVersion`
  — **corrected at research time; the earlier "at CurrentVersion" wording here was wrong.**
  `migrate.CurrentVersion` is `1` (`internal/migrate/migrate.go:54`), so the existing `migrate status`
  view fixture's `{Version: 1, Count: 40}` bucket is already *at* current, and a fixture without a
  below-current bucket never exercises `Pending()`'s bucket loop — a re-derivation of plain
  `pending = Absent` would pass it. See `09-RESEARCH.md` §Common Pitfalls 1 for the worked arithmetic
  of a corrected fixture and the three naive re-derivations it separates. Build version numbers as
  `cur := int(migrate.CurrentVersion)` and offsets from it, never as literals — the convention
  `internal/store/migrate_status_test.go:29-45` already follows.
- The rewritten doc row should state the `future` exclusion explicitly. It is the single most
  likely thing for an operator to get wrong when they check the arithmetic by hand against the
  `buckets`/`future` rows sitting directly above it.
- Keep the W3 correction to the `pending` row plus, if needed, the `future`/`future_total` rows'
  cross-reference. The rest of `guides/migrate.md` was verified accurate by the audit and should
  not be re-litigated in a debt-closure phase.

</specifics>

<deferred>
## Deferred Ideas

- **W1 — full-stack E2E for `engram migrate`** (CLI-layer↔store-layer join currently inferred).
  Already parked as ROADMAP Phase 999.2 (BACKLOG). This phase's D-05 test is a unit-level gate on
  the converter and does not close W1.
- **W4 — CLAUDE.md's "every surface" record-state claim.** The Memory contract section says every
  surface renders a record's derived state words; the MCP lane renders none. Already parked as
  ROADMAP Phase 999.3 (BACKLOG).
- **Unify `schema_version` proto typing** (`uint32` / `int32` / Go `int` in one proto file).
  Already parked as ROADMAP Phase 999.4 (BACKLOG).
- **dprint drift across 4 files, unenforced by CI** — neither `fmt:check` nor `dprint` appears in
  `.github/workflows/`, so it cannot block a PR. Pre-existing, not milestone-introduced. Worth its
  own issue: the interesting half is the *unenforced gate*, not the four files.
- **Move `2026-08-10-research-versioned-payload-migration-mechanism.md` from
  `.planning/todos/pending/` to `.planning/todos/done/`** — retired by Phase 8, never filed. See
  §`Reviewed Todos (not folded)`.

</deferred>

---

*Phase: 9-Report pending in migrate status*
*Context gathered: 2026-08-22*
