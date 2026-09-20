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
