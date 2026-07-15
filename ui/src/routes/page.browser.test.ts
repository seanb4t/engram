import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { persistResume, RESUME_KEY } from '$lib/resume';
import RootPage from './+page.svelte';

const { gotoSpy, listScopesSpy, listMemoriesSpy } = vi.hoisted(() => ({
  gotoSpy: vi.fn(),
  listScopesSpy: vi.fn(),
  listMemoriesSpy: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: { ...actual.engram, listScopes: listScopesSpy, listMemories: listMemoriesSpy }
  };
});

let qc: QueryClient;
function renderRoot() {
  return render(RootPage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

beforeEach(() => {
  gotoSpy.mockReset();
  listScopesSpy.mockReset().mockResolvedValue({ scopes: [], approximate: false });
  listMemoriesSpy.mockReset().mockResolvedValue({ memories: [], total: 0n, approximate: false });
  sessionStorage.clear();
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

// REAL root-/ui/ landing coverage (Codex round-3 MEDIUM): the prior suite
// only mounted the observe route directly and never exercised the actual
// /ui/ root's peek/goto/base-normalization behavior -- the OIDC callback
// always lands HERE (handlers.go:187), not the originating route.
describe('/ui/ root landing — resume envelope redirect (Codex round-3 HIGH)', () => {
  it('peeks the envelope and goto()s the base + base-relative returnPath WITHOUT deleting it first (peek-not-consume)', async () => {
    persistResume({
      returnPath: '/observe?scope=repo%3Ax',
      kind: 'memory',
      mode: 'create',
      recordId: null,
      values: { content: 'draft' }
    });

    await renderRoot();
    await expect.poll(() => gotoSpy.mock.calls.length).toBe(1);
    expect(gotoSpy).toHaveBeenCalledWith('/ui/observe?scope=repo%3Ax');
    // Peek-not-consume: the envelope must still be present after the
    // redirect fires -- the destination route is the one that consumes it,
    // only after its form acknowledges applying the restored values.
    expect(sessionStorage.getItem(RESUME_KEY)).not.toBeNull();
  });

  it('normalizes a returnPath mistakenly stored WITH the /ui base prefix to a single /ui (no /ui/ui/ double-prefix)', async () => {
    persistResume({
      returnPath: '/ui/observe?scope=repo%3Ax',
      kind: 'memory',
      mode: 'create',
      recordId: null,
      values: {}
    });

    await renderRoot();
    await expect.poll(() => gotoSpy.mock.calls.length).toBe(1);
    expect(gotoSpy).toHaveBeenCalledWith('/ui/observe?scope=repo%3Ax');
  });

  it('routes back to /ui/discovery for a discovery-kind envelope', async () => {
    persistResume({
      returnPath: '/discovery?scope=discovery%3Arepo%3Ax',
      kind: 'discovery',
      mode: 'create',
      recordId: null,
      values: {}
    });

    await renderRoot();
    await expect.poll(() => gotoSpy.mock.calls.length).toBe(1);
    expect(gotoSpy).toHaveBeenCalledWith('/ui/discovery?scope=discovery%3Arepo%3Ax');
  });

  it('does nothing when no envelope is present', async () => {
    await renderRoot();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(gotoSpy).not.toHaveBeenCalled();
  });

  it('rejects and discards an envelope whose returnPath is not an allowed console destination', async () => {
    persistResume({
      returnPath: 'https://evil.example/phish',
      kind: 'memory',
      mode: 'create',
      recordId: null,
      values: {}
    });

    await renderRoot();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(gotoSpy).not.toHaveBeenCalled();
    expect(sessionStorage.getItem(RESUME_KEY)).toBeNull();
  });
});
