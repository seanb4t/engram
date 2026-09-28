---
phase: 05-related-memories-graph-tag-cloud
plan: 02
subsystem: ui
tags: [svelte5, connect-web, keyboard, resume, related-memories]

# Dependency graph
requires:
  - phase: 05-related-memories-graph-tag-cloud
    provides: "05-01's /related route (independent — this plan builds the entry points, not the route itself)"
provides:
  - "related-params.ts: the one URL/path codec for /related (from, trail, relatedPath), reusing resume.ts's isAllowedDestination open-redirect guard"
  - "DetailPane's optional onrelated callback and Related button, rendered first in the action row for every record category"
  - "ResultsList's r row key opening the active row's related view, with its own legend hint"
  - "/search, /rules and /scheduled wired to all three entry points via a shared openRelated helper"
  - "CommandMenu's Related to {short_id} item in the Record group"
affects: [05-related-memories-graph-tag-cloud]

# Actuals (#2632)
actuals:
  tokens: 5950
  tasks: 3
  commits: 5

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "related-params.ts mirrors scheduled-params.ts's one-parse/one-encode codec contract"
    - "openRelated() repeated per-route (search/rules/scheduled) rather than factored into a shared helper — each route already owns its own goto/base/page imports and the body is three lines"

key-files:
  created:
    - ui/src/lib/search/related-params.ts
    - ui/src/lib/search/related-params.test.ts
  modified:
    - ui/src/lib/components/DetailPane.svelte
    - ui/src/lib/components/ResultsList.svelte
    - ui/src/lib/components/ResultsList.browser.test.ts
    - ui/src/lib/components/CommandMenu.svelte
    - ui/src/lib/components/CommandMenu.browser.test.ts
    - ui/src/routes/search/+page.svelte
    - ui/src/routes/search/search.browser.test.ts
    - ui/src/routes/rules/+page.svelte
    - ui/src/routes/rules/rules.browser.test.ts
    - ui/src/routes/scheduled/+page.svelte

key-decisions:
  - "The Related button renders unconditionally (no isRule/isDiscovery guard) — RelatedMemories accepts any readable record, matching the plan's action text"
  - "Related is placed first in DetailPane's .d-actions row, ahead of Edit/Supersede/Archive/etc"
  - "Legend hint 'r related' placed after the copy hints (c/⇧C), before the selectable/curation hints"
  - "⌘K's Related to {short_id} item sits after the copy items and before the curation items in recordItems"

patterns-established:
  - "openRelated(id) = goto(`${base}${relatedPath(id, { from: normalizeReturnPath(page.url.pathname + page.url.search), trail: [] })}`) is the canonical navigation call every entry point uses"

requirements-completed: []  # GRAPH-01 is shared with sibling plan 05-01 (same wave, different worktree) — the shared-ID gate (#2388) correctly reports 0/1 ready from this worktree's isolated checkout since 05-01's SUMMARY.md is not visible here. The orchestrator/next plan finalizes it once both plans have merged.

