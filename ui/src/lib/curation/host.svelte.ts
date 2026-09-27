import type { Memory } from '$lib/gen/engram_pb';

// The active curation host registry: a single shared reactive module (same
// "one shared reactive module" shape as flash.svelte.ts/display.svelte.ts)
// that lets a cross-route consumer -- the ⌘K command menu (plans 04-08,
// 04-10) -- reach whichever CurationSurfaces instance is mounted on the
// CURRENT route without importing route-specific components.

export type CurationAction = 'supersede' | 'archive' | 'restore' | 'chain';

export interface CurationHostApi {
  actionsFor(m: Memory): CurationAction[];
  run(action: CurationAction, ids: string[]): void;
}

export const curationHost = $state<{ current: CurationHostApi | null }>({ current: null });

// registerCurationHost sets `current` and returns an unregister function.
// The unregister clears `current` ONLY if it is STILL this api -- a stale
// unmount (e.g. a fast route switch remounting a new host before the old
// one's cleanup runs) can never clobber a newer registration that already
// replaced it.
export function registerCurationHost(api: CurationHostApi): () => void {
  curationHost.current = api;
  return () => {
    if (curationHost.current === api) curationHost.current = null;
  };
}

// defaultActionsFor is the fallback per-record action set: supersede iff the
// record is neither a rule nor a discovery (no `kind`); archive iff not
// already archived; restore iff archived; chain iff the record has a
// successor or at least one predecessor. Mirrors DetailPane's own per-action
// fences (D-05/D-06/D-15/D-16) so a consumer that has no bespoke gating of
// its own still gets the same rules.
export function defaultActionsFor(m: Memory): CurationAction[] {
  const actions: CurationAction[] = [];
  if (m.category !== 'rule' && m.kind === '') actions.push('supersede');
  if (m.archivedAt) actions.push('restore');
  else actions.push('archive');
  if (m.supersededBy || m.supersedes.length > 0) actions.push('chain');
  return actions;
}
