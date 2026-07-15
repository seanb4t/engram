---
phase: 19-console-write-ux
reviewed: 2026-07-15T15:26:58Z
depth: standard
files_reviewed: 42
files_reviewed_list:
  - .github/workflows/ci.yaml
  - ui/src/app.css
  - ui/src/app.css.test.ts
  - ui/src/lib/client.test.ts
  - ui/src/lib/client.ts
  - ui/src/lib/components/DeleteConfirmDialog.browser.test.ts
  - ui/src/lib/components/DeleteConfirmDialog.svelte
  - ui/src/lib/components/DiscoveryFormSheet.browser.test.ts
  - ui/src/lib/components/DiscoveryFormSheet.svelte
  - ui/src/lib/components/MemoryDetail.browser.test.ts
  - ui/src/lib/components/MemoryDetail.svelte
  - ui/src/lib/components/MemoryFormSheet.browser.test.ts
  - ui/src/lib/components/MemoryFormSheet.svelte
  - ui/src/lib/components/MemoryList.svelte
  - ui/src/lib/components/MemoryRow.browser.test.ts
  - ui/src/lib/components/MemoryRow.svelte
  - ui/src/lib/components/ShareWarningInline.browser.test.ts
  - ui/src/lib/components/ShareWarningInline.svelte
  - ui/src/lib/components/ui/button/button.svelte
  - ui/src/lib/components/ui/button/button.test.ts
  - ui/src/lib/components/WriteSurfaces.browser.test.ts
  - ui/src/lib/components/WriteSurfaces.svelte
  - ui/src/lib/interceptors/csrf.test.ts
  - ui/src/lib/interceptors/csrf.ts
  - ui/src/lib/interceptors/retryOnce.test.ts
  - ui/src/lib/interceptors/retryOnce.ts
  - ui/src/lib/mutations/discovery.test.ts
  - ui/src/lib/mutations/discovery.ts
  - ui/src/lib/mutations/memory.test.ts
  - ui/src/lib/mutations/memory.ts
  - ui/src/lib/resume.test.ts
  - ui/src/lib/resume.ts
  - ui/src/routes/+page.svelte
  - ui/src/routes/discovery/+page.svelte
  - ui/src/routes/discovery/discovery.browser.test.ts
  - ui/src/routes/observe/+page.svelte
  - ui/src/routes/observe/observe.browser.test.ts
  - ui/src/routes/page.browser.test.ts
  - ui/src/routes/search/+page.svelte
  - ui/src/routes/search/search.browser.test.ts
findings:
  critical: 0
  warning: 1
  info: 4
  total: 5
status: issues_found
---

# Phase 19: Code Review Report

**Reviewed:** 2026-07-15T15:26:58Z
**Depth:** standard
**Files Reviewed:** 42
**Status:** issues_found

## Summary

Reviewed the Phase 19 console write-UX slice: Connect-ES write transport
(`retryOnce` + `attachCsrf` interceptors), TanStack Query mutation hooks for
memory/discovery (create/schedule/update/delete/set-visibility), the
create-as-shared composite, the D-09 re-auth resume envelope, and the Svelte 5
write-surface components/routes.

The code is unusually well-hardened (five cross-AI review rounds), and each of
the five review-hardened invariants I was asked to verify holds against the
actual source:

1. **Write transport composition** — `client.ts:23` composes
   `[retryOnce, attachCsrf]` (retryOnce outer, attachCsrf inner), so a retry
   re-enters `attachCsrf` and re-reads `document.cookie` fresh. `retryOnce.ts`
   performs exactly one retry gated on `Unauthenticated`/`PermissionDenied`. Correct.
2. **Create-as-shared composite** — `shareIfRequested`
   (`memory.ts:125-137`) and `createDiscoveryComposite` (`discovery.ts:75-87`)
   catch *all* secondary `setVisibility` failures (including auth codes) and
   resolve to `created_private`, never rethrow — so the form's D-09 resubmit
   tier is never reached and no duplicate create is possible. Correct.
3. **DeleteConfirmDialog host-authoritative** — the dialog never self-closes on
   Delete; `WriteSurfaces.confirmDelete` clears the target only on success and
   retains it (with the re-auth CTA) on terminal auth failure. Correct.
4. **Resume envelope single owner** — forms only `persistResume`; `/ui/` peeks,
   the destination route consumes via `onresumeapplied → consumeResume`. Correct.
5. **Edit-visibility read-only for shared** — `isEditSharedReadOnly`
   (`MemoryFormSheet.svelte:79,98`) keeps `shared` out of the dirty mask, and
   `buildUpdateMemoryRequest` only adds `shared` when present, so `shared:false`
   is never emitted. Correct.

Security surfaces are sound: `MemoryDetail`'s `{@html}` sink is fed by
`renderMarkdown`, which routes through `marked` (no HTML passthrough) → a tight
DOMPurify allowlist with a scheme-hardening link hook — no XSS. The CSRF
double-submit interceptor only echoes the server-minted, non-HttpOnly cookie.
The resume `returnPath` open-redirect surface is gated by `isAllowedDestination`
(protocol-relative `//evil` and cross-route targets are rejected; all `goto`
targets remain same-origin). No hardcoded secrets, no debug artifacts, no
`as any`, no empty error-swallowing catches (the composite's `catch {}` is
deliberate control flow with a user-visible toast).

