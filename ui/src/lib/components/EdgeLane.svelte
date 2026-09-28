<script lang="ts">
  // One edge-type lane card (citation, tag or vector -- the supersession
  // lane is its own component, SupersessionLane.svelte): a header (swatch,
  // type, n/cap, caption, hide/show) and a body of one-line rows, each
  // selectable and lit by the ONE shared selection (D-02, D-05). Container
  // query `lane` swaps --cols and drops the evidence cell below 520px.
  import type { Candidate, LaneType, RelatedModel } from '$lib/related/graph';
  import { EDGE_STYLE } from '$lib/related/graph';
  import { emptyLaneReason, formatCosine, formatWeight, laneCaption, laneCountLabel } from '$lib/related/lanes';
  import { memoryStateWords } from '$lib/memorystate';

  let {
    type,
    rows,
    model,
    hidden,
    selectedId,
    selectedLane,
    onselect,
    ontogglehidden
  }: {
    type: LaneType;
    rows: Candidate[];
    model: RelatedModel;
    hidden: boolean;
    selectedId: string | null;
    selectedLane: LaneType | 'graph' | null;
    onselect: (id: string, lane: LaneType) => void;
    ontogglehidden: () => void;
  } = $props();

  const style = $derived(EDGE_STYLE[type]);

  function otherTypes(c: Candidate): LaneType[] {
    return c.types.filter((t) => t !== type);
  }

  function citationPills(c: Candidate): string[] {
    const edge = c.edges.citation;
    if (!edge || edge.evidence.case !== 'citation') return [];
    return edge.evidence.value.sharedCitations.map((ref) => `${ref.kind} ${ref.ref}`);
  }

  function tagPills(c: Candidate): { pills: string[]; sigma: string } {
    const edge = c.edges.tag;
    if (!edge || edge.evidence.case !== 'tag') return { pills: [], sigma: '' };
    return {
      pills: edge.evidence.value.sharedTags.map((t) => `#${t.tag} ${formatWeight(t.weight)}`),
      sigma: formatWeight(edge.evidence.value.tagWeight)
    };
  }

  function vectorScore(c: Candidate): number {
    return c.strength.vector ?? 0;
  }
</script>

