import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { describe, it, expect, vi } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { MemorySchema, ArchiveOutcome, type Memory, type ArchiveResult } from '$lib/gen/engram_pb';
import { parseConnectError, type ParsedConnectError } from '$lib/errors/connect-error';
import type { ArchiveSubmitOutcome } from '$lib/mutations/curation';
import ArchiveConfirmDialog from './ArchiveConfirmDialog.svelte';

function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

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

function baseProps(overrides: Record<string, unknown> = {}) {
  return {
    open: true,
    mode: 'archive' as const,
    records: [makeMemory()],
    onsubmit: vi.fn(() => Promise.resolve<ArchiveSubmitOutcome>({ kind: 'ok', results: [] })),
    oncancel: vi.fn(),
    ondone: vi.fn(),
    onundo: vi.fn(),
    onreauth: vi.fn(),
    ...overrides
  };
}

describe('ArchiveConfirmDialog — mode swap', () => {
  it('restore mode swaps header, subline and button style', async () => {
    const screen = await render(ArchiveConfirmDialog, baseProps({ mode: 'restore' }));
    await expect.element(screen.getByText('Restore 1 records?')).toBeInTheDocument();
    await expect.element(screen.getByText(/Clears archived_at/)).toBeInTheDocument();
    const btn = screen.getByRole('button', { name: 'Restore' });
    await expect.element(btn).toBeInTheDocument();
    await expect.element(btn).not.toHaveClass(/destructive/);
  });

  it('archive mode shows the danger-outline Archive button', async () => {
    const screen = await render(ArchiveConfirmDialog, baseProps());
    await expect.element(screen.getByText('Archive 1 records?')).toBeInTheDocument();
    const btn = screen.getByRole('button', { name: 'Archive' });
    await expect.element(btn).toBeInTheDocument();
  });
});

describe('ArchiveConfirmDialog — chip removal (E2 empty)', () => {
  it('× removes a chip and removing the last disables the verb button', async () => {
    const screen = await render(
      ArchiveConfirmDialog,
      baseProps({ records: [makeMemory({ id: 'm1', shortId: 's0000000001' })] })
    );
    await expect.element(screen.getByRole('button', { name: 'Archive' })).not.toBeDisabled();
    await screen.getByRole('button', { name: 'Remove s0000000001' }).click();
    await expect.element(screen.getByRole('button', { name: 'Archive' })).toBeDisabled();
  });
});

describe('ArchiveConfirmDialog — loading (E2 loading)', () => {
  it("shows 'archive_memory · 2 records…' and disables the button while the call is in flight", async () => {
    const gate = deferred<ArchiveSubmitOutcome>();
    const onsubmit = vi.fn(() => gate.promise);
    const screen = await render(
      ArchiveConfirmDialog,
      baseProps({
        records: [makeMemory({ id: 'm1', shortId: 's0000000001' }), makeMemory({ id: 'm2', shortId: 's0000000002' })],
        onsubmit
      })
    );
    await screen.getByRole('button', { name: 'Archive' }).click();
    await expect.element(screen.getByText('archive_memory · 2 records…')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Archive' })).toBeDisabled();
    gate.resolve({ kind: 'ok', results: [] });
  });
});

describe('ArchiveConfirmDialog — result view (E2 partial)', () => {
  function mixedResults(): ArchiveResult[] {
    return [
      { requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED } as ArchiveResult,
      { requested: 'm2', id: 'm2', outcome: ArchiveOutcome.ALREADY_ARCHIVED } as ArchiveResult,
      { requested: 'ghost-id', id: '', outcome: ArchiveOutcome.NOT_FOUND } as ArchiveResult
    ];
  }

  it("shows '✓ 1 archived', the idempotent info line, and the not-found copy naming only the requested input", async () => {
    const onsubmit = vi.fn(() => Promise.resolve<ArchiveSubmitOutcome>({ kind: 'ok', results: mixedResults() }));
    const screen = await render(
      ArchiveConfirmDialog,
      baseProps({
        records: [
          makeMemory({ id: 'm1', shortId: 's0000000001' }),
          makeMemory({ id: 'm2', shortId: 's0000000002' })
        ],
        onsubmit
      })
    );
    await screen.getByRole('button', { name: 'Archive' }).click();

    await expect.element(screen.getByText('✓ 1 archived')).toBeInTheDocument();
    await expect.element(screen.getByText(/s0000000002 already archived — idempotent, no change\./)).toBeInTheDocument();
    await expect.element(screen.getByText('not found: ghost-id')).toBeInTheDocument();
    await expect
      .element(
        screen.getByText('Not found, not owned and ambiguous short id are one rejection by design. Nothing was changed.')
      )
      .toBeInTheDocument();
    // The response `id` field must never render for a not_found row (T-04-02).
    await expect.element(screen.getByText('not found: ghost-id', { exact: false })).not.toHaveTextContent('m2');
  });

  it("'Undo — restore 1' calls onundo(['m1'])", async () => {
    const onsubmit = vi.fn(() =>
      Promise.resolve<ArchiveSubmitOutcome>({
        kind: 'ok',
        results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED } as ArchiveResult]
      })
    );
    const onundo = vi.fn();
    const screen = await render(
      ArchiveConfirmDialog,
      baseProps({ records: [makeMemory({ id: 'm1', shortId: 's0000000001' })], onsubmit, onundo })
    );
    await screen.getByRole('button', { name: 'Archive' }).click();
    await expect.element(screen.getByText('✓ 1 archived')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Undo — restore 1' }).click();
    expect(onundo).toHaveBeenCalledExactlyOnceWith(['m1']);
  });
});

