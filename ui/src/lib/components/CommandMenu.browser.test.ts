import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { headerSearch } from '$lib/search/header-search.svelte';
import CommandMenu from './CommandMenu.svelte';

const { gotoSpy, searchMemoriesSpy, getMemorySpy } = vi.hoisted(() => ({
  gotoSpy: vi.fn(),
  searchMemoriesSpy: vi.fn(),
  getMemorySpy: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));

// The menu makes no server call at all (D-11) -- these spies exist so every
// test below can assert they were never touched, not because CommandMenu
// imports $lib/client.
vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return {
    ...actual,
    engram: {
      ...actual.engram,
      searchMemories: searchMemoriesSpy,
      getMemory: getMemorySpy
    }
  };
});

function renderMenu() {
  return render(CommandMenu, { open: true });
}

beforeEach(() => {
  gotoSpy.mockReset();
  searchMemoriesSpy.mockReset();
  getMemorySpy.mockReset();
  headerSearch.text = '';
  headerSearch.focusSeq = 0;
});

describe('CommandMenu', () => {
  it('shows every navigation item and no hand-off row for an empty input, and makes no RPC', async () => {
    const screen = await renderMenu();
    await expect.element(screen.getByRole('option', { name: 'Home', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Observe', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Search', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Discovery', exact: true })).toBeInTheDocument();
    expect(document.body.textContent).not.toContain('no matches');
    expect(searchMemoriesSpy).not.toHaveBeenCalled();
    expect(getMemorySpy).not.toHaveBeenCalled();
  });

  it('shows the unfiltered hand-off row last for a memory term matching no static label, and never "no matches"', async () => {
    // Command.Dialog portals its content to document.body (not screen.container),
    // so DOM-order assertions below query document.body directly.
    const screen = await renderMenu();
    await screen.getByRole('combobox', { name: 'Command menu' }).fill('github');

    const handoff = screen.getByRole('option', { name: /search memories for "github"/i });
    await expect.element(handoff).toBeInTheDocument();
    expect(document.body.textContent).not.toContain('no matches');
    await expect.poll(() => {
      const opts = [...document.body.querySelectorAll('[role="option"]')];
      return opts.length > 0 && opts[opts.length - 1].textContent?.toLowerCase().includes('search memories for');
    }).toBe(true);
    expect(searchMemoriesSpy).not.toHaveBeenCalled();
    expect(getMemorySpy).not.toHaveBeenCalled();

    await handoff.click();
    expect(headerSearch.text).toBe('github');
    expect(headerSearch.focusSeq).toBe(1);
    expect(searchMemoriesSpy).not.toHaveBeenCalled();
    expect(getMemorySpy).not.toHaveBeenCalled();
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
  });

  it('filters navigation items by the typed term and still ends with the hand-off row', async () => {
    const screen = await renderMenu();
    await screen.getByRole('combobox', { name: 'Command menu' }).fill('obs');

    await expect.element(screen.getByRole('option', { name: 'Observe', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Discovery', exact: true })).not.toBeInTheDocument();
    await expect
      .element(screen.getByRole('option', { name: /search memories for "obs"/i }))
      .toBeInTheDocument();
  });

  it('resolves a pasted UUID to an "Open record" row and navigates to /search?q=<id>', async () => {
    const uuid = '753aba22-0000-4000-8000-000000000001';
    const screen = await renderMenu();
    await screen.getByRole('combobox', { name: 'Command menu' }).fill(uuid);

    const row = screen.getByRole('option', { name: /open record 753aba22/i });
    await expect.element(row).toBeInTheDocument();
    await row.click();

    expect(gotoSpy).toHaveBeenCalledWith(`/ui/search?q=${uuid}`);
    expect(searchMemoriesSpy).not.toHaveBeenCalled();
    expect(getMemorySpy).not.toHaveBeenCalled();
  });

  it('navigates and closes the menu when a Go-to item is selected', async () => {
    const screen = await renderMenu();
    await screen.getByRole('option', { name: 'Search', exact: true }).click();
    expect(gotoSpy).toHaveBeenCalledWith('/ui/search');
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
  });
});
