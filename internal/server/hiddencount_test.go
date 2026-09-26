// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// TestCountRecallHidden is a pure, table-driven proof of countRecallHidden's
// bucketing and precedence rules (D-02), with no Qdrant involved.
func TestCountRecallHidden(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	someID := "some-superseding-id"

	tests := []struct {
		name string
		m    store.Memory
		g    recallGateFlags
		want recallHidden
	}{
		{
			name: "live record is never counted",
			m:    store.Memory{},
			want: recallHidden{},
		},
		{
			name: "archived only",
			m:    store.Memory{ArchivedAt: &now},
			want: recallHidden{Total: 1, Archived: 1},
		},
		{
			name: "superseded only",
			m:    store.Memory{SupersededBy: &someID},
			want: recallHidden{Total: 1, Superseded: 1},
		},
		{
			name: "not_after == now is expired (exclusive bound)",
			m:    store.Memory{NotAfter: timePtr(now)},
			want: recallHidden{Total: 1, Expired: 1},
		},
		{
			name: "not_after one second after now is not counted",
			m:    store.Memory{NotAfter: timePtr(now.Add(time.Second))},
			want: recallHidden{},
		},
		{
			name: "not_before == now is not counted (inclusive bound, active)",
			m:    store.Memory{NotBefore: timePtr(now)},
			want: recallHidden{},
		},
		{
			name: "not_before one second after now is scheduled",
			m:    store.Memory{NotBefore: timePtr(now.Add(time.Second))},
			want: recallHidden{Total: 1, Scheduled: 1},
		},
		{
			name: "not_after in the past AND not_before in the future is expired, not scheduled",
			m:    store.Memory{NotAfter: timePtr(now.Add(-time.Hour)), NotBefore: timePtr(now.Add(time.Hour))},
			want: recallHidden{Total: 1, Expired: 1},
		},
		{
			name: "archived AND superseded on one record counts once in each, total once",
			m:    store.Memory{ArchivedAt: &now, SupersededBy: &someID},
			want: recallHidden{Total: 1, Archived: 1, Superseded: 1},
		},
		{
			name: "IncludeArchived true: an archived-only record is not counted",
			m:    store.Memory{ArchivedAt: &now},
			g:    recallGateFlags{IncludeArchived: true},
			want: recallHidden{},
		},
		{
			name: "IncludeScheduled true: expired and scheduled records are not counted",
			m:    store.Memory{NotAfter: timePtr(now.Add(-time.Hour)), NotBefore: timePtr(now.Add(time.Hour))},
			g:    recallGateFlags{IncludeScheduled: true},
			want: recallHidden{},
		},
		{
			name: "IncludeSuperseded true: a superseded record is not counted",
			m:    store.Memory{SupersededBy: &someID},
			g:    recallGateFlags{IncludeSuperseded: true},
			want: recallHidden{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := countRecallHidden([]store.Memory{tc.m}, tc.g, now)
			if got != tc.want {
				t.Errorf("countRecallHidden() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time { return &t }

// recallHiddenListFixture seeds the shared TestRecallHiddenListParity /
// TestRecallHiddenSearchParity fixture: owner A gets 1 live, 1 archived-only,
// 1 superseded-only, 1 expired, 1 scheduled, and 1 archived+superseded
// record; owner B gets 1 PRIVATE archived record in the SAME scope. Every
// record carries fixtureTag so a caller that passes Tags: []string{fixtureTag}
// never sees a record any other test leaves in the shared collection.
func seedRecallHiddenFixture(t *testing.T, st *store.Store, ownerA, ownerB, scope, fixtureTag string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	someID := uuid.NewString()

	seed := func(m store.Memory) {
		t.Helper()
		if m.ID == "" {
			m.ID = uuid.NewString()
		}
		m.Scope = scope
		m.Content = "recall-hidden fixture"
		m.Tags = []string{fixtureTag}
		if m.CreatedAt.IsZero() {
			m.CreatedAt = now
		}
		if err := st.Upsert(context.Background(), m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}

	seed(store.Memory{Owner: ownerA})                                                                      // live
	seed(store.Memory{Owner: ownerA, ArchivedAt: timePtr(now.Add(-2 * time.Hour))})                        // archived only
	seed(store.Memory{Owner: ownerA, SupersededBy: &someID})                                               // superseded only
	seed(store.Memory{Owner: ownerA, NotAfter: timePtr(now.Add(-time.Hour))})                              // expired
	seed(store.Memory{Owner: ownerA, NotBefore: timePtr(now.Add(time.Hour))})                              // scheduled
	seed(store.Memory{Owner: ownerA, ArchivedAt: timePtr(now.Add(-3 * time.Hour)), SupersededBy: &someID}) // archived+superseded
	seed(store.Memory{Owner: ownerB, ArchivedAt: timePtr(now.Add(-time.Hour))})                            // owner B, private, archived
}

// numFromAny converts a value read out of an MCP CallTool's StructuredContent
// (JSON round-tripped, so a stored uint64 arrives as float64) into a uint64
// for comparison against the Connect proto's native uint64 fields.
func numFromAny(t *testing.T, v any) uint64 {
	t.Helper()
	switch n := v.(type) {
	case float64:
		return uint64(n)
	case uint64:
		return n
	case int:
		return uint64(n)
	default:
		t.Fatalf("numFromAny: %v is %T, want a number", v, v)
		return 0
	}
}

// hiddenFromStructured extracts the recall_gate_hidden sub-map from an MCP
// StructuredContent map (JSON round-tripped: sub-map arrives as
// map[string]any), or fails the test if it is absent/malformed.
func hiddenFromStructured(t *testing.T, structured map[string]any) map[string]uint64 {
	t.Helper()
	raw, ok := structured["recall_gate_hidden"]
	if !ok {
		t.Fatalf("StructuredContent missing recall_gate_hidden key: %+v", structured)
	}
	h, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("recall_gate_hidden is %T, want map[string]any", raw)
	}
	return map[string]uint64{
		"total":      numFromAny(t, h["total"]),
		"archived":   numFromAny(t, h["archived"]),
		"superseded": numFromAny(t, h["superseded"]),
		"expired":    numFromAny(t, h["expired"]),
		"scheduled":  numFromAny(t, h["scheduled"]),
	}
}

func assertHiddenProto(t *testing.T, got *engramv1.RecallGateHidden, wantTotal, wantArchived, wantSuperseded, wantExpired, wantScheduled uint64) {
	t.Helper()
	if got == nil {
		t.Fatalf("RecallGateHidden is nil, want a populated message")
	}
	if got.GetTotal() != wantTotal || got.GetArchived() != wantArchived || got.GetSuperseded() != wantSuperseded ||
		got.GetExpired() != wantExpired || got.GetScheduled() != wantScheduled {
		t.Errorf("RecallGateHidden = {total:%d archived:%d superseded:%d expired:%d scheduled:%d}, want {total:%d archived:%d superseded:%d expired:%d scheduled:%d}",
			got.GetTotal(), got.GetArchived(), got.GetSuperseded(), got.GetExpired(), got.GetScheduled(),
			wantTotal, wantArchived, wantSuperseded, wantExpired, wantScheduled)
	}
}

func assertHiddenMap(t *testing.T, got map[string]uint64, wantTotal, wantArchived, wantSuperseded, wantExpired, wantScheduled uint64) {
	t.Helper()
	want := map[string]uint64{"total": wantTotal, "archived": wantArchived, "superseded": wantSuperseded, "expired": wantExpired, "scheduled": wantScheduled}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("recall_gate_hidden[%q] = %d, want %d (full: %+v)", k, got[k], w, got)
		}
	}
}

// TestRecallHiddenListParity proves D-01/D-02/D-03 for the list lane against
// a real Qdrant: Connect ListMemories and MCP list_memory report identical
// per-state hidden counts for the same fixture, cross_spine reports the same
// counts again, include_archived=true reclassifies the archived+superseded
// record as superseded-only, another owner's private archived record is
// never counted, and two identical calls are idempotent.
func TestRecallHiddenListParity(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	ownerA := "sub-recallhidden-list-a-" + uuid.NewString()
	ownerB := "sub-recallhidden-list-b-" + uuid.NewString()
	scope := "recallhidden-list:project:" + uuid.NewString()
	fixtureTag := "recallhidden-list-fixture-" + uuid.NewString()

	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll ownerA", st.DeleteAll(context.Background(), scope, store.Authenticated(ownerA)))
		cleanupErr(t, "DeleteAll ownerB", st.DeleteAll(context.Background(), scope, store.Authenticated(ownerB)))
	})

	seedRecallHiddenFixture(t, st, ownerA, ownerB, scope, fixtureTag)

	// Connect ListMemories, scope-confined, limit 0 (= all).
	connCtx := parityConnectCtx(ownerA)
	resp, err := api.ListMemories(connCtx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: scope, Limit: 0, Tags: []string{fixtureTag},
	}))
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if got := len(resp.Msg.GetMemories()); got != 1 {
		t.Fatalf("ListMemories: got %d memories, want 1 (the live record)", got)
	}
	assertHiddenProto(t, resp.Msg.GetRecallGateHidden(), 5, 2, 2, 1, 1)

	// MCP list_memory, same scope, limit 50.
	mcpCtx, cs := newMCPSession(t, d, ownerA)
	mcpRes, err := cs.CallTool(mcpCtx, &mcp.CallToolParams{
		Name:      "list_memory",
		Arguments: map[string]any{"scope": scope, "limit": 50, "tags": []string{fixtureTag}},
	})
	if err != nil || mcpRes.IsError {
		t.Fatalf("CallTool list_memory: err=%v isError=%v", err, mcpRes.IsError)
	}
	structured, ok := mcpRes.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent is %T, want map[string]any", mcpRes.StructuredContent)
	}
	assertHiddenMap(t, hiddenFromStructured(t, structured), 5, 2, 2, 1, 1)

	// cross_spine=true reports the identical counts again (Connect).
	crossResp, err := api.ListMemories(connCtx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		CrossSpine: true, Limit: 50, Tags: []string{fixtureTag},
	}))
	if err != nil {
		t.Fatalf("ListMemories cross_spine: %v", err)
	}
	assertHiddenProto(t, crossResp.Msg.GetRecallGateHidden(), 5, 2, 2, 1, 1)

	// include_archived=true: the archived-only record becomes visible; the
	// archived+superseded record stays hidden as superseded.
	includeResp, err := api.ListMemories(connCtx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: scope, Limit: 0, Tags: []string{fixtureTag}, IncludeArchived: true,
	}))
	if err != nil {
		t.Fatalf("ListMemories include_archived: %v", err)
	}
	assertHiddenProto(t, includeResp.Msg.GetRecallGateHidden(), 4, 0, 2, 1, 1)

	// Idempotency: two identical calls report identical counts.
	againResp, err := api.ListMemories(connCtx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: scope, Limit: 0, Tags: []string{fixtureTag},
	}))
	if err != nil {
		t.Fatalf("ListMemories (repeat): %v", err)
	}
	assertHiddenProto(t, againResp.Msg.GetRecallGateHidden(), 5, 2, 2, 1, 1)
}
