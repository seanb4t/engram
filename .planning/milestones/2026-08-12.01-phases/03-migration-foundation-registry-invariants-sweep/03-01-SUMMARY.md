---
phase: 03-migration-foundation-registry-invariants-sweep
plan: 01
subsystem: database
tags: [go, qdrant, migration, schema-versioning, testcontainers]

# Dependency graph
requires:
  - phase: 02-record-schema-versioning-foundation
    provides: internal/migrate leaf package (Version, CurrentVersion), schema_version payload key, monotonic stamping in payload()
provides:
  - internal/migrate step registry (NewStep, sealed Reversibility, Validate, StepsFrom, CheckAdditive)
  - internal/store's Store.Migrate re-derive-every-pass sweep with backlogFilter's absent-key-aware nested OR-group
affects: [04-migration-cli-and-first-customer]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 13108
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Sealed interface via unexported marker method (migrate.Reversibility) — closes the foreign-type escape hatch; explicit nil checks close the nil-interface hatch sealing cannot"
    - "Two-direction key-set diff (AddedKeys/RemovedKeys, set-equality not subset/superset) as a fail-closed apply-time gate, mirrored in production code and test fixtures"
    - "Re-derive-every-pass sweep with no persisted cursor — resume is 'call it again', never a stale-offset reconciliation"
    - "Nested Must:[FilterAsCondition(Should:[Range,IsEmpty])] OR-group for an absent-key-aware Qdrant filter, copied from activeWindowConditions' in-repo precedent"

key-files:
  created:
    - internal/migrate/step.go
    - internal/migrate/registry.go
    - internal/migrate/additive.go
    - internal/store/migratebacklog.go
    - internal/store/migrate.go
    - internal/store/migrate_test.go
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-01-red-1-range-only-filter.patch
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-01-red-2-declared-drift-written.patch
  modified:
    - internal/store/schemaversion_recallgate_test.go
    - internal/store/schemaversion_stamp_gate_test.go

key-decisions:
  - "additive.go (CheckAdditive) shipped in Task 1's commit rather than Task 3's — Task 1's own action text requires Store.Migrate to call migrate.CheckAdditive in its per-step loop, so the package cannot compile without it; the plan's per-task <files> list for Task 1 omitted it. Documented as a deviation below."
  - "Store.Migrate added to internal/store's two pre-existing completeness gates (operatorMigrationEmitters, partialWriteClassification) — both went red on the new method's addition; Migrate's SetPayload is the sanctioned D-02 exception (it stamps schema_version only after applying the record's full step chain, not as an unverified one-key merge)"

patterns-established:
  - "Migration step: migrate.NewStep(from, to, addsKeys, rev, apply) — positional-required constructor, no exported struct-literal path, panics on nil rev/apply"
  - "Sweep loop: fresh Count -> non-shrinking-backlog termination guard -> fresh Scroll (Offset always nil) -> per-point step chain with double-clone per step -> CheckAdditive -> added-keys-only SetPayload"

requirements-completed: [REQ-migration-step-registry, REQ-migration-additive-only-gated, REQ-migration-step-reversibility, REQ-migrate-converges-without-lock]

coverage:
  - id: D1
    description: "One genuinely-key-absent legacy record migrates v0->v1 end to end (registry -> sweep -> backlogFilter -> real Qdrant), and a second identical run writes nothing and changes nothing"
    requirement: "REQ-migration-step-registry"
    verification:
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateTracerLegacyRecordEndToEnd"
        status: pass
    human_judgment: false
  - id: D2
    description: "backlogFilter correctly partitions absent-key, below-target, and current records; target<=0 is a sweep no-op even though the filter alone is broad at target 0; a range-only filter observed turning the proof RED"
    requirement: "REQ-migrate-converges-without-lock"
    verification:
      - kind: integration
        ref: "internal/store/migrate_test.go#TestBacklogFilterMatchesAbsentAndBelowTarget"
        status: pass
    human_judgment: false
  - id: D3
    description: "A step whose actual behavior diverges from its AddsKeys declaration is refused before any write, including a step that mutates its input map in place; bypassing the check observed letting the undeclared key through"
    requirement: "REQ-migration-additive-only-gated"
    verification:
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateRefusesNonAdditiveStep"
        status: pass
    human_judgment: false
  - id: D4
    description: "A step conforming by key set but overwriting an existing value's content has that overwrite silently discarded — the write map is built from added keys only"
    requirement: "REQ-migration-additive-only-gated"
    verification:
      - kind: integration
        ref: "internal/store/migrate_test.go#TestMigrateWritesOnlyAddedKeys"
        status: pass
    human_judgment: false

