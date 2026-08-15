---
phase: 04-migration-cli-first-customer
plan: 01
subsystem: database
tags: [migration, schema-versioning, qdrant, go]

# Dependency graph
requires:
  - phase: 03-migration-foundation-registry-invariants-sweep
    provides: "migrate.Registry, migrate.NewStep, Store.Migrate's re-derivation sweep, CheckAdditive, backlogFilter, PA-3 termination guard"
provides:
  - "migrate.NewMintingStep / ApplyMinterFunc / Step.ApplyMinter / Step.ApplyMinterMint — the minter-aware step apply path"
  - "The registered v0->v1 backfill-short-ids step in migrate.Registry, migrate.CurrentVersion raised to 1 in the same change"
  - "CheckAdditive's pre-existing-declared-key carve-out (REVIEWS.md H1)"
  - "Store.MigrateOptions.{DryRun,Manifest} and Store.MigrateResult.{PreviewManifest,Spared,Appeared}"
  - "Store.Migrate's minter branch, full-backlog single-pass DryRun preview, and manifest-bridged single-pass apply"
affects: [04-02, 04-03, 04-04]

# Actuals (#2632)
actuals:
  tokens: 20100
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Optional-capability step apply path: a second constructor (NewMintingStep) alongside NewStep, discriminated at the point of use via Step.ApplyMinter(), never a widened signature for every step"
    - "Manifest-bridged preview/apply intersection with Spared/Appeared as []string identity sets, mirroring the shipped PurgeResult shape"

key-files:
  created:
    - internal/migrate/v1_step.go
  modified:
    - internal/migrate/step.go
    - internal/migrate/registry.go
    - internal/migrate/migrate.go
    - internal/migrate/additive.go
    - internal/migrate/migrate_test.go
    - internal/migrate/additive_test.go
    - internal/migrate/registry_test.go
    - internal/store/migrate.go
    - internal/store/migrate_test.go
    - internal/store/migrate_converge_test.go
    - internal/store/store_test.go

key-decisions:
  - "Task 2's test repairs (E1/E2) required Store.Migrate's minter branch to already exist against the real migrate.Registry — implemented Task 3's migrate.go changes before verifying Task 2's tests, then committed in plan order (test-only files first, migrate.go second) so the commit history still matches each task's stated files_modified"
  - "DryRun and Manifest are mutually exclusive, rejected with ErrInvalidArgument before any I/O, following the store tier's existing mutually-exclusive-options idiom (list cursor/offset, PreviewPurge AllScopes/Scope)"
  - "Spared is computed as a post-scroll set difference, never inside the point loop — verified this by temporarily reverting to a loop-classified variant and observing TestMigrateManifestIntersection go RED, then reverting the experiment"

patterns-established:
  - "A closure (not a separate method) factors shared per-record chain-application logic inside Store.Migrate, keeping every SetPayload call site lexically inside the single func the AST-derived partial-write classification gate is keyed on"

requirements-completed:
  - REQ-backfill-shortids-first-step
  - REQ-migrate-command
  - REQ-migrate-preview-apply-parity

coverage:
  - id: D1
    description: "v0->v1 minter-aware step (NewMintingStep/ApplyMinterFunc/Step.ApplyMinter) registered in migrate.Registry with CurrentVersion raised to 1 in the same change"
    requirement: "REQ-backfill-shortids-first-step"
    verification:
      - kind: unit
        ref: "internal/migrate/migrate_test.go#TestNewMintingStep"
        status: pass
      - kind: unit
        ref: "internal/migrate/migrate_test.go#TestV1FillShortID"
        status: pass
      - kind: unit
        ref: "internal/migrate/migrate_test.go#TestCurrentVersionValue"
        status: pass
    human_judgment: false
  - id: D2
    description: "CheckAdditive pre-existing-declared-key carve-out (H1) — a step declaring a key already present in before is satisfied without re-adding it, while a genuinely never-added key still fails"
    requirement: "REQ-backfill-shortids-first-step"
    verification:
      - kind: unit
        ref: "internal/migrate/migrate_test.go#TestCheckAdditivePreExistingKey"
        status: pass
    human_judgment: false
  - id: D3
    description: "Bare legacy record migrates end-to-end through the real registered step: short_id minted, schema_version stamped, second run no-op"
    requirement: "REQ-backfill-shortids-first-step"
    verification:
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateV0ToV1MintEndToEnd"
        status: pass
    human_judgment: false
  - id: D4
    description: "A record with a pre-existing short_id (no schema_version) is preserved verbatim, never re-minted, and stamped — the mixed-state production shape 04-04's BackfillShortIDs deletion depends on"
    requirement: "REQ-backfill-shortids-first-step"
    verification:
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateExistingShortIDPreserves"
        status: pass
    human_judgment: false
  - id: D5
    description: "Store.Migrate DryRun previews the FULL backlog (not one scroll batch) and writes nothing"
    requirement: "REQ-migrate-command"
    verification:
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateDryRunWritesNothing"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateFullBacklogProjection"
        status: pass
      - kind: unit
        ref: "internal/store/migrate_test.go#TestMigrateDryRunAndManifestMutuallyExclusive"
        status: pass
    human_judgment: false
  - id: D6
    description: "Manifest-bridged apply migrates exactly manifest ∩ fresh re-derivation, reporting Spared/Appeared as identity sets, with Backlog truthfully including Appeared records and no PA-3 firing"
    requirement: "REQ-migrate-preview-apply-parity"
    verification:
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateManifestIntersection"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateManifestSparedDeletedRecord"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateManifestBacklogAppeared"
        status: pass
    human_judgment: false
  - id: D7
    description: "The CurrentVersion 0->1 bump's wave-1 blast radius is repaired: the two shipped internal/store tests it reds are fixed, and PA-10a item 3's BLOCKING obligation is discharged"
    verification:
      - kind: integration
        ref: "internal/store/migrate_test.go#TestBacklogFilterMatchesAbsentAndBelowTarget"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_converge_test.go#TestMigrateConvergesWithoutLock"
        status: pass
      - kind: integration
        ref: "internal/store/schemaversion_compat_test.go#TestSchemaVersionForwardBackwardCompat"
        status: pass
    human_judgment: false

