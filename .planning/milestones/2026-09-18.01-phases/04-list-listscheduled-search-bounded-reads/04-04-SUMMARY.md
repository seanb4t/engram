---
phase: 04-list-listscheduled-search-bounded-reads
plan: 04
subsystem: database
tags: [qdrant, bounded-reads, search, rerank, discovery, two-phase, red-evidence]

# Dependency graph
requires:
  - phase: 04-list-listscheduled-search-bounded-reads
    provides: "04-02/04-03: readView/fullView/summaryView/perRPCLimit (03-shared-bounded-read-mechanism, composed by 04-02), MaxRecallLimit, and the recallTransmitters/recallEntryPointSeeds AST-gate machinery this plan extends"
provides:
  - "internal/store/searchfetch.go: includeIDs (the id-set inclusion filter, structural mirror of excludeSeen) and (*Store).fetchPayloadsByID — the D-09 batched, byte-budgeted payload fetch shared by Store.Search and Store.SearchDiscovery"
  - "Store.Search rebuilt as a payload-free vector Query plus fetchPayloadsByID, rebuilding the result by walking phase one's own returned id order (never re-sorted)"
  - "SearchOptions.Full — the fetch-phase payload projection flag (summary view by default); SearchReranked forces it on unconditionally for its own delegated call"
  - "Store.SearchDiscovery rebuilt onto the same two-phase fetch, always the full view (no discovery surface exposes a `full` flag, so none was invented)"
  - "Store.fetchPayloadsByID classified in recallTransmitters, reachable from the Search/SearchReranked/SearchDiscovery seeds; Store.Search's own classification updated for its now payload-free Query"
  - "memoriesFromPoints retained solely for TestMemoriesFromPointsCarriesScore (embedtext_test.go) — no production caller remains after both search entry points were rewritten"
affects: [04-05, 04-06, 04-07, 04-08]

# Actuals (#2632)
actuals:
  tokens: 8852
  tasks: 2
  commits: 2
  plan_head_before: 0ace2d89ab1e71a610423e33a44127a97f9a512f

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "D-09 two-phase search fetch: a payload-free vector Query for ids+scores, then a shared batched byte-budgeted Scroll fetch (fetchPayloadsByID) re-applying the IDENTICAL filter the query carried — deliberately NOT scrollOrderedPage, since an id-set fetch has no ordering/keyset-boundary concern the caller doesn't already hold."
    - "Rank order preserved by walking phase one's OWN returned id order rather than re-deriving or re-sorting by score — reproduces the vector query's exact tie order, stronger than a score sort."
    - "A surface with no `full`/projection knob in its own wire contract (SearchDiscovery, mirroring ListScheduled's precedent) hard-codes the full view rather than inventing a caller-facing flag no request message or tool schema exposes."

key-files:
  created:
    - internal/store/searchfetch.go
    - internal/store/searchtwophase_oversized_test.go
  modified:
    - internal/store/store.go
    - internal/store/export_test.go
    - internal/store/schemaversion_recallgate_test.go

key-decisions:
  - "D-09 executed exactly as locked: two-phase search adopted for Store.Search/SearchReranked/SearchDiscovery only, never List — the caller holds the ranking and the identical filter is re-applied on the fetch, so List's TOCTOU/GetPoints-order concerns this design would otherwise need do not apply here."
  - "SearchReranked forces opts.Full = true unconditionally on its delegated Search call: the lexical reranker scores against content for EVERY candidate, and candidateK clamps the pool at 100 regardless of k, so this one surface's fetch view is fixed by an internal consumer, never by the caller's own Full flag."
  - "SearchDiscovery's fetch always uses the full view — neither the Connect SearchDiscoveriesRequest message nor the MCP search_discovery tool arguments carry a `full` flag, so there is no caller projection to honor and none was invented (the same reasoning that keeps ListScheduled knob-free)."
  - "memoriesFromPoints was NOT deleted: TestMemoriesFromPointsCarriesScore (embedtext_test.go, package store) is its sole remaining caller after both search entry points were rewritten to decode via fromPayload directly, re-attaching the phase-one score by id."

patterns-established:
  - "Pattern D-09 (Task 1): fetchPayloadsByID — a shared, view-parameterized, filter-re-applying batched fetch, callable from any recall entry point that already holds a phase-one id+score list, its own primitive distinct from scrollOrderedPage's ordered-page machinery."
  - "Pattern (Task 2): a second search entry point (SearchDiscovery) composes the SAME fetch helper Task 1 proved on Store.Search, changing only which filter and which fixed view it passes — no per-caller special-casing in fetchPayloadsByID itself."

