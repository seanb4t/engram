import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import CommandPalette from './CommandPalette.svelte';

describe('CommandPalette', () => {
  it('renders the search input when open', () => {
    render(CommandPalette, { props: { open: true, onsearch: () => {}, onnavigate: () => {} } });
    expect(screen.getByPlaceholderText(/search memories/i)).toBeInTheDocument();
  });
});
