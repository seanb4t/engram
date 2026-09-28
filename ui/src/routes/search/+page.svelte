<script lang="ts">
  // Rebuilt as the recall surface (ENTRY-01/02/06, SC4): URL-driven, race-safe
  // queries (every query passes `{ signal }` and keys on the full normalized
  // params), the honest results header, and the shared ResultsList/RecallSplit/
  // DetailPane set from plans 02-06/02-07.
  import { onMount, onDestroy, untrack } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { ConnectError, Code } from '@connectrpc/connect';
  import { createQuery, createInfiniteQuery, keepPreviousData } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { peekResume, consumeResume, normalizeReturnPath } from '$lib/resume';
  import { normalizeVisibility } from '$lib/mutations/memory';
  import { flashRows } from '$lib/curation/flash.svelte';
  import { registerCurationHost, defaultActionsFor } from '$lib/curation/host.svelte';
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
    understandEligible,
    understandQueryKey,
    understandQueryRequest,
    visibleSuggestions,
    suggestionAnnouncement
  } from '$lib/search/understand';
  import { relatedPath } from '$lib/search/related-params';
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
  import SuggestedRow from '$lib/components/SuggestedRow.svelte';
  import ResultsList from '$lib/components/ResultsList.svelte';
  import RecallSplit from '$lib/components/RecallSplit.svelte';
  import DetailPane from '$lib/components/DetailPane.svelte';
  import TagBars from '$lib/components/TagBars.svelte';
  import WriteSurfaces from '$lib/components/WriteSurfaces.svelte';
  import CurationSurfaces from '$lib/components/CurationSurfaces.svelte';
  import RecallState, { type RecallStateInput } from '$lib/components/RecallState.svelte';
  import { Button } from '$lib/components/ui/button';
  import * as Sheet from '$lib/components/ui/sheet';
  import { scopeLabel } from '$lib/tags/tags';
  import XIcon from '@lucide/svelte/icons/x';

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

  // TAGS-01/D-13: the docked Tags panel's open state is page-local, not URL
  // state (a shared /search?... link keeps its tag filters, not the panel).
  let tagsPanelOpen = $state(false);
  // Mirrors RecallSplit's own narrow/wide breakpoint (bound below) -- reused
  // rather than a second width measurement on the page (Task 3).
  let splitNarrow = $state(false);

  // A bar click in the panel toggles that tag's URL facet chip (params.tags)
  // — an inline #tag token typed into the query is still shown as active via
  // `effective.tags` but is removed by editing the query, not by the bar.
  function toggleTag(tag: string) {
    navigate({ tags: params.tags.includes(tag) ? params.tags.filter((t) => t !== tag) : [...params.tags, tag], sel: '' });
  }

  // The page's own query input (debounced 160ms into the URL, replaceState so
  // typing doesn't spam browser history). `searchInputEl` also receives
  // focus when a Suggested-row accept/dismiss empties the last chip
  // (a11y gap-fill, not in D-13 — focus must never fall back to <body>).
  let searchInputEl: HTMLInputElement | undefined = $state();
  let inputText = $state(untrack(() => params.q));
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
  // WR-05 fix: every other timer/listener in this file cleans up on
  // teardown (onScroll, onTextSize, etc.) — this debounce was the one
  // exception, leaving a pending goto() to fire against a torn-down route.
  onDestroy(() => {
    if (debounceTimer) clearTimeout(debounceTimer);
  });

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

  // NLQ-03/D-04/D-10: the Suggested row's UnderstandQuery. Keyed by bare q
  // (never `effective` — the query-key pitfall note in 06-UI-SPEC.md), no
  // placeholderData (a previous q's chips must never show for a new q), and
  // latched off for the rest of the page session once a response reports
  // enabled: false.
  let understandingOff = $state(false);
  const understandQ = createQuery(() => ({
    queryKey: understandQueryKey(params.q),
    queryFn: ({ signal }) =>
      engram.understandQuery(
        understandQueryRequest(classified.kind === 'text' ? classified.text : '', effective),
        { signal }
      ),
    enabled: understandEligible(classified) && !understandingOff,
    staleTime: Infinity,
    meta: { silent: true }
  }));
  $effect(() => {
    if (understandQ.data?.enabled === false) understandingOff = true;
  });

  // D-12: dismissals are held per-q in memory only — never the URL, never
  // web storage — and forgotten whenever q changes, including a round trip
  // back to a q seen before (the effect keeps `dismissed.q` synced to the
  // live q the instant it changes, so returning to an old q value never
  // resurrects a stale dismissal that happened to share that same q).
  let dismissed = $state<{ q: string; keys: string[] }>({ q: untrack(() => params.q), keys: [] });
  $effect(() => {
    if (params.q !== dismissed.q) dismissed = { q: params.q, keys: [] };
  });
  const dismissedKeys = $derived(new Set(dismissed.q === params.q ? dismissed.keys : []));
  function dismissSuggestion(key: string) {
    dismissed = { q: params.q, keys: [...dismissed.keys, key] };
  }

  const visibleSuggested = $derived(
    understandEligible(classified) && understandQ.data?.enabled
      ? visibleSuggestions(understandQ.data.suggestions, effective, dismissedKeys)
      : []
  );

  // D-13/06-UI-SPEC "Row appearance announcement": fires once per NEW
  // UnderstandQuery response, never on a later accept/dismiss re-render of
  // the SAME response — the effect's only tracked dependency is
  // `understandQ.data` itself (everything else is read inside `untrack`),
  // so toggling a facet chip or dismissing a suggestion never re-announces.
  let suggestedAnnouncement = $state('');
  $effect(() => {
    const data = understandQ.data;
    if (data === undefined) return;
    untrack(() => {
      const n = understandEligible(classified) && data.enabled
        ? visibleSuggestions(data.suggestions, effective, dismissedKeys).length
        : 0;
      suggestedAnnouncement = n > 0 ? suggestionAnnouncement(n) : '';
    });
  });

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
  //
  // WR-02 fix: `params.sel` is the ONLY signal `navigate({ sel: '' })` can
  // produce, and it is always falsy once cleared — so a bare `params.sel ||
  // autoOpenId` fallback reopens the pane on every close click as long as
  // the query is still classified id/short_id and idQ.data stays loaded.
  // `dismissedAutoOpen` is the explicit "user closed it" flag the fallback
  // needs to respect; it resets whenever the resolved target itself changes
  // (a fresh id/short_id lookup should still auto-open once).
  let dismissedAutoOpen = $state(false);
  $effect(() => {
    idToFetch;
    dismissedAutoOpen = false;
  });

  const autoOpenId = $derived(
    (classified.kind === 'id' || classified.kind === 'short_id') &&
      idQ.data?.memory &&
      !shortIdMissText &&
      !dismissedAutoOpen
      ? idQ.data.memory.id
      : ''
  );
  const effectiveSel = $derived(params.sel || autoOpenId);
  const inResults = $derived(memories.some((m) => m.id === effectiveSel));

  // Every close affordance (RecallSplit/DetailPane close button, ResultsList
  // onescape, a second Enter on the open row) routes through here so the
  // dismissal is recorded before clearing `sel` — see the WR-02 note above.
  function closeSel() {
    dismissedAutoOpen = true;
    navigate({ sel: '' });
  }

  function toggleOpen(id: string) {
    // Compares against effectiveSel (not params.sel) so a second Enter/click
    // on an auto-opened id/short_id row is recognized as "close the open
    // row" rather than a redundant "open it again" (part of the WR-02 fix
    // above: openId={effectiveSel} is what ResultsList renders as open).
    if (effectiveSel === id) {
      closeSel();
    } else {
      navigate({ sel: id });
    }
  }

  // D-03/D-04: navigate to the record's related view, carrying where it was
  // opened from so the related view (and, later, its own Escape) can return
  // here.
  function openRelated(id: string) {
    goto(
      `${base}${relatedPath(id, { from: normalizeReturnPath(page.url.pathname + page.url.search), trail: [] })}`
    );
  }

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
  let curation: ReturnType<typeof CurationSurfaces> | undefined = $state();

  // D-01..D-04: the multi-select bulk-curation state lives at the route
  // level so both ResultsList (keyboard/pointer selection) and
  // ResultsHeader's bulk bar (Task 3) share it.
  let selectedIds = $state<string[]>([]);

  // E4: while a curation commit is in flight, disable the bulk bar's verb
  // buttons -- CurationSurfaces fires this around every archive/restore/
  // supersede COMMIT (never the supersede validate_only preview).
  let curationBusy = $state(false);

  onMount(() => {
    const env = peekResume();
    if (env && env.kind === 'memory') writeSurfaces?.reopenFromResume(env);
    else if (env && (env.kind === 'supersede' || env.kind === 'archive')) curation?.reopenFromResume(env);

    // Phase 2 D-11: registers /search as the active curation host for ⌘K's
    // record-group actions -- unregistered on teardown so a stale
    // registration can never survive a route switch.
    const unregister = registerCurationHost({
      actionsFor: defaultActionsFor,
      run: (action, ids) =>
        action === 'supersede'
          ? curation?.openSupersede(ids)
          : action === 'archive'
            ? curation?.openArchive(ids)
            : action === 'restore'
              ? curation?.openRestore(ids)
              : curation?.openChain(ids[0])
    });
    return unregister;
  });
