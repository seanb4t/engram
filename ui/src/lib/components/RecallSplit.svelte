<script lang="ts">
  import type { Snippet } from 'svelte';
  import * as Resizable from '$lib/components/ui/resizable';
  import { Button } from '$lib/components/ui/button';
  import XIcon from '@lucide/svelte/icons/x';

  // D-14/Pitfall B: the sketch's pinned, toggle-close, resizable right
  // detail pane. paneforge's keyboardResizeBy is a PERCENTAGE of the group
  // (confirmed via Context7/02-RESEARCH.md "Pitfall B"), never a literal
  // pixel count -- the ~32px step is recomputed from the live container
  // width on every resize, never passed as a literal 32.
  const NARROW_BREAKPOINT = 760;
  const MIN_DETAIL_PX = 280;
  const DEFAULT_DETAIL_PX = 440;
  const KEYBOARD_STEP_PX = 32;
  const MAX_DETAIL_PCT = 72;

  let {
    open,
    onclose,
    autoSaveId,
    list,
    detail
  }: {
    open: boolean;
    onclose: () => void;
    autoSaveId: string;
    list: Snippet;
    detail: Snippet;
  } = $props();

  let containerEl: HTMLElement | undefined = $state();
  let width = $state(0);
  let dragging = $state(false);
  let detailPane: { collapse: () => void; expand: () => void } | undefined = $state();

  const isNarrow = $derived(width > 0 && width < NARROW_BREAKPOINT);
  // Percentage-of-group equivalents of the sketch's pixel constants,
  // re-derived on every resize (never a literal percentage baked in).
  const stepPct = $derived(width > 0 ? (KEYBOARD_STEP_PX / width) * 100 : 5);
  const minPct = $derived(width > 0 ? (MIN_DETAIL_PX / width) * 100 : 20);
  const defaultPct = $derived(width > 0 ? (DEFAULT_DETAIL_PX / width) * 100 : 30);
  const initialDetailPct = $derived(open ? defaultPct : 0);

  $effect(() => {
    if (!containerEl) return;
    const ro = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width;
      if (w) width = w;
    });
    ro.observe(containerEl);
    width = containerEl.getBoundingClientRect().width;
    return () => ro.disconnect();
  });

  // Toggle-close: the host flips `open`; this effect is the imperative
  // bridge to paneforge's collapse()/expand(). Never runs in the narrow
  // (overlay) layout -- there the detail pane isn't part of the PaneGroup
  // at all, so there is nothing to collapse/expand.
  $effect(() => {
    if (!detailPane || isNarrow) return;
    if (open) detailPane.expand();
    else detailPane.collapse();
  });
</script>

<div class="recall-split" bind:this={containerEl}>
  {#if isNarrow}
    <div class="rs-list-full">{@render list()}</div>
    {#if open}
      <div class="rs-overlay">
        <div class="rs-overlay-head">
          <Button variant="ghost" size="icon-sm" aria-label="close detail" onclick={() => onclose()}><XIcon /></Button>
        </div>
        <div class="rs-overlay-body">{@render detail()}</div>
      </div>
    {/if}
  {:else}
    <Resizable.PaneGroup direction="horizontal" {autoSaveId} keyboardResizeBy={stepPct} class="rs-group">
      <Resizable.Pane defaultSize={100 - initialDetailPct} minSize={20} class="rs-list-pane">
        {@render list()}
      </Resizable.Pane>
      <Resizable.Handle onDraggingChange={(v) => (dragging = v)} />
      <Resizable.Pane
        bind:this={detailPane}
        defaultSize={initialDetailPct}
        minSize={minPct}
        maxSize={MAX_DETAIL_PCT}
        collapsible
        collapsedSize={0}
        class="rs-detail-pane {dragging ? 'rs-dragging' : ''}"
      >
        <div class="rs-detail-body">{@render detail()}</div>
      </Resizable.Pane>
    </Resizable.PaneGroup>
  {/if}
</div>

<style>
  .recall-split {
    width: 100%;
    height: 100%;
    min-height: 0;
    position: relative;
    overflow: hidden;
  }

  .rs-list-full {
    width: 100%;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

  .rs-overlay {
    position: absolute;
    inset: 0;
    z-index: 10;
    background: var(--background);
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .rs-overlay-head {
    display: flex;
    justify-content: flex-end;
    padding: calc(6 * var(--u));
    border-bottom: 1px solid var(--border);
    flex: none;
  }
  .rs-overlay-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
  }

  :global(.rs-detail-pane) {
    transition: width 0.15s ease;
    overflow: hidden;
  }
  :global(.rs-detail-pane.rs-dragging) {
    transition: none;
  }

  .rs-detail-body {
    height: 100%;
    min-height: 0;
    min-width: calc(280 * var(--u));
    overflow-y: auto;
  }
</style>
