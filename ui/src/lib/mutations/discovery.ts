import { createMutation, useQueryClient, type QueryClient } from '@tanstack/svelte-query';
import { create } from '@bufbuild/protobuf';
import { toast } from 'svelte-sonner';
import { engramWrite } from '$lib/client';
import {
  StoreDiscoveryRequestSchema,
  CitationSchema,
  DeleteMemoryRequestSchema,
  SetVisibilityRequestSchema,
  Visibility,
  type Memory,
  type StoreDiscoveryRequest
} from '$lib/gen/engram_pb';
import { type CacheEntry, type EngramWriteClient, type WriteResult } from './memory';

// Mirrors ui/src/lib/mutations/memory.ts's thunk/onMutate/onError/onSettled
// shape (Task 1). No discovery update/edit hook is exported here -- D-04
// fence: discoveries support create/delete/set-visibility only.

export interface DiscoveryCitationInput {
  kind: 'file' | 'commit' | 'url' | 'repo';
  ref: string;
  locator?: string;
  pin?: string;
  excerpt?: string;
}

export interface CreateDiscoveryVars {
  content: string;
  kind: 'map' | 'fact';
  // Must be `discovery:`-prefixed (e.g. `discovery:repo:<repo>`); the caller
  // (form layer, Plan 05) owns scope construction. Client-side presence
  // checks are UX-only -- the server's validateStoreDiscovery/protovalidate
  // (tools.go:575-602, engram.proto:130) is the authoritative validator.
  scope: string;
  citations: DiscoveryCitationInput[];
  tags?: string[];
  summary?: string;
  shared?: boolean;
}

// ---------------------------------------------------------------------------
// Pure request builder (node-testable, no client/QueryClient dependency)
// ---------------------------------------------------------------------------

export function buildStoreDiscoveryRequest(vars: CreateDiscoveryVars): StoreDiscoveryRequest {
  return create(StoreDiscoveryRequestSchema, {
    content: vars.content,
    kind: vars.kind,
    scope: vars.scope,
    tags: vars.tags ?? [],
    summary: vars.summary ?? '',
    citations: vars.citations.map((c) =>
      create(CitationSchema, {
        kind: c.kind,
        ref: c.ref,
        locator: c.locator ?? '',
        pin: c.pin ?? '',
        excerpt: c.excerpt ?? ''
      })
    )
  });
}

// ---------------------------------------------------------------------------
// Create-as-shared composite -- same STATE MACHINE as memory.ts's
// createMemoryComposite (Codex round-3 HIGH): StoreDiscoveryRequest carries
// no visibility field and no idempotency key/id, so a secondary
// SetVisibility failure (incl. Unauthenticated/PermissionDenied) is CAUGHT
// and returned as `created_private`, never rethrown -- a rethrow would drive
// the form's whole-create D-09 resubmit and re-issue storeDiscovery,
// DUPLICATING the record.
// ---------------------------------------------------------------------------

export async function createDiscoveryComposite(
  client: EngramWriteClient,
  vars: CreateDiscoveryVars
): Promise<WriteResult> {
  const resp = await client.storeDiscovery(buildStoreDiscoveryRequest(vars));
  if (!vars.shared) return { status: 'created', id: resp.id };
  try {
    await client.setVisibility(create(SetVisibilityRequestSchema, { id: resp.id, visibility: Visibility.SHARED }));
    return { status: 'created_shared', id: resp.id };
  } catch {
    return { status: 'created_private', id: resp.id };
  }
}

// ---------------------------------------------------------------------------
// Pure cache-transform factories (node-testable against a real QueryClient)
// ---------------------------------------------------------------------------

export function snapshotDiscoveryQueries(queryClient: QueryClient, id?: string): CacheEntry[] {
  const entries: CacheEntry[] = [...(queryClient.getQueriesData({ queryKey: ['searchDiscoveries'] }) as CacheEntry[])];
  if (id) entries.push(...(queryClient.getQueriesData({ queryKey: ['getMemory', id] }) as CacheEntry[]));
  return entries;
}

export function restoreDiscoveryQueries(queryClient: QueryClient, snapshot: CacheEntry[]): void {
  for (const [key, data] of snapshot) queryClient.setQueryData(key as readonly unknown[], data);
}

type DiscoveryListResponse = { discoveries?: Memory[] };

function mapDiscoveriesField<T extends DiscoveryListResponse | undefined>(
  old: T,
  fn: (m: Memory) => Memory | null
): T {
  if (!old || !Array.isArray(old.discoveries)) return old;
  const kept: Memory[] = [];
  for (const m of old.discoveries) {
    const next = fn(m);
    if (next) kept.push(next);
  }
  return { ...old, discoveries: kept };
}

