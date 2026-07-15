<script lang="ts">
  import { useQueryClient } from '@tanstack/svelte-query';
  import { ConnectError, Code } from '@connectrpc/connect';
  import { toast } from 'svelte-sonner';
  import type { Memory } from '$lib/gen/engram_pb';
  import { engram } from '$lib/client';
  import { describeError } from '$lib/errors';
  import { redirectToLogin, type ResumeEnvelope } from '$lib/resume';
  import { useDeleteMemory, useSetMemoryVisibility, normalizeVisibility } from '$lib/mutations/memory';
  import { useDeleteDiscovery, useSetDiscoveryVisibility } from '$lib/mutations/discovery';
  import { Button } from '$lib/components/ui/button';
  import MemoryFormSheet from './MemoryFormSheet.svelte';
  import DiscoveryFormSheet from './DiscoveryFormSheet.svelte';
  import DeleteConfirmDialog from './DeleteConfirmDialog.svelte';
  import ShareWarningInline from './ShareWarningInline.svelte';

  // WriteSurfaces is the route-level write host (Plan 06): it owns which
  // sheet is open, the edit/delete/share targets, and exposes an EXACT
  // bind:this exported-method contract the routes call: openCreate() /
  // openEdit(id) / requestDelete(id, kind) / requestShare(memory, kind) /
  // reopenFromResume(env). No "or equivalent" -- Task 2's route wiring
  // depends on these exact names (Codex+grok MEDIUM).
  let {
    kind,
    scope,
    onresumeapplied,
    ondeleted
  }: {
    kind: 'memory' | 'discovery';
    scope: string;
    onresumeapplied?: () => void;
    ondeleted?: (id: string) => void;
  } = $props();

  const queryClient = useQueryClient();

  // Create/edit sheet state -- ONE MemoryFormSheet instance covers both
  // create and edit (mode + editMemory). The sheet is remounted (via the
  // {#key} block below, "keyed by mode + recordId") on every open so its
  // internal $state initializers (which capture props once at mount, Plan
  // 05) always start from a clean slate for the new target -- this is what
  // lets it reconstruct edit mode correctly even though openEdit's
  // GetMemory fetch is asynchronous.
  let sheetMode = $state<'create' | 'edit'>('create');
  let sheetOpen = $state(false);
  let editMemory = $state<Memory | undefined>(undefined);
  let createScope = $state(scope);
  let sheetInstanceKey = $state(0);
  const sheetKey = $derived(`${sheetMode}-${editMemory?.id ?? 'none'}-${sheetInstanceKey}`);

  // Resume-envelope restore props, threaded into whichever sheet is open
  // (Codex round-3 HIGH: WriteSurfaces never peeks/deletes the envelope
  // itself -- it only forwards resumeValues/dirtyPaths and relays the
  // form's onresumeapplied acknowledgement up to the route via the exact
  // `onresumeapplied` prop, which the route wires to consumeResume()).
  let formResumeValues = $state<Record<string, unknown> | undefined>(undefined);
  let formResumeDirtyPaths = $state<string[] | undefined>(undefined);

  // Inline delete/share targets -- host-authoritative closure (Codex
  // round-5 HIGH, symmetric across both surfaces): DeleteConfirmDialog never
  // self-closes on confirm, and ShareWarningInline is host-rendered only
  // while shareTarget is set. Both close ONLY when this host clears the
  // target on success; a terminal Unauthenticated/PermissionDenied RETAINS
  // the target (dialog/banner stays visibly open) with the re-auth CTA
  // (SC3 for the inline actions, T-19-68).
  let deleteTarget = $state<{ id: string; kind: 'memory' | 'discovery' } | undefined>(undefined);
  let deleteDialogOpen = $state(false);
  let deleteAuthFailure = $state(false);

  let shareTarget = $state<{ id: string; kind: 'memory' | 'discovery' } | undefined>(undefined);
  let shareAuthFailure = $state(false);

  const deleteMemoryMutation = useDeleteMemory();
  const deleteDiscoveryMutation = useDeleteDiscovery();
  const setMemoryVisibilityMutation = useSetMemoryVisibility();
  const setDiscoveryVisibilityMutation = useSetDiscoveryVisibility();

  function handleFormResumeApplied() {
    formResumeValues = undefined;
    formResumeDirtyPaths = undefined;
    onresumeapplied?.();
  }

  export function openCreate(): void {
    sheetMode = 'create';
    editMemory = undefined;
    createScope = scope;
    formResumeValues = undefined;
    formResumeDirtyPaths = undefined;
    sheetInstanceKey += 1;
    sheetOpen = true;
  }

  // Fetches the FULL record via GetMemory BEFORE opening the sheet (Codex
  // round-2 HIGH, T-19-66): list/search rows are summary-shaped (server
  // clears content when full=false, connectapi.go:70) -- prefilling from
  // one would let a Save overwrite the real body with empty content. No-op
  // for kind=discovery (D-04 -- no discovery edit surface exists to open).
  export async function openEdit(id: string): Promise<void> {
    if (kind !== 'memory') return;
    try {
      const resp = await queryClient.fetchQuery({
        queryKey: ['getMemory', id],
        queryFn: () => engram.getMemory({ id })
      });
      editMemory = resp.memory;
      sheetMode = 'edit';
      formResumeValues = undefined;
      formResumeDirtyPaths = undefined;
      sheetInstanceKey += 1;
      sheetOpen = true;
    } catch (err) {
      toast.error(`couldn't load — ${describeError(err)}`);
    }
  }

  export function requestDelete(id: string, targetKind: 'memory' | 'discovery'): void {
    deleteAuthFailure = false;
    deleteTarget = { id, kind: targetKind };
    deleteDialogOpen = true;
  }

  // Visibility-aware (Codex round-2 HIGH): reads visibility from the passed
  // Memory (not an id-only lookup) so an already-shared record is a genuine
  // no-op -- row/detail already hide the Share item when shared (Plan 03);
  // this is the second no-op layer.
  export function requestShare(memory: Memory, targetKind: 'memory' | 'discovery'): void {
    if (normalizeVisibility(memory.visibility) === 'shared') return;
    shareAuthFailure = false;
    shareTarget = { id: memory.id, kind: targetKind };
  }

  // Re-auth resume consumption (Codex round-3 HIGH): reopens the correct
  // sheet and passes the restored values in as PROPS -- WriteSurfaces never
  // peeks/deletes the envelope itself, the route is the sole owner (Task
  // 2). Guarded on env.kind matching this host's kind as defense-in-depth
  // (the route already checks this before calling).
  export async function reopenFromResume(env: ResumeEnvelope): Promise<void> {
    if (env.kind !== kind) return;
    if (env.mode === 'edit' && env.recordId) {
      await openEdit(env.recordId);
      formResumeValues = env.values;
      formResumeDirtyPaths = env.dirtyPaths;
    } else {
      openCreate();
      formResumeValues = env.values;
      formResumeDirtyPaths = env.dirtyPaths;
    }
  }

  // Delete/share await settlement; closure is host-authoritative -- the
  // target (and thus the dialog/banner) is cleared ONLY on success. On a
  // terminal Unauthenticated/PermissionDenied, RETAIN the target so the
  // surface stays visibly open with the re-auth CTA (no auto-replay -- a
  // re-fire is operator-initiated). A non-auth error is already toasted by
  // the Plan-04 mutation's own onError; the target stays for a manual
  // retry or Cancel.
  async function confirmDelete(): Promise<void> {
    if (!deleteTarget) return;
    const { id, kind: targetKind } = deleteTarget;
    const mutation = targetKind === 'memory' ? deleteMemoryMutation : deleteDiscoveryMutation;
    await new Promise<void>((resolve) => {
      mutation.mutate(
        { id },
        {
          onSuccess: () => {
            deleteTarget = undefined;
            deleteAuthFailure = false;
            deleteDialogOpen = false;
            // Relay the deleted id up so the route can clear its selection
            // (WR-02): otherwise the still-enabled detail query refetches the
            // tombstone and NotFound raises a spurious global error banner.
            ondeleted?.(id);
            resolve();
          },
          onError: (err: unknown) => {
            const ce = err instanceof ConnectError ? err : ConnectError.from(err);
            if (ce.code === Code.Unauthenticated || ce.code === Code.PermissionDenied) {
              deleteAuthFailure = true;
            }
            resolve();
          }
        }
      );
    });
  }

  function cancelDelete(): void {
    deleteTarget = undefined;
    deleteAuthFailure = false;
    deleteDialogOpen = false;
  }

  function handleDeleteReauth(): void {
    redirectToLogin();
  }

  async function confirmShare(): Promise<void> {
    if (!shareTarget) return;
    const { id, kind: targetKind } = shareTarget;
    const mutation = targetKind === 'memory' ? setMemoryVisibilityMutation : setDiscoveryVisibilityMutation;
    await new Promise<void>((resolve) => {
      mutation.mutate(
        { id, visibility: 'shared' },
        {
          onSuccess: () => {
            shareTarget = undefined;
            shareAuthFailure = false;
            resolve();
          },
          onError: (err: unknown) => {
            const ce = err instanceof ConnectError ? err : ConnectError.from(err);
            if (ce.code === Code.Unauthenticated || ce.code === Code.PermissionDenied) {
              shareAuthFailure = true;
            }
            resolve();
          }
        }
      );
    });
  }

  function cancelShare(): void {
    shareTarget = undefined;
    shareAuthFailure = false;
  }

  function handleShareReauth(): void {
    redirectToLogin();
  }
