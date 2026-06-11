<script lang="ts">
  import { createQuery } from '@tanstack/svelte-query';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { engram } from '$lib/client';
  const scopesQ = createQuery({ queryKey: ['listScopes'], queryFn: () => engram.listScopes({}) });
</script>

<div class="p-4">
  <h1 class="mb-3" style="color:var(--accent)">engram — operator console</h1>
  {#if $scopesQ.isLoading}
    <div style="color:var(--muted)">loading scopes…</div>
  {:else if $scopesQ.error}
    <div style="color:var(--cat-gotcha)">failed to load scopes</div>
  {:else}
    <div class="grid gap-2" style="grid-template-columns:repeat(auto-fill,minmax(220px,1fr))">
      {#each $scopesQ.data?.scopes ?? [] as s (s.scope)}
        <button class="text-left p-3 rounded" style="background:var(--surface);border:1px solid var(--border)" onclick={() => goto(`${base}/observe?scope=${encodeURIComponent(s.scope)}`)}>
          <div style="color:var(--foreground)">{s.scope}</div>
          <div style="color:var(--accent);font-size:20px">{s.count}</div>
        </button>
      {/each}
    </div>
    {#if $scopesQ.data?.approximate}<div style="color:var(--muted)">counts approximate (scanCap)</div>{/if}
  {/if}
</div>
