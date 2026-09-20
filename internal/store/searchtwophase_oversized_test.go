// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-search-k-bounded's real-Qdrant regressions for
// Store.Search/Store.SearchReranked/Store.SearchDiscovery's D-09 two-phase
// fetch, over both oversized fixture shapes storetest.SeedOversized
// supports, at the named storetest.RecvLimit — OUR request shape and OUR
// page contract, never Qdrant's or grpc-go's own behavior (rule
// m45p2b4bp7).
package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
	"google.golang.org/grpc"
)

// searchFetchRecordedCall is one intercepted Query or Scroll RPC this file's
// tests care about, as searchFetchRecorder.intercept observed it.
type searchFetchRecordedCall struct {
	method string // "Query" or "Scroll"
	limit  uint32 // Scroll's requested Limit; zero for Query
	// payload is Query-only: true if the request's WithPayload selector
	// requested payload at all (Enable(true), Include, or Exclude) — the
	// two-phase vector query must always request none.
	payload bool
	filter  *qdrant.Filter
}

// searchFetchRecorder is a mutex-guarded recording grpc.UnaryClientInterceptor
// scoped to the two request types this file's tests care about (*qdrant.
// QueryPoints, *qdrant.ScrollPoints) — every other request passes through
// unrecorded. reset/snapshot let a subtest isolate the calls belonging to
// its own scenario, mirroring boundedread_oversized_test.go's scrollRecorder
// idiom.
type searchFetchRecorder struct {
	mu    sync.Mutex
	calls []searchFetchRecordedCall
}

func (r *searchFetchRecorder) intercept(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	var call searchFetchRecordedCall
	var recognized bool
	switch m := req.(type) {
	case *qdrant.QueryPoints:
		sel := m.GetWithPayload()
		call = searchFetchRecordedCall{
			method:  "Query",
			filter:  m.GetFilter(),
			payload: sel.GetEnable() || sel.GetInclude() != nil || sel.GetExclude() != nil,
		}
		recognized = true
	case *qdrant.ScrollPoints:
		call = searchFetchRecordedCall{method: "Scroll", limit: m.GetLimit(), filter: m.GetFilter()}
		recognized = true
	}
	err := invoker(ctx, method, req, reply, cc, opts...)
	if recognized {
		r.mu.Lock()
		r.calls = append(r.calls, call)
		r.mu.Unlock()
	}
	return err
}

func (r *searchFetchRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

func (r *searchFetchRecorder) snapshot() []searchFetchRecordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]searchFetchRecordedCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// filterCarriesIDSetAndNested reports whether f's top-level Must list
// contains BOTH a has_id condition (the id-set inclusion) and a nested
// filter condition (the caller's own filter, wrapped via
// qdrant.NewFilterAsCondition) — includeIDs' (searchfetch.go) narrow-only
// AND shape.
func filterCarriesIDSetAndNested(f *qdrant.Filter) (hasID, nested bool) {
	if f == nil {
		return false, false
	}
	for _, cond := range f.GetMust() {
		if cond.GetHasId() != nil {
			hasID = true
		}
		if cond.GetFilter() != nil {
			nested = true
		}
	}
	return hasID, nested
}

