// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
)

// csrfTestKey stands in for the HKDF-derived k_csrf in these tests.
// internal/server cannot import internal/webauth (that would make a
// production package in this layer depend on a package one layer up), so the
// HMAC-over-Owner verify construction is replicated inline here — the exact
// same shape as (*webauth.CSRFSigner).Verify, whose behavior is independently
// pinned by internal/webauth/csrf_test.go (plan 01).
var csrfTestKey = []byte("csrf-interceptor-fixed-test-key")

func csrfTestToken(owner string) string {
	mac := hmac.New(sha256.New, csrfTestKey)
	mac.Write([]byte(owner))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func csrfTestVerify(owner, token string) bool {
	return hmac.Equal([]byte(csrfTestToken(owner)), []byte(token))
}

// csrfStubResolve maps X-Test-Actor to an authenticated owner_claim, mirroring
// the stub resolvers already used by connectapi_negative_test.go and
// connectapi_cookie_test.go. A missing header rejects at the subject
// interceptor (Unauthenticated), never reaching the CSRF layer.
func csrfStubResolve(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
	actor := req.Header().Get("X-Test-Actor")
	if actor == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": actor}}, nil
}

// csrfHeaders configures the actor identity and CSRF cookie/header pair
// independently (unlike connectapi_negative_test.go's callWrite, which always
// pairs them) so missing/mismatched/cross-owner scenarios can be constructed
// directly.
type csrfHeaders struct {
	actor       string
	hasCookie   bool
	cookieValue string
	hasHeader   bool
	headerValue string
}

func doCSRFWrite[Req, Resp any](ctx context.Context, fn func(context.Context, *connect.Request[Req]) (*connect.Response[Resp], error), msg *Req, h csrfHeaders) error {
	req := connect.NewRequest(msg)
	if h.actor != "" {
		req.Header().Set("X-Test-Actor", h.actor)
	}
	if h.hasCookie {
		req.Header().Set("Cookie", CSRFCookieName+"="+h.cookieValue)
	}
	if h.hasHeader {
		req.Header().Set(CSRFHeaderName, h.headerValue)
	}
	_, err := fn(ctx, req)
	return err
}

// csrfWriteRPCCase pairs a write RPC name with a call closure that plugs a
// protovalidate-valid payload for that RPC in, so every CSRF matrix cell
// exercises the CSRF layer specifically and never fails on validation first.
type csrfWriteRPCCase struct {
	name string
	call func(ctx context.Context, c engramv1connect.EngramServiceClient, h csrfHeaders) error
}

// csrfWriteCases enumerates the same six write RPCs as
// connectapi_negative_test.go's writeRPCCase table, with minimal
// protovalidate-valid payloads, parameterized over csrfHeaders instead of a
// bare actor string.
func csrfWriteCases(futureNotBefore *timestamppb.Timestamp) []csrfWriteRPCCase {
	return []csrfWriteRPCCase{
		{
			name: "StoreMemory",
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient, h csrfHeaders) error {
				return doCSRFWrite(ctx, c.StoreMemory, &engramv1.StoreMemoryRequest{Content: "valid content", Scope: "test:scope", Category: "decision"}, h)
			},
		},
		{
			name: "StoreDiscovery",
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient, h csrfHeaders) error {
				return doCSRFWrite(ctx, c.StoreDiscovery, &engramv1.StoreDiscoveryRequest{
					Content:   "valid content",
					Kind:      "fact",
					Citations: []*engramv1.Citation{{Kind: "url", Ref: "https://example.com"}},
					Scope:     "discovery:test",
				}, h)
			},
		},
		{
			name: "UpdateMemory",
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient, h csrfHeaders) error {
				return doCSRFWrite(ctx, c.UpdateMemory, &engramv1.UpdateMemoryRequest{
					Id:         "some-id",
					Content:    "new content",
					UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"content"}},
				}, h)
			},
		},
		{
			name: "DeleteMemory",
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient, h csrfHeaders) error {
				return doCSRFWrite(ctx, c.DeleteMemory, &engramv1.DeleteMemoryRequest{Id: "some-id"}, h)
			},
		},
		{
			name: "SetVisibility",
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient, h csrfHeaders) error {
				return doCSRFWrite(ctx, c.SetVisibility, &engramv1.SetVisibilityRequest{Id: "some-id", Visibility: engramv1.Visibility_VISIBILITY_SHARED}, h)
			},
		},
		{
			name: "ScheduleMemory",
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient, h csrfHeaders) error {
				return doCSRFWrite(ctx, c.ScheduleMemory, &engramv1.ScheduleMemoryRequest{
					Content:   "valid content",
					Scope:     "test:scope",
					Category:  "decision",
					NotBefore: futureNotBefore,
				}, h)
			},
		},
	}
}

