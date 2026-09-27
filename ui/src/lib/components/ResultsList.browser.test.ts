// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page, userEvent } from 'vitest/browser';
import { describe, it, expect, vi } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import ResultsList from './ResultsList.svelte';

function makeMemories(count: number): Memory[] {
  return Array.from({ length: count }, (_, i) =>
    create(MemorySchema, {
      id: `m${String(i).padStart(4, '0')}`,
      category: 'convention',
      summary: `summary ${i}`,
      content: '',
      tags: [],
      scope: 'repo:test',
      createdAt: timestampFromDate(new Date()),
      visibility: 'private',
      owner: 'me',
      score: 0.5,
      shortId: `s${String(i).padStart(9, '0')}`
    })
  );
}

const THOUSAND = makeMemories(1000);

const HOVER_CARD_SELECTOR = '[data-slot="hover-card-content"]';

function fireKey(el: Element, key: string, opts: Partial<KeyboardEventInit> = {}) {
  el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...opts }));
}

describe('ResultsList', () => {
  it('renders a real WAI-ARIA listbox: one focusable option-bearing container, no role=region left', async () => {
    const onopen = vi.fn();
    const screen = await render(ResultsList, { memories: THOUSAND, label: 'Search results', onopen });

    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    await expect.element(listbox).toBeInTheDocument();

    // Not necessarily on first synchronous render (the wrapper may retry via
    // requestAnimationFrame) — expect.element/getByRole locators retry until
    // the assertion holds, closing that gap.
    const regions = screen.container.querySelectorAll('[role="region"]');
    expect(regions.length).toBe(0);

    const options = screen.container.querySelectorAll('[role="option"]');
    expect(options.length).toBeGreaterThan(0);
    expect(options.length).toBeLessThan(1000);
  });

  it('End/Home/j/k traverse aria-activedescendant, clamped at the ends; Enter opens the active row without moving focus', async () => {
    const onopen = vi.fn();
    const screen = await render(ResultsList, { memories: THOUSAND, label: 'Search results', onopen });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });

    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    await userEvent.keyboard('{End}');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0999');
    expect(screen.container.querySelector('#opt-m0999')).toBeTruthy();

    await userEvent.keyboard('{Home}');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    await userEvent.keyboard('j');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0001');

    await userEvent.keyboard('k');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    // k on the first row: no wrap, stays put.
    await userEvent.keyboard('k');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    await userEvent.keyboard('{Enter}');
    expect(onopen).toHaveBeenCalledWith('m0000');
    expect(document.activeElement).toBe(listbox.element());
  });

  // WR-01: applyRole() rewrites role="region" to role="listbox", which
  // un-matches the vendored library's own `[role='region']:focus-visible`
  // rule — a compensating rule for the rewritten role must restore the ring.
  it('the rewritten listbox keeps a visible :focus-visible ring', async () => {
    const onopen = vi.fn();
    const three = makeMemories(3);
    const screen = await render(ResultsList, { memories: three, label: 'Search results', onopen });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    const style = getComputedStyle(listbox.element());
    expect(style.outlineStyle).toBe('solid');
    expect(parseFloat(style.outlineWidth)).toBeGreaterThan(0);
  });

  it('re-rendering with a different array keeps role listbox and resets the active row to the first item', async () => {
    const onopen = vi.fn();
    const screen = await render(ResultsList, { memories: THOUSAND, label: 'Search results', onopen });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await userEvent.keyboard('{End}');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0999');

    const three = makeMemories(3);
    await screen.rerender({ memories: three, label: 'Search results', onopen });

    await expect.element(screen.getByRole('listbox', { name: 'Search results' })).toBeInTheDocument();
    await expect
      .element(screen.getByRole('listbox', { name: 'Search results' }))
      .toHaveAttribute('aria-activedescendant', 'opt-m0000');
  });

  it('zero memories renders no listbox', async () => {
    const onopen = vi.fn();
    const screen = await render(ResultsList, { memories: [], label: 'Search results', onopen });
    await expect.element(screen.getByRole('listbox', { name: 'Search results' })).not.toBeInTheDocument();
  });

  it('loading with no memories renders skeleton rows and no listbox', async () => {
    const onopen = vi.fn();
    const screen = await render(ResultsList, { memories: [], label: 'Search results', onopen, loading: true });
    await expect.element(screen.getByTestId('results-loading')).toBeInTheDocument();
    await expect.element(screen.getByRole('listbox', { name: 'Search results' })).not.toBeInTheDocument();
  });

  it('busy with memories keeps the rows rendered, dims them, and shows a progress bar', async () => {
    const onopen = vi.fn();
    const three = makeMemories(3);
    const screen = await render(ResultsList, { memories: three, label: 'Search results', onopen, busy: true });
    await expect.element(screen.getByRole('listbox', { name: 'Search results' })).toBeInTheDocument();
    await expect.element(screen.getByTestId('results-loadbar')).toBeInTheDocument();
    const wrapper = screen.container.querySelector('.results-listbox-wrapper');
    expect(wrapper?.classList.contains('busy')).toBe(true);
  });

  it('rel column appears list-wide only when at least one hit carries relevance', async () => {
    const onopen = vi.fn();
    const noRel = makeMemories(3);
    const screen = await render(ResultsList, { memories: noRel, label: 'Search results', onopen });
    expect(screen.container.querySelectorAll('.rel').length).toBe(0);

    const withRel = makeMemories(3);
    withRel[0] = create(MemorySchema, {
      id: withRel[0].id,
      category: withRel[0].category,
      summary: withRel[0].summary,
      scope: withRel[0].scope,
      relevance: 0.5
    });
    await screen.rerender({ memories: withRel, label: 'Search results', onopen });
    const relCols = screen.container.querySelectorAll('.rel');
    expect(relCols.length).toBe(withRel.length);
  });

  it('pointer hover (mousemove) opens the card after ~250ms without moving aria-activedescendant', async () => {
    const onopen = vi.fn();
    const five = makeMemories(5);
    const screen = await render(ResultsList, { memories: five, label: 'Search results', onopen });
    // Isolated mounts have no ancestor providing a real height (only
    // +layout.svelte's flex chain does that), so height:100% collapses to 0
    // and the legend line visually overlaps the (zero-height) list, blocking
    // Playwright's real pointer hover. Give the mount a real box, matching
    // what the shipped app's flex chain provides.
    screen.container.style.height = '600px';
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    const row3 = screen.container.querySelector('#opt-m0003') as HTMLElement;
    await page.elementLocator(row3).hover();

    // Not yet — well before the 250ms open delay.
    await new Promise((r) => setTimeout(r, 100));
    expect(document.querySelector(HOVER_CARD_SELECTOR)).toBeNull();

    await expect
      .poll(() => document.querySelector(HOVER_CARD_SELECTOR) !== null, { timeout: 800 })
      .toBe(true);
    // The hover card never mutates keyboard selection state (Pitfall 4).
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');
  });

  it('keyboard movement opens the card instantly for the new active row; document focus stays on the listbox', async () => {
    const onopen = vi.fn();
    const five = makeMemories(5);
    const screen = await render(ResultsList, { memories: five, label: 'Search results', onopen });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();

    await userEvent.keyboard('j');
    await expect.poll(() => document.querySelector(HOVER_CARD_SELECTOR) !== null).toBe(true);
    expect(document.activeElement).toBe(listbox.element());
  });

  it('the hover card shows short_id, full scope, every state word and every tag', async () => {
    const onopen = vi.fn();
    const mem = create(MemorySchema, {
      id: 'x1',
      category: 'convention',
      summary: 'a record with state and tags',
      scope: 'repo:acme/engram',
      shortId: 'abcdefghij',
      tags: ['alpha', 'beta', 'gamma'],
      archivedAt: timestampFromDate(new Date('2030-01-01T00:00:00Z')),
      supersededBy: 'succ'
    });
    const screen = await render(ResultsList, { memories: [mem], label: 'Search results', onopen });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await userEvent.keyboard('{End}'); // keyboard-follow: opens the card instantly, even for a single-item list

    await expect.poll(() => document.querySelector(HOVER_CARD_SELECTOR) !== null).toBe(true);
    const card = document.querySelector(HOVER_CARD_SELECTOR) as HTMLElement;
    expect(card.textContent).toContain('abcdefghij');
    expect(card.textContent).toContain('repo:acme/engram');
    expect(card.textContent).toContain('archived');
    expect(card.textContent).toContain('superseded');
    for (const tag of ['alpha', 'beta', 'gamma']) {
      expect(card.textContent).toContain(tag);
    }
  });

  it('no card shows for the row already open in the pane', async () => {
    const onopen = vi.fn();
    const mem = create(MemorySchema, { id: 'x2', category: 'convention', summary: 'x', scope: 's', shortId: 'sid0000001' });
    const screen = await render(ResultsList, { memories: [mem], label: 'Search results', onopen, openId: 'x2' });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await userEvent.keyboard('{End}');
    expect(document.querySelector(HOVER_CARD_SELECTOR)).toBeNull();
  });

  it('Esc closes an open card without calling onescape; a second Esc calls onescape', async () => {
    const onopen = vi.fn();
    const onescape = vi.fn();
    const three = makeMemories(3);
    const screen = await render(ResultsList, { memories: three, label: 'Search results', onopen, onescape });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await userEvent.keyboard('{End}');
    await expect.poll(() => document.querySelector(HOVER_CARD_SELECTOR) !== null).toBe(true);

    await userEvent.keyboard('{Escape}');
    expect(onescape).not.toHaveBeenCalled();
    await expect.poll(() => document.querySelector(HOVER_CARD_SELECTOR) === null).toBe(true);

    await userEvent.keyboard('{Escape}');
    expect(onescape).toHaveBeenCalledTimes(1);
  });

  it('row-action keys call the matching host callback on the active row', async () => {
    const onopen = vi.fn();
    const onedit = vi.fn();
    const onvisibility = vi.fn();
    const ondelete = vi.fn();
    const mem = create(MemorySchema, { id: 'y1', category: 'convention', summary: 'x', scope: 's', shortId: 'sid0000002' });
    const screen = await render(ResultsList, {
      memories: [mem],
      label: 'Search results',
      onopen,
      onedit,
      onvisibility,
      ondelete
    });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();

    await userEvent.keyboard('e');
    expect(onedit).toHaveBeenCalledWith('y1');

    await userEvent.keyboard('s');
    expect(onvisibility).toHaveBeenCalledWith(mem);

    await userEvent.keyboard('#');
    expect(ondelete).toHaveBeenCalledWith('y1');
    // '#' only ever calls the host's request — never an RPC of its own.
  });

  it('c copies the short_id and Shift+C copies the full id, both via the clipboard', async () => {
    const onopen = vi.fn();
    const mem = create(MemorySchema, { id: 'z1', category: 'convention', summary: 'x', scope: 's', shortId: 'sid0000003' });
    const screen = await render(ResultsList, { memories: [mem], label: 'Search results', onopen });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    const writeSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);

    fireKey(listbox.element(), 'c');
    await expect.poll(() => writeSpy.mock.calls.length).toBe(1);
    expect(writeSpy).toHaveBeenLastCalledWith('sid0000003');

    fireKey(listbox.element(), 'C', { shiftKey: true });
    await expect.poll(() => writeSpy.mock.calls.length).toBe(2);
    expect(writeSpy).toHaveBeenLastCalledWith('z1');

    // Pressing c twice copies the same short_id twice (idempotent).
    fireKey(listbox.element(), 'c');
    await expect.poll(() => writeSpy.mock.calls.length).toBe(3);
    expect(writeSpy).toHaveBeenLastCalledWith('sid0000003');

    writeSpy.mockRestore();
  });

  it('Meta+c and a row key typed into a focused input inside the wrapper do nothing', async () => {
    const onopen = vi.fn();
    const mem = create(MemorySchema, { id: 'w1', category: 'convention', summary: 'x', scope: 's', shortId: 'sid0000004' });
    const screen = await render(ResultsList, { memories: [mem], label: 'Search results', onopen });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    const writeSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);

    fireKey(listbox.element(), 'c', { metaKey: true });
    expect(writeSpy).not.toHaveBeenCalled();

    const wrapper = screen.container.querySelector('.results-listbox-wrapper') as HTMLElement;
    const input = document.createElement('input');
    wrapper.appendChild(input);
    input.focus();
    fireKey(input, 'c');
    expect(writeSpy).not.toHaveBeenCalled();

    input.remove();
    writeSpy.mockRestore();
  });

  it('e/s do nothing for a rule record; e also does nothing for a discovery record', async () => {
    const onopen = vi.fn();
    const onedit = vi.fn();
    const onvisibility = vi.fn();
    const rule = create(MemorySchema, { id: 'r1', category: 'rule', summary: 'x', scope: 's' });
    const screen = await render(ResultsList, { memories: [rule], label: 'Search results', onopen, onedit, onvisibility });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();

    await userEvent.keyboard('e');
    expect(onedit).not.toHaveBeenCalled();
    await userEvent.keyboard('s');
    expect(onvisibility).not.toHaveBeenCalled();

    const discovery = create(MemorySchema, { id: 'd1', category: 'discovery', summary: 'x', scope: 's' });
    await screen.rerender({ memories: [discovery], label: 'Search results', onopen, onedit, onvisibility });
    await userEvent.keyboard('e');
    expect(onedit).not.toHaveBeenCalled();
  });

  it('the legend shows Kbd hints for j, k, enter, esc, e, s, #, c and shift-C', async () => {
    const onopen = vi.fn();
    const three = makeMemories(3);
    const screen = await render(ResultsList, { memories: three, label: 'Search results', onopen });
    const legend = screen.container.querySelector('.results-legend') as HTMLElement;
    const kbdTexts = Array.from(legend.querySelectorAll('kbd')).map((el) => el.textContent);
    expect(kbdTexts).toEqual(['j', 'k', '↵', 'esc', 'e', 's', '#', 'c', '⇧C']);
  });
});

