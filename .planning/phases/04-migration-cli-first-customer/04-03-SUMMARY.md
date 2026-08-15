---
phase: 04-migration-cli-first-customer
plan: 03
subsystem: cli
tags: [migration, schema-versioning, cli, cobra, go]

# Dependency graph
requires:
  - phase: 04-migration-cli-first-customer (plan 01)
    provides: "Store.Migrate's DryRun/Manifest apply, MigrateOptions.{DryRun,Manifest}, MigrateResult.{PreviewManifest,Spared,Appeared}, migrate.CurrentVersion==1"
  - phase: 04-migration-cli-first-customer (plan 02)
    provides: "Store.MigrateStatus/MigrateStatusResult, store.RevertPlan/Store.PreviewRevert (the exported whole-range zero-write preflight), Store.Revert/RevertResult, store.RevertRefusalError"
provides:
  - "engram migrate — forward v0->v1 sweep CLI (preview by default, --apply performs the in-apply-closure re-preview + manifest∩fresh-re-derivation intersection)"
  - "engram migrate status — read-only server-side version-distribution histogram CLI"
  - "engram migrate revert --to <v> — reverse sweep CLI (whole-range refusal before any write, applies only fully-reversible ranges)"
  - "registerDestructive's admission gate generalized from Destructive:true to !ReadOnly (D-16), documented as an accepted name debt (M9)"
  - "mutatingCommandNames() — the NAMED --apply-required union (destructiveCommandNames() ∪ applyRoutedAdditions − pendingApplyConversion), replacing the rejected !ReadOnly derivation (C4-H1/M12)"
  - "migrateSweepPreviewRun/migrateSweepApplyRun/migrateWithTimeout/migrateFamilyStoreFromEnv — the shared sweep implementation and store seam 04-04's backfill-short-ids alias will reuse verbatim"
affects: [04-04]

# Actuals (#2632)
actuals:
  tokens: 24700
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Admission vs. requirement, two distinct predicates over one classification table: registerDestructive's ADMISSION gate (!class.ReadOnly, in destructive.go) answers 'may this command route through the preview/apply choke point?'; the --apply-REQUIRED set (mutatingCommandNames(), in destructive_test.go) is a NAMED union over the table, never derived from !ReadOnly — the two must never be conflated even though both live one file apart and both generalize the same original Destructive:true-only gate."
    - "In-apply-closure re-preview (H5): an --apply closure calls the preview RPC AGAIN, inside itself, capturing a fresh manifest and consuming it immediately in the same invocation — never a package-level var bridging two separate invocations. Shared by spine-review purge (03-07) and now migrate/migrate revert."
    - "Render-then-return for a refused mutation (C5-H4): render the full report document first (so json/text consumers see the complete refusal), THEN return a classified non-zero error — never return nil after rendering a refusal, which would make a REFUSED apply indistinguishable from a completed one to a script checking $?."

key-files:
  created:
    - cmd/engram/migrate_family.go
    - cmd/engram/migrate_family_test.go
  modified:
    - cmd/engram/destructive.go
    - cmd/engram/destructive_test.go
    - cmd/engram/cmdwalk_test.go
    - cmd/engram/operator_output_test.go
    - cmd/engram/operror_test.go
    - internal/surfaces/toolclass.go
    - internal/surfaces/rules.go
    - internal/surfaces/rules_test.go
    - docs-site/src/content/docs/guides/cli.md
    - skill/engram/skills/curating-memory/SKILL.md
    - cmd/engram/testdata/help.golden
    - cmd/engram/testdata/catalog.golden

key-decisions:
  - "migrateReportDoc/statusReportDoc/revertReportDoc are all FUNCTIONS (not type names) returning distinctly-named types (migrateOutputDoc, store.MigrateStatusResult reused directly, revertOutputDoc) — mirrors the 3-of-4 precedent in this package (backfillReportDoc/backfillOutputDoc, pruneReportDoc/pruneOutputDoc, reindexReportDoc/reindexOutputDoc) rather than the plan's shorthand 'type migrateReportDoc' phrasing, which would have collided a type and a func under one identifier."
  - "statusReportDoc reuses store.MigrateStatusResult directly as the CLI's JSON shape (its json tags already match the wire contract exactly) rather than declaring a parallel CLI-side type — the function's only job is the C5-L8 nil-to-empty-slice normalization."
  - "migrate revert's Long help text and the toolclass row both classify the command Destructive:true and Idempotent:true: a re-run against an already-reverted (empty above-target) range trivially preflights Reversible (empty observed-chains set) and touches zero records, matching the REST-DELETE-style idempotency reasoning delete_memory/migrate-remap-owner already carry in the same table."

