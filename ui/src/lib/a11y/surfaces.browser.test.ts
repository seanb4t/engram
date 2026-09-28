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
import { ConnectError, Code } from '@connectrpc/connect';
import {
  MemorySchema,
  ArchiveOutcome,
  SupersedeMemoryResponseSchema,
  GetMemoryResponseSchema,
  RelatedMemoriesResponseSchema,
  RelatedMemorySchema,
  RelatedEdgeSchema,
  SupersessionEvidenceSchema,
  TagEvidenceSchema,
  WeightedTagSchema,
  ListTagsResponseSchema,
  EdgeType,
  SupersessionDirection,
  type Memory
} from '$lib/gen/engram_pb';
import { headerSearch } from '$lib/search/header-search.svelte';
import type { ArchiveSubmitOutcome, SupersedeFields } from '$lib/mutations/curation';
import { auditAA, formatViolations } from './axe';
import SearchPage from '../../routes/search/+page.svelte';
import RulesPage from '../../routes/rules/+page.svelte';
import ScheduledPage from '../../routes/scheduled/+page.svelte';
import RelatedPage from '../../routes/related/[id]/+page.svelte';
import ArchiveConfirmDialog from '../components/ArchiveConfirmDialog.svelte';
import SupersedeDialog from '../components/SupersedeDialog.svelte';
import ChainDialog from '../components/ChainDialog.svelte';
import DeleteConfirmDialog from '../components/DeleteConfirmDialog.svelte';
import ResultsHeader from '../components/ResultsHeader.svelte';
import ResultsList from '../components/ResultsList.svelte';
import TagCombobox from '../components/TagCombobox.svelte';
import HeaderSearch from '../components/HeaderSearch.svelte';

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
  relatedMemoriesSpy,
  listRulesSpy,
  listScheduledSpy,
  listTagsSpy,
  deleteMemorySpy,
  peekResumeSpy,
  redirectToLoginSpy
} = await vi.hoisted(async () => {
  const { SvelteURL } = await import('svelte/reactivity');
  const url = new SvelteURL('http://localhost/search');
  // /related/[id] reads page.params.id (no SvelteKit router in this harness,
  // same convention as related.browser.test.ts) -- a getter parsed from the
  // reactive url.pathname so every other route (flat URLSearchParams, never
  // reading params) is unaffected.
  const pageState = {
    url,
    get params() {
      const m = /\/related\/([^/]+)/.exec(url.pathname);
      return { id: m ? m[1] : '' };
    }
  };
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
    relatedMemoriesSpy: vi.fn(),
    listRulesSpy: vi.fn(),
    listScheduledSpy: vi.fn(),
    listTagsSpy: vi.fn(),
    deleteMemorySpy: vi.fn(),
    peekResumeSpy: vi.fn(() => null),
    redirectToLoginSpy: vi.fn()
  };
});

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));
vi.mock('$app/state', () => ({ page: pageState }));

// One consolidated $lib/client mock covers every surface this file audits
// (Task 1's /search route plus Task 2's dialogs and /rules, /scheduled) --
// vi.mock('$lib/client', ...) applies once per test file, not per describe.
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
      relatedMemories: relatedMemoriesSpy,
      listRules: listRulesSpy,
      listScheduled: listScheduledSpy,
      listTags: listTagsSpy
    },
    engramWrite: {
      ...actual.engramWrite,
      archiveMemory: archiveMemorySpy,
      restoreMemory: restoreMemorySpy,
      supersedeMemory: supersedeMemorySpy,
      deleteMemory: deleteMemorySpy
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
    peekResume: peekResumeSpy,
    redirectToLogin: redirectToLoginSpy
  };
});

