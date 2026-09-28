// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { userEvent } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { ListTagsResponseSchema } from '$lib/gen/engram_pb';
import { defaultSearchParams } from '$lib/search/params';
import FacetStrip from './FacetStrip.svelte';

const { listTagsSpy } = vi.hoisted(() => ({ listTagsSpy: vi.fn() }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return { ...actual, engram: { ...actual.engram, listTags: listTagsSpy } };
});

function fakeTags() {
  return create(ListTagsResponseSchema, {
    tags: [
      { tag: 'qdrant', count: 9n },
      { tag: 'zebra', count: 4n }
    ],
    more: false
  });
}

let qc: QueryClient;

function baseProps(overrides: Partial<Parameters<typeof FacetStrip>[1]> = {}) {
  return {
    params: defaultSearchParams(),
    scopesLoading: false,
    scopesError: null,
    onchange: vi.fn(),
    ...overrides
  };
}

function renderStrip(props: Record<string, unknown> = {}) {
  return render(FacetStrip, baseProps(props), { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

beforeEach(() => {
  listTagsSpy.mockReset();
  listTagsSpy.mockResolvedValue(fakeTags());
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('FacetStrip', () => {
  it('shows a category chip\'s count from categoryCounts, dimming a zero-count chip', async () => {
    const screen = await renderStrip({ categoryCounts: { gotcha: 3, decision: 0, convention: 0, preference: 0 } });

    const gotcha = screen.getByRole('button', { name: /gotcha 3/ });
    await expect.element(gotcha).toBeInTheDocument();
    expect(gotcha.element().classList.contains('facet-chip-zero')).toBe(false);

    const decision = screen.getByRole('button', { name: /decision 0/ });
    expect(decision.element().classList.contains('facet-chip-zero')).toBe(true);
  });

  it('toggling a category chip calls onchange with that category added', async () => {
    const onchange = vi.fn();
    const screen = await renderStrip({ onchange });

    await screen.getByRole('button', { name: /gotcha/ }).click();
    expect(onchange).toHaveBeenCalledWith({ categories: ['gotcha'] });
  });

  it('toggling include_superseded calls onchange({ includeSuperseded: true })', async () => {
    const onchange = vi.fn();
    const screen = await renderStrip({ onchange });

    await screen.getByLabelText('include_superseded').click();
    expect(onchange).toHaveBeenCalledWith({ includeSuperseded: true });
  });

  it('removing a tag chip calls onchange with that tag gone', async () => {
    const onchange = vi.fn();
    const params = { ...defaultSearchParams(), tags: ['ci', 'gofmt'] };
    const screen = await renderStrip({ params, onchange });

    await screen.getByRole('button', { name: 'remove #ci' }).click();
    expect(onchange).toHaveBeenCalledWith({ tags: ['gofmt'] });
  });

  it('setting the created-window "after" date calls onchange with an RFC3339 midnight-UTC string', async () => {
    const onchange = vi.fn();
    const screen = await renderStrip({ onchange });

    await screen.getByRole('button', { name: 'created window' }).click();
    await screen.getByLabelText('created after').fill('2026-01-01');

    expect(onchange).toHaveBeenCalledWith({ createdAfter: '2026-01-01T00:00:00Z' });
  });

  it('the cross_spine chip reads "cross_spine"; turning it off calls onchange({ crossSpine: false })', async () => {
    const onchange = vi.fn();
    const screen = await renderStrip({ onchange });

    await expect.element(screen.getByText('cross_spine')).toBeInTheDocument();
    await screen.getByLabelText('cross_spine').click();
    expect(onchange).toHaveBeenCalledWith({ crossSpine: false });
  });

  it('hides the cross_spine chip while a scope is set', async () => {
    const params = { ...defaultSearchParams(), scope: 'repo:engram', crossSpine: false };
    const screen = await renderStrip({ params });
    await expect.element(screen.getByText('cross_spine')).not.toBeInTheDocument();
  });

  // WR-03: ScopeCombobox's own Retry button calls onretry?.() — FacetStrip
  // must actually thread its onretry prop through, not swallow it, or that
  // button silently does nothing on a ListScopes failure.
  it('wires its onretry prop through to ScopeCombobox\'s Retry button', async () => {
    const onretry = vi.fn();
    const screen = await renderStrip({ scopesError: new Error('boom'), onretry });

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
      const screen = await renderStrip({ categoryCounts: { gotcha: 3, decision: 1, convention: 2, preference: 0 } });
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

  // TAGS-02/D-16: the "+ tag" picker sits after the #tag chips and shares the
  // TagBars' cached ListTags(scope, 1000) query for the same scope.
  describe('"+ tag" picker (TAGS-02, D-16)', () => {
    it('renders a "+ tag" trigger after the #tag chips', async () => {
      const params = { ...defaultSearchParams(), tags: ['existing'] };
      const screen = await renderStrip({ params });

      await expect.element(screen.getByRole('button', { name: '+ tag' })).toBeInTheDocument();
    });

    it('choosing "qdrant" calls onchange({ tags: [...existing, "qdrant"] })', async () => {
      const onchange = vi.fn();
      const params = { ...defaultSearchParams(), tags: ['existing'] };
      const screen = await renderStrip({ params, onchange });

      await screen.getByRole('button', { name: '+ tag' }).click();
      const input = screen.getByRole('combobox', { name: 'Filter tags' });
      await input.fill('qdrant');
      await userEvent.keyboard('{Enter}');

      expect(onchange).toHaveBeenCalledWith({ tags: ['existing', 'qdrant'] });
    });

    it('choosing an already-active tag calls onchange with no duplicate added', async () => {
      const onchange = vi.fn();
      const params = { ...defaultSearchParams(), tags: ['qdrant'] };
      const screen = await renderStrip({ params, onchange });

      await screen.getByRole('button', { name: '+ tag' }).click();
      const input = screen.getByRole('combobox', { name: 'Filter tags' });
      await input.fill('qdrant');
      await userEvent.keyboard('{Enter}');

      expect(onchange).toHaveBeenCalledWith({ tags: ['qdrant'] });
    });

    it('"Add #zzz" for an unmatched tag calls onchange({ tags: [...existing, "zzz"] })', async () => {
      const onchange = vi.fn();
      const params = { ...defaultSearchParams(), tags: ['existing'] };
      const screen = await renderStrip({ params, onchange });

      await screen.getByRole('button', { name: '+ tag' }).click();
      const input = screen.getByRole('combobox', { name: 'Filter tags' });
      await input.fill('zzz');
      await screen.getByText('Add #zzz').click();

      expect(onchange).toHaveBeenCalledWith({ tags: ['existing', 'zzz'] });
    });

    it("with params.scope set, the picker's ListTags call uses that scope", async () => {
      const params = { ...defaultSearchParams(), scope: 'repo:acme/x' };
      const screen = await renderStrip({ params });

      await screen.getByRole('button', { name: '+ tag' }).click();
      await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
      expect(listTagsSpy).toHaveBeenCalledWith({ scope: 'repo:acme/x', limit: 1000n }, expect.anything());
    });
  });
});
