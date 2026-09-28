import { render } from 'vitest-browser-svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import {
  MemorySchema,
  ArchiveOutcome,
  RelatedMemoriesResponseSchema,
  UnderstandQueryResponseSchema,
  SuggestionSource,
  type Memory,
  type SupersedeMemoryRequest
} from '$lib/gen/engram_pb';
import { persistResume } from '$lib/resume';
import SearchPage from './+page.svelte';

// `vi.hoisted` runs before the module's own imports are linked (its factory
// is spliced in ahead of every `import`), so `SvelteURL` — imported normally
// above — is not yet bound when a synchronous factory would run. A dynamic
// `import()` inside the (awaited) factory sidesteps that ordering: it is a
// genuine async operation, resolved after linking, not a static binding.
const {
  gotoSpy,
  pageState,
  searchMemoriesSpy,
  getMemorySpy,
  listScopesSpy,
  listMemoriesSpy,
  listTagsSpy,
  consumeResumeSpy,
  archiveMemorySpy,
  restoreMemorySpy,
  supersedeMemorySpy,
  relatedMemoriesSpy,
  understandQuerySpy
} = await vi.hoisted(async () => {
  const { SvelteURL } = await import('svelte/reactivity');
  const url = new SvelteURL('http://localhost/search');
  const pageState = { url };
  const gotoSpy = vi.fn((href: string, _opts?: { replaceState?: boolean }) => {
    const next = new URL(href, 'http://localhost');
    pageState.url.href = next.href;
  });
  return {
    gotoSpy,
    pageState,
    searchMemoriesSpy: vi.fn(),
    getMemorySpy: vi.fn(),
    listScopesSpy: vi.fn(),
    listMemoriesSpy: vi.fn(),
    listTagsSpy: vi.fn(),
    consumeResumeSpy: vi.fn(),
    archiveMemorySpy: vi.fn(),
    restoreMemorySpy: vi.fn(),
    supersedeMemorySpy: vi.fn(),
    relatedMemoriesSpy: vi.fn(),
    understandQuerySpy: vi.fn()
  };
});

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));
vi.mock('$app/state', () => ({ page: pageState }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: {
      ...actual.engram,
      searchMemories: searchMemoriesSpy,
      getMemory: getMemorySpy,
      listScopes: listScopesSpy,
      listMemories: listMemoriesSpy,
      listTags: listTagsSpy,
      relatedMemories: relatedMemoriesSpy,
      understandQuery: understandQuerySpy
    },
    engramWrite: {
      ...actual.engramWrite,
      archiveMemory: archiveMemorySpy,
      restoreMemory: restoreMemorySpy,
      supersedeMemory: supersedeMemorySpy
    }
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

function emptyListResult() {
  return { memories: [], total: 0n, nextPageToken: '', searchedScopes: [], scopesTruncated: false, scopesUnknown: false };
}

beforeEach(() => {
  gotoSpy.mockClear();
  searchMemoriesSpy.mockReset().mockResolvedValue(emptySearchResult());
  getMemorySpy.mockReset();
  listScopesSpy.mockReset().mockResolvedValue({ scopes: [], approximate: false });
  listMemoriesSpy.mockReset().mockResolvedValue(emptyListResult());
  listTagsSpy.mockReset().mockResolvedValue({
    tags: [
      { tag: 'qdrant', count: 400n },
      { tag: 'mcp', count: 200n }
    ],
    more: false
  });
  consumeResumeSpy.mockReset();
  archiveMemorySpy.mockReset();
  restoreMemorySpy.mockReset();
  supersedeMemorySpy.mockReset();
  relatedMemoriesSpy.mockReset();
  understandQuerySpy.mockReset().mockResolvedValue({ enabled: false, suggestions: [] });
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

describe('search route — id/short_id pane close (WR-02)', () => {
  it('closing the pane via the close button does not silently reopen it', async () => {
    const uuid = '753aba22-1111-2222-3333-444455556666';
    pageState.url.href = `http://localhost/search?q=${uuid}`;
    const memory = makeMemory({ id: uuid, summary: 'the resolved memory' });
    getMemorySpy.mockResolvedValue({ memory });

    const screen = await renderSearch();
    await expect.element(screen.getByRole('heading', { name: 'the resolved memory' })).toBeInTheDocument();

    await screen.getByRole('button', { name: 'close', exact: true }).click();
    await expect.element(screen.getByRole('heading', { name: 'the resolved memory' })).not.toBeInTheDocument();
    await expect.poll(() => pageState.url.searchParams.get('sel')).toBe(null);

    // idQ.data stays loaded and the query is still classified id — without
    // the fix, effectiveSel falls back to autoOpenId and the pane reopens.
    await new Promise((r) => setTimeout(r, 30));
    await expect.element(screen.getByRole('heading', { name: 'the resolved memory' })).not.toBeInTheDocument();
  });

  it('clicking the auto-opened row a second time closes the pane instead of leaving it open', async () => {
    const uuid = '753aba22-1111-2222-3333-444455556667';
    pageState.url.href = `http://localhost/search?q=${uuid}`;
    const memory = makeMemory({ id: uuid, summary: 'the resolved memory' });
    getMemorySpy.mockResolvedValue({ memory });

    const screen = await renderSearch();
    await expect.element(screen.getByRole('heading', { name: 'the resolved memory' })).toBeInTheDocument();

    (screen.container.querySelector('[role="option"]') as HTMLElement).click();
    await expect.element(screen.getByRole('heading', { name: 'the resolved memory' })).not.toBeInTheDocument();
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

describe('search route — related entry point (D-03/D-04)', () => {
  it("the pane's Related button navigates to /related/{id} carrying where it came from", async () => {
    pageState.url.href = 'http://localhost/search?q=github&sel=m-1';
    const memory = makeMemory({ id: 'm-1', summary: 'related me' });
    searchMemoriesSpy.mockResolvedValue({
      memories: [memory],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    getMemorySpy.mockResolvedValue({ memory });

    const screen = await renderSearch();
    // Force the wide (non-overlay) RecallSplit layout so the pane's action
    // row is a normal in-flow element, not the narrow-mode absolute overlay
    // (RecallSplit.browser.test.ts's own convention for reliable clicks).
    screen.container.style.width = '1200px';
    await expect.poll(() => screen.container.querySelector('[aria-label="Memory detail"]') !== null).toBe(true);
    await expect.element(screen.getByRole('heading', { name: 'related me' })).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Related' }).click();

    expect(gotoSpy).toHaveBeenCalledWith(`/ui/related/m-1?from=${encodeURIComponent('/search?q=github&sel=m-1')}`);
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

describe('search route — honest empty state (ENTRY-03)', () => {
  it('cross-spine zero hits names the query, offers fix rows for hidden/category state, and never flashes "no matches" while in flight', async () => {
    pageState.url.href = 'http://localhost/search?q=zzz&cat=gotcha';
    let resolveSearch!: (v: unknown) => void;
    const pending = new Promise((resolve) => {
      resolveSearch = resolve;
    });
    searchMemoriesSpy.mockReturnValue(pending);

    const screen = await renderSearch();
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    await expect.element(screen.getByText('No memories match', { exact: false })).not.toBeInTheDocument();

    resolveSearch({
      memories: [],
      searchedScopes: ['repo:a', 'repo:b'],
      scopesTruncated: false,
      scopesUnknown: false,
      recallGateHidden: { total: 1n, archived: 0n, superseded: 1n, expired: 0n, scheduled: 0n }
    });

    await expect.element(screen.getByText('No memories match zzz in any scope you can read · 1 hidden by recall gate')).toBeInTheDocument();
    await expect.element(screen.getByText(/no matches/i)).not.toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Include superseded' })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Clear the category filter' })).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Include superseded' }).click();
    await expect.poll(() => pageState.url.searchParams.getAll('inc')).toEqual(['superseded']);
  });

  it('a scoped zero-hit response offers "Search every readable scope", which removes scope and restores cross-spine', async () => {
    pageState.url.href = 'http://localhost/search?q=zzz&scope=repo%3Ax';
    searchMemoriesSpy.mockResolvedValue({
      memories: [],
      searchedScopes: ['repo:x'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await renderSearch();
    await expect.element(screen.getByText('No memories match zzz in the 1 scope searched')).toBeInTheDocument();
    await screen.getByRole('button', { name: 'Search every readable scope' }).click();

    await expect.poll(() => pageState.url.searchParams.get('scope')).toBe(null);
    await expect.poll(() => pageState.url.searchParams.get('xs')).toBe(null);
  });
});

describe('search route — honest failure states (ENTRY-05)', () => {
  it('a rejected request renders the real envelope and its fix row; clicking it restores cross-spine', async () => {
    pageState.url.href = 'http://localhost/search?q=x&scope=repo%3Ax';
    searchMemoriesSpy.mockRejectedValue(
      new ConnectError('field=scope,cross_spine hint=conditional_required: scope is required unless cross_spine is true', Code.FailedPrecondition)
    );

    const screen = await renderSearch();
    await expect.element(screen.getByText('Server rejected the request')).toBeInTheDocument();
    await expect
      .element(screen.getByText('field=scope,cross_spine hint=conditional_required: scope is required unless cross_spine is true', { exact: false }))
      .toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Re-enable cross-spine' })).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Re-enable cross-spine' }).click();
    await expect.poll(() => pageState.url.searchParams.get('scope')).toBe(null);
    await expect.poll(() => pageState.url.searchParams.get('xs')).toBe(null);
  });

  it('an opaque failure says nothing was searched, offers Retry and Copy error, and shows no empty heading', async () => {
    pageState.url.href = 'http://localhost/search?q=x';
    searchMemoriesSpy.mockRejectedValue(new ConnectError('backend unreachable', Code.Unavailable));
    const writeSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);

    const screen = await renderSearch();
    await expect
      .element(screen.getByText('Search failed — nothing was searched. search_memory returned code=Unavailable'))
      .toBeInTheDocument();
    await expect.element(screen.getByText('Nothing was searched, so this is not an empty result')).toBeInTheDocument();
    await expect.element(screen.getByText('No memories match', { exact: false })).not.toBeInTheDocument();

    searchMemoriesSpy.mockClear();
    await screen.getByRole('button', { name: 'Retry' }).click();
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);

    await screen.getByRole('button', { name: 'Copy error' }).click();
    await expect.poll(() => writeSpy.mock.calls.at(-1)?.[0]).toBe('backend unreachable');
  });

  it('an ambiguous short_id renders the D-04 warning line', async () => {
    pageState.url.href = 'http://localhost/search?q=k3m9p2qr7a';
    getMemorySpy.mockRejectedValue(new ConnectError('ambiguous short id: k3m9p2qr7a', Code.FailedPrecondition));

    const screen = await renderSearch();
    await expect
      .element(screen.getByText('short_id k3m9p2qr7a is ambiguous — paste the full id to be exact'))
      .toBeInTheDocument();
  });

  it('a UUID with NotFound renders the not-found line', async () => {
    const uuid = '753aba22-1111-2222-3333-444455556666';
    pageState.url.href = `http://localhost/search?q=${uuid}`;
    getMemorySpy.mockRejectedValue(new ConnectError('not found', Code.NotFound));

    const screen = await renderSearch();
    await expect
      .element(screen.getByText('No memory with that id that you can read · not-found and not-yours look the same by design'))
      .toBeInTheDocument();
  });
});

describe('search route — Show more escalates k through the URL (D-08)', () => {
  it('shows "Show more" at exactly k results and navigates to the next K_STEPS value on click', async () => {
    pageState.url.href = 'http://localhost/search?q=x';
    const fifty = Array.from({ length: 50 }, (_, i) => makeMemory({ id: `m-${i}`, summary: `hit ${i}` }));
    searchMemoriesSpy.mockResolvedValue({ memories: fifty, searchedScopes: ['repo:test'], scopesTruncated: false, scopesUnknown: false });

    const screen = await renderSearch();
    await expect.element(screen.getByRole('button', { name: 'Show more' })).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Show more' }).click();
    await expect.poll(() => pageState.url.searchParams.get('k')).toBe('100');
    await expect.poll(() => searchMemoriesSpy.mock.calls.some((c) => c[0].k === 100n)).toBe(true);
  });

  it('shows no "Show more" when hits are below k', async () => {
    pageState.url.href = 'http://localhost/search?q=x';
    const thirtySeven = Array.from({ length: 37 }, (_, i) => makeMemory({ id: `m-${i}`, summary: `hit ${i}` }));
    searchMemoriesSpy.mockResolvedValue({ memories: thirtySeven, searchedScopes: ['repo:test'], scopesTruncated: false, scopesUnknown: false });

    const screen = await renderSearch();
    await expect.element(screen.getByText('hit 0')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Show more' })).not.toBeInTheDocument();
  });

  it('shows no "Show more" at the k=1000 ceiling', async () => {
    pageState.url.href = 'http://localhost/search?q=x&k=1000';
    const thousand = Array.from({ length: 1000 }, (_, i) => makeMemory({ id: `m-${i}`, summary: `hit ${i}` }));
    searchMemoriesSpy.mockResolvedValue({ memories: thousand, searchedScopes: ['repo:test'], scopesTruncated: false, scopesUnknown: false });

    const screen = await renderSearch();
    await expect.element(screen.getByText('hit 0')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Show more' })).not.toBeInTheDocument();
  });
});

describe('search route — operator-only input lists memories unranked with infinite scroll (D-09)', () => {
  it('runs ListMemories with the parsed operators, never SearchMemories, and shows the unranked header', async () => {
    pageState.url.href = `http://localhost/search?q=${encodeURIComponent('scope:repo:engram #ci')}`;
    listMemoriesSpy.mockResolvedValue({
      memories: [makeMemory({ id: 'm-1', summary: 'listed one', scope: 'repo:engram', tags: ['ci'] })],
      total: 1n,
      nextPageToken: '',
      searchedScopes: [],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await renderSearch();
    await expect.element(screen.getByText('listed one')).toBeInTheDocument();
    expect(searchMemoriesSpy).not.toHaveBeenCalled();
    expect(listMemoriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({ scope: 'repo:engram', tags: ['ci'], cursorMode: true, limit: 50n, pageToken: '' }),
      expect.objectContaining({ signal: expect.anything() })
    );
    await expect.element(screen.getByText(/unranked \(list — no score\)/)).toBeInTheDocument();
  });

  it('appends a second page when nextPageToken is set, and stops once it is empty', async () => {
    pageState.url.href = `http://localhost/search?q=${encodeURIComponent('is:gotcha')}`;
    listMemoriesSpy
      .mockResolvedValueOnce({
        memories: [makeMemory({ id: 'm-1', summary: 'page one hit', category: 'gotcha' })],
        total: 2n,
        nextPageToken: 't2',
        searchedScopes: ['repo:test'],
        scopesTruncated: false,
        scopesUnknown: false
      })
      .mockResolvedValueOnce({
        memories: [makeMemory({ id: 'm-2', summary: 'page two hit', category: 'gotcha' })],
        total: 2n,
        nextPageToken: '',
        searchedScopes: ['repo:test'],
        scopesTruncated: false,
        scopesUnknown: false
      });

    const screen = await renderSearch();
    await expect.element(screen.getByText('page one hit')).toBeInTheDocument();

    await expect.poll(() => listMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(2);
    expect(listMemoriesSpy.mock.calls[1][0].pageToken).toBe('t2');
    await expect.element(screen.getByText('page two hit')).toBeInTheDocument();

    const callCountAfterSecondPage = listMemoriesSpy.mock.calls.length;
    await new Promise((r) => setTimeout(r, 50));
    expect(listMemoriesSpy.mock.calls.length).toBe(callCountAfterSecondPage);
  });
});

describe('search route — debounce timer cleanup (WR-05)', () => {
  it('clears the pending debounced navigation on unmount so it never fires after teardown', async () => {
    pageState.url.href = 'http://localhost/search?q=';
    const screen = await renderSearch();
    gotoSpy.mockClear();

    const input = screen.getByRole('textbox', { name: 'Search query' });
    await input.fill('late');

    await screen.unmount();
    await new Promise((r) => setTimeout(r, 250));

    expect(gotoSpy).not.toHaveBeenCalled();
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

describe('search route — typing is debounced (ENTRY-06)', () => {
  it('coalesces rapid keystrokes into a single replaceState navigation for the final value', async () => {
    pageState.url.href = 'http://localhost/search?q=';
    const screen = await renderSearch();
    gotoSpy.mockClear();

    const input = screen.getByRole('textbox', { name: 'Search query' }).element() as HTMLInputElement;

    for (const value of ['a', 'ab', 'abc']) {
      input.value = value;
      input.dispatchEvent(new Event('input', { bubbles: true }));
    }

    expect(gotoSpy).not.toHaveBeenCalled();

    await expect.poll(() => gotoSpy.mock.calls.length).toBe(1);
    const [href, opts] = gotoSpy.mock.calls[0];
    expect(href).toContain('q=abc');
    expect(opts).toMatchObject({ replaceState: true });

    await new Promise((r) => setTimeout(r, 250));
    expect(gotoSpy).toHaveBeenCalledTimes(1);
  });
});

describe('search route — archive from the detail pane (CUR-02 tracer)', () => {
  it('archives the open record via CurationSurfaces/ArchiveConfirmDialog and patches the row in place', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const memory = makeMemory({ id: 'm1', summary: 'archive me' });
    searchMemoriesSpy.mockResolvedValue({
      memories: [memory],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    archiveMemorySpy.mockResolvedValue({
      results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED }]
    });

    const screen = await renderSearch();
    // Force the wide (non-overlay) RecallSplit layout so the pane's action
    // row is a normal in-flow element, not the narrow-mode absolute overlay
    // (RecallSplit.browser.test.ts's own convention for reliable clicks).
    screen.container.style.width = '1200px';
    await expect.element(screen.getByText('archive me')).toBeInTheDocument();

    // Open the pane on the m1 row.
    (screen.container.querySelector('[role="option"]') as HTMLElement).click();
    await expect.poll(() => screen.container.querySelector('[aria-label="Memory detail"]') !== null).toBe(true);

    await screen.getByRole('button', { name: 'Archive' }).click();

    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByText('Archive 1 records?')).toBeInTheDocument();
    expect(archiveMemorySpy).not.toHaveBeenCalled();

    await dialog.getByRole('button', { name: 'Archive' }).click();

    await expect.poll(() => archiveMemorySpy.mock.calls.length).toBe(1);
    const [req] = archiveMemorySpy.mock.calls[0];
    expect(req.ids).toEqual(['m1']);

    await dialog.getByRole('button', { name: 'Done' }).click();

    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    await expect.element(listbox.getByText('archive me')).toBeInTheDocument();
    await expect.element(listbox.getByText('archived')).toBeInTheDocument();
  });
});

describe('search route — row toolbar archive (D-05 tracer)', () => {
  it('hovering the m1 row and clicking its toolbar Archive button opens the confirm dialog for exactly that row', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const memory = makeMemory({ id: 'm1', summary: 'archive me' });
    searchMemoriesSpy.mockResolvedValue({
      memories: [memory],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    archiveMemorySpy.mockResolvedValue({
      results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED }]
    });

    const screen = await renderSearch();
    // Force the wide layout and wait for the ResizeObserver flip to settle,
    // same convention as the bulk-archive tracer below — a real pointer
    // hover needs a settled, non-zero-height row to land on.
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('archive me')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    const row = screen.container.querySelector('[role="option"]') as HTMLElement;
    await page.elementLocator(row).hover();

    const archiveBtn = screen.getByRole('button', { name: `Archive ${memory.shortId}` });
    await expect.element(archiveBtn).toBeInTheDocument();
    await archiveBtn.click();

    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByText('Archive 1 records?')).toBeInTheDocument();
    expect(archiveMemorySpy).not.toHaveBeenCalled();

    // The row action must not have navigated `sel` to open the detail pane.
    expect(pageState.url.searchParams.get('sel')).toBeFalsy();

    await dialog.getByRole('button', { name: 'Archive' }).click();
    await expect.poll(() => archiveMemorySpy.mock.calls.length).toBe(1);
    const [req] = archiveMemorySpy.mock.calls[0];
    expect(req.ids).toEqual(['m1']);
  });
});

describe('search route — bulk archive with x and a (D-01..D-03 tracer)', () => {
  it('selects two rows with x/j/x, archives them together through the confirm dialog, and clears the selection', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const m1 = makeMemory({ id: 'm1', summary: 'hit one' });
    const m2 = makeMemory({ id: 'm2', summary: 'hit two' });
    searchMemoriesSpy.mockResolvedValue({
      memories: [m1, m2],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    archiveMemorySpy.mockResolvedValue({
      results: [
        { requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED },
        { requested: 'm2', id: 'm2', outcome: ArchiveOutcome.ARCHIVED }
      ]
    });

    const screen = await renderSearch();
    // j needs a real box for SvelteVirtualList's scroll() to resolve (see
    // ResultsList.browser.test.ts's pointer-hover test for the same note).
    // Force the wide (non-overlay) RecallSplit layout (same convention as
    // the CUR-02 tracer test above) and wait for the ResizeObserver-driven
    // narrow->wide flip to actually settle before focusing anything -- the
    // flip unmounts/remounts ResultsList, and a stale focus() on a node from
    // the since-torn-down narrow branch silently blurs to <body>.
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('hit one')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m1');

    await userEvent.keyboard('x');
    await userEvent.keyboard('j');
    // moveActive resolves asynchronously (list.scroll) — wait for the active
    // row to actually land on m2 before the second x, or it would toggle m1
    // again against a stale activeIndex.
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m2');
    await userEvent.keyboard('x');
    await userEvent.keyboard('a');

    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByText('Archive 2 records?')).toBeInTheDocument();
    expect(archiveMemorySpy).not.toHaveBeenCalled();

    await dialog.getByRole('button', { name: 'Archive' }).click();
    await expect.poll(() => archiveMemorySpy.mock.calls.length).toBe(1);
    const [req] = archiveMemorySpy.mock.calls[0];
    expect(req.ids).toEqual(['m1', 'm2']);

    await dialog.getByRole('button', { name: 'Done' }).click();

    await expect.element(listbox.getByText('hit one')).toBeInTheDocument();
    await expect.element(listbox.getByText('hit two')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelectorAll('.states .st').length).toBe(2);

    const options = screen.container.querySelectorAll('[role="option"]');
    expect(options.length).toBe(2);
    for (const opt of options) {
      expect(opt.getAttribute('aria-selected')).toBe('false');
    }
  });
});

describe('search route — selection lifecycle (D-04)', () => {
  async function renderWideWithSelection(memories: Memory[]) {
    searchMemoriesSpy.mockResolvedValue({
      memories,
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    const screen = await renderSearch();
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText(memories[0].summary)).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await userEvent.keyboard('x');
    await expect
      .poll(() => screen.container.querySelectorAll('[role="option"][aria-selected="true"]').length)
      .toBe(1);
    return { screen, listbox };
  }

  it('changing the query clears the selection', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const { screen } = await renderWideWithSelection([makeMemory({ id: 'm1', summary: 'hit one' })]);

    const input = screen.getByRole('textbox', { name: 'Search query' }).element() as HTMLInputElement;
    input.value = 'other';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    await expect.poll(() => pageState.url.searchParams.get('q')).toBe('other');

    await expect
      .poll(() => screen.container.querySelectorAll('[role="option"][aria-selected="true"]').length)
      .toBe(0);
  });

  it('clicking Show more keeps the selection', async () => {
    pageState.url.href = 'http://localhost/search?q=x';
    const fifty = Array.from({ length: 50 }, (_, i) => makeMemory({ id: `m-${i}`, summary: `hit ${i}` }));
    const { screen } = await renderWideWithSelection(fifty);
    await expect.element(screen.getByRole('button', { name: 'Show more' })).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Show more' }).click();
    await expect.poll(() => pageState.url.searchParams.get('k')).toBe('100');

    await expect
      .poll(() => screen.container.querySelectorAll('[role="option"][aria-selected="true"]').length)
      .toBe(1);
  });

  it('a rejected archive keeps the selection', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    archiveMemorySpy.mockRejectedValue(new ConnectError('boom', Code.Internal));
    const { screen, listbox } = await renderWideWithSelection([makeMemory({ id: 'm1', summary: 'hit one' })]);

    await userEvent.keyboard('a');
    const dialog = screen.getByRole('dialog');
    await dialog.getByRole('button', { name: 'Archive' }).click();
    await expect.poll(() => archiveMemorySpy.mock.calls.length).toBe(1);
    await expect.element(dialog.getByText(/Could not archive/)).toBeInTheDocument();

    await expect
      .poll(() => listbox.element().querySelectorAll('[role="option"][aria-selected="true"]').length)
      .toBe(1);
  });

  it('the URL never contains the selected ids', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    await renderWideWithSelection([makeMemory({ id: 'm1', summary: 'hit one' })]);

    expect(pageState.url.toString()).not.toContain('m1');
    expect(pageState.url.searchParams.has('sel')).toBe(false);
  });
});

describe('search route — supersede with ⇧S (CUR-01 tracer)', () => {
  it('selects m1 and m2, ⇧S opens the supersede dialog, commits once, and both rows dim in place as superseded', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const m1 = makeMemory({ id: 'm1', shortId: 'S0000000001', summary: 'hit one', content: 'full content one' });
    const m2 = makeMemory({ id: 'm2', shortId: 'S0000000002', summary: 'hit two', content: 'full content two' });
    searchMemoriesSpy.mockResolvedValue({
      memories: [m1, m2],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    getMemorySpy.mockImplementation(async ({ id }: { id: string }) => {
      if (id === 'm1') return { memory: m1 };
      if (id === 'm2') return { memory: m2 };
      return { memory: undefined };
    });
    supersedeMemorySpy.mockImplementation(async (req: SupersedeMemoryRequest) => {
      if (req.validateOnly) {
        return { id: '', shortId: '', validated: true, supersedes: ['m1', 'm2'], targets: [] };
      }
      return { id: 'n1', shortId: 'N1SHORT0001', validated: false, supersedes: [], targets: [] };
    });

    const screen = await renderSearch();
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('hit one')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m1');

    await userEvent.keyboard('x');
    await userEvent.keyboard('j');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m2');
    await userEvent.keyboard('x');
    await userEvent.keyboard('S');

    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByText('Supersede 2 records into one')).toBeInTheDocument();
    expect(supersedeMemorySpy.mock.calls.filter((c) => (c[0] as SupersedeMemoryRequest).validateOnly === false)).toHaveLength(0);

    const submitBtn = screen.getByRole('button', { name: 'Supersede 2 → 1' });
    await expect.element(submitBtn).not.toBeDisabled();

    await submitBtn.click();

    const commitCalls = () =>
      supersedeMemorySpy.mock.calls.filter((c) => (c[0] as SupersedeMemoryRequest).validateOnly === false);
    await expect.poll(() => commitCalls().length).toBe(1);
    const commitReq = commitCalls()[0][0] as SupersedeMemoryRequest;
    expect(commitReq.supersedes).toEqual(['m1', 'm2']);

    await dialog.getByRole('button', { name: 'Done' }).click();

    await expect.element(listbox.getByText('hit one')).toBeInTheDocument();
    await expect.element(listbox.getByText('hit two')).toBeInTheDocument();
    const rows = screen.container.querySelectorAll('[role="option"]');
    expect(rows.length).toBe(2);
    for (const row of rows) {
      await expect.element(row.querySelector<HTMLElement>('.states')).toHaveTextContent('superseded');
      expect(row.querySelector('.sum')?.classList.contains('dim')).toBe(true);
    }
  });
});

describe('search route — curation resume reopen (CUR-05, D-06, D-15/D-16)', () => {
  it('reopens the supersede dialog from a seeded envelope on mount, runs exactly one validate_only preview, no commit, and consumes once', async () => {
    persistResume({
      returnPath: '/search?q=foo',
      kind: 'supersede',
      targets: ['m1'],
      fields: { summary: 's', content: 'c', category: 'convention', scope: '', tags: [] },
      idempotencyKey: 'abc123'
    });
    getMemorySpy.mockResolvedValue({ memory: makeMemory({ id: 'm1', shortId: 'S0000000001' }) });
    supersedeMemorySpy.mockImplementation(async (req: SupersedeMemoryRequest) => {
      if (req.validateOnly) return { id: '', shortId: '', validated: true, supersedes: ['m1'], targets: [] };
      return { id: 'n1', shortId: 'n1short0000', validated: false, supersedes: [], targets: [] };
    });

    const screen = await renderSearch();
    await expect.element(screen.getByText('Signed in again — review and resend')).toBeInTheDocument();

    const previewCalls = () =>
      supersedeMemorySpy.mock.calls.filter((c) => (c[0] as SupersedeMemoryRequest).validateOnly === true);
    await expect.poll(() => previewCalls().length).toBe(1);
    const commitCalls = supersedeMemorySpy.mock.calls.filter(
      (c) => (c[0] as SupersedeMemoryRequest).validateOnly === false
    );
    expect(commitCalls).toHaveLength(0);
    await expect.poll(() => consumeResumeSpy.mock.calls.length).toBe(1);
  });

  it('reopens the archive confirm from a seeded archive envelope with no ArchiveMemory call, consumed once', async () => {
    persistResume({ returnPath: '/search', kind: 'archive', mode: 'archive', ids: ['m1'] });
    getMemorySpy.mockResolvedValue({ memory: makeMemory({ id: 'm1' }) });

    const screen = await renderSearch();
    await expect.element(screen.getByText('Signed in again — review and resend')).toBeInTheDocument();
    expect(archiveMemorySpy).not.toHaveBeenCalled();
    await expect.poll(() => consumeResumeSpy.mock.calls.length).toBe(1);
  });
});

describe('search route — chain dialog entry points (D-06)', () => {
  it('a row toolbar Chain button opens the chain dialog for that anchor', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const m1 = makeMemory({ id: 'm1', shortId: 'S0000000001', summary: 'hit one', supersededBy: 'n1' });
    searchMemoriesSpy.mockResolvedValue({
      memories: [m1],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    relatedMemoriesSpy.mockResolvedValue(create(RelatedMemoriesResponseSchema, { anchor: m1, related: [] }));

    const screen = await renderSearch();
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('hit one')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    const row = screen.container.querySelector('[role="option"]') as HTMLElement;
    await page.elementLocator(row).hover();
    const chainBtn = screen.getByRole('button', { name: `Chain ${m1.shortId}` });
    await expect.element(chainBtn).toBeInTheDocument();
    await chainBtn.click();

    await expect.element(screen.getByText(`Chain · ${m1.shortId}`)).toBeInTheDocument();
    // The row action must not have navigated `sel` to open the detail pane.
    expect(pageState.url.searchParams.get('sel')).toBeFalsy();
  });

  it("the pane's View chain link opens the chain dialog for the selected record", async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const m1 = makeMemory({ id: 'm1', shortId: 'S0000000001', summary: 'hit one', supersededBy: 'n1' });
    searchMemoriesSpy.mockResolvedValue({
      memories: [m1],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    relatedMemoriesSpy.mockResolvedValue(create(RelatedMemoriesResponseSchema, { anchor: m1, related: [] }));

    const screen = await renderSearch();
    screen.container.style.width = '1200px';
    await expect.element(screen.getByText('hit one')).toBeInTheDocument();

    (screen.container.querySelector('[role="option"]') as HTMLElement).click();
    await expect.poll(() => screen.container.querySelector('[aria-label="Memory detail"]') !== null).toBe(true);

    await screen.getByRole('button', { name: 'View chain' }).click();
    await expect.element(screen.getByText(`Chain · ${m1.shortId}`)).toBeInTheDocument();
  });
});

describe('search route — supersede success footer (D-06, D-07)', () => {
  async function commitSupersedeOfM1(screen: Awaited<ReturnType<typeof renderSearch>>) {
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await userEvent.keyboard('x');
    await userEvent.keyboard('S');

    const dialog = screen.getByRole('dialog');
    const submitBtn = screen.getByRole('button', { name: 'Supersede 1 → 1' });
    await expect.element(submitBtn).not.toBeDisabled();
    await submitBtn.click();
    await expect.element(dialog.getByText(/No undo\./)).toBeInTheDocument();
    return dialog;
  }

  it('"View superseded (1)" closes the dialog, turns on include-superseded and flashes the predecessor', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const m1 = makeMemory({ id: 'm1', shortId: 'S0000000001', summary: 'hit one', content: 'c1' });
    searchMemoriesSpy.mockResolvedValue({
      memories: [m1],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    getMemorySpy.mockResolvedValue({ memory: m1 });
    supersedeMemorySpy.mockImplementation(async (req: SupersedeMemoryRequest) => {
      if (req.validateOnly) return { id: '', shortId: '', validated: true, supersedes: ['m1'], targets: [] };
      return { id: 'n1', shortId: 'N1SHORT0001', validated: false, supersedes: [], targets: [] };
    });

    const screen = await renderSearch();
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('hit one')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    const dialog = await commitSupersedeOfM1(screen);
    await dialog.getByRole('button', { name: 'View superseded (1)' }).click();

    await expect.poll(() => pageState.url.searchParams.getAll('inc')).toEqual(['superseded']);
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
  });

  it('"Open {short_id}" closes the dialog and navigates sel to the new record', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const m1 = makeMemory({ id: 'm1', shortId: 'S0000000001', summary: 'hit one', content: 'c1' });
    searchMemoriesSpy.mockResolvedValue({
      memories: [m1],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    getMemorySpy.mockResolvedValue({ memory: m1 });
    supersedeMemorySpy.mockImplementation(async (req: SupersedeMemoryRequest) => {
      if (req.validateOnly) return { id: '', shortId: '', validated: true, supersedes: ['m1'], targets: [] };
      return { id: 'n1', shortId: 'N1SHORT0001', validated: false, supersedes: [], targets: [] };
    });

    const screen = await renderSearch();
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('hit one')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    const dialog = await commitSupersedeOfM1(screen);
    await dialog.getByRole('button', { name: 'Open N1SHORT0001' }).click();

    await expect.poll(() => pageState.url.searchParams.get('sel')).toBe('n1');
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
  });
});

describe('search route — bulk-bar busy during a pending curation call (E4)', () => {
  it('disables the bulk bar verbs while an archive commit is in flight', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const m1 = makeMemory({ id: 'm1', summary: 'hit one' });
    searchMemoriesSpy.mockResolvedValue({
      memories: [m1],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    let resolveArchive!: (v: unknown) => void;
    archiveMemorySpy.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveArchive = resolve;
        })
    );

    const screen = await renderSearch();
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('hit one')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await userEvent.keyboard('x');
    await userEvent.keyboard('a');

    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByText('Archive 1 records?')).toBeInTheDocument();
    const bulkToolbar = screen.getByRole('toolbar', { name: 'Bulk actions' });
    await expect.element(bulkToolbar.getByRole('button', { name: /Archive/ })).not.toBeDisabled();

    await dialog.getByRole('button', { name: 'Archive' }).click();
    await expect.poll(() => archiveMemorySpy.mock.calls.length).toBe(1);
    await expect.element(bulkToolbar.getByRole('button', { name: /Archive/ })).toBeDisabled();

    // Once the archive settles, CurationSurfaces' onchanged clears the
    // selection -- the bulk bar (and its now-stale busy state) disappears
    // entirely rather than staying visibly re-enabled.
    resolveArchive({ results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED }] });
    await expect.element(bulkToolbar).not.toBeInTheDocument();
  });
});

describe('search route — docked Tags panel toggle (TAGS-01, D-13)', () => {
  it('starts closed; clicking "▦ Tags panel" opens it and draws bars from ListTags(scope="", limit=1000)', async () => {
    pageState.url.href = 'http://localhost/search';
    const screen = await renderSearch();
    screen.container.style.width = '1200px';

    const toggle = screen.getByRole('button', { name: '▦ Tags panel' });
    await expect.element(toggle).toHaveAttribute('aria-pressed', 'false');

    await toggle.click();
    await expect.element(toggle).toHaveAttribute('aria-pressed', 'true');

    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
    expect(listTagsSpy).toHaveBeenCalledWith({ scope: '', limit: 1000n }, expect.anything());
    await expect.element(screen.getByText('Tags · counts in all readable scopes')).toBeInTheDocument();
    await expect.element(screen.getByText('qdrant')).toBeInTheDocument();
  });

  it('with ?scope=repo:acme/x set, the panel calls ListTags with that scope and names it in the header', async () => {
    pageState.url.href = 'http://localhost/search?scope=repo:acme/x';
    const screen = await renderSearch();
    screen.container.style.width = '1200px';

    await screen.getByRole('button', { name: '▦ Tags panel' }).click();

    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
    expect(listTagsSpy).toHaveBeenCalledWith({ scope: 'repo:acme/x', limit: 1000n }, expect.anything());
    await expect.element(screen.getByText('Tags · counts in repo:acme/x')).toBeInTheDocument();
  });

  it('clicking the "qdrant" bar adds it as a #tag URL filter with sel cleared; clicking it again removes it', async () => {
    pageState.url.href = 'http://localhost/search';
    const screen = await renderSearch();
    screen.container.style.width = '1200px';

    await screen.getByRole('button', { name: '▦ Tags panel' }).click();
    await expect.element(screen.getByText('qdrant')).toBeInTheDocument();

    screen.getByRole('option', { name: /qdrant/ }).element().dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await expect.poll(() => pageState.url.searchParams.getAll('tag')).toEqual(['qdrant']);
    expect(pageState.url.searchParams.get('sel')).toBe(null);
    expect(gotoSpy.mock.calls.at(-1)?.[0]).toMatch(/^\/ui\/search\?/);

    screen.getByRole('option', { name: /qdrant/ }).element().dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await expect.poll(() => pageState.url.searchParams.getAll('tag')).toEqual([]);
  });
});

describe('search route — Tags panel shares the slot with the detail pane, narrow sheet (D-13)', () => {
  afterEach(async () => {
    await page.viewport(1024, 800);
  });

  it('opening a record hides the panel; closing the record shows the panel again', async () => {
    pageState.url.href = 'http://localhost/search?q=github';
    const memory = makeMemory({ id: 'm-1', summary: 'toggle panel record' });
    searchMemoriesSpy.mockResolvedValue({ memories: [memory], searchedScopes: ['repo:test'], scopesTruncated: false, scopesUnknown: false });
    getMemorySpy.mockResolvedValue({ memory });

    const screen = await renderSearch();
    screen.container.style.width = '1200px';
    await screen.getByRole('button', { name: '▦ Tags panel' }).click();
    await expect.element(screen.getByText('Tags · counts in all readable scopes')).toBeInTheDocument();

    (screen.container.querySelector('[role="option"]') as HTMLElement).click();
    await expect.poll(() => screen.container.querySelector('[aria-label="Memory detail"]') !== null).toBe(true);
    await expect.element(screen.getByText('Tags · counts in all readable scopes')).not.toBeInTheDocument();

    await screen.getByRole('button', { name: 'close' }).click();
    await expect.element(screen.getByText('Tags · counts in all readable scopes')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: '▦ Tags panel' })).toHaveAttribute('aria-pressed', 'true');
  });

  it("the slot's close control with no record open turns the toggle off", async () => {
    pageState.url.href = 'http://localhost/search';
    const screen = await renderSearch();
    screen.container.style.width = '1200px';

    await screen.getByRole('button', { name: '▦ Tags panel' }).click();
    await expect.element(screen.getByText('Tags · counts in all readable scopes')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'close' }).click();
    await expect.element(screen.getByRole('button', { name: '▦ Tags panel' })).toHaveAttribute('aria-pressed', 'false');
    await expect.element(screen.getByText('Tags · counts in all readable scopes')).not.toBeInTheDocument();
  });

  it('an active tag from ?tag=qdrant renders marked with the ● marker and aria-selected', async () => {
    pageState.url.href = 'http://localhost/search?tag=qdrant';
    const screen = await renderSearch();
    screen.container.style.width = '1200px';

    await screen.getByRole('button', { name: '▦ Tags panel' }).click();
    const row = screen.getByRole('option', { name: /qdrant/ });
    await expect.element(row).toHaveAttribute('aria-selected', 'true');
    await expect.element(row.getByText('●')).toBeInTheDocument();
  });

  it('at a 700px-wide viewport the panel opens as a bottom sheet instead of the slot; closing it clears the toggle', async () => {
    await page.viewport(700, 800);
    pageState.url.href = 'http://localhost/search';
    const screen = await renderSearch();

    await screen.getByRole('button', { name: '▦ Tags panel' }).click();
    const dialog = screen.getByRole('dialog');
    await expect.element(dialog).toBeInTheDocument();
    await expect.element(dialog.getByText('qdrant')).toBeInTheDocument();

    await dialog.getByRole('button', { name: 'Close' }).click();
    await expect.element(screen.getByRole('button', { name: '▦ Tags panel' })).toHaveAttribute('aria-pressed', 'false');
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
  });

  it('captures the docked panel and the narrow sheet in both themes', async () => {
    // Both phases drive the real viewport (not an inline container width) --
    // an explicit CSS width set on the mount root would keep overriding
    // RecallSplit's own ResizeObserver measurement after the later
    // page.viewport(700, ...) call, since a literal px width is not
    // viewport-relative.
    await page.viewport(1200, 800);
    pageState.url.href = 'http://localhost/search';
    const screen = await renderSearch();
    await screen.getByRole('button', { name: '▦ Tags panel' }).click();
    await expect.element(screen.getByText('Tags · counts in all readable scopes')).toBeInTheDocument();

    document.documentElement.classList.remove('dark');
    await page.screenshot();
    document.documentElement.classList.add('dark');
    await page.screenshot();
    document.documentElement.classList.remove('dark');

    await page.viewport(700, 800);
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await page.screenshot();
    document.documentElement.classList.add('dark');
    await page.screenshot();
    document.documentElement.classList.remove('dark');
  });
});

describe('search route — suggested filters (NLQ-03)', () => {
  it('accepts a suggested category chip to the same URL and SearchMemories calls the manual FacetStrip chip produces', async () => {
    const startUrl = 'http://localhost/search?q=what%20did%20we%20decide';
    const understandResponse = create(UnderstandQueryResponseSchema, {
      enabled: true,
      suggestions: [{ kind: { case: 'category', value: 'decision' }, source: SuggestionSource.DECIDED }]
    });
    understandQuerySpy.mockReset().mockResolvedValue(understandResponse);
    pageState.url.href = startUrl;

    const screen = await renderSearch();
    const chip = screen.getByRole('button', { name: 'Suggested filter, not applied: decision' });
    await expect.element(chip).toBeInTheDocument();

    // D-10/D-11: nothing changes on arrival — the suggested chip renders but
    // the SearchMemories request is unchanged until an explicit accept.
    const callsBeforeAccept = searchMemoriesSpy.mock.calls.length;
    expect(searchMemoriesSpy.mock.calls.every((c) => c[0].categories.length === 0)).toBe(true);
    expect(understandQuerySpy).toHaveBeenCalledTimes(1);
    const [understandReq, understandOpts] = understandQuerySpy.mock.calls[0];
    expect(understandReq).toEqual(
      expect.objectContaining({
        query: 'what did we decide',
        categories: [],
        tags: [],
        scope: '',
        crossSpine: true,
        createdAfter: '',
        createdBefore: ''
      })
    );
    expect(understandOpts?.signal).toBeInstanceOf(AbortSignal);

    await chip.click();
    await expect.poll(() => pageState.url.searchParams.get('cat')).toBe('decision');
    const suggestedHref = pageState.url.href;
    const suggestedCalls = searchMemoriesSpy.mock.calls.slice(callsBeforeAccept);

    await screen.unmount();
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    pageState.url.href = startUrl;
    searchMemoriesSpy.mockReset().mockResolvedValue(emptySearchResult());
    understandQuerySpy.mockReset().mockResolvedValue(understandResponse);

    const screen2 = await renderSearch();
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    const manualCallsBefore = searchMemoriesSpy.mock.calls.length;
    await screen2.getByRole('button', { name: /^decision \d+$/ }).click();
    await expect.poll(() => pageState.url.searchParams.get('cat')).toBe('decision');
    const manualHref = pageState.url.href;
    const manualCalls = searchMemoriesSpy.mock.calls.slice(manualCallsBefore);

    expect(suggestedHref).toBe(manualHref);
    expect(suggestedCalls).toEqual(manualCalls);
  });

  it('accepts a suggested time_window chip to the same URL and SearchMemories calls the manual created-after date input produces', async () => {
    const startUrl = 'http://localhost/search?q=what%20did%20we%20decide';
    const understandResponse = create(UnderstandQueryResponseSchema, {
      enabled: true,
      suggestions: [
        {
          kind: { case: 'timeWindow', value: { createdAfter: '2026-09-21T00:00:00Z', createdBefore: '', label: 'past week' } },
          source: SuggestionSource.DECIDED
        }
      ]
    });
    understandQuerySpy.mockReset().mockResolvedValue(understandResponse);
    pageState.url.href = startUrl;

    const screen = await renderSearch();
    const chip = screen.getByRole('button', { name: 'Suggested filter, not applied: past week' });
    await expect.element(chip).toBeInTheDocument();
    const callsBeforeAccept = searchMemoriesSpy.mock.calls.length;
    await chip.click();
    await expect.poll(() => pageState.url.searchParams.get('after')).toBe('2026-09-21T00:00:00Z');
    const suggestedHref = pageState.url.href;
    const suggestedCalls = searchMemoriesSpy.mock.calls.slice(callsBeforeAccept);

    await screen.unmount();
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    pageState.url.href = startUrl;
    searchMemoriesSpy.mockReset().mockResolvedValue(emptySearchResult());
    understandQuerySpy.mockReset().mockResolvedValue({ enabled: false, suggestions: [] });

    const screen2 = await renderSearch();
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    const manualCallsBefore = searchMemoriesSpy.mock.calls.length;
    await screen2.getByRole('button', { name: 'created window' }).click();
    await screen2.getByLabelText('created after').fill('2026-09-21');
    await expect.poll(() => pageState.url.searchParams.get('after')).toBe('2026-09-21T00:00:00Z');
    const manualHref = pageState.url.href;
    const manualCalls = searchMemoriesSpy.mock.calls.slice(manualCallsBefore);

    expect(suggestedHref).toBe(manualHref);
    expect(suggestedCalls).toEqual(manualCalls);
  });

  it('accepts a suggested scope chip to the same URL and SearchMemories calls the manual ScopeCombobox selection produces', async () => {
    const startUrl = 'http://localhost/search?q=what%20did%20we%20decide';
    const understandResponse = create(UnderstandQueryResponseSchema, {
      enabled: true,
      suggestions: [{ kind: { case: 'scope', value: 'repo:acme/x' }, source: SuggestionSource.DECIDED }]
    });
    listScopesSpy.mockReset().mockResolvedValue({ scopes: [{ scope: 'repo:acme/x', count: 5n }], approximate: false });
    understandQuerySpy.mockReset().mockResolvedValue(understandResponse);
    pageState.url.href = startUrl;

    const screen = await renderSearch();
    const chip = screen.getByRole('button', { name: 'Suggested filter, not applied: repo:acme/x' });
    await expect.element(chip).toBeInTheDocument();
    const callsBeforeAccept = searchMemoriesSpy.mock.calls.length;
    await chip.click();
    await expect.poll(() => pageState.url.searchParams.get('scope')).toBe('repo:acme/x');
    const suggestedHref = pageState.url.href;
    const suggestedCalls = searchMemoriesSpy.mock.calls.slice(callsBeforeAccept);

    await screen.unmount();
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    pageState.url.href = startUrl;
    searchMemoriesSpy.mockReset().mockResolvedValue(emptySearchResult());
    understandQuerySpy.mockReset().mockResolvedValue({ enabled: false, suggestions: [] });

    const screen2 = await renderSearch();
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    const manualCallsBefore = searchMemoriesSpy.mock.calls.length;
    await screen2.getByRole('button', { name: /any scope/ }).click();
    await screen2.getByText('repo:acme/x').click();
    await expect.poll(() => pageState.url.searchParams.get('scope')).toBe('repo:acme/x');
    const manualHref = pageState.url.href;
    const manualCalls = searchMemoriesSpy.mock.calls.slice(manualCallsBefore);

    expect(suggestedHref).toBe(manualHref);
    expect(suggestedCalls).toEqual(manualCalls);
  });

  it('accepts a suggested tag chip to the same URL and SearchMemories calls the manual Tags panel bar click produces', async () => {
    const startUrl = 'http://localhost/search?q=what%20did%20we%20decide';
    const understandResponse = create(UnderstandQueryResponseSchema, {
      enabled: true,
      suggestions: [{ kind: { case: 'tag', value: 'qdrant' }, source: SuggestionSource.MATCHED }]
    });
    understandQuerySpy.mockReset().mockResolvedValue(understandResponse);
    pageState.url.href = startUrl;

    const screen = await renderSearch();
    const chip = screen.getByRole('button', { name: 'Suggested filter, not applied: #qdrant' });
    await expect.element(chip).toBeInTheDocument();
    const callsBeforeAccept = searchMemoriesSpy.mock.calls.length;
    await chip.click();
    await expect.poll(() => pageState.url.searchParams.get('tag')).toBe('qdrant');
    const suggestedHref = pageState.url.href;
    const suggestedCalls = searchMemoriesSpy.mock.calls.slice(callsBeforeAccept);

    await screen.unmount();
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    pageState.url.href = startUrl;
    searchMemoriesSpy.mockReset().mockResolvedValue(emptySearchResult());
    understandQuerySpy.mockReset().mockResolvedValue({ enabled: false, suggestions: [] });

    const screen2 = await renderSearch();
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    const manualCallsBefore = searchMemoriesSpy.mock.calls.length;
    await screen2.getByRole('button', { name: '▦ Tags panel' }).click();
    await screen2.getByRole('option', { name: /^#qdrant/ }).click();
    await expect.poll(() => pageState.url.searchParams.get('tag')).toBe('qdrant');
    const manualHref = pageState.url.href;
    const manualCalls = searchMemoriesSpy.mock.calls.slice(manualCallsBefore);

    expect(suggestedHref).toBe(manualHref);
    expect(suggestedCalls).toEqual(manualCalls);
  });

  it('hides a category suggestion already reflected in the URL while other kinds still render', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide&cat=decision';
    understandQuerySpy.mockReset().mockResolvedValue({
      enabled: true,
      suggestions: [
        { kind: { case: 'category', value: 'decision' }, source: SuggestionSource.DECIDED },
        { kind: { case: 'tag', value: 'qdrant' }, source: SuggestionSource.MATCHED }
      ]
    });
    const screen = await renderSearch();
    await expect.element(screen.getByRole('button', { name: 'Suggested filter, not applied: #qdrant' })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Suggested filter, not applied: decision' })).not.toBeInTheDocument();
  });

  it('hides a scope suggestion while any scope is applied', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide&scope=repo%3Aother';
    understandQuerySpy.mockReset().mockResolvedValue({
      enabled: true,
      suggestions: [{ kind: { case: 'scope', value: 'repo:acme/x' }, source: SuggestionSource.DECIDED }]
    });
    const screen = await renderSearch();
    await new Promise((r) => setTimeout(r, 20));
    await expect.element(screen.getByRole('toolbar', { name: 'Suggested filters' })).not.toBeInTheDocument();
  });

  it('hides a time_window suggestion while any created bound is applied', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide&after=2020-01-01T00%3A00%3A00Z';
    understandQuerySpy.mockReset().mockResolvedValue({
      enabled: true,
      suggestions: [
        { kind: { case: 'timeWindow', value: { createdAfter: '2026-09-21T00:00:00Z', createdBefore: '', label: 'past week' } }, source: SuggestionSource.DECIDED }
      ]
    });
    const screen = await renderSearch();
    await new Promise((r) => setTimeout(r, 20));
    await expect.element(screen.getByRole('toolbar', { name: 'Suggested filters' })).not.toBeInTheDocument();
  });

  it('dismissing a chip is per-q, never touches the URL, and forgets the dismissal on a round trip through another q', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide';
    understandQuerySpy.mockReset().mockResolvedValue({
      enabled: true,
      suggestions: [{ kind: { case: 'category', value: 'decision' }, source: SuggestionSource.DECIDED }]
    });
    const screen = await renderSearch();
    const chip = screen.getByRole('button', { name: 'Suggested filter, not applied: decision' });
    await expect.element(chip).toBeInTheDocument();

    gotoSpy.mockClear();
    const callsBefore = searchMemoriesSpy.mock.calls.length;
    await screen.getByRole('button', { name: 'Dismiss suggested filter: decision' }).click();
    await expect.element(chip).not.toBeInTheDocument();
    expect(gotoSpy).not.toHaveBeenCalled();
    expect(searchMemoriesSpy.mock.calls.length).toBe(callsBefore);

    pageState.url.href = 'http://localhost/search?q=totally%20different%20query';
    await new Promise((r) => setTimeout(r, 20));
    const callsForQ2 = understandQuerySpy.mock.calls.length;

    // Coming back to q1 must be a cache hit (staleTime: Infinity, bare-q key)
    // — no additional UnderstandQuery call — and the dismissed chip must
    // reappear, since the dismissal was scoped to q1 and forgotten the
    // moment q changed away from it (never restored on a later return to
    // that same q string).
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide';
    await expect.element(screen.getByRole('button', { name: 'Suggested filter, not applied: decision' })).toBeInTheDocument();
    expect(understandQuerySpy.mock.calls.length).toBe(callsForQ2);
  });

  it('never calls UnderstandQuery for an id, a short_id, an operator-bearing query, or a single word', async () => {
    understandQuerySpy.mockReset().mockResolvedValue({ enabled: true, suggestions: [] });
    for (const q of ['753aba22-0000-4000-8000-000000000001', 'k3m9p2qr7a', '#ci flaky test', 'qdrant']) {
      understandQuerySpy.mockClear();
      pageState.url.href = `http://localhost/search?q=${encodeURIComponent(q)}`;
      const screen = await renderSearch();
      await new Promise((r) => setTimeout(r, 20));
      expect(understandQuerySpy).not.toHaveBeenCalled();
      await screen.unmount();
      qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    }
  });

  it('debounces keystrokes into a single UnderstandQuery call for the final committed text', async () => {
    pageState.url.href = 'http://localhost/search?q=';
    understandQuerySpy.mockReset().mockResolvedValue({ enabled: true, suggestions: [] });
    const screen = await renderSearch();
    const input = screen.getByRole('textbox', { name: 'Search query' }).element() as HTMLInputElement;

    for (const value of ['w', 'wh', 'what', 'what did we decide']) {
      input.value = value;
      input.dispatchEvent(new Event('input', { bubbles: true }));
    }

    await new Promise((r) => setTimeout(r, 250));
    expect(pageState.url.searchParams.get('q')).toBe('what did we decide');
    expect(understandQuerySpy).toHaveBeenCalledTimes(1);
    expect(understandQuerySpy.mock.calls[0][0].query).toBe('what did we decide');
  });

  it('latches off after the first {enabled:false} response and never calls again for a later eligible q', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide';
    understandQuerySpy.mockReset().mockResolvedValue({ enabled: false, suggestions: [] });
    const screen = await renderSearch();
    await expect.poll(() => understandQuerySpy.mock.calls.length).toBe(1);
    await expect.element(screen.getByRole('toolbar', { name: 'Suggested filters' })).not.toBeInTheDocument();

    pageState.url.searchParams.set('q', 'another eligible query');
    await new Promise((r) => setTimeout(r, 20));
    expect(understandQuerySpy).toHaveBeenCalledTimes(1);
  });

  it('discards a stale response for a superseded q — the earlier response never renders once q has moved on', async () => {
    pageState.url.href = 'http://localhost/search?q=alpha%20query';
    let resolveAlpha!: (v: unknown) => void;
    const alphaPromise = new Promise((resolve) => {
      resolveAlpha = resolve;
    });
    understandQuerySpy.mockReset().mockImplementation((req: { query: string }) => {
      if (req.query === 'alpha query') return alphaPromise;
      return Promise.resolve({
        enabled: true,
        suggestions: [{ kind: { case: 'category', value: 'gotcha' }, source: SuggestionSource.DECIDED }]
      });
    });

    const screen = await renderSearch();
    await expect.poll(() => understandQuerySpy.mock.calls.some((c) => c[0].query === 'alpha query')).toBe(true);

    pageState.url.searchParams.set('q', 'beta query');
    await expect.element(screen.getByRole('button', { name: 'Suggested filter, not applied: gotcha' })).toBeInTheDocument();

    resolveAlpha({
      enabled: true,
      suggestions: [{ kind: { case: 'category', value: 'convention' }, source: SuggestionSource.DECIDED }]
    });
    await new Promise((r) => setTimeout(r, 30));

    await expect.element(screen.getByRole('button', { name: 'Suggested filter, not applied: convention' })).not.toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Suggested filter, not applied: gotcha' })).toBeInTheDocument();
  });

  it('UnderstandQuery and SearchMemories run independently — neither blocks the other from rendering', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide';
    understandQuerySpy.mockReset().mockReturnValue(new Promise(() => {}));
    searchMemoriesSpy.mockReset().mockResolvedValue({
      memories: [makeMemory({ id: 'm-x', summary: 'a hit while understanding never resolves' })],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    const screen = await renderSearch();
    await expect.element(screen.getByText('a hit while understanding never resolves')).toBeInTheDocument();
    await screen.unmount();
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    understandQuerySpy.mockReset().mockResolvedValue({
      enabled: true,
      suggestions: [{ kind: { case: 'category', value: 'decision' }, source: SuggestionSource.DECIDED }]
    });
    searchMemoriesSpy.mockReset().mockReturnValue(new Promise(() => {}));
    const screen2 = await renderSearch();
    await expect.element(screen2.getByRole('button', { name: 'Suggested filter, not applied: decision' })).toBeInTheDocument();
  });

  it('a rejected UnderstandQuery renders no row and no error UI', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide';
    understandQuerySpy.mockReset().mockRejectedValue(new ConnectError('unavailable', Code.Unavailable));
    const screen = await renderSearch();
    await expect.poll(() => understandQuerySpy.mock.calls.length).toBe(1);
    await new Promise((r) => setTimeout(r, 20));
    await expect.element(screen.getByRole('toolbar', { name: 'Suggested filters' })).not.toBeInTheDocument();
  });

  it('zero suggestions renders no row', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide';
    understandQuerySpy.mockReset().mockResolvedValue({ enabled: true, suggestions: [] });
    const screen = await renderSearch();
    await expect.poll(() => understandQuerySpy.mock.calls.length).toBe(1);
    await expect.element(screen.getByRole('toolbar', { name: 'Suggested filters' })).not.toBeInTheDocument();
  });

  it('dismissing every chip removes the row entirely', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide';
    understandQuerySpy.mockReset().mockResolvedValue({
      enabled: true,
      suggestions: [{ kind: { case: 'category', value: 'decision' }, source: SuggestionSource.DECIDED }]
    });
    const screen = await renderSearch();
    const row = screen.getByRole('toolbar', { name: 'Suggested filters' });
    await expect.element(row).toBeInTheDocument();
    await screen.getByRole('button', { name: 'Dismiss suggested filter: decision' }).click();
    await expect.element(row).not.toBeInTheDocument();
  });

  it('hovering or focusing a suggested chip changes neither the URL nor the SearchMemories calls', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide';
    understandQuerySpy.mockReset().mockResolvedValue({
      enabled: true,
      suggestions: [{ kind: { case: 'category', value: 'decision' }, source: SuggestionSource.DECIDED }]
    });
    const screen = await renderSearch();
    const chip = screen.getByRole('button', { name: 'Suggested filter, not applied: decision' });
    await expect.element(chip).toBeInTheDocument();
    const hrefBefore = pageState.url.href;
    const callsBefore = searchMemoriesSpy.mock.calls.length;

    await chip.element().focus();
    await userEvent.hover(chip);

    expect(pageState.url.href).toBe(hrefBefore);
    expect(searchMemoriesSpy.mock.calls.length).toBe(callsBefore);
  });

  it('the live region announces the count once per new response and does not re-announce on a dismiss', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide';
    understandQuerySpy.mockReset().mockResolvedValue({
      enabled: true,
      suggestions: [
        { kind: { case: 'category', value: 'decision' }, source: SuggestionSource.DECIDED },
        { kind: { case: 'tag', value: 'qdrant' }, source: SuggestionSource.MATCHED }
      ]
    });
    const screen = await renderSearch();
    await expect.element(screen.getByText('2 suggested filters')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Dismiss suggested filter: #qdrant' }).click();
    await expect.element(screen.getByRole('button', { name: 'Suggested filter, not applied: #qdrant' })).not.toBeInTheDocument();
    await expect.element(screen.getByText('2 suggested filters')).toBeInTheDocument();
  });

  it('dismissing the only chip by Delete moves focus to the "Search query" input', async () => {
    pageState.url.href = 'http://localhost/search?q=what%20did%20we%20decide';
    understandQuerySpy.mockReset().mockResolvedValue({
      enabled: true,
      suggestions: [{ kind: { case: 'category', value: 'decision' }, source: SuggestionSource.DECIDED }]
    });
    const screen = await renderSearch();
    const chip = screen.getByRole('button', { name: 'Suggested filter, not applied: decision' });
    (chip.element() as HTMLElement).focus();
    await userEvent.keyboard('{Delete}');

    const input = screen.getByRole('textbox', { name: 'Search query' }).element();
    await expect.poll(() => document.activeElement).toBe(input);
  });
});
