// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
)

// TestSchemaVersionEndToEnd proves the whole schema-version slice in one
// path against real Qdrant: a record written through Store.Upsert carries
// schema_version = migrate.CurrentVersion through Store.List, Store.Get,
// and the JSON wire; a record written with no schema_version key at all
// (the legacy shape) is recalled by the same call and decodes to
// migrate.Version(0), with no backfill required.
func TestSchemaVersionEndToEnd(t *testing.T) {
	c := dialTestClient(t)
	ctx := context.Background()
	collection := testCollection("schemaversion_tracer")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })

	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	scope := "schemaversion:project:tracer"

	stampedID := "c0000000-0000-0000-0000-000000000001"
	stamped := Memory{ID: stampedID, Content: "stamped record", Scope: scope, Category: "gotcha"}
	if err := s.Upsert(ctx, stamped, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Upsert stamped: %v", err)
	}

	// A legacy-shaped record: no schema_version payload key at all, written
	// straight through the raw client (mirroring reindex_test.go's
	// seedSource raw-write idiom) since Store.Upsert always routes through
	// payload(), which always writes the key — this is the only way to
	// construct the absent-key shape a pre-adoption record actually has.
	// "owner": "" is included explicitly (not omitted) because a payload
	// with no owner key at all is invisible to every read (CLAUDE.md's
	// "Pre-isolation records" note) — that is a different, unrelated
	// invisibility this test must not trip while proving schema_version's.
	// "created_at" is likewise included: List's offset-mode Scroll orders
	// by the created_at index, and a point missing that indexed field is
	// dropped from an ordered scroll entirely — again unrelated to
	// schema_version, but required for this record to be recallable at all.
	legacyID := "c0000000-0000-0000-0000-000000000002"
	if _, err := c.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Wait:           qdrant.PtrOf(true),
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(legacyID),
			Vectors: qdrant.NewVectors(0.4, 0.5, 0.6),
			Payload: qdrant.NewValueMap(map[string]any{
				"content": "legacy record", "scope": scope, "category": "gotcha", "owner": "",
				"created_at": time.Now().UTC().Format(time.RFC3339),
			}),
		}},
	}); err != nil {
		t.Fatalf("raw Upsert legacy: %v", err)
	}

	items, _, _, err := s.List(ctx, scope, Anonymous(), ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byID := make(map[string]Memory, len(items))
	for _, m := range items {
		byID[m.ID] = m
	}
	gotStamped, ok := byID[stampedID]
	if !ok {
		t.Fatalf("List did not return stamped record %s; got %d items", stampedID, len(items))
	}
	if gotStamped.SchemaVersion != migrate.CurrentVersion {
		t.Fatalf("List: stamped record SchemaVersion = %d, want %d", gotStamped.SchemaVersion, migrate.CurrentVersion)
	}
	gotLegacy, ok := byID[legacyID]
	if !ok {
		t.Fatalf("List did not return legacy record %s; got %d items", legacyID, len(items))
	}
	if gotLegacy.SchemaVersion != migrate.Version(0) {
		t.Fatalf("List: legacy record (no schema_version key) SchemaVersion = %d, want 0", gotLegacy.SchemaVersion)
	}

	getStamped, err := s.Get(ctx, stampedID)
	if err != nil {
		t.Fatalf("Get stamped: %v", err)
	}
	if getStamped.SchemaVersion != migrate.CurrentVersion {
		t.Fatalf("Get: stamped record SchemaVersion = %d, want %d", getStamped.SchemaVersion, migrate.CurrentVersion)
	}

	// json.Marshal proof: the plain, no-omitempty json tag means BOTH
	// records — including the zero-versioned legacy one — carry an
	// explicit schema_version member on the wire.
	for _, tc := range []struct {
		name string
		m    Memory
	}{
		{"stamped", gotStamped},
		{"legacy (zero-versioned)", gotLegacy},
	} {
		b, err := json.Marshal(tc.m)
		if err != nil {
			t.Fatalf("json.Marshal(%s): %v", tc.name, err)
		}
		if !strings.Contains(string(b), `"schema_version"`) {
			t.Fatalf("json.Marshal(%s) = %s; missing schema_version member", tc.name, b)
		}
	}
}

