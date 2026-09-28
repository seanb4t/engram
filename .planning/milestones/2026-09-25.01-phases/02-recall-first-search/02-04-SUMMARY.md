---
phase: 02-recall-first-search
plan: 04
subsystem: ui
tags: [svelte5, bits-ui, tanstack-query, connect-web, header-search, command-menu]

requires:
  - phase: 02-recall-first-search
    provides: "RecallGateHidden proto message + recall_gate_hidden fields on SearchMemoriesResponse/ListMemoriesResponse (plan 02-01)"
  - phase: 02-recall-first-search
    provides: "D-13 design tokens, --u scaling unit, popover primitive install (plan 02-03)"
provides:
  - "ui/src/lib/search/classify.ts: the ONE UUID/short_id/text/operator classifier every id-accepting search box uses"
  - "ui/src/lib/components/HeaderSearch.svelte: the server-driven header search box + anchored dropdown replacing the lying CommandPalette's search half"
  - "ui/src/lib/errors/connect-error.ts: parseConnectError/fixRowsFor — rawMessage/code classification, ambiguous-short-id checked before the field=/hint= envelope"
  - "ui/src/lib/search/params.ts: the one URL/request codec for /search, the header hand-off and the ⌘K command menu (defaultSearchParams/parseSearchParams/encodeSearchParams/searchMemoriesKey/searchMemoriesRequest/applyChips)"
  - "ui/src/lib/search/header-search.svelte.ts: the headerSearch hand-off store (text, focusSeq) for the ⌘K command menu (02-05)"
affects: [02-05, 02-06, 02-07, 02-08]

actuals:
  tokens: 18188
  tasks: 3
  commits: 3
  plan_head_before: 5d7ad163

tech-stack:
  added: []
  patterns:
    - "bits-ui Command.Root wrapping BOTH the visible input and a Popover.Content dropdown, with the Popover's portal explicitly disabled (portalProps={{ disabled: true }}) — Command.Root's item registry (getValidItems) is DOM-scoped to its own ref, so the default body-portal makes every dropdown item invisible to arrow-key/Enter selection"
    - "A module-level Svelte 5 $state object (header-search.svelte.ts) as the cross-component hand-off point for search text + a focus-trigger counter, read by an $effect that skips its own first run so mounting never steals focus"
    - "ConnectError classification reads only .rawMessage/.code, never .message (code-prefixed); the ambiguous-short-id regex is checked before the field=/hint= envelope regex since both are CodeFailedPrecondition"

key-files:
  created:
    - ui/src/lib/search/classify.ts
    - ui/src/lib/search/classify.test.ts
    - ui/src/lib/search/params.ts
    - ui/src/lib/search/params.test.ts
    - ui/src/lib/search/header-search.svelte.ts
    - ui/src/lib/errors/connect-error.ts
    - ui/src/lib/errors/connect-error.test.ts
    - ui/src/lib/components/HeaderSearch.svelte
    - ui/src/lib/components/HeaderSearch.browser.test.ts
  modified:
    - ui/src/lib/components/AppShell.svelte
    - ui/src/lib/components/AppShell.browser.test.ts
    - .gitignore

key-decisions:
  - "Popover portal disabled (portalProps={{ disabled: true }}) rather than the research pattern's default body-portal — found live during Task 3 keyboard testing: bits-ui's CommandRootState.getValidItems() queries this.opts.ref.current.querySelectorAll(...), scoped to Command.Root's own DOM subtree, so a body-portaled dropdown is structurally invisible to arrow-key/Enter selection. Rendering inline keeps floating (fixed-position) placement correct since Floating UI positioning does not depend on portal target."
  - "onCloseAutoFocus prevented on Popover.Content alongside onOpenAutoFocus — without it, bits-ui's focus-scope returns focus to the input after Esc closes the dropdown, undoing the blur the D-11 keyboard model requires."
  - "applyChips (params.ts) includes ALL category chips (known and unknown) in the derived request, unlike HeaderSearch's own live-query categories (which filter to known-only) — the codec is a mechanical chip-to-params mapping; the known-only search-request gate is the header search's own business rule, kept local to Task 2's inline effectiveCategories."
  - "Scopes and Categories dropdown sections render unconditionally whenever the query is text/operators (not only while completing a scope:/is: token) — confirmed against the UI-SPEC's E2 populated truth (\"Dropdown sections render in order: top row, Memories, Scopes, Categories, Commands\"), not just the CONTEXT.md summary's completion-only framing."

