import { createMutation, useQueryClient, type QueryClient } from '@tanstack/svelte-query';
import { create } from '@bufbuild/protobuf';
import { FieldMaskSchema, timestampFromDate } from '@bufbuild/protobuf/wkt';
import { toast } from 'svelte-sonner';
import { engramWrite } from '$lib/client';
import {
  StoreMemoryRequestSchema,
  UpdateMemoryRequestSchema,
  DeleteMemoryRequestSchema,
  SetVisibilityRequestSchema,
  ScheduleMemoryRequestSchema,
  Visibility,
  type Memory,
  type StoreMemoryRequest,
  type UpdateMemoryRequest,
  type ScheduleMemoryRequest
} from '$lib/gen/engram_pb';

// Stable source identifier stamped on every console-originated write, so
// records created via this UI are distinguishable in `source` from MCP-tool
// writes (no existing convention to match — Claude's discretion, plan 04).
export const CONSOLE_SOURCE = 'console';

// Client-side field presence checked below is UX fast-fail only; the
// server's protovalidate interceptor (Phase 15) is the sole authoritative
// validator (T-19-44).

export type WriteResult = { status: 'created' | 'created_shared' | 'created_private'; id: string };

export type EngramWriteClient = typeof engramWrite;

// ---------------------------------------------------------------------------
// Pure request builders (node-testable, no client/QueryClient dependency)
// ---------------------------------------------------------------------------

export interface CreateMemoryVars {
  content: string;
  scope: string;
  category: string;
  tags?: string[];
  summary?: string;
  shared?: boolean;
}

export function buildStoreMemoryRequest(vars: CreateMemoryVars): StoreMemoryRequest {
  return create(StoreMemoryRequestSchema, {
    content: vars.content,
    scope: vars.scope,
    category: vars.category,
    tags: vars.tags ?? [],
    summary: vars.summary ?? '',
    source: CONSOLE_SOURCE
  });
}

export interface UpdateMemoryVars {
  id: string;
  content?: string;
  tags?: string[];
  summary?: string;
  shared?: boolean;
}

// Builds update_mask from EXACTLY the fields present on vars (dirty-mask
// rule, round-2 MED): the caller (form layer, Plan 05) diffs against the
// original record and supplies only changed fields. Re-sending an unchanged
// `content` forces an unnecessary re-embed (tools.go:1003) and re-sending an
// unchanged auto-summary flips its provenance to client-authored
// (store.go:1404) -- this builder never guesses, it trusts the caller's
// changed-field set.
export function buildUpdateMemoryRequest(vars: UpdateMemoryVars): UpdateMemoryRequest {
  const paths: string[] = [];
  if (vars.content !== undefined) paths.push('content');
  if (vars.tags !== undefined) paths.push('tags');
  if (vars.summary !== undefined) paths.push('summary');
  if (vars.shared !== undefined) paths.push('shared');
  return create(UpdateMemoryRequestSchema, {
    id: vars.id,
    content: vars.content ?? '',
    tags: vars.tags ?? [],
    summary: vars.summary ?? '',
    shared: vars.shared ?? false,
    updateMask: create(FieldMaskSchema, { paths })
  });
}

export interface ScheduleMemoryVars extends CreateMemoryVars {
  notBefore?: Date;
  notAfter?: Date;
}

export function buildScheduleMemoryRequest(vars: ScheduleMemoryVars): ScheduleMemoryRequest {
  return create(ScheduleMemoryRequestSchema, {
    content: vars.content,
    scope: vars.scope,
    category: vars.category,
    tags: vars.tags ?? [],
    summary: vars.summary ?? '',
    source: CONSOLE_SOURCE,
    notBefore: vars.notBefore ? timestampFromDate(vars.notBefore) : undefined,
    notAfter: vars.notAfter ? timestampFromDate(vars.notAfter) : undefined
  });
}

// A stored visibility of '' is legacy/unset and reads as private everywhere
// in the console (Codex+grok MEDIUM).
export function normalizeVisibility(v: string): 'private' | 'shared' {
  return v === 'shared' ? 'shared' : 'private';
}

