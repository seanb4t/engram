---
phase: 04
reviewers: [codex, opencode]
reviewed_at: 2026-08-14T21:16:15Z
plans_reviewed:
  - 04-01-PLAN.md — Tracer: v0→v1 short_id first customer
  - 04-02-PLAN.md — Store MigrateStatus histogram + Store.Revert + startup warning
  - 04-03-PLAN.md — CLI surface: migrate command family
  - 04-04-PLAN.md — backfill-short-ids as thin delegating alias
cycle: 3
plans_revision: 50667cbf
cycle_1_findings:
  high: [H1, H2, H3, H4, H5, H6]
  actionable: [M1, M2, M3, M4, M5, M6, M7, M8, M9, M10, M11]
cycle_2_verdict:
  resolved_high: [H1, H2, H3, H4]
  partial_high: [H6]
  unresolved_high: [H5]
  actionable_unresolved: [M8]
  new_high: [H7, H8]
  new_actionable: [M12, M13]
cycle_3_verdict:
  resolved_high: [H5, H7, H8]
  partial_high: [H6]
  unresolved_high: [M8]
  actionable_unresolved: [M3, M4, N1, N3, N4, N5, N6]
  new_high: []
  new_actionable: [N1, N3, N4, N5, N6]
  current_high_count: 2
  current_actionable_count: 7
  converges: false
---

# Cross-AI Plan Review — Phase 4 (Cycle 3)

Reviewers: Codex (`gpt-5.1-codex`, xhigh) and OpenCode. Both had full repo file access and
verified plan claims against shipped source. Plans reviewed are the revisions landed in
`50667cbf` ("docs(04): revise phase plans per cycle-2 review findings").

## Consensus Summary

The cycle-2 revisions closed most of the ledger. **H5 (manifest bridge), H7 (PA-3 vs appeared
records), H8 (unbounded operations), M12 and M13 are RESOLVED** — both reviewers independently
verified each fix against the shipped code and agreed. The manifest bridge now mirrors the purge
precedent (preview called *inside* the apply closure), the manifest-limited apply is a single
non-looping pass that never reaches the PA-3 guard, `--timeout` is preserved on the alias, and
both failing test assertions are explicitly rewritten.

The cycle **does not converge**. Two HIGH-severity items remain, and both reviewers found them
independently:

1. **M8 escalates to HIGH — `Store.PreviewRevert` / `store.RevertPlan` are required but defined
   by no plan.** The cycle-2 fix relocated the defect rather than closing it: the CLI no longer
   calls an unexported helper, it now calls an *exported* method that no task creates. Wave 3
   execution halts on a compile error.
2. **The revert unsupported-version preflight is batch-scoped, not whole-operation.** The
   must_have promises "refuses the entire operation with zero records touched", but the specified
   loop preflights only the current scroll batch and then writes it, so an unsupported record
   beyond the first 256-record batch is discovered after earlier records were already mutated.

### Agreed Strengths

- H5's manifest bridge is now structurally identical to the shipped purge precedent
  (`cmd/engram/spine_review_purge.go:365,373`), with a `! rg migrateLastPreviewManifest` verify
  gate pinning the old design's absence.
- H7's single-pass manifest-limited apply correctly bypasses the re-derivation loop
  (`internal/store/migrate.go:141-178`), so the PA-3 guard at `:167-178` has no second pass to
  compare; `Backlog == Appeared > 0` is asserted rather than the impossible `Backlog == 0`.
- H8's core defect is closed: `--timeout` is preserved on the alias (`cmd/engram/backfill.go:21,80`)
  and a 5-minute deadline helper mirrors `spinePurgeWithTimeout` (`spine_review_purge.go:330`).
- `StepsFrom(steps, to, from)` is pinned correctly against the forward-only contract
  (`internal/migrate/registry.go:92,102-127`), with both a presence and an absence grep gate.
- M12's `mutatingCommandNames()` derivation from `!ReadOnly` correctly repairs both directions of
  `TestDestructiveCommandsRequireApply` (`cmd/engram/destructive_test.go:88-106`).
- M13's replacement of the `len(Registry) == 0` fatal (`internal/migrate/additive_test.go:40-42`)
  with `>= 1` plus a `From()==0 && To()==1` pin is correct.

### Agreed Concerns

