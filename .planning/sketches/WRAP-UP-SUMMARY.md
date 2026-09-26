# Sketch Wrap-Up Summary

**Date:** 2026-09-26
**Sketches processed:** 4 (4 included, 0 excluded)
**Design areas:** Foundations, Recall surface, Curation, Related and tags
**Skill output:** `./.claude/skills/sketch-findings-engram/`

## Included Sketches

| # | Name | Winner | Design Area |
|---|------|--------|-------------|
| 001 | results-surface | B′ — fixed rows + 250ms overlay hover card + resizable right pane; second click on the open row closes the pane | Recall surface (+ Foundations: text size) |
| 002 | command-palette | B — top-bar inline search with anchored dropdown; Enter on free text opens /search | Recall surface |
| 003 | curation-dialogs | A — modal dialog with per-target validation chips, prefilled correcting record, live chain preview; small confirm for archive/restore | Curation |
| 004 | related-and-tags | Synthesis — B's edge-type lanes + right rail Graph (C's linked overview) \| Tags; shared selection; C's evidence drawer | Related and tags (+ Foundations: state chips) |

## Excluded Sketches

| # | Name | Reason |
|---|------|--------|
| — | — | None excluded |

## Design Direction

A dense developer tool in the Linear/Raycast mould. Recall owns the front door: one-line rows,
about 30 on screen, expanded by an overlay hover card rather than by pushing rows around, with a
pinned, resizable detail pane. The keyboard drives everything. Every entry point accepts a UUID,
a short_id or free text, says how it read the input, defaults to searching every readable scope,
and reports exactly what it searched and what the recall gate hid. Quiet GitHub-style neutrals,
one violet accent, category colours as the only other hue, monospace for every identifier and
number. Dark-first with a light theme.

## Key Decisions

- **Layout:** header search (anchored dropdown) → facet strip → honest results header →
  fixed-height listbox rows → right pane (paneforge) that toggles closed on a second click;
  column drop-out by container width. Related view = edge-type lanes plus a Graph | Tags rail
  with a slide-over evidence drawer. Curation in modal dialogs sized to risk.
- **Palette:** existing `app.css` tokens plus new surface-2, hover, selected, border-subtle,
  text-faint, primary-soft, warning, success, and the missing `--cat-rule`.
- **Typography:** sans prose, mono identifiers; one site-wide text size preference (12–16px,
  default 15, persisted per viewer, Aa popover in the header, `⌘+`/`⌘-`/`⌘0`).
- **Spacing:** 4px scale, 28px rows at 13px scaled by `--ui-font / 13`, radii 4/6/10px.
- **Interaction:** 0.15s transitions; 250ms hover delay, instant on keyboard focus; no
  flash-to-empty on re-query (keep previous results dimmed); `field=… hint=…` envelopes with
  selectable fix rows; state chips in canonical order, dim only past states, sit on the bottom
  of cards and nodes, and collapse to `first +N`; supersede says "No undo", archive offers Undo;
  edge types encoded by line style and glyph, never colour, never a blended score.
