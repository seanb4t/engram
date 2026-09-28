<script lang="ts">
  // /related/<id> (D-01, D-02): one RelatedMemories response, rendered as
  // an anchor card + mono call line, edge-type lanes as the body (plan
  // 05-04), and a sticky rail overview graph (this plan). Task 1 fetches
  // and draws the anchor + its candidates unfiltered; loading/error/empty
  // shell states are Task 3's job (D-01 route shell), lane cards are plan
  // 05-04's.
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { buildRelatedModel, visibleMembership, callLineParts, RELATED_K } from '$lib/related/graph';
  import RelatedGraph from '$lib/components/RelatedGraph.svelte';

  const id = $derived(page.params.id ?? '');

  const relatedQ = createQuery(() => ({
    queryKey: ['relatedMemories', id, RELATED_K, false],
    queryFn: ({ signal }) => engram.relatedMemories({ id, k: BigInt(RELATED_K), full: false }, { signal }),
    enabled: !!id,
    meta: { silent: true }
  }));

  let selectedId = $state<string | null>(null);

  function recenter(nextId: string) {
    selectedId = null;
    goto(`${base}/related/${nextId}`);
  }
</script>

<div class="related-page">
  {#if relatedQ.data && relatedQ.data.anchor}
    {@const model = buildRelatedModel(relatedQ.data, RELATED_K)}
    {@const membership = visibleMembership(model, { hiddenTypes: new Set(), vectorExpanded: false })}
    {@const call = callLineParts(model)}
    <div class="anchor-card">
      <div class="meta">
        <span class="cat-chip">
          <i class="cat-dot" aria-hidden="true" style="background: var(--cat-{model.anchor.category})"></i>
          <span class="cat-word">{model.anchor.category}</span>
        </span>
        <span class="short-id">{model.anchor.shortId}</span>
      </div>
      <div class="summary">{model.anchor.summary}</div>
      {#if model.anchor.tags.length > 0}
        <div class="tags">
          {#each model.anchor.tags as t (t)}<span class="tagc">#{t}</span>{/each}
        </div>
      {/if}
      <div class="call">
        {call.before}<span class="trunc" class:trunc-on={model.truncated}>{call.truncatedText}</span>{call.after}
      </div>
    </div>
    <div class="s-grid">
      <div class="lanes"><!-- plan 05-04 renders edge-type lanes here --></div>
      <aside class="rail">
        <RelatedGraph
          anchorId={model.anchor.id}
          nodes={membership.nodes}
          edges={membership.edges}
          {selectedId}
          onselect={(nid) => (selectedId = nid)}
          onrecenter={recenter}
        />
      </aside>
    </div>
  {/if}
</div>

<style>
  .related-page {
    display: flex;
    flex-direction: column;
    gap: calc(8 * var(--u));
    padding: calc(10 * var(--u)) calc(14 * var(--u));
    height: 100%;
    min-height: 0;
    overflow-y: auto;
  }
  .anchor-card {
    border-left: 3px solid var(--primary);
    background: var(--card);
    border-radius: calc(4 * var(--u));
    padding: calc(8 * var(--u)) calc(12 * var(--u));
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
  }
  .meta {
    display: flex;
    align-items: center;
    gap: calc(8 * var(--u));
  }
  .cat-chip {
    display: inline-flex;
    align-items: center;
    gap: calc(4 * var(--u));
  }
  .cat-dot {
    display: inline-block;
    width: calc(8 * var(--u));
    height: calc(8 * var(--u));
    border-radius: 50%;
  }
  .short-id {
    font-family: var(--font-mono, monospace);
    color: var(--muted-foreground);
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
  .call {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .trunc-on {
    color: var(--warning);
    font-weight: 600;
  }
  .s-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) calc(340 * var(--u));
    gap: calc(8 * var(--u));
    align-items: start;
  }
  .rail {
    position: sticky;
    top: 0;
    min-height: calc(460 * var(--u));
  }
</style>