# Metrics
duration: 18min
completed: 2026-08-15
status: complete
---

# Phase 4 Plan 01: Migration First Customer — v0->v1 Minting Step + Store.Migrate DryRun/Manifest Summary

**Registered the v0->v1 backfill-short-ids step as `migrate.Registry`'s first customer, raised `migrate.CurrentVersion` to 1, and gave `Store.Migrate` a minter-aware apply branch plus a full-backlog DryRun preview and a manifest-bridged single-pass apply proving preview/apply parity by identity set.**

## Performance

- **Duration:** ~18 min (first commit 09:14:17-04:00, last commit 09:31:47-04:00)
- **Started:** 2026-08-15T13:14:17Z
- **Completed:** 2026-08-15T13:31:47Z
- **Tasks:** 3 (plus one lint-fix commit)
- **Files modified:** 11 (1 created)

## Accomplishments

- `internal/migrate`: `ApplyMinterFunc`, `NewMintingStep`, `Step.ApplyMinter`/`ApplyMinterMint`, the CheckAdditive pre-existing-key carve-out (H1), `v1FillShortID`, the registered v0->v1 step, and `CurrentVersion` raised 0→1 in the same change with every falsified doc-comment claim rewritten.
- `internal/store`: `Store.Migrate` now branches on a minter-aware step via a lazily-built per-call mint closure; `MigrateOptions.{DryRun,Manifest}` and `MigrateResult.{PreviewManifest,Spared,Appeared}` implement a full-backlog single-pass DryRun preview and a manifest-bridged single-pass apply, with `Spared` derived as a post-scroll set difference (never a loop classification) and `Backlog` truthfully including `Appeared` records.
- Repaired the two shipped `internal/store` tests the `CurrentVersion` bump reds (`TestBacklogFilterMatchesAbsentAndBelowTarget`, `TestMigrateConvergesWithoutLock`) and discharged PA-10a item 3's BLOCKING obligation — the mid-sweep already-current record is now an ordinary `Upsert` with no `SchemaVersion`, proven against a target resolved from `migrate.CurrentVersion` alone.

## Task Commits

Each task was committed atomically. Task 2's tests required Task 3's `Store.Migrate` minter branch to be present before they could pass (discovered during execution — see Deviations); commits landed in plan order regardless, each touching exactly its task's stated `files_modified`.

