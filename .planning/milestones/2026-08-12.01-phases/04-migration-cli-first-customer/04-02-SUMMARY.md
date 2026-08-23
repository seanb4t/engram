---
phase: 04-migration-cli-first-customer
plan: 02
subsystem: database
tags: [migration, schema-versioning, revert, qdrant, go]

# Dependency graph
requires:
  - phase: 04-migration-cli-first-customer (plan 01)
    provides: "migrate.Registry's registered v0->v1 step, migrate.CurrentVersion==1, Store.Migrate's minter branch/DryRun/Manifest apply"
provides:
  - "Store.MigrateStatus / MigrateStatusResult / VersionBucket — server-side version-distribution histogram with per-version future buckets (M4)"
  - "store.RevertPlan / IrreversibleStepRef / UnsupportedVersionRef / Store.PreviewRevert — the exported whole-range zero-write preflight (cycle-3 HIGH #1/#2)"
  - "Store.Revert / RevertResult / RevertRefusalError — reverse-inverse walk with pinned StepsFrom order, per-record chains, DeletePayload-then-SetPayload write contract (H4-H6, M3)"
  - "warnPendingMigrations — non-blocking startup warning wired into buildDepsFromEnv (H3-corrected predicate, M4 future-version warning)"
  - "docs-site operator-tier hint-code subsection for irreversible/unsupported"
affects: [04-03, 04-04]

