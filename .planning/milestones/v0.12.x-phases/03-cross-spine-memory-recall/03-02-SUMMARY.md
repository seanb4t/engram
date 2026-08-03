---
phase: 03-cross-spine-memory-recall
plan: 02
subsystem: api
tags: [qdrant, mcp, authorization, testing, testcontainers]

# Dependency graph
requires:
  - phase: 03-cross-spine-memory-recall (plan 01)
    provides: "TestCrossSpineAuthzIsolation — the standing authz pin that made this plan's ownerScopeFilter edit safe to land"
provides:
  - "search_memory cross_spine=true: spans every scope the caller may read, wired end to end from the MCP tool argument through the D-03 guard to Store.Search"
  - "effectiveSearchScope: the D-03/D-07 guard, applied at both the MCP closure and deps.searchMemory (the last chokepoint before the store)"
  - "ownerScopeFilter with a conditional scope clause and an unconditional authz clause, mirroring Store.SearchDiscovery's shape"
  - "TestSearchMemoryCrossSpineIsolation (handler-level, D-17) and TestSearchCrossSpine (store-level, wiring proof) as standing regression pins"
affects: [03-03-list-cross-spine, 03-04-connect-proto-parity]

actuals:
  tokens: 5100
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "The D-03/D-07 guard (effectiveSearchScope) is applied twice on the same request — once at the transport boundary (MCP closure, for the fast-fail error path) and once at the typed core (deps.searchMemory, the true chokepoint) — because the typed core is what every future server-side caller to Store.Search must pass through, not just the MCP closure."
    - "A store-layer filter builder with exactly one production call site (ownerScopeFilter -> Store.Search) can have its scope clause made conditional as a local, in-place edit, provided the authz clause is kept outside and after that conditional — the shape Store.SearchDiscovery already established."
    - "Cross-spine fixture discipline for a shared-collection integration test: two owners over an OVERLAPPING scope name (so a dropped owner clause returns the wrong owner's records rather than silently nothing), plus one unique fixture tag per test appended to SearchOptions.Tags so the test's assertions are immune to whatever else the package-shared mem_eval_test collection holds."

key-files:
  created: []
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/server/tools.go
    - internal/server/tools_test.go

key-decisions:
  - "TDD RED observed as a genuine assertion failure, not a compile error: all of searchArgs.CrossSpine, coreSearchRequest.CrossSpine, effectiveSearchScope, and the MCP closure's wiring were added BEFORE the store.go edit, so TestSearchMemoryCrossSpineIsolation compiled and ran — and failed on 'missing A's record in scopeShared' etc. — against the still-unconditional ownerScopeFilter. Only after that observed RED was the store.go conditional-scope edit made, turning the test GREEN."
  - "Task 2 committed as a single feat commit per the plan's explicit instruction ('Commit as one change'), rather than the generic TDD test/feat split — the plan's per-task commit directive takes precedence over the default RED/GREEN commit convention for this task."
  - "effectiveSearchScope takes primitives (scope string, crossSpine bool) rather than mirroring effectiveDiscoveryScope's single-arg-struct shape, because by the end of the phase it will serve four call sites across three different request/arg types; noted as a deliberate divergence in its doc comment."

patterns-established:
  - "Guard-at-both-ends: a D-03-style rejection guard is applied at the transport boundary (fast user-facing error) AND at the typed core (the actual last chokepoint before the store), so no future server-side call path can reach the store with an unguarded value even if it bypasses the transport closure."

requirements-completed: [REQ-cross-spine-search, REQ-cross-spine-authz-verified]

coverage:
  - id: D1
    description: "An agent calling search_memory with cross_spine=true gets back hits from more than one scope it is permitted to read, spanning the MCP tool argument, the D-03 guard, the transport-neutral core, the typed core, and the ownerScopeFilter composition Qdrant evaluates."
    requirement: "REQ-cross-spine-search"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestSearchMemoryCrossSpineIsolation"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestSearchCrossSpine"
        status: pass
    human_judgment: false
  - id: D2
    description: "search_memory with cross_spine omitted or false returns only the named scope; every existing caller's behavior is unchanged, and an empty scope without cross_spine is rejected with 'scope is required unless cross_spine is true' at both the MCP closure and the typed core."
    requirement: "REQ-cross-spine-search"
    verification:
      - kind: unit
        ref: "internal/server/tools_test.go#TestEffectiveSearchScope"
        status: pass
      - kind: integration
        ref: "internal/server/tools_test.go#TestSearchMemoryCrossSpineIsolation (scope-confined sub-assertion)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Owner A's cross-spine search_memory never returns owner B's private record, and does return owner B's shared record (positive control), proven through the handler seam against real Qdrant over an overlapping scope name (D-17); the pre-existing store-level authz proof (TestCrossSpineAuthzIsolation, 03-01) still passes unchanged after the ownerScopeFilter edit."
    requirement: "REQ-cross-spine-authz-verified"
    verification:
      - kind: integration
        ref: "internal/server/tools_test.go#TestSearchMemoryCrossSpineIsolation"
        status: pass
      - kind: integration
        ref: "internal/store/store_test.go#TestCrossSpineAuthzIsolation"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-08-01
