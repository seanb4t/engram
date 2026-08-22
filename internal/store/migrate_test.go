// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"fmt"
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

// hasPayloadKey is a thin wrapper over rawPayload asserting key presence.
func hasPayloadKey(ctx context.Context, t *testing.T, s *Store, id, key string) bool {
	t.Helper()
	_, ok := rawPayload(ctx, t, s, id)[key]
	return ok
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

	// PA-4's short-circuit is reachable ONLY through a NEGATIVE Target as of
	// migrate.CurrentVersion == 1: MigrateOptions.Target == 0 is now a
	// SENTINEL that Store.Migrate rewrites to migrate.CurrentVersion (== 1)
	// BEFORE the target<=0 short-circuit is evaluated — see the DEFAULT
	// case immediately below, which proves the sentinel resolves to a real
	// sweep. A future reader must not "simplify" this negative Target back
	// to 0: that silently converts this assertion into its own opposite
	// (a live sweep that happens to migrate nothing, rather than a
	// short-circuit that never runs one). migrate.Version is a named type
	// over int, so a negative value is representable and reaches only the
	// target<=0 branch, never a real step chain.
	res, err := s.Migrate(ctx, MigrateOptions{Target: -1})
	if err != nil {
		t.Fatalf("Migrate(target=-1): %v", err)
	}
	if res.Migrated != 0 {
		t.Errorf("Migrate(target=-1) Migrated = %d, want 0", res.Migrated)
	}
	if _, ok := rawPayload(ctx, t, s, absentID)[schemaVersionKey]; ok {
		t.Errorf("Migrate(target=-1) stamped schema_version onto the absent record — PA-4 violated")
	}

	// The DEFAULT target (MigrateOptions{}) resolves through
	// migrate.CurrentVersion and DOES migrate the absent record. Without
	// this assertion the negative-target assertion above would also be
	// satisfied by a sweep that is broken for every target — this is what
	// makes it non-vacuous.
	defaultRes, err := s.Migrate(ctx, MigrateOptions{})
	if err != nil {
		t.Fatalf("Migrate(default target): %v", err)
	}
	if defaultRes.Migrated == 0 {
		t.Errorf("Migrate(default target) Migrated = 0, want > 0 — the default target resolves through migrate.CurrentVersion and must migrate the absent record")
	}
	if got := rawPayload(ctx, t, s, absentID)[schemaVersionKey].GetIntegerValue(); got != int64(migrate.CurrentVersion) {
		t.Errorf("Migrate(default target) absent record schema_version = %d, want %d (migrate.CurrentVersion)", got, migrate.CurrentVersion)
	}
}

