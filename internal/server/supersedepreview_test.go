// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"testing"

	"github.com/google/uuid"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"

	"connectrpc.com/connect"
)

// TestSupersedeMemoryConnectRoundTrip (Task 1 tracer, real Qdrant, D-08/D-09)
// proves the SupersedeMemory Connect handler and the shared validateSupersede
// dry run agree: a validate_only preview over two owned targets (addressed
// by short_id, UUID, and a duplicate UUID) resolves and deduplicates
// identically to a real commit, writes nothing, and the real commit that
// follows produces the exact supersedes list the preview reported.
func TestSupersedeMemoryConnectRoundTrip(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	owner := "sub-supersede-connect-" + uuid.NewString()
	scope := "iso-test:project:supersede-connect-" + uuid.NewString()

	seedCtx := authedContext(t, owner)
	seedCaller := callerFor(seedCtx, t)

	t1ID, t1SID, err := d.storeMemory(seedCtx, seedCaller, storeArgs{
		Content: "target one content", Scope: scope, Category: "gotcha", Source: "user-said",
	})
	if err != nil {
		t.Fatalf("seed T1: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete T1", d.st.Delete(context.Background(), t1ID, store.Authenticated(owner)))
	})

	t2ID, _, err := d.storeMemory(seedCtx, seedCaller, storeArgs{
		Content: "target two content", Scope: scope, Category: "gotcha", Source: "user-said",
	})
	if err != nil {
		t.Fatalf("seed T2: %v", err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete T2", d.st.Delete(context.Background(), t2ID, store.Authenticated(owner)))
	})

	connCtx := parityConnectCtx(owner)
	api := &engramAPI{d: d}

	// T1 addressed by its short_id, T2 by its UUID, and T1 AGAIN by its UUID
	// — proves the dedupe-by-resolved-id pass (PD-01) fires in the dry run
	// exactly as it does in a real call.
	req := &engramv1.SupersedeMemoryRequest{
		Content: "corrected content", Scope: scope, Category: "gotcha", Source: "user-said",
		Supersedes:   []string{t1SID, t2ID, t1ID},
		ValidateOnly: true,
	}
	previewResp, err := api.SupersedeMemory(connCtx, connect.NewRequest(req))
	if err != nil {
		t.Fatalf("validate_only SupersedeMemory: %v", err)
	}
	preview := previewResp.Msg
	if !preview.GetValidated() {
		t.Fatalf("preview.Validated = false, want true")
	}
	if preview.GetId() != "" {
		t.Errorf("preview.Id = %q, want empty (nothing written)", preview.GetId())
	}
	if got, want := preview.GetSupersedes(), []string{t1ID, t2ID}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("preview.Supersedes = %v, want %v (caller order, deduplicated)", got, want)
	}
	targets := preview.GetTargets()
	if len(targets) != 2 {
		t.Fatalf("preview.Targets has %d entries, want 2", len(targets))
	}
	for _, tgt := range targets {
		if tgt.GetContent() != "" {
			t.Errorf("preview target %s carries Content %q, want empty (compact view)", tgt.GetId(), tgt.GetContent())
		}
		if tgt.GetSummary() == "" {
			t.Errorf("preview target %s carries empty Summary, want a derived summary", tgt.GetId())
		}
	}

	// The preview must have written nothing: List still returns both
	// records, neither carries superseded_by.
	listed, _, _, err := d.st.List(ctx, scope, store.Authenticated(owner), store.ListOptions{Limit: 50})
	if err != nil {
		t.Fatalf("List after preview: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("List after preview returned %d records, want 2 (nothing written)", len(listed))
	}
	for _, id := range []string{t1ID, t2ID} {
		rec, err := d.st.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get(%s) after preview: %v", id, err)
		}
		if rec.SupersededBy != nil {
			t.Errorf("target %s carries SupersededBy=%v after preview, want nil", id, *rec.SupersededBy)
		}
	}

	// Now the SAME request, committed for real.
	req.ValidateOnly = false
	commitResp, err := api.SupersedeMemory(connCtx, connect.NewRequest(req))
	if err != nil {
		t.Fatalf("real SupersedeMemory: %v", err)
	}
	newID := commitResp.Msg.GetId()
	if newID == "" {
		t.Fatalf("commit.Id is empty, want a minted id")
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete new record", d.st.Delete(context.Background(), newID, store.Authenticated(owner)))
	})
	if commitResp.Msg.GetValidated() {
		t.Errorf("commit.Validated = true, want false (this is a real write)")
	}

	for _, id := range []string{t1ID, t2ID} {
		rec, err := d.st.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get(%s) after commit: %v", id, err)
		}
		if rec.SupersededBy == nil || *rec.SupersededBy != newID {
			t.Errorf("target %s.SupersededBy = %v, want %q", id, rec.SupersededBy, newID)
		}
	}
	newRec, err := d.st.Get(ctx, newID)
	if err != nil {
		t.Fatalf("Get(new record): %v", err)
	}
	if got, want := newRec.Supersedes, preview.GetSupersedes(); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("newRec.Supersedes = %v, want the preview's supersedes %v", got, want)
	}
}
