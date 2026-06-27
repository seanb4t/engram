import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import MemoryList from './MemoryList.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';

const mem = create(MemorySchema, { id: '1', summary: 'GOTCHA (path must match) upstream', category: 'gotcha', visibility: 'private', tags: ['mcp', 'routing'] });

type Props = {
  memories: Memory[]; total: bigint; loading: boolean; error: unknown; selectedId: string;
  onselect: (id: string) => void; scopeSelected?: boolean;
};

function baseProps(overrides: Partial<Props> = {}): Props {
  return { memories: [], total: 0n, loading: false, error: null, selectedId: '', onselect: () => {}, ...overrides };
}

describe('MemoryList', () => {
  it('renders the category badge and a de-duplicated summary', () => {
    render(MemoryList, { props: baseProps({ memories: [mem], total: 1n }) });
    expect(screen.getByText('gotcha')).toBeInTheDocument();
    expect(screen.getByText(/path must match/)).toBeInTheDocument();
    expect(screen.queryByText(/GOTCHA \(/)).not.toBeInTheDocument(); // redundant prefix stripped
  });
  it('shows an Empty state when there are no memories', () => {
    render(MemoryList, { props: baseProps() });
    expect(screen.getByText(/no memories/i)).toBeInTheDocument();
  });
  it('prompts to select a scope when none is chosen', () => {
    render(MemoryList, { props: baseProps({ scopeSelected: false }) });
    expect(screen.getByText(/select a scope/i)).toBeInTheDocument();
    expect(screen.queryByText(/no memories/i)).not.toBeInTheDocument();
  });
  it('shows a skeleton when loading', () => {
    render(MemoryList, { props: baseProps({ loading: true }) });
    expect(screen.getByTestId('list-loading')).toBeInTheDocument();
  });
  it('shows an error message when the list fails to load', () => {
    render(MemoryList, { props: baseProps({ error: new Error('boom') }) });
    expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
    expect(screen.queryByText(/no memories/i)).not.toBeInTheDocument();
  });
});
