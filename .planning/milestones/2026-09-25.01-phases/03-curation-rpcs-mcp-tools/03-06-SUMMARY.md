---
phase: 03-curation-rpcs-mcp-tools
plan: 06
subsystem: api
tags: [connectrpc, mcp-tools, proto, buf, tags, facet]

requires:
  - phase: 03-curation-rpcs-mcp-tools
    provides: "plan 03-01's option-a wire contract confirmation, item 5, plus 2026-09-25.01 Phase 1's Store.ListTags/TagCount (D-13..D-15)"
provides:
  - "ListTags Connect RPC, a read Procedure (CSRF-exempt), delegating to the same deps.listTags core the list_tags MCP tool calls (RPC-04, RPC-05)"
  - "list_tags MCP tool framed for tag reuse (D-03): check a scope's existing tags before store_memory rather than invent a near-duplicate; also useful for choosing a search_memory/list_memory tags filter"
  - "No server-side prefix filter (D-14): Phase 5's tag-chip autocomplete filters the returned top-N list on the client"
affects: [03-07-curation-rpcs-mcp-tools]

actuals:
  tokens: 30413
  tasks: 2
  commits: 2
  plan_head_before: 75f535c7c6c6bf0f3b6df7a0e6a53d7d1e0a4b1d

tech-stack:
  added: []
  patterns:
    - "tags.go/tags_test.go mirror related.go/related_test.go's shape exactly (03-05's precedent): a single-file shared core (deps.listTags) with no post-filter above the store, a flat tagCountView wire shape for MCP, and a real-Qdrant round-trip test proving the core end to end before the isolation/parity task even starts — both isolation and parity passed on FIRST run in Task 2, because Task 1 already implemented Subject-threading correctly."

key-files:
  created:
    - internal/server/tags.go
    - internal/server/tags_test.go
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/go/engram/v1/engramv1connect/engram.connect.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/server/store_iface.go
    - internal/server/fakestore_test.go
    - internal/server/connectapi.go
    - internal/server/protoconv.go
    - internal/server/tools.go
    - internal/surfaces/toolclass.go
    - internal/surfaces/toolclass_test.go
    - internal/server/registertools_test.go
    - internal/e2e/boot_test.go
    - internal/server/connectapi_parity_test.go
    - internal/server/connectcsrf_test.go
    - docs-site/src/content/docs/reference/tools.md

key-decisions:
  - "ListTagsRequest.scope carries a TRAILING comment only, per the plan's own conformance-trap note: a leading comment on a field named scope would otherwise need to state the scope-required-unless-cross-spine rule, which does not apply here (D-14 has no cross_spine field at all — empty scope IS the all-scopes read)."
  - "D-27 fixture strengthening (Task 1): the plan's literal mutation instruction ('call the store with an empty scope regardless of a.Scope') does not reliably go red for a single-scope-per-owner fixture, because empty-scope and single-scope reads return identical results when the caller has records in only one scope. Added a second, same-owner scope (otherScope, tag tagD) to TestListTagsRoundTripBothLanes so scope-narrowing is genuinely load-bearing; confirmed red with the literal mutation applied to that strengthened fixture, then reverted. See Deviations."

patterns-established:
  - "listTagsArgs/tagCountView/tagCountViews (tags.go) are this phase's fourth 'shared typed-core arg struct + flat MCP view' pair, following storeArgs, relatedArgs, and listRulesArgs — the same D-04 convention every curation RPC in this phase uses."

requirements-completed: [RPC-04, RPC-05, RPC-06]

