<script lang="ts">
  import * as Dialog from '$lib/components/ui/dialog';
  import { Button } from '$lib/components/ui/button';
  import type { Memory } from '$lib/gen/engram_pb';
  import type { ArchiveSubmitOutcome } from '$lib/mutations/curation';
  import { ArchiveOutcome } from '$lib/gen/engram_pb';

  // Host-authoritative closure, same shape as DeleteConfirmDialog: `open` is
  // driven entirely by the host. `mode`, `pending` and `outcome` are ALSO
  // bindable -- the normal confirm->submit path writes them from inside this
  // component (unchanged from the Task 1 tracer), but the D-09 result-body
  // "Undo — restore N" surface needs the HOST to drive an inverse submission
  // "through the same dialog" (mode flips, a new call runs, the same result
  // view re-renders) without a second confirm click. Making these three
  // bindable lets CurationSurfaces.onundo do exactly that from outside,
  // while every other path keeps working through this component's own
  // internal handleVerb, unchanged.
  let {
    open = $bindable(false),
    mode = $bindable<'archive' | 'restore'>('archive'),
    pending = $bindable(false),
    outcome = $bindable<ArchiveSubmitOutcome | undefined>(undefined),
    records,
    notice,
    onsubmit,
    oncancel,
    ondone,
    onundo,
    onreauth
  }: {
    open?: boolean;
    mode?: 'archive' | 'restore';
    pending?: boolean;
    outcome?: ArchiveSubmitOutcome | undefined;
    records: Memory[];
    notice?: string;
    onsubmit: (ids: string[]) => Promise<ArchiveSubmitOutcome>;
    oncancel: () => void;
    ondone: (changedIds: string[]) => void;
    onundo: (ids: string[]) => void;
    onreauth: (ids: string[]) => void;
  } = $props();

  // Held as a single constant so the literal text appears exactly once in
  // this file -- both the per-id NOT_FOUND result line and the top-level
  // rejected/"not-found" status block reference it rather than repeating it.
  const NOT_FOUND_NOTE = 'Not found, not owned and ambiguous short id are one rejection by design. Nothing was changed.';

  const verbLabel = $derived(mode === 'archive' ? 'Archive' : 'Restore');

  // currentIds is the removable chip set (E2 empty): × drops an id without
  // touching `records` itself. Resets whenever the host hands us a new
  // `records` array (a fresh openArchive/openRestore call) -- NOT on every
  // re-render, so a chip removed mid-session stays removed.
  let currentIds = $state<string[]>([]);
  let lastRecords: Memory[] | undefined;
  $effect(() => {
    if (records !== lastRecords) {
      currentIds = records.map((r) => r.id);
      lastRecords = records;
    }
  });
  const currentRecords = $derived(records.filter((r) => currentIds.includes(r.id)));

  const headerText = $derived(`${verbLabel} ${currentRecords.length} records?`);
  const sublineText = $derived(
    mode === 'archive'
      ? 'Stamps archived_at. Drops out of search, list, discovery and scheduled recall; still fetchable by id. Reversible — Restore clears it.'
      : 'Clears archived_at. The record returns to normal recall.'
  );

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

  // Best-effort short_id lookup for a result row -- ALREADY_ARCHIVED/
  // NOT_ARCHIVED rows are records the caller already owns/read, so this is
  // never privacy-sensitive (unlike NOT_FOUND, see notOwnedRecordFor below).
  function shortIdFor(id: string): string {
    return records.find((r) => r.id === id)?.shortId ?? id;
  }

  // Task 2 (option-a, server-answer only): a NOT_FOUND result whose
  // `requested` input matches a record THIS DIALOG WAS OPENED WITH that is
  // `visibility === 'shared'` gets the not-owned copy -- the caller could
  // read the record (it was resolved into `records`) but archive/restore is
  // owner-only. Every other NOT_FOUND (truly nonexistent, not-owned-and-
  // private, or ambiguous) renders the generic one-rejection copy. T-04-02:
  // never render the response `id`, only `requested`.
  function notOwnedRecordFor(requested: string): Memory | undefined {
    return records.find((r) => (r.id === requested || r.shortId === requested) && r.visibility === 'shared');
  }

  function removeChip(id: string) {
    currentIds = currentIds.filter((x) => x !== id);
  }

  async function handleVerb() {
    if (pending || currentIds.length === 0) return;
    pending = true;
    try {
      outcome = await onsubmit(currentIds);
    } finally {
      pending = false;
    }
  }

  function handleUndo() {
    onundo(resultChangedIds);
  }

  function handleDone() {
    const ids = resultChangedIds;
    outcome = undefined;
    ondone(ids);
  }

  function handleReauth() {
    onreauth(currentIds.length > 0 ? currentIds : records.map((r) => r.id));
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
      <div class="acd-result-lines">
        {#each outcome.results as r (r.requested)}
          {#if r.outcome === ArchiveOutcome.ALREADY_ARCHIVED}
            <div class="acd-info">{shortIdFor(r.id)} already archived — idempotent, no change.</div>
          {:else if r.outcome === ArchiveOutcome.NOT_ARCHIVED}
            <div class="acd-info">{shortIdFor(r.id)} was not archived — no change.</div>
          {:else if r.outcome === ArchiveOutcome.NOT_FOUND}
            {@const notOwned = notOwnedRecordFor(r.requested)}
            {#if notOwned}
              <div class="acd-info">shared by {notOwned.owner} — archive is owner-only</div>
            {:else}
              <div class="acd-rejected">not found: {r.requested}</div>
              <div class="acd-rejected-note">{NOT_FOUND_NOTE}</div>
            {/if}
          {/if}
        {/each}
      </div>
      <Dialog.Footer>
        {#if mode === 'archive' && resultCount > 0}
          <Button variant="ghost" onclick={handleUndo}>Undo — restore {resultCount}</Button>
        {/if}
        <Button variant="outline" onclick={handleDone}>Done</Button>
      </Dialog.Footer>
    {:else if outcome?.kind === 'reauth'}
      <Dialog.Header>
        <Dialog.Title>{headerText}</Dialog.Title>
        <Dialog.Description>{sublineText}</Dialog.Description>
      </Dialog.Header>
      <div role="alert" class="acd-alert">
        <span>Session expired. Nothing was written; your draft is kept.</span>
        <Button variant="outline" size="sm" class="self-start" onclick={handleReauth}>Re-authenticate</Button>
      </div>
      <Dialog.Footer>
        <Dialog.Close>
          {#snippet child({ props })}
            <Button variant="outline" {...props}>Cancel</Button>
          {/snippet}
        </Dialog.Close>
      </Dialog.Footer>
    {:else}
      <Dialog.Header>
        <Dialog.Title>{headerText}</Dialog.Title>
        <Dialog.Description>{sublineText}</Dialog.Description>
      </Dialog.Header>
      {#if notice}
        <div class="acd-notice">{notice}</div>
      {/if}
      {#if outcome?.kind === 'rejected'}
        <div role="alert" class="acd-alert">
          {#if outcome.parsed.kind === 'rejected'}
            <span>Rejected — fix the named field and resend</span>
            <div class="acd-pills">
              <span class="acd-pill">field={outcome.parsed.fields.join(',')}</span>
              <span class="acd-pill">hint={outcome.parsed.hint}</span>
            </div>
            <div class="acd-detail">{outcome.parsed.detail}</div>
          {:else if outcome.parsed.kind === 'not-found'}
            <span>not found</span>
            <div class="acd-rejected-note">{NOT_FOUND_NOTE}</div>
          {:else}
            <span>Could not archive — {outcome.parsed.codeName}</span>
            <div class="acd-detail">{outcome.parsed.detail}</div>
          {/if}
        </div>
      {/if}
      {#if pending}
        <div role="status" class="acd-loading">{mode === 'archive' ? 'archive_memory' : 'restore_memory'} · {currentRecords.length} records…</div>
      {/if}
      <div class="acd-chips">
        {#each currentRecords as record (record.id)}
          <div class="acd-chip">
            <span class="acd-cat-dot" style="background: var(--cat-{record.category})"></span>
            <span class="acd-sid">{record.shortId}</span>
            <span class="acd-sum">{record.summary || record.content}</span>
            <button
              type="button"
              class="acd-remove"
              aria-label="Remove {record.shortId}"
              disabled={pending}
              onclick={() => removeChip(record.id)}
            >
              ×
            </button>
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
          disabled={pending || currentIds.length === 0}
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
  .acd-remove {
    flex: none;
    border: none;
    background: none;
    cursor: pointer;
    color: var(--text-faint);
    font-size: calc(14 * var(--u));
    line-height: 1;
    padding: 0 calc(2 * var(--u));
  }
  .acd-remove:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }
  .acd-result-lines {
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
    font-size: calc(12 * var(--u));
  }
  .acd-info {
    color: var(--muted-foreground);
  }
  .acd-rejected {
    color: var(--destructive);
  }
  .acd-rejected-note {
    color: var(--text-faint);
  }
  .acd-loading {
    display: flex;
    align-items: center;
    gap: calc(6 * var(--u));
    color: var(--muted-foreground);
    font-size: calc(12 * var(--u));
  }
  .acd-alert {
    display: flex;
    flex-direction: column;
    gap: calc(6 * var(--u));
    font-size: calc(12 * var(--u));
    color: var(--cat-gotcha);
  }
  .acd-pills {
    display: flex;
    gap: calc(6 * var(--u));
  }
  .acd-pill {
    font-family: var(--font-mono, monospace);
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
    padding: 0 calc(5 * var(--u));
  }
  .acd-detail {
    font-family: var(--font-mono, monospace);
    color: var(--text-faint);
  }
  .acd-notice {
    font-size: calc(12 * var(--u));
    color: var(--warning);
  }
</style>
