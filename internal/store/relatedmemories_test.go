// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
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

// TestRelatedMemoriesSupersessionChain pins D-06, D-09, D-11: the chain walk
// in both directions, appearing even when soft-hidden, one entry per
// candidate even when a member is reached by two edge types, and a broken
// chain (an unreadable member) never leaking structure past it.
func TestRelatedMemoriesSupersessionChain(t *testing.T) {
	ctx := context.Background()
	scope := "related:project:supersession-chain"
	ownerA := Authenticated("related-owner-a")

	s := newSpineTestStore(t, "related_supersession_chain")
	r0 := "e0e00000-0000-0000-0000-000000000000"
	r1 := "e0e00000-0000-0000-0000-000000000001"
	r2 := "e0e00000-0000-0000-0000-000000000002"
	r3 := "e0e00000-0000-0000-0000-000000000003"
	r4 := "e0e00000-0000-0000-0000-000000000004"

	seedSpineMemoryVector(t, s, Memory{
		ID: r0, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "r0",
		SupersededBy: qdrant.PtrOf(r1),
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: r1, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "r1",
		Supersedes: []string{r0}, SupersededBy: qdrant.PtrOf(r2),
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: r2, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "r2",
		Supersedes: []string{r1}, SupersededBy: qdrant.PtrOf(r3),
	}, []float32{0, 0, 1})
	seedSpineMemoryVector(t, s, Memory{
		ID: r3, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "r3",
		Supersedes: []string{r2}, SupersededBy: qdrant.PtrOf(r4),
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{
		ID: r4, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "r4",
		Supersedes: []string{r3},
	}, []float32{0, 0, 0.95}) // close to R2 (the anchor) — the live head also surfaces as a vector edge

	res, err := s.RelatedMemories(ctx, r2, ownerA, 0)
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}
	if len(res.Related) < 4 {
		t.Fatalf("len(Related) = %d, want at least 4 (%+v)", len(res.Related), res.Related)
	}
	wantOrder := []string{r3, r4, r1, r0}
	gotOrder := make([]string, 4)
	for i := 0; i < 4; i++ {
		gotOrder[i] = res.Related[i].Memory.ID
	}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("first four entries = %v, want %v (%+v)", gotOrder, wantOrder, res.Related)
	}

	find := func(id string) RelatedMemory {
		t.Helper()
		for _, r := range res.Related {
			if r.Memory.ID == id {
				return r
			}
		}
		t.Fatalf("%s missing from Related: %+v", id, res.Related)
		return RelatedMemory{}
	}
	requireSupersession := func(id string, dir SupersessionDirection, depth int) {
		t.Helper()
		r := find(id)
		for _, e := range r.Edges {
			if e.Type == RelatedEdgeSupersession {
				if e.Direction != dir || e.Depth != depth {
					t.Fatalf("%s supersession edge = %+v, want direction=%s depth=%d", id, e, dir, depth)
				}
				return
			}
		}
		t.Fatalf("%s has no supersession edge: %+v", id, r.Edges)
	}
	requireSupersession(r3, SupersessionSuccessor, 1)
	requireSupersession(r4, SupersessionSuccessor, 2)
	requireSupersession(r1, SupersessionPredecessor, 1)
	requireSupersession(r0, SupersessionPredecessor, 2)

	// R4 is the live head (no SupersededBy), so it is NOT excluded from the
	// gated vector edge the way R0/R1/R3 (all superseded) are — it carries
	// BOTH edges, in canonical order.
	r4Entry := find(r4)
	if len(r4Entry.Edges) != 2 || r4Entry.Edges[0].Type != RelatedEdgeSupersession || r4Entry.Edges[1].Type != RelatedEdgeVector {
		t.Fatalf("R4.Edges = %+v, want [supersession, vector]", r4Entry.Edges)
	}
	for _, id := range []string{r0, r1, r3} {
		for _, e := range find(id).Edges {
			if e.Type == RelatedEdgeVector {
				t.Fatalf("%s carries a vector edge, want none (superseded records are gated out, D-11)", id)
			}
		}
	}

	// Second fixture: a broken chain behind an unreadable member (D-09/D-11
	// second half) — X and Z are b-private, so GetReadable fails for both,
	// and X's own Supersedes (pointing at a-owned Y) is never followed.
	s2 := newSpineTestStore(t, "related_supersession_chain_broken")
	q := "e0e00000-0000-0000-0000-000000000010"
	x := "e0e00000-0000-0000-0000-000000000011"
	y := "e0e00000-0000-0000-0000-000000000012"
	z := "e0e00000-0000-0000-0000-000000000013"
	seedSpineMemoryVector(t, s2, Memory{
		ID: q, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "q",
		Supersedes: []string{x}, SupersededBy: qdrant.PtrOf(z),
	}, []float32{0, 1, 0})
	seedSpineMemoryVector(t, s2, Memory{
		ID: x, Scope: scope, Owner: "related-owner-b", Category: "note", Summary: "x",
		Supersedes: []string{y},
	}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s2, Memory{
		ID: y, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "y",
	}, []float32{1, 1, 1})
	seedSpineMemoryVector(t, s2, Memory{
		ID: z, Scope: scope, Owner: "related-owner-b", Category: "note", Summary: "z",
	}, []float32{0, 0, 1})

	res2, err := s2.RelatedMemories(ctx, q, ownerA, 0)
	if err != nil {
		t.Fatalf("RelatedMemories(Q): %v", err)
	}
	for _, r := range res2.Related {
		if r.Memory.ID == x || r.Memory.ID == z {
			t.Fatalf("Related contains %s, must be absent (unreadable to a)", r.Memory.ID)
		}
		for _, e := range r.Edges {
			if e.Type == RelatedEdgeSupersession {
				t.Fatalf("%s carries a supersession edge, want none (chain broken by an unreadable member)", r.Memory.ID)
			}
		}
	}
}

