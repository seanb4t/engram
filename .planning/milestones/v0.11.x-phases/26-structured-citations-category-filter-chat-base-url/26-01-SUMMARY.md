---
phase: 26-structured-citations-category-filter-chat-base-url
plan: 01
subsystem: database
tags: [go, qdrant, store, search, category-filter]

# Dependency graph
requires:
  - phase: 25-supersession-with-history
    provides: soft-hide superseded-record recall gate (Store.Search's superseded_by NewIsEmpty condition), unchanged by this plan
provides:
  - store.SearchOptions{Tags, Categories, CreatedAfter, CreatedBefore} replacing the trailing positional params on Store.Search/SearchReranked
  - categoryMatchCondition shared OR-condition helper, used by both listFilter and Search
  - coreSearchRequest.Categories threaded end-to-end from deps.searchMemory
affects: [26-02, 26-03, 26-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "store.SearchOptions struct mirrors ListOptions' field-of-filters convention, keeping k a positional param deliberately excluded from the struct (D-09)"
    - "categoryMatchCondition extracted as a shared OR-condition builder consumed by both the list and search filter-assembly paths, preventing the two lanes from drifting"

key-files:
  created: []
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/store/service_principal_isolation_test.go
    - internal/store/rerank_test.go
    - internal/store/instrument_test.go
    - internal/retrievaleval/retrieval_eval_test.go
    - internal/server/tools.go
    - internal/server/store_iface.go
    - internal/server/fakestore_test.go

key-decisions:
  - "D-09 confirmed: SearchOptions{Tags, Categories, CreatedAfter, CreatedBefore} replaces the 9th/10th/11th positional params on Search/SearchReranked; k stays positional so SearchReranked's k==0 ErrInvalidArgument caller-discipline guard is unweakened"
  - "listFilter's inline per-category Should-loop extracted into categoryMatchCondition, with one deliberate flagged tightening: empty-string category elements are now skipped (mirroring tagMatchConditions), so a categories:[\"\"] list request becomes a passthrough instead of a filter matching nothing"

patterns-established:
  - "Pattern: any new Store.Search/List request filter composes as a peer condition appended to f.Must strictly AFTER ownerScopeFilter/ownerOrSharedCondition has established the outer authz constraint — never reordered ahead of it (SC4 invariant, enforced by TestCategoryFilterDoesNotWidenVisibility)"

requirements-completed: [REQ-category-filter]

coverage:
  - id: D1
    description: "Store.Search/SearchReranked accept SearchOptions.Categories and filter results with OR semantics as a hard Qdrant pre-filter"
    requirement: "REQ-category-filter"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestSearchCategoryFilter"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestSearchCategoryFilterPreRanking"
        status: pass
    human_judgment: false
  - id: D2
    description: "categoryMatchCondition helper: nil/empty/all-empty-string passthrough, OR-composed non-nil condition for mixed input"
    requirement: "REQ-category-filter"
    verification:
      - kind: unit
        ref: "internal/store/store_test.go#TestCategoryMatchConditionEdges"
        status: pass
    human_judgment: false
  - id: D3
    description: "Category filter cannot widen visibility: ownerOrSharedCondition stays the outer authz Must; a shared-readable record stays non-writable by a non-owner"
    requirement: "REQ-category-filter"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestCategoryFilterDoesNotWidenVisibility"
        status: pass
    human_judgment: false
  - id: D4
    description: "A category filter matching every record in scope does not reorder Search's result list"
    requirement: "REQ-category-filter"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestSearchCategoryFilterOrderingUnchanged"
        status: pass
    human_judgment: false
  - id: D5
    description: "coreSearchRequest.Categories threaded from deps.searchMemory into store.SearchOptions — the one production caller of SearchReranked"
    requirement: "REQ-category-filter"
    verification:
      - kind: integration
        ref: "internal/server (go test ./internal/server/...)"
        status: pass
    human_judgment: false

duration: 10min
completed: 2026-07-25
status: complete
---

# Phase 26 Plan 01: Category Filter Threaded Through Store.Search Summary

**`store.SearchOptions{Tags, Categories, CreatedAfter, CreatedBefore}` replaces Search/SearchReranked's positional tail, with a shared `categoryMatchCondition` OR-helper generalized out of `listFilter` so the list and search lanes cannot drift.**

## Performance

- **Duration:** ~10 min
- **Completed:** 2026-07-25T23:09:23Z
- **Tasks:** 2 completed
- **Files modified:** 9

## Accomplishments
- `Store.Search`/`Store.SearchReranked` reshaped onto `SearchOptions` (D-09); `k` deliberately kept positional so `SearchReranked`'s `k==0` `ErrInvalidArgument` guard is preserved verbatim.
- `categoryMatchCondition` extracted from `listFilter`'s inline category block and shared by both the list and search filter-assembly paths — OR semantics, D-11 no-allowlist, empty-string-element skipping mirroring `tagMatchConditions`.
- `coreSearchRequest.Categories` threaded end-to-end from `deps.searchMemory` (the one production caller of `SearchReranked`) down to the Qdrant `QueryPoints.Filter`.
- Five new store tests prove the OR semantics, the pre-ranking guarantee (SC2), the SC4/D-16 authz invariant (category filter cannot widen visibility, and a shared-readable record is not writable by a non-owner), and the ordering-unchanged edge.
- All ~25 call sites across `internal/store` test files and `internal/retrievaleval` updated to the new signature; `go vet ./...` is the exhaustive proof.

## Task Commits

Each task was committed atomically:

1. **Task 1: Thread one category filter end-to-end through SearchOptions (D-09)** - `8e2f5daf` (feat)
2. **Task 2: Prove pre-ranking, the SC4 authz invariant, and the ordering edge** - `c8482bb0` (test)

## Files Created/Modified
- `internal/store/store.go` - `SearchOptions` struct, `categoryMatchCondition` helper, reshaped `Search`/`SearchReranked`, `listFilter` refactored onto the shared helper
- `internal/store/store_test.go` - `TestSearchCategoryFilter`, `TestCategoryMatchConditionEdges`, `TestSearchCategoryFilterPreRanking`, `TestCategoryFilterDoesNotWidenVisibility`, `TestSearchCategoryFilterOrderingUnchanged`; ~20 existing `s.Search`/`SearchReranked` call sites updated to `SearchOptions{...}`
- `internal/store/service_principal_isolation_test.go` - 4 call sites updated
- `internal/store/rerank_test.go` - `SearchReranked` call site updated; unused `time` import removed
- `internal/store/instrument_test.go` - 2 call sites updated
- `internal/retrievaleval/retrieval_eval_test.go` - `SearchReranked`/`Search` call sites updated (`store.SearchOptions{}`)
- `internal/server/tools.go` - `coreSearchRequest.Categories` field added; `deps.searchMemory` builds `store.SearchOptions{...}`
- `internal/server/store_iface.go` - `memStore` interface's `SearchReranked` method signature updated to `store.SearchOptions` (unused `time` import removed)
- `internal/server/fakestore_test.go` - `spyStore.SearchReranked` fake updated to the new signature, honoring `Categories`/`Tags`/`CreatedAfter`/`CreatedBefore` from `SearchOptions` the same way its sibling `List` fake already honors `ListOptions`

## Decisions Made
- **D-09 confirmed:** `k` stays a positional parameter on both `Search` and `SearchReranked`, never folded into `SearchOptions` — burying it would weaken `SearchReranked`'s deliberate `k==0` caller-default-discipline guard.
- **Flagged behavior tightening (noted per plan, not silent):** `listFilter`'s prior inline category loop did not skip empty-string elements; `categoryMatchCondition` now does (mirroring `tagMatchConditions`), so a `categories: [""]` list request becomes a passthrough instead of a filter matching nothing. This is the one intentional behavior change in an otherwise behavior-preserving refactor.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated `internal/server/store_iface.go`'s `memStore` interface and its `spyStore` fake, not listed in the plan's `files_modified`**
- **Found during:** Task 1, `go build ./...` verification
- **Issue:** `internal/server/store_iface.go` declares a `memStore` interface with its own copy of `SearchReranked`'s old positional signature (a compile-time assertion `var _ memStore = (*store.Store)(nil)` pins `*store.Store` to satisfy it). Reshaping `Store.SearchReranked` onto `SearchOptions` without updating this interface broke `go build ./...` for the `internal/server` package, and `internal/server/fakestore_test.go`'s `spyStore` fake (which implements `memStore` for server-package tests) also needed its `SearchReranked` method updated to match.
- **Fix:** Updated `memStore.SearchReranked`'s signature to `opts store.SearchOptions` (removed the now-unused `time` import); updated `spyStore.SearchReranked` to accept `store.SearchOptions` and honor `Categories`/`Tags`/`CreatedAfter`/`CreatedBefore` from it, mirroring the fake's existing `List` implementation's handling of the equivalent `ListOptions` fields (rather than leaving `Categories` silently unfiltered in the fake, which would have been a fidelity gap against the real store).
- **Files modified:** `internal/server/store_iface.go`, `internal/server/fakestore_test.go`
- **Verification:** `go build ./...` and `go vet ./...` both exit 0; `go test ./internal/server/...` passes.
- **Committed in:** `8e2f5daf` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary mechanical fallout of the D-09 reshape that the plan's own call-site enumeration didn't name (it named test files and the one production caller, but not the interface carve that also pins the signature). No scope creep — same signature change, same package boundary.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `store.SearchOptions` and `categoryMatchCondition` are in place for 26-02/26-03 (MCP/Connect argument surface for `Categories` on `search_memory`) to wire into without touching the store layer again.
- `go vet ./...`, `gofmt -l .`, and `golangci-lint run ./internal/store/... ./internal/server/... ./internal/retrievaleval/...` are all clean; `go.mod`/`go.sum` unchanged (no new dependency).
- No blockers for 26-02 forward.

---
*Phase: 26-structured-citations-category-filter-chat-base-url*
*Completed: 2026-07-25*

## Self-Check: PASSED

- FOUND: internal/store/store.go
- FOUND: .planning/phases/26-structured-citations-category-filter-chat-base-url/26-01-SUMMARY.md
- FOUND: 8e2f5daf
- FOUND: c8482bb0