patterns-established:
  - "Tracer feedback gate: Task 1 (tracer) was re-verified end-to-end (both <verify> automated checks passed) before Task 2/3 expansion began, per auto-mode's re-run-and-continue rule."

requirements-completed: [ENTRY-01, ENTRY-02, ENTRY-04, ENTRY-05, ENTRY-06]

coverage:
  - id: D1
    description: "One shared classifier (classify.ts) resolves a raw string to id / short_id / text+chips / operators+chips; UUID and short_id checks run before operator parsing, single-token only"
    requirement: "ENTRY-01"
    verification:
      - kind: unit
        ref: "ui/src/lib/search/classify.test.ts"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#resolves a pasted UUID via GetMemory and never calls SearchMemories"
        status: pass
    human_judgment: false
  - id: D2
    description: "Free text with no scope chip sends SearchMemories with cross_spine true and empty scope; a scope: chip sends that scope with cross_spine false; removing the cross-spine chip leaves a dashed ghost chip and triggers the server's real rejection"
    requirement: "ENTRY-02"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#shows interpreted-as chips for scope: and sends the scope with crossSpine false"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#leaves a dashed \"+ cross-spine\" ghost chip when the cross-spine chip is removed, and sends the real rejection"
        status: pass
      - kind: unit
        ref: "ui/src/lib/search/params.test.ts#applyChips"
        status: pass
    human_judgment: false
  - id: D3
    description: "The header search dropdown is server-driven: bits-ui Command with shouldFilter={false}, never Command.Dialog; a term absent from every static label issues a real SearchMemories call and never renders \"no matches\" whether hits are present or zero (SC2)"
    requirement: "ENTRY-04"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#SC2: a term absent from every static label calls SearchMemories and never shows \"no matches\""
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#shows the honest empty-hits copy — never \"no matches\" — for zero memories"
        status: pass
    human_judgment: false
  - id: D4
    description: "Rejected requests render the real field=/hint= envelope with selectable fix rows; an ambiguous short_id renders the D-04 warning line with no candidates; a not-found UUID says so honestly; a short_id-shaped miss is re-searched as text with a visible note, never a silent reinterpretation"
    requirement: "ENTRY-05"
    verification:
      - kind: unit
        ref: "ui/src/lib/errors/connect-error.test.ts"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#renders a rejected envelope with the mono field=/hint= text and its fix rows (ENTRY-05)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#renders the D-04 ambiguous short_id warning with no candidate list"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#renders the not-found line for a UUID GetMemory reports missing"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#re-searches a short_id-shaped miss as text and says so, never silently reinterpreting"
        status: pass
    human_judgment: false
  - id: D5
    description: "Header search queries debounce ~70ms, pass a per-query AbortSignal, use placeholderData keepPreviousData with a \"previous results ·\" prefix while refetching, and set meta.silent so failures render inline rather than the global banner"
    requirement: "ENTRY-06"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#SC2: a term absent from every static label calls SearchMemories and never shows \"no matches\" (asserts the AbortSignal argument)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#shows a \"previous results ·\" prefix while a new query is in flight"
        status: pass
    human_judgment: false
  - id: D6
    description: "The /search URL codec (params.ts) parses and encodes SearchParams in one canonical key order, is order-insensitive for the query key, forces the scope/crossSpine adjacency rule, and applyChips derives params from a chip set (sorted/deduped, last-scope-wins, pending ignored)"
    requirement: "ENTRY-06"
    verification:
      - kind: unit
        ref: "ui/src/lib/search/params.test.ts"
        status: pass
    human_judgment: false
  - id: D7
    description: "Enter on the top row hands the classified query + chips to /search through the one shared codec and clears the input; Enter on a memory row does the same plus sel; / focuses the header search from anywhere not already a text field; handoffToHeaderSearch focuses and opens it; Esc closes and blurs"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#Enter on the top row hands the classified query and chips to /search through the codec, and clears the input"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#Enter on a memory row hands the same params plus sel to /search (D-10)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#\"/\" focuses the header search from anywhere that is not a text field"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#typing \"/\" inside another text field does not steal focus from it"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#handoffToHeaderSearch puts text in the input, focuses it and opens the dropdown"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#Esc closes the dropdown and blurs the input"
        status: pass
    human_judgment: false
  - id: D8
    description: "Honest coverage: recall-gate hidden count note (singular/plural), a per-scope source button expanding coverage ending with the discovery-lane note, and an unknown is:/danger chip"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#shows the per-state recall-gate hidden-count note"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#expands a per-scope coverage list from the source button, ending with the discovery note (D-06)"
        status: pass
      - kind: automated_ui
        ref: "ui/src/lib/components/HeaderSearch.browser.test.ts#renders an unknown is: category as a danger chip"
        status: pass
    human_judgment: false
  - id: D9
    description: "Visual correctness of chip layout, dropdown placement/overflow at narrow widths, and the header's below-620px wrap"
    verification: []
    human_judgment: true
    rationale: "No automated visual regression harness in this repo; layout correctness at narrow widths and both themes needs a human look, consistent with workflow.human_verify_mode=end-of-phase deferring UI judgment calls to end-of-phase UAT (matches 02-03's D6 precedent)."
  - id: D10
    description: "Scopes/Categories dropdown sections (ListScopes-backed / KNOWN_CATEGORIES-backed) render in the default position and reorder to the top while completing a scope:/in:/is: token"
    verification: []
    human_judgment: true
    rationale: "Implemented per the UI-SPEC's E2 populated truth, but no automated test exercises the token-completion reorder or the ListScopes-backed row content — deferred to end-of-phase UAT alongside D9."

