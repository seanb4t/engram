import { CATEGORIES } from '$lib/queries';

// This is the ONE classifier every id-accepting box in the console uses
// (header search, /search, the ⌘K hand-off row). It decides which RPC a raw
// string resolves to and never silently reinterprets: an id-shaped input is
// never sent as a semantic query, and a short_id-shaped miss is the caller's
// job to re-search as text with a visible note, not this module's.

// 8-4-4-4-12 hex, case-insensitive.
export const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

// Crockford base32, exactly 10 characters, excluding i, l, o, u (case-insensitive).
export const SHORT_ID_RE = /^[0-9a-hjkmnp-tv-z]{10}$/i;

export const KNOWN_CATEGORIES: readonly string[] = [...CATEGORIES, 'discovery', 'rule'];

export type OperatorChip =
  | { kind: 'scope'; value: string }
  | { kind: 'tag'; value: string }
  | { kind: 'category'; value: string; known: boolean }
  | { kind: 'pending'; raw: string };

export type Classified =
  | { kind: 'empty' }
  | { kind: 'id'; id: string }
  | { kind: 'short_id'; shortId: string }
  | { kind: 'text'; text: string; chips: OperatorChip[] }
  | { kind: 'operators'; chips: OperatorChip[] };

function parseOperatorToken(token: string): OperatorChip | null {
  let prefixLen: number;
  let kind: 'scope' | 'tag' | 'category';
  if (token.startsWith('scope:')) {
    prefixLen = 'scope:'.length;
    kind = 'scope';
  } else if (token.startsWith('in:')) {
    prefixLen = 'in:'.length;
    kind = 'scope';
  } else if (token.startsWith('tag:')) {
    prefixLen = 'tag:'.length;
    kind = 'tag';
  } else if (token.startsWith('#')) {
    prefixLen = 1;
    kind = 'tag';
  } else if (token.startsWith('is:')) {
    prefixLen = 'is:'.length;
    kind = 'category';
  } else {
    return null;
  }
  const value = token.slice(prefixLen);
  if (!value) return { kind: 'pending', raw: token };
  if (kind === 'category') return { kind: 'category', value, known: KNOWN_CATEGORIES.includes(value) };
  return { kind, value };
}

export function classifyInput(raw: string): Classified {
  const trimmed = raw.trim();
  if (!trimmed) return { kind: 'empty' };

  // Collapse whitespace runs so 'a   b' reads as 'a b'.
  const tokens = trimmed.split(/\s+/);

  // Single-token UUID/short_id checks run BEFORE operator parsing — an id
  // shaped like an operator token (none are, by construction) would still
  // never reach here since operators only ever match multi-char prefixes.
  if (tokens.length === 1) {
    if (UUID_RE.test(tokens[0])) return { kind: 'id', id: tokens[0].toLowerCase() };
    if (SHORT_ID_RE.test(tokens[0])) return { kind: 'short_id', shortId: tokens[0].toLowerCase() };
  }

  const chips: OperatorChip[] = [];
  const textTokens: string[] = [];
  for (const token of tokens) {
    const chip = parseOperatorToken(token);
    if (chip) chips.push(chip);
    else textTokens.push(token);
  }

  // Multiple scope chips are all kept here; applying them to a request (see
  // ui/src/lib/search/params.ts's applyChips) is documented there as
  // last-wins.
  const text = textTokens.join(' ');
  if (text) return { kind: 'text', text, chips };
  return { kind: 'operators', chips };
}
