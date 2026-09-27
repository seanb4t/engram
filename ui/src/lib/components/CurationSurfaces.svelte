<script lang="ts">
  import { useQueryClient } from '@tanstack/svelte-query';
  import { toast } from 'svelte-sonner';
  import { ConnectError, Code } from '@connectrpc/connect';
  import type { Memory } from '$lib/gen/engram_pb';
  import { engram } from '$lib/client';
  import {
    useArchiveMemory,
    useRestoreMemory,
    changedIds,
    type ArchiveSubmitOutcome
  } from '$lib/mutations/curation';
  import { parseConnectError } from '$lib/errors/connect-error';
  import { redirectToLogin } from '$lib/resume';
  import { flashRows } from '$lib/curation/flash.svelte.ts';
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
  let archivePending = $state(false);
  let archiveOutcome = $state<ArchiveSubmitOutcome | undefined>(undefined);

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
    archiveOutcome = undefined;
    archiveRecords = await resolveRecords(ids);
    archiveOpen = true;
  }

  export async function openRestore(ids: string[]): Promise<void> {
    archiveMode = 'restore';
    archiveOutcome = undefined;
    archiveRecords = await resolveRecords(ids);
    archiveOpen = true;
  }

  // Maps a mutation failure to ArchiveSubmitOutcome (D-08/D-09): an
  // Unauthenticated/PermissionDenied failure is a re-auth gate, everything
  // else routes through the shared connect-error classifier.
  function mapMutationError(err: unknown): ArchiveSubmitOutcome {
    if (err instanceof ConnectError && (err.code === Code.Unauthenticated || err.code === Code.PermissionDenied)) {
      return { kind: 'reauth' };
    }
    return { kind: 'rejected', parsed: parseConnectError(err) };
  }

  // The dialog's own confirm-click path: runs the CURRENT mode's mutation.
  // On success: patch the caches (the mutation hook's onSuccess already does
  // this), flash the changed rows and notify the route -- immediately, not
  // deferred to Done (D-10, and the D-09 result body needs the flash to
  // already be live when it renders).
  async function onsubmit(ids: string[]): Promise<ArchiveSubmitOutcome> {
    const mutation = archiveMode === 'archive' ? archiveMutation : restoreMutation;
    try {
      const resp = await mutation.mutateAsync({ ids });
      const changed = changedIds(resp.results);
      flashRows(changed);
      onchanged?.({ kind: archiveMode, ids: changed });
      return { kind: 'ok', results: resp.results };
    } catch (err) {
      return mapMutationError(err);
    }
  }

  function oncancel(): void {
    archiveOpen = false;
  }

  // D-09 surface 1 (result-body undo): "Undo — restore N" on a just-shown
  // archive result runs the INVERSE call through the SAME dialog -- no
  // second confirm. Flips the bound `mode`/`pending`/`outcome` directly
  // (ArchiveConfirmDialog re-renders its result view from the new outcome).
  async function onundo(ids: string[]): Promise<void> {
    const inverseMode = archiveMode === 'archive' ? 'restore' : 'archive';
    const inverseMutation = inverseMode === 'archive' ? archiveMutation : restoreMutation;
    archiveMode = inverseMode;
    archivePending = true;
    try {
      const resp = await inverseMutation.mutateAsync({ ids });
      const changed = changedIds(resp.results);
      flashRows(changed);
      onchanged?.({ kind: inverseMode, ids: changed });
      archiveOutcome = { kind: 'ok', results: resp.results };
    } catch (err) {
      archiveOutcome = mapMutationError(err);
    } finally {
      archivePending = false;
    }
  }

  // D-09 surface 2 (toast undo): closing an archive result with >=1 archived
  // id fires an 8s toast whose Undo action restores exactly those ids.
  // Restore results never get a toast (Flagged assumptions: no double undo).
  function ondone(ids: string[]): void {
    const wasArchive = archiveMode === 'archive';
    archiveOpen = false;
    if (wasArchive && ids.length > 0) {
      toast(`${ids.length} archived · Undo`, {
        duration: 8000,
        action: {
          label: 'Undo',
          onClick: () => {
            restoreMutation.mutate({ ids });
          }
        }
      });
    }
  }

  // plan 04-06 persists the v2 resume envelope before this redirect; this
  // plan wires the redirect only.
  function onreauth(_ids: string[]): void {
    redirectToLogin();
  }
</script>

<ArchiveConfirmDialog
  bind:open={archiveOpen}
  bind:mode={archiveMode}
  bind:pending={archivePending}
  bind:outcome={archiveOutcome}
  records={archiveRecords}
  {onsubmit}
  {oncancel}
  {ondone}
  {onundo}
  {onreauth}
/>
