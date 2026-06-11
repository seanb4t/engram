<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { derived } from 'svelte/store';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import SearchPalette from '$lib/components/SearchPalette.svelte';
  import MemoryList from '$lib/components/MemoryList.svelte';
  const q = $derived($page.url.searchParams.get('q') ?? '');
  const scope = $derived($page.url.searchParams.get('scope') ?? '');
  const discQ = createQuery(derived(page, ($p) => {
    const query = $p.url.searchParams.get('q') ?? '';
    const sc = $p.url.searchParams.get('scope') ?? '';
    return { queryKey: ['searchDiscoveries', query, sc], queryFn: () => engram.searchDiscoveries({ query, scope: sc, k: 50n }), enabled: !!query };
  }));
  function setQuery(next: string) { goto(`${base}/discovery?q=${encodeURIComponent(next)}${scope ? `&scope=${encodeURIComponent(scope)}` : ''}`); }
</script>
<div class="p-3"><SearchPalette value={q} onsubmit={setQuery} /></div>
<MemoryList memories={$discQ.data?.discoveries ?? []} total={BigInt($discQ.data?.discoveries.length ?? 0)} loading={$discQ.isLoading} error={$discQ.error} selectedId="" onselect={() => {}} />
