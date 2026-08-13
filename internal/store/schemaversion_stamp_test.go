// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
)

// TestEveryFullWriteMethodStampsSchemaVersion is D-03's BEHAVIORAL half: it
// observes what each caller-facing write mechanism actually leaves on disk
// against real Qdrant. It is NOT a claim of method-completeness on its own
// — Tasks 1-2's boundary scan (schemaversion_stamp_gate_test.go) is what
// fails when a fifth transmission appears; this table only proves the
// methods it names behave as documented.
//
// The distinct full-write mechanisms exercised are direct Store.Upsert,
// Store.Update, and Store.Supersede. The "scheduled" row is the SAME
// Store.Upsert method invoked with temporal fields set — kept as a row
// because the ROADMAP names the scheduled write path explicitly, not
// because it is a fourth mechanism.
func TestEveryFullWriteMethodStampsSchemaVersion(t *testing.T) {
	s := newTestStore(t, dialTestClient(t), testCollection("schemaversion_stamp"))
	ctx := context.Background()
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}
	scope := "schemaversion:project:stamp"
	owner := "sub-schemaversion-stamp"
	subj := Authenticated(owner)
	vec := []float32{0.1, 0.2, 0.3}

	const wantRows = 6
	executed := 0

	// Row 1: upsert-fresh — a new record's stored version is CurrentVersion.
	t.Run("upsert-fresh", func(t *testing.T) {
		executed++
		id := "e0000000-0000-0000-0000-000000000001"
		m := Memory{ID: id, Content: "fresh", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("upsert-fresh: Upsert: %v", err)
		}
		got, err := s.Get(ctx, id)
		if err != nil {
			t.Fatalf("upsert-fresh: Get: %v", err)
		}
		if got.SchemaVersion != migrate.CurrentVersion {
			t.Errorf("upsert-fresh: SchemaVersion = %d, want %d", got.SchemaVersion, migrate.CurrentVersion)
		}
	})

	// Row 2: upsert-scheduled — the SAME Store.Upsert method with
	// NotBefore/NotAfter set; asserted rather than assumed because the
	// ROADMAP names the schedule path explicitly.
	t.Run("upsert-scheduled", func(t *testing.T) {
		executed++
		id := "e0000000-0000-0000-0000-000000000002"
		nb := time.Now().UTC().Add(-time.Hour)
		na := time.Now().UTC().Add(time.Hour)
		m := Memory{
			ID: id, Content: "scheduled", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC(),
			NotBefore: &nb, NotAfter: &na,
		}
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("upsert-scheduled: Upsert: %v", err)
		}
		got, err := s.Get(ctx, id)
		if err != nil {
			t.Fatalf("upsert-scheduled: Get: %v", err)
		}
		if got.SchemaVersion != migrate.CurrentVersion {
			t.Errorf("upsert-scheduled: SchemaVersion = %d, want %d", got.SchemaVersion, migrate.CurrentVersion)
		}
	})

	// Row 3: update — Store.Update of the row-1-shaped record is still
	// CurrentVersion.
	t.Run("update", func(t *testing.T) {
		executed++
		id := "e0000000-0000-0000-0000-000000000003"
		m := Memory{ID: id, Content: "v1", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("update: Upsert: %v", err)
		}
		cur, err := s.FetchForUpdate(ctx, id, subj)
		if err != nil {
			t.Fatalf("update: FetchForUpdate: %v", err)
		}
		if err := s.Update(ctx, cur, "v2", nil, nil, nil, vec); err != nil {
			t.Fatalf("update: Update: %v", err)
		}
		got, err := s.Get(ctx, id)
		if err != nil {
			t.Fatalf("update: Get: %v", err)
		}
		if got.SchemaVersion != migrate.CurrentVersion {
			t.Errorf("update: SchemaVersion = %d, want %d", got.SchemaVersion, migrate.CurrentVersion)
		}
	})

	// Row 4: update-preserves-newer — D-05's monotonic rule observed at the
	// write path (the behavioral half of criterion 5): a version raised out
	// of band between the caller's snapshot and Store.Update's own call must
	// survive, never be downgraded back to the stale snapshot's value.
	t.Run("update-preserves-newer", func(t *testing.T) {
		executed++
		id := "e0000000-0000-0000-0000-000000000004"
		m := Memory{ID: id, Content: "v1", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, m, vec); err != nil {
			t.Fatalf("update-preserves-newer: Upsert: %v", err)
		}
		cur, err := s.FetchForUpdate(ctx, id, subj)
		if err != nil {
			t.Fatalf("update-preserves-newer: FetchForUpdate: %v", err)
		}
		raised := migrate.CurrentVersion + 2
		if _, err := s.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
			CollectionName: s.collection, Wait: qdrant.PtrOf(true),
			Payload:        qdrant.NewValueMap(map[string]any{schemaVersionKey: int(raised)}),
			PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
		}); err != nil {
			t.Fatalf("update-preserves-newer: raw SetPayload (simulated concurrent raise): %v", err)
		}
		if err := s.Update(ctx, cur, "v2", nil, nil, nil, vec); err != nil {
			t.Fatalf("update-preserves-newer: Update: %v", err)
		}
		got, err := s.Get(ctx, id)
		if err != nil {
			t.Fatalf("update-preserves-newer: Get: %v", err)
		}
		if got.SchemaVersion != raised {
			t.Errorf("update-preserves-newer: SchemaVersion = %d, want %d (the raised value; Update's in-lock re-read must not downgrade it)", got.SchemaVersion, raised)
		}
	})

	// Row 5: supersede — the NEW correcting record's version is
	// CurrentVersion.
	t.Run("supersede", func(t *testing.T) {
		executed++
		targetID := "e0000000-0000-0000-0000-000000000005"
		newID := "e0000000-0000-0000-0000-000000000006"
		target := Memory{ID: targetID, Content: "original", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC()}
		if err := s.Upsert(ctx, target, vec); err != nil {
			t.Fatalf("supersede: Upsert target: %v", err)
		}
		newMem := Memory{
			ID: newID, Content: "corrected", Scope: scope, Owner: owner, CreatedAt: time.Now().UTC(),
			Supersedes: []string{targetID},
		}
		if err := s.Supersede(ctx, newMem, vec, []string{targetID}, subj); err != nil {
			t.Fatalf("supersede: Supersede: %v", err)
		}
		got, err := s.Get(ctx, newID)
		if err != nil {
			t.Fatalf("supersede: Get: %v", err)
		}
		if got.SchemaVersion != migrate.CurrentVersion {
			t.Errorf("supersede: new record SchemaVersion = %d, want %d", got.SchemaVersion, migrate.CurrentVersion)
		}
	})

	// Row 6: partial-write-does-not-stamp — D-02's accepted limitation,
	// asserted rather than assumed: a v0 record (schema_version key
	// entirely ABSENT) touched by Store.SetVisibility (a setPayloadKeys
	// partial write) must stay with the key STILL ABSENT. Read via a RAW
	// point read, not Store.Get: a decoded Memory cannot distinguish
	// "key absent" from "key present with value 0" (both decode to
	// migrate.Version(0)) — this row's whole point is key PRESENCE, which
	// only the raw payload map can show.
	t.Run("partial-write-does-not-stamp", func(t *testing.T) {
		executed++
		id := "e0000000-0000-0000-0000-000000000007"
		// Raw-inject with the schema_version key entirely absent — the only
		// way to construct that shape, since Store.Upsert always stamps it
		// via payload().
		if _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: s.collection, Wait: qdrant.PtrOf(true),
			Points: []*qdrant.PointStruct{{
				Id:      qdrant.NewID(id),
				Vectors: qdrant.NewVectors(vec...),
				Payload: qdrant.NewValueMap(map[string]any{
					"content": "v0 legacy", "scope": scope, "owner": owner,
					"created_at": time.Now().UTC().Format(time.RFC3339),
				}),
			}},
		}); err != nil {
			t.Fatalf("partial-write-does-not-stamp: raw Upsert (v0 fixture): %v", err)
		}
		if err := s.SetVisibility(ctx, id, subj, true); err != nil {
			t.Fatalf("partial-write-does-not-stamp: SetVisibility: %v", err)
		}
		pts, err := s.client.Get(ctx, &qdrant.GetPoints{
			CollectionName: s.collection, Ids: []*qdrant.PointId{qdrant.NewID(id)},
			WithPayload: qdrant.NewWithPayload(true),
		})
		if err != nil {
			t.Fatalf("partial-write-does-not-stamp: raw Get: %v", err)
		}
		if len(pts) != 1 {
			t.Fatalf("partial-write-does-not-stamp: raw Get: got %d points, want 1", len(pts))
		}
		if _, ok := pts[0].GetPayload()[schemaVersionKey]; ok {
			t.Errorf("partial-write-does-not-stamp: %q key is PRESENT after SetVisibility on a v0 record — a partial write must never stamp currency it cannot honor (D-02)", schemaVersionKey)
		}
	})

	if executed != wantRows {
		t.Fatalf("executed %d rows, want %d — a row that silently failed to run must fail this suite", executed, wantRows)
	}
}
