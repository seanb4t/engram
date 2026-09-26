import { ConnectError, Code } from '@connectrpc/connect';

// Classifies a header-search (or /search) RPC failure from the SAME
// discipline ui/src/lib/client.ts's mapAuthError already uses: read
// `.rawMessage` (the unprefixed server text) and `.code` (the numeric Code
// enum), never `.message` (code-prefixed, see PITFALLS.md Pitfall C and
// 02-RESEARCH.md's "Don't Hand-Roll" anti-pattern). Two error shapes share
// CodeFailedPrecondition — an ambiguous short_id and a field=/hint= envelope
// rejection — so the ambiguous check MUST run before the envelope check.

export type ParsedConnectError =
  | { kind: 'not-found' }
  | { kind: 'ambiguous-short-id'; shortId: string }
  | { kind: 'rejected'; fields: string[]; hint: string; detail: string; code: Code }
  | { kind: 'opaque'; code?: Code; codeName: string; detail: string };

export type FixRow = {
  id: 'enable-cross-spine' | 'pick-scope' | 'lower-k' | 'without-full' | 'clear-created' | 'retry';
  label: string;
};

const AMBIGUOUS_RE = /^ambiguous short id: (.+)$/;
const ENVELOPE_RE = /^field=([^ ]+) hint=([^:]+): ([\s\S]*)$/;

export function parseConnectError(err: unknown): ParsedConnectError {
  if (!(err instanceof ConnectError)) {
    return { kind: 'opaque', codeName: 'unknown', detail: String(err) };
  }
  if (err.code === Code.NotFound) return { kind: 'not-found' };

  // Ambiguous short_id BEFORE the envelope check — both are
  // CodeFailedPrecondition (Pitfall C).
  const ambiguous = AMBIGUOUS_RE.exec(err.rawMessage);
  if (ambiguous) return { kind: 'ambiguous-short-id', shortId: ambiguous[1] };

  const envelope = ENVELOPE_RE.exec(err.rawMessage);
  if (envelope) {
    return {
      kind: 'rejected',
      fields: envelope[1].split(','),
      hint: envelope[2],
      detail: envelope[3],
      code: err.code
    };
  }

  return { kind: 'opaque', code: err.code, codeName: Code[err.code] ?? String(err.code), detail: err.rawMessage };
}

// fixRowsFor maps a rejected envelope's hint to actionable fix rows; retry is
// always last (and the only row for anything that is not a rejection).
export function fixRowsFor(p: ParsedConnectError): FixRow[] {
  const retry: FixRow = { id: 'retry', label: 'Retry the request' };
  if (p.kind !== 'rejected') return [retry];

  const rows: FixRow[] = [];
  if (p.hint === 'conditional_required' && p.fields.includes('cross_spine')) {
    rows.push({ id: 'enable-cross-spine', label: 'Re-enable cross-spine' });
    rows.push({ id: 'pick-scope', label: 'Pick one scope instead' });
  } else if (p.hint === 'out_of_range' && p.fields.includes('k')) {
    rows.push({ id: 'lower-k', label: 'Show fewer results' });
  } else if (p.hint === 'response_too_large') {
    rows.push({ id: 'lower-k', label: 'Show fewer results' });
    rows.push({ id: 'without-full', label: 'Retry without full content' });
  } else if (p.hint === 'format' && p.fields.includes('created_after')) {
    rows.push({ id: 'clear-created', label: 'Clear the created-time filter' });
  }
  rows.push(retry);
  return rows;
}
