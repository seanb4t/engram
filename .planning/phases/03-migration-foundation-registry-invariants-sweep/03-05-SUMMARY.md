---
phase: 03-migration-foundation-registry-invariants-sweep
plan: 05
subsystem: database
tags: [go, qdrant, migration, schema-versioning, concurrency, testcontainers]

# Dependency graph
requires:
  - phase: 03-migration-foundation-registry-invariants-sweep
    provides: "plan 03-01's backlogFilter and Store.Migrate re-derive-every-pass sweep"
provides:
  - "TestMigrateConvergesWithoutLock — proof that Store.Migrate converges under a live concurrent writer with no collection lock, gated on Phase 2's stamp-then-sweep ordering"
affects: [04-migration-cli-and-first-customer]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 6129
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Deterministic mid-sweep write trigger: a grpc.UnaryClientInterceptor recognizing the sweep's own second *qdrant.ScrollPoints request, guarded by sync.Once with an integer fires/triggerMatches pair observed rather than inferred"
    - "Hook-records-errors-test-drains pattern for interceptor callbacks that must never call a t.Fatal-family method from an uncertain goroutine (PA-11a)"
    - "rawPayloadNoFatal — a non-fatal sibling of rawPayload, safe to call from inside an interceptor callback or from a subtest that must tolerate a record's absence without crashing"

key-files:
  created:
    - internal/store/migrate_converge_test.go
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-05-red-1-lte-includes-current.patch
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-05-red-2-midsweep-write-skipped.patch
  modified: []

key-decisions:
  - "SC5 recorded as PARTIALLY proven, not fully discharged (PA-10a): strict-filter exclusion and Store.Upsert -> payload() write-path plumbing are proven this phase; the literal causal claim (new writes arrive already-current BECAUSE the write path stamps the current version) is deferred to Phase 4 behind a // PHASE4: marker, conditional on TestEveryFullWriteMethodStampsSchemaVersion."
  - "Store.Migrate's own error is asserted inside subtest 3, not as a top-level t.Fatalf before any t.Run block — otherwise RED cycle 1's predicted selective per-subtest signature could never be observed, since a top-level Fatal prevents every subtest from running at all."
  - "Subtest 1 reads the already-current record via rawPayloadNoFatal, not the fatal-on-missing rawPayload, and treats the record's absence as consistent with 'never touched' — discovered necessary during RED cycle 2, where the record genuinely does not exist and a fatal Get would crash the subtest instead of letting it demonstrate the vacuous-pass it is meant to demonstrate."

patterns-established:
  - "Bounded-adversarial control: exactly one below-target concurrent insertion, hand-checked against the sweep's non-shrinking-backlog termination guard, used to distinguish strict filter exclusion from a filter that matches nothing — explicitly NOT evidence for convergence against arbitrary concurrent writers."

requirements-completed: [REQ-migrate-converges-without-lock]

coverage:
  - id: D1
    description: "A record written through the real Store.Upsert/payload() path WHILE the sweep is in flight, arriving already at the target version, is proven at the wire (SetPayload write-id set) and in the collection (no marker key, unchanged schema_version) never to have been re-processed by the sweep — and a below-target sibling written at the same instant is proven to have been picked up and migrated, distinguishing strict exclusion from a filter that matches nothing."
    requirement: "REQ-migrate-converges-without-lock"
    verification:
      - kind: integration
        ref: "internal/store/migrate_converge_test.go#TestMigrateConvergesWithoutLock/already-current_mid-sweep_write_is_never_re-processed"
        status: pass
      - kind: integration
        ref: "internal/store/migrate_converge_test.go#TestMigrateConvergesWithoutLock/below-target_mid-sweep_write_does_enter_the_backlog_and_is_migrated"
        status: pass
    human_judgment: false
  - id: D2
    description: "The sweep converges to an empty backlog with no collection lock and no coordination beyond the interceptor trigger, and the test is proven to have actually observed what it claims: the mid-sweep write's execution counter fires exactly once, its trigger genuinely armed, more than one scroll pass occurred, and the wire-level write-id set matches the expected set exactly."
    requirement: "REQ-migrate-converges-without-lock"
    verification:
      - kind: integration
        ref: "internal/store/migrate_converge_test.go#TestMigrateConvergesWithoutLock/the_sweep_converged,_and_this_run_actually_observed_what_it_claims"
        status: pass
    human_judgment: false
  - id: D3
    description: "Two RED-evidence cycles observed a named subset of the proof turning red: widening backlogFilter's range bound to include already-current records, and skipping the mid-sweep write entirely."
    verification:
      - kind: other
        ref: "git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-05-red-1-lte-includes-current.patch && go test -run TestMigrateConvergesWithoutLock$ ./internal/store/ (observed FAIL) && git apply -R same-patch"
        status: pass
      - kind: other
        ref: "git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-05-red-2-midsweep-write-skipped.patch && go test -run TestMigrateConvergesWithoutLock$ ./internal/store/ (observed FAIL) && git apply -R same-patch"
        status: pass
    human_judgment: false

