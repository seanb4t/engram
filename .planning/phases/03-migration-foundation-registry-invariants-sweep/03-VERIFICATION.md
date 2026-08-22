---
phase: 03-migration-foundation-registry-invariants-sweep
verified: 2026-08-14T15:35:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 5/5
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 3: Migration Foundation — Registry Invariants & Sweep Verification Report

**Phase Goal:** A pure, dependency-free migration-step registry exists, and no step can be
registered without declaring both additive-only compliance and its reversibility — enforced
structurally, not caught in review. The sweep that drives it survives Qdrant's real batch
non-atomicity and converges without a collection lock.

**Verified:** 2026-08-14T15:35:00Z
**Status:** passed
**Re-verification:** Yes — triggered by `verification.status` reporting the prior 2c6fb84c
VERIFICATION.md as STALE, because `03-05-SUMMARY.md` was edited (commit `bd42b362`) after
`03-VERIFICATION.md` was committed. That edit rewrote two elided red-evidence file paths in
the SUMMARY's "Files Created/Modified" list to their full form so the artifact-integrity
checker could resolve them (`git show bd42b362` — a 2-line diff, no code, no claims, no test
content touched).

## Method — what was re-derived live vs. carried forward

**Independently re-confirmed live in this session** (not inherited from the prior report):

1. `git diff --name-only 2c6fb84c..HEAD -- . ':(exclude).planning'` returns empty — no source
   file changed since the prior verification. Only two `.planning/` docs commits exist since
   (`bd42b362` path-string fix, `ec8cb149` ROADMAP/STATE completion marker).
