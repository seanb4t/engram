---
phase: 03-migration-foundation-registry-invariants-sweep
plan: 04
subsystem: database
tags: [go, qdrant, migration, fault-injection, grpc-interceptor, testcontainers]

# Dependency graph
requires:
  - phase: 03-migration-foundation-registry-invariants-sweep
    provides: "plan 03-01's Store.Migrate re-derive-every-pass sweep, backlogFilter, and migrate_test.go helpers (seedLegacyRecord, markerStep, migrateBacklogIDs, hasPayloadKey, payloadToMap)"
provides:
  - "setPayloadFaultInjector / faultMode / dialFaultInjectingTestClient — a gRPC unary interceptor extending the schemaversion_recallgate_test.go seam, failing/observing SetPayload writes before or after they reach the server"
  - "TestMigratePartialFailureResume's three-scenario proof that Store.Migrate survives cross-call partial progress and unreliable error signals (SC4/D-09) against a real pinned Qdrant"
  - "Two committed RED-evidence patches for the sweep's re-derivation discipline"
affects: [04-migration-cli-and-first-customer]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 6000
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "gRPC unary interceptor as a fault-injecting AND pure-recording seam: mode=faultNone (the zero value) with failFrom=0 makes any setPayloadFaultInjector a safe, side-effect-free write observer by construction, so the same type serves both the injecting client and a disarmed resume-side recorder"
    - "Deriving a record's identity from CAPTURED WIRE TRAFFIC (the injector's own recorded point ids), never from fixture insertion order, when Qdrant's scroll order is not a contract"
    - "A second, independent observer wired to a SEPARATE client for a resumed operation, so 'the resume's writes were actually observed' is asserted rather than assumed"

key-files:
  created:
    - internal/store/migrate_faultinject_test.go
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-04-red-1-persisted-cursor.patch
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-04-red-2-trust-error-signal.patch
  modified: []

key-decisions:
  - "Cycle 1 (persisted cursor) reddened ZERO of the three subtests against the plan's own committed fixtures — diverges from the plan's predicted 'scenario 1 red, scenario 3 green' signature. Recorded as observed, not reconciled by editing the test (see Issues Encountered for the confirmed mechanism)."
  - "Cycle 2 (trust the write error) reddened ALL THREE subtests, since every scenario injects at least one write failure and the patched code now aborts on the first one it sees. Per the plan's own acceptance criteria this is labelled weaker (less selective) evidence, though the underlying mechanism — every scenario proves D-09's re-derivation discipline — is coherent across all three."

patterns-established:
  - "setPayloadFaultInjector{failFrom, failCount, mode} armed via arm(from, count, mode)/disarm() — ordinal-based, 1-indexed across ALL writes the injector observes, count==0 meaning unbounded"
  - "assertSortedIDSetEqual — a reusable two-direction sorted-set diff helper, printing both extra and missing ids on failure, for id-set assertions beyond the inline pattern migrate_test.go already established"

requirements-completed: [REQ-migrate-partial-failure-resume]

coverage:
  - id: D1
    description: "A single mid-sequence SetPayload write that never reaches the server self-heals in a later pass, with per-record rawPayload evidence (not just a count) showing the specific dropped record recovered"
    requirement: "REQ-migrate-partial-failure-resume"
    verification:
      - kind: integration
        ref: "internal/store/migrate_faultinject_test.go#TestMigratePartialFailureResume/single_mid-sequence_failure_self-heals_in_a_later_pass"
        status: pass
    human_judgment: false
  - id: D2
    description: "A persistent write failure terminates on the non-shrinking-backlog guard (naming both counts), and a resume through a SECOND *Store with its OWN never-armed recorder converges, with the resume's writes proven OBSERVED and set-equal to the outstanding records (not merely 'not the succeeded one')"
    requirement: "REQ-migrate-partial-failure-resume"
    verification:
      - kind: integration
        ref: "internal/store/migrate_faultinject_test.go#TestMigratePartialFailureResume/persistent_failure_terminates,_and_the_resume_converges"
        status: pass
    human_judgment: false
  - id: D3
    description: "A write that commits at the server while its call reports failure (semantic simulation of qdrant/qdrant#9371) still converges the sweep, with the sweep's OWN counters asserted WRONG (Migrated=0, Failed=4) and the re-derived collection asserted RIGHT (Backlog=0, all four records carry the marker)"
    requirement: "REQ-migrate-partial-failure-resume"
    verification:
      - kind: integration
        ref: "internal/store/migrate_faultinject_test.go#TestMigratePartialFailureResume/the_error_lies_and_the_sweep_converges_anyway"
        status: pass
    human_judgment: false
  - id: D4
    description: "A persisted cursor and a trusted write-error signal have each been exercised as committed, reproducible RED-evidence patches against the shipped test — with the OBSERVED (not predicted) per-subtest verdicts recorded honestly"
    requirement: "REQ-migrate-partial-failure-resume"
    verification:
      - kind: other
        ref: "git apply .planning/.../red-evidence/03-04-red-1-persisted-cursor.patch && go test -run TestMigratePartialFailureResume ./internal/store/ && git apply -R .planning/.../red-evidence/03-04-red-1-persisted-cursor.patch (and the -2 patch identically)"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-08-14
