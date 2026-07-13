// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"net/http"
	"time"
)

// resealThreshold is the ½-TTL soft threshold (D-05): Reseal only re-seals
// once a session's remaining lifetime drops below this. Because a re-seal
// resets remaining lifetime back to the full sessionTTL, this bounds
// Set-Cookie churn to at most ~once per 6h of continuous activity.
const resealThreshold = sessionTTL / 2

// resealSkew is a small, bounded clock-skew budget applied ONLY to the
// Reseal threshold comparison below, to avoid re-seal thrash from single-node
// clock jitter right at the resealThreshold boundary. It is NEVER applied to
// Resolver.Resolve's hard-expiry check (resolver.go:49-51), which stays
// byte-for-byte strict with zero skew tolerance (SC4, D-07).
const resealSkew = 60 * time.Second

// headerOnlyWriter adapts an http.Header to http.ResponseWriter so Reseal can
// reuse Handler.setCookie/setReadableCookie (which take a ResponseWriter and
// call http.SetCookie) without duplicating any cookie-attribute logic. This
// mirrors the read-direction dummy-*http.Request trick already used in
// resolver.go and connectcsrf.go. Write and WriteHeader are no-ops: they
// satisfy the interface but are never called by http.SetCookie, which only
// calls Header().Add.
type headerOnlyWriter struct{ h http.Header }

func (w headerOnlyWriter) Header() http.Header     { return w.h }
func (headerOnlyWriter) Write([]byte) (int, error) { return 0, nil }
func (headerOnlyWriter) WriteHeader(int)           {}

// Reseal is a best-effort, void-return refresh of the session cookie's
// sliding expiry (D-04): it never returns an error and never gates a
// request. It re-parses the session cookie from r, and — once the session's
// remaining lifetime has dropped below resealThreshold+resealSkew — re-seals
// BOTH the session cookie (a fresh, absolute nowUTC().Add(sessionTTL)
// expiry, D-06: never oldExpiry+delta) and the engram_csrf cookie (refreshed
// Max-Age, same HMAC(k_csrf, Owner) value, D-08) directly onto respHeader.
// Any Cookie/Unseal/Seal failure is a silent no-op — a re-seal failure must
// never turn a handler success into an error.
func (h *Handler) Reseal(respHeader http.Header, r *http.Request) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}
	sess, err := h.codec.Unseal(c.Value)
	if err != nil {
		return
	}
	// Defense-in-depth: never re-seal a payload Resolver.Resolve would reject.
	// Reseal is innermost in the interceptor chain and today is only reached
	// after the resolver has already validated the same cookie (resolver.go:
	// 49-66), but mirroring those guards here means Reseal fails closed on
	// its own — independent of interceptor ordering — matching the
	// connectcsrf.go D-05 precedent. In particular this must not resurrect an
	// already-expired session, launder a legacy-version cookie into the
	// current version, or re-issue a CSRF token for an empty owner.
	if sess.V != sessionPayloadVersion || sess.Owner == "" {
		return
	}
	remaining := sess.Expiry.Sub(nowUTC())
	if remaining <= 0 || remaining >= resealThreshold+resealSkew {
		return // expired (hard, zero-skew) or not yet due
	}
	sealed, err := h.codec.Seal(Session{
		Owner:  sess.Owner,
		Expiry: nowUTC().Add(sessionTTL),
	})
	if err != nil {
		return
	}
	h.setCookie(headerOnlyWriter{respHeader}, sessionCookieName, sealed, sessionTTL)
	h.setReadableCookie(headerOnlyWriter{respHeader}, CSRFCookieName, h.signer.Token(sess.Owner), sessionTTL)
}