- **M8 (HIGH, 04-02/04-03).** 04-03's `migrateFamilyStore` interface requires
  `PreviewRevert(ctx, to) (store.RevertPlan, error)` and attributes it to "04-02's M8 resolution",
  but 04-02 specifies only unexported `reversePreflight`, `preflightRecordVersionSupport`,
  `Revert`, and `revertWithSteps`. Neither `PreviewRevert` nor `RevertPlan` appears in any plan's
  `files_modified` or task action; 04-03 does not list `internal/store/revert.go` as modified.
- **Batch-scoped revert preflight (HIGH per Codex, MEDIUM per OpenCode; same mechanism).**
  04-02 Task 2 preflights "ALL records in the current scroll batch" and then processes that batch,
  reusing `Store.Migrate`'s one-batch-per-pass shape (`internal/store/migrate.go:185-196`). The
  planned test seeds only two records, so both land in the first batch and the defect stays
  invisible. Needs a full paginated preflight before any mutation, plus a test that places the
  unsupported record beyond the first page.
- **`--timeout` flags are registered but never read (MEDIUM).** `migrateWithTimeout` is specified
  as a fixed 5-minute `context.WithTimeout` taking no duration, while `migrateTimeout` and the
  preserved `backfillTimeout` are separately registered. This contradicts the shipped convention,
  which reads the flag-backed variable and treats zero as "disable"
  (`cmd/engram/spine_review_purge.go:107`), and makes 04-03's planned `--timeout 1s` test
  unpassable. The composition is never stated.
- **M4 (MEDIUM).** Future-version records are separated but collapsed into a single scalar
  `FutureVersion uint64`. The per-version distribution is discarded and the must_have's promised
  "version annotation" is not represented by the specified struct.
- **M3 (MEDIUM, partial).** The key-set diff cannot detect keys whose *value* changed in place —
  the plan documents this honestly, which clears the actionable bar for that half. The other half
  does not: there is still no reconciliation protocol or forced-mid-sequence test for a
  `DeletePayload` that succeeds followed by a `SetPayload` that fails, despite the shipped
  `TestMigratePartialFailureResume` precedent.

### Divergent Views

- **H6 status.** Codex holds H6 at PARTIAL and rates the batch-local preflight HIGH; OpenCode
  marks H6's *spec* RESOLVED (the `StepsFrom` order is pinned and correct) and files the same
  preflight hole as a separate MEDIUM (N2). They agree entirely on the mechanism and the fix —
  they disagree only on whether it is a residual H6 or a new finding, and on severity. Recorded
  here at the higher severity.
- **M3.** Codex reads the contradictory value-change prose as failing the actionable bar
  (UNRESOLVED); OpenCode reads it as an accepted-by-documentation disposition (PARTIAL). The
  unreconciled multi-RPC partial failure is unresolved on both readings.
- **Alias apply parity (Codex only).** Codex flags that `backfill-short-ids --apply` calls
  `Store.Migrate` with `DryRun:false` directly, bypassing the DryRun→manifest intersection the
  canonical `engram migrate --apply` path uses — so the "thin identical alias" claim does not
  hold for the apply path. OpenCode did not raise this.
- **Sibling conformance gates (OpenCode only).** OpenCode found that widening the `--apply` gate
  updates only `TestDestructiveCommandsRequireApply`, leaving `TestDestructiveCommandsRouteThroughGate`
  (`destructive_test.go:130-152`) and `TestDestructiveCommandsExactFlagSet` (`:~240-260`) still
  iterating `destructiveCommandNames()`. These do not fail — the safety net silently shrinks as
  the gate widens. Codex did not raise this.

---

## Codex Review

# Phase 4 Plan Review — Cycle 3

## 1. Ledger adjudication

