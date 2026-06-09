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

// subjectFromConnectContext resolves the Subject for a Connect request. A request
// that reached a handler without the interceptor populating the key is a
// programming error and fails closed (nil Subject -> store default-deny).
func subjectFromConnectContext(ctx context.Context) (store.Subject, error) {
	ti, _ := ctx.Value(connectSubjectKey{}).(*mcpauth.TokenInfo)
	return SubjectFromTokenInfo(ti)
}
