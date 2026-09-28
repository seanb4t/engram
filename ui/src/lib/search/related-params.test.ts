import { describe, it, expect } from 'vitest';
import {
  defaultRelatedParams,
  parseRelatedParams,
  encodeRelatedParams,
  relatedPath,
  RELATED_TRAIL_MAX,
  type RelatedParams
} from './related-params';

const UUID_1 = '11111111-2222-3333-4444-555555555555';
const UUID_2 = '66666666-7777-8888-9999-000000000000';
const SHORT_1 = 'q7kf2m9x0c';

describe('related-params', () => {
  it('defaults to no origin and an empty trail', () => {
    expect(defaultRelatedParams()).toEqual({ from: '', trail: [] });
  });

  it('encodes the default params to an empty string', () => {
    expect(encodeRelatedParams(defaultRelatedParams())).toBe('');
  });

  it('round-trips from and trail through parse(encode(p))', () => {
    const p: RelatedParams = { from: '/search?q=a', trail: [UUID_1, SHORT_1] };
    const encoded = encodeRelatedParams(p);
    expect(parseRelatedParams(new URLSearchParams(encoded))).toEqual(p);
  });

  it.each([
    ['https://evil.example/x', ''],
    ['//evil.example', ''],
    ['/observe', ''],
    ['/ui/related/x', '']
  ])('from=%s parses to %j (open-redirect guard)', (raw, expected) => {
    const sp = new URLSearchParams();
    sp.set('from', raw);
    expect(parseRelatedParams(sp).from).toBe(expected);
  });

  it.each(['/ui/search?q=a', '/rules?sel=b'])('from=%s is kept verbatim', (raw) => {
    const sp = new URLSearchParams();
    sp.set('from', raw);
    expect(parseRelatedParams(sp).from).toBe(raw);
  });

  it('drops trail entries that are neither UUID nor short_id', () => {
    const sp = new URLSearchParams();
    sp.set('trail', `${UUID_1},not-an-id,${SHORT_1},`);
    expect(parseRelatedParams(sp).trail).toEqual([UUID_1, SHORT_1]);
  });

  it('keeps only the last RELATED_TRAIL_MAX trail entries', () => {
    const ids = Array.from({ length: RELATED_TRAIL_MAX + 5 }, (_, i) => `${UUID_1.slice(0, -2)}${String(i).padStart(2, '0')}`);
    const sp = new URLSearchParams();
    sp.set('trail', ids.join(','));
    const trail = parseRelatedParams(sp).trail;
    expect(trail.length).toBe(RELATED_TRAIL_MAX);
    expect(trail).toEqual(ids.slice(-RELATED_TRAIL_MAX));
  });

  it('builds the related path with an encoded from parameter', () => {
    expect(relatedPath('q7kf2m9x0c', { from: '/search?q=a', trail: [] })).toBe('/related/q7kf2m9x0c?from=%2Fsearch%3Fq%3Da');
  });

  it('builds a bare path with no query when params are defaults', () => {
    expect(relatedPath('q7kf2m9x0c', defaultRelatedParams())).toBe('/related/q7kf2m9x0c');
  });

  it('URI-encodes the id path segment', () => {
    expect(relatedPath(UUID_1, defaultRelatedParams())).toBe(`/related/${UUID_1}`);
  });
});
