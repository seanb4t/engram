<script lang="ts">
  import { Select as SelectPrimitive } from 'bits-ui';
  import { cn } from '$lib/utils';

  // Thin, self-contained shadcn-svelte Select wrapper over the bits-ui Select
  // primitive: takes a flat options list and a bindable single value. Styled
  // with engram's CSS-variable tokens (app.css).
  type Option = { value: string; label: string };
  let {
    value = $bindable(''),
    options,
    onValueChange,
    ariaLabel,
    class: className
  }: {
    value?: string;
    options: Option[];
    onValueChange?: (v: string) => void;
    ariaLabel?: string;
    class?: string;
  } = $props();

  const selectedLabel = $derived(options.find((o) => o.value === value)?.label ?? options[0]?.label ?? '');
  const items = $derived(options.map((o) => ({ value: o.value, label: o.label })));
</script>

<SelectPrimitive.Root type="single" bind:value {onValueChange} {items}>
  <SelectPrimitive.Trigger
    aria-label={ariaLabel}
    class={cn(
      'inline-flex h-7 items-center gap-1 rounded border border-[var(--border)] bg-[var(--background)] px-2 text-[13px] text-[var(--foreground)] focus-visible:outline-none focus-visible:ring-1',
      className
    )}
  >
    {selectedLabel}
    <span class="text-[var(--muted)]">▾</span>
  </SelectPrimitive.Trigger>
  <SelectPrimitive.Portal>
    <SelectPrimitive.Content
      class="z-50 rounded border border-[var(--border)] bg-[var(--surface)] p-1 text-[13px] shadow"
    >
      {#each options as o (o.value)}
        <SelectPrimitive.Item
          value={o.value}
          label={o.label}
          class="flex cursor-pointer select-none items-center rounded px-2 py-1 text-[var(--foreground)] data-[highlighted]:bg-[var(--background)] data-[state=checked]:text-[var(--accent)]"
        >
          {o.label}
        </SelectPrimitive.Item>
      {/each}
    </SelectPrimitive.Content>
  </SelectPrimitive.Portal>
</SelectPrimitive.Root>
