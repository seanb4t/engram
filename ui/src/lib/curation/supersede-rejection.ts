// Maps a SupersedeMemory rejection (preview or commit) onto the per-target
// issue vocabulary the dialog renders. Server preflight order (tools.go
// supersedeArgChecks -> resolveAndAuthorizeSupersedeTargets ->
// validateSupersedeTargetState): field checks first, then targets, one
// class per response -- NotFound `not found: <inputs>`, FailedPrecondition
// `rules are always shared: <inputs> — delete the rule instead of
// superseding it`, FailedPrecondition `target is already superseded:
// <inputs>`. Inputs are the caller's own strings (id or short_id),
// comma-separated.
import { ConnectError, Code } from '@connectrpc/connect';
import { parseConnectError } from '$lib/errors/connect-error';

export type SupersedeRejection =
  | { kind: 'targets'; issue: 'not-found' | 'rule' | 'already-superseded'; inputs: string[] }
  | { kind: 'field'; fields: string[]; hint: string; detail: string }
  | { kind: 'reauth' }
  | { kind: 'opaque'; codeName: string; detail: string };

// UI-SPEC copy, verbatim, held once here so SupersedeDialog never re-types
// them. `notOwned`/`alreadySuperseded` are parameterized (owner / current
// head short_id); `rule`/`serverRace` are fixed strings.
export const TARGET_ISSUE_COPY = {
  notOwned: (owner: string) => `shared by ${owner} — readable, but only the owner can supersede`,
  rule: 'rules cannot be superseded — delete it instead',
  alreadySuperseded: (headShortId: string) => `not the live head — current head is ${headShortId}`,
  serverRace: 'already superseded by another session'
} as const;

export const NOT_FOUND_NOTE =
  'Not found, not owned and ambiguous short id are one rejection by design. Nothing was changed.';

const RULE_RE = /^rules are always shared: (.+) — delete the rule instead of superseding it$/;
const ALREADY_SUPERSEDED_RE = /^target is already superseded: (.+)$/;
const NOT_FOUND_RE = /^not found: (.+)$/;

function splitInputs(raw: string): string[] {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

export function parseSupersedeRejection(err: unknown): SupersedeRejection {
  if (err instanceof ConnectError && (err.code === Code.Unauthenticated || err.code === Code.PermissionDenied)) {
    return { kind: 'reauth' };
  }

  // parseConnectError handles the field=/hint= envelope shape (and the
  // ambiguous-short-id case, which is not part of this dialog's own
  // vocabulary -- an ambiguous short id falls through to opaque below,
  // same as any other ConnectError shape this parser does not name).
  const parsed = parseConnectError(err);
  if (parsed.kind === 'rejected') {
    return { kind: 'field', fields: parsed.fields, hint: parsed.hint, detail: parsed.detail };
  }

  if (err instanceof ConnectError) {
    const raw = err.rawMessage;
    const rule = RULE_RE.exec(raw);
    if (rule) return { kind: 'targets', issue: 'rule', inputs: splitInputs(rule[1]) };
    const alreadySuperseded = ALREADY_SUPERSEDED_RE.exec(raw);
    if (alreadySuperseded) return { kind: 'targets', issue: 'already-superseded', inputs: splitInputs(alreadySuperseded[1]) };
    const notFound = NOT_FOUND_RE.exec(raw);
    if (notFound) return { kind: 'targets', issue: 'not-found', inputs: splitInputs(notFound[1]) };
  }

  const codeName = err instanceof ConnectError ? (Code[err.code] ?? String(err.code)) : 'unknown';
  const detail = err instanceof ConnectError ? err.rawMessage : String(err);
  return { kind: 'opaque', codeName, detail };
}