</script>

{#if kind === 'memory'}
  <Button variant="outline" size="sm" onclick={openCreate}>New memory</Button>
  {#key sheetKey}
    <MemoryFormSheet
      bind:open={sheetOpen}
      mode={sheetMode}
      memory={editMemory}
      scope={createScope}
      resumeValues={formResumeValues}
      resumeDirtyPaths={formResumeDirtyPaths}
      onresumeapplied={handleFormResumeApplied}
    />
  {/key}
{:else}
  <Button variant="outline" size="sm" onclick={openCreate}>New discovery</Button>
  {#key sheetInstanceKey}
    <DiscoveryFormSheet
      bind:open={sheetOpen}
      scope={createScope}
      resumeValues={formResumeValues}
      onresumeapplied={handleFormResumeApplied}
    />
  {/key}
{/if}

<DeleteConfirmDialog
  bind:open={deleteDialogOpen}
  kind={deleteTarget?.kind ?? 'memory'}
  onconfirm={confirmDelete}
  oncancel={cancelDelete}
  authFailure={deleteAuthFailure}
  onreauth={handleDeleteReauth}
/>

{#if shareTarget}
  <ShareWarningInline
    onconfirm={confirmShare}
    oncancel={cancelShare}
    authFailure={shareAuthFailure}
    onreauth={handleShareReauth}
  />
{/if}
