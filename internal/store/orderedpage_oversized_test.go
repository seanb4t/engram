// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves the ordered-page primitive (orderedpage.go) against both
// oversized fixture shapes storetest.SeedOversized supports, at the named
// storetest.RecvLimit — OUR request shape and OUR page contract, never
// Qdrant's or grpc-go's own behavior (rule m45p2b4bp7). It reuses
// scrollRecorder (boundedread_oversized_test.go, same package) rather than
// redefining it.
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/qdrant/go-client/qdrant"
)

// TestScrollOrderedPageByteBudget walks both fixture shapes and both views,
// asserting the ordered-page contract per page (no error; Bytes within
// PageByteBudget; CutByBudget and Exhausted never both true; a cut page
// holds fewer items than the limit), across pages (no id twice; the union
// equals fx.IDs; CreatedAt never increases), and per recorded RPC (Limit
// within [1, PerRPCLimit(view)]; response within RPCByteBudget; no
// ResourceExhausted). In full view the first page is cut by budget; in
// summary view the first page holds the whole fixture and the following
// call reports Exhausted with zero items.
func TestScrollOrderedPageByteBudget(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		shape := shape
		t.Run(shape.String(), func(t *testing.T) {
			rec := &scrollRecorder{}
			c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
			name := store.PrefixedTestCollection("oversized_orderedpage_" + uuid.NewString())
			st := store.NewTestStore(t, c, name)
			ctx := context.Background()
			if err := st.EnsureCollection(ctx, 3); err != nil {
				t.Fatalf("EnsureCollection: %v", err)
			}
			t.Cleanup(func() {
				if err := c.DeleteCollection(ctx, name); err != nil {
					t.Errorf("DeleteCollection(%q): %v", name, err)
				}
			})

			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})

			subtests := []struct {
				name string
				view store.ReadView
			}{
				{"full", st.FullView()},
				{"summary", st.SummaryView()},
			}
			for _, sub := range subtests {
				sub := sub
				t.Run(sub.name, func(t *testing.T) {
					rec.reset()
					visited := map[string]int{}
					var lastCreatedAt time.Time
					haveLast := false
					exhaustedSeen := false

					from := store.ListCursor{}
					for i := 0; i < 200; i++ {
						page, err := st.ScrollOrderedPage(ctx, fx.Scope, store.Authenticated(fx.Owner), sub.view, qdrant.Direction_Desc, from, uint64(len(fx.IDs)))
						if err != nil {
							t.Fatalf("page %d: ScrollOrderedPage: %v", i, err)
						}
						if page.Bytes > store.PageByteBudget() {
							t.Errorf("page %d: Bytes = %d, exceeds PageByteBudget %d", i, page.Bytes, store.PageByteBudget())
						}
						if page.CutByBudget && page.Exhausted {
							t.Errorf("page %d: CutByBudget and Exhausted both true", i)
						}
						if page.CutByBudget && uint64(len(page.Items)) >= uint64(len(fx.IDs)) {
							t.Errorf("page %d: cut page has %d items, want fewer than limit %d", i, len(page.Items), len(fx.IDs))
						}

						if i == 0 && sub.name == "full" && !page.CutByBudget {
							t.Error("full: first page CutByBudget = false, want true")
						}
						if i == 0 && sub.name == "summary" {
							if page.CutByBudget {
								t.Error("summary: first page CutByBudget = true, want false")
							}
							if page.Exhausted {
								t.Error("summary: first page Exhausted = true, want false")
							}
							if len(page.Items) != len(fx.IDs) {
								t.Errorf("summary: first page holds %d items, want %d", len(page.Items), len(fx.IDs))
							}
						}
						if i == 1 && sub.name == "summary" {
							if len(page.Items) != 0 {
								t.Errorf("summary: second page holds %d items, want 0", len(page.Items))
							}
							if !page.Exhausted {
								t.Error("summary: second page Exhausted = false, want true")
							}
						}

						for _, m := range page.Items {
							visited[m.ID]++
							if visited[m.ID] > 1 {
								t.Fatalf("page %d: id %s visited twice", i, m.ID)
							}
							if haveLast && m.CreatedAt.After(lastCreatedAt) {
								t.Errorf("page %d: CreatedAt increased: %v after %v", i, m.CreatedAt, lastCreatedAt)
							}
							lastCreatedAt = m.CreatedAt
							haveLast = true
						}

						if page.Exhausted {
							exhaustedSeen = true
							break
						}
						from = page.Next
					}
					if !exhaustedSeen {
						t.Fatal("did not reach Exhausted within 200 pages")
					}

					if len(visited) != len(fx.IDs) {
						t.Errorf("visited %d distinct ids, want %d", len(visited), len(fx.IDs))
					}
					for _, id := range fx.IDs {
						if visited[id] != 1 {
							t.Errorf("id %s visited %d time(s), want exactly 1", id, visited[id])
						}
					}

					wantMaxLimit := uint32(store.PerRPCLimit(sub.view))
					for i, call := range rec.snapshot() {
						if call.limit < 1 || call.limit > wantMaxLimit {
							t.Errorf("call %d: Limit = %d, want between 1 and %d", i, call.limit, wantMaxLimit)
						}
						if call.respBytes > store.RPCByteBudget() {
							t.Errorf("call %d: response %d bytes exceeds RPCByteBudget %d", i, call.respBytes, store.RPCByteBudget())
						}
						if call.code == codes.ResourceExhausted {
							t.Errorf("call %d: ended ResourceExhausted", i)
						}
					}
				})
			}
		})
	}
}
