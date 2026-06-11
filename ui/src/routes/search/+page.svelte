<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { derived } from 'svelte/store';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import SearchPalette from '$lib/components/SearchPalette.svelte';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import MemoryDetail from '$lib/components/MemoryDetail.svelte';
  const q = $derived($page.url.searchParams.get('q') ?? '');
  const sel = $derived($page.url.searchParams.get('sel') ?? '');
  const searchQ = createQuery(derived(page, ($p) => {
    const query = $p.url.searchParams.get('q') ?? '';
    const scope = $p.url.searchParams.get('scope') ?? '';
    return { queryKey: ['searchMemories', query, scope], queryFn: () => engram.searchMemories({ query, scope, k: 50n }), enabled: !!query };
  }));
  const detailQ = createQuery(derived(page, ($p) => {
    const s = $p.url.searchParams.get('sel') ?? '';
    return { queryKey: ['getMemory', s], queryFn: () => engram.getMemory({ id: s }), enabled: !!s };
  }));
  function setQuery(next: string) { goto(`${base}/search?q=${encodeURIComponent(next)}`); }
  function select(id: string) { const sp = new URLSearchParams($page.url.searchParams); sp.set('sel', id); goto(`${base}/search?${sp}`); }
</script>
<div class="p-3"><SearchPalette value={q} onsubmit={setQuery} /></div>
<div class="flex">
  <div class="flex-1"><MemoryList memories={$searchQ.data?.memories ?? []} total={BigInt($searchQ.data?.memories.length ?? 0)} loading={$searchQ.isLoading} error={$searchQ.error} selectedId={sel} onselect={select} /></div>
  <MemoryDetail memory={$detailQ.data?.memory} loading={$detailQ.isLoading} error={$detailQ.error} />
</div>
