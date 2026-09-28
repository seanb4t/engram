---
phase: 05-related-memories-graph-tag-cloud
reviewed: 2026-09-28T05:37:41Z
depth: standard
files_reviewed: 44
files_reviewed_list:
  - .claude/skills/engram-connect-client/SKILL.md
  - .claude/skills/engram-console-conventions/SKILL.md
  - internal/e2e/console_browser_test.go
  - ui/package.json
  - ui/src/lib/a11y/surfaces.browser.test.ts
  - ui/src/lib/components/CommandMenu.browser.test.ts
  - ui/src/lib/components/CommandMenu.svelte
  - ui/src/lib/components/DetailPane.svelte
  - ui/src/lib/components/EdgeLane.svelte
  - ui/src/lib/components/EvidenceSection.svelte
  - ui/src/lib/components/FacetStrip.browser.test.ts
  - ui/src/lib/components/FacetStrip.svelte
  - ui/src/lib/components/GraphLegend.svelte
  - ui/src/lib/components/HeaderSearch.browser.test.ts
  - ui/src/lib/components/HeaderSearch.svelte
  - ui/src/lib/components/RecallSplit.svelte
  - ui/src/lib/components/RelatedGraph.browser.test.ts
  - ui/src/lib/components/RelatedGraph.svelte
  - ui/src/lib/components/ResultsList.browser.test.ts
  - ui/src/lib/components/ResultsList.svelte
  - ui/src/lib/components/SupersessionLane.svelte
  - ui/src/lib/components/TagBars.browser.test.ts
  - ui/src/lib/components/TagBars.svelte
  - ui/src/lib/components/TagCombobox.browser.test.ts
  - ui/src/lib/components/TagCombobox.svelte
  - ui/src/lib/components/TagMatchRow.svelte
  - ui/src/lib/related/graph.test.ts
  - ui/src/lib/related/graph.ts
  - ui/src/lib/related/lanes.test.ts
  - ui/src/lib/related/lanes.ts
  - ui/src/lib/related/zoom.test.ts
  - ui/src/lib/related/zoom.ts
  - ui/src/lib/search/related-params.test.ts
  - ui/src/lib/search/related-params.ts
  - ui/src/lib/tags/query.ts
  - ui/src/lib/tags/tags.test.ts
  - ui/src/lib/tags/tags.ts
  - ui/src/routes/related/[id]/+page.svelte
  - ui/src/routes/related/related.browser.test.ts
  - ui/src/routes/rules/+page.svelte
  - ui/src/routes/rules/rules.browser.test.ts
  - ui/src/routes/scheduled/+page.svelte
  - ui/src/routes/search/+page.svelte
  - ui/src/routes/search/search.browser.test.ts
findings:
  critical: 0
  warning: 4
  info: 2
  total: 6
status: issues_found
---

# Phase 05: Code Review Report

**Reviewed:** 2026-09-28T05:37:41Z
**Depth:** standard
**Files Reviewed:** 44
**Status:** issues_found

## Summary

Reviewed the related-memories graph/tag-cloud surfaces (`ui/src/lib/related/*`, `ui/src/lib/tags/*`,
`RelatedGraph.svelte`, `EdgeLane.svelte`, `SupersessionLane.svelte`, `EvidenceSection.svelte`,
`GraphLegend.svelte`, `TagBars.svelte`, `TagCombobox.svelte`, `TagMatchRow.svelte`, the `/related`,
`/rules`, `/scheduled`, `/search` routes, `HeaderSearch.svelte`, `CommandMenu.svelte`,
`DetailPane.svelte`, the Go console e2e harness, and package.json) at standard depth.

The pure model layer (`related/graph.ts`, `related/lanes.ts`, `related/zoom.ts`, `search/related-params.ts`,
`tags/tags.ts`) is well-factored, side-effect-free, and thoroughly unit-tested — no correctness issues
found there. The Svelte components generally follow the project's stated conventions (honest-feedback
copy, AA-safe dim rule, shared `hiddenTypes`/selection state, CSRF-gated writes untouched by this phase).
No hardcoded secrets, `eval`, unsanitized `{@html}`, or SQL/command-injection-shaped code was found in
scope. Findings below are maintainability/consistency gaps and two verifiable UX/robustness edge cases
rather than crashes or security defects — no BLOCKER-tier issue was found.

## Warnings