| Item | Status | Evidence and mechanism |
|---|---|---|
| **H5** | **RESOLVED** | Plan 04-03 now explicitly performs `Store.Migrate(DryRun:true)` inside the apply closure and immediately passes its manifest to `Store.Migrate(Manifest: ...)` ([04-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-03-PLAN.md:196)). This matches the shipped purge pattern, where `PreviewPurge` is called inside `spinePurgeApplyRun` before `ApplyPurge` ([spine_review_purge.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:339)). No package-level manifest bridge remains in the plan. |
| **H6** | **PARTIAL** | The exact chain construction is now pinned correctly: `StepsFrom(steps, to, from)`, followed by reversal ([04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:157)); this matches the actual forward-only `StepsFrom` contract ([registry.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/registry.go:92)). However, unsupported-version “whole-operation” preflight only checks the **current scroll batch**, then begins writes ([04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:173)). An unsupported record in a later batch can therefore be discovered after earlier records were reverted. The promised zero-write whole-operation preflight is still not achieved. |
| **H7** | **RESOLVED** | Manifest-limited apply is now explicitly a single full-backlog pass, followed by a fresh count, and does not enter the PA-3 convergence loop ([04-01-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-01-PLAN.md:160)). `Backlog` includes appeared records and tests assert it remains nonzero ([04-01-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-01-PLAN.md:202)). That avoids the current PA-3 guard, which rejects a non-shrinking second pass ([migrate.go](/Volumes/Code/github.com/seanb4t/engram/internal/store/migrate.go:156)). |
| **H8** | **RESOLVED** | The revised plans preserve `--timeout` on the alias and add a five-minute deadline to migrate/revert operations ([04-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-03-PLAN.md:181), [04-04-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-04-PLAN.md:99)). Thus operations are no longer unbounded; this closes H8’s core concern. There is a new flag-wiring defect below. |
| **M3** | **UNRESOLVED** | The plan says value-changed keys become `SetPayload` ([04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:175)), but then explicitly admits its key-set diff does **not detect changed values** ([04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:176)). The existing `AddedKeys`/`RemovedKeys` helpers only compare key presence ([additive.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive.go:12)). The plan also still gives no reconciliation protocol for `DeletePayload` succeeding and the subsequent `SetPayload` failing. This actionable finding is visible in PLAN.md, but only as a contradictory description—not as a task that fixes it—so it does not clear the bar. |
| **M4** | **UNRESOLVED** | Future versions are collapsed into one `FutureVersion uint64`, and their original buckets are deliberately removed ([04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:101), [04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:119)). A collection containing v2 and v42 therefore reports only a combined count; the requested per-version distribution/annotation remains discarded. The tests reinforce that aggregation. This actionable item is mentioned but not actually remedied, so it does not clear the bar. |
| **M8** | **UNRESOLVED** | Plan 04-03 requires exported `Store.PreviewRevert` in its interface ([04-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-03-PLAN.md:166)), but Plan 04-02—the plan owning `internal/store/revert.go`—only specifies unexported `reversePreflight`, `Revert`, and `revertWithSteps` ([04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:155)). Its artifact list likewise omits `PreviewRevert`. Plan 04-03 does not list `internal/store/revert.go` among modified files. The interface therefore depends on a method no task creates. This actionable item remains mechanically unexecutable. |
| **M12** | **RESOLVED** | Plan 04-03 explicitly replaces the `Destructive:true` set with `mutatingCommandNames()` derived from `!ReadOnly` in `TestDestructiveCommandsRequireApply` ([04-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-03-PLAN.md:122)). This addresses both directions of the current assertion ([destructive_test.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/destructive_test.go:84)). |
| **M13** | **RESOLVED** | Plan 04-01 explicitly replaces the current empty-registry assertion with v0→v1 registry assertions ([04-01-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-01-PLAN.md:128)). That directly updates the assertion currently at [additive_test.go](/Volumes/Code/github.com/seanb4t/engram/internal/migrate/additive_test.go:39). |

## 2. New concerns

### HIGH — “Whole-operation” revert preflight is only batch-local

Plan 04-02 says it checks every record in the “current scroll batch” and then processes that batch ([04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:173)). With more records than `Batch`, the sequence is:

1. Preflight batch 1.
2. Write batch 1.
3. Re-derive/scroll again.
4. Discover unsupported v42 in a later batch.
5. Return an error after writes already occurred.

The test only seeds one supported and one unsupported record ([04-02-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-02-PLAN.md:191)), so both will normally be in the first batch and the defect stays invisible. The plan needs a complete paginated preflight phase before any mutation, with a test placing the unsupported record beyond the first page.

### MEDIUM — `--timeout` flags are specified but ignored

The proposed helper always uses a hard-coded five minutes ([04-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-03-PLAN.md:181)), while commands separately register configurable `migrateTimeout`/`backfillTimeout` flags ([04-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-03-PLAN.md:194), [04-04-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-04-PLAN.md:99)).