</script>

<div class="search-page">
  <div class="search-input-row">
    <input
      class="search-input"
      aria-label="Search query"
      bind:this={searchInputEl}
      value={inputText}
      oninput={onInput}
      placeholder="Search, paste an id, scope: #tag is:"
    />
  </div>
  <p class="sr-only" aria-live="polite">{suggestedAnnouncement}</p>
  <FacetStrip
    params={effective}
    {categoryCounts}
    scopes={scopesQ.data}
    scopesLoading={scopesQ.isLoading}
    scopesError={scopesQ.error}
    onchange={(partial) => navigate({ ...partial, sel: '' })}
    onretry={() => scopesQ.refetch()}
    {tagsPanelOpen}
    ontagspanel={() => (tagsPanelOpen = !tagsPanelOpen)}
  />
  {#if visibleSuggested.length > 0}
    <SuggestedRow
      suggestions={visibleSuggested}
      params={effective}
      onchange={(partial) => navigate({ ...partial, sel: '' })}
      ondismiss={dismissSuggestion}
      onempty={() => searchInputEl?.focus()}
    />
  {/if}
  <ResultsHeader
    parts={headerParts}
    {scopeHits}
    k={effective.k}
    busy={headerBusy}
    selection={{
      count: selectedIds.length,
      busy: curationBusy,
      onsupersede: () => curation?.openSupersede(selectedIds),
      onarchive: () => curation?.openArchive(selectedIds),
      onrestore: () => curation?.openRestore(selectedIds),
      onclear: () => (selectedIds = [])
    }}
  />
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
    <CurationSurfaces
      bind:this={curation}
      returnPath={normalizeReturnPath(page.url.pathname + page.url.search)}
      onchanged={() => (selectedIds = [])}
      onviewsuperseded={(ids) => {
        navigate({ includeSuperseded: true });
        flashRows(ids);
      }}
      onopenrecord={(id) => navigate({ sel: id })}
      onresumeapplied={consumeResume}
      onbusychange={(b) => (curationBusy = b)}
    />
  </div>
  <div class="search-body">
    <RecallSplit
      open={!!effectiveSel || (tagsPanelOpen && !splitNarrow)}
      onclose={() => (effectiveSel ? closeSel() : (tagsPanelOpen = false))}
      autoSaveId="engram-search-split"
      bind:narrow={splitNarrow}
    >
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
            onopen={toggleOpen}
            onescape={closeSel}
            onedit={(id) => writeSurfaces?.openEdit(id)}
            onvisibility={(m) =>
              normalizeVisibility(m.visibility) === 'shared'
                ? writeSurfaces?.requestMakePrivate(m, 'memory')
                : writeSurfaces?.requestShare(m, 'memory')}
            ondelete={(id) => writeSurfaces?.requestDelete(id, 'memory')}
            selectable
            bind:selectedIds
            selectionKey={encodeSearchParams({ ...params, k: DEFAULT_K, sel: '' })}
            onsupersede={(ids) => curation?.openSupersede(ids)}
            onarchive={(ids) => curation?.openArchive(ids)}
            onrestore={(ids) => curation?.openRestore(ids)}
            onchain={(id) => curation?.openChain(id)}
            onrelated={openRelated}
          />
          {#if classified.kind === 'text' && memories.length === effective.k && nextK(effective.k) !== undefined}
            <div class="show-more-row">
              <Button variant="outline" size="sm" onclick={() => navigate({ k: nextK(effective.k) })}>Show more</Button>
            </div>
          {/if}
        {/if}
      {/snippet}
      {#snippet detail()}
        {#if effectiveSel}
          <DetailPane
            memory={selectedMemory}
            loading={detailQ.isLoading}
            error={detailQ.error}
            requestedId={effectiveSel}
            {inResults}
            hit={selectedMemory ? { score: selectedMemory.score, relevance: selectedMemory.relevance } : undefined}
            onclose={closeSel}
            onselect={(id) => navigate({ sel: id })}
            onedit={(id) => writeSurfaces?.openEdit(id)}
            onvisibility={(m) =>
              normalizeVisibility(m.visibility) === 'shared'
                ? writeSurfaces?.requestMakePrivate(m, 'memory')
                : writeSurfaces?.requestShare(m, 'memory')}
            ondelete={(id) => writeSurfaces?.requestDelete(id, 'memory')}
            onsupersede={(id) => curation?.openSupersede([id])}
            onarchive={(id) => curation?.openArchive([id])}
            onrestore={(id) => curation?.openRestore([id])}
            onchain={(id) => curation?.openChain(id)}
            onrelated={openRelated}
          />
        {:else if tagsPanelOpen}
          <div class="tags-panel-slot">
            <div class="tags-panel-slot-head">
              <Button variant="ghost" size="icon-sm" aria-label="close" onclick={() => (tagsPanelOpen = false)}><XIcon /></Button>
            </div>
            <TagBars
              mode="panel"
              scope={effective.scope}
              markedTags={new Set(effective.tags)}
              selectedTags={new Set(effective.tags)}
              ontoggle={toggleTag}
            />
          </div>
        {/if}
      {/snippet}
    </RecallSplit>
  </div>

  <!-- D-13: below RecallSplit's own narrow breakpoint the panel renders as a
       bottom sheet instead of sharing the slot; closing the sheet turns the
       toggle off, mirroring the slot's own close control above. -->
  {#if tagsPanelOpen && splitNarrow && !effectiveSel}
    <Sheet.Root open onOpenChange={(v) => { if (!v) tagsPanelOpen = false; }}>
      <Sheet.Content side="bottom" class="h-[62vh]">
        <Sheet.Header>
          <!-- sr-only: TagBars renders this exact text as its own visible
               .header line below -- the Title exists only to give the sheet
               its required accessible name (D-13: "the panel header as its
               title"), not to duplicate it visually. -->
          <Sheet.Title class="sr-only">Tags · counts in {scopeLabel(effective.scope)}</Sheet.Title>
        </Sheet.Header>
        <div class="tags-sheet-body">
          <TagBars
            mode="panel"
            scope={effective.scope}
            markedTags={new Set(effective.tags)}
            selectedTags={new Set(effective.tags)}
            ontoggle={toggleTag}
          />
        </div>
      </Sheet.Content>
    </Sheet.Root>
  {/if}
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
  .tags-panel-slot {
    height: 100%;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .tags-panel-slot-head {
    display: flex;
    justify-content: flex-end;
    padding: calc(6 * var(--u));
    flex: none;
  }
  .tags-panel-slot :global(.tag-bars) {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0 calc(10 * var(--u)) calc(10 * var(--u));
  }
  .tags-sheet-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0 calc(14 * var(--u)) calc(14 * var(--u));
  }
</style>
