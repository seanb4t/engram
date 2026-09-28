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
  returnPath: '/search?sel=abc',
  kind: 'memory',
  mode: 'create',
  recordId: null,
  values: { content: 'hello' }
};

const discoveryDraft: ResumeDraft = {
  returnPath: '/discovery?scope=discovery%3Arepo%3Ax',
  kind: 'discovery',
  mode: 'create',
  recordId: null,
  values: { content: 'a discovery draft' }
};

const supersedeDraft: ResumeDraft = {
  returnPath: '/search?q=foo',
  kind: 'supersede',
  targets: ['a', 'b'],
  fields: { content: 'merged content' },
  idempotencyKey: 'idem-1'
};

const archiveDraft: ResumeDraft = {
  returnPath: '/scheduled?state=expired',
  kind: 'archive',
  mode: 'archive',
  ids: ['a', 'b']
};

const deleteDraft: ResumeDraft = {
  returnPath: '/rules',
  kind: 'delete',
  id: 'a'
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
    expect(typeof peeked?.v).toBe('number');
    expect(typeof peeked?.ts).toBe('number');
  });

  it.each([
    ['memory', baseDraft],
    ['discovery', discoveryDraft],
    ['supersede', supersedeDraft],
    ['archive', archiveDraft],
    ['delete', deleteDraft]
  ])('%s: persists and peeks back with v === 2', (_kind, draft) => {
    persistResume(draft);
    const peeked = peekResume();
    expect(peeked).not.toBeNull();
    expect(peeked?.v).toBe(2);
    expect(peeked?.kind).toBe(draft.kind);
  });

  it('peeks null when the stored envelope has a wrong schema version', () => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...baseDraft, v: 999, ts: Date.now() }));
    expect(peekResume()).toBeNull();
  });

  it('a v1 memory envelope still peeks (drafts persisted by a pre-phase build survive the upgrade)', () => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...baseDraft, v: 1, ts: Date.now() }));
    expect(peekResume()).not.toBeNull();
  });

  it('a v1 envelope carrying a curation kind peeks null', () => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...supersedeDraft, v: 1, ts: Date.now() }));
    expect(peekResume()).toBeNull();
  });

  it('a v2 envelope of each of the five kinds peeks', () => {
    for (const draft of [baseDraft, discoveryDraft, supersedeDraft, archiveDraft, deleteDraft]) {
      sessionStorage.clear();
      sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...draft, v: 2, ts: Date.now() }));
      expect(peekResume()).not.toBeNull();
    }
  });

  it('TTL boundary: an envelope aged exactly RESUME_TTL_MS (600000 ms) still peeks', () => {
    const ts = Date.now() - 600000;
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...baseDraft, v: 2, ts }));
    expect(peekResume()).not.toBeNull();
  });

  it('TTL boundary: an envelope aged 600001 ms peeks null', () => {
    const ts = Date.now() - 600001;
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...baseDraft, v: 2, ts }));
    expect(peekResume()).toBeNull();
  });

  it('peeks null on malformed JSON', () => {
    sessionStorage.setItem(RESUME_KEY, '{not json');
    expect(peekResume()).toBeNull();
  });

  it.each([
    ['missing kind', { ...baseDraft, v: 2, ts: Date.now(), kind: undefined }],
    ['bad kind', { ...baseDraft, v: 2, ts: Date.now(), kind: 'rule' }],
    ['bad mode', { ...baseDraft, v: 2, ts: Date.now(), mode: 'delete' }],
    ['non-object values', { ...baseDraft, v: 2, ts: Date.now(), values: 'nope' }],
    ['array values', { ...baseDraft, v: 2, ts: Date.now(), values: [] }],
    ['bad recordId type', { ...baseDraft, v: 2, ts: Date.now(), recordId: 42 }]
  ])('peeks null on a structurally-invalid envelope: %s', (_name, malformed) => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify(malformed));
    expect(peekResume()).toBeNull();
  });

  it('supersede: empty targets array peeks null', () => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...supersedeDraft, v: 2, ts: Date.now(), targets: [] }));
    expect(peekResume()).toBeNull();
  });

  it('supersede: empty idempotencyKey peeks null', () => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...supersedeDraft, v: 2, ts: Date.now(), idempotencyKey: '' }));
    expect(peekResume()).toBeNull();
  });

  it('supersede: empty fields object still peeks (the dialog re-validates on reopen)', () => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...supersedeDraft, v: 2, ts: Date.now(), fields: {} }));
    expect(peekResume()).not.toBeNull();
  });

  it('supersede: targets round-trip in exactly the persisted order, no sort or de-dup', () => {
    persistResume({ ...supersedeDraft, targets: ['b', 'a', 'b'] });
    const peeked = peekResume();
    expect(peeked && 'targets' in peeked ? peeked.targets : undefined).toEqual(['b', 'a', 'b']);
  });

  it('archive: empty ids array peeks null', () => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...archiveDraft, v: 2, ts: Date.now(), ids: [] }));
    expect(peekResume()).toBeNull();
  });

  it('archive: ids round-trip in exactly the persisted order, no sort or de-dup', () => {
    persistResume({ ...archiveDraft, ids: ['b', 'a', 'b'] });
    const peeked = peekResume();
    expect(peeked && 'ids' in peeked ? peeked.ids : undefined).toEqual(['b', 'a', 'b']);
  });

  it('delete: empty id peeks null', () => {
    sessionStorage.setItem(RESUME_KEY, JSON.stringify({ ...deleteDraft, v: 2, ts: Date.now(), id: '' }));
    expect(peekResume()).toBeNull();
  });

  it('ts precision: ts is Date.now() in integer milliseconds, no rounding in the TTL comparison', () => {
    persistResume(baseDraft);
    const peeked = peekResume();
    expect(Number.isInteger(peeked?.ts)).toBe(true);
  });

  it('a draft whose fields carry a bigint persists nothing and persistResume never throws', () => {
    expect(() =>
      persistResume({ ...supersedeDraft, fields: { bad: 1n as unknown as number } })
    ).not.toThrow();
    expect(peekResume()).toBeNull();
    expect(sessionStorage.getItem(RESUME_KEY)).toBeNull();
  });

  it('consumeResume deletes the stored envelope', () => {
    persistResume(baseDraft);
    expect(peekResume()).not.toBeNull();
    consumeResume();
    expect(peekResume()).toBeNull();
    expect(sessionStorage.getItem(RESUME_KEY)).toBeNull();
  });

  it('normalizeReturnPath strips a leading /ui base prefix (no double /ui/ui)', () => {
    expect(normalizeReturnPath('/ui/search?sel=x')).toBe('/search?sel=x');
    expect(normalizeReturnPath('/ui')).toBe('/');
    expect(normalizeReturnPath('/search?sel=x')).toBe('/search?sel=x');
  });

  it('isAllowedDestination accepts search/discovery/rules/scheduled (and /ui-prefixed forms) and rejects the deleted route, /evil and an absolute URL', () => {
    expect(isAllowedDestination('/search')).toBe(true);
    expect(isAllowedDestination('/ui/search?q=foo')).toBe(true);
    expect(isAllowedDestination('/discovery/repo')).toBe(true);
    expect(isAllowedDestination('/rules')).toBe(true);
    expect(isAllowedDestination('/ui/rules')).toBe(true);
    expect(isAllowedDestination('/scheduled?state=expired')).toBe(true);
    expect(isAllowedDestination('/ui/scheduled')).toBe(true);
    expect(isAllowedDestination('/observe')).toBe(false);
    expect(isAllowedDestination('/evil')).toBe(false);
    expect(isAllowedDestination('https://evil.example/search')).toBe(false);
  });
});
