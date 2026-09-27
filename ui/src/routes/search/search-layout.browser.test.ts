// UI-REVIEW regression: with the detail pane open the /search layout must
// never scroll horizontally — neither the page nor the results list — and
// every row keeps its summary readable and its score inside the list.
// app.css is imported explicitly: without it --u and every calc(N*var(--u))
// dimension are invalid, so geometry would pass for the wrong reason.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { MemorySchema } from '$lib/gen/engram_pb';
import SearchPage from './+page.svelte';

const { pageState, searchMemoriesSpy, getMemorySpy, listScopesSpy } = await vi.hoisted(async () => {
  const { SvelteURL } = await import('svelte/reactivity');
  return {
    pageState: { url: new SvelteURL('http://localhost/search') },
    searchMemoriesSpy: vi.fn(),
    getMemorySpy: vi.fn(),
    listScopesSpy: vi.fn()
  };
});

vi.mock('$app/navigation', () => ({
  goto: (href: string) => {
    pageState.url.href = new URL(href, 'http://localhost').href;
  }
}));
vi.mock('$app/paths', () => ({ base: '/ui' }));
vi.mock('$app/state', () => ({ page: pageState }));
vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: { ...actual.engram, searchMemories: searchMemoriesSpy, getMemory: getMemorySpy, listScopes: listScopesSpy }
  };
});

const memories = Array.from({ length: 12 }, (_, i) =>
  create(MemorySchema, {
    id: `m-${i}`,
    shortId: `0kx0ab7qz${i % 10}`,
    category: ['convention', 'gotcha', 'decision', 'preference'][i % 4],
    summary: `release-please drives SemVer tags; the milestone label is CalVer and never feeds it (${i})`,
    content: 'body',
    tags: ['release', 'ops', 'config'],
    scope: 'repo:seanb4t/engram',
    visibility: 'shared',
    owner: 'me',
    score: 0.9 - i * 0.03,
    supersededBy: i === 1 ? 'm-0' : '',
    createdAt: timestampFromDate(new Date(Date.now() - (i + 1) * 86_400_000))
  })
);

let qc: QueryClient;

beforeEach(() => {
  searchMemoriesSpy.mockReset().mockResolvedValue({
    memories,
    searchedScopes: ['repo:seanb4t/engram'],
    scopesTruncated: false,
    scopesUnknown: false
  });
  getMemorySpy.mockReset().mockResolvedValue({ memory: memories[0] });
  listScopesSpy.mockReset().mockResolvedValue({ scopes: [], approximate: false });
  localStorage.clear();
  sessionStorage.clear();
  pageState.url.href = 'http://localhost/search?q=release&sel=m-0';
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

afterEach(async () => {
  await page.viewport(414, 896);
});

describe('search route — layout at narrow viewports with the detail pane open', () => {
  it.each([1024, 900, 860, 760, 620])('at a %ipx viewport nothing scrolls horizontally and every row stays readable', async (vw) => {
    await page.viewport(vw, 800);
    const screen = await render(SearchPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
    // Stand-in for AppShell's <main>: the viewport minus the 64u nav rail.
    Object.assign(screen.container.style, { width: 'calc(100vw - 64 * var(--u))', height: '720px', marginLeft: 'calc(64 * var(--u))' });

    await expect.poll(() => screen.container.querySelector('[aria-label="Memory detail"]') !== null).toBe(true);
    await expect.poll(() => screen.container.querySelectorAll('[role="option"] .result-row-line').length).toBeGreaterThan(0);
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));

    const doc = document.documentElement;
    expect(doc.scrollWidth).toBeLessThanOrEqual(doc.clientWidth);

    const listbox = screen.container.querySelector('[role="listbox"]') as HTMLElement;
    expect(listbox.scrollWidth).toBeLessThanOrEqual(listbox.clientWidth + 1);
    const listBox = listbox.getBoundingClientRect();

    for (const row of Array.from(screen.container.querySelectorAll<HTMLElement>('[role="option"] .result-row-line'))) {
      const sum = (row.querySelector('.sum') as HTMLElement).getBoundingClientRect();
      const score = (row.querySelector('.score') as HTMLElement).getBoundingClientRect();
      expect(sum.width).toBeGreaterThanOrEqual(60);
      expect(score.width).toBeGreaterThan(0);
      expect(score.right).toBeLessThanOrEqual(listBox.right + 0.5);
    }
  });
});
