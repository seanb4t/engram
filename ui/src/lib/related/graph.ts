// The pure related-neighbourhood model (GRAPH-01, D-01, D-02, D-06, D-09):
// maps one RelatedMemoriesResponse into typed candidates, builds the star
// edge set, and runs a deterministic, synchronous d3-force settle before
// first paint. This module owns no DOM and no fetch -- RelatedGraph.svelte
// and the /related/[id] route render exactly what it computes, never
// deriving membership or layout themselves (mirrors curation/chain.ts +
// ChainDialog.svelte).
//
// Task 1 lays down the full type/constant surface and a working but
// unfiltered visibleMembership; Task 2 completes laneRows, the vector-lane
// collapse, hidden-type filtering, chain arrows and node accessible names.
import type { Memory, RelatedMemoriesResponse, RelatedEdge } from '$lib/gen/engram_pb';
import { EdgeType, SupersessionDirection } from '$lib/gen/engram_pb';
import { compareChainNodes, type ChainNode } from '$lib/curation/chain';
import { memoryStateWords, type RecordStateWord } from '$lib/memorystate';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import {
  forceSimulation,
  forceLink,
  forceManyBody,
  forceCollide,
  forceX,
  forceY,
  type Simulation,
  type SimulationNodeDatum,
  type SimulationLinkDatum
} from 'd3-force';

export type LaneType = 'supersession' | 'citation' | 'tag' | 'vector';

// Canonical lane order everywhere a candidate's types or lanes are shown:
// supersession first, then citation, tag, vector.
export const LANE_ORDER: readonly LaneType[] = ['supersession', 'citation', 'tag', 'vector'];

export const RELATED_K = 64;
export const VECTOR_COLLAPSE = 8;
export const LANE_CAP: Record<LaneType, number> = { supersession: 16, citation: 8, tag: 8, vector: RELATED_K };
export const SETTLE_TICKS = 300;
export const LAYOUT_SEED = 42;
export const LABEL_ALL_MAX = 26;
export const NODE_R = 6;
export const NODE_R_DENSE = 4.2;
export const ANCHOR_R = 10;
export const EDGE_OFFSET = 12;

export const EDGE_STYLE: Record<LaneType, { dash: string; width: number; cap?: 'round' }> = {
  supersession: { dash: '', width: 2 },
  citation: { dash: '6 3', width: 1.5 },
  tag: { dash: '1.5 3.5', width: 1.8, cap: 'round' },
  vector: { dash: '', width: 1 }
};

export interface Candidate {
  id: string;
  shortId: string;
  memory: Memory;
  // canonical-order lane types this candidate was reached through
  types: LaneType[];
  edges: Partial<Record<LaneType, RelatedEdge>>;
  // strength is the vector score, the tag tagWeight, the citation
  // sharedCitations.length -- comparable only within its own lane
  strength: Partial<Record<LaneType, number>>;
  // negative = predecessor (older than the anchor), positive = successor
  // (newer); absent when this candidate carries no supersession edge
  signedDepth?: number;
}

export interface RelatedModel {
  anchor: Memory;
  candidates: Candidate[];
  truncated: boolean;
  k: number;
  counts: Record<LaneType, number>;
}

export interface GraphNode {
  id: string;
  shortId: string;
  category: string;
  summary: string;
  isAnchor: boolean;
  // visible types only
  types: LaneType[];
  states: RecordStateWord[];
  name: string;
  score?: number;
}

export interface GraphEdge {
  key: string;
  source: string;
  target: string;
  type: LaneType;
  offset: number;
  arrow: boolean;
  chain: boolean;
}

