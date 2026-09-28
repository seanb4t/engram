---
phase: 05-related-memories-graph-tag-cloud
plan: 01
subsystem: ui
tags: [svelte, d3-force, connect-rpc, related-memories, graph, tanstack-query]

requires:
  - phase: 03-curation-rpcs-mcp-tools
    provides: RelatedMemories Connect RPC, RelatedEdge discriminated-union evidence, ListTags
  - phase: 04-curation-surfaces
    provides: ChainDialog not-found copy, RecallState envelope pattern, resume.ts ALLOWED_DESTINATIONS convention
provides:
  - "/related/<id> route: anchor card, mono call line, honest loading/not-found/rejected/opaque/zero-candidate/truncated states"
  - "ui/src/lib/related/graph.ts: pure model (buildRelatedModel, laneRows, visibleMembership, graphEdges, nodeAccessibleName, neighbourhoodSummary, callLineParts, lcg, settleLayout) that lanes and the graph both read"
  - "RelatedGraph.svelte: Svelte-owned inline-SVG d3-force graph with the full GRAPH-03 visual encoding (edge line styles, arrowheads, anchor ring, selection halo, dimming, hidden-state dashing, label density)"
affects: [05-02, 05-04, 05-05, 05-08, 05-09]

actuals:
  tokens: 19300
  tasks: 3
  commits: 5

commits: 5
plan_head_before: 8be24a8c63517505b6bff8b7813cf3dbf934d081
plan_head_after: 2fbeb83012ec88e043e3abf6f6ea34d299aba97f

tech-stack:
  added: [d3-force@3.0.0, d3-drag@3.0.0, d3-zoom@3.0.0, d3-selection@3.0.0, "@types/d3-force@3.0.10", "@types/d3-drag@3.0.7", "@types/d3-zoom@3.0.8", "@types/d3-selection@3.0.12"]
  patterns:
    - "Svelte owns every SVG element (<g>/<circle>/<path>/<text> from {#each}); d3-force supplies only settled positions via a synchronous, seeded 300-tick simulation.tick() loop -- no tick-driven render loop, no d3-selection DOM ownership"
    - "One visibleMembership() result feeds lanes, the graph and (later) the aria-live summary -- never independently derived filters per surface"
    - "Deterministic layout via a one-line seeded LCG passed to d3-force's randomSource(), making graph renders and screenshot tests reproducible"

key-files:
  created:
    - ui/src/lib/related/graph.ts
    - ui/src/lib/related/graph.test.ts
    - ui/src/lib/components/RelatedGraph.svelte
    - ui/src/lib/components/RelatedGraph.browser.test.ts
    - ui/src/routes/related/[id]/+page.svelte
    - ui/src/routes/related/related.browser.test.ts
  modified:
    - ui/package.json
    - ui/pnpm-lock.yaml

key-decisions:
  - "Multi-type edge offsets are fixed per edge type (tag +12, vector -12, citation/supersession 0), not per-index -- matches sketch 006's ES table exactly so the same candidate always draws the same three curves"
  - "Chain (superseded_by) arrows are computed inside visibleMembership, never in graphEdges -- they only ever join two DRAWN non-anchor candidates, never the anchor (the star already covers anchor-adjacent supersession)"
  - "callLineParts always reads the RAW model counts from the server response, never the UI-filtered membership -- the mono call line never re-derives a number the server didn't send"
  - "Sticky-rail offset (RESEARCH Open Question 1) resolved via a local ResizeObserver writing --rail-max-h, not an AppShell.svelte edit -- zero blast radius on the shared shell"
  - "The route reaches into RelatedGraph's internal .graph SVG class via a scoped :global() selector for its 100%/440:380 sizing rule, rather than editing RelatedGraph.svelte, to keep Task 3's diff inside its own declared files_modified scope"

patterns-established:
  - "Pure related-neighbourhood model (graph.ts) mirrors curation/chain.ts's split: a testable, DOM-free module the Svelte component only renders"
  - "TDD RED evidence persisted and verified via `gsd-tools check tdd-red-evidence` before each GREEN implementation (Tasks 2 and 3)"

requirements-completed: [GRAPH-01, GRAPH-02, GRAPH-03]