describe('ArchiveConfirmDialog — rejected envelope (E2 error)', () => {
  it('renders field= and hint= pills plus the raw detail', async () => {
    const err = new ConnectError(
      'field=ids hint=out_of_range: too many ids',
      Code.FailedPrecondition
    );
    const parsed: ParsedConnectError = parseConnectError(err);
    const onsubmit = vi.fn(() => Promise.resolve<ArchiveSubmitOutcome>({ kind: 'rejected', parsed }));
    const screen = await render(ArchiveConfirmDialog, baseProps({ onsubmit }));
    await screen.getByRole('button', { name: 'Archive' }).click();

    await expect.element(screen.getByText('Rejected — fix the named field and resend')).toBeInTheDocument();
    await expect.element(screen.getByText('field=ids', { exact: false })).toBeInTheDocument();
    await expect.element(screen.getByText('hint=out_of_range', { exact: false })).toBeInTheDocument();
    await expect.element(screen.getByText(/too many ids/)).toBeInTheDocument();
    // The chips stay in place -- a rejected call never drops a record.
    await expect.element(screen.getByText('s0000000001')).toBeInTheDocument();
  });
});

describe('ArchiveConfirmDialog — re-auth (E2 error)', () => {
  it("renders 'Session expired. Nothing was written; your draft is kept.' with a Re-authenticate button calling onreauth(ids)", async () => {
    const onsubmit = vi.fn(() => Promise.resolve<ArchiveSubmitOutcome>({ kind: 'reauth' }));
    const onreauth = vi.fn();
    const screen = await render(
      ArchiveConfirmDialog,
      baseProps({ records: [makeMemory({ id: 'm1', shortId: 's0000000001' })], onsubmit, onreauth })
    );
    await screen.getByRole('button', { name: 'Archive' }).click();

    await expect.element(screen.getByText('Session expired. Nothing was written; your draft is kept.')).toBeInTheDocument();
    await screen.getByRole('button', { name: 'Re-authenticate' }).click();
    expect(onreauth).toHaveBeenCalledExactlyOnceWith(['m1']);
  });
});

describe('ArchiveConfirmDialog — screenshots (DSYS-04)', () => {
  it('captures the confirm, result and re-auth states', async () => {
    const gate = deferred<ArchiveSubmitOutcome>();
    const onsubmit = vi.fn(() => gate.promise);
    const screen = await render(
      ArchiveConfirmDialog,
      baseProps({ records: [makeMemory({ id: 'm1', shortId: 's0000000001' })], onsubmit })
    );
    await expect.element(screen.getByText('Archive 1 records?')).toBeInTheDocument();
    await page.screenshot();

    await screen.getByRole('button', { name: 'Archive' }).click();
    gate.resolve({
      kind: 'ok',
      results: [{ requested: 'm1', id: 'm1', outcome: ArchiveOutcome.ARCHIVED } as ArchiveResult]
    });
    await expect.element(screen.getByText('✓ 1 archived')).toBeInTheDocument();
    await page.screenshot();

    const onsubmitReauth = vi.fn(() => Promise.resolve<ArchiveSubmitOutcome>({ kind: 'reauth' }));
    const reauthScreen = await render(
      ArchiveConfirmDialog,
      baseProps({ records: [makeMemory({ id: 'm2', shortId: 's0000000002' })], onsubmit: onsubmitReauth })
    );
    await reauthScreen.getByRole('button', { name: 'Archive' }).click();
    await expect.element(reauthScreen.getByText('Session expired. Nothing was written; your draft is kept.')).toBeInTheDocument();
    await page.screenshot();
  });
});
