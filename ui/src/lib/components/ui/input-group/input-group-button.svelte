<script lang="ts" module>
	import { tv, type VariantProps } from "tailwind-variants";

	const inputGroupButtonVariants = tv({
		base: "gap-2 text-sm flex items-center shadow-none",
		variants: {
			size: {
				xs: "h-6 gap-1 rounded-[calc(var(--radius)-3px)] px-1.5 [&>svg:not([class*='size-'])]:size-3.5",
				sm: "cn-input-group-button-size-sm",
				"icon-xs": "size-6 rounded-[calc(var(--radius)-3px)] p-0 has-[>svg]:p-0",
				"icon-sm": "size-8 p-0 has-[>svg]:p-0",
			},
		},
		defaultVariants: {
			size: "xs",
		},
	});

	export type InputGroupButtonSize = VariantProps<typeof inputGroupButtonVariants>["size"];
</script>

<script lang="ts">
	import { cn } from "$lib/utils.js";
	import type { Snippet } from "svelte";
	import type { HTMLButtonAttributes } from "svelte/elements";
	import { Button, type ButtonVariant } from "$lib/components/ui/button/index.js";

	// Typed on HTMLButtonAttributes (button mode) rather than
	// ComponentProps<typeof Button>: the latter is the bits-ui anchor+button
	// prop union, and Omit-ing over it trips svelte-check's TS2590
	// "union type too complex to represent" (engram #311).
	let {
		ref = $bindable(null),
		class: className,
		children,
		type = "button",
		variant = "ghost",
		size = "xs",
		...restProps
	}: Omit<HTMLButtonAttributes, "size"> & {
		ref?: HTMLElement | null;
		children?: Snippet;
		variant?: ButtonVariant;
		size?: InputGroupButtonSize;
	} = $props();
</script>

<Button
	bind:ref
	{type}
	data-size={size}
	{variant}
	class={cn(inputGroupButtonVariants({ size }), className)}
	{...(restProps as HTMLButtonAttributes)}
>
	{@render children?.()}
</Button>