// TestRelatedMemoriesSupersessionCaps pins D-09's bounds: the per-direction
// depth cap and the total member cap.
func TestRelatedMemoriesSupersessionCaps(t *testing.T) {
	ctx := context.Background()
	ownerA := Authenticated("related-owner-a")

	t.Run("forward depth cap", func(t *testing.T) {
		s := newSpineTestStore(t, "related_supersession_caps_forward")
		scope := "related:project:supersession-caps-forward"
		c := make([]string, 11)
		for i := range c {
			c[i] = fmt.Sprintf("e1e00000-0000-0000-0000-0000000000%02x", i)
		}
		for i, id := range c {
			m := Memory{ID: id, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("c%d", i)}
			if i > 0 {
				m.Supersedes = []string{c[i-1]}
			}
			if i < len(c)-1 {
				m.SupersededBy = qdrant.PtrOf(c[i+1])
			}
			vec := []float32{1, 0, 0}
			if i == len(c)-1 {
				vec = []float32{0.99, 0.01, 0} // C10, the live head, also surfaces via the vector edge
			}
			seedSpineMemoryVector(t, s, m, vec)
		}

		res, err := s.RelatedMemories(ctx, c[0], ownerA, 0)
		if err != nil {
			t.Fatalf("RelatedMemories: %v", err)
		}
		for i := 1; i <= 8; i++ {
			found := false
			for _, r := range res.Related {
				if r.Memory.ID != c[i] {
					continue
				}
				found = true
				for _, e := range r.Edges {
					if e.Type == RelatedEdgeSupersession {
						if e.Depth != i {
							t.Fatalf("%s supersession depth = %d, want %d", c[i], e.Depth, i)
						}
					}
				}
			}
			if !found {
				t.Fatalf("C%d missing from Related: %+v", i, res.Related)
			}
		}
		for _, r := range res.Related {
			if r.Memory.ID == c[9] {
				t.Fatalf("C9 (beyond the depth cap) present in Related: %+v", res.Related)
			}
			for _, e := range r.Edges {
				if e.Type == RelatedEdgeSupersession && e.Depth > 8 {
					t.Fatalf("%s supersession depth = %d, want <= 8", r.Memory.ID, e.Depth)
				}
			}
		}
		for _, r := range res.Related {
			if r.Memory.ID != c[10] {
				continue
			}
			for _, e := range r.Edges {
				if e.Type == RelatedEdgeSupersession {
					t.Fatalf("C10 (the live head) carries a supersession edge: %+v", e)
				}
			}
		}
	})

	t.Run("backward member cap", func(t *testing.T) {
		s := newSpineTestStore(t, "related_supersession_caps_backward")
		scope := "related:project:supersession-caps-backward"
		p := "e2e00000-0000-0000-0000-0000000000ff"
		preds := make([]string, 20)
		for i := range preds {
			preds[i] = fmt.Sprintf("e2e00000-0000-0000-0000-0000000000%02x", i)
		}
		seedSpineMemoryVector(t, s, Memory{
			ID: p, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "p",
			Supersedes: preds,
		}, []float32{0, 0, 1})
		for i, id := range preds {
			seedSpineMemoryVector(t, s, Memory{
				ID: id, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("pred%d", i),
			}, []float32{1, 0, 0})
		}

		res, err := s.RelatedMemories(ctx, p, ownerA, 0)
		if err != nil {
			t.Fatalf("RelatedMemories: %v", err)
		}
		var gotPreds []string
		for _, r := range res.Related {
			for _, e := range r.Edges {
				if e.Type == RelatedEdgeSupersession {
					gotPreds = append(gotPreds, r.Memory.ID)
				}
			}
		}
		if len(gotPreds) != 16 {
			t.Fatalf("len(predecessor entries) = %d, want 16 (%v)", len(gotPreds), gotPreds)
		}
		wantLowest := append([]string(nil), preds[:16]...)
		gotSet := map[string]bool{}
		for _, id := range gotPreds {
			gotSet[id] = true
		}
		for _, id := range wantLowest {
			if !gotSet[id] {
				t.Fatalf("predecessor entries = %v, missing lowest id %s", gotPreds, id)
			}
		}
		for _, id := range preds[16:] {
			if gotSet[id] {
				t.Fatalf("predecessor entries = %v, must not include %s (beyond the member cap)", gotPreds, id)
			}
		}
	})
}

