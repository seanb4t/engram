---
phase: 04
reviewers: [codex, opencode]
reviewed_at: 2026-08-14T23:44:55Z
plans_reviewed:
  - 04-01-PLAN.md — Tracer: v0→v1 short_id first customer
  - 04-02-PLAN.md — Store MigrateStatus histogram + PreviewRevert/Revert + startup warning
  - 04-03-PLAN.md — CLI surface: migrate command family
  - 04-04-PLAN.md — backfill-short-ids as thin delegating alias
cycle: 5
plans_revision: 02a35f09
cycle_1_findings:
  high: [H1, H2, H3, H4, H5, H6]
  actionable: [M1, M2, M3, M4, M5, M6, M7, M8, M9, M10, M11]
cycle_2_verdict:
  resolved_high: [H1, H2, H3, H4]
  partial_high: [H6]
  unresolved_high: [H5]
  new_high: [H7, H8]
cycle_3_verdict:
  resolved_high: [H5, H7, H8]
  unresolved_high: [M8, H6-preflight]
  actionable_unresolved: [M3, M4, N1, N3, N4, N5, N6]
  current_high_count: 2
  current_actionable_count: 7
cycle_4_verdict:
  resolved_high: [C3-HIGH-1, C3-HIGH-2]
  resolved_actionable: [M3, M4, N3, N4, N5, alias-apply-parity]
  partial_actionable: [N1]
  new_high: [C4-H1, C4-H2, C4-H3, C4-H4, C4-H5, C4-H6]
  new_actionable: [C4-M1, C4-M2, C4-M3, C4-L1, C4-L2, C4-L3, C4-L4, C4-L5, C4-L6]
  current_high_count: 6
  current_actionable_count: 9
  converges: false
cycle_5_verdict:
  resolved_high: [C4-H1, C4-H2, C4-H3, C4-H4, C4-H5, C4-H6]
  resolved_actionable: [C4-M1, C4-M2, C4-M3, C4-L1, C4-L2, C4-L3, C4-L4, C4-L5, C4-L6]
  new_high: [C5-H1, C5-H2, C5-H3, C5-H4, C5-H5]
  new_actionable:
    [C5-M1, C5-M2, C5-M3, C5-M4, C5-M5, C5-M6, C5-M7,
     C5-L1, C5-L2, C5-L3, C5-L4, C5-L5, C5-L6, C5-L7, C5-L8, C5-L9, C5-L10, C5-L11]
  current_high_count: 5
  current_actionable_count: 18
  converges: false
---

# Cross-AI Plan Review — Phase 4 (Cycle 5)

Reviewers: Codex (`gpt-5.1-codex`, xhigh) and OpenCode (`openrouter/moonshotai/kimi-k3`).
Both had full repo file access and verified plan claims against shipped source; neither output
carries the `[reviewed-without-repo-access]` marker. Plans reviewed are the revisions landed in
`02a35f09` ("docs(04): revise phase plans per cycle-4 review findings").

OpenCode's lane invocation was again killed by the runner's hardcoded 660 s `timeoutFloorMs`
(`review-lane-descriptor.cjs:210`, ETIMEDOUT after ~11 min with a partial trace). As in cycle 4 it
was re-run directly (`opencode run --model … --format json -`) against the same prompt file with an
extended deadline. No lane was dropped. The orchestrator (Claude Opus 5) also ran the priority-1
ledger enumeration and the priority-2 derivation execution independently against shipped source;
findings attributable to that pass are marked **[orch]**.

## Consensus Summary

**The ledger's arithmetic is sound; its enumeration methodology is not complete.** Both external
reviewers independently re-ran the four discovery searches recorded in the ledger's "How it was
built" note and found no missing row *within the shapes those searches cover*. That is the correct
result — and it is also the finding: **the searches themselves are the blind spot.** Three of the
four cycle-5 HIGHs are registries or obligations that are structurally invisible to all four
searches, because they are hand-maintained literal lists, or in-source prose obligations, rather
than `reflect.DeepEqual` gates, directory scans, `t.Fatalf` defaults, or consumers of the three live
derivations. The ledger has also no trigger for the phase's own headline change: the trigger legend
runs `CMD`/`EMIT`/`WRITE`/`DEL`/`PKGFILE` and has **no `VERSION` trigger** for `migrate.CurrentVersion`
rising 0→1, which is precisely what 04-01 Task 1 does.

**C4-H1 is genuinely and verifiably closed.** The replacement derivation was executed against the
live `surfaces.Operations()` table, not read off the plan. See "C4-H1 derivation, executed" below —
wave 3 is exactly 5 and wave 4 is exactly 6, and each member genuinely carries `--apply` at that
wave. All of C4-H2..H6 and C4-M1..M3, C4-L1..L6 are verified closed against shipped source.

**But the "red at execution time" class the ledger was built to close is not closed — it moved.**
The ledger polices *conformance registries*; the three remaining red-at-execution defects are not
registry rows at all:

- **the `CurrentVersion` 0→1 bump turns two shipped `internal/store` tests RED at wave 1** (C5-H2) —
  found independently by the orchestrator (via the missing `VERSION` trigger) and by OpenCode (via
  the concrete assertions), and one of the two carries an in-source obligation the codebase itself
  labels **BLOCKING for Phase 4**;
- **04-03 Task 1 cannot pass its own verify** (C5-H5) — three new gates in Task 1 reference toolclass
  rows and commands that Tasks 2 and 3 produce. This is C4-M3's exact defect class, re-instantiated
  three times *inside the task the C4-M3 fix edited*;
- **04-04's C4-H3 fix over-corrected into a negative grep that cannot go green on a fully compliant
  implementation** (C5-H1) — two surviving doc comments in files no plan owns.

Plus one plain correctness defect in a contract rather than a gate: an irreversible
`migrate revert --apply` is specified to render its refusal and return `nil`, so automation sees
exit 0 after a refused mutation (C5-H4).

The pattern across all five is worth naming, because it is the same one that has survived four
cycles in different clothes: **each cycle's fix is applied to the instances the previous review
named, and not to the class those instances belong to.** C4-M3 → fixed one identifier reference,
left three derivation references (C5-H5). C4-M1/C4-L3 → applied comment-text discipline to
`tools.go` and `backfill.go`, not to `internal/ cmd/` (C5-H1). The ledger → enumerated four search
shapes exhaustively, and did not ask what a fifth shape or a sixth trigger would find (C5-H3, C5-H2).

### Agreed Strengths

- The `Spared = manifestIDs \ observed` post-scroll set difference is mechanically correct and both
  reviewers traced it to the live filter: `backlogFilter`'s only arms are
  `Lt(schema_version, target)` and `IsEmpty(schema_version)`
  (`internal/store/migratebacklog.go:58-70`), so a manifest member stamped current between preview
  and apply is excluded **at the query** and can never be observed in the point loop. The
  `PurgeResult` shape it conforms to is real and verbatim (`internal/store/spine.go:1239-1249`).
- The single-pass manifest and DryRun paths correctly sidestep the PA-3 non-shrinking-backlog guard
  (`internal/store/migrate.go:167-179`), which would otherwise misdiagnose a non-writing preview as
  replenishment.
- The `CheckAdditive` carve-out is genuinely task-limited: it touches only the
  declared-but-never-added branch (`internal/migrate/additive.go:81`) and leaves the
  undeclared-added and removed-key branches byte-identical.
- `applyRoutedAdditions` as a small NAMED set with a pinning test is the right shape, and it has a
  real precedent in the same package (`operatorCommandExclusions`, `cmd/engram/cmdwalk.go:63-97`).
- 04-04's C4-L3 fix is exemplary: it explicitly identifies that a whole-file
  `! rg -- '--dry-run' upgrade.md` would directly contradict the D-12 gate, and replaces it with a
  section-scoped Go assertion reusing the shipped `extractUnreleasedSection`
  (`cmd/engram/docsync_test.go:34-48`). That is the correct instinct — it is simply not applied to
  04-04's own `BackfillShortIDs` gate (C5-H1) or its `timeout.*remov` gate (C5-L1).
- 04-01's `-o | wc -l` discipline (never `rg -c`) is stated explicitly at the two places it matters
  (04-01:154, 04-03:215).

### Agreed Concerns

- 04-04's unfiltered deletion gate is unsatisfiable (C5-H1) — raised independently by the
  orchestrator and OpenCode, each naming the same two doc comments.
- The `CurrentVersion` bump's wave-1 blast radius is unowned (C5-H2) — the orchestrator found the
  undischarged BLOCKING obligation, OpenCode found the two concrete RED assertions; the same fix
  closes both.
- The refusal path for `migrate revert --apply` returns success (C5-H4) — Codex raised it; the
  orchestrator confirmed the mechanism end to end in shipped source.
- Refusal-envelope construction is duplicated between `internal/store` (`revertRefusalErr`) and the
  CLI, with no shared exported constructor (C5-M4).
- The facet `Limit` is mandated but its value is left to executor discretion, and its acceptance
  gate cannot tell an adequate bound from an inadequate one (C5-M1).

### Divergent Views

- Codex rated the facet `Limit` bound HIGH; the orchestrator rates it MEDIUM, because the plan
  already specifies the sum-invariant (facet buckets + absent Count == whole-collection Count) that
  *can* detect truncation at runtime — the gap is that the plan never requires that check to run in
  production, only in a test whose fixture is too small to truncate. Both agree the fix is the same.
- Codex rated the inverse value-change contradiction HIGH; the orchestrator rates it MEDIUM,
  because no step in this phase's registry has a value-mutating inverse (the only registered step is
  `Irreversible`), so the defect is executor ambiguity rather than shipped behaviour.
- OpenCode judged the ledger "for the first time in five cycles, actually complete" and the
  registry-enumeration defect class "closed"; the orchestrator disagrees — see C5-H3. Both statements
  are consistent with the evidence: the ledger is complete *for the shapes searched*, and OpenCode
  searched those shapes. The disagreement is about whether completeness-under-a-methodology counts
  as completeness, and this cycle produced a counterexample.
- Codex read 04-03's residual `!ReadOnly` prose as LOW narrative drift; the orchestrator upgraded it
  to MEDIUM after finding it in Task 1's `<name>` line and the threat model, not only in prose.

---

## C4-H1 derivation, executed (priority 2) — VERIFIED

Executed against the live table, not read off the plan. `surfaces.Operations()` was dumped by a
throwaway test in `internal/surfaces` (removed after):

```
CLI=migrate-remap-owner      ReadOnly=false Destructive=true
CLI=prune-expired            ReadOnly=false Destructive=true
CLI=spine-review purge       ReadOnly=false Destructive=true
CLI=backfill-short-ids       ReadOnly=false Destructive=false
CLI=store,reindex,summarize-missing,serve,migrate-set-owner,
    spine-review archive,spine-review restore   ReadOnly=false Destructive=false
```

Live `--apply` carriers = the three `registerDestructive` callers: `prune.go:159`,
`spine_review_purge.go:425`, `migrate.go:257`. Three, matching `destructiveCommandNames()` exactly —
`TestDestructiveCommandsRequireApply` (`destructive_test.go:88-106`) is green today.

`mutatingCommandNames() = destructiveCommandNames() ∪ applyRoutedAdditions − pendingApplyConversion`:

| | wave 3 | wave 4 |
|---|---|---|
| `destructiveCommandNames()` | `migrate-remap-owner`, `prune-expired`, `spine-review purge`, **`migrate revert`** (new row, `Destructive:true` — 04-03:384) | same 4 |
| `∪ applyRoutedAdditions` | `+ migrate`, `+ backfill-short-ids` | same |
| `− pendingApplyConversion` | `− backfill-short-ids` | `−` ∅ (deleted by 04-04 T2) |
| **result** | **5** — `migrate`, `migrate revert`, `migrate-remap-owner`, `prune-expired`, `spine-review purge` | **6** — those 5 + `backfill-short-ids` |
| live `--apply` carriers | 3 existing + `migrate` + `migrate revert` = **5** | + `backfill-short-ids` = **6** |

Both directions hold at both waves. **The plan's enumerated result is correct.** Two secondary
checks also hold:

- `applyRoutedAdditions`'s pin (`Destructive:false` ∧ `ReadOnly:false` for every member) is
  satisfiable: `backfill-short-ids` is (false,false) live, and `migrate`'s new toolclass row is
  specified `Destructive:false`. `migrate revert` is correctly *excluded* from
  `applyRoutedAdditions` because it is `Destructive:true` and would be redundant-by-construction.
- `destructiveFlagCases` at wave 3 needs only the `migrateRevertCmd` row (04-03 T3 step 1b), because
  the table still keys on `destructiveCommandNames()` at that wave and `migrate` is
  `Destructive:false`. 04-03:384 states this explicitly and correctly. The `migrate` and
  `backfill-short-ids` rows land at wave 4 with the rename to `mutatingFlagCases`. **No wave-3 gap.**

One imprecision, not a defect: ledger row A13 assigns `TestDestructiveCommandsRouteThroughGate` to
wave 4 only. `migrate revert` enters `destructiveCommandNames()` at **wave 3**, so that gate applies
to it a wave earlier than the ledger says. It passes anyway because 04-03 routes `migrate revert`
through `registerDestructive`; the ledger's wave column is simply understated.

---

## New HIGH concerns (cycle 5)

### C5-H1 — 04-04's unfiltered `BackfillShortIDs|missingShortIDFilter` gate is UNSATISFIABLE **[orch]**

04-04 turned C4-H3's fix into an unfiltered negative grep and made it a `<verify><automated>` clause
(04-04:146, :187, :197):

```
{ ! rg -n "BackfillShortIDs|missingShortIDFilter" internal/ cmd/; }
```

Removing the `_test.go` exclusion was right. But the unfiltered grep now also matches **two
explanatory doc comments in files 04-04 does not own**:

- `internal/store/migrate.go:60` — `// (Reindex store.go:3133, BackfillShortIDs store.go:2741, RemapOwner`
  — part of the load-bearing comment explaining why `Store.Migrate` re-derives instead of threading
  a cursor, by contrast with the three sweeps that do.
- `internal/store/migratebacklog.go:42` — `// missingShortIDFilter (store.go:2726-2731), whose doc comments already`
  — part of `backlogFilter`'s `IsEmpty`-arm caveat, which cites the two sibling filters that carry
  the same caveat.

`internal/store/migratebacklog.go` is in **no plan's** `files_modified`. `internal/store/migrate.go`
is in 04-01's, but 04-01 is wave 1 and carries no instruction to scrub that comment; by wave 4 it is
still there. So 04-04's own automated verify **cannot go green on a fully compliant implementation**
— which is the same defect class as the self-invalidating gates cycles 3 and 4 kept finding, just
inverted: instead of a gate that passes vacuously, this one fails unconditionally.

Verified: `rg -c 'BackfillShortIDs|missingShortIDFilter' internal/ cmd/` →
`internal/store/migrate.go:1`, `internal/store/migratebacklog.go:1` (plus the sites 04-04 does own).

**Fix (needed in 04-04):** either (a) scope the gate to *declarations and call sites* rather than any
occurrence — e.g. assert `rg -n 'func \(s \*Store\) BackfillShortIDs|func missingShortIDFilter|\.BackfillShortIDs\(|missingShortIDFilter\(' internal/ cmd/` returns nothing, which is the real property
and leaves cross-referencing prose alone — or (b) add both files to `files_modified` and instruct
the rewrite of those two comments (losing two useful cross-references for no gain). (a) is
preferred; it is the call-shaped-not-name-shaped discipline 04-02 already applies to `\.Migrate\(`
(C4-M1), which 04-04 did not carry across.

### C5-H2 — the `CurrentVersion` 0→1 bump turns two shipped `internal/store` tests RED at wave 1, and PA-10a item 3 (BLOCKING) is undischarged **[orch + OpenCode, independently]**

04-01 Task 1 raises `CurrentVersion` 0→1 (`internal/migrate/migrate.go:45`). `payload()` stamps
`int(max(migrate.CurrentVersion, m.SchemaVersion))` (`internal/store/store.go:646`), and
`Store.Migrate` resolves `opts.Target == 0` to `migrate.CurrentVersion`
(`internal/store/migrate.go:109-112`). Two shipped tests go RED, deterministically, in wave 1:

1. **`TestBacklogFilterMatchesAbsentAndBelowTarget`** (`internal/store/migrate_test.go:321-330`).
   Its PA-4 tail calls `s.Migrate(ctx, MigrateOptions{Target: 0})` and asserts `res.Migrated == 0`
   and that no `schema_version` was stamped onto the absent record ("PA-4 violated"). Post-bump,
   `Target: 0` no longer means "target zero" — it resolves to `CurrentVersion = 1`, the `target <= 0`
   short-circuit is never reached, `backlogFilter(1)` matches the absent record, and the registered
   v0→v1 step migrates it. **Both assertions fire.** `internal/store/migrate_test.go` IS in 04-01's
   `files_modified`, but no task action mentions this test, and it is absent from both of 04-01's
   `-run` regexes — it surfaces only at the plan-level `task` run.