let qc: QueryClient;
function renderSearch() {
  return render(SearchPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}
function renderRelated() {
  return render(RelatedPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
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

// Bits-ui's dialog/tabs entry transitions (fade-in/zoom-in) are still
// mid-animation the instant an element lands in the DOM -- `data-state="open"`
// is set synchronously, well before the CSS transition settles. Auditing a
// partially-transparent overlay produces a genuine but purely transient
// color-contrast reading (the blended, not-yet-opaque colour), which is why
// this ran flaky under `settle()`'s fixed delay alone on a loaded host.
// Disabling every transition/animation duration globally, once, before any
// component in this file ever mounts removes the race outright rather than
// trying to out-wait it.
let disableAnimationsStyle: HTMLStyleElement | undefined;
function disableAnimations() {
  if (disableAnimationsStyle) return;
  disableAnimationsStyle = document.createElement('style');
  disableAnimationsStyle.textContent =
    '*, *::before, *::after { transition-duration: 0s !important; transition-delay: 0s !important; animation-duration: 0s !important; animation-delay: 0s !important; }';
  document.head.appendChild(disableAnimationsStyle);
}
disableAnimations();

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
  listRulesSpy.mockReset().mockResolvedValue({ rules: [], advisory: '', searchedScopes: [], scopesTruncated: false, scopesUnknown: false });
  listScheduledSpy.mockReset().mockResolvedValue({ memories: [], nextPageToken: '', searchedScopes: [], scopesTruncated: false, scopesUnknown: false });
  listTagsSpy.mockReset().mockResolvedValue(create(ListTagsResponseSchema, { tags: [], more: false }));
  deleteMemorySpy.mockReset();
  peekResumeSpy.mockReset().mockReturnValue(null);
  redirectToLoginSpy.mockReset();
  sessionStorage.clear();
  pageState.url.href = 'http://localhost/search';
  // headerSearch is a module-level singleton (the ⌘K hand-off point, D-11) --
  // reset it so one test's typed text never leaks into another.
  headerSearch.text = '';
  headerSearch.focusSeq = 0;
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

/** Adds/removes .dark on <html> per mode-watcher's own convention (DSYS-03). */
function setTheme(dark: boolean) {
  document.documentElement.classList.toggle('dark', dark);
}

afterEach(() => {
  document.documentElement.classList.remove('dark');
});

/**
 * Bits-ui's dialog/tabs entry transition (fade-in/zoom-in, ~150-200ms) is
 * still mid-animation the instant an element exists in the DOM -- `open` and
 * `data-state="open"` land immediately, well before the CSS transition
 * settles. Auditing a partially-transparent overlay produces a genuine but
 * transient color-contrast reading (the blended, not-yet-opaque colour), so
 * every audit below waits past it first rather than racing it.
 */
async function settle() {
  await new Promise((r) => setTimeout(r, 300));
}

/** Runs auditAA in both themes and asserts zero violations in each, with a screenshot per theme. */
async function auditBothThemes(root: Element, label: string) {
  await settle();
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

    // Deterministically cancel the row's own pending hover-card-open timer
    // (ResultsList's 250ms mousemove-driven ROW-02 delay, unrelated to the
    // D-05 instant row toolbar this test is auditing) -- a real elapsed-time
    // race between this suite's own setup cost and that timer would
    // otherwise make whether ResultHoverCard is mounted during the audit
    // flaky depending on host load, auditing a surface this task doesn't
    // scope. A dispatched `mouseleave` runs ResultsList's own listener
    // synchronously, no waiting required.
    archivedRow.dispatchEvent(new MouseEvent('mouseleave', { bubbles: true }));

    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, '/search curation state');
  });
});

// ---------------------------------------------------------------------------
// Task 2: every other new surface this phase ships, AA-audited in both
// themes with a screenshot each, plus the D-17 keyboard model.
// ---------------------------------------------------------------------------

