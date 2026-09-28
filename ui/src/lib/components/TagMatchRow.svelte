<script lang="ts">
  // One ranked match row inside TagCombobox's popover and the header search
  // Tags group (D-16, D-18). Renders every part of the tag as a text node --
  // the matched substring is wrapped in <b>, never raw-HTML interpolation.
  import { barPercent, type TagMatch } from '$lib/tags/tags';

  let { match, max }: { match: TagMatch; max: number } = $props();
</script>

<div class="opt">
  <span class="nm" title={`#${match.tag}`}>
    #{#each match.parts as part, i (i)}{#if part.hit}<b>{part.text}</b>{:else}{part.text}{/if}{/each}
  </span>
  <span class="mini-track">
    <span class="mini" style={`width: ${barPercent(match.count, max)}%`}></span>
  </span>
  <span class="cnt">{match.count}</span>
</div>

<style>
  .opt {
    display: grid;
    grid-template-columns: minmax(0, 1fr) calc(80 * var(--u)) calc(44 * var(--u));
    gap: calc(8 * var(--u));
    align-items: center;
    min-height: calc(28 * var(--u));
  }
  .nm {
    font-family: var(--font-mono, monospace);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .nm b {
    color: var(--primary);
  }
  .mini-track {
    display: block;
  }
  .mini {
    display: block;
    height: calc(4 * var(--u));
    background: var(--text-faint);
    opacity: 0.6;
  }
  .cnt {
    font-family: var(--font-mono, monospace);
    text-align: right;
  }
</style>
