// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { ListTagsResponseSchema } from '$lib/gen/engram_pb';
import { tagListFooter, scopeLabel } from '$lib/tags/tags';
import TagBars from './TagBars.svelte';

const { listTagsSpy } = vi.hoisted(() => ({ listTagsSpy: vi.fn() }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return { ...actual, engram: { ...actual.engram, listTags: listTagsSpy } };
});

function fakeTags() {
  return create(ListTagsResponseSchema, {
    tags: [
      { tag: 'qdrant', count: 400n },
      { tag: 'mcp', count: 200n },
      { tag: 'oauth', count: 100n }
    ],
    more: false
  });
}

function tagsOfSize(n: number, more = false) {
  return create(ListTagsResponseSchema, {
    tags: Array.from({ length: n }, (_, i) => ({ tag: `tag${String(i).padStart(3, '0')}`, count: BigInt(n - i) })),
    more
  });
}

let qc: QueryClient;

function renderBars(props: Record<string, unknown> = {}, client: QueryClient = qc) {
  return render(
    TagBars,
    { scope: 'repo:acme/x', mode: 'panel', ontoggle: vi.fn(), ...props },
    { wrapper: QueryClientProvider, wrapperProps: { client } }
  );
}

beforeEach(() => {
  listTagsSpy.mockReset();
  listTagsSpy.mockResolvedValue(fakeTags());
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

describe('TagBars — one cached ListTags query, rows as linear bars', () => {
  it('fetches ListTags once with the scope and limit 1000, and draws three rows in response order with their counts', async () => {
    const screen = await renderBars();

    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
    expect(listTagsSpy).toHaveBeenCalledWith({ scope: 'repo:acme/x', limit: 1000n }, expect.anything());

    const rows = screen.container.querySelectorAll('[role="option"]');
    expect(rows.length).toBe(3);
    expect(rows[0].textContent).toContain('qdrant');
    expect(rows[0].textContent).toContain('400');
    expect(rows[1].textContent).toContain('mcp');
    expect(rows[1].textContent).toContain('200');
    expect(rows[2].textContent).toContain('oauth');
    expect(rows[2].textContent).toContain('100');
  });

  it('the first (largest) bar is 100% wide and a half-count bar is 50%', async () => {
    const screen = await renderBars();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const fills = screen.container.querySelectorAll('.fill');
    expect((fills[0] as HTMLElement).style.width).toBe('100%');
    expect((fills[1] as HTMLElement).style.width).toBe('50%');
  });

  it('clicking a row calls ontoggle with its tag', async () => {
    const ontoggle = vi.fn();
    const screen = await renderBars({ ontoggle });
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const rows = screen.container.querySelectorAll('[role="option"]');
    (rows[1] as HTMLElement).click();
    expect(ontoggle).toHaveBeenCalledWith('mcp');
  });

  it('mounting a second TagBars for the same scope in the same query client reuses the cache -- listTags is still called once', async () => {
    const screen1 = await renderBars();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const screen2 = await renderBars({}, qc);
    await expect.element(screen2.container.querySelectorAll('[role="option"]')[0]!).toBeInTheDocument();
    expect(listTagsSpy).toHaveBeenCalledTimes(1);
    void screen1;
  });
});

describe('TagBars — top 30 / show all (D-15, TAGS-01 boundary)', () => {
  it('31 loaded tags show 30 rows plus "show all", which reveals the 31st with listTags still called once', async () => {
    listTagsSpy.mockResolvedValue(tagsOfSize(31));
    const screen = await renderBars();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    let rows = screen.container.querySelectorAll('[role="option"]');
    expect(rows.length).toBe(30);
    const showAll = screen.getByRole('button', { name: 'show all' });
    await expect.element(showAll).toBeInTheDocument();

    await showAll.click();
    rows = screen.container.querySelectorAll('[role="option"]');
    expect(rows.length).toBe(31);
    expect(listTagsSpy).toHaveBeenCalledTimes(1);
  });

  it('30 loaded tags show no "show all"', async () => {
    listTagsSpy.mockResolvedValue(tagsOfSize(30));
    const screen = await renderBars();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
    expect(screen.container.querySelectorAll('[role="option"]').length).toBe(30);
    await expect.element(screen.getByRole('button', { name: 'show all' })).not.toBeInTheDocument();
  });

  it('1 loaded tag shows a single row and no "show all"', async () => {
    listTagsSpy.mockResolvedValue(tagsOfSize(1));
    const screen = await renderBars();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
    expect(screen.container.querySelectorAll('[role="option"]').length).toBe(1);
    await expect.element(screen.getByRole('button', { name: 'show all' })).not.toBeInTheDocument();
  });
});

describe('TagBars — filter box (E4 partial)', () => {
  it('narrows rows to the matching set and shows the no-match copy at zero', async () => {
    const screen = await renderBars();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const filter = screen.getByRole('textbox', { name: 'Filter tags' });
    await filter.fill('qd');
    let rows = screen.container.querySelectorAll('[role="option"]');
    expect(rows.length).toBe(1);
    expect(rows[0].textContent).toContain('qdrant');

    await filter.fill('zzz');
    rows = screen.container.querySelectorAll('[role="option"]');
    expect(rows.length).toBe(0);
    await expect.element(screen.getByText('No tags match "zzz" in the loaded set.')).toBeInTheDocument();
  });
});

describe('TagBars — honesty footer (D-15)', () => {
  it('renders the warning-toned footer computed by tagListFooter when expanded and more=true', async () => {
    listTagsSpy.mockResolvedValue(tagsOfSize(31, true));
    const screen = await renderBars();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    await screen.getByRole('button', { name: 'show all' }).click();
    const expected = tagListFooter({ loaded: 31, more: true, filtered: false, expanded: true });
    await expect.element(screen.getByText(expected!.text)).toBeInTheDocument();
  });
});

describe('TagBars — empty scope (E4 empty)', () => {
  it('renders the two-line empty copy naming the scope label', async () => {
    listTagsSpy.mockResolvedValue(tagsOfSize(0));
    const screen = await renderBars();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    await expect
      .element(screen.getByText(`No tags among recall-visible records in ${scopeLabel('repo:acme/x')}.`))
      .toBeInTheDocument();
    await expect
      .element(
        screen.getByText(
          "Tags on archived, superseded, expired, or not-yet-active records aren't counted — you can still type a #tag to filter."
        )
      )
      .toBeInTheDocument();
  });
});

describe('TagBars — loading (E4 loading)', () => {
  it('renders the ListTags call line and 8 skeleton bars while the query is pending', async () => {
    let resolveFetch!: (v: ReturnType<typeof fakeTags>) => void;
    listTagsSpy.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveFetch = resolve;
        })
    );
    const screen = await renderBars();
    await expect
      .element(screen.getByText('ListTags(scope="repo:acme/x", limit=1000) in flight▍'))
      .toBeInTheDocument();
    expect(screen.container.querySelectorAll('[data-slot="skeleton"]').length).toBe(8);

    resolveFetch(fakeTags());
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);
  });
});