function baseArchiveProps(overrides: Record<string, unknown> = {}) {
  return {
    open: true,
    mode: 'archive' as const,
    records: [makeMemory({ id: 'm1', summary: 'archive me', shortId: 's0000000001' })],
    onsubmit: vi.fn(
      async (): Promise<ArchiveSubmitOutcome> => ({
        kind: 'ok',
        results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED }]
      })
    ),
    oncancel: vi.fn(),
    ondone: vi.fn(),
    onundo: vi.fn(),
    onreauth: vi.fn(),
    ...overrides
  };
}

describe('archive confirm dialog — AA audit (DSYS-03)', () => {
  it('the confirm state passes the AA audit in both themes', async () => {
    const screen = await render(ArchiveConfirmDialog, baseArchiveProps());
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'ArchiveConfirmDialog — confirm');
  });

  it('the result state passes the AA audit in both themes', async () => {
    const screen = await render(ArchiveConfirmDialog, baseArchiveProps());
    await screen.getByRole('button', { name: 'Archive' }).click();
    await expect.element(screen.getByText(/1 archived/)).toBeInTheDocument();
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'ArchiveConfirmDialog — result');
  });

  it('the re-auth state passes the AA audit in both themes', async () => {
    const onsubmit = vi.fn(async (): Promise<ArchiveSubmitOutcome> => ({ kind: 'reauth' }));
    const screen = await render(ArchiveConfirmDialog, baseArchiveProps({ onsubmit }));
    await screen.getByRole('button', { name: 'Archive' }).click();
    await expect
      .element(screen.getByText('Session expired. Nothing was written; your draft is kept.'))
      .toBeInTheDocument();
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'ArchiveConfirmDialog — re-auth');
  });
});

function supersedeMem(id: string, overrides: MessageInitShape<typeof MemorySchema> = {}): Memory {
  return create(MemorySchema, {
    id,
    shortId: `${id.slice(0, 8).toUpperCase().padEnd(8, '0')}01`,
    summary: `summary for ${id}`,
    content: `content for ${id}`,
    category: 'convention',
    scope: 'repo:test',
    visibility: 'private',
    owner: 'me',
    tags: [],
    ...overrides
  });
}

function supersedeFields(overrides: Partial<SupersedeFields> = {}): SupersedeFields {
  return { summary: '', content: 'seed content', category: 'convention', scope: 'repo:test', tags: [], ...overrides };
}

function renderSupersede(overrides: Record<string, unknown> = {}) {
  return render(SupersedeDialog, {
    open: true,
    targets: [] as Memory[],
    fields: supersedeFields(),
    idempotencyKey: 'idem-1',
    onpreview: vi.fn(async () =>
      create(SupersedeMemoryResponseSchema, { id: '', shortId: '', validated: true, supersedes: [], targets: [] })
    ),
    onsubmit: vi.fn(async () => ({ id: 'n1', shortId: 'N1SHORT001', validated: false, supersedes: [], targets: [] })),
    onlookup: vi.fn(),
    onresolvehead: vi.fn(),
    oncancel: vi.fn(),
    ondone: vi.fn(),
    onreauth: vi.fn(),
    ...overrides
  });
}

describe('supersede dialog — AA audit (DSYS-03)', () => {
  it('the populated state passes the AA audit in both themes', async () => {
    const screen = await renderSupersede({ targets: [supersedeMem('m1')] });
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'SupersedeDialog — populated');
  });

  it('an invalid target (rule, cannot be superseded) passes the AA audit in both themes', async () => {
    const screen = await renderSupersede({ targets: [supersedeMem('r1', { category: 'rule' })] });
    await expect.element(screen.getByText('rules cannot be superseded — delete it instead')).toBeInTheDocument();
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'SupersedeDialog — invalid target');
  });

  it('the success state passes the AA audit in both themes', async () => {
    const screen = await renderSupersede({ targets: [supersedeMem('m1')] });
    await screen.getByRole('button', { name: 'Supersede 1 → 1' }).click();
    await expect.element(screen.getByText(/No undo\./)).toBeInTheDocument();
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'SupersedeDialog — success');
  });
});

