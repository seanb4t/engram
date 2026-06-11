<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  let { memories, total, approximate = false, loading, error, selectedId, onselect }: {
    memories: Memory[]; total: bigint; approximate?: boolean; loading: boolean;
    error: unknown; selectedId: string; onselect: (id: string) => void;
  } = $props();
  const catColor = (c: string) => `var(--cat-${c})`;
</script>

{#if loading}
  <div data-testid="list-loading" class="p-3" style="color:var(--muted)">loading…</div>
{:else if error}
  <div class="p-3" style="color:var(--cat-gotcha)">failed to load — retry</div>
{:else if memories.length === 0}
  <div class="p-3" style="color:var(--muted)">no memories in this scope/filter</div>
{:else}
  {#each memories as m (m.id)}
    <button class="block w-full text-left px-3 py-2" style="border-bottom:1px solid var(--border);{m.id === selectedId ? 'background:var(--surface)' : ''}" onclick={() => onselect(m.id)}>
      <span style="color:{catColor(m.category)};font-weight:700">{m.category}</span>
      <span> · {m.content.slice(0, 80)}</span>
      <div style="color:var(--muted);font-size:11px">{m.tags.map((t) => '#' + t).join(' ')} · {m.visibility}</div>
    </button>
  {/each}
  <div class="px-3 py-2 text-center" style="color:var(--muted)">{memories.length} of {total}{approximate ? ' (approximate)' : ''}</div>
{/if}
