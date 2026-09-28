---
phase: 02-recall-first-search
plan: 09
subsystem: ui
tags: [svelte5, tanstack-query, connect-web, recall-surface, honest-states, infinite-scroll]

requires:
  - phase: 02-recall-first-search
    provides: "rebuilt /search route, recall-header.ts formatter, ResultsList mode/hasMore/onloadmore props, connect-error.ts's parseConnectError/fixRowsFor, params.ts's SearchParams/K_STEPS (plan 02-08 and earlier)"
provides:
  - "ui/src/lib/components/RecallState.svelte: the one component every /search empty, rejected, ambiguous-short_id, not-found and opaque-failure state renders through"
  - "ui/src/lib/search/recall-header.ts: emptyHeading (ENTRY-03) and listingHeaderParts (D-09/D-02 list-mode)"
  - "ui/src/lib/search/params.ts: nextK (D-08 k escalation), listMemoriesCursorKey and listMemoriesRequest (D-09 cursor-mode ListMemories codec)"
  - "/search: honest empty/error states, a k 50->100->250->1000 'Show more' row, and an unranked infinite-scroll listing for operator-only input"
affects: [02-10]

actuals:
  tokens: 11032
  tasks: 3
  commits: 3
  plan_head_before: c125988bab0a2b14244e7809d263fe3098821ee8

tech-stack:
  added: []
  patterns:
    - "One honest-state pipeline (activeResult -> parsedError/isEmpty -> recallState) that every classification outcome (text, id/short_id, id/short_id short_id-miss fallback, operators) feeds — RecallState.svelte is the single render target for every empty/error branch, never a bare empty list"
    - "recall-header.ts stays the ONE header-copy formatter: emptyHeading and listingHeaderParts join rankedHeaderParts/resolutionLine/loadingLine as the only place header or empty-state clause text is authored"
    - "Empty-state fix rows (search-all-scopes/include-archived/include-superseded/include-scheduled/clear-category) use a page-local id vocabulary distinct from connect-error.ts's FixRow id union (enable-cross-spine/pick-scope/lower-k/without-full/clear-created/retry) — RecallState.svelte's RecallFixRow type is intentionally generic rather than reusing the narrower rejection-envelope type"
    - "createInfiniteQuery (v6 options-function form) for D-09's operator-only ListMemories cursor listing, flattening query.data.pages into the same ResultsList/mode='unranked' component the ranked path uses"

key-files:
  created:
    - ui/src/lib/components/RecallState.svelte
  modified:
    - ui/src/lib/search/recall-header.ts
    - ui/src/lib/search/recall-header.test.ts
    - ui/src/lib/search/params.ts
    - ui/src/lib/search/params.test.ts
    - ui/src/routes/search/+page.svelte
    - ui/src/routes/search/search.browser.test.ts

key-decisions:
  - "RecallState's empty-state fix rows use their own id vocabulary (search-all-scopes, include-archived, include-superseded, include-scheduled, clear-category), separate from connect-error.ts's FixRow id union used for rejected/opaque errors — the two vocabularies serve different classes of recovery action and connect-error.ts was not in this plan's files_modified."
  - "'pick-scope' (a fixRowsFor row for a scope-required rejection) opens the ScopeCombobox by dispatching a click on its existing `.scope-combobox-trigger` DOM node rather than adding an imperative open() API to ScopeCombobox.svelte or FacetStrip.svelte — neither file is in this plan's files_modified, and the combobox already renders a real button there."
  - "'without-full' is a page-local $state boolean (fullOverride), not a URL param — retrying a response_too_large rejection without full content is a one-off mitigation, not a durable, shareable preference."
  - "The unranked listing's 'Latest N memories' count uses the flattened, currently-loaded memories.length rather than the server's ListMemoriesResponse.total field — the header describes what has actually loaded as the user scrolls, not a server-side grand total the D-09 wording ('Latest N memories') was read to mean the loaded set."
  - "activeResult's id/short_id branch (no short_id-miss) sets isSuccess: false unconditionally — GetMemory has no 'empty success' concept (it either resolves a record or errors), so a found record can never accidentally route through the empty-state branch."
  - "recall-header.ts's not-found RecallState message omits the requested id, since ParsedConnectError's not-found variant carries no id (connect-error.ts was out of this plan's files_modified) — the D-04/not-found wording states the outcome without fabricating an id the parsed error does not carry."