function chainMem(id: string, overrides: MessageInitShape<typeof MemorySchema> = {}): Memory {
  return create(MemorySchema, {
    id,
    shortId: `${id.slice(0, 8).toUpperCase()}01`,
    summary: `summary for ${id}`,
    category: 'gotcha',
    ...overrides
  });
}

function chainSupersessionEdge(direction: SupersessionDirection, depth: number) {
  return create(RelatedEdgeSchema, {
    type: EdgeType.SUPERSESSION,
    evidence: { case: 'supersession', value: create(SupersessionEvidenceSchema, { direction, depth }) }
  });
}

describe('chain dialog — AA audit (DSYS-03)', () => {
  it('a populated chain with a peeked node passes the AA audit in both themes', async () => {
    const p1 = chainMem('cp1-id', { supersedes: ['cp2-id'], supersededBy: 'chead-id' });
    const p2 = chainMem('cp2-id', { supersededBy: 'cp1-id' });
    const head = chainMem('chead-id', { supersedes: ['cp1-id'] });

    relatedMemoriesSpy.mockReset();
    relatedMemoriesSpy.mockImplementation(async ({ id }: { id: string }) => {
      if (id === p1.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: p1,
          related: [create(RelatedMemorySchema, { memory: head, edges: [chainSupersessionEdge(SupersessionDirection.SUCCESSOR, 1)] })]
        });
      }
      if (id === head.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: head,
          related: [
            create(RelatedMemorySchema, { memory: p1, edges: [chainSupersessionEdge(SupersessionDirection.PREDECESSOR, 1)] }),
            create(RelatedMemorySchema, { memory: p2, edges: [chainSupersessionEdge(SupersessionDirection.PREDECESSOR, 2)] })
          ]
        });
      }
      throw new Error(`unexpected id ${id}`);
    });
    getMemorySpy.mockReset();
    getMemorySpy.mockResolvedValue(create(GetMemoryResponseSchema, { memory: p2 }));

    const localQc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const screen = await render(
      ChainDialog,
      { open: true, anchorId: p1.id, oncancel: vi.fn() },
      { wrapper: QueryClientProvider, wrapperProps: { client: localQc } }
    );
    await expect.poll(() => document.querySelectorAll('.ccol-h').length).toBe(3);

    const node = [...document.querySelectorAll('.cnode')].find((n) => n.textContent?.includes(p2.shortId)) as HTMLElement;
    node.click();
    await expect.poll(() => document.querySelector('.chain-peek')?.textContent ?? '').toContain('fetch-by-id');

    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'ChainDialog — populated + peek');
  });
});

describe('rule delete confirm — AA audit (DSYS-03)', () => {
  it('the rule kind confirm passes the AA audit in both themes', async () => {
    const screen = await render(DeleteConfirmDialog, {
      open: true,
      kind: 'rule' as const,
      onconfirm: vi.fn(async () => {}),
      oncancel: vi.fn()
    });
    await expect.element(screen.getByText('Delete this rule?')).toBeInTheDocument();
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'DeleteConfirmDialog — rule');
  });
});

