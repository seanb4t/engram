---
phase: 04-migration-cli-first-customer
plan: 04
subsystem: cli
tags: [migration, schema-versioning, cli, cobra, go, deprecation]

# Dependency graph
requires:
  - phase: 04-migration-cli-first-customer (plan 01)
    provides: "migrate.CurrentVersion==1, Store.Migrate's DryRun/Manifest apply, MigrateOptions.{DryRun,Manifest}, MigrateResult.{PreviewManifest,Spared,Appeared}, CheckAdditive's H1 pre-existing-key carve-out, TestMigrateExistingShortIDPreserves"
  - phase: 04-migration-cli-first-customer (plan 03)
    provides: "migrateSweepPreviewRun/migrateSweepApplyRun, migrateWithTimeout(ctx,d), migrateReportDoc/migrateSummary, migrateFamilyStoreFromEnv, mutatingCommandNames(), pendingApplyConversion (consumed only to delete it)"
provides:
  - "backfill-short-ids as a thin delegating alias through registerDestructive, sharing migrateSweepPreviewRun/migrateSweepApplyRun verbatim with engram migrate (apply-path parity)"
  - "Deletion of Store.BackfillShortIDs/missingShortIDFilter with coverage migrated onto Store.Migrate"
  - "mutatingCommandNames() as the sole apply-required derivation (pendingApplyConversion deleted); TestDestructiveCommandsRouteThroughGate/TestApplyFlagUsageComposesRuleSentence/TestDestructiveCommandsExactFlagSet widened to it"
  - "guides/upgrade.md Unreleased #13 + D-12 bidirectional doc<->code gate (TestUpgradeGuideReconcilesBackfill)"
affects: []

# Actuals (#2632)
actuals:
  tokens: 18900
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Thin delegating alias over a shared sweep: an old standalone command becomes a flag-registration shell whose preview/apply closures are one-line adapters over the canonical command's own run funcs, so 'identical behavior' is a structural fact (call-sequence equality) rather than a maintained claim."
    - "Section-scoped doc<->code gates over whole-file negative greps: a doc-conformance test that must assert BOTH 'X is documented here' and 'X is NOT documented there' extracts the specific section via a shared boundary-finding helper rather than grepping the whole file, because a whole-file gate for one direction directly contradicts the other direction's requirement."
    - "Joint-paragraph doc assertions over independent-Contains checks: a doc-side conformance assertion that needs 'these N facts are documented TOGETHER, about the SAME subject' must check for co-occurrence within one paragraph block, not merely that each fact independently appears somewhere in a large section — three scattered unrelated mentions can satisfy N independent Contains checks without ever describing the claimed subject."

key-files:
  created: []
  modified:
    - cmd/engram/backfill.go
    - cmd/engram/backfill_test.go
    - cmd/engram/destructive_test.go
    - cmd/engram/migrate_family.go
    - cmd/engram/operator_output_test.go
    - cmd/engram/spine_review_purge_test.go
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/store/migrate.go
    - internal/store/migrate_test.go
    - internal/store/migratebacklog.go
    - internal/store/schemaversion_recallgate_test.go
    - internal/store/schemaversion_stamp_gate_test.go
    - docs-site/src/content/docs/guides/upgrade.md
    - docs-site/src/content/docs/guides/cli.md

key-decisions:
  - "cmd/engram/migrate_family.go (not in Task 1's declared files) gained two justified //nolint:unparam directives on migrateReportDoc/migrateSummary. Sharing the formatter onto the new backfill-short-ids parity row and TestOperatorOutputEmpty entry (D-11) pushed golangci-lint's unparam evidence count for the `target` parameter from 3 call sites (clean under 04-03) to 5 (flagged): every LIVE caller, production and test, passes migrate.CurrentVersion today. The parameter is kept general on purpose (mirrors revertReportDoc's plan.To, which genuinely varies) rather than hardcoded, since hardcoding would touch every call site in a file this plan does not own and would defeat the function's intended generality for a future partial-forward-migration target."
  - "TestOperatorOutputEmpty's backfill-short-ids entry is migrateReportDoc(store.MigrateResult{}, migrate.CurrentVersion, false, 0) rather than a struct literal — this correctly exercises the SAME shared formatter the parity row and production code use, catching a future normalization regression the two-nolint-directive fix explicitly accepts as the honest tradeoff over silently narrowing the function's signature."
  - "cli.md's two-idiom-split paragraph is rewritten to state the boundary as 'does this command route through registerDestructive' rather than 'the blast-radius table's Destructive column', since backfill-short-ids stays Destructive:false (additive) yet is now --apply-gated via applyRoutedAdditions — the old boundary sentence became false the moment the alias moved idioms without changing its own classification."
  - "The v0.8.4 section's cross-reference to the new Unreleased entry uses plain prose ('see Unreleased #13 above'), not a markdown anchor link, after hand-computing the Starlight/rehype-slug anchor for a heading containing backticks, a semicolon, parentheses, and a hash mark showed the slug is long and error-prone to verify by hand with no build-time link checker in this repo's lint chain; a wrong guessed anchor would silently 404 rather than fail any gate."

