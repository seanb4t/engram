export const PAGE_LIMIT = 50;

export interface ObserveParams {
  scope: string; categories: string[]; visibility: string; offset: number; selectedId: string;
}

export function parseObserveParams(sp: URLSearchParams): ObserveParams {
  return {
    scope: sp.get('scope') ?? '',
    categories: sp.getAll('cat'),
    visibility: sp.get('vis') ?? '',
    offset: Number(sp.get('offset') ?? '0') || 0,
    selectedId: sp.get('sel') ?? ''
  };
}

export function observeSearch(p: ObserveParams): string {
  const sp = new URLSearchParams();
  if (p.scope) sp.set('scope', p.scope);
  for (const c of p.categories) sp.append('cat', c);
  if (p.visibility) sp.set('vis', p.visibility);
  if (p.offset) sp.set('offset', String(p.offset));
  if (p.selectedId) sp.set('sel', p.selectedId);
  return sp.toString();
}

export function listMemoriesKey(scope: string, categories: string[], visibility: string, limit: number, offset: number) {
  return ['listMemories', scope, categories, visibility, limit, offset];
}
