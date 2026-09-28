---
phase: 03-curation-rpcs-mcp-tools
plan: 04
subsystem: api
tags: [connectrpc, mcp-tools, proto, buf, cross-scope-read, coverage]

requires:
  - phase: 03-curation-rpcs-mcp-tools
    provides: "plan 03-01's option-a wire contract confirmation (ListRules empty scopes: one cross-scope read of every readable rule, oldest-first, up to 1000 total; coverage reuses searched_scopes/scopes_truncated/scopes_unknown filtered to rule:* scopes) and the shared searchedScopes/recallResultMap coverage helpers list_memory/list_scheduled established"
provides:
  - "ListRules Connect RPC, a read Procedure (CSRF-exempt), delegating to the same deps.listRuleRecords core the list_rules MCP tool calls (RPC-03, D-10)"
  - "An empty scopes list is now the all-scopes read on both lanes: one cross-scope Store.List, up to 1000 rules IN TOTAL, oldest-first, with rule-only coverage (D-10, T-03-18)"
  - "listRuleRecords/ruleAdvisory/ruleScopeCoverage: the shared raw core plans 03-05..03-07 (or future rule-surfaces work) can reuse without re-deriving the all-scopes idiom"
affects: [03-05-curation-rpcs-mcp-tools, 03-06-curation-rpcs-mcp-tools, 03-07-curation-rpcs-mcp-tools]

actuals:
  tokens: 29904
  tasks: 2
  commits: 2
  plan_head_before: 7e4f23073f65c4759e096ae7e32383912f86f7c1

tech-stack:
  added: []
  patterns:
    - "listRuleRecords: the shared raw core behind both listRules (MCP shaping) and the Connect ListRules RPC — one Store.List(ctx, \"\", ...) cross-scope call for an empty scopes list, the pre-existing per-scope loop otherwise — mirroring the ListMemories/ListScheduled precedent of a single core two thin transport adapters wrap."
    - "ruleScopeCoverage: searchedScopes filtered to validRuleScope entries, in a freshly allocated non-nil slice — the rule-surface-specific instance of the phase's now-repeated 'coverage is the shared enumeration, narrowed to the surface's own scope prefix' idiom."

key-files:
  created:
    - internal/server/listrules_test.go
  modified:
    - proto/engram/v1/engram.proto
    - gen/go/engram/v1/engram.pb.go
    - gen/go/engram/v1/engramv1connect/engram.connect.go
    - gen/ts/engram/v1/engram_pb.ts
    - ui/src/lib/gen/engram/v1/engram_pb.ts
    - internal/server/rules.go
    - internal/server/tools.go
    - internal/server/connectapi.go
    - internal/server/protoconv.go
    - internal/server/schemarequired_test.go
    - internal/server/connectapi_parity_test.go
    - internal/server/connectcsrf_test.go
    - docs-site/src/content/docs/reference/tools.md

key-decisions:
  - "ruleAdvisory renders the curation-smell text with scopes SORTED (per the plan's own action text), not in caller-supplied Scopes order — byte-identical to the pre-D-10 text only for the single-over-threshold-scope shape every existing test exercises (multi-scope-over-threshold ordering was never pinned and is not asserted here)."
  - "The list_rules MCP closure now returns (nil, nil, err) on error instead of always building the {rules:...} result map — a small behavior change specified by the plan's own action text (Rule 1 alignment with every other read tool's error path), not a deviation."
  - "Updated docs-site/reference/tools.md's list_rules section for the widened contract (scopes' Required column, the all-scopes framing, the coverage-key sentence) — same Rule 2 auto-fix pattern 03-03-SUMMARY.md documents for list_scheduled; task surfaces:gen confirms it is hand-authored prose (no further regeneration diff)."
  - "Corrected a stale per-plan commit ledger (gsd-plan-head-before-03-04): the sentinel already existed on disk pointing to a commit NOT an ancestor of this branch's HEAD at all (git merge-base --is-ancestor: false) — a leftover from an earlier milestone's own phase-3/plan-04. Corrected to this plan's actual starting HEAD (7e4f2307, matching STATE.md's own state_head lineage) before computing actuals.commits. Same class of issue 03-01-SUMMARY.md and 03-03-SUMMARY.md document for plans 01-03/01-04/03-01/03-03."
  - "The plan's own PLAN.md interfaces note claimed schemarequired_test.go's table had '26 rows today' and would keep '25 rows' after removing the listRulesArgs.Scopes row; the live count was 25 before removal and 24 after (exactly at the documented floor of 24, still >= 24 so the gate passes). Corrected in this SUMMARY rather than editing the (already-committed, historical) plan file."

