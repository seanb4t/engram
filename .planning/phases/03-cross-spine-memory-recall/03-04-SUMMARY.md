---
phase: 03-cross-spine-memory-recall
plan: 04
subsystem: api
tags: [protobuf, buf, connect, connectrpc, sveltekit, mcp-connect-parity]

# Dependency graph
requires:
  - phase: 03-cross-spine-memory-recall (plan 03)
    provides: "The complete, proven MCP lane: effectiveSearchScope guard, listFilter/ownerScopeFilter conditional-scope composition, (*deps).searchedScopes, recallResultMap, and ten passing tests — the exact contract this plan mirrors onto Connect."
provides:
  - "cross_spine (bool) on SearchMemoriesRequest (field 9) and ListMemoriesRequest (field 12); searched_scopes/scopes_truncated on SearchMemoriesResponse (fields 2/3) and ListMemoriesResponse (fields 5/6) — all six numbers additive, buf breaking clean against main"
  - "SearchMemories/ListMemories Connect handlers reading cross_spine EXPLICITLY via effectiveSearchScope, never inferring it from an empty scope (D-04) — an empty scope without cross_spine=true returns connect.CodeInvalidArgument, not CodeInternal and not a result set"
  - "Both handlers populate SearchedScopes/ScopesTruncated from the same (*deps).searchedScopes helper the MCP closures call, so the two lanes cannot diverge in what they report for the same query"
  - "TestConnectCrossSpineScopeRequired, TestConnectCrossSpineNotInferred, TestSearchMemoriesConnectCrossSpine — proving the D-04 non-inference, the SearchDiscoveries asymmetry as deliberate, and MCP<->Connect id-set parity (criterion 4) over one shared two-owner fixture"
affects: [03-05-docs-and-followups]

actuals:
  tokens: 6265
  tasks: 4
  commits: 4

tech-stack:
  added: []
  patterns:
    - "A hand-written wire-shape comment on cross_spine/searched_scopes/scopes_truncated states the exact semantics (authorized span, not scopes-with-hits) directly in the .proto, because that comment is what a generated-client consumer reads — getting it right there matters more than anywhere else in the plan."
    - "SearchDiscoveries' req.Msg.Scope == \"\" -> CrossSpine inference is now explicitly commented as a DELIBERATE DIVERGENCE at its declaration site, so a future reader comparing the three handlers does not 'fix' the asymmetry SearchMemories/ListMemories intentionally do not share."
    - "A proto field-count pin test (TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs, pre-existing) exists specifically to force acknowledging additive field changes rather than let them pass silently — this plan's fields tripped it as designed, and updating its counts/pins is the correct response, not a workaround."

