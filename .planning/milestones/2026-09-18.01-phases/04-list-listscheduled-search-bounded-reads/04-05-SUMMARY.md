---
phase: 04-list-listscheduled-search-bounded-reads
plan: 05
subsystem: database
tags: [qdrant, bounded-reads, projection, backfill, recall-maximum, red-evidence]

# Dependency graph
requires:
  - phase: 04-list-listscheduled-search-bounded-reads
    provides: "04-04: fetchPayloadsByID/includeIDs (searchfetch.go) and SearchOptions.Full — this plan's backfillNoSummaryContent is built directly on fetchPayloadsByID, and ListOptions.Full mirrors SearchOptions.Full's precedent"
provides:
  - "internal/store/store.go: ListOptions.Full, (*Store).recallView(full) — the ONE place Store.List chooses its payload projection — and isSummaryView(v), a package-level predicate over a readView's own selector"
  - "internal/store/searchfetch.go: (*Store).backfillNoSummaryContent — the shared no-summary content backfill, wired into both of Store.List's paging modes and Store.Search"
  - "internal/store/store.go: rejectOverMaximum(field, count) (D-10) — the shared FIRST-validation backstop wired into Store.List (both modes), Store.ListScheduled, Store.Search and Store.SearchDiscovery, before any filter construction or RPC; listByCursor's former silent clamp to MaxRecallLimit is deleted"
  - "internal/server/tools.go + connectapi.go: coreListRequest.Full wired through Connect ListMemories and MCP list_memory (Rule 2 fix) — full=true now also selects the store's own fetch projection, not just each transport's response shaping"
affects: [04-06, 04-07, 04-08]

# Actuals (#2632)
actuals:
  tokens: 11000
  tasks: 2
  commits: 2
  plan_head_before: 41baf7b33786ab477f906f818c974985f9d6714d

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "recallView(full bool) readView — the ONE place a recall read chooses its projection; a new read path composes this rather than calling fullView()/summaryView() by hand."
    - "isSummaryView(v readView) — a package-level predicate inspecting v's OWN selector (GetExclude() != nil), never re-deriving the caller's flag or re-calling summaryView(), so a call site gates the backfill on the view it actually selected."
    - "backfillNoSummaryContent — the ONE shared mechanism every summary-view read path calls to restore Content for a no-summary record, built on fetchPayloadsByID (04-04), never a second batched fetch."
    - "rejectOverMaximum(field, count) — a shared backstop guard, called as the FIRST validation (before any filter construction or RPC) at every recall entry point; SearchReranked needs no call of its own since candidateK(k) already bounds its actual RPC cost independent of k."

key-files:
  created:
    - internal/store/recallview_oversized_test.go
    - internal/store/recallmax_oversized_test.go
  modified:
    - internal/store/store.go
    - internal/store/searchfetch.go
    - internal/store/export_test.go
    - internal/store/listbounded_oversized_test.go
    - internal/store/listcontract_oversized_test.go
    - internal/store/store_test.go
    - internal/server/connectapi.go
    - internal/server/tools.go
    - internal/server/tools_test.go

