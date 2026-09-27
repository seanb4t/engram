// Single typed resume-envelope module (Codex round-3 HIGH): centralizes the
// whole persist/peek/consume/validate lifecycle for the D-09 re-auth resume
// flow in one place instead of splitting parse/consume/delete across the
// write forms and the route host. The forms (MemoryFormSheet/
// DiscoveryFormSheet) and the curation dialogs (supersede/archive/restore/
// delete) call ONLY `persistResume` before navigating to `/auth/login`; the
// route/host (WriteSurfaces / CurationSurfaces / the Rules view) is the SOLE
// owner of `peekResume`/`consumeResume`, and passes the restored values back
// into the form/dialog as props rather than the form reading sessionStorage
// itself. This kills the two-owner deletion race a component-scoped
// mount-restore would have.
//
// This reverses CONTEXT.md's "sessionStorage deferred" note for D-09 — the
// real OIDC redirect (`/auth/login` -> IdP -> `/auth/callback` ->
// `internal/webauth/handlers.go:187` -> `/ui/`) always lands on `/ui/`, which
// has no write host, so a component-scoped in-memory draft never survives
// the round-trip. The envelope is what makes D-09 actually hold across a
// real re-auth, not deferred polish (round-3/round-4 cross-AI review).
//
// D-16 (Phase 4): the envelope widens from the two form kinds (memory,
// discovery) to five kinds total, adding supersede/archive/delete for the
// curation surfaces. RESUME_VERSION goes to 2, but a v1 form-kind envelope
// from a pre-phase build still restores (FormResumeEnvelope.v: 1 | 2) — v2
// is additive, not a breaking rewrite of the schema peek/consume accept. A
// curation-kind envelope requires v === 2 (no pre-phase build could ever
// have written one).

const RESUME_VERSION = 2;
const RESUME_TTL_MS = 10 * 60 * 1000; // 10 minutes -- transient re-auth state, not durable.

export const RESUME_KEY = 'engram:resume';

// The FULL stored shape for the two write-form dialogs (MemoryFormSheet /
// DiscoveryFormSheet), including the version/timestamp `persistResume`
// stamps itself. `v` stays `1 | 2` so a pre-phase (v1) draft still
// type-checks and restores.
export interface FormResumeEnvelope {
  v: 1 | 2;
  ts: number;
  returnPath: string;
  kind: 'memory' | 'discovery';
  mode: 'create' | 'edit';
  recordId: string | null;
  values: Record<string, unknown>;
}

// The supersede dialog's draft: the target set, the correcting record's
// field values, and the one idempotency key reused across retries and
// across a re-auth reopen (D-07/D-15).
export interface SupersedeResumeEnvelope {
  v: 2;
  ts: number;
  returnPath: string;
  kind: 'supersede';
  targets: string[];
  fields: Record<string, unknown>;
  idempotencyKey: string;
}

// The archive/restore confirm dialog's draft. The Scheduled view's archive
// action reuses this exact kind with `returnPath: '/scheduled'` (D-16).
export interface ArchiveResumeEnvelope {
  v: 2;
  ts: number;
  returnPath: string;
  kind: 'archive';
  mode: 'archive' | 'restore';
  ids: string[];
}

// The Rules view's delete confirm. Distinct from the memory/discovery forms'
// implicit delete since it carries no `kind: 'memory' | 'discovery'`
// ambiguity to resolve -- a rule's delete is a single id, nothing else.
export interface DeleteResumeEnvelope {
  v: 2;
  ts: number;
  returnPath: string;
  kind: 'delete';
  id: string;
}

export type CurationResumeEnvelope = SupersedeResumeEnvelope | ArchiveResumeEnvelope | DeleteResumeEnvelope;

// The FULL stored shape across all five kinds.
export type ResumeEnvelope = FormResumeEnvelope | CurationResumeEnvelope;

// The caller-supplied shape -- WITHOUT `v`/`ts`, which `persistResume` stamps
// itself (Codex round-4 MEDIUM: every call site omits them, so the type must
// not require them or callers won't compile). Distributive over each member
// of the ResumeEnvelope union (via the `infer` trick below) so a caller
// passing e.g. a `kind: 'archive'` object type-checks against
// ArchiveResumeEnvelope's own fields, not a flattened union of every kind's
// fields.
export type ResumeDraft = ResumeEnvelope extends infer E
  ? E extends ResumeEnvelope
    ? Omit<E, 'v' | 'ts'>
    : never
  : never;

// SvelteKit's static-adapter base path (svelte.config.js: `paths.base =
// '/ui'`). A raw `window.location.pathname` carries this prefix; stripping
// it here means a later `goto(base + path)` on the `/ui/` landing can never
// double-prefix to `/ui/ui/...`.
const BASE_PREFIX = '/ui';

// D-14: `/observe` is removed from this list in the same change that deletes
// the route -- a stale envelope pointing at it now fails
// isAllowedDestination and is discarded, landing the operator on `/` through
// the existing rejection path, not a broken redirect. D-16 adds `/rules` and
// `/scheduled` as new resume destinations.
const ALLOWED_DESTINATIONS = ['/search', '/discovery', '/rules', '/scheduled'] as const;

