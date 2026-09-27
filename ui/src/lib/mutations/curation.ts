import { createMutation, useQueryClient, type QueryClient } from '@tanstack/svelte-query';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { engramWrite } from '$lib/client';
import { applyToMemoryCaches, CONSOLE_SOURCE } from './memory';
import {
  ArchiveMemoryRequestSchema,
  RestoreMemoryRequestSchema,
  ArchiveOutcome,
  SupersedeMemoryRequestSchema,
  type ArchiveResult,
  type SupersedeMemoryRequest,
  type SupersedeMemoryResponse
} from '$lib/gen/engram_pb';
import type { ParsedConnectError } from '$lib/errors/connect-error';

// ArchiveSubmitOutcome is what CurationSurfaces' onsubmit resolves to (never
// throws) -- ArchiveConfirmDialog switches its body on `kind` alone.
export type ArchiveSubmitOutcome =
  | { kind: 'ok'; results: ArchiveResult[] }
  | { kind: 'reauth' }
  | { kind: 'rejected'; parsed: ParsedConnectError };

// changedIds returns the ARCHIVED+RESTORED ids in result order -- the set
// that actually changed state (D-10's flash set, D-09's undo/toast targets).
export function changedIds(results: ArchiveResult[]): string[] {
  return results
    .filter((r) => r.outcome === ArchiveOutcome.ARCHIVED || r.outcome === ArchiveOutcome.RESTORED)
    .map((r) => r.id);
}

// applyArchiveResultsOptimistic patches archivedAt in place on every affected
// cache entry (listMemories/searchMemories/getMemory), per result outcome.
// The transform never returns null -- D-10: archived/restored rows stay in
// their list, they are never removed the way delete removes them.
export function applyArchiveResultsOptimistic(
  queryClient: QueryClient,
  results: ArchiveResult[],
  now: Date
): void {
  for (const r of results) {
    if (r.outcome === ArchiveOutcome.ARCHIVED) {
      applyToMemoryCaches(queryClient, r.id, (m) => ({ ...m, archivedAt: timestampFromDate(now) }));
    } else if (r.outcome === ArchiveOutcome.RESTORED) {
      applyToMemoryCaches(queryClient, r.id, (m) => ({ ...m, archivedAt: undefined }));
    }
    // ALREADY_ARCHIVED / NOT_ARCHIVED / NOT_FOUND touch nothing.
  }
}

// invalidateAfterCuration invalidates every recall surface an archive/restore
// can affect. The recall list queries use refetchType: 'none' (D-10) so the
// visible list does not jump -- applyArchiveResultsOptimistic already patched
// the rows in place; a background refetch on next mount/focus keeps the
// cache honest without disturbing what's on screen right now. getMemory/
// relatedMemories/listTags refetch normally -- those are not the "don't jump
// the list" surface.
export function invalidateAfterCuration(queryClient: QueryClient): void {
  queryClient.invalidateQueries({ queryKey: ['searchMemories'], refetchType: 'none' });
  queryClient.invalidateQueries({ queryKey: ['listMemories'], refetchType: 'none' });
  queryClient.invalidateQueries({ queryKey: ['listScheduled'], refetchType: 'none' });
  queryClient.invalidateQueries({ queryKey: ['listRules'], refetchType: 'none' });
  queryClient.invalidateQueries({ queryKey: ['getMemory'] });
  queryClient.invalidateQueries({ queryKey: ['relatedMemories'] });
  queryClient.invalidateQueries({ queryKey: ['listTags'] });
}

export interface ArchiveRestoreVars {
  ids: string[];
}

export function useArchiveMemory() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (vars: ArchiveRestoreVars) =>
      engramWrite.archiveMemory(create(ArchiveMemoryRequestSchema, { ids: vars.ids })),
    onSuccess: (resp) => {
      applyArchiveResultsOptimistic(queryClient, resp.results, new Date());
      invalidateAfterCuration(queryClient);
    }
    // No toast here -- the host (CurationSurfaces) owns feedback (result body
    // + undo toast), same division of labor as WriteSurfaces/mutation hooks.
  }));
}

export function useRestoreMemory() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (vars: ArchiveRestoreVars) =>
      engramWrite.restoreMemory(create(RestoreMemoryRequestSchema, { ids: vars.ids })),
    onSuccess: (resp) => {
      applyArchiveResultsOptimistic(queryClient, resp.results, new Date());
      invalidateAfterCuration(queryClient);
    }
  }));
}

// ---------------------------------------------------------------------------
// Supersede (CUR-01, D-07, D-10)
// ---------------------------------------------------------------------------

// The correcting record's field set -- exactly the fields SupersedeDialog's
// right column edits. Field NAMES match SupersedeMemoryRequestSchema's own
// (summary/content/category/scope/tags), so buildSupersedeRequest can spread
// this object directly into the request literal.
export interface SupersedeFields {
  summary: string;
  content: string;
  category: string;
  scope: string;
  tags: string[];
}

// A supersede draft: the target set (ids, caller order), the correcting
// record's fields, and the ONE idempotency key minted on open (crypto.
// randomUUID) and reused across every preview/commit/resend of this draft.
export interface SupersedeDraft {
  targets: string[];
  fields: SupersedeFields;
  idempotencyKey: string;
}

// buildSupersedeRequest(draft, {validateOnly}) -- validateOnly NEVER carries
// the idempotency_key (the preview never consults or records it, per the
// proto's own doc comment); only a real commit (validateOnly: false) carries
// it, so a resend after re-auth replays the same fingerprint.
export function buildSupersedeRequest(
  draft: SupersedeDraft,
  opts: { validateOnly: boolean }
): SupersedeMemoryRequest {
  return create(SupersedeMemoryRequestSchema, {
    ...draft.fields,
    source: CONSOLE_SOURCE,
    supersedes: draft.targets,
    idempotencyKey: opts.validateOnly ? '' : draft.idempotencyKey,
    validateOnly: opts.validateOnly
  });
}

export function previewSupersede(draft: SupersedeDraft, signal?: AbortSignal): Promise<SupersedeMemoryResponse> {
  return engramWrite.supersedeMemory(buildSupersedeRequest(draft, { validateOnly: true }), signal ? { signal } : undefined);
}

// useSupersedeMemory mirrors useArchiveMemory's onSuccess-does-the-cache-patch
// shape: draft.targets are always FULL ids by construction (every target a
// chip carries is a resolved Memory -- selection, add-by-short_id via
// onlookup, or a "use head" swap all resolve to a real record before joining
// `targets`), so the mutation's own draft argument is already the canonical
// predecessor set -- no separate canonicalization step is needed. The host
// (CurationSurfaces) still owns flash/onchanged, which need component-level
// props this hook has no access to.
export function useSupersedeMemory() {
  const queryClient = useQueryClient();
  return createMutation(() => ({
    mutationFn: (draft: SupersedeDraft) => engramWrite.supersedeMemory(buildSupersedeRequest(draft, { validateOnly: false })),
    onSuccess: (resp, draft) => {
      applySupersedeOptimistic(queryClient, draft.targets, resp.id);
      invalidateAfterCuration(queryClient);
    }
  }));
}

// applySupersedeOptimistic patches supersededBy onto every predecessor in
// place (never removes a row -- D-10, same discipline as archive/restore).
export function applySupersedeOptimistic(queryClient: QueryClient, predecessorIds: string[], newId: string): void {
  for (const id of predecessorIds) {
    applyToMemoryCaches(queryClient, id, (m) => ({ ...m, supersededBy: newId }));
  }
}
