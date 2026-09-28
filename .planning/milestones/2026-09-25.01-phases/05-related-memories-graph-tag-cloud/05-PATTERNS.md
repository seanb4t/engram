# Phase 5: Related-Memories Graph & Tag Cloud - Pattern Map

**Mapped:** 2026-09-27
**Files analyzed:** 12 new + 2 modified (route + components + pure-model modules + tests)
**Analogs found:** 12 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `ui/src/routes/related/[id]/+page.svelte` | route | request-response | `ui/src/routes/scheduled/+page.svelte` | role-match (path param vs query param, same client-router/query wiring) |
| `ui/src/routes/related/[id]/related.browser.test.ts` | test | request-response | `ui/src/routes/scheduled/scheduled.browser.test.ts` | exact (same `$app/state` mock shape) |
| `ui/src/lib/components/RelatedGraph.svelte` | component | event-driven (d3-force/drag/zoom + roving keyboard) | `ui/src/lib/components/ResultsList.svelte` (keyboard/ARIA) + new physics wiring (no in-repo analog) | role-match for keyboard; no analog for d3 physics (net-new) |
| `ui/src/lib/components/RelatedGraph.browser.test.ts` | test | event-driven | `ui/src/lib/components/ResultsList.browser.test.ts` | exact (roving-focus assertion shape) |
| `ui/src/lib/components/RelatedLanes.svelte` (or `EdgeLane.svelte` + `SupersessionLane.svelte`) | component | CRUD (read-only render of `RelatedMemoriesResponse`) | `ui/src/lib/components/ChainDialog.svelte` (supersession timeline) + `ui/src/lib/components/FacetStrip.svelte` (chip/row list rendering) | role-match |
| `ui/src/lib/components/EvidenceSection.svelte` | component | request-response (derived, no network call) | `ui/src/lib/components/ChainDialog.svelte`'s `chain-peek` block | role-match |
| `ui/src/lib/components/TagBars.svelte` + `.browser.test.ts` | component | CRUD (read `ListTags`, cached) | `ui/src/lib/components/FacetStrip.svelte` (chip rendering, one shared query) | role-match |
| `ui/src/lib/components/TagCombobox.svelte` + `.browser.test.ts` | component | request-response (client-side filter over cached response) | `ui/src/lib/components/ScopeCombobox.svelte` | exact |
| `ui/src/lib/related/graph.ts` + `.test.ts` | utility | transform (pure model builder) | `ui/src/lib/curation/chain.ts` | exact |
| `ui/src/lib/tags/tags.ts` + `.test.ts` | utility | transform (pure ranking/slicing) | `ui/src/lib/curation/chain.ts` (pure-function/test split pattern only, not domain) | role-match |
| `ui/src/lib/components/DetailPane.svelte` (modified: add "Related" button) | component | request-response | itself, existing actions block | exact (self-modification) |
| `ui/src/lib/components/RowActions.svelte` / `ResultsList.svelte` (modified: add `r` key) | component | event-driven (keyboard) | `ResultsList.svelte`'s existing `handleListboxKey` switch | exact |
| `ui/src/lib/components/CommandMenu.svelte` (modified: add "Related to {short_id}" item) | component | request-response | itself, existing item list | exact |
| `.claude/skills/engram-console-conventions/SKILL.md` (modified: keyboard table) | config/docs | n/a | itself | exact |

## Pattern Assignments

### `ui/src/routes/related/[id]/+page.svelte` (route, request-response)

**Analog:** `ui/src/routes/scheduled/+page.svelte`

**Imports pattern** (lines 1-30):
```svelte
import { page } from '$app/state';
import { goto } from '$app/navigation';
import { base } from '$app/paths';
import { createQuery, keepPreviousData } from '@tanstack/svelte-query';
import { engram } from '$lib/client';
import { parseConnectError, fixRowsFor } from '$lib/errors/connect-error';
```
Use `page.params.id` (typed route param from `[id]/+page.svelte`) instead of `page.url.searchParams` — this is the first path-segment param in the codebase (per RESEARCH.md Pattern 4), but the reactive-derivation idiom is identical:
```typescript
import { page } from '$app/state';
const id = $derived(page.params.id);
```

