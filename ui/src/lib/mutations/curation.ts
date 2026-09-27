import { createMutation, useQueryClient, type QueryClient } from '@tanstack/svelte-query';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { engramWrite } from '$lib/client';
import { applyToMemoryCaches } from './memory';
import {
  ArchiveMemoryRequestSchema,
  RestoreMemoryRequestSchema,
  ArchiveOutcome,
  type ArchiveResult
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
