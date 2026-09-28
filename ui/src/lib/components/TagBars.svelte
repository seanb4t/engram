<script lang="ts">
  // The shared tag popularity list (TAGS-01, D-12, D-13, D-15): one cached
  // ListTags(scope, 1000) query drives both /search's docked panel
  // (mode="panel") and /related's rail Tags tab (mode="rail"). Task 2 adds
  // the top-30/show-all cap, the filter box, the honesty footer (D-15 --
  // ListTags carries no total, so the footer never says "of N"), the
  // popularity/rarity tooltip, marked/selected rows, the roving-listbox
  // keyboard model, and every E4 state (empty/loading/error/populated/
  // partial/overflow).
  import { createQuery } from '@tanstack/svelte-query';
  import { Input } from '$lib/components/ui/input';
  import { Button } from '$lib/components/ui/button';
  import { Skeleton } from '$lib/components/ui/skeleton';
  import { listTagsQuery } from '$lib/tags/query';
  import {
    toTagRows,
    barPercent,
    visibleTagRows,
    tagListFooter,
    scopeLabel,
    tagsLoadingLine,
    tagsErrorCopy
  } from '$lib/tags/tags';
  import { parseConnectError } from '$lib/errors/connect-error';

  let {
    scope,
    mode,
    markedTags,
    selectedTags,
    rarity,
    ontoggle
  }: {
    scope: string;
    mode: 'panel' | 'rail';
    markedTags?: ReadonlySet<string>;
    selectedTags?: ReadonlySet<string>;
    rarity?: ReadonlyMap<string, number>;
    ontoggle: (tag: string) => void;
  } = $props();

  const uid = $props.id();

  const q = createQuery(() => listTagsQuery(scope));

  let filterValue = $state('');
  let expanded = $state(false);
  let activeIndex = $state(0);

  const rows = $derived(toTagRows(q.data));
  const max = $derived(rows.length > 0 ? rows[0].count : 0);
  const visible = $derived(visibleTagRows(rows, { filter: filterValue, expanded }));
  const footer = $derived(
    tagListFooter({
      loaded: rows.length,
      more: q.data?.more ?? false,
      filtered: filterValue.trim() !== '',
      expanded
    })
  );

  $effect(() => {
    if (activeIndex >= visible.rows.length) {
      activeIndex = Math.max(0, visible.rows.length - 1);
    }
  });

  function onListboxKeydown(e: KeyboardEvent) {
    const n = visible.rows.length;
    if (n === 0) return;
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        activeIndex = Math.min(activeIndex + 1, n - 1);
        break;
      case 'ArrowUp':
        e.preventDefault();
        activeIndex = Math.max(activeIndex - 1, 0);
        break;
      case 'Home':
        e.preventDefault();
        activeIndex = 0;
        break;
      case 'End':
        e.preventDefault();
        activeIndex = n - 1;
        break;
      case 'Enter':
      case ' ': {
        e.preventDefault();
        const r = visible.rows[activeIndex];
        if (r) ontoggle(r.tag);
        break;
      }
    }
  }
</script>

