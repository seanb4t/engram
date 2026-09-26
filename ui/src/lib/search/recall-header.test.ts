import { describe, it, expect } from 'vitest';
import { rankedHeaderParts, headerText, hiddenFromProto, hiddenTooltip, plural, loadingLine, type HiddenCounts } from './recall-header';

describe('recall-header', () => {
  it('renders the ranked happy path with hits/scopes/query/ranking/hidden clauses in that order', () => {
    const hidden: HiddenCounts = { total: 2, archived: 0, superseded: 0, expired: 0, scheduled: 0 };
    const parts = rankedHeaderParts({ hits: 12, scopeCount: 3, query: 'recall gate', reranked: false, hidden });
    expect(headerText(parts)).toBe('12 hits across 3 scopes for recall gate · ranked by cosine · 2 hidden by recall gate');
  });

  it('agrees in singular number for exactly one hit and one scope', () => {
    const zeroHidden: HiddenCounts = { total: 0, archived: 0, superseded: 0, expired: 0, scheduled: 0 };
    const parts = rankedHeaderParts({ hits: 1, scopeCount: 1, query: 'x', reranked: false, hidden: zeroHidden });
    expect(headerText(parts)).toBe('1 hit across 1 scope for x · ranked by cosine');
  });

  it('says reranked by jev when the reranker ran', () => {
    const zeroHidden: HiddenCounts = { total: 0, archived: 0, superseded: 0, expired: 0, scheduled: 0 };
    const parts = rankedHeaderParts({ hits: 3, scopeCount: 2, query: 'x', reranked: true, hidden: zeroHidden });
    expect(headerText(parts)).toBe('3 hits across 2 scopes for x · reranked by jev');
  });

  it('omits the hidden clause entirely when nothing was hidden', () => {
    const hidden: HiddenCounts = { total: 0, archived: 0, superseded: 0, expired: 0, scheduled: 0 };
    const parts = rankedHeaderParts({ hits: 5, scopeCount: 2, query: 'x', reranked: false, hidden });
    expect(headerText(parts)).toBe('5 hits across 2 scopes for x · ranked by cosine');
  });

  it('reports the hidden count as unavailable when the field is absent', () => {
    const parts = rankedHeaderParts({ hits: 5, scopeCount: 2, query: 'x', reranked: false });
    expect(headerText(parts)).toBe('5 hits across 2 scopes for x · ranked by cosine · hidden count unavailable');
  });

  it('hiddenFromProto converts bigint proto fields to plain numbers', () => {
    const proto = { $typeName: 'engram.v1.RecallGateHidden', total: 2n, archived: 1n, superseded: 1n, expired: 0n, scheduled: 0n } as never;
    expect(hiddenFromProto(proto)).toEqual({ total: 2, archived: 1, superseded: 1, expired: 0, scheduled: 0 });
  });

  it('hiddenFromProto returns undefined when the field itself is absent', () => {
    expect(hiddenFromProto(undefined)).toBeUndefined();
  });

  it('hiddenTooltip lists nonzero per-state counts, comma separated, in canonical order', () => {
    expect(hiddenTooltip({ total: 2, archived: 1, superseded: 1, expired: 0, scheduled: 0 })).toBe('1 archived, 1 superseded');
  });

  it('plural picks the singular/plural form by count', () => {
    expect(plural(1, 'hit', 'hits')).toBe('hit');
    expect(plural(0, 'hit', 'hits')).toBe('hits');
    expect(plural(2, 'hit', 'hits')).toBe('hits');
  });

  it('loadingLine names the scope count and the query', () => {
    expect(loadingLine('q', 3)).toBe('Searching 3 scopes for q…');
  });
});
