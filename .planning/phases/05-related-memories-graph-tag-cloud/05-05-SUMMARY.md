---
phase: 05-related-memories-graph-tag-cloud
plan: 05
subsystem: ui
tags: [svelte, d3-zoom, d3-drag, d3-selection, aria, keyboard, related-memories, graph]

requires:
  - phase: 05-related-memories-graph-tag-cloud
    provides: "plan 05-01's RelatedGraph.svelte (visual encoding, settled positions) and related/graph.ts (GraphNode/GraphEdge, neighbourhoodSummary, settleLayout, LABEL_ALL_MAX)"
provides:
  - "RelatedGraph.svelte fully operable without a pointer: one-Tab-stop roving keyboard (arrows in lane order, Space selects, Enter re-centres, Escape tiers), a screen-reader neighbourhood list plus aria-live summary, a keyboard focus ring and floating focus card, and sketch 006 A's pan/zoom/drag controls"
  - "ui/src/lib/related/zoom.ts: dependency-free zoom gate/fit/readout math (wheelZoomFilter, fitTransform, zoomPercent, isOutsideView)"
affects: [05-08]

actuals:
  tokens: 13681
  tasks: 3
  commits: 5

commits: 5
plan_head_before: 289c0bc2e90d22755de10aff1c57534891672a9c
plan_head_after: b30ee9d3e134074a0d77b62b9b9ca06af1407a31

tech-stack:
  added: []
  patterns:
    - "d3-zoom/d3-drag/d3-selection attach ONLY via select(el).call(behaviour) -- they write Svelte $state (a {k,x,y} transform, a SvelteMap of node positions) and never create or bind a DOM element themselves"
    - "A persistent d3-force simulation (recreated only on membership change, kept alive otherwise) backs a SvelteMap of positions so a drag's alphaTarget reheat has something to restart and stream ticks into"
    - "Reactive-loop discipline: an effect that both WRITES a reactive collection and READS its key-set/entries in the same run self-triggers (Svelte's effect_update_depth_exceeded); fixed by making mutations write-only (clear()+set(), never .keys() after .set() in the same effect) and by wrapping intentionally-one-way reads in untrack() where an effect must read state it does not want to depend on"

key-files:
  created:
    - ui/src/lib/related/zoom.ts
    - ui/src/lib/related/zoom.test.ts
  modified:
    - ui/src/lib/components/RelatedGraph.svelte
    - ui/src/lib/components/RelatedGraph.browser.test.ts

key-decisions:
  - "Card/list placement math takes the live zoom transform (zoomT) rather than raw viewBox coordinates, so the floating focus card and its left/right flip stay correct at any zoom/pan state (Task 2's placement code, extended in Task 3 once the transform existed)"
  - "The refit effect's only TRACKED dependencies are membershipKey and svgEl; positionsState is read inside untrack() so a drag tick or the effect's own applyTransform() dispatch can never re-trigger it -- this is what makes 'selection/focus/drag never refit' (D-20) hold, and it doubles as the fix for a genuine effect_update_depth_exceeded loop found during Task 3's GREEN pass"
  - "A stale activeId (the keyboard-focused node dropped by a nodes-prop refetch) is treated as index 0 (the anchor) by activeIndex(), so the next arrow key resumes navigation from the anchor's baseline per the plan's 'clears lostFocus and moves from the anchor' wording, rather than snapping back onto the anchor itself"

patterns-established:
  - "Reduced-motion drag: alphaTarget(0.25) reheat plus fx/fy pinning under normal motion; under reducedMotion the node's x/y are written directly (no reheat) and restored exactly to their pre-drag values on release -- both paths share one d3-drag behaviour, gated only by reducedMotionValue"

requirements-completed: [GRAPH-01, GRAPH-02]

