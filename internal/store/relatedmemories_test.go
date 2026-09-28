// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// tagRarityID builds a deterministic, valid-UUID-shaped id for the tag-edge
// tests below — a shared helper so each test's fixture only has to think in
// terms of small integer indices.
func tagRarityID(prefix string, n int) string {
	return fmt.Sprintf("%s0000-0000-0000-0000-%012x", prefix, n)
}

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

	res, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0, false)
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

	if _, err := s.RelatedMemories(ctx, privateAnchor.ID, ownerB, 0, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RelatedMemories(shared reader on private anchor) error = %v, want ErrNotFound", err)
	}

	if _, err := s.RelatedMemories(ctx, "dddd0000-0000-0000-0000-000000000099", ownerA, 0, false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RelatedMemories(nonexistent id) error = %v, want ErrNotFound", err)
	}

	if _, err := s.RelatedMemories(ctx, privateAnchor.ID, nil, 0, false); !errors.Is(err, ErrNotFound) {
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

	res, err := s2.RelatedMemories(ctx, archivedAnchor.ID, ownerA, 0, false)
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

	res, err := s.RelatedMemories(ctx, r2, ownerA, 0, false)
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

	res2, err := s2.RelatedMemories(ctx, q, ownerA, 0, false)
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

		res, err := s.RelatedMemories(ctx, c[0], ownerA, 0, false)
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

		res, err := s.RelatedMemories(ctx, p, ownerA, 0, false)
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

	res, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0, false)
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

	res, err := s.RelatedMemories(ctx, anchorA.ID, ownerA, 0, false)
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

	resAnon, err := s.RelatedMemories(ctx, anchorAnon.ID, anon, 0, false)
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

		res0, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0, false)
		if err != nil {
			t.Fatalf("RelatedMemories(k=0): %v", err)
		}
		if len(res0.Related) != 8 {
			t.Fatalf("k=0: len(Related) = %d, want 8 (%+v)", len(res0.Related), res0.Related)
		}

		res3, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 3, false)
		if err != nil {
			t.Fatalf("RelatedMemories(k=3): %v", err)
		}
		if len(res3.Related) != 3 {
			t.Fatalf("k=3: len(Related) = %d, want 3 (%+v)", len(res3.Related), res3.Related)
		}

		_, err = s.RelatedMemories(ctx, anchor.ID, ownerA, MaxRecallLimit+1, false)
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

		res1000, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 1000, false)
		if err != nil {
			t.Fatalf("RelatedMemories(k=1000): %v", err)
		}
		if len(res1000.Related) != 64 {
			t.Fatalf("k=1000: len(Related) = %d, want 64 (%d)", len(res1000.Related), len(res1000.Related))
		}
		if !res1000.Truncated {
			t.Fatalf("k=1000: Truncated = false, want true")
		}

		res0, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0, false)
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

	res1, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories (1st call): %v", err)
	}
	if len(res1.Related) != 2 || res1.Related[0].Memory.ID != n1.ID || res1.Related[1].Memory.ID != n2.ID {
		t.Fatalf("Related = %+v, want [%s, %s] (id-ascending tiebreak)", res1.Related, n1.ID, n2.ID)
	}

	res2, err := s.RelatedMemories(ctx, anchor.ID, ownerA, 0, false)
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

	res, err := s.RelatedMemories(ctx, anchorID, ownerA, 0, false)
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

// tagEdgeOf returns the tag-type RelatedEdge for id in res, if any.
func tagEdgeOf(res RelatedResult, id string) (RelatedEdge, bool) {
	for _, r := range res.Related {
		if r.Memory.ID != id {
			continue
		}
		for _, e := range r.Edges {
			if e.Type == RelatedEdgeTag {
				return e, true
			}
		}
	}
	return RelatedEdge{}, false
}

// tagEdgeOrder returns, in res.Related's own order, the ids of every entry
// carrying a tag-type edge.
func tagEdgeOrder(res RelatedResult) []string {
	var ids []string
	for _, r := range res.Related {
		for _, e := range r.Edges {
			if e.Type == RelatedEdgeTag {
				ids = append(ids, r.Memory.ID)
				break
			}
		}
	}
	return ids
}