requirements-completed: [REQ-search-k-bounded]

coverage:
  - id: D1
    description: "Store.Search and Store.SearchReranked run a payload-free vector Query plus a batched, byte-budgeted id-set fetch that re-applies the identical filter; rank order and scores are provably the vector query's own; an empty match issues exactly one gRPC call; a cross-owner id placed directly in a fetch batch never comes back"
    requirement: "REQ-search-k-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestSearchTwoPhaseBounded (few-large, many-small)"
        status: pass
      - kind: integration
        ref: "internal/store#TestSearchFetchSkipsEmptyBatch"
        status: pass
      - kind: integration
        ref: "internal/store#TestSearchPreservesRankOrder"
        status: pass
      - kind: unit
        ref: "internal/store#TestRecallEmissionSetIsCompleteAndClassified, TestSchemaVersionNeverGatesRecall"
        status: pass
    human_judgment: false
  - id: D2
    description: "Store.SearchDiscovery runs the same two-phase fetch, always in the full view; a superseded or archived discovery never appears; an anonymous caller never receives another owner's shared discovery (nor the fixture owner's own private records)"
    requirement: "REQ-search-k-bounded"
    verification:
      - kind: integration
        ref: "internal/store#TestSearchDiscoveryTwoPhaseBounded (few-large, many-small)"
        status: pass
      - kind: other
        ref: "task (lint + full test suite, including ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... -count=1)"
        status: pass
    human_judgment: false

# Metrics
duration: ~35min
completed: 2026-09-20
status: complete
---

# Phase 4 Plan 4: Search and SearchDiscovery Two-Phase Fetch Summary

**Both search entry points now issue a payload-free vector Query plus a shared, byte-budgeted id-set fetch (`fetchPayloadsByID`) that re-applies the identical authz/recall filter — closing the last two `List`-shaped read sites in Phase 4's inventory, so a search can safely request any count up to the recall maximum without its own response overflowing.**

## Performance

