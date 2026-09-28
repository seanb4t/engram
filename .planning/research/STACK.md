# Stack Research

**Domain:** SvelteKit 2.70 / Svelte 5.57 operator console — dense virtualized recall list, related-memories graph, tag cloud, debounced search, scope autocomplete/command palette (engram milestone 2026-09-25.01, Console Overhaul)
**Researched:** 2026-09-25
**Confidence:** HIGH — every package version below was checked live against the npm registry (`npm view`) and/or Context7's indexed source docs on 2026-09-25; bundle sizes are live Bundlephobia figures for the pinned versions; the bits-ui and TanStack Query behavior claims are quoted from each project's own docs via Context7, not training-data recall.

This file answers the six open questions carried in `.planning/research/questions.md` under "Console overhaul (2026-09-25)" and corrects/extends the provisional dispositions in `.planning/notes/console-overhaul-exploration.md`. **Nothing here touches the Go backend** — the new Connect RPCs and the Qdrant Facet API are out of scope for this file (they need zero new npm or Go dependencies either way).

## Recommended Stack

### Core additions (new npm dependencies)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `@humanspeak/svelte-virtual-list` | `0.5.14` | Virtualize the dense results list (50–1000 rows) | The only maintained-as-of-today candidate that is Svelte-5-**native** (runes + snippets, not a ported Svelte-3/4 store adapter), ships dynamic-height support, and documents a **keyboard-accessible viewport with a focusable, labeled region** out of the box — the exact shape `j`/`k` traversal needs to hang off. Published 2026-08-10 (this month), actively maintained. Peer dep `svelte: ^5.0.0`. |
| d3-force + d3-drag + d3-zoom + d3-selection, rendered as plain inline SVG (see below) | `3.0.0` each | Related-memories graph (50–500 nodes), pan/zoom, keyboard access | See "Related-memories graph" analysis below — this is the lightest, most accessible, and most Svelte-5-idiomatic option of the four compared. |

### Zero new dependencies (existing tree already covers it)

| Capability | Existing package | Why it's sufficient |
|---|---|---|
| Tag cloud with counts | `bits-ui` `Badge`/`Button` primitives already vendored via shadcn-svelte, plain `flex-wrap` | See "Tag cloud" analysis below — no library is warranted. |
| Debounced URL-driven search | `@tanstack/svelte-query` `^6.1.34` (already a dependency, already on latest-compatible minor `6.2.4`) | `placeholderData: keepPreviousData` is unchanged in v6; the codebase already uses v6's required `createQuery(() => ({...}))` thunk syntax everywhere (`MigrationBanner.svelte`, all four route files). No new package. |
| Scope autocomplete / server-driven palette | `bits-ui` `^2.18.1` → recommend bump to `2.19.3` (registry latest); `Combobox` + `Command` | Both primitives already ship the async/server-driven pattern natively — see "Combobox vs Command" analysis below. |

## Detailed Analysis

### 1. Dense virtualized results list (50–1000 rows, hover-expand, `j`/`k` traversal)

**Verdict: `@tanstack/svelte-virtual` is not usable today; use `@humanspeak/svelte-virtual-list`.**

