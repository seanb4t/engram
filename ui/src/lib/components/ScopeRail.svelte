<script lang="ts">
  import type { ScopeCount } from '$lib/gen/engram_pb';
  let { scopes, activeScope, categories, visibility, onscope, onfilter }: {
    scopes: ScopeCount[]; activeScope: string; categories: string[]; visibility: string;
    onscope: (s: string) => void; onfilter: (cats: string[], vis: string) => void;
  } = $props();
  const allCats = ['convention', 'gotcha', 'decision', 'preference'];
  function toggleCat(c: string) {
    const next = categories.includes(c) ? categories.filter((x) => x !== c) : [...categories, c];
    onfilter(next, visibility);
  }
</script>

<div class="p-3" style="border-right:1px solid var(--border);width:210px">
  <div style="color:var(--muted);text-transform:uppercase;font-size:10px">Scopes</div>
  {#each scopes as s (s.scope)}
    <button class="block w-full text-left px-2 py-1" style="{s.scope === activeScope ? 'background:var(--surface);border-left:2px solid var(--accent)' : ''}" onclick={() => onscope(s.scope)}>
      {s.scope} <span style="float:right;color:var(--muted)">{s.count}</span>
    </button>
  {/each}
  <div class="mt-3" style="color:var(--muted);text-transform:uppercase;font-size:10px">Filters</div>
  {#each allCats as c (c)}
    <label class="block" style="color:var(--cat-{c})"><input type="checkbox" checked={categories.includes(c)} onchange={() => toggleCat(c)} /> {c}</label>
  {/each}
  <label class="block mt-2">visibility
    <select value={visibility} onchange={(e) => onfilter(categories, e.currentTarget.value)}>
      <option value="">all</option><option value="private">private</option><option value="shared">shared</option>
    </select>
  </label>
</div>
