<script lang="ts">
  // svelte-query v6: reactive queries take an options FUNCTION (re-run via runes);
  // `page` is the runes form from $app/state (no derived store); results are read
  // directly off the query object (no $).
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { parseObserveParams, observeSearch, listMemoriesKey, PAGE_LIMIT, type ObserveParams } from '$lib/queries';
  import ScopeRail from '$lib/components/ScopeRail.svelte';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import MemoryDetail from '$lib/components/MemoryDetail.svelte';
  import { Button } from '$lib/components/ui/button';

  const params = $derived(parseObserveParams(page.url.searchParams));
  function navigate(next: Partial<ObserveParams>) {
    goto(`${base}/observe?${observeSearch({ ...params, ...next })}`, { keepFocus: true, noScroll: true });
  }

  const scopesQ = createQuery(() => ({ queryKey: ['listScopes'], queryFn: () => engram.listScopes({}) }));
  const listQ = createQuery(() => {
    const pp = parseObserveParams(page.url.searchParams);
    return {
      queryKey: listMemoriesKey(pp.scope, pp.categories, pp.visibility, PAGE_LIMIT, pp.offset),
      queryFn: () => engram.listMemories({ scope: pp.scope, limit: BigInt(PAGE_LIMIT), offset: BigInt(pp.offset), categories: pp.categories, visibility: pp.visibility }),
      enabled: !!pp.scope
    };
  });
  const detailQ = createQuery(() => {
    const sel = parseObserveParams(page.url.searchParams).selectedId;
    return { queryKey: ['getMemory', sel], queryFn: () => engram.getMemory({ id: sel }), enabled: !!sel };
  });
</script>

<div class="flex">
  <ScopeRail
    scopes={scopesQ.data?.scopes ?? []}
    activeScope={params.scope}
    categories={params.categories}
    visibility={params.visibility}
    loading={scopesQ.isLoading}
    error={scopesQ.error}
    onscope={(s) => navigate({ scope: s, offset: 0, selectedId: '' })}
    onfilter={(cats, vis) => navigate({ categories: cats, visibility: vis, offset: 0 })}
  />
  <div class="flex-1">
    <MemoryList
      memories={listQ.data?.memories ?? []}
      total={listQ.data?.total ?? 0n}
      approximate={listQ.data?.approximate ?? false}
      loading={listQ.isLoading}
      error={listQ.error}
      selectedId={params.selectedId}
      onselect={(id) => navigate({ selectedId: id })}
    />
    <div class="flex justify-between px-3 py-1 eg-muted">
      <Button variant="ghost" size="sm" disabled={params.offset === 0} onclick={() => navigate({ offset: Math.max(0, params.offset - PAGE_LIMIT) })}>‹ prev</Button>
      <Button variant="ghost" size="sm" disabled={params.offset + PAGE_LIMIT >= Number(listQ.data?.total ?? 0n)} onclick={() => navigate({ offset: params.offset + PAGE_LIMIT })}>next ›</Button>
    </div>
  </div>
  <MemoryDetail memory={detailQ.data?.memory} loading={detailQ.isLoading} error={detailQ.error} />
</div>
