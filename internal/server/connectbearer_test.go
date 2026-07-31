// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/seanb4t/engram/internal/auth"
)

func TestConnectBearerResolverAuthenticatesWellFormedBearer(t *testing.T) {
	wantTI := &mcpauth.TokenInfo{UserID: "bearer-user"}
	bearerVerify := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return wantTI, nil
	}
	cookieResolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{UserID: "cookie-user"}, nil
	}
	resolver := NewConnectResolver(bearerVerify, cookieResolve)

	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer sometoken")

	ti, lane, err := resolver(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lane != auth.LaneBearer {
		t.Errorf("lane = %v, want LaneBearer", lane)
	}
	if ti != wantTI {
		t.Error("expected the bearer verifier's TokenInfo to be returned")
	}
}

// TestBearerFailureNeverFallsThroughToCookie (D-01): a request carrying both
// a valid session cookie AND a well-formed Bearer credential the verifier
// rejects must be denied outright — the cookie half must never be consulted.
func TestBearerFailureNeverFallsThroughToCookie(t *testing.T) {
	bearerErr := errors.New("bearer rejected")
	bearerVerify := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return nil, bearerErr
	}
	var cookieCalls int
	cookieResolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		cookieCalls++
		return &mcpauth.TokenInfo{}, nil
	}
	resolver := NewConnectResolver(bearerVerify, cookieResolve)

	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer badtoken")
	req.Header().Set("Cookie", "engram_session=validcookievalue")

	_, lane, err := resolver(context.Background(), req)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if lane != auth.LaneUnknown {
		t.Errorf("lane = %v, want LaneUnknown", lane)
	}
	if cookieCalls != 0 {
		t.Errorf("cookie resolver called %d times, want 0 (D-01: bearer failure must never fall through to cookie)", cookieCalls)
	}
}

// TestMalformedAuthorizationFallsThroughToCookieLane (D-02): a non-Bearer
// Authorization scheme, with a valid session cookie present, is resolved by
// the cookie half.
func TestMalformedAuthorizationFallsThroughToCookieLane(t *testing.T) {
	var bearerCalls int
	bearerVerify := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		bearerCalls++
		return &mcpauth.TokenInfo{}, nil
	}
	wantTI := &mcpauth.TokenInfo{UserID: "cookie-user"}
	cookieResolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		return wantTI, nil
	}
	resolver := NewConnectResolver(bearerVerify, cookieResolve)

	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Basic zzz")
	req.Header().Set("Cookie", "engram_session=validcookievalue")

	ti, lane, err := resolver(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lane != auth.LaneCookie {
		t.Errorf("lane = %v, want LaneCookie", lane)
	}
	if ti != wantTI {
		t.Error("expected the cookie resolver's TokenInfo to be returned")
	}
	if bearerCalls != 0 {
		t.Errorf("bearer verifier called %d times, want 0", bearerCalls)
	}
}

// TestMalformedCredentialShapesFallThroughToCookieLane (D-02 boundary,
// MED-5): pins the confused-deputy boundary at the RESOLVER level, not just
// the parser — every malformed shape falls through to the cookie lane
// without ever invoking the bearer verifier.
func TestMalformedCredentialShapesFallThroughToCookieLane(t *testing.T) {
	tests := []struct {
		name  string
		setup func(req connect.AnyRequest)
	}{
		{"bare Bearer", func(req connect.AnyRequest) { req.Header().Set("Authorization", "Bearer") }},
		{"multi-field credential", func(req connect.AnyRequest) { req.Header().Set("Authorization", "Bearer a b") }},
		{"comma-coalesced duplicates", func(req connect.AnyRequest) { req.Header().Set("Authorization", "Bearer a, Bearer b") }},
		{"empty header value", func(req connect.AnyRequest) { req.Header().Set("Authorization", "") }},
		{
			name: "header set twice via Header().Add",
			setup: func(req connect.AnyRequest) {
				// http.Header.Get returns only the FIRST value; the first
				// value here is not a well-formed bearer credential, so the
				// resolver must fall through to the cookie lane without
				// ever combining or re-reading the header (single parse,
				// single decision — MED-4).
				req.Header().Add("Authorization", "Basic first-value")
				req.Header().Add("Authorization", "Bearer second-value")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bearerCalls int
			bearerVerify := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
				bearerCalls++
				return &mcpauth.TokenInfo{}, nil
			}
			wantTI := &mcpauth.TokenInfo{UserID: "cookie-user"}
			cookieResolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
				return wantTI, nil
			}
			resolver := NewConnectResolver(bearerVerify, cookieResolve)

			req := connect.NewRequest(&struct{}{})
			tt.setup(req)

			ti, lane, err := resolver(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lane != auth.LaneCookie {
				t.Errorf("lane = %v, want LaneCookie", lane)
			}
			if ti != wantTI {
				t.Error("expected the cookie resolver's TokenInfo to be returned")
			}
			if bearerCalls != 0 {
				t.Errorf("bearer verifier called %d times, want 0", bearerCalls)
			}
		})
	}
}

