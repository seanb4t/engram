import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import { persistResume } from '$lib/resume';
import SearchPage from './+page.svelte';

// `vi.hoisted` runs before the module's own imports are linked (its factory
// is spliced in ahead of every `import`), so `SvelteURL` — imported normally
// above — is not yet bound when a synchronous factory would run. A dynamic
// `import()` inside the (awaited) factory sidesteps that ordering: it is a
// genuine async operation, resolved after linking, not a static binding.
const { gotoSpy, pageState, searchMemoriesSpy, getMemorySpy, listScopesSpy, consumeResumeSpy } = await vi.hoisted(async () => {
  const { SvelteURL } = await import('svelte/reactivity');
  const url = new SvelteURL('http://localhost/search');
  const pageState = { url };
  const gotoSpy = vi.fn((href: string) => {
    const next = new URL(href, 'http://localhost');
    pageState.url.href = next.href;
  });
  return {
    gotoSpy,
    pageState,
    searchMemoriesSpy: vi.fn(),
    getMemorySpy: vi.fn(),
    listScopesSpy: vi.fn(),
    consumeResumeSpy: vi.fn()
  };
});

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));
vi.mock('$app/state', () => ({ page: pageState }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: { ...actual.engram, searchMemories: searchMemoriesSpy, getMemory: getMemorySpy, listScopes: listScopesSpy }
  };
});

vi.mock('$lib/resume', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/resume')>();
  return {
    ...actual,
    consumeResume: (...args: Parameters<typeof actual.consumeResume>) => {
      consumeResumeSpy(...args);
      return actual.consumeResume(...args);
    }
  };
});

