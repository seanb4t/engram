// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"testing"
)

// TestRelatedMemoriesVectorEdge is the phase's tracer test (STORE-02, SC3,
// D-10, D-12): the anchor's read-filtered query-by-id vector neighbourhood,
// ordered by score descending, excludes another owner's private
// near-duplicate even when its vector is identical to the anchor's.
func TestRelatedMemoriesVectorEdge(t *testing.T) {
	s := newSpineTestStore(t, "related_vector")
	ctx := context.Background()
	scope := "related:project:vector"
	ownerA := Authenticated("related-owner-a")

	anchor := Memory{ID: "aaaa0000-0000-0000-0000-000000000001", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor"}
	seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})

	n1 := Memory{ID: "aaaa0000-0000-0000-0000-000000000002", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "n1"}
	seedSpineMemoryVector(t, s, n1, []float32{0.9, 0.1, 0})

	n2 := Memory{ID: "aaaa0000-0000-0000-0000-000000000003", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "n2"}
	seedSpineMemoryVector(t, s, n2, []float32{0.5, 0.5, 0})

	// f is another owner's PRIVATE record whose vector is the anchor's exact
	// vector — the strongest possible near-duplicate signal — and must still
	// never appear in a's neighbourhood (D-10).
	f := Memory{ID: "bbbb0000-0000-0000-0000-000000000001", Scope: scope, Owner: "related-owner-b", Category: "note", Summary: "f"}
	seedSpineMemoryVector(t, s, f, []float32{1, 0, 0})

	res, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0)
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}
	if res.Anchor.ID != anchor.ID {
		t.Fatalf("Anchor.ID = %q, want %q", res.Anchor.ID, anchor.ID)
	}
	if len(res.Related) != 2 {
		t.Fatalf("len(Related) = %d, want 2 (%+v)", len(res.Related), res.Related)
	}
	if res.Related[0].Memory.ID != n1.ID || res.Related[1].Memory.ID != n2.ID {
		t.Fatalf("Related order = [%s, %s], want [%s, %s]",
			res.Related[0].Memory.ID, res.Related[1].Memory.ID, n1.ID, n2.ID)
	}
	for i, r := range res.Related {
		if len(r.Edges) != 1 || r.Edges[0].Type != RelatedEdgeVector {
			t.Fatalf("Related[%d].Edges = %+v, want exactly one vector edge", i, r.Edges)
		}
		if r.Edges[0].Score <= 0 {
			t.Fatalf("Related[%d].Edges[0].Score = %v, want > 0", i, r.Edges[0].Score)
		}
	}
	if res.Related[0].Edges[0].Score < res.Related[1].Edges[0].Score {
		t.Fatalf("N1 score %v < N2 score %v, want N1 >= N2",
			res.Related[0].Edges[0].Score, res.Related[1].Edges[0].Score)
	}
	for _, r := range res.Related {
		if r.Memory.ID == f.ID || r.Memory.ID == anchor.ID {
			t.Fatalf("Related contains %s, must be absent", r.Memory.ID)
		}
	}
}

// TestRelatedMemoriesAnchorAccess pins the anchor-resolution rules: an
// unreadable, nonexistent, or nil-Subject anchor is GetReadable's
// indistinguishable ErrNotFound; a readable anchor that is itself
// soft-hidden (archived) still returns its live neighbourhood.
func TestRelatedMemoriesAnchorAccess(t *testing.T) {
	s := newSpineTestStore(t, "related_anchor_access")
	ctx := context.Background()
	scope := "related:project:anchor-access"
	ownerA := Authenticated("related-owner-a")
	ownerB := Authenticated("related-owner-b")

	privateAnchor := Memory{ID: "cccc0000-0000-0000-0000-000000000001", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "private"}
	seedSpineMemoryVector(t, s, privateAnchor, []float32{1, 0, 0})

	if _, err := s.RelatedMemories(ctx, privateAnchor.ID, ownerB, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RelatedMemories(shared reader on private anchor) error = %v, want ErrNotFound", err)
	}

	if _, err := s.RelatedMemories(ctx, "dddd0000-0000-0000-0000-000000000099", ownerA, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RelatedMemories(nonexistent id) error = %v, want ErrNotFound", err)
	}

	if _, err := s.RelatedMemories(ctx, privateAnchor.ID, nil, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RelatedMemories(nil Subject) error = %v, want ErrNotFound", err)
	}

	// A separate collection: RelatedMemories reads cross-spine (D-10), so
	// sharing privateAnchor's collection here would make it a spurious
	// vector neighbour of archivedAnchor purely by owner match.
	s2 := newSpineTestStore(t, "related_anchor_access_archived")
	archivedAnchor := Memory{ID: "cccc0000-0000-0000-0000-000000000002", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "archived anchor"}
	seedSpineMemoryVector(t, s2, archivedAnchor, []float32{0, 1, 0})
	liveNeighbour := Memory{ID: "cccc0000-0000-0000-0000-000000000003", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "live neighbour"}
	seedSpineMemoryVector(t, s2, liveNeighbour, []float32{0, 0.9, 0.1})

	if _, err := s2.ArchiveAs(ctx, archivedAnchor.ID, ownerA); err != nil {
		t.Fatalf("ArchiveAs(archivedAnchor): %v", err)
	}

	res, err := s2.RelatedMemories(ctx, archivedAnchor.ID, ownerA, 0)
	if err != nil {
		t.Fatalf("RelatedMemories(archived anchor): %v", err)
	}
	if res.Anchor.ID != archivedAnchor.ID {
		t.Fatalf("Anchor.ID = %q, want %q", res.Anchor.ID, archivedAnchor.ID)
	}
	if len(res.Related) != 1 || res.Related[0].Memory.ID != liveNeighbour.ID {
		t.Fatalf("Related = %+v, want exactly [%s]", res.Related, liveNeighbour.ID)
	}
	if len(res.Related[0].Edges) != 1 || res.Related[0].Edges[0].Type != RelatedEdgeVector {
		t.Fatalf("Related[0].Edges = %+v, want exactly one vector edge", res.Related[0].Edges)
	}
}
