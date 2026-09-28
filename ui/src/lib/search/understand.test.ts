import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { FilterSuggestionSchema, SuggestionSource, type FilterSuggestion } from '$lib/gen/engram_pb';
import { classifyInput } from './classify';
import { defaultSearchParams, type SearchParams } from './params';
import {
  understandEligible,
  understandQueryRequest,
  suggestionKey,
  suggestionLabel,
  isApplied,
  visibleSuggestions,
  acceptPartial,
  suggestionAnnouncement
} from './understand';

function categorySuggestion(value: string): FilterSuggestion {
  return create(FilterSuggestionSchema, { kind: { case: 'category', value }, source: SuggestionSource.DECIDED });
}

function timeWindowSuggestion(createdAfter: string, createdBefore: string, label: string): FilterSuggestion {
  return create(FilterSuggestionSchema, {
    kind: { case: 'timeWindow', value: { createdAfter, createdBefore, label } },
    source: SuggestionSource.DECIDED
  });
}

function scopeSuggestion(value: string): FilterSuggestion {
  return create(FilterSuggestionSchema, { kind: { case: 'scope', value }, source: SuggestionSource.DECIDED });
}

function tagSuggestion(value: string): FilterSuggestion {
  return create(FilterSuggestionSchema, { kind: { case: 'tag', value }, source: SuggestionSource.MATCHED });
}

function undefinedSuggestion(): FilterSuggestion {
  return create(FilterSuggestionSchema, { source: SuggestionSource.DECIDED });
}

function params(overrides: Partial<SearchParams> = {}): SearchParams {
  return { ...defaultSearchParams(), ...overrides };
}

describe('understandEligible', () => {
  it('is true for a 2+ word text query with no operator chips', () => {
    expect(understandEligible(classifyInput('what did we decide'))).toBe(true);
  });

  it('is false for a single-word text query', () => {
    expect(understandEligible(classifyInput('qdrant'))).toBe(false);
  });

  it('is false for an id-classified query', () => {
    expect(understandEligible(classifyInput('753aba22-0000-4000-8000-000000000001'))).toBe(false);
  });

  it('is false for a short_id-classified query', () => {
    expect(understandEligible(classifyInput('k3m9p2qr7a'))).toBe(false);
  });

  it('is false when the query carries any operator chip, even alongside free text', () => {
    expect(understandEligible(classifyInput('fix #ci bug'))).toBe(false);
    expect(understandEligible(classifyInput('scope:repo:x notes'))).toBe(false);
    expect(understandEligible(classifyInput('is:gotcha old stuff'))).toBe(false);
  });

  it('is true for a 2+ word query with collapsed whitespace', () => {
    expect(understandEligible(classifyInput('  two   words '))).toBe(true);
  });
});

describe('understandQueryRequest', () => {
  it('forces crossSpine false whenever a scope is applied', () => {
    const req = understandQueryRequest('what did we decide', params({ scope: 'repo:x', crossSpine: true }));
    expect(req.scope).toBe('repo:x');
    expect(req.crossSpine).toBe(false);
  });

  it('carries crossSpine through unchanged when no scope is applied', () => {
    const req = understandQueryRequest('what did we decide', params({ scope: '', crossSpine: true }));
    expect(req.crossSpine).toBe(true);
  });
});

describe('suggestionKey / suggestionLabel', () => {
  it('keys and labels a category suggestion', () => {
    const s = categorySuggestion('decision');
    expect(suggestionKey(s)).toBe('category:decision');
    expect(suggestionLabel(s)).toBe('decision');
  });

  it('keys time_window as a fixed literal and labels it with the server string verbatim', () => {
    const s = timeWindowSuggestion('2026-09-21T00:00:00Z', '', 'past week');
    expect(suggestionKey(s)).toBe('time_window');
    expect(suggestionLabel(s)).toBe('past week');
  });

  it('keys and labels a scope suggestion with the bare scope string', () => {
    const s = scopeSuggestion('repo:acme/x');
    expect(suggestionKey(s)).toBe('scope:repo:acme/x');
    expect(suggestionLabel(s)).toBe('repo:acme/x');
  });

  it('keys and labels a tag suggestion with a leading #', () => {
    const s = tagSuggestion('qdrant');
    expect(suggestionKey(s)).toBe('tag:qdrant');
    expect(suggestionLabel(s)).toBe('#qdrant');
  });

  it('keys an undefined-case suggestion as the empty string', () => {
    const s = undefinedSuggestion();
    expect(suggestionKey(s)).toBe('');
  });
});

