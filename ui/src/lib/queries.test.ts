import { describe, it, expect } from 'vitest';
import { listMemoriesKey, observeSearch, parseObserveParams } from './queries';

describe('observe params + keys', () => {
  it('parses scope/categories/visibility/offset/selected from URLSearchParams', () => {
    const p = parseObserveParams(new URLSearchParams('scope=repo:x&cat=gotcha&cat=convention&vis=shared&offset=20&sel=abc'));
    expect(p.scope).toBe('repo:x');
    expect(p.categories).toEqual(['gotcha', 'convention']);
    expect(p.visibility).toBe('shared');
    expect(p.offset).toBe(20);
    expect(p.selectedId).toBe('abc');
  });
  it('builds a stable list query key', () => {
    expect(listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20))
      .toEqual(['listMemories', 'repo:x', ['gotcha'], 'shared', 50, 20]);
  });
  it('round-trips observeSearch: parse → build → parse is stable', () => {
    const original = parseObserveParams(
      new URLSearchParams('scope=repo:x&cat=gotcha&cat=convention&vis=shared&offset=20&sel=abc')
    );
    const round = parseObserveParams(new URLSearchParams(observeSearch(original)));
    expect(round).toEqual(original);
  });
  it('observeSearch omits empty fields', () => {
    const search = observeSearch({ scope: '', categories: [], visibility: '', offset: 0, selectedId: '' });
    expect(search).toBe('');
  });
});
