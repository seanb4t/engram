---
phase: 03-shared-bounded-read-mechanism-content-cap-decision
plan: 04
subsystem: store
tags: [ordered-page, keyset-paging, byte-budget, qdrant, D-02, D-03, D-06, D-07, D-08]

requires:
  - phase: 03-shared-bounded-read-mechanism-content-cap-decision
    provides: "plan 03-02's boundedread.go (RecordCaps, readView, fullView/summaryView, perRPCLimit, rpcByteBudget/pageByteBudget) — the per-view ceiling and budgets this plan's ordered-page helper reuses, and plan 03-03's production RecordCaps wiring"
provides:
  - "orderedPage{Items, Next, Exhausted, CutByBudget, Bytes} and (*Store).scrollOrderedPage in internal/store/orderedpage.go — the ordered half of the shared bounded-read mechanism, generalizing listByCursor's keyset paging to several small Scroll RPCs per logical page"
  - "excludeSeen(f, ids) — the pure has_id/must_not tie-exclusion filter builder that never widens the caller's authz filter"
  - "The D-08 call-site inventory (03-INVENTORY.md): all 27 Qdrant read sites in internal/store assigned to Phase 4, Phase 5, a Phase 3 primitive, or a written exemption, with Phase 5's closing-check commands"
affects: [04, 05]

actuals:
  tokens: 11143
  tasks: 3
  commits: 3
  plan_head_before: 25ef372232e7ae4bc2334ae6555587ae7c95fa3d

tech-stack:
  added: []
  patterns:
    - "scrollOrderedPage generalizes listByCursor's start_from + boundary-seen-set idiom to several small RPCs per logical page, resetting the seen set and StartFrom every time a point's created_at differs from the current boundary — sub-RPC-granular tie tracking, not just per-page"
    - "excludeSeen wraps the caller's filter as a single nested Must condition and adds only a MustNot has_id exclusion — narrows, never widens, the caller's authz filter"
    - "D-07's batch-of-1 fallback is implemented identically to plan 03-02's scrollAllPoints: an override branch computed before the normal n derivation, so a legacy-oversized-record retry bypasses the budget-cut check entirely until it is spent"

key-files:
  created:
    - internal/store/orderedpage.go
    - internal/store/orderedpage_oversized_test.go
    - .planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-INVENTORY.md
  modified:
    - internal/store/export_test.go
    - internal/store/schemaversion_recallgate_test.go

key-decisions:
  - "Classified Store.scrollOrderedPage in otherNonRecallEmitters (not recallTransmitters) — it is not yet wired into any recall entry point in this phase; Phase 4 reclassifies it in the same change that wires Store.List/listByCursor/ListScheduled onto it, per the recall gate's reachability subtest."
  - "The page-cut-short signal is exposed explicitly (CutByBudget/Exhausted/Next) rather than absorbed internally (RESEARCH.md's shape 1, not shape 2) — required because D-02's page budget makes a short, non-final page unavoidable, and Phase 4 must be able to see it to preserve REQ-list-contract-unchanged."
  - "D-07's fallback and input validation were deliberately deferred to Task 2 (not written alongside Task 1's core loop) so each task's own RED/GREEN cycle proved exactly the behavior that task's commit introduces — no code from a later task's scope leaked into an earlier commit."

requirements-completed: [REQ-byte-budget-pages]