### WR-01: `RelatedGraph.svelte`'s floating-card glyph map hand-copies `LANE_GLYPH` instead of importing it

**File:** `ui/src/lib/components/RelatedGraph.svelte:77`
**Issue:** The component defines its own `TYPE_LETTER` map:

```ts
const TYPE_LETTER: Record<LaneType, string> = { supersession: 'S', citation: 'C', tag: 'T', vector: 'V' };
```

`ui/src/lib/related/lanes.ts:21` already exports the canonical mapping for this exact purpose:

```ts
export const LANE_GLYPH: Record<LaneType, string> = { supersession: 'S', citation: 'C', tag: 'T', vector: 'V' };
```

and the project's own convention (comment right above `LANE_GLYPH`, and repeated in `GraphLegend.svelte`,
which correctly imports it) states: *"LANE_GLYPH is the one letter-glyph map every lane row, the evidence
section and the legend read -- never a second hand-copied mapping."* `RelatedGraph.svelte` is the one
place in this phase that violates that invariant. The values happen to agree today, but nothing enforces
that agreement — a future edit to one map (e.g. adding a 5th lane type, or restyling a glyph) can silently
desync the floating focus card from the legend/evidence section it is supposed to match.
**Fix:**
```ts
import { LANE_GLYPH } from '$lib/related/lanes';
// remove the local TYPE_LETTER const; use LANE_GLYPH[ty] in the template instead
```

### WR-02: `RelatedGraph.svelte`'s auto-pan effect can be re-triggered by an unrelated node's drag, not just the focused node's own movement

**File:** `ui/src/lib/components/RelatedGraph.svelte:248-259`
**Issue:** The auto-pan effect is written to depend only on `activeId` and that node's own position:

```ts
$effect(() => {
  const id = activeId;
  if (id === null) return;
  const pos = positionsState.get(id);
  if (!pos) return;
  untrack(() => {
    if (!svgEl || !zoomBehavior) return;
    if (isOutsideView(pos, zoomT, VIEW)) {
      select(svgEl).call(zoomBehavior.translateTo, pos.x, pos.y);
    }
  });
});
```

