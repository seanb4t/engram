<script lang="ts">
  // The "+ tag" picker (TAGS-02, D-16, D-18, D-19): mirrors ScopeCombobox's
  // shipped shape (Popover + bits-ui Command, its own filter pass disabled
  // per gotcha 3tz15e733n, manual substring ranking over the already-loaded
  // list). Shares the identical cached ListTags(scope, 1000) query TagBars
  // uses for the same scope key -- one fetch, two surfaces.
  import * as Popover from '$lib/components/ui/popover';
  import * as Command from '$lib/components/ui/command';
  import { createQuery } from '@tanstack/svelte-query';
  import { listTagsQuery } from '$lib/tags/query';
  import { toTagRows, rankTagMatches, matchFooter, tagsLoadingLine, tagsErrorCopy } from '$lib/tags/tags';
  import { parseConnectError } from '$lib/errors/connect-error';
  import TagMatchRow from './TagMatchRow.svelte';

  let { scope, onadd }: { scope: string; onadd: (tag: string) => void } = $props();

  // WCAG 4.1.2 (aria-required-attr): bits-ui's Command.Input sets
  // role="combobox" and aria-expanded, but its aria-controls only populates
  // from an (unused here) Command.Viewport -- an explicit id/aria-controls
  // pair closes the gap without adopting Viewport (mergeProps keeps our
  // aria-controls since bits-ui's own value is undefined without one).
  const uid = $props.id();
  const listId = `${uid}-list`;

  const q = createQuery(() => listTagsQuery(scope));

  let open = $state(false);
  let filterValue = $state('');

  const rows = $derived(toTagRows(q.data));
  const max = $derived(rows.length > 0 ? rows[0].count : 0);
  const more = $derived(q.data?.more ?? false);
  const ranked = $derived(rankTagMatches(rows, filterValue, { more }));
  const footer = $derived(matchFooter({ total: ranked.total, more, loaded: rows.length }));

  function select(tag: string) {
    onadd(tag);
    open = false;
    filterValue = '';
  }
</script>

<Popover.Root bind:open>
  <Popover.Trigger>
    {#snippet child({ props })}
      <button type="button" {...props} class="fbtn" aria-expanded={open}>+ tag</button>
    {/snippet}
  </Popover.Trigger>
  <Popover.Content class="w-[calc(280*var(--u))] p-0">
    <Command.Root shouldFilter={false} label="Filter tags">
      <div class="input-row">
        <span class="prefix mono">#</span>
        <Command.Input aria-label="Filter tags" aria-controls={listId} bind:value={filterValue} />
      </div>
      <Command.List id={listId} class="max-h-[calc(320*var(--u))] overflow-y-auto">
        {#if q.isLoading}
          <div class="status mono">{tagsLoadingLine(scope)}</div>
        {:else if q.isError}
          {@const parsed = parseConnectError(q.error)}
          {@const copy = tagsErrorCopy(parsed)}
          <div class="status error">
            <p>{copy.heading}</p>
            {#if copy.kind === 'rejected'}
              <pre class="envelope mono">{copy.envelope}</pre>
            {/if}
          </div>
        {:else}
          {#each ranked.matches as m (m.tag)}
            <Command.Item value={`tag:${m.tag}`} onSelect={() => select(m.tag)}>
              <TagMatchRow match={m} {max} />
            </Command.Item>
          {/each}
          {#if ranked.unknown}
            {@const unk = ranked.unknown}
            <Command.Item value={`add:${unk.tag}`} class="unknown" onSelect={() => select(unk.tag)}>
              <span class="add-label">Add #{unk.tag}</span>
              <span class="why mono">{unk.reason}</span>
            </Command.Item>
          {/if}
          <div class="footer mono">
            <span>{footer.text}</span>
            {#if footer.warn}<span class="warn">{footer.warn}</span>{/if}
          </div>
        {/if}
      </Command.List>
    </Command.Root>
  </Popover.Content>
</Popover.Root>

<style>
  .fbtn {
    height: calc(20 * var(--u));
    border-radius: 9999px;
    border: 1px dashed var(--border);
    background: transparent;
    font-size: calc(11 * var(--u));
    padding: 0 calc(8 * var(--u));
    cursor: pointer;
  }
  .fbtn[aria-expanded='true'] {
    border-style: solid;
    border-color: var(--primary);
    background: var(--primary-soft);
  }
  .input-row {
    display: flex;
    align-items: center;
    gap: calc(4 * var(--u));
    padding: 0 calc(8 * var(--u));
  }
  .prefix {
    color: var(--text-faint);
  }
  .status {
    padding: calc(6 * var(--u)) calc(10 * var(--u));
    font-size: calc(12 * var(--u));
    color: var(--muted-foreground);
  }
  .status.error {
    color: var(--destructive);
  }
  .envelope {
    font-size: calc(11 * var(--u));
    white-space: pre-wrap;
  }
  :global(.unknown) .add-label {
    display: block;
  }
  .why {
    color: var(--warning);
    font-size: calc(11 * var(--u));
  }
  .footer {
    padding: calc(6 * var(--u)) calc(10 * var(--u));
    font-size: calc(11 * var(--u));
    color: var(--text-faint);
    display: flex;
    flex-direction: column;
    gap: calc(2 * var(--u));
  }
  .footer .warn {
    color: var(--warning);
  }
</style>
