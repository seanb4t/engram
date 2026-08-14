---
phase: 03-migration-foundation-registry-invariants-sweep
verified: 2026-08-14T15:10:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 3: Migration Foundation — Registry Invariants & Sweep Verification Report

**Phase Goal:** A pure, dependency-free migration-step registry exists, and no step can be
registered without declaring both additive-only compliance and its reversibility — enforced
structurally, not caught in review. The sweep that drives it survives Qdrant's real batch
non-atomicity and converges without a collection lock.

**Verified:** 2026-08-14T15:10:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Method

Every claim below was checked directly against the codebase, not inferred from SUMMARY.md
prose. Specifically: `internal/migrate/*.go` and `internal/store/migrate*.go` were read in
full; `go list -deps ./internal/migrate` was run and its output filtered by hand;
`go build ./...`, `go vet ./...`, and `task lint` were run and returned clean; the full
`internal/migrate` suite and the full `internal/store` suite were each run once (the latter
spinning up a real `qdrant/qdrant:v1.18.2` testcontainer via Docker, confirmed present in this
environment) and both passed with zero skips on the migration-relevant tests. The
behavior-dependent success criteria (4 and 5) were confirmed to run against a **real Qdrant
instance**, not a mock — `TestMigratePartialFailureResume` and `TestMigrateConvergesWithoutLock`
both executed and passed live in this session.

## Goal Achievement

### Success Criteria (from ROADMAP.md)

