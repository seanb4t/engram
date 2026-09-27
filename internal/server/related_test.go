// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/shortid"
	"github.com/seanb4t/engram/internal/store"
)

// findRelatedMemory returns the *engramv1.RelatedMemory whose memory.id
// matches id, or nil.
func findRelatedMemory(rs []*engramv1.RelatedMemory, id string) *engramv1.RelatedMemory {
	for _, r := range rs {
		if r.GetMemory().GetId() == id {
			return r
		}
	}
	return nil
}

// findEdgeByType returns the first edge in edges whose Type matches et, or
// nil.
func findEdgeByType(edges []*engramv1.RelatedEdge, et engramv1.EdgeType) *engramv1.RelatedEdge {
	for _, e := range edges {
		if e.GetType() == et {
			return e
		}
	}
	return nil
}

// assertEveryEdgeEvidenceMatchesType is D-12's invariant, checked over every
// edge on every related entry in res: type and the oneof evidence case
// always match exactly.
func assertEveryEdgeEvidenceMatchesType(t *testing.T, related []*engramv1.RelatedMemory) {
	t.Helper()
	for _, r := range related {
		for _, e := range r.GetEdges() {
			switch e.GetType() {
			case engramv1.EdgeType_EDGE_TYPE_VECTOR:
				if e.GetVector() == nil {
					t.Errorf("%s: edge type VECTOR carries no VectorEvidence: %+v", r.GetMemory().GetId(), e)
				}
			case engramv1.EdgeType_EDGE_TYPE_TAG:
				if e.GetTag() == nil {
					t.Errorf("%s: edge type TAG carries no TagEvidence: %+v", r.GetMemory().GetId(), e)
				}
			case engramv1.EdgeType_EDGE_TYPE_CITATION:
				if e.GetCitation() == nil {
					t.Errorf("%s: edge type CITATION carries no CitationEvidence: %+v", r.GetMemory().GetId(), e)
				}
			case engramv1.EdgeType_EDGE_TYPE_SUPERSESSION:
				if e.GetSupersession() == nil {
					t.Errorf("%s: edge type SUPERSESSION carries no SupersessionEvidence: %+v", r.GetMemory().GetId(), e)
				}
			default:
				t.Errorf("%s: edge has unexpected type %v", r.GetMemory().GetId(), e.GetType())
			}
		}
	}
}

