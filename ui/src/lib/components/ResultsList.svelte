<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import SvelteVirtualList from '@humanspeak/svelte-virtual-list';
  import { toast } from 'svelte-sonner';
  import { Kbd } from '$lib/components/ui/kbd';
  import ResultRow from './ResultRow.svelte';
  import ResultHoverCard from './ResultHoverCard.svelte';

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
  let wrapperEl: HTMLElement | null = $state(null);
  let listFocused = $state(false);

  // ROW-02: the hover card's own state, kept SEPARATE from activeIndex/
  // activeId (Pitfall 4) — a mouse resting on a different row than the
  // keyboard-active one must never mutate aria-activedescendant, and moving
  // the keyboard selection must never be gated on pointer position.
  let cardOpen = $state(false);
  let cardMemory = $state<Memory | undefined>(undefined);
  let cardAnchor = $state<HTMLElement | null>(null);
  let cardRef = $state<HTMLElement | null>(null);
  let hoverRowId: string | undefined;
  let openTimer: ReturnType<typeof setTimeout> | undefined;
  let closeTimer: ReturnType<typeof setTimeout> | undefined;

  function clearOpenTimer() {
    if (openTimer !== undefined) {
      clearTimeout(openTimer);
      openTimer = undefined;
    }
  }
  function clearCloseTimer() {
    if (closeTimer !== undefined) {
      clearTimeout(closeTimer);
      closeTimer = undefined;
    }
  }

  function closeCard() {
    clearOpenTimer();
    clearCloseTimer();
    cardOpen = false;
  }

  // Never for the row already open in the pane (ROW-02) — redundant with the
  // pane's own content, and the pane already owns keyboard focus intent.
  function openCardFor(m: Memory, anchorEl: HTMLElement) {
    if (m.id === openId) {
      closeCard();
      return;
    }
    cardMemory = m;
    cardAnchor = anchorEl;
    cardOpen = true;
  }

  function scheduleCloseCard() {
    clearCloseTimer();
    closeTimer = setTimeout(() => {
      closeTimer = undefined;
      closeCard();
    }, 120);
  }

  // Pointer hover tracks by mousemove, NOT mouseenter, per D-07/ROW-02 — a
  // mouseenter fires once per row-enter and would miss the case of the
  // pointer resting still while the row underneath it changes via keyboard
  // scroll; mousemove keeps re-asserting hover intent on real pointer motion.
  function handleRowMouseMove(m: Memory, rowEl: HTMLElement) {
    if (hoverRowId === m.id) return;
    hoverRowId = m.id;
    clearCloseTimer();
    clearOpenTimer();
    openTimer = setTimeout(() => {
      openTimer = undefined;
      if (hoverRowId === m.id) openCardFor(m, rowEl);
    }, 250);
  }

  function handleRowMouseLeave(m: Memory) {
    if (hoverRowId !== m.id) return;
    hoverRowId = undefined;
    clearOpenTimer();
    scheduleCloseCard();
  }

  // openId changing means the pane just opened/switched/closed — the hover
  // card must not linger over stale state either way.
  $effect(() => {
    void openId;
    closeCard();
  });

  // Hides on viewport scroll (a still card anchored to a row that just
  // scrolled out from under it reads as broken) and on a text-size change
  // (the anchor's on-screen geometry just changed under it).
  $effect(() => {
    if (!viewportEl) return;
    const vp = viewportEl;
    function onScroll() {
      closeCard();
    }
    vp.addEventListener('scroll', onScroll);
    return () => vp.removeEventListener('scroll', onScroll);
  });
  $effect(() => {
    function onTextSize() {
      closeCard();
    }
    window.addEventListener('engram:textsize', onTextSize);
    return () => window.removeEventListener('engram:textsize', onTextSize);
  });

  // The pointer can travel INTO the card within a 120ms grace — cleared the
  // instant it actually arrives there, closed on the same delayed schedule
  // as leaving a row if it does not.
  $effect(() => {
    if (!cardRef) return;
    const el = cardRef;
    function onEnter() {
      clearCloseTimer();
    }
    function onLeave() {
      scheduleCloseCard();
    }
    el.addEventListener('mousemove', onEnter);
    el.addEventListener('mouseleave', onLeave);
    return () => {
      el.removeEventListener('mousemove', onEnter);
      el.removeEventListener('mouseleave', onLeave);
    };
  });

  async function copyToClipboard(text: string, message: string) {
    try {
      await navigator.clipboard.writeText(text);
      toast.success(message);
    } catch {
      toast.error('copy failed');
    }
  }

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

    // ROW-02: keyboard movement opens the hover card INSTANTLY for the new
    // active row (no 250ms pointer delay) — overriding any pending
    // pointer-driven timer, since the keyboard just asserted a new intent.
    hoverRowId = activeId;
    clearOpenTimer();
    clearCloseTimer();
    const m = memories[clamped];
    if (m) {
      const rowEl = wrapperEl?.querySelector<HTMLElement>(`#opt-${CSS.escape(m.id)}`) ?? null;
      if (rowEl) openCardFor(m, rowEl);
      else closeCard();
    }
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
        // D-17/foundations.md Esc layering: close the topmost layer only.
        // The hover card sits above the pane in z-order, so it closes first;
        // onescape (the host's own "close the pane" handler) fires only on a
        // SECOND Esc once no card is open.
        if (cardOpen) {
          closeCard();
        } else {
          onescape?.();
        }
        break;
      case 'e': {
        const m = memories[current];
        if (m && m.category !== 'rule' && m.category !== 'discovery') onedit?.(m.id);
        break;
      }
      case 's': {
        const m = memories[current];
        if (m && m.category !== 'rule') onvisibility?.(m);
        break;
      }
      case '#': {
        const m = memories[current];
        if (m) ondelete?.(m.id);
        break;
      }
      case 'c': {
        const m = memories[current];
        if (m) void copyToClipboard(m.shortId, 'copied short_id');
        break;
      }
      case 'C': {
        const m = memories[current];
        if (m) void copyToClipboard(m.id, 'copied id');
        break;
      }
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

  const NAV_KEYS = new Set(['j', 'k', 'ArrowDown', 'ArrowUp', 'Home', 'End', 'Enter']);
  // D-16: no binding for supersede or archive — reserved, not bound, until
  // their RPCs land (Phase 3).
  const ROW_ACTION_KEYS = new Set(['e', 's', '#', 'c', 'C']);
  const HANDLED_KEYS = new Set([...NAV_KEYS, ...ROW_ACTION_KEYS, 'Escape']);

  function isTypingTarget(target: EventTarget | null): boolean {
    if (!(target instanceof HTMLElement)) return false;
    const tag = target.tagName;
    return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable;
  }

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
      // D-17: row-action keys are ignored while a text field has focus, so
      // typing in a search box (or any input) never fires a row action.
      // Navigation keys are unaffected — j/k/Home/End/Enter never had this
      // restriction and nothing in this plan asks for one.
      if (ROW_ACTION_KEYS.has(event.key) && isTypingTarget(event.target)) return;
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
      bind:this={wrapperEl}
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
            onmousemove={(e) => handleRowMouseMove(m, e.currentTarget as HTMLElement)}
            onmouseleave={() => handleRowMouseLeave(m)}
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
  <div class="results-legend">
    <Kbd>j</Kbd><Kbd>k</Kbd> move · <Kbd>↵</Kbd> open / close · <Kbd>esc</Kbd> close · <Kbd>e</Kbd> edit ·
    <Kbd>s</Kbd> share · <Kbd>#</Kbd> delete · <Kbd>c</Kbd> copy short_id · <Kbd>⇧C</Kbd> copy id
  </div>
{/if}

{#if cardOpen && cardMemory && cardAnchor}
  <ResultHoverCard memory={cardMemory} anchor={cardAnchor} bind:open={cardOpen} bind:cardRef />
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
  .results-legend {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: calc(4 * var(--u));
    padding: calc(6 * var(--u)) calc(14 * var(--u));
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
    border-top: 1px solid var(--border-subtle);
    flex: none;
  }
  .results-listbox-wrapper {
    height: 100%;
    min-height: 0;
    transition: opacity 0.15s ease;
  }
  .results-listbox-wrapper.busy {
    opacity: 0.55;
  }
  /* WR-01 fix: applyRole() rewrites the vendored viewport's role from
     "region" to "listbox", which un-matches @humanspeak/svelte-virtual-list's
     own scoped `[role='region'][tabindex='0']:focus-visible` rule — restore
     an equivalent ring for the rewritten role so a keyboard user tabbing to
     or focusing the results list still gets a visible indicator. */
  .results-listbox-wrapper :global([role='listbox']:focus-visible) {
    outline: 2px solid currentColor;
    outline-offset: -2px;
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