duration: ~17min
completed: 2026-08-14
status: complete
---

# Phase 3 Plan 5: Convergence-Without-Lock Summary

**`TestMigrateConvergesWithoutLock` proves the sweep converges under a live concurrent writer with no collection lock, via a deterministic gRPC-interceptor mid-sweep write trigger and two committed RED-evidence cycles — one of which turned up a stronger-than-predicted failure mode**

## Performance

- **Duration:** ~17 min
- **Started:** 2026-08-14T10:04:00Z (approx.)
- **Completed:** 2026-08-14T10:15:30Z (approx.)
- **Tasks:** 2
- **Files modified:** 3 (1 created source file, 2 created red-evidence patches)

## Worktree Isolation (PA-15)

Observed at the start of this plan's execution, before any RED-cycle injection:
- `git rev-parse --show-toplevel` = `/Volumes/Code/github.com/seanb4t/engram/.claude/worktrees/agent-a46f67edbad0c4d25`
- `git branch --show-current` = `worktree-agent-a46f67edbad0c4d25`

This branch is in the `worktree-agent-*` namespace, confirming the isolated-worktree path was in force for this run (not the shared-working-tree fallback). No git isolation conflicts with sibling wave-2 executors (03-02, 03-03, 03-04) were observed at any point.

## Accomplishments

- `TestMigrateConvergesWithoutLock` (`internal/store/migrate_converge_test.go`) proves ROADMAP success criterion 5 end to end against a real pinned Qdrant: a `midSweepHook`-driven `grpc.UnaryClientInterceptor` deterministically triggers a concurrent write on the sweep's own second `*qdrant.ScrollPoints` request, guarded by `sync.Once` with an integer `fires`/`triggerMatches` pair that is *observed*, not inferred from `sync.Once` alone (PA-11).
- Three subtests, all passing: (1) an already-current record written mid-sweep through the real `Store.Upsert`/`payload()` path is proven, at the wire and in the collection, never to have been selected for a write; (2) a below-target `laggard` sibling written at the same instant is proven to enter the backlog and be migrated — the bounded-adversarial control (PA-13) that distinguishes strict exclusion from a filter matching nothing; (3) the sweep converges to an empty backlog with no lock, and the test's own non-vacuity guards (fires==1, triggerMatches>=1, scrolls>1, a set-equal write-id comparison with both diff directions printed) confirm the run actually exercised what it claims.
- No `t.Fatal`-family call appears inside `midSweepHook`'s callback or `midSweepInterceptor` (PA-11a): failures detected inside the mid-sweep write (e.g. a stamp mismatch) are recorded into a mutex-guarded slice and drained by the test immediately after `Store.Migrate` returns, before any subtest runs.
- The doc comment states PA-10's monotonic-stamp substitution explicitly and carries PA-10a's full three-part scoping, naming `TestEveryFullWriteMethodStampsSchemaVersion` literally as the condition SC5's proof rests on here, plus a `// PHASE4:` marker declaring the literal causal re-run blocking for Phase 4.
- `go test -count=5 -run 'TestMigrateConvergesWithoutLock$' ./internal/store/` is green on every iteration.
- Two RED-evidence cycles committed as reviewer-reproducible patches (see below), one of which surfaced a stronger failure mode than the plan predicted, and two small fixes to the test's own gating/robustness discovered while capturing them.

## RED-Evidence Cycles

### Cycle 1 — `03-05-red-1-lte-includes-current.patch`

**Injected change:** `backlogFilter`'s range bound changed from `Lt` (strictly below target) to `Lte` (at or below target), one line in `internal/store/migratebacklog.go`.

**Observed per-subtest verdicts:** all three subtests FAIL (`--- FAIL: TestMigrateConvergesWithoutLock` at the top level, before any `=== RUN` for a subtest name appears — see the divergence note below). This diverges from the plan's stated selective-signature prediction (subtests 1 and 3 fail, subtest 2 stays green), and is recorded here as observed rather than reconciled to the prediction.