// TestSearchTwoPhaseBounded proves Store.Search/Store.SearchReranked's D-09
// two-phase fetch over both oversized fixture shapes storetest.SeedOversized
// supports: a reranked search well above the default succeeds; every
// returned id is unique; every recorded fetch Scroll's limit stays within
// store.PerRPCLimit of the view it used; the vector Query itself carries a
// payload-free selector; each fetch Scroll's filter carries BOTH the id-set
// condition and the caller's own filter as a nested condition; and another
// owner's private record placed directly in a fetch batch, driven with the
// first owner's own captured filter, never comes back.
func TestSearchTwoPhaseBounded(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			rec := &searchFetchRecorder{}
			c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
			name := store.PrefixedTestCollection("oversized_searchtwophase_" + uuid.NewString())
			st := store.NewTestStore(t, c, name)
			ctx := context.Background()
			vec := []float32{0.1, 0.2, 0.3}
			if err := st.EnsureCollection(ctx, uint64(len(vec))); err != nil {
				t.Fatalf("%s: EnsureCollection: %v", shape, err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("%s: DeleteCollection(%q): %v", shape, name, err)
				}
			})

			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: vec})
			owner := store.Authenticated(fx.Owner)
			seededTotal := len(fx.IDs)

			// A reranked search well above the default succeeds end to end
			// over the oversized fixture; every returned id is unique.
			k := uint64(seededTotal)
			if k < 32 {
				k = 32
			}
			rec.reset()
			hits, err := st.SearchReranked(ctx, fx.Scope, owner, "", vec, k, store.SearchOptions{})
			if err != nil {
				t.Fatalf("%s: SearchReranked(max): %v", shape, err)
			}
			if len(hits) == 0 {
				t.Fatalf("%s: SearchReranked(max) returned no hits over a seeded fixture", shape)
			}
			seen := map[string]int{}
			for _, m := range hits {
				seen[m.ID]++
				if seen[m.ID] > 1 {
					t.Errorf("%s: id %s returned more than once", shape, m.ID)
				}
			}

			calls := rec.snapshot()
			fullPerRPC := uint32(store.PerRPCLimit(st.FullView()))
			var sawQuery, sawScroll bool
			var ownerFilter *qdrant.Filter
			for i, call := range calls {
				switch call.method {
				case "Query":
					sawQuery = true
					if ownerFilter == nil {
						ownerFilter = call.filter
					}
					if call.payload {
						t.Errorf("%s: call %d Query requested payload — the vector query must be payload-free (D-09)", shape, i)
					}
				case "Scroll":
					sawScroll = true
					if call.limit < 1 || call.limit > fullPerRPC {
						t.Errorf("%s: fetch call %d limit = %d, want between 1 and %d", shape, i, call.limit, fullPerRPC)
					}
					hasID, nested := filterCarriesIDSetAndNested(call.filter)
					if !hasID || !nested {
						t.Errorf("%s: fetch call %d filter = %+v, want a Must list carrying both the id-set condition (has_id) and the caller's filter as a nested condition (filter)", shape, i, call.filter)
					}
				}
			}
			if !sawQuery {
				t.Errorf("%s: no Query call recorded", shape)
			}
			if !sawScroll {
				t.Errorf("%s: no fetch Scroll call recorded", shape)
			}
			if ownerFilter == nil {
				t.Fatalf("%s: no captured Query filter to drive the cross-owner subtest", shape)
			}

			// Cross-owner: another owner's private record must never surface
			// through the fetch helper, even placed directly in the batch
			// alongside the first owner's own filter (captured above from
			// the Query call Search itself issued).
			otherOwner := "storetest-owner-" + uuid.NewString()
			otherID := uuid.NewString()
			if err := st.Upsert(ctx, store.Memory{
				ID: otherID, Content: "other owner private", Scope: fx.Scope, Owner: otherOwner,
				Actor: otherOwner, CreatedAt: time.Now(),
			}, vec); err != nil {
				t.Fatalf("%s: seed other owner's private record: %v", shape, err)
			}
			t.Cleanup(func() {
				if delErr := st.DeleteAll(context.Background(), fx.Scope, store.Authenticated(otherOwner)); delErr != nil {
					t.Errorf("%s: cleanup other owner's record: %v", shape, delErr)
				}
			})
			batchLen := 3
			if batchLen > len(fx.IDs) {
				batchLen = len(fx.IDs)
			}
			batch := append([]string{otherID}, fx.IDs[:batchLen]...)
			fetched, fErr := st.FetchPayloadsByID(ctx, ownerFilter, st.FullView(), batch)
			if fErr != nil {
				t.Fatalf("%s: FetchPayloadsByID(cross-owner): %v", shape, fErr)
			}
			if _, ok := fetched[otherID]; ok {
				t.Errorf("%s: FetchPayloadsByID returned another owner's private record %s", shape, otherID)
			}
		})
	}
}

