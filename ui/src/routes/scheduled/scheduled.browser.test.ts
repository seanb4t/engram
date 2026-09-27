import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { ConnectError, Code } from '@connectrpc/connect';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import ScheduledPage from './+page.svelte';

// `vi.hoisted` runs before this module's own imports are linked, so a
// dynamic `import()` inside the (awaited) factory is used for SvelteURL —
// mirrors search.browser.test.ts's identical ordering constraint.
const { gotoSpy, pageState, listScheduledSpy, getMemorySpy } = await vi.hoisted(async () => {
  const { SvelteURL } = await import('svelte/reactivity');
  const url = new SvelteURL('http://localhost/scheduled');
  const pageState = { url };
  const gotoSpy = vi.fn((href: string) => {
    const next = new URL(href, 'http://localhost');
    pageState.url.href = next.href;
  });
  return {
    gotoSpy,
    pageState,
    listScheduledSpy: vi.fn(),
    getMemorySpy: vi.fn()
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
      listScheduled: listScheduledSpy,
      getMemory: getMemorySpy
    }
  };
});

let qc: QueryClient;
function renderScheduled() {
  return render(ScheduledPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
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
    shortId: 's0000000001',
    ...overrides
  });
}

function emptyListScheduledResult() {
  return { memories: [], nextPageToken: '', searchedScopes: [], scopesTruncated: false, scopesUnknown: false };
}

