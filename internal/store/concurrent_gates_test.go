// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"
)

// TestArchiveAsRestoreAsConcurrentSerialize pins that concurrent gated
// ArchiveAs/RestoreAs calls on one id serialize through the core's per-id
// lock: every call succeeds with Changed or Already, the Changed outcomes
// alternate the state exactly once each (so the final archived state equals
// the parity of the Changed count), and the rest of the payload is never
// torn.
func TestArchiveAsRestoreAsConcurrentSerialize(t *testing.T) {
	s := newSpineTestStore(t, "archive_concurrent")
	ctx := context.Background()
	owner := Authenticated("race-owner")
	rec := Memory{
		ID: "ac000000-0000-0000-0000-000000000001", Content: "payload", Scope: "race:project:archive",
		Owner: "race-owner", Visibility: "shared", Category: "note", Tags: []string{"t1", "t2"},
		CreatedAt: time.Now().UTC(),
	}
	seedSpineMemoryVector(t, s, rec, []float32{0.1, 0.2, 0.3})

	const calls = 32
	outcomes := make([]ArchiveOutcome, calls)
	errs := make([]error, calls)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range calls {
		wg.Go(func() {
			<-start
			var res ArchiveResult
			if i%2 == 0 {
				res, errs[i] = s.ArchiveAs(ctx, rec.ID, owner)
			} else {
				res, errs[i] = s.RestoreAs(ctx, rec.ID, owner)
			}
			outcomes[i] = res.Outcome
		})
	}
	close(start)
	wg.Wait()

	changed := 0
	for i := range calls {
		if errs[i] != nil {
			t.Fatalf("call %d: %v", i, errs[i])
		}
		switch outcomes[i] {
		case ArchiveOutcomeChanged:
			changed++
		case ArchiveOutcomeAlready:
		default:
			t.Fatalf("call %d: Outcome = %q, want changed or already", i, outcomes[i])
		}
	}

	cur, err := s.Get(ctx, rec.ID)
	if err != nil {
		t.Fatalf("Get after race: %v", err)
	}
	if archived := cur.ArchivedAt != nil; archived != (changed%2 == 1) {
		t.Fatalf("archived = %v after %d Changed transitions from un-archived; transitions did not serialize", archived, changed)
	}
	if cur.Content != rec.Content || cur.Owner != rec.Owner || cur.Visibility != rec.Visibility || !reflect.DeepEqual(cur.Tags, rec.Tags) {
		t.Fatalf("payload torn by concurrent archive/restore: got %+v", cur)
	}
}

// TestListTagsUnderConcurrentWrites pins that ListTags' counts stay inside
// the caller's read and recall gates while writes race it: another owner's
// private records carrying the same tag never inflate the count, their
// private-only tag never appears, and the caller's own record being
// archived and restored moves the count only between the two legal values.
func TestListTagsUnderConcurrentWrites(t *testing.T) {
	s := newSpineTestStore(t, "listtags_concurrent")
	ctx := context.Background()
	scope := "race:project:listtags"
	ownerA := Authenticated("race-tags-a")

	ownIDs := make([]string, 3)
	for i := range ownIDs {
		ownIDs[i] = fmt.Sprintf("a1000000-0000-0000-0000-%012d", i)
		seedSpineMemory(t, s, Memory{
			ID: ownIDs[i], Content: "own", Scope: scope, Owner: "race-tags-a",
			Category: "note", Tags: []string{"hot"}, CreatedAt: time.Now().UTC(),
		})
	}

	done := make(chan struct{})
	writerErr := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Go(func() { // another owner's private records, same tag plus a private-only one
		for i := 0; ; i++ {
			select {
			case <-done:
				writerErr <- nil
				return
			default:
			}
			m := Memory{
				ID: fmt.Sprintf("b1000000-0000-0000-0000-%012d", i), Content: "other", Scope: scope,
				Owner: "race-tags-b", Category: "note", Tags: []string{"hot", "b-only"}, CreatedAt: time.Now().UTC(),
			}
			if err := s.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
				writerErr <- fmt.Errorf("b upsert %d: %w", i, err)
				return
			}
		}
	})
	wg.Go(func() { // the caller's own record toggling in and out of recall
		for i := 0; ; i++ {
			select {
			case <-done:
				writerErr <- nil
				return
			default:
			}
			op := s.ArchiveAs
			if i%2 == 1 {
				op = s.RestoreAs
			}
			if _, err := op(ctx, ownIDs[0], ownerA); err != nil {
				writerErr <- fmt.Errorf("toggle %d: %w", i, err)
				return
			}
		}
	})

	for i := range 25 {
		got, _, err := s.ListTags(ctx, ownerA, "", 0)
		if err != nil {
			close(done)
			wg.Wait()
			t.Fatalf("ListTags %d: %v", i, err)
		}
		for _, tc := range got {
			switch tc.Tag {
			case "hot":
				if tc.Count != 2 && tc.Count != 3 {
					close(done)
					wg.Wait()
					t.Fatalf("ListTags %d: hot = %d, want 2 or 3 (another owner's private records leaked or own count torn)", i, tc.Count)
				}
			default:
				close(done)
				wg.Wait()
				t.Fatalf("ListTags %d: unexpected tag %q (%+v)", i, tc.Tag, got)
			}
		}
	}
	close(done)
	wg.Wait()
	for range 2 {
		if err := <-writerErr; err != nil {
			t.Fatalf("writer: %v", err)
		}
	}
}

