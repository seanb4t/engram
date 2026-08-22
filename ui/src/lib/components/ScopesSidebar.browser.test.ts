import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import ScopesSidebar from './ScopesSidebar.svelte';
import { create } from '@bufbuild/protobuf';
import { ScopeCountSchema, type ScopeCount } from '$lib/gen/engram_pb';
import type { Category, Visibility } from '$lib/queries';

const scopes = [create(ScopeCountSchema, { scope: 'repo:github.com/fzymgc-house/selfhosted-cluster', count: 142n })];

type Props = {
  scopes: ScopeCount[]; activeScope: string; categories: Category[]; visibility: Visibility;
  includeArchived: boolean; includeSuperseded: boolean; includeScheduled: boolean;
  loading: boolean; error: unknown; onscope: (s: string) => void; onfilter: (c: Category[], v: Visibility) => void;
  oninclude: (archived: boolean, superseded: boolean, scheduled: boolean) => void;
};

function baseProps(overrides: Partial<Props> = {}): Props {
  return {
    scopes, activeScope: '', categories: [], visibility: '',
    includeArchived: false, includeSuperseded: false, includeScheduled: false,
    loading: false, error: null, onscope: () => {}, onfilter: () => {}, oninclude: () => {},
    ...overrides
  };
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

  it('renders the three include-state checkboxes with their exact lowercase labels', async () => {
    const screen = await render(ScopesSidebar, baseProps());
    await expect.element(screen.getByRole('checkbox', { name: 'include archived' })).toBeInTheDocument();
    await expect.element(screen.getByRole('checkbox', { name: 'include superseded' })).toBeInTheDocument();
    await expect.element(screen.getByRole('checkbox', { name: 'include scheduled' })).toBeInTheDocument();
    await expect.element(screen.getByText('include archived')).toBeInTheDocument();
    await expect.element(screen.getByText('include superseded')).toBeInTheDocument();
    await expect.element(screen.getByText('include scheduled')).toBeInTheDocument();
  });

  it('reflects each include prop as its checkbox checked state', async () => {
    const screen = await render(ScopesSidebar, baseProps({ includeArchived: true, includeScheduled: true }));
    await expect.element(screen.getByRole('checkbox', { name: 'include archived' })).toBeChecked();
    await expect.element(screen.getByRole('checkbox', { name: 'include superseded' })).not.toBeChecked();
    await expect.element(screen.getByRole('checkbox', { name: 'include scheduled' })).toBeChecked();
  });

  it('checking include archived calls oninclude with the other two incoming flags preserved', async () => {
    const oninclude = vi.fn();
    const screen = await render(ScopesSidebar, baseProps({ includeSuperseded: true, oninclude }));
    await screen.getByRole('checkbox', { name: 'include archived' }).click();
    expect(oninclude).toHaveBeenCalledWith(true, true, false);
  });

  it('unchecking a checked include flag calls oninclude with that flag false and the other two unchanged', async () => {
    const oninclude = vi.fn();
    const screen = await render(ScopesSidebar, baseProps({ includeArchived: true, includeSuperseded: true, oninclude }));
    await screen.getByRole('checkbox', { name: 'include superseded' }).click();
    expect(oninclude).toHaveBeenCalledWith(true, false, false);
  });

  it('keeps every existing control rendering and invoking onfilter unchanged alongside the new toggles', async () => {
    const onfilter = vi.fn();
    const screen = await render(ScopesSidebar, baseProps({ onfilter }));
    await expect.element(screen.getByRole('checkbox', { name: 'gotcha' })).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'visibility' })).toBeInTheDocument();
    await screen.getByRole('checkbox', { name: 'gotcha' }).click();
    expect(onfilter).toHaveBeenCalledWith(['gotcha'], '');
  });
});
