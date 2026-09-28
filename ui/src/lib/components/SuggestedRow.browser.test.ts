// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { FilterSuggestionSchema, SuggestionSource, type FilterSuggestion } from '$lib/gen/engram_pb';
import { defaultSearchParams } from '$lib/search/params';
import SuggestedRow from './SuggestedRow.svelte';

function categorySuggestion(value: string): FilterSuggestion {
  return create(FilterSuggestionSchema, { kind: { case: 'category', value }, source: SuggestionSource.DECIDED });
}
function tagSuggestion(value: string): FilterSuggestion {
  return create(FilterSuggestionSchema, { kind: { case: 'tag', value }, source: SuggestionSource.MATCHED });
}
function scopeSuggestion(value: string): FilterSuggestion {
  return create(FilterSuggestionSchema, { kind: { case: 'scope', value }, source: SuggestionSource.DECIDED });
}

function threeSuggestions(): FilterSuggestion[] {
  return [categorySuggestion('decision'), categorySuggestion('gotcha'), tagSuggestion('qdrant')];
}

async function renderRow(
  suggestions: FilterSuggestion[],
  overrides: { onchange?: ReturnType<typeof vi.fn>; ondismiss?: ReturnType<typeof vi.fn>; onempty?: ReturnType<typeof vi.fn> } = {}
) {
  const onchange = overrides.onchange ?? vi.fn();
  const ondismiss = overrides.ondismiss ?? vi.fn();
  const screen = await render(SuggestedRow, {
    suggestions,
    params: defaultSearchParams(),
    onchange,
    ondismiss,
    onempty: overrides.onempty
  });
  return { screen, onchange, ondismiss };
}

afterEach(() => {
  document.documentElement.classList.remove('dark');
});

describe('SuggestedRow — toolbar role and roving-tabindex keyboard model (D-13)', () => {
  it('the row is role="toolbar" with aria-label "Suggested filters"', async () => {
    const { screen } = await renderRow(threeSuggestions());
    await expect.element(screen.getByRole('toolbar', { name: 'Suggested filters' })).toBeInTheDocument();
  });

  it('Tab focuses the first accept control by default; every other accept control is tabindex -1', async () => {
    const before = document.createElement('button');
    before.id = 'before-toolbar';
    before.textContent = 'before';
    document.body.insertBefore(before, document.body.firstChild);
    const { screen } = await renderRow(threeSuggestions());

    before.focus();
    await userEvent.keyboard('{Tab}');

    const decisionEl = screen.getByRole('button', { name: 'Suggested filter, not applied: decision' }).element();
    expect(document.activeElement).toBe(decisionEl);
    expect(decisionEl.getAttribute('tabindex')).toBe('0');
    expect(screen.getByRole('button', { name: 'Suggested filter, not applied: gotcha' }).element().getAttribute('tabindex')).toBe(
      '-1'
    );
    expect(
      screen.getByRole('button', { name: 'Suggested filter, not applied: #qdrant' }).element().getAttribute('tabindex')
    ).toBe('-1');
    before.remove();
  });

  it('ArrowRight/ArrowLeft move roving focus among accept controls and clamp at the ends', async () => {
    const { screen } = await renderRow(threeSuggestions());
    const decision = screen.getByRole('button', { name: 'Suggested filter, not applied: decision' }).element();
    const gotcha = screen.getByRole('button', { name: 'Suggested filter, not applied: gotcha' }).element();
    const qdrant = screen.getByRole('button', { name: 'Suggested filter, not applied: #qdrant' }).element();

    (decision as HTMLElement).focus();
    await userEvent.keyboard('{ArrowRight}');
    expect(document.activeElement).toBe(gotcha);
    await userEvent.keyboard('{ArrowRight}');
    expect(document.activeElement).toBe(qdrant);
    await userEvent.keyboard('{ArrowRight}');
    expect(document.activeElement).toBe(qdrant);

    await userEvent.keyboard('{ArrowLeft}');
    expect(document.activeElement).toBe(gotcha);
    await userEvent.keyboard('{ArrowLeft}');
    expect(document.activeElement).toBe(decision);
    await userEvent.keyboard('{ArrowLeft}');
    expect(document.activeElement).toBe(decision);
  });

  it('Home/End jump to the first/last accept control', async () => {
    const { screen } = await renderRow(threeSuggestions());
    const decision = screen.getByRole('button', { name: 'Suggested filter, not applied: decision' }).element();
    const qdrant = screen.getByRole('button', { name: 'Suggested filter, not applied: #qdrant' }).element();

    (decision as HTMLElement).focus();
    await userEvent.keyboard('{End}');
    expect(document.activeElement).toBe(qdrant);
    await userEvent.keyboard('{Home}');
    expect(document.activeElement).toBe(decision);
  });

  it('Enter and Space each call onchange once with that chip’s acceptPartial', async () => {
    const { screen, onchange } = await renderRow(threeSuggestions());
    const decision = screen.getByRole('button', { name: 'Suggested filter, not applied: decision' });
    (decision.element() as HTMLElement).focus();
    await userEvent.keyboard('{Enter}');
    expect(onchange).toHaveBeenCalledTimes(1);
    expect(onchange).toHaveBeenCalledWith({ categories: ['decision'] });

    const gotcha = screen.getByRole('button', { name: 'Suggested filter, not applied: gotcha' });
    (gotcha.element() as HTMLElement).focus();
    await userEvent.keyboard(' ');
    expect(onchange).toHaveBeenCalledTimes(2);
    expect(onchange).toHaveBeenCalledWith({ categories: ['gotcha'] });
  });

  it('Delete and Backspace each call ondismiss once with that chip’s key', async () => {
    const { screen, ondismiss } = await renderRow(threeSuggestions());
    const decision = screen.getByRole('button', { name: 'Suggested filter, not applied: decision' });
    (decision.element() as HTMLElement).focus();
    await userEvent.keyboard('{Delete}');
    expect(ondismiss).toHaveBeenCalledTimes(1);
    expect(ondismiss).toHaveBeenCalledWith('category:decision');

    const gotcha = screen.getByRole('button', { name: 'Suggested filter, not applied: gotcha' });
    (gotcha.element() as HTMLElement).focus();
    await userEvent.keyboard('{Backspace}');
    expect(ondismiss).toHaveBeenCalledTimes(2);
    expect(ondismiss).toHaveBeenCalledWith('category:gotcha');
  });

  it('every dismiss (×) control is tabindex -1 and is never reached by Tab or arrows', async () => {
    const { screen } = await renderRow(threeSuggestions());
    for (const label of ['decision', 'gotcha', '#qdrant']) {
      const btn = screen.getByRole('button', { name: `Dismiss suggested filter: ${label}` }).element();
      expect(btn.getAttribute('tabindex')).toBe('-1');
    }
  });

  it('no accept or dismiss button is nested inside another button', async () => {
    const { screen } = await renderRow(threeSuggestions());
    const buttons = screen.container.querySelectorAll('button');
    for (const b of buttons) {
      expect(b.querySelector('button')).toBeNull();
    }
  });
});

