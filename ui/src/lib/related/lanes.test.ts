import { describe, it, expect } from 'vitest';
import { create, type MessageInitShape } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import {
  MemorySchema,
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
import type { Candidate, RelatedModel } from './graph';
import {
  evidenceLines,
  evidenceHeader,
  laneCaption,
  emptyLaneReason,
  supersessionColumns,
  noNeighboursLines,
  collapsedRowCopy,
  TRUNCATION_BANNER,
  formatCosine,
  formatWeight
} from './lanes';

const EARLIER = new Date('2030-01-01T00:00:00Z');
const LATER = new Date('2030-01-02T00:00:00Z');

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

function tagEdge(tag: string, weight: number, tagWeight = weight) {
  return create(RelatedEdgeSchema, {
    type: EdgeType.TAG,
    evidence: { case: 'tag', value: create(TagEvidenceSchema, { sharedTags: [create(WeightedTagSchema, { tag, weight })], tagWeight }) }
  });
}

function citationEdge(refs: Array<[string, string]>) {
  return create(RelatedEdgeSchema, {
    type: EdgeType.CITATION,
    evidence: {
      case: 'citation',
      value: create(CitationEvidenceSchema, { sharedCitations: refs.map(([kind, ref]) => create(CitationRefSchema, { kind, ref })) })
    }
  });
}

function candidate(id: string, opts: Omit<Partial<Candidate>, 'memory'> & { memory?: MessageInitShape<typeof MemorySchema> } = {}): Candidate {
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

function model(candidates: Candidate[], overrides: Partial<RelatedModel> = {}): RelatedModel {
  return {
    anchor: mem('anchor-id'),
    truncated: false,
    k: 64,
    counts: { supersession: 0, citation: 0, tag: 0, vector: 0 },
    candidates,
    ...overrides
  };
}

describe('evidenceLines', () => {
  it('renders a predecessor supersession line with its depth', () => {
    const c = candidate('p1', { types: ['supersession'], edges: { supersession: supersessionEdge(SupersessionDirection.PREDECESSOR, 2) } });
    expect(evidenceLines(c)).toEqual([{ type: 'supersession', glyph: 'S', label: 'supersession', text: 'predecessor · depth 2' }]);
  });

  it('renders a successor supersession line with its depth', () => {
    const c = candidate('s1', { types: ['supersession'], edges: { supersession: supersessionEdge(SupersessionDirection.SUCCESSOR, 1) } });
    expect(evidenceLines(c)[0].text).toBe('successor · depth 1');
  });

  it('renders shared citations joined by " · "', () => {
    const c = candidate('c1', {
      types: ['citation'],
      edges: {
        citation: citationEdge([
          ['file', 'a.go'],
          ['file', 'b.go']
        ])
      }
    });
    expect(evidenceLines(c)[0].text).toBe('file a.go · file b.go');
  });

  it('renders shared tags plus the summed Σ weight', () => {
    const c = candidate('t1', { types: ['tag'], edges: { tag: tagEdge('engram', 2.1, 2.1) } });
    expect(evidenceLines(c)[0].text).toBe('#engram 2.10 · Σ 2.10');
  });

  it('renders the vector cosine score to three decimals', () => {
    const c = candidate('v1', { types: ['vector'], edges: { vector: vectorEdge(0.8194) } });
    expect(evidenceLines(c)[0].text).toBe('cosine 0.819');
  });

  it('orders multiple lines in canonical LANE_ORDER, only for the types that reached the candidate', () => {
    const c = candidate('m1', { types: ['tag', 'vector'], edges: { tag: tagEdge('a', 1, 1), vector: vectorEdge(0.5) } });
    expect(evidenceLines(c).map((l) => l.type)).toEqual(['tag', 'vector']);
  });
});

describe('evidenceHeader', () => {
  it('is singular at one edge type', () => {
    expect(evidenceHeader(1)).toBe('Why it is related · 1 edge type');
  });

  it('is plural from two edge types', () => {
    expect(evidenceHeader(2)).toBe('Why it is related · 2 edge types');
  });
});

describe('laneCaption', () => {
  it('returns the planned copy for every lane, with vector carrying k', () => {
    expect(laneCaption('supersession', 64)).toBe('the whole chain both ways, hidden members included · 8 hops per direction · cap 16');
    expect(laneCaption('citation', 64)).toBe('shared citations matched on kind + ref · ranked by count · cap 8');
    expect(laneCaption('tag', 64)).toBe('shared tags, each weighted by rarity ln(n/df) · ranked by Σ · cap 8');
    expect(laneCaption('vector', 64)).toBe('raw cosine to the anchor (query by id) · ranked by score · k=64');
  });
});

describe('emptyLaneReason', () => {
  it('names what each empty lane searched and found nothing', () => {
    const m = model([]);
    expect(emptyLaneReason('supersession', m)).toBe('none — no superseded_by, no supersedes');
    expect(emptyLaneReason('citation', m)).toBe('none — no recall-visible record shares a citation (kind + ref)');
    expect(emptyLaneReason('vector', m)).toBe('none — 0 recall-visible neighbours at k=64');
  });

  it('distinguishes an anchor with no tags from one whose tags matched nothing', () => {
    const noTags = model([], { anchor: mem('anchor-id', { tags: [] }) });
    expect(emptyLaneReason('tag', noTags)).toBe('none — anchor carries no tags');
    const withTags = model([], { anchor: mem('anchor-id', { tags: ['engram'] }) });
    expect(emptyLaneReason('tag', withTags)).toBe('none — no recall-visible record shares a tag');
  });
});

describe('supersessionColumns', () => {
  it("orders columns by signed depth ascending, anchor at 0, labels '−2', '−1', 'anchor', '+1' (U+2212 minus)", () => {
    const m = model([
      candidate('pred2', { types: ['supersession'], signedDepth: -2 }),
      candidate('pred1', { types: ['supersession'], signedDepth: -1 }),
      candidate('succ1', { types: ['supersession'], signedDepth: 1 })
    ]);
    const cols = supersessionColumns(m);
    expect(cols.map((c) => c.label)).toEqual(['−2', '−1', 'anchor', '+1']);
    expect(cols.map((c) => c.depth)).toEqual([-2, -1, 0, 1]);
    const anchorCol = cols.find((c) => c.depth === 0);
    expect(anchorCol?.cards).toHaveLength(1);
    expect(anchorCol?.cards[0].isAnchor).toBe(true);
    expect(anchorCol?.cards[0].id).toBe('anchor-id');
  });

  it("orders a column's cards with compareChainNodes (createdAt ascending, ties by id)", () => {
    const m = model([
      candidate('later', { types: ['supersession'], signedDepth: -1, memory: { createdAt: timestampFromDate(LATER) } }),
      candidate('earlier', { types: ['supersession'], signedDepth: -1, memory: { createdAt: timestampFromDate(EARLIER) } })
    ]);
    const col = supersessionColumns(m).find((c) => c.depth === -1);
    expect(col?.cards.map((c) => c.id)).toEqual(['earlier', 'later']);
  });
});

describe('TRUNCATION_BANNER', () => {
  it('names the vector lane as the one cut', () => {
    expect(TRUNCATION_BANNER).toBe('▲ truncated=true — total ceiling 64 reached; only the vector lane is cut.');
  });
});

describe('collapsedRowCopy', () => {
  it('renders the untruncated collapsed-row copy', () => {
    expect(collapsedRowCopy({ shown: 8, total: 20, k: 64, truncated: false })).toBe('showing 8 of 20 · k=64 · show all 20 ▸');
  });

  it('inserts the ceiling-cut clause only when truncated', () => {
    expect(collapsedRowCopy({ shown: 8, total: 57, k: 64, truncated: true })).toBe('showing 8 of 57 · k=64 · ceiling cut the rest · show all 57 ▸');
  });
});

describe('noNeighboursLines', () => {
  it("lists the chain walk, citations, the anchor's tags and the vector k", () => {
    const m = model([], { anchor: mem('anchor-id', { tags: ['engram', 'go'] }), k: 64 });
    expect(noNeighboursLines(m)).toEqual([
      'chain walk: no superseded_by, no supersedes',
      'citations: no recall-visible record shares a kind + ref',
      'tags probed: #engram · #go',
      'vector k=64: 0 recall-visible neighbours'
    ]);
  });

  it('says the anchor carries no tags when it has none', () => {
    const m = model([], { anchor: mem('anchor-id', { tags: [] }) });
    expect(noNeighboursLines(m)[2]).toBe('tags probed: none — anchor carries no tags');
  });
});

describe('formatCosine / formatWeight', () => {
  it('formats a cosine score to three decimals and a weight to two', () => {
    expect(formatCosine(0.5)).toBe('0.500');
    expect(formatWeight(1)).toBe('1.00');
  });
});
