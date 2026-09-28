<script lang="ts">
  // The graph legend (D-10): one row per LANE_ORDER type -- the glyph, the
  // type name, its n/cap count, and a Checkbox that is the SAME switch as
  // that lane's hide/show button, sharing one hiddenTypes source of truth
  // with EdgeLane/SupersessionLane. Rendered in the rail after the evidence
  // section.
  import type { LaneType, RelatedModel } from '$lib/related/graph';
  import { LANE_ORDER } from '$lib/related/graph';
  import { LANE_GLYPH, laneCountLabel } from '$lib/related/lanes';
  import { Checkbox } from '$lib/components/ui/checkbox';

  let {
    model,
    hiddenTypes,
    ontoggle
  }: {
    model: RelatedModel;
    hiddenTypes: ReadonlySet<LaneType>;
    ontoggle: (type: LaneType) => void;
  } = $props();
</script>

<div class="legend" role="group" aria-label="Edge type legend">
  {#each LANE_ORDER as type (type)}
    <div class="legend-row">
      <span class="g g-{type}" aria-hidden="true">{LANE_GLYPH[type]}</span>
      <span class="name">{type}</span>
      <span class="cnt">{laneCountLabel(type, model)}</span>
      <Checkbox
        class="legend-cb"
        checked={!hiddenTypes.has(type)}
        onCheckedChange={() => ontoggle(type)}
        aria-label={`Show ${type} edges`}
      />
    </div>
  {/each}
</div>

<style>
  .legend {
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
    padding: calc(8 * var(--u));
    border-top: 1px solid var(--border-subtle);
  }
  .legend-row {
    display: grid;
    grid-template-columns: calc(16 * var(--u)) 1fr auto auto;
    align-items: center;
    gap: calc(8 * var(--u));
    font-size: calc(11 * var(--u));
  }
  .name {
    text-transform: capitalize;
  }
  .cnt {
    font-family: var(--font-mono, monospace);
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
  /* WCAG 2.2 SC 2.5.8 (target size): the checkbox's own h-3.5/w-3.5 (14px)
     falls under the 24 CSS px minimum, and there is no adjacent text label
     to widen the clickable region (unlike FacetStrip's <label> wrapping) --
     min-width/min-height floor the box at 24px regardless of the --u
     scaling, same technique as RowActions.svelte's .ra-btn. */
  :global(.legend-cb) {
    min-width: max(calc(24 * var(--u)), 24px);
    min-height: max(calc(24 * var(--u)), 24px);
  }
</style>