beforeEach(() => {
  gotoSpy.mockClear();
  listScheduledSpy.mockReset().mockResolvedValue(emptyListScheduledResult());
  getMemorySpy.mockReset();
  pageState.url.href = 'http://localhost/scheduled';
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('scheduled route — lists windowed records across scopes (CUR-04 tracer)', () => {
  it('calls ListScheduled cross-spine, state "scheduled", empty pageToken, and shows the window + reveal phrase', async () => {
    // A five-minute buffer over the exact 3-day mark keeps `windowPhrase`'s
    // real-`Date.now()` computation stable against render latency (the route
    // calls `windowPhrase(m)` with no injected `now` — that's time.test.ts's
    // job) — without it, a few seconds of latency can floor the day bucket
    // down to 2d and flake this assertion.
    const notBefore = new Date(Date.now() + 3 * 24 * 60 * 60 * 1000 + 5 * 60 * 1000);
    const memory = makeMemory({
      id: 'm1',
      summary: 'a future memory',
      notBefore: timestampFromDate(notBefore)
    });
    listScheduledSpy.mockResolvedValue({
      memories: [memory],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await renderScheduled();
    await expect.element(screen.getByText('a future memory')).toBeInTheDocument();

    expect(listScheduledSpy).toHaveBeenCalledWith(
      expect.objectContaining({ scope: '', state: 'scheduled', crossSpine: true, pageToken: '' }),
      expect.objectContaining({ signal: expect.anything() })
    );

    await expect.element(screen.getByText(/reveals in 3d/)).toBeInTheDocument();
    // A plain getByText('scheduled') is ambiguous here — the single-child
    // `.states` wrapper span and its `.st.scheduled` child both normalize to
    // the identical text "scheduled", so a CSS query is used instead of a
    // text locator to avoid a Playwright strict-mode ambiguity between them.
    await expect.poll(() => screen.container.querySelector('.st.scheduled')?.textContent).toBe('scheduled');
  });
});

describe('scheduled route — tabs (D-13)', () => {
  it('clicking the expired tab navigates with state=expired and calls listScheduled with state "expired"', async () => {
    const screen = await renderScheduled();
    await expect.poll(() => listScheduledSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    listScheduledSpy.mockClear();

    await screen.getByRole('tab', { name: 'expired' }).click();

    expect(pageState.url.searchParams.get('state')).toBe('expired');
    await expect
      .poll(() => listScheduledSpy.mock.calls.some((c) => c[0].state === 'expired'))
      .toBe(true);
  });

  it('clicking the all tab navigates with state=all and calls listScheduled with state "all"', async () => {
    const screen = await renderScheduled();
    await expect.poll(() => listScheduledSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    listScheduledSpy.mockClear();

    await screen.getByRole('tab', { name: 'all' }).click();

    expect(pageState.url.searchParams.get('state')).toBe('all');
    await expect.poll(() => listScheduledSpy.mock.calls.some((c) => c[0].state === 'all')).toBe(true);
  });
});

describe('scheduled route — per-tab honest empty state (E6, D-13)', () => {
  it.each([
    { state: 'scheduled', heading: 'No scheduled memories in any scope you can read' },
    { state: 'expired', heading: 'No expired memories in any scope you can read' },
    { state: 'all', heading: 'No windowed memories in any scope you can read' }
  ])('the $state tab shows its own empty copy, never a shared generic one', async ({ state, heading }) => {
    pageState.url.href = `http://localhost/scheduled?state=${state}`;
    const screen = await renderScheduled();
    await expect.element(screen.getByText(heading)).toBeInTheDocument();
    await page.screenshot();
  });
});

describe('scheduled route — a single record renders one row (E6 zero-one-many)', () => {
  it('one windowed record renders as a single row with no summary line', async () => {
    listScheduledSpy.mockResolvedValue({
      memories: [makeMemory({ id: 'm1', summary: 'lonely record' })],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    const screen = await renderScheduled();
    await expect.element(screen.getByText('lonely record')).toBeInTheDocument();
    expect(screen.container.querySelectorAll('[role="option"]').length).toBe(1);
    await page.screenshot();
  });
});

describe('scheduled route — cursor infinite scroll (E6 loading)', () => {
  it('appends page 2 below page 1 without reordering, showing a Loading more row while it fetches', async () => {
    let resolvePage2!: (v: unknown) => void;
    const page2Promise = new Promise((resolve) => {
      resolvePage2 = resolve;
    });
    listScheduledSpy
      .mockResolvedValueOnce({
        memories: [makeMemory({ id: 'm1', summary: 'page one hit' })],
        nextPageToken: 't2',
        searchedScopes: ['repo:test'],
        scopesTruncated: false,
        scopesUnknown: false
      })
      .mockImplementationOnce(() => page2Promise);

    const screen = await renderScheduled();
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('page one hit')).toBeInTheDocument();

    await expect.poll(() => listScheduledSpy.mock.calls.length).toBeGreaterThanOrEqual(2);
    expect(listScheduledSpy.mock.calls[1][0].pageToken).toBe('t2');
    await expect.element(screen.getByText('Loading more…')).toBeInTheDocument();

    resolvePage2({
      memories: [makeMemory({ id: 'm2', summary: 'page two hit' })],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    await expect.element(screen.getByText('page two hit')).toBeInTheDocument();
    const rows = Array.from(screen.container.querySelectorAll('[role="option"]'));
    expect(rows.map((r) => r.id)).toEqual(['opt-m1', 'opt-m2']);
    await expect.element(screen.getByText('Loading more…')).not.toBeInTheDocument();
  });

  it('a failed page-2 fetch keeps page 1 visible and shows the envelope with a Retry in place of the loading row', async () => {
    listScheduledSpy
      .mockResolvedValueOnce({
        memories: [makeMemory({ id: 'm1', summary: 'page one hit' })],
        nextPageToken: 't2',
        searchedScopes: ['repo:test'],
        scopesTruncated: false,
        scopesUnknown: false
      })
      .mockRejectedValueOnce(new ConnectError('field=page hint=out_of_range: boom', Code.InvalidArgument))
      .mockResolvedValueOnce({
        memories: [makeMemory({ id: 'm2', summary: 'page two hit' })],
        nextPageToken: '',
        searchedScopes: ['repo:test'],
        scopesTruncated: false,
        scopesUnknown: false
      });

    const screen = await renderScheduled();
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('page one hit')).toBeInTheDocument();
    await expect.poll(() => listScheduledSpy.mock.calls.length).toBeGreaterThanOrEqual(2);

    const retryBtn = screen.getByRole('button', { name: 'Retry' });
    await expect.element(retryBtn).toBeInTheDocument();
    await expect.element(screen.getByText('page one hit')).toBeInTheDocument();

    await retryBtn.click();
    await expect.element(screen.getByText('page two hit')).toBeInTheDocument();
  });

  it('a rejected first load renders the envelope, never a bare empty list', async () => {
    listScheduledSpy.mockRejectedValue(
      new ConnectError('field=state hint=out_of_range: bad state', Code.InvalidArgument)
    );
    const screen = await renderScheduled();
    await expect.element(screen.getByText('Server rejected the request')).toBeInTheDocument();
  });
});

describe('scheduled route — honest header (D-13)', () => {
  it('reads "{N} expired memories across {M} scopes" with "· scroll for more" while another page exists', async () => {
    pageState.url.href = 'http://localhost/scheduled?state=expired';
    // The virtual list eagerly calls onloadmore as soon as hasMore is true,
    // regardless of container size -- resolving every call would loop the
    // SAME mocked page forever (SvelteVirtualList's own duplicate-key
    // guard). The point of this test is that hasNextPage stays true and is
    // REPORTED (the header's "scroll for more" clause), not that a second
    // page actually lands, so every call after page 1 hangs forever.
    listScheduledSpy
      .mockResolvedValueOnce({
        memories: [makeMemory({ id: 'm1', summary: 'expired one' })],
        nextPageToken: 't2',
        searchedScopes: ['repo:test'],
        scopesTruncated: false,
        scopesUnknown: false
      })
      .mockImplementation(() => new Promise(() => {}));
    const screen = await renderScheduled();
    await expect.element(screen.getByText('expired one')).toBeInTheDocument();
    await expect.element(screen.getByText('· scroll for more')).toBeInTheDocument();
  });
});

describe('scheduled route — overflow (E6, DSYS-04)', () => {
  it('a max-length summary on a 360px-wide row causes no horizontal scroll, and the row tooltip carries both raw timestamps', async () => {
    const longSummary = 'x'.repeat(400);
    const notAfter = new Date(Date.now() + 5 * 24 * 60 * 60 * 1000);
    listScheduledSpy.mockResolvedValue({
      memories: [makeMemory({ id: 'm1', summary: longSummary, notAfter: timestampFromDate(notAfter) })],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    const screen = await renderScheduled();
    screen.container.style.width = '360px';
    screen.container.style.height = '600px';
    await expect.poll(() => screen.container.querySelector('[role="option"]') !== null).toBe(true);

    const row = screen.container.querySelector('.result-row-line') as HTMLElement;
    expect(row.scrollWidth).toBeLessThanOrEqual(row.clientWidth + 1);

    const win = screen.container.querySelector('.win') as HTMLElement;
    expect(win.getAttribute('title')).toContain('→');
    await page.screenshot();
  });
});
