import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import CommandPalette from './CommandPalette.svelte';

describe('CommandPalette', () => {
  it('renders the search input when open', async () => {
    const screen = await render(CommandPalette, { open: true, onsearch: () => {}, onnavigate: () => {} });
    await expect.element(screen.getByPlaceholder(/search memories/i)).toBeInTheDocument();
  });

  it('fires onsearch with the current query when the search item is selected', async () => {
    const onsearch = vi.fn();
    const screen = await render(CommandPalette, { open: true, onsearch, onnavigate: () => {} });
    // Empty input keeps every item visible (cmdk filtering hides items that
    // don't match the typed query); select the search item via its option role.
    await screen.getByRole('option', { name: /search memories for/i }).click();
    expect(onsearch).toHaveBeenCalledWith('');
  });

  it('fires onnavigate with the target href when a Go-to item is selected', async () => {
    const onnavigate = vi.fn();
    const screen = await render(CommandPalette, { open: true, onsearch: () => {}, onnavigate });
    await screen.getByRole('option', { name: 'Search', exact: true }).click();
    expect(onnavigate).toHaveBeenCalledWith(expect.stringContaining('/search'));
  });
});
