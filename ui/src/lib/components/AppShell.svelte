<script lang="ts">
  import { setMode, mode } from 'mode-watcher';
  import { base } from '$app/paths';
  import { page } from '$app/state';
  import BrandMark from './BrandMark.svelte';
  import { Button } from '$lib/components/ui/button';
  import { Kbd } from '$lib/components/ui/kbd';
  import EyeIcon from '@lucide/svelte/icons/eye';
  import SearchIcon from '@lucide/svelte/icons/search';
  import CompassIcon from '@lucide/svelte/icons/compass';
  import SunMoonIcon from '@lucide/svelte/icons/sun-moon';
  let { children, oncommand }: { children?: import('svelte').Snippet; oncommand?: () => void } = $props();
  const nav = [
    { href: `${base}/observe`, label: 'Observe', icon: EyeIcon },
    { href: `${base}/search`, label: 'Search', icon: SearchIcon },
    { href: `${base}/discovery`, label: 'Discovery', icon: CompassIcon }
  ];
  function cycleTheme() { setMode(mode.current === 'dark' ? 'light' : 'dark'); }
</script>

<div class="h-dvh flex flex-col overflow-hidden bg-background text-foreground">
  <header class="flex items-center gap-3 px-3 py-2 border-b border-border">
    <BrandMark />
    <Button variant="outline" aria-label="search" class="flex-1 justify-start text-muted-foreground" onclick={() => oncommand?.()}>
      <SearchIcon data-icon="inline-start" /> search memories… <Kbd class="ml-auto">⌘K</Kbd>
    </Button>
    <Button variant="outline" size="sm" aria-label="toggle theme" onclick={cycleTheme}><SunMoonIcon data-icon="inline-start" /></Button>
  </header>
  <div class="flex flex-1 min-h-0">
    <nav class="flex flex-col gap-1 p-2 border-r border-border w-[64px] items-center">
      {#each nav as n (n.href)}
        {@const active = page.url.pathname.startsWith(n.href)}
        <a href={n.href} aria-label={n.label}
           class={'relative flex flex-col items-center gap-1 p-2 rounded text-[10px] ' +
             (active ? 'text-primary bg-primary/10' : 'text-muted-foreground hover:bg-accent hover:text-foreground')}>
          {#if active}<span class="absolute left-0 top-1/4 bottom-1/4 w-[3px] rounded bg-primary"></span>{/if}
          <n.icon data-icon="inline-start" />{n.label}
        </a>
      {/each}
    </nav>
    <main class="flex-1 min-w-0 min-h-0">{@render children?.()}</main>
  </div>
</div>
