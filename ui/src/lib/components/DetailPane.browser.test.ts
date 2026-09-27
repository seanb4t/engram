import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { MemorySchema, CitationSchema } from '$lib/gen/engram_pb';
import DetailPane from './DetailPane.svelte';

const NOW = new Date();
const PAST = new Date(NOW.getTime() - 24 * 60 * 60 * 1000);
const FUTURE = new Date(NOW.getTime() + 24 * 60 * 60 * 1000);

const SUCCESSOR_ID = 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee';
const PRED_ID_1 = '11111111-2222-3333-4444-555555555555';
const PRED_ID_2 = '66666666-7777-8888-9999-000000000000';

// D-14/ROW-07: archived AND superseded, supersedes two predecessors, a
// currently-active window (not_before past, not_after future -- yields no
// expired/scheduled word), two citations, tags, usage counters, schema v3.
const fullRecord = create(MemorySchema, {
  id: 'full-record-uuid-0001',
  shortId: 'FULLREC001',
  content: 'full **body** content here',
  summary: 'a full ROW-07 record',
  category: 'gotcha',
  scope: 'repo:github.com/fzymgc-house/selfhosted-cluster',
  source: 'agent-inferred',
  actor: 'sean',
  owner: 'sean@example.com',
  visibility: 'private',
  summarySource: 'auto',
  tags: ['mcp', 'routing'],
  archivedAt: timestampFromDate(NOW),
  supersededBy: SUCCESSOR_ID,
  supersedes: [PRED_ID_1, PRED_ID_2],
  notBefore: timestampFromDate(PAST),
  notAfter: timestampFromDate(FUTURE),
  citations: [
    create(CitationSchema, { kind: 'file', ref: 'internal/store/store.go', locator: 'L120', pin: 'abc123def' }),
    create(CitationSchema, { kind: 'url', ref: 'https://example.com/doc' })
  ],
  accessCount: 7n,
  lastAccessedAt: timestampFromDate(NOW),
  schemaVersion: 3
});

// A record with no tags, no citations, no state -- ROW-07 empty case.
const emptyRecord = create(MemorySchema, {
  id: 'empty-record-uuid-0001',
  shortId: 'EMPTYREC01',
  content: 'plain content, nothing special',
  category: 'decision',
  scope: 'repo:x',
  source: 'user-said',
  actor: 'sean',
  owner: 'sean',
  visibility: 'private',
  summarySource: ''
});

