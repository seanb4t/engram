// Pure tag slicing, ranking and honesty copy shared by TagBars (TAGS-01) and
// TagCombobox (TAGS-02). No network calls live here -- everything is a pure
// function over an already-fetched ListTagsResponse (or its toTagRows()
// projection), so it is trivially unit-testable in node (tags.test.ts).
import type { ListTagsResponse } from '$lib/gen/engram_pb';
import type { ParsedConnectError } from '$lib/errors/connect-error';

export interface TagRow {
  tag: string;
  count: number;
}

/** Projects a ListTagsResponse into plain-number rows, preserving server
 * order (count descending, then tag ascending) -- callers never re-sort. */
export function toTagRows(resp: ListTagsResponse | undefined): TagRow[] {
  if (!resp) return [];
  return resp.tags.map((t) => ({ tag: t.tag, count: Number(t.count) }));
}

/** Linear bar width as a percentage of the largest loaded count, from zero. */
export function barPercent(count: number, max: number): number {
  if (max <= 0) return 0;
  return (count / max) * 100;
}

/** Default number of rows TagBars shows before "show all" (D-15). */
export const TAG_TOP = 30;

export interface VisibleTagRows {
  rows: TagRow[];
  hidden: number;
}

/** Filters (case-insensitive substring, order-preserving) then, only when
 * unfiltered and collapsed, slices to TAG_TOP. A filter never truncates --
 * the E4 "partial" state shows every match, not just the first 30. */
export function visibleTagRows(rows: TagRow[], { filter, expanded }: { filter: string; expanded: boolean }): VisibleTagRows {
  const f = filter.trim().toLowerCase();
  const matched = f ? rows.filter((r) => r.tag.toLowerCase().includes(f)) : rows;
  if (f || expanded) {
    return { rows: matched, hidden: 0 };
  }
  const visible = matched.slice(0, TAG_TOP);
  return { rows: visible, hidden: Math.max(0, matched.length - TAG_TOP) };
}

export interface TagListFooter {
  text: string;
  warn: boolean;
}

/** D-15's honesty rule: ListTags carries no total, so the footer only ever
 * says how many were LOADED, never "of N". Priority: a filtered, incomplete
 * list beats the collapsed-by-default footer (filtering already ignores the
 * top-30 cap, so the relevant honesty fact is about the underlying 1000-tag
 * fetch, not the 30-row default). */
export function tagListFooter({
  loaded,
  more,
  filtered,
  expanded
}: {
  loaded: number;
  more: boolean;
  filtered: boolean;
  expanded: boolean;
}): TagListFooter | null {
  const n = loaded.toLocaleString('en-US');
  if (filtered) {
    if (more) return { text: `matching among the ${n} most-used tags`, warn: true };
    return null;
  }
  if (!expanded && loaded > TAG_TOP) {
    return { text: 'showing the 30 most-used tags', warn: false };
  }
  if (expanded && more) {
    return { text: `showing the ${n} most-used tags — more exist`, warn: true };
  }
  return null;
}

/** The scope label rendered in copy -- the raw scope, or the friendly
 * "all readable scopes" for the empty (every-readable-scope) sentinel. */
export function scopeLabel(scope: string): string {
  return scope || 'all readable scopes';
}

export function tagsLoadingLine(scope: string): string {
  return `ListTags(scope="${scope}", limit=1000) in flight▍`;
}

export type TagsErrorCopy =
  | { kind: 'rejected'; heading: string; envelope: string }
  | { kind: 'opaque'; heading: string };

/** ListTags failure copy -- rejected envelopes are rendered verbatim
 * (never paraphrased); everything else is the opaque "nothing was listed"
 * line naming the RPC and the returned code, matching the console's
 * honest-feedback convention for recall surfaces. */
export function tagsErrorCopy(parsed: ParsedConnectError): TagsErrorCopy {
  if (parsed.kind === 'rejected') {
    return {
      kind: 'rejected',
      heading: 'Server rejected the request',
      envelope: `field=${parsed.fields.join(',')} hint=${parsed.hint}: ${parsed.detail}`
    };
  }
  const codeName = parsed.kind === 'opaque' ? parsed.codeName : 'unknown';
  return { kind: 'opaque', heading: `ListTags failed — nothing was listed. list_tags returned code=${codeName}` };
}

