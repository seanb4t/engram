<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { peekResume, consumeResume } from '$lib/resume';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import MemoryDetail from '$lib/components/MemoryDetail.svelte';
  import WriteSurfaces from '$lib/components/WriteSurfaces.svelte';
  import * as Resizable from '$lib/components/ui/resizable';
  const sel = $derived(page.url.searchParams.get('sel') ?? '');
  // Never seed a discovery create scope from a raw memory/observe scope
  // (grok MEDIUM) -- only carry the ?scope param through when it's already
  // discovery:-prefixed, else leave it empty for manual entry.
  const createScope = $derived.by(() => {
    const sc = page.url.searchParams.get('scope') ?? '';
    return sc.startsWith('discovery:') ? sc : '';
  });
  const discQ = createQuery(() => {
    const query = page.url.searchParams.get('q') ?? '';
    const sc = page.url.searchParams.get('scope') ?? '';
    return { queryKey: ['searchDiscoveries', query, sc], queryFn: () => engram.searchDiscoveries({ query, scope: sc, k: 50n }), enabled: !!query };
  });
  const detailQ = createQuery(() => {
    const s = page.url.searchParams.get('sel') ?? '';
    return { queryKey: ['getMemory', s], queryFn: () => engram.getMemory({ id: s }), enabled: !!s };
  });
  function select(id: string) { const sp = new URLSearchParams(page.url.searchParams); sp.set('sel', id); goto(`${base}/discovery?${sp}`); }
  // Clear the selection (WR-02): drops the ?sel param so the detail query
  // disables and never refetches a just-deleted tombstone.
  function clearSelection() { const sp = new URLSearchParams(page.url.searchParams); sp.delete('sel'); goto(`${base}/discovery?${sp}`); }

  // WriteSurfaces host (Plan 06): kind=discovery -- D-04 fence, no onedit
  // wired below (discovery has no edit surface).
  let writeSurfaces: ReturnType<typeof WriteSurfaces> | undefined = $state();

  // Re-auth landing recovery -- see observe/+page.svelte for the full
  // rationale; consumeResume() fires only via WriteSurfaces'
  // onresumeapplied passthrough, never here directly.
  onMount(() => {
    const env = peekResume();
    if (env && env.kind === 'discovery') writeSurfaces?.reopenFromResume(env);
  });
</script>
<div class="flex h-full min-h-0">
  <Resizable.PaneGroup direction="horizontal" class="flex-1 min-w-0">
    <Resizable.Pane defaultSize={60} minSize={35} class="flex flex-col min-h-0">
      <div class="flex items-center justify-end px-3 py-2 border-b border-border">
        <WriteSurfaces
          bind:this={writeSurfaces}
          kind="discovery"
          scope={createScope}
          onresumeapplied={consumeResume}
          ondeleted={(id) => { if (id === sel) clearSelection(); }}
        />
      </div>
      <div class="flex-1 overflow-y-auto">
        <MemoryList
          memories={discQ.data?.discoveries ?? []}
          total={BigInt(discQ.data?.discoveries.length ?? 0)}
          showScope={true}
          loading={discQ.isLoading}
          error={discQ.error}
          selectedId={sel}
          onselect={select}
          ondelete={(id) => writeSurfaces?.requestDelete(id, 'discovery')}
          onshare={(memory) => writeSurfaces?.requestShare(memory, 'discovery')}
        />
      </div>
    </Resizable.Pane>
    <Resizable.Handle />
    <Resizable.Pane defaultSize={40} minSize={25} class="min-h-0">
      <MemoryDetail memory={detailQ.data?.memory} loading={detailQ.isLoading} error={detailQ.error}
        ondelete={(id) => writeSurfaces?.requestDelete(id, 'discovery')}
        onshare={(memory) => writeSurfaces?.requestShare(memory, 'discovery')} />
    </Resizable.Pane>
  </Resizable.PaneGroup>
</div>
