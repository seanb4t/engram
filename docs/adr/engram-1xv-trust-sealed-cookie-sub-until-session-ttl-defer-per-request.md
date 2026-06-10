<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-1xv; do not edit manually; use `/adr update engram-1xv` -->

# Trust sealed cookie sub until session TTL; defer per-request IdP refresh

**Date:** 2026-06-10
**Status:** Accepted
**Decision:** engram-1xv
**Deciders:** Sean

## Context

engram-u9v (Accepted) chose a stateless encrypted-cookie session and stated the Connect interceptor 'decrypts, verifies, and refreshes the access token (if expired) per request.' This ADR AMENDS that single clause for the v1 read-only lane; engram-u9v's cookie-custody decision (stateless AES-GCM cookie, no server-side store, coarse revocation) is otherwise unchanged and remains Accepted. The v1 cookie resolver (auth-lane plan Task 8) trusts the sealed cookie's sub until the session Expiry field, with no per-request call to the IdP or the go-oidc verifier.

## Decision

v1 trusts the sealed cookie's sub until the session TTL (Expiry) with no per-request IdP verification or access-token refresh. Refresh-token rotation and re-seal on access-token expiry are deferred to a post-v1 refinement. This REFINES (does not supersede) engram-u9v's per-request-refresh clause.

## Rationale

- v1 is read-only; the residual risk of a stale session on a read-only console is bounded and accepted.
- Keeps the resolver a pure local operation (AES-GCM unseal + expiry check) with no per-request network dependency on the IdP.
- The session TTL (12h) already bounds the exposure window, consistent with engram-u9v's 'coarse revocation until expiry' rationale (the per-request-refresh clause was in tension with that anyway).
- Refresh is deferred, not abandoned: the refresh token is still sealed in the cookie for the future write phase.

## Alternatives Considered

**Per-request IdP token verification/refresh (as the engram-u9v clause stated)** — detects revocation within one access-token TTL, but adds a go-oidc/JWKS call to every RPC, couples request latency to issuer availability, and complicates the v1 read-only resolver. Deferred to a post-v1 refinement rather than chosen for v1.

## Consequences

Positive: resolver has no network dependency, so RPC latency is unaffected by IdP availability, and it is fully unit-testable without a live IdP. Negative: IdP-side session/token revocation is not reflected until the engram session cookie expires (up to the 12h TTL); future contributors must not add per-request IdP calls until the refresh refinement is designed. Neutral: the sealed refresh token remains in the cookie for the write phase; the mechanism exists but is not yet wired.
