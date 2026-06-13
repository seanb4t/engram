<script lang="ts">
  import { Checkbox as CheckboxPrimitive } from 'bits-ui';
  import { cn } from '$lib/utils';

  let {
    class: className,
    checked = $bindable(false),
    ref = $bindable(null),
    ...restProps
  }: CheckboxPrimitive.RootProps = $props();
</script>

<CheckboxPrimitive.Root
  bind:ref
  bind:checked
  class={cn(
    'inline-flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-sm border border-border bg-background align-middle focus-visible:outline-none focus-visible:ring-1 disabled:opacity-50 data-[state=checked]:bg-primary data-[state=checked]:text-primary-foreground',
    className
  )}
  {...restProps}
>
  {#snippet children({ checked: isChecked })}
    {#if isChecked}
      <span class="text-[10px] leading-none">✓</span>
    {/if}
  {/snippet}
</CheckboxPrimitive.Root>