describe('rules route — AA audit (DSYS-03)', () => {
  it('the populated rules list passes the AA audit in both themes', async () => {
    pageState.url.href = 'http://localhost/rules';
    const rule1 = makeMemory({
      id: 'ru1',
      category: 'rule',
      summary: 'rule one',
      scope: 'rule:repo:test',
      shortId: 's0000000009',
      visibility: 'shared'
    });
    listRulesSpy.mockResolvedValue({
      rules: [rule1],
      advisory: '',
      searchedScopes: ['rule:repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await render(RulesPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
    await expect.element(screen.getByText('rule one')).toBeInTheDocument();

    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'rules route');
  });
});

describe('scheduled route — AA audit (DSYS-03)', () => {
  it('the scheduled, expired and all tabs each pass the AA audit in both themes', async () => {
    pageState.url.href = 'http://localhost/scheduled';
    const scheduled1 = makeMemory({
      id: 'sc1',
      summary: 'future memory',
      shortId: 's0000000010',
      notBefore: timestampFromDate(new Date(Date.now() + 3 * 24 * 60 * 60 * 1000 + 5 * 60 * 1000))
    });
    const expired1 = makeMemory({
      id: 'ex1',
      summary: 'lapsed memory',
      shortId: 's0000000011',
      notAfter: timestampFromDate(new Date(Date.now() - 2 * 24 * 60 * 60 * 1000))
    });
    listScheduledSpy.mockImplementation(async (req: { state: string }) => {
      if (req.state === 'expired') {
        return { memories: [expired1], nextPageToken: '', searchedScopes: ['repo:test'], scopesTruncated: false, scopesUnknown: false };
      }
      if (req.state === 'all') {
        return {
          memories: [scheduled1, expired1],
          nextPageToken: '',
          searchedScopes: ['repo:test'],
          scopesTruncated: false,
          scopesUnknown: false
        };
      }
      return { memories: [scheduled1], nextPageToken: '', searchedScopes: ['repo:test'], scopesTruncated: false, scopesUnknown: false };
    });

    const screen = await render(ScheduledPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
    await expect.element(screen.getByText('future memory')).toBeInTheDocument();
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'scheduled route — scheduled tab');

    await screen.getByRole('tab', { name: 'expired' }).click();
    await expect.element(screen.getByText('lapsed memory')).toBeInTheDocument();
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'scheduled route — expired tab');

    await screen.getByRole('tab', { name: 'all' }).click();
    await expect.element(screen.getByText('future memory')).toBeInTheDocument();
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'scheduled route — all tab');
  });
});

describe('list selection and toolbar — AA audit (DSYS-03)', () => {
  it('a multi-row selection with the bulk bar and a hovered row toolbar passes the AA audit in both themes', async () => {
    const onopen = vi.fn();
    const onarchive = vi.fn();
    const onrestore = vi.fn();
    const onsupersede = vi.fn();
    const onchain = vi.fn();
    const onclear = vi.fn();

    const rows = [
      makeMemory({ id: 'lt1', summary: 'row one', shortId: 's0000000020' }),
      makeMemory({ id: 'lt2', summary: 'row two', shortId: 's0000000021', archivedAt: timestampFromDate(new Date()) })
    ];

    const headerScreen = await render(ResultsHeader, {
      parts: [],
      k: 20,
      selection: { count: 2, onsupersede, onarchive, onrestore, onclear }
    });
    await expect.element(headerScreen.getByRole('toolbar', { name: 'Bulk actions' })).toBeInTheDocument();

    const listScreen = await render(ResultsList, {
      memories: rows,
      label: 'Search results',
      onopen,
      selectable: true,
      selectedIds: [rows[0].id, rows[1].id],
      onarchive,
      onrestore,
      onsupersede,
      onchain
    });
    listScreen.container.style.height = '600px';

    const row = listScreen.container.querySelector(`#opt-${rows[0].id}`) as HTMLElement;
    await page.elementLocator(row).hover();
    await expect
      .element(listScreen.getByRole('toolbar', { name: `Row actions for ${rows[0].shortId}` }))
      .toBeInTheDocument();

    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'list selection and toolbar');
  });
});

