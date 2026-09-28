---
phase: 05-related-memories-graph-tag-cloud
plan: 07
subsystem: ui
tags: [svelte, tanstack-query, connect-web, bits-ui, tags, sheet]

# Dependency graph
requires:
  - phase: 05-related-memories-graph-tag-cloud
    provides: "plan 05-03's TagBars.svelte and TagCombobox.svelte (shared tag popularity list and + tag picker)"
provides:
  - "FacetStrip.svelte: 'tagsPanelOpen'/'ontagspanel' toggle props and a mounted TagCombobox after the #tag chips"
  - "/search +page.svelte: page-local tagsPanelOpen state, toggleTag(), and the docked Tags panel sharing RecallSplit's right slot with DetailPane, falling back to a bottom Sheet below the narrow breakpoint"
  - "RecallSplit.svelte: bindable 'narrow' prop mirroring its own isNarrow breakpoint"
affects: [05-09 phase verification]

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 7366
  tasks: 3
  commits: 5
  plan_head_before: 289c0bc2e90d22755de10aff1c57534891672a9c
  plan_head_after: 72d2ba556e1d9b8697b6036578a8ff5a843accc8

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "The Tags panel's open state is page-local ($state), never URL state -- a shared /search?... link keeps its tag filters, not the panel"
    - "A host reuses RecallSplit's own narrow/wide breakpoint via a bindable 'narrow' prop rather than a second width measurement on the page"
    - "A dialog's required accessible Title can duplicate an already-visible header from a child component by setting the Title to the identical text and hiding it visually (class=\"sr-only\") rather than inventing separate wording"

key-files:
  created: []
  modified:
    - ui/src/lib/components/FacetStrip.svelte
    - ui/src/lib/components/FacetStrip.browser.test.ts
    - ui/src/lib/components/RecallSplit.svelte
    - ui/src/routes/search/+page.svelte
    - ui/src/routes/search/search.browser.test.ts

key-decisions:
  - "Sheet.Title carries the identical 'Tags · counts in {scope}' text TagBars already renders as its own visible .header line, but is set class=\"sr-only\" -- satisfies both D-13's 'the panel header as its title' instruction and the required Dialog accessible name, without a visually duplicated heading"
  - "The panel's own close control (aria-label \"close\") in the wide-slot branch is a page-local Button + XIcon next to TagBars, since TagBars itself carries no close affordance and Task 3's files_modified scope excluded TagBars.svelte"
  - "Fixed two of Task 1's own tests (Rule 1 auto-fix): they asserted the docked-panel text without forcing a wide container width, which happened to pass before Task 3 existed but broke once the narrow bottom-sheet branch could also render the identical text -- the render harness's default width is below RecallSplit's 760px breakpoint"

requirements-completed: [TAGS-01, TAGS-02]

coverage:
  - id: D1
    description: "A '▦ Tags panel' toggle at the right end of FacetStrip opens a docked Tags panel in /search's right slot; a bar click toggles that tag's #tag facet chip in the URL and the query refetches"
    requirement: TAGS-01
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — docked Tags panel toggle (TAGS-01, D-13)"
        status: pass
    human_judgment: false
  - id: D2
    description: "A '+ tag' TagCombobox in FacetStrip, right after the #tag chips, adds a chosen or typed tag once to the facet tags with counts for the panel's own scope"
    requirement: TAGS-02
    verification:
      - kind: automated_ui
        ref: 'ui/src/lib/components/FacetStrip.browser.test.ts#"+ tag" picker (TAGS-02, D-16)'
        status: pass
    human_judgment: false
  - id: D3
    description: "The panel shares the right slot with DetailPane (opening a record replaces it, closing it brings the panel back), and below RecallSplit's narrow breakpoint renders as a bottom Sheet instead"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — Tags panel shares the slot with the detail pane, narrow sheet (D-13)"
        status: pass
    human_judgment: true
    rationale: "Behavior (slot swap, close control, sheet role, active-tag markers) is fully asserted, but the actual visual layout of the docked panel and the 62vh bottom sheet -- spacing, dashed-pill styling, sr-only title placement -- is only captured as unasserted screenshots, not pixel-verified."

# Metrics
duration: 34min
completed: 2026-09-28
status: complete
---

# Phase 5 Plan 7: /search Tag Surfaces Summary

**Docked Tags panel + "+ tag" picker land on /search: a `▦ Tags panel` toggle shares the detail-pane slot with a bottom-sheet fallback below 760px, and a TagCombobox beside the #tag chips adds a filter with live counts.**

## Performance

- **Duration:** 34 min
- **Started:** 2026-09-28T03:46:00Z
- **Completed:** 2026-09-28T04:20:50Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- `FacetStrip.svelte` gained a `▦ Tags panel` toggle (last element of the strip) and a `TagCombobox` right after the `#tag` chips, both reusing the strip's dashed-pill-turns-solid-violet look.
- `/search +page.svelte` wires `tagsPanelOpen` (page-local state, never URL state) and `toggleTag()` (toggles a tag in `params.tags` via the existing `navigate`/URL codec) so a panel bar click behaves exactly like removing a `#tag` chip in reverse.
- The panel takes turns with `DetailPane` in `RecallSplit`'s right slot: opening a record replaces the panel, closing the record (via `DetailPane`'s own close button) brings the panel back while the toggle stays on, and a matching close control on the panel branch turns the toggle off.
- Below `RecallSplit`'s own 760px breakpoint (exposed via a new bindable `narrow` prop, avoiding a second width measurement) the panel instead renders as a `Sheet.Content side="bottom"` (`62vh`) holding the identical `TagBars`.

