import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ScopeChip from './ScopeChip.svelte';

describe('ScopeChip', () => {
  it('shows repo name prominently and the type badge', () => {
    render(ScopeChip, { props: { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster' } });
    expect(screen.getByText('selfhosted-cluster')).toBeInTheDocument();
    expect(screen.getByText('repo')).toBeInTheDocument();
  });
  it('keeps the full scope available (title attr) — never destroyed', () => {
    render(ScopeChip, { props: { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster' } });
    expect(screen.getByTitle('repo:github.com/fzymgc-house/selfhosted-cluster')).toBeInTheDocument();
  });
});