coverage:
  - id: D1
    description: "ListTags lands on Connect: additive proto (TagCount, ListTagsRequest, ListTagsResponse, one RPC, no prefix field per D-14), and the Connect handler delegates to the shared deps.listTags core, which itself delegates straight to the pre-existing Store.ListTags (2026-09-25.01 Phase 1)"
    requirement: RPC-04
    verification:
      - kind: integration
        ref: "internal/server/tags_test.go#TestListTagsRoundTripBothLanes"
        status: pass
    human_judgment: false
  - id: D2
    description: "list_tags MCP tool registered over the same core, framed for tag reuse (D-03: check existing tags before store_memory); surfaces row and tool inventories (toolclass_test.go, registertools_test.go, boot_test.go) bumped 18->19 by construction; docs-site tool-blast-radius table regenerated"
    requirement: RPC-04
    verification:
      - kind: unit
        ref: "internal/surfaces/toolclass_test.go#TestOperationsCoverEveryTool"
        status: pass
      - kind: integration
        ref: "internal/server/registertools_test.go#TestRegisterToolsEnumerable"
        status: pass
      - kind: integration
        ref: "internal/server/tags_test.go#TestListTagsRoundTripBothLanes"
        status: pass
    human_judgment: false
  - id: D3
    description: "Another actor's private record's tag never appears in the caller's ListTags/list_tags results, scoped or across every scope, even though it shares a tag name with the same actor's SHARED record (positive control); a caller's own second scope never leaks into a scoped call for the first scope; the tag stays visible to its own owner (D-21)"
    requirement: RPC-04
    verification:
      - kind: integration
        ref: "internal/server/tags_test.go#TestListTagsNeverShowsPrivate"
        status: pass
    human_judgment: false
  - id: D4
    description: "The MCP flat JSON and the Connect proto carry identical tags/counts/order/more for the same scripted scope+limit, plus identical rejection envelopes for limit=1001, with limit=1000 succeeding on both lanes (D-20)"
    requirement: RPC-06
    verification:
      - kind: integration
        ref: "internal/server/connectapi_parity_test.go#TestReadParity/ListTags"
        status: pass
    human_judgment: false
  - id: D5
    description: "ListTags is a read Procedure: absent from csrfWriteProcedures and reachable without a CSRF token on both the allowlist gate and the live interceptor chain (RPC-05)"
    requirement: RPC-05
    verification:
      - kind: unit
        ref: "internal/server/connectcsrf_test.go#TestCSRFWriteProcedureAllowlist"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_test.go#TestReadRPCsCSRFExempt/ListTags"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-09-27
status: complete
---

# Phase 3 Plan 6: Curation RPCs & MCP Tools — ListTags Summary

**ListTags lands on Connect and as a `list_tags` MCP tool over one shared core wrapping 2026-09-25.01 Phase 1's `Store.ListTags`, with no server-side prefix filter, proven for private-tag isolation and flat-vs-proto parity on both lanes.**

## Performance

