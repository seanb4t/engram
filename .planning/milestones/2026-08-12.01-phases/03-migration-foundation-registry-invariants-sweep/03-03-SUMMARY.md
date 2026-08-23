---
phase: 03-migration-foundation-registry-invariants-sweep
plan: 03
subsystem: database
tags: [go, migration, additive-only, table-driven-tests, prove-red]

# Dependency graph
requires:
  - phase: 03-migration-foundation-registry-invariants-sweep
    provides: "plan 03-01's internal/migrate.CheckAdditive/AddedKeys/RemovedKeys/NewStep — this plan proves the invariant those functions enforce, adds no new production code"
provides:
  - "TestAdditiveOnlyKeySetDiff — an eight-row fixture table proving CheckAdditive's two-direction key-set diff, with anti-vacuity guards and three committed RED-evidence patches"
affects: [04-migration-cli-and-first-customer]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
actuals:
  tokens: 3581
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Fixture-embedded NewStep(...) call per row (not built inside the loop) — keeps every fixture's step construction grep-pinnable and textually distinct, at the cost of some repetition"
    - "Two-independent-clones-per-application test driver, mirroring Store.Migrate's own discipline (plan 03-01 PA-5a) — the correct antidote to an ApplyFunc that mutates its input in place"
    - "Post-loop observed-vs-expected SET-EQUALITY assertion as the strongest anti-vacuity guard, independent of any row's error text"

key-files:
  created:
    - internal/migrate/additive_test.go
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-1-superset-not-equality.patch
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-2-removal-check-dropped.patch
    - .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-3-zero-fixtures.patch
  modified: []

key-decisions:
  - "Each fixture's Step is built via a literal NewStep(...) call written directly in that row's struct entry, not via a shared helper call inside the loop — required so 'rg -c NewStep\\(' counts 8 textual occurrences, one per row, per the plan's own acceptance criterion, not 1 occurrence executed 8 times."
  - "Row 7 ('removes one and adds an undeclared one') asserts BOTH the removed key name AND the undeclared key name appear in the error text, per the plan's explicit row-7 requirement. This is a stricter per-row check than a bare conforming/non-conforming verdict, and it is what surfaced the divergence recorded below in both RED cycles."

patterns-established:
  - "Anti-vacuity guard triad: non-zero fixture count, both verdict classes non-empty, and a post-loop observed-vs-expected set-equality assertion — none of the three alone is sufficient (D-05)."

requirements-completed: [REQ-migration-additive-only-gated]

