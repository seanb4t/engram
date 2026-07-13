// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
)

// sessionCookieNameForTest mirrors webauth's unexported sessionCookieName
// ("engram_session"). internal/server cannot import internal/webauth's
// unexported identifiers (same layering constraint documented atop
// CSRFCookieName in connectcsrf.go), so the literal is duplicated here purely
// to prove the req.Header()->dummy-request cookie plumbing; its exact value
// is independently pinned by internal/webauth/handlers.go.
const sessionCookieNameForTest = "engram_session"

// TestNewConnectResealInterceptor_FiresOnSuccess proves the D-03 property:
// the interceptor has no procedure allowlist and fires on both a read and a
// write RPC, in contrast to newConnectCSRFInterceptor's write-only gate.
// connect.Spec.Procedure has no exported setter on *connect.Request, so this
// exercises the property structurally instead: the interceptor's source
// (connectreseal.go) never inspects req.Spec() at all — unlike
// newConnectCSRFInterceptor, which gates on csrfWriteProcedures[req.Spec().Procedure]
// — so it cannot distinguish read from write RPCs by construction. Each
// subtest below drives a differently-shaped request message (one read-typed,
// one write-typed) to cover both request shapes the interceptor will see in
// the real chain.
func TestNewConnectResealInterceptor_FiresOnSuccess(t *testing.T) {
	cases := map[string]connect.AnyRequest{
		"read":  connect.NewRequest(&engramv1.ListMemoriesRequest{}),
		"write": connect.NewRequest(&engramv1.StoreMemoryRequest{}),
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			var callCount int
			var gotHeader http.Header
			var gotReq *http.Request
			spy := resealFunc(func(h http.Header, r *http.Request) {
				callCount++
				gotHeader = h
				gotReq = r
			})

			resp := connect.NewResponse(&engramv1.ListMemoriesResponse{})
			next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
				return resp, nil
			})

			interceptor := newConnectResealInterceptor(spy)
			handler := interceptor(next)

			req.Header().Set("Cookie", sessionCookieNameForTest+"=sealed-value")

			gotResp, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotResp != connect.AnyResponse(resp) {
				t.Fatal("interceptor must return the SAME resp from next(), not re-wrap it")
			}
			if callCount != 1 {
				t.Fatalf("reseal called %d times, want exactly 1 (fires on read AND write, no allowlist — D-03)", callCount)
			}
			if gotHeader == nil {
				t.Fatal("reseal must receive resp.Header(), got nil")
			}

			// Passes request cookies: the dummy *http.Request built from
			// req.Header() must expose the original Cookie header.
			c, err := gotReq.Cookie(sessionCookieNameForTest)
			if err != nil {
				t.Fatalf("reseal's *http.Request could not read session cookie: %v", err)
			}
			if c.Value != "sealed-value" {
				t.Fatalf("cookie value = %q, want %q", c.Value, "sealed-value")
			}
		})
	}
}

// TestNewConnectResealInterceptor_SkipsOnError proves D-04: an errored
// response is never re-sealed, and the interceptor returns next's (nil, err)
// unchanged.
func TestNewConnectResealInterceptor_SkipsOnError(t *testing.T) {
	callCount := 0
	spy := resealFunc(func(http.Header, *http.Request) { callCount++ })

	wantErr := connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, wantErr
	})

	interceptor := newConnectResealInterceptor(spy)
	handler := interceptor(next)

	gotResp, gotErr := handler(context.Background(), connect.NewRequest(&engramv1.ListMemoriesRequest{}))
	if callCount != 0 {
		t.Fatalf("reseal called %d times on an errored response, want 0", callCount)
	}
	if gotResp != nil {
		t.Fatalf("expected nil response on error, got %v", gotResp)
	}
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("interceptor must return next's error unchanged, got %v want %v", gotErr, wantErr)
	}
}

// TestNewConnectResealInterceptor_SkipsOnNilResponse guards the
// generated-handler contract (connect-go returns a literal nil AnyResponse
// on some rejections even with a nil error) — the interceptor must not panic
// or call reseal in that case.
func TestNewConnectResealInterceptor_SkipsOnNilResponse(t *testing.T) {
	callCount := 0
	spy := resealFunc(func(http.Header, *http.Request) { callCount++ })

	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, nil
	})

	interceptor := newConnectResealInterceptor(spy)
	handler := interceptor(next)

	gotResp, gotErr := handler(context.Background(), connect.NewRequest(&engramv1.ListMemoriesRequest{}))
	if callCount != 0 {
		t.Fatalf("reseal called %d times on a nil response, want 0", callCount)
	}
	if gotResp != nil || gotErr != nil {
		t.Fatalf("expected (nil, nil) passthrough, got (%v, %v)", gotResp, gotErr)
	}
}

// TestNewConnectResealInterceptor_NilResealIsPassthrough proves
// newConnectResealInterceptor(nil) is a safe permanent no-op, mirroring the
// csrfVerify nil convention used by test call sites that don't exercise
// reseal.
func TestNewConnectResealInterceptor_NilResealIsPassthrough(t *testing.T) {
	resp := connect.NewResponse(&engramv1.ListMemoriesResponse{})
	next := connect.UnaryFunc(func(_ context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		return resp, nil
	})

	interceptor := newConnectResealInterceptor(nil)
	handler := interceptor(next)

	gotResp, err := handler(context.Background(), connect.NewRequest(&engramv1.ListMemoriesRequest{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotResp != connect.AnyResponse(resp) {
		t.Fatal("nil-reseal passthrough must return next's response unchanged")
	}
}
