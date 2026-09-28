// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"slices"
	"strings"
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

// TestRestoreAsOwnerGate mirrors TestArchiveAsOwnerGate for RestoreAs: a
// shared reader and an anonymous caller are rejected on an already-archived
// owned record (private or shared), leaving ArchivedAt set; the owner's
// RestoreAs clears it and the archive/restore round trip preserves Content
// and Tags.
func TestRestoreAsOwnerGate(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-authz-test:project:restore-gate"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id1 := "ea000000-0000-0000-0000-000000000010" // private
	id2 := "ea000000-0000-0000-0000-000000000011" // shared
	tags := []string{"tag-a", "tag-b"}
	if err := s.Upsert(ctx, Memory{ID: id1, Content: "v1", Tags: tags, Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert id1: %v", err)
	}
	if err := s.Upsert(ctx, Memory{ID: id2, Content: "v2", Tags: tags, Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert id2: %v", err)
	}
	if _, err := s.Archive(ctx, id1); err != nil {
		t.Fatalf("pre-archive id1: %v", err)
	}
	if _, err := s.Archive(ctx, id2); err != nil {
		t.Fatalf("pre-archive id2: %v", err)
	}

	t.Run("shared reader cannot restore", func(t *testing.T) {
		res, err := s.RestoreAs(ctx, id2, Authenticated("sub-A"))
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("RestoreAs by shared reader: want ErrNotFound, got %v", err)
		}
		if res.Outcome != ArchiveOutcomeNotFound {
			t.Errorf("RestoreAs by shared reader: Outcome = %q, want %q", res.Outcome, ArchiveOutcomeNotFound)
		}
		cur, gerr := s.Get(ctx, id2)
		if gerr != nil {
			t.Fatalf("Get id2 after rejected restore: %v", gerr)
		}
		if cur.ArchivedAt == nil {
			t.Errorf("id2 archived_at = nil, want still set (shared-reader restore must not mutate)")
		}
	})

	t.Run("anonymous cannot restore an owned record", func(t *testing.T) {
		res, err := s.RestoreAs(ctx, id1, Anonymous())
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("RestoreAs by anonymous: want ErrNotFound, got %v", err)
		}
		if res.Outcome != ArchiveOutcomeNotFound {
			t.Errorf("RestoreAs by anonymous: Outcome = %q, want %q", res.Outcome, ArchiveOutcomeNotFound)
		}
		cur, gerr := s.Get(ctx, id1)
		if gerr != nil {
			t.Fatalf("Get id1 after rejected restore: %v", gerr)
		}
		if cur.ArchivedAt == nil {
			t.Errorf("id1 archived_at = nil, want still set (anonymous restore must not mutate)")
		}
	})

	t.Run("owner restores own private record, content and tags preserved", func(t *testing.T) {
		res, err := s.RestoreAs(ctx, id1, Authenticated("sub-B"))
		if err != nil {
			t.Fatalf("owner RestoreAs id1: %v", err)
		}
		if res.Outcome != ArchiveOutcomeChanged {
			t.Errorf("owner RestoreAs id1: Outcome = %q, want %q", res.Outcome, ArchiveOutcomeChanged)
		}
		cur, gerr := s.Get(ctx, id1)
		if gerr != nil {
			t.Fatalf("Get id1 after owner restore: %v", gerr)
		}
		if cur.ArchivedAt != nil {
			t.Errorf("id1 archived_at = %v, want nil after owner restore", cur.ArchivedAt)
		}
		if cur.Content != "v1" {
			t.Errorf("id1 Content = %q, want %q (round trip must preserve content)", cur.Content, "v1")
		}
		if !slices.Equal(cur.Tags, tags) {
			t.Errorf("id1 Tags = %v, want %v (round trip must preserve tags)", cur.Tags, tags)
		}
	})

	t.Run("owner restores own shared record", func(t *testing.T) {
		res, err := s.RestoreAs(ctx, id2, Authenticated("sub-B"))
		if err != nil {
			t.Fatalf("owner RestoreAs id2: %v", err)
		}
		if res.Outcome != ArchiveOutcomeChanged {
			t.Errorf("owner RestoreAs id2: Outcome = %q, want %q", res.Outcome, ArchiveOutcomeChanged)
		}
	})
}

