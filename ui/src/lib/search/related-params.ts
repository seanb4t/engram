// The one URL/path codec for /related (D-03, D-04) — one parse function, one
// encode function, so the two cannot drift (mirrors scheduled-params.ts's
// contract). `from` records where the related view was opened from (D-04's
// entry half); `trail` records the walk across the related view's own graph
// (consumed starting in plan 05-08).

import { isAllowedDestination } from '$lib/resume';
import { UUID_RE, SHORT_ID_RE } from './classify';

export const RELATED_TRAIL_MAX = 32;

export interface RelatedParams {
  from: string;
  trail: string[];
}

export function defaultRelatedParams(): RelatedParams {
  return { from: '', trail: [] };
}

// `from` is kept verbatim only when it resolves — through the existing
// open-redirect guard, isAllowedDestination — to a same-app console route.
// An absolute or protocol-relative URL, or a route /related itself owns,
// parses to ''. `trail` entries that are neither a UUID nor a short_id are
// dropped; at most RELATED_TRAIL_MAX are kept, most-recent last.
export function parseRelatedParams(sp: URLSearchParams): RelatedParams {
  const rawFrom = sp.get('from') ?? '';
  const from = isAllowedDestination(rawFrom) ? rawFrom : '';
  const rawTrail = (sp.get('trail') ?? '').split(',');
  const trail = rawTrail.filter((t) => UUID_RE.test(t) || SHORT_ID_RE.test(t)).slice(-RELATED_TRAIL_MAX);
  return { from, trail };
}

// Defaults are never written back: encode(default()) round-trips to ''.
// Canonical key order: from, trail.
export function encodeRelatedParams(p: RelatedParams): string {
  const sp = new URLSearchParams();
  if (p.from) sp.set('from', p.from);
  if (p.trail.length > 0) sp.set('trail', p.trail.join(','));
  return sp.toString();
}

// Base-less: callers prefix `${base}` (SvelteKit's static-adapter base path).
export function relatedPath(id: string, p: RelatedParams): string {
  const query = encodeRelatedParams(p);
  return `/related/${encodeURIComponent(id)}${query ? `?${query}` : ''}`;
}