describe('SuggestedRow — visual contract', () => {
  it('the chip background is transparent at rest, on hover and while focused; the border style is dashed', async () => {
    const { screen } = await renderRow([categorySuggestion('decision')]);
    const chip = screen.container.querySelector('.suggested-chip') as HTMLElement;
    const accept = screen.getByRole('button', { name: 'Suggested filter, not applied: decision' });

    let cs = getComputedStyle(chip);
    expect(cs.backgroundColor).toMatch(/rgba?\(0, ?0, ?0, ?0\)|transparent/);
    expect(cs.borderTopStyle).toBe('dashed');

    await userEvent.hover(accept);
    cs = getComputedStyle(chip);
    expect(cs.backgroundColor).toMatch(/rgba?\(0, ?0, ?0, ?0\)|transparent/);

    (accept.element() as HTMLElement).focus();
    cs = getComputedStyle(chip);
    expect(cs.backgroundColor).toMatch(/rgba?\(0, ?0, ?0, ?0\)|transparent/);
  });

  it('the label color differs from --text-faint’s resolved value (never rendered as caption-dim text)', async () => {
    const { screen } = await renderRow([categorySuggestion('decision')]);
    const label = screen.container.querySelector('.suggested-label') as HTMLElement;
    const caption = screen.container.querySelector('.suggested-caption') as HTMLElement;
    expect(getComputedStyle(label).color).not.toBe(getComputedStyle(caption).color);
  });

  it('a very long scope label ellipsizes within the max chip width and carries the full text in title', async () => {
    const longScope = 'repo:' + 'a'.repeat(400);
    const { screen } = await renderRow([scopeSuggestion(longScope)]);
    const accept = screen.getByRole('button', { name: `Suggested filter, not applied: ${longScope}` });
    const acceptEl = accept.element() as HTMLElement;
    const label = acceptEl.querySelector('.suggested-label') as HTMLElement;

    expect(acceptEl.getAttribute('title')).toBe(longScope);
    expect(label.scrollWidth).toBeGreaterThan(label.clientWidth);

    const chip = acceptEl.closest('.suggested-chip') as HTMLElement;
    const maxWidthPx = parseFloat(getComputedStyle(chip).maxWidth);
    expect(chip.getBoundingClientRect().width).toBeLessThanOrEqual(maxWidthPx + 1);
  });

  it('with 20 chips the row height matches a single-chip row exactly (no wrap to a second line)', async () => {
    const one = await renderRow([tagSuggestion('solo')]);
    const oneRowHeight = (one.screen.container.querySelector('.suggested-row') as HTMLElement).getBoundingClientRect()
      .height;

    const many = Array.from({ length: 20 }, (_, i) => tagSuggestion(`tag${i}`));
    const { screen } = await renderRow(many);
    const row = screen.container.querySelector('.suggested-row') as HTMLElement;
    expect(row.getBoundingClientRect().height).toBeCloseTo(oneRowHeight, 0);
  });
});

describe('SuggestedRow — screenshots (light + dark)', () => {
  it('captures the populated row in both themes', async () => {
    await renderRow(threeSuggestions());

    document.documentElement.classList.remove('dark');
    await page.screenshot();

    document.documentElement.classList.add('dark');
    await page.screenshot();
  });
});
