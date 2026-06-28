import { render } from 'vitest-browser-svelte';
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
  it('renders the category badge and a de-duplicated summary', async () => {
    const screen = await render(MemoryList, baseProps({ memories: [mem], total: 1n }));
    await expect.element(screen.getByText('gotcha')).toBeInTheDocument();
    await expect.element(screen.getByText(/path must match/)).toBeInTheDocument();
    await expect.element(screen.getByText(/GOTCHA \(/)).not.toBeInTheDocument(); // redundant prefix stripped
  });
  it('shows an Empty state when there are no memories', async () => {
    const screen = await render(MemoryList, baseProps());
    await expect.element(screen.getByText(/no memories/i)).toBeInTheDocument();
  });
  it('prompts to select a scope when none is chosen', async () => {
    const screen = await render(MemoryList, baseProps({ scopeSelected: false }));
    await expect.element(screen.getByText(/select a scope/i)).toBeInTheDocument();
    await expect.element(screen.getByText(/no memories/i)).not.toBeInTheDocument();
  });
  it('shows a skeleton when loading', async () => {
    const screen = await render(MemoryList, baseProps({ loading: true }));
    await expect.element(screen.getByTestId('list-loading')).toBeInTheDocument();
  });
  it('shows an error message when the list fails to load', async () => {
    const screen = await render(MemoryList, baseProps({ error: new Error('boom') }));
    await expect.element(screen.getByText(/failed to load/i)).toBeInTheDocument();
    await expect.element(screen.getByText(/no memories/i)).not.toBeInTheDocument();
  });
});
