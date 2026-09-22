---
phase: 03-shared-bounded-read-mechanism-content-cap-decision
plan: 02
subsystem: store
tags: [bounded-read, byte-budget, scrollAllPoints, qdrant, D-02, D-03, D-04, D-06, D-07]

requires:
  - phase: 03-shared-bounded-read-mechanism-content-cap-decision
    provides: "plan 03-01's memoryWriteCaps (content/tags write caps) — the values plan 03-03 will wire into RecordCaps via WithRecordCaps"
provides:
  - "RecordCaps/DefaultRecordCaps/WithRecordCaps/(*Store).RecordCaps in internal/store/boundedread.go — the per-view record ceiling's configured inputs"
  - "fullRecordCeiling/summaryRecordCeiling/perRPCLimit — exact-arithmetic ceiling derivation from content+summary+tags+citations+an uncapped-fields allowance, never content alone"
  - "readView/fullView/summaryView/unbudgetedView/sweepLimit — the payload-selector-plus-ceiling bundle scrollAllPoints sizes every RPC from"
  - "scrollAllPoints(ctx, filter, view readView, fn) — the byte-derived per-RPC count and the D-07 batch-of-1 fallback, in place of the prior bare-selector signature"
affects: [03-04, 03-05, 05]

actuals:
  tokens: 10456
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "readView bundles a *qdrant.WithPayloadSelector with its derived byte ceiling so a projection can never be paired with the wrong ceiling (D-04)"
    - "package var budget seams (rpcByteBudget, pageByteBudget) mirroring spineScrollBatch's existing test-overridable-var precedent (D-06)"
    - "batch-of-1 fallback on a named sentinel match (errors.Is(err, ErrResponseTooLarge)) re-issues the SAME offset at Limit 1 rather than skipping (D-07)"

key-files:
  created:
    - internal/store/boundedread.go
    - internal/store/boundedread_test.go
    - internal/store/boundedread_oversized_test.go
  modified:
    - internal/store/store.go
    - internal/store/spine.go
    - internal/store/revert.go
    - internal/store/spine_test.go
    - internal/store/export_test.go

key-decisions:
  - "D-02/D-06 implemented with rpcByteBudget = pageByteBudget = 2<<20 (2 MiB), yielding perRPCLimit(fullRecordCeiling(DefaultRecordCaps())) = 2 and perRPCLimit(summaryRecordCeiling(DefaultRecordCaps())) = 61 — exactly the values RESEARCH.md projected."
  - "fullRecordCeiling/summaryRecordCeiling derive from ContentBytes + summaryTerm + tagsTerm + Citations*(CitationExcerptBytes+citationEntryAllowance) + uncappedFieldsAllowance (full) or summaryTerm + tagsTerm + uncappedFieldsAllowance (summary); summaryTerm falls back to ContentBytes when SummaryBytes == 0 (D-09's disabled-bound case), never silently assuming a bound that does not exist."
  - "scrollAllPoints extended in place (same name, same operatorMigrationEmitters classification) rather than a second function — every existing caller passes unbudgetedView(<its old selector>), byte-for-byte unchanged."
  - "D-07's fallback lives entirely inside scrollAllPoints's existing loop: a fallbackLeft counter forces Limit 1 for exactly the number of records the failed RPC had requested, then resumes the computed count — no new emitting function, so the recall gate needed no update."

requirements-completed: [REQ-byte-budget-pages]

