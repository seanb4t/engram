---
sketch: 005
name: tag-entry-points
question: "How do tag popularity (bars) and tag autocomplete enter /search without crowding the facet strip?"
winner: "C"
tags: [tags, list-tags, facet-strip, autocomplete, header-search, honest-feedback, phase-05]
---

# Sketch 005: Tag entry points

## Design Question

Phase 5 (05-CONTEXT D-12..D-19) locks linear tag bars, a `#`/`tag:` group in the header search,
a "+ tag" picker, and honest copy when the ListTags list is capped. What it leaves open is how
those land on `/search`: two controls, one merged control, or a docked panel.

## How to View

open .planning/sketches/005-tag-entry-points/index.html

Nav bar switches: **ListTags** state (`more=true` / `complete` / `loading` / `no tags`) and
**scope** (all readable vs one scope chip). Theme, viewport and text size are in the toolbar.

## Variants

- **A: Tags popover + "+ tag" picker.** Two controls: a compact autocomplete picker on the left of
  the strip, and a "▦ Tags" button on the right opening the bars popover (top 30, show all,
  filter box).
- **B: One "+ tag" picker.** Its empty state is the top-30 bars; typing turns it into ranked
  matches. It stays open for several adds (↵ toggles, esc closes). There is no separate Tags
  button.
- **C: Docked Tags panel + "+ tag" picker.** The bars sit in a right panel that shares the slot
  with the detail pane; selected tags stay marked ● while reading results. It becomes a bottom
  sheet below 760px.

All three share the header search Tags group (type `#qd`, `tag:rel`), the unknown-tag row, and
the scope-following counts.

## What to Look For

- Does B's merge (bars as the picker's empty state) read as one tool, or does it hide the
  popularity view behind "+ tag"?
- Is A's right-aligned "▦ Tags" discoverable next to the chips?
- Does C's panel earn its width, given the detail pane wants the same slot?
- Honesty copy: ListTags returns **no total**, so the sketch says "more exist beyond 1,000",
  never "of 1,340".
- **Data gap to note:** the rarity tooltip (`ln(n/df)`, D-15) needs `n`. The sketch assumes the
  recall-visible record count comes from ListScopes; confirm, or drop rarity from the `/search`
  tooltip and keep it only on `/related` tag-edge evidence.
- **Simplification the sketch assumes:** one ListTags(limit=1000) fetch per scope key feeds the
  header group, the picker and the bars. The bars show the first 30 and "show all" reveals the
  rest with no second request.
