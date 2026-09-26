import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { ConnectError, Code } from '@connectrpc/connect';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import HeaderSearch from './HeaderSearch.svelte';

const { gotoSpy, searchMemoriesSpy, getMemorySpy, listScopesSpy, listMemoriesSpy } = vi.hoisted(() => ({
  gotoSpy: vi.fn(),
  searchMemoriesSpy: vi.fn(),
  getMemorySpy: vi.fn(),
  listScopesSpy: vi.fn(),
  listMemoriesSpy: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));

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

function fakeMemory(
  overrides: Partial<{ id: string; summary: string; category: string; scope: string }> = {}
): Memory {
  return create(MemorySchema, {
    id: 'm1',
    content: 'body',
    scope: 'repo:x',
    category: 'gotcha',
    summary: 'a summary about github integration',
    createdAt: timestampFromDate(new Date()),
    ...overrides
  });
}

let qc: QueryClient;
function renderHeaderSearch() {
  return render(HeaderSearch, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

beforeEach(() => {
  gotoSpy.mockReset();
  searchMemoriesSpy.mockReset().mockResolvedValue({ memories: [] });
  getMemorySpy.mockReset();
  listScopesSpy.mockReset().mockResolvedValue({ scopes: [], approximate: false });
  listMemoriesSpy.mockReset().mockResolvedValue({ memories: [], total: 0n });
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('HeaderSearch', () => {
  it('shows the Commands section for an empty query and issues no RPC', async () => {
    const screen = await renderHeaderSearch();
    const input = screen.getByRole('combobox', { name: 'Search memories' });
    await expect.element(input).toBeInTheDocument();
    await input.click();

    await expect.element(screen.getByRole('option', { name: 'Observe', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Search', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Discovery', exact: true })).toBeInTheDocument();
    expect(searchMemoriesSpy).not.toHaveBeenCalled();
    expect(getMemorySpy).not.toHaveBeenCalled();
  });

  it('SC2: a term absent from every static label calls SearchMemories and never shows "no matches"', async () => {
    searchMemoriesSpy.mockResolvedValue({ memories: [fakeMemory({ id: 'a' }), fakeMemory({ id: 'b' })] });
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('github');

    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBe(1);
    const [req, opts] = searchMemoriesSpy.mock.calls[0] as [Record<string, unknown>, { signal?: AbortSignal }];
    expect(req).toMatchObject({ query: 'github', crossSpine: true, scope: '', k: 50n, full: false });
    expect(opts.signal).toBeInstanceOf(AbortSignal);

    await expect
      .element(screen.getByRole('option', { name: /a summary about github/ }).first())
      .toBeInTheDocument();
    expect(screen.container.textContent).not.toContain('no matches');
  });

  it('shows the honest empty-hits copy — never "no matches" — for zero memories', async () => {
    searchMemoriesSpy.mockResolvedValue({ memories: [] });
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('github');

    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBe(1);
    await expect
      .element(screen.getByText('No memories match github in any scope you can read'))
      .toBeInTheDocument();
    expect(screen.container.textContent).not.toContain('no matches');
  });

  it('resolves a pasted UUID via GetMemory and never calls SearchMemories', async () => {
    const uuid = '753aba22-0000-4000-8000-000000000001';
    getMemorySpy.mockResolvedValue({ memory: fakeMemory({ id: uuid, summary: 'resolved by id' }) });
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill(uuid);

    await expect.poll(() => getMemorySpy.mock.calls.length).toBe(1);
    expect(getMemorySpy).toHaveBeenCalledWith({ id: uuid }, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(searchMemoriesSpy).not.toHaveBeenCalled();

    await expect.element(screen.getByText(/Resolved id/)).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: /resolved by id/ })).toBeInTheDocument();
  });

  it('renders a rejected envelope with the mono field=/hint= text and its fix rows (ENTRY-05)', async () => {
    searchMemoriesSpy.mockRejectedValue(
      new ConnectError(
        'field=scope,cross_spine hint=conditional_required: scope is required unless cross_spine is true',
        Code.FailedPrecondition
      )
    );
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('github');

    await expect.element(screen.getByText('Server rejected the request')).toBeInTheDocument();
    await expect
      .element(
        screen.getByText(/field=scope,cross_spine hint=conditional_required: scope is required unless cross_spine is true/)
      )
      .toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Re-enable cross-spine' })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Pick one scope instead' })).toBeInTheDocument();
  });

  it('renders the D-04 ambiguous short_id warning with no candidate list', async () => {
    getMemorySpy.mockRejectedValue(new ConnectError('ambiguous short id: k3m9p2qr7a', Code.FailedPrecondition));
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('k3m9p2qr7a');

    await expect
      .element(screen.getByText('short_id k3m9p2qr7a is ambiguous — paste the full id to be exact'))
      .toBeInTheDocument();
    await expect.element(screen.getByText('Memories', { exact: true })).not.toBeInTheDocument();
    expect(searchMemoriesSpy).not.toHaveBeenCalled();
  });

  it('renders the not-found line for a UUID GetMemory reports missing', async () => {
    const uuid = '753aba22-0000-4000-8000-000000000001';
    getMemorySpy.mockRejectedValue(new ConnectError('not found', Code.NotFound));
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill(uuid);

    await expect
      .element(
        screen.getByText(`No memory with id ${uuid} that you can read · not-found and not-yours look the same by design`)
      )
      .toBeInTheDocument();
  });

  it('re-searches a short_id-shaped miss as text and says so, never silently reinterpreting', async () => {
    getMemorySpy.mockRejectedValue(new ConnectError('not found', Code.NotFound));
    searchMemoriesSpy.mockResolvedValue({ memories: [] });
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('k3m9p2qr7a');

    await expect.poll(() => getMemorySpy.mock.calls.length).toBe(1);
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBe(1);
    expect(searchMemoriesSpy.mock.calls[0][0]).toMatchObject({ query: 'k3m9p2qr7a' });
    await expect
      .element(screen.getByText('No short_id attachment. Searched it as text instead: k3m9p2qr7a'))
      .toBeInTheDocument();
  });

  it('shows interpreted-as chips for scope: and sends the scope with crossSpine false', async () => {
    searchMemoriesSpy.mockResolvedValue({ memories: [] });
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('scope:repo:engram gofmt');

    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBe(1);
    expect(searchMemoriesSpy.mock.calls[0][0]).toMatchObject({
      query: 'gofmt',
      scope: 'repo:engram',
      crossSpine: false
    });
    await expect.element(screen.getByText('text "gofmt"')).toBeInTheDocument();
    await expect.element(screen.getByText('scope:repo:engram')).toBeInTheDocument();
  });

  it('leaves a dashed "+ cross-spine" ghost chip when the cross-spine chip is removed, and sends the real rejection', async () => {
    const err = new ConnectError(
      'field=scope,cross_spine hint=conditional_required: scope is required unless cross_spine is true',
      Code.FailedPrecondition
    );
    searchMemoriesSpy.mockResolvedValueOnce({ memories: [] }).mockRejectedValueOnce(err);
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('github');
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBe(1);

    await screen.getByRole('button', { name: 'remove cross-spine' }).click();

    await expect.element(screen.getByRole('button', { name: '+ cross-spine' })).toBeInTheDocument();
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBe(2);
    expect(searchMemoriesSpy.mock.calls[1][0]).toMatchObject({ crossSpine: false, scope: '' });
    await expect.element(screen.getByText('Server rejected the request')).toBeInTheDocument();
  });

  it('renders an unknown is: category as a danger chip', async () => {
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('is:bogus');

    const chipsRow = screen.getByTestId('header-search-chips');
    await expect.element(chipsRow).toHaveTextContent('is:bogus');
    await expect.element(chipsRow.getByText('is:bogus')).toHaveClass(/text-destructive/);
  });

  it('shows the per-state recall-gate hidden-count note', async () => {
    searchMemoriesSpy.mockResolvedValue({
      memories: [fakeMemory()],
      searchedScopes: ['repo:x'],
      recallGateHidden: { total: 1n, archived: 0n, superseded: 1n, expired: 0n, scheduled: 0n }
    });
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('github');

    await expect
      .element(screen.getByText('+1 superseded match hidden by the recall gate (fetch by id)', { exact: false }))
      .toBeInTheDocument();
  });

  it('shows a "previous results ·" prefix while a new query is in flight', async () => {
    let resolveFirst!: (v: unknown) => void;
    searchMemoriesSpy.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveFirst = resolve;
        })
    );
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('github');
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBe(1);
    resolveFirst({ memories: [fakeMemory()], searchedScopes: ['repo:x'] });
    await expect.element(screen.getByTestId('header-search-status')).toHaveTextContent('1 hit across 1 scope');

    let resolveSecond!: (v: unknown) => void;
    searchMemoriesSpy.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveSecond = resolve;
        })
    );
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('github2');
    await expect.poll(() => searchMemoriesSpy.mock.calls.length).toBe(2);

    await expect.element(screen.getByTestId('header-search-status')).toHaveTextContent('previous results ·');
    resolveSecond({ memories: [], searchedScopes: [] });
  });

  it('expands a per-scope coverage list from the source button, ending with the discovery note (D-06)', async () => {
    searchMemoriesSpy.mockResolvedValue({
      memories: [fakeMemory({ id: 'a', scope: 'repo:a' }), fakeMemory({ id: 'b', scope: 'repo:b' })],
      searchedScopes: ['repo:a', 'repo:b', 'repo:c']
    });
    const screen = await renderHeaderSearch();
    await screen.getByRole('combobox', { name: 'Search memories' }).fill('github');

    const sourceButton = screen.getByRole('button', { name: 'SearchMemories · cross_spine · 3 scopes searched' });
    await expect.element(sourceButton).toBeInTheDocument();
    await sourceButton.click();

    await expect
      .element(screen.getByText('discovery:* not included (separate lane: search_discovery)'))
      .toBeInTheDocument();
  });
});
