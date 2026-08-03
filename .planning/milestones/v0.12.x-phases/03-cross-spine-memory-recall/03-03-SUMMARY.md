---
phase: 03-cross-spine-memory-recall
plan: 03
subsystem: api
tags: [qdrant, mcp, authorization, testing, testcontainers]

# Dependency graph
requires:
  - phase: 03-cross-spine-memory-recall (plan 02)
    provides: "effectiveSearchScope (the D-03/D-07 guard, already generic across four call sites), the ownerScopeFilter/SearchDiscovery conditional-scope composition pattern to mirror in listFilter, and TestCrossSpineAuthzIsolation as the standing authz pin"
provides:
  - "list_memory cross_spine=true: spans every scope the caller may read, wired end to end from the MCP tool argument through the reused effectiveSearchScope guard to Store.List's now-conditional listFilter"
  - "listFilter with a conditional scope clause and an unconditional ownerOrSharedCondition authz clause, mirroring ownerScopeFilter/SearchDiscovery's identical shape"
  - "TestListRulesRejectsEmptyScope pinning that the SECOND production path to Store.List (listRules, guarded by validRuleScope) already rejects an empty scope entry before the store is reached"
  - "(*deps).searchedScopes and recallResultMap: searched_scopes / scopes_truncated reporting on both search_memory and list_memory, present only on cross-spine calls (D-14 byte-identical guarantee otherwise)"
  - "TestListMemoryCrossSpineIsolation, TestSearchedScopesReporting, TestCrossSpineResultScope (handler-level), TestListCrossSpine, TestListCrossSpineTotal (store-level) as standing regression pins"
affects: [03-04-connect-proto-parity]

actuals:
  tokens: 7800
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "listFilter's scope match is conditional on scope != '', with ownerOrSharedCondition appended unconditionally outside that conditional — the identical shape plan 03-02 applied to ownerScopeFilter and Store.SearchDiscovery already established, now applied to a third store filter builder for structural consistency across all three cross-spine-capable reads."
    - "effectiveSearchScope, generic since 03-02 anticipated this exact reuse, now serves list_memory's two call sites (deps.listMemory and the MCP closure) in addition to search_memory's — zero new guard helper written, per D-03's 'same helper' instruction."
    - "searched_scopes/scopes_truncated live in a shared recallResultMap(base, crossSpine, scopes, truncated) helper both MCP closures call, rather than duplicated map-literal logic — this also makes the closures' exact result-map contract directly unit-testable (see Decisions Made)."
    - "The second production path to a store filter-builder (listRules -> Store.List, alongside deps.listMemory -> Store.List) is audited explicitly when the filter's empty-scope semantics change, not assumed safe from a single-call-site argument that only held at the store layer."

key-files:
  created: []
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/server/tools.go
    - internal/server/tools_test.go
    - internal/server/rules_test.go

key-decisions:
  - "TDD RED for Task 1 was observed as a genuine assertion failure (four distinct 'missing record'/'span only 0 scopes' failures), not a compile error: listArgs.CrossSpine, coreListRequest.CrossSpine, deps.listMemory's effectiveSearchScope call, and the list_memory MCP closure's wiring were all added BEFORE the listFilter conditional-scope edit, mirroring 03-02's Task 2 sequencing exactly."
  - "Task 2's TestSearchedScopesReporting exercises (*deps).searchedScopes and recallResultMap directly rather than through a full MCP client/session round trip, because no in-process MCP session-invocation harness exists anywhere in this codebase (confirmed via TestToolArgSchemasDoNotPanic's own comment: 'the handler tests use deps directly'). recallResultMap is the exact map-shaping function both the search_memory and list_memory closures call to assemble their final result map, so asserting on its output IS asserting on 'the result map' the plan's behavior block describes — not a parallel approximation of it. Building a new session-harness file was out of scope (files_modified lists no new test-infra file) and would have been a larger, unplanned addition for a codebase-wide gap unrelated to this plan's actual risk (T-03-01/T-03-02, both about authz, not about MCP wire-shape testing)."
  - "Task 3's three pins (TestListCrossSpine, TestListCrossSpineTotal, TestCrossSpineResultScope) all passed on first run with no RED — expected per the plan's own text ('The implementation from Tasks 1 and 2 should already satisfy all of them'), since they are standing regression pins over behavior Tasks 1-2 already implemented, not new-behavior drivers."