// assertWeightClose fails t unless got is within 1e-9 of want — floats are
// never compared with exact equality (per this plan's executor notes).
func assertWeightClose(t *testing.T, label string, got, want float64) {
	t.Helper()
	if diff := got - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("%s = %v, want %v (within 1e-9)", label, got, want)
	}
}

// TestRelatedMemoriesTagEdgeRarity is the tracer test for D-07: shared-tag
// edges carry exact rarity weights (ln(n/df)) drawn from the caller's
// readable, recall-visible set; a tag carried by more than half that set
// (here, "common") contributes zero weight and no edge; and the df implied
// by a weight matches ListTags' own count for the same tag.
func TestRelatedMemoriesTagEdgeRarity(t *testing.T) {
	s := newSpineTestStore(t, "related_tag_rarity")
	ctx := context.Background()
	scope := "related:project:tag-rarity"
	ownerA := Authenticated("related-owner-a")
	id := func(n int) string { return tagRarityID("1a1a", n) }

	anchorID, aID, bID, cID, dID, eID := id(0), id(1), id(2), id(3), id(4), id(5)

	seedSpineMemoryVector(t, s, Memory{ID: anchorID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor", Tags: []string{"t1", "t2", "common"}}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{ID: aID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "a", Tags: []string{"t1"}}, []float32{0.9, 0.1, 0})
	seedSpineMemoryVector(t, s, Memory{ID: bID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "b", Tags: []string{"t2"}}, []float32{0, 0.9, 0.1})
	seedSpineMemoryVector(t, s, Memory{ID: cID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "c", Tags: []string{"t1", "t2"}}, []float32{0, 0, 1})
	seedSpineMemoryVector(t, s, Memory{ID: dID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "d", Tags: []string{"t2"}}, []float32{0, 1, 0})
	seedSpineMemoryVector(t, s, Memory{ID: eID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "e", Tags: []string{"common"}}, []float32{1, 1, 0})
	fIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		fIDs[i] = id(6 + i)
		seedSpineMemoryVector(t, s, Memory{ID: fIDs[i], Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("f%d", i), Tags: []string{"common"}}, []float32{0.5, 0.5, 0.5})
	}

	res, err := s.RelatedMemories(ctx, anchorID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}

	wantOrder := []string{cID, aID, bID, dID}
	if gotOrder := tagEdgeOrder(res); !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("tag edge order = %v, want %v (%+v)", gotOrder, wantOrder, res.Related)
	}

	cEdge, ok := tagEdgeOf(res, cID)
	if !ok {
		t.Fatalf("C has no tag edge: %+v", res.Related)
	}
	assertWeightClose(t, "C.TagWeight", cEdge.TagWeight, math.Log(11.0/3.0)+math.Log(11.0/4.0))

	aEdge, ok := tagEdgeOf(res, aID)
	if !ok {
		t.Fatalf("A has no tag edge: %+v", res.Related)
	}
	assertWeightClose(t, "A.TagWeight", aEdge.TagWeight, math.Log(11.0/3.0))

	for _, r := range res.Related {
		for _, e := range r.Edges {
			for _, st := range e.SharedTags {
				if st.Tag == "common" {
					t.Fatalf("%s SharedTags contains %q, an ubiquitous tag that must contribute no weight: %+v", r.Memory.ID, st.Tag, e)
				}
			}
		}
	}
	for _, missing := range append([]string{eID}, fIDs...) {
		if _, ok := tagEdgeOf(res, missing); ok {
			t.Fatalf("%s has a tag edge, want none (only carries the ubiquitous 'common' tag)", missing)
		}
	}

	tagCounts, _, err := s.ListTags(ctx, ownerA, "", 0)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	dfByTag := map[string]uint64{}
	for _, tc := range tagCounts {
		dfByTag[tc.Tag] = tc.Count
	}
	if dfByTag["t1"] != 3 {
		t.Fatalf("ListTags df(t1) = %d, want 3", dfByTag["t1"])
	}
	if dfByTag["t2"] != 4 {
		t.Fatalf("ListTags df(t2) = %d, want 4", dfByTag["t2"])
	}
}

