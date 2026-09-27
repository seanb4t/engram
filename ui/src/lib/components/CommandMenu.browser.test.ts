import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { MemorySchema, type Memory } from '$lib/gen/engram_pb';
import { headerSearch } from '$lib/search/header-search.svelte';
import { DEFAULT_TEXT_SIZE } from '$lib/display.svelte';
import { registerCurationHost } from '$lib/curation/host.svelte.ts';
import CommandMenu from './CommandMenu.svelte';

const { gotoSpy, searchMemoriesSpy, getMemorySpy, setModeSpy, pageState } = vi.hoisted(() => ({
  gotoSpy: vi.fn(),
  searchMemoriesSpy: vi.fn(),
  getMemorySpy: vi.fn(),
  setModeSpy: vi.fn(),
  pageState: { url: new URL('http://localhost/ui/search') }
}));

vi.mock('$app/navigation', () => ({ goto: gotoSpy }));
vi.mock('$app/paths', () => ({ base: '/ui' }));
vi.mock('$app/state', () => ({ page: pageState }));
vi.mock('mode-watcher', () => ({
  setMode: setModeSpy,
  mode: { current: 'light' }
}));

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

function fakeMemory(overrides: Partial<{ id: string; shortId: string }> = {}): Memory {
  return create(MemorySchema, {
    id: 'm1',
    content: 'body',
    scope: 'repo:x',
    category: 'gotcha',
    ...overrides
  });
}

let qc: QueryClient;
function renderMenu() {
  return render(CommandMenu, { open: true }, { wrapper: QueryClientProvider, wrapperProps: { client: qc } });
}

