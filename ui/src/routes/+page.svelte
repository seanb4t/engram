<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { engram } from '$lib/client';
  import { PAGE_LIMIT } from '$lib/queries';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import { Button } from '$lib/components/ui/button';
  const scopesQ = createQuery({ queryKey: ['listScopes'], queryFn: () => engram.listScopes({}) });
  // Recent memories below the scope tiles (first page, server default desc order).
  const recentQ = createQuery({
    queryKey: ['listMemories', '', [], '', PAGE_LIMIT, 0],
    queryFn: () => engram.listMemories({ scope: '', limit: BigInt(PAGE_LIMIT), offset: 0n, categories: [], visibility: '' })
  });
  function openRecord(id: string) { goto(`${base}/observe?sel=${encodeURIComponent(id)}`); }
</script>

<div class="p-4">
  <h1 class="mb-3" style="color:var(--accent)">engram — operator console</h1>
  {#if $scopesQ.isLoading}
    <div class="eg-muted">loading scopes…</div>
  {:else if $scopesQ.error}
    <div class="eg-error">failed to load scopes</div>
  {:else}
    <div class="grid gap-2" style="grid-template-columns:repeat(auto-fill,minmax(220px,1fr))">
      {#each $scopesQ.data?.scopes ?? [] as s (s.scope)}
        <Button variant="surface" class="text-left p-3 h-auto block" onclick={() => goto(`${base}/observe?scope=${encodeURIComponent(s.scope)}`)}>
          <div style="color:var(--foreground)">{s.scope}</div>
          <div style="color:var(--accent);font-size:20px">{s.count}</div>
        </Button>
      {/each}
    </div>
    {#if $scopesQ.data?.approximate}<div class="eg-muted">counts approximate (scanCap)</div>{/if}
  {/if}

  <div class="mt-4 eg-label">Recent memories</div>
  <MemoryList
    memories={$recentQ.data?.memories ?? []}
    total={$recentQ.data?.total ?? 0n}
    approximate={$recentQ.data?.approximate ?? false}
    loading={$recentQ.isLoading}
    error={$recentQ.error}
    selectedId=""
    onselect={openRecord}
  />
</div>
