<script lang="ts">
  import '../app.css';
  import { QueryClient, QueryClientProvider, QueryCache } from '@tanstack/svelte-query';
  import { ModeWatcher } from 'mode-watcher';
  import { beforeNavigate, goto } from '$app/navigation';
  import { base } from '$app/paths';
  import { mapAuthError } from '$lib/client';
  import { errorBanner, reportError, clearError } from '$lib/errors';
  import { Toaster } from '$lib/components/ui/sonner';
  import { Button } from '$lib/components/ui/button';
  import AppShell from '$lib/components/AppShell.svelte';
  import CommandPalette from '$lib/components/CommandPalette.svelte';
  let { children } = $props();

  // PRESERVE: inline queryClient with the auth-redirect / error-report onError.
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 30_000 } },
    queryCache: new QueryCache({
      onError: (err) => {
        const target = mapAuthError(err);
        if (target) { window.location.assign(target); return; }
        reportError(err);
      }
    })
  });
  beforeNavigate(() => clearError());

  let cmdOpen = $state(false);
  function onkey(e: KeyboardEvent) { if ((e.metaKey || e.ctrlKey) && e.key === 'k') { e.preventDefault(); cmdOpen = true; } }
</script>

<svelte:window onkeydown={onkey} />
<ModeWatcher />
<Toaster />
<QueryClientProvider client={queryClient}>
  {#if $errorBanner}
    <div role="alert" class="flex items-center justify-between gap-3 px-3 py-2 bg-card text-cat-gotcha border-b border-cat-gotcha">
      <span>error: {$errorBanner}</span>
      <Button variant="ghost" size="sm" aria-label="dismiss error" onclick={clearError}>✕</Button>
    </div>
  {/if}
  <AppShell oncommand={() => (cmdOpen = true)}>{@render children()}</AppShell>
  <CommandPalette bind:open={cmdOpen} onsearch={(q) => goto(`${base}/search?q=${encodeURIComponent(q)}`)} onnavigate={(href) => goto(href)} />
</QueryClientProvider>
