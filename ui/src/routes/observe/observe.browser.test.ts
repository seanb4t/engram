import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { QueryClient, QueryClientProvider, QueryCache } from '@tanstack/svelte-query';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import { persistResume, RESUME_KEY } from '$lib/resume';
import { errorBanner, reportError } from '$lib/errors';
import { mapAuthError } from '$lib/client';
import ObservePage from './+page.svelte';
import DeleteBannerHarness from './DeleteBannerHarness.svelte';

const { gotoSpy, pageState, listScopesSpy, listMemoriesSpy, getMemorySpy, deleteMemorySpy, consumeResumeSpy } =
  vi.hoisted(() => ({
    gotoSpy: vi.fn(),
    pageState: { url: new URL('http://localhost/observe') },
    listScopesSpy: vi.fn(),
    listMemoriesSpy: vi.fn(),
    getMemorySpy: vi.fn(),
    deleteMemorySpy: vi.fn(),
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
    },
    engramWrite: {
      ...actual.engramWrite,
      deleteMemory: deleteMemorySpy
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
  deleteMemorySpy.mockReset().mockResolvedValue({});
  consumeResumeSpy.mockReset();
  errorBanner.set(null);
  sessionStorage.clear();
  pageState.url = new URL('http://localhost/observe');
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('observe route — onedit fetches the FULL record (Codex round-2 HIGH)', () => {
  it("pressing 'e' on the active row triggers openEdit's GetMemory fetch, never a summary-shaped prefill", async () => {
    pageState.url = new URL('http://localhost/observe?scope=repo:x');
    const rowMemory = fakeMemory({ id: 'r1', content: '', summary: 'row summary (content cleared)', scope: 'repo:x' });
    listMemoriesSpy.mockResolvedValue({ memories: [rowMemory], total: 1n, approximate: false });
    getMemorySpy.mockResolvedValue({ memory: fakeMemory({ id: 'r1', content: 'the real full body', scope: 'repo:x' }) });

    const screen = await renderObserve();
    await expect.element(screen.getByText('row summary (content cleared)')).toBeInTheDocument();
    const listbox = screen.getByRole('listbox', { name: 'Memories in repo:x' });
    listbox.element().focus();
    await userEvent.keyboard('e');

    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    expect(getMemorySpy).toHaveBeenCalledWith({ id: 'r1' });
    await expect.element(screen.getByLabelText('content')).toHaveValue('the real full body');
  });
});

// D-12: /observe adopts the shared ResultsList/RecallSplit/DetailPane set.
describe('observe route — shared list and pane (D-12)', () => {
  it('shows a "select a scope" guidance when no scope is chosen', async () => {
    pageState.url = new URL('http://localhost/observe');
    const screen = await renderObserve();
    await expect.element(screen.getByText('select a scope')).toBeInTheDocument();
  });

  it('renders a listbox named "Memories in <scope>" with ResultsList rows', async () => {
    pageState.url = new URL('http://localhost/observe?scope=repo:x');
    listMemoriesSpy.mockResolvedValue({
      memories: [fakeMemory({ id: 'r1', summary: 'first row', scope: 'repo:x' }), fakeMemory({ id: 'r2', summary: 'second row', scope: 'repo:x' })],
      total: 2n,
      approximate: false
    });

    const screen = await renderObserve();
    const listbox = screen.getByRole('listbox', { name: 'Memories in repo:x' });
    await expect.element(listbox).toBeInTheDocument();
    await expect.element(screen.getByText('first row')).toBeInTheDocument();
    await expect.element(screen.getByText('second row')).toBeInTheDocument();
  });

  it('clicking a row sets sel and opens the DetailPane', async () => {
    pageState.url = new URL('http://localhost/observe?scope=repo:x');
    const rowMemory = fakeMemory({ id: 'r1', summary: 'toggle me', scope: 'repo:x' });
    listMemoriesSpy.mockResolvedValue({ memories: [rowMemory], total: 1n, approximate: false });
    getMemorySpy.mockResolvedValue({ memory: rowMemory });

    const screen = await renderObserve();
    await expect.element(screen.getByText('toggle me')).toBeInTheDocument();

    (screen.container.querySelector('[role="option"]') as HTMLElement).click();
    await expect.poll(() => gotoSpy.mock.calls.length).toBeGreaterThan(0);
    expect(gotoSpy.mock.calls.at(-1)?.[0]).toContain('sel=r1');
  });

  it('clicking the already-open row clears sel', async () => {
    pageState.url = new URL('http://localhost/observe?scope=repo:x&sel=r1');
    const rowMemory = fakeMemory({ id: 'r1', summary: 'toggle me again', scope: 'repo:x' });
    listMemoriesSpy.mockResolvedValue({ memories: [rowMemory], total: 1n, approximate: false });
    getMemorySpy.mockResolvedValue({ memory: rowMemory });

    const screen = await renderObserve();
    const listbox = screen.getByRole('listbox', { name: 'Memories in repo:x' });
    await expect.element(listbox.getByText('toggle me again')).toBeInTheDocument();
    await expect.poll(() => screen.container.querySelector('[aria-label="Memory detail"]') !== null).toBe(true);

    (screen.container.querySelector('[role="option"]') as HTMLElement).click();
    await expect.poll(() => gotoSpy.mock.calls.length).toBeGreaterThan(0);
    expect(gotoSpy.mock.calls.at(-1)?.[0]).not.toContain('sel=');
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
      values: { content: 'edited-and-restored content' }
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

describe('observe route — deleting the selected record never flashes a NotFound banner (WR-04)', () => {
  it('does not refetch the tombstone or surface a role="alert" banner after deleting the selected record', async () => {
    // The layout wires QueryCache.onError -> (auth ? redirect : reportError).
    // Reproduce it exactly so a regression that re-invalidates ['getMemory', id]
    // on delete would refetch the deleted id, resolve NotFound, and set the
    // banner the harness renders.
    qc = new QueryClient({
      defaultOptions: { queries: { retry: false } },
      queryCache: new QueryCache({
        onError: (err) => {
          const target = mapAuthError(err);
          if (target) return;
          reportError(err);
        }
      })
    });

    // Select m1: the detail query fetches the full record once.
    pageState.url = new URL('http://localhost/observe?scope=repo:x&sel=m1');
    listMemoriesSpy.mockResolvedValue({
      memories: [fakeMemory({ id: 'm1', content: '', summary: 'row summary', scope: 'repo:x' })],
      total: 1n,
      approximate: false
    });
    getMemorySpy.mockResolvedValue({ memory: fakeMemory({ id: 'm1', content: 'the full body', scope: 'repo:x' }) });

    const screen = await render(DeleteBannerHarness, { client: qc });

    // Detail pane resolved (its title renders the FETCHED record's own
    // summary, "a summary" -- the fakeMemory default, distinct from the
    // list row's own "row summary" -- and the Delete button is available.
    await expect.element(screen.getByRole('heading', { name: 'a summary' })).toBeInTheDocument();
    expect(getMemorySpy).toHaveBeenCalledTimes(1);

    await screen.getByRole('button', { name: 'Delete', exact: true }).click();

    // Confirm dialog is host-authoritative; confirm the delete (scoped to the
    // dialog since DetailPane's own trigger is also named exactly "Delete").
    await expect.element(screen.getByText('Delete this memory?')).toBeInTheDocument();
    await screen.getByRole('dialog').getByRole('button', { name: 'Delete', exact: true }).click();

    // Delete succeeded: the deleted id is relayed up (ondeleted -> navigate).
    await expect.poll(() => deleteMemorySpy.mock.calls.length).toBe(1);
    await expect.poll(() => gotoSpy.mock.calls.length).toBeGreaterThan(0);

    // goto is async/mocked: `?sel` still equals the deleted id, so the detail
    // observer stays enabled. With the getMemory invalidation dropped, no
    // tombstone refetch occurs — getMemory is never re-called for the deleted id.
    await expect.poll(() => getMemorySpy.mock.calls.length).toBe(1);

    // And the global error banner never appears.
    await expect.element(screen.getByRole('alert')).not.toBeInTheDocument();
  });
});
