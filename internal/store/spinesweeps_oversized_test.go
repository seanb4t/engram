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
	"time"

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

// TestPreviewPurgeBoundedOverGRPCLimit proves derivePurgeEligible's reuse of
// summaryView (D-04) completes over both oversized fixture shapes at
// storetest.RecvLimit: the bulk fixture (none of it purge-eligible) leaves
// an empty manifest, and one additional record seeded through the public
// write path with Tags, Category, SupersededBy, NotAfter, ArchivedAt and
// CreatedAt all set is the manifest's only entry — proving every one of
// those keys survived the projection.
func TestPreviewPurgeBoundedOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_previewpurge_" + uuid.NewString())
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

			// The bulk fixture carries none of derivePurgeEligible's
			// eligibility fields (no NotAfter/ArchivedAt/SupersededBy), so
			// none of it is purge-eligible under any class.
			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})

			purgeNow := time.Now().UTC()
			pastNotAfter := purgeNow.Add(-2 * time.Hour)
			pastArchivedAt := purgeNow.Add(-100 * 24 * time.Hour)
			candidateID := uuid.NewString()
			// Seeded through the public Store.Upsert path (never a raw
			// *qdrant.Client), the way listscheduled_oversized_test.go
			// seeds its own edge-case records, so it can carry a state the
			// uniform Spec.Template cannot: SupersededBy names an already-
			// existing record (fx.IDs[0]) whose CreatedAt is recent
			// (satisfying checkExtractGate's per-record path, since it
			// postdates this candidate's own CreatedAt), while NotAfter
			// and ArchivedAt are both in the past relative to purgeNow,
			// making the record independently Expired- and Archived-
			// eligible.
			successorID := fx.IDs[0]
			if err := st.Upsert(ctx, store.Memory{
				ID: candidateID, Content: "purge candidate", Scope: fx.Scope, Owner: fx.Owner,
				Actor: fx.Owner, Category: "decision", Tags: []string{"important"},
				CreatedAt: purgeNow.Add(-3 * time.Hour), NotAfter: &pastNotAfter, ArchivedAt: &pastArchivedAt,
				SupersededBy: &successorID,
			}, []float32{0.4, 0.5, 0.6}); err != nil {
				t.Fatalf("%s: seed purge-eligible record: %v", shape, err)
			}

			manifest, err := st.PreviewPurge(ctx, store.PurgeOptions{
				Classes: []store.PurgeClass{store.PurgeClassExpired, store.PurgeClassArchived},
				Scope:   fx.Scope, OlderThan: time.Hour, Now: purgeNow,
			})
			if err != nil {
				t.Fatalf("%s: PreviewPurge: %v (request shape exceeded the %d-byte named receive limit, or the extract gate rejected the candidate)", shape, err, storetest.RecvLimit)
			}
			ids := manifest.IDs()
			if len(ids) != 1 || ids[0] != candidateID {
				t.Errorf("%s: PreviewPurge manifest = %v, want exactly [%s]", shape, ids, candidateID)
			}
		})
	}
}
