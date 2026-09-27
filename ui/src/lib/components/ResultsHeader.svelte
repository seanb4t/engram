<script lang="ts">
  import type { HeaderPart } from '$lib/search/recall-header';
  import { Kbd } from '$lib/components/ui/kbd';

  // D-03: the bulk-selection bar. A pure prop the route computes from its
  // own selectedIds/curation state -- this component never counts a
  // selection itself, only renders the shape it is given.
  interface BulkSelection {
    count: number;
    busy?: boolean;
    onsupersede?: () => void;
    onarchive?: () => void;
    onrestore?: () => void;
    onclear: () => void;
  }

  // The honest, one-line results header (D-01, D-05, D-06, ENTRY-03): every
  // clause comes from recall-header.ts's rankedHeaderParts/resolutionLine —
  // this component never invents or paraphrases copy, it only lays parts out.
  let {
    parts,
    scopeHits,
    k,
    busy = false,
    selection
  }: {
    parts: HeaderPart[];
    scopeHits?: { scope: string; hits: number; searched: boolean }[];
    k: number;
    busy?: boolean;
    selection?: BulkSelection;
  } = $props();

  // Task 2 turns the 'scopes' clause into a toggle exposing this list; Task 1
  // renders it plainly since scopeHits is not wired yet.
  let scopesExpanded = $state(false);

  const bulkActive = $derived(!!selection && selection.count > 0);
</script>

<div class="results-header" class:rh-bulk-active={bulkActive} data-k={k}>
  {#if busy}
    <div class="rh-progress" data-testid="results-header-progress"></div>
  {/if}
  {#if bulkActive && selection}
    <!-- D-03: the bulk bar REPLACES the parts line entirely (same outer
         element and height) while the selection is non-empty; it renders
         only the actions the route actually supplied. -->
    <div class="rh-bulk" role="toolbar" aria-label="Bulk actions">
      <strong>{selection.count} selected</strong>
      <span class="rh-bulk-sep">·</span>
      {#if selection.onsupersede}
        <button type="button" class="rh-bulk-action" disabled={selection.busy} onclick={selection.onsupersede}>
          Supersede {selection.count} into one… <Kbd>⇧S</Kbd>
        </button>
        <span class="rh-bulk-sep">·</span>
      {/if}
      {#if selection.onarchive}
        <button type="button" class="rh-bulk-action" disabled={selection.busy} onclick={selection.onarchive}>
          Archive <Kbd>a</Kbd>
        </button>
        <span class="rh-bulk-sep">·</span>
      {/if}
      {#if selection.onrestore}
        <button type="button" class="rh-bulk-action" disabled={selection.busy} onclick={selection.onrestore}>
          Restore <Kbd>⇧A</Kbd>
        </button>
        <span class="rh-bulk-sep">·</span>
      {/if}
      <button type="button" class="rh-bulk-action" onclick={selection.onclear}>clear</button>
    </div>
  {:else}
    <div class="rh-line" aria-live="polite">
      {#each parts as part, i (i)}
        {#if part.kind === 'count'}
          <strong>{part.text}</strong>
        {:else if part.kind === 'scopes' && scopeHits}
          <button type="button" class="rh-scopes-btn" aria-expanded={scopesExpanded} onclick={() => (scopesExpanded = !scopesExpanded)}>
            {part.text}
          </button>
        {:else if part.kind === 'query'}
          for <code class="rh-query" title={part.title}>{part.title}</code>
        {:else if part.kind === 'hidden'}
          <span title={part.title}>{part.text}</span>
        {:else}
          {part.text}
        {/if}
        {#if i < parts.length - 1}{' '}{/if}
      {/each}
    </div>
    {#if scopesExpanded && scopeHits}
      <div class="rh-scopes-list" data-testid="searched-scopes">
        <span class="rh-scopes-heading">searched_scopes:</span>
        {#each scopeHits as sh (sh.scope)}
          <span class="rh-scope-entry" class:rh-scope-unsearched={!sh.searched}>
            {sh.scope} <strong>{sh.hits}</strong> <span class="rh-scope-note">in top {k}</span>
          </span>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .results-header {
    position: relative;
    padding: calc(6 * var(--u)) calc(14 * var(--u));
    border-bottom: 1px solid var(--border-subtle, var(--border));
    flex: none;
  }
  /* D-03: violet tint across the whole header row while the bulk bar is
     showing -- the same visual "mode switch" cue the sketch uses. */
  .results-header.rh-bulk-active {
    background: var(--primary-soft);
  }
  .rh-line {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: calc(12.5 * var(--u));
  }
  /* Same box as .rh-line (same font-size, no extra padding) so swapping
     between them never changes .results-header's height; flex-wrap lets
     the actions drop to a second line below the narrow breakpoint instead
     of truncating. */
  .rh-bulk {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    column-gap: calc(6 * var(--u));
    row-gap: calc(2 * var(--u));
    font-size: calc(12.5 * var(--u));
  }
  .rh-bulk-sep {
    color: var(--text-faint, var(--muted-foreground));
  }
  .rh-bulk-action {
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
    background: none;
    border: none;
    padding: 0;
    color: var(--primary);
    cursor: pointer;
    font: inherit;
    white-space: nowrap;
  }
  .rh-bulk-action:disabled {
    color: var(--text-faint, var(--muted-foreground));
    cursor: not-allowed;
  }
  .rh-query {
    font-family: var(--font-mono, monospace);
    max-width: calc(240 * var(--u));
    display: inline-block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    vertical-align: bottom;
  }
  .rh-scopes-btn {
    background: none;
    border: none;
    padding: 0;
    color: var(--primary);
    text-decoration: underline;
    text-underline-offset: 2px;
    cursor: pointer;
    font: inherit;
  }
  .rh-scopes-list {
    margin-top: calc(4 * var(--u));
    display: flex;
    flex-wrap: wrap;
    gap: calc(4 * var(--u)) calc(10 * var(--u));
    font-size: calc(11 * var(--u));
    font-family: var(--font-mono, monospace);
  }
  .rh-scopes-heading {
    color: var(--text-faint, var(--muted-foreground));
  }
  .rh-scope-unsearched {
    color: var(--warning);
    border-style: dashed;
  }
  .rh-scope-note {
    color: var(--text-faint, var(--muted-foreground));
  }
  .rh-progress {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 2px;
    overflow: hidden;
    background: transparent;
  }
  .rh-progress::after {
    content: '';
    position: absolute;
    width: 30%;
    height: 100%;
    background: var(--primary);
    animation: rh-progress-anim 0.9s ease-in-out infinite;
  }
  @keyframes rh-progress-anim {
    0% {
      left: -30%;
    }
    100% {
      left: 100%;
    }
  }
</style>