key-decisions:
  - "recallView/isSummaryView route Store.List's projection selection only. Store.Search's own pre-existing (04-04) inline `view := s.summaryView(); if opts.Full { view = s.fullView() }` was left UNTOUCHED — the plan's own Task 1 action text only asks for a backfillNoSummaryContent call there, never a recallView migration, and touching it would have added a 4th recallView( call site against the plan's own acceptance criterion expecting exactly 3 (declaration + List's two paging modes)."
  - "isSummaryView compares the view's own selector shape (v.selector.GetExclude() != nil) rather than re-deriving via opts.Full or re-calling s.summaryView() — this keeps the literal s.summaryView() call count in store.go at 2 (recallView's own + Search's pre-existing one) instead of 3, closer to the plan's stated intent that only recallView names it."
  - "Rule 2 (missing critical functionality): wired ListOptions.Full/coreListRequest.Full through Connect ListMemories and MCP list_memory. Without it, a caller's full=true wire flag only changed each transport's SHAPING (shapeProtoMemories/shapeRecall) — the store itself kept fetching the summary view regardless, so full=true silently returned empty Citations. This is a direct violation of the plan's own MUST NOT (\"citations still appear when full records were asked for\"), caught by two pre-existing tests (TestConnectCompactViewOmitsCitations, TestSearchListMemoryCompactViewOmitsCitations)."
  - "D-10's rejectOverMaximum is a 2-parameter (field, count) helper with MaxRecallLimit baked into its own body, per the plan's literal action text — not a 3-parameter form naming the maximum at each call site. This is why store.go's literal MaxRecallLimit occurrence count (5) falls short of the plan's own acceptance criterion (>= 6): see Issues Encountered."
  - "SearchReranked needs no rejectOverMaximum call of its own: it delegates to Store.Search with candidateK(k), clamped to at most 100 regardless of k, so Search's own guard (which fires on the value it actually receives) can never see an over-maximum value through this path — documented in a comment at Search's guard rather than duplicating the check in SearchReranked."

patterns-established:
  - "Pattern (Task 1): recallView/isSummaryView/backfillNoSummaryContent — a caller-selected projection with a shared, gated compensating fetch, reusable by any future summary-view read path without re-deriving the caller's flag."
  - "Pattern (Task 2): rejectOverMaximum — a shared backstop guard called as the literal first statement (or first statement after telemetry setup) in every recall entry point, ahead of any filter construction or RPC, replacing a silent clamp with a named refusal."

requirements-completed: []
# REQ-list-bounded, REQ-search-k-bounded, REQ-list-contract-unchanged, and
# REQ-list-limit-contract-decided are all declared by sibling plans in this
# phase that have not yet finished (requirements.ready-ids returned 0/4
# ready) — none is marked complete here per the shared-ID gate (#2388).

coverage:
  - id: D1
    description: "Store.List reads the caller's projection: the summary view by default (content/citations excluded, cheap), the full view when ListOptions.Full is set; a record with no stored summary still carries its content via the shared backfill; ids/order/total/cursor behavior are unchanged by the projection"
    requirement: "REQ-list-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestRecallViewSelection (few-large, many-small)"
        status: pass
      - kind: integration
        ref: "internal/store#TestNoSummaryContentBackfill (few-large, many-small)"
        status: pass
      - kind: integration
        ref: "internal/store#TestStoreListContractInvariant, TestStoreListOffsetBounded, TestStoreListCursorBounded, TestStoreListDeepOffsetBounded (regression bundle)"
        status: pass
      - kind: unit
        ref: "internal/store#TestRecallEmissionSetIsCompleteAndClassified"
        status: pass
      - kind: integration
        ref: "internal/server (full suite)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Store.Search's summary-view fetch also gets the shared no-summary content backfill, so a search result whose record has no stored summary still shows its truncated content on the recall path exactly as before the projection existed there"
    requirement: "REQ-list-contract-unchanged"
    verification:
      - kind: integration
        ref: "internal/store#TestSearchTwoPhaseBounded, TestSearchFetchSkipsEmptyBatch (04-04, re-run green with backfillNoSummaryContent wired in — both fixtures seed with no stored summary, so every search result there already exercises the backfill path)"
        status: pass
    human_judgment: true
    rationale: "No test in this plan asserts .Content specifically on a Store.Search result restored by the backfill — only Store.List's backfill has a dedicated content assertion (TestNoSummaryContentBackfill). Search's wiring reuses the identical backfillNoSummaryContent function List's test already proves correct, and the full existing Search suite (which seeds all-no-summary fixtures) stays green with it wired in, but that suite predates this plan and does not itself assert Content restoration. Flagged for the verifier rather than silently claimed as directly proven."
  - id: D3
    description: "Every recall entry point (List in both paging modes, ListScheduled, Search, SearchDiscovery) refuses a count above store.MaxRecallLimit before any Qdrant call, naming the field and the maximum but never the rejected value; a count at the maximum succeeds; every zero-count default is unchanged; the cursor page's former silent clamp is gone, replaced by the same refusal"
    requirement: "REQ-list-limit-contract-decided"
    verification:
      - kind: integration
        ref: "internal/store#TestStoreRejectsOverMaximumCount (13 subtests: 4 entry points x at-maximum/one-above, 4 zero-default pins, 1 cursor-mode-refused)"
        status: pass
      - kind: other
        ref: "task (lint + full test suite, including ENGRAM_REQUIRE_QDRANT=1 go test ./... on a clean tree, and TestRedEvidencePatchesAreLive)"
        status: pass
    human_judgment: false
  - id: D4
    description: "A caller's full=true wire flag on Connect ListMemories and MCP list_memory also selects the store's own full-view fetch, not just each transport's response shaping — closing the gap 04-05's own projection default would otherwise have opened for full=true recall"
    requirement: "REQ-list-contract-unchanged"
    verification:
      - kind: integration
        ref: "internal/server#TestConnectCompactViewOmitsCitations, TestSearchListMemoryCompactViewOmitsCitations"
        status: pass
    human_judgment: false