// TestRelatedMemoriesTagEdgeCap pins D-07/D-12's boundary and precision
// edges: exactly relatedTagCap (8) tag neighbours come back, chosen as the
// lowest ids among equal-weight candidates.
func TestRelatedMemoriesTagEdgeCap(t *testing.T) {
	s := newSpineTestStore(t, "related_tag_cap")
	ctx := context.Background()
	scope := "related:project:tag-cap"
	ownerA := Authenticated("related-owner-a")
	id := func(n int) string { return tagRarityID("1b1b", n) }

	anchorID := id(0)
	seedSpineMemoryVector(t, s, Memory{ID: anchorID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor", Tags: []string{"rare-cap"}}, []float32{1, 0, 0})

	candidateIDs := make([]string, 10)
	for i := 0; i < 10; i++ {
		candidateIDs[i] = id(1 + i)
		seedSpineMemoryVector(t, s, Memory{ID: candidateIDs[i], Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("c%d", i), Tags: []string{"rare-cap"}}, []float32{float32(i) * 0.01, 1 - float32(i)*0.01, 0})
	}
	for i := 0; i < 12; i++ {
		fID := id(11 + i)
		seedSpineMemoryVector(t, s, Memory{ID: fID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("filler%d", i)}, []float32{0, 0, 1})
	}

	res, err := s.RelatedMemories(ctx, anchorID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}

	gotOrder := tagEdgeOrder(res)
	if len(gotOrder) != 8 {
		t.Fatalf("len(tag edges) = %d, want 8: %v", len(gotOrder), gotOrder)
	}
	wantOrder := append([]string(nil), candidateIDs[:8]...)
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("tag edge order = %v, want %v (the 8 lowest candidate ids, ascending)", gotOrder, wantOrder)
	}
}

// TestRelatedMemoriesTagWeightBeyondFacetLimit pins D-07's completeness
// requirement: when relatedTagFacetLimit truncates the facet below the
// anchor's own tag, the missing tag's df still comes from an exact
// fallback Count, never an estimate.
func TestRelatedMemoriesTagWeightBeyondFacetLimit(t *testing.T) {
	orig := relatedTagFacetLimit
	relatedTagFacetLimit = 1
	t.Cleanup(func() { relatedTagFacetLimit = orig })

	s := newSpineTestStore(t, "related_tag_facet_limit")
	ctx := context.Background()
	scope := "related:project:tag-facet-limit"
	ownerA := Authenticated("related-owner-a")
	id := func(n int) string { return tagRarityID("1c1c", n) }

	anchorID := id(0)
	seedSpineMemoryVector(t, s, Memory{ID: anchorID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor", Tags: []string{"often", "seldom"}}, []float32{1, 0, 0})

	for i := 0; i < 4; i++ {
		oID := id(1 + i)
		seedSpineMemoryVector(t, s, Memory{ID: oID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("often%d", i), Tags: []string{"often"}}, []float32{0.5, 0.5, 0})
	}
	sID := id(5)
	seedSpineMemoryVector(t, s, Memory{ID: sID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "seldom-sharer", Tags: []string{"seldom"}}, []float32{0, 1, 0})
	for i := 0; i < 4; i++ {
		fID := id(6 + i)
		seedSpineMemoryVector(t, s, Memory{ID: fID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("filler%d", i)}, []float32{0, 0, 1})
	}

	res, err := s.RelatedMemories(ctx, anchorID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}

	sEdge, ok := tagEdgeOf(res, sID)
	if !ok {
		t.Fatalf("seldom-sharer has no tag edge: %+v", res.Related)
	}
	found := false
	for _, wt := range sEdge.SharedTags {
		if wt.Tag == "seldom" {
			found = true
			assertWeightClose(t, "seldom weight", wt.Weight, math.Log(10.0/2.0))
		}
	}
	if !found {
		t.Fatalf("seldom-sharer's SharedTags = %+v, want a 'seldom' entry", sEdge.SharedTags)
	}
	assertWeightClose(t, "seldom-sharer.TagWeight", sEdge.TagWeight, math.Log(10.0/2.0))
}

// citationEdgeOf returns the citation-type RelatedEdge for id in res, if any.
func citationEdgeOf(res RelatedResult, id string) (RelatedEdge, bool) {
	for _, r := range res.Related {
		if r.Memory.ID != id {
			continue
		}
		for _, e := range r.Edges {
			if e.Type == RelatedEdgeCitation {
				return e, true
			}
		}
	}
	return RelatedEdge{}, false
}

// citationEdgeOrder returns, in res.Related's own order, the ids of every
// entry carrying a citation-type edge.
func citationEdgeOrder(res RelatedResult) []string {
	var ids []string
	for _, r := range res.Related {
		for _, e := range r.Edges {
			if e.Type == RelatedEdgeCitation {
				ids = append(ids, r.Memory.ID)
				break
			}
		}
	}
	return ids
}

// TestRelatedMemoriesCitationEdge pins D-08's boundary: a shared citation is
// the same kind AND ref within one citation object, regardless of locator,
// pin, or excerpt; the same ref under a different kind is not a match; and
// exactly relatedCitationCap (8) citation neighbours come back at the cap.
func TestRelatedMemoriesCitationEdge(t *testing.T) {
	s := newSpineTestStore(t, "related_citation_edge")
	ctx := context.Background()
	scope := "related:project:citation-edge"
	ownerA := Authenticated("related-owner-a")
	id := func(n int) string { return tagRarityID("2a2a", n) }

	anchorID, c1ID, c2ID, c3ID := id(0), id(1), id(2), id(3)

	anchor := Memory{
		ID: anchorID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor",
		Citations: []Citation{
			{Kind: "file", Ref: "internal/x/a.go", Locator: "1-10", Pin: "abc"},
			{Kind: "url", Ref: "https://example.test/doc"},
		},
	}
	seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})

	c1 := Memory{
		ID: c1ID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "c1",
		Citations: []Citation{{Kind: "file", Ref: "internal/x/a.go", Locator: "50-60", Pin: "zzz", Excerpt: "other"}},
	}
	seedSpineMemoryVector(t, s, c1, []float32{0, 1, 0})

	c2 := Memory{
		ID: c2ID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "c2",
		Citations: []Citation{{Kind: "commit", Ref: "internal/x/a.go"}},
	}
	seedSpineMemoryVector(t, s, c2, []float32{0, 0, 1})

	c3 := Memory{
		ID: c3ID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "c3",
		Citations: []Citation{
			{Kind: "file", Ref: "internal/x/a.go"},
			{Kind: "url", Ref: "https://example.test/doc"},
		},
	}
	seedSpineMemoryVector(t, s, c3, []float32{0.5, 0.5, 0})

	res, err := s.RelatedMemories(ctx, anchorID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}

	wantOrder := []string{c3ID, c1ID}
	if gotOrder := citationEdgeOrder(res); !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("citation edge order = %v, want %v (%+v)", gotOrder, wantOrder, res.Related)
	}
	if _, ok := citationEdgeOf(res, c2ID); ok {
		t.Fatalf("c2 has a citation edge, want none (same ref, different kind)")
	}

	c1Edge, ok := citationEdgeOf(res, c1ID)
	if !ok {
		t.Fatalf("c1 has no citation edge: %+v", res.Related)
	}
	wantC1 := []CitationRef{{Kind: "file", Ref: "internal/x/a.go"}}
	if !reflect.DeepEqual(c1Edge.SharedCitations, wantC1) {
		t.Fatalf("c1.SharedCitations = %+v, want %+v", c1Edge.SharedCitations, wantC1)
	}

	c3Edge, ok := citationEdgeOf(res, c3ID)
	if !ok {
		t.Fatalf("c3 has no citation edge: %+v", res.Related)
	}
	if len(c3Edge.SharedCitations) != 2 {
		t.Fatalf("c3.SharedCitations = %+v, want 2 entries", c3Edge.SharedCitations)
	}

	t.Run("cap", func(t *testing.T) {
		s2 := newSpineTestStore(t, "related_citation_edge_cap")
		scope2 := "related:project:citation-edge-cap"
		id2 := func(n int) string { return tagRarityID("2b2b", n) }
		anchorID2 := id2(0)
		seedSpineMemoryVector(t, s2, Memory{
			ID: anchorID2, Scope: scope2, Owner: "related-owner-a", Category: "note", Summary: "anchor",
			Citations: []Citation{{Kind: "file", Ref: "cap.go"}},
		}, []float32{1, 0, 0})

		candidateIDs := make([]string, 10)
		for i := 0; i < 10; i++ {
			candidateIDs[i] = id2(1 + i)
			seedSpineMemoryVector(t, s2, Memory{
				ID: candidateIDs[i], Scope: scope2, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("c%d", i),
				Citations: []Citation{{Kind: "file", Ref: "cap.go"}},
			}, []float32{float32(i) * 0.01, 1 - float32(i)*0.01, 0})
		}

		res2, err := s2.RelatedMemories(ctx, anchorID2, ownerA, 0, false)
		if err != nil {
			t.Fatalf("RelatedMemories: %v", err)
		}
		gotOrder := citationEdgeOrder(res2)
		if len(gotOrder) != 8 {
			t.Fatalf("len(citation edges) = %d, want 8: %v", len(gotOrder), gotOrder)
		}
		wantOrder := append([]string(nil), candidateIDs[:8]...)
		if !reflect.DeepEqual(gotOrder, wantOrder) {
			t.Fatalf("citation edge order = %v, want %v (the 8 lowest ids, ascending)", gotOrder, wantOrder)
		}
	})
}

