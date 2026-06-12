<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Sean Brandt
-->

# Operator Console Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the engram operator console (`ui/`) as a modern, scannable, responsive shadcn-svelte app — fixing the rendering bugs and migrating off the custom `eg-*`/`--cat-*` layer — without backend changes.

**Architecture:** Foundation-first. Migrate the theme to shadcn semantic tokens and add the component primitives; build pure formatters (`scope`, `time`); then rebuild each surface (shell → scopes sidebar/scope-chip → list/row → detail → command palette/states), wiring the existing svelte-query v6 + URL-as-state data flow unchanged. Each task is independently testable and the app stays runnable throughout.

**Tech Stack:** SvelteKit 2 / Svelte 5 (runes), Tailwind v4, shadcn-svelte (bits-ui 2.x, tailwind-variants, `cn`), @tanstack/svelte-query v6, connect-es v2, vitest + @testing-library/svelte, pnpm.

**Conventions (enforced — see `engram-kco.2` grounding notes):** semantic tokens only (`bg-background`, `text-muted-foreground`), component composition over custom markup, `cn()` for conditional classes, `truncate` shorthand, `flex gap-*` (no `space-*`), `size-*` for equal dims, icons via `data-icon` (no sizing classes), read `shadcn-svelte.com/docs/components/<name>.md` before using a component. Run all CLI via `pnpm dlx shadcn-svelte@latest`. Run tests from `ui/` via `pnpm exec vitest run <file>`.

---

## Task 1: Theme + token foundation

Migrate `app.css` to shadcn's standard semantic tokens (light + dark), keep the four category hues as accents, add the icon library, and vendor the base primitives. No component depends on `eg-*` after the redesign; this task establishes the tokens they will use.

**Files:**

- Modify: `ui/src/app.css`
- Modify: `ui/components.json` (add `iconLibrary`)
- Create (via CLI): `ui/src/lib/components/ui/{card,badge,sidebar,resizable,scroll-area,separator,tooltip,hover-card,command,dialog,sheet,dropdown-menu,avatar,empty,skeleton,sonner,pagination,kbd,item,input,tabs}/`

- [ ] **Step 1: Add the icon library to `components.json`**

Add the `iconLibrary` key (there is none today) so `shadcn-svelte` knows which icon package to wire:

```jsonc
{
  "$schema": "https://shadcn-svelte.com/schema.json",
  "style": "default",
  "tailwind": { "css": "src/app.css", "baseColor": "neutral", "cssVariables": true },
  "aliases": { "components": "$lib/components", "utils": "$lib/utils", "ui": "$lib/components/ui", "hooks": "$lib/hooks", "lib": "$lib" },
  "typescript": true,
  "registry": "https://shadcn-svelte.com/registry",
  "iconLibrary": "@lucide/svelte"
}
```

- [ ] **Step 2: Install the icon package**

Run: `pnpm add -D @lucide/svelte`
Expected: added to `ui/package.json` devDependencies.

- [ ] **Step 3: Rewrite `app.css` to shadcn semantic tokens + category accents**

Replace the entire `ui/src/app.css` with shadcn's neutral theme variables plus the four category hues as theme tokens. Remove all `eg-*` utilities and the old `--background/--surface/--foreground/--muted/--accent` set:

```css
@import 'tailwindcss';

@custom-variant dark (&:is(.dark *));

:root {
  --background: #ffffff; --foreground: #1f2328;
  --card: #ffffff; --card-foreground: #1f2328;
  --popover: #ffffff; --popover-foreground: #1f2328;
  --primary: #1a7f37; --primary-foreground: #ffffff;
  --secondary: #f6f8fa; --secondary-foreground: #1f2328;
  --muted: #f6f8fa; --muted-foreground: #59636e;
  --accent: #f6f8fa; --accent-foreground: #1f2328;
  --border: #d0d7de; --input: #d0d7de; --ring: #1a7f37;
  --radius: 0.5rem;
  --cat-convention: #0969da; --cat-gotcha: #bc4c00; --cat-decision: #8250df; --cat-preference: #0550ae;
}
.dark {
  --background: #0d1117; --foreground: #e6edf3;
  --card: #161b22; --card-foreground: #e6edf3;
  --popover: #161b22; --popover-foreground: #e6edf3;
  --primary: #3fb950; --primary-foreground: #0d1117;
  --secondary: #161b22; --secondary-foreground: #e6edf3;
  --muted: #161b22; --muted-foreground: #8b949e;
  --accent: #21262d; --accent-foreground: #e6edf3;
  --border: #30363d; --input: #30363d; --ring: #3fb950;
  --cat-convention: #a5d6ff; --cat-gotcha: #ffa657; --cat-decision: #d2a8ff; --cat-preference: #79c0ff;
}

@theme inline {
  --color-background: var(--background); --color-foreground: var(--foreground);
  --color-card: var(--card); --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover); --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary); --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary); --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted); --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent); --color-accent-foreground: var(--accent-foreground);
  --color-border: var(--border); --color-input: var(--input); --color-ring: var(--ring);
  --color-cat-convention: var(--cat-convention); --color-cat-gotcha: var(--cat-gotcha);
  --color-cat-decision: var(--cat-decision); --color-cat-preference: var(--cat-preference);
  --radius-lg: var(--radius);
}

body { background: var(--background); color: var(--foreground); font-family: ui-sans-serif, system-ui, -apple-system, sans-serif; font-size: 13px; }
```

> Verify against `shadcn-svelte.com/docs/theming.md` that the variable names match the installed `style: default` registry; adjust any name the CLI-added components expect.

- [ ] **Step 4: Vendor the base primitives**

Run: `pnpm dlx shadcn-svelte@latest add card badge sidebar resizable scroll-area separator tooltip hover-card command dialog sheet dropdown-menu avatar empty skeleton sonner pagination kbd item input tabs`
Expected: each component folder created under `ui/src/lib/components/ui/`. Accept overwrites only for new folders; do NOT overwrite the existing `button`/`checkbox`/`select` without review.