2. **`TestMigrateConvergesWithoutLock`** (`internal/store/migrate_converge_test.go:82`). The
   mid-sweep *laggard* is written through `writerStore.Upsert` with **no** `SchemaVersion`
   (`:155-158`); post-bump `payload()` stamps `max(1, 0) = 1`, and the at-write assertion at
   `:174-176` (`want 0 (below the sweep's target of 1)`) fails via `h.recordErr` → `drainErrs` →
   `t.Fatalf`. This is not a one-line fix: the laggard is the test's bounded-adversarial **control**
   (its own doc at `:143-153` — it is what distinguishes strict exclusion from a vacuous filter),
   and post-bump **no `Upsert` can create a below-target record at all**, so the control must move to
   raw injection. **`internal/store/migrate_converge_test.go` is in NO plan's `files_modified`**, yet
   04-01 Task 2's own `<verify>` runs it (04-01:262) — wave 1 stops at Task 2.

The second test additionally carries an explicit, shipped, self-describing Phase-4 obligation in its
PA-10a doc block (item 3):

> "PHASE4: the literal, causal half of SC5 — that new writes arrive already-current BECAUSE the
> write path stamps the current version — is deferred to Phase 4. When Phase 4 pairs
> CurrentVersion = 1 with the registered v0->v1 step, this same concurrency test must be re-run with
> an ORDINARY Memory carrying NO SchemaVersion at all, and MigrateOptions.Target left at zero so it
> resolves to CurrentVersion. **That re-run is the only direct proof of SC5's causal claim, and it is
> BLOCKING for Phase 4, not optional polish.**"

All four plans reference `TestMigrateConvergesWithoutLock` **only** as a "pre-existing store sweep
test still passes" regression check (04-01:249, :258, :262). No plan modifies it. The shipped test
currently substitutes for the missing constant by writing the *already-current* record with an
explicit `Memory.SchemaVersion = migrate.Version(1)` and supplying `MigrateOptions.Target: 1` itself
(`:179-180`) — exactly the substitution the source says Phase 4 must remove. `04-VALIDATION.md:57`
tracks the item open, and `04-RESEARCH.md:205,340` also calls it BLOCKING. Cycle 4 judged the new
bare-record end-to-end test to "cover" it; that judgment misses both that the *existing* test goes
red and that the causal half is a **concurrency** property (a mid-sweep ordinary write arriving
already-current because both sides read the same constant), which a sequential default-target test
cannot demonstrate. Leaving it means Phase 4 ships `CurrentVersion = 1` without the proof the
codebase itself declares blocking, and fails its own Nyquist validation.

This is also the visible symptom of a structural ledger gap: **the trigger legend has no `VERSION`
trigger.** `CMD`/`EMIT`/`WRITE`/`DEL`/`PKGFILE` do not cover "`migrate.CurrentVersion` rises 0→1",
which is 04-01 Task 1's headline change. The full CurrentVersion consumer set is
`internal/migrate/{migrate,registry,migrate_test}.go`, `internal/store/{migrate.go, store.go,
migrate_converge_test.go, schemaversion_compat_test.go, schemaversion_stamp_test.go,
schemaversion_test.go, store_test.go}`, `internal/server/schemaversion_wire_test.go`. All but
`migrate_converge_test.go` are CurrentVersion-*derived* and self-adapt; that one is not.

**Fix (needed in 04-01):**
1. Add a step (Task 1, or a new Task 1.5 before the sweep work) that repairs
   `TestBacklogFilterMatchesAbsentAndBelowTarget`'s PA-4 tail — post-bump, PA-4's `target <= 0`
   short-circuit is reachable only via a negative `Target`, so the assertion must be reframed rather
   than deleted, or the whole PA-4 guarantee loses its only test.
2. Add `internal/store/migrate_converge_test.go` to `files_modified` and a step reworking it per
   PA-10a item 3: laggard seeded via `seedLegacyRecord` (raw injection — `Upsert` can no longer
   produce a below-target record), already-current record written as an ordinary `Upsert` with **no**
   `SchemaVersion`, and `MigrateOptions.Target` left at zero so it resolves through `CurrentVersion`.
   That is simultaneously the repair and the discharge of the BLOCKING obligation.
3. Add a **`VERSION` trigger** to the ledger's trigger legend, with `TestBacklogFilterMatchesAbsentAndBelowTarget`,
   `TestMigrateConvergesWithoutLock` and `schemaversion_compat_test.go` (C5-L2) as its rows.

### C5-H3 — the ledger misses `operatorCommandFiles`, and misses the *class* it belongs to **[orch]**

`cmd/engram/operror_test.go:179-186` declares:

```go
// operatorCommandFiles lists the six operator command source files D-03
// scopes for full classification (CONTEXT.md). Kept as a literal here
// rather than a directory walk so a new operator command file must be
// added explicitly -- the same discipline TestExitCodeBaselineRowCount
// applies to its own row list.
var operatorCommandFiles = []string{
    "reindex.go", "prune.go", "summarize.go", "backfill.go", "migrate.go", "serve.go",
}
```

It drives `TestNoBareOperatorErrorReturns` (`operror_test.go:203-241`): no operator command file may
return a `fmt.Errorf`/`errors.New` the taxonomy could have classified, unwrapped, absent an
explicit `gsd:bare-operator-error-exception` marker.

04-03 creates **`cmd/engram/migrate_family.go`** — a new operator command file holding three new
operator commands and their `usageErrorf` validation. It is **not** added to `operatorCommandFiles`,
and no plan mentions the list (`rg operatorCommandFiles .planning/phases/04-*/` → nothing). Note the
list already contains `migrate.go`, which is the *existing* `migrate-remap-owner`/`migrate-set-owner`
file — so nothing about the name collision makes this self-correcting.

The consequence is the N1 failure mode 04-04 itself names: the gate does not go RED, it **silently
stops covering** the phase's three new operator commands. As 04-04:44 puts it about its own sibling
gates, "None of them FAILS today — which is worse than failing: the safety net silently shrinks
exactly as the gate widens."

