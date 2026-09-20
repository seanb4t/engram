---
phase: 05-operator-sweeps-ci-backstop
plan: 04
subsystem: database
tags: [qdrant, bounded-reads, summarize-missing, reindex, byte-budget, sentinel-error]

# Dependency graph
requires:
  - phase: 05-operator-sweeps-ci-backstop
    provides: "scrollAllPoints(ctx, collection, filter, view, fn) with a collection parameter and WithVectors(false) (plan 05-01); Store.Migrate's and Store.revertWithSteps' own pass loops migrated with the per-sweep sentinel-batch pattern established (plan 05-03)"
provides:
  - "Store.SummarizeMissing's hand-rolled 256-record ScrollAndOffset loop deleted, replaced by one scrollAllPoints(ctx, s.collection, filter, s.fullView()) call; its caller-limit mid-page early exit now expressed as errSummarizeLimitReached, unwrapped with errors.Is at the call site"
  - "Store.Reindex's hand-rolled batch-sized ScrollAndOffset loop over the source deleted, replaced by one scrollAllPoints(ctx, source, nil, s.fullView()) call over the EFFECTIVE source collection; the per-page reindexTargetContents resume lookup preserved via a closure-scoped page accumulator (flushReindexPage), with a new sentinel errReindexPageFull produced and consumed entirely within the scrollAllPoints callback (never propagated to scrollAllPoints itself, since any callback error halts the walk outright)"
  - "internal/store/summarizesweep_oversized_test.go: TestSummarizeMissingBoundedOverGRPCLimit (dry-run, mid-page caller-limit, and non-dry-run egress-boundary sub-cases), both fixture shapes, real-Qdrant"
  - "internal/store/reindexsweep_oversized_test.go: TestReindexBoundedOverGRPCLimit (apply, resume, and source-override sub-cases), both fixture shapes, real-Qdrant"
  - "operatorMigrationEmitters rows for Store.SummarizeMissing and Store.Reindex DELETED (not edited) — after this plan neither function has a direct s.client.* emission left in the recall gate's vocabulary; Store.scrollAllPoints' justification widened to name both migrated sweeps"
affects: [05-05, 05-06]

# Actuals (#2632)
actuals:
  tokens: 7531
  tasks: 2
  commits: 2
plan_head_before: ee40800304457967eeef93f08e23542767c36bac

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-sweep sentinel-error batch boundary (D-01), continued: SummarizeMissing's errSummarizeLimitReached follows the same shape as plan 05-03's migrate/revert sentinels — a package-level marker returned from the scrollAllPoints callback, unwrapped with errors.Is at the call site, never shared across sweeps."
    - "Closure-scoped page accumulator with an in-callback flush (Reindex only): because reindexTargetContents needs a whole page's points at once (one Get per PAGE, never per record) and any non-nil callback return halts scrollAllPoints outright, the page-full condition cannot escape the callback the way a cross-pass sentinel does for Migrate/Revert (which restart via a shrinking server-side filter Reindex's nil-filter walk has no equivalent of). Instead the accumulator's flush closure (flushReindexPage) runs synchronously inside the callback when the threshold is reached, and the callback always returns nil so the single scrollAllPoints call continues over the remaining points; errReindexPageFull is produced and consumed entirely within that closure boundary purely to mark a full-page flush for observability, never crossing scrollAllPoints itself. The identical closure runs the trailing, possibly-partial final flush after the iterator exhausts, so there is exactly one copy of the per-point body."

key-files:
  created:
    - internal/store/summarizesweep_oversized_test.go
    - internal/store/reindexsweep_oversized_test.go
  modified:
    - internal/store/summarize.go
    - internal/store/store.go
    - internal/store/schemaversion_recallgate_test.go

