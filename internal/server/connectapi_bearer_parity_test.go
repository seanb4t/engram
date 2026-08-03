// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/auth"
)

// assertConnectWireMessage asserts the EXACT message a Connect rejection puts
// on the wire (REVIEW.md WR-01). connect.NewError(code, err).Message() is
// err.Error() verbatim, so this is the Connect-lane counterpart to
// TestBearerLaneParityRejectionBodiesMatch's `body == ...` check on the MCP
// side — without it, a future errors.Join / fmt.Errorf("%w: %w") regression
// inside newConnectSubjectInterceptor, NewConnectResolver, or
// verifyBearerCredential would corrupt the Connect body while every existing
// test kept passing.
//
// A failed errors.As is a HARD failure, deliberately. Gating the comparison
// behind `if errors.As(...)` would make the whole assertion vanish the moment
// the error stopped being a *connect.Error — reintroducing on the fix the very
// can't-actually-fail shape WR-01 reports.
func assertConnectWireMessage(t *testing.T, err error, want string) {
	t.Helper()
	var cerr *connect.Error
	if !errors.As(err, &cerr) {
		t.Fatalf("Connect error is not a *connect.Error (%T: %v); cannot assert the wire message", err, err)
	}
	if cerr.Message() != want {
		t.Errorf("Connect wire message = %q, want exactly %q", cerr.Message(), want)
	}
}

// TestStubOIDCVerifierCarriesFutureExpiration is a one-line guard (REVIEWS.md
// MED-9) so a future edit that drops stubOIDCVerifier's Expiration field
// fails here, rather than confusingly inside four parity tests below that
// wrap the fixture in auth.EnforceExpiry.
func TestStubOIDCVerifierCarriesFutureExpiration(t *testing.T) {
	ti, err := stubOIDCVerifier("some-user", "some-owner")(context.Background(), "tok", nil)
	if err != nil {
		t.Fatalf("stubOIDCVerifier: %v", err)
	}
	if ti.Expiration.IsZero() {
		t.Fatal("stubOIDCVerifier's TokenInfo carries a zero Expiration — auth.EnforceExpiry rejects that (D-05), making it unusable under the decorator")
	}
	if !ti.Expiration.After(time.Now()) {
		t.Fatalf("stubOIDCVerifier's Expiration = %v, want a value after time.Now()", ti.Expiration)
	}
}

// bearerParityMCPContext runs a request through the REAL go-sdk
// RequireBearerToken middleware with the given verifier, mirroring
// tools_test.go's authedContext but accepting an arbitrary TokenVerifier
// value (rather than always minting one from a subject string) so this test
// can drive the MCP lane through the SAME verifier value the Connect lane
// uses. Returns the captured context and whether verification succeeded.
func bearerParityMCPContext(verify mcpauth.TokenVerifier, token string) (context.Context, bool) {
	var captured context.Context
	h := mcpauth.RequireBearerToken(verify, nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.Context()
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(httptest.NewRecorder(), req)
	return captured, captured != nil
}

// mountBearerParityConnect mounts a spy-backed Connect server with resolve
// as the ONLY configured lane (bearer, cookieResolve nil) — mirroring how
// serve.go composes NewConnectResolver — and returns a client plus the spy
// store, so the stored record is inspectable. Bearer-lane write RPCs are
// CSRF-exempt by construction (D-08), so csrfVerify is nil.
func mountBearerParityConnect(t *testing.T, bearerVerify mcpauth.TokenVerifier) (engramv1connect.EngramServiceClient, *spyStore) {
	t.Helper()
	d, sp := newSpyDeps()
	resolve := NewConnectResolver(bearerVerify, nil)
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolve, nil, nil); err != nil {
		t.Fatalf("mountConnect: %v", err)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL), sp
}