# Metrics
duration: ~40min
completed: 2026-09-20
status: complete
---

# Phase 4 Plan 5: Recall Projection, No-Summary Backfill, and the Over-Maximum Backstop Summary

**`Store.List` now fetches the caller's actual projection (summary by default, full on request) with a shared backfill keeping the no-summary truncation fallback byte-identical, and every recall entry point refuses a count above the documented maximum before touching Qdrant — closing a real full=true regression the projection change would otherwise have shipped silently.**

## Performance

- **Duration:** ~40 min
- **Started:** ~2026-09-20T09:02Z (approx)
- **Completed:** 2026-09-20T09:16Z
- **Tasks:** 2
- **Files modified:** 11 (2 created, 9 modified)

## Accomplishments

- Added `ListOptions.Full`, `(*Store).recallView(full bool) readView` (the one place `Store.List` chooses its projection), and the package-level `isSummaryView(v readView)` predicate; wired `recallView(opts.Full)` into both of `Store.List`'s paging modes (the offset branch and `listByCursor`).
- Added `(*Store).backfillNoSummaryContent` (`searchfetch.go`), built on 04-04's `fetchPayloadsByID`: restores `Content` for exactly the items whose stored summary is empty, re-applying the SAME filter the page read used, returning before any RPC when every item already has a summary. Wired into both `Store.List` paging modes and `Store.Search`.
- Followed the TDD gate for Task 1: authored `recallview_oversized_test.go`'s `TestRecallViewSelection`/`TestNoSummaryContentBackfill` first, reconstructed genuine RED by temporarily reverting the three wiring call sites (never `git stash`), observed and recorded the failures below, then restored the wiring and confirmed GREEN.
- **Rule 2 fix (missing critical functionality):** wired `coreListRequest.Full` through Connect `ListMemories` and MCP `list_memory` (`internal/server/tools.go`, `connectapi.go`) — without it, `full=true` requests kept receiving empty `Citations`, since only response *shaping* consulted the wire flag while the STORE fetch stayed summary-shaped regardless. Caught by two pre-existing tests (`TestConnectCompactViewOmitsCitations`, `TestSearchListMemoryCompactViewOmitsCitations`).
- Added `rejectOverMaximum(field string, count uint64) error` (D-10): the shared backstop, called as the FIRST validation (before any filter construction or RPC) in `Store.List` (covering both paging modes with one call on `opts.Limit`), `Store.ListScheduled` (before its zero-limit default), `Store.Search` and `Store.SearchDiscovery` (on `k`). `SearchReranked` needs no call of its own — documented in a comment, since it delegates to `Search` with `candidateK(k)`, always well under the maximum.
- Deleted `listByCursor`'s clamp of a limit above `MaxRecallLimit` down to it; `Store.List`'s new guard now refuses such a request before ever reaching `listByCursor`, proven by the dedicated `List/cursor_mode_refused` subtest.
- Followed the TDD gate for Task 2: authored `recallmax_oversized_test.go`'s `TestStoreRejectsOverMaximumCount` first, reconstructed genuine RED by reverting to the Task-1-committed `store.go` (`git checkout HEAD --`, restored afterward from a saved copy — never `git stash`), observed and recorded the failures below, then restored the guards and confirmed GREEN.
- **Rule 1 fixes:** updated 5 pre-existing tests whose fixtures/assumptions the projection and backstop changes genuinely broke (see Deviations).

