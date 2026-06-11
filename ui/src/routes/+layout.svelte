<script lang="ts">
  import '../app.css';
  import { QueryClient, QueryClientProvider, QueryCache } from '@tanstack/svelte-query';
  import { ModeWatcher } from 'mode-watcher';
  import { mapAuthError } from '$lib/client';
  import AppShell from '$lib/components/AppShell.svelte';

  let { children } = $props();

  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 30_000 } },
    queryCache: new QueryCache({
      onError: (err) => {
        const target = mapAuthError(err);
        if (target) window.location.assign(target);
      }
    })
  });
</script>

<ModeWatcher />
<QueryClientProvider client={queryClient}>
  <AppShell>{@render children()}</AppShell>
</QueryClientProvider>
