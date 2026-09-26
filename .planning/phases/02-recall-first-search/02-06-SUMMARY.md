---
phase: 02-recall-first-search
plan: 06
subsystem: ui
tags: [svelte5, virtualization, aria-listbox, bits-ui, hover-card, tailwind4]

requires:
  - phase: 02-recall-first-search
    provides: "--u scaling unit and D-13 design tokens (surface-2, hover, selected, border-subtle, text-faint, primary-soft, warning, cat-rule) from plan 02-03"
provides:
  - "ResultsList.svelte: virtualized WAI-ARIA listbox (role rewrite over @humanspeak/svelte-virtual-list), j/k/Home/End/Enter keyboard model, dense row rendering, showRel computation, first-load skeletons, re-query dimming, hover-card state machine, D-17 row-action keys, Esc layering, Kbd legend"
  - "ResultRow.svelte: the dense one-line grid (category, summary with inline code, state chips with first+N collapse, tags, scope, age, score, optional rel), container-query column drop-out"
  - "ResultHoverCard.svelte: controlled bits-ui LinkPreview overlay anchored to any row via customAnchor, never stealing focus"
  - "@humanspeak/svelte-virtual-list@0.5.14 (exact pin) as the one new UI dependency this milestone"
affects: [02-08, 02-10]

actuals:
  tokens: 15600
  tasks: 3
  commits: 3
  plan_head_before: 842d310a1bca47c63ed539a7aeb914fd7a5391a9

tech-stack:
  added: ["@humanspeak/svelte-virtual-list@0.5.14"]
  patterns:
    - "Pitfall A resolution: an ancestor-level Svelte action listening in the CAPTURE phase rewrites the library's hardcoded role=region viewport to role=listbox and pre-empts its native Home/End/Arrow scroll handling via stopPropagation before the event ever reaches the library's own (later-registered) same-element keydown listener"
    - "Active row tracked by memory id, never index — survives a re-queried array reference change, resetting to the first row only when the previously-active id is genuinely gone"
    - "Two fully separate state machines for keyboard-active vs pointer/keyboard-hovered row (Pitfall 4): aria-activedescendant only ever moves from j/k/Home/End/Enter; the hover card's cardOpen/cardMemory/cardAnchor never touch it"
    - "State-chip collapse ('first +N') uses its OWN bounded max-width budget (calc(96*var(--u))), independent of the row's container-query column drop-outs — a ResizeObserver measures scrollWidth vs clientWidth against that fixed budget, not the list's width"
    - "Isolated component browser tests must import ui/src/app.css directly — only +layout.svelte pulls it in normally, so every calc(N*var(--u)) product dimension is an invalid (silently-defaulted) CSS value without it"

key-files:
  created:
    - ui/src/lib/components/ResultsList.svelte
    - ui/src/lib/components/ResultsList.browser.test.ts
    - ui/src/lib/components/ResultRow.svelte
    - ui/src/lib/components/ResultRow.browser.test.ts
    - ui/src/lib/components/ResultHoverCard.svelte
  modified:
    - ui/package.json
    - ui/pnpm-lock.yaml

key-decisions:
  - "Pitfall A resolution (1) held (role-rewrite action) — resolution (2) (inner-wrapper listbox) was never needed; the accessibility-role test (1000 rows, one listbox, zero role=region, End/Home/j/k traversal) passed against it directly"
  - "State-chip collapse budget (calc(96*var(--u))) is a NEW, independent measurement surface, not derived from the row's own @container width — this was necessary because the auto-sized CSS grid track for the states column would otherwise size to its own unbounded max-content width regardless of the row's overall available space, making the collapse logic unreachable by any row width"
  - "ResultHoverCard's HoverCard.Root/Content render with NO Trigger element at all — open is driven purely as controlled host state from ResultsList, bypassing the primitive's own hover-delay/detection entirely (per Pattern 3); openDelay/closeDelay props are set to 0 since they are never exercised"
  - "Row-action keys (e/s/#/c/⇧C) are guarded against a focused input/textarea/select/contenteditable AND against Meta/Ctrl/Alt; navigation keys (j/k/Home/End/Enter) carry only the Meta/Ctrl/Alt guard, unchanged from Task 1 — the plan's D-17 wording scoped the typing-target guard to row keys specifically"

patterns-established:
  - "Tracer feedback gate: Task 1 (tracer, Pitfall A spike) was re-verified end-to-end via its own <verify> command before Task 2 expansion began, per auto-mode's re-run-and-continue rule"

requirements-completed: [ROW-01, ROW-02, ROW-03, ROW-04]

