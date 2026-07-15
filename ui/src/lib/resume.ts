// Single typed resume-envelope module (Codex round-3 HIGH): centralizes the
// whole persist/peek/consume/validate lifecycle for the D-09 re-auth resume
// flow in one place instead of splitting parse/consume/delete across the
// write forms and the route host. The forms (MemoryFormSheet/
// DiscoveryFormSheet) call ONLY `persistResume` before navigating to
// `/auth/login`; the route/host (Plan 06) is the SOLE owner of
// `peekResume`/`consumeResume`, and passes the restored values back into the
// form as props (`resumeValues`/`resumeDirtyPaths`) rather than the form
// reading sessionStorage itself. This kills the two-owner deletion race a
// component-scoped mount-restore would have.
//
// This reverses CONTEXT.md's "sessionStorage deferred" note for D-09 — the
// real OIDC redirect (`/auth/login` -> IdP -> `/auth/callback` ->
// `internal/webauth/handlers.go:187` -> `/ui/`) always lands on `/ui/`, which
// has no write host, so a component-scoped in-memory draft never survives
// the round-trip. The envelope is what makes D-09 actually hold across a
// real re-auth, not deferred polish (round-3/round-4 cross-AI review).

const RESUME_VERSION = 1;
const RESUME_TTL_MS = 10 * 60 * 1000; // 10 minutes -- transient re-auth state, not durable.

export const RESUME_KEY = 'engram:resume';

// The FULL stored shape, including the version/timestamp `persistResume`
// stamps itself.
export interface ResumeEnvelope {
  v: number;
  ts: number;
  returnPath: string;
  kind: 'memory' | 'discovery';
  mode: 'create' | 'edit';
  recordId: string | null;
  values: Record<string, unknown>;
  dirtyPaths?: string[];
}

// The caller-supplied shape -- WITHOUT `v`/`ts`, which `persistResume` stamps
// itself (Codex round-4 MEDIUM: every call site omits them, so the type must
// not require them or callers won't compile).
export type ResumeDraft = Omit<ResumeEnvelope, 'v' | 'ts'>;

// SvelteKit's static-adapter base path (svelte.config.js: `paths.base =
// '/ui'`). A raw `window.location.pathname` carries this prefix; stripping
// it here means a later `goto(base + path)` on the `/ui/` landing can never
// double-prefix to `/ui/ui/...`.
const BASE_PREFIX = '/ui';

const ALLOWED_DESTINATIONS = ['/observe', '/search', '/discovery'] as const;

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

// Persists a draft, stamping `v` (schema version) and `ts` (persist
// timestamp) itself. Never throws -- a storage failure degrades to "no
// resume available on the /ui/ landing", not a broken re-auth redirect.
export function persistResume(draft: ResumeDraft): void {
  const store = safeSessionStorage();
  if (!store) return;
  const envelope: ResumeEnvelope = { ...draft, v: RESUME_VERSION, ts: Date.now() };
  try {
    store.setItem(RESUME_KEY, JSON.stringify(envelope));
  } catch {
    // Quota / serialization failure -- best-effort, see above.
  }
}

function isValidShape(x: unknown): x is ResumeEnvelope {
  if (!x || typeof x !== 'object') return false;
  const o = x as Record<string, unknown>;
  if (typeof o.v !== 'number' || typeof o.ts !== 'number') return false;
  if (typeof o.returnPath !== 'string') return false;
  if (o.kind !== 'memory' && o.kind !== 'discovery') return false;
  if (o.mode !== 'create' && o.mode !== 'edit') return false;
  if (!(o.recordId === null || typeof o.recordId === 'string')) return false;
  if (typeof o.values !== 'object' || o.values === null || Array.isArray(o.values)) return false;
  if (o.dirtyPaths !== undefined) {
    if (!Array.isArray(o.dirtyPaths) || !o.dirtyPaths.every((p) => typeof p === 'string')) return false;
  }
  return true;
}

// Returns null on bad JSON, a wrong schema version, an expired TTL, OR a
// structurally-invalid shape (Codex round-4 LOW) -- never hands the host a
// malformed object typed as a valid ResumeEnvelope.
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
  if (parsed.v !== RESUME_VERSION) return null;
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
