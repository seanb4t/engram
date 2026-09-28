import { describe, it, expect } from 'vitest';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import {
  MemorySchema,
  RelatedMemoriesResponseSchema,
  RelatedMemorySchema,
  RelatedEdgeSchema,
  VectorEvidenceSchema,
  TagEvidenceSchema,
  CitationEvidenceSchema,
  SupersessionEvidenceSchema,
  WeightedTagSchema,
  CitationRefSchema,
  EdgeType,
  SupersessionDirection,
  type Memory
} from '$lib/gen/engram_pb';
import {
  buildRelatedModel,
  laneRows,
  visibleMembership,
  graphEdges,
  nodeAccessibleName,
  neighbourhoodSummary,
  callLineParts,
  lcg,
  settleLayout,
  VECTOR_COLLAPSE,
  EDGE_OFFSET,
  type Candidate,
  type RelatedModel,
  type LaneType,
  type GraphNode
} from './graph';

const NOW = new Date('2030-01-01T00:00:00Z');
const EARLIER = new Date(NOW.getTime() - 60_000);
const LATER = new Date(NOW.getTime() + 60_000);

function mem(id: string, overrides: MessageInitShape<typeof MemorySchema> = {}): Memory {
  return create(MemorySchema, {
    id,
    shortId: id.slice(0, 10).padEnd(10, '0'),
    category: 'convention',
    summary: `summary for ${id}`,
    content: '',
    tags: [],
    scope: 'repo:test',
    visibility: 'private',
    owner: 'me',
    ...overrides
  });
}

function supersessionEdge(direction: SupersessionDirection, depth: number) {
  return create(RelatedEdgeSchema, {
    type: EdgeType.SUPERSESSION,
    evidence: { case: 'supersession', value: create(SupersessionEvidenceSchema, { direction, depth }) }
  });
}

function vectorEdge(score: number) {
  return create(RelatedEdgeSchema, {
    type: EdgeType.VECTOR,
    evidence: { case: 'vector', value: create(VectorEvidenceSchema, { score }) }
  });
}

function tagEdge(tagWeight: number) {
  return create(RelatedEdgeSchema, {
    type: EdgeType.TAG,
    evidence: { case: 'tag', value: create(TagEvidenceSchema, { sharedTags: [create(WeightedTagSchema, { tag: 'x', weight: tagWeight })], tagWeight }) }
  });
}

function citationEdge(refs: string[]) {
  return create(RelatedEdgeSchema, {
    type: EdgeType.CITATION,
    evidence: {
      case: 'citation',
      value: create(CitationEvidenceSchema, { sharedCitations: refs.map((r) => create(CitationRefSchema, { kind: 'file', ref: r })) })
    }
  });
}

// candidate builds a bare Candidate for laneRows/nodeAccessibleName-level
// tests -- bypasses buildRelatedModel so ordering/formatting can be tested
// in isolation from evidence-mapping.
function candidate(id: string, opts: Partial<Candidate> & { memory?: MessageInitShape<typeof MemorySchema> } = {}): Candidate {
  const memory = mem(id, opts.memory ?? {});
  return {
    id,
    shortId: memory.shortId,
    memory,
    types: opts.types ?? [],
    edges: opts.edges ?? {},
    strength: opts.strength ?? {},
    signedDepth: opts.signedDepth
  };
}

function emptyModel(candidates: Candidate[], overrides: Partial<RelatedModel> = {}): RelatedModel {
  return {
    anchor: mem('anchor-id'),
    truncated: false,
    k: 64,
    counts: { supersession: 0, citation: 0, tag: 0, vector: 0 },
    candidates,
    ...overrides
  };
}

