---
sketch: 001
name: results-surface
question: "How should the dense result row, its hover/focus expansion, and the detail pane fit together on the recall-first /search page?"
winner: "B′"
tags: [results, search, dense-row, hover-expand, detail-pane, keyboard]
---

# Sketch 001: Results Surface

## Design Question

How should the dense result row, its hover/focus expansion, and the detail pane fit together on the recall-first /search page?

Fixed going in: one line per hit (summary, category, age, state words, scope chip, tags, score and optional `rel`), about 30 rows on screen, hover or keyboard focus reveals the first lines of content and the full tag set, a detail pane on the right, and a results header that says exactly what was searched.

## How to View

```bash
open .planning/sketches/001-results-surface/index.html
```

Keys: `j`/`k` or the arrow keys move, `Enter` opens (in C it focuses the detail), `Esc` closes, `/` focuses search. The State control in the top bar cycles populated, loading, empty and error. The toolbar at the bottom right switches the theme (default or light), the viewport (375/768/1280/full) and annotation mode.

## Variants

- **A: Inline expand + right pane.** The hovered or focused row grows in place and pushes the rows below it down. `Enter` or a click opens a resizable right pane (drag the handle, or focus it and use the left/right arrow keys).
- **B: Overlay hover card + right pane.** Rows never change height. A floating card anchored to the row shows the content preview and full tags, after 250ms on hover and at once on keyboard focus. The right pane works as in A.
- **C: Two-line row + split view.** Every row has a fixed, muted content snippet on a second line. The list and detail split 50/50, and the detail follows the active row. Expansion only reveals the full tag set.
- **B′: B + toggle-close + font size (★ Selected, opens by default).** B with two changes. A second click (or `Enter`) on the row already open in the pane closes the pane, and the list takes the width back over 0.15s. A click on any other row switches the pane to that record. The open row has a violet left bar and a tinted background. An **Aa** button in the header opens a Display popover with a text size control (12, 13, 14, 15 or 16 px; default 15) and a one-row preview. `⌘+` / `⌘-` step the size and `⌘0` resets it (Ctrl on other platforms). The shared `themes/display.js` module owns the value: it saves it in `localStorage` under `engram.console.textSize` and sets one variable, `--ui-font`, on `<html>`. All row heights, paddings, gaps, column widths, the hover card, the scope menu and the pane scale from it (A, B and C scale too). At a 900px-tall viewport, about 22 rows fit at 16px, 28 at 13px and 31 at 12px.

## What to Look For

1. **Scanning with `j`/`k`:** A's push-down moves the rows under your eye. B's card covers the next 5 or 6 rows. C shows nothing extra but fits about half as many rows. Which one lets you read about 30 hits fastest?
2. **Mouse travel:** in A, rows shift under a still pointer (hover waits 90ms before expanding). In B, the pointer can move into the card without closing it. Does either feel jumpy at real density?
3. **Pane behaviour:** in A and B the pane stays on the record you opened until you press `Enter` again (the opened row keeps a violet dot). In C the pane follows focus. Which fits recall best: pinned or follow-focus?
4. **Row density versus pane width:** open the pane in A or B and drag it wider. The tag column drops out below about 860px of list width and the scope column below 560px. Check that score, `rel` and the state words are still readable at 768px and 375px.
5. **Honest header and states:** toggle `cross_spine`, `include_*` and `jev rerank`, and type a query that matches nothing (for example `zzxq`). Check that "N hits across M scopes", "hidden by recall gate", `scopes_truncated` and "No memories match `q` in any scope you can read" each stay accurate. Also check that past-state rows (archived, superseded, expired) are dimmed and the future `scheduled` row is not.

## Selection

Winner: B′ — Variant B (fixed-height rows + 250ms overlay hover card + resizable right pane) with a second click on the open row closing the pane, and a console-wide text-size preference (12–16px, default 15, persisted per viewer) that scales the whole surface.

Text size follows the console-wide display preference (12–16px, default 15, shared across all pages via themes/display.js).
