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
  import * as Command from '$lib/components/ui/command';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Textarea } from '$lib/components/ui/textarea';
  import { Select } from '$lib/components/ui/select';
  import type { Memory, SupersedeMemoryResponse } from '$lib/gen/engram_pb';
  import { CATEGORIES } from '$lib/queries';
  import { classifyInput } from '$lib/search/classify';
  import type { SupersedeDraft, SupersedeFields } from '$lib/mutations/curation';
  import { parseSupersedeRejection, TARGET_ISSUE_COPY, NOT_FOUND_NOTE, type SupersedeRejection } from '$lib/curation/supersede-rejection';

  // The debounce window between the last draft edit and the auto-fired
  // validate_only preview (Task 1 action text).
  const PREVIEW_DEBOUNCE_MS = 250;
  const ADD_LOOKUP_DEBOUNCE_MS = 200;
  const SUMMARY_MAX_BYTES = 512;

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
    onlookup,
    onresolvehead,
    oncancel,
    ondone,
    onviewsuperseded,
    onopenrecord,
    onreauth
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
    onlookup?: (value: string) => Promise<Memory | undefined>;
    onresolvehead?: (id: string) => Promise<Memory | undefined>;
    oncancel: () => void;
    ondone: () => void;
    onviewsuperseded?: (ids: string[]) => void;
    onopenrecord?: (id: string) => void;
    onreauth?: (draft: SupersedeDraft) => void;
  } = $props();

  // currentTargets is the working chip set -- resets to `targets` whenever the
  // host hands us a fresh array (a new openSupersede call), same "reset on
  // reference change, not on every re-render" discipline as
  // ArchiveConfirmDialog's currentIds. A "use head" swap or an add-by-
  // short_id mutates it directly (never via a prop change), which is why the
  // reset guard keys on the PROP's identity, not the local array's.
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
  let contentTouched = $state(false);
  const hasContent = $derived(draftFields.content.trim().length > 0);

  // ---------------------------------------------------------------------------
  // Per-target issues: client-computed instantly (rule category, a target's
  // own supersededBy already set), overlaid by the LATEST server rejection
  // (preview or commit) by matching the rejection's raw inputs (id or
  // short_id) against each chip. The server's answer always wins over a
  // client guess for the SAME target when both exist.
  // ---------------------------------------------------------------------------

  const headRequestsInFlight = new Set<string>();
  let resolvedHeads = $state<Record<string, Memory>>({});
  $effect(() => {
    for (const t of currentTargets) {
      if (t.supersededBy && !headRequestsInFlight.has(t.id) && onresolvehead) {
        headRequestsInFlight.add(t.id);
        onresolvehead(t.id).then((head) => {
          if (head) resolvedHeads = { ...resolvedHeads, [t.id]: head };
        });
      }
    }
  });

  let previewRejection = $state<unknown>(undefined);
  let submitError = $state<unknown>(undefined);
  const previewRejectionParsed = $derived<SupersedeRejection | undefined>(
    previewRejection ? parseSupersedeRejection(previewRejection) : undefined
  );
  const submitRejectionParsed = $derived<SupersedeRejection | undefined>(
    submitError ? parseSupersedeRejection(submitError) : undefined
  );
  // The commit's own rejection is the more recent signal once it exists.
  const latestRejection = $derived(submitRejectionParsed ?? previewRejectionParsed);

  interface TargetIssue {
    text: string;
    note?: string;
    useHeadShortId?: string;
    onUseHead?: () => void;
  }

  const targetIssues = $derived.by(() => {
    const map = new Map<string, TargetIssue>();
    for (const t of currentTargets) {
      if (t.category === 'rule') {
        map.set(t.id, { text: TARGET_ISSUE_COPY.rule });
      } else if (t.supersededBy) {
        const head = resolvedHeads[t.id];
        map.set(t.id, {
          text: TARGET_ISSUE_COPY.alreadySuperseded(head?.shortId ?? '…'),
          useHeadShortId: head?.shortId,
          onUseHead: head ? () => useHead(t, head) : undefined
        });
      }
    }
    if (latestRejection?.kind === 'targets') {
      for (const input of latestRejection.inputs) {
        const t = currentTargets.find((x) => x.id === input || x.shortId === input);
        if (!t) continue;
        if (latestRejection.issue === 'rule') {
          map.set(t.id, { text: TARGET_ISSUE_COPY.rule });
        } else if (latestRejection.issue === 'already-superseded') {
          map.set(t.id, { text: TARGET_ISSUE_COPY.serverRace });
        } else if (latestRejection.issue === 'not-found') {
          if (t.visibility === 'shared') {
            map.set(t.id, { text: TARGET_ISSUE_COPY.notOwned(t.owner) });
          } else {
            map.set(t.id, { text: `not found: ${t.shortId}`, note: NOT_FOUND_NOTE });
          }
        }
      }
    }
    return map;
  });

  function useHead(target: Memory, head: Memory) {
    currentTargets = currentTargets.map((t) => (t.id === target.id ? head : t));
  }

  function removeChip(id: string) {
    currentTargets = currentTargets.filter((t) => t.id !== id);
  }

  // ---------------------------------------------------------------------------
  // Add target by id/short_id
  // ---------------------------------------------------------------------------

  let addValue = $state('');
  let addLookupPending = $state(false);
  let addResult = $state<Memory | undefined>(undefined);

  $effect(() => {
    const raw = addValue.trim();
    addResult = undefined;
    if (!raw || !onlookup) {
      addLookupPending = false;
      return;
    }
    const classified = classifyInput(raw);
    if (classified.kind !== 'id' && classified.kind !== 'short_id') {
      addLookupPending = false;
      return;
    }
    const timer = setTimeout(async () => {
      addLookupPending = true;
      try {
        const found = await onlookup(raw);
        addLookupPending = false;
        addResult = found;
      } catch {
        addLookupPending = false;
      }
    }, ADD_LOOKUP_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  });

  function selectAddResult(m: Memory) {
    if (!currentTargets.some((t) => t.id === m.id)) currentTargets = [...currentTargets, m];
    addValue = '';
    addResult = undefined;
  }

  // ---------------------------------------------------------------------------
  // Gates (in order) and byte counter / cross-scope warning
  // ---------------------------------------------------------------------------

  const invalidCount = $derived(currentTargets.filter((t) => targetIssues.has(t.id)).length);
  const gateReady = $derived(currentTargets.length > 0 && invalidCount === 0 && hasContent);
  const gateText = $derived.by(() => {
    if (currentTargets.length === 0) return 'Add at least one target';
    if (invalidCount > 0) return `Remove ${invalidCount} invalid target(s) — an invalid set rejects the whole call`;
    if (!hasContent) return 'content is required';
    return 'Additive: predecessors get superseded_by, nothing is deleted';
  });

  const summaryBytes = $derived(new TextEncoder().encode(draftFields.summary).length);
  const summaryDanger = $derived(summaryBytes > SUMMARY_MAX_BYTES);

  const targetScopes = $derived([...new Set(currentTargets.map((t) => t.scope))]);
  const scopeOptions = $derived(targetScopes.filter((s) => !s.startsWith('rule:')));

  // ---------------------------------------------------------------------------
  // Chain preview: derived-only, never keyed on the draft inputs -- rendering
  // it must never remount the summary/content fields (no focus loss).
  // ---------------------------------------------------------------------------

  const chainD2Ids = $derived([...new Set(currentTargets.flatMap((t) => t.supersedes))]);

  // ---------------------------------------------------------------------------
  // Validate_only preview: debounced, revision-guarded so a stale answer for
  // an earlier draft never marks a LATER draft as validated.
  // ---------------------------------------------------------------------------

  let previewRevision = 0;
  let previewPending = $state(false);
  let previewValidated = $state(false);
  let previewValidatedRevision = -1;

  $effect(() => {
    // Read every input this preview depends on so Svelte tracks them.
    const ids = currentIds;
    const f = { ...draftFields };
    const rev = ++previewRevision;
    previewValidated = false;
    previewRejection = undefined;
    submitError = undefined;
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
    gateReady && !previewPending && previewValidated && previewValidatedRevision === previewRevision && !submitPending
  );

  let submitPending = $state(false);
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

  function handleReauth() {
    onreauth?.({ targets: currentIds, fields: { ...draftFields }, idempotencyKey });
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

      {#if previewPending || submitPending}
        <div role="status" class="sd-loading">supersede_memory · {currentTargets.length} targets…</div>
      {/if}

      {#if submitRejectionParsed}
        {#if submitRejectionParsed.kind === 'field'}
          <div role="alert" class="sd-alert">
            <span>Rejected — fix the named field and resend</span>
            <div class="sd-pills">
              <span class="sd-pill">field={submitRejectionParsed.fields.join(',')}</span>
              <span class="sd-pill">hint={submitRejectionParsed.hint}</span>
            </div>
            <div class="sd-detail">{submitRejectionParsed.detail}</div>
          </div>
        {:else if submitRejectionParsed.kind === 'targets'}
          <div role="alert" class="sd-alert">
            <span>Server rejected the call — nothing was written</span>
            {#if submitRejectionParsed.issue === 'already-superseded'}
              <span class="sd-pill">target is already superseded</span>
              <div class="sd-detail">
                Another session superseded this target after you opened the editor. Supersede its current head instead.
              </div>
            {/if}
          </div>
        {:else if submitRejectionParsed.kind === 'reauth'}
          <div role="alert" class="sd-alert">
            <span>Session expired. Nothing was written; your draft is kept.</span>
            <Button variant="outline" size="sm" class="self-start" onclick={handleReauth}>Re-authenticate</Button>
          </div>
        {:else}
          <div role="alert" class="sd-alert">
            <span>Could not supersede — {submitRejectionParsed.codeName}</span>
            <Button variant="outline" size="sm" class="self-start" onclick={handleSubmit}>Retry</Button>
          </div>
        {/if}
      {/if}

      <div class="sd-cols">
        <div class="sd-col sd-col-targets">
          <span class="sd-col-label">Predecessors ({currentTargets.length})</span>
          <div class="sd-chips">
            {#each currentTargets as t (t.id)}
              {@const issue = targetIssues.get(t.id)}
              <div class="sd-chip" class:sd-chip-bad={!!issue}>
                <span class="sd-cat-dot" style="background: var(--cat-{t.category})"></span>
                <span class="sd-sid">{t.shortId}</span>
                <span class="sd-sum">{t.summary || t.content}</span>
                <button
                  type="button"
                  class="sd-chip-remove"
                  aria-label={`remove target ${t.shortId}`}
                  onclick={() => removeChip(t.id)}
                >
                  ×
                </button>
                {#if issue}
                  <div class="sd-tissue">
                    {issue.text}
                    {#if issue.useHeadShortId}
                      <button type="button" class="sd-use-head" onclick={issue.onUseHead}>
                        use head {issue.useHeadShortId}
                      </button>
                    {/if}
                  </div>
                  {#if issue.note}
                    <div class="sd-tissue-note">{issue.note}</div>
                  {/if}
                {:else}
                  <div class="sd-sub">has {t.supersedes.length} predecessor(s)</div>
                {/if}
              </div>
            {/each}
          </div>

          <div class="sd-add-target">
            <Input placeholder="add target by short_id…" aria-label="add target by short_id" bind:value={addValue} />
            {#if addLookupPending || addResult || addValue.trim()}
              <Command.Root shouldFilter={false} label="Add target" class="sd-add-results">
                <Command.List>
                  {#if addLookupPending}
                    <div class="sd-add-status">looking up…</div>
                  {:else if addResult}
                    <Command.Item value={addResult.id} onSelect={() => selectAddResult(addResult!)}>
                      {addResult.shortId} — {addResult.summary || addResult.content}
                    </Command.Item>
                  {:else}
                    <div class="sd-add-status">no match</div>
                  {/if}
                </Command.List>
              </Command.Root>
            {/if}
          </div>
        </div>

        <div class="sd-col sd-col-record">
          {#if prefillShortId}
            <div class="sd-prefill-note">Prefilled from newest predecessor {prefillShortId} · write the example correction</div>
          {/if}
          <label class="sd-field">
            <span class="sd-field-label">summary</span>
            <Input aria-label="summary" bind:value={draftFields.summary} />
            <span class="sd-byte-counter" class:sd-byte-counter-danger={summaryDanger}>{summaryBytes}/{SUMMARY_MAX_BYTES} B</span>
          </label>
          <label class="sd-field">
            <span class="sd-field-label">content</span>
            <Textarea aria-label="content" bind:value={draftFields.content} rows={8} onblur={() => (contentTouched = true)} />
            {#if contentTouched && !hasContent}
              <span class="sd-field-error">field=content hint=required: content must not be empty</span>
            {/if}
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
            <Select
              value={draftFields.scope}
              options={scopeOptions.map((s) => ({ value: s, label: s }))}
              ariaLabel="scope"
              onValueChange={(v) => (draftFields = { ...draftFields, scope: v })}
            />
          </label>
          {#if targetScopes.length > 1}
            <div class="sd-cross-scope">Targets span {targetScopes.length} scopes — the new record lands in one.</div>
          {/if}
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

      <div class="chain-cols" role="list" aria-label="Supersession chain preview">
        <div class="ccol" role="listitem">
          <span class="ccol-h">d2</span>
          {#each chainD2Ids as id (id)}
            <div class="cnode placeholder">{id}</div>
          {/each}
        </div>
        <div class="ccol" role="listitem">
          <span class="ccol-h">d1</span>
          {#each currentTargets as t (t.id)}
            <div class="cnode" class:bad={targetIssues.has(t.id)}>{t.shortId}</div>
          {/each}
        </div>
        <div class="ccol" role="listitem">
          <span class="ccol-h">head · d0</span>
          <div class="cnode new">(new)</div>
        </div>
      </div>
      <div class="chain-legend">
        ← predecessors (superseded, hidden from recall) · arrows read 'superseded by' · head is the live record
      </div>

      {#if gateText}
        <div class="sd-gate" class:sd-gate-ready={gateReady}>{gateText}</div>
      {/if}

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
    max-height: calc(300 * var(--u));
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
  .sd-chip-bad {
    border-color: color-mix(in srgb, var(--destructive) 45%, var(--border));
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
  .sd-chip-remove {
    flex: none;
    border: none;
    background: none;
    cursor: pointer;
    color: var(--text-faint);
    font-size: calc(14 * var(--u));
    line-height: 1;
    padding: 0 calc(2 * var(--u));
  }
  .sd-sub {
    flex-basis: 100%;
    font-size: calc(10.5 * var(--u));
    color: var(--text-faint);
  }
  .sd-tissue {
    flex-basis: 100%;
    font-size: calc(11 * var(--u));
    color: var(--destructive);
  }
  .sd-tissue-note {
    flex-basis: 100%;
    font-size: calc(10.5 * var(--u));
    color: var(--text-faint);
  }
  .sd-use-head {
    background: none;
    border: none;
    padding: 0;
    margin-left: calc(4 * var(--u));
    color: var(--primary);
    text-decoration: underline;
    cursor: pointer;
    font: inherit;
  }
  .sd-add-target {
    display: flex;
    flex-direction: column;
    gap: calc(2 * var(--u));
  }
  .sd-add-status {
    padding: calc(4 * var(--u)) calc(6 * var(--u));
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
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
  .sd-field-error {
    font-size: calc(11 * var(--u));
    color: var(--cat-gotcha);
    font-family: var(--font-mono, monospace);
  }
  .sd-byte-counter {
    align-self: flex-end;
    font-size: calc(10.5 * var(--u));
    color: var(--text-faint);
    font-family: var(--font-mono, monospace);
  }
  .sd-byte-counter-danger {
    color: var(--destructive);
  }
  .sd-cross-scope {
    font-size: calc(11 * var(--u));
    color: var(--warning);
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
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
    font-size: calc(12 * var(--u));
    color: var(--cat-gotcha);
  }
  .sd-pills {
    display: flex;
    gap: calc(6 * var(--u));
  }
  .sd-pill {
    font-family: var(--font-mono, monospace);
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
    padding: 0 calc(5 * var(--u));
  }
  .sd-detail {
    font-family: var(--font-mono, monospace);
    color: var(--text-faint);
  }
  .sd-success-body {
    font-size: calc(12 * var(--u));
  }
  .sd-no-undo {
    font-weight: 600;
    font-size: calc(12 * var(--u));
  }
  .sd-gate {
    font-size: calc(11.5 * var(--u));
    color: var(--destructive);
  }
  .sd-gate-ready {
    color: var(--text-faint);
  }

  .chain-cols {
    display: flex;
    gap: calc(10 * var(--u));
    overflow-x: auto;
  }
  .ccol {
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
    flex: none;
  }
  .ccol-h {
    font-size: calc(10 * var(--u));
    color: var(--muted-foreground);
    text-transform: uppercase;
  }
  .cnode {
    font-family: var(--font-mono, monospace);
    font-size: calc(11 * var(--u));
    border: 1px solid var(--border);
    border-radius: calc(4 * var(--u));
    padding: calc(2 * var(--u)) calc(6 * var(--u));
  }
  .cnode.bad {
    text-decoration: line-through;
    color: var(--destructive);
  }
  .cnode.new {
    border-style: dashed;
  }
  .cnode.placeholder {
    color: var(--text-faint);
  }
  .chain-legend {
    font-size: calc(10.5 * var(--u));
    color: var(--text-faint);
  }

  @container frame (max-width: 700px) {
    .sd-cols {
      flex-direction: column;
    }
  }
</style>