// TestRelatedMemoriesGatedEdgesFollowRecallGate pins D-11's gated-edge half:
// archived, superseded, expired, and not-yet-active scheduled candidates are
// excluded from the vector edge — only a live candidate comes back.
func TestRelatedMemoriesGatedEdgesFollowRecallGate(t *testing.T) {
	s := newSpineTestStore(t, "related_gated_edges")
	ctx := context.Background()
	scope := "related:project:gated-edges"
	ownerA := Authenticated("related-owner-a")
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	archivedAt := now
	unrelatedSuccessor := "f0f00000-0000-0000-0000-000000000099"

	anchor := Memory{ID: "f0f00000-0000-0000-0000-000000000000", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor"}
	seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})

	v1 := Memory{ID: "f0f00000-0000-0000-0000-000000000001", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "v1-archived", ArchivedAt: &archivedAt}
	seedSpineMemoryVector(t, s, v1, []float32{1, 0, 0})
	v2 := Memory{ID: "f0f00000-0000-0000-0000-000000000002", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "v2-superseded", SupersededBy: &unrelatedSuccessor}
	seedSpineMemoryVector(t, s, v2, []float32{1, 0, 0})
	v3 := Memory{ID: "f0f00000-0000-0000-0000-000000000003", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "v3-expired", NotAfter: &past}
	seedSpineMemoryVector(t, s, v3, []float32{1, 0, 0})
	v4 := Memory{ID: "f0f00000-0000-0000-0000-000000000004", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "v4-scheduled", NotBefore: &future}
	seedSpineMemoryVector(t, s, v4, []float32{1, 0, 0})
	v5 := Memory{ID: "f0f00000-0000-0000-0000-000000000005", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "v5-live"}
	seedSpineMemoryVector(t, s, v5, []float32{1, 0, 0})

	res, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0)
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}
	if len(res.Related) != 1 || res.Related[0].Memory.ID != v5.ID {
		t.Fatalf("Related = %+v, want exactly [%s]", res.Related, v5.ID)
	}
}

