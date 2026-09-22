// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-list-bounded/REQ-list-contract-unchanged's real-Qdrant
// regressions for Store.List's projection selection (04-CONTEXT.md D-04,
// Phase 4 04-05) over both oversized fixture shapes storetest.SeedOversized
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

// recallViewCallShape is one intercepted Scroll RPC's request Limit, payload
// selector shape, and whether its filter carries a top-level Must has_id
// condition — the shape includeIDs (searchfetch.go) always produces and
// excludeSeen (orderedpage.go) never does, letting this file's tests tell a
// page-fetch Scroll apart from a backfillNoSummaryContent id-set fetch
// Scroll without any other instrumentation.
type recallViewCallShape struct {
	limit   uint32
	exclude bool // true when the selector excludes content/citations (summary view)
	idSet   bool // true when the filter carries a top-level Must has_id condition (a backfill fetch)
}

// recallViewRecorder is a mutex-guarded recording grpc.UnaryClientInterceptor
// scoped to methods ending "/Scroll".
type recallViewRecorder struct {
	mu    sync.Mutex
	calls []recallViewCallShape
}

func filterHasTopLevelHasID(f *qdrant.Filter) bool {
	if f == nil {
		return false
	}
	for _, cond := range f.GetMust() {
		if cond.GetHasId() != nil {
			return true
		}
	}
	return false
}

func (r *recallViewRecorder) intercept(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	if len(method) < len("/Scroll") || method[len(method)-len("/Scroll"):] != "/Scroll" {
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	var call recallViewCallShape
	if sp, ok := req.(*qdrant.ScrollPoints); ok {
		call.limit = sp.GetLimit()
		call.exclude = sp.GetWithPayload().GetExclude() != nil
		call.idSet = filterHasTopLevelHasID(sp.GetFilter())
	}
	err := invoker(ctx, method, req, reply, cc, opts...)
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	return err
}

func (r *recallViewRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

func (r *recallViewRecorder) snapshot() []recallViewCallShape {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recallViewCallShape, len(r.calls))
	copy(out, r.calls)
	return out
}

// TestRecallViewSelection runs one subtest per fixture shape, few-large then
// many-small, over a fixture whose every record already carries a stored
// summary (so the no-summary backfill never fires and cannot contaminate
// this test's per-call selector/limit assertions — TestNoSummaryContentBackfill
// below is where the backfill itself is proven): a default (non-full) list's
// recorded Scrolls all carry the exclude-shaped (summary) payload selector
// and a materially larger per-RPC limit than a full list's, whose Scrolls
// all carry the full selector; both return the same ids in the same order
// with the same total.
func TestRecallViewSelection(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			rec := &recallViewRecorder{}
			c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
			name := store.PrefixedTestCollection("oversized_recallview_select_" + uuid.NewString())
			st := store.NewTestStore(t, c, name)
			ctx := context.Background()
			if err := st.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("%s: EnsureCollection: %v", shape, err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("%s: DeleteCollection(%q): %v", shape, name, err)
				}
			})

			fx := storetest.SeedOversized(t, st, storetest.Spec{
				Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3},
				Template: store.Memory{Summary: "recallview fixture summary"},
			})
			owner := store.Authenticated(fx.Owner)
			seededTotal := uint64(len(fx.IDs))

			summaryPerRPC := uint32(store.PerRPCLimit(st.SummaryView()))
			fullPerRPC := uint32(store.PerRPCLimit(st.FullView()))
			if summaryPerRPC <= fullPerRPC {
				t.Fatalf("%s: summary view PerRPCLimit (%d) is not materially larger than full view's (%d)", shape, summaryPerRPC, fullPerRPC)
			}

			rec.reset()
			defItems, defTotal, _, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: seededTotal})
			if err != nil {
				t.Fatalf("%s: default list: %v", shape, err)
			}
			defCalls := rec.snapshot()
			if len(defCalls) == 0 {
				t.Fatalf("%s: no Scroll calls recorded for the default list", shape)
			}
			for i, call := range defCalls {
				if call.idSet {
					t.Errorf("%s: default-list call %d is a backfill id-set fetch — every fixture record has a stored summary, so no backfill RPC should ever fire", shape, i)
				}
				if !call.exclude {
					t.Errorf("%s: default-list call %d selector did not exclude content/citations — want the summary view", shape, i)
				}
				if call.limit < 1 || call.limit > summaryPerRPC {
					t.Errorf("%s: default-list call %d limit = %d, want between 1 and %d", shape, i, call.limit, summaryPerRPC)
				}
			}

			rec.reset()
			fullItems, fullTotal, _, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: seededTotal, Full: true})
			if err != nil {
				t.Fatalf("%s: full list: %v", shape, err)
			}
			fullCalls := rec.snapshot()
			if len(fullCalls) == 0 {
				t.Fatalf("%s: no Scroll calls recorded for the full list", shape)
			}
			for i, call := range fullCalls {
				if call.idSet {
					t.Errorf("%s: full-list call %d is an unexpected backfill id-set fetch", shape, i)
				}
				if call.exclude {
					t.Errorf("%s: full-list call %d selector excluded content/citations — want the full view", shape, i)
				}
				if call.limit < 1 || call.limit > fullPerRPC {
					t.Errorf("%s: full-list call %d limit = %d, want between 1 and %d", shape, i, call.limit, fullPerRPC)
				}
			}

			if defTotal != seededTotal || fullTotal != seededTotal {
				t.Errorf("%s: total mismatch: default=%d full=%d, want %d", shape, defTotal, fullTotal, seededTotal)
			}
			if len(defItems) != len(fullItems) {
				t.Fatalf("%s: default list returned %d items, full list returned %d", shape, len(defItems), len(fullItems))
			}
			for i := range defItems {
				if defItems[i].ID != fullItems[i].ID {
					t.Errorf("%s: item %d id mismatch: default=%s full=%s (ids/order must be projection-independent)", shape, i, defItems[i].ID, fullItems[i].ID)
				}
			}
		})
	}
}

