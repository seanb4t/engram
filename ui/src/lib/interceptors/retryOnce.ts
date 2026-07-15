import { ConnectError, Code, type Interceptor } from '@connectrpc/connect';

// The two-code retry set below is a CLIENT-SIDE interpretation, not a
// dedicated server signal: there is no "needs rotation" error state
// (internal/webauth/resolver.go:49 rejects an expired session with a plain
// "session expired" error). Unauthenticated/PermissionDenied are the only two
// codes a write can fail with ahead of the handler (session Resolve at
// connectauth.go, CSRF verify at connectcsrf.go both precede next()).
const WRITE_RETRY_CODES = new Set<Code>([Code.Unauthenticated, Code.PermissionDenied]);

// retryOnce performs a SINGLE opportunistic auth-race retry — never a
// "retry through re-seal" and never a rotation-recovery mechanism. A failed
// request cannot itself trigger a server re-seal: newConnectResealInterceptor
// explicitly skips re-sealing when the handler returns an error or a nil
// response (internal/server/connectreseal.go:39-41). The retry therefore
// repairs only a session-cookie freshness race — e.g. a concurrent successful
// read re-sealed the engram_session cookie between the two attempts, so a
// second attempt built on the fresher cookie succeeds. The CSRF-freshness
// half is largely theoretical: engram_csrf is bound to Owner only
// (internal/webauth/csrf.go:38, HMAC(k_csrf, Owner)) and is intentionally
// stable across re-seals — a re-seal refreshes its Max-Age, not its value
// (internal/webauth/reseal.go:41). A truly hard-expired session deterministically
// fails the retry too, and that second failure is terminal (drives D-09's
// inline re-auth in the form layer) — it propagates unchanged.
//
// Retrying is safe from a double-mutation standpoint (T-19-11): both retry
// codes are rejected before the write handler mutates any state (session
// Resolve + CSRF verify run ahead of the handler; business errors never map
// to these two codes per internal/server/connecterror.go), so a retry cannot
// double-create a record. No exponential backoff, no configurable attempt
// count, no generic retry library — exactly one retry (D-08).
export const retryOnce: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (err) {
    const ce = ConnectError.from(err);
    if (!WRITE_RETRY_CODES.has(ce.code)) throw ce;
    return await next(req);
  }
};
