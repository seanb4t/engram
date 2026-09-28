import { render } from 'vitest-browser-svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { ConnectError, Code } from '@connectrpc/connect';
import { MemorySchema, ArchiveOutcome, type Memory } from '$lib/gen/engram_pb';
import { curationHost } from '$lib/curation/host.svelte.ts';
import { persistResume, peekResume, type ArchiveResumeEnvelope } from '$lib/resume';
import ScheduledPage from './+page.svelte';

// `vi.hoisted` runs before this module's own imports are linked, so a
// dynamic `import()` inside the (awaited) factory is used for SvelteURL —
// mirrors search.browser.test.ts's identical ordering constraint.
const {
  gotoSpy,
  pageState,
  listScheduledSpy,
  getMemorySpy,
  archiveMemorySpy,
  consumeResumeSpy,
  redirectToLoginSpy,
  toastSpy
} = await vi.hoisted(async () => {
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
    getMemorySpy: vi.fn(),
    archiveMemorySpy: vi.fn(),
    consumeResumeSpy: vi.fn(),
    redirectToLoginSpy: vi.fn(),
    toastSpy: vi.fn()
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
    },
    engramWrite: {
      ...actual.engramWrite,
      archiveMemory: archiveMemorySpy
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
    },
    redirectToLogin: redirectToLoginSpy
  };
});

vi.mock('svelte-sonner', () => ({ toast: toastSpy }));

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

// An expired memory (notAfter in the past) — the one state /scheduled
// permits archiving.
function makeExpired(overrides: MessageInitShape<typeof MemorySchema> = {}): Memory {
  return makeMemory({ notAfter: timestampFromDate(new Date(Date.now() - 2 * 24 * 60 * 60 * 1000)), ...overrides });
}

// A scheduled memory (notBefore in the future) — never archivable.
function makeScheduled(overrides: MessageInitShape<typeof MemorySchema> = {}): Memory {
  return makeMemory({ notBefore: timestampFromDate(new Date(Date.now() + 2 * 24 * 60 * 60 * 1000)), ...overrides });
}

