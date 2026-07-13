<!-- markdownlint-disable MD013 -->

# Stateless sliding-expiry session re-seal

**Date:** 2026-07-13
**Status:** Accepted
**Decision:** engram-slr8
**Deciders:** Sean

## Context

The read-only Connect lane hard-expires the sealed `{sub, expiry}` session cookie at a fixed 12h TTL (engram-8q3), with no per-request refresh (engram-1xv). The write lane (Phases 15-17) now needs a session that survives a long, actively-used working session without dropping an in-flight write partway through — but engram-u9v's foundational decision forbids introducing any server-side session store. This ADR resolves how to keep a session alive across a long working session while staying strictly stateless.

## Decision

Under statelessness, "rotation" means a sliding-expiry re-seal, not a token refresh and not server-side revocation. On an authenticated Connect request (read or write) whose remaining session lifetime has crossed a documented re-seal threshold (`resealThreshold = sessionTTL / 2`, i.e. 6h remaining on the 12h `sessionTTL`, with a bounded `resealSkew = 60s` budget applied only to that threshold comparison), the server re-seals the `{owner, expiry}` AES-GCM session cookie with a fresh absolute `nowUTC().Add(sessionTTL)` expiry, and refreshes the readable `engram_csrf` cookie's `Max-Age` using the same `HMAC(k_csrf, Owner)` double-submit value it already carried (Phase 16). This introduces zero server-side state: no session table, no token store, no per-request IdP call. It is explicitly NOT a token store and NOT server-side revocation.

This amends engram-u9v's original "refreshes the access token (if expired) per request" clause: engram-8q3 already dropped OIDC tokens from the cookie entirely, and this ADR further amends the framing from "refresh per request" to "re-seal the expiry once a threshold is crossed" — a bounded-frequency, expiry-only re-seal, never a token operation.

## Rationale

- Keeps write-capable sessions alive across long working sessions without hard-dropping an in-flight write mid-session.
- Preserves the stateless-cookie posture (engram-u9v) exactly — no new server-side state, no new operational dependency, no new secret or config variable.
- The new expiry is always an absolute `nowUTC().Add(sessionTTL)`, never a delta off the old expiry, so concurrent re-seals of the same near-expiry cookie are forward-monotonic by construction: every candidate expiry is `now + sessionTTL` with each `now` within milliseconds of the others, so no re-seal race can silently shorten a session.
- Re-issuing both cookies together prevents a subtle failure mode: if only the session cookie slid forward while `engram_csrf`'s original 12h `Max-Age` stayed fixed, the CSRF cookie would eventually lapse out from under a still-live session and silently break writes.

## Alternatives Considered

**Server-side session store / true revocation** — would give instant revocation, but reintroduces the operational dependency (a session table, Redis, or equivalent) that engram-u9v and engram-8q3 deliberately avoided for this self-hosted single-binary tool. This is a legitimate future direction but needs its own ADR and is not this milestone. Deferred.

**Re-seal on every response** — the simplest possible rule ("always re-seal"), but produces needless `Set-Cookie` churn on every single authenticated request. Rejected in favor of the ½-TTL threshold, which bounds re-seal frequency to at most roughly once per 6h of continuous activity while still preventing any session from hard-dropping mid-work.

## Consequences

Positive: a long-running, write-capable session no longer hard-expires mid-work purely from elapsed wall-clock time; the mechanism introduces no new server-side state and no new operator-facing configuration variable.

Negative (accepted risk — state prominently): a stolen sealed session cookie is valid for up to a full `sessionTTL` from the moment it is stolen, and because sliding re-seal *extends* the window on every request that crosses the threshold while the cookie is actively used, an actively-abused stolen cookie never expires on its own. This is a genuine, deliberate trade-off, not an oversight. The ONLY kill-switch is operator-triggered rotation of `ENGRAM_UI_COOKIE_KEY` (`ui.cookie_key`, `internal/config/registry.go:56`), which invalidates every sealed cookie at once by changing the AES-GCM key they were sealed with. This is a detection/response gap, not a preventable one: there is no per-session revocation, only an all-sessions key rotation.

Neutral: the hard-expiry check in `Resolver.Resolve` (`resolver.go:49-51`) stays fail-closed and byte-for-byte strict — it is untouched by this ADR. The bounded `resealSkew` budget applies exclusively to the soft re-seal-threshold comparison (to avoid re-seal thrash from single-node clock jitter right at the 6h boundary); it is never applied to the hard-expiry check, and a session past its absolute expiry is always rejected regardless of skew.

## References

- Amends: engram-u9v
- Governed by (unchanged): engram-8q3
- Revisits: engram-1xv
