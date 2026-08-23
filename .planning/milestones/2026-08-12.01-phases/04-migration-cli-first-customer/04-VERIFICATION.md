---
phase: 04-migration-cli-first-customer
verified: 2026-08-15T15:45:00Z
status: passed
score: 6/6 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 4: Migration CLI & First Customer Verification Report

**Phase Goal:** An operator can preview, apply, and revert schema migrations through the
standard destructive-tier CLI, with `backfill-short-ids` folded in as the registry's first
real step — never running automatically.

**Verified:** 2026-08-15T15:45:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `engram migrate` is registered via `registerDestructive`; bare invocation previews only, `--apply` is the explicit choke point, `--output json\|text` matches operator tier | ✓ VERIFIED | `cmd/engram/migrate_family.go:511-532` (`registerDestructive(migrateCmd, ...)`); orchestrator's live run: bare invocation returned `{"dry_run":true,"would_migrate":2,"migrated":0}`, follow-up `status` byte-identical (no writes); `--help` shows `--apply`/`--output`/`--timeout` |
| 2 | `engram migrate status` reports a version-distribution histogram, not a scalar; mixed-version collection correctly represented | ✓ VERIFIED | `internal/store/migrate_status.go` (`MigrateStatusResult{Buckets,Absent,Future,FutureTotal,Total}`, facet+absent-Count, truncation/race detector, D-08); orchestrator live run on 2 legacy records: `{"buckets":[],"absent":2,...}` → post-apply `{"buckets":[{"version":1,"count":2}],"absent":0,...}` |
| 3 | `--apply` acts only on the intersection of previewed set and fresh re-derivation (spine-review purge pattern); mismatch provable by test | ✓ VERIFIED | `migrateSweepApplyRun` (`cmd/engram/migrate_family.go:187-213`) — in-apply-closure re-preview then manifest-limited apply; `internal/store/migrate_test.go#TestMigrateManifestIntersection`, `TestMigrateManifestSparedDeletedRecord`, `TestMigrateManifestBacklogAppeared`; `cmd/engram/migrate_family_test.go#TestMigrateFamilyApplyIntersection`; orchestrator: exactly the 2 previewed records migrated, second `--apply` idempotent (`would_migrate:0`) |
| 4 | `backfill-short-ids` registered as v0→v1 step; standalone becomes thin delegating alias (soft deprecation); apply-by-default reconciled with preview-by-default via a tested `guides/upgrade.md` entry | ✓ VERIFIED | `internal/migrate/v1_step.go` + `registry.go` (registered step, `CurrentVersion`=1); `cmd/engram/backfill.go` (two one-line adapters over `migrateSweepPreviewRun`/`migrateSweepApplyRun`, `Deprecated = "use: engram migrate"`, no `--dry-run` flag — confirmed absent by grep and by `TestBackfillCmdFlagSet`); `docs-site/guides/upgrade.md` "## Unreleased #13"; `cmd/engram/backfill_test.go#TestUpgradeGuideReconcilesBackfill` (D-12 bidirectional doc↔code gate, RED-proven per SUMMARY); `Store.BackfillShortIDs`/`missingShortIDFilter` confirmed deleted (zero grep hits in `internal/`/`cmd/`); orchestrator: bare invocation deprecation-printed then previewed, pre-existing short_id preserved verbatim through apply (H1 carve-out) |
| 5 | `engram migrate revert` previews by default, runs declared inverses in reverse order, refuses the whole operation at any irreversible step in range (not partial), refusal names a snapshot as recovery | ✓ VERIFIED | `internal/store/revert.go` (`PreviewRevert`/`previewRevertWithSteps` — whole-range `scrollAllPoints` preflight before any write, D-13); `revertWithSteps` reverse-walks via `revertStepsFrom` (pinned `StepsFrom(steps, to, from)` reversed); `RevertRefusalError` names "recovery is a collection snapshot"; orchestrator: `migrate revert --to 0` returned `reversible:false` naming the irreversible step and snapshot recovery, `--apply` exited 2 with byte-unchanged payloads; orchestrator independently proved `PreviewRevert`'s whole-range preflight RED against 2 deliberate violators |
| 6 | No migration ever runs automatically on server startup; startup emits at most a non-blocking warning | ✓ VERIFIED | `internal/server/tools.go:210-211,486-537` — `warnPendingMigrations` runs inline in `buildDepsFromEnv`, is a `void` function (no error return, so it structurally cannot block/fail startup), 10s-bounded, uses H3-corrected pending predicate (`Absent + sum(buckets below CurrentVersion)`, excludes `Future`); zero `.Migrate(` call sites anywhere under `internal/server/` (grep confirmed); `internal/server/tools_test.go#TestWarnPendingMigrations` **re-run live against real pinned Qdrant during this verification** — 3/3 subtests pass (pending warns, already-current warns nothing, future-only warns only the compatibility message) |