Then install the `sonner` component's runtime peer dependency (the `sonner` registry component imports `toast`/`Toaster` from `svelte-sonner`, which is not a transitive dep today):

Run: `pnpm add svelte-sonner`
Expected: `svelte-sonner` added to `ui/package.json`. (Verify after the CLI add whether it already pulled it in; skip if present.)

- [ ] **Step 5: Verify the app still builds**

Run: `pnpm build`
Expected: build succeeds (existing components still reference old `eg-*` — that is fine until each is rewritten; `app.css` no longer defines them, so expect unstyled-but-not-broken; if the build errors on a missing class, it is a Tailwind utility, not these tokens — none of `eg-*` were `@apply`-ed, so build passes).

- [ ] **Step 6: Commit**

Commit per `references/vcs-preamble.md`: `feat(ui): adopt shadcn semantic tokens + vendor primitives (engram-kco)`.

---

## Task 2: `lib/scope.ts` — scope parser (pure, TDD)

Parse `type:body` scope strings into a structured value with short/long display forms for the ScopeChip.

**Files:**

- Create: `ui/src/lib/scope.ts`
- Test: `ui/src/lib/scope.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from 'vitest';
import { parseScope } from './scope';

describe('parseScope', () => {
  it('parses a github repo scope into type/org/name', () => {
    const s = parseScope('repo:github.com/fzymgc-house/selfhosted-cluster');
    expect(s.type).toBe('repo');
    expect(s.org).toBe('fzymgc-house');
    expect(s.name).toBe('selfhosted-cluster');
    expect(s.full).toBe('repo:github.com/fzymgc-house/selfhosted-cluster');
  });
  it('parses a discovery scope (nested repo)', () => {
    const s = parseScope('discovery:repo:github.com/seanb4t/engram');
    expect(s.type).toBe('discovery');
    expect(s.org).toBe('seanb4t');
    expect(s.name).toBe('engram');
  });
  it('parses a project scope with no org', () => {
    const s = parseScope('project:selfhosted-cluster');
    expect(s.type).toBe('project');
    expect(s.org).toBe('');
    expect(s.name).toBe('selfhosted-cluster');
  });
  it('falls back gracefully for an unknown shape', () => {
    const s = parseScope('weird');
    expect(s.type).toBe('');
    expect(s.name).toBe('weird');
    expect(s.full).toBe('weird');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec vitest run src/lib/scope.test.ts`
Expected: FAIL — `parseScope` not exported.

- [ ] **Step 3: Write minimal implementation**

```ts
export type ScopeType = 'repo' | 'discovery' | 'project' | '';

export interface ParsedScope {
  full: string;   // verbatim original — never lose this
  type: ScopeType;
  org: string;    // '' when none (e.g. project)
  name: string;   // the repo/project name (last path segment)
}

// Scopes look like `type:body`:
//   repo:github.com/org/name
//   discovery:repo:github.com/org/name   (discovery nests a repo body)
//   project:name
export function parseScope(full: string): ParsedScope {
  const firstColon = full.indexOf(':');
  if (firstColon === -1) return { full, type: '', org: '', name: full };

  const head = full.slice(0, firstColon);
  let rest = full.slice(firstColon + 1);
  let type: ScopeType = head === 'repo' || head === 'discovery' || head === 'project' ? head : '';

  // discovery:repo:github.com/... — unwrap the inner repo body for org/name.
  if (type === 'discovery' && rest.startsWith('repo:')) rest = rest.slice('repo:'.length);

  // Drop a leading host (github.com/...) so org/name leads.
  const segs = rest.split('/').filter(Boolean);
  const host = segs.length >= 3 && segs[0].includes('.') ? segs.shift() : undefined;
  void host;

  if (segs.length >= 2) return { full, type, org: segs[segs.length - 2], name: segs[segs.length - 1] };
  return { full, type, org: '', name: segs[0] ?? rest };
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec vitest run src/lib/scope.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

`feat(ui): scope parser for the ScopeChip (engram-kco)`.

---

## Task 3: `lib/time.ts` — relative + full timestamp (pure, TDD)

**Files:**

- Create: `ui/src/lib/time.ts`
- Test: `ui/src/lib/time.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from 'vitest';
import { relativeTime, fullTimestamp } from './time';

const NOW = new Date('2026-06-12T15:00:00Z');

describe('relativeTime', () => {
  it('renders hours', () => { expect(relativeTime(new Date('2026-06-12T10:00:00Z'), NOW)).toBe('5h'); });
  it('renders days', () => { expect(relativeTime(new Date('2026-06-10T15:00:00Z'), NOW)).toBe('2d'); });
  it('renders just now under a minute', () => { expect(relativeTime(new Date('2026-06-12T14:59:40Z'), NOW)).toBe('now'); });
});
describe('fullTimestamp', () => {
  it('renders an ISO-ish minute precision', () => { expect(fullTimestamp(new Date('2026-06-12T14:03:00Z'))).toMatch(/2026-06-12 14:03/); });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec vitest run src/lib/time.test.ts`
Expected: FAIL — functions not exported.

- [ ] **Step 3: Write minimal implementation**

```ts
// `now` is injectable for deterministic tests; defaults to current time at call.
export function relativeTime(d: Date, now: Date = new Date()): string {
  const s = Math.max(0, Math.floor((now.getTime() - d.getTime()) / 1000));
  if (s < 60) return 'now';
  const m = Math.floor(s / 60); if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60); if (h < 24) return `${h}h`;
  const days = Math.floor(h / 24); if (days < 30) return `${days}d`;
  const mo = Math.floor(days / 30); if (mo < 12) return `${mo}mo`;
  return `${Math.floor(mo / 12)}y`;
}

export function fullTimestamp(d: Date): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getUTCFullYear()}-${p(d.getUTCMonth() + 1)}-${p(d.getUTCDate())} ${p(d.getUTCHours())}:${p(d.getUTCMinutes())}`;
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec vitest run src/lib/time.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

`feat(ui): relative + full timestamp helpers (engram-kco)`.

---

## Task 4: `ScopeChip` component

A typed badge + de-emphasized org/name, full scope on a HoverCard. Two render modes: `stacked` (rail) and `inline` (rows/detail).

**Files:**

- Create: `ui/src/lib/components/ScopeChip.svelte`
- Test: `ui/src/lib/components/ScopeChip.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ScopeChip from './ScopeChip.svelte';

