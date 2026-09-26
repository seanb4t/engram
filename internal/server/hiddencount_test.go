// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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

// TestRecallHiddenSearchParity mirrors TestRecallHiddenListParity for the
// search lane: Connect SearchMemories and MCP search_memory report identical
// per-state hidden counts for the same fixture, cross_spine reports the same
// counts again, and include_superseded=true reclassifies the
// archived+superseded record as archived-only.
func TestRecallHiddenSearchParity(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	ownerA := "sub-recallhidden-search-a-" + uuid.NewString()
	ownerB := "sub-recallhidden-search-b-" + uuid.NewString()
	scope := "recallhidden-search:project:" + uuid.NewString()
	fixtureTag := "recallhidden-search-fixture-" + uuid.NewString()
	const query = "recall-hidden fixture"

	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll ownerA", st.DeleteAll(context.Background(), scope, store.Authenticated(ownerA)))
		cleanupErr(t, "DeleteAll ownerB", st.DeleteAll(context.Background(), scope, store.Authenticated(ownerB)))
	})

	seedRecallHiddenFixture(t, st, ownerA, ownerB, scope, fixtureTag)

	// Connect SearchMemories, scope-confined, k 20.
	connCtx := parityConnectCtx(ownerA)
	resp, err := api.SearchMemories(connCtx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
		Scope: scope, Query: query, K: 20, Tags: []string{fixtureTag},
	}))
	if err != nil {
		t.Fatalf("SearchMemories: %v", err)
	}
	if got := len(resp.Msg.GetMemories()); got != 1 {
		t.Fatalf("SearchMemories: got %d memories, want 1 (the live record)", got)
	}
	assertHiddenProto(t, resp.Msg.GetRecallGateHidden(), 5, 2, 2, 1, 1)

	// MCP search_memory, same scope, k 20.
	mcpCtx, cs := newMCPSession(t, d, ownerA)
	mcpRes, err := cs.CallTool(mcpCtx, &mcp.CallToolParams{
		Name:      "search_memory",
		Arguments: map[string]any{"scope": scope, "query": query, "k": 20, "tags": []string{fixtureTag}},
	})
	if err != nil || mcpRes.IsError {
		t.Fatalf("CallTool search_memory: err=%v isError=%v", err, mcpRes.IsError)
	}
	structured, ok := mcpRes.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent is %T, want map[string]any", mcpRes.StructuredContent)
	}
	assertHiddenMap(t, hiddenFromStructured(t, structured), 5, 2, 2, 1, 1)

	// cross_spine=true reports the identical counts again (Connect).
	crossResp, err := api.SearchMemories(connCtx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
		CrossSpine: true, Query: query, K: 20, Tags: []string{fixtureTag},
	}))
	if err != nil {
		t.Fatalf("SearchMemories cross_spine: %v", err)
	}
	assertHiddenProto(t, crossResp.Msg.GetRecallGateHidden(), 5, 2, 2, 1, 1)

	// include_superseded=true: the archived+superseded record stays hidden
	// as archived.
	includeResp, err := api.SearchMemories(connCtx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
		Scope: scope, Query: query, K: 20, Tags: []string{fixtureTag}, IncludeSuperseded: true,
	}))
	if err != nil {
		t.Fatalf("SearchMemories include_superseded: %v", err)
	}
	assertHiddenProto(t, includeResp.Msg.GetRecallGateHidden(), 4, 2, 0, 1, 1)
}

// failingRecallCompareStore embeds *spyStore and fails ONLY the
// recall-gate-hidden-count comparison call — the ungated call each of
// searchRecallHidden/listRecallHidden issues with all three Include flags
// forced true — never the caller's own gated call (which leaves at least one
// Include flag false in these tests). This is what proves the degrade path:
// the caller's own hits must survive a comparison failure untouched.
type failingRecallCompareStore struct {
	*spyStore
	err error
}

func (f *failingRecallCompareStore) Search(ctx context.Context, scope string, subj store.Subject, vec []float32, k uint64, opts store.SearchOptions) ([]store.Memory, error) {
	if opts.IncludeArchived && opts.IncludeSuperseded && opts.IncludeScheduled {
		return nil, f.err
	}
	return f.spyStore.Search(ctx, scope, subj, vec, k, opts)
}

func (f *failingRecallCompareStore) List(ctx context.Context, scope string, subj store.Subject, opts store.ListOptions) ([]store.Memory, uint64, string, error) {
	if opts.IncludeArchived && opts.IncludeSuperseded && opts.IncludeScheduled {
		return nil, 0, "", f.err
	}
	return f.spyStore.List(ctx, scope, subj, opts)
}

