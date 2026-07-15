import { describe, it, expect } from 'vitest';
import { buttonVariants } from './button.svelte';

describe('buttonVariants destructive variant', () => {
  it('emits bg-destructive and text-destructive-foreground classes', () => {
    const classes = buttonVariants({ variant: 'destructive' });
    expect(classes).toContain('bg-destructive');
    expect(classes).toContain('text-destructive-foreground');
  });
});