// TestMigrateRefusesNonAdditiveStep proves the fail-closed enforcement
// (Task 3): a step whose actual behavior diverges from its AddsKeys
// declaration is refused before any write, across three distinct
// sub-cases — an undeclared extra key, a removed key via a copying step,
// and a removed key via a step that mutates its input map IN PLACE (the
// case an aliasing bug in Store.Migrate's per-step re-cloning would make
// pass).
func TestMigrateRefusesNonAdditiveStep(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_nonadditive")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	cases := []struct {
		name string
		id   string
		step func() migrate.Step
	}{
		{
			name: "undeclared_extra_key",
			id:   "cf300000-0000-0000-0000-000000000001",
			step: func() migrate.Step {
				return migrate.NewStep(0, 1, []string{"declared_key"},
					migrate.Irreversible("test fixture: undeclared extra key"),
					func(payload map[string]any) (map[string]any, error) {
						out := maps.Clone(payload)
						out["declared_key"] = "v"
						out["undeclared_key"] = "v"
						return out, nil
					})
			},
		},
		{
			name: "removed_key_via_copy",
			id:   "cf300000-0000-0000-0000-000000000002",
			step: func() migrate.Step {
				return migrate.NewStep(0, 1, []string{"declared_key"},
					migrate.Irreversible("test fixture: removed key via a copy"),
					func(payload map[string]any) (map[string]any, error) {
						out := maps.Clone(payload)
						out["declared_key"] = "v"
						delete(out, "content")
						return out, nil
					})
			},
		},
		{
			// This sub-case is the one an aliasing bug (PA-5a) makes pass:
			// the ApplyFunc mutates its INPUT map in place and returns
			// that same map rather than a copy. If Store.Migrate's
			// per-step re-cloning were ever weakened to share one backing
			// map between before and after, this removal would be
			// invisible to the diff, CheckAdditive would return nil, and
			// the sweep would write happily. This sub-case differs from
			// the previous one ONLY in whether the step copies — and that
			// difference is the entire point.
			name: "removed_key_via_in_place_mutation",
			id:   "cf300000-0000-0000-0000-000000000003",
			step: func() migrate.Step {
				return migrate.NewStep(0, 1, []string{"declared_key"},
					migrate.Irreversible("test fixture: removed key via in-place mutation"),
					func(payload map[string]any) (map[string]any, error) {
						payload["declared_key"] = "v"
						delete(payload, "content")
						return payload, nil
					})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seedLegacyRecord(ctx, t, s, tc.id)

			res, err := s.Migrate(ctx, MigrateOptions{
				Target: 1,
				Steps:  []migrate.Step{tc.step()},
			})
			if err == nil {
				t.Fatalf("Migrate: got nil error, want non-nil (step is non-additive)")
			}
			if res.Migrated != 0 {
				t.Errorf("Migrated = %d, want 0", res.Migrated)
			}

			// Fail-closed proof: the refusal must precede the write, not
			// follow it — an independent rawPayload read shows the record
			// gained NEITHER key and still has no schema_version.
			if hasPayloadKey(ctx, t, s, tc.id, schemaVersionKey) {
				t.Errorf("record gained schema_version despite a refused step — refusal must precede the write")
			}
			if hasPayloadKey(ctx, t, s, tc.id, "declared_key") {
				t.Errorf("record gained declared_key despite a refused step — refusal must precede the write")
			}
		})
	}
}

// TestMigrateWritesOnlyAddedKeys is the containment proof for the gap
// migrate.CheckAdditive cannot close (T-03-12): a step that is fully
// conforming BY KEY SET — it declares exactly one new key and adds
// exactly that key — but whose ApplyFunc ALSO overwrites an EXISTING
// payload key's value must have that overwrite silently discarded,
// because Store.Migrate's write map is built from AddedKeys only.
// CheckAdditive catches key-set violations and is blind to value
// overwrites; Store.Migrate's added-keys-only write shaping is what makes
// that blindness harmless; neither alone is sufficient.
func TestMigrateWritesOnlyAddedKeys(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_write_shaping")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	id := "cf400000-0000-0000-0000-000000000001"
	seedLegacyRecord(ctx, t, s, id)
	originalContent := rawPayload(ctx, t, s, id)["content"].GetStringValue()

	step := migrate.NewStep(0, 1, []string{"new_declared_key"},
		migrate.Irreversible("test fixture: value-overwrite containment proof"),
		func(payload map[string]any) (map[string]any, error) {
			out := maps.Clone(payload)
			out["new_declared_key"] = "v"
			out["content"] = "OVERWRITTEN by a non-conforming value mutation"
			return out, nil
		})

	res, err := s.Migrate(ctx, MigrateOptions{Target: 1, Steps: []migrate.Step{step}})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if res.Migrated != 1 {
		t.Fatalf("Migrated = %d, want 1 (the step is additive by key set; CheckAdditive should accept it — this test is not about the checker)", res.Migrated)
	}

	raw := rawPayload(ctx, t, s, id)
	if got := raw["content"].GetStringValue(); got != originalContent {
		t.Errorf("content = %q, want unchanged %q (the write map is built from added keys only)", got, originalContent)
	}
	if _, ok := raw["new_declared_key"]; !ok {
		t.Errorf("record missing new_declared_key — the step did not demonstrably run, so the unchanged-content assertion above is not meaningful")
	}
	if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
		t.Errorf("schema_version = %d, want 1", got)
	}
}

