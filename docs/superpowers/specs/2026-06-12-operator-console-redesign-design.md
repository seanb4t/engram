<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# engram operator console — holistic shadcn-forward redesign

**Bead:** `engram-kco.2` (epic `engram-kco`) · **Date:** 2026-06-12 · **Status:** design

## Context

The v1 operator console (`/ui`, SvelteKit SPA shipped in #79) is functional but
spartan, and now that it is deployed against real data
(`engram.fzymgc.house/ui/observe`) the gaps are obvious: a 13px-monospace wall
with weak hierarchy, plus outright rendering bugs. The current UI is a thin
hand-rolled layer — custom `eg-*` utility classes, raw inline styles, a custom
reduced `--cat-*` token set — that barely uses the shadcn-svelte stack already
installed in the project. `components.json` is CLI-initialized (default style,
neutral base, `cssVariables: true`); bits-ui, tailwind-variants, `cn()`
(clsx + tailwind-merge), mode-watcher, and Tailwind v4 are all present, but only
`button`, `checkbox`, and `select` of ~50 registry primitives are vendored.

This redesign rethinks the entire presentation against the real data, leaning
into shadcn-svelte components and a modern, all-sans aesthetic, and migrates off
the custom token/utility layer onto shadcn's standard semantic theme. It is a
single coherent pass rather than the piecemeal fixes originally filed
(`engram-kco.1/.3/.4/.5/.6/.7`), which are folded in here as the requirements
inventory and will be re-materialized as implementation beads by `plan-to-beads`.

## Goals

- Replace the broken/spartan presentation with a modern, scannable, responsive
  console built from shadcn-svelte components.
- Migrate the theme to shadcn's standard semantic tokens (light + dark), keeping
  the four category hues as accents; retire `eg-*`, raw inline styles, and the
  custom `--cat-*` layer.
- Preserve the existing data architecture: svelte-query v6 (runes) + URL-as-state
  + the Connect API (`listScopes` / `listMemories` / `getMemory`), and the
  server's existing offset pagination.
- No backend/proto changes.

## Non-goals (deferred to later epics)

- Create / edit / delete memory write actions (separate write-phase epic).
- A net-new Dashboard view + LayerChart analytics.
- The sortable Data-Table "all memories" power view (option C from row
  exploration) — kept as a future browsing mode.

## Requirements inventory (the findings this design resolves)

Grounded in `ui/src/lib/components/{AppShell,ScopeRail,MemoryList,MemoryDetail,SearchPalette}.svelte`,
`ui/src/app.css`, and `ui/src/routes/{observe,search,discovery}/+page.svelte`:

1. **Scope-rail overflow/overlap** — `ScopeRail.svelte:41` `<span class="flex-1">{s.scope}</span>`
   has no `min-w-0 truncate`, so long scopes overflow the 210px rail and collide
   with the count. The shared `Button` base (`button.svelte:6`,
   `inline-flex items-center whitespace-nowrap`) leaks into the multi-line rows.
2. **List-row redundancy & truncation** (`MemoryList.svelte:24-26`) — `{category}`
   then `{content.slice(0,80)}` yields `gotcha · GOTCHA (…)`; the slice hard-cuts
   mid-word with no ellipsis; a dangling `· {visibility}` trails every row.
3. **No recency** — `created_at` shows only in the detail pane; the list can't be
   scanned by age.
4. **No visual hierarchy** — 13px mono, rows divided only by a 1px border; no
   spacing, chips, hover, or empty/loading affordances.
5. **Detail pane cramped & flat** (`MemoryDetail.svelte:11`) — fixed 300px, raw
   unwrapped content, date-only (`YYYY-MM-DD`), provenance crammed at the bottom.
6. **Non-responsive layout** — hard-coded pane widths (rail 210px, detail 300px).
7. **No navigation chrome** — header has only a ⌘K→/search button + theme toggle;
   `/search` is a raw `<input>` page, not a command palette.
8. **Non-actionable states** — "retry" is plain text; "no memories" is bare text.

## Design decisions (validated via the visual companion)

### Shell & navigation — Option A

A four-region resizable shell:

- **Nav rail** (far left, slim, icon + label): Observe / Search / Discovery, with
  room for future views. shadcn **Sidebar** (icon/collapsible mode).
- **Scopes sidebar** (collapsible): the scope list (see ScopeChip) + the
  category/visibility filters. shadcn **Sidebar** content + **Field** for filters.
- **List** and **Detail** panes: shadcn **Resizable** PaneGroup, each pane a
  **ScrollArea**, with sensible min/max widths; a narrow-viewport strategy
  collapses the detail into a **Sheet** overlay.
- **Top bar**: brand, ⌘K command trigger (**Kbd** affordance), theme toggle,
  future user menu (**DropdownMenu** + **Avatar**).

### Memory-list row — Option A (compact), all-sans

`Item`-based row: a category **Badge** chip + a single-line summary (real
`truncate`, with the redundant leading ALLCAPS category token stripped since the
badge carries it) + a muted meta line: relative time · first N tags · `+overflow`.
Clear hover + selected affordance (`data-` state, semantic tokens). Cross-scope
**Search** rows additionally carry a ScopeChip; **Observe** rows omit it (the rail
already fixes the scope).

### Typography & theme — all-sans + semantic tokens

All-sans (system/Inter) for a clean product feel, replacing all-monospace.
Monospace is reserved only for genuinely code-like data where it aids reading
(scope strings, the detail body may preserve whitespace). Adopt shadcn's standard
semantic token set (`background`/`foreground`/`card`/`muted`/`border`/`primary`/…)
in `app.css` for light + dark, retiring `--background/--surface/--foreground/--muted/--accent`
and the bespoke `eg-*` utilities. Keep the four category hues
(`convention`/`gotcha`/`decision`/`preference`) as Badge/ScopeChip accents,
expressed as theme variables.

### Detail pane — Option B

shadcn **Card** + **ScrollArea**: header (category Badge + relative time + full
timestamp on hover + a copy action) → a metadata block where the **full scope**
is shown verbatim on its own line (mono, ellipsis + Tooltip on overflow) and
`by` / `source` / `visibility` are compact pills below → content body (ScrollArea,
whitespace-preserved) → tag chips. Copy-to-clipboard (content / id) ships now.

### ScopeChip — one component, two render modes

Scopes are `type:body` (`repo:github.com/org/name`, `discovery:repo:…`,
`project:name`). One `ScopeChip` component:

- **Type marker**: a small color-coded text **Badge** (`repo` / `disc` / `proj` /
  …) — disambiguates a `repo` and a `project` that share a name.
- **Name**: split `org/name` on the last `/`; the **repo name is prominent**, the
  **org de-emphasized**. Repos drop the `github.com/` host; projects have no org.
- **Render modes**: **stacked** in the rail (repo name on a full-width line, org
  a tiny dim line below); **inline** elsewhere (search rows, detail — single line,
  org dim + smaller prefix, `truncate`).
- **Full fidelity**: the complete scope string is never destroyed — a **HoverCard**
  reveals type + full scope + record count + org.

Rejected: a color-dot type marker (ambiguous); org middle-compression (`fz…se`)
(unrecognizable across `fzymgc-house`/`fovea`/`holomush`).

### v1 extras (in scope)

- **⌘K Command palette** — **Command** inside a **Dialog** overlay: search
  memories + jump to scopes/views, replacing the plain `/search` input page.
- **States** — **Empty** (no results), **Skeleton** (loading), and **Sonner**
  toasts (the `sonner` registry component, which wraps `svelte-sonner`) for
  errors / copy confirmations across all views.
- **Pagination** — shadcn **Pagination** over the server's existing offset paging
  (`listMemories` `limit`/`offset`/`total`).

## Component architecture

New/added shadcn primitives (via `npx shadcn-svelte@latest add`): `sidebar`,
`resizable`, `scroll-area`, `separator`, `badge`, `tooltip`, `hover-card`, `card`,
`command`, `dialog`, `sheet`, `dropdown-menu`, `avatar`, `empty`, `skeleton`,
`sonner`, `pagination`, `kbd`, `item`, `input`, `tabs` (as needed). Add an
`iconLibrary` (`@lucide/svelte`) to `components.json` — there is none today;
icons in buttons use `data-icon`, no sizing classes.

App components (rewritten/added under `ui/src/lib/components/`):

| Component | Responsibility | Built from |
|---|---|---|
| `AppShell` | nav rail + top bar + resizable content frame, theme | Sidebar, Resizable, Kbd, DropdownMenu |
| `ScopesSidebar` (was `ScopeRail`) | scope list + filters | Sidebar, ScopeChip, Field, Select |
| `ScopeChip` | typed, footprint-reduced scope rendering | Badge, HoverCard, Tooltip |
| `MemoryList` | compact rows + pagination + states | Item, Badge, Pagination, Empty, Skeleton |
| `MemoryRow` | one compact row | Item, Badge, ScopeChip (search only) |
| `MemoryDetail` | Card detail (Option B) + copy | Card, ScrollArea, Tooltip, Sonner |
| `CommandPalette` | ⌘K search/nav | Command, Dialog, Kbd |
| `lib/scope.ts` | parse `type:body` → {type,host,org,name}; short/long forms | — |
| `lib/time.ts` | relative-time + full-timestamp formatting | — |

Each component keeps one clear purpose and gets a focused component test
(`*.test.ts`, vitest + @testing-library/svelte, matching the existing pattern in
`MemoryList.test.ts` / `AppShell.test.ts` / `SearchPalette.test.ts`).

## Data flow (unchanged)

Preserve the svelte-query v6 runes pattern and URL-as-state from
`observe/+page.svelte`: `parseObserveParams` / `observeSearch` drive
`scope/categories/visibility/offset/selectedId`; `createQuery(() => …)` options
functions re-run via runes; results read directly off the query object. The
ScopeChip and relative-time helpers are pure formatters over the existing
`ScopeCount` / `Memory` proto types. Pagination maps the shadcn component onto the
existing `offset`/`limit`/`total`.

## Theme migration

`app.css` moves from the bespoke palette to shadcn's standard semantic variables
(light `:root` + `.dark`), generated/aligned via the shadcn theming docs. The four
category accents become theme variables (e.g. `--cat-convention` retained but
expressed in the shadcn token system / referenced by Badge variants). All `eg-*`
utilities and raw inline `style="…var(--…)"` usages are removed in favor of
semantic Tailwind utilities and component variants. This is a mechanical but
broad migration touching every component — done as part of each component's
rewrite, not as a separate big-bang pass.

## Testing & verification

- Per-component vitest tests (rendering, truncation/no-overflow, state branches,
  scope parsing edge cases: `repo:`/`discovery:repo:`/`project:`, missing org,
  long names).
- `task ui:build` produces the vendored `internal/webauth/static/` bundle; the
  `ui vendored-asset drift` CI job verifies it. The Go `//go:embed all:static`
  contract (engram-0y0) already covers `_app/*`.
- Manual verification against the deployed console with real data (the original
  trigger): rail no longer overflows, rows scan cleanly with recency, detail
  preserves full scope, responsive panes, ⌘K palette works.

## Risks & mitigations

- **Scope of change is large** (every component + tokens) → sequence as
  foundation first (token migration + shell), then sidebar/scope-chip, then
  list/row, then detail, then palette/states; each independently testable and
  shippable behind the same shell.
- **svelte-query v6 + bits-ui 2.x API drift** → follow the per-component
  shadcn-svelte docs (`shadcn-svelte.com/docs/components/<name>.md`) and the
  existing runes patterns already in the codebase; do not regress the URL-as-state
  contract.
- **Vendored-asset drift / embed** → already guarded by CI + the `all:static`
  embed fix; rebuild via `task ui:build`.

## Out of scope (restated)

Write actions (create/edit/delete), Dashboard + charts, the Data-Table power
view, and any backend/proto/auth changes.
