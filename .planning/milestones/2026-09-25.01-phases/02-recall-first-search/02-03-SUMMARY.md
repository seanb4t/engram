---
phase: 02-recall-first-search
plan: 03
subsystem: ui
tags: [svelte5, shadcn-svelte, bits-ui, tailwind4, design-tokens, accessibility]

requires:
  - phase: 02-recall-first-search
    provides: existing app.css tokens, AppShell header, +layout.svelte shell (plan 02-01)
provides:
  - "Site-wide text-size preference (D-13): display.svelte.ts store module, engram.console.textSize localStorage key, engram:textsize event, --ui-font/--u CSS scaling unit"
  - "Aa Display popover in the app header (shadcn Popover + bits-ui RadioGroup)"
  - "Nine new D-13 design tokens (surface-2, hover, selected, border-subtle, text-faint, primary-soft, warning, success, cat-rule) in :root/.dark, bridged in @theme"
  - "Every surviving app component free of fixed-px arbitrary Tailwind sizes"
affects: [02-04, 02-05, 02-06, 02-07, 02-08, 02-09, 02-10]

actuals:
  tokens: 17490
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Svelte 5 $state store module in a .svelte.ts file (display.svelte.ts) replacing the sketch's window.EngramDisplay singleton"
    - "Anti-flash <script> pattern extended: a second, independent, dependency-free inline script in app.html applies --ui-font/data-text-size before first paint, alongside the existing theme script"
    - "--u: calc(1rem / 13) scaling unit + html { font-size: var(--ui-font) } lets both rem-based Tailwind utilities and explicit calc(N*var(--u)) dimensions scale from one preference"
    - "Fixed-px control pattern: the Display popover's own segmented control intentionally uses literal px (never var(--u)) so it doesn't jump under the pointer while everything else scales"