describe('buildRelatedModel', () => {
  it('maps a vector edge to strength.vector and counts.vector', () => {
    const resp = create(RelatedMemoriesResponseSchema, {
      anchor: mem('anchor-id'),
      related: [create(RelatedMemorySchema, { memory: mem('v1'), edges: [vectorEdge(0.81)] })]
    });
    const model = buildRelatedModel(resp, 64);
    expect(model.candidates[0].strength.vector).toBeCloseTo(0.81);
    expect(model.candidates[0].types).toEqual(['vector']);
    expect(model.counts).toEqual({ supersession: 0, citation: 0, tag: 0, vector: 1 });
  });

  it('maps a tag edge to strength.tag (the summed tagWeight)', () => {
    const resp = create(RelatedMemoriesResponseSchema, {
      anchor: mem('anchor-id'),
      related: [create(RelatedMemorySchema, { memory: mem('t1'), edges: [tagEdge(3.05)] })]
    });
    const model = buildRelatedModel(resp, 64);
    expect(model.candidates[0].strength.tag).toBeCloseTo(3.05);
    expect(model.counts.tag).toBe(1);
  });

  it('maps a citation edge to strength.citation (sharedCitations.length)', () => {
    const resp = create(RelatedMemoriesResponseSchema, {
      anchor: mem('anchor-id'),
      related: [create(RelatedMemorySchema, { memory: mem('c1'), edges: [citationEdge(['a.go', 'b.go'])] })]
    });
    const model = buildRelatedModel(resp, 64);
    expect(model.candidates[0].strength.citation).toBe(2);
    expect(model.counts.citation).toBe(1);
  });

  it('maps a PREDECESSOR supersession edge to a negative signedDepth', () => {
    const resp = create(RelatedMemoriesResponseSchema, {
      anchor: mem('anchor-id'),
      related: [create(RelatedMemorySchema, { memory: mem('p1'), edges: [supersessionEdge(SupersessionDirection.PREDECESSOR, 2)] })]
    });
    const model = buildRelatedModel(resp, 64);
    expect(model.candidates[0].signedDepth).toBe(-2);
    expect(model.counts.supersession).toBe(1);
  });

  it('maps a SUCCESSOR supersession edge to a positive signedDepth', () => {
    const resp = create(RelatedMemoriesResponseSchema, {
      anchor: mem('anchor-id'),
      related: [create(RelatedMemorySchema, { memory: mem('s1'), edges: [supersessionEdge(SupersessionDirection.SUCCESSOR, 1)] })]
    });
    const model = buildRelatedModel(resp, 64);
    expect(model.candidates[0].signedDepth).toBe(1);
  });

  it('records every type that reached a multi-edge candidate, in canonical lane order', () => {
    const resp = create(RelatedMemoriesResponseSchema, {
      anchor: mem('anchor-id'),
      related: [create(RelatedMemorySchema, { memory: mem('m1'), edges: [vectorEdge(0.5), tagEdge(1.2)] })]
    });
    const model = buildRelatedModel(resp, 64);
    expect(model.candidates[0].types).toEqual(['tag', 'vector']);
  });
});

describe('laneRows', () => {
  it('sorts a non-supersession lane by strength descending, ties by id ascending', () => {
    const model = emptyModel([
      candidate('c-b', { types: ['tag'], strength: { tag: 5 } }),
      candidate('c-a', { types: ['tag'], strength: { tag: 5 } }),
      candidate('c-z', { types: ['tag'], strength: { tag: 9 } })
    ]);
    expect(laneRows(model, 'tag').map((c) => c.id)).toEqual(['c-z', 'c-a', 'c-b']);
  });

  it('sorts the supersession lane by signedDepth ascending (oldest predecessor first, then successors)', () => {
    const model = emptyModel([
      candidate('succ-1', { types: ['supersession'], signedDepth: 1, memory: { createdAt: timestampFromDate(LATER) } }),
      candidate('pred-2', { types: ['supersession'], signedDepth: -2, memory: { createdAt: timestampFromDate(EARLIER) } }),
      candidate('pred-1', { types: ['supersession'], signedDepth: -1, memory: { createdAt: timestampFromDate(NOW) } })
    ]);
    expect(laneRows(model, 'supersession').map((c) => c.id)).toEqual(['pred-2', 'pred-1', 'succ-1']);
  });

  it('breaks a supersession tie with compareChainNodes (createdAt ascending, then id ascending)', () => {
    const model = emptyModel([
      candidate('later-same-depth', { types: ['supersession'], signedDepth: -1, memory: { createdAt: timestampFromDate(LATER) } }),
      candidate('earlier-same-depth', { types: ['supersession'], signedDepth: -1, memory: { createdAt: timestampFromDate(EARLIER) } })
    ]);
    expect(laneRows(model, 'supersession').map((c) => c.id)).toEqual(['earlier-same-depth', 'later-same-depth']);
  });
});

