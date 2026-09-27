# Phase 5: Related-Memories Graph & Tag Cloud - Context

**Gathered:** 2026-09-27
**Status:** Ready for planning

<domain>
## Phase Boundary

The console gains two browse surfaces on top of the Phase 3 read RPCs:

- A **related view** for one record: that record plus its `RelatedMemories` neighbourhood,
  never a global view. The body is a set of edge-type lanes and the rail holds an inline-SVG
  d3-force graph as a linked overview. The graph has pan, zoom, drag, a fixed settle budget,
  capped edges, and a legend with per-type toggles (GRAPH-01). It is fully keyboard- and
  ARIA-equivalent (GRAPH-02) and works in both themes on the category tokens (GRAPH-03).
- **Tag popularity** from `ListTags` counts, shown as a linear bar list (TAGS-01, amended below
  by D-12). The list opens from `/search` and from the related view's rail. The tag filter chip
  offers autocomplete with counts, so a user can filter by tag without opening the list
  (TAGS-02).

Out of scope: Jev edge labels (GRAPH-04, v2), a tag click that highlights graph nodes as a
cross-surface link (GRAPH-05, v2), neighbour-to-neighbour or 2-hop edges, any server or proto
change, NL query understanding (Phase 6), and client-tier CLI verbs (#630).

</domain>

<decisions>
## Implementation Decisions

### Related view: shape and entry
- **D-01:** The view is a **route, `/related/<id>`** (full UUID or short_id). It follows the
  sketch 004 synthesis layout:
  - an anchor bar with a short_id **crumb trail**, a `← back [` button, and the anchor card with
    its mono call line;
  - **edge-type lanes as the body**;
  - a sticky right rail with **Graph | Tags** tabs (`g` toggles; last tab remembered per viewer
    in local storage, safe to lose).

  Below 900px the rail stacks under the lanes and the evidence drawer becomes a bottom sheet
  (max 62vh), as in the sketch.
- **D-02:** **Lanes are the primary body, and the graph is the rail overview.** GRAPH-01..03
  are met by the rail graph. The graph is not the primary evidence surface (sketch avoid-note).
- **D-03:** Three entry points:
  1. a **"Related" inline button** in `DetailPane` actions;
  2. a **row key `r`** on the active row, shown in the listbox legend and added to the
     `engram-console-conventions` keyboard table (`r` and `g` are unbound today);
  3. a **⌘K item** "Related to `<short_id>`" when a record is selected.
- **D-04:** **Re-centre** (Enter, double-click, or the drawer's "Re-centre on … ↵") navigates to
  `/related/<new-id>`. It pushes a history entry and a crumb, and it clears the selection. `[`
  and browser Back walk back through the trail. **Escape** closes the evidence drawer and clears
  the selection first. A second Escape leaves the graph region. Escape outside the graph returns
  to where the view was opened from.
- **D-05:** **The evidence drawer is GRAPH-03's "detail pane"** on this route. It is the same
  surface for a node click, a lane-row click, and a chain-card click. It holds:
  - the candidate header (category, short_id, date, summary, tags with shared ones highlighted,
    state chips);
  - per-type "Why it is related" blocks, ending with "Evidence is per type; there is no blended
    score.";
  - **Re-centre ↵**;
  - an **"Open record"** link to `/search?sel=<id>` for the full `DetailPane` and its curation
    actions.

  The REQUIREMENTS/ROADMAP wording ("selects it in the detail pane") is satisfied by this
  drawer. Verification should read it that way.

### Graph: edges, caps, layout, keyboard
- **D-06:** The edge set is a **star plus chain arrows**. Every drawn edge comes from the one
  `RelatedMemories` response: one edge per type between the anchor and each candidate (offset
  curves when a candidate has several types), plus `superseded_by` arrows between chain members
  when both are drawn. There are **no neighbour-to-neighbour edges**, whether computed in the
  browser or on the server. Every edge has server evidence in the drawer.
- **D-07:** **"Capped edge count per node" means the server's per-type caps** (about 8 per type;
  chain 8 hops / 16 members; total ceiling 64; Phase 1 D-12). There is no new client
  scoring heuristic. That settles the ROADMAP's "no precedent" flag. The graph **mirrors the
  vector lane's collapse**: it shows 8 vector nodes until the user expands "show all N" in the
  lane, and then the extra nodes appear in the graph too. Graph and lanes always show the same
  membership. Scores are compared only within a type.
- **D-08:** **Jev `same_subject` is out.** GRAPH-04 stays v2. `RelatedMemories` carries no
  decision field, and this phase adds none.
- **D-09:** **The settle budget is a fixed tick count, run synchronously before first paint,
  then static.** Seed the layout deterministically so renders and tests are stable. A drag
  briefly reheats the simulation. Under `prefers-reduced-motion` there is no animated reheat.
  Rendering follows `.planning/research/STACK.md`: d3-force, d3-drag, d3-zoom and d3-selection,
  with Svelte owning every `<g>`/`<path>`/`<circle>` and d3-selection used only as the zoom/drag
  call target. These are the milestone's second allowed small UI dependency set (with
  `@humanspeak/svelte-virtual-list`).
