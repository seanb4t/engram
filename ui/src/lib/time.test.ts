import { describe, it, expect } from 'vitest';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { relativeTime, fullTimestamp, windowRange, windowPhrase } from './time';

const NOW = new Date('2026-06-12T15:00:00Z');

describe('relativeTime', () => {
  it('renders hours', () => { expect(relativeTime(new Date('2026-06-12T10:00:00Z'), NOW)).toBe('5h'); });
  it('renders days', () => { expect(relativeTime(new Date('2026-06-10T15:00:00Z'), NOW)).toBe('2d'); });
  it('renders just now under a minute', () => { expect(relativeTime(new Date('2026-06-12T14:59:40Z'), NOW)).toBe('now'); });
});
describe('fullTimestamp', () => {
  it('renders an ISO-ish minute precision', () => { expect(fullTimestamp(new Date('2026-06-12T14:03:00Z'))).toMatch(/2026-06-12 14:03/); });
});

// D-13/CUR-04: /scheduled's window range and relative reveal/expiry phrase.
describe('windowRange', () => {
  it('renders both bounds as "{not_before} → {not_after}"', () => {
    const m = {
      notBefore: timestampFromDate(new Date('2026-06-10T00:00:00Z')),
      notAfter: timestampFromDate(new Date('2026-06-20T00:00:00Z'))
    };
    expect(windowRange(m)).toBe('2026-06-10 00:00 → 2026-06-20 00:00');
  });

  it('renders "—" for an absent not_before', () => {
    const m = { notBefore: undefined, notAfter: timestampFromDate(new Date('2026-06-20T00:00:00Z')) };
    expect(windowRange(m)).toBe('— → 2026-06-20 00:00');
  });

  it('renders "—" for an absent not_after', () => {
    const m = { notBefore: timestampFromDate(new Date('2026-06-10T00:00:00Z')), notAfter: undefined };
    expect(windowRange(m)).toBe('2026-06-10 00:00 → —');
  });
});

describe('windowPhrase', () => {
  it('reads "expired just now" when not_after equals now', () => {
    const m = { notBefore: undefined, notAfter: timestampFromDate(NOW) };
    expect(windowPhrase(m, NOW)).toBe('expired just now');
  });

  it('reads "expired {N}d ago" when not_after is 2 days in the past', () => {
    const m = { notBefore: undefined, notAfter: timestampFromDate(new Date('2026-06-10T15:00:00Z')) };
    expect(windowPhrase(m, NOW)).toBe('expired 2d ago');
  });

  it('reads "" when not_before equals now (already active, not scheduled)', () => {
    const m = { notBefore: timestampFromDate(NOW), notAfter: undefined };
    expect(windowPhrase(m, NOW)).toBe('');
  });

  it('reads "reveals in {N}d" when not_before is 3 days ahead', () => {
    const m = { notBefore: timestampFromDate(new Date('2026-06-15T15:00:00Z')), notAfter: undefined };
    expect(windowPhrase(m, NOW)).toBe('reveals in 3d');
  });

  it('reads "reveals in under a minute" when not_before is 20 seconds ahead', () => {
    const m = { notBefore: timestampFromDate(new Date('2026-06-12T15:00:20Z')), notAfter: undefined };
    expect(windowPhrase(m, NOW)).toBe('reveals in under a minute');
  });

  it('reads "" for a record with neither bound', () => {
    expect(windowPhrase({ notBefore: undefined, notAfter: undefined }, NOW)).toBe('');
  });

  it('expired takes precedence over an (invalid) inverted window', () => {
    // not_after in the past AND not_before in the future -- expired must win
    // (mirrors memorystate.ts's own precedence for an inverted window).
    const m = {
      notBefore: timestampFromDate(new Date('2026-06-13T00:00:00Z')),
      notAfter: timestampFromDate(new Date('2026-06-11T00:00:00Z'))
    };
    expect(windowPhrase(m, NOW)).toMatch(/^expired/);
  });
});