coverage:
  - id: D1
    description: "scrollOrderedPage assembles one logical created_at-ordered page from several small Scroll RPCs, each sized from the view's byte-derived per-RPC ceiling and the page's still-remaining byte budget, and proves the contract against both storetest.SeedOversized shapes at storetest.RecvLimit in both views (D-02, D-03)"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: integration
        ref: "internal/store/orderedpage_oversized_test.go#TestScrollOrderedPageByteBudget"
        status: pass
    human_judgment: false
  - id: D2
    description: "A page cut short by the byte budget is never reported as exhausted (CutByBudget and Exhausted are never both true), and the resume position always advances — proven at the general case, at a page budget smaller than one ceiling, at a limit reached with budget left, and at an exact-final-page boundary"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: integration
        ref: "internal/store/orderedpage_oversized_test.go#TestScrollOrderedPageBudgetBoundaries"
        status: pass
    human_judgment: false
  - id: D3
    description: "Keyset resume uses Qdrant's documented start_from + must_not has_id mechanism, never Offset combined with OrderBy; the has_id exclusion only ever narrows the caller's filter (another owner's private records at the same timestamp never leak in); ties spanning RPC boundaries within one page are resolved correctly; ascending order works the same way"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: integration
        ref: "internal/store/orderedpage_oversized_test.go#TestScrollOrderedPageTiesAcrossRPCBoundaries"
        status: pass
      - kind: integration
        ref: "internal/store/orderedpage_oversized_test.go#TestScrollOrderedPageAscending"
        status: pass
    human_judgment: false
  - id: D4
    description: "A legacy over-cap record triggers the batch-of-1 fallback and is returned exactly once; a single record still over the receive limit fails the call with store.ErrResponseTooLarge and zero items, never silently skipped (D-07)"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: integration
        ref: "internal/store/orderedpage_oversized_test.go#TestScrollOrderedPageBatchOfOneFallback"
        status: pass
    human_judgment: false
  - id: D5
    description: "Invalid input (zero limit, an unbudgeted view, a Seen set with no boundary, an oversized Seen set) is rejected with store.ErrInvalidArgument before any RPC, and the new helper is classified in the recall gate without disturbing existing classifications or listByCursor/List/ListScheduled"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: integration
        ref: "internal/store/orderedpage_oversized_test.go#TestScrollOrderedPageRejectsInvalidInput"
        status: pass
      - kind: integration
        ref: "internal/store TestRecallEmissionSetIsCompleteAndClassified"
        status: pass
    human_judgment: false
  - id: D6
    description: "The D-08 call-site inventory lists every one of the 27 Qdrant read sites in internal/store, assigned to Phase 4/5/a Phase 3 primitive/an exemption, with the mechanical commands Phase 5's closing check re-runs — mechanically verified to equal the derived set"
    requirement: "REQ-bounded-read-mechanism"
    verification:
      - kind: other
        ref: "verify.sh derived-set-vs-listed-set check (03-04-PLAN.md verify block)"
        status: pass
    human_judgment: false

duration: 38min
completed: 2026-09-19
status: complete
---

# Phase 3 Plan 4: Ordered-Page Primitive & Call-Site Inventory Summary

**`scrollOrderedPage` generalizes `listByCursor`'s keyset paging to several small Scroll RPCs per logical page, each stopping on a per-view byte-derived count and the page's remaining byte budget, with a D-07 batch-of-1 legacy fallback — and the D-08 inventory assigns all 27 Qdrant read sites in `internal/store` to their migrating phase or a written exemption.**

## Performance

- **Duration:** 38 min
- **Started:** 2026-09-19T21:25:00Z
- **Completed:** 2026-09-19T22:03:22Z
- **Tasks:** 3 completed
- **Files modified:** 5 (3 created, 2 modified)

## Accomplishments

- Built `internal/store/orderedpage.go`: `orderedPage{Items, Next, Exhausted, CutByBudget, Bytes}`, `excludeSeen` (pure `has_id`/`must_not` filter builder that only ever narrows the caller's filter), and `(*Store).scrollOrderedPage` — issuing one or more `Scroll` RPCs per logical page, each sized from `min(perRPCLimit(view.maxRecordBytes), want, remainingPageBudget/maxRecordBytes)`, tracking a boundary/seen-set that resets every time a received point's `created_at` advances (sub-RPC-granular tie tracking).
- Proved the byte-budget contract against real Qdrant at `storetest.RecvLimit` over both `storetest.SeedOversized` shapes and both views (`TestScrollOrderedPageByteBudget`): full-view's first page is always cut by budget; summary-view's first page holds the whole fixture and the next call reports `Exhausted` with zero items; no id is ever visited twice; `CreatedAt` never increases; every recorded `Scroll` has `1 <= Limit <= PerRPCLimit(view)` and no response exceeds `RPCByteBudget()`.
- Added D-07's batch-of-1 fallback (mirroring plan 03-02's `scrollAllPoints`) and input validation (`limit == 0`, an unbudgeted view, a `Seen` set with no boundary, a `Seen` set over `maxListLimit`) — each rejected with `store.ErrInvalidArgument` before any RPC.
- Proved tie-safety across RPC boundaries (seven records at one `created_at`, three at the prior second, two private other-owner records at the tie point, summary-view RPCs shrunk to 2 records via `SetByteBudgets`), ascending order, every named budget boundary (a page budget smaller than one ceiling, a limit reached with budget left, an exact-final-page), and the legacy-record fallback (a batch-of-1 retry that succeeds, and a single record still over `storetest.RecvLimit` that fails named).
- Classified `Store.scrollOrderedPage` in `otherNonRecallEmitters` (not yet wired into any recall entry point) — `TestRecallEmissionSetIsCompleteAndClassified` and the other two recall-gate tests stay green, and `listByCursor`/`List`/`cursor.go` were confirmed byte-for-byte untouched.
- Recorded `03-INVENTORY.md` (D-08): all 27 Qdrant read sites in `internal/store`, each assigned to Phase 4 (5 recall paths), Phase 5 (10 sweeps), a Phase 3 primitive (`scrollAllPoints`/`scrollOrderedPage`), or a written exemption (10 sites) — the derived-set-vs-listed-set equality is mechanically verified, and the table names all five current `unbudgetedView` callers of `scrollAllPoints` that Phase 5 must migrate.

