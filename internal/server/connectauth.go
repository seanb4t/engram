// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/seanb4t/engram/internal/auth"
)

// newConnectSubjectInterceptor returns a unary interceptor that resolves the
// caller identity into a *mcpauth.TokenInfo plus WHICH lane authenticated it
// (auth.Lane, D-07), and stashes both under their engram-owned connect
// context keys for subjectFromConnectContext / laneFromConnectContext.
// resolve abstracts the auth source: NewConnectResolver's composed
// bearer+cookie resolver supplies a real resolver; tests and the anonymous
// (no-issuer) case supply one that returns nil/auth.LaneCookie or similar.
func newConnectSubjectInterceptor(resolve func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error)) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			ti, lane, err := resolve(ctx, req)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			ctx = withConnectTokenInfo(ctx, ti)
			ctx = withConnectLane(ctx, lane)
			return next(ctx, req)
		}
	}
}
