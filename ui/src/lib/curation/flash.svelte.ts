import { SvelteSet } from 'svelte/reactivity';

// D-10: the 1600ms row-flash set. A row whose id is patched in place after an
// archive/restore joins `flashing` for FLASH_MS, then leaves it -- plan 04-05
// makes ResultRow read `flashing` to render the transient highlight. This
// module owns exactly one reactive set shared by every curation surface, the
// same "one shared singleton" shape as ui/src/lib/display.svelte.ts.

export const FLASH_MS = 1600;

export const flashing = new SvelteSet<string>();

// Per-id pending timers, so re-flashing an id (a second archive/restore of
// the same row within the window) restarts its own timer rather than
// stacking a second one that could fire early and drop the id before the
// newer flash's window elapses.
const timers = new Map<string, ReturnType<typeof setTimeout>>();

export function flashRows(ids: string[], ms = FLASH_MS): void {
  for (const id of ids) {
    flashing.add(id);
    const existing = timers.get(id);
    if (existing) clearTimeout(existing);
    const timer = setTimeout(() => {
      flashing.delete(id);
      timers.delete(id);
    }, ms);
    timers.set(id, timer);
  }
}