// TestRelatedMemoriesReadScope pins D-10: cross-spine reach across scopes,
// shared-record visibility, and anonymous-vs-authenticated bucket isolation.
func TestRelatedMemoriesReadScope(t *testing.T) {
	s := newSpineTestStore(t, "related_read_scope")
	ctx := context.Background()
	scopeOne := "related:project:one"
	scopeTwo := "related:project:two"
	ownerA := Authenticated("related-owner-a")

	anchorA := Memory{ID: "a0a00000-0000-0000-0000-000000000000", Scope: scopeOne, Owner: "related-owner-a", Category: "note", Summary: "anchor-a"}
	seedSpineMemoryVector(t, s, anchorA, []float32{1, 0, 0})

	nScopeTwo := Memory{ID: "a0a00000-0000-0000-0000-000000000001", Scope: scopeTwo, Owner: "related-owner-a", Category: "note", Summary: "n-scope-two"}
	seedSpineMemoryVector(t, s, nScopeTwo, []float32{1, 0, 0})

	nBShared := Memory{ID: "a0a00000-0000-0000-0000-000000000002", Scope: scopeOne, Owner: "related-owner-b", Visibility: "shared", Category: "note", Summary: "n-b-shared"}
	seedSpineMemoryVector(t, s, nBShared, []float32{1, 0, 0})

	nBPrivate := Memory{ID: "a0a00000-0000-0000-0000-000000000003", Scope: scopeOne, Owner: "related-owner-b", Category: "note", Summary: "n-b-private"}
	seedSpineMemoryVector(t, s, nBPrivate, []float32{1, 0, 0})

	res, err := s.RelatedMemories(ctx, anchorA.ID, ownerA, 0)
	if err != nil {
		t.Fatalf("RelatedMemories(a): %v", err)
	}
	got := map[string]string{}
	for _, r := range res.Related {
		got[r.Memory.ID] = r.Memory.Scope
	}
	if scope, ok := got[nScopeTwo.ID]; !ok || scope != scopeTwo {
		t.Fatalf("Related missing %s with Scope %q; got %+v", nScopeTwo.ID, scopeTwo, got)
	}
	if _, ok := got[nBShared.ID]; !ok {
		t.Fatalf("Related missing b-shared neighbour %s: %+v", nBShared.ID, got)
	}
	if _, ok := got[nBPrivate.ID]; ok {
		t.Fatalf("Related contains b-private neighbour %s, must be absent", nBPrivate.ID)
	}
	if len(res.Related) != 2 {
		t.Fatalf("len(Related) = %d, want 2 (%+v)", len(res.Related), res.Related)
	}

	anon := Anonymous()
	anchorAnon := Memory{ID: "a0a00000-0000-0000-0000-000000000004", Scope: scopeOne, Owner: "", Category: "note", Summary: "anchor-anon"}
	seedSpineMemoryVector(t, s, anchorAnon, []float32{1, 0, 0})
	nAnon := Memory{ID: "a0a00000-0000-0000-0000-000000000005", Scope: scopeOne, Owner: "", Category: "note", Summary: "n-anon"}
	seedSpineMemoryVector(t, s, nAnon, []float32{1, 0, 0})

	resAnon, err := s.RelatedMemories(ctx, anchorAnon.ID, anon, 0)
	if err != nil {
		t.Fatalf("RelatedMemories(anon): %v", err)
	}
	if len(resAnon.Related) != 1 || resAnon.Related[0].Memory.ID != nAnon.ID {
		t.Fatalf("Related(anon) = %+v, want exactly [%s] (b-shared and a-private must stay invisible)", resAnon.Related, nAnon.ID)
	}
}