describe('DetailPane', () => {
  it('renders every ROW-07 field for a full record, sections in D-14 fixed order', async () => {
    const onselect = vi.fn();
    const ontag = vi.fn();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);

    const screen = await render(DetailPane, {
      memory: fullRecord,
      loading: false,
      error: null,
      requestedId: fullRecord.id,
      hit: { score: 0.81234, relevance: 0.5 },
      onselect,
      ontag
    });

    // D-14 fixed order: State, Content, Tags, Metadata (h3 headings) after the title.
    const headingTexts = screen.container.querySelectorAll('h3');
    expect(Array.from(headingTexts).map((h) => h.textContent)).toEqual(['State', 'Content', 'Tags', 'Metadata']);

    await expect.element(screen.getByText(/archived since/)).toBeInTheDocument();

    const successorLink = screen.getByText(SUCCESSOR_ID, { exact: true });
    await expect.element(successorLink).toBeInTheDocument();
    await successorLink.click();
    expect(onselect).toHaveBeenCalledWith(SUCCESSOR_ID);

    const pred1 = screen.getByText(PRED_ID_1, { exact: true });
    await pred1.click();
    expect(onselect).toHaveBeenCalledWith(PRED_ID_1);
    const pred2 = screen.getByText(PRED_ID_2, { exact: true });
    await pred2.click();
    expect(onselect).toHaveBeenCalledWith(PRED_ID_2);

    // Metadata: both id forms, scope, created_at + age, owner, actor,
    // visibility, summary_source, source, schema, access_count,
    // last_accessed_at, and both citations' kind + ref. Scoped to the
    // Metadata region -- short_id/id also render in the sticky head.
    const meta = screen.getByRole('region', { name: 'Metadata' });
    await expect.element(meta.getByText(fullRecord.id, { exact: true })).toBeInTheDocument();
    await expect.element(meta.getByText(fullRecord.shortId, { exact: true })).toBeInTheDocument();
    await expect.element(meta.getByText(fullRecord.scope, { exact: true })).toBeInTheDocument();
    await expect.element(meta.getByText('sean@example.com')).toBeInTheDocument();
    await expect.element(meta.getByText('agent-inferred')).toBeInTheDocument();
    await expect.element(meta.getByText('schema v3')).toBeInTheDocument();
    await expect.element(meta.getByText('7', { exact: true })).toBeInTheDocument();
    await expect.element(meta.getByText('internal/store/store.go')).toBeInTheDocument();
    await expect.element(meta.getByText('https://example.com/doc')).toBeInTheDocument();

    // hit-derived score/relevance to 4 decimals.
    await expect.element(meta.getByText('0.8123')).toBeInTheDocument();
    await expect.element(meta.getByText('0.5000')).toBeInTheDocument();

    // Copy id / Copy short_id.
    await meta.getByRole('button', { name: 'Copy id' }).click();
    expect(writeText).toHaveBeenCalledWith(fullRecord.id);
    await meta.getByRole('button', { name: 'Copy short_id' }).click();
    expect(writeText).toHaveBeenCalledWith(fullRecord.shortId);

    // Tag click.
    await screen.getByRole('button', { name: 'mcp' }).click();
    expect(ontag).toHaveBeenCalledWith('mcp');
  });

  it('a record with no tags, citations or state shows no State/Tags heading but still shows Metadata with both ids', async () => {
    const screen = await render(DetailPane, {
      memory: emptyRecord,
      loading: false,
      error: null,
      requestedId: emptyRecord.id
    });

    await expect.element(screen.getByRole('heading', { name: 'State' })).not.toBeInTheDocument();
    await expect.element(screen.getByRole('heading', { name: 'Tags' })).not.toBeInTheDocument();
    await expect.element(screen.getByRole('heading', { name: 'Metadata' })).toBeInTheDocument();
    const meta = screen.getByRole('region', { name: 'Metadata' });
    await expect.element(meta.getByText(emptyRecord.id, { exact: true })).toBeInTheDocument();
    await expect.element(meta.getByText(emptyRecord.shortId, { exact: true })).toBeInTheDocument();
  });

  it('loading shows the requested id in the head and a skeleton', async () => {
    const screen = await render(DetailPane, {
      memory: undefined,
      loading: true,
      error: null,
      requestedId: 'k3m9p2qr7a'
    });
    await expect.element(screen.getByText('k3m9p2qr7a')).toBeInTheDocument();
    expect(screen.container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
  });

  it('a NotFound error shows the E7 not-found copy with the requested id', async () => {
    const screen = await render(DetailPane, {
      memory: undefined,
      loading: false,
      error: new ConnectError('missing', Code.NotFound),
      requestedId: 'k3m9p2qr7a'
    });
    await expect
      .element(
        screen.getByText('No memory with id k3m9p2qr7a that you can read · not-found and not-yours look the same by design')
      )
      .toBeInTheDocument();
  });

  it('shows a generic failure message with the error code name for a non-NotFound ConnectError', async () => {
    const screen = await render(DetailPane, {
      memory: undefined,
      loading: false,
      error: new ConnectError('boom', Code.Unavailable),
      requestedId: 'k3m9p2qr7a'
    });
    await expect.element(screen.getByText(/Could not load this record/)).toBeInTheDocument();
  });

  it('shows the not-in-results note when inResults is false', async () => {
    const screen = await render(DetailPane, {
      memory: emptyRecord,
      loading: false,
      error: null,
      requestedId: emptyRecord.id,
      inResults: false
    });
    await expect
      .element(screen.getByText('Not in the current results (filtered out or not matched). Showing it by id, like get_memory.'))
      .toBeInTheDocument();
  });

  it('the close button calls onclose', async () => {
    const onclose = vi.fn();
    const screen = await render(DetailPane, {
      memory: emptyRecord,
      loading: false,
      error: null,
      requestedId: emptyRecord.id,
      onclose
    });
    await screen.getByRole('button', { name: 'close' }).click();
    expect(onclose).toHaveBeenCalledTimes(1);
  });
});

