import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import SearchPalette from './SearchPalette.svelte';

describe('SearchPalette', () => {
  it('calls onsubmit with the typed query on Enter', async () => {
    const user = userEvent.setup();
    const onsubmit = vi.fn();
    render(SearchPalette, { props: { value: '', onsubmit } });
    const input = screen.getByRole('searchbox') as HTMLInputElement;
    // userEvent.type dispatches real keystrokes so Svelte 5 bind:value updates
    // the underlying $state (fireEvent.input alone does not), then Enter submits.
    await user.type(input, 'ci gate');
    expect(input.value).toBe('ci gate');
    await user.type(input, '{Enter}');
    expect(onsubmit).toHaveBeenCalledWith('ci gate');
  });
});
