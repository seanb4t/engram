// Pure tag slicing, ranking and honesty copy shared by TagBars (TAGS-01) and
// TagCombobox (TAGS-02). No network calls live here -- everything is a pure
// function over an already-fetched ListTagsResponse (or its toTagRows()
// projection), so it is trivially unit-testable in node (tags.test.ts).
import type { ListTagsResponse } from '$lib/gen/engram_pb';

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
