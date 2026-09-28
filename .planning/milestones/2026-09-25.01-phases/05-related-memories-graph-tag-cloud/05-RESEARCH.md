# Phase 5: Related-Memories Graph & Tag Cloud - Research

**Researched:** 2026-09-27
**Domain:** Svelte 5 / SvelteKit 2 operator console — inline-SVG d3-force graph with keyboard/ARIA
equivalence, a `ListTags`-backed tag-bars component shared across two hosts, and a new
`/related/<id>` route on the static-adapter SPA.
**Confidence:** HIGH — every wire-shape claim below was verified by reading `proto/engram/v1/engram.proto`
and `internal/store/relatedmemories.go`/`listtags.go` directly this session (line-cited); the stack
choice (`d3-force`/`d3-drag`/`d3-zoom`/`d3-selection` `3.0.0`) was independently re-verified live
against the npm registry and the `package-legitimacy` seam today, on top of the already-HIGH
`.planning/research/STACK.md` (2026-09-25); d3 API behavior (`simulation.tick()`, `.stop()`,
`.randomSource()`, `zoom.filter`) is quoted from Context7's indexed d3 docs, not training recall.
Design decisions are almost entirely pre-locked by `05-CONTEXT.md` (D-01..D-20) and `05-UI-SPEC.md` —
this document grounds those decisions against the actual code and fills the implementation gaps
they left open (d3/Svelte 5 integration mechanics, deterministic testing, route wiring, keyboard
model extension mechanics).

## Summary

Phase 5 builds a `/related/<id>` route and a shared `TagBars`/`TagCombobox` pair on top of RPCs and
store methods that already shipped in Phases 1 and 3 — there is no server or proto work in this
phase. The two hardest technical problems are already resolved by locked decisions, and this
research confirms the code backing them exists exactly as described: (1) the "no established
precedent" edge-capping heuristic flagged in ROADMAP.md is **settled** — `internal/store/relatedmemories.go`
already enforces per-type server caps (`relatedTagCap = 8`, `relatedCitationCap = 8`,
`relatedSupersessionCap = 16` over an 8-hop-per-direction walk, `relatedTotalCeiling = 64`, vector
default `relatedVectorDefaultK = 8`), so the graph does no new scoring — it mirrors the same merged
`RelatedMemory[]` the lanes render; and (2) the graph library choice (`d3-force`/`d3-drag`/`d3-zoom`/
`d3-selection` `3.0.0`, Svelte-owns-every-DOM-node) was already researched and is reconfirmed live
today at the same versions, `OK` verdict, official `d3/*` GitHub repos, no postinstall scripts.

What remains genuinely new implementation work — and what this research grounds — is: wiring a
synchronous, deterministic 300-tick `d3-force` simulation into Svelte 5's rune reactivity without an
animated first render; a `d3-zoom` wheel filter that requires ⌘/Ctrl so a plain wheel falls through
to the sticky rail's native page scroll (verified against d3's own filter semantics, not assumed);
extending the shipped `role="listbox"`/`aria-activedescendant` roving-focus pattern (already proven
on `ResultsList.svelte`) onto an SVG region with lane-order (not spatial) arrow traversal; a new
dynamic route under the static SPA adapter (`ssr=false`, `fallback: 'index.html'`, no per-route
config needed — this is a plain client-routed segment); and a `TagBars`/`TagCombobox` pair sharing
one cached `ListTags(scope, limit=1000)` query across three entry points (rail Tags tab, `/search`
docked panel, header-search Tags group + FacetStrip "+ tag" picker).

**Primary recommendation:** Build `RelatedGraph.svelte` as a pure Svelte-owned SVG renderer fed by a
`$state` node/edge position array that a `d3-force` simulation (run to 300 ticks synchronously via a
plain `for` loop before first paint, then `.stop()`ped) computes once per membership change; use
`d3-zoom`'s `.filter()` (not a hand-rolled wheel listener) to implement the ⌘/Ctrl-to-zoom gate; copy
`ResultsList.svelte`'s roving `aria-activedescendant` listbox pattern onto the graph's `<svg>`
element rather than inventing a second keyboard model; and build `TagBars.svelte` once, parameterized
by `{ scope, mode: 'panel' | 'rail' }`, as the UI-SPEC's Component Composition section already
specifies.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Edge scoring, per-type caps, ceiling merge | API / Backend (`internal/store/relatedmemories.go`, shipped Phase 1) | — | `relatedTagCap`/`relatedCitationCap`/`relatedSupersessionCap`/`relatedTotalCeiling` are Go constants enforced server-side; the graph performs **no client-side scoring or capping** of its own (D-07) |
| Tag popularity counts, rarity weight | API / Backend (`internal/store/listtags.go`, `facetTags`, shipped Phase 1) | — | Exact recall-visible counts via Qdrant `Facet`; the client never estimates or re-derives a count |
| Graph physics (force layout, drag/zoom gesture math) | Browser / Client (`d3-force`/`d3-drag`/`d3-zoom`, new this phase) | — | Pure client-side visualization math over data already fetched; no server round-trip per tick |
| Graph rendering (every `<svg>`/`<g>`/`<circle>`/`<path>`) | Browser / Client (Svelte 5, `RelatedGraph.svelte`) | — | `STACK.md`'s locked verdict: d3 supplies math only, Svelte owns the DOM — required for native ARIA/keyboard access (canvas/WebGL alternatives were rejected specifically because they have no per-node DOM to attach `role`/`tabindex` to) |
| Route state (`/related/<id>`, trail, selection, tag filter) | Browser / Client (SvelteKit client router, `ssr=false`) | — | Static-adapter SPA; no SSR tier exists in this project (confirmed: `ui/svelte.config.js`, `+layout.js`) |
| Tag-bars/`TagCombobox` shared component + query cache | Browser / Client (`TagBars.svelte`, `TagCombobox.svelte`, TanStack Query) | API / Backend (`ListTags`, shipped) | One `ListTags(scopeKey, 1000)` query per scope key backs three UI surfaces (D-13, D-16); the client owns caching/filtering, the server owns exact counts |
| Curation actions reachable from `/related` | API / Backend (existing Phase 3/4 RPCs + dialogs) | Browser (`/search?sel=` hand-off) | `/related` is read-only this phase (D-05's "Open record ↗", UI-SPEC's resume-envelope note); no new write surface is introduced here |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `d3-force` | `3.0.0` | Force-directed layout simulation (`forceSimulation`, `forceLink`, `forceManyBody`, `forceCenter`) | [VERIFIED: npm registry, package-legitimacy seam, this session — `OK`, published 2021-06-05, 25.9M weekly downloads, repo `github.com/d3/d3-force`, no postinstall] Officially maintained d3 micro-package; already the milestone's locked choice (`.planning/research/STACK.md` §2, re-verified live today) |
| `d3-drag` | `3.0.0` | Per-node drag gesture recognition | [VERIFIED: npm registry, package-legitimacy seam, this session — `OK`, published 2021-06-09, 36.1M weekly downloads, repo `github.com/d3/d3-drag`, no postinstall] |
| `d3-zoom` | `3.0.0` | Pan/zoom gesture recognition + `.filter()`/`.scaleExtent()`/`.translateExtent()` | [VERIFIED: npm registry, package-legitimacy seam, this session — `OK`, published 2021-06-10, 35.3M weekly downloads, repo `github.com/d3/d3-zoom`, no postinstall] |
| `d3-selection` | `3.0.0` | `d3.select()` call target for `.call(drag)`/`.call(zoom)` only — never used to render nodes/edges | [VERIFIED: npm registry, package-legitimacy seam, this session — `OK`, published 2021-06-07, 37.3M weekly downloads, repo `github.com/d3/d3-selection`, no postinstall] |
| `@types/d3-force`, `@types/d3-drag`, `@types/d3-zoom`, `@types/d3-selection` | `3.0.10` / `3.0.7` / `3.0.8` / `3.0.12` | Dev-only type declarations | [VERIFIED: npm registry `npm view`, this session] All four have been at major `3` since 2022 — stable, low-churn |

