import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import { persistResume } from '$lib/resume';
import SearchPage from './+page.svelte';

// `vi.hoisted` runs before the module's own imports are linked (its factory
// is spliced in ahead of every `import`), so `SvelteURL` — imported normally
// above — is not yet bound when a synchronous factory would run. A dynamic
// `import()` inside the (awaited) factory sidesteps that ordering: it is a
// genuine async operation, resolved after linking, not a static binding.
const { gotoSpy, pageState, searchMemoriesSpy, getMemorySpy, listScopesSpy, listMemoriesSpy, consumeResumeSpy } = await vi.hoisted(async () => {
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
    listMemoriesSpy: vi.fn(),
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
    engram: {
      ...actual.engram,
      searchMemories: searchMemoriesSpy,
      getMemory: getMemorySpy,
      listScopes: listScopesSpy,
      listMemories: listMemoriesSpy
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
    const [href, opts] = gotoSpy.mock.calls[0] as [string, { replaceState?: boolean }];
    expect(href).toContain('q=abc');
    expect(opts).toMatchObject({ replaceState: true });

    await new Promise((r) => setTimeout(r, 250));
    expect(gotoSpy).toHaveBeenCalledTimes(1);
  });
});