// TestCSRFWriteProcedureAllowlist pins csrfWriteProcedures at the data level
// (SC3 / T-16-08): exactly the six generated write Procedure constants, and
// none of the five read Procedure constants — independent of, and a faster
// backstop than, the full httptest matrices below.
func TestCSRFWriteProcedureAllowlist(t *testing.T) {
	wantWrite := []string{
		engramv1connect.EngramServiceStoreMemoryProcedure,
		engramv1connect.EngramServiceStoreDiscoveryProcedure,
		engramv1connect.EngramServiceUpdateMemoryProcedure,
		engramv1connect.EngramServiceDeleteMemoryProcedure,
		engramv1connect.EngramServiceSetVisibilityProcedure,
		engramv1connect.EngramServiceScheduleMemoryProcedure,
	}
	if got := len(csrfWriteProcedures); got != 6 {
		t.Fatalf("csrfWriteProcedures has %d entries, want exactly 6", got)
	}
	for _, p := range wantWrite {
		if !csrfWriteProcedures[p] {
			t.Errorf("csrfWriteProcedures missing write procedure %q", p)
		}
	}

	readProcedures := []string{
		engramv1connect.EngramServiceListScopesProcedure,
		engramv1connect.EngramServiceListMemoriesProcedure,
		engramv1connect.EngramServiceSearchMemoriesProcedure,
		engramv1connect.EngramServiceGetMemoryProcedure,
		engramv1connect.EngramServiceSearchDiscoveriesProcedure,
	}
	for _, p := range readProcedures {
		if csrfWriteProcedures[p] {
			t.Errorf("csrfWriteProcedures must NOT contain read procedure %q (SC3)", p)
		}
	}
}

// TestNoAnonymousWrite (D-06) drives all six write RPCs, authenticated
// (valid Subject) but with NO engram_csrf cookie and NO X-CSRF-Token header,
// over the real interceptor chain. Every case must be rejected with
// PermissionDenied — and critically NEVER Unimplemented, which would mean
// the CSRF layer let the request fall through to the still-stub handler.
// Permanent CI gate: a future resolver/interceptor-ordering change cannot
// silently reopen an unauthenticated write path without breaking this test.
func TestNoAnonymousWrite(t *testing.T) {
	d := &deps{} // no Qdrant: stubs return before any store access
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, csrfStubResolve, csrfTestVerify); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()
	futureNotBefore := timestamppb.New(time.Now().Add(time.Hour))

	for _, tc := range csrfWriteCases(futureNotBefore) {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(ctx, client, csrfHeaders{actor: "actor-A"})
			if connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Errorf("got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
			}
			if connect.CodeOf(err) == connect.CodeUnimplemented {
				t.Errorf("got Unimplemented — CSRF did not fire before the stub handler (D-06 violation)")
			}
		})
	}
}

// TestConnectCSRFTokenMatrix (SC2) proves the double-submit token is both
// required and validated against the resolved Subject on a write RPC:
// matching cookie+header passes CSRF and reaches the now-wired StoreMemory
// handler, which succeeds against the spy-backed deps (round-5 HIGH,
// Codex+grok — the permanent Phase-16 CSRF gate stays green through the
// wiring, 17-04 Task 2); a missing header, a cookie/header mismatch, and a
// cookie minted for a different owner are all rejected with
// PermissionDenied, before ever reaching the handler.
func TestConnectCSRFTokenMatrix(t *testing.T) {
	// Spy-backed (non-nil store + non-nil embedder, round-5 HIGH Codex+grok):
	// the happy-path cell passes CSRF and reaches the real StoreMemory
	// handler, which needs a working store + embedder.
	d, _ := newSpyDeps()
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, csrfStubResolve, csrfTestVerify); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	call := csrfWriteCases(timestamppb.New(time.Now().Add(time.Hour)))[0].call // StoreMemory

	const ownerA = "actor-A"
	tokenA := csrfTestToken(ownerA)
	tokenOtherOwner := csrfTestToken("actor-B")

	tests := []struct {
		name        string
		h           csrfHeaders
		wantSuccess bool // CSRF passed AND the scripted StoreMemory outcome succeeded
		want        connect.Code
	}{
		{
			// Passed CSRF, reached the now-wired StoreMemory handler, which
			// succeeds against the spy-backed deps. round-6 LOW, Codex: assert
			// err == nil directly — connect.CodeOf(nil) is CodeUnknown, not a
			// success code, so a code-equality check on success would be
			// meaningless.
			name:        "matching cookie and header",
			h:           csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: tokenA, hasHeader: true, headerValue: tokenA},
			wantSuccess: true,
		},
		{
			name: "cookie present, header missing",
			h:    csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: tokenA},
			want: connect.CodePermissionDenied,
		},
		{
			name: "cookie != header",
			h:    csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: tokenA, hasHeader: true, headerValue: tokenA + "-tampered"},
			want: connect.CodePermissionDenied,
		},
		{
			name: "header present, cookie minted for a different owner",
			h:    csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: tokenOtherOwner, hasHeader: true, headerValue: tokenOtherOwner},
			want: connect.CodePermissionDenied,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := call(ctx, client, tc.h)
			if tc.wantSuccess {
				if err != nil {
					t.Errorf("got err %v, want success (CSRF passed, reached the wired StoreMemory handler)", err)
				}
				return
			}
			if connect.CodeOf(err) != tc.want {
				t.Errorf("got code %v (%v), want %v", connect.CodeOf(err), err, tc.want)
			}
		})
	}
}

