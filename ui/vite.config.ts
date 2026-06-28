import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { playwright } from '@vitest/browser-playwright';
// vitest 4 dropped the `test` key from vite's UserConfig type; import
// defineConfig from vitest/config so the inline test block type-checks.
import { defineConfig, type Plugin } from 'vitest/config';

// svelte.config.js pins kit.paths.base = '/ui', so the sveltekit() plugin sets
// Vite's `base` to '/ui'. Vitest 4 browser mode serves its orchestrator/client
// assets at root-absolute URLs (/__vitest_browser__/*, /@fs/*) that carry no
// base prefix, so under '/ui' they 404 and the browser session never connects
// (60s "Failed to connect to the browser session" stall). Force base back to
// '/' under Vitest only; the real `vite build` keeps '/ui'.
const forceTestBase: Plugin = {
  name: 'engram:force-test-base',
  config: () => (process.env.VITEST ? { base: '/' } : {})
};

export default defineConfig({
  plugins: [tailwindcss(), sveltekit(), forceTestBase],
  test: {
    projects: [
      {
        extends: true,
        test: {
          name: 'node',
          include: ['src/**/*.test.ts'],
          exclude: ['src/**/*.browser.test.ts'],
          environment: 'happy-dom',
          setupFiles: ['./vitest-setup.ts'],
          environmentOptions: { happyDOM: { url: 'http://localhost/' } }
        }
      },
      {
        extends: true,
        test: {
          name: { label: 'browser', color: 'green' },
          include: ['src/**/*.browser.test.ts'],
          setupFiles: ['./vitest-setup.browser.ts'],
          browser: {
            enabled: true,
            provider: playwright(),
            headless: true,
            instances: [{ browser: 'chromium' }]
          }
        }
      }
    ]
  }
});
