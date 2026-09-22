---
phase: 06-cross-spine-partial-results
plan: 01
subsystem: api
tags: [protobuf, connect-rpc, mcp, cross-spine, error-handling, go]

# Dependency graph
requires:
  - phase: 02-error-classification-resourceexhausted-mapping
    provides: "the 'log the cause server-side, keep the wire generic' precedent (connecterror.go, responsetoolarge.go) this phase's D-02 copies verbatim"
provides:
  - "Additive `scopes_unknown` proto field on ListMemoriesResponse (7) and SearchMemoriesResponse (4), regenerated into gen/go, gen/ts, ui/src/lib/gen"
  - "`(*deps).searchedScopes` returns a `scopeCoverage{Scopes, Truncated, Unknown}` value with NO error — the shape all four call sites, the CLI renderer (06-02), and the red-evidence patches (06-03) build on"
  - "All four discard sites (MCP search_memory/list_memory closures, Connect ListMemories/SearchMemories) keep already-computed hits when the coverage query fails"
  - "The ListScopes cause is logged server-side exactly once (ERROR level) and never reaches either wire"
  - "internal/server/crossspinecoverage_test.go proving the coverage-unknown path end to end on both transports, plus the three-state table test"
affects: [06-02-cli-and-docs, 06-03-red-evidence-and-requirement]

# Actuals (#2632)
actuals:
  tokens: 19182
  tasks: 3
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Value-returning coverage struct (scopeCoverage) replaces a helper's error return so a partial-failure signal is compile-forced into every caller's result path instead of being abortable (D-06)"

key-files:
  created:
    - internal/server/crossspinecoverage_test.go
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/server/tools.go
    - internal/server/connectapi.go
    - internal/server/connectdescriptor_test.go
    - internal/server/tools_test.go

key-decisions:
  - "scopeCoverage{Scopes []string; Truncated bool; Unknown bool} is the struct searchedScopes returns (resolved_discretion) — chosen because removing the error return makes 'abort and discard the hits' unrepresentable at all four call sites, not merely discouraged"
  - "scopes_unknown lands at field 7 on ListMemoriesResponse (next free after the deprecated approximate=3) and field 4 on SearchMemoriesResponse (next free after scopes_truncated=3) — both permanent, per-message numbering"
  - "recallResultMap keeps `if crossSpine` as the sole outer gate; inside it, the unknown branch adds ONLY scopes_unknown and the known branch adds ONLY searched_scopes/scopes_truncated — never both, never a third key on the wrong branch"
  - "The Connect handlers set all three response fields (SearchedScopes, ScopesTruncated, ScopesUnknown) unconditionally from the scopeCoverage value — the struct's own zero-value composition already implements all three D-03 states, so no per-state branching is needed in the handler"
  - "failingListScopesStore.ListScopes delegates to the embedded spy when listErr is nil (generalized in Task 3), so the SAME double drives all three coverage states instead of a second type for the 'known' row"

requirements-completed: [REQ-cross-spine-partial]

coverage:
  - id: D1
    description: "Additive scopes_unknown field (7/4) on both recall response messages, regenerated into all three generated trees, with the descriptor gate bumped to match"
    requirement: REQ-cross-spine-partial
    verification:
      - kind: unit
        ref: "internal/server/connectdescriptor_test.go#TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs"
        status: pass
      - kind: other
        ref: "task proto:gen && git diff --exit-code -- proto gen ui/src/lib/gen"
        status: pass
    human_judgment: false
  - id: D2
    description: "searchedScopes returns coverage as a value (scopeCoverage, no error); recallResultMap and both Connect handlers stop discarding already-computed hits on a coverage-unknown response"
    requirement: REQ-cross-spine-partial
    verification:
      - kind: unit
        ref: "internal/server/crossspinecoverage_test.go#TestCrossSpineCoverageUnknownConnectSearch"
        status: pass
      - kind: unit
        ref: "internal/server/crossspinecoverage_test.go#TestCrossSpineCoverageUnknownConnectList"
        status: pass
      - kind: unit
        ref: "internal/server/crossspinecoverage_test.go#TestCrossSpineCoverageUnknownMCPSearch"
        status: pass
      - kind: unit
        ref: "internal/server/crossspinecoverage_test.go#TestCrossSpineCoverageUnknownMCPList"
        status: pass
    human_judgment: false
  - id: D3
    description: "The three coverage states (not cross-spine / coverage known / coverage unknown) are mutually distinguishable on both MCP and Connect, with the coverage-unknown state never representable as an empty-but-present searched_scopes"
    requirement: REQ-cross-spine-partial
    verification:
      - kind: unit
        ref: "internal/server/crossspinecoverage_test.go#TestCrossSpineCoverageThreeStates"
        status: pass
    human_judgment: false
  - id: D4
    description: "The seven pre-existing server-side cross-spine tests (this plan's share of the phase's nine-test regression list; the other two are CLI-side, owned by 06-02) stay green, unchanged, run against real Qdrant"
    verification:
      - kind: integration
        ref: "ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -run '^(TestSearchedScopesReporting|TestListMemoryCrossSpineIsolation|TestSearchMemoryCrossSpineIsolation|TestSearchMemoriesConnectCrossSpine|TestCrossSpineResultScope|TestConnectCrossSpineNotInferred|TestConnectCrossSpineScopeRequired)$'"
        status: pass
    human_judgment: false

