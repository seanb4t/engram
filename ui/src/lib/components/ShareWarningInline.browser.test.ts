import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import ShareWarningInline from './ShareWarningInline.svelte';

function deferred<T = void>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

describe('ShareWarningInline', () => {
  it('renders the accurate disclosure copy and the button pair as an alert', async () => {
    const screen = await render(ShareWarningInline, { onconfirm: vi.fn(() => Promise.resolve()), oncancel: vi.fn() });
    await expect.element(screen.getByRole('alert')).toBeInTheDocument();
    await expect
      .element(screen.getByText("sharing makes this readable by every authenticated caller. you can stop sharing later, but you can't retract what's already been read."))
      .toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Share anyway' })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
  });

  it('invokes the awaitable onconfirm on Share anyway', async () => {
    const onconfirm = vi.fn(() => Promise.resolve());
    const screen = await render(ShareWarningInline, { onconfirm, oncancel: vi.fn() });
    await screen.getByRole('button', { name: 'Share anyway' }).click();
    expect(onconfirm).toHaveBeenCalledTimes(1);
  });

  it('invokes oncancel on Cancel', async () => {
    const oncancel = vi.fn();
    const screen = await render(ShareWarningInline, { onconfirm: vi.fn(() => Promise.resolve()), oncancel });
    await screen.getByRole('button', { name: 'Cancel' }).click();
    expect(oncancel).toHaveBeenCalledTimes(1);
  });

  it('disables Share anyway/Cancel while the awaitable onconfirm is in flight', async () => {
    const gate = deferred<void>();
    const onconfirm = vi.fn(() => gate.promise);
    const screen = await render(ShareWarningInline, { onconfirm, oncancel: vi.fn() });
    const shareBtn = screen.getByRole('button', { name: 'Share anyway' });
    await shareBtn.click();
    await expect.element(shareBtn).toBeDisabled();
    await expect.element(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled();
    expect(onconfirm).toHaveBeenCalledTimes(1);
    gate.resolve();
    await expect.element(shareBtn).not.toBeDisabled();
  });

  it('renders the re-auth CTA on authFailure', async () => {
    const onreauth = vi.fn();
    const screen = await render(ShareWarningInline, {
      onconfirm: vi.fn(() => Promise.resolve()),
      oncancel: vi.fn(),
      authFailure: true,
      onreauth
    });
    await expect.element(screen.getByText('write failed — session expired. re-authenticate to continue.')).toBeInTheDocument();
    const reauth = screen.getByRole('button', { name: 'Re-authenticate' });
    await reauth.click();
    expect(onreauth).toHaveBeenCalledTimes(1);
  });

  it('omits the re-auth CTA when authFailure is false', async () => {
    const screen = await render(ShareWarningInline, { onconfirm: vi.fn(() => Promise.resolve()), oncancel: vi.fn(), authFailure: false });
    await expect.element(screen.getByText(/re-authenticate to continue/)).not.toBeInTheDocument();
  });
});