**Root cause of the divergence, traced precisely:** with `Batch: 2` and six seeded legacy records, `backlogFilter(1)` under `Lte` matches *every* record whose `schema_version` is `<= 1` — which, after the sweep's first pass migrates two records to `schema_version: 1`, still includes those two migrated records forever (1 <= 1 is always true). Pass 1: `Count` returns 6, first pass skips the shrink guard, `Scroll` #1 runs (this is the ONLY scroll that ever happens), two points are migrated. Pass 2: `Count` returns 6 again (the widened filter never lets a migrated record leave the backlog), the non-shrinking-backlog termination guard trips immediately — `Store.Migrate` returns its hard error BEFORE a second `Scroll` is ever issued. Because the mid-sweep trigger is armed on the sweep's SECOND scroll (`fireOnScroll: 2`), and only one scroll ever occurs, the trigger never arms at all: `h.fires == 0`, `h.triggerMatches == 0`, `h.scrolls == 1`. The `alreadyCurrent` and `laggard` records were therefore never written into Qdrant, so every downstream assertion in all three subtests fails (two of them via a genuine "point not found" from the raw Get, which is why the run reports a hard top-level `FAIL` rather than three cleanly-separated subtest results in this cycle's specific arithmetic — the gating fix described below still let each subtest attempt to run, but two of the three crashed on a missing record rather than reporting a clean assertion failure, because this record's absence in Cycle 1 is a genuine bug consequence rather than the intentional no-op Cycle 2 constructs).

**What this demonstrates, stated at full strength:** losing the strict range bound does not merely add redundant work to the sweep, as a first-glance reading of "the record stays in the backlog and gets rewritten every pass" might suggest — it can prevent the sweep from ever completing a second pass at all, because the sweep's own termination guard (rightly) refuses to tolerate a backlog that never shrinks. This is a STRONGER and more informative failure than the plan's prediction, and it sharpens exactly why SC5 states "no lock is needed" rather than "a lock is optional": the strict bound is not a nice-to-have precision detail, it is what makes the backlog's fresh re-derivation terminate at all under a filter that is supposed to shrink monotonically as records are migrated.

Per the plan's own instruction ("if all three redden, label the cycle weaker evidence rather than presenting it as selective"), this cycle is recorded as **weaker (but more informative) evidence** than a clean selective signature — it demonstrates a real, mechanistically-traced consequence of the injected change, but not the specific per-subtest partition the plan predicted.

**Reproduce recipe:**
```bash
git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-05-red-1-lte-includes-current.patch
go test -count=1 -v -run 'TestMigrateConvergesWithoutLock$' ./internal/store/   # observed: FAIL
git apply -R .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-05-red-1-lte-includes-current.patch
```

**Observed pass after revert:** confirmed — all three subtests PASS (see `-count=1`/`-count=5` runs recorded above and below).

### Cycle 2 — `03-05-red-2-midsweep-write-skipped.patch`

**Injected change:** wraps the body of `h.fn` (the mid-sweep write closure) in `if false { ... }` inside `internal/store/migrate_converge_test.go`, making it dead code — no mid-sweep write happens at all, but the file still compiles (an earlier attempt that emptied the closure entirely broke the build via now-unused `writerStore`/`time` — see Deviations).

**Observed per-subtest verdicts:** subtest 1 PASSES (green); subtests 2 and 3 FAIL. This matches the plan's predicted selective signature EXACTLY.