patterns-established:
  - "listRuleRecords/ruleScopeCoverage complete the three-surface set (list_memory, list_scheduled, list_rules) that all share the searchedScopes-then-filter coverage idiom — a fourth cross-scope-capable surface should reach for the same shape rather than inventing a new one."

requirements-completed: [RPC-03, RPC-05, RPC-06]

coverage:
  - id: D1
    description: "ListRules lands on Connect as a CSRF-exempt read RPC delegating to the same core (listRuleRecords) the list_rules MCP tool calls; an empty scopes list is one cross-scope read of every readable rule:* scope, up to 1000 rules in total, oldest-first, with rule-only coverage (RPC-03, D-10)"
    requirement: RPC-03
    verification:
      - kind: integration
        ref: "internal/server/listrules_test.go#TestListRulesConnectAllScopes"
        status: pass
      - kind: integration
        ref: "internal/server/listrules_test.go#TestListRulesAllScopesBothLanes"
        status: pass
    human_judgment: false
  - id: D2
    description: "The tags filter composes with the all-scopes read on both lanes; explicit scopes keep today's per-scope contract exactly (byte-identical order, no coverage keys)"
    requirement: RPC-03
    verification:
      - kind: integration
        ref: "internal/server/listrules_test.go#TestListRulesAllScopesBothLanes/tags_compose_with_all_scopes"
        status: pass
      - kind: integration
        ref: "internal/server/listrules_test.go#TestListRulesAllScopesBothLanes/explicit_scopes_no_coverage"
        status: pass
    human_judgment: false
  - id: D3
    description: "Rule-only coverage three-state contract: explicit scopes carry no coverage keys; an all-scopes call with a working enumeration reports searched_scopes filtered to rule:* scopes ONLY (a readable non-rule scope never appears, T-03-18); a failing enumeration reports scopes_unknown with rules still returned, on both lanes"
    requirement: RPC-03
    verification:
      - kind: integration
        ref: "internal/server/listrules_test.go#TestListRulesCoverageThreeStates"
        status: pass
    human_judgment: false
  - id: D4
    description: "D-19: the old listRulesArgs.Scopes required-field row is removed (not duplicated) from schemarequired_test.go; D-20: TestReadParity gains a ListRules row proving identical ids/order/advisory/coverage on the MCP core and the Connect handler, and an identical rejection envelope for a non-rule scope"
    requirement: RPC-06
    verification:
      - kind: unit
        ref: "internal/server/schemarequired_test.go#TestSchemaRequiredMovedToGoLevel"
        status: pass
      - kind: integration
        ref: "internal/server/connectapi_parity_test.go#TestReadParity/ListRules"
        status: pass
    human_judgment: false
  - id: D5
    description: "ListRules is a read Procedure: absent from csrfWriteProcedures and reachable without a CSRF token on both the allowlist gate and the live interceptor chain (RPC-05)"
    requirement: RPC-05
    verification:
      - kind: unit
        ref: "internal/server/connectcsrf_test.go#TestCSRFWriteProcedureAllowlist"
        status: pass
      - kind: integration
        ref: "internal/server/connectcsrf_test.go#TestReadRPCsCSRFExempt/ListRules"
        status: pass
    human_judgment: false

duration: 1h20m
completed: 2026-09-27
status: complete
---

