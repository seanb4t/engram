<script lang="ts">
  // Placeholder pass-through shell. Task B3 replaces this with the real
  // top-bar + theme-toggle app shell; for now it just renders its children so
  // the layout (and the rest of this bead's tree) builds.
  let { children } = $props();
</script>

{@render children()}
