<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { stripCategoryPrefix } from '$lib/summary';

  // Minimal Task 1 shape: fixed-height single-line row with a category dot
  // and the ellipsized summary. Task 2 fills in the full dense grid (state
  // chips, tags, scope, age, score, rel) on top of this same fixed height.
  let {
    memory,
    mode = 'ranked',
    showRel = false,
    active = false,
    opened = false,
    listFocused = false
  }: {
    memory: Memory;
    mode?: 'ranked' | 'unranked';
    showRel?: boolean;
    active?: boolean;
    opened?: boolean;
    listFocused?: boolean;
  } = $props();

  const summary = $derived(stripCategoryPrefix(memory.summary, memory.category));
</script>

<div class="result-row-line" style="--c:var(--cat-{memory.category})">
  <span class="cat-dot" aria-hidden="true" style="background:var(--c)"></span>
  <span class="sum">{summary}</span>
</div>

<style>
  .result-row-line {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
    column-gap: calc(6 * var(--u));
    height: calc(28 * var(--u));
    padding: 0 calc(12 * var(--u)) 0 calc(14 * var(--u));
  }
  .cat-dot {
    width: calc(6 * var(--u));
    height: calc(6 * var(--u));
    border-radius: 999px;
    flex: none;
  }
  .sum {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: calc(13 * var(--u));
  }
</style>