# Phase 3 Plan 4: Curation RPCs & MCP Tools — ListRules Summary

**ListRules lands on Connect over a new shared raw core (listRuleRecords) that turns an empty scopes list into one cross-scope Store.List read — up to 1000 rules in total, oldest-first, with coverage filtered to rule:* scopes only — on both the MCP tool and the Connect RPC.**

## Performance

- **Duration:** 1h20m
- **Started:** 2026-09-27T04:39:00Z (approx)
- **Completed:** 2026-09-27T05:59:00Z (approx)
- **Tasks:** 2 (1 tracer, 1 auto/tdd)
- **Files modified:** 12 modified, 1 created

## Accomplishments

- New `internal/server/listRuleRecords` (the shared raw core): one `Store.List(ctx, "", ...)` cross-scope read for an empty scopes list (D-10 — an empty scope spans every scope the caller may read, per `internal/store`'s existing `listFilter` idiom), the pre-existing per-scope loop otherwise. `listRules` (MCP shaping) becomes a thin wrapper over it; the pre-D-10 empty-scopes rejection is gone.
- New `ruleAdvisory` (scopes sorted, renders the curation-smell text over-threshold — byte-identical to the pre-D-10 inline computation for the single-scope-over-threshold shape every test exercises) and `ruleScopeCoverage` (the shared `searchedScopes` enumeration filtered to `rule:*` scopes only, in a freshly allocated non-nil slice, T-03-18).
- New `ListRules` Connect RPC (`proto/engram/v1/engram.proto` + regenerated `gen/go`, `gen/ts`, `ui/src/lib/gen`): additive `ListRulesRequest`/`ListRulesResponse` mirroring `ListMemoriesResponse`'s coverage-field shapes; the handler delegates to `listRuleRecords` and `ruleScopeCoverage`, shaping rules through the same `shapeProtoMemories` every other read RPC uses.
- `list_rules`'s MCP closure widened: Description now documents the all-scopes read; the closure returns `(nil, nil, err)` on error (matching every other read tool's error path) and adds coverage keys via `recallResultMap` only when `len(a.Scopes) == 0`.
- `schemarequired_test.go`'s `listRulesArgs.Scopes` required-field row removed (D-19) — an empty/omitted `Scopes` is now the all-scopes read, not a rejection; the per-entry `validRuleScope` guard (a blank or non-rule scope INSIDE a non-empty list) stays pinned by the pre-existing `TestListRulesRejectsEmptyScope`.
- All-scopes read, tag composition, and rule-only coverage proven against a real Qdrant (`TestListRulesConnectAllScopes`, `TestListRulesAllScopesBothLanes`) and via spies (`TestListRulesCoverageThreeStates`, including a fixture whose second scope holds only another owner's SHARED, non-rule record — readable via `ListScopes`' owner-or-shared predicate but never reported among `searched_scopes` for a rules read).
- `TestReadParity` gains a `ListRules` row (explicit scope, empty-scopes/all-scopes, and a non-rule-scope rejection envelope — identical ids/order/advisory/coverage or identical code/message text on both lanes) and `ListRules` joins the read-Procedure CSRF-exemption allowlist plus `TestReadRPCsCSRFExempt`'s case table (RPC-05).
- `docs-site/reference/tools.md`'s `## list_rules` section updated for the widened contract (Rule 2 auto-fix — see Deviations).

## Task Commits

Each task was committed atomically:

1. **Task 1: ListRules end to end — proto, shared raw core with the all-scopes read, rule-scope coverage, Connect RPC, MCP widening (tracer)** — `c68f0503` (feat)
2. **Task 2: D-19 all-scopes and coverage proofs on both lanes, ListRules parity rows, CSRF exemption (auto/tdd)** — `b06b3c76` (test)

**Plan metadata:** pending (this commit)

## Files Created/Modified