but `positionsState` is a single shared `SvelteMap` that the drag handler's `sim.on('tick', ...)` listener
(lines 114-118) writes into for **every** `simNode` on every simulation tick, not just the dragged one —
because `simulation.alphaTarget(0.25).restart()` (drag `start`, line 386) reheats the whole force layout,
and other nodes (linked via `forceLink`/`forceManyBody`) legitimately move in response to the dragged
node. If a keyboard-focused node (`activeId`) is different from the node currently being dragged, its
position can still shift slightly during that reheat, re-running this effect and — if `isOutsideView`
trips — calling `zoomBehavior.translateTo`, which re-centres the viewport out from under the user while
they are mid-drag on a different node. This is not covered by `RelatedGraph.browser.test.ts`'s drag test
(which only exercises a single-node graph where this interaction can't manifest).
**Fix:** Gate the effect on a `dragging` flag (already implicitly known to the component via the drag
action's `start`/`end` callbacks) so it never calls `translateTo` while any drag is in progress, e.g.
track `let anyDragActive = $state(false)` set in `dragAction`'s `start`/`end` handlers and check
`if (anyDragActive) return;` inside the effect before calling `translateTo`.

### WR-03: `TagBars.svelte`'s roving-tabindex listbox gives no visible indication of the currently-focused row

**File:** `ui/src/lib/components/TagBars.svelte:148-174`
**Issue:** `TagBars` implements a standard roving `aria-activedescendant` listbox (`activeIndex` state,
`onListboxKeydown` moves it, `aria-activedescendant={`${uid}-opt-${activeIndex}`}` on the container), but
no row ever receives a class or style keyed off `activeIndex`:

```svelte
{#each visible.rows as row, i (row.tag)}
  ...
  <div
    id={`${uid}-opt-${i}`}
    role="option"
    aria-selected={selectedTags?.has(row.tag) ?? false}
    class="bar-row"
    class:marked={markedTags?.has(row.tag) ?? false}
    ...
```

`aria-selected` is driven by `selectedTags` (the applied tag filter), not by `activeIndex`, so there is no
`class:active={i === activeIndex}` (or equivalent) anywhere in this component — confirmed by
`TagBars.browser.test.ts:265-281`, which only asserts `aria-activedescendant` moves correctly and never
asserts any visual state on the row. A sighted keyboard user arrowing through the tag list gets no visual
cue of which row Enter/Space will toggle — this is the exact `.kfocus` treatment `RelatedGraph.svelte`
gives its own roving-focus graph nodes (`.node.kfocus .focus { stroke: var(--primary); ... }`), just
missing here. This is a discoverability/robustness gap for WCAG 2.4.7-style focus visibility on a
composite widget, not merely a style nit — a screen-reader user is fine, but a sighted keyboard user is
not.
**Fix:** Add `class:active={i === activeIndex}` to the row and a corresponding `.bar-row.active { outline:
1px solid var(--primary); }` (or `background: var(--selected)`) rule, mirroring `ResultsList`/
`RelatedGraph`'s own keyboard-focus treatment.

### WR-04: `/related/[id]`'s trail-driven "back" can strand the user outside the app on a direct/bookmarked URL

**File:** `ui/src/routes/related/[id]/+page.svelte:204-210`
**Issue:**
```ts
function goBack() {
  if (params.trail.length > 0) {
    history.back();
  } else {
    exitToOrigin();
  }
}
```
`params.trail` is parsed straight from the URL's `?trail=` query param (`related-params.ts`), so a user who
opens a **bookmarked or pasted** `/related/<id>?trail=<a>,<b>` link — i.e. `trail` is non-empty but the
browser's session history contains nothing from this app (a fresh tab, or history cleared) — will have `[`
/ "← back" call `history.back()`, which navigates the browser to whatever page preceded this tab in real
browser history (possibly a different site, or nothing, showing a blank/about:blank-ish state) instead of
falling back to `exitToOrigin()`. `related.browser.test.ts:789` tests only the case where `history.back()`
is the *intended* outcome for a non-empty trail; it does not (and, being a component-level test with a
mocked `goto`, cannot easily) cover the "no real browser history behind this trail" case. This is a
foreseeable dead-end for anyone who bookmarks or shares a `/related/...?trail=...` URL, which the app
itself produces via the crumb links rendered a few lines above (`relatedPath(entry, {...})`).
**Fix:** Track whether the current session actually pushed the trail via in-app `goto` (e.g. a
`sessionStorage`/module-level flag set on `recenter()`), and only call `history.back()` when that flag is
set for the current trail depth; otherwise fall through to `exitToOrigin()` the same as the empty-trail
case.

## Info

### IN-01: `RelatedGraph.svelte`'s `cardPlacement` hardcodes the viewBox half-extents instead of deriving them from `VIEW`

**File:** `ui/src/lib/components/RelatedGraph.svelte:349-359`
**Issue:** The component already defines `const VIEW = { w: 440, h: 380 };` (line 42) and uses it
throughout for fit/zoom math, but `cardPlacement` re-hardcodes the same numbers as separate literals:
```ts
const leftPct = ((sx + 220) / 440) * 100;
const topPct = ((sy + 190) / 380) * 100;
```
`220`/`440` and `190`/`380` are `VIEW.w / 2`, `VIEW.w`, `VIEW.h / 2`, `VIEW.h` respectively. A future
change to `VIEW` (or to the `viewBox` attribute, which is also hand-written as `"-220 -190 440 380"` on
the `<svg>`) would silently desync the focus-card placement math from the actual drawn viewport.
**Fix:** `const leftPct = ((sx + VIEW.w / 2) / VIEW.w) * 100; const topPct = ((sy + VIEW.h / 2) / VIEW.h) * 100;`

### IN-02: Skeleton call-line text hardcodes `k=64` instead of interpolating `RELATED_K`

**File:** `ui/src/routes/related/[id]/+page.svelte:288`
**Issue:** The loading-state skeleton renders a literal string:
```svelte
<div class="call">{'RelatedMemories(subj, "…", k=64) — resolving anchor, 4 typed sub-queries in flight▍'}</div>
```
while the real call line (`callLineParts`, used once data loads) always derives `k` from `model.k`, which
is seeded from the imported `RELATED_K` constant (`related/graph.ts`). The two currently agree (`64`), but
nothing ties them together — a future change to `RELATED_K` would leave the loading-state copy silently
wrong.
**Fix:** `` `RelatedMemories(subj, "…", k=${RELATED_K}) — resolving anchor, 4 typed sub-queries in flight▍` ``
(template literal interpolating the already-imported `RELATED_K`).

---

_Reviewed: 2026-09-28T05:37:41Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
