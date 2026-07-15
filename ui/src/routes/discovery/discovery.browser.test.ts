import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { persistResume } from '$lib/resume';
import DiscoveryPage from './+page.svelte';

const { gotoSpy, pageState, searchDiscoveriesSpy, getMemorySpy, consumeResumeSpy } = vi.hoisted(() => ({
  gotoSpy: vi.fn(),
  pageState: { url: new URL('http://localhost/discovery') },
  searchDiscoveriesSpy: vi.fn(),
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
    engram: { ...actual.engram, searchDiscoveries: searchDiscoveriesSpy, getMemory: getMemorySpy }
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
function renderDiscovery() {
  return render(DiscoveryPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

beforeEach(() => {
  gotoSpy.mockReset();
  searchDiscoveriesSpy.mockReset().mockResolvedValue({ discoveries: [] });
  getMemorySpy.mockReset();
  consumeResumeSpy.mockReset();
  sessionStorage.clear();
  pageState.url = new URL('http://localhost/discovery');
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('discovery route — New discovery + no edit surface (D-04)', () => {
  it('defaults the create scope to the discovery:-prefixed ?scope param', async () => {
    pageState.url = new URL('http://localhost/discovery?scope=discovery:repo:x');
    const screen = await renderDiscovery();
    await screen.getByRole('button', { name: 'New discovery' }).click();
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('discovery:repo:x');
  });

  it('never seeds the create scope from a raw (non-discovery:-prefixed) memory scope', async () => {
    pageState.url = new URL('http://localhost/discovery?scope=repo:x');
    const screen = await renderDiscovery();
    await screen.getByRole('button', { name: 'New discovery' }).click();
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('');
  });
});

// Discovery-create recovery (Codex round-3 MEDIUM): the prior suite never
// covered the discovery route reopening its create-only sheet from a
// resume envelope.
describe('discovery route — re-auth landing recovery', () => {
  it('reopens the DiscoveryFormSheet with restored values from a seeded create-mode envelope, then consumes it once', async () => {
    persistResume({
      returnPath: '/discovery?q=x',
      kind: 'discovery',
      mode: 'create',
      recordId: null,
      values: { content: 'restored discovery draft', scope: 'discovery:repo:restored', kind: 'fact' }
    });

    const screen = await renderDiscovery();
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await expect.element(screen.getByRole('heading', { name: 'New discovery' })).toBeInTheDocument();
    await expect.element(screen.getByLabelText('content')).toHaveValue('restored discovery draft');
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('discovery:repo:restored');
    await expect.poll(() => consumeResumeSpy.mock.calls.length).toBe(1);
  });

  it('does not reopen anything for a memory-kind envelope (kind mismatch)', async () => {
    persistResume({
      returnPath: '/observe',
      kind: 'memory',
      mode: 'create',
      recordId: null,
      values: {}
    });
    const screen = await renderDiscovery();
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
    expect(consumeResumeSpy).not.toHaveBeenCalled();
  });
});