duration: 13min
completed: 2026-08-14
status: complete
---

# Phase 3 Plan 1: Migration Registry, Additive-Only Diff, and Re-Derive Sweep Summary

**`internal/migrate` step registry (sealed Reversibility, positional-required NewStep, Validate, StepsFrom, CheckAdditive) plus `Store.Migrate`'s re-derive-every-pass sweep with an absent-key-aware nested backlog filter — proven end to end against real Qdrant with two committed RED-evidence cycles**

## Performance

- **Duration:** ~13 min
- **Started:** 2026-08-14T13:41:00Z (approx.)
- **Completed:** 2026-08-14T13:53:38Z
- **Tasks:** 3
- **Files modified:** 10 (8 created, 2 modified)

## Accomplishments

- `internal/migrate` grew from a two-symbol leaf into a full step registry: sealed `Reversibility` (unexported marker method, two constructors, explicit nil checks that sealing alone cannot provide), `Step` with every field unexported, `NewStep`'s positional-required constructor, `Validate`'s three ordering rules (transition uniqueness / advance / contiguity, each named for exactly what it checks — not "idempotency"), `StepsFrom`'s per-record chain selection, and `CheckAdditive`'s two-direction (removed-empty, added-set-equal-to-declared) key-set diff — all stdlib-only (`go list -deps` confirms zero third-party imports).
- `Store.Migrate` sweeps the collection with no persisted cursor: a fresh exact `Count` every pass, a non-shrinking-backlog termination guard derived from that fresh count (never from a write signal), a fresh `Scroll` with `Offset` always `nil`, a mandatory double-clone per step (defends against an `ApplyFunc` that mutates its input in place), a fail-closed `CheckAdditive` call before any write, and a write map built from `AddedKeys(original, current)` plus `schema_version` — never from the step's full returned payload.
- `backlogFilter` uses the nested `Must:[FilterAsCondition(Should:[Range,IsEmpty])]` OR-group (copied in shape from `activeWindowConditions`), because Qdrant evaluates a bare `Range` on an absent key as `false`, not "below everything" — the phase's single highest-risk line, proven both by a passing test and by a committed RED-evidence cycle showing the naive range-only filter converges having migrated nothing.
- One end-to-end tracer moves a genuinely-key-absent legacy record through every layer to a real pinned Qdrant and back, then proves sweep-level idempotence: an identical second `Migrate` call writes nothing (`Migrated==0`, `Failed==0`, `Passes==1`) and changes nothing (`reflect.DeepEqual` raw-payload snapshot).
- Two RED-evidence cycles, both committed as reviewer-reproducible patches: (1) a range-only `backlogFilter` observed losing the absent-key record from the derived backlog; (2) a bypassed `CheckAdditive` call site observed letting an undeclared key land in Qdrant, confirmed with a throwaway (uncommitted) diagnostic before reverting.

## Task Commits

1. **Task 1: End-to-end tracer** — `0363c6e0` (feat) — registry, additive diff (see Deviations), backlog filter, sweep, tracer test + helpers
2. **Task 2: Prove the backlog filter, RED against range-only** — `df66d108` (test) — `TestBacklogFilterMatchesAbsentAndBelowTarget` + red-evidence patch 1
3. **Task 3: Fail-closed additive-only enforcement, RED against a bypassed check** — `63caa6fd` (test) — `TestMigrateRefusesNonAdditiveStep`, `TestMigrateWritesOnlyAddedKeys` + red-evidence patch 2 + two gate-classification fixes

