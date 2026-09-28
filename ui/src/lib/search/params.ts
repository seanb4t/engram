import { INCLUDE_STATES } from '$lib/queries';
import type { OperatorChip } from './classify';

// The one URL/request codec for /search, the header search hand-off (D-10)
// and the command menu (Phase 3, 02-05) — declared once so parse and encode
// cannot drift (ENTRY-06).

export interface SearchParams {
  q: string;
  scope: string;
  crossSpine: boolean;
  categories: string[];
  tags: string[];
  createdAfter: string;
  createdBefore: string;
  includeArchived: boolean;
  includeSuperseded: boolean;
  includeScheduled: boolean;
  k: number;
  sel: string;
}

export const DEFAULT_K = 50;
export const K_STEPS: readonly number[] = [50, 100, 250, 1000];

export function defaultSearchParams(): SearchParams {
  return {
    q: '',
    scope: '',
    crossSpine: true,
    categories: [],
    tags: [],
    createdAfter: '',
    createdBefore: '',
    includeArchived: false,
    includeSuperseded: false,
    includeScheduled: false,
    k: DEFAULT_K,
    sel: ''
  };
}

// scope is required unless cross_spine is true (the server's own rule) — a
// non-empty scope always wins over whatever `xs` says; `xs=0` is the only
// way to express "cross-spine off with no scope" (the state that triggers
// the server's real rejection rather than an inferred client-side one).
export function parseSearchParams(sp: URLSearchParams): SearchParams {
  const scope = sp.get('scope') ?? '';
  const crossSpine = scope ? false : sp.get('xs') !== '0';
  const inc = sp.getAll('inc').filter((v) => INCLUDE_STATES.includes(v));
  const kRaw = Number(sp.get('k') ?? '');
  const k = K_STEPS.includes(kRaw) ? kRaw : DEFAULT_K;
  return {
    q: sp.get('q') ?? '',
    scope,
    crossSpine,
    categories: sp.getAll('cat'),
    tags: sp.getAll('tag'),
    createdAfter: sp.get('after') ?? '',
    createdBefore: sp.get('before') ?? '',
    includeArchived: inc.includes('archived'),
    includeSuperseded: inc.includes('superseded'),
    includeScheduled: inc.includes('scheduled'),
    k,
    sel: sp.get('sel') ?? ''
  };
}

// Canonical key order: q, scope, xs, cat, tag, after, before, inc, k, sel.
// Defaults are never written back, so parse(encode(defaultSearchParams()))
// round-trips to '' rather than a URL full of redundant params.
export function encodeSearchParams(p: SearchParams): string {
  const sp = new URLSearchParams();
  if (p.q) sp.set('q', p.q);
  if (p.scope) sp.set('scope', p.scope);
  if (!p.scope && !p.crossSpine) sp.set('xs', '0');
  for (const c of [...new Set(p.categories)].sort()) sp.append('cat', c);
  for (const t of [...new Set(p.tags)].sort()) sp.append('tag', t);
  if (p.createdAfter) sp.set('after', p.createdAfter);
  if (p.createdBefore) sp.set('before', p.createdBefore);
  const incFlags: Record<string, boolean> = {
    archived: p.includeArchived,
    superseded: p.includeSuperseded,
    scheduled: p.includeScheduled
  };
  for (const v of INCLUDE_STATES) if (incFlags[v]) sp.append('inc', v);
  if (p.k !== DEFAULT_K) sp.set('k', String(p.k));
  if (p.sel) sp.set('sel', p.sel);
  return sp.toString();
}

// First element is the literal RPC name so the existing write mutations'
// invalidation and optimistic patching (keyed off that same RPC name) reach
// this query too.
export function searchMemoriesKey(p: SearchParams, full: boolean) {
  return [
    'searchMemories',
    p.q,
    p.scope,
    p.crossSpine,
    [...p.categories].sort(),
    [...p.tags].sort(),
    p.createdAfter,
    p.createdBefore,
    p.includeArchived,
    p.includeSuperseded,
    p.includeScheduled,
    p.k,
    full
  ];
}

export function searchMemoriesRequest(p: SearchParams, full: boolean) {
  return {
    query: p.q,
    // Never both: a non-empty scope always forces crossSpine false.
    scope: p.scope,
    crossSpine: !p.scope && p.crossSpine,
    k: BigInt(p.k),
    tags: p.tags,
    categories: p.categories,
    full,
    createdAfter: p.createdAfter,
    createdBefore: p.createdBefore,
    includeArchived: p.includeArchived,
    includeSuperseded: p.includeSuperseded,
    includeScheduled: p.includeScheduled
  };
}

// D-08: Show more escalates k through K_STEPS; undefined once already at the
// ceiling (1000), which the caller uses to decide whether to render the row.
export function nextK(k: number): number | undefined {
  const idx = K_STEPS.indexOf(k);
  if (idx === -1 || idx === K_STEPS.length - 1) return undefined;
  return K_STEPS[idx + 1];
}

// D-09: the cursor-mode key for operator-only ListMemories infinite scroll.
// Shape mirrors ui/src/lib/queries.ts's listMemoriesKey (same leading RPC
// name and index-3 visibility slot the existing mutations' invalidation
// reads) but marks cursor mode explicitly ('cursor' at the offset slot, 50 at
// the limit slot) so it can never collide with an offset-mode key.
export function listMemoriesCursorKey(p: SearchParams) {
  return [
    'listMemories',
    p.scope,
    [...p.categories].sort(),
    '',
    50,
    'cursor',
    p.includeArchived,
    p.includeSuperseded,
    p.includeScheduled,
    !p.scope && p.crossSpine,
    [...p.tags].sort(),
    p.createdAfter,
    p.createdBefore
  ];
}

export function listMemoriesRequest(p: SearchParams, pageToken: string) {
  return {
    scope: p.scope,
    // Never both: a non-empty scope always forces crossSpine false.
    crossSpine: !p.scope && p.crossSpine,
    limit: 50n,
    cursorMode: true,
    pageToken,
    tags: p.tags,
    categories: p.categories,
    full: true,
    createdAfter: p.createdAfter,
    createdBefore: p.createdBefore,
    includeArchived: p.includeArchived,
    includeSuperseded: p.includeSuperseded,
    includeScheduled: p.includeScheduled
  };
}

// Derives params from a COMPLETE chip set (classify.ts re-parses the raw
// input on every keystroke, so chips always represent the current whole
// state — this replaces rather than merges). Multiple scope chips: the last
// one wins (documented in classify.ts).
export function applyChips(p: SearchParams, chips: OperatorChip[]): SearchParams {
  const scopeChips = chips.filter((c): c is Extract<OperatorChip, { kind: 'scope' }> => c.kind === 'scope');
  const tagChips = chips.filter((c): c is Extract<OperatorChip, { kind: 'tag' }> => c.kind === 'tag');
  const categoryChips = chips.filter((c): c is Extract<OperatorChip, { kind: 'category' }> => c.kind === 'category');

  const scope = scopeChips.length > 0 ? scopeChips[scopeChips.length - 1].value : p.scope;
  const tags = [...new Set(tagChips.map((c) => c.value))].sort();
  const categories = [...new Set(categoryChips.map((c) => c.value))].sort();

  return {
    ...p,
    scope,
    crossSpine: scope ? false : p.crossSpine,
    tags,
    categories
  };
}
