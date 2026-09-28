# Related memories and tag popularity

How a record's related neighbourhood (`RelatedMemories`) and the store's tag counts
(`ListTags`) are shown: sketch 004 (winner Synthesis) for the layout, sketch 006 (winner A1)
for graph controls and where evidence goes, and sketch 005 (winner C) for the tag entry points
on `/search`. Read `foundations.md` for tokens, state chips and keys.

> **Superseded (sketch 006):** 004's slide-over **evidence drawer** is rejected. It covered the
> node the user had just clicked. Evidence now stacks **under the graph** in the rail (see
> "Evidence section" below).

## Design Decisions

### Data shape the UI is built on

`RelatedResult { anchor, related: [{ memory, edges: [RelatedEdge] }], truncated }`, one entry
per candidate with every edge type that reached it. Edge types, in canonical order:

| Type | Evidence | Cap |
|---|---|---|
| `supersession` | `direction` (predecessor / successor) and `depth` | chain 8 hops / 16 members |
| `citation` | `shared_citations` (kind + ref; locator and pin excluded) | 8 |
| `tag` | `shared_tags` each with `weight = ln(n/df)`, summed `tag_weight` | 8 |
| `vector` | raw cosine `score` (query-by-id) | `k` (8 by default; the only one the caller widens) |

Total ceiling 64; only the vector edge can reach it, so `truncated=true` means vector
neighbours were cut. Superseded and archived chain members are included (dimmed); every other
lane only contains recall-visible records.

### Layout: lanes are the body, the graph is an overview

- **Anchor bar** on top: a **trail** of short_id crumbs (click to go back to any of them),
  `← back [` button, the Aa button; then an **anchor card** with a 3px violet left border:
  category chip, short_id, summary, tags on the right, and a mono call line
  `RelatedMemories(subj, "q7kf…", k=8) → 14 related · supersession 3 · citation 2 · tag 6 · vector 8 · truncated=false · 14ms`,
  with state chips along the bottom.
