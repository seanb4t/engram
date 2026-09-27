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

// D-06's verbatim coverage clauses — extracted here (rather than left inline
// in rankedHeaderParts) so rulesHeaderParts below reuses the exact same
// strings; rankedHeaderParts' own output is unchanged (byte-identical).
const SCOPES_TRUNCATED_CLAUSE = '· scopes_truncated: scope list incomplete';
const SCOPES_UNKNOWN_CLAUSE = '· scopes_unknown: scope coverage could not be listed';

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
    parts.push({ kind: 'coverage', text: SCOPES_TRUNCATED_CLAUSE });
  }
  if (input.scopesUnknown) {
    parts.push({ kind: 'coverage', text: SCOPES_UNKNOWN_CLAUSE });
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

// D-09's unranked listing header: "Latest N memories across M scopes ·
// unranked (list — no score)" plus a hidden clause summed across every
// loaded page (D-02 list-mode display) — 'hidden count unavailable' when any
// loaded page's count could not be computed, since summing past an unknown
// value would silently understate it.
export interface ListingHeaderInput {
  total: number;
  scopes: number;
  hiddenPages: (HiddenCounts | undefined)[];
}

export function listingHeaderParts(input: ListingHeaderInput): HeaderPart[] {
  const parts: HeaderPart[] = [];
  parts.push({ kind: 'count', text: `Latest ${input.total} ${plural(input.total, 'memory', 'memories')}` });
  parts.push({ kind: 'scopes', text: `across ${input.scopes} ${plural(input.scopes, 'scope', 'scopes')}` });
  parts.push({ kind: 'ranking', text: '· unranked (list — no score)' });

  if (input.hiddenPages.some((p) => p === undefined)) {
    parts.push({ kind: 'hidden', text: '· hidden count unavailable' });
  } else {
    const sum = input.hiddenPages.reduce((acc, p) => acc + (p?.total ?? 0), 0);
    if (sum > 0) {
      parts.push({ kind: 'hidden', text: `· ${sum} hidden by recall gate`, title: 'counted within each loaded page' });
    }
  }

  return parts;
}

// D-12's /rules header: "N rules across M scopes", the same cross-spine
// coverage clauses as rankedHeaderParts (verbatim, never inferred). Rules has
// no free-text query, so there is no query clause here.
export interface RulesHeaderInput {
  count: number;
  scopeCount: number;
  scopesTruncated?: boolean;
  scopesUnknown?: boolean;
}

export function rulesHeaderParts(input: RulesHeaderInput): HeaderPart[] {
  const parts: HeaderPart[] = [];
  parts.push({ kind: 'count', text: `${input.count} ${plural(input.count, 'rule', 'rules')}` });

  const scopesText = input.scopesUnknown
    ? 'across every readable scope'
    : `across ${input.scopeCount} ${plural(input.scopeCount, 'scope', 'scopes')}`;
  parts.push({ kind: 'scopes', text: scopesText });

  if (input.scopesTruncated) {
    parts.push({ kind: 'coverage', text: SCOPES_TRUNCATED_CLAUSE });
  }
  if (input.scopesUnknown) {
    parts.push({ kind: 'coverage', text: SCOPES_UNKNOWN_CLAUSE });
  }

  return parts;
}

// D-12's honest-feedback empty state for /rules — fixed, cross-spine only
// (Rules has no free-text query to name, unlike emptyHeading above).
export function rulesEmptyHeading(): string {
  return 'No rules in any scope you can read';
}

// D-13's /scheduled tab state word — shared by scheduledHeaderParts and
// scheduledEmptyHeading so the two never drift: the `all` tab reads
// "windowed" in copy, never "all".
export type ScheduledTabState = 'scheduled' | 'expired' | 'all';

const SCHEDULED_STATE_WORD: Record<ScheduledTabState, string> = {
  scheduled: 'scheduled',
  expired: 'expired',
  all: 'windowed'
};

// D-13's /scheduled header: "N {state} memories across M scopes", with an
// optional "· scroll for more" clause while another cursor page exists,
// following the listing-header pattern (researcher default — not in the
// UI-SPEC verbatim, per the plan's flagged assumption).
export interface ScheduledHeaderInput {
  state: ScheduledTabState;
  count: number;
  scopeCount: number;
  more?: boolean;
}

export function scheduledHeaderParts(input: ScheduledHeaderInput): HeaderPart[] {
  const parts: HeaderPart[] = [];
  const stateWord = SCHEDULED_STATE_WORD[input.state];
  parts.push({ kind: 'count', text: `${input.count} ${stateWord} ${plural(input.count, 'memory', 'memories')}` });
  parts.push({ kind: 'scopes', text: `across ${input.scopeCount} ${plural(input.scopeCount, 'scope', 'scopes')}` });
  if (input.more) {
    parts.push({ kind: 'text', text: '· scroll for more' });
  }
  return parts;
}

// D-13's per-tab empty state — each tab states its own honest empty line,
// never a shared generic one.
export function scheduledEmptyHeading(state: ScheduledTabState): string {
  return `No ${SCHEDULED_STATE_WORD[state]} memories in any scope you can read`;
}
