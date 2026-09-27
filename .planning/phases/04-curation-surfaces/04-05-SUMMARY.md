---
phase: 04-curation-surfaces
plan: 05
subsystem: ui
tags: [svelte, tanstack-query, curation, multi-select, keyboard, tdd]

# Dependency graph
requires:
  - phase: 04-curation-surfaces
    provides: "04-01's CurationSurfaces.openArchive/openRestore, ArchiveConfirmDialog, flash.svelte.ts (flashing/FLASH_MS/flashRows) and applyToMemoryCaches export"
provides:
  - "ResultsList/ResultRow/ResultsHeader multi-select: x/⇧X/⇧click select, a/⇧A/⇧S archive/restore/supersede the selection (or the active row alone), a check column, a per-row flash highlight, and a bulk-bar header swap"
  - "Selection lifecycle (D-04): a selectionKey-driven clear on query/facet change, survival across Show more and pane open/close, pruning against the live memories array, and a clear after any completed curation call"
  - "applyToMemoryCaches widened to infinite-query { pages: [...] } caches (listMemories cursor mode and listScheduled), plus useDeleteMemory now invalidating listRules/listScheduled through a new component-free invalidateAfterDelete"
affects: [04-06, 04-07, 04-08, 04-09, 04-10]

# Actuals (#2632)
actuals:
  tokens: 15384
  tasks: 3
  commits: 5
  plan_head_before: 24d54c860b2e75704db8b0884b606b844e99f55d
  plan_head_after: 3ae1210430627527d5c7eead83b09e1ab26ff98d

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Range-select anchor: only the single-toggle key (x) sets `anchorId`; both the range-extend key (Shift+X) and shift+click read from it without reassigning, so repeated range extensions stay relative to the first toggled row"
    - "selectionKey as a route-owned 'what changed' digest: ResultsList clears its own selectedIds when the prop VALUE changes, never on array/object identity, so the route computes it once (encodeSearchParams with k/sel pinned) and the component stays agnostic of URL shape"
    - "ResultsHeader.selection is a pure render prop (count + optional per-verb callbacks + a required onclear) — the component never counts a selection or owns curation logic, only lays out the bulk bar the route hands it"
    - "applyToMemoryCaches detects an infinite-query cache STRUCTURALLY (an array `pages` field), not by query key, so mapMemoriesField's per-record transform composes into a page-aware version without a second walker per list shape"
    - "invalidateAfterDelete mirrors curation.ts's invalidateAfterCuration: a component-free (no useQueryClient) exported function taking a QueryClient, so its invalidation set is spyable from a plain node test without a Svelte QueryClientProvider"

key-files:
  created: []
  modified:
    - ui/src/lib/components/ResultsList.svelte
    - ui/src/lib/components/ResultsList.browser.test.ts
    - ui/src/lib/components/ResultRow.svelte
    - ui/src/lib/components/ResultRow.browser.test.ts
    - ui/src/lib/components/ResultsHeader.svelte
    - ui/src/lib/components/ResultsHeader.browser.test.ts
    - ui/src/routes/search/+page.svelte
    - ui/src/routes/search/search.browser.test.ts
    - ui/src/lib/mutations/memory.ts
    - ui/src/lib/mutations/memory.test.ts

key-decisions:
  - "Range-select anchor semantics: Shift+X and shift+click both extend from the LAST `x`-toggled row, never from the previously active row or a reassigned anchor — matches the plan's 'x on row 1 sets the anchor... selects rows 1-4... selects 4-6 as well' description read as successive extensions of one anchor, not independent range picks."
  - "The check column's opacity trigger is list-wide (`selectionActive` = selectable && selectedIds.length > 0), not per-row hover-only — a row with no selection anywhere still shows the check faintly on its own hover, but once ANY row is selected every row's check brightens, matching the must-have wording 'faint until row hover or a non-empty selection' read as an OR of two list-level/row-level conditions."
  - "ResultsHeader's bulk-bar action buttons are bare `<button>` elements styled like `.rh-scopes-btn` (background:none, no fixed height), not the shadcn `Button` component — Button's `size=\"sm\"` carries a fixed Tailwind `h-7` (28px, NOT scaled by the site's --u/--ui-font token), which would make the bulk bar taller than the plain `.rh-line` text row and violate the must-have 'the results header swaps its whole content (same height)'."
  - "applyToMemoryCaches's infinite-page detection is structural (`Array.isArray(data.pages)`) rather than keyed off queryKey shape, so a future list shape that also happens to use createInfiniteQuery is covered automatically without a new branch."

patterns-established:
  - "A route-level selection array lives at the /search level ($state, bound down via bind:selectedIds) and both ResultsList's own keyboard/pointer paths and ResultsHeader's bulk bar mutate it through the SAME prop, keeping curation targets always computed from one source of truth"