- **Duration:** ~35 min
- **Started:** ~2026-09-20T08:00Z (approx, following 04-03's completion)
- **Completed:** 2026-09-20T08:30Z
- **Tasks:** 2
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- Added `internal/store/searchfetch.go`: `includeIDs` (the structural mirror of `orderedpage.go`'s `excludeSeen`, narrow-only, MUST-only, never mutating the caller's filter) and `(*Store).fetchPayloadsByID` — the D-09 shared batched fetch whose first statement returns before any RPC on an empty id batch, keeping the recall gate's exact search capture counts at one.
- Rebuilt `Store.Search`: the `Query` now requests `qdrant.NewWithPayload(false)`; payloads arrive through `fetchPayloadsByID` using the SAME filter value; the result is assembled by walking phase one's own returned id order, re-attaching the held score, and skipping any id the fetch did not return.
- Added `SearchOptions.Full` (summary view by default, full view when set); `SearchReranked` forces it on unconditionally, documented as fixed by the lexical reranker's own content dependency, not the caller's flag.
- Rebuilt `Store.SearchDiscovery` onto the identical two-phase shape, always in the full view (no discovery surface exposes a `full` flag).
- Classified `Store.fetchPayloadsByID` in `recallTransmitters`, reachable from the Search/SearchReranked/SearchDiscovery seeds; updated `Store.Search`'s own justification for its now payload-free `Query`. Did not touch any `recallInvocationRows` expected count or method multiset.
- Retained `memoriesFromPoints` (not deleted): `TestMemoriesFromPointsCarriesScore` (`embedtext_test.go`) is its sole remaining caller.
- Authored `internal/store/searchtwophase_oversized_test.go`: `TestSearchTwoPhaseBounded`, `TestSearchDiscoveryTwoPhaseBounded` (both fixture shapes each), `TestSearchFetchSkipsEmptyBatch` (plus a nested drop-on-disappear subtest), and `TestSearchPreservesRankOrder`. Followed the TDD gate for Task 2 (`tdd="true"`): authored the discovery-facing tests first, observed genuine RED against the pre-rewrite `SearchDiscovery` (receive-limit overflow, and a payload-requested Query), then implemented and confirmed GREEN.

## Task Commits

Each task was committed atomically:

1. **Task 1: End to end — Store.Search's two-phase fetch, the shared helper, and its recall-gate classification** - `f2e3641e` (feat)
2. **Task 2: Store.SearchDiscovery on the same fetch, the empty-batch property pinned, and rank order proven preserved** - `c1934283` (feat)

_No plan-metadata commit yet; STATE.md/ROADMAP.md updates follow this summary._

## RED Evidence (Task 2, TDD)

Observed against the pre-rewrite `Store.SearchDiscovery` (single unbounded full-payload `Query`), before the rewrite:

```
searchtwophase_oversized_test.go:309: few-large: SearchDiscovery(max): Query() failed: ...: qdrant response exceeded the client's receive limit: /qdrant.Points/Query: grpc: received message after decompression larger than max 4194304
searchtwophase_oversized_test.go:309: many-small: SearchDiscovery(max): Query() failed: ...: qdrant response exceeded the client's receive limit: /qdrant.Points/Query: grpc: received message after decompression larger than max 4194304
--- FAIL: TestSearchDiscoveryTwoPhaseBounded (1.10s)
    --- FAIL: TestSearchDiscoveryTwoPhaseBounded/few-large (0.27s)
    --- FAIL: TestSearchDiscoveryTwoPhaseBounded/many-small (0.83s)
searchtwophase_oversized_test.go:410: SearchDiscovery: Query requested payload — the vector query must be payload-free (D-09)
--- FAIL: TestSearchFetchSkipsEmptyBatch (0.18s)
    --- PASS: TestSearchFetchSkipsEmptyBatch/drop-on-disappear (0.00s)
```

Both `TestSearchDiscoveryTwoPhaseBounded` failures are genuine RED (an assertion for the planned behavior — bounded, successful search over an oversized fixture — failing on the pre-fix single-Query overflow), not `INVALID_RED`. `TestSearchFetchSkipsEmptyBatch`'s failure is likewise genuine RED, isolated to the `SearchDiscovery` leg's payload-free assertion (its `Search`/`SearchReranked` legs already passed, having been fixed in Task 1). `TestSearchPreservesRankOrder` and the `drop-on-disappear` subtest passed on first authoring — see Issues Encountered for why that is expected, not a TDD violation.

After the rewrite, all three test functions pass in full (see Self-Check).

## Files Created/Modified

- `internal/store/searchfetch.go` (new) - `includeIDs`, `(*Store).fetchPayloadsByID`
- `internal/store/store.go` - `SearchOptions.Full`; `Store.Search` and `Store.SearchDiscovery` rewritten onto the two-phase fetch; `SearchReranked` forces `Full = true`; `memoriesFromPoints`'s doc comment updated to record its sole remaining test caller
- `internal/store/export_test.go` - `IncludeIDs`, `FetchPayloadsByID` shims
- `internal/store/schemaversion_recallgate_test.go` - `Store.fetchPayloadsByID` classification entry added to `recallTransmitters`; `Store.Search`'s justification updated
- `internal/store/searchtwophase_oversized_test.go` (new) - `TestSearchTwoPhaseBounded`, `TestSearchDiscoveryTwoPhaseBounded`, `TestSearchFetchSkipsEmptyBatch`, `TestSearchPreservesRankOrder`, plus the shared `searchFetchRecorder` interceptor and `filterCarriesIDSetAndNested` helper

## Decisions Made

See key-decisions in frontmatter.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **Literal-command mismatch in the plan's own acceptance/verification greps (same pattern documented in 04-02-SUMMARY.md and 04-03-SUMMARY.md's Issues Encountered):**
  1. Task 1's and the plan-level `<verification>`'s `rg -v '^\s*//' internal/store/store.go | rg -c -e 'qdrant[.]NewWithPayload[(]true[)]'` is written to expect `0` ("no read path in store.go asks the vector query for a full payload any more"), but the true whole-file count is `4` after Task 1 (`Store.Get`, `Store.SearchDiscovery`'s not-yet-rewritten `Query`, `Reindex`'s `ScrollAndOffset`, and `reindexTargetContents`' `Get`) and `3` after Task 2 (the same three non-Query call sites — `Get`/`ScrollAndOffset`/`Get` — none of which is a vector `Query` and none of which this plan's `files_modified` touches). Verified instead against the criterion's own stated intent, scoped to the functions this plan actually rewrites: `awk '/^func \(s \*Store\) Search\(/,/^}/' internal/store/store.go | rg -c -e 'NewWithPayload\(true\)'` and the same for `SearchDiscovery` both print nothing (zero matches) after both tasks.
  2. `ripgrep`'s `-c` on piped stdin input does NOT print a literal `"0"` when there are zero matches (unlike GNU/BSD `grep -c`) — it prints nothing and exits `1`. Every acceptance criterion phrased as "`rg -c ... prints 0`" was verified by exit code / `wc -l` rather than by literally reading a `"0"` from stdout.
