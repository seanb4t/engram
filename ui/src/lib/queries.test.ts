import { describe, it, expect } from 'vitest';
import { listMemoriesKey, observeSearch, parseObserveParams, type ObserveParams } from './queries';

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
    expect(listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20, false, false, false))
      .toEqual(['listMemories', 'repo:x', ['gotcha'], 'shared', 50, 20, false, false, false]);
  });
  it('round-trips observeSearch: parse → build → parse is stable', () => {
    const original = parseObserveParams(
      new URLSearchParams('scope=repo:x&cat=gotcha&cat=convention&vis=shared&offset=20&sel=abc')
    );
    const round = parseObserveParams(new URLSearchParams(observeSearch(original)));
    expect(round).toEqual(original);
  });
  it('observeSearch omits empty fields', () => {
    const search = observeSearch({
      scope: '', categories: [], visibility: '', offset: 0, selectedId: '',
      includeArchived: false, includeSuperseded: false, includeScheduled: false
    });
    expect(search).toBe('');
  });

  it('parseObserveParams yields all three include flags false by default', () => {
    const p = parseObserveParams(new URLSearchParams(''));
    expect(p.includeArchived).toBe(false);
    expect(p.includeSuperseded).toBe(false);
    expect(p.includeScheduled).toBe(false);
  });

  it('observeSearch emits no inc parameter at all when all three include flags are false', () => {
    const search = observeSearch({
      scope: 'repo:x', categories: [], visibility: '', offset: 0, selectedId: '',
      includeArchived: false, includeSuperseded: false, includeScheduled: false
    });
    expect(search).not.toContain('inc=');
  });

  it('observeSearch output for an all-false ObserveParams is identical to the pre-phase output for the same non-include fields', () => {
    const p: ObserveParams = {
      scope: 'repo:x', categories: ['gotcha'], visibility: 'shared', offset: 20, selectedId: 'abc',
      includeArchived: false, includeSuperseded: false, includeScheduled: false
    };
    const expected = new URLSearchParams();
    expected.set('scope', 'repo:x');
    expected.append('cat', 'gotcha');
    expected.set('vis', 'shared');
    expected.set('offset', '20');
    expected.set('sel', 'abc');
    expect(observeSearch(p)).toBe(expected.toString());
  });

  it('observeSearch emits inc=archived and inc=scheduled but not inc=superseded when only those two are true', () => {
    const search = observeSearch({
      scope: '', categories: [], visibility: '', offset: 0, selectedId: '',
      includeArchived: true, includeSuperseded: false, includeScheduled: true
    });
    expect(search).toContain('inc=archived');
    expect(search).toContain('inc=scheduled');
    expect(search).not.toContain('inc=superseded');
  });

  it('parseObserveParams reads exactly the two set inc flags back from that string', () => {
    const search = observeSearch({
      scope: '', categories: [], visibility: '', offset: 0, selectedId: '',
      includeArchived: true, includeSuperseded: false, includeScheduled: true
    });
    const p = parseObserveParams(new URLSearchParams(search));
    expect(p.includeArchived).toBe(true);
    expect(p.includeSuperseded).toBe(false);
    expect(p.includeScheduled).toBe(true);
  });

  it('drops an unrecognised inc value, mirroring how an unrecognised cat value is filtered out', () => {
    const p = parseObserveParams(new URLSearchParams('inc=bogus&inc=archived'));
    expect(p.includeArchived).toBe(true);
    expect(p.includeSuperseded).toBe(false);
    expect(p.includeScheduled).toBe(false);
  });

  it('observeSearch(parseObserveParams(sp)) is idempotent for every combination of the three include flags', () => {
    for (let mask = 0; mask < 8; mask++) {
      const sp = new URLSearchParams();
      sp.set('scope', 'repo:x');
      if (mask & 1) sp.append('inc', 'archived');
      if (mask & 2) sp.append('inc', 'superseded');
      if (mask & 4) sp.append('inc', 'scheduled');
      const first = parseObserveParams(sp);
      const second = parseObserveParams(new URLSearchParams(observeSearch(first)));
      expect(second).toEqual(first);
    }
  });

  it('listMemoriesKey differs between two calls whose only difference is includeSuperseded', () => {
    const a = listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20, false, false, false);
    const b = listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20, false, true, false);
    expect(a).not.toEqual(b);
  });
});
