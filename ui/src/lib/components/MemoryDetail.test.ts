import { render, screen, within } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, it, expect } from 'vitest';
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

describe('MemoryDetail', () => {
  it('defaults to the Summary tab and shows summary + auto provenance', () => {
    render(MemoryDetail, { props: { memory: withSummary, loading: false, error: null } });
    expect(screen.getByText('terse digest line')).toBeInTheDocument();
    expect(screen.getByText(/auto/i)).toBeInTheDocument();
  });
  it('marks a client-authored summary as authored, not auto', () => {
    render(MemoryDetail, { props: { memory: clientSummary, loading: false, error: null } });
    expect(screen.getByText('human-authored digest')).toBeInTheDocument();
    expect(screen.getByText('authored')).toBeInTheDocument();
    expect(screen.queryByText('✦ auto')).not.toBeInTheDocument();
  });
  it('shows no provenance badge when the summary has no recorded source', () => {
    render(MemoryDetail, { props: { memory: unsourcedSummary, loading: false, error: null } });
    expect(screen.getByText('sourceless digest')).toBeInTheDocument();
    expect(screen.queryByText('authored')).not.toBeInTheDocument();
    expect(screen.queryByText('✦ auto')).not.toBeInTheDocument();
  });
  it('exposes scope, actor, source, visibility, and tags on the Meta tab', async () => {
    const user = userEvent.setup();
    render(MemoryDetail, { props: { memory: withSummary, loading: false, error: null } });
    await user.click(screen.getByRole('tab', { name: 'Meta' }));
    // Once Meta is active, bits-ui leaves only its panel un-hidden, so the lone
    // accessible tabpanel is the Meta panel — scope the assertions to it.
    const meta = screen.getByRole('tabpanel');
    expect(within(meta).getByText(withSummary.scope)).toBeInTheDocument();
    expect(within(meta).getByText('sean')).toBeInTheDocument();
    expect(within(meta).getByText('agent-inferred')).toBeInTheDocument();
    expect(within(meta).getByText('private')).toBeInTheDocument();
    expect(within(meta).getByText('mcp')).toBeInTheDocument();
    expect(within(meta).getByText('routing')).toBeInTheDocument();
  });
  it('falls through to Content (rendered markdown) when there is no summary', () => {
    render(MemoryDetail, { props: { memory: noSummary, loading: false, error: null } });
    expect(screen.getByText('only content here')).toBeInTheDocument();
  });
  it('prompts to select when nothing is chosen', () => {
    render(MemoryDetail, { props: { memory: undefined, loading: false, error: null } });
    expect(screen.getByText(/select a record/i)).toBeInTheDocument();
  });
  it('shows a loading indicator while fetching', () => {
    render(MemoryDetail, { props: { memory: undefined, loading: true, error: null } });
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });
  it('shows a not-found message for a NotFound error', () => {
    render(MemoryDetail, { props: { memory: undefined, loading: false, error: new ConnectError('missing', Code.NotFound) } });
    expect(screen.getByText(/record not found/i)).toBeInTheDocument();
  });
  it('shows a generic failure message for a non-NotFound error', () => {
    render(MemoryDetail, { props: { memory: undefined, loading: false, error: new Error('boom') } });
    expect(screen.getByText(/failed to load record/i)).toBeInTheDocument();
  });
});
