import { describe, it, expect, beforeEach } from 'vitest';
import {
  persistResume,
  peekResume,
  consumeResume,
  normalizeReturnPath,
  isAllowedDestination,
  RESUME_KEY,
  type ResumeDraft
} from './resume';

// Node tier runs `environment: 'node'` (no DOM) -- sessionStorage is not a
// Node global, so install a minimal in-memory Storage stub, mirroring
// vitest-setup.ts's localStorage stub for mode-watcher.
function installSessionStorageStub() {
  const store: Record<string, string> = {};
  Object.defineProperty(globalThis, 'sessionStorage', {
    value: {
      getItem: (k: string) => store[k] ?? null,
      setItem: (k: string, v: string) => {
        store[k] = v;
      },
      removeItem: (k: string) => {
        delete store[k];
      },
      clear: () => {
        for (const k in store) delete store[k];
      },
      get length() {
        return Object.keys(store).length;
      },
      key: (i: number) => Object.keys(store)[i] ?? null
    },
    configurable: true,
    writable: true
  });
}

const baseDraft: ResumeDraft = {
  returnPath: '/observe?sel=abc',
  kind: 'memory',
  mode: 'create',
  recordId: null,
  values: { content: 'hello' }
};

describe('resume', () => {
  beforeEach(() => {
    installSessionStorageStub();
  });

  it('round-trips a draft (no v/ts at the call site) and stamps both on peek', () => {
    persistResume(baseDraft);
    const peeked = peekResume();
    expect(peeked).not.toBeNull();
    expect(peeked?.returnPath).toBe(baseDraft.returnPath);
    expect(peeked?.kind).toBe('memory');
    expect(peeked?.mode).toBe('create');
    expect(peeked?.recordId).toBeNull();
    expect(peeked?.values).toEqual({ content: 'hello' });
    expect(typeof peeked?.v).toBe('number');
    expect(typeof peeked?.ts).toBe('number');
  });

  it('peeks null when the stored envelope has a wrong schema version', () => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...baseDraft, v: 999, ts: Date.now() }));
    expect(peekResume()).toBeNull();
  });

  it('peeks null when the stored envelope has expired (ts older than the TTL)', () => {
    const expiredTs = Date.now() - 11 * 60 * 1000; // 11min > 10min TTL
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...baseDraft, v: 1, ts: expiredTs }));
    expect(peekResume()).toBeNull();
  });

  it('peeks null on malformed JSON', () => {
    sessionStorage.setItem(RESUME_KEY, '{not json');
    expect(peekResume()).toBeNull();
  });

  it.each([
    ['missing kind', { ...baseDraft, v: 1, ts: Date.now(), kind: undefined }],
    ['bad kind', { ...baseDraft, v: 1, ts: Date.now(), kind: 'rule' }],
    ['bad mode', { ...baseDraft, v: 1, ts: Date.now(), mode: 'delete' }],
    ['non-object values', { ...baseDraft, v: 1, ts: Date.now(), values: 'nope' }],
    ['array values', { ...baseDraft, v: 1, ts: Date.now(), values: [] }],
    ['bad recordId type', { ...baseDraft, v: 1, ts: Date.now(), recordId: 42 }],
    ['non-string dirtyPaths entries', { ...baseDraft, v: 1, ts: Date.now(), dirtyPaths: [1, 2] }]
  ])('peeks null on a structurally-invalid envelope: %s', (_name, malformed) => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify(malformed));
    expect(peekResume()).toBeNull();
  });

  it('consumeResume deletes the stored envelope', () => {
    persistResume(baseDraft);
    expect(peekResume()).not.toBeNull();
    consumeResume();
    expect(peekResume()).toBeNull();
    expect(sessionStorage.getItem(RESUME_KEY)).toBeNull();
  });

  it('normalizeReturnPath strips a leading /ui base prefix (no double /ui/ui)', () => {
    expect(normalizeReturnPath('/ui/observe?sel=x')).toBe('/observe?sel=x');
    expect(normalizeReturnPath('/ui')).toBe('/');
    expect(normalizeReturnPath('/observe?sel=x')).toBe('/observe?sel=x');
  });

  it('isAllowedDestination accepts observe/search/discovery and rejects everything else', () => {
    expect(isAllowedDestination('/observe')).toBe(true);
    expect(isAllowedDestination('/ui/observe?sel=x')).toBe(true);
    expect(isAllowedDestination('/search?q=foo')).toBe(true);
    expect(isAllowedDestination('/discovery/repo')).toBe(true);
    expect(isAllowedDestination('/evil')).toBe(false);
    expect(isAllowedDestination('https://evil.example/observe')).toBe(false);
  });
});