describe('isApplied', () => {
  it('a category suggestion is applied when its value is in effective.categories', () => {
    expect(isApplied(categorySuggestion('decision'), params({ categories: ['decision'] }))).toBe(true);
    expect(isApplied(categorySuggestion('decision'), params({ categories: ['gotcha'] }))).toBe(false);
  });

  it('a tag suggestion is applied when its value is in effective.tags', () => {
    expect(isApplied(tagSuggestion('qdrant'), params({ tags: ['qdrant'] }))).toBe(true);
    expect(isApplied(tagSuggestion('qdrant'), params({ tags: [] }))).toBe(false);
  });

  it('a scope suggestion is applied when any scope is applied, not just an exact match', () => {
    expect(isApplied(scopeSuggestion('repo:acme/x'), params({ scope: 'repo:other' }))).toBe(true);
    expect(isApplied(scopeSuggestion('repo:acme/x'), params({ scope: '' }))).toBe(false);
  });

  it('a time_window suggestion is applied when either created bound is non-empty', () => {
    expect(isApplied(timeWindowSuggestion('2026-09-21T00:00:00Z', '', 'past week'), params({ createdAfter: '2020-01-01T00:00:00Z' }))).toBe(true);
    expect(isApplied(timeWindowSuggestion('2026-09-21T00:00:00Z', '', 'past week'), params({ createdBefore: '2020-01-01T00:00:00Z' }))).toBe(true);
    expect(isApplied(timeWindowSuggestion('2026-09-21T00:00:00Z', '', 'past week'), params())).toBe(false);
  });
});

describe('visibleSuggestions', () => {
  it('drops an already-applied suggestion but keeps original order for the rest', () => {
    const list = [categorySuggestion('decision'), timeWindowSuggestion('2026-09-21T00:00:00Z', '', 'past week'), scopeSuggestion('repo:x'), tagSuggestion('qdrant')];
    const visible = visibleSuggestions(list, params({ categories: ['decision'] }), new Set());
    expect(visible.map(suggestionKey)).toEqual(['time_window', 'scope:repo:x', 'tag:qdrant']);
  });

  it('drops a dismissed key', () => {
    const list = [categorySuggestion('decision'), tagSuggestion('qdrant')];
    const visible = visibleSuggestions(list, params(), new Set(['tag:qdrant']));
    expect(visible.map(suggestionKey)).toEqual(['category:decision']);
  });

  it('never re-sorts the surviving suggestions', () => {
    const list = [tagSuggestion('zeta'), categorySuggestion('decision'), scopeSuggestion('repo:x')];
    const visible = visibleSuggestions(list, params(), new Set());
    expect(visible.map(suggestionKey)).toEqual(['tag:zeta', 'category:decision', 'scope:repo:x']);
  });
});

describe('acceptPartial', () => {
  it('category: unions by inclusion', () => {
    expect(acceptPartial(categorySuggestion('decision'), params({ categories: [] }))).toEqual({
      categories: ['decision']
    });
    expect(acceptPartial(categorySuggestion('decision'), params({ categories: ['decision'] }))).toEqual({
      categories: ['decision']
    });
  });

  it('time_window: carries the RFC3339 bounds through verbatim', () => {
    expect(acceptPartial(timeWindowSuggestion('2026-09-21T00:00:00Z', '', 'past week'), params())).toEqual({
      createdAfter: '2026-09-21T00:00:00Z',
      createdBefore: ''
    });
  });

  it('scope: replaces scope and forces crossSpine false', () => {
    expect(acceptPartial(scopeSuggestion('repo:acme/x'), params({ scope: '', crossSpine: true }))).toEqual({
      scope: 'repo:acme/x',
      crossSpine: false
    });
  });

  it('tag: unions by inclusion', () => {
    expect(acceptPartial(tagSuggestion('qdrant'), params({ tags: [] }))).toEqual({ tags: ['qdrant'] });
    expect(acceptPartial(tagSuggestion('qdrant'), params({ tags: ['qdrant'] }))).toEqual({ tags: ['qdrant'] });
  });
});

describe('suggestionAnnouncement', () => {
  it('is singular at 1', () => {
    expect(suggestionAnnouncement(1)).toBe('1 suggested filter');
  });

  it('is plural otherwise', () => {
    expect(suggestionAnnouncement(3)).toBe('3 suggested filters');
  });
});
