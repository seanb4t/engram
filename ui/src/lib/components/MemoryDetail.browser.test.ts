import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import MemoryDetail from './MemoryDetail.svelte';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { MemorySchema } from '$lib/gen/engram_pb';
import { fullTimestamp } from '$lib/time';

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

// Relative to REAL wall-clock time: MemoryDetail's memoryStateWords call uses
// the default now = new Date() (no injectable now on this surface), so
// expired/scheduled derivation must be evaluated against actual current time.
const now = new Date();
const past = new Date(now.getTime() - 24 * 60 * 60 * 1000);
const future = new Date(now.getTime() + 24 * 60 * 60 * 1000);
const laterFuture = new Date(now.getTime() + 48 * 60 * 60 * 1000);
const SUCCESSOR_ID = 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee';
const PRED_ID_1 = '11111111-2222-3333-4444-555555555555';
const PRED_ID_2 = '66666666-7777-8888-9999-000000000000';

const liveWithSchema = create(MemorySchema, {
  id: '7', shortId: 'LIVE000001', content: 'body', summary: 'a live record', category: 'decision',
  scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private', schemaVersion: 3
});
const archivedMem = create(MemorySchema, {
  id: '8', shortId: 'ARCH000001', content: 'body', summary: 'an archived record', category: 'decision',
  scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private',
  archivedAt: timestampFromDate(now)
});
const supersededMem = create(MemorySchema, {
  id: '9', shortId: 'SUPD000001', content: 'body', summary: 'a superseded record', category: 'decision',
  scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private',
  supersededBy: SUCCESSOR_ID
});
const supersedesMem = create(MemorySchema, {
  id: '10', shortId: 'SUCC000001', content: 'body', summary: 'a successor record', category: 'decision',
  scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private',
  supersedes: [PRED_ID_1, PRED_ID_2]
});
const expiredMem = create(MemorySchema, {
  id: '11', shortId: 'EXPD000001', content: 'body', summary: 'an expired record', category: 'decision',
  scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private',
  notAfter: timestampFromDate(past)
});
const scheduledMem = create(MemorySchema, {
  id: '12', shortId: 'SCHD000001', content: 'body', summary: 'a scheduled record', category: 'decision',
  scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private',
  notBefore: timestampFromDate(future)
});
const scheduledWithCloseMem = create(MemorySchema, {
  id: '13', shortId: 'SCHD000002', content: 'body', summary: 'a scheduled record with a close', category: 'decision',
  scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private',
  notBefore: timestampFromDate(future), notAfter: timestampFromDate(laterFuture)
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

  it('a live record shows the schema chip and no State section', async () => {
    const screen = await render(MemoryDetail, { memory: liveWithSchema, loading: false, error: null });
    await screen.getByRole('tab', { name: 'Meta' }).click();
    const meta = screen.getByRole('tabpanel');
    await expect.element(meta.getByText('schema')).toBeInTheDocument();
    // The chip's value shares a text run with the "schema" label sibling
    // span, so it is not its own element — assert via substring, matching
    // how the existing by/src/vis chip tests assert their values.
    await expect.element(meta.getByText('v3')).toBeInTheDocument();
    await expect.element(meta.getByText('State', { exact: true })).not.toBeInTheDocument();
  });

  it('an archived record renders a State section naming the archival moment', async () => {
    const screen = await render(MemoryDetail, { memory: archivedMem, loading: false, error: null });
    await screen.getByRole('tab', { name: 'Meta' }).click();
    const meta = screen.getByRole('tabpanel');
    await expect.element(meta.getByText(new RegExp(`archived ${fullTimestamp(now)}`))).toBeInTheDocument();
  });

  it('a superseded record renders a full-UUID link whose click target and visible text are identical', async () => {
    const onselect = vi.fn();
    const screen = await render(MemoryDetail, { memory: supersededMem, loading: false, error: null, onselect });
    await screen.getByRole('tab', { name: 'Meta' }).click();
    const meta = screen.getByRole('tabpanel');
    const link = meta.getByText(SUCCESSOR_ID, { exact: true });
    await expect.element(link).toBeInTheDocument();
    expect(link.element().textContent?.trim()).toBe(SUCCESSOR_ID);
    await link.click();
    expect(onselect).toHaveBeenCalledWith(SUCCESSOR_ID);
    // Never sourced from the loaded record's own short_id.
    expect(screen.container.textContent).not.toContain(supersededMem.shortId);
  });

  it('a record with two supersedes entries renders two predecessor links', async () => {
    const screen = await render(MemoryDetail, { memory: supersedesMem, loading: false, error: null, onselect: () => {} });
    await screen.getByRole('tab', { name: 'Meta' }).click();
    const meta = screen.getByRole('tabpanel');
    await expect.element(meta.getByText(PRED_ID_1, { exact: true })).toBeInTheDocument();
    await expect.element(meta.getByText(PRED_ID_2, { exact: true })).toBeInTheDocument();
  });

  it('renders without an onselect prop for a superseded record without throwing', async () => {
    const screen = await render(MemoryDetail, { memory: supersededMem, loading: false, error: null });
    await screen.getByRole('tab', { name: 'Meta' }).click();
    const meta = screen.getByRole('tabpanel');
    await expect.element(meta.getByText(SUCCESSOR_ID, { exact: true })).toBeInTheDocument();
  });

  it('an expired record renders a State section naming the expiry moment', async () => {
    const screen = await render(MemoryDetail, { memory: expiredMem, loading: false, error: null });
    await screen.getByRole('tab', { name: 'Meta' }).click();
    const meta = screen.getByRole('tabpanel');
    await expect.element(meta.getByText(new RegExp(`expired ${fullTimestamp(past)}`))).toBeInTheDocument();
  });

  it('a scheduled record renders a not-yet-active line naming the open moment', async () => {
    const screen = await render(MemoryDetail, { memory: scheduledMem, loading: false, error: null });
    await screen.getByRole('tab', { name: 'Meta' }).click();
    const meta = screen.getByRole('tabpanel');
    await expect.element(meta.getByText(new RegExp(`opens ${fullTimestamp(future)}`))).toBeInTheDocument();
  });

  it('a scheduled record with a future not_after also renders a closing line', async () => {
    const screen = await render(MemoryDetail, { memory: scheduledWithCloseMem, loading: false, error: null });
    await screen.getByRole('tab', { name: 'Meta' }).click();
    const meta = screen.getByRole('tabpanel');
    await expect.element(meta.getByText(new RegExp(`opens ${fullTimestamp(future)}`))).toBeInTheDocument();
    await expect.element(meta.getByText(new RegExp(`closes ${fullTimestamp(laterFuture)}`))).toBeInTheDocument();
  });
});
