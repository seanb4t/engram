import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi } from 'vitest';
import { createRawSnippet } from 'svelte';
import RecallSplit from './RecallSplit.svelte';

function fakeListSnippet() {
  return createRawSnippet(() => ({
    render: () => `<div data-testid="fake-list" style="height:100%">LIST</div>`
  }));
}

function fakeDetailSnippet(onCloseSpy: () => void) {
  return createRawSnippet(() => ({
    render: () => `<div data-testid="fake-detail" style="height:100%">
      <button type="button" data-testid="fake-detail-close">fake pane internal close</button>
      DETAIL CONTENT
    </div>`,
    setup: (node: Element) => {
      const btn = node.querySelector('[data-testid="fake-detail-close"]');
      btn?.addEventListener('click', onCloseSpy);
    }
  }));
}

function pressKey(el: HTMLElement, key: string) {
  el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
}

async function waitForWidth(getEl: () => Element | null, predicate: (w: number) => boolean, timeoutMs = 2000): Promise<number> {
  const start = Date.now();
  // eslint-disable-next-line no-constant-condition
  while (true) {
    const el = getEl();
    if (el) {
      const w = el.getBoundingClientRect().width;
      if (predicate(w)) return w;
    }
    if (Date.now() - start > timeoutMs) {
      const el2 = getEl();
      throw new Error(`timed out waiting for width predicate; last width=${el2 ? el2.getBoundingClientRect().width : 'no element'}`);
    }
    await new Promise((r) => setTimeout(r, 20));
  }
}

describe('RecallSplit', () => {
  it('at 1200px, open=false collapses the detail pane and the list takes full width', async () => {
    const onclose = vi.fn();
    const screen = await render(RecallSplit, {
      open: false,
      onclose,
      autoSaveId: 'test-recall-split-1',
      list: fakeListSnippet(),
      detail: fakeDetailSnippet(onclose)
    });
    screen.container.style.width = '1200px';
    screen.container.style.height = '400px';

    const detailPane = () => screen.container.querySelector('.rs-detail-pane');
    await waitForWidth(detailPane, (w) => w < 10);

    const listPane = screen.container.querySelector('.rs-list-pane');
    expect(listPane).not.toBeNull();
    const listWidth = listPane!.getBoundingClientRect().width;
    expect(listWidth).toBeGreaterThan(1100);
  });

  it('at 1200px, open=true shows the detail pane at about 440px', async () => {
    const onclose = vi.fn();
    const screen = await render(RecallSplit, {
      open: true,
      onclose,
      autoSaveId: 'test-recall-split-2',
      list: fakeListSnippet(),
      detail: fakeDetailSnippet(onclose)
    });
    screen.container.style.width = '1200px';
    screen.container.style.height = '400px';

    const detailPane = () => screen.container.querySelector('.rs-detail-pane');
    const w = await waitForWidth(detailPane, (width) => width > 50);
    expect(w).toBeGreaterThan(440 - 30);
    expect(w).toBeLessThan(440 + 30);
  });

  it('the resize handle widens the detail pane by ~32px on ArrowLeft, narrows on ArrowRight, and clamps to [280px, 72%]', async () => {
    const onclose = vi.fn();
    const screen = await render(RecallSplit, {
      open: true,
      onclose,
      autoSaveId: 'test-recall-split-3',
      list: fakeListSnippet(),
      detail: fakeDetailSnippet(onclose)
    });
    screen.container.style.width = '1200px';
    screen.container.style.height = '400px';

    const detailPane = () => screen.container.querySelector('.rs-detail-pane') as HTMLElement;
    const startWidth = await waitForWidth(detailPane, (w) => w > 50);

    const handleEl = screen.container.querySelector('[role="separator"]') as HTMLElement;
    expect(handleEl).not.toBeNull();
    handleEl.focus();
    pressKey(handleEl, 'ArrowLeft');

    const afterLeft = await waitForWidth(detailPane, (w) => Math.abs(w - startWidth) > 5);
    expect(afterLeft).toBeGreaterThan(startWidth);
    expect(afterLeft - startWidth).toBeGreaterThan(32 - 6);
    expect(afterLeft - startWidth).toBeLessThan(32 + 6);

    pressKey(handleEl, 'ArrowRight');
    const afterRight = await waitForWidth(detailPane, (w) => Math.abs(w - afterLeft) > 5);
    expect(afterLeft - afterRight).toBeGreaterThan(32 - 6);
    expect(afterLeft - afterRight).toBeLessThan(32 + 6);

    // Clamp floor: hammer ArrowRight (each press yields to the microtask
    // queue so paneforge's reactive resize actually applies) until it stops
    // shrinking at the 280px floor.
    for (let i = 0; i < 30; i++) {
      pressKey(handleEl, 'ArrowRight');
      await new Promise((r) => setTimeout(r, 20));
    }
    const floor = detailPane()!.getBoundingClientRect().width;
    expect(floor).toBeGreaterThan(280 - 6);
    expect(floor).toBeLessThan(280 + 30);

    // Clamp ceiling: hammer ArrowLeft until it stops growing (72% of 1200 = 864).
    for (let i = 0; i < 60; i++) {
      pressKey(handleEl, 'ArrowLeft');
      await new Promise((r) => setTimeout(r, 20));
    }
    const ceiling = detailPane()!.getBoundingClientRect().width;
    expect(ceiling).toBeLessThan(1200 * 0.72 + 20);
  });

  it('the detail pane close button calls onclose', async () => {
    const onclose = vi.fn();
    const screen = await render(RecallSplit, {
      open: true,
      onclose,
      autoSaveId: 'test-recall-split-4',
      list: fakeListSnippet(),
      detail: fakeDetailSnippet(onclose)
    });
    screen.container.style.width = '1200px';
    screen.container.style.height = '400px';

    await screen.getByTestId('fake-detail-close').click();
    expect(onclose).toHaveBeenCalledTimes(1);
  });

  it('below 760px, the open detail pane renders as a full overlay over the list, with its own close button', async () => {
    const onclose = vi.fn();
    const screen = await render(RecallSplit, {
      open: true,
      onclose,
      autoSaveId: 'test-recall-split-5',
      list: fakeListSnippet(),
      detail: fakeDetailSnippet(onclose)
    });
    screen.container.style.width = '700px';
    screen.container.style.height = '400px';

    await expect.element(screen.getByTestId('fake-list')).toBeInTheDocument();
    const overlay = () => screen.container.querySelector('.rs-overlay');
    await waitForWidth(overlay, (w) => w > 600);
    await expect.element(screen.getByRole('button', { name: 'close detail' })).toBeInTheDocument();

    await screen.getByRole('button', { name: 'close detail' }).click();
    expect(onclose).toHaveBeenCalledTimes(1);
  });
});