**Core query pattern** (mirrors `scheduled/+page.svelte:37-56`, adapted to `relatedMemories`):
```typescript
const relatedQuery = createQuery(() => ({
  queryKey: ['relatedMemories', id, 8, false],
  queryFn: ({ signal }) => engram.relatedMemories({ id, k: 8n, full: false }, { signal }),
  meta: { silent: true } // route renders its own error/not-found UI
}));
```
Query-key prefix `'relatedMemories'` already matches `ChainDialog.svelte:34` and the curation-invalidation list (`engram-connect-client` SKILL, "seven query-key prefixes") — no new invalidation wiring needed.

**Error handling pattern:** reuse `parseConnectError`/`fixRowsFor` exactly as `scheduled/+page.svelte` does for its own `listQ` (not shown above but present at file's error-render block) — the not-found case renders the shared copy string `No memory with id {id} that you can read` (same string `ChainDialog`'s peek uses, per D-01 canonical copy).

**Route-state pattern (trail/re-centre, D-01/D-04):** `navigate()` in `scheduled/+page.svelte:33-35` is the direct template for re-centre:
```typescript
function navigate(next: Partial<ScheduledParams>) {
  goto(`${base}/scheduled?${encodeScheduledParams({ ...params, ...next })}`, { keepFocus: true, noScroll: true });
}
```
becomes, for `/related`:
```typescript
function recenter(newId: string) {
  goto(`${base}/related/${newId}`); // pushes a history entry (D-04); no query-string codec needed
}
```

---

### `ui/src/routes/related/[id]/related.browser.test.ts` (test)

**Analog:** `ui/src/routes/scheduled/scheduled.browser.test.ts`

