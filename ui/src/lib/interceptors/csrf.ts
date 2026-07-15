import type { Interceptor } from '@connectrpc/connect';

// Cookie/header names MUST match internal/webauth/csrf.go's exported wire-contract
// constants (CSRFCookieName = "engram_csrf", CSRFHeaderName = "X-CSRF-Token") verbatim.
// This interceptor only ECHOES the server-minted cookie value back as a header — it
// never mints or validates the token itself; internal/server/connectcsrf.go is the
// sole authoritative verifier (double-submit check). The cookie is deliberately
// non-HttpOnly so this client-side read is possible (double-submit design).
//
// The token is read fresh from document.cookie on every request and never cached
// across requests, so a server-refreshed cookie value (e.g. minted by a later
// Callback, or read on a retry after retryOnce re-enters this interceptor) is
// always picked up on the next call.
export const attachCsrf: Interceptor = (next) => async (req) => {
  const token = document.cookie
    .split('; ')
    .find((c) => c.startsWith('engram_csrf='))
    ?.split('=')[1];
  if (token) req.header.set('X-CSRF-Token', token);
  return await next(req);
};
