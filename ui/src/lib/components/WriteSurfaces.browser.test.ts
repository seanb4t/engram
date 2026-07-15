import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import WriteSurfaces from './WriteSurfaces.svelte';

const {
  getMemorySpy,
  deleteMemoryMutateSpy,
  setMemoryVisibilityMutateSpy,
  deleteDiscoveryMutateSpy,
  setDiscoveryVisibilityMutateSpy,
  redirectToLoginSpy
} = vi.hoisted(() => ({
  getMemorySpy: vi.fn(),
  deleteMemoryMutateSpy: vi.fn(),
  setMemoryVisibilityMutateSpy: vi.fn(),
  deleteDiscoveryMutateSpy: vi.fn(),
  setDiscoveryVisibilityMutateSpy: vi.fn(),
  redirectToLoginSpy: vi.fn()
}));

// Only engram.getMemory is faked (WriteSurfaces' openEdit fetch path) --
// engramWrite is untouched but never invoked directly (all writes go
// through the mocked mutation hooks below).
vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return { ...actual, engram: { ...actual.engram, getMemory: getMemorySpy } };
});

// Mocks only the delete/set-visibility hooks (RESEARCH-sanctioned "mocked
// hook" pattern, mirrors MemoryFormSheet/DiscoveryFormSheet browser tests) --
// useCreateMemory/useUpdateMemory/useScheduleMemory stay real since a real
// QueryClientProvider wrapper is present below and these hooks' mutationFns
// are never invoked in this suite (no Create/Save clicks).
vi.mock('$lib/mutations/memory', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/mutations/memory')>();
  return {
    ...actual,
    useDeleteMemory: () => ({ mutate: deleteMemoryMutateSpy }),
    useSetMemoryVisibility: () => ({ mutate: setMemoryVisibilityMutateSpy })
  };
});
vi.mock('$lib/mutations/discovery', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/mutations/discovery')>();
  return {
    ...actual,
    useDeleteDiscovery: () => ({ mutate: deleteDiscoveryMutateSpy }),
    useSetDiscoveryVisibility: () => ({ mutate: setDiscoveryVisibilityMutateSpy })
  };
});
vi.mock('$lib/resume', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/resume')>();
  return { ...actual, redirectToLogin: redirectToLoginSpy };
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
    content: 'full body content, fetched via GetMemory',
    scope: 'repo:x',
    category: 'gotcha',
    tags: ['a', 'b'],
    summary: 'orig summary',
    visibility: 'private',
    ...overrides
  });
}