2. `go list -deps ./internal/migrate | rg -v "^$(go list -m)(/|$)" | rg '^[^/]+\.[^/]+/' | wc -l`
   → `0` (corrected leaf-purity check, per known-open-item #3). Full stdlib-only dependency
   list read in full — no `qdrant`/`auth`/other-module import present.
3. `rg -n '"github.com/.*internal/store' internal/migrate/*.go` → zero hits (migrate never
   imports store); `rg -n '"github.com/.*internal/migrate' internal/store/*.go` → 10 hits
   (store imports migrate, one-way).
4. Read `internal/migrate/step.go`, `registry.go`, `additive.go` in full, directly — not
   quoted from the prior report.
5. `go build ./...` — clean, zero output.
6. `task lint` — `0 issues` from golangci-lint; all other lint stages (yamlfmt, actionlint,
   rumdl, ruff) passed. (One stale golangci-lint cache warning referenced a deleted worktree
   path unrelated to this phase — not a finding.)
7. `go test -count=1 -v -run 'TestMigratePartialFailureResume$' ./internal/store/` — ran live
   against a real `qdrant/qdrant:v1.18.2` testcontainer (Docker confirmed available via
   `docker info`), all 3 subtests **PASS**.
8. `go test -count=1 -v -run 'TestMigrateConvergesWithoutLock$' ./internal/store/` — ran live
   against a real Qdrant testcontainer, all 3 subtests **PASS** (including the trigger-fired
   assertion: `sync.Once` fired exactly once across 3 matching scroll passes).
9. `go test -count=1 -run 'TestNewStepPanicsOnNilReversibility|TestReversiblePanicsOnNilInverse|TestIrreversiblePanicsOnEmptyReason|TestReversibilityIsSealedToThisPackage' -v ./internal/migrate/...`
   — all pass, including the out-of-package build-failure probe subtest.
10. `rg -n 'Offset:' internal/store/*.go` (excluding `_test.go`) — exactly one
    `Offset: nil` in `migrate.go:189`; the other `Offset: offset` hits are in unrelated files
    (`summarize.go`, `spine.go`, `store.go`) that are not part of the migration sweep.
11. `rg -n 'TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER' internal/migrate/*.go internal/store/migrate*.go`
    → zero matches.
12. Cross-checked `.planning/REQUIREMENTS.md` traceability table against the 5 phase
    requirement IDs (see Requirements Coverage below) — found a real, unresolved
    documentation gap (see Gaps Summary / Requirements Coverage note).

**Carried forward as unchanged-by-inspection** (no code changed, so re-deriving from scratch
would reproduce identical output; spot-checked rather than re-typed from the prior report):
the `03-REVIEW.md` clean code-review finding (0 critical / 0 warning / 2 info, independently
reproduced via live mutation testing per its own method section) and the artifact inventory
table, since `go build`/`go test`/`go vet` all passing on unchanged files is direct evidence
those artifacts still exist and compile.

## Goal Achievement

### Observable Truths / Success Criteria (from ROADMAP.md)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `internal/migrate` is a stdlib-only leaf, imported one-way by `internal/store`, with a single `Validate` invariant over ordering/idempotency-precondition | ✓ VERIFIED | Re-run leaf-purity check (corrected form) = 0; full `go list -deps` output read, stdlib-only. Import-direction greps confirm one-way (`store→migrate`, 10 hits; `migrate→store`, 0 hits). `registry.go:63` `Validate` checks transition-uniqueness (both From and To), advance (`To > From`), and contiguity, with its own doc comment correctly disclaiming it alone proves idempotence (that's proven behaviorally elsewhere, per the comment). |
| 2 | A non-additive step fails to build or fails a test, not a review catch; the step interface leaves room for a per-version decoder | ✓ VERIFIED | `additive.go:CheckAdditive` (read in full) diffs key sets in both directions: `RemovedKeys` rejects any removal, and `AddedKeys` vs. `s.AddsKeys()` is a true set-equality check (both "undeclared-added" and "declared-but-missing" are separately reported) — not a subset/superset check. `internal/migrate/decoder.go:25` defines an optional `Decoder` interface that `Step` does not implement; a "door open" test (`TestDecoderDoorIsOpenAndUnclaimed`, present in the file list) proves this reachable-by-embedding for Phase 4. |
| 3 | A step silent about reversibility fails the same way — "nobody thought about it" is unrepresentable | ✓ VERIFIED | `step.go:NewStep` (read in full) panics on `rev == nil` and `apply == nil`. `Reversibility` is a sealed interface via an unexported `isReversibility()` marker, satisfiable only by this package's two unexported types; `Reversible(nil)` and `Irreversible("")` both panic explicitly. Re-ran `TestNewStepPanicsOnNilReversibility`, `TestReversiblePanicsOnNilInverse`, `TestIrreversiblePanicsOnEmptyReason`, `TestReversibilityIsSealedToThisPackage` (3 subtests, including the out-of-package build-failure probe) live — all PASS. |
| 4 | `Store.Migrate` survives a forced mid-sequence partial `SetPayload` failure against a real pinned Qdrant, then a resume converges the backlog to zero via re-derivation, never trusting the write call's own signal | ✓ VERIFIED | `TestMigratePartialFailureResume` re-run live in this session against a real `qdrant/qdrant:v1.18.2` testcontainer: all 3 subtests PASS — (a) single dropped write self-heals; (b) persistent failure terminates on the non-shrinking-backlog guard, and a resume through a second independent Store converges; (c) "the error lies" scenario (server commits, client reports error) proves re-derivation discipline, not trust in the write call's signal. See Notes for the one documented, structural-only sub-property. |
| 5 | The sweep runs with no collection lock: writes that arrive already-current are never re-processed, proven by a test that writes mid-sweep and confirms non-reprocessing | ✓ VERIFIED | `TestMigrateConvergesWithoutLock` re-run live in this session against a real Qdrant testcontainer: all 3 subtests PASS — an already-current mid-sweep write (via the real `Store.Upsert`/`payload()` production write path) is never selected and never gains the step's marker key; a below-target mid-sweep write IS migrated (positive control); the sweep converges to backlog=0 with the mid-sweep trigger proven to have fired via a real integer counter inside a `sync.Once`. No lock/mutex anywhere in `Store.Migrate` (confirmed by read). See Notes for the causal-half scope boundary, explicitly deferred to Phase 4. |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified)

### Notes on Criteria 4 and 5 (documented, not gaps — carried forward, independently re-confirmed)

**Criterion 4 — the no-persisted-cursor property is protected structurally, not behaviorally.**
`Store.Migrate` never persists a scroll cursor across passes (D-07): `Offset: nil` on every
call. Re-confirmed live: `rg -n 'Offset:' internal/store/*.go` (excluding tests) shows exactly
one `Offset: nil` in `migrate.go:189` and zero cursor-threading shapes in that file — matching
plan 03-01's pinned acceptance criterion. The behavioral half of criterion 4 (partial failure
survives, resume converges via re-derivation) IS fully behaviorally proven by the 3 live
subtests re-run above; only the narrower claim that a persisted-cursor variant would also
break this is proven structurally rather than by a reddened test (plan 03-04's RED-evidence
cycle 1 found the naive mutation didn't reliably red the target subtest, because Qdrant's own
scroll-exhaustion behavior incidentally recovers a skipped record before the guard fires).