That contradicts the existing timeout convention, which reads the flag-backed variable and supports zero disabling the deadline ([spine_review_purge.go](/Volumes/Code/github.com/seanb4t/engram/cmd/engram/spine_review_purge.go:107)). It also makes the planned `--timeout 1s` test impossible to pass ([04-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-03-PLAN.md:219)).

This is actionable and currently not cleared: change the helper to accept/read the applicable flag value and explicitly decide whether zero disables, matching existing operator semantics.

### MEDIUM — Backfill alias apply bypasses manifest parity

The main migrate apply path correctly performs DryRun then manifest-limited apply ([04-03-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-03-PLAN.md:196)). The supposedly thin, identical backfill alias instead calls `Store.Migrate` with `DryRun:false` directly ([04-04-PLAN.md](/Volumes/Code/github.com/seanb4t/engram/.planning/phases/04-migration-cli-first-customer/04-04-PLAN.md:101)).

That means `backfill-short-ids --apply` does not use the same preview/intersection behavior as `engram migrate --apply`, despite both delegating to the same migration. This is actionable and absent from the plan’s tests and acceptance criteria.

## 3. Per-plan notes

### 04-01

H7 and M13 are adequately addressed. No remaining plan-specific blocker found.

### 04-02

Still blocked by:

- Batch-local rather than whole-operation unsupported-version preflight.
- Contradictory value-change write specification.
- No partial `DeletePayload`/`SetPayload` reconciliation.
- Future-version aggregation losing individual versions.
- No actual task or artifact creating exported `PreviewRevert`.

### 04-03

The H5 manifest bridge and M12 derivation are corrected. However:

- `migrateWithTimeout` ignores the registered timeout value.
- It assumes `Store.PreviewRevert` was produced by 04-02, but 04-02 never instructs the executor to create it.

### 04-04

The timeout is preserved, closing H8’s unbounded-operation concern, but its value is not wired into the shared helper. The alias apply path also bypasses the manifest-limited parity used by the canonical migrate command.

## 4. Risk assessment

**Overall risk: HIGH.**

The cycle does **not converge**. H6 remains partially unresolved at HIGH severity because the proposed whole-operation revert preflight can still mutate earlier pages before discovering an unsupported record. M3 also leaves the anti-additive write path internally contradictory and unreconciled across multiple RPCs.

**Convergence verdict: NO — unresolved HIGH remains.**

---

## OpenCode Review

# Cross-AI Plan Review — Phase 4, Cycle 3

Verified against source at repo root. Commit `50667cbf`'s revisions were checked claim-by-claim against the shipped code.

## 1. Ledger Adjudication

