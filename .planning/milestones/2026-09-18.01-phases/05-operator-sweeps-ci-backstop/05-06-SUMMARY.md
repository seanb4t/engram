---
phase: 05-operator-sweeps-ci-backstop
plan: 06
subsystem: store
tags: [red-evidence, inventory, requirements, issue-497, D-01, D-07]

requires:
  - phase: 05-operator-sweeps-ci-backstop
    provides: "plans 05-01 through 05-05 — every sweep migrated onto scrollAllPoints, unbudgetedView deleted, and the 64 MiB backstop in place; this plan proves each of those guarantees is live and closes the phase"
provides:
  - "Eleven registered red-evidence patches for phase 5, taking internal/store's harness from 42 to 53 live apply-RED-revert directions"
  - "The Phase 5 closing-check results for 03-INVENTORY.md, recorded in writing"
  - "This phase's four requirements ticked; GitHub #497 closed on #498's evidence"
affects: [06, 07]

actuals:
  tasks: 3
  commits: 3
  plan_head_before: 3e2178920e9f1dcbbc6a6f8a1cf68bd88e0e62dd

key-files:
  created:
    - .planning/phases/05-operator-sweeps-ci-backstop/red-evidence/ (11 patches)
  modified:
    - internal/store/redevidence_harness_test.go
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-INVENTORY.md
    - .planning/REQUIREMENTS.md

key-decisions:
  - "The inventory's closing check (a) was reconciled IN WRITING rather than by editing the derivation command to match — the command is part of the recorded contract, and changing it to produce a desired number would game the gate rather than close it."
  - "Store.reindexTargetContents reclassified Phase 5 -> Exempt: it is a direct s.client.Get that routes through no shared primitive, and its batch is bounded by the page its caller accumulates, not by a byte-budget view. Recording the operator's --batch sizing as a load-bearing responsibility is more honest than implying an inherited bound it does not have."
  - "#497 closed on #498's existing evidence per D-07 — no new test, no CI change, no fixture-size gate."

requirements-completed: [REQ-bounded-read-mechanism, REQ-sweeps-bounded, REQ-recv-limit-backstop, REQ-ci-store-green]

duration: interrupted-and-closed-by-orchestrator
completed: 2026-09-20
status: complete
---

# Phase 5 Plan 6: Red Evidence, Inventory Close & Requirements Summary

**Every guarantee phase 5 added is now a live, harness-checked RED direction (42 → 53 patches), the call-site inventory's closing checks are reconciled in writing, and the milestone's one-shared-mechanism requirement is closed.**

## Execution note: interrupted, closed out by the orchestrator

The executor for this plan completed Task 1 and did the substantive work of Tasks 2 and 3, but
stalled twice waiting on its own backgrounded `task` gate and ended its turn without committing.
After a resume nudge produced no forward progress (≈500k tokens, 212 tool calls), the orchestrator
stopped the agent and closed the plan out by hand — the documented recovery for an executor that
has work on disk but no SUMMARY.

What the orchestrator did, in order:
1. Reconciled the tree: confirmed `internal/store/revert.go` was CLEAN vs HEAD (no red-evidence
   patch left applied mid-cycle) and that all 11 patch files plus the full `redEvidenceDirs`
   registration were present but uncommitted.
2. Committed Task 2's work (`9b0d6476`) and Task 3's work (`4a5dfbea`) as two coherent commits.
3. Re-ran the gate the executor never reached, on the now-clean committed tree:
   `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/ -run '^TestRedEvidencePatchesAreLive$' -count=1 -timeout 45m -v`
   → **PASS, exit 0, 242.467s**, with all 11 phase-5 subtests green.

No work was redone or invented; the executor's content was committed as found.

## Accomplishments

