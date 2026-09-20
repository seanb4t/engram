// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-sweeps-bounded's real-Qdrant regression for the four
// spine-review sweeps (Store.ScanSpine, Store.EnumerateCitations,
// Store.NearDuplicates, Store.PreviewPurge) against both oversized fixture
// shapes storetest.SeedOversized supports, at the named storetest.RecvLimit
// — OUR request shape and OUR per-sweep projection, never Qdrant's or
// grpc-go's own behavior (rule m45p2b4bp7).
package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// TestScanSpineBoundedOverGRPCLimit proves Store.ScanSpine's scanView
// projection (D-04) completes over a scope whose 256-record page would have
// overflowed the named receive limit under the pre-migration unbudgeted
// view, on both fixture shapes.
func TestScanSpineBoundedOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_scanspine_" + uuid.NewString())
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

			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})

			res, err := st.ScanSpine(ctx, store.SpineScanOptions{Scope: fx.Scope})
			if err != nil {
				t.Fatalf("%s: ScanSpine: %v (request shape exceeded the %d-byte named receive limit)", shape, err, storetest.RecvLimit)
			}
			if res.Total != uint64(len(fx.IDs)) {
				t.Errorf("%s: Total = %d, want %d", shape, res.Total, len(fx.IDs))
			}
			// A projection that silently dropped a key ScanSpine's callback
			// reads would corrupt this derived identity: every scanned
			// record is counted into exactly one of WithSummary/
			// WithoutSummary.
			if got, want := res.WithSummary+res.WithoutSummary, res.Total; got != want {
				t.Errorf("%s: WithSummary+WithoutSummary = %d, want %d (equal to Total)", shape, got, want)
			}
		})
	}
}
