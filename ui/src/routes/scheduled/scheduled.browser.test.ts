import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
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