// TestMigrateV0ToV1MintEndToEnd is the production first-customer proof: a
// bare legacy record (no short_id, no schema_version) driven through
// Store.Migrate against real pinned Qdrant with the REGISTERED v0->v1 step
// (MigrateOptions{} — target and steps both default to production) mints a
// short_id via Store.MintShortID, stamps schema_version=1, and a second
// Migrate run reports Backlog 0 (no-op) — the re-derivation identity
// REQ-migrate-preview-apply-parity is built on.
func TestMigrateV0ToV1MintEndToEnd(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_v0_v1_mint")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	id := "cf500000-0000-0000-0000-000000000001"
	seedLegacyRecord(ctx, t, s, id)
	if _, ok := rawPayload(ctx, t, s, id)["short_id"]; ok {
		t.Fatalf("seeded record already carries short_id, want genuinely absent")
	}

	res, err := s.Migrate(ctx, MigrateOptions{})
	if err != nil {
		t.Fatalf("Migrate (first run): %v", err)
	}
	if res.Migrated != 1 {
		t.Errorf("first run Migrated = %d, want 1", res.Migrated)
	}
	if res.Backlog != 0 {
		t.Errorf("first run Backlog = %d, want 0", res.Backlog)
	}

	raw := rawPayload(ctx, t, s, id)
	sid := raw["short_id"].GetStringValue()
	if len(sid) != 10 {
		t.Errorf("short_id = %q, want a 10-char Crockford base32 handle", sid)
	}
	if got := raw[schemaVersionKey].GetIntegerValue(); got != int64(migrate.CurrentVersion) {
		t.Errorf("schema_version = %d, want %d", got, migrate.CurrentVersion)
	}

	// Added-keys concern: only short_id + schema_version were added, no
	// other key removed — a general additive-only check over this record.
	if got := raw["content"]; got == nil {
		t.Errorf("content key vanished — additive-only violated")
	}

	// Second run is a no-op: no re-processing, backlog stays 0.
	res2, err := s.Migrate(ctx, MigrateOptions{})
	if err != nil {
		t.Fatalf("Migrate (second run): %v", err)
	}
	if res2.Migrated != 0 {
		t.Errorf("second run Migrated = %d, want 0", res2.Migrated)
	}
	if res2.Backlog != 0 {
		t.Errorf("second run Backlog = %d, want 0", res2.Backlog)
	}
	if got := rawPayload(ctx, t, s, id)["short_id"].GetStringValue(); got != sid {
		t.Errorf("short_id changed across re-run: %q -> %q, want unchanged", sid, got)
	}
}

// TestMigrateExistingShortIDPreserves is the H1/M2 mixed-state proof — the
// critical production state a prior standalone BackfillShortIDs run
// leaves behind: short_id present, schema_version absent. Migrate must
// preserve the existing short_id VERBATIM (never re-mint) and stamp
// schema_version, because the CheckAdditive carve-out treats a
// pre-existing declared key as satisfying the declaration. This is what
// makes 04-04's deletion of Store.BackfillShortIDs safe.
func TestMigrateExistingShortIDPreserves(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_existing_shortid")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	id := "cf600000-0000-0000-0000-000000000001"
	seedLegacyRecord(ctx, t, s, id)
	const priorShortID = "abc123abcd"
	injectRawPayload(ctx, t, s, id, qdrant.NewValueMap(map[string]any{"short_id": priorShortID}))
	before := rawPayload(ctx, t, s, id)
	if got := before["short_id"].GetStringValue(); got != priorShortID {
		t.Fatalf("seed: short_id = %q, want %q", got, priorShortID)
	}
	if _, ok := before[schemaVersionKey]; ok {
		t.Fatalf("seed: schema_version key present, want genuinely absent")
	}

	res, err := s.Migrate(ctx, MigrateOptions{})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if res.Backlog != 0 {
		t.Errorf("Backlog = %d, want 0", res.Backlog)
	}

	after := rawPayload(ctx, t, s, id)
	if got := after["short_id"].GetStringValue(); got != priorShortID {
		t.Errorf("short_id = %q, want UNCHANGED %q (never re-minted)", got, priorShortID)
	}
	if got := after[schemaVersionKey].GetIntegerValue(); got != int64(migrate.CurrentVersion) {
		t.Errorf("schema_version = %d, want %d", got, migrate.CurrentVersion)
	}
	// No other key was added or removed.
	beforeMap, err := payloadToMap(before)
	if err != nil {
		t.Fatalf("payloadToMap(before): %v", err)
	}
	afterMap, err := payloadToMap(after)
	if err != nil {
		t.Fatalf("payloadToMap(after): %v", err)
	}
	added := migrate.AddedKeys(beforeMap, afterMap)
	if len(added) != 1 || added[0] != schemaVersionKey {
		t.Errorf("added keys = %v, want exactly [%s] (short_id already existed, so it is not an ADDED key)", added, schemaVersionKey)
	}
	if removed := migrate.RemovedKeys(beforeMap, afterMap); len(removed) != 0 {
		t.Errorf("removed keys = %v, want none", removed)
	}
}