status: complete
---

# Phase 3 Plan 4: Migration Partial-Failure Fault Injection Summary

**A gRPC unary interceptor (`setPayloadFaultInjector`) proves `Store.Migrate` survives cross-call partial write progress and a write that commits while its own call reports failure — three scenarios against a real pinned Qdrant, plus two committed RED-evidence patches whose observed verdicts diverged from the plan's predictions and are recorded as observed.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-08-14T14:04:00Z (approx.)
- **Completed:** 2026-08-14T14:17:07Z
- **Tasks:** 2
- **Files modified:** 3 (all created)

## Worktree Isolation Mode (PA-15)

Observed at the start of Task 1, before any injection:

```
git rev-parse --show-toplevel -> /Volumes/Code/github.com/seanb4t/engram/.claude/worktrees/agent-a2c533984e18c4a59
git branch --show-current      -> worktree-agent-a2c533984e18c4a59
```

Branch matches the `worktree-agent-*` namespace — the isolated-worktree path was in force, not the shared-working-tree fallback. This executor's captured hunks on `internal/store/migrate.go` ran in a private working tree and index, so no collision was possible with plan 03-05's hunks on `internal/store/migratebacklog.go`.

## Accomplishments

- `setPayloadFaultInjector` / `faultMode` / `dialFaultInjectingTestClient` / `setPayloadFaultInterceptor` extend the `dialCapturingTestClient` seam plan 02-03 built (`schemaversion_recallgate_test.go`), reusing its exact skip/`requireQdrant`/host-port-parsing boilerplate. An injector left disarmed (`mode: faultNone`, `failFrom: 0`, the zero value) is a pure recorder by construction — the same type serves both the failing client and a resume-side observer.
- `TestMigratePartialFailureResume`'s three subtests, all passing against a real pinned `qdrant/qdrant:v1.18.2` testcontainer, on the first run before any RED-evidence injection:
  1. **Self-heal** — a single mid-sequence write drop (fail-before-invoke) recovers in a later pass; `res.Failed==1`, backlog converges to empty, and per-record `rawPayload` reads confirm the specific dropped record now carries the marker.
  2. **Persistent failure + resume** — an unbounded fail-before-invoke run terminates on the non-shrinking-backlog guard (`res.Migrated==1`, error names both backlog counts). `succeededID` is derived from the FIRST injector-recorded write id (never fixture insertion order). The resume runs through a SECOND `*Store` on a separate client carrying its OWN never-armed `resumeInj`; its writes are asserted `seen() > 0` and set-equal to the outstanding ids — a replay of the already-migrated record or an unobserved resume both fail this assertion.
  3. **Lying error** — every write commits (fail-after-invoke) while every call reports failure; `res.Migrated==0`, `res.Failed==4` (the write signals are wrong) while `res.Backlog==0` and every record carries the marker (the collection is right) — the D-09 proof.
- Two RED-evidence cycles, both committed as reviewer-reproducible patches, with OBSERVED (not merely predicted) per-subtest verdicts recorded — see Deviations below for the substantial finding in Cycle 1.
- SC4's evidence, worded per PA-2/review-cycle-1: per-point `SetPayload` writing eliminates the within-call multi-ID partiality class BY CONSTRUCTION (no test here reproduces it); cross-call partial progress and unreliable error signals are what these three scenarios prove.

## Task Commits

1. **Task 1: The SetPayload fault injector and the three-scenario partial-failure/resume proof** — `4659764c` (test) — `migrate_faultinject_test.go`
2. **Task 2: Two RED cycles — persisted cursor, trust the error** — `7e08cf04` (test) — both red-evidence patches

**Plan metadata:** pending (this SUMMARY's own commit)

## Files Created/Modified

- `internal/store/migrate_faultinject_test.go` — `setPayloadFaultInjector`, `faultMode`, `setPayloadFaultInterceptor`, `dialFaultInjectingTestClient`, `selectedPointID`, `assertSortedIDSetEqual`, `TestMigratePartialFailureResume` (3 subtests)
- `.planning/.../red-evidence/03-04-red-1-persisted-cursor.patch` — threads `ScrollAndOffset`'s cursor across outer passes (BackfillShortIDs's shape)
- `.planning/.../red-evidence/03-04-red-2-trust-error-signal.patch` — replaces the record-and-continue branch with an immediate `return res, err` on any `SetPayload` error

## Decisions Made