## Task Commits

Each task was committed atomically:

1. **Task 1: End to end — an ordered page of a few very large records stops on the byte budget and resumes with nothing lost** - `39b8e256` (feat, tracer, tdd)
2. **Task 2: The ordered page's edges — ties across RPC boundaries, ascending order, budget boundaries, legacy fallback, invalid input** - `ddf6355f` (feat, tdd)
3. **Task 3: Record the D-08 call-site inventory that Phase 5's closing check compares against** - `fe45ce43` (docs)

_Tasks 1 and 2 carried `tdd="true"` (Task 1 additionally `type="tracer"`); each RED was observed by a temporary scoped revert to the prior commit's file state (never `git stash`), confirming the target test(s) failed for the expected reason, then restoring/adding the fix and re-confirming GREEN before the single commit. See "TDD Gate Compliance" below._

## Files Created/Modified

- `internal/store/orderedpage.go` (new) - `orderedPage`, `excludeSeen`, `sortedSeenIDs`, `(*Store).scrollOrderedPage` (keyset resume, per-RPC/page budget sizing, D-07 fallback, input validation)
- `internal/store/orderedpage_oversized_test.go` (new, package `store_test`) - `TestScrollOrderedPageByteBudget`, `TestScrollOrderedPageTiesAcrossRPCBoundaries`, `TestScrollOrderedPageAscending`, `TestScrollOrderedPageBudgetBoundaries`, `TestScrollOrderedPageBatchOfOneFallback`, `TestScrollOrderedPageRejectsInvalidInput`, plus `seedFiveOrderedRecords`/`tooManySeenIDs` helpers
- `internal/store/export_test.go` - `OrderedPage`/`ListCursor` aliases, `(*Store).ScrollOrderedPage`, `PerRPCLimit`, `UnbudgetedView`, `SetByteBudgets` test shims
- `internal/store/schemaversion_recallgate_test.go` - added `Store.scrollOrderedPage` to `otherNonRecallEmitters`; reworded the list's header comment to restate no count (mirroring `operatorMigrationEmitters`)
- `.planning/phases/03-shared-bounded-read-mechanism-content-cap-decision/03-INVENTORY.md` (new) - the D-08 call-site inventory and Phase 5's closing-check commands

## Decisions Made

See `key-decisions` in frontmatter (the `otherNonRecallEmitters` classification per the recall gate's reachability rule, the explicit page-cut-short signal per REQ-list-contract-unchanged, and the Task 1/Task 2 scope split so each commit's RED/GREEN cycle proved exactly what it introduced). All three were within Claude's Discretion per `03-CONTEXT.md`; no new architectural decision was made.

## Deviations from Plan

None - plan executed exactly as written. All acceptance criteria across all three tasks (the `rg -o` counts, the `store.go`/`cursor.go` untouched-diff check, the derived-set-vs-listed-set inventory check, `go vet`, `golangci-lint`, `task license:check`, `git diff --exit-code -- go.mod go.sum`) pass exactly as specified in each task.

## TDD Gate Compliance

