<script lang="ts">
  import { ConnectError, Code } from '@connectrpc/connect';
  import type { Memory } from '$lib/gen/engram_pb';
  import { CATEGORIES } from '$lib/queries';
  import { describeError } from '$lib/errors';
  import { persistResume, normalizeReturnPath, redirectToLogin } from '$lib/resume';
  import {
    useCreateMemory,
    useUpdateMemory,
    useScheduleMemory,
    normalizeVisibility,
    type CreateMemoryVars
  } from '$lib/mutations/memory';
  import * as Sheet from '$lib/components/ui/sheet';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Textarea } from '$lib/components/ui/textarea';
  import { Select } from '$lib/components/ui/select';
  import { Checkbox } from '$lib/components/ui/checkbox';
  import { Badge } from '$lib/components/ui/badge';
  import ShareWarningInline from './ShareWarningInline.svelte';

  // MemoryFormSheet ONLY persists a resume envelope on a hard PRIMARY-write
  // auth failure (via persistResume, Codex round-3 HIGH) -- it never reads or
  // deletes sessionStorage itself. Restoration is entirely PROP-driven:
  // `resumeValues` is supplied by the route/host (Plan 06), which owns
  // peek/consume/delete. This form applies the props to $state once and fires
  // `onresumeapplied` so the host can consumeResume().
  let {
    open = $bindable(false),
    mode,
    memory,
    scope: defaultScope,
    resumeValues,
    onresumeapplied
  }: {
    open?: boolean;
    mode: 'create' | 'edit';
    memory?: Memory;
    scope: string;
    resumeValues?: Record<string, unknown>;
    onresumeapplied?: () => void;
  } = $props();

  const isEdit = mode === 'edit';

  // Initial-value capture: the host keys this component by mode+recordId
  // (Plan 06), so a fresh instance is mounted per edit target -- reading
  // `memory`/`defaultScope` once at $state init time is correct, not a
  // reactivity gap.
  let content = $state(isEdit && memory ? memory.content : '');
  let scopeVal = $state(isEdit && memory ? memory.scope : defaultScope);
  let category = $state<string>(isEdit && memory ? memory.category : (CATEGORIES[0] ?? 'convention'));
  let tags = $state<string[]>(isEdit && memory ? [...memory.tags] : []);
  let tagInput = $state('');
  let summary = $state(isEdit && memory ? memory.summary : '');
  // Stored '' reads as private (Codex+grok MEDIUM normalization).
  let visibility = $state<'private' | 'shared'>(isEdit && memory ? normalizeVisibility(memory.visibility) : 'private');
  // Only set true by an explicit ShareWarningInline confirm -- the `shared`
  // intent (sharedIntent below) never fires until this is true.
  let shareAcknowledged = $state(false);
  // Schedule window -- CREATE mode only (ScheduleMemoryRequest has no id;
  // routing an edit through it would duplicate the record).
  let scheduleEnabled = $state(false);
  let notBefore = $state('');
  let notAfter = $state('');

  let submitting = $state(false);
  let hardAuthFailure = $state(false);
  let genericError = $state('');
  let resumeApplied = $state(false);

  // Locked D-07: an already-shared record's visibility control is READ-ONLY
  // in edit mode -- `shared` never enters its dirty mask, so `useUpdateMemory`
  // can never emit `shared:false` (no accidental unshare, round-4 MEDIUM). A
  // currently-private record may still be moved to shared (one-way).
  const isEditSharedReadOnly = $derived(isEdit && !!memory && normalizeVisibility(memory.visibility) === 'shared');
  const showShareWarning = $derived(!isEditSharedReadOnly && visibility === 'shared' && !shareAcknowledged);
  const sharedIntent = $derived(visibility === 'shared' && shareAcknowledged);

  function tagsEqual(a: string[], b: string[]): boolean {
    return a.length === b.length && a.every((t, i) => t === b[i]);
  }

  // Edit dirty-mask: only fields that actually changed vs. the original
  // record go into the update_mask (round-2 MEDIUM) -- re-sending an
  // unchanged content forces a needless re-embed and flips an unchanged
  // auto-summary's provenance. `shared` enters the mask ONLY as `true` on a
  // currently-private record (never `false`).
  const dirty = $derived.by(() => {
    const changes: { content?: string; tags?: string[]; summary?: string; shared?: boolean } = {};
    if (!isEdit || !memory) return changes;
    if (content !== memory.content) changes.content = content;
    if (!tagsEqual(tags, memory.tags)) changes.tags = tags;
    if (summary !== memory.summary) changes.summary = summary;
    if (!isEditSharedReadOnly && sharedIntent) changes.shared = true;
    return changes;
  });
  const hasChanges = $derived(Object.keys(dirty).length > 0);

  const contentError = $derived(content.trim() ? '' : 'content is required');
  const scopeTrimmed = $derived(scopeVal.trim());
  // StoreMemory/ScheduleMemory require scope.min_len=1 (engram.proto:106,210);
  // the search route defaults scope to '' -- fail fast client-side rather
  // than bouncing server-side with InvalidArgument (Codex round-3 MEDIUM).
  const scopeError = $derived(!isEdit && !scopeTrimmed ? 'scope is required' : '');

  function parseLocalDate(dtStr: string): Date | null {
    if (!dtStr) return null;
    const d = new Date(dtStr);
    return Number.isNaN(d.getTime()) ? null : d;
  }

  // Mirrors the proto CEL at engram.proto:203-208 client-side (Codex MEDIUM).
  const scheduleError = $derived.by(() => {
    if (isEdit || !scheduleEnabled) return '';
    const nb = parseLocalDate(notBefore);
    const na = parseLocalDate(notAfter);
    if (!nb && !na) return 'set a not-before or not-after window';
    if (nb && na && na.getTime() <= nb.getTime()) return 'not-after must be after not-before';
    return '';
  });

  const canSubmit = $derived(
    !submitting && !contentError && (isEdit ? hasChanges : !scopeError && !scheduleError)
  );

  function addTagFromInput() {
    const raw = tagInput.trim().replace(/,$/, '');
    if (raw && !tags.includes(raw)) tags = [...tags, raw];
    tagInput = '';
  }
  function handleTagKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      addTagFromInput();
    } else if (e.key === 'Backspace' && tagInput === '' && tags.length) {
      tags = tags.slice(0, -1);
    }
  }
  function removeTag(t: string) {
    tags = tags.filter((x) => x !== t);
  }

  function handleVisibilityChange(v: string) {
    visibility = v === 'shared' ? 'shared' : 'private';
    if (visibility === 'private') shareAcknowledged = false;
  }
  async function confirmShare() {
    shareAcknowledged = true;
  }
  function cancelShare() {
    visibility = 'private';
    shareAcknowledged = false;
  }

  const createMutation = useCreateMemory();
  const updateMutation = useUpdateMemory();
  const scheduleMutation = useScheduleMemory();

  function buildCreateVars(): CreateMemoryVars {
    return {
      content: content.trim(),
      scope: scopeTrimmed,
      category,
      tags,
      summary: summary.trim() || undefined,
      shared: sharedIntent
    };
  }

  // Consumes the Plan-04 composite's discriminated result (Codex round-3
  // HIGH): `created`/`created_shared`/`created_private` are ALL success --
  // `created_private` (secondary SetVisibility auth failure) never enters
  // the D-09 resubmit tier below, so a partial failure can never duplicate
  // the record.
  function handleWriteSuccess() {
    submitting = false;
    hardAuthFailure = false;
    genericError = '';
    open = false;
  }

  // Only a REJECTED promise from the PRIMARY create/schedule/update reaches
  // here -- a `created_private` composite result resolves via
  // handleWriteSuccess above, never this handler.
  function handleWriteError(err: unknown) {
    submitting = false;
    const ce = err instanceof ConnectError ? err : ConnectError.from(err);
    if (ce.code === Code.Unauthenticated || ce.code === Code.PermissionDenied) {
      hardAuthFailure = true;
    } else {
      genericError = `couldn't save — ${describeError(err)}`;
    }
  }

  function handleSubmit() {
    if (!canSubmit) return;
    genericError = '';
    submitting = true;
    if (!isEdit) {
      const vars = buildCreateVars();
      if (scheduleEnabled) {
        scheduleMutation.mutate(
          { ...vars, notBefore: parseLocalDate(notBefore) ?? undefined, notAfter: parseLocalDate(notAfter) ?? undefined },
          { onSuccess: handleWriteSuccess, onError: handleWriteError }
        );
      } else {
        createMutation.mutate(vars, { onSuccess: handleWriteSuccess, onError: handleWriteError });
      }
    } else if (memory) {
      updateMutation.mutate({ id: memory.id, ...dirty }, { onSuccess: handleWriteSuccess, onError: handleWriteError });
    }
  }

  // D-09 redirect tier: persists a versioned+TTL resume envelope BEFORE the
  // full OIDC redirect (whose callback always lands on /ui/, never the
  // originating route -- handlers.go:187). This form only PERSISTS; the
  // route/host (Plan 06) owns peek/consume/delete on the /ui/ landing.
  function handleReauthenticate() {
    const returnPath = normalizeReturnPath(window.location.pathname + window.location.search);
    if (isEdit && memory) {
      persistResume({
        returnPath,
        kind: 'memory',
        mode: 'edit',
        recordId: memory.id,
        values: dirty
      });
    } else {
      persistResume({
        returnPath,
        kind: 'memory',
        mode: 'create',
        recordId: null,
        values: { content, scope: scopeVal, category, tags, summary, visibility, scheduleEnabled, notBefore, notAfter }
      });
    }
    redirectToLogin();
  }

  // Prop-driven restore (never a self-read/delete of sessionStorage): applies
  // once when the host passes resumeValues in, then acknowledges via
  // onresumeapplied so the host can consumeResume(). Survives the host's
  // async edit-target fetch because it's driven by prop presence, not mount.
  $effect(() => {
    if (resumeValues && !resumeApplied) {
      resumeApplied = true;
      const rv = resumeValues;
      if (typeof rv.content === 'string') content = rv.content;
      if (typeof rv.scope === 'string') scopeVal = rv.scope;
      if (typeof rv.category === 'string') category = rv.category;
      if (Array.isArray(rv.tags)) tags = rv.tags as string[];
      if (typeof rv.summary === 'string') summary = rv.summary;
      if (rv.visibility === 'shared' || rv.shared === true) {
        visibility = 'shared';
        shareAcknowledged = true;
      }
      if (rv.scheduleEnabled !== undefined) scheduleEnabled = !!rv.scheduleEnabled;
      if (typeof rv.notBefore === 'string') notBefore = rv.notBefore;
      if (typeof rv.notAfter === 'string') notAfter = rv.notAfter;
      onresumeapplied?.();
    }
  });
