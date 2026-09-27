<script lang="ts">
  // CUR-04 tracer (D-11, D-13): /scheduled lists windowed records — memories
  // the recall gate is hiding for a future or lapsed window — across every
  // scope, showing each row's window and when it reveals/expired.
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { createInfiniteQuery, createQuery, keepPreviousData } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { parseScheduledParams, encodeScheduledParams, type ScheduledParams } from '$lib/search/scheduled-params';
  import { windowRange, windowPhrase } from '$lib/time';
  import type { Memory } from '$lib/gen/engram_pb';
  import ResultsHeader from '$lib/components/ResultsHeader.svelte';
  import ResultsList from '$lib/components/ResultsList.svelte';
  import RecallSplit from '$lib/components/RecallSplit.svelte';
  import DetailPane from '$lib/components/DetailPane.svelte';

  const params = $derived(parseScheduledParams(page.url.searchParams));

  function navigate(next: Partial<ScheduledParams>) {
    goto(`${base}/scheduled?${encodeScheduledParams({ ...params, ...next })}`, { keepFocus: true, noScroll: true });
  }

  const listQ = createInfiniteQuery(() => ({
    queryKey: ['listScheduled', params.state, true],
    queryFn: ({ pageParam, signal }) =>
      engram.listScheduled(
        {
          scope: '',
          state: params.state,
          limit: 50n,
          createdAfter: '',
          createdBefore: '',
          crossSpine: true,
          pageToken: pageParam as string
        },
        { signal }
      ),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextPageToken || undefined,
    placeholderData: keepPreviousData,
    meta: { silent: true }
  }));

  const rows = $derived(listQ.data?.pages.flatMap((p) => p.memories) ?? []);
  const listLoading = $derived(listQ.isLoading && !listQ.data);
  const listBusy = $derived(listQ.isFetching && !listQ.isFetchingNextPage && !!listQ.data);

  const effectiveSel = $derived(params.sel);
  const inResults = $derived(rows.some((m) => m.id === effectiveSel));

  function closeSel() {
    navigate({ sel: '' });
  }
  function toggleOpen(id: string) {
    if (effectiveSel === id) closeSel();
    else navigate({ sel: id });
  }

  const detailQ = createQuery(() => ({
    queryKey: ['getMemory', effectiveSel],
    queryFn: ({ signal }) => engram.getMemory({ id: effectiveSel }, { signal }),
    enabled: !!effectiveSel && !rows.some((m) => m.id === effectiveSel),
    meta: { silent: true }
  }));

  const selectedMemory = $derived.by(() => {
    if (!effectiveSel) return undefined;
    return rows.find((m) => m.id === effectiveSel) ?? detailQ.data?.memory;
  });
</script>

<div class="scheduled-page">
  <div class="scheduled-head">
    <h1 class="scheduled-title">Scheduled</h1>
  </div>
  <ResultsHeader parts={[]} k={0} busy={listBusy} />
  <div class="scheduled-body">
    <RecallSplit open={!!effectiveSel} onclose={closeSel} autoSaveId="engram-scheduled-split">
      {#snippet list()}
        <ResultsList
          memories={rows}
          mode="unranked"
          label="Scheduled memories"
          openId={effectiveSel}
          loading={listLoading}
          busy={listBusy}
          hasMore={listQ.hasNextPage}
          onloadmore={() => listQ.fetchNextPage()}
          onopen={toggleOpen}
          onescape={closeSel}
        >
          {#snippet rowTrailing(m: Memory)}
            <span class="win" title={windowRange(m)}>{windowRange(m)} · {windowPhrase(m)}</span>
          {/snippet}
        </ResultsList>
      {/snippet}
      {#snippet detail()}
        <DetailPane
          memory={selectedMemory}
          loading={detailQ.isLoading}
          error={detailQ.error}
          requestedId={effectiveSel}
          {inResults}
          onclose={closeSel}
          onselect={(id) => navigate({ sel: id })}
        />
      {/snippet}
    </RecallSplit>
  </div>
</div>

<style>
  .scheduled-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .scheduled-head {
    flex: none;
    padding: calc(10 * var(--u)) calc(14 * var(--u)) calc(4 * var(--u));
  }
  .scheduled-title {
    font-size: calc(15 * var(--u));
    font-weight: 600;
  }
  .scheduled-body {
    flex: 1;
    min-height: 0;
  }
  .win {
    color: var(--text-faint);
    font-size: calc(11.5 * var(--u));
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