- **`assertSortedIDSetEqual` added as a reusable helper** rather than inlining the diff-printing logic three times (scenario 2's backlog derivation, and its resume-id-set assertion) — the same discipline `TestBacklogFilterMatchesAbsentAndBelowTarget` (migrate_test.go) applies inline, factored out here because this file needs it twice with identical shape.
- **`succeededID` derivation reads `inj.ids()[0]`**, the FIRST recorded write across the whole run (ordinal 1, the only write never armed to fail in scenario 2) — never `seededIDs[0]`, per PA-16/review-cycle-1's explicit prohibition on trusting Qdrant's scroll order.
- **No production code changed.** `git diff --exit-code -- internal/store/store.go` and `internal/store/migrate.go` both succeed at plan end — this plan is test-only, matching its `<files>` manifest.

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written for Task 1's three scenarios (all passed on the first run, no auto-fixes needed).

### Task 2 — Observed RED-evidence verdicts diverge from the plan's stated predictions

Both cycles were captured, applied, run, and reverted per the plan's exact recipe. Neither cycle's OBSERVED per-subtest verdict matched the plan's stated prediction; per the plan's own instruction ("Observed verdicts are recorded as observed... the difference is written down, not reconciled by editing the test"), both are recorded here rather than forced to match.

**Cycle 1 — `03-04-red-1-persisted-cursor.patch` (persisted cursor).**

- **Injected change:** threads `ScrollAndOffset`'s `next_page_offset` across `Store.Migrate`'s outer passes — `var offset *qdrant.PointId` declared outside the loop, passed as `Offset` on the scroll, and set to `next` at the end of each pass — the exact shape `BackfillShortIDs` already uses (`store.go:2792-2795`), which is why this is named in the plan as the most plausible future regression in this file.
- **Expected selective signature (per plan):** scenario 1 (self-heal) RED, scenario 3 (lying error) GREEN, scenario 2 unconstrained.
- **OBSERVED:** all three subtests stayed GREEN. `go test -count=1 -v -run 'TestMigratePartialFailureResume$' ./internal/store/` reported `--- PASS` for all three, both with the patch applied and (as expected) after reverting it.
- **Confirmed mechanism (via a throwaway, uncommitted diagnostic, per the 03-01-SUMMARY.md precedent — never committed, deleted before finalizing this patch):** `Store.Migrate`'s OUTER re-derivation loop calls `ScrollAndOffset` on every pass. Once the persisted cursor scrolls past the true end of the collection, Qdrant returns `next_page_offset == nil` — signaling "no more pages," not "no more backlog." Because the patched code does `offset = next` unconditionally, that `nil` resets the cursor, and the VERY NEXT pass restarts scanning from the beginning of the collection with `Offset: nil` — incidentally re-including any record the persisted cursor had skipped over. In the plan's own 6-record/`Batch:3` fixture (self-heal scenario), this reset happens on pass 3, one pass before the non-shrinking-backlog guard would otherwise fire, so the dropped record (`ordinal 2` of the run) is quietly recovered and the whole call still converges. Debug-traced live: pass 1 scrolls records 1–3 (record 2's write dropped, `next` = record 4's id); pass 2 scrolls records 4–6 with `Offset` = that id — CONFIRMING record 2 is skipped despite still matching the backlog filter (`cnt=4` at pass-2 entry but only 3 records returned) — and `next == nil` because records 4–6 exhaust the 6-record collection; pass 3 then scrolls with `Offset: nil` (the reset) and finds record 2, whose write now succeeds (ordinal well past the armed range). A second, larger, non-evenly-dividing throwaway fixture (10 records, `Batch:3`, same single mid-sequence drop) showed the identical self-healing wraparound — `Migrated:10 Failed:1 Passes:6 Backlog:0`, `err:<nil>` — confirming this is not an artifact of the specific 6/3 ratio.
- **Why scenario 2 (persistent failure) also stayed green, mechanistically:** its unbounded failure means the backlog count PLATEAUS (stops shrinking) on the very next pass after the failures begin — well before the cursor could ever reach the end-of-collection reset — so the non-shrinking-backlog guard fires on the FIRST `Migrate` call for essentially the same reason it would without the bug (the guard's error text and `res.Migrated` count are unaffected by cursor threading in this fixture shape). The RESUME (a second `Migrate` call through a fresh `*Store`) starts its own `offset` local variable at `nil` regardless of the first call's internal state, so it converges normally either way.
- **What this proves, honestly:** the injected defect IS real and independently confirmed (a record whose write failed sits behind the persisted cursor and is skipped by at least one full pass, exactly as the plan states) — but this plan's specific committed fixtures do not happen to exercise a collection/batch shape where that skip survives to the non-shrinking-backlog guard before an incidental wraparound recovers it. This diverges from "reddens all three" (the plan's own stated weaker-evidence caveat) in the opposite direction — it reddens NONE — which is recorded here as an even weaker outcome than the plan anticipated, not reconciled by editing `migrate_faultinject_test.go`.

**Cycle 2 — `03-04-red-2-trust-error-signal.patch` (trust the write error).**

- **Injected change:** replaced the record-and-continue branch (`lastWriteErr = werr; res.Failed++; continue`) with an immediate `err = werr; return res, err` on any `SetPayload` error.
- **Expected (per plan):** scenario 3 (lying error) FAILS; scenario 1 "expected to move as well."
- **OBSERVED:** all three subtests FAILED. `single mid-sequence failure...`: `Migrate` returned the injected fail-before-invoke error instead of nil (the sweep now aborts on the FIRST write failure it sees, rather than continuing to the next record). `persistent failure terminates...`: the FIRST call's error is now the raw injected gRPC error rather than the non-shrinking-backlog guard's message (`"does not name the non-shrinking-backlog guard"`). `the error lies...`: `Migrate` returned the injected fail-after-invoke error instead of nil, exactly as the plan predicted for this subtest.
- **What this proves:** the write the patched code believes in scenario 3 is provably a lie — the write committed at the server (fail-AFTER-invoke), yet the patched sweep aborts as though nothing landed. All three subtests reddening (not a selective scenario-3-only signature) is a direct, mechanistic consequence of every one of the three scenarios injecting at least one write failure somewhere in its run: the record-and-continue branch this patch removes is exactly what lets scenarios 1 and 2 also converge/terminate correctly despite their own injected failures. Per the plan's acceptance criteria ("a cycle that reddened all three subtests is recorded as such and labelled weaker evidence"), this cycle is labelled weaker/less-selective evidence — though the underlying reddening reason is coherent across all three (D-09's re-derivation discipline is what each subtest was actually relying on).

