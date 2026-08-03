// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/seanb4t/engram/internal/auth"
)

func TestSubjectFromConnectContext(t *testing.T) {
	// injected authenticated owner-claim value — key present, ti non-nil.
	ctx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "sub-A"}})
	if got, err := subjectFromConnectContext(ctx); err != nil || got.Owner() != "sub-A" {
		t.Errorf("authed: got (%v,%v), want Authenticated(sub-A)", got, err)
	}
	// key present, ti==nil (interceptor ran, auth disabled / anonymous caller) -> anonymous bucket.
	anonCtx := withConnectTokenInfo(context.Background(), nil)
	if got, err := subjectFromConnectContext(anonCtx); err != nil || got.Owner() != "" {
		t.Errorf("anon (key present, nil ti): got (%v,%v), want Anonymous", got, err)
	}
	// key absent (interceptor not installed) -> fail closed with error.
	if got, err := subjectFromConnectContext(context.Background()); err == nil {
		t.Errorf("absent key: expected error (fail-closed), got Subject=%v", got)
	}
}

func TestNewConnectSubjectInterceptor_ErrorPath(t *testing.T) {
	resolveErr := errors.New("auth backend unavailable")

	t.Run("resolver_error_returns_unauthenticated", func(t *testing.T) {
		nextCalled := false
		next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			nextCalled = true
			return nil, nil
		})

		interceptor := newConnectSubjectInterceptor(func(_ context.Context, _ connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
			return nil, auth.LaneUnknown, resolveErr
		})
		handler := interceptor(next)

		_, err := handler(context.Background(), connect.NewRequest(&struct{}{}))
		if err == nil {
			t.Fatal("expected error from interceptor, got nil")
		}
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("expected CodeUnauthenticated, got %v", connect.CodeOf(err))
		}
		if nextCalled {
			t.Error("next handler must NOT be called when resolver returns error")
		}
	})

	t.Run("resolver_success_calls_next_with_key_populated", func(t *testing.T) {
		nextCalled := false
		var capturedCtx context.Context
		next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			nextCalled = true
			capturedCtx = ctx
			return nil, nil
		})

		interceptor := newConnectSubjectInterceptor(func(_ context.Context, _ connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
			return nil, auth.LaneCookie, nil // success: anonymous / no-issuer path
		})
		handler := interceptor(next)

		_, err := handler(context.Background(), connect.NewRequest(&struct{}{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !nextCalled {
			t.Fatal("next handler must be called on resolver success")
		}
		// Key must be populated (ok==true) so subjectFromConnectContext doesn't fail closed.
		_, ok := capturedCtx.Value(connectSubjectKey{}).(*mcpauth.TokenInfo)
		if !ok {
			t.Error("connectSubjectKey not populated in context passed to next")
		}
	})
}
