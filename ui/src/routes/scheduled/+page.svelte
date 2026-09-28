<script lang="ts">
  // CUR-04 (D-11, D-13): /scheduled lists windowed records — memories the
  // recall gate is hiding for a future or lapsed window — by state tab
  // (scheduled | expired | all), across every scope, each row showing its
  // window and when it reveals/expired. Cursor infinite scroll only, never
  // numbered pages (D-13's own rule).
  import { onMount } from 'svelte';
  import { toast } from 'svelte-sonner';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { createInfiniteQuery, createQuery, keepPreviousData } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { parseScheduledParams, encodeScheduledParams, type ScheduledParams, type ScheduledState } from '$lib/search/scheduled-params';
  import { relatedPath } from '$lib/search/related-params';
  import { scheduledHeaderParts, scheduledEmptyHeading, type HeaderPart } from '$lib/search/recall-header';
  import { windowRange, windowPhrase } from '$lib/time';
  import { memoryStateWords } from '$lib/memorystate';
  import { parseConnectError, fixRowsFor } from '$lib/errors/connect-error';
  import { peekResume, consumeResume, normalizeReturnPath } from '$lib/resume';
  import { registerCurationHost } from '$lib/curation/host.svelte.ts';
  import type { Memory } from '$lib/gen/engram_pb';
  import ResultsHeader from '$lib/components/ResultsHeader.svelte';
  import ResultsList from '$lib/components/ResultsList.svelte';
  import RecallSplit from '$lib/components/RecallSplit.svelte';
  import DetailPane from '$lib/components/DetailPane.svelte';
  import CurationSurfaces from '$lib/components/CurationSurfaces.svelte';
  import RecallState, { type RecallStateInput } from '$lib/components/RecallState.svelte';
  import * as Tabs from '$lib/components/ui/tabs';
  import { Button } from '$lib/components/ui/button';

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
  const firstPage = $derived(listQ.data?.pages[0]);
  const scopeCount = $derived(firstPage?.searchedScopes?.length ?? 0);

  // Loading (first load only) mirrors /search's own listLoading; busy is a
  // background refetch (tab switch, keepPreviousData) — appending a page
  // never dims the whole list (isFetchingNextPage excluded from both).
  const listLoading = $derived(listQ.isLoading && !listQ.data);
  const listBusy = $derived(listQ.isFetching && !listQ.isFetchingNextPage && !!listQ.data);

  // A first-load rejection has no data at all — a next-page failure leaves
  // page 1's data in place, so `parsedError` (which blanks the WHOLE list)
  // never fires for it; the trailing envelope+Retry row below is what
  // surfaces a page-2 failure instead.
  const parsedError = $derived(listQ.isError && !listQ.data ? parseConnectError(listQ.error) : undefined);
  const isEmpty = $derived(listQ.isSuccess && !listQ.isFetching && rows.length === 0);

  const recallState = $derived.by((): RecallStateInput | undefined => {
    if (parsedError) return { kind: 'error', parsed: parsedError, fixes: fixRowsFor(parsedError) };
    if (isEmpty) return { kind: 'empty', heading: scheduledEmptyHeading(params.state), fixes: [] };
    return undefined;
  });

  const headerParts = $derived.by((): HeaderPart[] => {
    if (!listQ.data) return [];
    return scheduledHeaderParts({ state: params.state, count: rows.length, scopeCount, more: listQ.hasNextPage });
  });

  const nextPageError = $derived(
    listQ.isFetchNextPageError && listQ.error ? parseConnectError(listQ.error) : undefined
  );

  const effectiveSel = $derived(params.sel);
  const inResults = $derived(rows.some((m) => m.id === effectiveSel));

  function closeSel() {
    navigate({ sel: '' });
  }
  function toggleOpen(id: string) {
    if (effectiveSel === id) closeSel();
    else navigate({ sel: id });
  }

  // D-03/D-04: navigate to the record's related view, carrying where it was
  // opened from.
  function openRelated(id: string) {
    goto(
      `${base}${relatedPath(id, { from: normalizeReturnPath(page.url.pathname + page.url.search), trail: [] })}`
    );
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

  // D-13's archive-on-expired rule (CUR-04/CUR-05, D-16): archive is the
  // only curation action /scheduled ever offers, and only for a row whose
  // window has already lapsed.
  const isExpired = (m: Memory) => memoryStateWords(m).includes('expired');

  let selectedIds = $state<string[]>([]);
  let curation: ReturnType<typeof CurationSurfaces> | undefined = $state();

  // Keeps the ids whose row is expired, in `rows` list order (never `ids`
  // order) -- a request whose targets include no expired row opens nothing
  // and says so instead.
  function archiveExpired(ids: string[]) {
    const kept = rows.filter((m) => ids.includes(m.id) && isExpired(m)).map((m) => m.id);
    if (kept.length === 0) {
      toast('Archive applies to expired rows only');
      return;
    }
    curation?.openArchive(kept);
  }

  onMount(() => {
    const env = peekResume();
    if (env && env.kind === 'archive') curation?.reopenFromResume(env);
    return registerCurationHost({
      actionsFor: (m) => (isExpired(m) ? ['archive'] : []),
      run: (action, ids) => {
        if (action === 'archive') archiveExpired(ids);
      }
    });
  });
</script>

<div class="scheduled-page">
  <div class="scheduled-head">
    <h1 class="scheduled-title">Scheduled</h1>
    <Tabs.Root value={params.state} onValueChange={(v) => navigate({ state: v as ScheduledState, sel: '' })}>
      <Tabs.List>
        <Tabs.Trigger value="scheduled">scheduled</Tabs.Trigger>
        <Tabs.Trigger value="expired">expired</Tabs.Trigger>
        <Tabs.Trigger value="all">all</Tabs.Trigger>
      </Tabs.List>
    </Tabs.Root>
  </div>
  <ResultsHeader
    parts={headerParts}
    k={0}
    busy={listBusy}
    selection={{
      count: selectedIds.length,
      onarchive: () => archiveExpired(selectedIds),
      onclear: () => (selectedIds = [])
    }}
  />
  <!-- CurationSurfaces lives in a STABLE location outside RecallSplit --
       that component switches its narrow/wide layout branch via a
       ResizeObserver measurement that settles a tick after mount, and both
       branches independently render the list snippet; mounting it inside
       either branch would tear down and recreate it (losing bind:this and
       the one-shot onMount resume-restore above) the first time the
       measured width crosses the narrow breakpoint. -->
  <div class="scheduled-toolbar">
    <CurationSurfaces
      bind:this={curation}
      returnPath={normalizeReturnPath(page.url.pathname + page.url.search)}
      onchanged={() => (selectedIds = [])}
      onresumeapplied={consumeResume}
    />
  </div>
  <div class="scheduled-body">
    <RecallSplit open={!!effectiveSel} onclose={closeSel} autoSaveId="engram-scheduled-split">
      {#snippet list()}
        {#if recallState}
          <RecallState state={recallState} onfix={() => listQ.refetch()} onretry={() => listQ.refetch()} />
        {:else}
          <ResultsList
            memories={rows}
            mode="unranked"
            label="Scheduled memories"
            openId={effectiveSel}
            loading={listLoading}
            busy={listBusy}
            hasMore={listQ.hasNextPage && !listQ.isFetchNextPageError}
            onloadmore={() => listQ.fetchNextPage()}
            onopen={toggleOpen}
            onescape={closeSel}
            selectable
            bind:selectedIds
            selectionKey={params.state}
            rowActions={(m) => (isExpired(m) ? ['archive'] : [])}
            onarchive={archiveExpired}
            onrelated={openRelated}
          >
            {#snippet rowTrailing(m: Memory)}
              <span class="win" title={windowRange(m)}>
                <span class="win-range">{windowRange(m)}</span>
                <span class="win-phrase">{windowPhrase(m)}</span>
              </span>
            {/snippet}
          </ResultsList>
          {#if listQ.isFetchingNextPage}
            <div class="loading-more-row" data-testid="loading-more">Loading more…</div>
          {:else if nextPageError}
            <div class="next-page-error" role="alert" data-testid="next-page-error">
              {#if nextPageError.kind === 'rejected'}
                <pre>field={nextPageError.fields.join(',')} hint={nextPageError.hint}: {nextPageError.detail}</pre>
              {:else if nextPageError.kind === 'opaque'}
                <pre>{nextPageError.detail}</pre>
              {:else}
                <pre>Could not load the next page</pre>
              {/if}
              <Button variant="outline" size="sm" onclick={() => listQ.fetchNextPage()}>Retry</Button>
            </div>
          {/if}
        {/if}
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
          onarchive={selectedMemory && isExpired(selectedMemory) ? (id) => archiveExpired([id]) : undefined}
          onrelated={openRelated}
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
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: calc(10 * var(--u));
    padding: calc(10 * var(--u)) calc(14 * var(--u)) calc(4 * var(--u));
  }
  .scheduled-title {
    font-size: calc(15 * var(--u));
    font-weight: 600;
  }
  .scheduled-toolbar {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    padding: calc(6 * var(--u)) calc(12 * var(--u));
    border-bottom: 1px solid var(--border-subtle, var(--border));
    flex: none;
  }
  .scheduled-body {
    flex: 1;
    min-height: 0;
  }
  /* E6 overflow: the relative phrase truncates before the window
     timestamps -- the range stays a fixed, never-shrinking track, the
     phrase is the one flex child allowed to ellipsize. */
  .win {
    display: flex;
    align-items: baseline;
    gap: calc(4 * var(--u));
    min-width: 0;
    color: var(--text-faint);
    font-size: calc(11.5 * var(--u));
  }
  .win-range {
    flex: none;
    white-space: nowrap;
  }
  .win-phrase {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .loading-more-row {
    flex: none;
    padding: calc(8 * var(--u)) calc(14 * var(--u));
    font-size: calc(11.5 * var(--u));
    color: var(--text-faint);
  }
  .next-page-error {
    flex: none;
    display: flex;
    align-items: center;
    gap: calc(8 * var(--u));
    padding: calc(8 * var(--u)) calc(14 * var(--u));
  }
  .next-page-error pre {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--destructive);
    margin: 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
