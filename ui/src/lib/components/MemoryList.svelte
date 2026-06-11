<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { Button } from '$lib/components/ui/button';
  let { memories, total, approximate = false, showTotal = true, loading, error, selectedId, onselect }: {
    memories: Memory[]; total: bigint; approximate?: boolean; showTotal?: boolean; loading: boolean;
    error: unknown; selectedId: string; onselect: (id: string) => void;
  } = $props();
  const catColor = (c: string) => `var(--cat-${c})`;
</script>

{#if loading}
  <div data-testid="list-loading" class="p-3 eg-muted">loading…</div>
{:else if error}
  <div class="p-3 eg-error">failed to load — retry</div>
{:else if memories.length === 0}
  <div class="p-3 eg-muted">no memories in this scope/filter</div>
{:else}
  {#each memories as m (m.id)}
    <Button
      variant="ghost"
      class="block w-full text-left px-3 py-2 h-auto eg-row {m.id === selectedId ? 'eg-surface' : ''}"
      onclick={() => onselect(m.id)}
    >
      <span style="color:{catColor(m.category)};font-weight:700">{m.category}</span>
      <span> · {m.content.slice(0, 80)}</span>
      <div class="eg-muted" style="font-size:11px">{m.tags.map((t) => '#' + t).join(' ')} · {m.visibility}</div>
    </Button>
  {/each}
  <div class="px-3 py-2 text-center eg-muted">
    {showTotal
      ? `${memories.length} of ${total}${approximate ? ' (approximate)' : ''}`
      : `${memories.length} result${memories.length === 1 ? '' : 's'}`}
  </div>
{/if}
