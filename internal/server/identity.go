// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/seanb4t/engram/internal/store"
)

// SubjectFromTokenInfo maps a verified TokenInfo to a store.Subject. It is the
// single sub->Subject mapping shared by every auth lane (the MCP bearer lane via
// subjectFromContext, and the Connect cookie lane via subjectFromConnectContext).
// nil TokenInfo (auth disabled) yields the anonymous bucket; a present token with
// a missing/empty sub fails closed rather than collapsing to anonymous.
func SubjectFromTokenInfo(ti *mcpauth.TokenInfo) (store.Subject, error) {
	if ti == nil {
		return store.Anonymous(), nil
	}
	if sub, ok := ti.Extra["sub"].(string); ok && sub != "" {
		return store.Authenticated(sub), nil
	}
	return nil, fmt.Errorf("validated token missing subject")
}

// connectSubjectKey is engram-owned (NOT the go-sdk's unexported key); the
// Connect interceptor writes the resolved TokenInfo under it and
// subjectFromConnectContext reads it. Tests use withConnectTokenInfo to inject.
type connectSubjectKey struct{}

func withConnectTokenInfo(ctx context.Context, ti *mcpauth.TokenInfo) context.Context {
	return context.WithValue(ctx, connectSubjectKey{}, ti)
}

// subjectFromConnectContext resolves the Subject for a Connect request.
//
// Two distinct cases:
//   - Key present, ti==nil  (interceptor ran, auth disabled / anonymous caller):
//     resolves to the anonymous bucket via SubjectFromTokenInfo(nil). This is the
//     legitimate no-issuer path.
//   - Key absent (interceptor was never installed — a programming error):
//     fails closed with an error rather than silently granting anonymous-bucket access.
func subjectFromConnectContext(ctx context.Context) (store.Subject, error) {
	ti, ok := ctx.Value(connectSubjectKey{}).(*mcpauth.TokenInfo)
	if !ok {
		// Interceptor not installed — a programming error. Fail closed rather
		// than silently granting anonymous-bucket access.
		return nil, fmt.Errorf("connect subject key absent: interceptor not installed")
	}
	return SubjectFromTokenInfo(ti)
}