| Item | Status | Evidence |
|------|--------|----------|
| **H5** (manifest bridge non-functional) | **RESOLVED** | 04-03 Task 2 step 2 now specifies the apply closure calls `st.Migrate(ctx, {DryRun:true})` *inside itself*, captures `previewResult.PreviewManifest`, then calls `st.Migrate(ctx, {Manifest: previewManifest})` — structurally identical to the purge precedent where `spinePurgeApplyRun` calls `st.PreviewPurge` at `cmd/engram/spine_review_purge.go:365` and passes the manifest to `ApplyPurge` at `:373`, all within one invocation. The package-var design is gone; a `! rg -n "migrateLastPreviewManifest"` verify gate pins its absence. 04-01 supplies the store half (`MigrateOptions.Manifest`, `MigrateResult.PreviewManifest`). |
| **H6** (StepsFrom order + unsupported-version preflight) | **RESOLVED** | 04-02 Task 2 pins the invocation and it is *mechanically correct*: `StepsFrom(steps, from, to)` walks forward from→to (`internal/migrate/registry.go:102-127`), so for a record at version `from` reverting to `to`, `StepsFrom(steps, to, from)` yields the forward chain `to→from`, which reversed gives inverse-application order. Grep gates (`rg "StepsFrom\(steps, to, from\)"` present, `StepsFrom\(steps, from, to\)` absent) pin it. Whole-operation preflight added via `preflightRecordVersionSupport` — **but see New Finding N2: the preflight is batch-scoped, not whole-operation, which partially undercuts this resolution in practice.** The spec-level pinning is done; the mechanism has a hole. |
| **H7** (PA-3 vs Appeared records) | **RESOLVED** | 04-01 Task 2 step 5 makes the manifest-limited apply a single pass (one Count, full scroll, one post-Count), bypassing the re-derivation `for {}` loop at `internal/store/migrate.go:141-178` entirely, so the PA-3 guard at `:167-178` has no second pass to compare. `res.Backlog` is set to the true post-apply Count including Appeared records, and a must_have prohibition forbids claiming `Backlog == 0` when Appeared exist. Test 13 asserts `Backlog == Appeared > 0` then full-sweep convergence to 0. Correct. |
| **H8** (timeout removal → unbounded ops) | **RESOLVED (with a new spec contradiction — N3)** | 04-03 adds `migrateWithTimeout` (5-min default) mirroring `spinePurgeWithTimeout` (`spine_review_purge.go:330`, used at `:362`), and 04-04 Task 1 step 2 explicitly PRESERVES the `--timeout` flag and `backfillTimeout` var (`cmd/engram/backfill.go:21,80`). `registerDestructive`'s signal-only cancellation (`destructive.go:125`) is acknowledged as insufficient. The unbounded-operation defect is closed; the *composition* of flag vs. helper is now contradictory (N3). |
| **M8** (unexported preflight uncallable from CLI) | **UNRESOLVED — HIGH** | The cycle-3 revision moved the problem rather than fixing it. 04-03's `migrateFamilyStore` interface requires `PreviewRevert(ctx, to) (store.RevertPlan, error)` and its context block claims this comes "from 04-02 M8 resolution". **But 04-02 defines no such symbol.** 04-02 Task 2 specifies only the *unexported* `reversePreflight(steps, to) error` and `preflightRecordVersionSupport`, plus `Revert`/`revertWithSteps`/`RevertResult`. Neither `Store.PreviewRevert` nor the `RevertPlan` type appears in any plan's `files_modified` or task action — 04-02 touches `internal/store/{migrate_status,revert}.go` without them; 04-03 touches only `cmd/engram/` + `internal/surfaces/`. The CLI is again told to call a store method no plan creates. This is precisely the M8 failure class: mechanically uncallable as specified. |
| **M3** (inverse write contract) | **PARTIAL** | 04-02 Task 2 step 3 now specifies RemovedKeys→DeletePayload, AddedKeys→SetPayload, schema_version always re-stamped, and *explicitly documents* that value-changed-in-place keys are invisible to the key-set diff ("Keys whose VALUE changed … are NOT detected"). That is an honest disposition of the changed-value half, but it is prose containment, not a mechanism — an inverse that rewrites a value in place silently no-ops at the write boundary, and no test asserts detection/refusal of that case. Partial multi-RPC inverse failure is covered only by "resume = call Revert again", with no forced-mid-sequence test analogous to `TestMigratePartialFailureResume`. Actionable bar: the contract *is* written into the plan, so an executor can implement it; the residual risk is accepted-by-documentation. Clears the actionable bar, hence PARTIAL not UNRESOLVED. |
| **M4** (future-version distribution discarded) | **PARTIAL** | 04-02 Task 1 now separates future-version records — but collapses *all* of them into one scalar `FutureVersion uint64`. The per-version distribution (v2 vs. v3) is still discarded, and the must_have truth's promise of "a distinct count **with a version annotation**" is not met by the specified struct, which carries no annotation. The startup warning (Task 3) consumes only the scalar. For the current single-step registry this is cosmetically fine, but the finding's substance — individual version annotation — remains unimplemented. |
| **M12** (TestDestructiveCommandsRequireApply fails on migrate) | **RESOLVED (with a sibling-test coverage gap — N1)** | 04-03 Task 1 steps 5–6 add `mutatingCommandNames()` derived from `!op.Class.ReadOnly && op.CLICommand != ""` and re-derive the `--apply` set in `TestDestructiveCommandsRequireApply` (`cmd/engram/destructive_test.go:88-106`) from it. The `migrate` row (Destructive:false, ReadOnly:false) is then correctly in the want-set, so the second-direction assertion at `:101-104` no longer fails. |
| **M13** (additive_test.go empty-registry assertion) | **RESOLVED** | 04-01 Task 1 step 10 replaces the `len(Registry) != 0` fatal at `internal/migrate/additive_test.go:40-42` with a `>= 1` assertion plus a `Registry[0].From()==0 && To()==1` pin, keeping the fixture table on test-only steps. Matches the actual code. |

## 2. New Concerns (cycle-3 revisions)

### N1 — MEDIUM (actionable): Routing/flag-set conformance gates silently stop covering the new mutating commands

