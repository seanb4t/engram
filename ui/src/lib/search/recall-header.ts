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
  // The unfiltered hit count — present only when a category filter is
  // active and actually narrows the total (ROW-05: "12 hits of 20").
  hitsWithoutCategory?: number;
  scopeCount: number;
  query: string;
  reranked: boolean;
  hidden?: HiddenCounts;
  // D-06: verbatim, never inferred. scopesTruncated means the searched_scopes
  // LIST was cut at its ceiling; scopesUnknown means coverage could not be
  // listed at all (the scopes clause itself then reads "every readable
  // scope" rather than a count it cannot back).
  scopesTruncated?: boolean;
  scopesUnknown?: boolean;
}

// The ranked happy-path clauses, in the fixed ENTRY-03 order: hits · scopes ·
// query · ranking · hidden · coverage.
export function rankedHeaderParts(input: RankedHeaderInput): HeaderPart[] {
  const parts: HeaderPart[] = [];
  const hitsWord = plural(input.hits, 'hit', 'hits');
  const countText =
    input.hitsWithoutCategory !== undefined && input.hitsWithoutCategory !== input.hits
      ? `${input.hits} ${hitsWord} of ${input.hitsWithoutCategory}`
      : `${input.hits} ${hitsWord}`;
  parts.push({ kind: 'count', text: countText });

  const scopesText = input.scopesUnknown
    ? 'across every readable scope'
    : `across ${input.scopeCount} ${plural(input.scopeCount, 'scope', 'scopes')}`;
  parts.push({ kind: 'scopes', text: scopesText });

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

  if (input.scopesTruncated) {
    parts.push({ kind: 'coverage', text: '· scopes_truncated: scope list incomplete' });
  }
  if (input.scopesUnknown) {
    parts.push({ kind: 'coverage', text: '· scopes_unknown: scope coverage could not be listed' });
  }

  return parts;
}

// The honest id/short_id outcome line (ENTRY-03/"Honest id outcomes").
// `id`'s `supersededBy` note only fires when the resolved record IS
// superseded — a pure successor with no predecessor of its own carries no
// such note.
export type ResolutionInput =
  | { kind: 'id'; id: string; supersededBy?: string }
  | { kind: 'short_id'; shortId: string }
  | { kind: 'short_id_miss'; q: string };

export function resolutionLine(input: ResolutionInput): string {
  switch (input.kind) {
    case 'id': {
      let line = `Resolved id ${input.id} → 1 memory`;
      if (input.supersededBy) {
        line += ` · this record is superseded → ${input.supersededBy}; hidden from search, fetchable by id`;
      }
      return line;
    }
    case 'short_id':
      return `Resolved short_id ${input.shortId} → 1 memory`;
    case 'short_id_miss':
      return `No short_id attachment. Searched it as text instead: ${input.q}`;
  }
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

// ENTRY-03's empty-state heading: names the query and exactly what was
// searched (cross-spine vs a bounded scope count), plus the hidden-by-gate
// clause only when something was actually hidden — never a bare "no
// results" (recall-surface.md "Empty and error states say what happened").
export interface EmptyHeadingInput {
  query: string;
  crossSpine: boolean;
  scopesSearched: number;
  hidden?: HiddenCounts;
}

export function emptyHeading(input: EmptyHeadingInput): string {
  const base = input.crossSpine
    ? `No memories match ${input.query} in any scope you can read`
    : `No memories match ${input.query} in the ${input.scopesSearched} ${plural(input.scopesSearched, 'scope', 'scopes')} searched`;
  if (input.hidden && input.hidden.total > 0) {
    return `${base} · ${input.hidden.total} hidden by recall gate`;
  }
  return base;
}