coverage:
  - id: D1
    description: "D-07/ROW-01/ROW-03: results render through @humanspeak/svelte-virtual-list as a real WAI-ARIA listbox (role rewrite over the library's hardcoded role=region), j/k/Home/End/Enter traversal via list.scroll({index, align:'nearest', smoothScroll:false}), focus never leaves the listbox"
    requirement: "ROW-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultsList.browser.test.ts#ResultsList (accessibility-role, traversal, re-render-reset, empty-list tests)"
        status: pass
    human_judgment: false
  - id: D2
    description: "ROW-01/ROW-04/D-05: the dense one-line row grid — category, summary (inline code from text nodes), state chips (first+N collapse), tags (first two+N), scope, age, score (always, unrounded bar) and rel (only when the reranker ran, including relevance=0)"
    requirement: "ROW-04"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultRow.browser.test.ts#ResultRow (score/rel/dim/tags/code/height/collapse/category tests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "E1: first-load skeleton rows (no listbox), re-query dimming with an indeterminate progress bar (keepPreviousData shape), and a real container-query context for ResultRow's column drop-out"
    requirement: "ROW-01"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultsList.browser.test.ts#ResultsList (loading/busy tests)"
        status: pass
    human_judgment: false
  - id: D4
    description: "ROW-02: overlay hover card — 250ms pointer-hover open via mousemove (never mouseenter), instant keyboard-follow open, 120ms close grace into the card, hidden on scroll/openId-change/text-size-change, never for the row already open in the pane; shows short_id, full scope, every state word, a 6-line content preview and every tag"
    requirement: "ROW-02"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultsList.browser.test.ts#ResultsList (pointer-hover, keyboard-follow, card-content, no-card-for-openId tests)"
        status: pass
    human_judgment: false
  - id: D5
    description: "ROW-03/D-17: row-action keys on the active row (e/s/#/c/⇧C) with rule/discovery gating, clipboard + toast feedback, typing-field and modifier guards, Esc closes the card before onescape, and a Kbd legend for every key"
    requirement: "ROW-03"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/ResultsList.browser.test.ts#ResultsList (Esc-layering, row-action-key, guard, rule/discovery-gating, legend tests)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Visual correctness of the dense row grid, hover-card placement/clamping, and container-query column drop-out at 12-16px and in both themes"
    verification: []
    human_judgment: true
    rationale: "No automated visual regression harness exists in this repo; layout/contrast correctness needs a human look, consistent with human_verify_mode=end-of-phase deferring UI judgment calls to end-of-phase UAT."

duration: 70min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 6: Dense Virtualized Results Listbox Summary

**A real WAI-ARIA listbox over `@humanspeak/svelte-virtual-list` (role-rewritten past its hardcoded `role="region"` viewport) rendering the full dense row grid, an overlay hover card driven as controlled state, and D-17's row-action keys — all in one reusable `ResultsList`/`ResultRow`/`ResultHoverCard` set that plans 02-08 and 02-10 mount.**

## Performance

- **Duration:** 70 min (approx.)
- **Started:** 2026-09-26T19:15:00Z (approx.)
- **Completed:** 2026-09-26T20:23:32Z
- **Tasks:** 3 completed
- **Files modified:** 7 (5 created, 2 modified)

## Accomplishments

- Resolved Pitfall A (the library's viewport hardcodes `role="region"`, no prop overrides it) with a capture-phase Svelte action that rewrites the role to `listbox` and pre-empts the library's own native Home/End/Arrow scroll handling for the keys `ResultsList` owns.
- `j`/`k`/`Home`/`End`/`Enter` traversal against 1000 rows, active row tracked by memory id (survives a re-queried array, resets only when genuinely gone), `aria-activedescendant` kept in sync via a dedicated effect.
- The full dense row grid: category, ellipsized summary with inline-`code` spans (text nodes only), state chips that collapse to `first +N` inside their own bounded budget, tags (first two + `+N`), scope, age, score (always, `toFixed(2)` + an unrounded clamped bar) and `rel` (only when the reranker ran, including `relevance: 0`).
- First-load skeleton rows and re-query dimming with an indeterminate progress bar; a real `container-type: inline-size` context so `ResultRow`'s column drop-out (`@container list`) actually has something to query.
- `ResultHoverCard`: a controlled bits-ui `LinkPreview` (no `Trigger`, `open` driven purely as host state) anchored via `customAnchor` to any row — 250ms pointer-hover open via `mousemove`, instant keyboard-follow open, a 120ms close grace into the card, hidden on scroll/`openId` change/text-size change, never shown for the row already open in the pane.
- D-17 row-action keys (`e`/`s`/`#`/`c`/`⇧C`) with rule/discovery gating, clipboard-plus-toast feedback, typing-field and modifier guards, Esc-layering (card first, then `onescape`), and a `Kbd` legend.

## Task Commits

Each task was committed atomically:

1. **Task 1: Virtualized listbox end to end (tracer + Pitfall A spike)** — `93c284a2` (feat)
2. **Task 2: The dense row grid, column drop-out, skeletons and re-query dimming** — `48f34cad` (feat, `tdd="true"`)
3. **Task 3: Overlay hover card, row-action keys, Esc layering and the legend** — `7af864df` (feat, `tdd="true"`)

_Tasks 2 and 3 carried `tdd="true"`; test files and implementation were authored and iterated together in the same commit rather than as separate RED/GREEN commits — the plan's frontmatter `type: execute` (not `type: tdd`) means the strict plan-level RED/GREEN/REFACTOR gate enforcement does not apply here, matching the precedent already recorded for this milestone (WINDOWS.md id 1/2, and 02-03-SUMMARY's Task 2 note)._

