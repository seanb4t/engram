// lanes.ts -- pure lane, evidence and empty-state copy for /related's
// edge-type lanes and the evidence section under the graph (GRAPH-01,
// GRAPH-03, D-02, D-05, D-10). EdgeLane.svelte, SupersessionLane.svelte,
// EvidenceSection.svelte and GraphLegend.svelte render exactly what this
// module computes -- it owns no DOM, mirroring graph.ts's split.
import { SupersessionDirection } from '$lib/gen/engram_pb';
import { LANE_CAP, LANE_ORDER, type Candidate, type LaneType, type RelatedModel } from './graph';

export interface EvidenceLine {
  type: LaneType;
  glyph: string;
  label: string;
  text: string;
}

// LANE_GLYPH is the one letter-glyph map every lane row, the evidence
// section and the legend read -- never a second hand-copied mapping.
export const LANE_GLYPH: Record<LaneType, string> = { supersession: 'S', citation: 'C', tag: 'T', vector: 'V' };

// formatCosine/formatWeight are the ONE place a vector score or a tag weight
// is rendered -- the evidence lines and the lane rows both read through
// these so a score never gets a different precision in two places.
export function formatCosine(score: number): string {
  return score.toFixed(3);
}

export function formatWeight(weight: number): string {
  return weight.toFixed(2);
}

// evidenceLines maps a candidate's per-type evidence into the D-05 "why it
// is related" lines, in canonical LANE_ORDER -- only the types that
// actually reached this candidate (E3 partial: never a placeholder line for
// an absent type).
export function evidenceLines(candidate: Candidate): EvidenceLine[] {
  const lines: EvidenceLine[] = [];
  for (const type of LANE_ORDER) {
    const edge = candidate.edges[type];
    if (!edge) continue;
    let text = '';
    if (type === 'supersession' && edge.evidence.case === 'supersession') {
      const { direction, depth } = edge.evidence.value;
      text = `${direction === SupersessionDirection.PREDECESSOR ? 'predecessor' : 'successor'} · depth ${depth}`;
    } else if (type === 'citation' && edge.evidence.case === 'citation') {
      text = edge.evidence.value.sharedCitations.map((c) => `${c.kind} ${c.ref}`).join(' · ');
    } else if (type === 'tag' && edge.evidence.case === 'tag') {
      const { sharedTags, tagWeight } = edge.evidence.value;
      const perTag = sharedTags.map((t) => `#${t.tag} ${formatWeight(t.weight)}`).join(' · ');
      text = `${perTag} · Σ ${formatWeight(tagWeight)}`;
    } else if (type === 'vector' && edge.evidence.case === 'vector') {
      text = `cosine ${formatCosine(edge.evidence.value.score)}`;
    }
    lines.push({ type, glyph: LANE_GLYPH[type], label: type, text });
  }
  return lines;
}

// evidenceHeader is the D-05 section heading -- singular at one edge type,
// plural from two.
export function evidenceHeader(n: number): string {
  return `Why it is related · ${n} edge type${n === 1 ? '' : 's'}`;
}

// laneCaption is the one-line "what this lane measures and its cap" string
// in each lane header (planner-authored copy, not derived from response
// data except vector's k).
export function laneCaption(type: LaneType, k: number): string {
  switch (type) {
    case 'supersession':
      return 'the whole chain both ways, hidden members included · 8 hops per direction · cap 16';
    case 'citation':
      return 'shared citations matched on kind + ref · ranked by count · cap 8';
    case 'tag':
      return 'shared tags, each weighted by rarity ln(n/df) · ranked by Σ · cap 8';
    case 'vector':
      return `raw cosine to the anchor (query by id) · ranked by score · k=${k}`;
  }
}

// laneCountLabel is the mono `n/cap` count in each lane header -- vector's
// cap is the request's own k (which the caller can widen), every other
// lane's cap is its fixed LANE_CAP.
export function laneCountLabel(type: LaneType, model: RelatedModel): string {
  const cap = type === 'vector' ? model.k : LANE_CAP[type];
  return `${model.counts[type]}/${cap}`;
}

// emptyLaneReason names what an empty lane searched and found nothing --
// never a bare "no results" (honest-feedback rule).
export function emptyLaneReason(type: LaneType, model: RelatedModel): string {
  switch (type) {
    case 'supersession':
      return 'none — no superseded_by, no supersedes';
    case 'citation':
      return 'none — no recall-visible record shares a citation (kind + ref)';
    case 'tag':
      return model.anchor.tags.length === 0 ? 'none — anchor carries no tags' : 'none — no recall-visible record shares a tag';
    case 'vector':
      return `none — 0 recall-visible neighbours at k=${model.k}`;
  }
}