04-03 Task 1 updates only `TestDestructiveCommandsRequireApply` to the `!ReadOnly` derivation. Three sibling gates in `cmd/engram/destructive_test.go` still iterate `destructiveCommandNames()` (Destructive:true only):

- `TestDestructiveCommandsRouteThroughGate` (`destructive_test.go:130-152`) — proves RunE is the `registerDestructive` closure. After the gate generalization, `migrate` and `backfill-short-ids` route through `registerDestructive` but are **not** in this test's set, so a hand-rolled RunE on either would pass green — re-opening exactly the bypass class this test exists to catch.
- `TestDestructiveCommandsExactFlagSet` (`destructive_test.go:~240-260`) — the "no escape hatch" flag-set equality gate keyed on `destructiveFlagCases` (`:231-238`). No rows are added for `migrate`/`migrate revert`/`backfill-short-ids`, so a stray bypass flag on any of them is invisible.
- `TestApplyFlagUsageComposesRuleSentence` (`:200-220`) — same derivation; coverage-only, not a failure.

None of these *fail* — which is worse than failing: the safety net shrinks precisely as the gate widens. The plan must state whether these three tests move to `mutatingCommandNames()` (RouteThroughGate and ExactFlagSet should; the flag-case table needs rows for the three new commands) or explicitly accept the gap. Currently silent → invisible to the executor.

### N2 — MEDIUM: 04-02's "whole-operation" unsupported-version preflight is only batch-scoped

04-02 Task 2 step 3 says the revert loop keeps "the same re-derive-per-pass loop shape as `Store.Migrate` — per pass a fresh exact Count … then a fresh Scroll (Offset nil)" with `Limit: batch`, and the preflight "iterate[s] ALL records in the **current scroll batch**". `Store.Migrate` processes one batch per pass (`internal/store/migrate.go:185-196`). So with >`migrateBatch` (256) records above target, pass 1's records are **written** before pass 2's preflight ever sees a record in a later batch. An unsupported-version record beyond the first batch is discovered only after earlier records were modified — directly contradicting the must_have truth "refuses the entire operation with **zero records touched**" and the H6 whole-operation guarantee. Fix is cheap (preflight pass scrolls the *entire* above-target set before the first write pass, or the first pass is preflight-only), but the plan as written does not say it.

### N3 — MEDIUM (actionable): 04-04's timeout composition is internally contradictory

04-04 Task 1 step 2 says the alias "PRESERVEs the `--timeout` flag and its `backfillTimeout` var" and step 3 says both closures "use the shared `migrateWithTimeout` helper from 04-03". But `migrateWithTimeout` (04-03 Task 2 step 0b) is a **fixed** 5-minute `context.WithTimeout` taking no duration. If the closures use the fixed helper, the preserved `backfillTimeout` var (`backfill.go:21`, registered at `:80` with "0 disables" semantics) is never read — a dead flag the D-12 gate then asserts *exists*. The plan never states that `migrateWithTimeout` takes a duration parameter or that the flag feeds it. Same ambiguity in 04-03 Task 2 step 2 ("`--timeout` flag" vs. fixed helper "when no explicit timeout is provided" — composition unspecified). An executor must guess whether `--timeout 30s` does anything.

### N4 — LOW (actionable): 04-01 DryRun count semantics contradict themselves

04-01's must_haves say DryRun "counts would-migrate", but Task 2 test 11 asserts `res.Migrated == 0` for DryRun, and no `WouldMigrate` field is added to `MigrateResult` (only `PreviewManifest`/`Spared`/`Appeared`). The would-migrate count is then only recoverable as `len(PreviewManifest)` — say so, or add the field. As written the executor cannot tell which field carries the projection count.

### N5 — LOW: 04-03 Task 1 step 8 makes the rule-sentence update conditional

`RuleDestructiveRequiresApply`'s Sentence is literally "a **destructive** operator command previews by default…" (`internal/surfaces/rules.go:232`), and `addApplyFlag` composes every `--apply` Usage string from it (`destructive.go:57-63`). Once `migrate` (additive) carries `--apply`, the sentence misdescribes it — so the plan's "if … would misdescribe … leave it" hedge resolves to "must update", and updating it re-anchors the sentence on every prose surface the conformance gate resolves it to. This should be a pinned task, not a conditional; currently the executor may legitimately skip it.