key-files:
  created:
    - internal/server/connectapi_crossspine_test.go
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/webauth/static/** (re-vendored SPA build; not in files_modified, see Deviations)
    - internal/server/connectapi.go
    - internal/server/connectdescriptor_test.go (see Deviations)

key-decisions:
  - "Task 1's checkpoint:decision was pre-resolved by Sean (ship-as-researched) before this execution began; the plan file was amended in place with a <resolution> block recording that, per the orchestrator's instruction, rather than re-asking."
  - "Field numbers re-verified live against proto/engram/v1/engram.proto immediately before writing: SearchMemoriesRequest fields 1-8 in use -> cross_spine=9; SearchMemoriesResponse field 1 in use -> searched_scopes=2, scopes_truncated=3; ListMemoriesRequest fields 1-11 in use -> cross_spine=12; ListMemoriesResponse fields 1,2,4 in use plus DECLARED-deprecated 3 (approximate) -> searched_scopes=5 (not 3), scopes_truncated=6. buf breaking --against main confirmed clean both immediately after the edit and again after the full plan's commits."
  - "Task 3's commit contains ONLY internal/server/connectapi.go and Task 4's commit contains ONLY internal/server/connectapi_crossspine_test.go, matching each task's <files> field exactly. The TDD RED->GREEN cycle (write the test file, watch it fail with CodeInternal, then implement) happened in the working tree across both tasks; only the file-to-commit assignment follows the plan's literal <files> split."
  - "Task 2's task ui:build regenerated internal/webauth/static/ (different content hashes) because the SPA bundle embeds the changed ui/src/lib/gen TS types. This directory is CI-checked for drift by a required 'ui vendored-asset drift' job (not in files_modified, but the natural consequence of the proto edit) — committed alongside the proto/gen changes rather than left to trip CI. See Deviations, Rule 3."

patterns-established:
  - "When a proto field-count/wire-shape pin test exists specifically to catch additive schema drift (SC4-style descriptor reflection test), tripping it on an intentional additive change is the test doing its job, not a false positive — update its pinned counts/shapes in the same plan, in-scope, rather than treating it as unrelated breakage."

requirements-completed: [REQ-cross-spine-search, REQ-cross-spine-result-provenance, REQ-cross-spine-authz-verified]

coverage:
  - id: D1
    description: "cross_spine, searched_scopes, and scopes_truncated are wire-visible on SearchMemories/ListMemories via additive protobuf field numbers (9, 2/3, 12, 5/6), confirmed additive by buf breaking against main, with zero drift in the committed gen/ and ui/src/lib/gen/ trees and a successful vendored-console build."
    requirement: "REQ-cross-spine-search"
    verification:
      - kind: other
        ref: "go tool buf breaking --against 'https://github.com/seanb4t/engram.git#branch=main' (clean, both immediately after the proto edit and after the final commit)"
        status: pass
      - kind: other
        ref: "git diff --exit-code -- gen/ ui/src/lib/gen/ after a post-commit re-run of task proto:gen (zero drift)"
        status: pass
      - kind: other
        ref: "task ui:build (succeeds)"
        status: pass
    human_judgment: false
  - id: D2
    description: "SearchMemories and ListMemories reject an empty scope without cross_spine=true with connect.CodeInvalidArgument specifically (never CodeInternal, never a result set) — the D-04/T-03-02 guard, asserted on the error CODE."
    requirement: "REQ-cross-spine-authz-verified"
    verification:
      - kind: integration
        ref: "internal/server/connectapi_crossspine_test.go#TestConnectCrossSpineScopeRequired"
        status: pass
    human_judgment: false
  - id: D3
    description: "SearchMemories/ListMemories never infer cross-spine from an empty scope, even against a fixture seeded across two scopes; SearchDiscoveries' pre-existing empty-scope inference is proven unchanged in the same test, pinning the asymmetry as deliberate rather than an inconsistency to normalize (T-03-03)."
    requirement: "REQ-cross-spine-authz-verified"
    verification:
      - kind: integration
        ref: "internal/server/connectapi_crossspine_test.go#TestConnectCrossSpineNotInferred"
        status: pass
    human_judgment: false
  - id: D4
    description: "MCP<->Connect parity (criterion 4): the sorted id set returned by the Connect handler with cross_spine=true equals the set returned by the corresponding deps method, for both search and list, over one shared two-owner/two-scope fixture; every result carries a non-empty scope; searched_scopes contains both seeded scope names (containment, not equality); owner B's private record appears in neither lane; a scope-confined response carries neither searched_scopes nor scopes_truncated (D-14 byte-identical guarantee)."
    requirement: "REQ-cross-spine-result-provenance"
    verification:
      - kind: integration
        ref: "internal/server/connectapi_crossspine_test.go#TestSearchMemoriesConnectCrossSpine"
        status: pass
    human_judgment: false

duration: ~10min
completed: 2026-08-01
status: complete
---

# Phase 3 Plan 4: Connect Wire Parity for Cross-Spine Recall Summary

**Mirrored the proven MCP cross-spine lane onto the Connect wire via six additive protobuf fields (`cross_spine`, `searched_scopes`, `scopes_truncated` on `SearchMemories`/`ListMemories`), with the one deliberate divergence from `SearchDiscoveries` — explicit-field-only, never scope-inferred — pinned by a behavioral test.**

## Performance

- **Duration:** ~10 min (commit-to-commit span; task execution only)
- **Completed:** 2026-08-01
- **Tasks:** 4/4 (Task 1 checkpoint pre-resolved; Tasks 2-4 executed)
- **Files modified:** 7 (plus vendored SPA build artifacts, see Deviations)

## Accomplishments

- Added `bool cross_spine = 9` to `SearchMemoriesRequest` and `bool cross_spine = 12` to `ListMemoriesRequest`; added `repeated string searched_scopes` / `bool scopes_truncated` to `SearchMemoriesResponse` (fields 2/3) and `ListMemoriesResponse` (fields 5/6 — field 3 there is the DECLARED-deprecated `approximate`, correctly left reserved). Every field carries a comment explaining that `searched_scopes` is the authorized span, not the scopes that produced hits.
- Regenerated `gen/go/engram/v1/`, `gen/ts/engram/v1/`, and `ui/src/lib/gen/engram/v1/` in the same commit as the proto edit; `buf breaking --against main` is clean and a post-commit re-run of `task proto:gen` shows zero diff in either generated tree.
- Ran `task ui:build` (the CI-required check no phase gate normally runs) — it succeeded and re-vendored `internal/webauth/static/` since the SPA bundle embeds the changed generated TS types.
- Wired `SearchMemories` and `ListMemories` to read `cross_spine` EXPLICITLY via `effectiveSearchScope` (the same D-03/D-07 guard the typed core carries), converting a bare guard error into `connect.CodeInvalidArgument` at the boundary rather than letting it reach `connectError` and get misclassified as `CodeInternal`. Both handlers pass `CrossSpine` through to `coreSearchRequest`/`coreListRequest` and populate `SearchedScopes`/`ScopesTruncated` from `(*deps).searchedScopes` — the identical helper both MCP closures use.
- Left `SearchDiscoveries` untouched and added an inline comment at its declaration recording that its `req.Msg.Scope == ""` inference is a DELIBERATE DIVERGENCE, not a pattern to copy onto the memory RPCs.
- Added `internal/server/connectapi_crossspine_test.go` with three tests: `TestConnectCrossSpineScopeRequired` (error-code pin), `TestConnectCrossSpineNotInferred` (the D-04 anti-pattern guard plus the SearchDiscoveries asymmetry pin), and `TestSearchMemoriesConnectCrossSpine` (criterion-4 id-set parity for both search and list, per-result scope attribution, `searched_scopes` containment, owner-B leak check, and the scope-confined no-coverage-keys check).
- Fixed a pre-existing pinned-field-count descriptor test (`TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs`) that this plan's additive fields correctly tripped — updated its expected counts and added wire-shape pins for the two new response fields on each message (Deviations, Rule 1).

## Task Commits

Each task was committed atomically:

1. **Task 1: Confirm the six permanent protobuf field numbers** — checkpoint pre-resolved by Sean (`ship-as-researched`); plan file amended in place with a `<resolution>` block, no code commit.
2. **Task 2: Add the six additive proto fields and regenerate both client trees** — `ac457527` (feat)
3. **Task 3: Wire both Connect handlers — explicitly, never by inference** — `0c8a3e22` (feat)
4. **Task 4: Pin MCP<->Connect parity and the deliberate non-inference** — `84336ec7` (test)
5. **Deviation fix: update pinned proto field-count assertions** — `422f1ad3` (fix, Rule 1)

**Plan metadata:** committed below.

## TDD Red->Green Evidence (Task 3)

RED was observed as a genuine assertion failure via error-code misclassification, not a compile error: `internal/server/connectapi_crossspine_test.go` was written in full (covering Task 3's behavior bullets and Task 4's parity/asymmetry tests together, since Task 4 "completes" the same file Task 3 drives) against the Task 2 tree, before either handler read `cross_spine`.

```
=== RUN   TestConnectCrossSpineScopeRequired/SearchMemories
    connectapi_crossspine_test.go:80: SearchMemories empty scope, cross_spine=false: got internal: internal error, want CodeInvalidArgument
=== RUN   TestConnectCrossSpineScopeRequired/ListMemories
    connectapi_crossspine_test.go:89: ListMemories empty scope, cross_spine=false: got internal: internal error, want CodeInvalidArgument
--- FAIL: TestConnectCrossSpineScopeRequired
=== RUN   TestConnectCrossSpineNotInferred/SearchMemories_empty_scope_errors_not_cross_spine
    connectapi_crossspine_test.go:117: SearchMemories empty scope: got code internal, want CodeInvalidArgument
--- FAIL: TestConnectCrossSpineNotInferred
=== RUN   TestSearchMemoriesConnectCrossSpine/search
    connectapi_crossspine_test.go:236: Connect SearchMemories cross-spine: internal: internal error
--- FAIL: TestSearchMemoriesConnectCrossSpine
```

This is exactly the failure mode the plan predicted: a bare guard error reaching `connectError` and being misclassified as `CodeInternal` rather than surfaced as `CodeInvalidArgument`. GREEN was produced by the `effectiveSearchScope` boundary call plus the `CrossSpine`/`searchedScopes` wiring in both handlers:

```
--- PASS: TestConnectCrossSpineScopeRequired (0.10s)
    --- PASS: TestConnectCrossSpineScopeRequired/SearchMemories (0.00s)
    --- PASS: TestConnectCrossSpineScopeRequired/ListMemories (0.00s)
--- PASS: TestConnectCrossSpineNotInferred (0.02s)
    --- PASS: TestConnectCrossSpineNotInferred/SearchMemories_empty_scope_errors_not_cross_spine (0.00s)
    --- PASS: TestConnectCrossSpineNotInferred/ListMemories_empty_scope_errors_not_cross_spine (0.00s)
    --- PASS: TestConnectCrossSpineNotInferred/SearchDiscoveries_empty_scope_still_infers_cross_spine (0.00s)
    --- PASS: TestConnectCrossSpineNotInferred/scope_confined_search_still_works (0.00s)
--- PASS: TestSearchMemoriesConnectCrossSpine (0.02s)
    --- PASS: TestSearchMemoriesConnectCrossSpine/search (0.00s)
    --- PASS: TestSearchMemoriesConnectCrossSpine/list (0.00s)
    --- PASS: TestSearchMemoriesConnectCrossSpine/scope_confined_carries_no_coverage_keys (0.00s)
```

## Gate Results

All plan-level verification gates, run after the deviation fix:

```
task proto:lint                                          -> clean (buf lint + NO_SIDE_EFFECTS grep ban)
go tool buf breaking --against main                       -> clean, no breaking change (both after Task 2 and after final commit)
task proto:gen && git diff --exit-code gen/ ui/src/lib/gen/  -> zero drift (post-commit re-run)
task ui:build                                             -> succeeds
go vet ./...                                              -> exit 0
git diff --exit-code -- go.mod go.sum                      -> clean, zero new dependencies

go test ./internal/server/... -run 'TestConnectCrossSpineNotInferred|TestConnectCrossSpineScopeRequired|TestSearchMemoriesConnectCrossSpine' -v -count=1
  --- PASS: TestConnectCrossSpineScopeRequired (both sub-tests)
  --- PASS: TestConnectCrossSpineNotInferred (all four sub-tests)
  --- PASS: TestSearchMemoriesConnectCrossSpine (all three sub-tests)

task  (lint + full suite)  -> all green
  golangci-lint, rumdl, actionlint, yamlfmt, ruff check/format: clean
  go test ./...: all packages ok (internal/server 5.478s)
  pytest skill/engram/hooks/tests: 33 passed
```

## Files Created/Modified

- `proto/engram/v1/engram.proto` — six additive fields across the four memory request/response messages, each with a comment stating the authorized-span semantics for `searched_scopes`.
- `gen/go/engram/v1/engram.pb.go`, `gen/ts/engram/v1/engram_pb.ts`, `ui/src/lib/gen/engram/v1/engram_pb.ts` — regenerated, zero drift.
- `internal/webauth/static/**` — re-vendored by `task ui:build` (chunk-hash renames + `index.html`); not in the plan's `files_modified` but committed to satisfy CI's required "ui vendored-asset drift" job (Deviations, Rule 3).
- `internal/server/connectapi.go` — `SearchMemories`/`ListMemories` read `cross_spine` explicitly via `effectiveSearchScope`, pass it through to the typed core, and populate `SearchedScopes`/`ScopesTruncated` from `(*deps).searchedScopes`; `SearchDiscoveries` gained a comment documenting its divergence but no behavior change.
- `internal/server/connectapi_crossspine_test.go` (new) — the three cross-spine Connect tests plus the shared `crossSpineFixture` and `connectCtxFor` helpers.
- `internal/server/connectdescriptor_test.go` — updated pinned field counts/wire-shapes for `ListMemoriesRequest`/`Response` and `SearchMemoriesRequest`/`Response` to reflect the six new fields (Deviations, Rule 1).

## Decisions Made

- Task 1's checkpoint was pre-resolved by Sean before this run; recorded in the plan file with a `<resolution>` block rather than re-litigated.
- Field numbers verified live against the file (not just the plan's research) immediately before writing, per the plan's own instruction — matched exactly, including the `ListMemoriesResponse` field-3-is-taken subtlety.
- Task 3's commit is `internal/server/connectapi.go` only; Task 4's commit is `internal/server/connectapi_crossspine_test.go` only — matching each task's `<files>` field exactly, even though the TDD RED observation (writing the test file, watching it fail) necessarily happened before Task 3's implementation commit in the working tree.
- `searchedScopes`/`scopesTruncated` are always assigned from the helper's return values directly (never behind an `if crossSpine` branch) on both handlers, per the plan's explicit instruction that `(nil, false, nil)` already serializes as absent under proto3 — no redundant omission branch.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Committed the re-vendored `internal/webauth/static/` alongside the proto/gen changes**
- **Found during:** Task 2 (`task ui:build` verification step)
- **Issue:** `task ui:build` regenerated `internal/webauth/static/` with different asset-chunk content hashes, because the SPA bundle embeds `ui/src/lib/gen`'s changed generated TS types. This directory is not in the plan's `files_modified` list, but CI runs a required "ui vendored-asset drift" job (`.github/workflows/ci.yaml:155-182`) that fails the PR if it's stale.
- **Fix:** Committed the re-vendored directory in the same commit as the proto/gen changes (Task 2's commit), matching the pattern the CI job itself expects (`git diff --exit-code internal/webauth/static/`).
- **Files modified:** `internal/webauth/static/**` (chunk renames + `index.html`).
- **Verification:** `task ui:build` succeeds; the CI drift check's own comparison method (`git diff --exit-code internal/webauth/static/`) was reasoned through manually — the committed tree now matches what a fresh `pnpm build` produces.
- **Committed in:** `ac457527` (Task 2 commit).

**2. [Rule 1 - Bug] Updated a pre-existing pinned-field-count descriptor test**
- **Found during:** the `task` (lint + full suite) plan-level gate, run after Task 4.
- **Issue:** `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` (`internal/server/connectdescriptor_test.go`) asserts exact field counts and per-field wire shapes on `ListMemoriesRequest`/`Response` and `SearchMemoriesRequest`/`Response` as an SC4 drift guard. This plan's six additive fields on exactly those four messages correctly tripped it (field counts 11->12, 4->6, 8->9, 1->3).
- **Fix:** Updated the four `assertFields` calls to the new counts and added wire-shape pins (`name`, `kind: StringKind`, `repeated: true` for `searched_scopes`; `kind: BoolKind` for `scopes_truncated`) for the two new fields on each response message.
- **Files modified:** `internal/server/connectdescriptor_test.go`.
- **Verification:** `TestEngramServiceDescriptor_ReadLaneUnaffectedAndNoSideEffectsRPCs` passes; full `task` gate green afterward.
- **Committed in:** `422f1ad3` (separate fix commit, since it was discovered after Task 4's commit during the plan-level gate run).

---

**Total deviations:** 2 auto-fixed (1 blocking/CI-required-check, 1 bug/pre-existing-test-drift)
**Impact on plan:** Both were direct, necessary consequences of the plan's own intentional changes (a proto edit touching generated TS; additive fields on messages a drift-guard test pins) — no scope creep, no architectural decisions, no Rule 4 escalation needed.

## Issues Encountered

None beyond the two deviations above, both resolved within the fix-attempt limit on first diagnosis.

## User Setup Required

None — no external service configuration required. Docker/testcontainers Qdrant, the Node/pnpm toolchain, and GitHub reachability (for `buf breaking --against main`) were all available throughout.

## Next Phase Readiness

- ROADMAP criterion 4 (MCP<->Connect parity via additive proto fields) is satisfied: `cross_spine` is available on both transports, proven by set-equality parity over a shared fixture, not just symmetric argument shapes.
- The phase's second and last one-way door (the six field numbers) is closed, confirmed additive by `buf breaking` at two points (immediately after the edit and after the full plan's final commit).
- `SearchDiscoveries`' asymmetric empty-scope inference is now documented in two places (the handler comment and the pinned test) as deliberate, closing the risk that a future contributor "fixes" it into false consistency.
- No blockers for 03-05 (docs and remaining phase follow-ups per the affects list).

---
*Phase: 03-cross-spine-memory-recall*
*Completed: 2026-08-01*

## Self-Check: PASSED

- FOUND: `proto/engram/v1/engram.proto`
- FOUND: `internal/server/connectapi.go`
- FOUND: `internal/server/connectapi_crossspine_test.go`
- FOUND: `internal/server/connectdescriptor_test.go`
- FOUND: `.planning/phases/03-cross-spine-memory-recall/03-04-SUMMARY.md`
- FOUND commits: `ac457527`, `0c8a3e22`, `84336ec7`, `422f1ad3` (all present in `git log --oneline --all`)
