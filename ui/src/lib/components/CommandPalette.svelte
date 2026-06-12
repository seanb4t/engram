<script lang="ts">
  import * as Command from '$lib/components/ui/command';
  import { base } from '$app/paths';

  let { open = $bindable(false), onsearch, onnavigate }: { open?: boolean; onsearch: (q: string) => void; onnavigate: (href: string) => void } = $props();
  let q = $state('');
</script>

<Command.Dialog bind:open>
  <Command.Input placeholder="search memories…" bind:value={q} />
  <Command.List>
    <Command.Empty>no matches</Command.Empty>
    <Command.Group heading="Search">
      <Command.Item onSelect={() => { onsearch(q); open = false; }}>Search memories for "{q}"</Command.Item>
    </Command.Group>
    <Command.Group heading="Go to">
      <Command.Item onSelect={() => { onnavigate(`${base}/observe`); open = false; }}>Observe</Command.Item>
      <Command.Item onSelect={() => { onnavigate(`${base}/discovery`); open = false; }}>Discovery</Command.Item>
    </Command.Group>
  </Command.List>
</Command.Dialog>
