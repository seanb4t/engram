<script lang="ts">
  import lockup from '$lib/assets/engram-lockup.svg?raw';
  let { class: klass = '' }: { class?: string } = $props();
</script>

<span class={'inline-flex items-center text-primary ' + klass} role="img" aria-label="engram">
  {@html lockup}
</span>

<style>
  span :global(svg) { height: 1.5rem; width: auto; display: block; }
  :global(.dark) span :global(svg) { filter: drop-shadow(0 0 6px var(--primary)); }
</style>