None of these four packages are yet in `ui/package.json` [VERIFIED: `ui/package.json` grep, this
session — only `@humanspeak/svelte-virtual-list` from the milestone's first UI-library slot is
present]. This phase spends the milestone's **second and last** allowed small-UI-library slot
(UI-SPEC, Design System table).

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `bits-ui` `Tabs` | `^2.18.1` (already installed, unchanged) | Rail `Graph \| Tags` tabs | Already used for `/scheduled`'s tabs; second consumer, no version bump this phase (UI-SPEC explicitly defers the `2.19.3` bump) |
| `bits-ui` `Popover` + `Command` | `^2.18.1` (already installed) | `TagCombobox` "+ tag" picker | Exact `ScopeCombobox.svelte` pattern: `Command.Root shouldFilter={false}`, manual substring filter, portal disabled inside the Popover |
| `axe-core` | `4.13.0` (already installed) | WCAG 2.2 AA audit of the new graph/rail/tag surfaces | Reuses `ui/src/lib/a11y/axe.ts`'s `auditAA()` helper — no new a11y tooling needed |

### Alternatives Considered

Not re-litigated this phase — `.planning/research/STACK.md` already did the Sigma.js/Cytoscape.js/
layercake/tag-cloud-library comparison in depth (HIGH confidence, live Bundlephobia + registry
checks) and `05-CONTEXT.md`/`05-UI-SPEC.md` lock the outcome. Nothing in this session's re-verification
changed that comparison's conclusion.

**Installation:**
```bash
cd ui
pnpm add d3-force@3.0.0 d3-drag@3.0.0 d3-zoom@3.0.0 d3-selection@3.0.0
pnpm add -D @types/d3-force@3 @types/d3-drag@3 @types/d3-zoom@3 @types/d3-selection@3
```

**Version verification:** confirmed live via `npm view <pkg> version` (all four resolve to `3.0.0`,
matching `STACK.md`'s 2026-09-25 finding, no drift in the two days since) and via the
`package-legitimacy` seam (all four `OK`, high download counts, official `d3/*` repos, no
`postinstall` scripts).

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `d3-force` | npm | ~5.3 yrs (since 2021-06-05) | 25.9M/wk | `github.com/d3/d3-force` | OK | Approved |
| `d3-drag` | npm | ~5.3 yrs (since 2021-06-09) | 36.1M/wk | `github.com/d3/d3-drag` | OK | Approved |
| `d3-zoom` | npm | ~5.3 yrs (since 2021-06-10) | 35.3M/wk | `github.com/d3/d3-zoom` | OK | Approved |
| `d3-selection` | npm | ~5.3 yrs (since 2021-06-07) | 37.3M/wk | `github.com/d3/d3-selection` | OK | Approved |

**Packages removed due to `[SLOP]` verdict:** none.
**Packages flagged as suspicious `[SUS]`:** none.

All four are official d3.js project packages (part of the `d3` umbrella, `github.com/d3` org),
independently cross-checked via Context7's `/d3/d3-force` and `/d3/d3` indexed docs (High source
reputation) in addition to the registry check — this satisfies the stricter bar for `[VERIFIED]`
(an authoritative source, not registry-existence alone). No `checkpoint:human-verify` gating is
needed for these four installs.

## Architecture Patterns

### System Architecture Diagram

