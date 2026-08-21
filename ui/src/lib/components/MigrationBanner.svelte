<script lang="ts">
  // Fetches once per root QueryClient lifetime (staleTime: Infinity, no
  // mount/focus refetch, no retry) -- the migration backlog changes on
  // operator-run `engram migrate` timescales, not request timescales. Never
  // re-derives pending from the bucket list client-side: 07-06 computes it
  // server-side via the single MigrateStatusResult.Pending() helper.
  //
  // Deliberately NO error-logging $effect here. The silent query-meta opt-out
  // below is the whole mechanism -- a rejected fetch routes through the exported
  // handleQueryError (wired as +layout.svelte's QueryCache.onError), which
  // is the single logging owner. A component-level log here would double it.
  import { createQuery } from '@tanstack/svelte-query';
  import { engram } from '$lib/client';
  import ArrowUpCircleIcon from '@lucide/svelte/icons/arrow-up-circle';
  import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';

  const query = createQuery(() => ({
    queryKey: ['migrateStatus'],
    queryFn: () => engram.migrateStatus({}),
    meta: { silent: true },
    staleTime: Infinity,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    retry: false
  }));

  const pending = $derived(query.data?.pending ?? 0n);
  const futureTotal = $derived(query.data?.futureTotal ?? 0n);
</script>

{#if pending > 0n}
  <div class="px-3 py-2 text-[13px] flex items-center gap-2 border-b border-border bg-muted text-foreground">
    <ArrowUpCircleIcon aria-hidden="true" />
    <span>{pending} record{pending === 1n ? '' : 's'} pending migration — run <span class="font-mono">engram migrate</span></span>
  </div>
{/if}
{#if futureTotal > 0n}
  <div class="px-3 py-2 text-[13px] flex items-center gap-2 border-b border-destructive/30 bg-destructive/10 text-destructive">
    <TriangleAlertIcon aria-hidden="true" />
    <span>{futureTotal} record{futureTotal === 1n ? '' : 's'} at a newer schema version than this server — some data may not render correctly</span>
  </div>
{/if}