patterns-established:
  - "Tracer feedback gate: Task 1 (tracer) was re-verified end-to-end (both <verify> automated checks passed) before Task 2/3 expansion began, per auto-mode's re-run-and-continue rule."

requirements-completed: [ENTRY-03, ENTRY-05, ROW-01]

coverage:
  - id: D1
    description: "ENTRY-03/ENTRY-05/D-04: /search renders an honest empty state (query + scope coverage + one-click fixes) only once fetching has settled, and an honest failure state (rejected envelope with fix rows, opaque failure with Retry/Copy error, D-04 ambiguous-short_id warning, not-found) — never a bare empty list for a failed search"
    requirement: "ENTRY-03"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — honest empty state (ENTRY-03) > cross-spine zero hits names the query, offers fix rows for hidden/category state, and never flashes \"no matches\" while in flight"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — honest empty state (ENTRY-03) > a scoped zero-hit response offers \"Search every readable scope\", which removes scope and restores cross-spine"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — honest failure states (ENTRY-05) > a rejected request renders the real envelope and its fix row; clicking it restores cross-spine"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — honest failure states (ENTRY-05) > an opaque failure says nothing was searched, offers Retry and Copy error, and shows no empty heading"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — honest failure states (ENTRY-05) > an ambiguous short_id renders the D-04 warning line"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — honest failure states (ENTRY-05) > a UUID with NotFound renders the not-found line"
        status: pass
      - kind: unit
        ref: "ui/src/lib/search/recall-header.test.ts#emptyHeading (4 cases)"
        status: pass
    human_judgment: false
  - id: D2
    description: "ENTRY-05: a rejected request's fix rows are wired to real navigation/retry actions (enable-cross-spine, retry, and a page-local without-full override), not just displayed copy"
    requirement: "ENTRY-05"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — honest failure states (ENTRY-05) (rejected/opaque fix-row click assertions above)"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-08: search starts at k 50; a 'Show more' row appears only when the result count fills k and a next K_STEPS value exists, escalating 50 -> 100 -> 250 -> 1000 through the URL while keeping previous rows visible"
    requirement: "ROW-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — Show more escalates k through the URL (D-08) (3 cases: shows at k, hides below k, hides at the 1000 ceiling)"
        status: pass
      - kind: unit
        ref: "ui/src/lib/search/params.test.ts#nextK / listMemoriesCursorKey / listMemoriesRequest (7 cases)"
        status: pass
    human_judgment: false
  - id: D4
    description: "ROW-01 at scale: after Show more reaches k 1000 the virtualized list stays smooth and keyboard-traversable to the last row"
    requirement: "ROW-01"
    verification: []
    human_judgment: true
    rationale: "Smoothness/keyboard-traversal at 1000 real virtualized rows needs a live browser observation (frame timing, scroll feel) no unit or DOM-assertion test captures; the virtualizer itself (ResultsList.svelte, plan 02-06) is unchanged by this plan. Deferred to end-of-phase UAT per workflow.human_verify_mode=end-of-phase, consistent with every prior 02-0x SUMMARY in this phase."
  - id: D5
    description: "D-09/D-02: operator-only input (no free text) runs an unranked ListMemories cursor listing — never SearchMemories — with score rendered as '—', a 'Latest N memories across M scopes · unranked (list — no score)' header, a hidden-count clause summed across every loaded page ('hidden count unavailable' if any page's count is missing), and infinite-scroll pagination via nextPageToken"
    requirement: "ROW-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — operator-only input lists memories unranked with infinite scroll (D-09) > runs ListMemories with the parsed operators, never SearchMemories, and shows the unranked header"
        status: pass
      - kind: automated_ui
        ref: "ui/src/routes/search/search.browser.test.ts#search route — operator-only input lists memories unranked with infinite scroll (D-09) > appends a second page when nextPageToken is set, and stops once it is empty"
        status: pass
      - kind: unit
        ref: "ui/src/lib/search/recall-header.test.ts#listingHeaderParts (3 cases)"
        status: pass
    human_judgment: false

duration: ~40min (approx.)
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 9: Honest Empty/Failure States, Show More, and Unranked Listing Summary

**`/search` now says what happened for every zero-hit and failure outcome through one `RecallState.svelte` component, grows `k` 50→100→250→1000 on request via a "Show more" row, and lists operator-only input (`scope:`/`#tag`/`is:`) as an unranked, infinite-scrolling `ListMemories` cursor query instead of returning an empty result set.**

## Performance

