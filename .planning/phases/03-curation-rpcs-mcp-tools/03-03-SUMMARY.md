---
phase: 03-curation-rpcs-mcp-tools
plan: 03
subsystem: api
tags: [connectrpc, mcp-tools, csrf, proto, buf, cursor-paging, cross-spine]

requires:
  - phase: 03-curation-rpcs-mcp-tools
    provides: "plan 03-01's option-a wire contract confirmation (ListScheduled: Connect page_token/next_page_token, MCP cursor/next_cursor, no full knob) and the shared searchedScopes/recallResultMap coverage helpers list_memory established"
provides:
  - "ListScheduled Connect RPC, a read Procedure (CSRF-exempt), delegating to the same deps.listScheduled core the list_scheduled MCP tool calls (RPC-03, D-11)"
  - "cross_spine and an opaque cursor on both lanes for ListScheduled, with the owner-only deferred-reveal guarantee preserved across every scope (D-11, D-19)"
  - "Store.ListScheduled/collectOrderedPages widened to (items, nextCursor/next, exhausted, err) — an empty scope now spans every scope while staying owner-only"
  - "TestReadParity: the read-lane sibling of TestWriteParity, with ListScheduled as its first row"
affects: [03-04-curation-rpcs-mcp-tools, 03-05-curation-rpcs-mcp-tools, 03-06-curation-rpcs-mcp-tools, 03-07-curation-rpcs-mcp-tools]

actuals:
  tokens: 36041
  tasks: 2
  commits: 2
  plan_head_before: 2ad56f11f9cf050a9faec268addc41314dc4396b

tech-stack:
  added: []
  patterns:
    - "collectOrderedPages now returns (items, next listCursor, exhausted bool, err) instead of (items, err) — the two pre-existing callers (List's offset mode, ListScheduled) either discard the extra values or use them for their own resume mechanics, so the shared ordered-page assembly loop serves both a cursor-oblivious caller and a cursor-resuming one without a second primitive."
    - "ListScheduled's empty-scope-spans-every-scope widening mirrors listFilter's existing scope-optional idiom (List/Search's D-08 precedent) rather than inventing a new one — the owner-only condition, the state clause, and both soft-hide conditions stay unconditional regardless of whether the scope Match is present."

key-files:
  created:
    - internal/server/scheduled_test.go
  modified:
    - internal/store/store.go
    - internal/store/store_test.go
    - internal/store/listscheduled_oversized_test.go
    - internal/store/recallmax_oversized_test.go
    - internal/store/schemaversion_compat_test.go
    - internal/store/schemaversion_recallgate_test.go
    - internal/server/store_iface.go
    - internal/server/fakestore_test.go
    - internal/server/tools.go
    - internal/server/tools_test.go
    - internal/server/schemarequired_test.go
    - internal/server/surfaces_test.go
    - internal/server/schemarequired_test.go
    - internal/server/connectapi.go
    - internal/server/protoconv.go
    - internal/server/connectapi_parity_test.go
    - internal/server/connectcsrf_test.go
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/go/engram/v1/engramv1connect/engram.connect.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - docs-site/src/content/docs/reference/tools.md

key-decisions:
  - "Task 1's store-layer change resolves scope via a bare `if scope != \"\"` guard on the Match condition (mirroring listFilter's existing idiom) rather than a new helper — the owner-only, state, and soft-hide conditions were already unconditional, so widening required touching only the one Match append."
  - "deps.listScheduled resolves scope FIRST via effectiveSearchScope(a.Scope, a.CrossSpine), replacing the old unconditional required-scope check — this is the load-bearing change that makes listScheduledArgs.Scope's schemarequired_test.go row expect HintConditionalRequired (naming both \"scope\" and \"cross_spine\") instead of HintRequired, matching the existing search_memory/list_memory/list_rules precedent exactly."
  - "docs-site/reference/tools.md's list_scheduled section was updated in this plan (scope's Required column, new cross_spine/cursor rows, the widened Returns line) even though it is not in the plan's declared files_modified list — the tool's own documented contract changed materially (a field that read \"required: yes\" is no longer unconditionally required), and leaving it stale would actively mislead a reader. Treated as a Rule 2 auto-fix (missing/inaccurate documentation of a changed public contract), not scope creep: no other section of tools.md was touched."
  - "spyStore.ListScheduled's cursor is a plain decimal offset into the sorted (CreatedAt desc, then ID) match set — not a JSON/base64 listCursor like the real store's — since the spy's job is deterministic paging over an in-memory map, not reproducing Qdrant's boundary-tie mechanics; TestReadParity/ListScheduled only ever compares the MCP-lane and Connect-lane spy cursors to each other, never to the real store's cursor shape."
  - "Corrected a stale per-plan commit ledger (gsd-plan-head-before-03-03): the sentinel already existed on disk from an earlier milestone's own phase-03/plan-03 (a docs commit dated 2026-09-23) and pointed well before this plan's actual start; corrected to this plan's actual starting HEAD (2ad56f11, this milestone's phase-3 plan-02 close) before computing actuals.commits — the same class of issue 03-01-SUMMARY.md and 03-02-SUMMARY.md document for plans 01-03/01-04/03-01/03-02."

