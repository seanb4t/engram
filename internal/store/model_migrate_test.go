// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/seanb4t/engram/internal/migrate"
	"pgregory.net/rapid"
)

type fataler interface {
	Helper()
	Fatalf(format string, args ...any)
}

func modelRawPayload(ctx context.Context, t fataler, s *Store, id string) map[string]any {
	t.Helper()
	pts, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collection, Ids: []*qdrant.PointId{qdrant.NewID(id)},
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil || len(pts) == 0 {
		t.Fatalf("Get(%s): %d points, err %v", id, len(pts), err)
	}
	m, err := payloadToMap(pts[0].Payload)
	if err != nil {
		t.Fatalf("payloadToMap(%s): %v", id, err)
	}
	return m
}

func modelDeleteKeys(ctx context.Context, t fataler, s *Store, id string, keys ...string) {
	t.Helper()
	if _, err := s.client.DeletePayload(ctx, &qdrant.DeletePayloadPoints{
		CollectionName: s.collection, Wait: new(true), Keys: keys,
		PointsSelector: qdrant.NewPointsSelectorIDs([]*qdrant.PointId{qdrant.NewID(id)}),
	}); err != nil {
		t.Fatalf("DeletePayload(%s, %v): %v", id, keys, err)
	}
}

func schemaVersionOf(p map[string]any) int64 {
	v, _ := p[schemaVersionKey].(int64)
	return v
}

// TestModelMigrateSweep seeds generated version-0 records, runs the sweep
// twice, and compares the stored payloads with the additive contract.
func TestModelMigrateSweep(t *testing.T) {
	ctx := context.Background()
	c := dialTestClient(t)
	collection := testCollection("model_migrate")
	_ = c.DeleteCollection(ctx, collection)
	t.Cleanup(func() { _ = c.DeleteCollection(context.Background(), collection) })
	s := newTestStore(t, c, collection)
	if err := s.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection: %v", err)
	}

	var checks atomic.Uint32
	rapid.Check(t, func(t *rapid.T) {
		n := checks.Add(1)
		count := rapid.IntRange(1, 6).Draw(t, "records")
		before := map[string]map[string]any{}
		for i := range count {
			id := fmt.Sprintf("%08x-0000-4000-8000-%012x", n, i+1)
			mem := Memory{
				ID: id, Content: "legacy " + id, Scope: fmt.Sprintf("model-migrate:%d", n),
				Owner:     rapid.SampledFrom([]string{"", "alice"}).Draw(t, "owner"),
				Category:  rapid.SampledFrom([]string{"note", "decision", "rule"}).Draw(t, "category"),
				Tags:      rapid.SliceOfN(rapid.StringMatching(`[a-z]{1,6}`), 0, 3).Draw(t, "tags"),
				CreatedAt: time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC),
			}
			if rapid.Bool().Draw(t, "has_short_id") {
				// A short_id that backfill-short-ids stamped before any
				// migration ran.
				mem.ShortID = rapid.StringMatching(`[0-9a-hjkmnp-tv-z]{10}`).Draw(t, "short_id")
			}
			if err := s.Upsert(ctx, mem, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("Upsert: %v", err)
			}
			modelDeleteKeys(ctx, t, s, id, schemaVersionKey)
			if mem.ShortID == "" {
				modelDeleteKeys(ctx, t, s, id, "short_id")
			}
			before[id] = modelRawPayload(ctx, t, s, id)
		}

		if _, err := s.Migrate(ctx, MigrateOptions{}); err != nil {
			t.Fatalf("Migrate (first run): %v", err)
		}
		after := map[string]map[string]any{}
		for id, b := range before {
			a := modelRawPayload(ctx, t, s, id)
			after[id] = a
			if got := schemaVersionOf(a); got != int64(migrate.CurrentVersion) {
				t.Fatalf("%s: schema_version = %v after the first run, want %d", id, got, migrate.CurrentVersion)
			}
			for k := range b {
				if _, ok := a[k]; !ok {
					t.Fatalf("%s: key %q removed by the sweep", id, k)
				}
			}
			if sid, ok := b["short_id"]; ok && a["short_id"] != sid {
				t.Fatalf("%s: short_id %v rewritten to %v", id, sid, a["short_id"])
			}
			if sid, _ := a["short_id"].(string); len(sid) != 10 {
				t.Fatalf("%s: short_id = %q, want a 10-rune handle", id, sid)
			}
		}

		res, err := s.Migrate(ctx, MigrateOptions{})
		if err != nil {
			t.Fatalf("Migrate (second run): %v", err)
		}
		if res.Migrated != 0 {
			t.Fatalf("second run migrated %d records, want 0", res.Migrated)
		}
		for id, a := range after {
			again := modelRawPayload(ctx, t, s, id)
			if !reflect.DeepEqual(again, a) {
				t.Fatalf("%s: second run changed the payload\nfirst:  %v\nsecond: %v", id, a, again)
			}
		}
	})
}
