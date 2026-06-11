<script lang="ts">
  import type { ScopeCount } from '$lib/gen/engram_pb';
  import { Button } from '$lib/components/ui/button';
  import { Checkbox } from '$lib/components/ui/checkbox';
  import { Select } from '$lib/components/ui/select';
  let { scopes, activeScope, categories, visibility, loading = false, error = null, onscope, onfilter }: {
    scopes: ScopeCount[]; activeScope: string; categories: string[]; visibility: string;
    loading?: boolean; error?: unknown;
    onscope: (s: string) => void; onfilter: (cats: string[], vis: string) => void;
  } = $props();
  const allCats = ['convention', 'gotcha', 'decision', 'preference'];
  function toggleCat(c: string) {
    const next = categories.includes(c) ? categories.filter((x) => x !== c) : [...categories, c];
    onfilter(next, visibility);
  }
  const visOptions = [
    { value: '', label: 'all' },
    { value: 'private', label: 'private' },
    { value: 'shared', label: 'shared' }
  ];
</script>

<div class="p-3 eg-panel-r" style="width:210px">
  <div class="eg-label">Scopes</div>
  {#if error}
    <div data-testid="scopes-error" class="eg-error py-1">failed to load scopes — retry</div>
  {:else if loading}
    <div class="eg-muted py-1">loading scopes…</div>
  {:else if scopes.length === 0}
    <div class="eg-muted py-1">no scopes</div>
  {:else}
    {#each scopes as s (s.scope)}
      <Button
        variant="ghost"
        size="block"
        class={s.scope === activeScope ? 'eg-surface' : ''}
        style={s.scope === activeScope ? 'border-left:2px solid var(--accent)' : ''}
        onclick={() => onscope(s.scope)}
      >
        <span class="flex-1">{s.scope}</span>
        <span class="eg-muted">{s.count}</span>
      </Button>
    {/each}
  {/if}
  <div class="mt-3 eg-label">Filters</div>
  {#each allCats as c (c)}
    <label class="flex items-center gap-2" style="color:var(--cat-{c})">
      <Checkbox checked={categories.includes(c)} onCheckedChange={() => toggleCat(c)} aria-label={c} />
      {c}
    </label>
  {/each}
  <div class="block mt-2">
    <div class="eg-muted">visibility</div>
    <Select value={visibility} options={visOptions} ariaLabel="visibility" onValueChange={(v) => onfilter(categories, v)} />
  </div>
</div>