// upsertRawNoOwner writes a point straight through the client whose payload
// omits the owner key entirely (mirrors seedSource's raw point) — relocated
// here from store_test.go (04-04) as the one caller left after the deleted
// Store.BackfillShortIDs' TestBackfillShortIDs, which used it to prove the
// absent-owner-key invariant survived that method's payload-only SetPayload.
// TestMigrateOwnerlessRecordInvariant below carries that same proof across
// onto Store.Migrate.
func upsertRawNoOwner(ctx context.Context, t *testing.T, s *Store, id string, vec []float32) error {
	t.Helper()
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Wait:           qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(id),
			Vectors: qdrant.NewVectors(vec...),
			Payload: qdrant.NewValueMap(map[string]any{"content": "c", "scope": "s"}),
		}},
	})
	return err
}

// TestMigrateOwnerlessRecordInvariant carries across the absent-owner-key
// invariant the deleted Store.BackfillShortIDs' TestBackfillShortIDs
// asserted (04-04, REVIEWS.md C4-H3): a record written with NO owner key at
// all must still have none after Store.Migrate's payload-only SetPayload —
// the sweep must never synthesize an owner key it never saw. This is a
// genuinely legacy-shaped raw insert (no owner, no short_id, no
// schema_version), so it is picked up by backlogFilter directly with no
// seedLegacyRecord raw-delete step needed.
func TestMigrateOwnerlessRecordInvariant(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_ownerless_invariant")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	id := "cf700000-0000-0000-0000-000000000001"
	if err := upsertRawNoOwner(ctx, t, s, id, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsertRawNoOwner: %v", err)
	}
	if _, ok := rawPayload(ctx, t, s, id)["owner"]; ok {
		t.Fatalf("seed: owner key present, want genuinely absent")
	}

	res, err := s.Migrate(ctx, MigrateOptions{})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if res.Migrated != 1 {
		t.Errorf("Migrated = %d, want 1", res.Migrated)
	}

	after := rawPayload(ctx, t, s, id)
	if _, ok := after["owner"]; ok {
		t.Error("Migrate synthesized an owner key on a record that never carried one")
	}
	if got := after["short_id"].GetStringValue(); len(got) != 10 {
		t.Errorf("short_id = %q, want a 10-char Crockford base32 handle", got)
	}
	if got := after[schemaVersionKey].GetIntegerValue(); got != int64(migrate.CurrentVersion) {
		t.Errorf("schema_version = %d, want %d", got, migrate.CurrentVersion)
	}
}

// TestMigrateHonorsCancel replaces the deleted Store.BackfillShortIDs'
// TestBackfillShortIDsHonorsCancel (04-04, REVIEWS.md C4-H3): Store.Migrate
// must propagate context cancellation to its own Qdrant calls instead of
// running to completion — the property the CLI's --timeout/Ctrl-C wiring
// (migrateWithTimeout, cmd/engram/migrate_family.go) relies on.
func TestMigrateHonorsCancel(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_honors_cancel")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	seedLegacyRecord(ctx, t, s, "cf800000-0000-0000-0000-000000000001")

	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Migrate(cctx, MigrateOptions{}); err == nil {
		t.Error("cancelled context: expected error")
	}
}

// seedNLegacyRecords seeds n genuinely-key-absent legacy records with
// sequential deterministic ids under prefix, returning them sorted.
func seedNLegacyRecords(ctx context.Context, t *testing.T, s *Store, prefix string, n int) []string {
	t.Helper()
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%s-0000-0000-0000-%012d", prefix, i+1)
		seedLegacyRecord(ctx, t, s, id)
		ids[i] = id
	}
	sort.Strings(ids)
	return ids
}