duration: 56min
completed: 2026-09-20
status: complete
---

# Phase 6 Plan 1: One Coverage-Unknown Path End to End Summary

**Cross-spine `search_memory`/`list_memory`/Connect `ListMemories`/`SearchMemories` now return already-authorized hits instead of discarding them when the follow-up `ListScopes` coverage query fails, via a new additive `scopes_unknown` proto field (7/4) and a value-returning `scopeCoverage` helper.**

## Performance

- **Duration:** 56 min
- **Started:** 2026-09-20T21:52:03Z
- **Completed:** 2026-09-20T22:47:47Z
- **Tasks:** 3
- **Files modified:** 9 (1 created, 8 modified)

## Accomplishments

- Added `bool scopes_unknown = 7` to `ListMemoriesResponse` and `bool scopes_unknown = 4` to `SearchMemoriesResponse`, regenerated `gen/go`, `gen/ts`, and `ui/src/lib/gen`, and bumped the descriptor gate's field-count pins to 7/4 with new per-field pins.
- Replaced `(*deps).searchedScopes`'s `([]string, bool, error)` return with a `scopeCoverage{Scopes, Truncated, Unknown}` value carrying no error, logging the `ListScopes` cause once via `slog.ErrorContext` inside the helper itself.
- Updated `recallResultMap` and both Connect handlers (`ListMemories`, `SearchMemories`) so none of the four discard sites can abort with an error anymore — the already-computed hits always reach the caller.
- Proved the coverage-unknown path end to end on both transports with a new `internal/server/crossspinecoverage_test.go`: four per-surface tests (Connect search/list, MCP search/list via a real in-process MCP session) plus a three-state table test (`TestCrossSpineCoverageThreeStates`) pinning the mutual distinguishability of "not cross-spine" / "coverage known" / "coverage unknown" on both transports.
- Confirmed the seven pre-existing server-side cross-spine regression tests stay green against real Qdrant, and diagnosed (via a longer-timeout diagnostic run) that `internal/store`'s unrelated `TestRedEvidencePatchesAreLive` timeout during `task` is a pre-existing environmental/infra characteristic, not a regression — documented in `deferred-items.md` and `.planning/WINDOWS.md`.

## Task Commits

Each task was committed atomically:

