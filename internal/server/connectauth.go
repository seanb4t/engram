// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

// newConnectSubjectInterceptor returns a unary interceptor that resolves the
// caller identity into a *mcpauth.TokenInfo and stashes it under the engram-owned
// connect context key for subjectFromConnectContext. resolve abstracts the auth
// source: the cookie/OIDC lane (later plan) supplies a real resolver; tests and
// the anonymous (no-issuer) case supply one that returns nil.
func newConnectSubjectInterceptor(resolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error)) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			ti, err := resolve(ctx, req)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			return next(withConnectTokenInfo(ctx, ti), req)
		}
	}
}
