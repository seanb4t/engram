---
sketch: 006
name: graph-interaction-density
question: "How does the /related rail graph feel on the keyboard, under pan/zoom and drag, and when the vector lane expands past its 8-row collapse?"
winner: "A1"
tags: [related-memories, graph, d3-force, keyboard, a11y, pan-zoom, drag, density, phase-05]
---

# Sketch 006: Graph interaction & density

## Design Question

Sketch 004 settled what the related graph shows. Phase 5 (05-CONTEXT D-06..D-11) settled the
edge set, the caps, the settle budget, the roving keyboard model and the SR list. This sketch
tests how it feels in the 340px rail: the keyboard focus look, whether pan/zoom controls sit
on the canvas, what drag does, and how the graph reads when the vector lane expands.

## How to View

open .planning/sketches/006-graph-interaction-density/index.html

This needs network: it loads the real d3 v7.9.0 from jsDelivr (pinned, with SRI), the same
APIs as the d3-force / d3-zoom / d3-drag / d3-selection micro-packages. The nav bar sets the
vector lane: collapsed (8 of 50) / expanded (50) / ceiling 64 (truncated). The toolbar can
reveal the visually hidden SR list and the aria-live line, and can simulate reduced motion.

## Variants

Round 1 picked **A**'s shape (corner controls, ⌘-scroll zoom, dashed focus ring and focus card,
drag springs back). It **rejected the evidence drawer** from sketch 004 / 05-CONTEXT D-05: a
node click slid the drawer over the rail and covered the node just clicked. Round 2 keeps A
and moves the evidence off the graph:

- **A1: evidence under the graph.** The evidence stacks directly beneath the graph in the rail,
  and the rail scrolls if needed. The graph stays whole.
- **A2: evidence inline in its lane.** The candidate's row expands in its lane (the lane you
  clicked, or its first lane when selected from the graph) and scrolls into view. The rail
  shows a one-line pointer ("evidence is open in the tag lane · jump"). Click the row again or
  press esc to close.
- **A3: evidence card beside the node.** A compact card on the top or bottom half of the graph,
  away from the selected node. The 340px rail is too narrow for a side card. It follows
  pan/zoom/drag.
- **B: No chrome · keyboard zoom · halo band.** The round-1 alternative, kept for reference
  (evidence under the graph, like A1). No on-canvas controls; `+` `−` `0` and a free wheel;
  solid focus ring and label pill; vector-only neighbours in an outer radial band; drag pins.

Shared: one Tab stop with roving `aria-activedescendant`; arrows move in lane order;
Space selects and opens the evidence drawer; Enter or double-click re-centres (trail, `[`
back); Escape clears the selection, then leaves the graph. Past 26 nodes only the anchor,
focused and selected nodes are labelled, and nodes shrink.

## What to Look For

- Does ⌘-scroll (A) or free wheel zoom (B) fit a sticky rail inside a scrolling page?
- Is the focus card (A) worth covering neighbours, or is the pill (B) enough?
- At 50–64 nodes, does B's vector halo keep the typed core (S/C/T) readable where A's plain
  force mixes them?
- Drag: spring back (A) or pin (B)?

## Findings while building

- **Refit on membership change.** Keeping the zoom across expand/collapse or type toggles
  clipped the new nodes at 132%. Refit when lane membership changes; keep the zoom only for
  selection, focus and drag.
- **The collapse is client-side.** Collapsing is the lane's view of one `k=64` response (as in
  sketch 004's "showing 8 of 57 · k=64"). The call line always shows k=64, and expanding
  needs no refetch.
- **The ceiling reaches the vector lane quickly.** With 3 chain + 2 citation + 6 tag
  neighbours, more than about 53 distinct vector hits reaches the 64 total ceiling.
- **Auto-pan on focus.** When arrow focus moves to a node outside the zoomed viewport, the
  graph pans to it; otherwise keyboard users lose the focused node.