status: complete
---

# Phase 3 Plan 2: Cross-Spine search_memory Tracer Summary

**Wired `cross_spine=true` through `search_memory` end to end on the MCP lane — one tracer path from the tool argument through `effectiveSearchScope` to a now-conditional `ownerScopeFilter`, with a TDD RED observed as a real assertion failure before the store-layer edit, and both a handler-level and a store-level isolation/wiring pin.**

## Performance

- **Duration:** ~12 min
- **Completed:** 2026-08-01
- **Tasks:** 2/2 (Task 1 was a checkpoint:decision, pre-resolved by Sean as `ship-as-researched` before this execution began)
- **Files modified:** 4

## Accomplishments

- `ownerScopeFilter` (`internal/store/store.go`) now appends the `scope` match to its `Must` slice only when `scope != ""`; `ownerOrSharedCondition(subj)` is appended unconditionally, outside and after that conditional — mirroring `Store.SearchDiscovery`'s already-shipped shape exactly, not inventing a variant.
- `effectiveSearchScope(scope string, crossSpine bool) (string, error)` added beside `effectiveDiscoveryScope`: cross-spine returns `("", nil)`; non-cross-spine with an empty scope returns an error naming `cross_spine`; otherwise the scope passes through unchanged.
- `searchArgs` gained `CrossSpine bool` (`json:"cross_spine,omitempty"`); `Scope` became `omitempty` with a `required unless cross_spine` jsonschema note — the exact contract Task 1's checkpoint approved, mirroring `searchDiscoveryArgs` byte for byte.
- `coreSearchRequest` gained `CrossSpine bool`; `deps.searchMemory` calls `effectiveSearchScope` as its first statement and passes the RESOLVED scope — never `req.Scope` directly — to `Store.SearchReranked`, making the typed core the last chokepoint before the store (closing the hole D-07 deliberately leaves open by declining a store-level flag).
- The `search_memory` MCP closure calls `effectiveSearchScope` for the fast-fail error path, and logs at Info (no scope-value interpolation, D-02) when `cross_spine=true` arrives alongside a non-empty scope — copying `deps.searchDiscovery`'s discipline verbatim.
- `TestSearchMemoryCrossSpineIsolation` (handler-level, `internal/server/tools_test.go`): two owners over an overlapping scope name (D-16), one fixture tag unique to the test. Proves cross-spine spans scopes, never leaks owner B's private record, does return owner B's shared record (positive control), and scope-confined search is unaffected.
- `TestSearchCrossSpine` (store-level, `internal/store/store_test.go`): the wiring proof, distinct from `TestCrossSpineAuthzIsolation`'s authz proof — one owner, two scopes, two records per scope, asserts on the set of distinct `Scope` values returned.
- `TestEffectiveSearchScope` (`internal/server/tools_test.go`): table-driven over all four `(scope, crossSpine)` combinations, pinning the D-03/D-07 guard.

## Task Commits

Each task was committed atomically:

1. **Task 1: Confirm the published MCP tool-argument schema** — checkpoint:decision, pre-resolved by Sean (`ship-as-researched`, 2026-08-01) before this execution began. No commit; recorded here per the checkpoint-already-resolved instruction.
2. **Task 2: End-to-end "cross-spine search_memory"** — `9d763790` (feat)
3. **Task 3: Pin the guard and the store-level scope semantics** — `741ee457` (test)

**Plan metadata:** committed below.

## TDD Red→Green Evidence (Task 2)

RED was observed as a genuine assertion failure, not a compile error, by sequencing the edit: `searchArgs.CrossSpine`, `coreSearchRequest.CrossSpine`, `effectiveSearchScope`, `deps.searchMemory`'s guard call, and the MCP closure's wiring were all added FIRST — so `TestSearchMemoryCrossSpineIsolation` compiled and ran — while `ownerScopeFilter` in `store.go` was still unconditional (literal `scope==""` match).

```
=== RUN   TestSearchMemoryCrossSpineIsolation
    tools_test.go:2199: cross-spine: missing A's record in scopeShared: c5c50001-0000-0000-0000-000000000001
    tools_test.go:2202: cross-spine: missing A's record in scopeAOnly: c5c50001-0000-0000-0000-000000000002
    tools_test.go:2205: cross-spine: hits span only 0 distinct scope(s), want >1: map[]
    tools_test.go:2215: cross-spine: missing owner B's shared record (positive control): c5c50001-0000-0000-0000-000000000004
--- FAIL: TestSearchMemoryCrossSpineIsolation (0.13s)
```

GREEN was then produced by the single `ownerScopeFilter` edit (making the scope match conditional):

```
=== RUN   TestSearchMemoryCrossSpineIsolation
--- PASS: TestSearchMemoryCrossSpineIsolation (0.14s)
```

