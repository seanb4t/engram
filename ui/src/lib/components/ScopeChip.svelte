<script lang="ts">
  import { parseScope } from '$lib/scope';
  import { Badge } from '$lib/components/ui/badge';
  import * as HoverCard from '$lib/components/ui/hover-card';

  // DSYS-03/D-17: `dim` forces every category-hued/opacity-reduced text
  // node here to --muted-foreground (a solid colour proven >4.5:1 against
  // every background this component renders on -- surfaces.browser.test.ts).
  // Callers pass it for an archived/superseded/active/opened row; the org
  // segment's opacity was ALSO unconditionally unsafe (opacity-60/70 on an
  // already-faint colour) and is dropped regardless of `dim`.
  let { scope, mode = 'inline', count, dim = false }: { scope: string; mode?: 'inline' | 'stacked'; count?: number; dim?: boolean } = $props();
  const p = $derived(parseScope(scope));
  const catClass = {
    repo: 'text-cat-convention',
    discovery: 'text-cat-decision',
    project: 'text-cat-preference',
    '': 'text-muted-foreground',
  } as const;
  const dimStyle = $derived(dim ? 'color: var(--muted-foreground);' : undefined);
</script>

<HoverCard.Root>
  <HoverCard.Trigger>
    <span class="inline-flex items-center gap-2 min-w-0" title={p.full}>
      <Badge variant="outline" class="shrink-0 text-[calc(10*var(--u))] uppercase {catClass[p.type]}" style={dimStyle}>{p.type || 'scope'}</Badge>
      {#if mode === 'stacked'}
        <span class="flex flex-col min-w-0">
          <span class="truncate font-mono text-[calc(13*var(--u))]" style={dimStyle}>{p.name}</span>
          {#if p.org}<span class="truncate font-mono text-[calc(10*var(--u))] text-muted-foreground">{p.org}</span>{/if}
        </span>
      {:else}
        <span class="truncate font-mono text-[calc(12*var(--u))]" style={dimStyle}>
          {#if p.org}<span class="text-muted-foreground text-[calc(11*var(--u))]">{p.org}/</span>{/if}{p.name}
        </span>
      {/if}
      {#if count !== undefined}<span class="ml-auto shrink-0 rounded-full border border-border bg-card px-2 py-0.5 text-[calc(11*var(--u))] tabular-nums">{count}</span>{/if}
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
