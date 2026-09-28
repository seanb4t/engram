<script lang="ts">
  import { useQueryClient } from '@tanstack/svelte-query';
  import { toast } from 'svelte-sonner';
  import { create } from '@bufbuild/protobuf';
  import { ConnectError, Code } from '@connectrpc/connect';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { MemorySchema, type Memory, type SupersedeMemoryResponse } from '$lib/gen/engram_pb';
  import { engram } from '$lib/client';
  import {
    useArchiveMemory,
    useRestoreMemory,
    useSupersedeMemory,
    previewSupersede,
    changedIds,
    type ArchiveSubmitOutcome,
    type SupersedeDraft,
    supersedeFieldsFrom,
    type SupersedeFields
  } from '$lib/mutations/curation';
  import { parseConnectError } from '$lib/errors/connect-error';
  import { persistResume, redirectToLogin, type CurationResumeEnvelope } from '$lib/resume';
  import { flashRows } from '$lib/curation/flash.svelte';
  import { headIdFrom } from '$lib/curation/chain';
  import ArchiveConfirmDialog from './ArchiveConfirmDialog.svelte';
  import SupersedeDialog from './SupersedeDialog.svelte';
  import ChainDialog from './ChainDialog.svelte';

  // CurationSurfaces is the route-level curation write host (mirrors
  // WriteSurfaces): it owns the archive/restore confirm dialog's open state,
  // resolves ids to Memory records for the chip display, runs the mutation,
  // and reports the changed ids back up to the route via `onchanged`.
  let {
    returnPath,
    onchanged,
    onresumeapplied,
    onviewsuperseded,
    onopenrecord,
    onbusychange
  }: {
    returnPath: string;
    onchanged?: (e: { kind: 'archive' | 'restore' | 'supersede'; ids: string[]; newId?: string }) => void;
    onresumeapplied?: () => void;
    // The route (plan 04-08) supplies these -- e.g. turning on the
    // include-superseded facet, or selecting the new record. CurationSurfaces
    // wraps them to close the supersede dialog first (D-07's own success
    // footer contract), then forwards to the route.
    onviewsuperseded?: (ids: string[]) => void;
    onopenrecord?: (id: string) => void;
    // Fired around every archive/restore/supersede COMMIT call (never the
    // supersede validate_only preview) -- lets the route disable the bulk
    // bar's verb buttons while a curation write is in flight (E4).
    onbusychange?: (busy: boolean) => void;
  } = $props();

  const queryClient = useQueryClient();
  const archiveMutation = useArchiveMemory();
  const restoreMutation = useRestoreMemory();
  const supersedeMutation = useSupersedeMemory();

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

  // resolveRecordsKeepAll is resolveRecords, but an id that cannot be
  // resolved becomes a placeholder Memory-shaped chip instead of being
  // silently dropped -- reopenFromResume's own contract ("an unreadable id
  // becomes a not-found chip, never dropped") needs every envelope target to
  // stay visibly represented, unlike the ordinary open path where a missing
  // chip is an acceptable degrade (the server still reports NOT_FOUND for
  // the full id set on submit).
  async function resolveRecordsKeepAll(ids: string[]): Promise<Memory[]> {
    const records = await resolveRecords(ids);
    const byId = new Map(records.map((m) => [m.id, m]));
    return ids.map(
      (id) =>
        byId.get(id) ??
        create(MemorySchema, {
          id,
          shortId: id,
          summary: `not found: ${id}`,
          content: '',
          category: 'convention',
          scope: '',
          visibility: 'private',
          owner: '',
          tags: []
        })
    );
  }

  export async function openArchive(ids: string[]): Promise<void> {
    archiveMode = 'archive';
    archiveOutcome = undefined;
    archiveNotice = undefined;
    archiveRecords = await resolveRecords(ids);
    archiveOpen = true;
  }

  export async function openRestore(ids: string[]): Promise<void> {
    archiveMode = 'restore';
    archiveOutcome = undefined;
    archiveNotice = undefined;
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
    onbusychange?.(true);
    try {
      const resp = await mutation.mutateAsync({ ids });
      const changed = changedIds(resp.results);
      flashRows(changed);
      onchanged?.({ kind: archiveMode, ids: changed });
      return { kind: 'ok', results: resp.results };
    } catch (err) {
      return mapMutationError(err);
    } finally {
      onbusychange?.(false);
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
    onbusychange?.(true);
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
      onbusychange?.(false);
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

  // D-15: persists a v2 archive resume envelope BEFORE the redirect, so the
  // /ui/ landing can restore this exact dialog (mode + ids) after the OIDC
  // round trip. Replaces the redirect-only behaviour from plan 04-01.
  let archiveNotice = $state<string | undefined>(undefined);

  function onreauth(ids: string[]): void {
    persistResume({ returnPath, kind: 'archive', mode: archiveMode, ids });
    redirectToLogin();
  }

  // ---------------------------------------------------------------------------
  // Supersede (CUR-01, D-07, D-10)
  // ---------------------------------------------------------------------------

  let supersedeOpen = $state(false);
  let supersedeTargets = $state<Memory[]>([]);
  let supersedeFields = $state<SupersedeFields>({ summary: '', content: '', category: '', scope: '', tags: [] });
  let supersedePrefillShortId = $state('');
  let supersedeIdempotencyKey = $state('');
  let supersedeNotice = $state<string | undefined>(undefined);
  let supersedeResend = $state(false);

  // openSupersede resolves the target set to real Memory records (chip
  // display), prefills the correcting-record form from the newest
  // predecessor's FULL record (validate_only targets on the preview response
  // are compact -- content cleared -- so the prefill needs its own
  // GetMemory), and mints the ONE idempotency key this draft reuses across
  // every preview/commit/resend.
  export async function openSupersede(ids: string[]): Promise<void> {
    const uniqueIds = [...new Set(ids)];
    const records = await resolveRecords(uniqueIds);
    supersedeTargets = records;

    let newest: Memory | undefined;
    for (const r of records) {
      const rTime = r.createdAt ? timestampDate(r.createdAt).getTime() : 0;
      const nTime = newest?.createdAt ? timestampDate(newest.createdAt).getTime() : -1;
      if (!newest || rTime >= nTime) newest = r;
    }

    let full = newest;
    if (newest) {
      try {
        const resp = await queryClient.fetchQuery({
          queryKey: ['getMemory', newest.id],
          queryFn: () => engram.getMemory({ id: newest!.id })
        });
        if (resp.memory) full = resp.memory;
      } catch {
        // Keep the summary-shaped record as a fallback prefill source -- an
        // unresolvable full fetch degrades the prefill, not the whole open.
      }
    }

    supersedeFields = {
      summary: full?.summary ?? '',
      content: full?.content ?? '',
      category: full?.category ?? 'convention',
      scope: full && !full.scope.startsWith('rule:') ? full.scope : '',
      tags: full ? [...full.tags] : []
    };
    supersedePrefillShortId = full?.shortId ?? '';
    supersedeIdempotencyKey = crypto.randomUUID();
    supersedeNotice = undefined;
    supersedeResend = false;
    supersedeOpen = true;
  }

  function supersedeOnpreview(draft: SupersedeDraft, signal: AbortSignal): Promise<SupersedeMemoryResponse> {
    return previewSupersede(draft, signal);
  }

  // Add-target-by-id/short_id lookup: GetMemory accepts a short_id anywhere
  // an id is accepted (memory contract), so a single fetch resolves either
  // shape -- no separate short_id resolution RPC exists.
  async function supersedeOnlookup(value: string): Promise<Memory | undefined> {
    try {
      const resp = await queryClient.fetchQuery({
        queryKey: ['getMemory', value],
        queryFn: () => engram.getMemory({ id: value })
      });
      return resp.memory;
    } catch {
      return undefined;
    }
  }

  // "use head {short_id}" resolution: a RelatedMemories read from the
  // superseded target finds its SUCCESSOR edge (chain.ts's own headIdFrom,
  // 04-04), then GetMemory fetches the resolved head's full-enough record
  // for the swap. No successor found (target IS the head already, or the
  // read failed) resolves to undefined -- the dialog just keeps showing the
  // "…" placeholder until a later read succeeds.
  async function supersedeOnresolvehead(id: string): Promise<Memory | undefined> {
    try {
      const related = await engram.relatedMemories({ id, k: 1n, full: false });
      const headId = headIdFrom(related);
      if (!headId || headId === id) return undefined;
      const resp = await queryClient.fetchQuery({
        queryKey: ['getMemory', headId],
        queryFn: () => engram.getMemory({ id: headId })
      });
      return resp.memory;
    } catch {
      return undefined;
    }
  }

  // The commit path itself: useSupersedeMemory's own onSuccess already
  // patches supersededBy in place and invalidates recall surfaces (mirrors
  // useArchiveMemory); this wrapper adds the parts that need component-level
  // props -- the row flash and the route's onchanged notification.
  async function supersedeOnsubmit(draft: SupersedeDraft): Promise<SupersedeMemoryResponse> {
    onbusychange?.(true);
    try {
      const resp = await supersedeMutation.mutateAsync(draft);
      flashRows([...draft.targets, resp.id]);
      onchanged?.({ kind: 'supersede', ids: draft.targets, newId: resp.id });
      return resp;
    } finally {
      onbusychange?.(false);
    }
  }

  function supersedeOncancel(): void {
    supersedeOpen = false;
  }

  function supersedeOndone(): void {
    supersedeOpen = false;
  }

  // D-07 success-footer contract: both actions leave supersede-review mode,
  // so CurationSurfaces closes its own dialog state before forwarding to the
  // route's handler (navigate/flash for View superseded, sel-navigate for
  // Open) -- SupersedeDialog itself never sets `open`, per its
  // host-authoritative pattern.
  function handleViewSuperseded(ids: string[]): void {
    supersedeOpen = false;
    onviewsuperseded?.(ids);
  }

  function handleOpenRecord(id: string): void {
    supersedeOpen = false;
    onopenrecord?.(id);
  }

  // ---------------------------------------------------------------------------
  // Chain dialog (D-06)
  // ---------------------------------------------------------------------------

  let chainOpen = $state(false);
  let chainAnchor = $state('');

  export function openChain(id: string): void {
    chainAnchor = id;
    chainOpen = true;
  }

  // D-15: persists a v2 supersede resume envelope (targets in chip order,
  // the current field values, and the SAME idempotency_key the draft was
  // minted with) before the redirect, so a resend after re-auth replays
  // rather than duplicates.
  function handleSupersedeReauth(draft: SupersedeDraft): void {
    persistResume({
      returnPath,
      kind: 'supersede',
      targets: draft.targets,
      fields: { ...draft.fields },
      idempotencyKey: draft.idempotencyKey
    });
    redirectToLogin();
  }

  // ---------------------------------------------------------------------------
  // Re-auth resume (CUR-05, D-15/D-16)
  // ---------------------------------------------------------------------------
  //
  // reopenFromResume never reads or clears sessionStorage itself -- the
  // route (plans 04-08, 04-10) is the sole owner of the resume-envelope
  // read/delete lifecycle and hands the already-validated envelope in here.
  // Both restored dialogs
  // show the 'Signed in again — review and resend' notice and NEVER auto-
  // resubmit -- the operator reviews the re-validated preview and clicks
  // Resend/the verb button themselves.
  export async function reopenFromResume(env: CurationResumeEnvelope): Promise<void> {
    if (env.kind === 'supersede') {
      const records = await resolveRecordsKeepAll(env.targets);
      supersedeTargets = records;
      supersedeFields = supersedeFieldsFrom(env.fields);
      supersedePrefillShortId = '';
      supersedeIdempotencyKey = env.idempotencyKey;
      supersedeNotice = 'Signed in again — review and resend';
      supersedeResend = true;
      supersedeOpen = true;
    } else if (env.kind === 'archive') {
      const records = await resolveRecordsKeepAll(env.ids);
      archiveMode = env.mode;
      archiveOutcome = undefined;
      archiveRecords = records;
      archiveNotice = 'Signed in again — review and resend';
      archiveOpen = true;
    }
    // env.kind === 'delete' is the Rules view's own delete confirm (plan
    // 04-08/04-09), not a surface this host owns.
    onresumeapplied?.();
  }
</script>

<ArchiveConfirmDialog
  bind:open={archiveOpen}
  bind:mode={archiveMode}
  bind:pending={archivePending}
  bind:outcome={archiveOutcome}
  records={archiveRecords}
  notice={archiveNotice}
  {onsubmit}
  {oncancel}
  {ondone}
  {onundo}
  {onreauth}
/>

<SupersedeDialog
  bind:open={supersedeOpen}
  targets={supersedeTargets}
  fields={supersedeFields}
  idempotencyKey={supersedeIdempotencyKey}
  prefillShortId={supersedePrefillShortId}
  notice={supersedeNotice}
  resend={supersedeResend}
  onpreview={supersedeOnpreview}
  onsubmit={supersedeOnsubmit}
  onlookup={supersedeOnlookup}
  onresolvehead={supersedeOnresolvehead}
  oncancel={supersedeOncancel}
  ondone={supersedeOndone}
  onreauth={handleSupersedeReauth}
  onviewsuperseded={handleViewSuperseded}
  onopenrecord={handleOpenRecord}
/>

<ChainDialog
  bind:open={chainOpen}
  anchorId={chainAnchor}
  onsupersedehead={(id) => {
    chainOpen = false;
    openSupersede([id]);
  }}
  oncancel={() => (chainOpen = false)}
/>
