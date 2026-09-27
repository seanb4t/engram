import { render } from 'vitest-browser-svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
import {
  MemorySchema,
  RelatedMemoriesResponseSchema,
  RelatedMemorySchema,
  RelatedEdgeSchema,
  SupersessionEvidenceSchema,
  EdgeType,
  SupersessionDirection
} from '$lib/gen/engram_pb';
import ChainDialog from './ChainDialog.svelte';

const { relatedMemoriesSpy } = vi.hoisted(() => ({ relatedMemoriesSpy: vi.fn() }));

vi.mock('$lib/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/client')>();
  return { ...actual, engram: { ...actual.engram, relatedMemories: relatedMemoriesSpy } };
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

const P1 = mem('p1-id');
const P2 = mem('p2-id');
const HEAD = mem('head-id');

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
  });
});
