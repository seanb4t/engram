<script lang="ts">
  // The Chain dialog (D-06): renders one record's supersession history as
  // columns, oldest-left -> head-right, built from two RelatedMemories reads
  // — the anchor (to discover the live head via a SUCCESSOR edge) and the
  // head (to walk its PREDECESSOR edges). Renders the pure model from
  // $lib/curation/chain — never walks the chain itself.
  //
  // Host-authoritative `open` (DeleteConfirmDialog's pattern): the host
  // drives `open` from its own chain-target state; this component closes
  // only via oncancel (bits-ui-initiated close), never by setting
  // `open = false` itself.
  import * as Dialog from '$lib/components/ui/dialog';
  import * as ScrollArea from '$lib/components/ui/scroll-area';
  import { Button } from '$lib/components/ui/button';
  import { createQuery } from '@tanstack/svelte-query';
  import { ConnectError, Code } from '@connectrpc/connect';
  import { engram } from '$lib/client';
  import { headIdFrom, buildChain, type ChainNode } from '$lib/curation/chain';
  import { EdgeType, SupersessionDirection } from '$lib/gen/engram_pb';

  let {
    open = $bindable(false),
    anchorId,
    onsupersedehead,
    oncancel
  }: {
    open?: boolean;
    anchorId: string;
    onsupersedehead?: (headId: string) => void;
    oncancel: () => void;
  } = $props();

  const anchorQuery = createQuery(() => ({
    queryKey: ['relatedMemories', anchorId, 1, false],
    queryFn: ({ signal }) => engram.relatedMemories({ id: anchorId, k: 1n, full: false }, { signal }),
    enabled: !!anchorId && open
  }));

  const headId = $derived(anchorQuery.data ? headIdFrom(anchorQuery.data) : undefined);
  const needsHeadQuery = $derived(!!headId && headId !== anchorId);

  const headQuery = createQuery(() => ({
    queryKey: ['relatedMemories', headId, 1, false],
    queryFn: ({ signal }) => engram.relatedMemories({ id: headId as string, k: 1n, full: false }, { signal }),
    enabled: needsHeadQuery
  }));

  const headResp = $derived(needsHeadQuery ? headQuery.data : anchorQuery.data);
  const chain = $derived(headResp ? buildChain(headResp, anchorId) : undefined);
  const headMem = $derived(headResp?.anchor);

  const anchorMem = $derived(anchorQuery.data?.anchor);
  const anchorShortId = $derived(anchorMem?.shortId ?? '');
  const headShortId = $derived(needsHeadQuery ? headMem?.shortId ?? '' : anchorShortId);

  // The anchor's own immediate successor (the depth-1 SUCCESSOR hop in the
  // anchor's own RelatedMemories response) — never the ultimate head, which
  // can be several hops further along the chain.
  const immediateSuccessor = $derived.by(() => {
    const resp = anchorQuery.data;
    if (!resp) return undefined;
    for (const rel of resp.related) {
      if (!rel.memory) continue;
      for (const edge of rel.edges) {
        if (
          edge.type === EdgeType.SUPERSESSION &&
          edge.evidence.case === 'supersession' &&
          edge.evidence.value.direction === SupersessionDirection.SUCCESSOR &&
          edge.evidence.value.depth === 1
        ) {
          return rel.memory;
        }
      }
    }
    return undefined;
  });

  const subline = $derived.by(() => {
    if (!chain || !anchorMem) return '';
    const predCount = anchorMem.supersedes.length;
    const successor = immediateSuccessor?.shortId ?? '—';
    let text = `depth d${chain.anchorDepth} below head ${headShortId} · ${predCount} direct predecessor(s) · successor ${successor}`;
    if (chain.beyondCap) text += ' · chain continues beyond 8 hops';
    return text;
  });

  // The head can be superseded again iff it is not a rule (rules cannot be
  // superseded) and carries no discovery `kind`.
  const headSupersedable = $derived(!!headMem && headMem.category !== 'rule' && headMem.kind === '');

  let peekId = $state<string | undefined>(undefined);

  const peekQuery = createQuery(() => ({
    queryKey: ['getMemory', peekId],
    queryFn: ({ signal }) => engram.getMemory({ id: peekId as string }, { signal }),
    enabled: !!peekId,
    retry: false
  }));

  function togglePeek(id: string) {
    peekId = peekId === id ? undefined : id;
  }

  function columnHeader(col: ChainNode[]) {
    const depth = col[0]?.depth ?? 0;
    return depth === 0 ? 'head · d0' : `d${depth}`;
  }

  const peekNotFound = $derived(peekQuery.error instanceof ConnectError && peekQuery.error.code === Code.NotFound);

  // Fires only when bits-ui itself closes the dialog (Escape, overlay
  // click, close button) — never on a host-driven `open = false` assignment.
  function handleOpenChange(next: boolean) {
    if (!next) oncancel();
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content class="sm:max-w-[calc(640*var(--u))]">
    <Dialog.Header>
      <Dialog.Title>Chain · {anchorShortId}</Dialog.Title>
      <Dialog.Description class="text-[calc(11*var(--u))]">
        ← predecessors (superseded, hidden from recall) · arrows read 'superseded by' · head is the live record
      </Dialog.Description>
    </Dialog.Header>

    {#if chain}
      <div class="chain-sub text-[calc(11*var(--u))] text-muted-foreground">{subline}</div>

      <ScrollArea.Root orientation="horizontal">
        <div class="chain-cols" role="list" aria-label="Supersession chain">
          {#each chain.columns as col (col[0]?.depth ?? 0)}
            <div class="ccol" role="listitem">
              <span class="ccol-h">{columnHeader(col)}</span>
              {#each col as node (node.id)}
                {#if node.placeholder}
                  <div class="cnode placeholder">
                    <span class="csum">No memory with id {node.id} that you can read</span>
                  </div>
                {:else}
                  <button type="button" class="cnode" class:hl={node.id === anchorId} onclick={() => togglePeek(node.id)}>
                    <span class="csum">{node.summary}</span>
                    <span class="mono">{node.shortId}</span>
                  </button>
                  {#if peekId === node.id}
                    <div class="chain-peek">
                      {#if peekQuery.isLoading}
                        <span>loading…</span>
                      {:else if peekNotFound}
                        <span>No memory with id {node.id} that you can read</span>
                      {:else if peekQuery.isError}
                        <span>Could not load this record</span>
                      {:else if peekQuery.data?.memory}
                        {@const peeked = peekQuery.data.memory}
                        <span class="mono">
                          get_memory {peeked.shortId} · fetch-by-id ignores the recall gate · superseded_by {peeked.supersededBy ||
                            '—'}
                        </span>
                        <p class="chain-peek-summary">{peeked.summary}</p>
                      {/if}
                    </div>
                  {/if}
                {/if}
              {/each}
            </div>
          {/each}
        </div>
      </ScrollArea.Root>
    {/if}

    {#if headSupersedable}
      <Dialog.Footer>
        <Button variant="outline" onclick={() => onsupersedehead?.(headId as string)}>Supersede head…</Button>
      </Dialog.Footer>
    {/if}
  </Dialog.Content>
</Dialog.Root>
