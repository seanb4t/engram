// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import { defaultSearchParams } from '$lib/search/params';
import FacetStrip from './FacetStrip.svelte';

function baseProps(overrides: Partial<Parameters<typeof FacetStrip>[1]> = {}) {
  return {
    params: defaultSearchParams(),
    scopesLoading: false,
    scopesError: null,
    onchange: vi.fn(),
    ...overrides
  };
}

describe('FacetStrip', () => {
  it('shows a category chip\'s count from categoryCounts, dimming a zero-count chip', async () => {
    const screen = await render(FacetStrip, baseProps({ categoryCounts: { gotcha: 3, decision: 0, convention: 0, preference: 0 } }));

    const gotcha = screen.getByRole('button', { name: /gotcha 3/ });
    await expect.element(gotcha).toBeInTheDocument();
    expect(gotcha.element().classList.contains('facet-chip-zero')).toBe(false);

    const decision = screen.getByRole('button', { name: /decision 0/ });
    expect(decision.element().classList.contains('facet-chip-zero')).toBe(true);
  });

  it('toggling a category chip calls onchange with that category added', async () => {
    const onchange = vi.fn();
    const screen = await render(FacetStrip, baseProps({ onchange }));

    await screen.getByRole('button', { name: /gotcha/ }).click();
    expect(onchange).toHaveBeenCalledWith({ categories: ['gotcha'] });
  });

  it('toggling include_superseded calls onchange({ includeSuperseded: true })', async () => {
    const onchange = vi.fn();
    const screen = await render(FacetStrip, baseProps({ onchange }));

    await screen.getByLabelText('include_superseded').click();
    expect(onchange).toHaveBeenCalledWith({ includeSuperseded: true });
  });

  it('removing a tag chip calls onchange with that tag gone', async () => {
    const onchange = vi.fn();
    const params = { ...defaultSearchParams(), tags: ['ci', 'gofmt'] };
    const screen = await render(FacetStrip, baseProps({ params, onchange }));

    await screen.getByRole('button', { name: 'remove #ci' }).click();
    expect(onchange).toHaveBeenCalledWith({ tags: ['gofmt'] });
  });

  it('setting the created-window "after" date calls onchange with an RFC3339 midnight-UTC string', async () => {
    const onchange = vi.fn();
    const screen = await render(FacetStrip, baseProps({ onchange }));

    await screen.getByRole('button', { name: 'created window' }).click();
    await screen.getByLabelText('created after').fill('2026-01-01');

    expect(onchange).toHaveBeenCalledWith({ createdAfter: '2026-01-01T00:00:00Z' });
  });

  it('the cross_spine chip reads "cross_spine"; turning it off calls onchange({ crossSpine: false })', async () => {
    const onchange = vi.fn();
    const screen = await render(FacetStrip, baseProps({ onchange }));

    await expect.element(screen.getByText('cross_spine')).toBeInTheDocument();
    await screen.getByLabelText('cross_spine').click();
    expect(onchange).toHaveBeenCalledWith({ crossSpine: false });
  });

  it('hides the cross_spine chip while a scope is set', async () => {
    const params = { ...defaultSearchParams(), scope: 'repo:engram', crossSpine: false };
    const screen = await render(FacetStrip, baseProps({ params }));
    await expect.element(screen.getByText('cross_spine')).not.toBeInTheDocument();
  });
});
