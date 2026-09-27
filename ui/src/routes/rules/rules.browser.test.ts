import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import RulesPage from './+page.svelte';

// `vi.hoisted` runs before the module's own imports are linked — mirrors
// search.browser.test.ts's route test harness (04-09-PLAN.md read_first).
const {
  gotoSpy,
  pageState,
  listRulesSpy,
  getMemorySpy,
  deleteMemorySpy,
  persistResumeSpy,
  redirectToLoginSpy,
  peekResumeSpy,
  consumeResumeSpy
} = await vi.hoisted(async () => {
  const { SvelteURL } = await import('svelte/reactivity');
  const url = new SvelteURL('http://localhost/rules');
  const pageState = { url };
  const gotoSpy = vi.fn((href: string) => {
    const next = new URL(href, 'http://localhost');
    pageState.url.href = next.href;
  });
  return {
    gotoSpy,
    pageState,
    listRulesSpy: vi.fn(),
    getMemorySpy: vi.fn(),
    deleteMemorySpy: vi.fn(),
    persistResumeSpy: vi.fn(),
    redirectToLoginSpy: vi.fn(),
    peekResumeSpy: vi.fn(() => null),
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
      listRules: listRulesSpy,
      getMemory: getMemorySpy
    },
    engramWrite: {
      ...actual.engramWrite,
      deleteMemory: deleteMemorySpy
    }
  };
});

vi.mock('$lib/resume', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/resume')>();
  return {
    ...actual,
    persistResume: persistResumeSpy,
    redirectToLogin: redirectToLoginSpy,
    peekResume: peekResumeSpy,
    consumeResume: consumeResumeSpy
  };
});

function fireKey(el: Element, key: string, opts: Partial<KeyboardEventInit> = {}) {
  el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...opts }));
}

let qc: QueryClient;
function renderRules() {
  return render(RulesPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

function makeRule(overrides: MessageInitShape<typeof MemorySchema> = {}): Memory {
  return create(MemorySchema, {
    id: 'r-default',
    category: 'rule',
    summary: 'default rule summary',
    content: '',
    tags: [],
    scope: 'rule:repo:test',
    visibility: 'shared',
    owner: 'me',
    shortId: 's0000000001',
    ...overrides
  });
}

function emptyListRulesResult() {
  return { rules: [], advisory: '', searchedScopes: [], scopesTruncated: false, scopesUnknown: false };
}

beforeEach(() => {
  gotoSpy.mockClear();
  listRulesSpy.mockReset().mockResolvedValue(emptyListRulesResult());
  getMemorySpy.mockReset();
  deleteMemorySpy.mockReset();
  persistResumeSpy.mockReset();
  redirectToLoginSpy.mockReset();
  peekResumeSpy.mockReset().mockReturnValue(null);
  consumeResumeSpy.mockReset();
  pageState.url.href = 'http://localhost/rules';
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('rules route — lists every readable rule grouped by scope (CUR-03 tracer)', () => {
  it('calls ListRules with empty scopes and full false, renders scope group headers with a shared chip, and opens the pane via GetMemory on click', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [
        makeRule({ id: 'r-1', shortId: 's0000000001', scope: 'rule:repo:alpha', summary: 'alpha rule' }),
        makeRule({ id: 'r-2', shortId: 's0000000002', scope: 'rule:repo:beta', summary: 'beta rule' })
      ],
      advisory: '',
      searchedScopes: ['rule:repo:alpha', 'rule:repo:beta'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    getMemorySpy.mockResolvedValue({ memory: makeRule({ id: 'r-1', scope: 'rule:repo:alpha', content: 'full rule content' }) });

    const screen = await renderRules();

    await expect.element(screen.getByText('rule:repo:alpha')).toBeInTheDocument();

    expect(listRulesSpy).toHaveBeenCalledTimes(1);
    expect(listRulesSpy).toHaveBeenCalledWith({ scopes: [], tags: [], full: false }, expect.anything());

    await expect.element(screen.getByText('rule:repo:beta')).toBeInTheDocument();

    const chips = screen.getByText('shared');
    await expect.element(chips.first()).toBeInTheDocument();

    (screen.container.querySelector('[role="option"]') as HTMLElement).click();

    await expect.poll(() => getMemorySpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    expect(getMemorySpy).toHaveBeenCalledWith({ id: 'r-1' }, expect.anything());
    await expect.element(screen.getByText('full rule content')).toBeInTheDocument();
  });
});

describe('rules route — honest header, coverage and advisory (D-12)', () => {
  it('reads the header from count/searchedScopes and renders the scopes_truncated clause verbatim', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [
        makeRule({ id: 'r-1', scope: 'rule:repo:alpha' }),
        makeRule({ id: 'r-2', scope: 'rule:repo:beta' }),
        makeRule({ id: 'r-3', scope: 'rule:repo:beta' })
      ],
      advisory: '',
      searchedScopes: ['rule:repo:alpha', 'rule:repo:beta'],
      scopesTruncated: true,
      scopesUnknown: false
    });

    const screen = await renderRules();
    await expect.element(screen.getByText('3 rules')).toBeInTheDocument();
    await expect.element(screen.getByText('across 2 scopes', { exact: false })).toBeInTheDocument();
    await expect.element(screen.getByText('scopes_truncated: scope list incomplete', { exact: false })).toBeInTheDocument();
  });

  it('renders "across every readable scope" and the scopes_unknown clause when coverage could not be listed', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha' })],
      advisory: '',
      searchedScopes: [],
      scopesTruncated: false,
      scopesUnknown: true
    });

    const screen = await renderRules();
    await expect.element(screen.getByText('across every readable scope', { exact: false })).toBeInTheDocument();
    await expect.element(screen.getByText('scopes_unknown: scope coverage could not be listed', { exact: false })).toBeInTheDocument();
  });

  it('renders a non-empty advisory verbatim under the header', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha' })],
      advisory: 'rule:repo:alpha holds more rules than the soft per-scope threshold',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await renderRules();
    await expect
      .element(screen.getByText('rule:repo:alpha holds more rules than the soft per-scope threshold'))
      .toBeInTheDocument();
  });

  it('renders no advisory line when the response advisory is empty', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha' })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await renderRules();
    await expect.element(screen.getByText('rule:repo:alpha')).toBeInTheDocument();
    expect(screen.container.querySelector('.rules-advisory')).toBeNull();
  });
});

