<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-8q3; do not edit manually; use `/adr update engram-8q3` -->

# Session cookie seals only sub+expiry; no OIDC tokens stored client-side

**Date:** 2026-06-10
**Status:** Accepted
**Decision:** engram-8q3
**Deciders:** Sean

## Context

PR #67's review (Octopus finding e9d.15) flagged that the sealed session cookie carried the OIDC access+refresh tokens, while the read-only v1 lane only reads sub+expiry. A refresh token in the browser cookie is a long-lived credential exposed on any cookie leak. This SUPERSEDES engram-1xv (which had the refresh token riding along in the cookie for the future write phase) and AMENDS engram-u9v's '{access, refresh, sub}' payload clause; engram-u9v's broader decision (stateless AES-GCM cookie, no server-side store) is unchanged.

## Decision

The session cookie seals only {sub, expiry}. No OIDC access or refresh token is stored client-side. The cookie's sub is trusted until the session TTL with no per-request IdP call (carried forward from engram-1xv). When the write phase needs to act on the user's behalf, it re-introduces token handling SERVER-SIDE (e.g. a session-keyed token store), never in the cookie.

## Rationale

- The read-only lane never used the tokens, so storing them was gratuitous attack surface (least privilege).
- A leaked cookie no longer yields a long-lived refresh credential — only a sub claim bounded by the session TTL.
- Preserves the stateless-cookie ergonomics (engram-u9v) for the read lane while deferring token custody to the write phase, where it is actually needed.

## Alternatives Considered

**Keep tokens in the cookie with httpOnly+SameSite+short-TTL+key-rotation** (engram-u9v's original mitigation) — accepted initially, reversed here: for a read-only lane the tokens are unused, so those mitigations guard a risk that carries no benefit. Rejected.
**Server-side session store for v1** — instant revocation but reintroduces the operational dependency engram-u9v deliberately avoided; not warranted for the read-only lane. Deferred to the write phase.

## Consequences

Positive: smaller blast radius on cookie leak; simpler Session type ({sub, expiry}); the read lane carries zero token-handling code. Negative: the write phase must design server-side token custody (refresh storage/rotation) rather than reusing the cookie — future work. Neutral: sub-trust-until-TTL and the stateless, no-server-store posture (engram-u9v) are unchanged.
