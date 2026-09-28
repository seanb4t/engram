// The one URL/request codec for /rules (D-11) — declared once so parse and
// encode cannot drift, mirroring params.ts's rule for /search. Rules has no
// free-text query or facets: the ONLY URL state is which rule is open.

export interface RulesParams {
  sel: string;
}

export function defaultRulesParams(): RulesParams {
  return { sel: '' };
}

export function parseRulesParams(sp: URLSearchParams): RulesParams {
  return { sel: sp.get('sel') ?? '' };
}

// Defaults are never written back — encode(default()) round-trips to ''.
export function encodeRulesParams(p: RulesParams): string {
  const sp = new URLSearchParams();
  if (p.sel) sp.set('sel', p.sel);
  return sp.toString();
}
