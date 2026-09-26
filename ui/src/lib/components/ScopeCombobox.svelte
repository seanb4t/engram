<script lang="ts">
  import * as Popover from '$lib/components/ui/popover';
  import * as Command from '$lib/components/ui/command';
  import type { ListScopesResponse } from '$lib/gen/engram_pb';

  // ROW-06: a scope combobox over ListScopes — every readable scope with its
  // readable-record count, plus an "Every readable scope (cross_spine)" entry.
  // shouldFilter={false} + manual substring filtering (not bits-ui's own
  // default filter): the whole ListScopes list is already loaded client-side
  // (unlike the header search's server-driven dropdown, where shouldFilter
  // MUST be false for a different reason — see classify/HeaderSearch), so
  // manual filtering over the complete list is equally "honest"; it also
  // sidesteps the bits-ui 2.18.1/Svelte 5 default-filter content-emptying
  // gotcha this milestone already hit once in plan 02-05.
  let {
    value,
    crossSpine,
    scopes,
    loading,
    error,
    onselect,
    onretry
  }: {
    value: string;
    crossSpine: boolean;
    scopes?: ListScopesResponse;
    loading: boolean;
    error: unknown;
    onselect: (scope: string) => void;
    onretry?: () => void;
  } = $props();

  let open = $state(false);
  let filterValue = $state('');

  const entries = $derived(scopes?.scopes ?? []);
  const filtered = $derived.by(() => {
    const f = filterValue.trim().toLowerCase();
    if (!f) return entries;
    return entries.filter((s) => s.scope.toLowerCase().includes(f));
  });

  const triggerLabel = $derived(value ? `scope: ${value}` : 'any scope');

  function select(scope: string) {
    onselect(scope);
    open = false;
    filterValue = '';
  }

  function formatCount(n: bigint): string {
    return `${scopes?.approximate ? '~' : ''}${n}`;
  }
</script>

<Popover.Root bind:open>
  <Popover.Trigger>
    {#snippet child({ props })}
      <button type="button" {...props} class="scope-combobox-trigger">
        {triggerLabel} ▾
      </button>
    {/snippet}
  </Popover.Trigger>
  <Popover.Content class="w-[calc(280*var(--u))] p-0" onOpenAutoFocus={(e) => e.preventDefault()}>
    <Command.Root shouldFilter={false} label="Filter scopes">
      <Command.Input placeholder="Filter scopes…" aria-label="Filter scopes" bind:value={filterValue} />
      <Command.List class="max-h-[calc(320*var(--u))] overflow-y-auto">
        {#if loading}
          <div class="scope-combobox-status" data-testid="scope-combobox-loading">loading scopes…</div>
        {:else if error}
          <div class="scope-combobox-status">
            <div class="scope-combobox-error">Could not load scopes</div>
            <button type="button" class="scope-combobox-retry" onclick={() => onretry?.()}>Retry</button>
          </div>
        {:else}
          <Command.Item value="__cross_spine__" aria-selected={crossSpine && !value} onSelect={() => select('')}>
            Every readable scope (cross_spine)
          </Command.Item>
          {#if filtered.length === 0}
            <div class="scope-combobox-status">No scope matches {filterValue}</div>
          {:else}
            {#each filtered as s (s.scope)}
              <Command.Item value={s.scope} aria-selected={s.scope === value} onSelect={() => select(s.scope)}>
                <span class="scope-combobox-name" title={s.scope}>{s.scope}</span>
                <span class="scope-combobox-count">{formatCount(s.count)}</span>
              </Command.Item>
            {/each}
          {/if}
        {/if}
      </Command.List>
    </Command.Root>
  </Popover.Content>
</Popover.Root>

<style>
  .scope-combobox-trigger {
    height: calc(20 * var(--u));
    border-radius: var(--radius-full, 9999px);
    border: 1px solid var(--border);
    background: var(--surface-2, var(--muted));
    font-size: calc(11 * var(--u));
    padding: 0 calc(8 * var(--u));
    cursor: pointer;
  }
  .scope-combobox-status {
    padding: calc(6 * var(--u)) calc(10 * var(--u));
    font-size: calc(12 * var(--u));
    color: var(--muted-foreground);
  }
  .scope-combobox-error {
    color: var(--cat-gotcha, var(--destructive));
  }
  .scope-combobox-retry {
    background: none;
    border: none;
    padding: 0;
    color: var(--primary);
    text-decoration: underline;
    cursor: pointer;
    font: inherit;
  }
  .scope-combobox-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: calc(200 * var(--u));
    font-family: var(--font-mono, monospace);
  }
  .scope-combobox-count {
    margin-left: auto;
    font-family: var(--font-mono, monospace);
    color: var(--muted-foreground);
  }
</style>
