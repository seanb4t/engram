import { describe, it, expect } from 'vitest';
import { QueryClient } from '@tanstack/svelte-query';
import { create } from '@bufbuild/protobuf';
import { createClient, ConnectError, Code, createRouterTransport } from '@connectrpc/connect';
import { EngramService, MemorySchema, Visibility, type Memory } from '$lib/gen/engram_pb';
import {
  buildStoreMemoryRequest,
  buildUpdateMemoryRequest,
  buildScheduleMemoryRequest,
  normalizeVisibility,
  createMemoryComposite,
  scheduleMemoryComposite,
  snapshotMemoryQueries,
  restoreMemoryQueries,
  applyUpdateOptimistic,
  applyDeleteOptimistic,
  applySetVisibilityOptimistic
} from './memory';

// ---------------------------------------------------------------------------
// Pure request builders
// ---------------------------------------------------------------------------

describe('buildStoreMemoryRequest', () => {
  it('builds a StoreMemoryRequest with the console source, no visibility field', () => {
    const req = buildStoreMemoryRequest({ content: 'hello', scope: 'repo:x', category: 'gotcha', tags: ['a'] });
    expect(req.content).toBe('hello');
    expect(req.scope).toBe('repo:x');
    expect(req.category).toBe('gotcha');
    expect(req.tags).toEqual(['a']);
    expect(req.source).toBe('console');
    expect('visibility' in req).toBe(false);
  });
});

describe('buildUpdateMemoryRequest', () => {
  it('builds an update_mask FieldMask restricted to exactly the supplied fields', () => {
    const req = buildUpdateMemoryRequest({ id: 'm1', content: 'new content' });
    expect(req.updateMask?.paths).toEqual(['content']);
    expect(req.content).toBe('new content');
  });

  it('never adds tags/summary/shared to the mask when they are not supplied', () => {
    const req = buildUpdateMemoryRequest({ id: 'm1', tags: ['x', 'y'] });
    expect(req.updateMask?.paths).toEqual(['tags']);
  });

  it('supports a multi-field diff (dirty-mask rule)', () => {
    const req = buildUpdateMemoryRequest({ id: 'm1', summary: 's', shared: true });
    expect(req.updateMask?.paths.sort()).toEqual(['shared', 'summary']);
  });
});

describe('buildScheduleMemoryRequest', () => {
  it('has no id field and carries not_before/not_after as Timestamps', () => {
    const notBefore = new Date('2026-08-01T00:00:00Z');
    const req = buildScheduleMemoryRequest({
      content: 'c',
      scope: 's',
      category: 'gotcha',
      notBefore
    });
    expect('id' in req).toBe(false);
    expect(req.notBefore).toBeDefined();
    expect(req.notAfter).toBeUndefined();
  });
});

describe('normalizeVisibility', () => {
  it('treats an empty string as private', () => {
    expect(normalizeVisibility('')).toBe('private');
  });
  it('passes through shared', () => {
    expect(normalizeVisibility('shared')).toBe('shared');
  });
});

// ---------------------------------------------------------------------------
// Create/schedule-as-shared composite state machine
// ---------------------------------------------------------------------------

function buildFakeClient(opts: {
  storeMemory?: (n: number) => { id: string; shortId: string };
  scheduleMemory?: (n: number) => { id: string; shortId: string };
  setVisibility?: (n: number) => { id: string; shortId: string } | never;
}) {
  const calls = { storeMemory: 0, scheduleMemory: 0, setVisibility: 0 };
  const seenVisibility: Visibility[] = [];
  const transport = createRouterTransport(({ service }) => {
    service(EngramService, {
      storeMemory: () => {
        calls.storeMemory += 1;
        return opts.storeMemory?.(calls.storeMemory) ?? { id: 'm1', shortId: 's1' };
      },
      scheduleMemory: () => {
        calls.scheduleMemory += 1;
        return opts.scheduleMemory?.(calls.scheduleMemory) ?? { id: 'm1', shortId: 's1' };
      },
      setVisibility: (req) => {
        calls.setVisibility += 1;
        seenVisibility.push(req.visibility);
        if (opts.setVisibility) return opts.setVisibility(calls.setVisibility);
        return { id: req.id, shortId: 's1' };
      }
    });
  });
  return { client: createClient(EngramService, transport), calls, seenVisibility };
}

