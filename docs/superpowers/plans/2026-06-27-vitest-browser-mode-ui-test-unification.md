<!--
SPDX-License-Identifier: Apache-2.0
-->

# Vitest 4 Browser-Mode UI Test Unification — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the per-file `// @vitest-environment` emulator split with a two-tier vitest config — a fast node tier + a real-Chromium browser tier — so the DOMPurify sanitizer test and all 7 bits-ui component tests run in one production-equivalent DOM, and `jsdom`/`happy-dom` leave the dependency tree.

**Architecture:** One `ui/vite.config.ts` declares `test.projects[]`: a `node` project (glob `*.test.ts`) for the 6 logic tests + `app.css`, and a `browser` project (glob `*.browser.test.ts`, Playwright/Chromium) for the sanitizer + components. Component tests are rewritten from `@testing-library/svelte` + `userEvent` onto `vitest-browser-svelte`'s async retriable locators. A new required `ui-test` CI job runs both tiers.

**Tech Stack:** vitest 4.1.9, `@vitest/browser` + `@vitest/browser-playwright`, `playwright` (Chromium), `vitest-browser-svelte`, `@testing-library/jest-dom` (matchers via `expect.element`), SvelteKit 2 / Svelte 5, pnpm 11.

**Spec:** `docs/superpowers/specs/2026-06-27-vitest-browser-mode-ui-test-unification-design.md`
**Design bead:** engram-utdv

---

## Pre-flight reference: the API shift (read once before Task 2+)

Every component-test rewrite follows the **same mechanical transform** (grounded via context7 `/vitest-community/vitest-browser-svelte`):

| `@testing-library/svelte` (old) | `vitest-browser-svelte` (new) |
|---|---|
| `import { render, screen } from '@testing-library/svelte'` | `import { render } from 'vitest-browser-svelte'` |
| `import userEvent from '@testing-library/user-event'` | *(deleted — no userEvent)* |
| `import { within } from '@testing-library/svelte'` | *(deleted — use locator chaining)* |
| `render(C, { props: {...} })` | `const screen = await render(C, {...})` *(props passed **directly**, not under `props:`)* |
| `render(C)` | `const screen = await render(C)` |
| `const { rerender } = render(...)` | `const screen = await render(...)` then `await screen.rerender({...})` *(direct props, async)* |
| `screen.getByText('x')` *(sync)* | `screen.getByText('x')` *(returns a locator; assert via `await expect.element(...)`)* |
| `screen.getByLabelText('x')` | `screen.getByLabel('x')` |
| `screen.getByPlaceholderText(/x/)` | `screen.getByPlaceholder(/x/)` |
| `screen.getByTitle / getByTestId / getByRole` | **same names** |
| `screen.getAllByRole(...)` → array, `arr[0]` | `screen.getByRole(...).first()` (locator) or `.nth(0)` |
| `within(el).getByText('x')` | `el.getByText('x')` *(locators chain directly)* |
| `expect(screen.getByText('x')).toBeInTheDocument()` | `await expect.element(screen.getByText('x')).toBeInTheDocument()` |
| `expect(screen.queryByText('x')).not.toBeInTheDocument()` | `await expect.element(screen.getByText('x')).not.toBeInTheDocument()` |
| `expect(el).toHaveTextContent('x')` | `await expect.element(el).toHaveTextContent('x')` |
| `const user = userEvent.setup(); await user.click(el)` | `await el.click()` *(el is a locator)* |

**jest-dom matchers (`toBeInTheDocument`, `toHaveTextContent`) are retained** — they work on locators through `expect.element` (context7 shows `.toHaveStyle()` used this way). They are wired via a browser `setupFiles` (Task 1).

**Key behavioral note:** browser-mode locators are **retriable** — `await expect.element(locator)` polls until the assertion passes or times out. This is why every assertion is `await`ed. A bare `screen.getByText(...)` does NOT throw on absence (it returns a lazy locator); absence is asserted via `await expect.element(...).not.toBeInTheDocument()`.

