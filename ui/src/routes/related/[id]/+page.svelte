<script lang="ts">
  // /related/<id> (D-01, D-02): one RelatedMemories response, rendered as
  // an anchor card + mono call line, edge-type lanes as the body (plan
  // 05-04), and a sticky rail overview graph (this plan). The route is
  // honest in every state -- loading, not-found, rejected, opaque failure,
  // zero candidates and populated -- all through the same shared client and
  // error-parsing conventions every other recall surface uses. Lane cards
  // themselves are plan 05-04's.
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { createQuery } from '@tanstack/svelte-query';
  import { SvelteMap } from 'svelte/reactivity';
  import { toast } from 'svelte-sonner';
  import { engram } from '$lib/client';
  import { buildRelatedModel, visibleMembership, callLineParts, RELATED_K } from '$lib/related/graph';
  import { parseConnectError, fixRowsFor } from '$lib/errors/connect-error';
  import RelatedGraph from '$lib/components/RelatedGraph.svelte';
  import { Skeleton } from '$lib/components/ui/skeleton';
  import { Button } from '$lib/components/ui/button';

  const id = $derived(page.params.id ?? '');

  // Keyed by id so a re-centre to a different anchor doesn't carry the
  // previous anchor's measured duration into the new call line.
  const durations = new SvelteMap<string, number>();

  const relatedQ = createQuery(() => ({
    queryKey: ['relatedMemories', id, RELATED_K, false],
    queryFn: async ({ signal }) => {
      const t0 = performance.now();
      const res = await engram.relatedMemories({ id, k: BigInt(RELATED_K), full: false }, { signal });
      durations.set(id, Math.round(performance.now() - t0));
      return res;
    },
    enabled: !!id,
    meta: { silent: true }
  }));

  let selectedId = $state<string | null>(null);

  function recenter(nextId: string) {
    selectedId = null;
    goto(`${base}/related/${nextId}`);
  }

  const parsedError = $derived(relatedQ.isError ? parseConnectError(relatedQ.error) : undefined);

  async function copyError(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      toast.success('copied error text');
    } catch {
      toast.error('copy failed');
    }
  }

  // Sticky-rail offset (RESEARCH Open Question 1): AppShell's header has no
  // fixed px height, so the offset is measured locally rather than copied
  // from the sketch's hardcoded 56/66.
  let rootEl: HTMLDivElement | undefined = $state();

  onMount(() => {
    if (!rootEl) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        rootEl?.style.setProperty('--rail-max-h', `${entry.contentRect.height}px`);
      }
    });
    ro.observe(rootEl);
    return () => ro.disconnect();
  });
</script>

<div class="related-page" bind:this={rootEl}>
  {#if relatedQ.isLoading}
    <div class="anchor-card skeleton-anchor">
      <div class="call">{'RelatedMemories(subj, "…", k=64) — resolving anchor, 4 typed sub-queries in flight▍'}</div>
    </div>
    <div class="s-grid">
      <div class="lanes">
        {#each { length: 4 } as _unused, i (i)}
          <Skeleton class="lane-skel" data-testid="lane-skeleton" />
        {/each}
      </div>
      <aside class="rail">
        <Skeleton class="rail-skel" />
      </aside>
    </div>
  {:else if parsedError?.kind === 'not-found'}
    <div class="err-state" data-testid="err-not-found">
      <p class="err-heading">No memory with id {id} that you can read</p>
    </div>
  {:else if parsedError?.kind === 'ambiguous-short-id'}
    <div class="err-state err-warning" data-testid="err-ambiguous">
      <p class="err-heading">short_id {parsedError.shortId} is ambiguous — paste the full id to be exact</p>
    </div>
  {:else if parsedError?.kind === 'rejected'}
    {@const parsed = parsedError}
    <div class="err-state" data-testid="err-rejected">
      <p class="err-heading">Server rejected the request</p>
      <pre class="err-envelope">field={parsed.fields.join(',')} hint={parsed.hint}: {parsed.detail}</pre>
      <div class="err-fixes">
        {#each fixRowsFor(parsed) as fix (fix.id)}
          <Button variant="outline" size="sm" onclick={() => relatedQ.refetch()}>{fix.label}</Button>
        {/each}
      </div>
    </div>
  {:else if parsedError?.kind === 'opaque'}
    {@const parsed = parsedError}
    <div class="err-state" data-testid="err-opaque">
      <p class="err-heading">RelatedMemories failed — nothing was related. related_memories returned code={parsed.codeName}</p>
      <p class="err-subtext">Nothing was searched, so this is not an empty neighbourhood</p>
      <pre class="err-detail">{parsed.detail}</pre>
      <div class="err-fixes">
        <Button variant="outline" size="sm" onclick={() => relatedQ.refetch()}>Retry</Button>
        <Button variant="outline" size="sm" onclick={() => copyError(parsed.detail)}>Copy error</Button>
      </div>
    </div>
  {:else if relatedQ.data}
    {@const model = buildRelatedModel(relatedQ.data, RELATED_K)}
    {@const membership = visibleMembership(model, { hiddenTypes: new Set(), vectorExpanded: false })}
    {@const call = callLineParts(model, durations.get(id))}
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
    overflow-y: auto;
    container: frame / inline-size;
    --rail-max-h: 100%;
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
    max-height: var(--rail-max-h);
    overflow-y: auto;
    min-height: calc(460 * var(--u));
  }
  /* The graph is scoped inside RelatedGraph.svelte, but this route owns the
     rail's sizing contract -- reach into the child component's own .graph
     class rather than duplicating the rule there. */
  .rail :global(svg.graph) {
    width: 100%;
    aspect-ratio: 440 / 380;
  }
  @container frame (max-width: 900px) {
    .s-grid {
      grid-template-columns: 1fr;
    }
    .rail {
      position: relative;
      top: auto;
      max-height: none;
    }
  }

  /* :global() -- these classes are forwarded through Skeleton's own `class`
     prop, so Svelte's compiler never sees a literal match in this
     component's own markup and would otherwise prune the rule as unused. */
  :global(.lane-skel) {
    height: calc(88 * var(--u));
    border-radius: calc(4 * var(--u));
  }
  :global(.rail-skel) {
    height: calc(460 * var(--u));
    border-radius: calc(4 * var(--u));
  }

  .err-state {
    display: flex;
    flex-direction: column;
    gap: calc(10 * var(--u));
    padding: calc(24 * var(--u)) calc(16 * var(--u));
  }
  .err-heading {
    font-size: calc(14 * var(--u));
  }
  .err-warning .err-heading {
    color: var(--warning);
  }
  .err-subtext {
    color: var(--text-faint);
    font-size: calc(12 * var(--u));
  }
  .err-envelope,
  .err-detail {
    font-family: var(--font-mono, monospace);
    font-size: calc(12 * var(--u));
    border: 1px solid var(--destructive);
    color: var(--destructive);
    background: var(--surface-2);
    border-radius: calc(4 * var(--u));
    padding: calc(8 * var(--u));
    white-space: pre-wrap;
    user-select: text;
  }
  .err-fixes {
    display: flex;
    flex-wrap: wrap;
    gap: calc(8 * var(--u));
  }
</style>
