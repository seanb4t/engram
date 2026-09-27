import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import {
  MemorySchema,
  RelatedMemoriesResponseSchema,
  RelatedMemorySchema,
  RelatedEdgeSchema,
  SupersessionEvidenceSchema,
  VectorEvidenceSchema,
  EdgeType,
  SupersessionDirection
} from '$lib/gen/engram_pb';
import { headIdFrom, buildChain, compareChainNodes, type ChainNode } from './chain';

const NOW = new Date('2030-01-01T00:00:00Z');
const EARLIER = new Date(NOW.getTime() - 60_000);
const LATER = new Date(NOW.getTime() + 60_000);

function mem(id: string, overrides: Partial<Record<string, unknown>> = {}) {
  return create(MemorySchema, {
    id,
    shortId: id.slice(0, 10).toUpperCase(),
    summary: `summary for ${id}`,
    category: 'gotcha',
    createdAt: timestampFromDate(NOW),
    ...overrides
  });
}

function supersessionEdge(direction: SupersessionDirection, depth: number) {
  return create(RelatedEdgeSchema, {
    type: EdgeType.SUPERSESSION,
    evidence: { case: 'supersession', value: create(SupersessionEvidenceSchema, { direction, depth }) }
  });
}

function vectorEdge() {
  return create(RelatedEdgeSchema, {
    type: EdgeType.VECTOR,
    evidence: { case: 'vector', value: create(VectorEvidenceSchema, { score: 0.9 }) }
  });
}

describe('headIdFrom', () => {
  it('returns the anchor id when there is no successor edge', () => {
    const resp = create(RelatedMemoriesResponseSchema, {
      anchor: mem('anchor-id'),
      related: []
    });
    expect(headIdFrom(resp)).toBe('anchor-id');
  });

  it('returns the successor with the greatest supersession depth', () => {
    const resp = create(RelatedMemoriesResponseSchema, {
      anchor: mem('anchor-id'),
      related: [
        create(RelatedMemorySchema, {
          memory: mem('successor-1'),
          edges: [supersessionEdge(SupersessionDirection.SUCCESSOR, 1)]
        }),
        create(RelatedMemorySchema, {
          memory: mem('successor-2-head'),
          edges: [supersessionEdge(SupersessionDirection.SUCCESSOR, 2)]
        })
      ]
    });
    expect(headIdFrom(resp)).toBe('successor-2-head');
  });

  it('ignores non-supersession edges', () => {
    const resp = create(RelatedMemoriesResponseSchema, {
      anchor: mem('anchor-id'),
      related: [create(RelatedMemorySchema, { memory: mem('vector-neighbor'), edges: [vectorEdge()] })]
    });
    expect(headIdFrom(resp)).toBe('anchor-id');
  });
});

