import { describe, it, expect } from 'vitest';
import { classifyInput, UUID_RE, SHORT_ID_RE, KNOWN_CATEGORIES } from './classify';

describe('classifyInput', () => {
  it('classifies empty and whitespace-only input as empty', () => {
    expect(classifyInput('')).toEqual({ kind: 'empty' });
    expect(classifyInput('   ')).toEqual({ kind: 'empty' });
  });

  it('classifies a single UUID token as id, trimmed and lowercased', () => {
    expect(classifyInput(' 753ABA22-0000-4000-8000-000000000001 ')).toEqual({
      kind: 'id',
      id: '753aba22-0000-4000-8000-000000000001'
    });
  });

  it('classifies a 10-char Crockford short_id token case-insensitively', () => {
    expect(classifyInput('k3m9p2qr7a')).toEqual({ kind: 'short_id', shortId: 'k3m9p2qr7a' });
    expect(classifyInput('K3M9P2QR7A')).toEqual({ kind: 'short_id', shortId: 'k3m9p2qr7a' });
  });

  it('treats a 9- or 11-character near-miss as text', () => {
    expect(classifyInput('k3m9p2qr7')).toEqual({ kind: 'text', text: 'k3m9p2qr7', chips: [] });
    expect(classifyInput('k3m9p2qr7ai')).toEqual({ kind: 'text', text: 'k3m9p2qr7ai', chips: [] });
  });

  it('treats a 10-char token containing disallowed letters (o, l) as text', () => {
    expect(classifyInput('postgresql')).toEqual({ kind: 'text', text: 'postgresql', chips: [] });
  });

  it('classifies plain text with no chips', () => {
    expect(classifyInput('github')).toEqual({ kind: 'text', text: 'github', chips: [] });
  });

  it('splits a scope: operator from surrounding free text', () => {
    expect(classifyInput('scope:repo:engram gofmt')).toEqual({
      kind: 'text',
      text: 'gofmt',
      chips: [{ kind: 'scope', value: 'repo:engram' }]
    });
  });

  it('treats in: as an alias for scope:', () => {
    expect(classifyInput('in:repo:x')).toEqual({
      kind: 'operators',
      chips: [{ kind: 'scope', value: 'repo:x' }]
    });
  });

  it('collects #tag and tag: operators (AND, chip per occurrence)', () => {
    expect(classifyInput('#ci tag:lint')).toEqual({
      kind: 'operators',
      chips: [
        { kind: 'tag', value: 'ci' },
        { kind: 'tag', value: 'lint' }
      ]
    });
  });

  it('marks category chips known or unknown against KNOWN_CATEGORIES', () => {
    expect(classifyInput('is:gotcha is:bogus')).toEqual({
      kind: 'operators',
      chips: [
        { kind: 'category', value: 'gotcha', known: true },
        { kind: 'category', value: 'bogus', known: false }
      ]
    });
    expect(KNOWN_CATEGORIES).toContain('gotcha');
    expect(KNOWN_CATEGORIES).not.toContain('bogus');
  });

  it('classifies an operator with no value as pending', () => {
    expect(classifyInput('scope:')).toEqual({
      kind: 'operators',
      chips: [{ kind: 'pending', raw: 'scope:' }]
    });
  });

  it('collapses internal whitespace runs to single spaces', () => {
    expect(classifyInput('a   b')).toEqual({ kind: 'text', text: 'a b', chips: [] });
  });

  it('exports the UUID and short_id regexes the classifier itself uses', () => {
    expect(UUID_RE.test('753aba22-0000-4000-8000-000000000001')).toBe(true);
    expect(UUID_RE.test('not-a-uuid')).toBe(false);
    expect(SHORT_ID_RE.test('k3m9p2qr7a')).toBe(true);
    expect(SHORT_ID_RE.test('postgresql')).toBe(false);
  });
});