duration: 40min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 4: Header Search — One Classifier, Honest States, the /search Codec Summary

**Replaced the lying CommandPalette's search half with a server-driven header search box: one shared classifier resolves id/short_id/text, every outcome (hits, empty, rejected, ambiguous, missing, reinterpreted) says what actually happened, and Enter/`/` hand off through one URL/request codec — fixing the milestone's motivating "no matches for github" bug at its root.**

## Performance

- **Duration:** 40 min
- **Started:** 2026-09-26T18:45:00Z
- **Completed:** 2026-09-26T19:23:42Z
- **Tasks:** 3 (1 tracer, 2 auto/tdd)
- **Files modified:** 12 (9 created, 3 modified)

## Accomplishments

- `ui/src/lib/search/classify.ts`: the ONE UUID/short_id/text/operator classifier (`classifyInput`), with `scope:`/`in:` → scope chip, `#`/`tag:` → tag chip, `is:` → category chip (known/unknown against `KNOWN_CATEGORIES`), an empty operator value → pending chip — 13 unit tests covering every edge case named in the plan.
- `ui/src/lib/components/HeaderSearch.svelte`: bits-ui `Command.Root shouldFilter={false}` wrapping the visible input and an anchored, non-portaled `Popover.Content` dropdown — text queries `SearchMemories` (cross_spine on by default), id/short_id queries `GetMemory`, operator-only input lists via `ListMemories`. The SC2 regression test (a term in no static label calls the server and never shows "no matches") passes for both populated and zero-hit results.
- `ui/src/lib/errors/connect-error.ts`: `parseConnectError`/`fixRowsFor` classify strictly on `.rawMessage`/`.code` (never `.message`), checking the ambiguous-short-id shape before the field=/hint= envelope since both share `CodeFailedPrecondition` (Pitfall C).
- Honest states wired into `HeaderSearch.svelte`: the rejected-envelope box with selectable fix rows, the D-04 ambiguous-short_id warning (no candidates), UUID not-found copy, a short_id miss re-searched as text with a visible note, interpreted-as chips (kind/scope/tag/category/pending, danger chip for unknown categories, dashed "+ cross-spine" ghost chip), and a status line with hit/scope counts, the per-state recall-gate hidden-count note, a "previous results ·" prefix while refetching, and an expandable per-scope source button ending in the discovery-lane note.
- `ui/src/lib/search/params.ts`: the one URL/request codec (`defaultSearchParams`/`parseSearchParams`/`encodeSearchParams`/`searchMemoriesKey`/`searchMemoriesRequest`/`applyChips`) — canonical param order, sorted/deduped chip values, the scope/crossSpine adjacency rule enforced in one place, `xs=0` only for cross-spine-off-with-no-scope.
- `ui/src/lib/search/header-search.svelte.ts`: the `headerSearch` hand-off store (`text`, `focusSeq`) plus `handoffToHeaderSearch`/`focusHeaderSearch`, wired into `HeaderSearch.svelte` so Enter/`/`/the future ⌘K hand-off (02-05) all drive the same input.
- `AppShell.svelte`: the lying search `Button` is replaced with `<HeaderSearch />`; a compact `⌘K` trigger remains for the command menu.

