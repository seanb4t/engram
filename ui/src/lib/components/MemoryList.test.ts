import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import MemoryList from './MemoryList.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const mem = create(MemorySchema, { id: '1', content: 'CI gate set', category: 'convention', visibility: 'shared', tags: ['ci'] });

describe('MemoryList', () => {
  it('shows a loading skeleton', () => {
    render(MemoryList, { props: { memories: [], total: 0n, loading: true, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByTestId('list-loading')).toBeInTheDocument();
  });
  it('shows an empty state', () => {
    render(MemoryList, { props: { memories: [], total: 0n, loading: false, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByText(/no memories/i)).toBeInTheDocument();
  });
  it('renders rows with category + content', () => {
    render(MemoryList, { props: { memories: [mem], total: 1n, loading: false, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByText('convention')).toBeInTheDocument();
    expect(screen.getByText(/CI gate set/)).toBeInTheDocument();
  });
});