describe('graphEdges', () => {
  it('gives a three-type candidate three edges with offsets -12 (vector), 0 (citation), +12 (tag)', () => {
    const c = candidate('multi', { types: ['citation', 'tag', 'vector'] });
    const edges = graphEdges('anchor-id', [c]);
    expect(edges).toHaveLength(3);
    const byType = Object.fromEntries(edges.map((e) => [e.type, e.offset]));
    expect(byType).toEqual({ citation: 0, tag: EDGE_OFFSET, vector: -EDGE_OFFSET });
  });

  it('points a predecessor supersession edge from the older candidate to the anchor, with an arrow', () => {
    const c = candidate('pred', { types: ['supersession'], signedDepth: -1 });
    const [edge] = graphEdges('anchor-id', [c]);
    expect(edge).toMatchObject({ source: 'pred', target: 'anchor-id', type: 'supersession', arrow: true, chain: false });
  });

  it('points a successor supersession edge from the anchor to the newer candidate, with an arrow', () => {
    const c = candidate('succ', { types: ['supersession'], signedDepth: 1 });
    const [edge] = graphEdges('anchor-id', [c]);
    expect(edge).toMatchObject({ source: 'anchor-id', target: 'succ', type: 'supersession', arrow: true, chain: false });
  });
});

describe('visibleMembership', () => {
  it('orders nodes anchor-first, each candidate once at its first canonical lane', () => {
    const model = emptyModel([
      candidate('vec-only', { types: ['vector'], strength: { vector: 0.9 } }),
      candidate('tag-and-vec', { types: ['tag', 'vector'], strength: { tag: 1, vector: 0.5 } }),
      candidate('sup-only', { types: ['supersession'], signedDepth: -1 })
    ]);
    const membership = visibleMembership(model, { hiddenTypes: new Set(), vectorExpanded: false });
    expect(membership.nodes.map((n) => n.id)).toEqual(['anchor-id', 'sup-only', 'tag-and-vec', 'vec-only']);
  });

  it('collapses the vector lane to 8 of 9 when vectorExpanded is false, all 9 when true', () => {
    const candidates: Candidate[] = [];
    for (let i = 0; i < 9; i++) {
      candidates.push(candidate(`v${i}`, { types: ['vector'], strength: { vector: 1 - i * 0.01 } }));
    }
    const model = emptyModel(candidates);
    const collapsed = visibleMembership(model, { hiddenTypes: new Set(), vectorExpanded: false });
    expect(collapsed.lanes.vector).toHaveLength(VECTOR_COLLAPSE);
    expect(collapsed.vectorTotal).toBe(9);
    expect(collapsed.vectorCollapsed).toBe(true);
    expect(collapsed.nodes).toHaveLength(1 + VECTOR_COLLAPSE);

    const expanded = visibleMembership(model, { hiddenTypes: new Set(), vectorExpanded: true });
    expect(expanded.lanes.vector).toHaveLength(9);
    expect(expanded.vectorCollapsed).toBe(false);
    expect(expanded.nodes).toHaveLength(10);
  });

  it('a hidden type removes that type\'s edges, and drops a node left with no drawn edge', () => {
    const model = emptyModel([
      candidate('tag-only', { types: ['tag'], strength: { tag: 2 } }),
      candidate('tag-and-vec', { types: ['tag', 'vector'], strength: { tag: 1, vector: 0.5 } })
    ]);
    const withTagHidden = visibleMembership(model, { hiddenTypes: new Set(['tag']), vectorExpanded: false });
    // tag-only had no other type -- it must be dropped entirely.
    expect(withTagHidden.nodes.map((n) => n.id)).toEqual(['anchor-id', 'tag-and-vec']);
    // tag-and-vec survives via its vector edge, but the tag edge is gone.
    const survivor = withTagHidden.nodes.find((n) => n.id === 'tag-and-vec');
    expect(survivor?.types).toEqual(['vector']);
    expect(withTagHidden.edges.some((e) => e.target === 'tag-and-vec' && e.type === 'tag')).toBe(false);
    expect(withTagHidden.edges.some((e) => e.target === 'tag-and-vec' && e.type === 'vector')).toBe(true);
  });

  it('joins two non-anchor chain members with a chain arrow when the older one\'s supersededBy is the newer id, never duplicating an anchor star edge', () => {
    const older = candidate('older-member', {
      types: ['supersession'],
      signedDepth: -2,
      memory: { supersededBy: 'newer-member' }
    });
    const newer = candidate('newer-member', { types: ['supersession'], signedDepth: -1 });
    const model = emptyModel([older, newer]);
    const membership = visibleMembership(model, { hiddenTypes: new Set(), vectorExpanded: false });
    const chainEdges = membership.edges.filter((e) => e.chain);
    expect(chainEdges).toHaveLength(1);
    expect(chainEdges[0]).toMatchObject({ source: 'older-member', target: 'newer-member', type: 'supersession', arrow: true });
    // No chain edge ever touches the anchor -- that relationship is already
    // covered by the star edges.
    expect(chainEdges.every((e) => e.source !== 'anchor-id' && e.target !== 'anchor-id')).toBe(true);
  });

  it('hiding supersession drops chain arrows between members still drawn through another type', () => {
    const older = candidate('older-member', {
      types: ['supersession', 'tag'],
      signedDepth: -2,
      memory: { supersededBy: 'newer-member' }
    });
    const newer = candidate('newer-member', { types: ['supersession', 'tag'], signedDepth: -1 });
    const model = emptyModel([older, newer]);
    const membership = visibleMembership(model, { hiddenTypes: new Set(['supersession']), vectorExpanded: false });
    expect(membership.nodes.map((n) => n.id).sort()).toEqual(['anchor-id', 'newer-member', 'older-member']);
    expect(membership.edges.some((e) => e.type === 'supersession')).toBe(false);
  });

  it('zero candidates yield one node (the anchor) and no edges', () => {
    const model = emptyModel([]);
    const membership = visibleMembership(model, { hiddenTypes: new Set(), vectorExpanded: false });
    expect(membership.nodes).toHaveLength(1);
    expect(membership.nodes[0].isAnchor).toBe(true);
    expect(membership.edges).toHaveLength(0);
  });
});