```
                     ┌─────────────────────────────────────────────┐
                     │  Entry points (D-03)                        │
                     │  DetailPane "Related" button │ row key `r`  │
                     │  ⌘K "Related to {short_id}"                 │
                     └───────────────────┬──────────────────────────┘
                                         │ goto(`/related/${id}`)
                                         ▼
┌────────────────────────────────────────────────────────────────────────┐
│  /related/[id]/+page.svelte  (SvelteKit client route, ssr=false)       │
│                                                                        │
│  page.params.id ──▶ createQuery(['relatedMemories', id, k, full])     │
│                        │                                              │
│                        ▼                                              │
│              engram.relatedMemories({id, k, full})  (Connect, read)   │
│                        │                                              │
│                        ▼                                              │
│        RelatedMemoriesResponse { anchor, related[], truncated }       │
│                        │                                              │
│         ┌──────────────┼───────────────────────┐                     │
│         ▼              ▼                       ▼                     │
│   Anchor bar      Lanes (body)          Rail (sticky, tabs)          │
│   trail+back+     supersession/         ┌─── Graph tab ───┐          │
│   anchor card     citation/tag/         │ RelatedGraph.svelte        │
│                    vector rows          │  d3-force layout (300      │
│                                         │  ticks, sync, seeded)      │
│                                         │  d3-zoom (⌘+wheel only)    │
│                                         │  d3-drag (spring-back)     │
│                                         │  Svelte renders every      │
│                                         │  <g>/<circle>/<path>       │
│                                         └──────┬──────────────┘      │
│                                                │ select/re-centre    │
│                                                ▼                     │
│                                    Evidence section (D-05)           │
│                                    shared selection state ───────────┼──▶ lanes + graph + chain
│                                                │                     │    (one lit candidate)
│                                                ▼                     │
│                                    "Open record ↗" → /search?sel=id  │
│                                    "Re-centre ↵" → goto(/related/id2)│
│                                                                       │
│                                         ┌─── Tags tab ────┐          │
│                                         │ TagBars.svelte           │
│                                         │  mode='rail'             │
│                                         │  ListTags(all-scopes,1000)│
│                                         └──────┬───────────┘        │
│                                                │ click bar          │
│                                                ▼                    │
│                                    in-view filter chip (D-14):      │
│                                    dims lane rows + graph nodes     │
│                                    lacking the tag to 30%           │
└────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────┐
│  /search  (existing route)                                         │
│  FacetStrip "▦ Tags panel" toggle ──▶ TagBars.svelte mode='panel'  │
│    shares the DetailPane right slot; click bar → toggles #tag chip │
│  FacetStrip "+ tag" TagCombobox ──▶ same cached ListTags query     │
│  HeaderSearch "#"/"tag:" ──▶ Tags suggestion group, same query     │
└────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
ui/src/routes/related/[id]/
├── +page.svelte              # route shell: anchor bar, lanes, rail host
└── related.browser.test.ts   # route-level vitest-browser test (follows search.browser.test.ts's shape)

ui/src/lib/components/
├── RelatedGraph.svelte        # new — owns <svg>, $state node/edge array, d3-force/drag/zoom mount
├── RelatedGraph.browser.test.ts
├── RelatedLanes.svelte        # new — the four edge-type lane cards (or one component per lane; planner's call)
├── EvidenceSection.svelte     # new — D-05's "why it is related" panel, shared by node/row/chain-card selection
├── TagBars.svelte             # new — shared bar-list component, mode='panel'|'rail'
├── TagBars.browser.test.ts
├── TagCombobox.svelte         # new — "+ tag" picker, mirrors ScopeCombobox.svelte
└── TagCombobox.browser.test.ts

ui/src/lib/related/
├── graph.ts                   # pure functions: build node/edge arrays from RelatedMemoriesResponse,
│                               #   lane-order comparator, membership diffing for refit-vs-retain (D-20)
└── graph.test.ts              # node-testable pure logic (no DOM), mirrors curation/chain.ts's split

ui/src/lib/tags/
├── tags.ts                    # pure functions: substring+prefix ranking (D-18), top-30/show-all slicing (D-15)
└── tags.test.ts
```

This mirrors the existing `ChainDialog.svelte` + `$lib/curation/chain.ts` split (pure model function,
dumb rendering component) that the codebase already uses for supersession-chain logic reachable
through the same `RelatedMemories` RPC.

### Pattern 1: Synchronous, deterministic force-layout settle (D-09)

**What:** Run the `d3-force` simulation to a fixed tick count with a seeded random source, before
Svelte ever renders a node position — no `simulation.on('tick', render)` animation loop for the
initial layout.

**When to use:** Every time the graph's node/edge membership changes (initial anchor load, vector
lane expand/collapse, a legend/type toggle, re-centre) — this is the same event set D-20 says
triggers a **refit**.

**Example:**
```typescript
// Source: Context7 /d3/d3-force ("Perform Time-Limited Warm-Up", "simulation.tick(iterations)")
// https://github.com/d3/d3-force/blob/main/_autodocs/INTEGRATION_GUIDE.md
import { forceSimulation, forceLink, forceManyBody, forceCenter } from 'd3-force';

const simulation = forceSimulation(nodes)
  .force('link', forceLink(edges).id((d) => d.id).distance(60))
  .force('charge', forceManyBody().strength(-120))
  .force('center', forceCenter(width / 2, height / 2))
  .randomSource(seededRandom) // D-09: "seed the layout deterministically"
  .stop(); // prevent the internal requestAnimationFrame-driven timer entirely

for (let i = 0; i < 300; i++) simulation.tick(); // D-09/D-20: fixed 300-tick settle budget

// Positions are now final. Copy them into a Svelte $state array ONCE —
// do not keep the simulation "running" or subscribe to further ticks here.
nodePositions = nodes.map((n) => ({ id: n.id, x: n.x, y: n.y }));
```

`simulation.randomSource(source)` [CITED: Context7 `/d3/d3-force`, `forceSimulation.md`/`types.md`]
takes any `() => number` in `[0, 1)` — a small seeded PRNG (e.g. a mulberry32/xorshift one-liner,
no new dependency needed) makes every render/test produce byte-identical layout for the same input
graph, which is what makes a vitest-browser screenshot test of the graph viable at all.

### Pattern 2: `d3-zoom` wheel gate — plain wheel scrolls, ⌘/Ctrl+wheel zooms (D-20)

**What:** d3-zoom's **default** `.filter()` already treats every `wheel` event as zoom-eligible
regardless of modifier keys — `(!event.ctrlKey || event.type === 'wheel') && !event.button`
[CITED: Context7 `/d3/d3`, `d3-zoom.md`, "Define zoom filter function"]. That default is the
**opposite** of D-20's requirement (plain wheel must scroll the sticky rail's page, not zoom). A
custom filter is required — this is not a default-config toggle.