coverage:
  - id: D1
    description: "d3-force/d3-drag/d3-zoom/d3-selection installed at exact 3.0.0 pins (+ matching @types/* dev deps), pre-vetted by RESEARCH.md's Package Legitimacy Audit (all OK, official d3 repos, no postinstall) -- no install checkpoint needed"
    requirement: "GRAPH-01"
    verification:
      - kind: unit
        ref: "ui/package.json pin check (Task 1 acceptance: 4x d3-* + 4x @types/d3-* exact-version greps)"
        status: pass
      - kind: integration
        ref: "pnpm --dir ui install --frozen-lockfile && task fmt:check"
        status: pass
    human_judgment: false
  - id: D2
    description: "graph.ts pure model: buildRelatedModel maps evidence to strength/signedDepth/counts; laneRows orders each lane by its own strength (supersession by signed depth then compareChainNodes); visibleMembership is the one lane-ordered, vector-collapsed, hidden-type-filtered, chain-arrow-added result lanes and the graph both read; graphEdges draws the anchor star with correct offsets and supersession direction; nodeAccessibleName/neighbourhoodSummary/callLineParts render the locked copy formats; settleLayout is deterministic and fixes the anchor at the origin"
    requirement: "GRAPH-02"
    verification:
      - kind: unit
        ref: "ui/src/lib/related/graph.test.ts (24/24 passing: buildRelatedModel x6, laneRows x3, graphEdges x3, visibleMembership x6, nodeAccessibleName x2, neighbourhoodSummary x1, callLineParts x1, settleLayout x2)"
        status: pass
    human_judgment: false
  - id: D3
    description: "RelatedGraph.svelte: inline-SVG graph with GRAPH-03's full visual encoding (edge line style + width + dasharray from EDGE_STYLE, supersession arrowheads with a selected variant, anchor double violet ring, selection halo/.selon/.has-sel, dimmedIds/.fdim, hidden-state dashing, label density switch past 26 drawn nodes), zero d3-selection usage, click/dblclick wired to onselect/onrecenter"
    requirement: "GRAPH-03"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/RelatedGraph.browser.test.ts (11/11 passing: option identity, edge encoding, anchor rings, selection, dimming, hidden-state, label density x2, click/dblclick, both-theme screenshots)"
        status: pass
    human_judgment: false
  - id: D4
    description: "/related/<id> route fetches exactly one RelatedMemories(id, k=64, full=false) through the read client and renders the anchor card, mono call line and rail graph fed from the shared membership -- the populated path"
    requirement: "GRAPH-01"
    verification:
      - kind: unit
        ref: "ui/src/routes/related/related.browser.test.ts (populated-path cases: call fired once with correct args, call line counts, 4 role=option nodes, zero-candidate '0 related', truncated warning styling, ms timing)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Route shell states: loading skeleton (anchor line + 4 lane skeletons + rail skeleton), Code.NotFound copy with nothing rendered below, FailedPrecondition rejected envelope with fixRowsFor buttons, opaque-failure block with Retry/Copy error, sticky rail with a measured --rail-max-h and narrow-width stacking under 900px"
    requirement: "GRAPH-01"
    verification:
      - kind: unit
        ref: "ui/src/routes/related/related.browser.test.ts (Task 3 cases: loading copy + 4 skeletons, not-found, rejected envelope, opaque + Retry refetch, screenshots of loading/not-found/populated)"
        status: pass
    human_judgment: true
    rationale: "The @container frame (max-width: 900px) narrow-stacking layout and the ResizeObserver-measured --rail-max-h sticky offset are asserted structurally (CSS text greps, both-theme screenshots) but not viewport-resized in a live browser in this test suite -- a human/visual pass at a real narrow width is the stronger proof."

duration: 41min
completed: 2026-09-28
status: complete
---

# Phase 5 Plan 1: Related-Memories Graph Rendering Core Summary

**`/related/<id>` renders one record's RelatedMemories neighbourhood end to end: anchor card, mono call line, and a deterministic Svelte-owned d3-force graph in a sticky rail, with an honest shell for loading, not-found, rejected, opaque-failure and zero-candidate states.**

## Performance

- **Duration:** ~41 min
- **Started:** 2026-09-28T02:58:40Z
- **Completed:** 2026-09-28T03:40:09Z
- **Tasks:** 3
- **Files modified:** 8 (6 created, 2 modified)

## Accomplishments

- `graph.ts`: the pure related-neighbourhood model -- evidence-to-strength mapping, per-lane strength ordering, the single vector-collapsed/hidden-type-filtered/chain-arrow-added membership result, the anchor star edge set, accessible names, and a deterministic seeded d3-force settle
- `RelatedGraph.svelte`: a Svelte-owned inline-SVG graph implementing GRAPH-03's full visual encoding (line-style edge typing, arrowheads, anchor ring, selection, dimming, hidden-state, label density) with zero d3-selection DOM ownership
- `/related/[id]` route: fetches exactly one `RelatedMemories` response through the read client and renders it honestly in every state -- loading skeletons, not-found, rejected envelope, opaque failure, zero-candidate, truncated warning, and a sticky/narrow-responsive rail layout
- Four d3 micro-packages installed at exact pins, pre-cleared by RESEARCH.md's package audit

## Task Commits

Each task was committed atomically (Task 2 and Task 3 as TDD RED → GREEN pairs, no REFACTOR commit needed for either):

