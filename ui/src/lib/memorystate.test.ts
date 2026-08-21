import { describe, it, expect } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { MemorySchema } from '$lib/gen/engram_pb';
import { memoryStateWords, isPastState, STATE_WORD_ORDER, type RecordStateWord } from './memorystate';

const now = new Date('2030-06-15T12:00:00Z');
const past = new Date(now.getTime() - 24 * 60 * 60 * 1000);
const future = new Date(now.getTime() + 24 * 60 * 60 * 1000);

describe('memoryStateWords', () => {
  it('empty record yields []', () => {
    expect(memoryStateWords(create(MemorySchema, {}), now)).toEqual([]);
  });

  it('archived only', () => {
    const m = create(MemorySchema, { archivedAt: timestampFromDate(now) });
    expect(memoryStateWords(m, now)).toEqual(['archived']);
  });

  it('superseded only', () => {
    const m = create(MemorySchema, { supersededBy: 'successor-id' });
    expect(memoryStateWords(m, now)).toEqual(['superseded']);
  });

  it('expired only (not_after strictly in the past)', () => {
    const m = create(MemorySchema, { notAfter: timestampFromDate(past) });
    expect(memoryStateWords(m, now)).toEqual(['expired']);
  });

  it('scheduled only (not_before strictly in the future)', () => {
    const m = create(MemorySchema, { notBefore: timestampFromDate(future) });
    expect(memoryStateWords(m, now)).toEqual(['scheduled']);
  });

  it('archived + superseded + expired compound, canonical order', () => {
    const m = create(MemorySchema, {
      archivedAt: timestampFromDate(now),
      supersededBy: 'successor-id',
      notAfter: timestampFromDate(past)
    });
    expect(memoryStateWords(m, now)).toEqual(['archived', 'superseded', 'expired']);
  });

  it('order independence: same compound, fields set in reverse declaration order', () => {
    const m = create(MemorySchema, {
      notAfter: timestampFromDate(past),
      supersededBy: 'successor-id',
      archivedAt: timestampFromDate(now)
    });
    expect(memoryStateWords(m, now)).toEqual(['archived', 'superseded', 'expired']);
  });

  it('not_before === now yields NO word (inclusive Lte bound, already active)', () => {
    const m = create(MemorySchema, { notBefore: timestampFromDate(now) });
    expect(memoryStateWords(m, now)).toEqual([]);
  });

  it('not_after === now yields expired (exclusive Gt bound, already expired)', () => {
    const m = create(MemorySchema, { notAfter: timestampFromDate(now) });
    expect(memoryStateWords(m, now)).toEqual(['expired']);
  });

  it('expired and scheduled never co-occur: inverted window yields exactly [expired]', () => {
    // notAfter in the past AND notBefore in the future — RuleWindowOrdering
    // forbids WRITING this, but it is a write-time rule; the wire can still
    // carry an inverted window (e.g. a legacy record), so this fixture
    // bypasses that rule directly via create(MemorySchema, ...).
    const m = create(MemorySchema, { notAfter: timestampFromDate(past), notBefore: timestampFromDate(future) });
    expect(memoryStateWords(m, now)).toEqual(['expired']);
  });

  it('canonical order matches STATE_WORD_ORDER', () => {
    expect(STATE_WORD_ORDER).toEqual(['archived', 'superseded', 'expired', 'scheduled']);
  });
});

describe('isPastState', () => {
  it('true for each past word alone', () => {
    expect(isPastState(['archived'])).toBe(true);
    expect(isPastState(['superseded'])).toBe(true);
    expect(isPastState(['expired'])).toBe(true);
  });

  it('false for scheduled alone', () => {
    expect(isPastState(['scheduled'])).toBe(false);
  });

  it('false for []', () => {
    expect(isPastState([])).toBe(false);
  });

  it('true when a past word is combined with scheduled', () => {
    const words: RecordStateWord[] = ['archived', 'scheduled'];
    expect(isPastState(words)).toBe(true);
  });
});