**When to use:** On the zoom behavior's own `.filter()`, not as a separate manual wheel listener
racing d3-zoom's own.

**Example:**
```typescript
// Source: Context7 /d3/d3, d3-zoom.md ("zoom.filter", "Disable wheel-driven zooming")
import { zoom as d3zoom, zoomIdentity } from 'd3-zoom';
import { select } from 'd3-selection';

const zoomBehavior = d3zoom()
  .scaleExtent([0.5, 4]) // D-20
  .translateExtent([[0, 0], [width, height]]) // D-20: "bounded to the graph extent"
  .filter((event) => {
    if (event.type === 'wheel') return event.ctrlKey || event.metaKey; // D-20's ⌘/Ctrl gate
    return !event.button; // drag/touch gestures still pass through normally
  })
  .on('zoom', (event) => { transform = event.transform; }); // feeds a Svelte $state, not d3-managed DOM

select(svgEl).call(zoomBehavior);
```
When `.filter()` returns `false` for a plain wheel, d3-zoom ignores the event entirely (no
`preventDefault()`), so the browser's native scroll on the sticky rail proceeds — this is exactly
the mechanism the "flash the hint for ~0.9s on a plain wheel" behavior (UI-SPEC copy table) hooks
into: listen for the *raw* `wheel` event alongside the zoom behavior (not instead of it) to trigger
the hint, independent of whether d3-zoom itself reacted.

### Pattern 3: Roving-focus keyboard model on an SVG region (D-10)

**What:** The exact `role="listbox" tabindex="0"` + `aria-activedescendant` pattern
`ResultsList.svelte` already ships and already has a passing test suite — reapply it to the graph's
`<svg>` element with `role="option"` on each node's `<g>`, and a **lane-order** (not spatial)
comparator for arrow-key traversal.

**Verified precedent** [VERIFIED: `ui/src/lib/components/ResultsList.browser.test.ts:1-90`, read this
session]: the shipped test asserts `listbox.element().focus()` sets `aria-activedescendant` to the
first option's id, `{End}`/`{Home}` jump to the last/first, `j`/`k` move one step and clamp at the
ends, and `{Enter}` fires a callback without moving DOM focus off the listbox. The graph's own test
suite should assert the identical shape, substituting the lane-order comparator (supersession ›
citation › tag › vector, then strength within lane — D-10) for `ResultsList`'s list-index order, and
Space/click in place of `ResultsList`'s Enter-opens.

```typescript
// Pattern to replicate in RelatedGraph.svelte's keydown handler — id/element
// naming follows ResultsList.svelte's own convention (opt-<id> / aria-activedescendant)
function laneOrderIndex(node: GraphNode): number {
  // supersession=0, citation=1, tag=2, vector=3, then by within-lane strength
}
const orderedIds = $derived(nodes.slice().sort((a, b) => laneOrderIndex(a) - laneOrderIndex(b)).map((n) => n.id));
```

### Pattern 4: Dynamic route on the static SPA adapter

**What:** `/related/[id]` needs **no special SvelteKit config** beyond the route file itself.

**Verified** [VERIFIED: `ui/svelte.config.js:1-15`, `ui/src/routes/+layout.js:1-2`, read this
session]: `adapter-static({ fallback: 'index.html' })` plus root `ssr = false` / `prerender = false`
means every route — static or dynamic-segment — is resolved entirely client-side after the static
`index.html` fallback loads; there is no prerendering step that would need an `entries()` export for
`[id]`. This is the same mechanism `/rules?sel=` and `/scheduled?sel=` already rely on for
query-string state; `/related/[id]` is the first **path-segment** parameter in this codebase, but
the underlying mechanism (client router owns everything) is identical.

```typescript
// Source: ui/src/routes/scheduled/+page.svelte:9 pattern, applied to a path param
// instead of a query param
import { page } from '$app/state';
const id = $derived(page.params.id); // SvelteKit's typed route param, from +page.svelte in [id]/
```

Test mocking follows the same `vi.mock('$app/state', () => ({ page: pageState }))` shape already
used in `rules.browser.test.ts`/`scheduled.browser.test.ts`/`search.browser.test.ts` — `pageState`
just needs a `params: { id: '...' }` field alongside the existing `url` field.

### Anti-Patterns to Avoid

- **Letting d3 touch the DOM.** `d3.select()` is used only as `.call(zoomBehavior)`/`.call(dragBehavior)`'s
  target — never `d3.select(...).append('circle')` or similar. Every `<circle>`/`<path>`/`<g>` must
  be a Svelte-rendered element from an `{#each}` over `$state`, or keyboard/ARIA attributes silently
  stop being reactive to Svelte's own state changes.
- **Keeping the simulation "running" via `.on('tick', ...)` for the initial settle.** D-09 requires a
  synchronous settle before first paint — an animated tick-by-tick render for 300 ticks would violate
  "no visible 'graph is still moving' state" (UI-SPEC E2 loading row) and cost far more than the
  fixed budget implies.
- **A manual `wheel` event listener that calls `preventDefault()` unconditionally.** This would break
  native page scroll on the sticky rail even when d3-zoom's own filter rejected the gesture — gate on
  the same `ctrlKey || metaKey` check the zoom filter uses, and never call `preventDefault()` in the
  hint-flash listener itself.
- **A second keyboard-model implementation diverging from `ResultsList.svelte`'s.** Arrow direction
  in the graph is lane-order, not spatial, precisely so the same mental model (and largely the same
  code shape) as the shipped listbox applies — do not invent a directional/spatial nav scheme.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Force-directed layout math | A custom spring/repulsion simulation | `d3-force`'s `forceSimulation`/`forceLink`/`forceManyBody`/`forceCenter` | Verlet-integration physics with 15+ years of tuning; reinventing this for 64 nodes is pure risk with no benefit |
