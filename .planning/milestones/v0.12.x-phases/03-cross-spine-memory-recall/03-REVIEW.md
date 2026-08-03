---
phase: 03-cross-spine-memory-recall
reviewed: 2026-08-01T00:00:00Z
depth: deep
files_reviewed: 12
files_reviewed_list:
  - CLAUDE.md
  - docs-site/src/content/docs/reference/tools.md
  - gen/go/engram/v1/engram.pb.go
  - gen/ts/engram/v1/engram_pb.ts
  - internal/server/connectapi.go
  - internal/server/connectapi_crossspine_test.go
  - internal/server/connectdescriptor_test.go
  - internal/server/rules_test.go
  - internal/server/tools.go
  - internal/server/tools_test.go
  - internal/store/store.go
  - internal/store/store_test.go
  - skill/engram/skills/curating-memory/SKILL.md
findings:
  critical: 0
  warning: 0
  info: 2
  total: 2
status: clean
---

# Phase 3: Code Review Report — Cross-Spine Memory Recall

**Reviewed:** 2026-08-01
**Depth:** deep
**Files Reviewed:** 12 (+ vendored SPA rebuild noise, not functionally reviewed)
**Status:** clean

## Summary

This phase widens `search_memory`/`list_memory` to an opt-in `cross_spine` mode by making the
scope clause in `ownerScopeFilter` and `listFilter` conditional, while the authz clause
(`ownerOrSharedCondition`) stays a separate, unconditional `Must` element. That is the one property
this phase cannot get wrong, and it does not get it wrong — verified three independent ways, not
just by reading:

1. **Static read of both functions** (`internal/store/store.go:761-769`, `:1066-1078`): the scope
   match is inside `if scope != ""`; `ownerOrSharedCondition(subj)` is appended unconditionally
   immediately after, exactly matching D-05/D-06 and the pre-existing `SearchDiscovery` shape.