/** Max match rows the "+ tag" picker / header Tags group ever show (D-18). */
export const MATCH_TOP = 8;

export interface TagMatchPart {
  text: string;
  hit: boolean;
}

export interface TagMatch {
  tag: string;
  count: number;
  parts: TagMatchPart[];
}

export interface RankedTagMatches {
  matches: TagMatch[];
  total: number;
  unknown: { tag: string; reason: string } | null;
}

/** A leading '#' or 'tag:' is a token-entry artifact, not part of the tag
 * text itself -- strip it before matching (D-18). */
function stripQueryPrefix(raw: string): string {
  let q = raw.trim();
  if (q.startsWith('#')) {
    q = q.slice(1);
  } else if (q.toLowerCase().startsWith('tag:')) {
    q = q.slice('tag:'.length);
  }
  return q.trim();
}

/** Splits a tag into [before, hit, after] parts around a case-insensitive
 * substring match, omitting empty edge parts -- the caller bolds the hit
 * part and renders the rest as plain text nodes (never raw HTML). */
function highlightParts(tag: string, query: string): TagMatchPart[] {
  if (!query) return [{ text: tag, hit: false }];
  const idx = tag.toLowerCase().indexOf(query.toLowerCase());
  if (idx === -1) return [{ text: tag, hit: false }];
  const parts: TagMatchPart[] = [];
  if (idx > 0) parts.push({ text: tag.slice(0, idx), hit: false });
  parts.push({ text: tag.slice(idx, idx + query.length), hit: true });
  if (idx + query.length < tag.length) parts.push({ text: tag.slice(idx + query.length), hit: false });
  return parts;
}

function byCountDescThenTagAsc(a: TagRow, b: TagRow): number {
  return b.count - a.count || a.tag.localeCompare(b.tag);
}

/** Substring match over the loaded (top-1000) rows, ranked prefix matches
 * first then by count (D-18), capped to `limit` (MATCH_TOP by default) with
 * `total` reporting the full match count before capping. A typed tag that
 * is not itself a loaded tag always gets an `unknown` row alongside any
 * real matches -- it is never silently blocked (D-19). */
export function rankTagMatches(
  rows: TagRow[],
  rawQuery: string,
  { more, limit = MATCH_TOP }: { more: boolean; limit?: number }
): RankedTagMatches {
  const query = stripQueryPrefix(rawQuery);

  if (!query) {
    const top = rows.slice(0, limit).map((r) => ({ tag: r.tag, count: r.count, parts: highlightParts(r.tag, '') }));
    return { matches: top, total: rows.length, unknown: null };
  }

  const lower = query.toLowerCase();
  const matched = rows.filter((r) => r.tag.toLowerCase().includes(lower));
  const prefix = matched.filter((r) => r.tag.toLowerCase().startsWith(lower)).sort(byCountDescThenTagAsc);
  const prefixSet = new Set(prefix.map((r) => r.tag));
  const rest = matched.filter((r) => !prefixSet.has(r.tag)).sort(byCountDescThenTagAsc);
  const ordered = [...prefix, ...rest];

  const matches = ordered.slice(0, limit).map((r) => ({ tag: r.tag, count: r.count, parts: highlightParts(r.tag, query) }));
  const exact = rows.some((r) => r.tag.toLowerCase() === lower);
  const unknown = exact
    ? null
    : { tag: query, reason: more ? `#${query} — not among the loaded tags` : `#${query} — 0 recall-visible records` };

  return { matches, total: ordered.length, unknown };
}

export interface MatchFooter {
  text: string;
  warn: string | null;
}

/** No singular special case (E5 zero-one-many backstop) -- the template is
 * identical for 1 through MATCH_TOP matches. */
export function matchFooter({ total, more, loaded }: { total: number; more: boolean; loaded: number }): MatchFooter {
  return {
    text: `${total} matches · top ${MATCH_TOP} shown`,
    warn: more ? `matching among the ${loaded.toLocaleString('en-US')} most-used tags` : null
  };
}
