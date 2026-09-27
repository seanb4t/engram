import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { page } from 'vitest/browser';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import {
  MemorySchema,
  GetMemoryResponseSchema,
  type GetMemoryResponse,
  RelatedMemoriesResponseSchema,
  RelatedMemorySchema,
  RelatedEdgeSchema,
  SupersessionEvidenceSchema,
  EdgeType,
  SupersessionDirection
} from '$lib/gen/engram_pb';
import ChainDialog from './ChainDialog.svelte';

const { relatedMemoriesSpy, getMemorySpy } = vi.hoisted(() => ({ relatedMemoriesSpy: vi.fn(), getMemorySpy: vi.fn() }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return { ...actual, engram: { ...actual.engram, relatedMemories: relatedMemoriesSpy, getMemory: getMemorySpy } };
});

function mem(id: string, overrides: Partial<Record<string, unknown>> = {}) {
  return create(MemorySchema, {
    id,
    shortId: `${id.slice(0, 8).toUpperCase()}01`,
    summary: `summary for ${id}`,
    category: 'gotcha',
    ...overrides
  });
}

function supersessionEdge(direction: SupersessionDirection, depth: number) {
  return create(RelatedEdgeSchema, {
    type: EdgeType.SUPERSESSION,
    evidence: { case: 'supersession', value: create(SupersessionEvidenceSchema, { direction, depth }) }
  });
}

const P1 = mem('p1-id', { supersedes: ['p2-id'], supersededBy: 'head-id' });
const P2 = mem('p2-id', { supersededBy: 'p1-id' });
const HEAD = mem('head-id', { supersedes: ['p1-id'] });

function renderDialog(client = new QueryClient({ defaultOptions: { queries: { retry: false } } })) {
  return render(
    ChainDialog,
    { open: true, anchorId: P1.id, oncancel: vi.fn() },
    { wrapper: QueryClientProvider, wrapperProps: { client } }
  );
}

describe('ChainDialog — Task 1 tracer: real three-node chain', () => {
  beforeEach(() => {
    relatedMemoriesSpy.mockReset();
    relatedMemoriesSpy.mockImplementation(async ({ id }: { id: string }) => {
      if (id === P1.id) {
        // First call: opened for p1, whose successor (head) is returned.
        return create(RelatedMemoriesResponseSchema, {
          anchor: P1,
          related: [create(RelatedMemorySchema, { memory: HEAD, edges: [supersessionEdge(SupersessionDirection.SUCCESSOR, 1)] })]
        });
      }
      if (id === HEAD.id) {
        // Second call: the head's own predecessor tree (p1 depth 1, p2 depth 2).
        return create(RelatedMemoriesResponseSchema, {
          anchor: HEAD,
          related: [
            create(RelatedMemorySchema, { memory: P1, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] }),
            create(RelatedMemorySchema, { memory: P2, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 2)] })
          ]
        });
      }
      throw new Error(`unexpected id ${id}`);
    });
  });

  it('renders the header, three columns d2/d1/head·d0, and p1 highlighted', async () => {
    const screen = await renderDialog();

    await expect.poll(() => relatedMemoriesSpy).toHaveBeenCalledTimes(2);

    await expect.element(screen.getByText(`Chain · ${P1.shortId}`)).toBeInTheDocument();

    // Dialog content is rendered into a portal outside vitest-browser-svelte's
    // render container, so column nodes are queried from `document` directly.
    await expect.poll(() => document.querySelectorAll('.ccol-h').length).toBe(3);
    const headers = document.querySelectorAll('.ccol-h');
    expect(Array.from(headers).map((h) => h.textContent)).toEqual(['d2', 'd1', 'head · d0']);

    const p1Node = document.querySelector('.cnode.hl');
    expect(p1Node?.textContent).toContain(P1.shortId);

    // E3 populated — screenshot 1/3.
    await page.screenshot({ path: '__screenshots__/chain-dialog-populated.png' });
  });
});

function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

function nodeByShortId(shortId: string): HTMLElement {
  const found = [...document.querySelectorAll('.cnode')].find((n) => n.textContent?.includes(shortId));
  if (!found) throw new Error(`no .cnode contains ${shortId}`);
  return found as HTMLElement;
}

