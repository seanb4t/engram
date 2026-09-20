// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-sweeps-bounded's real-Qdrant regressions for
// engram migrate's default sweep mode (TestMigrateBoundedOverGRPCLimit,
// task 1) and engram migrate revert's apply path
// (TestRevertApplyBoundedOverGRPCLimit, task 3): OUR request shape stays
// bounded through the shared byte-budget iterator over a scope whose
// 256-record page would have overflowed the named receive limit under the
// pre-migration nil-offset ScrollAndOffset call, on both fixture shapes —
// never that gRPC enforces a ceiling (rule m45p2b4bp7).
package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// migrateSweepBacklogFilter reconstructs backlogFilter(target)'s exact
// shape (internal/store/migratebacklog.go) from outside the package: a
// record strictly below target OR carrying no schema_version key at all.
// store.SchemaVersionKey() is the one-line seam that keeps this from
// repeating the field name as a bare string literal.
func migrateSweepBacklogFilter(target int) *qdrant.Filter {
	key := store.SchemaVersionKey()
	return &qdrant.Filter{
		Must: []*qdrant.Condition{
			qdrant.NewFilterAsCondition(&qdrant.Filter{
				Should: []*qdrant.Condition{
					qdrant.NewRange(key, &qdrant.Range{Lt: qdrant.PtrOf(float64(target))}),
					qdrant.NewIsEmpty(key),
				},
			}),
		},
	}
}

// independentIDScroll is a direct, single-page scroll of filter through the
// raw client — the same discipline internal/store/migrate_test.go's
// migrateBacklogIDs follows (an INDEPENDENT re-derivation of a backlog,
// never MigrateResult's/RevertResult's own counters). Payload is
// deliberately excluded: only identity is asserted here, so the single
// large page never approaches storetest.RecvLimit regardless of fixture
// shape.
func independentIDScroll(ctx context.Context, t *testing.T, c *qdrant.Client, collection string, filter *qdrant.Filter) []string {
	t.Helper()
	pts, _, err := c.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
		CollectionName: collection,
		Filter:         filter,
		Limit:          qdrant.PtrOf(uint32(2000)),
		WithPayload:    qdrant.NewWithPayload(false),
	})
	if err != nil {
		t.Fatalf("independentIDScroll(%q): %v", collection, err)
	}
	ids := make([]string, 0, len(pts))
	for _, p := range pts {
		ids = append(ids, p.Id.GetUuid())
	}
	return ids
}

// TestMigrateBoundedOverGRPCLimit proves engram migrate's default sweep
// mode (Store.Migrate with neither DryRun nor Manifest set) drains an
// oversized backlog through the shared byte-budget iterator, on both
// fixture shapes. Its dry-run projection arm is added by task 2, once
// Store.Migrate's DryRun walk is itself migrated onto the shared iterator
// — before that, a dry-run call at the DEFAULT batch over the FewLarge
// shape would genuinely overflow the receive limit (one 256-record page,
// still requesting full payload, returns all 40 large records at once),
// which is precisely the bug task 2 fixes, not task 1.
func TestMigrateBoundedOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_migratesweep_" + uuid.NewString())
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

			// SeedOversized's records are stamped migrate.CurrentVersion by
			// the ordinary Store.Upsert path they go through — strip that
			// key raw, producing the genuinely key-absent legacy shape
			// backlogFilter's IsEmpty arm matches, mirroring
			// seedLegacyRecord's own discipline (migrate_test.go).
			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})

			key := store.SchemaVersionKey()
			pointIDs := make([]*qdrant.PointId, len(fx.IDs))
			for i, id := range fx.IDs {
				pointIDs[i] = qdrant.NewID(id)
			}
			if _, err := c.DeletePayload(ctx, &qdrant.DeletePayloadPoints{
				CollectionName: name,
				Wait:           qdrant.PtrOf(true),
				Keys:           []string{key},
				PointsSelector: qdrant.NewPointsSelectorIDs(pointIDs),
			}); err != nil {
				t.Fatalf("%s: DeletePayload(%s): %v", shape, key, err)
			}

			// Assert the key really is gone before calling Migrate — a
			// seeding step that silently failed to strip it would make the
			// whole regression vacuous.
			before := independentIDScroll(ctx, t, c, name, migrateSweepBacklogFilter(int(migrate.CurrentVersion)))
			if len(before) != len(fx.IDs) {
				t.Fatalf("%s: independent backlog re-derivation before Migrate = %d ids, want %d (schema_version key not fully stripped)", shape, len(before), len(fx.IDs))
			}

			// Apply arm: default sweep mode drains the whole backlog.
			res, err := st.Migrate(ctx, store.MigrateOptions{})
			if err != nil {
				t.Fatalf("%s: Migrate: %v (request shape may have exceeded the %d-byte named receive limit)", shape, err, storetest.RecvLimit)
			}
			if res.Migrated != uint64(len(fx.IDs)) {
				t.Errorf("%s: res.Migrated = %d, want %d", shape, res.Migrated, len(fx.IDs))
			}
			if res.Failed != 0 {
				t.Errorf("%s: res.Failed = %d, want 0", shape, res.Failed)
			}
			if shape == storetest.ManySmall && res.Passes <= 1 {
				t.Errorf("%s: res.Passes = %d, want > 1 (1000 records at a 256 batch cannot drain in a single pass — the sentinel must end a pass at the batch boundary)", shape, res.Passes)
			}
			t.Logf("%s: res.Passes=%d res.Migrated=%d res.Failed=%d", shape, res.Passes, res.Migrated, res.Failed)

			after := independentIDScroll(ctx, t, c, name, migrateSweepBacklogFilter(int(migrate.CurrentVersion)))
			if len(after) != 0 {
				t.Errorf("%s: independent backlog re-derivation after apply = %d ids, want 0 (empty)", shape, len(after))
			}
		})
	}
}