// TestRelatedMemoriesBounds pins D-12's boundary edge: the vector cap
// default and caller override, the shared MaxRecallLimit rejection, and the
// total ceiling's Truncated signal.
func TestRelatedMemoriesBounds(t *testing.T) {
	ctx := context.Background()
	ownerA := Authenticated("related-owner-a")

	t.Run("k caps and rejection", func(t *testing.T) {
		s := newSpineTestStore(t, "related_bounds_k")
		scope := "related:project:bounds-k"
		anchor := Memory{ID: "b0b00000-0000-0000-0000-000000000000", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor"}
		seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})
		for i := 0; i < 12; i++ {
			id := fmt.Sprintf("b0b00000-0000-0000-0000-0000000001%02x", i)
			m := Memory{ID: id, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("n%d", i)}
			seedSpineMemoryVector(t, s, m, []float32{1 - float32(i)*0.01, float32(i) * 0.01, 0})
		}

		res0, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0)
		if err != nil {
			t.Fatalf("RelatedMemories(k=0): %v", err)
		}
		if len(res0.Related) != 8 {
			t.Fatalf("k=0: len(Related) = %d, want 8 (%+v)", len(res0.Related), res0.Related)
		}

		res3, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 3)
		if err != nil {
			t.Fatalf("RelatedMemories(k=3): %v", err)
		}
		if len(res3.Related) != 3 {
			t.Fatalf("k=3: len(Related) = %d, want 3 (%+v)", len(res3.Related), res3.Related)
		}

		_, err = s.RelatedMemories(ctx, anchor.ID, ownerA, MaxRecallLimit+1)
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("RelatedMemories(k=MaxRecallLimit+1) error = %v, want ErrInvalidArgument", err)
		}
		if !strings.Contains(err.Error(), "k") {
			t.Fatalf("RelatedMemories(k=MaxRecallLimit+1) error = %q, want it to name %q", err.Error(), "k")
		}
	})

	t.Run("total ceiling and truncation", func(t *testing.T) {
		s := newSpineTestStore(t, "related_bounds_ceiling")
		scope := "related:project:bounds-ceiling"
		anchor := Memory{ID: "b0b00000-0000-0000-0000-000000000000", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor"}
		seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})
		for i := 0; i < 70; i++ {
			id := fmt.Sprintf("b0b00000-0000-0000-0000-0000000002%02x", i)
			m := Memory{ID: id, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("n%d", i)}
			seedSpineMemoryVector(t, s, m, []float32{1, float32(i) * 0.001, 0})
		}

		res1000, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 1000)
		if err != nil {
			t.Fatalf("RelatedMemories(k=1000): %v", err)
		}
		if len(res1000.Related) != 64 {
			t.Fatalf("k=1000: len(Related) = %d, want 64 (%d)", len(res1000.Related), len(res1000.Related))
		}
		if !res1000.Truncated {
			t.Fatalf("k=1000: Truncated = false, want true")
		}

		res0, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0)
		if err != nil {
			t.Fatalf("RelatedMemories(k=0): %v", err)
		}
		if res0.Truncated {
			t.Fatalf("k=0: Truncated = true, want false")
		}
	})
}

