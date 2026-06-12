import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ScopesSidebar from './ScopesSidebar.svelte';
import { create } from '@bufbuild/protobuf';
import { ScopeCountSchema } from '$lib/gen/engram_pb';

const scopes = [create(ScopeCountSchema, { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster', count: 142n })];

describe('ScopesSidebar', () => {
  it('renders a scope chip and the filter categories', () => {
    render(ScopesSidebar, { props: { scopes, activeScope: '', categories: [], visibility: '', loading: false, error: null, onscope: () => {}, onfilter: () => {} } });
    expect(screen.getByText('selfhosted-cluster')).toBeInTheDocument();
    expect(screen.getByText('gotcha')).toBeInTheDocument();
  });
});
