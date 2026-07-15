import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import DeleteConfirmDialog from './DeleteConfirmDialog.svelte';

function deferred<T = void>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

describe('DeleteConfirmDialog', () => {
  it('renders the memory-kind copy and a destructive Delete button', async () => {
    const screen = await render(DeleteConfirmDialog, {
      open: true,
      kind: 'memory',
      onconfirm: vi.fn(() => Promise.resolve()),
      oncancel: vi.fn()
    });
    await expect.element(screen.getByText('Delete this memory?')).toBeInTheDocument();
    await expect.element(screen.getByText(/record and its content are removed permanently/)).toBeInTheDocument();
    const del = screen.getByRole('button', { name: 'Delete' });
    await expect.element(del).toBeInTheDocument();
    await expect.element(del).toHaveClass('bg-destructive');
  });

  it('renders the discovery-kind copy', async () => {
    const screen = await render(DeleteConfirmDialog, {
      open: true,
      kind: 'discovery',
      onconfirm: vi.fn(() => Promise.resolve()),
      oncancel: vi.fn()
    });
    await expect.element(screen.getByText('Delete this discovery?')).toBeInTheDocument();
    await expect.element(screen.getByText(/map\/fact and its citations are removed permanently/)).toBeInTheDocument();
  });

  it('invokes the awaitable onconfirm on Delete but does not self-close', async () => {
    const onconfirm = vi.fn(() => Promise.resolve());
    const oncancel = vi.fn();
    const screen = await render(DeleteConfirmDialog, { open: true, kind: 'memory', onconfirm, oncancel });
    await screen.getByRole('button', { name: 'Delete' }).click();
    expect(onconfirm).toHaveBeenCalledTimes(1);
    expect(oncancel).not.toHaveBeenCalled();
    // Host owns `open` — the dialog must stay visibly open after confirm settles.
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await expect.element(screen.getByText('Delete this memory?')).toBeInTheDocument();
  });

  it('disables Delete/Cancel while the awaitable onconfirm is in flight (no double-fire)', async () => {
    const gate = deferred<void>();
    const onconfirm = vi.fn(() => gate.promise);
    const screen = await render(DeleteConfirmDialog, { open: true, kind: 'memory', onconfirm, oncancel: vi.fn() });
    const del = screen.getByRole('button', { name: 'Delete' });
    await del.click();
    await expect.element(del).toBeDisabled();
    await expect.element(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled();
    expect(onconfirm).toHaveBeenCalledTimes(1);
    // Delete is now natively disabled, so it can't be re-clicked to
    // double-fire onconfirm while pending.
    gate.resolve();
    await expect.element(del).not.toBeDisabled();
  });

  it('invokes oncancel and lets the dialog close on Cancel', async () => {
    const oncancel = vi.fn();
    const screen = await render(DeleteConfirmDialog, {
      open: true,
      kind: 'memory',
      onconfirm: vi.fn(() => Promise.resolve()),
      oncancel
    });
    await screen.getByRole('button', { name: 'Cancel' }).click();
    expect(oncancel).toHaveBeenCalledTimes(1);
  });

  it('closes when the host drives open=false (success)', async () => {
    const oncancel = vi.fn();
    const screen = await render(DeleteConfirmDialog, {
      open: true,
      kind: 'memory',
      onconfirm: vi.fn(() => Promise.resolve()),
      oncancel
    });
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
    await screen.rerender({ open: false, kind: 'memory', onconfirm: vi.fn(() => Promise.resolve()), oncancel });
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
    // Host-driven close is not a cancellation.
    expect(oncancel).not.toHaveBeenCalled();
  });

  it('renders the re-auth CTA on authFailure and keeps the dialog open', async () => {
    const onreauth = vi.fn();
    const screen = await render(DeleteConfirmDialog, {
      open: true,
      kind: 'memory',
      onconfirm: vi.fn(() => Promise.resolve()),
      oncancel: vi.fn(),
      authFailure: true,
      onreauth
    });
    await expect.element(screen.getByText('write failed — session expired. re-authenticate to continue.')).toBeInTheDocument();
    const reauth = screen.getByRole('button', { name: 'Re-authenticate' });
    await expect.element(reauth).toBeInTheDocument();
    await reauth.click();
    expect(onreauth).toHaveBeenCalledTimes(1);
    await expect.element(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('omits the re-auth CTA when authFailure is false', async () => {
    const screen = await render(DeleteConfirmDialog, {
      open: true,
      kind: 'memory',
      onconfirm: vi.fn(() => Promise.resolve()),
      oncancel: vi.fn(),
      authFailure: false
    });
    await expect.element(screen.getByText(/re-authenticate to continue/)).not.toBeInTheDocument();
  });
});
