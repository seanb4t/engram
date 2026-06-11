<script lang="ts">
  let { value, onsubmit }: { value: string; onsubmit: (q: string) => void } = $props();
  let q = $state(value);
  function onkey(e: KeyboardEvent) { if (e.key === 'Enter') onsubmit(q); }
</script>
<input role="searchbox" placeholder="⌘K search memories…" bind:value={q} onkeydown={onkey}
  class="w-full px-3 py-2" style="background:var(--surface);border:1px solid var(--border);color:var(--foreground)" />
