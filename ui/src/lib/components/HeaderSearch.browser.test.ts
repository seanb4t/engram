import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import HeaderSearch from './HeaderSearch.svelte';

const { gotoSpy, searchMemoriesSpy, getMemorySpy } = vi.hoisted(() => ({
  gotoSpy: vi.fn(),
  searchMemoriesSpy: vi.fn(),
  getMemorySpy: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: { ...actual.engram, searchMemories: searchMemoriesSpy, getMemory: getMemorySpy }
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
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('HeaderSearch', () => {
  it('shows the Commands section for an empty query and issues no RPC', async () => {
    const screen = await renderHeaderSearch();
    const input = screen.getByRole('combobox', { name: 'Search memories' });
    await expect.element(input).toBeInTheDocument();
    await input.click();

    await expect.element(screen.getByRole('option', { name: 'Observe' })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Search', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Discovery' })).toBeInTheDocument();
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
});
