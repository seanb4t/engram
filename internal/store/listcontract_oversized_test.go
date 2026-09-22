// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-list-contract-unchanged's total / next_cursor /
// ordering / recall-gating invariance suite, independent of Store.List's
// internal batching, over both oversized fixture shapes
// storetest.SeedOversized supports at the named storetest.RecvLimit — OUR
// request shape and OUR page contract, never Qdrant's or grpc-go's own
// behavior (rule m45p2b4bp7).
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// TestStoreListContractInvariant runs one subtest per fixture shape,
// few-large then many-small, proving the contract properties
// REQ-list-contract-unchanged names hold independent of the internal
// batching this phase introduced:
//
//   - total is the exact seeded count on every cursor page and on an offset
//     call, and is NOT reduced by store.MaxRecallLimit even when the
//     seeded count is at it (the many-small shape).
//   - A cursor walk driven to completion visits every seeded id exactly
//     once and ends with an empty cursor; the LAST page is the only page
//     with an empty cursor.
//   - A cursor call over a shrunken page byte budget (requested with
//     ListOptions.Full: true, 04-05, so the budget shrunk against
//     st.FullView()'s own ceiling actually forces a cut — this test's
//     subject is the D-06 budget-cut contract, independent of 04-05's
//     separate default-projection change) returns a short page AND a
//     non-empty cursor, and resuming from that cursor still reaches every
//     remaining id — a byte-cut page is never the last page.
//   - The concatenated cursor walk and an equivalent offset call agree on
//     the SET of ids, and both have a non-increasing created_at sequence
//     (the relative order of two records sharing one created_at is
//     Qdrant's, out of contract per rule m45p2b4bp7 — never asserted here).
//   - Recall gating: a superseded and an archived record seeded into the
//     same scope appear in neither mode and do not affect total.
func TestStoreListContractInvariant(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_listcontract_" + uuid.NewString())
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
			seededTotal := uint64(len(fx.IDs))

			// Recall gating fixtures: a superseded and an archived record
			// in the same scope must appear in neither mode, and must not
			// affect total.
			otherID := "z0000000-0000-0000-0000-000000000000"
			supersededID := uuid.NewString()
			if err := st.Upsert(ctx, store.Memory{
				ID: supersededID, Content: "x", Scope: fx.Scope, Owner: fx.Owner, Actor: fx.Owner,
				Category: "decision", CreatedAt: time.Now().UTC(), SupersededBy: &otherID,
			}, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("seed superseded record: %v", err)
			}
			archivedID := uuid.NewString()
			archivedAt := time.Now().UTC()
			if err := st.Upsert(ctx, store.Memory{
				ID: archivedID, Content: "x", Scope: fx.Scope, Owner: fx.Owner, Actor: fx.Owner,
				Category: "decision", CreatedAt: time.Now().UTC(), ArchivedAt: &archivedAt,
			}, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("seed archived record: %v", err)
			}
			hidden := map[string]bool{supersededID: true, archivedID: true}

			// Cursor walk to completion.
			visited := map[string]int{}
			var cursorLastCreatedAt time.Time
			haveCursorLast := false
			cursor := ""
			emptyCursorPages := 0
			var cursorTotal uint64
			for i := 0; i < 200; i++ {
				items, total, next, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: 50, Cursor: cursor, CursorMode: true})
				if err != nil {
					t.Fatalf("cursor page %d: List: %v", i, err)
				}
				cursorTotal = total
				if total != seededTotal {
					t.Errorf("cursor page %d: total = %d, want %d", i, total, seededTotal)
				}
				if next == "" {
					emptyCursorPages++
				}
				for _, m := range items {
					if hidden[m.ID] {
						t.Errorf("cursor page %d: hidden id %s appeared", i, m.ID)
					}
					visited[m.ID]++
					if visited[m.ID] > 1 {
						t.Fatalf("cursor page %d: id %s visited twice", i, m.ID)
					}
					if haveCursorLast && m.CreatedAt.After(cursorLastCreatedAt) {
						t.Errorf("cursor page %d: created_at increased: %v after %v", i, m.CreatedAt, cursorLastCreatedAt)
					}
					cursorLastCreatedAt = m.CreatedAt
					haveCursorLast = true
				}
				if next == "" {
					break
				}
				cursor = next
			}
			if cursorTotal != seededTotal {
				t.Errorf("final cursor total = %d, want %d", cursorTotal, seededTotal)
			}
			if emptyCursorPages != 1 {
				t.Errorf("saw %d page(s) with an empty next cursor, want exactly 1 (the last page)", emptyCursorPages)
			}
			if len(visited) != len(fx.IDs) {
				t.Errorf("cursor walk visited %d distinct ids, want %d", len(visited), len(fx.IDs))
			}
			for _, id := range fx.IDs {
				if visited[id] != 1 {
					t.Errorf("id %s visited %d time(s) by cursor walk, want exactly 1", id, visited[id])
				}
			}

			// Budget-cut cursor page: never the last page, and resuming
			// still reaches every remaining id. Restored immediately after
			// use (rather than deferred to subtest end) so the ordering +
			// offset-equivalence check below runs at the default budget.
			origPageBudget := store.PageByteBudget()
			store.SetByteBudgets(t, store.RPCByteBudget(), store.ViewMaxRecordBytes(st.FullView())/2)
			cutItems, cutTotal, cutNext, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: seededTotal, CursorMode: true, Full: true})
			if err != nil {
				t.Fatalf("budget-cut List: %v", err)
			}
			if cutTotal != seededTotal {
				t.Errorf("budget-cut total = %d, want %d", cutTotal, seededTotal)
			}
			if uint64(len(cutItems)) >= seededTotal {
				t.Errorf("budget-cut got %d items, want fewer than the requested %d", len(cutItems), seededTotal)
			}
			if cutNext == "" {
				t.Fatal("budget-cut next cursor is empty, want non-empty (a budget-cut page is never the last page)")
			}
			resumeVisited := map[string]bool{}
			for _, m := range cutItems {
				resumeVisited[m.ID] = true
			}
			resumeCursor := cutNext
			// Bound generously: a severely shrunken page byte budget can
			// force the ceiling-based per-record accounting down to a
			// single item per page, so a 1000-record fixture (many-small)
			// needs up to ~1000 resume calls to reach exhaustion.
			for i := 0; i < int(seededTotal)+10 && resumeCursor != ""; i++ {
				items, _, next, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: seededTotal, Cursor: resumeCursor, CursorMode: true})
				if err != nil {
					t.Fatalf("resume page %d: List: %v", i, err)
				}
				for _, m := range items {
					resumeVisited[m.ID] = true
				}
				resumeCursor = next
			}
			for _, id := range fx.IDs {
				if !resumeVisited[id] {
					t.Errorf("id %s never reached after resuming from the budget-cut cursor", id)
				}
			}
			store.SetByteBudgets(t, store.RPCByteBudget(), origPageBudget)

			// Ordering + offset equivalence: the concatenated cursor walk
			// and an equivalent offset call agree on the SET of ids, and
			// both have a non-increasing created_at sequence.
			offsetItems, offsetTotal, offsetNext, err := st.List(ctx, fx.Scope, owner, store.ListOptions{Limit: seededTotal})
			if err != nil {
				t.Fatalf("offset List: %v", err)
			}
			if offsetTotal != seededTotal {
				t.Errorf("offset total = %d, want %d", offsetTotal, seededTotal)
			}
			if offsetNext != "" {
				t.Errorf("offset next = %q, want empty", offsetNext)
			}
			offsetIDs := map[string]bool{}
			var offsetLastCreatedAt time.Time
			haveOffsetLast := false
			for _, m := range offsetItems {
				if hidden[m.ID] {
					t.Errorf("offset: hidden id %s appeared", m.ID)
				}
				offsetIDs[m.ID] = true
				if haveOffsetLast && m.CreatedAt.After(offsetLastCreatedAt) {
					t.Errorf("offset: created_at increased: %v after %v", m.CreatedAt, offsetLastCreatedAt)
				}
				offsetLastCreatedAt = m.CreatedAt
				haveOffsetLast = true
			}
			if uint64(len(offsetIDs)) != seededTotal {
				t.Errorf("offset returned %d distinct ids, want %d", len(offsetIDs), seededTotal)
			}
			if uint64(len(visited)) != uint64(len(offsetIDs)) {
				t.Errorf("cursor walk visited %d ids, offset call returned %d — sets differ in size", len(visited), len(offsetIDs))
			}
			for id := range visited {
				if !offsetIDs[id] {
					t.Errorf("id %s visited by cursor walk but not returned by offset call", id)
				}
			}
		})
	}
}
