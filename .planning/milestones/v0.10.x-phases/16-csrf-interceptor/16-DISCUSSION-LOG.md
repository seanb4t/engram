# Phase 16: CSRF Interceptor - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-11
**Phase:** 16-csrf-interceptor
**Areas discussed:** Failure mode, Anonymous mode
**Areas deferred (locked to recommended default):** Defense topology, Token binding

---

## Gray-area selection

| Area | Discussed | Outcome |
|------|-----------|---------|
| Defense topology | — | Locked to default → D-07 |
| Token binding | — | Locked to default → D-08 |
| Failure mode | ✓ | D-02, D-03, D-04 |
| Anonymous mode | ✓ | D-05, D-06 |

---

## Failure mode

### Chain order — where the token check sits

| Option | Description | Selected |
|--------|-------------|----------|
| After subject, before validate | Mirrors D-10; a CSRF-failing caller never sees field-level InvalidArgument detail | ✓ (via "you decide") |
| After validate (last) | Payload validated first; leaks request-contract detail to CSRF-failing callers | |

**User's choice:** "You decide" → locked to *after subject, before validate* (→ D-02).
**Notes:** Placement after `subject` is forced by SC2 (token bound to resolved Subject); the only real choice was before vs after `validate`, resolved in favor of the D-10 information-posture rationale.

### Error code on token failure

| Option | Description | Selected |
|--------|-------------|----------|
| PermissionDenied | Authenticated but lacks valid CSRF proof → 403-equivalent; no retriable hint | ✓ |
| FailedPrecondition | "Token stale — fetch fresh + retry"; friendlier for console retry, advertises recoverability | |
| Unauthenticated | Treats invalid CSRF like an auth failure; conflates with the passed subject gate | |

**User's choice:** PermissionDenied (→ D-03).

### CrossOriginProtection rejection — raw vs normalized

| Option | Description | Selected |
|--------|-------------|----------|
| Raw HTTP 403 | Stdlib default; only fires for genuine cross-origin browsers; simplest | |
| Normalize via SetDenyHandler | Custom deny handler emits a Connect-shaped 403 so clients parse all failures uniformly | ✓ |

**User's choice:** Normalize via SetDenyHandler (→ D-04).
**Notes:** Locked to a `permission_denied`/403 envelope matching D-03 so both CSRF layers surface one consistent shape.

---

## Anonymous mode

> **Reframed by a codebase finding:** there is no anonymous caller on the Connect
> write lane (OIDC-required web lane; nil resolver ⇒ Connect not mounted;
> resolver rejects empty-owner). `Subject.Owner` is guaranteed non-empty by the
> time CSRF runs. The area reduced to how defensively to lock that invariant.

### Fail-closed on missing/empty Subject

| Option | Description | Selected |
|--------|-------------|----------|
| Independently fail-closed | CSRF interceptor re-asserts non-empty Subject, rejects PermissionDenied if absent; defense-in-depth | ✓ |
| Trust upstream | Rely on subject interceptor 401 + resolver empty-owner rejection | |

**User's choice:** Independently fail-closed (→ D-05).

### Locking the "no anonymous write" invariant

| Option | Description | Selected |
|--------|-------------|----------|
| Add a permanent regression test | Enumerate write RPCs vs cookieless/empty-owner request; assert rejected before handler logic | ✓ |
| Rely on resolver rejection | The resolver's existing empty-owner rejection is the guarantee; no new test | |

**User's choice:** Add a permanent regression test (→ D-06).

---

## Claude's Discretion

- Chain-order placement (D-02) — user delegated ("you decide").
- Exact CSRF cookie/header names, SameSite level, HKDF label, new config keys — see CONTEXT.md `<decisions>` § Claude's Discretion.
- Precise Connect error-envelope bytes for the SetDenyHandler.
- `AddTrustedOrigin`/`AddInsecureBypassPattern` usage (default: none).

## Locked Defaults (deferred areas)

- **D-07 — Defense topology:** CrossOriginProtection wraps the whole top-level server handler; the token interceptor is write-only (gated by the six generated write-procedure constants).
- **D-08 — Token binding:** HMAC over `Owner` only (survives Phase-18 re-seal); `k_csrf` derived from `ui.cookie_key` via `crypto/hkdf` labeled sub-key; non-HttpOnly + Secure + SameSite cookie.

## Deferred Ideas

- Phase 19 console client (token attach + silent retry) — cookie/header contract defined here, no client shipped.
- Phase 18 session sliding re-seal — D-08 Owner-only binding is the forward-compat hook.
- Cross-origin console deployment / trusted-origin allowlist — deferred; strict same-origin default.
- CSRF for the MCP transport — out of scope by construction (non-browser, cookieless, no Origin).