## Task Commits

Each task was committed atomically:

1. **Task 1: End to end — a default list reads summary-shaped payloads, and a record with no stored summary still shows its truncated content** - `99f1dc8b` (feat)
2. **Task 2: An over-maximum count is refused before any Qdrant call, on every recall entry point** - `5e22a8bc` (feat)

_No plan-metadata commit yet; STATE.md/ROADMAP.md updates follow this summary._

## RED Evidence (Task 1, TDD)

Observed against the pre-wiring code (List's offset mode and `listByCursor` still hardcoded `s.fullView()`, `Store.Search` had no backfill call) by temporarily reverting the three wiring call sites in `store.go` (restored afterward from a saved copy, never `git stash`):

```
recallview_oversized_test.go:144: few-large: default-list call N selector did not exclude content/citations — want the summary view
recallview_oversized_test.go:144: many-small: default-list call N selector did not exclude content/citations — want the summary view
--- FAIL: TestRecallViewSelection/few-large
--- FAIL: TestRecallViewSelection/many-small
recallview_oversized_test.go:276: few-large: default list with a no-summary record recorded no backfill id-set Scroll, want at least one
recallview_oversized_test.go:287: few-large: default list: with-summary record content = "extra record content, has a stored summary", want empty (content was never fetched, never backfilled)
recallview_oversized_test.go:276: many-small: default list with a no-summary record recorded no backfill id-set Scroll, want at least one
recallview_oversized_test.go:287: many-small: default list: with-summary record content = "extra record content, has a stored summary", want empty (content was never fetched, never backfilled)
--- FAIL: TestNoSummaryContentBackfill/few-large
--- FAIL: TestNoSummaryContentBackfill/many-small
```

Both failures are genuine RED (assertions for the planned behavior — the exclude-shaped selector and the backfill firing — failing on the pre-wiring always-full-view code), not `INVALID_RED`. After restoring the wiring, both test functions pass in full.

## RED Evidence (Task 2, TDD)

Observed against the Task-1-committed `store.go` (no `rejectOverMaximum`, no guard call sites, `listByCursor` still clamping) by temporarily `git checkout HEAD -- internal/store/store.go` (restored afterward from a saved copy, never `git stash`):

```
recallmax_oversized_test.go:156: errors.Is(err, store.ErrInvalidArgument) = false; err = <nil>   (x4: List, ListScheduled, Search, SearchDiscovery one-above-maximum)
recallmax_oversized_test.go:194: errors.Is(err, store.ErrInvalidArgument) = false; err = <nil>   (cursor-mode-refused)
--- FAIL: TestStoreRejectsOverMaximumCount/List/one_above_maximum
--- FAIL: TestStoreRejectsOverMaximumCount/ListScheduled/one_above_maximum
--- FAIL: TestStoreRejectsOverMaximumCount/Search/one_above_maximum
--- FAIL: TestStoreRejectsOverMaximumCount/SearchDiscovery/one_above_maximum
--- FAIL: TestStoreRejectsOverMaximumCount/List/cursor_mode_refused
```

All five failures are genuine RED (the exact planned behavior — a refusal — never happening on the pre-guard code, which instead silently succeeded or silently clamped), not `INVALID_RED`. The 8 other subtests (at-maximum successes, zero-count defaults, `SearchReranked/zero_rejected`) passed on first authoring since they exercise pre-existing, unchanged behavior — not new production behavior this task introduces. After restoring the guards, all 13 subtests pass.

## Files Created/Modified

- `internal/store/store.go` - `ListOptions.Full`; `(*Store).recallView`, `isSummaryView`; `Store.List`'s offset mode and `listByCursor` wired onto `recallView`/backfill; `Store.Search`'s backfill call; `rejectOverMaximum`; guard call sites in `List`/`ListScheduled`/`Search`/`SearchDiscovery`; `listByCursor`'s clamp deleted; `MaxRecallLimit`'s doc comment updated
- `internal/store/searchfetch.go` - `(*Store).backfillNoSummaryContent`
- `internal/store/export_test.go` - `RecallView`, `BackfillNoSummaryContent` shims
- `internal/store/recallview_oversized_test.go` (new) - `TestRecallViewSelection`, `TestNoSummaryContentBackfill`
- `internal/store/recallmax_oversized_test.go` (new) - `TestStoreRejectsOverMaximumCount`
- `internal/store/listbounded_oversized_test.go` - `TestStoreListDeepOffsetBounded`'s deep-offset call and `TestStoreListCursorBounded`'s budget-cut call both pass `Full: true` (Rule 1)
- `internal/store/listcontract_oversized_test.go` - `TestStoreListContractInvariant`'s budget-cut call passes `Full: true` (Rule 1)
- `internal/store/store_test.go` - `TestListCrossSpine`'s `Limit: 10000` lowered to `MaxRecallLimit` (Rule 1, flagged in `04-02-SUMMARY.md`/cross-plan-note as this plan's scope)
- `internal/server/tools.go` - `coreListRequest.Full` field; `deps.listMemory` and the MCP `list_memory` closure both pass it through (Rule 2)
- `internal/server/connectapi.go` - `ListMemories` handler passes `Full: req.Msg.Full` (Rule 2)
- `internal/server/tools_test.go` - `TestSearchListMemoryCompactViewOmitsCitations`'s direct `coreListRequest{}` call passes `Full: true` (Rule 1)

