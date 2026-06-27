<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { stripCategoryPrefix } from '$lib/summary';
  import { relativeTime } from '$lib/time';
  import ScopeChip from './ScopeChip.svelte';
  let { memory, selected, showScope = false, onselect }: { memory: Memory; selected: boolean; showScope?: boolean; onselect: (id: string) => void } = $props();
  // summary is server-guaranteed non-empty on list/search. stripCategoryPrefix
  // drops a leading "CATEGORY (...)" token when present — common on auto
  // summaries and truncation previews — so the category isn't shown twice
  // (the row already renders the category separately below).
  const summary = $derived(stripCategoryPrefix(memory.summary, memory.category));
  const isAuto = $derived(memory.summarySource === 'auto');
  const when = $derived(memory.createdAt ? relativeTime(timestampDate(memory.createdAt)) : '');
  const shownTags = $derived(memory.tags.slice(0, 3));
  const overflow = $derived(Math.max(0, memory.tags.length - 3));
</script>

<button
  type="button"
  onclick={() => onselect(memory.id)}
  style="--c:var(--cat-{memory.category})"
  class={'relative w-full text-left px-3 py-2 border-b border-border flex flex-col gap-1 hover:bg-accent ' + (selected ? 'bg-accent' : '')}
>
  <span class="absolute left-0 top-2 bottom-2 w-[3px] rounded-r" style="background:var(--c)"></span>
  <div class="flex items-center gap-2 min-w-0">
    <span class="truncate flex-1 text-[13px]">{summary}</span>
    {#if isAuto}<span aria-label="auto-generated summary" title="auto-generated summary" class="shrink-0 text-[10px] text-primary">✦</span>{/if}
  </div>
  <div class="flex items-center gap-2 text-[11px] text-muted-foreground min-w-0">
    <span class="font-medium shrink-0" style="color:var(--c)">{memory.category}</span>
    <span class="shrink-0">·</span>
    <span class="tabular-nums shrink-0">{when}</span>
    {#if showScope && memory.scope}<span class="shrink-0"><ScopeChip scope={memory.scope} /></span>{/if}
    {#each shownTags as t (t)}<span class="shrink-0 px-1 rounded bg-muted font-mono text-[10.5px]">{t}</span>{/each}
    {#if overflow > 0}<span class="shrink-0">+{overflow}</span>{/if}
  </div>
</button>
