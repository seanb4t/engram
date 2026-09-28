import { describe, it, expect } from 'vitest';
import {
  defaultScheduledParams,
  parseScheduledParams,
  encodeScheduledParams,
  type ScheduledParams
} from './scheduled-params';

describe('scheduled-params', () => {
  it('defaults to the scheduled tab with no selection', () => {
    expect(defaultScheduledParams()).toEqual({ state: 'scheduled', sel: '' });
  });

  it('encodes the default params to an empty string', () => {
    expect(encodeScheduledParams(defaultScheduledParams())).toBe('');
  });

  it('parses an unknown state value as "scheduled"', () => {
    const sp = new URLSearchParams('state=bogus');
    expect(parseScheduledParams(sp).state).toBe('scheduled');
  });

  it('parses a missing state as "scheduled"', () => {
    expect(parseScheduledParams(new URLSearchParams('')).state).toBe('scheduled');
  });

  it.each([
    { state: 'scheduled', sel: '' },
    { state: 'expired', sel: '' },
    { state: 'all', sel: '' },
    { state: 'expired', sel: 'm1' }
  ] satisfies ScheduledParams[])('round-trips %j through parse(encode(p))', (p) => {
    const encoded = encodeScheduledParams(p);
    expect(parseScheduledParams(new URLSearchParams(encoded))).toEqual(p);
  });

  it('omits state=scheduled and an empty sel from the encoded string', () => {
    expect(encodeScheduledParams({ state: 'scheduled', sel: '' })).toBe('');
    expect(encodeScheduledParams({ state: 'expired', sel: '' })).toBe('state=expired');
  });
});
