// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/auth"
)

// csrfStubResolveWithLane is the unit-shaped lane-aware stub-resolver
// factory: it reproduces csrfStubResolve's owner-from-X-Test-Actor behavior
// but returns the caller-supplied lane, so a test can drive the CSRF
// interceptor's switch directly without going through a real bearer/cookie
// composition.
func csrfStubResolveWithLane(lane auth.Lane) func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
	return func(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		actor := req.Header().Get("X-Test-Actor")
		if actor == "" {
			return nil, auth.LaneUnknown, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
		}
		return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": actor}}, lane, nil
	}
}

// TestBearerLaneExemptFromCSRF is the phase's genuine red-first test
// (REVIEWS.md MED-6): a write RPC whose resolver stamps auth.LaneBearer,
// with NO CSRF cookie and NO CSRF header, succeeds. Against pre-
// implementation connectcsrf.go (no lane switch), this fails at the
// existing subject re-check / double-submit gate. Observed --- FAIL before
// the exemption branch existed; this is the primary red-green evidence for
// Task 2, recorded verbatim in the SUMMARY.
func TestBearerLaneExemptFromCSRF(t *testing.T) {
	d, _ := newSpyDeps()
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, csrfStubResolveWithLane(auth.LaneBearer), csrfTestVerify, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	call := csrfWriteCases(timestamppb.New(time.Now().Add(time.Hour)))[0].call // StoreMemory
	// No CSRF cookie, no CSRF header: a bearer-lane caller carries none by
	// design.
	if err := call(ctx, client, csrfHeaders{actor: "actor-A"}); err != nil {
		t.Fatalf("bearer-lane write with no CSRF material: got err %v, want success", err)
	}
}

// TestCSRFCookieCallerCannotSelfDeclareBearerLane (D-02/D-08, end-to-end)
// mounts the REAL composed resolver — a bearer verifier that rejects
// everything and a cookie half that authenticates X-Test-Actor — and issues
// a write carrying a valid session identity, a valid CSRF cookie, a garbage
// credential header value that is NOT a well-formed bearer credential (so
// D-02 routes it to the cookie lane), and NO X-CSRF-Token header.
// Attaching the garbage credential header must buy no exemption.
func TestCSRFCookieCallerCannotSelfDeclareBearerLane(t *testing.T) {
	d, _ := newSpyDeps()
	bearerVerify := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return nil, errors.New("bearer always rejected")
	}
	var sawAuthHeader bool
	cookieResolve := func(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		if req.Header().Get("Authorization") != "" {
			sawAuthHeader = true
		}
		actor := req.Header().Get("X-Test-Actor")
		if actor == "" {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
		}
		return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": actor}}, nil
	}
	resolver := NewConnectResolver(bearerVerify, cookieResolve)

	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolver, csrfTestVerify, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	call := csrfWriteCases(timestamppb.New(time.Now().Add(time.Hour)))[0].call // StoreMemory

	const ownerA = "actor-A"
	validCookie := csrfTestToken(ownerA)
	err := call(ctx, client, csrfHeaders{
		actor:       ownerA,
		hasCookie:   true,
		cookieValue: validCookie,
		// hasHeader deliberately false: NO X-CSRF-Token.
		authorization: "garbage-not-a-well-formed-bearer-credential",
	})
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
	}
	if !sawAuthHeader {
		t.Fatal("test bug: request did not actually carry the Authorization header — the attack input was never sent")
	}
}

