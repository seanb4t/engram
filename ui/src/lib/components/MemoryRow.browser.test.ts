import { render } from 'vitest-browser-svelte';
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
  it('renders the real summary (category prefix stripped) and tags', async () => {
    const screen = await render(MemoryRow, { memory: autoMem, selected: false, onselect: () => {} });
    await expect.element(screen.getByText(/path must match/)).toBeInTheDocument();
    await expect.element(screen.getByText(/GOTCHA \(/)).not.toBeInTheDocument(); // cosmetic strip
    await expect.element(screen.getByText('gotcha')).toBeInTheDocument();
    await expect.element(screen.getByText('mcp')).toBeInTheDocument();
  });

  it('shows the auto provenance glyph only for summary_source=auto', async () => {
    const screen = await render(MemoryRow, { memory: autoMem, selected: false, onselect: () => {} });
    await expect.element(screen.getByLabelText('auto-generated summary')).toBeInTheDocument();
    const clientMem = create(MemorySchema, { id: '7', summary: 'authored line', summarySource: 'client', category: 'decision' });
    await screen.rerender({ memory: clientMem, selected: false, onselect: () => {} });
    await expect.element(screen.getByLabelText('auto-generated summary')).not.toBeInTheDocument();
  });

  it('fires onselect with the memory id when clicked', async () => {
    const onselect = vi.fn();
    const screen = await render(MemoryRow, { memory: autoMem, selected: false, onselect });
    await screen.getByRole('button').click();
    expect(onselect).toHaveBeenCalledWith('42');
  });
});
