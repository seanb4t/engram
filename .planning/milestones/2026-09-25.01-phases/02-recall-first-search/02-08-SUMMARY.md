---
phase: 02-recall-first-search
plan: 08
subsystem: ui
tags: [svelte5, tanstack-query, bits-ui, connect-web, recall-surface, facet-chips]

requires:
  - phase: 02-recall-first-search
    provides: "RecallGateHidden proto message + recall_gate_hidden fields on SearchMemoriesResponse/ListMemoriesResponse (plan 02-01)"
  - phase: 02-recall-first-search
    provides: "ui/src/lib/search/classify.ts, params.ts, header-search.svelte.ts, connect-error.ts (plan 02-04)"
  - phase: 02-recall-first-search
    provides: "ResultsList.svelte, ResultRow.svelte, ResultHoverCard.svelte (plan 02-06)"
  - phase: 02-recall-first-search
    provides: "DetailPane.svelte, RecallSplit.svelte, WriteSurfaces.requestMakePrivate (plan 02-07)"
provides:
  - "ui/src/lib/search/recall-header.ts: HiddenCounts, hiddenFromProto, hiddenTooltip, plural, rankedHeaderParts, resolutionLine, headerText, loadingLine — the one formatter every /search header clause comes from"
  - "ui/src/lib/components/ResultsHeader.svelte: the honest one-line results header with an expandable searched_scopes list and hidden-count tooltip"
  - "ui/src/lib/components/FacetStrip.svelte: URL-persisted category/scope/tag/state/created-window chips, no navigation of its own"
  - "ui/src/lib/components/ScopeCombobox.svelte: a ListScopes-backed scope picker (Popover + Command, manual substring filter)"
  - "the rebuilt ui/src/routes/search/+page.svelte: URL-driven, race-safe /search recall surface"
affects: [02-09, 02-10]

actuals:
  tokens: 17980
  tasks: 3
  commits: 3
  plan_head_before: 5d748bbd9ab445b3298403485ce944d8ac199e8c

tech-stack:
  added: []
  patterns:
    - "WriteSurfaces (and any component the page keeps a bind:this reference to across the whole page lifetime) must live in a location RecallSplit's own {#if isNarrow}/{:else} branch switch cannot tear down — both branches independently {@render list()}, so Svelte destroys and recreates a snippet's content across that switch, invalidating any bind:this captured inside it the first time the ResizeObserver's post-mount width measurement crosses the narrow breakpoint"
    - "Facet-strip state (scope/tags/categories, persisted directly as URL params) is folded into classify.ts's chip vocabulary as synthesized OperatorChip objects and placed AHEAD of the query text's own inline operator chips before calling applyChips — applyChips derives tags/categories from the chip set alone (discarding whatever the base params carried), so without this the query box's own inline scope:/#tag/is: parsing would silently wipe out FacetStrip's selections on every keystroke of plain text"
    - "bits-ui Command's own aria-labelledby (an internal, auto-generated combobox label) always outranks a consumer-supplied aria-label on Command.Input for accessible-name computation — pass label to Command.Root instead of aria-label on Command.Input, or target the input by role=combobox in tests"
    - "ScopeCombobox uses shouldFilter={false} plus manual substring filtering over the complete, already-loaded ListScopes list, not bits-ui's own default filter — deliberately sidesteps the bits-ui 2.18.1/Svelte 5 default-filter content-emptying gotcha this milestone already hit once in plan 02-05, even though the whole list being loaded upfront would otherwise make the default filter honest too"

key-files:
  created:
    - ui/src/lib/search/recall-header.ts
    - ui/src/lib/search/recall-header.test.ts
    - ui/src/lib/components/ResultsHeader.svelte
    - ui/src/lib/components/ResultsHeader.browser.test.ts
    - ui/src/lib/components/FacetStrip.svelte
    - ui/src/lib/components/FacetStrip.browser.test.ts
    - ui/src/lib/components/ScopeCombobox.svelte
    - ui/src/lib/components/ScopeCombobox.browser.test.ts
  modified:
    - ui/src/routes/search/+page.svelte
    - ui/src/routes/search/search.browser.test.ts