describe('createMemoryComposite', () => {
  it('shared:false issues one storeMemory call and returns created', async () => {
    const { client, calls } = buildFakeClient({});
    const result = await createMemoryComposite(client, { content: 'c', scope: 's', category: 'gotcha' });
    expect(result).toEqual({ status: 'created', id: 'm1' });
    expect(calls.storeMemory).toBe(1);
    expect(calls.setVisibility).toBe(0);
  });

  it('shared:true issues storeMemory THEN setVisibility(SHARED) and returns created_shared', async () => {
    const { client, calls, seenVisibility } = buildFakeClient({});
    const result = await createMemoryComposite(client, {
      content: 'c',
      scope: 's',
      category: 'gotcha',
      shared: true
    });
    expect(result).toEqual({ status: 'created_shared', id: 'm1' });
    expect(calls.storeMemory).toBe(1);
    expect(calls.setVisibility).toBe(1);
    expect(seenVisibility).toEqual([Visibility.SHARED]);
  });

  it.each([
    ['Unauthenticated', Code.Unauthenticated],
    ['PermissionDenied', Code.PermissionDenied]
  ])(
    'a SetVisibility failure with %s is CAUGHT into created_private and issues EXACTLY ONE storeMemory (no duplicate create)',
    async (_name, code) => {
      const { client, calls } = buildFakeClient({
        setVisibility: () => {
          throw new ConnectError('secondary auth failure', code);
        }
      });
      const result = await createMemoryComposite(client, {
        content: 'c',
        scope: 's',
        category: 'gotcha',
        shared: true
      });
      expect(result).toEqual({ status: 'created_private', id: 'm1' });
      expect(calls.storeMemory).toBe(1);
      expect(calls.setVisibility).toBe(1);
    }
  );

  it('a PRIMARY storeMemory failure propagates (nothing created)', async () => {
    const { client } = buildFakeClient({
      storeMemory: () => {
        throw new ConnectError('bad content', Code.InvalidArgument);
      }
    });
    await expect(
      createMemoryComposite(client, { content: '', scope: 's', category: 'gotcha' })
    ).rejects.toMatchObject({ code: Code.InvalidArgument });
  });
});

