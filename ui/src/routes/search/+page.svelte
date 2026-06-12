<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import MemoryDetail from '$lib/components/MemoryDetail.svelte';
  const scope = $derived(page.url.searchParams.get('scope') ?? '');
  const sel = $derived(page.url.searchParams.get('sel') ?? '');
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
</script>
<div class="flex">
  <div class="flex-1"><MemoryList memories={searchQ.data?.memories ?? []} total={BigInt(searchQ.data?.memories.length ?? 0)} showScope={true} loading={searchQ.isLoading} error={searchQ.error} selectedId={sel} onselect={select} /></div>
  <MemoryDetail memory={detailQ.data?.memory} loading={detailQ.isLoading} error={detailQ.error} />
</div>