describe('TagBars — errors (E4 error)', () => {
  it('a rejected envelope renders "Server rejected the request" plus the raw envelope', async () => {
    listTagsSpy.mockRejectedValue(
      new ConnectError('field=scope hint=invalid: scope is malformed', Code.FailedPrecondition)
    );
    const screen = await renderBars();
    await expect.element(screen.getByText('Server rejected the request')).toBeInTheDocument();
    await expect.element(screen.getByText('field=scope hint=invalid: scope is malformed')).toBeInTheDocument();
  });

  it('an opaque failure renders the opaque line and a Retry that refetches', async () => {
    listTagsSpy.mockRejectedValue(new ConnectError('boom', Code.Internal));
    const screen = await renderBars();
    await expect
      .element(screen.getByText('ListTags failed — nothing was listed. list_tags returned code=Internal'))
      .toBeInTheDocument();

    listTagsSpy.mockResolvedValue(fakeTags());
    await screen.getByRole('button', { name: 'Retry' }).click();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(2);
    await expect.element(screen.container.querySelectorAll('[role="option"]')[0]!).toBeInTheDocument();
  });
});

describe('TagBars — markers, selection and tooltip', () => {
  it('marked rows carry the ● marker and .marked class', async () => {
    const screen = await renderBars({ markedTags: new Set(['qdrant']) });
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const rows = screen.container.querySelectorAll('[role="option"]');
    expect(rows[0].classList.contains('marked')).toBe(true);
    expect(rows[0].textContent).toContain('●');
    expect(rows[1].classList.contains('marked')).toBe(false);
  });

  it('selected rows carry aria-selected="true"', async () => {
    const screen = await renderBars({ selectedTags: new Set(['mcp']) });
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const rows = screen.container.querySelectorAll('[role="option"]');
    expect(rows[1].getAttribute('aria-selected')).toBe('true');
    expect(rows[0].getAttribute('aria-selected')).toBe('false');
  });

  it('a row\'s title is "count {N}", plus a second "rarity ln(n/df) {x.xx}" line when rarity is supplied', async () => {
    const screen = await renderBars({ rarity: new Map([['qdrant', 1.1]]) });
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const rows = screen.container.querySelectorAll('[role="option"]');
    expect(rows[0].getAttribute('title')).toBe('count 400\nrarity ln(n/df) 1.10');
    expect(rows[1].getAttribute('title')).toBe('count 200');
  });
});

function fireKey(el: Element, key: string) {
  el.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
}

describe('TagBars — keyboard', () => {
  it('ArrowDown/Home/End move aria-activedescendant, and Enter/Space call ontoggle on the active row', async () => {
    const ontoggle = vi.fn();
    const screen = await renderBars({ ontoggle });
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    const listbox = screen.getByRole('listbox');
    const el = (await listbox.element()) as HTMLElement;
    el.focus();

    fireKey(el, 'ArrowDown');
    await expect.poll(() => document.getElementById(el.getAttribute('aria-activedescendant')!)?.textContent).toContain('mcp');

    fireKey(el, 'End');
    await expect.poll(() => document.getElementById(el.getAttribute('aria-activedescendant')!)?.textContent).toContain('oauth');

    fireKey(el, 'Home');
    await expect.poll(() => document.getElementById(el.getAttribute('aria-activedescendant')!)?.textContent).toContain('qdrant');

    fireKey(el, 'Enter');
    await expect.poll(() => ontoggle.mock.calls.length).toBe(1);
    expect(ontoggle).toHaveBeenCalledWith('qdrant');
  });
});

describe('TagBars — screenshots (light + dark)', () => {
  afterEach(() => {
    document.documentElement.classList.remove('dark');
  });

  it('captures the populated bar list in both themes', async () => {
    const screen = await renderBars();
    await expect.poll(() => listTagsSpy).toHaveBeenCalledTimes(1);

    document.documentElement.classList.remove('dark');
    await page.screenshot();

    document.documentElement.classList.add('dark');
    await page.screenshot();
    void screen;
  });
});
