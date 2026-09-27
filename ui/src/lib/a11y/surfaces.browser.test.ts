// app.css is not pulled in by an isolated route/component mount (only
// +layout.svelte imports it) -- without it every design token this audit
// relies on (category colours, --muted-foreground, --warning, ...) is
// undefined in this test's cascade, so a color-contrast check here would be
// auditing the browser's UA-stylesheet defaults, not the shipped console
// (same rationale as ResultRow.browser.test.ts / search-layout.browser.test.ts).
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import { auditAA, formatViolations } from './axe';
import SearchPage from '../../routes/search/+page.svelte';

// `vi.hoisted` runs before this module's own imports are linked, so a
// dynamic `import()` inside the (awaited) factory is used for SvelteURL --
// identical ordering constraint to search.browser.test.ts, whose harness
// this describe block reuses verbatim.
const {
  gotoSpy,
  pageState,
  searchMemoriesSpy,
  getMemorySpy,
  listScopesSpy,
  listMemoriesSpy,
  consumeResumeSpy,
  archiveMemorySpy,
  restoreMemorySpy,
  supersedeMemorySpy,
  relatedMemoriesSpy
} = await vi.hoisted(async () => {
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
    consumeResumeSpy: vi.fn(),
    archiveMemorySpy: vi.fn(),
    restoreMemorySpy: vi.fn(),
    supersedeMemorySpy: vi.fn(),
    relatedMemoriesSpy: vi.fn()
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
      relatedMemories: relatedMemoriesSpy
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
    tags: ['alpha', 'beta'],
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
  archiveMemorySpy.mockReset();
  restoreMemorySpy.mockReset();
  supersedeMemorySpy.mockReset();
  relatedMemoriesSpy.mockReset();
  sessionStorage.clear();
  pageState.url.href = 'http://localhost/search';
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

/** Adds/removes .dark on <html> per mode-watcher's own convention (DSYS-03). */
function setTheme(dark: boolean) {
  document.documentElement.classList.toggle('dark', dark);
}

afterEach(() => {
  document.documentElement.classList.remove('dark');
});

/** Runs auditAA in both themes and asserts zero violations in each, with a screenshot per theme. */
async function auditBothThemes(root: Element, label: string) {
  for (const dark of [false, true]) {
    setTheme(dark);
    await page.screenshot({ element: root });
    const violations = await auditAA(root);
    expect(violations, `${label} (${dark ? 'dark' : 'light'}): ${formatViolations(violations)}`).toHaveLength(0);
  }
}

describe('/search curation state (DSYS-03 tracer)', () => {
  it('a selected row, an archived (dimmed) row and a superseded row pass an automated WCAG 2.2 AA audit in both themes', async () => {
    pageState.url.href = 'http://localhost/search?q=github';

    const plain = makeMemory({ id: 'm1', summary: 'plain hit', shortId: 's0000000001' });
    const archived = makeMemory({
      id: 'm2',
      summary: 'archived hit',
      shortId: 's0000000002',
      archivedAt: timestampFromDate(new Date())
    });
    const superseded = makeMemory({
      id: 'm3',
      summary: 'superseded hit',
      shortId: 's0000000003',
      supersededBy: 'succ1'
    });

    searchMemoriesSpy.mockResolvedValue({
      memories: [plain, archived, superseded],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await renderSearch();
    // Wide layout + settle, same convention as search.browser.test.ts's own
    // hover/selection tracers -- a real pointer hover needs a settled,
    // non-zero-height row to land on, and j/x need SvelteVirtualList's
    // scroll() to resolve against a real box.
    screen.container.style.width = '1200px';
    screen.container.style.height = '600px';
    await expect.element(screen.getByText('plain hit')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('.rs-group') !== null).toBe(true);

    // Select the plain row (x).
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m1');
    await userEvent.keyboard('x');

    // Hover the archived row so its row toolbar renders (Restore, not
    // Archive -- D-05).
    const archivedRow = screen.container.querySelector('#opt-m2') as HTMLElement;
    await page.elementLocator(archivedRow).hover();
    await expect.element(screen.getByRole('button', { name: `Restore ${archived.shortId}` })).toBeInTheDocument();

    // Sanity: the archived and superseded rows are actually dimmed and their
    // state words render (would otherwise silently audit undimmed rows).
    const supersededRow = screen.container.querySelector('#opt-m3') as HTMLElement;
    expect(archivedRow.querySelector('.cat.dim, .sum.dim, .tags.dim, .scope.dim')).not.toBeNull();
    expect(supersededRow.querySelector('.cat.dim, .sum.dim, .tags.dim, .scope.dim')).not.toBeNull();
    expect(archivedRow.querySelector('.st')?.textContent).toBe('archived');
    expect(supersededRow.querySelector('.st')?.textContent).toBe('superseded');

    await auditBothThemes(document.body, '/search curation state');
  });
});
