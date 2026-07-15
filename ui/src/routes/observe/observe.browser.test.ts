import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import { persistResume, RESUME_KEY } from '$lib/resume';
import ObservePage from './+page.svelte';

const { gotoSpy, pageState, listScopesSpy, listMemoriesSpy, getMemorySpy, consumeResumeSpy } = vi.hoisted(() => ({
  gotoSpy: vi.fn(),
  pageState: { url: new URL('http://localhost/observe') },
  listScopesSpy: vi.fn(),
  listMemoriesSpy: vi.fn(),
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
    engram: {
      ...actual.engram,
      listScopes: listScopesSpy,
      listMemories: listMemoriesSpy,
      getMemory: getMemorySpy
    }
  };
});

// consumeResume is wrapped (not replaced) so real sessionStorage deletion
// still happens -- the spy lets the tests assert the exact-once, after-ack
// timing (Codex round-3/round-4 HIGH) without duplicating resume.ts's logic.
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

function fakeMemory(
  overrides: Partial<{
    id: string;
    content: string;
    scope: string;
    category: string;
    tags: string[];
    summary: string;
    visibility: string;
  }> = {}
): Memory {
  return create(MemorySchema, {
    id: 'm1',
    content: 'the real full body, fetched via GetMemory',
    scope: 'repo:x',
    category: 'gotcha',
    tags: [],
    summary: 'a summary',
    visibility: 'private',
    ...overrides
  });
}

let qc: QueryClient;
function renderObserve() {
  return render(ObservePage, {}, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

beforeEach(() => {
  gotoSpy.mockReset();
  listScopesSpy.mockReset().mockResolvedValue({ scopes: [], approximate: false });
  listMemoriesSpy.mockReset().mockResolvedValue({ memories: [], total: 0n, approximate: false });
  getMemorySpy.mockReset();
  consumeResumeSpy.mockReset();
  sessionStorage.clear();
  pageState.url = new URL('http://localhost/observe');
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('observe route — onedit fetches the FULL record (Codex round-2 HIGH)', () => {
  it("onedit(id) on a row triggers openEdit's GetMemory fetch, never a summary-shaped prefill", async () => {
    pageState.url = new URL('http://localhost/observe?scope=repo:x');
    const rowMemory = fakeMemory({ id: 'r1', content: '', summary: 'row summary (content cleared)', scope: 'repo:x' });
    listMemoriesSpy.mockResolvedValue({ memories: [rowMemory], total: 1n, approximate: false });
    getMemorySpy.mockResolvedValue({ memory: fakeMemory({ id: 'r1', content: 'the real full body', scope: 'repo:x' }) });

    const screen = await renderObserve();
    await screen.getByRole('button', { name: 'row actions' }).click();
    await screen.getByRole('menuitem', { name: 'Edit' }).click();

    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    expect(getMemorySpy).toHaveBeenCalledWith({ id: 'r1' });
    await expect.element(screen.getByLabelText('content')).toHaveValue('the real full body');
  });
});

describe('observe route — re-auth landing recovery (Codex round-3 HIGH/MEDIUM)', () => {
  it('reopens the edit sheet from a seeded edit-mode envelope, prefilled with the FULL record overlaid by dirty values, and consumes the envelope EXACTLY ONCE after the form applies them', async () => {
    let resolveGetMemory!: (v: { memory: Memory }) => void;
    getMemorySpy.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveGetMemory = resolve;
        })
    );
    persistResume({
      returnPath: '/observe',
      kind: 'memory',
      mode: 'edit',
      recordId: 'm1',
      values: { content: 'edited-and-restored content' },
      dirtyPaths: ['content']
    });

    const screen = await renderObserve();
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
    expect(consumeResumeSpy).not.toHaveBeenCalled();

    resolveGetMemory({ memory: fakeMemory({ id: 'm1', content: 'original full body', scope: 'repo:x' }) });

    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await expect.element(screen.getByText('Edit memory')).toBeInTheDocument();
    await expect.element(screen.getByLabelText('content')).toHaveValue('edited-and-restored content');
    await expect.element(screen.getByTestId('scope-readonly')).toHaveTextContent('repo:x');

    await expect.poll(() => consumeResumeSpy.mock.calls.length).toBe(1);
    expect(sessionStorage.getItem(RESUME_KEY)).toBeNull();
  });

  it('reopens the create sheet with restored values from a seeded create-mode envelope', async () => {
    persistResume({
      returnPath: '/observe',
      kind: 'memory',
      mode: 'create',
      recordId: null,
      values: { content: 'restored draft', scope: 'repo:restored' }
    });

    const screen = await renderObserve();
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await expect.element(screen.getByRole('heading', { name: 'New memory' })).toBeInTheDocument();
    await expect.element(screen.getByLabelText('content')).toHaveValue('restored draft');
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('repo:restored');
    await expect.poll(() => consumeResumeSpy.mock.calls.length).toBe(1);
  });

  it('does not reopen anything for a discovery-kind envelope (kind mismatch)', async () => {
    persistResume({
      returnPath: '/discovery',
      kind: 'discovery',
      mode: 'create',
      recordId: null,
      values: { content: 'wrong route' }
    });
    const screen = await renderObserve();
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
    expect(consumeResumeSpy).not.toHaveBeenCalled();
    // Envelope is left for the actual discovery route to consume.
    expect(sessionStorage.getItem(RESUME_KEY)).not.toBeNull();
  });
});
