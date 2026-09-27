<script lang="ts">
  import { useQueryClient } from '@tanstack/svelte-query';
  import type { Memory } from '$lib/gen/engram_pb';
  import { engram } from '$lib/client';
  import { useArchiveMemory, useRestoreMemory, type ArchiveSubmitOutcome } from '$lib/mutations/curation';
  import ArchiveConfirmDialog from './ArchiveConfirmDialog.svelte';

  // CurationSurfaces is the route-level curation write host (mirrors
  // WriteSurfaces): it owns the archive/restore confirm dialog's open state,
  // resolves ids to Memory records for the chip display, runs the mutation,
  // and reports the changed ids back up to the route via `onchanged`.
  let {
    // returnPath is threaded through for the re-auth resume envelope (wired
    // in plan 04-06); this plan accepts and stores it but does not yet use
    // it, so it is intentionally not destructured into a local binding.
    onchanged
  }: {
    returnPath: string;
    onchanged?: (e: { kind: 'archive' | 'restore' | 'supersede'; ids: string[]; newId?: string }) => void;
  } = $props();

  const queryClient = useQueryClient();
  const archiveMutation = useArchiveMemory();
  const restoreMutation = useRestoreMemory();

  let archiveOpen = $state(false);
  let archiveMode = $state<'archive' | 'restore'>('archive');
  let archiveRecords = $state<Memory[]>([]);

  // Resolves each id to a Memory from the query cache (searchMemories/
  // listMemories pages, then getMemory), falling back to a direct fetch for
  // an id not present in any loaded page.
  async function resolveRecords(ids: string[]): Promise<Memory[]> {
    const found = new Map<string, Memory>();
    for (const prefix of [['searchMemories'], ['listMemories']] as const) {
      for (const [, data] of queryClient.getQueriesData({ queryKey: prefix })) {
        const memories = (data as { memories?: Memory[] } | undefined)?.memories ?? [];
        for (const m of memories) {
          if (ids.includes(m.id) && !found.has(m.id)) found.set(m.id, m);
        }
      }
    }
    for (const id of ids) {
      if (found.has(id)) continue;
      const cached = queryClient.getQueryData<{ memory?: Memory }>(['getMemory', id]);
      if (cached?.memory) {
        found.set(id, cached.memory);
        continue;
      }
    }
    const missing = ids.filter((id) => !found.has(id));
    for (const id of missing) {
      try {
        const resp = await queryClient.fetchQuery({
          queryKey: ['getMemory', id],
          queryFn: () => engram.getMemory({ id })
        });
        if (resp.memory) found.set(id, resp.memory);
      } catch {
        // Unresolvable id (deleted between selection and open) -- the dialog
        // simply shows fewer chips than ids; the submit call still carries
        // the full id set and the server reports NOT_FOUND for it.
      }
    }
    return ids.map((id) => found.get(id)).filter((m): m is Memory => !!m);
  }

  export async function openArchive(ids: string[]): Promise<void> {
    archiveMode = 'archive';
    archiveRecords = await resolveRecords(ids);
    archiveOpen = true;
  }

  export async function openRestore(ids: string[]): Promise<void> {
    archiveMode = 'restore';
    archiveRecords = await resolveRecords(ids);
    archiveOpen = true;
  }

  // Task 3 (D-08/D-09) maps a mutation error to { kind: 'reauth' | 'rejected' }
  // for the dialog's status block; this tracer slice runs only the success
  // path -- error mapping is deliberately out of scope until Task 3.
  async function onsubmit(ids: string[]): Promise<ArchiveSubmitOutcome> {
    const mutation = archiveMode === 'archive' ? archiveMutation : restoreMutation;
    const resp = await mutation.mutateAsync({ ids });
    return { kind: 'ok', results: resp.results };
  }

  function oncancel(): void {
    archiveOpen = false;
  }

  function ondone(changed: string[]): void {
    archiveOpen = false;
    onchanged?.({ kind: archiveMode, ids: changed });
  }
</script>

<ArchiveConfirmDialog bind:open={archiveOpen} mode={archiveMode} records={archiveRecords} {onsubmit} {oncancel} {ondone} />
