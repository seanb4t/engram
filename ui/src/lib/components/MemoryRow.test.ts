import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import MemoryRow from './MemoryRow.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const mem = create(MemorySchema, {
  id: '42',
  content: 'GOTCHA (path must match) upstream',
  category: 'gotcha',
  tags: ['mcp', 'routing']
});

describe('MemoryRow', () => {
  it('renders the category badge, the de-duplicated summary, and tags', () => {
    render(MemoryRow, { props: { memory: mem, selected: false, onselect: () => {} } });
    expect(screen.getByText('gotcha')).toBeInTheDocument();
    expect(screen.getByText(/path must match/)).toBeInTheDocument();
    expect(screen.queryByText(/GOTCHA \(/)).not.toBeInTheDocument(); // redundant prefix stripped
    expect(screen.getByText('mcp')).toBeInTheDocument();
  });

  it('fires onselect with the memory id when clicked', async () => {
    const user = userEvent.setup();
    const onselect = vi.fn();
    render(MemoryRow, { props: { memory: mem, selected: false, onselect } });
    await user.click(screen.getByRole('button'));
    expect(onselect).toHaveBeenCalledWith('42');
  });
});
