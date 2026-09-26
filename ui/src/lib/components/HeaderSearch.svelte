<script lang="ts">
  import { Command as CommandPrimitive } from 'bits-ui';
  import { createQuery, keepPreviousData } from '@tanstack/svelte-query';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import * as Popover from '$lib/components/ui/popover';
  import * as Command from '$lib/components/ui/command';
  import { Kbd } from '$lib/components/ui/kbd';
  import { Badge } from '$lib/components/ui/badge';
  import ScopeChip from './ScopeChip.svelte';
  import SearchIcon from '@lucide/svelte/icons/search';
  import EyeIcon from '@lucide/svelte/icons/eye';
  import CompassIcon from '@lucide/svelte/icons/compass';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { engram } from '$lib/client';
  import { classifyInput } from '$lib/search/classify';
  import { relativeTime } from '$lib/time';

  // The header search box (D-11): one shared classifier decides which RPC
  // runs, and the dropdown is server-driven (ENTRY-01/02/04) — never a
  // client-filtered palette (see recall-surface.md "What to Avoid").

  let text = $state('');
  let debouncedText = $state('');
  let open = $state(false);
  let inputWrapperEl = $state<HTMLElement | null>(null);

  // 70ms debounce (ENTRY-06): re-derive debouncedText from text after the
  // caller stops typing; the effect's own cleanup cancels a stale timer.
  $effect(() => {
    const current = text;
    const timer = setTimeout(() => {
      debouncedText = current;
    }, 70);
    return () => clearTimeout(timer);
  });

  const classified = $derived(classifyInput(debouncedText));
  const queryText = $derived(classified.kind === 'text' ? classified.text : '');
  const idToFetch = $derived(
    classified.kind === 'id' ? classified.id : classified.kind === 'short_id' ? classified.shortId : ''
  );

  const searchQuery = createQuery(() => ({
    queryKey: ['headerSearch', queryText, '', true, [], []],
    queryFn: ({ signal }) =>
      engram.searchMemories(
        { query: queryText, scope: '', crossSpine: true, tags: [], categories: [], k: 50n, full: false },
        { signal }
      ),
    enabled: classified.kind === 'text',
    placeholderData: keepPreviousData,
    meta: { silent: true }
  }));

  const getQuery = createQuery(() => ({
    queryKey: ['getMemory', idToFetch],
    queryFn: ({ signal }) => engram.getMemory({ id: idToFetch }, { signal }),
    enabled: classified.kind === 'id' || classified.kind === 'short_id',
    meta: { silent: true }
  }));

  const hitCount = $derived(searchQuery.data?.memories.length ?? 0);
  const memories = $derived(
    searchQuery.data?.memories ?? (getQuery.data?.memory ? [getQuery.data.memory] : [])
  );
  const shownMemories = $derived(memories.slice(0, 5));
  const moreCount = $derived(Math.max(0, memories.length - 5));

  const commandItems = [
    { label: 'Observe', href: `${base}/observe`, icon: EyeIcon },
    { label: 'Search', href: `${base}/search`, icon: SearchIcon },
    { label: 'Discovery', href: `${base}/discovery`, icon: CompassIcon }
  ];
  const visibleCommands = $derived(
    debouncedText.trim()
      ? commandItems.filter((c) => c.label.toLowerCase().includes(debouncedText.trim().toLowerCase()))
      : commandItems
  );

  function runTopSearch() {
    if (classified.kind !== 'text') return;
    goto(`${base}/search?q=${encodeURIComponent(classified.text)}`);
    text = '';
    debouncedText = '';
    open = false;
  }

  function openMemory(id: string) {
    goto(`${base}/search?sel=${encodeURIComponent(id)}`);
    open = false;
  }

  function goCommand(href: string) {
    goto(href);
    open = false;
  }
</script>

<div class="hd-search relative mx-auto w-full max-w-[calc(560*var(--u))]">
  <CommandPrimitive.Root shouldFilter={false} class="contents">
    <div
      bind:this={inputWrapperEl}
      class="hd-input flex h-9 items-center gap-2 rounded-md border border-input bg-background px-2 focus-within:border-primary focus-within:ring-[calc(3*var(--u))] focus-within:ring-primary/20"
    >
      <SearchIcon class="size-4 shrink-0 opacity-50" />
      <CommandPrimitive.Input
        bind:value={text}
        aria-label="Search memories"
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
      >
        <Command.List class="max-h-[calc(540*var(--u))]">
          {#if classified.kind === 'text'}
            <Command.Group heading="Results">
              <Command.Item value="__search-all__" onSelect={runTopSearch}>
                Search all memories for &quot;{classified.text}&quot; &nbsp; {hitCount} hits &nbsp;
                <Kbd>↵</Kbd> /search
              </Command.Item>
            </Command.Group>
          {/if}

          {#if (classified.kind === 'id' || classified.kind === 'short_id') && getQuery.data?.memory}
            <div class="px-2 py-1.5 text-xs text-muted-foreground">
              Resolved id {idToFetch.slice(0, 8)}… → 1 memory
            </div>
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
          {:else if classified.kind === 'text' && searchQuery.isSuccess}
            <div class="px-2 py-1.5 text-sm text-muted-foreground">
              No memories match {classified.text} in any scope you can read
            </div>
            <Command.Item value="__search-discoveries__" onSelect={() => goCommand(`${base}/discovery`)}>
              Search discoveries for {classified.text}
            </Command.Item>
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
      </Popover.Content>
    </Popover.Root>
  </CommandPrimitive.Root>
</div>
