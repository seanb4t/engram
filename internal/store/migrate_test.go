// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"maps"
	"reflect"
	"slices"
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

// TestBacklogFilterMatchesAbsentAndBelowTarget proves the backlog filter
// (Task 2): a record whose schema_version key is genuinely absent, a
// record with an explicit below-target value, and a record already at
// target are correctly partitioned, and the target<=0 short-circuit is
// proven to live in Store.Migrate rather than in backlogFilter itself.
func TestBacklogFilterMatchesAbsentAndBelowTarget(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_backlog_filter")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	absentID := "cf200000-0000-0000-0000-000000000001"
	belowID := "cf200000-0000-0000-0000-000000000002"
	currentID := "cf200000-0000-0000-0000-000000000003"

	// absent: schema_version genuinely absent — the only record that can
	// distinguish a correct filter from a broken one.
	seedLegacyRecord(ctx, t, s, absentID)
	if _, ok := rawPayload(ctx, t, s, absentID)[schemaVersionKey]; ok {
		t.Fatalf("absent record: schema_version key present, want genuinely absent")
	}

	// below: normally upserted, then raw-injected to an explicit
	// below-target value. The key is PRESENT with a below-target value.
	belowMem := Memory{
		ID: belowID, Content: "below-target record",
		Scope: "migrate-test:project:below", Owner: "sub-migrate-test",
		Category: "note", CreatedAt: time.Now().UTC(),
	}
	if err := s.Upsert(ctx, belowMem, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed below: %v", err)
	}
	injectRawPayload(ctx, t, s, belowID, qdrant.NewValueMap(map[string]any{schemaVersionKey: 0}))

	// current: normally upserted whose Memory.SchemaVersion is
	// migrate.Version(1), so payload()'s monotonic max stamps 1 through the
	// REAL production write path — not injectRawPayload.
	currentMem := Memory{
		ID: currentID, Content: "current record",
		Scope: "migrate-test:project:current", Owner: "sub-migrate-test",
		Category: "note", CreatedAt: time.Now().UTC(),
		SchemaVersion: migrate.Version(1),
	}
	if err := s.Upsert(ctx, currentMem, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed current: %v", err)
	}
	if got := rawPayload(ctx, t, s, currentID)[schemaVersionKey].GetIntegerValue(); got != 1 {
		t.Fatalf("current record: schema_version = %d, want 1 (stamped by the production write path)", got)
	}

	// Assert as a SORTED SET, in both directions — not a length check, not
	// a contains check: a length check passes for the wrong two records,
	// and a contains check passes for a filter that matches everything.
	got := migrateBacklogIDs(ctx, t, s, 1)
	want := []string{absentID, belowID}
	sort.Strings(want)
	if !slices.Equal(got, want) {
		gotSet := make(map[string]bool, len(got))
		for _, id := range got {
			gotSet[id] = true
		}
		wantSet := make(map[string]bool, len(want))
		for _, id := range want {
			wantSet[id] = true
		}
		var extra, missing []string
		for _, id := range got {
			if !wantSet[id] {
				extra = append(extra, id)
			}
		}
		for _, id := range want {
			if !gotSet[id] {
				missing = append(missing, id)
			}
		}
		t.Errorf("backlog at target=1: got %v, want %v (extra=%v missing=%v)", got, want, extra, missing)
	}

	// Non-vacuity: the derived set must be non-empty AND must exclude
	// current — both, as separate assertions. An empty result would
	// satisfy any "does not contain current" phrasing on its own.
	if len(got) == 0 {
		t.Errorf("backlog at target=1 is empty, want non-empty")
	}
	for _, id := range got {
		if id == currentID {
			t.Errorf("backlog at target=1 contains current record %s, want excluded", currentID)
		}
	}

	// target<=0, split into the two halves PA-4 actually names:

	// The FILTER alone at target 0 is broad, deliberately: the IsEmpty arm
	// matches an absent-key record at ANY target, so a caller who builds
	// the filter and scrolls it directly gets legacy records back. This is
	// correct behavior for a filter in isolation and is exactly why the
	// safety property cannot live here (see backlogFilter's own doc
	// comment).
	zeroTargetBacklog := migrateBacklogIDs(ctx, t, s, 0)
	foundAbsent := false
	for _, id := range zeroTargetBacklog {
		if id == absentID {
			foundAbsent = true
		}
	}
	if !foundAbsent {
		t.Errorf("backlogFilter(0) does not contain absent record %s — the IsEmpty arm should match it at any target", absentID)
	}

	// The SWEEP at target 0 does nothing: THIS assertion — not the
	// filter's shape — is PA-4's actual guarantee. A record at v0 needs no
	// work, and the sweep must not stamp the whole collection to prove
	// otherwise.
	res, err := s.Migrate(ctx, MigrateOptions{Target: 0})
	if err != nil {
		t.Fatalf("Migrate(target=0): %v", err)
	}
	if res.Migrated != 0 {
		t.Errorf("Migrate(target=0) Migrated = %d, want 0", res.Migrated)
	}
	if _, ok := rawPayload(ctx, t, s, absentID)[schemaVersionKey]; ok {
		t.Errorf("Migrate(target=0) stamped schema_version onto the absent record — PA-4 violated")
	}
}