| Package | Version | Svelte peer dep | Last publish | Unpacked size | Notes |
|---|---|---|---|---|---|
| `@tanstack/svelte-virtual` | `3.13.39` | `^3.48.0 \|\| ^4.0.0 \|\| ^5.0.0` (claims support) | — | 9.3 KB | **Corrected finding confirmed**: issue [TanStack/virtual#866](https://github.com/TanStack/virtual/issues/866) ("Svelte 5 support"), opened 2024-10-28, is **still open** on 2026-09-25 — nearly two years later. The reported failure is exactly the shape that matters here: the virtualizer can't track the scroll element's initial binding under Svelte 5's reactivity, producing an empty/broken list unless the caller manually forces a `$virtualizer._willUpdate()` call. The peer-dep range claiming `^5.0.0` support is aspirational, not proven — do not build on it. |
| `svelte-virtuallists` | `1.4.2` | `^5.20.5` (Svelte-5-only) | 2025-03-01 | 29 KB | Genuinely Svelte-5-native, supports variable sizing via a `szCalculator(index, item) => number` prediction function, ~5 KB gzipped per its own docs. **Rejected**: last published over a year before this research date (2025-03-01) with no visible activity since — thin maintenance signal for a component the console will depend on long-term. Its variable-height model also requires *predicting* height in advance rather than measuring the rendered DOM, which is the wrong shape for hover-expand (expanded height depends on how much content/how many tags a given record has — that's a measurement, not a formula). |
| `@humanspeak/svelte-virtual-list` | `0.5.14` | `^5.0.0` | 2026-08-10 | 245 KB unpacked (includes full TS source/docs; runtime bundle is a small fraction of this) | **Recommended.** Svelte 5 runes/snippets native, dynamic-height support via measured content (not predicted), automatic resize handling, SSR/hydration compatible, and — the deciding factor — ships a documented keyboard-accessible viewport (focusable labeled region, standard scroll keys) rather than leaving all keyboard behavior to the consumer. Actively maintained (published this month). |
| `svelte-tiny-virtual-list` | `4.0.0` | `^5.0.0` | 2026-08-11 | 48 KB, **zero runtime dependencies** | Worth naming as the fallback: also Svelte-5-native, also actively maintained, supports variable heights/widths and scroll-to-index, and is the smallest/leanest of the three real candidates. If `@humanspeak/svelte-virtual-list`'s dynamic-height remeasurement proves too heavy in profiling, this is the next thing to try — but it does not document built-in keyboard accessibility the way humanspeak's does, so it would need the roving-`tabindex`/`aria-activedescendant` wiring built by hand (which the console needs anyway per the WAI-ARIA Listbox pattern already dispositioned in the exploration note — see next paragraph). |

**Architectural point that matters more than the library choice:** hover-expand should **not** be modeled as the virtualized row growing taller in-flow. That reintroduces exactly the dynamic-height remeasurement complexity these libraries exist to hide, and couples scroll math to hover state. Render each virtualized row at one fixed collapsed height; render the hover/focus-expanded content (first lines of body, full tag set) as an absolutely-positioned overlay anchored to the row's `getBoundingClientRect()`, outside the virtualizer's flow entirely. This is simpler, keeps `@humanspeak/svelte-virtual-list`'s dynamic-height feature unused (fixed-height mode is enough), and — because the detail pane already exists on the right per the exploration note's decision 2 — the hover overlay can be the same lightweight preview the detail pane renders on selection, just triggered earlier.

Keyboard traversal (`j`/`k`) is independent of the virtualizer choice: implement the WAI-ARIA APG Listbox pattern already correctly identified in the exploration note — a single `tabindex="0"` on the scroll container, `aria-activedescendant` pointing at the active row's `id`, and `j`/`k`/arrow keys moving that pointer plus calling the virtualizer's `scrollToIndex`. None of the three libraries above manage `aria-activedescendant` for you; humanspeak's "keyboard-accessible viewport" covers native scroll-key handling on the container, which is a real but smaller piece of the puzzle.

### 2. Related-memories graph (50–500 nodes, dark/light theming, pan/zoom, keyboard access)

**Verdict: d3-force (physics) + d3-drag/d3-zoom (gestures) + plain Svelte-owned inline SVG rendering. Reject Sigma.js, Cytoscape.js, and layercake for this use case.**

| Approach | Gzip size (Bundlephobia, live-checked) | Svelte 5 fit | Accessibility story | Verdict |
|---|---|---|---|---|
| d3-force `3.0.0` + d3-selection `3.0.0` + d3-zoom `3.0.0` + d3-drag `3.0.0` | 5.7 + 4.1 + 15.5 + 5.6 = **~31 KB** total | d3's force/zoom/drag modules are framework-agnostic math/gesture engines with no DOM opinions of their own; the idiomatic Svelte 5 pattern (proven in multiple public Svelte-5 playgrounds, e.g. the "D3 Force Graph - svg" playground and `theWebalyst/d3-fdg-svelte`) is to let the simulation's `tick` callback update a reactive `$state` array of `{x, y}` positions and let Svelte's `{#each}` own the actual `<circle>`/`<line>` elements. d3-selection is only needed to `d3.select()` the zoom/drag target element for `d3-zoom`/`d3-drag`'s `call()` API — it never touches node/edge rendering. | **Best of the four.** Every node is a real SVG element Svelte put there — `<g role="button" tabindex="0" aria-label="...">` is direct, native markup. Keyboard access (Tab/arrow-key node-to-node traversal, Enter to open) is ordinary DOM/ARIA work, no synthetic accessibility layer needed. Dark/light theming is CSS custom properties on real SVG `fill`/`stroke`, inherited automatically from `mode-watcher`'s existing theme class — zero extra work. | **Recommended.** |
| Sigma.js `3.0.3` + graphology `0.26.0` | 26.0 + 12.8 = **~39 KB**, plus a separate layout package (e.g. `graphology-layout-forceatlas2`) needed since Sigma does not compute layout itself — realistic total closer to 45–55 KB | Framework-agnostic, mounts into a container div; no Svelte-specific wrapper exists that's actively maintained, so it'd be a thin custom Svelte component wrapping imperative Sigma calls (`onMount`/`$effect` for lifecycle) — workable but more integration glue than the SVG approach, and fights Svelte's own reactivity rather than using it. | **Worse.** Sigma renders to WebGL/Canvas. There is no per-node DOM element to attach `tabindex`/`role`/`aria-label` to — a11y requires building a synthetic accessible layer (an invisible parallel DOM tree or an ARIA live-region announcer keyed to a virtual cursor), which is real extra work this milestone doesn't need to take on for 50–500 nodes. | Rejected — its whole value proposition (WebGL performance for thousands-to-millions of nodes) doesn't apply at this node count, and its accessibility cost is strictly higher than SVG. |
| Cytoscape.js `3.34.3` | **137 KB** gzip (single bundle) | Same imperative-mount-into-a-div shape as Sigma; no Svelte-native wrapper. | Canvas-based by default — same synthetic-a11y problem as Sigma. A linked GitHub discussion ([cytoscape/cytoscape.js#3125](https://github.com/cytoscape/cytoscape.js/discussions/3125), "How do I support keyboard events to enable Accessibility?") confirms this is a known, unsolved-by-the-library gap that app authors must build themselves. | **Rejected.** Heaviest by far (137 KB gzip is ~4.4x the entire d3-force+drag+zoom+selection combination), and its strength — a full graph-theory toolkit with dozens of layout algorithms and centrality/pathfinding analyses — is overkill for rendering a simple 4-edge-type neighbor graph. This is the wrong tool for the job even before weighing bundle size. |
| layercake `11.0.0` | ~46 KB (unpacked proxy; no direct force-graph primitive) | Genuinely Svelte-idiomatic and Svelte-5-compatible (peer dep `svelte: >=5.40`; v11 rewrote its own examples to runes) — but it is a **headless charting scaffold** (provides scaled coordinate-space context via Svelte context, expects you to bring your own layout math and SVG/Canvas/WebGL layer). It has no force-directed-graph-specific helper; you'd still need d3-force underneath it for the physics. | N/A — it's a layout/scaling context provider, not a rendering or a11y solution by itself. | **Rejected as unnecessary.** It would add a dependency and a new mental model (LayerCake's `Html`/`Svg`/`Canvas` wrapper components, scale context) on top of d3-force for no capability the plain-SVG approach doesn't already have at this data size. LayerCake earns its keep for charts with many linked/composed layers (axes, gridlines, multiple mark types); a single force-directed node/edge graph doesn't need that scaffold. |

**Bottom line:** at 50–500 nodes, the deciding factors are accessibility (native DOM > canvas/WebGL for keyboard/ARIA) and bundle discipline (31 KB vs 39–137 KB), and d3-force's math-only modules compose naturally with Svelte 5's own reactivity rather than fighting it. Use d3-force purely for the physics simulation (`forceSimulation`, `forceLink`, `forceManyBody`, `forceCenter`), d3-drag for per-node drag gestures, d3-zoom for pan/zoom on the SVG viewport, and let Svelte own every rendered element.

### 3. Tag cloud with counts

**Verdict: no library. Flex-wrap of the existing shadcn-svelte `Badge` component with a count-driven size/weight scale.**

A dedicated JS "tag cloud" ecosystem exists (`svelte-d3-cloud`, "OpenTagCloud"'s Svelte-5 component) but every option in it targets the classic *randomized-packing, rotated-text, decorative word cloud* aesthetic — optimized for visual density and novelty, not for an operator console where tags need to be: scannable in reading order, keyboard-focusable, screen-reader-legible (a packed/rotated cloud's DOM order rarely matches its visual order), and clickable to filter. Those are all things a plain `flex-wrap` layout of ordinary interactive `Badge`/`Button` elements does for free, using components already installed. Bucket counts into 3–4 size/weight/opacity tiers (e.g. quantile buckets on `log(count)`, since tag-count distributions are typically power-law) via a Tailwind class map — no runtime layout algorithm needed. This is the same "compose from primitives already installed" instinct the exploration note already applied to the results row and command palette.

### 4. Debounced URL-driven search with TanStack Query 6

**Verdict: `placeholderData: keepPreviousData` is unchanged and still the right idiom in v6; confirm the existing thunk-function calling convention.**

Confirmed directly from TanStack's own docs (via Context7, `/tanstack/query`):
- `keepPreviousData` is a small helper function you pass as `placeholderData: keepPreviousData` on the query options. This shape was introduced in v5 (replacing v4's `keepPreviousData: true` boolean + `isPreviousData` flag) and **nothing in the v5→v6 Svelte migration guide changes it** — the v6 migration guide's only documented breaking change for `@tanstack/svelte-query` is the calling convention: options must be wrapped in a thunk, `createQuery(() => ({ ... }))` instead of `createQuery({ ... })`, because the v6 Svelte adapter dropped its legacy Svelte-3/4 store-compatibility shim and moved fully to runes/signals for reactivity.
- **The engram console already uses this exact v6 syntax everywhere** (`grep` confirms `createQuery(() => (...))` in `MigrationBanner.svelte` and all four route files) — so the only new work for debounced search is adding `placeholderData: keepPreviousData` to the search query's options object and including the debounced search term in the `queryKey` (so each keystroke's settled value gets its own cache entry, and the *previous* key's data stays visible while the *new* key's `queryFn` is in flight, avoiding the flash-to-loading-skeleton on every keystroke). No new package — `keepPreviousData` is exported from `@tanstack/svelte-query` (re-exported from `@tanstack/query-core`) at the already-pinned `^6.1.34`.
- The idiomatic Svelte 5 runes pattern for the debounce itself: a `$state` for the raw input value, a `$derived` (or a small `$effect` with `setTimeout`/`clearTimeout`) producing the debounced term, and that debounced term — not the raw input — feeding both the URL (`goto`, `replaceState: true`, `keepFocus: true`) and the `queryKey`. This keeps the URL and the query key in lockstep, which is what makes the search shareable/bookmarkable per the exploration note's "the entry point must not lie" principle.

### 5. Combobox vs Command for scope autocomplete and server-driven palette (bits-ui 2.19.3)

**Verdict: `Command.Root shouldFilter={false}` for the palette (already dispositioned correctly); plain `Combobox` for scope autocomplete — it needs no `shouldFilter` prop at all.**

Confirmed directly from bits-ui's own docs (via Context7, `/huntabyte/bits-ui`):
- **`Command`** ships an opinionated default: it scores and filters items client-side against the typed query using an internal `computeCommandScore` algorithm, UNLESS you set `Command.Root shouldFilter={false}`, at which point — per bits-ui's own docs — "disabling filtering is particularly useful when handling custom logic, fetching items asynchronously, or managing the filtering process manually." This is exactly what the exploration note's bug-2 fix already prescribes for the palette (drive the list from `SearchMemories` results, never let `Command`'s own scorer see and reject the query text against static item labels).
- **`Combobox`** has no such built-in scorer to disable in the first place. bits-ui's own reference implementation of a "reusable custom Combobox" shows the consumer owning `items` as a plain reactive array and applying (or not applying) any filtering itself before passing it to `Combobox.Root items={...}`. This means server-driven/async scope autocomplete is the *default* shape for `Combobox`, not an opt-out — bind the debounced input to a query, pass the query's `data` straight through as `items`, and there is nothing to disable. Use `Combobox` (not `Command`) for the scope-autocomplete field precisely because it is the primitive built for "an input plus a list of options that may come from anywhere," while `Command` is built for "a static, locally-scored list you might want to bypass" — the palette needs the bypass; the scope field never had the client-side scorer to begin with.

### 6. Upgrading bits-ui 2.18.1 → 2.19.3

**Verdict: safe, recommended, no breaking changes for anything the console currently uses.**

Checked the GitHub Releases changelog for every version in the range:
- **2.19.0** (Aug 20): adds a new `Select.Value` component (additive) plus minor form-integration/focus improvements.
- **2.19.1** (Sep 8): `Combobox` now highlights the first matching item after custom filtering updates (fixes a rough edge relevant to the scope-autocomplete work above); `Select` scroll-position retention improved; `Dialog`/`AlertDialog` outside-click dismissal timing improved.
- **2.19.2** (Sep 9): fixes missing `id` attributes on `Select`/`Combobox`/`DropdownMenu` content elements so `aria-describedby` on triggers resolves correctly — a genuine accessibility fix, directly relevant since the console already uses `DropdownMenu` and will newly use `Combobox`; `Dialog`/`AlertDialog` body-style restoration fixed; `Tabs` now preserves explicit `tabindex` values on `Content` elements (the console uses `Tabs`).
- **2.19.3** (Sep 22, registry latest as of this research): fixes `Combobox` opening on touch-tap-down while scrolling (mobile-only issue); `Select` scroll-position fix at list boundaries; `Dialog`/`AlertDialog` attribute-rendering fix.

None of these are breaking changes to `Command`, `Sidebar`, `Sheet`, `Pagination`, `Kbd`, `DropdownMenu`, `Dialog`, or `Tabs` — the components the console already relies on. The 2.19.x line is a strict superset of fixes on top of 2.18.x's public API, several of which are accessibility improvements that directly benefit this milestone's new `Combobox` usage. Bump `bits-ui` from `^2.18.1` to `^2.19.3` (or leave the existing `^2.18.1` semver range as-is, which already resolves to `2.19.3` on a fresh install — but pin the `package.json` floor to `^2.19.3` explicitly so CI and local installs stay in sync with the version this research verified).

## Installation

```bash
# Core additions
pnpm add @humanspeak/svelte-virtual-list@0.5.14
pnpm add d3-force@3.0.0 d3-selection@3.0.0 d3-zoom@3.0.0 d3-drag@3.0.0

# TypeScript types for the d3 micro-packages (dev-only, no runtime cost)
pnpm add -D @types/d3-force@3 @types/d3-selection@3 @types/d3-zoom@3 @types/d3-drag@3

# Bump existing dependency (no code change required beyond the semver bump)
pnpm add bits-ui@2.19.3

# Nothing to install for: tag cloud (existing Badge), debounced search
# (existing @tanstack/svelte-query, existing bits-ui Combobox/Command)
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| `@humanspeak/svelte-virtual-list` for the results list | `svelte-tiny-virtual-list` | If profiling shows `@humanspeak/svelte-virtual-list`'s dynamic-height remeasurement is a real cost (unlikely, since the fixed-height-row + overlay-expand architecture above avoids exercising that feature) — it's zero-dependency, actively maintained, and equally Svelte-5-native, just without the built-in keyboard-accessible-viewport wiring. |
| `@humanspeak/svelte-virtual-list` for the results list | A plain hand-rolled windowed `{#each}` (no dependency) | If the team's dependency-discipline preference extends from the Go backend's "zero new Go deps" standing constraint to the `ui/` tree as a hard rule (not stated explicitly for `ui/` in CLAUDE.md) — 50–1000 rows is small enough that a ~60-line hand-rolled windowed list bound to `$state` scroll position, a fixed row height constant, and an overscan buffer is genuinely tractable, and it avoids taking on a third-party runtime dependency for a console that already carries a fairly deliberate, curated dependency list. Recommend the library first because it removes scroll-math edge cases (resize, overscan tuning, scroll-into-view on keyboard nav) that are easy to get subtly wrong by hand, but this is a legitimate team call either way. |
| d3-force + inline SVG for the related-memories graph | Sigma.js + graphology | If a future milestone needs to render graphs at a materially larger scale (thousands of nodes) where WebGL rendering genuinely matters — not the case at 50–500 nodes. |
| Plain `Badge` flex-wrap for the tag cloud | `svelte-d3-cloud` / OpenTagCloud's Svelte-5 component | If a future "browse/newcomer" surface (audience (c) in the build order) wants a genuinely decorative, exploratory word-cloud aesthetic rather than a scannable operator control — worth revisiting only if the product goal shifts from "filter by tag" to "get a feel for the corpus." |
| `Combobox` for scope autocomplete | `Command` with `shouldFilter={false}` | If the scope field needs to live inside the same command-palette surface as the free-text/id search (e.g. a unified `⌘K` palette with a scope sub-mode) rather than as its own standalone field — `Command` groups/sections may fit that composition better than a separate `Combobox`. Not the case for a dedicated scope-filter control. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|--------------|
| `@tanstack/svelte-virtual` | Its Svelte 5 support issue ([TanStack/virtual#866](https://github.com/TanStack/virtual/issues/866)) has been open since 2024-10-28 and is still open on 2026-09-25 — the peer-dep range claiming `svelte: ^5.0.0` support does not reflect a proven-working integration; the reported failure mode (broken scroll-element binding under Svelte 5 reactivity, requiring a manual `_willUpdate()` workaround) is exactly the kind of subtle bug this milestone can't afford to debug from scratch in a results list that's the front door of the whole console. | `@humanspeak/svelte-virtual-list` |
| Cytoscape.js for the related-memories graph | 137 KB gzip for a single bundle — by far the heaviest option evaluated, and its core value (a full graph-theory analysis toolkit: centrality, pathfinding, dozens of layout algorithms) solves a problem this milestone doesn't have. Canvas-based rendering also means building a synthetic accessibility layer from scratch (confirmed as an open, library-unsolved gap by the maintainers themselves). | d3-force + inline SVG |
| Sigma.js + graphology for the related-memories graph | WebGL rendering exists to make thousands-to-millions of nodes fast; at 50–500 nodes that performance headroom is unused, while its accessibility cost (no per-node DOM to attach ARIA/keyboard handling to) is paid regardless of graph size. | d3-force + inline SVG |
| layercake for the related-memories graph | It's a headless scaling/context scaffold for composed multi-layer charts, not a force-directed-graph primitive — you'd still need d3-force underneath it, so adopting it here adds a dependency and a new mental model for no capability gain over plain d3-force + Svelte. | d3-force + inline SVG |
| Any dedicated "tag cloud" library (`svelte-d3-cloud`, OpenTagCloud, etc.) | Built for decorative, randomly-packed, rotated-text word clouds — the wrong shape for a scannable, keyboard-accessible, filterable operator control; reading-order and DOM-order diverge from visual order in a packed layout, which is an accessibility regression, not a neutral stylistic choice. | Plain `flex-wrap` of the existing shadcn-svelte `Badge` component, sized by a count-quantile bucket |
| Rebuilding hover-expand as a virtualizer-managed dynamic row height | Reintroduces the exact remeasurement/reflow complexity these virtualization libraries exist to abstract away, and couples scroll math to transient hover state — a correctness and performance risk for no benefit, since the console already has a right-hand detail pane that can show the same expanded content. | Fixed-height virtualized rows + an absolutely-positioned hover/focus overlay anchored to the hovered row |

## Stack Patterns by Variant

**If the team wants zero new runtime npm dependencies for the results list (matching the Go backend's "zero new deps" ethos as a project-wide norm, not just a Go rule):**
- Hand-roll a windowed `{#each}` over `$state` scroll position + a fixed row-height constant + an overscan buffer (~20 extra rows above/below viewport).
- Because at 1000 rows max, the complexity ceiling of a hand-rolled windowed list is genuinely low, and it removes one third-party dependency from a console the team has otherwise kept deliberately lean (16 domain components, 24 shadcn primitives, no UI framework beyond what's already pinned).

**If a future milestone needs the related-memories graph to scale past low thousands of nodes:**
- Re-evaluate Sigma.js + graphology at that point — its WebGL rendering and graphology's built-in centrality/community-detection algorithms become genuinely worth their weight once node counts outgrow what SVG can render smoothly (roughly low thousands of simultaneously-visible DOM nodes).
- Because the accessibility and bundle-size tradeoffs that reject Sigma.js today are specifically a function of *this* milestone's stated 50–500 node scope, not a permanent verdict on the library.

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|------------------|-------|
| `@humanspeak/svelte-virtual-list@0.5.14` | `svelte@5.57.1` (pinned) | Peer dep is `svelte: ^5.0.0`; no upper-bound conflict. Published 2026-08-10, after the console's current `svelte@5.57.1` pin, so it was built and tested against a Svelte 5 at least as new as the console's. |
| `d3-force@3.0.0` / `d3-selection@3.0.0` / `d3-zoom@3.0.0` / `d3-drag@3.0.0` | Any framework, any Svelte version | These are pure-JS/DOM-standard modules with no framework peer dependency at all — the "Svelte 5 fit" question doesn't apply to them the way it does to Svelte-specific component libraries; they're compatible by construction. All four have been at major version `3.0.0` since 2022 with no subsequent major bump — this is a stable, low-churn layer of the d3 ecosystem, not an abandoned one. |
| `bits-ui@2.19.3` | `svelte@5.57.1` (pinned) | Peer dep is `svelte: ^5.33.0`, satisfied by the console's `5.57.1`. Also declares a peer on `@internationalized/date: ^3.8.1`, satisfied by the console's already-pinned `^3.12.2`. |
| `@tanstack/svelte-query@^6.1.34` (existing) | `svelte@5.57.1` (pinned) | Peer dep is `svelte: ^5.25.0`, satisfied. Registry latest is `6.2.4` — a minor bump within the existing `^6.1.34` semver range; no `package.json` change required, but running `pnpm update @tanstack/svelte-query` picks it up. |
| `bits-ui@2.19.3` Combobox/Command | `@humanspeak/svelte-virtual-list@0.5.14` | No interaction — the palette/combobox and the results-list virtualizer are independent surfaces (palette results are typically ≤20 items and don't need virtualization; the virtualized list is the full search-results view). No shared DOM/focus-management conflict expected, but worth a UAT check once both land: a combobox `Portal`-rendered dropdown floating over a virtualized list should not fight the list's own `aria-activedescendant` roving focus. |

## Sources

- `npm view <pkg> version peerDependencies` (live registry queries, 2026-09-25) — `@tanstack/svelte-virtual@3.13.39`, `svelte-virtuallists@1.4.2`, `@humanspeak/svelte-virtual-list@0.5.14`, `svelte-tiny-virtual-list@4.0.0`, `bits-ui@2.19.3`, `@tanstack/svelte-query@6.2.4`, `d3-force/d3-selection/d3-zoom/d3-drag@3.0.0`, `sigma@3.0.3`, `graphology@0.26.0`, `cytoscape@3.34.3`, `layercake@11.0.0` — HIGH confidence, primary source.
- [TanStack/virtual#866](https://github.com/TanStack/virtual/issues/866) — "Svelte 5 support," confirmed open as of this research date — HIGH confidence.
- Context7 `/huntabyte/bits-ui` (2065–1001 snippets, High source reputation) — `Command.Root shouldFilter={false}`, `computeCommandScore`, Combobox's unopinionated `items` model with no built-in filter — HIGH confidence, quoted directly from bits-ui's own docs source.
- Context7 `/tanstack/query` — `createQuery(() => ({...}))` thunk requirement in the Svelte v5→v6 migration guide, `keepPreviousData`/`placeholderData` semantics unchanged since the v4→v5 merge — HIGH confidence, quoted directly from TanStack's own docs source.
- GitHub Releases, `huntabyte/bits-ui` (2.19.0 through 2.19.3 changelog entries) — HIGH confidence, fetched directly.
- Bundlephobia live size API (`bundlephobia.com/api/size?package=...`) for `cytoscape`, `sigma`, `graphology`, `d3-force`, `d3-selection`, `d3-zoom`, `d3-drag` — HIGH confidence, live-queried minified+gzip figures for the pinned versions.
- [cytoscape/cytoscape.js#3125](https://github.com/cytoscape/cytoscape.js/discussions/3125) — confirms Cytoscape.js keyboard accessibility is an unsolved, app-author responsibility — MEDIUM confidence (community discussion, not official docs, but consistent with Canvas rendering's known DOM limitations).
- Live `grep`/`rg` of `/Volumes/Code/github.com/seanb4t/engram/ui/src` — confirmed every current `createQuery` call site already uses the v6 thunk syntax, confirmed no existing `keepPreviousData` usage — HIGH confidence, read directly from the working tree.
- `/Volumes/Code/github.com/seanb4t/engram/ui/package.json` — current pinned versions (`svelte@5.57.1`, `bits-ui@^2.18.1`, `@tanstack/svelte-query@^6.1.34`, etc.) — HIGH confidence, read directly.

---
*Stack research for: engram milestone 2026-09-25.01 "Console Overhaul" — frontend/UI additions only*
*Researched: 2026-09-25*