requirements-completed: [CUR-02]

coverage:
  - id: D1
    description: "Task 1 tracer: select two rows on /search with x, press a, confirm archives both in place and clears the selection"
    requirement: "CUR-02"
    verification:
      - kind: automated_ui
        ref: "src/routes/search/search.browser.test.ts#search route — bulk archive with x and a (D-01..D-03 tracer) > selects two rows with x/j/x, archives them together through the confirm dialog, and clears the selection"
        status: pass
    human_judgment: false
  - id: D2
    description: "Range selection (Shift+X, shift+click), three-tier Escape (selection > card > onescape), the leading check column with correct grid-track parity at every container band, and the key legend showing only supplied hints"
    verification:
      - kind: automated_ui
        ref: "src/lib/components/ResultsList.browser.test.ts (describe 'ResultsList — multi-select (D-01, D-02, D-03)', 10 cases incl. a DSYS-04 screenshot)"
        status: pass
      - kind: automated_ui
        ref: "src/lib/components/ResultRow.browser.test.ts (describe 'selection (D-02, D-10)' and 'flash (D-10)', 5 cases)"
        status: pass
    human_judgment: false
  - id: D3
    description: "The bulk bar (header content swap, same height, wraps below the narrow breakpoint), the D-04 selection lifecycle, and the infinite-page cache patch for listMemories cursor mode and listScheduled plus delete invalidation"
    verification:
      - kind: automated_ui
        ref: "src/lib/components/ResultsHeader.browser.test.ts (describe 'ResultsHeader — bulk bar (D-03)', 6 cases incl. a wrap screenshot)"
        status: pass
      - kind: automated_ui
        ref: "src/routes/search/search.browser.test.ts (describe 'search route — selection lifecycle (D-04)', 4 cases)"
        status: pass
      - kind: unit
        ref: "src/lib/mutations/memory.test.ts (describe 'applyToMemoryCaches — infinite query pages' and 'invalidateAfterDelete', 4 cases)"
        status: pass
    human_judgment: false

# Metrics
duration: 30min
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 5: Bulk selection, curation keys and the row-flash/cache-patch contract for every list shape Summary

**Multi-select ships on /search: x/⇧X/⇧click build a selection, a/⇧A/⇧S drive it through the existing archive/restore confirm, the results header becomes a bulk bar while a selection is active, and `applyToMemoryCaches` now patches infinite-query pages (listMemories cursor mode, listScheduled) in place instead of just offset-mode lists.**

## Performance

- **Duration:** ~30 min
- **Tasks:** 3 of 3 completed (Task 1 tracer, Task 2 TDD, Task 3 TDD)
- **Files modified:** 10 (0 created, 10 modified — exactly the plan's `files_modified` list)

## Accomplishments

- `ResultsList.svelte`: `selectable`/`selectedIds` (bindable)/`selectionKey`/`onsupersede`/`onarchive`/`onrestore` props. `x` toggles the active row and sets the range-select anchor; `⇧X` and shift+click both extend an inclusive range FROM that anchor (never reassigning it) without opening the pane; a click on the row's check cell also toggles without opening. `a`/`⇧A`/`⇧S` submit the selection (in list order) — or the active row alone when nothing is selected — to `onarchive`/`onrestore`/`onsupersede`. Escape now clears a non-empty selection first, only falling through to the existing hover-card-then-`onescape` tiers once the selection is empty. `aria-multiselectable="true"` and per-row `aria-selected` (selection membership, not just the active row) are wired when `selectable`. The legend appends `x`/`⇧X` hints when `selectable`, and `⇧S`/`a`/`⇧A` hints only for the callbacks actually supplied.
- `ResultRow.svelte`: `selectable`/`selected`/`selectionActive` props render a leading `calc(28 * var(--u))` check-column grid track (base rule and both `@container list` bands), a decorative `aria-hidden` check element (never a focusable control nested inside `role="option"`, per WCAG 4.1.2), faint until row-hover or list-wide `selectionActive`. Reads the shared `flashing` set from `flash.svelte.ts` to apply a 1.6s `row-flash` highlight after a curation write.
- `ResultsHeader.svelte`: new `selection` prop swaps the parts line for a `role="toolbar" aria-label="Bulk actions"` bulk bar exactly while `count > 0` — `"{N} selected · Supersede N into one… ⇧S · Archive a · Restore ⇧A · clear"`, rendering only the actions the route supplies, `busy` disabling the verb buttons (never `clear`). Built from bare styled `<button>`s (not the shadcn `Button`, whose fixed `h-7` would break height parity with `.rh-line`), so the bar shares the plain header's exact box height; `flex-wrap` lets actions drop to a second line below the narrow breakpoint instead of truncating.
- `/search/+page.svelte`: route-level `selectedIds` state wired to `ResultsList` (`selectable`, `bind:selectedIds`, `selectionKey={encodeSearchParams({ ...params, k: DEFAULT_K, sel: '' })}`, `onarchive`/`onrestore` opening the existing `CurationSurfaces` dialog) and to `ResultsHeader` (`selection={{ count, onarchive, onrestore, onclear }}`); `CurationSurfaces.onchanged` clears the selection after any completed archive/restore.
- `memory.ts`: `applyToMemoryCaches` now detects an infinite-query cache value structurally (`Array.isArray(data.pages)`) and maps each page through the same per-record `fn` used for offset-mode lists — covers the `listMemories` cursor-mode cache and a new `listScheduled` walk; `MEMORY_LIST_PREFIXES` gained `['listScheduled']` so `snapshotMemoryQueries`/`restoreMemoryQueries` cover it too. `useDeleteMemory`'s `onSettled` now calls a new exported, component-free `invalidateAfterDelete(queryClient)` (mirrors `curation.ts`'s `invalidateAfterCuration`) that also invalidates `listRules` and `listScheduled`.