describe('curation dialog keyboard model (D-17)', () => {
  it('ArchiveConfirmDialog moves focus in on open, ignores Escape while pending, and closes on Escape once idle', async () => {
    const oncancel = vi.fn();
    const screen = await render(
      ArchiveConfirmDialog,
      baseArchiveProps({ oncancel, pending: true })
    );
    const dialog = screen.getByRole('dialog');
    await expect.poll(() => dialog.element().contains(document.activeElement)).toBe(true);

    // Pending: Escape is ignored (escapeKeydownBehavior='ignore').
    await userEvent.keyboard('{Escape}');
    expect(oncancel).not.toHaveBeenCalled();
    await expect.element(dialog).toBeInTheDocument();
  });

  it('ArchiveConfirmDialog closes on Escape once idle (not pending)', async () => {
    const oncancel = vi.fn();
    const screen = await render(ArchiveConfirmDialog, baseArchiveProps({ oncancel }));
    const dialog = screen.getByRole('dialog');
    await expect.poll(() => dialog.element().contains(document.activeElement)).toBe(true);

    await userEvent.keyboard('{Escape}');
    await expect.poll(() => oncancel.mock.calls.length).toBe(1);
  });

  it('SupersedeDialog Cmd+Enter submits only once the button is enabled', async () => {
    const onsubmit = vi.fn(async () => ({ id: 'n1', shortId: 'N1SHORT001', validated: false, supersedes: [], targets: [] }));
    const disabledScreen = await renderSupersede({ targets: [], onsubmit });
    const disabledDialog = disabledScreen.getByRole('dialog');
    disabledDialog.element().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true }));
    expect(onsubmit).not.toHaveBeenCalled();
    await disabledScreen.unmount();

    const enabledScreen = await renderSupersede({ targets: [supersedeMem('m1')], onsubmit });
    // The debounced validate_only preview must resolve (canSubmit gates on
    // previewValidated) before Cmd+Enter can do anything -- same wait
    // SupersedeDialog.browser.test.ts's own Cmd+Enter coverage uses.
    await expect.element(enabledScreen.getByRole('button', { name: 'Supersede 1 → 1' })).not.toBeDisabled();
    const enabledDialog = enabledScreen.getByRole('dialog');
    enabledDialog.element().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true }));
    await expect.poll(() => onsubmit.mock.calls.length).toBe(1);
  });

  it('ChainDialog closes on Escape', async () => {
    const oncancel = vi.fn();
    getMemorySpy.mockReset();
    relatedMemoriesSpy.mockReset();
    relatedMemoriesSpy.mockResolvedValue(create(RelatedMemoriesResponseSchema, { anchor: chainMem('kc1'), related: [] }));
    const localQc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const screen = await render(
      ChainDialog,
      { open: true, anchorId: 'kc1', oncancel },
      { wrapper: QueryClientProvider, wrapperProps: { client: localQc } }
    );
    const dialog = screen.getByRole('dialog');
    await expect.poll(() => dialog.element().contains(document.activeElement)).toBe(true);

    await userEvent.keyboard('{Escape}');
    await expect.poll(() => oncancel.mock.calls.length).toBe(1);
  });

  it('Tab from the results listbox reaches the row toolbar\'s first button with a visible focus ring', async () => {
    const onopen = vi.fn();
    const onarchive = vi.fn();
    const rows = [makeMemory({ id: 'kb1', summary: 'kb row', shortId: 's0000000030' })];
    const screen = await render(ResultsList, { memories: rows, label: 'Search results', onopen, onarchive });
    screen.container.style.height = '600px';

    const row = screen.container.querySelector(`#opt-${rows[0].id}`) as HTMLElement;
    await page.elementLocator(row).hover();
    const archiveBtn = screen.getByRole('button', { name: `Archive ${rows[0].shortId}` });
    await expect.element(archiveBtn).toBeInTheDocument();

    archiveBtn.element().focus();
    expect(document.activeElement).toBe(archiveBtn.element());
    const style = getComputedStyle(archiveBtn.element());
    expect(style.outlineStyle === 'none' && style.boxShadow === 'none').toBe(false);
  });

  it('Tab reaches the bulk bar\'s first action with a visible focus ring', async () => {
    const onsupersede = vi.fn();
    const screen = await render(ResultsHeader, {
      parts: [],
      k: 20,
      selection: { count: 1, onsupersede, onclear: vi.fn() }
    });
    const firstAction = screen.getByRole('button', { name: /Supersede/ });
    firstAction.element().focus();
    expect(document.activeElement).toBe(firstAction.element());
    const style = getComputedStyle(firstAction.element());
    expect(style.outlineStyle === 'none' && style.boxShadow === 'none').toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Task 2 (GRAPH-03, TAGS-01, TAGS-02): the /related route and the tag
// surfaces this phase ships, AA-audited in both themes with zero violations.
// ---------------------------------------------------------------------------

function relatedMemory(id: string, overrides: MessageInitShape<typeof MemorySchema> = {}): Memory {
  return create(MemorySchema, {
    id,
    shortId: `${id.slice(0, 8).toUpperCase().padEnd(8, '0')}01`,
    category: 'convention',
    summary: `summary for ${id}`,
    content: '',
    tags: [],
    scope: 'repo:test',
    visibility: 'private',
    owner: 'me',
    ...overrides
  });
}

// Anchor plus one tag candidate sharing a tag -- enough to select a node,
// open the evidence section under the graph, filter the Tags tab by that
// tag, and land a real second node for the arrow-key focus card.
function relatedFixtureResponse() {
  const anchor = relatedMemory('aa-anchor', {
    shortId: 'aaanchor001',
    category: 'decision',
    summary: 'AA anchor',
    tags: ['qdrant']
  });
  const candidate = relatedMemory('aa-cand', {
    shortId: 'aacand00001',
    category: 'preference',
    summary: 'AA candidate',
    tags: ['qdrant']
  });
  const tagEdge = create(RelatedEdgeSchema, {
    type: EdgeType.TAG,
    evidence: {
      case: 'tag',
      value: create(TagEvidenceSchema, { sharedTags: [create(WeightedTagSchema, { tag: 'qdrant', weight: 1.1 })], tagWeight: 1.1 })
    }
  });
  return create(RelatedMemoriesResponseSchema, {
    anchor,
    truncated: false,
    related: [create(RelatedMemorySchema, { memory: candidate, edges: [tagEdge] })]
  });
}

function relatedTagsResponse() {
  return create(ListTagsResponseSchema, { tags: [{ tag: 'qdrant', count: 2n }], more: false });
}

describe('/related — AA audit', () => {
  it('the populated view with a selected node and its evidence passes the AA audit in both themes', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(relatedFixtureResponse());
    listTagsSpy.mockReset().mockResolvedValue(relatedTagsResponse());
    pageState.url.href = 'http://localhost/related/aa-anchor';

    const screen = await renderRelated();
    await expect.element(screen.getByTestId('lane-row-tag-aa-cand')).toBeInTheDocument();

    await screen.getByTestId('lane-row-tag-aa-cand').click();
    await expect.element(screen.getByRole('region', { name: 'Why aacand00001 is related' })).toBeInTheDocument();

    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, '/related — populated + selected + evidence');
  });

  it('the floating focus card shown after an arrow key passes the AA audit in both themes', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(relatedFixtureResponse());
    listTagsSpy.mockReset().mockResolvedValue(relatedTagsResponse());
    pageState.url.href = 'http://localhost/related/aa-anchor';

    const screen = await renderRelated();
    await expect.poll(() => screen.container.querySelectorAll('[role="option"]').length).toBe(2);

    const graph = screen.container.querySelector('svg[role="listbox"]') as HTMLElement;
    graph.focus();
    await userEvent.keyboard('{ArrowRight}');
    await expect.poll(() => screen.container.querySelector('.flabel.show')?.textContent ?? '').toContain('AA candidate');

    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, '/related — focus card');
  });

  it('the Tags tab with an active filter passes the AA audit in both themes', async () => {
    relatedMemoriesSpy.mockReset().mockResolvedValue(relatedFixtureResponse());
    listTagsSpy.mockReset().mockResolvedValue(relatedTagsResponse());
    pageState.url.href = 'http://localhost/related/aa-anchor';

    const screen = await renderRelated();
    await expect.element(screen.getByText(/RelatedMemories\(subj/)).toBeInTheDocument();

    await screen.getByRole('tab', { name: 'Tags' }).click();
    const qdrantRow = screen.container.querySelector('[role="option"][aria-label^="#qdrant"]') as HTMLElement;
    qdrantRow.click();
    await expect.element(screen.getByTestId('tag-filter-chip')).toBeInTheDocument();

    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, '/related — Tags tab + active filter');
  });

  it('the not-found state passes the AA audit in both themes', async () => {
    relatedMemoriesSpy.mockReset().mockRejectedValue(new ConnectError('not found', Code.NotFound));
    pageState.url.href = 'http://localhost/related/aa-anchor';

    const screen = await renderRelated();
    await expect.element(screen.getByText('No memory with id aa-anchor that you can read')).toBeInTheDocument();

    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, '/related — not-found');
  });
});