describe('rules route — empty, loading and error states (E5)', () => {
  it('shows the honest empty heading with no fix rows when no rules are readable', async () => {
    listRulesSpy.mockResolvedValue(emptyListRulesResult());

    const screen = await renderRules();
    await expect.element(screen.getByText('No rules in any scope you can read')).toBeInTheDocument();
    const empty = screen.container.querySelector('[data-testid="recall-empty"]') as HTMLElement;
    expect(empty.querySelectorAll('button').length).toBe(0);
  });

  it('shows the skeleton rows on first load only, never on a re-fetch (keepPreviousData)', async () => {
    let resolveFirst!: (v: unknown) => void;
    const firstPromise = new Promise((resolve) => {
      resolveFirst = resolve;
    });
    listRulesSpy.mockReturnValueOnce(firstPromise);

    const screen = await renderRules();
    await expect.element(screen.getByTestId('results-loading')).toBeInTheDocument();

    resolveFirst({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha', summary: 'first-load rule' })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    await expect.element(screen.getByText('first-load rule')).toBeInTheDocument();
    expect(screen.container.querySelector('[data-testid="results-loading"]')).toBeNull();

    // A re-fetch (triggered directly through the shared QueryClient, since
    // /rules has no facet control of its own to drive one) keeps the prior
    // row visible — never a flash back to the skeleton.
    let resolveSecond!: (v: unknown) => void;
    const secondPromise = new Promise((resolve) => {
      resolveSecond = resolve;
    });
    listRulesSpy.mockReturnValueOnce(secondPromise);
    void qc.refetchQueries({ queryKey: ['listRules'] });
    await expect.element(screen.getByText('first-load rule')).toBeInTheDocument();
    expect(screen.container.querySelector('[data-testid="results-loading"]')).toBeNull();
    resolveSecond({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha', summary: 'first-load rule' })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });
  });

  it('a rejected ListRules call renders the field=/hint= envelope and a working Retry fix row', async () => {
    listRulesSpy.mockRejectedValue(
      new ConnectError('field=scopes hint=too_many: at most 1000 rules', Code.FailedPrecondition)
    );

    const screen = await renderRules();
    await expect.element(screen.getByText('Server rejected the request')).toBeInTheDocument();
    await expect
      .element(screen.getByText('field=scopes hint=too_many: at most 1000 rules', { exact: false }))
      .toBeInTheDocument();

    listRulesSpy.mockClear();
    listRulesSpy.mockResolvedValue(emptyListRulesResult());
    await screen.getByRole('button', { name: 'Retry the request' }).click();
    await expect.poll(() => listRulesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
  });
});

describe('rules route — adjacency and ordering (CUR-03)', () => {
  it('two identical summaries in different scopes render as two rows under two separate headers', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [
        makeRule({ id: 'r-1', scope: 'rule:repo:alpha', summary: 'reuse existing tags before storing' }),
        makeRule({ id: 'r-2', scope: 'rule:repo:beta', summary: 'reuse existing tags before storing' })
      ],
      advisory: '',
      searchedScopes: ['rule:repo:alpha', 'rule:repo:beta'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await renderRules();
    await expect.element(screen.getByText('rule:repo:alpha')).toBeInTheDocument();
    await expect.element(screen.getByText('rule:repo:beta')).toBeInTheDocument();
    expect(screen.container.querySelectorAll('.results-group-header').length).toBe(2);
    expect(screen.container.querySelectorAll('[role="option"]').length).toBe(2);
  });

  it('two identical summaries sharing one scope render as two rows under one header', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [
        makeRule({ id: 'r-1', scope: 'rule:repo:alpha', summary: 'reuse existing tags before storing' }),
        makeRule({ id: 'r-2', scope: 'rule:repo:alpha', summary: 'reuse existing tags before storing' })
      ],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await renderRules();
    await expect.element(screen.getByText('rule:repo:alpha')).toBeInTheDocument();
    expect(screen.container.querySelectorAll('.results-group-header').length).toBe(1);
    expect(screen.container.querySelectorAll('[role="option"]').length).toBe(2);
  });
});

describe('rules route — long summaries stay one line (E5/CUR-03 backstop)', () => {
  it('a 512-byte multi-byte summary keeps the same row height as a short one', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-short', scope: 'rule:repo:alpha', summary: 'short' })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    const screen = await renderRules();
    await expect.element(screen.getByText('short', { exact: true })).toBeInTheDocument();
    const shortHeight = (screen.container.querySelector('[role="option"]') as HTMLElement).getBoundingClientRect()
      .height;

    // A 512-byte multi-byte (3-byte-per-char) summary — well past a naive
    // per-character truncation and past the 512-byte server bound alike.
    const longSummary = '漢'.repeat(170);
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-long', scope: 'rule:repo:alpha', summary: longSummary })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    void qc.refetchQueries({ queryKey: ['listRules'] });
    await expect.element(screen.getByText(longSummary)).toBeInTheDocument();
    const longHeight = (screen.container.querySelector('[role="option"]') as HTMLElement).getBoundingClientRect()
      .height;
    expect(longHeight).toBe(shortHeight);
    await page.screenshot();
  });
});

describe('rules route — delete only (D-12, CUR-05)', () => {
  it('the pane and legend offer delete only — no edit/visibility/archive/restore/supersede/selection', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha', summary: 'alpha rule' })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    getMemorySpy.mockResolvedValue({
      memory: makeRule({ id: 'r-1', scope: 'rule:repo:alpha', content: 'full content' })
    });

    const screen = await renderRules();
    await expect.element(screen.getByText('alpha rule')).toBeInTheDocument();

    const legend = screen.container.querySelector('.results-legend') as HTMLElement;
    const kbdTexts = Array.from(legend.querySelectorAll('kbd')).map((el) => el.textContent);
    expect(kbdTexts).toEqual(['j', 'k', '↵', 'esc', '#', 'c', '⇧C']);
    expect(screen.container.querySelector('.row-check')).toBeNull();

    (screen.container.querySelector('[role="option"]') as HTMLElement).click();
    await expect.element(screen.getByText('full content')).toBeInTheDocument();

    const detail = screen.container.querySelector('[aria-label="Memory detail"]') as HTMLElement;
    const actionButtons = Array.from(detail.querySelectorAll('.d-actions button')).map((b) => b.textContent?.trim());
    expect(actionButtons).toEqual(['Delete']);
  });

  it('# opens the delete confirm for the active row', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha', summary: 'alpha rule' })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });

    const screen = await renderRules();
    await expect.element(screen.getByText('alpha rule')).toBeInTheDocument();
    const listbox = screen.getByRole('listbox', { name: 'Rules' });
    listbox.element().focus();
    fireKey(listbox.element(), '#');

    await expect.element(screen.getByText('Delete this rule?')).toBeInTheDocument();
  });

  it('Delete calls deleteMemory with the id, clears sel and invalidates listRules', async () => {
    pageState.url.href = 'http://localhost/rules?sel=r-1';
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha', summary: 'alpha rule' })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    getMemorySpy.mockResolvedValue({
      memory: makeRule({ id: 'r-1', scope: 'rule:repo:alpha', content: 'full content' })
    });
    deleteMemorySpy.mockResolvedValue({});

    const screen = await renderRules();
    await expect.element(screen.getByText('alpha rule')).toBeInTheDocument();
    const listbox = screen.getByRole('listbox', { name: 'Rules' });
    listbox.element().focus();
    fireKey(listbox.element(), '#');
    await expect.element(screen.getByText('Delete this rule?')).toBeInTheDocument();

    listRulesSpy.mockClear();
    await screen.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();

    await expect.poll(() => deleteMemorySpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    expect(deleteMemorySpy.mock.calls[0][0].id).toBe('r-1');
    await expect.poll(() => pageState.url.searchParams.get('sel')).toBe(null);
    await expect.poll(() => listRulesSpy.mock.calls.length).toBeGreaterThanOrEqual(1);
    await expect.element(screen.getByText('Delete this rule?')).not.toBeInTheDocument();
  });

  it('an Unauthenticated delete shows the re-auth block and persists a v2 delete envelope before redirecting', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha', summary: 'alpha rule' })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    deleteMemorySpy.mockRejectedValue(new ConnectError('unauthenticated', Code.Unauthenticated));

    const screen = await renderRules();
    await expect.element(screen.getByText('alpha rule')).toBeInTheDocument();
    const listbox = screen.getByRole('listbox', { name: 'Rules' });
    listbox.element().focus();
    fireKey(listbox.element(), '#');
    await expect.element(screen.getByText('Delete this rule?')).toBeInTheDocument();

    await screen.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
    await expect
      .element(screen.getByText('write failed — session expired. re-authenticate to continue.'))
      .toBeInTheDocument();

    await screen.getByRole('button', { name: 'Re-authenticate' }).click();
    expect(persistResumeSpy).toHaveBeenCalledWith({
      returnPath: '/rules',
      kind: 'delete',
      id: 'r-1'
    });
    expect(redirectToLoginSpy).toHaveBeenCalledTimes(1);
  });

  it('a seeded delete envelope reopens the confirm with the review-and-resend notice, deletes nothing until Delete is clicked, and consumes the envelope once', async () => {
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha', summary: 'alpha rule' })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    peekResumeSpy.mockReturnValue({
      v: 2,
      ts: Date.now(),
      returnPath: '/rules',
      kind: 'delete',
      id: 'r-1'
    });
    deleteMemorySpy.mockResolvedValue({});

    const screen = await renderRules();
    await expect.element(screen.getByText('Delete this rule?')).toBeInTheDocument();
    await expect.element(screen.getByText('Signed in again — review and resend')).toBeInTheDocument();
    expect(deleteMemorySpy).not.toHaveBeenCalled();
    expect(consumeResumeSpy).toHaveBeenCalledTimes(1);

    await screen.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
    await expect.poll(() => deleteMemorySpy.mock.calls.length).toBe(1);
    expect(deleteMemorySpy.mock.calls[0][0].id).toBe('r-1');
  });

  it('a NotFound delete closes the confirm and clears sel without crashing', async () => {
    pageState.url.href = 'http://localhost/rules?sel=r-1';
    listRulesSpy.mockResolvedValue({
      rules: [makeRule({ id: 'r-1', scope: 'rule:repo:alpha', summary: 'alpha rule' })],
      advisory: '',
      searchedScopes: ['rule:repo:alpha'],
      scopesTruncated: false,
      scopesUnknown: false
    });
    getMemorySpy.mockResolvedValue({
      memory: makeRule({ id: 'r-1', scope: 'rule:repo:alpha', content: 'full content' })
    });
    deleteMemorySpy.mockRejectedValue(new ConnectError('not found', Code.NotFound));

    const screen = await renderRules();
    await expect.element(screen.getByText('alpha rule')).toBeInTheDocument();
    const listbox = screen.getByRole('listbox', { name: 'Rules' });
    listbox.element().focus();
    fireKey(listbox.element(), '#');
    await expect.element(screen.getByText('Delete this rule?')).toBeInTheDocument();

    await screen.getByRole('dialog').getByRole('button', { name: 'Delete' }).click();
    await expect.element(screen.getByText('Delete this rule?')).not.toBeInTheDocument();
    await expect.poll(() => pageState.url.searchParams.get('sel')).toBe(null);
  });
});
