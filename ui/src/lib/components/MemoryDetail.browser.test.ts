import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import MemoryDetail from './MemoryDetail.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const withSummary = create(MemorySchema, {
  id: '1', content: 'full **body** here', summary: 'terse digest line',
  summarySource: 'auto', category: 'gotcha',
  scope: 'repo:github.com/fzymgc-house/selfhosted-cluster',
  source: 'agent-inferred', actor: 'sean', visibility: 'private', tags: ['mcp', 'routing']
});
const noSummary = create(MemorySchema, {
  id: '2', content: 'only content here', summary: '', summarySource: '',
  category: 'decision', scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private'
});
const clientSummary = create(MemorySchema, {
  id: '3', content: 'body', summary: 'human-authored digest',
  summarySource: 'client', category: 'decision', scope: 'repo:y',
  source: 'user-said', actor: 'sean', visibility: 'shared'
});
const unsourcedSummary = create(MemorySchema, {
  id: '4', content: 'body', summary: 'sourceless digest', summarySource: '',
  category: 'gotcha', scope: 'repo:z', source: 'user-said', actor: 'sean', visibility: 'private'
});
const sharedMem = create(MemorySchema, {
  id: '5', content: 'body', summary: 'already shared', category: 'decision', scope: 'repo:x',
  source: 'user-said', actor: 'sean', visibility: 'shared'
});
const ruleMem = create(MemorySchema, {
  id: '6', content: 'body', summary: 'a normative rule', category: 'rule', scope: 'rule:repo:x',
  source: 'user-said', actor: 'sean', visibility: 'private'
});

