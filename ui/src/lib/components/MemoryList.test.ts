import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import MemoryList from './MemoryList.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const mem = create(MemorySchema, { id: '1', content: 'GOTCHA (path must match) upstream', category: 'gotcha', visibility: 'private', tags: ['mcp', 'routing'] });

describe('MemoryList', () => {
  it('renders the category badge and a de-duplicated summary', () => {
    render(MemoryList, { props: { memories: [mem], total: 1n, loading: false, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByText('gotcha')).toBeInTheDocument();
    expect(screen.getByText(/path must match/)).toBeInTheDocument();
    expect(screen.queryByText(/GOTCHA \(/)).not.toBeInTheDocument(); // redundant prefix stripped
  });
  it('shows an Empty state when there are no memories', () => {
    render(MemoryList, { props: { memories: [], total: 0n, loading: false, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByText(/no memories/i)).toBeInTheDocument();
  });
  it('shows a skeleton when loading', () => {
    render(MemoryList, { props: { memories: [], total: 0n, loading: true, error: null, selectedId: '', onselect: () => {} } });
    expect(screen.getByTestId('list-loading')).toBeInTheDocument();
  });
});
