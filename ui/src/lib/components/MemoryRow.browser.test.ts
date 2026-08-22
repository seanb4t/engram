import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import MemoryRow from './MemoryRow.svelte';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
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

const now = new Date('2030-06-15T12:00:00Z');
const future = timestampFromDate(new Date(now.getTime() + 24 * 60 * 60 * 1000));

const liveMem = create(MemorySchema, { id: '20', summary: 'a live record', category: 'decision' });
const archivedMem = create(MemorySchema, { id: '21', summary: 'an archived record', category: 'decision', archivedAt: timestampFromDate(now) });
const archivedSupersededMem = create(MemorySchema, {
  id: '22',
  summary: 'archived and superseded',
  category: 'decision',
  archivedAt: timestampFromDate(now),
  supersededBy: 'successor-id'
});
const scheduledMem = create(MemorySchema, { id: '23', summary: 'not yet active', category: 'decision', notBefore: future });
const scheduledArchivedMem = create(MemorySchema, {
  id: '24',
  summary: 'archived but also carries a future window',
  category: 'decision',
  archivedAt: timestampFromDate(now),
  notBefore: future
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

  it('a live record renders no state badges and no dim class — unchanged from today', async () => {
    const screen = await render(MemoryRow, { memory: liveMem, selected: false, onselect: () => {} });
    for (const word of ['archived', 'superseded', 'expired', 'scheduled']) {
      await expect.element(screen.getByText(word, { exact: true })).not.toBeInTheDocument();
    }
    await expect.element(screen.getByText('a live record')).not.toHaveClass('opacity-60');
  });

  it('an archived record renders one "archived" badge and dims the summary', async () => {
    const screen = await render(MemoryRow, { memory: archivedMem, selected: false, onselect: () => {} });
    await expect.element(screen.getByText('archived', { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText('superseded', { exact: true })).not.toBeInTheDocument();
    await expect.element(screen.getByText('an archived record')).toHaveClass('opacity-60');
  });

  it('archived + superseded renders both badges in canonical order with the SAME single dim treatment', async () => {
    const screen = await render(MemoryRow, { memory: archivedSupersededMem, selected: false, onselect: () => {} });
    const badges = screen.container.querySelectorAll('[data-slot="badge"]');
    const badgeTexts = Array.from(badges).map((b) => b.textContent?.trim());
    expect(badgeTexts).toEqual(['archived', 'superseded']);
    await expect.element(screen.getByText('archived and superseded')).toHaveClass('opacity-60');
  });

  it('in a dimmed row the badge element does not carry the dim class', async () => {
    const screen = await render(MemoryRow, { memory: archivedMem, selected: false, onselect: () => {} });
    const badge = screen.container.querySelector('[data-slot="badge"]');
    expect(badge?.className).not.toMatch(/opacity-60/);
  });

  it('a scheduled-only record renders its badge and is NOT dimmed', async () => {
    const screen = await render(MemoryRow, { memory: scheduledMem, selected: false, onselect: () => {} });
    await expect.element(screen.getByText('scheduled', { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText('not yet active')).not.toHaveClass('opacity-60');
  });

  it('scheduled + archived renders both badges and IS dimmed (past component drives dimming)', async () => {
    const screen = await render(MemoryRow, { memory: scheduledArchivedMem, selected: false, onselect: () => {} });
    await expect.element(screen.getByText('archived', { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText('scheduled', { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText('archived but also carries a future window')).toHaveClass('opacity-60');
  });

  it('the meta line wraps rather than truncating (flex-wrap present)', async () => {
    const screen = await render(MemoryRow, { memory: archivedSupersededMem, selected: false, onselect: () => {} });
    const metaLine = screen.getByText('archived', { exact: true }).element().closest('div');
    expect(metaLine?.className).toMatch(/flex-wrap/);
  });
});
