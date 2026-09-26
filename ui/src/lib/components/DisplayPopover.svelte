<script lang="ts">
  // D-13's Aa Display popover: a fixed-px segmented control (deliberately
  // does not scale, per foundations.md) that drives the one console-wide
  // text-size preference. Only the live preview row below the control
  // scales with the preference it demonstrates.
  import { toast } from 'svelte-sonner';
  import { RadioGroup } from 'bits-ui';
  import * as Popover from '$lib/components/ui/popover';
  import { Button } from '$lib/components/ui/button';
  import { Kbd } from '$lib/components/ui/kbd';
  import { display, MIN_TEXT_SIZE, MAX_TEXT_SIZE, DEFAULT_TEXT_SIZE, setTextSize } from '$lib/display.svelte';

  const sizes = $derived(
    Array.from({ length: MAX_TEXT_SIZE - MIN_TEXT_SIZE + 1 }, (_, i) => MIN_TEXT_SIZE + i)
  );

  function selectSize(v: string): void {
    const result = setTextSize(Number(v));
    const suffix = result.atMax ? ' (max)' : result.atMin ? ' (min)' : '';
    toast(`Text size ${result.size}px${suffix}`);
  }
</script>

<Popover.Root>
  <Popover.Trigger>
    {#snippet child({ props })}
      <Button {...props} variant="outline" size="sm" aria-label="Display settings" aria-haspopup="dialog">
        Aa
      </Button>
    {/snippet}
  </Popover.Trigger>
  <Popover.Content role="dialog" aria-label="Display" class="w-[280px] flex flex-col gap-2">
    <h3 class="text-sm font-semibold">Display</h3>

    <div class="flex items-baseline justify-between text-[11px] uppercase tracking-wide text-muted-foreground">
      <span id="display-popover-textsize-label">Text size</span>
      <span class="normal-case tracking-normal font-mono text-foreground">{display.size}px</span>
    </div>

    <RadioGroup.Root
      orientation="horizontal"
      value={String(display.size)}
      onValueChange={selectSize}
      aria-labelledby="display-popover-textsize-label"
      class="grid grid-cols-5 border border-border rounded-md overflow-hidden"
    >
      {#each sizes as n (n)}
        <RadioGroup.Item
          value={String(n)}
          class="flex items-baseline justify-center gap-px border-r border-border last:border-r-0 font-mono text-muted-foreground data-[state=checked]:bg-primary/10 data-[state=checked]:text-foreground hover:bg-accent focus-visible:outline-2 focus-visible:outline-primary focus-visible:-outline-offset-2"
          style="height: 28px; font-size: 12px;"
        >
          {n}<small style="font-size: 9px;">px</small>
        </RadioGroup.Item>
      {/each}
    </RadioGroup.Root>

    <div class="flex flex-wrap gap-2 text-[11px] text-muted-foreground">
      <span><Kbd>⌘</Kbd><Kbd>+</Kbd> larger</span>
      <span><Kbd>⌘</Kbd><Kbd>−</Kbd> smaller</span>
      <span><Kbd>⌘</Kbd><Kbd>0</Kbd> reset to {DEFAULT_TEXT_SIZE}</span>
    </div>

    <div
      class="flex items-center rounded-md border border-border bg-background"
      style="height: calc(28 * var(--u)); gap: calc(8 * var(--u)); padding: 0 calc(10 * var(--u));"
    >
      <span class="cat-dot" style="background: var(--cat-convention)"></span>
      <span class="flex-1 min-w-0 truncate" style="font-size: var(--ui-font);">A sample result row</span>
      <span class="font-mono text-muted-foreground" style="font-size: calc(11 * var(--u));">2d</span>
      <span class="font-mono text-muted-foreground" style="font-size: calc(11 * var(--u));">0.82</span>
    </div>

    <div class="text-[11px] text-muted-foreground">Applies to every console page, not just this list.</div>
  </Popover.Content>
</Popover.Root>
