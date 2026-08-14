---
phase: 04
reviewers: [codex, opencode]
reviewed_at: 2026-08-14T22:18:03Z
plans_reviewed:
  - 04-01-PLAN.md — Tracer: v0→v1 short_id first customer
  - 04-02-PLAN.md — Store MigrateStatus histogram + PreviewRevert/Revert + startup warning
  - 04-03-PLAN.md — CLI surface: migrate command family
  - 04-04-PLAN.md — backfill-short-ids as thin delegating alias
cycle: 4
plans_revision: 1208b945
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
