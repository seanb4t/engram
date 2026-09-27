// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/shortid"
	"github.com/seanb4t/engram/internal/store"
)

// TestArchiveMemoryConnectRoundTrip proves ArchiveMemory/RestoreMemory end to
// end on Connect over a real Qdrant (milestone 2026-09-25.01 Phase 3,
// D-06/D-07): an owned record archived by full UUID reports one
// {requested: id, id: id, outcome: ARCHIVED} result, Store.Get shows
// ArchivedAt set with content and tags unchanged; restoring the same record
// by its short_id reports {requested: short_id, id: canonical id, outcome:
// RESTORED} and Store.Get shows ArchivedAt nil again; a subsequent
// list_memory recall (deps.listMemory, same scope) returns the record.
func TestArchiveMemoryConnectRoundTrip(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	owner := "sub-archive-roundtrip-" + uuid.NewString()
	scope := "iso-test:project:archive-roundtrip-" + uuid.NewString()
	ctx := parityConnectCtx(owner)

	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
	})

	shortID, err := shortid.New()
	if err != nil {
		t.Fatalf("shortid.New: %v", err)
	}

	id := uuid.NewString()
	tags := []string{"archive-roundtrip"}
	seed := store.Memory{
		ID: id, ShortID: shortID,
		Content: "archive round trip fixture", Scope: scope,
		Category: "gotcha", Source: "user-said", Tags: tags,
		Owner: owner, CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := st.Upsert(context.Background(), seed, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}

	archResp, err := api.ArchiveMemory(ctx, connect.NewRequest(&engramv1.ArchiveMemoryRequest{Ids: []string{id}}))
	if err != nil {
		t.Fatalf("ArchiveMemory: %v", err)
	}
	if got := len(archResp.Msg.GetResults()); got != 1 {
		t.Fatalf("ArchiveMemory results = %d, want 1", got)
	}
	archRow := archResp.Msg.GetResults()[0]
	if archRow.GetRequested() != id || archRow.GetId() != id || archRow.GetOutcome() != engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_ARCHIVED {
		t.Errorf("ArchiveMemory row = %+v, want {requested: %s, id: %s, outcome: ARCHIVED}", archRow, id, id)
	}

	rec, err := st.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("st.Get after archive: %v", err)
	}
	if rec.ArchivedAt == nil {
		t.Error("expected ArchivedAt to be set after ArchiveMemory")
	}
	if rec.Content != seed.Content {
		t.Errorf("content mutated by archive: got %q, want %q", rec.Content, seed.Content)
	}
	if !slices.Equal(rec.Tags, tags) {
		t.Errorf("tags mutated by archive: got %v, want %v", rec.Tags, tags)
	}

	restResp, err := api.RestoreMemory(ctx, connect.NewRequest(&engramv1.RestoreMemoryRequest{Ids: []string{shortID}}))
	if err != nil {
		t.Fatalf("RestoreMemory: %v", err)
	}
	if got := len(restResp.Msg.GetResults()); got != 1 {
		t.Fatalf("RestoreMemory results = %d, want 1", got)
	}
	restRow := restResp.Msg.GetResults()[0]
	if restRow.GetRequested() != shortID || restRow.GetId() != id || restRow.GetOutcome() != engramv1.ArchiveOutcome_ARCHIVE_OUTCOME_RESTORED {
		t.Errorf("RestoreMemory row = %+v, want {requested: %s, id: %s, outcome: RESTORED}", restRow, shortID, id)
	}

	rec2, err := st.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("st.Get after restore: %v", err)
	}
	if rec2.ArchivedAt != nil {
		t.Error("expected ArchivedAt to be nil after RestoreMemory")
	}

	c, err := callerFromTokenInfo(nil)
	if err != nil {
		t.Fatalf("callerFromTokenInfo(nil): %v", err)
	}
	c.Subj = store.Authenticated(owner)
	res, err := d.listMemory(context.Background(), c, coreListRequest{Scope: scope, Limit: 20})
	if err != nil {
		t.Fatalf("listMemory after restore: %v", err)
	}
	found := false
	for _, m := range res.Memories {
		if m.ID == id {
			found = true
		}
	}
	if !found {
		t.Errorf("restored record %s did not reappear in list_memory recall", id)
	}
}