Both patches apply cleanly (`git apply --check`), reverse cleanly (`git apply --check -R`), and leave `internal/store/migrate.go` byte-identical to its pre-cycle state (`git diff --exit-code -- internal/store/migrate.go` succeeds). After both cycles, `go test -count=1 -v -run 'TestMigratePartialFailureResume$' ./internal/store/ | rg -c -- '    --- PASS: TestMigratePartialFailureResume/'` prints `3`.

---

**Total deviations:** 0 auto-fixed; 2 RED-evidence cycles with observed verdicts diverging from stated predictions (both recorded honestly per the plan's own instruction, not treated as auto-fixable deviations under Rules 1-3).
**Impact on plan:** No production code changed. Both patches remain valid, reproducible reviewer artifacts regardless of whether their observed signature matches the original prediction — the plan explicitly anticipates and requires recording exactly this kind of divergence.

## Issues Encountered

**Cycle 1's persisted-cursor regression is masked by an incidental end-of-collection wraparound**, fully described under Deviations above. This is not a defect in the fault-injection test or the interceptor — it is a genuine property of layering a persisted-cursor "regression" underneath `Store.Migrate`'s OUTER re-derivation loop: because that loop calls `ScrollAndOffset` repeatedly rather than once, a `nil` `next_page_offset` (end of collection) resets the persisted cursor on the very next pass, giving the buggy code an unplanned "free" fresh restart before the non-shrinking-backlog guard can fire. A future reviewer wanting a fixture shape that reliably exposes this class of regression under the guard would need either a much larger collection (many multiples of `Batch`) or a batch size that never lets a single pass exhaust the remaining collection while a skipped record is still pending — outside this plan's scope to construct, and not requested by any acceptance criterion (which asks only that the OBSERVED verdict be recorded, which it is).

## Next Phase Readiness

- `Store.Migrate`'s partial-failure/resume behavior (REQ-migrate-partial-failure-resume) is now proven against a real pinned Qdrant with both self-heal and lying-error scenarios, and the resume-is-just-call-it-again property (D-07) is proven with an independently observing second client (PA-16).
- Plan 03-05 (convergence-without-lock) builds on the same `Store.Migrate`/`backlogFilter` surface with no further changes needed here; this plan touched no files 03-05 owns (`internal/store/migratebacklog.go`).
- Phase 4 (Migration CLI & First Customer) can rely on `Store.Migrate`'s error/counter semantics as proven here: a non-nil error names the non-shrinking-backlog guard's two counts; `MigrateResult.Migrated`/`Failed` are telemetry-only and must never gate CLI control flow, only `Backlog`.
- No blockers.

## Self-Check: PASSED

All 3 created files verified present (`internal/store/migrate_faultinject_test.go`, both red-evidence patches). Both task commits verified present in git history (`4659764c`, `7e08cf04`). `go test -count=1 ./internal/store/... ./internal/migrate/...` is fully green after both plans' work.

---
*Phase: 03-migration-foundation-registry-invariants-sweep*
*Completed: 2026-08-14*
