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

	resolve := func(_ context.Context, req connect.AnyRequest) (*mcpauth.TokenInfo, error) {
		switch req.Header().Get("X-Test-Actor") {
		case "A":
			return &mcpauth.TokenInfo{Extra: map[string]any{"sub": "actor-A"}}, nil
		case "B":
			return &mcpauth.TokenInfo{Extra: map[string]any{"sub": "actor-B"}}, nil
		default:
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
		}
	}

	mux := http.NewServeMux()
	if err := d.mountConnect(mux, resolve); err != nil {
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
