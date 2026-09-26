<script lang="ts">
  // svelte-query v6: reactive queries take an options FUNCTION (re-run via runes);
  // `page` is the runes form from $app/state (no derived store); results are read
  // directly off the query object (no $).
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { parseObserveParams, observeSearch, listMemoriesKey, PAGE_LIMIT, type ObserveParams } from '$lib/queries';
  import { peekResume, consumeResume } from '$lib/resume';
  import { normalizeVisibility } from '$lib/mutations/memory';
  import ScopesSidebar from '$lib/components/ScopesSidebar.svelte';
  import ResultsList from '$lib/components/ResultsList.svelte';
  import RecallSplit from '$lib/components/RecallSplit.svelte';
  import DetailPane from '$lib/components/DetailPane.svelte';
  import WriteSurfaces from '$lib/components/WriteSurfaces.svelte';
  import * as Empty from '$lib/components/ui/empty';
  import * as Pagination from '$lib/components/ui/pagination';

  const params = $derived(parseObserveParams(page.url.searchParams));
  function navigate(next: Partial<ObserveParams>) {
    goto(`${base}/observe?${observeSearch({ ...params, ...next })}`, { keepFocus: true, noScroll: true });
  }

  const scopesQ = createQuery(() => ({ queryKey: ['listScopes'], queryFn: () => engram.listScopes({}) }));
  const listQ = createQuery(() => {
    const pp = parseObserveParams(page.url.searchParams);
    return {
      queryKey: listMemoriesKey(
        pp.scope, pp.categories, pp.visibility, PAGE_LIMIT, pp.offset,
        pp.includeArchived, pp.includeSuperseded, pp.includeScheduled, false
      ),
      queryFn: () => engram.listMemories({
        scope: pp.scope, limit: BigInt(PAGE_LIMIT), offset: BigInt(pp.offset), categories: pp.categories, visibility: pp.visibility,
        includeArchived: pp.includeArchived, includeSuperseded: pp.includeSuperseded, includeScheduled: pp.includeScheduled
      }),
      enabled: !!pp.scope
    };
  });
  const detailQ = createQuery(() => {
    const sel = parseObserveParams(page.url.searchParams).selectedId;
    return { queryKey: ['getMemory', sel], queryFn: () => engram.getMemory({ id: sel }), enabled: !!sel };
  });

  const listMemories = $derived(listQ.data?.memories ?? []);
  const inResults = $derived(listMemories.some((m) => m.id === params.selectedId));
  // D-07/Pitfall 3 parity with /search: the initial load shows the ResultsList
  // skeleton; a param-driven refetch (offset/filter change) dims the existing
  // rows via `busy` instead of flashing to empty.
  const listLoading = $derived(listQ.isLoading && !listQ.data);
  const listBusy = $derived(listQ.isFetching && !!listQ.data);

  // WriteSurfaces host (Plan 06): kind=memory, current scope default for New
  // memory. bind:this reaches the exact exported-method contract Task 1
  // shipped (openEdit/requestDelete/requestShare/reopenFromResume). Lives in
  // a STABLE location outside RecallSplit (mirrors /search, plan 02-09):
  // RecallSplit switches its narrow/wide layout branch based on a
  // ResizeObserver measurement that settles a tick after mount, and Svelte
  // tears down and recreates a snippet's content across that {#if}/{:else}
  // switch -- which would destroy and recreate WriteSurfaces (losing its
  // bind:this reference and the one-shot onMount resume-restore below) the
  // first time the measured width crosses the narrow breakpoint.
  let writeSurfaces: ReturnType<typeof WriteSurfaces> | undefined = $state();

  // Re-auth landing recovery (Codex round-3 HIGH): on mount, peek the
  // envelope the /ui/ root just routed back here for; only reopen when its
  // kind matches this route's write surface (memory). consumeResume() fires
  // ONLY via WriteSurfaces' onresumeapplied passthrough below -- i.e. only
  // after the form has applied the restored values -- never here directly.
  onMount(() => {
    const env = peekResume();
    if (env && env.kind === 'memory') writeSurfaces?.reopenFromResume(env);
  });
</script>

<div class="flex h-full min-h-0">
  <ScopesSidebar
    scopes={scopesQ.data?.scopes ?? []} activeScope={params.scope}
    categories={params.categories} visibility={params.visibility}
    includeArchived={params.includeArchived} includeSuperseded={params.includeSuperseded} includeScheduled={params.includeScheduled}
    loading={scopesQ.isLoading} error={scopesQ.error}
    onscope={(s) => navigate({ scope: s, offset: 0, selectedId: '' })}
    onfilter={(cats, vis) => navigate({ categories: cats, visibility: vis, offset: 0 })}
    oninclude={(archived, superseded, scheduled) => navigate({ includeArchived: archived, includeSuperseded: superseded, includeScheduled: scheduled, offset: 0 })}
  />
  <div class="flex-1 min-w-0 flex flex-col min-h-0">
    <div class="flex items-center justify-end px-3 py-2 border-b border-border">
      <WriteSurfaces
        bind:this={writeSurfaces}
        kind="memory"
        scope={params.scope}
        onresumeapplied={consumeResume}
        ondeleted={(id) => { if (id === params.selectedId) navigate({ selectedId: '' }); }}
      />
    </div>
    {#if !params.scope}
      <Empty.Root class="p-8"><Empty.Title>select a scope</Empty.Title><Empty.Description>choose a scope from the sidebar to browse its memories</Empty.Description></Empty.Root>
    {:else}
      <div class="flex-1 min-h-0">
        <RecallSplit open={!!params.selectedId} onclose={() => navigate({ selectedId: '' })} autoSaveId="engram-observe-split">
          {#snippet list()}
            <div class="flex flex-col h-full min-h-0">
              <div class="flex-1 overflow-y-auto min-h-0">
                <ResultsList
                  memories={listMemories}
                  mode="unranked"
                  label={`Memories in ${params.scope}`}
                  openId={params.selectedId}
                  loading={listLoading}
                  busy={listBusy}
                  onopen={(id) => navigate({ selectedId: params.selectedId === id ? '' : id })}
                  onescape={() => navigate({ selectedId: '' })}
                  onedit={(id) => writeSurfaces?.openEdit(id)}
                  onvisibility={(m) =>
                    normalizeVisibility(m.visibility) === 'shared'
                      ? writeSurfaces?.requestMakePrivate(m, 'memory')
                      : writeSurfaces?.requestShare(m, 'memory')}
                  ondelete={(id) => writeSurfaces?.requestDelete(id, 'memory')}
                />
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
            </div>
          {/snippet}
          {#snippet detail()}
            <DetailPane
              memory={detailQ.data?.memory}
              loading={detailQ.isLoading}
              error={detailQ.error}
              requestedId={params.selectedId}
              {inResults}
              onclose={() => navigate({ selectedId: '' })}
              onselect={(id) => navigate({ selectedId: id })}
              onedit={(id) => writeSurfaces?.openEdit(id)}
              onvisibility={(m) =>
                normalizeVisibility(m.visibility) === 'shared'
                  ? writeSurfaces?.requestMakePrivate(m, 'memory')
                  : writeSurfaces?.requestShare(m, 'memory')}
              ondelete={(id) => writeSurfaces?.requestDelete(id, 'memory')}
            />
          {/snippet}
        </RecallSplit>
      </div>
    {/if}
  </div>
</div>