// TestEnsureCollectionIndexesSchemaVersion pins D-12: EnsureCollection
// provisions a schema_version payload index, asserted from LIVE collection
// info (not source text).
func TestEnsureCollectionIndexesSchemaVersion(t *testing.T) {
	s := newTestStore(t, dialTestClient(t), testCollection("schemaversion_index"))
	ctx := context.Background()
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	info, err := s.client.GetCollectionInfo(ctx, s.collection)
	if err != nil {
		t.Fatalf("GetCollectionInfo: %v", err)
	}
	schema := info.GetPayloadSchema()
	if _, ok := schema[schemaVersionKey]; !ok {
		t.Fatalf("payload index missing for %q; have %v", schemaVersionKey, keysOf(schema))
	}
}

// TestUpdateRefreshesSchemaVersionUnderLock proves the Task 1 in-lock
// refresh fix: Store.Update's in-lock re-read now also copies
// fresh.SchemaVersion (alongside Supersedes/SupersededBy/ArchivedAt), so
// D-05's monotonic max is computed against the LATEST STORED stamp rather
// than only the caller's FetchForUpdate snapshot.
//
// The race is driven deterministically, never via unsynchronized
// goroutines: the "concurrent writer" is a raw client.SetPayload issued by
// this test, sequenced strictly between FetchForUpdate (which snapshots
// cur at the pre-raise version) and the call to Store.Update — landing
// unambiguously in the window Update's own in-lock re-read is about to
// re-scan. Without Task 1's fix, cur.SchemaVersion would remain fixed at
// its FetchForUpdate-time value for the whole call, and Update's Upsert
// would silently downgrade the just-raised stored version back down.
//
// This does NOT prove the narrower window between that in-lock re-read and
// Update's own Upsert is protected — it is not: a writer landing strictly
// there still loses, the same pre-existing lost-update window every field
// on Memory shares (recorded on the SchemaVersion field's own doc
// comment). updateAfterReadHook fires exactly in that later window (after
// the re-read, before the Upsert) — deliberately not used here to inject,
// since anything it writes would be overwritten by this same call's
// subsequent Upsert; that is precisely the boundary this test does not
// claim to cover.
func TestUpdateRefreshesSchemaVersionUnderLock(t *testing.T) {
	s := newTestStore(t, dialTestClient(t), testCollection("schemaversion_lockrefresh"))
	ctx := context.Background()
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	scope := "schemaversion:project:lockrefresh"
	subj := Authenticated("sub-lockrefresh")

	id := "c0000000-0000-0000-0000-000000000003"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "v1", Scope: scope, Owner: "sub-lockrefresh", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	cur, err := s.FetchForUpdate(ctx, id, subj)
	if err != nil {
		t.Fatalf("FetchForUpdate: %v", err)
	}
	if cur.SchemaVersion != migrate.CurrentVersion {
		t.Fatalf("precondition: cur.SchemaVersion = %d, want %d", cur.SchemaVersion, migrate.CurrentVersion)
	}

	// The "concurrent" raise: lands after cur was snapshotted, before
	// Update's in-lock re-read runs (Update has not been called yet).
	raised := migrate.CurrentVersion + 2
	if _, err := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: s.collection,
		Wait:           qdrant.PtrOf(true),
		Payload:        qdrant.NewValueMap(map[string]any{schemaVersionKey: int(raised)}),
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	}); err != nil {
		t.Fatalf("raw SetPayload (simulated concurrent raise): %v", err)
	}

	if err := s.Update(ctx, cur, "v2", nil, nil, nil, []float32{0.2, 0.3, 0.4}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SchemaVersion != raised {
		t.Fatalf("post-Update SchemaVersion = %d, want %d (the raised value, picked up by the in-lock re-read) — the stale cur snapshot must not downgrade it", got.SchemaVersion, raised)
	}
	if got.Content != "v2" {
		t.Errorf("post-Update Content = %q, want %q (Update's content edit must still land)", got.Content, "v2")
	}
}