1. **Task 1: `/related/<id>` fetches one RelatedMemories response and draws the anchor, its candidates and their edges** - `e0ab3cd7` (feat)
2. **Task 2 RED: failing tests for the completed model and visual encoding** - `0a58be9a` (test)
2. **Task 2 GREEN: related graph model and edge encoding (GRAPH-01, GRAPH-03, D-06, D-07)** - `4fb28be9` (feat)
3. **Task 3 RED: failing tests for the route shell states** - `099d030e` (test)
3. **Task 3 GREEN: loading, not-found, error and layout states (D-01)** - `2fbeb830` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `ui/src/lib/related/graph.ts` - Pure model: types, constants (LANE_ORDER, RELATED_K=64, VECTOR_COLLAPSE=8, SETTLE_TICKS=300, LABEL_ALL_MAX=26, EDGE_STYLE, ...), buildRelatedModel, laneRows, visibleMembership, graphEdges, nodeAccessibleName, neighbourhoodSummary, callLineParts, lcg, settleLayout
- `ui/src/lib/related/graph.test.ts` - 24 node-project tests covering every exported function
- `ui/src/lib/components/RelatedGraph.svelte` - The inline-SVG graph component
- `ui/src/lib/components/RelatedGraph.browser.test.ts` - 11 vitest-browser tests including both-theme screenshots
- `ui/src/routes/related/[id]/+page.svelte` - The `/related/<id>` route: query, anchor card, call line, rail, all shell states, sticky-rail ResizeObserver
- `ui/src/routes/related/related.browser.test.ts` - 13 vitest-browser tests (3 Task 1 + 10 Task 3) covering the populated path and every shell state
- `ui/package.json` / `ui/pnpm-lock.yaml` - d3-force/d3-drag/d3-zoom/d3-selection@3.0.0 + matching @types/* dev deps, exact pins

## Decisions Made

See `key-decisions` in frontmatter above.

## Deviations from Plan

None - plan executed exactly as written. Task 1's `visibleMembership` intentionally shipped as an unfiltered pass-through (per the plan's own text: "in this task it returns every candidate unfiltered; Task 2 adds the vector collapse and hidden types") and Task 2 replaced it with the full implementation, matching the plan's task-by-task scoping precisely.

## Issues Encountered

None. Both TDD tasks (2 and 3) produced clean RED-EVIDENCE_OK verdicts via `gsd-tools check tdd-red-evidence` on their first attempt, and GREEN implementations passed on the first run with no fix-attempt cycles needed.

## Known Stubs

- **`ui/src/routes/related/[id]/+page.svelte`**, the `.lanes` div: renders only an HTML comment placeholder (`<!-- plan 05-04 renders edge-type lanes here -->`), no lane cards. This is the plan's own declared scope boundary, not an oversight -- 05-01's objective states "Plans 05-04 (lanes, evidence), 05-05 (keyboard, zoom, drag) and 05-08 (tags rail, trail) build on the contracts fixed here," and 05-04's frontmatter (`depends_on: [05-01, 05-02]`, `files_modified` includes `ui/src/routes/related/[id]/+page.svelte`) confirms it is the plan that fills this placeholder. No route currently links to `/related/<id>` yet either (the entry points -- DetailPane button, row key `r`, ⌘K item -- are D-03 items scoped to a later plan per 05-CONTEXT.md's Executor notes), so this route is reachable only by direct navigation until that wiring lands.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The pure model (`graph.ts`) and the graph component (`RelatedGraph.svelte`) expose the exact contracts plan 05-04 (lanes, evidence section) and 05-05 (keyboard, pan/zoom, drag) depend on: `RelatedModel`, `Candidate`, `Membership`, `GraphNode`, `GraphEdge`, `visibleMembership`, `laneRows`, `nodeAccessibleName`, `neighbourhoodSummary`.
- The route's shell-state scaffolding (loading/error/empty) is in place for 05-04 to extend with lane cards inside the existing `.lanes` div and an evidence section under the rail graph.
- No blockers. `pnpm --dir ui build` succeeds with the new route; all 24 node-project + 24 browser-project tests for this plan's files pass.

## Self-Check: PASSED

- All 6 created files confirmed present on disk (`[ -f ]`).
- All 5 commits (`e0ab3cd7`, `0a58be9a`, `4fb28be9`, `099d030e`, `2fbeb830`) confirmed in `git log --oneline`.
- All 3 tasks' `<verify>` commands re-run and passing: `pnpm --dir ui vitest run --project browser src/routes/related/related.browser.test.ts` (13/13), `pnpm --dir ui vitest run --project node src/lib/related/graph.test.ts` (24/24), `pnpm --dir ui vitest run --project browser src/lib/components/RelatedGraph.browser.test.ts` (11/11), `pnpm --dir ui install --frozen-lockfile && task fmt:check` (clean), `pnpm --dir ui build` (exit 0).
- All acceptance-criteria greps for all 3 tasks re-run and matching their required counts.

---

*Phase: 05-related-memories-graph-tag-cloud*
*Completed: 2026-09-28*
