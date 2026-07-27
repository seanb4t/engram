---
phase: 26-structured-citations-category-filter-chat-base-url
plan: 02
subsystem: server
tags: [go, mcp, search, list, category-filter]

# Dependency graph
requires:
  - phase: 26-structured-citations-category-filter-chat-base-url
    plan: 01
    provides: store.SearchOptions{Categories} + categoryMatchCondition; coreSearchRequest.Categories field (already threaded into store.SearchOptions by deps.searchMemory)
provides:
  - searchArgs.Categories / listArgs.Categories — the MCP-advertised `categories` argument on search_memory and list_memory
  - OR-explicit jsonschema wording pattern for a filter field adjacent to an AND-semantics field
affects: [26-03, 26-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MCP arg struct field -> core*Request field -> deps method, zero lane-specific interpretation in the closure (D-07 transport-neutral core, reused verbatim for Categories)"

key-files:
  created: []
  modified:
    - internal/server/tools.go
    - internal/server/tools_test.go

key-decisions:
  - "D-08 confirmed as shipped: categories composes as OR (jsonschema states this explicitly, in contrast to the adjacent tags field's ALL/AND wording) since a record carries exactly one category and AND across two categories is always empty"
  - "coreSearchRequest.Categories and deps.searchMemory's threading into store.SearchOptions.Categories, and coreListRequest.Categories/deps.listMemory's threading into store.ListOptions.Categories, were both already shipped in 26-01 — this plan's only gap was the two MCP arg-struct fields and their two closure-literal wirings"

patterns-established:
  - "Table-driven MCP-boundary edge test (TestCategoriesArgEdges) covering the same edge set (omitted/empty/empty-element/unknown-value/ordering) across both search_memory and list_memory in one test via a name+idsErr-closure table, reusing a single seeded fixture"

requirements-completed: [REQ-category-filter]

coverage:
  - id: D1
    description: "search_memory and list_memory each accept an optional categories argument and return only records in the listed categories (SC2)"
    requirement: "REQ-category-filter"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestSearchMemoryCategoriesArg"
        status: pass
      - kind: integration
        ref: "internal/server/tools_test.go#TestListMemoryCategoriesArg"
        status: pass
    human_judgment: false
  - id: D2
    description: "categories is plural []string on both searchArgs and listArgs, matching the already-plural ListOptions.Categories/coreListRequest.Categories/proto ListMemoriesRequest.categories (D-08)"
    requirement: "REQ-category-filter"
    verification:
      - kind: unit
        ref: "internal/server/tools.go — searchArgs.Categories, listArgs.Categories field declarations"
        status: pass
    human_judgment: false
  - id: D3
    description: "categories jsonschema description states ANY/OR semantics explicitly, distinct from the adjacent tags field's ALL/AND wording"
    requirement: "REQ-category-filter"
    verification:
      - kind: unit
        ref: "internal/server/tools.go — literal jsonschema text 'restrict to records in ANY of the listed categories (OR)' on both structs"
        status: pass
    human_judgment: false
  - id: D4
    description: "omitted categories, an empty array, and a slice containing only an empty string are all an identical passthrough — never a contradiction that returns nothing"
    requirement: "REQ-category-filter"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestCategoriesArgEdges/*/empty_slice_is_passthrough, .../empty_string_element_is_passthrough"
        status: pass
    human_judgment: false
  - id: D5
    description: "a categories value matching no stored category returns zero results with a nil error (D-11), never rejected as invalid input; prefix/whitespace-padded values are not fuzzy-matched"
    requirement: "REQ-category-filter"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestCategoriesArgEdges/*/unknown_value_returns_zero_and_nil_error, .../prefix_value_is_not_a_match, .../whitespace_padded_value_is_not_a_match"
        status: pass
    human_judgment: false
  - id: D6
    description: "a categories filter that excludes nothing leaves list_memory's most-recent-first order and search_memory's rerank order byte-identical to the same call with categories omitted"
    requirement: "REQ-category-filter"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestCategoriesArgEdges/*/all_categories_present_leaves_ordering_unchanged"
        status: pass
    human_judgment: false
  - id: D7
    description: "the MCP argument is a pure passthrough into coreSearchRequest/coreListRequest — no authz decision, scope rewrite, or owner substitution in the closure (SC4/T-26-05)"
    requirement: "REQ-category-filter"
    verification:
      - kind: judgment
        ref: "internal/server/tools.go closure diffs add only `Categories: a.Categories` to each core*Request literal; no other line in either closure changed"
        status: pass
    human_judgment: true
  human_judgment_notes: "D7 verified by direct code inspection: the search_memory and list_memory closures gained exactly one field assignment each (Categories: a.Categories), with everything else — K default, CursorMode:true, shapeRecall — byte-identical to pre-change. No scope/owner/authz logic touches the new field."

duration: 9min
completed: 2026-07-25
status: complete
---

# Phase 26 Plan 02: Categories Argument on search_memory/list_memory Summary

**Added a plural `categories []string` argument to `search_memory` and `list_memory`, wired as a pure passthrough into the already-Categories-capable `coreSearchRequest`/`coreListRequest`, with an explicit ANY/OR jsonschema description so agents don't assume AND symmetry with the adjacent `tags` field.**

## Performance

- **Duration:** ~9 min
- **Completed:** 2026-07-25
- **Tasks:** 2 completed
- **Files modified:** 2

## Accomplishments

- `searchArgs.Categories` and `listArgs.Categories` added, both carrying the load-bearing jsonschema wording `restrict to records in ANY of the listed categories (OR) — unlike tags, which requires ALL` (D-08).
- Both MCP closures (`search_memory`, `list_memory`) gained exactly one field assignment (`Categories: a.Categories`) on their existing `coreSearchRequest`/`coreListRequest` literals — no other closure behavior touched, confirming the SC4 pure-passthrough constraint (no authz decision in the closure).
- `TestSearchMemoryCategoriesArg` and `TestListMemoryCategoriesArg` seed a `decision`/`preference`/`gotcha` fixture and assert on returned id sets for the single-value and two-value OR cases, plus list_memory's omitted-argument passthrough.
- `TestCategoriesArgEdges` runs a shared table across both tools covering: omitted/empty-slice/empty-string-element passthrough parity, an unknown value returning zero results with a nil error (D-11 — no allowlist), a strict-prefix and a whitespace-padded value proving exact-match (not fuzzy) semantics, and an all-categories-present case proving result order is unchanged.
- Confirmed the store-lane plumbing from `26-01` (`store.SearchOptions.Categories`, `categoryMatchCondition`, `coreSearchRequest.Categories`, `deps.searchMemory`'s threading) needed zero changes — this plan's only gap was the two MCP arg-struct fields and their two closure wirings, exactly as scoped.

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire categories from the MCP argument to the store filter (D-08)** - `b462b6f0` (feat)
2. **Task 2: Pin the empty, unknown-value, and ordering edges of the categories argument** - `ac5e4c74` (test)
3. **Lint fix (revive context-as-argument on the new test helper)** - `9c0ad08a` (fix)

## Files Created/Modified

- `internal/server/tools.go` - `searchArgs.Categories`, `listArgs.Categories` field declarations; `Categories: a.Categories` added to the `search_memory` closure's `coreSearchRequest` literal and the `list_memory` closure's `coreListRequest` literal
- `internal/server/tools_test.go` - `seedCategoryFixture` shared fixture helper; `TestSearchMemoryCategoriesArg`, `TestListMemoryCategoriesArg`, `TestCategoriesArgEdges`

## Decisions Made

- **D-08 confirmed as shipped, not re-derived:** `categories` is plural on both `searchArgs` and `listArgs`, matching `ListOptions.Categories`/`coreListRequest.Categories`/the proto's plural convention. The jsonschema description explicitly states ANY/OR semantics — copy-pasting the adjacent `tags` field's ALL/AND wording was the exact trap D-08 calls out, and was avoided by writing distinct wording that names both semantics for contrast.
- **No new store-layer or core-request-layer work needed:** `26-01` had already landed `coreSearchRequest.Categories` and `deps.searchMemory`'s threading into `store.SearchOptions.Categories`, and `coreListRequest.Categories`/`deps.listMemory` threading into `store.ListOptions.Categories` had shipped even earlier. This plan's entire diff is two struct fields plus two one-line closure additions plus tests — no deviation from the plan's stated minimal scope was needed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `seedCategoryFixture` test helper parameter order failed golangci-lint's `revive` `context-as-argument` check**
- **Found during:** post-Task-2 `golangci-lint run ./internal/server/...` verification (run proactively per CLAUDE.md's lint gate, ahead of the plan's own listed verification commands)
- **Issue:** the helper was declared `seedCategoryFixture(t *testing.T, d *deps, ctx context.Context, scope string)` — `context.Context` must be the first parameter per the repo's established convention (mirrored by every other `ctx`-taking test helper in this file, e.g. `callerFor(ctx context.Context, t *testing.T)`).
- **Fix:** reordered to `seedCategoryFixture(ctx context.Context, t *testing.T, d *deps, scope string)` and updated its three call sites.
- **Files modified:** `internal/server/tools_test.go`
- **Verification:** `golangci-lint run ./internal/server/...` reports `0 issues`; `go test ./internal/server/... -run 'TestSearchMemoryCategoriesArg|TestListMemoryCategoriesArg|TestCategoriesArgEdges' -v` still all pass.
- **Committed in:** `9c0ad08a`

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Purely a test-helper lint fix; zero effect on production code or test coverage/assertions.

## Issues Encountered

None. Docker was available in this session; all live-Qdrant tests ran (none skipped).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `search_memory`/`list_memory`'s `categories` argument is fully wired end-to-end (MCP arg → transport-neutral core → store filter) and covered by id-asserting tests plus the full edge-probe set (empty, unknown-value, ordering).
- `go build ./...`, `go vet ./...`, `gofmt -l .`, and `golangci-lint run ./internal/server/...` are all clean; `go.mod`/`go.sum` unchanged (no new dependency, confirmed via `git diff --exit-code -- go.mod go.sum`).
- No blockers for `26-03` forward.

---
*Phase: 26-structured-citations-category-filter-chat-base-url*
*Completed: 2026-07-25*

## Self-Check: PASSED

- FOUND: internal/server/tools.go
- FOUND: internal/server/tools_test.go
- FOUND: b462b6f0
- FOUND: ac5e4c74
- FOUND: 9c0ad08a