<section class="card lane">
  <div class="lane-h">
    <svg class="swatch" width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
      <line
        x1="1"
        y1="8"
        x2="15"
        y2="8"
        stroke="var(--muted-foreground)"
        stroke-width={style.width}
        stroke-dasharray={style.dash || undefined}
        stroke-linecap={style.cap ?? undefined}
      />
    </svg>
    <span class="name">{type}</span>
    <span class="cnt">{laneCountLabel(type, model)}</span>
    <span class="cap">{laneCaption(type, model.k)}</span>
    <button type="button" class="eye" aria-pressed={!hidden} onclick={ontogglehidden}>{hidden ? 'show' : 'hide'}</button>
  </div>
  {#if !hidden}
    <div class="lane-body">
      {#if rows.length === 0}
        <p class="empty">{emptyLaneReason(type, model)}</p>
      {:else}
        {#each rows as c (c.id)}
          {@const states = memoryStateWords(c.memory)}
          <button
            type="button"
            class="row"
            class:sel={selectedId === c.id}
            class:sel-here={selectedId === c.id && selectedLane === type}
            data-testid={`lane-row-${type}-${c.id}`}
            onclick={() => onselect(c.id, type)}
          >
            <span class="id">
              <i class="dot" aria-hidden="true" style="background:var(--cat-{c.memory.category})"></i>{c.shortId}
            </span>
            <span class="sum">{c.memory.summary}</span>
            <span class="row-ev">
              {#if type === 'citation'}
                {#each citationPills(c) as p (p)}<span class="cite">{p}</span>{/each}
              {:else if type === 'tag'}
                {@const t = tagPills(c)}
                {#each t.pills as p (p)}<span class="tagw">{p}</span>{/each}
                {#if t.sigma}<span class="sigma">Σ {t.sigma}</span>{/if}
              {:else if type === 'vector'}
                <span class="vscore">{formatCosine(vectorScore(c))}</span>
                <span class="vbar"><b style="width:{Math.min(100, Math.max(0, vectorScore(c) * 100))}%"></b></span>
              {/if}
            </span>
            <span class="multi">
              {#if c.types.length > 1}
                <span class="x" title={`reached by ${c.types.length} edge types: ${c.types.join(', ')}`}>×{c.types.length}</span>
                {#each otherTypes(c) as t (t)}<span class="g g-{t}">{t[0].toUpperCase()}</span>{/each}
              {/if}
            </span>
            <span class="states">
              {#each states as w (w)}<span class="st">{w}</span>{/each}
            </span>
          </button>
        {/each}
      {/if}
    </div>
  {/if}
</section>

<style>
  .lane {
    container: lane / inline-size;
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
  .lane-body {
    display: flex;
    flex-direction: column;
  }
  .empty {
    padding: calc(8 * var(--u));
    font-size: calc(12 * var(--u));
    color: var(--text-faint);
  }
  .row {
    --cols: calc(96 * var(--u)) minmax(0, 1fr) auto auto auto;
    display: grid;
    grid-template-columns: var(--cols);
    align-items: center;
    gap: calc(8 * var(--u));
    padding: calc(4 * var(--u)) calc(8 * var(--u));
    border-bottom: 1px solid var(--border-subtle);
    text-align: left;
    width: 100%;
  }
  .row:last-child {
    border-bottom: none;
  }
  .row.sel {
    background: var(--primary-soft);
  }
  .row.sel-here {
    box-shadow: inset calc(3 * var(--u)) 0 0 var(--primary);
  }
  .id {
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
    min-width: 0;
    overflow: hidden;
  }
  .dot {
    width: calc(6 * var(--u));
    height: calc(6 * var(--u));
    border-radius: 999px;
    flex: none;
  }
  .sum {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: calc(12 * var(--u));
    min-width: 0;
  }
  .row-ev {
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
  }
  .cite {
    background: var(--surface-2);
    border: 1px dashed var(--border);
    border-radius: calc(4 * var(--u));
    padding: 0 calc(4 * var(--u));
  }
  .tagw {
    background: var(--surface-2);
    border: 1px dotted var(--text-faint);
    border-radius: calc(4 * var(--u));
    padding: 0 calc(4 * var(--u));
  }
  .sigma {
    color: var(--text-faint);
  }
  .vscore {
    color: var(--muted-foreground);
  }
  .vbar {
    width: calc(48 * var(--u));
    height: calc(3 * var(--u));
    border-radius: 2px;
    background: var(--border-subtle);
    overflow: hidden;
    display: inline-block;
  }
  .vbar b {
    display: block;
    height: 100%;
    background: var(--primary);
  }
  .multi {
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
  }
  .x {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .g {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: calc(16 * var(--u));
    height: calc(16 * var(--u));
    font: 600 calc(9 * var(--u)) / 1 var(--font-mono, monospace);
    color: var(--muted-foreground);
    border: 1px solid var(--text-faint);
    border-radius: calc(4 * var(--u));
  }
  .g-supersession {
    border: 2px solid var(--muted-foreground);
  }
  .g-citation {
    border-style: dashed;
  }
  .g-tag {
    border-style: dotted;
    border-width: 2px;
  }
  .g-vector {
    border-color: var(--border);
  }
  .states {
    display: inline-flex;
    gap: calc(4 * var(--u));
    justify-self: end;
  }
  .st {
    font-family: var(--font-mono, monospace);
    font-size: calc(10 * var(--u));
    padding: 0 calc(5 * var(--u));
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
    color: var(--text-faint);
  }
  @container lane (max-width: 520px) {
    .row {
      --cols: calc(96 * var(--u)) minmax(0, 1fr) auto auto;
    }
    .row-ev {
      display: none;
    }
  }
</style>