- **Duration:** 55 min (approx)
- **Started:** 2026-09-27T09:57:00Z (approx, per STATE.md's prior session timestamp)
- **Completed:** 2026-09-27T10:52:00Z (approx)
- **Tasks:** 2 (1 tracer, 1 auto/tdd)
- **Files modified:** 17 modified, 2 created

## Accomplishments

- Additive proto (`proto/engram/v1/engram.proto`): `TagCount { string tag; uint64 count; }`, `ListTagsRequest { string scope; uint64 limit; }` (scope carries a TRAILING comment only — no server-side prefix field, D-14), `ListTagsResponse { repeated TagCount tags; bool more; }`, and the `ListTags` RPC — `buf lint`/`buf breaking` clean, `gen/go`, `gen/ts`, and `ui/src/lib/gen` regenerated (confirmed idempotent: re-running `task proto:gen && task surfaces:gen` a second time produced byte-identical output).
- New `internal/server/tags.go`: `listTagsArgs`/`tagCountView` (the phase's fourth shared typed-core arg struct + flat MCP view pair), `deps.listTags` (validates `limit` via `rejectOverMaximumCount`, then delegates straight to `d.st.ListTags(ctx, c.Subj, a.Scope, a.Limit)` — no post-filter, no prefix filter), and `tagCountViews` for the MCP JSON shape.
- `protoconv.go` gains `tagCountsToProto`; `connectapi.go` gains the `ListTags` handler (thin adapter: `callerFromConnectContext` → `a.d.listTags` → `connectError` on failure → `ListTagsResponse`); `tools.go` registers `list_tags` framed for tag reuse (D-03).
- `TestListTagsRoundTripBothLanes` (real Qdrant): three records tagged `a-<u>`, one tagged `b-<u>`, one ARCHIVED record tagged `c-<u>` (recall-hidden), plus a second same-owner scope carrying `d-<u>` (D-27 fixture strengthening — see Deviations): Connect `ListTags(scope)` returns exactly `[{a-<u>,3},{b-<u>,1}]` in that order, `more=false`; `limit=1` returns `[{a-<u>,3}]`, `more=true`; the `list_tags` MCP tool (`newMCPSession` as the owner) returns the same tags/counts/more for both calls; `c-<u>` and `d-<u>` never appear; two identical Connect calls return identical responses (`proto.Equal`).
- `list_tags` joins the surfaces registry (D-23: `ReadOnly: true, Destructive: false, Idempotent: true, OpenWorld: false`); the three tool-inventory tests (`toolclass_test.go`, `registertools_test.go`, `boot_test.go`) bumped 18→19; `docs-site/reference/tools.md`'s generated tool-blast-radius table regenerated.
- `TestListTagsNeverShowsPrivate` proves D-21 on both lanes, scoped and all-scopes: owner B's PRIVATE record's tag never appears in owner A's results even though it shares a tag name with B's SHARED record (positive control, which DOES appear with the correct count); A's own second scope's tag never leaks into a scoped call for the first scope; as B, `ListTags(S)` contains the private tag (visible to its own owner) — first-run pass on both lanes (production code from Task 1 already threads the caller's Subject correctly, matching 03-05's identical precedent).
- `TestReadParity/ListTags` (scripted spy, two tags sharing the same count) proves the MCP flat JSON and the Connect proto carry identical tags/counts/order/more for the same scope+limit, plus an identical rejection envelope for `limit=1001` and success for `limit=1000` on both lanes (D-20).
- `ListTags` joins the read-Procedure CSRF-exemption allowlist and `TestReadRPCsCSRFExempt`'s case table (RPC-05).

## Task Commits

Each task was committed atomically:

1. **Task 1: ListTags end to end on both lanes — proto, shared core, Connect RPC, list_tags MCP tool, blast-radius row (tracer)** — `f0e543a0` (feat)
2. **Task 2: private-tag isolation on both lanes, ListTags parity rows, CSRF exemption (auto/tdd)** — `3084b23f` (test)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/server/tags.go` — `listTagsArgs`, `tagCountView`, `deps.listTags`, `tagCountViews`
- `internal/server/tags_test.go` — `TestListTagsRoundTripBothLanes`, `TestListTagsNeverShowsPrivate`, plus shared helpers (`protoTagCountsToViews`, `decodeListTagsViews`, `findTagCount`, `assertTagCount{Absent,Contains,Exact}`, `assertListTagsMatchesProto`)
- `proto/engram/v1/engram.proto` — `TagCount`, `ListTagsRequest`, `ListTagsResponse`, one RPC (additive; regenerated `gen/go`, `gen/ts`, `ui/src/lib/gen`)
- `internal/server/store_iface.go` / `fakestore_test.go` — `memStore.ListTags`; `spyStore`'s scripted `tags`/`tagsMore` fields and fake
- `internal/server/connectapi.go` / `protoconv.go` — the `ListTags` handler and `tagCountsToProto`
- `internal/server/tools.go` — the `list_tags` tool registration; `textResult`'s read-tools doc comment updated
- `internal/surfaces/toolclass.go` / `toolclass_test.go`, `internal/server/registertools_test.go`, `internal/e2e/boot_test.go` — one new `Operation` row, inventories bumped 18→19
- `internal/server/connectapi_parity_test.go` — `TestReadParity`'s `ListTags` subtest
- `internal/server/connectcsrf_test.go` — `ListTags` added to the read-Procedure allowlist and `TestReadRPCsCSRFExempt`'s case table
- `docs-site/src/content/docs/reference/tools.md` — regenerated tool-blast-radius table (via `task surfaces:gen`)

## Decisions Made

See `key-decisions` in the frontmatter above (the trailing-comment-only `scope` field per the plan's conformance-trap note, and the D-27 fixture strengthening needed to make the mutation observably red).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Strengthened TestListTagsRoundTripBothLanes' fixture so the plan's own D-27 mutation instruction is actually observable**
- **Found during:** Task 1's D-27 evidence step
- **Issue:** The plan's literal instruction ("temporarily call the store with an empty scope regardless of a.Scope and confirm the scoped counts assertion goes red") does not reliably go red against the fixture as originally written: this test's owner only ever creates records in ONE scope, so an empty-scope ("every scope the caller can read") query returns exactly the same records as the scoped query — mutating `a.Scope` away to `""` is invisible with a single-scope fixture. Empirically confirmed: applying the exact mutation and re-running the test still passed.
- **Fix:** Added a second, same-owner scope (`otherScope`) carrying its own tag (`tagD`) to the fixture, plus an explicit assertion that `tagD` never appears in the scoped `ListTags(scope)` result (in addition to the pre-existing `len(tags) != 2` check, which already catches a leak). Re-applied the exact mutation (`d.st.ListTags(ctx, c.Subj, "", a.Limit)`) against the strengthened fixture: confirmed RED (`tags = [...tagD...], want exactly 2`). Reverted; test passes green.
- **Files modified:** `internal/server/tags_test.go`
- **Verification:** Mutation applied → RED (documented below); mutation reverted → `go test -run TestListTagsRoundTripBothLanes` green; full `internal/server` package green.
- **Committed in:** `f0e543a0` (part of Task 1's commit — the strengthened fixture is the test as committed, not a separate follow-up)

---

**Total deviations:** 1 auto-fixed (1 bug — a plan-instruction mutation that would not have gone red against the originally-written fixture).
**Impact on plan:** No scope creep — the fix strengthens exactly the one fixture the plan's own D-27 step needed, so the mutation the plan specifies actually proves what it claims to prove.

## Non-Vacuity Evidence (D-27)

Each new/behavior-proving test was confirmed to go red against a temporary, uncommitted mutation of the guarded code, then the mutation was reverted (no harness, patch file, or meta-test committed):

1. **Task 1 — scope narrowing.** Temporarily changed `deps.listTags` (`internal/server/tags.go`) to call `d.st.ListTags(ctx, c.Subj, "", a.Limit)` — ignoring `a.Scope` entirely, always passing empty (every scope the caller can read). Against the ORIGINAL single-scope fixture this did NOT go red (see Deviations above — the fixture only proved a live regression once strengthened with a second, same-owner scope). Against the STRENGTHENED fixture (with the `otherScope`/`tagD` addition), `TestListTagsRoundTripBothLanes` went red: `ListTags tags = [...tagD...], want exactly 2`. Reverted; the test passes green.
2. **Task 2 — the authz clause in `recallVisibleFilter` (Pitfall 4/DEC-cgb).** Temporarily replaced `f := s.ownerScopeFilter(ctx, scope, subj)` with `f := &qdrant.Filter{}` in `internal/store/listtags.go` (2026-09-25.01 Phase 1 code, not modified by this plan — the mutation was applied, observed, and reverted with zero net diff), dropping the authz clause while keeping the three recall-gate conditions unconditional. `TestListTagsNeverShowsPrivate` went red immediately: `Connect A/S: tags contains "p-<u>", want absent` — owner B's private tag leaked into owner A's scoped AND all-scopes results on BOTH lanes (Connect and MCP), because the caller's own Subject was no longer part of the filter at all. Reverted (`git diff --stat -- internal/store/listtags.go` confirmed empty); the test passes green.

## TDD Gate Compliance (Task 2)

Task 2 (`type="auto" tdd="true"`) followed this phase's established single-commit convention (03-01 through 03-05-SUMMARY.md): a single `test(server): ...` commit containing the new/extended test files, with red evidence via temporary uncommitted mutation (D-27, above) rather than a committed RED-phase commit.

- **RED:** `TestListTagsNeverShowsPrivate` was written first and passed on its FIRST run against Task 1's already-implemented production code — expected, not vacuous: Task 1 (one task earlier, same plan) already implemented Subject-threading correctly (`d.st.ListTags(ctx, c.Subj, a.Scope, a.Limit)`, matching RelatedMemories' identical 03-05 precedent). `TestReadParity/ListTags` and the CSRF-exemption case likewise passed on first run over Task 1's code. The D-27 mutation above (item 2) proves `TestListTagsNeverShowsPrivate` is load-bearing rather than passing vacuously.
- **GREEN:** All tests pass with production code unchanged from its Task-1 state.
- **Evidence of load-bearing tests:** the two D-27 mutations (Task 1's scope-narrowing, Task 2's authz-clause removal) prove the two hardest-to-get-right invariants — scope filtering actually reaches the store, and the caller's own Subject is genuinely the enforcement point — are both actually enforced by these tests, not merely exercised.

## Issues Encountered

- **D-27 mutation instruction did not go red as literally written.** See Deviations above — resolved by strengthening the fixture, not by weakening the mutation.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plan 03-07 can proceed against the confirmed option-a wire contract; all seven curation RPCs named in the phase objective now exist (`ArchiveMemory`/`RestoreMemory` 03-01, `SupersedeMemory` 03-02, `ListScheduled` 03-03, `ListRules` 03-04, `RelatedMemories` 03-05, `ListTags` 03-06).
- `RPC-04`/`RPC-05`/`RPC-06` are shared with sibling plans (03-07 in particular); `requirements-completed` above lists them per this plan's own frontmatter, and the shared-ID gate (`requirements.ready-ids` reported 0/3 ready at this plan's close) governs when they actually check off in `REQUIREMENTS.md`.
- No blockers.

## Self-Check: PASSED