2. **Live RED-by-mutation, reproduced independently of the plan's own transcript.** I temporarily
   struck the `ownerOrSharedCondition` element from (a) the store-level isolation test's own filter
   construction and (b) the real `ownerScopeFilter` production code, and ran the pinned tests against
   real Qdrant (testcontainers) both times:
   - `TestCrossSpineAuthzIsolation` failed with `leaked owner B's private record ... c5c50000-...002`
     when the test's own filter had no authz element.
   - `TestSearchMemoryCrossSpineIsolation` (handler-level, D-17's second required test) failed with
     `leaked owner B's private record: c5c50001-...003` when `ownerScopeFilter` itself had the authz
     append removed.
   Both files were restored to a byte-identical `git diff` (empty) afterward. This is the strongest
   form of evidence against a vacuous-green isolation gate, and it holds.
3. **Full test run** of every new cross-spine test (`internal/store` and `internal/server`, both MCP
   and Connect lanes) against the actual tree: all pass. `go build ./...`, `go vet ./...`, `buf lint`,
   and `buf breaking --against 1339164b` are all clean; the additive proto fields introduce zero
   breaking changes and the committed `gen/` tree matches `proto/` exactly (field numbers, names,
   kinds all cross-checked against `gen/go` and `gen/ts`).

Beyond the authz clause itself, I checked every item in the risk brief and found no defect:

- **Sole chokepoint (D-07):** every call site that can reach `Store.Search`/`Store.List` with a
  caller-influenced scope passes through `effectiveSearchScope` (`tools.go`, both MCP closures and
  both Connect handlers) — except `deps.listRules` (`internal/server/rules.go:184-192`), which was
  independently confirmed to reject an empty scope entry via `validRuleScope`'s prefix check (a
  string with no `rule:repo:`/`rule:project:` prefix, including `""`, never matches), and that
  rejection is now pinned by `TestListRulesRejectsEmptyScope` (including the mixed-entry case, so a
  valid scope earlier in the list can't mask a later empty one).
- **D-04 non-inference:** read `SearchMemories`/`ListMemories` directly — both read
  `req.Msg.CrossSpine` as an explicit field and never derive it from `Scope == ""`; the pre-existing
  `SearchDiscoveries` inference at `connectapi.go:321` is untouched and is now explicitly commented as
  a deliberate, non-transferable divergence, pinned by `TestConnectCrossSpineNotInferred`.
- **Information disclosure via `searched_scopes`:** `Store.ListScopes` filters on
  `ownerOrSharedCondition(subj)` alone (`store.go:1401`) — the same authz predicate a cross-spine
  query runs under, so a caller can never see a scope name they couldn't otherwise read into. The
  D-02 log lines (`tools.go:1583`, `:1630`) do not interpolate the caller-supplied scope value; grepped
  to confirm no format-string leak.
- **Response-shape compatibility (D-14):** `recallResultMap` only adds `searched_scopes`/
  `scopes_truncated` when `crossSpine` is true, so a scope-confined MCP call's result map is
  unchanged; on Connect, both new fields are proto3 scalars/repeated whose zero value already omits
  from the wire, and `TestConnectCrossSpineNotInferred`/`TestSearchMemoriesConnectCrossSpine` both
  assert `len(SearchedScopes) == 0 && !ScopesTruncated` on scope-confined calls.
- **Tests that cannot fail:** every new isolation test uses D-16's overlapping-scope-name fixture
  with a positive control (owner B's *shared* record must appear) alongside the negative assertion
  (owner B's *private* record must never appear), so an accidental zero-result regression reads as a
  failure, not a pass. `TestCrossSpineAuthzIsolation` additionally guards against the shared-collection
  scroll-page-truncation failure mode explicitly (`if len(pts) >= limit { t.Fatalf(...) }`).

## Info

### IN-01: `effectiveSearchScope` is invoked twice per request on both lanes

**File:** `internal/server/tools.go:1576-1584` (search_memory closure), `:1622` (list_memory closure,
same pattern); `internal/server/connectapi.go:172`, `:242`

**Issue:** Both the MCP closures and the Connect handlers call `effectiveSearchScope(scope,
crossSpine)` once at the top (to get an early, correctly-classified error — `CodeInvalidArgument` on
Connect, and to gate the D-02 log line on MCP) and discard the result, then call `deps.searchMemory`/
`deps.listMemory`, which call it again internally to get the actual resolved scope. This is
documented and deliberate (the Connect comments explain the error-code-fidelity reasoning), and
since the function is pure over the same inputs both calls are guaranteed to agree — this is not a
correctness bug. It is a minor duplication that a future refactor could resolve by having the
first call's resolved value threaded through, rather than relying on determinism to keep two call
sites in sync.

**Fix:** No action required. If touched again, consider having the boundary call return the
resolved scope and pass it through `coreSearchRequest`/`coreListRequest` instead of the raw
`(Scope, CrossSpine)` pair, so there is one evaluation instead of two provably-identical ones.

### IN-02: A `ListScopes` failure discards an already-successful cross-spine search's hits

**File:** `internal/server/tools.go:1592-1595` (search_memory), `:1633-1636` (list_memory);
`deps.searchedScopes` doc comment at `tools.go:1218-1220`

**Issue:** On a cross-spine call, `d.searchMemory`/`d.listMemory` runs first and can succeed with
real hits; the handler then calls `d.searchedScopes`, which issues a second, independent
`Store.ListScopes` query. If that second query errors (e.g. a transient Qdrant hiccup), the handler
returns the error and the caller gets nothing back — the valid search results computed a moment
earlier are discarded rather than returned with `searched_scopes` omitted or degraded. This is a
deliberate, documented trade-off (`tools.go:1218-1220`: "An error from ListScopes fails the call
rather than degrading to an empty list... an empty searched_scopes would read as 'searched
nothing'"), not an oversight, and it does not weaken authorization in any way. It is a pure
availability trade-off: a transient failure in the coverage-reporting side-query now fails a request
that would otherwise have succeeded.

**Fix:** No action required if the trade-off is intentional (it reads as intentional and
well-reasoned in the code). If this surfaces in practice as flaky `search_memory`/`list_memory`
failures correlated with `ListScopes` scroll load, consider returning the already-computed hits with
`scopes_truncated` (or an explicit sentinel) rather than failing the whole call — a smaller
enhancement, not a phase-3 defect.

---

_Reviewed: 2026-08-01_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