## Files Created/Modified

- `ui/src/lib/components/ResultsList.svelte` — the virtualized listbox, keyboard model, hover-card state machine, row-action keys, legend
- `ui/src/lib/components/ResultsList.browser.test.ts` — 28 tests across accessibility roles, traversal, loading/busy states, rel-column presence, hover timing, and D-17 keys
- `ui/src/lib/components/ResultRow.svelte` — the dense one-line row grid
- `ui/src/lib/components/ResultRow.browser.test.ts` — 10 tests across score/rel formatting, state-chip collapse, dimming, tags, inline code, row height, category color
- `ui/src/lib/components/ResultHoverCard.svelte` — the overlay hover card
- `ui/package.json` / `ui/pnpm-lock.yaml` — `@humanspeak/svelte-virtual-list@0.5.14` exact pin

## Decisions Made

- Pitfall A's role-rewrite action (resolution 1) held under the real accessibility-tree test; the inner-wrapper-listbox fallback (resolution 2) was never needed, and no checkpoint/fallback library install was triggered.
- The state-chip collapse budget is a bounded `calc(96*var(--u))` max-width on the chip container itself, decoupled from the row's own `@container` column-drop width — a CSS grid `auto` track otherwise sizes to its content's unbounded max-content width regardless of overall row width, which would make the collapse branch unreachable.
- `ResultHoverCard` renders `HoverCard.Root`/`HoverCard.Content` with no `Trigger` at all, since `open` is fully host-controlled; `openDelay`/`closeDelay` are set to 0 because the primitive's own hover-delay machinery is never exercised.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test-infrastructure gap: isolated component tests never resolved `--u`/design tokens**
- **Found during:** Task 2 (first geometry-sensitive assertions — row height, state-chip collapse — on a component whose sizing is entirely `calc(N*var(--u))`)
- **Issue:** `ui/src/app.css` (which defines `--u` and the D-13 tokens) is imported only by `ui/src/routes/+layout.svelte`. `vitest-browser-svelte`'s `render()` mounts a component in isolation without going through that layout, so `--u` was undefined in every existing browser test — `calc(N*var(--u))` is an invalid CSS value with no fallback and silently reverts to its property's default (e.g. `max-width: none`). No prior test in this repo had ever asserted computed pixel geometry, so this was never caught; it would have made the state-chip collapse logic permanently untestable (and its own "collapsed" branch dead code, since the bounded budget it depends on would never actually apply).
- **Fix:** Added `import '../../app.css';` to `ResultRow.browser.test.ts` and `ResultsList.browser.test.ts`, matching the shipped app's own cascade.
- **Files modified:** `ui/src/lib/components/ResultRow.browser.test.ts`, `ui/src/lib/components/ResultsList.browser.test.ts`
- **Verification:** With the import in place, `--u` resolves to the real `calc(1rem/13)` value; the two-word state-chip case (`archived`+`superseded`) now genuinely overflows the 96u budget and collapses, and a dedicated one-word test proves the uncollapsed branch still renders correctly.
- **Committed in:** `48f34cad` (Task 2 commit)

