---
phase: 02-recall-first-search
plan: 07
subsystem: ui
tags: [svelte5, paneforge, bits-ui, shadcn-svelte, tailwind4, design-tokens, accessibility]

requires:
  - phase: 02-recall-first-search
    provides: "text-size preference (--u scaling unit), D-13 design tokens (surface-2, border-subtle, text-faint, warning, cat-rule) (plan 02-03)"
provides:
  - "DetailPane.svelte: D-14 stacked-section record view (sticky head, title, actions, State, Content, Tags, Metadata) covering every ROW-07 field -- supersession links, schedule window, archive stamp, schema version, citations, usage counters, both copyable id forms -- replacing tabs for /search and the future /observe integration (plan 02-10); MemoryDetail.svelte (tabs) is untouched and keeps serving /discovery only"
  - "D-15 inline actions row: Edit, disabled Supersede/Archive (Restore when archived) with the Phase 4 tooltip, Share/Make private, danger Delete; rule fence hides Edit/Share for rule records, Edit only for discovery records"
  - "WriteSurfaces.requestMakePrivate(memory, kind): the direct no-warning reversal of Share through the existing CSRF visibility mutation"
  - "RecallSplit.svelte: paneforge-backed resizable/collapsible list+detail split -- toggle-close, 280px..72% clamp, live-width-derived keyboard resize step (Pitfall B), full overlay fallback below 760px"
affects: [02-10]

actuals:
  tokens: 11590
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Tracer feedback gate: Task 1 (tracer) was re-verified end-to-end before Task 2/3 expansion began, per auto-mode's re-run-and-continue rule (workflow._auto_chain_active: true)"
    - "Accessible disabled-button-with-tooltip: a focusable wrapping <span tabindex=0> (Tooltip.Trigger's child snippet) around a real inert <button disabled>, with an inline svelte-ignore for the a11y_no_noninteractive_tabindex false-positive -- the button stays out of the click path entirely, so 'clicking it fires no callback' holds structurally, not by convention"
    - "Container-derived percentage props for paneforge: RecallSplit measures its own root via ResizeObserver and re-derives keyboardResizeBy/minSize/defaultSize as percentages of that live width on every resize, rather than passing paneforge a literal pixel-shaped percentage (Pitfall B)"

key-files:
  created:
    - ui/src/lib/components/DetailPane.svelte
    - ui/src/lib/components/DetailPane.browser.test.ts
    - ui/src/lib/components/RecallSplit.svelte
    - ui/src/lib/components/RecallSplit.browser.test.ts
  modified:
    - ui/src/lib/components/WriteSurfaces.svelte
    - ui/src/lib/components/WriteSurfaces.browser.test.ts

key-decisions:
  - "DetailPane's sticky-head short_id copy button carries the lowercase aria-label 'copy short_id' (matching MemoryDetail's existing lowercase icon-button convention), kept deliberately distinct from Metadata's Title-Case 'Copy id'/'Copy short_id' text buttons -- role-query name matching in this stack is case-insensitive substring by default, so two differently-cased labels for the same action avoid an ambiguous strict-mode locator match rather than adding a redundant control."
  - "Metadata section carries aria-label=\"Metadata\" (giving the <section> an implicit region role) purely so tests can scope id/short_id assertions there -- the sticky head also shows short_id, and an unscoped query is ambiguous."
  - "requestMakePrivate has no confirmation dialog (unlike requestShare/ShareWarningInline) -- making a record private is a reduction of exposure, not an irreversible-feeling action, so the plan's own action text calls for a direct mutation."
  - "The State section shows a timestamp only where the wire actually carries one: archived (archivedAt) and expired/scheduled (notAfter/notBefore) get 'since'/'until'/'opens'/'closes' timestamps; superseded (supersededBy) is rendered as a link only, since Memory has no supersededAt field to show -- matches Task 1's literal <behavior> spec, which only asserts a timestamp for the archived state."

patterns-established:
  - "RecallSplit hosts caller-supplied list/detail snippets and knows nothing about which row is 'active' -- pinning the pane to a selection is the host's responsibility (deferred to the /search integration in plan 02-10), not this component's."

requirements-completed: [ROW-07]