## Decisions Made

See key-decisions in frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] `coreListRequest.Full` wired through Connect `ListMemories` and MCP `list_memory`**
- **Found during:** Task 1's own `<verify>` (`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -count=1`)
- **Issue:** `full=true` on Connect `ListMemories` / MCP `list_memory` only changed each transport's response SHAPING (`shapeProtoMemories`/`shapeRecall`); the underlying `deps.listMemory` call never set `store.ListOptions.Full`, so once `Store.List`'s default fetch became summary-shaped (this plan), a `full=true` caller silently received empty `Citations` — a direct violation of the plan's own MUST NOT ("citations still appear when full records were asked for").
- **Fix:** Added `coreListRequest.Full`; `deps.listMemory` passes it into `store.ListOptions.Full`; both the Connect `ListMemories` handler and the MCP `list_memory` closure now set it from their own `full` flag.
- **Files modified:** `internal/server/tools.go`, `internal/server/connectapi.go`
- **Verification:** `TestConnectCompactViewOmitsCitations` and `TestSearchListMemoryCompactViewOmitsCitations` both pass; full `internal/server` suite green.
- **Committed in:** `99f1dc8b` (Task 1's commit)

**2. [Rule 1 - Bug] Three pre-existing tests updated to pass `Full: true` after Store.List's default projection changed**
- **Found during:** Task 1's own `<verify>` (the regression bundle: `TestStoreListDeepOffsetBounded`, `TestStoreListContractInvariant`, `TestStoreListCursorBounded`)
- **Issue:** `TestStoreListDeepOffsetBounded`'s deep-offset assertion ("the trailing page RPC carries the full one") and both `TestStoreListContractInvariant`'s and `TestStoreListCursorBounded`'s D-06 budget-cut subtests (which shrink the page byte budget below one FULL-view record ceiling to force a cut) all implicitly assumed `Store.List` always fetched the full view — true before this plan, no longer true by default after it.
- **Fix:** Each now passes `ListOptions.Full: true` to keep testing its own documented (non-projection) invariant; doc comments updated to record why.
- **Files modified:** `internal/store/listbounded_oversized_test.go`, `internal/store/listcontract_oversized_test.go`
- **Verification:** All three tests pass; full regression bundle green.
- **Committed in:** `99f1dc8b` (Task 1's commit)

**3. [Rule 1 - Bug] `TestSearchListMemoryCompactViewOmitsCitations`'s direct typed-core call updated**
- **Found during:** Task 1's own `<verify>` (full `internal/server` suite)
- **Issue:** This test calls `d.listMemory(ctx, c, coreListRequest{...})` directly (bypassing both transport handlers) then shapes the result with `shapeRecall(..., true, ...)`, assuming the fetched `store.Memory` already carried `Citations` — true before this plan.
- **Fix:** Added `Full: true` to its `coreListRequest{}` literal.
- **Files modified:** `internal/server/tools_test.go`
- **Verification:** Test passes; full `internal/server` suite green.
- **Committed in:** `99f1dc8b` (Task 1's commit)

**4. [Rule 1 - Bug] `TestListCrossSpine`'s `Limit: 10000` lowered to `MaxRecallLimit`**
- **Found during:** Task 2's own `<verify>` (full `internal/store`/`internal/server` suite)
- **Issue:** Flagged in advance by `04-02-SUMMARY.md` and this plan's own cross-plan note: `TestListCrossSpine` (pre-existing, `store_test.go`) passed `Limit: 10000` purely to mean "enough to return all 4 seeded records" — once Task 2's backstop landed, this became an over-maximum rejection instead of a success.
- **Fix:** Lowered to `MaxRecallLimit`, which is still far more than the 4 seeded records need.
- **Files modified:** `internal/store/store_test.go`
- **Verification:** `TestListCrossSpine` passes.
- **Committed in:** `5e22a8bc` (Task 2's commit)

**5. [Rule 1 - Bug] This plan's own `TestNoSummaryContentBackfill` exceeded the new maximum for the many-small shape**
- **Found during:** Task 2's own `<verify>` (full `internal/store` suite, run after Task 2's implementation)
- **Issue:** The many-small fixture shape seeds exactly `storetest.ManySmallRecords` (1000) records, equal to `MaxRecallLimit`; `TestNoSummaryContentBackfill` then requested `Limit: seededTotal+2` (1002) to cover two extra edge-case records, which Task 2's new backstop now refuses.
- **Fix:** Clamped the request to `min(seededTotal+2, MaxRecallLimit)` while still asserting the true (unclamped) `total`; the two extras carry a later `CreatedAt` than every fixture record, so a desc-ordered page at the clamped limit still contains both.
- **Files modified:** `internal/store/recallview_oversized_test.go`
- **Verification:** `TestNoSummaryContentBackfill` passes for both shapes.
- **Committed in:** `5e22a8bc` (Task 2's commit)

---

**Total deviations:** 5 auto-fixed (1 Rule 2 — missing critical functionality; 4 Rule 1 — bugs, all direct consequences of this plan's own intentional behavior changes)
**Impact on plan:** No scope creep. The Rule 2 fix was necessary to keep the plan's own explicit MUST NOT intact; all four Rule 1 fixes update pre-existing (or this-plan's-own) test assumptions that the plan's own intentional changes (projection default, over-maximum refusal) directly invalidated. No production behavior outside this plan's stated scope was touched.

## Issues Encountered

- **Literal-command mismatches in the plan's own acceptance criteria (same pattern documented in every prior plan's SUMMARY this phase — 04-02, 04-03, 04-04):**
  1. Task 1's acceptance criterion `rg -v '^\s*//' internal/store/store.go | rg -c -e 's[.]summaryView[(][)]'` expects `1` ("only recallView names it"), but the true count is `2`: `recallView`'s own `return s.summaryView()` plus `Store.Search`'s pre-existing (04-04) `view := s.summaryView()`. The action text explicitly scopes `recallView` wiring to `Store.List`'s two paging modes only ("Wire recallView(opts.Full) into both of Store.List's paging modes") and never asks to touch `Store.Search`'s own inline selection — only to add a `backfillNoSummaryContent` call there. Routing `Store.Search` through `recallView` too would satisfy this criterion but break the very next one (recallView( count = 3, "the declaration plus exactly the two Store.List paging-mode call sites" — explicitly excluding Search). Implemented per the action's literal instructions (Search's inline selection untouched); verified the achievable count (2, down from a naive 3 if `isSummaryView` had re-called `s.summaryView()`) and documented this as the closest satisfiable state.
  2. Task 1's acceptance criterion `rg -v '^\s*//' internal/store/store.go | rg -c -e 'Full: true'` expects `1` ("the rerank helper's forced override"), but the actual pre-existing (04-04) code reads `opts.Full = true` (an assignment, not a struct-literal field), which the literal pattern `Full: true` (with a colon) does not match at all — true count is `0`. Verified instead via `rg -n -e 'Full: true|Full = true' internal/store/store.go`, which finds exactly one line (`SearchReranked`'s `opts.Full = true`), confirming the criterion's own stated INTENT holds even though its literal grep pattern does not.
  3. Task 2's acceptance criterion `rg -v '^\s*//' internal/store/store.go | rg -c -e 'MaxRecallLimit'` expects "at least 6" ("the declaration, the zero-limit resolution, and one guard call site per entry point"), but the true count is `5`. The plan's own action text specifies `rejectOverMaximum` as a 2-parameter `(field, count)` helper with `MaxRecallLimit` baked into its own body — so the 4 call sites (`List`, `ListScheduled`, `Search`, `SearchDiscovery`) never write the literal identifier `MaxRecallLimit` themselves; it appears only in the declaration, inside `rejectOverMaximum`'s own guard condition and error message (2 more occurrences), the offset-mode zero-limit resolution, and the decoded-cursor Seen-set bound — 5 total. Implemented exactly per the action's own 2-parameter helper signature; the criterion's expectation appears to assume a call-site-visible constant reference that the specified signature does not produce.
- No blockers encountered executing either task; Docker/Qdrant was reachable throughout, and no auth gates were hit.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Every Phase 4 recall entry point now composes the shared bounded-read mechanism AND selects its projection AND refuses an over-maximum count at the store — this plan closes the store-side half of D-10; the named `out_of_range` hint and the wire-boundary rejection on the seven MCP/Connect knobs remain 04-06's scope.
- `recallView`/`isSummaryView`/`backfillNoSummaryContent` are available for any future summary-view read path; `rejectOverMaximum` is available for any future recall-count knob.
- `coreListRequest.Full` is now a real, wired field — 04-06/04-07 touching Connect/MCP list surfaces should be aware it exists and is honored.
- REQ-list-bounded, REQ-search-k-bounded, REQ-list-contract-unchanged and REQ-list-limit-contract-decided all stay **blocked** (not marked complete) pending the other plans that also declare them (`requirements.ready-ids` returned 0/4 ready) — this plan does not run `requirements.mark-complete` for any of them.
- The D2 coverage gap (no dedicated assertion that `Store.Search`'s backfill restores `Content`) is flagged above for the verifier; the shared mechanism is proven correct by `TestNoSummaryContentBackfill` (List) and Search's own suite stays green with it wired in.
- No blockers for 04-06.

---
*Phase: 04-list-listscheduled-search-bounded-reads*
*Completed: 2026-09-20*

## Self-Check: PASSED

Both created files verified present on disk: `internal/store/recallview_oversized_test.go`, `internal/store/recallmax_oversized_test.go`. Both commits (`99f1dc8b`, `5e22a8bc`) verified present in `git log --oneline --all`. Every acceptance criterion for both tasks re-run and confirmed passing at final HEAD except the three documented literal-vs-intent mismatches above (all verified against their own stated intent via scoped commands). The plan-level `<verification>` block passes in full: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... ./internal/server/... -count=1` ok (including `TestRedEvidencePatchesAreLive` on the clean committed tree — confirmed the earlier mid-edit "files already dirty" failure was a dirty-tree artifact, not a genuinely stale patch); `task` (lint + full test suite, `ENGRAM_REQUIRE_QDRANT=1`) exits 0; `go test ./internal/keylinks/ -count=1` ok; `task license:check` clean; `git diff --exit-code HEAD -- go.mod go.sum` exits 0 (no diff); `git diff --exit-code HEAD -- internal/server/summary.go` exits 0 (no shaping code touched).
