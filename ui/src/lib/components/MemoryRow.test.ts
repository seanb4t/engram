import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import MemoryRow from './MemoryRow.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const autoMem = create(MemorySchema, {
  id: '42',
  summary: 'GOTCHA (path must match) upstream routing rule',
  summarySource: 'auto',
  category: 'gotcha',
  tags: ['mcp', 'routing']
});

describe('MemoryRow', () => {
  it('renders the real summary (category prefix stripped) and tags', () => {
    render(MemoryRow, { props: { memory: autoMem, selected: false, onselect: () => {} } });
    expect(screen.getByText(/path must match/)).toBeInTheDocument();
    expect(screen.queryByText(/GOTCHA \(/)).not.toBeInTheDocument(); // cosmetic strip
    expect(screen.getByText('gotcha')).toBeInTheDocument();
    expect(screen.getByText('mcp')).toBeInTheDocument();
  });

  it('shows the auto provenance glyph only for summary_source=auto', () => {
    const { rerender } = render(MemoryRow, { props: { memory: autoMem, selected: false, onselect: () => {} } });
    expect(screen.getByLabelText('auto-generated summary')).toBeInTheDocument();
    const clientMem = create(MemorySchema, { id: '7', summary: 'authored line', summarySource: 'client', category: 'decision' });
    rerender({ memory: clientMem, selected: false, onselect: () => {} });
    expect(screen.queryByLabelText('auto-generated summary')).not.toBeInTheDocument();
  });

  it('fires onselect with the memory id when clicked', async () => {
    const user = userEvent.setup();
    const onselect = vi.fn();
    render(MemoryRow, { props: { memory: autoMem, selected: false, onselect } });
    await user.click(screen.getByRole('button'));
    expect(onselect).toHaveBeenCalledWith('42');
  });
});