// bearerParityStoreMemory drives one StoreMemory write through the given
// client with the given bearer token and returns the connect error (if any)
// and, on success, the stored record.
func bearerParityStoreMemory(t *testing.T, client engramv1connect.EngramServiceClient, token, scope string) (*engramv1.StoreMemoryResponse, error) {
	t.Helper()
	req := connect.NewRequest(&engramv1.StoreMemoryRequest{
		Content: "bearer parity content", Scope: scope,
		Category: "gotcha", Source: "user-said",
	})
	req.Header().Set("Authorization", "Bearer "+token)
	resp, err := client.StoreMemory(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

// TestBearerLaneParity proves REQ-connect-bearer-identity's core property:
// the SAME bearer token, verified by the SAME verifier value (wrapped ONCE
// in auth.EnforceExpiry and passed to both lanes — never constructed
// twice), resolves to the identical Actor and owner on the MCP tool path and
// on a Connect write RPC. It also settles RESEARCH.md Assumption A1 (see the
// plan SUMMARY for the disposition).
func TestBearerLaneParity(t *testing.T) {
	const (
		userID = "bearer-parity-user@example.com"
		owner  = "bearer-parity-user@example.com"
		token  = "bearer-parity-token"
	)
	verify := auth.EnforceExpiry(stubOIDCVerifier(userID, owner))

	// MCP lane.
	dMCP, spMCP := newSpyDeps()
	mcpCtx, ok := bearerParityMCPContext(verify, token)
	if !ok {
		t.Fatal("MCP lane: bearer verification failed")
	}
	mcpCaller := callerFor(mcpCtx, t)
	mcpID, _, mcpErr := dMCP.storeMemory(mcpCtx, mcpCaller, storeArgs{
		Content: "bearer parity content", Scope: "parity:project:bearer",
		Category: "gotcha", Source: "user-said",
	})
	if mcpErr != nil {
		t.Fatalf("MCP lane storeMemory: %v", mcpErr)
	}
	mcpRec, ok := spMCP.records[mcpID]
	if !ok {
		t.Fatalf("MCP lane: record %s missing from spy", mcpID)
	}

	// Connect lane — the SAME verify value, never a second construction.
	client, spConn := mountBearerParityConnect(t, verify)
	connResp, connErr := bearerParityStoreMemory(t, client, token, "parity:project:bearer")
	if connErr != nil {
		t.Fatalf("Connect lane StoreMemory: %v", connErr)
	}
	connRec, ok := spConn.records[connResp.Id]
	if !ok {
		t.Fatalf("Connect lane: record %s missing from spy", connResp.Id)
	}

	if mcpRec.Actor != connRec.Actor {
		t.Errorf("Actor mismatch: mcp=%q connect=%q, want identical (same token, same verifier value)", mcpRec.Actor, connRec.Actor)
	}
	if mcpRec.Owner != connRec.Owner {
		t.Errorf("Owner mismatch: mcp=%q connect=%q, want identical (same token, same verifier value)", mcpRec.Owner, connRec.Owner)
	}
	if mcpRec.Actor == "" || mcpRec.Owner == "" {
		t.Fatalf("expected non-empty Actor/Owner on the MCP lane, got Actor=%q Owner=%q", mcpRec.Actor, mcpRec.Owner)
	}
}

// TestBearerLaneParityActorFallback proves the D-07/landmine-3 fallback
// (ti.UserID empty -> caller.Actor falls back to the resolved owner) applies
// identically on both lanes when driven by the SAME verifier value.
func TestBearerLaneParityActorFallback(t *testing.T) {
	const (
		owner = "bearer-parity-fallback-owner"
		token = "bearer-parity-fallback-token"
	)
	verify := auth.EnforceExpiry(stubOIDCVerifier("", owner)) // empty UserID, non-empty owner claim

	dMCP, spMCP := newSpyDeps()
	mcpCtx, ok := bearerParityMCPContext(verify, token)
	if !ok {
		t.Fatal("MCP lane: bearer verification failed")
	}
	mcpCaller := callerFor(mcpCtx, t)
	mcpID, _, mcpErr := dMCP.storeMemory(mcpCtx, mcpCaller, storeArgs{
		Content: "bearer parity fallback content", Scope: "parity:project:bearer-fallback",
		Category: "gotcha", Source: "user-said",
	})
	if mcpErr != nil {
		t.Fatalf("MCP lane storeMemory: %v", mcpErr)
	}
	mcpRec := spMCP.records[mcpID]

	client, spConn := mountBearerParityConnect(t, verify)
	connResp, connErr := bearerParityStoreMemory(t, client, token, "parity:project:bearer-fallback")
	if connErr != nil {
		t.Fatalf("Connect lane StoreMemory: %v", connErr)
	}
	connRec := spConn.records[connResp.Id]

	if mcpRec.Actor != owner {
		t.Errorf("MCP lane Actor = %q, want the owner fallback %q", mcpRec.Actor, owner)
	}
	if connRec.Actor != owner {
		t.Errorf("Connect lane Actor = %q, want the owner fallback %q", connRec.Actor, owner)
	}
	if mcpRec.Actor != connRec.Actor {
		t.Errorf("Actor fallback mismatch: mcp=%q connect=%q, want identical", mcpRec.Actor, connRec.Actor)
	}
}

// TestBearerLaneParityRejectsExpiredOnBothLanes is the SC1 "a token rejected
// on MCP is rejected on Connect" half: the shared expiry-wrapped verifier
// returns a past Expiration with a nil error, and both lanes reject.
func TestBearerLaneParityRejectsExpiredOnBothLanes(t *testing.T) {
	const token = "bearer-parity-expired-token"
	expiredStub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{
			Expiration: time.Now().Add(-time.Minute),
			Extra:      map[string]any{auth.OwnerClaimExtraKey: "bearer-parity-expired-owner"},
		}, nil
	}
	verify := auth.EnforceExpiry(expiredStub)

	if _, ok := bearerParityMCPContext(verify, token); ok {
		t.Fatal("MCP lane: expected bearer verification to fail for an expired token")
	}

	client, _ := mountBearerParityConnect(t, verify)
	_, connErr := bearerParityStoreMemory(t, client, token, "parity:project:bearer-expired")
	if connErr == nil {
		t.Fatal("Connect lane: expected an error for an expired token")
	}
	if connect.CodeOf(connErr) != connect.CodeUnauthenticated {
		t.Errorf("Connect lane code = %v, want CodeUnauthenticated", connect.CodeOf(connErr))
	}
	assertConnectWireMessage(t, connErr, "token expired")
}

// TestBearerLaneParityRejectsZeroExpirationOnBothLanes is D-05's cross-lane
// agreement property for the zero-Expiration case. It deliberately builds
// its OWN stub (never stubOIDCVerifier) so this is the one case that
// supplies a zero expiration on purpose, rather than relying on
// stubOIDCVerifier's former accidental zero value (REVIEWS.md MED-9).
func TestBearerLaneParityRejectsZeroExpirationOnBothLanes(t *testing.T) {
	const token = "bearer-parity-zero-exp-token"
	zeroExpStub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{
			Extra: map[string]any{auth.OwnerClaimExtraKey: "bearer-parity-zero-exp-owner"},
		}, nil // zero-value Expiration, nil error — the case this test exists for.
	}
	verify := auth.EnforceExpiry(zeroExpStub)

	if _, ok := bearerParityMCPContext(verify, token); ok {
		t.Fatal("MCP lane: expected bearer verification to fail for a zero-Expiration token")
	}

	client, _ := mountBearerParityConnect(t, verify)
	_, connErr := bearerParityStoreMemory(t, client, token, "parity:project:bearer-zero-exp")
	if connErr == nil {
		t.Fatal("Connect lane: expected an error for a zero-Expiration token")
	}
	if connect.CodeOf(connErr) != connect.CodeUnauthenticated {
		t.Errorf("Connect lane code = %v, want CodeUnauthenticated", connect.CodeOf(connErr))
	}
	assertConnectWireMessage(t, connErr, "token missing expiration")
}

