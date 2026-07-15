import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { CATEGORIES } from '$lib/queries';

// app.css is read from source rather than imported: `@tailwindcss/vite` consumes
// `.css` imports (even `?raw` resolves to an empty string), so the raw token text
// is only reachable off disk. (svelte-check flags the node:* imports because the
// project tsconfig scopes `types` to jest-dom; vitest — the actual runner — and
// CI's `pnpm build` are unaffected.)
//
// MemoryRow + MemoryDetail render every memory Category, plus `discovery`-category
// records on the /discovery route, through `var(--cat-{category})`. A category
// without a token renders an unstyled dot/badge, so each must be defined. The list
// derives from the source-of-truth CATEGORIES (queries.ts) so a newly added
// category can't silently reintroduce the missing-token bug.
const RENDERABLE_CATEGORIES = [...CATEGORIES, 'discovery'] as const;

const css = readFileSync(resolve(process.cwd(), 'src/app.css'), 'utf8');
// Extract each block's body by selector, not by position, so a reordered file (or
// a second @theme block) can't silently shift the boundaries. None of these blocks
// nest braces, so `[^}]*` captures the whole body. A bare `.dark` also appears in
// the `@custom-variant dark (&:is(.dark *))` line, but the required `{` excludes it.
const body = (re: RegExp): string => css.match(re)?.[1] ?? '';
const root = body(/:root\s*\{([^}]*)\}/);
const dark = body(/\.dark\s*\{([^}]*)\}/);
const theme = body(/@theme[^{]*\{([^}]*)\}/);

describe('category color tokens', () => {
  for (const cat of RENDERABLE_CATEGORIES) {
    it(`defines --cat-${cat} in :root, .dark, and the @theme bridge`, () => {
      expect(root).toContain(`--cat-${cat}:`);
      expect(dark).toContain(`--cat-${cat}:`);
      expect(theme).toContain(`--color-cat-${cat}:`);
    });
  }
});

describe('destructive tokens', () => {
  it('defines --destructive and --destructive-foreground in :root and .dark', () => {
    expect(root).toContain('--destructive:');
    expect(root).toContain('--destructive-foreground:');
    expect(dark).toContain('--destructive:');
    expect(dark).toContain('--destructive-foreground:');
  });

  it('bridges --color-destructive and --color-destructive-foreground in @theme', () => {
    expect(theme).toContain('--color-destructive:');
    expect(theme).toContain('--color-destructive-foreground:');
  });

  it('sets .dark --destructive-foreground per-theme (var(--background)), not hardcoded white', () => {
    // Round-2 contrast fix: dark-mode --cat-gotcha is a light orange (#ffa657)
    // where white text reads weak. --destructive-foreground must resolve via
    // var(--background) in both blocks, not a literal #ffffff in .dark.
    expect(dark).toContain('--destructive-foreground: var(--background);');
    expect(dark).not.toMatch(/--destructive-foreground:\s*#fff/i);
  });
});
