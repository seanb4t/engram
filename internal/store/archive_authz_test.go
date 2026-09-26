// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestArchiveAsOwnerGate is the phase's FIRST test (SC1, D-05): it proves,
// entirely inside internal/store — never a handler — that Store.ArchiveAs
// enforces the owner-only gate the way Delete/Update/Supersede already do.
// An owner archives their own record (private or shared); an authenticated
// non-owner and an anonymous caller are both rejected with ErrNotFound on an
// owned record, and the record is left un-archived by the rejection.
func TestArchiveAsOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-authz-test:project:owner-gate"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id1 := "ea000000-0000-0000-0000-000000000001" // private
	id2 := "ea000000-0000-0000-0000-000000000002" // shared
	if err := s.Upsert(ctx, Memory{ID: id1, Content: "v", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert id1: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: id2, Content: "v", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert id2: %v", err)
	}

	t.Run("shared reader cannot archive", func(t *testing.T) {
		res, err := s.ArchiveAs(ctx, id2, Authenticated("sub-A"))
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("ArchiveAs by shared reader: want ErrNotFound, got %v", err)
		}
		if res.Outcome != ArchiveOutcomeNotFound {
			t.Errorf("ArchiveAs by shared reader: Outcome = %q, want %q", res.Outcome, ArchiveOutcomeNotFound)
		}
		cur, gerr := s.Get(ctx, id2)
		if gerr != nil {
			t.Fatalf("Get id2 after rejected archive: %v", gerr)
		}
		if cur.ArchivedAt != nil {
			t.Errorf("id2 archived_at = %v, want nil (shared-reader archive must not mutate)", cur.ArchivedAt)
		}
	})

	t.Run("anonymous cannot archive an owned record", func(t *testing.T) {
		res, err := s.ArchiveAs(ctx, id1, Anonymous())
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("ArchiveAs by anonymous: want ErrNotFound, got %v", err)
		}
		if res.Outcome != ArchiveOutcomeNotFound {
			t.Errorf("ArchiveAs by anonymous: Outcome = %q, want %q", res.Outcome, ArchiveOutcomeNotFound)
		}
		cur, gerr := s.Get(ctx, id1)
		if gerr != nil {
			t.Fatalf("Get id1 after rejected archive: %v", gerr)
		}
		if cur.ArchivedAt != nil {
			t.Errorf("id1 archived_at = %v, want nil (anonymous archive must not mutate)", cur.ArchivedAt)
		}
	})

	t.Run("owner archives own private record", func(t *testing.T) {
		res, err := s.ArchiveAs(ctx, id1, Authenticated("sub-B"))
		if err != nil {
			t.Fatalf("owner ArchiveAs id1: %v", err)
		}
		if res.Outcome != ArchiveOutcomeChanged {
			t.Errorf("owner ArchiveAs id1: Outcome = %q, want %q", res.Outcome, ArchiveOutcomeChanged)
		}
		if res.ID != id1 {
			t.Errorf("owner ArchiveAs id1: ID = %q, want %q", res.ID, id1)
		}
		cur, gerr := s.Get(ctx, id1)
		if gerr != nil {
			t.Fatalf("Get id1 after owner archive: %v", gerr)
		}
		if cur.ArchivedAt == nil {
			t.Errorf("id1 archived_at = nil, want set after owner archive")
		}
	})

	t.Run("owner archives own shared record", func(t *testing.T) {
		res, err := s.ArchiveAs(ctx, id2, Authenticated("sub-B"))
		if err != nil {
			t.Fatalf("owner ArchiveAs id2: %v", err)
		}
		if res.Outcome != ArchiveOutcomeChanged {
			t.Errorf("owner ArchiveAs id2: Outcome = %q, want %q", res.Outcome, ArchiveOutcomeChanged)
		}
	})
}