patterns-established:
  - "RED-first non-vacuity proof for a WIDENED conformance gate, done via a temporary in-repo mutation (a hand-assigned RunE, a throwaway --force flag) rather than a mocked/injected seam — proves the widened gate structurally cannot be bypassed by the exact class of defect it exists to catch, then reverts the mutation before committing."

requirements-completed:
  - REQ-backfill-shortids-first-step
  - REQ-migrate-never-automatic

coverage:
  - id: D1
    description: "backfill-short-ids is rebuilt as a thin delegating alias through registerDestructive: two one-line adapter closures over migrateSweepPreviewRun/migrateSweepApplyRun, no store.MigrateOptions constructed in backfill.go, sharing the migrate report envelope (D-11) instead of the deleted backfillSummary/backfillReportDoc/backfillOutputDoc"
    requirement: "REQ-backfill-shortids-first-step"
    verification:
      - kind: unit
        ref: "cmd/engram/backfill_test.go#TestBackfillPreviewsByDefaultAndSharesMigrateEnvelope"
        status: pass
      - kind: unit
        ref: "cmd/engram/backfill_test.go#TestBackfillApplyPerformsSharedEnvelope"
        status: pass
      - kind: unit
        ref: "cmd/engram/backfill_test.go#TestBackfillApplyPathParityWithMigrateApply"
        status: pass
      - kind: unit
        ref: "cmd/engram/backfill_test.go#TestBackfillCmdFlagSet"
        status: pass
      - kind: unit
        ref: "cmd/engram/backfill_test.go#TestBackfillTimeoutWiring"
        status: pass
      - kind: unit
        ref: "cmd/engram/backfill_test.go#TestBackfillDeprecatedPointsAtMigrate"
        status: pass
    human_judgment: false
  - id: D2
    description: "Store.BackfillShortIDs and missingShortIDFilter deleted (D-10); Store.MintShortID kept; the two classification-gate rows (recall-emission and partial-write) removed with the deleted method; the three deleted store_test.go behaviors (dry-run-then-apply/idempotence, context cancellation, mid-run-failure resume) each migrated onto an explicit replacement on the Store.Migrate path, including the absent-owner-key invariant and context-cancellation coverage the deleted tests carried"
    requirement: "REQ-backfill-shortids-first-step"
    verification:
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateOwnerlessRecordInvariant"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateHonorsCancel"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateV0ToV1MintEndToEnd"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_faultinject_test.go#TestMigratePartialFailureResume"
        status: pass
      - kind: unit
        ref: "internal/store/schemaversion_recallgate_test.go#TestRecallEmissionSetIsCompleteAndClassified"
        status: pass
      - kind: unit
        ref: "internal/store/schemaversion_stamp_gate_test.go#TestPartialWritePathsAreClassifiedNonStamping"
        status: pass
    human_judgment: false
  - id: D3
    description: "The three sibling conformance gates (routing, exact-flag-set both directions, rule-sentence Usage) widen from destructiveCommandNames() to mutatingCommandNames() (REVIEWS.md N1); pendingApplyConversion deleted and the membership pin moved to six names in the same task backfill-short-ids gains --apply; both widened gates proven non-vacuous by a RED-first experiment (hand-assigned RunE, a throwaway --force flag), observed to fail naming exactly the mutated command, then reverted"
    requirement: "REQ-migrate-never-automatic"
    verification:
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveCommandsRouteThroughGate"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveCommandsExactFlagSet"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestApplyFlagUsageComposesRuleSentence"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestMutatingCommandNamesMembership"
        status: pass
    human_judgment: false
  - id: D4
    description: "guides/upgrade.md's ## Unreleased #13 documents the --dry-run removal and preview-by-default change for backfill-short-ids (NOT --timeout removal, which is preserved per H8); the v0.8.4 section's stale --dry-run instruction is corrected; cli.md's two-idiom-split paragraph and upgrade.md section 5's stale timeout-group enumeration are both repaired; TestUpgradeGuideReconcilesBackfill is the D-12 bidirectional gate (doc-side joint-paragraph assertion, code-side flag/Deprecated assertion, a C4-L3 section-scoped stale-instruction assertion, and a C5-L1/C6-H1 clause-scoped no-timeout-removal-claim assertion), each proven RED once by reverting its own half"
    requirement: "REQ-backfill-shortids-first-step"
    verification:
      - kind: unit
        ref: "cmd/engram/backfill_test.go#TestUpgradeGuideReconcilesBackfill"
        status: pass
      - kind: unit
        ref: "cmd/engram/docsync_test.go#TestUpgradeGuideNamesEveryChangedCommand"
        status: pass
    human_judgment: false
  - id: D5
    description: "M10 (a record previously backfilled by the old standalone Store.BackfillShortIDs -- short_id present, schema_version absent -- converges through the alias) is discharged by composition: 04-01's TestMigrateExistingShortIDPreserves proves the store-layer behavior against real pinned Qdrant, and TestBackfillApplyPathParityWithMigrateApply + TestBackfillPreBackfilledRecordDelegates prove the alias invokes exactly the same sweep with no store.MigrateOptions of its own construction. No CLI-lane container harness was built (cmd/engram has none, and the fixture requires a raw client only internal/store can construct), matching 04-03's C6-H7 rationale"
    verification:
      - kind: unit
        ref: "cmd/engram/backfill_test.go#TestBackfillPreBackfilledRecordDelegates"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateExistingShortIDPreserves"
        status: pass
    human_judgment: false
  - id: D6
    description: "Real-binary smoke check (go run ./cmd/engram backfill-short-ids / --apply / --help against a live test Qdrant, named in this plan's <verification> section)"
    verification: []
    human_judgment: true
    rationale: "No live Qdrant instance is available in this execution environment (worktree sandbox running go test's own testcontainers-managed Qdrant only, no standalone long-lived instance for an interactive go run smoke check). All behavior this smoke check would exercise is proven at the CLI boundary via the fakeMigrateFamilyStore injection seam and at the store layer against real pinned Qdrant (internal/store's full unfiltered suite passed, including testcontainers-backed Migrate/Revert/MigrateStatus tests). A human with a live Qdrant should run the three commands named in the plan's verification section before shipping, matching 04-03's D7 precedent."