// TestSearchDiscoveryTwoPhaseBounded proves Store.SearchDiscovery's D-09
// two-phase fetch over both oversized fixture shapes: a discovery search
// well above the default succeeds end to end with no Qdrant response
// exceeding the per-RPC budget, a superseded discovery and an archived
// discovery never appear, and an anonymous caller never receives another
// owner's shared discovery (nor any of the fixture owner's own records).
func TestSearchDiscoveryTwoPhaseBounded(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			rec := &searchFetchRecorder{}
			c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
			name := store.PrefixedTestCollection("oversized_searchdiscovery_" + uuid.NewString())
			st := store.NewTestStore(t, c, name)
			ctx := context.Background()
			vec := []float32{0.1, 0.2, 0.3}
			if err := st.EnsureCollection(ctx, uint64(len(vec))); err != nil {
				t.Fatalf("%s: EnsureCollection: %v", shape, err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("%s: DeleteCollection(%q): %v", shape, name, err)
				}
			})

			fx := storetest.SeedOversized(t, st, storetest.Spec{
				Limit: storetest.RecvLimit, Shape: shape, Vector: vec,
				Template: store.Memory{Category: "discovery", Kind: "fact"},
			})
			owner := store.Authenticated(fx.Owner)
			seededTotal := len(fx.IDs)

			// Edge-case records seeded directly through the public Upsert
			// path (never a raw *qdrant.Client), mirroring
			// listscheduled_oversized_test.go's idiom: none belong to
			// fx.IDs and none should ever surface from SearchDiscovery.
			supersededID := uuid.NewString()
			supersededBy := uuid.NewString()
			if err := st.Upsert(ctx, store.Memory{
				ID: supersededID, Content: "superseded discovery", Scope: fx.Scope, Owner: fx.Owner,
				Actor: fx.Owner, CreatedAt: time.Now(), Category: "discovery", Kind: "fact",
				SupersededBy: &supersededBy,
			}, vec); err != nil {
				t.Fatalf("%s: seed superseded discovery: %v", shape, err)
			}

			archivedID := uuid.NewString()
			archivedAt := time.Now()
			if err := st.Upsert(ctx, store.Memory{
				ID: archivedID, Content: "archived discovery", Scope: fx.Scope, Owner: fx.Owner,
				Actor: fx.Owner, CreatedAt: time.Now(), Category: "discovery", Kind: "fact",
				ArchivedAt: &archivedAt,
			}, vec); err != nil {
				t.Fatalf("%s: seed archived discovery: %v", shape, err)
			}

			otherOwner := "storetest-owner-" + uuid.NewString()
			sharedID := uuid.NewString()
			if err := st.Upsert(ctx, store.Memory{
				ID: sharedID, Content: "shared discovery", Scope: fx.Scope, Owner: otherOwner,
				Actor: otherOwner, Visibility: "shared", CreatedAt: time.Now(), Category: "discovery", Kind: "fact",
			}, vec); err != nil {
				t.Fatalf("%s: seed another owner's shared discovery: %v", shape, err)
			}
			t.Cleanup(func() {
				if delErr := st.DeleteAll(context.Background(), fx.Scope, store.Authenticated(otherOwner)); delErr != nil {
					t.Errorf("%s: cleanup other owner's discovery: %v", shape, delErr)
				}
			})

			k := uint64(seededTotal)
			if k < 32 {
				k = 32
			}
			rec.reset()
			hits, err := st.SearchDiscovery(ctx, fx.Scope, "", owner, vec, k)
			if err != nil {
				t.Fatalf("%s: SearchDiscovery(max): %v", shape, err)
			}
			if len(hits) == 0 {
				t.Fatalf("%s: SearchDiscovery(max) returned no hits over a seeded fixture", shape)
			}
			// sharedID is deliberately NOT checked here: per
			// ownerOrSharedCondition's own contract, ANY visibility=="shared"
			// record is visible to any authenticated caller regardless of
			// its actual owner — fx.Owner legitimately sees it. The
			// anonymous assertion below is what proves shared records
			// require an authenticated subject.
			for _, m := range hits {
				if m.ID == supersededID || m.ID == archivedID {
					t.Errorf("%s: SearchDiscovery(max) returned excluded id %s", shape, m.ID)
				}
			}

			fullPerRPC := uint32(store.PerRPCLimit(st.FullView()))
			for i, call := range rec.snapshot() {
				if call.method != "Scroll" {
					continue
				}
				if call.limit < 1 || call.limit > fullPerRPC {
					t.Errorf("%s: fetch call %d limit = %d, want between 1 and %d", shape, i, call.limit, fullPerRPC)
				}
			}

			// An anonymous caller never receives another owner's shared
			// discovery, nor any of fx.Owner's own (private) records — the
			// scope carries no genuinely ownerless discovery, so the
			// anonymous result must be empty.
			anonHits, aErr := st.SearchDiscovery(ctx, fx.Scope, "", store.Anonymous(), vec, k)
			if aErr != nil {
				t.Fatalf("%s: SearchDiscovery(anonymous): %v", shape, aErr)
			}
			if len(anonHits) != 0 {
				t.Errorf("%s: SearchDiscovery(anonymous) returned %d hit(s) in a scope with no ownerless discoveries, want 0", shape, len(anonHits))
			}
		})
	}
}