// TestRelatedMemoriesMultiEdgeEntry pins D-06: a candidate reached by
// citation, tag, AND vector edges is ONE entry, with Edges in canonical
// order (citation, tag, vector) each carrying non-empty/non-zero evidence.
func TestRelatedMemoriesMultiEdgeEntry(t *testing.T) {
	s := newSpineTestStore(t, "related_multi_edge_entry")
	ctx := context.Background()
	scope := "related:project:multi-edge"
	ownerA := Authenticated("related-owner-a")
	id := func(n int) string { return tagRarityID("4a4a", n) }

	anchorID, mID := id(0), id(1)
	anchor := Memory{
		ID: anchorID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor",
		Tags: []string{"rare-multi"}, Citations: []Citation{{Kind: "file", Ref: "multi.go"}},
	}
	seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})

	m := Memory{
		ID: mID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "m",
		Tags: []string{"rare-multi"}, Citations: []Citation{{Kind: "file", Ref: "multi.go"}},
	}
	seedSpineMemoryVector(t, s, m, []float32{0.99, 0.01, 0}) // near the anchor's vector

	for i := 0; i < 3; i++ {
		fID := id(2 + i)
		seedSpineMemoryVector(t, s, Memory{ID: fID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("filler%d", i)}, []float32{0, 1, 0})
	}

	res, err := s.RelatedMemories(ctx, anchorID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}

	var mEntries []RelatedMemory
	for _, r := range res.Related {
		if r.Memory.ID == mID {
			mEntries = append(mEntries, r)
		}
	}
	if len(mEntries) != 1 {
		t.Fatalf("M appears %d times in Related, want exactly 1: %+v", len(mEntries), mEntries)
	}
	mEntry := mEntries[0]
	wantTypes := []RelatedEdgeType{RelatedEdgeCitation, RelatedEdgeTag, RelatedEdgeVector}
	gotTypes := make([]RelatedEdgeType, len(mEntry.Edges))
	for i, e := range mEntry.Edges {
		gotTypes[i] = e.Type
	}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("M.Edges types = %v, want %v (%+v)", gotTypes, wantTypes, mEntry.Edges)
	}
	if len(mEntry.Edges[0].SharedCitations) == 0 {
		t.Fatalf("M's citation edge has no SharedCitations: %+v", mEntry.Edges[0])
	}
	if len(mEntry.Edges[1].SharedTags) == 0 || mEntry.Edges[1].TagWeight <= 0 {
		t.Fatalf("M's tag edge = %+v, want non-empty SharedTags and TagWeight > 0", mEntry.Edges[1])
	}
	if mEntry.Edges[2].Score <= 0 {
		t.Fatalf("M's vector edge Score = %v, want > 0", mEntry.Edges[2].Score)
	}
}

