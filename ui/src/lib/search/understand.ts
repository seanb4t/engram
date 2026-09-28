import type { FilterSuggestion } from '$lib/gen/engram_pb';
import type { Classified } from './classify';
import type { SearchParams } from './params';

// NLQ-03/D-10/D-11: the pure eligibility/query-key/request/label/accept-path
// module for the Suggested row. Task 1 wires the category kind end to end;
// Task 2 completes the remaining kinds (time_window/scope/tag), the hide
// rule, dismissal filtering and the announcement text.

// D-10: fires only for a committed q the classifier reads as `text` with
// zero operator chips and 2+ words.
export function understandEligible(c: Classified): boolean {
  return c.kind === 'text' && c.chips.length === 0 && c.text.trim().split(/\s+/).length >= 2;
}

// Bare q — never the merged `effective` params — per the query-key pitfall
// note (06-UI-SPEC.md): filters ride the request, not the key.
export function understandQueryKey(q: string) {
  return ['understandQuery', q] as const;
}

export function understandQueryRequest(text: string, p: SearchParams) {
  return {
    query: text,
    scope: p.scope,
    // Never both: a non-empty scope always forces crossSpine false, same
    // rule searchMemoriesRequest/listMemoriesRequest already enforce.
    crossSpine: !p.scope && p.crossSpine,
    categories: p.categories,
    tags: p.tags,
    createdAfter: p.createdAfter,
    createdBefore: p.createdBefore
  };
}

export function suggestionLabel(s: FilterSuggestion): string {
  switch (s.kind.case) {
    case 'category':
      return s.kind.value;
    case 'timeWindow':
      return s.kind.value.label;
    case 'scope':
      return s.kind.value;
    case 'tag':
      return `#${s.kind.value}`;
    default:
      return '';
  }
}

// The dismissal-set / hide-rule identity for one suggestion. There is at
// most one time_window suggestion per response (D-03), so the literal
// string is a stable, sufficient key for that kind.
export function suggestionKey(s: FilterSuggestion): string {
  switch (s.kind.case) {
    case 'category':
      return `category:${s.kind.value}`;
    case 'timeWindow':
      return 'time_window';
    case 'scope':
      return `scope:${s.kind.value}`;
    case 'tag':
      return `tag:${s.kind.value}`;
    default:
      return '';
  }
}

// D-12: whether `s` is already reflected in the currently-applied (merged
// `effective`) params — a scope/time_window suggestion hides on ANY applied
// scope/window, not just an exact-value match, since the server only asked
// because nothing was applied at request time.
export function isApplied(s: FilterSuggestion, p: SearchParams): boolean {
  switch (s.kind.case) {
    case 'category':
      return p.categories.includes(s.kind.value);
    case 'tag':
      return p.tags.includes(s.kind.value);
    case 'scope':
      return !!p.scope;
    case 'timeWindow':
      return !!(p.createdAfter || p.createdBefore);
    default:
      return false;
  }
}

// D-12: the surviving suggestions in server response order — never
// re-sorted — filtering out anything already applied or already dismissed
// (keyed by suggestionKey, scoped by the caller to the current q).
export function visibleSuggestions(
  list: FilterSuggestion[],
  p: SearchParams,
  dismissed: ReadonlySet<string>
): FilterSuggestion[] {
  return list.filter((s) => {
    const key = suggestionKey(s);
    if (!key) return false;
    if (isApplied(s, p)) return false;
    if (dismissed.has(key)) return false;
    return true;
  });
}

// D-11: the exact partial FacetStrip's own manual controls would produce for
// the same value, so an accepted suggestion is indistinguishable from a
// manually added chip.
export function acceptPartial(s: FilterSuggestion, p: SearchParams): Partial<SearchParams> {
  switch (s.kind.case) {
    case 'category': {
      const v = s.kind.value;
      return { categories: p.categories.includes(v) ? p.categories : [...p.categories, v] };
    }
    case 'timeWindow':
      return { createdAfter: s.kind.value.createdAfter, createdBefore: s.kind.value.createdBefore };
    case 'scope':
      return { scope: s.kind.value, crossSpine: false };
    case 'tag': {
      const v = s.kind.value;
      return { tags: p.tags.includes(v) ? p.tags : [...p.tags, v] };
    }
    default:
      return {};
  }
}

// The screen-reader-only row-appearance announcement (count only, never the
// labels — the roving toolbar itself is the per-label equivalent once a
// keyboard/screen-reader user tabs in).
export function suggestionAnnouncement(n: number): string {
  return `${n} suggested filter${n === 1 ? '' : 's'}`;
}