// TestReadRPCsCSRFExempt (SC3) drives all five read RPCs, authenticated but
// with NO X-CSRF-Token header at all, over the real interceptor chain and
// asserts none is rejected with PermissionDenied — the write-only allowlist
// gate must never fire on a read Procedure. Uses testDeps (real Qdrant) so
// each read actually executes past the CSRF gate instead of risking a nil
// deps.st panic.
func TestReadRPCsCSRFExempt(t *testing.T) {
	d := testDeps(t)
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, csrfStubResolve, csrfTestVerify); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	cases := []struct {
		name      string
		procedure string
		call      func(ctx context.Context, c engramv1connect.EngramServiceClient) error
	}{
		{
			name:      "ListScopes",
			procedure: engramv1connect.EngramServiceListScopesProcedure,
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient) error {
				req := connect.NewRequest(&engramv1.ListScopesRequest{})
				req.Header().Set("X-Test-Actor", "actor-A")
				_, err := c.ListScopes(ctx, req)
				return err
			},
		},
		{
			name:      "ListMemories",
			procedure: engramv1connect.EngramServiceListMemoriesProcedure,
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient) error {
				req := connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: "test:scope"})
				req.Header().Set("X-Test-Actor", "actor-A")
				_, err := c.ListMemories(ctx, req)
				return err
			},
		},
		{
			name:      "SearchMemories",
			procedure: engramv1connect.EngramServiceSearchMemoriesProcedure,
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient) error {
				req := connect.NewRequest(&engramv1.SearchMemoriesRequest{Scope: "test:scope", Query: "test query"})
				req.Header().Set("X-Test-Actor", "actor-A")
				_, err := c.SearchMemories(ctx, req)
				return err
			},
		},
		{
			name:      "GetMemory",
			procedure: engramv1connect.EngramServiceGetMemoryProcedure,
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient) error {
				req := connect.NewRequest(&engramv1.GetMemoryRequest{Id: "00000000-0000-0000-0000-000000000000"})
				req.Header().Set("X-Test-Actor", "actor-A")
				_, err := c.GetMemory(ctx, req)
				return err
			},
		},
		{
			name:      "SearchDiscoveries",
			procedure: engramv1connect.EngramServiceSearchDiscoveriesProcedure,
			call: func(ctx context.Context, c engramv1connect.EngramServiceClient) error {
				req := connect.NewRequest(&engramv1.SearchDiscoveriesRequest{Scope: "discovery:test", Query: "test query"})
				req.Header().Set("X-Test-Actor", "actor-A")
				_, err := c.SearchDiscoveries(ctx, req)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(ctx, client)
			if connect.CodeOf(err) == connect.CodePermissionDenied {
				t.Errorf("%s: got PermissionDenied — CSRF interceptor incorrectly gated a read RPC (SC3 violation): %v", tc.procedure, err)
			}
		})
	}
}

// TestConnectCSRFInterceptor_EmptyOwner (D-05) drives a write RPC through a
// chain whose resolver "succeeds" (no error, so the subject interceptor does
// not reject) but supplies a nil TokenInfo — SubjectFromTokenInfo(nil)
// resolves to the anonymous bucket (Owner()==""). This simulates a future
// interceptor-ordering regression where an anonymous caller's write request
// reaches the CSRF layer, and proves the CSRF interceptor independently
// rejects it (never trusting that the subject gate already ran) even when a
// well-formed CSRF cookie/header pair is presented.
func TestConnectCSRFInterceptor_EmptyOwner(t *testing.T) {
	d := &deps{} // no Qdrant: StoreMemory's stub is never reached
	resolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		return nil, nil
	}
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolve, csrfTestVerify); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	emptyOwnerToken := csrfTestToken("")
	req := connect.NewRequest(&engramv1.StoreMemoryRequest{Content: "valid content", Scope: "test:scope", Category: "decision"})
	req.Header().Set("Cookie", CSRFCookieName+"="+emptyOwnerToken)
	req.Header().Set(CSRFHeaderName, emptyOwnerToken)

	_, err := client.StoreMemory(ctx, req)
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("empty-owner write: got code %v (%v), want PermissionDenied (D-05)", connect.CodeOf(err), err)
	}
}
