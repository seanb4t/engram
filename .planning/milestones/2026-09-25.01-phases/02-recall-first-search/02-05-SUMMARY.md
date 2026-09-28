---
phase: 02-recall-first-search
plan: 05
subsystem: ui
tags: [svelte5, bits-ui, command-menu, keyboard-shortcuts]

requires:
  - phase: 02-recall-first-search
    provides: "classifyInput, handoffToHeaderSearch/headerSearch, defaultSearchParams/encodeSearchParams (plan 02-04)"
provides:
  - "ui/src/lib/components/CommandMenu.svelte: the ⌘K command menu -- navigation, display/theme, copy-id, always ending in an unfiltered hand-off row to the header search"
affects: [02-06, 02-07, 02-08]

actuals:
  tokens: 5524
  tasks: 2
  commits: 2
  plan_head_before: 6ab8f2a7917e4aae9d60805b476f9cdefedcde9c

tech-stack:
  added: []
  patterns:
    - "Command.Dialog with shouldFilter={false} and app-owned substring filtering (matchesQuery over each item's own label), not the Command primitive's built-in fuzzy scoring -- discovered live that bits-ui 2.18.1's default shouldFilter=true drives a DOM-reparenting sort pass (CommandRootState#sort's raw appendChild calls) that silently corrupts Svelte 5's own reactive DOM tracking once a forceMount group is present, collapsing the whole Dialog.Content to empty comment nodes on the first keystroke"
    - "Command.Dialog portals its content to document.body (bits-ui default, unlike HeaderSearch's inline Popover with portalProps={{ disabled: true }}) -- browser tests must query document.body, not vitest-browser-svelte's own screen.container, for DOM-order or full-page-text assertions"

key-files:
  created:
    - ui/src/lib/components/CommandMenu.svelte
    - ui/src/lib/components/CommandMenu.browser.test.ts
  modified:
    - ui/src/routes/+layout.svelte
  deleted:
    - ui/src/lib/components/CommandPalette.svelte
    - ui/src/lib/components/CommandPalette.browser.test.ts

key-decisions:
  - "shouldFilter={false} + app-owned matchesQuery substring filtering, not the Command primitive's own fuzzy scoring -- found live during Task 1: the default shouldFilter=true triggers a DOM-node-reparenting sort pass on every keystroke that fights Svelte 5's reactive DOM ownership once a forceMount group exists, silently emptying Dialog.Content with zero console errors. Reproduced with a bare Dialog.Root + a single uncontrolled <input> (no Command involved at all), confirming this is a Command/Dialog+typing interaction, not a bug in this plan's own logic."
  - "Display and Record groups are rendered from filtered arrays (visibleDisplayItems/visibleRecordItems), exactly like the Go-to group's visibleNavItems, so 'these are static items filtered client-side' (plan's own framing) applies uniformly across every group, not just navigation."
  - "Record group's items are derived reactively from page.url.searchParams.get('sel') and the TanStack query cache key ['getMemory', sel] that MemoryDetail/DetailPane already populate -- no new query, no new RPC."

requirements-completed: [ENTRY-01, ENTRY-04]

coverage:
  - id: D1
    description: "⌘K opens a command menu; the last option is always an unfiltered hand-off row reading 'Search memories for \"<text>\" ↵' (or 'Open record <id>…' for a pasted UUID/short_id) that hands off to the header search or /search, and the menu never shows 'no matches' or calls SearchMemories/GetMemory"
    requirement: "ENTRY-04"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#shows the unfiltered hand-off row last for a memory term matching no static label, and never \"no matches\""
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#shows every navigation item and no hand-off row for an empty input, and makes no RPC"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#filters navigation items by the typed term and still ends with the hand-off row"
        status: pass
    human_judgment: false
  - id: D2
    description: "A UUID or short_id pasted into ⌘K resolves through the shared classifier and opens /search?q=<id> on selection, never calling SearchMemories/GetMemory itself"
    requirement: "ENTRY-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#resolves a pasted UUID to an \"Open record\" row and navigates to /search?q=<id>"
        status: pass
    human_judgment: false
  - id: D3
    description: "CommandPalette.svelte and its test are deleted; +layout.svelte mounts CommandMenu on ⌘K instead"
    verification:
      - kind: other
        ref: "rg -l CommandPalette ui/src (0 matches) + pnpm --dir ui build exits 0"
        status: pass
    human_judgment: false
  - id: D4
    description: "Display group: Larger/Smaller text and Reset text size call stepTextSize/resetTextSize and toast the new size; Toggle theme calls mode-watcher's setMode with the opposite mode; all filtered client-side like every other group"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#steps the text size up on \"Larger text\""
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#returns the text size to the default on \"Reset text size\""
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#toggles the theme to the opposite of the current mode"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#filters to the Display items for a typed term and still ends with the hand-off row"
        status: pass
    human_judgment: false
  - id: D5
    description: "Record group (copy full id / copy short_id) appears only when ?sel= names a selected record and the query cache holds its shortId; copies write to the clipboard with a copied/copy-failed toast; absent with no ?sel="
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#offers copy-id commands when a record is selected via ?sel=, and copies the short_id"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#copies the full id when \"Copy full id\" is selected"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/CommandMenu.browser.test.ts#shows no Record group when there is no ?sel="
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 5: ⌘K Command Menu — Navigation, Display/Theme, Copy-Id, and an Unfiltered Search Hand-off Summary

