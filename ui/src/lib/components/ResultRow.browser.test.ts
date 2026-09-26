// app.css is not pulled in by isolated component mounts (only
// +layout.svelte imports it) — without it every calc(N*var(--u)) product
// dimension in ResultRow (row height, the state-chip collapse budget) is an
// INVALID CSS value in this test's cascade and silently falls back to its
// default, making a geometry assertion pass for the wrong reason. Import it
// explicitly so --u and the design tokens are real here, same as the shipped app.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { MemorySchema } from '$lib/gen/engram_pb';
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

  it("category 'gotcha' shows the word colored by --cat-gotcha", async () => {
    const mem = create(MemorySchema, { id: '14', summary: 'x', category: 'gotcha' });
    const screen = await render(ResultRow, { memory: mem });
    await expect.element(screen.container.querySelector('.cat-word') as HTMLElement).toHaveTextContent('gotcha');
    const catEl = screen.container.querySelector('.result-row-line') as HTMLElement;
    expect(catEl.style.getPropertyValue('--c')).toBe('var(--cat-gotcha)');
  });
});