1. **Task 1: v0→v1 minter step, CheckAdditive carve-out, register + bump** — `8fb9d6d9` (feat)
2. **(lint fix on Task 1's test file, surfaced by `task lint` during Task 3)** — `dd3723a8` (fix)
3. **Task 2: VERSION-bump blast radius repair, PA-10a item 3 discharge** — `0fa76d62` (test)
4. **Task 3: Store.Migrate minter branch + DryRun preview + manifest apply** — `96711281` (feat)

## Files Created/Modified

- `internal/migrate/v1_step.go` - `v1FillShortID`: preserves an existing short_id verbatim, mints when absent
- `internal/migrate/step.go` - `ApplyMinterFunc`, `NewMintingStep`, `Step.ApplyMinter`/`ApplyMinterMint`, `applyMinter` field
- `internal/migrate/registry.go` - registers the v0→v1 step; rewrote the "ships EMPTY" doc comment
- `internal/migrate/migrate.go` - `CurrentVersion` raised 0→1; rewrote the three-reason doc block (re-tiered reason 3 to name the server-layer mint sites)
- `internal/migrate/additive.go` - `CheckAdditive`'s pre-existing-declared-key carve-out (H1)
- `internal/migrate/migrate_test.go`, `additive_test.go`, `registry_test.go` - new/updated tests for the above (M13, C1-C5 conformance rows)
- `internal/store/migrate.go` - minter branch, DryRun preview, manifest-limited apply, expanded `MigrateOptions`/`MigrateResult`
- `internal/store/migrate_test.go` - E1 repair plus 8 new Task-3 end-to-end tests
- `internal/store/migrate_converge_test.go` - E2 repair, `seedLegacyRecordNoFatal`, PA-10/PA-10a prose rewrite
- `internal/store/store_test.go` - E4 prose repair (the `CurrentVersion is 0` sentinel claim)

## Decisions Made

- Task 2's `TestBacklogFilterMatchesAbsentAndBelowTarget` default-target sub-case and Task 2's own `<verify>` gate (`go test ./internal/store/ -count=1` unfiltered) turned out to depend on Task 3's `Store.Migrate` minter branch: calling `s.Migrate(ctx, MigrateOptions{})` against the now-registered `migrate.Registry` panics on a nil `Step.apply` unless the sweep already checks `Step.ApplyMinter()` first. Implemented Task 3's code before verifying Task 2's tests, then committed in the plan's stated task order so history still matches each task's declared `files_modified` — Task 2's commit is test-only, Task 3's is `migrate.go` plus its own tests.
- `MigrateOptions{DryRun: true, Manifest: non-nil}` is rejected with `ErrInvalidArgument` before any I/O (checked by dialing a store with no `EnsureCollection`'d collection — proving no Count/Scroll was attempted, since either would fail with a transport "not found" error, not the validation sentinel).
- `Spared` is computed once, after the manifest-limited apply's scroll is exhausted — never inside the point loop. Verified this claim experimentally: temporarily reverted the computation to a loop-classified variant, observed `TestMigrateManifestIntersection` go RED (`Spared = []`, cardinality mismatch), then reverted the experiment and re-confirmed green.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Task 2's test edits required Task 3's Store.Migrate code to already exist**
- **Found during:** Task 2, verifying `TestBacklogFilterMatchesAbsentAndBelowTarget`'s new default-target assertion
- **Issue:** `MigrateOptions{}` resolves `Steps` to `migrate.Registry`, which (after Task 1) contains a `NewMintingStep`-built step with `apply == nil`. `Store.Migrate`'s existing per-step loop called `step.Apply(...)` unconditionally, so this call panicked with a nil-pointer dereference — confirmed by running the test before any Task 3 code existed.
- **Fix:** Implemented Task 3's full scope (minter branch, DryRun, Manifest) before finalizing verification of Task 2's tests, then committed strictly in plan order so each commit's diff still matches its task's declared `files_modified`.
- **Files modified:** internal/store/migrate.go (Task 3's own file, not Task 2's)
- **Verification:** `go test ./internal/store/ -count=1` green unfiltered after both tasks landed; each task's own `<verify>` command re-run individually post-hoc and confirmed passing against the final tree.
- **Committed in:** `96711281` (Task 3 commit, which is where the dependency actually resolves)

**2. [Rule 1 - Bug] Unused `mint` parameter flagged by `task lint` in three test fixtures**
- **Found during:** running `task lint` after Task 3 (the gate covers the whole wave, per this repo's lint-at-wave-boundary convention)
- **Issue:** Three test-only `ApplyMinterFunc` fixtures in `internal/migrate/migrate_test.go` (added in Task 1) declared a `mint` parameter they never used, tripping golangci-lint's `revive` unused-parameter check.
- **Fix:** Renamed to `_` per the linter's own suggestion.
- **Files modified:** internal/migrate/migrate_test.go
- **Verification:** `task lint` clean afterward.
- **Committed in:** `dd3723a8`

---

**Total deviations:** 2 auto-fixed (1 blocking dependency resolved via commit reordering, 1 lint bug)
**Impact on plan:** No scope creep — both fixes were required to reach the plan's own stated gates. The Task 2/Task 3 interdependency is a real property of the code (a registered minting step needs sweep support before any default-options Migrate call is safe), not a planning error to relitigate; documented here so a future reader is not surprised that Task 2's commit alone does not build a fully passing `internal/store` in isolation.

## Issues Encountered

None beyond the deviation above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `store.MigrateOptions.DryRun`/`.Manifest`, `store.MigrateResult.PreviewManifest`/`.Spared`/`.Appeared`, and `migrate.CurrentVersion == 1` are all in place for 04-02 (revert path, startup warning) and 04-03 (the `engram migrate` CLI family, which wires `migrateSweepPreviewRun`/`migrateSweepApplyRun` directly onto this plan's store API) to consume.
- `internal/migrate`'s `TestMigratePackageIsStdlibOnlyLeaf` and `TestReversibilityIsSealedToThisPackage` conformance gates are unaffected — `v1_step.go` imports stdlib only, and no exported carrier of `isReversibility` was introduced.
- `internal/keylinks`'s `TestActiveMilestoneKeyLinksSatisfiable` confirmed phase 3's two key links anchored in `internal/store/migrate.go` (`migrate.StepsFrom`, `migrate.CheckAdditive`) remain satisfiable against the rewritten file.
- No blockers for 04-02/04-03/04-04.

## Known Stubs

None — no stub or placeholder data was introduced by this plan.

---
*Phase: 04-migration-cli-first-customer*
*Completed: 2026-08-15*
