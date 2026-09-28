// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves ListTags/list_tags end to end (milestone 2026-09-25.01
// Phase 3, D-03/D-14/D-21): the shared core delegates straight to
// store.Store.ListTags with the caller's own Subject, exact recall-visible
// counts, count-descending then tag-ascending ordering, and the `more`
// truncation signal, identically on both lanes.

package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// assertListTagsMatchesProto decodes an MCP list_tags structured result and
// compares its tags/more against the Connect-side []*engramv1.TagCount and
// more value for the same call.
func assertListTagsMatchesProto(t *testing.T, label string, structured map[string]any, want []*engramv1.TagCount, wantMore bool) {
	t.Helper()
	rawTags, ok := structured["tags"].([]any)
	if !ok {
		t.Fatalf("%s: tags is %T, want []any", label, structured["tags"])
	}
	if len(rawTags) != len(want) {
		t.Fatalf("%s: tags = %+v, want %d entries matching %+v", label, rawTags, len(want), want)
	}
	for i, rt := range rawTags {
		m, ok := rt.(map[string]any)
		if !ok {
			t.Fatalf("%s: tags[%d] is %T, want map[string]any", label, i, rt)
		}
		tag, _ := m["tag"].(string)
		count, _ := m["count"].(float64)
		if tag != want[i].GetTag() || uint64(count) != want[i].GetCount() {
			t.Errorf("%s: tags[%d] = %+v, want {%s %d}", label, i, m, want[i].GetTag(), want[i].GetCount())
		}
	}
	more, _ := structured["more"].(bool)
	if more != wantMore {
		t.Errorf("%s: more = %v, want %v", label, more, wantMore)
	}
}