**Split ⌘K from search (D-11): the lying CommandPalette (a client-filtered "Search memories for…" item that quietly disappeared from its own results list) is deleted, replaced by a static command menu whose forceMount hand-off row can never claim "no matches" for a memory term because it never runs a filter over memory content at all.**

## Performance

- **Duration:** 35 min
- **Started:** 2026-09-26T21:30:00Z
- **Completed:** 2026-09-26T22:05:12Z
- **Tasks:** 2 (1 tracer, 1 auto/tdd)
- **Files modified:** 5 (2 created, 1 modified, 2 deleted)

## Accomplishments

- `ui/src/lib/components/CommandMenu.svelte`: a `Command.Dialog` with `shouldFilter={false}` and app-owned substring filtering (`matchesQuery`) over every group's own item labels -- "Go to" (Home/Observe/Search/Discovery), "Display" (text size, theme), and "Record" (copy id/short_id, only when a record is selected). The final group is a `forceMount` `Command.Group`/`Command.Item` pair whose label is derived reactively from the live input (`classifyInput(q)`), never snapshotted -- the exact bug class that made the deleted palette lie.
- The hand-off row hands typed text to the header search (`handoffToHeaderSearch`) for free text, or opens `/search?q=<id>` directly for a pasted UUID/short_id -- through the same shared classifier and URL codec the header search and `/search` itself use (`$lib/search/classify`, `$lib/search/params`).
- `+layout.svelte` mounts `CommandMenu` on ⌘K in place of the deleted `CommandPalette`.
- Display group items call the same `stepTextSize`/`resetTextSize` functions and toast wording as the console-wide ⌘+/⌘-/⌘0 shortcuts (D-13); Toggle theme calls `mode-watcher`'s `setMode`.
- Record group reads the currently-selected record's id/short_id from `page.url.searchParams.get('sel')` and the `['getMemory', sel]` TanStack query-cache entry `MemoryDetail`/`DetailPane` already populate -- no new RPC, no new query.
- 12 browser tests cover every truth in the plan's `must_haves`: empty state, hand-off row presence/absence, per-group filtering, UUID resolution, Display actions, and Record-group presence/absence/copy behavior.

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): ⌘K command menu end to end -- navigation, the forceMount hand-off row, mounted in the layout; CommandPalette deleted** - `bf593958` (feat)
2. **Task 2 (tdd): Theme, text size and copy-current-record commands** - `eb9fe494` (feat)

**Plan metadata:** commit follows this SUMMARY.

## Files Created/Modified

- `ui/src/lib/components/CommandMenu.svelte` -- the ⌘K command menu
- `ui/src/lib/components/CommandMenu.browser.test.ts` -- 12 browser tests
- `ui/src/routes/+layout.svelte` -- mounts `CommandMenu` instead of `CommandPalette`
- `ui/src/lib/components/CommandPalette.svelte` / `CommandPalette.browser.test.ts` -- deleted

## Decisions Made