# Metrics
duration: 12min
completed: 2026-08-15
status: complete
---

# Phase 4 Plan 04: Migration CLI First Customer — backfill-short-ids Fold-In Summary

**Folded `backfill-short-ids` into `engram migrate`'s own sweep as a thin delegating alias (apply-path parity proven by call-sequence equality), deleted the now-dead `Store.BackfillShortIDs`/`missingShortIDFilter`, widened the three sibling conformance gates to the mutating command set (RED-first proven non-vacuous), and closed the doc<->code loop with a bidirectional gate on the upgrade guide's `--dry-run` removal entry.**

## Performance

- **Duration:** ~12 min (first commit 10:58:36-04:00, last commit 11:10:08-04:00)
- **Started:** 2026-08-15T14:58:36Z
- **Completed:** 2026-08-15T15:10:08Z
- **Tasks:** 3
- **Files modified:** 17 (0 created)

## Accomplishments

- `cmd/engram/backfill.go`: rebuilt as a flag-registration shell over 04-03's `migrateSweepPreviewRun`/`migrateSweepApplyRun` — `backfillDryRun`/`backfillSummary`/`backfillOutputDoc`/`backfillReportDoc` and the hand-rolled `signal.NotifyContext`/`context.WithTimeout` block are gone; `backfillTimeout`/`--timeout` PRESERVED (H8) and passed as an argument into the shared run funcs; `backfillShortIDsCmd.Deprecated = "use: engram migrate"` (D-12).
- `internal/store/store.go`: deleted `Store.BackfillShortIDs` and `missingShortIDFilter` (D-10); kept `Store.MintShortID`. The live-caller gate (`! rg -n 'func \(s \*Store\) BackfillShortIDs|func missingShortIDFilter|\.BackfillShortIDs\(|missingShortIDFilter\(' internal/ cmd/`) holds with no test-file exclusion, after migrating seven `store_test.go` call sites and three `operator_output_test.go` references.
- `internal/store/migrate_test.go`: `TestMigrateOwnerlessRecordInvariant` and `TestMigrateHonorsCancel` carry across the absent-owner-key invariant and context-cancellation behavior the deleted `TestBackfillShortIDs`/`TestBackfillShortIDsHonorsCancel` asserted; mid-run-failure resume was already covered by the shipped `TestMigratePartialFailureResume`.
- `cmd/engram/destructive_test.go`: `pendingApplyConversion` deleted and `TestMutatingCommandNamesMembership`'s pin moved to six names in Task 1 (the same task giving `backfill-short-ids` its `--apply` flag); `destructiveFlagCases` renamed to `mutatingFlagCases` with new `migrateCmd`/`backfillShortIDsCmd` rows; `TestDestructiveCommandsRouteThroughGate`/`TestApplyFlagUsageComposesRuleSentence`/`TestDestructiveCommandsExactFlagSet` (both directions) widened from `destructiveCommandNames()` to `mutatingCommandNames()` in Task 2, each proven non-vacuous by a RED-first mutation experiment.
- `docs-site/src/content/docs/guides/upgrade.md`: new `## Unreleased #13` entry; v0.8.4 section's stale `--dry-run` example corrected; section 5's stale "four of the six" `--timeout` group enumeration replaced with a rule statement pointing at `cli.md`'s live table.
- `docs-site/src/content/docs/guides/cli.md`: the two-idiom-split paragraph rewritten — `reindex`/`summarize-missing` keep `--dry-run`; `backfill-short-ids` moved to `--apply` as a `migrate` alias, boundary restated as "routes through `registerDestructive`" rather than the now-insufficient `Destructive` column alone.
- `cmd/engram/backfill_test.go`: `TestUpgradeGuideReconcilesBackfill` — the D-12 bidirectional gate, four independently-RED-provable assertions (doc-side joint-paragraph, code-side flag/Deprecated, C4-L3 section-scoped stale-instruction, C5-L1/C6-H1 clause-scoped no-removal-claim).