- **Main body: one lane per edge type** (B's lanes), each a card:
  - Header: the edge-type line swatch, the type name, `n/cap` count (mono), a one-line caption
    stating what the lane measures and its cap, and a **hide/show** toggle.
  - **Supersession lane is a horizontal timeline**: columns from the oldest predecessor
    (`−2`, `−1`) through the anchor to successors (`+1`); cards about 180px wide with category
    dot, short_id, `±depth`, two-line summary, date, `×N` badge, state chips on the bottom;
    arrows between columns are the supersession swatch labelled `superseded_by`. Hidden
    members render dashed at 55% opacity.
  - Other lanes are one-line rows: category dot + short_id · summary · **evidence inline** ·
    `×N` multi-type badge · state chips.
    - citation: `file internal/store/relatedmemories.go` pills (dashed border)
    - tag: `#tag 4.32` pills (dotted border) per shared tag, then `Σ 7.10`
    - vector: `0.781` plus a 48px bar
  - Rows are sorted by that lane's own strength; **scores are only compared within a lane**.
  - Empty lanes say why: "none — anchor carries no citations", "none — no recall-visible record
    shares a tag".
- **Right rail (about 340px, sticky) with two tabs: Graph | Tags.** The rail remembers the last
  tab per viewer; `g` toggles it.
  - **Graph:** C's compact force graph as a **linked overview**. Node fill = category colour;
    edge line style = edge type (below); the anchor has a double violet ring. Legend toggles are
    the **same switches** as the lane hide buttons, with `n/cap` counts.
    Click selects, double-click or `Enter` re-centres.
  - **Tags:** B's **linear bar list** from `ListTags` (exact counts): tag name (mono), bar from
    zero, count. Tags on the anchor are marked `●` with a violet bar. A filter box narrows the
    list. Clicking a bar sets a tag filter: lane rows lacking the tag dim to 30% and a filter
    chip shows "#qdrant 3 of 14 carry it ×".
- **One shared selection.** Selecting a row, card or node lights **every appearance** of that
  candidate across lanes and the graph (violet-soft background; the lane where you clicked gets
  a 3px violet bar; the node gets a violet halo and its edges turn violet; unrelated nodes drop
  to 45%). In the synthesis `×N` is a **count, not a jump link**.
- **Evidence section (sketch 006 A1), directly under the graph in the rail, never over it.**
  Selecting a node, lane row or chain card leaves the graph whole. The selection is lit and the
  evidence stacks beneath the graph, above the legend; the rail scrolls
  (`max-height: calc(100vh − header)`, `overflow-y: auto`). Contents:
  - the candidate header (category dot, category, short_id, date) with `esc ×`, the summary, and
    state chips;
  - "Why it is related · N edge types", then one line per edge type: glyph · type · evidence
    (`cosine 0.819` / `#tag … · Σ 3.05` / `file internal/…` / `predecessor · depth 2`);
  - **Re-centre ↵** (primary) and **Open record ↗** (hands off to `/search?sel=<id>` for the full
    detail pane and its curation actions);
  - "Evidence is per type; there is no blended score."

  The same content serves as GRAPH-03's "detail pane" on this route. The deeper tag table from
  004 (`shared tag | df | ln(n/df)`) may expand inside the tag line. Below 900px the rail stacks
  under the lanes and the section stays under the graph.
- **Re-centre** navigates to `/related/<id>` for the selected record, pushes it onto the trail
  and browser history, and clears the selection. Loading shows a skeleton anchor card with
  `RelatedMemories(subj, "…", k=8) — resolving anchor, 4 typed sub-queries in flight▍`.

### Edge encoding: never colour alone

Edges are grey; type is carried by **line style**, repeated in a **letter glyph** whose border
uses the same style, so edge types are nameable in greyscale and at small sizes.

| Type | Line | Glyph |
|---|---|---|
| supersession | solid, 2px, **arrowhead** toward the live head | `S`, 2px solid border |
| citation | dashed `6 3`, 1.5px | `C`, dashed border |
| tag | dotted `1.5 3.5`, 1.8px, round caps | `T`, 2px dotted border |
| vector | thin solid 1px, fainter | `V`, faint border |

Selection is the only thing allowed to colour an edge (violet).

### Graph controls and keyboard (sketch 006 A)

- Edges are a **star plus chain arrows**, all from one `RelatedMemories` response: anchor ↔
  candidate once per edge type (offset quadratic curves, ±12, when a candidate has several), plus
  `superseded_by` arrows between chain members. There are no neighbour-to-neighbour edges.
- **Corner controls**, top-right of the canvas: `+`, `−`, `⤢` fit, and a mono zoom readout
  (`132%`). Scale runs 0.5–4 and translation is bounded to the graph extent. `+` and `−` disable
  at the limits.
- **Plain wheel scrolls the page** (the rail is sticky). **⌘/Ctrl+wheel zooms.** A plain wheel
  over the graph flashes a centred hint, "hold ⌘ to zoom with the wheel", for about 0.9s.
  Double-click is re-centre, never zoom.
- **One Tab stop.** The SVG is `role="listbox" tabindex="0"` with `aria-activedescendant` on
  `role="option"` node groups. Arrows move in **lane order** (anchor first, then supersession ›
  citation › tag › vector, strength within the lane), and Home/End jump to the ends. Space or
  click selects and shows the evidence section. Enter re-centres. Escape clears the selection,
  then leaves the graph.
- **Focus look:** a dashed violet ring (`stroke-dasharray: 3 2.5`, r + 6) plus a **floating focus
  card** (short_id, edge glyphs, two-line summary, state chips). The same card appears on
  hover. It flips to the node's left near the rail edge, and it is hidden for the selected node,
  whose evidence is already under the graph.
- **Auto-pan:** when arrow focus lands on a node outside the viewport, `translateTo` it.
- **Labels:** at 26 drawn nodes or fewer, every node has a short_id label under it. Beyond 26,
  only the anchor, focused and selected nodes are labelled, and node radius drops from 6 to about
  4.2 (anchor 10).
- **Drag springs back:** the node is fixed while dragged, the simulation briefly reheats
  (`alphaTarget(0.25)`), and the node is released on drop. There is no pinning. Under
  `prefers-reduced-motion` the node moves with no reheat and zoom transitions are instant.
- **Settle:** a fixed 300 ticks run synchronously before first paint with a seeded
  `randomSource`, then the layout stays static. The anchor is fixed at the origin.
- **Refit when lane membership changes** (expand/collapse the vector lane, type toggles,
  re-centre). Keep the zoom only across selection, focus and drag.
- **The vector collapse is client-side.** The lane shows 8 rows of one `k=64` response, and the
  graph mirrors exactly the lane's membership. Expanding needs no refetch. With about 11 typed
  neighbours, more than about 53 distinct vector hits reaches the 64 total ceiling.
- **Screen-reader equivalent:** a visually hidden list in lane order (`short_id, category, edge
  types, states`; the focused item is `aria-current`) plus a polite `aria-live` line
  ("4 of 17: jbagp95vmc, discovery, supersession", "vector lane expanded…").

### Truncation and ceilings

- At the 64-entry ceiling a warning banner: "▲ truncated=true — total ceiling 64 reached; only
  the vector lane is cut." The call line shows `truncated=true` in warning colour.
- The vector lane collapses to **8 rows** plus a more-row: "showing 8 of 57 · k=64 · ceiling cut
  the rest · show all 57 ▸". Other lanes are always below their caps.
- **No neighbours:** a card listing what each edge type searched and found nothing:
  chain walk (no superseded_by, no supersedes), citations (anchor carries 0), tags probed
  (`#tag (df 3)` each), vector k=8 (0 recall-visible neighbours), plus a way forward.

### Tag popularity

- **Linear bars from zero, printed counts.** Counts stay comparable; no axis needed.
- **Rarity is the inverse of popularity.** Tag-edge weight is `ln(n/df)`, so small tags make
  strong edges; the tag tooltip shows `count 400 · rarity ln(n/df) 1.10`. Popularity helps pick a
  filter; rarity explains a tag edge. Keep both visible, never merge them.
- Show the top 30 by default with a "show all" control; the filter box states when a query
  matches nothing in the loaded set.
- **One fetch.** `ListTags(scope, limit=1000)` is fetched once per scope key and cached. It
  feeds the bars (first 30, then show all with no refetch), the "+ tag" picker, and the header
  search Tags group.
- **ListTags has no total.** With `more=true`, say "showing the 30 most-used tags" or
  "matching among the 1,000 most-used tags — more exist" (warning colour). Never write "of N".
- The rarity half of the tooltip needs `n` (the recall-visible record count), which the sketch
  takes from ListScopes counts. If no exact `n` is available, show rarity only in `/related`
  tag-edge evidence.

### Tag entry points on `/search` (sketch 005 C)

- **Docked Tags panel.** A "▦ Tags panel" toggle (`aria-pressed`) sits at the right end of the
  facet strip. The panel (about 340px) takes the **right slot shared with the detail pane**:
  opening a record replaces it. The panel has a header, a filter box, and the bar list with
  footer honesty copy. Active filter tags keep `●` and a violet bar, so the selection stays
  visible while the user reads results. Clicking a bar **toggles** its `#tag` chip; ↑/↓, Home/End
  and ↵ work on the listbox. It becomes a bottom sheet (62vh) at narrow widths.
- **Compact "+ tag" picker** beside the chips, a dashed pill that turns solid when open. Its
  popover holds a `#`-prefixed mono input with `esc` and a match list: `#tag` with the match
  bolded in violet, a faint mini bar, and the count. Matching is a substring search ranked
  prefix-first, then by count, top 8 shown with "N matches · top 8 shown". A final **unknown-tag
  row**, "Add #foo", gives its reason in warning mono: "not among the 1,000 loaded tags" when
  `more`, "0 recall-visible records" when the list is complete. ↵ adds the tag and closes.
- **Header search Tags group.** Typing `#` or `tag:` in the header search swaps the dropdown to
  a "Tags · counts in <scope>" group with the same rows, the same unknown-tag row and the same
  footer. ↵ adds the chip and clears the input. Free text still shows "Search memories for …".
- **Counts follow the scope chip** (exactly one scope chip → that scope; otherwise all
  readable). The scope label appears in the panel header and in the picker and dropdown
  footers.
- **States:** loading shows `ListTags(scope="…", limit=1000) in flight▍` with skeleton bars.
  Empty explains that tags on archived, superseded, expired or not-yet-active records are not
  counted, and that a `#tag` can still be typed.

### Stack mapping

| Sketch piece | Real stack |
|---|---|
| Lanes, rows, chain timeline | plain Svelte components; Card styling from shadcn-svelte |
| Rail tabs | shadcn-svelte `Tabs`; persist the tab in the viewer's local storage (a convenience, safe to lose) |
| Graph | a Svelte-owned SVG component driven by d3-force / d3-zoom / d3-drag / d3-selection (`.planning/research/STACK.md`, rule `xvqj44e5mk`); 004's hand-rolled layout is superseded |
| Evidence section | a plain section under the graph inside the scrolling rail (never an overlay) |
| Graph controls | d3-zoom (`scaleExtent`, `translateExtent`, wheel filter for ⌘/Ctrl) + d3-drag; buttons are plain Svelte |
| `/search` Tags panel | a docked panel in the right slot `DetailPane` uses (paneforge pane), `Sheet` side="bottom" when narrow |
| "+ tag" picker, header Tags group | bits-ui `Command` in a `Popover` (`shouldFilter={false}`, portal disabled), as `ScopeCombobox` |
| Tag tooltip, `×N` tooltip, state-chip tooltip | shadcn-svelte `Tooltip` |
| Data | new Connect read RPCs `RelatedMemories(id)` and `ListTags(scope)` with counts |
| Chart styling | load the `dataviz` skill before building bars or the scale marker |

Apply selection in place (toggle classes / reactive state per item) rather than re-rendering the
lanes, so focus, scroll position and the evidence section survive.

## CSS Patterns

```css
.s-grid { display: grid; grid-template-columns: minmax(0, 1fr) calc(340*var(--u)); gap: calc(10*var(--u)); align-items: start; }
.rail { position: sticky; top: 56px; overflow: hidden; min-height: calc(460*var(--u)); }
@container frame (max-width: 900px) { .s-grid { grid-template-columns: 1fr; } .rail { position: relative; top: auto; } }

/* lane row */
.row { display: grid; grid-template-columns: calc(96*var(--u)) minmax(0,1fr) auto auto auto; gap: calc(10*var(--u));
  align-items: center; min-height: var(--row-h); padding: calc(3*var(--u)) calc(10*var(--u)); border-bottom: 1px solid var(--color-border-subtle); }
.row.sel { background: var(--color-primary-soft); }
.row.sel-here { box-shadow: inset calc(3*var(--u)) 0 0 var(--color-primary); }
.row.fdim { opacity: .3; }                 /* fails the tag filter */
.lane.off .lane-body { display: none; }
.lane.off .lane-h { opacity: .6; }

/* edge-type glyphs: identity by border style + letter */
.g { display: inline-flex; align-items: center; justify-content: center; width: calc(16*var(--u)); height: calc(16*var(--u));
  font: 600 calc(9*var(--u))/1 var(--font-mono); color: var(--color-text-muted); border: 1px solid var(--color-text-faint); border-radius: calc(3*var(--u)); }
.g-supersession { border: 2px solid var(--color-text-muted); }
.g-citation { border-style: dashed; }
.g-tag { border-style: dotted; border-width: 2px; }
.g-vector { border-color: var(--color-border); }

/* evidence pills */
.cite { font-family: var(--font-mono); font-size: var(--text-2xs); background: var(--color-surface-2); border: 1px dashed var(--color-border); border-radius: var(--radius-sm); }
.tagw { font-family: var(--font-mono); font-size: var(--text-2xs); background: var(--color-surface-2); border: 1px dotted var(--color-text-faint); border-radius: var(--radius-sm); }

/* chain cards */
.chain-card { width: calc(180*var(--u)); display: flex; flex-direction: column; padding: calc(6*var(--u)) calc(8*var(--u));
  border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-bg); }
.chain-card.is-anchor { border-color: var(--color-primary); background: var(--color-primary-soft); }
.chain-card.hidden-state { opacity: .55; border-style: dashed; }
.chain-card .states-foot { margin-top: auto; }   /* state chips pinned to the bottom edge */

/* graph */
.edge { fill: none; stroke: var(--color-text-muted); transition: opacity .15s ease, stroke .15s ease; }
.edge.t-vector { stroke: var(--color-text-faint); }
svg .edge.selon { stroke: var(--color-primary); opacity: 1; }
svg.has-sel .node:not(.sel):not(.anchor) { opacity: .45; }
.node.sel .halo { stroke: var(--color-primary); }
.node.hidden-state circle { opacity: .45; stroke-dasharray: 2 2; }

/* evidence section under the graph (006 A1) — the rail scrolls, nothing overlays the graph */
.rail { position: sticky; top: 56px; max-height: calc(100vh - 66px); overflow-y: auto; }
.ev-below:empty { display: none; }
.ev { display: flex; flex-direction: column; gap: calc(6*var(--u)); padding: calc(8*var(--u)) calc(10*var(--u)); border-top: 1px solid var(--color-border-subtle); }
.ev-line { display: grid; grid-template-columns: auto calc(78*var(--u)) minmax(0,1fr); gap: calc(6*var(--u)); align-items: baseline;
  font: var(--text-2xs)/1.5 var(--font-mono); color: var(--color-text-muted); }

/* graph controls (006 A) */
.zctl { position: absolute; top: calc(8*var(--u)); right: calc(8*var(--u)); display: flex; flex-direction: column; background: var(--color-surface);
  border: 1px solid var(--color-border); border-radius: var(--radius-md); box-shadow: var(--shadow-sm); }
.zctl button { width: calc(26*var(--u)); height: calc(24*var(--u)); font: 600 var(--text-xs) var(--font-mono); color: var(--color-text-muted); }
.zctl .pct { font: var(--text-2xs) var(--font-mono); color: var(--color-text-faint); border-top: 1px solid var(--color-border-subtle); text-align: center; }
.node.kfocus .focus { stroke: var(--color-primary); stroke-width: 1.5; stroke-dasharray: 3 2.5; fill: none; }   /* r + 6 */
.flabel { position: absolute; pointer-events: none; max-width: calc(230*var(--u)); background: var(--color-surface-2);
  border: 1px solid var(--color-primary); border-radius: var(--radius-md); box-shadow: var(--shadow-md); }
.node .lbl { paint-order: stroke; stroke: var(--color-surface); stroke-width: 3px; }  /* legible over edges */

/* /search tag picker rows (005 C) */
.opt { display: grid; grid-template-columns: minmax(0,1fr) calc(80*var(--u)) calc(44*var(--u)); gap: calc(8*var(--u)); align-items: center; min-height: var(--row-h); }
.opt .nm b { color: var(--color-primary); }                 /* matched substring */
.opt .mini { height: calc(4*var(--u)); background: var(--color-text-faint); opacity: .6; }
.opt.unknown .why { color: var(--color-warning); font-family: var(--font-mono); font-size: var(--text-2xs); }
.fbtn { border: 1px dashed var(--color-border); border-radius: var(--radius-full); }
.fbtn[aria-expanded="true"], .fbtn[aria-pressed="true"] { border-style: solid; border-color: var(--color-primary); background: var(--color-primary-soft); }

/* tag bars: linear from zero */
.bar-row { display: grid; grid-template-columns: calc(132*var(--u)) 1fr calc(36*var(--u)); gap: calc(8*var(--u)); align-items: center; }
.bar-row .fill { height: calc(8*var(--u)); background: var(--color-text-faint); border-radius: 0 calc(4*var(--u)) calc(4*var(--u)) 0; min-width: 2px; }
.bar-row.anchor-tag .fill { background: var(--color-primary); }
.bar-row.anchor-tag .nm::before { content: '● '; color: var(--color-primary); }
```

SVG edge attributes (from `ES` in the source):

```js
const ES = {
  supersession: { dash: '',        w: 2   },  // + arrowhead marker
  citation:     { dash: '6 3',     w: 1.5 },
  tag:          { dash: '1.5 3.5', w: 1.8 },  // stroke-linecap: round
  vector:       { dash: '',        w: 1   },  // fainter stroke
};
```

## HTML Structures

```html
<div class="anchor-bar">
  <div class="crumbs"><span>trail</span><button class="crumb">m0x…</button><span>›</span><button class="crumb cur">q7kf…</button>
    <span class="nav-spacer"></span><button class="btn">← back <kbd>[</kbd></button><button class="aa">Aa</button></div>
  <div class="anchor-card">
    <div class="meta"><span class="chip cat-chip"><i></i>decision</span><span class="mono muted">q7kf2m9x0c</span></div>
    <div class="sum">…</div><div class="tags"><span class="tagc">#engram</span>…</div>
    <div class="call mono faint">RelatedMemories(subj, "q7kf2m9x0c", k=8) → <b>14</b> related · … · truncated=false · 14ms</div>
    <div class="states-foot"></div>
  </div>
</div>
<div class="s-grid">
  <div class="lanes">
    <section class="card lane">
      <div class="lane-h"><svg class="swatch"/><span class="name">tag</span><span class="cnt">6/8</span>
        <span class="cap">shared tags, each weighted by rarity ln(n/df), n=1,200 · ranked by Σ · cap 8</span><button class="eye">hide</button></div>
      <div class="lane-body">
        <div class="row" tabindex="0" data-rid="…" data-lane="tag">
          <span class="id"><i class="dot"></i>h2vd…</span><span class="sum">…</span>
          <span class="ev"><span class="tagw">#related-memories <b>5.14</b></span><span class="mono faint">Σ 7.10</span></span>
          <span class="multi"><span class="x" title="reached by 2 edge types: tag, vector">×2</span><span class="g g-vector">V</span></span>
          <span class="st"><!-- state chips --></span>
        </div>
      </div>
    </section>
    <!-- supersession (timeline), citation, vector lanes -->
  </div>
  <aside class="card rail">
    <div class="rail-tabs" role="tablist"><button role="tab" aria-selected="true" class="rail-tab on">Graph</button>
      <button role="tab" aria-selected="false" class="rail-tab">Tags</button><kbd>g</kbd></div>
    <div class="rail-body"><!-- graph + legend toggles, or tag filter + bars --></div>
    <div class="gwrap"><svg class="graph" role="listbox" tabindex="0" aria-activedescendant="n-…">…</svg>
      <div class="zctl"><button aria-label="Zoom in">+</button><button aria-label="Zoom out">−</button><button aria-label="Fit to view">⤢</button><div class="pct">100%</div></div></div>
    <div class="ev-below"><div class="ev" role="region" aria-label="Why … is related">…Why it is related · 2 edge types…</div></div>
    <div class="legend">…</div>
    <div class="sr-only">…neighbourhood list…</div><div class="sr-only" aria-live="polite"></div>
  </aside>
</div>
```

## What to Avoid

- **Radial graph as the main view (004 A).** Placing each candidate on the ring of its *first*
  edge type empties the outer **vector ring** (most vector hits also share a tag or citation, so
  they land on inner rings), which reads as "no vector neighbours" when there are many. At the
  **64 ceiling** the vector ring becomes a wall of spokes, and a candidate with three edge types
  draws three curves. Unreadable exactly when the data is interesting.
- **Log-scaled tag cloud (004 A).** Font size lies about dominance (`engram` 400 vs
  `related-memories` 7) and position carries no meaning; decoration, not a filter tool.
- **Squarified treemap for tags (004 C).** Packs 30+ tags but the long tail collapses into
  unlabeled tiles, and area comparisons are hard; bars with printed counts won.
- **The graph as the primary evidence surface (004 C alone).** A compact graph answers "how many,
  of which type, how clustered"; reading *why* needs per-type evidence in text. Keep the graph as
  the overview.
- **An evidence drawer or card over the graph (004 synthesis, 006 A3).** The slide-over drawer
  covered the node just clicked. A card on the far half of a 340px rail still hid half the
  graph. Keep evidence under the graph.
- **Evidence inline in the lane (006 A2).** It works for lane clicks, but for a graph click it
  scrolls the page away from the graph and leaves only a pointer in the rail.
- **Free wheel zoom in a sticky rail (006 B).** It hijacks page scrolling; require ⌘/Ctrl.
- **Chrome-free, keyboard-only zoom (006 B).** Undiscoverable. Pin-on-drag leaves stray state
  that needs an unpin key. A radial halo band for vector-only nodes was not needed.
- **Keeping the zoom across a membership change.** It clipped newly drawn nodes at 132%.
- **Two tag controls (005 A)** (a picker plus a separate Tags popover) and a **merged picker
  whose empty state is the bars (005 B)**: A crowds the strip and hides the bars when closed; B
  hides popularity behind "+ tag". The docked panel keeps the selection visible.
- **Totals in tag copy.** ListTags returns `more`, not a count; "of 1,340" would be invented.
- **`×N` as a jump link** in the synthesis. Shared selection already lights every appearance.
- **One blended relevance score across edge types.** Evidence is per type; say so.
- **Colour-coded edges.** Line style and glyphs carry type; colour is for selection only.
- **Showing a truncated lane without saying what was cut,** or treating an empty neighbourhood as
  an error instead of explaining what was searched.

## Origin

Synthesized from sketches: 004 (winner Synthesis: B lanes + C graph, state chips on the bottom of
cards and nodes; its evidence drawer superseded), 006 (winner A1: corner controls, ⌘-wheel zoom,
focus ring + card, evidence under the graph), 005 (winner C: docked Tags panel + compact picker +
header Tags group).
Source files available in: `sources/004-related-and-tags/index.html`,
`sources/006-graph-interaction-density/index.html` (needs network: d3 7.9.0 from jsDelivr),
`sources/005-tag-entry-points/index.html`.
