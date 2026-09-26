import { describe, it, expect } from 'vitest';
import {
  rankedHeaderParts,
  headerText,
  hiddenFromProto,
  hiddenTooltip,
  plural,
  loadingLine,
  resolutionLine,
  type HiddenCounts
} from './recall-header';

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

  const zeroHidden: HiddenCounts = { total: 0, archived: 0, superseded: 0, expired: 0, scheduled: 0 };

  it('shows "of N" when a category filter narrows the total hit count', () => {
    const parts = rankedHeaderParts({ hits: 12, hitsWithoutCategory: 20, scopeCount: 3, query: 'x', reranked: false, hidden: zeroHidden });
    expect(headerText(parts)).toBe('12 hits of 20 across 3 scopes for x · ranked by cosine');
  });

  it('omits "of N" when the unfiltered total equals the filtered hit count', () => {
    const parts = rankedHeaderParts({ hits: 12, hitsWithoutCategory: 12, scopeCount: 3, query: 'x', reranked: false, hidden: zeroHidden });
    expect(headerText(parts)).toBe('12 hits across 3 scopes for x · ranked by cosine');
  });

  it('renders the verbatim scopes_truncated coverage clause, never inferring a count', () => {
    const parts = rankedHeaderParts({ hits: 5, scopeCount: 3, query: 'x', reranked: false, hidden: zeroHidden, scopesTruncated: true });
    expect(headerText(parts)).toBe('5 hits across 3 scopes for x · ranked by cosine · scopes_truncated: scope list incomplete');
  });

  it('renders "across every readable scope" and the verbatim scopes_unknown coverage clause', () => {
    const parts = rankedHeaderParts({ hits: 5, scopeCount: 3, query: 'x', reranked: false, hidden: zeroHidden, scopesUnknown: true });
    expect(headerText(parts)).toBe(
      '5 hits across every readable scope for x · ranked by cosine · scopes_unknown: scope coverage could not be listed'
    );
  });

  it('hiddenTooltip appends the overlap note when the per-state sum exceeds total', () => {
    expect(hiddenTooltip({ total: 1, archived: 1, superseded: 1, expired: 0, scheduled: 0 })).toBe(
      '1 archived, 1 superseded · a record can carry more than one state'
    );
  });

  it('resolutionLine describes a resolved id, and notes when it is superseded and hidden from search', () => {
    const id = '753aba22-1111-2222-3333-444455556666';
    expect(resolutionLine({ kind: 'id', id })).toBe(`Resolved id ${id} → 1 memory`);
    expect(resolutionLine({ kind: 'id', id, supersededBy: 'k3m9p2qr7a' })).toBe(
      `Resolved id ${id} → 1 memory · this record is superseded → k3m9p2qr7a; hidden from search, fetchable by id`
    );
  });

  it('resolutionLine describes a resolved short_id', () => {
    expect(resolutionLine({ kind: 'short_id', shortId: 'k3m9p2qr7a' })).toBe('Resolved short_id k3m9p2qr7a → 1 memory');
  });

  it('resolutionLine describes a short_id-shaped miss re-searched as text, never a silent reinterpretation', () => {
    expect(resolutionLine({ kind: 'short_id_miss', q: 'k3m9p2qr7a' })).toBe(
      'No short_id attachment. Searched it as text instead: k3m9p2qr7a'
    );
  });
});
