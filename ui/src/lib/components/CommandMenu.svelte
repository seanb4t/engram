<script lang="ts">
  import * as Command from '$lib/components/ui/command';
  import { Kbd } from '$lib/components/ui/kbd';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { page } from '$app/state';
  import { useQueryClient } from '@tanstack/svelte-query';
  import { setMode, mode } from 'mode-watcher';
  import { toast } from 'svelte-sonner';
  import HouseIcon from '@lucide/svelte/icons/house';
  import EyeIcon from '@lucide/svelte/icons/eye';
  import SearchIcon from '@lucide/svelte/icons/search';
  import CompassIcon from '@lucide/svelte/icons/compass';
  import { classifyInput } from '$lib/search/classify';
  import { handoffToHeaderSearch } from '$lib/search/header-search.svelte';
  import { defaultSearchParams, encodeSearchParams } from '$lib/search/params';
  import { stepTextSize, resetTextSize } from '$lib/display.svelte';
  import type { Memory } from '$lib/gen/engram_pb';

  // D-11: ⌘K is a command menu of static actions (navigation, display,
  // theme, copy-id) filtered client-side against their own labels -- honest,
  // since they are not memories -- that always ends in an unfiltered
  // hand-off row to the header search. There is no Command.Empty anywhere in
  // this file: the menu never claims "no matches" for a memory term, and it
  // makes no server call at all (see recall-surface.md "What to Avoid").

  let { open = $bindable(false) }: { open?: boolean } = $props();
  let q = $state('');

  // Client-side substring filtering against each item's own label (honest --
  // these are commands, not memories). The Command primitive's own
  // shouldFilter/scoring is disabled below (shouldFilter={false}, matching
  // HeaderSearch's precedent): its native filter drives a DOM-reparenting
  // sort pass that fights Svelte's own reactive updates once a forceMount
  // group is present, so visibility is computed and rendered by US instead.
  function matchesQuery(label: string): boolean {
    const needle = q.trim().toLowerCase();
    return !needle || label.toLowerCase().includes(needle);
  }

  const navItems = [
    { label: 'Home', href: `${base}/`, icon: HouseIcon },
    { label: 'Observe', href: `${base}/observe`, icon: EyeIcon },
    { label: 'Search', href: `${base}/search`, icon: SearchIcon },
    { label: 'Discovery', href: `${base}/discovery`, icon: CompassIcon }
  ];
  const visibleNavItems = $derived(navItems.filter((item) => matchesQuery(item.label)));

  function goNav(href: string) {
    goto(href);
    open = false;
  }

  // Display group (static, filtered like everything else): text size and
  // theme. Mirrors the ⌘+/⌘-/⌘0 shortcuts' own toast wording (D-13).
  function notifyTextSize(result: { size: number; atMin: boolean; atMax: boolean }) {
    const suffix = result.atMax ? ' (max)' : result.atMin ? ' (min)' : '';
    toast(`Text size ${result.size}px${suffix}`);
  }

  function selectLargerText() {
    notifyTextSize(stepTextSize(1));
    open = false;
  }

  function selectSmallerText() {
    notifyTextSize(stepTextSize(-1));
    open = false;
  }

  function selectResetTextSize() {
    notifyTextSize(resetTextSize());
    open = false;
  }

  function selectToggleTheme() {
    setMode(mode.current === 'dark' ? 'light' : 'dark');
    open = false;
  }

  const displayItems: { label: string; shortcut?: string; onSelect: () => void }[] = [
    { label: 'Larger text', shortcut: '⌘=', onSelect: selectLargerText },
    { label: 'Smaller text', shortcut: '⌘-', onSelect: selectSmallerText },
    { label: 'Reset text size', shortcut: '⌘0', onSelect: selectResetTextSize },
    { label: 'Toggle theme', onSelect: selectToggleTheme }
  ];
  const visibleDisplayItems = $derived(displayItems.filter((item) => matchesQuery(item.label)));

  // Record group: copy the currently-open record's id/short_id (only present
  // when a record is selected via ?sel=, matching the same query-cache key
  // MemoryDetail/DetailPane already populate).
  const queryClient = useQueryClient();
  const sel = $derived(page.url.searchParams.get('sel') ?? '');
  const selMemory = $derived(sel ? queryClient.getQueryData<{ memory?: Memory }>(['getMemory', sel])?.memory : undefined);
  const recordItems = $derived.by(() => {
    if (!sel) return [] as { label: string; onSelect: () => void }[];
    const items: { label: string; onSelect: () => void }[] = [
      { label: `Copy full id ${sel.slice(0, 8)}…`, onSelect: () => copyText(sel) }
    ];
    if (selMemory?.shortId) {
      const shortId = selMemory.shortId;
      items.push({ label: `Copy short_id ${shortId}`, onSelect: () => copyText(shortId) });
    }
    return items;
  });
  const visibleRecordItems = $derived(recordItems.filter((item) => matchesQuery(item.label)));

  async function copyText(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      toast.success('copied');
    } catch {
      // clipboard write can reject (denied permission, insecure context, lost
      // focus) -- surface it so the item never appears to silently do nothing.
      toast.error('copy failed');
    }
    open = false;
  }

  // Reactive derivation -- classifyInput re-runs on every keystroke, so the
  // hand-off row's label and target always reflect the CURRENT q, never a
  // value snapshotted when the row was first mounted (the deleted lying
  // palette's live bug).
  const classified = $derived(classifyInput(q));
  const isId = $derived(classified.kind === 'id' || classified.kind === 'short_id');
  const idValue = $derived(
    classified.kind === 'id' ? classified.id : classified.kind === 'short_id' ? classified.shortId : ''
  );

  function selectHandoff() {
    const text = q.trim();
    if (!text) return;
    if (isId) {
      goto(`${base}/search?${encodeSearchParams({ ...defaultSearchParams(), q: idValue })}`);
    } else {
      handoffToHeaderSearch(text);
    }
    open = false;
  }
</script>

<Command.Dialog bind:open shouldFilter={false}>
  <Command.Input bind:value={q} placeholder="Jump to… or search memories" aria-label="Command menu" />
  <Command.List>
    <Command.Group heading="Go to">
      {#each visibleNavItems as item (item.href)}
        <Command.Item value={item.label} onSelect={() => goNav(item.href)}>
          <item.icon data-icon="inline-start" />
          {item.label}
        </Command.Item>
      {/each}
    </Command.Group>

    {#if visibleDisplayItems.length > 0}
      <Command.Group heading="Display">
        {#each visibleDisplayItems as item (item.label)}
          <Command.Item value={item.label} onSelect={item.onSelect}>
            {item.label}
            {#if item.shortcut}<Command.Shortcut>{item.shortcut}</Command.Shortcut>{/if}
          </Command.Item>
        {/each}
      </Command.Group>
    {/if}

    {#if visibleRecordItems.length > 0}
      <Command.Group heading="Record">
        {#each visibleRecordItems as item (item.label)}
          <Command.Item value={item.label} onSelect={item.onSelect}>{item.label}</Command.Item>
        {/each}
      </Command.Group>
    {/if}

    {#if q.trim()}
      <Command.Group forceMount>
        <Command.Item value={`handoff:${q}`} forceMount onSelect={selectHandoff}>
          {#if isId}
            Open record {idValue.slice(0, 8)}…
          {:else}
            Search memories for &quot;<span
              class="inline-block max-w-[24ch] truncate align-bottom font-mono"
              title={q.trim()}>{q.trim()}</span
            >&quot; <Kbd>↵</Kbd>
          {/if}
        </Command.Item>
      </Command.Group>
    {/if}
  </Command.List>
</Command.Dialog>