// TestBearerLaneParityRejectionBodiesMatch (REVIEWS.md MED-8): the MCP
// lane's 401 response body is exactly what the go-sdk's own verify()
// emitted before auth.EnforceExpiry existed — "token expired" for the
// past-Expiration case, "token missing expiration" for the zero case.
// Asserted with == on the trimmed body: a Contains assertion would pass on
// a doubled "invalid token\ntoken expired" message and is not acceptable
// here (D-04's planner note forbids that shape).
func TestBearerLaneParityRejectionBodiesMatch(t *testing.T) {
	drive := func(t *testing.T, verify mcpauth.TokenVerifier, token string) (int, string) {
		t.Helper()
		h := mcpauth.RequireBearerToken(verify, nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("handler must not be reached for a rejected token")
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, strings.TrimSpace(rec.Body.String())
	}

	t.Run("expired", func(t *testing.T) {
		expiredStub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
			return &mcpauth.TokenInfo{
				Expiration: time.Now().Add(-time.Minute),
				Extra:      map[string]any{auth.OwnerClaimExtraKey: "bearer-parity-body-owner"},
			}, nil
		}
		code, body := drive(t, auth.EnforceExpiry(expiredStub), "bearer-parity-body-expired-token")
		if code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", code, http.StatusUnauthorized)
		}
		if body != "token expired" {
			t.Errorf("body = %q, want exactly %q", body, "token expired")
		}
	})

	t.Run("zero expiration", func(t *testing.T) {
		zeroExpStub := func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
			return &mcpauth.TokenInfo{
				Extra: map[string]any{auth.OwnerClaimExtraKey: "bearer-parity-body-owner"},
			}, nil
		}
		code, body := drive(t, auth.EnforceExpiry(zeroExpStub), "bearer-parity-body-zero-exp-token")
		if code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", code, http.StatusUnauthorized)
		}
		if body != "token missing expiration" {
			t.Errorf("body = %q, want exactly %q", body, "token missing expiration")
		}
	})
}