| Pan/zoom gesture recognition (wheel, pinch, drag-to-pan) + bounded scale/translate | Hand-rolled pointer/wheel math | `d3-zoom` (`scaleExtent`, `translateExtent`, `.filter()`) | Cross-browser wheel-delta normalization and touch-pinch-as-wheel handling are exactly the kind of platform quirks a maintained library absorbs |
| Per-node drag with spring-back release | Hand-rolled pointer-capture drag math | `d3-drag` + clearing `fx`/`fy` on `dragend` (the standard d3-force "drag" recipe) | The alpha-reheat-then-cool mechanic is `d3-force`'s own contract (`alphaTarget`); building it independently would fight the simulation instead of using its API |
| Tag popularity ranking (substring + prefix-first + by-count) | A fuzzy-match library | Plain `Array.prototype.filter`/`.sort()` over the cached top-1000 `ListTags` response | D-18: "substring over top 1000... results rank prefix matches first, then by count" — this is a ~15-line pure function, not a search-library problem |
| Roving keyboard focus / `aria-activedescendant` | A third-party a11y-listbox library | The already-shipped `ResultsList.svelte` pattern, copied | The codebase already has a WCAG-audited, test-covered implementation of exactly this pattern one file away |

**Key insight:** every "hard" problem in this phase either has a locked server-side answer already
(capping/scoring), a maintained micro-library that solves exactly this shape (d3's force/zoom/drag
split), or a shipped in-repo precedent to copy (the listbox). There is no genuinely novel algorithm
to design in this phase — the work is wiring, not invention.

## Runtime State Inventory

Not applicable — this is a greenfield phase (new route, new components) with no rename, refactor,
or data migration. No existing stored data, service config, OS-registered state, secrets, or build
artifacts reference anything this phase changes.

## Common Pitfalls

### Pitfall 1: Treating the shipped d3-zoom default filter as already correct

**What goes wrong:** Wiring `d3zoom()` with no `.filter()` override "works" in the sense that
scrolling zooms — but violates D-20 outright (a plain wheel over the graph must scroll the sticky
rail's page, not zoom it), and the mistake is easy to miss in a quick manual check because zoom-on-
wheel *feels* like the expected default behavior for a graph widget.
**Why it happens:** d3-zoom's actual default filter (`(!event.ctrlKey || event.type === 'wheel') &&
!event.button`) is non-obvious — it does *not* mean "wheel only zooms with ctrl," it means "wheel
always zooms regardless of ctrl" (the `event.type === 'wheel'` clause short-circuits the ctrlKey
check for wheel events specifically). [CITED: Context7 `/d3/d3`, d3-zoom.md]
**How to avoid:** Always set an explicit `.filter()` per Pattern 2 above; add a vitest-browser test
that dispatches a bare `wheel` event (no modifier) over the graph and asserts the zoom transform is
unchanged and the page's scroll position moved.
**Warning signs:** A manual smoke test where scrolling the mouse wheel over the graph zooms it
without holding a modifier.

### Pitfall 2: No `--header-height` CSS custom property exists yet for the sticky rail offset

**What goes wrong:** Copying the sketch's `.rail { top: 56px; }` literally (as UI-SPEC's Deltas
table already flags) breaks the sticky offset at any text-size setting other than the sketch's own
fixed baseline.
**Why it happens:** [VERIFIED: `ui/src/lib/components/AppShell.svelte:27-37`, read this session —
the header is `<header class="flex items-center gap-3 px-3 py-2 border-b border-border">`, padding-
driven with no fixed height and **no existing CSS custom property** exposing its rendered height]
There is nothing today to read the offset from — this is new plumbing, not a lookup of an existing
value.
**How to avoid:** Either (a) have `AppShell.svelte` set a CSS custom property (e.g.
`--app-header-h`) via `getBoundingClientRect()` in an `$effect`/`ResizeObserver`, exposed to every
descendant route, or (b) have `RelatedGraph`'s rail measure its own ancestor header via
`getBoundingClientRect()` directly. Option (a) is more reusable if a future phase needs the same
offset elsewhere; option (b) is more contained to this phase. Flagged as an open question below —
UI-SPEC explicitly leaves the choice to the executor ("a CSS custom property the layout already
sets, **or** `getBoundingClientRect()` if none exists yet").
**Warning signs:** The rail's top edge visibly overlaps or gaps from the header at 12px or 16px text
size (the two non-default settings the `--u` scaling rule exists to protect).

### Pitfall 3: Asserting d3's physics output instead of node/edge membership and ARIA state

**What goes wrong:** A test that asserts exact `(x, y)` pixel coordinates for a node is fragile
across d3-force version bumps, browser floating-point differences, or an unrelated force-strength
tuning change (an explicit "Claude's Discretion" item in CONTEXT.md) — and it tests the library's
math, not this codebase's logic.
**Why it happens:** It's tempting to snapshot-test the whole rendered SVG including positions once
the deterministic seed makes it reproducible.
**How to avoid:** Phase 4 D-18 already states this rule explicitly for this milestone ("Do not
assert d3's physics; assert our node/edge membership, caps mirroring, keyboard traversal order, ARIA
names, and filter/chip behaviour"). Assert `screen.container.querySelectorAll('[role="option"]').length`,
`aria-activedescendant` values, node accessible names (`{short_id}, {category}, {edge types}, {state
words}` per D-11), and the visually-hidden neighbourhood list's text content — not coordinates.
**Warning signs:** A test file importing anything from `d3-force`'s own exports to compute an
expected position.

### Pitfall 4: Forgetting the vector-lane/graph membership-mirroring invariant when implementing "show all"

**What goes wrong:** If the lane's "show all N" expansion and the graph's node set are wired to two
different pieces of state (e.g. the lane reads a `showAllVector` boolean but the graph reads the
raw `RelatedMemoriesResponse.related` length), they can desync — showing 8 vector rows in the lane
while the graph still draws all N, or vice versa.
**Why it happens:** D-07/D-20 require the graph to "mirror the lane's collapse" — this is a shared
derived-state requirement, not two independent features that happen to look similar.
**How to avoid:** Keep exactly one `$state`/`$derived` boolean (e.g. `vectorExpanded`) that both the
lane's slice-to-8 and the graph's node-filter read from; the "refit" trigger (D-20) fires off that
same state change. A single vitest-browser test that toggles "show all" and asserts both the lane
row count and the graph's rendered `[role="option"]` count changed together closes this gap.
**Warning signs:** The graph and lane show different counts for the same collapsed/expanded state
during manual testing.

## Code Examples

### Building the node/edge arrays from `RelatedMemoriesResponse` (star + chain arrows, D-06)

```typescript
// Source: proto/engram/v1/engram.proto:640-682 (RelatedEdge/RelatedMemory/RelatedMemoriesResponse
// shapes, read this session) + 05-CONTEXT.md D-06
import { EdgeType } from '$lib/gen/engram_pb';
import type { RelatedMemoriesResponse } from '$lib/gen/engram_pb';

interface GraphNode { id: string; shortId: string; category: string; isAnchor: boolean; }
interface GraphEdge { source: string; target: string; type: EdgeType; offsetIndex: number; }

function buildGraph(resp: RelatedMemoriesResponse): { nodes: GraphNode[]; edges: GraphEdge[] } {
  const nodes: GraphNode[] = [{ id: resp.anchor!.id, shortId: resp.anchor!.shortId, category: resp.anchor!.category, isAnchor: true }];
  const edges: GraphEdge[] = [];
  for (const rel of resp.related) {
    if (!rel.memory) continue;
    nodes.push({ id: rel.memory.id, shortId: rel.memory.shortId, category: rel.memory.category, isAnchor: false });
    // One edge per type reached (D-06's offset-curve rule when >1 type) — never
    // merged into one blended edge.
    rel.edges.forEach((edge, i) => edges.push({ source: resp.anchor!.id, target: rel.memory!.id, type: edge.type, offsetIndex: i }));
  }
  // Chain arrows: supersession edges ALSO draw a direct predecessor->successor
  // arrow between chain members when both are drawn (D-06) — derive from each
  // RelatedEdge's SupersessionEvidence.depth/direction, never a second RPC call.
  return { nodes, edges };
}
```

### `TagBars.svelte`'s shared query (D-13, D-15, D-18 — one fetch, three consumers)

```typescript
// Source: 05-CONTEXT.md D-15/D-18, engram-connect-client SKILL.md's ListTags row
// (query-key prefix 'listTags'), this session
const scopeKey = $derived(scope || '__all_readable__'); // D-17: empty scope = every readable scope
const tagsQuery = createQuery(() => ({
  queryKey: ['listTags', scopeKey, 1000],
  queryFn: ({ signal }) => engram.listTags({ scope, limit: 1000n }, { signal }),
  staleTime: Infinity // D-15: "the same cached ListTags(limit=1000) response the autocomplete uses"
}));
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| ROADMAP's "no established precedent" framing for edge capping | Server per-type caps already shipped in Phase 1 (`relatedTagCap`/`relatedCitationCap`/`relatedSupersessionCap`/`relatedTotalCeiling`) | Phase 1 (completed 2026-09-26), reconfirmed by D-07 | The graph does zero new scoring/capping work — it is a pure rendering layer over an already-capped, already-merged response |
| Sketch 004's evidence-drawer-over-the-graph | Evidence section under the graph (D-05, sketch 006 A1) | 2026-09-27 (this phase's own discuss-phase session) | Every drawer-shaped HTML/CSS fragment in `related-and-tags.md`'s sketch corpus is historical, not buildable — the executor must not copy it |
| TAGS-01's original "cloud sized by count quantile" wording | Linear bars with printed counts (D-12) | 2026-09-27, same session | `.planning/REQUIREMENTS.md` and ROADMAP's Phase 5 SC4 are both already amended; no further requirements-doc work needed this phase |

