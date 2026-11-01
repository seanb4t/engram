import { describe, it, expect } from 'vitest';
import fc from 'fast-check';
import { classifyInput } from './classify';

const space = fc.constantFrom(' ', '  ', '\t', '\n', ' \t ');
const pad = fc.array(space, { maxLength: 3 }).map((s) => s.join(''));
const gaps = fc.array(space, { maxLength: 6 });
const hex = (n: number) => fc.stringMatching(new RegExp(`^[0-9a-fA-F]{${n}}$`));
const uuid = fc.tuple(hex(8), hex(4), hex(4), hex(4), hex(12)).map((p) => p.join('-'));
const shortId = fc.stringMatching(/^[0-9a-hjkmnp-tv-zA-HJKMNP-TV-Z]{10}$/);
const operator = fc
  .tuple(fc.constantFrom('scope:', 'in:', 'tag:', '#', 'is:'), fc.stringMatching(/^[a-z0-9:/._-]{0,12}$/))
  .map(([prefix, value]) => prefix + value);
// Letters only and at most nine of them, so a word is never id-shaped and
// never an operator.
const word = fc.stringMatching(/^[a-z]{1,9}$/);
const isWord = (t: string) => /^[a-z]+$/.test(t);

function join(tokens: string[], lead: string, between: string[], trail: string): string {
  return lead + tokens.map((t, i) => (i === 0 ? t : (between[i] || ' ') + t)).join('') + trail;
}

describe('classifyInput properties', () => {
  it('empty: blank input is empty', () => {
    fc.assert(
      fc.property(pad, (raw) => {
        expect(classifyInput(raw)).toEqual({ kind: 'empty' });
      }),
    );
  });

  it('id: a UUID is id, lowercased', () => {
    fc.assert(
      fc.property(uuid, pad, pad, (id, lead, trail) => {
        expect(classifyInput(lead + id + trail)).toEqual({ kind: 'id', id: id.toLowerCase() });
      }),
    );
  });

  it('short_id: a canonical short_id is short_id, lowercased', () => {
    fc.assert(
      fc.property(shortId, pad, pad, (sid, lead, trail) => {
        expect(classifyInput(lead + sid + trail)).toEqual({ kind: 'short_id', shortId: sid.toLowerCase() });
      }),
    );
  });

  it('operators: operator-only input is operators', () => {
    fc.assert(
      fc.property(fc.array(operator, { minLength: 1, maxLength: 4 }), pad, gaps, pad, (ops, lead, between, trail) => {
        const got = classifyInput(join(ops, lead, between, trail));
        expect(got.kind).toBe('operators');
        expect(got.kind === 'operators' ? got.chips.length : -1).toBe(ops.length);
      }),
    );
  });

  it('text: other input is text, with whitespace trimmed and collapsed', () => {
    const tokens = fc.array(fc.oneof(word, operator), { minLength: 1, maxLength: 6 }).filter((ts) => ts.some(isWord));
    fc.assert(
      fc.property(tokens, pad, gaps, pad, (ts, lead, between, trail) => {
        expect(classifyInput(join(ts, lead, between, trail))).toMatchObject({
          kind: 'text',
          text: ts.filter(isWord).join(' '),
        });
      }),
    );
  });
});
