<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { stripCategoryPrefix } from '$lib/summary';
  import { relativeTime } from '$lib/time';
  import { memoryStateWords, isPastState } from '$lib/memorystate';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
  import ScopeChip from './ScopeChip.svelte';
  import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';
  import PencilIcon from '@lucide/svelte/icons/pencil';
  import Trash2Icon from '@lucide/svelte/icons/trash-2';
  import ShareIcon from '@lucide/svelte/icons/share';

  let {
    memory,
    selected,
    showScope = false,
    onselect,
    onedit,
    ondelete,
    onshare
  }: {
    memory: Memory;
    selected: boolean;
    showScope?: boolean;
    onselect: (id: string) => void;
    onedit?: (id: string) => void;
    ondelete?: (id: string) => void;
    onshare?: (memory: Memory) => void;
  } = $props();
  // summary is server-guaranteed non-empty on list/search. stripCategoryPrefix
  // drops a leading "CATEGORY (...)" token when present — common on auto
  // summaries and truncation previews — so the category isn't shown twice
  // (the row already renders the category separately below).
  const summary = $derived(stripCategoryPrefix(memory.summary, memory.category));
  const isAuto = $derived(memory.summarySource === 'auto');
  const when = $derived(memory.createdAt ? relativeTime(timestampDate(memory.createdAt)) : '');
  const shownTags = $derived(memory.tags.slice(0, 3));
  const overflow = $derived(Math.max(0, memory.tags.length - 3));
  // D-05 rule fence (mechanical, not "parent omits callbacks") + visibility
  // gate on Share (locked D-07: private->shared is one-way, no unshare item).
  const isRule = $derived(memory.category === 'rule');
  const isShared = $derived(memory.visibility === 'shared');
  const showMenu = $derived(!isRule && !!(onedit || ondelete || (onshare && !isShared)));
  // D-13/D-15: state words drive the badges below and the dim-iff-past
  // treatment. dimmed applies opacity-60 to individual dimmable elements —
  // never to a shared ancestor of the badges, since CSS opacity multiplies
  // through descendants (accessibility carve-out: the badge stays full
  // opacity even inside a dimmed row).
  const stateWords = $derived(memoryStateWords(memory));
  const dimmed = $derived(isPastState(stateWords));
  const dimCls = $derived(dimmed ? 'opacity-60' : '');
</script>

<div
  style="--c:var(--cat-{memory.category})"
  class={'group relative flex items-stretch border-b border-border ' + (selected ? 'bg-accent' : 'hover:bg-accent')}
>
  <span class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r" style="background:var(--c)"></span>
  <button
    type="button"
    onclick={() => onselect(memory.id)}
    class="flex-1 min-w-0 text-left px-3 py-2 flex flex-col gap-1"
  >
    <div class="flex items-center gap-2 min-w-0">
      <span class={'truncate flex-1 text-[13px] ' + dimCls}>{summary}</span>
      {#if isAuto}<span aria-label="auto-generated summary" title="auto-generated summary" class="shrink-0 text-[10px] text-primary">✦</span>{/if}
    </div>
    <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-muted-foreground min-w-0">
      <span class={'font-medium shrink-0 ' + dimCls} style="color:var(--c)">{memory.category}</span>
      <span class={'shrink-0 ' + dimCls}>·</span>
      <span class={'tabular-nums shrink-0 ' + dimCls}>{when}</span>
      {#each stateWords as word (word)}<Badge variant="outline" class="text-[10px] uppercase shrink-0">{word}</Badge>{/each}
      {#if showScope && memory.scope}<span class={'shrink-0 ' + dimCls}><ScopeChip scope={memory.scope} /></span>{/if}
      {#each shownTags as t (t)}<span class={'shrink-0 px-1 rounded bg-muted font-mono text-[10.5px] ' + dimCls}>{t}</span>{/each}
      {#if overflow > 0}<span class={'shrink-0 ' + dimCls}>+{overflow}</span>{/if}
    </div>
  </button>
  {#if showMenu}
    <div class="shrink-0 flex items-center pr-2 opacity-0 group-hover:opacity-100 focus-within:opacity-100">
      <DropdownMenu.Root>
        <DropdownMenu.Trigger>
          {#snippet child({ props })}
            <Button variant="ghost" size="icon-sm" aria-label="row actions" {...props}>
              <EllipsisVerticalIcon />
            </Button>
          {/snippet}
        </DropdownMenu.Trigger>
        <DropdownMenu.Content align="end">
          {#if onedit}
            <DropdownMenu.Item onSelect={() => onedit?.(memory.id)}><PencilIcon /> Edit</DropdownMenu.Item>
          {/if}
          {#if ondelete}
            <DropdownMenu.Item variant="destructive" onSelect={() => ondelete?.(memory.id)}><Trash2Icon /> Delete</DropdownMenu.Item>
          {/if}
          {#if onshare && !isShared}
            <DropdownMenu.Item onSelect={() => onshare?.(memory)}><ShareIcon /> Share</DropdownMenu.Item>
          {/if}
        </DropdownMenu.Content>
      </DropdownMenu.Root>
    </div>
  {/if}
</div>
