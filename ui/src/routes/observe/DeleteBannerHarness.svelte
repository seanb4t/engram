<script lang="ts">
  // Test-only harness (WR-04): reproduces the layout's global error banner so a
  // route-level test can assert that deleting the currently-selected record does
  // NOT surface a spurious top-level `role="alert"` banner. The layout wires a
  // QueryCache onError -> reportError; on a NotFound tombstone refetch that path
  // would set errorBanner and render the alert below. The client is injected so
  // the test controls the exact QueryCache onError wiring.
  import { QueryClientProvider, type QueryClient } from '@tanstack/svelte-query';
  import { errorBanner } from '$lib/errors';
  import ObservePage from './+page.svelte';

  let { client }: { client: QueryClient } = $props();
</script>

<QueryClientProvider {client}>
  {#if $errorBanner}
    <div role="alert">error: {$errorBanner}</div>
  {/if}
  <ObservePage />
</QueryClientProvider>