coverage:
  - id: D1
    description: "The /related URL/path codec (related-params.ts) plus the DetailPane 'Related' button on /search, navigating to /related/<id>?from=... with D-04's origin capture"
    requirement: GRAPH-01
    verification:
      - kind: unit
        ref: "ui/src/lib/search/related-params.test.ts (14 tests)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts + ui/src/lib/components/DetailPane.browser.test.ts (61 tests)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Row key r opens the active row's related view on /search, /rules and /scheduled, with a legend hint; ignored under Meta/Ctrl or while typing"
    requirement: GRAPH-01
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultsList.browser.test.ts + rules.browser.test.ts + scheduled.browser.test.ts + search.browser.test.ts (162 tests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "⌘K offers 'Related to {short_id}' when a record is selected, navigating to the related view and closing the menu"
    requirement: GRAPH-01
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts (19 tests)"
        status: pass
    human_judgment: false

duration: 23min
completed: 2026-09-28
status: complete
---

# Phase 5 Plan 2: Related-view entry points Summary

**Three entry points to /related — the DetailPane's Related button, the `r` row key on /search /rules /scheduled, and ⌘K's "Related to {short_id}" — sharing one URL codec (`related-params.ts`) that records where the view was opened from.**

## Performance

- **Duration:** 23 min
- **Started:** 2026-09-28T02:58:40Z
- **Completed:** 2026-09-28T03:24:01Z
- **Tasks:** 3
- **Files modified:** 12 (2 created, 10 modified)

## Accomplishments
- `related-params.ts`: `defaultRelatedParams`, `parseRelatedParams`, `encodeRelatedParams`, `relatedPath` — one codec, mirroring `scheduled-params.ts`'s contract, reusing `resume.ts`'s `isAllowedDestination` open-redirect guard for `from` and `classify.ts`'s `UUID_RE`/`SHORT_ID_RE` for `trail` entries (capped at `RELATED_TRAIL_MAX = 32`)
- `DetailPane` gained an optional `onrelated` callback rendering a "Related" button first in the action row, for every record category (rules and discoveries included)
- `ResultsList` gained the `r` row key (added to `ROW_ACTION_KEYS`, so the existing typing-target and modifier guards apply for free) plus its own legend hint, shown only when `onrelated` is supplied
- `/search`, `/rules` and `/scheduled` each wire a local `openRelated(id)` helper to both their `ResultsList` and `DetailPane`
- `CommandMenu`'s `recordItems` gained a "Related to {short_id}" item, placed after the copy items and before the curation items, present only when a record is selected and its `short_id` is known

## Task Commits

Each task was committed atomically (RED/GREEN split for the two `tdd="true"` tasks):

1. **Task 1: The /related codec and the DetailPane "Related" button (tracer)** - `191fbc40` (feat)
2. **Task 2 RED: failing tests for row key r** - `e5de8176` (test)
3. **Task 2 GREEN: r opens the active row's related view** - `429a04ea` (feat)
4. **Task 3 RED: failing tests for the ⌘K "Related to" item** - `e4c4e5d2` (test)
5. **Task 3 GREEN: ⌘K opens the selected record's related view** - `1935f787` (feat)

_Task 1 (`type="tracer"`) followed the standard commit protocol — no separate RED/GREEN split, since it is not `tdd="true"`. Both TDD tasks needed no REFACTOR commit; the GREEN implementations were already minimal._

## Files Created/Modified
- `ui/src/lib/search/related-params.ts` - the one URL/path codec for `/related`
- `ui/src/lib/search/related-params.test.ts` - 14 node tests covering defaults, round-trip, the open-redirect guard, trail filtering/capping, and path building
- `ui/src/lib/components/DetailPane.svelte` - `onrelated` prop + "Related" button
- `ui/src/lib/components/ResultsList.svelte` - `onrelated` prop, `r` in `ROW_ACTION_KEYS`, `case 'r'`, legend hint
- `ui/src/lib/components/ResultsList.browser.test.ts` - 4 new tests for the `r` key (call, modifier/typing-target exclusion, legend)
- `ui/src/lib/components/CommandMenu.svelte` - `Related to {short_id}` record item
- `ui/src/lib/components/CommandMenu.browser.test.ts` - 3 new tests (item present + navigates, absent with no sel, filters on "rel")
- `ui/src/routes/search/+page.svelte` - `openRelated` helper wired to DetailPane and ResultsList
- `ui/src/routes/search/search.browser.test.ts` - 1 new test for the pane's Related button
- `ui/src/routes/rules/+page.svelte` - `openRelated` helper wired to DetailPane and ResultsList
- `ui/src/routes/rules/rules.browser.test.ts` - updated two pre-existing assertions (legend/action-buttons) that Related now appears in
- `ui/src/routes/scheduled/+page.svelte` - `openRelated` helper wired to DetailPane and ResultsList

## Decisions Made
- Related renders unconditionally in `DetailPane` (no `isRule`/`isDiscovery` guard) — the plan's action text is explicit that `RelatedMemories` accepts any readable record, rules and discoveries included
- `openRelated` is duplicated per route rather than factored into a shared helper: each route already imports `goto`/`base`/`page` independently and the body is three lines, so a shared helper would add an import boundary for no real deduplication
- Legend and menu placement followed the plan's action text literally: `r related` after the copy hints, `Related to {short_id}` after the copy items and before the curation items

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated rules.browser.test.ts's pre-existing delete-only assertions**
- **Found during:** Task 2 verification (`rules.browser.test.ts` failed after wiring `onrelated` onto `/rules`)
- **Issue:** A pre-existing test titled "the pane and legend offer delete only — no edit/visibility/archive/restore/supersede/selection" asserted an exact legend Kbd list and an exact `.d-actions` button list that did not yet include the new (intentional) Related affordance
- **Fix:** Updated both assertions to include `'r'` in the legend list and `'Related'` as the first `.d-actions` button — the test's own title remains accurate (Related is none of edit/visibility/archive/restore/supersede/selection)
- **Files modified:** ui/src/routes/rules/rules.browser.test.ts
- **Verification:** `rules.browser.test.ts` full suite passes (17/17)
- **Committed in:** `429a04ea` (Task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (1 Rule 1 — pre-existing test updated for new intentional behavior)
**Impact on plan:** No scope creep; the fix was a direct, necessary consequence of Task 2's own action text (wire onrelated on /rules).

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The three D-03 entry points are wired end to end (codec, pane button, row key, ⌘K item), each carrying D-04's origin capture via `from`.
- `related-params.ts`'s `trail` field is authored but unconsumed here — plan 05-08 is the stated consumer for the walk/Escape-return behavior.
- GRAPH-01 stays unmarked in this plan's own commit since it is shared with sibling plan 05-01 (same wave, parallel worktree) — the shared-ID gate correctly deferred it; re-run `requirements.mark-complete` once both plans have merged.

## Self-Check: PASSED

Verified: `ui/src/lib/search/related-params.ts` exists, `ui/src/lib/search/related-params.test.ts` exists, all 5 commit hashes (`191fbc40`, `e5de8176`, `429a04ea`, `e4c4e5d2`, `1935f787`) found in git log.

---
*Phase: 05-related-memories-graph-tag-cloud*
*Completed: 2026-09-28*
