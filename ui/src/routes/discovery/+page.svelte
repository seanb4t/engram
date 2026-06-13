<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import MemoryDetail from '$lib/components/MemoryDetail.svelte';
  import * as Resizable from '$lib/components/ui/resizable';
  const sel = $derived(page.url.searchParams.get('sel') ?? '');
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
</script>
<div class="flex h-full min-h-0">
  <Resizable.PaneGroup direction="horizontal" class="flex-1 min-w-0">
    <Resizable.Pane defaultSize={60} minSize={35} class="flex flex-col min-h-0">
      <div class="flex-1 overflow-y-auto">
        <MemoryList
          memories={discQ.data?.discoveries ?? []}
          total={BigInt(discQ.data?.discoveries.length ?? 0)}
          showScope={true}
          loading={discQ.isLoading}
          error={discQ.error}
          selectedId={sel}
          onselect={select}
        />
      </div>
    </Resizable.Pane>
    <Resizable.Handle />
    <Resizable.Pane defaultSize={40} minSize={25} class="min-h-0">
      <MemoryDetail memory={detailQ.data?.memory} loading={detailQ.isLoading} error={detailQ.error} />
    </Resizable.Pane>
  </Resizable.PaneGroup>
</div>