// TestRelatedMemoriesTagAndCitationEdgesFollowGates pins D-10/D-11 for the
// new edge types: archived, superseded, expired, scheduled, and b-private
// carriers of the anchor's rare tag and citation never surface at all; a
// b-shared carrier surfaces with BOTH edges for an authenticated caller and
// is invisible to an anonymous caller querying an ownerless anchor; and the
// rare tag's weight is computed over only the live readable carriers.
func TestRelatedMemoriesTagAndCitationEdgesFollowGates(t *testing.T) {
	s := newSpineTestStore(t, "related_gates_tag_citation")
	ctx := context.Background()
	scope := "related:project:gates-tag-citation"
	ownerA := Authenticated("related-owner-a")
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	archivedAt := now
	id := func(n int) string { return tagRarityID("5a5a", n) }

	anchorID := id(0)
	archivedID := id(1)
	supersededID := id(2)
	expiredID := id(3)
	scheduledID := id(4)
	bPrivateID := id(5)
	bSharedID := id(6)
	filler1ID := id(7)
	filler2ID := id(8)
	unrelatedSuccessor := id(9)

	tag := "rare-gate"
	citation := []Citation{{Kind: "file", Ref: "gate.go"}}

	seedSpineMemoryVector(t, s, Memory{ID: anchorID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor", Tags: []string{tag}, Citations: citation}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{ID: archivedID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "archived", Tags: []string{tag}, Citations: citation, ArchivedAt: &archivedAt}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{ID: supersededID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "superseded", Tags: []string{tag}, Citations: citation, SupersededBy: &unrelatedSuccessor}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{ID: expiredID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "expired", Tags: []string{tag}, Citations: citation, NotAfter: &past}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{ID: scheduledID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "scheduled", Tags: []string{tag}, Citations: citation, NotBefore: &future}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{ID: bPrivateID, Scope: scope, Owner: "related-owner-b", Category: "note", Summary: "b-private", Tags: []string{tag}, Citations: citation}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{ID: bSharedID, Scope: scope, Owner: "related-owner-b", Visibility: "shared", Category: "note", Summary: "b-shared", Tags: []string{tag}, Citations: citation}, []float32{1, 0, 0})
	seedSpineMemoryVector(t, s, Memory{ID: filler1ID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "filler1"}, []float32{0, 1, 0})
	seedSpineMemoryVector(t, s, Memory{ID: filler2ID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "filler2"}, []float32{0, 0, 1})

	res, err := s.RelatedMemories(ctx, anchorID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories: %v", err)
	}
	for _, gatedID := range []string{archivedID, supersededID, expiredID, scheduledID, bPrivateID} {
		for _, r := range res.Related {
			if r.Memory.ID == gatedID {
				t.Fatalf("%s appears in Related, want absent entirely (soft-hidden or unreadable): %+v", gatedID, r)
			}
		}
	}

	bSharedTagEdge, ok := tagEdgeOf(res, bSharedID)
	if !ok {
		t.Fatalf("b-shared has no tag edge: %+v", res.Related)
	}
	bSharedCitationEdge, ok := citationEdgeOf(res, bSharedID)
	if !ok {
		t.Fatalf("b-shared has no citation edge: %+v", res.Related)
	}
	if len(bSharedCitationEdge.SharedCitations) == 0 {
		t.Fatalf("b-shared citation edge has no SharedCitations: %+v", bSharedCitationEdge)
	}
	// a's visible set: anchor, b-shared, filler1, filler2 (n=4); the tag is
	// carried by anchor and b-shared only (df=2, live readable carriers
	// only) — 2*2 <= 4, so it is not ubiquitous.
	assertWeightClose(t, "b-shared.TagWeight", bSharedTagEdge.TagWeight, math.Log(4.0/2.0))

	// Anonymous perspective on an ownerless anchor: the b-shared carrier of
	// the SAME rare tag/citation must be invisible (shared records require
	// an authenticated reader).
	anon := Anonymous()
	idAnon := func(n int) string { return tagRarityID("5b5b", n) }
	anchorAnonID := idAnon(0)
	bSharedAnonID := idAnon(1)
	fillerAnon1ID := idAnon(2)
	fillerAnon2ID := idAnon(3)
	anonTag := "rare-gate-anon"
	anonCitation := []Citation{{Kind: "file", Ref: "gate-anon.go"}}

	seedSpineMemoryVector(t, s, Memory{ID: anchorAnonID, Scope: scope, Owner: "", Category: "note", Summary: "anchor-anon", Tags: []string{anonTag}, Citations: anonCitation}, []float32{0, 0, 1})
	seedSpineMemoryVector(t, s, Memory{ID: bSharedAnonID, Scope: scope, Owner: "related-owner-b", Visibility: "shared", Category: "note", Summary: "b-shared-anon", Tags: []string{anonTag}, Citations: anonCitation}, []float32{0, 0, 1})
	seedSpineMemoryVector(t, s, Memory{ID: fillerAnon1ID, Scope: scope, Owner: "", Category: "note", Summary: "filler-anon1"}, []float32{0, 1, 0})
	seedSpineMemoryVector(t, s, Memory{ID: fillerAnon2ID, Scope: scope, Owner: "", Category: "note", Summary: "filler-anon2"}, []float32{1, 0, 0})

	resAnon, err := s.RelatedMemories(ctx, anchorAnonID, anon, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories(anon): %v", err)
	}
	for _, r := range resAnon.Related {
		if r.Memory.ID == bSharedAnonID {
			t.Fatalf("b-shared-anon appears in Related(anon), want absent (shared record invisible to anonymous): %+v", r)
		}
	}
}

// TestRelatedMemoriesAllEdgesDeterministic pins the idempotency edge across
// ALL four edge types at once: two consecutive calls over unchanged data
// (supersession, citation, tag, and vector edges all present) are deep-equal.
func TestRelatedMemoriesAllEdgesDeterministic(t *testing.T) {
	s := newSpineTestStore(t, "related_all_edges_deterministic")
	ctx := context.Background()
	scope := "related:project:all-edges-deterministic"
	ownerA := Authenticated("related-owner-a")
	id := func(n int) string { return tagRarityID("6a6a", n) }

	anchorID, succID, mID := id(0), id(1), id(2)

	anchor := Memory{
		ID: anchorID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "anchor",
		Tags: []string{"rare-det"}, Citations: []Citation{{Kind: "file", Ref: "det.go"}},
		SupersededBy: qdrant.PtrOf(succID),
	}
	seedSpineMemoryVector(t, s, anchor, []float32{1, 0, 0})

	succ := Memory{
		ID: succID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "succ",
		Supersedes: []string{anchorID},
	}
	seedSpineMemoryVector(t, s, succ, []float32{0, 1, 0})

	m := Memory{
		ID: mID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: "m",
		Tags: []string{"rare-det"}, Citations: []Citation{{Kind: "file", Ref: "det.go"}},
	}
	seedSpineMemoryVector(t, s, m, []float32{0.9, 0.1, 0})

	for i := 0; i < 3; i++ {
		fID := id(3 + i)
		seedSpineMemoryVector(t, s, Memory{ID: fID, Scope: scope, Owner: "related-owner-a", Category: "note", Summary: fmt.Sprintf("filler%d", i)}, []float32{0, 0, 1})
	}

	res1, err := s.RelatedMemories(ctx, anchorID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories (1st call): %v", err)
	}
	res2, err := s.RelatedMemories(ctx, anchorID, ownerA, 0, false)
	if err != nil {
		t.Fatalf("RelatedMemories (2nd call): %v", err)
	}
	if !reflect.DeepEqual(res1, res2) {
		t.Fatalf("two consecutive calls returned different results:\n1st: %+v\n2nd: %+v", res1, res2)
	}

	var sawSupersession, sawCitation, sawTag, sawVector bool
	for _, r := range res1.Related {
		for _, e := range r.Edges {
			switch e.Type {
			case RelatedEdgeSupersession:
				sawSupersession = true
			case RelatedEdgeCitation:
				sawCitation = true
			case RelatedEdgeTag:
				sawTag = true
			case RelatedEdgeVector:
				sawVector = true
			}
		}
	}
	if !sawSupersession || !sawCitation || !sawTag || !sawVector {
		t.Fatalf("fixture did not exercise all four edge types: supersession=%v citation=%v tag=%v vector=%v (%+v)", sawSupersession, sawCitation, sawTag, sawVector, res1.Related)
	}
}