- **Self-caught test-authoring mistake in `TestSearchDiscoveryTwoPhaseBounded`:** the first draft asserted that another owner's `shared` discovery must never appear in the fixture OWNER's own (authenticated) search hits. That is wrong per `ownerOrSharedCondition`'s own documented contract: for an authenticated caller the condition is `owner==sub OR visibility=="shared"`, so ANY `shared` record — regardless of whose it actually is — legitimately matches for any authenticated subject. Running the test surfaced this as a failure indistinguishable at a glance from genuine RED; inspecting `ownerOrSharedCondition`'s doc comment confirmed it was a test bug, not a production one. Fixed by dropping the `sharedID` check from the authenticated-owner assertion and keeping it only on the anonymous-caller assertion (where it correctly holds, since `ownerOrSharedCondition` denies the shared bucket to an anonymous principal). No production code was touched for this fix.
- **Two of Task 2's four new test functions passed on first authoring, before the `SearchDiscovery` rewrite:** `TestSearchPreservesRankOrder` and the `drop-on-disappear` subtest of `TestSearchFetchSkipsEmptyBatch` exercise `Store.Search`/`store.FetchPayloadsByID` behavior already proven correct by Task 1's own commit — they are not new production behavior Task 2 introduces, so no RED was expected or observed from them specifically. `TestSearchDiscoveryTwoPhaseBounded` and `TestSearchFetchSkipsEmptyBatch`'s `SearchDiscovery` leg (the two assertions that DO depend on this task's own rewrite) both showed genuine RED, recorded above.
- **One-off environmental Qdrant testcontainer flake:** a single `ENGRAM_REQUIRE_QDRANT=1 task` run failed with `connection reset by peer: error reading server preface` against the shared testcontainer across roughly forty otherwise-unrelated tests simultaneously (same category as 04-02-SUMMARY.md's Deferred Items entry: "Docker/testcontainer flake on one run; not a code defect, not reproduced since"). An immediate retry of `internal/store` alone, then the full `task` gate, both passed cleanly with no code changes in between.
- No blockers encountered executing either task; Docker/Qdrant was reachable throughout (after the one-off retry above), and no auth gates were hit.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Every `List`-shaped and `Search`-shaped recall path in 03-INVENTORY's Phase 4 rows is now migrated onto Phase 3's shared bounded-read mechanism: `Store.List` (04-02/04-03), `Store.ListScheduled` (04-03), and `Store.Search`/`SearchReranked`/`SearchDiscovery` (this plan). Phase 4's own read-site migration inventory closes here.
- `fetchPayloadsByID` and `includeIDs` (`searchfetch.go`) are available as the D-09 pattern for any future recall path that needs a filter-re-applying id-set fetch; no other Phase 4 site needs one.
- `REQ-search-k-bounded` is declared by this plan AND by 04-05, 04-06, and 04-08 (confirmed via `requirements.ready-ids`) — it stays **blocked**, not marked complete, until all four plans finish; this plan does not run `requirements.mark-complete` for it.
- The two literal-vs-scoped acceptance-criteria mismatches above (both inherited from the plan's own written greps, not this plan's own choices) are flagged for the verifier per the same convention 04-02/04-03 established.
- No blockers for 04-05.

---
*Phase: 04-list-listscheduled-search-bounded-reads*
*Completed: 2026-09-20*

## Self-Check: PASSED

Both created files verified present on disk: `internal/store/searchfetch.go`, `internal/store/searchtwophase_oversized_test.go`. Both commits (`f2e3641e`, `c1934283`) verified present in `git log --oneline --all`. Every acceptance criterion for both tasks re-run and confirmed passing at final HEAD: `includeIDs`/`fetchPayloadsByID` declaration counts, the `MustNot`-absence check, the `searchfetch.go` mention inside `recallTransmitters`, the `candidateK` clamp (unchanged), the `memoriesFromPoints`/`fetchPayloadsByID` occurrence counts in `store.go` (2 and 2 respectively, both explained above), the discovery proto `full`-field absence check, `go test ./internal/keylinks/ -count=1`, and `task license:check` (all clean). The plan-level `<verification>` block passes in full: `ENGRAM_REQUIRE_QDRANT=1 go test ./internal/store/... ./internal/server/... -count=1` ok; `task` (lint + full test suite) exits 0; `task license:check` clean; `go test ./internal/keylinks/ -count=1` ok; `git diff --exit-code HEAD -- go.mod go.sum` exits 0 (no diff); the `qdrant.NewWithPayload(true)` absence check holds when scoped to `Store.Search`/`Store.SearchDiscovery` as documented under Issues Encountered.
