import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { svelteTesting } from '@testing-library/svelte/vite';
// vitest 4 dropped the `test` key from vite's UserConfig type; import
// defineConfig from vitest/config so the inline test block type-checks.
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit(), svelteTesting()],
  test: {
    environment: 'happy-dom',
    setupFiles: ['./vitest-setup.ts'],
    environmentOptions: { happyDOM: { url: 'http://localhost/' } }
  }
});