key-decisions:
  - "Reindex's sentinel-and-resume design deliberately differs from Migrate/Revert's cross-pass restart pattern (plan 05-03), because it cannot be applied here: Migrate/Revert restart scrollAllPoints between passes and rely on a server-side filter that SHRINKS as each pass's writes remove processed records from it (schema_version stamped on the SOURCE), so a fresh pass naturally skips already-migrated records. Reindex's walk uses filter=nil over a read-only source with no such shrinking mechanism, and Reindex's own action text requires exactly ONE scrollAllPoints call — so re-invoking it after a sentinel-triggered halt would restart at the beginning and either lose all subsequent records (if treated as terminal) or reprocess the same first page forever (if retried naively). The only design that visits every record exactly once, keeps one Get per page, and never drops a trailing partial page is a synchronous in-callback flush: errReindexPageFull is declared, produced, and consumed entirely inside the single scrollAllPoints call, distinguishing a full-page flush from the trailing partial one purely for observability (accumulator-flush counting), and the callback always returns nil so the underlying scan is never interrupted. This is exercised by 05-CONTEXT.md's own 'Claude's Discretion' allowance for 'the sentinel-error shape for early termination.'"
  - "The oversized regression's Batch is sized per shape (reindexResumeSafeBatch), not left at the production default (256): reindexTargetContents' Get() call is explicitly OUT OF this migration's scope (unchanged, per the plan's <trap_note> and prohibition against touching it) and remains byte-UNbounded — it requests up to `batch` records' full payload in one RPC regardless of content size. The default batch=256 is safe for ManySmall's ~5.2 KiB records but would make that one Get() request ~5.24 MiB for FewLarge's 40 131072-byte records — the exact overflow this plan's SOURCE-walk migration fixes, just relocated to the untouched per-page lookup. Sizing Batch to keep that Get() under half of storetest.RecvLimit (clamped to [2, 256], never 1 — never a per-record lookup, T-05-04-03) lets the resume sub-case actually complete on FewLarge while still exercising multiple accumulator flushes (observed 3 for few-large at Batch=16, 4 for many-small at the default 256)."

requirements-completed: []  # Plan 05-06 owns ticking REQ-bounded-read-mechanism/REQ-sweeps-bounded (shared-ID gate, engram_project_gates); requirements.ready-ids confirmed 0/2 ready at this plan's close

coverage:
  - id: D1
    description: "Store.SummarizeMissing's own-loop sweep bounded on the shared byte-budget iterator, its caller-limit mid-page early exit expressed as errSummarizeLimitReached, proven over an oversized no-summary scope on both fixture shapes with the in-place summarize characterization suite unedited and green; the caller-limit path and the non-dry-run egress boundary are covered by a test for the first time"
    requirement: "REQ-sweeps-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestSummarizeMissingBoundedOverGRPCLimit/few-large"
        status: pass
      - kind: integration
        ref: "internal/store#TestSummarizeMissingBoundedOverGRPCLimit/many-small"
        status: pass
      - kind: integration
        ref: "internal/store (in-place summarize characterization suite: all 8 TestSummarize*/TestFillSummary* tests, run via -run '^TestSummarize|^TestFillSummary')"
        status: pass
    human_judgment: false
  - id: D2
    description: "Store.Reindex's own-loop walk over its effective source collection bounded on the shared byte-budget iterator with its per-page target resume lookup preserved via a closure-scoped accumulator, proven over an oversized source on both fixture shapes (apply, resume, and source-override sub-cases) with the in-place reindex characterization suite (17 tests) unedited and green"
    requirement: "REQ-sweeps-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestReindexBoundedOverGRPCLimit/few-large"
        status: pass
      - kind: integration
        ref: "internal/store#TestReindexBoundedOverGRPCLimit/many-small"
        status: pass
      - kind: integration
        ref: "internal/store (in-place reindex characterization suite: all 17 TestReindex* tests, run via -run '^TestReindex')"
        status: pass
    human_judgment: false
  - id: D3
    description: "Store.SummarizeMissing and Store.Reindex rows DELETED from operatorMigrationEmitters (neither function retains a direct s.client.* emission after migration); Store.scrollAllPoints' justification widened to name both; recall gate's set-equality subtest reports zero extra/missing names"
    requirement: "REQ-bounded-read-mechanism"
    verification:
      - kind: integration
        ref: "internal/store#TestRecallEmissionSetIsCompleteAndClassified (all subtests)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The full internal/store suite (all in-place tests across the package) stays green on a clean, committed tree; TestRedEvidencePatchesAreLive reported zero failures (phase 5's own red-evidence patches are not yet registered — plan 05-06's job), matching the pattern already observed in plans 05-01 and 05-03"
    verification:
      - kind: unit
        ref: "internal/store (whole-package run, go test ./internal/store/ -count=1 -timeout 40m)"
        status: pass
    human_judgment: false