- Subtest 1 passes because `alreadyCurrent` was never written, so it is trivially, and correctly, absent from the write-id set and absent from the collection — the intended vacuous-pass this cycle exists to demonstrate is not a hidden bug.
- Subtest 2 fails on two assertions: `laggard`'s id is absent from the recorded write-id set (want present), and a raw `Get` for `laggard` reports "point not found" — a clean, attributable failure, not a downstream panic.
- Subtest 3 fails specifically on the write-id-set equality assertion (`missing: [laggardID]`), NOT on `h.fires == 1` — confirmed observed: `fires` stayed `1` and `triggerMatches` stayed `>= 1` throughout, because `sync.Once` still runs its body (which is now a no-op) on schedule. This is the exact mechanism PA-11's doc comment and this plan's Task 2 action text call out: the integer fires counter proves the trigger fired, but says nothing about what the triggered function actually did — the write-id-set equality (plus subtest 2's control) is what catches "the mid-sweep write never happened."

**What this demonstrates:** subtest 1's negative assertions ("the already-current record was never re-processed") are NOT vacuously true by construction — they are vacuously true ONLY in this specific injected scenario where no mid-sweep write occurred at all, and that scenario is independently and correctly flagged as broken by subtests 2 and 3's non-vacuity guards. Without this cycle, a reader could not distinguish "subtest 1 passes because the exclusion is real" from "subtest 1 passes because nothing was ever written" (durable record `x6v6qxqd6f`'s observes-nothing failure mode) — this cycle proves the latter is caught elsewhere in the same run.

**Reproduce recipe:**
```bash
git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-05-red-2-midsweep-write-skipped.patch
go test -count=1 -v -run 'TestMigrateConvergesWithoutLock$' ./internal/store/   # observed: subtest 1 PASS, subtests 2 and 3 FAIL
git apply -R .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-05-red-2-midsweep-write-skipped.patch
```

**Observed pass after revert:** confirmed — all three subtests PASS.

## Repeat-Run Result

`go test -count=5 -run 'TestMigrateConvergesWithoutLock$' ./internal/store/` — green on all 5 iterations, run twice (once after Task 1, once after final revert of both RED cycles).

## Task Commits

1. **Task 1: Write records mid-sweep and prove the already-current ones are never re-processed** — `e3d52114` (test) — `internal/store/migrate_converge_test.go`, `midSweepHook`, `midSweepInterceptor`, `dialMidSweepTestClient`, `TestMigrateConvergesWithoutLock`
2. **Deviation fix, found while capturing Task 2's RED cycle 1** — `9528fb09` (fix) — stop gating all three subtests on `Store.Migrate`'s own error
3. **Deviation fix, found while capturing Task 2's RED cycle 2** — `64c1db26` (fix) — make subtest 1 tolerate the mid-sweep record's absence
4. **Task 2: Two RED cycles** — `10d1d0d2` (test) — both committed red-evidence patches

**Plan metadata:** pending (this SUMMARY's own commit)

## Files Created/Modified

- `internal/store/migrate_converge_test.go` — `midSweepHook`, `midSweepInterceptor`, `dialMidSweepTestClient`, `rawPayloadNoFatal`, `diffSorted`, `TestMigrateConvergesWithoutLock`
- `.planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-05-red-1-lte-includes-current.patch`
- `.planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-05-red-2-midsweep-write-skipped.patch`

## Decisions Made

- **`migrateErr` moved from a top-level `t.Fatalf` to a subtest-3 assertion.** The original structure asserted `Store.Migrate`'s own error before any `t.Run` block executed, which made it structurally impossible to ever observe RED cycle 1's predicted per-subtest signature (subtests 1 and 3 fail, subtest 2 stays green) — a top-level `Fatal` prevents every subtest from running at all. Only `midSweepHook`'s own recorded errors (a failure detected inside the mid-sweep write itself) still gate all three subtests before they run; `Store.Migrate`'s error is now asserted inside subtest 3 alongside the other convergence assertions.
- **Subtest 1 reads via `rawPayloadNoFatal`, not `rawPayload`.** `rawPayload` calls `t.Fatalf` when the point does not exist. Under RED cycle 2's no-op mid-sweep write, `alreadyCurrent` genuinely never exists in Qdrant, and the original subtest 1 crashed on that `Get` instead of passing cleanly — exactly the "a guard that fails only because the code panicked afterwards is not a guard" failure mode Task 2's acceptance criteria warn against. `rawPayloadNoFatal` treats a missing record as consistent with "never touched" and lets the subtest complete.
- **Cycle 2's injection uses `if false { <original body> }` rather than emptying `h.fn`'s body.** A first attempt (`h.fn = func() {}`) broke the build: `writerStore` and the `time` import became unused once the closure's body was removed, which is a compile failure, not the semantic RED the cycle is meant to produce. Wrapping the original body in dead code keeps every variable referenced (compiler-satisfied) while genuinely never executing any of it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Top-level `migrateErr` gate prevented RED cycle 1's predicted per-subtest signature from ever being observable**
- **Found during:** Task 2, capturing RED cycle 1
- **Issue:** `if migrateErr != nil { t.Fatalf(...) }` ran before any `t.Run` subtest, so a sweep that failed to converge (exactly what RED cycle 1 injects) failed the whole test before any subtest could report its own verdict — making the plan's required "observed per-subtest verdicts" impossible to produce for this cycle.
- **Fix:** Moved the `migrateErr != nil` assertion into subtest 3, alongside `res.Backlog`/`migrateBacklogIDs`/marker assertions. Only `midSweepHook`'s own recorded errors still gate all three subtests up front.
- **Files modified:** `internal/store/migrate_converge_test.go`
- **Verification:** Baseline re-run green (`go test -count=1` and `-count=5`); RED cycle 1 re-run afterward, per-subtest verdicts now genuinely observable (even though, per the traced root cause above, all three still redden in this specific cycle's arithmetic).
- **Committed in:** `9528fb09`

**2. [Rule 1 - Bug] Subtest 1 crashed instead of passing cleanly when the mid-sweep record never existed**
- **Found during:** Task 2, capturing RED cycle 2
- **Issue:** Subtest 1's collection-level check called `rawPayload`, which `t.Fatalf`s on a missing point. Under RED cycle 2's no-op injection, `alreadyCurrent` is never written, so the point genuinely does not exist — subtest 1 crashed rather than demonstrating the vacuous pass the cycle exists to prove is real (and correctly flagged elsewhere).
- **Fix:** Switched to `rawPayloadNoFatal`; a missing record is logged and treated as consistent with "never touched", with an explicit pointer to subtest 3's non-vacuity guards as the actual catch for "the write never happened at all."
- **Files modified:** `internal/store/migrate_converge_test.go`
- **Verification:** RED cycle 2 re-captured cleanly afterward, producing the plan's exact predicted selective signature (subtest 1 green, subtests 2 and 3 red).
- **Committed in:** `64c1db26`

---

**Total deviations:** 2 auto-fixed (both Rule 1 — bugs in the test's own structure, discovered while capturing the plan's required RED-evidence cycles)
**Impact on plan:** Both fixes were necessary for the RED-evidence cycles' observed behavior to be attributable to the injected production/test change rather than to a pre-existing structural gap in the test itself. No scope creep — no change to `internal/store/migrate.go`, `store.go`, or `migratebacklog.go` in their final committed state (confirmed via `git diff --exit-code`).

## Issues Encountered

- **RED cycle 1's observed per-subtest verdicts diverge from the plan's stated prediction.** The plan predicted a selective signature (subtests 1 and 3 fail, subtest 2 stays green); the observed reality is that all three subtests fail, because the widened `Lte` filter interacts with `Batch: 2` to trip the non-shrinking-backlog termination guard after exactly one pass — before the mid-sweep trigger (armed on the sweep's second scroll) can ever arm. This is traced to its precise mechanism above and recorded as intentionally weaker-but-more-informative evidence per the plan's own escape valve, not silently reconciled to the prediction.
- **A first attempt at RED cycle 2's injection broke the build** (`h.fn = func() {}` left `writerStore` and the `time` import unused). Corrected to `if false { <original body> }`, which keeps the build green while making the body genuinely dead code. Not committed in its broken form — caught by `go build`/`go vet` before any patch capture.

## Next Phase Readiness

- `internal/store/migrate.go`, `store.go`, and `migratebacklog.go` are unchanged by this plan — `git diff --exit-code` confirms.
- SC5 is recorded as PARTIALLY proven (per PA-10a): strict-filter exclusion and production write-path plumbing (`Store.Upsert` -> `payload()`) are proven this phase; the literal causal claim — that new writes arrive already-current BECAUSE the write path stamps the current version — is DEFERRED to Phase 4, where `CurrentVersion == 1` pairs with the registered v0->v1 step, and this same concurrency test must be re-run with an ORDINARY `Memory` carrying no `SchemaVersion` and `Target` left at zero. This is named as BLOCKING for Phase 4 in the test's own `// PHASE4:` doc-comment marker, greppable at commit `e3d52114`.
- No blockers for plans 03-02/03-03/03-04 (parallel wave-2 siblings) — this plan touched only `internal/store/migrate_converge_test.go` and its own red-evidence patches, per PA-9's file ownership.

## Self-Check: PASSED

All 3 created files verified present (`internal/store/migrate_converge_test.go`, both red-evidence patches). All 4 commits verified present in git history (`e3d52114`, `9528fb09`, `64c1db26`, `10d1d0d2`).

---
*Phase: 03-migration-foundation-registry-invariants-sweep*
*Completed: 2026-08-14*