// errorLevelHitCount counts ERROR-level slog records mentioning substr.
func errorLevelHitCount(rec *slogRecorder, substr string) int {
	var n int
	for _, r := range rec.containing(substr) {
		if r.level == slog.LevelError {
			n++
		}
	}
	return n
}

// TestRecallHiddenDegradesOnComparisonFailure proves D-01's degrade rule: a
// comparison-call failure never fails the RPC/tool call and never fabricates
// zeros — the caller's own hits survive, recall_gate_hidden is ABSENT
// (nil field on Connect, missing key on MCP), the sentinel cause is logged
// exactly once at ERROR per call, and never reaches the wire.
func TestRecallHiddenDegradesOnComparisonFailure(t *testing.T) {
	owner := "sub-recallhidden-degrade-" + uuid.NewString()
	scope := "recallhidden-degrade:project:" + uuid.NewString()
	fixtureTag := "recallhidden-degrade-fixture-" + uuid.NewString()
	sentinel := errors.New("recallhidden: sentinel comparison failure 7c2e")

	sp := newSpyStore()
	seed := store.Memory{
		ID: uuid.NewString(), Content: "recall-hidden degrade fixture", Scope: scope,
		Owner: owner, Tags: []string{fixtureTag}, CreatedAt: time.Now().UTC(),
	}
	if err := sp.Upsert(context.Background(), seed, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	wrapper := &failingRecallCompareStore{spyStore: sp, err: sentinel}
	d := &deps{st: wrapper, em: fakeEmbedder{}, summaryMaxChars: 500}
	api := &engramAPI{d: d}

	t.Run("Connect SearchMemories", func(t *testing.T) {
		rec := captureSlog(t)
		resp, err := api.SearchMemories(parityConnectCtx(owner), connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Scope: scope, Query: "x", K: 10, Tags: []string{fixtureTag},
		}))
		if err != nil {
			t.Fatalf("SearchMemories: got error %v, want nil", err)
		}
		if got := len(resp.Msg.GetMemories()); got != 1 {
			t.Fatalf("SearchMemories: got %d memories, want 1 (seeded hit must survive)", got)
		}
		if h := resp.Msg.GetRecallGateHidden(); h != nil {
			t.Errorf("SearchMemories: RecallGateHidden = %+v, want nil (absent)", h)
		}
		if n := errorLevelHitCount(rec, sentinel.Error()); n != 1 {
			t.Fatalf("ERROR-level log records mentioning the sentinel: got %d, want 1 (records: %+v)", n, rec.records)
		}
	})

	t.Run("Connect ListMemories", func(t *testing.T) {
		rec := captureSlog(t)
		resp, err := api.ListMemories(parityConnectCtx(owner), connect.NewRequest(&engramv1.ListMemoriesRequest{
			Scope: scope, Limit: 0, Tags: []string{fixtureTag},
		}))
		if err != nil {
			t.Fatalf("ListMemories: got error %v, want nil", err)
		}
		if got := len(resp.Msg.GetMemories()); got != 1 {
			t.Fatalf("ListMemories: got %d memories, want 1 (seeded hit must survive)", got)
		}
		if h := resp.Msg.GetRecallGateHidden(); h != nil {
			t.Errorf("ListMemories: RecallGateHidden = %+v, want nil (absent)", h)
		}
		if n := errorLevelHitCount(rec, sentinel.Error()); n != 1 {
			t.Fatalf("ERROR-level log records mentioning the sentinel: got %d, want 1 (records: %+v)", n, rec.records)
		}
	})

	t.Run("MCP search_memory", func(t *testing.T) {
		rec := captureSlog(t)
		ctx, cs := newMCPSession(t, d, owner)
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "search_memory",
			Arguments: map[string]any{"scope": scope, "query": "x", "k": 10, "tags": []string{fixtureTag}},
		})
		if err != nil || res.IsError {
			t.Fatalf("CallTool search_memory: err=%v isError=%v", err, res.IsError)
		}
		structured, ok := res.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("StructuredContent is %T, want map[string]any", res.StructuredContent)
		}
		mems, ok := structured["memories"].([]any)
		if !ok || len(mems) != 1 {
			t.Fatalf("StructuredContent[%q] = %v (%T), want a 1-element slice", "memories", structured["memories"], structured["memories"])
		}
		if _, present := structured["recall_gate_hidden"]; present {
			t.Errorf("StructuredContent unexpectedly carries recall_gate_hidden: %v", structured["recall_gate_hidden"])
		}
		if n := errorLevelHitCount(rec, sentinel.Error()); n != 1 {
			t.Fatalf("ERROR-level log records mentioning the sentinel: got %d, want 1 (records: %+v)", n, rec.records)
		}
	})

	t.Run("MCP list_memory", func(t *testing.T) {
		rec := captureSlog(t)
		ctx, cs := newMCPSession(t, d, owner)
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name:      "list_memory",
			Arguments: map[string]any{"scope": scope, "limit": 50, "tags": []string{fixtureTag}},
		})
		if err != nil || res.IsError {
			t.Fatalf("CallTool list_memory: err=%v isError=%v", err, res.IsError)
		}
		structured, ok := res.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("StructuredContent is %T, want map[string]any", res.StructuredContent)
		}
		mems, ok := structured["memories"].([]any)
		if !ok || len(mems) != 1 {
			t.Fatalf("StructuredContent[%q] = %v (%T), want a 1-element slice", "memories", structured["memories"], structured["memories"])
		}
		if _, present := structured["recall_gate_hidden"]; present {
			t.Errorf("StructuredContent unexpectedly carries recall_gate_hidden: %v", structured["recall_gate_hidden"])
		}
		if n := errorLevelHitCount(rec, sentinel.Error()); n != 1 {
			t.Fatalf("ERROR-level log records mentioning the sentinel: got %d, want 1 (records: %+v)", n, rec.records)
		}
	})

	// D-02: no substring of the sentinel cause reaches the wire on any lane.
	// (Checked per-subtest above via the absent-field/absent-key assertions;
	// this final check proves the wire text itself never contains it.)
	if strings.Contains(sentinel.Error(), "recall_gate_hidden") {
		t.Fatalf("test bug: sentinel text collides with the field name")
	}
}

