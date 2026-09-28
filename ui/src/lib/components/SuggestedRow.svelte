<script lang="ts">
  import { tick } from 'svelte';
  import * as ScrollArea from '$lib/components/ui/scroll-area';
  import type { FilterSuggestion } from '$lib/gen/engram_pb';
  import type { SearchParams } from '$lib/search/params';
  import { suggestionLabel, suggestionKey, acceptPartial } from '$lib/search/understand';

  // NLQ-03/D-11/D-12/D-13: the Suggested row — unapplied, dashed chips that
  // accept through the page's own FacetStrip onchange path, dismiss
  // (pointer-only) per query, and a roving-tabindex toolbar keyboard model
  // (WAI-ARIA APG Toolbar pattern) over the accept controls only.
  let {
    suggestions,
    params,
    onchange,
    ondismiss,
    onempty
  }: {
    suggestions: FilterSuggestion[];
    params: SearchParams;
    onchange: (partial: Partial<SearchParams>) => void;
    ondismiss: (key: string) => void;
    onempty?: () => void;
  } = $props();

  // Roving tabindex: exactly one accept control is a Tab stop at a time.
  let active = $state(0);
  $effect(() => {
    if (active >= suggestions.length) active = Math.max(0, suggestions.length - 1);
  });

  let acceptEls = $state<(HTMLButtonElement | null)[]>([]);

  function focusAt(i: number) {
    active = i;
    acceptEls[i]?.focus();
  }

  function accept(s: FilterSuggestion) {
    const wasLast = suggestions.length <= 1;
    onchange(acceptPartial(s, params));
    if (wasLast) onempty?.();
  }

  function dismiss(s: FilterSuggestion, i: number, focusAfter: boolean) {
    const key = suggestionKey(s);
    const remaining = suggestions.length - 1;
    ondismiss(key);
    if (remaining <= 0) {
      onempty?.();
    } else if (focusAfter) {
      const nextIndex = Math.min(i, remaining - 1);
      tick().then(() => focusAt(nextIndex));
    }
  }

  function onAcceptKeydown(e: KeyboardEvent, i: number) {
    const n = suggestions.length;
    switch (e.key) {
      case 'ArrowRight':
        e.preventDefault();
        focusAt(Math.min(i + 1, n - 1));
        break;
      case 'ArrowLeft':
        e.preventDefault();
        focusAt(Math.max(i - 1, 0));
        break;
      case 'Home':
        e.preventDefault();
        focusAt(0);
        break;
      case 'End':
        e.preventDefault();
        focusAt(n - 1);
        break;
      case 'Delete':
      case 'Backspace':
        e.preventDefault();
        dismiss(suggestions[i], i, true);
        break;
      // Enter/Space: no custom handler — the native <button> click already
      // fires onclick for both keys.
    }
  }

  // Edge-fade: identical ScrollArea mechanism to FacetStrip's own strip.
  let viewportEl: HTMLElement | null = $state(null);
  let fadeStart = $state(false);
  let fadeEnd = $state(false);

  function measureFade() {
    if (!viewportEl) return;
    const max = viewportEl.scrollWidth - viewportEl.clientWidth;
    fadeStart = viewportEl.scrollLeft > 1;
    fadeEnd = viewportEl.scrollLeft < max - 1;
  }

  $effect(() => {
    const el = viewportEl;
    if (!el) return;
    const ro = new ResizeObserver(() => measureFade());
    ro.observe(el);
    if (el.firstElementChild) ro.observe(el.firstElementChild);
    el.addEventListener('scroll', measureFade, { passive: true });
    measureFade();
    return () => {
      ro.disconnect();
      el.removeEventListener('scroll', measureFade);
    };
  });
</script>

<ScrollArea.Root
  orientation="horizontal"
  type="auto"
  class="suggested-scroll"
  scrollbarXClasses="data-horizontal:h-1.5"
  bind:viewportRef={viewportEl}
  data-fade-start={fadeStart || undefined}
  data-fade-end={fadeEnd || undefined}