// TestRelatedMemoriesDeterministic pins D-12's precision and idempotency
// edges: equal-score neighbours tiebreak on id ascending, and two
// consecutive calls over unchanged data are deep-equal.
func TestRelatedMemoriesDeterministic(t *testing.T) {
	s := newSpineTestStore(t, "related_deterministic")
	ctx := context.Background()
	scope := "related:project:deterministic"
	ownerA := Authenticated("related-owner-a")

	anchor := Memory{ID: "d0d00000-0000-0000-0000-000000000000", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor"}
	seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})

	// n1's id sorts before n2's; both carry the IDENTICAL vector, so their
	// scores tie and the id-ascending tiebreak decides order.
	n1 := Memory{ID: "d0d00000-0000-0000-0000-000000000001", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "n1"}
	seedSpineMemoryVector(t, s, n1, []float32{0.5, 0.5, 0})
	n2 := Memory{ID: "d0d00000-0000-0000-0000-000000000002", Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "n2"}
	seedSpineMemoryVector(t, s, n2, []float32{0.5, 0.5, 0})

	res1, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0)
	if err != nil {
		t.Fatalf("RelatedMemories (1st call): %v", err)
	}
	if len(res1.Related) != 2 || res1.Related[0].Memory.ID != n1.ID || res1.Related[1].Memory.ID != n2.ID {
		t.Fatalf("Related = %+v, want [%s, %s] (id-ascending tiebreak)", res1.Related, n1.ID, n2.ID)
	}

	res2, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0)
	if err != nil {
		t.Fatalf("RelatedMemories (2nd call): %v", err)
	}
	if !reflect.DeepEqual(res1, res2) {
		t.Fatalf("two consecutive calls returned different results:\n1st: %+v\n2nd: %+v", res1, res2)
	}
}

// TestRelatedMemoriesEntryShape pins the summary-view entry shape: a chain
// member with a Summary and Citations comes back with Citations nil and
// Content empty; a vector neighbour with no Summary keeps its Content
// (no-summary backfill); the anchor itself carries no Citations.
func TestRelatedMemoriesEntryShape(t *testing.T) {
	s := newSpineTestStore(t, "related_entry_shape")
	ctx := context.Background()
	scope := "related:project:entry-shape"
	ownerA := Authenticated("related-owner-a")

	anchorID := "e5e00000-0000-0000-0000-000000000000"
	succID := "e5e00000-0000-0000-0000-000000000001"
	neighbourID := "e5e00000-0000-0000-0000-000000000002"

	anchor := Memory{
		ID: anchorID, Scope: scope, Owner: "related-owner-a", Category: "note",
		Summary: "anchor summary", Content: "anchor content",
		Citations:    []Citation{{Kind: "file", Ref: "anchor.go"}},
		SupersededBy: qdrant.PtrOf(succID),
	}
	seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})

	succ := Memory{
		ID: succID, Scope: scope, Owner: "related-owner-a", Category: "note",
		Summary: "succ summary", Content: "succ content should be hidden",
		Citations:  []Citation{{Kind: "file", Ref: "succ.go"}},
		Supersedes: []string{anchorID},
	}
	seedSpineMemoryVector(t, s, succ, []float32{0, 1, 0})

	neighbour := Memory{
		ID: neighbourID, Scope: scope, Owner: "related-owner-a", Category: "note",
		Content: "raw content for backfill",
	}
	seedSpineMemoryVector(t, s, neighbour, []float32{0.9, 0.1, 0})

	res, err := s.RelatedMemories(ctx, anchorID, ownerA, 0)
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}
	if res.Anchor.Citations != nil {
		t.Fatalf("Anchor.Citations = %+v, want nil", res.Anchor.Citations)
	}

	var succEntry, neighbourEntry *RelatedMemory
	for i := range res.Related {
		switch res.Related[i].Memory.ID {
		case succID:
			succEntry = &res.Related[i]
		case neighbourID:
			neighbourEntry = &res.Related[i]
		}
	}
	if succEntry == nil {
		t.Fatalf("Related missing successor %s: %+v", succID, res.Related)
	}
	if succEntry.Memory.Citations != nil || succEntry.Memory.Content != "" {
		t.Fatalf("successor entry = %+v, want Citations nil and Content empty (has a Summary)", succEntry.Memory)
	}
	if neighbourEntry == nil {
		t.Fatalf("Related missing vector neighbour %s: %+v", neighbourID, res.Related)
	}
	if neighbourEntry.Memory.Citations != nil {
		t.Fatalf("neighbour entry Citations = %+v, want nil", neighbourEntry.Memory.Citations)
	}
	if neighbourEntry.Memory.Content != "raw content for backfill" {
		t.Fatalf("neighbour entry Content = %q, want %q (no-summary backfill)", neighbourEntry.Memory.Content, "raw content for backfill")
	}
}
