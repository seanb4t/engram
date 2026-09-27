<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog';
  import { Button } from '$lib/components/ui/button';
  import type { Memory } from '$lib/gen/engram_pb';
  import type { ArchiveSubmitOutcome } from '$lib/mutations/curation';
  import { ArchiveOutcome } from '$lib/gen/engram_pb';

  // Host-authoritative closure, same shape as DeleteConfirmDialog (Task 1
  // tracer slice): `open` is driven entirely by the host. This component
  // never sets `open = false` itself -- it closes only via Cancel/dismiss
  // (oncancel) or when the host clears its target on Done.
  let {
    open = $bindable(false),
    mode,
    records,
    onsubmit,
    oncancel,
    ondone
  }: {
    open?: boolean;
    mode: 'archive' | 'restore';
    records: Memory[];
    onsubmit: (ids: string[]) => Promise<ArchiveSubmitOutcome>;
    oncancel: () => void;
    ondone: (changedIds: string[]) => void;
  } = $props();

  const verbLabel = $derived(mode === 'archive' ? 'Archive' : 'Restore');
  const headerText = $derived(`${verbLabel} ${records.length} records?`);
  const sublineText = $derived(
    mode === 'archive'
      ? 'Stamps archived_at. Drops out of search, list, discovery and scheduled recall; still fetchable by id. Reversible — Restore clears it.'
      : 'Clears archived_at. The record returns to normal recall.'
  );

  let pending = $state(false);
  let outcome = $state<ArchiveSubmitOutcome | undefined>(undefined);

  const resultCount = $derived.by(() => {
    if (!outcome || outcome.kind !== 'ok') return 0;
    const target = mode === 'archive' ? ArchiveOutcome.ARCHIVED : ArchiveOutcome.RESTORED;
    return outcome.results.filter((r) => r.outcome === target).length;
  });

  const resultChangedIds = $derived.by(() => {
    if (!outcome || outcome.kind !== 'ok') return [];
    const target = mode === 'archive' ? ArchiveOutcome.ARCHIVED : ArchiveOutcome.RESTORED;
    return outcome.results.filter((r) => r.outcome === target).map((r) => r.id);
  });

  async function handleVerb() {
    if (pending || records.length === 0) return;
    pending = true;
    try {
      outcome = await onsubmit(records.map((r) => r.id));
    } finally {
      pending = false;
    }
  }

  function handleDone() {
    const ids = resultChangedIds;
    outcome = undefined;
    ondone(ids);
  }

  function handleOpenChange(next: boolean) {
    if (!next) {
      if (outcome?.kind === 'ok') {
        handleDone();
      } else {
        outcome = undefined;
        oncancel();
      }
    }
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content
    style="max-width: calc(440 * var(--u));"
    showCloseButton={!pending}
    escapeKeydownBehavior={pending ? 'ignore' : 'close'}
    interactOutsideBehavior={pending ? 'ignore' : 'close'}
  >
    {#if outcome?.kind === 'ok'}
      <Dialog.Header>
        <Dialog.Title class="text-[color:var(--success)]">✓ {resultCount} {mode === 'archive' ? 'archived' : 'restored'}</Dialog.Title>
        <Dialog.Description>
          {mode === 'archive' ? 'Hidden from recall; get_memory still returns them.' : 'Back in normal recall.'}
        </Dialog.Description>
      </Dialog.Header>
      <Dialog.Footer>
        <Button variant="outline" onclick={handleDone}>Done</Button>
      </Dialog.Footer>
    {:else}
      <Dialog.Header>
        <Dialog.Title>{headerText}</Dialog.Title>
        <Dialog.Description>{sublineText}</Dialog.Description>
      </Dialog.Header>
      <div class="acd-chips">
        {#each records as record (record.id)}
          <div class="acd-chip">
            <span class="acd-cat-dot" style="background: var(--cat-{record.category})"></span>
            <span class="acd-sid">{record.shortId}</span>
            <span class="acd-sum">{record.summary || record.content}</span>
          </div>
        {/each}
      </div>
      <Dialog.Footer>
        <Dialog.Close>
          {#snippet child({ props })}
            <Button variant="outline" {...props} disabled={pending}>Cancel</Button>
          {/snippet}
        </Dialog.Close>
        <Button
          variant="outline"
          class={mode === 'archive'
            ? 'text-[color:var(--destructive)] border-[color-mix(in_srgb,var(--destructive)_45%,var(--border))]'
            : ''}
          disabled={pending || records.length === 0}
          onclick={handleVerb}
        >
          {verbLabel}
        </Button>
      </Dialog.Footer>
    {/if}
  </Dialog.Content>
</Dialog.Root>

<style>
  .acd-chips {
    display: flex;
    flex-direction: column;
    gap: calc(6 * var(--u));
    max-height: calc(240 * var(--u));
    overflow-y: auto;
  }
  .acd-chip {
    display: flex;
    align-items: center;
    gap: calc(6 * var(--u));
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
    padding: calc(4 * var(--u)) calc(6 * var(--u));
    font-size: calc(12 * var(--u));
  }
  .acd-cat-dot {
    width: calc(8 * var(--u));
    height: calc(8 * var(--u));
    border-radius: 50%;
    flex: none;
  }
  .acd-sid {
    font-family: var(--font-mono, monospace);
    flex: none;
    white-space: nowrap;
  }
  .acd-sum {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
    color: var(--muted-foreground);
  }
</style>
