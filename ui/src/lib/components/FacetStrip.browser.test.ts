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

  // WR-03: ScopeCombobox's own Retry button calls onretry?.() — FacetStrip
  // must actually thread its onretry prop through, not swallow it, or that
  // button silently does nothing on a ListScopes failure.
  it('wires its onretry prop through to ScopeCombobox\'s Retry button', async () => {
    const onretry = vi.fn();
    const screen = await render(FacetStrip, baseProps({ scopesError: new Error('boom'), onretry }));

    await screen.getByRole('button', { name: /any scope/ }).click();
    await expect.element(screen.getByText('Could not load scopes')).toBeInTheDocument();

    await screen.getByRole('button', { name: 'Retry' }).click();
    expect(onretry).toHaveBeenCalledTimes(1);
  });

  // UI-REVIEW WARNING: the strip is one horizontally scrollable line; when it
  // overflows (e.g. 1024px viewport), the user must SEE that chips continue
  // off-screen without having to hover it first.
  describe('horizontal-scroll affordance', () => {
    const nextFrame = () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
    const maskOf = (el: HTMLElement) => {
      const cs = getComputedStyle(el);
      return cs.maskImage || cs.getPropertyValue('-webkit-mask-image');
    };

    async function renderAt(width: number) {
      const screen = await render(FacetStrip, baseProps({ categoryCounts: { gotcha: 3, decision: 1, convention: 2, preference: 0 } }));
      screen.container.style.width = `${width}px`;
      await nextFrame();
      const q = (sel: string) => screen.container.querySelector(sel) as HTMLElement | null;
      return {
        root: q('.facet-strip') as HTMLElement,
        viewport: q('[data-slot="scroll-area-viewport"]') as HTMLElement,
        scrollbar: () => q('[data-slot="scroll-area-scrollbar"][data-orientation="horizontal"]')
      };
    }

    it('an overflowing strip shows a scrollbar and fades the edge that hides chips, without hover', async () => {
      const { viewport, scrollbar } = await renderAt(600);
      expect(viewport.scrollWidth).toBeGreaterThan(viewport.clientWidth);

      await expect.poll(() => scrollbar()?.getBoundingClientRect().height ?? 0).toBeGreaterThan(0);
      expect(getComputedStyle(scrollbar() as HTMLElement).visibility).not.toBe('hidden');

      // At the start only the trailing edge hides chips, so only it fades.
      await expect.poll(() => maskOf(viewport)).toContain('linear-gradient');
      const atStart = maskOf(viewport);
      expect(atStart).toMatch(/^linear-gradient\(to right, (rgb\(0, 0, 0\)|black)/);
      expect(atStart).toMatch(/(rgba\(0, 0, 0, 0\)|transparent)\)$/);

      viewport.scrollLeft = viewport.scrollWidth;
      viewport.dispatchEvent(new Event('scroll'));
      // Scrolled to the end: the leading edge now hides chips, the trailing one does not.
      await expect.poll(() => maskOf(viewport)).not.toBe(atStart);
      expect(maskOf(viewport)).toMatch(/^linear-gradient\(to right, (rgba\(0, 0, 0, 0\)|transparent)/);
      expect(maskOf(viewport)).not.toMatch(/(rgba\(0, 0, 0, 0\)|transparent)\)$/);
    });

    it('a strip that fits shows neither a scrollbar nor a fade', async () => {
      const { viewport, scrollbar } = await renderAt(3000);
      expect(viewport.scrollWidth).toBeLessThanOrEqual(viewport.clientWidth);
      await nextFrame();
      expect(scrollbar()?.getBoundingClientRect().height ?? 0).toBe(0);
      expect(maskOf(viewport)).toBe('none');
    });
  });
});