- **Duration:** ~40 min (approx.)
- **Tasks:** 3 (1 tracer, 2 auto)
- **Files modified:** 7 (1 created, 6 modified)

## Accomplishments

- `RecallState.svelte`: the one component every honest empty/failure outcome on `/search` renders through — an empty state names the query and exactly what was searched (cross-spine vs. a bounded scope count) plus one-click fix rows; a rejected request shows `Server rejected the request` and the real `field=<f> hint=<code>: <text>` envelope with its fix rows; an ambiguous short_id shows the D-04 warning line; a not-found id says so without fabricating an id it was never given; an opaque failure says "Search failed — nothing was searched" plus "Nothing was searched, so this is not an empty result" with Retry and Copy error — never a fabricated request id, never a bare empty list for a failure.
- `recall-header.ts` gains `emptyHeading` (ENTRY-03's empty heading, singular/plural-correct, the hidden clause only when non-zero) and `listingHeaderParts` (D-09's unranked-listing header, summing `recall_gate_hidden` across every loaded page and reporting "hidden count unavailable" if any page's count is missing).
- `/search` computes its empty state only when `isSuccess && !isFetching && count === 0` (Pitfall 3 — never a flash to empty mid-flight) through one `activeResult`/`parsedError`/`isEmpty`/`recallState` pipeline that every classification outcome (plain text, id/short_id, the short_id-miss text fallback, and operator-only listing) feeds.
- `params.ts` gains `nextK` (walks `K_STEPS`, undefined at the 1000 ceiling), `listMemoriesCursorKey` and `listMemoriesRequest` (the cursor-mode `ListMemories` codec, mirroring `queries.ts`'s `listMemoriesKey` index-3 visibility-slot contract for the existing mutation-invalidation code).
- `/search` renders a "Show more" row below the ranked list exactly when hits fill the current `k` and a next `K_STEPS` value exists, escalating via the URL while `keepPreviousData` keeps the prior rows visible.
- `/search` runs a `createInfiniteQuery`-backed `ListMemories` cursor listing for operator-only input (no free text), rendering `ResultsList` in `mode="unranked"` (dash-score rows, already supported by plan 02-06's `ResultRow`) with `hasMore`/`onloadmore` wired to the virtualizer's own near-end trigger.

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): Honest empty and failure states end to end — RecallState on /search** — `966e3af8` (feat)
2. **Task 2: Show more — k 50 → 100 → 250 → 1000 in the URL — and the list-mode codec helpers** — `e81f3249` (feat)
3. **Task 3: Operator-only input — unranked ListMemories infinite scroll and its header** — `ea0fb561` (feat)

**Plan metadata:** commit follows this SUMMARY.

_Tasks 2 and 3 carried `tdd="true"` in PLAN.md; implementation and their behavior tests landed together in one commit per task rather than as separate RED/GREEN commits — this plan's frontmatter is `type: execute` (not `type: tdd`), so the strict plan-level RED/GREEN/REFACTOR gate does not apply, matching the already-recorded precedent for this phase (`WINDOWS.md` id 1/2, and every prior 02-0x SUMMARY)._

## Files Created/Modified

- `ui/src/lib/components/RecallState.svelte` — the one empty/error/ambiguous/not-found/opaque render target for `/search`
- `ui/src/lib/search/recall-header.ts` / `recall-header.test.ts` — `emptyHeading`, `listingHeaderParts`, and their unit tests
- `ui/src/lib/search/params.ts` / `params.test.ts` — `nextK`, `listMemoriesCursorKey`, `listMemoriesRequest`, and their unit tests
- `ui/src/routes/search/+page.svelte` — the honest-state pipeline, Show more row, and operator-only `ListMemories` infinite scroll
- `ui/src/routes/search/search.browser.test.ts` — 11 new cases across empty/failure/Show-more/operator-listing behavior

## Decisions Made

- Empty-state fix rows (`search-all-scopes`, `include-archived`, `include-superseded`, `include-scheduled`, `clear-category`) use their own id vocabulary rather than reusing `connect-error.ts`'s narrower `FixRow` id union (`enable-cross-spine`/`pick-scope`/`lower-k`/`without-full`/`clear-created`/`retry`), which is scoped to rejected/opaque errors and lives in a file outside this plan's `files_modified`.
- `pick-scope` opens the `ScopeCombobox` by clicking its existing `.scope-combobox-trigger` DOM node rather than adding an imperative `open()` API to `ScopeCombobox.svelte`/`FacetStrip.svelte` — neither file is in this plan's scope, and the combobox already renders a real trigger button there.
- `without-full` is a page-local `$state` boolean, not a URL param — a one-off `response_too_large` mitigation, not a durable/shareable preference.
- The unranked listing's "Latest N memories" count uses the flattened, currently-loaded `memories.length` rather than `ListMemoriesResponse.total` — the header describes what has actually loaded as the user scrolls, matching D-09's "Latest N" framing.
- `activeResult`'s id/short_id branch (no short_id-miss) sets `isSuccess: false` unconditionally, since `GetMemory` has no "empty success" concept — a found record can never accidentally route through the empty-state branch.
- The not-found `RecallState` message omits the requested id, since `ParsedConnectError`'s `not-found` variant carries no id and `connect-error.ts` was out of this plan's `files_modified`.

## Deviations from Plan

None — plan executed exactly as written. The task-boundary split above (which files/hunks landed in which task's commit) required reconstructing intermediate working-tree states for genuinely atomic commits, since the three tasks share deeply-coupled reactive derivations in `+page.svelte`; each intermediate state was independently re-verified against its own task's `<verify>` commands before committing (see Task Commits above).

## Known Stubs

None introduced by this plan. Carried forward from `02-08-SUMMARY.md`'s "Known Stubs" (untouched by this plan's `files_modified`): when `/search?q=<uuid>` auto-opens the pane on the resolved record (no explicit `sel` in the URL), clicking the pane's close button does not fully collapse it, since the pane's open state falls back to the auto-resolved id whenever the URL's own `sel` is empty. This plan's work did not touch `autoOpenId`/`effectiveSel`, so the rough edge is unchanged and still flagged for the verifier's attention.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `/search`'s honest-state pipeline (`activeResult`/`parsedError`/`isEmpty`/`recallState`) and `RecallState.svelte` are the pattern any future recall surface (`/`, `/observe`) should reuse rather than re-deriving empty/error handling.
- `recall-header.ts` remains the one place every header/empty-heading clause is authored — `emptyHeading` and `listingHeaderParts` joined `rankedHeaderParts`/`resolutionLine`/`loadingLine`.
- `params.ts`'s `nextK`/`listMemoriesCursorKey`/`listMemoriesRequest` are ready for plan 02-10 or any future cursor-mode listing to reuse verbatim.
- ROW-01 could not be checked off in `REQUIREMENTS.md` by this plan (`requirements.mark-complete` reports `not_found` for every requirement this milestone — `WINDOWS.md` id 17, a pre-existing, milestone-wide condition where every traceability row was seeded `Mapped` at milestone creation, a status the verb does not recognize as a flippable FROM-state). Recorded in this SUMMARY's `requirements-completed` per the standing workaround; not caused by this plan.
- The virtualized-smoothness-at-1000-rows judgment call (coverage `D4`, `human_judgment: true`) is deferred to end-of-phase UAT per `workflow.human_verify_mode: end-of-phase`.
- No blockers.

---
*Phase: 02-recall-first-search*
*Completed: 2026-09-26*

## Self-Check: PASSED

- Created file exists: `ui/src/lib/components/RecallState.svelte` — found.
- Commits exist: `966e3af8`, `e81f3249`, `ea0fb561` — all found in `git log --oneline --all`.
- Plan `<acceptance_criteria>` re-run and passing: Task 1 (`Nothing was searched...` = 1, `fixRowsFor(` ≥ 1 found 1, `request id` = 0), Task 2 (`export function nextK(` = 1, `Show more` ≥ 1 found 1), Task 3 (`createInfiniteQuery` ≥ 1 found 2, `unranked (list — no score)` ≥ 1 found 2).
- Plan-level `<verification>` re-run: `pnpm --dir ui vitest run --project node src/lib/search` (3 files / 60 tests) and `pnpm --dir ui vitest run --project browser src/routes/search` (1 file / 22 tests) both pass; `pnpm --dir ui build` exits 0.
- Full `pnpm --dir ui vitest run` (47 files / 498 tests) passes (one file, `ScopesSidebar.browser.test.ts`, hit the pre-documented Playwright pointer-interception flake on a combined run and passed clean in isolation — not a regression from this plan). `npx tsc --noEmit` shows only the pre-existing badge/button/tabs `*.svelte` module-resolution errors, unchanged from `02-08-SUMMARY.md`'s own self-check.