describe('MemoryDetail', () => {
  it('defaults to the Summary tab and shows summary + auto provenance', async () => {
    const screen = await render(MemoryDetail, { memory: withSummary, loading: false, error: null });
    await expect.element(screen.getByText('terse digest line')).toBeInTheDocument();
    await expect.element(screen.getByText(/auto/i)).toBeInTheDocument();
  });
  it('marks a client-authored summary as authored, not auto', async () => {
    const screen = await render(MemoryDetail, { memory: clientSummary, loading: false, error: null });
    await expect.element(screen.getByText('human-authored digest')).toBeInTheDocument();
    await expect.element(screen.getByText('authored', { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText('✦ auto')).not.toBeInTheDocument();
  });
  it('shows no provenance badge when the summary has no recorded source', async () => {
    const screen = await render(MemoryDetail, { memory: unsourcedSummary, loading: false, error: null });
    await expect.element(screen.getByText('sourceless digest')).toBeInTheDocument();
    await expect.element(screen.getByText('authored')).not.toBeInTheDocument();
    await expect.element(screen.getByText('✦ auto')).not.toBeInTheDocument();
  });
  it('exposes scope, actor, source, visibility, and tags on the Meta tab', async () => {
    const screen = await render(MemoryDetail, { memory: withSummary, loading: false, error: null });
    await screen.getByRole('tab', { name: 'Meta' }).click();
    // Once Meta is active, bits-ui leaves only its panel un-hidden, so the lone
    // accessible tabpanel is the Meta panel — scope the assertions to it.
    const meta = screen.getByRole('tabpanel');
    await expect.element(meta.getByText(withSummary.scope)).toBeInTheDocument();
    await expect.element(meta.getByText('sean')).toBeInTheDocument();
    await expect.element(meta.getByText('agent-inferred')).toBeInTheDocument();
    await expect.element(meta.getByText('private')).toBeInTheDocument();
    await expect.element(meta.getByText('mcp')).toBeInTheDocument();
    await expect.element(meta.getByText('routing')).toBeInTheDocument();
  });
  it('falls through to Content (rendered markdown) when there is no summary', async () => {
    const screen = await render(MemoryDetail, { memory: noSummary, loading: false, error: null });
    await expect.element(screen.getByText('only content here')).toBeInTheDocument();
  });
  it('prompts to select when nothing is chosen', async () => {
    const screen = await render(MemoryDetail, { memory: undefined, loading: false, error: null });
    await expect.element(screen.getByText(/select a record/i)).toBeInTheDocument();
  });
  it('shows a loading indicator while fetching', async () => {
    const screen = await render(MemoryDetail, { memory: undefined, loading: true, error: null });
    await expect.element(screen.getByText(/loading/i)).toBeInTheDocument();
  });
  it('shows a not-found message for a NotFound error', async () => {
    const screen = await render(MemoryDetail, { memory: undefined, loading: false, error: new ConnectError('missing', Code.NotFound) });
    await expect.element(screen.getByText(/record not found/i)).toBeInTheDocument();
  });
  it('shows a generic failure message for a non-NotFound error', async () => {
    const screen = await render(MemoryDetail, { memory: undefined, loading: false, error: new Error('boom') });
    await expect.element(screen.getByText(/failed to load record/i)).toBeInTheDocument();
  });

  it('no actions kebab renders when no write callbacks are supplied', async () => {
    const screen = await render(MemoryDetail, { memory: withSummary, loading: false, error: null });
    await expect.element(screen.getByRole('button', { name: 'record actions' })).not.toBeInTheDocument();
  });

  it('exposes Edit/Delete/Share firing with the memory id / full memory, without disturbing the copy button', async () => {
    const onedit = vi.fn();
    const ondelete = vi.fn();
    const onshare = vi.fn();
    const screen = await render(MemoryDetail, { memory: withSummary, loading: false, error: null, onedit, ondelete, onshare });
    const trigger = screen.getByRole('button', { name: 'record actions' });
    await expect.element(trigger).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: /copy content/i })).toBeInTheDocument();

    await trigger.click();
    await screen.getByRole('menuitem', { name: 'Edit' }).click();
    expect(onedit).toHaveBeenCalledWith(withSummary.id);

    await trigger.click();
    await screen.getByRole('menuitem', { name: 'Delete' }).click();
    expect(ondelete).toHaveBeenCalledWith(withSummary.id);

    await trigger.click();
    await screen.getByRole('menuitem', { name: 'Share' }).click();
    expect(onshare).toHaveBeenCalledWith(withSummary);
  });

  it('suppresses the actions kebab entirely for rule records even with all callbacks supplied', async () => {
    const screen = await render(MemoryDetail, {
      memory: ruleMem,
      loading: false,
      error: null,
      onedit: vi.fn(),
      ondelete: vi.fn(),
      onshare: vi.fn()
    });
    await expect.element(screen.getByRole('button', { name: 'record actions' })).not.toBeInTheDocument();
  });

  it('hides the Share item when the record is already shared (Delete still shows)', async () => {
    const screen = await render(MemoryDetail, {
      memory: sharedMem,
      loading: false,
      error: null,
      ondelete: vi.fn(),
      onshare: vi.fn()
    });
    await screen.getByRole('button', { name: 'record actions' }).click();
    await expect.element(screen.getByRole('menuitem', { name: 'Delete' })).toBeInTheDocument();
    await expect.element(screen.getByRole('menuitem', { name: 'Share' })).not.toBeInTheDocument();
  });

  it('gates each item on its own callback — discovery-route shape (ondelete+onshare, no onedit) never shows Edit', async () => {
    const screen = await render(MemoryDetail, {
      memory: withSummary,
      loading: false,
      error: null,
      ondelete: vi.fn(),
      onshare: vi.fn()
    });
    await screen.getByRole('button', { name: 'record actions' }).click();
    await expect.element(screen.getByRole('menuitem', { name: 'Edit' })).not.toBeInTheDocument();
    await expect.element(screen.getByRole('menuitem', { name: 'Delete' })).toBeInTheDocument();
    await expect.element(screen.getByRole('menuitem', { name: 'Share' })).toBeInTheDocument();
  });
});
