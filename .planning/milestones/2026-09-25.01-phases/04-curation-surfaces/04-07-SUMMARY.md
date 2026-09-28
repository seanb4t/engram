---
phase: 04-curation-surfaces
plan: 07
subsystem: ui
tags: [svelte, virtual-list, keyboard, curation, honest-feedback, tdd]

# Dependency graph
requires:
  - phase: 04-curation-surfaces
    provides: "04-05's ResultsList selectable/selectedIds/onarchive/onrestore/onsupersede props and multi-select keyboard model; 04-06's curation/host.svelte.ts (CurationAction, defaultActionsFor)"
provides:
  - "RowActions.svelte: the hover/focus row action toolbar (D-05) — Supersede/Archive-or-Restore/Chain, gated per action on the caller's callback being supplied, anchored to the pointer-hovered row or the keyboard-active row while the list has focus"
  - "ResultsList: rowTrailing/groupKey/groupHeader props (D-11/D-12/D-13) — a route-supplied trailing cell that replaces score/rel, and scope group headers rendered as role=\"presentation\" items the keyboard model skips entirely"
  - "ResultRow: trailing?: Snippet<[Memory]> prop replacing the score/rel cells with the caller's own content, grid track count matching the no-rel layout"
  - "recall-header.ts: rulesHeaderParts/rulesEmptyHeading/scheduledHeaderParts/scheduledEmptyHeading — the honest-feedback header/empty formatters for /rules and /scheduled, reusing the same verbatim scopes_truncated/scopes_unknown clauses as rankedHeaderParts"
  - "Legend gating: the e/s/# hints now render only when onedit/onvisibility/ondelete are supplied, matching the existing curation-key gating pattern, so /rules and /scheduled never advertise a key that does nothing"
affects: [04-08, 04-09, 04-10]

# Actuals (#2632)
actuals:
  tokens: 13459
  tasks: 3
  commits: 5
  plan_head_before: 2b6d0e98e8ac5390361d1e744ac5efdf48634efa
  plan_head_after: abeae4f74f052c821ded99e8c7d64a517b94677f

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "RowActions mirrors ResultHoverCard's overlap-grace-period lifecycle: it is a SIBLING overlay painted on top of its anchored row (not a DOM descendant), so a real pointer move from the row onto the toolbar's own buttons fires the row's native mouseleave — a short (120ms) grace timer, cancelled the instant the pointer actually lands on the toolbar (via a bindable toolbarRef), keeps the toolbar mounted through that transition instead of tearing down mid-click"
    - "RowActions computes its own position (`top`, relative to ResultsList's position:relative `.results-list-container`) from anchor/container getBoundingClientRect rather than pulling in a floating-ui/popover library — it only ever needs vertical centering against an anchor that shares the container's right edge, not full collision-aware placement"
    - "ResultsList's `items` derived array interleaves `{kind:'header'}`/`{kind:'row', rowIndex}` entries for the virtual list rather than forking a grouped variant of the component; activeId/activeIndex stay derived over the ungrouped `memories` array and `moveActive` maps a row index to its item-array position only at the point it calls `list.scroll`"
    - "SCOPES_TRUNCATED_CLAUSE/SCOPES_UNKNOWN_CLAUSE extracted as module-level constants in recall-header.ts so rulesHeaderParts reuses rankedHeaderParts' exact coverage-clause text byte for byte, with rankedHeaderParts' own output unchanged"

key-files:
  created:
    - ui/src/lib/components/RowActions.svelte
  modified:
    - ui/src/lib/components/ResultsList.svelte
    - ui/src/lib/components/ResultsList.browser.test.ts
    - ui/src/lib/components/ResultRow.svelte
    - ui/src/lib/components/ResultRow.browser.test.ts
    - ui/src/routes/search/search.browser.test.ts
    - ui/src/lib/search/recall-header.ts
    - ui/src/lib/search/recall-header.test.ts