**Criterion 5 — the causal half is deferred to Phase 4 by explicit, load-bearing design.**
`migrate.CurrentVersion` is pinned at 0 in this phase (empty `Registry`, confirmed by direct
read of `registry.go:30`). With no real step chain, `TestMigrateConvergesWithoutLock` cannot
exercise an *ordinary* write that "arrives already-current because the write path stamps
CurrentVersion" — it substitutes an explicit `SchemaVersion: 1` write through the real
`payload()` stamp logic. What IS proven (re-confirmed live): the backlog filter strictly
excludes an already-target record, and the sweep converges under a live concurrent writer with
no lock. What remains deferred: the literal causal proof using an ordinary `Memory` with no
explicit version set, once a real step chain exists. This is marked `// PHASE4:` in
`migrate_converge_test.go` (re-confirmed present, 2 occurrences) as a blocking Phase 4
obligation — not a silently dropped requirement.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/migrate/step.go` | ApplyFunc, sealed Reversibility + constructors, Step, NewStep | ✓ VERIFIED | Read in full this session; nil checks, sealed interface, unexported fields all present and match the phase goal. |
| `internal/migrate/registry.go` | Empty package-level `Registry`, `Validate`, `StepsFrom` | ✓ VERIFIED | Read in full this session; `Registry = []Step{}` at package scope with `// PHASE4:` marker; `Validate` (3-rule, `errors.Join`) and `StepsFrom` (bounded-search sub-chain selection) both present. |
| `internal/migrate/additive.go` | `AddedKeys`, `RemovedKeys`, `CheckAdditive` | ✓ VERIFIED | Read in full this session; two-direction diff, true set-equality on added-vs-declared. |
| `internal/migrate/decoder.go` | Optional `Decoder` interface | ✓ VERIFIED | `type Decoder interface` present at line 25; `Step` does not implement it (confirmed via grep — no `Decode` method on `Step`). |
| `internal/store/migrate.go` | `Store.Migrate` sweep | ✓ VERIFIED | Imports `internal/migrate`; `Offset: nil` at line 189 (re-derive-every-pass, no persisted cursor). |
| `internal/store/migratebacklog.go` | `backlogFilter`, `versionOf`, `payloadToMap` | ✓ VERIFIED | All three symbols present (confirmed via grep at lines 13/58/71+). |
| `internal/store/migrate_faultinject_test.go` | Fault-injecting interceptor + 3-scenario proof | ✓ VERIFIED | Re-run live against real Qdrant this session; all 3 subtests pass. |
| `internal/store/migrate_converge_test.go` | Mid-sweep-write interceptor + convergence proof | ✓ VERIFIED | Re-run live against real Qdrant this session; all 3 subtests pass. |

