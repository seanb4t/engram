export const PAGE_LIMIT = 50;

export type Category = 'convention' | 'gotcha' | 'decision' | 'preference';
export type Visibility = '' | 'private' | 'shared';

export const CATEGORIES: readonly Category[] = ['convention', 'gotcha', 'decision', 'preference'];
const VISIBILITIES: readonly Visibility[] = ['', 'private', 'shared'];

/** Recognised `inc` URL-parameter values, in canonical order. Shared by parse and encode
 * so the two never drift out of sync. */
export const INCLUDE_STATES: readonly string[] = ['archived', 'superseded', 'scheduled'];

export interface ObserveParams {
  scope: string; categories: Category[]; visibility: Visibility; offset: number; selectedId: string;
  includeArchived: boolean; includeSuperseded: boolean; includeScheduled: boolean;
}

export function parseObserveParams(sp: URLSearchParams): ObserveParams {
  const vis = sp.get('vis') ?? '';
  const inc = sp.getAll('inc').filter((v) => INCLUDE_STATES.includes(v));
  return {
    scope: sp.get('scope') ?? '',
    categories: sp.getAll('cat').filter((c): c is Category => (CATEGORIES as readonly string[]).includes(c)),
    visibility: (VISIBILITIES as readonly string[]).includes(vis) ? (vis as Visibility) : '',
    offset: Number(sp.get('offset') ?? '0') || 0,
    selectedId: sp.get('sel') ?? '',
    includeArchived: inc.includes('archived'),
    includeSuperseded: inc.includes('superseded'),
    includeScheduled: inc.includes('scheduled')
  };
}

export function observeSearch(p: ObserveParams): string {
  const sp = new URLSearchParams();
  if (p.scope) sp.set('scope', p.scope);
  for (const c of p.categories) sp.append('cat', c);
  if (p.visibility) sp.set('vis', p.visibility);
  const incFlags: Record<string, boolean> = {
    archived: p.includeArchived, superseded: p.includeSuperseded, scheduled: p.includeScheduled
  };
  for (const v of INCLUDE_STATES) if (incFlags[v]) sp.append('inc', v);
  if (p.offset) sp.set('offset', String(p.offset));
  if (p.selectedId) sp.set('sel', p.selectedId);
  return sp.toString();
}

export function listMemoriesKey(
  scope: string, categories: string[], visibility: string, limit: number, offset: number,
  includeArchived: boolean, includeSuperseded: boolean, includeScheduled: boolean
) {
  return ['listMemories', scope, categories, visibility, limit, offset, includeArchived, includeSuperseded, includeScheduled];
}
