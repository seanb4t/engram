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
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import { headIdFrom, buildChain } from '$lib/curation/chain';

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

  const anchorShortId = $derived(anchorQuery.data?.anchor?.shortId ?? '');

  function columnHeader(col: ReturnType<typeof buildChain>['columns'][number]) {
    const depth = col[0]?.depth ?? 0;
    return depth === 0 ? 'head · d0' : `d${depth}`;
  }

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
      <div class="chain-cols" role="list" aria-label="Supersession chain">
        {#each chain.columns as col (col[0]?.depth ?? 0)}
          <div class="ccol" role="listitem">
            <span class="ccol-h">{columnHeader(col)}</span>
            {#each col as node (node.id)}
              <button type="button" class="cnode" class:hl={node.id === anchorId}>
                <span class="csum">{node.summary}</span>
                <span class="mono">{node.shortId}</span>
              </button>
            {/each}
          </div>
        {/each}
      </div>
    {/if}
  </Dialog.Content>
</Dialog.Root>