// TestNoSummaryContentBackfill proves the shared no-summary content backfill
// (backfillNoSummaryContent, searchfetch.go) over both oversized fixture
// shapes: a page whose records ALL carry a stored summary issues no backfill
// (id-set) Scroll at all; once a stored-summary record and a no-summary
// record are added, a default list restores content for exactly the
// no-summary record (never the summarized one, whose content was never
// fetched at all) while a full list restores content for both; and ids,
// order and total stay identical between the default and full runs in every
// case.
func TestNoSummaryContentBackfill(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			rec := &recallViewRecorder{}
			c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
			name := store.PrefixedTestCollection("oversized_recallview_backfill_" + uuid.NewString())
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
				Template: store.Memory{Summary: "backfill fixture summary"},
			})
			owner := store.Authenticated(fx.Owner)
			seededTotal := uint64(len(fx.IDs))

			// A page over the all-summarized fixture alone issues no
			// backfill (id-set) Scroll at all.
			rec.reset()
			baseItems, baseTotal, _, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: seededTotal})
			if err != nil {
				t.Fatalf("%s: base default list: %v", shape, err)
			}
			if baseTotal != seededTotal || uint64(len(baseItems)) != seededTotal {
				t.Fatalf("%s: base list returned %d items (total %d), want %d", shape, len(baseItems), baseTotal, seededTotal)
			}
			for i, call := range rec.snapshot() {
				if call.idSet {
					t.Errorf("%s: base list call %d recorded a backfill id-set Scroll over an all-summarized page — want none", shape, i)
				}
			}

			// Two edge-case records seeded directly through the public
			// Upsert path (never a raw *qdrant.Client), mirroring
			// listscheduled_oversized_test.go's/searchtwophase_oversized_test.go's
			// idiom: neither belongs to fx.IDs.
			withSummaryID := uuid.NewString()
			withSummaryContent := "extra record content, has a stored summary"
			if err := st.Upsert(ctx, store.Memory{
				ID: withSummaryID, Content: withSummaryContent, Summary: "extra stored summary",
				Scope: fx.Scope, Owner: fx.Owner, Actor: fx.Owner, CreatedAt: time.Now(),
			}, vec); err != nil {
				t.Fatalf("%s: seed with-summary record: %v", shape, err)
			}
			noSummaryID := uuid.NewString()
			noSummaryContent := "extra record content, has NO stored summary"
			if err := st.Upsert(ctx, store.Memory{
				ID: noSummaryID, Content: noSummaryContent,
				Scope: fx.Scope, Owner: fx.Owner, Actor: fx.Owner, CreatedAt: time.Now(),
			}, vec); err != nil {
				t.Fatalf("%s: seed no-summary record: %v", shape, err)
			}

			totalWithExtras := seededTotal + 2
			// requestLimit stays at or below store.MaxRecallLimit (D-10,
			// plan 04-05 Task 2): the ManySmall shape seeds exactly
			// store.MaxRecallLimit records, so totalWithExtras alone would
			// be refused as an over-maximum count. The two extras were
			// upserted with a later CreatedAt than every fixture record, so
			// a desc-ordered page capped at requestLimit still contains
			// both — only the oldest fixture records (never inspected by
			// this test) would ever be dropped.
			requestLimit := totalWithExtras
			if requestLimit > store.MaxRecallLimit {
				requestLimit = store.MaxRecallLimit
			}
			wantCount := requestLimit

			rec.reset()
			defItems, defTotal, _, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: requestLimit})
			if err != nil {
				t.Fatalf("%s: default list with extras: %v", shape, err)
			}
			if defTotal != totalWithExtras || uint64(len(defItems)) != wantCount {
				t.Fatalf("%s: default list with extras returned %d items (total %d), want %d items (total %d)", shape, len(defItems), defTotal, wantCount, totalWithExtras)
			}
			var sawBackfill bool
			for _, call := range rec.snapshot() {
				if call.idSet {
					sawBackfill = true
				}
			}
			if !sawBackfill {
				t.Errorf("%s: default list with a no-summary record recorded no backfill id-set Scroll, want at least one", shape)
			}

			defByID := make(map[string]store.Memory, len(defItems))
			for _, m := range defItems {
				defByID[m.ID] = m
			}
			if got := defByID[noSummaryID].Content; got != noSummaryContent {
				t.Errorf("%s: default list: no-summary record content = %q, want %q (backfill must restore it)", shape, got, noSummaryContent)
			}
			if got := defByID[withSummaryID].Content; got != "" {
				t.Errorf("%s: default list: with-summary record content = %q, want empty (content was never fetched, never backfilled)", shape, got)
			}

			rec.reset()
			fullItems, fullTotal, _, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: requestLimit, Full: true})
			if err != nil {
				t.Fatalf("%s: full list with extras: %v", shape, err)
			}
			if fullTotal != totalWithExtras || uint64(len(fullItems)) != wantCount {
				t.Fatalf("%s: full list with extras returned %d items (total %d), want %d items (total %d)", shape, len(fullItems), fullTotal, wantCount, totalWithExtras)
			}
			fullByID := make(map[string]store.Memory, len(fullItems))
			for _, m := range fullItems {
				fullByID[m.ID] = m
			}
			if got := fullByID[noSummaryID].Content; got != noSummaryContent {
				t.Errorf("%s: full list: no-summary record content = %q, want %q", shape, got, noSummaryContent)
			}
			if got := fullByID[withSummaryID].Content; got != withSummaryContent {
				t.Errorf("%s: full list: with-summary record content = %q, want %q", shape, got, withSummaryContent)
			}

			if len(defItems) != len(fullItems) {
				t.Fatalf("%s: default list returned %d items, full list returned %d", shape, len(defItems), len(fullItems))
			}
			for i := range defItems {
				if defItems[i].ID != fullItems[i].ID {
					t.Errorf("%s: item %d id mismatch: default=%s full=%s (ids/order must be projection-independent)", shape, i, defItems[i].ID, fullItems[i].ID)
				}
			}
		})
	}
}
