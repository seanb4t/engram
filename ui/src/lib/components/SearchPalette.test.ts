import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import SearchPalette from './SearchPalette.svelte';

describe('SearchPalette', () => {
  it('calls onsubmit with the typed query on Enter', async () => {
    const onsubmit = vi.fn();
    render(SearchPalette, { props: { value: '', onsubmit } });
    const input = screen.getByRole('searchbox') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: 'ci gate' } }); // drives bind:value
    await fireEvent.keyDown(input, { key: 'Enter' });
    expect(onsubmit).toHaveBeenCalledWith('ci gate');
  });
});