coverage:
  - id: D1
    description: "A budgeted sweep sizes every Scroll from the view's record ceiling and walks both oversized fixtures at storetest.RecvLimit without overflow or loss (D-02, D-03, D-04)"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: integration
        ref: "internal/store/boundedread_oversized_test.go#TestScrollAllPointsByteBudget"
        status: pass
    human_judgment: false
  - id: D2
    description: "A legacy over-cap record is re-read one record at a time (batch-of-1 fallback) rather than skipped; a single record still over the receive limit fails the sweep with the named store.ErrResponseTooLarge (D-07)"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: integration
        ref: "internal/store/boundedread_oversized_test.go#TestScrollAllPointsBatchOfOneFallback"
        status: pass
      - kind: integration
        ref: "internal/store/boundedread_oversized_test.go#TestScrollAllPointsSingleOversizedRecordFailsNamed"
        status: pass
    human_judgment: false
  - id: D3
    description: "The derived per-record ceilings are exact integers from the configured caps, and hold empirically for a record written at every default cap in both views"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: unit
        ref: "internal/store/boundedread_test.go#TestRecordCeilingDerivesFromCaps"
        status: pass
      - kind: integration
        ref: "internal/store/boundedread_oversized_test.go#TestRecordCeilingHoldsForMaxCapRecord"
        status: pass
    human_judgment: false
  - id: D4
    description: "WithRecordCaps normalizes non-positive fields to DefaultRecordCaps() except SummaryBytes: 0 (bound disabled); sweepLimit's own branching (unbudgeted vs. spineScrollBatch-capped budgeted) is correct"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: unit
        ref: "internal/store/boundedread_test.go#TestWithRecordCapsNormalizesNonPositive"
        status: pass
      - kind: unit
        ref: "internal/store/boundedread_test.go#TestSweepLimit"
        status: pass
    human_judgment: false
  - id: D5
    description: "Every existing scrollAllPoints caller (ScanSpine, EnumerateCitations, NearDuplicates, derivePurgeEligible, previewRevertWithSteps, spine_test.go's snapshotCollection) keeps today's exact behavior via unbudgetedView; the recall gate and Phase 1/2 regressions stay green"
    requirement: "REQ-byte-budget-pages"
    verification:
      - kind: integration
        ref: "internal/store TestRecallEmissionSetIsCompleteAndClassified"
        status: pass
      - kind: integration
        ref: "internal/store TestListScopesFullPayloadsOverGRPCLimit"
        status: pass
      - kind: integration
        ref: "internal/store TestStoreListOverflowIsResponseTooLarge"
        status: pass
      - kind: integration
        ref: "internal/store TestPurgeDerivationPaginatesEveryPage"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-09-19
status: complete
---

# Phase 3 Plan 2: Byte-Budget Sweep Primitive Summary

**`scrollAllPoints` now sizes every `ScrollAndOffset` RPC from a per-view record ceiling (2/61 records per RPC for full/summary views at the default caps) instead of a bare 256-record batch, with a batch-of-1 fallback for legacy over-cap records that fails loudly (never silently) when a single record still overflows.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-09-19T20:35:00Z
- **Completed:** 2026-09-19T21:00:28Z
- **Tasks:** 2 completed
- **Files modified:** 8 (3 created, 5 modified)

## Accomplishments

- Built `internal/store/boundedread.go`: `RecordCaps`/`DefaultRecordCaps`/`WithRecordCaps`/`(*Store).RecordCaps`, `fullRecordCeiling`/`summaryRecordCeiling`/`perRPCLimit` (exact-arithmetic ceiling derivation — content + summary/content-fallback + tags + citations + a documented allowance for every payload field with no write cap), `rpcByteBudget`/`pageByteBudget` (2 MiB `var` seams), and `readView`/`fullView`/`summaryView`/`unbudgetedView`/`sweepLimit`.
- Extended `scrollAllPoints` (`spine.go`) in place — same name, same `operatorMigrationEmitters` classification, no recall-gate change needed — to size every `ScrollAndOffset` request's `Limit` from `sweepLimit(view)`. Every existing caller (`ScanSpine`, `EnumerateCitations`, `NearDuplicates`' id enumeration, `derivePurgeEligible`, `revert.go`'s `previewRevertWithSteps`, `spine_test.go`'s `snapshotCollection`) now passes `unbudgetedView(<its old selector>)`, preserving today's exact `spineScrollBatch`-per-RPC behavior byte-for-byte.
- Added the D-07 batch-of-1 fallback: on a budgeted view whose computed-`Limit` RPC fails with the named `ErrResponseTooLarge` sentinel, the SAME offset position is re-issued at `Limit: 1` for exactly that many RPCs before resuming the computed count. A single record still overflowing at `Limit: 1` returns the sentinel unchanged — never silently skipped or truncated.
- Proved the whole mechanism against real Qdrant at `storetest.RecvLimit` (4 MiB): `TestScrollAllPointsByteBudget` (both fixture shapes × both views, via a recording gRPC interceptor asserting our own request/response shape — rule `m45p2b4bp7`), `TestScrollAllPointsBatchOfOneFallback` and `TestScrollAllPointsSingleOversizedRecordFailsNamed` (legacy over-cap records written via a raw `Store.Upsert`), and `TestRecordCeilingHoldsForMaxCapRecord` (one record at every default cap, in both views).
- Pinned the exact derivation with no-Qdrant unit tests: `TestRecordCeilingDerivesFromCaps` (970240/34304 full/summary ceilings at defaults; 1035264/99328 with `SummaryBytes: 0`; `perRPCLimit` == 1 when `ContentBytes` is `4<<20`), `TestWithRecordCapsNormalizesNonPositive`, `TestSweepLimit`.

