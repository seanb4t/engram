<script lang="ts">
  import { setMode, mode } from 'mode-watcher';
  import { goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { Button } from '$lib/components/ui/button';
  let { children } = $props();
  // mode-watcher 0.5: `mode` is a derived store, read via `$mode` (NOT mode.current).
  function cycleTheme() {
    const next = $mode === 'dark' ? 'light' : 'dark';
    setMode(next);
  }
</script>

<div class="min-h-screen flex flex-col" style="background:var(--background);color:var(--foreground)">
  <header class="flex items-center gap-3 px-3 py-2 eg-surface" style="border-bottom:1px solid var(--border)">
    <span style="color:var(--accent);font-weight:700">◆ engram</span>
    <Button
      variant="outline"
      aria-label="search"
      class="flex-1 justify-start text-[var(--muted)]"
      onclick={() => goto(`${base}/search`)}
    >⌘K  search memories…</Button>
    <Button variant="outline" size="sm" aria-label="toggle theme" onclick={cycleTheme}>◐</Button>
  </header>
  <main class="flex-1">{@render children?.()}</main>
</div>