patterns-established:
  - "Shared timeout helper parameterized by duration (migrateWithTimeout(ctx, d)), reused across three leaves each passing its OWN flag-backed var — never a bare zero-arg helper reading one shared package var — so `--timeout` is provably read per-leaf, not merely registered."

requirements-completed:
  - REQ-migrate-command
  - REQ-migrate-status-histogram
  - REQ-migrate-preview-apply-parity
  - REQ-migrate-revert

coverage:
  - id: D1
    description: "engram migrate previews the full backlog by default (Store.Migrate DryRun, no writes) and --apply performs the H5 in-apply-closure re-preview, migrating only the manifest ∩ fresh-re-derivation intersection (Spared/Appeared reported as identity-set differences, never a count comparison)"
    requirement: "REQ-migrate-command, REQ-migrate-preview-apply-parity"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyPreviewAndApply"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyApplyIntersection"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyReportFields"
        status: pass
    human_judgment: false
  - id: D2
    description: "engram migrate status renders Store.MigrateStatus's server-side version-distribution histogram (buckets/absent/future/future_total/total), never a scalar, with buckets/future marshalling as [] never null"
    requirement: "REQ-migrate-status-histogram"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyPreviewAndApply/migrate_status_renders_a_distribution"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyStatusReportDocNeverMarshalsNull"
        status: pass
    human_judgment: false
  - id: D3
    description: "engram migrate revert previews via the exported Store.PreviewRevert (M8), refuses any irreversible or unsupported-version range WHOLE before any write (naming every step/version + snapshot recovery via 04-02's exported store.RevertRefusalError), and applies only a fully-reversible range"
    requirement: "REQ-migrate-revert"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyRevertRefusals"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyRevertReversible"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyRevertToValidation"
        status: pass
    human_judgment: false
  - id: D4
    description: "A refused `migrate revert --apply` exits exitUsage (2) for both refusal classes while its bare preview exits 0 (C5-H4); the refusal text is the EXACT store.RevertRefusalError(plan).Error() string on both the text and json surfaces (C5-M4)"
    requirement: "REQ-migrate-revert"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyRevertRefusals"
        status: pass
    human_judgment: false
  - id: D5
    description: "registerDestructive's admission gate is !class.ReadOnly (D-16); the --apply-required set is the NAMED union mutatingCommandNames() = destructiveCommandNames() ∪ applyRoutedAdditions − pendingApplyConversion, never !ReadOnly (C4-H1/M12); the rule sentence reads 'a mutating operator command …' across the registry, its pin, both anchored doc regions, and both goldens (N5)"
    verification:
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestDestructiveCommandsRequireApply"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestMutatingCommandNamesMembership"
        status: pass
      - kind: unit
        ref: "cmd/engram/destructive_test.go#TestApplyRoutedAdditionsArePinned"
        status: pass
      - kind: unit
        ref: "internal/surfaces/rules_test.go#TestRuleByIDDestructiveRequiresApply"
        status: pass
    human_judgment: false
  - id: D6
    description: "Every migrate-family mutating command (migrate, migrate revert) plus the read-only migrate status carries a finite 5-minute default deadline that its own --timeout flag actually changes, with 0 disabling (H8/N3/C5-M6), and each is assigned to the published zero-disables timeoutGroups row"
    verification:
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyTimeoutWiring"
        status: pass
      - kind: unit
        ref: "cmd/engram/migrate_family_test.go#TestMigrateFamilyRevertTimeoutWiring"
        status: pass
      - kind: unit
        ref: "cmd/engram/operator_output_test.go#TestTimeoutGroupMatrix"
        status: pass
    human_judgment: false
  - id: D7
    description: "The real-binary smoke check (go run ./cmd/engram migrate / migrate revert --to 0 / migrate --help against a live test Qdrant) named in this plan's <verification> section"
    verification: []
    human_judgment: true
    rationale: "No live Qdrant instance is available in this execution environment (worktree sandbox, no Docker/Qdrant service running). All behavior this smoke check would exercise is proven at the CLI boundary via injected fakes (migrateFamilyStoreFromEnv seam) in migrate_family_test.go; a human with a live Qdrant should run the three commands named in the plan's verification section before shipping."

# Metrics
duration: 19min
completed: 2026-08-15
status: complete
---

# Phase 4 Plan 03: Migration CLI First Customer — engram migrate Command Family Summary

