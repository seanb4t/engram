# Related memories and tag popularity

How a record's related neighbourhood (`RelatedMemories`) and the store's tag counts
(`ListTags`) are shown (sketch 004, winner Synthesis). Read `foundations.md` for tokens, state
chips and keys.

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
- **Evidence drawer.** Selection slides C's evidence panel over the rail (0.2s): header
  "Evidence · supersession › citation › tag › vector · Esc ×", the candidate (category,
  short_id, date, summary, tags with shared ones highlighted, state chips, primary
  **"Re-centre on q7kf… ↵"**), then **"Why it is related · N edge types"** with one block per
  edge type:
  - tag: a table `shared tag | df | ln(1200/df)` with a `tag_weight` total row
  - vector: `cosine 0.781` with a 0–1 scale marker
  - citation: kind + ref per line
  - supersession: the path from anchor with direction and depth

  closing with "Evidence is per type; there is no blended score." Below 900px the drawer
  becomes a bottom sheet (max 62vh).
- **Re-centre** loads the neighbourhood of the selected record, pushes it onto the trail, and
  clears the selection. Loading shows a skeleton anchor card with
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
- Show the top ~30 by default with a "load all" control; the filter box states when a query
  matches nothing in the loaded set.

### Stack mapping

| Sketch piece | Real stack |
|---|---|
| Lanes, rows, chain timeline | plain Svelte components; Card styling from shadcn-svelte |
| Rail tabs | shadcn-svelte `Tabs`; persist the tab in the viewer's local storage (a convenience, safe to lose) |
| Graph | a small SVG component with a precomputed or lightweight force layout (the sketch hand-rolls ~40 lines); no graph library is required at 64 nodes |
| Evidence drawer | an absolutely positioned panel inside the rail (not a modal); `Sheet` side="bottom" below 900px |
| Tag tooltip, `×N` tooltip, state-chip tooltip | shadcn-svelte `Tooltip` |
| Data | new Connect read RPCs `RelatedMemories(id)` and `ListTags(scope)` with counts |
| Chart styling | load the `dataviz` skill before building bars or the scale marker |

Apply selection in place (toggle classes / reactive state per item) rather than re-rendering the
lanes, so focus, scroll position and the drawer transition survive.

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

/* evidence drawer over the rail; bottom sheet when narrow */
.drawer { position: absolute; inset: 0; z-index: 5; display: flex; flex-direction: column; background: var(--color-surface);
  border-left: 1px solid var(--color-border); box-shadow: var(--shadow-md); transform: translateX(104%); visibility: hidden;
  transition: transform .2s ease, visibility .2s ease; }
.drawer.open { transform: none; visibility: visible; }
@container frame (max-width: 900px) {
  .drawer { position: fixed; inset: auto 0 0 0; max-height: 62vh; border-radius: var(--radius-lg) var(--radius-lg) 0 0; transform: translateY(104%); }
}

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
    <div class="drawer open" role="dialog" aria-label="evidence">…Why it is related · 2 edge types…</div>
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
- **`×N` as a jump link** in the synthesis. Shared selection already lights every appearance.
- **One blended relevance score across edge types.** Evidence is per type; say so.
- **Colour-coded edges.** Line style and glyphs carry type; colour is for selection only.
- **Showing a truncated lane without saying what was cut,** or treating an empty neighbourhood as
  an error instead of explaining what was searched.

## Origin

Synthesized from sketches: 004 (winner Synthesis: B lanes + C graph/evidence, state chips on the
bottom of cards and nodes).
Source files available in: `sources/004-related-and-tags/index.html`.
