<!--
SPDX-License-Identifier: Apache-2.0
-->

# Memory Display + Auto-Summary UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface the server's auto-summary (`summary` + `summary_source`) in the engram web console — summary-forward rows, a segmented `Summary/Content/Meta` detail panel, and safely-rendered markdown content.

**Architecture:** The Connect wire already returns `summary` (real digest or head-truncated content) + `summary_source` on `full=false` recall, and full content via `getMemory`. We (1) stop forcing `full: true` on list/search so rows read the real `summary`; (2) rebuild `MemoryRow` summary-forward; (3) rebuild `MemoryDetail` as bits-ui `Tabs`; (4) add a sanitized markdown renderer (`marked` + `DOMPurify`) for the Content tab.

**Tech Stack:** Svelte 5.56 (runes), SvelteKit static adapter, Tailwind 4, bits-ui (`ui/tabs`), `@tanstack/svelte-query` v6, `marked`, `dompurify`, vitest 4 (happy-dom). Spec: `docs/superpowers/specs/2026-06-26-memory-display-summary-ux-design.md`. Bead: **engram-gyo7**.

**Cross-cutting constraints (read before starting):**

- **CI gate is `ui vendored-asset drift`**, not vitest/svelte-check. After *any* `ui/` source change, the vendored SPA in `internal/webauth/static` must be rebuilt (`task ui:build`) and committed or CI fails. Task 7 does this once at the end.
- **bits-ui under happy-dom:** only the active tab's content mounts; tab-switch clicks don't work in tests. Assert the **default** tab via render-reflection only. Markdown/XSS coverage lives in the pure `markdown.test.ts`.
- **Tests have no server**, so summary-shaped fallback does not apply — mocks must set `summary` explicitly via `create(MemorySchema, { summary: '…', summarySource: 'auto' })`.
- VCS is jj (colocated). Commit steps use `jj commit -m` per `references/vcs-preamble.md`. Conventional Commits; PR title validated in CI.

---

### Task 1: Add `marked` + `dompurify` dependencies

**Model:** haiku (mechanical)

**Files:**

- Modify: `ui/package.json` (dependencies)
- Modify: `ui/pnpm-lock.yaml` (generated)

- [ ] **Step 1: Add the deps**

Run (from `ui/`):

```bash
pnpm add marked dompurify
```

`marked` and `dompurify` (v3+) both ship their own TypeScript types — no `@types/*` needed. The repo's pnpm supply-chain gate (`minimumReleaseAge`) only blocks freshly-published versions; both are mature, so current stable resolves cleanly.

- [ ] **Step 2: Verify they landed in `dependencies` (not dev)**

Run: `jq '.dependencies | {marked, dompurify}' ui/package.json`
Expected: both keys present with caret-pinned versions (e.g. `"marked": "^16.x"`, `"dompurify": "^3.x"`).

- [ ] **Step 3: Verify the SPA still builds with the new deps**

Run (from `ui/`): `pnpm build`
Expected: build completes, no resolution errors.

- [ ] **Step 4: Commit**

```bash
jj commit -m "build(ui): add marked + dompurify for memory markdown rendering (engram-gyo7)"
```

---

### Task 2: Add the `--code-bg` design token

**Model:** haiku (mechanical)

**Files:**

- Modify: `ui/src/app.css:5-17` (`:root`), `:18-29` (`.dark`), `:31-44` (`@theme inline`)

- [ ] **Step 1: Add `--code-bg` to `:root`**

In `ui/src/app.css`, change the `:root` `--cat-discovery` line (line 16) from:

```css
  --cat-discovery: #0d9488;
```

to:

```css
  --cat-discovery: #0d9488;
  --code-bg: #f6f8fa;
```

- [ ] **Step 2: Add `--code-bg` to `.dark`**

Change the `.dark` `--cat-discovery` line (line 28) from:

```css
  --cat-discovery: #2dd4bf;
```

to:

```css
  --cat-discovery: #2dd4bf;
  --code-bg: #1b2230;
```

- [ ] **Step 3: Map it in `@theme inline`**

Change the `--radius-lg` line (line 43) from:

```css
  --radius-lg: var(--radius);
```

to:

```css
  --color-code-bg: var(--code-bg);
  --radius-lg: var(--radius);
```

- [ ] **Step 4: Verify the token resolves in a build**

