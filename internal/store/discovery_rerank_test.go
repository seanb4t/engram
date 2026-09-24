// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// discoveryRerankFixture seeds six discoveries in scope with vectors of
// strictly decreasing cosine similarity to query = {1,0,0} — id N (1-indexed)
// is the Nth-closest, so ids[5] (index 5, the sixth) is always the
// "lowest-vector" discovery in SearchDiscovery's own vector order.
func discoveryRerankFixture(t *testing.T, s *Store, scope, owner string) (ids []string, query []float32) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	ids = []string{
		"e7000000-0000-0000-0000-000000000001",
		"e7000000-0000-0000-0000-000000000002",
		"e7000000-0000-0000-0000-000000000003",
		"e7000000-0000-0000-0000-000000000004",
		"e7000000-0000-0000-0000-000000000005",
		"e7000000-0000-0000-0000-000000000006",
	}
	vecs := [][]float32{
		{1.0, 0.0, 0.0},
		{0.9, 0.1, 0.0},
		{0.8, 0.2, 0.0},
		{0.7, 0.3, 0.0},
		{0.6, 0.4, 0.0},
		{0.5, 0.5, 0.0},
	}
	for i, id := range ids {
		m := Memory{
			ID: id, Content: "discovery content", Scope: scope, Category: "discovery",
			Kind: "fact", Owner: owner, CreatedAt: now,
			Citations: []Citation{{Kind: "file", Ref: "f.go"}},
		}
		if err := s.Upsert(ctx, m, vecs[i]); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}
	return ids, []float32{1.0, 0.0, 0.0}
}

// TestSearchDiscoveryRerankedFallbackIsShippedOrder proves D-03/D-07's
// fallback contract for discoveries: a nil hook and a hook returning an
// error both yield exactly SearchDiscovery's own ids and order at k, with
// no Relevance stamped.
func TestSearchDiscoveryRerankedFallbackIsShippedOrder(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery-reranked-fallback:project:test"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	subj := Authenticated("owner-fallback")
	_, query := discoveryRerankFixture(t, s, scope, "owner-fallback")

	want, err := s.SearchDiscovery(ctx, scope, "", subj, query, 3)
	if err != nil {
		t.Fatalf("SearchDiscovery: %v", err)
	}
	wantIDs := recordIDs(want)

	nilHookGot, err := s.SearchDiscoveryReranked(ctx, scope, "", subj, "query text", query, 3, nil)
	if err != nil {
		t.Fatalf("SearchDiscoveryReranked(nil hook): %v", err)
	}
	if got := recordIDs(nilHookGot); !slices.Equal(got, wantIDs) {
		t.Fatalf("nil hook ids = %v, want shipped order %v", got, wantIDs)
	}
	for _, m := range nilHookGot {
		if m.Relevance != nil {
			t.Errorf("nil hook: hit %s has non-nil Relevance %v", m.ID, *m.Relevance)
		}
	}

	errHook := RankHook(func(_ context.Context, _ string, _ []Memory) (map[string]float64, error) {
		return nil, errors.New("boom: scripted hook failure")
	})
	errHookGot, err := s.SearchDiscoveryReranked(ctx, scope, "", subj, "query text", query, 3, errHook)
	if err != nil {
		t.Fatalf("SearchDiscoveryReranked(error hook): %v", err)
	}
	if got := recordIDs(errHookGot); !slices.Equal(got, wantIDs) {
		t.Fatalf("error hook ids = %v, want shipped order %v", got, wantIDs)
	}
	for _, m := range errHookGot {
		if m.Relevance != nil {
			t.Errorf("error hook: hit %s has non-nil Relevance %v", m.ID, *m.Relevance)
		}
	}
}

