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
  import ScopesSidebar from '$lib/components/ScopesSidebar.svelte';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import MemoryDetail from '$lib/components/MemoryDetail.svelte';
  import * as Resizable from '$lib/components/ui/resizable';
  import * as Pagination from '$lib/components/ui/pagination';

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

<div class="flex h-full min-h-0">
  <ScopesSidebar
    scopes={scopesQ.data?.scopes ?? []} activeScope={params.scope}
    categories={params.categories} visibility={params.visibility}
    loading={scopesQ.isLoading} error={scopesQ.error}
    onscope={(s) => navigate({ scope: s, offset: 0, selectedId: '' })}
    onfilter={(cats, vis) => navigate({ categories: cats, visibility: vis, offset: 0 })}
  />
  <Resizable.PaneGroup direction="horizontal" class="flex-1 min-w-0">
    <Resizable.Pane defaultSize={60} minSize={35} class="flex flex-col min-h-0">
      <div class="flex-1 overflow-y-auto">
        <MemoryList memories={listQ.data?.memories ?? []} total={listQ.data?.total ?? 0n}
          approximate={listQ.data?.approximate ?? false} loading={listQ.isLoading} error={listQ.error}
          selectedId={params.selectedId} onselect={(id) => navigate({ selectedId: id })} />
      </div>
      <Pagination.Root
        count={Number(listQ.data?.total ?? 0n)}
        perPage={PAGE_LIMIT}
        page={Math.floor(params.offset / PAGE_LIMIT) + 1}
        onPageChange={(p) => navigate({ offset: (p - 1) * PAGE_LIMIT })}
      >
        {#snippet children({ pages, currentPage })}
          <Pagination.Content>
            <Pagination.Item><Pagination.Previous /></Pagination.Item>
            {#each pages as pg (pg.key)}
              {#if pg.type === 'ellipsis'}
                <Pagination.Item><Pagination.Ellipsis /></Pagination.Item>
              {:else}
                <Pagination.Item><Pagination.Link page={pg} isActive={currentPage === pg.value}>{pg.value}</Pagination.Link></Pagination.Item>
              {/if}
            {/each}
            <Pagination.Item><Pagination.Next /></Pagination.Item>
          </Pagination.Content>
        {/snippet}
      </Pagination.Root>
    </Resizable.Pane>
    <Resizable.Handle />
    <Resizable.Pane defaultSize={40} minSize={25} class="min-h-0">
      <MemoryDetail memory={detailQ.data?.memory} loading={detailQ.isLoading} error={detailQ.error} />
    </Resizable.Pane>
  </Resizable.PaneGroup>
</div>
