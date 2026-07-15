import { describe, it, expect } from 'vitest';
import { QueryClient } from '@tanstack/svelte-query';
import { create } from '@bufbuild/protobuf';
import { createClient, ConnectError, Code, createRouterTransport } from '@connectrpc/connect';
import { EngramService, MemorySchema, Visibility, type Memory } from '$lib/gen/engram_pb';
import * as discoveryMutations from './discovery';
import {
  buildStoreDiscoveryRequest,
  createDiscoveryComposite,
  snapshotDiscoveryQueries,
  restoreDiscoveryQueries,
  applyDeleteDiscoveryOptimistic,
  applySetDiscoveryVisibilityOptimistic
} from './discovery';

// ---------------------------------------------------------------------------
// No discovery edit/update hook exists (D-04 fence)
// ---------------------------------------------------------------------------

describe('discovery.ts exports', () => {
  it('exports exactly the three mutation hooks (create/delete/set-visibility) and no update/edit hook', () => {
    expect(typeof discoveryMutations.useCreateDiscovery).toBe('function');
    expect(typeof discoveryMutations.useDeleteDiscovery).toBe('function');
    expect(typeof discoveryMutations.useSetDiscoveryVisibility).toBe('function');
    expect((discoveryMutations as Record<string, unknown>).useUpdateDiscovery).toBeUndefined();
    expect((discoveryMutations as Record<string, unknown>).useEditDiscovery).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// Pure request builder
// ---------------------------------------------------------------------------

describe('buildStoreDiscoveryRequest', () => {
  it('builds a StoreDiscoveryRequest with non-empty content, kind, discovery:-prefixed scope, and typed Citation[]', () => {
    const req = buildStoreDiscoveryRequest({
      content: 'a codebase map',
      kind: 'map',
      scope: 'discovery:repo:engram',
      citations: [{ kind: 'url', ref: 'https://example.com/doc' }, { kind: 'file', ref: 'internal/store/store.go' }]
    });
    expect(req.content).toBe('a codebase map');
    expect(req.kind).toBe('map');
    expect(req.scope).toBe('discovery:repo:engram');
    expect(req.citations).toHaveLength(2);
    expect(req.citations[0]).toMatchObject({ kind: 'url', ref: 'https://example.com/doc' });
    expect(req.citations[1]).toMatchObject({ kind: 'file', ref: 'internal/store/store.go' });
  });
});

// ---------------------------------------------------------------------------
// Create-as-shared composite state machine (mirrors memory.ts)
// ---------------------------------------------------------------------------

function buildFakeClient(opts: {
  storeDiscovery?: (n: number) => { id: string; shortId: string };
  setVisibility?: (n: number) => { id: string; shortId: string } | never;
}) {
  const calls = { storeDiscovery: 0, setVisibility: 0 };
  const seenVisibility: Visibility[] = [];
  const transport = createRouterTransport(({ service }) => {
    service(EngramService, {
      storeDiscovery: () => {
        calls.storeDiscovery += 1;
        return opts.storeDiscovery?.(calls.storeDiscovery) ?? { id: 'd1', shortId: 's1' };
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

describe('createDiscoveryComposite', () => {
  it('shared:false issues one storeDiscovery call', async () => {
    const { client, calls } = buildFakeClient({});
    const result = await createDiscoveryComposite(client, {
      content: 'c',
      kind: 'fact',
      scope: 'discovery:repo:x',
      citations: [{ kind: 'file', ref: 'f.go' }]
    });
    expect(result).toEqual({ status: 'created', id: 'd1' });
    expect(calls.storeDiscovery).toBe(1);
    expect(calls.setVisibility).toBe(0);
  });

  it('shared:true issues storeDiscovery THEN setVisibility(SHARED)', async () => {
    const { client, calls, seenVisibility } = buildFakeClient({});
    const result = await createDiscoveryComposite(client, {
      content: 'c',
      kind: 'fact',
      scope: 'discovery:repo:x',
      citations: [{ kind: 'file', ref: 'f.go' }],
      shared: true
    });
    expect(result).toEqual({ status: 'created_shared', id: 'd1' });
    expect(calls.storeDiscovery).toBe(1);
    expect(calls.setVisibility).toBe(1);
    expect(seenVisibility).toEqual([Visibility.SHARED]);
  });

  it.each([
    ['Unauthenticated', Code.Unauthenticated],
    ['PermissionDenied', Code.PermissionDenied]
  ])(
    'a SetVisibility failure with %s is CAUGHT into created_private and issues EXACTLY ONE storeDiscovery',
    async (_name, code) => {
      const { client, calls } = buildFakeClient({
        setVisibility: () => {
          throw new ConnectError('secondary auth failure', code);
        }
      });
      const result = await createDiscoveryComposite(client, {
        content: 'c',
        kind: 'fact',
        scope: 'discovery:repo:x',
        citations: [{ kind: 'file', ref: 'f.go' }],
        shared: true
      });
      expect(result).toEqual({ status: 'created_private', id: 'd1' });
      expect(calls.storeDiscovery).toBe(1);
      expect(calls.setVisibility).toBe(1);
    }
  );
});

// ---------------------------------------------------------------------------
// Pure cache-transform factories against a real QueryClient
// ---------------------------------------------------------------------------

function discovery(overrides: { id?: string; visibility?: string }): Memory {
  return create(MemorySchema, {
    id: 'd1',
    content: 'c',
    scope: 'discovery:repo:x',
    visibility: 'private',
    tags: [],
    ...overrides
  });
}

describe('applyDeleteDiscoveryOptimistic / snapshot / restore', () => {
  it('removes the row from searchDiscoveries pages; rollback restores it', () => {
    const qc = new QueryClient();
    const searchKey = ['searchDiscoveries', 'q', 'discovery:repo:x'];
    qc.setQueryData(searchKey, { discoveries: [discovery({}), discovery({ id: 'd2' })] });
    qc.setQueryData(['getMemory', 'd1'], { memory: discovery({}) });

    const snapshot = snapshotDiscoveryQueries(qc, 'd1');
    applyDeleteDiscoveryOptimistic(qc, 'd1');

    expect((qc.getQueryData(searchKey) as any).discoveries.map((m: Memory) => m.id)).toEqual(['d2']);

    restoreDiscoveryQueries(qc, snapshot);
    expect((qc.getQueryData(searchKey) as any).discoveries.map((m: Memory) => m.id)).toEqual(['d1', 'd2']);
    expect(qc.getQueryData(['getMemory', 'd1'])).toEqual({ memory: discovery({}) });
  });
});

describe('applySetDiscoveryVisibilityOptimistic', () => {
  it('patches visibility in place (searchDiscoveries carries no visibility filter to drop against)', () => {
    const qc = new QueryClient();
    const searchKey = ['searchDiscoveries', 'q', 'discovery:repo:x'];
    qc.setQueryData(searchKey, { discoveries: [discovery({ visibility: 'private' })] });

    const snapshot = snapshotDiscoveryQueries(qc, 'd1');
    applySetDiscoveryVisibilityOptimistic(qc, 'd1', 'shared');

    expect((qc.getQueryData(searchKey) as any).discoveries[0].visibility).toBe('shared');

    restoreDiscoveryQueries(qc, snapshot);
    expect((qc.getQueryData(searchKey) as any).discoveries[0].visibility).toBe('private');
  });
});