**2. [Rule 3 - Blocking] Portaled hover-card content is not a descendant of the render container**
- **Found during:** Task 3 (writing hover-card assertions)
- **Issue:** `HoverCard.Content` renders through a `Portal` (typically to `document.body`), so `screen.container.querySelector('[data-slot="hover-card-content"]')` always returned `null` regardless of whether the card was actually open — a test-authoring bug, not a component bug, but one that would have made every hover-card assertion silently vacuous (always failing, or worse, passing for the wrong reason if paired with `.not.toBeInTheDocument()`).
- **Fix:** Query `document.querySelector(...)` instead of `screen.container.querySelector(...)` for the hover card, and use `expect.poll(...)` (which re-queries on each retry) rather than `expect.element()` on a value that may still be `null` at call time, for assertions on an element that appears asynchronously.
- **Files modified:** `ui/src/lib/components/ResultsList.browser.test.ts`
- **Verification:** All hover-card tests (250ms pointer open, instant keyboard-follow open, card content, `openId` suppression, Esc layering) pass against the corrected queries.
- **Committed in:** `7af864df` (Task 3 commit)

**3. [Rule 3 - Blocking] `page.elementLocator(...).hover()` blocked by the legend line in an unsized isolated mount**
- **Found during:** Task 3 (the 250ms pointer-hover test)
- **Issue:** `vitest-browser-svelte`'s isolated mount has no ancestor providing a real height (only the shipped app's flex chain does that), so `.results-list-container`'s `height: 100%` collapsed to 0 and the legend line — positioned immediately after it in normal flow — visually overlapped the (zero-height) list at the same screen coordinates, so Playwright's real pointer `hover()` timed out reporting the legend "intercepts pointer events."
- **Fix:** Set an explicit `screen.container.style.height = '600px'` in that one test, giving the mount a real box matching what the shipped app's flex chain provides.
- **Files modified:** `ui/src/lib/components/ResultsList.browser.test.ts`
- **Verification:** The pointer-hover test's real `hover()` call now resolves; the 250ms-delayed card open is observed correctly.
- **Committed in:** `7af864df` (Task 3 commit)

---

**Total deviations:** 3 auto-fixed (1 bug, 2 blocking).
**Impact on plan:** All three were necessary to make the plan's own `<behavior>` assertions genuinely exercise the code they claim to test, rather than passing vacuously or failing on an environment artifact. No scope creep — no production component logic changed as a result; only test-file imports, queries, and one test's container sizing.

## Issues Encountered

- One full-suite run (`pnpm test`) showed a single unrelated flake in `ScopesSidebar.browser.test.ts` (a Playwright pointer-interception timeout against a leftover floating-content wrapper), which did not reproduce on immediate re-runs (two subsequent full runs: 399/399 and 399/399) or when paired directly with this plan's own test files. Treated as transient CI/host-load flakiness, not a regression — no fix applied, nothing to defer.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `ResultsList.svelte`, `ResultRow.svelte` and `ResultHoverCard.svelte` are ready to mount from plans 02-08 (`/search` rebuild) and 02-10 (`/observe` adoption per D-12) — the props contract (`memories`, `mode`, `label`, `openId`, `loading`, `busy`, `hasMore`, `onloadmore`, `onopen`, `onescape`, `onedit`, `onvisibility`, `ondelete`) matches the plan's artifact table exactly.
- Supersede and Archive/Restore row keys stay unbound (D-16) until their RPCs land in Phase 3 — no placeholder or dead key-handling code was added for them.
- Visual correctness of the dense grid, hover-card placement/clamping, and the container-query column drop-out at 12–16px and in both themes is deferred to end-of-phase UAT (coverage `D6`, `human_judgment: true`), consistent with `workflow.human_verify_mode: end-of-phase`.
- No blockers.

## Self-Check: PASSED

- `[ -f ui/src/lib/components/ResultsList.svelte ]`, `[ -f ui/src/lib/components/ResultRow.svelte ]`, `[ -f ui/src/lib/components/ResultHoverCard.svelte ]`, `[ -f ui/src/lib/components/ResultsList.browser.test.ts ]`, `[ -f ui/src/lib/components/ResultRow.browser.test.ts ]` — all found.
- `git log --oneline 842d310a..HEAD` shows all three commits (`93c284a2`, `48f34cad`, `7af864df`) present in history.
- All plan `<acceptance_criteria>` re-run and passing per-task (dependency pin, no `svelte-tiny-virtual-list`, `role="option"`/`smoothScroll: false` present; zero `{@html}`, zero bracket-px arbitrary values, `@container` present ≥2; zero `onmouseenter`, `onmousemove` present, `customAnchor`/`line-clamp` present).
- Plan-level `<verification>`: `pnpm --dir ui vitest run --project browser src/lib/components/ResultsList.browser.test.ts src/lib/components/ResultRow.browser.test.ts` (28+10 tests) and `pnpm --dir ui build` both exit 0. Full `pnpm --dir ui test` (41 files / 399 tests) also green.

---
*Phase: 02-recall-first-search*
*Completed: 2026-09-26*