describe('ChainDialog — peek by id (Task 2)', () => {
  beforeEach(() => {
    relatedMemoriesSpy.mockReset();
    getMemorySpy.mockReset();
    relatedMemoriesSpy.mockImplementation(async ({ id }: { id: string }) => {
      if (id === P1.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: P1,
          related: [create(RelatedMemorySchema, { memory: HEAD, edges: [supersessionEdge(SupersessionDirection.SUCCESSOR, 1)] })]
        });
      }
      if (id === HEAD.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: HEAD,
          related: [
            create(RelatedMemorySchema, { memory: P1, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] }),
            create(RelatedMemorySchema, { memory: P2, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 2)] })
          ]
        });
      }
      throw new Error(`unexpected id ${id}`);
    });
  });

  it('clicking a node shows a loading line on that node only, then the peek copy', async () => {
    const gate = deferred<GetMemoryResponse>();
    getMemorySpy.mockImplementation(() => gate.promise);

    await renderDialog();
    await expect.poll(() => document.querySelectorAll('.ccol-h').length).toBe(3);

    nodeByShortId(P2.shortId).click();

    await expect.poll(() => document.querySelector('.chain-peek')?.textContent ?? '').toMatch(/loading/i);
    // Only the clicked node's own peek region exists — never a dialog-wide spinner.
    expect(document.querySelectorAll('.chain-peek')).toHaveLength(1);

    gate.resolve(create(GetMemoryResponseSchema, { memory: P2 }));

    await expect
      .poll(() => document.querySelector('.chain-peek')?.textContent ?? '')
      .toContain(`get_memory ${P2.shortId} · fetch-by-id ignores the recall gate · superseded_by ${P2.supersededBy}`);

    // E3 peek — screenshot 2/3.
    await page.screenshot({ path: '__screenshots__/chain-dialog-peek.png' });
  });

  it('a peek NotFound renders the not-found copy inline', async () => {
    getMemorySpy.mockRejectedValue(new ConnectError('missing', Code.NotFound));

    await renderDialog();
    await expect.poll(() => document.querySelectorAll('.ccol-h').length).toBe(3);

    nodeByShortId(P2.shortId).click();

    await expect
      .poll(() => document.querySelector('.chain-peek')?.textContent ?? '')
      .toContain(`No memory with id ${P2.id} that you can read`);
  });
});

describe('ChainDialog — placeholders for unreadable predecessors (Task 2)', () => {
  const MID_HEAD = mem('mid-head-id', { supersedes: ['p1b-id'] });
  const P1B = mem('p1b-id', { supersedes: ['missing-id'], supersededBy: 'mid-head-id' });

  beforeEach(() => {
    relatedMemoriesSpy.mockReset();
    getMemorySpy.mockReset();
    relatedMemoriesSpy.mockImplementation(async ({ id }: { id: string }) => {
      if (id === P1B.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: P1B,
          related: [create(RelatedMemorySchema, { memory: MID_HEAD, edges: [supersessionEdge(SupersessionDirection.SUCCESSOR, 1)] })]
        });
      }
      if (id === MID_HEAD.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: MID_HEAD,
          related: [create(RelatedMemorySchema, { memory: P1B, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] })]
        });
      }
      throw new Error(`unexpected id ${id}`);
    });
  });

  it('a predecessor absent from the RelatedMemories result renders as a not-clickable placeholder carrying the not-found copy', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    await render(
      ChainDialog,
      { open: true, anchorId: P1B.id, oncancel: vi.fn() },
      { wrapper: QueryClientProvider, wrapperProps: { client } }
    );

    await expect.poll(() => document.querySelectorAll('.ccol-h').length).toBe(3);

    const placeholder = document.querySelector('.cnode.placeholder');
    expect(placeholder).toBeTruthy();
    expect(placeholder?.tagName).not.toBe('BUTTON');
    expect(placeholder?.textContent).toContain('No memory with id missing-id that you can read');

    // E3 partial — screenshot 3/3.
    await page.screenshot({ path: '__screenshots__/chain-dialog-placeholder.png' });
  });
});

