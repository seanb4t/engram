<script lang="ts">
  import * as ScrollArea from '$lib/components/ui/scroll-area';
  import * as Popover from '$lib/components/ui/popover';
  import { Checkbox } from '$lib/components/ui/checkbox';
  import { CATEGORIES } from '$lib/queries';
  import type { SearchParams } from '$lib/search/params';
  import type { ListScopesResponse } from '$lib/gen/engram_pb';
  import ScopeCombobox from './ScopeCombobox.svelte';

  // ROW-05/ROW-06/E4/E5: one horizontally scrollable line of URL-persisted
  // filter chips. Every chip maps to a real SearchMemoriesRequest field —
  // this component owns no navigation; every change flows out through
  // `onchange` and the page (+page.svelte) is what turns that into a URL.
  let {
    params,
    categoryCounts,
    scopes,
    scopesLoading,
    scopesError,
    onchange,
    onretry
  }: {
    params: SearchParams;
    categoryCounts?: Record<string, number>;
    scopes?: ListScopesResponse;
    scopesLoading: boolean;
    scopesError: unknown;
    onchange: (next: Partial<SearchParams>) => void;
    onretry?: () => void;
  } = $props();

  const totalHits = $derived(categoryCounts ? Object.values(categoryCounts).reduce((a, b) => a + b, 0) : undefined);

  function toggleCategory(c: string) {
    const has = params.categories.includes(c);
    onchange({ categories: has ? params.categories.filter((x) => x !== c) : [...params.categories, c] });
  }

  function removeTag(t: string) {
    onchange({ tags: params.tags.filter((x) => x !== t) });
  }

  function toRfc3339Midnight(dateInput: string): string {
    return dateInput ? `${dateInput}T00:00:00Z` : '';
  }

  function fromRfc3339ToDateInput(value: string): string {
    return value ? value.slice(0, 10) : '';
  }

  // UI-REVIEW: a visible scroll cue. The strip fades whichever edge hides
  // chips, re-measured on scroll and whenever the viewport or its content
  // resizes (window width, pane drag, text size, chips added/removed).
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

  let createdOpen = $state(false);
  const createdLabel = $derived(
    params.createdAfter || params.createdBefore
      ? `created ${params.createdAfter ? `after ${fromRfc3339ToDateInput(params.createdAfter)}` : ''}${
          params.createdAfter && params.createdBefore ? ' · ' : ''
        }${params.createdBefore ? `before ${fromRfc3339ToDateInput(params.createdBefore)}` : ''}`
      : 'created window'
  );
</script>

<ScrollArea.Root
  orientation="horizontal"
  type="auto"
  class="facet-strip"
  scrollbarXClasses="data-horizontal:h-1.5"
  bind:viewportRef={viewportEl}
  data-fade-start={fadeStart || undefined}
  data-fade-end={fadeEnd || undefined}
