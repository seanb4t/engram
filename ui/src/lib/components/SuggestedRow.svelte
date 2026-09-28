<script lang="ts">
  import type { FilterSuggestion } from '$lib/gen/engram_pb';
  import type { SearchParams } from '$lib/search/params';
  import { suggestionLabel, acceptPartial } from '$lib/search/understand';

  // NLQ-03/D-11: the Suggested row — unapplied, dashed chips that accept
  // through the page's own FacetStrip onchange path. Task 1 wires the
  // accept control only; Task 2 adds the dismiss control and every
  // suggestion kind; Task 3 adds the roving toolbar keyboard model and the
  // full visual contract.
  let {
    suggestions,
    params,
    onchange
  }: {
    suggestions: FilterSuggestion[];
    params: SearchParams;
    onchange: (partial: Partial<SearchParams>) => void;
  } = $props();
</script>

<div class="suggested-row" role="toolbar" aria-label="Suggested filters">
  <span class="suggested-caption" aria-hidden="true">Suggested</span>
  {#each suggestions as s, i (i)}
    {@const label = suggestionLabel(s)}
    <div class="suggested-chip">
      <button
        type="button"
        class="suggested-accept"
        aria-label={`Suggested filter, not applied: ${label}`}
        title={label}
        onclick={() => onchange(acceptPartial(s, params))}
      >
        <span class="suggested-label">{label}</span>
      </button>
    </div>
  {/each}
</div>

<style>
  .suggested-row {
    display: flex;
    align-items: center;
    gap: calc(6 * var(--u));
    padding: calc(6 * var(--u)) calc(14 * var(--u));
  }
  .suggested-caption {
    font-size: calc(11 * var(--u));
    color: var(--text-faint, var(--muted-foreground));
  }
  .suggested-chip {
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
    height: calc(20 * var(--u));
    border-radius: 9999px;
    border: 1px dashed var(--border);
    background: transparent;
    font-size: calc(11 * var(--u));
    padding: 0 calc(8 * var(--u));
    white-space: nowrap;
    flex: none;
  }
  .suggested-accept {
    display: inline-flex;
    align-items: center;
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    color: inherit;
    cursor: pointer;
  }
  .suggested-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
</style>