### Key Link Verification

| From | To | Via | Status |
|------|-----|-----|--------|
| `internal/store/migrate.go` | `internal/migrate` (Validate, CheckAdditive, StepsFrom, AddedKeys) | direct import and calls | WIRED — confirmed by grep (import present) and by live-passing tests exercising each call site this session |
| `internal/store/*.go` (import direction) | `internal/migrate` never imports `internal/store` | one-way leaf dependency | WIRED — re-confirmed via grep this session: 0 hits for migrate→store, 10 hits for store→migrate |
| `internal/store/migrate_faultinject_test.go` | `internal/store/migrate.go` | dial-time gRPC interceptor on the real client | WIRED — confirmed by live test run this session against a real Qdrant testcontainer |
| `internal/store/migrate_converge_test.go` | `internal/store/store.go` (`Upsert`/`payload()`) | mid-sweep write goes through the real production write path | WIRED — confirmed by live test run this session |

### Behavioral Spot-Checks (live re-runs, this session)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Sweep survives partial-failure and resumes | `go test -run 'TestMigratePartialFailureResume$' ./internal/store/` (real Qdrant testcontainer) | 3/3 subtests PASS | ✓ PASS |
| Sweep converges with no lock, mid-sweep writes not re-processed | `go test -run 'TestMigrateConvergesWithoutLock$' ./internal/store/` (real Qdrant testcontainer) | 3/3 subtests PASS | ✓ PASS |
| Sealed reversibility / nil-check invariants | `go test -run 'TestNewStepPanicsOnNilReversibility\|TestReversiblePanicsOnNilInverse\|TestIrreversiblePanicsOnEmptyReason\|TestReversibilityIsSealedToThisPackage' ./internal/migrate/...` | 4/4 tests (7 subtests) PASS | ✓ PASS |
| Package builds | `go build ./...` | clean, zero output | ✓ PASS |
| Lint clean | `task lint` | `0 issues` (golangci-lint); all other stages pass | ✓ PASS |
| Leaf-purity gate (corrected form) | `go list -deps ./internal/migrate \| rg -v "^$(go list -m)(/\|$)" \| rg '^[^/]+\.[^/]+/' \| wc -l` | `0` | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description Match | Status | Evidence |
|-------------|-------------|--------------------|--------|----------|
| REQ-migration-step-registry | 03-01, 03-02 | Stdlib-only leaf, imported by store, `Validate` invariant | ✓ SATISFIED | Leaf-purity re-confirmed 0; `Validate` read and tested live. |
| REQ-migration-additive-only-gated | 03-01, 03-02, 03-03 | Non-additive step fails build/test, decoder door open | ✓ SATISFIED | `CheckAdditive` enforced at apply time (`internal/store/migrate.go`); decoder door confirmed present. |
| REQ-migration-step-reversibility | 03-01, 03-02 | Mandatory reversibility declaration | ✓ SATISFIED | Sealed `Reversibility`, panics on nil/empty, re-run live. |
| REQ-migrate-partial-failure-resume | 03-04 | Live Qdrant partial-failure + resume | ✓ SATISFIED (code) / ⚠️ **REQUIREMENTS.md checkbox not updated** | `TestMigratePartialFailureResume` re-run live, 3/3 pass — code-level evidence is unambiguous. **BUT** `.planning/REQUIREMENTS.md` still shows this requirement as `- [ ]` (unchecked) in both the v1 Requirements list (line 34) and the Traceability table (`Phase 3 \| Pending`, line 100) — the only checkbox among this phase's 5 requirements never flipped to `[x]`/`Complete`. |
| REQ-migrate-converges-without-lock | 03-01, 03-05 | No-lock convergence, mid-sweep writes not re-processed | ✓ SATISFIED | `TestMigrateConvergesWithoutLock` re-run live, 3/3 pass; causal half correctly deferred to Phase 4 (see Notes). |