## Task Commits

Each task was committed atomically (Tasks 2 and 3 ran under TDD: test → feat):

1. **Task 1 (tracer): bulk archive with x and a** — `85e75c55` (feat)
2. **Task 2: range selection, Escape tiers, check column, row flash** — `33d64441` (test, RED) → `d9e7e867` (feat, GREEN)
3. **Task 3: bulk bar, selection lifecycle, infinite-page cache patch** — `060cdcd3` (test, RED) → `3ae12104` (feat, GREEN)

**Plan metadata:** this commit (docs: complete plan)

_Note: no REFACTOR commit was needed for either TDD task — each GREEN implementation required no follow-up cleanup._

## Files Created/Modified

- `ui/src/lib/components/ResultsList.svelte` — multi-select, range selection, curation keys, selection lifecycle
- `ui/src/lib/components/ResultsList.browser.test.ts` — bulk-archive tracer coverage plus the Task 2 multi-select describe block (10 cases)
- `ui/src/lib/components/ResultRow.svelte` — check column, row flash
- `ui/src/lib/components/ResultRow.browser.test.ts` — selection grid-track/aria-hidden and flash-lifecycle coverage (5 cases)
- `ui/src/lib/components/ResultsHeader.svelte` — the bulk-bar header swap
- `ui/src/lib/components/ResultsHeader.browser.test.ts` — bulk-bar coverage (6 cases)
- `ui/src/routes/search/+page.svelte` — selectedIds route state, selectionKey, selection wiring
- `ui/src/routes/search/search.browser.test.ts` — bulk-archive tracer test plus D-04 selection-lifecycle describe block (4 cases)
- `ui/src/lib/mutations/memory.ts` — infinite-page-aware `applyToMemoryCaches`, `invalidateAfterDelete`
- `ui/src/lib/mutations/memory.test.ts` — infinite-page patch coverage plus `invalidateAfterDelete` spy test

## Decisions Made

See `key-decisions` in frontmatter for the full rationale on each. Summary: the range-select anchor never reassigns except on `x`; the check column's brightening is list-wide (any active selection), not purely per-row hover; the bulk bar uses plain styled buttons instead of the shadcn `Button` to preserve exact header height parity; `applyToMemoryCaches`'s infinite-page detection is structural rather than keyed, so it generalizes to any future `createInfiniteQuery` list shape without a new branch.

## Deviations from Plan

None — all three tasks implement exactly what PLAN.md specifies. No Rule 1-4 auto-fixes were needed.

## TDD Gate Compliance

Task 2 (`tdd="true"`) and Task 3 (`tdd="true"`):

