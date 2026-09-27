// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves ListScheduled's cross_spine + cursor widening (milestone
// 2026-09-25.01 Phase 3, D-11) end to end on Connect, and — once Task 2
// lands — the D-19 deferred-reveal, paging, and coverage proofs on both
// lanes plus the D-20 read-lane parity rows.

package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// seedScheduledFixture upserts a windowed (pending or expired) record for
// owner in scope, with the given created_at, and registers its cleanup.
// pending selects a not_before an hour in the future (hidden until then);
// !pending selects a not_after an hour in the past (already lapsed).
func seedScheduledFixture(t *testing.T, st *store.Store, owner, scope string, createdAt time.Time, pending, shared bool) string {
	t.Helper()
	id := uuid.NewString()
	m := store.Memory{
		ID: id, Content: "scheduled fixture", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: owner,
		CreatedAt: createdAt,
	}
	if pending {
		future := time.Now().UTC().Add(time.Hour)
		m.NotBefore = &future
	} else {
		past := time.Now().UTC().Add(-time.Hour)
		m.NotAfter = &past
	}
	if shared {
		m.Visibility = "shared"
	}
	if err := st.Upsert(context.Background(), m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed scheduled fixture %s: %v", id, err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete "+id, st.Delete(context.Background(), id, store.Authenticated(owner)))
	})
	return id
}

// TestListScheduledConnectCrossSpinePages proves D-11 end to end against a
// real Qdrant: owner A's five pending records across two scopes page
// correctly under cross_spine+limit, in created_at-descending order, with no
// duplicates and no gaps, while owner B's SHARED pending record in the same
// scope as some of A's records never appears (deferred reveal, D-11).
func TestListScheduledConnectCrossSpinePages(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	ownerA := "sub-listscheduled-a-" + uuid.NewString()
	ownerB := "sub-listscheduled-b-" + uuid.NewString()
	scope1 := "iso-test:project:listscheduled-s1-" + uuid.NewString()
	scope2 := "iso-test:project:listscheduled-s2-" + uuid.NewString()
	ctx := parityConnectCtx(ownerA)

	base := time.Now().UTC().Truncate(time.Second)
	var wantIDs []string
	// Three in scope1, two in scope2, distinct whole-second created_at —
	// descending offsets so the union is already in the expected order.
	for i := range 3 {
		wantIDs = append(wantIDs, seedScheduledFixture(t, st, ownerA, scope1, base.Add(time.Duration(-i)*time.Second), true, false))
	}
	for i := 3; i < 5; i++ {
		wantIDs = append(wantIDs, seedScheduledFixture(t, st, ownerA, scope2, base.Add(time.Duration(-i)*time.Second), true, false))
	}
	// Owner B's SHARED pending record in scope1 must never appear to owner A.
	bID := seedScheduledFixture(t, st, ownerB, scope1, base.Add(time.Second), true, true)

	var (
		gotIDs    []string
		pageSizes []int
		lastAt    time.Time
		haveLast  bool
		token     string
	)
	for {
		resp, err := api.ListScheduled(ctx, connect.NewRequest(&engramv1.ListScheduledRequest{
			CrossSpine: true, Limit: 2, PageToken: token,
		}))
		if err != nil {
			t.Fatalf("ListScheduled: %v", err)
		}
		mems := resp.Msg.GetMemories()
		pageSizes = append(pageSizes, len(mems))
		for _, m := range mems {
			gotIDs = append(gotIDs, m.GetId())
			at := m.GetCreatedAt().AsTime()
			if haveLast && at.After(lastAt) {
				t.Errorf("created_at increased mid-sequence: %v after %v (not non-increasing)", at, lastAt)
			}
			lastAt, haveLast = at, true
		}
		if resp.Msg.GetScopesUnknown() {
			t.Error("scopes_unknown = true, want false")
		}
		if len(resp.Msg.GetSearchedScopes()) == 0 {
			t.Error("searched_scopes empty on a cross_spine response, want non-empty")
		}
		token = resp.Msg.GetNextPageToken()
		if token == "" {
			break
		}
	}

	if len(pageSizes) != 3 || pageSizes[0] != 2 || pageSizes[1] != 2 || pageSizes[2] != 1 {
		t.Errorf("page sizes = %v, want [2 2 1]", pageSizes)
	}
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("got %d ids across all pages, want %d", len(gotIDs), len(wantIDs))
	}
	seen := make(map[string]bool, len(gotIDs))
	for _, id := range gotIDs {
		if seen[id] {
			t.Errorf("duplicate id %s across pages", id)
		}
		seen[id] = true
		if id == bID {
			t.Errorf("owner B's shared record %s appeared in owner A's cross_spine ListScheduled — deferred reveal violated (D-11)", bID)
		}
	}
	for _, id := range wantIDs {
		if !seen[id] {
			t.Errorf("expected id %s missing from the paged result", id)
		}
	}
}