describe('DetailPane — inline actions (D-15, D-16)', () => {
  const privateGotcha = create(MemorySchema, {
    id: 'private-gotcha-0001',
    shortId: 'PRIVGOT001',
    content: 'a private gotcha',
    category: 'gotcha',
    scope: 'repo:x',
    source: 'user-said',
    actor: 'sean',
    owner: 'sean',
    visibility: 'private'
  });
  const archivedDecision = create(MemorySchema, {
    id: 'archived-decision-0001',
    shortId: 'ARCHDEC001',
    content: 'an archived decision',
    category: 'decision',
    scope: 'repo:x',
    source: 'user-said',
    actor: 'sean',
    owner: 'sean',
    visibility: 'private',
    archivedAt: timestampFromDate(NOW)
  });
  const sharedDecision = create(MemorySchema, {
    id: 'shared-decision-0001',
    shortId: 'SHRDEC0001',
    content: 'a shared decision',
    category: 'decision',
    scope: 'repo:x',
    source: 'user-said',
    actor: 'sean',
    owner: 'sean',
    visibility: 'shared'
  });
  const ruleRecord = create(MemorySchema, {
    id: 'rule-record-0001',
    shortId: 'RULEREC001',
    content: 'a normative rule',
    category: 'rule',
    scope: 'rule:repo:x',
    source: 'user-said',
    actor: 'sean',
    owner: 'sean',
    visibility: 'private'
  });
  const discoveryRecord = create(MemorySchema, {
    id: 'discovery-record-0001',
    shortId: 'DISCREC001',
    content: 'a discovery map',
    category: 'gotcha',
    kind: 'map',
    scope: 'discovery:repo:x',
    source: 'user-said',
    actor: 'sean',
    owner: 'sean',
    visibility: 'private'
  });

  it('a private gotcha with onarchive supplied shows a live Archive button, Edit, Share and Delete', async () => {
    const onedit = vi.fn();
    const ondelete = vi.fn();
    const onvisibility = vi.fn();
    const onarchive = vi.fn();
    const screen = await render(DetailPane, {
      memory: privateGotcha,
      loading: false,
      error: null,
      requestedId: privateGotcha.id,
      onedit,
      ondelete,
      onvisibility,
      onarchive
    });

    await expect.element(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Supersede…' })).not.toBeInTheDocument();
    const archive = screen.getByRole('button', { name: 'Archive' });
    await expect.element(archive).toBeInTheDocument();
    await expect.element(archive).not.toBeDisabled();
    await expect.element(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();

    await archive.click();
    expect(onarchive).toHaveBeenCalledWith(privateGotcha.id);

    await screen.getByRole('button', { name: 'Edit' }).click();
    expect(onedit).toHaveBeenCalledWith(privateGotcha.id);
    await screen.getByRole('button', { name: 'Delete' }).click();
    expect(ondelete).toHaveBeenCalledWith(privateGotcha.id);
    await screen.getByRole('button', { name: 'Share' }).click();
    expect(onvisibility).toHaveBeenCalledWith(privateGotcha);
  });

  it('an archived record with onrestore supplied shows a live Restore button calling onrestore', async () => {
    const onrestore = vi.fn();
    const screen = await render(DetailPane, {
      memory: archivedDecision,
      loading: false,
      error: null,
      requestedId: archivedDecision.id,
      onrestore
    });
    const restore = screen.getByRole('button', { name: 'Restore' });
    await expect.element(restore).toBeInTheDocument();
    await expect.element(restore).not.toBeDisabled();
    await expect.element(screen.getByRole('button', { name: 'Archive' })).not.toBeInTheDocument();

    await restore.click();
    expect(onrestore).toHaveBeenCalledWith(archivedDecision.id);
  });

  it('with neither onarchive nor onrestore supplied, neither button renders', async () => {
    const screen = await render(DetailPane, { memory: privateGotcha, loading: false, error: null, requestedId: privateGotcha.id });
    await expect.element(screen.getByRole('button', { name: 'Archive' })).not.toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Restore' })).not.toBeInTheDocument();
  });

  it('with no ondelete supplied, the Delete button does not render', async () => {
    const screen = await render(DetailPane, { memory: privateGotcha, loading: false, error: null, requestedId: privateGotcha.id });
    await expect.element(screen.getByRole('button', { name: 'Delete' })).not.toBeInTheDocument();
  });

  it('a shared record shows "Make private" and clicking it calls onvisibility(memory)', async () => {
    const onvisibility = vi.fn();
    const screen = await render(DetailPane, { memory: sharedDecision, loading: false, error: null, requestedId: sharedDecision.id, onvisibility });
    await screen.getByRole('button', { name: 'Make private' }).click();
    expect(onvisibility).toHaveBeenCalledWith(sharedDecision);
    await expect.element(screen.getByRole('button', { name: 'Share' })).not.toBeInTheDocument();
  });

  it('a rule record shows no Edit and no Share/Make private (delete only), even with onedit/onvisibility supplied', async () => {
    const onedit = vi.fn();
    const onvisibility = vi.fn();
    const ondelete = vi.fn();
    const screen = await render(DetailPane, {
      memory: ruleRecord,
      loading: false,
      error: null,
      requestedId: ruleRecord.id,
      onedit,
      onvisibility,
      ondelete
    });
    await expect.element(screen.getByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Share' })).not.toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Make private' })).not.toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
  });

  it('a discovery record shows no Edit, even with onedit supplied', async () => {
    const onedit = vi.fn();
    const onvisibility = vi.fn();
    const screen = await render(DetailPane, {
      memory: discoveryRecord,
      loading: false,
      error: null,
      requestedId: discoveryRecord.id,
      onedit,
      onvisibility
    });
    await expect.element(screen.getByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Share' })).toBeInTheDocument();
  });
});