// TestArchiveAsRestoreAsOwnedRule pins D-03: the gated path allows the owner
// to archive and restore a record whose category is "rule"; a different
// authenticated caller who can read that (shared) rule cannot.
func TestArchiveAsRestoreAsOwnedRule(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-authz-test:project:owned-rule"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "ea000000-0000-0000-0000-000000000020"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "always branch + PR", Category: "rule", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if res, err := s.ArchiveAs(ctx, id, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) || res.Outcome != ArchiveOutcomeNotFound {
		t.Errorf("non-owner ArchiveAs rule: want ErrNotFound/not_found, got err=%v outcome=%q", err, res.Outcome)
	}
	if cur, gerr := s.Get(ctx, id); gerr != nil || cur.ArchivedAt != nil {
		t.Fatalf("rule should be un-archived after non-owner ArchiveAs: err=%v archivedAt=%v", gerr, cur.ArchivedAt)
	}

	if res, err := s.ArchiveAs(ctx, id, Authenticated("sub-B")); err != nil || res.Outcome != ArchiveOutcomeChanged {
		t.Errorf("owner ArchiveAs rule: want changed, got err=%v outcome=%q", err, res.Outcome)
	}

	if res, err := s.RestoreAs(ctx, id, Authenticated("sub-A")); !errors.Is(err, ErrNotFound) || res.Outcome != ArchiveOutcomeNotFound {
		t.Errorf("non-owner RestoreAs rule: want ErrNotFound/not_found, got err=%v outcome=%q", err, res.Outcome)
	}
	if cur, gerr := s.Get(ctx, id); gerr != nil || cur.ArchivedAt == nil {
		t.Fatalf("rule should still be archived after non-owner RestoreAs: err=%v archivedAt=%v", gerr, cur.ArchivedAt)
	}

	if res, err := s.RestoreAs(ctx, id, Authenticated("sub-B")); err != nil || res.Outcome != ArchiveOutcomeChanged {
		t.Errorf("owner RestoreAs rule: want changed, got err=%v outcome=%q", err, res.Outcome)
	}
}

// TestArchiveAsFailsClosed pins D-02 (no nil-means-operator sentinel) and
// D-04/DEC-xa6 (a readable-but-not-owned record is indistinguishable from a
// nonexistent id) for both ArchiveAs and RestoreAs.
func TestArchiveAsFailsClosed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-authz-test:project:fails-closed"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	owned := "ea000000-0000-0000-0000-000000000030"
	if err := s.Upsert(ctx, Memory{ID: owned, Content: "v", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert owned: %v", err)
	}

	t.Run("nil Subject", func(t *testing.T) {
		res, err := s.ArchiveAs(ctx, owned, nil)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("ArchiveAs(nil): want ErrNotFound, got %v", err)
		}
		if res.Outcome != ArchiveOutcomeNotFound {
			t.Errorf("ArchiveAs(nil): Outcome = %q, want %q", res.Outcome, ArchiveOutcomeNotFound)
		}
		if cur, gerr := s.Get(ctx, owned); gerr != nil || cur.ArchivedAt != nil {
			t.Fatalf("owned record mutated by ArchiveAs(nil): err=%v archivedAt=%v", gerr, cur.ArchivedAt)
		}

		res2, err2 := s.RestoreAs(ctx, owned, nil)
		if !errors.Is(err2, ErrNotFound) {
			t.Errorf("RestoreAs(nil): want ErrNotFound, got %v", err2)
		}
		if res2.Outcome != ArchiveOutcomeNotFound {
			t.Errorf("RestoreAs(nil): Outcome = %q, want %q", res2.Outcome, ArchiveOutcomeNotFound)
		}
		if cur, gerr := s.Get(ctx, owned); gerr != nil || cur.ArchivedAt != nil {
			t.Fatalf("owned record mutated by RestoreAs(nil): err=%v archivedAt=%v", gerr, cur.ArchivedAt)
		}
	})

	missing := "ea000000-0000-0000-0000-000000000099"
	foreign := "ea000000-0000-0000-0000-000000000031"
	if err := s.Upsert(ctx, Memory{ID: foreign, Content: "v", Scope: scope, Owner: "sub-B", Visibility: "shared", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert foreign: %v", err)
	}

	t.Run("missing id indistinguishable from readable-but-not-owned (ArchiveAs)", func(t *testing.T) {
		resMissing, errMissing := s.ArchiveAs(ctx, missing, Authenticated("sub-A"))
		resForeign, errForeign := s.ArchiveAs(ctx, foreign, Authenticated("sub-A"))
		if resMissing.Outcome != resForeign.Outcome {
			t.Errorf("Outcome mismatch: missing=%q foreign=%q", resMissing.Outcome, resForeign.Outcome)
		}
		if resMissing.ID != missing {
			t.Errorf("resMissing.ID = %q, want %q", resMissing.ID, missing)
		}
		if resForeign.ID != foreign {
			t.Errorf("resForeign.ID = %q, want %q", resForeign.ID, foreign)
		}
		normMissing := strings.ReplaceAll(errMissing.Error(), missing, "ID")
		normForeign := strings.ReplaceAll(errForeign.Error(), foreign, "ID")
		if normMissing != normForeign {
			t.Errorf("error text distinguishable: missing=%q foreign=%q", errMissing.Error(), errForeign.Error())
		}
	})

	t.Run("missing id indistinguishable from readable-but-not-owned (RestoreAs)", func(t *testing.T) {
		resMissing, errMissing := s.RestoreAs(ctx, missing, Authenticated("sub-A"))
		resForeign, errForeign := s.RestoreAs(ctx, foreign, Authenticated("sub-A"))
		if resMissing.Outcome != resForeign.Outcome {
			t.Errorf("Outcome mismatch: missing=%q foreign=%q", resMissing.Outcome, resForeign.Outcome)
		}
		if resMissing.ID != missing {
			t.Errorf("resMissing.ID = %q, want %q", resMissing.ID, missing)
		}
		if resForeign.ID != foreign {
			t.Errorf("resForeign.ID = %q, want %q", resForeign.ID, foreign)
		}
		normMissing := strings.ReplaceAll(errMissing.Error(), missing, "ID")
		normForeign := strings.ReplaceAll(errForeign.Error(), foreign, "ID")
		if normMissing != normForeign {
			t.Errorf("error text distinguishable: missing=%q foreign=%q", errMissing.Error(), errForeign.Error())
		}
	})
}