Both edits (guard plumbing + store conditional) landed in Task 2's single commit `9d763790`, per the plan's explicit "Commit as one change" instruction — the RED/GREEN cycle governed the development sequence, not the commit boundary.

## Gate Results

All plan-level verification gates, run at the end of Task 3:

```
go test ./internal/store/... -run 'TestCrossSpineAuthzIsolation|TestSearchCrossSpine' -v -count=1
  --- PASS: TestCrossSpineAuthzIsolation (0.13s)
  --- PASS: TestSearchCrossSpine (0.02s)

go test ./internal/server/... -run 'TestEffectiveSearchScope|TestSearchMemoryCrossSpineIsolation' -v -count=1
  --- PASS: TestEffectiveSearchScope (0.00s)
  --- PASS: TestSearchMemoryCrossSpineIsolation (0.12s)

task  (lint + full suite)  -> all green, including internal/server and internal/store integration suites

go vet ./...  -> exit 0
git diff --exit-code -- go.mod go.sum  -> exit 0 (zero new dependencies)
```

D-18 ordering evidence: `git log --oneline -- internal/store/store.go` shows `737178e2` (03-01's isolation test commit) strictly before `9d763790` (this plan's `ownerScopeFilter` commit) — the isolation test genuinely preceded the widening edit.

## Files Created/Modified

- `internal/store/store.go` — `ownerScopeFilter`'s scope match is now conditional on `scope != ""`; `ownerOrSharedCondition` stays unconditional, outside the conditional.
- `internal/store/store_test.go` — added `TestSearchCrossSpine` (store-level wiring proof), placed immediately after `TestCrossSpineAuthzIsolation`.
- `internal/server/tools.go` — `searchArgs.CrossSpine`, `coreSearchRequest.CrossSpine`, new `effectiveSearchScope`, `deps.searchMemory`'s guard call, the `search_memory` MCP closure's guard/log/pass-through wiring, and its Description string.
- `internal/server/tools_test.go` — added `TestSearchMemoryCrossSpineIsolation` (handler-level isolation proof) and `TestEffectiveSearchScope` (table-driven guard pin).

## Decisions Made

- TDD RED sequencing: implement all guard/plumbing code before the store.go conditional edit, so the RED observation is a real assertion failure (see TDD Red→Green Evidence above) rather than a stand-in compile error. This was Claude's discretion on HOW to sequence the single-commit task, not a deviation from what the plan required.
- `effectiveSearchScope` signature: `(scope string, crossSpine bool) (string, error)` rather than mirroring `effectiveDiscoveryScope`'s single-arg-struct shape — explicitly anticipated and permitted by the plan (action item 3), since this helper will serve `list_memory`'s two call sites in addition to `search_memory`'s two, once 03-03 lands.
- No other deviations from the plan's decisions (D-01 through D-18 in `03-CONTEXT.md`, and Task 1's pre-resolved checkpoint, were premises this plan operated under).

## Deviations from Plan

None — plan executed exactly as written. Every premise cited in the plan's `<read_first>` blocks (line numbers for `ownerScopeFilter`, `SearchDiscovery`, `searchArgs`, `searchDiscoveryArgs`, `effectiveDiscoveryScope`, `deps.searchDiscovery`, `coreSearchRequest`, `deps.searchMemory`, the `search_memory` MCP closure, and the test fixture helpers) matched the live tree exactly as described. No Rule 1/2/3 auto-fixes were needed.

## Issues Encountered

None. `gofmt` reported no formatting issues on any touched file; `task` (lint + full suite) passed clean on the first run after Task 3's edits.

## User Setup Required

None — no external service configuration required. Docker/testcontainers Qdrant was already running and reachable throughout.

## Next Phase Readiness

- `search_memory` cross-spine is fully wired and pinned by four tests (two new handler/store proofs plus the two standing 03-01 proofs), giving plan 03-03 a proven pattern to replicate for `list_memory`'s `listFilter` and `listArgs`.
- Task 1's checkpoint already confirmed the identical `CrossSpine bool` / `json:"cross_spine,omitempty"` / `required unless cross_spine` contract for `listArgs`, so 03-03 does not need to re-open that decision.
- `effectiveSearchScope`'s primitive-args signature is documented as intentionally generic (four call sites by end of phase); 03-03 can call it directly for `list_memory`'s guard rather than writing a parallel helper.
- `listFilter`, `listArgs`, the `list_memory` closure, `proto/`, `gen/`, and `connectapi.go` remain untouched, exactly as this plan's acceptance criteria required — no scope leakage into 03-03/03-04's territory.
- No blockers for 03-03.

---
*Phase: 03-cross-spine-memory-recall*
*Completed: 2026-08-01*

## Self-Check: PASSED

- FOUND: `internal/store/store.go`
- FOUND: `internal/store/store_test.go`
- FOUND: `internal/server/tools.go`
- FOUND: `internal/server/tools_test.go`
- FOUND: `.planning/phases/03-cross-spine-memory-recall/03-02-SUMMARY.md`
- FOUND commits: `9d763790`, `741ee457` (both present in `git log --oneline --all`)