beforeEach(() => {
  gotoSpy.mockClear();
  listScheduledSpy.mockReset().mockResolvedValue(emptyListScheduledResult());
  getMemorySpy.mockReset();
  archiveMemorySpy.mockReset();
  consumeResumeSpy.mockReset();
  redirectToLoginSpy.mockReset();
  toastSpy.mockReset();
  sessionStorage.clear();
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

describe('scheduled route — archive for expired rows only (D-13)', () => {
  it('the all tab shows an Archive toolbar button on an expired row and none on a scheduled row', async () => {
    pageState.url.href = 'http://localhost/scheduled?state=all';
    const expired = makeExpired({ id: 'm-exp', summary: 'expired row' });
    const scheduled = makeScheduled({ id: 'm-sch', summary: 'scheduled row' });
    listScheduledSpy.mockResolvedValue({
      memories: [expired, scheduled],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await renderScheduled();
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('expired row')).toBeInTheDocument();

    const expiredRow = screen.container.querySelector('#opt-m-exp') as HTMLElement;
    await page.elementLocator(expiredRow).hover();
    await expect.element(screen.getByRole('button', { name: `Archive ${expired.shortId}` })).toBeInTheDocument();

    const scheduledRow = screen.container.querySelector('#opt-m-sch') as HTMLElement;
    await page.elementLocator(scheduledRow).hover();
    expect(screen.container.querySelector(`[aria-label="Archive ${scheduled.shortId}"]`)).toBeNull();
  });

  it('the pane shows Archive for an expired record and none for a scheduled one; no other curation action ever renders', async () => {
    const expired = makeExpired({ id: 'm-exp', summary: 'expired row' });
    listScheduledSpy.mockResolvedValue({
      memories: [expired],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    const screen = await renderScheduled();
    screen.container.style.width = '1200px';
    await expect.element(screen.getByText('expired row')).toBeInTheDocument();
    (screen.container.querySelector('[role="option"]') as HTMLElement).click();
    await expect.poll(() => screen.container.querySelector('[aria-label="Memory detail"]') !== null).toBe(true);

    await expect.element(screen.getByRole('button', { name: 'Archive' })).toBeInTheDocument();
    for (const label of ['Edit', 'Share', 'Make private', 'Supersede…', 'Restore', 'Delete']) {
      expect(screen.container.querySelector(`.d-actions button[aria-label="${label}"]`)).toBeNull();
    }
  });

  it('the pane Archive opens the confirm for an expired ?sel= record whose row is not loaded', async () => {
    pageState.url.href = 'http://localhost/scheduled?state=expired&sel=m-offpage';
    const offPage = makeExpired({ id: 'm-offpage', summary: 'off-page expired' });
    listScheduledSpy.mockResolvedValue({
      memories: [makeExpired({ id: 'm-loaded', summary: 'loaded row' })],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    getMemorySpy.mockResolvedValue({ memory: offPage });

    const screen = await renderScheduled();
    screen.container.style.width = '1200px';
    await expect.poll(() => screen.container.querySelector('[aria-label="Memory detail"]') !== null).toBe(true);
    await screen.getByRole('button', { name: 'Archive' }).click();

    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByText('Archive 1 records?')).toBeInTheDocument();
    expect(toastSpy).not.toHaveBeenCalledWith('Archive applies to expired rows only');
  });

  it('pressing "a" on an active expired row opens the archive confirm for it', async () => {
    const expired = makeExpired({ id: 'm-exp', summary: 'expired row' });
    listScheduledSpy.mockResolvedValue({
      memories: [expired],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    // CurationSurfaces' resolveRecords only scans the searchMemories/
    // listMemories caches; a /scheduled row is resolved through a live
    // GetMemory fallback instead (never cached under listScheduled).
    getMemorySpy.mockResolvedValue({ memory: expired });

    const screen = await renderScheduled();
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('expired row')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    const listbox = screen.getByRole('listbox', { name: 'Scheduled memories' });
    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m-exp');
    await userEvent.keyboard('a');

    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByText('Archive 1 records?')).toBeInTheDocument();
    expect(archiveMemorySpy).not.toHaveBeenCalled();
  });

  it('a selection of one expired and one scheduled row opens the confirm with only the expired row', async () => {
    pageState.url.href = 'http://localhost/scheduled?state=all';
    const expired = makeExpired({ id: 'm-exp', summary: 'expired row' });
    const scheduled = makeScheduled({ id: 'm-sch', summary: 'scheduled row' });
    listScheduledSpy.mockResolvedValue({
      memories: [expired, scheduled],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    getMemorySpy.mockImplementation(async ({ id }: { id: string }) => ({
      memory: [expired, scheduled].find((m) => m.id === id)
    }));

    const screen = await renderScheduled();
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('expired row')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    const listbox = screen.getByRole('listbox', { name: 'Scheduled memories' });
    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m-exp');
    await userEvent.keyboard('x');
    await userEvent.keyboard('j');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m-sch');
    await userEvent.keyboard('x');
    await userEvent.keyboard('a');

    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByText('Archive 1 records?')).toBeInTheDocument();
    await expect.element(dialog.getByText('expired row')).toBeInTheDocument();
    await expect.element(dialog.getByText('scheduled row')).not.toBeInTheDocument();
  });

  it('a selection with only scheduled rows opens nothing and toasts "Archive applies to expired rows only"', async () => {
    const scheduled = makeScheduled({ id: 'm-sch', summary: 'scheduled row' });
    listScheduledSpy.mockResolvedValue({
      memories: [scheduled],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    const screen = await renderScheduled();
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('scheduled row')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    const listbox = screen.getByRole('listbox', { name: 'Scheduled memories' });
    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m-sch');
    await userEvent.keyboard('x');
    await userEvent.keyboard('a');

    expect(screen.container.querySelector('[role="dialog"]')).toBeNull();
    expect(toastSpy).toHaveBeenCalledWith('Archive applies to expired rows only');
  });

  it('after confirming, the archived row stays in place dimmed with "archived" until the tab is re-queried', async () => {
    const expired = makeExpired({ id: 'm-exp', summary: 'expired row' });
    listScheduledSpy.mockResolvedValue({
      memories: [expired],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    archiveMemorySpy.mockResolvedValue({
      results: [{ requested: 'm-exp', id: 'm-exp', outcome: ArchiveOutcome.ARCHIVED }]
    });
    getMemorySpy.mockResolvedValue({ memory: expired });
    const screen = await renderScheduled();
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('expired row')).toBeInTheDocument();

    const row = screen.container.querySelector('#opt-m-exp') as HTMLElement;
    await page.elementLocator(row).hover();
    await screen.getByRole('button', { name: `Archive ${expired.shortId}` }).click();

    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByRole('button', { name: 'Archive' })).toBeEnabled();
    await dialog.getByRole('button', { name: 'Archive' }).click();
    await expect.poll(() => archiveMemorySpy.mock.calls.length).toBe(1);
    await dialog.getByRole('button', { name: 'Done' }).click();

    const listbox = screen.getByRole('listbox', { name: 'Scheduled memories' });
    await expect.element(listbox.getByText('expired row')).toBeInTheDocument();
    // "archived"/"superseded" chips render as a neutral (unclassed) `.st`
    // span -- only expired/scheduled get their own CSS class
    // (engram-console-conventions) -- so the state word is asserted by its
    // TEXT, not a `.archived` class selector.
    await expect
      .poll(() => Array.from(screen.container.querySelectorAll('.st')).some((el) => el.textContent === 'archived'))
      .toBe(true);
  });

  it('a PermissionDenied archive persists a v2 archive envelope with returnPath "/scheduled?state=expired" on Re-authenticate', async () => {
    pageState.url.href = 'http://localhost/scheduled?state=expired';
    const expired = makeExpired({ id: 'm-exp', summary: 'expired row' });
    listScheduledSpy.mockResolvedValue({
      memories: [expired],
      nextPageToken: '',
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    archiveMemorySpy.mockRejectedValue(new ConnectError('forbidden', Code.PermissionDenied));
    getMemorySpy.mockResolvedValue({ memory: expired });

    const screen = await renderScheduled();
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('expired row')).toBeInTheDocument();

    const row = screen.container.querySelector('#opt-m-exp') as HTMLElement;
    await page.elementLocator(row).hover();
    await screen.getByRole('button', { name: `Archive ${expired.shortId}` }).click();

    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByRole('button', { name: 'Archive' })).toBeEnabled();
    await dialog.getByRole('button', { name: 'Archive' }).click();
    await expect
      .element(dialog.getByText('Session expired. Nothing was written; your draft is kept.'))
      .toBeInTheDocument();

    await dialog.getByRole('button', { name: 'Re-authenticate' }).click();
    expect(redirectToLoginSpy).toHaveBeenCalledTimes(1);

    const persisted = peekResume() as ArchiveResumeEnvelope;
    expect(persisted?.kind).toBe('archive');
    expect(persisted.mode).toBe('archive');
    expect(persisted.ids).toEqual(['m-exp']);
    expect(persisted.returnPath).toBe('/scheduled?state=expired');
  });

  it('a seeded archive envelope reopens the confirm with those ids and the notice; archiveMemory is not called; consumeResume runs once', async () => {
    persistResume({ returnPath: '/scheduled?state=expired', kind: 'archive', mode: 'archive', ids: ['m-exp'] });
    const expired = makeExpired({ id: 'm-exp', summary: 'expired row' });
    getMemorySpy.mockResolvedValue({ memory: expired });

    const screen = await renderScheduled();
    const dialog = screen.getByRole('dialog');
    await expect.element(dialog.getByText('Signed in again — review and resend')).toBeInTheDocument();
    await expect.poll(() => consumeResumeSpy.mock.calls.length).toBe(1);
    expect(archiveMemorySpy).not.toHaveBeenCalled();
  });

  it('the registered curation host reports actionsFor as ["archive"] only for expired records', async () => {
    listScheduledSpy.mockResolvedValue(emptyListScheduledResult());
    await renderScheduled();
    await expect.poll(() => curationHost.current !== null).toBe(true);

    const expired = makeExpired({ id: 'm-exp' });
    const scheduled = makeScheduled({ id: 'm-sch' });
    expect(curationHost.current?.actionsFor(expired)).toEqual(['archive']);
    expect(curationHost.current?.actionsFor(scheduled)).toEqual([]);
  });
});
