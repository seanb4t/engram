---
phase: 04
reviewers: [codex, opencode]
reviewed_at: 2026-08-15T02:33:02Z
plans_reviewed:
  - 04-01-PLAN.md — Tracer: v0→v1 short_id first customer
  - 04-02-PLAN.md — Store MigrateStatus histogram + PreviewRevert/Revert + startup warning
  - 04-03-PLAN.md — CLI surface: migrate command family
  - 04-04-PLAN.md — backfill-short-ids as thin delegating alias
cycle: 7
plans_revision: ff935731
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
cycle_6_verdict:
  resolved_high: [C5-H1-flagship-gate, C5-H3, C5-H4]
  partial_high: [C5-H1, C5-H2, C5-H5]
  resolved_actionable:
    [C5-M1, C5-M3, C5-M4, C5-M6, C5-M7,
     C5-L2, C5-L3, C5-L4, C5-L5, C5-L6, C5-L7, C5-L8, C5-L9, C5-L10, C5-L11]
  partial_actionable: [C5-M2, C5-M5, C5-L1]
  new_high: [C6-H1, C6-H2, C6-H3, C6-H4, C6-H5, C6-H6, C6-H7]
  new_actionable:
    [C6-M1, C6-M2, C6-M3, C6-M4, C6-M5, C6-M6, C6-M7, C6-M8, C6-M9, C6-M10, C6-M11, C6-M12,
     C6-L1, C6-L2, C6-L3, C6-L4, C6-L5, C6-L6, C6-L7, C6-L8, C6-L9, C6-L10]
  current_high_count: 7
  current_actionable_count: 22
  converges: false
cycle_7_verdict:
  resolved_high: [C6-H1, C6-H2, C6-H3, C6-H4, C6-H5, C6-H6, C6-H7]
  resolved_actionable:
    [C6-M1, C6-M2, C6-M3, C6-M4, C6-M5, C6-M6, C6-M7, C6-M8, C6-M9, C6-M10, C6-M11, C6-M12,
     C6-L1, C6-L2, C6-L3, C6-L4, C6-L5, C6-L6, C6-L7, C6-L8, C6-L9, C6-L10]
  new_high: []
  new_actionable: [C7-M1, C7-M2, C7-M3, C7-M4, C7-M5, C7-L1]
  current_high_count: 0
  current_actionable_count: 6
  converges: true
  reviewer_verdicts:
    codex: "high=0 actionable=3"
    opencode: "high=0 actionable=2 — READY TO EXECUTE"
---
# Cross-AI Plan Review — Phase 4 (Cycle 7)

**Cycle 7 is the decisive convergence review.** No further review cycles were authorised. The
plans were revised in `ff935731` to attack the four root causes cycle 6 identified. This cycle
re-ran the three class sweeps independently against the plans as they now stand, re-verified
every cycle-6 HIGH against shipped source, re-audited the ledger's boundary section for a third
load-bearing exclusion, and sampled all 22 cycle-6 actionables.

**Verdict: the four root causes are genuinely fixed. Zero HIGH concerns remain.** Both external
reviewers independently reach `high=0` and OpenCode records an explicit **READY TO EXECUTE**.
Six actionable non-HIGH concerns are open, all newly surfaced by this cycle's deeper verification.
All six are bounded, local edits — a reworded predicate, a qualified invariant sentence, three
named prose sites, one ledger row plus a step, and one gate re-anchor. None is structural, none
requires re-planning, and none re-opens a closed class.

---

## Root-cause verification (the four claims of `ff935731`)

### Root cause 1 — the comment-stripping idiom: **ELIMINATED**

Independent sweep over all four plans for `rg -v`, `grep -v`, `sed '/…/d'` and awk comment
stripping returns **zero occurrences in any executable gate**. The idiom survives in exactly
three places, all of them prohibitions forbidding it: `04-01:98` (INV-2, phase-wide), `04-01:64`
(must_have), `04-04:67` (must_have).

All 19 negative gates were individually shape-classified and executed against the current tree.
Every one is DECLARATION-, CALL-, COMPOSITE-LITERAL-, ROW-, STRING-LITERAL- or IMPORT-shaped,
with the sole exception of two `docs-site` prose-literal gates — and prose is the correct shape
for a prose file. Ten gates are currently RED and go GREEN on completion (verified by execution,
e.g. `^\s*Target:\s*1,` matches `internal/store/migrate_converge_test.go:180` today); the
remainder are prohibition guards, each paired in the same `&&` chain with a positive gate
carrying the progress claim.

**No gate's search scope includes a file where the plan mandates writing the banned token.** The
two nearest misses are both already handled by the plan itself: `Store.Migrate` in `tools.go` is
mandated in *bare* form against a *call-shaped* gate (`04-02:72`), and the whole-file `--dry-run`
greps over `upgrade.md` were deliberately deleted in favour of section-scoped Go assertions
precisely because they would have contradicted the D-12 gate (`04-04:367`, `:382`). Also
verified: no `rg -c`-as-count anywhere (every count uses `rg -o … | wc -l` per INV-3); no `head`
masking exit status; no `git diff --stat`.

### Root cause 2 — INV-1 made structural: **VERIFIED, with one documented exception**

All twelve tasks (3 per plan × 4 plans) carry a bare unfiltered package-level
`go test ./<pkg>/ -count=1` as the first clause of their `<automated>` block. No task's only
`go test` carries a `-run`. Seven of the twelve carry two or three unfiltered package runs.

The substantive question — does any task leave the tree in a state its own unfiltered run would
reject? — resolves to **exactly one task, and the plan says so outright**. `04-01` T1 raises
`migrate.CurrentVersion` 0→1 and gates only `internal/migrate`; `04-01:221-227` states that this
"deliberately reds two shipped `internal/store` tests (ledger rows E1/E2); Task 2 — the very next
task, same wave — repairs them", and its `<done>` closes with "`internal/store` is knowingly RED
until Task 2."

The blast-radius accounting behind that exception was independently re-derived and is **accurate
and complete**. The two named RED sites are real (`internal/store/migrate_test.go:321-330`, the
PA-4 tail; `internal/store/migrate_converge_test.go:152-176`, the laggard control, which reds
because `payload()` at `store.go:646` stamps `max(1,0)`). Everything else is self-adapting and
stays green — confirmed by tracing `runCompatRow` (`schemaversion_compat_test.go:250-340`, derives
`expectedRowCount`/`rows`/`olderThanCovered` from `migrate.CurrentVersion` and raw-injects its
own `schema_version`), `TestPayloadRoundTripsSchemaVersion` (`store_test.go:2985-3050`, every
expectation derived), `schemaversion_stamp_test.go:169-212` (raw-injects the key-absent shape),
`TestMigratePartialFailureResume` (`migrate_faultinject_test.go:310+`, explicit `Target: 1` and a
fixture `markerStep(0,1,…)`), and all of `internal/server` (`schemaversion_wire_test.go:32` uses
`migrate.CurrentVersion + 1`).

Because the exception is stated in PLAN.md rather than hidden, it is **not counted as an
actionable finding** under this cycle's definition. It is recorded here because Codex raised it
independently and because the phase-wide invariant's *wording* ("every task leaves a green tree")
is false for this one task — the honest wording is "every task's own package(s)".

### The 04-03 re-sequencing: **CORRECT, verified per test**

At the end of `04-03` T2, all seven named gates pass. Each was resolved by reading the shipped
test body and determining what it derives its expected set from:

| Test | Derives from | End of T2 | Mechanism |
|---|---|---|---|
| `TestOperatorOutputParity` (`operator_output_test.go:310-345`) | rows gated **bidirectionally** against `commandKeySet(operatorCommands())` at `:317-327` | **PASS** | T2 step 6a(b) adds one row each for `migrate` and `migrate status`, each one result value through both formatters |
| `TestOperatorCommands` (`cmdwalk_test.go:154-184`) | **hardcoded** `wantOperatorCommandKeys` (`:117-130`), bidirectional, plus an exclusion-set pin (`:168-176`) | **PASS** | T2 step 6a(a) adds both keys; both satisfy the live predicate at `cmdwalk.go:101-116` |
| `TestEveryOperatorCommandRejectsInvalidOutput` (`operator_output_test.go:570-583`) | iterates `operatorCommands()`; `t.Fatalf` default at `:560` | **PASS** | T2 step 6a(c) adds both arg vectors; step 3 also fixes body order so `operatorOutputFormat` runs **before** `migrateFamilyStoreFromEnv()`, else the subtest gets `exitUnavailable` not `exitUsage` |
| `TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs` (`cmdwalk_test.go:194-229`) | decodes the **committed** `testdata/catalog.golden` and set-equates | **PASS — via step 6c** | this is the one that fails from a stale *golden*, not a stale list; T2 step 6c mandates `task surfaces:gen` as T2's final step |
| `TestHelpGolden` (`golden_test.go:290-301`) | walks the live tree vs `testdata/help.golden` | **PASS** | step 6c regeneration |
| `TestCatalogGolden` (`golden_test.go:306-311`) | `buildCatalog(rootCmd)` vs `testdata/catalog.golden` | **PASS** | step 6c regeneration |
| `TestDestructiveCommandsRequireApply` (`destructive_test.go:88-106`) — **both directions** | A: every `want` key has a live `--apply`; B: every live `--apply` carrier is in `want` | **PASS both** | live table enumerated: `toolclass.go` has exactly three `Destructive:true` rows with non-empty `CLICommand` (`:172/:177`, `:180/:184`, `:327/:328`; those at `:99,:108,:115` are MCP-only). End of T2: `mutatingCommandNames()` = 4; live `--apply` carriers = the three shipped `registerDestructive` callers (`prune.go:159`, `spine_review_purge.go:425`, `migrate.go:257`) + T2's `migrateCmd` = 4. **Exact set equality.** |

Sixteen further same-package gates the plan does not name were also checked and are green at end
of T2, because they derive from the live tree and T2 lands command + toolclass row together —
including `TestCatalogBlastRadiusMatchesToolClasses`, `TestSurfaceConformanceCobraUsage`,
`TestDestructiveCommandsExactFlagSet` (T2 correctly adds **no** `destructiveFlagCases` row for
`migrate`, which is `Destructive:false`), and `TestTimeoutGroupMatrix` (T2 step 0c explicitly
forbids adding `"migrate revert"`, whose command does not exist until T3 — the reverse-direction
trap is correctly identified).

The sharpest sequencing call in the phase is also correct: `04-04` T1 deletes
`pendingApplyConversion` **in T1, not T2**, because adding `--apply` to `backfill-short-ids`
immediately fires `TestDestructiveCommandsRequireApply`'s second direction
(`destructive_test.go:101-104`).

### Root cause 3 — self-referential bookkeeping deleted: **VERIFIED**

`rg -o '04-0[1-4]:[0-9]+'` over all four plans returns **zero** matches: no line anchor into a
PLAN document survives. Gate/file/form self-counts, ledger methodology history and cycle-N
changelogs are gone; where a count would have been re-authored, the text now names the shape
instead (`04-01:64`) or defers to the ledger (`04-01:263`). INV-4 forbids reintroduction.

A side effect worth recording: because the cycle-N changelogs were deliberately removed, closure
of a cycle-6 actionable had to be verified by finding its **substantive incorporation** into a
task's action / acceptance criteria / `<verify>` / `<files>` / must_have / prohibition. That is
how all 22 were checked below.

### Root cause 4 — `.planning/**` promoted to ledger section F: **VERIFIED, and a third exclusion found**

F1/F2 (`04-01:785-788`) now own `TestActiveMilestoneKeyLinksSatisfiable`; the `sweep_test.go`
exclusion is re-scoped to that file alone with an explicit "`gate_test.go` IS IN SCOPE"
(`04-01:810`); `internal/keylinks` is removed from the untouched-packages exclusion (`:805`); the
constraint is carried as `04-01` T3 step 13b with `go test ./internal/keylinks/ -count=1` in T3's
`<automated>`. Anchors verified live: `gate_test.go:93` = `TestNoEscapedPatternsRepoWide`,
`:128` = `TestActiveMilestoneKeyLinksSatisfiable`.

**A third load-bearing exclusion does exist** — see C7-M3 below.

---

## Cycle-6 HIGH closure (C6-H1 … C6-H7), verified against shipped source

| ID | Verdict | Evidence |
|---|---|---|
| **C6-H1** — negative-gate class not closed | **FULLY CLOSED** | Root cause fixed rather than the four instances (see above). (a) → `^\s*Target:\s*1,` composite-literal gate, non-vacuous (matches `migrate_converge_test.go:180` today). (b) → count gate replaced by a positive `rg -n 'MigrateOptions\{Target: -1\}'`. (c) → `! rg -n '&migrateTimeout' cmd/engram/migrate_family.go`, now present in acceptance, **both** `<automated>` blocks and the phase `<verification>` — no bare form survives. (d) → `! rg -n '^var .*PreviewManifest'`. The C5-L1 reinstatement is fixed: `{0,40}` replaced by a clause boundary `--timeout[^.;]*\b(removed\|no longer accepted\|dropped)\b`, verified empirically — the permitted sentence does **not** match (rc=1), the prohibited claim does (rc=0). |
| **C6-H2** — forward-reference / false acceptance evidence | **FULLY CLOSED** | Fix option 1 taken. `04-03` T2 steps 6a–6c land registry rows A1/A2/A3/A7/A22, `TestApplyRoutedAdditionsArePinned`, the `TestDestructiveCommandsRequireApply` switch **and** the golden regeneration in the same task as `migrate`/`migrate status`; only `migrate revert`'s rows are in T3. The claim at `04-03:556` is now true rather than asserted. |
| **C6-H3** — `MigrateResult.Migrated` type | **FULLY CLOSED** | `slices.Contains(res.Migrated…)` is gone (zero hits in any plan). Shipped type confirmed `Migrated uint64` (`internal/store/migrate.go:49`). Parity now proven by stored-payload probes plus the cardinality identity `res.Migrated == uint64(len(previewManifest) - len(res.Spared))` — compiles (int subtraction then conversion) and is sound since `Spared = manifest \ observed`. The payload probe is **discriminating**: the fixture stamps only `schema_version`, so the spared record carries no `short_id`, and a migrating apply would mint one (`payload()` writes `short_id` only when non-empty, `store.go:658-660`). No int/uint64 mismatch anywhere in the chain. |
| **C6-H4** — adapter vs. grep contradiction | **CLOSED for execution** (stale prose remains) | The blocking gate is gone, replaced by a count pin: `test "$(rg -o 'migrateSweep(Preview\|Apply)Run\(ctx, cmd, backfillOutput, backfillTimeout\)' cmd/engram/backfill.go \| wc -l)" = 2` (`04-04:232`, live at `:250`). It pins what it claims — `04-03:339-340` declares exactly those 4-arg signatures, `04-04:151-152` mandates the exact call text; remove an adapter → 1 → RED, add one → 3 → RED, refactor the file away → 0 → RED. Residue: `04-04:473` still describes `migrateWithTimeout(ctx, backfillTimeout)`, the call form step 3 forbids. Prose only; no gate reads it; the action text at `:149` is emphatic and correct. |
| **C6-H5** — PA-11a violation | **FULLY CLOSED** | `seedLegacyRecordNoFatal` does not exist in shipped source; the plan creates it (`04-01:333-340`) error-returning, no `*testing.T`, routed into `h.recordErr`, mirroring the shipped `rawPayloadNoFatal` (`migrate_converge_test.go:452`, doc at `:448-451` citing PA-11a). It addresses the stated failure mode: shipped `seedLegacyRecord` (`migrate_test.go:47-64`) calls `t.Fatalf` at `:58`/`:62`. PA-11a is now in T2's `<read_first>` and the acceptance grep is re-pinned to the new name. The paired negative guard is argument-scoped so it does not catch the legitimate seeding loop at `migrate_converge_test.go:111` (verified rc=1 today). |
| **C6-H6** — ledger `.planning/**` boundary | **FULLY CLOSED** | See root cause 4. Additionally verified: a third phase-3 key link (`pattern: 'PHASE4'`, `03-02-PLAN.md:84`) is anchored in a file `04-01` T1 edits, but is separately covered by ledger row C2 and explicit preserve-the-marker instructions at `04-01:189`, `:208`, `:216`. Effective coverage is complete. |
| **C6-H7** — M10 has no harness | **CLOSED for execution** (stale prose remains) | Premise verified independently: `rg -l 'testcontainers\|StartQdrant\|dockertest'` returns `internal/store/*`, `internal/server/tools_test.go`, `internal/e2e/harness_test.go`, `internal/retrievaleval/*` — **nothing under `cmd/`**. Downgrade option 2 taken, and the composition genuinely covers M10: half one (`04-01:496`, T3 step 10) seeds `short_id`-present / `schema_version`-absent against real pinned Qdrant and asserts `short_id` unchanged + `schema_version == 1` — constructible there, since `Memory.ShortID` is written verbatim by `payload()` (`store.go:658-660`) and `deleteRawPayloadKeys` strips `schema_version`. Half two is the alias call-sequence equality. Residue: `04-04:434`/`:436` (threat model T-04-08b) still read "proves this with `--apply` against real Qdrant" — the claim the downgrade retracts. Prose only. |

