// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { describe, it, expect, vi } from 'vitest';
import { rankedHeaderParts, type HeaderPart, type HiddenCounts } from '$lib/search/recall-header';
import ResultsHeader from './ResultsHeader.svelte';

const zeroHidden: HiddenCounts = { total: 0, archived: 0, superseded: 0, expired: 0, scheduled: 0 };

describe('ResultsHeader', () => {
  it('reveals searched_scopes with each scope\'s hit count "in top k", zeros included, when the scopes button is clicked', async () => {
    const parts = rankedHeaderParts({ hits: 3, scopeCount: 3, query: 'x', reranked: false, hidden: zeroHidden });
    const scopeHits = [
      { scope: 'repo:a', hits: 2, searched: true },
      { scope: 'repo:b', hits: 1, searched: true },
      { scope: 'repo:c', hits: 0, searched: true }
    ];
    const screen = await render(ResultsHeader, { parts, scopeHits, k: 50, busy: false });

    const button = screen.getByRole('button', { name: /across 3 scopes/ });
    await button.click();

    await expect.element(screen.getByText('searched_scopes:')).toBeInTheDocument();
    await expect.element(screen.getByText('repo:a')).toBeInTheDocument();
    await expect.element(screen.getByText('repo:c')).toBeInTheDocument();
    expect(screen.container.querySelectorAll('.rh-scope-note').length).toBe(3);
    expect(screen.container.textContent).toContain('in top 50');
  });

  it('carries the hidden clause tooltip with the per-state breakdown', async () => {
    const hidden: HiddenCounts = { total: 2, archived: 1, superseded: 1, expired: 0, scheduled: 0 };
    const parts = rankedHeaderParts({ hits: 5, scopeCount: 2, query: 'x', reranked: false, hidden });
    const screen = await render(ResultsHeader, { parts, k: 50, busy: false });

    const hiddenEl = screen.container.querySelector('[title="1 archived, 1 superseded"]');
    expect(hiddenEl).toBeTruthy();
    expect(hiddenEl?.textContent).toContain('2 hidden by recall gate');
  });

  it('ellipsizes a long query in the mono code span and keeps the full text in its title', async () => {
    const longQuery = 'q'.repeat(200);
    const parts: HeaderPart[] = [{ kind: 'query', text: `for ${longQuery}`, title: longQuery }];
    const screen = await render(ResultsHeader, { parts, k: 50, busy: false });

    const code = screen.container.querySelector('.rh-query');
    expect(code?.getAttribute('title')).toBe(longQuery);
    expect(code?.textContent).toBe(longQuery);
    const style = code ? getComputedStyle(code) : undefined;
    expect(style?.textOverflow).toBe('ellipsis');
  });
});

describe('ResultsHeader — bulk bar (D-03)', () => {
  const parts = rankedHeaderParts({ hits: 3, scopeCount: 1, query: 'x', reranked: false, hidden: zeroHidden });

  it('with selection.count 0 or undefined, the normal parts line renders unchanged', async () => {
    const noSelection = await render(ResultsHeader, { parts, k: 50, busy: false });
    expect(noSelection.container.querySelector('[role="toolbar"]')).toBeNull();
    await expect.element(noSelection.container.querySelector('.rh-line') as HTMLElement).toBeInTheDocument();

    const zeroCount = await render(ResultsHeader, {
      parts,
      k: 50,
      busy: false,
      selection: { count: 0, onclear: vi.fn() }
    });
    expect(zeroCount.container.querySelector('[role="toolbar"]')).toBeNull();
    await expect.element(zeroCount.container.querySelector('.rh-line') as HTMLElement).toBeInTheDocument();
  });

  it('with count 3 and archive/restore supplied, renders the bulk bar with Archive/Restore/clear and no Supersede', async () => {
    const onarchive = vi.fn();
    const onrestore = vi.fn();
    const onclear = vi.fn();
    const screen = await render(ResultsHeader, {
      parts,
      k: 50,
      busy: false,
      selection: { count: 3, onarchive, onrestore, onclear }
    });

    const toolbar = screen.getByRole('toolbar', { name: 'Bulk actions' });
    await expect.element(toolbar).toBeInTheDocument();
    await expect.element(screen.getByText('3 selected')).toBeInTheDocument();
    await expect.element(screen.getByRole('button', { name: 'Supersede into one…' })).not.toBeInTheDocument();

    const archiveBtn = screen.getByRole('button', { name: /Archive/ });
    await expect.element(archiveBtn).toBeInTheDocument();
    const restoreBtn = screen.getByRole('button', { name: /Restore/ });
    await expect.element(restoreBtn).toBeInTheDocument();

    const kbdTexts = Array.from(toolbar.element().querySelectorAll('kbd')).map((el) => el.textContent);
    expect(kbdTexts).toEqual(['a', '⇧A']);

    await archiveBtn.click();
    expect(onarchive).toHaveBeenCalledTimes(1);
    await screen.getByRole('button', { name: 'clear' }).click();
    expect(onclear).toHaveBeenCalledTimes(1);
  });

  it('with onsupersede supplied, renders "Supersede 3 into one…" with kbd ⇧S', async () => {
    const onsupersede = vi.fn();
    const screen = await render(ResultsHeader, {
      parts,
      k: 50,
      busy: false,
      selection: { count: 3, onsupersede, onclear: vi.fn() }
    });
    await expect.element(screen.getByText('Supersede 3 into one…')).toBeInTheDocument();
    const toolbar = screen.getByRole('toolbar', { name: 'Bulk actions' });
    const kbdTexts = Array.from(toolbar.element().querySelectorAll('kbd')).map((el) => el.textContent);
    expect(kbdTexts).toEqual(['⇧S']);
  });

  it('busy disables the verb buttons but never the clear button', async () => {
    const onarchive = vi.fn();
    const onclear = vi.fn();
    const screen = await render(ResultsHeader, {
      parts,
      k: 50,
      busy: false,
      selection: { count: 3, busy: true, onarchive, onclear }
    });
    await expect.element(screen.getByRole('button', { name: /Archive/ })).toBeDisabled();
    await expect.element(screen.getByRole('button', { name: 'clear' })).not.toBeDisabled();
  });

  it('the bulk bar has the same height as the normal header', async () => {
    const plain = await render(ResultsHeader, { parts, k: 50, busy: false });
    const bulk = await render(ResultsHeader, {
      parts,
      k: 50,
      busy: false,
      selection: { count: 2, onarchive: vi.fn(), onclear: vi.fn() }
    });
    const plainHeight = (plain.container.querySelector('.results-header') as HTMLElement).getBoundingClientRect()
      .height;
    const bulkHeight = (bulk.container.querySelector('.results-header') as HTMLElement).getBoundingClientRect()
      .height;
    expect(bulkHeight).toBe(plainHeight);
  });

  it('below the narrow breakpoint the actions wrap to a second line before any label truncates', async () => {
    const screen = await render(ResultsHeader, {
      parts,
      k: 50,
      busy: false,
      selection: {
        count: 3,
        onsupersede: vi.fn(),
        onarchive: vi.fn(),
        onrestore: vi.fn(),
        onclear: vi.fn()
      }
    });
    screen.container.style.width = '360px';
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));

    const buttons = Array.from(screen.container.querySelectorAll('.rh-bulk button')) as HTMLElement[];
    const tops = new Set(buttons.map((b) => b.getBoundingClientRect().top));
    expect(tops.size).toBeGreaterThan(1);
    for (const b of buttons) {
      expect(b.scrollWidth).toBeLessThanOrEqual(b.clientWidth + 1);
    }
    await page.screenshot();
  });
});
