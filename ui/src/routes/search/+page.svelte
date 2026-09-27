<script lang="ts">
  // Rebuilt as the recall surface (ENTRY-01/02/06, SC4): URL-driven, race-safe
  // queries (every query passes `{ signal }` and keys on the full normalized
  // params), the honest results header, and the shared ResultsList/RecallSplit/
  // DetailPane set from plans 02-06/02-07.
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { ConnectError, Code } from '@connectrpc/connect';
  import { createQuery, createInfiniteQuery, keepPreviousData } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { peekResume, consumeResume } from '$lib/resume';
  import { normalizeVisibility } from '$lib/mutations/memory';
  import { parseConnectError, fixRowsFor } from '$lib/errors/connect-error';
  import {
    parseSearchParams,
    encodeSearchParams,
    searchMemoriesKey,
    searchMemoriesRequest,
    applyChips,
    nextK,
    listMemoriesCursorKey,
    listMemoriesRequest,
    K_STEPS,
    DEFAULT_K,
    type SearchParams
  } from '$lib/search/params';
  import { classifyInput, type OperatorChip } from '$lib/search/classify';
  import {
    rankedHeaderParts,
    listingHeaderParts,
    emptyHeading,
    hiddenFromProto,
    loadingLine,
    resolutionLine,
    type HeaderPart,
    type ResolutionInput,
    type HiddenCounts
  } from '$lib/search/recall-header';
  import ResultsHeader from '$lib/components/ResultsHeader.svelte';
  import FacetStrip from '$lib/components/FacetStrip.svelte';
  import ResultsList from '$lib/components/ResultsList.svelte';
  import RecallSplit from '$lib/components/RecallSplit.svelte';
  import DetailPane from '$lib/components/DetailPane.svelte';
  import WriteSurfaces from '$lib/components/WriteSurfaces.svelte';
  import RecallState, { type RecallStateInput } from '$lib/components/RecallState.svelte';
  import { Button } from '$lib/components/ui/button';

  const params = $derived(parseSearchParams(page.url.searchParams));
  const classified = $derived(classifyInput(params.q));

  // Facet-strip state (scope/tags/categories) lives directly in the URL
  // (Task 3's FacetStrip), independent of any operator token typed into the
  // query box. Synthesizing it as chips and folding it in AHEAD of the
  // classifier's own inline chips (scope:/#tag/is:) lets applyChips' "last
  // scope wins"/union-tags rule combine both sources correctly, since
  // applyChips derives tags/categories from the chip set alone.
  const facetChips = $derived.by((): OperatorChip[] => {
    const list: OperatorChip[] = [];
    if (params.scope) list.push({ kind: 'scope', value: params.scope });
    for (const t of params.tags) list.push({ kind: 'tag', value: t });
    for (const c of params.categories) list.push({ kind: 'category', value: c, known: true });
    return list;
  });
  const inlineChips = $derived(classified.kind === 'text' || classified.kind === 'operators' ? classified.chips : []);
  const effective = $derived(applyChips(params, [...facetChips, ...inlineChips]));

  function navigate(next: Partial<SearchParams>) {
    goto(`${base}/search?${encodeSearchParams({ ...params, ...next })}`, { keepFocus: true, noScroll: true });
  }

  // The page's own query input (debounced 160ms into the URL, replaceState so
  // typing doesn't spam browser history).
  let inputText = $state(params.q);
  $effect(() => {
    inputText = params.q;
  });
  let debounceTimer: ReturnType<typeof setTimeout> | undefined;
  function onInput(e: Event) {
    const value = (e.currentTarget as HTMLInputElement).value;
    inputText = value;
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      goto(`${base}/search?${encodeSearchParams({ ...params, q: value, sel: '' })}`, {
        replaceState: true,
        keepFocus: true,
        noScroll: true
      });
    }, 160);
  }

  // ENTRY-05 fix row "without-full": a page-local override, not a URL param
  // (retrying without full content is a one-off mitigation for a
  // response_too_large rejection, not a durable preference worth persisting
  // or sharing via a link).
  let fullOverride = $state(true);

  const idToFetch = $derived(
    classified.kind === 'id' ? classified.id : classified.kind === 'short_id' ? classified.shortId : ''
  );
  const idQ = createQuery(() => ({
    queryKey: ['getMemory', idToFetch],
    queryFn: ({ signal }) => engram.getMemory({ id: idToFetch }, { signal }),
    enabled: classified.kind === 'id' || classified.kind === 'short_id',
    meta: { silent: true }
  }));

  // A short_id-shaped input GetMemory reports not-found is re-searched as
  // text — never a silent reinterpretation (carried-forward rule).
  const shortIdMissText = $derived(
    classified.kind === 'short_id' && idQ.error instanceof ConnectError && idQ.error.code === Code.NotFound
      ? classified.shortId
      : ''
  );
  const fallbackQ = createQuery(() => ({
    queryKey: searchMemoriesKey({ ...effective, q: shortIdMissText }, fullOverride),
    queryFn: ({ signal }) =>
      engram.searchMemories(searchMemoriesRequest({ ...effective, q: shortIdMissText }, fullOverride), { signal }),
    enabled: !!shortIdMissText,
    placeholderData: keepPreviousData,
    meta: { silent: true }
  }));

  const searchQ = createQuery(() => ({
    queryKey: searchMemoriesKey(effective, fullOverride),
    queryFn: ({ signal }) => engram.searchMemories(searchMemoriesRequest(effective, fullOverride), { signal }),
    enabled: classified.kind === 'text',
    placeholderData: keepPreviousData,
    meta: { silent: true }
  }));

  const scopesQ = createQuery(() => ({
    queryKey: ['listScopes'],
    queryFn: ({ signal }) => engram.listScopes({}, { signal }),
    meta: { silent: true }
  }));

  // D-09: operator-only input (no free text — e.g. `scope:x #tag is:gotcha`)
  // is an unranked ListMemories cursor listing that infinite-scrolls, never a
  // SearchMemories call.
  const listQ = createInfiniteQuery(() => ({
    queryKey: listMemoriesCursorKey(effective),
    queryFn: ({ pageParam, signal }) => engram.listMemories(listMemoriesRequest(effective, pageParam as string), { signal }),
    initialPageParam: '',
    getNextPageParam: (last) => last.nextPageToken || undefined,
    enabled: classified.kind === 'operators',
    meta: { silent: true }
  }));

  const listMemories = $derived(listQ.data?.pages.flatMap((p) => p.memories) ?? []);
  const listHiddenPages = $derived(listQ.data?.pages.map((p) => hiddenFromProto(p.recallGateHidden)) ?? []);
  const listScopeCount = $derived.by(() => {
    const first = listQ.data?.pages[0];
    if (!first) return 0;
    return effective.scope ? 1 : (first.searchedScopes?.length ?? 0);
  });
  // isFetchingNextPage is excluded so appending a page never dims the whole
  // list — only the initial load and a param-driven refetch do (Pitfall 3).
  const listBusy = $derived(listQ.isFetching && !listQ.isFetchingNextPage && !!listQ.data);
  const listLoading = $derived(listQ.isLoading && !listQ.data);

  const memories = $derived.by(() => {
    if (classified.kind === 'id' || classified.kind === 'short_id') {
      if (shortIdMissText) return fallbackQ.data?.memories ?? [];
      return idQ.data?.memory ? [idQ.data.memory] : [];
    }
    if (classified.kind === 'text') return searchQ.data?.memories ?? [];
    if (classified.kind === 'operators') return listMemories;
    return [];
  });

  // An id/short_id resolution opens the pane on the resolved record even
  // though the URL never carried an explicit `sel` (ENTRY-01: "shows the
  // resolution line and one row, and opens the pane"); an explicit `sel`
  // (from clicking a row, or from a shared /search?...&sel=... URL) always
  // wins otherwise.
  const autoOpenId = $derived(
    (classified.kind === 'id' || classified.kind === 'short_id') && idQ.data?.memory && !shortIdMissText
      ? idQ.data.memory.id
      : ''
  );
  const effectiveSel = $derived(params.sel || autoOpenId);
  const inResults = $derived(memories.some((m) => m.id === effectiveSel));

  const detailQ = createQuery(() => ({
    queryKey: ['getMemory', effectiveSel],
    queryFn: ({ signal }) => engram.getMemory({ id: effectiveSel }, { signal }),
    enabled: !!effectiveSel && !memories.some((m) => m.id === effectiveSel),
    meta: { silent: true }
  }));

  const selectedMemory = $derived.by(() => {
    if (!effectiveSel) return undefined;
    return memories.find((m) => m.id === effectiveSel) ?? detailQ.data?.memory;
  });

  // ROW-05: the category filter's unfiltered total ("N hits of M"). When no
  // category filter is active, `searchQ` already ran with `categories: []`
  // (the same key this query would build), so counts come straight from its
  // own hits and this second query stays disabled — no redundant fetch.
  const categoryCountsQ = createQuery(() => ({
    queryKey: searchMemoriesKey({ ...effective, categories: [] }, fullOverride),
    queryFn: ({ signal }) =>
      engram.searchMemories(searchMemoriesRequest({ ...effective, categories: [] }, fullOverride), { signal }),
    enabled: classified.kind === 'text' && effective.categories.length > 0,
    placeholderData: keepPreviousData,
    meta: { silent: true }
  }));

  const categoryCounts = $derived.by((): Record<string, number> | undefined => {
    if (classified.kind !== 'text') return undefined;
    const source = effective.categories.length > 0 ? categoryCountsQ.data?.memories : searchQ.data?.memories;
    if (!source) return undefined;
    const counts: Record<string, number> = {};
    for (const m of source) counts[m.category] = (counts[m.category] ?? 0) + 1;
    return counts;
  });

  const hitsWithoutCategory = $derived(
    effective.categories.length > 0 ? categoryCountsQ.data?.memories.length : undefined
  );

  // D-06: derived client-side by grouping the returned hits by scope, every
  // searched_scopes entry listed (zeros included) in server order; a scoped
  // (non-cross-spine) search has exactly the one scope it searched.
  const scopeHits = $derived.by(() => {
    if (classified.kind !== 'text' || !searchQ.data) return undefined;
    const counts = new Map<string, number>();
    for (const m of searchQ.data.memories) counts.set(m.scope, (counts.get(m.scope) ?? 0) + 1);
    if (effective.scope) {
      return [{ scope: effective.scope, hits: counts.get(effective.scope) ?? 0, searched: true }];
    }
    return (searchQ.data.searchedScopes ?? []).map((s) => ({ scope: s, hits: counts.get(s) ?? 0, searched: true }));
  });

  const headerParts = $derived.by((): HeaderPart[] => {
    if (classified.kind === 'id' || classified.kind === 'short_id') {
      if (shortIdMissText) {
        return [{ kind: 'text', text: resolutionLine({ kind: 'short_id_miss', q: shortIdMissText }) }];
      }
      if (idQ.data?.memory) {
        const m = idQ.data.memory;
        const resolution: ResolutionInput =
          classified.kind === 'id'
            ? { kind: 'id', id: classified.id, supersededBy: m.supersededBy || undefined }
            : { kind: 'short_id', shortId: classified.shortId };
        return [{ kind: 'text', text: resolutionLine(resolution) }];
      }
      return [];
    }
    if (classified.kind === 'text') {
      if (!searchQ.data) {
        return searchQ.isLoading
          ? [{ kind: 'text', text: loadingLine(effective.q, scopesQ.data?.scopes.length ?? 0) }]
          : [];
      }
      return rankedHeaderParts({
        hits: searchQ.data.memories.length,
        hitsWithoutCategory,
        scopeCount: effective.scope ? 1 : (searchQ.data.searchedScopes?.length ?? 0),
        query: effective.q,
        reranked: searchQ.data.memories.some((m) => m.relevance !== undefined),
        hidden: hiddenFromProto(searchQ.data.recallGateHidden),
        scopesTruncated: searchQ.data.scopesTruncated,
        scopesUnknown: searchQ.data.scopesUnknown
      });
    }
    if (classified.kind === 'operators') {
      if (!listQ.data) return [];
      return listingHeaderParts({ total: listMemories.length, scopes: listScopeCount, hiddenPages: listHiddenPages });
    }
    return [];
  });

  const headerBusy = $derived(
    classified.kind === 'operators' ? listBusy : searchQ.isFetching && searchQ.isPlaceholderData
  );

  // ENTRY-03/ENTRY-05: one honest-state pipeline that every classification
  // outcome feeds — the empty check requires isSuccess && !isFetching
  // (Pitfall 3: never flash to empty mid-flight), and an error always wins
  // over an empty read since a rejected/opaque query is never truly "zero
  // hits".
  const activeResult = $derived.by(():
    | {
        error: unknown;
        isSuccess: boolean;
        isFetching: boolean;
        count: number;
        scopeCount: number;
        hidden?: HiddenCounts;
        refetch: () => void;
      }
    | undefined => {
    if (classified.kind === 'text') {
      return {
        error: searchQ.error,
        isSuccess: searchQ.isSuccess,
        isFetching: searchQ.isFetching,
        count: searchQ.data?.memories.length ?? 0,
        scopeCount: effective.scope ? 1 : (searchQ.data?.searchedScopes?.length ?? 0),
        hidden: hiddenFromProto(searchQ.data?.recallGateHidden),
        refetch: () => searchQ.refetch()
      };
    }
    if (classified.kind === 'operators') {
      return {
        error: listQ.error,
        isSuccess: listQ.isSuccess,
        isFetching: listQ.isFetching && !listQ.isFetchingNextPage,
        count: listMemories.length,
        scopeCount: listScopeCount,
        hidden: listHiddenPages[0],
        refetch: () => listQ.refetch()
      };
    }
    if (classified.kind === 'id' || classified.kind === 'short_id') {
      if (shortIdMissText) {
        return {
          error: fallbackQ.error,
          isSuccess: fallbackQ.isSuccess,
          isFetching: fallbackQ.isFetching,
          count: fallbackQ.data?.memories.length ?? 0,
          scopeCount: effective.scope ? 1 : (fallbackQ.data?.searchedScopes?.length ?? 0),
          hidden: hiddenFromProto(fallbackQ.data?.recallGateHidden),
          refetch: () => fallbackQ.refetch()
        };
      }
      // A GetMemory lookup has no "empty success" concept — isSuccess stays
      // false here so a found record never routes through the empty branch.
      return {
        error: idQ.error,
        isSuccess: false,
        isFetching: idQ.isFetching,
        count: 0,
        scopeCount: 0,
        hidden: undefined,
        refetch: () => idQ.refetch()
      };
    }
    return undefined;
  });

  const parsedError = $derived(activeResult?.error ? parseConnectError(activeResult.error) : undefined);

  const isEmpty = $derived(
    !!activeResult && !parsedError && activeResult.isSuccess && !activeResult.isFetching && activeResult.count === 0
  );

  const emptyHeadingText = $derived.by(() => {
    if (!isEmpty || !activeResult) return '';
    return emptyHeading({
      query: effective.q,
      crossSpine: effective.crossSpine,
      scopesSearched: activeResult.scopeCount,
      hidden: activeResult.hidden
    });
  });

  const emptyFixes = $derived.by((): { id: string; label: string }[] => {
    if (!isEmpty || !activeResult) return [];
    const fixes: { id: string; label: string }[] = [];
    if (effective.scope) fixes.push({ id: 'search-all-scopes', label: 'Search every readable scope' });
    const h = activeResult.hidden;
    if (h?.archived) fixes.push({ id: 'include-archived', label: 'Include archived' });
    if (h?.superseded) fixes.push({ id: 'include-superseded', label: 'Include superseded' });
    if ((h?.scheduled ?? 0) > 0 || (h?.expired ?? 0) > 0) {
      fixes.push({ id: 'include-scheduled', label: 'Include scheduled' });
    }
    if (effective.categories.length > 0) fixes.push({ id: 'clear-category', label: 'Clear the category filter' });
    return fixes;
  });

  const recallState = $derived.by((): RecallStateInput | undefined => {
    if (parsedError) return { kind: 'error', parsed: parsedError, fixes: fixRowsFor(parsedError) };
    if (isEmpty) return { kind: 'empty', heading: emptyHeadingText, fixes: emptyFixes };
    return undefined;
  });

  function onRecallFix(id: string) {
    switch (id) {
      case 'search-all-scopes':
        navigate({ scope: '', crossSpine: true });
        return;
      case 'include-archived':
        navigate({ includeArchived: true });
        return;
      case 'include-superseded':
        navigate({ includeSuperseded: true });
        return;
      case 'include-scheduled':
        navigate({ includeScheduled: true });
        return;
      case 'clear-category':
        navigate({ categories: [] });
        return;
      case 'enable-cross-spine':
        navigate({ crossSpine: true, scope: '' });
        return;
      case 'pick-scope':
        (document.querySelector('.scope-combobox-trigger') as HTMLButtonElement | null)?.click();
        return;
      case 'lower-k': {
        const idx = K_STEPS.indexOf(effective.k);
        navigate({ k: idx > 0 ? K_STEPS[idx - 1] : DEFAULT_K });
        return;
      }
      case 'without-full':
        fullOverride = false;
        return;
      case 'clear-created':
        navigate({ createdAfter: '', createdBefore: '' });
        return;
      case 'retry':
        activeResult?.refetch();
        return;
    }
  }

  function onRecallRetry() {
    activeResult?.refetch();
  }

  let writeSurfaces: ReturnType<typeof WriteSurfaces> | undefined = $state();

  onMount(() => {
    const env = peekResume();
    if (env && env.kind === 'memory') writeSurfaces?.reopenFromResume(env);
  });
