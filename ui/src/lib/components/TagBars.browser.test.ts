// See ResultRow.browser.test.ts for why this import is required: isolated
// component mounts never pull in +layout.svelte's app.css, so --u and the
// design tokens are otherwise invalid/no-op in this cascade.
import '../../app.css';
import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import { ListTagsResponseSchema } from '$lib/gen/engram_pb';
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
