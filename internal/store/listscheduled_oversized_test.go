// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-list-scheduled-bounded's real-Qdrant regression for
// Store.ListScheduled against both oversized fixture shapes
// storetest.SeedOversized supports, at the named storetest.RecvLimit — OUR
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
	"google.golang.org/grpc"
)

// TestListScheduledBounded proves Store.ListScheduled's bounded assembly
// (D-05) over both oversized fixture shapes storetest.SeedOversized
// supports: a call at store.MaxRecallLimit succeeds and returns every seeded
// scheduled record exactly once, with every recorded Scroll's limit staying
// within store.PerRPCLimit of the full view; a zero-limit call still
// returns twenty, unchanged from today; a currently-active (already past
// its not_before) windowed record never appears; a superseded and an
// archived scheduled record never appear; another owner's shared scheduled
// record never appears to an authenticated caller; and a creation-time
// window narrows the result to the expected subset on every assembled RPC,
// not just the first.
func TestListScheduledBounded(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			rec := &scrollRecorder{}
			c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rec.intercept))
			name := store.PrefixedTestCollection("oversized_listscheduled_" + uuid.NewString())
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

			future := time.Now().Add(24 * time.Hour)
			fx := storetest.SeedOversized(t, st, storetest.Spec{
				Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3},
				Template: store.Memory{NotBefore: &future},
			})
			owner := store.Authenticated(fx.Owner)
			seededTotal := uint64(len(fx.IDs))

			// Edge-case records, seeded directly through the public Upsert
			// path (never a raw *qdrant.Client) so each can carry a state
			// SeedOversized's uniform Template cannot: a currently-active
			// windowed record (already past its own not_before), a
			// superseded scheduled record, an archived scheduled record,
			// and another owner's shared scheduled record. None belong to
			// fx.IDs and none should ever surface from ListScheduled.
			edgeVector := []float32{0.4, 0.5, 0.6}
			past := time.Now().Add(-1 * time.Hour)

			activeID := uuid.NewString()
			if err := st.Upsert(ctx, store.Memory{
				ID: activeID, Content: "active", Scope: fx.Scope, Owner: fx.Owner,
				Actor: fx.Owner, CreatedAt: time.Now(), NotBefore: &past,
			}, edgeVector); err != nil {
				t.Fatalf("%s: seed active windowed record: %v", shape, err)
			}

			supersededID := uuid.NewString()
			supersededBy := uuid.NewString()
			if err := st.Upsert(ctx, store.Memory{
				ID: supersededID, Content: "superseded", Scope: fx.Scope, Owner: fx.Owner,
				Actor: fx.Owner, CreatedAt: time.Now(), NotBefore: &future,
				SupersededBy: &supersededBy,
			}, edgeVector); err != nil {
				t.Fatalf("%s: seed superseded scheduled record: %v", shape, err)
			}

			archivedID := uuid.NewString()
			archivedAt := time.Now()
			if err := st.Upsert(ctx, store.Memory{
				ID: archivedID, Content: "archived", Scope: fx.Scope, Owner: fx.Owner,
				Actor: fx.Owner, CreatedAt: time.Now(), NotBefore: &future,
				ArchivedAt: &archivedAt,
			}, edgeVector); err != nil {
				t.Fatalf("%s: seed archived scheduled record: %v", shape, err)
			}

			otherOwner := "storetest-owner-" + uuid.NewString()
			sharedID := uuid.NewString()
			if err := st.Upsert(ctx, store.Memory{
				ID: sharedID, Content: "shared", Scope: fx.Scope, Owner: otherOwner,
				Actor: otherOwner, Visibility: "shared", CreatedAt: time.Now(), NotBefore: &future,
			}, edgeVector); err != nil {
				t.Fatalf("%s: seed another owner's shared scheduled record: %v", shape, err)
			}
			t.Cleanup(func() {
				if delErr := st.DeleteAll(context.Background(), fx.Scope, store.Authenticated(otherOwner)); delErr != nil {
					t.Errorf("%s: cleanup other owner's record: %v", shape, delErr)
				}
			})

			// A call at the maximum returns every seeded record exactly
			// once, no extras, with every RPC's limit staying inside
			// PerRPCLimit of the full view.
			rec.reset()
			items, err := st.ListScheduled(ctx, fx.Scope, owner, store.ScheduledPending, store.ListOptions{Limit: store.MaxRecallLimit})
			if err != nil {
				t.Fatalf("%s: ListScheduled(max): %v", shape, err)
			}
			seen := map[string]int{}
			for _, m := range items {
				seen[m.ID]++
			}
			for _, id := range fx.IDs {
				if seen[id] != 1 {
					t.Errorf("%s: seeded id %s appeared %d time(s), want exactly 1", shape, id, seen[id])
				}
			}
			if uint64(len(items)) != seededTotal {
				t.Errorf("%s: ListScheduled(max) returned %d items, want exactly %d (no extras)", shape, len(items), seededTotal)
			}
			for _, m := range items {
				if m.ID == activeID || m.ID == supersededID || m.ID == archivedID || m.ID == sharedID {
					t.Errorf("%s: ListScheduled(max) returned excluded id %s", shape, m.ID)
				}
			}
			fullPerRPC := uint32(store.PerRPCLimit(st.FullView()))
			for i, call := range rec.snapshot() {
				if call.limit < 1 || call.limit > fullPerRPC {
					t.Errorf("%s: call %d limit = %d, want between 1 and %d", shape, i, call.limit, fullPerRPC)
				}
			}

			// Zero limit still yields twenty, unchanged from today.
			zItems, zErr := st.ListScheduled(ctx, fx.Scope, owner, store.ScheduledPending, store.ListOptions{})
			if zErr != nil {
				t.Fatalf("%s: ListScheduled(zero-limit): %v", shape, zErr)
			}
			if len(zItems) != 20 {
				t.Errorf("%s: ListScheduled(zero-limit) returned %d items, want 20", shape, len(zItems))
			}

			// A creation-time window narrows the result to the expected
			// subset on every assembled RPC, not just the first: fx's
			// records are strictly increasing one second apart. Derive the
			// boundary from the SAME method under test (an unbounded
			// desc-ordered fetch) rather than re-deriving storetest's
			// internal base-timestamp math.
			half := seededTotal / 2
			if half == 0 {
				half = 1
			}
			allItems, allErr := st.ListScheduled(ctx, fx.Scope, owner, store.ScheduledPending, store.ListOptions{Limit: store.MaxRecallLimit})
			if allErr != nil {
				t.Fatalf("%s: ListScheduled(all, for window boundary): %v", shape, allErr)
			}
			if uint64(len(allItems)) != seededTotal {
				t.Fatalf("%s: ListScheduled(all, for window boundary) returned %d items, want %d", shape, len(allItems), seededTotal)
			}
			boundary := allItems[half-1].CreatedAt
			wantIDs := map[string]bool{}
			for _, m := range allItems[:half] {
				wantIDs[m.ID] = true
			}
			windowItems, wErr := st.ListScheduled(ctx, fx.Scope, owner, store.ScheduledPending, store.ListOptions{Limit: store.MaxRecallLimit, CreatedAfter: boundary})
			if wErr != nil {
				t.Fatalf("%s: ListScheduled(window): %v", shape, wErr)
			}
			if uint64(len(windowItems)) != half {
				t.Errorf("%s: ListScheduled(window) returned %d items, want %d", shape, len(windowItems), half)
			}
			for _, m := range windowItems {
				if !wantIDs[m.ID] {
					t.Errorf("%s: ListScheduled(window) returned id %s outside the expected newest-%d subset", shape, m.ID, half)
				}
			}
		})
	}
}