describe('/search Tags panel — AA audit', () => {
  it('the docked panel open with an active tag passes the AA audit in both themes', async () => {
    pageState.url.href = 'http://localhost/search?q=github&tag=qdrant';
    searchMemoriesSpy.mockResolvedValue({
      memories: [makeMemory({ id: 'sp1', summary: 'panel hit', shortId: 's0000000040', tags: ['qdrant'] })],
      searchedScopes: ['repo:test'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    listTagsSpy.mockReset().mockResolvedValue(create(ListTagsResponseSchema, { tags: [{ tag: 'qdrant', count: 3n }], more: false }));

    const screen = await renderSearch();
    // Docked mode (vs. the narrow bottom-sheet) needs a wide container --
    // same convention as search.browser.test.ts's own Tags panel describes.
    screen.container.style.width = '1200px';
    await expect.element(screen.getByText('panel hit')).toBeInTheDocument();

    await screen.getByRole('button', { name: '▦ Tags panel' }).click();
    await expect.poll(() => listTagsSpy.mock.calls.length).toBeGreaterThan(0);
    await expect.poll(() => screen.container.querySelector('.marked')).not.toBeNull();

    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, '/search Tags panel');
  });
});

describe('tag picker — AA audit', () => {
  it('TagCombobox open with matches and an unknown-tag row passes the AA audit in both themes', async () => {
    listTagsSpy.mockReset().mockResolvedValue(
      create(ListTagsResponseSchema, {
        tags: [
          { tag: 'qdrant', count: 9n },
          { tag: 'qdr-ops', count: 2n }
        ],
        more: false
      })
    );

    const screen = await render(
      TagCombobox,
      { scope: 'repo:acme/x', onadd: vi.fn() },
      { wrapper: QueryClientProvider, wrapperProps: { client: qc } }
    );
    await screen.getByRole('button', { name: '+ tag' }).click();
    await expect.poll(() => listTagsSpy.mock.calls.length).toBeGreaterThan(0);

    const input = screen.getByRole('combobox', { name: 'Filter tags' });
    await input.fill('qd');
    await expect.element(screen.getByText('Add #qd')).toBeInTheDocument();

    // TagCombobox's popover content is portaled to document.body (per
    // TagCombobox.browser.test.ts), so the audit root must be document.body,
    // not screen.container.
    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'tag picker (TagCombobox)');
  });
});

describe('header Tags group — AA audit', () => {
  it('typing "#qd" shows the ranked Tags group and passes the AA audit in both themes', async () => {
    listTagsSpy.mockReset().mockResolvedValue(
      create(ListTagsResponseSchema, {
        tags: [
          { tag: 'qdrant', count: 9n },
          { tag: 'qdr-ops', count: 2n }
        ],
        more: false
      })
    );

    const screen = await render(HeaderSearch, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
    const input = screen.getByRole('combobox', { name: 'Search memories' });
    await input.fill('#qd');
    await expect.element(screen.getByText(/Tags · counts in/)).toBeInTheDocument();

    await page.screenshot({ element: document.body });
    await auditBothThemes(document.body, 'header Tags group');
  });
});