# Metrics
duration: 68min
completed: 2026-09-20
status: complete
---

# Phase 5 Plan 4: The Last Two Own-Loop Sweeps Bounded on the Shared Iterator

**Deleted `engram summarize-missing`'s and `engram reindex`'s hand-rolled `ScrollAndOffset` cursor loops, replacing each with `scrollAllPoints`, and removed the two `operatorMigrationEmitters` rows that stopped being true the moment each function's last direct emission disappeared — closing REQ-bounded-read-mechanism's own-loop-sweep half.**

## Performance

- **Duration:** 68 min
- **Started:** 2026-09-20T17:16:58Z (approx, from prior plan's completion)
- **Completed:** 2026-09-20T18:25:00Z (approx)
- **Tasks:** 2
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- `Store.SummarizeMissing`'s 256-record `ScrollAndOffset` loop is gone, replaced by one `scrollAllPoints(ctx, s.collection, filter, s.fullView())` call; its caller-limit mid-page early exit is `errSummarizeLimitReached`, a package-level sentinel unwrapped with `errors.Is` at the call site. Observed on real Qdrant: `few-large` — dry-run `Scanned=40 Filled=40`, limited(20) `Scanned=20`, apply `Scanned=40 Filled=40 summariser_calls=40`; `many-small` — dry-run `Scanned=1000 Filled=1000`, limited(500) `Scanned=500`, apply `Scanned=1000 Filled=1000 summariser_calls=1000`.
- `Store.Reindex`'s batch-sized `ScrollAndOffset` loop over the source is gone, replaced by one `scrollAllPoints(ctx, source, nil, s.fullView())` call over the EFFECTIVE source collection (never `s.collection` unconditionally). The per-page `reindexTargetContents` resume lookup (one `Get` per page, never per record) is preserved via a closure-scoped accumulator (`flushReindexPage`), called from both the mid-scan threshold branch and the trailing partial-page flush, so no page is silently dropped and there is exactly one copy of the per-point embed/skip/upsert body. `reindexTargetContents` itself is byte-for-byte untouched. Observed on real Qdrant (`Batch` sized per shape — see Decisions): `few-large` (`Batch=16`) — apply `Scanned=40 Upserted=40`, target count 40, 3 accumulator flushes; resume `Unchanged=40 Upserted=0`; source-override `Scanned=40`. `many-small` (`Batch=256`, the production default) — apply `Scanned=1000 Upserted=1000`, target count 1000, 4 accumulator flushes; resume `Unchanged=1000 Upserted=0`; source-override `Scanned=1000`.
- `internal/store/summarizesweep_oversized_test.go` (new, `package store_test`) adds `TestSummarizeMissingBoundedOverGRPCLimit`: a dry-run full-sweep sub-case, a mid-page caller-limit sub-case (the case none of the 8 in-place tests reaches, since none spans more than one page), and a non-dry-run sub-case asserting the summariser was invoked exactly `res.Filled` times.
- `internal/store/reindexsweep_oversized_test.go` (new, `package store_test`) adds `TestReindexBoundedOverGRPCLimit`: an apply sub-case into a fresh target, a resume sub-case proving `res.Unchanged` covers the whole scope with zero re-upserts, and a source-override sub-case proving a store constructed against a DIFFERENT collection still reads the effective `Source`.
- `Store.SummarizeMissing`'s and `Store.Reindex`'s `operatorMigrationEmitters` rows are DELETED (not edited, unlike plan 05-03's `Store.Migrate`/`Store.revertWithSteps` rows, which kept a direct `Count`): after this plan neither function emits anything in the recall gate's vocabulary. `Store.scrollAllPoints`'s justification is widened to name both.
- `s.client.ScrollAndOffset(` call sites in production `internal/store` (excluding a pre-existing doc comment at `spine.go:76`, see Issues Encountered) are down to exactly 1: the shared iterator itself (`spine.go:97`). Of the ten Phase 5 inventory rows, this closes the last two own-loop sweeps.

## Task Commits

Each task was committed atomically:

1. **Task 1: The summarize sweep bounded, its caller limit expressed as a sentinel, and its classification row removed in the same commit (tracer)** — `58a4af5c` (refactor)
2. **Task 2: The reindex walk moved onto the iterator over its effective source collection, its per-page target lookup preserved, and its classification row removed in the same commit** — `97834b7e` (refactor)

**Plan metadata:** (this commit)

## Files Created/Modified

- `internal/store/summarize.go` — added `errSummarizeLimitReached`; `SummarizeMissing`'s own loop now routes through `scrollAllPoints`.
- `internal/store/store.go` — added `errReindexPageFull`; `Reindex`'s own loop over the source now routes through `scrollAllPoints` via the `flushReindexPage` closure/accumulator; `reindexTargetContents` untouched (verified via `git diff` on the commit).
- `internal/store/schemaversion_recallgate_test.go` — deleted the `Store.SummarizeMissing` and `Store.Reindex` rows from `operatorMigrationEmitters`; widened `Store.scrollAllPoints`'s justification.
- `internal/store/summarizesweep_oversized_test.go` (new) — `TestSummarizeMissingBoundedOverGRPCLimit`, real-Qdrant, both fixture shapes.
- `internal/store/reindexsweep_oversized_test.go` (new) — `TestReindexBoundedOverGRPCLimit`, real-Qdrant, both fixture shapes.

## Decisions Made

- **Reindex's sentinel-and-resume design deliberately differs from Migrate/Revert's cross-pass restart pattern** (see key-decisions in frontmatter for the full reasoning): Migrate/Revert restart `scrollAllPoints` between passes against a server-side filter that shrinks as writes land on the SOURCE; Reindex's `filter=nil` walk over a read-only source has no equivalent, and the action text requires exactly one `scrollAllPoints` call. The synchronous in-callback flush (`flushReindexPage`) is the only design that visits every record exactly once, keeps one `Get` per page, never drops a trailing partial page, and never lets a callback error halt the single scan mid-way. `errReindexPageFull` is declared and genuinely produced/consumed, scoped entirely within the callback boundary, purely to mark and count full-page flushes.
- **The oversized test's `Batch` is sized per shape** (`reindexResumeSafeBatch`), not left at the production default of 256 — see key-decisions for the full reasoning. `reindexTargetContents`'s `Get()` call is explicitly out of this migration's scope and remains byte-unbounded; the default batch overflows it for `FewLarge`'s very large records, so the test computes a smaller, still-genuinely-multi-record batch (never 1) to exercise the resume path safely while still proving multiple accumulator flushes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A doc comment initially named the sibling sentinel by identifier, tripping this task's own acceptance criterion**

- **Found during:** Task 1, running the acceptance-criteria grep `rg -o -e 'errMigratePassBatchComplete|errRevertPassBatchComplete' internal/store/summarize.go | wc -l` (must print `0` — each sweep's sentinel is declared separately)
- **Issue:** `errSummarizeLimitReached`'s doc comment initially cross-referenced `errMigratePassBatchComplete` by its literal identifier for context, which the criterion's substring grep — deliberately scoped to catch exactly this class of accidental sharing (the same trap plan 05-03 hit in `errRevertPassBatchComplete`'s comment) — correctly flagged.
- **Fix:** Reworded the comment to describe the relationship ("the same per-sweep sentinel-batch shape as migrate.go's own sweep-pass sentinel") without naming the other identifier.
- **Files modified:** `internal/store/summarize.go`
- **Verification:** `rg -o -e 'errMigratePassBatchComplete|errRevertPassBatchComplete' internal/store/summarize.go | wc -l` prints `0`; full task 1 verify re-run clean afterward.
- **Committed in:** `58a4af5c` (Task 1 commit — caught and fixed before committing, never landed in a bad state)

**2. [Rule 3 - Blocking] The oversized reindex regression's default Batch overflowed reindexTargetContents' out-of-scope Get() on FewLarge**

- **Found during:** Task 2, first real-Qdrant run of `TestReindexBoundedOverGRPCLimit`'s resume sub-case
- **Issue:** With the production default `Batch=256` and `FewLarge`'s 40 records of 131072 content bytes each, the accumulator never reaches the threshold before the source is exhausted, so all 40 records land in ONE trailing flush — and `reindexTargetContents`'s `Get()` call for that flush requests all 40 records' full payload at once (~5.24 MiB), overflowing `storetest.RecvLimit` (4 MiB). `reindexTargetContents` is explicitly out of this migration's scope (per the plan's `<trap_note>` and an explicit prohibition against touching it), so this is a genuine, pre-existing limitation the test exposed, not a code path this plan is asked to fix.
- **Fix:** Sized the test's `ReindexOptions.Batch` per shape via `reindexResumeSafeBatch(fx.RecordBytes)` — budgeting half of `storetest.RecvLimit` for the `Get()` request, clamped to `[2, 256]` (never a per-record lookup). This is a test-design change only; production code is untouched.
- **Files modified:** `internal/store/reindexsweep_oversized_test.go`
- **Verification:** `TestReindexBoundedOverGRPCLimit` passes on both shapes (`few-large` at `Batch=16`, 3 accumulator flushes; `many-small` at the still-default `Batch=256`, 4 flushes); full task 2 verify and the whole-package suite both re-run clean afterward.
- **Committed in:** `97834b7e` (Task 2 commit — caught and fixed before committing, never landed in a bad state)

---

**Total deviations:** 2 auto-fixed (1 bug in a doc comment, caught by the task's own verify gate; 1 test-design fix for an out-of-scope function's pre-existing byte-size limitation, caught by the task's own real-Qdrant run)
**Impact on plan:** No scope creep. Neither fix touches production behavior beyond what the plan already specified; the second fix is entirely test-parameter sizing, explicitly justified by the `<trap_note>`'s own boundary around `reindexTargetContents`.

## Issues Encountered

- **The stated acceptance-criteria literal `rg -o -e 's[.]client[.]ScrollAndOffset[(]' internal/store --glob '!*_test.go' | wc -l` prints `1`, but the actual count is `2`** — identical stale-literal pattern already documented in `05-03-SUMMARY.md`'s Issues Encountered: the second match is `spine.go:76`'s pre-existing doc comment ("the enclosing function of each direct `s.client.ScrollAndOffset(` call"), predating this plan (confirmed via `git show HEAD~2:internal/store/spine.go` — unchanged since plan 05-01). The actual CODE call-site count (excluding the comment) is exactly `1` — `spine.go:97`, the shared iterator itself — matching the plan's own stated invariant and the orchestrator-verified fact that only `spine.go:97` should remain. No code change follows.
- **A widespread, spurious first failure run against a DIRTY, uncommitted tree** (matching the engram project gate's own warning: "Run the harness only against a COMMITTED tree — a dirty tree produces spurious failures"): before Task 2's commit, running the full `internal/store` suite showed ~15 unrelated tests failing (`TestStoreListOverflowIsResponseTooLarge`, `TestSearchTwoPhaseBounded`, `TestScanSpineBoundedOverGRPCLimit`, several `TestRedEvidencePatchesAreLive` subtests, etc.). Root-caused via `redevidence_harness_test.go`'s own log line: `refusing to apply <patch>: files it touches are already dirty: M internal/store/store.go` — the red-evidence harness safety-refuses to apply ANY patch touching a file with uncommitted changes, and Task 2's in-progress edits to `store.go` were still uncommitted at that point. After Task 2's commit, a full re-run against the clean, committed tree showed the full suite green with ZERO failures (not even the plan's expected single `TestRedEvidencePatchesAreLive` red — see next item), confirming the earlier failures were entirely an artifact of testing against a dirty tree, not a regression from this plan's code.
- **The full `internal/store` suite ran fully green (zero failures, including `TestRedEvidencePatchesAreLive`) rather than the plan's stated "single expected RED"** — identical finding to `05-01-SUMMARY.md` and `05-03-SUMMARY.md`: `redEvidenceDirs` has no Phase 5 entry yet (plan 05-06's job), so there is nothing phase-5-specific for that test to fail on at this point in the phase. Confirmed with a full 366.5s run (`ok`, zero failures). No code change follows; the actual, better outcome supersedes the plan's assumption.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All four own-loop sweeps named in `.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-INVENTORY.md`'s Phase 5 rows (`Store.Migrate` ×3 loops, `Store.SummarizeMissing`, `Store.revertWithSteps`, `Store.Reindex`) are now migrated onto `scrollAllPoints`. Every site in the inventory now routes through a shared primitive or carries a written exemption (`reindexTargetContents`'s `Get`).
- `Store.SummarizeMissing`'s and `Store.Reindex`'s `operatorMigrationEmitters` rows are both DELETED (matching the orchestrator-verified fact that these two rows disappear entirely, unlike plan 05-03's `Store.Migrate`/`Store.revertWithSteps` rows, which SURVIVED with edited justifications because those two keep a direct `Count`).
- Not one counter, guard, message, filter, or write-shaping rule changed in either command; the full in-place `summarize_test.go` (8 tests) and `reindex_test.go` (17 tests) characterization suites ran unedited and green throughout — no test broke in either an expected or unexpected way.
- Plan 05-05 is unaffected by this plan's changes.
- Plan 05-06 still owns: authoring and registering this phase's own red-evidence patches, the 64 MiB `MaxCallRecvMsgSize` backstop (D-05/D-06), closing #497 (D-07), and ticking `REQ-bounded-read-mechanism`/`REQ-sweeps-bounded` once every declaring plan has a SUMMARY (shared-ID gate #2388, confirmed 0/2 ready via `requirements.ready-ids` at this plan's close) — this plan's `requirements-completed` is deliberately empty for that reason.

## Self-Check: PASSED

- `[ -f internal/store/summarizesweep_oversized_test.go ]` → FOUND
- `[ -f internal/store/reindexsweep_oversized_test.go ]` → FOUND
- `git log --oneline --all | grep -q 58a4af5c` → FOUND
- `git log --oneline --all | grep -q 97834b7e` → FOUND
- All plan-level `<acceptance_criteria>` re-verified passing per-task (see per-task grep/build/test output above), with one stale-literal clarification documented under Issues Encountered (matching 05-01/05-03 precedent) rather than silently forced to match, and two caught-before-commit deviations documented above.
- Plan-level `<verification>` block re-run on the clean, committed tree: `TestSummarizeMissingBoundedOverGRPCLimit`/`TestReindexBoundedOverGRPCLimit` — 4/4 subtests PASS; `s.client.ScrollAndOffset(` in non-test `internal/store` files — 1 real call site (`spine.go:97`), 1 pre-existing doc-comment match (see Issues Encountered); full suite (`go test ./internal/store/ -count=1 -timeout 40m`) — `ok`, 366.507s, zero failures (better than the plan's expected "one red"; see Issues Encountered); `task lint` clean; `task license:check` clean; `git diff --exit-code HEAD -- go.mod go.sum` clean.

---

*Phase: 05-operator-sweeps-ci-backstop*
*Completed: 2026-09-20*
