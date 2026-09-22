// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-sweeps-bounded's real-Qdrant regression for
// Store.Revert's whole-range preflight (previewRevertWithSteps), the last of
// Phase 5's migrated sweeps: OUR request shape stays bounded on
// schemaVersionOnlyView's one-field projection over an above-target range
// whose 256-record page would have overflowed the named receive limit under
// the pre-migration unbudgeted view, on both fixture shapes — never that
// gRPC enforces a ceiling (rule m45p2b4bp7).
package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// TestRevertPreviewBoundedOverGRPCLimit proves the revert preflight
// completes over an oversized above-target scope on both fixture shapes and
// reaches the identical whole-range refusal verdict Store.Revert always
// returns against the production migrate.Registry's single Irreversible
// v0->v1 step: every seeded record's schema_version is migrate.CurrentVersion
// (stamped by Store.Upsert on write), aboveTargetFilter(0) matches all of
// them, and the preflight's own reverse-chain lookup finds that step
// Irreversible — never Unsupported, since the chain IS reachable, it simply
// declines to run backward. Asserting plan.Irreversible[0].To equals the
// seeded schema version is what proves schemaVersionOnlyView's projection
// still delivered the one field the preflight reads, on every page.
func TestRevertPreviewBoundedOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_revertpreview_" + uuid.NewString())
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

			// A fresh, per-test collection: the fixture is the ONLY content
			// in it, so aboveTargetFilter(0)'s collection-wide (not
			// scope-scoped) match still lands exactly on the seeded records
			// — plan.Candidates below is checked against len(fx.IDs), not
			// an approximation.
			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})

			_, err := st.Revert(ctx, 0)
			if err == nil {
				t.Fatalf("%s: Revert: got nil error, want a whole-range refusal (request shape exceeded the %d-byte named receive limit, or the fixture is unexpectedly reversible)", shape, storetest.RecvLimit)
			}
			var refused *store.RevertRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("%s: Revert error %v is not a *store.RevertRefusedError (request shape exceeded the %d-byte named receive limit)", shape, err, storetest.RecvLimit)
			}
			plan := refused.Plan
			if plan.To != 0 {
				t.Errorf("%s: plan.To = %d, want 0", shape, plan.To)
			}
			if plan.Candidates != uint64(len(fx.IDs)) {
				t.Errorf("%s: plan.Candidates = %d, want %d (schemaVersionOnlyView must still count every above-target record on every page)", shape, plan.Candidates, len(fx.IDs))
			}
			if plan.Reversible {
				t.Errorf("%s: plan.Reversible = true, want false (production migrate.Registry's v0->v1 step is Irreversible)", shape)
			}
			if len(plan.Irreversible) != 1 {
				t.Fatalf("%s: len(plan.Irreversible) = %d, want 1", shape, len(plan.Irreversible))
			}
			if plan.Irreversible[0].From != 0 || plan.Irreversible[0].To != int(migrate.CurrentVersion) {
				t.Errorf("%s: plan.Irreversible[0] = %+v, want From=0 To=%d (the seeded records' current schema version)", shape, plan.Irreversible[0], migrate.CurrentVersion)
			}
			if len(plan.Unsupported) != 0 {
				t.Errorf("%s: plan.Unsupported = %+v, want empty (the reverse chain is reachable, only declared Irreversible)", shape, plan.Unsupported)
			}
		})
	}
}
