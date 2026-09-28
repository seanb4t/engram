<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import * as HoverCard from '$lib/components/ui/hover-card';
  import { memoryStateWords } from '$lib/memorystate';
  import { fullTimestamp, relativeTime } from '$lib/time';
  import { stripCategoryPrefix } from '$lib/summary';

  // ROW-02: the overlay hover card. `open` is driven entirely by the HOST
  // (ResultsList's own pointer/keyboard state machine) — never by this
  // primitive's own hover detection (Pattern 3, Pitfall 4) — and `anchor` is
  // the row DOM node to position under. `customAnchor` is what lets a
  // keyboard-driven open (no real pointer hover on that row) still anchor
  // correctly to the active row's element.
  let {
    memory,
    anchor,
    open = $bindable(false),
    cardRef = $bindable<HTMLElement | null>(null)
  }: {
    memory: Memory;
    anchor: HTMLElement | null;
    open?: boolean;
    cardRef?: HTMLElement | null;
  } = $props();

  const summary = $derived(stripCategoryPrefix(memory.summary, memory.category));
  const stateWords = $derived(memoryStateWords(memory));
  const created = $derived(memory.createdAt ? timestampDate(memory.createdAt) : undefined);
  // First ~6 lines only — the card itself also clamps visually via
  // -webkit-line-clamp, this just avoids handing it a huge string.
  const contentPreview = $derived(memory.content ? memory.content.split('\n').slice(0, 6).join('\n') : '');
</script>

<HoverCard.Root bind:open openDelay={0} closeDelay={0}>
  <HoverCard.Content
    customAnchor={anchor}
    side="bottom"
    sideOffset={6}
    collisionPadding={12}
    class="hcard"
    bind:ref={cardRef}
  >
    <div class="hc-head">
      <!-- DSYS-03/D-17: raw category hues fail 4.5:1 against the card's own
           --surface-2 background for several categories (verified by
           surfaces.browser.test.ts) -- --muted-foreground passes in both
           themes; the category name text still identifies the category. -->
      <span class="hc-cat">{memory.category}</span>
      <span class="hc-short-id">{memory.shortId}</span>
    </div>
    <div class="hc-scope">{memory.scope}</div>
    <div class="hc-summary">{summary}</div>
    {#if stateWords.length > 0}
      <div class="hc-states">
        {#each stateWords as w (w)}<span class="hc-st">{w}</span>{/each}
      </div>
    {/if}
    {#if contentPreview}
      <div class="hc-prev">{contentPreview}</div>
    {/if}
    {#if memory.tags.length > 0}
      <div class="hc-tags">
        {#each memory.tags as t (t)}<span class="hc-tag">{t}</span>{/each}
      </div>
    {/if}
    <div class="hc-footer">
      {#if created}<span>{fullTimestamp(created)} · {relativeTime(created)}</span> · {/if}<span
        >{memory.visibility || 'private'}</span
      > · <span>{memory.owner}</span>
      <span class="hc-hint">↵ open</span>
    </div>
  </HoverCard.Content>
</HoverCard.Root>

<style>
  :global(.hcard[data-slot='hover-card-content']) {
    width: min(calc(480 * var(--u)), calc(100vw - 24px));
    max-width: calc(100vw - 24px);
    background: var(--surface-2);
    border-color: var(--border);
    box-shadow: var(--shadow-lg, 0 10px 30px -5px rgb(0 0 0 / 0.3));
    padding: calc(10 * var(--u)) calc(12 * var(--u)) calc(12 * var(--u));
    display: flex;
    flex-direction: column;
    gap: calc(6 * var(--u));
  }
  .hc-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: calc(8 * var(--u));
    font-size: calc(11 * var(--u));
  }
  .hc-cat {
    font-weight: 600;
    text-transform: uppercase;
    color: var(--muted-foreground);
  }
  .hc-short-id {
    font-family: var(--font-mono, monospace);
    color: var(--muted-foreground);
  }
  .hc-scope {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .hc-summary {
    font-size: calc(13 * var(--u));
  }
  .hc-states {
    display: flex;
    flex-wrap: wrap;
    gap: calc(4 * var(--u));
  }
  .hc-st {
    font-family: var(--font-mono, monospace);
    font-size: calc(10 * var(--u));
    padding: 0 calc(5 * var(--u));
    border: 1px solid var(--border);
    border-radius: calc(3 * var(--u));
    color: var(--text-faint);
  }
  .hc-prev {
    font-size: calc(12 * var(--u));
    color: var(--muted-foreground);
    white-space: pre-line;
    display: -webkit-box;
    -webkit-line-clamp: 6;
    line-clamp: 6;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .hc-tags {
    display: flex;
    flex-wrap: wrap;
    gap: calc(4 * var(--u));
  }
  .hc-tag {
    font-family: var(--font-mono, monospace);
    font-size: calc(10.5 * var(--u));
    background: var(--muted);
    border-radius: calc(3 * var(--u));
    padding: 0 calc(4 * var(--u));
  }
  .hc-footer {
    display: flex;
    align-items: center;
    gap: calc(4 * var(--u));
    font-family: var(--font-mono, monospace);
    font-size: calc(10.5 * var(--u));
    color: var(--text-faint);
  }
  .hc-hint {
    margin-left: auto;
  }
</style>
