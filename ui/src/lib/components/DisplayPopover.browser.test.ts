import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import { describe, it, expect, beforeEach } from 'vitest';
import DisplayPopover from './DisplayPopover.svelte';
import { display, DEFAULT_TEXT_SIZE, TEXT_SIZE_KEY } from '$lib/display.svelte';

beforeEach(() => {
  display.size = DEFAULT_TEXT_SIZE;
  localStorage.clear();
  document.documentElement.style.removeProperty('--ui-font');
});

describe('DisplayPopover', () => {
  it('opens a Display dialog with a Text size radiogroup, current value checked', async () => {
    const screen = await render(DisplayPopover);
    await screen.getByRole('button', { name: 'Display settings' }).click();

    const dialog = screen.getByRole('dialog', { name: 'Display' });
    await expect.element(dialog).toBeInTheDocument();

    const group = screen.getByRole('radiogroup', { name: 'Text size' });
    await expect.element(group).toBeInTheDocument();

    for (const n of [12, 13, 14, 15, 16]) {
      await expect.element(screen.getByRole('radio', { name: `${n}px` })).toBeInTheDocument();
    }
    await expect.element(screen.getByRole('radio', { name: `${DEFAULT_TEXT_SIZE}px` })).toHaveAttribute('aria-checked', 'true');
  });

  it('clicking the 13 radio sets --ui-font, persists, and checks it', async () => {
    const screen = await render(DisplayPopover);
    await screen.getByRole('button', { name: 'Display settings' }).click();

    await screen.getByRole('radio', { name: '13px' }).click();

    expect(document.documentElement.style.getPropertyValue('--ui-font')).toBe('13px');
    expect(localStorage.getItem(TEXT_SIZE_KEY)).toBe('13');
    await expect.element(screen.getByRole('radio', { name: '13px' })).toHaveAttribute('aria-checked', 'true');
  });

  it('ArrowRight on the focused radiogroup moves to the next size and applies it', async () => {
    const screen = await render(DisplayPopover);
    await screen.getByRole('button', { name: 'Display settings' }).click();
    await screen.getByRole('radio', { name: '13px' }).click();

    await userEvent.keyboard('{ArrowRight}');

    await expect.element(screen.getByRole('radio', { name: '14px' })).toHaveAttribute('aria-checked', 'true');
    expect(document.documentElement.style.getPropertyValue('--ui-font')).toBe('14px');
    expect(localStorage.getItem(TEXT_SIZE_KEY)).toBe('14');
  });

  it('shows the "applies everywhere" copy and the shortcut hints', async () => {
    const screen = await render(DisplayPopover);
    await screen.getByRole('button', { name: 'Display settings' }).click();

    await expect.element(screen.getByText('Applies to every console page, not just this list.')).toBeInTheDocument();
    await expect.element(screen.getByText('larger')).toBeInTheDocument();
    await expect.element(screen.getByText('smaller')).toBeInTheDocument();
    await expect.element(screen.getByText(`reset to ${DEFAULT_TEXT_SIZE}`)).toBeInTheDocument();
  });

  it('keeps the radio control at a fixed pixel height at size 12 and 16', async () => {
    const screen = await render(DisplayPopover);
    await screen.getByRole('button', { name: 'Display settings' }).click();

    await screen.getByRole('radio', { name: '12px' }).click();
    const heightAt12 = screen.getByRole('radio', { name: '12px' }).element().getBoundingClientRect().height;

    await screen.getByRole('radio', { name: '16px' }).click();
    const heightAt16 = screen.getByRole('radio', { name: '16px' }).element().getBoundingClientRect().height;

    expect(heightAt16).toBe(heightAt12);
  });
});