// TestSearchFetchSkipsEmptyBatch pins the empty-batch short-circuit
// TestSchemaVersionNeverGatesRecall's six search rows depend on (RESEARCH.md
// Pitfall 1): a search against an EMPTY collection issues exactly one
// payload-free gRPC Query and never an additional Scroll, for all three
// search entry points; store.FetchPayloadsByID driven directly with an
// empty id slice records zero calls and returns a nil error; and an id that
// does not exist, batched alongside ids that do, is simply absent from the
// returned map — the drop-on-disappear semantics, never an error.
func TestSearchFetchSkipsEmptyBatch(t *testing.T) {
	rec := &searchFetchRecorder{}
	c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
	name := store.PrefixedTestCollection("oversized_searchfetch_empty_" + uuid.NewString())
	st := store.NewTestStore(t, c, name)
	ctx := context.Background()
	vec := []float32{0.1, 0.2, 0.3}
	if err := st.EnsureCollection(ctx, uint64(len(vec))); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() {
		if err := c.DeleteCollection(ctx, name); err != nil {
			t.Errorf("DeleteCollection(%q): %v", name, err)
		}
	})
	scope := "storetest-empty:project:" + uuid.NewString()
	ownerStr := "storetest-owner-" + uuid.NewString()
	owner := store.Authenticated(ownerStr)

	assertOneQueryNoScroll := func(t *testing.T, label string) {
		t.Helper()
		var queries, scrolls int
		for _, call := range rec.snapshot() {
			switch call.method {
			case "Query":
				queries++
				if call.payload {
					t.Errorf("%s: Query requested payload — the vector query must be payload-free (D-09)", label)
				}
			case "Scroll":
				scrolls++
			}
		}
		if queries != 1 {
			t.Errorf("%s: recorded %d Query call(s), want exactly 1", label, queries)
		}
		if scrolls != 0 {
			t.Errorf("%s: recorded %d Scroll call(s), want 0 (an empty id batch must never issue a fetch RPC)", label, scrolls)
		}
	}

	rec.reset()
	if _, err := st.Search(ctx, scope, owner, vec, 5, store.SearchOptions{}); err != nil {
		t.Fatalf("Search(empty): %v", err)
	}
	assertOneQueryNoScroll(t, "Search")

	rec.reset()
	if _, err := st.SearchReranked(ctx, scope, owner, "q", vec, 5, store.SearchOptions{}); err != nil {
		t.Fatalf("SearchReranked(empty): %v", err)
	}
	assertOneQueryNoScroll(t, "SearchReranked")

	rec.reset()
	if _, err := st.SearchDiscovery(ctx, scope, "", owner, vec, 5); err != nil {
		t.Fatalf("SearchDiscovery(empty): %v", err)
	}
	assertOneQueryNoScroll(t, "SearchDiscovery")

	rec.reset()
	fetched, fErr := st.FetchPayloadsByID(ctx, nil, st.FullView(), nil)
	if fErr != nil {
		t.Fatalf("FetchPayloadsByID(empty ids): %v", fErr)
	}
	if len(fetched) != 0 {
		t.Errorf("FetchPayloadsByID(empty ids) returned %d entries, want 0", len(fetched))
	}
	if calls := rec.snapshot(); len(calls) != 0 {
		t.Errorf("FetchPayloadsByID(empty ids) recorded %d call(s), want 0", len(calls))
	}

	t.Run("drop-on-disappear", func(t *testing.T) {
		id1, id2 := uuid.NewString(), uuid.NewString()
		if err := st.Upsert(ctx, store.Memory{
			ID: id1, Content: "present one", Scope: scope, Owner: ownerStr,
			Actor: ownerStr, CreatedAt: time.Now(),
		}, vec); err != nil {
			t.Fatalf("seed id1: %v", err)
		}
		if err := st.Upsert(ctx, store.Memory{
			ID: id2, Content: "present two", Scope: scope, Owner: ownerStr,
			Actor: ownerStr, CreatedAt: time.Now(),
		}, vec); err != nil {
			t.Fatalf("seed id2: %v", err)
		}
		t.Cleanup(func() {
			if delErr := st.DeleteAll(context.Background(), scope, owner); delErr != nil {
				t.Errorf("cleanup: %v", delErr)
			}
		})
		missingID := uuid.NewString()
		fetched, fErr := st.FetchPayloadsByID(ctx, nil, st.FullView(), []string{id1, missingID, id2})
		if fErr != nil {
			t.Fatalf("FetchPayloadsByID(drop-on-disappear): %v", fErr)
		}
		if _, ok := fetched[missingID]; ok {
			t.Errorf("FetchPayloadsByID returned an id (%s) that was never upserted", missingID)
		}
		if _, ok := fetched[id1]; !ok {
			t.Errorf("FetchPayloadsByID dropped id1 (%s), which exists", id1)
		}
		if _, ok := fetched[id2]; !ok {
			t.Errorf("FetchPayloadsByID dropped id2 (%s), which exists", id2)
		}
	})
}