## Task Commits

Each task was committed atomically:

1. **Task 1: backfill-short-ids -> thin delegating alias; delete dead store code; delete pendingApplyConversion; regenerate goldens** — `9695616d` (feat)
2. **Task 2: widen the three sibling conformance gates to the mutating set** — `e95abec3` (feat)
3. **Task 3: upgrade-guide entry + D-12 bidirectional doc<->code gate** — `b51e4bdb` (docs)

## Files Created/Modified

- `cmd/engram/backfill.go` - the alias rebuild
- `cmd/engram/backfill_test.go` - rewritten alias tests + apply-path parity + M10 delegation test + D-12 gate
- `cmd/engram/destructive_test.go` - `pendingApplyConversion` deletion, six-name pin, `mutatingFlagCases` rename + new rows, three gates widened, RED-first experiments (reverted)
- `cmd/engram/migrate_family.go` - two justified `//nolint:unparam` directives (deviation, see below)
- `cmd/engram/operator_output_test.go` - `backfill-short-ids` parity row and empty-doc-map entry migrated onto `migrateSummary`/`migrateReportDoc`
- `cmd/engram/spine_review_purge_test.go` - stale `destructiveFlagCases` cross-reference comment repaired (trivial accuracy fix caused by the rename)
- `cmd/engram/testdata/{help,catalog}.golden` - regenerated for the alias's changed flag set (`--dry-run` out, `--apply` in) — the phase's third and final regeneration
- `internal/store/store.go` - `Store.BackfillShortIDs`/`missingShortIDFilter` deleted
- `internal/store/store_test.go` - three deleted test functions + `upsertRawNoOwner` helper (relocated to `migrate_test.go`)
- `internal/store/migrate_test.go` - `upsertRawNoOwner` (relocated), `TestMigrateOwnerlessRecordInvariant`, `TestMigrateHonorsCancel`
- `internal/store/migrate.go`, `internal/store/migratebacklog.go` - two cross-reference doc comments repaired (citing deleted symbols)
- `internal/store/schemaversion_recallgate_test.go`, `internal/store/schemaversion_stamp_gate_test.go` - `Store.BackfillShortIDs` rows deleted from both classification tables
- `docs-site/src/content/docs/guides/upgrade.md` - new Unreleased entry, v0.8.4 fix, section 5 enumeration fix
- `docs-site/src/content/docs/guides/cli.md` - two-idiom-split paragraph rewrite