Run (from `ui/`): `pnpm build`
Expected: build succeeds (Tailwind 4 accepts the new `@theme inline` mapping).

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(ui): add --code-bg token for markdown code blocks (engram-gyo7)"
```

---

### Task 3: Sanitized markdown renderer

**Model:** sonnet (security-sensitive logic + test design)

**Files:**

- Create: `ui/src/lib/markdown.ts`
- Test: `ui/src/lib/markdown.test.ts`

- [ ] **Step 1: Write the failing test**

Create `ui/src/lib/markdown.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { renderMarkdown } from './markdown';

describe('renderMarkdown', () => {
  it('renders benign markdown structure', () => {
    const html = renderMarkdown('# h\n\n- a\n- b\n\n`code` and **bold**');
    expect(html).toContain('<ul>');
    expect(html).toContain('<li>a</li>');
    expect(html).toContain('<code>code</code>');
    expect(html).toContain('<strong>bold</strong>');
  });

  it('keeps fenced code blocks', () => {
    const html = renderMarkdown('```\nx := 1\n```');
    expect(html).toContain('<pre>');
    expect(html).toContain('x := 1');
  });

  it('strips script tags', () => {
    expect(renderMarkdown('<script>alert(1)</script>')).not.toContain('<script');
  });

  it('strips event-handler attributes and dangerous img', () => {
    const html = renderMarkdown('<img src=x onerror="alert(1)">');
    expect(html).not.toContain('onerror');
  });

  it('drops javascript: links but keeps https links with safe rel/target', () => {
    const js = renderMarkdown('[x](javascript:alert(1))');
    expect(js).not.toContain('javascript:');
    const ok = renderMarkdown('[x](https://example.com)');
    expect(ok).toContain('href="https://example.com"');
    expect(ok).toContain('rel="noopener noreferrer"');
    expect(ok).toContain('target="_blank"');
  });

  it('returns empty string for empty input', () => {
    expect(renderMarkdown('')).toBe('');
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run (from `ui/`): `pnpm test src/lib/markdown.test.ts`
Expected: FAIL — `Failed to resolve import './markdown'`.

- [ ] **Step 3: Implement the renderer**

Create `ui/src/lib/markdown.ts`:

```ts
// SPDX-License-Identifier: Apache-2.0
import { marked } from 'marked';
import DOMPurify from 'dompurify';

// Memory content is caller-authored and `shared` records are cross-actor
// readable, so the only safe path is: marked (no HTML passthrough trust) ->
// DOMPurify tight allowlist -> {@html}. marked.parse() is synchronous unless
// async mode is set, so the string return type holds.
marked.use({ gfm: true, breaks: true });

const ALLOWED_TAGS = [
  'p', 'br', 'strong', 'em', 'del', 'code', 'pre', 'blockquote',
  'ul', 'ol', 'li', 'a', 'h1', 'h2', 'h3', 'h4', 'hr',
  'table', 'thead', 'tbody', 'tr', 'th', 'td'
];
// Deliberate delta from spec §3.4 (['href','title']): `target`/`rel` are
// allow-listed so the afterSanitizeAttributes hook's link-hardening survives
// across all DOMPurify v3 patch levels. No security regression — marked never
// emits them from input, and they are inert attributes.
const ALLOWED_ATTR = ['href', 'title', 'target', 'rel'];
const SAFE_SCHEME = /^(https?|mailto):/i;

let hookInstalled = false;
function installLinkHook(): void {
  if (hookInstalled) return;
  hookInstalled = true;
  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (!(node instanceof Element) || !node.hasAttribute('href')) return;
    const href = node.getAttribute('href') ?? '';
    if (!SAFE_SCHEME.test(href)) {
      node.removeAttribute('href');
      return;
    }
    if (node.tagName === 'A') {
      node.setAttribute('target', '_blank');
      node.setAttribute('rel', 'noopener noreferrer');
    }
  });
}

// renderMarkdown turns caller-authored markdown into sanitized HTML safe for
// {@html}. Returns "" for empty input.
export function renderMarkdown(src: string): string {
  if (!src) return '';
  installLinkHook();
  const html = marked.parse(src, { async: false }) as string;
  return DOMPurify.sanitize(html, { ALLOWED_TAGS, ALLOWED_ATTR });
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run (from `ui/`): `pnpm test src/lib/markdown.test.ts`
Expected: PASS (6 tests). DOMPurify resolves the global `window` happy-dom provides via `vitest-setup.ts` (`environment: 'happy-dom'`). If `DOMPurify.sanitize` is undefined, the env lacks `window` — import as `import DOMPurify from 'dompurify'` is correct for the browser build; do not switch to `isomorphic-dompurify`.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(ui): add sanitized markdown renderer (marked + DOMPurify) (engram-gyo7)"
```

---

### Task 4: Summary-forward `MemoryRow` (R2)

**Model:** sonnet (component + test authoring)

**Files:**

- Modify: `ui/src/lib/components/MemoryRow.svelte` (full rewrite)
- Test: `ui/src/lib/components/MemoryRow.test.ts` (update)

- [ ] **Step 1: Update the test for the new contract**

Replace `ui/src/lib/components/MemoryRow.test.ts` with:

```ts
import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import MemoryRow from './MemoryRow.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const autoMem = create(MemorySchema, {
  id: '42',
  summary: 'GOTCHA (path must match) upstream routing rule',
  summarySource: 'auto',
  category: 'gotcha',
  tags: ['mcp', 'routing']
});

describe('MemoryRow', () => {
  it('renders the real summary (category prefix stripped) and tags', () => {
    render(MemoryRow, { props: { memory: autoMem, selected: false, onselect: () => {} } });
    expect(screen.getByText(/path must match/)).toBeInTheDocument();
    expect(screen.queryByText(/GOTCHA \(/)).not.toBeInTheDocument(); // cosmetic strip
    expect(screen.getByText('gotcha')).toBeInTheDocument();
    expect(screen.getByText('mcp')).toBeInTheDocument();
  });

  it('shows the auto provenance glyph only for summary_source=auto', () => {
    const { rerender } = render(MemoryRow, { props: { memory: autoMem, selected: false, onselect: () => {} } });
    expect(screen.getByLabelText('auto-generated summary')).toBeInTheDocument();
    const clientMem = create(MemorySchema, { id: '7', summary: 'authored line', summarySource: 'client', category: 'decision' });
    rerender({ memory: clientMem, selected: false, onselect: () => {} });
    expect(screen.queryByLabelText('auto-generated summary')).not.toBeInTheDocument();
  });

  it('fires onselect with the memory id when clicked', async () => {
    const user = userEvent.setup();
    const onselect = vi.fn();
    render(MemoryRow, { props: { memory: autoMem, selected: false, onselect } });
    await user.click(screen.getByRole('button'));
    expect(onselect).toHaveBeenCalledWith('42');
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run (from `ui/`): `pnpm test src/lib/components/MemoryRow.test.ts`
Expected: FAIL — the current row reads `memory.content` (empty here) and has no `auto-generated summary` label.

- [ ] **Step 3: Rewrite the component (R2 layout)**

Replace `ui/src/lib/components/MemoryRow.svelte` with:

```svelte
<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { stripCategoryPrefix } from '$lib/summary';
  import { relativeTime } from '$lib/time';
  import ScopeChip from './ScopeChip.svelte';
  let { memory, selected, showScope = false, onselect }: { memory: Memory; selected: boolean; showScope?: boolean; onselect: (id: string) => void } = $props();
  // summary is server-guaranteed non-empty on list/search; stripCategoryPrefix
  // is a cosmetic no-op on real summaries, meaningful only for truncation
  // previews that begin with the category token.
  const summary = $derived(stripCategoryPrefix(memory.summary, memory.category));
  const isAuto = $derived(memory.summarySource === 'auto');
  const when = $derived(memory.createdAt ? relativeTime(timestampDate(memory.createdAt)) : '');
  const shownTags = $derived(memory.tags.slice(0, 3));
  const overflow = $derived(Math.max(0, memory.tags.length - 3));
</script>

<button
  type="button"
  onclick={() => onselect(memory.id)}
  style="--c:var(--cat-{memory.category})"
  class={'relative w-full text-left pl-3 pr-3 py-2 border-b border-border flex flex-col gap-1 hover:bg-accent ' + (selected ? 'bg-accent' : '')}
>
  <span class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r" style="background:var(--c)"></span>
  <div class="flex items-center gap-2 min-w-0">
    <span class="truncate flex-1 text-[13px]">{summary}</span>
    {#if isAuto}<span aria-label="auto-generated summary" title="auto-generated summary" class="shrink-0 text-[10px] text-primary">✦</span>{/if}
  </div>
  <div class="flex items-center gap-2 text-[11px] text-muted-foreground min-w-0">
    <span class="font-medium shrink-0" style="color:var(--c)">{memory.category}</span>
    <span class="shrink-0">·</span>
    <span class="tabular-nums shrink-0">{when}</span>
    {#if showScope && memory.scope}<span class="shrink-0"><ScopeChip scope={memory.scope} /></span>{/if}
    {#each shownTags as t (t)}<span class="shrink-0 px-1 rounded bg-muted font-mono text-[10.5px]">{t}</span>{/each}
    {#if overflow > 0}<span class="shrink-0">+{overflow}</span>{/if}
  </div>
</button>
```

- [ ] **Step 4: Run the test to verify it passes**

Run (from `ui/`): `pnpm test src/lib/components/MemoryRow.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(ui): summary-forward memory rows with provenance glyph (engram-gyo7)"
```

---

### Task 5: Segmented `MemoryDetail` (Summary / Content / Meta)

**Model:** sonnet (component + test authoring)

**Files:**

- Modify: `ui/src/lib/components/MemoryDetail.svelte` (full rewrite)
- Test: `ui/src/lib/components/MemoryDetail.test.ts` (update)

- [ ] **Step 1: Update the test (tabbed structure breaks the old assertions)**

Replace `ui/src/lib/components/MemoryDetail.test.ts` with:

```ts
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import MemoryDetail from './MemoryDetail.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const withSummary = create(MemorySchema, {
  id: '1', content: 'full **body** here', summary: 'terse digest line',
  summarySource: 'auto', category: 'gotcha',
  scope: 'repo:github.com/fzymgc-house/selfhosted-cluster',
  source: 'agent-inferred', actor: 'sean', visibility: 'private', tags: ['mcp']
});
const noSummary = create(MemorySchema, {
  id: '2', content: 'only content here', summary: '', summarySource: '',
  category: 'decision', scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private'
});

describe('MemoryDetail', () => {
  it('defaults to the Summary tab and shows summary + auto provenance', () => {
    render(MemoryDetail, { props: { memory: withSummary, loading: false, error: null } });
    expect(screen.getByText('terse digest line')).toBeInTheDocument();
    expect(screen.getByText(/auto/i)).toBeInTheDocument();
  });
  it('falls through to Content (rendered markdown) when there is no summary', () => {
    render(MemoryDetail, { props: { memory: noSummary, loading: false, error: null } });
    expect(screen.getByText('only content here')).toBeInTheDocument();
  });
  it('prompts to select when nothing is chosen', () => {
    render(MemoryDetail, { props: { memory: undefined, loading: false, error: null } });
    expect(screen.getByText(/select a record/i)).toBeInTheDocument();
  });
  it('shows a loading indicator while fetching', () => {
    render(MemoryDetail, { props: { memory: undefined, loading: true, error: null } });
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });
  it('shows a not-found message for a NotFound error', () => {
    render(MemoryDetail, { props: { memory: undefined, loading: false, error: new ConnectError('missing', Code.NotFound) } });
    expect(screen.getByText(/record not found/i)).toBeInTheDocument();
  });
  it('shows a generic failure message for a non-NotFound error', () => {
    render(MemoryDetail, { props: { memory: undefined, loading: false, error: new Error('boom') } });
    expect(screen.getByText(/failed to load record/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run (from `ui/`): `pnpm test src/lib/components/MemoryDetail.test.ts`
Expected: FAIL — old component renders `content`/`scope` directly; new assertions (`terse digest line`, tab fallthrough) are absent.

- [ ] **Step 3: Rewrite the component**

Replace `ui/src/lib/components/MemoryDetail.svelte` with:

```svelte
<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { ConnectError, Code } from '@connectrpc/connect';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { ScrollArea } from '$lib/components/ui/scroll-area';
  import * as Tabs from '$lib/components/ui/tabs';
  import { toast } from 'svelte-sonner';
  import { relativeTime, fullTimestamp } from '$lib/time';
  import { renderMarkdown } from '$lib/markdown';
  import CopyIcon from '@lucide/svelte/icons/copy';
  let { memory, loading, error }: { memory: Memory | undefined; loading: boolean; error: unknown } = $props();
  const notFound = $derived(error instanceof ConnectError && error.code === Code.NotFound);
  const created = $derived(memory?.createdAt ? timestampDate(memory.createdAt) : undefined);
  const hasSummary = $derived(!!memory?.summary?.trim());
  const defaultTab = $derived(hasSummary ? 'summary' : 'content');
  const bodyHtml = $derived(memory ? renderMarkdown(memory.content) : '');
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
      <span class="cat-dot" style="background:var(--cat-{memory.category})"></span>
      <Badge variant="outline" class="text-[10px] uppercase" style="color:var(--cat-{memory.category})">{memory.category}</Badge>
      {#if created}<span class="text-[11px] text-muted-foreground" title={fullTimestamp(created)}>{relativeTime(created)}</span>{/if}
      <Button variant="outline" size="sm" class="ml-auto" aria-label="copy content" onclick={copy}><CopyIcon data-icon="inline-start" /> copy</Button>
    </div>
    <Tabs.Root value={defaultTab} class="flex-1 flex flex-col min-h-0">
      <Tabs.List class="mx-3 mt-2">
        <Tabs.Trigger value="summary">Summary</Tabs.Trigger>
        <Tabs.Trigger value="content">Content</Tabs.Trigger>
        <Tabs.Trigger value="meta">Meta</Tabs.Trigger>
      </Tabs.List>

      <Tabs.Content value="summary" class="p-3 min-h-0">
        {#if hasSummary}
          <div class="flex items-center justify-between mb-2">
            <span class="text-[9.5px] uppercase tracking-wide text-muted-foreground font-semibold">Summary</span>
            {#if memory.summarySource === 'auto'}
              <span class="inline-flex items-center gap-1 text-[10px] text-primary border border-primary/45 rounded-full px-2 py-0.5 bg-primary/10">✦ auto</span>
            {:else if memory.summarySource === 'client'}
              <span class="inline-flex items-center text-[10px] text-muted-foreground border border-border rounded-full px-2 py-0.5">authored</span>
            {/if}
          </div>
          <div class="text-[13.5px] leading-relaxed">{memory.summary}</div>
        {:else}
          <div class="text-[12px] text-muted-foreground">No summary — see Content.</div>
        {/if}
      </Tabs.Content>

      <Tabs.Content value="content" class="min-h-0 flex flex-col">
        <ScrollArea class="flex-1 min-h-0"><div class="markdown-body p-3 text-[13px] leading-relaxed">{@html bodyHtml}</div></ScrollArea>
      </Tabs.Content>

      <Tabs.Content value="meta" class="p-3 flex flex-col gap-2 min-h-0">
        <div class="text-[11.5px] font-mono break-all" title={memory.scope}>{memory.scope}</div>
        <div class="flex gap-1.5 flex-wrap text-[10.5px]">
          <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">by</span> {memory.actor}</span>
          <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">src</span> {memory.source}</span>
          <span class="border border-border rounded px-1.5 py-0.5"><span class="text-muted-foreground">vis</span> {memory.visibility}</span>
        </div>
        <div class="flex gap-1.5 flex-wrap">
          {#each memory.tags as t (t)}<span class="px-1.5 rounded bg-muted font-mono text-[10.5px]">{t}</span>{/each}
        </div>
      </Tabs.Content>
    </Tabs.Root>
  {/if}
</div>

<style>
  /* {@html} output can't take Tailwind utilities, so style the rendered
     markdown via :global on the wrapper. Mirrors the visual-companion mockup. */
  .markdown-body :global(h1),
  .markdown-body :global(h2),
  .markdown-body :global(h3),
  .markdown-body :global(h4) { font-weight: 650; margin: 0.9em 0 0.4em; }
  .markdown-body :global(h3) { font-size: 13px; }
  .markdown-body :global(p) { margin: 0 0 0.7em; }
  .markdown-body :global(ul),
  .markdown-body :global(ol) { margin: 0 0 0.7em; padding-left: 1.3em; }
  .markdown-body :global(li) { margin: 0.2em 0; }
  .markdown-body :global(strong) { font-weight: 650; color: var(--foreground); }
  .markdown-body :global(code) { font-family: ui-monospace, Menlo, monospace; font-size: 11.5px; background: var(--accent); border-radius: 4px; padding: 1px 5px; }
  .markdown-body :global(pre) { background: var(--code-bg); border: 1px solid var(--border); border-radius: 8px; padding: 10px 11px; overflow: auto; margin: 0 0 0.7em; }
  .markdown-body :global(pre code) { background: none; padding: 0; font-size: 11.5px; line-height: 1.5; }
  .markdown-body :global(a) { color: var(--primary); text-decoration: underline; text-underline-offset: 2px; }
  .markdown-body :global(blockquote) { border-left: 3px solid var(--border); margin: 0 0 0.7em; padding-left: 0.8em; color: var(--muted-foreground); }
</style>
```

- [ ] **Step 4: Run the test to verify it passes**

Run (from `ui/`): `pnpm test src/lib/components/MemoryDetail.test.ts`
Expected: PASS (6 tests). The Summary and Content default-tab cases assert via the default `value`; Meta is not asserted (would need a click, unsupported under happy-dom).

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(ui): segmented memory detail (Summary/Content/Meta) with markdown body (engram-gyo7)"
```

---

### Task 6: Stop forcing `full: true` on list/search queries

**Model:** haiku (mechanical, 3 one-line deletions)

**Files:**

- Modify: `ui/src/routes/observe/+page.svelte:27`
- Modify: `ui/src/routes/search/+page.svelte:14`
- Modify: `ui/src/routes/+page.svelte:15`

- [ ] **Step 1: observe — drop `full: true`**

In `ui/src/routes/observe/+page.svelte` line 27, change:

```ts
      queryFn: () => engram.listMemories({ scope: pp.scope, limit: BigInt(PAGE_LIMIT), offset: BigInt(pp.offset), categories: pp.categories, visibility: pp.visibility, full: true }),
```

to (remove `, full: true`):

```ts
      queryFn: () => engram.listMemories({ scope: pp.scope, limit: BigInt(PAGE_LIMIT), offset: BigInt(pp.offset), categories: pp.categories, visibility: pp.visibility }),
```

- [ ] **Step 2: search — drop `full: true`**

In `ui/src/routes/search/+page.svelte` line 14, change:

```ts
    return { queryKey: ['searchMemories', query, scope], queryFn: () => engram.searchMemories({ query, scope, k: 50n, full: true }), enabled: !!query };
```

to:

```ts
    return { queryKey: ['searchMemories', query, scope], queryFn: () => engram.searchMemories({ query, scope, k: 50n }), enabled: !!query };
```

- [ ] **Step 3: root — drop `full: true`**

In `ui/src/routes/+page.svelte` line 15, change:

```ts
    queryFn: () => engram.listMemories({ scope: '', limit: BigInt(PAGE_LIMIT), offset: 0n, categories: [], visibility: '', full: true })
```

to:

```ts
    queryFn: () => engram.listMemories({ scope: '', limit: BigInt(PAGE_LIMIT), offset: 0n, categories: [], visibility: '' })
```

- [ ] **Step 4: Type-check + full test pass**

Run (from `ui/`): `pnpm check && pnpm test`
Expected: `svelte-check` clean; all vitest suites pass (markdown, MemoryRow, MemoryDetail, and untouched suites). The detail panel still uses `getMemory` (full), so copy + Content tab are unaffected.

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(ui): fetch summary-shaped list/search; rows now show real summaries (engram-gyo7)"
```

---

### Task 7: Rebuild + re-vendor the SPA (CI drift gate)

**Model:** haiku (build + vendor + commit)

**Files:**

- Modify: `internal/webauth/static/**` (regenerated vendored bundle)

- [ ] **Step 1: Rebuild and vendor**

Run (from repo root): `task ui:build`
Expected: `pnpm install --frozen-lockfile`, `pnpm build`, then the bundle is copied into `internal/webauth/static/`.

- [ ] **Step 2: Confirm the vendored tree changed and embeds cleanly**

Run: `jj st` — expect modified/added files under `internal/webauth/static/`.
Run: `go build ./...`
Expected: build succeeds (`//go:embed all:static` picks up the refreshed `_app/` assets).

- [ ] **Step 3: Run the Go test + drift-sensitive gates locally**

Run (from repo root): `task test`
Expected: passes. (The `ui vendored-asset drift` CI job rebuilds and diffs `internal/webauth/static`; committing the fresh build keeps it green.)

- [ ] **Step 4: Commit**

```bash
jj commit -m "chore(ui): re-vendor SPA after memory display redesign (engram-gyo7)"
```

---

## Verification (whole-feature)

- [ ] `cd ui && pnpm check && pnpm test` — svelte-check clean, all suites green.
- [ ] `task test` from repo root — Go suite + lint green; no `internal/webauth/static` drift.
- [ ] Manual (optional): `cd ui && pnpm dev`, open `/observe` on a scope with both summarized and short records — rows show real summaries with `✦` on auto ones; detail defaults to Summary with the provenance chip; Content tab renders markdown; a record with no summary opens straight to Content.
<!-- adr-capture: sha256=ec26e9c4b6b28c78; session=cli; ts=2026-06-27T00:58:44Z; adrs=engram-3nas -->
