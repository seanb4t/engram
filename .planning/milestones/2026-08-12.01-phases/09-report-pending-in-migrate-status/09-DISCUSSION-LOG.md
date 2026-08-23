# Phase 9: Report pending in migrate status - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-22
**Phase:** 9-report-pending-in-migrate-status
**Areas discussed:** Report struct shape, Text headline, Documentation correction, Acceptance gate shape, Scope boundary
**Mode:** `--all --auto` — all gray areas auto-selected; `--auto` resolved every question to the
recommended option without prompting (per `modes/all.md` §Combination rules: `--auto` wins for the
discussion phase, `--all`'s contribution is area auto-selection).

`[--auto] Selected all gray areas: Report struct shape, Text headline, Documentation correction, Acceptance gate shape, Scope boundary.`

---

## Report struct shape

`[auto] Report struct shape — Q: "Where does the new pending field sit in migrateStatusReportDoc?" → Selected: "Append last, after current_version" (recommended default)`

| Option | Description | Selected |
|--------|-------------|----------|
| Append last, after `current_version` | Preserves the struct's documented invariant that its first five fields mirror `store.MigrateStatusResult`'s keys/tags/order exactly; keeps every pre-existing json key at its pre-existing index; renders last in the text field table, after the counts it derives from | ✓ |
| Insert after `absent` | Reads more naturally as a semantic grouping (absent → pending) | |
| Insert first | Treats pending as the headline value | |

**Choice rationale:** the struct's own doc comment states the invariant, and `CurrentVersion` was
appended rather than interleaved for exactly this reason (06-01-PLAN.md §R2). Struct order is also
render order — `viewFields` walks the marshalled object in document order — so appending is a single
decision governing both lanes.

**Notes:** rated **costly** to reverse: `pending` becomes a published key in an operator verb's
`--output json` contract.

---

## Text headline

`[auto] Text headline — Q: "Does statusSummary state pending, and where in the sentence?" → Selected: "Yes, unconditionally, after buckets and before the future clause" (recommended default)`

| Option | Description | Selected |
|--------|-------------|----------|
| Unconditional, after buckets, before future | Matches `total`/`absent`'s unconditional treatment; keeps every "behind this binary" fact contiguous and leaves the excluded-from-pending `future` clause last | ✓ |
| Conditional on `pending > 0` | Mirrors the `future` clause's conditional treatment | |
| Struct field only, no headline change | Minimal diff | |

**Choice rationale:** the phase goal names the text summary explicitly, so a struct-only fix does
not close W2. A clause that vanishes at zero makes "no pending clause" ambiguous between zero-pending
and a pre-fix binary. Placing pending before `future` prevents the misreading that future records
contribute to it — they are deliberately excluded.

**Notes:** exact wording left to the planner. `statusSummary`'s own doc comment (D-04) states the
sentence is additive prose whose wording may change in any release, and that enumeration belongs in
the field table.

---

## Documentation correction

`[auto] Documentation correction — Q: "How is the pending row at guides/migrate.md:279 rewritten?" → Selected: "Rewrite to state all three surfaces carry it, from one definition" (recommended default)`

| Option | Description | Selected |
|--------|-------------|----------|
| Rewrite: all three surfaces, one definition | States the arithmetic (`absent` + buckets below `current_version`, `future` excluded), names `engram migrate status` (text + json), `engram migration-status`, and Connect `MigrateStatusResponse` field 7, all reading the same `MigrateStatusResult.Pending()` | ✓ |
| Delete the false derivation sentence only | Removes W3's literal false claim, leaves "Connect lane only" — which D-01/D-03 make wrong | |
| Edit the doc to admit the gap | The audit's own named alternative if W2 were not closed | |

**Choice rationale:** the row carries two claims. "Connect lane only" becomes false once the field
lands; "the CLI's own text and json output derive the equivalent number" was **never** true —
`statusSummary` did none of that arithmetic. The audit records that closing W2 closes W3, so the row
should describe the post-fix reality rather than be softened.

**Notes:** the protojson `uint64`-as-string paragraph immediately below the row stays untouched — it
describes an encoding difference, not a field-presence difference.

---

## Acceptance gate shape

`[auto] Acceptance gate shape — Q: "How is the no-re-derivation rule proven, and how is the doc fix gated?" → Selected: "Discriminating behavioural fixture + zero-occurrence inflection-free doc gate, both self-tested" (recommended default)`

| Option | Description | Selected |
|--------|-------------|----------|
| Discriminating fixture + zero-occurrence doc gate, both with controls | Fixture whose `Pending()` differs from every naive re-derivation; extend the existing exact `orderedKeyDiff` key-order gate; anchor the doc gate on the inflection-free `the equivalent number from`; watch each gate fire RED against a constructed defect | ✓ |
| Presence grep for `res.Pending()` in `migrate_family.go` | Cheap; catches an obviously-copied loop | |
| Key-presence assertion (`"pending"` appears in the marshalled json) | Simplest possible assertion | |

**Choice rationale:** three prior findings in this repo's own memory make the weaker options
unacceptable. A presence grep is a claim, not evidence (`v2rbxwg2r8`). A key-*presence* check
tolerates the field landing in the wrong position and tolerates a wrong value. And the exact W3
sentence already defeated one audit sweep on a verb inflection (`derives` vs `derive`,
`6jj4dn31f8`), which is why the doc gate anchors on a phrase carrying no verb.

**Notes:** verified during discussion that `orderedKeyDiff` is already exact (length + positional),
so extending `want` with `"pending"` is sufficient and the helper must not be relaxed. Confirmed
`TestOperatorOutputEmpty` correctly has no `"migrate status"` entry (`uint64` cannot marshal to
`null`; its null-safety is gated directly by `TestMigrateFamilyStatusReportDocNeverMarshalsNull`).

---

## Scope boundary

`[auto] Scope boundary — Q: "Does this phase touch any other audit item?" → Selected: "W2 + W3 only" (recommended default)`

| Option | Description | Selected |
|--------|-------------|----------|
| W2 + W3 only | The one code fix the ROADMAP goal describes, plus the doc sentence it makes true | ✓ |
| W2 + W3 + W4 (CLAUDE.md "every surface") | Also a documentation-accuracy defect | |
| Sweep all six tech-debt items | Close the audit in one pass | |

**Choice rationale:** W1, W4, and the proto-typing item were each deliberately parked in the ROADMAP
backlog as Phases 999.2 / 999.3 / 999.4 by commit `c2098b23`. Absorbing a parked item into a
debt-closure phase would undo a decision already recorded in the roadmap. Confirmed during scouting
that `engram migration-status`, the Connect RPC, the MCP startup advisory, and the client footer all
already report `pending` — the operator-tier verb is the sole outlier, exactly as W2 states.

---

## Claude's Discretion

- Exact clause wording and punctuation in `statusSummary` (explicitly non-contractual per D-04).
- Exact prose of the rewritten `pending` doc row, provided it states the arithmetic, the `future`
  exclusion, and all three reporting surfaces.
- Plan decomposition — one plan is likely right; if split, the doc gate must not run before the code
  change lands.
- Whether to cross-link the doc row to `reference/memory-record.md`'s `schema_version` section.

## Deferred Ideas

- **W1** — full-stack E2E for `engram migrate`. Already ROADMAP Phase 999.2 (BACKLOG).
- **W4** — CLAUDE.md's "every surface" record-state claim overstates (MCP lane renders no state
  words). Already ROADMAP Phase 999.3 (BACKLOG).
- **Cross-cutting** — `schema_version` typed three ways in one proto file. Already ROADMAP Phase
  999.4 (BACKLOG).
- **dprint drift, 4 files, unenforced by CI** — neither `fmt:check` nor `dprint` appears in
  `.github/workflows/`. Pre-existing. The unenforced gate is the more interesting half.
- **Move `2026-08-10-research-versioned-payload-migration-mechanism.md` to `.planning/todos/done/`**
  — retired by Phase 8 but never filed.

## Todo cross-reference

One todo matched at score 0.60 (auto-fold threshold is 0.40) and was **reviewed but not folded**:
`2026-08-10-research-versioned-payload-migration-mechanism.md`. Phase 8's CONTEXT.md already folded
and retired it as "the origin of this milestone". Re-folding it would re-open settled scope into a
debt-closure phase. Recorded under CONTEXT.md §`Reviewed Todos (not folded)`.