key-decisions:
  - "The toolbar's `pointerRowId` tracking is deliberately SEPARATE from the hover-card's existing `hoverRowId` (both fed by the same row mousemove handler) — the toolbar reveals instantly with no open delay, and needs its own scroll/text-size suppression flag and its own overlap grace-period timer, independent of the card's 250ms-delayed, 120ms-grace lifecycle."
  - "`scheduledHeaderParts`'s scroll-for-more clause and populated-header shape follow the plan's own flagged assumption (not in the UI-SPEC verbatim): a researcher default matching the listing-header pattern, since the UI-SPEC only specifies the Scheduled empty-state copy explicitly."

requirements-completed: []

coverage:
  - id: D1
    description: "Task 1 tracer: on /search, hovering a row reveals its action toolbar; clicking Archive opens ArchiveConfirmDialog for exactly that row, without opening the detail pane or moving aria-activedescendant"
    requirement: "CUR-02"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — row toolbar archive (D-05 tracer) > hovering the m1 row and clicking its toolbar Archive button opens the confirm dialog for exactly that row"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultsList.browser.test.ts (describe 'ResultsList — row action toolbar (D-05)', 4 cases incl. a DSYS-04 screenshot)"
        status: pass
    human_judgment: false
  - id: D2
    description: "ResultRow's trailing snippet (replaces score/rel, matching grid-track count at every container width) and ResultsList's rowTrailing/groupKey/groupHeader (D-11/D-12/D-13), plus the e/s/# legend gating"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultRow.browser.test.ts (describe 'trailing (D-11)', 2 cases)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultsList.browser.test.ts (describes 'ResultsList — rowTrailing (D-11)' and 'ResultsList — scope group headers (D-12)', 7 cases)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultsList.browser.test.ts (updated legend tests plus the new ondelete-only gating case)"
        status: pass
    human_judgment: false
  - id: D3
    description: "recall-header.ts's four new formatters (rulesHeaderParts, rulesEmptyHeading, scheduledHeaderParts, scheduledEmptyHeading), with rankedHeaderParts' own output unchanged after the shared-constant extraction"
    requirement: "CUR-03"
    verification:
      - kind: unit
        ref: "ui/src/lib/search/recall-header.test.ts (describes 'rulesHeaderParts (D-12)', 'rulesEmptyHeading (D-12)', 'scheduledHeaderParts (D-13)', 'scheduledEmptyHeading (D-13)', 9 cases)"
        status: pass
      - kind: unit
        ref: "ui/src/lib/search/recall-header.test.ts (all pre-existing rankedHeaderParts/listingHeaderParts cases, unaffected)"
        status: pass
    human_judgment: false

# Metrics
duration: ~50min
completed: 2026-09-27
status: complete
---

# Phase 4 Plan 7: Row action toolbar, trailing cells, group headers, Rules/Scheduled copy Summary

**Ships the generic list capabilities /rules, /scheduled and /search's supersede/chain wiring build on: a D-05 hover/focus row action toolbar (Supersede/Archive-or-Restore/Chain), a route-supplied trailing cell and scope group headers on the shared `ResultsList`/`ResultRow`, and the honest-feedback header/empty formatters for Rules and Scheduled in `recall-header.ts`.**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-09-27
- **Completed:** 2026-09-27
- **Tasks:** 3 of 3 completed (Task 1 tracer, Task 2 TDD, Task 3 TDD)
- **Files modified:** 8 (1 created, 7 modified)

## Accomplishments

