import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { persistResume } from '$lib/resume';
import SearchPage from './+page.svelte';

const { gotoSpy, pageState, searchMemoriesSpy, getMemorySpy, consumeResumeSpy } = vi.hoisted(() => ({
  gotoSpy: vi.fn(),
  pageState: { url: new URL('http://localhost/search') },
  searchMemoriesSpy: vi.fn(),
  getMemorySpy: vi.fn(),
  consumeResumeSpy: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));
vi.mock('$app/state', () => ({ page: pageState }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: { ...actual.engram, searchMemories: searchMemoriesSpy, getMemory: getMemorySpy }
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

beforeEach(() => {
  gotoSpy.mockReset();
  searchMemoriesSpy.mockReset().mockResolvedValue({ memories: [] });
  getMemorySpy.mockReset();
  consumeResumeSpy.mockReset();
  sessionStorage.clear();
  pageState.url = new URL('http://localhost/search');
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
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