## Task Commits

Each task was committed atomically:

1. **Task 1: End to end — a budgeted sweep sizes every Scroll from the view's record ceiling and walks both oversized fixtures at the named 4 MiB limit** - `b7bdf6f8` (feat)
2. **Task 2: Legacy over-cap records — batch-of-1 fallback, the named failure, and the provable ceiling** - `e3d5245f` (feat)

_Both tasks carried `tdd="true"` (Task 1 additionally `type="tracer"`); each RED was observed by a temporary revert (Task 1: forced `sweepLimit` to return `spineScrollBatch`; Task 2: the fallback branch simply did not exist yet), confirming the target test failed for the expected reason, then restoring/adding the fix and re-confirming GREEN. See "TDD Gate Compliance" below._

## Files Created/Modified

- `internal/store/boundedread.go` (new) - `RecordCaps`, `DefaultRecordCaps`, `WithRecordCaps`, `(*Store).RecordCaps`, budget vars, ceiling/allowance constants, `summaryTerm`/`tagsTerm`/`fullRecordCeiling`/`summaryRecordCeiling`/`perRPCLimit`, `readView`/`fullView`/`summaryView`/`unbudgetedView`/`sweepLimit`
- `internal/store/store.go` - added `caps *RecordCaps` field to `Store` (the only change; both `store.go`-targeting red-evidence patches still `git apply --check` clean afterward)
- `internal/store/spine.go` - `scrollAllPoints`'s signature and body extended for the `readView` parameter and the D-07 fallback; its five in-package/cross-file callers updated to `unbudgetedView(...)`
- `internal/store/revert.go` - `previewRevertWithSteps` updated to `unbudgetedView(qdrant.NewWithPayload(true))`
- `internal/store/spine_test.go` - `snapshotCollection` updated to `unbudgetedView(qdrant.NewWithPayload(true))`
- `internal/store/export_test.go` - `ReadView`/`FullView`/`SummaryView`/`ViewMaxRecordBytes`/`SweepLimit`/`RPCByteBudget`/`PageByteBudget`/`ScrollAllPoints` shims for `package store_test`
- `internal/store/boundedread_test.go` (new, package `store`) - `TestRecordCeilingDerivesFromCaps`, `TestWithRecordCapsNormalizesNonPositive`, `TestSweepLimit`
- `internal/store/boundedread_oversized_test.go` (new, package `store_test`) - `scrollRecorder`, `TestScrollAllPointsByteBudget`, `TestScrollAllPointsBatchOfOneFallback`, `TestScrollAllPointsSingleOversizedRecordFailsNamed`, `TestRecordCeilingHoldsForMaxCapRecord`

## Decisions Made