**Deprecated/outdated:** Sketch 004's radial-graph layout and log-scaled/treemap tag-cloud variants
are explicitly rejected in `related-and-tags.md`'s "What to Avoid" section — do not resurrect either
even as a "simpler first pass."

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A small hand-written seeded PRNG (mulberry32/xorshift-style, no new dependency) is an acceptable way to satisfy `simulation.randomSource()`'s deterministic-seed requirement, rather than a dedicated seeded-random npm package | Pattern 1 (Code Examples) | Low — this is a ~5-line pure function; if the team prefers a named library (e.g. `seedrandom`), it would be a third small dependency beyond the two-library budget, which D-20's "Claude's Discretion: the exact tick count, force strengths" note suggests is not required |
| A2 | The header-height CSS custom property does not yet exist and must be added this phase (rather than the planner discovering one under a different name) | Pitfall 2 | Low-medium — confirmed absent by reading `AppShell.svelte` in full this session, but a differently-named existing mechanism (e.g. a Tailwind arbitrary-value convention elsewhere) was not exhaustively ruled out across every CSS file in `ui/src` |
| A3 | `RelatedLanes.svelte`/`EvidenceSection.svelte`/`graph.ts`/`tags.ts` file names and the one-component-per-lane vs. one-component-for-all-lanes split are reasonable defaults, not binding | Recommended Project Structure | Low — purely organizational; the planner is free to split differently as long as the pure-model/dumb-render separation (mirroring `curation/chain.ts` + `ChainDialog.svelte`) is preserved |

**If this table is empty:** N/A — three low-risk assumptions logged above; none touch a locked
decision, a compliance requirement, or a security-relevant boundary.

## Open Questions

1. **Where does the sticky-rail header-offset CSS custom property live?**
   - What we know: `AppShell.svelte`'s header has no fixed height and exposes no existing offset
     variable (verified this session); UI-SPEC leaves the choice between "add one to the layout" and
     "measure locally via `getBoundingClientRect()`" to the executor.
   - What's unclear: Whether a future phase would also want this offset (favoring a shared layout-level
     variable) or whether `/related` is the only consumer for the foreseeable future (favoring a local
     measurement, less blast radius).
   - Recommendation: Default to a local `getBoundingClientRect()` measurement inside `RelatedGraph`'s
     rail wrapper (lowest blast radius, no `AppShell.svelte` edit) unless the planner identifies another
     near-term consumer.