</script>

<Sheet.Root bind:open>
  <Sheet.Content side="right" class="flex flex-col gap-0">
    <Sheet.Header>
      <Sheet.Title>{isEdit ? 'Edit memory' : 'New memory'}</Sheet.Title>
    </Sheet.Header>
    <div class="flex-1 flex flex-col gap-3 px-4 overflow-y-auto min-h-0">
      <div class="flex flex-col gap-1">
        <label for="mfs-content" class="text-[10.5px] uppercase text-muted-foreground">content</label>
        <Textarea id="mfs-content" bind:value={content} placeholder="write the memory…" rows={6} />
        {#if contentError}<span class="text-[11px] text-cat-gotcha">{contentError}</span>{/if}
      </div>

      <div class="flex flex-col gap-1">
        <span id="mfs-scope-label" class="text-[10.5px] uppercase text-muted-foreground">scope</span>
        {#if isEdit}
          <div class="text-[12px] font-mono text-muted-foreground" data-testid="scope-readonly">{scopeVal}</div>
        {:else}
          <Input aria-labelledby="mfs-scope-label" bind:value={scopeVal} placeholder="repo:..." />
          {#if scopeError}<span class="text-[11px] text-cat-gotcha">{scopeError}</span>{/if}
        {/if}
      </div>

      <div class="flex flex-col gap-1">
        <span class="text-[10.5px] uppercase text-muted-foreground">category</span>
        {#if isEdit}
          <div class="text-[12px] text-muted-foreground" data-testid="category-readonly">{category}</div>
        {:else}
          <Select
            value={category}
            options={CATEGORIES.map((c) => ({ value: c, label: c }))}
            ariaLabel="category"
            onValueChange={(v) => (category = v)}
          />
        {/if}
      </div>

      <div class="flex flex-col gap-1">
        <span class="text-[10.5px] uppercase text-muted-foreground">tags</span>
        <div class="flex flex-wrap gap-1.5 items-center">
          {#each tags as t (t)}
            <Badge variant="outline" class="bg-muted font-mono text-[10.5px] gap-1">
              {t}
              <button type="button" aria-label={`remove tag ${t}`} onclick={() => removeTag(t)}>×</button>
            </Badge>
          {/each}
          <Input
            class="h-6 w-28"
            aria-label="add tag"
            placeholder="add tag…"
            bind:value={tagInput}
            onkeydown={handleTagKeydown}
          />
        </div>
      </div>

      <div class="flex flex-col gap-1">
        <span class="text-[10.5px] uppercase text-muted-foreground">visibility</span>
        {#if isEditSharedReadOnly}
          <div class="text-[12px] text-muted-foreground" data-testid="visibility-readonly">shared</div>
        {:else}
          <Select
            value={visibility}
            options={[
              { value: 'private', label: 'private' },
              { value: 'shared', label: 'shared' }
            ]}
            ariaLabel="visibility"
            onValueChange={handleVisibilityChange}
          />
        {/if}
        {#if showShareWarning}
          <ShareWarningInline onconfirm={confirmShare} oncancel={cancelShare} />
        {/if}
      </div>

      <div class="flex flex-col gap-1">
        <label for="mfs-summary" class="text-[10.5px] uppercase text-muted-foreground">summary (optional)</label>
        <Textarea id="mfs-summary" bind:value={summary} rows={2} />
      </div>

      {#if !isEdit}
        <div class="flex flex-col gap-2 border-t border-border pt-2">
          <label class="flex items-center gap-2 text-[12px]">
            <Checkbox
              checked={scheduleEnabled}
              onCheckedChange={(v) => (scheduleEnabled = v === true)}
              aria-label="schedule this memory"
            />
            schedule this memory
          </label>
          {#if scheduleEnabled}
            <div class="flex flex-col gap-2">
              <label class="flex flex-col gap-1 text-[11px] text-muted-foreground" for="mfs-not-before">
                not before
                <Input id="mfs-not-before" type="datetime-local" bind:value={notBefore} />
              </label>
              <label class="flex flex-col gap-1 text-[11px] text-muted-foreground" for="mfs-not-after">
                not after
                <Input id="mfs-not-after" type="datetime-local" bind:value={notAfter} />
              </label>
              {#if scheduleError}<span class="text-[11px] text-cat-gotcha">{scheduleError}</span>{/if}
            </div>
          {/if}
        </div>
      {/if}

      {#if hardAuthFailure}
        <div role="alert" class="flex flex-col gap-2 p-3 text-cat-gotcha bg-card border border-cat-gotcha rounded text-[12px]">
          <span>write failed — session expired. re-authenticate to continue.</span>
          <Button variant="outline" size="sm" class="self-start" onclick={handleReauthenticate}>Re-authenticate</Button>
        </div>
      {:else if genericError}
        <div role="alert" class="text-cat-gotcha text-[12px]">{genericError}</div>
      {/if}
    </div>

    <Sheet.Footer>
      <Sheet.Close>
        {#snippet child({ props })}
          <Button variant="outline" {...props}>Cancel</Button>
        {/snippet}
      </Sheet.Close>
      <Button disabled={!canSubmit} onclick={handleSubmit}>{isEdit ? 'Save' : 'Create'}</Button>
    </Sheet.Footer>
  </Sheet.Content>
</Sheet.Root>
