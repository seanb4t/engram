import { describe, it, expect } from 'vitest';
import {
  defaultSearchParams,
  parseSearchParams,
  encodeSearchParams,
  searchMemoriesKey,
  searchMemoriesRequest,
  applyChips,
  nextK,
  listMemoriesCursorKey,
  listMemoriesRequest,
  DEFAULT_K,
  type SearchParams
} from './params';
import type { OperatorChip } from './classify';

describe('parseSearchParams / defaultSearchParams', () => {
  it('parses an empty URLSearchParams to the same shape as defaultSearchParams', () => {
    expect(parseSearchParams(new URLSearchParams())).toEqual(defaultSearchParams());
    expect(defaultSearchParams()).toEqual({
      q: '',
      scope: '',
      crossSpine: true,
      categories: [],
      tags: [],
      createdAfter: '',
      createdBefore: '',
      includeArchived: false,
      includeSuperseded: false,
      includeScheduled: false,
      k: 50,
      sel: ''
    });
  });

  it('a non-empty scope forces crossSpine false regardless of xs', () => {
    expect(parseSearchParams(new URLSearchParams('scope=repo:x&xs=1')).crossSpine).toBe(false);
  });

  it('xs=0 with no scope parses crossSpine false', () => {
    const p = parseSearchParams(new URLSearchParams('xs=0'));
    expect(p.crossSpine).toBe(false);
    expect(p.scope).toBe('');
  });

  it('accepts only K_STEPS values for k, else falls back to DEFAULT_K', () => {
    expect(parseSearchParams(new URLSearchParams('k=77')).k).toBe(DEFAULT_K);
    expect(parseSearchParams(new URLSearchParams('k=250')).k).toBe(250);
  });
});

describe('encodeSearchParams', () => {
  it('encodes the defaults to an empty string', () => {
    expect(encodeSearchParams(defaultSearchParams())).toBe('');
  });

  it('re-encodes xs=0 for cross-spine-off-with-no-scope', () => {
    const p = parseSearchParams(new URLSearchParams('xs=0'));
    expect(encodeSearchParams(p)).toBe('xs=0');
  });

  it('round-trips a full params object through encode/parse', () => {
    const p: SearchParams = {
      q: 'gofmt',
      scope: '',
      crossSpine: true,
      categories: ['gotcha', 'decision'],
      tags: ['ci', 'lint'],
      createdAfter: '2026-01-01T00:00:00Z',
      createdBefore: '2026-02-01T00:00:00Z',
      includeArchived: true,
      includeSuperseded: false,
      includeScheduled: true,
      k: 250,
      sel: 'abc123'
    };
    const encoded = encodeSearchParams(p);
    const parsed = parseSearchParams(new URLSearchParams(encoded));
    // Categories/tags are canonicalized (sorted) by encode, so compare the
    // sorted form of the original rather than requiring encode to preserve
    // input order.
    expect(parsed).toEqual({ ...p, categories: [...p.categories].sort(), tags: [...p.tags].sort() });
  });

  it('writes parameters in one canonical order', () => {
    const p: SearchParams = {
      ...defaultSearchParams(),
      q: 'gofmt',
      scope: '',
      tags: ['b', 'a'],
      categories: ['z', 'a'],
      createdAfter: '2026-01-01',
      createdBefore: '2026-02-01',
      includeArchived: true,
      k: 100,
      sel: 'id1'
    };
    const encoded = encodeSearchParams(p);
    expect(encoded).toBe(
      'q=gofmt&cat=a&cat=z&tag=a&tag=b&after=2026-01-01&before=2026-02-01&inc=archived&k=100&sel=id1'
    );
  });
});

describe('searchMemoriesKey', () => {
  it('is order-insensitive to both URL param order and chip order', () => {
    const a = parseSearchParams(new URLSearchParams('q=gofmt&cat=a&cat=b&tag=x&tag=y'));
    const b = parseSearchParams(new URLSearchParams('tag=y&tag=x&cat=b&cat=a&q=gofmt'));
    expect(searchMemoriesKey(a, false)).toEqual(searchMemoriesKey(b, false));
  });

  it('leads with the literal RPC name so existing write invalidation reaches it', () => {
    expect(searchMemoriesKey(defaultSearchParams(), false)[0]).toBe('searchMemories');
  });
});

