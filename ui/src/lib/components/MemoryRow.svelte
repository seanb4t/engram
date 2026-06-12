<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { Badge } from '$lib/components/ui/badge';
  import { stripCategoryPrefix } from '$lib/summary';
  import { relativeTime } from '$lib/time';
  import ScopeChip from './ScopeChip.svelte';
  let { memory, selected, showScope = false, onselect }: { memory: Memory; selected: boolean; showScope?: boolean; onselect: (id: string) => void } = $props();
  const summary = $derived(stripCategoryPrefix(memory.content, memory.category));
  const when = $derived(memory.createdAt ? relativeTime(timestampDate(memory.createdAt)) : '');
  const shownTags = $derived(memory.tags.slice(0, 3));
  const overflow = $derived(Math.max(0, memory.tags.length - 3));
</script>

<button
  type="button"
  onclick={() => onselect(memory.id)}
  class={'w-full text-left px-3 py-2 border-b border-border flex flex-col gap-1.5 hover:bg-accent ' + (selected ? 'bg-accent shadow-[inset_2px_0_0_var(--primary)]' : '')}
>
  <div class="flex items-center gap-2 min-w-0">
    <Badge variant="outline" class="shrink-0 text-[10px] uppercase" style="color:var(--cat-{memory.category})">{memory.category}</Badge>
    <span class="truncate flex-1 text-[13px]">{summary}</span>
  </div>
  <div class="flex items-center gap-2 text-[11px] text-muted-foreground min-w-0">
    <span class="tabular-nums shrink-0">{when}</span>
    {#if showScope && memory.scope}<span class="shrink-0"><ScopeChip scope={memory.scope} /></span>{/if}
    {#each shownTags as t (t)}<span class="shrink-0 px-1 rounded bg-muted font-mono text-[10.5px]">{t}</span>{/each}
    {#if overflow > 0}<span class="shrink-0">+{overflow}</span>{/if}
  </div>
</button>