// TestSearchDiscoveryRerankedSortsAndStamps proves the hook is handed the
// WHOLE candidate pool (not k) in SearchDiscovery's vector order, and that a
// scored discovery is promoted to first place and carries its value.
func TestSearchDiscoveryRerankedSortsAndStamps(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery-reranked-sorts:project:test"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	subj := Authenticated("owner-sorts")
	_, query := discoveryRerankFixture(t, s, scope, "owner-sorts")

	pool, err := s.SearchDiscovery(ctx, scope, "", subj, query, 6)
	if err != nil {
		t.Fatalf("SearchDiscovery (pool probe): %v", err)
	}
	if len(pool) != 6 {
		t.Fatalf("pool size = %d, want 6", len(pool))
	}
	lowest := pool[len(pool)-1]

	var capturedPool []Memory
	hook := RankHook(func(_ context.Context, _ string, hits []Memory) (map[string]float64, error) {
		capturedPool = hits
		rel := make(map[string]float64, len(hits))
		for _, h := range hits {
			if h.ID == lowest.ID {
				rel[h.ID] = 0.95
				continue
			}
			rel[h.ID] = 0.1
		}
		return rel, nil
	})

	got, err := s.SearchDiscoveryReranked(ctx, scope, "", subj, "query text", query, 3, hook)
	if err != nil {
		t.Fatalf("SearchDiscoveryReranked: %v", err)
	}
	if len(capturedPool) != 6 {
		t.Fatalf("hook handed %d hits, want the whole pool of 6 (not k=3)", len(capturedPool))
	}
	if len(got) != 3 {
		t.Fatalf("got %d hits, want 3", len(got))
	}
	if got[0].ID != lowest.ID {
		t.Fatalf("first hit = %s, want the 0.95-scored lowest-vector discovery %s", got[0].ID, lowest.ID)
	}
	for _, m := range got {
		if m.Relevance == nil {
			t.Errorf("hit %s missing Relevance", m.ID)
		}
	}
}

// TestSearchDiscoveryRerankedOwnerIsolation proves the hook never sees, and
// the response never carries, another owner's private discovery — even when
// the hook itself tries to smuggle a high score in for it.
func TestSearchDiscoveryRerankedOwnerIsolation(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "discovery-reranked-isolation:project:test"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	vec := []float32{0.1, 0.2, 0.3}
	now := time.Now().UTC()
	cite := []Citation{{Kind: "file", Ref: "f.go"}}

	aOwn := Memory{
		ID: "e9000000-0000-0000-0000-000000000001", Content: "A's discovery", Scope: scope,
		Category: "discovery", Kind: "fact", Owner: "owner-A", CreatedAt: now, Citations: cite,
	}
	bPriv := Memory{
		ID: "e9000000-0000-0000-0000-000000000002", Content: "B's private discovery", Scope: scope,
		Category: "discovery", Kind: "fact", Owner: "owner-B", CreatedAt: now, Citations: cite,
	}
	bShared := Memory{
		ID: "e9000000-0000-0000-0000-000000000003", Content: "B's shared discovery", Scope: scope,
		Category: "discovery", Kind: "fact", Owner: "owner-B", Visibility: "shared", CreatedAt: now, Citations: cite,
	}
	for _, m := range []Memory{aOwn, bPriv, bShared} {
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("upsert %s: %v", m.ID, err)
		}
	}

	var received []Memory
	hook := RankHook(func(_ context.Context, _ string, hits []Memory) (map[string]float64, error) {
		received = hits
		rel := make(map[string]float64, len(hits)+1)
		for _, h := range hits {
			rel[h.ID] = 0.5
		}
		// Attempt to smuggle a high score in for B's private discovery — must
		// have zero effect, since the hook is never handed it in the first
		// place (ranking can only reorder the hits it received).
		rel[bPriv.ID] = 1.0
		return rel, nil
	})

	got, err := s.SearchDiscoveryReranked(ctx, scope, "", Authenticated("owner-A"), "query", vec, 10, hook)
	if err != nil {
		t.Fatalf("SearchDiscoveryReranked: %v", err)
	}

	wantIDs := []string{aOwn.ID, bShared.ID}
	gotIDs := recordIDs(got)
	slices.Sort(gotIDs)
	slices.Sort(wantIDs)
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("SearchDiscoveryReranked ids = %v, want exactly %v", gotIDs, wantIDs)
	}
	receivedIDs := recordIDs(received)
	slices.Sort(receivedIDs)
	if !slices.Equal(receivedIDs, wantIDs) {
		t.Fatalf("hook received %v, want exactly the authz-filtered pool %v", receivedIDs, wantIDs)
	}
}

// TestSearchDiscoveryRerankedRejectsZeroK proves the k<=0 contract
// hermetically: the guard runs before any Qdrant call, so a Store built over
// a nil client never dereferences it.
func TestSearchDiscoveryRerankedRejectsZeroK(t *testing.T) {
	t.Parallel()
	s := New(nil, "unused-hermetic-collection")
	_, err := s.SearchDiscoveryReranked(context.Background(), "scope", "", Authenticated("actor"),
		"query", []float32{0.1, 0.2}, 0, nil)
	if err == nil {
		t.Fatal("SearchDiscoveryReranked(k=0) should error, not silently over-fetch-then-truncate-to-empty")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("SearchDiscoveryReranked(k=0) error = %v, want ErrInvalidArgument", err)
	}
}