- New `ui/src/lib/components/RowActions.svelte`: the D-05 row action toolbar — `role="toolbar"` with `aria-label="Row actions for {short_id}"`, absolutely positioned at the anchored row's right edge (computed from the anchor's and `ResultsList`'s own `.results-list-container` bounding rects), rendering Supersede (`replace` icon), Archive-or-Restore (`archive`/`archive-restore` icon), and Chain (`link-2` icon) buttons — each only when the row's `actions` list includes it AND the matching callback is supplied. A click never mutates `activeId` or opens the pane.
- `ResultsList.svelte`: `onchain?`/`rowActions?` (default `defaultActionsFor`) props feed the toolbar, mounted once as a sibling of the listbox wrapper — never inside `renderItem`'s `role="option"` row. The toolbar anchors to the pointer-hovered row instantly, or to the keyboard-active row while the list has focus, and hides on scroll/`engram:textsize`/empty `memories`. A dedicated 120ms overlap grace period (mirroring `ResultHoverCard`'s own lifecycle) keeps the toolbar mounted through the moment a real pointer crosses from the row onto the toolbar's own buttons — the row's native `mouseleave` fires there since the toolbar is painted on top but is not a DOM descendant.
- `ResultsList.svelte`/`ResultRow.svelte`: `rowTrailing?: Snippet<[Memory]>` (passed to each row as `trailing`) replaces the score/rel cells with the caller's own content, matching the no-rel grid-track count at every container width. `groupKey?: (m) => string`/`groupHeader?: Snippet<[string, number]>` build a derived `items` array interleaving `role="presentation"` group headers (default content: key + count) before each group's first row; `j`/`k`/`Home`/`End` and `aria-activedescendant` only ever see rows, never a header, and a single-row group still gets its own header.
- Legend: the `e`/`s`/`#` hints now render only when `onedit`/`onvisibility`/`ondelete` are supplied (matching the gating already applied to the curation keys), so a route like `/rules` or `/scheduled` that supplies only some of the row callbacks never advertises a key that does nothing.
- `ui/src/lib/search/recall-header.ts`: `rulesHeaderParts`/`rulesEmptyHeading` and `scheduledHeaderParts`/`scheduledEmptyHeading` added next to `listingHeaderParts`, reusing `plural` and two newly extracted `SCOPES_TRUNCATED_CLAUSE`/`SCOPES_UNKNOWN_CLAUSE` constants (also now used by `rankedHeaderParts`, whose own output is unchanged). The Scheduled `all` tab reads "windowed" everywhere via one shared state-word map.

## Task Commits

Each task was committed atomically (Tasks 2 and 3 ran under TDD: test → feat):

1. **Task 1 (tracer): row action toolbar end to end** — `f1a7249a` (feat)
2. **Task 2: trailing cells and group headers** — `cab163db` (test, RED) → `6bebe63f` (feat, GREEN)
3. **Task 3: Rules and Scheduled header/empty copy** — `34be01bc` (test, RED) → `abeae4f7` (feat, GREEN)

**Plan metadata:** this commit (docs: complete plan)

_Note: no REFACTOR commit was needed for either TDD task — each GREEN implementation required no follow-up cleanup._

## Files Created/Modified

- `ui/src/lib/components/RowActions.svelte` — the D-05 hover/focus row action toolbar
- `ui/src/lib/components/ResultsList.svelte` — toolbar mount + overlap-grace lifecycle, rowTrailing/groupKey/groupHeader, e/s/# legend gating
- `ui/src/lib/components/ResultsList.browser.test.ts` — toolbar, rowTrailing, group-header, and updated legend coverage
- `ui/src/lib/components/ResultRow.svelte` — `trailing` prop
- `ui/src/lib/components/ResultRow.browser.test.ts` — trailing-cell coverage
- `ui/src/routes/search/search.browser.test.ts` — the D-05 row-toolbar-archive tracer test
- `ui/src/lib/search/recall-header.ts` — the four new Rules/Scheduled formatters, shared coverage-clause constants
- `ui/src/lib/search/recall-header.test.ts` — coverage for all four new formatters

## Decisions Made

