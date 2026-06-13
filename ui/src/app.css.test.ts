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
