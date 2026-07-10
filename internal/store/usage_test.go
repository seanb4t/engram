// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// TestUsageAccessCountRoundtrip proves AccessCount/LastAccessedAt round-trip
// losslessly through payload()/fromPayload() via a real Qdrant point: the
// uint64 is preserved as a Value_IntegerValue (not coerced to a double), and
// LastAccessedAt survives the RFC3339 format/parse round-trip to second
// precision (D-03, RESEARCH.md Pitfall 1).
func TestUsageAccessCountRoundtrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:usage-roundtrip"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "b0000000-0000-0000-0000-000000000001"
	lastAccessed := time.Now().UTC().Truncate(time.Second)
	m := Memory{
		ID: id, Content: "v", Scope: scope, Owner: "sub-usage",
		CreatedAt: time.Now().UTC(), AccessCount: 3, LastAccessedAt: lastAccessed,
	}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Read back through the store's own round-trip.
	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AccessCount != 3 {
		t.Errorf("AccessCount = %d, want 3", got.AccessCount)
	}
	if !got.LastAccessedAt.Equal(lastAccessed) {
		t.Errorf("LastAccessedAt = %v, want %v", got.LastAccessedAt, lastAccessed)
	}

	// Read the raw Qdrant point payload directly to confirm access_count is
	// stored as an IntegerValue oneof, not a DoubleValue (no float coercion).
	pts, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collection, Ids: []*qdrant.PointId{qdrant.NewID(id)},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		t.Fatalf("raw get: %v", err)
	}
	if len(pts) != 1 {
		t.Fatalf("raw get: got %d points, want 1", len(pts))
	}
	v, ok := pts[0].Payload["access_count"]
	if !ok {
		t.Fatal("raw payload missing access_count key")
	}
	if v.GetIntegerValue() != 3 {
		t.Errorf("raw access_count IntegerValue = %d, want 3", v.GetIntegerValue())
	}
	if v.GetDoubleValue() != 0 {
		t.Errorf("raw access_count DoubleValue = %v, want 0 (should be stored as IntegerValue, not coerced to double)", v.GetDoubleValue())
	}
}

// TestUsageLegacyRecordReadsZero proves a record whose payload predates D-03
// (missing both access_count and last_accessed_at keys) reads back
// AccessCount==0 and LastAccessedAt.IsZero()==true — no backfill required.
func TestUsageLegacyRecordReadsZero(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:usage-legacy"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "b0000000-0000-0000-0000-000000000002"
	// Upsert directly via the raw client with a payload that predates the
	// access_count/last_accessed_at keys, bypassing payload() entirely so this
	// genuinely simulates a legacy record.
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Points: []*qdrant.PointStruct{{
			Id:      qdrant.NewID(id),
			Vectors: qdrant.NewVectors(0.1, 0.2, 0.3),
			Payload: qdrant.NewValueMap(map[string]any{
				"content": "legacy record", "scope": scope, "owner": "sub-usage",
				"created_at": time.Now().UTC().Format(time.RFC3339),
			}),
		}},
	})
	if err != nil {
		t.Fatalf("raw upsert: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AccessCount != 0 {
		t.Errorf("legacy record AccessCount = %d, want 0", got.AccessCount)
	}
	if !got.LastAccessedAt.IsZero() {
		t.Errorf("legacy record LastAccessedAt = %v, want zero", got.LastAccessedAt)
	}
}

// TestUpdateBumpsAccessCountByOne proves the update-path bump (D-04): a single
// store.Update call raises the persisted access_count by exactly 1 and stamps
// a non-zero last_accessed_at, piggybacking the existing Upsert with zero
// extra store round-trip.
func TestUpdateBumpsAccessCountByOne(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "iso-test:project:usage-update-bump"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "b0000000-0000-0000-0000-000000000003"
	m := Memory{ID: id, Content: "v1", Scope: scope, Owner: "sub-usage", CreatedAt: time.Now().UTC(), AccessCount: 5}
	if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	cur, err := s.FetchForUpdate(ctx, id, Authenticated("sub-usage"))
	if err != nil {
		t.Fatalf("FetchForUpdate: %v", err)
	}
	if err := s.Update(ctx, cur, "v2", nil, nil, nil, []float32{0.4, 0.5, 0.6}); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.AccessCount != 6 {
		t.Errorf("AccessCount after one Update = %d, want 6 (5+1)", got.AccessCount)
	}
	if got.LastAccessedAt.IsZero() {
		t.Error("LastAccessedAt after Update is zero, want a non-zero stamp")
	}
}
