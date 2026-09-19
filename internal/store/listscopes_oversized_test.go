// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package store_test hosts the migrated #583 regression test
// (TestListScopesFullPayloadsOverGRPCLimit): it proves ListScopes' request
// shape against real Qdrant with the receive limit NAMED by the test
// (storetest.RecvLimit, D-04) — covering OUR request shape, never grpc-go's
// or Qdrant's own behavior (rule m45p2b4bp7). It now covers both fixture
// shapes storetest.SeedOversized supports; the -short skip is inherited from
// the seeder (D-12).
package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// TestListScopesFullPayloadsOverGRPCLimit runs one subtest per fixture
// shape, FewLarge then ManySmall, named by Shape.String(). Each dials with
// the named receive limit (storetest.RecvLimit), seeds a fresh collection
// at that same named limit via storetest.SeedOversized, and asserts every
// seeded record is counted by ListScopes.
func TestListScopesFullPayloadsOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_listscopes_" + uuid.NewString())
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

			scopes, _, err := st.ListScopes(ctx, store.Authenticated(fx.Owner))
			if err != nil {
				t.Fatalf("ListScopes: %v (full-payload scroll exceeded the %d-byte named receive limit)", err, storetest.RecvLimit)
			}
			counts := map[string]uint64{}
			for _, sc := range scopes {
				counts[sc.Scope] = sc.Count
			}
			if want := uint64(len(fx.IDs)); counts[fx.Scope] != want {
				t.Errorf("counts[%s] = %d, want %d", fx.Scope, counts[fx.Scope], want)
			}
		})
	}
}

// TestManySmallShapeFitsOneListPage pins storetest.ManySmallRecords against
// internal/store's own maxListLimit (exposed as store.MaxListLimit), so the
// ManySmall shape always fits within one List page (D-05).
func TestManySmallShapeFitsOneListPage(t *testing.T) {
	if storetest.ManySmallRecords != store.MaxListLimit {
		t.Errorf("storetest.ManySmallRecords = %d, want store.MaxListLimit (%d)", storetest.ManySmallRecords, store.MaxListLimit)
	}
}
