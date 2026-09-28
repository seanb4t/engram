---
phase: 05-related-memories-graph-tag-cloud
plan: 03
subsystem: ui
tags: [svelte, tanstack-query, connect-web, bits-ui, tags]

# Dependency graph
requires:
  - phase: 03-curation-rpcs-mcp-tools
    provides: ListTags Connect RPC (RPC-04) and its per-RPC client contract
provides:
  - "ui/src/lib/tags/query.ts: listTagsKey/listTagsQuery, one cached ListTags(scope, 1000) query per scope key"
  - "ui/src/lib/tags/tags.ts: pure tag slicing, ranking and honesty-copy helpers (toTagRows, barPercent, visibleTagRows, tagListFooter, scopeLabel, tagsLoadingLine, tagsErrorCopy, rankTagMatches, matchFooter)"
  - "TagBars.svelte: the shared tag popularity list (TAGS-01, D-12, D-13) -- top-30/show-all, filter box, honesty footer, tooltip, markers, keyboard, every E4 state"
  - "TagMatchRow.svelte + TagCombobox.svelte: the '+ tag' picker with counts and an honest unknown-tag row (TAGS-02, D-16, D-18, D-19), every E5 state"
affects: [05-06 header Tags group, 05-07 /search Tags panel and picker, 05-08 /related rail Tags tab]

# Actuals (#2632) -- pairs with the plan's `estimate` to calibrate future estimates.
actuals:
  tokens: 13464
  tasks: 3
  commits: 6
  plan_head_before: 8be24a8c63517505b6bff8b7813cf3dbf934d081
  plan_head_after: d9e843ca851180c75a319530667a5771b5a2f9f9

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure-function tag logic (ranking, top-30 slicing, honesty-footer copy) lives entirely in tags.ts, unit-tested in node -- Svelte components only render its output, never re-derive it"
    - "One cached ListTags(scope, 1000) TanStack query per scope key (staleTime Infinity, meta.silent) shared verbatim by TagBars and TagCombobox -- proven by a same-scope second-mount test asserting the RPC fires exactly once"

key-files:
  created:
    - ui/src/lib/tags/query.ts
    - ui/src/lib/tags/tags.ts
    - ui/src/lib/tags/tags.test.ts
    - ui/src/lib/components/TagBars.svelte
    - ui/src/lib/components/TagBars.browser.test.ts
    - ui/src/lib/components/TagMatchRow.svelte
    - ui/src/lib/components/TagCombobox.svelte
    - ui/src/lib/components/TagCombobox.browser.test.ts
  modified: []

key-decisions:
  - "Wrote the ● marked-tag indicator as a real conditional DOM text node (a <span class=\"marker\">), not a CSS ::before -- textContent-based assertions (and any real screen reader) need an actual node; CSS generated content is invisible to both."
  - "TagCombobox's popover content is the default (non-disabled) bits-ui portal per 05-CONTEXT.md's own flagged assumption -- its Command.Root sits wholly inside Popover.Content, so the item registry moves with the portal; tests query document.querySelectorAll/getByText (portaled to document.body) rather than screen.container."
  - "gsd_run check tdd-red-evidence misclassified both genuine RED commits as INVALID_RED/zero_tests_discovered: vitest's tap and tap-flat reporters never emit node:test's own '# tests/# pass/# fail' summary lines the checker's TAP parser requires, even though the same record's own failing_tests list correctly named every target assertion. Documented as a vitest/TAP-dialect gap in the checker (parallel to tdd.md's disclosed Rust gap) rather than worked around silently; RED was verified by hand (TypeError: not a function / genuine assertion mismatches, no fixture/syntax crash) before proceeding to GREEN both times."

requirements-completed: [TAGS-01, TAGS-02]

coverage:
  - id: D1
    description: "TagBars draws a scope's ListTags counts as linear bars from one cached query per scope key, top-30/show-all, filter box, honesty footer (never \"of N\"), popularity/rarity tooltip, marked/selected rows, roving-listbox keyboard model, and every E4 state (empty/loading/error/populated/partial/overflow)"
    requirement: TAGS-01
    verification:
      - kind: unit
        ref: "ui/src/lib/tags/tags.test.ts#visibleTagRows, tagListFooter, barPercent"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/TagBars.browser.test.ts (18 cases incl. light/dark screenshots)"
        status: pass
    human_judgment: false
  - id: D2
    description: "TagCombobox ('+ tag' picker) and TagMatchRow rank substring matches (prefix-first, then count desc/tag asc), cap at 8 with an honest match-count footer, and always offer an honest 'Add #{tag}' row for a typed tag that is not itself loaded -- never silently blocked, never implying the tag exists"
    requirement: TAGS-02
    verification:
      - kind: unit
        ref: "ui/src/lib/tags/tags.test.ts#rankTagMatches, matchFooter"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/TagCombobox.browser.test.ts (8 cases incl. 1-vs-8-match and 128-byte backstops, light/dark screenshots)"
        status: pass
    human_judgment: false

