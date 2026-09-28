import { describe, it, expect } from 'vitest';
import { barPercent, TAG_TOP, visibleTagRows, tagListFooter, rankTagMatches, matchFooter, type TagRow } from './tags';

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

const MATCH_FIXTURE: TagRow[] = [
  { tag: 'qdrant', count: 9 },
  { tag: 'sqlite', count: 12 },
  { tag: 'qdr-ops', count: 2 },
  { tag: 'quad', count: 9 }
];

describe('rankTagMatches', () => {
  it('"qd" returns qdrant then qdr-ops (prefix first, by count), total 2, plus an unknown row', () => {
    const r = rankTagMatches(MATCH_FIXTURE, 'qd', { more: true });
    expect(r.matches.map((m) => m.tag)).toEqual(['qdrant', 'qdr-ops']);
    expect(r.total).toBe(2);
    expect(r.unknown).toEqual({ tag: 'qd', reason: '#qd — not among the loaded tags' });
  });

  it('"q" puts prefix matches (qdrant, quad, qdr-ops -- tied counts break by tag ascending) before the non-prefix match (sqlite)', () => {
    const r = rankTagMatches(MATCH_FIXTURE, 'q', { more: false });
    expect(r.matches.map((m) => m.tag)).toEqual(['qdrant', 'quad', 'qdr-ops', 'sqlite']);
  });

  it('a query equal to a loaded tag (case-insensitive) yields no unknown row', () => {
    const r = rankTagMatches(MATCH_FIXTURE, 'SQLite', { more: true });
    expect(r.unknown).toBeNull();
  });

  it('strips a leading "#" or "tag:" before matching', () => {
    const hash = rankTagMatches(MATCH_FIXTURE, '#qd', { more: true });
    const prefixed = rankTagMatches(MATCH_FIXTURE, 'tag:qd', { more: true });
    const bare = rankTagMatches(MATCH_FIXTURE, 'qd', { more: true });
    expect(hash).toEqual(bare);
    expect(prefixed).toEqual(bare);
  });

  it('unknown reason depends on more: "not among the loaded tags" vs "0 recall-visible records"', () => {
    expect(rankTagMatches(MATCH_FIXTURE, 'qd', { more: true }).unknown?.reason).toBe('#qd — not among the loaded tags');
    expect(rankTagMatches(MATCH_FIXTURE, 'qd', { more: false }).unknown?.reason).toBe('#qd — 0 recall-visible records');
  });

  it('more than 8 matches return 8 rows with total the full count', () => {
    const many: TagRow[] = Array.from({ length: 10 }, (_, i) => ({ tag: `alpha${i}`, count: 10 - i }));
    const r = rankTagMatches(many, 'alpha', { more: false });
    expect(r.matches.length).toBe(8);
    expect(r.total).toBe(10);
  });

  it('each match splits into parts with the hit flagged', () => {
    const r = rankTagMatches(MATCH_FIXTURE, 'q', { more: false });
    const sqliteMatch = r.matches.find((m) => m.tag === 'sqlite')!;
    expect(sqliteMatch.parts).toEqual([
      { text: 's', hit: false },
      { text: 'q', hit: true },
      { text: 'lite', hit: false }
    ]);
    const qdrantMatch = r.matches.find((m) => m.tag === 'qdrant')!;
    expect(qdrantMatch.parts).toEqual([
      { text: 'q', hit: true },
      { text: 'drant', hit: false }
    ]);
  });

  it('an empty query returns the first 8 rows unranked (server order), total = rows.length, no unknown', () => {
    const r = rankTagMatches(MATCH_FIXTURE, '', { more: false });
    expect(r.matches.map((m) => m.tag)).toEqual(['qdrant', 'sqlite', 'qdr-ops', 'quad']);
    expect(r.total).toBe(4);
    expect(r.unknown).toBeNull();
  });
});

describe('matchFooter', () => {
  it('uses the same template regardless of total -- no singular special case (E5 zero-one-many backstop)', () => {
    expect(matchFooter({ total: 1, more: false, loaded: 1000 })).toEqual({ text: '1 matches · top 8 shown', warn: null });
    expect(matchFooter({ total: 8, more: false, loaded: 1000 })).toEqual({ text: '8 matches · top 8 shown', warn: null });
  });

  it('adds the warning line when more is true', () => {
    expect(matchFooter({ total: 3, more: true, loaded: 1000 })).toEqual({
      text: '3 matches · top 8 shown',
      warn: 'matching among the 1,000 most-used tags'
    });
  });
});