## Decisions Made

- Adds justified `//nolint:unparam` directives to `cmd/engram/migrate_family.go`'s `migrateReportDoc`/`migrateSummary` rather than narrowing their `target` parameter — sharing the formatter onto the new backfill-short-ids test call sites (D-11) pushed golangci-lint's evidence count for "always the same value" past its threshold; the parameter is kept general on purpose (mirrors `revertReportDoc`'s genuinely-varying `plan.To`), which a signature change would defeat while also touching a file outside this task's declared scope.
- `TestOperatorOutputEmpty`'s `backfill-short-ids` entry uses `migrateReportDoc` over a zero-valued `store.MigrateResult` rather than a bare struct literal, so it exercises the same shared formatter production code uses (catches a future normalization regression the nolint fix explicitly does not paper over).
- `cli.md`'s two-idiom boundary is restated as "routes through `registerDestructive`" rather than "the blast-radius table's `Destructive` column" — the old boundary sentence became false the moment `backfill-short-ids` moved idioms without changing its own `Destructive:false` classification.
- The v0.8.4 section's pointer to the new Unreleased entry is plain prose, not a markdown anchor link — the Starlight/rehype-slug anchor for a heading containing backticks, a semicolon, parens, and a `#` is long and error-prone to hand-verify with no build-time link checker in this repo's lint chain.
- `TestUpgradeGuideReconcilesBackfill`'s doc-side assertion checks that `backfill-short-ids`, `--dry-run`, and `--apply` co-occur in ONE paragraph block inside `## Unreleased`, not that each independently appears somewhere in the section — this was a genuine RED-first finding (see Deviations): an earlier, simpler "three independent Contains checks" version PASSED even with the actual entry's content stubbed out, because those three tokens each already appeared elsewhere in the (very long) Unreleased section, describing different commands.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] golangci-lint's unparam flagged `migrateReportDoc`/`migrateSummary`'s `target` parameter after sharing the formatter onto new test call sites**
- **Found during:** Task 1, running `task lint` after migrating the `operatorParityRows()` backfill row and `TestOperatorOutputEmpty` entry onto `migrateSummary`/`migrateReportDoc`
- **Issue:** `cmd/engram/migrate_family.go`'s `migrateReportDoc`/`migrateSummary` (04-03, not in this task's declared files) already had every call site — production and test — pass `migrate.CurrentVersion` for `target`. 04-03's tree had 3 such call sites and stayed lint-clean; this task's two new call sites (the `backfill-short-ids` parity row, the empty-doc-map entry) brought the count to 5, which tipped golangci-lint's `unparam` analyzer into flagging the parameter as "always receives migrate.CurrentVersion". Confirmed by isolating: `git stash` back to the pre-task tree reproduced `0 issues`; restoring the two new call sites reproduced the 2-issue failure deterministically (cache-cleared re-run).
- **Fix:** Added justified `//nolint:unparam` directives to both functions, explaining WHY `target` is kept general (mirrors `revertReportDoc`'s `plan.To`, which genuinely varies; a future partial-forward-migration target would need it) rather than narrowing the signature, which would touch a file outside Task 1's declared scope and defeat the function's intended generality for no functional gain.
- **Files modified:** cmd/engram/migrate_family.go
- **Verification:** `golangci-lint run ./cmd/engram/...` returns `0 issues` after the fix; `gofmt -l` clean (gofmt relocates a `//nolint:` directive comment to be the line immediately preceding the declaration — the doc comment was restructured so the directive stays a clean, standalone final line rather than fragmenting a narrative paragraph).
- **Committed in:** `9695616d` (Task 1 commit)

**2. [Rule 1 - Bug] `TestUpgradeGuideReconcilesBackfill`'s first doc-side assertion drafted as three independent `strings.Contains` checks was a false-pass**
- **Found during:** Task 3, RED-first authoring of the D-12 gate (memory x6v6qxqd6f discipline) — stubbing out the new Unreleased entry's content to prove the assertion goes RED
- **Issue:** The initial assertion checked only that `"## Unreleased"`'s FULL TEXT contained `"backfill-short-ids"`, `"--dry-run"`, and `"--apply"` independently. With the new entry's body replaced by a placeholder heading, the test still PASSED: item #2's operator-command list already names `"backfill-short-ids"`, item #10 already names `"--dry-run"` (for `migrate-remap-owner`), and item #9 already names `"--apply"` (for `prune-expired`) — three scattered, unrelated mentions satisfying three independent checks without ever documenting backfill-short-ids's own removal.
- **Fix:** Rewrote the assertion to require all three tokens co-occur within ONE paragraph block (`\n\n`-delimited) inside `## Unreleased`, proving joint documentation of the same subject rather than independent presence.
- **Files modified:** cmd/engram/backfill_test.go
- **Verification:** Re-ran the RED experiment against the fixed assertion — correctly failed with the stubbed entry; passed once the real entry was restored. All four assertions confirmed RED-then-green per their own half (see Coverage D4).
- **Committed in:** `b51e4bdb` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (both Rule 1 — a lint bug surfaced by sharing a formatter correctly, and a self-authored test gate's own false-pass caught by its own RED-first discipline). No scope creep: both fixes were required to reach this task's own stated gates (`task lint` clean; the D-12 gate genuinely RED-provable).
**Impact on plan:** No functional behavior changed by either fix — one is a lint annotation with a written justification, the other tightens a test assertion this plan itself was authoring. Both are documented rather than silently absorbed.

## Issues Encountered

None beyond the two deviations above.

## User Setup Required

None - no external service configuration required. One item is deferred to a human with a live Qdrant instance: the plan's `<verification>` real-binary smoke check (`go run ./cmd/engram backfill-short-ids` / `--apply` / `--help`) could not be run in this sandboxed execution environment (no standalone long-lived Qdrant service; only `go test`'s own testcontainers-managed instances, which the full `internal/store` suite already exercised against real pinned Qdrant). See coverage item D6.

## Next Phase Readiness

- This is the phase's final wave (04-04). No later plan in this milestone consumes a symbol this plan produces.
- Final phase-wide ledger audit (04-01-PLAN.md's Conformance Registry Impact ledger, all nine search shapes S1-S9) re-run against the finished tree:
  - **S1-S6, S8** produced no hits outside already-tracked rows or already-dismissed categories (client tier, untouched packages, unrelated pre-existing "missing from" prose in `store_test.go`).
  - **S7** (docs-site/skill-reading test gates): `backfill_test.go`'s new `TestUpgradeGuideReconcilesBackfill` reads `docs-site/.../guides/upgrade.md` via the SHARED `upgradeGuideRelPath` const (defined in `docsync_test.go`, already a tracked consumer) rather than a literal path string, so S7's literal-string search does not directly surface `backfill_test.go` as a new hit — an S9-class blind spot (reaching a tracked surface through a shared identifier rather than a literal). No new row needed: the underlying doc file is already tracked by rows A18/D4/D9/D10, and no untracked file is read.
  - **S9**: this plan does not change `migrate.CurrentVersion`'s value (04-01 already did, wave 1); the two new `MigrateOptions{}` sentinel-reaching test calls (`TestMigrateOwnerlessRecordInvariant`, `TestMigrateHonorsCancel`) correctly resolve through `Store.Migrate`'s own default-target logic and pass against real Qdrant — not a stale-value risk.
  - **Boundary re-audit** (non-coverage table): "registries in untouched packages" holds (no file outside the ledger's package set was touched); "deleting a test function" is directly load-bearing here and was honored (Task 1's SUMMARY section above names each deleted `store_test.go` behavior against its replacement); the runtime-invariant and proto/codegen exclusions hold (no PA-2/PA-5a-class change, no `proto/**` edit, confirmed zero `gen/`/`ui/` diff after `task surfaces:gen`).
- `go test ./... -count=1`, `go build ./...`, `go vet ./...`, and `task` (lint + full suite, including markdown/actions/yaml/python) all green on the finished tree.

## Known Stubs

None — no stub or placeholder data was introduced by this plan.

## Threat Flags

None — this plan's changes are exactly what its own `<threat_model>` section registered and mitigated (the alias's routing through `registerDestructive`, the dead-code deletion's grep-before-delete discipline, the doc<->code gate). No new trust boundary was introduced.

## Self-Check: PASSED

- FOUND: cmd/engram/backfill.go
- FOUND: cmd/engram/backfill_test.go
- FOUND: cmd/engram/destructive_test.go
- FOUND: internal/store/migrate_test.go
- FOUND: docs-site/src/content/docs/guides/upgrade.md
- FOUND: .planning/phases/04-migration-cli-first-customer/04-04-SUMMARY.md
- FOUND: 9695616d, e95abec3, b51e4bdb

---
*Phase: 04-migration-cli-first-customer*
*Completed: 2026-08-15*