</script>

<div class="search-page">
  <div class="search-input-row">
    <input
      class="search-input"
      aria-label="Search query"
      value={inputText}
      oninput={onInput}
      placeholder="Search, paste an id, scope: #tag is:"
    />
  </div>
  <FacetStrip
    params={effective}
    {categoryCounts}
    scopes={scopesQ.data}
    scopesLoading={scopesQ.isLoading}
    scopesError={scopesQ.error}
    onchange={(partial) => navigate({ ...partial, sel: '' })}
    onretry={() => scopesQ.refetch()}
  />
  <ResultsHeader parts={headerParts} {scopeHits} k={effective.k} busy={headerBusy} />
  <!-- WriteSurfaces lives in a STABLE location outside RecallSplit: that
       component switches its narrow/wide layout branch based on a
       ResizeObserver measurement that settles a tick after mount, and both
       branches independently `{@render list()}` — Svelte tears down and
       recreates a snippet's content across an {#if}/{:else} switch, which
       would destroy and recreate WriteSurfaces (losing its bind:this
       reference and the one-shot onMount resume-restore below) the first
       time the measured width crosses the narrow breakpoint. -->
  <div class="search-toolbar">
    <WriteSurfaces
      bind:this={writeSurfaces}
      kind="memory"
      scope={effective.scope}
      onresumeapplied={consumeResume}
      ondeleted={(id) => {
        if (id === effectiveSel) navigate({ sel: '' });
      }}
    />
  </div>
  <div class="search-body">
    <RecallSplit open={!!effectiveSel} onclose={() => navigate({ sel: '' })} autoSaveId="engram-search-split">
      {#snippet list()}
        {#if recallState}
          <RecallState state={recallState} onfix={onRecallFix} onretry={onRecallRetry} />
        {:else}
          <ResultsList
            {memories}
            mode={classified.kind === 'operators' ? 'unranked' : 'ranked'}
            label="Search results"
            openId={effectiveSel}
            loading={classified.kind === 'operators' ? listLoading : searchQ.isLoading && !searchQ.data}
            busy={classified.kind === 'operators' ? listBusy : searchQ.isFetching && searchQ.isPlaceholderData}
            hasMore={classified.kind === 'operators' ? listQ.hasNextPage : false}
            onloadmore={classified.kind === 'operators' ? () => listQ.fetchNextPage() : undefined}
            onopen={(id) => navigate({ sel: params.sel === id ? '' : id })}
            onescape={() => navigate({ sel: '' })}
            onedit={(id) => writeSurfaces?.openEdit(id)}
            onvisibility={(m) =>
              normalizeVisibility(m.visibility) === 'shared'
                ? writeSurfaces?.requestMakePrivate(m, 'memory')
                : writeSurfaces?.requestShare(m, 'memory')}
            ondelete={(id) => writeSurfaces?.requestDelete(id, 'memory')}
          />
          {#if classified.kind === 'text' && memories.length === effective.k && nextK(effective.k) !== undefined}
            <div class="show-more-row">
              <Button variant="outline" size="sm" onclick={() => navigate({ k: nextK(effective.k) })}>Show more</Button>
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
          hit={selectedMemory ? { score: selectedMemory.score, relevance: selectedMemory.relevance } : undefined}
          onclose={() => navigate({ sel: '' })}
          onselect={(id) => navigate({ sel: id })}
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
</div>

<style>
  .search-page {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .search-input-row {
    flex: none;
    padding: calc(8 * var(--u)) calc(14 * var(--u));
    border-bottom: 1px solid var(--border-subtle, var(--border));
  }
  .search-input {
    width: 100%;
    height: calc(32 * var(--u));
    border: 1px solid var(--input, var(--border));
    border-radius: calc(6 * var(--u));
    background: var(--background);
    padding: 0 calc(10 * var(--u));
    font-size: calc(13 * var(--u));
  }
  .search-input:focus-visible {
    outline: none;
    border-color: var(--primary);
    box-shadow: 0 0 0 calc(3 * var(--u)) var(--primary-soft, color-mix(in srgb, var(--primary) 20%, transparent));
  }
  .search-toolbar {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    padding: calc(6 * var(--u)) calc(12 * var(--u));
    border-bottom: 1px solid var(--border-subtle, var(--border));
    flex: none;
  }
  .search-body {
    flex: 1;
    min-height: 0;
  }
  .show-more-row {
    display: flex;
    justify-content: center;
    padding: calc(10 * var(--u)) 0;
    flex: none;
  }
</style>