- **`shouldFilter={false}` with app-owned `matchesQuery` substring filtering**, not the Command primitive's own fuzzy scoring. Found live while writing Task 1's typing tests: bits-ui 2.18.1's default `shouldFilter=true` runs a DOM-node-reparenting sort pass (`CommandRootState#sort`'s raw `appendChild` calls) on every keystroke to reorder items by score. Once a `forceMount` group exists, this collided with Svelte 5's own fine-grained DOM ownership and silently collapsed the entire `Dialog.Content` to empty comment nodes -- with zero console errors, zero thrown exceptions, and `document.body` still reporting the Dialog as `open`. Isolated to a bare `Dialog.Root` + a single **uncontrolled** `<input>` with no `Command` primitive involved at all, confirming this is a `Dialog`+typing interaction in this exact bits-ui/Svelte version combination, not a bug in this plan's own reactive logic. `shouldFilter={false}` (matching `HeaderSearch`'s own precedent) disables the internal scoring/sort pass entirely; visibility is computed by the template's own `{#each visibleXItems}` derivations instead.
- **Display and Record groups use the same filtered-array pattern as the Go-to group** (`visibleDisplayItems`/`visibleRecordItems`), so "these are static items filtered client-side against their labels" (the plan's own framing) holds uniformly, not just for navigation.
- **Record group reads the existing `['getMemory', sel]` query-cache entry** rather than issuing its own fetch -- `MemoryDetail`/`DetailPane`/the `/search` and `/discovery` routes already populate this key when a record is selected via `?sel=`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected a stale per-plan commit ledger sentinel**
- **Found during:** computing this SUMMARY's `actuals.commits`
- **Issue:** `.git/gsd-plan-head-before-02-05` already existed on disk (a leftover from an earlier milestone's own same-numbered "02-05" plan, per the recurring gotcha this project's STATE.md already documents for plans 01-03/01-04/03-05) pointing at a commit from months earlier in this repo's history. The ledger-creation guard (`[ -f "$_GSD_LEDGER" ] || ...`) correctly refused to overwrite the pre-existing file, so `git rev-list --count` against it returned 115 -- every commit since the milestone started, not this plan's own 2.
- **Fix:** Verified the true pre-Task-1 HEAD via `bf593958^` (`6ab8f2a7917e4aae9d60805b476f9cdefedcde9c`, the exact commit the session's initial `git status` context named as `HEAD`) and overwrote the sentinel with that value before computing `actuals.commits`.
- **Files modified:** `.git/gsd-plan-head-before-02-05` (not a tracked repo file)
- **Verification:** `git rev-list --count 6ab8f2a7917e4aae9d60805b476f9cdefedcde9c..HEAD` now reports `2`, matching the two task commits.
- **Committed in:** n/a (ledger sentinel lives outside the working tree)

---

**Total deviations:** 1 auto-fixed (1 bug -- a stale cross-milestone artifact, not a defect in this plan's own commits).
**Impact on plan:** No scope creep; only affects the SUMMARY's own `actuals.commits` accuracy.

## Issues Encountered

- Extensive live debugging was required to isolate the `shouldFilter`/Dialog-typing interaction documented above as a key decision -- confirmed via a minimal reproduction (bare `Dialog.Root` + uncontrolled `<input>`) before concluding it was a Command/Dialog-level behavior rather than a bug in this plan's own component code.
- A related, second false lead during the same investigation: `Command.Dialog` portals its content to `document.body` by default (unlike `HeaderSearch`'s inline `Popover.Content` with `portalProps={{ disabled: true }}`), so an early debug script querying `screen.container` (vitest-browser-svelte's own render-target div) found nothing and looked like a second bug. Confirmed via a direct `document.body.innerHTML` check that the content was present and correct all along, just portaled outside the queried subtree. `CommandMenu.browser.test.ts`'s two DOM-order assertions query `document.body` directly, with an inline comment explaining why.

## User Setup Required

None -- no external service configuration required.

## Next Phase Readiness

- ⌘K's command menu is complete and live in `+layout.svelte`; the lying `CommandPalette` is fully removed (`rg -l CommandPalette ui/src` reports zero matches).
- The `shouldFilter={false}` + app-owned-filtering pattern (and the `Command.Dialog` portal-target gotcha) are now documented here for any future `Command.Dialog` consumer in this codebase.
- No blockers for plans 02-06 through 02-08.

---
*Phase: 02-recall-first-search*
*Completed: 2026-09-26*

## Self-Check: PASSED

- Created files exist: `ui/src/lib/components/CommandMenu.svelte`, `ui/src/lib/components/CommandMenu.browser.test.ts` -- both found.
- Deleted files confirmed absent: `ui/src/lib/components/CommandPalette.svelte`, `ui/src/lib/components/CommandPalette.browser.test.ts` -- both gone; `rg -l CommandPalette ui/src` reports 0 matches.
- Commits exist: `bf593958` (Task 1), `eb9fe494` (Task 2) -- both found in `git log --oneline --all`.
- All plan `<acceptance_criteria>` re-run and passing: `test ! -e CommandPalette.svelte` exits 0; `Command.Empty` count 0; `forceMount` count 3 (>= 2); `engram.` count 0; `stepTextSize(|resetTextSize(` count 3 (>= 3); `navigator.clipboard.writeText` count 1 (>= 1).
- Plan-level `<verification>` re-run: `pnpm --dir ui vitest run --project browser src/lib/components/CommandMenu.browser.test.ts` (12/12 pass); `pnpm --dir ui build` exits 0; `rg -l CommandPalette ui/src` empty.
- Full `pnpm --dir ui vitest run` (43 files / 429 tests) and `pnpm --dir ui build` both clean; `npx tsc --noEmit` shows only the pre-existing, unrelated badge/button/tabs module-resolution errors documented in plan 02-04's SUMMARY.