// TestListTagsRoundTripBothLanes proves RPC-04's ListTags/list_tags core end
// to end against a real Qdrant: exact counts, count-descending then
// tag-ascending order, the more truncation signal, an archived record's tag
// never appearing (recall-gated), repeatability, and identical results on
// both lanes.
func TestListTagsRoundTripBothLanes(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	owner := "sub-listtags-roundtrip-" + uuid.NewString()
	scope := "iso-test:project:listtags-roundtrip-" + uuid.NewString()
	// otherScope is a SECOND scope owned by the same caller (D-27 fixture,
	// see below): this is what makes the scoped assertions below load-
	// bearing rather than vacuous — without it, a mutated core that ignores
	// a.Scope entirely would still pass, because this owner would otherwise
	// have records in only one scope.
	otherScope := "iso-test:project:listtags-roundtrip-other-" + uuid.NewString()
	ctx := parityConnectCtx(owner)

	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll "+scope, st.DeleteAll(context.Background(), scope, store.Authenticated(owner)))
		cleanupErr(t, "DeleteAll "+otherScope, st.DeleteAll(context.Background(), otherScope, store.Authenticated(owner)))
	})

	tagA := "a-" + uuid.NewString()
	tagB := "b-" + uuid.NewString()
	tagC := "c-" + uuid.NewString()
	tagD := "d-" + uuid.NewString()
	vec := []float32{0.1, 0.2, 0.3}
	now := time.Now().UTC().Truncate(time.Second)

	for i := range 3 {
		m := store.Memory{
			ID: uuid.NewString(), Content: "a-tagged", Scope: scope,
			Category: "gotcha", Source: "user-said", Owner: owner,
			Tags: []string{tagA}, Summary: "a summary",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := st.Upsert(context.Background(), m, vec); err != nil {
			t.Fatalf("seed a-tagged[%d]: %v", i, err)
		}
	}

	bMem := store.Memory{
		ID: uuid.NewString(), Content: "b-tagged", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: owner,
		Tags: []string{tagB}, Summary: "b summary", CreatedAt: now.Add(3 * time.Second),
	}
	if err := st.Upsert(context.Background(), bMem, vec); err != nil {
		t.Fatalf("seed b-tagged: %v", err)
	}

	archivedAt := now
	cMem := store.Memory{
		ID: uuid.NewString(), Content: "c-tagged (archived)", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: owner,
		Tags: []string{tagC}, Summary: "c summary", CreatedAt: now.Add(4 * time.Second),
		ArchivedAt: &archivedAt,
	}
	if err := st.Upsert(context.Background(), cMem, vec); err != nil {
		t.Fatalf("seed c-tagged (archived): %v", err)
	}

	// D-27 fixture: a record owned by the SAME caller but in otherScope,
	// carrying its own tag (tagD). The scoped calls below must never see
	// tagD — proving scope narrowing is genuinely enforced, not vacuously
	// true because this owner happens to have records in only one scope.
	dMem := store.Memory{
		ID: uuid.NewString(), Content: "d-tagged (other scope)", Scope: otherScope,
		Category: "gotcha", Source: "user-said", Owner: owner,
		Tags: []string{tagD}, Summary: "d summary", CreatedAt: now.Add(5 * time.Second),
	}
	if err := st.Upsert(context.Background(), dMem, vec); err != nil {
		t.Fatalf("seed d-tagged (other scope): %v", err)
	}

	// --- Connect: exact tags, order, counts, more=false ---
	connResp, err := api.ListTags(ctx, connect.NewRequest(&engramv1.ListTagsRequest{Scope: scope}))
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	tags := connResp.Msg.GetTags()
	if len(tags) != 2 {
		t.Fatalf("ListTags tags = %+v, want exactly 2", tags)
	}
	if tags[0].GetTag() != tagA || tags[0].GetCount() != 3 {
		t.Errorf("ListTags tags[0] = %+v, want {%s 3}", tags[0], tagA)
	}
	if tags[1].GetTag() != tagB || tags[1].GetCount() != 1 {
		t.Errorf("ListTags tags[1] = %+v, want {%s 1}", tags[1], tagB)
	}
	if connResp.Msg.GetMore() {
		t.Error("ListTags more = true, want false")
	}
	for _, tc := range tags {
		if tc.GetTag() == tagC {
			t.Errorf("ListTags tags contains archived-only tag %q, want absent", tagC)
		}
		if tc.GetTag() == tagD {
			t.Errorf("ListTags(scope=%s) tags contains other-scope tag %q, want absent (scope narrowing)", scope, tagD)
		}
	}

	// --- Connect: limit 1 -> more true ---
	limitedResp, err := api.ListTags(ctx, connect.NewRequest(&engramv1.ListTagsRequest{Scope: scope, Limit: 1}))
	if err != nil {
		t.Fatalf("ListTags(limit=1): %v", err)
	}
	limitedTags := limitedResp.Msg.GetTags()
	if len(limitedTags) != 1 || limitedTags[0].GetTag() != tagA || limitedTags[0].GetCount() != 3 {
		t.Errorf("ListTags(limit=1) tags = %+v, want exactly [{%s 3}]", limitedTags, tagA)
	}
	if !limitedResp.Msg.GetMore() {
		t.Error("ListTags(limit=1) more = false, want true")
	}

	// --- MCP: same tags/counts/more for both calls ---
	mcpCtx, cs := newMCPSession(t, d, owner)

	mcpResult, mcpText := callToolTextJSON(mcpCtx, t, cs, "list_tags", map[string]any{"scope": scope})
	assertTextIsStructuredJSON(t, "list_tags", mcpText, mcpResult)
	assertListTagsMatchesProto(t, "MCP", mcpResult, tags, false)

	mcpLimited, mcpLimitedText := callToolTextJSON(mcpCtx, t, cs, "list_tags", map[string]any{"scope": scope, "limit": 1})
	assertTextIsStructuredJSON(t, "list_tags", mcpLimitedText, mcpLimited)
	assertListTagsMatchesProto(t, "MCP limit=1", mcpLimited, limitedTags, true)

	// --- Repeatability: two identical Connect calls ---
	repeatResp, err := api.ListTags(ctx, connect.NewRequest(&engramv1.ListTagsRequest{Scope: scope}))
	if err != nil {
		t.Fatalf("ListTags (repeat): %v", err)
	}
	if !proto.Equal(connResp.Msg, repeatResp.Msg) {
		t.Errorf("repeated ListTags call diverged:\nfirst:  %v\nsecond: %v", connResp.Msg, repeatResp.Msg)
	}
}

// protoTagCountsToViews maps Connect's []*engramv1.TagCount onto
// tagCountView — the same flat shape the MCP JSON decode below produces —
// so both lanes' results can be compared and asserted on with one set of
// helpers.
func protoTagCountsToViews(ts []*engramv1.TagCount) []tagCountView {
	out := make([]tagCountView, len(ts))
	for i, t := range ts {
		out[i] = tagCountView{Tag: t.GetTag(), Count: t.GetCount()}
	}
	return out
}