**Built `engram migrate` / `engram migrate status` / `engram migrate revert --to <v>` — the operator-facing CLI for the phase's forward sweep, histogram, and reverse-inverse walk — routed through a generalized `registerDestructive` admission gate (`!ReadOnly`, D-16) with a NAMED `--apply`-required union (`mutatingCommandNames()`) that deliberately does not derive from that same predicate (C4-H1).**

## Performance

- **Duration:** ~19 min (first commit 10:12:17-04:00, last commit 10:31:00-04:00)
- **Started:** 2026-08-15T14:12:17Z
- **Completed:** 2026-08-15T14:31:00Z
- **Tasks:** 3
- **Files modified:** 14 (2 created)

## Accomplishments

- `cmd/engram/destructive.go`: `destructiveByClassification`'s predicate widened from `class.Destructive` to `!class.ReadOnly` — the ADMISSION question ("may this command route through the preview/apply choke point?") — with the M9 name-debt documented and the rejected-injectable-seam block left untouched.
- `cmd/engram/destructive_test.go`: `applyRoutedAdditions` (the named additive-routed set: `migrate`, `backfill-short-ids`), `pendingApplyConversion` (the one-wave `backfill-short-ids` exclusion 04-04 deletes), and `mutatingCommandNames()` (the NAMED `--apply`-required union, C4-H1/M12) — `TestDestructiveCommandsRequireApply` switched onto it and green in both directions at five names by the end of the wave; `TestApplyRoutedAdditionsArePinned` and `TestMutatingCommandNamesMembership` pin the derivation.
- `cmd/engram/migrate_family.go` (new): `migrateFamilyStore` interface + injectable `migrateFamilyStoreFromEnv` (M7); `migrateWithTimeout(ctx, d)` shared helper (H8/N3); `migrateSweepPreviewRun`/`migrateSweepApplyRun` (the single sweep implementation 04-04 reuses); `migrateCmd`/`migrateStatusCmd`/`migrateRevertCmd` with leaf-only `Use` strings (M6); the revert family's whole-range preflight-then-write contract, refusal envelope (`store.RevertRefusalError`, C5-M4), and non-zero exit on a refused `--apply` (C5-H4).
- `internal/surfaces/toolclass.go`: three new rows — `migrate` (ReadOnly:false, Destructive:false), `migrate status` (ReadOnly:true, Destructive:false), `migrate revert` (ReadOnly:false, Destructive:true).
- `internal/surfaces/rules.go`/`rules_test.go`: `RuleDestructiveRequiresApply`'s Sentence updated unconditionally to "a mutating operator command …" (N5), propagated via `task surfaces:gen` into both anchored doc regions and both goldens across all three tasks.
- Every command-keyed registry (`wantOperatorCommandKeys`, `operatorParityRows`, `operatorInvalidOutputArgs`, `timeoutGroups`, `operatorCommandFiles`) updated in the SAME task as the command it measures (INV-1), each proven via an unfiltered `go test ./cmd/engram/ -count=1`; `operatorCommandFiles`'s new `migrate_family.go` entry was proven non-vacuous with a RED-first experiment (a temporary bare `fmt.Errorf` made `TestNoBareOperatorErrorReturns` fail naming the file, then was reverted).

## Task Commits

Each task was committed atomically:

1. **Task 1: Generalize registerDestructive's admission predicate to !ReadOnly + declare mutatingCommandNames() + pin the rule-sentence update** — `d207656d` (feat)
2. **Task 2: engram migrate + engram migrate status** — `e23e2968` (feat)
3. **Task 3: engram migrate revert --to <v>** — `2002ca24` (feat)

## Files Created/Modified

- `cmd/engram/migrate_family.go` - the three migrate-family commands, shared sweep run funcs, timeout helper, store seam, report envelopes
- `cmd/engram/migrate_family_test.go` - the full `TestMigrateFamily*` test suite (preview/apply, intersection, timeout wiring x2, report fields, status-doc-never-null, revert refusals x2 classes, revert reversible, revert --to validation, revert timeout wiring)
- `cmd/engram/destructive.go` - `!ReadOnly` admission predicate, updated panic messages, M9 doc
- `cmd/engram/destructive_test.go` - `applyRoutedAdditions`/`pendingApplyConversion`/`mutatingCommandNames()`, `firstAdditiveRoutedTopLevelCommandName`, `TestApplyRoutedAdditionsArePinned`, `TestMutatingCommandNamesMembership`, `TestDestructiveCommandsRequireApply` switch, `TestDestructiveGatePreventsMutation`'s additive sub-test, `destructiveFlagCases`' `migrateRevertCmd` row
- `cmd/engram/cmdwalk_test.go` - `wantOperatorCommandKeys` gains `migrate`/`migrate status`/`migrate revert`
- `cmd/engram/operator_output_test.go` - `timeoutGroups`/`timeoutGroupCaseArgs`/`operatorParityRows`/`operatorInvalidOutputArgs` entries for all three commands
- `cmd/engram/operror_test.go` - `operatorCommandFiles` gains `migrate_family.go`; stale file-count doc comment replaced
- `internal/surfaces/toolclass.go` - three new blast-radius rows
- `internal/surfaces/rules.go`/`rules_test.go` - Sentence update + pin
- `docs-site/src/content/docs/guides/cli.md` - the unanchored destructive-command paragraph rewritten to the mutating scope (D9), the three-group `--timeout` table extended
- `skill/engram/skills/curating-memory/SKILL.md`, `cmd/engram/testdata/{help,catalog}.golden` - regenerated via `task surfaces:gen` at the end of each task that changed the tree