let qc: QueryClient;
function renderSearch() {
  return render(SearchPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

function makeMemory(overrides: MessageInitShape<typeof MemorySchema> = {}): Memory {
  return create(MemorySchema, {
    id: 'm-default',
    category: 'convention',
    summary: 'default summary',
    content: '',
    tags: [],
    scope: 'repo:test',
    visibility: 'private',
    owner: 'me',
    score: 0.5,
    shortId: 's0000000001',
    ...overrides
  });
}

function emptySearchResult() {
  return { memories: [], searchedScopes: [], scopesTruncated: false, scopesUnknown: false };
}

beforeEach(() => {
  gotoSpy.mockClear();
  searchMemoriesSpy.mockReset().mockResolvedValue(emptySearchResult());
  getMemorySpy.mockReset();
  listScopesSpy.mockReset().mockResolvedValue({ scopes: [], approximate: false });
  consumeResumeSpy.mockReset();
  sessionStorage.clear();
  pageState.url.href = 'http://localhost/search';
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('search route — race safety (SC4)', () => {
  it('never renders a stale response: the earlier query is aborted and the later one wins', async () => {
    pageState.url.href = 'http://localhost/search?q=alpha';
    let resolveAlpha!: (v: unknown) => void;
    const alphaPromise = new Promise((resolve) => {
      resolveAlpha = resolve;
    });
    let alphaSignal: AbortSignal | undefined;
    searchMemoriesSpy.mockImplementation((req: { query: string }, opts?: { signal?: AbortSignal }) => {
      if (req.query === 'alpha') {
        alphaSignal = opts?.signal;
        return alphaPromise;
      }
      if (req.query === 'beta') {
        return Promise.resolve({
          memories: [makeMemory({ id: 'm-beta', summary: 'beta hit' })],
          searchedScopes: ['repo:test'],
          scopesTruncated: false,
          scopesUnknown: false
        });
      }
      return Promise.resolve(emptySearchResult());
    });

    const screen = await renderSearch();
    await expect.poll(() => searchMemoriesSpy.mock.calls.some((c) => c[0].query === 'alpha')).toBe(true);

    pageState.url.searchParams.set('q', 'beta');
    await expect.element(screen.getByText('beta hit')).toBeInTheDocument();

    resolveAlpha({
      memories: [makeMemory({ id: 'm-alpha', summary: 'alpha hit' })],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    await new Promise((r) => setTimeout(r, 30));

    await expect.element(screen.getByText('alpha hit')).not.toBeInTheDocument();
    await expect.element(screen.getByText('beta hit')).toBeInTheDocument();
    expect(alphaSignal?.aborted).toBe(true);
  });
});

describe('search route — keep previous data while re-querying', () => {
  it('keeps the previous rows visible and marks the list busy until the newer query resolves', async () => {
    pageState.url.href = 'http://localhost/search?q=alpha';
    let resolveBeta!: (v: unknown) => void;
    const betaPromise = new Promise((resolve) => {
      resolveBeta = resolve;
    });
    searchMemoriesSpy.mockImplementation((req: { query: string }) => {
      if (req.query === 'alpha') {
        return Promise.resolve({
          memories: [makeMemory({ id: 'm-alpha', summary: 'alpha hit' })],
          searchedScopes: ['repo:test'],
          scopesTruncated: false,
          scopesUnknown: false
        });
      }
      if (req.query === 'beta') return betaPromise;
      return Promise.resolve(emptySearchResult());
    });

    const screen = await renderSearch();
    await expect.element(screen.getByText('alpha hit')).toBeInTheDocument();

    pageState.url.searchParams.set('q', 'beta');
    await expect.poll(() => searchMemoriesSpy.mock.calls.some((c) => c[0].query === 'beta')).toBe(true);
    await expect.element(screen.getByText('alpha hit')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.results-listbox-wrapper.busy') !== null).toBe(true);

    resolveBeta({
      memories: [makeMemory({ id: 'm-beta', summary: 'beta hit' })],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    await expect.element(screen.getByText('beta hit')).toBeInTheDocument();
  });
});

describe('search route — defaults', () => {
  it('sends crossSpine true, an empty scope, k=50n and full=true for plain text', async () => {
    pageState.url.href = 'http://localhost/search?q=github';

    await renderSearch();
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    const [req] = searchMemoriesSpy.mock.calls[0];
    expect(req.crossSpine).toBe(true);
    expect(req.scope).toBe('');
    expect(req.k).toBe(50n);
    expect(req.full).toBe(true);
  });
});

describe('search route — id resolution', () => {
  it('resolves a UUID via GetMemory, never calls SearchMemories, shows the resolution line and opens the pane', async () => {
    const uuid = '753aba22-1111-2222-3333-444455556666';
    pageState.url.href = `http://localhost/search?q=${uuid}`;
    const memory = makeMemory({ id: uuid, summary: 'the resolved memory' });
    getMemorySpy.mockResolvedValue({ memory });

    const screen = await renderSearch();
    // The record appears twice — once as the single result row, once as the
    // open pane's title — so scope this to the pane heading specifically.
    await expect.element(screen.getByRole('heading', { name: 'the resolved memory' })).toBeInTheDocument();

    expect(searchMemoriesSpy).not.toHaveBeenCalled();
    expect(getMemorySpy).toHaveBeenCalledWith({ id: uuid }, expect.objectContaining({ signal: expect.anything() }));
    await expect.element(screen.getByText(/Resolved id/)).toBeInTheDocument();
    expect(screen.container.querySelectorAll('[role="option"]').length).toBe(1);
    await expect.poll(() => screen.container.querySelector('[aria-label="Memory detail"]') !== null).toBe(true);
  });
});

describe('search route — toggle pane', () => {
  it('clicking a row opens the pane; clicking the same row again closes it', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const memory = makeMemory({ id: 'm-1', summary: 'toggle me' });
    searchMemoriesSpy.mockResolvedValue({ memories: [memory], searchedScopes: ['repo:test'], scopesTruncated: false, scopesUnknown: false });

    const screen = await renderSearch();
    await expect.element(screen.getByText('toggle me')).toBeInTheDocument();

    (screen.container.querySelector('[role="option"]') as HTMLElement).click();
    await expect.poll(() => pageState.url.searchParams.get('sel')).toBe('m-1');

    (screen.container.querySelector('[role="option"]') as HTMLElement).click();
    await expect.poll(() => pageState.url.searchParams.get('sel')).toBe(null);
  });
});

describe('search route — row-action keys', () => {
  it("'e' on the active row prefetches the record for editing via WriteSurfaces.openEdit", async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const memory = makeMemory({ id: 'm-edit', summary: 'edit me' });
    searchMemoriesSpy.mockResolvedValue({ memories: [memory], searchedScopes: ['repo:test'], scopesTruncated: false, scopesUnknown: false });
    getMemorySpy.mockResolvedValue({ memory });

    const screen = await renderSearch();
    await expect.element(screen.getByText('edit me')).toBeInTheDocument();

    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await userEvent.keyboard('e');

    await expect.poll(() => getMemorySpy.mock.calls.some((c) => c[0]?.id === 'm-edit')).toBe(true);
  });
});

describe('search route — facet chips round-trip through the URL (ROW-05)', () => {
  it('loads facet params from the URL and sends them on the SearchMemories request', async () => {
    pageState.url.href = 'http://localhost/search?q=x&cat=gotcha&tag=ci&inc=superseded&after=2026-01-01T00%3A00%3A00Z';

    await renderSearch();
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    const [req] = searchMemoriesSpy.mock.calls[0];
    expect(req.categories).toEqual(['gotcha']);
    expect(req.tags).toEqual(['ci']);
    expect(req.includeSuperseded).toBe(true);
    expect(req.createdAfter).toBe('2026-01-01T00:00:00Z');
  });

  it('toggling a facet chip rewrites the URL through encodeSearchParams', async () => {
    pageState.url.href = 'http://localhost/search?q=x';
    searchMemoriesSpy.mockResolvedValue(emptySearchResult());

    const screen = await renderSearch();
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);

    await screen.getByRole('button', { name: /gotcha/ }).click();
    await expect.poll(() => pageState.url.searchParams.get('cat')).toBe('gotcha');
  });

  it('with a category filter active, a second SearchMemories call without categories supplies "N hits of M"', async () => {
    pageState.url.href = 'http://localhost/search?q=x&cat=gotcha';
    searchMemoriesSpy.mockImplementation((req: { categories: string[] }) => {
      if (req.categories.length > 0) {
        return Promise.resolve({
          memories: [makeMemory({ id: 'm-a', summary: 'a', category: 'gotcha' })],
          searchedScopes: ['repo:test'],
          scopesTruncated: false,
          scopesUnknown: false
        });
      }
      return Promise.resolve({
        memories: [
          makeMemory({ id: 'm-a', summary: 'a', category: 'gotcha' }),
          makeMemory({ id: 'm-b', summary: 'b', category: 'convention' })
        ],
        searchedScopes: ['repo:test'],
        scopesTruncated: false,
        scopesUnknown: false
      });
    });

    const screen = await renderSearch();
    await expect.element(screen.getByText(/1 hit of 2/)).toBeInTheDocument();
  });
});

// Search-create recovery (Codex round-3 MEDIUM): the prior suite never
// covered a seeded create-mode envelope landing on the search route.
describe('search route — re-auth landing recovery', () => {
  it('reopens the memory create sheet with restored values from a seeded create-mode envelope, then consumes it once', async () => {
    persistResume({
      returnPath: '/search?q=foo',
      kind: 'memory',
      mode: 'create',
      recordId: null,
      values: { content: 'restored search-route draft', scope: 'repo:restored' }
    });

    const screen = await renderSearch();
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await expect.element(screen.getByRole('heading', { name: 'New memory' })).toBeInTheDocument();
    await expect.element(screen.getByLabelText('content')).toHaveValue('restored search-route draft');
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('repo:restored');
    await expect.poll(() => consumeResumeSpy.mock.calls.length).toBe(1);
  });

  it('does not reopen anything for a discovery-kind envelope (kind mismatch)', async () => {
    persistResume({
      returnPath: '/discovery',
      kind: 'discovery',
      mode: 'create',
      recordId: null,
      values: {}
    });
    const screen = await renderSearch();
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
    expect(consumeResumeSpy).not.toHaveBeenCalled();
  });
});