**Net: 5 of 7 fully closed; 2 closed for execution purposes with prose residue that no gate reads
and no action text contradicts.** Neither residue is counted as actionable — an executor follows
the emphatic step text, not a stale artifact-list line — but both are named here so a future
reader does not mistake them for live claims.

---

## Ledger boundary re-audit — a third load-bearing exclusion **was** found

Every remaining excluded item was traced to what it refers to in the repo and tested empirically
against the phase's changes.

**Correctly excluded** (with the evidence that nothing breaks):

- **Registries in untouched packages.** No `_test.go` in `internal/{authz,summarize,embed,webauth,telemetry,auth,shortid,openaiurl,retrievaleval,testhttp,e2e}` performs a cross-package scan. `ui/src` has zero CLI-command references. The stated scope list omits `internal/surfacesgen` and `internal/e2e`, both genuinely reachable — but both check out: `surfacesgen/main.go:100-103` maps `RuleDestructiveRequiresApply` to exactly `guides/cli.md` + `curating-memory/SKILL.md` (identical to ledger row D2, no third target), and `renderToolBlastRadius` (`:146-161`) `continue`s on `op.MCPTool == ""`, so the three new CLI-only toolclass rows emit nothing and the CI `surfaces` drift job stays clean.
- **Deleting a test function.** The row's stated rationale ("no registry is keyed on test-function names") is **false** — `03-05-PLAN.md:52` declares `pattern: 'TestEveryFullWriteMethodStampsSchemaVersion'`, live-evaluated by F1's gate. All 44 `pattern:` fields under `.planning/phases` were checked; it is the only test-name-keyed one, it is not among the five tests `04-04` touches, and `CheckSatisfiable` (`internal/keylinks/keylinks.go:313-337`) falls back to the `to` file which defines it. Not load-bearing here.
- **Runtime/behavioural invariants with no static registry** (PA-2, PA-5a, the `int(target)` cast, PA-11a) — re-running the S1–S9 shapes surfaces no set-equality, directory scan, `t.Fatalf` default or hand-maintained literal keyed on any of them.
- **`buf`/proto codegen drift.** No plan's `files_modified` contains `proto/**`; `surfacesgen` implements no proto anchor for the changed rule sentence; CI's `git diff --exit-code -- proto/ docs-site/ skill/ gen/ …` (`ci.yaml:286-292`) is satisfied by rows D2 + A11; A21's `MCPTool: ""` constraint keeps `renderToolBlastRadius` byte-identical.
- **Helm chart invariants.** `rg 'backfill|migrate|schema_version|reindex|prune-expired' charts/` returns four comment lines, all about `reindex`/`migrate-remap-owner`, none falsified. No new `ENGRAM_` field, so the `engram.containerEnv` checksum pin (`Taskfile.yaml:204`) is inert.
- **The ARCHIVED-milestone key-link sweep.** `collectV013Phase12Links` (`sweep_test.go:94-100`) walks only the archived root, and the live/archived split is itself pinned by `TestGateScopesAreDistinct` (`gate_test.go:138+`).
- **Lint-only regressions**, as to `golangci-lint unused`.

**Load-bearing — see C7-M3.** The `Prose accuracy in files no test reads` exclusion fails its own
survival criterion at a third site.

Two further observations that are **not** findings: `.rumdl.toml:29-30` excludes `docs-site` and
`.planning` outright, so four plan assertions that "`task lint` runs rumdl over the edited
docs-site markdown" (`04-02:319`, `04-03:620`, `04-04:401`, `:450`) are false — nothing fails
because of it, and the genuinely rumdl-exposed file is `skill/engram/skills/curating-memory/SKILL.md`
(rewritten by `04-03` T1's `surfaces:gen`). And ledger row A18's trigger description understates
`TestUpgradeGuideNamesEveryChangedCommand`, which re-checks **every** `changes:true` row against
the current `## Unreleased` text on every run and ignores `landed` — mitigated because `04-04` T3's
`<automated>` runs it by name.

---

## `-run` selector resolvability — converged

62 `go test <pkg> -run <selector>` invocations across the four plans, covering 51 distinct test
tokens, were each resolved to one of: exists in shipped source, created by an earlier task in
wave order, or created by the same task. **Zero broken selectors — nothing matches nothing.**
Cross-package consistency is clean: every (package path, test name) pair is correct, and no
selector reaches for a name defined in a package it does not run.

Over-matching was checked and is deliberate in all three cases where it occurs
(`TestDestructive` → 5 shipped, `TestCatalogGolden` → 2, `TestExitCodeBaseline` → 4; the last is
explicitly reasoned about in `04-03`'s T3 acceptance, which relies on `TestExitCodeBaselineRowCount`'s
`wantRows = 37` pin being reached). No selector matches **fewer** tests than intended — no typos,
no case errors, no stale names from earlier cycles.

Residual consistency defects against the plans' own INV-3, all **cosmetic** (a superset selector
loses no coverage, and 04-04 authors no `TestMigrateFamily*` test of its own, so the vacuity
failure mode INV-3 defends against cannot bite): `04-04:229` and `:447` use the forbidden bare
`TestMigrate` form in `cmd/engram`, as does `04-03:619`'s own `<verification>` block;
`04-04:449` gives `./internal/migrate/` a selector that reaches only `TestMigratePackageIsStdlibOnlyLeaf`
(the three tests `04-01` T1 creates there carry no `TestMigrate` prefix) — not vacuous, and
`04-04:445` mandates a bare `go test ./... -count=1` immediately above it; and `04-04:217` vs
`:500` name the same new test two different ways (`…Delegates` / `…Converges`), which the
prefix selector `TestBackfill` covers either way.

---

## Cycle-6 actionables (C6-M1 … C6-M12, C6-L1 … C6-L10) — **all 22 CLOSED**

Each was verified by locating its substantive incorporation into a task, not a changelog mention.

| ID | Closing passage |
|---|---|
| C6-M1 | `04-02:344` mandates ONE converged / THREE stuck against the shipped `arm(2,0,…)` precedent (`migrate_faultinject_test.go:408,423`); `:376` requires the converged id from `inj.ids()[0]` — wire traffic, not fixture position |
| C6-M2 | `04-02:73` mandates a package-level **`var`** citing the `spineScrollBatch` precedent (verified live at `internal/store/spine.go:28`); `04-02:207` gates `^var migrateStatusFacetLimit uint64 = 1024` **and** `! rg '^const migrateStatusFacetLimit'`; the "executor picks" fork is deleted |
| C6-M3 | New search shape **S9** (`04-01:639`) with the `MigrateOptions{` sentinel grep; S8's Found column corrected to "E2, E3, E4 (partial — see S9)" |
| C6-M4 | Closed by removal — the inconsistent counts are gone; `04-01:263` now defers to ledger section E, `:390` enumerates the five derived files by name with no count |
| C6-M5 | Ledger row **D9** (`04-01:728`); `04-03:222` step 8c owns `cli.md:136-140`; `04-04:352` step 1b owns `:143-150`; `guides/cli.md` added to `04-04`'s `files_modified` |
| C6-M6 | `04-03:172` reworded to "must derive its want-set from the NAMED union `mutatingCommandNames()`, **never from the `!ReadOnly` predicate**"; `:96`, `:116`, `:642`, `:654` restate it. No "must derive from `!ReadOnly`" string survives |
| C6-M7 | `04-02:39` now reads "**The delta is a KEY-PRESENCE diff only.**"; `rg 'added/changed'` over all four plans returns nothing |
| C6-M8 | `04-03` T2 `<files>` (`:262-274`) and T3 `<files>` (`:458-469`) now list all six omitted files; `04-02` T2 `<files>` includes `schemaversion_stamp_gate_test.go` |
| C6-M9 | Phase-wide **INV-3** (`04-01:105`) plus exact-name mandates at every creating step (`04-01:199-201`, `:513`; `04-02:186`, `:420`) |
| C6-M10 | `04-03:405`, `:417`, `:533`, `:561` mandate the `TestMigrateFamily…` prefix and state why (the five shipped matches at `cmd/engram/migrate_test.go:71,86,104,133,145`) |
| C6-M11 | Ledger row **D10** (`04-01:729`); `04-04:344` step 1c amends §5's enumeration; `04-04:399` gates `! rg -n 'four of the six'` |
| C6-M12 | `04-02:172-176`: truncation is `uint64(len(facetHits)) >= migrateStatusFacetLimit` **only**; a sum mismatch without a reached bound retries the three RPCs **once**, then raises a distinct non-truncation error naming a concurrent writer. Pinned by a subtest at `:201` |
| C6-L1 | False counts gone; `04-01:64` restates the invariant by shape and names each gate in force |
| C6-L2 | `04-03:392` adds "replace the list's doc-comment COUNT phrase with a non-counting sentence"; closure at `:433`; ledger A22 carries the same instruction |
| C6-L3 | `task license:check` appended to all three `04-04` `<automated>` blocks (`:250`, `:315`, `:404`) and stated as a must_have at `:70` |
| C6-L4 | `04-03:362` corrects the rationale (04-04 T1 step 6c **REPLACES** the entry, does not add `statusReportDoc`) and mandates the exact test name |
| C6-L5 | Anchors re-verified live: `cli.md` header `:375` / zero-disables row `:378`; `usageErrorf` at `client_common.go:251` (with an explicit note that `operror.go` does not hold it); `spine.go` `var spineScrollBatch` at `:28`, `scrollAllPoints` at `:46`; `migrate status`/`migrateStatusTimeout` now in the C5-M6 summary |
| C6-L6 | Ledger row **E4** (`04-01:776`) names both sites as "NOT name-greppable (found only by S9, C6-L6)" with owners; both files added to `files_modified`; E5 amended so its do-not-touch constraint does not block the prose repair |
| C6-L7 | Ledger row **A24** (`04-01:691`) now reads "a CONSISTENCY gate among declared sets… It does NOT verify that a live command produces each allowlisted code", with the same caveat for A22 |
| C6-L8 | Closed by prohibition — the idiom is forbidden phase-wide, so the wrong-line-number defect is unreachable |
| C6-L9 | `04-04:192` extends the coverage-migration naming obligation to `TestBackfillCmdHasDryRunAndTimeoutFlags` and `TestBackfillSummaryUnchanged`, and adds a `newTestStore` do-not-delete guard citing row B8 |
| C6-L10 | `04-04:197` names `backfill_test.go:35` and `:38` as FORMAT-STRING hits the discovery grep misses but the step-6c acceptance gate matches |

---

## Actionable non-HIGH concerns (cycle 7)

Six. Each would be invisible to `/gsd-execute-phase` unless incorporated into PLAN.md or
explicitly deferred there.

### C7-M1 — `04-02`'s facet-truncation predicate is stated standalone, inviting an implementation that rejects a valid 1,024-bucket histogram, and no test discriminates the two readings. [Codex, verified orch]

`04-02:167` declares `var migrateStatusFacetLimit uint64 = 1024`; `:170` requests
`Limit: qdrant.PtrOf(migrateStatusFacetLimit)`; `:173` then states, standalone: *"**Truncation**
is signalled by `uint64(len(facetHits)) >= migrateStatusFacetLimit` — the bound was reached. …
This is the only condition that reports truncation."*

In the pinned client (`go.mod:19` → `qdrant/go-client v1.18.3`), `FacetCounts.Limit` is
documented `// Max number of facets. Default is 10.` (`points.pb.go:7849-7850`) and `FacetResponse`
(`:10344-10353`) carries **no truncation flag**. So `len(hits) == Limit` is genuinely ambiguous: a
collection with exactly 1,024 distinct `schema_version` values returns a complete histogram
indistinguishable, from the response alone, from a truncated one.

The plan does mandate a second, independent signal at `:171` (`Total` is a whole-collection exact
`Count`, compared against the derived sum), and truncation always implies `sum != Total`. But two
implementations satisfy the prose equally: **guarded** (`if sum != Total { if len >= limit →
truncation else → retry }`) — correct, no defect; and **hoisted** (`if len >= limit { return
truncationErr }`) — which `:173`'s standalone phrasing directly invites, and which false-positives
at exactly 1,024 legitimate versions. **No test discriminates them**: `:199` seeds *strictly more*
distinct versions than the lowered bound, and `:201`'s C6-M12 subtest drives a mismatch *without*
reaching the bound. The boundary is invisible to the plan's own gates. Nothing anywhere discusses
`len == limit` being potentially complete (the two `1025` mentions at `:73`/`:164` are only about
seeding cost).

**Fix.** Reword `:173` to pin the ordering rather than state a standalone predicate — the sum
comparison is the *only* entry into the failure branch, a reconciling sum proves completeness even
when `len(facetHits) == migrateStatusFacetLimit` (because `FacetResponse` carries no truncation
flag and `Limit` is a maximum), and only *inside* that branch does `len >= limit` classify
truncation vs. concurrent writer. Add one acceptance bullet: a subtest with
`len(hits) == migrateStatusFacetLimit` **and** a reconciling sum must return a clean histogram —
which goes RED against the hoisted form. (The over-fetch alternative, `Limit: limit+1` with
truncation iff `len > limit`, also works but collides with acceptance criterion `:198`, which
asserts the literal passes `migrateStatusFacetLimit` itself.)

### C7-M2 — revert preflight discards the reachable prefix of a broken chain, so the refusal can omit irreversible steps that `04-02:364`'s own acceptance criterion says it must name. [Codex, verified orch]

`internal/migrate/registry.go:102-127`: `StepsFrom` accumulates into `out` and, on a missing link
at `:116-119`, returns `nil, err` — **discarding the partial chain**. `Validate` rule 3
(`:57-62`, `:83-88`) enforces contiguity ("a single linear sequence, never a graph"), which makes
that discarded prefix provably part of any chain the record would ever traverse. The information
is knowable and is thrown away.

The plan then discards it a second time: `04-02:260` has `preflightRecordVersionSupport` report
`(chain, err == nil)` — chain is `nil` on error — and `:274` says *"If `!ok`, accumulate into
`plan.Unsupported`… **If `ok`**, append `chain` to `observedChains`"*, with
`plan.Irreversible = reversePreflight(observedChains)` at `:275`.

Reachable today: the registry is one step, v0→v1, Irreversible. A collection holding only v42
records, `revert --to 0` → `Unsupported = [{42,n}]`, `Irreversible = []`, refusal names only
`hint=unsupported`. The operator closes the version gap, retries, and gets a *second*, different
refusal. Add one v1 record and `Irreversible` populates, which is why it is easy to miss.

**Writes are never at risk** (`Unsupported` non-empty ⇒ `Reversible == false` ⇒
`RevertRefusalError` ⇒ zero records touched). The defect is diagnostic completeness — but
`04-02:364` states as an acceptance criterion that *"the refusal names EVERY irreversible step
(From/To/reason) and EVERY unsupported version… never a first-offender sample"*, and the specified
control flow does not achieve that. No gate catches it: test 8 (`:324`) is irreversible-only, test
11 (`:325-332`) is unsupported-only over a **fully reversible** fixture chain, and the C5-L4
discriminating subtest (`:373`) keeps all records at a supported version. The plan uses an explicit
"known limitation, accepted and documented" idiom elsewhere (`:259`, `:290`), so the silence here
is an omission rather than a choice.

**Fix — either one.** (a) Close it: at `:274`, when `!ok`, derive the maximal contiguous chain
upward from `to` (well-defined, since the registry is contiguity-validated) and append it to
`observedChains`, so one envelope names both classes; add a bullet to test 11 seeding a v42 record
against a fixture whose v0→v1 is Irreversible, asserting both `field=record_version hint=unsupported`
**and** `field=steps hint=irreversible`. (b) Or defer it explicitly at `:260` in the `:290` idiom,
and soften `:364` to match, so the acceptance criterion and the algorithm stop contradicting.

