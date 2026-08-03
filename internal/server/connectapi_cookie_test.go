// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/gen/go/engram/v1/engramv1connect"
	"github.com/seanb4t/engram/internal/auth"
	"github.com/seanb4t/engram/internal/store"
)

// TestConnectCookieLaneIsolation drives the REAL interceptor chain over an
// httptest HTTP server. The stub resolver maps a fixed request header to a sub,
// standing in for webauth.Resolver (which this package cannot import without a
// cycle). It proves the interceptor → subjectFromConnectContext → store path
// enforces isolation: caller B never sees actor-A's private record, and an
// identity-less request gets CodeUnauthenticated.
func TestConnectCookieLaneIsolation(t *testing.T) {
	d := testDeps(t) // existing helper; skips when Qdrant unavailable
	ctx := context.Background()
	scope := "iso-cookie:project:xactor"

	aPriv := store.Memory{ID: "c0000000-0000-0000-0000-000000000001", Content: "A private", Scope: scope, Owner: "actor-A", Visibility: "private", Category: "convention", Source: "agent-inferred", CreatedAt: timeNow()}
	bPriv := store.Memory{ID: "c0000000-0000-0000-0000-000000000002", Content: "B private", Scope: scope, Owner: "actor-B", Visibility: "private", Category: "convention", Source: "agent-inferred", CreatedAt: timeNow()}
	for _, m := range []store.Memory{aPriv, bPriv} {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	defer func() {
		_ = d.st.Delete(ctx, aPriv.ID, store.Authenticated("actor-A"))
		_ = d.st.Delete(ctx, bPriv.ID, store.Authenticated("actor-B"))
	}()

	resolve := func(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		switch req.Header().Get("X-Test-Actor") {
		case "A":
			return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-A"}}, auth.LaneCookie, nil
		case "B":
			return &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-B"}}, auth.LaneCookie, nil
		default:
			return nil, auth.LaneUnknown, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
		}
	}

	mux := http.NewServeMux()
	// TestConnectCookieLaneIsolation only exercises reads (ListMemories),
	// which the CSRF interceptor never gates (SC3) — nil is fine.
	if err := d.mountConnect(mux, resolve, nil, nil); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := engramv1connect.NewEngramServiceClient(http.DefaultClient, srv.URL)

	// Caller B lists scope-x: must see B's own record (non-vacuous) and never A's.
	reqB := connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope})
	reqB.Header().Set("X-Test-Actor", "B")
	respB, err := client.ListMemories(ctx, reqB)
	if err != nil {
		t.Fatalf("ListMemories(B): %v", err)
	}
	sawBOwn := false
	for _, m := range respB.Msg.Memories {
		if m.Owner == "actor-A" {
			t.Fatalf("caller B saw actor-A record %q — isolation breach", m.Id)
		}
		if m.Id == bPriv.ID {
			sawBOwn = true
		}
	}
	if !sawBOwn {
		t.Fatalf("caller B did not see its own record %q — test would pass vacuously on an empty scope", bPriv.ID)
	}

	// No identity header → Unauthenticated.
	reqAnon := connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope})
	_, err = client.ListMemories(ctx, reqAnon)
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anon request: got code %v want unauthenticated", connect.CodeOf(err))
	}
}

// TestConnectNoCORSHeaders verifies that the Connect handler does NOT emit
// Access-Control-Allow-Origin (or any CORS headers) in response to a
// cross-origin preflight. The Connect lane is same-origin only; the web UI
// is served from the same host so no CORS grant is needed or safe.
func TestConnectNoCORSHeaders(t *testing.T) {
	d := &deps{} // no Qdrant needed
	mux := http.NewServeMux()
	resolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, auth.Lane, error) {
		return nil, auth.LaneCookie, nil
	}
	// TestConnectNoCORSHeaders only exercises an OPTIONS preflight, never a
	// write RPC, so csrfVerify is never invoked — nil is fine.
	if err := d.mountConnect(mux, resolve, nil, nil); err != nil {
		t.Fatalf("mountConnect: %v", err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/engram.v1.EngramService/ListMemories", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Connect handler must not emit CORS headers; got Access-Control-Allow-Origin=%q", got)
	}
}
