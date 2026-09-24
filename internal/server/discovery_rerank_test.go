// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// TestSearchDiscoveryDefaultPathUnchanged proves the D-07 CONTEXT boundary:
// with d.rankHook nil (the default), MCP's d.searchDiscovery and Connect's
// SearchDiscoveries both call SearchDiscovery exactly as before this plan —
// their own already-defaulted k (8 MCP, 20 Connect) — and never
// SearchDiscoveryReranked.
func TestSearchDiscoveryDefaultPathUnchanged(t *testing.T) {
	d, sp := newSpyDeps()
	scope := "discovery-default-unchanged:project:test"
	now := time.Now().UTC()
	for i := 0; i < 25; i++ {
		id := shortIDForIndex(i)
		m := store.Memory{
			ID: id, Content: "discovery content", Scope: scope, Category: "discovery",
			Kind: "fact", Owner: "actor-A", CreatedAt: now.Add(time.Duration(i) * time.Second),
			Citations: []store.Citation{{Kind: "file", Ref: "f.go"}},
		}
		if err := sp.Upsert(context.Background(), m, nil); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}

	mcpCtx := authedContext(t, "actor-A")
	mcpCaller := callerFor(mcpCtx, t)
	mcpOut, err := d.searchDiscovery(mcpCtx, mcpCaller, searchDiscoveryArgs{Query: "q", Scope: scope})
	if err != nil {
		t.Fatalf("MCP d.searchDiscovery: %v", err)
	}
	if len(mcpOut) != 8 {
		t.Fatalf("MCP lane returned %d discoveries, want the MCP default k=8", len(mcpOut))
	}

	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-A"}})
	resp, err := api.SearchDiscoveries(actx, connect.NewRequest(&engramv1.SearchDiscoveriesRequest{Query: "q", Scope: scope}))
	if err != nil {
		t.Fatalf("Connect SearchDiscoveries: %v", err)
	}
	if len(resp.Msg.Discoveries) != 20 {
		t.Fatalf("Connect lane returned %d discoveries, want the Connect default k=20", len(resp.Msg.Discoveries))
	}

	calls := sp.callLog()
	var searchDiscoveryCalls, rerankedCalls int
	for _, c := range calls {
		switch c.Method {
		case "SearchDiscovery":
			searchDiscoveryCalls++
		case "SearchDiscoveryReranked":
			rerankedCalls++
		}
	}
	if searchDiscoveryCalls != 2 {
		t.Errorf("SearchDiscovery call count = %d, want 2 (one per lane)", searchDiscoveryCalls)
	}
	if rerankedCalls != 0 {
		t.Errorf("SearchDiscoveryReranked call count = %d, want 0 (default path never reaches it)", rerankedCalls)
	}
}

// shortIDForIndex mints a deterministic, distinct UUID-shaped id for the ith
// seeded fixture record in this file's tests.
func shortIDForIndex(i int) string {
	return fmt.Sprintf("ea000000-0000-0000-%04d-000000000000", i)
}