### C7-M3 — a THIRD load-bearing exclusion: `guides/upgrade.md` §12 denies the existence of `engram migrate`, inside the same `## Unreleased` section this phase adds its entry to. [orch]

The `Prose accuracy in files no test reads` exclusion survives only *"for prose this phase does not
touch **and does not falsify**."* It fails that at `docs-site/src/content/docs/guides/upgrade.md:313-343`
(§12, "Records now carry a `schema_version` stamp"):

- `:318` — "**No backfill is required and none is offered**"
- `:331-333` — "re-run the migration sweep against the affected record. **That sweep does not exist in this release** — there is no `engram migrate` command to run yet, **so do not look for one**."
- `:342` — "**Who should act:** nobody. This is purely additive, forward-looking groundwork; no existing behavior changes."

Why this is worse than the two already-promoted sites (D4/D9/D10): **§12 is inside `## Unreleased`**
(`rg -n '^## '` shows that section spans `:11-346`; §12 is subsection 12, and the entry `04-04` T3
adds becomes §13). The shipped release note would announce `engram migrate` in §13 and, ~30 lines
above it *in the same section*, tell the reader the command does not exist and not to look for one.
`04-04:66`'s own prohibition argues that even the older-section case is unacceptable — *"the file
self-contradicts across two sections while every gate stays green — a documentation surface that is
wrong AND passing is worse than one that is merely wrong."*

Every gate stays green: `extractUnreleasedSection` (`cmd/engram/docsync_test.go:34-48`) feeds
`TestUpgradeGuideNamesEveryChangedCommand` (`:72-100`), which checks only *presence* of command
names — a contradicting sentence satisfies it; the new `TestUpgradeGuideReconcilesBackfill` asserts
only `--dry-run`/preview-by-default strings, and §12 contains neither; `.rumdl.toml:29` excludes
`docs-site` outright. And no plan owns it: `rg 'upgrade\.md'` across the four plans shows exactly
three owned sites — the new `## Unreleased` entry, §5 (`:142-144`, D10 / T3 step 1c) and §v0.8.4
(`:436-437`, D4 / T3 step 1d). `:313-343` appears in no plan, ledger row, or exclusion — while
`guides/upgrade.md` **is** already in `04-04`'s `files_modified` (`04-04:22`), so this is prose the
phase owns and edits, not prose it merely neighbours. That is precisely the D9/D10 defect class
this cycle's revision promoted.

Scope is bounded: `rg -ni 'engram migrate|no backfill|migration sweep|not a migration' docs-site/ skill/ README.md CLAUDE.md`
returns only this section plus legitimate `migrate-remap-owner` references.

**Fix.** Add a step **1e** under `04-04` Task 3 and a ledger row **D11** (trigger `DOC`+`CMD`,
wave 4, owner 04-04 T3) rewriting `:318`, `:331-333` and `:342` to point forward at the new entry.
No frontmatter change needed.

### C7-M4 — `04-01` Task 3's cardinality must_have is over-broad: the identity is false whenever a manifest-mode write fails. [OpenCode]

The must_have states the written set is `manifest ∩ fresh re-derivation` and that `Migrated` is
its cardinality: `res.Migrated == uint64(len(manifest) - len(res.Spared))`.

That identity holds only on a run where every attempted write succeeds. In the manifest path, a
record in `observed ∩ manifestIDs` whose `SetPayload` fails increments `res.Failed` (the shipped
discipline at `internal/store/migrate.go:265-270`) but **remains in `observed`**, so it is not in
`Spared = manifestIDs \ observed` either. The general relation is therefore
`len(manifest) - len(Spared) == Migrated + Failed`, and the stated form is the `Failed == 0`
special case.

The *implementation* steps are correct — step 5b mandates counting successes, and
`MigrateResult.Migrated`'s own doc (`migrate.go:39-47`) defines it as successful writes — and step
12 asserts the formula only under a no-failure fixture, so the shipped code will be right. This
matters because the identity is C6-H3's replacement parity evidence: an executor writing a future
manifest-mode fault-injection test from the must_have text would produce a **failing test for
correct code**.

**Fix.** One sentence at the must_have: qualify it to "on a run where every attempted write
succeeds, `res.Migrated == uint64(len(manifest) - len(res.Spared))`; in general
`len(manifest) - len(res.Spared) == res.Migrated + res.Failed`, because a failed record is observed
(not Spared) but not written." The step-12 assertion is untouched.

### C7-M5 — three prose sites the `CurrentVersion` bump falsifies escape BOTH E4's sweep and the ledger's own new S9 pattern — in files the phase is already editing. [OpenCode]

This phase made prose falsification a named, rowed defect class (E4, C5-L3, C6-L6) and instructs
the executor to "repair EVERY prose claim the bump falsifies". Three sites would ship freshly false:

1. `internal/store/migrate.go:28` — the `MigrateOptions` doc says the production registry "stays empty through this phase". `04-01` T3 edits this exact struct's doc to add `DryRun`/`Manifest`.
2. `internal/migrate/step.go:74` — the `Irreversible` doc says "nothing in this phase's empty production Registry exercises the init-time half, because this phase registers no steps at all". After registration the registered Irreversible step *does* exercise the init-time evaluation. `04-01` T1 edits this file.
3. `internal/migrate/additive_test.go:17` — "never migrate.Registry, which ships EMPTY this phase (D-06)". T1 step 10 edits the assertion twenty lines below.

None contains the token `CurrentVersion`, so step 9's sweep (`rg -n 'CurrentVersion' internal/migrate/`)
cannot find them — and **the ledger's own new S9 prose pattern misses all three too**:
`additive_test.go` line-breaks between "Registry, which" (`:16`) and "ships EMPTY" (`:17`), and the
other two use lowercase "empty"/"stays empty". S9 was added this cycle precisely to catch this
class, so this is a gap in the newly added mitigation, not in a pre-existing one.

**Fix.** Name all three in `04-01` Task 1 step 9's site list (or in the E4 row), and widen the
E4/S9 prose pattern to catch the class — e.g.
`rg -ni 'registry\b[^.\n]{0,40}\bempty|empty[^.\n]{0,40}\bregistry|stays empty' internal cmd`.

### C7-L1 — `04-04:400`'s `guides/cli.md` closure gate is inert in both directions: the pattern spans a hard-wrapped newline. [orch]

```
! rg -n 'summarize-missing., and .backfill-short-ids. — is classified' docs-site/src/content/docs/guides/cli.md
```

The shipped paragraph is hard-wrapped:

```
Every other mutating operator command — `reindex`, `summarize-missing`, and
`backfill-short-ids` — is classified **non-destructive** and keeps its
pre-existing opt-in **preview** idiom, `--dry-run`.
```

The pattern spans the newline between `and` and `` `backfill-short-ids` ``. `rg` is line-oriented
with multiline OFF, so it returns no match **today, before any work**, and no match after — it
cannot go RED. Root cause is visible in the plan: step 1b (`04-04:354`) quotes the paragraph
reflowed onto one line, and the grep was derived from the reflowed quote rather than from the file.

