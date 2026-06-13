import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import ScopesSidebar from './ScopesSidebar.svelte';
import { create } from '@bufbuild/protobuf';
import { ScopeCountSchema, type ScopeCount } from '$lib/gen/engram_pb';
import type { Category, Visibility } from '$lib/queries';

const scopes = [create(ScopeCountSchema, { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster', count: 142n })];

type Props = {
  scopes: ScopeCount[]; activeScope: string; categories: Category[]; visibility: Visibility;
  loading: boolean; error: unknown; onscope: (s: string) => void; onfilter: (c: Category[], v: Visibility) => void;
};

function baseProps(overrides: Partial<Props> = {}): Props {
  return { scopes, activeScope: '', categories: [], visibility: '', loading: false, error: null, onscope: () => {}, onfilter: () => {}, ...overrides };
}

describe('ScopesSidebar', () => {
  it('renders a scope chip and the filter categories', () => {
    render(ScopesSidebar, { props: baseProps() });
    expect(screen.getByText('selfhosted-cluster')).toBeInTheDocument();
    expect(screen.getByText('gotcha')).toBeInTheDocument();
  });

  it('fires onscope with the scope when a scope button is clicked', async () => {
    const user = userEvent.setup();
    const onscope = vi.fn();
    render(ScopesSidebar, { props: baseProps({ onscope }) });
    // The scope row nests a ScopeChip whose HoverCard trigger renders an
    // <a role="button">, so two role=button elements match; the outer row
    // button (first in DOM) is the one wired to onscope.
    const [scopeButton] = screen.getAllByRole('button', { name: /selfhosted-cluster/i });
    await user.click(scopeButton);
    expect(onscope).toHaveBeenCalledWith('repo:github.com/fzymgc-house/selfhosted-cluster');
  });

  it('toggles a category on via onfilter when its checkbox is clicked', async () => {
    const user = userEvent.setup();
    const onfilter = vi.fn();
    render(ScopesSidebar, { props: baseProps({ onfilter }) });
    await user.click(screen.getByRole('checkbox', { name: 'decision' }));
    expect(onfilter).toHaveBeenCalledWith(['decision'], '');
  });

  it('toggles a category off when it is already active', async () => {
    const user = userEvent.setup();
    const onfilter = vi.fn();
    render(ScopesSidebar, { props: baseProps({ categories: ['gotcha'], onfilter }) });
    await user.click(screen.getByRole('checkbox', { name: 'gotcha' }));
    expect(onfilter).toHaveBeenCalledWith([], '');
  });

  it('reflects the active visibility in the select trigger', () => {
    render(ScopesSidebar, { props: baseProps({ visibility: 'shared' }) });
    // bits-ui's Select popover cannot be reliably opened under jsdom, so the
    // onValueChange→onfilter path is exercised via the observe page in practice;
    // here we cover the value-binding branch (visibility → trigger label).
    expect(screen.getByRole('button', { name: 'visibility' })).toHaveTextContent('shared');
  });

  it('shows an error message when scopes fail to load', () => {
    render(ScopesSidebar, { props: baseProps({ error: new Error('boom') }) });
    expect(screen.getByTestId('scopes-error')).toBeInTheDocument();
    expect(screen.queryByText('selfhosted-cluster')).not.toBeInTheDocument();
  });

  it('shows skeletons while loading', () => {
    render(ScopesSidebar, { props: baseProps({ loading: true }) });
    expect(screen.getByTestId('scopes-loading')).toBeInTheDocument();
    expect(screen.queryByText('selfhosted-cluster')).not.toBeInTheDocument();
  });
});