// TestConnectBearerRejectsNilTokenInfo (MED-7): a bearer verifier that
// violates its contract by returning (nil, nil) must not panic and must not
// authenticate the request; the cookie half must not be consulted either.
func TestConnectBearerRejectsNilTokenInfo(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("resolver panicked on a (nil, nil) bearer verifier return: %v", r)
		}
	}()
	bearerVerify := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return nil, nil
	}
	var cookieCalls int
	cookieResolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		cookieCalls++
		return &mcpauth.TokenInfo{}, nil
	}
	resolver := NewConnectResolver(bearerVerify, cookieResolve)

	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer sometoken")

	_, lane, err := resolver(context.Background(), req)
	if err == nil {
		t.Fatal("expected a non-nil error")
	}
	if lane != auth.LaneUnknown {
		t.Errorf("lane = %v, want LaneUnknown", lane)
	}
	if cookieCalls != 0 {
		t.Errorf("cookie resolver called %d times, want 0", cookieCalls)
	}
}

// TestCookieOnlyCompositionIgnoresCredentialHeader: with no bearer verifier
// configured, a well-formed Bearer credential is ignored entirely — the
// composed resolver behaves byte-for-byte as today's UI-only deployment.
func TestCookieOnlyCompositionIgnoresCredentialHeader(t *testing.T) {
	wantTI := &mcpauth.TokenInfo{UserID: "cookie-user"}
	cookieResolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		return wantTI, nil
	}
	resolver := NewConnectResolver(nil, cookieResolve)

	req := connect.NewRequest(&struct{}{})
	req.Header().Set("Authorization", "Bearer sometoken")
	req.Header().Set("Cookie", "engram_session=validcookievalue")

	ti, lane, err := resolver(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lane != auth.LaneCookie {
		t.Errorf("lane = %v, want LaneCookie", lane)
	}
	if ti != wantTI {
		t.Error("expected the cookie resolver's TokenInfo to be returned")
	}
}

func TestNewConnectResolverBothHalvesNilReturnsNil(t *testing.T) {
	if NewConnectResolver(nil, nil) != nil {
		t.Error("expected NewConnectResolver(nil, nil) to be nil so mountConnect's resolve==nil guard still decides mounting (D-12)")
	}
}

// TestConnectSubjectInterceptorStampsLane drives a request through the real
// interceptor with a lane-aware stub resolver and asserts a handler-side
// read of laneFromConnectContext observes the stamped lane.
func TestConnectSubjectInterceptorStampsLane(t *testing.T) {
	wantLane := auth.LaneBearer
	resolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		return &mcpauth.TokenInfo{}, wantLane, nil
	}
	var gotLane auth.Lane
	next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		gotLane = laneFromConnectContext(ctx)
		return nil, nil
	})
	handler := newConnectSubjectInterceptor(resolve)(next)

	if _, err := handler(context.Background(), connect.NewRequest(&struct{}{})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLane != wantLane {
		t.Errorf("laneFromConnectContext observed %v, want %v", gotLane, wantLane)
	}
}

func TestLaneFromConnectContextAbsentIsUnknown(t *testing.T) {
	if got := laneFromConnectContext(context.Background()); got != auth.LaneUnknown {
		t.Errorf("laneFromConnectContext(background) = %v, want LaneUnknown", got)
	}
}

// TestConnectLaneIsolatedAcrossConcurrentRequests runs bearer-lane and
// cookie-lane requests concurrently through the subject interceptor under
// -race: each handler invocation must observe only its own request's
// stamped lane, never another concurrent request's.
func TestConnectLaneIsolatedAcrossConcurrentRequests(t *testing.T) {
	bearerResolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		return &mcpauth.TokenInfo{}, auth.LaneBearer, nil
	}
	cookieResolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		return &mcpauth.TokenInfo{}, auth.LaneCookie, nil
	}
	bearerHandlerFactory := newConnectSubjectInterceptor(bearerResolve)
	cookieHandlerFactory := newConnectSubjectInterceptor(cookieResolve)

	const n = 50
	var wg sync.WaitGroup
	errCh := make(chan string, n*2)

	run := func(interceptor connect.UnaryInterceptorFunc, want auth.Lane) {
		defer wg.Done()
		next := connect.UnaryFunc(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			if got := laneFromConnectContext(ctx); got != want {
				errCh <- fmt.Sprintf("got lane %v, want %v", got, want)
			}
			return nil, nil
		})
		handler := interceptor(next)
		if _, err := handler(context.Background(), connect.NewRequest(&struct{}{})); err != nil {
			errCh <- err.Error()
		}
	}

	for range n {
		wg.Add(2)
		go run(bearerHandlerFactory, auth.LaneBearer)
		go run(cookieHandlerFactory, auth.LaneCookie)
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		t.Error(e)
	}
}