let qc: QueryClient;
function renderWS(props: { kind: 'memory' | 'discovery'; scope: string }) {
  return render(WriteSurfaces, props, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

beforeEach(() => {
  getMemorySpy.mockReset();
  deleteMemoryMutateSpy.mockReset();
  setMemoryVisibilityMutateSpy.mockReset();
  deleteDiscoveryMutateSpy.mockReset();
  setDiscoveryVisibilityMutateSpy.mockReset();
  redirectToLoginSpy.mockReset();
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('WriteSurfaces — memory kind: New memory / create', () => {
  it('renders "New memory" and opens the create sheet on click, defaulting scope to the passed value', async () => {
    const screen = await renderWS({ kind: 'memory', scope: 'repo:default' });
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
    await screen.getByRole('button', { name: 'New memory' }).click();
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('repo:default');
  });
});

describe('WriteSurfaces — openEdit(id): full-content fetch before open', () => {
  it('fetches getMemory BEFORE opening the sheet, and prefills from the FULL record (not summary-shaped)', async () => {
    let resolveGetMemory!: (v: { memory: Memory }) => void;
    getMemorySpy.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveGetMemory = resolve;
        })
    );
    const screen = await renderWS({ kind: 'memory', scope: 'repo:default' });

    const pending = screen.component.openEdit('m1');
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
    expect(getMemorySpy).toHaveBeenCalledWith({ id: 'm1' });

    resolveGetMemory({ memory: fakeMemory({ id: 'm1', content: 'the real full body' }) });
    await pending;

    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await expect.element(screen.getByText('Edit memory')).toBeInTheDocument();
    await expect.element(screen.getByLabelText('content')).toHaveValue('the real full body');
    await expect.element(screen.getByTestId('scope-readonly')).toHaveTextContent('repo:x');
  });

  it('toasts and does not open a blank edit sheet on a fetch error', async () => {
    getMemorySpy.mockRejectedValue(new ConnectError('not found', Code.NotFound));
    const screen = await renderWS({ kind: 'memory', scope: 'repo:default' });
    await screen.component.openEdit('missing');
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
  });
});

describe('WriteSurfaces — requestDelete', () => {
  it('opens the confirm dialog and invokes the delete mutation for the id, clearing the target only on success', async () => {
    deleteMemoryMutateSpy.mockImplementation((_vars, opts) => {
      opts.onSuccess();
    });
    const screen = await renderWS({ kind: 'memory', scope: 'repo:x' });
    await screen.component.requestDelete('m1', 'memory');
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Delete' }).click();
    expect(deleteMemoryMutateSpy).toHaveBeenCalledTimes(1);
    expect(deleteMemoryMutateSpy.mock.calls[0][0]).toEqual({ id: 'm1' });
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
  });

  it('requestDelete(id, "discovery") invokes the discovery delete mutation, not the memory one', async () => {
    deleteDiscoveryMutateSpy.mockImplementation((_vars, opts) => {
      opts.onSuccess();
    });
    const screen = await renderWS({ kind: 'discovery', scope: 'discovery:repo:x' });
    await screen.component.requestDelete('d1', 'discovery');
    await screen.getByRole('button', { name: 'Delete' }).click();
    expect(deleteDiscoveryMutateSpy).toHaveBeenCalledTimes(1);
    expect(deleteDiscoveryMutateSpy.mock.calls[0][0]).toEqual({ id: 'd1' });
    expect(deleteMemoryMutateSpy).not.toHaveBeenCalled();
  });

  it('Cancel closes the dialog and clears the target without invoking the mutation', async () => {
    const screen = await renderWS({ kind: 'memory', scope: 'repo:x' });
    await screen.component.requestDelete('m1', 'memory');
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await screen.getByRole('button', { name: 'Cancel' }).click();
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
    expect(deleteMemoryMutateSpy).not.toHaveBeenCalled();
  });
});

describe('WriteSurfaces — requestShare(memory, kind): visibility-aware', () => {
  it('shows the warning for a PRIVATE record and Share anyway invokes set-visibility(SHARED), clearing the target on success', async () => {
    setMemoryVisibilityMutateSpy.mockImplementation((_vars, opts) => {
      opts.onSuccess();
    });
    const screen = await renderWS({ kind: 'memory', scope: 'repo:x' });
    await screen.component.requestShare(fakeMemory({ id: 'm2', visibility: 'private' }), 'memory');
    await expect.element(screen.getByText(/sharing makes this readable/)).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Share anyway' }).click();
    expect(setMemoryVisibilityMutateSpy).toHaveBeenCalledTimes(1);
    expect(setMemoryVisibilityMutateSpy.mock.calls[0][0]).toEqual({ id: 'm2', visibility: 'shared' });
    await expect.element(screen.getByText(/sharing makes this readable/)).not.toBeInTheDocument();
  });

  it('is a no-op for an ALREADY-SHARED record -- no warning shown, no set-visibility call', async () => {
    const screen = await renderWS({ kind: 'memory', scope: 'repo:x' });
    await screen.component.requestShare(fakeMemory({ id: 'm3', visibility: 'shared' }), 'memory');
    await expect.element(screen.getByText(/sharing makes this readable/)).not.toBeInTheDocument();
    expect(setMemoryVisibilityMutateSpy).not.toHaveBeenCalled();
  });

  it('normalizes a stored empty-string visibility as private (not already-shared)', async () => {
    const screen = await renderWS({ kind: 'memory', scope: 'repo:x' });
    await screen.component.requestShare(fakeMemory({ id: 'm4', visibility: '' }), 'memory');
    await expect.element(screen.getByText(/sharing makes this readable/)).toBeInTheDocument();
  });

  it('Cancel closes the banner and clears the target without invoking the mutation', async () => {
    const screen = await renderWS({ kind: 'memory', scope: 'repo:x' });
    await screen.component.requestShare(fakeMemory({ id: 'm2', visibility: 'private' }), 'memory');
    await expect.element(screen.getByText(/sharing makes this readable/)).toBeInTheDocument();
    await screen.getByRole('button', { name: 'Cancel' }).click();
    await expect.element(screen.getByText(/sharing makes this readable/)).not.toBeInTheDocument();
    expect(setMemoryVisibilityMutateSpy).not.toHaveBeenCalled();
  });
});

describe('WriteSurfaces — SC3 inline delete/share terminal-auth retention (Codex round-4/5 HIGH)', () => {
  it.each([Code.Unauthenticated, Code.PermissionDenied])(
    'delete: a terminal %s RETAINS the target, keeps the dialog visibly open, and shows the re-auth CTA (exactly one mutate call)',
    async (code) => {
      deleteMemoryMutateSpy.mockImplementation((_vars, opts) => {
        opts.onError(new ConnectError('session expired', code));
      });
      const screen = await renderWS({ kind: 'memory', scope: 'repo:x' });
      await screen.component.requestDelete('m1', 'memory');
      await screen.getByRole('button', { name: 'Delete' }).click();

      await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
      await expect
        .element(screen.getByText('write failed — session expired. re-authenticate to continue.'))
        .toBeInTheDocument();
      expect(deleteMemoryMutateSpy).toHaveBeenCalledTimes(1);

      await screen.getByRole('button', { name: 'Re-authenticate' }).click();
      expect(redirectToLoginSpy).toHaveBeenCalledTimes(1);
    }
  );

  it.each([Code.Unauthenticated, Code.PermissionDenied])(
    'share: a terminal %s RETAINS the target, keeps the banner visibly open, and shows the re-auth CTA (exactly one mutate call)',
    async (code) => {
      setMemoryVisibilityMutateSpy.mockImplementation((_vars, opts) => {
        opts.onError(new ConnectError('session expired', code));
      });
      const screen = await renderWS({ kind: 'memory', scope: 'repo:x' });
      await screen.component.requestShare(fakeMemory({ id: 'm2', visibility: 'private' }), 'memory');
      await screen.getByRole('button', { name: 'Share anyway' }).click();

      await expect.element(screen.getByText(/sharing makes this readable/)).toBeInTheDocument();
      await expect
        .element(screen.getByText('write failed — session expired. re-authenticate to continue.'))
        .toBeInTheDocument();
      expect(setMemoryVisibilityMutateSpy).toHaveBeenCalledTimes(1);

      await screen.getByRole('button', { name: 'Re-authenticate' }).click();
      expect(redirectToLoginSpy).toHaveBeenCalledTimes(1);
    }
  );
});

describe('WriteSurfaces — discovery kind', () => {
  it('renders "New discovery" + DiscoveryFormSheet on click, and exposes no functional openEdit path (D-04)', async () => {
    const screen = await renderWS({ kind: 'discovery', scope: 'discovery:repo:x' });
    await expect.element(screen.getByRole('button', { name: 'New discovery' })).toBeInTheDocument();
    await screen.getByRole('button', { name: 'New discovery' }).click();
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await expect.element(screen.getByRole('heading', { name: 'New discovery' })).toBeInTheDocument();
    await expect.element(screen.getByRole('textbox', { name: 'scope' })).toHaveValue('discovery:repo:x');

    await screen.component.openEdit('d1');
    expect(getMemorySpy).not.toHaveBeenCalled();
  });
});