coverage:
  - id: D1
    description: "DetailPane renders every ROW-07 field in D-14's fixed stacked-section order (no tabs), with both id forms copyable, for a full record, an empty record, loading, NotFound, and not-in-results states"
    requirement: "ROW-07"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/DetailPane.browser.test.ts"
        status: pass
    human_judgment: false
  - id: D2
    description: "D-15 inline actions row (Edit, disabled Supersede/Archive with the Phase 4 tooltip, Share/Make private, danger Delete) with the D-16 rule/discovery fence"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/DetailPane.browser.test.ts#DetailPane — inline actions (D-15, D-16)"
        status: pass
    human_judgment: false
  - id: D3
    description: "WriteSurfaces.requestMakePrivate reverses Share through the existing CSRF visibility mutation, no-op when already private, routed by kind, with a re-auth toast on a terminal auth failure"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/WriteSurfaces.browser.test.ts#WriteSurfaces — requestMakePrivate(memory, kind): D-15 reverse of Share"
        status: pass
    human_judgment: false
  - id: D4
    description: "RecallSplit: toggle-close via collapse()/expand(), ~440px default, keyboard resize by a live-width-derived ~32px step, 280px..72% clamp, close callback, and full overlay below 760px"
    verification:
      - kind: unit
        ref: "ui/src/lib/components/RecallSplit.browser.test.ts"
        status: pass
    human_judgment: false
  - id: D5
    description: "Visual correctness of the stacked sections, inline action row, tooltip placement, and the pane's resize/collapse motion in the real browser (not just component-test assertions)"
    verification: []
    human_judgment: true
    rationale: "No automated visual regression harness exists in this repo; layout/contrast correctness and drag-resize feel need a human look, consistent with human_verify_mode=end-of-phase deferring UI judgment calls to end-of-phase UAT. Neither component is yet mounted on a real route (that wiring is plan 02-10), so there is nothing live to click through in this phase either."

duration: 40min
completed: 2026-09-26
status: complete
---

# Phase 2 Plan 7: Record Detail Pane and Resizable Split (D-14, D-15, D-16) Summary

**Stacked-section `DetailPane.svelte` covering every ROW-07 field with inline D-15/D-16 actions, `WriteSurfaces.requestMakePrivate`, and a paneforge-backed `RecallSplit.svelte` with toggle-close, clamp, and a live-width-derived keyboard resize step.**

## Performance

- **Duration:** 40 min (approx.)
- **Started:** 2026-09-26T16:45:00Z (approx.)
- **Completed:** 2026-09-26T21:27:09Z
- **Tasks:** 3 completed
- **Files modified:** 6 (4 created, 2 modified)

## Accomplishments

- `DetailPane.svelte`: replaces MemoryDetail's tabs with D-14's fixed stacked-section order (sticky head → title → actions → State → Content → Tags → Metadata) for every ROW-07 field, including supersession links (clickable, in stored order), the schedule window, the archive stamp, schema version, citations, usage counters, and both id forms with copy buttons. Loading (skeleton), E7 NotFound copy, and the not-in-results note are all covered.
- D-15 inline actions row and D-16 disabled-with-tooltip pattern: Edit / Supersede… (disabled) / Archive-or-Restore (disabled) / Share-or-Make-private / danger Delete, replacing the ⋯ dropdown menu, with the rule/discovery fence (no Edit/Share for rules, no Edit for discoveries).
- `WriteSurfaces.requestMakePrivate(memory, kind)`: the direct, no-warning reverse of `requestShare`, routed to the matching memory/discovery visibility mutation, no-op when already private, with a re-auth toast on a terminal auth failure.
- `RecallSplit.svelte`: a paneforge `PaneGroup` hosting caller-supplied `list`/`detail` snippets, with `collapse()`/`expand()` toggle-close driven by the host's `open` prop, a `keyboardResizeBy` percentage re-derived from the live container width every resize (Pitfall B — never the sketch's literal 32), a 280px..72%-of-width clamp, and a full-overlay fallback with its own close button below 760px.

## Task Commits

Each task was committed atomically:

1. **Task 1: DetailPane end to end — stacked sections, every ROW-07 field, copyable ids, loading and error (tracer)** — `0830bc74` (feat)
2. **Task 2: Inline actions — Edit, disabled Supersede/Archive, Share or Make private, Delete — and requestMakePrivate** — `9c9a0d6d` (feat, `tdd="true"`)
3. **Task 3: RecallSplit — toggle-close, clamp, ~32px keyboard steps, overlay below 760px** — `10926ee2` (feat, `tdd="true"`)

