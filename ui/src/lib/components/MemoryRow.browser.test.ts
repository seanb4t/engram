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

const sharedMem = create(MemorySchema, { id: '9', summary: 'already shared record', category: 'decision', visibility: 'shared' });
const privateEmptyVis = create(MemorySchema, { id: '10', summary: 'legacy no-visibility record', category: 'decision', visibility: '' });
const ruleMem = create(MemorySchema, { id: '11', summary: 'a normative rule', category: 'rule' });

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

  it('root is not a <button> and no kebab renders when no write callbacks are passed', async () => {
    const screen = await render(MemoryRow, { memory: autoMem, selected: false, onselect: () => {} });
    expect(screen.container.firstElementChild?.tagName).not.toBe('BUTTON');
    await expect.element(screen.getByRole('button', { name: 'row actions' })).not.toBeInTheDocument();
  });

  it('renders a hover kebab exposing Edit/Delete/Share that fire without triggering onselect', async () => {
    const onselect = vi.fn();
    const onedit = vi.fn();
    const ondelete = vi.fn();
    const onshare = vi.fn();
    const screen = await render(MemoryRow, { memory: autoMem, selected: false, onselect, onedit, ondelete, onshare });
    const trigger = screen.getByRole('button', { name: 'row actions' });
    await expect.element(trigger).toBeInTheDocument();
    // Sibling structure, never a nested button-in-button.
    expect(trigger.element().closest('button')).toBe(trigger.element());

    await trigger.click();
    await screen.getByRole('menuitem', { name: 'Edit' }).click();
    expect(onedit).toHaveBeenCalledWith('42');

    await trigger.click();
    await screen.getByRole('menuitem', { name: 'Delete' }).click();
    expect(ondelete).toHaveBeenCalledWith('42');

    await trigger.click();
    await screen.getByRole('menuitem', { name: 'Share' }).click();
    expect(onshare).toHaveBeenCalledWith(autoMem);

    expect(onselect).not.toHaveBeenCalled();
  });

  it('suppresses the kebab entirely for rule records even with all callbacks supplied', async () => {
    const screen = await render(MemoryRow, {
      memory: ruleMem,
      selected: false,
      onselect: () => {},
      onedit: vi.fn(),
      ondelete: vi.fn(),
      onshare: vi.fn()
    });
    await expect.element(screen.getByRole('button', { name: 'row actions' })).not.toBeInTheDocument();
  });

  it('hides the Share item when the record is already shared (Delete still shows)', async () => {
    const onshare = vi.fn();
    const ondelete = vi.fn();
    // Also pass ondelete so the kebab still has something to show for an
    // already-shared record (an onshare-only row would have no menu at all).
    const screen = await render(MemoryRow, { memory: sharedMem, selected: false, onselect: () => {}, ondelete, onshare });
    await screen.getByRole('button', { name: 'row actions' }).click();
    await expect.element(screen.getByRole('menuitem', { name: 'Delete' })).toBeInTheDocument();
    await expect.element(screen.getByRole('menuitem', { name: 'Share' })).not.toBeInTheDocument();
  });

  it('shows the Share item when private, including a stored empty-string visibility', async () => {
    const onshare = vi.fn();
    const ondelete = vi.fn();
    const screen = await render(MemoryRow, { memory: privateEmptyVis, selected: false, onselect: () => {}, ondelete, onshare });
    await screen.getByRole('button', { name: 'row actions' }).click();
    await expect.element(screen.getByRole('menuitem', { name: 'Share' })).toBeInTheDocument();
  });

  it('gates each menu item on its own callback — discovery-route shape (ondelete+onshare, no onedit) never shows Edit', async () => {
    const ondelete = vi.fn();
    const onshare = vi.fn();
    const screen = await render(MemoryRow, { memory: autoMem, selected: false, onselect: () => {}, ondelete, onshare });
    await screen.getByRole('button', { name: 'row actions' }).click();
    await expect.element(screen.getByRole('menuitem', { name: 'Edit' })).not.toBeInTheDocument();
    await expect.element(screen.getByRole('menuitem', { name: 'Delete' })).toBeInTheDocument();
    await expect.element(screen.getByRole('menuitem', { name: 'Share' })).toBeInTheDocument();
  });
});
