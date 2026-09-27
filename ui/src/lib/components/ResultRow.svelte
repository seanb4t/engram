<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { stripCategoryPrefix } from '$lib/summary';
  import { relativeTime } from '$lib/time';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { memoryStateWords, isPastState, type RecordStateWord } from '$lib/memorystate';
  import { flashing } from '$lib/curation/flash.svelte.ts';
  import ScopeChip from './ScopeChip.svelte';
  import CheckIcon from '@lucide/svelte/icons/check';

  // ROW-01/ROW-04/D-05: the full dense one-line row grid — category, summary
  // (inline code, ellipsized), state chips (first +N), tags (first two +N),
  // scope, age, score (always) and rel (only when the reranker ran).
  let {
    memory,
    mode = 'ranked',
    showRel = false,
    active = false,
    opened = false,
    listFocused = false,
    selectable = false,
    selected = false,
    selectionActive = false
  }: {
    memory: Memory;
    mode?: 'ranked' | 'unranked';
    showRel?: boolean;
    active?: boolean;
    opened?: boolean;
    listFocused?: boolean;
    selectable?: boolean;
    selected?: boolean;
    selectionActive?: boolean;
  } = $props();

  const summary = $derived(stripCategoryPrefix(memory.summary, memory.category));

  // Backtick spans become <code> from TEXT NODES only — never a raw-HTML
  // insertion directive (T-02-16).
  type SummaryPart = { code: boolean; text: string };
  function splitInlineCode(text: string): SummaryPart[] {
    const parts: SummaryPart[] = [];
    const re = /`([^`]+)`/g;
    let lastIndex = 0;
    let match: RegExpExecArray | null;
    while ((match = re.exec(text))) {
      if (match.index > lastIndex) parts.push({ code: false, text: text.slice(lastIndex, match.index) });
      parts.push({ code: true, text: match[1] });
      lastIndex = match.index + match[0].length;
    }
    if (lastIndex < text.length || parts.length === 0) parts.push({ code: false, text: text.slice(lastIndex) });
    return parts;
  }
  const summaryParts = $derived(splitInlineCode(summary));

  const stateWords = $derived(memoryStateWords(memory));
  const dimmed = $derived(isPastState(stateWords));

  function stateClass(word: RecordStateWord): string {
    if (word === 'expired') return 'expired';
    if (word === 'scheduled') return 'scheduled';
    return '';
  }

  // Tags: first two, then a "+N" overflow marker.
  const shownTags = $derived(memory.tags.slice(0, 2));
  const tagOverflow = $derived(Math.max(0, memory.tags.length - 2));

  const when = $derived(memory.createdAt ? relativeTime(timestampDate(memory.createdAt)) : '');

  // ROW-04/D-05: score always renders; the bar uses the UNROUNDED score,
  // clamped 0-100. rel only renders when the caller says the reranker ran
  // AND this specific hit carries a relevance value (0 is a real value,
  // never hidden — only `undefined` means "not present").
  const scoreDisplay = $derived(mode === 'unranked' ? '—' : memory.score.toFixed(2));
  const scoreBarPct = $derived(Math.min(100, Math.max(0, memory.score * 100)));
  const relDisplay = $derived(
    showRel && memory.relevance !== undefined ? `rel ${memory.relevance.toFixed(2)}` : ''
  );

  // State chips first-try to fit inside a bounded budget (independent of the
  // row's own container-query column drops); collapse to `first +N` on
  // overflow, per foundations.md's fitStates note. Re-measured on resize of
  // that bounded box and on a text-size change (both change --u, and hence
  // the box's own pixel width).
  let statesEl: HTMLElement | undefined = $state();
  let statesCollapsed = $state(false);

  function measureStatesOverflow() {
    if (!statesEl) return;
    if (stateWords.length <= 1) {
      statesCollapsed = false;
      return;
    }
    if (!statesCollapsed && statesEl.scrollWidth > statesEl.clientWidth + 1) {
      statesCollapsed = true;
    }
  }

  $effect(() => {
    // Reset per state-word-set change (e.g. a different memory mounted at
    // this DOM position) so a shorter set is never left stuck collapsed.
    void stateWords;
    statesCollapsed = false;
  });

  $effect(() => {
    if (!statesEl) return;
    const ro = new ResizeObserver(() => measureStatesOverflow());
    ro.observe(statesEl);
    measureStatesOverflow();
    function onTextSize() {
      measureStatesOverflow();
    }
    window.addEventListener('engram:textsize', onTextSize);
    return () => {
      ro.disconnect();
      window.removeEventListener('engram:textsize', onTextSize);
    };
  });

  const stateTitle = $derived(stateWords.join(' · '));
</script>

<div
  class="result-row-line"
  class:show-rel={showRel}
  class:active-row={active}
  class:opened-row={opened}
  class:list-focused={listFocused}
  class:selectable
  class:selection-active={selectionActive}
  class:flash={flashing.has(memory.id)}
  style="--c:var(--cat-{memory.category})"
  title={opened ? 'Open in the detail pane: click or press ↵ again to close' : undefined}
>
  {#if selectable}
    <span class="row-check" aria-hidden="true" class:checked={selected}>
      {#if selected}<CheckIcon size={12} aria-hidden="true" />{/if}
    </span>
  {/if}
  <span class="cat" class:dim={dimmed}>
    <i class="cat-dot" aria-hidden="true" style="background:var(--c)"></i>
    <span class="cat-word">{memory.category}</span>
  </span>
  <span class="sum" class:dim={dimmed}>
    {#each summaryParts as part, i (i)}{#if part.code}<code>{part.text}</code>{:else}{part.text}{/if}{/each}
  </span>
  <span class="states" bind:this={statesEl} class:collapsed={statesCollapsed} class:single={stateWords.length === 1} title={statesCollapsed ? stateTitle : undefined}>
    {#if statesCollapsed}
      <span class="st {stateClass(stateWords[0])}">{stateWords[0]}</span>
      <span class="st more">+{stateWords.length - 1}</span>
    {:else}
      {#each stateWords as w (w)}<span class="st {stateClass(w)}">{w}</span>{/each}
    {/if}
  </span>
  <span class="tags" class:dim={dimmed}>
    {#each shownTags as t (t)}<span class="tag">{t}</span>{/each}
    {#if tagOverflow > 0}<span class="tag more">+{tagOverflow}</span>{/if}
  </span>
  <span class="scope" class:dim={dimmed}>
    {#if memory.scope}<ScopeChip scope={memory.scope} />{/if}
  </span>
  <span class="age">{when}</span>
  <span class="score">
    <span class="num">{scoreDisplay}</span>
    {#if mode !== 'unranked'}
      <span class="bar"><b style="width:{scoreBarPct}%"></b></span>
    {/if}
  </span>
  {#if showRel}
    <span class="rel">{relDisplay}</span>
  {/if}
</div>

<style>
  /* The column template is ONE variable (--cols) that each @container list
     band below swaps (recall-surface.md). The summary keeps a floor
     (--sum-min, relative to the list width so it can always be met) and the
     optional tracks (state chips, tags, scope) are sized minmax(0, …), so
     they yield before the summary does; age, score and rel keep fixed tracks. */
  .result-row-line {
    --sum-min: min(calc(120 * var(--u)), 20cqi);
    --cols: calc(96 * var(--u)) minmax(var(--sum-min), 1fr) auto minmax(0, calc(172 * var(--u)))
      minmax(0, calc(132 * var(--u))) calc(34 * var(--u)) calc(70 * var(--u));
    display: grid;
    grid-template-columns: var(--cols);
    align-items: center;
    column-gap: calc(10 * var(--u));
    height: calc(28 * var(--u));
    padding: 0 calc(12 * var(--u)) 0 calc(14 * var(--u));
  }
  /* D-02: the leading check-column track, prepended only when selectable.
     Higher specificity than the base rule above (two classes), so it wins
     regardless of source order. */
  .result-row-line.selectable {
    --cols: calc(28 * var(--u)) calc(96 * var(--u)) minmax(var(--sum-min), 1fr) auto minmax(0, calc(172 * var(--u)))
      minmax(0, calc(132 * var(--u))) calc(34 * var(--u)) calc(70 * var(--u));
  }
  .result-row-line.show-rel {
    grid-template-columns: var(--cols) calc(56 * var(--u));
  }
  .result-row-line.active-row {
    background: var(--selected);
  }
  .result-row-line.opened-row {
    background: color-mix(in srgb, var(--primary) 10%, var(--selected));
    box-shadow: inset calc(3 * var(--u)) 0 0 var(--primary);
  }

  .cat {
    display: inline-flex;
    align-items: center;
    gap: calc(6 * var(--u));
    min-width: 0;
    font-size: calc(11 * var(--u));
    font-weight: 500;
    color: var(--c);
  }
  .cat-dot {
    width: calc(6 * var(--u));
    height: calc(6 * var(--u));
    border-radius: 999px;
    flex: none;
    display: inline-block;
  }
  .cat-word {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cat.dim {
    opacity: 0.5;
  }

  .sum {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: calc(13 * var(--u));
    min-width: 0;
  }
  .sum.dim {
    opacity: 0.5;
  }
  .sum code {
    font-family: var(--font-mono, monospace);
    font-size: calc(12 * var(--u));
    background: var(--surface-2);
    border-radius: calc(3 * var(--u));
    padding: 0 calc(3 * var(--u));
  }

  .states {
    display: inline-flex;
    align-items: center;
    gap: calc(3 * var(--u));
    min-width: 0;
    max-width: calc(96 * var(--u));
    overflow: hidden;
  }
  .st {
    flex: none;
    white-space: nowrap;
    font-family: var(--font-mono, monospace);
    font-size: calc(10 * var(--u));
    padding: 0 calc(5 * var(--u));
    border: 1px solid var(--border);
    border-radius: calc(3 * var(--u));
    color: var(--text-faint);
  }
  /* A lone chip cannot collapse to "first +N"; when the row is so tight that
     the state track yields below the chip's width, ellipsize it rather than
     hard-clip it. */
  .states.collapsed > .st:first-child,
  .states.single > .st {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .st.expired {
    color: var(--warning);
    border-color: color-mix(in srgb, var(--warning) 45%, transparent);
  }
  .st.scheduled {
    color: var(--primary);
    border-color: color-mix(in srgb, var(--primary) 45%, transparent);
  }
  .st.more {
    color: var(--text-faint);
  }

  .tags {
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
    min-width: 0;
    overflow: hidden;
  }
  .tags.dim {
    opacity: 0.5;
  }
  .tag {
    flex: none;
    font-family: var(--font-mono, monospace);
    font-size: calc(10.5 * var(--u));
    background: var(--muted);
    border-radius: calc(3 * var(--u));
    padding: 0 calc(4 * var(--u));
  }
  .tag.more {
    background: none;
    color: var(--text-faint);
  }

  .scope {
    min-width: 0;
    overflow: hidden;
  }
  .scope.dim {
    opacity: 0.5;
  }

  .age {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
    text-align: right;
    white-space: nowrap;
  }

  .score {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: calc(6 * var(--u));
  }
  .score .num {
    font-family: var(--font-mono, monospace);
    font-size: calc(12 * var(--u));
    color: var(--muted-foreground);
  }
  .score .bar {
    width: calc(28 * var(--u));
    height: calc(3 * var(--u));
    border-radius: 2px;
    background: var(--border-subtle);
    overflow: hidden;
  }
  .score .bar b {
    display: block;
    height: 100%;
    background: var(--primary);
  }

  .rel {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
    text-align: right;
    white-space: nowrap;
  }

  /* Column drop-out by LIST width (container query on the ancestor named
     "list", set on ResultsList's wrapper) — states/score/rel always stay.
     Each band drops the hidden cells' tracks from --cols, not just the cells:
     a hidden cell leaves no grid item, so a stale track would both starve the
     summary and shift age/score one track left. */
  @container list (max-width: 860px) {
    .result-row-line {
      --cols: calc(96 * var(--u)) minmax(var(--sum-min), 1fr) auto minmax(0, calc(132 * var(--u)))
        calc(34 * var(--u)) calc(70 * var(--u));
    }
    .result-row-line.selectable {
      --cols: calc(28 * var(--u)) calc(96 * var(--u)) minmax(var(--sum-min), 1fr) auto
        minmax(0, calc(132 * var(--u))) calc(34 * var(--u)) calc(70 * var(--u));
    }
    .tags {
      display: none;
    }
  }
  @container list (max-width: 560px) {
    .result-row-line {
      --cols: calc(6 * var(--u)) minmax(var(--sum-min), 1fr) auto calc(34 * var(--u)) calc(40 * var(--u));
    }
    .result-row-line.selectable {
      --cols: calc(28 * var(--u)) calc(6 * var(--u)) minmax(var(--sum-min), 1fr) auto calc(34 * var(--u))
        calc(40 * var(--u));
    }
    .tags,
    .scope {
      display: none;
    }
    .cat-word {
      display: none;
    }
    .score .bar {
      display: none;
    }
  }

  /* D-02: the check column. Decorative only (aria-hidden — the option's
     aria-selected carries the real semantics, foundations.md WCAG 4.1.2:
     no focusable control nested inside role="option"). Faint until the row
     is hovered or the list has a non-empty selection somewhere, so an
     unselected row's checkbox doesn't compete with the summary at rest. */
  .row-check {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: calc(14 * var(--u));
    height: calc(14 * var(--u));
    border: 1px solid var(--border);
    border-radius: calc(3 * var(--u));
    color: var(--primary-foreground);
    opacity: 0.35;
    transition: opacity 0.1s ease;
  }
  .result-row-line:hover .row-check,
  .result-row-line.selection-active .row-check {
    opacity: 1;
  }
  .row-check.checked {
    background: var(--primary);
    border-color: var(--primary);
    opacity: 1;
  }

  /* D-10: a row whose id is in the shared `flashing` set (flash.svelte.ts)
     briefly highlights after a successful curation write. */
  .result-row-line.flash {
    animation: row-flash 1.6s ease;
  }
  @keyframes row-flash {
    0%,
    30% {
      background: var(--primary-soft);
    }
    100% {
      background: transparent;
    }
  }
</style>