## Decisions Made

- `migrateReportDoc`/`statusReportDoc`/`revertReportDoc` are FUNCTIONS returning distinctly-named types (`migrateOutputDoc`, `store.MigrateStatusResult` reused directly, `revertOutputDoc`) rather than type names matching the plan's suggested struct literal — this follows the dominant 3-of-4 naming precedent already in the package and avoids a Go identifier collision the plan's shorthand phrasing would have produced.
- `statusReportDoc` reuses `store.MigrateStatusResult` directly as the CLI JSON shape (its json tags already match) instead of declaring a parallel CLI-side type; its only job is the C5-L8 nil-to-empty-slice normalization.
- `migrate revert`'s toolclass row is `Idempotent: true` — a re-run against an already-reverted (empty above-target) range trivially preflights `Reversible` (an empty observed-chains set) and the write loop's first `Count` is zero, so it touches nothing further.

## Deviations from Plan

### Auto-fixed Issues

None — no Rule 1-3 auto-fixes were required. Every acceptance criterion and every `<verify>` gate in all three tasks passed exactly as the plan specified, including every INV-1/INV-2/INV-3 negative/positive gate and the RED-first non-vacuity experiment on `TestNoBareOperatorErrorReturns`.

### Scope simplifications (documented, not corrective)

None beyond the two naming decisions recorded above (Decisions Made), which follow existing codebase precedent rather than weakening any proven guarantee.

---

**Total deviations:** 0.
**Impact on plan:** All three tasks' stated `<verify>` commands pass, all four REQ-* requirements are met, and the full unfiltered `task` (lint + test across all 17 non-empty packages) is green.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. One item is deferred to a human with a live Qdrant instance: the plan's `<verification>` real-binary smoke check (`go run ./cmd/engram migrate` / `migrate revert --to 0` / `migrate --help` against a test Qdrant) could not be run in this sandboxed execution environment (no Qdrant service available). All behavior it would exercise is proven at the CLI boundary via `migrateFamilyStoreFromEnv`'s injected-fake seam in `migrate_family_test.go` — see coverage item D7.

## Next Phase Readiness

- `migrateWithTimeout`, `migrateSweepPreviewRun`/`migrateSweepApplyRun`, `migrateReportDoc` + `migrateSummary`, and `migrateFamilyStoreFromEnv` are all in place for 04-04's `backfill-short-ids` alias to delegate to directly (cycle-3 #7 — the alias's preview/apply parity with `migrate` is a structural fact, not a claim).
- `mutatingCommandNames()` and `pendingApplyConversion` are in place for 04-04 Task 1 to widen the sibling gates (`TestDestructiveCommandsRouteThroughGate`, `TestDestructiveCommandsExactFlagSet`, `TestApplyFlagUsageComposesRuleSentence`) onto the mutating derivation and delete `pendingApplyConversion` in the same task that gives `backfill-short-ids` its `--apply` flag.
- `TestMutatingCommandNamesMembership`'s doc comment already states the wave-4 target: six names, adding `backfill-short-ids`.
- No blockers for 04-04.

## Known Stubs

None — no stub or placeholder data was introduced by this plan.

## Threat Flags

None — every new command's blast radius (write/read paths, refusal envelope) is exactly what this plan's own `<threat_model>` section registered and mitigated; no new trust boundary was introduced.

## Self-Check: PASSED

- FOUND: cmd/engram/migrate_family.go
- FOUND: cmd/engram/migrate_family_test.go
- FOUND: .planning/phases/04-migration-cli-first-customer/04-03-SUMMARY.md
- FOUND: d207656d, e23e2968, 2002ca24

---
*Phase: 04-migration-cli-first-customer*
*Completed: 2026-08-15*