| Task | Gate | Commit | Notes |
|------|------|--------|-------|
| 2 | RED | `test(04-05): add failing tests for range selection, Escape tiers, check column and row flash` (`33d64441`) | All 10 new cases across `ResultsList.browser.test.ts`/`ResultRow.browser.test.ts` failed on real assertions (unimplemented range-select/Escape-tier/check-column/flash behavior) — confirmed genuine RED (not import crashes, not zero-test discovery); the pre-existing 45 cases stayed green. |
| 2 | GREEN | `feat(04-05): range selection, check column, Esc tiers and row flash (D-01..D-03, D-10)` (`d9e7e867`) | All 56 cases across both files pass; two test-authoring timing fixes were needed mid-GREEN (see Issues Encountered) and are included in this commit. |
| 2 | REFACTOR | — | Not needed. |
| 3 | RED | `test(04-05): add failing tests for the bulk bar, selection lifecycle and infinite-page cache patch` (`060cdcd3`) | 3 of 27 `memory.test.ts` cases and 4 of 9 `ResultsHeader.browser.test.ts` cases failed genuinely (unpatched infinite pages, missing `invalidateAfterDelete` export, missing `selection` prop). 1 of the 4 new `search.browser.test.ts` D-04 cases was genuine RED ("changing the query clears the selection"); the other 3 already held true without the `selectionKey` feature — the tdd.md-sanctioned "feature may already exist" case (same shape as 04-01-SUMMARY.md's `curation.test.ts` precedent), not a gate violation. |
| 3 | GREEN | `feat(04-05): bulk bar, selection lifecycle and in-place patch for every list shape (D-03, D-04, D-10)` (`3ae12104`) | All 3 `<verify>` commands pass: node project (27/27), browser project (97/97 across all four component/route files), and `pnpm --dir ui build` exits 0. |
| 3 | REFACTOR | — | Not needed. |

`workflow.tdd_mode` is off for this project (absent from `.planning/config.json`, defaults `false`), so the machine-enforced `gsd_run check tdd-red-evidence` CLI gate was not invoked; RED validity was confirmed by direct inspection of each run's failure output (real assertion failures on named tests, matching test counts before/after), the same manual-verification precedent 04-01-SUMMARY.md recorded.

## Issues Encountered

Two test-authoring races were found and fixed while writing Task 1's tracer test and Task 2's RED tests (not implementation bugs — the production code was correct once the test itself stopped racing its own assertions):

1. **RecallSplit's narrow→wide flip is asynchronous (ResizeObserver).** Setting `screen.container.style.width` synchronously after mount does not retroactively change what `RecallSplit`'s `isNarrow` derived saw at MOUNT time — if the container's natural (pre-style) width was narrow, the component initially renders the narrow/overlay branch, then flips to wide once the ResizeObserver callback fires on a later frame, unmounting/remounting `ResultsList` in the process. A `.focus()` call issued between mount and that flip silently blurs to `<body>` when the flip tears down the focused node. Fixed by polling for `.rs-group` (the wide-layout marker) before focusing/keying, on top of the CUR-02 tracer's existing width-forcing convention.
2. **A raw `dispatchEvent`/direct DOM mutation is not synchronous with Svelte's reactive re-render.** `fireKey(...)` and manual `MouseEvent` dispatches (`X`+shift, the row-check click) update `$state` synchronously, but the resulting DOM attribute change (`aria-selected`) lands on a later microtask. Tests that asserted immediately after a bare dispatch (no `await userEvent.keyboard(...)`, which already awaits internally) needed an explicit `expect.poll(...)` added.

## Known Stubs

None. `onsupersede` is fully wired through `ResultsList`/`ResultsHeader` (keys, targets, legend hint, bulk-bar button) but `/search` itself does not yet pass an `onsupersede` callback to either component — this is explicitly out of this plan's scope ("Supersede keys fire `onsupersede`, which /search wires in plan 04-08" per the plan's own objective) and produces no dangling UI: the legend/bulk-bar hint for supersede simply does not render until a callback is supplied, by design (both already tested for the no-onsupersede case).

## Threat Flags

None. No new RPC calls, endpoints, or trust-boundary-crossing surface was introduced beyond what the plan's own `<threat_model>` (T-04-11, T-04-12) already covers — this plan's changes are entirely client-side selection state and existing-cache-shape patching.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The selection model (`selectedIds`, `selectionKey`, `anchorId`) and the bulk bar contract (`ResultsHeader.selection`) are the shared shape plans 04-07 through 04-10 build their own selection-aware surfaces on.
- `applyToMemoryCaches`'s infinite-page support is available to any future curation write that touches a `createInfiniteQuery`-backed list, without a new per-list-shape walker.
- Plan 04-08 is expected to wire `/search`'s own `onsupersede` callback into `ResultsList`/`ResultsHeader`, completing the D-01 key set already implemented here.
- Plan 04-06 (per this plan's `coupling_justified` note) can land in either order relative to this plan — `applyToMemoryCaches`'s exported signature is unchanged, only its internals widened.

---
*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All 10 key-files (modified) verified present on disk with `[ -f ]`.
- All 5 commits (`85e75c55`, `33d64441`, `d9e7e867`, `060cdcd3`, `3ae12104`) verified present in `git log --oneline 24d54c86..HEAD`.
- Re-ran all three tasks' plan-level `<verify>` commands: browser project (4 files, 97/97 tests passed), node project (2 files, 27/27 tests passed), `pnpm --dir ui build` exits 0 (confirmed separately during Task 3 GREEN).
- Re-ran all 9 acceptance-criteria `rg` checks across the three tasks — all matched their required counts (Task 1: 4/1/1; Task 2: 1/5/1/1; Task 3: 2/1/1/1).
