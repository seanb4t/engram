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

// D-11: the exact partial FacetStrip's own manual controls would produce for
// the same value, so an accepted suggestion is indistinguishable from a
// manually added chip.
export function acceptPartial(s: FilterSuggestion, p: SearchParams): Partial<SearchParams> {
  switch (s.kind.case) {
    case 'category': {
      const v = s.kind.value;
      return { categories: p.categories.includes(v) ? p.categories : [...p.categories, v] };
    }
    default:
      return {};
  }
}