2. **Exact file/component split for the four lanes (one `RelatedLanes.svelte` vs. four
   `*Lane.svelte` files, one per edge type)**
   - What we know: The supersession lane is structurally distinct (horizontal timeline vs. the other
     three's one-line-row list) — UI-SPEC's Deltas table already calls this out as "its own layout, own
     column labels," distinct from `ChainDialog`'s modal grid.
   - What's unclear: Whether sharing row-rendering logic across citation/tag/vector lanes (three
     structurally similar one-line-row lists differing only in their evidence-inline rendering) is
     better served by one parameterized `EdgeLane.svelte` or three separate files.
   - Recommendation: One parameterized `EdgeLane.svelte` for citation/tag/vector (they share the
     `.row` grid template and `×N` badge exactly, per `related-and-tags.md`'s CSS), plus a distinct
     `SupersessionLane.svelte` for the timeline — this is a planning-time call, not a research gap.

## Environment Availability

Not applicable — this phase has no external service/tool dependency beyond what's already installed
in `ui/` (Node/pnpm toolchain, already verified functioning by every prior phase in this milestone).

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | vitest `5.0.1` + `vitest-browser-svelte` (browser project, Chromium via Playwright `1.63.0`) + node project for pure-function tests [VERIFIED: `ui/package.json`, `ui/vite.config.ts`, read this session] |
| Config file | `ui/vite.config.ts` (two projects: `node` for `src/**/*.test.ts` excluding `*.browser.test.ts`, `browser` for `*.browser.test.ts`) |
| Quick run command | `cd ui && pnpm test:browser -- RelatedGraph` (single new component); `cd ui && pnpm test:node -- graph.test` (pure-function logic) |
| Full suite command | `cd ui && pnpm test` (runs both `test:node` and `test:browser` projects) |

**Note (same finding Phase 4's research already recorded, reconfirmed):** `task test` (the Go-facing
repo gate) does **not** run `ui/`'s vitest suite [VERIFIED: `Taskfile.yaml` — `test`/`test:go`/
`test:python` blocks, no `ui:test` target exists, read this session]. This phase's verification loop
must invoke `pnpm test`/`pnpm test:browser` directly under `ui/`.

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| GRAPH-01 | Graph renders anchor+neighbourhood only, pan/zoom/drag, settle budget, capped edges, legend+toggles | vitest-browser (`RelatedGraph.svelte`) | `pnpm test:browser -- RelatedGraph` | ❌ Wave 0 — new |
| GRAPH-02 | Keyboard reachability (Tab/arrows/Enter/Escape), accessible names, `aria-live` neighbourhood list | vitest-browser, same file, following `ResultsList.browser.test.ts`'s `userEvent.keyboard`/`fireKey` shape | `pnpm test:browser -- RelatedGraph` | ❌ Wave 0 — new; precedent exists at `ResultsList.browser.test.ts:1-90` |
| GRAPH-03 | Light/dark category tokens, node click selects (evidence section, D-05's stand-in for "detail pane") | vitest-browser, both-theme screenshot pattern (Phase 4 D-18 precedent) | `pnpm test:browser -- RelatedGraph` | ❌ Wave 0 — new |
| TAGS-01 | Tag bars from `ListTags`, DOM order = reading order, click adds filter chip | vitest-browser (`TagBars.svelte`) + node (`tags.ts` slicing/sort pure functions) | `pnpm test:browser -- TagBars` / `pnpm test:node -- tags.test` | ❌ Wave 0 — new |
| TAGS-02 | Chip autocomplete with counts (header search group + `TagCombobox`) | vitest-browser (`TagCombobox.svelte`, `HeaderSearch.browser.test.ts` extended) | `pnpm test:browser -- TagCombobox` / `pnpm test:browser -- HeaderSearch` | ❌ Wave 0 — new component; existing file extended |

### Sampling Rate
- **Per task commit:** `pnpm test:browser -- <touched component>` for the component just changed.
- **Per wave merge:** `pnpm test` (full `ui/` suite, both projects).
- **Phase gate:** `pnpm test` + `pnpm check` (svelte-check) + `task ui:build` (SPA rebuild, vendors
  into `internal/webauth/static`) + the `ui-drift` CI job's local equivalent (`git diff
  internal/webauth/static` after rebuild) green before `/gsd-verify-work`. No Go-side or chromedp
  e2e test is required this phase — `/related` introduces no new server RPC and no new write path,
  so `internal/e2e`'s existing coverage is unaffected (unlike Phase 4, which added a chromedp round
  trip specifically for its new write surfaces).

### Wave 0 Gaps
- [ ] `ui/src/lib/components/RelatedGraph.svelte` + `.browser.test.ts` — new
- [ ] `ui/src/lib/components/TagBars.svelte` + `.browser.test.ts` — new
- [ ] `ui/src/lib/components/TagCombobox.svelte` + `.browser.test.ts` — new
- [ ] `ui/src/lib/related/graph.ts` + `.test.ts` — new pure-model logic (node-testable)
- [ ] `ui/src/lib/tags/tags.ts` + `.test.ts` — new pure-model logic (node-testable)
- [ ] `ui/src/routes/related/[id]/+page.svelte` + `.browser.test.ts` — new route, following
      `ui/src/routes/scheduled/scheduled.browser.test.ts`'s `vi.mock('$app/state', ...)` shape
- [ ] A seeded-random helper for the deterministic 300-tick settle (Assumption A1) — small enough to
      inline in `graph.ts` rather than a new file, but the planner should decide placement

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (indirect) | Unchanged — `/related` is a read-only route over the already-authenticated Connect `engram` (read) client; no new auth surface |
| V3 Session Management | no (indirect only) | No new write RPC this phase, so no new CSRF surface; `RelatedMemories`/`ListTags` are both already-shipped read RPCs routed through `engram`, not `engramWrite` |
| V4 Access Control | yes (indirect) | Server-enforced only — `RelatedMemories`/`ListTags` already compose the caller's read predicate server-side (Phase 1 STORE-02/STORE-03); the graph/tag-bars render exactly what the server returned, never re-deriving visibility client-side |
| V5 Input Validation | yes (indirect) | The `/related/[id]` route param is passed straight through to `RelatedMemoriesRequest.id` — the server's existing id/short_id resolution and not-found handling apply unchanged; the client performs no local id-shape validation beyond what `classify.ts` already does elsewhere |
| V6 Cryptography | no | No new crypto surface |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| An `id`-shaped route param used to probe for record existence (timing/error-shape oracle) | Information Disclosure | Already mitigated server-side — an unreadable or nonexistent anchor returns the same `not_found`-shaped rejection regardless of which is true [VERIFIED: `proto/engram/v1/engram.proto:673-677`, "An unreadable or nonexistent anchor reads not_found, echoing only the caller's original input"]. The route renders the shared not-found copy (UI-SPEC Copywriting Contract) — no new client-side branching that could leak which case occurred. |
| Cross-surface data leakage via the graph rendering a category/tag from a record the caller cannot fully read | Information Disclosure | Not applicable — `RelatedMemory.memory` is always compact-or-full per the caller's own `full` request flag and the server's read filter; a record the caller cannot read is never present in `related[]` to begin with (composed into the Qdrant filter server-side, not post-filtered) |
| A crafted `#tag` value in the `TagCombobox`'s "Add #{tag}" free-entry path used to inject an unexpected filter or an XSS-shaped tag string rendered unescaped | Tampering / (minor) XSS | Not a new risk this phase — tag values are rendered as plain text content (Svelte's default text-interpolation auto-escapes), and the "Add #{tag}" affordance only ever constructs a `tags` facet chip / `ListTags`-shaped read filter, never a write; no server-side tag creation happens client-side |

## Sources

### Primary (HIGH confidence — read in full this session)
- `.planning/phases/05-related-memories-graph-tag-cloud/05-CONTEXT.md` — locked decisions D-01..D-20
- `.planning/phases/05-related-memories-graph-tag-cloud/05-UI-SPEC.md` — visual/interaction contract
- `.planning/phases/05-related-memories-graph-tag-cloud/05-DISCUSSION-LOG.md`
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` (Phase 5 section + milestone-goal prose)
- `.planning/research/STACK.md` (d3-force/drag/zoom/selection verdict, live-checked 2026-09-25)
- `proto/engram/v1/engram.proto:577-742` (EdgeType, RelatedEdge/RelatedMemory/RelatedMemoriesRequest/
  Response, TagCount/ListTagsRequest/Response, service RPC list)
- `internal/store/relatedmemories.go:150-368` (all per-type caps: `relatedVectorDefaultK`,
  `relatedTotalCeiling`, `relatedSupersessionDepth`/`Cap`, `relatedTagCap`/`ProbeCap`,
  `relatedCitationCap`/`ProbeCap`, the compile-time invariant assertion)
- `internal/store/listtags.go:1-80` (`listTagsDefaultLimit = 100`, `recallVisibleFilter`, `facetTags`)
- `.claude/skills/engram-connect-client/SKILL.md` (per-RPC contract for `RelatedMemories`/`ListTags`,
  query-key conventions, curation-invalidation list)
- `.claude/skills/engram-console-conventions/SKILL.md` (tokens, state words, keyboard model,
  `--u` scaling rule)
- `.claude/skills/sketch-findings-engram/references/related-and-tags.md` (primary design source,
  CSS/HTML patterns, edge encoding, "what to avoid")
- `ui/src/lib/components/ChainDialog.svelte`, `ScopeCombobox.svelte`, `ResultsList.browser.test.ts`,
  `FacetStrip.svelte`, `RowActions.svelte`, `CommandMenu.svelte`, `ui/src/lib/memorystate.ts`,
  `ui/src/lib/resume.ts` (`ALLOWED_DESTINATIONS`), `ui/src/lib/components/AppShell.svelte`
- `ui/svelte.config.js`, `ui/src/routes/+layout.js`, `ui/vite.config.ts`, `ui/package.json`
- `ui/src/routes/scheduled/+page.svelte`, `rules/+page.svelte`, `discovery/+page.svelte` (`$app/state`
  `page` usage pattern, `vi.mock('$app/state', ...)` test-mocking convention)
- `Taskfile.yaml` (`test`/`test:go`/`test:python`/`ui:build` blocks — no `ui:test` target)
- `CLAUDE.md` (project instructions)

### Secondary (MEDIUM confidence)
- Context7 `/d3/d3-force` — `simulation.tick(iterations)`, `simulation.stop()`,
  `simulation.randomSource()`, the "Perform Time-Limited Warm-Up" integration-guide snippet — quoted
  directly from d3's own indexed docs, High source reputation.
- Context7 `/d3/d3` — `zoom.filter`, `zoom.scaleExtent`, `zoom.translateExtent`, the default filter
  function and the "Disable wheel-driven zooming" recipe — quoted directly, High source reputation.

### Tertiary (LOW confidence)
- None — every claim in this document was either read directly from the codebase/proto this session,
  re-verified live against the npm registry and package-legitimacy seam, or quoted from Context7's
  indexed official d3 docs. The two Open Questions above are flagged as genuinely undecided rather
  than answered on low-confidence grounds.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — live registry + package-legitimacy re-verification of a stack already
  independently researched two days prior, unchanged.
- Architecture: HIGH — the hard architectural question (edge-capping precedent) is resolved by
  reading the actual shipped Go constants, not inferred; the d3/Svelte integration pattern is
  documented by the milestone's own `STACK.md` and confirmed against Context7's official docs for
  the specific APIs (`filter`, `randomSource`, `tick`) this phase newly exercises.
- Pitfalls: HIGH — all four pitfalls are grounded in either a direct code read (header CSS, the
  vector-lane mirroring invariant) or a precise reading of d3's own documented default behavior
  (the zoom-filter gotcha), not speculation.

**Research date:** 2026-09-27
**Valid until:** 30 days (stable domain — d3's `3.x` line has had no major bump since 2022; the
locked design decisions in `05-CONTEXT.md`/`05-UI-SPEC.md` do not expire on their own timeline)