describe('nodeAccessibleName', () => {
  it('formats a candidate as "{shortId}, {category}, {types joined by \' and \'}, {states}"', () => {
    const name = nodeAccessibleName({
      shortId: 'q7kf2m9x0c',
      category: 'decision',
      isAnchor: false,
      types: ['tag', 'vector'],
      states: ['superseded']
    });
    expect(name).toBe('q7kf2m9x0c, decision, tag and vector, superseded');
  });

  it('formats the anchor as "{shortId}, {category}, anchor"', () => {
    const name = nodeAccessibleName({
      shortId: 'anchor0001',
      category: 'gotcha',
      isAnchor: true,
      types: [],
      states: []
    });
    expect(name).toBe('anchor0001, gotcha, anchor');
  });
});

describe('neighbourhoodSummary', () => {
  it('counts non-anchor nodes per visible type', () => {
    const nodes: GraphNode[] = [
      { id: 'anchor-id', shortId: 'anchor0001', category: 'gotcha', summary: '', isAnchor: true, types: [], states: [], name: '' },
      { id: 'n1', shortId: 'n1', category: 'tag', summary: '', isAnchor: false, types: ['tag' as LaneType], states: [], name: '' },
      {
        id: 'n2',
        shortId: 'n2',
        category: 'tag',
        summary: '',
        isAnchor: false,
        types: ['tag' as LaneType, 'vector' as LaneType],
        states: [],
        name: ''
      }
    ];
    expect(neighbourhoodSummary(nodes)).toBe('2 related · supersession 0 · citation 0 · tag 2 · vector 1');
  });
});

describe('callLineParts', () => {
  it('carries truncated=true when the response did', () => {
    const model = emptyModel([], { truncated: true });
    const parts = callLineParts(model);
    expect(parts.truncatedText).toBe('truncated=true');
  });
});

describe('settleLayout', () => {
  it('is deterministic: two runs over the same input yield identical positions', () => {
    const nodes: GraphNode[] = [
      { id: 'anchor-id', shortId: 'anchor0001', category: 'gotcha', summary: '', isAnchor: true, types: [], states: [], name: '' },
      { id: 'n1', shortId: 'n1', category: 'tag', summary: '', isAnchor: false, types: ['tag' as LaneType], states: [], name: '' }
    ];
    const edges = graphEdges('anchor-id', [candidate('n1', { types: ['tag'], strength: { tag: 1 } })]);
    const run1 = settleLayout(nodes, edges).simNodes.map((n) => ({ id: n.id, x: n.x, y: n.y }));
    const run2 = settleLayout(nodes, edges).simNodes.map((n) => ({ id: n.id, x: n.x, y: n.y }));
    expect(run1).toEqual(run2);
  });

  it('fixes the anchor at the origin', () => {
    const nodes: GraphNode[] = [{ id: 'anchor-id', shortId: 'anchor0001', category: 'gotcha', summary: '', isAnchor: true, types: [], states: [], name: '' }];
    const { simNodes } = settleLayout(nodes, []);
    expect(simNodes[0].x).toBe(0);
    expect(simNodes[0].y).toBe(0);
  });
});

describe('lcg', () => {
  it('produces the same sequence for the same seed', () => {
    const a = lcg(42);
    const b = lcg(42);
    expect([a(), a(), a()]).toEqual([b(), b(), b()]);
  });
});