coverage:
  - id: D1
    description: "Task 1 (tracer): the graph is one Tab stop (svg role=listbox tabindex=0); arrows walk nodes in lane order (anchor, supersession, citation, tag, vector) and clamp at the ends; Home/End jump to the first/last; Space selects/deselects; Enter re-centres a candidate and no-ops on the anchor; all keys ignored while Meta/Ctrl/Alt is held; focus never leaves the svg"
    requirement: "GRAPH-02"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/RelatedGraph.browser.test.ts -- 'RelatedGraph — keyboard traversal (GRAPH-02, D-10)' (8/8 passing)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Task 2: a visually hidden neighbourhood list (one li per node, lane order, aria-current on the active item) and a polite aria-live summary (neighbourhoodSummary by default, summary prop override); Escape clears the selection then leaves the graph, both tiers stopping propagation; a dashed violet focus ring and a floating focus card (short_id, edge glyphs, two-line summary, state chips) on keyboard focus or hover, absent for the selected node, labelled past LABEL_ALL_MAX; a refetch that drops the focused node shows a not-found message until the next arrow key"
    requirement: "GRAPH-02"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/RelatedGraph.browser.test.ts -- screen-reader list/live summary, Escape tiers, focus ring/card describe blocks (11/11 passing)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Task 3: corner +/-/fit zoom controls (scale 0.5-4, disabled at the limits) with a live 'Zoom {N}%' readout; a plain wheel never zooms or blocks scroll and flashes a ⌘-hint; ⌘/Ctrl+wheel zooms; dblclick never zooms; the view refits on drawn-membership change only (never on selection/focus/drag) and auto-pans a keyboard-focused node back into view; candidates spring back on drag release (brief reheat unless reducedMotion), the anchor never drags"
    requirement: "GRAPH-01"
    verification:
      - kind: unit
        ref: "ui/src/lib/related/zoom.test.ts (12/12 passing: wheelZoomFilter, fitTransform, zoomPercent, isOutsideView)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/RelatedGraph.browser.test.ts -- zoom controls/wheel gate and spring-back drag describe blocks (8/8 passing)"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-28
status: complete
---

# Phase 5 Plan 5: Related Graph Keyboard, ARIA and Zoom/Drag Controls Summary

**RelatedGraph.svelte gains a full one-Tab-stop roving keyboard model in lane order with a screen-reader list and live summary, Escape tiers, a keyboard focus ring and card, and sketch 006 A's zoom.ts-backed pan/zoom/⌘-wheel-gated controls plus d3-drag spring-back.**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-09-27T23:57:19Z
- **Completed:** 2026-09-28T00:16:47Z
- **Tasks:** 3
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- One Tab stop: `role="listbox" tabindex="0"` on the svg with roving `aria-activedescendant`, arrows walking `nodes` in the prop's own lane order, Space to select/deselect, Enter to re-centre, all ignored under Meta/Ctrl/Alt
- Screen-reader equivalence (D-11): a visually hidden `<ul>` neighbourhood list mirroring lane order with `aria-current`, plus a polite `aria-live` summary (`neighbourhoodSummary(nodes)` default, `summary` prop override)
- Escape tiers (D-04, graph half): selection-then-leave, both stopping propagation so a future route-level Escape never double-fires
- Keyboard focus ring (dashed violet, r+6) and a floating focus card (short_id, edge-type glyphs, two-line summary, state chips) on keyboard focus or hover, absent for the selected node, with a "no longer readable" fallback when a refetch drops the focused node
- `ui/src/lib/related/zoom.ts`: dependency-free `wheelZoomFilter`, `fitTransform`, `zoomPercent`, `isOutsideView` plus the `SCALE_MIN/MAX`, `TRANSLATE_EXTENT`, `ZOOM_STEP`, `WHEEL_HINT`/`WHEEL_HINT_MS` constants
- Corner zoom controls (`+`/`−`/`⤢`, disabled at the scale limits) with a live `Zoom {N}%` readout; a plain wheel scrolls the page and flashes a ⌘-hint, ⌘/Ctrl+wheel zooms, dblclick never zooms
- Refit-on-membership-change only (never on selection/focus/drag), auto-pan of a keyboard-focused node back into view, and d3-drag spring-back with a reduced-motion path that skips the reheat and restores position exactly

## Task Commits

Each task was committed atomically (Tasks 2 and 3 as TDD RED → GREEN pairs; no REFACTOR commit needed for either):

1. **Task 1: One Tab stop, arrows in lane order, Space/Enter (tracer)** - `58309590` (feat)
2. **Task 2 RED: failing tests for SR list, live summary, Escape tiers, focus card** - `fab8aeff` (test)
2. **Task 2 GREEN: screen-reader list, live summary, Escape tiers and focus card** - `8e309ded` (feat)
3. **Task 3 RED: failing tests for zoom controls, wheel gate and spring-back drag** - `9d4473ef` (test)
3. **Task 3 GREEN: zoom controls, ⌘-wheel gate, refit and spring-back drag** - `b30ee9d3` (feat)

**Plan metadata:** committed separately after this SUMMARY.

## Files Created/Modified

