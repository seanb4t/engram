// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/seanb4t/engram/internal/auth"
)

// errBearerVerificationFailed is a terse, non-leaking rejection returned
// when the bearer verifier reports success (no error) but hands back a nil
// TokenInfo — a verifier contract violation. It never embeds the presented
// credential (no-leak discipline).
var errBearerVerificationFailed = errors.New("bearer verification failed")

// verifyBearerCredential adapts the transport-agnostic mcpauth.TokenVerifier
// to the Connect lane. It takes the ALREADY-EXTRACTED credential (never a
// connect.AnyRequest) so it performs no header parsing of its own — that is
// what makes "single parse, single decision" true in code, not merely in
// comment (MED-4). It reuses the sanctioned dummy-*http.Request trick
// (internal/webauth/resolver.go) so the verifier can still inspect headers
// if it needs to.
func verifyBearerCredential(ctx context.Context, verify mcpauth.TokenVerifier, token string, hdr http.Header) (*mcpauth.TokenInfo, error) {
	dummy := &http.Request{Header: hdr}
	ti, err := verify(ctx, token, dummy)
	if err != nil {
		return nil, err
	}
	if ti == nil {
		// MED-7: independent nil guard — never dereference, never leak the
		// credential in the rejection.
		return nil, errBearerVerificationFailed
	}
	return ti, nil
}

// NewConnectResolver composes the bearer half and the cookie half of Connect
// identity resolution into the single connectResolver
// mountConnect/newConnectSubjectInterceptor consume, and decides which lane
// authenticated the request (D-07).
//
// Composition happens ONCE, at construction time, branching on which halves
// are configured (only-configured-lanes composition, CONTEXT.md discretion
// call):
//
//   - both nil -> returns a nil func, so mountConnect's existing
//     `resolve == nil` guard still decides mounting (D-12).
//   - bearerVerify nil, cookieResolve non-nil -> every request is delegated
//     to the cookie half, stamped LaneCookie, with NO credential-header
//     inspection at all — a UI-only deployment behaves byte-for-byte as it
//     does today, structurally rather than by test.
//   - bearerVerify non-nil (cookieResolve nil or non-nil) -> the returned
//     closure reads the credential header off req.Header() EXACTLY ONCE,
//     calls auth.ExtractBearerCredential EXACTLY ONCE, and dispatches on the
//     ok it returned. A well-formed match commits exclusively to
//     verifyBearerCredential with the token already in hand (D-01): its
//     error is returned as-is with LaneUnknown, and the cookie half is never
//     called. A malformed/absent/non-bearer credential falls through to the
//     cookie half when one is configured (LaneCookie), or denies with
//     LaneUnknown when it is not.
//
// The lane returned is decided EXCLUSIVELY by which half actually
// succeeded, never by a second read of the request (D-02's load-bearing
// constraint — one code path, one decision, one stamp). Every error path
// returns auth.LaneUnknown.
func NewConnectResolver(bearerVerify mcpauth.TokenVerifier, cookieResolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)) func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
	if bearerVerify == nil && cookieResolve == nil {
		return nil
	}

	if bearerVerify == nil {
		return func(ctx context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
			ti, err := cookieResolve(ctx, req)
			if err != nil {
				return nil, auth.LaneUnknown, err
			}
			return ti, auth.LaneCookie, nil
		}
	}

	return func(ctx context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		token, ok := auth.ExtractBearerCredential(req.Header().Get("Authorization"))
		if ok {
			ti, err := verifyBearerCredential(ctx, bearerVerify, token, req.Header())
			if err != nil {
				return nil, auth.LaneUnknown, err
			}
			return ti, auth.LaneBearer, nil
		}
		if cookieResolve == nil {
			return nil, auth.LaneUnknown, errors.New("no bearer credential and no cookie lane configured")
		}
		ti, err := cookieResolve(ctx, req)
		if err != nil {
			return nil, auth.LaneUnknown, err
		}
		return ti, auth.LaneCookie, nil
	}
}
