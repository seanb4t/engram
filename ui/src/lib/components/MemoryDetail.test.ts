import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import MemoryDetail from './MemoryDetail.svelte';
import { create } from '@bufbuild/protobuf';
import { MemorySchema } from '$lib/gen/engram_pb';

const mem = create(MemorySchema, { id: '1', content: 'full body here', category: 'gotcha', scope: 'repo:github.com/fzymgc-house/selfhosted-cluster', source: 'agent-inferred', actor: 'sean', visibility: 'private', tags: ['mcp'] });

describe('MemoryDetail', () => {
  it('shows the full scope verbatim and the body', () => {
    render(MemoryDetail, { props: { memory: mem, loading: false, error: null } });
    expect(screen.getByText('repo:github.com/fzymgc-house/selfhosted-cluster')).toBeInTheDocument();
    expect(screen.getByText('full body here')).toBeInTheDocument();
  });
  it('prompts to select when nothing is chosen', () => {
    render(MemoryDetail, { props: { memory: undefined, loading: false, error: null } });
    expect(screen.getByText(/select a record/i)).toBeInTheDocument();
  });
});
