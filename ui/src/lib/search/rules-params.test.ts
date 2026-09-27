import { describe, it, expect } from 'vitest';
import { defaultRulesParams, parseRulesParams, encodeRulesParams } from './rules-params';

describe('parseRulesParams / defaultRulesParams', () => {
  it('parses an empty URLSearchParams to the same shape as defaultRulesParams', () => {
    expect(parseRulesParams(new URLSearchParams())).toEqual(defaultRulesParams());
    expect(defaultRulesParams()).toEqual({ sel: '' });
  });

  it('parses sel from the URL', () => {
    expect(parseRulesParams(new URLSearchParams('sel=abc123'))).toEqual({ sel: 'abc123' });
  });
});

describe('encodeRulesParams', () => {
  it('encodes the defaults to an empty string', () => {
    expect(encodeRulesParams(defaultRulesParams())).toBe('');
  });

  it('round-trips a non-default sel through parse -> encode -> parse', () => {
    const p = parseRulesParams(new URLSearchParams('sel=r-1'));
    const encoded = encodeRulesParams(p);
    expect(encoded).toBe('sel=r-1');
    expect(parseRulesParams(new URLSearchParams(encoded))).toEqual(p);
  });
});
