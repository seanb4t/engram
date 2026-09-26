// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// TestEnsureIndexesCreatesTagsIndex pins D-16: ensureIndexes creates a tags
// keyword payload index, and a second call is AlreadyExists-tolerant —
// mirrors TestEnsureIndexesCreatesShortIDIndex's shape (store_test.go).
func TestEnsureIndexesCreatesTagsIndex(t *testing.T) {
	st := newSpineTestStore(t, "listtags_index")
	info, err := st.client.GetCollectionInfo(context.Background(), st.collection)
	if err != nil {
		t.Fatal(err)
	}
	schema, ok := info.GetPayloadSchema()["tags"]
	if !ok {
		t.Fatalf("tags payload index not created; schema keys: %v", info.GetPayloadSchema())
	}
	if schema.GetDataType() != qdrant.PayloadSchemaType_Keyword {
		t.Fatalf("tags payload index data type = %v, want Keyword", schema.GetDataType())
	}
	// Idempotence: a second ensureIndexes is AlreadyExists-tolerant.
	if err := st.ensureIndexes(context.Background(), st.collection); err != nil {
		t.Fatalf("second ensureIndexes: %v", err)
	}
}

// TestListTagsCountsOwnedTags is the phase's tracer test: one owner's tags
// come back as exact, sorted counts through the filtered Facet, and another
// owner's private tags are absent.
func TestListTagsCountsOwnedTags(t *testing.T) {
	s := newSpineTestStore(t, "listtags_tracer")
	ctx := context.Background()
	scope := "listtags:project:tracer"

	seedSpineMemory(t, s, Memory{
		ID: "aaaaaaaa-0000-0000-0000-000000000001", Content: "r1", Scope: scope,
		Owner: "listtags-owner-a", Category: "note", Tags: []string{"go", "qdrant"},
		CreatedAt: time.Now().UTC(),
	})
	seedSpineMemory(t, s, Memory{
		ID: "aaaaaaaa-0000-0000-0000-000000000002", Content: "r2", Scope: scope,
		Owner: "listtags-owner-a", Category: "note", Tags: []string{"go"},
		CreatedAt: time.Now().UTC(),
	})
	seedSpineMemory(t, s, Memory{
		ID: "bbbbbbbb-0000-0000-0000-000000000001", Content: "other", Scope: scope,
		Owner: "listtags-owner-b", Category: "note", Tags: []string{"go", "secret-b"},
		CreatedAt: time.Now().UTC(),
	})

	got, more, err := s.ListTags(ctx, Authenticated("listtags-owner-a"), "", 0)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	want := []TagCount{{Tag: "go", Count: 2}, {Tag: "qdrant", Count: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListTags = %+v, want %+v", got, want)
	}
	if more {
		t.Fatalf("ListTags more = true, want false")
	}
}
