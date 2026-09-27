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