patterns-established:
  - "A store-layer filter builder gaining a conditional scope clause is only as safe as every production caller of the store method it feeds — auditing the SECOND call site (not just the one under edit) is now an explicit plan action, not an assumption inherited from an earlier single-call-site justification that held at a narrower layer."

requirements-completed: [REQ-cross-spine-search, REQ-cross-spine-result-provenance, REQ-cross-spine-authz-verified]

coverage:
  - id: D1
    description: "An agent calling list_memory with cross_spine=true gets back records from every scope it may read; with cross_spine omitted or false it gets only the named scope (D-08), and an empty scope with cross_spine=false is rejected at deps.listMemory via the reused effectiveSearchScope guard (D-03)."
    requirement: "REQ-cross-spine-search"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestListMemoryCrossSpineIsolation"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestListCrossSpine"
        status: pass
    human_judgment: false
  - id: D2
    description: "Cross-spine list_memory does not widen authorization: owner A's cross-spine list never returns owner B's private record over an overlapping scope name, and does return owner B's shared record (positive control); the pre-existing store-level authz proof (TestCrossSpineAuthzIsolation, 03-01) still passes unchanged after the listFilter edit."
    requirement: "REQ-cross-spine-authz-verified"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestListMemoryCrossSpineIsolation"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestCrossSpineAuthzIsolation"
        status: pass
    human_judgment: false
  - id: D3
    description: "The second production path to Store.List (listRules) cannot become a store-wide read now that an empty scope means 'everything readable': validRuleScope rejects an empty scope entry before Store.List is reached, and the rejection is pinned so a future relaxation of the validator fails loudly."
    requirement: "REQ-cross-spine-authz-verified"
    verification:
      - kind: unit
        ref: "internal/server/rules_test.go#TestListRulesRejectsEmptyScope"
        status: pass
    human_judgment: false
  - id: D4
    description: "A cross-spine list_memory reports an exact total across every readable scope (D-09), strictly greater than the equivalent scope-confined total, computed server-side via Store.List's existing exact Count."
    requirement: "REQ-cross-spine-search"
    verification:
      - kind: integration
        ref: "internal/store/store_test.go#TestListCrossSpineTotal"
        status: pass
    human_judgment: false
  - id: D5
    description: "A cross-spine response (search_memory and list_memory) names the scopes the query covered via searched_scopes, plus a scopes_truncated bool, so a zero-hit result is distinguishable from a scope-confined miss; a non-cross-spine response carries neither key at all (byte-identical to today's, D-14)."
    requirement: "REQ-cross-spine-result-provenance"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestSearchedScopesReporting"
        status: pass
    human_judgment: false
  - id: D6
    description: "Every cross-spine result carries its originating scope on both the compact view (recallView.Scope) and the full view (store.Memory.Scope), across more than one distinct scope (D-11)."
    requirement: "REQ-cross-spine-result-provenance"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestCrossSpineResultScope"
        status: pass
    human_judgment: false

duration: ~20min
completed: 2026-08-01
status: complete
---

# Phase 3 Plan 3: Cross-Spine list_memory + Searched-Scope Reporting Summary

