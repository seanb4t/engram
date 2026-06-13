<script lang="ts" module>
  import { tv, type VariantProps } from 'tailwind-variants';

  // shadcn-svelte Button styled with engram's CSS-variable tokens (app.css).
  export const buttonVariants = tv({
    base: 'inline-flex items-center justify-center whitespace-nowrap rounded text-[13px] font-[inherit] transition-colors focus-visible:outline-none focus-visible:ring-1 disabled:pointer-events-none disabled:opacity-50',
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:opacity-90',
        outline: 'border border-border bg-background text-foreground hover:bg-accent',
        ghost: 'text-foreground hover:bg-accent',
        surface: 'bg-secondary text-secondary-foreground border border-border hover:bg-accent'
      },
      size: {
        default: 'h-8 px-3 py-1',
        sm: 'h-7 px-2',
        block: 'w-full text-left justify-start px-2 py-1',
        icon: 'h-8 w-8 p-0',
        'icon-sm': 'h-7 w-7 p-0'
      }
    },
    defaultVariants: { variant: 'outline', size: 'default' }
  });

  export type ButtonVariant = VariantProps<typeof buttonVariants>['variant'];
  export type ButtonSize = VariantProps<typeof buttonVariants>['size'];
</script>

<script lang="ts">
  import { Button as ButtonPrimitive } from 'bits-ui';
  import { cn } from '$lib/utils';

  let {
    class: className,
    variant = 'outline',
    size = 'default',
    ref = $bindable(null),
    children,
    ...restProps
  }: ButtonPrimitive.RootProps & { variant?: ButtonVariant; size?: ButtonSize } = $props();
</script>

<ButtonPrimitive.Root bind:ref class={cn(buttonVariants({ variant, size }), className)} {...restProps}>
  {@render children?.()}
</ButtonPrimitive.Root>
