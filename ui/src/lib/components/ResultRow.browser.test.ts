// app.css is not pulled in by isolated component mounts (only
// +layout.svelte imports it) — without it every calc(N*var(--u)) product
// dimension in ResultRow (row height, the state-chip collapse budget) is an
// INVALID CSS value in this test's cascade and silently falls back to its
// default, making a geometry assertion pass for the wrong reason. Import it
// explicitly so --u and the design tokens are real here, same as the shipped app.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { createRawSnippet } from 'svelte';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import { flashing, flashRows, FLASH_MS } from '$lib/curation/flash.svelte';
import ResultRow from './ResultRow.svelte';

const now = new Date('2030-06-15T12:00:00Z');
const future = timestampFromDate(new Date(now.getTime() + 24 * 60 * 60 * 1000));

describe('ResultRow', () => {
  it('a ranked hit shows score to 2dp and a bar filled to the unrounded score, clamped at 100%', async () => {
    const mem = create(MemorySchema, { id: '1', summary: 'x', category: 'convention', score: 0.8134 });
    const screen = await render(ResultRow, { memory: mem, mode: 'ranked' });
    await expect.element(screen.container.querySelector('.score .num') as HTMLElement).toHaveTextContent('0.81');
    const bar = screen.container.querySelector('.score .bar b') as HTMLElement;
    expect(bar.style.width).toBe('81.34%');

    const overMem = create(MemorySchema, { id: '2', summary: 'x', category: 'convention', score: 1.7 });
    await screen.rerender({ memory: overMem, mode: 'ranked' });
    const barOver = screen.container.querySelector('.score .bar b') as HTMLElement;
    expect(barOver.style.width).toBe('100%');
  });

  it('unranked mode shows an em-dash and no bar', async () => {
    const mem = create(MemorySchema, { id: '3', summary: 'x', category: 'convention', score: 0.5 });
    const screen = await render(ResultRow, { memory: mem, mode: 'unranked' });
    await expect.element(screen.container.querySelector('.score .num') as HTMLElement).toHaveTextContent('—');
    expect(screen.container.querySelector('.score .bar')).toBeNull();
  });

  it('showRel renders rel to 2dp including 0, and nothing when relevance is unset', async () => {
    const withRel = create(MemorySchema, { id: '4', summary: 'x', category: 'convention', score: 0.5, relevance: 0.834 });
    const screen = await render(ResultRow, { memory: withRel, mode: 'ranked', showRel: true });
    await expect.element(screen.container.querySelector('.rel') as HTMLElement).toHaveTextContent('rel 0.83');

    const relZero = create(MemorySchema, { id: '5', summary: 'x', category: 'convention', score: 0.5, relevance: 0 });
    await screen.rerender({ memory: relZero, mode: 'ranked', showRel: true });
    await expect.element(screen.container.querySelector('.rel') as HTMLElement).toHaveTextContent('rel 0.00');

    const noRel = create(MemorySchema, { id: '6', summary: 'x', category: 'convention', score: 0.5 });
    await screen.rerender({ memory: noRel, mode: 'ranked', showRel: true });
    await expect.element(screen.container.querySelector('.rel') as HTMLElement).toHaveTextContent('');
  });

  it('archived-only (one word) fits inside the chip budget uncollapsed', async () => {
    const mem = create(MemorySchema, { id: '7a', summary: 'x', category: 'convention', archivedAt: timestampFromDate(now) });
    const screen = await render(ResultRow, { memory: mem });
    const statesEl = screen.container.querySelector('.states') as HTMLElement;
    await expect.element(statesEl).not.toHaveClass('collapsed');
    const chips = Array.from(statesEl.querySelectorAll('.st')).map((el) => el.textContent);
    expect(chips).toEqual(['archived']);
  });

  it('archived+superseded overflows the bounded chip budget, collapsing to "first +N" with a full title in canonical order', async () => {
    const mem = create(MemorySchema, {
      id: '7',
      summary: 'x',
      category: 'convention',
      archivedAt: timestampFromDate(now),
      supersededBy: 'successor'
    });
    const screen = await render(ResultRow, { memory: mem });
    const statesEl = screen.container.querySelector('.states') as HTMLElement;
    await expect.element(statesEl).toHaveClass('collapsed');
    const chips = Array.from(statesEl.querySelectorAll('.st')).map((el) => el.textContent);
    expect(chips).toEqual(['archived', '+1']);
    expect(statesEl.getAttribute('title')).toBe('archived · superseded');
  });

  it('archived+superseded+scheduled collapses the same way, title lists all three in canonical order', async () => {
    const mem = create(MemorySchema, {
      id: '7b',
      summary: 'x',
      category: 'convention',
      archivedAt: timestampFromDate(now),
      supersededBy: 'successor',
      notBefore: future
    });
    const screen = await render(ResultRow, { memory: mem });
    const statesEl = screen.container.querySelector('.states') as HTMLElement;
    await expect.element(statesEl).toHaveClass('collapsed');
    const chips = Array.from(statesEl.querySelectorAll('.st')).map((el) => el.textContent);
    expect(chips).toEqual(['archived', '+2']);
    expect(statesEl.getAttribute('title')).toBe('archived · superseded · scheduled');
  });

  it('an archived record dims the summary; a scheduled-only record does not', async () => {
    const archivedMem = create(MemorySchema, {
      id: '8',
      summary: 'archived record summary',
      category: 'convention',
      archivedAt: timestampFromDate(now)
    });
    const screen = await render(ResultRow, { memory: archivedMem });
    await expect.element(screen.container.querySelector('.sum') as HTMLElement).toHaveClass('dim');

    const scheduledMem = create(MemorySchema, {
      id: '9',
      summary: 'scheduled record summary',
      category: 'convention',
      notBefore: future
    });
    await screen.rerender({ memory: scheduledMem });
    await expect.element(screen.container.querySelector('.sum') as HTMLElement).not.toHaveClass('dim');
  });

  it('three tags show the first two and a +1 overflow marker', async () => {
    const mem = create(MemorySchema, { id: '10', summary: 'x', category: 'convention', tags: ['a', 'b', 'c'] });
    const screen = await render(ResultRow, { memory: mem });
    const tagEls = Array.from(screen.container.querySelectorAll('.tags .tag'));
    expect(tagEls.map((el) => el.textContent)).toEqual(['a', 'b', '+1']);
  });

  it('a backtick span in the summary renders as a <code> text node, never {@html}', async () => {
    const mem = create(MemorySchema, {
      id: '11',
      summary: 'gofmt drift is NOT caught by `golangci-lint`',
      category: 'gotcha'
    });
    const screen = await render(ResultRow, { memory: mem });
    const code = screen.container.querySelector('.sum code');
    expect(code?.textContent).toBe('golangci-lint');
  });

  it('a 400-character summary keeps the same row height as a short one', async () => {
    const shortMem = create(MemorySchema, { id: '12', summary: 'short', category: 'convention' });
    const screen = await render(ResultRow, { memory: shortMem });
    const shortHeight = (screen.container.querySelector('.result-row-line') as HTMLElement).getBoundingClientRect()
      .height;

    const longMem = create(MemorySchema, { id: '13', summary: 'x'.repeat(400), category: 'convention' });
    await screen.rerender({ memory: longMem });
    const longHeight = (screen.container.querySelector('.result-row-line') as HTMLElement).getBoundingClientRect()
      .height;
    expect(longHeight).toBe(shortHeight);
  });

  // UI-REVIEW BLOCKER: the column drop-outs must shrink the grid, not just
  // hide cells — at every list width the summary stays readable and the
  // score / rel / state chips stay fully inside the row (no horizontal
  // overflow). Worst case: rel on, a state chip, tags and a scope.
  describe.each([
    { font: '15px', label: 'default text size' },
    { font: '16px', label: 'largest text size' }
  ])('column drop-out by list width ($label)', ({ font }) => {
    const nextFrame = () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));

    it.each([1000, 861, 800, 561, 520, 390])('at a %ipx list the summary keeps usable width and score/rel/states fit', async (width) => {
      document.documentElement.style.setProperty('--ui-font', font);
      try {
        const mem = create(MemorySchema, {
          id: `w-${width}`,
          summary: 'release-please drives SemVer tags; the milestone label is CalVer and never feeds it',
          category: 'convention',
          tags: ['release', 'ops', 'config'],
          scope: 'repo:seanb4t/engram',
          score: 0.81,
          relevance: 0.77,
          supersededBy: 'successor',
          createdAt: timestampFromDate(now)
        });
        const screen = await render(ResultRow, { memory: mem, mode: 'ranked', showRel: true });
        Object.assign(screen.container.style, { width: `${width}px`, containerType: 'inline-size', containerName: 'list' });
        await nextFrame();

        const q = (sel: string) => screen.container.querySelector(sel) as HTMLElement;
        const row = q('.result-row-line');
        const rowBox = row.getBoundingClientRect();
        const inside = (el: HTMLElement) => {
          const b = el.getBoundingClientRect();
          return b.width > 0 && b.left >= rowBox.left - 0.5 && b.right <= rowBox.right + 0.5;
        };

        expect(row.scrollWidth).toBeLessThanOrEqual(row.clientWidth + 1);
        expect(q('.sum').getBoundingClientRect().width).toBeGreaterThanOrEqual(width >= 800 ? 130 : 70);
        expect(inside(q('.score'))).toBe(true);
        expect(inside(q('.score .num'))).toBe(true);
        expect(inside(q('.rel'))).toBe(true);
        expect(inside(q('.states .st'))).toBe(true);
        expect(inside(q('.age'))).toBe(true);
      } finally {
        document.documentElement.style.removeProperty('--ui-font');
      }
    });
  });

  it("category 'gotcha' shows the word colored by --cat-gotcha", async () => {
    const mem = create(MemorySchema, { id: '14', summary: 'x', category: 'gotcha' });
    const screen = await render(ResultRow, { memory: mem });
    await expect.element(screen.container.querySelector('.cat-word') as HTMLElement).toHaveTextContent('gotcha');
    const catEl = screen.container.querySelector('.result-row-line') as HTMLElement;
    expect(catEl.style.getPropertyValue('--c')).toBe('var(--cat-gotcha)');
  });

  describe('trailing (D-11)', () => {
    function trailingSnippet() {
      return createRawSnippet((getMemory: () => Memory) => ({
        render: () => `<span data-testid="trail-content">trail:${getMemory().id}</span>`
      }));
    }

    it('a trailing snippet replaces the score and rel cells with its own output', async () => {
      const mem = create(MemorySchema, { id: 'tr1', summary: 'x', category: 'convention', score: 0.5, relevance: 0.4 });
      const screen = await render(ResultRow, {
        memory: mem,
        mode: 'ranked',
        showRel: true,
        trailing: trailingSnippet()
      });
      expect(screen.container.querySelector('.score')).toBeNull();
      expect(screen.container.querySelector('.rel')).toBeNull();
      await expect
        .element(screen.container.querySelector('[data-testid="trail-content"]') as HTMLElement)
        .toHaveTextContent('trail:tr1');
    });

    it('the grid track count with a trailing snippet equals the no-rel count at every container width', async () => {
      const nextFrame = () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
      const mem = create(MemorySchema, { id: 'tr2', summary: 'x', category: 'convention' });

      const plain = await render(ResultRow, { memory: mem, mode: 'ranked', showRel: true });
      const withTrailing = await render(ResultRow, {
        memory: mem,
        mode: 'ranked',
        showRel: true,
        trailing: trailingSnippet()
      });

      for (const width of [1000, 800, 390]) {
        Object.assign(plain.container.style, { width: `${width}px`, containerType: 'inline-size', containerName: 'list' });
        Object.assign(withTrailing.container.style, {
          width: `${width}px`,
          containerType: 'inline-size',
          containerName: 'list'
        });
        await nextFrame();
        const plainRow = plain.container.querySelector('.result-row-line') as HTMLElement;
        const trailRow = withTrailing.container.querySelector('.result-row-line') as HTMLElement;
        const trackCount = (el: HTMLElement) => getComputedStyle(el).gridTemplateColumns.trim().split(/\s+/).length;
        // `plain` has showRel=true (one extra track over the no-rel count);
        // the trailing version must match the NO-REL count -- one fewer.
        expect(trackCount(trailRow)).toBe(trackCount(plainRow) - 1);
      }
    });
  });

  describe('selection (D-02, D-10)', () => {
    const nextFrame = () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));

    it.each([1000, 800, 500])(
      'selectable adds exactly one grid-template-columns track at a %ipx list width, and the check element is aria-hidden',
      async (width) => {
        const mem = create(MemorySchema, { id: `sel-${width}`, summary: 'x', category: 'convention' });

        const plain = await render(ResultRow, { memory: mem });
        Object.assign(plain.container.style, { width: `${width}px`, containerType: 'inline-size', containerName: 'list' });

        const selectableScreen = await render(ResultRow, { memory: mem, selectable: true });
        Object.assign(selectableScreen.container.style, {
          width: `${width}px`,
          containerType: 'inline-size',
          containerName: 'list'
        });
        await nextFrame();

        const plainRow = plain.container.querySelector('.result-row-line') as HTMLElement;
        const selRow = selectableScreen.container.querySelector('.result-row-line') as HTMLElement;
        const trackCount = (el: HTMLElement) => getComputedStyle(el).gridTemplateColumns.trim().split(/\s+/).length;
        expect(trackCount(selRow)).toBe(trackCount(plainRow) + 1);

        const check = selectableScreen.container.querySelector('.row-check');
        expect(check).toBeTruthy();
        expect(check?.getAttribute('aria-hidden')).toBe('true');
      }
    );

    it('the check element is not rendered when selectable is false', async () => {
      const mem = create(MemorySchema, { id: 'nosel', summary: 'x', category: 'convention' });
      const screen = await render(ResultRow, { memory: mem });
      expect(screen.container.querySelector('.row-check')).toBeNull();
    });
  });

  describe('flash (D-10)', () => {
    afterEach(() => {
      flashing.clear();
    });

    it('a row whose id is in the flashing set gets the flash class, which clears after FLASH_MS', async () => {
      vi.useFakeTimers();
      try {
        const mem = create(MemorySchema, { id: 'flash-1', summary: 'x', category: 'convention' });
        flashRows([mem.id]);
        const screen = await render(ResultRow, { memory: mem });
        const row = screen.container.querySelector('.result-row-line') as HTMLElement;
        expect(row.classList.contains('flash')).toBe(true);

        await vi.advanceTimersByTimeAsync(FLASH_MS);
        expect(row.classList.contains('flash')).toBe(false);
      } finally {
        vi.useRealTimers();
      }
    });

    it('a row whose id is NOT in the flashing set never gets the flash class', async () => {
      const mem = create(MemorySchema, { id: 'no-flash', summary: 'x', category: 'convention' });
      const screen = await render(ResultRow, { memory: mem });
      const row = screen.container.querySelector('.result-row-line') as HTMLElement;
      expect(row.classList.contains('flash')).toBe(false);
    });
  });
});