// TestSearchDiscoveryRelevanceBothLanes proves the hook-gated discovery path
// end to end over a real Qdrant: MCP and Connect agree on order and
// per-hit relevance under a scripted hook, both fall back to the identical
// vector order on a hook failure, and the hook's owner isolation holds on
// both lanes.
func TestSearchDiscoveryRelevanceBothLanes(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}
	ctx := context.Background()
	scope := "discovery-relevance-both-lanes:project:test"
	now := time.Now().UTC()
	cite := []store.Citation{{Kind: "file", Ref: "f.go"}}

	rec1 := store.Memory{ID: "eb000000-0000-0000-0000-000000000001", Content: "discovery one", Scope: scope, Category: "discovery", Kind: "fact", Owner: "actor-A", CreatedAt: now, Citations: cite}
	rec2 := store.Memory{ID: "eb000000-0000-0000-0000-000000000002", Content: "discovery two", Scope: scope, Category: "discovery", Kind: "fact", Owner: "actor-A", CreatedAt: now, Citations: cite}
	rec3 := store.Memory{ID: "eb000000-0000-0000-0000-000000000003", Content: "discovery three", Scope: scope, Category: "discovery", Kind: "fact", Owner: "actor-A", CreatedAt: now, Citations: cite}
	rec4 := store.Memory{ID: "eb000000-0000-0000-0000-000000000004", Content: "discovery four", Scope: scope, Category: "discovery", Kind: "fact", Owner: "actor-A", CreatedAt: now, Citations: cite}
	records := []store.Memory{rec1, rec2, rec3, rec4}
	// testDepsWithStore's fakeEmbedder always embeds any query text to
	// {0.1, 0.2, 0.3} (tools_test.go), so these vectors are chosen for
	// strictly decreasing cosine similarity to THAT fixed vector — rec1 is
	// parallel to it (similarity 1.0), rec4 is anti-parallel on its leading
	// component (negative similarity) — giving a deterministic, known vector
	// order (rec1, rec2, rec3, rec4) independent of the query string's text.
	vecs := [][]float32{
		{0.1, 0.2, 0.3},
		{0.1, 0.2, 0.0},
		{0.1, 0.0, 0.0},
		{-0.1, 0.0, 0.0},
	}
	for i, m := range records {
		if err := st.Upsert(ctx, m, vecs[i]); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	t.Cleanup(func() {
		for _, m := range records {
			cleanupErr(t, "Delete "+m.ID, st.Delete(ctx, m.ID, store.Authenticated("actor-A")))
		}
	})

	const query = "discovery search"

	mcpCtx := authedContext(t, "actor-A")
	mcpCaller := callerFor(mcpCtx, t)
	actx := withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-A"}})

	withDiscoveryRankHook := func(t *testing.T, hook store.RankHook) {
		t.Helper()
		prev := d.rankHook
		d.rankHook = hook
		t.Cleanup(func() { d.rankHook = prev })
	}

	mcpIDs := func(t *testing.T, k uint64) ([]string, []*float64) {
		t.Helper()
		out, err := d.searchDiscovery(mcpCtx, mcpCaller, searchDiscoveryArgs{Query: query, Scope: scope, K: k})
		if err != nil {
			t.Fatalf("MCP d.searchDiscovery: %v", err)
		}
		ids := make([]string, len(out))
		rel := make([]*float64, len(out))
		for i, m := range out {
			ids[i] = m.ID
			rel[i] = m.Relevance
		}
		return ids, rel
	}
	connectIDs := func(t *testing.T, k uint64) ([]string, []*float64) {
		t.Helper()
		resp, err := api.SearchDiscoveries(actx, connect.NewRequest(&engramv1.SearchDiscoveriesRequest{Query: query, Scope: scope, K: k}))
		if err != nil {
			t.Fatalf("Connect SearchDiscoveries: %v", err)
		}
		ids := make([]string, len(resp.Msg.Discoveries))
		rel := make([]*float64, len(resp.Msg.Discoveries))
		for i, m := range resp.Msg.Discoveries {
			ids[i] = m.Id
			if m.Relevance != nil {
				v := m.GetRelevance()
				rel[i] = &v
			}
		}
		return ids, rel
	}

	t.Run("both lanes carry the same relevance", func(t *testing.T) {
		hook := store.RankHook(func(_ context.Context, _ string, hits []store.Memory) (map[string]float64, error) {
			probs := map[string]float64{rec1.ID: 0.1, rec2.ID: 0.2, rec3.ID: 0.3, rec4.ID: 0.99}
			out := make(map[string]float64, len(hits))
			for _, h := range hits {
				out[h.ID] = probs[h.ID]
			}
			return out, nil
		})
		withDiscoveryRankHook(t, hook)

		mIDs, mRel := mcpIDs(t, 3)
		cIDs, cRel := connectIDs(t, 3)

		wantOrder := []string{rec4.ID, rec3.ID, rec2.ID}
		if !slices.Equal(mIDs, wantOrder) {
			t.Fatalf("MCP order = %v, want %v (promoted by relevance)", mIDs, wantOrder)
		}
		if !slices.Equal(cIDs, wantOrder) {
			t.Fatalf("Connect order = %v, want %v (promoted by relevance)", cIDs, wantOrder)
		}
		for i := range mIDs {
			if mRel[i] == nil || cRel[i] == nil {
				t.Fatalf("relevance not set on both lanes at index %d: MCP=%v Connect=%v", i, mRel[i], cRel[i])
			}
			if *mRel[i] != *cRel[i] {
				t.Errorf("relevance mismatch at id %s: MCP=%v Connect=%v", mIDs[i], *mRel[i], *cRel[i])
			}
		}
	})

	t.Run("hook error keeps vector order", func(t *testing.T) {
		errHook := store.RankHook(func(_ context.Context, _ string, _ []store.Memory) (map[string]float64, error) {
			return nil, errors.New("boom: scripted discovery hook failure")
		})
		withDiscoveryRankHook(t, errHook)

		mIDs, mRel := mcpIDs(t, 3)
		cIDs, cRel := connectIDs(t, 3)

		wantOrder := []string{rec1.ID, rec2.ID, rec3.ID}
		if !slices.Equal(mIDs, wantOrder) {
			t.Fatalf("MCP order under hook error = %v, want vector order %v", mIDs, wantOrder)
		}
		if !slices.Equal(cIDs, wantOrder) {
			t.Fatalf("Connect order under hook error = %v, want vector order %v", cIDs, wantOrder)
		}
		for i := range mIDs {
			if mRel[i] != nil {
				t.Errorf("MCP hit %s carries relevance %v under hook error, want nil", mIDs[i], *mRel[i])
			}
			if cRel[i] != nil {
				t.Errorf("Connect hit %s carries relevance %v under hook error, want nil", cIDs[i], *cRel[i])
			}
		}
	})

	t.Run("another owner's private discovery never appears", func(t *testing.T) {
		bPriv := store.Memory{ID: "eb000000-0000-0000-0000-000000000099", Content: "owner B private discovery", Scope: scope, Category: "discovery", Kind: "fact", Owner: "actor-B", CreatedAt: now, Citations: cite}
		if err := st.Upsert(ctx, bPriv, []float32{0.6, 0.4, 0.0}); err != nil {
			t.Fatalf("seed bPriv: %v", err)
		}
		t.Cleanup(func() { cleanupErr(t, "Delete "+bPriv.ID, st.Delete(ctx, bPriv.ID, store.Authenticated("actor-B"))) })

		var seen []string
		hook := store.RankHook(func(_ context.Context, _ string, hits []store.Memory) (map[string]float64, error) {
			out := make(map[string]float64, len(hits))
			for _, h := range hits {
				seen = append(seen, h.ID)
				if h.ID == bPriv.ID {
					out[h.ID] = 1.0
					continue
				}
				out[h.ID] = 0.1
			}
			return out, nil
		})
		withDiscoveryRankHook(t, hook)

		mIDs, _ := mcpIDs(t, 10)
		cIDs, _ := connectIDs(t, 10)

		if slices.Contains(mIDs, bPriv.ID) {
			t.Errorf("MCP returned actor-B's private discovery %s under the jev hook", bPriv.ID)
		}
		if slices.Contains(cIDs, bPriv.ID) {
			t.Errorf("Connect returned actor-B's private discovery %s under the jev hook", bPriv.ID)
		}
		if slices.Contains(seen, bPriv.ID) {
			t.Errorf("hook was handed actor-B's private discovery %s, want it never reaching the hook", bPriv.ID)
		}
	})
}
