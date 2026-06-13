export type ScopeType = 'repo' | 'discovery' | 'project' | '';

export interface ParsedScope {
  full: string;   // verbatim original — never lose this
  type: ScopeType;
  org: string;    // '' when none (e.g. project)
  name: string;   // the repo/project name (last path segment)
}

// Scopes look like `type:body`:
//   repo:github.com/org/name
//   discovery:repo:github.com/org/name   (discovery nests a repo body)
//   project:name
export function parseScope(full: string): ParsedScope {
  const firstColon = full.indexOf(':');
  if (firstColon === -1) return { full, type: '', org: '', name: full };

  const head = full.slice(0, firstColon);
  let rest = full.slice(firstColon + 1);
  let type: ScopeType = head === 'repo' || head === 'discovery' || head === 'project' ? head : '';

  // discovery:repo:github.com/... — unwrap the inner repo body for org/name.
  if (type === 'discovery' && rest.startsWith('repo:')) rest = rest.slice('repo:'.length);

  // Drop a leading host (github.com/...) so org/name leads.
  const segs = rest.split('/').filter(Boolean);
  if (segs.length >= 3 && segs[0].includes('.')) segs.shift();

  if (segs.length >= 2) return { full, type, org: segs[segs.length - 2], name: segs[segs.length - 1] };
  return { full, type, org: '', name: segs[0] ?? rest };
}
