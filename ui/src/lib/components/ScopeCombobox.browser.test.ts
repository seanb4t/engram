// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { ListScopesResponseSchema, type ListScopesResponse } from '$lib/gen/engram_pb';
import ScopeCombobox from './ScopeCombobox.svelte';

function makeScopes(overrides: MessageInitShape<typeof ListScopesResponseSchema> = {}): ListScopesResponse {
  return create(ListScopesResponseSchema, {
    scopes: [
      { scope: 'repo:engram', count: 12n },
      { scope: 'repo:other', count: 3n }
    ],
    approximate: false,
    ...overrides
  });
}

describe('ScopeCombobox', () => {
  it('lists "Every readable scope (cross_spine)" first, then every ListScopes entry in response order with its count', async () => {
    const onselect = vi.fn();
    const screen = await render(ScopeCombobox, { value: '', crossSpine: true, scopes: makeScopes(), loading: false, error: null, onselect });

    await screen.getByRole('button', { name: /any scope/ }).click();
    await expect.element(screen.getByText('Every readable scope (cross_spine)')).toBeInTheDocument();
    await expect.element(screen.getByText('repo:engram')).toBeInTheDocument();
    await expect.element(screen.getByText('repo:other')).toBeInTheDocument();
    await expect.element(screen.getByText('12')).toBeInTheDocument();
    await expect.element(screen.getByText('3')).toBeInTheDocument();
  });

  it('renders a "~" prefix on counts when ListScopes reports approximate', async () => {
    const onselect = vi.fn();
    const screen = await render(ScopeCombobox, {
      value: '',
      crossSpine: true,
      scopes: makeScopes({ approximate: true }),
      loading: false,
      error: null,
      onselect
    });
    await screen.getByRole('button', { name: /any scope/ }).click();
    await expect.element(screen.getByText('~12')).toBeInTheDocument();
  });

  it('typing filters entries to those containing the substring', async () => {
    const onselect = vi.fn();
    const screen = await render(ScopeCombobox, { value: '', crossSpine: true, scopes: makeScopes(), loading: false, error: null, onselect });
    await screen.getByRole('button', { name: /any scope/ }).click();

    const input = screen.getByRole('combobox');
    await input.fill('eng');

    await expect.element(screen.getByText('repo:engram')).toBeInTheDocument();
    await expect.element(screen.getByText('repo:other')).not.toBeInTheDocument();
  });

  it('shows "No scope matches zzz" when nothing matches, keeping the pinned cross_spine entry', async () => {
    const onselect = vi.fn();
    const screen = await render(ScopeCombobox, { value: '', crossSpine: true, scopes: makeScopes(), loading: false, error: null, onselect });
    await screen.getByRole('button', { name: /any scope/ }).click();

    const input = screen.getByRole('combobox');
    await input.fill('zzz');

    await expect.element(screen.getByText('No scope matches zzz')).toBeInTheDocument();
    await expect.element(screen.getByText('Every readable scope (cross_spine)')).toBeInTheDocument();
  });

  it('selecting a scope calls onselect with that scope', async () => {
    const onselect = vi.fn();
    const screen = await render(ScopeCombobox, { value: '', crossSpine: true, scopes: makeScopes(), loading: false, error: null, onselect });
    await screen.getByRole('button', { name: /any scope/ }).click();

    await screen.getByText('repo:engram').click();
    expect(onselect).toHaveBeenCalledWith('repo:engram');
  });

  it('shows a "loading scopes…" row while ListScopes loads', async () => {
    const onselect = vi.fn();
    const screen = await render(ScopeCombobox, { value: '', crossSpine: true, scopes: undefined, loading: true, error: null, onselect });
    await screen.getByRole('button', { name: /any scope/ }).click();
    await expect.element(screen.getByText('loading scopes…')).toBeInTheDocument();
  });

  it('shows "Could not load scopes" with a Retry entry on a ListScopes failure', async () => {
    const onselect = vi.fn();
    const onretry = vi.fn();
    const screen = await render(ScopeCombobox, {
      value: '',
      crossSpine: true,
      scopes: undefined,
      loading: false,
      error: new Error('boom'),
      onselect,
      onretry
    });
    await screen.getByRole('button', { name: /any scope/ }).click();
    await expect.element(screen.getByText('Could not load scopes')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Retry' }).click();
    expect(onretry).toHaveBeenCalledTimes(1);
  });
});
