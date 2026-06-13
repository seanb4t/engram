<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { Skeleton } from '$lib/components/ui/skeleton';
  import * as Empty from '$lib/components/ui/empty';
  import MemoryRow from './MemoryRow.svelte';
  let { memories, total, approximate = false, loading, error, selectedId, onselect, showScope = false }: {
    memories: Memory[]; total: bigint; approximate?: boolean; loading: boolean; error: unknown; selectedId: string; onselect: (id: string) => void; showScope?: boolean;
  } = $props();
</script>

{#if loading}
  <div data-testid="list-loading" class="p-3 flex flex-col gap-2"><Skeleton class="h-12 w-full" /><Skeleton class="h-12 w-full" /><Skeleton class="h-12 w-full" /></div>
{:else if error}
  <div class="p-3 text-cat-gotcha">failed to load — retry from the toolbar</div>
{:else if memories.length === 0}
  <Empty.Root class="p-8"><Empty.Title>no memories</Empty.Title><Empty.Description>nothing in this scope / filter</Empty.Description></Empty.Root>
{:else}
  {#each memories as m (m.id)}
    <MemoryRow memory={m} selected={m.id === selectedId} {showScope} {onselect} />
  {/each}
  <div class="px-3 py-2 text-center text-muted-foreground text-[11px]">{memories.length} of {total}{approximate ? ' (approximate)' : ''}</div>
{/if}