// decodeListTagsViews decodes a list_tags MCP structured result's "tags"
// key into []tagCountView — the same bytes a real MCP client receives.
func decodeListTagsViews(t *testing.T, structured map[string]any) []tagCountView {
	t.Helper()
	raw, err := json.Marshal(structured["tags"])
	if err != nil {
		t.Fatalf("json.Marshal(list_tags tags): %v", err)
	}
	var out []tagCountView
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal(list_tags tags): %v", err)
	}
	return out
}

// findTagCount returns a pointer to the tagCountView for tag in vs, or nil.
func findTagCount(vs []tagCountView, tag string) *tagCountView {
	for i := range vs {
		if vs[i].Tag == tag {
			return &vs[i]
		}
	}
	return nil
}

// assertTagCountAbsent fails if tag appears anywhere in vs.
func assertTagCountAbsent(t *testing.T, label string, vs []tagCountView, tag string) {
	t.Helper()
	if v := findTagCount(vs, tag); v != nil {
		t.Errorf("%s: tags contains %q, want absent: %+v", label, tag, vs)
	}
}

// assertTagCountContains fails unless tag appears in vs (any count).
func assertTagCountContains(t *testing.T, label string, vs []tagCountView, tag string) {
	t.Helper()
	if findTagCount(vs, tag) == nil {
		t.Fatalf("%s: tags missing %q: %+v", label, tag, vs)
	}
}

// assertTagCountExact fails unless tag appears in vs with exactly count.
func assertTagCountExact(t *testing.T, label string, vs []tagCountView, tag string, count uint64) {
	t.Helper()
	v := findTagCount(vs, tag)
	if v == nil {
		t.Fatalf("%s: tags missing %q: %+v", label, tag, vs)
	}
	if v.Count != count {
		t.Errorf("%s: tag %q count = %d, want %d", label, tag, v.Count, count)
	}
}