## Task Commits

Each task was committed atomically, with tasks 2 and 3 following RED (`test(...)`) then GREEN (`feat(...)`) commits per `tdd.md`; task 1 (tracer) committed directly since it is not a `tdd="true"` task:

1. **Task 1: "▦ Tags panel" toggle opens TagBars in /search's right slot; a bar click toggles a URL tag filter** (tracer) - `8fa6faea` (feat)
2. **Task 2: The "+ tag" picker beside the tag chips** (tdd) - `b3f73c6e` (test, RED) -> `095e1836` (feat, GREEN)
3. **Task 3: Slot sharing with DetailPane, active markers, and the narrow bottom sheet** (tdd) - `9c8218d1` (test, RED) -> `72d2ba55` (feat, GREEN)

_Plan metadata commit follows this SUMMARY._

## Files Created/Modified

- `ui/src/lib/components/FacetStrip.svelte` - Tags panel toggle button; mounted `TagCombobox` after the `#tag` chips
- `ui/src/lib/components/FacetStrip.browser.test.ts` - QueryClientProvider wrapper + `engram.listTags` mock (now required for every render since `TagCombobox` mounts unconditionally); new "+ tag" picker behavior tests
- `ui/src/lib/components/RecallSplit.svelte` - bindable `narrow` prop mirroring `isNarrow`
- `ui/src/routes/search/+page.svelte` - `tagsPanelOpen`/`toggleTag`/`splitNarrow` state; docked panel in the shared slot; narrow bottom `Sheet`
- `ui/src/routes/search/search.browser.test.ts` - `engram.listTags` mock/spy; toggle, scope-aware call, bar-click, slot-sharing, and narrow-sheet behavior tests plus screenshots

## Decisions Made

See `key-decisions` in the frontmatter above (sr-only Sheet.Title reusing TagBars' own header text, the page-local close control for the panel branch, and the Task 1 test fix).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Sheet.Title duplicated TagBars' own visible header text**
- **Found during:** Task 3, first full-file test run after implementing the narrow sheet
- **Issue:** Setting `Sheet.Title` to the same "Tags · counts in …" text the plan specifies, with no visual distinction, rendered TWO elements with identical text inside the sheet (the dialog's heading plus `TagBars`' own `.header` line) — any text-based query hit a Playwright strict-mode "resolved to 2 elements" failure, and visually it read as a redundant duplicate heading.
- **Fix:** Kept the identical text (satisfies the required accessible dialog name and the plan's "the panel header as its title" instruction) but added `class="sr-only"` to `Sheet.Title` so only `TagBars`' own visible header renders on screen.
- **Files modified:** `ui/src/routes/search/+page.svelte`
- **Verification:** `search.browser.test.ts`'s narrow-sheet tests pass; no ambiguous-match errors
- **Committed in:** `72d2ba55` (Task 3 GREEN commit)

**2. [Rule 1 - Bug] Two of Task 1's own tests never forced a wide container width**
- **Found during:** Task 3, the same first full-file run
- **Issue:** `search.browser.test.ts`'s Task 1 tests ("starts closed…", "with ?scope=… set…") asserted the docked-panel text without setting `screen.container.style.width`. This happened to pass under Task 1 alone because nothing else in the tree could render that text elsewhere, but once Task 3 added a narrow-viewport bottom sheet carrying the identical text, the render harness's default (unstyled) width — measured to be below `RecallSplit`'s 760px breakpoint — meant these tests were unknowingly exercising the *narrow* branch, and after the sr-only fix above they'd have silently asserted against the sheet instead of the docked panel.
- **Fix:** Added `screen.container.style.width = '1200px'` to both tests, matching the existing convention used elsewhere in the same file for forcing the wide layout.
- **Files modified:** `ui/src/routes/search/search.browser.test.ts`
- **Verification:** All three "docked Tags panel toggle" tests pass under the explicit wide width
- **Committed in:** `9c8218d1` (Task 3 test commit, alongside the new Task 3 tests)

---

**Total deviations:** 2 auto-fixed (2 bugs, both test-correctness issues surfaced by the plan's own verification loop). **Impact on plan:** Both were necessary corrections; no scope creep or behavior beyond what the plan specified.

## Issues Encountered

One test in this file ("a row toolbar Chain button opens the chain dialog for that anchor", pre-existing, not authored or modified by this plan) intermittently timed out on a hover-driven interaction when the full 50-test file ran under load, but passed reliably alone and on a clean re-run of the full file. Confirmed as pre-existing flakiness unrelated to this plan's changes (final full-suite runs — used for the plan's own `<verify>` gates — passed clean at 50/50, 60/60, and 15/15 across the three affected files).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All three of `05-07`'s must-have truths are implemented and behavior-tested: the docked panel toggle, the `+ tag` picker, slot sharing with `DetailPane`, active-tag markers, and the narrow bottom sheet.
- No blockers for phase verification (05-09). The one flagged item is the D3 visual-layout human-judgment note above — the docked panel and sheet look correct in unasserted screenshots but have not been pixel-verified by a human.

## Self-Check: PASSED

All 5 modified files verified present on disk with the expected content; all 5 commits (`8fa6faea`, `b3f73c6e`, `095e1836`, `9c8218d1`, `72d2ba55`) verified present in `git log`.

---
*Phase: 05-related-memories-graph-tag-cloud*
*Completed: 2026-09-28*