- **Eleven red-evidence patches registered**, one per RED direction this phase created:

  | Patch | Target test |
  |---|---|
  | `05-01-scanspine-view-unbudgeted` | `TestScanSpineBoundedOverGRPCLimit` |
  | `05-01-citations-view-unbudgeted` | `TestEnumerateCitationsBoundedOverGRPCLimit` |
  | `05-01-purge-view-unbudgeted` | `TestPreviewPurgeBoundedOverGRPCLimit` |
  | `05-02-revert-preview-view-unbudgeted` | `TestRevertPreviewBoundedOverGRPCLimit` |
  | `05-03-migrate-sweep-view-unbudgeted` | `TestMigrateBoundedOverGRPCLimit` |
  | `05-03-migrate-pass-sentinel-escapes-as-failure` | `TestMigrateBoundedOverGRPCLimit` |
  | `05-03-revert-pass-sentinel-escapes-as-failure` | `TestRevertApplyBoundedOverGRPCLimit` |
  | `05-04-summarize-limit-sentinel-escapes` | `TestSummarizeMissingBoundedOverGRPCLimit` |
  | `05-04-reindex-walks-store-collection-not-source` | `TestReindexSourceOverride` |
  | `05-04-reindex-final-partial-page-dropped` | `TestReindexBoundedOverGRPCLimit` |
  | `05-05-backstop-appended-after-caller-options` | `TestQdrantRecvLimitBackstopPrecedesCallerOptions` |

- **Inventory closing checks reconciled** (`03-INVENTORY.md`, new `## Phase 5 closing-check results`):
  - **(a)** The derivation's real output is **26 function names, not 27**. Its 27th line is BLANK — a
    command artifact: the per-file `awk` pass matches the literal `s.client.ScrollAndOffset(`
    inside `spine.go`'s own doc comment describing the mechanism it replaced, which appears before
    that file's first `^func`, so `fn` is still unset and an empty string is printed. Recorded
    rather than "fixed": excluding comments would require parsing Go rather than matching text.
    (The orchestrator had itself reported 27 from the raw `wc -l` before this was caught.)
  - `Store.fetchPayloadBatch` gained the row Phase 4's WR-01 code-review fix never added.
  - `Store.listByCursor` and `Store.ListScheduled` are annotated as invisible-to-the-derivation
    (they compose `scrollOrderedPage` one level down) rather than absent from the mechanism.
  - `Store.reindexTargetContents` reclassified **Phase 5 → Exempt** with its operator-sized
    `--batch` stated as a real responsibility.
  - **(b)** prints `0` — `unbudgetedView` is gone (verified whole-package, not just production-scoped).
  - **(d)** recall-gate classifications match: `Store.SummarizeMissing`/`Store.Reindex` rows deleted
    (no direct emission survives), `Store.Migrate`/`Store.revertWithSteps` edited (both keep a `Count`).

- **Four requirements ticked**: REQ-bounded-read-mechanism (spanning phases 3–5, closing here),
  REQ-sweeps-bounded, REQ-recv-limit-backstop, REQ-ci-store-green.

- **#497 closed on #498's evidence** per D-07 — a CI-stability issue whose root cause (one shared
  `services:` Qdrant for the whole job) shipped and has held 60+ runs. No new test, no CI change.

## Task Commits

1. **Register the backstop-order direction** — `2b13fd46` (test)
2. **Register the remaining ten patches** — `9b0d6476` (test, committed by the orchestrator)
3. **Reconcile the inventory and close the requirements** — `4a5dfbea` (docs, committed by the orchestrator)

## Deviations from Plan

- **Execution interrupted; closed out by the orchestrator** (see the note above). The plan's content
  was executed as written; only the commit and final-gate steps were performed by the orchestrator.
- **The derivation count in the plan's own framing (27) was wrong** — it is 26 real names plus a
  blank artifact. Reconciled in writing per the plan's own prohibition on editing the command.

## Issues Encountered

- The red-evidence harness gives spurious failures against a DIRTY tree; every phase-5 plan hit this.
  The gate was run only after committing, as the plan requires.
- Patch cost is materially higher this phase (most targets seed real oversized Qdrant fixtures):
  the phase-5 block alone took 56s of the 242s run. No timeout in `Taskfile.yaml` or CI was raised
  to accommodate it, per the plan's explicit prohibition — an extended `-timeout` was passed on the
  one-off command instead.

## Known Stubs

None.

## Next Phase Readiness

- `TestRedEvidencePatchesAreLive` is GREEN at 53 patches (Phase 1×4, 2×8, 3×13, 4×17, 5×11).
- Every site in the Phase 3 inventory now routes through a shared primitive or carries a written
  exemption — `internal/store` has exactly ONE direct `ScrollAndOffset` left in production code,
  inside `Store.scrollAllPoints` itself.
- Phases 6 (cross-spine partial results) and 7 (bounded provider responses) are independent of this
  work and unblocked.

---
*Phase: 05-operator-sweeps-ci-backstop*
*Completed: 2026-09-20*
