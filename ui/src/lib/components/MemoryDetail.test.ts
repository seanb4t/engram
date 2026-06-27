import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import MemoryDetail from './MemoryDetail.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const withSummary = create(MemorySchema, {
  id: '1', content: 'full **body** here', summary: 'terse digest line',
  summarySource: 'auto', category: 'gotcha',
  scope: 'repo:github.com/fzymgc-house/selfhosted-cluster',
  source: 'agent-inferred', actor: 'sean', visibility: 'private', tags: ['mcp']
});
const noSummary = create(MemorySchema, {
  id: '2', content: 'only content here', summary: '', summarySource: '',
  category: 'decision', scope: 'repo:x', source: 'user-said', actor: 'sean', visibility: 'private'
});

describe('MemoryDetail', () => {
  it('defaults to the Summary tab and shows summary + auto provenance', () => {
    render(MemoryDetail, { props: { memory: withSummary, loading: false, error: null } });
    expect(screen.getByText('terse digest line')).toBeInTheDocument();
    expect(screen.getByText(/auto/i)).toBeInTheDocument();
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