// TestArchiveAsAnonymousBucket pins that an ownerless record (Owner == "")
// is archivable/restorable by Anonymous() — own_records matches the
// anonymous bucket the same way it matches any other owner.
func TestArchiveAsAnonymousBucket(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-authz-test:project:anon-bucket"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "ea000000-0000-0000-0000-000000000040"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "v", Scope: scope, Owner: "", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if res, err := s.ArchiveAs(ctx, id, Anonymous()); err != nil || res.Outcome != ArchiveOutcomeChanged {
		t.Errorf("anonymous ArchiveAs ownerless record: want changed, got err=%v outcome=%q", err, res.Outcome)
	}
	if res, err := s.RestoreAs(ctx, id, Anonymous()); err != nil || res.Outcome != ArchiveOutcomeChanged {
		t.Errorf("anonymous RestoreAs ownerless record: want changed, got err=%v outcome=%q", err, res.Outcome)
	}
}

// TestArchiveAsIdempotent pins that a second gated ArchiveAs/RestoreAs on an
// already-settled owned record reports ArchiveOutcomeAlready with a nil
// error and no further mutation — mirroring TestArchiveIdempotent for the
// gated path.
func TestArchiveAsIdempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	scope := "archive-authz-test:project:idempotent"
	defer func() { cleanupErr(t, "DeleteAllRaw "+scope, s.DeleteAllRaw(ctx, scope)) }()

	id := "ea000000-0000-0000-0000-000000000050"
	if err := s.Upsert(ctx, Memory{ID: id, Content: "v", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	res1, err := s.ArchiveAs(ctx, id, Authenticated("sub-B"))
	if err != nil {
		t.Fatalf("first ArchiveAs: %v", err)
	}
	if res1.Outcome != ArchiveOutcomeChanged {
		t.Fatalf("first ArchiveAs: Outcome = %q, want %q", res1.Outcome, ArchiveOutcomeChanged)
	}
	first, gerr := s.Get(ctx, id)
	if gerr != nil {
		t.Fatalf("Get after first ArchiveAs: %v", gerr)
	}
	if first.ArchivedAt == nil {
		t.Fatalf("Get after first ArchiveAs: ArchivedAt = nil, want set")
	}

	res2, err := s.ArchiveAs(ctx, id, Authenticated("sub-B"))
	if err != nil {
		t.Errorf("second ArchiveAs: %v", err)
	}
	if res2.Outcome != ArchiveOutcomeAlready {
		t.Errorf("second ArchiveAs: Outcome = %q, want %q", res2.Outcome, ArchiveOutcomeAlready)
	}
	second, gerr := s.Get(ctx, id)
	if gerr != nil {
		t.Fatalf("Get after second ArchiveAs: %v", gerr)
	}
	if !second.ArchivedAt.Equal(*first.ArchivedAt) {
		t.Errorf("second ArchiveAs changed the stamp: first=%v second=%v", first.ArchivedAt, second.ArchivedAt)
	}

	neverArchived := "ea000000-0000-0000-0000-000000000051"
	if err := s.Upsert(ctx, Memory{ID: neverArchived, Content: "v", Scope: scope, Owner: "sub-B", CreatedAt: time.Now().UTC()}, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("upsert neverArchived: %v", err)
	}
	res3, err := s.RestoreAs(ctx, neverArchived, Authenticated("sub-B"))
	if err != nil {
		t.Errorf("RestoreAs on never-archived record: %v", err)
	}
	if res3.Outcome != ArchiveOutcomeAlready {
		t.Errorf("RestoreAs on never-archived record: Outcome = %q, want %q", res3.Outcome, ArchiveOutcomeAlready)
	}
	if cur, gerr := s.Get(ctx, neverArchived); gerr != nil || cur.ArchivedAt != nil {
		t.Errorf("RestoreAs on never-archived record mutated it: err=%v archivedAt=%v", gerr, cur.ArchivedAt)
	}
}
