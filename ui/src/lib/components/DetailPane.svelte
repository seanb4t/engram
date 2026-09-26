<script lang="ts">
  import type { Memory } from '$lib/gen/engram_pb';
  import { timestampDate } from '@bufbuild/protobuf/wkt';
  import { ConnectError, Code } from '@connectrpc/connect';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Separator } from '$lib/components/ui/separator';
  import { Skeleton } from '$lib/components/ui/skeleton';
  import * as Tooltip from '$lib/components/ui/tooltip';
  import { toast } from 'svelte-sonner';
  import { relativeTime, fullTimestamp } from '$lib/time';
  import { memoryStateWords } from '$lib/memorystate';
  import { renderMarkdown } from '$lib/markdown';
  import { stripCategoryPrefix } from '$lib/summary';
  import { normalizeVisibility } from '$lib/mutations/memory';
  import CopyIcon from '@lucide/svelte/icons/copy';
  import XIcon from '@lucide/svelte/icons/x';

  const CURATION_TOOLTIP = 'Arrives with curation — Phase 4';

  // D-14/ROW-07: the record view is stacked sections in a fixed order — no
  // tabs. MemoryDetail.svelte (tabs) stays untouched and keeps serving
  // /discovery; this component replaces it for /search (this plan) and
  // / and /observe (plan 02-10).
  let {
    memory,
    loading,
    error,
    requestedId,
    inResults = true,
    hit,
    onclose,
    onselect,
    ontag,
    onedit,
    onvisibility,
    ondelete
  }: {
    memory: Memory | undefined;
    loading: boolean;
    error: unknown;
    requestedId: string;
    inResults?: boolean;
    hit?: { score: number; relevance?: number };
    onclose?: () => void;
    onselect?: (id: string) => void;
    ontag?: (tag: string) => void;
    onedit?: (id: string) => void;
    onvisibility?: (memory: Memory) => void;
    ondelete?: (id: string) => void;
  } = $props();

  // D-15/D-16 rule fence (mechanical, not "parent omits callbacks"): a rule
  // record never shows Edit or Share/Make private (delete only); a discovery
  // record (kind non-empty -- discovery-only field, empty on plain memories)
  // never shows Edit.
  const isRule = $derived(memory?.category === 'rule');
  const isDiscovery = $derived(!!memory?.kind);
  const isShared = $derived(memory ? normalizeVisibility(memory.visibility) === 'shared' : false);

  const notFound = $derived(error instanceof ConnectError && error.code === Code.NotFound);
  const errorCodeName = $derived(error instanceof ConnectError ? Code[error.code] : undefined);

  // D-13/D-14: state words gate the State section; supersedes-present alone
  // (a pure successor carrying none of the four state words itself) also
  // opens the section so its predecessor links still render.
  const stateWords = $derived(memory ? memoryStateWords(memory) : []);
  const hasState = $derived(stateWords.length > 0 || (memory?.supersedes.length ?? 0) > 0);
  const hasTags = $derived((memory?.tags.length ?? 0) > 0);
  const hasCitations = $derived((memory?.citations.length ?? 0) > 0);
  const closesInFuture = $derived(!!memory?.notAfter && timestampDate(memory.notAfter) > new Date());

  const title = $derived.by(() => {
    if (!memory) return '';
    const summary = memory.summary?.trim();
    if (summary) return summary;
    const stripped = stripCategoryPrefix(memory.content ?? '', memory.category);
    return stripped.split('\n')[0] ?? '';
  });

  const bodyHtml = $derived(memory ? renderMarkdown(memory.content) : '');

  async function copyText(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      toast.success('copied');
    } catch {
      // clipboard write can reject (denied permission, insecure context,
      // lost focus) -- surface it so the button never appears to silently
      // do nothing.
      toast.error('copy failed');
    }
  }
</script>