// TestSearchPreservesRankOrder proves the two-phase fetch reproduces the
// vector query's own rank order and scores exactly: seeded at deliberately
// different (strictly decreasing, never tied) cosine distances from the
// query vector, Store.Search's result order and per-hit scores match a
// control run that requests the payload inline — a single full-payload
// Query issued directly against the raw client, mirroring the pre-04-04
// single-phase shape — at the same small count, and every score is
// non-zero and non-increasing along the result.
func TestSearchPreservesRankOrder(t *testing.T) {
	c := storetest.Dial(t, storetest.RecvLimit)
	name := store.PrefixedTestCollection("oversized_searchrankorder_" + uuid.NewString())
	st := store.NewTestStore(t, c, name)
	ctx := context.Background()
	if err := st.EnsureCollection(ctx, 2); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	t.Cleanup(func() {
		if err := c.DeleteCollection(ctx, name); err != nil {
			t.Errorf("DeleteCollection(%q): %v", name, err)
		}
	})

	scope := "storetest-rankorder:project:" + uuid.NewString()
	ownerStr := "storetest-owner-" + uuid.NewString()
	owner := store.Authenticated(ownerStr)
	queryVec := []float32{1, 0}
	// Deliberately different cosine distances from queryVec, strictly
	// decreasing similarity — never tied.
	vecs := [][]float32{{1, 0}, {0.9, 0.1}, {0.7, 0.3}, {0.5, 0.5}, {0.3, 0.7}}
	ids := make([]string, len(vecs))
	for i, v := range vecs {
		ids[i] = uuid.NewString()
		if err := st.Upsert(ctx, store.Memory{
			ID: ids[i], Content: "rankorder", Scope: scope, Owner: ownerStr,
			Actor: ownerStr, CreatedAt: time.Now(),
		}, v); err != nil {
			t.Fatalf("seed record %d: %v", i, err)
		}
	}
	t.Cleanup(func() {
		if delErr := st.DeleteAll(context.Background(), scope, owner); delErr != nil {
			t.Errorf("cleanup: %v", delErr)
		}
	})

	// Control: a single full-payload Query directly against the raw
	// client — the independent reference this test compares the two-phase
	// fetch against.
	controlFilter := &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatch("scope", scope)}}
	control, cErr := c.Query(ctx, &qdrant.QueryPoints{
		CollectionName: name, Query: qdrant.NewQuery(queryVec...),
		Filter: controlFilter, Limit: qdrant.PtrOf(uint64(len(ids))), WithPayload: qdrant.NewWithPayload(true),
	})
	if cErr != nil {
		t.Fatalf("control Query: %v", cErr)
	}
	if len(control) != len(ids) {
		t.Fatalf("control Query returned %d hits, want %d", len(control), len(ids))
	}
	wantOrder := make([]string, len(control))
	wantScores := make([]float32, len(control))
	for i, p := range control {
		wantOrder[i] = p.Id.GetUuid()
		wantScores[i] = p.Score
	}

	hits, err := st.Search(ctx, scope, owner, queryVec, uint64(len(ids)), store.SearchOptions{Full: true})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != len(wantOrder) {
		t.Fatalf("Search returned %d hits, want %d", len(hits), len(wantOrder))
	}
	for i, m := range hits {
		if m.ID != wantOrder[i] {
			t.Errorf("hit %d id = %s, want %s (control order)", i, m.ID, wantOrder[i])
		}
		if m.Score != wantScores[i] {
			t.Errorf("hit %d score = %v, want %v (control score)", i, m.Score, wantScores[i])
		}
		if m.Score == 0 {
			t.Errorf("hit %d score is zero, want non-zero", i)
		}
		if i > 0 && m.Score > hits[i-1].Score {
			t.Errorf("hit %d score %v > previous hit's score %v — result must be non-increasing", i, m.Score, hits[i-1].Score)
		}
	}
}
