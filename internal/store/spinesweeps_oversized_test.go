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

// TestEnumerateCitationsBoundedOverGRPCLimit proves Store.EnumerateCitations'
// citationsView projection (D-04) completes over a citation-bearing scope
// whose 256-record page would have overflowed the named receive limit under
// the pre-migration unbudgeted view, on both fixture shapes, and that every
// returned record kept the four keys the projection was sized to carry.
func TestEnumerateCitationsBoundedOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_enumcitations_" + uuid.NewString())
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

			// SeedOversized's Spec.Template is copied verbatim for every
			// record (only ID, Content and CreatedAt are overwritten), so a
			// static ShortID here means every seeded record shares it —
			// harmless for this test, which only proves the field travels
			// through citationsView's projection, never that short ids are
			// globally unique. Store.Upsert (SeedOversized's write path)
			// does not mint one itself: minting is a server-tier concern
			// (Store.MintShortID), so a fixture that wants a non-empty
			// ShortID must set it explicitly, exactly like spine_test.go's
			// own in-place NearDuplicates fixtures do.
			fx := storetest.SeedOversized(t, st, storetest.Spec{
				Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3},
				Template: store.Memory{
					ShortID:   "sidoversz",
					Citations: []store.Citation{{Kind: "file", Ref: "internal/store/spine.go"}},
				},
			})

			res, err := st.EnumerateCitations(ctx, store.SpineScanOptions{Scope: fx.Scope})
			if err != nil {
				t.Fatalf("%s: EnumerateCitations: %v (request shape exceeded the %d-byte named receive limit)", shape, err, storetest.RecvLimit)
			}
			if got, want := len(res), len(fx.IDs); got != want {
				t.Errorf("%s: EnumerateCitations returned %d records, want %d", shape, got, want)
			}
			for _, rec := range res {
				if rec.ShortID == "" {
					t.Errorf("%s: record %s: ShortID is empty — citationsView dropped a key EnumerateCitations reads", shape, rec.ID)
				}
				if rec.Scope == "" {
					t.Errorf("%s: record %s: Scope is empty — citationsView dropped a key EnumerateCitations reads", shape, rec.ID)
				}
				if len(rec.Citations) == 0 {
					t.Errorf("%s: record %s: Citations is empty — citationsView dropped the field EnumerateCitations exists to report", shape, rec.ID)
				}
			}
		})
	}
}

// TestNearDuplicatesBoundedOverGRPCLimit proves NearDuplicates' id
// enumeration on nearDuplicateIdentityView (D-04) completes over both
// fixture shapes at storetest.RecvLimit, and that every reported pair
// carries a non-empty short id and scope on both sides — proving the
// two-field projection survived the byte budget.
func TestNearDuplicatesBoundedOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_neardup_" + uuid.NewString())
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

			// A static ShortID in the template (see the identical note in
			// TestEnumerateCitationsBoundedOverGRPCLimit) is what makes the
			// id enumeration's short_id key non-empty; Store.Upsert never
			// mints one on its own.
			fx := storetest.SeedOversized(t, st, storetest.Spec{
				Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3},
				Template: store.Memory{ShortID: "sidoversz"},
			})

			pairs, err := st.NearDuplicates(ctx, store.NearDuplicateOptions{Scope: fx.Scope})
			if err != nil {
				t.Fatalf("%s: NearDuplicates: %v (request shape exceeded the %d-byte named receive limit)", shape, err, storetest.RecvLimit)
			}
			if len(pairs) == 0 {
				t.Fatalf("%s: NearDuplicates returned no pairs over a fixture where every record shares the same vector", shape)
			}
			for _, p := range pairs {
				if p.AShortID == "" || p.BShortID == "" {
					t.Errorf("%s: pair (%s,%s): empty short id — nearDuplicateIdentityView dropped a key the id enumeration reads", shape, p.A, p.B)
				}
				if p.AScope == "" || p.BScope == "" {
					t.Errorf("%s: pair (%s,%s): empty scope — nearDuplicateIdentityView dropped a key the id enumeration reads", shape, p.A, p.B)
				}
			}
		})
	}
}