# Metrics
duration: 22min
completed: 2026-09-28
status: complete
---

# Phase 5 Plan 3: Tag Bars and Tag Picker Summary

**Shared, host-independent TagBars (linear popularity bars) and TagCombobox (+ tag picker with counts) built once over one cached `ListTags(scope, 1000)` TanStack query per scope key, with all ranking/honesty logic factored into a pure, unit-tested `tags.ts` module.**

## Performance

- **Duration:** 22 min
- **Started:** 2026-09-28T03:10:26Z
- **Completed:** 2026-09-28T03:32:51Z
- **Tasks:** 3
- **Files modified:** 8 (all new)

## Accomplishments

- `ui/src/lib/tags/query.ts` + `tags.ts`: one cached `ListTags(scope, 1000)` query per scope key (`staleTime: Infinity`), and every pure helper (row projection, bar-width math, top-30 slicing, honesty-footer copy, match ranking, footer copy) is unit-tested in node with zero DOM/network dependency.
- `TagBars.svelte` (TAGS-01, D-12, D-13, D-15): the shared tag popularity list -- top-30 with "show all", a filter box, a footer that never claims a total ListTags cannot back, a popularity/rarity tooltip kept as two separate facts, marked/selected rows, and a roving `aria-activedescendant` keyboard model, covering every E4 UI consideration (empty/loading/error/populated/partial/overflow/zero-one-many/long-text).
- `TagMatchRow.svelte` + `TagCombobox.svelte` (TAGS-02, D-16, D-18, D-19): the "+ tag" picker mirroring `ScopeCombobox`'s shape -- substring matching ranked prefix-first then by count, capped at 8 with an honest footer, and an always-present "Add #{tag}" row naming why an unmatched tag is absent rather than pretending it exists, covering every E5 consideration including the two backstop rows (1-vs-8-match identical structure, 128-byte long-text no-overflow).
- Both components share the identical cached query for a given scope key -- proven by a same-client, same-scope second-mount test asserting `ListTags` fires exactly once.

## Task Commits

Each task was committed atomically, with TDD tasks 2 and 3 following RED (`test(...)`) then GREEN (`feat(...)`) commits per `tdd.md`:

1. **Task 1: One cached ListTags query per scope key, and TagBars drawing its rows as linear bars that toggle on click** (tracer) - `69dc6f53` (feat)
2. **Task 2: TagBars complete -- top 30 and show all, filter box, honesty footer, tooltip, markers, keyboard, every E4 state** (tdd) - `4ca2398b` (test, RED) -> `96e945f4` (feat, GREEN)
3. **Task 3: TagMatchRow and the "+ tag" TagCombobox -- ranking, top 8, footer, unknown-tag row, every E5 state** (tdd) - `2769ae3c` (test, RED) -> `e0710103` (feat, GREEN) -> `d9e843ca` (fix: two `tsc --noEmit` cast errors in TagBars tests, caught during post-implementation verification)

_Plan metadata commit follows this SUMMARY._

## Files Created/Modified

- `ui/src/lib/tags/query.ts` - `listTagsKey`/`listTagsQuery`: one cached `ListTags(scope, 1000)` query per scope key
- `ui/src/lib/tags/tags.ts` - pure tag slicing, ranking and honesty-copy helpers shared by both components
- `ui/src/lib/tags/tags.test.ts` - 21 node-run unit tests over every pure helper
- `ui/src/lib/components/TagBars.svelte` - the shared tag popularity list
- `ui/src/lib/components/TagBars.browser.test.ts` - 18 browser cases incl. light/dark screenshots
- `ui/src/lib/components/TagMatchRow.svelte` - one ranked match row (bolded hit parts, mini bar, count)
- `ui/src/lib/components/TagCombobox.svelte` - the "+ tag" picker
- `ui/src/lib/components/TagCombobox.browser.test.ts` - 8 browser cases incl. visual-state backstops and screenshots

