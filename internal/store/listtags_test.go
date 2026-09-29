// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"reflect"
	"strings"
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
// come back as exact, sorted counts, and another
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

// TestListTagsRecallVisibleOnly pins D-13: counts cover only the owner's
// live, recall-visible records — archived, superseded, expired, and
// not-yet-active scheduled records (and their tags) never appear, and a tag
// shared with a live record counts only the live one.
func TestListTagsRecallVisibleOnly(t *testing.T) {
	s := newSpineTestStore(t, "listtags_recallvisible")
	ctx := context.Background()
	scope := "listtags:project:recallvisible"
	const owner = "listtags-owner-a"
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	otherID := "cccccccc-0000-0000-0000-000000000099"
	archivedAt := now

	seedSpineMemory(t, s, Memory{ID: "cccccccc-0000-0000-0000-000000000001", Content: "live", Scope: scope, Owner: owner, Category: "note", Tags: []string{"go", "live-only"}, CreatedAt: now})
	seedSpineMemory(t, s, Memory{ID: "cccccccc-0000-0000-0000-000000000002", Content: "archived", Scope: scope, Owner: owner, Category: "note", Tags: []string{"go", "archived-only"}, CreatedAt: now, ArchivedAt: &archivedAt})
	seedSpineMemory(t, s, Memory{ID: "cccccccc-0000-0000-0000-000000000003", Content: "superseded", Scope: scope, Owner: owner, Category: "note", Tags: []string{"go", "superseded-only"}, CreatedAt: now, SupersededBy: &otherID})
	seedSpineMemory(t, s, Memory{ID: "cccccccc-0000-0000-0000-000000000004", Content: "expired", Scope: scope, Owner: owner, Category: "note", Tags: []string{"go", "expired-only"}, CreatedAt: now, NotAfter: &past})
	seedSpineMemory(t, s, Memory{ID: "cccccccc-0000-0000-0000-000000000005", Content: "scheduled", Scope: scope, Owner: owner, Category: "note", Tags: []string{"go", "scheduled-only"}, CreatedAt: now, NotBefore: &future})

	got, more, err := s.ListTags(ctx, Authenticated(owner), "", 0)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	want := []TagCount{{Tag: "go", Count: 1}, {Tag: "live-only", Count: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListTags = %+v, want %+v", got, want)
	}
	if more {
		t.Fatalf("ListTags more = true, want false")
	}
}

// TestListTagsReadFilter pins the V4 authz read filter: an authenticated
// caller counts their own private tags plus a shared record's tags, but
// never another owner's private tags; an anonymous caller counts only the
// ownerless bucket and never shared records; a nil Subject fails closed to
// an empty, non-nil slice.
func TestListTagsReadFilter(t *testing.T) {
	s := newSpineTestStore(t, "listtags_readfilter")
	ctx := context.Background()
	scope := "listtags:project:readfilter"
	now := time.Now().UTC()

	seedSpineMemory(t, s, Memory{ID: "dddddddd-0000-0000-0000-000000000001", Content: "a-private", Scope: scope, Owner: "listtags-owner-a", Category: "note", Tags: []string{"go"}, CreatedAt: now})
	seedSpineMemory(t, s, Memory{ID: "dddddddd-0000-0000-0000-000000000002", Content: "b-private", Scope: scope, Owner: "listtags-owner-b", Category: "note", Tags: []string{"go", "b-secret"}, CreatedAt: now})
	seedSpineMemory(t, s, Memory{ID: "dddddddd-0000-0000-0000-000000000003", Content: "b-shared", Scope: scope, Owner: "listtags-owner-b", Visibility: "shared", Category: "note", Tags: []string{"go", "b-shared"}, CreatedAt: now})
	seedSpineMemory(t, s, Memory{ID: "dddddddd-0000-0000-0000-000000000004", Content: "ownerless", Scope: scope, Owner: "", Category: "note", Tags: []string{"anon-tag"}, CreatedAt: now})

	t.Run("owner sees own private plus readable shared", func(t *testing.T) {
		got, more, err := s.ListTags(ctx, Authenticated("listtags-owner-a"), "", 0)
		if err != nil {
			t.Fatalf("ListTags(a): %v", err)
		}
		want := []TagCount{{Tag: "go", Count: 2}, {Tag: "b-shared", Count: 1}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ListTags(a) = %+v, want %+v", got, want)
		}
		if more {
			t.Fatalf("ListTags(a) more = true, want false")
		}
	})

	t.Run("anonymous sees only the ownerless bucket, never shared", func(t *testing.T) {
		got, more, err := s.ListTags(ctx, Anonymous(), "", 0)
		if err != nil {
			t.Fatalf("ListTags(anonymous): %v", err)
		}
		want := []TagCount{{Tag: "anon-tag", Count: 1}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ListTags(anonymous) = %+v, want %+v", got, want)
		}
		if more {
			t.Fatalf("ListTags(anonymous) more = true, want false")
		}
	})

	t.Run("nil Subject fails closed to an empty, non-nil slice", func(t *testing.T) {
		got, more, err := s.ListTags(ctx, nil, "", 0)
		if err != nil {
			t.Fatalf("ListTags(nil): %v", err)
		}
		if got == nil {
			t.Fatal("ListTags(nil) returned a nil slice, want a non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("ListTags(nil) = %+v, want empty", got)
		}
		if more {
			t.Fatalf("ListTags(nil) more = true, want false")
		}
	})
}

// TestListTagsScope pins D-14: a named scope counts only that scope's
// records; an empty scope counts across every scope the caller can read.
func TestListTagsScope(t *testing.T) {
	s := newSpineTestStore(t, "listtags_scope")
	ctx := context.Background()
	owner := Authenticated("listtags-owner-a")
	scopeX := "listtags:project:x"
	scopeY := "listtags:project:y"
	now := time.Now().UTC()

	seedSpineMemory(t, s, Memory{ID: "eeeeeeee-0000-0000-0000-000000000001", Content: "x", Scope: scopeX, Owner: "listtags-owner-a", Category: "note", Tags: []string{"go", "x-only"}, CreatedAt: now})
	seedSpineMemory(t, s, Memory{ID: "eeeeeeee-0000-0000-0000-000000000002", Content: "y", Scope: scopeY, Owner: "listtags-owner-a", Category: "note", Tags: []string{"go", "y-only"}, CreatedAt: now})

	gotX, moreX, err := s.ListTags(ctx, owner, scopeX, 0)
	if err != nil {
		t.Fatalf("ListTags(scope=x): %v", err)
	}
	wantX := []TagCount{{Tag: "go", Count: 1}, {Tag: "x-only", Count: 1}}
	if !reflect.DeepEqual(gotX, wantX) {
		t.Fatalf("ListTags(scope=x) = %+v, want %+v", gotX, wantX)
	}
	if moreX {
		t.Fatalf("ListTags(scope=x) more = true, want false")
	}

	gotAll, moreAll, err := s.ListTags(ctx, owner, "", 0)
	if err != nil {
		t.Fatalf("ListTags(scope=\"\"): %v", err)
	}
	wantAll := []TagCount{{Tag: "go", Count: 2}, {Tag: "x-only", Count: 1}, {Tag: "y-only", Count: 1}}
	if !reflect.DeepEqual(gotAll, wantAll) {
		t.Fatalf("ListTags(scope=\"\") = %+v, want %+v", gotAll, wantAll)
	}
	if moreAll {
		t.Fatalf("ListTags(scope=\"\") more = true, want false")
	}
}

// TestListTagsMatchesListTotals pins D-13's parity contract (the
// transparency prohibition: a returned count must never disagree with what
// filtering by that tag returns) over a mixed fixture spanning live,
// archived, superseded, expired, scheduled, another owner's private, and a
// shared record with overlapping tags across two scopes — checked for both
// an empty (cross-spine) scope and one named scope. It also pins that
// ListTags is read-only: two consecutive calls agree exactly.
func TestListTagsMatchesListTotals(t *testing.T) {
	s := newSpineTestStore(t, "listtags_parity")
	ctx := context.Background()
	owner := Authenticated("listtags-owner-a")
	scopeP1 := "listtags:project:parity1"
	scopeP2 := "listtags:project:parity2"
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	otherID := "ffffffff-0000-0000-0000-000000000099"
	archivedAt := now

	seedSpineMemory(t, s, Memory{ID: "ffffffff-0000-0000-0000-000000000001", Content: "live", Scope: scopeP1, Owner: "listtags-owner-a", Category: "note", Tags: []string{"go", "alpha"}, CreatedAt: now})
	seedSpineMemory(t, s, Memory{ID: "ffffffff-0000-0000-0000-000000000002", Content: "archived", Scope: scopeP1, Owner: "listtags-owner-a", Category: "note", Tags: []string{"go", "archived-only"}, CreatedAt: now, ArchivedAt: &archivedAt})
	seedSpineMemory(t, s, Memory{ID: "ffffffff-0000-0000-0000-000000000003", Content: "superseded", Scope: scopeP1, Owner: "listtags-owner-a", Category: "note", Tags: []string{"go", "superseded-only"}, CreatedAt: now, SupersededBy: &otherID})
	seedSpineMemory(t, s, Memory{ID: "ffffffff-0000-0000-0000-000000000004", Content: "expired", Scope: scopeP1, Owner: "listtags-owner-a", Category: "note", Tags: []string{"go", "expired-only"}, CreatedAt: now, NotAfter: &past})
	seedSpineMemory(t, s, Memory{ID: "ffffffff-0000-0000-0000-000000000005", Content: "scheduled", Scope: scopeP1, Owner: "listtags-owner-a", Category: "note", Tags: []string{"go", "scheduled-only"}, CreatedAt: now, NotBefore: &future})
	seedSpineMemory(t, s, Memory{ID: "ffffffff-0000-0000-0000-000000000006", Content: "b-private", Scope: scopeP1, Owner: "listtags-owner-b", Category: "note", Tags: []string{"go", "b-secret"}, CreatedAt: now})
	seedSpineMemory(t, s, Memory{ID: "ffffffff-0000-0000-0000-000000000007", Content: "b-shared", Scope: scopeP1, Owner: "listtags-owner-b", Visibility: "shared", Category: "note", Tags: []string{"go", "b-shared", "alpha"}, CreatedAt: now})
	seedSpineMemory(t, s, Memory{ID: "ffffffff-0000-0000-0000-000000000008", Content: "p2-live", Scope: scopeP2, Owner: "listtags-owner-a", Category: "note", Tags: []string{"go", "beta"}, CreatedAt: now})

	checkParity := func(t *testing.T, scope string) {
		t.Helper()
		got, _, err := s.ListTags(ctx, owner, scope, 0)
		if err != nil {
			t.Fatalf("ListTags(scope=%q): %v", scope, err)
		}
		if len(got) == 0 {
			t.Fatalf("ListTags(scope=%q) returned no tags — fixture setup is wrong", scope)
		}
		for _, tc := range got {
			_, total, _, err := s.List(ctx, scope, owner, ListOptions{Tags: []string{tc.Tag}, Limit: MaxRecallLimit})
			if err != nil {
				t.Fatalf("List(scope=%q, tag=%q): %v", scope, tc.Tag, err)
			}
			if total != tc.Count {
				t.Errorf("scope=%q tag=%q: ListTags count=%d, List total=%d", scope, tc.Tag, tc.Count, total)
			}
		}
	}

	t.Run("scope empty (cross-spine)", func(t *testing.T) { checkParity(t, "") })
	t.Run("named scope", func(t *testing.T) { checkParity(t, scopeP1) })

	t.Run("idempotent: two consecutive calls agree", func(t *testing.T) {
		first, firstMore, err := s.ListTags(ctx, owner, "", 0)
		if err != nil {
			t.Fatalf("first ListTags: %v", err)
		}
		second, secondMore, err := s.ListTags(ctx, owner, "", 0)
		if err != nil {
			t.Fatalf("second ListTags: %v", err)
		}
		if !reflect.DeepEqual(first, second) || firstMore != secondMore {
			t.Fatalf("ListTags not idempotent: first=%+v (more=%v), second=%+v (more=%v)", first, firstMore, second, secondMore)
		}
	})
}

// TestListTagsLimitAndMore pins D-15: exact limit/more boundary behavior —
// truncation below the distinct-tag count, no truncation at exactly the
// limit, the default limit for 0, and rejection above MaxRecallLimit.
func TestListTagsLimitAndMore(t *testing.T) {
	s := newSpineTestStore(t, "listtags_limit")
	ctx := context.Background()
	owner := Authenticated("listtags-owner-a")
	scope := "listtags:project:limit"
	now := time.Now().UTC()

	seed := func(id, tag string) {
		seedSpineMemory(t, s, Memory{ID: id, Content: tag, Scope: scope, Owner: "listtags-owner-a", Category: "note", Tags: []string{tag}, CreatedAt: now})
	}
	seed("11111111-0000-0000-0000-000000000001", "alpha")
	seed("11111111-0000-0000-0000-000000000002", "alpha")
	seed("11111111-0000-0000-0000-000000000003", "alpha")
	seed("11111111-0000-0000-0000-000000000004", "beta")
	seed("11111111-0000-0000-0000-000000000005", "beta")
	seed("11111111-0000-0000-0000-000000000006", "gamma")
	seed("11111111-0000-0000-0000-000000000007", "gamma")

	t.Run("limit 1 truncates to the top tag", func(t *testing.T) {
		got, more, err := s.ListTags(ctx, owner, scope, 1)
		if err != nil {
			t.Fatalf("ListTags(limit=1): %v", err)
		}
		want := []TagCount{{Tag: "alpha", Count: 3}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ListTags(limit=1) = %+v, want %+v", got, want)
		}
		if !more {
			t.Fatal("ListTags(limit=1) more = false, want true")
		}
	})

	t.Run("limit 2 ties break by tag ascending", func(t *testing.T) {
		got, more, err := s.ListTags(ctx, owner, scope, 2)
		if err != nil {
			t.Fatalf("ListTags(limit=2): %v", err)
		}
		want := []TagCount{{Tag: "alpha", Count: 3}, {Tag: "beta", Count: 2}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ListTags(limit=2) = %+v, want %+v", got, want)
		}
		if !more {
			t.Fatal("ListTags(limit=2) more = false, want true")
		}
	})

	t.Run("limit exactly the distinct-tag count is not truncated", func(t *testing.T) {
		got, more, err := s.ListTags(ctx, owner, scope, 3)
		if err != nil {
			t.Fatalf("ListTags(limit=3): %v", err)
		}
		want := []TagCount{{Tag: "alpha", Count: 3}, {Tag: "beta", Count: 2}, {Tag: "gamma", Count: 2}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ListTags(limit=3) = %+v, want %+v", got, want)
		}
		if more {
			t.Fatal("ListTags(limit=3) more = true, want false")
		}
	})

	t.Run("limit 0 defaults to listTagsDefaultLimit", func(t *testing.T) {
		got, more, err := s.ListTags(ctx, owner, scope, 0)
		if err != nil {
			t.Fatalf("ListTags(limit=0): %v", err)
		}
		want := []TagCount{{Tag: "alpha", Count: 3}, {Tag: "beta", Count: 2}, {Tag: "gamma", Count: 2}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ListTags(limit=0) = %+v, want %+v", got, want)
		}
		if more {
			t.Fatal("ListTags(limit=0) more = true, want false")
		}
	})

	t.Run("limit above MaxRecallLimit is rejected", func(t *testing.T) {
		_, _, err := s.ListTags(ctx, owner, scope, MaxRecallLimit+1)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("ListTags(limit=MaxRecallLimit+1): want ErrInvalidArgument, got %v", err)
		}
		if !strings.Contains(err.Error(), "limit") {
			t.Fatalf("ListTags(limit=MaxRecallLimit+1) error %q does not name %q", err.Error(), "limit")
		}
	})
}
