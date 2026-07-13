// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
)

// resealFunc mirrors webauth.Handler.Reseal's signature: a best-effort,
// void-return refresh of the session (and CSRF) cookie's sliding expiry,
// applied directly to a response http.Header. It never returns an error and
// never gates a request (D-04).
type resealFunc func(http.Header, *http.Request)

// newConnectResealInterceptor returns a unary interceptor that re-seals the
// session cookie (and refreshes the engram_csrf cookie's Max-Age) on every
// successful, fully-authorized Connect response — read AND write (D-03, the
// deliberate opposite of newConnectCSRFInterceptor's write-only allowlist:
// keeping a session alive during active reading is exactly what prevents a
// write, arriving after a long read-heavy session, from hitting a dead
// cookie).
//
// This interceptor is NOT procedure-gated and MUST be appended LAST
// (innermost) in mountConnect's connect.WithInterceptors list, after
// newConnectValidateInterceptor, so it wraps the handler and fires only for
// a request that has already passed subject(401) → CSRF(403) → validate(400)
// and whose handler returned a response (D-04). On any upstream rejection or
// handler error — next returns a nil response or a non-nil error — it skips
// re-sealing entirely and returns next's (resp, err) unchanged: a re-seal
// failure must never convert a handler success into an error, and it must
// never manufacture a response where none exists.
func newConnectResealInterceptor(reseal resealFunc) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err != nil || resp == nil || reseal == nil {
				return resp, err
			}

			// connect.AnyRequest.Header() already returns http.Header; wrap it
			// in a throwaway *http.Request to reuse the stdlib cookie parser
			// inside reseal (the sanctioned way to read a cookie from a
			// Connect interceptor — mirrors connectcsrf.go's dummy-request
			// trick and webauth.Resolver.Resolve).
			dummy := &http.Request{Header: req.Header()}
			reseal(resp.Header(), dummy)

			// Mutate and return the SAME resp from next() — never re-wrap it.
			return resp, nil
		}
	}
}