// TestRelatedMemoriesConnectRoundTrip proves RPC-04's Connect half and D-12/
// D-13 end to end against a real Qdrant: the supersession, citation, and tag
// edges each carry the correctly-matching oneof evidence, compact is the
// default view, full=true switches to full content, repeated calls are
// idempotent, and an unknown id reads CodeNotFound echoing the caller's own
// input (milestone 2026-09-25.01 Phase 3 D-12/D-13).
func TestRelatedMemoriesConnectRoundTrip(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	owner := "sub-related-roundtrip-" + uuid.NewString()
	scope := "iso-test:project:related-roundtrip-" + uuid.NewString()
	ctx := parityConnectCtx(owner)
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		t.Fatalf("callerFromConnectContext: %v", err)
	}

	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
	})

	tag := "rt-" + uuid.NewString()
	citRef := "rt-" + uuid.NewString() + ".go"
	vec := []float32{0.1, 0.2, 0.3}
	now := time.Now().UTC().Truncate(time.Second)

	// P: the record X will supersede. No tag/citation of its own.
	pID := uuid.NewString()
	pShort, err := shortid.New()
	if err != nil {
		t.Fatalf("shortid.New (P): %v", err)
	}
	pMem := store.Memory{
		ID: pID, ShortID: pShort, Content: "P content", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: owner, Visibility: "",
		Summary: "P summary", CreatedAt: now,
	}
	if err := st.Upsert(context.Background(), pMem, vec); err != nil {
		t.Fatalf("seed P: %v", err)
	}

	// Two plain padding records (no tag/citation) so the tag edge's rarity
	// weight is provably non-ubiquitous (relatedTagEdges' `2*d > n` guard)
	// regardless of what else is visible in the shared test collection: this
	// fixture alone guarantees n >= 5 for a tag carried by exactly 2 records
	// (X and Y), well clear of the n >= 4 floor the guard requires.
	for i, suffix := range []string{"r1", "r2"} {
		rID := uuid.NewString()
		rMem := store.Memory{
			ID: rID, Content: "padding " + suffix, Scope: scope,
			Category: "gotcha", Source: "user-said", Owner: owner,
			Summary: "padding summary " + suffix, CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := st.Upsert(context.Background(), rMem, vec); err != nil {
			t.Fatalf("seed padding %s: %v", suffix, err)
		}
	}

	// Y: shares X's tag and citation, otherwise unrelated.
	yID := uuid.NewString()
	yMem := store.Memory{
		ID: yID, Content: "Y content", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: owner,
		Tags:      []string{tag},
		Citations: []store.Citation{{Kind: "file", Ref: citRef}},
		Summary:   "Y summary", CreatedAt: now.Add(2 * time.Second),
	}
	if err := st.Upsert(context.Background(), yMem, vec); err != nil {
		t.Fatalf("seed Y: %v", err)
	}

	// X: created by supersedeMemory, superseding P. Carries the same tag
	// and citation as Y.
	xID, xShort, err := d.supersedeMemory(ctx, c, supersedeArgs{
		storeArgs: storeArgs{
			Content: "X content", Scope: scope, Category: "gotcha", Source: "user-said",
			Tags:      []string{tag},
			Citations: []citationArg{{Kind: "file", Ref: citRef}},
			Summary:   "X summary",
		},
		Supersedes: []string{pID},
	})
	if err != nil {
		t.Fatalf("supersedeMemory (X supersedes P): %v", err)
	}
	if xID == "" || xShort == "" {
		t.Fatalf("supersedeMemory returned empty id/short_id: id=%q short_id=%q", xID, xShort)
	}

	// --- Compact call ---
	compactResp, err := api.RelatedMemories(ctx, connect.NewRequest(&engramv1.RelatedMemoriesRequest{Id: xShort}))
	if err != nil {
		t.Fatalf("RelatedMemories (compact): %v", err)
	}

	anchor := compactResp.Msg.GetAnchor()
	if anchor.GetId() != xID {
		t.Fatalf("anchor id = %q, want %q", anchor.GetId(), xID)
	}
	if anchor.GetContent() != "" {
		t.Errorf("compact anchor content = %q, want empty", anchor.GetContent())
	}
	if anchor.GetSummary() == "" {
		t.Error("compact anchor summary is empty, want set")
	}
	if len(anchor.GetCitations()) != 0 {
		t.Errorf("compact anchor citations = %+v, want none", anchor.GetCitations())
	}

	related := compactResp.Msg.GetRelated()

	pRow := findRelatedMemory(related, pID)
	if pRow == nil {
		t.Fatalf("Related missing P (%s): %+v", pID, related)
	}
	if pRow.GetMemory().GetContent() != "" {
		t.Errorf("P row content = %q, want empty (compact)", pRow.GetMemory().GetContent())
	}
	supEdge := findEdgeByType(pRow.GetEdges(), engramv1.EdgeType_EDGE_TYPE_SUPERSESSION)
	if supEdge == nil {
		t.Fatalf("P row missing a SUPERSESSION edge: %+v", pRow.GetEdges())
	}
	if supEdge.GetSupersession().GetDirection() != engramv1.SupersessionDirection_SUPERSESSION_DIRECTION_PREDECESSOR {
		t.Errorf("P row SUPERSESSION direction = %v, want PREDECESSOR", supEdge.GetSupersession().GetDirection())
	}
	if supEdge.GetSupersession().GetDepth() != 1 {
		t.Errorf("P row SUPERSESSION depth = %d, want 1", supEdge.GetSupersession().GetDepth())
	}

	yRow := findRelatedMemory(related, yID)
	if yRow == nil {
		t.Fatalf("Related missing Y (%s): %+v", yID, related)
	}
	if yRow.GetMemory().GetContent() != "" {
		t.Errorf("Y row content = %q, want empty (compact)", yRow.GetMemory().GetContent())
	}
	citEdge := findEdgeByType(yRow.GetEdges(), engramv1.EdgeType_EDGE_TYPE_CITATION)
	if citEdge == nil {
		t.Fatalf("Y row missing a CITATION edge: %+v", yRow.GetEdges())
	}
	if shared := citEdge.GetCitation().GetSharedCitations(); len(shared) != 1 || shared[0].GetKind() != "file" || shared[0].GetRef() != citRef {
		t.Errorf("Y row CITATION shared_citations = %+v, want exactly [{file %s}]", shared, citRef)
	}
	tagEdge := findEdgeByType(yRow.GetEdges(), engramv1.EdgeType_EDGE_TYPE_TAG)
	if tagEdge == nil {
		t.Fatalf("Y row missing a TAG edge: %+v", yRow.GetEdges())
	}
	foundTag := false
	for _, wt := range tagEdge.GetTag().GetSharedTags() {
		if wt.GetTag() != tag {
			continue
		}
		foundTag = true
		if wt.GetWeight() <= 0 {
			t.Errorf("shared tag %q weight = %v, want positive", tag, wt.GetWeight())
		}
	}
	if !foundTag {
		t.Errorf("Y row TAG shared_tags = %+v, want containing %q", tagEdge.GetTag().GetSharedTags(), tag)
	}

	assertEveryEdgeEvidenceMatchesType(t, related)

	// --- Full call ---
	fullResp, err := api.RelatedMemories(ctx, connect.NewRequest(&engramv1.RelatedMemoriesRequest{Id: xShort, Full: true}))
	if err != nil {
		t.Fatalf("RelatedMemories (full): %v", err)
	}
	if fullResp.Msg.GetAnchor().GetContent() == "" {
		t.Error("full anchor content is empty, want non-empty")
	}
	yRowFull := findRelatedMemory(fullResp.Msg.GetRelated(), yID)
	if yRowFull == nil {
		t.Fatalf("full Related missing Y (%s): %+v", yID, fullResp.Msg.GetRelated())
	}
	if yRowFull.GetMemory().GetContent() == "" {
		t.Error("full Y row content is empty, want non-empty")
	}
	assertEveryEdgeEvidenceMatchesType(t, fullResp.Msg.GetRelated())

	// --- Repeatability ---
	repeatResp, err := api.RelatedMemories(ctx, connect.NewRequest(&engramv1.RelatedMemoriesRequest{Id: xShort}))
	if err != nil {
		t.Fatalf("RelatedMemories (repeat): %v", err)
	}
	if !proto.Equal(compactResp.Msg, repeatResp.Msg) {
		t.Errorf("repeated RelatedMemories call diverged:\nfirst:  %v\nsecond: %v", compactResp.Msg, repeatResp.Msg)
	}

	// --- Unknown short_id ---
	unknown := "rt-unknown-" + uuid.NewString()
	_, err = api.RelatedMemories(ctx, connect.NewRequest(&engramv1.RelatedMemoriesRequest{Id: unknown}))
	if err == nil {
		t.Fatal("RelatedMemories(unknown short_id) succeeded, want CodeNotFound")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("RelatedMemories(unknown short_id) code = %v, want CodeNotFound", connect.CodeOf(err))
	}
	if !strings.Contains(err.Error(), unknown) {
		t.Errorf("RelatedMemories(unknown short_id) error = %q, want it to name %q", err.Error(), unknown)
	}
}