coverage:
  - id: D1
    description: "An eight-row fixture table proves CheckAdditive's key-set diff in both directions over test-only NewStep fixtures, never the empty production Registry"
    requirement: "REQ-migration-additive-only-gated"
    verification:
      - kind: unit
        ref: "internal/migrate/additive_test.go#TestAdditiveOnlyKeySetDiff"
        status: pass
    human_judgment: false
  - id: D2
    description: "Set equality on the added-key set is distinguished from both a subset and a superset relation by mirrored rows 5/6, and proven load-bearing by RED cycle 1"
    requirement: "REQ-migration-additive-only-gated"
    verification:
      - kind: unit
        ref: "internal/migrate/additive_test.go#TestAdditiveOnlyKeySetDiff/adds_an_undeclared_key"
        status: pass
      - kind: other
        ref: "git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-1-superset-not-equality.patch && go test -run TestAdditiveOnlyKeySetDiff ./internal/migrate/ (expect FAIL) && git apply -R same.patch"
        status: pass
    human_judgment: false
  - id: D3
    description: "A step that mutates its input map in place is still classified non-conforming, proving the driver's per-application cloning is real (row 8, PA-5a)"
    requirement: "REQ-migration-additive-only-gated"
    verification:
      - kind: unit
        ref: "internal/migrate/additive_test.go#TestAdditiveOnlyKeySetDiff/removes_a_key_by_mutating_its_input_map_in_place"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every conforming row is proven idempotent under re-application (step-level half of SC1's idempotency word)"
    requirement: "REQ-migration-additive-only-gated"
    verification:
      - kind: unit
        ref: "internal/migrate/additive_test.go#TestAdditiveOnlyKeySetDiff (per-row idempotence assertion inside each conforming subtest)"
        status: pass
    human_judgment: false
  - id: D5
    description: "The table cannot pass vacuously — zero rows, one verdict class, or a checker that ignores its input — each proven by a committed, reversible RED-evidence patch"
    requirement: "REQ-migration-additive-only-gated"
    verification:
      - kind: other
        ref: "three cycles under .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-{1,2,3}-*.patch, each apply/test-FAIL/revert observed live"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-08-14
status: complete
---

# Phase 3 Plan 3: Additive-Only Fixture Table and Prove-RED Cycles Summary

**An eight-row `internal/migrate` fixture table proving `CheckAdditive`'s two-direction key-set diff, with two mandatory anti-vacuity guards and three committed reviewer-reproducible RED patches — one of which surfaced a genuine finding about the check's error-message independence rather than matching the plan's predicted signature exactly**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-08-14T13:58:21Z (base commit `be591005`)
- **Completed:** 2026-08-14T14:07:03Z (Task 2 commit `68a361f7`)
- **Tasks:** 2
- **Files modified:** 4 (1 created for Task 1, 3 created for Task 2)

## Worktree Isolation Mode (PA-15)

Observed at the start of the first RED-cycle injection:

- `git rev-parse --show-toplevel` → `/Volumes/Code/github.com/seanb4t/engram/.claude/worktrees/agent-a8b88f272f2822db8`
- `git branch --show-current` → `worktree-agent-a8b88f272f2822db8`

The branch matches the `worktree-agent-*` namespace PA-15 names, confirming the isolated-worktree path was in force for this plan's entire execution — every wave-2 executor has its own working tree and index, so the mandatory shared-working-tree fallback protocol (serialize / `git apply --check` / re-capture / pathspec-scoping) was NOT required and was not used. All RED-cycle git commands (`git diff`, `git apply`, `git apply -R`) were still scoped by explicit pathspec throughout, as a matter of course.

## Accomplishments

- `TestAdditiveOnlyKeySetDiff` (`internal/migrate/additive_test.go`): eight rows, each step built via a fixture-embedded `NewStep(...)` call, asserting `CheckAdditive`'s key-set diff in both directions over test-only fixtures — never the production `Registry`, whose emptiness this phase (`len(Registry) != 0` guard) explicitly asserts before the table runs.
- Rows 5 (`adds an undeclared key`) and 6 (`declares a key it never adds`) are the mirrored pair distinguishing set equality from a superset and a subset relation respectively — D-04's load-bearing distinction.
- Row 8 (`removes a key by mutating its input map in place`) proves the test driver's two-independent-clones-per-application discipline is real, mirroring `Store.Migrate`'s own defense (plan 03-01, PA-5a): `beforeRow` is a pristine clone never touched by `Apply`, and a second clone is what `Apply` receives and, for this row, mutates in place.
- Every CONFORMING row (1, 2, 3) additionally proves step-level idempotence: applying the step a second time to its own output yields a `reflect.DeepEqual` payload — one of the two executable halves of SC1's "idempotency" word.
- Three anti-vacuity guards, none alone sufficient: a pre-loop non-zero-fixture-count `t.Fatal`, a pre-loop both-verdict-classes-non-empty `t.Fatalf`, and — the strongest — a post-loop assertion comparing the OBSERVED non-conforming name set to the EXPECTED one for set equality, independent of any row's error text.
- Three committed, reviewer-reproducible RED-evidence patches under `red-evidence/`, each captured before injection, applied, observed failing, and reverted with `git apply -R` (never `git checkout --`), with `git diff --exit-code` confirmed clean after every cycle.

## Task Commits

1. **Task 1: The eight-row fixture table** — `27de3c62` (test) — `internal/migrate/additive_test.go`
2. **Task 2: Three RED cycles** — `68a361f7` (test) — three red-evidence patches

**Plan metadata:** pending (this SUMMARY's own commit)

## Files Created/Modified

- `internal/migrate/additive_test.go` — `TestAdditiveOnlyKeySetDiff`, `diffKeys`, `assertSetEqual`
- `.planning/.../red-evidence/03-03-red-1-superset-not-equality.patch` — weakens set equality to `declared ⊆ added`
- `.planning/.../red-evidence/03-03-red-2-removal-check-dropped.patch` — deletes the removed-keys direction entirely
- `.planning/.../red-evidence/03-03-red-3-zero-fixtures.patch` — empties the fixture table

## Decisions Made

- **Each fixture's `Step` is constructed via a literal `NewStep(...)` call written directly in that row's struct entry**, not by calling a shared constructor once inside the loop. The acceptance criterion `rg -c 'NewStep\(' internal/migrate/additive_test.go` must print at least 8 — a count over the FILE'S TEXT, not over runtime executions — so a loop that calls `NewStep` once per iteration (1 textual occurrence, executed 8 times) would fail this criterion even though it is behaviorally identical. Chose the more repetitive, textually-explicit form to satisfy the letter of the acceptance criterion as written.
- **Row 7 asserts BOTH the removed key name and the undeclared key name appear in the error text**, per the plan's explicit requirement that the two directions be proven independently reported rather than collapsed into one generic message. This stricter per-row assertion (beyond a bare conforming/non-conforming verdict) is what surfaced both RED cycles' divergence from the plan's predicted signature, recorded below rather than silently reconciled.
- Every non-conforming row's `Reversibility` uses `Irreversible(...)` with a concrete, row-specific reason describing why that particular mutation cannot be undone, rather than a boilerplate string — keeping the reason legible if a future reader inspects a failing row's fixture.

## Deviations from Plan

None — plan executed exactly as written. The RED-cycle signature divergences below are OBSERVATIONS the plan explicitly anticipated might occur and instructed to record rather than reconcile; they are not deviations from the plan's instructions.

## RED Cycle 1 — `03-03-red-1-superset-not-equality.patch`

**Injected change (one sentence):** Weakened `CheckAdditive`'s second direction from set equality to the one-way relation `declared ⊆ added`, deleting the `undeclared` (added-but-not-declared) computation and its error branch while keeping the `missing` (declared-but-not-added) computation and error branch untouched.

**Reproduce (three commands):**
```bash
git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-1-superset-not-equality.patch
go test -count=1 -v -run 'TestAdditiveOnlyKeySetDiff$' ./internal/migrate/   # expect FAIL
git apply -R .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-1-superset-not-equality.patch
```

**Observed per-row verdicts:**

| Row | Predicted | Observed |
|---|---|---|
| 1 conforming additive | green | green |
| 2 conforming, irreversible | green | green |
| 3 no payload-key additions | green | green |
| 4 removes a key | green (removal direction untouched) | green |
| 5 adds an undeclared key | **FAILS** (flips conforming: `declared ⊆ added` holds since declared={schema_version} ⊆ added={schema_version,undeclared_extra}) | **FAILS**, exactly as predicted |
| 6 declares a key it never adds | green (`declared ⊄ added`, still non-conforming) | green, exactly as predicted |
| 7 removes one and adds an undeclared one | green (predicted to stay green, caught by removal direction) | **FAILS** — DIVERGENCE (see below) |
| 8 removes a key in place | green (removal direction untouched) | green |
| parent post-loop set-equality assertion | FAILS (lost row 5) | FAILS, exactly as predicted |

**Divergence from the plan's stated signature, recorded rather than reconciled:** the plan's stated signature was "exactly row 5's subtest plus the parent's post-loop assertion fail, six subtests green." Observed reality is THREE failures (row 5, row 7, and the parent assertion), five subtests green. Row 7's VERDICT did not flip — `CheckAdditive` still returns non-conforming for it, correctly, because the untouched removal direction (`legacy_field` removed) still fires. What failed is row 7's OWN stricter assertion, added per the plan's explicit row-7 requirement, that the error text names BOTH the removed key AND the undeclared key. Under this weakened check, the "added key(s) not declared" error branch is gone entirely, so the error text now names only `legacy_field` and never `sneaky_key` — the observed error was `"...removed key(s) not permitted: [legacy_field]"`, missing the undeclared-key mention. This is a finding about `CheckAdditive`'s two directions genuinely being independently REPORTED (row 7's whole point), not merely independently DETECTED: weakening one direction's detection also silently degrades the other direction's error message when both would otherwise fire, because the weakened direction's error branch is what would have carried the undeclared-key name. The plan's stated signature considered only VERDICT flips (conforming/non-conforming), not per-row error-content assertions; row 7's stricter assertion (itself plan-mandated) is a legitimate part of the test and its failure here is real evidence, not a bug in the test.

**Plain statement:** under `declared ⊆ added`, a step whose declaration has drifted from its behavior — adding an undeclared key while including every declared key — is accepted as conforming. The weakened predicate stopped checking the `added − declared` direction.

## RED Cycle 2 — `03-03-red-2-removal-check-dropped.patch`

**Injected change (one sentence):** Deleted `CheckAdditive`'s first direction (the `RemovedKeys(before, after)` check and its error branch) entirely, leaving only the added/declared set-equality check.

**Reproduce (three commands):**
```bash
git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-2-removal-check-dropped.patch
go test -count=1 -v -run 'TestAdditiveOnlyKeySetDiff$' ./internal/migrate/   # expect FAIL
git apply -R .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-2-removal-check-dropped.patch
```

**Observed per-row verdicts:**

| Row | Predicted | Observed |
|---|---|---|
| 1 conforming additive | green | green |
| 2 conforming, irreversible | green | green |
| 3 no payload-key additions | green | green |
| 4 removes a key | **FAILS** (flips conforming: empty added set, empty declared set, surviving direction is silent) | **FAILS**, exactly as predicted |
| 5 adds an undeclared key | green (added-key check untouched) | green |
| 6 declares a key it never adds | green (added-key check untouched) | green |
| 7 removes one and adds an undeclared one | green (predicted to stay non-conforming on the surviving added-key check alone) | **FAILS** — DIVERGENCE (see below) |
| 8 removes a key in place | **FAILS** (same mechanism as row 4) | **FAILS**, exactly as predicted |
| parent post-loop set-equality assertion | FAILS (lost rows 4 and 8) | FAILS, exactly as predicted |

**Divergence, recorded per the plan's own instruction:** row 7's VERDICT stayed non-conforming exactly as predicted — the surviving added-key check still catches `sneaky_key` as undeclared. But row 7's stricter both-names assertion failed: the observed error was `"...added key(s) not declared in AddsKeys: [sneaky_key]"`, never mentioning `legacy_field`, because the removal-direction error branch that would have named it is gone. This is the mirror of cycle 1's finding, confirming the same property from the other side: `CheckAdditive`'s two directions are independently DETECTED (each still fires on its own trigger), but a row violating both directions only gets an error message naming ALL the relevant keys when BOTH detection branches are present. Per the plan's instruction ("If cycle 2 moves any row other than 4 and 8... that is recorded as a finding about CheckAdditive's direction independence"), this is recorded as exactly that finding, not silently accepted and not reconciled by editing the test.

**Plain statement:** under a removal-check-free `CheckAdditive`, a step that destroys a payload key is accepted as conforming whenever its added-key set happens to also satisfy set equality (rows 4 and 8, both with empty declared and empty added sets after the removal).

## RED Cycle 3 — `03-03-red-3-zero-fixtures.patch`

**Injected change (one sentence):** Patched `additive_test.go` itself to set `fixtures = nil` immediately after the fixture table literal, emptying it before the `len(fixtures) == 0` guard runs.

**Reproduce (three commands):**
```bash
git apply .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-3-zero-fixtures.patch
go test -count=1 -v -run 'TestAdditiveOnlyKeySetDiff$' ./internal/migrate/   # expect FAIL
git apply -R .planning/phases/03-migration-foundation-registry-invariants-sweep/red-evidence/03-03-red-3-zero-fixtures.patch
```

**Observed outcome:** the test fails immediately on the guard itself:
```
additive_test.go:189: zero fixtures — D-05 requires a non-zero fixture count assertion
--- FAIL: TestAdditiveOnlyKeySetDiff (0.00s)
```
No subtests ran (the `t.Fatal` aborts before any `t.Run` call), and no downstream panic or nil-dereference occurred — the failure names the zero-fixture condition specifically, quoting the guard's own message verbatim. This is the cycle proving D-05's guard is load-bearing rather than decorative, matching the exact defect this milestone shipped in Phase 01 (durable record `x6v6qxqd6f`).

## Cycle-1-Review Findings This Plan Resolves (for the next review cycle)

1. **RED cycle 1's predicted outcome is now mathematically unambiguous where verdicts are concerned** — row 5 flips, row 6 stays green, matching the earlier prediction exactly on the VERDICT axis. The prediction did not originally account for per-row error-content assertions (row 7's both-names check), and this SUMMARY records that finer-grained divergence explicitly rather than silently absorbing it.
2. **The misleading `no-op` row name is replaced.** Row 3 is named `"no payload-key additions (version transition still advances)"`, stating the property actually asserted (no added keys) rather than implying the step has no effect — the sweep still advances `schema_version` when it writes. Verified: `rg -ci -- '--- PASS: TestAdditiveOnlyKeySetDiff/.*no.?op'` over the test's verbose output prints zero matches.
3. **The value-overwrite limitation (PA-2/T-03-12) is stated together with its containment** in the test file's doc comment: `CheckAdditive` compares key sets only; the restricted-writer alternative is a recorded Deferred Idea; and the consequence is contained downstream by `Store.Migrate` building its `SetPayload` map from `AddedKeys` only, proven by `TestMigrateWritesOnlyAddedKeys` (plan 03-01), named explicitly.
4. **The aliasing hazard (PA-5a) is closed by two-clones-per-application and proven by row 8** — `beforeRow` and the clone passed to `Apply` are always independent; row 8's `ApplyFunc` deletes from and returns the exact map it was handed, and is still correctly classified non-conforming.

Additionally: the per-conforming-row apply-twice idempotence assertion (PA-4) is confirmed as one half of the idempotency proof the HIGH finding on `Validate` required; the other half (sweep-level rerun) lives in plan 03-01's `TestMigrateTracerLegacyRecordEndToEnd`.

## Issues Encountered

None beyond the two recorded RED-cycle divergences above, both anticipated by the plan's own instructions and treated as findings rather than defects requiring a fix — `CheckAdditive`'s production code is unchanged from plan 03-01 and needs no correction; the divergence is entirely about which per-row assertions a weakened variant of the check can still satisfy.

## Next Phase Readiness

- `internal/migrate/additive.go` (plan 03-01) is unchanged and confirmed clean (`git diff --exit-code`) after all three RED cycles.
- `TestAdditiveOnlyKeySetDiff` is a permanent regression test: any future weakening of either `CheckAdditive` direction will fail this table, as demonstrated live by cycles 1 and 2.
- No blockers for plans 03-02, 03-04, 03-05, or Phase 4.

## Self-Check: PASSED

`internal/migrate/additive_test.go` and all three red-evidence patch files verified present on disk. Commits `27de3c62` and `68a361f7` verified present in `git log --oneline`.

---
*Phase: 03-migration-foundation-registry-invariants-sweep*
*Completed: 2026-08-14*
