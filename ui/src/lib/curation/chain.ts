// The pure chain model (D-06) built from two RelatedMemories responses: one
// read from the record the dialog was opened for (the anchor) to discover
// the live head via a SUCCESSOR edge, and one read from the head to walk its
// PREDECESSOR edges. ChainDialog.svelte renders this model — it never walks
// the chain itself (key_links: buildChain).
import type { Memory, RelatedMemoriesResponse } from '$lib/gen/engram_pb';
import { EdgeType, SupersessionDirection } from '$lib/gen/engram_pb';
import { timestampDate } from '@bufbuild/protobuf/wkt';

export interface ChainNode {
  id: string;
  shortId: string;
  summary: string;
  category: string;
  kind: string;
  createdAt?: Date;
  depth: number;
  placeholder: boolean;
  supersededBy?: string;
}

export interface ChainModel {
  headId: string;
  anchorId: string;
  anchorDepth: number;
  // Deepest (oldest) column first, head (depth 0) last — oldest-left,
  // head-right, matching the dialog's rendering order.
  columns: ChainNode[][];
  beyondCap: boolean;
}

// headIdFrom scans the anchor's RelatedMemories response for SUCCESSOR-typed
// supersession edges and returns the id of the memory at the greatest depth
// (the record furthest along the forward chain the server returned) — the
// live head as far as this read can see. With no SUCCESSOR edge at all, the
// anchor has no successor and is itself the head.
export function headIdFrom(anchorResp: RelatedMemoriesResponse): string {
  const anchorId = anchorResp.anchor?.id ?? '';
  let bestId = anchorId;
  let bestDepth = -1;
  for (const rel of anchorResp.related) {
    if (!rel.memory) continue;
    for (const edge of rel.edges) {
      if (edge.type === EdgeType.SUPERSESSION && edge.evidence.case === 'supersession' && edge.evidence.value.direction === SupersessionDirection.SUCCESSOR) {
        if (edge.evidence.value.depth > bestDepth) {
          bestDepth = edge.evidence.value.depth;
          bestId = rel.memory.id;
        }
      }
    }
  }
  return bestId;
}

function toNode(m: Memory, depth: number): ChainNode {
  return {
    id: m.id,
    shortId: m.shortId,
    summary: m.summary,
    category: m.category,
    kind: m.kind,
    createdAt: m.createdAt ? timestampDate(m.createdAt) : undefined,
    depth,
    placeholder: false,
    supersededBy: m.supersededBy
  };
}

// compareChainNodes orders nodes within one column: createdAt ascending,
// ties (or a missing createdAt on either side) broken by id ascending — the
// same input always renders the same order (CUR-01).
export function compareChainNodes(a: ChainNode, b: ChainNode): number {
  const at = a.createdAt?.getTime();
  const bt = b.createdAt?.getTime();
  if (at !== undefined && bt !== undefined && at !== bt) return at - bt;
  if (at === undefined && bt !== undefined) return -1;
  if (at !== undefined && bt === undefined) return 1;
  return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}

// buildChain places headResp.anchor at depth 0 and every PREDECESSOR-edged
// memory in headResp.related at its edge depth, groups by depth, sorts each
// group with compareChainNodes, and returns columns deepest-first (index 0
// is the oldest) so the dialog renders oldest-left to head-right.
export function buildChain(headResp: RelatedMemoriesResponse, anchorId: string): ChainModel {
  const headMem = headResp.anchor;
  const headId = headMem?.id ?? '';
  const byDepth = new Map<number, ChainNode[]>();
  if (headMem) byDepth.set(0, [toNode(headMem, 0)]);

  for (const rel of headResp.related) {
    if (!rel.memory) continue;
    for (const edge of rel.edges) {
      if (edge.type === EdgeType.SUPERSESSION && edge.evidence.case === 'supersession' && edge.evidence.value.direction === SupersessionDirection.PREDECESSOR) {
        const depth = edge.evidence.value.depth;
        const arr = byDepth.get(depth) ?? [];
        arr.push(toNode(rel.memory, depth));
        byDepth.set(depth, arr);
      }
    }
  }

  const maxDepth = byDepth.size === 0 ? 0 : Math.max(...byDepth.keys());
  const columns: ChainNode[][] = [];
  for (let depth = maxDepth; depth >= 0; depth--) {
    const nodes = (byDepth.get(depth) ?? []).slice().sort(compareChainNodes);
    if (nodes.length > 0) columns.push(nodes);
  }

  let anchorDepth = 0;
  for (const [depth, nodes] of byDepth) {
    if (nodes.some((n) => n.id === anchorId)) {
      anchorDepth = depth;
      break;
    }
  }

  return { headId, anchorId, anchorDepth, columns, beyondCap: false };
}
