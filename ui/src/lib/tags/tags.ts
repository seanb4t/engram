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