patterns-established:
  - "TestReadParity (connectapi_parity_test.go) is the read-lane sibling of TestWriteParity: later plans in this phase add rows to it the same way TestWriteParity accumulates write-RPC rows, rather than each read RPC inventing its own parity test."

requirements-completed: [RPC-03, RPC-05, RPC-06]

coverage:
  - id: D1
    description: "ListScheduled lands on Connect as a read Procedure delegating to the same deps.listScheduled core the list_scheduled MCP tool calls; cross_spine and an opaque cursor land on both lanes, with the store's empty-scope-spans-every-scope widening staying owner-only throughout (RPC-03, D-11)"
    requirement: RPC-03
    verification:
      - kind: integration
        ref: "internal/server/scheduled_test.go#TestListScheduledConnectCrossSpinePages"
        status: pass
    human_judgment: false
  - id: D2
    description: "Deferred reveal holds across every scope and every state (scheduled/expired/all) on both lanes: another actor's SHARED pending or expired record never appears, even under cross_spine (D-19)"
    requirement: RPC-03
    verification:
      - kind: integration
        ref: "internal/server/scheduled_test.go#TestListScheduledCrossSpineDeferredReveal"
        status: pass
    human_judgment: false
  - id: D3
    description: "Cursor paging on the MCP lane covers a cross-spine fixture exactly once each with no duplicates and no gaps, and state=expired with cross_spine returns only expired records (D-19)"
    requirement: RPC-03
    verification:
      - kind: integration
        ref: "internal/server/scheduled_test.go#TestListScheduledCursorPagesBothLanes"
        status: pass
    human_judgment: false
  - id: D4
    description: "Coverage three-state table for ListScheduled: not-cross-spine carries none of the three coverage keys, cross-spine-known reports the sorted readable scopes (including one holding only another owner's SHARED non-scheduled record), cross-spine-failed reports scopes_unknown with searched_scopes absent and memories intact (D-19)"
    requirement: RPC-03
    verification:
      - kind: integration
        ref: "internal/server/scheduled_test.go#TestListScheduledCoverageThreeStates"
        status: pass
    human_judgment: false
  - id: D5
    description: "MCP↔Connect read-lane parity for ListScheduled: identical ids/next-token/coverage across a paged cross_spine query on both lanes, and identical rejection envelopes for an invalid state and a missing scope without cross_spine (D-20/D-22)"
    requirement: RPC-06
    verification:
      - kind: integration
        ref: "internal/server/connectapi_parity_test.go#TestReadParity/ListScheduled"
        status: pass
    human_judgment: false
  - id: D6
    description: "ListScheduled is a read Procedure: absent from csrfWriteProcedures and reachable without a CSRF token on both the allowlist gate and the live interceptor chain (RPC-05)"
    requirement: RPC-05
    verification:
      - kind: integration
        ref: "internal/server/connectcsrf_test.go#TestReadRPCsCSRFExempt/ListScheduled"
        status: pass
      - kind: unit
        ref: "internal/server/connectcsrf_test.go#TestCSRFWriteProcedureAllowlist"
        status: pass
    human_judgment: false

duration: 55min
completed: 2026-09-27
status: complete
---

# Phase 3 Plan 3: Curation RPCs & MCP Tools — ListScheduled Summary

**ListScheduled lands on Connect as a CSRF-exempt read RPC over the same core the list_scheduled MCP tool calls, with cross_spine and an opaque cursor widened onto both lanes while the owner-only deferred-reveal guarantee holds across every scope.**

## Performance

- **Duration:** 55 min (estimated)
- **Started:** 2026-09-27T07:26:00Z (approx)
- **Completed:** 2026-09-27T08:21:21Z
- **Tasks:** 2 (1 tracer, 1 auto/tdd)
- **Files modified:** 22 modified, 1 created

## Accomplishments