**Score:** 6/6 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/migrate/v1_step.go` | v0→v1 minting step | ✓ VERIFIED | `v1FillShortID`, registered in `Registry` |
| `internal/migrate/step.go` | `NewMintingStep`/`ApplyMinterFunc` | ✓ VERIFIED | Optional-capability apply path, constructor-enforced single apply path (D-02) |
| `internal/store/migrate.go` | minter branch, DryRun/Manifest options | ✓ VERIFIED | `MigrateOptions{DryRun,Manifest}`, `MigrateResult{PreviewManifest,Spared,Appeared}` |
| `internal/store/migrate_status.go` | server-side histogram | ✓ VERIFIED | `MigrateStatus`, facet+absent-Count+future-bucket, truncation detector |
| `internal/store/revert.go` | `PreviewRevert`/`Revert`/`RevertRefusalError` | ✓ VERIFIED | Whole-range preflight, per-record reverse chain, single refusal constructor |
| `cmd/engram/migrate_family.go` | `migrate`/`migrate status`/`migrate revert` CLI | ✓ VERIFIED | 533 lines, subcommand tree, `registerDestructive` wiring, shared run funcs |
| `cmd/engram/backfill.go` | thin delegating alias | ✓ VERIFIED | 59 lines, two one-line adapters, `Deprecated` set, no `--dry-run` |
| `internal/server/tools.go` | `warnPendingMigrations` | ✓ VERIFIED | Wired into `buildDepsFromEnv`, non-blocking |
| `internal/surfaces/toolclass.go` | 3 new rows | ✓ VERIFIED | `migrate` (RO:false/D:false), `migrate status` (RO:true), `migrate revert` (RO:false/D:true) — matches D-05/D-07/D-16 exactly |
| `docs-site/guides/upgrade.md` | `--dry-run` removal entry | ✓ VERIFIED | "## Unreleased #13", gated by `TestUpgradeGuideReconcilesBackfill` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `migrateCmd`/`migrateRevertCmd` | `registerDestructive` | `init()` AddCommand-then-register | ✓ WIRED | `migrate_family.go:511-533`; `migrateStatusCmd` correctly NOT routed (read-only, D-05) |
| `backfillShortIDsCmd` | `migrateSweepPreviewRun`/`migrateSweepApplyRun` | one-line adapter closures | ✓ WIRED | `backfill.go:36-42`, structural call-sequence equality with `migrate --apply`, confirmed by `TestBackfillApplyPathParityWithMigrateApply` |
| `warnPendingMigrations` | `Store.MigrateStatus` | direct call | ✓ WIRED | `tools.go:504`; never calls `Store.Migrate` (grep-confirmed) |
| `revertApplyRun` | `Store.PreviewRevert` then `Store.Revert` | in-closure re-preflight | ✓ WIRED | `migrate_family.go:453-489`; refusal path never calls `Revert` |
| toolclass rows | `TestCatalogBlastRadiusMatchesToolClasses` | both-directions gate | ✓ WIRED | Per SUMMARY, gate exercised and green in `task` run |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Startup warning is non-blocking and correctly predicated (SC6, the orchestrator's un-exercised criterion) | `go test ./internal/server/ -run TestWarnPendingMigrations -v` against real pinned Qdrant | 3/3 subtests PASS (pending warns, already-current silent, future-only warns compatibility message only) | ✓ PASS |
| No automatic migration call exists anywhere in the server/startup path | `rg -n "\.Migrate\(" internal/server/ cmd/engram/root.go cmd/engram/serve.go` | 0 hits | ✓ PASS |
| No debt markers in phase-touched files | `rg -n "TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER"` across all key-files | 0 hits | ✓ PASS |
| Build/vet clean | `go build ./...`, `go vet ./...` | exit 0 both | ✓ PASS |
| `Store.BackfillShortIDs`/`missingShortIDFilter` genuinely deleted | `rg "BackfillShortIDs\(|missingShortIDFilter\("` across `internal/`, `cmd/` | 0 hits | ✓ PASS |
| No `--dry-run` flag survives on `backfill-short-ids` | `rg "dry-run" cmd/engram/backfill.go` | 0 hits (only present in test file asserting its absence) | ✓ PASS |

Full `task` (lint + test, 17 packages) already run by the orchestrator and confirmed green; not re-run in full here (single named test re-run above per spot-check discipline).

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|--------------|--------|----------|
| REQ-migrate-command | 04-01, 04-03 | `engram migrate` via `registerDestructive`, preview-default, `--apply` choke point, `--output` parity | ✓ SATISFIED | Truth #1, artifacts above |
| REQ-migrate-status-histogram | 04-02, 04-03 | version-distribution histogram, not scalar | ✓ SATISFIED | Truth #2 |
| REQ-migrate-preview-apply-parity | 04-01, 04-03 | `--apply` = previewed ∩ fresh re-derivation, test-provable | ✓ SATISFIED | Truth #3 |
| REQ-backfill-shortids-first-step | 04-01, 04-04 | v0→v1 first customer, thin alias, upgrade-guide gate | ✓ SATISFIED | Truth #4 |
| REQ-migrate-revert | 04-02, 04-03 | preview-default, reverse inverses, whole-range refusal, snapshot recovery | ✓ SATISFIED | Truth #5 |
| REQ-migrate-never-automatic | 04-02, 04-04 | never automatic at startup, non-blocking warning | ✓ SATISFIED | Truth #6 |

All six Phase 4 requirement rows in `.planning/REQUIREMENTS.md` are currently marked `Pending` (both the checkbox list at lines ~31-39 and the traceability table at lines ~102-107). Based on the evidence above, **all six should be marked complete** — each has passing store-layer tests against real pinned Qdrant, passing CLI-layer tests against injected fakes, and (for every criterion except the CLI's own real-binary smoke check, itself independently covered by the orchestrator's live run) direct runtime confirmation. No orphaned requirements were found for this phase.

### Anti-Patterns Found

None. No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers in any phase-touched file. No stub returns, no hardcoded empty data flowing to output.

### Known Open Findings (Not Blockers)

Two WARNING-level findings from `04-REVIEW.md`, both judged **non-blocking to the phase goal**:

- **WR-01** (`migrate_family.go:187-213`): `--apply`'s single `--timeout` budget covers two full-backlog `Store.Migrate` passes (mint-heavy preview + apply), undocumented in `migrateCmd.Long`/`cli.md`. This is a robustness/documentation gap on a *large-collection* edge case, not a failure of any of the six success criteria — none of them specify a timeout-budget contract. Confirmed still present by reading `migrate_family.go:187-213` during this verification; not fixed, correctly left as follow-up.
- **WR-02** (`internal/store/revert.go:160-181`): `RevertRefusalError` can concatenate TWO `field=`/`hint=` envelopes into one error string when a revert range contains both an irreversible-chain record and an unsupported-version record. SC5 requires the refusal to name a snapshot as the recovery path (it does, in both clauses) and to refuse the whole range (it does) — the multi-envelope format is a machine-parseability nuance on top of an already-correct refusal, not a violation of SC5's stated contract. Confirmed still present by reading `revert.go:160-181` during this verification.
- **IN-01**: purely a documentation-completeness note tied to WR-01.

Neither finding blocks the phase goal ("preview, apply, and revert... through the standard destructive-tier CLI... never running automatically") — the CLI genuinely does preview/apply/revert correctly in every tested case; these are edge-case robustness gaps the review itself classified as non-blocking (0 criticals).

### Locked Decisions (04-CONTEXT.md) Cross-Check

Spot-checked the decisions most likely to have been quietly diverged from during implementation:

- D-05 (subcommands, not flags-on-one-command): confirmed — `migrate`/`migrate status`/`migrate revert` are three distinct cobra commands with disjoint flag sets.
- D-07 (toolclass rows): confirmed exact match — `migrate` Destructive:false, `migrate status` ReadOnly:true, `migrate revert` Destructive:true.
- D-09/D-12 (backfill `--dry-run` removed outright, gated doc↔code test): confirmed — flag absent, test exists and is described as RED-proven per SUMMARY.
- D-13/D-14 (whole-range preflight before any write, refusal names every irreversible step + snapshot recovery): confirmed in `revert.go` and via orchestrator's live run.
- D-15 (`Store.Revert` is a dedicated method, not a direction flag on `Store.Migrate`): confirmed — separate `revertWithSteps` in `revert.go`, no direction parameter added to `Store.Migrate`.
- REQ-migrate-never-automatic's "non-blocking" clarification (comment in `tools.go:486-499` — means "never gates startup," not "asynchronous," and explicitly rejects a goroutine approach with reasoning): implementation matches this exactly — synchronous, 10s-bounded, void return.

No divergence found in any spot-checked decision.

### Human Verification Required

None. All six success criteria have either direct runtime evidence (from the orchestrator's live smoke test, or from this verification's fresh re-run of `TestWarnPendingMigrations`) or code-level proof sufficient to consider them fact, not claim.

### Gaps Summary

No gaps. All six phase success criteria are observably true in the codebase, all six requirement IDs are genuinely satisfied and should be marked complete in `REQUIREMENTS.md`, and the two open review findings (WR-01, WR-02) are correctly classified as non-blocking follow-up work rather than phase-goal failures.

---

*Verified: 2026-08-15T15:45:00Z*
*Verifier: Claude (gsd-verifier)*
