import type { RecallGateHidden } from '$lib/gen/engram_pb';

// The ONE formatter for the /search results header (D-05, D-06, ENTRY-03).
// Every clause's copy is pinned by recall-header.test.ts, including the
// verbatim truncated/unknown wording — never paraphrased at a call site.

export interface HiddenCounts {
  total: number;
  archived: number;
  superseded: number;
  expired: number;
  scheduled: number;
}

// The header's clause vocabulary. `title` carries a clause's tooltip (the
// hidden count's per-state breakdown, a query's full un-ellipsized text).
export type HeaderPart = {
  kind: 'count' | 'scopes' | 'query' | 'ranking' | 'hidden' | 'coverage' | 'text';
  text: string;
  title?: string;
};

// Converts the wire's bigint fields to plain numbers for arithmetic/display.
// Absent (comparison-call failed server-side) stays absent here too — never
// fabricate zeros the server did not report.
export function hiddenFromProto(h?: RecallGateHidden): HiddenCounts | undefined {
  if (!h) return undefined;
  return {
    total: Number(h.total),
    archived: Number(h.archived),
    superseded: Number(h.superseded),
    expired: Number(h.expired),
    scheduled: Number(h.scheduled)
  };
}

export function plural(n: number, one: string, many: string): string {
  return n === 1 ? one : many;
}

// '1 archived, 1 superseded' — nonzero buckets only, in canonical state order.
// A record can carry more than one state simultaneously, so the per-state sum
// can exceed `total` (the count of distinct hidden records); when it does,
// the tooltip says so rather than letting the arithmetic look like a bug.
export function hiddenTooltip(h: HiddenCounts): string {
  const order: [number, string][] = [
    [h.archived, 'archived'],
    [h.superseded, 'superseded'],
    [h.expired, 'expired'],
    [h.scheduled, 'scheduled']
  ];
  const parts = order.filter(([n]) => n > 0).map(([n, word]) => `${n} ${word}`);
  let tooltip = parts.join(', ');
  const sum = h.archived + h.superseded + h.expired + h.scheduled;
  if (sum > h.total) tooltip += ' · a record can carry more than one state';
  return tooltip;
}

export interface RankedHeaderInput {
  hits: number;
  scopeCount: number;
  query: string;
  reranked: boolean;
  hidden?: HiddenCounts;
}

// The ranked happy-path clauses, in the fixed ENTRY-03 order: hits · scopes ·
// query · ranking · hidden. (Task 2 extends this with "of N", coverage
// clauses and id/short_id resolution lines.)
export function rankedHeaderParts(input: RankedHeaderInput): HeaderPart[] {
  const parts: HeaderPart[] = [];
  parts.push({ kind: 'count', text: `${input.hits} ${plural(input.hits, 'hit', 'hits')}` });
  parts.push({ kind: 'scopes', text: `across ${input.scopeCount} ${plural(input.scopeCount, 'scope', 'scopes')}` });
  parts.push({ kind: 'query', text: `for ${input.query}`, title: input.query });
  parts.push({ kind: 'ranking', text: input.reranked ? '· reranked by jev' : '· ranked by cosine' });
  if (input.hidden === undefined) {
    parts.push({ kind: 'hidden', text: '· hidden count unavailable' });
  } else if (input.hidden.total > 0) {
    parts.push({
      kind: 'hidden',
      text: `· ${input.hidden.total} hidden by recall gate`,
      title: hiddenTooltip(input.hidden)
    });
  }
  return parts;
}

// A plain-text rendering of a part list — used by tests and anywhere a flat
// string suffices. ResultsHeader.svelte renders the same `parts` richly
// (bold counts, a mono query span, an expandable scopes button) rather than
// this string.
export function headerText(parts: HeaderPart[]): string {
  return parts.map((p) => p.text).join(' ');
}

export function loadingLine(query: string, scopeCount: number): string {
  return `Searching ${scopeCount} ${plural(scopeCount, 'scope', 'scopes')} for ${query}…`;
}