Mitigated but not harmless: step 1b's action text is explicit and prescriptive, so a diligent
executor still rewrites the paragraph — and `04-04:400` half-concedes this ("the substantive check
is that the executor read the rewritten paragraph… and recorded it in the SUMMARY"). But this is
the closure gate for C6-M5, it silently self-certifies, and an executor who skips step 1b ships a
docs page flatly contradicting the binary.

**Fix.** Anchor on a fragment that exists on one line — `! rg -n 'pre-existing opt-in \*\*preview\*\* idiom' docs-site/src/content/docs/guides/cli.md`
(verified present on a single line at `:145`) — or drop the grep and section-scope it in Go, as the
plan already does for `upgrade.md`.

---

## Non-actionable observations (recorded, not counted)

- **`04-01` T1 leaves `internal/store` RED**, repaired by T2 in the same wave. Explicitly stated in PLAN.md (`04-01:221-227` and its `<done>`), blast radius independently confirmed accurate. Excluded from the count because it is documented, not invisible. The invariant's wording should say "every task's own package(s)", not "the tree".
- **`04-04:473` and `:434`/`:436`** restate claims C6-H4 and C6-H7 retracted (the `migrateWithTimeout(ctx, backfillTimeout)` call form; "proves this with `--apply` against real Qdrant"). Prose only; no gate reads them; the action text is emphatic and correct.
- **`04-04:155`** reinstates a comment-text prohibition its own must_have at `:60` explicitly deletes, and mis-describes the gate (`'"dry-run"'`, a quoted string, not the bare literal). Worst case is harmless over-compliance.
- **The formatter deletion gate cannot see a surviving `type backfillOutputDoc`** (`cmd/engram/backfill.go:67`): `func backfillOutputDoc` never matches a type, and `backfillOutputDoc\(` matches neither the declaration nor a composite literal. `.golangci.yaml` does not enable `unused`, so lint will not catch an orphaned type either. Dead unexported type, no behavioural consequence; add `|type backfillOutputDoc` to the alternation if you want it covered.
- **Four plan assertions that `task lint` runs rumdl over the edited docs-site markdown are false** (`.rumdl.toml:29-30` excludes `docs-site` and `.planning`). Nothing fails. The genuinely rumdl-exposed file is `skill/engram/skills/curating-memory/SKILL.md`.
- **`CONTRIBUTING.md:70`** names `TestBackfillShortIDs` in a worked debugging example; `04-04` T1 deletes that test. Ungated, cosmetic. Note that `CONTRIBUTING.md`/`README.md`/`RELEASING.md` are in no exclusion row — the prose exclusion enumerates only `docs-site/**` and `CLAUDE.md`'s Layout table.
- **`cmd/engram/operror.go:49`** — a non-test doc comment enumerating "the four sweep-style operator commands" goes stale once the migrate family routes through `classifyOperatorErr`. Ungated, cosmetic.
- **INV-3 consistency residue** — three bare `TestMigrate` selectors and one duplicate test name, all cosmetic (see the `-run` section).
- **`rg -n "NewMintingStep\(0, 1" internal/migrate/registry.go` is formatting-fragile** [OpenCode] — a line-wrapped registration call would fail the positive gate. gofmt will not wrap it and the plan mandates the exact literal, so it is recoverable at execution.
- **Gate self-count divergence** — this cycle's two independent enumerations of the negative gates came to 18 (OpenCode) and 19 (orch). Immaterial, and correctly *not* recorded in the plans: INV-4 forbids re-introducing a self-count, and neither enumeration changes any gate's shape verdict.

---

## Risk Assessment

**LOW-to-MEDIUM, and converged.** Every structural class this phase has fought across six cycles
is now closed at the root: the comment-stripping idiom is gone phase-wide, all twelve tasks carry
unfiltered package runs, the 04-03 re-sequencing holds in both directions at exact set equality,
no `-run` selector is vacuous, no PLAN line anchor survives, and all 22 cycle-6 actionables are
substantively incorporated. Zero HIGH concerns remain.

Both external reviewers converge on `high=0`, and OpenCode — which ran the packages and reports
green baselines throughout (`cmd/engram` 2.3s, `internal/migrate`/`surfaces`/`keylinks` <1s,
`internal/server` 6.3s, `internal/store` 41s with containers) — records an explicit **READY TO
EXECUTE**.

The six open items are bounded and local: two are single-passage rewordings in `04-02` that pin an
ordering and a diagnostic the plan already half-specifies (C7-M1, C7-M2); one qualifies a single
invariant sentence in `04-01` T3 (C7-M4); one names three prose sites and widens one search shape
in `04-01` T1 (C7-M5); one adds a step + ledger row in `04-04` T3 over a docs section already in
`files_modified` (C7-M3); and one re-anchors a single gate (C7-L1). None re-opens a closed class,
none requires re-planning, and none is structural. C7-M3 and C7-L1 are both documentation
correctness in the same wave-4 task and can be fixed together, as can C7-M4 and C7-M5 in `04-01`.

Two patterns are worth naming for the record. First, **the remaining defects cluster in prose that
gates cannot see** — an over-broad invariant sentence, three stale doc comments, a release-note
section, and a gate whose pattern does not match the file it guards. That is a materially better
failure surface than cycles 1–6, where the defects were in the gates and the sequencing themselves.
Second, **C7-M5 is a gap in a mitigation this cycle added** (search shape S9 misses the very class
it was introduced to catch), which is the one place where the revision's own new machinery is not
yet self-sufficient.

The residual execution risk is the documented `04-01` T1 red-tree window, bounded to one task
inside one wave, whose blast radius has now been independently re-derived twice and confirmed
accurate and complete.

---

## Codex Review (cycle 7)

## Summary

The plans are substantially stronger after cycle 7: the command/registry sequencing, shaped negative gates, unfiltered package tests, and cross-surface conformance coverage are mostly coherent and source-backed. However, the phase is **NOT READY TO EXECUTE** yet. Three actionable issues remain: one task knowingly leaves an edited package red, the status histogram rejects a valid exact-boundary result, and revert preflight can omit irreversible steps when the same record also has an unsupported-version gap.

## Strengths

- The cycle-7 command sequencing now reflects the real bidirectional registries. `operatorParityRows` is checked for missing and extra commands at [`cmd/engram/operator_output_test.go:307`](cmd/engram/operator_output_test.go:307), invalid-output coverage is similarly derived at [`cmd/engram/operator_output_test.go:532`](cmd/engram/operator_output_test.go:532), and catalog output flags are compared against the live command tree at [`cmd/engram/cmdwalk_test.go:186`](cmd/engram/cmdwalk_test.go:186). Plan 04-03 now places each command and its rows in the same task.

- The revised negative gates are property-shaped. I found no executable use of the banned `rg -v '^\s*//'`, `rg -c`, `head`, or `git diff --stat` patterns. The backfill deletion checks use declarations, calls, table rows, string literals, and imports, avoiding failures caused solely by explanatory comments.

- The key-link boundary is now correctly treated as load-bearing. The shipped gate scans active phase plans under `.planning/phases` at [`internal/keylinks/gate_test.go:103`](internal/keylinks/gate_test.go:103), and the revised plans include the corresponding unfiltered `internal/keylinks` test rather than dismissing `.planning/**` wholesale.

- The planned full-collection revert preflight uses the existing paginated helper rather than a one-page fixture abstraction. `scrollAllPoints` advances `NextPageOffset` until exhaustion at [`internal/store/spine.go:46`](internal/store/spine.go:46), supporting the plan’s multi-page proof.

- Generalizing `registerDestructive` is aimed at the correct seam. Its current implementation validates classification and owns the single preview/apply `RunE` branch at [`cmd/engram/destructive.go:110`](cmd/engram/destructive.go:110); widening admission there preserves the centralized mutation choke point.

## Concerns

- **MEDIUM — Plan 04-01 Task 1 still violates its own atomic green-task invariant.** The phase-wide invariant says each task runs the unfiltered tests for every package it edits at [`.planning/phases/04-migration-cli-first-customer/04-01-PLAN.md:84`](.planning/phases/04-migration-cli-first-customer/04-01-PLAN.md:84). Task 1 edits `internal/store/migrate.go`, but its verification runs only `go test ./internal/migrate/`; its scope note explicitly acknowledges that two `internal/store` tests remain red until Task 2 at [`.planning/phases/04-migration-cli-first-customer/04-01-PLAN.md:221`](.planning/phases/04-migration-cli-first-customer/04-01-PLAN.md:221). An executor committing by task therefore produces a knowingly broken intermediate commit. This is actionable because merely adding an unfiltered store test would make Task 1 fail.

- **MEDIUM — The status facet truncation test rejects a valid histogram with exactly 1,024 distinct versions.** Plan 04-02 requests `Limit: 1024` and treats `len(hits) >= 1024` as proof of truncation at [`.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:150`](.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:150). In the pinned Qdrant client, `FacetCounts.Limit` is the maximum returned count; a response whose length equals the limit may be complete. Consequently, exactly 1,024 legitimate version buckets produce a false “incomplete” error. The proposed test covers over-limit cardinality but not the exact boundary. This matters because the milestone explicitly permits arbitrary newer schema versions.

- **MEDIUM — Revert preflight can omit irreversible steps when a record also has an unsupported-version gap.** The proposed `preflightRecordVersionSupport` records a chain only when `revertStepsFrom` succeeds, then computes irreversible steps solely from those successful chains at [`.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:256`](.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:256). The existing `StepsFrom` discards its partial chain and returns `nil, err` when it encounters a missing link at [`internal/migrate/registry.go:102`](internal/migrate/registry.go:102). Thus, for a future-version record whose reachable prefix includes v0→v1, preflight can report only “unsupported version” and omit the irreversible v0→v1 step. Writes remain safely refused, but D-14’s requirement to name every applicable irreversible step and reason is not met.

## Suggestions

- Merge 04-01 Tasks 1 and 2, or move the `CurrentVersion` bump and registry activation into Task 2 alongside all affected `internal/store` repairs. Each committed task should end with both `go test ./internal/migrate/ -count=1` and `go test ./internal/store/ -count=1` green.

- Request one extra facet bucket: define a maximum accepted bucket count of 1,024, request 1,025, and reject only when more than 1,024 hits are returned. Add separate tests for exactly 1,024 buckets succeeding and 1,025 buckets producing the bounded-result error.

- Make the revert chain walk return its reachable partial chain alongside the unsupported-version result, and derive irreversible steps from that prefix. Add a mixed preflight fixture that asserts both `Unsupported` and `Irreversible` are populated and that `RevertRefusalError` renders both clauses.

## Risk Assessment

**MEDIUM.** No remaining issue appears capable of causing an unsafe partial revert or automatic migration, but the current plans would create a red intermediate task commit and two observable correctness defects in status and refusal diagnostics.

TALLY: high=0 actionable=3

---

## OpenCode Review (cycle 7)

_(OpenCode ran source-grounded with repo access; its interleaved tool narration is trimmed. Verification baselines it reports: `cmd/engram` 2.3s, `internal/migrate`/`surfaces`/`keylinks` <1s, `internal/server` 6.3s, `internal/store` 41s with containers — all green.)_

# Cycle-7 Cross-AI Plan Review — Phase 4 (Migration CLI & First Customer)

## Summary

This revision holds up under source-level scrutiny. The four claimed root-cause fixes are real: **(a)** no comment-stripping idiom survives in any executable gate — all 18 negative gates are declaration-, call-, composite-literal-, row-, string-literal-, or import-shaped, and the two prose-phrase gates live in acceptance criteria (not `<automated>`) where no mandated comment can trip them; **(b)** all twelve `<verify>` blocks carry a bare unfiltered `go test ./<pkg>/ -count=1`, and my independent simulation of the 04-03 re-sequencing (every `operatorCommands()`-keyed registry row landing in the same task as its command) comes up green at the end of T2; **(c)** self-referential bookkeeping is gone from gate positions; **(d)** the `.planning/**` keylinks gates are correctly promoted to rows F1/F2 (verified: `03-01-PLAN.md:75/:79` declare `migrate[.]StepsFrom`/`migrate[.]CheckAdditive` anchored solely in `internal/store/migrate.go:200/:230`, and `go test ./internal/keylinks/` is green today). All seven cycle-6 HIGHs are genuinely closed against shipped source, all 42 shipped test names in `-run` selectors resolve, and every line anchor I checked (~80) is accurate. Two LOW actionable findings: one over-broad invariant statement in 04-01's must_haves, and three unrowed stale-prose sites that escape both E4's sweep and the ledger's own new S9 pattern. **Verdict: READY TO EXECUTE.**

## Strengths

- **The C6-H2 re-sequencing simulates green.** I walked the end of 04-03 T2 against live derivations: `operatorCommands()` (`cmdwalk.go:101-116`: non-nil RunE, no `server` flag, not in `{serve, version}`) admits `migrate`/`migrate status` the moment they exist, and T2's step 6a updates `wantOperatorCommandKeys` (`cmdwalk_test.go:117-130`), `operatorParityRows` (`operator_output_test.go:137`, bidirectional at :316-326), `operatorInvalidOutputArgs` (`:532-563`, fatal at :560), and regenerates goldens in the same task. `TestDestructiveCommandsRequireApply` switched to `mutatingCommandNames()` in T2 yields exactly 4 names = 4 live `--apply` carriers (`prune.go:159`, `spine_review_purge.go:425`, `migrate.go:257`, + `migrate`) — verified each call site. The `timeoutGroupCaseArgs` `t.Fatalf` default (:510) and the `TestTimeoutGroupMatrix` set equality (:625-648) are correctly handled per-task.
- **The two composite-literal gates are RED today, GREEN after.** `rg -n '^\s*SchemaVersion: migrate\.Version\(1\),' internal/store/migrate_converge_test.go` matches :132 and `^\s*Target:\s*1,` matches :180 right now — and a comment line (which begins `//` after whitespace) can never match either pattern. Same for the `seedLegacyRecord(ctx, t, writerStore` gate: the legitimate six-record seeding loop at :111 uses `sweepStore`, so the writerStore-scoped negative gate correctly spares it (C6-H5 closure verified structurally, with `rawPayloadNoFatal` at :448-452 as the mandated precedent and PA-11a at :117-124 now in `read_first`).
- **C4-H1's rejection is independently verified.** I counted the live `surfaces.Operations()` rows myself: `!ReadOnly` with non-empty `CLICommand` selects exactly the eleven commands the plan names (incl. `serve`, `migrate-set-owner`, `spine-review archive/restore` at `toolclass.go:222/:239/:302/:311`), of which only three are `Destructive:true` (`:172`, `:180` area, `:327`). The named-union derivation (`destructiveCommandNames()` ∪ `applyRoutedAdditions` − `pendingApplyConversion`) is the correct shape, and the enumerated 4→5→6 membership progression across T2/T3/wave-4 checks out against the live flag carriers.
- **The fault-injection arithmetic is now right.** The shipped precedent proves `inj.arm(2, 0, faultBeforeInvoke)` over six records yields `res.Migrated == 1` (`migrate_faultinject_test.go:408,423`); the plan's phase-1 expectation of ONE converged / THREE stuck over four records matches, the converged id comes from `inj.ids()[0]` wire traffic (:434-440), and the interceptor's `*qdrant.SetPayloadPoints`-only type switch (:176-180) makes the DeletePayload-lands/SetPayload-fails sequence constructible exactly as specified.
- **B5a is real and correctly constrained.** `qdrant.FacetCounts` *does* implement `GetFilter()` (verified via `go doc`), so `recallCaptureInterceptor`'s default branch (`schemaversion_recallgate_test.go:893-901`) would `t.Fatalf` on a filtered Facet — and `Limit` defaults to 10 per the proto doc, so the named `var migrateStatusFacetLimit` + runtime truncation detector is genuinely needed, with the `spineScrollBatch` var precedent (`spine.go:22-28`) correctly cited for why it's a var (C6-M2).
- **The keylinks F1 row is precise.** `TestActiveMilestoneKeyLinksSatisfiable` (`internal/keylinks/gate_test.go:128`) scans the live `.planning/phases`; phase 3's links at `03-01-PLAN.md:75/:79` anchor on `internal/store/migrate.go:200` (`migrate.StepsFrom`) and `:86/:230` (`migrate.CheckAdditive`); 04-01 T3's step 13b + `<verify>` runs the package.
- **Every C6 actionable I sampled is closed.** C6-M1 (arithmetic), C6-M2 (var), C6-M3 (S9 added), C6-M4 (enumeration gone), C6-M5 (D9 rows + cli.md in 04-04's files), C6-M6 (no `derive from !ReadOnly` text survives), C6-M7 (no "added/changed"), C6-M8 (T3's `<files>` now lists all edited files), C6-M9 (every new test mandated by exact name), C6-M10 (`TestMigrateFamily` prefix), C6-M11 (D10 amendment), C6-M12 (truncation-vs-racer distinction + retry), C6-L2/L3/L4/L6/L9/L10 all verified in plan text against source.

## Concerns

- **[LOW — actionable] 04-01 Task 3's must_have states a cardinality identity that is false under manifest-mode write failure.** The must_have says: "The written set is manifest ∩ fresh re-derivation, and `Migrated` is its CARDINALITY: `res.Migrated == uint64(len(manifest) - len(res.Spared))`". Mechanism of failure: in the manifest path, a record in `observed ∩ manifestIDs` whose `SetPayload` fails increments `res.Failed` (per the shipped discipline at `internal/store/migrate.go:265-270`), but it remains in `observed`, so it is not in `Spared = manifestIDs \ observed` either. Then `len(manifest) - len(Spared) = Migrated + Failed ≠ Migrated`. The implementation steps are correct (step 5b mandates counting successes, and `MigrateResult.Migrated`'s doc at `migrate.go:39-47` defines it as successful writes), and step 12 asserts the formula only under a no-failure fixture — so the code will be right and only the invariant statement is over-broad. But an executor writing a future manifest-mode fault-injection test from the must_have text would produce a failing test for correct code.
- **[LOW — actionable] Three value-restating prose sites falsified by the `CurrentVersion` bump escape both E4's stated sweep and the ledger's own new S9 pattern.** (1) `internal/store/migrate.go:28` — the `MigrateOptions` doc says the production registry "stays empty through this phase"; 04-01 T3 edits this exact struct's doc to add `DryRun`/`Manifest`. (2) `internal/migrate/step.go:74` — the `Irreversible` doc says "nothing in this phase's empty production Registry exercises the init-time half, because this phase registers no steps at all" — post-registration this is false (the registered `Irreversible` step *does* exercise the init-time evaluation); 04-01 T1 edits this file. (3) `internal/migrate/additive_test.go:17` — "never migrate.Registry, which ships EMPTY this phase (D-06)"; T1 step 10 edits the assertion 20 lines below. None contains the token `CurrentVersion`, so step 9's sweep (`rg -n 'CurrentVersion' internal/migrate/`) cannot find them; the ledger's own new S9 prose pattern (`Registry.*EMPTY|CurrentVersion is 0|is pinned at 0`) misses all three too — `additive_test.go:17` line-breaks between "Registry, which" (:16) and "ships EMPTY" (:17), and the other two use lowercase "empty"/"stays empty". This phase made prose-falsification a named, rowed defect class (E4, C5-L3, C6-L6) and the executor is instructed to "repair EVERY prose claim the bump falsifies"; these three would ship freshly-false in the very files being edited.
- **[LOW — not actionable] `rg -n "NewMintingStep\(0, 1" internal/migrate/registry.go` is formatting-fragile.** If an executor wraps the long registration call across lines, the positive gate fails; gofmt won't wrap it and the plan mandates the exact literal, so it's recoverable at execution.
- **[LOW — not actionable] 04-04 T1 step 3's "no comment in `backfill.go` may contain the literal `dry-run`" is stricter than its own string-literal gate** (`"dry-run"` quoted), which already permits unquoted prose mentions. Harmless redundancy.

## Suggestions

1. **For the cardinality identity (04-01 Task 3, must_have):** qualify the formula — change "`Migrated` is its CARDINALITY: `res.Migrated == uint64(len(manifest) - len(res.Spared))`" to "on a run where every attempted write succeeds, `res.Migrated == uint64(len(manifest) - len(res.Spared))`; in general `len(manifest) - len(res.Spared) == res.Migrated + res.Failed`, because a failed record is observed (not Spared) but not written." One sentence; the step-12 assertion is untouched.
2. **For the three prose sites:** name them in 04-01 Task 1 step 9's site list (or the E4 row) — `internal/store/migrate.go:28` (drop the "stays empty" clause), `internal/migrate/step.go:74` (re-tier the init-time sentence to its discharged form), `internal/migrate/additive_test.go:17` (update the doc comment alongside the :40-42 assertion) — and widen the E4/S9 prose pattern to catch the class, e.g. `rg -ni 'registry\b[^.\n]{0,40}\bempty|empty[^.\n]{0,40}\bregistry|stays empty' internal cmd`.

## Risk Assessment

