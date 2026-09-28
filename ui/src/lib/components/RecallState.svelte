<script lang="ts" module>
  import type { ParsedConnectError } from '$lib/errors/connect-error';

  // The empty-state fix rows come from +page.svelte (search-every-scope,
  // include-archived/superseded/scheduled, clear-category) — a DIFFERENT
  // vocabulary than connect-error.ts's FixRow id union (which is scoped to
  // rejection envelopes), so this local shape is intentionally generic
  // rather than reusing that narrower type.
  export type RecallFixRow = { id: string; label: string };
  export type RecallEmptyState = { kind: 'empty'; heading: string; fixes: RecallFixRow[] };
  export type RecallErrorState = { kind: 'error'; parsed: ParsedConnectError; fixes: RecallFixRow[] };
  export type RecallStateInput = RecallEmptyState | RecallErrorState;
</script>

<script lang="ts">
  import { toast } from 'svelte-sonner';
  import { Button } from '$lib/components/ui/button';

  // ENTRY-03/ENTRY-05/D-04: the one component every honest empty/failure
  // state on /search renders through — never a bare empty list, and never a
  // fabricated tracing token the server itself does not emit.
  let {
    state,
    onfix,
    onretry
  }: {
    state: RecallStateInput;
    onfix: (id: string) => void;
    onretry: () => void;
  } = $props();

  async function copyError(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      toast.success('copied error text');
    } catch {
      toast.error('copy failed');
    }
  }
</script>

{#if state.kind === 'empty'}
  <div class="recall-state" data-testid="recall-empty">
    <p class="rs-heading">{state.heading}</p>
    {#if state.fixes.length > 0}
      <div class="rs-fixes">
        {#each state.fixes as fix (fix.id)}
          <Button variant="outline" size="sm" onclick={() => onfix(fix.id)}>{fix.label}</Button>
        {/each}
      </div>
    {/if}
  </div>
{:else if state.parsed.kind === 'rejected'}
  {@const parsed = state.parsed}
  <div class="recall-state" data-testid="recall-error-rejected">
    <p class="rs-heading">Server rejected the request</p>
    <pre class="rs-envelope">field={parsed.fields.join(',')} hint={parsed.hint}: {parsed.detail}</pre>
    {#if state.fixes.length > 0}
      <div class="rs-fixes">
        {#each state.fixes as fix (fix.id)}
          <Button variant="outline" size="sm" onclick={() => onfix(fix.id)}>{fix.label}</Button>
        {/each}
      </div>
    {/if}
  </div>
{:else if state.parsed.kind === 'ambiguous-short-id'}
  {@const parsed = state.parsed}
  <div class="recall-state rs-warning" data-testid="recall-ambiguous">
    <p class="rs-heading">short_id {parsed.shortId} is ambiguous — paste the full id to be exact</p>
    <div class="rs-fixes">
      <Button variant="outline" size="sm" onclick={onretry}>Retry the request</Button>
    </div>
  </div>
{:else if state.parsed.kind === 'not-found'}
  <div class="recall-state" data-testid="recall-not-found">
    <p class="rs-heading">No memory with that id that you can read · not-found and not-yours look the same by design</p>
  </div>
{:else}
  {@const parsed = state.parsed}
  <div class="recall-state" data-testid="recall-opaque">
    <p class="rs-heading">Search failed — nothing was searched. search_memory returned code={parsed.codeName}</p>
    <p class="rs-subtext">Nothing was searched, so this is not an empty result</p>
    <pre class="rs-detail">{parsed.detail}</pre>
    <div class="rs-fixes">
      <Button variant="outline" size="sm" onclick={onretry}>Retry</Button>
      <Button variant="outline" size="sm" onclick={() => copyError(parsed.detail)}>Copy error</Button>
    </div>
  </div>
{/if}

<style>
  .recall-state {
    display: flex;
    flex-direction: column;
    gap: calc(10 * var(--u));
    padding: calc(24 * var(--u)) calc(16 * var(--u));
  }
  .rs-heading {
    font-size: calc(14 * var(--u));
  }
  .rs-warning .rs-heading {
    color: var(--warning);
  }
  .rs-subtext {
    color: var(--text-faint);
    font-size: calc(12 * var(--u));
  }
  .rs-envelope,
  .rs-detail {
    font-family: var(--font-mono, monospace);
    font-size: calc(12 * var(--u));
    border: 1px solid var(--destructive);
    color: var(--destructive);
    background: var(--surface-2);
    border-radius: calc(4 * var(--u));
    padding: calc(8 * var(--u));
    white-space: pre-wrap;
    user-select: text;
  }
  .rs-fixes {
    display: flex;
    flex-wrap: wrap;
    gap: calc(8 * var(--u));
  }
</style>