describe('scheduleMemoryComposite', () => {
  it('shared:false issues one scheduleMemory call', async () => {
    const { client, calls } = buildFakeClient({});
    const result = await scheduleMemoryComposite(client, { content: 'c', scope: 's', category: 'gotcha' });
    expect(result).toEqual({ status: 'created', id: 'm1' });
    expect(calls.scheduleMemory).toBe(1);
    expect(calls.setVisibility).toBe(0);
  });

  it('shared:true issues scheduleMemory THEN setVisibility(SHARED) (two calls)', async () => {
    const { client, calls, seenVisibility } = buildFakeClient({});
    const result = await scheduleMemoryComposite(client, {
      content: 'c',
      scope: 's',
      category: 'gotcha',
      shared: true
    });
    expect(result).toEqual({ status: 'created_shared', id: 'm1' });
    expect(calls.scheduleMemory).toBe(1);
    expect(calls.setVisibility).toBe(1);
    expect(seenVisibility).toEqual([Visibility.SHARED]);
  });

  it('a scheduled+shared SetVisibility failure keeps the record and issues EXACTLY ONE scheduleMemory', async () => {
    const { client, calls } = buildFakeClient({
      setVisibility: () => {
        throw new ConnectError('secondary auth failure', Code.Unauthenticated);
      }
    });
    const result = await scheduleMemoryComposite(client, {
      content: 'c',
      scope: 's',
      category: 'gotcha',
      shared: true
    });
    expect(result).toEqual({ status: 'created_private', id: 'm1' });
    expect(calls.scheduleMemory).toBe(1);
    expect(calls.setVisibility).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// Pure cache-transform factories against a real QueryClient
// ---------------------------------------------------------------------------

function memory(overrides: { id?: string; content?: string; visibility?: string }): Memory {
  return create(MemorySchema, { id: 'm1', content: 'c', scope: 's', visibility: 'private', tags: [], ...overrides });
}

describe('applyUpdateOptimistic / snapshot / restore', () => {
  it('patches getMemory + matching list/search pages, and rollback restores the exact prior cache state', () => {
    const qc = new QueryClient();
    const original = { memory: memory({}) };
    const listKey = ['listMemories', 's', [], '', 50, 0];
    const searchKey = ['searchMemories', 'q', 's'];
    qc.setQueryData(['getMemory', 'm1'], original);
    qc.setQueryData(listKey, { memories: [memory({})], total: 1n });
    qc.setQueryData(searchKey, { memories: [memory({})] });

    const snapshot = snapshotMemoryQueries(qc, 'm1');
    applyUpdateOptimistic(qc, { id: 'm1', content: 'updated content' });

    expect((qc.getQueryData(['getMemory', 'm1']) as any).memory.content).toBe('updated content');
    expect((qc.getQueryData(listKey) as any).memories[0].content).toBe('updated content');
    expect((qc.getQueryData(searchKey) as any).memories[0].content).toBe('updated content');

    restoreMemoryQueries(qc, snapshot);

    expect(qc.getQueryData(['getMemory', 'm1'])).toEqual(original);
    expect(qc.getQueryData(listKey)).toEqual({ memories: [memory({})], total: 1n });
    expect(qc.getQueryData(searchKey)).toEqual({ memories: [memory({})] });
  });

  it('DROPS the record from a visibility-filtered list page on a private→shared edit (WR-01)', () => {
    const qc = new QueryClient();
    const unfilteredKey = ['listMemories', 's', [], '', 50, 0];
    const privateFilteredKey = ['listMemories', 's', [], 'private', 50, 0];
    qc.setQueryData(unfilteredKey, { memories: [memory({ visibility: 'private' })], total: 1n });
    qc.setQueryData(privateFilteredKey, { memories: [memory({ visibility: 'private' })], total: 1n });
    qc.setQueryData(['getMemory', 'm1'], { memory: memory({ visibility: 'private' }) });

    const snapshot = snapshotMemoryQueries(qc, 'm1');
    applyUpdateOptimistic(qc, { id: 'm1', shared: true });

    // Unfiltered page keeps the record, now patched to shared.
    expect((qc.getQueryData(unfilteredKey) as any).memories[0].visibility).toBe('shared');
    // Private-filtered page drops the now-shared record and decrements total.
    expect((qc.getQueryData(privateFilteredKey) as any).memories).toEqual([]);
    expect((qc.getQueryData(privateFilteredKey) as any).total).toBe(0n);
    // getMemory (not a list page) still reflects the new visibility.
    expect((qc.getQueryData(['getMemory', 'm1']) as any).memory.visibility).toBe('shared');

    restoreMemoryQueries(qc, snapshot);
    expect((qc.getQueryData(privateFilteredKey) as any).memories[0].visibility).toBe('private');
  });
});

describe('applyDeleteOptimistic', () => {
  it('removes the row from list/search pages and decrements total, rollback restores it', () => {
    const qc = new QueryClient();
    const listKey = ['listMemories', 's', [], '', 50, 0];
    qc.setQueryData(['getMemory', 'm1'], { memory: memory({}) });
    qc.setQueryData(listKey, { memories: [memory({}), memory({ id: 'm2' })], total: 2n });

    const snapshot = snapshotMemoryQueries(qc, 'm1');
    applyDeleteOptimistic(qc, 'm1');

    const after = qc.getQueryData(listKey) as any;
    expect(after.memories.map((m: Memory) => m.id)).toEqual(['m2']);
    expect(after.total).toBe(1n);

    restoreMemoryQueries(qc, snapshot);
    const restored = qc.getQueryData(listKey) as any;
    expect(restored.memories.map((m: Memory) => m.id)).toEqual(['m1', 'm2']);
    expect(restored.total).toBe(2n);
  });
});

describe('applySetVisibilityOptimistic', () => {
  it('patches visibility on an unfiltered page but DROPS the record from a page whose visibility filter no longer matches', () => {
    const qc = new QueryClient();
    const unfilteredKey = ['listMemories', 's', [], '', 50, 0];
    const privateFilteredKey = ['listMemories', 's', [], 'private', 50, 0];
    qc.setQueryData(unfilteredKey, { memories: [memory({ visibility: 'private' })], total: 1n });
    qc.setQueryData(privateFilteredKey, { memories: [memory({ visibility: 'private' })], total: 1n });
    qc.setQueryData(['getMemory', 'm1'], { memory: memory({ visibility: 'private' }) });

    const snapshot = snapshotMemoryQueries(qc, 'm1');
    applySetVisibilityOptimistic(qc, 'm1', 'shared');

    expect((qc.getQueryData(unfilteredKey) as any).memories[0].visibility).toBe('shared');
    expect((qc.getQueryData(privateFilteredKey) as any).memories).toEqual([]);
    expect((qc.getQueryData(privateFilteredKey) as any).total).toBe(0n);
    expect((qc.getQueryData(['getMemory', 'm1']) as any).memory.visibility).toBe('shared');

    restoreMemoryQueries(qc, snapshot);
    expect((qc.getQueryData(privateFilteredKey) as any).memories[0].visibility).toBe('private');
  });
});