export interface Membership {
  lanes: Record<LaneType, Candidate[]>;
  vectorTotal: number;
  vectorCollapsed: boolean;
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface CallLineParts {
  before: string;
  truncatedText: string;
  after: string;
}

function laneTypeFor(edge: RelatedEdge): LaneType | undefined {
  switch (edge.type) {
    case EdgeType.SUPERSESSION:
      return 'supersession';
    case EdgeType.CITATION:
      return 'citation';
    case EdgeType.TAG:
      return 'tag';
    case EdgeType.VECTOR:
      return 'vector';
    default:
      return undefined;
  }
}

// buildRelatedModel maps a RelatedMemoriesResponse into typed candidates,
// each with the canonical-order lane types it was reached through, a
// per-lane strength pulled from that lane's own evidence case, and a signed
// supersession depth. Call only once resp.anchor is known to be present --
// the not-found/error path never reaches this function.
export function buildRelatedModel(resp: RelatedMemoriesResponse, k: number): RelatedModel {
  const counts: Record<LaneType, number> = { supersession: 0, citation: 0, tag: 0, vector: 0 };
  const candidates: Candidate[] = [];
  for (const rel of resp.related) {
    if (!rel.memory) continue;
    const types: LaneType[] = [];
    const edges: Partial<Record<LaneType, RelatedEdge>> = {};
    const strength: Partial<Record<LaneType, number>> = {};
    let signedDepth: number | undefined;
    for (const edge of rel.edges) {
      const lane = laneTypeFor(edge);
      if (!lane) continue;
      types.push(lane);
      edges[lane] = edge;
      counts[lane]++;
      if (edge.evidence.case === 'vector') {
        strength.vector = edge.evidence.value.score;
      } else if (edge.evidence.case === 'tag') {
        strength.tag = edge.evidence.value.tagWeight;
      } else if (edge.evidence.case === 'citation') {
        strength.citation = edge.evidence.value.sharedCitations.length;
      } else if (edge.evidence.case === 'supersession') {
        const { direction, depth } = edge.evidence.value;
        signedDepth = direction === SupersessionDirection.PREDECESSOR ? -depth : depth;
      }
    }
    types.sort((a, b) => LANE_ORDER.indexOf(a) - LANE_ORDER.indexOf(b));
    candidates.push({ id: rel.memory.id, shortId: rel.memory.shortId, memory: rel.memory, types, edges, strength, signedDepth });
  }
  return { anchor: resp.anchor as Memory, candidates, truncated: resp.truncated, k, counts };
}

// A candidate reached by more than one type draws an offset curve per type
// (D-06) using a fixed per-type offset, not a per-index one -- the same
// three-type candidate always yields the same three offsets regardless of
// canonical-order position, matching sketch 006's ES table.
const MULTI_OFFSET: Partial<Record<LaneType, number>> = { tag: EDGE_OFFSET, vector: -EDGE_OFFSET };

// graphEdges returns the star edge set (D-06): one edge per candidate per
// edge type that reached it. A supersession edge points from the OLDER
// record to the newer one and carries an arrowhead; every other type is
// non-directional (source is always the anchor). Chain (superseded_by)
// arrows between two non-anchor candidates are added later, in
// visibleMembership (Task 2) -- this function draws only the anchor star.
export function graphEdges(anchorId: string, candidates: readonly Candidate[]): GraphEdge[] {
  const edges: GraphEdge[] = [];
  for (const c of candidates) {
    const multi = c.types.length > 1;
    for (const type of c.types) {
      const offset = multi ? (MULTI_OFFSET[type] ?? 0) : 0;
      let source = anchorId;
      let target = c.id;
      const arrow = type === 'supersession';
      if (type === 'supersession' && c.signedDepth !== undefined && c.signedDepth < 0) {
        // predecessor: the candidate is older than the anchor -- the arrow
        // points from the older record (candidate) to the newer (anchor).
        source = c.id;
        target = anchorId;
      }
      edges.push({ key: `${source}:${target}:${type}`, source, target, type, offset, arrow, chain: false });
    }
  }
  return edges;
}

// lcg is the sketch's one-line linear-congruential PRNG, used as d3-force's
// randomSource so the same input graph always settles to the same layout
// (D-09) -- required for both deterministic renders and screenshot tests.
export function lcg(seed: number): () => number {
  let s = seed >>> 0;
  return () => {
    s = (s * 1664525 + 1013904223) >>> 0;
    return s / 4294967296;
  };
}

export interface SimNode extends GraphNode, SimulationNodeDatum {}

interface SimLink extends SimulationLinkDatum<SimNode> {
  type: LaneType;
}

export interface SettleResult {
  simulation: Simulation<SimNode, SimLink>;
  simNodes: SimNode[];
}

const LINK_DISTANCE: Record<Exclude<LaneType, 'vector'>, number> = { supersession: 48, citation: 70, tag: 82 };

function linkDistance(l: SimLink): number {
  if (l.type !== 'vector') return LINK_DISTANCE[l.type];
  const target = l.target as SimNode;
  const score = target.score ?? 0.7;
  return 96 + (1 - score) * 180;
}

function linkStrength(l: SimLink): number {
  return l.type === 'supersession' ? 0.9 : 0.35;
}

// settleLayout runs the sketch-006 force parameters (link distance/strength
// per type, charge, collide, gentle x/y centring) to a fixed, seeded
// SETTLE_TICKS-tick settle synchronously, before Svelte ever renders a node
// position (D-09) -- there is no tick-driven animation for the initial
// layout, only `simulation.tick()` called SETTLE_TICKS times in a plain
// loop. The anchor is fixed at the origin for the whole settle.
export function settleLayout(nodes: readonly GraphNode[], edges: readonly GraphEdge[]): SettleResult {
  const simNodes: SimNode[] = nodes.map((n) => ({ ...n }));
  const simLinks: SimLink[] = edges.map((e) => ({ source: e.source, target: e.target, type: e.type }));
  const n = simNodes.length;
  const labelsAll = n <= LABEL_ALL_MAX;
  const charge = n > LABEL_ALL_MAX ? -28 : -70;

  for (const node of simNodes) {
    if (node.isAnchor) {
      node.fx = 0;
      node.fy = 0;
    }
  }

  const simulation = forceSimulation(simNodes)
    .randomSource(lcg(LAYOUT_SEED))
    .force(
      'link',
      forceLink<SimNode, SimLink>(simLinks)
        .id((d) => d.id)
        .distance(linkDistance)
        .strength(linkStrength)
    )
    .force('charge', forceManyBody().strength(charge))
    .force('collide', forceCollide<SimNode>((d) => (d.isAnchor ? ANCHOR_R : NODE_R) + (labelsAll ? 7 : 2)))
    .force('x', forceX(0).strength(0.03))
    .force('y', forceY(0).strength(0.03))
    .stop();

  for (let i = 0; i < SETTLE_TICKS; i++) simulation.tick();

  return { simulation, simNodes };
}

// laneRows sorts one lane by its own strength: supersession by signedDepth
// ascending (oldest predecessor first, then successors), ties broken by
// compareChainNodes (createdAt ascending, then id ascending) -- the same
// chronological tiebreak the supersession chain dialog uses. Every other
// lane sorts by strength descending (highest first), ties by id ascending.
// Scores are never compared across lanes (Phase 1 D-06..D-16).
export function laneRows(model: RelatedModel, type: LaneType): Candidate[] {
  const rows = model.candidates.filter((c) => c.types.includes(type));
  if (type === 'supersession') {
    return rows.slice().sort((a, b) => {
      const ad = a.signedDepth ?? 0;
      const bd = b.signedDepth ?? 0;
      if (ad !== bd) return ad - bd;
      const an: Pick<ChainNode, 'id' | 'createdAt'> = { id: a.id, createdAt: a.memory.createdAt ? timestampDate(a.memory.createdAt) : undefined };
      const bn: Pick<ChainNode, 'id' | 'createdAt'> = { id: b.id, createdAt: b.memory.createdAt ? timestampDate(b.memory.createdAt) : undefined };
      return compareChainNodes(an as ChainNode, bn as ChainNode);
    });
  }
  return rows.slice().sort((a, b) => {
    const as = a.strength[type] ?? 0;
    const bs = b.strength[type] ?? 0;
    if (as !== bs) return bs - as;
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
  });
}

// nodeAccessibleName is the single source of a node's accessible name,
// shared by the graph's role="option" aria-label and the visually hidden
// neighbourhood list (D-11): "{short_id}, {category}, {types joined by
// ' and '}[, {states}]"; the anchor form omits the types list in favour of
// the literal word "anchor".
export function nodeAccessibleName(input: {
  shortId: string;
  category: string;
  isAnchor: boolean;
  types: LaneType[];
  states: readonly string[];
}): string {
  const statesPart = input.states.length > 0 ? `, ${input.states.join(', ')}` : '';
  const middle = input.isAnchor ? 'anchor' : input.types.join(' and ');
  return `${input.shortId}, ${input.category}, ${middle}${statesPart}`;
}

// neighbourhoodSummary is the aria-live announcement text (D-11): counts
// every non-anchor node's VISIBLE types (post hidden-type/vector-collapse
// filtering), never the raw model counts -- it describes what is actually
// drawn, not what the server returned.
export function neighbourhoodSummary(nodes: readonly GraphNode[]): string {
  const nonAnchor = nodes.filter((n) => !n.isAnchor);
  const counts: Record<LaneType, number> = { supersession: 0, citation: 0, tag: 0, vector: 0 };
  for (const n of nonAnchor) {
    for (const t of n.types) counts[t]++;
  }
  return (
    `${nonAnchor.length} related · supersession ${counts.supersession} · citation ${counts.citation}` +
    ` · tag ${counts.tag} · vector ${counts.vector}`
  );
}

// visibleMembership is the single membership result the lanes, the graph
// and the aria-live summary all read (D-07: "graph and lanes always show
// the same membership"). Hidden types empty their lane entirely; the vector
// lane additionally collapses to VECTOR_COLLAPSE candidates unless
// vectorExpanded. A candidate with no visible lane is dropped from `nodes`
// by construction (it never appears in any lanes[] array). Chain
// (superseded_by) arrows are added between two DRAWN non-anchor candidates
// only -- never touching the anchor, which is already covered by the star.
export function visibleMembership(
  model: RelatedModel,
  opts: { hiddenTypes: ReadonlySet<LaneType>; vectorExpanded: boolean }
): Membership {
  const vectorAll = laneRows(model, 'vector');
  const vectorHidden = opts.hiddenTypes.has('vector');
  const vectorTotal = vectorAll.length;
  const vectorCollapsed = !vectorHidden && !opts.vectorExpanded && vectorTotal > VECTOR_COLLAPSE;
  const vectorRows = vectorHidden ? [] : opts.vectorExpanded ? vectorAll : vectorAll.slice(0, VECTOR_COLLAPSE);

  const lanes: Record<LaneType, Candidate[]> = {
    supersession: opts.hiddenTypes.has('supersession') ? [] : laneRows(model, 'supersession'),
    citation: opts.hiddenTypes.has('citation') ? [] : laneRows(model, 'citation'),
    tag: opts.hiddenTypes.has('tag') ? [] : laneRows(model, 'tag'),
    vector: vectorRows
  };

  const seen = new Set<string>();
  const orderedCandidates: Candidate[] = [];
  for (const type of LANE_ORDER) {
    for (const c of lanes[type]) {
      if (!seen.has(c.id)) {
        seen.add(c.id);
        orderedCandidates.push(c);
      }
    }
  }

  function visibleTypesFor(id: string): LaneType[] {
    return LANE_ORDER.filter((t) => lanes[t].some((c) => c.id === id));
  }

  const anchorStates = memoryStateWords(model.anchor);
  const anchorNode: GraphNode = {
    id: model.anchor.id,
    shortId: model.anchor.shortId,
    category: model.anchor.category,
    summary: model.anchor.summary,
    isAnchor: true,
    types: [],
    states: anchorStates,
    name: nodeAccessibleName({ shortId: model.anchor.shortId, category: model.anchor.category, isAnchor: true, types: [], states: anchorStates })
  };

  const candidateNodes: GraphNode[] = orderedCandidates.map((c) => {
    const types = visibleTypesFor(c.id);
    const states = memoryStateWords(c.memory);
    return {
      id: c.id,
      shortId: c.shortId,
      category: c.memory.category,
      summary: c.memory.summary,
      isAnchor: false,
      types,
      states,
      name: nodeAccessibleName({ shortId: c.shortId, category: c.memory.category, isAnchor: false, types, states }),
      score: c.strength.vector
    };
  });

  // Star edges: recompute per-candidate offsets from the VISIBLE type set
  // only, so hiding one of a multi-type candidate's edges also collapses
  // its offset back to 0 when only one type remains visible.
  const visibleCandidatesForEdges: Candidate[] = orderedCandidates.map((c) => ({ ...c, types: visibleTypesFor(c.id) }));
  const starEdges = graphEdges(model.anchor.id, visibleCandidatesForEdges);

  const drawnIds = new Set(orderedCandidates.map((c) => c.id));
  const chainEdges: GraphEdge[] = [];
  for (const c of orderedCandidates) {
    const supersededBy = c.memory.supersededBy;
    if (supersededBy && drawnIds.has(supersededBy)) {
      chainEdges.push({
        key: `${c.id}:${supersededBy}:supersession:chain`,
        source: c.id,
        target: supersededBy,
        type: 'supersession',
        offset: 0,
        arrow: true,
        chain: true
      });
    }
  }

  return {
    lanes,
    vectorTotal,
    vectorCollapsed,
    nodes: [anchorNode, ...candidateNodes],
    edges: [...starEdges, ...chainEdges]
  };
}

// callLineParts renders the mono call line in three pieces so the route can
// colour the `truncated=` word in the warning token without string
// matching: `RelatedMemories(subj, "{short_id}", k={k}) →
// {before}{truncatedText}{after}`. `ms` is the measured request duration
// (Task 3); omitted, `after` is empty. Counts are the raw model totals from
// the server response, never the UI-filtered membership.
export function callLineParts(model: RelatedModel, ms?: number): CallLineParts {
  const before =
    `RelatedMemories(subj, "${model.anchor.shortId}", k=${model.k}) → ${model.candidates.length} related` +
    ` · supersession ${model.counts.supersession} · citation ${model.counts.citation}` +
    ` · tag ${model.counts.tag} · vector ${model.counts.vector} · `;
  const truncatedText = `truncated=${model.truncated}`;
  const after = ms === undefined ? '' : ` · ${ms}ms`;
  return { before, truncatedText, after };
}
