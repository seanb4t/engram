import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import RulesPage from './+page.svelte';

// `vi.hoisted` runs before the module's own imports are linked — mirrors
// search.browser.test.ts's route test harness (04-09-PLAN.md read_first).
const { gotoSpy, pageState, listRulesSpy, getMemorySpy } = await vi.hoisted(async () => {
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
      listRules: listRulesSpy,
      getMemory: getMemorySpy
    }
  };
});

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
