// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

func TestMemoryToProto(t *testing.T) {
	now := time.Now().UTC()
	m := store.Memory{
		ID: "id1", Content: "c", Scope: "s", Owner: "sub-A",
		Visibility: "shared", Tags: []string{"x", "y"}, CreatedAt: now,
	}
	pb := memoryToProto(m)
	if pb.Id != "id1" || pb.Owner != "sub-A" || pb.Visibility != "shared" {
		t.Errorf("scalar fields not mapped: %+v", pb)
	}
	if len(pb.Tags) != 2 || pb.CreatedAt.AsTime().Unix() != now.Unix() {
		t.Errorf("tags/created_at not mapped: %+v", pb)
	}
}

// seedCrossActorRecords inserts a shared and a private record owned by actor-A into the
// given store, returning cleanup functions. Uses a unique scope derived from the test name.
func seedCrossActorRecords(t *testing.T, d *deps, scope string) (shared, priv store.Memory) {
	t.Helper()
	shared = store.Memory{
		ID: "d2222222-0000-0000-0000-000000000001", Content: "shared",
		Scope: scope, Owner: "actor-A", Visibility: "shared", CreatedAt: timeNow(),
	}
	priv = store.Memory{
		ID: "d2222222-0000-0000-0000-000000000002", Content: "private",
		Scope: scope, Owner: "actor-A", Visibility: "", CreatedAt: timeNow(),
	}
	ctx := context.Background()
	for _, m := range []store.Memory{shared, priv} {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete shared", d.st.Delete(ctx, shared.ID, store.Authenticated("actor-A")))
		cleanupErr(t, "Delete priv", d.st.Delete(ctx, priv.ID, store.Authenticated("actor-A")))
	})
	return shared, priv
}

func TestConnectCrossActorIsolation(t *testing.T) {
	d := testDeps(t)
	api := &engramAPI{d: d}
	scope := "iso-test:project:connect-xactor"
	shared, priv := seedCrossActorRecords(t, d, scope)
	ctx := context.Background()

	// caller B (distinct authed sub) injected via the test seam.
	bctx := withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"sub": "actor-B"}})

	t.Run("ListMemories_negative_no_private_leak", func(t *testing.T) {
		resp, err := api.ListMemories(bctx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10}))
		if err != nil {
			t.Fatalf("ListMemories: %v", err)
		}
		for _, m := range resp.Msg.Memories {
			if m.Owner == "actor-A" && m.Visibility != "shared" {
				t.Errorf("B leaked A's private record %s via Connect", m.Id)
			}
		}
	})

	t.Run("ListMemories_positive_shared_returned", func(t *testing.T) {
		resp, err := api.ListMemories(bctx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10}))
		if err != nil {
			t.Fatalf("ListMemories: %v", err)
		}
		found := false
		for _, m := range resp.Msg.Memories {
			if m.Id == shared.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("B should see A's shared record %s via ListMemories, but it was absent", shared.ID)
		}
	})

	t.Run("GetMemory_private_denied", func(t *testing.T) {
		_, err := api.GetMemory(bctx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: priv.ID}))
		if err == nil {
			t.Fatal("GetMemory: expected error for A's private record, got nil")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("GetMemory: expected CodeNotFound, got %v", connect.CodeOf(err))
		}
	})

	t.Run("GetMemory_shared_allowed", func(t *testing.T) {
		resp, err := api.GetMemory(bctx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: shared.ID}))
		if err != nil {
			t.Fatalf("GetMemory: expected success for A's shared record, got %v", err)
		}
		if resp.Msg.Memory.Id != shared.ID {
			t.Errorf("GetMemory: returned id %q, want %q", resp.Msg.Memory.Id, shared.ID)
		}
	})

	t.Run("SearchMemories_no_private_leak", func(t *testing.T) {
		resp, err := api.SearchMemories(bctx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Scope: scope, Query: "any", K: 20,
		}))
		if err != nil {
			t.Fatalf("SearchMemories: %v", err)
		}
		for _, m := range resp.Msg.Memories {
			if m.Owner == "actor-A" && m.Visibility != "shared" {
				t.Errorf("SearchMemories: B leaked A's private record %s", m.Id)
			}
		}
	})

	t.Run("Anonymous_ListMemories_sees_nothing_of_actor_A", func(t *testing.T) {
		// Anonymous = interceptor ran with nil TokenInfo (auth disabled / no issuer).
		// Inject via the test seam the same way the interceptor would: key present, ti==nil.
		anonCtx := withConnectTokenInfo(context.Background(), nil)
		resp, err := api.ListMemories(anonCtx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10}))
		if err != nil {
			t.Fatalf("ListMemories(anon): %v", err)
		}
		for _, m := range resp.Msg.Memories {
			if m.Owner != "" {
				t.Errorf("Anonymous caller saw record owned by %q (id=%s); expected empty-owner bucket only", m.Owner, m.Id)
			}
		}
	})

	t.Run("NoInterceptor_fails_closed", func(t *testing.T) {
		// Bare context.Background() has no connectSubjectKey → interceptor was never installed.
		// subjectFromConnectContext must fail closed with an error, NOT silently grant anonymous access.
		_, err := api.ListMemories(context.Background(), connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10}))
		if err == nil {
			t.Fatal("ListMemories with absent interceptor key: expected error (fail-closed), got nil")
		}
	})
}

// Note: SearchDiscoveries shares the identical subjectFromConnectContext seam.
// Coverage is skipped here because discovery-scope seeding requires a separate
// collection setup and discovery-specific Upsert path not yet exposed from testDeps.
