<script lang="ts">
  import { onMount } from 'svelte';
  import { createQuery } from '@tanstack/svelte-query';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { engram } from '$lib/client';
  import { PAGE_LIMIT } from '$lib/queries';
  import { peekResume, consumeResume, normalizeReturnPath, isAllowedDestination } from '$lib/resume';
  import MemoryList from '$lib/components/MemoryList.svelte';
  import ScopeChip from '$lib/components/ScopeChip.svelte';
  import { Button } from '$lib/components/ui/button';
  // svelte-query v6: options wrapped in a function; results are runes objects read directly (no $).
  const scopesQ = createQuery(() => ({ queryKey: ['listScopes'], queryFn: () => engram.listScopes({}) }));
  // Recent memories below the scope tiles (first page, server default desc order).
  const recentQ = createQuery(() => ({
    queryKey: ['listMemories', '', [], '', PAGE_LIMIT, 0],
    queryFn: () => engram.listMemories({ scope: '', limit: BigInt(PAGE_LIMIT), offset: 0n, categories: [], visibility: '' })
  }));
  function openRecord(id: string) { goto(`${base}/observe?sel=${encodeURIComponent(id)}`); }

  // D-09 re-auth landing (Codex round-3 HIGH): the OIDC callback always lands
  // here (/ui/, handlers.go:187), never the originating route. This root
  // page PEEKS the resume envelope and routes back to its returnPath WITHOUT
  // deleting it -- the destination route (observe/search/discovery) still
  // needs it to reopen the sheet and pass resumeValues in as props; it is
  // the sole owner of consumeResume() (after the form's onresumeapplied
  // acknowledgement). base + normalizeReturnPath guarantees the redirect
  // never double-prefixes to /ui/ui/observe (base='/ui', svelte.config.js:9).
  // A malformed/tampered returnPath that fails isAllowedDestination is
  // rejected and the envelope discarded rather than followed.
  onMount(() => {
    const env = peekResume();
    if (!env) return;
    if (!isAllowedDestination(env.returnPath)) {
      consumeResume();
      return;
    }
    goto(`${base}${normalizeReturnPath(env.returnPath)}`);
  });
</script>

<div class="p-4">
  <h1 class="mb-3 text-primary">engram — operator console</h1>
  {#if scopesQ.isLoading}
    <div class="text-muted-foreground">loading scopes…</div>
  {:else if scopesQ.error}
    <div class="text-cat-gotcha">failed to load scopes</div>
  {:else}
    <div class="grid gap-2" style="grid-template-columns:repeat(auto-fill,minmax(215px,1fr))">
      {#each scopesQ.data?.scopes ?? [] as s (s.scope)}
        <Button variant="surface" class="relative text-left p-3 h-auto block overflow-hidden" onclick={() => goto(`${base}/observe?scope=${encodeURIComponent(s.scope)}`)}>
          <span class="absolute left-0 top-0 bottom-0 w-[3px] bg-primary"></span>
          <ScopeChip scope={s.scope} mode="stacked" />
          <div class="text-primary text-[24px] tabular-nums mt-1">{s.count}</div>
        </Button>
      {/each}
    </div>
    {#if scopesQ.data?.approximate}<div class="text-muted-foreground">counts approximate (scanCap)</div>{/if}
  {/if}

  <div class="mt-4 text-[10px] uppercase text-muted-foreground">Recent memories</div>
  <MemoryList
    memories={recentQ.data?.memories ?? []}
    total={recentQ.data?.total ?? 0n}
    approximate={recentQ.data?.approximate ?? false}
    loading={recentQ.isLoading}
    error={recentQ.error}
    selectedId=""
    onselect={openRecord}
  />
</div>