// searchDiscoveries keys (['searchDiscoveries', query, scope]) carry no
// visibility filter, unlike listMemoriesKey -- so unlike memory.ts's
// applySetVisibilityOptimistic, no filtered-membership removal is needed
// here, only a field patch.
function applyToDiscoveryCaches(queryClient: QueryClient, id: string, fn: (m: Memory) => Memory | null): void {
  for (const [key, data] of queryClient.getQueriesData({ queryKey: ['searchDiscoveries'] })) {
    queryClient.setQueryData(
      key,
      mapDiscoveriesField(data as DiscoveryListResponse | undefined, (m) => (m.id === id ? fn(m) : m))
    );
  }
  queryClient.setQueryData(['getMemory', id], (old: { memory?: Memory } | undefined) => {
    if (!old?.memory || old.memory.id !== id) return old;
    const next = fn(old.memory);
    return next ? { ...old, memory: next } : old;
  });
}

export function applyDeleteDiscoveryOptimistic(queryClient: QueryClient, id: string): void {
  applyToDiscoveryCaches(queryClient, id, () => null);
}

export function applySetDiscoveryVisibilityOptimistic(
  queryClient: QueryClient,
  id: string,
  visibility: 'private' | 'shared'
): void {
  applyToDiscoveryCaches(queryClient, id, (m) => ({ ...m, visibility }));
}

// ---------------------------------------------------------------------------
// Mutation hooks (exactly three: create, delete, set-visibility -- D-04, no
// discovery update/edit hook). The pure factories above are additional
// named module exports (the node-test surface), distinct from these three
// createMutation wrappers (Codex round-4 LOW).
// ---------------------------------------------------------------------------

export function useCreateDiscovery() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (vars: CreateDiscoveryVars) => createDiscoveryComposite(engramWrite, vars),
    // Sole toast site (Codex round-4 LOW) -- the composite never toasts.
    onSuccess: (result: WriteResult) => {
      if (result.status === 'created_private') toast.warning('created (private) — sharing failed');
      else toast.success('created');
    },
    onError: () => {
      toast.error('write failed');
    },
    onSettled: () => {
      // Create is invalidate-only (StoreDiscoveryResponse is {id, short_id}).
      queryClient.invalidateQueries({ queryKey: ['searchDiscoveries'] });
      queryClient.invalidateQueries({ queryKey: ['listScopes'] });
    }
  }));
}

export function useDeleteDiscovery() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    // Discoveries are records addressed by id -- deleted via DeleteMemory.
    mutationFn: (vars: { id: string }) => engramWrite.deleteMemory(create(DeleteMemoryRequestSchema, { id: vars.id })),
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey: ['searchDiscoveries'] });
      await queryClient.cancelQueries({ queryKey: ['getMemory', vars.id] });
      const snapshot = snapshotDiscoveryQueries(queryClient, vars.id);
      applyDeleteDiscoveryOptimistic(queryClient, vars.id);
      return { snapshot };
    },
    onError: (_err, _vars, context) => {
      if (context?.snapshot) restoreDiscoveryQueries(queryClient, context.snapshot);
      toast.error('write failed');
    },
    onSuccess: () => {
      toast.success('deleted');
    },
    onSettled: (_data, _err, vars) => {
      queryClient.invalidateQueries({ queryKey: ['searchDiscoveries'] });
      queryClient.invalidateQueries({ queryKey: ['getMemory', vars.id] });
      queryClient.invalidateQueries({ queryKey: ['listScopes'] });
    }
  }));
}

export interface SetDiscoveryVisibilityVars {
  id: string;
  visibility: 'private' | 'shared';
}

export function useSetDiscoveryVisibility() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (vars: SetDiscoveryVisibilityVars) =>
      engramWrite.setVisibility(
        create(SetVisibilityRequestSchema, {
          id: vars.id,
          visibility: vars.visibility === 'shared' ? Visibility.SHARED : Visibility.PRIVATE
        })
      ),
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey: ['searchDiscoveries'] });
      await queryClient.cancelQueries({ queryKey: ['getMemory', vars.id] });
      const snapshot = snapshotDiscoveryQueries(queryClient, vars.id);
      applySetDiscoveryVisibilityOptimistic(queryClient, vars.id, vars.visibility);
      return { snapshot };
    },
    onError: (_err, _vars, context) => {
      if (context?.snapshot) restoreDiscoveryQueries(queryClient, context.snapshot);
      toast.error('write failed');
    },
    onSuccess: () => {
      toast.success('saved');
    },
    onSettled: (_data, _err, vars) => {
      queryClient.invalidateQueries({ queryKey: ['searchDiscoveries'] });
      queryClient.invalidateQueries({ queryKey: ['getMemory', vars.id] });
    }
  }));
}