- **D-10:** **Graph keyboard: one Tab stop with roving arrows**, the same pattern as the shipped
  listbox:
  - The SVG region takes focus and uses `aria-activedescendant`.
  - Arrows move in **lane order**: supersession › citation › tag › vector, then strength within
    the lane.
  - Home and End jump to the ends. Space or click selects and opens the drawer. Enter
    re-centres. Escape follows D-04.
  - Each node has an accessible name: short_id, category, edge types, and state words.
  - Legend toggles are the **same switches** as the lane hide buttons, with `n/cap` counts.
- **D-11:** **The screen-reader equivalent is a visually hidden neighbourhood list plus a polite
  `aria-live` summary.** The list, inside the graph region, names each node (short_id, category,
  edge types, state). The summary line announces re-centre, toggle and filter results, for
  example "14 related · supersession 3 · citation 2 · tag 6 · vector 8". The lanes remain the
  visible text equivalent.

### Tag popularity surface
- **D-12:** The surface is **linear bars, and TAGS-01 is amended.** The sketch 004 bar list
  (tag name in mono, a bar from zero, the printed count, sorted by count descending, DOM order =
  reading order) replaces "a cloud sized by count quantile". `.planning/REQUIREMENTS.md` TAGS-01
  and ROADMAP Phase 5 success criterion 4 are amended to match (Sean, 2026-09-27). The phase
  name stays as is.
  — **Reversibility:** reversible — presentation only; the data (`ListTags`) is unchanged.
- **D-13:** Two hosts share **one tag-bars component**:
  - **`/search`**: a "Tags" button in `FacetStrip` opens a panel/popover. Counts are for the
    current scope chip when exactly one is set, and for all readable scopes otherwise. Clicking
    a bar **adds a `#tag` facet chip**.
  - **`/related` rail Tags tab**: counts are for **all readable scopes** (neighbours are
    cross-spine, Phase 1 D-10). The anchor's own tags are marked `●` with a violet bar.
- **D-14:** In the `/related` rail, clicking a tag **filters the lanes in place**. Lane rows and
  graph nodes that lack the tag dim to 30%, and a chip reads "#qdrant 3 of 14 carry it ×". This
  is the view's own filter chip (TAGS-01). It is not GRAPH-05, which stays deferred.
- **D-15:** The list shows the **top 30 by default**. "Load all" refetches at the 1000 maximum.
  When `more` is true, the header says "showing the N most-used tags". A filter box narrows the
  loaded set and says when nothing in it matches. Each tag's tooltip shows popularity and rarity
  separately (`count 400 · rarity ln(n/df) 1.10`), never merged.

### Tag chip autocomplete (TAGS-02)
- **D-16:** Autocomplete appears in **two places, backed by one `ListTags` query per scope key**:
  1. Typing `#` or `tag:` in the **header search** shows a Tags group with counts. This closes
     the Phase 2 deferred item "Tags section of the header dropdown".
  2. A **"+ tag" `TagCombobox`** in `FacetStrip` mirrors `ScopeCombobox`.
- **D-17:** **Count scope follows the scope chip.** With exactly one scope chip it is
  `ListTags(scope)`, otherwise all readable scopes. A tag showing N yields N results under that
  filter (Phase 1 D-13).
- **D-18:** **Matching is a substring search over the top 1000**, fetched once per scope key and
  cached. Results rank prefix matches first, then by count. The server has no prefix filter
  (Phase 3 D-14). When `more` is true the footer says "matching among the 1000 most-used tags".
- **D-19:** **A tag that is not in the loaded list can still be added**, with a reason shown:
  "#foo — not among the loaded tags" when `more` is true, or "#foo — 0 recall-visible records"
  when the list is complete. It is never silently blocked, and the copy never implies the tag
  exists.

