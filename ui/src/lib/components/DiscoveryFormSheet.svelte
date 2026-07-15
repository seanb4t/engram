<script lang="ts">
  import { ConnectError, Code } from '@connectrpc/connect';
  import { describeError } from '$lib/errors';
  import { persistResume, normalizeReturnPath, redirectToLogin } from '$lib/resume';
  import { useCreateDiscovery, type DiscoveryCitationInput } from '$lib/mutations/discovery';
  import * as Sheet from '$lib/components/ui/sheet';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Textarea } from '$lib/components/ui/textarea';
  import { Select } from '$lib/components/ui/select';
  import { Badge } from '$lib/components/ui/badge';
  import ShareWarningInline from './ShareWarningInline.svelte';

  // Create-only Sheet (D-04 fence -- no edit surface, no memory/recordId
  // prop). Mirrors MemoryFormSheet's D-09 resume lifecycle: this form ONLY
  // persists a resume envelope on a hard PRIMARY-write auth failure; the
  // route/host (Plan 06) owns peek/consume/delete and passes restored
  // values back in as `resumeValues` props.
  let {
    open = $bindable(false),
    scope: defaultScope,
    resumeValues,
    onresumeapplied
  }: {
    open?: boolean;
    scope: string;
    resumeValues?: Record<string, unknown>;
    onresumeapplied?: () => void;
  } = $props();

  let content = $state('');
  let kind = $state<'map' | 'fact'>('map');
  let scopeVal = $state(defaultScope);
  let citations = $state<DiscoveryCitationInput[]>([{ kind: 'file', ref: '' }]);
  let tags = $state<string[]>([]);
  let tagInput = $state('');
  let summary = $state('');
  let visibility = $state<'private' | 'shared'>('private');
  let shareAcknowledged = $state(false);

  let submitting = $state(false);
  let hardAuthFailure = $state(false);
  let genericError = $state('');
  let resumeApplied = $state(false);

  const showShareWarning = $derived(visibility === 'shared' && !shareAcknowledged);
  const sharedIntent = $derived(visibility === 'shared' && shareAcknowledged);

  const contentError = $derived(content.trim() ? '' : 'content is required');
  const scopeTrimmed = $derived(scopeVal.trim());
  // StoreDiscoveryRequest requires a `discovery:`-prefixed scope
  // (engram.proto:130, tools.go:575-602) -- fail fast client-side.
  const scopeError = $derived(
    !scopeTrimmed ? 'scope is required' : !scopeTrimmed.startsWith('discovery:') ? 'scope must start with discovery:' : ''
  );
  // Every citation needs a non-empty ref (kind always has a valid default
  // from the select); at least one citation is required.
  const citationsError = $derived(
    citations.length === 0
      ? 'at least one citation is required'
      : citations.some((c) => !c.ref.trim())
        ? 'every citation needs a ref'
        : ''
  );

  const canSubmit = $derived(!submitting && !contentError && !scopeError && !citationsError);

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

  function addCitation() {
    citations = [...citations, { kind: 'file', ref: '' }];
  }
  function removeCitation(i: number) {
    citations = citations.filter((_, idx) => idx !== i);
  }
  function updateCitationKind(i: number, v: string) {
    const nextKind = (['file', 'commit', 'url', 'repo'] as const).includes(v as DiscoveryCitationInput['kind'])
      ? (v as DiscoveryCitationInput['kind'])
      : 'file';
    citations = citations.map((c, idx) => (idx === i ? { ...c, kind: nextKind } : c));
  }
  function updateCitationRef(i: number, ref: string) {
    citations = citations.map((c, idx) => (idx === i ? { ...c, ref } : c));
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

  const createMutation = useCreateDiscovery();

  // Consumes the Plan-04 composite's discriminated result exactly like
  // MemoryFormSheet: created/created_shared/created_private are ALL
  // success -- created_private (secondary SetVisibility auth failure) never
  // enters the D-09 resubmit tier, so a partial failure can never duplicate
  // the record.
  function handleWriteSuccess() {
    submitting = false;
    hardAuthFailure = false;
    genericError = '';
    open = false;
  }

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
    createMutation.mutate(
      {
        content: content.trim(),
        kind,
        scope: scopeTrimmed,
        citations: citations.map((c) => ({ kind: c.kind, ref: c.ref.trim() })),
        tags,
        summary: summary.trim() || undefined,
        shared: sharedIntent
      },
      { onSuccess: handleWriteSuccess, onError: handleWriteError }
    );
  }

  // D-09 redirect tier -- persist-only, mirrors MemoryFormSheet.
  function handleReauthenticate() {
    const returnPath = normalizeReturnPath(window.location.pathname + window.location.search);
    persistResume({
      returnPath,
      kind: 'discovery',
      mode: 'create',
      recordId: null,
      values: { content, kind, scope: scopeVal, citations, tags, summary, visibility }
    });
    redirectToLogin();
  }

  // Prop-driven restore -- applies once, never self-reads/deletes storage.
  $effect(() => {
    if (resumeValues && !resumeApplied) {
      resumeApplied = true;
      const rv = resumeValues;
      if (typeof rv.content === 'string') content = rv.content;
      if (rv.kind === 'map' || rv.kind === 'fact') kind = rv.kind;
      if (typeof rv.scope === 'string') scopeVal = rv.scope;
      if (Array.isArray(rv.citations)) citations = rv.citations as DiscoveryCitationInput[];
      if (Array.isArray(rv.tags)) tags = rv.tags as string[];
      if (typeof rv.summary === 'string') summary = rv.summary;
      if (rv.visibility === 'shared') {
        visibility = 'shared';
        shareAcknowledged = true;
      }
      onresumeapplied?.();
    }
  });