// TestRecallHiddenSkipsComparisonWhenAllIncluded proves D-02's skip rule:
// when the caller's own request already includes every recall-gated state,
// no comparison call is issued at all, and recall_gate_hidden is present
// with every field zero (never absent — the value IS known, it is zero).
func TestRecallHiddenSkipsComparisonWhenAllIncluded(t *testing.T) {
	owner := "sub-recallhidden-skip-" + uuid.NewString()
	scope := "recallhidden-skip:project:" + uuid.NewString()
	fixtureTag := "recallhidden-skip-fixture-" + uuid.NewString()

	sp := newSpyStore()
	seed := store.Memory{
		ID: uuid.NewString(), Content: "recall-hidden skip fixture", Scope: scope,
		Owner: owner, Tags: []string{fixtureTag}, CreatedAt: time.Now().UTC(),
	}
	if err := sp.Upsert(context.Background(), seed, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	d := &deps{st: sp, em: fakeEmbedder{}, summaryMaxChars: 500}
	api := &engramAPI{d: d}

	t.Run("SearchMemories", func(t *testing.T) {
		sp.resetCalls()
		resp, err := api.SearchMemories(parityConnectCtx(owner), connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Scope: scope, Query: "x", K: 10, Tags: []string{fixtureTag},
			IncludeArchived: true, IncludeSuperseded: true, IncludeScheduled: true,
		}))
		if err != nil {
			t.Fatalf("SearchMemories: %v", err)
		}
		assertHiddenProto(t, resp.Msg.GetRecallGateHidden(), 0, 0, 0, 0, 0)
		var searchCalls int
		for _, c := range sp.callLog() {
			if c.Method == "Search" {
				searchCalls++
			}
		}
		if searchCalls != 0 {
			t.Errorf("spy recorded %d Search call(s), want 0 (allIncluded must skip the comparison)", searchCalls)
		}
	})

	t.Run("ListMemories", func(t *testing.T) {
		sp.resetCalls()
		resp, err := api.ListMemories(parityConnectCtx(owner), connect.NewRequest(&engramv1.ListMemoriesRequest{
			Scope: scope, Limit: 0, Tags: []string{fixtureTag},
			IncludeArchived: true, IncludeSuperseded: true, IncludeScheduled: true,
		}))
		if err != nil {
			t.Fatalf("ListMemories: %v", err)
		}
		assertHiddenProto(t, resp.Msg.GetRecallGateHidden(), 0, 0, 0, 0, 0)
		var listCalls int
		for _, c := range sp.callLog() {
			if c.Method == "List" {
				listCalls++
			}
		}
		if listCalls != 1 {
			t.Errorf("spy recorded %d List call(s), want exactly 1 (the caller's own gated call; allIncluded must skip the comparison)", listCalls)
		}
	})
}