## Decisions Made

See `key-decisions` in the frontmatter above (marker-as-DOM-node, portal targeting in tests, and the documented `tdd-red-evidence`/vitest gap).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TagBars' marked-tag ● indicator implemented as CSS `::before` content, invisible to `textContent` and screen readers**
- **Found during:** Task 2, writing the marker/selection browser test
- **Issue:** `content: '● '` via CSS is not a real DOM text node -- `element.textContent` never includes it, and it carries no accessible text either
- **Fix:** Render the marker as an actual conditional `<span class="marker">● </span>` text node ahead of the tag name
- **Files modified:** `ui/src/lib/components/TagBars.svelte`
- **Verification:** `TagBars.browser.test.ts` "marked rows carry the ● marker and .marked class" passes
- **Committed in:** `96e945f4` (part of Task 2's GREEN commit)

**2. [Rule 1 - Bug] TagCombobox browser tests queried `screen.container` for portaled popover content**
- **Found during:** Task 3, first TagCombobox browser test run
- **Issue:** The Popover's default (non-disabled) portal renders its content into `document.body`, outside the component's own mount container -- `screen.container.querySelectorAll('.opt')` always returned zero elements despite the picker rendering correctly (confirmed via failure screenshot)
- **Fix:** Query `document.querySelectorAll`/`document.querySelector` for portaled elements; `unmount()` the first instance before mounting a second in the same test to avoid cross-instance DOM contamination
- **Files modified:** `ui/src/lib/components/TagCombobox.browser.test.ts`
- **Verification:** All 8 `TagCombobox.browser.test.ts` cases pass
- **Committed in:** `e0710103` (part of Task 3's GREEN commit)

**3. [Rule 1 - Bug] Two `tsc --noEmit` type errors in TagBars.browser.test.ts**
- **Found during:** Post-implementation type-check pass (svelte-check is broken in this environment per the documented STATE.md gap; substituted `tsc --noEmit` per the same precedent)
- **Issue:** `expect.element(...)` received a raw `querySelectorAll(...)[0]` typed as the wide DOM `Element`, not assignable to the `HTMLElement | Locator | SVGElement` union the matcher expects
- **Fix:** Cast to `HTMLElement` at both call sites
- **Files modified:** `ui/src/lib/components/TagBars.browser.test.ts`
- **Verification:** `tsc --noEmit` no longer reports either error; full 26-case browser suite still green
- **Committed in:** `d9e843ca`

---

**Total deviations:** 3 auto-fixed (2 test-correctness bugs, 1 type-check bug). **Impact on plan:** All three were necessary corrections surfaced by the plan's own verification loop (browser assertions and the post-hoc type check); no scope creep, no behavior beyond what the plan specified.

## Issues Encountered

`gsd_run check tdd-red-evidence` could not classify either RED commit's evidence: vitest's `tap`/`tap-flat` reporters produce TAP13-compliant output but never emit `node --test`'s own `# tests N`/`# pass N`/`# fail N` summary lines, which the checker's `parseNodeTestSummary` strictly requires. Both times, the checker's own `failing_tests` array correctly named every target assertion as failing (via `not ok` line extraction, which does work against vitest's TAP output), but `summary.tests` stayed `0`, forcing verdict `INVALID_RED`/`zero_tests_discovered` regardless of the genuine failures underneath. This is a vitest/TAP-dialect gap in the checker, not an invalid RED -- confirmed by manually inspecting each RED run's failure reasons (`TypeError: X is not a function` for the not-yet-implemented helpers, or a real `AssertionError` for `TAG_TOP`), none of which is a fixture/syntax/load crash. Proceeded to GREEN on the manually-verified evidence both times, and recorded the gap here and as a `key-decision` for future TS/Svelte TDD plans in this repo.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `TagBars.svelte` and `TagCombobox.svelte` are ready to be mounted by their hosts: plan 05-06 (header search Tags group), 05-07 (`/search` docked Tags panel and its `TagCombobox`), and 05-08 (`/related` rail Tags tab, which will additionally pass `rarity` weights from tag edges).
- No blockers. `rankTagMatches`/`matchFooter`/`visibleTagRows`/`tagListFooter` are stable, tested contracts the downstream host plans can build directly against.

---
*Phase: 05-related-memories-graph-tag-cloud*
*Completed: 2026-09-28*
