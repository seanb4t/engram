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
  const currentScope = $derived(page.url.searchParams.get('scope') ?? '');
  const searchQ = createQuery(() => {
    const query = page.url.searchParams.get('q') ?? '';
    const scope = page.url.searchParams.get('scope') ?? '';
    return { queryKey: ['searchMemories', query, scope], queryFn: () => engram.searchMemories({ query, scope, k: 50n }), enabled: !!query };
  });
  const detailQ = createQuery(() => {
    const s = page.url.searchParams.get('sel') ?? '';
    return { queryKey: ['getMemory', s], queryFn: () => engram.getMemory({ id: s }), enabled: !!s };
  });
  function select(id: string) { const sp = new URLSearchParams(page.url.searchParams); sp.set('sel', id); goto(`${base}/search?${sp}`); }
  // Clear the selection (WR-02): drops the ?sel param so the detail query
  // disables and never refetches a just-deleted tombstone.
  function clearSelection() { const sp = new URLSearchParams(page.url.searchParams); sp.delete('sel'); goto(`${base}/search?${sp}`); }

  // WriteSurfaces host (Plan 06): kind=memory, defaulting New memory's scope
  // to the current ?scope param (empty/manual entry when absent).
  let writeSurfaces: ReturnType<typeof WriteSurfaces> | undefined = $state();

  // Re-auth landing recovery -- see observe/+page.svelte for the full
  // rationale; consumeResume() fires only via WriteSurfaces'
  // onresumeapplied passthrough, never here directly.
  onMount(() => {
    const env = peekResume();
    if (env && env.kind === 'memory') writeSurfaces?.reopenFromResume(env);
  });
</script>
<div class="flex h-full min-h-0">
  <Resizable.PaneGroup direction="horizontal" class="flex-1 min-w-0">
    <Resizable.Pane defaultSize={60} minSize={35} class="flex flex-col min-h-0">
      <div class="flex items-center justify-end px-3 py-2 border-b border-border">
        <WriteSurfaces
          bind:this={writeSurfaces}
          kind="memory"
          scope={currentScope}
          onresumeapplied={consumeResume}
          ondeleted={(id) => { if (id === sel) clearSelection(); }}
        />
      </div>
      <div class="flex-1 overflow-y-auto">
        <MemoryList memories={searchQ.data?.memories ?? []} total={BigInt(searchQ.data?.memories.length ?? 0)} showScope={true} loading={searchQ.isLoading} error={searchQ.error} selectedId={sel} onselect={select}
          onedit={(id) => writeSurfaces?.openEdit(id)}
          ondelete={(id) => writeSurfaces?.requestDelete(id, 'memory')}
          onshare={(memory) => writeSurfaces?.requestShare(memory, 'memory')} />
      </div>
    </Resizable.Pane>
    <Resizable.Handle />
    <Resizable.Pane defaultSize={40} minSize={25} class="min-h-0">
      <MemoryDetail memory={detailQ.data?.memory} loading={detailQ.isLoading} error={detailQ.error}
        onedit={(id) => writeSurfaces?.openEdit(id)}
        ondelete={(id) => writeSurfaces?.requestDelete(id, 'memory')}
        onshare={(memory) => writeSurfaces?.requestShare(memory, 'memory')} />
    </Resizable.Pane>
  </Resizable.PaneGroup>
</div>
