<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:engram-u9v; do not edit manually; use `/adr update engram-u9v` -->

# Stateless encrypted-cookie session, no server-side store

**Date:** 2026-06-09
**Status:** Accepted
**Decision:** engram-u9v
**Deciders:** Sean

## Context

After OIDC authorization-code login the BFF must maintain a session carrying the access token, refresh token, and sub. engram is a self-hosted single binary with no external session store. The choice — stateless encrypted cookie vs a server-side session store (in-memory, Redis, or Qdrant-backed) — determines restart behavior, operational complexity, and the revocation model.

## Decision

After OIDC login, engram seals {access, refresh, sub} into an httpOnly + SameSite + AES-GCM encrypted cookie. No server-side session store is used; the Connect interceptor decrypts, verifies, and refreshes the access token (if expired) per request.

## Rationale

- Stateless sessions survive binary restarts — important for a self-hosted tool with no external state dependency.\n- Operational simplicity: no Redis, no session table, no second system to provision.\n- httpOnly + SameSite + short access-token TTL + key rotation adequately mitigates the refresh-token-in-cookie risk for the target environment.\n- Coarse revocation (until expiry) is an accepted trade-off for a single-user/small-team internal console; revisit if multi-user revocation sharpens.

## Alternatives Considered

**Server-side session store (in-memory or external)** — instant revocation and the refresh token never leaves the server, but in-memory loses sessions on restart and an external store (Redis etc.) adds an operational dependency and a second storage system to the self-hosted stack. Rejected.

## Consequences

Positive: zero additional storage dependency (the single binary stays self-contained); sessions survive restarts without loss. Negative: the encrypted refresh token is present in the cookie and revocation is coarse-grained; an operator-provisioned cookie encryption key (MEM_UI_COOKIE_KEY) must be rotated. Neutral: CSRF posture for the write phase (phases 2-3) is deferred; v1 is read-only and not at risk.
