// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// Lane records WHICH transport credential family authenticated a Connect
// request: the bearer half or the session-cookie half of the composed
// resolver (D-07). It is unrelated to the unexported lane type in chain.go,
// which discriminates bearer SHAPE (JWT vs opaque) inside ChainVerifier
// before a verifier runs. The two types coexist deliberately: lane answers
// "which mechanism family should verify this bearer", Lane answers "which
// mechanism family actually authenticated this Connect request" — one
// operates before verification, the other records the outcome after it.
//
// LaneUnknown is the zero value and is ALWAYS invalid (D-08): an unstamped
// context yields it via laneFromConnectContext's comma-ok fallback, and any
// consumer gating on the lane (e.g. the CSRF exemption) must reject on it
// rather than treat it as a permissive default. A future third lane cannot
// silently regress to this zero value without a caller explicitly declaring
// one of the two named constants below.
type Lane int

const (
	// LaneUnknown is the zero value — deliberately invalid (D-08).
	LaneUnknown Lane = iota
	// LaneBearer marks a request authenticated via a well-formed Authorization: Bearer credential.
	LaneBearer
	// LaneCookie marks a request authenticated via the sealed session cookie.
	LaneCookie
)

// String renders the lane for logs and test failure messages. It never
// appears on the wire.
func (l Lane) String() string {
	switch l {
	case LaneBearer:
		return "bearer"
	case LaneCookie:
		return "cookie"
	default:
		return "unknown"
	}
}

// ExtractBearerCredential reproduces the go-sdk's private verify() header
// parse byte-for-byte (mcpauth auth/auth.go: strings.Fields, exactly two
// fields, a case-insensitive "bearer" scheme, a non-empty credential). It is
// the D-01/D-02 structural discriminator: a well-formed match commits the
// caller to the bearer lane; anything else means "not a bearer credential"
// and the caller falls through to the cookie lane.
//
// Boundary policy (MED-5, D-02 — locked): a header value that DECLARES the
// bearer scheme but supplies a malformed credential (a bare "Bearer", a
// multi-field credential such as "Bearer a b", or a comma-coalesced pair
// such as "Bearer a, Bearer b") is NOT treated as a committed bearer
// attempt. It returns ("", false) exactly like a non-bearer scheme, so the
// composed resolver falls through to the cookie lane. This is safe only in
// this direction: the fallthrough moves toward the MORE restrictive lane —
// such a caller receives LaneCookie provenance and still faces the full
// CSRF check. A stricter reading ("once the scheme is declared, fail
// closed") would reject those callers outright instead of demoting them;
// that reading is deliberately NOT adopted here, because D-02 names the
// permissive-fallthrough rule and because a fallthrough toward a MORE
// restrictive lane cannot itself produce a bypass.
func ExtractBearerCredential(authHeader string) (token string, ok bool) {
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") {
		return "", false
	}
	if fields[1] == "" {
		return "", false
	}
	return fields[1], true
}

// ErrTokenMissingExpiration and ErrTokenExpired are the two sentinel
// denials EnforceExpiry can produce. Their Error() strings are load-bearing
// (MED-8): they must match the go-sdk's own verify() rejections at
// auth/auth.go byte-for-byte ("token missing expiration", "token expired"),
// because the go-sdk's RequireBearerToken returns err.Error() verbatim as
// the MCP 401 body when the verifier reports ErrInvalidToken. See the
// invalidTokenError doc comment below for why these are wrapped instead of
// joined.
var (
	ErrTokenMissingExpiration = errors.New("token missing expiration")
	ErrTokenExpired           = errors.New("token expired")
)

// invalidTokenError is an unexported wrapper that lets EnforceExpiry's
// denials satisfy errors.Is(err, mcpauth.ErrInvalidToken) — required so the
// go-sdk's RequireBearerToken middleware maps the rejection to 401 — while
// keeping Error() equal to exactly the wrapped sentinel's message.
//
// This deliberately departs from the errors.Join(mcpauth.ErrInvalidToken,
// <sentinel>) convention used elsewhere in this package (chain.go,
// static_token.go). A errors.Join here would be wrong: the go-sdk's
// verify() (auth/auth.go) returns err.Error() verbatim as the wire body
// when errors.Is(err, ErrInvalidToken) holds, and it reaches that branch
// BEFORE its own expiry checks — so a joined error would turn today's
// "token expired" MCP response into a doubled "invalid token\ntoken
// expired" body (D-04's planner note forbids a "confusing double-error or
// differently-shaped 401"). Do not "fix" this back to errors.Join; that is
// exactly the regression this wrapper exists to prevent.
type invalidTokenError struct {
	sentinel error
}

func (e *invalidTokenError) Error() string {
	return e.sentinel.Error()
}

func (e *invalidTokenError) Unwrap() []error {
	return []error{mcpauth.ErrInvalidToken, e.sentinel}
}

// EnforceExpiry decorates v so TokenInfo.Expiration is actually enforced
// (D-04): expiry is a property of the verifier, not of any one lane, so
// every present and future caller of the decorated verifier inherits the
// check. This is the direct fix for the gap where Expiration was written by
// ChainVerifier's lanes and read by nothing except
// mcpauth.RequireBearerToken's own private verify() — which the Connect
// lane never goes through.
//
// Clock skew is deliberately ZERO (D-05), matching
// mcpauth.RequireBearerToken's own hard-expiry check and
// internal/webauth/resolver.go's existing zero-skew precedent. Do not add a
// grace window here. internal/auth/static_token.go's 100-year sentinel
// Expiration is what keeps the static-token lane passing this check now
// that expiry is enforced on every lane, not only inside
// RequireBearerToken's verify().
func EnforceExpiry(v mcpauth.TokenVerifier) mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, req *http.Request) (*mcpauth.TokenInfo, error) {
		ti, err := v(ctx, token, req)
		if err != nil {
			return nil, err
		}
		if ti == nil {
			// MED-7: forward (nil, nil) UNCHANGED, before touching any
			// field. Do not dereference and do not synthesize a rejection
			// here — the go-sdk's own verify() already rejects a nil
			// TokenInfo itself, with a 500 "token validation failed"
			// (auth/auth.go). Reshaping that into a 401 here would change
			// the MCP wire response for what is a verifier CONTRACT
			// VIOLATION, not a credential problem. The Connect adapter
			// (verifyBearerCredential) carries its own independent nil
			// guard, so both lanes stay safe without either changing shape.
			return nil, nil
		}
		if ti.Expiration.IsZero() {
			return nil, &invalidTokenError{sentinel: ErrTokenMissingExpiration}
		}
		if ti.Expiration.Before(time.Now()) {
			return nil, &invalidTokenError{sentinel: ErrTokenExpired}
		}
		return ti, nil
	}
}