// ---------------------------------------------------------------------------
// Create/schedule-as-shared composite state machine (Codex round-3 HIGH)
//
// StoreMemoryRequest/ScheduleMemoryRequest carry NO visibility field and no
// idempotency key/id (engram.proto:104,203), so "create as shared" is a
// two-call composite: Store*/Schedule, then (only if shared) SetVisibility.
// A secondary SetVisibility failure -- INCLUDING Unauthenticated/
// PermissionDenied -- is CAUGHT here and returned as `created_private`,
// NEVER rethrown: rethrowing would reach the form's whole-create D-09
// resubmit path and re-issue Store/Schedule, DUPLICATING the record (T-19-45).
// Re-sharing a `created_private` record is the ordinary by-id
// useSetMemoryVisibility action, never a whole-create replay.
// ---------------------------------------------------------------------------

async function shareIfRequested(
  client: Pick<EngramWriteClient, 'setVisibility'>,
  id: string,
  shared: boolean | undefined
): Promise<WriteResult> {
  if (!shared) return { status: 'created', id };
  try {
    await client.setVisibility(create(SetVisibilityRequestSchema, { id, visibility: Visibility.SHARED }));
    return { status: 'created_shared', id };
  } catch {
    return { status: 'created_private', id };
  }
}

export async function createMemoryComposite(client: EngramWriteClient, vars: CreateMemoryVars): Promise<WriteResult> {
  const resp = await client.storeMemory(buildStoreMemoryRequest(vars));
  return shareIfRequested(client, resp.id, vars.shared);
}

export async function scheduleMemoryComposite(
  client: EngramWriteClient,
  vars: ScheduleMemoryVars
): Promise<WriteResult> {
  const resp = await client.scheduleMemory(buildScheduleMemoryRequest(vars));
  return shareIfRequested(client, resp.id, vars.shared);
}

// ---------------------------------------------------------------------------
// Pure cache-transform factories (node-testable against a real QueryClient,
// without a Svelte QueryClientProvider -- createMutation/useQueryClient
// throw outside Svelte component context, Codex+grok MEDIUM).
// ---------------------------------------------------------------------------

export type CacheEntry = [readonly unknown[], unknown];

const MEMORY_LIST_PREFIXES = [['listMemories'], ['searchMemories']] as const;

export function snapshotMemoryQueries(queryClient: QueryClient, id?: string): CacheEntry[] {
  const entries: CacheEntry[] = [];
  for (const prefix of MEMORY_LIST_PREFIXES) {
    entries.push(...(queryClient.getQueriesData({ queryKey: prefix }) as CacheEntry[]));
  }
  if (id) entries.push(...(queryClient.getQueriesData({ queryKey: ['getMemory', id] }) as CacheEntry[]));
  return entries;
}

export function restoreMemoryQueries(queryClient: QueryClient, snapshot: CacheEntry[]): void {
  for (const [key, data] of snapshot) queryClient.setQueryData(key as readonly unknown[], data);
}

type ListLikeResponse = { memories?: Memory[]; total?: bigint };

function mapMemoriesField<T extends ListLikeResponse | undefined>(old: T, fn: (m: Memory) => Memory | null): T {
  if (!old || !Array.isArray(old.memories)) return old;
  const kept: Memory[] = [];
  let removed = 0;
  for (const m of old.memories) {
    const next = fn(m);
    if (next) kept.push(next);
    else removed += 1;
  }
  const patched = { ...old, memories: kept };
  if (removed > 0 && typeof old.total === 'bigint') {
    (patched as ListLikeResponse).total = old.total - BigInt(removed);
  }
  return patched;
}

interface MemoryCacheCtx {
  isListPage: boolean;
  visibilityFilter?: string;
}

