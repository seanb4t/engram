<script lang="ts">
  // The shared tag popularity list (TAGS-01, D-12, D-13): one cached
  // ListTags(scope, 1000) query drives both /search's docked panel
  // (mode="panel") and /related's rail Tags tab (mode="rail"). Task 1 is the
  // thinnest tracer slice -- draw the server's rows as linear bars and report
  // clicks; top-30/filter/honesty-footer/tooltip/markers/keyboard/E4 states
  // land in Task 2.
  import { createQuery } from '@tanstack/svelte-query';
  import { listTagsQuery } from '$lib/tags/query';
  import { toTagRows, barPercent } from '$lib/tags/tags';

  let {
    scope,
    mode,
    markedTags,
    selectedTags,
    rarity,
    ontoggle
  }: {
    scope: string;
    mode: 'panel' | 'rail';
    markedTags?: ReadonlySet<string>;
    selectedTags?: ReadonlySet<string>;
    rarity?: ReadonlyMap<string, number>;
    ontoggle: (tag: string) => void;
  } = $props();

  const q = createQuery(() => listTagsQuery(scope));

  const rows = $derived(toTagRows(q.data));
  const max = $derived(rows.length > 0 ? rows[0].count : 0);
</script>

<div class="tag-bars" data-mode={mode}>
  <div role="listbox" aria-label="Tags by count" class="bar-list">
    {#each rows as row (row.tag)}
      <div
        role="option"
        aria-selected={selectedTags?.has(row.tag) ?? false}
        class="bar-row"
        class:marked={markedTags?.has(row.tag) ?? false}
        tabindex="-1"
        onclick={() => ontoggle(row.tag)}
      >
        <span class="nm">{row.tag}</span>
        <span class="track">
          <span class="fill" style={`width: ${barPercent(row.count, max)}%`}></span>
        </span>
        <span class="cnt">{row.count}</span>
      </div>
    {/each}
  </div>
</div>

<style>
  .bar-row {
    display: grid;
    grid-template-columns: calc(132 * var(--u)) 1fr calc(36 * var(--u));
    gap: calc(8 * var(--u));
    align-items: center;
    cursor: pointer;
  }
  .nm {
    font-family: var(--font-mono, monospace);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .track {
    display: block;
  }
  .fill {
    display: block;
    height: calc(8 * var(--u));
    min-width: 2px;
    background: var(--text-faint);
    border-radius: 0 calc(4 * var(--u)) calc(4 * var(--u)) 0;
  }
  .bar-row.marked .fill {
    background: var(--primary);
  }
  .bar-row.marked .nm::before {
    content: '● ';
    color: var(--primary);
  }
  .cnt {
    font-family: var(--font-mono, monospace);
    text-align: right;
  }
</style>