key-decisions:
  - "Facet chips (URL-persisted scope/tags/categories) and the query box's own inline operator chips are combined by synthesizing the facet state as OperatorChip objects and concatenating them ahead of the inline chips before calling params.ts's applyChips — applyChips itself was not modified (out of this plan's files_modified list); this composition lives entirely in +page.svelte."
  - "WriteSurfaces (and its bind:this) moved to a stable toolbar row outside RecallSplit, not inside its `list` snippet — found live while writing the resume-recovery tests: RecallSplit's narrow/wide layout switch recreates snippet content across its {#if}/{:else} branches, which was silently destroying the WriteSurfaces instance the onMount resume-restore call was targeting."
  - "ScopeCombobox filters the loaded ListScopes list manually (shouldFilter={false}) rather than relying on bits-ui Command's own default filter, even though the whole list is loaded upfront (making the default filter 'honest' per the plan's own framing) — chosen to avoid the bits-ui 2.18.1/Svelte 5 default-filter gotcha this milestone's plan 02-05 already documented hitting."
  - "FacetStrip's category chips use queries.ts's CATEGORIES (the four record categories: convention/gotcha/decision/preference), not classify.ts's KNOWN_CATEGORIES (which also lists discovery/rule) — those two are separate record KINDS, not SearchMemoriesRequest.categories filter values, matching ScopesSidebar's existing category-chip convention."
  - "hitsWithoutCategory is computed via a second SearchMemories call keyed with categories stripped (searchMemoriesKey({ ...eff, categories: [] })) — the same key the unfiltered view already uses, so it is usually cache-warm rather than a fresh network round trip."

patterns-established:
  - "Tracer feedback gate: Task 1 (tracer) was re-verified end-to-end (both <verify> automated checks passed) before Task 2/3 expansion began, per auto-mode's re-run-and-continue rule."

requirements-completed: [ENTRY-01, ENTRY-02, ENTRY-03, ENTRY-06, ROW-04, ROW-05, ROW-06]

coverage:
  - id: D1
    description: "ENTRY-01/02/06, SC4: /search resolves id/short_id via GetMemory (never SearchMemories) and text via SearchMemories with cross_spine true by default; every query carries an AbortSignal and keys on the full normalized params, so a stale response is provably aborted and never rendered, and previous rows stay visible (keepPreviousData) while a newer query is in flight"
    requirement: "ENTRY-06"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — race safety (SC4) > never renders a stale response: the earlier query is aborted and the later one wins"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — keep previous data while re-querying > keeps the previous rows visible and marks the list busy until the newer query resolves"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — defaults > sends crossSpine true, an empty scope, k=50n and full=true for plain text"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — id resolution > resolves a UUID via GetMemory, never calls SearchMemories, shows the resolution line and opens the pane"
        status: pass
    human_judgment: false
  - id: D2
    description: "ENTRY-01/02/06 keyboard/pane wiring: clicking a row opens the pane and toggles closed on a second click; 'e' on the active row prefetches the record for WriteSurfaces.openEdit; the two pre-existing resume-recovery tests keep passing"
    requirement: "ENTRY-02"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — toggle pane > clicking a row opens the pane; clicking the same row again closes it"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — row-action keys > 'e' on the active row prefetches the record for editing via WriteSurfaces.openEdit"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — re-auth landing recovery (both cases)"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-05/D-06/ENTRY-03: the ranked header states hits (with 'of N' under a category filter), scope coverage (verbatim scopes_truncated/scopes_unknown, never inferred, 'every readable scope' when unknown), ranking source (cosine vs jev), the per-state hidden-count clause and tooltip, an expandable searched_scopes list, and the honest id/short_id/short_id-miss resolution lines"
    requirement: "ENTRY-03"
    verification:
      - kind: unit
        ref: "ui/src/lib/search/recall-header.test.ts (18 cases)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultsHeader.browser.test.ts (3 cases: scopes expansion, hidden tooltip, long-query ellipsis)"
        status: pass
    human_judgment: false
  - id: D4
    description: "ROW-05: category (OR/multi-select), tag, created-window and state-inclusion filters are removable/toggleable chips that map one-to-one onto SearchMemoriesRequest fields, round-trip through the URL, and never navigate themselves"
    requirement: "ROW-05"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/FacetStrip.browser.test.ts (7 cases)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — facet chips round-trip through the URL (ROW-05) (3 cases)"
        status: pass
    human_judgment: false
  - id: D5
    description: "ROW-06: the scope combobox lists 'Every readable scope (cross_spine)' first, then every ListScopes entry in response order with its readable-record count (~-prefixed when approximate), filters by substring over the complete list, and reports loading/error states with Retry"
    requirement: "ROW-06"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ScopeCombobox.browser.test.ts (7 cases)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Visual correctness of the recall surface's layout — the input row, facet strip, results header and the list/pane split rendering together as one coherent page at real widths and in both themes"
    verification: []
    human_judgment: true
    rationale: "No automated visual regression harness exists in this repo; layout/contrast correctness needs a human look, consistent with workflow.human_verify_mode=end-of-phase deferring UI judgment calls to end-of-phase UAT (matches the precedent already recorded for plans 02-04/02-06/02-07 in this phase)."

