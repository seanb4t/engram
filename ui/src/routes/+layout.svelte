<script lang="ts">
  import '../app.css';
  import { onMount } from 'svelte';
  import { QueryClient, QueryClientProvider, QueryCache } from '@tanstack/svelte-query';
  import { ModeWatcher } from 'mode-watcher';
  import { beforeNavigate } from '$app/navigation';
  import { errorBanner, handleQueryError, clearError } from '$lib/errors';
  import { Toaster } from '$lib/components/ui/sonner';
  import { Button } from '$lib/components/ui/button';
  import { display, readTextSize, installDisplayShortcuts } from '$lib/display.svelte';
  import AppShell from '$lib/components/AppShell.svelte';
  import CommandMenu from '$lib/components/CommandMenu.svelte';
  let { children } = $props();

  // D-13: apply the persisted text-size preference (the anti-flash script in
  // app.html already set --ui-font before first paint; this syncs the store
  // so the Display popover reflects it) and install the console-wide
  // ⌘+/⌘-/⌘0 shortcuts.
  onMount(() => {
    try {
      display.size = readTextSize(localStorage);
    } catch {
      // best-effort, mirrors readTextSize's own fallback to the default.
    }
    return installDisplayShortcuts(window);
  });

  // PRESERVE: the root queryClient, delegating all query errors to the
  // shared handleQueryError (auth redirect, then silent-query opt-out, then
  // error-report) so production and every test lane share one routing
  // implementation.
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 30_000 } },
    queryCache: new QueryCache({
      onError: handleQueryError
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
  <CommandMenu bind:open={cmdOpen} />
</QueryClientProvider>