### Carried forward (decided earlier, do not re-ask)
- Sketch winner `6akjphx3k7`: related view = lanes (004 B) + compact graph and evidence drawer
  (004 C) synthesis. One shared selection lights every appearance of a candidate across lanes,
  graph and chain. `×N` is a count, not a jump link. State chips sit along the bottom of cards
  and nodes.
- Edge encoding (sketch): edges are grey, and type is shown by **line style plus a letter
  glyph** (S solid 2px with an arrowhead, C dashed `6 3`, T dotted `1.5 3.5`, V thin and faint).
  Selection is the only thing that colours an edge (violet). Node fill = category token. The
  anchor has a double violet ring. Hidden-state chain members are dashed at 55%.
- Truncation and empty-state copy per `related-and-tags.md`: the ceiling banner names the
  vector lane as the one cut; an empty neighbourhood lists what each edge type searched.
- Wire contract (Phase 3 D-12/D-13, `eneecjyzxh`): `RelatedEdge` = `EdgeType` + `oneof
  evidence` (discriminated union in TS); compact view by default with `full` opt-in;
  `truncated` carried through. ListTags: exact recall-visible counts, count-descending, default
  100, max 1000, `more`; empty scope = all readable.
- Phase 1 D-06..D-16: one entry per candidate, rarity-weighted tag edges `ln(n/df)`, citation
  match on kind+ref, full supersession chain (hidden members included), non-supersession edges
  recall-gated, per-type caps with `k` widening only the vector cap.
- The sketch's "no graph library required" is **overridden** by GRAPH-01 and
  `.planning/research/STACK.md` (d3 micro-packages), consistent with rule `xvqj44e5mk`.
- Testing (Phase 4 D-18; rules `m45p2b4bp7`, `3p0zsqrhmb`; preference `x0krpn67b0`): test the
  behaviour of code we own, with no manual UAT. vitest-browser covers each surface, with
  screenshots in both themes. Do not assert d3's physics; assert our node/edge membership, caps
  mirroring, keyboard traversal order, ARIA names, and filter/chip behaviour. WCAG 2.2 AA
  failures get fixed in-phase; other findings become issues.

### Claude's Discretion
- Curation actions on `/related`: none by default. "Open record" hands off to `/search?sel=`,
  where the Phase 4 keys and dialogs live. Row keys other than `r` are not bound on `/related`.
- Pan/zoom extents, the exact tick count, force strengths, label density (the sketch shows
  labels at ≤26 visible nodes), and the scale factor tied to the text-size preference.
- Whether `/related` joins the resume `ALLOWED_DESTINATIONS` so a re-auth lands back on the
  same anchor. It is read-only, so no draft kind is needed.
- The shape of the `/search` Tags panel (popover or side panel) and the query-key naming for
  `['listTags', scopeKey, limit]`, following the `engram-connect-client` conventions.
- Loading skeleton copy, e.g. `RelatedMemories(subj, "…", k=8) — resolving anchor…▍`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope
- `.planning/REQUIREMENTS.md`: GRAPH-01..03, TAGS-01 (amended by D-12), TAGS-02;
  GRAPH-04/05 v2; "two small UI libraries allowed"
- `.planning/ROADMAP.md`: Phase 5 goal and success criteria 1–4 (SC4 amended by D-12)
- `.planning/research/STACK.md`: d3-force/drag/zoom/selection verdict and install lines;
  Svelte-owned SVG rendering pattern; tag-surface analysis
- `.planning/notes/console-overhaul-exploration.md`: milestone decisions

### Design direction (locked by Phase 01.1 sketches)
- `.claude/skills/sketch-findings-engram/references/related-and-tags.md`: lanes, rail, graph,
  evidence drawer, edge encoding, truncation, tag bars, what to avoid (**primary design
  source**)
- `.claude/skills/sketch-findings-engram/sources/004-related-and-tags/index.html`: runnable
  synthesis winner (`drawGraph`, `forceGraph`, `evidenceBlock`)
- `.claude/skills/sketch-findings-engram/references/foundations.md`: tokens, state chips,
  timings, text-size scale
- `.claude/skills/engram-console-conventions/SKILL.md`: keyboard model (add `r`, `g`, graph
  keys), category tokens, state words, honest feedback
- `.claude/skills/engram-connect-client/SKILL.md`: read client, query keys, `RelatedMemories` /
  `ListTags` per-RPC rows
- `dataviz` skill: load before building the tag bars or the cosine scale marker

