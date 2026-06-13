import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import CommandPalette from './CommandPalette.svelte';

describe('CommandPalette', () => {
  it('renders the search input when open', () => {
    render(CommandPalette, { props: { open: true, onsearch: () => {}, onnavigate: () => {} } });
    expect(screen.getByPlaceholderText(/search memories/i)).toBeInTheDocument();
  });

  it('fires onsearch with the current query when the search item is selected', async () => {
    const user = userEvent.setup();
    const onsearch = vi.fn();
    render(CommandPalette, { props: { open: true, onsearch, onnavigate: () => {} } });
    // Empty input keeps every item visible (cmdk filtering hides items that
    // don't match the typed query); select the search item via its option role.
    await user.click(screen.getByRole('option', { name: /search memories for/i }));
    expect(onsearch).toHaveBeenCalledWith('');
  });

  it('fires onnavigate with the target href when a Go-to item is selected', async () => {
    const user = userEvent.setup();
    const onnavigate = vi.fn();
    render(CommandPalette, { props: { open: true, onsearch: () => {}, onnavigate } });
    await user.click(screen.getByRole('option', { name: 'Search' }));
    expect(onnavigate).toHaveBeenCalledWith(expect.stringContaining('/search'));
  });
});
