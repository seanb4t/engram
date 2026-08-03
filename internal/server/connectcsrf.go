// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"

	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/auth"
)

// CSRFCookieName and CSRFHeaderName are the server-side copies of the wire
// contract minted by webauth.CSRFCookieName / webauth.CSRFHeaderName. The two
// packages cannot import each other (resolver.go precedent: internal/server
// already depends on internal/webauth's Resolve func value, never the
// reverse), so these constants are re-declared here and cross-checked by a
// cmd/engram test (plan 03) that asserts they are byte-for-byte equal.
const (
	CSRFCookieName = "engram_csrf"
	CSRFHeaderName = "X-CSRF-Token"
)

// csrfWriteProcedures is the write-only allowlist (D-07): the CSRF
// interceptor gates on this set instead of every Connect procedure. Keyed on
// the generated Procedure string constants (never a hand-maintained path
// list, Pitfall 3) so a Phase-15 proto regen can't silently drift the
// allowlist away from the actual six write RPCs.
var csrfWriteProcedures = map[string]bool{
	engramv1connect.EngramServiceStoreMemoryProcedure:    true,
	engramv1connect.EngramServiceStoreDiscoveryProcedure: true,
	engramv1connect.EngramServiceUpdateMemoryProcedure:   true,
	engramv1connect.EngramServiceDeleteMemoryProcedure:   true,
	engramv1connect.EngramServiceSetVisibilityProcedure:  true,
	engramv1connect.EngramServiceScheduleMemoryProcedure: true,
}

// newConnectCSRFInterceptor returns a unary interceptor enforcing the
// session-bound double-submit CSRF token on the six write Procedures only
// (D-07); the five read Procedures pass through untouched (SC3). It must run
// AFTER the subject interceptor (D-02: needs the resolved Subject) and
// BEFORE the validate interceptor (D-02: a forged-origin caller learns
// nothing about payload shape). Every rejection maps to CodePermissionDenied
// (D-03) via a fixed, generic message — never err.Error() verbatim — so the
// wire response never hints at which check failed or leaks recoverability
// detail to an attacker.
//
// verify independently re-derives the expected token for an owner (typically
// (*webauth.CSRFSigner).Verify); the interceptor never trusts the resolved
// Subject blindly (D-05): it re-reads the Subject via
// subjectFromConnectContext and fails closed if that errors or the owner is
// empty, even though upstream (subject interceptor + webauth.Resolver)
// already guarantees a non-empty owner. This is defense-in-depth against a
// future interceptor-ordering regression.
func newConnectCSRFInterceptor(verify func(owner, token string) bool) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if !csrfWriteProcedures[req.Spec().Procedure] {
				return next(ctx, req) // SC3: read RPCs untouched.
			}

			// D-08: the exemption decision comes from the stamped lane
			// ALONE — never from Authorization, a cookie, Peer(), the HTTP
			// method, or Content-Type (RESEARCH.md Pitfall 1). A bearer
			// caller carries no ambient cookie by design, so CSRF does not
			// apply to it; the exemption is total for write procedures. A
			// cookie-lane caller falls through to the existing subject
			// re-check and double-submit comparison below, byte-for-byte
			// unchanged. An absent or unrecognized lane (LaneUnknown, or
			// any future value this switch does not yet know about) is
			// rejected outright with NO CSRF check attempted: "treat
			// unknown as cookie" would let a misordered-interceptor bug
			// succeed whenever the caller happens to carry valid CSRF
			// material, so the wiring fault would never surface.
			switch laneFromConnectContext(ctx) {
			case auth.LaneBearer:
				return next(ctx, req)
			case auth.LaneCookie:
				// Fall through to the cookie-lane double-submit check below.
			default:
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: no lane"))
			}

			subj, err := subjectFromConnectContext(ctx)
			if err != nil || subj.Owner() == "" {
				// D-05: independent fail-closed re-check, even though the
				// subject interceptor + resolver already guarantee a
				// non-empty owner by the time a write request reaches here.
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: no subject"))
			}

			// connect.AnyRequest.Header() already returns http.Header; wrap
			// it in a throwaway *http.Request to reuse the stdlib cookie
			// parser (the only sanctioned way to read a cookie from a
			// Connect interceptor — mirrors webauth.Resolver.Resolve).
			dummy := &http.Request{Header: req.Header()}
			c, err := dummy.Cookie(CSRFCookieName)
			if err != nil {
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: no token cookie"))
			}

			header := req.Header().Get(CSRFHeaderName)
			if header == "" || !verify(subj.Owner(), c.Value) || c.Value != header {
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("csrf: token mismatch"))
			}

			return next(ctx, req)
		}
	}
}
