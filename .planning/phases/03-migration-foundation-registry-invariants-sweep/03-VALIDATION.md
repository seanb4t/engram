---
phase: 3
slug: migration-foundation-registry-invariants-sweep
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-13
validated: 2026-08-16
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (Go 1.x, stdlib testing) |
| **Config file** | none — `Taskfile.yaml` wraps lint + test as `task` |
| **Quick run command** | `go test ./internal/migrate/... -count=1` |
| **Full suite command** | `task` (lint + `go test ./...`) |
| **Estimated runtime** | ~0.2s (migrate leaf, pure) · ~3s (store, phase-3 tests) · ~100s (red-evidence harness) |

**Note on the two tiers.** `internal/migrate` is stdlib-only and has no Qdrant dependency (SC1),
so its tests are sub-second and can run after every task commit with no container cost. The
`internal/store` sweep tests require a real pinned Qdrant via testcontainers and belong to the
per-wave tier. **Prefix every command below with `ENGRAM_REQUIRE_QDRANT=1`** so a missing Qdrant
fails instead of skipping silently.

**Re-resolve every `-run` before trusting it.** `go test -run X` that matches nothing exits 0 with
`ok … [no tests to run]`. This repo has been bitten by that false green across two milestones
(durable record `bsbsvn4hbc`). Every `-run` in this file has been re-resolved against the real
source and proven with an explicit `--- PASS` line, never a package-level `ok`.

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/migrate/... -count=1`
- **After every plan wave:** Run `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/migrate/... ./internal/store/... -count=1`
- **Before `/gsd-verify-work`:** Full suite (`task`) must be green
- **Max feedback latency:** ~0.2s (migrate leaf), ~5s (both packages, `-short`), ~110s (including the red-evidence harness)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 3-01-01 | 01 | 1 | REQ-migration-step-registry | T-03-01 | A legacy key-absent record migrates v0→v1 through registry→sweep→filter; a second run writes nothing | integration | `go test ./internal/store/... -run '^TestMigrateTracerLegacyRecordEndToEnd$' -count=1 -v` | ✅ | ✅ green |
| 3-01-02 | 01 | 1 | REQ-migrate-converges-without-lock | T-03-04 | `backlogFilter` partitions absent-key / below-target / current, and never reaches a recall or authz filter | integration | `go test ./internal/store/... -run '^TestBacklogFilterMatchesAbsentAndBelowTarget$' -count=1 -v` | ✅ | ✅ green |
| 3-01-03 | 01 | 1 | REQ-migration-additive-only-gated | T-03-01 / T-03-12 | A step whose behavior diverges from `AddsKeys` is refused before any write; the write map is built from added keys only | integration | `go test ./internal/store/... -run '^TestMigrateRefusesNonAdditiveStep$' -count=1 -v && go test ./internal/store/... -run '^TestMigrateWritesOnlyAddedKeys$' -count=1 -v` | ✅ | ✅ green |
| 3-02-01 | 02 | 2 | REQ-migration-step-reversibility | T-03-05 / T-03-07 / T-03-21 | Explicit-nil reversibility panics at construction; the `Reversibility` seal is closed to out-of-package implementors | unit | `go test ./internal/migrate/... -run '^TestNewStepPanicsOnNilReversibility$' -count=1 -v && go test ./internal/migrate/... -run '^TestNewStepPanicsOnNilApplyFunc$' -count=1 -v && go test ./internal/migrate/... -run '^TestIrreversiblePanicsOnEmptyReason$' -count=1 -v && go test ./internal/migrate/... -run '^TestReversiblePanicsOnNilInverse$' -count=1 -v && go test ./internal/migrate/... -run '^TestReversibilityIsSealedToThisPackage$' -count=1 -v` | ✅ | ✅ green |
| 3-02-02 | 02 | 2 | REQ-migration-step-registry | T-03-22 | `Validate`'s three rules each observed; `Registry` cannot be relocated into a builder without failing | unit | `go test ./internal/migrate/... -run '^TestValidateRejectsOrderingAndUniquenessViolations$' -count=1 -v && go test ./internal/migrate/... -run '^TestStepsFromSelectsContiguousChain$' -count=1 -v && go test ./internal/migrate/... -run '^TestRegistryIsAPackageLevelVarWithPhase4Marker$' -count=1 -v` | ✅ | ✅ green |
| 3-02-03 | 02 | 2 | REQ-migration-step-registry | T-03-09 | `internal/migrate` is stdlib-only with zero module imports; the per-version decoder door stays open and unclaimed | unit | `go test ./internal/migrate/... -run '^TestMigratePackageIsStdlibOnlyLeaf$' -count=1 -v && go test ./internal/migrate/... -run '^TestDecoderDoorIsOpenAndUnclaimed$' -count=1 -v` | ✅ | ✅ green |
| 3-03-01 | 03 | 2 | REQ-migration-additive-only-gated | T-03-11 / T-03-20 | Set equality on the added-key set, distinguished from subset and superset; in-place input mutation still classified non-conforming | unit | `go test ./internal/migrate/... -run '^TestAdditiveOnlyKeySetDiff$' -count=1 -v` | ✅ | ✅ green |
| 3-03-02 | 03 | 2 | REQ-migration-additive-only-gated | T-03-08 | The additive table cannot pass vacuously — zero rows, one verdict class, or an input-ignoring checker each proven RED | integration | `go test ./internal/store/... -run '^TestRedEvidencePatchesAreLive$' -count=1 -v` | ✅ | ✅ green |
| 3-04-01 | 04 | 2 | REQ-migrate-partial-failure-resume | T-03-13 / T-03-14 / T-03-16 | Partial `SetPayload` application self-heals; a lying write error still converges; the injector is proven to have fired | integration | `go test ./internal/store/... -run '^TestMigratePartialFailureResume$' -count=1 -v` | ✅ | ✅ green |
| 3-04-02 | 04 | 2 | REQ-migrate-partial-failure-resume | T-03-08 | Trusting the write-error signal instead of re-deriving the backlog is proven RED | integration | `go test ./internal/store/... -run '^TestRedEvidencePatchesAreLive$' -count=1 -v` | ✅ | ✅ green |
| 3-05-01 | 05 | 2 | REQ-migrate-converges-without-lock | T-03-17 / T-03-18 / T-03-19 | An already-current mid-sweep write is never re-processed (proven at the wire and in the collection); a below-target sibling is | integration | `go test ./internal/store/... -run '^TestMigrateConvergesWithoutLock$' -count=1 -v` | ✅ | ✅ green |
| 3-05-02 | 05 | 2 | REQ-migrate-converges-without-lock | T-03-08 | Widening `backlogFilter` to include already-current records, and skipping the mid-sweep write, are each proven RED | integration | `go test ./internal/store/... -run '^TestRedEvidencePatchesAreLive$' -count=1 -v` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

The three RED-cycle tasks (`3-03-02`, `3-04-02`, `3-05-02`) share one command because
`TestRedEvidencePatchesAreLive` covers every patch in this phase's `red-evidence/` directory in a
single run; each task's own patches are individually named subtests within it.

**Run that command once at a time.** The harness mutates the working tree, so two concurrent runs
interleave their apply/revert cycles and can leave a source file dirty — the per-patch dirty-check
fails the second run, but the damage to the tree is already possible. `go test ./...` is fine (one
package, not `t.Parallel`); the hazard is launching a second run while one is in flight. Verified
the hard way on 2026-08-16. If a run dies unexpectedly, check `git status --porcelain` before
trusting the tree.

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements — all three seeded Wave 0 items were
delivered:

- [x] `internal/migrate/` fixture steps exercising every registry invariant — delivered across
      `step_test.go`, `registry_test.go`, and `additive_test.go`'s eight-row table (conforming
      additive step, irreversible step with a stated reason, a key-removing step, and a step whose
      actual adds diverge from its declared `addsKeys`)
- [x] sweep tests against a real pinned Qdrant — `internal/store/migrate_test.go`,
      `migrate_faultinject_test.go`, `migrate_converge_test.go`
- [x] gRPC fault-injection interceptor extending the existing seam — `setPayloadFaultInjector` and
      `midSweepInterceptor`; the rejected `setPayloadKeys` test-hook field pattern was **not**
      resurrected (T-03-15 confirms production `Store` stayed byte-identical)

*Framework install: not required.*

---

## Manual-Only Verifications

All phase behaviors have automated verification.

Every success criterion is machine-checkable: SC1 by an import/dependency assertion, SC2/SC3 by
compile-failure and registry-invariant tests, SC4/SC5 by integration tests against a real pinned
Qdrant with deterministic fault injection. Nothing here needs a human to look at a screen.

The RED cycles — originally `kind: other` shell commands a human had to run — are now automated by
`TestRedEvidencePatchesAreLive`, so no deliverable in this phase is manual-only.

---

## Non-Vacuity Requirements (phase-specific)

Verified 2026-08-16, each by reading the shipped test and executing it:

- [x] The additive-only key-set diff (D-04) asserts a **non-zero fixture count** — guard at
      `additive_test.go:184-185`, executed before the loop it guards (T-03-08); `03-03-red-3-zero-fixtures.patch`
      proves it RED.
- [x] The additive-only diff asserts **set equality in both directions** — mirrored fixture rows
      "adds an undeclared key" / "declares a key it never adds" (T-03-11); `03-03-red-1-superset-not-equality.patch`
      proves the superset direction is load-bearing.
- [x] The backlog-derivation test seeds a **genuinely key-absent** record via raw payload injection —
      `TestBacklogFilterMatchesAbsentAndBelowTarget`; `03-01-red-1-range-only-filter.patch` proves a
      bare `Range` (without `IsEmpty`) silently derives an empty backlog.
- [x] Each RED cycle is proven by an exact reversible patch under `red-evidence/`, with
      `git diff --exit-code` clean after revert — **now enforced continuously** rather than observed
      once, by `TestRedEvidencePatchesAreLive`. See the audit below: this requirement was silently
      violated for four patches before today.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies — 12/12
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references — all three delivered
- [x] No watch-mode flags
- [x] Feedback latency < 45s for the per-wave tier (the red-evidence harness is a pre-ship gate, not a per-commit one)
- [x] Every `-run` re-resolved against real source and proven with `-v` RUN/PASS pairs
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-08-16

---

## Validation Audit 2026-08-16

| Metric | Count |
|--------|-------|
| Gaps found | 2 |
| Resolved | 2 |
| Escalated | 0 (1 finding converted to issue #501) |

This file was **seeded at plan time** with provisional task IDs (`3-0?-*`) and predicted test names.
Those predictions were coarse but not wrong — each predicted `-run` (`TestValidate`,
`TestIrreversible`, `TestAdditiveOnly`, `TestMigratePartialFailure`, `TestMigrateConverges`) happens
to be a prefix of the delivered name, and Go's `-run` is an unanchored match, so they resolved by
luck rather than by design. All twelve rows above are now reconciled to real task IDs and exact,
anchored test names, and all seventeen named test functions were confirmed to resolve to real source
and executed green under `ENGRAM_REQUIRE_QDRANT=1`.

**G-1 — Phase 4 silently broke four of this phase's RED-proof patches.** `03-01-red-2-declared-drift-written`,
`03-03-red-1-superset-not-equality`, `03-04-red-1-persisted-cursor` and `03-05-red-2-midsweep-write-skipped`
no longer applied. Cause: Phase 4's `8fb9d6d9`, `0fa76d62`, `96711281`, `9695616d`. `0fa76d62` is
literally *"repair internal/store tests for the CurrentVersion 0→1 blast radius"* — Phase 4 repaired
the **tests** and left the **evidence** for the same code behind, because only the tests had a gate.

Three were regenerated (commit `afcf832e`) by re-deriving each mutation against current source —
never by transcribing the stale patch, since `additive.go` had been semantically changed by Phase 4's
CheckAdditive carve-out — and each was re-proven through the full apply → RED → revert cycle.

**G-2 — the RED cycles had no automated gate.** `TestRedEvidencePatchesAreLive`, built for Phase 2,
was generalized from one hardcoded directory to a per-phase map. Each phase directory gets its own
glob, its own zero-applicability `t.Fatal`, and its own both-directions set-equality mapping check,
so one phase's evidence can never silently cover for another's. Observed: 21 patch subtests across
two phase directories, all green, tree clean after; Phase 2's original 9 still pass unchanged.

**The fourth patch was never evidence, and is retired.** `03-04-red-1-persisted-cursor.patch` does
not drive any test RED — and did not on the day it was written. `03-04-SUMMARY.md`'s own
key-decisions entry records it: *"Cycle 1 (persisted cursor) reddened ZERO of the three subtests
against the plan's own committed fixtures."* Three candidate targets were re-tried against current
source and all stayed green. The mutation only misbehaves when a below-target record whose id sorts
*before* an already-advanced cursor arrives mid-sweep, and no test constructs that ordering.

It was deleted rather than exempted: a patch that cannot go RED is not evidence, and keeping it
would have required a per-item exclusion in the harness — the exact sampling hatch the harness
exists to remove. It is recoverable from git history. The genuine coverage gap it points at is filed
as **issue #501**, and coverage deliverable `03-04` D4 should be read as carried by
`03-04-red-2-trust-error-signal.patch` alone.

### The finding worth carrying forward

This phase's threat register anticipated Phase 4 threatening its **registry** (T-03-22, *"Phase 4
relocating `Registry` into a builder function"*) and gated it with
`TestRegistryIsAPackageLevelVarWithPhase4Marker` — that gate held. It did not anticipate Phase 4
threatening its **evidence**. T-03-23 covers the RED patches and is classified *"Not code-auditable"*,
its evidence a one-time human observation: *"13 distinct red-evidence patches … clean working tree."*
That is precisely the assumption that decayed, and precisely what a machine gate would have caught
the day it broke. A property worth a threat-register row is worth a gate, not a snapshot.