// TestListTagsNeverShowsPrivate proves D-21 on both lanes, scoped and
// all-scopes: owner B's PRIVATE record's tag never appears in owner A's
// ListTags/list_tags results even though it shares A's tag name with a
// SHARED record (positive control), and A's second-scope tag never leaks
// into a scoped call for the first scope. As B, ListTags(S) contains the
// private tag — visible to its own owner.
func TestListTagsNeverShowsPrivate(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	ownerA := "sub-listtags-iso-a-" + uuid.NewString()
	ownerB := "sub-listtags-iso-b-" + uuid.NewString()
	scopeS := "iso-test:project:listtags-iso-" + uuid.NewString()
	scopeS2 := "iso-test:project:listtags-iso-s2-" + uuid.NewString()
	tagShared := "s-" + uuid.NewString()
	tagPrivate := "p-" + uuid.NewString()
	tagX := "x-" + uuid.NewString()
	vec := []float32{0.1, 0.2, 0.3}
	now := time.Now().UTC().Truncate(time.Second)

	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll(A) "+scopeS, st.DeleteAll(context.Background(), scopeS, store.Authenticated(ownerA)))
		cleanupErr(t, "DeleteAll(A) "+scopeS2, st.DeleteAll(context.Background(), scopeS2, store.Authenticated(ownerA)))
		cleanupErr(t, "DeleteAll(B) "+scopeS, st.DeleteAll(context.Background(), scopeS, store.Authenticated(ownerB)))
	})

	// B: PRIVATE record in S, tagged p-<u>.
	bPrivate := store.Memory{
		ID: uuid.NewString(), Content: "B private", Scope: scopeS,
		Category: "gotcha", Source: "user-said", Owner: ownerB,
		Tags: []string{tagPrivate}, Summary: "B private summary", CreatedAt: now,
	}
	if err := st.Upsert(context.Background(), bPrivate, vec); err != nil {
		t.Fatalf("seed B private: %v", err)
	}

	// B: SHARED record in S, tagged s-<u> (positive control).
	bShared := store.Memory{
		ID: uuid.NewString(), Content: "B shared", Scope: scopeS,
		Category: "gotcha", Source: "user-said", Owner: ownerB, Visibility: "shared",
		Tags: []string{tagShared}, Summary: "B shared summary", CreatedAt: now.Add(time.Second),
	}
	if err := st.Upsert(context.Background(), bShared, vec); err != nil {
		t.Fatalf("seed B shared: %v", err)
	}

	// A: record in S, tagged s-<u> too (same tag as B's shared record ->
	// count 2, the positive control's exact count).
	aInS := store.Memory{
		ID: uuid.NewString(), Content: "A in S", Scope: scopeS,
		Category: "gotcha", Source: "user-said", Owner: ownerA,
		Tags: []string{tagShared}, Summary: "A in S summary", CreatedAt: now.Add(2 * time.Second),
	}
	if err := st.Upsert(context.Background(), aInS, vec); err != nil {
		t.Fatalf("seed A in S: %v", err)
	}

	// A: record in S2 (a SECOND scope owned by A), tagged x-<u>.
	aInS2 := store.Memory{
		ID: uuid.NewString(), Content: "A in S2", Scope: scopeS2,
		Category: "gotcha", Source: "user-said", Owner: ownerA,
		Tags: []string{tagX}, Summary: "A in S2 summary", CreatedAt: now.Add(3 * time.Second),
	}
	if err := st.Upsert(context.Background(), aInS2, vec); err != nil {
		t.Fatalf("seed A in S2: %v", err)
	}

	// As A, Connect ListTags(S): contains {s-<u>, 2}, never p-<u>, never x-<u>.
	ctxA := parityConnectCtx(ownerA)
	connS, err := api.ListTags(ctxA, connect.NewRequest(&engramv1.ListTagsRequest{Scope: scopeS}))
	if err != nil {
		t.Fatalf("Connect ListTags(A, S): %v", err)
	}
	connSViews := protoTagCountsToViews(connS.Msg.GetTags())
	assertTagCountExact(t, "Connect A/S", connSViews, tagShared, 2)
	assertTagCountAbsent(t, "Connect A/S", connSViews, tagPrivate)
	assertTagCountAbsent(t, "Connect A/S", connSViews, tagX)

	// As A, Connect ListTags(""): contains s-<u> and x-<u>, never p-<u>.
	connAll, err := api.ListTags(ctxA, connect.NewRequest(&engramv1.ListTagsRequest{}))
	if err != nil {
		t.Fatalf("Connect ListTags(A, all scopes): %v", err)
	}
	connAllViews := protoTagCountsToViews(connAll.Msg.GetTags())
	assertTagCountContains(t, "Connect A/all", connAllViews, tagShared)
	assertTagCountContains(t, "Connect A/all", connAllViews, tagX)
	assertTagCountAbsent(t, "Connect A/all", connAllViews, tagPrivate)

	// As A, MCP list_tags(S) and list_tags(""), same assertions.
	mcpCtxA, csA := newMCPSession(t, d, ownerA)

	mcpS, mcpSText := callToolTextJSON(mcpCtxA, t, csA, "list_tags", map[string]any{"scope": scopeS})
	assertTextIsStructuredJSON(t, "list_tags", mcpSText, mcpS)
	mcpSViews := decodeListTagsViews(t, mcpS)
	assertTagCountExact(t, "MCP A/S", mcpSViews, tagShared, 2)
	assertTagCountAbsent(t, "MCP A/S", mcpSViews, tagPrivate)
	assertTagCountAbsent(t, "MCP A/S", mcpSViews, tagX)

	mcpAll, mcpAllText := callToolTextJSON(mcpCtxA, t, csA, "list_tags", map[string]any{})
	assertTextIsStructuredJSON(t, "list_tags", mcpAllText, mcpAll)
	mcpAllViews := decodeListTagsViews(t, mcpAll)
	assertTagCountContains(t, "MCP A/all", mcpAllViews, tagShared)
	assertTagCountContains(t, "MCP A/all", mcpAllViews, tagX)
	assertTagCountAbsent(t, "MCP A/all", mcpAllViews, tagPrivate)

	// As B, ListTags(S) contains the private tag — visible to its own
	// owner — on both lanes.
	ctxB := parityConnectCtx(ownerB)
	connB, err := api.ListTags(ctxB, connect.NewRequest(&engramv1.ListTagsRequest{Scope: scopeS}))
	if err != nil {
		t.Fatalf("Connect ListTags(B, S): %v", err)
	}
	assertTagCountContains(t, "Connect B/S", protoTagCountsToViews(connB.Msg.GetTags()), tagPrivate)

	mcpCtxB, csB := newMCPSession(t, d, ownerB)
	mcpB, mcpBText := callToolTextJSON(mcpCtxB, t, csB, "list_tags", map[string]any{"scope": scopeS})
	assertTextIsStructuredJSON(t, "list_tags", mcpBText, mcpB)
	assertTagCountContains(t, "MCP B/S", decodeListTagsViews(t, mcpB), tagPrivate)
}
