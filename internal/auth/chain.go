// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// Sentinel causes joined into mcpauth.ErrInvalidToken on the chain's
// deny-by-default paths (D-04). None of these ever embed the raw bearer
// value, matching the no-leak discipline already used throughout this
// package.
var (
	errUnrecognizedBearerShape = errors.New("unrecognized bearer shape")
	errOIDCLaneNotConfigured   = errors.New("OIDC lane not configured")
	errStaticLaneNotConfigured = errors.New("static-token lane not configured")
)

// lane is the outcome of the D-04 structural discriminator: which verifier
// family owns a given bearer. It exists (rather than branching directly on
// looksLikeJWT) so chainVerifier's routing switch has an explicit
// deny-by-default arm for any bearer shape neither branch claims.
type lane int

const (
	laneOIDC lane = iota
	laneStatic
	laneUnrecognized
)

// looksLikeJWT is the D-04 structural discriminator: a cheap segment-count
// check, never a parse (parsing is the verifier's job). A JWT has exactly
// three base64url segments (header, payload, signature) joined by two dots;
// anything else is treated as an opaque bearer.
func looksLikeJWT(token string) bool {
	return strings.Count(token, ".") == 2
}

// discriminate routes a bearer to a lane by shape alone, before any verifier
// runs. An empty token is not JWT-shaped and falls to laneStatic, which
// denies when no static verifier is configured — still deny-by-default, just
// via the nil-mechanism guard rather than a distinct "empty" case.
func discriminate(token string) lane {
	if looksLikeJWT(token) {
		return laneOIDC
	}
	if token == "" {
		return laneUnrecognized
	}
	return laneStatic
}

// chainVerifier composes up to three mcpauth.TokenVerifier lanes into the
// single verifier withAuth wraps in place of the lone human verifier today
// (D-01). Each bearer is routed by shape BEFORE any verifier runs (D-04): a
// JWT-shaped bearer tries oidcHuman then, on failure, oidcService, in that
// order (D-02); an opaque bearer goes to static only. The two mechanism
// families never both run on one bearer (anti-Pitfall-9) — this is never
// "try all three, take the first success."
//
// Any of the three lane verifiers may be nil when its mechanism is
// unconfigured (D-03 independent enablement): a routed nil verifier denies
// with errors.Join(mcpauth.ErrInvalidToken, ...) rather than panicking. A
// bearer matching no structural shape (currently only the empty string)
// denies the same way. All three lanes are stateless closures, so the
// returned verifier is safe for concurrent use and deterministic across
// repeated calls with the same token.
func chainVerifier(oidcHuman, oidcService, static mcpauth.TokenVerifier) mcpauth.TokenVerifier {
	return func(ctx context.Context, token string, req *http.Request) (*mcpauth.TokenInfo, error) {
		switch discriminate(token) {
		case laneOIDC:
			return verifyOIDCBranch(ctx, oidcHuman, oidcService, token, req)
		case laneStatic:
			if static == nil {
				return nil, errors.Join(mcpauth.ErrInvalidToken, errStaticLaneNotConfigured)
			}
			return static(ctx, token, req)
		default:
			return nil, errors.Join(mcpauth.ErrInvalidToken, errUnrecognizedBearerShape)
		}
	}
}

// verifyOIDCBranch tries oidcHuman first, then oidcService only if the human
// lane errors (D-02 order — the first to succeed wins). A nil lane is
// treated as "not configured" and skipped without being dereferenced; if
// every configured lane fails (or none are configured), the branch denies.
func verifyOIDCBranch(ctx context.Context, oidcHuman, oidcService mcpauth.TokenVerifier, token string, req *http.Request) (*mcpauth.TokenInfo, error) {
	if oidcHuman != nil {
		if info, err := oidcHuman(ctx, token, req); err == nil {
			return info, nil
		}
	}
	if oidcService != nil {
		if info, err := oidcService(ctx, token, req); err == nil {
			return info, nil
		}
	}
	return nil, errors.Join(mcpauth.ErrInvalidToken, errOIDCLaneNotConfigured)
}