export function normalizeReturnPath(returnPath: string): string {
  if (returnPath === BASE_PREFIX) return '/';
  if (returnPath.startsWith(`${BASE_PREFIX}/`)) return returnPath.slice(BASE_PREFIX.length);
  return returnPath;
}

// Rejects anything that isn't a same-app relative path under one of the
// known console routes -- closes an open-redirect-shaped envelope-tampering
// path (a malicious/corrupted `returnPath` could otherwise send `goto` to an
// absolute URL or an unrelated route).
export function isAllowedDestination(returnPath: string): boolean {
  const normalized = normalizeReturnPath(returnPath);
  if (!normalized.startsWith('/')) return false;
  return ALLOWED_DESTINATIONS.some(
    (d) => normalized === d || normalized.startsWith(`${d}?`) || normalized.startsWith(`${d}/`)
  );
}

function safeSessionStorage(): Storage | null {
  try {
    return typeof sessionStorage === 'undefined' ? null : sessionStorage;
  } catch {
    // sessionStorage access can throw (disabled storage, some private-mode
    // configurations) -- resume is best-effort input preservation, never a
    // hard requirement for the write itself.
    return null;
  }
}

// Persists a draft, stamping `v` (schema version, always the CURRENT
// RESUME_VERSION -- only peekResume ever reads an older-version envelope)
// and `ts` (persist timestamp) itself. Never throws -- a storage failure
// (including a value JSON.stringify can't serialize, e.g. a bigint anywhere
// in `fields`/`values`) degrades to "no resume available on the /ui/
// landing", not a broken re-auth redirect or write flow.
export function persistResume(draft: ResumeDraft): void {
  const store = safeSessionStorage();
  if (!store) return;
  const envelope = { ...draft, v: RESUME_VERSION, ts: Date.now() } as ResumeEnvelope;
  try {
    store.setItem(RESUME_KEY, JSON.stringify(envelope));
  } catch {
    // Quota / serialization failure -- best-effort, see above.
  }
}

function isNonEmptyStringArray(x: unknown): x is string[] {
  return Array.isArray(x) && x.length > 0 && x.every((v) => typeof v === 'string' && v.length > 0);
}

function isPlainObject(x: unknown): x is Record<string, unknown> {
  return typeof x === 'object' && x !== null && !Array.isArray(x);
}

// Per-kind structural validation. Common fields (`v`, `ts`, `returnPath`)
// are checked once up front; everything else is a `switch` on `kind` so
// each variant's own shape (and its own `v` requirement) is checked in
// isolation -- a malformed envelope of any kind peeks null instead of
// reaching a host typed as valid.
function isValidShape(x: unknown): x is ResumeEnvelope {
  if (!x || typeof x !== 'object') return false;
  const o = x as Record<string, unknown>;
  if (typeof o.v !== 'number' || typeof o.ts !== 'number') return false;
  if (typeof o.returnPath !== 'string') return false;
  switch (o.kind) {
    case 'memory':
    case 'discovery':
      if (o.v !== 1 && o.v !== 2) return false;
      if (o.mode !== 'create' && o.mode !== 'edit') return false;
      if (!(o.recordId === null || typeof o.recordId === 'string')) return false;
      if (!isPlainObject(o.values)) return false;
      return true;
    case 'supersede':
      if (o.v !== 2) return false;
      if (!isNonEmptyStringArray(o.targets)) return false;
      if (!isPlainObject(o.fields)) return false;
      if (typeof o.idempotencyKey !== 'string' || o.idempotencyKey.length === 0) return false;
      return true;
    case 'archive':
      if (o.v !== 2) return false;
      if (o.mode !== 'archive' && o.mode !== 'restore') return false;
      if (!isNonEmptyStringArray(o.ids)) return false;
      return true;
    case 'delete':
      if (o.v !== 2) return false;
      if (typeof o.id !== 'string' || o.id.length === 0) return false;
      return true;
    default:
      return false;
  }
}

// Returns null on bad JSON, a per-kind schema-version mismatch, an expired
// TTL, OR a structurally-invalid shape (Codex round-4 LOW) -- never hands
// the host a malformed object typed as a valid ResumeEnvelope. The TTL
// comparison stays integer-millisecond with no rounding; an envelope aged
// exactly RESUME_TTL_MS still peeks (the boundary is `>`, not `>=`).
export function peekResume(): ResumeEnvelope | null {
  const store = safeSessionStorage();
  if (!store) return null;
  let raw: string | null;
  try {
    raw = store.getItem(RESUME_KEY);
  } catch {
    return null;
  }
  if (!raw) return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!isValidShape(parsed)) return null;
  if (Date.now() - parsed.ts > RESUME_TTL_MS) return null;
  return parsed;
}

export function consumeResume(): void {
  const store = safeSessionStorage();
  if (!store) return;
  try {
    store.removeItem(RESUME_KEY);
  } catch {
    // best-effort, see persistResume.
  }
}

// A thin, mockable seam around the real browser navigation the write forms
// trigger after persistResume(). Kept here (rather than an inline
// `window.location.assign(...)` call at each form's call site) purely so
// browser-mode component tests can intercept the redirect without touching
// `window.location`/`Location.prototype`, whose `assign` method is a
// non-configurable own property on real Location instances in Chromium and
// cannot be `vi.spyOn`'d directly.
export function redirectToLogin(): void {
  window.location.assign('/auth/login');
}
