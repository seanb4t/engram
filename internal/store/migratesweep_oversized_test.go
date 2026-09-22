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
	"maps"
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
// oversized backlog through the shared byte-budget iterator, and that its
// dry-run projection still covers the whole backlog while writing nothing
// — on both fixture shapes. The dry-run arm is only meaningful once
// Store.Migrate's DryRun walk is itself migrated onto the shared iterator
// (task 2): before that, a dry-run call at the DEFAULT batch over the
// FewLarge shape genuinely overflows the receive limit (one 256-record
// page, still requesting full payload, returns all 40 large records at
// once) — exactly the bug task 2 fixes.
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

			// Dry-run arm FIRST: the apply arm below drains the whole
			// backlog, which would make a subsequent dry-run vacuous. Safe
			// to run at the default batch now that Store.Migrate's DryRun
			// walk (task 2) also routes through the shared byte-budget
			// iterator — scrollAllPoints self-limits per-RPC page size
			// from s.fullView()'s own byte ceiling, independent of Batch.
			dryRes, err := st.Migrate(ctx, store.MigrateOptions{DryRun: true})
			if err != nil {
				t.Fatalf("%s: Migrate(DryRun): %v (request shape may have exceeded the %d-byte named receive limit)", shape, err, storetest.RecvLimit)
			}
			if len(dryRes.PreviewManifest) != len(fx.IDs) {
				t.Errorf("%s: len(PreviewManifest) = %d, want %d (the preview must cover the whole backlog on every page)", shape, len(dryRes.PreviewManifest), len(fx.IDs))
			}
			if dryRes.Migrated != 0 {
				t.Errorf("%s: dry-run res.Migrated = %d, want 0 (DryRun writes nothing)", shape, dryRes.Migrated)
			}
			afterDry := independentIDScroll(ctx, t, c, name, migrateSweepBacklogFilter(int(migrate.CurrentVersion)))
			if len(afterDry) != len(fx.IDs) {
				t.Errorf("%s: independent backlog re-derivation after DryRun = %d ids, want %d unchanged (DryRun must write nothing)", shape, len(afterDry), len(fx.IDs))
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

// migrateSweepAboveTargetFilter reconstructs aboveTargetFilter(to)'s exact
// shape (internal/store/revert.go) from outside the package: a record
// whose schema_version is strictly greater than to.
func migrateSweepAboveTargetFilter(to int) *qdrant.Filter {
	key := store.SchemaVersionKey()
	return &qdrant.Filter{
		Must: []*qdrant.Condition{
			qdrant.NewRange(key, &qdrant.Range{Gt: qdrant.PtrOf(float64(to))}),
		},
	}
}

// revertFixtureStep builds a test-only conforming migrate.Step declaring
// exactly [key], mirroring internal/store/migrate_test.go's own markerStep
// (unexported, package store — reimplemented here since this file must be
// package store_test per memory y02a9ft3gy). Its inverse is what makes this
// chain REVERSIBLE: the production migrate.Registry's only step is
// Irreversible, so a real-registry revert can never apply — this fixture
// chain is what lets the apply path actually run.
func revertFixtureStep(from, to migrate.Version, key string) migrate.Step {
	return migrate.NewStep(from, to, []string{key},
		migrate.Reversible(func(payload map[string]any) (map[string]any, error) {
			out := maps.Clone(payload)
			delete(out, key)
			return out, nil
		}),
		func(payload map[string]any) (map[string]any, error) {
			out := maps.Clone(payload)
			out[key] = "fixture:" + key
			return out, nil
		},
	)
}

// TestRevertApplyBoundedOverGRPCLimit proves engram migrate revert's apply
// path (Store.revertWithSteps, reached here through the RevertWithSteps
// shim with a locally built REVERSIBLE fixture chain) drains an oversized
// above-target range through the shared byte-budget iterator, on both
// fixture shapes.
func TestRevertApplyBoundedOverGRPCLimit(t *testing.T) {
	shapes := []storetest.Shape{storetest.FewLarge, storetest.ManySmall}
	for _, shape := range shapes {
		t.Run(shape.String(), func(t *testing.T) {
			c := storetest.Dial(t, storetest.RecvLimit)
			name := store.PrefixedTestCollection("oversized_revertapply_" + uuid.NewString())
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

			// Every seeded record already carries migrate.CurrentVersion
			// (1) via the ordinary Store.Upsert path — no manipulation
			// needed: every seeded record is already above target 0.
			fx := storetest.SeedOversized(t, st, storetest.Spec{Limit: storetest.RecvLimit, Shape: shape, Vector: []float32{0.1, 0.2, 0.3}})

			fixtureSteps := []migrate.Step{revertFixtureStep(0, migrate.CurrentVersion, "revertFixtureKey")}

			res, err := st.RevertWithSteps(ctx, 0, fixtureSteps)
			if err != nil {
				t.Fatalf("%s: RevertWithSteps: %v (request shape may have exceeded the %d-byte named receive limit, or the fixture chain is unexpectedly irreversible/unsupported)", shape, err, storetest.RecvLimit)
			}
			if res.Reverted != uint64(len(fx.IDs)) {
				t.Errorf("%s: res.Reverted = %d, want %d", shape, res.Reverted, len(fx.IDs))
			}
			if res.Failed != 0 {
				t.Errorf("%s: res.Failed = %d, want 0", shape, res.Failed)
			}
			if shape == storetest.ManySmall && res.Passes <= 1 {
				t.Errorf("%s: res.Passes = %d, want > 1 (1000 records at a 256 batch cannot drain in a single pass)", shape, res.Passes)
			}
			t.Logf("%s: res.Passes=%d res.Reverted=%d res.Failed=%d", shape, res.Passes, res.Reverted, res.Failed)

			after := independentIDScroll(ctx, t, c, name, migrateSweepAboveTargetFilter(0))
			if len(after) != 0 {
				t.Errorf("%s: independent above-target re-derivation after apply = %d ids, want 0 (empty)", shape, len(after))
			}
		})
	}
}