| Task | RED observed | GREEN restored | Commit scope |
|------|---------------|-----------------|---------------|
| 1 | Yes — with the per-RPC count clamped only to `min(perRPCLimit, want)` (no remaining-page-budget clamp), `TestScrollOrderedPageByteBudget`'s `full` subtests failed: `page 0: Bytes = 5266400, exceeds PageByteBudget 2097152` and `full: first page CutByBudget = false, want true` (both `few-large` and `many-small`) | Yes — restored the `byBudget := (pageByteBudget - totalBytes) / view.maxRecordBytes` clamp; all 4 shape/view subtests pass | `feat(store)` — test + fix landed together per the plan's Task 1 action; RED genuinely observed via a scoped temporary edit (never `git stash`), confirmed via `-v` output, then reverted to the fix before the single commit |
| 2 | Yes — with Task 1's committed code (no validation, no fallback) restored temporarily and Task 2's five new tests present: `--- FAIL: TestScrollOrderedPageBatchOfOneFallback/legacy-window` and `--- FAIL: TestScrollOrderedPageRejectsInvalidInput/zero_limit` (and `/unbudgeted_view`) | Yes — restored the validation block and the `fallbackLeft`-driven batch-of-1 branch; all 6 top-level `TestScrollOrderedPage*` tests pass, plus the full `internal/store/...` suite under `ENGRAM_REQUIRE_QDRANT=1` | `feat(store)` |

Each RED was produced by copying the prior commit's file (`git show <hash>:<path>`) over the working file, running the target test(s), confirming the expected failure, then restoring the GREEN implementation via `cp` from a saved copy and re-confirming — never `git stash`, never a partial commit.

## Issues Encountered

One self-caught deviation during Task 2's acceptance-criteria pass: the file doc comment's D-05 paragraph originally quoted the literal string `payload_bytes` verbatim, which made the acceptance grep (`rg -o 'payload_bytes' internal/store`) count 1 match (the comment) instead of the required 0 (mirroring exactly the same self-match pitfall plan 03-02's own SUMMARY records for `ErrResponseTooLarge`). Reworded the comment to describe the field without repeating the literal token — no behavior change, doc-only, folded into Task 2's commit before it landed (never a separate fix commit).

## Known Stubs

None. No hardcoded empty values, placeholder text, or unwired data sources were introduced.

## Threat Flags

None beyond this plan's own `<threat_model>` register (T-03-04-01 through T-03-04-04, T-03-04-SC), which already covers every new surface this plan introduces (the byte-derived per-RPC count, the tie-exclusion filter's narrow-only guarantee, the explicit cut/exhausted signal, and the recall-gate classification).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Both shared bounded-read primitives this milestone needs (`scrollAllPoints`'s byte-budget extension from plan 03-02, and this plan's `scrollOrderedPage`) are built, proven against both oversized fixture shapes, and ready for Phase 4 to wire `Store.List`/`Store.listByCursor`/`Store.ListScheduled` onto `scrollOrderedPage` (reclassifying it into `recallTransmitters` in that same change) and `Store.Search`/`Store.SearchDiscovery` onto bounded reads.
- The D-08 inventory (`03-INVENTORY.md`) is complete and mechanically verified against the derived call-site set — Phase 4 and Phase 5 both have their exact migration checklist, and Phase 5's closing check has its exact re-run commands.
- Phase 3's own red-evidence patches are registered by plan 03-06 (not this plan) per the phase's executor notes.
- No blockers.

---
*Phase: 03-shared-bounded-read-mechanism-content-cap-decision*
*Completed: 2026-09-19*

## Self-Check: PASSED

- All 5 key files (3 created + 2 modified) confirmed present on disk.
- All 3 task commit hashes (`39b8e256`, `ddf6355f`, `fe45ce43`) confirmed present in `git log --oneline --all`.
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1`: `ok` (includes the recall gate, Phase 1/2 regressions, and `TestRedEvidencePatchesAreLive`).
- `go test ./internal/store/... -short -count=1`: `ok`.
- `go test ./internal/keylinks/ -run TestNoEscapedPatternsRepoWide -count=1`: `ok`.
- `task lint` / `task license:check` / `git diff --exit-code HEAD -- go.mod go.sum`: all clean.
- `go vet ./internal/store/...` / `golangci-lint run ./internal/store/...`: clean (0 issues).
