// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-list-bounded's real-Qdrant regressions for Store.List
// against both oversized fixture shapes storetest.SeedOversized supports, at
// the named storetest.RecvLimit — OUR request shape and OUR page contract,
// never Qdrant's or grpc-go's own behavior (rule m45p2b4bp7). Task 1 proves
// cursor mode (TestStoreListCursorBounded); Task 2 extends this file with
// TestStoreListOffsetBounded.
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// TestStoreListCursorBounded runs one subtest per fixture shape, few-large
// then many-small: it walks Store.List in cursor mode (ListOptions{Limit:
// 50, CursorMode: true}, resuming with each returned cursor) until the
// cursor comes back empty, asserting every call succeeds, total is exact on
// every page, every seeded id is visited exactly once across the walk,
// created_at never increases across the concatenated walk, and a non-empty
// cursor is returned on every page but the last. It then proves the D-06
// budget-cut page: shrinking the page byte budget below one full-view
// record ceiling still returns a page with a non-empty cursor and fewer
// items than requested.
func TestStoreListCursorBounded(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_listbounded_cursor_" + uuid.NewString())
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
			owner := store.Authenticated(fx.Owner)

			visited := map[string]int{}
			var lastCreatedAt time.Time
			haveLast := false
			cursor := ""
			var gotTotal uint64
			for i := 0; i < 200; i++ {
				items, total, next, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: 50, Cursor: cursor, CursorMode: true})
				if err != nil {
					t.Fatalf("page %d: List: %v", i, err)
				}
				gotTotal = total
				if total != uint64(len(fx.IDs)) {
					t.Errorf("page %d: total = %d, want %d", i, total, len(fx.IDs))
				}
				for _, m := range items {
					visited[m.ID]++
					if visited[m.ID] > 1 {
						t.Fatalf("page %d: id %s visited twice", i, m.ID)
					}
					if haveLast && m.CreatedAt.After(lastCreatedAt) {
						t.Errorf("page %d: created_at increased: %v after %v", i, m.CreatedAt, lastCreatedAt)
					}
					lastCreatedAt = m.CreatedAt
					haveLast = true
				}
				if next == "" {
					break
				}
				cursor = next
			}
			if gotTotal != uint64(len(fx.IDs)) {
				t.Errorf("final total = %d, want %d", gotTotal, len(fx.IDs))
			}
			if len(visited) != len(fx.IDs) {
				t.Errorf("visited %d distinct ids, want %d", len(visited), len(fx.IDs))
			}
			for _, id := range fx.IDs {
				if visited[id] != 1 {
					t.Errorf("id %s visited %d time(s), want exactly 1", id, visited[id])
				}
			}

			// D-06 budget-cut page: shrinking the page byte budget below one
			// full-view record ceiling still returns a page with a
			// non-empty cursor and fewer items than requested — never the
			// last page. Inlined (no nested t.Run) so this file's verify
			// command's per-shape "--- PASS:" count stays exactly 2.
			store.SetByteBudgets(t, store.RPCByteBudget(), store.ViewMaxRecordBytes(st.FullView())/2)
			cutItems, cutTotal, cutNext, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: uint64(len(fx.IDs)), CursorMode: true})
			if err != nil {
				t.Fatalf("budget-cut List: %v", err)
			}
			if cutTotal != uint64(len(fx.IDs)) {
				t.Errorf("budget-cut total = %d, want %d", cutTotal, len(fx.IDs))
			}
			if len(cutItems) >= len(fx.IDs) {
				t.Errorf("budget-cut: got %d items, want fewer than the requested %d", len(cutItems), len(fx.IDs))
			}
			if cutNext == "" {
				t.Error("budget-cut: next cursor is empty, want non-empty (a budget-cut page is never the last page)")
			}
		})
	}
}