See `key-decisions` in frontmatter (D-02/D-06 budget values, the ceiling formula, in-place `scrollAllPoints` extension, and the fallback's placement inside the existing loop). All decisions were pre-locked in `03-CONTEXT.md`; no new architectural decision was made in this plan — every numeric choice (allowance constants, the 2 MiB budgets) was Claude's discretion within D-02/D-06's stated bounds, and the empirical test (`TestRecordCeilingHoldsForMaxCapRecord`) confirmed the chosen allowances hold without needing adjustment.

## Deviations from Plan

None - plan executed exactly as written. All acceptance criteria (the `rg -o` counts, the `store.go`-diff `git diff` check, both red-evidence `git apply --check` calls, `go vet`, `golangci-lint`, `task license:check`, `git diff --exit-code -- go.mod go.sum`) pass exactly as specified in each task.

One micro-adjustment during Task 2's acceptance-criteria pass: the `scrollAllPoints` doc comment originally quoted the literal expression `errors.Is(err, ErrResponseTooLarge)` in prose, which made the acceptance grep (`rg -o 'errors[.]Is[(]err, ErrResponseTooLarge[)]'`) count 2 matches (the doc comment plus the real code) instead of the required 1. Reworded the doc comment to describe the match without repeating the literal expression — no behavior change, doc-only.

## TDD Gate Compliance

| Task | RED observed | GREEN restored | Commit scope |
|------|---------------|-----------------|---------------|
| 1 | Yes — `sweepLimit` temporarily forced to always return `spineScrollBatch`; `TestScrollAllPointsByteBudget` failed: `SweepLimit(FullView()) = 256, want 2`, `SweepLimit(SummaryView()) = 256, want 61`, and the few-large/full subtest genuinely overflowed real Qdrant (`ScrollAndOffset() failed: ... exceeded the client's receive limit ... larger than max 4194304`, classified as `store.ErrResponseTooLarge`) | Yes — restored `sweepLimit`'s byte-derived branch; all 4 shape/view subtests pass | `feat(store)` — signature extension + ceiling derivation + test landed together per the plan's Task 1 action; RED/GREEN observed via a scoped temporary revert (never `git stash`) rather than a separate RED commit |
| 2 | Yes — `TestScrollAllPointsBatchOfOneFallback` written and run before the fallback branch existed: `ScrollAndOffset() failed: ... exceeded the client's receive limit ... larger than max 4194304` (the plain overflow, uncaught) | Yes — added the `fallbackLeft` branch inside `scrollAllPoints`; the same test, plus `TestScrollAllPointsSingleOversizedRecordFailsNamed` and `TestRecordCeilingHoldsForMaxCapRecord`, all pass | `feat(store)` |

Each RED was produced by a scoped `Edit`/re-`Edit` revert-and-restore cycle (never `git stash`), confirmed via `go build`/the target test's `-v` output, then finalized before each task's single commit.

## Issues Encountered

None.

## Known Stubs

None. No hardcoded empty values, placeholder text, or unwired data sources were introduced.

## Threat Flags

None beyond this plan's own `<threat_model>` register (T-03-02-01 through T-03-02-04, T-03-02-SC), which already covers every new surface this plan introduces (the byte-budget sweep, the batch-of-1 fallback, the ceiling derivation, and the unmigrated-caller behavior-preservation guarantee).

## User Setup Required

None - no external service configuration required. `WithRecordCaps` and the two budget `var` seams have documented defaults; plan 03-03 wires the production `RecordCaps` from plan 03-01's config-derived caps.

## Next Phase Readiness

- The byte-budget sweep primitive (`scrollAllPoints`'s `readView` extension, `sweepLimit`, the batch-of-1 fallback) is proven against both `storetest.SeedOversized` shapes and both payload views, and is ready for plan 03-04's ordered-page helper to reuse the same ceiling/budget constants.
- Plan 03-03 still needs to wire `WithRecordCaps` with the production config-derived caps (plan 03-01's `memoryWriteCaps`) — this plan's `RecordCaps`/`DefaultRecordCaps` exist and are tested, but nothing in production calls `WithRecordCaps` yet (a `Store` built without it silently uses `DefaultRecordCaps()`, which is the correct interim default, not a gap).
- Migrating `scrollAllPoints`'s five existing callers off `unbudgetedView` onto `fullView`/`summaryView` is explicitly Phase 5's job, not this plan's — every current caller behaves identically to before this plan.
- No blockers.

---
*Phase: 03-shared-bounded-read-mechanism-content-cap-decision*
*Completed: 2026-09-19*

## Self-Check: PASSED

- All 8 key files (3 created + 5 modified) confirmed present on disk.
- Both task commit hashes (`b7bdf6f8`, `e3d5245f`) confirmed present in `git log --oneline --all`.
- `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1`: `ok` (includes the recall gate, Phase 1/2 regressions, and `TestRedEvidencePatchesAreLive`).
- `go test ./internal/store/... -short -count=1`: `ok`.
- `go test ./internal/keylinks/ -run TestNoEscapedPatternsRepoWide -count=1`: `ok`.
- `task lint` / `task license:check` / `git diff --exit-code HEAD -- go.mod go.sum`: all clean.
- `go vet ./internal/store/...` / `golangci-lint run ./internal/store/...`: clean (0 issues).