## Task Commits

Each task was committed atomically:

1. **Task 1 (tracer): Header search end to end — shared classifier, server-driven dropdown, SC2 test** — `70d201a8` (feat)
2. **Task 2 (tdd): Honest states — envelope, ambiguous and missing ids, chips, coverage** — `dd66467d` (feat)
3. **Task 3 (tdd): The /search URL codec, Enter hand-off, `/` to focus** — `9c994d7d` (feat)

**Plan metadata:** commit follows this SUMMARY.

_Note: Tasks 2 and 3 carried `tdd="true"`; per the plan's own action ordering ("write the tests from `<behavior>` first; run; fix code, never tests") and this phase's `type: execute` frontmatter (not `type: tdd`), the strict plan-level RED/GREEN/REFACTOR gate enforcement does not apply — implementation and its behavior tests landed together in one commit per task, matching plan 02-03's already-documented precedent for this same pattern (WINDOWS.md id 1/2)._

## Files Created/Modified

- `ui/src/lib/search/classify.ts` / `classify.test.ts` — the shared classifier
- `ui/src/lib/errors/connect-error.ts` / `connect-error.test.ts` — ConnectError classification and fix-row mapping
- `ui/src/lib/search/params.ts` / `params.test.ts` — the `/search` URL/request codec
- `ui/src/lib/search/header-search.svelte.ts` — the header-search hand-off store
- `ui/src/lib/components/HeaderSearch.svelte` / `HeaderSearch.browser.test.ts` — the header search box and dropdown (24 browser tests)
- `ui/src/lib/components/AppShell.svelte` / `AppShell.browser.test.ts` — wires `HeaderSearch` into the header
- `.gitignore` — ignores `ui/.vitest/` (browser-mode failure-screenshot artifacts, discovered while running this plan's tests)

## Decisions Made

- **Popover portal disabled** (`portalProps={{ disabled: true }}`) rather than the research pattern's default body-portal. Found live while writing Task 3's keyboard tests: bits-ui's `CommandRootState.getValidItems()` queries `this.opts.ref.current.querySelectorAll(...)`, scoped to `Command.Root`'s own DOM subtree — a body-portaled dropdown is therefore structurally invisible to arrow-key/Enter selection (`getValidItems()` always returned `[]`, so Enter silently no-opped). Rendering the dropdown inline fixes this; Floating UI's fixed-position placement is unaffected since it does not depend on portal target.
- **`onCloseAutoFocus` prevented** on `Popover.Content` alongside `onOpenAutoFocus` — without it, bits-ui's focus-scope returns focus to the input after Esc closes the dropdown, undoing the blur the keyboard model (D-11) requires.
- **`applyChips` (params.ts) includes all category chips, known and unknown** in the derived request/params — a mechanical chip-to-params mapping. The "known-only" business rule for the live header-search request stays local to Task 2's inline `effectiveCategories` filter, since the plan's action text explicitly scoped it there ("known category chips → categories") while `applyChips`'s own behavior test explicitly includes an unknown category in its expected output.
- **Scopes/Categories sections render unconditionally**, not only while completing a `scope:`/`is:` token — confirmed against the UI-SPEC's own E2 populated truth ("Dropdown sections render in order: top row, Memories, Scopes, Categories, Commands"), which is more specific than CONTEXT.md's completion-only framing of the same feature.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] bits-ui Command keyboard navigation invisible to body-portaled dropdown items**
- **Found during:** Task 3 (writing the Enter-hand-off keyboard tests)
- **Issue:** `Popover.Content`'s default portal (to `document.body`) moved every `Command.Item` outside `Command.Root`'s own DOM subtree. `CommandRootState.getValidItems()` — the mechanism behind arrow-key movement, first-item auto-selection, and Enter-triggers-click — queries `Command.Root`'s own `ref.current.querySelectorAll(...)`, so it always found zero items; Enter silently did nothing (`goto` never called).
- **Fix:** `portalProps={{ disabled: true }}` on `Popover.Content` renders the dropdown inline; added `onCloseAutoFocus={(e) => e.preventDefault()}` alongside the existing `onOpenAutoFocus` guard so Esc's blur is not immediately undone by bits-ui's own focus-return behavior.
- **Files modified:** `ui/src/lib/components/HeaderSearch.svelte`
- **Verification:** All 6 of Task 3's new keyboard-hand-off tests pass, including two that failed with `goto` never called before the fix (isolated with a throwaway debug test asserting `document.querySelector('[data-selected="true"]')` was `undefined` pre-fix and populated post-fix).
- **Committed in:** `9c994d7d` (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — a real, previously-unverified interaction between two bits-ui primitives this plan was the first to compose this way).
**Impact on plan:** Necessary for the plan's own D-10/D-11 keyboard contract to work at all; no scope creep — the fix only touches `HeaderSearch.svelte`'s `Popover.Content` props.