1. **Task 1: One coverage-unknown path end to end** — `1f01dcab` (feat)
2. **Task 2: The other three discard sites proven** — `fa85bdcc` (test)
3. **Task 3: The three coverage states, distinguishable on both transports** — `d4d8419d` (test)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `proto/engram/v1/engram.proto` - Additive `scopes_unknown` field on both recall response messages
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts` - Regenerated from the proto edit
- `internal/server/tools.go` - `scopeCoverage` struct, value-returning `searchedScopes`, updated `recallResultMap`, both MCP closures un-aborted
- `internal/server/connectapi.go` - `ListMemories`/`SearchMemories` un-aborted, `ScopesUnknown` wired
- `internal/server/connectdescriptor_test.go` - Field-count pins bumped to 7/4 with new field pins
- `internal/server/crossspinecoverage_test.go` - New: failure-injection double + five tests proving the coverage-unknown contract on both transports
- `internal/server/tools_test.go` - Mechanical retype of `TestSearchedScopesReporting`'s call sites for the new `searchedScopes`/`recallResultMap` signatures (compile-forced by the signature change; assertions unchanged)

## Decisions Made

- `scopeCoverage{Scopes []string; Truncated bool; Unknown bool}` is the concrete shape (resolved_discretion): idiomatic and makes the four call sites structurally unable to discard hits, rather than merely discouraged from it.
- Field numbers 7 (`ListMemoriesResponse`) and 4 (`SearchMemoriesResponse`) — each message's own next free number, permanent commitments per D-04.
- Connect handlers set all three coverage fields unconditionally from the `scopeCoverage` value rather than branching per state — the struct's zero-value composition already produces the correct wire shape for all three D-03 states.
- Generalized the test double (`failingListScopesStore.ListScopes`) to delegate to the embedded spy when `listErr` is nil, letting one double drive all three states in `TestCrossSpineCoverageThreeStates`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Retyped `TestSearchedScopesReporting`'s call sites for the new `searchedScopes`/`recallResultMap` signatures**
- **Found during:** Task 1 (immediately after changing the helper's signature)
- **Issue:** `internal/server/tools_test.go` called the old `([]string, bool, error)` / `(base, crossSpine, scopes, truncated)` shapes, which no longer compiled after Task 1's signature change.
- **Fix:** Mechanically retyped both call sites to the new `scopeCoverage` value and struct-literal argument, preserving every existing assertion byte-for-byte in intent (cross-spine still asserts containment of both seeded scopes; non-cross-spine still asserts the zero value; the two-value-lookup absence checks are unchanged).
- **Files modified:** `internal/server/tools_test.go`
- **Verification:** `go build ./...`, `go vet ./internal/server/...`, and the regression run (`ENGRAM_REQUIRE_QDRANT=1 go test ... -run '^(TestSearchedScopesReporting|...)$'`) all pass.
- **Committed in:** `1f01dcab` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking — a compile-forced mechanical retype, not a behavior change)
**Impact on plan:** None on scope or contract; `TestSearchedScopesReporting` is unweakened and unretargeted (it still asserts the same containment/zero-value/absence properties it did before this plan).

## Issues Encountered

**`task`'s full gate hit an unrelated, pre-existing environmental timeout (not a regression from this plan) — see `deferred-items.md` for full detail.**

`internal/store`'s `TestRedEvidencePatchesAreLive` (which sequentially applies all 54 currently-registered red-evidence patches via subprocess `go test -run` invocations) hit Go's default 601s per-package timeout twice during `task` runs against this plan's committed tree, compounded by heavy, confirmed-concurrent, *unrelated* `go test`/`golangci-lint` load from other active sessions sharing this machine. Both timeouts left exactly one already-applied red-evidence patch un-reverted on disk (`t.Cleanup` cannot run once Go's timeout kills the process); each was hand-verified against `git diff` and restored with `git checkout -- <file>` before continuing — no red-evidence patch registration, target test, or unrelated source file was modified by this plan.

A third, diagnostic run (`go test ./internal/store/... -count=1 -timeout 20m`, bypassing `task`'s unconfigured default) **passed cleanly at 685.150s**, with zero `--- FAIL:` lines and a clean `git status` afterward — conclusive proof `internal/store` is correct and the harness genuinely needs more wall-clock time than Go's 601s default under load, not that anything broke. Zero files under `internal/store` are in this plan's `files_modified` or actual diff. Recorded in `.planning/WINDOWS.md` (entry 13, kind `deviation`, status `open`) and `.planning/phases/06-cross-spine-partial-results/deferred-items.md`, with a recommendation (bump the timeout or reduce per-patch subprocess cost) for whichever phase owns `internal/store`'s test-harness performance — out of scope here since no `internal/store` file is in this plan's `files_modified`.

Every check actually scoped to this plan passed cleanly, twice over: `go build ./...`, `go vet ./internal/server/...` (and repo-wide, modulo one unrelated pre-existing `cmd/engram/operator_view_test.go` vet finding from commit `62c39c22`, untouched by this plan), all `internal/server` package tests, the four new per-surface tests, the three-state table test, the descriptor gate, `task proto:gen` + a clean generated-tree diff, the seven-test regression list under `ENGRAM_REQUIRE_QDRANT=1` (7/7 PASS, 0 SKIP), and `task license:check`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 06-02 (CLI + docs) can proceed: `scopeCoverage`'s field names and the proto's `GetScopesUnknown()` accessor are shipped and stable for `renderCoverageFooter`'s third form.
- Plan 06-03 (red-evidence patches + REQ ticking) can proceed: the shipped names (`scopeCoverage`, `searchedScopes`, `recallResultMap`, `ScopesUnknown`) match what 06-03's red-evidence patches are authored against, per this plan's `resolved_discretion` commitment.
- `REQ-cross-spine-partial` is shared across all three plans in this phase (06-01/06-02/06-03) and is intentionally left unticked in `.planning/REQUIREMENTS.md` — plan 06-03 owns ticking it once all three plans' SUMMARYs exist (shared-ID gate).
- No blockers for 06-02/06-03. The `internal/store` red-evidence-harness timeout (see Issues Encountered) is orthogonal to this phase's remaining plans, which touch no `internal/store` files either.

---
*Phase: 06-cross-spine-partial-results*
*Completed: 2026-09-20*

## Self-Check: PASSED

- All 9 key files (created + modified) confirmed present on disk.
- All 3 task commits (`1f01dcab`, `fa85bdcc`, `d4d8419d`) confirmed in `git log`.
- Re-ran acceptance criteria: `TestCrossSpineCoverageUnknownConnectSearch` PASS, `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` PASS, `task proto:gen && git diff --exit-code -- proto gen ui/src/lib/gen` exit 0, `TestCrossSpineCoverageUnknownConnectList`/`MCPSearch`/`MCPList`/`TestCrossSpineCoverageThreeStates` all PASS.
- Re-ran the plan-level `<verification>` seven-test regression list under `ENGRAM_REQUIRE_QDRANT=1`: 7/7 top-level PASS, 0 SKIP.
- `git status --short` clean of any tracked-file modification (only the intentional `.planning/WINDOWS.md` update and two new untracked docs files, plus the pre-existing unrelated `.planning/milestone.lock`).