### Prior phase decisions
- `.planning/phases/01-store-prerequisites/01-CONTEXT.md`: D-06..D-16 RelatedMemories and
  ListTags semantics
- `.planning/phases/03-curation-rpcs-mcp-tools/03-CONTEXT.md`: D-12..D-14 wire shape, no
  ListTags prefix
- `.planning/phases/04-curation-surfaces/04-CONTEXT.md`: D-01 key guards, D-11 route pattern,
  D-16 resume envelope, D-17/D-18 audit and test policy
- `.planning/phases/02-recall-first-search/02-CONTEXT.md`: D-07 listbox pattern, D-11 ⌘K,
  D-15 pane actions; deferred "Tags section of the header dropdown"

### API contract
- `proto/engram/v1/engram.proto`: `EdgeType`, `RelatedEdge`, `RelatedMemory`,
  `RelatedMemoriesRequest/Response`, `TagCount`, `ListTagsRequest/Response`
- `ui/src/lib/gen/`: generated TS client

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ui/src/lib/components/ChainDialog.svelte` + `ui/src/lib/curation/chain.ts`: already read
  `RelatedMemories` (query key `['relatedMemories', id, k, full]`) and build the pure chain
  model. Reuse it for the supersession lane timeline.
- `ui/src/lib/components/ScopeCombobox.svelte`: pattern for `TagCombobox` (bits-ui Command,
  `shouldFilter={false}` per gotcha `3tz15e733n`, portal disabled inside a Popover).
- `ui/src/lib/components/HeaderSearch.svelte` + `ui/src/lib/search/classify.ts`: `#`/`tag:`
  tokens already classify to tag chips; add the Tags suggestion group here.
- `ui/src/lib/components/FacetStrip.svelte` + `ui/src/lib/search/params.ts`: `tags` URL state
  (`tag=` repeated, sorted) and chip removal already exist.
- `ui/src/lib/components/DetailPane.svelte` / `RowActions.svelte` / `CommandMenu.svelte`: where
  the Related button, `r` key, and ⌘K item attach.
- `ui/src/lib/mutations/curation.ts`: already invalidates `['relatedMemories']` after writes.
- `ui/src/lib/memorystate.ts`: state words and canonical order for node/card chips.

### Established Patterns
- Listbox with `aria-activedescendant` and `isTypingTarget`/modifier guards on row keys
  (Phase 2 D-07, Phase 4 D-01). The graph's roving model copies this.
- Route pages (`/rules`, `/scheduled`) with their own URL state reuse shared components.
- TanStack Query v6 thunk syntax, `AbortSignal`, `keepPreviousData`. Read RPCs go through the
  `engram` (non-CSRF) client.
- Container-query drop-outs must swap the grid `--cols` var (gotcha `xx98my50ng`). This applies
  to lane rows.

### Integration Points
- New route `ui/src/routes/related/[id]/`, plus nav/⌘K wiring.
- `ui/src/lib/resume.ts` `ALLOWED_DESTINATIONS` (discretion: add `/related`).
- `ui/package.json`: add `d3-force`, `d3-drag`, `d3-zoom`, `d3-selection` (+ `@types/*` dev).
- `internal/e2e` chromedp: optional related-view smoke test per the D-18 policy.

</code_context>

<specifics>
## Specific Ideas

- The anchor card's mono call line is written as the sketch shows it:
  `RelatedMemories(subj, "q7kf…", k=8) → 14 related · supersession 3 · citation 2 · tag 6 ·
  vector 8 · truncated=false · 14ms`.
- Empty lanes say why ("none — anchor carries no citations"). An empty neighbourhood lists
  what each edge type searched and offers a way forward.
- Popularity helps pick a filter, and rarity explains a tag edge. Keep both visible and never
  merge them.

</specifics>

<deferred>
## Deferred Ideas

- GRAPH-04: edges labelled by a Jev consolidate verdict (seed
  `jev-edge-labelling-in-related-graph`), v2.
- GRAPH-05: a tag-cloud click highlights matching graph nodes across surfaces, v2. D-14's
  in-view dimming is not this.
- Neighbour-to-neighbour or 2-hop edges (client-derived or extra `RelatedMemories` calls):
  rejected for this phase because they drift toward a global view and lack server evidence.
- A server-side ListTags prefix search: only if the D-18 client substring search over the top
  1000 proves insufficient (carried from Phase 3).

</deferred>

---

*Phase: 05-related-memories-graph-tag-cloud*
*Context gathered: 2026-09-27*