describe('ChainDialog — subline (Task 2)', () => {
  beforeEach(() => {
    relatedMemoriesSpy.mockReset();
    relatedMemoriesSpy.mockImplementation(async ({ id }: { id: string }) => {
      if (id === P1.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: P1,
          related: [create(RelatedMemorySchema, { memory: HEAD, edges: [supersessionEdge(SupersessionDirection.SUCCESSOR, 1)] })]
        });
      }
      if (id === HEAD.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: HEAD,
          related: [
            create(RelatedMemorySchema, { memory: P1, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] }),
            create(RelatedMemorySchema, { memory: P2, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 2)] })
          ]
        });
      }
      throw new Error(`unexpected id ${id}`);
    });
  });

  it("reads depth d{N} below head ... N direct predecessor(s) ... successor ...", async () => {
    const screen = await renderDialog();
    await expect
      .element(screen.getByText(`depth d1 below head ${HEAD.shortId} · 1 direct predecessor(s) · successor ${HEAD.shortId}`))
      .toBeInTheDocument();
  });
});

describe('ChainDialog — Supersede head… footer (Task 2)', () => {
  it('appears for a convention head and calls onsupersedehead(headId)', async () => {
    relatedMemoriesSpy.mockReset();
    relatedMemoriesSpy.mockImplementation(async ({ id }: { id: string }) => {
      if (id === P1.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: P1,
          related: [create(RelatedMemorySchema, { memory: HEAD, edges: [supersessionEdge(SupersessionDirection.SUCCESSOR, 1)] })]
        });
      }
      if (id === HEAD.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: HEAD,
          related: [create(RelatedMemorySchema, { memory: P1, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] })]
        });
      }
      throw new Error(`unexpected id ${id}`);
    });

    const onsupersedehead = vi.fn();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const screen = await render(
      ChainDialog,
      { open: true, anchorId: P1.id, oncancel: vi.fn(), onsupersedehead },
      { wrapper: QueryClientProvider, wrapperProps: { client } }
    );

    const btn = screen.getByRole('button', { name: 'Supersede head…' });
    await expect.element(btn).toBeInTheDocument();
    await btn.click();
    expect(onsupersedehead).toHaveBeenCalledWith(HEAD.id);
  });

  it('is absent when the head is a rule record', async () => {
    const RULE_HEAD = mem('rule-head-id', { category: 'rule', supersedes: ['q1-id'] });
    const Q1 = mem('q1-id', { supersededBy: 'rule-head-id' });
    relatedMemoriesSpy.mockReset();
    relatedMemoriesSpy.mockImplementation(async ({ id }: { id: string }) => {
      if (id === Q1.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: Q1,
          related: [create(RelatedMemorySchema, { memory: RULE_HEAD, edges: [supersessionEdge(SupersessionDirection.SUCCESSOR, 1)] })]
        });
      }
      if (id === RULE_HEAD.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: RULE_HEAD,
          related: [create(RelatedMemorySchema, { memory: Q1, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] })]
        });
      }
      throw new Error(`unexpected id ${id}`);
    });

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const screen = await render(
      ChainDialog,
      { open: true, anchorId: Q1.id, oncancel: vi.fn() },
      { wrapper: QueryClientProvider, wrapperProps: { client } }
    );
    await expect.poll(() => document.querySelectorAll('.ccol-h').length).toBe(2);
    await expect.element(screen.getByText('Supersede head…')).not.toBeInTheDocument();
  });
});

describe('ChainDialog — zero-one-many (Task 2)', () => {
  it('a depth-1 chain (one predecessor, one head) still renders two columns', async () => {
    const HEAD3 = mem('head3-id', { supersedes: ['p1c-id'] });
    const P1C = mem('p1c-id', { supersededBy: 'head3-id' });
    relatedMemoriesSpy.mockReset();
    relatedMemoriesSpy.mockImplementation(async ({ id }: { id: string }) => {
      if (id === P1C.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: P1C,
          related: [create(RelatedMemorySchema, { memory: HEAD3, edges: [supersessionEdge(SupersessionDirection.SUCCESSOR, 1)] })]
        });
      }
      if (id === HEAD3.id) {
        return create(RelatedMemoriesResponseSchema, {
          anchor: HEAD3,
          related: [create(RelatedMemorySchema, { memory: P1C, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] })]
        });
      }
      throw new Error(`unexpected id ${id}`);
    });

    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    await render(ChainDialog, { open: true, anchorId: P1C.id, oncancel: vi.fn() }, { wrapper: QueryClientProvider, wrapperProps: { client } });

    await expect.poll(() => document.querySelectorAll('.ccol-h').length).toBe(2);
    const headers = document.querySelectorAll('.ccol-h');
    expect(Array.from(headers).map((h) => h.textContent)).toEqual(['d1', 'head · d0']);
  });
});