- `ui/src/lib/related/zoom.ts` - Pure zoom gate/fit/readout math (`wheelZoomFilter`, `fitTransform`, `zoomPercent`, `isOutsideView`), no DOM, no d3 dependency
- `ui/src/lib/related/zoom.test.ts` - 12 node-project tests covering every exported function
- `ui/src/lib/components/RelatedGraph.svelte` - Roving keyboard model, sr-only list/live region, Escape tiers, focus ring/card, viewport `<g>` driven by a d3-zoom transform, corner controls, wheel-hint, refit/auto-pan effects, d3-drag spring-back
- `ui/src/lib/components/RelatedGraph.browser.test.ts` - 35 vitest-browser tests (8 keyboard, 3 SR list/live summary, 2 Escape tiers, 5 focus card, 6 zoom controls, 2 drag, plus the 9 pre-existing Task-01 tests)

## Decisions Made

- **Reactive-loop fix (deviation, see below):** the settle effect was rewritten write-only (`positionsState.clear()` + `.set()`, no `.keys()` read in the same run) and the refit effect's positions/transform reads were wrapped in `untrack()` — both required to eliminate a genuine `effect_update_depth_exceeded` loop, not merely to satisfy D-20's "selection/focus/drag never refit" requirement (though the same fix delivers that too)
- Card/list placement math takes the live zoom transform, not raw viewBox coordinates, so the focus card stays correctly placed and flip-aware at any zoom/pan state
- A stale `activeId` (focused node dropped by a props change) resumes the next arrow key from the anchor's baseline (index 0) rather than re-landing on the anchor

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed an infinite reactive-effect loop between the settle and refit effects**
- **Found during:** Task 3 (manual test run before committing GREEN — even a trivial render test hung, then failed with Svelte's `effect_update_depth_exceeded`)
- **Issue:** The membership-settle effect wrote `positionsState` (a reactive `SvelteMap`) via `.set()` calls and then read `positionsState.keys()` in the same effect run to prune stale entries — an effect that reads and writes the same reactive collection in one pass is exactly the self-referential pattern Svelte's loop guard exists to catch, and it fired: the write scheduled the effect's own re-run indefinitely.
- **Fix:** Rewrote the settle effect to be write-only (`positionsState.clear()` then `.set()` for the fresh membership, never a `.keys()`/`.get()` read in the same body); wrapped the refit effect's `computeBounds()`/`applyTransform()` calls in `untrack()` so its only tracked dependencies are `membershipKey` and `svgEl`, never `positionsState` or `zoomT`.
- **Files modified:** ui/src/lib/components/RelatedGraph.svelte
- **Verification:** the previously-hanging basic render test now completes in under 1s; full 35-test suite passes
- **Committed in:** b30ee9d3 (Task 3 GREEN commit)

**2. [Rule 1 - Bug] Fixed drag test event construction missing `view`/`cancelable`**
- **Found during:** Task 3 (RED→GREEN verification — d3-drag/d3-zoom's internal `mousedowned` handlers threw `Cannot read properties of null (reading 'document')` on synthetic events)
- **Issue:** The drag tests' synthetic `MouseEvent`s omitted `view: window` and `cancelable: true`, which d3-drag's `nodrag`/`nopropagation` helpers rely on internally.
- **Fix:** Added `view: window, cancelable: true` to every mousedown/mousemove/mouseup dispatch in the drag test block.
- **Files modified:** ui/src/lib/components/RelatedGraph.browser.test.ts
- **Verification:** drag tests pass with no unhandled errors
- **Committed in:** b30ee9d3 (Task 3 GREEN commit)

---

**Total deviations:** 2 auto-fixed (2 bugs — one production-code reactive-loop fix, one test-harness fix). **Impact:** both were necessary for correctness; the reactive-loop fix is the mechanism D-20's "selection/focus/drag never refit" contract runs on, so this was not scope creep but the actual implementation of the locked decision.

## Issues Encountered

None beyond the deviations documented above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

RelatedGraph.svelte now exposes `summary?`, `onleave?`, `reducedMotion?` props alongside the existing `anchorId/nodes/edges/selectedId/dimmedIds/onselect/onrecenter` set from plan 05-01. Plan 05-08 wires `summary` and `onleave` into the `/related/[id]` route (the route itself is untouched by this plan, per the plan's own scope note). No blockers.

---
*Phase: 05-related-memories-graph-tag-cloud*
*Completed: 2026-09-28*

## Self-Check: PASSED

- FOUND: ui/src/lib/related/zoom.ts
- FOUND: ui/src/lib/related/zoom.test.ts
- FOUND commit 58309590 (Task 1)
- FOUND commit b30ee9d3 (Task 3 GREEN)
- All 3 tasks' `<verify>` commands re-confirmed passing before this SUMMARY was written (zoom.test.ts 12/12, RelatedGraph.browser.test.ts + related.browser.test.ts 48/48, `pnpm --dir ui build` succeeded)