- `Store.ListScheduled` and `Store.collectOrderedPages` widened: `collectOrderedPages` now returns `(items, next, exhausted, err)` instead of `(items, err)`, and `ListScheduled` returns `(items, nextCursor, err)` — an empty scope spans every scope the caller may read while the owner-only condition, the state clause, and both soft-hide conditions (`superseded_by`, `archived_at`) stay unconditional, preserving deferred reveal (D-11).
- `listScheduledArgs` gains `cross_spine`/`cursor`; `deps.listScheduled` resolves scope via `effectiveSearchScope(a.Scope, a.CrossSpine)` FIRST — the same conditional-rule envelope `search_memory`/`list_memory` already use — and returns a new transport-neutral `coreScheduledResult{Memories, NextCursor}`.
- New `ListScheduled` Connect RPC (`proto/engram/v1/engram.proto` + regenerated `gen/go`, `gen/ts`, `ui/src/lib/gen`): additive `ListScheduledRequest`/`ListScheduledResponse` messages mirroring `ListMemoriesRequest`'s `cross_spine`/coverage-field shapes; the handler delegates to `deps.listScheduled` and shares the `searchedScopes` coverage helper every cross-spine surface uses.
- The `list_scheduled` MCP tool's closure now surfaces `next_cursor` and the coverage keys (`searched_scopes`/`scopes_truncated`/`scopes_unknown`) via the same `recallResultMap` helper `list_memory` uses; its Description composes `scopeRule.Sentence` plus the new cross_spine/cursor sentences.
- Deferred reveal, cursor paging, and honest coverage are proven on both lanes against a real Qdrant (`TestListScheduledConnectCrossSpinePages`, `TestListScheduledCrossSpineDeferredReveal`, `TestListScheduledCursorPagesBothLanes`) and via spies (`TestListScheduledCoverageThreeStates`), including a coverage-known fixture whose second scope holds only another owner's SHARED, non-scheduled record — readable via `ListScopes`' owner-or-shared predicate even though `ListScheduled` itself never returns anything from it.
- New `TestReadParity` (`connectapi_parity_test.go`) — the read-lane sibling of `TestWriteParity` — gains a `ListScheduled` row: identical ids/next-token/coverage across a two-page cross-spine query on both lanes, and identical rejection envelopes (`assertEnvelopeParity`) for an invalid state and a missing scope without `cross_spine`.
- `ListScheduled` joins the read-Procedure CSRF-exemption allowlist (`TestCSRFWriteProcedureAllowlist`'s `readProcedures`) and `TestReadRPCsCSRFExempt`'s live-interceptor case table (RPC-05).
- `schemarequired_test.go`'s `listScheduledArgs.Scope` row updated to expect `HintConditionalRequired` (naming both `scope` and `cross_spine`); `surfaces_test.go`'s `jsonschemaArgStructs` gained `listScheduledArgs` so the D-05 jsonschema-tag conformance gate covers it.
- `docs-site/reference/tools.md`'s `## list_scheduled` section updated for the widened contract (Rule 2 auto-fix — see Deviations).

## Task Commits

Each task was committed atomically:

1. **Task 1: ListScheduled end to end with cross_spine and cursor — store paging, shared core, Connect RPC, MCP tool (tracer)** — `6b9a745a` (feat)
2. **Task 2: D-19 proofs on both lanes — deferred reveal, MCP paging, coverage three states — plus read parity and the CSRF exemption (auto/tdd)** — `2f5b80ee` (test)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/store/store.go` — `Store.ListScheduled` (now `(items, nextCursor, err)`, empty scope spans every scope, resumes from `opts.Cursor`); `Store.collectOrderedPages` (now `(items, next, exhausted, err)`); `Store.List`'s offset mode updated to the new 4-value signature (discards the extra two)
- `internal/store/store_test.go`, `listscheduled_oversized_test.go`, `recallmax_oversized_test.go`, `schemaversion_compat_test.go`, `schemaversion_recallgate_test.go` — every `ListScheduled` call site updated to the three-value return shape; the `scrollOrderedPage` classification row's justification text extended to mention cursor resume
- `internal/server/store_iface.go` — `memStore.ListScheduled` retyped to `(items, nextCursor, err)`
- `internal/server/fakestore_test.go` — `spyStore.ListScheduled` (empty-scope-spans-all, sorted by CreatedAt desc/ID, decimal-offset cursor paging); `spyStore.ListScopes` sorted by scope
- `internal/server/tools.go` — `listScheduledArgs.CrossSpine`/`.Cursor`; `coreScheduledResult`; rewritten `deps.listScheduled`; the `list_scheduled` MCP closure now returns coverage + `next_cursor`
- `internal/server/tools_test.go` — `TestListScheduledTool` updated to read `.Memories`
- `internal/server/schemarequired_test.go` — the `listScheduledArgs.Scope` row now expects `HintConditionalRequired`
- `internal/server/surfaces_test.go` — `jsonschemaArgStructs` gained `listScheduledArgs`
- `proto/engram/v1/engram.proto` — `ListScheduledRequest`/`ListScheduledResponse`, one RPC (additive; regenerated `gen/go`, `gen/ts`, `ui/src/lib/gen`)
- `internal/server/connectapi.go` — the `ListScheduled` handler
- `internal/server/protoconv.go` — `listScheduledRequestToArgs`
- `internal/server/scheduled_test.go` — new file: `seedScheduledFixture`, `mcpMemoryIDs`, `TestListScheduledConnectCrossSpinePages`, `TestListScheduledCrossSpineDeferredReveal`, `TestListScheduledCursorPagesBothLanes`, `TestListScheduledCoverageThreeStates`
- `internal/server/connectapi_parity_test.go` — `mcpMemorySchedIDs`, `connectMemoryIDs`, `TestReadParity` (new) with a `ListScheduled` row
- `internal/server/connectcsrf_test.go` — `EngramServiceListScheduledProcedure` added to `TestCSRFWriteProcedureAllowlist`'s `readProcedures` and `TestReadRPCsCSRFExempt`'s case table
- `docs-site/src/content/docs/reference/tools.md` — `## list_scheduled` section updated (scope's Required column, `cross_spine`/`cursor` rows, widened Returns line)

## Decisions Made

See `key-decisions` in the frontmatter above (the scope-widening idiom reuse, the `effectiveSearchScope`-first resolution order, the docs-site auto-fix, the spy cursor's decimal-offset shape, and the plan-ledger correction).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Updated docs-site/reference/tools.md's list_scheduled section for the widened contract**
- **Found during:** Task 1 (regenerating `task surfaces:gen` and reviewing the tool's documented contract)
- **Issue:** `scope`'s documented Required column said "yes" unconditionally; the widened contract makes it required only unless `cross_spine`. No `cross_spine`/`cursor` rows existed, and the Returns line didn't mention `next_cursor` or the coverage keys.
- **Fix:** Updated the Required column, added `cross_spine`/`cursor` argument rows, and widened the Returns line to mention `next_cursor` and coverage — matching the pattern already established for `list_memory`'s own section.
- **Files modified:** `docs-site/src/content/docs/reference/tools.md`
- **Verification:** `task lint:markdown` clean; `task surfaces:gen` produces no further diff on this file (hand-authored prose, not machine-regenerated).
- **Committed in:** `6b9a745a` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 missing-critical documentation fix).
**Impact on plan:** Keeps the shipped documentation accurate for a materially changed public tool contract. No scope creep — no other section of `tools.md` was touched.

## Non-Vacuity Evidence (D-27)

Each new/behavior-proving test was confirmed to go red against a temporary, uncommitted mutation of the guarded code, then the mutation was reverted (no harness, patch file, or meta-test committed):

1. **Task 1 — deferred reveal (owner-only condition).** Temporarily replaced `s.ownerOnlyCondition(ctx, subj)` with `s.ownerOrSharedCondition(ctx, subj)` in `Store.ListScheduled`'s filter. `TestListScheduledConnectCrossSpinePages` went red: page sizes became `[2 2 2 0]` (want `[2 2 1]`) and 6 ids were returned across all pages (want 5) — owner B's shared pending record leaked into owner A's cross-spine result. Reverted; the test passes green.
2. **Task 1 — the encoded cursor.** Temporarily made `ListScheduled` return `("", nil)` instead of `encodeCursor(next)` on a non-exhausted page. `TestListScheduledConnectCrossSpinePages` went red: page sizes became `[2]` (want `[2 2 1]`) with only 2 of the expected 5 ids returned — the caller had no way to reach the remaining pages. Reverted; the test passes green.
3. **Task 2 — deferred reveal, again (both lanes this time).** Temporarily replaced `s.ownerOnlyCondition(ctx, subj)` with `s.ownerOrSharedCondition(ctx, subj)` in `Store.ListScheduled`'s filter (same mutation as item 1, exercised against the new deferred-reveal test's broader fixture). `TestListScheduledCrossSpineDeferredReveal` went red on every Connect and MCP subtest (`scheduled`/`expired`/`all`): owner B's shared pending and expired records both appeared in owner A's cross-spine results. Reverted; the test passes green on both lanes.
4. **Task 2 — the MCP coverage wiring.** Temporarily hardcoded `false` in place of `a.CrossSpine` in the `list_scheduled` MCP closure's `recallResultMap` call. `TestListScheduledCoverageThreeStates` went red on the MCP side of both cross-spine cases: `searched_scopes`/`scopes_truncated` were absent when coverage was known (want present), and `scopes_unknown` was absent when the coverage query failed (want true) — the Connect side (unaffected by this mutation) stayed green throughout, confirming the mutation isolated the MCP closure's own wiring. Reverted; the test passes green on both lanes.

## TDD Gate Compliance (Task 2)

Task 2 (`type="auto" tdd="true"`) followed the plan's explicit single-commit instruction (one `test(...)` commit covering all four new/extended tests and the small `TestCSRFWriteProcedureAllowlist`/`TestReadRPCsCSRFExempt` extensions) rather than a separate RED-phase/GREEN-phase commit pair — the same established convention 03-01-SUMMARY.md and 03-02-SUMMARY.md document for this phase: red evidence via temporary uncommitted mutation (D-27, above) rather than a committed RED-phase commit.

- **RED (mixed, per the plan's own "record any first-run pass" instruction):** `TestListScheduledCrossSpineDeferredReveal` and `TestListScheduledCoverageThreeStates` passed on their first run — expected, not vacuous: Task 1 already implemented the full deferred-reveal and coverage wiring one task earlier in this same plan, and the two D-27 mutations above (items 3–4) are what prove these two tests are load-bearing rather than passing by construction. `TestListScheduledCursorPagesBothLanes` and `TestReadParity/ListScheduled` each failed on their FIRST run — not from a missing implementation, but from test-authoring bugs: the cursor-paging test's cross-spine loop omitted `state: "all"` (so it only ever paged the 5 pending records, undercounting against the 7-record fixture that also included 2 expired records), and the read-parity test seeded its `NotBefore` fixture relative to `fixedParityNow` (a fixed 2026-03-01 date) rather than real wall-clock time, which `spyStore.ListScheduled`'s pending/expired check compares against — both are FIXTURE bugs, fixed in the test file only, never in production code.
- **GREEN:** All four new tests pass, plus the CSRF allowlist/exempt extensions, with production code unchanged from its Task-1 state. `golangci-lint run ./internal/server/... ./internal/store/...` and the full `internal/server` package (`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ -count=1`) are clean.
- **Evidence of load-bearing tests:** the four D-27 mutations above prove the hardest-to-get-right invariants (owner-only deferred reveal on both the Task-1 and Task-2 fixtures, and the MCP closure's coverage-flag wiring) are actually enforced by these tests, not merely passing vacuously.

## Issues Encountered

- **Test-fixture bugs, not production bugs (see TDD Gate Compliance above).** `TestListScheduledCursorPagesBothLanes` initially undercounted because its cross-spine paging loop didn't request `state: "all"` (default state is `scheduled`, so the 2 expired fixture records were silently excluded); `TestReadParity/ListScheduled` initially reported an empty second page because its seeded `NotBefore` was computed from `fixedParityNow` (2026-03-01), a fixed date already in the past relative to real wall-clock time, against which `spyStore.ListScheduled` (unlike the real store) always compares — its `pending` derivation was `false`, so the fixture matched neither `scheduled` nor `expired` under `state=all`. Both fixed in the test files only.
- **Stale per-plan commit ledger.** `gsd-plan-head-before-03-03` already existed on disk pointing to a docs commit dated 2026-09-23 — a leftover sentinel from an earlier milestone's own phase-03/plan-03 — well before this plan's actual start. Corrected the sentinel to `2ad56f11` (this milestone's actual HEAD when this plan began, matching the phase's own prior-plan close) before computing `actuals.commits`. Same class of issue 03-01-SUMMARY.md and 03-02-SUMMARY.md record for plans 01-03, 01-04, 03-01, and 03-02.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plans 03-04..03-07 can proceed against the confirmed option-a wire contract with no further decision gate; `ListScheduled`'s shape (item 4 of 03-01's bundle) is now shipped alongside `ArchiveMemory`/`RestoreMemory` (03-01), `SupersedeMemory` (03-02).
- `TestReadParity` is now established as the read-lane sibling of `TestWriteParity` — later plans in this phase (e.g. `ListRules`, `RelatedMemories`, `ListTags`) can add their own rows to it rather than inventing a new parity test each time.
- No blockers.

## Self-Check: PASSED