**Mid-migration caveat:** Task 1 drops the `svelteTesting()` vite plugin while the un-migrated `*.test.ts` component files still import `@testing-library/svelte` (its dep isn't removed until Task 10). This is latent, not live — `sveltekit()` handles `.svelte` compilation (the dropped plugin only added auto-cleanup), and the plan never runs the **node** tier until Task 11, by which point every component file has been renamed to `*.browser.test.ts` and excluded from the node glob. **Do not run a bare `pnpm test` (both tiers) between Tasks 1 and 11** — run the per-file `--project browser` commands each task specifies. The full-suite run is Task 12.

---

## Task 1: Spike-gate — two-project config with SvelteKit, one component test green

**This is a GO/NO-GO gate.** The engram-s2ao spike proved the `sveltekit()` Vite plugin stalls browser-session startup (60s timeout) when present. The whole component migration rests on resolving this. Do NOT proceed to Task 2+ until a SvelteKit-compiled component renders in browser mode.

**Files:**
- Modify: `ui/package.json` (add browser-mode devDeps)
- Modify: `ui/vite.config.ts` (two-project config)
- Create: `ui/vitest-setup.browser.ts` (jest-dom matchers for the browser tier)
- Create (temporary probe): `ui/src/lib/components/ScopeChip.browser.test.ts`

- [ ] **Step 1: Install the browser-mode devDependencies**

Run (from `ui/`):
```
pnpm add -D @vitest/browser@4.1.9 @vitest/browser-playwright@4.1.9 playwright vitest-browser-svelte
pnpm exec playwright install chromium
```
Expected: 4 devDeps added; Chromium downloaded (~190MB headless shell, may be cached).

- [ ] **Step 2: Create the browser-tier setup file**

Create `ui/vitest-setup.browser.ts`:
```ts
// Browser-tier setup: jest-dom matchers attach to vitest's expect, usable on
// locators via expect.element(...). The node tier's localStorage stub is NOT
// needed here — a real browser provides Storage.
import '@testing-library/jest-dom/vitest';
```

- [ ] **Step 3: Rewrite `ui/vite.config.ts` as a two-project config**

Replace the entire file with:
```ts
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { playwright } from '@vitest/browser-playwright';
// vitest 4 dropped the `test` key from vite's UserConfig type; import
// defineConfig from vitest/config so the inline test block type-checks.
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()],
  test: {
    projects: [
      {
        extends: true,
        test: {
          name: 'node',
          include: ['src/**/*.test.ts'],
          exclude: ['src/**/*.browser.test.ts'],
          environment: 'happy-dom',
          setupFiles: ['./vitest-setup.ts'],
          environmentOptions: { happyDOM: { url: 'http://localhost/' } }
        }
      },
      {
        extends: true,
        test: {
          name: { label: 'browser', color: 'green' },
          include: ['src/**/*.browser.test.ts'],
          setupFiles: ['./vitest-setup.browser.ts'],
          browser: {
            enabled: true,
            provider: playwright(),
            headless: true,
            instances: [{ browser: 'chromium' }]
          }
        }
      }
    ]
  }
});
```
Note: the node tier keeps `happy-dom` for now (the `environment: 'node'` + drop-happy-dom decision is deferred to Task 11, gated on the logic tests passing). `svelteTesting()` is intentionally dropped from `plugins` — its removal pairs with the `@testing-library/svelte` removal in Task 10.

- [ ] **Step 4: Write a throwaway probe test that renders a SvelteKit-compiled component in the browser**

Create `ui/src/lib/components/ScopeChip.browser.test.ts`:
```ts
import { render } from 'vitest-browser-svelte';
import { describe, it, expect } from 'vitest';
import ScopeChip from './ScopeChip.svelte';

describe('browser-mode smoke (spike-gate)', () => {
  it('renders a SvelteKit-compiled component in real Chromium', async () => {
    const screen = await render(ScopeChip, { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster' });
    await expect.element(screen.getByText('selfhosted-cluster')).toBeInTheDocument();
  });
});
```

- [ ] **Step 5: Run the probe — this is the gate**

Run (from `ui/`): `pnpm exec vitest run --project browser`
Expected: **1 passed.** If it hangs ~60s then reports "no tests" / "Failed to connect to the browser session", the SvelteKit-plugin stall is reproducing. Resolution levers, in order:
  1. Add `optimizeDeps: { include: ['bits-ui', 'svelte', '@testing-library/jest-dom'] }` to the config root.
  2. Add `test.server: { deps: { inline: [/svelte/, /bits-ui/] } }` to the browser project.
  3. Pin `test.browser.headless: true` (already set) and try `test.browser.isolate: false`.
  4. Consult context7 `/vitest-dev/vitest` "browser mode sveltekit" + the SvelteKit `vitest-browser` recipe.

**NO-GO fallback:** if no lever resolves the stall within reasonable effort, STOP. Re-scope to engram-s2ao-only (sanitizer in browser, components stay happy-dom) — file that as a finding on engram-utdv and convert this plan to the sanitizer-only subset (Tasks 1, 2, 10-partial, 12, 13). The remaining tasks assume GO.

- [ ] **Step 6: Delete the throwaway probe**

Run (from `ui/`): `rm src/lib/components/ScopeChip.browser.test.ts`
(ScopeChip gets its real migration in Task 4.)

- [ ] **Step 7: Commit**

```
jj describe -m "test(ui): two-project vitest config + browser-tier spike-gate (engram-utdv)

Stand up node + browser test projects in one vite.config.ts; resolve the
SvelteKit-plugin browser-startup stall found in engram-s2ao. Browser-tier
setup file wires jest-dom matchers for expect.element.

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 2: Migrate the sanitizer test (markdown) to the browser tier

The safest first real migration — no `.svelte`, no component API, just drop the jsdom pragma and rename.

**Files:**
- Rename: `ui/src/lib/markdown.test.ts` → `ui/src/lib/markdown.browser.test.ts`
- Test: the same file

- [ ] **Step 1: Move the file and drop the pragma**

Run (from `ui/`): `jj file move src/lib/markdown.test.ts src/lib/markdown.browser.test.ts` (or `mv` + let jj track).
Then edit the new file to remove lines 1-4 (the `// @vitest-environment jsdom` block + its explanatory comment). The new file head becomes:
```ts
import { describe, it, expect } from 'vitest';
import { renderMarkdown } from './markdown';

describe('renderMarkdown', () => {
```
The 6 `it()` bodies are pure string assertions (`expect(html).toContain(...)`) on the return of `renderMarkdown` — **no DOM queries, no change needed.**

- [ ] **Step 2: Run the sanitizer test in the browser tier**

Run (from `ui/`): `pnpm exec vitest run --project browser src/lib/markdown.browser.test.ts`
Expected: **6 passed.** Critically, the `javascript:`-link and fenced-code assertions pass (they FAIL on happy-dom — that's the bug this whole effort fixes).

- [ ] **Step 3: Commit**

```
jj describe -m "test(ui): run DOMPurify sanitizer tests in real Chromium (engram-utdv)

markdown.test.ts -> markdown.browser.test.ts; drop the jsdom @vitest-environment
pragma. The scheme-guard + fenced-code assertions now run on a real DOM.

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 3: Migrate AppShell (render-only component)

First component migration — render-only, no interaction, establishes the locator pattern.

**Files:**
- Rename: `ui/src/lib/components/AppShell.test.ts` → `AppShell.browser.test.ts`

- [ ] **Step 1: Rename the file**

Run (from `ui/`): `jj file move src/lib/components/AppShell.test.ts src/lib/components/AppShell.browser.test.ts`

- [ ] **Step 2: Rewrite the file contents**

Replace the file with:
```ts
import { render } from 'vitest-browser-svelte';
import { describe, it, expect } from 'vitest';
import AppShell from './AppShell.svelte';

describe('AppShell', () => {
  it('renders nav links and the command trigger', async () => {
    const screen = await render(AppShell);
    await expect.element(screen.getByRole('link', { name: /observe/i })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: /search/i })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: /toggle theme/i })).toBeInTheDocument();
  });

  it('renders the engram brand mark in the header', async () => {
    const screen = await render(AppShell);
    await expect.element(screen.getByRole('img', { name: 'engram' })).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run it**

Run (from `ui/`): `pnpm exec vitest run --project browser src/lib/components/AppShell.browser.test.ts`
Expected: **2 passed.**

- [ ] **Step 4: Commit**

```
jj describe -m "test(ui): migrate AppShell test to browser tier (engram-utdv)

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 4: Migrate ScopeChip (render-only, getByTitle)

**Files:**
- Rename: `ui/src/lib/components/ScopeChip.test.ts` → `ScopeChip.browser.test.ts`

- [ ] **Step 1: Rename the file**

Run (from `ui/`): `jj file move src/lib/components/ScopeChip.test.ts src/lib/components/ScopeChip.browser.test.ts`

- [ ] **Step 2: Rewrite the file contents**

Replace the file with:
```ts
import { render } from 'vitest-browser-svelte';
import { describe, it, expect } from 'vitest';
import ScopeChip from './ScopeChip.svelte';

describe('ScopeChip', () => {
  it('shows repo name prominently and the type badge', async () => {
    const screen = await render(ScopeChip, { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster' });
    await expect.element(screen.getByText('selfhosted-cluster')).toBeInTheDocument();
    await expect.element(screen.getByText('repo')).toBeInTheDocument();
  });
  it('keeps the full scope available (title attr) — never destroyed', async () => {
    const screen = await render(ScopeChip, { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster' });
    await expect.element(screen.getByTitle('repo:github.com/fzymgc-house/selfhosted-cluster')).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run it**

Run (from `ui/`): `pnpm exec vitest run --project browser src/lib/components/ScopeChip.browser.test.ts`
Expected: **2 passed.**

- [ ] **Step 4: Commit**

```
jj describe -m "test(ui): migrate ScopeChip test to browser tier (engram-utdv)

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 5: Migrate MemoryList (render-only, getByTestId, queryByText→absence)

**Files:**
- Rename: `ui/src/lib/components/MemoryList.test.ts` → `MemoryList.browser.test.ts`

- [ ] **Step 1: Rename the file**

Run (from `ui/`): `jj file move src/lib/components/MemoryList.test.ts src/lib/components/MemoryList.browser.test.ts`

- [ ] **Step 2: Rewrite the file contents**

Replace the file with:
```ts
import { render } from 'vitest-browser-svelte';
import { describe, it, expect } from 'vitest';
import MemoryList from './MemoryList.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';

const mem = create(MemorySchema, { id: '1', summary: 'GOTCHA (path must match) upstream', category: 'gotcha', visibility: 'private', tags: ['mcp', 'routing'] });

type Props = {
  memories: Memory[]; total: bigint; loading: boolean; error: unknown; selectedId: string;
  onselect: (id: string) => void; scopeSelected?: boolean;
};

function baseProps(overrides: Partial<Props> = {}): Props {
  return { memories: [], total: 0n, loading: false, error: null, selectedId: '', onselect: () => {}, ...overrides };
}

describe('MemoryList', () => {
  it('renders the category badge and a de-duplicated summary', async () => {
    const screen = await render(MemoryList, baseProps({ memories: [mem], total: 1n }));
    await expect.element(screen.getByText('gotcha')).toBeInTheDocument();
    await expect.element(screen.getByText(/path must match/)).toBeInTheDocument();
    await expect.element(screen.getByText(/GOTCHA \(/)).not.toBeInTheDocument(); // redundant prefix stripped
  });
  it('shows an Empty state when there are no memories', async () => {
    const screen = await render(MemoryList, baseProps());
    await expect.element(screen.getByText(/no memories/i)).toBeInTheDocument();
  });
  it('prompts to select a scope when none is chosen', async () => {
    const screen = await render(MemoryList, baseProps({ scopeSelected: false }));
    await expect.element(screen.getByText(/select a scope/i)).toBeInTheDocument();
    await expect.element(screen.getByText(/no memories/i)).not.toBeInTheDocument();
  });
  it('shows a skeleton when loading', async () => {
    const screen = await render(MemoryList, baseProps({ loading: true }));
    await expect.element(screen.getByTestId('list-loading')).toBeInTheDocument();
  });
  it('shows an error message when the list fails to load', async () => {
    const screen = await render(MemoryList, baseProps({ error: new Error('boom') }));
    await expect.element(screen.getByText(/failed to load/i)).toBeInTheDocument();
    await expect.element(screen.getByText(/no memories/i)).not.toBeInTheDocument();
  });
});
```
Note: `render(MemoryList, baseProps({...}))` — `baseProps()` returns the props object passed **directly** (no `{ props: ... }` wrapper).

- [ ] **Step 3: Run it**

Run (from `ui/`): `pnpm exec vitest run --project browser src/lib/components/MemoryList.browser.test.ts`
Expected: **5 passed.**

- [ ] **Step 4: Commit**

```
jj describe -m "test(ui): migrate MemoryList test to browser tier (engram-utdv)

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 6: Migrate MemoryRow (interaction + rerender)

First interaction migration — `userEvent.click` → locator `.click()`, and `rerender`.

**Files:**
- Rename: `ui/src/lib/components/MemoryRow.test.ts` → `MemoryRow.browser.test.ts`

- [ ] **Step 1: Rename the file**

Run (from `ui/`): `jj file move src/lib/components/MemoryRow.test.ts src/lib/components/MemoryRow.browser.test.ts`

- [ ] **Step 2: Rewrite the file contents**

Replace the file with:
```ts
import { render } from 'vitest-browser-svelte';
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
  it('renders the real summary (category prefix stripped) and tags', async () => {
    const screen = await render(MemoryRow, { memory: autoMem, selected: false, onselect: () => {} });
    await expect.element(screen.getByText(/path must match/)).toBeInTheDocument();
    await expect.element(screen.getByText(/GOTCHA \(/)).not.toBeInTheDocument(); // cosmetic strip
    await expect.element(screen.getByText('gotcha')).toBeInTheDocument();
    await expect.element(screen.getByText('mcp')).toBeInTheDocument();
  });

  it('shows the auto provenance glyph only for summary_source=auto', async () => {
    const screen = await render(MemoryRow, { memory: autoMem, selected: false, onselect: () => {} });
    await expect.element(screen.getByLabel('auto-generated summary')).toBeInTheDocument();
    const clientMem = create(MemorySchema, { id: '7', summary: 'authored line', summarySource: 'client', category: 'decision' });
    await screen.rerender({ memory: clientMem, selected: false, onselect: () => {} });
    await expect.element(screen.getByLabel('auto-generated summary')).not.toBeInTheDocument();
  });

  it('fires onselect with the memory id when clicked', async () => {
    const onselect = vi.fn();
    const screen = await render(MemoryRow, { memory: autoMem, selected: false, onselect });
    await screen.getByRole('button').click();
    expect(onselect).toHaveBeenCalledWith('42');
  });
});
```
Changes: props passed directly; `getByLabelText`→`getByLabel`; `rerender` is now `await screen.rerender(directProps)`; `userEvent.setup()`/`user.click(el)` → `await screen.getByRole('button').click()`.

- [ ] **Step 3: Run it**

Run (from `ui/`): `pnpm exec vitest run --project browser src/lib/components/MemoryRow.browser.test.ts`
Expected: **3 passed.**

- [ ] **Step 4: Commit**

```
jj describe -m "test(ui): migrate MemoryRow test to browser tier (engram-utdv)

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 7: Migrate CommandPalette (cmdk option interaction)

**Files:**
- Rename: `ui/src/lib/components/CommandPalette.test.ts` → `CommandPalette.browser.test.ts`

- [ ] **Step 1: Rename the file**

Run (from `ui/`): `jj file move src/lib/components/CommandPalette.test.ts src/lib/components/CommandPalette.browser.test.ts`

- [ ] **Step 2: Rewrite the file contents**

Replace the file with:
```ts
import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import CommandPalette from './CommandPalette.svelte';

describe('CommandPalette', () => {
  it('renders the search input when open', async () => {
    const screen = await render(CommandPalette, { open: true, onsearch: () => {}, onnavigate: () => {} });
    await expect.element(screen.getByPlaceholder(/search memories/i)).toBeInTheDocument();
  });

  it('fires onsearch with the current query when the search item is selected', async () => {
    const onsearch = vi.fn();
    const screen = await render(CommandPalette, { open: true, onsearch, onnavigate: () => {} });
    // Empty input keeps every item visible (cmdk filtering hides items that
    // don't match the typed query); select the search item via its option role.
    await screen.getByRole('option', { name: /search memories for/i }).click();
    expect(onsearch).toHaveBeenCalledWith('');
  });

  it('fires onnavigate with the target href when a Go-to item is selected', async () => {
    const onnavigate = vi.fn();
    const screen = await render(CommandPalette, { open: true, onsearch: () => {}, onnavigate });
    await screen.getByRole('option', { name: 'Search' }).click();
    expect(onnavigate).toHaveBeenCalledWith(expect.stringContaining('/search'));
  });
});
```
Changes: props direct; `getByPlaceholderText`→`getByPlaceholder`; `userEvent` clicks → locator `.click()`.

- [ ] **Step 3: Run it**

Run (from `ui/`): `pnpm exec vitest run --project browser src/lib/components/CommandPalette.browser.test.ts`
Expected: **3 passed.**

- [ ] **Step 4: Commit**

```
jj describe -m "test(ui): migrate CommandPalette test to browser tier (engram-utdv)

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 8: Migrate ScopesSidebar (getAllByRole, checkbox, retire stale Select comment)

**Files:**
- Rename: `ui/src/lib/components/ScopesSidebar.test.ts` → `ScopesSidebar.browser.test.ts`

- [ ] **Step 1: Rename the file**

Run (from `ui/`): `jj file move src/lib/components/ScopesSidebar.test.ts src/lib/components/ScopesSidebar.browser.test.ts`

- [ ] **Step 2: Rewrite the file contents**

Replace the file with:
```ts
import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import ScopesSidebar from './ScopesSidebar.svelte';
import { create } from '@bufbuild/protobuf';
import { ScopeCountSchema, type ScopeCount } from '$lib/gen/engram_pb';
import type { Category, Visibility } from '$lib/queries';

const scopes = [create(ScopeCountSchema, { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster', count: 142n })];

type Props = {
  scopes: ScopeCount[]; activeScope: string; categories: Category[]; visibility: Visibility;
  loading: boolean; error: unknown; onscope: (s: string) => void; onfilter: (c: Category[], v: Visibility) => void;
};

function baseProps(overrides: Partial<Props> = {}): Props {
  return { scopes, activeScope: '', categories: [], visibility: '', loading: false, error: null, onscope: () => {}, onfilter: () => {}, ...overrides };
}

describe('ScopesSidebar', () => {
  it('renders a scope chip and the filter categories', async () => {
    const screen = await render(ScopesSidebar, baseProps());
    await expect.element(screen.getByText('selfhosted-cluster')).toBeInTheDocument();
    await expect.element(screen.getByText('gotcha')).toBeInTheDocument();
  });

  it('fires onscope with the scope when a scope button is clicked', async () => {
    const onscope = vi.fn();
    const screen = await render(ScopesSidebar, baseProps({ onscope }));
    // The scope row nests a ScopeChip whose HoverCard trigger renders an
    // <a role="button">, so two role=button elements match; the outer row
    // button (first in DOM) is the one wired to onscope.
    await screen.getByRole('button', { name: /selfhosted-cluster/i }).first().click();
    expect(onscope).toHaveBeenCalledWith('repo:github.com/fzymgc-house/selfhosted-cluster');
  });

  it('toggles a category on via onfilter when its checkbox is clicked', async () => {
    const onfilter = vi.fn();
    const screen = await render(ScopesSidebar, baseProps({ onfilter }));
    await screen.getByRole('checkbox', { name: 'decision' }).click();
    expect(onfilter).toHaveBeenCalledWith(['decision'], '');
  });

  it('toggles a category off when it is already active', async () => {
    const onfilter = vi.fn();
    const screen = await render(ScopesSidebar, baseProps({ categories: ['gotcha'], onfilter }));
    await screen.getByRole('checkbox', { name: 'gotcha' }).click();
    expect(onfilter).toHaveBeenCalledWith([], '');
  });

  it('reflects the active visibility in the select trigger', async () => {
    const screen = await render(ScopesSidebar, baseProps({ visibility: 'shared' }));
    // Real Chromium DOM: the bits-ui Select trigger label binds to the active
    // value. (The popover-open path is exercisable here too; this asserts the
    // value→label binding branch.)
    await expect.element(screen.getByRole('button', { name: 'visibility' })).toHaveTextContent('shared');
  });

  it('shows an error message when scopes fail to load', async () => {
    const screen = await render(ScopesSidebar, baseProps({ error: new Error('boom') }));
    await expect.element(screen.getByTestId('scopes-error')).toBeInTheDocument();
    await expect.element(screen.getByText('selfhosted-cluster')).not.toBeInTheDocument();
  });

  it('shows skeletons while loading', async () => {
    const screen = await render(ScopesSidebar, baseProps({ loading: true }));
    await expect.element(screen.getByTestId('scopes-loading')).toBeInTheDocument();
    await expect.element(screen.getByText('selfhosted-cluster')).not.toBeInTheDocument();
  });
});
```
Changes: `getAllByRole(...)[0]` → `getByRole(...).first()`; the stale "Select popover cannot be reliably opened under the test DOM env" comment is retired (browser mode opens it); `userEvent` → locator clicks; props direct.

- [ ] **Step 3: Run it**

Run (from `ui/`): `pnpm exec vitest run --project browser src/lib/components/ScopesSidebar.browser.test.ts`
Expected: **7 passed.**

- [ ] **Step 4: Commit**

```
jj describe -m "test(ui): migrate ScopesSidebar test to browser tier (engram-utdv)

Retire the stale 'Select popover cannot be reliably opened' comment — the
real Chromium DOM has no such limitation.

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 9: Migrate MemoryDetail (Tabs interaction + within→locator chaining)

The richest migration: tab clicks + `within(tabpanel)` scoping.

**Files:**
- Rename: `ui/src/lib/components/MemoryDetail.test.ts` → `MemoryDetail.browser.test.ts`

- [ ] **Step 1: Rename the file**

Run (from `ui/`): `jj file move src/lib/components/MemoryDetail.test.ts src/lib/components/MemoryDetail.browser.test.ts`

- [ ] **Step 2: Rewrite the file contents**

Replace the file with:
```ts
import { render } from 'vitest-browser-svelte';
import { describe, it, expect } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import MemoryDetail from './MemoryDetail.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const withSummary = create(MemorySchema, {
  id: '1', content: 'full **body** here', summary: 'terse digest line',
  summarySource: 'auto', category: 'gotcha',
  scope: 'repo:github.com/fzymgc-house/selfhosted-cluster',
  source: 'agent-inferred', actor: 'sean', visibility: 'private', tags: ['mcp', 'routing']
});
const noSummary = create(MemorySchema, {
  id: '2', content: 'only content here', summary: '', summarySource: '',
  category: 'decision', scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private'
});
const clientSummary = create(MemorySchema, {
  id: '3', content: 'body', summary: 'human-authored digest',
  summarySource: 'client', category: 'decision', scope: 'repo:y',
  source: 'user-said', actor: 'sean', visibility: 'shared'
});
const unsourcedSummary = create(MemorySchema, {
  id: '4', content: 'body', summary: 'sourceless digest', summarySource: '',
  category: 'gotcha', scope: 'repo:z', source: 'user-said', actor: 'sean', visibility: 'private'
});

describe('MemoryDetail', () => {
  it('defaults to the Summary tab and shows summary + auto provenance', async () => {
    const screen = await render(MemoryDetail, { memory: withSummary, loading: false, error: null });
    await expect.element(screen.getByText('terse digest line')).toBeInTheDocument();
    await expect.element(screen.getByText(/auto/i)).toBeInTheDocument();
  });
  it('marks a client-authored summary as authored, not auto', async () => {
    const screen = await render(MemoryDetail, { memory: clientSummary, loading: false, error: null });
    await expect.element(screen.getByText('human-authored digest')).toBeInTheDocument();
    await expect.element(screen.getByText('authored')).toBeInTheDocument();
    await expect.element(screen.getByText('✦ auto')).not.toBeInTheDocument();
  });
  it('shows no provenance badge when the summary has no recorded source', async () => {
    const screen = await render(MemoryDetail, { memory: unsourcedSummary, loading: false, error: null });
    await expect.element(screen.getByText('sourceless digest')).toBeInTheDocument();
    await expect.element(screen.getByText('authored')).not.toBeInTheDocument();
    await expect.element(screen.getByText('✦ auto')).not.toBeInTheDocument();
  });
  it('exposes scope, actor, source, visibility, and tags on the Meta tab', async () => {
    const screen = await render(MemoryDetail, { memory: withSummary, loading: false, error: null });
    await screen.getByRole('tab', { name: 'Meta' }).click();
    // Once Meta is active, bits-ui leaves only its panel un-hidden, so the lone
    // accessible tabpanel is the Meta panel — scope the assertions to it.
    const meta = screen.getByRole('tabpanel');
    await expect.element(meta.getByText(withSummary.scope)).toBeInTheDocument();
    await expect.element(meta.getByText('sean')).toBeInTheDocument();
    await expect.element(meta.getByText('agent-inferred')).toBeInTheDocument();
    await expect.element(meta.getByText('private')).toBeInTheDocument();
    await expect.element(meta.getByText('mcp')).toBeInTheDocument();
    await expect.element(meta.getByText('routing')).toBeInTheDocument();
  });
  it('falls through to Content (rendered markdown) when there is no summary', async () => {
    const screen = await render(MemoryDetail, { memory: noSummary, loading: false, error: null });
    await expect.element(screen.getByText('only content here')).toBeInTheDocument();
  });
  it('prompts to select when nothing is chosen', async () => {
    const screen = await render(MemoryDetail, { memory: undefined, loading: false, error: null });
    await expect.element(screen.getByText(/select a record/i)).toBeInTheDocument();
  });
  it('shows a loading indicator while fetching', async () => {
    const screen = await render(MemoryDetail, { memory: undefined, loading: true, error: null });
    await expect.element(screen.getByText(/loading/i)).toBeInTheDocument();
  });
  it('shows a not-found message for a NotFound error', async () => {
    const screen = await render(MemoryDetail, { memory: undefined, loading: false, error: new ConnectError('missing', Code.NotFound) });
    await expect.element(screen.getByText(/record not found/i)).toBeInTheDocument();
  });
  it('shows a generic failure message for a non-NotFound error', async () => {
    const screen = await render(MemoryDetail, { memory: undefined, loading: false, error: new Error('boom') });
    await expect.element(screen.getByText(/failed to load record/i)).toBeInTheDocument();
  });
});
```
Changes: `within(meta).getByText(...)` → `meta.getByText(...)` (locator chaining); the `within` import is dropped; tab click via locator; props direct. `const meta = screen.getByRole('tabpanel')` is a lazy locator re-resolved on each `expect.element`, so it correctly points at whichever panel is un-hidden after the Meta click.

- [ ] **Step 3: Run it**

Run (from `ui/`): `pnpm exec vitest run --project browser src/lib/components/MemoryDetail.browser.test.ts`
Expected: **9 passed.**

- [ ] **Step 4: Run the FULL browser tier — all 8 files together**

Run (from `ui/`): `pnpm exec vitest run --project browser`
Expected: **37 passed** (6 markdown + 2 AppShell + 2 ScopeChip + 5 MemoryList + 3 MemoryRow + 3 CommandPalette + 7 ScopesSidebar + 9 MemoryDetail).

- [ ] **Step 5: Commit**

```
jj describe -m "test(ui): migrate MemoryDetail test to browser tier (engram-utdv)

Completes the browser-tier migration: within(tabpanel) -> locator chaining.
All 8 browser-tier files (37 tests) now run in real Chromium.

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 10: Remove the dead testing-library + jsdom dependencies

All browser-tier files now use `vitest-browser-svelte`; no file imports `@testing-library/svelte`, `@testing-library/user-event`, or relies on `jsdom`.

**Files:**
- Modify: `ui/package.json`
- Modify: `ui/vite.config.ts` (already drops `svelteTesting()` from Task 1 — verify)

- [ ] **Step 1: Confirm no remaining imports of the dead deps**

Run (from `ui/`): `rg "@testing-library/svelte|@testing-library/user-event|@vitest-environment jsdom" src`
Expected: **no matches.** (If any match, that file was missed — migrate it before continuing.)

- [ ] **Step 2: Remove the dead devDependencies**

Run (from `ui/`): `pnpm remove @testing-library/svelte @testing-library/user-event jsdom`
Expected: 3 devDeps removed; `undici` drops from the lockfile as a transitive of jsdom. `@testing-library/jest-dom` is **kept** (matchers used via `expect.element`).

- [ ] **Step 3: Verify `svelteTesting()` is gone from the config**

Run (from `ui/`): `rg "svelteTesting|@testing-library/svelte/vite" vite.config.ts`
Expected: **no matches** (removed in Task 1; this is the verification step).

- [ ] **Step 4: Run the full browser tier again to confirm nothing broke**

Run (from `ui/`): `pnpm exec vitest run --project browser`
Expected: **37 passed.**

- [ ] **Step 5: Commit**

```
jj describe -m "chore(ui): drop testing-library + jsdom test deps (engram-utdv)

Browser tier uses vitest-browser-svelte; jsdom (+transitive undici),
@testing-library/svelte, and @testing-library/user-event are now unused.
Keeps @testing-library/jest-dom for expect.element matchers.

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 11: Resolve the node-tier environment (drop happy-dom if clean)

Spec §5 deferred decision: prefer `environment: 'node'` + drop `happy-dom`; fall back to `happy-dom` only if a logic test transitively needs a DOM global.

**Files:**
- Modify: `ui/vite.config.ts` (node project `environment`)
- Modify: `ui/vitest-setup.ts` (the localStorage stub)
- Possibly modify: `ui/package.json` (drop `happy-dom`)

- [ ] **Step 1: Flip the node tier to `environment: 'node'`**

In `ui/vite.config.ts`, change the node project's `environment: 'happy-dom'` to `environment: 'node'` and remove the `environmentOptions: { happyDOM: ... }` line from that project.

- [ ] **Step 2: Run the node tier**

Run (from `ui/`): `pnpm exec vitest run --project node`
Expected: **26 passed** (2 client + 3 errors + 4 queries + 4 scope + 4 summary + 4 time + 5 app.css-loop). If any test fails with a missing DOM global (`document`/`window`/`ResizeObserver`), the node env is too bare for that test.

- [ ] **Step 3a (if Step 2 is GREEN): drop happy-dom**

The `vitest-setup.ts` localStorage stub already guards with `if (typeof localStorage === 'undefined')` — under `environment: 'node'` it installs the in-memory stub, which is what `mode-watcher` needs at module-eval. Keep `vitest-setup.ts` as-is.
Run (from `ui/`): `pnpm remove happy-dom`
Then re-run: `pnpm exec vitest run --project node` → Expected: **26 passed.**

- [ ] **Step 3b (if Step 2 is RED): keep happy-dom, revert Step 1**

Revert `ui/vite.config.ts` to `environment: 'happy-dom'` + the `environmentOptions` line for the node project. Do NOT remove `happy-dom`. Append a note to engram-utdv: "node tier retains happy-dom — <failing test> needs <DOM global>". The spec's decision rule explicitly allows this fallback.

- [ ] **Step 4: Commit**

```
jj describe -m "chore(ui): node test tier runs on <node|happy-dom> (engram-utdv)

<If 3a: drop happy-dom — the 6 logic tests + app.css need no DOM, only the
localStorage stub which vitest-setup.ts provides under environment:node.>
<If 3b: retain happy-dom — <test> needs <global>.>

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 12: Wire pnpm scripts + run the whole suite

**Files:**
- Modify: `ui/package.json` (`scripts`)

- [ ] **Step 1: Confirm the default `test` script runs both tiers**

`ui/package.json` already has `"test": "vitest run"`. With the `projects[]` config, a bare `vitest run` executes BOTH the node and browser projects in one pass. No script change is strictly required, but add an explicit browser-only escape hatch for fast local iteration:
```json
  "scripts": {
    "dev": "vite dev",
    "build": "vite build",
    "test": "vitest run",
    "test:browser": "vitest run --project browser",
    "test:node": "vitest run --project node",
    "check": "svelte-check --tsconfig ./tsconfig.json"
  },
```

- [ ] **Step 2: Run the complete suite — both tiers, one command**

Run (from `ui/`): `pnpm test`
Expected: **63 passed** (37 browser + 26 node), with the reporter showing `node` and `browser` project labels.

- [ ] **Step 3: Commit**

```
jj describe -m "chore(ui): add test:browser / test:node project scripts (engram-utdv)

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 13: Add the `ui-test` CI job (Playwright-cached)

**Files:**
- Modify: `.github/workflows/ci.yaml`

- [ ] **Step 1: Add the `ui-test` job after the `ui-drift` job**

In `.github/workflows/ci.yaml`, add this job (mirrors the `ui-drift` setup pattern: pnpm/action-setup pointed at `ui/package.json`, node 24, pnpm cache):
```yaml
  ui-test:
    name: ui tests
    runs-on: ubuntu-latest
    if: ${{ !startsWith(github.head_ref, 'release-please--') }}
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7
      - uses: pnpm/action-setup@0ebf47130e4866e96fce0953f49152a61190b271 # v6
        with:
          package_json_file: ui/package.json
      - uses: actions/setup-node@48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e # v6
        with:
          node-version: '24'
          cache: pnpm
          cache-dependency-path: ui/pnpm-lock.yaml
      - run: cd ui && pnpm install --frozen-lockfile
      # Cache the Playwright browser binaries keyed on the resolved version so
      # warm runs skip the ~190MB Chromium download.
      - name: playwright version
        id: pw
        run: cd ui && echo "version=$(pnpm ls playwright --depth 0 --parseable --long | sed -n 's/.*playwright@//p' | head -1)" >> "$GITHUB_OUTPUT"
      - uses: actions/cache@v4 # IMPLEMENTER: pin to a SHA — repo convention SHA-pins every action (see ui-drift job)
        id: pw-cache
        with:
          path: ~/.cache/ms-playwright
          key: playwright-${{ runner.os }}-${{ steps.pw.outputs.version }}
      - run: cd ui && pnpm exec playwright install --with-deps chromium
        if: steps.pw-cache.outputs.cache-hit != 'true'
      - run: cd ui && pnpm exec playwright install-deps chromium
        if: steps.pw-cache.outputs.cache-hit == 'true'
      - run: cd ui && pnpm test
```
Note: on a cache hit the binaries are restored but OS-level shared libs are not cached, so `install-deps` (apt packages only, fast) still runs. On a miss, `install --with-deps` fetches both.

- [ ] **Step 2: Verify the workflow is valid YAML / actionlint-clean**

Run (from repo root): `task lint` (or `actionlint .github/workflows/ci.yaml` if available).
Expected: no errors on the new job.

- [ ] **Step 3: Commit**

```
jj describe -m "ci: add ui-test job running both vitest tiers (engram-utdv)

New job runs the node + browser test projects; Playwright Chromium binaries
cached by version. Becomes a required check (ruleset update in next commit).

Co-Authored-By: Claude <noreply@anthropic.com>"
jj new
```

---

## Task 14: Make `ui-test` a required check (protect-main ruleset, lockstep)

**This step touches GitHub repo settings, not files.** Per memory `protect-main-ruleset-id-17228701`, the ruleset matches required checks by **exact job name** (`ui tests` — the `name:` from Task 13), and the update MUST land together with the merged workflow change or open PRs strand on "Expected."

- [ ] **Step 1: Confirm the new job's exact name**

The job `name:` is `ui tests` (from Task 13 Step 1). The ruleset must reference this string verbatim.

- [ ] **Step 2: Add `ui tests` to the protect-main ruleset (id 17228701)**

After this plan's PR is merged to `main` and the `ui-test` job has reported at least one green run, add `ui tests` to the required status checks of ruleset 17228701 (integration_id 15368 — same GitHub Actions app as the existing 7 checks):
```
gh api -X PATCH repos/seanb4t/engram/rulesets/17228701 --input <patch.json>
```
where `<patch.json>` adds `{ "context": "ui tests", "integration_id": 15368 }` to the existing `required_status_checks` array (the current 7: test, golangci-lint, commit-lint, license headers, helm chart, actionlint, python). **Do not remove or rename** any existing entry. Verify first with `gh api repos/seanb4t/engram/rulesets/17228701` to capture the current array.

- [ ] **Step 3: Verify a fresh PR shows `ui tests` as required**

Open (or re-push) a trivial PR and confirm `ui tests` appears under required checks and does NOT strand on "Expected." If it strands, the context string doesn't match the job name — fix the ruleset entry to match `ui tests` exactly.

- [ ] **Step 4: No file commit** — this is a settings change. Record on the bead:

Run: `bd note engram-utdv "ui tests added to protect-main ruleset 17228701 as 8th required check"`

---

## Self-review checklist (completed by plan author)

- **Spec coverage:** §3 two-project config → Task 1. §3.1 spike-gate → Task 1 (go/no-go). §4 file routing/rename → Tasks 2-9. §4 dep churn → Task 10. §5 node env → Task 11. §6 CI → Tasks 13-14. §7 rollout order → Tasks 1→14 spine. All covered.
- **Test count reconciles:** 37 browser (6+2+2+5+3+3+7+9) + 26 node (2+3+4+4+4+4+ app.css-loop 5) = 63. Matches the spec's runtime count.
- **Type/signature consistency:** every component task uses `await render(C, directProps)`, `await expect.element(locator).matcher()`, locator `.click()` — no `userEvent`, no `{ props }`, no `within`. `rerender` is async with direct props (Task 6). Locator chaining `meta.getByText` (Task 9). All consistent with the pre-flight reference table.
- **No placeholders:** every code step shows full file contents or exact diffs; every run step shows the command + expected pass count.