// Iterates every cached listMemories/searchMemories page plus the getMemory
// cache via getQueriesData/setQueryData per-key (setQueriesData's updater
// signature does not receive the key, and a listMemoriesKey visibility
// filter must be inspected for the set-visibility filtered-membership rule
// below), applying `fn` to the matching record. `fn` returning null removes
// the record from that cache entry.
function applyToMemoryCaches(
  queryClient: QueryClient,
  id: string,
  fn: (m: Memory, ctx: MemoryCacheCtx) => Memory | null
): void {
  for (const [key, data] of queryClient.getQueriesData({ queryKey: ['listMemories'] })) {
    const visibilityFilter = typeof key[3] === 'string' ? (key[3] as string) : undefined;
    queryClient.setQueryData(
      key,
      mapMemoriesField(data as ListLikeResponse | undefined, (m) =>
        m.id === id ? fn(m, { isListPage: true, visibilityFilter }) : m
      )
    );
  }
  for (const [key, data] of queryClient.getQueriesData({ queryKey: ['searchMemories'] })) {
    queryClient.setQueryData(
      key,
      mapMemoriesField(data as ListLikeResponse | undefined, (m) =>
        m.id === id ? fn(m, { isListPage: false }) : m
      )
    );
  }
  queryClient.setQueryData(['getMemory', id], (old: { memory?: Memory } | undefined) => {
    if (!old?.memory || old.memory.id !== id) return old;
    const next = fn(old.memory, { isListPage: false });
    return next ? { ...old, memory: next } : old;
  });
}

export function applyUpdateOptimistic(queryClient: QueryClient, vars: UpdateMemoryVars): void {
  applyToMemoryCaches(queryClient, vars.id, (m, ctx) => {
    const nextVisibility =
      vars.shared !== undefined ? (vars.shared ? 'shared' : 'private') : m.visibility;
    // Drop from a filtered list page whose visibility filter no longer matches
    // the record's new visibility (private→shared edit), mirroring
    // applySetVisibilityOptimistic's filtered-cache-membership rule (WR-01).
    if (ctx.isListPage && ctx.visibilityFilter && ctx.visibilityFilter !== nextVisibility) return null;
    return {
      ...m,
      ...(vars.content !== undefined ? { content: vars.content } : {}),
      ...(vars.tags !== undefined ? { tags: vars.tags } : {}),
      ...(vars.summary !== undefined ? { summary: vars.summary } : {}),
      ...(vars.shared !== undefined ? { visibility: nextVisibility } : {})
    };
  });
}

export function applyDeleteOptimistic(queryClient: QueryClient, id: string): void {
  applyToMemoryCaches(queryClient, id, () => null);
}

// listMemoriesKey(scope, categories, visibility, limit, offset) -- key[3] is
// the page's visibility filter (''/'private'/'shared'). A set-visibility
// change must DROP the record from a page whose filter no longer matches the
// new value (round-2 MED filtered-cache-membership), not just patch the
// field in place.
export function applySetVisibilityOptimistic(
  queryClient: QueryClient,
  id: string,
  visibility: 'private' | 'shared'
): void {
  applyToMemoryCaches(queryClient, id, (m, ctx) => {
    if (ctx.isListPage && ctx.visibilityFilter && ctx.visibilityFilter !== visibility) return null;
    return { ...m, visibility };
  });
}

// ---------------------------------------------------------------------------
// Mutation hooks
// ---------------------------------------------------------------------------

export function useCreateMemory() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (vars: CreateMemoryVars) => createMemoryComposite(engramWrite, vars),
    // Sole toast site (Codex round-4 LOW): mutationFn/the composite never
    // toasts, so a partial success can never fire both the warning and the
    // normal `created` toast.
    onSuccess: (result: WriteResult) => {
      if (result.status === 'created_private') toast.warning('created (private) — sharing failed');
      else toast.success('created');
    },
    onError: () => {
      toast.error('write failed');
    },
    onSettled: () => {
      // Create is invalidate-only (StoreMemoryResponse is {id, short_id} --
      // no temp-id for an optimistic list insert).
      queryClient.invalidateQueries({ queryKey: ['listMemories'] });
      queryClient.invalidateQueries({ queryKey: ['searchMemories'] });
      queryClient.invalidateQueries({ queryKey: ['listScopes'] });
    }
  }));
}