// TestCSRFFailedBearerNeverFallsThroughToExemption (D-01, end-to-end,
// permanent negative): the same real composed resolver, but the request
// carries a WELL-FORMED bearer credential the stub verifier rejects, plus a
// valid session cookie and valid CSRF material. It must fail with
// CodeUnauthenticated at the subject interceptor — never reaching the CSRF
// layer, and never succeeding as a cookie-authenticated write.
func TestCSRFFailedBearerNeverFallsThroughToExemption(t *testing.T) {
	d, _ := newSpyDeps()
	bearerVerify := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return nil, errors.New("bearer always rejected")
	}
	cookieResolve := func(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		actor := req.Header().Get("X-Test-Actor")
		if actor == "" {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
		}
		return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": actor}}, nil
	}
	resolver := NewConnectResolver(bearerVerify, cookieResolve)

	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolver, csrfTestVerify, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	call := csrfWriteCases(timestamppb.New(time.Now().Add(time.Hour)))[0].call // StoreMemory

	const ownerA = "actor-A"
	validToken := csrfTestToken(ownerA)
	err := call(ctx, client, csrfHeaders{
		actor:         ownerA,
		hasCookie:     true,
		cookieValue:   validToken,
		hasHeader:     true,
		headerValue:   validToken,
		authorization: "Bearer well-formed-but-rejected",
	})
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("got code %v (%v), want Unauthenticated", connect.CodeOf(err), err)
	}
}

// TestCSRFCookieCallerOmittingHeaderIsStillRejected (D-08): a LaneCookie
// write with no credential header at all and no X-CSRF-Token ->
// PermissionDenied.
func TestCSRFCookieCallerOmittingHeaderIsStillRejected(t *testing.T) {
	d, _ := newSpyDeps()
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, csrfStubResolveWithLane(auth.LaneCookie), csrfTestVerify, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	call := csrfWriteCases(timestamppb.New(time.Now().Add(time.Hour)))[0].call
	err := call(ctx, client, csrfHeaders{actor: "actor-A"}) // no cookie, no header
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
	}
}

// TestCSRFLaneUnstampedFailsClosed (D-08): a resolver returning
// auth.LaneUnknown on a write RPC, with a fully valid CSRF cookie AND
// matching header, is still rejected — proving no CSRF check was attempted
// and the rejection came from the lane arm.
func TestCSRFLaneUnstampedFailsClosed(t *testing.T) {
	d, _ := newSpyDeps()
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, csrfStubResolveWithLane(auth.LaneUnknown), csrfTestVerify, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	const ownerA = "actor-A"
	validToken := csrfTestToken(ownerA)
	call := csrfWriteCases(timestamppb.New(time.Now().Add(time.Hour)))[0].call
	err := call(ctx, client, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken, hasHeader: true, headerValue: validToken})
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
	}
}

// TestCSRFReadProceduresUnaffectedByLane: a read RPC with auth.LaneUnknown
// and no CSRF material succeeds, because the write-procedure gate still
// short-circuits first.
func TestCSRFReadProceduresUnaffectedByLane(t *testing.T) {
	d := testDeps(t)
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, csrfStubResolveWithLane(auth.LaneUnknown), csrfTestVerify, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	req := connect.NewRequest(&engramv1.ListScopesRequest{})
	req.Header().Set("X-Test-Actor", "actor-A")
	if _, err := client.ListScopes(ctx, req); err != nil {
		t.Fatalf("read RPC with unstamped lane: got err %v, want success", err)
	}
}

// TestCSRFCookieLaneStillEnforcesDoubleSubmit: a request stamped
// auth.LaneCookie with a matching CSRF cookie and header succeeds — the
// positive control that the cookie path is unchanged.
func TestCSRFCookieLaneStillEnforcesDoubleSubmit(t *testing.T) {
	d, _ := newSpyDeps()
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, csrfStubResolveWithLane(auth.LaneCookie), csrfTestVerify, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	const ownerA = "actor-A"
	validToken := csrfTestToken(ownerA)
	call := csrfWriteCases(timestamppb.New(time.Now().Add(time.Hour)))[0].call
	err := call(ctx, client, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken, hasHeader: true, headerValue: validToken})
	if err != nil {
		t.Fatalf("cookie-lane write with matching CSRF cookie+header: got err %v, want success", err)
	}
}