beforeEach(() => {
  gotoSpy.mockReset();
  searchMemoriesSpy.mockReset();
  getMemorySpy.mockReset();
  setModeSpy.mockReset();
  pageState.url = new URL('http://localhost/ui/search');
  headerSearch.text = '';
  headerSearch.focusSeq = 0;
  document.documentElement.style.setProperty('--ui-font', `${DEFAULT_TEXT_SIZE}px`);
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('CommandMenu', () => {
  it('shows every navigation item and no hand-off row for an empty input, and makes no RPC', async () => {
    const screen = await renderMenu();
    await expect.element(screen.getByRole('option', { name: 'Home', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Rules', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Scheduled', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Search', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Discovery', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Observe', exact: true })).not.toBeInTheDocument();
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
    await screen.getByRole('combobox', { name: 'Command menu' }).fill('rul');

    await expect.element(screen.getByRole('option', { name: 'Rules', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Discovery', exact: true })).not.toBeInTheDocument();
    await expect
      .element(screen.getByRole('option', { name: /search memories for "rul"/i }))
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

  it('steps the text size up on "Larger text"', async () => {
    const screen = await renderMenu();
    await screen.getByRole('option', { name: /^Larger text/ }).click();
    expect(document.documentElement.style.getPropertyValue('--ui-font')).toBe(`${DEFAULT_TEXT_SIZE + 1}px`);
  });

  it('returns the text size to the default on "Reset text size"', async () => {
    document.documentElement.style.setProperty('--ui-font', `${DEFAULT_TEXT_SIZE + 2}px`);
    const screen = await renderMenu();
    await screen.getByRole('option', { name: /^Reset text size/ }).click();
    expect(document.documentElement.style.getPropertyValue('--ui-font')).toBe(`${DEFAULT_TEXT_SIZE}px`);
  });

  it('toggles the theme to the opposite of the current mode', async () => {
    const screen = await renderMenu();
    await screen.getByRole('option', { name: 'Toggle theme', exact: true }).click();
    expect(setModeSpy).toHaveBeenCalledWith('dark');
  });

  it('filters to the Display items for a typed term and still ends with the hand-off row', async () => {
    const screen = await renderMenu();
    await screen.getByRole('combobox', { name: 'Command menu' }).fill('text');

    await expect.element(screen.getByRole('option', { name: /^Larger text/ })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: /^Smaller text/ })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: /^Reset text size/ })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Toggle theme', exact: true })).not.toBeInTheDocument();
    await expect
      .element(screen.getByRole('option', { name: /search memories for "text"/i }))
      .toBeInTheDocument();
  });

  it('offers copy-id commands when a record is selected via ?sel=, and copies the short_id', async () => {
    const uuid = '753aba22-0000-4000-8000-000000000001';
    pageState.url = new URL(`http://localhost/ui/search?sel=${uuid}`);
    qc.setQueryData(['getMemory', uuid], { memory: fakeMemory({ id: uuid, shortId: 'k3m9p2qr7a' }) });
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);

    const screen = await renderMenu();
    const copyFullId = screen.getByRole('option', { name: /copy full id 753aba22/i });
    const copyShortId = screen.getByRole('option', { name: 'Copy short_id k3m9p2qr7a', exact: true });
    await expect.element(copyFullId).toBeInTheDocument();
    await expect.element(copyShortId).toBeInTheDocument();

    await copyShortId.click();
    expect(writeText).toHaveBeenCalledWith('k3m9p2qr7a');
  });

  it('copies the full id when "Copy full id" is selected', async () => {
    const uuid = '753aba22-0000-4000-8000-000000000001';
    pageState.url = new URL(`http://localhost/ui/search?sel=${uuid}`);
    qc.setQueryData(['getMemory', uuid], { memory: fakeMemory({ id: uuid, shortId: 'k3m9p2qr7a' }) });
    const writeText = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);

    const screen = await renderMenu();
    await screen.getByRole('option', { name: /copy full id 753aba22/i }).click();
    expect(writeText).toHaveBeenCalledWith(uuid);
  });

  it('shows no Record group when there is no ?sel=', async () => {
    const screen = await renderMenu();
    await expect.element(screen.getByText(/copy full id/i)).not.toBeInTheDocument();
    await expect.element(screen.getByText(/copy short_id/i)).not.toBeInTheDocument();
  });
});

describe('CommandMenu — curation row actions from the registered host (Phase 2 D-11)', () => {
  let unregister: (() => void) | undefined;

  afterEach(() => {
    unregister?.();
    unregister = undefined;
  });

  it('lists Supersede/Archive/Show chain items for the selected record when a host is registered', async () => {
    const uuid = '753aba22-0000-4000-8000-000000000001';
    pageState.url = new URL(`http://localhost/ui/search?sel=${uuid}`);
    qc.setQueryData(['getMemory', uuid], { memory: fakeMemory({ id: uuid, shortId: 'k3m9p2qr7a' }) });
    const run = vi.fn();
    unregister = registerCurationHost({ actionsFor: () => ['supersede', 'archive', 'chain'], run });

    const screen = await renderMenu();
    await expect.element(screen.getByRole('option', { name: 'Supersede k3m9p2qr7a…', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Archive k3m9p2qr7a', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Show chain k3m9p2qr7a', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Restore k3m9p2qr7a', exact: true })).not.toBeInTheDocument();
  });

  it('selecting "Archive {short}" closes the menu and calls run("archive", [id])', async () => {
    const uuid = '753aba22-0000-4000-8000-000000000001';
    pageState.url = new URL(`http://localhost/ui/search?sel=${uuid}`);
    qc.setQueryData(['getMemory', uuid], { memory: fakeMemory({ id: uuid, shortId: 'k3m9p2qr7a' }) });
    const run = vi.fn();
    unregister = registerCurationHost({ actionsFor: () => ['archive'], run });

    const screen = await renderMenu();
    await screen.getByRole('option', { name: 'Archive k3m9p2qr7a', exact: true }).click();

    expect(run).toHaveBeenCalledExactlyOnceWith('archive', [uuid]);
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument();
  });

  it("an archived record's host actionsFor returning ['restore'] lists \"Restore {short}\" instead of Archive", async () => {
    const uuid = '753aba22-0000-4000-8000-000000000001';
    pageState.url = new URL(`http://localhost/ui/search?sel=${uuid}`);
    qc.setQueryData(['getMemory', uuid], { memory: fakeMemory({ id: uuid, shortId: 'k3m9p2qr7a' }) });
    unregister = registerCurationHost({ actionsFor: () => ['restore'], run: vi.fn() });

    const screen = await renderMenu();
    await expect.element(screen.getByRole('option', { name: 'Restore k3m9p2qr7a', exact: true })).toBeInTheDocument();
    await expect.element(screen.getByRole('option', { name: 'Archive k3m9p2qr7a', exact: true })).not.toBeInTheDocument();
  });

  it('with no host registered, only the copy items appear for a selected record', async () => {
    const uuid = '753aba22-0000-4000-8000-000000000001';
    pageState.url = new URL(`http://localhost/ui/search?sel=${uuid}`);
    qc.setQueryData(['getMemory', uuid], { memory: fakeMemory({ id: uuid, shortId: 'k3m9p2qr7a' }) });

    const screen = await renderMenu();
    await expect.element(screen.getByRole('option', { name: /copy full id/i })).toBeInTheDocument();
    await expect
      .element(screen.getByRole('option', { name: 'Supersede k3m9p2qr7a…', exact: true }))
      .not.toBeInTheDocument();
    await expect
      .element(screen.getByRole('option', { name: 'Show chain k3m9p2qr7a', exact: true }))
      .not.toBeInTheDocument();
  });
});
