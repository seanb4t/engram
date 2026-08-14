// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"maps"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
)

// markerStep builds a test-only conforming migrate.Step: it declares
// exactly [key], its ApplyFunc sets payload[key] to a fixed, observable
// string, and its inverse (used only if a future plan exercises revert)
// removes that same key. The marker key is what makes "was this record
// processed?" directly observable in later plans; without it a zero-step
// sweep's only effect would be a version stamp that cannot distinguish a
// first pass from a second.
func markerStep(from, to migrate.Version, key string) migrate.Step {
	return migrate.NewStep(from, to, []string{key},
		migrate.Reversible(func(payload map[string]any) (map[string]any, error) {
			out := maps.Clone(payload)
			delete(out, key)
			return out, nil
		}),
		func(payload map[string]any) (map[string]any, error) {
			out := maps.Clone(payload)
			out[key] = "marker:" + key
			return out, nil
		},
	)
}

// seedLegacyRecord upserts a normal record through Store.Upsert, then
// raw-deletes schema_version, producing the genuinely-absent-key legacy
// shape (payload() unconditionally writes the key, so Upsert alone can
// never construct this state). It asserts the key really is gone before
// returning — a helper that silently failed to strip it would make every
// test in this phase vacuous.
func seedLegacyRecord(ctx context.Context, t *testing.T, s *Store, id string) {
	t.Helper()
	mem := Memory{
		ID:        id,
		Content:   "legacy record " + id,
		Scope:     "migrate-test:project:" + id,
		Owner:     "sub-migrate-test",
		Category:  "note",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, mem, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seedLegacyRecord: seed upsert(%s): %v", id, err)
	}
	deleteRawPayloadKeys(ctx, t, s, id, []string{schemaVersionKey})
	if _, ok := rawPayload(ctx, t, s, id)[schemaVersionKey]; ok {
		t.Fatalf("seedLegacyRecord(%s): schema_version key still present after delete — helper failed to construct the key-absent shape", id)
	}
}

// migrateBacklogIDs is an INDEPENDENT re-derivation of the backlog: it
// scrolls backlogFilter(target) directly through s.client and returns the
// sorted point ids. Tests assert against THIS, never against
// MigrateResult's counters — the same re-derivation discipline the
// production sweep itself follows.
func migrateBacklogIDs(ctx context.Context, t *testing.T, s *Store, target migrate.Version) []string {
	t.Helper()
	pts, _, err := s.client.ScrollAndOffset(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         backlogFilter(target),
		Limit:          qdrant.PtrOf(uint32(1000)),
		WithPayload:    qdrant.NewWithPayload(false),
	})
	if err != nil {
		t.Fatalf("migrateBacklogIDs(target=%d): %v", target, err)
	}
	ids := make([]string, 0, len(pts))
	for _, p := range pts {
		ids = append(ids, p.Id.GetUuid())
	}
	sort.Strings(ids)
	return ids
}

// rawPayloadSnapshot reads a record's full raw payload through rawPayload
// and converts it with payloadToMap into a comparable Go value, so two
// snapshots taken at different times can be compared with
// reflect.DeepEqual and the difference printed on failure.
func rawPayloadSnapshot(ctx context.Context, t *testing.T, s *Store, id string) map[string]any {
	t.Helper()
	m, err := payloadToMap(rawPayload(ctx, t, s, id))
	if err != nil {
		t.Fatalf("rawPayloadSnapshot(%s): payloadToMap: %v", id, err)
	}
	return m
}

// TestMigrateTracerLegacyRecordEndToEnd is the tracer (Task 1): one
// genuinely-key-absent legacy record travels from a markerStep fixture
// through migrate.Validate/StepsFrom, through Store.Migrate's re-derived
// backlog and backlogFilter's nested OR-group, to a real pinned Qdrant and
// back — carrying the step's declared key, schema_version == 1 and its
// original content — and a second identical run is proven to write
// nothing and change nothing.
func TestMigrateTracerLegacyRecordEndToEnd(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_tracer")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	id := "cf100000-0000-0000-0000-000000000001"
	seedLegacyRecord(ctx, t, s, id)
	originalContent := rawPayload(ctx, t, s, id)["content"].GetStringValue()

	opts := MigrateOptions{
		Target: 1,
		Steps:  []migrate.Step{markerStep(0, 1, "tracer_marker")},
		Batch:  8,
	}

	res, err := s.Migrate(ctx, opts)
	if err != nil {
		t.Fatalf("Migrate (first run): %v", err)
	}
	if res.Migrated != 1 {
		t.Errorf("first run Migrated = %d, want 1", res.Migrated)
	}
	if backlog := migrateBacklogIDs(ctx, t, s, 1); len(backlog) != 0 {
		t.Errorf("backlog after first run = %v, want empty", backlog)
	}

	raw := rawPayload(ctx, t, s, id)
	markerVal, ok := raw["tracer_marker"]
	if !ok {
		t.Fatalf("record %s missing tracer_marker after migrate", id)
	}
	if got := markerVal.GetStringValue(); got != "marker:tracer_marker" {
		t.Errorf("tracer_marker = %q, want %q", got, "marker:tracer_marker")
	}
	if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
		t.Errorf("schema_version = %d, want 1", got)
	}
	if got := raw["content"].GetStringValue(); got != originalContent {
		t.Errorf("content = %q, want unchanged %q (the sweep must merge, not replace)", got, originalContent)
	}

	// Sweep-level idempotence — the executable half of SC1's "idempotency"
	// word that migrate.Validate's transition-uniqueness rule does NOT
	// cover (see Validate's doc comment). The counters alone could be zero
	// because the sweep did nothing, and the payload alone could be
	// unchanged because the step is a no-op; asserting both together, over
	// a step that DID visibly change the record on the first run, is what
	// proves the re-run was genuinely a no-op. This proves the SWEEP is
	// idempotent across re-runs (what D-07's "resume is just run it again"
	// rests on); the STEP-level half — applying one step twice to the same
	// payload yields the same payload — is proven per fixture in plan
	// 03-03.
	snapshotAfterFirst := rawPayloadSnapshot(ctx, t, s, id)

	res2, err := s.Migrate(ctx, opts)
	if err != nil {
		t.Fatalf("Migrate (second run): %v", err)
	}
	if res2.Migrated != 0 {
		t.Errorf("second run Migrated = %d, want 0", res2.Migrated)
	}
	if res2.Failed != 0 {
		t.Errorf("second run Failed = %d, want 0", res2.Failed)
	}
	if res2.Passes != 1 {
		t.Errorf("second run Passes = %d, want 1 (the fresh count came back zero on the first pass)", res2.Passes)
	}
	if res2.Backlog != 0 {
		t.Errorf("second run Backlog = %d, want 0", res2.Backlog)
	}

	snapshotAfterSecond := rawPayloadSnapshot(ctx, t, s, id)
	if !reflect.DeepEqual(snapshotAfterFirst, snapshotAfterSecond) {
		t.Errorf("raw payload changed on re-run:\nbefore: %#v\nafter:  %#v", snapshotAfterFirst, snapshotAfterSecond)
	}
}
