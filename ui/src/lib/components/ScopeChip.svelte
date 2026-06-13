<script lang="ts">
  import { parseScope } from '$lib/scope';
  import { Badge } from '$lib/components/ui/badge';
  import * as HoverCard from '$lib/components/ui/hover-card';

  let { scope, mode = 'inline', count }: { scope: string; mode?: 'inline' | 'stacked'; count?: number } = $props();
  const p = $derived(parseScope(scope));
  const catClass = {
    repo: 'text-cat-convention',
    discovery: 'text-cat-decision',
    project: 'text-cat-preference',
    '': 'text-muted-foreground',
  } as const;
</script>

<HoverCard.Root>
  <HoverCard.Trigger>
    <span class="inline-flex items-center gap-2 min-w-0" title={p.full}>
      <Badge variant="outline" class="shrink-0 text-[10px] uppercase {catClass[p.type]}">{p.type || 'scope'}</Badge>
      {#if mode === 'stacked'}
        <span class="flex flex-col min-w-0">
          <span class="truncate font-mono text-[13px]">{p.name}</span>
          {#if p.org}<span class="truncate font-mono text-[10px] text-muted-foreground opacity-70">{p.org}</span>{/if}
        </span>
      {:else}
        <span class="truncate font-mono text-[12px]">
          {#if p.org}<span class="text-muted-foreground opacity-60 text-[11px]">{p.org}/</span>{/if}{p.name}
        </span>
      {/if}
      {#if count !== undefined}<span class="ml-auto shrink-0 rounded-full border border-border bg-card px-2 py-0.5 text-[11px] tabular-nums">{count}</span>{/if}
    </span>
  </HoverCard.Trigger>
  <HoverCard.Content>
    <div class="flex flex-col gap-1 text-xs">
      <span class="font-mono break-all">{p.full}</span>
      <div class="flex items-center gap-2 text-muted-foreground">
        <span class="uppercase font-semibold">Type:</span>
        <span>{p.type || 'scope'}</span>
      </div>
      {#if p.org}
        <div class="flex items-center gap-2 text-muted-foreground">
          <span class="uppercase font-semibold">Org:</span>
          <span class="font-mono">{p.org}</span>
        </div>
      {/if}
      {#if count !== undefined}
        <div class="flex items-center gap-2 text-muted-foreground">
          <span class="uppercase font-semibold">Count:</span>
          <span class="tabular-nums">{count}</span>
        </div>
      {/if}
    </div>
  </HoverCard.Content>
</HoverCard.Root>