| # | Criterion | Status | Evidence |
|---|-----------|--------|----------|
| 1 | `internal/migrate` is a stdlib-only leaf, imported one-way by `internal/store`, with a single `Validate` invariant over ordering/idempotency-precondition | VERIFIED | `go list -deps ./internal/migrate` shows only stdlib packages (`io/fs` etc.) plus the package itself — zero `qdrant`/`auth`/module imports. `rg -l 'internal/migrate' internal/store/*.go` shows the import direction is one-way (migrate files reference "internal/store" only in comments, never in import statements — confirmed by direct read). `internal/migrate/registry.go:Validate` checks transition-uniqueness, advance, and contiguity, with its doc comment explicitly and correctly disclaiming that it alone proves idempotence. `TestMigratePackageIsStdlibOnlyLeaf` (`leafpurity_test.go`) is a non-vacuous AST gate (`t.Fatal`s on zero scanned files) and passes. All `internal/migrate` tests pass (`go test ./internal/migrate/...` — 100% green, ran directly). Per the documented known-open-item, the `go list -deps | rg -c '^[^/]+\.[^/]+/'` acceptance command from the plans is a false positive (reproduces against the pre-existing clean `internal/openaiurl` leaf too) — criterion 1 is verified via the test and via `go list -deps` read correctly, not via that command. |
| 2 | A non-additive step fails to build or fails a test, not a review catch; the step interface leaves room for a per-version decoder | VERIFIED | `internal/migrate/additive.go:CheckAdditive` diffs key sets in BOTH directions (removed-keys check + set-equality on added-vs-declared, read line-by-line, not a subset/superset). `internal/store/migrate.go:230-236` calls `CheckAdditive` before every write and refuses the whole sweep on violation (apply-time enforcement, fail-closed). `TestMigrateRefusesNonAdditiveStep` (3 subtests: undeclared extra key, removed key via copy, removed key via in-place mutation) passes live against real Qdrant. `TestAdditiveOnlyKeySetDiff`'s 8-row fixture table in `additive_test.go` asserts both verdict classes are represented (`t.Fatalf` guard on `conformingCount==0 || nonConformingCount==0`) — non-vacuous. `internal/migrate/decoder.go` defines an optional `Decoder` interface that `Step` does not implement today, proven reachable-by-embedding via `TestDecoderDoorIsOpenAndUnclaimed`. |
| 3 | A step silent about reversibility fails the same way — "nobody thought about it" is unrepresentable | VERIFIED | `internal/migrate/step.go:NewStep` panics on `rev == nil` and `apply == nil` (explicit checks, correctly reasoned in the doc comment as necessary beyond positional-required alone, since Go permits explicit nil for interface params). `Reversibility` is a sealed interface (unexported `isReversibility()` marker) satisfiable only by the package's own two unexported types; `Reversible(nil)` and `Irreversible("")` both panic. `TestNewStepPanicsOnNilReversibility`, `TestReversiblePanicsOnNilInverse`, `TestIrreversiblePanicsOnEmptyReason`, and `TestReversibilityIsSealedToThisPackage` (including an out-of-package build-failure probe) all pass. |
| 4 | `Store.Migrate` survives a forced mid-sequence partial `SetPayload` failure against a real pinned Qdrant, and a resume converges the backlog to zero via re-derivation, never trusting the write call's own signal | VERIFIED (with one documented, structural-only sub-property — see Notes) | `TestMigratePartialFailureResume` in `internal/store/migrate_faultinject_test.go` was run live against a real `qdrant/qdrant:v1.18.2` testcontainer in this session and passed all 3 subtests: (a) a single dropped write self-heals in a later pass with per-record evidence of the specific recovered record; (b) a persistent failure terminates on the non-shrinking-backlog guard, and a resume through a **second, independent `*Store`/client** converges, with the resume's own writes independently observed and asserted set-equal to exactly the outstanding ids; (c) every write commits at the server but every call reports an error ("the error lies") — the sweep's counters are asserted wrong while the collection is asserted right, proving D-09's re-derivation discipline. The injection is via a real `grpc.WithUnaryInterceptor` on the dial options, so production code is exercised byte-identically. |
| 5 | The sweep runs with no collection lock, because the write path stamps current version before the sweep runs, proven by mid-sweep writes never being re-processed | VERIFIED (proven property is narrower than the criterion's full causal claim — see Notes; not a gap, per known-open-item #2) | `TestMigrateConvergesWithoutLock` was run live against real Qdrant and passed all 3 subtests: an already-current mid-sweep write (via the real `Store.Upsert`/`payload()` write path) is never selected for a `SetPayload` and never gains the step's marker key; a below-target mid-sweep write (the positive control) IS picked up and migrated; the sweep converges to backlog=0 with the mid-sweep trigger proven to have actually fired exactly once (integer counter inside a `sync.Once`, not a boolean) and more than one scroll pass observed. No lock, mutex, or coordination primitive is used anywhere in the test or in `Store.Migrate`. |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified)

### Notes on Criteria 4 and 5 (documented, not gaps)

**Criterion 4 — persisted-cursor property is structurally, not behaviorally, proven.**
`Store.Migrate`'s design deliberately never persists a scroll cursor across passes (`Offset: nil` on every call, D-07) — this is what makes "resume" nothing more than calling `Migrate` again. Plan 03-04's RED-evidence cycle 1 (reintroducing a persisted cursor) reddened **zero** of the three `TestMigratePartialFailureResume` subtests rather than the predicted selective signature, because Qdrant's own scroll-exhaustion behavior (`next_page_offset == nil` at collection end) incidentally resets a persisted cursor and recovers the skipped record before the non-shrinking-backlog guard could fire. I independently confirmed the property is protected **structurally**: `rg -n 'Offset:' internal/store/*.go` (excluding tests) shows exactly one `Offset: nil` in `migrate.go` and zero `Offset: offset` / cursor-threading shapes anywhere in the sweep — matching plan 03-01's pinned acceptance criterion exactly. This is a real, if narrower-than-ideal, form of protection: a future edit that reintroduces cursor-threading would have to change that one line, and any reviewer or future test reading the doc comment at `migrate.go:57-64` is told explicitly not to "fix" it back. I judge this **sufficient to keep criterion 4 VERIFIED** — the *behavior* the criterion cares about (partial failure survives, resume converges via re-derivation) is fully behaviorally proven by all three live subtests; only the *implementation detail* that a persisted-cursor variant would also break this is proven structurally rather than behaviorally, and the SUMMARY records this honestly rather than overclaiming.