key-files:
  created:
    - ui/src/lib/display.svelte.ts
    - ui/src/lib/display.test.ts
    - ui/src/app.html.test.ts
    - ui/src/lib/components/DisplayPopover.svelte
    - ui/src/lib/components/DisplayPopover.browser.test.ts
    - ui/src/lib/components/ui/popover/*.svelte (shadcn-svelte primitive, 8 files) + index.ts
  modified:
    - ui/src/app.html
    - ui/src/app.css
    - ui/src/app.css.test.ts
    - ui/src/routes/+layout.svelte
    - ui/src/lib/components/AppShell.svelte
    - ui/components.json
    - ui/src/lib/components/MigrationBanner.svelte
    - ui/src/lib/components/MemoryFormSheet.svelte
    - ui/src/lib/components/MemoryList.svelte
    - ui/src/lib/components/ScopesSidebar.svelte
    - ui/src/lib/components/DiscoveryFormSheet.svelte
    - ui/src/lib/components/DeleteConfirmDialog.svelte
    - ui/src/lib/components/MemoryRow.svelte
    - ui/src/lib/components/ShareWarningInline.svelte
    - ui/src/lib/components/MemoryDetail.svelte
    - ui/src/lib/components/ScopeChip.svelte
    - ui/src/routes/+page.svelte

key-decisions:
  - "installDisplayShortcuts' default notify parameter calls svelte-sonner's toast directly (per the plan's action text), so +layout.svelte can call installDisplayShortcuts(window) with no second argument and still get toast feedback"
  - "--u is expressed as calc(1rem / 13) rather than calc(var(--ui-font) / 13) — equivalent once html { font-size: var(--ui-font) } is set, and matches the plan's explicit artifact contract verbatim"
  - "The Display popover's segmented control and preview row deliberately use only pre-existing tokens (border, background, muted-foreground, primary/10) rather than forward-referencing Task 3's new tokens, since Task 2 lands before Task 3"

patterns-established:
  - "Tracer feedback gate: Task 1 (tracer) was verified end-to-end (both <verify> automated checks) before Task 2/3 expansion began, per auto-mode's re-run-and-continue rule"

requirements-completed: [ROW-01]

coverage:
  - id: D1
    description: "One site-wide text-size preference (12-16px, default 15) persisted in localStorage and applied on every route before first paint"
    requirement: "ROW-01"
    verification:
      - kind: unit
        ref: "ui/src/lib/display.test.ts"
        status: pass
      - kind: unit
        ref: "ui/src/app.html.test.ts"
        status: pass
    human_judgment: false
  - id: D2
    description: "⌘+/⌘-/⌘0 shortcuts step/reset the preference with toast feedback, ignored when Alt is held; cross-tab storage sync"
    verification:
      - kind: unit
        ref: "ui/src/lib/display.test.ts#installDisplayShortcuts"
        status: pass
    human_judgment: false
  - id: D3
    description: "Aa Display popover: 5-segment radiogroup, live scaled preview, shortcut hints, fixed-px control that does not jump under the pointer"
    verification:
      - kind: automated_ui
        ref: "ui/src/lib/components/DisplayPopover.browser.test.ts"
        status: pass
    human_judgment: false
  - id: D4
    description: "Nine new D-13 tokens land in both themes and bridge into @theme; --u scaling unit established"
    verification:
      - kind: unit
        ref: "ui/src/app.css.test.ts#console foundation tokens (D-13)"
        status: pass
    human_judgment: false
  - id: D5
    description: "Every surviving app component is free of fixed-px arbitrary sizes, so the preference reaches every route"
    verification:
      - kind: unit
        ref: "pnpm --dir ui test (full suite, 35 files / 312 tests green) + acceptance-criteria rg counts = 0"
        status: pass
    human_judgment: false
  - id: D6
    description: "Visual correctness of the Aa popover placement, live preview readability, and overall density at 12/13/14/15/16px across both themes"
    verification: []
    human_judgment: true
    rationale: "No automated visual regression harness exists in this repo; layout/contrast correctness at each of the five sizes and in both themes needs a human look, consistent with human_verify_mode=end-of-phase deferring UI judgment calls to end-of-phase UAT."

duration: 27min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 3: Console Design Foundations (D-13) Summary

**Site-wide text-size preference (12-16px, default 15) with an Aa Display popover, plus nine new design tokens and a scripted sweep replacing every fixed-px component dimension with `calc(N*var(--u))`.**

## Performance

- **Duration:** 27 min
- **Started:** 2026-09-26T18:05:00Z (approx.)
- **Completed:** 2026-09-26T18:30:08Z
- **Tasks:** 3 completed
- **Files modified:** 31 (17 created, 14 modified, per git diff-stat against the pre-plan commit)

## Accomplishments

- `display.svelte.ts` store module: `clampTextSize`/`readTextSize`/`setTextSize`/`stepTextSize`/`resetTextSize`/`installDisplayShortcuts`, backed by the `engram.console.textSize` localStorage key and the `engram:textsize` window event, never throwing on storage failure.
- A second, dependency-free anti-flash `<script>` in `app.html` applies `--ui-font`/`data-text-size` before first paint, alongside the untouched theme script.
- `app.css`: `--ui-font`/`--u` scaling tokens, `html { font-size: var(--ui-font) }`, body's hard-coded `13px` replaced by `1rem`, plus nine new D-13 tokens (`surface-2`, `hover`, `selected`, `border-subtle`, `text-faint`, `primary-soft`, `warning`, `success`, `cat-rule`) in both themes, bridged into `@theme`.
- `DisplayPopover.svelte`: an Aa trigger opening a `role="dialog"` popover with a fixed-px 5-segment `RadioGroup`, a scaled live preview row, shortcut hints, and the "Applies to every console page, not just this list." line — wired into `AppShell`'s header.
- A scripted `perl` sweep turned every fixed-px Tailwind arbitrary value across 12 surviving components/routes (plus `MemoryDetail`'s `<style>` block) into `calc(N*var(--u))`, so the preference reaches every route with zero fixed-px component dimensions left.

## Task Commits

Each task was committed atomically:

1. **Task 1: Text size end to end (tracer)** — `0364003b` (feat)
2. **Task 2: The Aa Display popover in the header** — `5a172f0e` (feat, `tdd="true"`)
3. **Task 3: New tokens and a scripted fixed-px sweep** — `8dbab4d8` (feat)

_Task 2 carried `tdd="true"`; test file and implementation were authored and verified together in one commit rather than as separate RED/GREEN commits — the plan's frontmatter `type: execute` (not `type: tdd`) means the strict plan-level RED/GREEN/REFACTOR gate enforcement does not apply here, but this is flagged for visibility since the milestone already tracks the same pattern as an open broken window (WINDOWS.md id 1/2, prior phase)._

## Files Created/Modified

- `ui/src/lib/display.svelte.ts` — the text-size preference store module
- `ui/src/lib/display.test.ts` — its unit tests (26 cases)
- `ui/src/app.html` / `ui/src/app.html.test.ts` — anti-flash script + its `node:vm` test
- `ui/src/app.css` / `ui/src/app.css.test.ts` — scaling tokens, D-13 tokens, extended coverage
- `ui/src/routes/+layout.svelte` — syncs the store on mount, installs shortcuts
- `ui/src/lib/components/DisplayPopover.svelte` / `.browser.test.ts` — the Aa popover
- `ui/src/lib/components/ui/popover/*` — shadcn-svelte primitive (generated)
- `ui/src/lib/components/AppShell.svelte` — wires in `DisplayPopover`
- `ui/components.json` — fixed `iconLibrary` value (see Deviations)
- 11 more components + `+page.svelte` — fixed-px → `calc(N*var(--u))` sweep

## Decisions Made

- `installDisplayShortcuts`'s default `notify` calls `toast` directly (per the plan's literal action text), letting `+layout.svelte` call `installDisplayShortcuts(window)` with no second argument.
- `--u: calc(1rem / 13)` (not `calc(var(--ui-font) / 13)`) — numerically identical once `html { font-size: var(--ui-font) }` is set, and matches the plan's artifact contract verbatim.
- Task 2's popover styling uses only tokens that existed before this plan (Task 3 lands after it), so no forward-reference to tokens not yet defined.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed a pre-existing `components.json` config bug blocking the required popover install**
- **Found during:** Task 2 (installing the shadcn-svelte popover primitive)
- **Issue:** `ui/components.json` had `"iconLibrary": "@lucide/svelte"`, which is not a valid shadcn-svelte `iconLibrary` enum value (valid values are `lucide`/`hugeicons`/`phosphor`/etc.). `pnpm exec shadcn-svelte add popover --yes` failed outright with a config validation error, blocking the task's required CLI command.
- **Fix:** Changed `iconLibrary` to `"lucide"` (the project already imports icons from `@lucide/svelte` everywhere; `lucide` is shadcn-svelte's own name for that icon set).
- **Files modified:** `ui/components.json`
- **Verification:** `pnpm exec shadcn-svelte add popover --yes` then succeeded; `git diff` confirms the only other change to `components.json` was this one line; `package.json`/`pnpm-lock.yaml` unchanged (acceptance criterion).
- **Committed in:** `5a172f0e` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking).
**Impact on plan:** Necessary to complete Task 2's required install step. No scope creep — no other config fields were touched, and a separate `style: "default" → nova` CLI warning (also pre-existing, unrelated to iconLibrary) was left alone since it didn't block the install and touches nothing this plan's acceptance criteria check.

## Known Stubs

None — every artifact this plan promises is fully wired (store, anti-flash script, popover, tokens, swept components).

## Issues Encountered

None beyond the components.json fix documented above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plans 02-04 through 02-10 can now size rows, cards, panes and chips in `calc(N * var(--u))` and colour them with the new token set, as the plan's objective intended.
- The Aa popover, shortcuts, and anti-flash script are all live in the shipped console — visual correctness across both themes and all five sizes is deferred to end-of-phase UAT (coverage `D6`, `human_judgment: true`), consistent with `workflow.human_verify_mode: end-of-phase`.
- No blockers.

## Self-Check: PASSED

- `[ -f ui/src/lib/display.svelte.ts ]`, `[ -f ui/src/lib/components/DisplayPopover.svelte ]`, `[ -f ui/src/lib/components/ui/popover/index.ts ]` — all found.
- `git log --oneline --all --grep="02-03"` — no plan-id grep convention was used in these commit messages (they follow `feat(ui): ...`), so verified instead via `git log --oneline -5` showing all three commits (`0364003b`, `5a172f0e`, `8dbab4d8`) present in history.
- All plan `<acceptance_criteria>` re-run and passing (see per-task verification above).
- Plan-level `<verification>`: `pnpm --dir ui test` (35 files / 312 tests) and `pnpm --dir ui build` both exit 0.

---
*Phase: 02-recall-first-search*
*Completed: 2026-09-26*
