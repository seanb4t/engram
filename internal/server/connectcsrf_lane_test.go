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
	"github.com/seanb4t/engram/internal/store"
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

// archiveCSRFDo issues one ArchiveMemory/RestoreMemory-shaped request over a
// bare client (mirroring doCSRFWrite's header assembly) and returns the
// decoded response alongside the error, so the caller can assert on the
// per-id outcome as well as the CSRF rejection code — doCSRFWrite discards
// the response, which TestCSRFCurationWritesRequireDoubleSubmit needs.
func archiveCSRFDo[Req, Resp any](ctx context.Context, fn func(context.Context, *connect.Request[Req]) (*connect.Response[Resp], error), msg *Req, h csrfHeaders) (*Resp, error) {
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
	resp, err := fn(ctx, req)
	if resp == nil {
		return nil, err
	}
	return resp.Msg, err
}

// TestCSRFCurationWritesRequireDoubleSubmit (D-15, SC1, RPC-05) is the
// phase's primary CSRF red-first test for milestone 2026-09-25.01 Phase 3:
// explicit subtests "ArchiveMemory" and "RestoreMemory" (never a range over
// csrfWriteProcedures) over the real interceptor chain and a spy record
// owned by actor-A (pre-archived for the RestoreMemory subtest). Each
// subtest proves: no cookie and no header -> permission_denied; cookie but
// no header -> permission_denied; no token and an EMPTY ids list ->
// permission_denied (proving CSRF runs before validation, since an empty
// list carries no buf.validate rule to reject it first); a matching cookie
// and header -> success with one result reporting the "changed" outcome;
// the SAME valid token again -> success with the "already" outcome (the
// idempotent repeat).
func TestCSRFCurationWritesRequireDoubleSubmit(t *testing.T) {
	const ownerA = "actor-A"
	validToken := csrfTestToken(ownerA)

	t.Run("ArchiveMemory", func(t *testing.T) {
		d, sp := newSpyDeps()
		mux := http.NewServeMux()
		if err := d.mountConnect(mux, csrfStubResolveWithLane(auth.LaneCookie), csrfTestVerify, nil); err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(mux)
		defer srv.Close()
		client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
		ctx := context.Background()

		const id = "d0000000-0000-0000-0000-000000000001"
		sp.records[id] = store.Memory{ID: id, Owner: ownerA}

		do := func(ids []string, h csrfHeaders) (*engramv1.ArchiveMemoryResponse, error) {
			return archiveCSRFDo(ctx, client.ArchiveMemory, &engramv1.ArchiveMemoryRequest{Ids: ids}, h)
		}

		if _, err := do([]string{id}, csrfHeaders{actor: ownerA}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("no cookie, no header: got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
		}
		if _, err := do([]string{id}, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("cookie, no header: got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
		}
		if _, err := do([]string{}, csrfHeaders{actor: ownerA}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("no token, empty ids: got code %v (%v), want PermissionDenied (CSRF must precede validation)", connect.CodeOf(err), err)
		}

		resp, err := do([]string{id}, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken, hasHeader: true, headerValue: validToken})
		if err != nil {
			t.Fatalf("matching cookie+header: got err %v, want success", err)
		}
		if got := resp.GetResults(); len(got) != 1 || got[0].GetOutcome() != engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ARCHIVED {
			t.Errorf("first ArchiveMemory result = %+v, want one ARCHIVED row", got)
		}

		resp2, err := do([]string{id}, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken, hasHeader: true, headerValue: validToken})
		if err != nil {
			t.Fatalf("repeat with the same valid token: got err %v, want success", err)
		}
		if got := resp2.GetResults(); len(got) != 1 || got[0].GetOutcome() != engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ALREADY_ARCHIVED {
			t.Errorf("second ArchiveMemory result = %+v, want one ALREADY_ARCHIVED row", got)
		}
	})

	t.Run("RestoreMemory", func(t *testing.T) {
		d, sp := newSpyDeps()
		mux := http.NewServeMux()
		if err := d.mountConnect(mux, csrfStubResolveWithLane(auth.LaneCookie), csrfTestVerify, nil); err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(mux)
		defer srv.Close()
		client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
		ctx := context.Background()

		const id = "d0000000-0000-0000-0000-000000000002"
		archivedAt := time.Now().UTC()
		sp.records[id] = store.Memory{ID: id, Owner: ownerA, ArchivedAt: &archivedAt}

		do := func(ids []string, h csrfHeaders) (*engramv1.RestoreMemoryResponse, error) {
			return archiveCSRFDo(ctx, client.RestoreMemory, &engramv1.RestoreMemoryRequest{Ids: ids}, h)
		}

		if _, err := do([]string{id}, csrfHeaders{actor: ownerA}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("no cookie, no header: got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
		}
		if _, err := do([]string{id}, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("cookie, no header: got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
		}
		if _, err := do([]string{}, csrfHeaders{actor: ownerA}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("no token, empty ids: got code %v (%v), want PermissionDenied (CSRF must precede validation)", connect.CodeOf(err), err)
		}

		resp, err := do([]string{id}, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken, hasHeader: true, headerValue: validToken})
		if err != nil {
			t.Fatalf("matching cookie+header: got err %v, want success", err)
		}
		if got := resp.GetResults(); len(got) != 1 || got[0].GetOutcome() != engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_RESTORED {
			t.Errorf("first RestoreMemory result = %+v, want one RESTORED row", got)
		}

		resp2, err := do([]string{id}, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken, hasHeader: true, headerValue: validToken})
		if err != nil {
			t.Fatalf("repeat with the same valid token: got err %v, want success", err)
		}
		if got := resp2.GetResults(); len(got) != 1 || got[0].GetOutcome() != engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_NOT_ARCHIVED {
			t.Errorf("second RestoreMemory result = %+v, want one NOT_ARCHIVED row", got)
		}
	})

	// SupersedeMemory/SupersedeMemory_validate_only (plan 03-02, D-08/D-15):
	// validate_only is still a call on this write Procedure, so CSRF applies
	// to it exactly as it does to a real supersede call.
	t.Run("SupersedeMemory", func(t *testing.T) {
		d, sp := newSpyDeps()
		mux := http.NewServeMux()
		if err := d.mountConnect(mux, csrfStubResolveWithLane(auth.LaneCookie), csrfTestVerify, nil); err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(mux)
		defer srv.Close()
		client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
		ctx := context.Background()

		const id = "d0000000-0000-0000-0000-000000000003"
		sp.records[id] = store.Memory{ID: id, Owner: ownerA}

		do := func(supersedes []string, h csrfHeaders) (*engramv1.SupersedeMemoryResponse, error) {
			return archiveCSRFDo(ctx, client.SupersedeMemory, &engramv1.SupersedeMemoryRequest{
				Content: "valid content", Scope: "test:scope", Source: "agent-inferred", Category: "decision",
				Supersedes: supersedes,
			}, h)
		}

		if _, err := do([]string{id}, csrfHeaders{actor: ownerA}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("no cookie, no header: got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
		}
		if _, err := do([]string{id}, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("cookie, no header: got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
		}
		if _, err := do([]string{}, csrfHeaders{actor: ownerA}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("no token, empty supersedes: got code %v (%v), want PermissionDenied (CSRF must precede validation)", connect.CodeOf(err), err)
		}

		resp, err := do([]string{id}, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken, hasHeader: true, headerValue: validToken})
		if err != nil {
			t.Fatalf("matching cookie+header: got err %v, want success", err)
		}
		if resp.GetId() == "" {
			t.Errorf("SupersedeMemory result = %+v, want a non-empty id", resp)
		}
	})

	t.Run("SupersedeMemory_validate_only", func(t *testing.T) {
		d, sp := newSpyDeps()
		mux := http.NewServeMux()
		if err := d.mountConnect(mux, csrfStubResolveWithLane(auth.LaneCookie), csrfTestVerify, nil); err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(mux)
		defer srv.Close()
		client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)
		ctx := context.Background()

		const id = "d0000000-0000-0000-0000-000000000004"
		sp.records[id] = store.Memory{ID: id, Owner: ownerA}

		do := func(supersedes []string, h csrfHeaders) (*engramv1.SupersedeMemoryResponse, error) {
			return archiveCSRFDo(ctx, client.SupersedeMemory, &engramv1.SupersedeMemoryRequest{
				Content: "valid content", Scope: "test:scope", Source: "agent-inferred", Category: "decision",
				Supersedes: supersedes, ValidateOnly: true,
			}, h)
		}

		if _, err := do([]string{id}, csrfHeaders{actor: ownerA}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("no cookie, no header: got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
		}
		if _, err := do([]string{id}, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("cookie, no header: got code %v (%v), want PermissionDenied", connect.CodeOf(err), err)
		}
		if _, err := do([]string{}, csrfHeaders{actor: ownerA}); connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("no token, empty supersedes: got code %v (%v), want PermissionDenied (CSRF must precede validation)", connect.CodeOf(err), err)
		}

		resp, err := do([]string{id}, csrfHeaders{actor: ownerA, hasCookie: true, cookieValue: validToken, hasHeader: true, headerValue: validToken})
		if err != nil {
			t.Fatalf("matching cookie+header: got err %v, want success", err)
		}
		if !resp.GetValidated() || len(resp.GetSupersedes()) != 1 || resp.GetSupersedes()[0] != id {
			t.Errorf("validate_only result = %+v, want validated true and supersedes [%s]", resp, id)
		}
	})
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
