<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import SvelteVirtualList from '@humanspeak/svelte-virtual-list';
  import ResultRow from './ResultRow.svelte';

  let {
    memories,
    mode = 'ranked',
    label,
    openId,
    loading = false,
    busy = false,
    hasMore = false,
    onloadmore,
    onopen,
    onescape,
    onedit,
    onvisibility,
    ondelete
  }: {
    memories: Memory[];
    mode?: 'ranked' | 'unranked';
    label: string;
    openId?: string;
    loading?: boolean;
    busy?: boolean;
    hasMore?: boolean;
    onloadmore?: () => void;
    onopen: (id: string) => void;
    onescape?: () => void;
    onedit?: (id: string) => void;
    onvisibility?: (m: Memory) => void;
    ondelete?: (id: string) => void;
  } = $props();

  // D-07 / Pitfall A: @humanspeak/svelte-virtual-list's own viewport is a
  // hardcoded role="region" — there is no prop to change it. The
  // `listboxViewport` action below rewrites that element's role to
  // "listbox" after mount and keeps it in sync across re-renders. This is
  // resolution (1) from the research spike (role-rewrite action); it held
  // under the accessibility-role test below (no scroll/re-render undid it),
  // so resolution (2) (inner-wrapper listbox) was not needed.
  let list: ReturnType<typeof SvelteVirtualList> | undefined = $state();
  let viewportEl: HTMLElement | null = $state(null);
  let listFocused = $state(false);

  // Active row tracked BY ID, not index — this is what lets a memories-array
  // change (re-query, re-sort-free re-render) keep the same row active when
  // it is still present, and reset to the first row when it is not.
  // Left undefined at declaration (not `memories[0]?.id`, which the Svelte
  // compiler flags as only-capturing-the-initial-value): the reset $effect
  // below runs immediately on mount too, and since no memory has an
  // undefined id it unconditionally seeds activeId to the first row then.
  let activeId = $state<string | undefined>(undefined);

  const activeIndex = $derived.by(() => {
    if (memories.length === 0) return -1;
    const idx = memories.findIndex((m) => m.id === activeId);
    return idx >= 0 ? idx : 0;
  });

  // Reset to the first row whenever the current active id is no longer in
  // the array (covers both "array replaced" and "array shrank" cases).
  $effect(() => {
    if (memories.length === 0) {
      activeId = undefined;
      return;
    }
    if (!memories.some((m) => m.id === activeId)) {
      activeId = memories[0]?.id;
    }
  });

  // Keep the viewport's aria-activedescendant in sync with the active row,
  // independent of the listboxViewport action's own (rarer) update calls —
  // activeId changes far more often than the label/role setup does.
  $effect(() => {
    const id = activeId;
    if (!viewportEl) return;
    if (id) {
      viewportEl.setAttribute('aria-activedescendant', `opt-${id}`);
    } else {
      viewportEl.removeAttribute('aria-activedescendant');
    }
  });

  // Row height in px at the CURRENT text-size preference (D-13): 28 * var(--u)
  // evaluated against the live root font-size, so the virtualizer's initial
  // estimate is never a stale 13px-basis guess. Recomputed on engram:textsize
  // so a mid-session size change re-seeds the estimate for freshly measured rows.
  function computeRowHeightPx(): number {
    const rootFontSizePx = parseFloat(getComputedStyle(document.documentElement).fontSize) || 15;
    return (28 * rootFontSizePx) / 13;
  }
  let rowHeightPx = $state(typeof document !== 'undefined' ? computeRowHeightPx() : 28);
  $effect(() => {
    function onTextSize() {
      rowHeightPx = computeRowHeightPx();
    }
    window.addEventListener('engram:textsize', onTextSize);
    return () => window.removeEventListener('engram:textsize', onTextSize);
  });

  async function moveActive(targetIndex: number) {
    if (memories.length === 0) return;
    const clamped = Math.max(0, Math.min(memories.length - 1, targetIndex));
    if (list) {
      await list.scroll({ index: clamped, align: 'nearest', smoothScroll: false });
    }
    activeId = memories[clamped]?.id;
  }

  function handleListboxKey(key: string) {
    const current = activeIndex;
    switch (key) {
      case 'j':
      case 'ArrowDown':
        void moveActive(current + 1);
        break;
      case 'k':
      case 'ArrowUp':
        void moveActive(current - 1);
        break;
      case 'Home':
        void moveActive(0);
        break;
      case 'End':
        void moveActive(memories.length - 1);
        break;
      case 'Enter': {
        const m = memories[current];
        if (m) onopen(m.id);
        break;
      }
      case 'Escape':
        onescape?.();
        break;
    }
  }

  function selectRow(id: string) {
    activeId = id;
    onopen(id);
  }

  // ROW-01: showRel is a single list-level flag so every row's column
  // layout agrees — a per-row check would let rows in the SAME list
  // disagree about whether a rel column exists at all.
  const showRel = $derived(memories.some((m) => m.relevance !== undefined));

  const HANDLED_KEYS = new Set(['j', 'k', 'ArrowDown', 'ArrowUp', 'Home', 'End', 'Enter']);

  // Svelte action: resolves Pitfall A and owns the listbox keydown model.
  // Attached to the WRAPPER (an ancestor of the library's own viewport
  // element), listening in the CAPTURE phase — this is deliberate, not
  // incidental: a keydown listener added to the viewport element itself
  // would run AFTER the library's own onkeydown (same-element listeners
  // fire in registration order regardless of capture flag; ours would be
  // registered later, at action-mount time). Listening on an ANCESTOR with
  // capture:true runs BEFORE the event reaches the library's target element,
  // so stopPropagation() here prevents the library's own Home/End/Arrow
  // native-scroll handling from ever firing for keys we own.
  function listboxViewport(node: HTMLElement, params: { label: string; onKey: (key: string) => void }) {
    let current = params;

    function findViewport(): HTMLElement | null {
      return node.querySelector('[data-svl-viewport]');
    }

    function onFocus() {
      listFocused = true;
    }
    function onBlur() {
      listFocused = false;
    }

    function applyRole() {
      const vp = findViewport();
      if (!vp) return;
      if (vp !== viewportEl) {
        vp.addEventListener('focus', onFocus);
        vp.addEventListener('blur', onBlur);
      }
      vp.setAttribute('role', 'listbox');
      vp.setAttribute('aria-label', current.label);
      viewportEl = vp;
    }

    applyRole();
    // The virtual list's viewport can mount a tick after this action runs
    // (child component instantiation order) — retry once on the next frame
    // so the role rewrite still lands before any test/user interaction.
    if (!viewportEl) {
      requestAnimationFrame(applyRole);
    }

    function onKeydown(event: KeyboardEvent) {
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      if (!HANDLED_KEYS.has(event.key)) return;
      event.preventDefault();
      event.stopPropagation();
      current.onKey(event.key);
    }

    node.addEventListener('keydown', onKeydown, true);

    return {
      update(newParams: typeof params) {
        current = newParams;
        applyRole();
      },
      destroy() {
        node.removeEventListener('keydown', onKeydown, true);
        viewportEl?.removeEventListener('focus', onFocus);
        viewportEl?.removeEventListener('blur', onBlur);
      }
    };
  }