- `internal/server/rules.go` — `listRuleRecords` (shared raw core), `ruleAdvisory`, `ruleScopeCoverage`; `listRules` rewritten as the MCP shaping wrapper over `listRuleRecords`
- `internal/server/listrules_test.go` — new file: `TestListRulesConnectAllScopes`, `TestListRulesAllScopesBothLanes`, `TestListRulesCoverageThreeStates`, plus the `rulesContent`/`ruleViewIDs` helpers
- `proto/engram/v1/engram.proto` — `ListRulesRequest`/`ListRulesResponse`, one RPC (additive; regenerated `gen/go`, `gen/ts`, `ui/src/lib/gen`)
- `internal/server/connectapi.go` — the `ListRules` handler
- `internal/server/protoconv.go` — `listRulesRequestToArgs`
- `internal/server/tools.go` — `list_rules`'s widened Description and closure (all-scopes coverage, error-first return)
- `internal/server/schemarequired_test.go` — removed the `listRulesArgs.Scopes` required-field row
- `internal/server/connectapi_parity_test.go` — `TestReadParity` gains a `ListRules` row (`ruleViewIDs` from `listrules_test.go`)
- `internal/server/connectcsrf_test.go` — `EngramServiceListRulesProcedure` added to `TestCSRFWriteProcedureAllowlist`'s `readProcedures` and `TestReadRPCsCSRFExempt`'s case table
- `docs-site/src/content/docs/reference/tools.md` — `## list_rules` section updated (scopes' Required column, all-scopes framing, coverage-key sentence)

## Decisions Made

See `key-decisions` in the frontmatter above (the sorted-scopes advisory rendering, the error-first MCP closure return, the docs-site auto-fix, the plan-ledger correction, and the schemarequired_test.go row-count correction).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Updated docs-site/reference/tools.md's list_rules section for the widened contract**
- **Found during:** Task 1 (regenerating `task surfaces:gen` and reviewing the tool's documented contract)
- **Issue:** `scopes`' documented Required column said "yes" unconditionally; the widened contract makes it optional (omit for the all-scopes read). No sentence described the all-scopes read's coverage keys.
- **Fix:** Updated the Required column, widened the intro sentence to describe the all-scopes read, and added a sentence describing `searched_scopes`/`scopes_truncated`/`scopes_unknown` on that path — matching the pattern 03-03-SUMMARY.md established for `list_scheduled`.
- **Files modified:** `docs-site/src/content/docs/reference/tools.md`
- **Verification:** `task lint:markdown` clean; `task surfaces:gen` produces no further diff on this file (hand-authored prose, not machine-regenerated).
- **Committed in:** `c68f0503` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 missing-critical documentation fix).
**Impact on plan:** Keeps the shipped documentation accurate for a materially changed public tool contract. No scope creep — no other section of `tools.md` was touched.

## Non-Vacuity Evidence (D-27)

Each new/behavior-proving test was confirmed to go red against a temporary, uncommitted mutation of the guarded code, then the mutation was reverted (no harness, patch file, or meta-test committed):

1. **Task 1 — the empty-scopes rejection.** Temporarily restored `if len(a.Scopes) == 0 { return nil, "", argErrf(...) }` at the top of `listRuleRecords`. `TestListRulesConnectAllScopes` went red immediately: `ListRules (empty scopes): invalid_argument: field=scopes hint=required: ...` — the all-scopes read never reached `Store.List` at all. Reverted; the test passes green.
2. **Task 1 / Task 2 — the `validRuleScope` filter in `ruleScopeCoverage`.** Temporarily widened the filter to `if true || validRuleScope(sc)`. Per the plan's own hedge, this did **not** turn `TestListRulesConnectAllScopes` red: that real-Qdrant test's owner B happens to have no readable non-rule scope in its fixture, so the every-entry assertion passed vacuously. `TestListRulesCoverageThreeStates` (Task 2, whose fixture deliberately seeds a non-rule scope readable by the caller) DID go red on both the MCP and Connect subtests of the "all scopes, coverage known" case: `searched_scopes = [listrules-coverage:project:... rule:repo:...], want [rule:repo:...] (the non-rule scope must be filtered out)`. Reverted; both tests pass green — confirming Task 2's spy test, not Task 1's real-Qdrant test, is the actual proof of this invariant, exactly as the plan anticipated.
3. **Task 2 — the tags filter on the all-scopes read.** Temporarily replaced `Tags: a.Tags` with `Tags: nil` in `listRuleRecords`' empty-scopes branch. `TestListRulesAllScopesBothLanes` went red on exactly the `tags_compose_with_all_scopes` subtest: `list_rules {tags} ids = map[<3 ids>:true], want exactly [<the one tagged rule>]` — all three rules returned instead of the one carrying the tag. The `MCP_all_scopes`, `Connect_all_scopes`, and `explicit_scopes_no_coverage` subtests stayed green throughout, confirming the mutation isolated the all-scopes branch's own tag wiring. Reverted; the test passes green.

## TDD Gate Compliance (Task 2)

Task 2 (`type="auto" tdd="true"`) followed the plan's explicit single-commit instruction (one `test(...)` commit covering all new/extended tests) rather than a separate RED-phase/GREEN-phase commit pair — the same established convention 03-01/03-02/03-03-SUMMARY.md document for this phase: red evidence via temporary uncommitted mutation (D-27, above) rather than a committed RED-phase commit.

- **RED:** All three of Task 2's new/extended assertions (`TestListRulesAllScopesBothLanes`, `TestListRulesCoverageThreeStates`, `TestReadParity/ListRules`) passed on their FIRST run against Task 1's already-implemented production code — expected, not vacuous: Task 1 (this same plan, one task earlier) already implemented the full all-scopes read and coverage wiring these tests exercise. The three D-27 mutations above (items 2-3) are what prove these tests are load-bearing rather than passing by construction.
- **GREEN:** All tests pass with production code unchanged from its Task-1 state. `golangci-lint run ./internal/server/...` and the full `internal/server`/`internal/store` packages (`ENGRAM_REQUIRE_QDRANT=1 go test ./internal/server/ ./internal/store/ -count=1`) are clean.
- **Evidence of load-bearing tests:** the three D-27 mutations above prove the hardest-to-get-right invariants (the all-scopes read actually reaching `Store.List`, rule-only coverage filtering, and tag composition on the all-scopes branch) are actually enforced by these tests, not merely passing vacuously.

## Issues Encountered

- **Stale per-plan commit ledger.** `gsd-plan-head-before-03-04` already existed on disk pointing to a commit that `git merge-base --is-ancestor` confirmed is NOT an ancestor of this branch's HEAD at all — a leftover sentinel from an earlier milestone's own phase-3/plan-04. Corrected the sentinel to `7e4f2307` (this plan's actual starting HEAD) before computing `actuals.commits`. Same class of issue 03-01-SUMMARY.md and 03-03-SUMMARY.md record for plans 01-03, 01-04, 03-01, and 03-03.
- **Plan's own row-count note was slightly stale.** `03-04-PLAN.md`'s interfaces note claimed `schemarequired_test.go`'s table had "26 rows today" and would keep "25 rows" after removing the `listRulesArgs.Scopes` row. The live count was 25 before removal and 24 after — exactly at the documented floor of 24 (the gate checks `< 24`, so it still passes). No code or test change was needed; noted here rather than editing the historical plan file.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plans 03-05..03-07 can proceed against the confirmed option-a wire contract with no further decision gate; `ListRules`' shape (item 3 of 03-01's bundle) is now shipped alongside `ArchiveMemory`/`RestoreMemory` (03-01), `SupersedeMemory` (03-02), `ListScheduled` (03-03).
- `RPC-03`/`RPC-05`/`RPC-06` are shared with not-yet-executed sibling plans (03-05, 03-06, 03-07); `requirements-completed` above lists them per this plan's own frontmatter, and the shared-ID gate governs when they actually check off in `REQUIREMENTS.md`.
- No blockers.

## Self-Check: PASSED