<div class="tag-bars" data-mode={mode}>
  {#if mode === 'panel'}
    <div class="header mono">Tags · counts in {scopeLabel(scope)}</div>
  {:else}
    <div class="header-rail mono">counts in {scopeLabel(scope)}</div>
  {/if}

  <Input
    aria-label="Filter tags"
    placeholder="Filter tags…"
    bind:value={filterValue}
  />

  {#if q.isLoading}
    <div class="status-line mono">{tagsLoadingLine(scope)}</div>
    <div class="skeleton-list">
      {#each Array(8) as _, i (i)}
        <Skeleton class="h-[calc(8*var(--u))] w-full" />
      {/each}
    </div>
  {:else if q.isError}
    {@const parsed = parseConnectError(q.error)}
    {@const copy = tagsErrorCopy(parsed)}
    <div class="status-line error">
      <p>{copy.heading}</p>
      {#if copy.kind === 'rejected'}
        <pre class="envelope mono">{copy.envelope}</pre>
      {/if}
      <Button variant="outline" size="sm" onclick={() => q.refetch()}>Retry</Button>
    </div>
  {:else if rows.length === 0}
    <div class="status-line">
      <p>No tags among recall-visible records in {scopeLabel(scope)}.</p>
      <p class="muted">
        Tags on archived, superseded, expired, or not-yet-active records aren't counted — you can
        still type a #tag to filter.
      </p>
    </div>
  {:else if visible.rows.length === 0}
    <div class="status-line muted">No tags match "{filterValue.trim()}" in the loaded set.</div>
  {:else}
    <div
      role="listbox"
      aria-label="Tags by count"
      class="bar-list"
      tabindex="0"
      aria-activedescendant={`${uid}-opt-${activeIndex}`}
      onkeydown={onListboxKeydown}
    >
      {#each visible.rows as row, i (row.tag)}
        {@const rarityVal = rarity?.get(row.tag)}
        {@const title = rarityVal !== undefined ? `count ${row.count}\nrarity ln(n/df) ${rarityVal.toFixed(2)}` : `count ${row.count}`}
        {@const label = rarityVal !== undefined ? `#${row.tag}, count ${row.count}, rarity ${rarityVal.toFixed(2)}` : `#${row.tag}, count ${row.count}`}
        <!-- WAI-ARIA APG listbox: options are NOT tab stops and take no
             keyboard handler of their own -- the container (role="listbox")
             owns all keyboard interaction via aria-activedescendant, and
             click is a supplementary pointer affordance. -->
        <!-- svelte-ignore a11y_interactive_supports_focus -->
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div
          id={`${uid}-opt-${i}`}
          role="option"
          aria-selected={selectedTags?.has(row.tag) ?? false}
          class="bar-row"
          class:marked={markedTags?.has(row.tag) ?? false}
          {title}
          aria-label={label}
          tabindex="-1"
          onclick={() => ontoggle(row.tag)}
        >
          <span class="nm">{#if markedTags?.has(row.tag)}<span class="marker">● </span>{/if}{row.tag}</span>
          <span class="track">
            <span class="fill" style={`width: ${barPercent(row.count, max)}%`}></span>
          </span>
          <span class="cnt">{row.count}</span>
        </div>
      {/each}
    </div>
    {#if !filterValue.trim() && !expanded && visible.hidden > 0}
      <Button variant="ghost" size="sm" class="show-all-btn" onclick={() => (expanded = true)}>show all</Button>
    {/if}
    {#if footer}
      <div class="footer mono" class:warn={footer.warn}>{footer.text}</div>
    {/if}
  {/if}
</div>

<style>
  .tag-bars {
    display: flex;
    flex-direction: column;
    gap: calc(8 * var(--u));
  }
  .header,
  .header-rail {
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
  }
  .bar-list {
    display: flex;
    flex-direction: column;
    overflow-y: auto;
  }
  .bar-row {
    display: grid;
    grid-template-columns: calc(132 * var(--u)) 1fr calc(36 * var(--u));
    gap: calc(8 * var(--u));
    align-items: center;
    cursor: pointer;
  }
  .nm {
    font-family: var(--font-mono, monospace);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .track {
    display: block;
  }
  .fill {
    display: block;
    height: calc(8 * var(--u));
    min-width: 2px;
    background: var(--text-faint);
    border-radius: 0 calc(4 * var(--u)) calc(4 * var(--u)) 0;
  }
  .bar-row.marked .fill {
    background: var(--primary);
  }
  .marker {
    color: var(--primary);
  }
  .cnt {
    font-family: var(--font-mono, monospace);
    text-align: right;
  }
  .status-line {
    font-size: calc(12 * var(--u));
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
  }
  .status-line.error {
    color: var(--destructive);
  }
  .status-line .muted,
  .muted {
    color: var(--text-faint);
  }
  .envelope {
    font-size: calc(11 * var(--u));
    white-space: pre-wrap;
    background: var(--surface-2);
    border: 1px solid var(--destructive);
    border-radius: calc(4 * var(--u));
    padding: calc(6 * var(--u));
  }
  .skeleton-list {
    display: flex;
    flex-direction: column;
    gap: calc(6 * var(--u));
  }
  .footer {
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
  }
  .footer.warn {
    color: var(--warning);
  }
</style>