<aside class="detail-pane" tabindex="-1" aria-label="Memory detail">
  <div class="d-head">
    {#if memory}
      <Badge variant="outline" class="d-cat-badge" style="color:var(--cat-{memory.category})">{memory.category}</Badge>
      <span class="d-sid">{memory.shortId}</span>
      <Button variant="ghost" size="icon-sm" aria-label="copy short_id" onclick={() => copyText(memory!.shortId)}>
        <CopyIcon />
      </Button>
      <span class="d-vis">{memory.visibility}</span>
    {:else}
      <span class="d-sid">{requestedId}</span>
    {/if}
    <span class="d-grow"></span>
    <Button variant="ghost" size="icon-sm" aria-label="close" onclick={() => onclose?.()}><XIcon /></Button>
  </div>

  <div class="d-body">
    {#if loading}
      <div class="d-loading">
        <Skeleton class="h-[calc(12*var(--u))] w-[70%]" />
        <Skeleton class="h-[calc(12*var(--u))] w-full" />
        <Skeleton class="h-[calc(12*var(--u))] w-[85%]" />
        <Skeleton class="h-[calc(12*var(--u))] w-[60%]" />
      </div>
    {:else if notFound}
      <div class="d-error" role="alert">
        No memory with id {requestedId} that you can read · not-found and not-yours look the same by design
      </div>
    {:else if error}
      <div class="d-error" role="alert">Could not load this record{errorCodeName ? ` · ${errorCodeName}` : ''}</div>
    {:else if memory}
      {#if !inResults}
        <div class="d-note">
          Not in the current results (filtered out or not matched). Showing it by id, like get_memory.
        </div>
      {/if}
      <h2 class="d-title">{title}</h2>
      <div class="d-actions">
        {#if !isRule && !isDiscovery}
          <Button variant="outline" size="sm" onclick={() => onedit?.(memory!.id)}>Edit</Button>
        {/if}

        <Tooltip.Provider delayDuration={0}>
          <Tooltip.Root>
            <Tooltip.Trigger>
              {#snippet child({ props })}
                <!-- svelte-ignore a11y_no_noninteractive_tabindex -- a wrapping focus target for a disabled button's tooltip is the standard accessible pattern; the real disabled <button> inside stays inert. -->
                <span {...props} tabindex="0" class="d-tooltip-wrap">
                  <Button variant="outline" size="sm" disabled>Supersede…</Button>
                </span>
              {/snippet}
            </Tooltip.Trigger>
            <Tooltip.Content>{CURATION_TOOLTIP}</Tooltip.Content>
          </Tooltip.Root>
        </Tooltip.Provider>

        <Tooltip.Provider delayDuration={0}>
          <Tooltip.Root>
            <Tooltip.Trigger>
              {#snippet child({ props })}
                <!-- svelte-ignore a11y_no_noninteractive_tabindex -- see the Supersede trigger above. -->
                <span {...props} tabindex="0" class="d-tooltip-wrap">
                  <Button variant="outline" size="sm" disabled>{memory.archivedAt ? 'Restore' : 'Archive'}</Button>
                </span>
              {/snippet}
            </Tooltip.Trigger>
            <Tooltip.Content>{CURATION_TOOLTIP}</Tooltip.Content>
          </Tooltip.Root>
        </Tooltip.Provider>

        {#if !isRule}
          <Button variant="outline" size="sm" onclick={() => onvisibility?.(memory!)}>{isShared ? 'Make private' : 'Share'}</Button>
        {/if}

        <Button variant="destructive" size="sm" onclick={() => ondelete?.(memory!.id)}>Delete</Button>
      </div>

      {#if hasState}
        <Separator />
        <section class="d-sec">
          <h3>State</h3>
          <div class="d-statelist">
            {#if stateWords.includes('archived') && memory.archivedAt}
              {@const archivedDate = timestampDate(memory.archivedAt)}
              <div>archived since {fullTimestamp(archivedDate)} · {relativeTime(archivedDate)}</div>
            {/if}
            {#if stateWords.includes('superseded') && memory.supersededBy}
              <div>
                superseded by
                <button type="button" class="d-link" onclick={() => onselect?.(memory!.supersededBy!)}>{memory.supersededBy}</button>
              </div>
            {/if}
            {#if memory.supersedes.length > 0}
              <div>
                supersedes
                {#each memory.supersedes as predId (predId)}
                  <button type="button" class="d-link" onclick={() => onselect?.(predId)}>{predId}</button>
                {/each}
              </div>
            {/if}
            {#if stateWords.includes('expired') && memory.notAfter}
              <div>expired until {fullTimestamp(timestampDate(memory.notAfter))}</div>
            {/if}
            {#if stateWords.includes('scheduled') && memory.notBefore}
              <div>opens {fullTimestamp(timestampDate(memory.notBefore))}</div>
              {#if closesInFuture && memory.notAfter}
                <div>closes {fullTimestamp(timestampDate(memory.notAfter))}</div>
              {/if}
            {/if}
          </div>
        </section>
      {/if}

      <Separator />
      <section class="d-sec">
        <h3>Content</h3>
        {#if memory.content}
          <div class="markdown-body">{@html bodyHtml}</div>
        {:else}
          <div class="d-empty-content">Content not loaded</div>
        {/if}
      </section>

      {#if hasTags}
        <Separator />
        <section class="d-sec">
          <h3>Tags</h3>
          <div class="d-tags">
            {#each memory.tags as t (t)}
              <button type="button" class="d-tag" onclick={() => ontag?.(t)}>{t}</button>
            {/each}
          </div>
        </section>
      {/if}

      <Separator />
      <section class="d-sec" aria-label="Metadata">
        <h3>Metadata</h3>
        <dl class="d-meta">
          <dt>id</dt>
          <dd>
            <span class="d-meta-val">{memory.id}</span>
            <button type="button" class="d-copy-btn" onclick={() => copyText(memory!.id)}>Copy id</button>
          </dd>
          <dt>short_id</dt>
          <dd>
            <span class="d-meta-val">{memory.shortId}</span>
            <button type="button" class="d-copy-btn" onclick={() => copyText(memory!.shortId)}>Copy short_id</button>
          </dd>
          <dt>scope</dt>
          <dd class="d-meta-val">{memory.scope}</dd>
          {#if memory.createdAt}
            {@const createdDate = timestampDate(memory.createdAt)}
            <dt>created_at</dt>
            <dd class="d-meta-val">{fullTimestamp(createdDate)} · {relativeTime(createdDate)}</dd>
          {/if}
          {#if hit}
            <dt>score</dt>
            <dd class="d-meta-val">{hit.score.toFixed(4)}</dd>
            {#if hit.relevance !== undefined}
              <dt>relevance</dt>
              <dd class="d-meta-val">{hit.relevance.toFixed(4)}</dd>
            {/if}
          {/if}
          <dt>owner</dt>
          <dd class="d-meta-val">{memory.owner}</dd>
          <dt>actor</dt>
          <dd class="d-meta-val">{memory.actor}</dd>
          <dt>visibility</dt>
          <dd class="d-meta-val">{memory.visibility}</dd>
          <dt>summary_source</dt>
          <dd class="d-meta-val">{memory.summarySource}</dd>
          <dt>source</dt>
          <dd class="d-meta-val">{memory.source}</dd>
          <dt>schema</dt>
          <dd class="d-meta-val">schema v{memory.schemaVersion ?? 0}</dd>
          <dt>access_count</dt>
          <dd class="d-meta-val">{memory.accessCount}</dd>
          <dt>last_accessed_at</dt>
          <dd class="d-meta-val">{memory.lastAccessedAt ? fullTimestamp(timestampDate(memory.lastAccessedAt)) : 'never'}</dd>
          {#if memory.summaryModel}
            <dt>summary_model</dt>
            <dd class="d-meta-val">{memory.summaryModel}</dd>
          {/if}
        </dl>
        {#if hasCitations}
          <div class="d-citations">
            <h4>Citations</h4>
            {#each memory.citations as c, i (i)}
              <div class="d-citation">
                <span class="d-cite-kind">{c.kind}</span>
                <span class="d-cite-ref">{c.ref}</span>
                {#if c.locator}<span class="d-cite-locator">{c.locator}</span>{/if}
                {#if c.pin}<span class="d-cite-pin">{c.pin}</span>{/if}
              </div>
            {/each}
          </div>
        {/if}
      </section>
    {/if}
  </div>
</aside>

<style>
  .detail-pane {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
    background: var(--background);
  }

  .d-head {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: calc(8 * var(--u));
    padding: calc(10 * var(--u)) calc(12 * var(--u));
    border-bottom: 1px solid var(--border);
    background: var(--background);
    flex: none;
  }
  .d-sid {
    font-family: var(--font-mono, monospace);
    font-size: calc(12 * var(--u));
    white-space: nowrap;
  }
  .d-vis {
    font-size: calc(11 * var(--u));
    color: var(--muted-foreground);
  }
  .d-grow {
    flex: 1;
  }

  .d-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: calc(12 * var(--u));
    display: flex;
    flex-direction: column;
    gap: calc(10 * var(--u));
    overflow-wrap: anywhere;
  }

  .d-loading {
    display: flex;
    flex-direction: column;
    gap: calc(8 * var(--u));
  }
  .d-error {
    color: var(--cat-gotcha);
    font-size: calc(13 * var(--u));
  }
  .d-note {
    font-size: calc(11.5 * var(--u));
    color: var(--warning);
    background: var(--surface-2);
    border: 1px solid var(--border-subtle);
    border-radius: calc(4 * var(--u));
    padding: calc(6 * var(--u)) calc(8 * var(--u));
  }

  .d-title {
    font-size: calc(15 * var(--u));
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .d-actions {
    display: flex;
    gap: calc(6 * var(--u));
    flex-wrap: wrap;
  }
  .d-tooltip-wrap {
    display: inline-flex;
  }
  .d-tooltip-wrap:focus-visible {
    outline: none;
  }

  .d-sec h3,
  .d-sec h4 {
    font-size: calc(10 * var(--u));
    text-transform: uppercase;
    letter-spacing: 0.03em;
    color: var(--muted-foreground);
    font-weight: 600;
    margin: 0 0 calc(6 * var(--u));
  }

  .d-statelist {
    display: flex;
    flex-direction: column;
    gap: calc(4 * var(--u));
    font-size: calc(12 * var(--u));
  }
  .d-link {
    color: var(--primary);
    text-decoration: underline;
    text-underline-offset: 2px;
    overflow-wrap: anywhere;
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    font: inherit;
  }

  .d-empty-content {
    color: var(--muted-foreground);
    font-size: calc(12 * var(--u));
  }

  .d-tags {
    display: flex;
    flex-wrap: wrap;
    gap: calc(5 * var(--u));
  }
  .d-tag {
    font-family: var(--font-mono, monospace);
    font-size: calc(10.5 * var(--u));
    background: var(--muted);
    border: none;
    border-radius: calc(3 * var(--u));
    padding: calc(2 * var(--u)) calc(6 * var(--u));
    cursor: pointer;
  }

  .d-meta {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    column-gap: calc(10 * var(--u));
    row-gap: calc(4 * var(--u));
    font-size: calc(11.5 * var(--u));
  }
  .d-meta dt {
    color: var(--muted-foreground);
  }
  .d-meta dd {
    margin: 0;
    display: flex;
    align-items: baseline;
    gap: calc(6 * var(--u));
    min-width: 0;
  }
  .d-meta-val {
    font-family: var(--font-mono, monospace);
    overflow-wrap: anywhere;
  }
  .d-copy-btn {
    background: none;
    border: 1px solid var(--border);
    border-radius: calc(3 * var(--u));
    font-size: calc(10 * var(--u));
    padding: 0 calc(5 * var(--u));
    cursor: pointer;
    color: var(--muted-foreground);
    flex: none;
  }

  .d-citations {
    margin-top: calc(8 * var(--u));
  }
  .d-citation {
    display: flex;
    gap: calc(8 * var(--u));
    font-size: calc(11 * var(--u));
    font-family: var(--font-mono, monospace);
    overflow-wrap: anywhere;
    padding: calc(2 * var(--u)) 0;
  }
  .d-cite-kind {
    color: var(--text-faint);
  }

  /* {@html} output can't take Tailwind utilities, so style the rendered
     markdown via :global on the wrapper. Mirrors MemoryDetail.svelte. */
  .markdown-body :global(h1),
  .markdown-body :global(h2),
  .markdown-body :global(h3),
  .markdown-body :global(h4) {
    font-weight: 650;
    margin: 0.9em 0 0.4em;
  }
  .markdown-body :global(h3) {
    font-size: calc(13 * var(--u));
  }
  .markdown-body :global(p) {
    margin: 0 0 0.7em;
    font-size: calc(13 * var(--u));
    line-height: 1.5;
  }
  .markdown-body :global(ul),
  .markdown-body :global(ol) {
    margin: 0 0 0.7em;
    padding-left: 1.3em;
  }
  .markdown-body :global(li) {
    margin: 0.2em 0;
  }
  .markdown-body :global(strong) {
    font-weight: 650;
    color: var(--foreground);
  }
  .markdown-body :global(code) {
    font-family: ui-monospace, Menlo, monospace;
    font-size: calc(11.5 * var(--u));
    background: var(--accent);
    border-radius: 4px;
    padding: 1px 5px;
  }
  .markdown-body :global(pre) {
    background: var(--code-bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 10px 11px;
    overflow: auto;
    margin: 0 0 0.7em;
  }
  .markdown-body :global(pre code) {
    background: none;
    padding: 0;
    font-size: calc(11.5 * var(--u));
    line-height: 1.5;
  }
  .markdown-body :global(a) {
    color: var(--primary);
    text-decoration: underline;
    text-underline-offset: 2px;
  }
  .markdown-body :global(blockquote) {
    border-left: 3px solid var(--border);
    margin: 0 0 0.7em;
    padding-left: 0.8em;
    color: var(--muted-foreground);
  }
</style>