export function useUpdateMemory() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (vars: UpdateMemoryVars) => engramWrite.updateMemory(buildUpdateMemoryRequest(vars)),
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey: ['getMemory', vars.id] });
      await queryClient.cancelQueries({ queryKey: ['listMemories'] });
      await queryClient.cancelQueries({ queryKey: ['searchMemories'] });
      const snapshot = snapshotMemoryQueries(queryClient, vars.id);
      applyUpdateOptimistic(queryClient, vars);
      return { snapshot };
    },
    onError: (_err, _vars, context) => {
      // Rollback, but do NOT swallow the error -- the caller's own onError
      // (passed to .mutate()) still fires so the form/row layer can drive
      // D-09/SC3 inline re-auth on a terminal auth failure.
      if (context?.snapshot) restoreMemoryQueries(queryClient, context.snapshot);
      toast.error('write failed');
    },
    onSuccess: () => {
      toast.success('saved');
    },
    onSettled: (_data, _err, vars) => {
      queryClient.invalidateQueries({ queryKey: ['getMemory', vars.id] });
      queryClient.invalidateQueries({ queryKey: ['listMemories'] });
      queryClient.invalidateQueries({ queryKey: ['searchMemories'] });
    }
  }));
}

export function useDeleteMemory() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (vars: { id: string }) => engramWrite.deleteMemory(create(DeleteMemoryRequestSchema, { id: vars.id })),
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey: ['getMemory', vars.id] });
      await queryClient.cancelQueries({ queryKey: ['listMemories'] });
      await queryClient.cancelQueries({ queryKey: ['searchMemories'] });
      const snapshot = snapshotMemoryQueries(queryClient, vars.id);
      applyDeleteOptimistic(queryClient, vars.id);
      return { snapshot };
    },
    onError: (_err, _vars, context) => {
      if (context?.snapshot) restoreMemoryQueries(queryClient, context.snapshot);
      toast.error('write failed');
    },
    onSuccess: () => {
      toast.success('deleted');
    },
    onSettled: (_data, _err, vars) => {
      queryClient.invalidateQueries({ queryKey: ['getMemory', vars.id] });
      queryClient.invalidateQueries({ queryKey: ['listMemories'] });
      queryClient.invalidateQueries({ queryKey: ['searchMemories'] });
      queryClient.invalidateQueries({ queryKey: ['listScopes'] });
    }
  }));
}

export interface SetMemoryVisibilityVars {
  id: string;
  visibility: 'private' | 'shared';
}

export function useSetMemoryVisibility() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (vars: SetMemoryVisibilityVars) =>
      engramWrite.setVisibility(
        create(SetVisibilityRequestSchema, {
          id: vars.id,
          visibility: vars.visibility === 'shared' ? Visibility.SHARED : Visibility.PRIVATE
        })
      ),
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey: ['getMemory', vars.id] });
      await queryClient.cancelQueries({ queryKey: ['listMemories'] });
      await queryClient.cancelQueries({ queryKey: ['searchMemories'] });
      const snapshot = snapshotMemoryQueries(queryClient, vars.id);
      applySetVisibilityOptimistic(queryClient, vars.id, vars.visibility);
      return { snapshot };
    },
    onError: (_err, _vars, context) => {
      if (context?.snapshot) restoreMemoryQueries(queryClient, context.snapshot);
      toast.error('write failed');
    },
    onSuccess: () => {
      toast.success('saved');
    },
    onSettled: (_data, _err, vars) => {
      queryClient.invalidateQueries({ queryKey: ['getMemory', vars.id] });
      queryClient.invalidateQueries({ queryKey: ['listMemories'] });
      queryClient.invalidateQueries({ queryKey: ['searchMemories'] });
    }
  }));
}

export function useScheduleMemory() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (vars: ScheduleMemoryVars) => scheduleMemoryComposite(engramWrite, vars),
    onSuccess: (result: WriteResult) => {
      if (result.status === 'created_private') toast.warning('created (private) — sharing failed');
      else toast.success('created');
    },
    onError: () => {
      toast.error('write failed');
    },
    onSettled: () => {
      // Schedule is create-only (ScheduleMemoryRequest has no id field) --
      // invalidate-only, same as create.
      queryClient.invalidateQueries({ queryKey: ['listMemories'] });
      queryClient.invalidateQueries({ queryKey: ['searchMemories'] });
      queryClient.invalidateQueries({ queryKey: ['listScopes'] });
    }
  }));
}
