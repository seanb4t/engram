<script lang="ts">
  // D-05's evidence section, directly under the graph in the rail, never
  // over it: the candidate header, "Why it is related" per-type lines in
  // canonical order, Re-centre/Open record, and the closing note. Renders
  // exactly what lanes.ts computes -- it owns no evidence derivation itself.
  import type { Candidate } from '$lib/related/graph';
  import type { Memory } from '$lib/gen/engram_pb';
  import { evidenceHeader, evidenceLines } from '$lib/related/lanes';
  import { memoryStateWords } from '$lib/memorystate';
  import { relativeTime } from '$lib/time';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { Button } from '$lib/components/ui/button';

  let {
    candidate,
    anchor,
    openHref,
    onclose,
    onrecenter
  }: {
    candidate: Candidate;
    anchor: Memory;
    openHref: string;
    onclose: () => void;
    onrecenter: () => void;
  } = $props();

  const lines = $derived(evidenceLines(candidate));
  const header = $derived(evidenceHeader(lines.length));
  const states = $derived(memoryStateWords(candidate.memory));
  const when = $derived(candidate.memory.createdAt ? relativeTime(timestampDate(candidate.memory.createdAt)) : '');
  const anchorTags = $derived(new Set(anchor.tags));
</script>

<section class="ev" aria-label={`Why ${candidate.shortId} is related`}>
  <div class="ev-head">
    <span class="cat-chip">
      <i class="cat-dot" aria-hidden="true" style="background:var(--cat-{candidate.memory.category})"></i>
      <span class="cat-word">{candidate.memory.category}</span>
    </span>
    <span class="short-id">{candidate.shortId}</span>
    {#if when}<span class="when">{when}</span>{/if}
    <button type="button" class="close" aria-label="Clear selection" onclick={onclose}>esc ×</button>
  </div>
  <p class="sum">{candidate.memory.summary}</p>
  {#if candidate.memory.tags.length > 0}
    <div class="tags">
      {#each candidate.memory.tags as t (t)}
        <span class="tagc" class:shared={anchorTags.has(t)}>#{t}</span>
      {/each}
    </div>
  {/if}
  {#if states.length > 0}
    <div class="states">
      {#each states as w (w)}<span class="st">{w}</span>{/each}
    </div>
  {/if}
  <div class="why">
    <p class="why-h">{header}</p>
    {#each lines as line (line.type)}
      <div class="ev-line">
        <span class="glyph">{line.glyph}</span>
        <span class="label">{line.label}</span>
        <span class="text">{line.text}</span>
      </div>
    {/each}
  </div>
  <div class="actions">
    <Button size="sm" onclick={onrecenter}>Re-centre ↵</Button>
    <a class="open" href={openHref}>Open record ↗</a>
  </div>
  <p class="note">Evidence is per type; there is no blended score.</p>
</section>

<style>
  .ev {
    display: flex;
    flex-direction: column;
    gap: calc(6 * var(--u));
    padding: calc(8 * var(--u)) calc(10 * var(--u));
    border-top: 1px solid var(--border-subtle);
  }
  .ev-head {
    display: flex;
    align-items: center;
    gap: calc(8 * var(--u));
  }
  .cat-chip {
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
    font-size: calc(11 * var(--u));
  }
  .cat-dot {
    width: calc(8 * var(--u));
    height: calc(8 * var(--u));
    border-radius: 999px;
    flex: none;
  }
  .short-id {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .when {
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
  }
  .close {
    margin-left: auto;
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .sum {
    font-size: calc(12 * var(--u));
  }
  .tags {
    display: flex;
    flex-wrap: wrap;
    gap: calc(4 * var(--u));
  }
  .tagc {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    background: var(--surface-2);
    border-radius: calc(4 * var(--u));
    padding: 0 calc(4 * var(--u));
  }
  .tagc.shared {
    outline: 1px solid var(--primary);
  }
  .states {
    display: flex;
    gap: calc(4 * var(--u));
  }
  .st {
    font-family: var(--font-mono, monospace);
    font-size: calc(10 * var(--u));
    padding: 0 calc(5 * var(--u));
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
    color: var(--text-faint);
  }
  .why {
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
  }
  .why-h {
    font-weight: 600;
    font-size: calc(11 * var(--u));
  }
  .ev-line {
    display: grid;
    grid-template-columns: auto calc(76 * var(--u)) minmax(0, 1fr);
    gap: calc(4 * var(--u));
    align-items: baseline;
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .label {
    text-transform: capitalize;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: calc(8 * var(--u));
  }
  .open {
    font-size: calc(12 * var(--u));
    color: var(--primary);
  }
  .note {
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
  }
</style>
