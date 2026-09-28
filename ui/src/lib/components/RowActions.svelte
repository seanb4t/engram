<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import type { CurationAction } from '$lib/curation/host.svelte';
  import ReplaceIcon from '@lucide/svelte/icons/replace';
  import ArchiveIcon from '@lucide/svelte/icons/archive';
  import ArchiveRestoreIcon from '@lucide/svelte/icons/archive-restore';
  import Link2Icon from '@lucide/svelte/icons/link-2';

  // D-05: the hover/focus row action toolbar. Rendered as a SIBLING of the
  // listbox wrapper (never inside a role="option" row — WCAG: a toolbar is
  // not nested inside an option), absolutely positioned over the right edge
  // of `anchor` (the anchored row's own DOM element) within `container` (the
  // list's own position:relative ancestor, ResultsList's
  // `.results-list-container`). T-04-17: the anchored row's id is captured
  // at render via `memory` — the host is responsible for hiding this
  // component (unmounting it) on scroll/text-size change so it never floats
  // over a row it no longer represents.
  let {
    memory,
    anchor,
    container,
    actions,
    onsupersede,
    onarchive,
    onrestore,
    onchain,
    toolbarRef = $bindable<HTMLElement | null>(null)
  }: {
    memory: Memory;
    anchor: HTMLElement;
    container: HTMLElement;
    actions: CurationAction[];
    onsupersede?: (ids: string[]) => void;
    onarchive?: (ids: string[]) => void;
    onrestore?: (ids: string[]) => void;
    onchain?: (id: string) => void;
    toolbarRef?: HTMLElement | null;
  } = $props();

  let top = $state(0);

  function reposition() {
    const a = anchor.getBoundingClientRect();
    const c = container.getBoundingClientRect();
    top = a.top - c.top + a.height / 2;
  }

  $effect(() => {
    // Re-run whenever the anchored element itself changes (a different row,
    // or the same row re-measured after a virtualizer re-render).
    void anchor;
    void container;
    reposition();
  });

  // The container can resize independently of any anchor change (a window
  // resize, a text-size step) — keep the vertical position accurate.
  $effect(() => {
    const el = container;
    const ro = new ResizeObserver(() => reposition());
    ro.observe(el);
    return () => ro.disconnect();
  });
</script>

<div
  class="row-acts"
  role="toolbar"
  aria-label={`Row actions for ${memory.shortId}`}
  style="top:{top}px"
  bind:this={toolbarRef}
>
  {#if actions.includes('supersede') && onsupersede}
    <button
      type="button"
      class="ra-btn"
      aria-label={`Supersede ${memory.shortId}`}
      onclick={() => onsupersede?.([memory.id])}
    >
      <ReplaceIcon size={12} aria-hidden="true" />
      <span>Supersede</span>
    </button>
  {/if}
  {#if actions.includes('archive') && onarchive}
    <button
      type="button"
      class="ra-btn"
      aria-label={`Archive ${memory.shortId}`}
      onclick={() => onarchive?.([memory.id])}
    >
      <ArchiveIcon size={12} aria-hidden="true" />
      <span>Archive</span>
    </button>
  {/if}
  {#if actions.includes('restore') && onrestore}
    <button
      type="button"
      class="ra-btn"
      aria-label={`Restore ${memory.shortId}`}
      onclick={() => onrestore?.([memory.id])}
    >
      <ArchiveRestoreIcon size={12} aria-hidden="true" />
      <span>Restore</span>
    </button>
  {/if}
  {#if actions.includes('chain') && onchain}
    <button type="button" class="ra-btn" aria-label={`Chain ${memory.shortId}`} onclick={() => onchain?.(memory.id)}>
      <Link2Icon size={12} aria-hidden="true" />
      <span>Chain</span>
    </button>
  {/if}
</div>

<style>
  /* Absolutely positioned within ResultsList's position:relative
     `.results-list-container`; `top` is computed in script from the
     anchored row's own rect, `right:0` aligns to the container's edge
     (rows already span the container's full width). */
  .row-acts {
    position: absolute;
    right: 0;
    transform: translateY(-50%);
    z-index: 1;
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
    padding-left: calc(24 * var(--u));
    padding-right: calc(8 * var(--u));
    background: linear-gradient(90deg, transparent, var(--hover) calc(20 * var(--u)));
  }
  .ra-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: calc(3 * var(--u));
    background: none;
    border: none;
    padding: calc(2 * var(--u)) calc(5 * var(--u));
    /* DSYS-03/D-17, WCAG 2.2 SC 2.5.8 (target size): the padded/font-scaled
       box alone falls under 24 CSS px at the smallest text-size preference
       (12px baseline) -- max() keeps the --u scaling everywhere else in
       this console while guaranteeing the 24px floor regardless of size. */
    min-height: max(calc(24 * var(--u)), 24px);
    border-radius: calc(3 * var(--u));
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
    cursor: pointer;
  }
  .ra-btn:hover,
  .ra-btn:focus-visible {
    color: var(--primary);
    background: var(--surface-2);
  }
</style>
