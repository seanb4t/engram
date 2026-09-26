---
sketch: 004
name: related-and-tags
question: "How should a memory's related neighbourhood and the store's tag popularity be visualised so edge types stay distinct and the views stay useful, not decorative?"
winner: "Synthesis"
tags: [related-memories, graph, tag-cloud, edges, dataviz]
---

# Sketch 004: Related & Tags

## Design Question

How should a memory's related neighbourhood (`RelatedMemories`: supersession, citation, tag and vector edges, merged into one entry per candidate) and the store's tag popularity (`ListTags`) be visualised so edge types stay distinct and the views stay useful, not decorative?

## How to View

```bash
open .planning/sketches/004-related-and-tags/index.html
```

Use the **state** control in the tab bar to cycle populated, loading, no neighbours and truncated (k=64, 64-entry ceiling). The toolbar at bottom-right switches theme (default/light), viewport (375/768/1280/full) and annotation mode (design notes plus a style inspector).

## Variants

- **A: Radial graph + tag cloud.** The anchor sits in the centre. Citation, tag and vector neighbours sit on rings, and each candidate goes on the ring of its first edge type. The supersession chain is a vertical directed path. Hover shows the evidence and click re-centres. A log-scaled tag cloud filters the graph.
- **B: Edge-type lanes + tag bars.** There is no graph. Each edge type gets a lane of one-line rows with its evidence inline, and the supersession chain is a horizontal timeline. An `×N` badge links a candidate's appearances in other lanes. Tag popularity is a sorted, linear bar list with a search box.
- **C: Split graph + evidence, tag treemap.** A compact force graph handles navigation (click selects, double-click or Enter re-centres, `j`/`k` move). The panel lists the selected neighbour's evidence per edge type. Tags are a squarified treemap sized by count, with an unsized `more` control.
- **Synthesis: B lanes + C graph/evidence** (default tab). B's edge-type lanes are the main body, with the supersession chain as a horizontal timeline and per-lane hide toggles and `n/8` counts. A right rail has two tabs: Graph (C's compact graph as a linked overview) and Tags (B's bar list). The rail remembers the last tab. One selection is shared by the lanes and the graph: select a row or node and every appearance lights up (`×N` is a count, not a jump), and C's evidence panel slides over the rail as a drawer. Keys: `j`/`k` move, `Enter` or double-click re-centres, `[` goes back, `g` switches Graph/Tags, `Esc` closes the drawer. Below 900px the drawer becomes a bottom sheet.

## What to Look For

1. **Can you name an edge type without colour?** Edges are grey and encode type by line style: solid with an arrow for supersession, dashed for citation, dotted for tag, thin for vector. Glyphs repeat the same border styles. Check whether this holds up in A's multi-line fan and in C's small graph, or whether only B's labelled lanes make it obvious.
2. **Multi-edge candidates.** A draws one curve per type, so a candidate with three edge types gets three curves. B repeats the row in each lane and links the copies with a badge. C shows one node and three evidence blocks. Which one tells you why a record is related fastest?
3. **The truncated state.** At 64 entries, A's vector ring turns into a wall of spokes. B collapses to 8 rows plus "show all". C crowds the graph, but the list stays usable. Does any graph survive the ceiling?
4. **Placement by first edge empties the outer ring.** Most vector hits also share a tag or citation, so in A they land on inner rings. The vector ring shows only vector-only records. Is that honest, or is it confusing?
5. **Tag views and rarity.** The cloud (log-scaled), the bars (linear) and the treemap (area) disagree about how dominant `engram` (400) is. Tag-edge weight is rarity, ln(n/df), so the small tags make the strong edges. Which view helps someone pick a filter, and which one is decoration?

## Selection

Winner: Synthesis — B's edge-type lanes as the main body, a right rail with Graph (C's compact graph as a linked overview) and Tags (B's bars), one shared selection across lanes and graph, and C's evidence panel as a drawer on selection. State chips never overflow: they collapse to `first +N`.

State chips sit along the bottom of cards and nodes. Text size follows the console-wide display preference (12–16px, default 15, shared via themes/display.js).

State words keep the canonical order archived › superseded › expired › scheduled. The full list is in the chip's tooltip. The fixture marks `tr5wa372rr` as archived, superseded and expired, so a 180px chain card shows `archived +2`.