## Known Stubs

- **Tab does not cycle dropdown sections.** Task 3's action text calls for "Tab moves to the first item of the next group," alongside the implemented `/`-focus, Esc-close, and Enter-hand-off behaviors. This one keyboard affordance was not implemented in this plan (browser's native Tab order still applies) — deferred as a follow-up rather than expanding this already-large plan further. Logged to `WINDOWS.md`.

## Issues Encountered

None beyond the bits-ui portal/keyboard-navigation interaction documented above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- The header search box is live and wired into `AppShell.svelte`; `ui/src/lib/search/header-search.svelte.ts`'s `handoffToHeaderSearch`/`focusHeaderSearch` are ready for plan 02-05's ⌘K command menu to call.
- `ui/src/lib/search/params.ts` is ready to be `/search`'s own URL/request codec (plan 02-06+ rebuilding the recall surface) and needs no further extension for that page's own facet-chip UI — `applyChips`/`encodeSearchParams`/`searchMemoriesRequest` already cover the full `SearchParams` shape that page will read from its URL.
- `ui/src/lib/search/classify.ts` and `ui/src/lib/errors/connect-error.ts` are the shared modules every later id-accepting surface (⌘K hand-off, `/search`'s own input) must import rather than re-deriving.
- Visual correctness (D9/D10 above) and the Tab-cycling gap are deferred to end-of-phase UAT, consistent with `workflow.human_verify_mode: end-of-phase`.
- No blockers.

---
*Phase: 02-recall-first-search*
*Completed: 2026-09-26*

## Self-Check: PASSED

- Created files exist: `ui/src/lib/search/classify.ts`, `classify.test.ts`, `params.ts`, `params.test.ts`, `header-search.svelte.ts`, `ui/src/lib/errors/connect-error.ts`, `connect-error.test.ts`, `ui/src/lib/components/HeaderSearch.svelte`, `HeaderSearch.browser.test.ts` — all found.
- Commits exist: `70d201a8` (Task 1), `dd66467d` (Task 2), `9c994d7d` (Task 3) — all found in `git log --oneline --all`.
- All plan `<acceptance_criteria>` re-run and passing (rg counts for `shouldFilter={false}`, zero `Command.Dialog`, `{ signal }` ≥2, `HeaderSearch` in AppShell ≥2, `.rawMessage` ≥2/`err.message` 0, ambiguous/short_id-miss copy present, zero `{@html}`, `INCLUDE_STATES` used by parse+encode, no hardcoded include-states list, exactly one `'searchMemories'` literal, `encodeSearchParams(` used in HeaderSearch.svelte).
- Plan-level `<verification>` re-run: `pnpm --dir ui vitest run --project node src/lib/search src/lib/errors` (49 tests, 4 files) and `pnpm --dir ui vitest run --project browser src/lib/components/HeaderSearch.browser.test.ts src/lib/components/AppShell.browser.test.ts` (24 tests, 2 files) both pass; `pnpm --dir ui build` exits 0.
- Full `pnpm --dir ui vitest run` (39 files / 371 tests) and `npx tsc --noEmit` (zero new errors beyond the pre-existing, unrelated badge/button/tabs `*.svelte` module-resolution errors, confirmed present before this plan's first commit) both clean.
