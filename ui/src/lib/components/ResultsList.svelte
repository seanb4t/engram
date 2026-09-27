<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { Memory } from '$lib/gen/engram_pb';
  import SvelteVirtualList from '@humanspeak/svelte-virtual-list';
  import { toast } from 'svelte-sonner';
  import { Kbd } from '$lib/components/ui/kbd';
  import ResultRow from './ResultRow.svelte';
  import ResultHoverCard from './ResultHoverCard.svelte';
  import RowActions from './RowActions.svelte';
  import { defaultActionsFor, type CurationAction } from '$lib/curation/host.svelte.ts';

  // D-11/D-12/D-13: a group-header item, or a row item carrying its index
  // into `memories` (never re-derived from the virtual list's own index,
  // which includes headers). `groupKey`/`groupHeader` are consumed by
  // /rules (scope headers) and left unset everywhere else.
  type ListItem =
    | { kind: 'header'; key: string; count: number }
    | { kind: 'row'; memory: Memory; rowIndex: number };

  let {
    memories,
    mode = 'ranked',
    label,
    openId,
    loading = false,
    busy = false,
    hasMore = false,
    selectable = false,
    selectedIds = $bindable<string[]>([]),
    selectionKey,
    onloadmore,
    onopen,
    onescape,
    onedit,
    onvisibility,
    ondelete,
    onsupersede,
    onarchive,
    onrestore,
    onchain,
    rowActions = defaultActionsFor,
    rowTrailing,
    groupKey,
    groupHeader
  }: {
    memories: Memory[];
    mode?: 'ranked' | 'unranked';
    label: string;
    openId?: string;
    loading?: boolean;
    busy?: boolean;
    hasMore?: boolean;
    selectable?: boolean;
    selectedIds?: string[];
    selectionKey?: string;
    onloadmore?: () => void;
    onopen: (id: string) => void;
    onescape?: () => void;
    onedit?: (id: string) => void;
    onvisibility?: (m: Memory) => void;
    ondelete?: (id: string) => void;
    onsupersede?: (ids: string[]) => void;
    onarchive?: (ids: string[]) => void;
    onrestore?: (ids: string[]) => void;
    onchain?: (id: string) => void;
    rowActions?: (m: Memory) => CurationAction[];
    rowTrailing?: Snippet<[Memory]>;
    groupKey?: (m: Memory) => string;
    groupHeader?: Snippet<[string, number]>;
  } = $props();

  // D-12: a header before the first row of each key, in `memories` order —
  // routes pre-sort, this never re-sorts. Without `groupKey`, `items` is
  // `memories` wrapped as row items, unchanged in effect from before this
  // prop existed.
  const items = $derived.by((): ListItem[] => {
    if (!groupKey) return memories.map((memory, rowIndex) => ({ kind: 'row', memory, rowIndex }) as const);
    const out: ListItem[] = [];
    const counts = new Map<string, number>();
    for (const m of memories) counts.set(groupKey(m), (counts.get(groupKey(m)) ?? 0) + 1);
    let lastKey: string | undefined;
    memories.forEach((memory, rowIndex) => {
      const key = groupKey(memory);
      if (key !== lastKey) {
        out.push({ kind: 'header', key, count: counts.get(key) ?? 0 });
        lastKey = key;
      }
      out.push({ kind: 'row', memory, rowIndex });
    });
    return out;
  });

  // Maps a `memories` index to its position in `items` (headers shift every
  // row after the first group forward) — `moveActive` needs this to scroll
  // the VIRTUAL list, which is indexed over `items`, not `memories`.
  function itemIndexForRow(rowIndex: number): number {
    return items.findIndex((it) => it.kind === 'row' && it.rowIndex === rowIndex);
  }

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
  let containerEl: HTMLElement | null = $state(null);
  let listFocused = $state(false);

  // D-05: the row action toolbar's own anchor tracking, kept SEPARATE from
  // the hover-card's `hoverRowId`/timers below — the toolbar has no open
  // delay (it must reveal instantly on hover per the sketch's gradient-fade
  // affordance) and falls back to the keyboard-active row while the list has
  // focus, independent of whether the hover card's own 250ms timer has fired.
  // `toolbarMemory`/its positioning effect are declared further down, once
  // `activeId` exists.
  let pointerRowId = $state<string | undefined>(undefined);
  let toolbarSuppressed = $state(false);
  let toolbarAnchorEl = $state<HTMLElement | null>(null);
  let toolbarRef: HTMLElement | null = $state(null);
  let toolbarCloseTimer: ReturnType<typeof setTimeout> | undefined;

  // Mirrors ResultHoverCard's own close-lifecycle (openCardFor/
  // scheduleCloseCard/cardRef below): the toolbar is a SIBLING overlay
  // painted on top of the row it anchors to, so moving the real pointer from
  // the row onto the toolbar's own buttons fires the row's mouseleave (the
  // browser resolves hover by paint order, not DOM ancestry) — clearing
  // pointerRowId immediately there would unmount the toolbar out from under
  // an in-flight click. A short grace period, cancelled the instant the
  // pointer actually lands on the toolbar, closes that gap.
  function clearToolbarCloseTimer() {
    if (toolbarCloseTimer !== undefined) {
      clearTimeout(toolbarCloseTimer);
      toolbarCloseTimer = undefined;
    }
  }
  function scheduleToolbarClose() {
    clearToolbarCloseTimer();
    toolbarCloseTimer = setTimeout(() => {
      toolbarCloseTimer = undefined;
      pointerRowId = undefined;
    }, 120);
  }

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
    // D-05: the toolbar reveals instantly on hover — no delay, and no
    // early-return on an unchanged hoverRowId, so a scroll-suppressed
    // toolbar re-asserts on the very next mousemove over the same row.
    pointerRowId = m.id;
    toolbarSuppressed = false;
    clearToolbarCloseTimer();
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
    // Scheduled, not immediate (see scheduleToolbarClose's comment) — the
    // pointer may be travelling onto the toolbar itself, which is painted
    // on top of this row but is not its DOM descendant.
    if (pointerRowId === m.id) scheduleToolbarClose();
    if (hoverRowId !== m.id) return;
    hoverRowId = undefined;
    clearOpenTimer();
    scheduleCloseCard();
  }

  // The pointer can travel from the row onto the toolbar's own buttons
  // within the 120ms grace scheduleToolbarClose sets up — cancelled the
  // instant it actually arrives there (mirrors the hover card's cardRef
  // effect below).
  $effect(() => {
    if (!toolbarRef) return;
    const el = toolbarRef;
    function onEnter() {
      clearToolbarCloseTimer();
    }
    function onLeave() {
      scheduleToolbarClose();
    }
    el.addEventListener('mousemove', onEnter);
    el.addEventListener('mouseleave', onLeave);
    return () => {
      el.removeEventListener('mousemove', onEnter);
      el.removeEventListener('mouseleave', onLeave);
    };
  });

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

  // D-05: the toolbar's target row — the pointer-hovered row when there is
  // one, else the keyboard-active row while the list has focus.
  const toolbarMemory = $derived.by(() => {
    if (toolbarSuppressed || memories.length === 0) return undefined;
    const id = pointerRowId ?? (listFocused ? activeId : undefined);
    if (!id) return undefined;
    return memories.find((m) => m.id === id);
  });

  $effect(() => {
    const mem = toolbarMemory;
    if (!mem || !wrapperEl) {
      toolbarAnchorEl = null;
      return;
    }
    toolbarAnchorEl = wrapperEl.querySelector<HTMLElement>(`#opt-${CSS.escape(mem.id)}`);
  });

  // Hides the toolbar on scroll/text-size change, same trigger as the hover
  // card's own effects above — a floating toolbar over a row that just
  // scrolled out from under it (or resized under a text-size step) is
  // exactly the T-04-17 risk this clears.
  $effect(() => {
    if (!viewportEl) return;
    const vp = viewportEl;
    function onScroll() {
      toolbarSuppressed = true;
    }
    vp.addEventListener('scroll', onScroll);
    return () => vp.removeEventListener('scroll', onScroll);
  });
  $effect(() => {
    function onTextSize() {
      toolbarSuppressed = true;
    }
    window.addEventListener('engram:textsize', onTextSize);
    return () => window.removeEventListener('engram:textsize', onTextSize);
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
      // D-12: `list` is indexed over `items` (headers included), not
      // `memories` — map the row index to its item position before scrolling.
      const itemIndex = itemIndexForRow(clamped);
      await list.scroll({ index: itemIndex >= 0 ? itemIndex : clamped, align: 'nearest', smoothScroll: false });
    }
    activeId = memories[clamped]?.id;
    // D-05: keyboard movement re-asserts the toolbar too, clearing any prior
    // scroll/text-size suppression — the active row is a fresh, deliberate
    // target the user just navigated to.
    toolbarSuppressed = false;

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

  // D-02: the range-select anchor. Only `x` sets it — Shift+X and shift+click
  // both extend FROM this same anchor, never reassign it, so repeated range
  // extensions stay relative to the row the operator first toggled.
  let anchorId = $state<string | undefined>(undefined);

  // D-04: selection lifecycle. `selectionKey` is a route-owned digest of
  // "what changed" (see /search's `encodeSearchParams({ ...params, k:
  // DEFAULT_K, sel: '' })`) — a change clears the selection; the SAME key
  // across a Show more (k-only) or a pane open/close (sel-only) or an
  // in-place cache patch leaves it untouched. `lastSelectionKey` is plain
  // (non-reactive) bookkeeping, not $state — it only needs to survive
  // between effect runs, never to trigger one itself.
  let lastSelectionKey: string | undefined;
  $effect(() => {
    const key = selectionKey;
    const first = lastSelectionKey === undefined;
    if (key !== lastSelectionKey) {
      lastSelectionKey = key;
      if (!first) selectedIds = [];
    }
  });

  // Keep the selection honest: drop any id no longer present in `memories`
  // (e.g. after a delete). An in-place cache patch never removes an id from
  // the array, so this never disturbs a live selection's membership.
  $effect(() => {
    if (selectedIds.length === 0) return;
    const present = new Set(memories.map((m) => m.id));
    const pruned = selectedIds.filter((id) => present.has(id));
    if (pruned.length !== selectedIds.length) selectedIds = pruned;
  });

  // x toggles selection membership for one row, keeping `selectedIds` in
  // LIST order (not insertion order) so a/A/S always submit ids in the
  // order the rows appear, not the order they were selected.
  function toggleSelection(id: string) {
    anchorId = id;
    if (selectedIds.includes(id)) {
      selectedIds = selectedIds.filter((x) => x !== id);
    } else {
      const set = new Set(selectedIds);
      set.add(id);
      selectedIds = memories.filter((m) => set.has(m.id)).map((m) => m.id);
    }
  }

  // Shift+X / shift+click: the inclusive range from the anchor to `toId`,
  // added to (not replacing) the current selection.
  function selectRange(toId: string) {
    const anchor = anchorId ?? activeId;
    if (!anchor) return;
    const anchorIdx = memories.findIndex((m) => m.id === anchor);
    const toIdx = memories.findIndex((m) => m.id === toId);
    if (anchorIdx === -1 || toIdx === -1) return;
    const [lo, hi] = anchorIdx <= toIdx ? [anchorIdx, toIdx] : [toIdx, anchorIdx];
    const set = new Set(selectedIds);
    for (let i = lo; i <= hi; i++) set.add(memories[i].id);
    selectedIds = memories.filter((m) => set.has(m.id)).map((m) => m.id);
  }

  // a/A/S targets: the selection (in list order) when non-empty, else the
  // active row alone.
  function selectionTargets(): string[] {
    if (selectedIds.length > 0) {
      const set = new Set(selectedIds);
      return memories.filter((m) => set.has(m.id)).map((m) => m.id);
    }
    const m = memories[activeIndex];
    return m ? [m.id] : [];
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
      case 'x': {
        if (!selectable) break;
        const m = memories[current];
        if (m) toggleSelection(m.id);
        break;
      }
      case 'X': {
        if (!selectable) break;
        const m = memories[current];
        if (m) selectRange(m.id);
        break;
      }
      case 'a': {
        const targets = selectionTargets();
        if (targets.length > 0) onarchive?.(targets);
        break;
      }
      case 'A': {
        const targets = selectionTargets();
        if (targets.length > 0) onrestore?.(targets);
        break;
      }
      case 'S': {
        const targets = selectionTargets();
        if (targets.length > 0) onsupersede?.(targets);
        break;
      }
      case 'Escape':
        // D-03/D-17: close the topmost layer only, selection first. A
        // non-empty selection is the topmost "layer" — clearing it leaves
        // the hover card/pane untouched. Only once the selection is empty
        // does Escape fall through to the existing card-then-onescape tiers.
        if (selectedIds.length > 0) {
          selectedIds = [];
        } else if (cardOpen) {
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

  // D-02: a click on the row-check cell toggles selection (never opens); a
  // shift+click extends the range from the anchor (never opens); otherwise
  // the click opens the row as before.
  function handleOptionClick(event: MouseEvent, id: string) {
    if (selectable) {
      const target = event.target as HTMLElement | null;
      if (target?.closest('.row-check')) {
        toggleSelection(id);
        return;
      }
      if (event.shiftKey) {
        selectRange(id);
        return;
      }
    }
    selectRow(id);
  }

  // ROW-01: showRel is a single list-level flag so every row's column
  // layout agrees — a per-row check would let rows in the SAME list
  // disagree about whether a rel column exists at all.
  const showRel = $derived(memories.some((m) => m.relevance !== undefined));

  const NAV_KEYS = new Set(['j', 'k', 'ArrowDown', 'ArrowUp', 'Home', 'End', 'Enter']);
  // D-16 update (Phase 4): x/X/a/A/S are now bound — x toggles selection, X
  // (Shift+x) extends it as a range from the anchor, a archives, A (Shift+a)
  // restores, S (Shift+s) supersedes the selection (or the active row alone
  // when nothing is selected).
  const ROW_ACTION_KEYS = new Set(['e', 's', '#', 'c', 'C', 'x', 'X', 'a', 'A', 'S']);
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
  function listboxViewport(
    node: HTMLElement,
    params: { label: string; onKey: (key: string) => void; selectable: boolean }
  ) {
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
      if (current.selectable) {
        vp.setAttribute('aria-multiselectable', 'true');
      } else {
        vp.removeAttribute('aria-multiselectable');
      }
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
  <div class="results-list-container" bind:this={containerEl}>
    {#if busy}
      <!-- E1 loading (re-query): previous rows stay, dimmed, with an
           indeterminate progress bar — never a flash to empty. -->
      <div class="loadbar" data-testid="results-loadbar"></div>
    {/if}
    <div
      class="results-listbox-wrapper"
      class:busy
      bind:this={wrapperEl}
      use:listboxViewport={{ label, onKey: handleListboxKey, selectable }}
    >
      <SvelteVirtualList
        bind:this={list}
        {items}
        itemKey={(it: ListItem) => (it.kind === 'header' ? `h:${it.key}` : it.memory.id)}
        defaultEstimatedItemHeight={rowHeightPx}
        bufferSize={10}
        viewportLabel={label}
        {hasMore}
        onLoadMore={onloadmore}
      >
        {#snippet renderItem(it: ListItem)}
          {#if it.kind === 'header'}
            <!-- D-12: a group header — role="presentation" (not an option;
                 j/k/Home/End skip it, aria-activedescendant never names it). -->
            <div role="presentation" class="results-group-header">
              {#if groupHeader}
                {@render groupHeader(it.key, it.count)}
              {:else}
                <span class="rgh-key">{it.key}</span>
                <span class="rgh-count">({it.count})</span>
              {/if}
            </div>
          {:else}
            {@const m = it.memory}
            <!-- WAI-ARIA APG listbox: options are NOT tab stops and take no
                 keyboard handler of their own — the container (role="listbox")
                 owns all keyboard interaction via aria-activedescendant, and
                 click is a supplementary pointer affordance. -->
            <!-- svelte-ignore a11y_interactive_supports_focus -->
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <div
              role="option"
              id={`opt-${m.id}`}
              aria-selected={selectable ? selectedIds.includes(m.id) : it.rowIndex === activeIndex}
              onclick={(e) => handleOptionClick(e, m.id)}
              onmousemove={(e) => handleRowMouseMove(m, e.currentTarget as HTMLElement)}
              onmouseleave={() => handleRowMouseLeave(m)}
            >
              <ResultRow
                memory={m}
                {mode}
                {showRel}
                trailing={rowTrailing}
                active={it.rowIndex === activeIndex}
                opened={m.id === openId}
                {listFocused}
                {selectable}
                selected={selectedIds.includes(m.id)}
                selectionActive={selectable && selectedIds.length > 0}
              />
            </div>
          {/if}
        {/snippet}
      </SvelteVirtualList>
    </div>
    <!-- D-05: the row action toolbar — a sibling of the listbox wrapper, NEVER
         inside renderItem's role="option" row. -->
    {#if toolbarMemory && toolbarAnchorEl && containerEl}
      <RowActions
        memory={toolbarMemory}
        anchor={toolbarAnchorEl}
        container={containerEl}
        actions={rowActions(toolbarMemory)}
        bind:toolbarRef
        {onsupersede}
        {onarchive}
        {onrestore}
        {onchain}
      />
    {/if}
  </div>
  <div class="results-legend">
    <Kbd>j</Kbd><Kbd>k</Kbd> move · <Kbd>↵</Kbd> open / close · <Kbd>esc</Kbd> close
    {#if onedit}
      · <Kbd>e</Kbd> edit
    {/if}
    {#if onvisibility}
      · <Kbd>s</Kbd> share
    {/if}
    {#if ondelete}
      · <Kbd>#</Kbd> delete
    {/if}
    · <Kbd>c</Kbd> copy short_id · <Kbd>⇧C</Kbd> copy id
    {#if selectable}
      · <Kbd>x</Kbd> select · <Kbd>⇧X</Kbd> range
    {/if}
    {#if onsupersede}
      · <Kbd>⇧S</Kbd> supersede
    {/if}
    {#if onarchive}
      · <Kbd>a</Kbd> archive
    {/if}
    {#if onrestore}
      · <Kbd>⇧A</Kbd> restore
    {/if}
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
  /* D-12: a presentation-only group header — never a role="option", so it
     is skipped by j/k/Home/End and never named by aria-activedescendant. */
  .results-group-header {
    display: flex;
    align-items: baseline;
    gap: calc(6 * var(--u));
    height: calc(22 * var(--u));
    padding: 0 calc(14 * var(--u));
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
    background: var(--surface-2);
    border-bottom: 1px solid var(--border-subtle);
  }
  .rgh-key {
    font-family: var(--font-mono, monospace);
    font-weight: 500;
  }
  .rgh-count {
    color: var(--text-faint);
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