See `key-decisions` in the frontmatter for the full rationale. Summary: the toolbar's own `pointerRowId`/suppression/grace-timer state is kept independent of the hover card's existing `hoverRowId`/timers (different reveal timing, different lifecycle needs); the Scheduled header's populated-state copy and scroll-for-more clause follow the plan's own flagged researcher-default assumption since the UI-SPEC only specifies the empty-state copy verbatim.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The row toolbar tore itself down mid-click when the real pointer moved from the anchored row onto the toolbar's own buttons**
- **Found during:** Task 1, writing the ResultsList.browser.test.ts click-through tests
- **Issue:** `RowActions` is a sibling overlay painted on top of its anchored row (not a DOM descendant of the `role="option"` row). A real pointer move onto one of its buttons fires the underlying row's native `mouseleave` (browsers resolve hover by paint order, not DOM ancestry). The initial implementation cleared `pointerRowId` immediately on that `mouseleave`, unmounting the toolbar out from under Playwright's in-flight click and causing "element was detached from the DOM, retrying" timeouts.
- **Fix:** Mirrored `ResultHoverCard`'s own overlap-grace-period lifecycle: `handleRowMouseLeave` now schedules a 120ms delayed clear (`scheduleToolbarClose`) instead of clearing immediately, and a new bindable `toolbarRef` lets `ResultsList` attach `mousemove`/`mouseleave` listeners directly to the toolbar element, cancelling the scheduled clear the instant the pointer actually lands there.
- **Files modified:** `ui/src/lib/components/RowActions.svelte`, `ui/src/lib/components/ResultsList.svelte`
- **Verification:** All row-toolbar click tests (in both `ResultsList.browser.test.ts` and `search.browser.test.ts`) pass reliably; full regression stayed green.
- **Committed in:** `f1a7249a` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — a genuine interaction bug found while writing the tracer's own test coverage, not a plan gap). **Impact:** Necessary for the toolbar to be clickable at all under real pointer interaction; no scope creep beyond making Task 1's own specified behavior actually work.

## Issues Encountered

None beyond the deviation documented above.

## Known Stubs

None. `onchain` is accepted as a prop on both `RowActions` and `ResultsList` but no route in this plan passes it — this is explicitly out of this plan's scope (the Chain dialog and its host wiring are a later plan's responsibility) and produces no dangling UI: the Chain button simply does not render until a route supplies `onchain`, by design (tested for both the no-`onchain` and non-chained-row cases).

## Threat Flags

None beyond the plan's own `<threat_model>` (T-04-17), mitigated as specified: the toolbar's overlap-grace-period fix (see Deviations above) is itself part of what keeps the anchored row's id captured at render and the toolbar hidden on scroll/text-size change, so it never floats over a different row; every write still opens a confirm dialog listing the target chip.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `RowActions.svelte`, `ResultsList`'s `rowTrailing`/`groupKey`/`groupHeader` props, and `recall-header.ts`'s four new Rules/Scheduled formatters are all ready for plans 04-09 (`/rules`) and 04-10 (`/scheduled`) to consume without touching the shared list components again.
- `onsupersede`/`onchain` wiring on `/search` itself remains plan 04-08's responsibility, as before this plan.
- `requirements-completed` is empty here by design: CUR-02/CUR-03/CUR-04 are each shared with sibling plans (04-01/04-05/04-12 for CUR-02, 04-09 for CUR-03, 04-10 for CUR-04) whose `*-SUMMARY.md` files are not yet visible in this worktree — `requirements.ready-ids` reported 0/3 ready. They will be marked complete once every plan declaring them has finished.
- No blockers.

---
*Phase: 04-curation-surfaces*
*Completed: 2026-09-27*

## Self-Check: PASSED

- All 8 key-files (1 created, 7 modified) verified present on disk with `ls -la`.
- All 5 commits (`f1a7249a`, `cab163db`, `6bebe63f`, `34be01bc`, `abeae4f7`) verified present in `git log --oneline --all`.
- Re-ran all three tasks' plan-level `<verify>` commands: browser project (`ResultsList.browser.test.ts` + `search.browser.test.ts`, 64/64; then + `ResultRow.browser.test.ts`, 71/71), node project (`recall-header.test.ts`, 34/34), `pnpm --dir ui build` exits 0 (confirmed after each task).
- Full regression: `pnpm --dir ui vitest run --project browser` (31 files / 385 tests) and `--project node` (23 files / 272 tests) both green after the final commit.
- Re-ran all 3 acceptance-criteria `rg`/`awk` checks across the three tasks — all matched their required counts (Task 1: `<RowActions` count 1/0-inside-renderItem, `row toolbar archive` count 1; Task 2: `results-group-header` count 1, `trailing[?]: Snippet` count 1; Task 3: 4 unique exported formatter names).
