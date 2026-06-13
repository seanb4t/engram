import { describe, it, expect } from 'vitest';
import { parseScope } from './scope';

describe('parseScope', () => {
  it('parses a github repo scope into type/org/name', () => {
    const s = parseScope('repo:github.com/fzymgc-house/selfhosted-cluster');
    expect(s.type).toBe('repo');
    expect(s.org).toBe('fzymgc-house');
    expect(s.name).toBe('selfhosted-cluster');
    expect(s.full).toBe('repo:github.com/fzymgc-house/selfhosted-cluster');
  });
  it('parses a discovery scope (nested repo)', () => {
    const s = parseScope('discovery:repo:github.com/seanb4t/engram');
    expect(s.type).toBe('discovery');
    expect(s.org).toBe('seanb4t');
    expect(s.name).toBe('engram');
  });
  it('parses a project scope with no org', () => {
    const s = parseScope('project:selfhosted-cluster');
    expect(s.type).toBe('project');
    expect(s.org).toBe('');
    expect(s.name).toBe('selfhosted-cluster');
  });
  it('falls back gracefully for an unknown shape', () => {
    const s = parseScope('weird');
    expect(s.type).toBe('');
    expect(s.name).toBe('weird');
    expect(s.full).toBe('weird');
  });
});
