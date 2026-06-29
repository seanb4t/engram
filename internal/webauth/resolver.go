// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package webauth

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// Resolver turns the engram session cookie on a Connect request into the
// *mcpauth.TokenInfo the Connect interceptor seam expects. It is the single
// authz entry for the Connect lane (R2): no cookie / expired / tampered → error
// → the interceptor maps it to CodeUnauthenticated. There is no anonymous
// fallthrough — the cookie's verified owner-claim value is the only identity
// this lane grants.
type Resolver struct {
	codec *SessionCodec
}

// NewResolver builds a Resolver over the session codec.
func NewResolver(codec *SessionCodec) *Resolver {
	if codec == nil {
		panic("webauth: NewResolver requires a non-nil SessionCodec")
	}
	return &Resolver{codec: codec}
}

// Resolve matches the connectResolver signature consumed by
// server.newConnectSubjectInterceptor.
func (r *Resolver) Resolve(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
	// connect.AnyRequest.Header() already returns http.Header; wrap it in a
	// throwaway *http.Request to reuse the stdlib cookie parser.
	dummy := &http.Request{Header: req.Header()}
	c, err := dummy.Cookie(sessionCookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie")
	}
	sess, err := r.codec.Unseal(c.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid session cookie")
	}
	if sess.Expiry.IsZero() || nowUTC().After(sess.Expiry) {
		return nil, fmt.Errorf("session expired")
	}
	if sess.Owner == "" {
		return nil, fmt.Errorf("session has empty owner")
	}
	return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": sess.Owner}}, nil
}
