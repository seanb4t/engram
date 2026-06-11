import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({ fallback: 'index.html' }),
    paths: { base: '/ui', relative: false },
    // Pin the app version so the build is byte-reproducible: SvelteKit's default
    // version is a Date.now() timestamp (written to _app/version.json and folded
    // into entry-chunk hashes), which makes the vendored SPA differ on every
    // rebuild and breaks the ui-drift CI gate. The binary carries the real version.
    version: { name: 'engram' }
  }
};
export default config;
