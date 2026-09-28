import { describe, it, expect } from 'vitest';
import { barPercent, TAG_TOP, visibleTagRows, tagListFooter, type TagRow } from './tags';

function rows(n: number, startCount = n): TagRow[] {
  return Array.from({ length: n }, (_, i) => ({ tag: `tag${i}`, count: startCount - i }));
}

describe('barPercent', () => {
  it('scales a count against the largest loaded count', () => {
    expect(barPercent(1, 400)).toBe(0.25);
  });

  it('returns 0 when the max is 0 (no rows loaded)', () => {
    expect(barPercent(5, 0)).toBe(0);
  });
});

describe('visibleTagRows', () => {
  const thirtyOne = rows(31);

  it('returns 30 of 31 rows unfiltered and collapsed', () => {
    const { rows: visible, hidden } = visibleTagRows(thirtyOne, { filter: '', expanded: false });
    expect(visible.length).toBe(30);
    expect(hidden).toBe(1);
    expect(visible.map((r) => r.tag)).toEqual(thirtyOne.slice(0, 30).map((r) => r.tag));
  });

  it('returns all 31 rows when expanded', () => {
    const { rows: visible, hidden } = visibleTagRows(thirtyOne, { filter: '', expanded: true });
    expect(visible.length).toBe(31);
    expect(hidden).toBe(0);
  });

  it('filter matches case-insensitively, preserving order, unaffected by expanded/collapsed', () => {
    const source: TagRow[] = [
      { tag: 'Qdrant', count: 10 },
      { tag: 'mcp', count: 8 },
      { tag: 'QDR-OPS', count: 5 }
    ];
    const { rows: visible, hidden } = visibleTagRows(source, { filter: 'qd', expanded: false });
    expect(visible.map((r) => r.tag)).toEqual(['Qdrant', 'QDR-OPS']);
    expect(hidden).toBe(0);
  });
});

describe('tagListFooter', () => {
  it('collapsed, over 30 loaded -> "showing the 30 most-used tags", no warn', () => {
    const footer = tagListFooter({ loaded: 31, more: false, filtered: false, expanded: false });
    expect(footer).toEqual({ text: 'showing the 30 most-used tags', warn: false });
  });

  it('expanded, more=true -> "showing the 1,000 most-used tags -- more exist", warn', () => {
    const footer = tagListFooter({ loaded: 1000, more: true, filtered: false, expanded: true });
    expect(footer).toEqual({ text: 'showing the 1,000 most-used tags — more exist', warn: true });
  });

  it('filtered, more=true -> "matching among the 1,000 most-used tags", warn', () => {
    const footer = tagListFooter({ loaded: 1000, more: true, filtered: true, expanded: false });
    expect(footer).toEqual({ text: 'matching among the 1,000 most-used tags', warn: true });
  });

  it('returns null when none of the truncation conditions apply', () => {
    expect(tagListFooter({ loaded: 10, more: false, filtered: false, expanded: false })).toBeNull();
    expect(tagListFooter({ loaded: 1000, more: false, filtered: false, expanded: true })).toBeNull();
    expect(tagListFooter({ loaded: 1000, more: false, filtered: true, expanded: false })).toBeNull();
  });

  it('never returns a string containing " of "', () => {
    const cases = [
      tagListFooter({ loaded: 31, more: false, filtered: false, expanded: false }),
      tagListFooter({ loaded: 1000, more: true, filtered: false, expanded: true }),
      tagListFooter({ loaded: 1000, more: true, filtered: true, expanded: false })
    ];
    for (const c of cases) {
      expect(c?.text ?? '').not.toContain(' of ');
    }
  });
});

describe('TAG_TOP', () => {
  it('is 30', () => {
    expect(TAG_TOP).toBe(30);
  });
});
