import { render } from 'vitest-browser-svelte';
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
  it('renders a scope chip and the filter categories', async () => {
    const screen = await render(ScopesSidebar, baseProps());
    await expect.element(screen.getByText('selfhosted-cluster')).toBeInTheDocument();
    await expect.element(screen.getByText('gotcha')).toBeInTheDocument();
  });

  it('fires onscope with the scope when a scope button is clicked', async () => {
    const onscope = vi.fn();
    const screen = await render(ScopesSidebar, baseProps({ onscope }));
    // The scope row nests a ScopeChip whose HoverCard trigger renders an
    // <a role="button">, so two role=button elements match; the outer row
    // button (first in DOM) is the one wired to onscope.
    await screen.getByRole('button', { name: /selfhosted-cluster/i }).first().click();
    expect(onscope).toHaveBeenCalledWith('repo:github.com/fzymgc-house/selfhosted-cluster');
  });

  it('toggles a category on via onfilter when its checkbox is clicked', async () => {
    const onfilter = vi.fn();
    const screen = await render(ScopesSidebar, baseProps({ onfilter }));
    await screen.getByRole('checkbox', { name: 'decision' }).click();
    expect(onfilter).toHaveBeenCalledWith(['decision'], '');
  });

  it('toggles a category off when it is already active', async () => {
    const onfilter = vi.fn();
    const screen = await render(ScopesSidebar, baseProps({ categories: ['gotcha'], onfilter }));
    await screen.getByRole('checkbox', { name: 'gotcha' }).click();
    expect(onfilter).toHaveBeenCalledWith([], '');
  });

  it('reflects the active visibility in the select trigger', async () => {
    const screen = await render(ScopesSidebar, baseProps({ visibility: 'shared' }));
    await expect.element(screen.getByRole('button', { name: 'visibility' })).toHaveTextContent('shared');
  });

  it('fires onfilter with the chosen visibility when a select option is clicked', async () => {
    const onfilter = vi.fn();
    const screen = await render(ScopesSidebar, baseProps({ categories: ['gotcha'], onfilter }));
    // Browser tier opens the bits-ui Select for real (no jsdom/happy-dom popover limits).
    await screen.getByRole('button', { name: 'visibility' }).click();
    await screen.getByRole('option', { name: 'private' }).click();
    expect(onfilter).toHaveBeenCalledWith(['gotcha'], 'private');
  });

  it('shows an error message when scopes fail to load', async () => {
    const screen = await render(ScopesSidebar, baseProps({ error: new Error('boom') }));
    await expect.element(screen.getByTestId('scopes-error')).toBeInTheDocument();
    await expect.element(screen.getByText('selfhosted-cluster')).not.toBeInTheDocument();
  });

  it('shows skeletons while loading', async () => {
    const screen = await render(ScopesSidebar, baseProps({ loading: true }));
    await expect.element(screen.getByTestId('scopes-loading')).toBeInTheDocument();
    await expect.element(screen.getByText('selfhosted-cluster')).not.toBeInTheDocument();
  });
});
