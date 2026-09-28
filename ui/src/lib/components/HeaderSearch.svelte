<script lang="ts">
  import { Command as CommandPrimitive } from 'bits-ui';
  import { createQuery, keepPreviousData } from '@tanstack/svelte-query';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import * as Popover from '$lib/components/ui/popover';
  import * as Command from '$lib/components/ui/command';
  import { Kbd } from '$lib/components/ui/kbd';
  import { Badge } from '$lib/components/ui/badge';
  import ScopeChip from './ScopeChip.svelte';
  import TagMatchRow from './TagMatchRow.svelte';
  import SearchIcon from '@lucide/svelte/icons/search';
  import ScrollTextIcon from '@lucide/svelte/icons/scroll-text';
  import CalendarClockIcon from '@lucide/svelte/icons/calendar-clock';
  import CompassIcon from '@lucide/svelte/icons/compass';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { engram } from '$lib/client';
  import { classifyInput, KNOWN_CATEGORIES, type OperatorChip } from '$lib/search/classify';
  import { relativeTime } from '$lib/time';
  import { parseConnectError, fixRowsFor, type FixRow } from '$lib/errors/connect-error';
  import { defaultSearchParams, encodeSearchParams, applyChips } from '$lib/search/params';
  import { headerSearch } from '$lib/search/header-search.svelte';
  import { listTagsQuery } from '$lib/tags/query';
  import { toTagRows, rankTagMatches, scopeLabel, matchFooter, tagsLoadingLine, tagsErrorCopy } from '$lib/tags/tags';

  type ScopeChipT = Extract<OperatorChip, { kind: 'scope' }>;
  type TagChipT = Extract<OperatorChip, { kind: 'tag' }>;
  type CategoryChipT = Extract<OperatorChip, { kind: 'category' }>;
  type PendingChipT = Extract<OperatorChip, { kind: 'pending' }>;
  const isScopeChip = (c: OperatorChip): c is ScopeChipT => c.kind === 'scope';
  const isTagChip = (c: OperatorChip): c is TagChipT => c.kind === 'tag';
  const isCategoryChip = (c: OperatorChip): c is CategoryChipT => c.kind === 'category';
  const isPendingChip = (c: OperatorChip): c is PendingChipT => c.kind === 'pending';

  // The header search box (D-11): one shared classifier decides which RPC
  // runs, and the dropdown is server-driven (ENTRY-01/02/04) — never a
  // client-filtered palette (see recall-surface.md "What to Avoid").

  // WCAG 4.1.2 (aria-required-attr): CommandPrimitive.Input sets
  // role="combobox"/aria-expanded, but its aria-controls only populates from
  // an (unused here) Command.Viewport -- an explicit id/aria-controls pair
  // closes the gap without adopting Viewport (mergeProps keeps our
  // aria-controls since bits-ui's own value is undefined without one).
  const uid = $props.id();
  const listId = `${uid}-list`;

  let debouncedText = $state('');
  let open = $state(false);
  let inputWrapperEl = $state<HTMLElement | null>(null);
  let inputEl = $state<HTMLInputElement | null>(null);
  let crossSpineOff = $state(false);
  let sourceOpen = $state(false);

  // 70ms debounce (ENTRY-06): re-derive debouncedText from headerSearch.text
  // after the caller stops typing; the effect's own cleanup cancels a stale
  // timer. headerSearch.text (not a local $state) is the ONE hand-off point
  // the ⌘K command menu (Phase 3, 02-05) writes through (D-11).
  $effect(() => {
    const current = headerSearch.text;
    const timer = setTimeout(() => {
      debouncedText = current;
    }, 70);
    return () => clearTimeout(timer);
  });

  // handoffToHeaderSearch/focusHeaderSearch bump focusSeq; react by opening
  // the dropdown and focusing the input. Skip the effect's own first run so
  // mounting the component does not steal focus unprompted.
  let sawFirstFocusSeq = false;
  $effect(() => {
    headerSearch.focusSeq;
    if (!sawFirstFocusSeq) {
      sawFirstFocusSeq = true;
      return;
    }
    open = true;
    inputEl?.focus();
  });

  function onGlobalKeydown(e: KeyboardEvent) {
    const target = e.target as HTMLElement | null;
    const tag = target?.tagName;
    const isEditable =
      tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target?.isContentEditable === true;
    if (e.key === '/' && !isEditable && !e.metaKey && !e.ctrlKey && !e.altKey) {
      e.preventDefault();
      open = true;
      inputEl?.focus();
    } else if (e.key === 'Escape' && open) {
      open = false;
      inputEl?.blur();
    }
  }

  const classified = $derived(classifyInput(debouncedText));
  const queryText = $derived(classified.kind === 'text' ? classified.text : '');
  const idToFetch = $derived(
    classified.kind === 'id' ? classified.id : classified.kind === 'short_id' ? classified.shortId : ''
  );

  // Chips (ENTRY-02, D-04 carried-forward "never silently reinterpret" rule).
  const activeChips = $derived(
    classified.kind === 'text' || classified.kind === 'operators' ? classified.chips : []
  );
  const scopeChip = $derived(activeChips.find(isScopeChip));
  const tagChips = $derived(activeChips.filter(isTagChip));
  const categoryChips = $derived(activeChips.filter(isCategoryChip));
  const pendingChip = $derived(activeChips.find(isPendingChip));

  // A scope chip forces crossSpine off (last scope chip wins if more than
  // one is present); otherwise crossSpine follows the user's own toggle.
  const effectiveScope = $derived(scopeChip ? scopeChip.value : '');
  const effectiveCrossSpine = $derived(effectiveScope ? false : !crossSpineOff);
  const effectiveTags = $derived(tagChips.map((c) => c.value));
  const effectiveCategories = $derived(categoryChips.filter((c) => c.known).map((c) => c.value));

  const searchQuery = createQuery(() => ({
    queryKey: ['headerSearch', queryText, effectiveScope, effectiveCrossSpine, effectiveTags, effectiveCategories],
    queryFn: ({ signal }) =>
      engram.searchMemories(
        {
          query: queryText,
          scope: effectiveScope,
          crossSpine: effectiveCrossSpine,
          tags: effectiveTags,
          categories: effectiveCategories,
          k: 50n,
          full: false
        },
        { signal }
      ),
    enabled: classified.kind === 'text',
    placeholderData: keepPreviousData,
    meta: { silent: true }
  }));

  const listQuery = createQuery(() => ({
    queryKey: ['headerSearchList', effectiveScope, effectiveCrossSpine, effectiveTags, effectiveCategories],
    queryFn: ({ signal }) =>
      engram.listMemories(
        {
          scope: effectiveScope,
          crossSpine: effectiveCrossSpine,
          tags: effectiveTags,
          categories: effectiveCategories,
          limit: 5n,
          cursorMode: true
        },
        { signal }
      ),
    enabled: classified.kind === 'operators',
    placeholderData: keepPreviousData,
    meta: { silent: true }
  }));

  const getQuery = createQuery(() => ({
    queryKey: ['getMemory', idToFetch],
    queryFn: ({ signal }) => engram.getMemory({ id: idToFetch }, { signal }),
    enabled: classified.kind === 'id' || classified.kind === 'short_id',
    meta: { silent: true }
  }));

  const getError = $derived(getQuery.error ? parseConnectError(getQuery.error) : null);
  const searchError = $derived(searchQuery.error ? parseConnectError(searchQuery.error) : null);

  // WR-04 fix: `k` (50n) and `full` (false) are hardcoded for every
  // searchQuery call in this component — there is no local state for
  // 'lower-k'/'without-full'/'clear-created' to mutate, so applyFixRow
  // collapses all three to a plain retry that fails again with the same
  // error. Filter them out of the rendered rows rather than show an
  // action-labelled button that silently does nothing different from
  // "Retry"; `fixRowsFor` always appends a plain retry row last, so the
  // generic fallback survives this filter untouched.
  const NOOP_FIX_ROW_IDS = new Set(['lower-k', 'without-full', 'clear-created']);
  const visibleFixRows = $derived(
    searchError ? fixRowsFor(searchError).filter((row) => !NOOP_FIX_ROW_IDS.has(row.id)) : []
  );

  // A short_id-shaped input GetMemory reports not-found is re-searched as
  // text — never a silent reinterpretation (carried-forward rule).
  const shortIdFallbackText = $derived(
    classified.kind === 'short_id' && getError?.kind === 'not-found' ? classified.shortId : ''
  );

  const fallbackSearchQuery = createQuery(() => ({
    queryKey: ['headerSearchFallback', shortIdFallbackText],
    queryFn: ({ signal }) =>
      engram.searchMemories(
        { query: shortIdFallbackText, scope: '', crossSpine: true, tags: [], categories: [], k: 50n, full: false },
        { signal }
      ),
    enabled: !!shortIdFallbackText,
    meta: { silent: true }
  }));

  const hitCount = $derived(searchQuery.data?.memories.length ?? 0);
  const memories = $derived(
    classified.kind === 'operators'
      ? (listQuery.data?.memories ?? [])
      : shortIdFallbackText
        ? (fallbackSearchQuery.data?.memories ?? [])
        : (searchQuery.data?.memories ?? (getQuery.data?.memory ? [getQuery.data.memory] : []))
  );
  const shownMemories = $derived(memories.slice(0, 5));
  const moreCount = $derived(Math.max(0, memories.length - 5));

  // Honest status line (D-01..D-06): hit/scope coverage, recall-gate hidden
  // count, previous-results-while-refetching, and a per-scope source button.
  const searchedScopesCount = $derived(searchQuery.data?.searchedScopes?.length ?? null);
  const hiddenNote = $derived.by(() => {
    const h = searchQuery.data?.recallGateHidden;
    if (!h || h.total === 0n) return '';
    const parts: [bigint, string][] = [
      [h.archived, 'archived'],
      [h.superseded, 'superseded'],
      [h.expired, 'expired'],
      [h.scheduled, 'scheduled']
    ];
    const nonzero = parts.filter(([n]) => n > 0n);
    if (nonzero.length === 1) {
      const [n, word] = nonzero[0];
      return `+${h.total} ${word} ${n === 1n ? 'match' : 'matches'} hidden by the recall gate (fetch by id)`;
    }
    return `+${h.total} matches hidden by the recall gate`;
  });
  const sourceLabel = $derived(
    `SearchMemories · ${effectiveScope ? `scope ${effectiveScope}` : 'cross_spine'} · ${searchedScopesCount ?? 0} scopes searched`
  );
  const perScopeCoverage = $derived.by(() => {
    const scopes = searchQuery.data?.searchedScopes ?? [];
    const counts = new Map<string, number>();
    for (const m of searchQuery.data?.memories ?? []) counts.set(m.scope, (counts.get(m.scope) ?? 0) + 1);
    return scopes.map((s) => ({ scope: s, count: counts.get(s) ?? 0 }));
  });

  // Scope/category token completion (moves the relevant group to the top
  // while a `scope:`/`in:`/`is:` token is unfinished — ENTRY-04 "populated").
  const scopePrefixInProgress = $derived.by(() => {
    if (!pendingChip) return null;
    if (pendingChip.raw.startsWith('scope:')) return pendingChip.raw.slice('scope:'.length);
    if (pendingChip.raw.startsWith('in:')) return pendingChip.raw.slice('in:'.length);
    return null;
  });
  const categoryPrefixInProgress = $derived.by(() => {
    if (!pendingChip) return null;
    if (pendingChip.raw.startsWith('is:')) return pendingChip.raw.slice('is:'.length);
    return null;
  });

  // Tags autocomplete group (TAGS-02, D-16..D-19): detected from the raw last
  // token directly, NOT from classify.ts's chip kind — '#qd' already forms a
  // complete { kind: 'tag' } chip (its value is non-empty), but the user is
  // still typing more characters for it, so the group must stay keyed off
  // whether the last token is an unfinished '#'/'tag:' prefix, not whether
  // classify.ts considers it "pending".
  const tagPrefixInProgress = $derived.by(() => {
    if (!debouncedText || /\s$/.test(debouncedText)) return null;
    const tokens = debouncedText.trim().split(/\s+/);
    const last = tokens[tokens.length - 1];
    if (last.startsWith('#')) return last.slice(1);
    if (last.toLowerCase().startsWith('tag:')) return last.slice('tag:'.length);
    return null;
  });

  // Shares the identical cached ListTags(scope, 1000) query key TagBars and
  // TagCombobox use for the same scope key (D-16 "one fetch, two surfaces"
  // extended to a third) — only enabled while a tag token is in progress.
  const tagsQuery = createQuery(() => ({
    ...listTagsQuery(effectiveScope),
    enabled: tagPrefixInProgress !== null
  }));
  const tagRows = $derived(toTagRows(tagsQuery.data));
  const tagMax = $derived(tagRows.length > 0 ? tagRows[0].count : 0);
  const tagRank = $derived(
    rankTagMatches(tagRows, tagPrefixInProgress ?? '', { more: tagsQuery.data?.more ?? false })
  );

  const listScopesQuery = createQuery(() => ({
    queryKey: ['listScopes'],
    queryFn: ({ signal }) => engram.listScopes({}, { signal }),
    enabled: classified.kind === 'text' || classified.kind === 'operators',
    meta: { silent: true }
  }));

  const filteredScopes = $derived.by(() => {
    const all = listScopesQuery.data?.scopes ?? [];
    if (scopePrefixInProgress === null) return all.slice(0, 8);
    return all.filter((s) => s.scope.toLowerCase().includes(scopePrefixInProgress.toLowerCase()));
  });
  const filteredCategories = $derived.by(() => {
    if (categoryPrefixInProgress === null) return KNOWN_CATEGORIES;
    return KNOWN_CATEGORIES.filter((c) => c.toLowerCase().includes(categoryPrefixInProgress.toLowerCase()));
  });

  const commandItems = [
    { label: 'Rules', href: `${base}/rules`, icon: ScrollTextIcon },
    { label: 'Scheduled', href: `${base}/scheduled`, icon: CalendarClockIcon },
    { label: 'Search', href: `${base}/search`, icon: SearchIcon },
    { label: 'Discovery', href: `${base}/discovery`, icon: CompassIcon }
  ];
  const visibleCommands = $derived(
    debouncedText.trim()
      ? commandItems.filter((c) => c.label.toLowerCase().includes(debouncedText.trim().toLowerCase()))
      : commandItems
  );

  // Enter on the top row hands the classified query and chips to /search
  // through the one shared codec (D-10, ENTRY-06).
  function runTopSearch() {
    if (classified.kind !== 'text') return;
    const params = applyChips({ ...defaultSearchParams(), q: classified.text }, activeChips);
    goto(`${base}/search?${encodeSearchParams(params)}`);
    headerSearch.text = '';
    debouncedText = '';
    open = false;
  }

  function runTopList() {
    const params = applyChips({ ...defaultSearchParams() }, activeChips);
    goto(`${base}/search?${encodeSearchParams(params)}`);
    headerSearch.text = '';
    debouncedText = '';
    open = false;
  }

  // A memory row carries the same params as the top row plus `sel`; an
  // id/short_id row's `q` is the raw pasted input (no chips to apply).
  function openMemory(id: string) {
    const q = classified.kind === 'text' ? classified.text : debouncedText;
    const params = applyChips({ ...defaultSearchParams(), q, sel: id }, activeChips);
    goto(`${base}/search?${encodeSearchParams(params)}`);
    headerSearch.text = '';
    open = false;
  }

  function goCommand(href: string) {
    goto(href);
    open = false;
  }

  // Removing a chip's token from the raw input. Scope/tag/category tokens
  // are reconstructed from their canonical prefix; a chip's own `✕` removes
  // exactly its own token, never the others.
  function removeChip(chip: OperatorChip) {
    const patterns: RegExp[] =
      chip.kind === 'scope'
        ? [new RegExp(`^(scope|in):${escapeReg(chip.value)}$`)]
        : chip.kind === 'tag'
          ? [new RegExp(`^(#|tag:)${escapeReg(chip.value)}$`)]
          : chip.kind === 'category'
            ? [new RegExp(`^is:${escapeReg(chip.value)}$`)]
            : [new RegExp(`^${escapeReg(chip.raw)}$`)];
    headerSearch.text = headerSearch.text
      .split(/\s+/)
      .filter((t) => !patterns.some((re) => re.test(t)))
      .join(' ');
  }

  function escapeReg(s: string): string {
    return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  function completeToken(newToken: string) {
    const raw = headerSearch.text.trim();
    const tokens = raw.length ? raw.split(/\s+/) : [];
    // A tag token in progress ('#qd', 'tag:q') already forms a complete chip
    // per classify.ts (its value is non-empty), so pendingChip alone would
    // miss it — completeToken must still replace the whole unfinished last
    // token, not append a duplicate one (D-16..D-19: "replaces the partial
    // token").
    if ((pendingChip || tagPrefixInProgress !== null) && tokens.length > 0) tokens[tokens.length - 1] = newToken;
    else tokens.push(newToken);
    headerSearch.text = `${tokens.join(' ')} `;
  }

  function applyFixRow(row: FixRow) {
    switch (row.id) {
      case 'enable-cross-spine':
        crossSpineOff = false;
        break;
      case 'pick-scope':
        headerSearch.text = headerSearch.text.trim() ? `${headerSearch.text.trim()} scope:` : 'scope:';
        break;
      case 'lower-k':
      case 'without-full':
      case 'clear-created':
      case 'retry':
        searchQuery.refetch();
        break;
    }
  }
</script>

<svelte:window onkeydown={onGlobalKeydown} />

<div class="hd-search relative mx-auto w-full max-w-[calc(560*var(--u))]">
  <CommandPrimitive.Root shouldFilter={false} class="contents">
    <div
      bind:this={inputWrapperEl}
      class="hd-input flex h-9 items-center gap-2 rounded-md border border-input bg-background px-2 focus-within:border-primary focus-within:ring-[calc(3*var(--u))] focus-within:ring-primary/20"
    >
      <SearchIcon class="size-4 shrink-0 opacity-50" />
      <CommandPrimitive.Input
        bind:value={headerSearch.text}
        bind:ref={inputEl}
        aria-label="Search memories"
        aria-controls={listId}
        placeholder="Search, paste an id, scope: #tag is:"
        class="flex-1 bg-transparent text-sm outline-none"
        onfocus={() => (open = true)}
      />
      <Kbd class="shrink-0">/</Kbd>
    </div>
    <Popover.Root bind:open>
      <Popover.Content
        customAnchor={inputWrapperEl}
        class="w-[var(--bits-popover-anchor-width)] max-h-[calc(540*var(--u))] overflow-hidden p-0"
        onOpenAutoFocus={(e) => e.preventDefault()}
        onCloseAutoFocus={(e) => e.preventDefault()}
        portalProps={{ disabled: true }}
      >
        {#if classified.kind === 'text' || classified.kind === 'operators'}
          <div
            class="chips flex flex-wrap items-center gap-1 border-b border-border px-2 py-1.5 text-xs"
            data-testid="header-search-chips"
          >
            <span class="text-muted-foreground">interpreted as</span>
            {#if classified.kind === 'text'}
              <span class="chip rounded-full border border-primary bg-primary/10 px-2 py-0.5">
                text &quot;{classified.text}&quot;
              </span>
            {/if}
            {#each activeChips as chip}
              {#if chip.kind === 'scope'}
                <span class="chip rounded-full border border-border px-2 py-0.5">
                  scope:{chip.value}
                  <button type="button" aria-label={`remove scope:${chip.value}`} onclick={() => removeChip(chip)}>✕</button>
                </span>
              {:else if chip.kind === 'tag'}
                <span class="chip rounded-full border border-border px-2 py-0.5">
                  #{chip.value}
                  <button type="button" aria-label={`remove #${chip.value}`} onclick={() => removeChip(chip)}>✕</button>
                </span>
              {:else if chip.kind === 'category'}
                <span
                  data-chip="category"
                  data-known={String(chip.known)}
                  class={'chip rounded-full border px-2 py-0.5 ' +
                    (chip.known ? 'border-border' : 'border-destructive text-destructive')}
                >
                  is:{chip.value}
                  <button type="button" aria-label={`remove is:${chip.value}`} onclick={() => removeChip(chip)}>✕</button>
                </span>
              {:else}
                <span class="chip rounded-full border border-dashed border-border px-2 py-0.5 text-muted-foreground">
                  {chip.raw}
                </span>
              {/if}
            {/each}
            {#if !effectiveScope}
              {#if !crossSpineOff}
                <span class="chip rounded-full border border-border px-2 py-0.5">
                  cross-spine
                  <button type="button" aria-label="remove cross-spine" onclick={() => (crossSpineOff = true)}>✕</button>
                </span>
              {:else}
                <button
                  type="button"
                  class="chip rounded-full border border-dashed border-border px-2 py-0.5 text-muted-foreground"
                  onclick={() => (crossSpineOff = false)}
                >
                  + cross-spine
                </button>
              {/if}
            {/if}
          </div>
        {/if}

        {#snippet tagsGroup()}
          <Command.Group heading={'Tags · counts in ' + scopeLabel(effectiveScope)}>
            {#if tagsQuery.isPending}
              <div class="px-2 py-1.5 text-xs font-mono text-muted-foreground">
                {tagsLoadingLine(effectiveScope)}
              </div>
            {:else if tagsQuery.isError}
              {@const tagsParsed = parseConnectError(tagsQuery.error)}
              {@const tagsCopy = tagsErrorCopy(tagsParsed)}
              <div class="px-2 py-1.5 text-sm">
                <p>{tagsCopy.heading}</p>
                {#if tagsCopy.kind === 'rejected'}
                  <pre class="whitespace-pre-wrap rounded border border-destructive bg-card p-1.5 font-mono text-xs text-destructive">{tagsCopy.envelope}</pre>
                {/if}
              </div>
            {:else}
              {#each tagRank.matches as m (m.tag)}
                <Command.Item value={`tag-${m.tag}`} onSelect={() => completeToken('#' + m.tag)}>
                  <TagMatchRow match={m} max={tagMax} />
                </Command.Item>
              {/each}
              {#if tagRank.unknown}
                {@const unk = tagRank.unknown}
                <Command.Item value={`tag-add-${unk.tag}`} onSelect={() => completeToken('#' + unk.tag)}>
                  <span>Add #{unk.tag}</span>
                  <span class="block font-mono text-[calc(11*var(--u))] text-[var(--warning)]">{unk.reason}</span>
                </Command.Item>
              {/if}
              {@const tagFooter = matchFooter({
                total: tagRank.total,
                more: tagsQuery.data?.more ?? false,
                loaded: tagRows.length
              })}
              <div class="px-2 py-1.5 text-xs font-mono text-muted-foreground">
                <span>{tagFooter.text}</span>
                {#if tagFooter.warn}
                  <span class="block text-[var(--warning)]">{tagFooter.warn}</span>
                {/if}
              </div>
            {/if}
          </Command.Group>
        {/snippet}

        <Command.List id={listId} class="max-h-[calc(540*var(--u))]">
          {#if tagPrefixInProgress !== null && classified.kind !== 'text'}
            {@render tagsGroup()}
          {/if}

          {#if classified.kind === 'text'}
            {#if searchError?.kind === 'rejected'}
              <div class="px-2 py-1.5 text-sm">
                <p>Server rejected the request</p>
                <pre class="whitespace-pre-wrap rounded border border-destructive bg-card p-1.5 font-mono text-xs text-destructive">field={searchError.fields.join(',')} hint={searchError.hint}: {searchError.detail}</pre>
              </div>
              <Command.Group heading="Fix it">
                {#each visibleFixRows as row (row.id)}
                  <Command.Item value={`fix-${row.id}`} onSelect={() => applyFixRow(row)}>{row.label}</Command.Item>
                {/each}
              </Command.Group>
            {:else if searchError?.kind === 'opaque'}
              <div class="px-2 py-1.5 text-sm">
                <p>Search failed — nothing was searched. search_memory returned code={searchError.codeName}</p>
                <p class="text-xs text-muted-foreground">{searchError.detail}</p>
              </div>
              <Command.Item value="fix-retry" onSelect={() => searchQuery.refetch()}>Retry</Command.Item>
            {:else}
              <Command.Group heading="Results">
                <Command.Item value="__search-all__" onSelect={runTopSearch}>
                  Search all memories for &quot;{classified.text}&quot; &nbsp; {hitCount} hits &nbsp;
                  <Kbd>↵</Kbd> /search
                </Command.Item>
              </Command.Group>
            {/if}
          {/if}

          {#if tagPrefixInProgress !== null && classified.kind === 'text'}
            {@render tagsGroup()}
          {/if}

          {#if classified.kind === 'operators'}
            <Command.Group heading="Results">
              <Command.Item value="__list-all__" onSelect={runTopList}>
                List memories matching {debouncedText} &nbsp; {listQuery.data?.total ?? 0n} total &nbsp;
                <Kbd>↵</Kbd> /search
              </Command.Item>
            </Command.Group>
          {/if}

          {#if classified.kind === 'id' && getError?.kind === 'not-found'}
            <div class="px-2 py-1.5 text-sm text-muted-foreground">
              No memory with id {idToFetch} that you can read · not-found and not-yours look the same by design
            </div>
          {/if}

          {#if classified.kind === 'short_id' && getError?.kind === 'ambiguous-short-id'}
            <div class="px-2 py-1.5 text-sm text-amber-600 dark:text-amber-400">
              short_id {getError.shortId} is ambiguous — paste the full id to be exact
            </div>
          {/if}

          {#if shortIdFallbackText}
            <div class="px-2 py-1.5 text-xs text-muted-foreground">
              No short_id attachment. Searched it as text instead: {shortIdFallbackText}
            </div>
          {/if}

          {#if (classified.kind === 'id' || classified.kind === 'short_id') && getQuery.data?.memory}
            <div class="px-2 py-1.5 text-xs text-muted-foreground">
              Resolved id {idToFetch.slice(0, 8)}… → 1 memory
            </div>
          {/if}

          {#if scopePrefixInProgress !== null}
            <Command.Group heading="Scopes">
              {#each filteredScopes as s (s.scope)}
                <Command.Item value={`scope-${s.scope}`} onSelect={() => completeToken(`scope:${s.scope}`)}>
                  <ScopeChip scope={s.scope} count={Number(s.count)} />
                </Command.Item>
              {/each}
            </Command.Group>
          {/if}
          {#if categoryPrefixInProgress !== null}
            <Command.Group heading="Categories">
              {#each filteredCategories as c (c)}
                <Command.Item value={`is-${c}`} onSelect={() => completeToken(`is:${c}`)}>is:{c}</Command.Item>
              {/each}
            </Command.Group>
          {/if}

          {#if shownMemories.length > 0}
            <Command.Group heading="Memories">
              {#each shownMemories as m (m.id)}
                <Command.Item value={m.id} onSelect={() => openMemory(m.id)}>
                  <Badge variant="outline" class="shrink-0">{m.category}</Badge>
                  <span class="min-w-0 flex-1 truncate">{m.summary || m.content}</span>
                  <ScopeChip scope={m.scope} />
                  <span class="shrink-0 text-xs text-muted-foreground">
                    {m.createdAt ? relativeTime(timestampDate(m.createdAt)) : ''}
                  </span>
                </Command.Item>
              {/each}
              {#if moreCount > 0}
                <div class="px-2 py-1.5 text-xs text-muted-foreground">{moreCount} more on /search</div>
              {/if}
            </Command.Group>
          {:else if classified.kind === 'text' && searchQuery.isSuccess && !searchError}
            <div class="px-2 py-1.5 text-sm text-muted-foreground">
              No memories match {classified.text} in any scope you can read
            </div>
            <Command.Item value="__search-discoveries__" onSelect={() => goCommand(`${base}/discovery`)}>
              Search discoveries for {classified.text}
            </Command.Item>
          {/if}

          {#if scopePrefixInProgress === null}
            <Command.Group heading="Scopes">
              {#each filteredScopes as s (s.scope)}
                <Command.Item value={`scope-${s.scope}`} onSelect={() => completeToken(`scope:${s.scope}`)}>
                  <ScopeChip scope={s.scope} count={Number(s.count)} />
                </Command.Item>
              {/each}
            </Command.Group>
          {/if}
          {#if categoryPrefixInProgress === null}
            <Command.Group heading="Categories">
              {#each filteredCategories as c (c)}
                <Command.Item value={`is-${c}`} onSelect={() => completeToken(`is:${c}`)}>is:{c}</Command.Item>
              {/each}
            </Command.Group>
          {/if}

          <Command.Group heading="Commands">
            {#each visibleCommands as c (c.href)}
              <Command.Item value={c.href} onSelect={() => goCommand(c.href)}>
                <c.icon data-icon="inline-start" />
                {c.label}
              </Command.Item>
            {/each}
          </Command.Group>
        </Command.List>

        {#if classified.kind === 'text' && searchQuery.isSuccess && !searchError}
          <div
            class="status flex items-center justify-between gap-2 border-t border-border px-2 py-1.5 text-xs"
            data-testid="header-search-status"
          >
            <span>
              {#if searchQuery.isFetching && searchQuery.isPlaceholderData}
                <span class="text-muted-foreground">previous results · </span>
              {/if}
              <strong>{hitCount}</strong>
              {hitCount === 1 ? 'hit' : 'hits'}
              {#if searchedScopesCount !== null}
                across <strong>{searchedScopesCount}</strong>
                {searchedScopesCount === 1 ? 'scope' : 'scopes'}
              {/if}
              {#if hiddenNote}
                · {hiddenNote}
              {/if}
            </span>
            <button
              type="button"
              class="shrink-0 whitespace-nowrap text-muted-foreground"
              onclick={() => (sourceOpen = !sourceOpen)}
            >
              {sourceLabel}
            </button>
          </div>
          {#if sourceOpen}
            <div class="px-2 py-1.5 text-xs">
              {#each perScopeCoverage as row (row.scope)}
                <div>{row.count > 0 ? '✓' : '·'} {row.scope} &nbsp; {row.count} {row.count === 1 ? 'hit' : 'hits'}</div>
              {/each}
              <div>discovery:* not included (separate lane: search_discovery)</div>
            </div>
          {/if}
        {/if}
      </Popover.Content>
    </Popover.Root>
  </CommandPrimitive.Root>
</div>