describe('ResultsList — multi-select (D-01, D-02, D-03)', () => {
  it('x on row 1 sets the anchor; moving to row 4 and pressing Shift+X selects the inclusive range 1-4', async () => {
    const onopen = vi.fn();
    const five = makeMemories(5);
    const screen = await render(ResultsList, { memories: five, label: 'Search results', onopen, selectable: true });
    screen.container.style.height = '600px';
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');

    await userEvent.keyboard('x'); // anchor = row 1 (m0000)
    await userEvent.keyboard('j');
    await userEvent.keyboard('j');
    await userEvent.keyboard('j');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0003'); // row 4

    fireKey(listbox.element(), 'X', { shiftKey: true });

    const selected = Array.from(screen.container.querySelectorAll('[role="option"][aria-selected="true"]')).map(
      (el) => el.id
    );
    expect(selected).toEqual(['opt-m0000', 'opt-m0001', 'opt-m0002', 'opt-m0003']);
  });

  it('shift+click on a later row extends the range from the same anchor, without opening the pane', async () => {
    const onopen = vi.fn();
    const six = makeMemories(6);
    const screen = await render(ResultsList, { memories: six, label: 'Search results', onopen, selectable: true });
    screen.container.style.height = '600px';
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();

    await userEvent.keyboard('x'); // anchor = row 1 (m0000)
    await userEvent.keyboard('j');
    await userEvent.keyboard('j');
    await userEvent.keyboard('j');
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0003');
    fireKey(listbox.element(), 'X', { shiftKey: true }); // selects rows 1-4

    const row6 = screen.container.querySelector('#opt-m0005') as HTMLElement;
    row6.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, shiftKey: true }));

    const selected = Array.from(screen.container.querySelectorAll('[role="option"][aria-selected="true"]')).map(
      (el) => el.id
    );
    expect(selected).toEqual(['opt-m0000', 'opt-m0001', 'opt-m0002', 'opt-m0003', 'opt-m0004', 'opt-m0005']);
    expect(onopen).not.toHaveBeenCalled();
  });

  it('a click on the row-check cell toggles selection and does not open the pane', async () => {
    const onopen = vi.fn();
    const three = makeMemories(3);
    const screen = await render(ResultsList, { memories: three, label: 'Search results', onopen, selectable: true });
    screen.container.style.height = '600px';

    const check = screen.container.querySelector('#opt-m0000 .row-check') as HTMLElement;
    check.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));

    expect(screen.container.querySelector('#opt-m0000')?.getAttribute('aria-selected')).toBe('true');
    expect(onopen).not.toHaveBeenCalled();
  });

  it('Escape clears a non-empty selection first, leaves the card/pane state untouched; a second Escape closes the card; a third calls onescape', async () => {
    const onopen = vi.fn();
    const onescape = vi.fn();
    const three = makeMemories(3);
    const screen = await render(ResultsList, {
      memories: three,
      label: 'Search results',
      onopen,
      onescape,
      selectable: true
    });
    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await userEvent.keyboard('x');
    await userEvent.keyboard('{End}');
    await expect.poll(() => document.querySelector(HOVER_CARD_SELECTOR) !== null).toBe(true);
    expect(screen.container.querySelector('[role="option"][aria-selected="true"]')).toBeTruthy();

    await userEvent.keyboard('{Escape}');
    expect(screen.container.querySelector('[role="option"][aria-selected="true"]')).toBeNull();
    expect(document.querySelector(HOVER_CARD_SELECTOR)).not.toBeNull();
    expect(onescape).not.toHaveBeenCalled();

    await userEvent.keyboard('{Escape}');
    await expect.poll(() => document.querySelector(HOVER_CARD_SELECTOR) === null).toBe(true);
    expect(onescape).not.toHaveBeenCalled();

    await userEvent.keyboard('{Escape}');
    expect(onescape).toHaveBeenCalledTimes(1);
  });

  it("typing 'a' or 'x' into an input inside the list container fires nothing", async () => {
    const onopen = vi.fn();
    const onarchive = vi.fn();
    const mem = create(MemorySchema, { id: 'ty1', category: 'convention', summary: 'x', scope: 's', shortId: 'sid0000005' });
    const screen = await render(ResultsList, {
      memories: [mem],
      label: 'Search results',
      onopen,
      selectable: true,
      onarchive
    });
    const wrapper = screen.container.querySelector('.results-listbox-wrapper') as HTMLElement;
    const input = document.createElement('input');
    wrapper.appendChild(input);
    input.focus();

    fireKey(input, 'x');
    fireKey(input, 'a');

    expect(onarchive).not.toHaveBeenCalled();
    expect(screen.container.querySelector('[role="option"]')?.getAttribute('aria-selected')).toBe('false');
    input.remove();
  });

  it('the legend shows select/range/curation Kbd hints only for the callbacks supplied', async () => {
    const onopen = vi.fn();
    const three = makeMemories(3);
    const onarchive = vi.fn();
    const onrestore = vi.fn();
    const onsupersede = vi.fn();
    const screen = await render(ResultsList, {
      memories: three,
      label: 'Search results',
      onopen,
      selectable: true,
      onarchive,
      onrestore,
      onsupersede
    });
    const legend = screen.container.querySelector('.results-legend') as HTMLElement;
    const kbdTexts = Array.from(legend.querySelectorAll('kbd')).map((el) => el.textContent);
    expect(kbdTexts).toEqual(['j', 'k', '↵', 'esc', 'e', 's', '#', 'c', '⇧C', 'x', '⇧X', '⇧S', 'a', '⇧A']);
  });

  it('with only onarchive supplied, the legend shows select/range/archive hints but not restore or supersede', async () => {
    const onopen = vi.fn();
    const three = makeMemories(3);
    const onarchive = vi.fn();
    const screen = await render(ResultsList, {
      memories: three,
      label: 'Search results',
      onopen,
      selectable: true,
      onarchive
    });
    const legend = screen.container.querySelector('.results-legend') as HTMLElement;
    const kbdTexts = Array.from(legend.querySelectorAll('kbd')).map((el) => el.textContent);
    expect(kbdTexts).toEqual(['j', 'k', '↵', 'esc', 'e', 's', '#', 'c', '⇧C', 'x', '⇧X', 'a']);
  });

  it('without selectable, neither the check column nor the selection hints render, and aria-selected follows the active row', async () => {
    const onopen = vi.fn();
    const three = makeMemories(3);
    const screen = await render(ResultsList, { memories: three, label: 'Search results', onopen });
    const legend = screen.container.querySelector('.results-legend') as HTMLElement;
    const kbdTexts = Array.from(legend.querySelectorAll('kbd')).map((el) => el.textContent);
    expect(kbdTexts).toEqual(['j', 'k', '↵', 'esc', 'e', 's', '#', 'c', '⇧C']);
    expect(screen.container.querySelector('.row-check')).toBeNull();

    const listbox = screen.getByRole('listbox', { name: 'Search results' });
    listbox.element().focus();
    await expect.element(listbox).toHaveAttribute('aria-activedescendant', 'opt-m0000');
    expect(screen.container.querySelector('#opt-m0000')?.getAttribute('aria-selected')).toBe('true');
    expect(screen.container.querySelector('#opt-m0001')?.getAttribute('aria-selected')).toBe('false');
  });
});
