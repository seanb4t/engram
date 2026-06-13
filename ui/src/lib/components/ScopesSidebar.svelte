<script lang="ts">
  import type { ScopeCount } from '$lib/gen/engram_pb';
  import type { Category, Visibility } from '$lib/queries';
  import { Button } from '$lib/components/ui/button';
  import { Checkbox } from '$lib/components/ui/checkbox';
  import { Select } from '$lib/components/ui/select';
  import { Skeleton } from '$lib/components/ui/skeleton';
  import ScopeChip from './ScopeChip.svelte';
  let { scopes, activeScope, categories, visibility, loading = false, error = null, onscope, onfilter }: {
    scopes: ScopeCount[]; activeScope: string; categories: Category[]; visibility: Visibility;
    loading?: boolean; error?: unknown; onscope: (s: string) => void; onfilter: (c: Category[], v: Visibility) => void;
  } = $props();
  const allCats: Category[] = ['convention', 'gotcha', 'decision', 'preference'];
  function toggleCat(c: Category) {
    onfilter(categories.includes(c) ? categories.filter((x) => x !== c) : [...categories, c], visibility);
  }
  const visOptions = [{ value: '', label: 'all' }, { value: 'private', label: 'private' }, { value: 'shared', label: 'shared' }];
</script>

<div class="w-[240px] shrink-0 border-r border-border p-3 flex flex-col gap-1 overflow-y-auto">
  <div class="text-[10px] uppercase text-muted-foreground">Scopes</div>
  {#if error}
    <div data-testid="scopes-error" class="text-cat-gotcha py-1 text-sm">failed to load scopes</div>
  {:else if loading}
    <div data-testid="scopes-loading" class="flex flex-col gap-1"><Skeleton class="h-7 w-full" /><Skeleton class="h-7 w-full" /></div>
  {:else}
    {#each scopes as s (s.scope)}
      <Button variant="ghost" class={'h-auto justify-start w-full min-w-0 ' + (s.scope === activeScope ? 'bg-primary/10 text-primary' : '')} onclick={() => onscope(s.scope)}>
        <ScopeChip scope={s.scope} mode="stacked" count={Number(s.count)} />
      </Button>
    {/each}
  {/if}
  <div class="mt-3 text-[10px] uppercase text-muted-foreground">Filters</div>
  {#each allCats as c (c)}
    <label class="flex items-center gap-2 text-sm" style="color:var(--cat-{c})">
      <Checkbox checked={categories.includes(c)} onCheckedChange={() => toggleCat(c)} aria-label={c} />{c}
    </label>
  {/each}
  <div class="mt-2 text-[10px] uppercase text-muted-foreground">visibility</div>
  <Select value={visibility} options={visOptions} ariaLabel="visibility" onValueChange={(v) => onfilter(categories, v as Visibility)} />
</div>