</script>

<Sheet.Root bind:open>
  <Sheet.Content side="right" class="flex flex-col gap-0">
    <Sheet.Header>
      <Sheet.Title>New discovery</Sheet.Title>
    </Sheet.Header>
    <div class="flex-1 flex flex-col gap-3 px-4 overflow-y-auto min-h-0">
      <div class="flex flex-col gap-1">
        <label for="dfs-content" class="text-[10.5px] uppercase text-muted-foreground">content</label>
        <Textarea id="dfs-content" bind:value={content} placeholder="write the discovery…" rows={6} />
        {#if contentError}<span class="text-[11px] text-cat-gotcha">{contentError}</span>{/if}
      </div>

      <div class="flex flex-col gap-1">
        <span class="text-[10.5px] uppercase text-muted-foreground">kind</span>
        <Select
          value={kind}
          options={[
            { value: 'map', label: 'map' },
            { value: 'fact', label: 'fact' }
          ]}
          ariaLabel="kind"
          onValueChange={(v) => (kind = v === 'fact' ? 'fact' : 'map')}
        />
      </div>

      <div class="flex flex-col gap-1">
        <span id="dfs-scope-label" class="text-[10.5px] uppercase text-muted-foreground">scope</span>
        <Input aria-labelledby="dfs-scope-label" bind:value={scopeVal} placeholder="discovery:repo:..." />
        {#if scopeError}<span class="text-[11px] text-cat-gotcha">{scopeError}</span>{/if}
      </div>

      <div class="flex flex-col gap-2">
        <span class="text-[10.5px] uppercase text-muted-foreground">citations</span>
        {#each citations as c, i (i)}
          <div class="flex gap-1.5 items-center" data-testid="citation-row">
            <Select
              value={c.kind}
              options={[
                { value: 'file', label: 'file' },
                { value: 'commit', label: 'commit' },
                { value: 'url', label: 'url' },
                { value: 'repo', label: 'repo' }
              ]}
              ariaLabel={`citation ${i + 1} kind`}
              onValueChange={(v) => updateCitationKind(i, v)}
            />
            <Input
              class="flex-1"
              aria-label={`citation ${i + 1} ref`}
              placeholder="ref…"
              value={c.ref}
              oninput={(e) => updateCitationRef(i, (e.currentTarget as HTMLInputElement).value)}
            />
            <Button
              variant="ghost"
              size="icon-sm"
              type="button"
              aria-label={`remove citation ${i + 1}`}
              disabled={citations.length <= 1}
              onclick={() => removeCitation(i)}
            >
              ×
            </Button>
          </div>
        {/each}
        <Button variant="outline" size="sm" type="button" class="self-start" onclick={addCitation}>add citation</Button>
        {#if citationsError}<span class="text-[11px] text-cat-gotcha">{citationsError}</span>{/if}
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
        <Select
          value={visibility}
          options={[
            { value: 'private', label: 'private' },
            { value: 'shared', label: 'shared' }
          ]}
          ariaLabel="visibility"
          onValueChange={handleVisibilityChange}
        />
        {#if showShareWarning}
          <ShareWarningInline onconfirm={confirmShare} oncancel={cancelShare} />
        {/if}
      </div>

      <div class="flex flex-col gap-1">
        <label for="dfs-summary" class="text-[10.5px] uppercase text-muted-foreground">summary (optional)</label>
        <Textarea id="dfs-summary" bind:value={summary} rows={2} />
      </div>

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
      <Button disabled={!canSubmit} onclick={handleSubmit}>Create</Button>
    </Sheet.Footer>
  </Sheet.Content>
</Sheet.Root>