## 3. Per-Plan Notes

- **04-01:** Sound. The CheckAdditive carve-out (declared key already present in `before`) is correctly scoped against `internal/migrate/additive.go:87-91` — the `missing` branch is the only one touched; undeclared/removed branches at `:82-86,64-66` stay intact. The minter branch preserves the two-clone discipline at `migrate.go:224-225`. Only N4 needs a one-line fix.
- **04-02:** Two holes: it never creates the `PreviewRevert`/`RevertPlan` symbols 04-03 depends on (M8), and its preflight is batch-scoped (N2). The `StepsFrom(steps, to, from)` pinning is verified correct against `registry.go:102-127`. `reversePreflight` walking steps with `To() > to` is correct for the linear chain.
- **04-03:** H5/H8/M12 fixes are real and match the purge precedent. Blocked on 04-02's missing `PreviewRevert` (M8); N1 and N5 are its own gaps.
- **04-04:** Correctly preserves `--timeout` per H8 and correctly limits the upgrade-guide entry to `--dry-run` removal; N3 (dead flag vs. fixed helper) must be reconciled. Dead-code deletion order (grep-then-delete, `MintShortID` kept) is right.

## 4. Risk Assessment

**Overall: MEDIUM-HIGH. Convergence verdict: DOES NOT CONVERGE** — one unresolved HIGH remains.

- **M8 is UNRESOLVED (HIGH):** the revision relocated the missing `PreviewRevert`/`RevertPlan` definition from "unexported helper the CLI can't call" to "exported method no plan defines". 04-03's interface, preview/apply closures, tests, and acceptance gates all reference `store.RevertPlan`/`Store.PreviewRevert`; zero tasks create them. Wave 3 execution would halt on this immediately. The fix is small — add a `PreviewRevert` + `RevertPlan` task to 04-02 (or an explicit store-side task in 04-03 with `internal/store/revert.go` in `files_modified`) — but it must be written down.
- Supporting MEDIUMs (N1 conformance-gate coverage shrink, N2 batch-scoped preflight, N3 dead `--timeout` flag) are each one-paragraph plan edits.
- Everything else on the ledger is genuinely closed: H5, H7, H8, M12, M13 are resolved with correct mechanisms verified against source; H6 is spec-correct (N2 is an implementation-shape gap, not a spec error); M3/M4 are documented dispositions that clear the actionable bar.

**One more cycle, narrowly scoped:** define `Store.PreviewRevert`/`RevertPlan` in 04-02, widen the preflight to the full above-target set (N2), extend the three sibling conformance tests to the mutating set (N1), and pin the `--timeout`→helper composition (N3). With those four edits, this phase converges.

---

## Verification coverage

Both lanes ran with repo file access and cited `file:line` evidence; neither output carries the
`[reviewed-without-repo-access]` marker, so both verdicts count at full consensus weight.

Source files independently cited by at least one reviewer while adjudicating this cycle:

| File | Cited for |
|---|---|
| `cmd/engram/spine_review_purge.go:107,330,339-377` | purge precedent for in-closure preview and for flag-backed timeout semantics |
| `cmd/engram/destructive.go:57-63,125` | `addApplyFlag` usage composition; signal-only cancellation (no deadline) |
| `cmd/engram/destructive_test.go:84,88-106,130-152,200-220,231-238` | the four conformance gates and the flag-case table |
| `cmd/engram/backfill.go:21,80` | preserved `--timeout` flag and `backfillTimeout` var |
| `internal/store/migrate.go:141-178,185-196,224-225` | PA-3 guard, one-batch-per-pass loop shape, two-clone minter discipline |
| `internal/store/store.go:2782` | old backfill writing `short_id` without `schema_version` |
| `internal/migrate/registry.go:92,102-127` | forward-only `StepsFrom` contract |
| `internal/migrate/additive.go:12,38,64-66,82-91` | key-presence-only diff helpers; `CheckAdditive` branch scoping |
| `internal/migrate/additive_test.go:39-42` | empty-registry assertion (M13) |
| `internal/surfaces/rules.go:232` | `RuleDestructiveRequiresApply` sentence text |

Claims verified by tracing rather than accepted from plan text: the manifest bridge dispatch path,
the PA-3 second-pass comparison, the `StepsFrom` argument order, the `CheckAdditive` carve-out
branch, and both directions of `TestDestructiveCommandsRequireApply`.