**Criterion 5 — the causal half is deferred to Phase 4 by explicit, load-bearing design, not by oversight.**
`migrate.CurrentVersion` is pinned at 0 this phase (empty production registry, no v0→v1 step registered yet). Plan 03-05's `TestMigrateConvergesWithoutLock` therefore cannot exercise an *ordinary* write that "arrives already-current because the write path stamps `CurrentVersion`" — there is no real step chain to stamp against yet. The test substitutes an explicit `Memory.SchemaVersion: 1` on the mid-sweep write and reaches the target through `payload()`'s monotonic-max stamp logic, which the doc comment (PA-10a, `migrate_converge_test.go:49-73`) states precisely: what IS proven here is the backlog filter's strict exclusion of an already-target record AND that the sweep converges under a live concurrent writer with no lock, both through the real production `Store.Upsert`/`payload()` path (not a raw-payload injection) — conditional on the stamp==target invariant `TestEveryFullWriteMethodStampsSchemaVersion` (Phase 2, confirmed to exist and pass) pins; what is explicitly deferred, and marked `// PHASE4:` as **blocking** for Phase 4, is the literal causal proof using an ordinary `Memory` with no `SchemaVersion` set and `Target` resolving through the real constant. This is the honest and correct scope for what is achievable with an empty registry — the roadmap's Phase 3/4 split (documented in REQUIREMENTS.md's "Phase 3 note") anticipated this: Phase 3 owns the mechanism, Phase 4 owns registering the first real step. I judge criterion 5 **VERIFIED for what Phase 3 can prove**, with the causal completion correctly scheduled as a Phase 4 blocking obligation rather than silently dropped.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/migrate/step.go` | ApplyFunc, sealed Reversibility + constructors, Step, NewStep | VERIFIED | Read in full; matches must_haves exactly (nil checks, sealed interface, unexported fields). |
| `internal/migrate/registry.go` | Empty package-level `Registry`, `Validate`, `StepsFrom` | VERIFIED | `Registry = []Step{}` at package scope with `// PHASE4:` marker naming D-03; `Validate` and `StepsFrom` both present and tested. |
| `internal/migrate/additive.go` | `AddedKeys`, `RemovedKeys`, `CheckAdditive` | VERIFIED | Two-direction key-set diff, set-equality (not subset/superset) on the added-vs-declared half. |
| `internal/migrate/decoder.go` | Optional `Decoder` interface | VERIFIED | Present; `Step` does not implement it; door proven open via test. |
| `internal/store/migrate.go` | `Store.Migrate` sweep | VERIFIED | Re-derive-every-pass loop, `Offset: nil`, apply-time `CheckAdditive`, per-point `SetPayload`, non-shrinking-backlog termination guard, `int(target)` cast at the Qdrant boundary. |
| `internal/store/migratebacklog.go` | `backlogFilter`, `versionOf`, `payloadToMap` | VERIFIED | Nested `Must:[FilterAsCondition(Should:[Range,IsEmpty])]` shape matching `activeWindowConditions` precedent; 7-variant type switch in `payloadToMap`. |
| `internal/store/migrate_faultinject_test.go` | Fault-injecting interceptor + 3-scenario proof | VERIFIED | Ran live against real Qdrant; all 3 subtests pass. |
| `internal/store/migrate_converge_test.go` | Mid-sweep-write interceptor + convergence proof | VERIFIED | Ran live against real Qdrant; all 3 subtests pass. |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `internal/store/migrate.go` | `internal/migrate` (Validate, CheckAdditive, StepsFrom, AddedKeys) | direct import and calls | WIRED — confirmed by read and by passing tests exercising each call site |
| `internal/migrate/additive_test.go` | `internal/migrate/additive.go` (`CheckAdditive`) | fixture table drives production function | WIRED — confirmed live test run |
| `internal/store/migrate_faultinject_test.go` | `internal/store/migrate.go` | dial-time gRPC interceptor on the real client | WIRED — confirmed live test run, production code path exercised byte-identically |
| `internal/store/migrate_converge_test.go` | `internal/store/store.go` (`Upsert`/`payload()`) | mid-sweep write goes through the real production write path | WIRED — confirmed by read (`writerStore.Upsert(...)`) and live test run |

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| REQ-migration-step-registry | 03-01, 03-02 | SATISFIED | Stdlib-only leaf confirmed via `go list -deps`; `Validate` invariant present and tested. |
| REQ-migration-additive-only-gated | 03-01, 03-02, 03-03 | SATISFIED | `CheckAdditive` enforced at apply time in `Store.Migrate`; 8-row fixture table with non-vacuity guards. |
| REQ-migration-step-reversibility | 03-01, 03-02 | SATISFIED | Sealed `Reversibility`, mandatory declaration, panics on nil/empty. |
| REQ-migrate-partial-failure-resume | 03-04 | SATISFIED | `TestMigratePartialFailureResume` — live, passing, 3 scenarios. |
| REQ-migrate-converges-without-lock | 03-01, 03-05 | SATISFIED (scope-appropriate for Phase 3; causal completion correctly deferred to Phase 4 per REQUIREMENTS.md's documented Phase 3/4 split) | `TestMigrateConvergesWithoutLock` — live, passing, 3 subtests; PA-10a documents the deferred causal half. |

No orphaned requirements: all 5 phase requirement IDs given match REQUIREMENTS.md's Phase 3 traceability table exactly, and REQUIREMENTS.md lists no additional Phase 3 requirement beyond these 5.

### Anti-Patterns Found

None. `rg -n 'TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER'` across all 15 phase-touched files in `internal/migrate` and `internal/store/migrate*.go` returned zero matches. No stub returns, no hardcoded-empty data flowing to production paths, no console.log-only implementations.

### Code Review Cross-Reference

`03-REVIEW.md` (2026-08-14, standard depth, 14 files, status clean, 0 critical / 0 warning / 2 info) independently reproduced several of this verification's findings via live mutation testing (e.g., copying `internal/migrate` to a scratch module and mutating `registry.go` to confirm the placement gate fires correctly). The 2 info-level findings (IN-01: registry-placement gate scoped to a hardcoded filename; IN-02: sealed-interface test probe's temp dir location) are both non-blocking, cosmetic/maintenance notes, not load-bearing to any success criterion — I concur with that classification independently.

### Human Verification Required

None. All 5 success criteria are proven by live tests against a real, pinned Qdrant instance, run directly in this verification session (not merely claimed by SUMMARY.md), plus direct source reading of every enforcement mechanism named in the must-haves.

### Gaps Summary

No gaps. All 5 ROADMAP success criteria are verified against the actual codebase: the registry is a genuinely stdlib-only leaf (mechanically confirmed via `go list -deps`, not merely by reading a doc comment), both the additive-only and reversibility invariants are enforced by code that panics/errors rather than by convention, and both sweep-survivability behavioral criteria (4 and 5) were exercised live against a real Qdrant server in this session and passed. The two documented open items (structural-only proof of the no-persisted-cursor property for criterion 4; the causal half of criterion 5 deferred to a blocking Phase 4 obligation) are both honestly recorded in the shipped code's own comments and in the SUMMARY files, are correctly scoped to what an empty-registry Phase 3 can prove, and do not constitute unmet criteria — they are documented, load-bearing scope boundaries consistent with the milestone's own Phase 3/Phase 4 split.

---

_Verified: 2026-08-14T15:10:00Z_
_Verifier: Claude (gsd-verifier)_
