import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import { describe, it, expect, vi } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import ResultsList from './ResultsList.svelte';

function makeMemories(count: number): Memory[] {
  return Array.from({ length: count }, (_, i) =>
    create(MemorySchema, {
      id: `m${String(i).padStart(4, '0')}`,
      category: 'convention',
      summary: `summary ${i}`,
      content: '',
      tags: [],
      scope: 'repo:test',
      createdAt: timestampFromDate(new Date()),
      visibility: 'private',
      owner: 'me',
      score: 0.5,
      shortId: `s${String(i).padStart(9, '0')}`
    })
  );
}

const THOUSAND = makeMemories(1000);

describe('ResultsList', () => {
  it('renders a real WAI-ARIA listbox: one focusable option-bearing container, no role=region left', async () => {
    const onopen = vi.fn();
    const screen = await render(ResultsList, { memories: THOUSAND, label: 'Search results', onopen });

    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    await expect.element(listbox).toBeInTheDocument();

    // Not necessarily on first synchronous render (the wrapper may retry via
    // requestAnimationFrame) — expect.element/getByRole locators retry until
    // the assertion holds, closing that gap.
    const regions = screen.container.querySelectorAll('[role="region"]');
    expect(regions.length).toBe(0);

    const options = screen.container.querySelectorAll('[role="option"]');
    expect(options.length).toBeGreaterThan(0);
    expect(options.length).toBeLessThan(1000);
  });

  it('End/Home/j/k traverse aria-activedescendant, clamped at the ends; Enter opens the active row without moving focus', async () => {
    const onopen = vi.fn();
    const screen = await render(ResultsList, { memories: THOUSAND, label: 'Search results', onopen });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });

    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    await userEvent.keyboard('{End}');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0999');
    expect(screen.container.querySelector('#opt-m0999')).toBeTruthy();

    await userEvent.keyboard('{Home}');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    await userEvent.keyboard('j');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0001');

    await userEvent.keyboard('k');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    // k on the first row: no wrap, stays put.
    await userEvent.keyboard('k');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    await userEvent.keyboard('{Enter}');
    expect(onopen).toHaveBeenCalledWith('m0000');
    expect(document.activeElement).toBe(listbox.element());
  });

  it('re-rendering with a different array keeps role listbox and resets the active row to the first item', async () => {
    const onopen = vi.fn();
    const screen = await render(ResultsList, { memories: THOUSAND, label: 'Search results', onopen });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await userEvent.keyboard('{End}');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0999');

    const three = makeMemories(3);
    await screen.rerender({ memories: three, label: 'Search results', onopen });

    await expect.element(screen.getByRole('listbox', { name: 'Search results' })).toBeInTheDocument();
    await expect
      .element(screen.getByRole('listbox', { name: 'Search results' }))
      .toHaveAttribute('aria-activedescendant', 'opt-m0000');
  });

  it('zero memories renders no listbox', async () => {
    const onopen = vi.fn();
    const screen = await render(ResultsList, { memories: [], label: 'Search results', onopen });
    await expect.element(screen.getByRole('listbox', { name: 'Search results' })).not.toBeInTheDocument();
  });
});
