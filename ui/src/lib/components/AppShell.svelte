<script lang="ts">
  import { setMode, mode } from 'mode-watcher';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  let { children } = $props();
  // mode-watcher 0.5: `mode` is a Svelte readable store; read reactively via `$mode`.
  function cycleTheme() {
    const next = $mode === 'dark' ? 'light' : 'dark';
    setMode(next);
  }
</script>

<div class="min-h-screen flex flex-col" style="background:var(--background);color:var(--foreground)">
  <header class="flex items-center gap-3 px-3 py-2" style="background:var(--surface);border-bottom:1px solid var(--border)">
    <span style="color:var(--accent);font-weight:700">◆ engram</span>
    <button aria-label="search" class="flex-1 text-left px-2 py-1 rounded" style="background:var(--background);border:1px solid var(--border);color:var(--muted)" onclick={() => goto(`${base}/search`)}>⌘K  search memories…</button>
    <button aria-label="toggle theme" onclick={cycleTheme} style="border:1px solid var(--border);border-radius:6px;padding:2px 6px">◐</button>
  </header>
  <main class="flex-1">{@render children?.()}</main>
</div>