Findings below are one UX-robustness gap in the re-auth flow and four minor
maintainability/defensive items.

## Warnings

### WR-01: Inline delete/share re-auth discards route + action context

**File:** `ui/src/lib/components/WriteSurfaces.svelte:188-190,222-224`
**Issue:** On a terminal auth failure of an inline **delete** or **share**, the
re-auth CTA calls `handleDeleteReauth` / `handleShareReauth`, which invoke
`redirectToLogin()` directly **without** persisting any resume envelope. The
form flows (`MemoryFormSheet`/`DiscoveryFormSheet`) instead call `persistResume`
with a `returnPath`, so after the OIDC round-trip (`/auth/callback → /ui/`) the
`/ui/` root routes the user back to their originating filtered route and
reopens the sheet. The delete/share paths persist nothing, so the user lands on
the console home (`/ui/`) — losing their scope/filter/selection context and the
pending action entirely. This is an asymmetry in the phase's central re-auth
UX: two of the four write surfaces recover their location, two do not.
**Fix:** Persist a navigation-only envelope (no mutation auto-replay) before
redirecting, so `/ui/` returns the user to where they were. Reuse the existing
`ResumeDraft` with empty `values`, or add a lightweight returnPath-only path:
```ts
function handleDeleteReauth(): void {
  const returnPath = normalizeReturnPath(window.location.pathname + window.location.search);
  if (isAllowedDestination(returnPath)) {
    persistResume({ returnPath, kind, mode: 'create', recordId: null, values: {} });
  }
  redirectToLogin();
}
```
(Destination routes already no-op `reopenFromResume` on an empty/create
envelope with no restorable draft, so no sheet is spuriously opened — only the
navigation is restored. Confirm the empty-`values` reopen is a true no-op, or
gate the reopen on non-empty values.) If dropping the user at home is the
intended design, document it explicitly so the asymmetry is a decision, not a gap.

## Info

### IN-01: Fragile cookie-value parse in `attachCsrf`

**File:** `ui/src/lib/interceptors/csrf.ts:15-18`
**Issue:** The token is extracted with `...split('=')[1]`, which returns only
the substring up to the *first* `=` after the cookie name. This is currently
safe because `CSRFSigner.Token` uses `base64.RawURLEncoding`
(`internal/webauth/csrf.go:62`), which never emits `=`. But the parse silently
truncates the token if the encoding ever changes to padded base64 or the value
otherwise contains `=`, which would fail every write with a CSRF mismatch that
is hard to diagnose.
**Fix:** Split only on the first `=`:
```ts
const raw = document.cookie.split('; ').find((c) => c.startsWith('engram_csrf='));
const token = raw?.slice('engram_csrf='.length);
```

### IN-02: Hand-written query key duplicates `listMemoriesKey` shape

**File:** `ui/src/routes/+page.svelte:16`
**Issue:** `recentQ` hand-writes `queryKey: ['listMemories', '', [], '', PAGE_LIMIT, 0]`
instead of calling `listMemoriesKey('', [], '', PAGE_LIMIT, 0)`. If the key
shape in `queries.ts:34` ever changes (e.g. field reorder), this key silently
drifts out of sync with the optimistic-cache transforms in `memory.ts`
(`applyToMemoryCaches` reads `key[3]` as the visibility filter), and the root
page's recent list would stop matching invalidations/patches.
**Fix:** Use the shared helper: `queryKey: listMemoriesKey('', [], '', PAGE_LIMIT, 0)`.

### IN-03: Partial optional chaining on repeated-field `.length`

**File:** `ui/src/routes/discovery/+page.svelte:58`, `ui/src/routes/search/+page.svelte:50`
**Issue:** `total={BigInt(discQ.data?.discoveries.length ?? 0)}` (and the search
equivalent) short-circuits on `data` but then reads `.discoveries.length`
unguarded. This is safe *only* because protobuf-es always initializes repeated
fields to `[]`; it is also inconsistent with the sibling `memories={discQ.data?.discoveries ?? []}`
prop on the same element, which treats the field as possibly undefined. The
mixed treatment invites a real `TypeError` if the response shape ever loosens.
**Fix:** Make it consistent: `total={BigInt(discQ.data?.discoveries?.length ?? 0)}`.

### IN-04: Resume `values.citations` applied without element validation

**File:** `ui/src/lib/components/DiscoveryFormSheet.svelte:176`, `ui/src/lib/resume.ts:101-104`
**Issue:** `peekResume`'s `isValidShape` validates the envelope frame but leaves
`values` opaque (`typeof o.values === 'object'`). `DiscoveryFormSheet` then
restores `citations = rv.citations as DiscoveryCitationInput[]` with only an
`Array.isArray` guard — element `kind`/`ref` shape is unchecked before the
values reach `buildStoreDiscoveryRequest`. The blast radius is limited (the
envelope lives in the user's own sessionStorage, not attacker-controlled input,
and `create()` coerces field types), so this is defense-in-depth rather than an
exploitable flaw.
**Fix:** Validate citation elements on restore (drop entries missing a valid
`kind`/string `ref`) so a corrupted envelope can't seed malformed citation rows.

---

_Reviewed: 2026-07-15T15:26:58Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