**Wired `cross_spine=true` through `list_memory` end to end (mirroring plan 03-02's `search_memory` shape exactly), and made both cross-spine MCP verbs report `searched_scopes`/`scopes_truncated` so a zero-hit cross-spine result is distinguishable from a scope-confined miss — closing the MCP lane for wave 4's Connect wire mirror.**

## Performance

- **Duration:** ~20 min
- **Completed:** 2026-08-01
- **Tasks:** 3/3
- **Files modified:** 5

## Accomplishments

- `listFilter` (`internal/store/store.go`) now appends the `scope` match to its `Must` slice only when `scope != ""`; `ownerOrSharedCondition(subj)` stays unconditional, outside and after that conditional — identical in shape to `ownerScopeFilter`/`SearchDiscovery`, not a variant.
- `listArgs` gained `CrossSpine bool` (`json:"cross_spine,omitempty"`); `Scope` became `omitempty` with the same `required unless cross_spine` jsonschema note plan 03-02's checkpoint pre-approved for both tools.
- `coreListRequest` gained `CrossSpine bool`; `deps.listMemory` calls the REUSED `effectiveSearchScope` (no new list-specific guard written, per D-03) as its first statement and passes the resolved scope — never `req.Scope` directly — to `Store.List`.
- The `list_memory` MCP closure fast-fails on an invalid scope/cross_spine combination, logs at Info without echoing the caller-supplied scope value when `cross_spine=true` arrives with a scope set (D-02), and passes `CursorMode: true` unchanged (cursor paging keysets on `created_at` plus a boundary seen-id set, never scope — D-09).
- Audited the SECOND production path to `Store.List`: `listRules` (`rules.go:196`) is guarded by `validRuleScope` (`rules.go:189`), which already rejects an empty scope entry — `strings.HasPrefix("", prefix)` is false for either non-empty rule-scope prefix. No live widening existed; `TestListRulesRejectsEmptyScope` pins the rejection so a future relaxation of the validator fails loudly instead of silently turning `list_rules` into a store-wide read.
- `(*deps).searchedScopes(ctx, c, crossSpine)` reports the span a cross-spine query covered: `(nil, false, nil)` with no query when not cross-spine (D-13); otherwise calls `Store.ListScopes` (authz predicate alone — no recall-window/superseded/tag/category conditions, D-12) and maps `[]store.ScopeCount` to scope names, discarding counts. A `ListScopes` error fails the call rather than degrading to an empty list.
- `recallResultMap(base, crossSpine, scopes, truncated)` assembles both MCP closures' result maps: adds `searched_scopes` and `scopes_truncated` (truncated emitted even when false) ONLY on a cross-spine call; a non-cross-spine call gets neither key at all (D-14 byte-identical guarantee). Both `search_memory` and `list_memory` closures now call this one shared function.
- `TestListMemoryCrossSpineIsolation` (handler-level, mirrors `TestSearchMemoryCrossSpineIsolation`): D-16 overlapping-scope fixture, proves cross-spine spans scopes, never leaks owner B's private record, does return owner B's shared record, scope-confined list is unaffected, and empty-scope-without-cross-spine is rejected.
- `TestListRulesRejectsEmptyScope`: an empty entry in `listRulesArgs.Scopes` (alone, and mixed with a valid entry) is rejected before `Store.List` is reached.
- `TestSearchedScopesReporting`: `searchedScopes` returns a set CONTAINING both seeded scopes on cross-spine (never asserting equality, per D-12), returns `(nil, false, nil)` with no query on non-cross-spine; `recallResultMap` adds both new keys on cross-spine and neither on non-cross-spine, checked via two-value map lookups against both `search_memory`'s and `list_memory`'s base result shapes.
- `TestCrossSpineResultScope` (D-11): a cross-spine `search_memory` result set spans >=2 distinct scopes, and every result's scope matches its seeded scope on BOTH `recallView.Scope` (compact) and `store.Memory.Scope` (full).
- `TestListCrossSpine` / `TestListCrossSpineTotal` (store-level, the list analogs of `TestSearchCrossSpine`): empty scope spans both fixture scopes with owner isolation intact; the cross-spine `total` is an exact server-side Count (5, from 3+2 tagged records) strictly greater than the scope-confined total (3).

## Task Commits

Each task was committed atomically:

1. **Task 1: cross-spine `list_memory`, and close the second path to `Store.List`** — `3ba10569` (feat)
2. **Task 2: Report the span a cross-spine query covered** — `88080d3f` (feat)
3. **Task 3: Pin list scope semantics, the exact cross-spine total, and per-result attribution** — `4033d915` (test)

**Plan metadata:** committed below.

## TDD Red→Green Evidence (Task 1)

RED was observed as a genuine assertion failure, not a compile error, using the same sequencing 03-02 established: `listArgs.CrossSpine`, `coreListRequest.CrossSpine`, `deps.listMemory`'s guard call, and the `list_memory` closure's wiring were all added FIRST — so `TestListMemoryCrossSpineIsolation` compiled and ran — while `listFilter` in `store.go` was still unconditional.

```
=== RUN   TestListMemoryCrossSpineIsolation
    tools_test.go:2340: cross-spine: missing A's record in scopeShared: c5c50003-0000-0000-0000-000000000001
    tools_test.go:2343: cross-spine: missing A's record in scopeAOnly: c5c50003-0000-0000-0000-000000000002
    tools_test.go:2346: cross-spine: hits span only 0 distinct scope(s), want >1: map[]
    tools_test.go:2355: cross-spine: missing owner B's shared record (positive control): c5c50003-0000-0000-0000-000000000004
--- FAIL: TestListMemoryCrossSpineIsolation (0.14s)
```

GREEN was produced by the single `listFilter` edit (making the scope match conditional):

```
=== RUN   TestListMemoryCrossSpineIsolation
--- PASS: TestListMemoryCrossSpineIsolation (0.02s)
```

`TestListRulesRejectsEmptyScope` passed immediately (no RED expected — it pins an already-correct rejection at `validRuleScope`, per the plan's explicit "if it is, add the test" branch).

Task 2's `TestSearchedScopesReporting` failed to compile until `(*deps).searchedScopes` and `recallResultMap` were written (both introduced in a single pass with the wiring, per the plan's single-commit-per-task shape); the compile-blocked state is the RED analog here since the function under test did not exist yet. Task 3's three pins passed on first run with no RED, as the plan itself anticipates ("The implementation from Tasks 1 and 2 should already satisfy all of them").

## Gate Results

All plan-level verification gates, run after Task 3:

```
go test ./internal/store/... -run 'TestCrossSpineAuthzIsolation|TestSearchCrossSpine|TestListCrossSpine' -v -count=1
  --- PASS: TestCrossSpineAuthzIsolation (0.09s)
  --- PASS: TestSearchCrossSpine (0.01s)
  --- PASS: TestListCrossSpine (0.01s)
  --- PASS: TestListCrossSpineTotal (0.01s)

go test ./internal/server/... -run 'TestEffectiveSearchScope|TestSearchMemoryCrossSpineIsolation|TestListMemoryCrossSpineIsolation|TestSearchedScopesReporting|TestCrossSpineResultScope|TestListRulesRejectsEmptyScope' -v -count=1
  --- PASS: TestListRulesRejectsEmptyScope (0.09s)
  --- PASS: TestEffectiveSearchScope (0.00s)   [table-driven, all 4 sub-cases]
  --- PASS: TestSearchMemoryCrossSpineIsolation (0.01s)
  --- PASS: TestListMemoryCrossSpineIsolation (0.01s)
  --- PASS: TestCrossSpineResultScope (0.01s)
  --- PASS: TestSearchedScopesReporting (0.01s)

task  (lint + full suite)  -> all green
  golangci-lint, rumdl, actionlint, yamlfmt, ruff check/format: clean
  go test ./...: all packages ok (internal/server 7.195s, internal/store 8.176s)
  pytest skill/engram/hooks/tests: 33 passed

go vet ./...  -> exit 0
git diff --exit-code -- go.mod go.sum  -> exit 0 (zero new dependencies)
```

## Files Created/Modified

- `internal/store/store.go` — `listFilter`'s scope match is now conditional on `scope != ""`; `ownerOrSharedCondition` stays unconditional, outside the conditional.
- `internal/store/store_test.go` — added `TestListCrossSpine` and `TestListCrossSpineTotal`, placed immediately after `TestSearchCrossSpine`.
- `internal/server/tools.go` — `listArgs.CrossSpine`, `coreListRequest.CrossSpine`, `deps.listMemory`'s guard call, the `list_memory` closure's guard/log/pass-through wiring and Description, `(*deps).searchedScopes`, `recallResultMap`, and both closures' `result` assembly.
- `internal/server/tools_test.go` — added `TestListMemoryCrossSpineIsolation`, `TestCrossSpineResultScope`, `TestSearchedScopesReporting`, and the `cloneMap` test helper.
- `internal/server/rules_test.go` — added `TestListRulesRejectsEmptyScope`.

## Decisions Made

- `effectiveSearchScope` reused as-is for `list_memory` with zero modification — 03-02 deliberately gave it a primitive-args signature anticipating exactly this reuse; the D-03 instruction to use "the same helper" was followed literally.
- `TestSearchedScopesReporting` tests `(*deps).searchedScopes` and `recallResultMap` directly rather than through a simulated MCP tool call — see `key-decisions` in the frontmatter for the full rationale (no in-process MCP session harness exists in this codebase; `recallResultMap` IS the closures' exact map-assembly logic, so this is a direct proof, not an approximation).
- `recallResultMap` was factored out as a small shared helper (not in the plan's literal action text, which described inline map assembly in "both MCP closures") so the identical searched_scopes/scopes_truncated logic isn't duplicated across `search_memory` and `list_memory`, and so it is independently unit-testable. This is Claude's implementation discretion on HOW to wire the two closures identically, not a deviation from what the plan required (the plan's `done` criterion — "a cross-spine response tells the caller which scopes the query covered... a scope-confined response is unchanged" — is met either way).
- No other deviations from the plan's decisions (D-01 through D-18 in `03-CONTEXT.md`) or premises in its `<read_first>` blocks — every cited line range and existing function shape matched the live tree exactly.

## Deviations from Plan

None — plan executed exactly as written, with the one documented implementation-discretion choice above (the `recallResultMap` extraction) noted for transparency. No Rule 1/2/3 auto-fixes were needed; no Rule 4 architectural questions arose.

## Issues Encountered

One self-inflicted test bug during Task 2 authoring: an early draft of `TestSearchedScopesReporting` compared `map[string]any` values containing `[]any{}` with `!=`, which panics on Go's "comparing uncomparable type" at runtime. Caught immediately by the test's own first run (a real, if trivial, RED), fixed by switching to key-count and presence-only checks. Not a Rule 1/2/3 deviation — a bug in test code written this session, fixed before the commit that introduced it.

## User Setup Required

None — no external service configuration required. Docker/testcontainers Qdrant was already running and reachable throughout.

## Next Phase Readiness

- The MCP lane is now complete for both `search_memory` and `list_memory`: cross-spine argument, guard, store-layer conditional-scope composition, searched-scope reporting, and result-scope attribution are all wired and pinned by ten passing tests (four new this plan, six standing from 03-01/03-02).
- `proto/`, `gen/`, and `connectapi.go` remain untouched, exactly as this plan's acceptance criteria required — wave 4 (03-04) has only the Connect wire to mirror against an already-proven MCP contract: `cross_spine`, `searched_scopes`, `scopes_truncated` on both RPCs.
- `effectiveSearchScope` and `recallResultMap` are both generic and ready for the Connect handlers to call directly if 03-04 chooses to route them through the same typed core rather than duplicating the guard/map logic.
- No blockers for 03-04.

---
*Phase: 03-cross-spine-memory-recall*
*Completed: 2026-08-01*

## Self-Check: PASSED

- FOUND: `internal/store/store.go`
- FOUND: `internal/store/store_test.go`
- FOUND: `internal/server/tools.go`
- FOUND: `internal/server/tools_test.go`
- FOUND: `internal/server/rules_test.go`
- FOUND: `.planning/phases/03-cross-spine-memory-recall/03-03-SUMMARY.md`
- FOUND commits: `3ba10569`, `88080d3f`, `4033d915` (all present in `git log --oneline --all`)