**Mock pattern** (line 47):
```typescript
vi.mock('$app/state', () => ({ page: pageState }));
```
Extend `pageState` with a `params: { id: '...' }` field alongside the existing `url` field (RESEARCH.md Pattern 4's exact guidance) — do not invent a second mocking mechanism.

---

### `ui/src/lib/components/RelatedGraph.svelte` (component, event-driven)

**No direct analog for d3 physics** — this is net-new per RESEARCH.md ("Don't Hand-Roll" table). For the **keyboard/ARIA half only**, copy `ResultsList.svelte`'s roving-focus pattern verbatim in shape:

**Analog:** `ui/src/lib/components/ResultsList.svelte`

**aria-activedescendant sync pattern** (lines 335-346):
```typescript
$effect(() => {
  const id = activeId;
  if (!viewportEl) return;
  if (id) {
    viewportEl.setAttribute('aria-activedescendant', `opt-${id}`);
  } else {
    viewportEl.removeAttribute('aria-activedescendant');
  }
});
```

**Keyboard switch pattern** (lines 504-525) — substitute lane-order comparator (D-10) for list-index order, Space/click for Enter-opens:
```typescript
function handleListboxKey(key: string) {
  const current = activeIndex;
  switch (key) {
    case 'ArrowDown': void moveActive(current + 1); break; // lane-order successor
    case 'ArrowUp': void moveActive(current - 1); break;
    case 'Home': void moveActive(0); break;
    case 'End': void moveActive(memories.length - 1); break;
    case 'Enter': { /* re-centre, not open */ break; }
  }
}
```
`role="option"` on each node's `<g>`, container `role="listbox" tabindex="0"` — same shape as `ResultsList.svelte:772` (`role="option"`) and its listbox container comment at lines 766-767 ("the container (role=\"listbox\") owns all keyboard interaction via aria-activedescendant").

**d3 wiring (net-new, from RESEARCH.md Patterns 1-2 — cite these verbatim in the plan, no in-repo analog exists):**
```typescript
import { forceSimulation, forceLink, forceManyBody, forceCenter } from 'd3-force';
import { zoom as d3zoom } from 'd3-zoom';
import { select } from 'd3-selection';
// 300-tick synchronous settle (D-09), zoom .filter() gating wheel to ctrlKey||metaKey (D-20)
```

---

### `ui/src/lib/components/RelatedGraph.browser.test.ts` (test)

**Analog:** `ui/src/lib/components/ResultsList.browser.test.ts`

**fireKey/userEvent pattern** (lines 36, 59-84):
```typescript
function fireKey(el: Element, key: string, opts: Partial<KeyboardEventInit> = {}) { /* ... */ }
// ...
await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');
await userEvent.keyboard('{End}');
await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0999');
```
Substitute `'opt-<shortId>'` ids and lane-order expected sequence. Per Phase 4 D-18 / RESEARCH.md Pitfall 3: assert `[role="option"]` count, `aria-activedescendant`, and accessible names — never d3-computed `(x,y)` coordinates.

---

### `ui/src/lib/components/RelatedLanes.svelte` / `EdgeLane.svelte` / `SupersessionLane.svelte` (component, CRUD read-render)

**Analog (supersession lane):** `ui/src/lib/components/ChainDialog.svelte`

**Timeline column pattern** (lines 104-107, 131-167):
```svelte
<div class="chain-cols" role="list" aria-label="Supersession chain">
  {#each chain.columns as col (col[0]?.depth ?? 0)}
    <div class="ccol" role="listitem">
      <span class="ccol-h">{columnHeader(col)}</span>
      {#each col as node (node.id)}
        {#if node.placeholder}
          <div class="cnode placeholder">...</div>
        {:else}
          <button type="button" class="cnode" class:hl={node.id === anchorId} onclick={() => togglePeek(node.id)}>
            <span class="csum">{node.summary}</span>
            <span class="mono">{node.shortId}</span>
          </button>
        {/if}
      {/each}
    </div>
  {/each}
</div>
```
Reuse `$lib/curation/chain.ts`'s `headIdFrom`/`buildChain` directly (RESEARCH.md/CONTEXT.md explicit instruction) — do not re-derive the chain model; only relabel columns per the Deltas table (`−2/−1/anchor/+1`, not `d2/d1/head·d0`).

**Analog (citation/tag/vector one-line-row lanes):** `ui/src/lib/components/FacetStrip.svelte`'s chip-row rendering (lines 98-152) for the `{#each ... as x (x)}` + row-grid shape, and `ResultsList.svelte`'s `role="option"` row convention for the shared-selection lit-row behavior.

**Query reuse:** the same `['relatedMemories', id, k, full]` key `ChainDialog.svelte:34` and RESEARCH.md's architecture diagram both use — one fetch backs lanes, graph, and evidence.

---

### `ui/src/lib/components/EvidenceSection.svelte` (component, request-response/derived)

**Analog:** `ui/src/lib/components/ChainDialog.svelte`'s `chain-peek` block (lines 145-162)

```svelte
{#if peekId === node.id}
  <div class="chain-peek">
    {#if peekQuery.isLoading}
      <span>loading…</span>
    {:else if peekNotFound}
      <span>No memory with id {node.id} that you can read</span>
    {:else if peekQuery.data?.memory}
      ...
    {/if}
  </div>
{/if}
```
D-05's evidence section differs in that it needs **no separate query** — it derives entirely from the already-fetched `RelatedMemoriesResponse` (per UI-SPEC E3 "loading" row: "no separate network call"). Model it as a `$derived` over the selected node id and the existing `relatedQuery.data`, not a second `createQuery`.

---

### `ui/src/lib/components/TagBars.svelte` (component, CRUD)

**Analog:** `ui/src/lib/components/FacetStrip.svelte`

**Shared-query-per-scope-key pattern** — follow the `ScopeCombobox` prop/query relationship `FacetStrip.svelte:117-125` establishes (parent passes `scopes`/`scopesLoading`/`scopesError` down; combobox is presentation-only over already-fetched data). `TagBars` should take the equivalent props (`scope`, `mode: 'panel' | 'rail'`) and internally own the `createQuery`:
```typescript
// Source: 05-RESEARCH.md Code Examples, engram-connect-client SKILL's ListTags row
const scopeKey = $derived(scope || '__all_readable__');
const tagsQuery = createQuery(() => ({
  queryKey: ['listTags', scopeKey, 1000],
  queryFn: ({ signal }) => engram.listTags({ scope, limit: 1000n }, { signal }),
  staleTime: Infinity
}));
```

**Chip toggle pattern:** `FacetStrip.svelte:39-41` (`removeTag`) and `:147-152` (tag chip render with remove button) is the direct precedent for "click a bar toggles a `#tag` facet chip" (D-13).

---

### `ui/src/lib/components/TagCombobox.svelte` (component, request-response)

**Analog:** `ui/src/lib/components/ScopeCombobox.svelte` (full file, 134 lines — exact structural match, gotcha `3tz15e733n` explicitly named in CONTEXT.md)

**Imports pattern** (lines 1-4):
```svelte
import * as Popover from '$lib/components/ui/popover';
import * as Command from '$lib/components/ui/command';
import type { ListScopesResponse } from '$lib/gen/engram_pb'; // -> ListTagsResponse
```

**shouldFilter={false} + manual substring filter pattern** (lines 33-41, 64-66):
```typescript
let filterValue = $state('');
const filtered = $derived.by(() => {
  const f = filterValue.trim().toLowerCase();
  if (!f) return entries;
  return entries.filter((s) => s.scope.toLowerCase().includes(f)); // -> tag substring, D-18 prefix-first then count
});
```
```svelte
<Command.Root shouldFilter={false} label="Filter scopes">
  <Command.Input placeholder="Filter scopes…" aria-label="Filter scopes" bind:value={filterValue} />
```
Portal disabled inside the Popover — copy `Popover.Content` usage at line 64 unchanged (`onOpenAutoFocus={(e) => e.preventDefault()}`).

**Unknown-value row pattern (D-19):** `ScopeCombobox` has no equivalent (every scope in its list is always addable) — this is the one genuinely new branch; model it as an extra `Command.Item` appended when `filterValue` matches no `filtered` entry, per the "Add #{tag}" copy in UI-SPEC.

**Error/loading pattern** (lines 68-74): copy the `loading`/`error`/`retry` status-row shape verbatim.

---

### `ui/src/lib/related/graph.ts` (utility, transform)

**Analog:** `ui/src/lib/curation/chain.ts` (full file read)

**Pure-model-from-RPC-response pattern** (lines 1-6, 30-49):
```typescript
// The pure chain model (D-06) built from two RelatedMemories responses...
// ChainDialog.svelte renders this model — it never walks the chain itself (key_links: buildChain).
import type { Memory, RelatedMemoriesResponse } from '$lib/gen/engram_pb';
import { EdgeType, SupersessionDirection } from '$lib/gen/engram_pb';

export function headIdFrom(anchorResp: RelatedMemoriesResponse): string {
  const anchorId = anchorResp.anchor?.id ?? '';
  let bestId = anchorId;
  let bestDepth = -1;
  for (const rel of anchorResp.related) {
    if (!rel.memory) continue;
    for (const edge of rel.edges) {
      if (edge.type === EdgeType.SUPERSESSION && edge.evidence.case === 'supersession' && edge.evidence.value.direction === SupersessionDirection.SUCCESSOR) {
        if (edge.evidence.value.depth > bestDepth) { bestDepth = edge.evidence.value.depth; bestId = rel.memory.id; }
      }
    }
  }
  return bestId;
}
```
`graph.ts`'s `buildGraph(resp)` (RESEARCH.md Code Examples section has the full stub) follows this exact shape: read `RelatedEdge.evidence`'s discriminated union (`edge.evidence.case`/`.value`) the same way, one pure function per concern, dumb Svelte component consumes the output — never re-walks the RPC response itself.

**Discriminated-union access pattern (engram-connect-client SKILL, "Per-RPC contract" section):**
```
RelatedEdge.evidence is a discriminated union ({ case, value }) keyed by RelatedEdge.type
(vector | tag | citation | supersession) — read `case` before touching `value`.
```

---

### `ui/src/lib/tags/tags.ts` (utility, transform)

**Analog (structural, not domain):** `ui/src/lib/curation/chain.ts`'s pure-function/`.test.ts` split — same "small pure function, node-testable, no DOM" shape. No existing tag-ranking code to copy domain logic from; substring+prefix+count ranking is a ~15-line function per RESEARCH.md's "Don't Hand-Roll" table (do not import a fuzzy-match library).

---

### `ui/src/lib/components/DetailPane.svelte`, `RowActions.svelte`, `CommandMenu.svelte` (modified, entry points D-03)

**Analog:** the files' own existing action/item lists (self-analogs — these are additive edits, not new components).

- `RowActions.svelte` / `ResultsList.svelte`'s `handleListboxKey` switch (lines 504-525 above): add `case 'r':` alongside the existing `case 'x':` branch, guarded by the same `isTypingTarget`/modifier checks the file already applies elsewhere (per `NAV_KEYS` set at line 622 — add `'r'` there too if it must skip the typing-target guard the same way nav keys do).
- `CommandMenu.svelte`: add a `Related to {short_id}` item following the existing item-list pattern in that file (214 lines total — read the existing `Command.Item` blocks near the file's list-rendering section before adding).
- `DetailPane.svelte`: add a `Related` button beside the existing action buttons in its actions block (533 lines total — locate the existing button row, e.g. near "Supersede…"/"Archive" buttons, and follow their exact `Button`/`onclick` shape).

---

## Shared Patterns

### Query-key convention and RPC client routing
**Source:** `.claude/skills/engram-connect-client/SKILL.md` ("Per-RPC contract (curation RPCs)" table)
**Apply to:** every new query in this phase
```
RelatedMemories -> engram client, query-key prefix 'relatedMemories'
ListTags        -> engram client, query-key prefix 'listTags'
```
Both are reads — route through `engram`, never `engramWrite`; no CSRF interceptor involved.

### Roving aria-activedescendant listbox
**Source:** `ui/src/lib/components/ResultsList.svelte` lines 335-346, 404-431, 504-525
**Apply to:** `RelatedGraph.svelte` (D-10's one-Tab-stop model)

### Pure-model / dumb-component split
**Source:** `ui/src/lib/curation/chain.ts` + `ui/src/lib/components/ChainDialog.svelte`
**Apply to:** `ui/src/lib/related/graph.ts` + `RelatedGraph.svelte`/`RelatedLanes.svelte`, `ui/src/lib/tags/tags.ts` + `TagBars.svelte`/`TagCombobox.svelte`

### bits-ui Command combobox with manual filtering
**Source:** `ui/src/lib/components/ScopeCombobox.svelte` (full file)
**Apply to:** `TagCombobox.svelte`

### `$app/state` route-param mocking in vitest-browser
**Source:** `ui/src/routes/scheduled/scheduled.browser.test.ts` line 47
**Apply to:** `related.browser.test.ts` (add `params: { id }` to the mocked `pageState`)

### Not-found / error envelope copy
**Source:** `ui/src/lib/errors/connect-error.ts` (`parseConnectError`, `fixRowsFor`) + `ChainDialog.svelte` line 150 (`No memory with id {id} that you can read`)
**Apply to:** `/related/[id]/+page.svelte`'s anchor-not-found state, and any chain-member placeholder in the supersession lane

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `RelatedGraph.svelte`'s d3-force/drag/zoom wiring (physics mount, tick loop, zoom filter) | component | event-driven (gesture/physics) | No d3 usage exists anywhere in this codebase yet (first consumer of the milestone's second UI-library slot) — RESEARCH.md's Patterns 1-2 and Code Examples are the sole source; copy those verbatim rather than inventing an approach. |
| Seeded-PRNG helper for `simulation.randomSource()` | utility | transform | Net-new, ~5 lines, no in-repo precedent (RESEARCH.md Assumption A1) — inline in `graph.ts`. |

## Metadata

**Analog search scope:** `ui/src/lib/components/`, `ui/src/lib/curation/`, `ui/src/routes/`, `ui/src/lib/errors/`, `.claude/skills/engram-connect-client/`, `.claude/skills/engram-console-conventions/`
**Files scanned:** `ScopeCombobox.svelte`, `ChainDialog.svelte`, `ResultsList.svelte` (+ `.browser.test.ts`), `FacetStrip.svelte`, `RowActions.svelte`, `CommandMenu.svelte`, `DetailPane.svelte`, `curation/chain.ts`, `routes/scheduled/+page.svelte` (+ test), `client.ts`/`errors/connect-error.ts` (referenced via SKILL.md)
**Pattern extraction date:** 2026-09-27