**LOW.** Every load-bearing mechanism was traced to shipped source and found as described; all baselines are green (`cmd/engram` 2.3s, `internal/migrate`/`surfaces`/`keylinks` <1s, `internal/server` 6.3s, `internal/store` 41s with containers); the task-level green-state simulation passes at every wave boundary I checked (04-01 T1's declared internal/store RED window is owned by T2 in the same wave with an explicit scope note); and the only surviving defects are one over-broad invariant sentence and three stale-prose sites, all LOW. Execution risk concentrates in 04-03 T2's registry sweep, and that is exactly where the plan is now most explicit.

TALLY: high=0 actionable=2

# Cross-AI Plan Review — Phase 4 (Cycle 6)

Reviewers: Codex (`gpt-5.1-codex`, xhigh) and OpenCode (`openrouter/moonshotai/kimi-k3`).
Both had full repo file access and verified plan claims against shipped source; neither output
carries the `[reviewed-without-repo-access]` marker. Plans reviewed are the revisions landed in
`ec1e00bc` ("docs(04): close cycle-5 findings as classes; extend ledger to 8 shapes/triggers").

OpenCode's lane invocation would again have been killed by the runner's hardcoded 660 s
`timeoutFloorMs`; as in cycles 4 and 5 it was run directly (`opencode run --model … --format json -`)
against the same prompt file with an extended deadline. No lane was dropped. The orchestrator
(Claude Opus 5) additionally re-ran the two class sweeps, the `-run` selector sweep, the ledger
boundary audit and the C5 closure check independently against shipped source; findings attributable
to that pass are marked **[orch]**.

Cycle 6 was directed to fix CLASSES rather than instances, and to make the ledger self-extending.
**Both class claims are falsified**, though by narrower margins than in cycle 5: the plans are
materially better and most C5 findings are genuinely closed against source. What survives is (a)
gate forms whose stated defence does not cover the way a comment is actually written, (b) one more
instance of the forward-reference class one task boundary further along than the three the revision
moved, and (c) a small number of plan instructions that cannot compile, cannot pass, or contradict
a shipped invariant.

---

## Verified CLOSED against shipped source

These were checked line-by-line against the tree, not against the plans' own claims.

| Finding | Verification |
|---|---|
| **C5-H4** (revert-refusal exit code) | `TestExitCodeBaselineRowCount` really is a COUNT pin + name-uniqueness (`cmd/engram/exitcode_baseline_test.go:440-453`); the only other consumer is `docsync_test.go:87`, a one-direction derivation FROM the table. **No set-equality in either direction**, so the missing row is genuinely inert and `wantRows = 37` correctly stands. `exitUsage = 2` is "usage or validation error" (`client_common.go:222`); `usageErrorf` is already the sanctioned operator-tier route (`spine_review_purge.go:65,92`, `migrate.go:35`, and `operror_test.go:238` names it explicitly). `exitFindings` (7) is scoped by its own doc to a command that succeeded. Render-then-return precedent exact at `spine_review_verify.go:659-662`. **Claim verified.** |
| **C5-H3** (`operatorCommandFiles`) | Hand-maintained literal at `cmd/engram/operror_test.go:179-186`, six entries, `migrate.go` already present; the gate at `:212-241` reads each named file and cannot fail on an unlisted one. The RED-first experiment is the right evidence. **Claim verified.** |
| **C5-L4** (`reversePreflight` union) | Not a mention — signature (`04-02:255`), producer (`:258`), accumulator (`:272`), consumer (`:273`) and a discriminating subtest (`:372`) all changed. **Genuinely closed.** |
| **C5-M7 / C5-L3** (CurrentVersion prose) | `store.go:646` stamps `int(max(CurrentVersion, m.SchemaVersion))`; `:659-661` omits `short_id` when empty; `MintShortID` is called at `internal/server/tools.go:1144`, `:1287`, `:2097` — all three exact. Five prose sites individually adjudicated at `04-01:175-181`. **Closed.** |
| **C5-L6** (`Facet` must not carry a `Filter`) | `recognizedFilterCarryingRequestMethods` = exactly `{Query, Scroll, Count}` (`schemaversion_recallgate_test.go:866-870`), `t.Fatalf` at `:900`, `Facet` absent from `recallEmissionMethods` (`:336-342`). New ledger row B5a is correct. **Closed.** |
| **C5-L5, C5-L9, C5-L10, C5-L11, C5-L2, C5-M4, C5-L8** | All verified present in plan text and consistent with source. |
| **D8** (new finding, `hint=` outside `argerror.go`) | `rg -n 'hint=' --glob '!**/*_test.go' cmd internal/store internal/surfaces` returns **nothing** today; `reference/errors.md:94-97` does claim the ten-code table "cannot list a code the server does not emit"; `internal/server/argerror.go` declares exactly ten `HintCode` constants. No test reads the table back, so it is correctly rowed as an owner obligation with `docs-site/.../reference/errors.md` in 04-02's `files_modified`. **Correct and well-caught.** |
| **C5-H1's flagship gate** | `! rg -n 'func \(s \*Store\) BackfillShortIDs\|func missingShortIDFilter\|\.BackfillShortIDs\(\|missingShortIDFilter\(' internal/ cmd/` run against the shipped tree: **12 hits, all real declaration/call sites, zero prose hits.** `migrate.go:60` writes `BackfillShortIDs store.go:2741` (no paren) and `migratebacklog.go:42` writes `missingShortIDFilter (store.go:…)` (space before paren) — both dodge the call-shaped form exactly as claimed. **This one is genuinely fixed.** |
| **`-run` phantom sweep** | Every individual name in every `-run` selector across all four plans resolves to a shipped `func Test…` or to a test the **same task** creates. The two known phantoms are explicitly retired in place (`TestCheckAdditive` → `TestAdditiveOnlyKeySetDiff` at `04-01:208`; `TestEveryUpsertSiteIsClassified` → `TestEveryPointWriteRoutesThroughPayload` at `04-02:381-386`). **No true phantom remains.** |
| **Forward references named by the revision** | All six relocations verified: `04-03` T1→T3 (`TestMutatingCommandNamesMembership`, `TestApplyRoutedAdditionsArePinned`, the `RequireApply` switch, the `destructiveFlagCases` row), T2→T3 (`"migrate revert"` `timeoutGroups`), `04-04` T2→T1 (`pendingApplyConversion`, golden regeneration). **All correctly moved.** |

---

## HIGH concerns (cycle 6)

### C6-H1 — The negative-gate class (INV-2) is NOT closed. The claim "twelve gates audited, exactly two bare-name forms survive, each paired" is false on both halves. [Codex + orch]

An independent enumeration finds **~30 distinct negative/count assertions across ~43 sites**, and
**nine bare-name forms, of which four are unpaired**. Every one of the four is invalidated by prose
the plan's own `<action>` text instructs the executor to write.

| # | Gate | Site | Why a compliant implementation fails it |
|---|---|---|---|
| a | `rg -n 'Target: 1' internal/store/migrate_converge_test.go` returns nothing | `04-01:374` | Step 2e (`04-01:322-329`) *mandates* rewriting PA-10a item 2, whose shipped text (`migrate_converge_test.go:63-65`) reads "the test supplies both the stamp's input and the sweep's target itself" and "is now FALSE and must be rewritten, not left standing". The natural rewrite names the removed `Target: 1`. No paired prohibition — the two literals covered at `04-01:331-342` are only `SchemaVersion: migrate.Version(1)` and `BLOCKING for Phase 4`. |
| b | `rg -o 'Target: -1' internal/store/migrate_test.go \| wc -l` returns **1** | `04-01:373` | Step 1 (`04-01:273-276`) mandates a doc comment containing the literal `Target: -1` ("a future reader must not 'simplify' `Target: -1` back to `Target: 0`"). Count becomes **2**. The gate and the prose that breaks it are twelve lines apart in the same `<action>` block. **This gate is unsatisfiable as written.** |
| c | `rg -n "migrateTimeout" cmd/engram/migrate_family.go` returns nothing | `04-03:538`, `04-03:593` | `04-03:395` states the correct comment-filtered form *and the exact reason the bare form is wrong* — "a comment saying 'not `migrateTimeout` — already bound to migrate-set-owner' fails a bare-name grep on a fully compliant file". The `<verify>` blocks at `:406`/`:542` use the filtered form; the **bare form survives in the acceptance criteria and the phase `<verification>` section**, where an executor reads it as the standing requirement. Steps `:325`, `:347`, `:441` all mandate recording the reason. |
| d | `rg -n "migrateLastPreviewManifest" cmd/engram/` returns nothing | `04-03:588` (prose form `:403`) | `04-03:408-411` names this exact string as the prior revision's defect ("bare-name greps over the whole package … a compliant file that documents its own rules would fail both"), then leaves it standing in `<verification>`. Whole-**directory** scope, so a `_test.go` naming the absent var also trips it. |

**And the "comment-filtered" category itself does not do what the plans claim.** `rg -v '^\s*//'`
strips only **full-line** comments; a trailing comment on a code line survives the filter and matches.
The plans mirror this from the shipped `TestNoBareOperatorErrorReturns` (`operror_test.go:225`,
`strings.HasPrefix(trimmed, "//")`), where it is safe because the forbidden literal is
`return fmt.Errorf(`. Here the forbidden literal is precisely what the plan tells the executor to write:

- `04-04:217` justifies comment-filtering the `store\.MigrateOptions` gate **"because step 3 instructs the executor to record the prohibition"** — a trailing comment recording it fails the gate. Same for `signal\.NotifyContext` (`04-04:221`).
- `04-03:395` reasons identically for `migrateTimeout`, and `04-03:302` **mandates a trailing comment** in that very file (`PreviewRevert(…) // REVIEWS.md M8 — exported preflight accessor`).
- Also affected: `04-03:542`'s `hint=irreversible|hint=unsupported`, `04-04:235`/`:295`'s `pendingApplyConversion`.
- `04-02:380`'s `StepsFrom(steps, from, to)` is the one that survives, because it carries an **additional** explicit text prohibition (`04-02:66`) — which is the discipline the others need.

**A fresh instance of the same class was created by the C5-L1 fix.** `04-04:346` specifies
`--timeout[^\n]{0,40}\b(removed|no longer accepted|dropped)\b`, replacing a whole-file
co-occurrence grep with what it calls "a CLAIM that the flag was removed, rather than a
CO-OCCURRENCE of two words". Executed against the plan's own repeatedly-quoted permitted sentence
— `` `--timeout` is preserved; `--dry-run` is removed `` (`04-04:67`, `:345`, `:357`) — the gap is
**31 characters**, so `{0,40}` **matches and the gate fails a compliant document**. `{0,30}`, the
width cycle 5 actually specified, does not. The fix reinstated the defect it was written to remove.

**Fix:** (a)+(c)+(d) — replace with the comment-filtered forms already present elsewhere in the same
plans, or add the missing paired text prohibitions. (b) — change to `-ge 1`, or comment-filter.
Class — swap `rg -v '^\s*//'` for a filter that also strips trailing comments (`sed 's://.*::'`), or
state one phase-wide prohibition: *no comment in a gated file may contain a gated literal*. C5-L1 —
narrow to `{0,30}`.

### C6-H2 — The forward-reference class (INV-1) re-instantiates one task boundary later, and the plan's own stated acceptance evidence for the class fix is false. [orch, corroborated by OpenCode]

04-03 Task 2 puts `migrateCmd` and `migrateStatusCmd` into the live cobra tree. Both satisfy
`operatorCommands()`'s predicate the moment they exist — non-nil `RunE` (`registerDestructive` sets
it at `destructive.go:124`), no `server` flag, not in `operatorCommandExclusions`
(`cmdwalk.go:101-116`). Every registry keyed on that derivation therefore changes at Task 2, and the
plan defers all of them to Task 3 step 3b (`04-03:474-488`, "Do this task LAST in the wave … Expect
exactly these four" failures).

Verified RED at the end of Task 2:

- `TestOperatorOutputParity` — bidirectional set equality against `commandKeySet(operatorCommands())` (`operator_output_test.go:311-327`): fires twice, "`operatorParityRows()` is missing a row for operator command".
- `TestOperatorCommands` via `wantOperatorCommandKeys` (`cmdwalk_test.go:154`).
- `TestEveryOperatorCommandRejectsInvalidOutput` (`operator_output_test.go:570` → `t.Fatalf` default at `:560`).
- `TestCatalogOutputFlagMatchesOperatorCommandsUnionClientVerbs` (`cmdwalk_test.go:194`).
- `TestHelpGolden`/`TestCatalogGolden` (`golden_test.go:290`,`:306`) — the goldens are only regenerated in Task 3 step 6.
- `TestDestructiveCommandsRequireApply`'s reverse direction (`destructive_test.go:100-104`) — `04-03:229` keeps it deriving from `destructiveCommandNames()` through the end of Task 2, so `migrate` (carries `--apply`, `Destructive:false`) fires "carries --apply but is not classified destructive".

Against that, `04-03:525` and `:613` assert — **as the acceptance evidence for the C5-H5 class fix** —
that "**Tasks 1 and 2 each pass `go test ./cmd/engram/ -count=1` unfiltered on their own**". Task 1
does (`:241`). Task 2's `<verify>` (`04-03:406`) contains **no unfiltered run at all**, only a
four-name `-run` subset every member of which is green at that point — the exact concealment pattern
the plan condemns at `04-03:585`.

**Fix — one of two, and the current text is neither:**
1. Split step 3b so the `migrate` / `migrate status` rows (A1, A2, A3, A7) and the first golden regeneration land in **Task 2**, and only `migrate revert`'s land in Task 3 — the pattern the plan already applies to `timeoutGroups` (`:334`/`:442`) and to `TestCatalogBlastRadiusMatchesToolClasses` (`:375`/`:470`). Then the claim at `:525` becomes true.
2. Delete the claim at `:525`/`:613`, declare Task 2 a deliberately-RED intermediate committed as part of one wave, and drop the "committable on its own" framing from `04-03:227`/`:241`.

### C6-H3 — 04-01 Task 3 step 12 mandates an assertion that cannot compile, and the plan contradicts itself about `MigrateResult.Migrated`'s type. [Codex, verified orch]

`04-01:470` requires `!slices.Contains(res.Migrated…)`. `MigrateResult.Migrated` is
`uint64` (`internal/store/migrate.go:49`), and the same plan requires `res.Migrated >= 1` (`:461`)
and `res.Migrated == 300` (`:467`). `slices.Contains` does not compile on a scalar.

The must_haves are themselves inconsistent: `04-01:42` calls all **three** outcomes "IDENTITY SETS"
and defines `Migrated = manifest ∩ fresh re-derivation`, while `:41` says "`MigrateResult.Migrated`
is 0 in DryRun mode" and `:45` types only `Spared`/`Appeared` as `[]string`. So the plan neither
declares a migrated id set nor permits the counter to carry identities — and the test it mandates
is the one place the difference matters, because the whole point of step 12 is to prove parity
**"by id, not by count"**.

**Fix:** either declare `MigratedIDs []string` alongside the counter (and add it to `:42`/`:45` and
to the ledger's artifact list), or replace the assertion with stored-payload evidence — the spared
record retained no minted `short_id`, every other manifest member reached v1, and
`res.Migrated == uint64(len(manifest) - len(res.Spared))`. Then reword `:42` so `Migrated` is not
described as an identity set.

### C6-H4 — 04-04 Task 1's implementation instruction and its acceptance criterion are mutually exclusive. [Codex, verified orch]

`04-04:147-149` (step 3) requires the two closures to be **"ONE-LINE ADAPTERS"**:
`return migrateSweepPreviewRun(ctx, cmd, backfillOutput, backfillTimeout)`. Per `04-03:350`/`:352`,
`migrateSweepPreviewRun`/`migrateSweepApplyRun` install the deadline themselves
(`ctx, cancel := migrateWithTimeout(ctx, timeout)`), so `backfill.go` never calls it.

`04-04:218` nevertheless requires `rg -n "migrateWithTimeout\(ctx, backfillTimeout\)" cmd/engram/backfill.go`
to find the wiring, and `04-04:146` (step 2) separately asserts the var "is PASSED to 04-03's
helper — `ctx, cancel := migrateWithTimeout(ctx, backfillTimeout); defer cancel()`" *in backfill.go*.
Satisfying the grep means either duplicating the deadline install (a nested, redundant timeout) or
adding dead/comment-only text — both contradicting the thin-adapter design that is this task's
stated cycle-3 #7 fix.

**Fix:** change the acceptance grep to
`rg -n 'migrateSweep(Preview|Apply)Run\(ctx, cmd, backfillOutput, backfillTimeout\)' cmd/engram/backfill.go`
and keep the three-case behavioural deadline test as the real proof that the value reaches
`migrateWithTimeout`. Reword step 2 so the pass-through is described as an argument, not a call site.

### C6-H5 — C5-H2's mandated laggard mechanism violates a shipped invariant declared in the same file, and the acceptance criterion pins that exact call. [orch]

`04-01:305-309` (step 2c) requires replacing the laggard's `writerStore.Upsert(...)` with
`seedLegacyRecord(ctx, t, writerStore, laggardID)` **inside the mid-sweep hook `h.fn`**, and
`04-01:374` pins it: `rg -n 'seedLegacyRecord\(ctx, t, writerStore' internal/store/migrate_converge_test.go`.

`internal/store/migrate_converge_test.go:117-124` declares **PA-11a**: `h.fn` "MUST NOT CALL
t.Fatal/t.Fatalf/t.Error/t.Errorf … a fatal call from an uncertain goroutine silently fails to fail
the test. Failures are instead recorded into h's mutex-guarded error slice via h.recordErr". The
file ships `rawPayloadNoFatal` (`:448-452`) purely to honour this. `seedLegacyRecord`
(`internal/store/migrate_test.go:47-70`) calls `t.Fatalf` twice and routes through
`deleteRawPayloadKeys`/`rawPayload`, both `t.Fatalf`-family helpers.

The *intent* is correct — post-bump no `Upsert` can construct a below-target record, so raw injection
is the only option. The prescribed and gated *mechanism* is not. Task 2's `<read_first>` covers
`:33-47`, `:49-73`, `:126-180` and `:181-200`; PA-11a at `:117-124` falls in the one gap.

**Fix:** mandate a non-fatal raw-injection variant (a `seedLegacyRecordNoFatal` returning `error`
into `h.recordErr`, mirroring `rawPayloadNoFatal`), add `:117-124` to Task 2's `<read_first>`, and
re-pin the acceptance grep to the new helper's name.

### C6-H6 — The ledger's `.planning/**` boundary exclusion is load-bearing for this phase: it names only the archived sweep and misses a sibling gate in the same package that scans the LIVE milestone. [orch]

`04-01:757` excludes "`.planning/**` gates (`internal/keylinks/sweep_test.go:100`)" on the ground
that "it walks `.planning/milestones/v0.13.x-phases/` — an ARCHIVED path. The current milestone's
phases are not in its scope", promoting only if "the sweep's path being widened to the live
milestone".

`internal/keylinks/gate_test.go` — same package, not named anywhere in the ledger — runs two gates
against the **live** tree:

- `TestNoEscapedPatternsRepoWide` scans `.planning` repo-wide (`gate_test.go:71`).
- `TestActiveMilestoneKeyLinksSatisfiable` scans **`.planning/phases`** under `ModeSatisfiability` (`:106`), whose doc says its scope "is ONLY the active milestone (.planning/phases) … Satisfiability depends on the code as it stands **right now**" (`:117-126`).

`CheckSatisfiable` (`keylinks.go:314-338`) requires each `pattern:` to match the link's `from` file
or its `to` file. Phase 3's five plans — all in `.planning/phases`, all with sibling `-SUMMARY.md`
so none is skipped by `hasSiblingSummary` (`:493-497`) — declare **19 key links**, and phase 4
rewrites four of the five files they anchor on. Two are anchored **solely** in a file 04-01 Task 3
rewrites:

- `from: internal/store/migrate.go`, `pattern: migrate[.]StepsFrom` — one occurrence, `migrate.go:200`; the `to` file `internal/migrate/registry.go` cannot match a `migrate.`-qualified form.
- `from: internal/store/migrate.go`, `pattern: migrate[.]CheckAdditive` — `migrate.go:86,230`; same.

If 04-01 Task 3's manifest/single-pass rewrite drops either call from that file,
`TestActiveMilestoneKeyLinksSatisfiable` goes RED, in a package **no plan owns and no ledger row
covers**, and the boundary section explicitly told the executor not to look there. The gates are
green today (`go test ./internal/keylinks/ -run 'TestActiveMilestoneKeyLinksSatisfiable|TestNoEscapedPatternsRepoWide' -count=1` → ok).

**Fix:** replace the exclusion row with a real ledger row — trigger `PKGFILE`/`WRITE`, gate
`TestActiveMilestoneKeyLinksSatisfiable` (`internal/keylinks/gate_test.go:128`), constraint "04-01
Task 3 must preserve a `migrate.StepsFrom` and a `migrate.CheckAdditive` call site in
`internal/store/migrate.go`" — and re-scope the exclusion to `sweep_test.go` alone, stating that
`gate_test.go` IS in scope.

### C6-H7 — 04-04 Task 1's M10 integration test cannot be written where the plan puts it: `cmd/engram` has no real-Qdrant harness, and no plan owns one. [OpenCode, verified orch]

`04-04:202` (step 8) mandates: "**against real pinned Qdrant**, seed a record WITH `short_id` … and
WITHOUT `schema_version`. Invoke `backfill-short-ids --apply` … Assert: Backlog 0; the record's
`short_id` is UNCHANGED; `schema_version == 1`." The file that owns it is `cmd/engram/backfill_test.go`
(the only backfill test file in `files_modified`, and step 7's target).

`cmd/engram` has **zero** container harness — verified: `rg -l 'testcontainers|StartQdrant|dockertest'`
over the module returns `internal/store/{store_test.go,migrate_converge_test.go,migrate_faultinject_test.go,schemaversion_recallgate_test.go}`,
`internal/server/tools_test.go`, `internal/e2e/harness_test.go`, `internal/retrievaleval/retrieval_eval_test.go`
— and nothing under `cmd/`. Every shipped `cmd/engram` Qdrant test exercises the **unreachable**
path (`exitcode_baseline_test.go:257,266,295,319,360`), through the in-process `runClient` harness.

Worse, the fixture itself needs a raw Qdrant client: `payload()` unconditionally stamps
`schema_version` (`internal/store/store.go:646`), so "short_id present, schema_version absent" can
only be built by `Upsert` + raw `DeletePayload` — which is exactly why `internal/store` ships
`seedLegacyRecord`. Neither `internal/e2e/*` nor any new harness file is in 04-04's `files_modified`,
and no task text says which lane the invocation uses.

An executor therefore either silently substitutes the fake store — gutting the production-shaped
proof this step exists to give — or builds an unplanned container harness inside a task that already
carries the alias rebuild, the golden regeneration, the `pendingApplyConversion` deletion, three
registry-row deletions and a doc repair.

**Fix — one of two, in writing:**
1. Re-locate M10 to `internal/e2e` (binary-exec harness at `internal/e2e/harness_test.go:108-160`), add that file to 04-04's `files_modified`, and state how the fixture is seeded through the harness's Qdrant client.
2. Formally downgrade M10 to the two proofs that already exist — the store-level carve-out (04-01 T3's `TestMigrateExistingShortIDPreserves`, real Qdrant) and the delegation call-sequence equality (04-04's fake-store apply-parity assertion) — and record in the plan why a CLI-lane real-Qdrant proof adds nothing over their composition.

---

## Actionable non-HIGH concerns (cycle 6)

Each of these is invisible to `/gsd-execute-phase` unless it lands in a PLAN.md task, action,
acceptance criterion, verify command, must_have, prohibition, or explicit deferral.

- **C6-M1 — `04-02` Task 2's partial-failure fixture arithmetic is inverted; the test cannot pass as specified. [orch]** `04-02:341` correctly mandates `inj.arm(2, 0, faultBeforeInvoke)` over a 4-record fixture (`:338`). Against the shipped injector (`migrate_faultinject_test.go:186-187`), ordinal 1 succeeds and ordinals 2/3/4 plus every retry fail forever → **1 converged, 3 stuck**. But `04-02:345` says "the OTHER THREE records DID converge" and `:350` says "the three already-converged records". The shipped precedent at `:408` (`arm(2, 0, …)` over six records) asserts `res.Migrated == 1`. Also, the plan identifies "the victim" positionally, while the precedent (`:434-440`) derives the succeeding id from captured wire traffic *because "Qdrant's scroll order is not a contract"*. **Fix:** correct the expected counts to 1 converged / 3 stuck, and derive the converged id from wire traffic rather than position.

- **C6-M2 — `migrateStatusFacetLimit` cannot be both a `const` and lowerable from a test. [orch]** `04-02:167` mandates `const migrateStatusFacetLimit uint64 = 1024` and `:199` pins it as "a named constant, not a literal at the call site"; `:200` requires driving `MigrateStatus` "against a fixture holding MORE distinct versions than a deliberately-lowered bound (inject via a test-only seam, or seed `migrateStatusFacetLimit+1` distinct versions … the executor picks)". A `const` cannot be lowered. The same-package precedent resolves exactly this with a package-level **var** — `internal/store/spine.go:23-28`, "A package-level var, not a const, so a test can force it". **Fix:** specify `var migrateStatusFacetLimit uint64 = 1024` with that precedent named, and delete the "executor picks" fork (Codex raised the same unresolved-seam concern independently). Seeding 1025 distinct versions against real Qdrant is not a viable alternative.

- **C6-M3 — Search S8 provably could not have produced row E1, so re-running it at the next VERSION bump will miss that class. [orch]** `04-01:610` credits S8 (`rg -ln 'CurrentVersion' internal cmd`, "11 files") with finding "E1, E2, E3, E4". `internal/store/migrate_test.go` — E1's file — contains **zero** occurrences of `CurrentVersion` (verified: the 11 files are `internal/migrate/{migrate.go,migrate_test.go,registry.go}`, `internal/server/schemaversion_wire_test.go`, `internal/store/{migrate.go,migrate_converge_test.go,schemaversion_compat_test.go,schemaversion_stamp_test.go,schemaversion_test.go,store.go,store_test.go}`). E1 breaks through the `Target: 0` **sentinel** (`internal/store/migrate.go:109-112`), not a textual reference. The ledger's re-audit obligation (`04-01:761`) is built on re-running these searches, so a search that cannot reproduce its own row is a self-extension defect. **Fix:** add a ninth search shape — call sites passing a zero/default value to an API whose default resolves through the changed constant (`rg -n 'MigrateOptions\{' internal cmd`) — and correct S8's "Found" column.

- **C6-M4 — `04-01` Task 2's action prose contradicts the ledger it is derived from. [orch]** `04-01:246` says "**Nine** files reference it; **eight** derive from it and self-adapt. **Two** do not" — internally inconsistent (8+2≠9) and wrong on the file count (11). `:364` compounds it ("the **seven** other consumers", then names five). `04-01:727` (ledger §E) says "Eleven files … seven DERIVE … one is a pin … three carry work", which is correct. **Fix:** restate `:246` and `:364` from the ledger.

- **C6-M5 — `guides/cli.md`'s destructive-command prose is left stating the opposite of what this phase ships, in a file that IS inside the ledger's claimed D-section coverage. [OpenCode]** `docs-site/src/content/docs/guides/cli.md:136-138` reads "This applies to every command the blast-radius table classifies `destructive`: today, `prune-expired`, `migrate-remap-owner`, and `spine-review purge`" — after wave 3 the admission predicate is `!ReadOnly`, `migrate revert` is a fourth destructive command, and `migrate` previews-by-default while classified non-destructive. `:143-145` reads "Every other mutating operator command — `reindex`, `summarize-missing`, and `backfill-short-ids` — … keeps its pre-existing opt-in **preview** idiom, `--dry-run`" — which 04-04 makes flatly false, and which is the very break 04-04 exists to document. Ledger row D2 covers only the **anchored** region at `:135`; D7 covers only the `--timeout` table at `:375-378`; the surrounding prose is owned by nobody, and `guides/cli.md` is not in 04-04's `files_modified`. **Fix:** add a ledger row (trigger `DOC`+`CMD`) and assign the destructive-list sentence to 04-03 Task 1 step 8b and the `backfill-short-ids`/`--dry-run` paragraph to 04-04 Task 3, adding `guides/cli.md` to 04-04's `files_modified`.

- **C6-M6 — C5-M5's sixth occurrence survives in a `<read_first>`, and the plan's own closure gate cannot see it. [orch]** `04-03:188` reads "M12 — RequireApply **must derive from** `!ReadOnly`" — unambiguously the rejected predicate (b) — in the block an executor reads *before* writing code. The gate at `04-03:529` matches only `derived from !ReadOnly` / `derives from !ReadOnly`, so "must derive from" matches neither and the plan self-certifies closed (verified: the gate returns nothing). Secondarily, `04-03:39`/`:131`/`:529` cite the legitimate admission occurrences as "`:167`, `:207`, `:511`" — pre-revision line numbers; `:167` is now blank and `:207` contains no `!ReadOnly`. **Fix:** reword `:188`, widen the gate's alternation to `(derive|derives|derived) from`, and re-anchor the three line citations.

- **C6-M7 — `04-02:43` still carries the "added/changed keys" wording C5-M2 was raised about, and `04-02:373` asserts it does not. [orch]** `:39` was correctly reworded to key-presence-only; `:43` still reads "ONE SetPayload that carries the **added/changed keys** AND the schema_version stamp", one line below, in the must_have an executor reads when implementing write order. `AddedKeys`/`RemovedKeys` are presence-only (`internal/migrate/additive.go:11-24`). `:373`'s acceptance criterion — "no must_have in this plan claims value-changed keys land" — is literally false against `:43`. **Fix:** strike "/changed" at `:43`.

- **C6-M8 — 04-03 Task 3's `<files>` block omits six files its own action edits. [orch]** `04-03:422-427` lists only `migrate_family.go`, `toolclass.go`, `migrate_family_test.go`, `catalog_test.go`. The action edits `operator_output_test.go` (steps 1a, 3b(b), 3b(c)), `destructive_test.go` (1b, 3c), `cmdwalk_test.go` (3b(a), 3b(d)), `operror_test.go` (3b(e)), `guides/cli.md` (1a), and both goldens (step 6). All six are in the plan-level `files_modified`, so there is no missing owner — but a task-scoped executor reading only `<files>` will not know it may touch them. Same omission, smaller, in `04-02` Task 2's `<files>` (`schemaversion_stamp_gate_test.go`, required by its own `:367` gate and prohibition `:68`).

- **C6-M9 — six `-run` selectors name a test the task's action text never mandates by that exact name; one of them can go fully empty. [orch]** `go test -run` matching nothing exits 0 and prints `[no tests to run]`. `04-02:441`'s `-run 'TestStartupWarnPendingMigrations|TestWarnPendingMigrations'` names **neither a shipped test nor a name the task mandates** (step 3 says only "Add a Qdrant-backed server integration test"); the `<automated>` at `:448` is saved only by its third, shipped alternative. Same shape for `TestNewMintingStep`, `TestV1FillShortID` (`04-01:201`), `TestMigrateV0ToV1MintEndToEnd`, `TestMigrateExistingShortIDPreserves`, `TestMigrateDryRunWritesNothing`, `TestMigrateFullBacklogProjection`, `TestMigrateManifestBacklogAppeared` (`04-01:479`), and `TestMigrateStatus…` (`04-02:195`) — all named only in the acceptance criteria or the plan's artifact ledger. 04-02 already shows the fix at `:319` ("all subtests below name `TestMigrateRevert...` so the phase's `-run TestMigrateRevert` selector reaches them"). **Fix:** mandate the exact function name in each creating step, as `04-02:319` does.

- **C6-M10 — `-run TestMigrate` in `cmd/engram` cannot distinguish "the new tests pass" from "the new tests do not exist". [orch]** `04-03:406`, `:531`, `04-04:214`, `:408` use `-run 'TestMigrate…'` against `cmd/engram`, where **five shipped, unrelated** owner-migration tests already match (`cmd/engram/migrate_test.go:71,86,104,133,145`). The selector reports green off those five regardless of the migrate-family tests. It is not a phantom, but it is a gate that cannot go RED for the reason it was written. **Fix:** anchor on the new file's test prefix (e.g. `TestMigrateFamily…`) and mandate that prefix in `04-03` Task 2/3.

- **C6-M11 — `guides/upgrade.md` §5's `--timeout` enumeration is already stale, and 04-04's rationale for not touching it asserts the opposite. [OpenCode, verified orch]** `upgrade.md:142-144` reads "This is the opposite convention from **four of the six** operator commands' own `--timeout` (`reindex`, `prune-expired`, `summarize-missing`, `backfill-short-ids`)". The shipped `zero-disables` group has **ten** members — the four named plus all six `spine-review` leaves (`cmd/engram/operator_output_test.go:452-460`). `04-04:318` says "the shipped documentation therefore remains accurate as written and needs no correction here", which is false today and triply false once `migrate`, `migrate status` and `migrate revert` join the same group. Nothing reads §5 back. **Fix:** replace the "remains accurate" rationale with a two-line amendment to §5's enumeration (or a footnote that the group has since grown), in the same `## Unreleased` edit 04-04 T3 already makes.

- **C6-M12 — `MigrateStatus`'s truncation detector can false-positive on a concurrent write, and the startup warning swallows the distinction. [OpenCode]** `04-02:181` makes the sum invariant a **production** error: `sum(Buckets) + sum(Future) + Absent != Total → return an error naming migrateStatusFacetLimit`. Facet, absent-`Count` and whole-collection `Count` are three non-atomic RPCs; a write landing between them makes a correct implementation report "histogram INCOMPLETE". 04-02 T3's `warnPendingMigrations` then folds any error into one "could not check for pending schema migrations" line at every startup on a busy collection — log noise indistinguishable from transport trouble. **Fix:** distinguish invariant-mismatch from truncation in the error text (only `len(hits) == migrateStatusFacetLimit` implies truncation), and retry once when the delta is small.

- **C6-L1 — the "exactly two bare-name forms" and "twelve negative gates" counts are both wrong.** Actual: ~30 gates, nine bare-name forms, five of them properly paired (`SchemaVersion: migrate.Version(1)`, `BLOCKING for Phase 4`, `ten entries`, `a destructive operator command previews by default`, `dry-run`). 04-04 is the only plan whose INV-2 self-audit is sound; `04-01:62`'s claim that every gate in that plan is "DECLARATION-SHAPED or CALL-SHAPED, never bare-name-shaped" is false about its own Task 2. **Fix:** restate the counts and list all nine.

- **C6-L2 — `operatorCommandFiles`' doc comment goes stale, and the ledger re-authors the stale count.** `cmd/engram/operror_test.go:174` says "lists **the six** operator command source files"; `04-03:490` says only "Add `"migrate_family.go"` to the literal list" (contrast `:478`, which does say "extend its doc comment" for a sibling), and `04-01:654` repeats "**SIX** … SOURCE FILES". No test reads the comment. **Fix:** add "and update the doc comment's count phrase to a non-counting sentence" to `04-03:490`, mirroring the discipline `04-02:300` applies to "ten entries".

- **C6-L3 — 04-04 promises a `task license:check` backstop no task runs.** `04-04:68` asserts "`task license:check` still runs as an acceptance gate … this is the phase's last wave", but none of the three `<automated>` blocks (`:235`, `:303`, `:365`) nor the `<verification>` section runs it. 04-04 creates no new Go file so there is no live risk, but the phase-final backstop does not exist. **Fix:** append `&& task license:check` to `04-04:365`.

- **C6-L4 — 04-03's C5-L8 rationale rests on a claim 04-04 does not honour.** `04-03:365` justifies the non-nil-slice requirement with "04-04 Task 1 step 6c ADDS a `migrate`-family entry to that map"; `04-04:190-192` actually **replaces** the `backfill-short-ids` entry with `migrateReportDoc` and never adds `statusReportDoc`. The direct unit assertion at `04-03:398` is therefore the only gate, and its test name is unspecified — so Task 2's `-run 'TestMigrate|…'` reaches it only by luck. **Fix:** correct the rationale and mandate the assertion's test name.

- **C6-L5 — anchor rot across the plans.** `guides/cli.md`'s three-group table is cited as `:373-377`/`:377`; the header is at `:375` and the zero-disables row to edit is `:378`. `04-03:430` points at `operror.go` for `usageErrorf`, which is at `client_common.go:251`. `04-02:292` still names the retired `revertRefusalErr`. `04-02:326`/`:375` cite `spine.go:42-67` for a function at `:46-68`. `04-03:626`/`:630`/`:634` omit `migrate status`/`migrateStatusTimeout` from the C5-M6 summary ledger (the Task-2 action and ledger row A4 are correct). **Fix:** re-anchor.

- **C6-L6 — two more C5-L3-class prose-staleness sites lie outside the plan's own sweep.** `internal/store/store_test.go:3006-3008` ("While CurrentVersion is 0 the value is a synthetic negative sentinel") and `internal/migrate/registry_test.go:139-142` ("migrate.Registry ships EMPTY this phase") both go factually false at `CurrentVersion = 1`. Neither line contains the token `CurrentVersion`, so `04-01:209`'s acceptance grep cannot find them, and ledger row E5 explicitly instructs *not* to touch `store_test.go`. Both tests still pass — cosmetic, but it is exactly the class E4 exists to catch. **Fix:** name both as owner obligations under E4, or widen E4's sweep to `Registry.*EMPTY|CurrentVersion is 0`.

- **C6-L7 — the ledger's A24 note overstates what `TestCatalogExitCodesMatchMapper` proves. [Codex]** That gate unions the hand-maintained `nonConnectProducedCodes` into the expected mapper set (`cmd/engram/catalog_test.go:347`) and compares it with the catalog (`:368`). It verifies agreement among declared sets; it does not verify that a live command produces each allowlisted code, despite `04-01:656`'s wording. Inert for this phase (no new constant), but the row should say "consistency gate", not "production evidence". Same for A22: it cannot detect omission from its own literal list — the RED-first experiment validates coverage only *after* registration.

- **C6-L8 — `rg -n` on the second stage of a comment-filter pipe reports line numbers of the filtered stream, not the source file.** Every `{ ! rg -v '^\s*//' FILE | rg -n PAT; }` gate that fires will name a wrong line. Cosmetic, but it will cost an executor time. **Fix:** drop `-n` from the second stage, or use `rg -n PAT FILE | rg -v ':\s*//'`.

- **C6-L9 — 04-04's coverage-migration naming obligation covers only the three `internal/store` deletions; two `cmd/engram` tests go stale in the same wave. [OpenCode]** `TestBackfillCmdHasDryRunAndTimeoutFlags` (`cmd/engram/backfill_test.go:8-13`) asserts the `--dry-run` flag exists — deleted by T1 step 1 — and `TestBackfillSummaryUnchanged` (`:33-39`) pins the deleted `backfillSummary` output literals. Step 7's "rewrite existing backfill tests" covers them implicitly, but the headline "deleting a test is a COVERAGE DECISION … name its replacement" discipline is stated only over `store_test.go`. **Fix:** extend the naming obligation to these two.

- **C6-L10 — the deleted-formatter gate matches shipped `t.Errorf` FORMAT STRINGS, which the plan's worklist does not name. [orch]** `! rg -n 'func backfillSummary|…|backfill(Summary|ReportDoc|OutputDoc)\(' internal/ cmd/` also matches `cmd/engram/backfill_test.go:35` and `:38` (`t.Errorf("backfillSummary(3, true) = %q, want %q", …)`) — string literals, not call sites. `04-04:190-192` (step 6c) enumerates only two sites, both in `operator_output_test.go`. The finished tree is correct because step 7 rewrites the file, but the executor's worklist is short by two, in the same "the sweep found N and stopped" shape the ledger exists to end. Note the discovery grep the plan does mandate (`04-04:163`, `rg -n "BackfillShortIDs|missingShortIDFilter" internal/ cmd/`) does **not** find them — the formatters are a separate name family.

---

## Codex Review (cycle 6)

## Summary

Cycle 6 materially improves the plans: the `CurrentVersion` blast radius, revert refusal exit status, command registries, and several self-invalidating greps are now handled credibly. However, the review is not converged. I found two execution-blocking contradictions and one surviving class defect in the claimed negative-grep audit. Overall risk remains **HIGH** until these are corrected.

## Strengths

- The `CurrentVersion` 0→1 blast radius is now explicitly owned. The revised plan correctly accounts for zero being a default-target sentinel in `Store.Migrate` ([internal/store/migrate.go:109](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:109)) and for `payload()` stamping the current version. Task 2’s negative-target repair and raw legacy-record injection directly address the cycle-5 failures.

- The exhaustive revert preflight uses the repository’s actual cursor-paginating iterator. `scrollAllPoints` advances `NextPageOffset` until exhaustion ([internal/store/spine.go:46](/Volumes/Code/github.com/seanb4t/engram/internal/store/spine.go:46)), unlike the migration loop’s deliberately repeated `Offset:nil` batches ([internal/store/migrate.go:182](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:182)). The multi-page refusal test is therefore well targeted.

- The exit-code analysis is correct. `exitCodeBaseline` has a count and uniqueness pin, not command-set equality ([cmd/engram/exitcode_baseline_test.go:437](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/exitcode_baseline_test.go:437)). No baseline row is mechanically required for the new operator path. `exitUsage` is defensible because its shipped meaning is “usage or validation error” ([cmd/engram/client_common.go:220](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/client_common.go:220)); an unrevertible `--to` target is a validation refusal, while bare preview can legitimately exit zero.

- The newly recognized `operatorCommandFiles` coverage hole is real. The shipped registry is a literal list ([cmd/engram/operror_test.go:174](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operror_test.go:174)), so adding `migrate_family.go` plus a deliberate RED experiment is the correct repair.

- The histogram truncation detector is much stronger than the prior token gate: a whole-collection count compared with facet-derived totals is independent evidence rather than a tautology.

## Concerns

- **HIGH — The manifest-intersection test is specified against the wrong result type.**  
  Plan 04-01 requires `!slices.Contains(res.Migrated…)` ([04-01-PLAN.md:470](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-01-PLAN.md:470)). But `MigrateResult.Migrated` remains numeric telemetry, while only `Spared` and `Appeared` become identity sets; the same plan repeatedly requires comparisons such as `res.Migrated == 300` ([04-01-PLAN.md:467](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-01-PLAN.md:467)). `slices.Contains` cannot compile on a `uint64`. More importantly, the test cannot prove by identity that the spared record was not migrated because no migrated-ID set exists.

- **HIGH — The backfill timeout requirements are mutually inconsistent.**  
  Task 1 says `backfill.go` must be a one-line adapter calling `migrateSweepPreviewRun(..., backfillTimeout)` or `migrateSweepApplyRun(..., backfillTimeout)` ([04-04-PLAN.md:147](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-04-PLAN.md:147)). Those shared functions—not `backfill.go`—call `migrateWithTimeout`. Yet acceptance requires `rg` to find `migrateWithTimeout(ctx, backfillTimeout)` inside `backfill.go` ([04-04-PLAN.md:218](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-04-PLAN.md:218)). Satisfying that grep would require duplicating timeout setup or adding dead/comment-only text, contradicting the thin-adapter design.

- **HIGH — The negative-grep class audit is still not closed.**  
  Multiple gates described as “comment-filtered” use `rg -v '^\s*//'`. That removes only full-line comments. A compliant line such as:
  ```go
  return preview(ctx, cmd) // no migrateTimeout package variable
  ```
  survives the filter and fails the gate. This affects the 04-02 `StepsFrom` gate, 04-03 `migrateTimeout`/hint gates, and 04-04 `store.MigrateOptions`, `signal.NotifyContext`, and `pendingApplyConversion` gates. The shipped precedent explicitly implements the same full-line-only behavior ([cmd/engram/operror_test.go:225](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/operror_test.go:225)); its claim that comments cannot break the gate is itself overbroad because trailing comments are retained. Priority 1(a) specifically asks whether any code comment can cause a false failure, so this is a surviving class-level HIGH.

- **MEDIUM — The histogram truncation test leaves its seam unresolved.**  
  The production design fixes `migrateStatusFacetLimit` as a constant and passes it directly to Qdrant, but the test may “inject via a test-only seam” or seed 1,025 versions ([04-02-PLAN.md:198](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:198)). A seam is not otherwise designed, while the real-Qdrant alternative is unusually heavy. This is implementable, but the plan should choose one mechanism before execution.

- **LOW — A24’s source comment overstates its independence.**  
  `TestCatalogExitCodesMatchMapper` unions the hand-maintained `nonConnectProducedCodes` into the expected mapper set ([cmd/engram/catalog_test.go:347](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog_test.go:347)) and compares that with the catalog ([cmd/engram/catalog_test.go:368](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/catalog_test.go:368)). It does not verify that a real command produces each allowlisted code, despite the comment claiming a stale entry fails. This does not block the phase because no new exit-code constant or allowlist entry is planned, but the ledger should call it a consistency gate, not production evidence.

## Suggestions

- Replace the invalid `slices.Contains(res.Migrated, …)` assertion with stored-payload evidence:

  - confirm the spared record retained no newly minted `short_id`;
  - confirm every other manifest ID reached v1;
  - confirm `res.Migrated == len(manifest)-len(res.Spared)`.

  If migrated identity itself is a required contract, add a deliberate `MigratedIDs []string` field rather than pretending the counter carries identities.

- In 04-04, change the timeout acceptance check to:
  ```sh
  rg -n 'migrateSweep(Preview|Apply)Run\\(ctx, cmd, backfillOutput, backfillTimeout\\)' cmd/engram/backfill.go
  ```
  Keep the behavioral deadline tests as the real proof that the value reaches `migrateWithTimeout`.

- Replace shell comment filtering with AST-based checks where practical. At minimum, strip inline comments using Go tokenization rather than `rg -v`. String literals should also be distinguished from identifiers/calls when the asserted property is syntactic.

- Commit to a histogram-limit test seam now—prefer an unexported parameterized helper used by production with `migrateStatusFacetLimit` and by tests with a small bound. Keep the public constant and production call path fixed.

- Amend the ledger to describe A22 and A24 honestly:

  - A22 cannot detect omission from its own literal list; the RED experiment validates coverage only after registration.
  - A24 checks agreement among declared sets, not that a live path produces every allowlisted code.

## Risk Assessment

**HIGH.** Most cycle-5 closures are genuinely present and the forward-reference sequencing is substantially improved, but 04-01 currently specifies a non-compiling assertion, 04-04 contains mutually incompatible implementation and acceptance requirements, and the headline negative-grep class claim is still false for trailing comments. These are plan-execution blockers, not cosmetic concerns.

---

## OpenCode Review (cycle 6)

# Cycle-6 Cross-AI Plan Review — Phase 4 (Migration CLI & First Customer)

**Summary.** The cycle-6 revision (commit `ec1e00bc`) is substantially sound against the shipped tree. I independently re-ran both class sweeps and the verdict is: the automated `<verify>` blocks are now clean — no phantom selectors, no remaining forward references that a bidirectional gate can expose, and the negative-grep discipline holds in every automated block. However, the "class" claims are narrower than advertised in two places: three bare-name negative greps survive in acceptance/verification *prose* (outside the automated blocks), and the ledger's exclusion-7 boundary ("prose no test reads") is tripped twice by the phase's own edits. The one genuinely load-bearing hole is 04-04 M10's "real pinned Qdrant" CLI test, which has no harness anywhere in the phase. The C5 closure claims are otherwise real and mechanically verified — E1/E2/E3 match the shipped tests verbatim, the exit-taxonomy claim is correct, and every newly-added gate's mechanism exists as described.

## Strengths

- **The C5-H2 / PA-10a discharge is the correct surgery, verified line-by-line.** E1 cites `internal/store/migrate_test.go:321-330` verbatim — the shipped tail really does call `s.Migrate(ctx, MigrateOptions{Target: 0})` and asserts both `res.Migrated == 0` and no stamp, and post-bump `Target: 0` is rewritten to `CurrentVersion` at `internal/store/migrate.go:110-112` *before* the `target <= 0` short-circuit at `:128-130`. The plan's reframe (negative `Target` plus a default-target counterpart assertion) is the only non-vacuous repair. E2's laggard (`migrate_converge_test.go:152-177`) and the explicit `Target: 1` (`:181-185`) are exactly as cited, and the replacement constructor `seedLegacyRecord` exists at `migrate_test.go:47`. E3's self-adapting row set exists at `schemaversion_compat_test.go:43-84`, including the `olderThanCovered` transfer that makes the first-ever `older-explicit` execution observable.
- **INV-1 is enforced by real bidirectional gates, and the placements are correct.** `TestTimeoutGroupMatrix` checks both directions (`operator_output_test.go:640-648`), so `migrate revert`'s group entry genuinely must land in the same task as the command (04-03 T3 step 1a split from T2 step 0c is load-bearing, not cosmetic). `TestDestructiveCommandsRequireApply`'s reverse direction (`destructive_test.go:101-104`) genuinely forces `pendingApplyConversion`'s deletion into 04-04's T1, as the plan now specifies. The one cycle-6 self-catch (revert entry moved out of T2) is verified necessary against the shipped gate.
- **`RegisterDestructive` mechanics as revised are faithful to source.** The panic gate is at `destructive.go:116-122`, the predicate is `class.Destructive` at `:47`, `addApplyFlag` composes Usage from the rule Sentence at `:57-63`, and the RunE dispatch returns the closure's error verbatim at `:124-131`. The additive-admission extension (`TestDestructiveGatePreventsMutation`'s throwaway-command pattern, `destructive_test.go:167-195`) can resolve `backfill-short-ids` (already `Destructive:false` at `toolclass.go:195`) at wave-3 Task 1 without waiting for the `migrate` row — no forward reference there.
- **The new ledger rows' mechanisms all check out.** A22: `operatorCommandFiles` is a 6-file literal at `operror_test.go:179-186` driving `TestNoBareOperatorErrorReturns` (`:212-242`), invisible to set-equality scans as claimed. B5a: the interceptor's default branch `t.Fatalf`s on any non-Query/Scroll/Count request implementing `GetFilter()` non-nil (`schemaversion_recallgate_test.go:898-901`); `Facet` carries `Filter` unset in the plan's literal, so the constraint is real and load-bearing. D8: exactly ten `HintCode` constants at `argerror.go:27-36`, the "ten hint codes" heading and the "cannot list a code the server does not emit" claim are verbatim in `errors.md:94-97`, and `rg -ln 'hint=' --glob '!**/*_test.go' cmd internal/store internal/surfaces internal/migrate` returns nothing today — the factual chain under D8 is fully verified. B1's companion claim: the `partialWriteClassification` doc comment does say "ten entries" against an 11-row table (`schemaversion_stamp_gate_test.go:356-375`) — already stale, as the plan says.
- **The pinned `StepsFrom` invocation is correct — and its failure mode is loud.** `StepsFrom(steps, from, to)` (`internal/migrate/registry.go:92-127`) walks forward `from`→`to` via a `byFrom` map and *errors* on the wrong-arg form (`StepsFrom(steps, 2, 0)` returns "no step chain from version 2 to 0: broke at 2", not a wrong-order chain). Reverting v2→v0 via `StepsFrom(steps, 0, 2)` + reverse yields the correct inverse order `[1→2, 0→1]`, matching the plan's pinned test. Worth noting for the executor: a wrong-order implementation errors noisily rather than producing a subtly wrong chain.
- **Priority-4 claim verified exactly as stated.** `TestExitCodeBaselineRowCount` (`exitcode_baseline_test.go:440-453`) is a count pin (`wantRows = 37`) plus name-uniqueness only — no set-equality against any live command set in either direction — so the new refusal path legitimately needs no row. The plan's code choice is the taxonomically defensible one: `exitUsage = 2 // usage or validation error` (`client_common.go:222`) matches a range-validation refusal, and `exitFindings`'s own doc ("the command itself succeeded", `client_common.go:227-235`) indeed disqualifies 7.
- **The purge/fault-injection precedents are real.** `spinePurgeApplyRun` re-previews inside the apply closure (`spine_review_purge.go:365-377`) — the H5 pattern 04-03 copies verbatim. The fault injector's "count == 0 means unbounded" idiom is at `migrate_faultinject_test.go:112-115` and the interceptor type-switches on `*qdrant.SetPayloadPoints` only (`:176-180`), so DeletePayload passes through — the C4-H6 two-phase test design is mechanically sound. `PurgeResult.Spared []string`/`.Appeared []string` with the exact semantics cited (`spine.go:1239-1250`) anchor C4-L6, and `scrollAllPoints` (`spine.go:46`) plus the `spineScrollBatch` var (`spine.go:28`) make the multi-page preflight fixture implementable as specified.
- **No phantom selectors.** I resolved 30+ named tests across all four plans; every `-run` name is either shipped today (e.g. `TestSchemaVersionNeverGatesRecall` at `schemaversion_recallgate_test.go:1148`, `TestNoUnregisteredConditionalRejection` at `conditionalsweep_test.go:61`) or created by an earlier/equal task in the same plan, with exact names pinned in the actions or acceptance criteria. The `CurrentVersion` consumer set is exactly the 11 files the ledger's S8 claims.

## Concerns

- **HIGH — 04-04 M10's "real pinned Qdrant" integration test has no harness and no owning file.** Task 1 step 8 says: seed a record with `short_id` present and `schema_version` *absent*, then "Invoke `backfill-short-ids --apply`". But `cmd/engram` contains zero testcontainers usage (no `TestMain`, no container, no skip discipline — verified by grep over `cmd/engram/*_test.go`); its `runClient` harness (`clienttest_test.go:210`) is in-process cobra against dead endpoints. The closest real-Qdrant harnesses are `internal/server`'s `TestMain`+testcontainer (`tools_test.go:119-223`) and `internal/e2e`'s binary-exec harness (`harness_test.go:108-160`) — and 04-04's `files_modified` includes neither `internal/e2e/*` nor any new cmd/engram scaffolding file, and no task text says which lane the invocation uses or how the unreachable-through-public-writes fixture is seeded (`payload()` always stamps, `store.go:646`; the fixture requires raw key deletion under a real Qdrant client). An executor must either silently substitute the fake store — gutting the production-shaped proof — or build an unplanned ~100-line container harness inside a task that already carries the alias rebuild, the golden regen, the `pendingApplyConversion` deletion, and the doc repair. Both halves (store-level carve-out at 04-01's `TestMigrateExistingShortIDPreserves`; delegation call-sequence equality via the fake) are proven elsewhere, so the recommended fix is to either (a) explicitly re-locate M10 into `internal/e2e` and add that file to `files_modified`, or (b) downgrade M10 in writing to the two proven halves and state why the CLI-lane real-Qdrant proof is redundant.
- **MEDIUM — `cli.md:136-139` goes stale one line below the region the phase regenerates, and no gate reads it.** Shipped text: "This applies to every command the [blast-radius] table below classifies `destructive`: today, `prune-expired`, `migrate-remap-owner`, and `spine-review purge`. A bare invocation reports what the sweep *would* do…". After 04-03 T1's sentence change regenerates the anchored region at `:135` ("a **mutating** operator command…"), this hand written neighbor is wrong twice over: it scopes to "destructive" while the sentence above says "mutating", and its enumerated "today" list omits `migrate`, `migrate revert`, and `backfill-short-ids`. `surfaces:gen` rewrites only the anchored span (Taskfile.yaml:244-264); `TestSurfaceConformanceProseFiles` checks anchored sentences, not this prose. This is precisely the D7 failure class — "a documented surface that is wrong AND passing" — at a site the phase itself edits. It needs a D-row or an owner bullet on 04-03 T1 step 8b.
- **MEDIUM — A red-golden window exists between 04-03 T2 and T3-step-6, made silent by T2's verify shape.** `TestHelpGolden`/`TestCatalogGolden` compare a live-tree walk against committed fixtures (`golden_test.go:290,306`); T1's regeneration (`step 8b`) predates the three new commands, so both tests are RED from the moment T2 lands `migrate`/`migrate status` until T3 step 6's second regeneration. The plan documents this (C4-M2), but T2's automated block runs only named subsets — no unfiltered `go test ./cmd/engram/ -count=1`, unlike T1 and T3 — so a task-atomic commit after T2 lands the tree red with nothing in that task's gates observing it. Either regenerate goldens at T2 end as well, or state the known-red pair in T2's verify comment.
- **MEDIUM — The explicit "do not amend §5/§6, it remains accurate" rationale in 04-04 T3 step 1 is factually wrong, today and more so after this phase.** `upgrade.md:140-143` says the zero-disables convention covers "four of the six operator commands' own `--timeout` (`reindex`, `prune-expired`, `summarize-missing`, `backfill-short-ids`)". Against the shipped tree, the zero-disables group is 10 of 12 (`operator_output_test.go:452-460` — the six `spine-review` leaves were already added). The phase adds `migrate`, `migrate status`, `migrate revert` to that same group *and* documents them in the same `## Unreleased` section's new entry. The sentence is stale before this phase touches it and triply stale after; "remains accurate as written" is the only phase rationale, and it fails. Note also D7 (the `cli.md` table at `:373-378`) *is* handled — this is its ungated sibling in the guides.
- **LOW — Three bare-name negative greps survive in acceptance/verification prose, so the INV-2 class claim holds only for automated blocks.** (i) 04-02 Task 2 acceptance: `rg -n "Direction|Reverse" internal/store/migrate.go` finds no direction parameter — bare-name, where 04-01's own sibling gate for the identical property is declaration-shaped (`^\s*(Direction|Reverse)\s`); a compliant doc comment saying "no Direction parameter; the Reverse walk lives in Store.Revert" fails it on a fully compliant file. Green today (verified: no match), fragile by form. (ii) 04-01 Task 2 acceptance: `rg -n 'Target: 1' …migrate_converge_test.go` returns nothing — bare-name, and the mandated PA-10a prose rewrite could legitimately write "the previous explicit `Target: 1`"; no paired comment-text prohibition accompanies this one (unlike its two neighbors, which have them explicitly). (iii) 04-03's plan-level Verification: `rg -n "migrateLastPreviewManifest" cmd/engram/` returns nothing — bare whole-package, while the same name is comment-filtered at task level. None are in `<automated>` blocks, so none halt execution — but the cycle's "all twelve audited and recategorised" claim is true of the automated blocks only.
- **LOW — Stale cross-plan references.** 04-04-PLAN.md:276 says "(the `migrateRevertCmd` row already landed in 04-03 Task 1 step 9)", contradicting 04-03:238 ("MOVED TO TASK 3") and 04-03:524 ("in THIS task (step 1b), not Task 1") — the gate placement is right; only the pointer is stale. Separately, 04-03's artifacts ledger says "`migrate` and `migrate revert` added to the `zero-disables` `timeoutGroups` entry" but omits `migrate status`, which T2 step 0c correctly adds — the producer ledger under-reports its own artifact.
- **LOW — Two stale citation/metadata nits.** 04-03 T3's `read_first` cites "cmd/engram/operror.go (usageErrorf → exit 2)"; `usageErrorf` lives at `client_common.go:251` (operror.go holds `classifyOperatorErr`/`classifyOperatorErrConstruction`). And 04-03's C5-L8 justification says "04-04 Task 1 step 6c ADDS a `migrate`-family entry to [TestOperatorOutputEmpty]" — 04-04 6c *replaces* the backfill entry only (verified `operator_output_test.go:374-394` is a hand-maintained map covering 7 of 12 operator commands); the null-safety itself is genuinely pinned by the new direct zero-value assertion, so only the rationale is off, not the fix.
- **LOW — The `MigrateStatus` truncation detector has an undiscussed concurrent-write false positive.** C5-M1's fix makes the sum invariant a *production* error (`sum(Buckets)+sum(Future)+Absent != Total → return error`). Facet, absent-Count, and total-Count are three non-atomic RPCs; a live write landing between them makes a correct implementation return "histogram INCOMPLETE" spuriously — and `warnPendingMigrations` (04-02 T3) folds any error including this one into "could not check for pending schema migrations" at every startup on a busy collection. Low probability (milliseconds window), but the failure mode is log noise indistinguishable from transport trouble. A one-sentence mitigation note (retry once, or distinguish invariant-mismatch from transport error in the message) would close it.
- **LOW — 04-04's coverage-migration prohibition names only the three `internal/store` deletions; two `cmd/engram` tests become stale in the same wave.** `TestBackfillCmdHasDryRunAndTimeoutFlags` (`backfill_test.go:8-13`) asserts the `--dry-run` flag exists (deleted by T1) and `TestBackfillSummaryUnchanged` (`:33`) pins the deleted `backfillSummary` literals. Task 1 step 7's rewrite covers them implicitly, but the headline "deleting a test is a COVERAGE DECISION … name its replacement" discipline is stated only over `store_test.go`. Extend the same naming obligation to these two.

## Suggestions (mechanical)

1. **04-04 T1 step 8:** change the M10 text to name its lane explicitly — either "adds `internal/e2e/…` (add to `files_modified`) and execs the built binary with `ENGRAM_QDRANT_ADDR` against the e2e container, seeding via the harness's qdrant client", or formally replace M10 with pointers to the two existing proofs plus one sentence on why the CLI-lane real-Qdrant proof adds nothing.
2. **04-03 T1 step 8b:** add one line: "hand-edit the sentence following the anchored region at `cli.md:136-139` to match the mutating scope and current command set", and optionally record it as ledger row D9 so exclusion 7 stops owning a site the phase edits.
3. **04-03 T2 verify:** append a comment naming `TestHelpGolden|TestCatalogGolden` as known-red until T3 step 6, or add a third regeneration at T2 end.
4. **04-04 T3 step 1 / D-12:** replace "remains accurate as written" with a two-line amendment to `upgrade.md` §5's enumeration (or a footnote that the group list has since grown), and have the D-12 test assert the §5 sentence's count claims are correct *as of the edit*, not frozen.
5. Sweep the three bare-name gates into their declaration-shaped/comment-filtered forms (they already exist as siblings): `Direction|Reverse` → `^\s*(Direction|Reverse)\s`; `Target: 1` → pair with a comment-text prohibition as its neighbors do; `migrateLastPreviewManifest` → use the task-level filtered form in the Verification section too.
6. Fix the two stale pointers (04-04:276 → "04-03 Task 3 step 1b"; 04-03 artifacts → include `migrate status` in the `timeoutGroups` bullet) and the `usageErrorf` citation.
7. `MigrateStatus`: distinguish "invariant broke (racing writer)" from "facet truncation" in the error text, and retry once before erroring when the delta is small.
8. Extend the coverage-migration naming obligation in 04-04's prohibitions to the two `backfill_test.go` deletions.

## Risk Assessment

**MEDIUM.** The phase's structural claims survived a genuinely adversarial re-check: the automated gates are now self-consistent (I verified the bidirectional mechanics that make the INV-1 placements load-bearing), every `-run` selector resolves, the exit-code taxonomy claim is exactly right, and the five C5-H closures are real against source — the ledger's honesty about its own non-coverage is also accurate as sampled (S5/S6 re-runs match the ledger's enumerated hits and dismissals). What remains is concentrated, not diffuse: one feasibility hole (M10's harness), one docs-drift class at surfaces the phase edits (`cli.md:136`, `upgrade.md` §5 — exclusion 7 doing exactly what the ledger's boundary table warns about), a silent red-golden window mid-wave-3, and form-level survivors in prose gates. None of these contradict shipped behavior or block compilation, but the first two are the kind of thing that surfaces at execution as either an improvised harness or a shipped doc that contradicts the phase's own sentence one line away. If M10 is re-located (or formally downgraded) and the two docs surfaces get an owner, this drops to LOW.

---

## Consensus Summary

Both prompt-fed reviewers had full repo access and verified against shipped source; neither output
carries the `[reviewed-without-repo-access]` marker, so both count at full weight. There was no
diff-only reviewer in this cycle.

### Agreed strengths

- **The exit-code analysis (C5-H4) is correct and was independently re-derived by all three passes.** `TestExitCodeBaselineRowCount` is a count pin plus name-uniqueness with no set-equality in either direction (`cmd/engram/exitcode_baseline_test.go:440-453`), so the missing row is genuinely inert and `wantRows = 37` stands. `exitUsage` (2) is the taxonomically right code — "usage or validation error" (`client_common.go:222`), with `exitFindings`'s own doc ("the command itself succeeded") disqualifying 7 — and the render-then-return shape has an exact shipped precedent at `spine_review_verify.go:659-662`.
- **The `CurrentVersion` blast radius is now genuinely owned.** The `Target: 0` sentinel mechanism (`internal/store/migrate.go:109-112` resolving before the `target <= 0` short-circuit at `:127-129`), `migrate.Version` being a signed named type, and the E1/E2/E3 citations all check out verbatim. The two-breaker claim is correct.
- **`operatorCommandFiles` (C5-H3) is a real coverage hole, correctly diagnosed, and the RED-first experiment is the right evidence** — the gate reads only the files it names, so an unlisted file makes it silently stop covering rather than fail.
- **No phantom `-run` selectors remain.** All three passes resolved every name across all four plans; the two known phantoms are explicitly retired in place.
- **D8 is a genuinely new and well-caught finding.** `rg -n 'hint=' --glob '!**/*_test.go' cmd internal/store internal/surfaces` returns nothing today, `internal/server/argerror.go` declares exactly ten `HintCode` constants, and `reference/errors.md:94-97` does claim the table "cannot list a code the server does not emit".
- **The `BackfillShortIDs` call-shaped gate (C5-H1's flagship) really is fixed** — 12 hits against the shipped tree, all real sites, zero prose hits, with both nearby doc comments dodging the call form by construction.

### Agreed concerns

- **The negative-grep class claim is false** (Codex HIGH, orchestrator HIGH, OpenCode LOW-with-the-same-instances). All three found surviving bare-name forms; Codex and the orchestrator additionally showed that `rg -v '^\s*//'` strips only full-line comments, so the "comment-filtered" category does not defend against the trailing comment the plans themselves mandate. OpenCode graded it LOW because none of the survivors sits in an `<automated>` block; Codex and the orchestrator graded it HIGH because the class claim is the deliverable, one survivor (`Target: -1`, `04-01:373`) is unsatisfiable as written, and the C5-L1 fix created a fresh instance. **Resolved as HIGH** on the unsatisfiable gate and the fresh instance.
- **04-03 Task 2 leaves the tree RED.** OpenCode reached it via the golden pair; the orchestrator reached it via `TestOperatorOutputParity`'s bidirectional equality and three siblings. Both note that Task 2's `<verify>` is the only one of the three with no unfiltered package run, so nothing in that task observes it — while `04-03:525` asserts the opposite as the acceptance evidence for the C5-H5 class fix.
- **Docs surfaces the phase edits are left contradicting what it ships.** OpenCode found `cli.md:136-145` (destructive-command list and the `backfill-short-ids --dry-run` paragraph) and `upgrade.md` §5's stale "four of the six" enumeration, both ungated and both un-owned; the ledger's exclusion 7 is doing exactly what its own text warns about.
- **A24's ledger note overstates what its gate proves** (Codex), and A22 cannot detect omission from its own list (Codex) — both inert this phase, both worth restating honestly.

### Divergent views

- **Severity of the surviving bare-name gates.** OpenCode: LOW, because none is in an `<automated>` block and all are green today. Codex + orchestrator: HIGH, because `04-01:373`'s `Target: -1` count pin is arithmetically unsatisfiable against the doc comment step 1 mandates twelve lines above it, and because acceptance/verification prose is what an executor reads as the standing requirement. Resolved toward HIGH; the LOW reading is recorded because it is correct about blast radius.
- **04-01's `slices.Contains(res.Migrated…)`.** Codex flagged it HIGH as a non-compiling assertion; OpenCode did not reach it. The orchestrator confirmed `MigrateResult.Migrated` is `uint64` (`internal/store/migrate.go:49`) and that the same plan asserts `res.Migrated == 300` twelve lines earlier — so it is a genuine execution blocker, not a reading difference.
- **M10's harness.** OpenCode HIGH; Codex did not reach it. The orchestrator confirmed `cmd/engram` has no testcontainers usage anywhere. Kept at HIGH.
- **PA-11a vs `seedLegacyRecord`.** Orchestrator only. Neither external reviewer read `migrate_converge_test.go:117-124`, which is in the one gap Task 2's `<read_first>` leaves. Verified directly against source and kept at HIGH.

### Convergence assessment

Cycle 6 is the first cycle where the count of *newly raised* HIGHs is dominated by execution
blockers (a non-compiling assertion, an unsatisfiable count pin, two mutually exclusive
requirements, a shipped-invariant violation) rather than by fresh instances of an old class — and
the two class sweeps, while incomplete, are each incomplete in one identifiable, mechanical way
(prose-vs-`<automated>` scope for INV-2; the T1→T2 boundary for INV-1) rather than in an open-ended
one. The three previously-recurring classes (phantom selectors, forward references named by a prior
cycle, comment-invalidatable call-shaped gates) are genuinely closed.

The remaining work is bounded and mostly mechanical. It does not converge yet.

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