// TestMigrateDryRunWritesNothing proves REVIEWS.md H2's non-writing half:
// DryRun performs its full backlog projection and issues no SetPayload at
// all — a fresh payload probe over every seeded record shows no point
// gained short_id or schema_version.
func TestMigrateDryRunWritesNothing(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_dryrun_nowrite")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	ids := seedNLegacyRecords(ctx, t, s, "cf700000", 5)

	res, err := s.Migrate(ctx, MigrateOptions{DryRun: true, Steps: []migrate.Step{markerStep(0, 1, "dryrun_marker")}})
	if err != nil {
		t.Fatalf("Migrate(DryRun): %v", err)
	}
	// NOT the projection count — stated explicitly, since Migrated stays 0
	// under DryRun by construction (nothing is written).
	if res.Migrated != 0 {
		t.Errorf("Migrated = %d, want 0 (DryRun writes nothing)", res.Migrated)
	}
	if len(res.PreviewManifest) != len(ids) {
		t.Errorf("len(PreviewManifest) = %d, want %d", len(res.PreviewManifest), len(ids))
	}
	for _, id := range ids {
		raw := rawPayload(ctx, t, s, id)
		if _, ok := raw["dryrun_marker"]; ok {
			t.Errorf("record %s gained dryrun_marker under DryRun — a write landed", id)
		}
		if _, ok := raw[schemaVersionKey]; ok {
			t.Errorf("record %s gained schema_version under DryRun — a write landed", id)
		}
	}
}

// TestMigrateFullBacklogProjection proves REVIEWS.md H2's full-backlog
// half: the preview projects the WHOLE backlog, not one scroll batch — a
// batch-scoped preview would silently under-report. The backlog here is
// deliberately larger than Batch. It then applies (DryRun:false, no
// Manifest) over the same stable collection and proves the applied count
// equals the earlier projection count.
func TestMigrateFullBacklogProjection(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_full_backlog_projection")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	const seeded = 40
	const batch = 8 // deliberately smaller than seeded, so a batch-scoped
	// preview bug would report `batch`, not `seeded`.
	ids := seedNLegacyRecords(ctx, t, s, "cf800000", seeded)

	opts := MigrateOptions{Steps: []migrate.Step{markerStep(0, 1, "projection_marker")}, Batch: batch}

	dryOpts := opts
	dryOpts.DryRun = true
	dryRes, err := s.Migrate(ctx, dryOpts)
	if err != nil {
		t.Fatalf("Migrate(DryRun): %v", err)
	}
	if dryRes.Migrated != 0 {
		t.Errorf("DryRun Migrated = %d, want 0", dryRes.Migrated)
	}
	if len(dryRes.PreviewManifest) != seeded {
		t.Errorf("len(PreviewManifest) = %d, want %d (the full backlog, not Batch=%d)", len(dryRes.PreviewManifest), seeded, batch)
	}
	for _, id := range ids {
		if _, ok := dryRes.PreviewManifest[id]; !ok {
			t.Errorf("PreviewManifest missing seeded id %s", id)
		}
	}

	applyRes, err := s.Migrate(ctx, opts)
	if err != nil {
		t.Fatalf("Migrate(apply): %v", err)
	}
	if applyRes.Backlog != 0 {
		t.Errorf("apply Backlog = %d, want 0", applyRes.Backlog)
	}
	if applyRes.Migrated != uint64(len(dryRes.PreviewManifest)) {
		t.Errorf("apply Migrated = %d, want %d (equal to the earlier projection count on this stable collection)", applyRes.Migrated, len(dryRes.PreviewManifest))
	}
}

