<script lang="ts">
  // v5 reactive queries: pass a `derived` store of the options (NOT $derived(createQuery)).
  // `page` is the STORE form from $app/stores; query results are stores accessed with $.
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { derived } from 'svelte/store';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { parseObserveParams, observeSearch, listMemoriesKey, PAGE_LIMIT, type ObserveParams } from '$lib/queries';
  import ScopeRail from '$lib/components/ScopeRail.svelte';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import MemoryDetail from '$lib/components/MemoryDetail.svelte';
  import { Button } from '$lib/components/ui/button';

  const params = $derived(parseObserveParams($page.url.searchParams));
  function navigate(next: Partial<ObserveParams>) {
    goto(`${base}/observe?${observeSearch({ ...params, ...next })}`, { keepFocus: true, noScroll: true });
  }

  const scopesQ = createQuery({ queryKey: ['listScopes'], queryFn: () => engram.listScopes({}) });
  const listQ = createQuery(derived(page, ($p) => {
    const pp = parseObserveParams($p.url.searchParams);
    return {
      queryKey: listMemoriesKey(pp.scope, pp.categories, pp.visibility, PAGE_LIMIT, pp.offset),
      queryFn: () => engram.listMemories({ scope: pp.scope, limit: BigInt(PAGE_LIMIT), offset: BigInt(pp.offset), categories: pp.categories, visibility: pp.visibility }),
      enabled: !!pp.scope
    };
  }));
  const detailQ = createQuery(derived(page, ($p) => {
    const sel = parseObserveParams($p.url.searchParams).selectedId;
    return { queryKey: ['getMemory', sel], queryFn: () => engram.getMemory({ id: sel }), enabled: !!sel };
  }));
</script>

<div class="flex">
  <ScopeRail
    scopes={$scopesQ.data?.scopes ?? []}
    activeScope={params.scope}
    categories={params.categories}
    visibility={params.visibility}
    loading={$scopesQ.isLoading}
    error={$scopesQ.error}
    onscope={(s) => navigate({ scope: s, offset: 0, selectedId: '' })}
    onfilter={(cats, vis) => navigate({ categories: cats, visibility: vis, offset: 0 })}
  />
  <div class="flex-1">
    <MemoryList
      memories={$listQ.data?.memories ?? []}
      total={$listQ.data?.total ?? 0n}
      approximate={$listQ.data?.approximate ?? false}
      loading={$listQ.isLoading}
      error={$listQ.error}
      selectedId={params.selectedId}
      onselect={(id) => navigate({ selectedId: id })}
    />
    <div class="flex justify-between px-3 py-1 eg-muted">
      <Button variant="ghost" size="sm" disabled={params.offset === 0} onclick={() => navigate({ offset: Math.max(0, params.offset - PAGE_LIMIT) })}>‹ prev</Button>
      <Button variant="ghost" size="sm" disabled={params.offset + PAGE_LIMIT >= Number($listQ.data?.total ?? 0n)} onclick={() => navigate({ offset: params.offset + PAGE_LIMIT })}>next ›</Button>
    </div>
  </div>
  <MemoryDetail memory={$detailQ.data?.memory} loading={$detailQ.isLoading} error={$detailQ.error} />
</div>
