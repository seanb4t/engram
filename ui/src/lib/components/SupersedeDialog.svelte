<script lang="ts">
  // The Supersede dialog (CUR-01, D-07): preview-before-commit correction of
  // one or more predecessor records. This component never calls the write
  // client directly -- every RPC (preview, commit, add-by-short_id lookup,
  // head resolution) is a callback prop the host performs, which keeps the
  // dialog testable in isolation (Executor notes).
  //
  // Host-authoritative `open` (DeleteConfirmDialog's pattern, extended with
  // ArchiveConfirmDialog's bindable-state shape): the host drives `open` from
  // its own supersede-target state; this component closes only via oncancel
  // (a bits-ui-initiated dismiss with no result yet) or ondone (a
  // bits-ui-initiated dismiss AFTER a successful commit).
  import * as Dialog from '$lib/components/ui/dialog';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Textarea } from '$lib/components/ui/textarea';
  import { Select } from '$lib/components/ui/select';
  import type { Memory } from '$lib/gen/engram_pb';
  import { CATEGORIES } from '$lib/queries';
  import type { SupersedeDraft, SupersedeFields } from '$lib/mutations/curation';
  import type { SupersedeMemoryResponse } from '$lib/gen/engram_pb';

  // The debounce window between the last draft edit and the auto-fired
  // validate_only preview (Task 1 action text).
  const PREVIEW_DEBOUNCE_MS = 250;

  let {
    open = $bindable(false),
    targets,
    fields,
    idempotencyKey,
    prefillShortId,
    notice,
    resend = false,
    onpreview,
    onsubmit,
    oncancel,
    ondone,
    onviewsuperseded,
    onopenrecord
  }: {
    open?: boolean;
    targets: Memory[];
    fields: SupersedeFields;
    idempotencyKey: string;
    prefillShortId?: string;
    notice?: string;
    resend?: boolean;
    onpreview: (draft: SupersedeDraft, signal: AbortSignal) => Promise<SupersedeMemoryResponse>;
    onsubmit: (draft: SupersedeDraft) => Promise<SupersedeMemoryResponse>;
    oncancel: () => void;
    ondone: () => void;
    onviewsuperseded?: (ids: string[]) => void;
    onopenrecord?: (id: string) => void;
  } = $props();

  // currentTargets is the working chip set -- resets to `targets` whenever the
  // host hands us a fresh array (a new openSupersede call), same "reset on
  // reference change, not on every re-render" discipline as
  // ArchiveConfirmDialog's currentIds.
  let currentTargets = $state<Memory[]>([]);
  let lastTargets: Memory[] | undefined;
  $effect(() => {
    if (targets !== lastTargets) {
      currentTargets = [...targets];
      lastTargets = targets;
    }
  });

  // draftFields is the editable correcting-record form -- resets to `fields`
  // only when the host hands us a fresh object (a new open/reopen), never on
  // every keystroke re-render.
  let draftFields = $state<SupersedeFields>({ summary: '', content: '', category: '', scope: '', tags: [] });
  let lastFields: SupersedeFields | undefined;
  $effect(() => {
    if (fields !== lastFields) {
      draftFields = { ...fields };
      lastFields = fields;
    }
  });

  let tagInput = $state('');
  function addTagFromInput() {
    const raw = tagInput.trim().replace(/,$/, '');
    if (raw && !draftFields.tags.includes(raw)) draftFields = { ...draftFields, tags: [...draftFields.tags, raw] };
    tagInput = '';
  }
  function handleTagKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      addTagFromInput();
    } else if (e.key === 'Backspace' && tagInput === '' && draftFields.tags.length) {
      draftFields = { ...draftFields, tags: draftFields.tags.slice(0, -1) };
    }
  }
  function removeTag(t: string) {
    draftFields = { ...draftFields, tags: draftFields.tags.filter((x) => x !== t) };
  }

  const currentIds = $derived(currentTargets.map((t) => t.id));
  const hasContent = $derived(draftFields.content.trim().length > 0);

  // Validate_only preview: debounced, revision-guarded so a stale answer for
  // an earlier draft never marks a LATER draft as validated.
  let previewRevision = 0;
  let previewPending = $state(false);
  let previewValidated = $state(false);
  let previewValidatedRevision = -1;
  let previewRejection = $state<unknown>(undefined);

  $effect(() => {
    // Read every input this preview depends on so Svelte tracks them.
    const ids = currentIds;
    const f = { ...draftFields };
    const rev = ++previewRevision;
    previewValidated = false;
    previewRejection = undefined;
    if (ids.length === 0 || !f.content.trim()) {
      previewPending = false;
      return;
    }
    const controller = new AbortController();
    const timer = setTimeout(async () => {
      previewPending = true;
      try {
        const draft: SupersedeDraft = { targets: ids, fields: f, idempotencyKey };
        const resp = await onpreview(draft, controller.signal);
        if (rev !== previewRevision) return; // superseded by a later draft
        previewPending = false;
        previewValidated = resp.validated;
        previewValidatedRevision = rev;
      } catch (err) {
        if (rev !== previewRevision) return;
        previewPending = false;
        previewRejection = err;
      }
    }, PREVIEW_DEBOUNCE_MS);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  });

  const canSubmit = $derived(
    !previewPending && previewValidated && previewValidatedRevision === previewRevision && !submitPending
  );

  let submitPending = $state(false);
  let submitError = $state<unknown>(undefined);
  let result = $state<{ id: string; shortId: string } | undefined>(undefined);

  async function handleSubmit() {
    if (!canSubmit) return;
    submitPending = true;
    submitError = undefined;
    try {
      const draft: SupersedeDraft = { targets: currentIds, fields: { ...draftFields }, idempotencyKey };
      const resp = await onsubmit(draft);
      result = { id: resp.id, shortId: resp.shortId };
    } catch (err) {
      submitError = err;
    } finally {
      submitPending = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault();
      handleSubmit();
    }
  }

  function handleDone() {
    result = undefined;
    ondone();
  }

  // Fires only when bits-ui itself closes the dialog (Cancel, Escape, overlay
  // click, or the Done button) -- never on a host-driven `open = false`
  // assignment.
  function handleOpenChange(next: boolean) {
    if (!next) {
      if (result) {
        handleDone();
      } else {
        oncancel();
      }
    }
  }

  function submitErrorText(err: unknown): string {
    return err instanceof Error ? err.message : String(err);
  }
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
  <Dialog.Content
    style="max-width: calc(920 * var(--u));"
    showCloseButton={!submitPending}
    escapeKeydownBehavior={submitPending ? 'ignore' : 'close'}
    interactOutsideBehavior={submitPending ? 'ignore' : 'close'}
    onkeydown={handleKeydown}
  >
    {#if result}
      <Dialog.Header>
        <Dialog.Title class="text-[color:var(--success)]">✓ Superseded {currentTargets.length} records → {result.shortId}</Dialog.Title>
      </Dialog.Header>
      <p class="sd-success-body">
        Stored {result.shortId} and stamped superseded_by on {currentTargets.map((t) => t.shortId).join(', ')}. They drop
        out of recall but stay fetchable by id.
      </p>
      <p class="sd-no-undo">
        No undo. Supersession is additive history, not an edit. To change it, supersede {result.shortId} again.
      </p>
      <Dialog.Footer>
        <Button variant="ghost" onclick={() => onviewsuperseded?.(currentTargets.map((t) => t.id))}>
          View superseded ({currentTargets.length})
        </Button>
        <Button variant="outline" onclick={() => onopenrecord?.(result!.id)}>Open {result.shortId}</Button>
        <Dialog.Close>
          {#snippet child({ props })}
            <Button {...props} onclick={handleDone}>Done</Button>
          {/snippet}
        </Dialog.Close>
      </Dialog.Footer>
    {:else}
      <Dialog.Header>
        <Dialog.Title>Supersede {currentTargets.length} records into one</Dialog.Title>
        <Dialog.Description>
          Stores a new correcting record and stamps superseded_by on every predecessor. Additive — nothing is deleted.
        </Dialog.Description>
      </Dialog.Header>

      {#if notice}
        <div class="sd-notice">{notice}</div>
      {/if}

      {#if previewPending}
        <div role="status" class="sd-loading">supersede_memory · {currentTargets.length} targets…</div>
      {/if}

      {#if submitError}
        <div role="alert" class="sd-alert">{submitErrorText(submitError)}</div>
      {/if}

      <div class="sd-cols">
        <div class="sd-col sd-col-targets">
          <span class="sd-col-label">Predecessors ({currentTargets.length})</span>
          <div class="sd-chips">
            {#each currentTargets as t (t.id)}
              <div class="sd-chip">
                <span class="sd-cat-dot" style="background: var(--cat-{t.category})"></span>
                <span class="sd-sid">{t.shortId}</span>
                <span class="sd-sum">{t.summary || t.content}</span>
                <div class="sd-sub">has {t.supersedes.length} predecessor(s)</div>
              </div>
            {/each}
          </div>
        </div>

        <div class="sd-col sd-col-record">
          {#if prefillShortId}
            <div class="sd-prefill-note">Prefilled from newest predecessor {prefillShortId} · write the example correction</div>
          {/if}
          <label class="sd-field">
            <span class="sd-field-label">summary</span>
            <Input bind:value={draftFields.summary} />
          </label>
          <label class="sd-field">
            <span class="sd-field-label">content</span>
            <Textarea bind:value={draftFields.content} rows={8} />
          </label>
          <label class="sd-field">
            <span class="sd-field-label">category</span>
            <Select
              value={draftFields.category}
              options={CATEGORIES.map((c) => ({ value: c, label: c }))}
              ariaLabel="category"
              onValueChange={(v) => (draftFields = { ...draftFields, category: v })}
            />
          </label>
          <label class="sd-field">
            <span class="sd-field-label">scope</span>
            <Input bind:value={draftFields.scope} />
          </label>
          <div class="sd-field">
            <span class="sd-field-label">tags</span>
            <div class="sd-tags">
              {#each draftFields.tags as t (t)}
                <span class="sd-tag">
                  {t}
                  <button type="button" aria-label={`remove tag ${t}`} onclick={() => removeTag(t)}>×</button>
                </span>
              {/each}
              <Input class="h-6 w-28" aria-label="add tag" bind:value={tagInput} onkeydown={handleTagKeydown} />
            </div>
          </div>
        </div>
      </div>

      <Dialog.Footer>
        <Dialog.Close>
          {#snippet child({ props })}
            <Button variant="outline" {...props} disabled={submitPending}>Cancel</Button>
          {/snippet}
        </Dialog.Close>
        <Button disabled={!canSubmit} onclick={handleSubmit}>
          Supersede {currentTargets.length} → 1
        </Button>
      </Dialog.Footer>
    {/if}
  </Dialog.Content>
</Dialog.Root>

<style>
  .sd-cols {
    display: flex;
    gap: calc(16 * var(--u));
    container-type: inline-size;
    container-name: frame;
  }
  .sd-col {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: calc(8 * var(--u));
  }
  .sd-col-label {
    font-size: calc(10.5 * var(--u));
    text-transform: uppercase;
    color: var(--muted-foreground);
  }
  .sd-chips {
    display: flex;
    flex-direction: column;
    gap: calc(6 * var(--u));
    max-height: calc(360 * var(--u));
    overflow-y: auto;
  }
  .sd-chip {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: calc(6 * var(--u));
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
    padding: calc(4 * var(--u)) calc(6 * var(--u));
    font-size: calc(12 * var(--u));
  }
  .sd-cat-dot {
    width: calc(8 * var(--u));
    height: calc(8 * var(--u));
    border-radius: 50%;
    flex: none;
  }
  .sd-sid {
    font-family: var(--font-mono, monospace);
    flex: none;
    white-space: nowrap;
  }
  .sd-sum {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
    flex: 1;
    color: var(--muted-foreground);
  }
  .sd-sub {
    flex-basis: 100%;
    font-size: calc(10.5 * var(--u));
    color: var(--text-faint);
  }
  .sd-prefill-note {
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .sd-field {
    display: flex;
    flex-direction: column;
    gap: calc(2 * var(--u));
  }
  .sd-field-label {
    font-size: calc(10.5 * var(--u));
    text-transform: uppercase;
    color: var(--muted-foreground);
  }
  .sd-tags {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: calc(5 * var(--u));
  }
  .sd-tag {
    font-family: var(--font-mono, monospace);
    font-size: calc(10.5 * var(--u));
    background: var(--muted);
    border-radius: calc(3 * var(--u));
    padding: calc(2 * var(--u)) calc(6 * var(--u));
  }
  .sd-notice {
    font-size: calc(12 * var(--u));
    color: var(--warning);
  }
  .sd-loading {
    font-size: calc(12 * var(--u));
    color: var(--muted-foreground);
  }
  .sd-alert {
    font-size: calc(12 * var(--u));
    color: var(--cat-gotcha);
  }
  .sd-success-body {
    font-size: calc(12 * var(--u));
  }
  .sd-no-undo {
    font-weight: 600;
    font-size: calc(12 * var(--u));
  }

  @container frame (max-width: 700px) {
    .sd-cols {
      flex-direction: column;
    }
  }
</style>