// TestMigrateDryRunAndManifestMutuallyExclusive pins REVIEWS.md C5-L9: the
// undefined DryRun+Manifest combination is REJECTED before any I/O, not
// resolved by branch precedence. The anti-vacuity half: the store here is
// never EnsureCollection'd, so any actual backend call (Count/Scroll
// against a nonexistent collection) would fail with a Qdrant "not found"
// transport error, NOT ErrInvalidArgument — asserting the validation
// sentinel specifically proves no backend call was attempted.
func TestMigrateDryRunAndManifestMutuallyExclusive(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_dryrun_manifest_excl")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })
	s := newTestStore(t, c, collection)
	// Deliberately no EnsureCollection: a real Count/Scroll against this
	// collection would fail with a transport-level "not found" error, not
	// ErrInvalidArgument — proving the rejection below fires before any
	// I/O rather than merely alongside it.

	res, err := s.Migrate(ctx, MigrateOptions{
		DryRun:   true,
		Manifest: map[string]migrate.Version{"x": 0},
	})
	if err == nil {
		t.Fatal("Migrate(DryRun+Manifest) = nil error, want a validation error")
	}
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("Migrate(DryRun+Manifest) error = %v, want it to wrap ErrInvalidArgument (proving no backend I/O was attempted against the nonexistent collection)", err)
	}
	if res.Migrated != 0 {
		t.Errorf("Migrated = %d, want 0", res.Migrated)
	}
	if res.PreviewManifest != nil {
		t.Errorf("PreviewManifest = %v, want nil", res.PreviewManifest)
	}
}

// manifestDriftFixture seeds three legacy records, previews them via
// DryRun with a fixture markerStep, then simulates drift between preview
// and apply: a fourth record inserted AFTER the preview (appeared), and
// one previewed member stamped to target directly (spared). It returns
// the preview manifest and the three role ids.
func manifestDriftFixture(ctx context.Context, t *testing.T, s *Store) (manifest map[string]migrate.Version, keptIDs []string, sparedID, appearedID string) {
	t.Helper()
	steps := []migrate.Step{markerStep(0, 1, "drift_marker")}

	ids := seedNLegacyRecords(ctx, t, s, "cf900000", 3)
	dryRes, err := s.Migrate(ctx, MigrateOptions{DryRun: true, Steps: steps})
	if err != nil {
		t.Fatalf("Migrate(DryRun): %v", err)
	}
	if len(dryRes.PreviewManifest) != 3 {
		t.Fatalf("len(PreviewManifest) = %d, want 3", len(dryRes.PreviewManifest))
	}

	sparedID = ids[0]
	injectRawPayload(ctx, t, s, sparedID, qdrant.NewValueMap(map[string]any{schemaVersionKey: int(1)}))

	appearedID = "cf900000-0000-0000-0000-999999999999"
	seedLegacyRecord(ctx, t, s, appearedID)

	keptIDs = []string{ids[1], ids[2]}
	return dryRes.PreviewManifest, keptIDs, sparedID, appearedID
}

// TestMigrateManifestIntersection proves REVIEWS.md H6/SC3/C4-H5: apply
// migrates exactly manifest ∩ fresh re-derivation, Spared/Appeared report
// the two set differences BY IDENTITY (through stored-payload evidence,
// never through Migrated, which is a uint64 counter — C6-H3), and the
// cardinality reconciles.
func TestMigrateManifestIntersection(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_manifest_intersection")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	manifest, keptIDs, sparedID, appearedID := manifestDriftFixture(ctx, t, s)

	res, err := s.Migrate(ctx, MigrateOptions{
		Manifest: manifest,
		Steps:    []migrate.Step{markerStep(0, 1, "drift_marker")},
	})
	if err != nil {
		t.Fatalf("Migrate(Manifest): %v", err)
	}

	if !slices.Contains(res.Appeared, appearedID) {
		t.Errorf("Appeared = %v, want it to contain %s", res.Appeared, appearedID)
	}
	appearedRaw := rawPayload(ctx, t, s, appearedID)
	if _, ok := appearedRaw[schemaVersionKey]; ok {
		t.Errorf("appeared record %s carries schema_version — it must NOT have been migrated", appearedID)
	}

	if !slices.Contains(res.Spared, sparedID) {
		t.Errorf("Spared = %v, want it to contain %s", res.Spared, sparedID)
	}
	sparedRaw := rawPayload(ctx, t, s, sparedID)
	if _, ok := sparedRaw["drift_marker"]; ok {
		t.Errorf("spared record %s carries drift_marker — this apply must not have written it", sparedID)
	}

	for _, id := range keptIDs {
		raw := rawPayload(ctx, t, s, id)
		if _, ok := raw["drift_marker"]; !ok {
			t.Errorf("kept manifest member %s missing drift_marker — it should have been migrated", id)
		}
		if got := raw[schemaVersionKey].GetIntegerValue(); got != 1 {
			t.Errorf("kept manifest member %s schema_version = %d, want 1", id, got)
		}
	}

	if res.Failed != 0 {
		t.Fatalf("Failed = %d, want 0 (this fixture injects no faults; the no-failure cardinality form below assumes it)", res.Failed)
	}
	if want := uint64(len(manifest) - len(res.Spared)); res.Migrated != want {
		t.Errorf("Migrated = %d, want %d (= len(manifest) - len(Spared), the no-failure form)", res.Migrated, want)
	}
	if want := uint64(len(manifest) - len(res.Spared)); want != res.Migrated+res.Failed {
		t.Errorf("general reconciliation failed: len(manifest)-len(Spared) = %d, want == Migrated(%d)+Failed(%d)", want, res.Migrated, res.Failed)
	}

	if res.Backlog != uint64(len(res.Appeared)) {
		t.Errorf("Backlog = %d, want %d (== len(Appeared): the only remaining below-target records)", res.Backlog, len(res.Appeared))
	}

	if !sort.StringsAreSorted(res.Spared) {
		t.Errorf("Spared not sorted: %v", res.Spared)
	}
	if !sort.StringsAreSorted(res.Appeared) {
		t.Errorf("Appeared not sorted: %v", res.Appeared)
	}
}