</script>

{#if loading && memories.length === 0}
  <!-- E1 loading (first load only): skeleton rows shaped like the real grid. -->
  <div class="skeleton-rows" data-testid="results-loading">
    {#each Array.from({ length: 8 }) as _, i (i)}
      <div class="skeleton-row">
        <div class="sk sk-cat"></div>
        <div class="sk sk-sum"></div>
        <div class="sk sk-score"></div>
      </div>
    {/each}
  </div>
{:else if memories.length > 0}
  <div class="results-list-container">
    {#if busy}
      <!-- E1 loading (re-query): previous rows stay, dimmed, with an
           indeterminate progress bar — never a flash to empty. -->
      <div class="loadbar" data-testid="results-loadbar"></div>
    {/if}
    <div
      class="results-listbox-wrapper"
      class:busy
      use:listboxViewport={{ label, onKey: handleListboxKey }}
    >
      <SvelteVirtualList
        bind:this={list}
        items={memories}
        itemKey={(m) => m.id}
        defaultEstimatedItemHeight={rowHeightPx}
        bufferSize={10}
        viewportLabel={label}
        {hasMore}
        onLoadMore={onloadmore}
      >
        {#snippet renderItem(m: Memory, index: number)}
          <!-- WAI-ARIA APG listbox: options are NOT tab stops and take no
               keyboard handler of their own — the container (role="listbox")
               owns all keyboard interaction via aria-activedescendant, and
               click is a supplementary pointer affordance. -->
          <!-- svelte-ignore a11y_interactive_supports_focus -->
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <div
            role="option"
            id={`opt-${m.id}`}
            aria-selected={index === activeIndex}
            onclick={() => selectRow(m.id)}
          >
            <ResultRow
              memory={m}
              {mode}
              {showRel}
              active={index === activeIndex}
              opened={m.id === openId}
              {listFocused}
            />
          </div>
        {/snippet}
      </SvelteVirtualList>
    </div>
  </div>
{/if}

<style>
  .results-list-container {
    height: 100%;
    min-height: 0;
    position: relative;
    /* Column drop-out (ResultRow's @container list queries) reads THIS
       container's inline size, not the viewport's. */
    container-type: inline-size;
    container-name: list;
  }
  .results-listbox-wrapper {
    height: 100%;
    min-height: 0;
    transition: opacity 0.15s ease;
  }
  .results-listbox-wrapper.busy {
    opacity: 0.55;
  }
  .loadbar {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 2px;
    z-index: 1;
    overflow: hidden;
    background: transparent;
  }
  .loadbar::after {
    content: '';
    position: absolute;
    width: 30%;
    height: 100%;
    background: var(--primary);
    animation: results-loadbar 0.9s ease-in-out infinite;
  }
  @keyframes results-loadbar {
    0% {
      left: -30%;
    }
    100% {
      left: 100%;
    }
  }

  .skeleton-rows {
    display: flex;
    flex-direction: column;
  }
  .skeleton-row {
    display: grid;
    grid-template-columns: calc(96 * var(--u)) minmax(0, 1fr) calc(70 * var(--u));
    align-items: center;
    column-gap: calc(10 * var(--u));
    height: calc(28 * var(--u));
    padding: 0 calc(12 * var(--u)) 0 calc(14 * var(--u));
  }
  .sk {
    background: var(--muted);
    border-radius: calc(3 * var(--u));
    animation: results-skeleton-pulse 1.5s ease-in-out infinite;
  }
  .sk-cat {
    height: calc(10 * var(--u));
    width: calc(56 * var(--u));
  }
  .sk-sum {
    height: calc(10 * var(--u));
  }
  .sk-score {
    height: calc(10 * var(--u));
    width: calc(40 * var(--u));
  }
  @keyframes results-skeleton-pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.5;
    }
  }
</style>
