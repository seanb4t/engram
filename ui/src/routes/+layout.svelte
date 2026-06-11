<script lang="ts">
  import '../app.css';
  import { QueryClient, QueryClientProvider, QueryCache } from '@tanstack/svelte-query';
  import { ModeWatcher } from 'mode-watcher';
  import { beforeNavigate } from '$app/navigation';
  import { mapAuthError } from '$lib/client';
  import { errorBanner, reportError, clearError } from '$lib/errors';
  import AppShell from '$lib/components/AppShell.svelte';
  import { Button } from '$lib/components/ui/button';

  let { children } = $props();

  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 30_000 } },
    queryCache: new QueryCache({
      onError: (err) => {
        const target = mapAuthError(err);
        if (target) {
          // Auth errors redirect to login and are not surfaced as a banner.
          window.location.assign(target);
          return;
        }
        // Previously non-auth errors were silently dropped. Surface them.
        reportError(err);
      }
    })
  });

  // Clear any stale error banner when navigating between routes.
  beforeNavigate(() => clearError());
</script>

<ModeWatcher />
<QueryClientProvider client={queryClient}>
  {#if $errorBanner}
    <div role="alert" class="flex items-center justify-between gap-3 px-3 py-2 eg-error" style="background:var(--surface);border-bottom:1px solid var(--cat-gotcha)">
      <span>error: {$errorBanner}</span>
      <Button variant="ghost" size="sm" aria-label="dismiss error" onclick={clearError}>✕</Button>
    </div>
  {/if}
  <AppShell>{@render children()}</AppShell>
</QueryClientProvider>