>
  <div class="suggested-row" role="toolbar" aria-label="Suggested filters">
    <span class="suggested-caption" aria-hidden="true">Suggested</span>
    {#each suggestions as s, i (suggestionKey(s))}
      {@const label = suggestionLabel(s)}
      <div class="suggested-chip">
        {#if s.kind.case === 'category'}
          <span class="suggested-dot" aria-hidden="true" style={`background: var(--cat-${s.kind.value})`}></span>
        {/if}
        <button
          type="button"
          class="suggested-accept"
          bind:this={acceptEls[i]}
          tabindex={i === active ? 0 : -1}
          aria-label={`Suggested filter, not applied: ${label}`}
          title={label}
          onkeydown={(e) => onAcceptKeydown(e, i)}
          onclick={() => accept(s)}
        >
          <span class="suggested-label">{label}</span>
        </button>
        <button
          type="button"
          class="suggested-dismiss"
          tabindex="-1"
          aria-label={`Dismiss suggested filter: ${label}`}
          onclick={() => dismiss(s, i, false)}
        >
          ×
        </button>
      </div>
    {/each}
  </div>
</ScrollArea.Root>

<style>
  :global(.suggested-scroll) {
    width: 100%;
    white-space: nowrap;
    --facet-fade: calc(32 * var(--u));
  }
  :global(.suggested-scroll[data-fade-end] [data-slot='scroll-area-viewport']) {
    mask-image: linear-gradient(to right, black calc(100% - var(--facet-fade)), transparent);
  }
  :global(.suggested-scroll[data-fade-start] [data-slot='scroll-area-viewport']) {
    mask-image: linear-gradient(to right, transparent, black var(--facet-fade));
  }
  :global(.suggested-scroll[data-fade-start][data-fade-end] [data-slot='scroll-area-viewport']) {
    mask-image: linear-gradient(
      to right,
      transparent,
      black var(--facet-fade),
      black calc(100% - var(--facet-fade)),
      transparent
    );
  }
  .suggested-row {
    display: flex;
    align-items: center;
    gap: calc(6 * var(--u));
    padding: calc(6 * var(--u)) calc(14 * var(--u));
    animation: suggested-in 0.15s ease;
  }
  @keyframes suggested-in {
    from {
      opacity: 0;
      transform: translateY(calc(-2 * var(--u)));
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
  .suggested-caption {
    font-size: calc(11 * var(--u));
    color: var(--text-faint, var(--muted-foreground));
    flex: none;
  }
  .suggested-chip {
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
    height: calc(20 * var(--u));
    border-radius: calc(9999 * var(--u));
    border: 1px dashed var(--border);
    background: transparent;
    font-size: calc(11 * var(--u));
    padding: 0 calc(8 * var(--u));
    white-space: nowrap;
    flex: none;
    max-width: calc(220 * var(--u));
  }
  .suggested-chip:hover,
  .suggested-chip:has(:focus-visible) {
    border-color: var(--primary);
  }
  .suggested-chip:has(:focus-visible) {
    box-shadow: 0 0 0 calc(3 * var(--u)) var(--primary-soft);
  }
  .suggested-accept {
    display: inline-flex;
    align-items: center;
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    color: inherit;
    cursor: pointer;
    min-width: 0;
  }
  .suggested-accept:focus-visible {
    outline: none;
  }
  .suggested-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .suggested-dot {
    display: inline-block;
    width: calc(6 * var(--u));
    height: calc(6 * var(--u));
    border-radius: 50%;
    flex: none;
  }
  .suggested-dismiss {
    display: inline-flex;
    align-items: center;
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    color: var(--text-faint);
    cursor: pointer;
  }
  .suggested-dismiss:hover,
  .suggested-dismiss:focus-visible {
    color: var(--foreground);
  }
</style>