// TestMigrateManifestSparedDeletedRecord pins the Spared semantic C4-H5
// documents: a manifest member deleted between preview and apply is
// reported Spared (not an error) — "ineligible or already gone" is one
// fact, and a post-scroll set difference reports it either way.
func TestMigrateManifestSparedDeletedRecord(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_manifest_spared_deleted")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	steps := []migrate.Step{markerStep(0, 1, "deleted_marker")}
	ids := seedNLegacyRecords(ctx, t, s, "cfa00000", 2)
	dryRes, err := s.Migrate(ctx, MigrateOptions{DryRun: true, Steps: steps})
	if err != nil {
		t.Fatalf("Migrate(DryRun): %v", err)
	}

	deletedID := ids[0]
	if _, derr := c.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collection, Wait: qdrant.PtrOf(true),
		Points: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(deletedID)}),
	}); derr != nil {
		t.Fatalf("delete %s: %v", deletedID, derr)
	}

	res, err := s.Migrate(ctx, MigrateOptions{Manifest: dryRes.PreviewManifest, Steps: steps})
	if err != nil {
		t.Fatalf("Migrate(Manifest) after delete: %v", err)
	}
	if !slices.Contains(res.Spared, deletedID) {
		t.Errorf("Spared = %v, want it to contain deleted id %s", res.Spared, deletedID)
	}
}

// TestMigrateManifestBacklogAppeared proves REVIEWS.md H7: Backlog after a
// manifest-limited apply TRUTHFULLY includes Appeared records this apply
// intentionally did not touch — never 0 merely because the manifest's own
// members are done — and a subsequent full sweep (no Manifest) cleans
// Appeared up, converging Backlog to 0. This also proves the PA-3 guard
// was never reached: a single-pass manifest apply has no second pass to
// compare against.
func TestMigrateManifestBacklogAppeared(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("migrate_manifest_backlog_appeared")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	manifest, _, _, appearedID := manifestDriftFixture(ctx, t, s)
	steps := []migrate.Step{markerStep(0, 1, "drift_marker")}

	res, err := s.Migrate(ctx, MigrateOptions{Manifest: manifest, Steps: steps})
	if err != nil {
		t.Fatalf("Migrate(Manifest): %v", err)
	}
	if res.Backlog == 0 {
		t.Fatalf("Backlog = 0, want > 0 (the Appeared record %s remains below target)", appearedID)
	}
	if res.Backlog != uint64(len(res.Appeared)) {
		t.Errorf("Backlog = %d, want %d (== len(Appeared))", res.Backlog, len(res.Appeared))
	}

	full, err := s.Migrate(ctx, MigrateOptions{Steps: steps})
	if err != nil {
		t.Fatalf("Migrate(full sweep): %v", err)
	}
	if full.Backlog != 0 {
		t.Errorf("full sweep Backlog = %d, want 0 — the Appeared record must be cleanable by a subsequent full sweep", full.Backlog)
	}
}
