// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

// relatedEntryDecode decodes one related_memories MCP entry's memory id and
// flat edges — the same shape a real MCP client receives (D-12: the MCP
// side stays flat, never a oneof).
type relatedEntryDecode struct {
	Memory struct {
		ID string `json:"id"`
	} `json:"memory"`
	Edges []store.RelatedEdge `json:"edges"`
}

// decodeRelated re-marshals structured (an MCP related_memories result) and
// decodes it into typed pieces for assertions — the same bytes a real MCP
// client would receive.
func decodeRelated(t *testing.T, structured map[string]any) (anchorID string, entries []relatedEntryDecode, truncated bool) {
	t.Helper()
	raw, err := json.Marshal(structured)
	if err != nil {
		t.Fatalf("json.Marshal(structured related_memories result): %v", err)
	}
	var decoded struct {
		Anchor struct {
			ID string `json:"id"`
		} `json:"anchor"`
		Related   []relatedEntryDecode `json:"related"`
		Truncated bool                 `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(structured related_memories result): %v", err)
	}
	return decoded.Anchor.ID, decoded.Related, decoded.Truncated
}

// assertRelatedIsolationProto checks D-21 on a Connect RelatedMemoriesResponse's
// related list: wID never appears; vID appears with tag and citation edges
// (positive control).
func assertRelatedIsolationProto(t *testing.T, label string, related []*engramv1.RelatedMemory, wID, vID string) {
	t.Helper()
	for _, r := range related {
		if r.GetMemory().GetId() == wID {
			t.Errorf("%s: related list contains W (%s), want absent (private, not owned by caller)", label, wID)
		}
	}
	vRow := findRelatedMemory(related, vID)
	if vRow == nil {
		t.Fatalf("%s: related list missing V (%s) — positive control failed", label, vID)
	}
	if findEdgeByType(vRow.GetEdges(), engramv1.EdgeType_EDGE_TYPE_TAG) == nil {
		t.Errorf("%s: V row missing a TAG edge: %+v", label, vRow.GetEdges())
	}
	if findEdgeByType(vRow.GetEdges(), engramv1.EdgeType_EDGE_TYPE_CITATION) == nil {
		t.Errorf("%s: V row missing a CITATION edge: %+v", label, vRow.GetEdges())
	}
}

// assertRelatedIsolationMCP is assertRelatedIsolationProto's MCP-side twin,
// over decoded related_memories entries.
func assertRelatedIsolationMCP(t *testing.T, label string, entries []relatedEntryDecode, wID, vID string) {
	t.Helper()
	var vRow *relatedEntryDecode
	for i := range entries {
		if entries[i].Memory.ID == wID {
			t.Errorf("%s: related list contains W (%s), want absent (private, not owned by caller)", label, wID)
		}
		if entries[i].Memory.ID == vID {
			vRow = &entries[i]
		}
	}
	if vRow == nil {
		t.Fatalf("%s: related list missing V (%s) — positive control failed", label, vID)
	}
	var hasTag, hasCitation bool
	for _, e := range vRow.Edges {
		if e.Type == store.RelatedEdgeTag {
			hasTag = true
		}
		if e.Type == store.RelatedEdgeCitation {
			hasCitation = true
		}
	}
	if !hasTag {
		t.Errorf("%s: V row missing a tag edge: %+v", label, vRow.Edges)
	}
	if !hasCitation {
		t.Errorf("%s: V row missing a citation edge: %+v", label, vRow.Edges)
	}
}

// TestRelatedMemoriesNeverShowsPrivate proves D-21 on both lanes, compact and
// full: owner B's PRIVATE record never appears in owner A's neighbourhood
// even though it shares A's anchor's tag and citation, while B's SHARED
// record (the positive control) does appear with matching edges. As B,
// RelatedMemories on A's PRIVATE anchor reads not_found, echoing only B's
// own supplied input.
func TestRelatedMemoriesNeverShowsPrivate(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	ownerA := "sub-related-iso-a-" + uuid.NewString()
	ownerB := "sub-related-iso-b-" + uuid.NewString()
	scope := "iso-test:project:related-iso-" + uuid.NewString()
	tag := "iso-" + uuid.NewString()
	citRef := "iso-" + uuid.NewString() + ".go"
	vec := []float32{0.1, 0.2, 0.3}
	now := time.Now().UTC().Truncate(time.Second)

	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll(A) "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(ownerA)))
		cleanupErr(t, "DeleteAll(B) "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(ownerB)))
	})

	// X: owner A's anchor.
	xID := uuid.NewString()
	xShort, err := shortid.New()
	if err != nil {
		t.Fatalf("shortid.New (X): %v", err)
	}
	xMem := store.Memory{
		ID: xID, ShortID: xShort, Content: "X content", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: ownerA,
		Tags:      []string{tag},
		Citations: []store.Citation{{Kind: "file", Ref: citRef}},
		Summary:   "X summary", CreatedAt: now,
	}
	if err := st.Upsert(context.Background(), xMem, vec); err != nil {
		t.Fatalf("seed X: %v", err)
	}

	// Two plain padding records (owned by A, no tag/citation) so the tag
	// edge's rarity weight is provably non-ubiquitous regardless of what
	// else is visible in the shared test collection — see
	// TestRelatedMemoriesConnectRoundTrip's identical reasoning.
	for i, suffix := range []string{"p1", "p2"} {
		pID := uuid.NewString()
		pMem := store.Memory{
			ID: pID, Content: "padding " + suffix, Scope: scope,
			Category: "gotcha", Source: "user-said", Owner: ownerA,
			Summary: "padding summary " + suffix, CreatedAt: now.Add(time.Duration(i+1) * time.Second),
		}
		if err := st.Upsert(context.Background(), pMem, vec); err != nil {
			t.Fatalf("seed padding %s: %v", suffix, err)
		}
	}

	// W: owner B's PRIVATE record, sharing X's tag and citation.
	wID := uuid.NewString()
	wMem := store.Memory{
		ID: wID, Content: "W content", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: ownerB,
		Tags:      []string{tag},
		Citations: []store.Citation{{Kind: "file", Ref: citRef}},
		Summary:   "W summary", CreatedAt: now.Add(3 * time.Second),
	}
	if err := st.Upsert(context.Background(), wMem, vec); err != nil {
		t.Fatalf("seed W: %v", err)
	}

	// V: owner B's SHARED record, sharing X's tag and citation (positive
	// control — must be readable and present).
	vID := uuid.NewString()
	vMem := store.Memory{
		ID: vID, Content: "V content", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: ownerB, Visibility: "shared",
		Tags:      []string{tag},
		Citations: []store.Citation{{Kind: "file", Ref: citRef}},
		Summary:   "V summary", CreatedAt: now.Add(4 * time.Second),
	}
	if err := st.Upsert(context.Background(), vMem, vec); err != nil {
		t.Fatalf("seed V: %v", err)
	}

	// As A, through the Connect handler, compact and full.
	ctxA := parityConnectCtx(ownerA)
	connCompact, err := api.RelatedMemories(ctxA, connect.NewRequest(&engramv1.RelatedMemoriesRequest{Id: xID}))
	if err != nil {
		t.Fatalf("Connect RelatedMemories (compact): %v", err)
	}
	assertRelatedIsolationProto(t, "Connect compact", connCompact.Msg.GetRelated(), wID, vID)

	connFull, err := api.RelatedMemories(ctxA, connect.NewRequest(&engramv1.RelatedMemoriesRequest{Id: xID, Full: true}))
	if err != nil {
		t.Fatalf("Connect RelatedMemories (full): %v", err)
	}
	assertRelatedIsolationProto(t, "Connect full", connFull.Msg.GetRelated(), wID, vID)

	// As A, through the MCP tool, compact and full.
	mcpCtx, cs := newMCPSession(t, d, ownerA)
	mcpCompact, mcpCompactText := callToolTextJSON(mcpCtx, t, cs, "related_memories", map[string]any{"id": xID})
	assertTextIsStructuredJSON(t, "related_memories", mcpCompactText, mcpCompact)
	_, entriesCompact, _ := decodeRelated(t, mcpCompact)
	assertRelatedIsolationMCP(t, "MCP compact", entriesCompact, wID, vID)

	mcpFull, mcpFullText := callToolTextJSON(mcpCtx, t, cs, "related_memories", map[string]any{"id": xID, "full": true})
	assertTextIsStructuredJSON(t, "related_memories", mcpFullText, mcpFull)
	_, entriesFull, _ := decodeRelated(t, mcpFull)
	assertRelatedIsolationMCP(t, "MCP full", entriesFull, wID, vID)

	// As B: RelatedMemories on A's PRIVATE anchor reads not_found, echoing
	// only B's own supplied input — never A's canonical UUID.
	ctxB := parityConnectCtx(ownerB)
	_, connErr := api.RelatedMemories(ctxB, connect.NewRequest(&engramv1.RelatedMemoriesRequest{Id: xShort}))
	if connErr == nil {
		t.Fatal("Connect RelatedMemories (B on A's private X) succeeded, want not_found")
	}
	if connect.CodeOf(connErr) != connect.CodeNotFound {
		t.Errorf("Connect RelatedMemories (B on A's private X) code = %v, want CodeNotFound", connect.CodeOf(connErr))
	}
	if !strings.Contains(connErr.Error(), xShort) {
		t.Errorf("Connect RelatedMemories (B on A's private X) error = %q, want it to name %q", connErr.Error(), xShort)
	}

	mcpCtxB, csB := newMCPSession(t, d, ownerB)
	resB, err := csB.CallTool(mcpCtxB, &mcp.CallToolParams{Name: "related_memories", Arguments: map[string]any{"id": xShort}})
	if err != nil {
		t.Fatalf("CallTool(related_memories) as B: %v", err)
	}
	if !resB.IsError {
		t.Fatal("CallTool(related_memories) as B on A's private X: IsError = false, want true")
	}
	if len(resB.Content) != 1 {
		t.Fatalf("CallTool(related_memories) as B: len(Content) = %d, want 1 (content: %+v)", len(resB.Content), resB.Content)
	}
	tcB, ok := resB.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(related_memories) as B: Content[0] is %T, want *mcp.TextContent", resB.Content[0])
	}
	if !strings.Contains(tcB.Text, xShort) {
		t.Errorf("MCP related_memories (B on A's private X) text = %q, want it to name %q", tcB.Text, xShort)
	}
}

// TestRelatedMemoriesCompactVsFullMCP proves D-13 through the related_memories
// MCP tool: compact carries no "content" key anywhere (anchor or entry) and
// always carries "summary"; full's anchor content equals the stored content;
// every edge, on every entry, is a flat object whose keys are drawn only
// from the documented evidence vocabulary (never a nested oneof wrapper).
func TestRelatedMemoriesCompactVsFullMCP(t *testing.T) {
	d, st := testDepsWithStore(t)

	owner := "sub-related-compactfull-" + uuid.NewString()
	scope := "iso-test:project:related-compactfull-" + uuid.NewString()
	tag := "cf-" + uuid.NewString()
	citRef := "cf-" + uuid.NewString() + ".go"
	vec := []float32{0.1, 0.2, 0.3}
	now := time.Now().UTC().Truncate(time.Second)

	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
	})

	pID := uuid.NewString()
	pShort, err := shortid.New()
	if err != nil {
		t.Fatalf("shortid.New (P): %v", err)
	}
	pMem := store.Memory{
		ID: pID, ShortID: pShort, Content: "P content", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: owner,
		Summary: "P summary", CreatedAt: now,
	}
	if err := st.Upsert(context.Background(), pMem, vec); err != nil {
		t.Fatalf("seed P: %v", err)
	}

	for i, suffix := range []string{"r1", "r2"} {
		rID := uuid.NewString()
		rMem := store.Memory{
			ID: rID, Content: "padding " + suffix, Scope: scope,
			Category: "gotcha", Source: "user-said", Owner: owner,
			Summary: "padding summary " + suffix, CreatedAt: now.Add(time.Duration(i+1) * time.Second),
		}
		if err := st.Upsert(context.Background(), rMem, vec); err != nil {
			t.Fatalf("seed padding %s: %v", suffix, err)
		}
	}

	yID := uuid.NewString()
	yMem := store.Memory{
		ID: yID, Content: "Y content", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: owner,
		Tags:      []string{tag},
		Citations: []store.Citation{{Kind: "file", Ref: citRef}},
		Summary:   "Y summary", CreatedAt: now.Add(3 * time.Second),
	}
	if err := st.Upsert(context.Background(), yMem, vec); err != nil {
		t.Fatalf("seed Y: %v", err)
	}

	ctx := parityConnectCtx(owner)
	c, err := callerFromConnectContext(ctx)
	if err != nil {
		t.Fatalf("callerFromConnectContext: %v", err)
	}
	_, xShort, err := d.supersedeMemory(ctx, c, supersedeArgs{
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

	mcpCtx, cs := newMCPSession(t, d, owner)

	compact, compactText := callToolTextJSON(mcpCtx, t, cs, "related_memories", map[string]any{"id": xShort})
	assertTextIsStructuredJSON(t, "related_memories", compactText, compact)

	anchorMap, ok := compact["anchor"].(map[string]any)
	if !ok {
		t.Fatalf("compact anchor is %T, want map[string]any", compact["anchor"])
	}
	if _, present := anchorMap["content"]; present {
		t.Errorf("compact anchor carries a content key: %v", anchorMap)
	}
	if _, present := anchorMap["summary"]; !present {
		t.Errorf("compact anchor missing summary key: %v", anchorMap)
	}

	compactRelated, ok := compact["related"].([]any)
	if !ok || len(compactRelated) == 0 {
		t.Fatalf("compact related is %T (%v), want a non-empty []any", compact["related"], compact["related"])
	}
	for _, re := range compactRelated {
		entry, ok := re.(map[string]any)
		if !ok {
			t.Fatalf("compact related entry is %T, want map[string]any", re)
		}
		mem, ok := entry["memory"].(map[string]any)
		if !ok {
			t.Fatalf("compact related entry memory is %T, want map[string]any", entry["memory"])
		}
		if _, present := mem["content"]; present {
			t.Errorf("compact related entry carries a content key: %v", mem)
		}
		if _, present := mem["summary"]; !present {
			t.Errorf("compact related entry missing summary key: %v", mem)
		}
	}

	full, fullText := callToolTextJSON(mcpCtx, t, cs, "related_memories", map[string]any{"id": xShort, "full": true})
	assertTextIsStructuredJSON(t, "related_memories", fullText, full)

	fullAnchor, ok := full["anchor"].(map[string]any)
	if !ok {
		t.Fatalf("full anchor is %T, want map[string]any", full["anchor"])
	}
	if got, _ := fullAnchor["content"].(string); got != "X content" {
		t.Errorf("full anchor content = %q, want %q", got, "X content")
	}

	allowedEdgeKeys := map[string]bool{
		"type": true, "score": true, "shared_tags": true, "tag_weight": true,
		"shared_citations": true, "direction": true, "depth": true,
	}
	fullRelated, ok := full["related"].([]any)
	if !ok || len(fullRelated) == 0 {
		t.Fatalf("full related is %T (%v), want a non-empty []any", full["related"], full["related"])
	}
	var sawAnyEdge bool
	for _, re := range fullRelated {
		entry, ok := re.(map[string]any)
		if !ok {
			t.Fatalf("full related entry is %T, want map[string]any", re)
		}
		edges, ok := entry["edges"].([]any)
		if !ok {
			t.Fatalf("full related entry edges is %T, want []any", entry["edges"])
		}
		for _, ee := range edges {
			sawAnyEdge = true
			edge, ok := ee.(map[string]any)
			if !ok {
				t.Fatalf("full related entry edge is %T, want map[string]any", ee)
			}
			for k := range edge {
				if !allowedEdgeKeys[k] {
					t.Errorf("edge carries unexpected key %q: %v", k, edge)
				}
			}
		}
	}
	if !sawAnyEdge {
		t.Fatal("no edges observed in full response — assertion would be vacuous")
	}
}
