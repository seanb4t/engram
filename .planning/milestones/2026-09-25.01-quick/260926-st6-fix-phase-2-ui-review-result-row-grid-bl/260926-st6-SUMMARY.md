---
quick_id: 260926-st6
status: complete
date: 2026-09-26
commits:
  - 335f9a49
  - d36359cb
  - 7ed1f786
  - 20a0e010
---

# Quick 260926-st6 Summary: Phase 2 UI-REVIEW blocker + coupled warnings

## What changed

- **Result-row grid (BLOCKER, `ui/src/lib/components/ResultRow.svelte`, 335f9a49).** Each
  `@container list` band now swaps `--cols` (the locked sketch pattern) instead of only hiding
  cells: ≤860px drops the tags track; ≤560px drops tags and scope, shrinks the category track to
  the dot (6u) and the score track to the number (40u, bar hidden). The summary track is
  `minmax(min(120u, 20cqi), 1fr)` and tags/scope are `minmax(0, <width>)`, so the optional
  tracks yield before the summary. Age, score and rel keep fixed tracks. A lone state chip now
  ellipsizes instead of hard-clipping when the state track yields. Breakpoints stay 860/560px.
- **Facet strip scroll cue (WARNING, `ui/src/lib/components/FacetStrip.svelte`, d36359cb).**
  `ScrollArea type="auto"` keeps the (slimmer) horizontal scrollbar visible whenever the strip
  overflows, not only on hover. A mask fades whichever edge still hides chips. The
  `data-fade-start` / `data-fade-end` attributes track the viewport's scroll position and size.
- **Score clip / horizontal scroll (WARNING).** Fixed by the grid change. The new layout test
  (7ed1f786) guards it.
- **Vendored SPA** rebuilt via `task ui:build` (20a0e010); a second rebuild is byte-identical.

## Tests added (each fails without the fix)

| Test | Pre-fix result |
|---|---|
| `ResultRow.browser.test.ts` › column drop-out by list width, at 1000/861/800/561/520/390px × 15px/16px text (rel on, state chip, tags, scope) | 11 of 12 fail (summary 0–21px, or row overflow 596–711px in a 390–561px list). 1000px@15px already passed. |
| `FacetStrip.browser.test.ts` › overflowing strip shows scrollbar + edge fade without hover | fails (no scrollbar rendered without hover) |
| `FacetStrip.browser.test.ts` › strip that fits shows neither | passes pre-fix (guards against an always-on cue) |
| `routes/search/search-layout.browser.test.ts` › 1024/900/860/760/620px viewport, pane open: no page or list horizontal scroll, summary ≥60px, score inside list | 5 of 5 fail (listbox scrollWidth 524 > 511/386/346; summary 0–5px) |

## Verification

- `pnpm --dir ui test`: 48 files, 532 tests passed
- `pnpm --dir ui build`: ok. `task ui:build`: ok, idempotent.
- Visual check: `/search` at 1024px with the pane open (dark) shows readable summaries and the
  score column, and fades the trailing edge of the facet strip.

## Known limits

- The worst case is the jev reranker on (rel column), a visible state chip, and 15–16px text
  at the narrow end of a band (for example, a 561px list). Here the state chip and scope share
  what remains after the summary floor, so the chip ellipsizes. Below about a 330px list at
  16px with rel on, the row still overflows by a few px.
- `pnpm --dir ui check` (svelte-check) fails before any code is checked: svelte-check 4.7.6
  requires TypeScript 6 alongside TypeScript 7. This is not caused by this task.