describe('searchMemoriesRequest', () => {
  it('never carries a non-empty scope together with crossSpine true', () => {
    const withScope = searchMemoriesRequest({ ...defaultSearchParams(), scope: 'repo:x', crossSpine: true }, false);
    expect(withScope.scope).toBe('repo:x');
    expect(withScope.crossSpine).toBe(false);

    const withoutScope = searchMemoriesRequest({ ...defaultSearchParams(), crossSpine: true }, false);
    expect(withoutScope.scope).toBe('');
    expect(withoutScope.crossSpine).toBe(true);
  });

  it('sends k as a bigint', () => {
    const req = searchMemoriesRequest({ ...defaultSearchParams(), k: 250 }, false);
    expect(req.k).toBe(250n);
    expect(typeof req.k).toBe('bigint');
  });
});

describe('applyChips', () => {
  it('applies scope/tag/category chips, sorts and dedupes, and ignores pending chips', () => {
    const chips: OperatorChip[] = [
      { kind: 'scope', value: 'repo:x' },
      { kind: 'tag', value: 'b' },
      { kind: 'tag', value: 'a' },
      { kind: 'category', value: 'gotcha', known: true },
      { kind: 'category', value: 'bogus', known: false },
      { kind: 'pending', raw: 'scope:' }
    ];
    const result = applyChips(defaultSearchParams(), chips);
    expect(result.scope).toBe('repo:x');
    expect(result.crossSpine).toBe(false);
    expect(result.tags).toEqual(['a', 'b']);
    expect(result.categories).toEqual(['bogus', 'gotcha']);
  });

  it('the last of multiple scope chips wins', () => {
    const chips: OperatorChip[] = [
      { kind: 'scope', value: 'repo:x' },
      { kind: 'scope', value: 'repo:y' }
    ];
    expect(applyChips(defaultSearchParams(), chips).scope).toBe('repo:y');
  });

  it('leaves crossSpine at the caller-provided default when there is no scope chip', () => {
    const result = applyChips(defaultSearchParams(), []);
    expect(result.scope).toBe('');
    expect(result.crossSpine).toBe(true);
  });
});

describe('nextK', () => {
  it('escalates through K_STEPS and returns undefined at the ceiling', () => {
    expect(nextK(50)).toBe(100);
    expect(nextK(100)).toBe(250);
    expect(nextK(250)).toBe(1000);
    expect(nextK(1000)).toBeUndefined();
  });

  it('returns undefined for a value outside K_STEPS', () => {
    expect(nextK(77)).toBeUndefined();
  });
});

describe('listMemoriesCursorKey', () => {
  it('starts with listMemories and has an empty visibility slot at index 3', () => {
    const key = listMemoriesCursorKey(defaultSearchParams());
    expect(key[0]).toBe('listMemories');
    expect(key[3]).toBe('');
  });

  it('is equal for chip-order permutations (categories/tags sorted)', () => {
    const a = { ...defaultSearchParams(), categories: ['a', 'b'], tags: ['x', 'y'] };
    const b = { ...defaultSearchParams(), categories: ['b', 'a'], tags: ['y', 'x'] };
    expect(listMemoriesCursorKey(a)).toEqual(listMemoriesCursorKey(b));
  });

  it('never carries a non-empty scope together with crossSpine true', () => {
    const withScope = listMemoriesCursorKey({ ...defaultSearchParams(), scope: 'repo:x', crossSpine: true });
    // crossSpine slot is index 9
    expect(withScope[9]).toBe(false);
  });
});

describe('listMemoriesRequest', () => {
  it('sets cursorMode true, limit 50n, and passes pageToken through', () => {
    const req = listMemoriesRequest(defaultSearchParams(), 't2');
    expect(req.cursorMode).toBe(true);
    expect(req.limit).toBe(50n);
    expect(req.pageToken).toBe('t2');
  });

  it('sends crossSpine true when no scope is set, never with a scope', () => {
    const noScope = listMemoriesRequest({ ...defaultSearchParams(), crossSpine: true }, '');
    expect(noScope.scope).toBe('');
    expect(noScope.crossSpine).toBe(true);

    const withScope = listMemoriesRequest({ ...defaultSearchParams(), scope: 'repo:x', crossSpine: true }, '');
    expect(withScope.scope).toBe('repo:x');
    expect(withScope.crossSpine).toBe(false);
  });
});