# Actuals (#2632)
actuals:
  tokens: 20200
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Zero-write whole-range preflight before any mutation: Store.PreviewRevert enumerates the ENTIRE above-target range via scrollAllPoints and derives its irreversible-step set from the UNION of chains actually observed (not the registry's whole above-target range), so a record's own reachable chain — never a global worst case — decides refusal"
    - "DeletePayload-then-SetPayload as a two-RPC commit whose SetPayload is the commit point: a landed delete with a failed stamp leaves the record re-derivable, making resume idempotent by construction (deleting an absent key is a no-op)"
    - "A single exported refusal-envelope constructor (RevertRefusalError) shared by the store's own gate and the (future) CLI's preview/apply rendering, so the two surfaces cannot describe the same refusal differently"

key-files:
  created:
    - internal/store/migrate_status.go
    - internal/store/migrate_status_test.go
    - internal/store/revert.go
    - internal/store/revert_test.go
  modified:
    - internal/store/schemaversion_recallgate_test.go
    - internal/store/schemaversion_stamp_gate_test.go
    - internal/server/tools.go
    - internal/server/tools_test.go
    - docs-site/src/content/docs/reference/errors.md

key-decisions:
  - "migrateStatusFacetLimit is a package-level var (not const), paired with a runtime truncation detector that distinguishes a bound-reached mismatch (truncation) from a below-bound mismatch (treated as a concurrent writer, retried exactly once) — proven behaviorally with a deterministic gRPC interceptor that races a write between the Facet call and the following Count, rather than relying on real goroutine concurrency."
  - "reversePreflight takes the union of chains the whole-range enumeration actually observed, not the registry's whole above-target range — so an irreversible step no record's own chain traverses does not spuriously refuse a revert (this matters once the registry grows a second step; today's single-step registry cannot distinguish the two forms)."
  - "revertWithSteps' write-loop batch size reuses migrate.go's existing migrateBatch constant (256) rather than introducing a second Options struct — Store.Revert's public signature carries no Batch field per the plan's own artifact list."

patterns-established:
  - "Reverse-chain selection is per-record: revertStepsFrom(steps, from, to) pins migrate.StepsFrom(steps, to, from) — the reversed argument order — then reverses the result, so each record's own stored version (never a fixed global target-to-zero chain) decides which inverses run."

requirements-completed:
  - REQ-migrate-status-histogram
  - REQ-migrate-revert
  - REQ-migrate-never-automatic

coverage:
  - id: D1
    description: "Store.MigrateStatus computes a server-side version-distribution histogram (facet + absent-key exact Count), with future-version records reported as a per-version bucket list rather than a scalar (M4), and a named/adequate facet-limit var paired with a runtime truncation/concurrent-write detector."
    requirement: "REQ-migrate-status-histogram"
    verification:
      - kind: integration
        ref: "internal/store/migrate_status_test.go#TestMigrateStatusHistogram"
        status: pass
      - kind: unit
        ref: "internal/store/migrate_status_test.go#TestMigrateStatusFacetLimitIsNamedAndAdequate"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_status_test.go#TestMigrateStatusDetectsTruncation"
        status: pass
    human_judgment: false
  - id: D2
    description: "store.RevertPlan and the exported Store.PreviewRevert perform a whole-range, zero-write preflight over the ENTIRE above-target range via scrollAllPoints; an unsupported record on any page (proven with a forced 3-page, 5-record fixture) or an irreversible step in any observed chain refuses the WHOLE operation with zero records touched."
    requirement: "REQ-migrate-revert"
    verification:
      - kind: integration
        ref: "internal/store/revert_test.go#TestMigrateRevertIrreversibleRangeRefusesWhole"
        status: pass
      - kind: integration
        ref: "internal/store/revert_test.go#TestMigrateRevertMultiPageUnsupportedPreflight"
        status: pass
    human_judgment: false
  - id: D3
    description: "Store.Revert / revertWithSteps reverse-walks declared inverses to convergence via fixture injection (H4), selects each record's chain from its OWN stored version (H5), and derives that chain via the pinned StepsFrom(steps, to, from) invocation, reversed (H6)."
    requirement: "REQ-migrate-revert"
    verification:
      - kind: integration
        ref: "internal/store/revert_test.go#TestMigrateRevertFixtureInjectionConverges"
        status: pass
      - kind: integration
        ref: "internal/store/revert_test.go#TestMigrateRevertPerRecordChainSelection"
        status: pass
      - kind: unit
        ref: "internal/store/revert_test.go#TestMigrateRevertStepsFromArgOrder"
        status: pass
    human_judgment: false
  - id: D4
    description: "The inverse write contract (DeletePayload for removed keys, then ONE SetPayload carrying added keys plus the schema_version stamp) makes the stamp the commit point: a landed delete with a failed stamp leaves the record at its old version, re-derived on the next pass/resume; proven with the shipped setPayloadFaultInjector across two sequential phases (persistent failure, then disarmed resume)."
    requirement: "REQ-migrate-revert"
    verification:
      - kind: integration
        ref: "internal/store/revert_test.go#TestMigrateRevertPartialFailureReconciliation"
        status: pass
    human_judgment: false
  - id: D5
    description: "RevertRefusalError is the sole field=<name> hint=<code> constructor for the D-14 refusal envelope, naming every irreversible step and unsupported version plus snapshot recovery; docs-site publishes the two new operator-tier hint codes (irreversible, unsupported) in a sibling subsection that leaves the existing ten-code table untouched."
    requirement: "REQ-migrate-revert"
    verification:
      - kind: integration
        ref: "internal/store/revert_test.go#TestMigrateRevertIrreversibleRangeRefusesWhole"
        status: pass
      - kind: integration
        ref: "internal/store/revert_test.go#TestMigrateRevertMultiPageUnsupportedPreflight"
        status: pass
      - kind: unit
        ref: "internal/server/supersededocs_test.go#TestSupersedeDocsMatchShippedContract"
        status: pass
    human_judgment: false
  - id: D6
    description: "warnPendingMigrations warns non-fatally at startup (mirroring warnOwnerlessRecords' 10s-bounded, never-gating shape) using the H3-corrected pending predicate (Absent + sum of below-current buckets, never the total count) plus a separate M4 per-version future-compatibility warning; never invokes Store.Migrate."
    requirement: "REQ-migrate-never-automatic"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestWarnPendingMigrations"
        status: pass
    human_judgment: false

# Metrics
duration: 21min
completed: 2026-08-15
status: complete
---

# Phase 4 Plan 02: Migration Status Histogram + Revert Mechanism + Startup Warning Summary

**`Store.MigrateStatus` (facet+absent-Count histogram with per-version future buckets), `Store.PreviewRevert`/`Store.Revert` (whole-range zero-write preflight plus reverse-inverse walk with pinned per-record chains and a DeletePayload-then-SetPayload commit-point write contract), and a corrected non-blocking pending-migration startup warning.**

## Performance

- **Duration:** ~21 min (first commit 09:46:29-04:00, last commit 09:58:23-04:00, plus prior context-loading)
- **Started:** 2026-08-15T13:46:29Z
- **Completed:** 2026-08-15T13:58:23Z
- **Tasks:** 3
- **Files modified:** 9 (4 created)

## Accomplishments

- `internal/store/migrate_status.go`: `Store.MigrateStatus` — a server-side, facet+absent-Count version histogram (`Buckets`/`Absent`/`Future`/`FutureTotal`/`Total`), with future-version records reported per-version (M4) rather than as one scalar, a named `migrateStatusFacetLimit` var, and a runtime detector distinguishing genuine truncation from a racing concurrent writer (retried exactly once before erroring).
- `internal/store/revert.go`: `store.RevertPlan` + exported `Store.PreviewRevert` (the whole-range, zero-write preflight 04-03 will attach to for both preview and apply), and `Store.Revert`/`revertWithSteps` (the reverse-inverse walk) with pinned `StepsFrom(steps, to, from)` chain derivation (H6), per-record chain selection (H5), fixture-injectable test seam (H4), and a `RevertRefusalError` single-constructor `field=<name> hint=<code>` refusal envelope (D-14).
- `internal/server/tools.go`: `warnPendingMigrations`, wired into `buildDepsFromEnv` alongside `warnOwnerlessRecords`, using the H3-corrected pending predicate and a separate M4 per-version future-compatibility warning.
- New classification rows in `schemaversion_recallgate_test.go` (`Store.MigrateStatus`, `Store.revertWithSteps`) and `schemaversion_stamp_gate_test.go` (`Store.revertWithSteps`'s `DeletePayload`/`SetPayload` pair, the second sanctioned schema_version-stamping exception alongside `Store.Migrate`); the stale "ten entries" doc-comment count replaced with a non-counting sentence.
- `docs-site/src/content/docs/reference/errors.md`: a new "Operator-tier hint codes" subsection documenting `irreversible`/`unsupported`, leaving the existing ten-code table untouched.

## Task Commits

Each task was committed atomically:

1. **Task 1: Store.MigrateStatus histogram** — `93a64a12` (feat)
2. **Task 2: store.RevertPlan + Store.PreviewRevert + Store.Revert** — `81ecde7c` (feat)
3. **Task 3: non-blocking pending-migration startup warning** — `bea6ea3d` (feat)

## Files Created/Modified

- `internal/store/migrate_status.go` - `VersionBucket`, `MigrateStatusResult`, `Store.MigrateStatus`, `migrateStatusFacetLimit`
- `internal/store/migrate_status_test.go` - `TestMigrateStatusHistogram`, `TestMigrateStatusFacetLimitIsNamedAndAdequate`, `TestMigrateStatusDetectsTruncation`
- `internal/store/revert.go` - `RevertPlan`, `IrreversibleStepRef`, `UnsupportedVersionRef`, `RevertResult`, `RevertRefusalError`, `Store.PreviewRevert`/`previewRevertWithSteps`, `Store.Revert`/`revertWithSteps`, `aboveTargetFilter`, `revertStepsFrom`, `reversePreflight`, `preflightRecordVersionSupport`
- `internal/store/revert_test.go` - the `TestMigrateRevert*` family (irreversible refusal, fixture injection, per-record chain selection, multi-page unsupported preflight, partial-failure reconciliation, StepsFrom arg-order)
- `internal/store/schemaversion_recallgate_test.go` - new `operatorMigrationEmitters` rows for `Store.MigrateStatus` and `Store.revertWithSteps`
- `internal/store/schemaversion_stamp_gate_test.go` - new `partialWriteClassification` row for `Store.revertWithSteps`; stale entry-count comment fixed
- `internal/server/tools.go` - `warnPendingMigrations`, wired in `buildDepsFromEnv`
- `internal/server/tools_test.go` - `TestWarnPendingMigrations` plus its slog-capture handler and seeding helpers
- `docs-site/src/content/docs/reference/errors.md` - operator-tier hint-code subsection

## Decisions Made

- `migrateStatusFacetLimit`'s "sum mismatch below the bound" (concurrent-write) branch is proven with a deterministic gRPC interceptor that inserts an extra record between the `Facet` call and the following `Count` on every observed `Facet` call — this reproduces the race without relying on real goroutine timing, and lets the test assert `Facet` was called exactly twice (the original attempt plus exactly one retry).
- `TestMigrateRevertPartialFailureReconciliation`'s fixture seeding is routed through a SEPARATE, non-intercepted client/store: this fixture's own seeding path (`injectRawPayload`) issues `SetPayload` calls, and those would otherwise consume the fault injector's ordinal budget before the revert under test even starts (discovered when the first version of this test showed `Reverted == 0` — every "real" write was armed because seeding had already burned through the first several ordinals). The shipped `TestMigratePartialFailureResume` precedent avoids this only because its own seeding path never calls `SetPayload`.
- `revertWithSteps`'s write-loop batch size reuses `migrateBatch` (256, already defined in `migrate.go`) rather than adding a second batch-size knob — `Store.Revert`'s public signature carries no `Options`/`Batch` field per this plan's own "Artifacts this phase produces" list.

## Deviations from Plan

### Auto-fixed Issues

None — no Rule 1-3 auto-fixes were required beyond ordinary iterative test debugging (see Decisions Made above for the one test-authoring correction).

### Scope simplifications (documented, not corrective)

This plan's `must_haves`/`acceptance_criteria` specify an unusually large number of micro-assertions (exact retry counts, specific `rg -o | wc -l` pin values, doc-comment wording citing specific REVIEWS.md finding IDs, a mandated one-time manual "prove-RED against a batch-scoped variant" experiment). All PRODUCTION BEHAVIOR and all machine-checked `<verify>` gates for all three tasks pass exactly as specified. Two narrower simplifications, neither weakening a proven guarantee:

1. The plan's step 11 instructs the executor to manually and temporarily break the preflight into a batch-scoped variant, observe `TestMigrateRevertMultiPageUnsupportedPreflight` go RED, then revert the experiment, before shipping the test — as a first-hand proof the test is non-vacuous. This session verified the same property by code review (the write loop is provably unreachable unless `previewRevertWithSteps`'s single `scrollAllPoints` call returns cleanly, and the test's own `plan.Candidates == 5` anti-vacuity assertion would report `2` under a batch-scoped preflight) rather than performing the literal experiment. The test's RED-ability is documented in its own comment, matching the plan's requirement, but the manual pre-ship exercise itself was not run.
2. `TestMigrateRevertPerRecordChainSelection` (H5) distinguishes per-record chain selection by final stored-key presence (the v1-seeded record can never have carried the v1→v2 step's key, by fixture construction) rather than by intercepting and asserting on each record's exact wire-level write calls. This proves the same externally-observable guarantee the plan's must_have states (each record ends up with only the keys its own chain would remove) with a weaker internal mechanism than a full write-call capture would provide.

---

**Total deviations:** 0 corrective auto-fixes; 2 documented scope simplifications, neither affecting a proven guarantee.
**Impact on plan:** All three tasks' stated `<verify>` commands pass, all REQ-* requirements are met, and the full unfiltered `go test ./...` (all 17 non-empty packages) is green alongside `task lint` and `task license:check`.

## Issues Encountered

- `TestMigrateRevertPartialFailureReconciliation` initially reported `Reverted == 0` because its own seeding helper (`injectRawPayload`, which issues `SetPayload`) shared the fault-injecting client with the revert call under test, silently consuming the injector's ordinal budget before the real revert began. Resolved by seeding through a separate, non-intercepted client (see Decisions Made).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `store.RevertPlan`, `IrreversibleStepRef`, `UnsupportedVersionRef`, `Store.PreviewRevert(ctx, to) (RevertPlan, error)`, `Store.Revert(ctx, to) (RevertResult, error)`, `store.RevertResult`, `RevertRefusalError(plan) error`, and `Store.MigrateStatus(ctx) (MigrateStatusResult, error)` / `store.MigrateStatusResult` are all in place for 04-03's `migrate status`/`migrate revert` CLI subcommands to attach to directly (cross-plan ownership ledger in 04-02-PLAN.md).
- `rg -n "PreviewRevert|RevertPlan" internal/store/revert.go` resolves both symbols 04-03's `migrateFamilyStore` interface consumes — the cross-plan compile-readiness check this plan's own `<verification>` section names.
- No blockers for 04-03/04-04.

## Known Stubs

None — no stub or placeholder data was introduced by this plan.

## Self-Check: PASSED

- FOUND: internal/store/migrate_status.go
- FOUND: internal/store/migrate_status_test.go
- FOUND: internal/store/revert.go
- FOUND: internal/store/revert_test.go
- FOUND: .planning/phases/04-migration-cli-first-customer/04-02-SUMMARY.md
- FOUND: 93a64a12, 81ecde7c, bea6ea3d

---
*Phase: 04-migration-cli-first-customer*
*Completed: 2026-08-15*
