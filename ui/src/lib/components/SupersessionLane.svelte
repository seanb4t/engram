<script lang="ts">
  // The supersession timeline lane: a horizontal strip of columns
  // (oldest-left, head-right) built from supersessionColumns(model) --
  // its own layout and column-label vocabulary (`−2 / −1 / anchor / +1`),
  // distinct from ChainDialog's `d2 / d1 / head · d0` modal grid (05-UI-SPEC
  // Deltas table). Shares EdgeLane's header shape and shared-selection
  // convention, never its row grid.
  import type { LaneType, RelatedModel } from '$lib/related/graph';
  import { EDGE_STYLE } from '$lib/related/graph';
  import { laneCaption, laneCountLabel, supersessionColumns } from '$lib/related/lanes';
  import { relativeTime } from '$lib/time';
  import { ScrollArea } from '$lib/components/ui/scroll-area';

  let {
    model,
    hidden,
    selectedId,
    selectedLane,
    dimmedIds,
    onselect,
    ontogglehidden
  }: {
    model: RelatedModel;
    hidden: boolean;
    selectedId: string | null;
    selectedLane: LaneType | 'graph' | null;
    dimmedIds?: ReadonlySet<string>;
    onselect: (id: string, lane: LaneType) => void;
    ontogglehidden: () => void;
  } = $props();

  const style = $derived(EDGE_STYLE.supersession);
  const columns = $derived(supersessionColumns(model));
</script>

<section class="card lane">
  <div class="lane-h">
    <svg class="swatch" width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
      <line x1="1" y1="8" x2="15" y2="8" stroke="var(--muted-foreground)" stroke-width={style.width} />
    </svg>
    <span class="name">supersession</span>
    <span class="cnt">{laneCountLabel('supersession', model)}</span>
    <span class="cap">{laneCaption('supersession', model.k)}</span>
    <button type="button" class="eye" aria-pressed={!hidden} onclick={ontogglehidden}>{hidden ? 'show' : 'hide'}</button>
  </div>
  {#if !hidden}
    <div class="lane-body">
      {#if columns.length <= 1}
        <p class="empty">none — no superseded_by, no supersedes</p>
      {:else}
        <ScrollArea orientation="horizontal">
          <div class="chain-cols">
            {#each columns as col, i (col.depth)}
              {#if i > 0}
                <span class="arrow" aria-hidden="true" title="superseded_by">→</span>
              {/if}
              <div class="ccol">
                <span class="ccol-h">{col.label}</span>
                {#each col.cards as card (card.id)}
                  <button
                    type="button"
                    class="chain-card"
                    class:is-anchor={card.isAnchor}
                    class:hidden-state={card.states.length > 0}
                    class:sel={selectedId === card.id}
                    class:sel-here={selectedId === card.id && selectedLane === 'supersession'}
                    class:fdim={dimmedIds?.has(card.id)}
                    data-testid={`chain-card-${card.id}`}
                    onclick={() => onselect(card.id, 'supersession')}
                  >
                    <span class="ch-meta">
                      <i class="dot" aria-hidden="true" style="background:var(--cat-{card.category})"></i>
                      <span class="mono">{card.shortId}</span>
                      <span class="depth mono">{card.isAnchor ? 'anchor' : col.label}</span>
                    </span>
                    <span class="ch-sum">{card.summary}</span>
                    {#if card.createdAt}<span class="ch-date">{relativeTime(card.createdAt)}</span>{/if}
                    {#if card.types.length > 1}
                      <span class="x" title={`reached by ${card.types.length} edge types: ${card.types.join(', ')}`}
                        >×{card.types.length}</span
                      >
                    {/if}
                    <span class="states-foot">
                      {#each card.states as w (w)}<span class="st">{w}</span>{/each}
                    </span>
                  </button>
                {/each}
              </div>
            {/each}
          </div>
        </ScrollArea>
      {/if}
    </div>
  {/if}
</section>

<style>
  .lane {
    display: flex;
    flex-direction: column;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
  }
  .lane-h {
    display: flex;
    align-items: center;
    gap: calc(8 * var(--u));
    padding: calc(4 * var(--u)) calc(8 * var(--u));
    border-bottom: 1px solid var(--border-subtle);
  }
  .swatch {
    flex: none;
  }
  .name {
    font-weight: 600;
    font-size: calc(11 * var(--u));
    text-transform: capitalize;
  }
  .cnt {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .cap {
    flex: 1;
    min-width: 0;
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
    overflow-wrap: break-word;
  }
  .eye {
    flex: none;
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
    padding: 0 calc(6 * var(--u));
  }
  .empty {
    padding: calc(8 * var(--u));
    font-size: calc(12 * var(--u));
    color: var(--text-faint);
  }
  .lane-body {
    padding: calc(8 * var(--u));
  }
  .chain-cols {
    display: flex;
    flex-wrap: nowrap;
    align-items: flex-start;
    gap: calc(8 * var(--u));
  }
  .arrow {
    align-self: center;
    color: var(--text-faint);
    font-size: calc(14 * var(--u));
  }
  .ccol {
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
    flex: none;
  }
  .ccol-h {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
    text-align: center;
  }
  .chain-card {
    width: calc(180 * var(--u));
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
    padding: calc(4 * var(--u)) calc(8 * var(--u));
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
    background: var(--background);
    text-align: left;
  }
  .chain-card.is-anchor {
    border-color: var(--primary);
    background: var(--primary-soft);
  }
  .chain-card.hidden-state {
    border-style: dashed;
    color: var(--muted-foreground);
  }
  .chain-card.sel {
    background: var(--primary-soft);
  }
  .chain-card.sel-here {
    box-shadow: inset calc(3 * var(--u)) 0 0 var(--primary);
  }
  /* D-14: a chain card whose candidate fails the in-view tag filter dims to
     30% on its non-text marks (dot, x-N badge), never a raw opacity on text
     (see EdgeLane's identical rule and the conventions skill's dim rule). */
  .chain-card.fdim .dot,
  .chain-card.fdim .x {
    opacity: 0.3;
  }
  .chain-card.fdim .ch-sum {
    color: var(--muted-foreground);
  }
  .ch-meta {
    display: flex;
    align-items: center;
    gap: calc(4 * var(--u));
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .dot {
    width: calc(6 * var(--u));
    height: calc(6 * var(--u));
    border-radius: 999px;
    flex: none;
  }
  .depth {
    margin-left: auto;
  }
  .ch-sum {
    font-size: calc(12 * var(--u));
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .ch-date {
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
  }
  .x {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .states-foot {
    margin-top: auto;
    display: flex;
    gap: calc(4 * var(--u));
    flex-wrap: wrap;
  }
  .st {
    font-family: var(--font-mono, monospace);
    font-size: calc(10 * var(--u));
    padding: 0 calc(5 * var(--u));
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
    color: var(--text-faint);
  }
</style>
