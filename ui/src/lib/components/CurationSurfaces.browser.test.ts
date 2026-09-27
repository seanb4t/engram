import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { create } from '@bufbuild/protobuf';
import { MemorySchema, ArchiveOutcome, type Memory } from '$lib/gen/engram_pb';
import { flashing, FLASH_MS } from '$lib/curation/flash.svelte.ts';
import CurationSurfaces from './CurationSurfaces.svelte';

const { archiveMemorySpy, restoreMemorySpy, getMemorySpy, toastSpy, redirectToLoginSpy } = vi.hoisted(() => ({
  archiveMemorySpy: vi.fn(),
  restoreMemorySpy: vi.fn(),
  getMemorySpy: vi.fn(),
  toastSpy: vi.fn(),
  redirectToLoginSpy: vi.fn()
}));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: { ...actual.engram, getMemory: getMemorySpy },
    engramWrite: { ...actual.engramWrite, archiveMemory: archiveMemorySpy, restoreMemory: restoreMemorySpy }
  };
});

vi.mock('svelte-sonner', () => ({ toast: toastSpy }));

vi.mock('$lib/resume', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/resume')>();
  return { ...actual, redirectToLogin: redirectToLoginSpy };
});

function makeMemory(overrides: Partial<Memory> = {}): Memory {
  return create(MemorySchema, {
    id: 'm1',
    category: 'convention',
    summary: 'archive me',
    content: '',
    tags: [],
    scope: 'repo:test',
    visibility: 'private',
    owner: 'me',
    shortId: 's0000000001',
    ...overrides
  });
}

let qc: QueryClient;
function renderCS(props: { returnPath: string; onchanged?: (e: { kind: string; ids: string[] }) => void }) {
  return render(CurationSurfaces, props, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

beforeEach(() => {
  archiveMemorySpy.mockReset();
  restoreMemorySpy.mockReset();
  getMemorySpy.mockReset();
  toastSpy.mockReset();
  redirectToLoginSpy.mockReset();
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  qc.setQueryData(['searchMemories', 'q', ''], { memories: [makeMemory({ id: 'm1' })] });
});

afterEach(() => {
  vi.useRealTimers();
});

describe('CurationSurfaces — archive toast undo (D-09 surface 2)', () => {
  it("Done after an archive result toasts '1 archived · Undo' (duration 8000), and the toast's Undo runs RestoreMemory with the archived ids", async () => {
    archiveMemorySpy.mockResolvedValue({
      results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED }]
    });
    restoreMemorySpy.mockResolvedValue({
      results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.RESTORED }]
    });

    const screen = await renderCS({ returnPath: '/search' });
    await screen.component.openArchive(['m1']);
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Archive' }).click();
    await expect.element(screen.getByText('✓ 1 archived')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Done' }).click();
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();

    expect(toastSpy).toHaveBeenCalledExactlyOnceWith(
      '1 archived · Undo',
      expect.objectContaining({ duration: 8000, action: expect.objectContaining({ label: 'Undo' }) })
    );

    const [, opts] = toastSpy.mock.calls[0] as [string, { action: { onClick: () => void } }];
    opts.action.onClick();
    await expect.poll(() => restoreMemorySpy.mock.calls.length).toBe(1);
    expect(restoreMemorySpy.mock.calls[0][0].ids).toEqual(['m1']);
  });
});

describe('CurationSurfaces — result-body undo (D-09 surface 1)', () => {
  it("'Undo — restore 1' runs RestoreMemory through the same dialog and shows '✓ 1 restored'", async () => {
    archiveMemorySpy.mockResolvedValue({
      results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED }]
    });
    restoreMemorySpy.mockResolvedValue({
      results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.RESTORED }]
    });

    const screen = await renderCS({ returnPath: '/search' });
    await screen.component.openArchive(['m1']);
    await screen.getByRole('button', { name: 'Archive' }).click();
    await expect.element(screen.getByText('✓ 1 archived')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Undo — restore 1' }).click();

    await expect.poll(() => restoreMemorySpy.mock.calls.length).toBe(1);
    expect(restoreMemorySpy.mock.calls[0][0].ids).toEqual(['m1']);
    await expect.element(screen.getByText('✓ 1 restored')).toBeInTheDocument();
    // Still the same dialog -- no second confirm click was required.
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
  });
});

describe('CurationSurfaces — flash + onchanged (D-10)', () => {
  it('adds the changed ids to the flashing set immediately on success, removes them after FLASH_MS, and fires onchanged', async () => {
    vi.useFakeTimers();
    archiveMemorySpy.mockResolvedValue({
      results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED }]
    });
    const onchanged = vi.fn();

    const screen = await renderCS({ returnPath: '/search', onchanged });
    await screen.component.openArchive(['m1']);
    await screen.getByRole('button', { name: 'Archive' }).click();
    await expect.element(screen.getByText('✓ 1 archived')).toBeInTheDocument();

    expect(flashing.has('m1')).toBe(true);
    expect(onchanged).toHaveBeenCalledExactlyOnceWith({ kind: 'archive', ids: ['m1'] });

    await vi.advanceTimersByTimeAsync(FLASH_MS);
    expect(flashing.has('m1')).toBe(false);
  });
});

describe('CurationSurfaces — re-auth redirect', () => {
  it('a PermissionDenied/Unauthenticated mutation failure redirects via redirectToLogin on Re-authenticate', async () => {
    const { ConnectError, Code } = await import('@connectrpc/connect');
    archiveMemorySpy.mockRejectedValue(new ConnectError('session expired', Code.Unauthenticated));

    const screen = await renderCS({ returnPath: '/search' });
    await screen.component.openArchive(['m1']);
    await screen.getByRole('button', { name: 'Archive' }).click();
    await expect.element(screen.getByText('Session expired. Nothing was written; your draft is kept.')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Re-authenticate' }).click();
    expect(redirectToLoginSpy).toHaveBeenCalledTimes(1);
  });
});
