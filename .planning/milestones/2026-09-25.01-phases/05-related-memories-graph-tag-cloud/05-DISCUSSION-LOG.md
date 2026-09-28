# Phase 5: Related-Memories Graph & Tag Cloud - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-27
**Phase:** 05-related-memories-graph-tag-cloud
**Areas discussed:** Related view shape & entry, Graph edges & per-node cap, Tag cloud vs tag bars, Tag chip autocomplete

---

## Related view shape & entry

| Option | Description | Selected |
|--------|-------------|----------|
| Route /related/<id> | Full page per sketch 004; URL-addressable, trail, back/forward | ✓ |
| Large modal dialog | Like ChainDialog; no URL/resume | |
| Detail-pane section | ~400px; lanes+rail+graph won't fit | |

| Option | Description | Selected |
|--------|-------------|----------|
| Lanes body, graph in rail | Sketch winner as-is | ✓ |
| Graph primary, lanes secondary | Reverses the sketch avoid-note | |
| Toggle between them | More surface to build and test | |

| Option | Description | Selected |
|--------|-------------|----------|
| Pane button + row key + ⌘K | Consistent with Phase 4 action wiring | ✓ |
| Pane button only | Single entry | |
| Pane + row hover button | No key | |

| Option | Description | Selected |
|--------|-------------|----------|
| Navigate + trail | Re-centre pushes /related/<id>; `[`/Back returns | ✓ |
| In-place, trail only | Refresh loses the walk | |
| You decide | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Evidence drawer = the pane | Drawer serves as GRAPH-03's detail pane, with an "Open record" link | ✓ |
| Reuse DetailPane on /related | Three columns; heavier | |
| Drawer + DetailPane on demand | Overlay DetailPane with actions | |

**Notes:** `r` and `g` verified unbound in `ui/src`.

---

## Graph edges & per-node cap

| Option | Description | Selected |
|--------|-------------|----------|
| Star + chain arrows | Every edge from the one RelatedMemories response | ✓ |
| Also client-derived neighbour↔neighbour | No server evidence | |
| Also server neighbour↔neighbour | N extra calls; drifts toward a global view | |

| Option | Description | Selected |
|--------|-------------|----------|
| Server per-type caps + vector collapse | No new heuristic; graph mirrors lane membership | ✓ |
| Client top-N overall | Needs a blended cross-type ranking | |
| Separate graph cap per type | Graph and lanes disagree | |

| Option | Description | Selected |
|--------|-------------|----------|
| Out — GRAPH-04 stays v2 | No Jev this phase | ✓ |
| Order-only when reranker is on | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed ticks, then static | Deterministic seed; reduced-motion safe | ✓ |
| Animated settle | | |
| You decide | | |

| Option | Description | Selected |
|--------|-------------|----------|
| One tab stop, roving arrows | Mirrors the shipped listbox | ✓ |
| Every node a Tab stop | Up to 64 stops; fragile spatial nav | |

| Option | Description | Selected |
|--------|-------------|----------|
| Visually-hidden list + live summary | Lanes stay the visible equivalent | ✓ |
| Lanes are the equivalent | aria-describedby + live line only | |

---

## Tag cloud vs tag bars

| Option | Description | Selected |
|--------|-------------|----------|
| Linear bars (amend TAGS-01) | Sketch winner; requirement wording amended | ✓ |
| Quantile cloud + printed counts | Keeps TAGS-01 literal | |
| Both, user-toggled | | |

| Option | Description | Selected |
|--------|-------------|----------|
| /search panel + /related rail | FacetStrip "Tags" button + rail tab, one component | ✓ |
| New /tags route | | |
| /related rail only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Filter lanes in place | Sketch; dims rows/nodes lacking the tag | ✓ |
| Go to /search?tag= | Loses the neighbourhood | |

| Option | Description | Selected |
|--------|-------------|----------|
| Top 30, 'load all' to 1000 | Sketch default | ✓ |
| Top 100 (server default) | | |
| You decide | | |

| Option | Description | Selected |
|--------|-------------|----------|
| All readable scopes (rail) | Neighbours are cross-spine | ✓ |
| Anchor's scope | | |

**Notes:** Sean amended TAGS-01 and ROADMAP SC4 wording to "linear bars with printed counts".

---

## Tag chip autocomplete

| Option | Description | Selected |
|--------|-------------|----------|
| Header `#`/`tag:` + FacetStrip combobox | One shared ListTags query | ✓ |
| FacetStrip combobox only | | |
| Header only | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Follow scope chip, else all | Counts match results under the filter | ✓ |
| Always all readable | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Substring over top 1000, honest | Prefix-first ranking; `more` footer | ✓ |
| Prefix over top 100 | | |

| Option | Description | Selected |
|--------|-------------|----------|
| Allow, say why | Never silently blocked | ✓ |
| Only known tags | | |

---

## Claude's Discretion

- Curation actions on /related (none by default; "Open record" hands off to /search)
- Pan/zoom extents, tick count, force strengths, label density
- Adding /related to resume ALLOWED_DESTINATIONS
- /search Tags panel shape; ListTags query-key naming
- Loading skeleton copy

## Deferred Ideas

- GRAPH-04 Jev edge labels (v2); GRAPH-05 cross-surface tag highlight (v2)
- Neighbour↔neighbour / 2-hop edges
- Server-side ListTags prefix search