duration: 130min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 8: Rebuilt /search — Race-Safe Recall Surface Summary

**`/search` rebuilt end to end on URL-driven params: id/short_id resolve via `GetMemory`, free text runs `SearchMemories` cross-spine by default with a per-query `AbortSignal` and `keepPreviousData` (SC4's race test proves the stale response is aborted, never rendered), the one honest results-header formatter (`recall-header.ts`) states hits/scopes/ranking/hidden/coverage verbatim, and URL-persisted facet chips (category, tag, scope via a `ListScopes` combobox, created window, state-inclusion) round-trip through `encodeSearchParams` with the strip itself owning no navigation.**

## Performance

- **Duration:** ~130 min
- **Started:** 2026-09-26T18:07:00Z (approx.)
- **Completed:** 2026-09-26T20:17:00Z (approx.)
- **Tasks:** 3 (1 tracer, 2 auto/tdd)
- **Files modified:** 10 (8 created, 2 modified)

## Accomplishments

- `ui/src/lib/search/recall-header.ts`: the ONE header formatter — `rankedHeaderParts` (hits · scopes · query · ranking · hidden · coverage, in that fixed order), `resolutionLine` (id/short_id/short_id-miss outcomes, including the "hidden from search, fetchable by id" note for a superseded resolved record), `hiddenTooltip` (per-state breakdown plus the overlap note when a record's states sum past its total), `plural`, `loadingLine` — 18 unit tests pinning every clause's exact copy.
- `ui/src/lib/components/ResultsHeader.svelte`: the one-line `aria-live="polite"` header — bold counts, the query in a mono code span (ellipsized, full text in its title), a "N scopes" button expanding a `searched_scopes:` list with each scope's hit count "in top k" (zeros included, server order), the hidden clause's tooltip, a 2px progress bar while busy.
- Rebuilt `ui/src/routes/search/+page.svelte`: URL params parsed once (`parseSearchParams`), the shared classifier decides id/short_id/text, every query (`getMemory`, `searchMemories`, the short_id-miss fallback search, `listScopes`, the category-counts search, the detail-pane `getMemory`) carries `{ signal }` and keys on the full normalized params; `keepPreviousData` on every search-shaped query; the shared `ResultsList`/`RecallSplit`/`DetailPane` set from plans 02-06/02-07 renders the list and pane, with row keys routed through `WriteSurfaces`.
- `ui/src/lib/components/ScopeCombobox.svelte`: a `Popover` + `Command` scope picker over `ListScopes` — the pinned "Every readable scope (cross_spine)" entry first, then every scope in response order with its readable-record count (`~`-prefixed when approximate), substring-filtered over the complete loaded list, loading/error states with Retry.
- `ui/src/lib/components/FacetStrip.svelte`: one horizontally scrollable line of URL-persisted chips — category toggles with live counts (zero-count chips dimmed), the scope combobox, a `cross_spine` checkbox (hidden while a scope is set), `include_archived`/`include_superseded`/`include_scheduled` checkboxes, removable tag chips, and a created-window popover producing RFC3339 midnight-UTC strings — the strip itself calls only `onchange(partial)`, never `goto`.
- Category-filtered searches get a second, categories-stripped `SearchMemories` call (same cache key the unfiltered view already used) supplying both the per-category facet counts and the header's "N hits of M" clause.

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): /search end to end — URL, classifier, race-safe queries, header, list, pane, row keys, the SC4 race test** — `c3dc2aa4` (feat)
2. **Task 2: The complete honest header — resolution lines, "of N", coverage expansion, verbatim truncated/unknown** — `9a92d316` (feat, `tdd="true"`)
3. **Task 3: FacetStrip and ScopeCombobox — chips round-trip through the URL, facet counts** — `ca4beac7` (feat, `tdd="true"`)

**Plan metadata:** commit follows this SUMMARY.

_Tasks 2 and 3 carried `tdd="true"`; implementation and their behavior tests landed together in one commit per task rather than as separate RED/GREEN commits — this plan's frontmatter is `type: execute` (not `type: tdd`), so the strict plan-level RED/GREEN/REFACTOR gate does not apply, matching the already-recorded precedent for this phase (`WINDOWS.md` id 1/2, and every prior 02-0x SUMMARY in this phase)._

## Files Created/Modified

- `ui/src/lib/search/recall-header.ts` / `recall-header.test.ts` — the header formatter and its 18 unit tests
- `ui/src/lib/components/ResultsHeader.svelte` / `ResultsHeader.browser.test.ts` — the results header component and its 3 browser tests
- `ui/src/lib/components/FacetStrip.svelte` / `FacetStrip.browser.test.ts` — the facet chip strip and its 7 browser tests
- `ui/src/lib/components/ScopeCombobox.svelte` / `ScopeCombobox.browser.test.ts` — the scope picker and its 7 browser tests
- `ui/src/routes/search/+page.svelte` — the rebuilt recall surface
- `ui/src/routes/search/search.browser.test.ts` — rewritten with 11 cases (race, keep-previous, defaults, id resolution, toggle, row keys, 3 facet-chip cases, 2 resume tests kept)

## Decisions Made

- Facet-strip state (URL-persisted scope/tags/categories) is folded into the query box's own chip vocabulary as synthesized `OperatorChip`s, placed ahead of the inline chips the classifier derives from typed text, before calling `params.ts`'s `applyChips` — `applyChips` itself was left unmodified (outside this plan's `files_modified`); without this composition, typing plain text with no operators would call `applyChips` with an empty chip list and silently wipe out whatever category/tag facets the strip had set, since `applyChips` derives tags/categories from the chip set alone.
- `WriteSurfaces` moved to a stable toolbar row outside `RecallSplit`'s `list` snippet, not inside it — found live while writing the resume-recovery tests: `RecallSplit` measures its container width via a post-mount `ResizeObserver` and switches between two layout branches (`{#if isNarrow}`/`{:else}`) that each independently `{@render list()}`; Svelte tears down and recreates a snippet's rendered content across that kind of branch switch, which was silently destroying the `WriteSurfaces` instance the page's one-shot `onMount` resume-restore call was targeting, so the seeded create-mode envelope's dialog never opened.
- `ScopeCombobox` filters the loaded `ListScopes` list manually (`shouldFilter={false}`) rather than using bits-ui `Command`'s own default filter, even though loading the whole list upfront would make the default filter equally "honest" per the plan's own framing — chosen specifically to avoid the bits-ui 2.18.1/Svelte 5 default-filter content-emptying gotcha this milestone's plan 02-05 already hit and documented.
- `FacetStrip`'s category chips use `queries.ts`'s `CATEGORIES` (the four record categories), not `classify.ts`'s `KNOWN_CATEGORIES` (which also lists `discovery`/`rule`) — those two are separate record kinds, not values `SearchMemoriesRequest.categories` filters on, matching the existing `ScopesSidebar` category-chip convention.
- `hitsWithoutCategory` is computed via a second `SearchMemories` call keyed with `categories: []` stripped — the exact key the unfiltered view already used, so it is usually cache-warm rather than a fresh network round trip, per the plan's own "same key… usually cached" framing.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `WriteSurfaces` silently destroyed and recreated by `RecallSplit`'s narrow/wide layout switch**
- **Found during:** Task 1 (writing the re-auth landing recovery tests carried forward from the pre-existing suite)
- **Issue:** The plan's own action text placed `WriteSurfaces` inside `RecallSplit`'s `list` snippet (mirroring the pre-rebuild page's layout). `RecallSplit` measures its own container width via a `ResizeObserver` inside an `$effect` that only settles a tick after mount, and switches between two layout branches that each independently call `{@render list()}`. Svelte tears down and recreates a snippet's rendered content when the enclosing `{#if}`/`{:else}` branch it lives in changes — so the very first width measurement (almost always narrower than the isolated test's actual real-app flex-chain width) destroyed the initial `WriteSurfaces` instance and created a new one, invalidating the `bind:this` reference the page's `onMount` resume-restore call had already captured. The seeded create-mode envelope's dialog never opened.
- **Fix:** Moved `WriteSurfaces` (and its `bind:this`) to a stable toolbar row directly in `+page.svelte`, outside `RecallSplit` entirely, so it is never inside either of `RecallSplit`'s branch-switching snippets.
- **Files modified:** `ui/src/routes/search/+page.svelte`
- **Verification:** Both re-auth landing recovery tests (create-mode restore, discovery-kind mismatch no-op) pass; full `pnpm --dir ui vitest run` (47 files / 473 tests) stays green.
- **Committed in:** `c3dc2aa4` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — a real, previously-unverified interaction between `RecallSplit`'s resize-driven layout switch and a component the page needs to stay mounted across the whole page lifetime).
**Impact on plan:** Necessary for the plan's own resume-recovery tests (carried forward from the pre-rebuild suite) to pass at all; no scope creep — the fix only relocates one component's markup position, no new component logic.

## Known Stubs

None — every artifact this plan promises is fully wired end to end against real (mocked-in-tests) RPCs. The unranked, operator-only listing (D-09: "operator-only input… is an unranked ListMemories listing that infinite-scrolls") is explicitly out of this plan's scope per its own action text ("operators-only listing is plan 02-09") — `+page.svelte` returns an empty result set for that classifier outcome rather than a placeholder UI, deferred to plan 02-09 as designed, not a broken promise of this plan.

One minor, untested rough edge: when `/search?q=<uuid>` auto-opens the pane on the resolved record (no explicit `sel` in the URL), clicking the pane's close button does not fully collapse it, since the pane's open state falls back to the auto-resolved id whenever the URL's own `sel` is empty. This interaction was not in this plan's `<behavior>` list and has no test; noted here for visibility rather than silently left undiscovered.

## Issues Encountered

- `vi.hoisted`'s factory runs before the test file's own `import` statements are linked, so a synchronous `SvelteURL` import could not be referenced inside it directly (`Cannot access '__vi_import_3__' before initialization`) — resolved with an `await`ed async `vi.hoisted` factory that dynamically `import()`s `svelte/reactivity` inside itself, sidestepping the ordering issue since a dynamic import is a genuine async operation resolved after linking.
- `create(MemorySchema, { ...typedOverrides })`/`create(ListScopesResponseSchema, { ...typedOverrides })` test helpers initially typed their `overrides` parameter as `Partial<Memory>`/`Partial<ListScopesResponse>`, which widens `$typeName` back to `"engram.v1.Memory" | undefined` and fails `create()`'s stricter `MessageInitShape<Desc>` constraint (which requires `$typeName` to be exactly `undefined`); fixed by typing `overrides` as the schema's own `MessageInitShape<typeof XSchema>` (already all-fields-optional) with no extra `Partial<>` wrapper.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `/search` is now the honest, race-safe recall surface ENTRY-01/02/03/06 and ROW-05/06 describe; plan 02-09 can build the unranked operator-only (D-09) listing and the response-too-large/rejected-envelope error states on top of this same page without re-deriving the query/header/facet wiring.
- `ui/src/lib/search/recall-header.ts` is the one place every later plan (02-09, 02-10) must extend for new header clauses — no call site should hand-write header copy.
- `FacetStrip.svelte`/`ScopeCombobox.svelte` are ready for `/observe` or any future recall surface to reuse verbatim (same `SearchParams` shape, same `onchange(partial)` contract).
- Visual correctness of the assembled page (coverage `D6`, `human_judgment: true`) is deferred to end-of-phase UAT, consistent with `workflow.human_verify_mode: end-of-phase`.
- One untested rough edge (id-mode pane close, see Known Stubs) is flagged for the verifier's attention, not hidden.
- No blockers.

---
*Phase: 02-recall-first-search*
*Completed: 2026-09-26*

## Self-Check: PASSED

- Created files exist: `ui/src/lib/search/recall-header.ts`, `recall-header.test.ts`, `ui/src/lib/components/ResultsHeader.svelte`, `ResultsHeader.browser.test.ts`, `ui/src/lib/components/FacetStrip.svelte`, `FacetStrip.browser.test.ts`, `ui/src/lib/components/ScopeCombobox.svelte`, `ScopeCombobox.browser.test.ts` — all found. `ui/src/routes/search/+page.svelte` and `search.browser.test.ts` modified in place.
- Commits exist: `c3dc2aa4` (Task 1), `9a92d316` (Task 2), `ca4beac7` (Task 3) — all found in `git log --oneline`.
- All plan `<acceptance_criteria>` re-run and passing per task (Task 1: `{ signal }` ≥2 found 13, `placeholderData: keepPreviousData` ≥1 found 3, `k: 50n` = 0, `peekResume|consumeResume` ≥2 found 4; Task 2: `scope list incomplete` ≥1 found 1, `'…not searched…'` = 0, `reranked by jev` ≥1 found 1; Task 3: `No scope matches` ≥1 found 1, `includeArchived|includeSuperseded|includeScheduled` distinct = 3, `goto(` in FacetStrip/ScopeCombobox = 0).
- Plan-level `<verification>` re-run: `pnpm --dir ui vitest run --project node src/lib/search` (3 files / 46 tests) and `pnpm --dir ui vitest run --project browser src/routes/search src/lib/components/ResultsHeader.browser.test.ts src/lib/components/FacetStrip.browser.test.ts src/lib/components/ScopeCombobox.browser.test.ts` (4 files / 28 tests) both pass; `pnpm --dir ui build` exits 0.
- Full `pnpm --dir ui vitest run` (47 files / 473 tests) and `npx tsc --noEmit` (zero new errors beyond the pre-existing, unrelated badge/button/tabs `*.svelte` module-resolution errors, confirmed present before this plan by every prior 02-0x SUMMARY in this phase) both clean.