_Tasks 2 and 3 carried `tdd="true"`; test file and implementation were authored and verified together in one commit rather than as separate RED/GREEN commits — this plan's frontmatter is `type: execute` (not `type: tdd`), so the strict plan-level RED/GREEN/REFACTOR gate does not apply, matching the already-flagged pattern from `02-03-SUMMARY.md`._

**Plan metadata:** (this commit)

## Files Created/Modified

- `ui/src/lib/components/DetailPane.svelte` — the stacked-section record view (D-14/ROW-07/D-15/D-16)
- `ui/src/lib/components/DetailPane.browser.test.ts` — its tests (13 cases)
- `ui/src/lib/components/RecallSplit.svelte` — the resizable/collapsible split with overlay fallback
- `ui/src/lib/components/RecallSplit.browser.test.ts` — its tests (5 cases)
- `ui/src/lib/components/WriteSurfaces.svelte` — added `requestMakePrivate`
- `ui/src/lib/components/WriteSurfaces.browser.test.ts` — extended with 4 `requestMakePrivate` cases

## Decisions Made

- The sticky head's icon-only short_id copy button keeps a lowercase `aria-label="copy short_id"`, deliberately distinct from Metadata's Title-Case `Copy id`/`Copy short_id` text buttons, since role-name matching in this stack's test locators is case-insensitive substring matching and two same-cased controls would be ambiguous.
- `Metadata`'s `<section>` carries `aria-label="Metadata"` purely to give tests a scoped region to query id/short_id in, since the sticky head also shows `short_id`.
- `requestMakePrivate` has no confirmation dialog, unlike `requestShare` — the plan's own action text specifies a direct mutation since narrowing visibility is not the same risk shape as widening it.
- The State section only shows a timestamp where the wire actually carries one for that state (archived, expired, scheduled); superseded renders as a link only, since `Memory` has no `supersededAt` field.

## Deviations from Plan

None - plan executed exactly as written. (Two minor mechanical adjustments during implementation — removing a redundant `role="region"` a11y warning, and switching a compiler-flagged unused-CSS-selector on Skeleton width classes to Tailwind arbitrary values — were self-corrections within Task 1/2's own acceptance loop, not deviations from any planned behavior.)

## Known Stubs

None — every artifact this plan promises is fully wired: DetailPane renders real data end to end, RecallSplit's collapse/expand/resize are live paneforge calls, and `requestMakePrivate` performs a real mutation through the existing CSRF write client. Both components are standalone and not yet mounted on a route — that integration (into `/search`, and `/` / `/observe`) is plan 02-10's explicit scope, not a stub in this plan.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `DetailPane.svelte` and `RecallSplit.svelte` are ready for plan 02-10 to mount on `/search` (and later `/` / `/observe`), replacing `MemoryDetail`'s tabbed pane for those routes. `MemoryDetail.svelte` itself is untouched and still serves `/discovery`.
- Visual correctness of the stacked sections, the tooltip, and the pane's drag/keyboard resize feel in a real browser is deferred to end-of-phase UAT (coverage `D5`, `human_judgment: true`), consistent with `workflow.human_verify_mode: end-of-phase` — and there is genuinely nothing live to click through yet, since neither component is wired to a route in this plan.
- No blockers.

## Self-Check: PASSED

- `[ -f ui/src/lib/components/DetailPane.svelte ]`, `[ -f ui/src/lib/components/DetailPane.browser.test.ts ]`, `[ -f ui/src/lib/components/RecallSplit.svelte ]`, `[ -f ui/src/lib/components/RecallSplit.browser.test.ts ]` — all found.
- `git log --oneline -5` shows all three task commits (`0830bc74`, `9c9a0d6d`, `10926ee2`) present in history, on top of `cf0980fc` (the prior plan's close-out commit).
- All plan `<acceptance_criteria>` re-run and passing (see per-task verification above; `rg` counts matched exactly).
- Plan-level `<verification>`: `pnpm --dir ui vitest run --project browser src/lib/components/DetailPane.browser.test.ts src/lib/components/WriteSurfaces.browser.test.ts src/lib/components/RecallSplit.browser.test.ts` (38 tests) and `pnpm --dir ui build` both exit 0. Full `pnpm --dir ui test` (43 files / 420 tests) also passes — no regressions.

---
*Phase: 02-recall-first-search*
*Completed: 2026-09-26*