>
  <div class="facet-strip-row">
    <button type="button" class="facet-chip" class:facet-chip-active={params.categories.length === 0} onclick={() => onchange({ categories: [] })}>
      all{totalHits !== undefined ? ` ${totalHits}` : ''}
    </button>

    {#each CATEGORIES as c (c)}
      {@const count = categoryCounts?.[c] ?? 0}
      <button
        type="button"
        class="facet-chip"
        class:facet-chip-active={params.categories.includes(c)}
        class:facet-chip-zero={count === 0}
        style="--cat-color: var(--cat-{c})"
        onclick={() => toggleCategory(c)}
      >
        {c} {count}
      </button>
    {/each}

    <ScopeCombobox
      value={params.scope}
      crossSpine={params.crossSpine}
      {scopes}
      loading={scopesLoading}
      error={scopesError}
      onselect={(s) => onchange({ scope: s, crossSpine: s ? false : true })}
      {onretry}
    />

    {#if !params.scope}
      <label class="facet-chip">
        <Checkbox checked={params.crossSpine} onCheckedChange={(v) => onchange({ crossSpine: !!v })} aria-label="cross_spine" />
        cross_spine
      </label>
    {/if}

    <label class="facet-chip">
      <Checkbox checked={params.includeArchived} onCheckedChange={(v) => onchange({ includeArchived: !!v })} aria-label="include_archived" />
      include_archived
    </label>
    <label class="facet-chip">
      <Checkbox checked={params.includeSuperseded} onCheckedChange={(v) => onchange({ includeSuperseded: !!v })} aria-label="include_superseded" />
      include_superseded
    </label>
    <label class="facet-chip">
      <Checkbox checked={params.includeScheduled} onCheckedChange={(v) => onchange({ includeScheduled: !!v })} aria-label="include_scheduled" />
      include_scheduled
    </label>

    {#each params.tags as t (t)}
      <span class="facet-chip facet-chip-removable" title={t}>
        #{t}
        <button type="button" aria-label={`remove #${t}`} onclick={() => removeTag(t)}>✕</button>
      </span>
    {/each}

    <Popover.Root bind:open={createdOpen}>
      <Popover.Trigger>
        {#snippet child({ props })}
          <button type="button" {...props} class="facet-chip" title={createdLabel}>{createdLabel}</button>
        {/snippet}
      </Popover.Trigger>
      <Popover.Content class="facet-created-popover">
        <label class="facet-created-field">
          after (inclusive)
          <input
            type="date"
            aria-label="created after"
            value={fromRfc3339ToDateInput(params.createdAfter)}
            oninput={(e) => onchange({ createdAfter: toRfc3339Midnight((e.currentTarget as HTMLInputElement).value) })}
          />
        </label>
        <label class="facet-created-field">
          before (exclusive)
          <input
            type="date"
            aria-label="created before"
            value={fromRfc3339ToDateInput(params.createdBefore)}
            oninput={(e) => onchange({ createdBefore: toRfc3339Midnight((e.currentTarget as HTMLInputElement).value) })}
          />
        </label>
        <button type="button" class="facet-created-clear" onclick={() => onchange({ createdAfter: '', createdBefore: '' })}>
          Clear
        </button>
      </Popover.Content>
    </Popover.Root>
  </div>
</ScrollArea.Root>

<style>
  :global(.facet-strip) {
    width: 100%;
    white-space: nowrap;
  }
  /* Edge fades: a mask (alpha only, not a colour) over the viewport on the
     edge(s) still hiding chips. */
  :global(.facet-strip) {
    --facet-fade: calc(32 * var(--u));
  }
  :global(.facet-strip[data-fade-end] [data-slot='scroll-area-viewport']) {
    mask-image: linear-gradient(to right, black calc(100% - var(--facet-fade)), transparent);
  }
  :global(.facet-strip[data-fade-start] [data-slot='scroll-area-viewport']) {
    mask-image: linear-gradient(to right, transparent, black var(--facet-fade));
  }
  :global(.facet-strip[data-fade-start][data-fade-end] [data-slot='scroll-area-viewport']) {
    mask-image: linear-gradient(
      to right,
      transparent,
      black var(--facet-fade),
      black calc(100% - var(--facet-fade)),
      transparent
    );
  }
  .facet-strip-row {
    display: flex;
    align-items: center;
    gap: calc(6 * var(--u));
    padding: calc(6 * var(--u)) calc(14 * var(--u));
  }
  .facet-chip {
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
    height: calc(20 * var(--u));
    border-radius: var(--radius-full, 9999px);
    border: 1px solid var(--border);
    background: var(--surface-2, var(--muted));
    font-size: calc(11 * var(--u));
    padding: 0 calc(8 * var(--u));
    cursor: pointer;
    white-space: nowrap;
    flex: none;
  }
  .facet-chip-active {
    border-color: var(--primary);
    background: var(--primary-soft, color-mix(in srgb, var(--primary) 16%, transparent));
  }
  /* DSYS-03/D-17: opacity-dimmed text on --surface-2 fell below 4.5:1;
     --muted-foreground is the same fix ResultRow's dim cells use. */
  .facet-chip-zero {
    color: var(--muted-foreground);
  }
  .facet-chip-removable button {
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    color: inherit;
  }
  .facet-created-popover {
    display: flex;
    flex-direction: column;
    gap: calc(8 * var(--u));
    padding: calc(10 * var(--u));
  }
  .facet-created-field {
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
    font-size: calc(11 * var(--u));
  }
  .facet-created-clear {
    align-self: flex-start;
    background: none;
    border: none;
    padding: 0;
    color: var(--primary);
    text-decoration: underline;
    cursor: pointer;
    font: inherit;
  }
</style>
