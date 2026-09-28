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
  import { SvelteMap, SvelteSet } from 'svelte/reactivity';
  import { toast } from 'svelte-sonner';
  import { engram } from '$lib/client';
  import {
    buildRelatedModel,
    visibleMembership,
    callLineParts,
    neighbourhoodSummary,
    RELATED_K,
    type LaneType
  } from '$lib/related/graph';
  import { parseConnectError, fixRowsFor } from '$lib/errors/connect-error';
  import { parseRelatedParams, relatedPath } from '$lib/search/related-params';
  import { defaultSearchParams, encodeSearchParams } from '$lib/search/params';
  import RelatedGraph from '$lib/components/RelatedGraph.svelte';
  import EdgeLane from '$lib/components/EdgeLane.svelte';
  import SupersessionLane from '$lib/components/SupersessionLane.svelte';
  import EvidenceSection from '$lib/components/EvidenceSection.svelte';
  import GraphLegend from '$lib/components/GraphLegend.svelte';
  import TagBars from '$lib/components/TagBars.svelte';
  import { noNeighboursLines, TRUNCATION_BANNER } from '$lib/related/lanes';
  import { Skeleton } from '$lib/components/ui/skeleton';
  import { Button } from '$lib/components/ui/button';
  import { Kbd } from '$lib/components/ui/kbd';
  import * as Tabs from '$lib/components/ui/tabs';

  const id = $derived(page.params.id ?? '');
  const params = $derived(parseRelatedParams(page.url.searchParams));

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

  // The model and its membership projection are lifted to the top level
  // (not computed inline in the template) so recenter() and the
  // evidence-section lookup below can read them too.
  const model = $derived(relatedQ.data ? buildRelatedModel(relatedQ.data, RELATED_K) : undefined);

  let selection = $state<{ id: string; lane: LaneType | 'graph' } | null>(null);
  let vectorExpanded = $state(false);
  const hiddenTypes = new SvelteSet<LaneType>();

  // D-01: which rail tab is showing -- Task 2 wires the `g` key and
  // localStorage persistence; Task 1 only drives it from state and from
  // D-05 continuity (a lane/chain-card selection switches back to Graph).
  let railTab = $state<'graph' | 'tags'>('graph');

  // D-14: the view's own in-place tag filter -- never GRAPH-05, never
  // touches membership, the lanes' order or the RelatedMemories request.
  let filterTag = $state<string | null>(null);

  const membership = $derived(model ? visibleMembership(model, { hiddenTypes, vectorExpanded }) : undefined);

  const selectedCandidate = $derived.by(() => {
    if (!selection || !model) return undefined;
    return model.candidates.find((c) => c.id === selection.id);
  });

  // D-14 counts: N candidates (of the model's full candidate set, not just
  // the drawn/visible membership) carry filterTag; M is that same total.
  const filterCounts = $derived.by(() => {
    if (!filterTag || !model) return null;
    const total = model.candidates.length;
    const carriers = model.candidates.filter((c) => c.memory.tags.includes(filterTag)).length;
    return { carriers, total };
  });

  // D-14 dimming: every candidate whose OWN tags lack filterTag -- applied
  // identically to lane rows, chain cards and graph nodes via one shared
  // set, never the anchor (which is never a member of model.candidates).
  const filterDimmed = $derived.by(() => {
    if (!filterTag || !model) return new Set<string>();
    return new Set(model.candidates.filter((c) => !c.memory.tags.includes(filterTag)).map((c) => c.id));
  });

  // D-15: rarity is read from the candidates' own tag-edge evidence
  // (WeightedTag.weight = ln(n/df)) -- never merged with ListTags' raw
  // popularity count, and absent for a tag no tag edge actually carries.
  const tagRarity = $derived.by(() => {
    const map = new Map<string, number>();
    if (!model) return map;
    for (const c of model.candidates) {
      const edge = c.edges.tag;
      if (edge && edge.evidence.case === 'tag') {
        for (const t of edge.evidence.value.sharedTags) map.set(t.tag, t.weight);
      }
    }
    return map;
  });

  // D-11: the graph's polite live summary is the neighbourhood summary plus,
  // while a tag filter is active, the same "N of M carry it" clause the chip
  // shows -- so re-centres, toggles and filter results are all announced.
  const graphSummary = $derived.by(() => {
    if (!membership) return undefined;
    const base = neighbourhoodSummary(membership.nodes);
    if (!filterTag || !filterCounts) return base;
    return `${base} · #${filterTag} ${filterCounts.carriers} of ${filterCounts.total} carry it`;
  });

  // D-04: the anchor never carries its own change, so this effect fires
  // only on a genuine re-centre/direct navigation to a new anchor.
  $effect(() => {
    void id;
    selection = null;
    vectorExpanded = false;
    filterTag = null;
  });

  function toggleHidden(type: LaneType) {
    if (hiddenTypes.has(type)) hiddenTypes.delete(type);
    else hiddenTypes.add(type);
  }

  // Shared by every lane (EdgeLane and SupersessionLane alike): clicking the
  // anchor's own supersession card clears the selection, matching the
  // graph's anchor-node click (D-05: the anchor is never related to itself
  // and never carries an evidence section). Selecting a real candidate also
  // switches the rail to Graph (D-05 continuity) so the evidence section --
  // which lives under the graph -- is visible even if the Tags tab was
  // showing.
  function selectFromLane(candidateId: string, lane: LaneType) {
    if (model && candidateId === model.anchor.id) {
      selection = null;
    } else {
      selection = { id: candidateId, lane };
      railTab = 'graph';
    }
  }

  // recenter walks the trail to a new anchor (D-04): pushes a history entry
  // via goto, carrying the origin (`from`) and the growing trail forward.
  function recenter(nodeId: string) {
    if (!model) return;
    const candidate = model.candidates.find((c) => c.id === nodeId);
    if (!candidate) return;
    goto(`${base}${relatedPath(candidate.shortId, { from: params.from, trail: [...params.trail, model.anchor.shortId] })}`);
  }

  function openHrefFor(candidateId: string): string {
    return `${base}/search?${encodeSearchParams({ ...defaultSearchParams(), sel: candidateId })}`;
  }

  // The three one-line-row lanes rendered by EdgeLane, in canonical order --
  // supersession renders separately via SupersessionLane (plan 05-04 Task 2).
  const EDGE_LANE_TYPES: LaneType[] = ['citation', 'tag', 'vector'];

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
  {:else if model && membership}
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
    {#if model.truncated}
      <p class="truncation-banner">{TRUNCATION_BANNER}</p>
    {/if}
    <div class="s-grid">
      <div class="lanes">
        {#if filterTag && filterCounts}
          <div class="tag-filter-chip" data-testid="tag-filter-chip">
            <span class="mono">#{filterTag} {filterCounts.carriers} of {filterCounts.total} carry it</span>
            <button type="button" class="chip-clear" aria-label="Clear tag filter" onclick={() => (filterTag = null)}>×</button>
          </div>
        {/if}
        {#if model.candidates.length === 0}
          <div class="no-neighbours" data-testid="no-neighbours">
            <p class="nn-heading">Nothing related to {model.anchor.shortId}</p>
            {#each noNeighboursLines(model) as line (line)}
              <p class="nn-line">{line}</p>
            {/each}
            <a class="nn-link" href={openHrefFor(model.anchor.id)}>Open {model.anchor.shortId} in search ↗</a>
          </div>
        {:else}
          <SupersessionLane
            {model}
            hidden={hiddenTypes.has('supersession')}
            selectedId={selection?.id ?? null}
            selectedLane={selection?.lane ?? null}
            dimmedIds={filterDimmed}
            onselect={selectFromLane}
            ontogglehidden={() => toggleHidden('supersession')}
          />
          {#each EDGE_LANE_TYPES as laneType (laneType)}
            <EdgeLane
              type={laneType}
              rows={membership.lanes[laneType]}
              {model}
              hidden={hiddenTypes.has(laneType)}
              selectedId={selection?.id ?? null}
              selectedLane={selection?.lane ?? null}
              collapsed={laneType === 'vector' && membership.vectorCollapsed
                ? {
                    shown: membership.lanes.vector.length,
                    total: membership.vectorTotal,
                    k: model.k,
                    truncated: model.truncated,
                    onexpand: () => (vectorExpanded = true)
                  }
                : null}
              dimmedIds={filterDimmed}
              onselect={selectFromLane}
              ontogglehidden={() => toggleHidden(laneType)}
            />
          {/each}
        {/if}
      </div>
      <aside class="rail">
        <div class="rail-tabs-row">
          <Tabs.Root value={railTab} onValueChange={(v) => (railTab = v === 'tags' ? 'tags' : 'graph')}>
            <Tabs.List>
              <Tabs.Trigger value="graph">Graph</Tabs.Trigger>
              <Tabs.Trigger value="tags">Tags</Tabs.Trigger>
            </Tabs.List>
          </Tabs.Root>
          <Kbd>g</Kbd>
        </div>
        {#if railTab === 'graph'}
          <RelatedGraph
            anchorId={model.anchor.id}
            nodes={membership.nodes}
            edges={membership.edges}
            selectedId={selection?.id ?? null}
            dimmedIds={filterDimmed}
            summary={graphSummary}
            onselect={(nid) => (selection = nid ? { id: nid, lane: 'graph' } : null)}
            onrecenter={recenter}
          />
          {#if selectedCandidate}
            <EvidenceSection
              candidate={selectedCandidate}
              anchor={model.anchor}
              openHref={openHrefFor(selectedCandidate.id)}
              onclose={() => (selection = null)}
              onrecenter={() => recenter(selectedCandidate.id)}
            />
          {/if}
          <GraphLegend {model} {hiddenTypes} ontoggle={toggleHidden} />
        {:else}
          <TagBars
            scope=""
            mode="rail"
            markedTags={new Set(model.anchor.tags)}
            selectedTags={filterTag ? new Set([filterTag]) : new Set()}
            rarity={tagRarity}
            ontoggle={(t) => (filterTag = filterTag === t ? null : t)}
          />
        {/if}
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
  .truncation-banner {
    font-size: calc(12 * var(--u));
    font-weight: 600;
    color: var(--warning);
  }
  .no-neighbours {
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
    padding: calc(8 * var(--u)) calc(12 * var(--u));
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
  }
  .nn-heading {
    font-weight: 600;
    font-size: calc(13 * var(--u));
  }
  .nn-line {
    font-size: calc(12 * var(--u));
    color: var(--muted-foreground);
  }
  .nn-link {
    align-self: flex-start;
    font-size: calc(12 * var(--u));
    color: var(--primary);
  }
  /* D-14: the in-view tag filter chip, rendered above the lanes so it stays
     visible regardless of which rail tab is showing. */
  .tag-filter-chip {
    display: flex;
    align-items: center;
    gap: calc(6 * var(--u));
    align-self: flex-start;
    font-size: calc(11 * var(--u));
    background: var(--surface-2);
    border: 1px solid var(--primary);
    border-radius: calc(4 * var(--u));
    padding: calc(2 * var(--u)) calc(4 * var(--u)) calc(2 * var(--u)) calc(8 * var(--u));
  }
  .chip-clear {
    font-family: var(--font-mono, monospace);
    color: var(--muted-foreground);
    border-radius: calc(4 * var(--u));
    padding: 0 calc(4 * var(--u));
  }
  .s-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) calc(340 * var(--u));
    gap: calc(8 * var(--u));
    align-items: start;
  }
  .lanes {
    display: flex;
    flex-direction: column;
    gap: calc(8 * var(--u));
    min-width: 0;
  }
  .rail {
    position: sticky;
    top: 0;
    max-height: var(--rail-max-h);
    overflow-y: auto;
    min-height: calc(460 * var(--u));
    display: flex;
    flex-direction: column;
  }
  /* The graph is scoped inside RelatedGraph.svelte, but this route owns the
     rail's sizing contract -- reach into the child component's own .graph
     class rather than duplicating the rule there. */
  .rail :global(svg.graph) {
    width: 100%;
    aspect-ratio: 440 / 380;
  }
  .rail-tabs-row {
    display: flex;
    align-items: center;
    gap: calc(6 * var(--u));
    padding-bottom: calc(4 * var(--u));
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