**The class matters more than the row.** This registry is invisible to all four of the ledger's
discovery searches by construction: it has no `reflect.DeepEqual`, no `os.ReadDir`/`Walk`/`Glob`
(deliberately — its doc comment says so), no `"no row defined"`/`"stale entry"` message, and no
reference to `operatorCommands()`/`walkCommands(`/`surfaces.Operations()`. A fifth search shape is
needed: **package-level literal `[]string`/`map[string]bool` registries in `_test.go` files.** Run
`rg -n --glob '**/*_test.go' '^var [a-zA-Z_]+ (=|\[\])' cmd internal` — 15 hits in `cmd/engram`, 30
across `internal/`. Every other hit was checked and is either already in the ledger
(`timeoutGroups`→A4, `exitCodeBaseline`→A18/A19, `partialWriteClassification`→B1/B2/B3,
`qdrantClientHolderAllowlist`→B7, `proseTargets`→D3) or out of blast radius
(`allowedClientImports`, `scopeCrossSpineFlagCommands` — client tier; `envDerivedFlagDefaults` — no
new flag has an env-derived default; `qdrantBackedPackages` — no new package;
`wantRegisteredToolNames` — no new MCP tool, consistent with A21's `MCPTool: ""` constraint).
`operatorCommandFiles` is the one genuine miss.

**Fix (needed in 04-03 and in 04-01's ledger):** add row **A22** — `operatorCommandFiles` +
`TestNoBareOperatorErrorReturns`, `cmd/engram/operror_test.go:179-186`, trigger `CMD` (a new operator
command *file*), wave 3, owner 04-03 T3, with `cmd/engram/operror_test.go` added to 04-03's
`files_modified`; and add the fifth discovery search to the ledger's "How it was built" note.

### C5-H4 — an irreversible `migrate revert --apply` is specified to exit 0 **[Codex, confirmed orch]**

04-03:387 specifies the apply closure:

> "When `!plan.Reversible`, return the identical refusal rendering and **do not call `st.Revert` at
> all** — zero records touched…"

Traced through shipped source:

- `renderOperator` (`cmd/engram/operator_output.go:64-75`) returns the writer's error — `nil` on a
  successful render.
- `registerDestructive`'s `RunE` (`cmd/engram/destructive.go:124-131`) is
  `if applyRequested(*target) { return apply(ctx, cmd) }` — it returns the closure's error verbatim.

So `engram migrate revert --to 0 --apply` against the only registered step (which is `Irreversible`
by design, D-03) renders the refusal and **exits 0**. A script that checks `$?` sees the migration
succeeded. This contradicts 04-03's own must_have — "refuses the entire operation … in the
`field=<name> hint=<code>` envelope (D-13/D-14/D-16)" — and REQ-migrate-revert's "refuses"
criterion. No exit code is pinned anywhere in 04-03 for either refusal class: the only `exitUsage`
mentions in the whole plan (04-03:408) are the `operatorInvalidOutputArgs` rows, and the acceptance
criteria for the two refusal tests (04-03:416-417, :440) assert only on rendered text and on
`Revert` being called zero times — never on an exit code.

The bare (non-`--apply`) preview rendering an unreversible plan at exit 0 is correct and should stay.
Only `--apply` needs to fail.

**Fix (needed in 04-03):** after rendering, the apply closure must return a classified error
carrying a non-zero exit code (see `classifyOperatorErr` / `usageErrorf` in `cmd/engram/operror.go`
and the `exitCodeBaseline` conventions). Pin it with a CLI test asserting the exit code for BOTH
refusal classes (`Irreversible` and `Unsupported`), and add the expected code to the acceptance
criteria. Pairs with C5-M4: the cleanest form is an exported store-side constructor turning a
`RevertPlan` into the canonical refusal error, used by both preview-render and apply-return.

### C5-H5 — 04-03 Task 1 cannot pass its own verify: three new gates reference Task 2/3 artifacts **[OpenCode, confirmed orch]**

C4-M3's lesson was "no task may reference a gate artifact a later task produces". 04-03 applied it to
the one instance cycle 4 named (the `destructiveFlagCases` row keyed on `migrateRevertCmd`, moved to
Task 3 — 04-03:203) and **re-instantiated it three times in the same task**:

- **`TestMutatingCommandNamesMembership`** (Task 1 step 5b, 04-03:187) asserts
  `mutatingCommandNames()` equals exactly the five names **including `"migrate revert"`**. But
  `migrate revert`'s toolclass row — the only thing that puts it into `destructiveCommandNames()` —
  lands in **Task 3 step 3** (04-03:389). At the end of Task 1 the derivation yields four names. RED.
- **`TestApplyRoutedAdditionsArePinned`** (Task 1 step 5b, 04-03:186) asserts
  `surfaces.ClassForCommand(name)` resolves for every member of `applyRoutedAdditions`, which
  includes `"migrate"`. The `migrate` toolclass row lands in **Task 2**. RED.
- **`TestDestructiveCommandsRequireApply`** switched to `mutatingCommandNames()` (Task 1 step 6,
  04-03:196) puts `migrate` in the want-set, but `migrateCmd` and its `--apply` flag land in
  **Task 2**. `"mutating command \"migrate\" has no --apply flag"` fires. RED.

Task 1's own acceptance `<automated>` (04-03:221) runs all three:
`go test ./cmd/engram/ -run 'TestDestructive|TestMutatingCommandNamesMembership|TestApplyRoutedAdditionsArePinned|…'`.
The plan states at :203 that Task 1's `<verify>` compiles and runs the package "so Task 1 could not
pass in isolation" — the exact reasoning, applied to the identifier case and not to the three
derivation cases.

**Fix (needed in 04-03):** either (a) move `TestApplyRoutedAdditionsArePinned`,
`TestMutatingCommandNamesMembership`, and the `TestDestructiveCommandsRequireApply` switch into
Task 3, alongside the last toolclass row and command; or (b) pull the `migrate` toolclass row +
`migrateCmd` registration forward into Task 1 and leave only the membership pin for Task 3. Then
state the C4-M3 principle as a plan-level invariant in 04-03's preface, so it is applied to the
*class* rather than to one more instance.

---

## Actionable non-HIGH concerns (cycle 5)

- **C5-M1 — the facet `Limit` bound is undecided and its gate cannot detect an inadequate one.**
  04-02:142 leaves the value as `<explicit bound>` with "generous headroom" guidance; 04-02:162's
  acceptance gate is `rg -o 'Limit:' internal/store/migrate_status.go | wc -l` returns **at least 1**
  — satisfied by `Limit: 1`, and by a comment containing the token. Records are explicitly
  forward-compatible (the plan's own fixture uses versions 2 and 42), so no finite heuristic is
  provably safe, and the sum-invariant that *would* detect truncation is asserted only in a test
  whose fixture is too small to truncate. **PLAN.md change:** name a concrete constant with its
  reasoning, and add a runtime truncation check — either compare `facet buckets + absent Count`
  against a whole-collection exact `Count` inside `MigrateStatus` and return an error/`Truncated`
  flag on mismatch, or fail when `len(hits) == Limit`. Replace the `rg -o 'Limit:'` gate with a Go
  assertion on the constant's value. *(Codex rated HIGH.)*
- **C5-M2 — 04-02's inverse-write contract contradicts itself.** must_have 04-02:38 says "keys newly
  added **or value-changed** become `SetPayload`"; the task text at 04-02:244 says an inverse that
  changes a key's value in place "silently does not land" and "Do NOT attempt a deep value diff in
  this phase." An executor graded on must_haves would build the value diff the task forbids.
  **PLAN.md change:** reword the must_have to match the accepted, documented limitation (key-presence
  diffs only), keeping the `revert.go` doc-comment requirement. *(Codex rated HIGH.)*
- **C5-M3 — the persistent-failure arm prescribes a heuristic magic number when the shipped idiom is
  exact.** 04-02:282 tells the executor to "pick the saturating value that idiom supports; if
  `failCount` counts armed calls rather than passes, choose a bound comfortably above
  `4 × maxPasses`" — and then warns that a value which runs out mid-test silently reintroduces
  C4-H6. The signature it points at documents the exact answer one line above itself
  (`internal/store/migrate_faultinject_test.go:112-114`): "**count == 0 means unbounded**: every
  write from ordinal `from` onward fails." **PLAN.md change:** mandate
  `inj.arm(2, 0, faultBeforeInvoke)` and delete the heuristic. *(Raised by OpenCode's partial trace;
  verified against source.)*
- **C5-M4 — refusal-envelope construction is duplicated across store and CLI.** 04-02 creates an
  unexported `revertRefusalErr(plan)`; 04-03 independently formats the same `field=steps
  hint=irreversible` and `field=record_version hint=unsupported` clauses. The CLI cannot call the
  unexported helper, so the two will drift on fields, hint codes, reasons, and snapshot wording.
  **PLAN.md change:** export one constructor from `internal/store/revert.go` (04-02's `files_modified`
  already covers it) and have 04-03 consume it. *(Codex.)*
- **C5-L1 — `rg -n "timeout.*remov" upgrade.md` can self-invalidate.** 04-04:268 permits the
  `## Unreleased` entry to mention `--timeout` provided it says the flag is preserved; a single
  natural sentence ("`--timeout` is preserved; `--dry-run` is removed") trips the regex, and the gate
  is in 04-04 Task 3's `<automated>` (04-04:307). **PLAN.md change:** make it a section-scoped Go
  assertion like its C4-L3 sibling, or tighten the pattern to
  `--timeout[^\n]{0,30}\b(removed|no longer)\b`.
- **C5-L2 — `schemaversion_compat_test.go`'s dormant `older-explicit` row executes for the first
  time at `CurrentVersion = 1`.** The row set is CurrentVersion-derived and self-adapting
  (`internal/store/schemaversion_compat_test.go:43-84`), so it should not go RED — but the
  `older-explicit` row (`schemaVersion: CurrentVersion - 1`, `postUpdateVersion: CurrentVersion`) has
  never run, and the `absent` row's older-than claim is simultaneously withdrawn (`:103`).
  **PLAN.md change:** 04-01 should run `go test ./internal/store/ -run 'TestSchemaVersion' -count=1`
  as a named acceptance criterion and record the newly-executing row in the SUMMARY.
- **C5-L3 — three doc comments become factually false at `CurrentVersion = 1`.**
  `internal/migrate/registry.go:12` ("It ships EMPTY this phase: migrate.CurrentVersion stays 0"),
  `registry.go:61`, and `internal/migrate/migrate.go:31`. 04-01 step 9 updates only
  `migrate.go`'s `CurrentVersion` doc, and step 8 protects `registry.go`'s `// PHASE4:` marker
  without touching the surrounding prose. **PLAN.md change:** name all three in 04-01 T1.
- **C5-L4 — irreversibility is derived from the registry range, not from actual candidate chains.**
  04-02's `reversePreflight(steps, to)` marks every irreversible step with `To > target` regardless
  of whether any record has reached it. Harmless now (one step), but once v2+ exists an unused
  irreversible future step blocks reverts of records whose own chains are fully reversible — which
  contradicts the plan's otherwise per-record-chain model. **PLAN.md change:** either derive the
  irreversible set from the union of the candidate reverse chains the exhaustive preflight already
  collects, or record an explicit deferral with the reason. *(Codex.)*
- **C5-L5 — "non-blocking" startup warning is synchronous with a 10 s timeout.** Errors do not abort
  startup, but the prescribed `context.WithTimeout(…, 10*time.Second)` still blocks
  `buildDepsFromEnv`. REQ-migrate-never-automatic says "non-blocking". **PLAN.md change:** state
  which is meant — "non-fatal, synchronous, bounded at 10 s" or genuinely asynchronous — in the
  must_have and in the comment the task mandates. *(Codex.)*
- **C5-L7 — no plan mentions SPDX headers for the ~7 new Go files.** *(OpenCode.)* `v1_step.go`,
  `migrate_status.go`, `migrate_status_test.go`, `revert.go`, `revert_test.go`, `migrate_family.go`,
  `migrate_family_test.go` are all in scope per `.licenserc.yaml`. `license:check` is not part of
  `task` (lint + test), so this fails in CI rather than at execution — the worst place to find it.
  **PLAN.md change:** one line in each plan's new-file step, or a single "run `task license:add`
  before committing new files" in 04-01.
- **C5-L8 — `statusReportDoc`'s `buckets`/`future` slices can marshal as `null`.** *(OpenCode.)* They
  are `[]store.VersionBucket`; a zero-valued result marshals to JSON `null`, which
  `TestOperatorOutputEmpty` (`cmd/engram/operator_output_test.go:374-395`) rejects if a
  `migrate status` row is ever added there. 04-03's nil-slice discipline covers `migrateReportDoc`'s
  id lists but not the status doc. **PLAN.md change:** require both slices be initialised to empty at
  construction.
- **C5-L9 — `MigrateOptions{DryRun: true, Manifest: …}` is unspecified.** *(Codex and OpenCode
  independently.)* 04-01 step 6 scopes the DryRun branch to "`opts.DryRun` is true (and
  `opts.Manifest` is nil — DryRun does not combine with manual manifest)" but neither rejects the
  combination nor defines it, so the behaviour falls out of branch precedence.
  **PLAN.md change:** return a validation error for the combination, or state the precedence in the
  field doc comment and pin it with a test.
- **C5-L10 — 04-02's multi-page preflight fixture relies on an implicit id-ordering assumption.**
  *(OpenCode.)* The v42 record is expected to land on a later scroll page by being the
  "highest-sorting point id". That is sound — cursor pagination is id-ordered, the same property
  `scrollAllPoints` (`internal/store/spine.go:42-67`) already depends on — but the assumption is
  currently unstated, and it is what makes the multi-page test non-vacuous.
  **PLAN.md change:** state it in the test's mandated doc comment.
- **C5-L11 — DryRun's read-side cost is undocumented.** *(Codex.)* The preview path calls
  `MintShortID` for every eligible record; minting performs collision `Count` probes, so a
  300-record preview is materially more expensive than an ordinary dry run. The plan books this as
  PA-14 debt but never surfaces it to an operator. **PLAN.md change:** one sentence in
  `MigrateOptions.DryRun`'s doc comment.
- **C5-M5 — 04-03 still describes the `--apply` set as deriving from `!ReadOnly` in four
  high-visibility places, including the task's own name.** *(Codex raised as LOW; upgraded after
  verification — it is in the title line an executor reads first.)* Task 1's `<name>` (04-03:143)
  reads "…update TestDestructiveCommandsRequireApply derivation to !ReadOnly (M12)"; the objective
  (:84), the Output paragraph (:108), the threat model's routing row (":460 — The `--apply` set is
  now derived from `!ReadOnly` (M12), not `Destructive:true`"), and the success criteria (:504) all
  repeat it. This is exactly the C4-H1 formulation the same plan rejects three times in its
  must_haves (:51, :53, :55, :76). Note the *separate* and legitimate `!ReadOnly`: `registerDestructive`'s
  **admission** predicate does become `!class.ReadOnly` (:167, :207, :511) — that one is correct and
  must stay. The two must be verbally separated, because collapsing them is the original defect.
  Mitigating factor: an executor who implements the rejected predicate fails their own acceptance
  criteria (`applyRoutedAdditions` grep, `TestMutatingCommandNamesMembership`) and 04-04:220's STOP
  precondition. **04-04 carries the same defect in its Task 2 `read_first`** *(OpenCode)*, which
  describes `mutatingCommandNames()` as "derived from `!op.Class.ReadOnly && op.CLICommand != \"\"`,
  minus `pendingApplyConversion`" — the rejected predicate — while its own action precondition
  (04-04:220) has the correct derivation. **PLAN.md change:** rewrite the four `--apply`-set
  occurrences in 04-03 and the one in 04-04's `read_first` to the named-union formulation, and leave
  the admission-predicate occurrences alone.
- **C5-L6 — `recognizedFilterCarryingRequestMethods` is a latent trigger the ledger does not name.**
  `internal/store/schemaversion_recallgate_test.go:866-870` recognizes exactly `Query`/`Scroll`/`Count`;
  the interceptor `t.Fatalf`s (`:900`) on any other request type that implements `GetFilter()` and
  returns non-nil. 04-02's `MigrateStatus` adds a `Facet` call. It is inert **only because** the
  plan's `FacetCounts` literal carries `CollectionName`/`Key`/`Exact`/`Limit` and **no `Filter`**
  (04-02:142). That is load-bearing and currently implicit. The ledger's B5 note addresses
  `recallEmissionMethods` (`Facet` is absent from it) but says nothing about this second set.
  **PLAN.md change:** add a ledger constraint under B5 — "`MigrateStatus`'s `Facet` request MUST NOT
  carry a `Filter`; if a future revision adds one, widen `recognizedFilterCarryingRequestMethods` and
  the interceptor's type switch in the same edit" — and restate it in 04-02 T1's action.
- **C5-M6 — `migrate status` is the only Qdrant-dialing operator command with no `--timeout` and no
  signal handling.** *(OpenCode.)* 04-03 Task 2 step 3 gives it a plain `RunE` with flag set exactly
  `{output}`. Every other Qdrant-dialing operator command, **including the read-only ones**, installs
  both (`cmd/engram/spine_review_scan.go:55-59`, `spine_review_verify.go:639-643`), and every member
  of all three published timeout groups carries `--timeout` (`operator_output_test.go:445-465`). The
  plan's stated reason — that omitting it "keeps it out of `TestTimeoutGroupMatrix`'s set" — is
  circular: that gate exists to force a group assignment for a `--timeout`-bearing command, not to
  discourage the flag. A hung Qdrant makes `engram migrate status` block with no deadline and no
  graceful Ctrl-C, in a phase whose own H8/N3 findings are "every RPC path carries a finite
  deadline". **PLAN.md change:** add `migrateStatusTimeout` (5m default, zero-disables), the
  `signal.NotifyContext` + `migrateWithTimeout` wiring, the `timeoutGroups`/`timeoutGroupCaseArgs`
  rows, and the `guides/cli.md:377` table entry — all in the same task.
- **C5-M7 — the v1 honest-stamp guarantee is never stated, and `migrate.go:36-39`'s condition 3 goes
  stale.** *(OpenCode.)* Post-bump `payload()` stamps v1 on every write (`internal/store/store.go:646`)
  but omits `short_id` when `Memory.ShortID == ""` (`:658-659`). The v1 property holds only because
  the **server layer** mints before every Upsert (`internal/server/tools.go:1144`, `:1287`, `:2097`)
  — the codec cannot guarantee it, which is exactly what `internal/migrate/migrate.go:36-39`'s
  condition 3 says ("payload() cannot honour a v1 claim… exactly the false-currency claim rejected
  for partial writes"). 04-01 Task 1 step 9 requires updating the "no phase has yet registered a
  step" sentence but not condition 3, the most load-bearing of the three. A direct `Store.Upsert`
  with `ShortID == ""` (a test, a future caller) silently stamps a false v1 claim no sweep will
  revisit, because the record is at-target. Production paths mint first, so this is documentation
  and robustness, not a live defect — but this phase is meticulous about exactly the false-currency
  class. **PLAN.md change:** require 04-01 T1 step 9 to rewrite condition 3, stating that the stamp's
  honesty is a server-layer invariant with the `tools.go` mint sites named.

---

## Cycle-4 closure verification

All twelve verified closed against shipped source:

| Finding | Verdict | Evidence |
|---|---|---|
| C4-H1 | **CLOSED** | Derivation executed against live `surfaces.Operations()`; 5 at wave 3, 6 at wave 4, `--apply` carriers match both directions at both waves. See the executed table above. |
| C4-H2 | **CLOSED** | All four `operatorCommands()`-keyed registries owned by 04-03 T3 with `cmdwalk_test.go` + `operator_output_test.go` in `files_modified`; A7 (`TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs`, `cmdwalk_test.go:194-228`) newly found and rowed. |
| C4-H3 | **CLOSED** (but see C5-H1) | The `_test.go` exclusion is gone; both hidden call-site clusters (`store_test.go` seven sites, `operator_output_test.go` three refs) are owned. The unfiltered gate over-corrected — that is C5-H1, a new defect, not a reopening. |
| C4-H4 | **CLOSED** | Both classification tables rowed: `partialWriteClassification` (B1/B2/B3) and the recall-emitter three-way partition (B5/B6). Stale-row deletions owned by 04-04 T1. |
| C4-H5 | **CLOSED** | `Spared` is specified as a post-scroll set difference with `backlogFilter`'s `Lt`/`IsEmpty` arms named as the reason (04-01:38, :194); the dead-code prohibition is explicit (04-01:52). A RED-first observation against a loop-classified variant is required (04-01:252). |
| C4-H6 | **CLOSED** | Two sequential phases with disjoint expectations; the shipped one-shot self-heal precedent (`migrate_faultinject_test.go:313-360`) is cited as the reason. (Its arming instruction is imprecise — C5-M3.) |
| C4-M1 | **CLOSED** | Gate is call-shaped `! rg -n '\.Migrate\(' internal/server/tools.go`, which does not match `.MigrateStatus(`; the mandated comment must use bare form. Verified correct. |
| C4-M2 | **CLOSED** | A11 assigns golden regeneration to the end of wave 3 (04-03 T3) and again to 04-04 T2, after the last command lands. |
| C4-M3 | **CLOSED** | The `migrateRevertCmd` flag-case row moved from 04-03 T1 step 9 to T3 step 1b, alongside the identifier it references (04-03:203, :384). |
| C4-L1 | **CLOSED** | `rg -n 'did not shrink' internal/store/revert.go` gates the PA-3-analog termination guard, asserted by phase 1 of the reconciliation test. |
| C4-L2 | **CLOSED** as raised (Limit now mandatory) — its *value* is a new concern, C5-M1. |
| C4-L3 | **CLOSED** | v0.8.4 section reconciled in place, gated by a section-scoped Go assertion reusing `extractUnreleasedSection`, with the whole-file-grep contradiction explicitly rejected (04-04:281-287). Exemplary. |
| C4-L4 | **CLOSED** | Duplicate `-run` flags split into separate commands. |
| C4-L5 | **CLOSED** | Verified: `internal/store/reindex.go` does not exist; `ReindexOptions.DryRun` is at `internal/store/store.go:3020`, cited correctly (04-01:176). |
| C4-L6 | **CLOSED** | `Spared`/`Appeared` are `[]string` identity sets cross-referencing `PurgeResult` (`internal/store/spine.go:1245,1248` — verified verbatim); a negative gate forbids the `uint64` shape. |

---

## Ledger completeness audit (priority 1) — independent enumeration

The orchestrator re-ran the ledger's four searches plus a fifth shape. Results:

**Package-directory scans** (`os.ReadDir`/`filepath.Walk`/`WalkDir`/`Glob` in `_test.go`) — 11 hits.
Nine resolve to ledger rows or the ledger's own explicit dismissals. Two are uncited **helpers** of
already-rowed gates, not separate registries, and need no row:
`schemaversion_stamp_gate_test.go:198` (`scanPackageDirForCalls`, the scanner B1/B4 use) and
`schemaversion_recallgate_test.go:394` (`buildSamePackageCallGraph`, the reachability graph B5 uses).
`collectionprefix_conformance_test.go:336` (`extractTestCollectionPrefix`) is likewise a helper of B8.

**Bidirectional `reflect.DeepEqual` set gates** — 37 hits across `cmd` and `internal`. All
blast-radius-relevant ones are rowed. Checked and dismissed with reasons:
`catalog_test.go:385` (`TestCatalogExitCodesMatchMapper` — exit codes, not commands; this phase adds
no exit code), `catalog_test.go:416` (golden exit-code-7 description), `operror_test.go:169`
(`TestClassifyOperatorErrCodesAreDistinct` — sentinel exit codes, unchanged),
`surfaces_test.go:132` (derivation-vs-derivation, self-maintaining like A9/A10),
`registertools_test.go:93` (MCP tool names — A21's `MCPTool: ""` constraint keeps it inert).

**`t.Fatalf` on an unhandled default / "no row defined" / "stale entry"** — 9 hits, all rowed or
inert. One deserves a note: `schemaversion_recallgate_test.go:900` fatals when a gRPC method carries
a `*qdrant.Filter` but is absent from `recognizedFilterCarryingRequestMethods` (`:866`). 04-02's
`MigrateStatus` issues a `Facet`; the ledger's B5 note addresses `recallEmissionMethods` but not this
second set. It is inert **only if** the `Facet` request carries no filter — which the plan's design
(facet over present values + a separate filtered `Count`) satisfies. That load-bearing implicit is
C5-L6.

**Consumers of `operatorCommands()` / `walkCommands(` / `surfaces.Operations()`** — all rowed.

**Fifth shape, not in the ledger's methodology: package-level literal registries in tests**
(`rg -n --glob '**/*_test.go' '^var [a-zA-Z_]+ (=|\[\])' cmd internal`). This is the shape that
found C5-H3 (`operatorCommandFiles`). Every other hit was checked; see C5-H3 for the dismissals.

**Sixth gap, structural: no `VERSION` trigger** for `migrate.CurrentVersion` 0→1 — see C5-H2.

**Corroboration, and what it corroborates.** OpenCode ran the same four shapes independently across
the same five packages and likewise reported **no missing row** — it enumerated 19 in-scope
bidirectional gates and mapped every one to a ledger row, and independently verified the two
dismissals (`conditionalsweep_test.go:76`, `client_common_test.go:579,929`). Codex did the same and
also found none. That is three independent passes agreeing, and it is exactly why C5-H3 is a HIGH
about **methodology** rather than about one row: two reviewers using the ledger's own search shapes
converged on "complete", and the registry that is actually missed is the one no shape looks for.

**Net: 3 additions needed to the ledger** — row A22 (`operatorCommandFiles`), a `VERSION` trigger row
covering `TestBacklogFilterMatchesAbsentAndBelowTarget`, `migrate_converge_test.go` and
`schemaversion_compat_test.go`, and the two extra discovery searches recorded in the "How it was
built" note (literal test registries; `CurrentVersion` consumers). 04-04's existing "final phase-wide
ledger audit" step (04-04:350) should be extended to re-run all six.

---

## New-gate vacuity audit (priority 4)

The revision's two self-caught defects are real and correctly fixed: 04-02's `Store.Migrate` gate is
now call-shaped (C4-M1), and 04-04's `--dry-run` gate is now section-scoped rather than whole-file
(C4-L3). Both were verified. Three new defects of the same family were introduced:

| Gate | Location | Class | Finding |
|---|---|---|---|
| `! rg -n "BackfillShortIDs\|missingShortIDFilter" internal/ cmd/` | 04-04:146,187,197 | unsatisfiable — fails on compliant code | **C5-H1** |
| `rg -o 'Limit:' … \| wc -l` returns **at least 1** | 04-02:162 | vacuous — cannot distinguish an adequate bound | **C5-M1** |
| `! rg -n "timeout.*remov" upgrade.md` | 04-04:300,307 | self-invalidating against the sibling gate's own required text | **C5-L1** |
| `inj.arm(2, <large enough>, …)` | 04-02:282 | fixture-too-small class, when an exact idiom exists | **C5-M3** |
| Task 1's three new pins | 04-03:186,187,196 + verify :221 | gate asserted before the artifact it measures exists | **C5-H5** |

Gates checked and found sound: `rg -o … | wc -l` exact-count gates (04-01:154 = 4, 04-03:439 = 2,
04-03:215 = 1) correctly use `-o | wc -l` rather than `rg -c`, and each has a stated expected value;
04-03:350's file-set property (`rg -l "store.MigrateOptions" cmd/engram/ --glob '!**/*_test.go'`
names only `migrate_family.go`) is the right shape for the property it asserts; 04-02's `\.Migrate\(`
gate is genuinely immune to both `.MigrateStatus(` and its own doc comment; 04-04:239's
`destructiveCommandNames()` count is explicitly labelled advisory ("Verify by reading each of the
four, not by the count alone"), which is honest.

---

## Risk Assessment

**HIGH. Does not converge — 5 HIGH, 18 actionable.**

Two things this cycle had to establish both hold, and they are real progress: the 38-row ledger is
complete for the shapes it searches (three independent enumerations agree), and C4-H1's replacement
derivation is correct when executed against the live table rather than read. The mechanism designs —
minter injection, the `CheckAdditive` carve-out, the single-pass manifest apply with post-scroll
`Spared`, the whole-range revert preflight, the commit-point inverse write order — were traced to
shipped source by all three reviewers and found sound. No cycle-4 finding reopened.

What did not hold is the revision's broader claim that the red-at-execution class was systematically
closed. Three of the five HIGHs are wave-blocking and none of them is a conformance registry: a
wave-1 constant bump that reds two shipped tests (one of which the codebase labels BLOCKING for this
phase), a wave-3 task whose own verify cannot pass, and a wave-4 gate that cannot go green. The
fourth is a refused mutation reporting exit 0. The fifth is the ledger's methodology, not its
contents.

Every one of the five is a one-task plan edit with a self-describing failure mode. The reason to
take them seriously anyway is the pattern in the Consensus Summary: this phase has now spent five
cycles fixing named instances and leaving their classes intact. Cycle 6 converges if the revision
fixes the classes — a `VERSION` trigger and a fifth search shape in the ledger, the C4-M3 principle
stated as a plan-level invariant rather than applied to one more row, and the comment-text
discipline applied to every negative grep rather than to the two it was reported against.

## Codex Review (cycle 5)

### Codex — Cycle 5 report

## Summary

The revision is substantially stronger than cycle 4. The conformance ledger appears complete for the requested blast-radius categories: independent searches found no affected bidirectional registry or package-file scan missing from the ledger. The C4-H1 replacement also computes correctly: at wave 3 the named union yields five `--apply` carriers, and after removing the temporary exclusion it yields six at wave 4.

Three material design issues remain. Plan 02 permits silently truncated status histograms through an arbitrary facet limit. It also specifies contradictory handling for inverse value changes. Plan 03 renders an unreversible `--apply` request as successful output instead of returning a refusal error, making automation see exit 0 after no mutation.

Overall risk: **HIGH until these three issues are corrected**.

---

# Plan 01 — Registry First Customer and Migration Preview/Apply

## Summary

Plan 01 is detailed, executable, and well aligned with the shipped migration sweep. Its strongest work is the identity-based manifest design and the explicit treatment of records already processed by the legacy backfill path. I found no new blocking defect in this plan.

## Strengths

- The `CheckAdditive` carve-out is narrowly targeted. The existing implementation currently requires exact equality between declared and newly added keys at [additive.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive.go:38), and specifically reports every declared-but-not-added key at [additive.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive.go:81). Checking whether a missing declared key was already present in `before` addresses the legacy `short_id` state without weakening undeclared-addition or removal checks.

- The manifest semantics correctly follow the shipped purge precedent. `PurgeResult` already defines `Spared` and `Appeared` as identity sets, including the “ineligible or already gone” meaning at [spine.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/spine.go:1235). Reusing `[]string` avoids a same-package semantic mismatch.

- The plan correctly recognizes why `Spared` cannot be classified inside the migration loop. `backlogFilter` returns only absent or below-target records at [migratebacklog.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migratebacklog.go:58), so a now-current manifest member is absent from the scroll and can only be found through post-scroll set subtraction.

- The dedicated single-pass paths for dry-run and manifest-limited application correctly avoid the existing PA-3 convergence guard. That guard rejects a non-shrinking second pass at [migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:156), which would be inappropriate for a deliberately non-writing preview or bounded manifest application.

- The plan preserves the two-clone discipline around `Apply`, which is necessary because the current sweep explicitly protects against an apply function mutating its input map at [migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:213).

## Concerns

- **LOW — Preview has real read-side effects and cost.** The dry-run path invokes `MintShortID` for every eligible record. Although it does not mutate Qdrant, minting performs collision `Count` calls. This is consistent with D-04, but a 300-record preview is substantially more expensive than an ordinary dry-run. The plan documents this as PA-14 debt, so this is operational risk rather than a correctness gap.

- **LOW — `Manifest` as a raw map has no provenance binding.** The store API accepts any `map[string]migrate.Version`, not a compiler-enforced manifest type like purge. The CLI creates it safely, but another internal caller could submit an arbitrary set. This does not violate the requirement because eligibility is re-derived and only intersections are written.

## Suggestions

- Add an explicit `DryRun && Manifest != nil` validation error rather than relying only on branch precedence. That prevents an ambiguous API invocation from silently choosing one mode.

- Document that dry-run minting performs collision probes and may be expensive on large collections.

## Risk Assessment

**LOW.** The plan’s central mechanics match the live filters, sweep loop, and purge identity-set precedent.

---

# Plan 02 — Status, Revert, and Startup Warning

## Summary

Plan 02 closes the most dangerous backend gaps from previous cycles: exhaustive preflight, per-record reverse chains, partial-write reconciliation, and read/write conformance classifications. However, it still contains two serious correctness problems: the histogram may truncate valid versions, and the inverse-write contract contradicts itself about value changes.

## Strengths

- The exhaustive preflight correctly selects the package’s established pagination primitive. `scrollAllPoints` advances `NextPageOffset` until nil at [spine.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/spine.go:46), while the current migration loop intentionally resets offset to nil because each pass drains its own filter at [migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:182). The plan correctly distinguishes these cases.

- The exact reverse-chain invocation is sound. `StepsFrom(steps, from, to)` walks forward from `from` to `to` at [registry.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/registry.go:92), so obtaining the forward target-to-source chain via `StepsFrom(steps, to, from)` and reversing it is correct.

- The multi-page unsupported-version fixture is appropriately non-vacuous: five records with a page size of two ensures the unsupported record can occur beyond the first page.

- The two-phase fault-injection test fixes the mutually exclusive postcondition problem from cycle 4. A persistent failure can expose the intermediate commit-point state; convergence is then tested only after disarming.

- Both Qdrant conformance tables are covered. The live code has independent read-emitter and partial-write registries at [schemaversion_recallgate_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/schemaversion_recallgate_test.go:514) and [schemaversion_stamp_gate_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/schemaversion_stamp_gate_test.go:356). The plan updates both.

## Concerns

- **HIGH — The facet limit can silently truncate the required histogram.** The plan mandates an explicit limit but suggests “CurrentVersion plus generous headroom.” There is no bounded set of future versions: records are explicitly forward-compatible, and the test itself uses versions 2 and 42. Any finite heuristic can omit legitimate buckets, violating `REQ-migrate-status-histogram`. The total derived from returned facets would then also be wrong. An explicit limit avoids Qdrant’s default of 10 but does not solve truncation unless it is a documented maximum accepted by the API or truncation is detected.

- **HIGH — The inverse-write contract is internally contradictory.** The plan says newly added **or value-changed** keys become `SetPayload`, but later instructs implementation using only `AddedKeys` and `RemovedKeys`, explicitly accepting that same-key value changes “silently do not land.” The live helpers only compare key presence, not values, at [additive.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive.go:12). A reversible step whose inverse changes an existing value would be reported as successful while leaving the wrong value stored.

- **MEDIUM — Irreversibility is derived from the registry range rather than the chains represented by actual candidates.** `reversePreflight(steps, to)` is specified to mark every irreversible step with `To > target`, even if no record has reached that step. Once later migrations exist, an unused irreversible future step could prevent reverting records whose actual per-record chains are entirely reversible. This conflicts with the otherwise correct per-record-chain model.

- **MEDIUM — “Non-blocking” startup warning can delay startup by ten seconds.** The warning is best-effort in that errors do not abort startup, but the prescribed synchronous `context.WithTimeout(..., 10*time.Second)` still blocks `buildDepsFromEnv` until completion or timeout. If “non-blocking” is meant literally, it should run asynchronously; if it means “non-fatal,” the requirement and comments should say that.

## Suggestions

- Replace the heuristic facet limit with one of:

  - a sufficiently maximal API-supported limit plus a documented backend cap;
  - an exact per-version count strategy over a known supported range plus explicit discovery of future versions; or
  - a facet call whose truncation is detectable, returning an error rather than a false histogram.

- Add a value-diff helper using `reflect.DeepEqual` or protobuf/Qdrant value equality and include changed existing keys in the final `SetPayload`. Remove the statement that such changes may silently disappear.

- Derive irreversible steps from the union of actual candidate reverse chains collected during exhaustive preflight, not every registry step above the target.

- Clarify whether startup checking is synchronous-but-nonfatal or genuinely asynchronous.

## Risk Assessment

**HIGH.** Exhaustive preflight is well designed, but histogram truncation and silently dropped inverse value changes can produce materially false operator state.

---

# Plan 03 — Migration CLI Family

## Summary

Plan 03 handles the command-tree and conformance blast radius unusually thoroughly. The named `applyRoutedAdditions` approach is the correct replacement for the rejected `!ReadOnly` derivation, and the four command-keyed registries appear fully covered. The remaining critical problem is CLI refusal semantics: an unreversible `--apply` can return exit 0.

## Strengths

- The C4-H1 replacement arithmetic is correct against the live table. Today the destructive CLI commands are `migrate-remap-owner`, `prune-expired`, and `spine-review purge`; the additive routed additions are `migrate` and `backfill-short-ids`. At wave 3, subtracting pending backfill gives exactly:

  - `migrate`
  - `migrate revert`
  - `migrate-remap-owner`
  - `prune-expired`
  - `spine-review purge`

  At wave 4, removing the exclusion adds `backfill-short-ids`, giving exactly six. This avoids wrongly sweeping in writable commands such as `serve`, `reindex`, and `summarize-missing`, whose classifications are visible in [toolclass.go](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/toolclass.go:159).

- Generalizing the routing guard to `!ReadOnly` is consistent with the live class semantics: `ReadOnly` means no environmental mutation, while `Destructive` is only removal/overwrite at [toolclass.go](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/toolclass.go:15).

- The in-apply-closure preview avoids cross-invocation state and properly mirrors the purge flow.

- The timeout design is coherent: separate flag-backed variables, a duration-taking helper, and behavioral tests for 1s/default/zero. This catches a registered-but-unread flag.

- The command registry audit is complete. The live repository contains:

  - hand-known `operatorCommands` membership at [cmdwalk_test.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/cmdwalk_test.go:109);
  - operator output parity and invalid-output dispatch at [operator_output_test.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output_test.go:307);
  - catalog/toolclass equality at [catalog_test.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog_test.go:422);
  - timeout-group set equality via the live tree;
  - generated help/catalog goldens.

  All affected instances appear in the ledger and have plan owners.

## Concerns

- **HIGH — An unreversible `migrate revert --apply` is rendered as success instead of rejected with a nonzero exit.** The plan instructs the apply closure to render the refusal and not call `Store.Revert`. `renderOperator` returns nil on successful rendering, so `registerDestructive` returns nil from its selected apply closure. The current gate simply returns the closure error at [destructive.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive.go:124). Automation would therefore see exit 0 even though the explicitly requested mutation was refused. This conflicts with the machine-stable `field=... hint=...` error-envelope decision and the success criterion that the operation “refuses.”

- **MEDIUM — Refusal formatting is duplicated across store and CLI.** Plan 02 creates `revertRefusalErr(plan)`, while Plan 03 independently formats the same irreversible and unsupported clauses. The CLI cannot call the unexported helper, so drift is likely. The plan’s claim that preflight logic is shared is true, but the operator-visible error contract is not shared.

- **LOW — Several narrative statements still say the `--apply` set derives from `!ReadOnly`.** The actual prescribed implementation correctly uses the named union, but objective/success prose retains the superseded description. This increases executor ambiguity in a plan already carrying a long history of derivation corrections.

## Suggestions

- For bare preview, rendering an unreversible plan at exit 0 is reasonable.

- For `--apply`, return a classified error after rendering or, preferably, export a store/domain helper that converts `RevertPlan` to the canonical refusal error. The expected exit should be nonzero and pinned by a CLI test.

- Centralize refusal construction so store and CLI cannot disagree on fields, hints, reasons, or snapshot language.

- Replace all residual “derives from `!ReadOnly`” prose with the precise named-union formulation.

## Risk Assessment

**HIGH.** Command registration and conformance coverage are strong, but a refused destructive operation reporting success is unsafe for scripts and operators.

---

# Plan 04 — Backfill Alias and Documentation Reconciliation

## Summary

Plan 04 is comprehensive and directly addresses the deletion and hidden-test-call-site failures found in cycle 4. The alias delegates through the shared sweep runners, preserves timeout behavior, widens the remaining safety gates at the correct wave, and reconciles both current and historical documentation. No new blocking issue was found.

## Strengths

- The deletion blast radius is correctly handled across both source and tests. The live code currently names `Store.BackfillShortIDs` in both the read-emitter classification at [schemaversion_recallgate_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/schemaversion_recallgate_test.go:520) and the partial-write classification at [schemaversion_stamp_gate_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/schemaversion_stamp_gate_test.go:364). Plan 04 deletes both stale rows.

- The unfiltered caller grep fixes the earlier vacuity problem. It will see test references rather than excluding precisely the files that must be migrated.

- The test coverage migration is explicit rather than treating deleted tests as cleanup. In particular, the plan identifies cancellation, retry/resume, no-op convergence, and preservation of owner absence as separate behaviors.

- The alias shares both preview and apply runners. This structurally prevents the alias from bypassing the DryRun→Manifest intersection used by the canonical command.

- The widening of routing, flag-set, and usage-string gates is sequenced after the final additive command gains `--apply`. This preserves a green wave-3 baseline and makes the deliberate RED experiments meaningful.

- The documentation gate correctly scopes the negative assertion. Requiring `--dry-run` inside `## Unreleased` while forbidding it outside that section avoids the self-contradictory whole-file grep identified in prior review.

## Concerns

- **MEDIUM — Real-Qdrant command integration through global Cobra state may be brittle.** The plan mixes fake-store command tests, package-level Cobra commands, timeout variables, and a real-store alias test. Existing tests already require aggressive global flag resetting because state leaks between commands, as documented at [exitcode_baseline_test.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/exitcode_baseline_test.go:457). The new tests must reset `Changed` state and bound variables consistently.

- **LOW — The final grep cannot distinguish declarations from commentary outside its current scope.** The plan handles this by forbidding the literal in `backfill.go` and removing all exact exported-symbol references under `internal/` and `cmd/`. That is acceptable, but future documentation comments in those directories could make the gate unnecessarily fragile.

## Suggestions

- Use the repository’s existing whole-tree flag-reset helper before every command invocation in table-driven alias/canonical parity tests.

- Ensure the real-Qdrant integration test restores `migrateFamilyStoreFromEnv`, output flags, timeout variables, and apply booleans with `t.Cleanup`.

- Record the replacement coverage mapping in the summary exactly as requested; this is important evidence for deleting the old store tests.

## Risk Assessment

**MEDIUM.** The design is sound, but the wave makes broad deletions and mutates shared Cobra test state. The mandated unfiltered package runs mitigate most of that risk.

---

# Ledger Completeness Audit

I independently searched the requested shapes across `cmd/engram`, `internal/store`, `internal/surfaces`, `internal/migrate`, and `internal/server`.

The affected registries/scans found were:

- live Cobra command membership and catalog walkers;
- operator-output parity and invalid-output dispatch;
- timeout grouping;
- destructive/mutating routing and exact flag sets;
- catalog ↔ toolclass equality;
- help/catalog goldens;
- surface-rule applicability;
- Qdrant full-write and partial-write AST scans;
- Qdrant read-emitter classification;
- module-wide `qdrant.Client` holder scan;
- test-store construction scans;
- `internal/migrate` package-file stdlib/sealed-type scans;
- tool annotation MCP-name equality;
- conditional-error package glob.

All relevant affected instances are represented in the canonical ledger or explicitly dismissed there as outside the phase’s trigger set. I found **no missing ledger row** meeting the review’s HIGH-severity definition.

The live test-store scanner does indeed inspect every `_test.go` file in its package directories at [collectionprefix_conformance_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/collectionprefix_conformance_test.go:216), and the Qdrant holder gate walks module-wide non-test Go files at [schemaversion_stamp_gate_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/schemaversion_stamp_gate_test.go:813), matching ledger rows B8 and B7.

---

# Cycle-4 Closure Check

C4-H1 through C4-H6, C4-M1 through C4-M3, and C4-L1 through C4-L6 are genuinely addressed in the revised plans:

- named mutating-set derivation replaces the invalid `!ReadOnly` selection;
- all command-keyed registries are owned;
- both read and write Qdrant classification tables are handled;
- `Spared` is post-scroll set difference;
- persistent failure and convergence are separated;
- call-shaped and section-scoped greps avoid self-invalidation;
- facet `Limit` is now explicit, although its chosen bound remains a new correctness concern;
- unfiltered deletion grep includes tests;
- duplicate `-run` flags are split into separate commands;
- `Spared`/`Appeared` use identity-set shapes;
- golden regeneration occurs after command registration.

The remaining findings above are new cycle-5 issues rather than repetitions of closed cycle-4 findings.

---

## OpenCode Review (cycle 5)

### OpenCode — Cycle 5 report

**Reviewed with full repo access.** Every claim below was verified against source at the cited `file:line`.

## 1. Summary

The revision's central claim — that instance-patching was replaced by an exhaustive mechanical enumeration of conformance registries — is **largely borne out**: I independently re-ran the four discovery shapes (bidirectional `reflect.DeepEqual`, `os.ReadDir`/`filepath.Walk*`/`Glob` scans, `t.Fatalf` on unhandled defaults, consumers of `operatorCommands()`/`walkCommands()`/`surfaces.Operations()`) across `cmd/engram`, `internal/store`, `internal/surfaces`, `internal/migrate`, `internal/server` and found **no missing registry row** in the 38-row ledger. The core mechanism designs (minter-injected step, single-pass manifest apply, whole-range revert preflight, commit-point inverse write) all trace correctly against shipped code. However, the revision still contains **three execution-time blockers of the very class it claims to have eliminated**: the `CurrentVersion` 0→1 bump breaks two pre-existing `internal/store` tests no plan repairs (including the BLOCKING PA-10a item 3 re-run, still open in `04-VALIDATION.md:57`), 04-04's headline unfiltered gate self-invalidates on two surviving doc comments, and 04-03 Task 1 adds gates that cannot pass until Tasks 2–3 land — the same task-sequencing defect the revision's C4-M3 resolution articulated and then re-instantiated one task earlier.

## 2. Strengths

- **The ledger is genuinely exhaustive for its trigger shapes.** I re-derived the enumeration: every bidirectional set-equality gate in scope (`cmdwalk_test.go:117-130`, `operator_output_test.go:137/316-326/532-563/445-465`, `catalog_test.go:162-180/429-469/790-820`, `destructive_test.go:88-270`, `schemaversion_recallgate_test.go:666-745`, `schemaversion_stamp_gate_test.go:607-736/923-950`, `collectionprefix_conformance_test.go:240-330`, `leafpurity_test.go:70`, `step_test.go:138-165`, `additive_test.go:40-42`, `registry_test.go:242`, `migrate_test.go:16-19`, `rules_test.go:56`, `docsync_test.go:72-100`, `exitcode_baseline_test.go:440`, `toolannotations_test.go:27-46`, `surfaces_test.go:42-95`, `flaggroup_test.go:424-453`) maps to a ledger row, and the two excluded scans (`conditionalsweep_test.go:76` — `argErrf`-hint-only, `client_common_test.go:579,929` — client tier) are correctly dismissed with reasons I verified.
- **C4-H1's replacement derivation is arithmetically correct.** Against the live `operations` table (`toolclass.go:65-330`): 11 `ReadOnly:false` CLI commands, 3 `Destructive:true`. Wave 3: `destructiveCommandNames()` (4 incl. new `migrate revert` row) ∪ `applyRoutedAdditions` {migrate, backfill-short-ids} − `pendingApplyConversion` {backfill-short-ids} = **exactly 5**, matching the 5 `registerDestructive` callers (`prune.go:159`, `spine_review_purge.go:425`, `migrate.go:257` + 2 new). Wave 4: **exactly 6**. The `operatorCommandExclusions` precedent (`cmdwalk.go:81-84`) is faithfully followed, and `TestApplyRoutedAdditionsArePinned`'s `!Class.Destructive` check correctly makes a future reclassification fail loudly.
- **C4-H5/H6 closures are mechanically sound.** The post-scroll `Spared = manifestIDs \ observed` derivation is forced by `backlogFilter(target)`'s `Lt`/`IsEmpty` arms (`migrate.go:136`, `migratebacklog.go:58-70`) — a stamped-current record is excluded at the query, so a loop-local classification is genuinely unreachable. The two-phase reconciliation test is satisfiable: `setPayloadFaultInjector.arm(from, 0, mode)` means unbounded failure (`migrate_faultinject_test.go:113-115`), the interceptor passes `DeletePayload` through untouched (`:176-180`), and phases 1/2 assert disjoint postconditions.
- **C4-M1's call-shaped gate is verified well-formed**: `\.Migrate\(` does not match `.MigrateStatus(`, and `internal/server/tools.go` contains zero `.Migrate(` calls today.
- **Both stale literal counts the revision flags are real**: `partialWriteClassification`'s doc says "ten entries" with 11 actual rows (`schemaversion_stamp_gate_test.go:357-376`), and `operatorMigrationEmitters`' doc says "ten entries" with 11 actual rows (`schemaversion_recallgate_test.go:517-565`). Replacing rather than incrementing is the right call.
- **The H5 in-closure re-preview pattern is faithfully copied**: `spinePurgeApplyRun` does call `st.PreviewPurge` inside the apply closure (`spine_review_purge.go:365`), and `spinePurgeWithTimeout` is the zero-disables idiom claimed (`:107-113`). The `migrateTimeout` collision is real (`migrate.go:22`).
- **Store-side integration points all check out**: `PurgeResult.Spared/Appeared []string` at `spine.go:1245,1248` (the C4-L6 shape), `spineScrollBatch` is a var (`spine.go:26`), `scrollAllPoints` advances a cursor (`spine.go:42-67`), `schema_version` carries an integer index so `Facet` works (`store.go:579`), `recallEmissionMethods` excludes `Facet` (`schemaversion_recallgate_test.go:336-342`), `qdrantClientHolderAllowlist` is the claimed two entries (`:753-763`), and the seven `st.BackfillShortIDs` call sites in `store_test.go:5762-5893` are exactly as 04-04 states.

## 3. Concerns

### HIGH — Wave-1 breakage from the `CurrentVersion` bump is unplanned; PA-10a item 3 (BLOCKING) undischarged

04-01 Task 1 raises `CurrentVersion` 0→1 (`migrate.go:45`). `payload()` stamps `int(max(migrate.CurrentVersion, m.SchemaVersion))` (`store.go:646`). Two pre-existing tests deterministically go RED:

1. **`TestBacklogFilterMatchesAbsentAndBelowTarget`** (`migrate_test.go:206`): its tail (`:321-330`) calls `s.Migrate(ctx, MigrateOptions{Target: 0})` and asserts `Migrated == 0` and no stamp ("PA-4 violated"). Post-bump, `Target: 0` resolves to `CurrentVersion=1` (`migrate.go:109-112`), the `target <= 0` short-circuit is unreached, and the sweep migrates the absent record and the raw-injected `schema_version:0` record — both assertions fire. `migrate_test.go` is in 04-01's `files_modified`, but **no task action mentions this test**. It is not in 04-01 Task 2's verify regex either; it surfaces at the plan-level `task` run.
2. **`TestMigrateConvergesWithoutLock`** (`migrate_converge_test.go:82`): the laggard write (`:158-162`, no `SchemaVersion`) stamps `max(1, 0) = 1`, and the at-write assertion at `:175` (`want 0`) fails via `h.recordErr` → `drainErrs` → `t.Fatalf`. Worse, the laggard is the test's bounded-adversarial *control* (its doc at `:143-153`: it distinguishes strict exclusion from a vacuous filter) — post-bump, **no `Upsert` can create a below-target record at all**, so the control must move to raw injection, i.e. the test needs redesign, not a one-line fix. **`migrate_converge_test.go` is in NO plan's `files_modified`.** 04-01 Task 2's own verify runs this test (`-run 'TestMigrateConvergesWithoutLock|…'`) — wave 1 stops at Task 2.

This is precisely the BLOCKING PA-10a item 3: the test's own doc comment (`migrate_converge_test.go:60-66`) says "When Phase 4 pairs CurrentVersion = 1 with the registered v0->v1 step, this same concurrency test must be re-run with an ORDINARY Memory carrying NO SchemaVersion at all, and MigrateOptions.Target left at zero… **BLOCKING for Phase 4, not optional polish**." `04-VALIDATION.md:57` tracks it open; RESEARCH (`04-RESEARCH.md:205,340`) calls it BLOCKING; cycle 4's reviewer (`04-REVIEWS.md:462`) judged the bare-record test "covers" it — but that judgment misses that the *existing* test goes red and that the causal-half proof (mid-sweep ordinary write arriving already-current via the shared constant) is a concurrency property, not a default-target property. No plan discharges it.

### HIGH — 04-04's C4-H3 terminal gate self-invalidates on two surviving doc comments

04-04 Task 1's gate is `! rg -n "BackfillShortIDs|missingShortIDFilter" internal/ cmd/` (unfiltered — the C4-H3 fix). After the deletion, this still matches:

- `internal/store/migrate.go:60` — `Store.Migrate`'s doc comment: `// (Reindex store.go:3133, BackfillShortIDs store.go:2741, RemapOwner …`. 04-01 edits this file in wave 1 but nothing in 04-01's action removes the reference; at wave 4 it is stale *and* matches the gate.
- `internal/store/migratebacklog.go:42` — `backlogFilter`'s doc comment: `// … missingShortIDFilter (store.go:2726-2731), whose doc comments already state exactly this caveat …`. This file is in **no plan's** `files_modified`.

The plan's own action text asserts the unfiltered grep "is exactly steps 6b and 6c below plus `cmd/engram/backfill.go` (step 3) and the two classification tables" — running it today returns those two comment hits as well, so the enumeration is false and the terminal gate is unsatisfiable without editing two unlisted files. This is the C4-M1 defect class (a negative grep matched by documenting prose) in a **new instance the revision introduced** while fixing it for `tools.go` (bare-form `Store.Migrate`) and `backfill.go` (`dry-run`). The comment-text discipline was applied to two of four sites.

### HIGH — 04-03 Task 1 cannot pass its own verify (C4-M3's class, one task earlier)

Task 1 adds two pinning tests and updates a third, all referencing artifacts produced in Tasks 2–3:

- `TestApplyRoutedAdditionsArePinned` (Task 1 step 5b) requires `surfaces.ClassForCommand("migrate")` to resolve — the `migrate` toolclass row lands in **Task 2 step 5**. RED after Task 1.
- `TestMutatingCommandNamesMembership` (Task 1 step 5b) pins the five-name set — after Task 1, `mutatingCommandNames()` yields only 4 ({remap-owner, prune, purge, migrate}; the `migrate revert` row that puts it in `destructiveCommandNames()` lands in **Task 3 step 3**). RED.
- `TestDestructiveCommandsRequireApply` (Task 1 step 6) derives from `mutatingCommandNames()`, which includes `migrate` — but `migrateCmd` and its `--apply` flag land in **Task 2**, so "mutating command \"migrate\" has no --apply flag" fires. RED.

Task 1's acceptance criteria run all three (`-run 'TestDestructive|TestMutatingCommandNamesMembership|TestApplyRoutedAdditionsArePinned|…'`). This is structurally identical to cycle 4's C4-M3 ("a `destructiveFlagCases` row referenced an identifier declared a task later… Task 1 could not pass in isolation") — the revision moved the flag row to Task 3 but left the derivation/pinning gates in Task 1 referencing Task 2/3 artifacts. Fix options: move the two new pinning tests + the `RequireApply` switch to Task 3 (after both rows and both commands exist), or move the `migrate` toolclass row + `migrateCmd` into Task 1 and pin `TestMutatingCommandNamesMembership` in Task 3.

### MEDIUM — `migrate status` is the only Qdrant-dialing operator command with no `--timeout` and no signal handling

04-03 Task 2 step 3 gives `migrate status` a plain `RunE` (`operatorOutputFormat` → dial → `st.MigrateStatus(ctx)` → render) with flag set exactly `{output}` — no `--timeout`, no `signal.NotifyContext`. Every other Qdrant-dialing operator command, **including the read-only ones**, installs both: `spine_review_scan.go:55-59`, `spine_review_verify.go:639-643` (signal + own timeout var), and all ten zero-disables members plus the two reject-zero members carry `--timeout` (`operator_output_test.go:445-465`). The plan's stated reason ("keeps it out of `TestTimeoutGroupMatrix`'s set") is circular — the matrix's set equality (`:625-648`) exists to force a group assignment for `--timeout`-bearing commands, not to discourage the flag. A hung Qdrant makes `engram migrate status` block with no deadline and no graceful Ctrl-C cancellation, in a phase whose own H8/N3 findings are "every RPC path carries a finite deadline." Adding `--timeout` (default 5m, zero-disables row + case-args entry + cli.md:377 table entry) is a ~15-line change consistent with the tier.

### MEDIUM — The v1 honest-stamp guarantee is never stated; `migrate.go:36-39`'s condition-3 reasoning goes stale

Post-bump, `payload()` stamps v1 on every write (`store.go:646`) but omits `short_id` when `Memory.ShortID == ""` (`store.go:658-659`). The v1 property ("record carries short_id") holds only because the **server layer** mints before every Upsert (`tools.go:1144`, `:1287`, `:2097`) — the codec cannot guarantee it, which is exactly what `migrate.go:36-39`'s condition 3 says ("payload() cannot honour a v1 claim… exactly the false-currency claim rejected for partial writes"). 04-01 Task 1 step 9 requires updating the doc comment's "no phase has yet registered a step" claim but doesn't require addressing condition 3 — the most load-bearing of the three. A direct `Store.Upsert` with `ShortID==""` (tests, any future caller) silently stamps a false v1 claim that no sweep will ever revisit (the record is at-target). Production paths mint first, so this is documentation/robustness, not a live defect — but this phase is meticulous about exactly the false-currency class (`partialWriteClassification`'s entire justification) and should state where the guarantee lives.

### LOW

- **04-04 Task 2's `read_first` is stale**: it describes `mutatingCommandNames()` as "derived from `!op.Class.ReadOnly && op.CLICommand != \"\"`, minus `pendingApplyConversion`" — the rejected C4-H1 predicate. The action's precondition has the correct derivation. In a phase whose lesson is stale text, this one will confuse the executor.
- **SPDX headers**: ~7 new Go files (`v1_step.go`, `migrate_status.go`, `migrate_status_test.go`, `revert.go`, `revert_test.go`, `migrate_family.go`, `migrate_family_test.go`) need the Apache-2.0 header. No plan mentions it; `license:check` is not in `task` (lint+test) so it fails in CI, not at execution. One line in a plan pre-empts it.
- **`statusReportDoc` nil-slice marshaling**: `buckets`/`future` are `[]store.VersionBucket`; a zero-valued result marshals them as `null`, which `TestOperatorOutputEmpty` (`operator_output_test.go:374-395`) rejects if the row is ever added there. The plan's nil-slice discipline covers `migrateReportDoc`'s id lists but not the status doc — initialize to empty slices at construction.
- **`MigrateOptions{DryRun:true, Manifest:…}` unspecified**: 04-01 step 6 says DryRun "does not combine with manual manifest" but neither rejects nor defines the combination. An error or a doc line closes it.
- **04-02's multi-page preflight test** relies on the v42 record landing on a later scroll page via "highest-sorting point id" — sound because cursor pagination is id-ordered (the same assumption `scrollAllPoints` is already load-bearing on, `spine.go:42-67`), but the plan should say the assumption out loud; it is currently implicit.

## 4. Suggestions

1. **Add a Task 0 / Task 1.5 to 04-01**: update `TestBacklogFilterMatchesAbsentAndBelowTarget` (drop or reframe the `Target: 0` sweep assertions — note PA-4 becomes reachable only via negative `Target` post-bump) and rework `TestMigrateConvergesWithoutLock` per PA-10a item 3 (laggard via `seedLegacyRecord`; already-current record as an ordinary `Upsert` with no `SchemaVersion`; `Target` left zero to resolve through `CurrentVersion`). Add `internal/store/migrate_converge_test.go` to 04-01's `files_modified`. This discharges the BLOCKING validation item and un-reds wave 1.
2. **Extend 04-04 Task 1 step 6's grep-first enumeration** to include `internal/store/migrate.go:60` and `internal/store/migratebacklog.go:42`; add both files to `files_modified` and delete/update the two stale references as part of the deletion task (they *should* be updated anyway — they name deleted symbols). Generalize the lesson in the ledger: a deletion gate over `internal/ cmd/` must be run **before** the plan is finalized, and every hit — comment or code — owned by a step.
3. **Re-sequence 04-03**: either (a) move `TestApplyRoutedAdditionsArePinned`, `TestMutatingCommandNamesMembership`, and the `TestDestructiveCommandsRequireApply` switch into Task 3 (with the `migrate revert` row), or (b) pull the `migrate` toolclass row and `migrateCmd` registration into Task 1 and leave only the membership pin for Task 3. Apply the C4-M3 principle — "no task may reference a gate artifact a later task produces" — as a stated invariant in the plan's preface.
4. **Give `migrate status` a `--timeout`** (zero-disables group, 5m default, `migrateStatusTimeout` var) and the standard `signal.NotifyContext` + `migrateWithTimeout` wiring; add the `timeoutGroups`/`timeoutGroupCaseArgs` row and the cli.md:377 table entry in the same edit.
5. **State the honest-stamp guarantee** in 04-01's `migrate.go` doc-comment update: "new writes carry short_id because the server mints before Upsert (`tools.go:1144`); the stamp's honesty is a server-layer invariant, not a codec one."
6. Add SPDX-header creation to each plan's new-file steps (or one line in 04-01: "run `task license:add` before committing new files").

## 5. Risk Assessment

**Overall: MEDIUM-HIGH.**

The phase's load-bearing *mechanisms* are in good shape: I traced the minter-injection path, the CheckAdditive carve-out (`additive.go:87-91` — the `missing` branch is the only one touched; the empty-string/`null` `short_id` edge where a mint would be silently dropped by the key-presence write-shaping is unreachable in production because `payload()` omits empty `short_id` at `store.go:658-659` and the old backfill's `IsEmpty` filter never matched empty strings), the single-pass manifest apply, the whole-range preflight, and the commit-point revert write order against shipped source and found them sound. The 38-row ledger is, for the first time in five cycles, actually complete against an independent re-enumeration — the registry-enumeration defect class is closed.

The residual risk is concentrated and specific: **three execution-time blockers**, each failing loudly with a self-describing message and a mechanical fix — the wave-1 `CurrentVersion`-bump test breakage (with the BLOCKING PA-10a re-run undischarged), the wave-3 Task-1 gate sequencing, and the wave-4 self-invalidating deletion gate. None is a design flaw; all three contradict the revision's claim that the red-at-execution class was systematically closed, and the first one blocks the phase's own Nyquist validation (`04-VALIDATION.md:57`). **Convergence verdict: NO** — three HIGH findings, all one-task plan edits. With the PA-4/converge-test repairs planned, the 04-04 enumeration completed, and 04-03's Task 1 re-sequenced, cycle 6 should converge.
---


# Cross-AI Plan Review — Phase 4 (Cycle 4)

Reviewers: Codex (`gpt-5.1-codex`, xhigh) and OpenCode (`openrouter/moonshotai/kimi-k3`).
Both had full repo file access and verified plan claims against shipped source; neither output
carries the `[reviewed-without-repo-access]` marker. Plans reviewed are the revisions landed in
`1208b945` ("docs(04): revise phase plans per cycle-3 review findings").

OpenCode's first lane invocation was killed by the runner's 660 s `timeoutFloorMs` (ETIMEDOUT
on an empty output). It was re-run directly against the same prompt file with an extended
deadline and completed; the review below is that second, complete run. No lane was dropped.

## Consensus Summary

**Both cycle-3 HIGHs are genuinely RESOLVED**, and both reviewers verified each independently
against shipped source. `store.RevertPlan` / `Store.PreviewRevert` now have a named sole
producer in 04-02 (`internal/store/revert.go`, in `files_modified` and the artifact ledger),
with a STOP instruction in 04-03 if the symbol is absent at execution time. The revert
unsupported-version preflight is now a separate zero-write pass over `s.scrollAllPoints` — a
genuine cursor-advancing iterator (`internal/store/spine.go:46-69`), not `Store.Migrate`'s
`Offset: nil` re-scroll — proven by a 5-record/3-page test that forces `spineScrollBatch = 2`.

Of the seven cycle-3 actionables, **six are RESOLVED** (M3, M4, N3 `--timeout`, N4 DryRun
semantics, N5 rule sentence, alias apply parity) and **one is PARTIAL** (N1: the three sibling
gates *are* widened, but the shared derivation they are widened to is wrong — see C4-H1). All
five of the revision's self-claimed blocker fixes are real and correctly applied; the
`migrateTimeout` collision at `cmd/engram/migrate.go:22` is confirmed, and two independent
symbol sweeps found **zero** other same-package collisions across ~40 newly-declared identifiers.

**The cycle does not converge.** Six HIGH-severity defects remain, all newly raised. Every one
is an *executable* blocker verified against shipped source rather than a design critique — the
plans' mutation logic is sound; the failures are concentrated in conformance-test derivations,
command-name-keyed registries, and two test contracts that assert mutually exclusive states.

### Agreed Strengths

- The whole-range preflight is mechanically correct: `scrollAllPoints` advances a
  `*qdrant.PointId` cursor until `next == nil` (`internal/store/spine.go:46-69`), and the plan
  explicitly *forbids* reusing `Store.Migrate`'s `Offset: nil` shape, naming why.
- The M3 write contract is constructible exactly as planned: the shipped fault injector
  type-switches on `*qdrant.SetPayloadPoints` only (`internal/store/migrate_faultinject_test.go:176-180`),
  so a record's `DeletePayload` passes through untouched while its following `SetPayload` is armed.
- M4 now carries `Future []VersionBucket` + derived `FutureTotal` with a v2-and-v42 fixture
  asserting `len(Future) == 2`, and the startup warning renders the version list.
- N3's `migrateWithTimeout(ctx, d time.Duration)` is duration-taking with per-leaf vars and
  behavioural 1s/default/0 deadline tests — the flag is now actually read.
- N5's sentence change is unconditional and all six ripple locations are enumerated and
  verified present (`internal/surfaces/rules.go:232`, `rules_test.go:56`, the anchored regions
  at `cli.md:135` and `SKILL.md:379`, `help.golden` ×3, `catalog.golden` ×3), with
  `task surfaces:gen` as the sanctioned propagation.
- Cross-plan symbol ownership is now explicit throughout, with a final
  `go build ./... && go vet ./...` resolution sweep in 04-04.

### Agreed Concerns

- **C4-H1 (HIGH, 04-03/04-04) — `mutatingCommandNames()` = `!ReadOnly` is over-broad.**
  Raised by OpenCode, independently confirmed by executing the predicate against
  `surfaces.Operations()`. Four conformance gates go RED across waves 3–4. See below.
- **C4-H2 (HIGH, 04-03) — the three new operator commands enter three command-keyed
  registries the plans never touch.** Raised in part by both reviewers (Codex named
  `TestOperatorOutputParity`; OpenCode named `operatorInvalidOutputArgs`); a third —
  `wantOperatorCommandKeys` in `cmd/engram/cmdwalk_test.go` — was found only in this
  cycle's own source-grounding sweep and is in **zero** plan files.
- **C4-H3 (HIGH, 04-04) — the wave-4 dead-code deletion breaks compilation in two test files
  the live-caller grep gate deliberately excludes.** Codex found the `cmd/engram` half; the
  `internal/store/store_test.go` half (7 call sites) was found in this cycle's sweep.
- **C4-H4 (HIGH, 04-02/04-04) — `partialWriteClassification` breaks in both directions.**
  Codex found the stale-row half; the unclassified-new-write-site half (wave 2) is new here.
- **C4-H5 (HIGH, 04-01) — `Spared` is unobservable through `backlogFilter`.** Codex only;
  confirmed against `internal/store/migratebacklog.go:58-70`.
- **C4-H6 (HIGH, 04-02) — the revert reconciliation test asserts mutually exclusive
  postconditions.** Codex only; confirmed against
  `internal/store/migrate_faultinject_test.go:313-360`.

### Divergent Views

- **C4-H1.** OpenCode rates the `!ReadOnly` over-breadth HIGH and enumerates the seven
  offending commands. Codex marked N1 flatly RESOLVED, having verified only that the *sibling
  gates were switched*, not what the shared derivation actually selects. Independent
  verification (executing the predicate over the live table) confirms OpenCode. Recorded HIGH.
- **Self-claimed blocker 5 (recall-emission allowlist).** Codex rates it PARTIAL because the
  plans miss the second, independent set-equality row for the same deleted method
  (`internal/store/schemaversion_stamp_gate_test.go:370`); OpenCode rates the *recall gate*
  RESOLVED, which it is — the two reviewers are adjudicating different tables. Both are right;
  the residue is folded into C4-H4.
- **Codex-only:** C4-H5 (`Spared`) and C4-H6 (fault-injection contract). OpenCode did not
  reach either; both were independently confirmed against source here.
- **OpenCode-only:** the four MEDIUM/LOW sequencing and gate-hygiene items (C4-M1 through
  C4-L3). Codex did not raise them; all four were spot-checked and hold.

## Verified new HIGH concerns (union, all source-grounded)

### C4-H1 — `mutatingCommandNames()` = `!ReadOnly && CLICommand != ""` selects 7 commands that will never carry `--apply`

04-03 Task 1 step 5 (`04-03-PLAN.md:156`) defines the derivation; 04-04 Task 2 widens three
more gates onto it. Executing that exact predicate over the live `surfaces.Operations()` table
yields **11 existing commands**, of which only three carry `--apply` today
(`prune.go:159`, `spine_review_purge.go:425`, `migrate.go:257` are `registerDestructive`'s only
callers). The seven with no `--apply` and no `registerDestructive` routing are:

`store`, `reindex`, `summarize-missing`, `serve`, `migrate-set-owner`,
`spine-review archive`, `spine-review restore`.

`pendingApplyConversion` (`04-03-PLAN.md:157`) excludes only `backfill-short-ids`, and its
stated rationale — "`backfill-short-ids` is the last mutating command to gain `--apply`" — is
factually wrong against `internal/surfaces/toolclass.go`.

Consequences: `TestDestructiveCommandsRequireApply` fails on 7 commands at **wave 3** (04-03
Task 1's own `<verify>` runs it); at **wave 4**, `TestDestructiveCommandsRouteThroughGate`,
`TestDestructiveCommandsExactFlagSet`, and `TestApplyFlagUsageComposesRuleSentence` each fail
on the same 7. 04-04's RED-first non-vacuity experiments are also undermined — the gates are
already red, so a deliberate violation proves nothing.

The registerDestructive-routed tier is not expressible from the table (no tier column). The
package already carries the right precedent: `operatorCommandExclusions`
(`cmd/engram/cmdwalk.go:63-97`), a small NAMED set with per-entry justification and a pinning
test.

### C4-H2 — three new operator commands enter three command-keyed registries no plan updates

`migrate`, `migrate status`, and `migrate revert` all satisfy `operatorCommands()`'s predicate
(`cmd/engram/cmdwalk.go:101-117`: non-nil RunE, no `server` flag, not in the named exclusions),
so all three enter it at wave 3. 04-03 lists `cmd/engram/operator_output_test.go` in
`files_modified` but assigns work there **only** for `timeoutGroups`. Three registries go RED:

1. `wantOperatorCommandKeys` (`cmd/engram/cmdwalk_test.go:117-130`) — `TestOperatorCommands`
   asserts set equality in **both** directions (`:155-165`). `cmd/engram/cmdwalk_test.go`
   appears in **zero** plan files, in `files_modified` or any task action.
2. `operatorParityRows()` (`cmd/engram/operator_output_test.go:137`) — `TestOperatorOutputParity`
   gates the row set both directions against `operatorCommands()` (`:316-326`).
3. `operatorInvalidOutputArgs()` (`:532-563`) — its default branch is
   `t.Fatalf("operatorInvalidOutputArgs: no row defined for command %q")` (`:560`), reached from
   `TestEveryOperatorCommandRejectsInvalidOutput` (`:570-583`), which iterates every
   `operatorCommands()` member. Zero plan hits.

This is the same defect class the revision counted as a self-found blocker for
`TestTimeoutGroupMatrix`; it found one instance of it and stopped.

### C4-H3 — wave-4 dead-code deletion breaks two test files the plan's own gate excludes

04-04 Task 1 step 6 (`04-04-PLAN.md:130`) gates live callers with
`rg -n "BackfillShortIDs|missingShortIDFilter" --glob '!**/*_test.go'` — which excludes exactly
the files that will fail to compile:

- `internal/store/store_test.go` — 7 call sites on `st.BackfillShortIDs` across three test
  functions (`:5762, :5777, :5802, :5823, :5861, :5871, :5893`). `store_test.go` is in no
  plan's `files_modified`.
- `cmd/engram/operator_output_test.go` — references `backfillSummary`/`backfillReportDoc`, which
  step 3 (`:124`) drops, at `:171`, `:172`, and `:379`. That file is **not** in 04-04's
  `files_modified` (only 04-03's). `cmd/engram/backfill_test.go` *is* covered, so that half is fine.

### C4-H4 — `partialWriteClassification` set-equality breaks in both directions, in both waves

`TestPartialWritePathsAreClassifiedNonStamping`
(`internal/store/schemaversion_stamp_gate_test.go:698`) scans the **whole `internal/store`
package directory** for every `SetPayload`/`DeletePayload`/`OverwritePayload` site
(`scanPackageDirForCalls(fset, ".", ".go", "_test.go", partialWriteMethods)` at `:701`) and
asserts set equality against `partialWriteClassification` (`:364`) in both directions
(`:714-736`). `internal/store/schemaversion_stamp_gate_test.go` appears in **zero** plan files.

- **Wave 2 (04-02):** the new `internal/store/revert.go` puts a `DeletePayload`-then-`SetPayload`
  pair in `Store.revertWithSteps` — derived sites with no classification entry →
  `"has no classification entry"`.
- **Wave 4 (04-04):** deleting `Store.BackfillShortIDs` leaves its row at `:370` →
  `"stale classification entry"`.

04-02 and 04-04 handle only the *recall* gate (`schemaversion_recallgate_test.go`), never the
*stamp* gate. The table's doc comment also states a literal count ("ten entries") already stale
at 11 rows.

### C4-H5 — manifest apply cannot observe `Spared` as designed

04-01 (`:36`, `:174`, `:209`) specifies that a manifest-limited apply scrolls the backlog and,
for each point, classifies a manifest member no longer below target as `Spared`. But
`Store.Migrate` scrolls `backlogFilter(target)` (`internal/store/migrate.go:136`), which returns
only records whose `schema_version` is `< target` or absent
(`internal/store/migratebacklog.go:58-70`). A previewed record stamped current between preview
and apply is excluded **at the query**, never reaches the point loop, and therefore cannot be
classified. Test 12 (`04-01-PLAN.md:209`) requires exactly that observation. The test and the
implementation contract cannot both pass.

### C4-H6 — the revert reconciliation test asserts mutually exclusive postconditions

04-02 Task 2 step 12 (`04-02-PLAN.md:254-260`) arms a one-shot fault
(`inj.arm(2, 1, faultBeforeInvoke)`) and then asserts, after `revertWithSteps` returns, **both**
that the victim's `schema_version` is still 2 **and** that `res.Backlog == 0`. With
`failCount == 1` the re-derivation loop self-heals the victim before return — the shipped
precedent proves exactly this (`internal/store/migrate_faultinject_test.go:313-360`:
`Failed == 1`, `Passes > 1`, `backlog after self-heal = empty`, victim carries the marker). The
mid-failure state is transient and unavailable to a post-return `rawPayload` read. The test
needs a persistent first-invocation failure, an interceptor-side observation at the failure
boundary, or a write seam that pauses between the two RPCs.

## Actionable non-HIGH concerns

- **C4-M1** — 04-02 Task 3's acceptance gate `! rg -n "Store.Migrate" internal/server/tools.go`
  self-invalidates: the same task's action text mandates a comment saying the function
  "MUST NOT invoke `Store.Migrate`". 04-04 applies exactly this "comment-text discipline" for
  `dry-run`; 04-02 did not. Use a call-shaped pattern (`! rg -n '\.Migrate\(' …`, which does not
  match `.MigrateStatus(`).
- **C4-M2** — 04-03 runs `task surfaces:gen` in Task 1 step 8b, but Tasks 2–3 then add three
  commands; `TestHelpGolden`/`TestCatalogGolden` walk the live tree, so the goldens drift again
  with no scheduled second regeneration. 04-04 Task 2 step 7 gets this ordering right.
- **C4-M3** — 04-03 Task 1 step 9 adds a `destructiveFlagCases` row keyed on `migrateRevertCmd`,
  which Task 3 declares. Task 1's own `<verify>` compiles the package, so Task 1 cannot pass in
  isolation. Move step 9 to Task 3.
- **C4-L1** — 04-02's `Store.Revert` loop is specified as "the same re-derive-per-pass loop shape
  as `Store.Migrate`" without naming the non-shrinking-backlog termination guard
  (`internal/store/migrate.go:167-178`). One sentence closes it.
- **C4-L2** — `Store.MigrateStatus`'s `Facet` call sets no `Limit`; `FacetCounts.Limit` defaults
  to 10, silently truncating a histogram with >10 distinct versions.
- **C4-L3** — `docs-site/src/content/docs/guides/upgrade.md:436-437` (the v0.8.4 section) still
  instructs `engram backfill-short-ids --dry-run`, which exits 2 after 04-04. The D-12 gate pins
  only the *new* entry, so the file self-contradicts while the gate stays green.
- **C4-L4** — 04-04's acceptance command
  `go test ./cmd/engram/ -run 'TestBackfill|TestMigrate' ./internal/store/ -run 'TestRecallEmissionSetIsCompleteAndClassified'`
  (`04-04-PLAN.md:146`) is wrong: `-run` is a package-wide flag, so the second supersedes the
  first and the command package runs no matching tests. Split into two invocations, as the later
  `<verify>` block already does.
- **C4-L5** — 04-01 Task 2's `read_first` cites `internal/store/reindex.go`, which does not
  exist; `Reindex`/`ReindexOptions.DryRun` live in `internal/store/store.go` (the line numbers
  cited are correct for `store.go`).
- **C4-L6** — `Spared` and `Appeared` already exist in `internal/store` as
  `PurgeResult.Spared []string` / `.Appeared []string` (`internal/store/spine.go:1245,1248`) —
  **identity sets**. 04-01 reuses both names in `MigrateResult` as `uint64` **counts**, in a plan
  whose stated thesis is that parity is proven by identity set and not by count equality. Not a
  Go collision; a same-package shape divergence worth a naming decision or an explicit note.

---

## Codex Review

## 1. Ledger adjudication

| Cycle-3 item | Status | Evidence and mechanism |
|---|---|---|
| HIGH #1 — `Store.PreviewRevert` / `store.RevertPlan` had no producer | RESOLVED | 04-02 now explicitly creates both in `internal/store/revert.go` and lists that file in `files_modified`; 04-03 depends on 04-02 and only consumes them ([04-02 plan:123](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:123), [04-03 plan:194](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-03-PLAN.md:194)). |
| HIGH #2 — unsupported-version revert preflight was batch-scoped | RESOLVED | 04-02 requires a separate zero-write `PreviewRevert` pass through the existing cursor-advancing `scrollAllPoints`, plus a 5-record/3-page late-offender test. The iterator advances `NextPageOffset` at [spine.go:46](/Volumes/Code/github.com/seanb4t/engram/internal/store/spine.go:46). |
| M3 — partial multi-RPC inverse reconciliation | PARTIAL | The DeletePayload→SetPayload commit-point design is now specified, but its planned test is internally inconsistent. With a one-shot `inj.arm(2, 1, faultBeforeInvoke)`, the re-derivation loop should self-heal before `revertWithSteps` returns, just as the existing migrate precedent does at [migrate_faultinject_test.go:313](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate_faultinject_test.go:313). The plan nevertheless requires inspecting the returned state and finding the victim still at v2 ([04-02 plan:296](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:296)). That observation cannot coexist with same-call convergence. |
| M4 — future versions collapsed to one scalar | RESOLVED | 04-02 now specifies `Future []VersionBucket`, distinct v2/v42 fixtures, differing counts, sort order, and `FutureTotal`; 04-03 consumes the list in CLI output. |
| N3 — `--timeout` registered but unread | RESOLVED | 04-03 requires duration-taking `migrateWithTimeout(ctx, d)`, distinct `migrateSweepTimeout` / `migrateRevertTimeout`, and behavioral 1s/default/0 tests. 04-04 passes `backfillTimeout` through the shared runners. |
| N1 — sibling conformance gates not widened | RESOLVED | 04-03 introduces `mutatingCommandNames` with a named one-wave exclusion; 04-04 removes it after the alias conversion and widens routing, exact-flag, and rule-sentence gates, including explicit RED-first experiments. This matches the current gates that still use `destructiveCommandNames` at [destructive_test.go:130](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive_test.go:130), [destructive_test.go:200](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive_test.go:200), and [destructive_test.go:248](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive_test.go:248). |
| Backfill alias apply bypassed manifest parity | RESOLVED | 04-03 produces shared `migrateSweepPreviewRun` / `migrateSweepApplyRun`; 04-04 makes both alias closures call them and forbids constructing `store.MigrateOptions` in `backfill.go`. The two-call sequence is compared against canonical `migrate --apply`. |
| 04-01 DryRun count semantics contradictory | RESOLVED | The plan now makes `len(PreviewManifest)` the only projection count and keeps `Migrated == 0`; it explicitly prohibits `WouldMigrate` in `store.MigrateResult`. |
| Rule-sentence update conditional | RESOLVED | 04-03 now mandates the update unconditionally in `rules.go`, updates the pinned test string, and invokes `task surfaces:gen` for generated anchors and goldens. Current source confirms the pin and registry are separate declarations at [rules_test.go:52](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/rules_test.go:52) and [rules.go:231](/Volumes/Code/github.com/seanb4t/engram/internal/surfaces/rules.go:231). |
| Self-found: `migrateTimeout` collision | RESOLVED | Existing package symbol is confirmed at [migrate.go:22](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/migrate.go:22). Revised plans use `migrateSweepTimeout` and `migrateRevertTimeout`. |
| Self-found: `TestTimeoutGroupMatrix` set-equality break | RESOLVED | 04-03 adds both new timeout-bearing paths to `timeoutGroups` and `timeoutGroupCaseArgs`. The current test really is bidirectional set equality at [operator_output_test.go:599](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output_test.go:599). |
| Self-found: wave-3 `!ReadOnly` ordering defect | RESOLVED | `pendingApplyConversion` explicitly excludes `backfill-short-ids` during wave 3 and is owned for deletion by 04-04 after the alias receives `--apply`. |
| Self-found: six-location rule-sentence ripple | RESOLVED | Source declaration, test pin, two generated anchored regions, and two golden files are all named; regeneration is assigned to the sanctioned task. |
| Self-found: recall-emission allowlist rows | PARTIAL | The plans add/remove `operatorMigrationEmitters` rows, including the existing backfill row at [schemaversion_recallgate_test.go:517](/Volumes/Code/github.com/seanb4t/engram/internal/store/schemaversion_recallgate_test.go:517). However, they miss the independent partial-write set-equality row for the same deleted method at [schemaversion_stamp_gate_test.go:370](/Volumes/Code/github.com/seanb4t/engram/internal/store/schemaversion_stamp_gate_test.go:370). |

## 2. New concerns

### HIGH — Manifest apply cannot observe `Spared` records as designed

04-01 says manifest-limited apply scrolls the “entire backlog” and counts manifest members that are no longer below target as `Spared` ([04-01 plan:181](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-01-PLAN.md:181)). But the shipped backlog filter returns only absent or `< target` records ([migratebacklog.go:63](/Volumes/Code/github.com/seanb4t/engram/internal/store/migratebacklog.go:63)). A previewed record stamped current between preview and apply is excluded before the point loop and therefore cannot be classified `Spared`.

The specified drift test requires exactly that observation. The plan needs either:

- a separate lookup/scan for manifest IDs;
- a scan not filtered to the backlog; or
- an explicit post-pass derivation such as manifest IDs minus observed eligible IDs, with defined handling for deleted/missing records.

As written, the test and implementation contract cannot both pass.

### HIGH — Revert reconciliation test requires mutually exclusive postconditions

The planned one-shot fault is automatically retried by the same convergence loop. Existing precedent confirms a single mid-sequence failure self-heals before return ([migrate_faultinject_test.go:337](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate_faultinject_test.go:337), [migrate_faultinject_test.go:360](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate_faultinject_test.go:360)). Yet 04-02 requires both:

- inspecting the returned store and finding the victim still at old version; and
- same-call or subsequent convergence.

With `failCount=1`, the first state is transient and unavailable after return. The test needs a persistent failure for the first invocation, an interceptor-side observation at the failure boundary, or a write seam that pauses between RPCs.

### HIGH — Operator parity gates are not updated for the new commands, and wave 4 deletes symbols they still compile against

`TestOperatorOutputParity` gates its row names bidirectionally against every live operator command ([operator_output_test.go:307](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output_test.go:307)). The revised 04-03 plan adds `migrate`, `migrate status`, and `migrate revert`, but only assigns `operator_output_test.go` work for timeout groups; it never adds parity rows for the three commands.

Worse, 04-04 deletes `backfillSummary` and `backfillReportDoc`, while current parity fixtures reference both at [operator_output_test.go:169](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output_test.go:169) and again in the document-shape rows around [operator_output_test.go:379](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operator_output_test.go:379). 04-04 does not list `operator_output_test.go` in `files_modified`.

Consequences:

- wave 3 full tests fail the bidirectional parity gate for missing migrate-family rows;
- wave 4 does not compile after deleting the legacy formatter symbols.

04-03 must add rows for all three new commands, and 04-04 must migrate the backfill row to the shared migrate formatter/document.

### HIGH — Dead `Store.BackfillShortIDs` leaves another set-equality classification stale

04-04 removes the recall-emission row, but not the independent `partialWriteClassification` row at [schemaversion_stamp_gate_test.go:370](/Volumes/Code/github.com/seanb4t/engram/internal/store/schemaversion_stamp_gate_test.go:370). That table is explicitly derived over every Set/Delete/Overwrite payload emitter and currently documents a fixed entry count at [schemaversion_stamp_gate_test.go:353](/Volumes/Code/github.com/seanb4t/engram/internal/store/schemaversion_stamp_gate_test.go:353).

Deleting the method without removing this row leaves the full suite RED. Add `internal/store/schemaversion_stamp_gate_test.go` to 04-04 ownership and update both the row and count comment.

### LOW — One combined acceptance command does not run the advertised command-package tests

04-04 gives:

```sh
go test ./cmd/engram/ -run 'TestBackfill|TestMigrate' ./internal/store/ -run 'TestRecallEmissionSetIsCompleteAndClassified'
```

The second `-run` is a package-wide flag and supersedes the first; it does not apply only to the following package. Thus the command package is filtered by `TestRecallEmissionSetIsCompleteAndClassified` and runs no matching tests. This appears at [04-04 plan:146](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-04-PLAN.md:146).

Use two `go test` invocations, as the later `<verify>` block already does.

## 3. Per-plan notes

### 04-01

Strong improvements: minter construction, production-registry transition, full-backlog DryRun, projection semantics, and mixed-state compatibility are concrete and testable.

Blocking issue: manifest-limited scanning through the below-target filter cannot produce the promised `Spared` telemetry. The algorithm needs revision before execution.

### 04-02

The missing symbol producer and whole-range preflight defects are convincingly closed. The multi-page test is appropriately non-vacuous.

Blocking issue: the single-failure reconciliation test attempts to inspect a transient state after the method has already self-healed it. The test mechanism must change.

### 04-03

Timeout collision, timeout-group membership, wave-local temporary exclusion, rule regeneration, interface seam, and cross-plan ownership are well handled.

Blocking issue: `TestOperatorOutputParity` is a second bidirectional operator-command registry that the plan overlooks. All three migrate-family rows must land in wave 3.

### 04-04

Alias parity and conformance-gate widening are now structurally sound, and the documentation gate is materially better.

Blocking issues:

- removing legacy backfill formatters breaks existing operator parity fixtures;
- deleting `Store.BackfillShortIDs` leaves the schema-version stamp allowlist stale;
- one acceptance command silently selects the wrong tests.

## 4. Risk assessment

Overall risk: **HIGH**.

The revision resolves both prior HIGH design gaps and most actionable cycle-3 findings, but four executable blockers remain. Two are algorithm/test-contract contradictions; two will make the full Go test suite fail through omitted set-equality/formatter updates.

**Convergence verdict: NO**

CYCLE_SUMMARY: current_high=4 current_actionable=1

---

## OpenCode Review

# Cross-AI Plan Review — Phase 4, Cycle 4

Verified against source at repo root (`1208b945`). Every claimed fix below was checked against the actual code; nothing is taken from the revision's word.

## 1. Ledger adjudication

| Cycle-3 item | Verdict | Evidence and mechanism |
|---|---|---|
| **HIGH #1** — `Store.PreviewRevert`/`store.RevertPlan` produced by no plan | **RESOLVED** | 04-02 Task 2 step 0 declares `type RevertPlan` and step 2 declares `func (s *Store) PreviewRevert`; `internal/store/revert.go` is in 04-02's `files_modified` and its artifact ledger names both as "SOLE producer". 04-03 Task 2 step 0 explicitly consumes-only, with a STOP instruction if `rg -n "func \(s \*Store\) PreviewRevert" internal/store/revert.go` finds nothing. `depends_on: [04-01, 04-02]` on 04-03 is correct. |
| **HIGH #2** — batch-scoped revert preflight | **RESOLVED** | 04-02 Task 2 step 3: `previewRevertWithSteps` enumerates via `s.scrollAllPoints` — verified at `internal/store/spine.go:46-69` that it advances a `*qdrant.PointId` cursor until `next == nil` — as a separate zero-write pass before the write loop, which is unreachable unless `plan.Reversible`. The multi-page test forces `spineScrollBatch = 2` (package var verified at `spine.go:28`; the behavioral-pagination precedent at `reindex_test.go:673` is real), seeds 5 records with the unsupported one on page 3, and asserts all five payloads byte-identical post-refusal. Qdrant's next-page-offset is a point-ID watermark (repo's own claim at `store.go:2741` doc), so "highest-sorting id lands last" is grounded. |
| (a) **M3** — no multi-RPC inverse reconciliation | **RESOLVED** | 04-02 Task 2 step 5 pins DeletePayload-then-one-SetPayload with `schema_version` as commit point, plus test 12 on the shipped injector. Verified the interceptor type-switches on `*qdrant.SetPayloadPoints` **only** (`migrate_faultinject_test.go:176-180`), so DeletePayload passes through — the forced sequence is constructible exactly as planned. `arm`/`seen`/`disarm` exist at `:115`,`:140`,`:128`. |
| (b) **M4** — future versions collapsed to scalar | **RESOLVED** | 04-02 Task 1: `Future []VersionBucket` + derived `FutureTotal`, sorted ascending, test seeds v2 **and** v42 with distinct counts asserting `len(Future) == 2`. Task 3's warning renders the version list. `PointsClient.Facet`/`FacetCounts` verified via `go doc` (see new LOW-7 for a `Limit` caveat). |
| (c) **N3** — `--timeout` registered but never read | **RESOLVED** | 04-03 Task 2 step 0b: `migrateWithTimeout(ctx, d time.Duration)` takes the duration, 0 disables, per-leaf vars `migrateSweepTimeout`/`migrateRevertTimeout`; three-case deadline tests (1s ≤ 2s / default ~5m / 0 → no deadline). 04-04 Task 1 step 2 passes `backfillTimeout` into the same helper. |
| (d) **N1** — sibling conformance gates not widened | **PARTIAL — incorporated but defectively derived** | 04-04 Task 2 does widen all three gates and 04-03 Task 1 re-derives `TestDestructiveCommandsRequireApply` — but the shared derivation `!op.Class.ReadOnly && op.CLICommand != ""` (04-03-PLAN.md:156) is wrong for all four gates. See **New HIGH-1**. |
| (e) **alias apply bypasses manifest parity** | **RESOLVED** | 04-03 Task 2 step 1a produces `migrateSweepPreviewRun`/`migrateSweepApplyRun`; 04-04 Task 1 step 3 makes both alias closures one-line adapters over them, with a test asserting the alias's and `engram migrate --apply`'s recorded `Migrate` call sequences are equal (DryRun→Manifest). `rg -n "store.MigrateOptions" cmd/engram/backfill.go` negative gate is well-formed. |
| (f) **N4** — DryRun count semantics contradictory | **RESOLVED** | 04-01: projection count is `len(MigrateResult.PreviewManifest)`, `Migrated` stays 0 in DryRun, no `WouldMigrate` field, one-sentence doc contract on `MigrateOptions.DryRun`. 04-03 derives `would_migrate` from `len(res.PreviewManifest)` with a pinning test. |
| (g) **N5** — rule-sentence update conditional | **RESOLVED** | 04-03 Task 1 steps 8/8a/8b are unconditional. All six locations verified present: `rules.go:232`, `rules_test.go:56` (pinned `wantSentence`), anchored regions `cli.md:135` and `SKILL.md:379`, `help.golden` ×3 (:85,:119,:260), `catalog.golden` ×3 (:157,:241,:606). `task surfaces:gen` verified at `Taskfile.yaml:244-264` including the `-update -count=1` golden regeneration. |
| **Self-claimed blocker 1** — `migrateTimeout` collision | **VERIFIED REAL; RESOLVED** | `migrateTimeout time.Duration` exists at `cmd/engram/migrate.go:22`, registered at `:239`. The plan's `migrateSweepTimeout`/`migrateRevertTimeout` avoid it. My independent sweep of ~40 newly-declared symbols (types, funcs, vars, test names, filenames) found **zero** other collisions. |
| **Self-claimed blocker 2** — `TestTimeoutGroupMatrix` set equality | **VERIFIED REAL; RESOLVED here — but the sibling gate was missed** | Set equality confirmed at `operator_output_test.go:626-649`; 04-03 step 0c/1a adds both commands to `zero-disables` plus `timeoutGroupCaseArgs` cases plus the cli.md table (verified at `cli.md:374-378`). The same defect class in `operatorInvalidOutputArgs` was **not** caught — see **New HIGH-2**. |
| **Self-claimed blocker 3** — wave-ordering (`pendingApplyConversion`) | **PARTIAL** | The exclusion is real and necessary for `backfill-short-ids` (currently `Destructive:false` at `toolclass.go:194-200`, no `--apply` at `backfill.go:77-81`). But it excludes only that one command while the same derivation sweeps in seven others — see **New HIGH-1**. |
| **Self-claimed blocker 4** — six-location sentence ripple | **VERIFIED REAL; RESOLVED** | All six locations enumerated above under N5. |
| **Self-claimed blocker 5** — recall-gate allowlist rows | **VERIFIED REAL; RESOLVED** | `operatorMigrationEmitters` at `schemaversion_recallgate_test.go:517-565` with the `Store.BackfillShortIDs` row at `:520-524` and a "ten entries" doc at `:517`; the gate at `:666` is set-equality in both directions; `recallEmissionMethods` (`:336-342`) = Query/QueryBatch/Scroll/ScrollAndOffset/Count, so `MigrateStatus`'s `Count` requires a row (planned) and `scrollAllPoints` is already classified (preflight needs none). 04-04 Task 1 step 6a deletes the row with the method. |

## 2. New concerns

### HIGH-1 — `mutatingCommandNames()` = `!ReadOnly` is over-broad: four conformance gates go RED across waves 3–4

**Severity: HIGH** (wave-halting; requires a design decision the plan should have made)

The M12/N1 remediation derives the `--apply`-required set as `!op.Class.ReadOnly && op.CLICommand != ""` (04-03-PLAN.md:156; consumed by 04-04-PLAN.md:182). Enumerating that predicate over the verified `toolclass.go` table, post-wave-3 rows included:

- **mutatingCommandNames() (13):** `store` (:66-72), `reindex` (:162-170), `migrate-remap-owner` (:171-178), `prune-expired` (:179-185), `summarize-missing` (:186-193), `backfill-short-ids` (:194-200), `serve` (:222-224), `migrate-set-owner` (:239-241), `spine-review archive` (:302-304), `spine-review restore` (:311-313), `spine-review purge` (:327-329), + new `migrate`, `migrate revert`.
- **`--apply` carriers after wave 4 (6):** only commands routed through `registerDestructive` — verified its only callers are `prune.go:159`, `spine_review_purge.go:425`, `migrate.go:257`, plus the planned `migrate`, `migrate revert`, `backfill-short-ids`. The `--apply` flag exists nowhere else (`rg '"apply"' cmd/engram/*.go` hits only `destructive.go:62` and tests).

Consequences, wave by wave:

1. **Wave 3, `TestDestructiveCommandsRequireApply`** (04-03 Task 1 step 6): want = 12 (13 minus `pendingApplyConversion{backfill-short-ids}`), got = 5. First direction fails on **7 commands**: `store`, `reindex`, `summarize-missing`, `serve`, `migrate-set-owner`, `spine-review archive`, `spine-review restore`. Task 1's own `<verify>` runs `-run 'TestDestructive|…'`, so the executor hits this immediately.
2. **Wave 4, `TestDestructiveCommandsRouteThroughGate`** (04-04 Task 2 step 2): those same 7 commands' RunE is not the `registerDestructive` closure → 7 errors.
3. **Wave 4, `TestDestructiveCommandsExactFlagSet`** (step 4): the 7 have no `mutatingFlagCases` row → forward-direction failure.
4. **Wave 4, `TestApplyFlagUsageComposesRuleSentence`** (step 3): the 7 have no `--apply` flag → `t.Errorf("%s: no --apply flag")` ×7.

`pendingApplyConversion` covers only `backfill-short-ids`, which shows the authors modeled "mutating" as "the preview/apply tier ∪ backfill" and never enumerated what `!ReadOnly` actually selects. Both cycle-3 reviewers marked M12 RESOLVED by checking only the *second* direction (`migrate` enters the want-set); neither enumerated the first direction. The 04-04 RED-first observation requirement is also undermined: the gates are already red, so the planned deliberate-violation experiments prove nothing.

**Fix direction:** the registerDestructive-routed set is not expressible from the table (no tier column). The same package already has the right precedent — `operatorCommandExclusions` (`cmdwalk.go:82-97`), a small NAMED set with per-entry justification and a pinning test. Derive `want = destructiveCommandNames() ∪ {"migrate", "backfill-short-ids"}` as a named additive-routed set (with `pendingApplyConversion` still subtracting `backfill-short-ids` at wave 3 only), and iterate that set in all four gates.

### HIGH-2 — `operatorInvalidOutputArgs` has no rows for the three new commands: `TestEveryOperatorCommandRejectsInvalidOutput` fails at wave 3

**Severity: HIGH** (same "executable blocker" class the revision counted for `TestTimeoutGroupMatrix`; the sibling instance was missed)

`operatorCommands()` (`cmdwalk.go:101-117`) includes every RunE-bearing command without a `server` flag outside `{serve, version}` — so `migrate`, `migrate status`, and `migrate revert` all enter at wave 3. `TestEveryOperatorCommandRejectsInvalidOutput` (`operator_output_test.go:570-583`) calls `operatorInvalidOutputArgs(t, name)`, whose default branch is `t.Fatalf("operatorInvalidOutputArgs: no row defined for command %q")` (`:560`). No plan mentions this gate (grep over all four plans: zero hits). Three failing subtests at wave 3's `task` run. Fix is mechanical — three rows mirroring the existing ones (`["migrate"]`, `["migrate","status"]`, `["migrate","revert","--to","0"]`) — and `operator_output_test.go` is already in 04-03's `files_modified`, but the plan must say it.

### MEDIUM-3 — 04-02 Task 3's `! rg -n "Store.Migrate" internal/server/tools.go` gate self-invalidates against the plan's own mandated comment language

**Severity: MEDIUM.** The action text instructs the executor to write that the function "MUST NOT invoke `Store.Migrate` under any circumstance" — and this repo's doc-comment culture (e.g. `warnOwnerlessRecords` at `tools.go:455-482`, the sweep's 40-line doc at `migrate.go:55-94`) makes a comment containing the literal `Store.Migrate` near-certain. The pattern matches that comment and the gate goes RED on the compliant implementation. The 04-04 plan itself recognizes this exact hazard class for `dry-run` in `backfill.go` ("comment-text discipline") but 04-02 did not apply it. Fix: grep a call-shaped pattern instead, e.g. `! rg -n '\.Migrate\(' internal/server/tools.go` (which does not match `.MigrateStatus(`).

### MEDIUM-4 — Golden regeneration is sequenced before the commands it must capture (04-03)

**Severity: MEDIUM.** Task 1 step 8b runs `task surfaces:gen`, but Tasks 2–3 then add three commands; `TestHelpGolden`/`TestCatalogGolden` walk the live tree (`golden_test.go:80,106`), so the goldens drift again and the plan-level verification (`-run '…|TestHelpGolden|TestCatalogGolden'`) goes RED with no scheduled second regeneration. Fix: move or repeat the `surfaces:gen` step after Task 3 (04-04 Task 2 step 7 already does this correctly for wave 4).

### MEDIUM-5 — 04-03 Task 1 step 9 references `migrateRevertCmd`, which Task 3 declares

**Severity: MEDIUM.** The `destructiveFlagCases` row `{migrateRevertCmd, …}` is added in Task 1, but the var is created in Task 3. Under per-task verify (Task 1's `<verify>` compiles the package), Task 1 cannot compile in isolation. Fix: move step 9 to Task 3 (whose own toolclass row for `migrate revert` lands there anyway).

### LOW-6 — `Store.Revert`'s loop carries no stated PA-3-analog termination guard

04-02 Task 2 step 5 says "the same re-derive-per-pass loop shape as `Store.Migrate`" but never names the non-shrinking-backlog guard (`migrate.go:167-178`). With a persistently failing write and a live context, a store-level caller spins forever (the CLI path is bounded by `migrateWithTimeout`). An executor mirroring the loop will probably include the guard — but the plan is silent where it is elsewhere meticulous. One sentence closes it.

### LOW-7 — `MigrateStatus`'s `Facet` call doesn't set `Limit` (default 10)

Verified via `go doc`: `FacetCounts.Limit` — "Max number of facets. Default is 10." A collection with >10 distinct `schema_version` values silently truncates the histogram. Unreachable in practice (each version requires a registered step), and the sum-invariant test would catch it if ever exercised — but one line (`Limit: qdrant.PtrOf(...)`) removes the latent trap.

### LOW-8 — The v0.8.4 section of `upgrade.md` keeps recommending the removed flag

`docs-site/src/content/docs/guides/upgrade.md:436-437` currently instructs: `engram backfill-short-ids --dry-run  # run this first` and `engram backfill-short-ids  # apply to all memories`. After 04-04, the first line exits 2 (unknown flag) and the second previews instead of applying. The D-12 gate pins only the *new* entry, so the file will self-contradict while the gate stays green. A one-line pointer in the v0.8.4 section (or refreshing the examples) closes it.

## 3. Per-plan notes

- **04-01:** Clean. The CheckAdditive carve-out is correctly scoped to the `missing` branch only (`additive.go:87-91`); undeclared/removed branches stay intact. Registry literal + `PHASE4` marker (`registry.go:16,30`), `CurrentVersion` doc forbidding a standalone bump (`migrate.go:42-44`), the `len(Registry) != 0` fatal at `additive_test.go:40-42`, and the pin at `migrate_test.go:16-19` all verified as described. The bare-record default-target test covers the research's PA-10a item 3 (default `MigrateOptions{}` now sweeps at `CurrentVersion=1` rather than no-oping via the `target <= 0` short-circuit at `migrate.go:128`). Cosmetic: the read-first cites "internal/store/reindex.go" — the file doesn't exist; ReindexOptions.DryRun is at `store.go:3010-3020` (line numbers right, filename wrong).
- **04-02:** The two cycle-3 HIGHs are genuinely closed; the `StepsFrom(steps, to, from)` pinning is mechanically correct against `registry.go:102-127`, and the grep pair is well-formed (the negative pattern does not false-match the `revertStepsFrom` declaration). One residual ambiguity: the inverse write contract's "for each inverse ApplyFunc result, take the key-set delta" doesn't state whether removed/added sets accumulate across a multi-step chain into one per-record DeletePayload+SetPayload pair (the "exactly ONE SetPayload" wording implies per-record; in-scope inverses make the readings equivalent). MEDIUM-3 and LOW-6/LOW-7 are its remaining items.
- **04-03:** HIGH-1 (Task 1 step 6), HIGH-2 (unhandled `operatorInvalidOutputArgs`), MEDIUM-4, MEDIUM-5. The H5 in-closure re-preview, the `migrateTimeout` collision avoidance, the timeout-matrix handling, and the N5 sentence propagation are all verified correct — the wave-3 CLI wiring is fine; it's the conformance-test derivations that break.
- **04-04:** The alias-parity fix (cycle-3 #7) is structurally sound and the recall-gate row deletion is correctly paired with the method deletion. HIGH-1's wave-4 half (Task 2's three widened gates) and LOW-8 are its items. The `backfill/unreachable-qdrant` exit-baseline row (`exitcode_baseline_test.go:319-326`) survives the alias conversion (same StoreFromEnv→classify path), and the 37-row pin doesn't force new rows — no break there.

## 4. Risk assessment

**Overall: MEDIUM-HIGH.** The phase's load-bearing mutation paths — the minter-injected sweep, the manifest-bridged apply, the whole-range revert preflight, the commit-point inverse write — all verified sound against shipped source; every cycle-3 item except N1 is genuinely closed, and the five self-claimed blockers are all real (one partially). The remaining risk is concentrated in the CLI conformance-test layer: two wave-3 blockers (HIGH-1, HIGH-2) that no executor can pass while following the plans literally, both failing loudly with self-describing messages, both with mechanical fixes once HIGH-1's derivation decision is made.

**Convergence verdict: NO** — two HIGH-severity plan defects remain; both are one-paragraph plan edits (re-derive the gate set per the `operatorCommandExclusions` named-set precedent; add three `operatorInvalidOutputArgs` rows). With those plus the four MEDIUM sequencing/gate-hygiene edits, cycle 5 should converge.

CYCLE_SUMMARY: current_high=2 current_actionable=6

---

## Verification coverage

Both lanes ran with repo file access and cited `file:line` evidence; neither output carries the
`[reviewed-without-repo-access]` marker, so both verdicts count at full consensus weight.
OpenCode's first invocation was a runner-timeout kill, not a crash — it was re-run to completion
against the identical prompt, so no lane was dropped from this cycle.

Every HIGH recorded above was re-verified against shipped source during synthesis rather than
accepted from a reviewer's assertion. Two findings (the `wantOperatorCommandKeys` registry, and
both halves of the `partialWriteClassification` break) were surfaced only by that independent
sweep and appear in neither reviewer's output.

Source files independently read while adjudicating this cycle:

| File | Cited for |
|---|---|
| `cmd/engram/cmdwalk.go:63-117` | `operatorCommands()` predicate and `operatorCommandExclusions` named-set precedent |
| `cmd/engram/cmdwalk_test.go:108-165` | `wantOperatorCommandKeys` and `TestOperatorCommands`' bidirectional set equality |
| `cmd/engram/operator_output_test.go:137-190,258-330,368-395,518-583,599-649` | `operatorParityRows`, the parity gate, the empty-doc map, `operatorInvalidOutputArgs`, `TestTimeoutGroupMatrix` |
| `cmd/engram/destructive_test.go:20-36,84-106,130-152,200-220,231-270` | `destructiveCommandNames` derivation and the four conformance gates |
| `cmd/engram/destructive.go:50-63,123` | `addApplyFlag` usage composition; `registerDestructive`'s apply registration |
| `cmd/engram/migrate.go:20-24,257` | the `migrateTimeout` collision and `registerDestructive`'s third caller |
| `cmd/engram/backfill.go:21,51,59,73,80` | preserved `--timeout`; `backfillSummary`/`backfillReportDoc` definitions |
| `cmd/engram/spine_review_purge.go:107,330,365-425` | purge precedent for in-closure preview, flag-backed timeout, `registerDestructive` |
| `internal/store/migratebacklog.go:58-80` | `backlogFilter`'s `Lt`/`IsEmpty` arms — the C4-H5 mechanism |
| `internal/store/migrate.go:124-210` | `backlogFilter` use, PA-3 guard, one-batch-per-pass loop |
| `internal/store/migrate_faultinject_test.go:300-365` | `TestMigratePartialFailureResume` self-heal semantics — the C4-H6 mechanism |
| `internal/store/schemaversion_stamp_gate_test.go:353-375,698-736` | `partialWriteClassification`, whole-package-dir scan, bidirectional equality |
| `internal/store/store_test.go:5741-5893` | the 7 `st.BackfillShortIDs` call sites |
| `internal/store/spine.go:28,46-69,1245-1248` | `scrollAllPoints` cursor advance; `PurgeResult.Spared/Appeared` shapes |
| `internal/surfaces/toolclass.go:44-350` | the `Class` table `mutatingCommandNames()` derives from |
| `internal/surfaces/rules.go:232` | `RuleDestructiveRequiresApply.Sentence` |
| `cmd/engram/catalog_test.go:422-452` | catalog↔toolclass bidirectional gate |

Claims verified by execution rather than reading: the `!op.Class.ReadOnly && op.CLICommand != ""`
predicate was run against the live `surfaces.Operations()` table with a throwaway test, producing
the exact 11-command membership that grounds C4-H1.