**Plan metadata:** pending (this SUMMARY's own commit)

## Files Created/Modified

- `internal/migrate/step.go` — `ApplyFunc`, sealed `Reversibility` (+ `Reversible`/`Irreversible`/`Inverse`/`IrreversibleReason`), `Step`, `NewStep`
- `internal/migrate/registry.go` — empty package-level `Registry` with the `// PHASE4:` load-bearing marker, `Validate`, `StepsFrom`
- `internal/migrate/additive.go` — `AddedKeys`, `RemovedKeys`, `CheckAdditive` (see Deviations for why this shipped in Task 1)
- `internal/store/migratebacklog.go` — `backlogFilter`, `versionOf`, `payloadToMap` (all seven `qdrant.Value` variants by name)
- `internal/store/migrate.go` — `MigrateOptions`, `MigrateResult`, `Store.Migrate`
- `internal/store/migrate_test.go` — all four of this plan's tests plus `markerStep`/`seedLegacyRecord`/`migrateBacklogIDs`/`hasPayloadKey`/`rawPayloadSnapshot` helpers
- `internal/store/schemaversion_recallgate_test.go` — added `Store.Migrate` to `operatorMigrationEmitters`
- `internal/store/schemaversion_stamp_gate_test.go` — added `Store.Migrate` to `partialWriteClassification`
- `.planning/.../red-evidence/03-01-red-1-range-only-filter.patch`, `03-01-red-2-declared-drift-written.patch`

## Decisions Made

- **`additive.go` shipped in Task 1, not Task 3.** Task 1's own action text specifies that `Store.Migrate`'s per-step loop calls `migrate.CheckAdditive` before any write — the package cannot compile or the tracer cannot run without `CheckAdditive` existing. The plan's per-task `<files>` list for Task 1 did not include `internal/migrate/additive.go` (it appears only in Task 3's `<files>` and in the plan-level frontmatter manifest). Rather than stub `CheckAdditive` in Task 1 and rewrite it in Task 3, the full, correctly-documented implementation matching Task 3's exact specification was written once in Task 1's commit; Task 3's commit then adds only the two dedicated tests (`TestMigrateRefusesNonAdditiveStep`, `TestMigrateWritesOnlyAddedKeys`) and its own RED-evidence patch, which together fully discharge Task 3's `<done>` criteria without any further change to `additive.go` itself.
- **`Store.Migrate` classified in two pre-existing completeness gates.** `TestRecallEmissionSetIsCompleteAndClassified` and `TestPartialWritePathsAreClassifiedNonStamping` (both from Phase 2) derive every `Count`/`ScrollAndOffset` and every `SetPayload`/`DeletePayload`/`OverwritePayload` call site in `internal/store` by AST scan and require each to carry a classification entry; `Store.Migrate`'s addition made both gates fail (`Store.Migrate` derived but unclassified). Added to `operatorMigrationEmitters` (Count/ScrollAndOffset against `backlogFilter`, never reachable from a recall entry point — same D-16 rationale as `BackfillShortIDs`/`Reindex`) and to `partialWriteClassification` with a justification explaining `Store.Migrate`'s `SetPayload` is the **sanctioned exception** to D-02's "a partial write must never stamp currency it cannot honor" rule: unlike every other entry, it stamps `schema_version` only after applying the record's full step chain, and its write map is built from exactly the keys that chain declared and added — a correctly-earned currency claim, not an unverified one-key merge.
- **Doc-comment wording adjusted to keep a grep-pinned acceptance criterion exact.** `backlogFilter`'s doc comment originally referenced `qdrant.NewFilterAsCondition` in prose, which doubled the acceptance criterion's `rg -c 'qdrant\.NewFilterAsCondition'` count from the required 1 to 2. Reworded the prose to describe the wrapper without repeating the literal call expression; the code itself is unchanged.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `internal/migrate/additive.go` created in Task 1 instead of Task 3**
- **Found during:** Task 1 (writing `Store.Migrate`'s per-step loop, which the plan's own action text specifies must call `migrate.CheckAdditive`)
- **Issue:** Task 1's `<files>` list omits `internal/migrate/additive.go`, but Task 1's action requires the sweep to call `migrate.CheckAdditive` — the package would not compile without it.
- **Fix:** Wrote the complete, fully-documented `CheckAdditive`/`AddedKeys`/`RemovedKeys` implementation (matching Task 3's specification exactly) in Task 1's commit.
- **Files modified:** `internal/migrate/additive.go`
- **Verification:** `go build ./...` succeeds; Task 3's dedicated tests (`TestMigrateRefusesNonAdditiveStep`, `TestMigrateWritesOnlyAddedKeys`) pass against this same implementation with no further edits to `additive.go`.
- **Committed in:** `0363c6e0` (Task 1 commit)

**2. [Rule 1 - Bug] Two pre-existing completeness gates regressed red on `Store.Migrate`'s addition**
- **Found during:** Task 3 (`task` full-suite run after adding Task 3's tests)
- **Issue:** `TestRecallEmissionSetIsCompleteAndClassified` and `TestPartialWritePathsAreClassifiedNonStamping` derive every emission/partial-write call site in `internal/store` by AST scan and assert set-equality against a hand-maintained classification list; `Store.Migrate`'s `Count`/`ScrollAndOffset`/`SetPayload` calls were derived but had no classification entry.
- **Fix:** Added `Store.Migrate` to `operatorMigrationEmitters` (recall gate) and to `partialWriteClassification` (stamp gate), each with a specific, non-boilerplate justification (see Decisions above).
- **Files modified:** `internal/store/schemaversion_recallgate_test.go`, `internal/store/schemaversion_stamp_gate_test.go`
- **Verification:** Both gate tests pass; full `task` (lint + repo-wide `go test ./...`) is green.
- **Committed in:** `63caa6fd` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both were necessary for the plan's own action text to compile and for pre-existing Phase 2 gates to stay green with the new method's addition. No scope creep — no new production behavior was added beyond what the plan specified.

## Issues Encountered

- **Acceptance-criteria false-positive: `go list -deps ./internal/migrate | rg -c '^[^/]+\.[^/]+/'` prints `1`, not the `0` the plan's acceptance criteria specifies.** The pattern matches the module's own import path (`github.com/seanb4t/engram/internal/migrate`), which `go list -deps` always includes for the package being listed, and whose module path (`github.com/...`) happens to contain a dot — matching the "non-stdlib" regex against itself. Verified this is not specific to this plan's code: the identical command against the pre-existing, known-good stdlib-only leaf `internal/openaiurl` also prints `1`. The actual invariant this criterion is checking — zero non-stdlib runtime dependencies — is independently confirmed by inspecting the full `go list -deps ./internal/migrate` output, which lists only Go stdlib packages plus the package itself. Recording per the repo's own `bsbsvn4hbc` precedent (verification commands can false-green/false-fail) rather than silently reporting the criterion as passed.
- **RED-evidence cycle 1's observed sub-case verdicts diverge from the plan's stated prediction.** The plan predicted "the set-equality sub-case and the non-vacuity sub-case are expected to flip; the target<=0 sub-cases are expected to STAY GREEN." Observed reality: the set-equality sub-case flipped (as predicted, naming `absent` as missing); the non-vacuity sub-case (structured as two separate assertions — non-empty, and excludes-current) did **not** flip, because under the naive range-only filter the backlog is `[belowID]` alone, which is still non-empty and still excludes `current`; and of the two target<=0 sub-cases, the filter-alone-is-broad assertion **did** flip (the naive filter's `IsEmpty` arm no longer exists, so `backlogFilter(0)` no longer contains the absent record), while the sweep-at-target-0 assertion stayed green exactly as predicted (`Store.Migrate`'s short-circuit returns before ever building the filter). This is honest, partial, selective evidence rather than "reddens every sub-case" — recorded here per the plan's own instruction to label any divergence from its prediction rather than presenting it as a clean match.
- **RED-evidence cycle 2's "record gained the undeclared key" claim was confirmed via a throwaway, uncommitted diagnostic test** (not `TestMigrateRefusesNonAdditiveStep` itself, which asserts only `err != nil` and stops at the first `t.Fatalf` on the primary sub-case before reaching a key-presence check). The diagnostic seeded a legacy record, ran `Store.Migrate` with the bypass in place and a step declaring one key while adding two, and logged `hasUndeclared=true hasDeclared=true` — confirming the undeclared key physically landed in Qdrant under the bypass, exactly as the plan requires this cycle to demonstrate. The diagnostic was removed before reverting the bypass; it was never committed.

## Next Phase Readiness

- `migrate.CurrentVersion` is still `0` and `migrate.Registry` is still an empty package-level `var` carrying the `// PHASE4:` marker — unchanged, as required.
- `Store.Migrate` and the full `internal/migrate` registry API are ready for Phase 4 to register the first real step (`backfill-short-ids`, v0->v1) and wire the CLI.
- Plans 03-02 (decoder door, sealed-interface tests, leaf-purity), 03-03 (additive-only fixture table), 03-04 (partial-failure fault injection), and 03-05 (convergence-without-lock) all build directly on this plan's `Step`/`Registry`/`CheckAdditive`/`Store.Migrate` surface with no further changes needed here.
- No blockers.

## Self-Check: PASSED

All 8 created files verified present (`internal/migrate/step.go`, `registry.go`, `additive.go`; `internal/store/migratebacklog.go`, `migrate.go`, `migrate_test.go`; both red-evidence patches) plus this SUMMARY.md. All three task commits verified present in git history (`0363c6e0`, `df66d108`, `63caa6fd`).

---
*Phase: 03-migration-foundation-registry-invariants-sweep*
*Completed: 2026-08-14*
