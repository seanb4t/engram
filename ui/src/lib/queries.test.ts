import { describe, it, expect } from 'vitest';
import { listMemoriesKey } from './queries';

describe('listMemoriesKey', () => {
  it('builds a stable list query key', () => {
    expect(listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20, false, false, false, false))
      .toEqual(['listMemories', 'repo:x', ['gotcha'], 'shared', 50, 20, false, false, false, false]);
  });

  it('appends crossSpine as a trailing, explicit index-9 slot without disturbing indexes 0-8', () => {
    const withoutCrossSpine = listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20, false, false, false, false);
    const withCrossSpine = listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20, false, false, false, true);
    expect(withCrossSpine).not.toEqual(withoutCrossSpine);
    expect(withCrossSpine.slice(0, 9)).toEqual(withoutCrossSpine.slice(0, 9));
    expect(withCrossSpine[9]).toBe(true);
    expect(withoutCrossSpine[9]).toBe(false);
  });

  it('differs between two calls whose only difference is includeSuperseded', () => {
    const a = listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20, false, false, false, false);
    const b = listMemoriesKey('repo:x', ['gotcha'], 'shared', 50, 20, false, true, false, false);
    expect(a).not.toEqual(b);
  });
});