describe('buildChain', () => {
  it('places head at depth 0 and predecessors at their edge depth, oldest-first columns', () => {
    const head = mem('head-id');
    const p1 = mem('p1-id');
    const p2 = mem('p2-id');
    const headResp = create(RelatedMemoriesResponseSchema, {
      anchor: head,
      related: [
        create(RelatedMemorySchema, { memory: p1, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] }),
        create(RelatedMemorySchema, { memory: p2, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 2)] })
      ]
    });

    const chain = buildChain(headResp, 'p1-id');

    expect(chain.columns).toHaveLength(3);
    expect(chain.columns[0].map((n) => n.id)).toEqual(['p2-id']);
    expect(chain.columns[1].map((n) => n.id)).toEqual(['p1-id']);
    expect(chain.columns[2].map((n) => n.id)).toEqual(['head-id']);
    expect(chain.headId).toBe('head-id');
  });

  it('anchorDepth equals the anchor id`s own depth in the chain', () => {
    const head = mem('head-id');
    const p1 = mem('p1-id');
    const headResp = create(RelatedMemoriesResponseSchema, {
      anchor: head,
      related: [create(RelatedMemorySchema, { memory: p1, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] })]
    });

    expect(buildChain(headResp, 'p1-id').anchorDepth).toBe(1);
    expect(buildChain(headResp, 'head-id').anchorDepth).toBe(0);
  });

  it('two depth-1 nodes are ordered by createdAt ascending, then id ascending on a tie', () => {
    const head = mem('head-id');
    const later = mem('later-id', { createdAt: timestampFromDate(LATER) });
    const earlier = mem('earlier-id', { createdAt: timestampFromDate(EARLIER) });
    const headResp = create(RelatedMemoriesResponseSchema, {
      anchor: head,
      related: [
        create(RelatedMemorySchema, { memory: later, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] }),
        create(RelatedMemorySchema, { memory: earlier, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] })
      ]
    });

    const chain = buildChain(headResp, 'head-id');
    expect(chain.columns[0].map((n) => n.id)).toEqual(['earlier-id', 'later-id']);
  });

  it('non-supersession edges are ignored when building columns', () => {
    const head = mem('head-id');
    const headResp = create(RelatedMemoriesResponseSchema, {
      anchor: head,
      related: [create(RelatedMemorySchema, { memory: mem('vector-neighbor'), edges: [vectorEdge()] })]
    });

    const chain = buildChain(headResp, 'head-id');
    expect(chain.columns).toHaveLength(1);
    expect(chain.columns[0].map((n) => n.id)).toEqual(['head-id']);
  });
});

describe('buildChain — placeholders for unreadable predecessors and beyondCap (Task 2)', () => {
  it('a readable depth-1 node whose supersedes lists an id absent from the result yields a placeholder node at depth 2', () => {
    const head = mem('head-id');
    const p1 = mem('p1-id', { supersedes: ['missing-id'] });
    const headResp = create(RelatedMemoriesResponseSchema, {
      anchor: head,
      related: [create(RelatedMemorySchema, { memory: p1, edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 1)] })]
    });

    const chain = buildChain(headResp, 'p1-id');

    expect(chain.columns).toHaveLength(3);
    expect(chain.columns[0]).toEqual([expect.objectContaining({ id: 'missing-id', placeholder: true, depth: 2 })]);
    expect(chain.columns[1].map((n) => n.id)).toEqual(['p1-id']);
    expect(chain.columns[2].map((n) => n.id)).toEqual(['head-id']);
  });

  it('a head-only response (no predecessors) yields one column [[head]]', () => {
    const head = mem('head-id');
    const headResp = create(RelatedMemoriesResponseSchema, { anchor: head, related: [] });
    const chain = buildChain(headResp, 'head-id');
    expect(chain.columns).toHaveLength(1);
    expect(chain.columns[0].map((n) => n.id)).toEqual(['head-id']);
  });

  it('beyondCap is true when the top-most successor (the head) itself still has supersededBy set', () => {
    const head = mem('head-id', { supersededBy: 'beyond-the-cap-id' });
    const headResp = create(RelatedMemoriesResponseSchema, { anchor: head, related: [] });
    expect(buildChain(headResp, 'head-id').beyondCap).toBe(true);
  });

  it('beyondCap is false when the head has no supersededBy', () => {
    const head = mem('head-id');
    const headResp = create(RelatedMemoriesResponseSchema, { anchor: head, related: [] });
    expect(buildChain(headResp, 'head-id').beyondCap).toBe(false);
  });
});

function chainNode(id: string, createdAt?: Date): ChainNode {
  return { id, shortId: id, summary: '', category: '', kind: '', createdAt, depth: 0, placeholder: false };
}

describe('compareChainNodes', () => {
  it('breaks a createdAt tie by id ascending', () => {
    const a = chainNode('b-id', NOW);
    const b = chainNode('a-id', NOW);
    expect(compareChainNodes(a, b)).toBeGreaterThan(0);
  });
});