// TestRelatedMemoriesCandidateVanishesBeforeFetch pins that a candidate
// found by a sub-query but deleted, or made unreadable, before the payload
// fetch is dropped silently: no error, no Truncated signal, and the
// surviving candidates are unaffected. The window is hit deterministically
// through relatedBeforeFetchHook.
func TestRelatedMemoriesCandidateVanishesBeforeFetch(t *testing.T) {
	s := newSpineTestStore(t, "related_vanish")
	ctx := context.Background()
	scope := "race:project:related"
	ownerA := Authenticated("race-rel-a")
	ownerB := Authenticated("race-rel-b")

	anchor := Memory{ID: "ab000000-0000-0000-0000-000000000001", Scope: scope, Owner: "race-rel-a", Category: "note", Summary: "anchor"}
	seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})
	deleted := Memory{ID: "ab000000-0000-0000-0000-000000000002", Scope: scope, Owner: "race-rel-a", Category: "note", Summary: "deleted"}
	seedSpineMemoryVector(t, s, deleted, []float32{0.9, 0.1, 0})
	unshared := Memory{ID: "ab000000-0000-0000-0000-000000000003", Scope: scope, Owner: "race-rel-b", Visibility: "shared", Category: "note", Summary: "unshared"}
	seedSpineMemoryVector(t, s, unshared, []float32{0.8, 0.2, 0})
	survivor := Memory{ID: "ab000000-0000-0000-0000-000000000004", Scope: scope, Owner: "race-rel-a", Category: "note", Summary: "survivor"}
	seedSpineMemoryVector(t, s, survivor, []float32{0.7, 0.3, 0})

	relatedIDs := func(res RelatedResult) []string {
		ids := make([]string, 0, len(res.Related))
		for _, r := range res.Related {
			ids = append(ids, r.Memory.ID)
		}
		return ids
	}

	before, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories before: %v", err)
	}
	if want := []string{deleted.ID, unshared.ID, survivor.ID}; !reflect.DeepEqual(relatedIDs(before), want) {
		t.Fatalf("Related before = %v, want %v", relatedIDs(before), want)
	}

	fired := 0
	relatedBeforeFetchHook = func(ids []string) {
		fired++
		if !slices.Contains(ids, deleted.ID) || !slices.Contains(ids, unshared.ID) {
			t.Errorf("hook ids = %v, want both %s and %s", ids, deleted.ID, unshared.ID)
		}
		if err := s.Delete(ctx, deleted.ID, ownerA); err != nil {
			t.Errorf("Delete in hook: %v", err)
		}
		if err := s.SetVisibility(ctx, unshared.ID, ownerB, false); err != nil {
			t.Errorf("SetVisibility in hook: %v", err)
		}
	}
	t.Cleanup(func() { relatedBeforeFetchHook = nil })

	res, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories with vanishing candidates: %v", err)
	}
	if fired != 1 {
		t.Fatalf("hook fired %d times, want 1", fired)
	}
	if want := []string{survivor.ID}; !reflect.DeepEqual(relatedIDs(res), want) {
		t.Fatalf("Related = %v, want %v", relatedIDs(res), want)
	}
	if res.Truncated {
		t.Fatalf("Truncated = true, want false (a vanished candidate is not a truncation)")
	}
}

// TestRelatedMemoriesAdmitsByTypeNotScore pins D-12's prohibition on a
// blended cross-type score: a candidate reached only by a citation edge is
// admitted ahead of a near-identical vector neighbour, and each entry
// carries only its own type's evidence.
func TestRelatedMemoriesAdmitsByTypeNotScore(t *testing.T) {
	s := newSpineTestStore(t, "related_type_order")
	ctx := context.Background()
	scope := "race:project:type-order"
	ownerA := Authenticated("race-order-a")

	anchor := Memory{
		ID: "ad000000-0000-0000-0000-000000000001", Scope: scope, Owner: "race-order-a", Category: "note", Summary: "anchor",
		Citations: []Citation{{Kind: "file", Ref: "order.go"}},
	}
	seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})
	cited := Memory{
		ID: "ad000000-0000-0000-0000-000000000002", Scope: scope, Owner: "race-order-a", Category: "note", Summary: "cited",
		Citations: []Citation{{Kind: "file", Ref: "order.go"}},
	}
	seedSpineMemoryVector(t, s, cited, []float32{0, 0, 1}) // orthogonal: no vector affinity
	near := Memory{ID: "ad000000-0000-0000-0000-000000000003", Scope: scope, Owner: "race-order-a", Category: "note", Summary: "near"}
	seedSpineMemoryVector(t, s, near, []float32{0.99, 0.01, 0})

	res, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 1, false) // k=1: only near gets a vector edge
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}
	if len(res.Related) != 2 || res.Related[0].Memory.ID != cited.ID || res.Related[1].Memory.ID != near.ID {
		t.Fatalf("Related = %+v, want [cited, near]", res.Related)
	}
	c, n := res.Related[0], res.Related[1]
	if len(c.Edges) != 1 || c.Edges[0].Type != RelatedEdgeCitation || c.Edges[0].Score != 0 {
		t.Fatalf("cited.Edges = %+v, want one citation edge with no score", c.Edges)
	}
	if len(n.Edges) != 1 || n.Edges[0].Type != RelatedEdgeVector || n.Edges[0].Score <= 0 || len(n.Edges[0].SharedCitations) != 0 {
		t.Fatalf("near.Edges = %+v, want one vector edge carrying only a score", n.Edges)
	}
}