describe('ScopeChip', () => {
  it('shows repo name prominently and the type badge', () => {
    render(ScopeChip, { props: { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster' } });
    expect(screen.getByText('selfhosted-cluster')).toBeInTheDocument();
    expect(screen.getByText('repo')).toBeInTheDocument();
  });
  it('keeps the full scope available (title attr) — never destroyed', () => {
    render(ScopeChip, { props: { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster' } });
    expect(screen.getByTitle('repo:github.com/fzymgc-house/selfhosted-cluster')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec vitest run src/lib/components/ScopeChip.test.ts`
Expected: FAIL — component not found.

- [ ] **Step 3: Write minimal implementation**

```svelte
<script lang="ts">
  import { parseScope } from '$lib/scope';
  import { Badge } from '$lib/components/ui/badge';
  let { scope, mode = 'inline', count }: { scope: string; mode?: 'inline' | 'stacked'; count?: number } = $props();
  const p = $derived(parseScope(scope));
  const catClass = { repo: 'text-cat-convention', discovery: 'text-cat-decision', project: 'text-cat-preference', '': 'text-muted-foreground' } as const;
</script>

<span class="inline-flex items-center gap-2 min-w-0" title={p.full}>
  <Badge variant="outline" class="shrink-0 text-[10px] uppercase {catClass[p.type]}">{p.type || 'scope'}</Badge>
  {#if mode === 'stacked'}
    <span class="flex flex-col min-w-0">
      <span class="truncate font-mono text-[13px]">{p.name}</span>
      {#if p.org}<span class="truncate font-mono text-[10px] text-muted-foreground opacity-70">{p.org}</span>{/if}
    </span>
  {:else}
    <span class="truncate font-mono text-[12px]">
      {#if p.org}<span class="text-muted-foreground opacity-60 text-[11px]">{p.org}/</span>{/if}{p.name}
    </span>
  {/if}
  {#if count !== undefined}<span class="ml-auto shrink-0 text-[11px] tabular-nums text-muted-foreground">{count}</span>{/if}
</span>
```

> A later refinement (separate step within this task) wraps the chip in `HoverCard` (`shadcn-svelte.com/docs/components/hover-card.md`) to show type + full scope + count + org; the `title` attr above is the minimal accessible fallback the test asserts. Add the HoverCard, keep the `title`, re-run the test.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec vitest run src/lib/components/ScopeChip.test.ts`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

`feat(ui): ScopeChip (typed badge + de-emphasized org + full-scope hover) (engram-kco)`.

---

## Task 5: `AppShell` — nav rail + resizable frame + top bar

Rebuild `AppShell.svelte` as the Option-A shell: a slim icon nav rail (Sidebar), a top bar (brand, ⌘K trigger with Kbd, theme toggle), and a Resizable content frame the routes fill.

**Files:**

- Modify: `ui/src/lib/components/AppShell.svelte`
- Modify: `ui/src/lib/components/AppShell.test.ts`

- [ ] **Step 1: Update the failing test**

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
  it('renders nav links and the command trigger', () => {
    render(AppShell);
    expect(screen.getByRole('link', { name: /observe/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /search/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /toggle theme/i })).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec vitest run src/lib/components/AppShell.test.ts`
Expected: FAIL — no `observe` link yet (current shell only has search + theme).

- [ ] **Step 3: Rewrite `AppShell.svelte`**

```svelte
<script lang="ts">
  import { setMode, mode } from 'mode-watcher';
  import { base } from '$app/paths';
  import { Button } from '$lib/components/ui/button';
  import * as Sidebar from '$lib/components/ui/sidebar';
  import { Kbd } from '$lib/components/ui/kbd';
  import EyeIcon from '@lucide/svelte/icons/eye';
  import SearchIcon from '@lucide/svelte/icons/search';
  import CompassIcon from '@lucide/svelte/icons/compass';
  import SunMoonIcon from '@lucide/svelte/icons/sun-moon';
  let { children, oncommand }: { children?: import('svelte').Snippet; oncommand?: () => void } = $props();
  const nav = [
    { href: `${base}/observe`, label: 'Observe', icon: EyeIcon },
    { href: `${base}/search`, label: 'Search', icon: SearchIcon },
    { href: `${base}/discovery`, label: 'Discovery', icon: CompassIcon }
  ];
  function cycleTheme() { setMode(mode.current === 'dark' ? 'light' : 'dark'); }
</script>

<div class="min-h-screen flex flex-col bg-background text-foreground">
  <header class="flex items-center gap-3 px-3 py-2 border-b border-border">
    <span class="font-bold text-primary">◆ engram</span>
    <Button variant="outline" aria-label="search" class="flex-1 justify-start text-muted-foreground" onclick={() => oncommand?.()}>
      <SearchIcon data-icon="inline-start" /> search memories… <Kbd class="ml-auto">⌘K</Kbd>
    </Button>
    <Button variant="outline" size="sm" aria-label="toggle theme" onclick={cycleTheme}><SunMoonIcon data-icon="inline-start" /></Button>
  </header>
  <div class="flex flex-1 min-h-0">
    <nav class="flex flex-col gap-1 p-2 border-r border-border w-[64px] items-center">
      {#each nav as n (n.href)}
        <a href={n.href} aria-label={n.label} class="flex flex-col items-center gap-1 p-2 rounded text-[10px] text-muted-foreground hover:bg-accent hover:text-foreground">
          <n.icon data-icon="inline-start" />{n.label}
        </a>
      {/each}
    </nav>
    <main class="flex-1 min-w-0">{@render children?.()}</main>
  </div>
</div>
```

> If the `sidebar` import is unused after this minimal nav, drop it; the icon-rail nav above is sufficient for v1. Read `shadcn-svelte.com/docs/components/kbd.md` for the `Kbd` import shape.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec vitest run src/lib/components/AppShell.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

`feat(ui): AppShell nav rail + command trigger + theme (engram-kco)`.

---

## Task 6: `ScopesSidebar` — scope list (ScopeChip) + filters

Replace `ScopeRail.svelte` with `ScopesSidebar.svelte`: scopes rendered via `ScopeChip mode="stacked"` (no overflow), category checkboxes, visibility select.

**Files:**

- Create: `ui/src/lib/components/ScopesSidebar.svelte`
- Create: `ui/src/lib/components/ScopesSidebar.test.ts`
- Delete: `ui/src/lib/components/ScopeRail.svelte` (after route rewire in Task 10)

- [ ] **Step 1: Write the failing test**

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ScopesSidebar from './ScopesSidebar.svelte';
import { create } from '@bufbuild/protobuf';
import { ScopeCountSchema } from '$lib/gen/engram_pb';

const scopes = [create(ScopeCountSchema, { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster', count: 142n })];

describe('ScopesSidebar', () => {
  it('renders a scope chip and the filter categories', () => {
    render(ScopesSidebar, { props: { scopes, activeScope: '', categories: [], visibility: '', loading: false, error: null, onscope: () => {}, onfilter: () => {} } });
    expect(screen.getByText('selfhosted-cluster')).toBeInTheDocument();
    expect(screen.getByText('gotcha')).toBeInTheDocument();
  });
});
```

> Confirm `ScopeCountSchema`'s `count` field type with `mcp__probe__extract_code ScopeCount` before writing the fixture (proto numeric → bigint in protobuf-es).

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm exec vitest run src/lib/components/ScopesSidebar.test.ts`
Expected: FAIL — component not found.

- [ ] **Step 3: Write `ScopesSidebar.svelte`**

```svelte
<script lang="ts">
  import type { ScopeCount } from '$lib/gen/engram_pb';
  import type { Category, Visibility } from '$lib/queries';
  import { Button } from '$lib/components/ui/button';
  import { Checkbox } from '$lib/components/ui/checkbox';
  import { Select } from '$lib/components/ui/select';
  import { Skeleton } from '$lib/components/ui/skeleton';
  import ScopeChip from './ScopeChip.svelte';
  let { scopes, activeScope, categories, visibility, loading = false, error = null, onscope, onfilter }: {
    scopes: ScopeCount[]; activeScope: string; categories: Category[]; visibility: Visibility;
    loading?: boolean; error?: unknown; onscope: (s: string) => void; onfilter: (c: Category[], v: Visibility) => void;
  } = $props();
  const allCats: Category[] = ['convention', 'gotcha', 'decision', 'preference'];
  function toggleCat(c: Category) {
    onfilter(categories.includes(c) ? categories.filter((x) => x !== c) : [...categories, c], visibility);
  }
  const visOptions = [{ value: '', label: 'all' }, { value: 'private', label: 'private' }, { value: 'shared', label: 'shared' }];
</script>

<div class="w-[240px] shrink-0 border-r border-border p-3 flex flex-col gap-1 overflow-y-auto">
  <div class="text-[10px] uppercase text-muted-foreground">Scopes</div>
  {#if error}
    <div data-testid="scopes-error" class="text-cat-gotcha py-1 text-sm">failed to load scopes</div>
  {:else if loading}
    <Skeleton class="h-7 w-full" /><Skeleton class="h-7 w-full" />
  {:else}
    {#each scopes as s (s.scope)}
      <Button variant="ghost" class={'h-auto justify-start w-full ' + (s.scope === activeScope ? 'bg-accent' : '')} onclick={() => onscope(s.scope)}>
        <ScopeChip scope={s.scope} mode="stacked" count={Number(s.count)} />
      </Button>
    {/each}
  {/if}
  <div class="mt-3 text-[10px] uppercase text-muted-foreground">Filters</div>
  {#each allCats as c (c)}
    <label class="flex items-center gap-2 text-sm" style="color:var(--cat-{c})">
      <Checkbox checked={categories.includes(c)} onCheckedChange={() => toggleCat(c)} aria-label={c} />{c}
    </label>
  {/each}
  <div class="mt-2 text-[10px] uppercase text-muted-foreground">visibility</div>
  <Select value={visibility} options={visOptions} ariaLabel="visibility" onValueChange={(v) => onfilter(categories, v as Visibility)} />
</div>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm exec vitest run src/lib/components/ScopesSidebar.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

`feat(ui): ScopesSidebar with ScopeChip + filters (engram-kco)`.

---

## Task 7: `MemoryList` + compact `MemoryRow` + states + pagination

Rewrite `MemoryList.svelte` to render compact rows (Badge chip + stripped one-line summary + meta line), with `Empty`/`Skeleton` states; move pagination into the list footer via the `Pagination` component.

**Files:**

- Modify: `ui/src/lib/components/MemoryList.svelte`
- Create: `ui/src/lib/components/MemoryRow.svelte`
- Modify: `ui/src/lib/components/MemoryList.test.ts`
- Create: `ui/src/lib/summary.ts` + `ui/src/lib/summary.test.ts` (strip the redundant leading category token)

- [ ] **Step 1: Write the failing summary test**

```ts
import { describe, it, expect } from 'vitest';
import { stripCategoryPrefix } from './summary';

describe('stripCategoryPrefix', () => {
  it('strips a leading ALLCAPS category token matching the category', () => {
    expect(stripCategoryPrefix('GOTCHA (agentgateway path) must match', 'gotcha')).toBe('agentgateway path) must match');
  });
  it('leaves content alone when there is no redundant prefix', () => {
    expect(stripCategoryPrefix('Uptime Kuma monitors as IaC', 'convention')).toBe('Uptime Kuma monitors as IaC');
  });
});
```

- [ ] **Step 2: Run + implement `summary.ts`**

Run: `pnpm exec vitest run src/lib/summary.test.ts` → FAIL, then:

```ts
// Records are often authored "CATEGORY (…": the badge already shows the
// category, so strip a leading ALLCAPS token equal to the category plus an
// optional "(" and surrounding punctuation/space.
export function stripCategoryPrefix(content: string, category: string): string {
  const re = new RegExp('^' + category.toUpperCase() + '\\s*\\(?\\s*[:\\-]?\\s*', '');
  return content.replace(re, '').trimStart();
}
```

Re-run → PASS.

- [ ] **Step 3: Write the failing `MemoryRow`/`MemoryList` test**

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import MemoryList from './MemoryList.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const mem = create(MemorySchema, { id: '1', content: 'GOTCHA (path must match) upstream', category: 'gotcha', visibility: 'private', tags: ['mcp', 'routing'] });

describe('MemoryList', () => {
  it('renders the category badge and a de-duplicated summary', () => {
    render(MemoryList, { props: { memories: [mem], total: 1n, loading: false, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByText('gotcha')).toBeInTheDocument();
    expect(screen.getByText(/path must match/)).toBeInTheDocument();
    expect(screen.queryByText(/GOTCHA \(/)).not.toBeInTheDocument(); // redundant prefix stripped
  });
  it('shows an Empty state when there are no memories', () => {
    render(MemoryList, { props: { memories: [], total: 0n, loading: false, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByText(/no memories/i)).toBeInTheDocument();
  });
  it('shows a skeleton when loading', () => {
    render(MemoryList, { props: { memories: [], total: 0n, loading: true, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByTestId('list-loading')).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Run to verify it fails**

Run: `pnpm exec vitest run src/lib/components/MemoryList.test.ts`
Expected: FAIL (prefix not stripped / Empty not used).

- [ ] **Step 5: Write `MemoryRow.svelte`**

```svelte
<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { Badge } from '$lib/components/ui/badge';
  import { stripCategoryPrefix } from '$lib/summary';
  import { relativeTime } from '$lib/time';
  import ScopeChip from './ScopeChip.svelte';
  let { memory, selected, showScope = false, onselect }: { memory: Memory; selected: boolean; showScope?: boolean; onselect: (id: string) => void } = $props();
  const summary = $derived(stripCategoryPrefix(memory.content, memory.category));
  const when = $derived(memory.createdAt ? relativeTime(timestampDate(memory.createdAt)) : '');
  const shownTags = $derived(memory.tags.slice(0, 3));
  const overflow = $derived(Math.max(0, memory.tags.length - 3));
</script>

<button
  type="button"
  onclick={() => onselect(memory.id)}
  class={'w-full text-left px-3 py-2 border-b border-border flex flex-col gap-1.5 hover:bg-accent ' + (selected ? 'bg-accent shadow-[inset_2px_0_0_var(--primary)]' : '')}
>
  <div class="flex items-center gap-2 min-w-0">
    <Badge variant="outline" class="shrink-0 text-[10px] uppercase" style="color:var(--cat-{memory.category})">{memory.category}</Badge>
    <span class="truncate flex-1 text-[13px]">{summary}</span>
  </div>
  <div class="flex items-center gap-2 text-[11px] text-muted-foreground min-w-0">
    <span class="tabular-nums shrink-0">{when}</span>
    {#if showScope && memory.scope}<span class="shrink-0"><ScopeChip scope={memory.scope} /></span>{/if}
    {#each shownTags as t (t)}<span class="shrink-0 px-1 rounded bg-muted font-mono text-[10.5px]">{t}</span>{/each}
    {#if overflow > 0}<span class="shrink-0">+{overflow}</span>{/if}
  </div>
</button>
```

- [ ] **Step 6: Rewrite `MemoryList.svelte`**

```svelte
<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { Skeleton } from '$lib/components/ui/skeleton';
  import * as Empty from '$lib/components/ui/empty';
  import MemoryRow from './MemoryRow.svelte';
  let { memories, total, approximate = false, loading, error, selectedId, onselect }: {
    memories: Memory[]; total: bigint; approximate?: boolean; loading: boolean; error: unknown; selectedId: string; onselect: (id: string) => void;
  } = $props();
</script>

{#if loading}
  <div data-testid="list-loading" class="p-3 flex flex-col gap-2"><Skeleton class="h-12 w-full" /><Skeleton class="h-12 w-full" /><Skeleton class="h-12 w-full" /></div>
{:else if error}
  <div class="p-3 text-cat-gotcha">failed to load — retry from the toolbar</div>
{:else if memories.length === 0}
  <Empty.Root class="p-8"><Empty.Title>no memories</Empty.Title><Empty.Description>nothing in this scope / filter</Empty.Description></Empty.Root>
{:else}
  {#each memories as m (m.id)}
    <MemoryRow memory={m} selected={m.id === selectedId} {onselect} />
  {/each}
  <div class="px-3 py-2 text-center text-muted-foreground text-[11px]">{memories.length} of {total}{approximate ? ' (approximate)' : ''}</div>
{/if}
```

> Read `shadcn-svelte.com/docs/components/empty.md` for the exact `Empty.*` barrel exports; adjust import if the registry uses different subcomponent names.

- [ ] **Step 7: Run to verify it passes**

Run: `pnpm exec vitest run src/lib/components/MemoryList.test.ts src/lib/summary.test.ts`
Expected: PASS.

- [ ] **Step 8: Commit**

`feat(ui): compact memory rows + Empty/Skeleton states (engram-kco)`.

---

## Task 8: `MemoryDetail` — Card layout (Option B) + copy

Rewrite `MemoryDetail.svelte`: Card header (badge + relative time + copy) → metadata block (full scope own line + provenance pills) → ScrollArea body → tags.

**Files:**

- Modify: `ui/src/lib/components/MemoryDetail.svelte`
- Create: `ui/src/lib/components/MemoryDetail.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import MemoryDetail from './MemoryDetail.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const mem = create(MemorySchema, { id: '1', content: 'full body here', category: 'gotcha', scope: 'repo:github.com/fzymgc-house/selfhosted-cluster', source: 'agent-inferred', actor: 'sean', visibility: 'private', tags: ['mcp'] });

describe('MemoryDetail', () => {
  it('shows the full scope verbatim and the body', () => {
    render(MemoryDetail, { props: { memory: mem, loading: false, error: null } });
    expect(screen.getByText('repo:github.com/fzymgc-house/selfhosted-cluster')).toBeInTheDocument();
    expect(screen.getByText('full body here')).toBeInTheDocument();
  });
  it('prompts to select when nothing is chosen', () => {
    render(MemoryDetail, { props: { memory: undefined, loading: false, error: null } });
    expect(screen.getByText(/select a record/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `pnpm exec vitest run src/lib/components/MemoryDetail.test.ts`
Expected: FAIL (full scope not rendered verbatim in current 300px layout / new structure absent).

- [ ] **Step 3: Rewrite `MemoryDetail.svelte`**

```svelte
<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { ConnectError, Code } from '@connectrpc/connect';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { ScrollArea } from '$lib/components/ui/scroll-area';
  import { toast } from 'svelte-sonner';
  import { relativeTime, fullTimestamp } from '$lib/time';
  import CopyIcon from '@lucide/svelte/icons/copy';
  let { memory, loading, error }: { memory: Memory | undefined; loading: boolean; error: unknown } = $props();
  const notFound = $derived(error instanceof ConnectError && error.code === Code.NotFound);
  const created = $derived(memory?.createdAt ? timestampDate(memory.createdAt) : undefined);
  async function copy() { if (memory) { await navigator.clipboard.writeText(memory.content); toast.success('copied'); } }
</script>

<div class="w-[360px] shrink-0 border-l border-border flex flex-col min-h-0">
  {#if loading}
    <div class="p-3 text-muted-foreground">loading…</div>
  {:else if notFound}
    <div class="p-3 text-muted-foreground">record not found</div>
  {:else if error}
    <div class="p-3 text-cat-gotcha">failed to load record</div>
  {:else if !memory}
    <div class="p-3 text-muted-foreground">select a record</div>
  {:else}
    <div class="flex items-center gap-2 p-3 border-b border-border">
      <Badge variant="outline" class="text-[10px] uppercase" style="color:var(--cat-{memory.category})">{memory.category}</Badge>
      {#if created}<span class="text-[11px] text-muted-foreground" title={fullTimestamp(created)}>{relativeTime(created)}</span>{/if}
      <Button variant="outline" size="sm" class="ml-auto" aria-label="copy content" onclick={copy}><CopyIcon data-icon="inline-start" /> copy</Button>
    </div>
    <div class="p-3 border-b border-border flex flex-col gap-2">
      <div class="text-[11.5px] font-mono truncate" title={memory.scope}>{memory.scope}</div>
      <div class="flex gap-1.5 flex-wrap text-[10.5px]">
        <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">by</span> {memory.actor}</span>
        <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">src</span> {memory.source}</span>
        <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">vis</span> {memory.visibility}</span>
      </div>
    </div>
    <ScrollArea class="flex-1 min-h-0"><div class="p-3 text-[13px] leading-relaxed whitespace-pre-wrap">{memory.content}</div></ScrollArea>
    <div class="p-3 border-t border-border flex gap-1.5 flex-wrap">
      {#each memory.tags as t (t)}<span class="px-1.5 rounded bg-muted font-mono text-[10.5px]">{t}</span>{/each}
    </div>
  {/if}
</div>
```

> Read `shadcn-svelte.com/docs/components/scroll-area.md` and `.../sonner.md`. `svelte-sonner`'s `<Toaster />` (the `sonner` component) must be mounted once in the root layout — added in Task 9.

- [ ] **Step 4: Run to verify it passes**

Run: `pnpm exec vitest run src/lib/components/MemoryDetail.test.ts`
Expected: PASS. (If `navigator.clipboard` is referenced at render, the test still passes — `copy()` is only called on click.)

- [ ] **Step 5: Commit**

`feat(ui): MemoryDetail Card layout + full scope + copy (engram-kco)`.

---

## Task 9: `CommandPalette` (⌘K) + Sonner toaster, wired in the layout

Add a ⌘K Command-in-Dialog palette for searching memories + jumping to views/scopes, mount the Sonner `<Toaster />`, and wire the global ⌘K shortcut.

**Files:**

- Create: `ui/src/lib/components/CommandPalette.svelte`
- Create: `ui/src/lib/components/CommandPalette.test.ts`
- Modify: `ui/src/routes/+layout.svelte`

- [ ] **Step 1: Write the failing test**

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import CommandPalette from './CommandPalette.svelte';

describe('CommandPalette', () => {
  it('renders the search input when open', () => {
    render(CommandPalette, { props: { open: true, onsearch: () => {}, onnavigate: () => {} } });
    expect(screen.getByPlaceholderText(/search memories/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `pnpm exec vitest run src/lib/components/CommandPalette.test.ts`
Expected: FAIL — component not found.

- [ ] **Step 3: Write `CommandPalette.svelte`**

```svelte
<script lang="ts">
  import * as Command from '$lib/components/ui/command';
  let { open = $bindable(false), onsearch, onnavigate }: { open?: boolean; onsearch: (q: string) => void; onnavigate: (href: string) => void } = $props();
  import { base } from '$app/paths';
  let q = $state('');
</script>

<Command.Dialog bind:open>
  <Command.Input placeholder="search memories…" bind:value={q} />
  <Command.List>
    <Command.Empty>no matches</Command.Empty>
    <Command.Group heading="Search">
      <Command.Item onSelect={() => { onsearch(q); open = false; }}>Search memories for “{q}”</Command.Item>
    </Command.Group>
    <Command.Group heading="Go to">
      <Command.Item onSelect={() => { onnavigate(`${base}/observe`); open = false; }}>Observe</Command.Item>
      <Command.Item onSelect={() => { onnavigate(`${base}/discovery`); open = false; }}>Discovery</Command.Item>
    </Command.Group>
  </Command.List>
</Command.Dialog>
```

> Read `shadcn-svelte.com/docs/components/command.md` for the exact `Command.Dialog`/`Command.Input` API (bits-ui 2.x) and adjust prop names if needed.

- [ ] **Step 4: Wire it + the Toaster in `+layout.svelte` (PRESERVE existing error wiring)**

The current `+layout.svelte` constructs `queryClient` **inline** with a `QueryCache.onError` (auth-redirect via `mapAuthError` + `reportError`), runs `beforeNavigate(clearError)`, and renders an error banner. **Do not import `queryClient` from `$lib/client` — it is not exported there.** Keep all of that; only ADD the `Toaster`, the `CommandPalette`, the ⌘K handler, and the `oncommand` prop, and migrate the banner's `eg-error`/`var(--surface)` styling to semantic tokens:

```svelte
<script lang="ts">
  import '../app.css';
  import { QueryClient, QueryClientProvider, QueryCache } from '@tanstack/svelte-query';
  import { ModeWatcher } from 'mode-watcher';
  import { beforeNavigate, goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { mapAuthError } from '$lib/client';
  import { errorBanner, reportError, clearError } from '$lib/errors';
  import { Toaster } from '$lib/components/ui/sonner';
  import { Button } from '$lib/components/ui/button';
  import AppShell from '$lib/components/AppShell.svelte';
  import CommandPalette from '$lib/components/CommandPalette.svelte';
  let { children } = $props();

  // PRESERVE: inline queryClient with the auth-redirect / error-report onError.
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 30_000 } },
    queryCache: new QueryCache({
      onError: (err) => {
        const target = mapAuthError(err);
        if (target) { window.location.assign(target); return; }
        reportError(err);
      }
    })
  });
  beforeNavigate(() => clearError());

  let cmdOpen = $state(false);
  function onkey(e: KeyboardEvent) { if ((e.metaKey || e.ctrlKey) && e.key === 'k') { e.preventDefault(); cmdOpen = true; } }
</script>

<svelte:window onkeydown={onkey} />
<ModeWatcher />
<Toaster />
<QueryClientProvider client={queryClient}>
  {#if $errorBanner}
    <div role="alert" class="flex items-center justify-between gap-3 px-3 py-2 bg-card text-cat-gotcha border-b border-cat-gotcha">
      <span>error: {$errorBanner}</span>
      <Button variant="ghost" size="sm" aria-label="dismiss error" onclick={clearError}>✕</Button>
    </div>
  {/if}
  <AppShell oncommand={() => (cmdOpen = true)}>{@render children()}</AppShell>
  <CommandPalette bind:open={cmdOpen} onsearch={(q) => goto(`${base}/search?q=${encodeURIComponent(q)}`)} onnavigate={(href) => goto(href)} />
</QueryClientProvider>
```

> `$lib/errors` exports `errorBanner` (a store, read with `$`), `reportError`, `clearError`; `$lib/client` exports `mapAuthError`. Verify these with `mcp__probe__extract_code` before editing.

- [ ] **Step 5: Run to verify it passes**

Run: `pnpm exec vitest run src/lib/components/CommandPalette.test.ts`
Expected: PASS.

- [ ] **Step 6: Commit**

`feat(ui): ⌘K command palette + sonner toaster (engram-kco)`.

---

## Task 10: Wire routes to the new components; remove the old shell pieces

Point `observe/+page.svelte` at `ScopesSidebar` + the new `MemoryList` inside a `Resizable` frame; update `search`/`discovery` to the new components; delete `ScopeRail.svelte` and `SearchPalette.svelte`.

**Files:**

- Modify: `ui/src/routes/observe/+page.svelte`
- Modify: `ui/src/routes/search/+page.svelte`
- Modify: `ui/src/routes/discovery/+page.svelte`
- Delete: `ui/src/lib/components/ScopeRail.svelte`, `ui/src/lib/components/SearchPalette.svelte` (+ `SearchPalette.test.ts`)

- [ ] **Step 1: Rewrite `observe/+page.svelte` body**

Keep the `<script>` query wiring (svelte-query v6 runes + `parseObserveParams`/`observeSearch`/`navigate`) exactly as it is today; replace only the markup and component imports:

```svelte
<script lang="ts">
  // ...unchanged query wiring from the current file (params, navigate, scopesQ, listQ, detailQ)...
  import ScopesSidebar from '$lib/components/ScopesSidebar.svelte';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import MemoryDetail from '$lib/components/MemoryDetail.svelte';
  import * as Resizable from '$lib/components/ui/resizable';
  import { Pagination } from '$lib/components/ui/pagination';
  import { PAGE_LIMIT } from '$lib/queries';
</script>

<div class="flex h-full min-h-0">
  <ScopesSidebar
    scopes={scopesQ.data?.scopes ?? []} activeScope={params.scope}
    categories={params.categories} visibility={params.visibility}
    loading={scopesQ.isLoading} error={scopesQ.error}
    onscope={(s) => navigate({ scope: s, offset: 0, selectedId: '' })}
    onfilter={(cats, vis) => navigate({ categories: cats, visibility: vis, offset: 0 })}
  />
  <Resizable.PaneGroup direction="horizontal" class="flex-1 min-w-0">
    <Resizable.Pane defaultSize={60} minSize={35} class="flex flex-col min-h-0">
      <div class="flex-1 overflow-y-auto">
        <MemoryList memories={listQ.data?.memories ?? []} total={listQ.data?.total ?? 0n}
          approximate={listQ.data?.approximate ?? false} loading={listQ.isLoading} error={listQ.error}
          selectedId={params.selectedId} onselect={(id) => navigate({ selectedId: id })} />
      </div>
      <Pagination
        count={Number(listQ.data?.total ?? 0n)} perPage={PAGE_LIMIT} page={Math.floor(params.offset / PAGE_LIMIT) + 1}
        onPageChange={(p) => navigate({ offset: (p - 1) * PAGE_LIMIT })} />
    </Resizable.Pane>
    <Resizable.Handle />
    <Resizable.Pane defaultSize={40} minSize={25} class="min-h-0">
      <MemoryDetail memory={detailQ.data?.memory} loading={detailQ.isLoading} error={detailQ.error} />
    </Resizable.Pane>
  </Resizable.PaneGroup>
</div>
```

> Read `shadcn-svelte.com/docs/components/{resizable,pagination}.md` for the exact `Resizable.*` and `Pagination` prop names (bits-ui 2.x) and adapt the bindings; preserve the URL-as-state contract (`navigate({...})`).

- [ ] **Step 2: Update `search` + `discovery` routes**

Replace the raw `SearchPalette` input usage with the shared components: search results render via `MemoryList`, passing `showScope` through to rows (cross-scope results need the chip; `MemoryRow` already has the `showScope` prop from Task 7). Discovery uses the same list/detail. Mirror the observe wiring; reuse the existing query hooks in each route. **Note:** the new `MemoryList` interface (Task 7) drops the old `showTotal` prop — `search/+page.svelte` and `discovery/+page.svelte` currently pass `showTotal={false}`, so remove those prop usages in this step or the build will TypeScript-error. Add a `showScope` prop to `MemoryList` that it forwards to each `MemoryRow` (default `false`; observe passes nothing, search passes `true`).

- [ ] **Step 3: Delete the obsolete components**

Run: `git rm ui/src/lib/components/ScopeRail.svelte ui/src/lib/components/SearchPalette.svelte ui/src/lib/components/SearchPalette.test.ts` (jj: just delete the files; `jj` auto-tracks).
Then `rg -n "ScopeRail|SearchPalette" ui/src` → expect no remaining imports.

- [ ] **Step 4: Run the full UI test suite + build**

Run: `pnpm test && pnpm build`
Expected: all tests pass; build succeeds.

- [ ] **Step 5: Commit**

`feat(ui): wire routes to redesigned shell; remove eg-* components (engram-kco)`.

---

## Task 11: Vendored-asset rebuild + final verification

**Files:**

- Modify (generated): `internal/webauth/static/**`

- [ ] **Step 1: Rebuild the vendored SPA bundle**

Run: `task ui:build`
Expected: `internal/webauth/static/` regenerated (pnpm install + pnpm build + copy). The `//go:embed all:static` directive (engram-0y0) already embeds `_app/*`.

- [ ] **Step 2: Verify Go embed + serve unchanged**

Run: `go test ./internal/webauth/`
Expected: PASS (`TestStaticHandlerServesAppAssets` still serves a hashed `_app` asset).

- [ ] **Step 3: Confirm no stale `eg-*` / inline tokens remain**

Run: `rg -n "eg-row|eg-surface|eg-muted|eg-panel|eg-label|eg-error" ui/src` → expect no matches.
Run: `rg -n "var\(--surface\)|var\(--accent\)" ui/src` → expect no matches outside `app.css` token defs.

- [ ] **Step 4: Manual verification against real data**

Build + run the server locally (or deploy), open `/ui/observe`: scope rail no longer overflows (chips truncate, counts aligned); rows scan cleanly with badges + recency + no `GOTCHA (` redundancy; detail shows the full scope verbatim; panes resize; ⌘K opens the palette; Empty/loading states render.

- [ ] **Step 5: Commit**

`chore(ui): rebuild vendored console bundle (engram-kco)`.

---

## Sequencing & notes

- Tasks 1-4 are foundation (tokens, helpers, ScopeChip) and unblock everything.
- Tasks 5-9 are independent component rebuilds (can parallelize across workers once Task 1 lands).
- Task 10 integrates; Task 11 verifies + rebuilds the embed.
- **Do not regress** the svelte-query v6 runes pattern or the URL-as-state contract in `observe/+page.svelte` — only markup/imports change there.
- Every `Resizable`/`Pagination`/`Command`/`Empty`/`ScrollArea`/`Sonner` usage MUST be reconciled against its `shadcn-svelte.com/docs/components/<name>.md` page (bits-ui 2.x API) — the prop names in this plan are the intended shape, not a guarantee of the exact registry signature.
- **Model routing (Rule 5):** pure-formatter tasks (Task 2 `scope`, Task 3 `time`, and the `summary` helper in Task 7) → `model:haiku`; all Svelte-component and integration tasks (1, 4, 5, 6, 7-components, 8, 9, 10, 11) → `model:sonnet`. `plan-to-beads` should stamp these `model:*` labels on the materialized child beads.
- **Deferred within the redesign:** the spec's narrow-viewport "detail collapses into a `Sheet`" fallback is NOT in these tasks — `Resizable` covers the desktop operator-console case (the only deployed use). File a follow-up bead to add the `Sheet` fallback + a viewport breakpoint if small-screen use materializes. Search/discovery route rewrites (Task 10 Step 2) intentionally mirror the fully-shown observe wiring rather than repeating it verbatim.
<!-- adr-capture: sha256=57125085b08c21e8; session=cli; ts=2026-06-12T21:43:22Z; adrs=engram-lzz -->
