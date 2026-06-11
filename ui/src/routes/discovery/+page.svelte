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
  const discQ = createQuery(derived(page, ($p) => {
    const query = $p.url.searchParams.get('q') ?? '';
    const sc = $p.url.searchParams.get('scope') ?? '';
    return { queryKey: ['searchDiscoveries', query, sc], queryFn: () => engram.searchDiscoveries({ query, scope: sc, k: 50n }), enabled: !!query };
  }));
  const detailQ = createQuery(derived(page, ($p) => {
    const s = $p.url.searchParams.get('sel') ?? '';
    return { queryKey: ['getMemory', s], queryFn: () => engram.getMemory({ id: s }), enabled: !!s };
  }));
  function setQuery(next: string) {
    const sp = new URLSearchParams($page.url.searchParams);
    sp.set('q', next);
    goto(`${base}/discovery?${sp}`);
  }
  function select(id: string) { const sp = new URLSearchParams($page.url.searchParams); sp.set('sel', id); goto(`${base}/discovery?${sp}`); }
</script>
<div class="p-3"><SearchPalette value={q} onsubmit={setQuery} /></div>
<div class="flex">
  <div class="flex-1">
    <MemoryList
      memories={$discQ.data?.discoveries ?? []}
      total={BigInt($discQ.data?.discoveries.length ?? 0)}
      showTotal={false}
      loading={$discQ.isLoading}
      error={$discQ.error}
      selectedId={sel}
      onselect={select}
    />
  </div>
  <MemoryDetail memory={$detailQ.data?.memory} loading={$detailQ.isLoading} error={$detailQ.error} />
</div>