**Requirements-tracking gap (not a code defect, but a real, verifiable inconsistency):**
`git log --follow -p -- .planning/REQUIREMENTS.md` shows the only commit that ever touched
Phase 3's requirement checkboxes is `b90e140b` ("docs(03-01): complete migration foundation
registry invariants sweep plan"), timestamped 09:56 — *before* plan 03-04 (which owns
`REQ-migrate-partial-failure-resume` and completed at 10:20) had even run. That commit checked
off the three requirements 03-01 itself satisfies plus `REQ-migrate-converges-without-lock`
(also claimed by 03-01's plan frontmatter), but never `REQ-migrate-partial-failure-resume`
(owned solely by 03-04). No later commit in this phase — including `ec8cb149` ("mark phase 3
complete in roadmap and state"), which touched only `ROADMAP.md` and `STATE.md` — ever
revisited `REQUIREMENTS.md` to flip this checkbox. The actual code and live tests fully
satisfy the requirement; this is a documentation/traceability omission in a tool-tracked file,
not a phase-goal defect. **Recommend**: flip `REQ-migrate-partial-failure-resume`'s checkbox
(line 34) and its traceability-table status (line 100, `Pending → Complete`) to keep
`REQUIREMENTS.md` an accurate milestone-completion source of truth — this is a value-only edit
to an existing tool-emitted shape (a checkbox and a status cell), not new structure, so it is
safe to fix directly.

No orphaned requirements: all 5 phase requirement IDs match REQUIREMENTS.md's Phase 3
traceability table exactly, and REQUIREMENTS.md lists no additional Phase 3 requirement beyond
these 5.

### Anti-Patterns Found

None. `rg -n 'TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER' internal/migrate/*.go internal/store/migrate*.go`
re-run this session returned zero matches. No stub returns, no hardcoded-empty data flowing to
production paths.

### Code Review Cross-Reference

`03-REVIEW.md` (2026-08-14, standard depth, 14 files, status clean, 0 critical / 0 warning / 2
info) independently reproduced several of this verification's findings via live mutation
testing. No source file it reviewed has changed since (confirmed by the empty `git diff`
above), so its findings are unchanged-by-inspection. The 2 info-level findings (IN-01:
registry-placement gate scoped to a hardcoded filename; IN-02: sealed-interface test probe's
temp dir location) remain non-blocking, cosmetic/maintenance notes.

### Human Verification Required

None. All 5 success criteria are proven by live tests re-run against a real, pinned Qdrant
instance in this verification session (not merely claimed by a prior report), plus direct
source reading of every enforcement mechanism named in the must-haves.

### Gaps Summary

No code or test gaps block the phase goal. All 5 ROADMAP success criteria remain verified
against the actual codebase, re-confirmed live rather than inherited: the registry is a
genuinely stdlib-only leaf (mechanically re-confirmed via the corrected `go list -deps` check),
both the additive-only and reversibility invariants are enforced by code that panics/errors
rather than by convention, and both sweep-survivability behavioral criteria (4 and 5) were
re-exercised live against a real Qdrant server in this session and passed. The two previously
documented open items (structural-only proof of the no-persisted-cursor property for criterion
4; the causal half of criterion 5 deferred to a blocking Phase 4 obligation) are re-confirmed
accurate and remain correctly scoped, non-blocking design boundaries.

One documentation-only gap was found during this re-verification and is **not** a phase-goal
blocker: `.planning/REQUIREMENTS.md` never had its checkbox flipped for
`REQ-migrate-partial-failure-resume`, despite the requirement being fully satisfied in code
and by a live-passing test. This should be corrected (see Requirements Coverage above) so the
milestone's tracked completion state matches reality, but it does not indicate any defect in
the migration mechanism itself.

---

_Verified: 2026-08-14T15:35:00Z_
_Verifier: Claude (gsd-verifier) — re-verification_
