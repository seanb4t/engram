// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/decide/jev"
	"github.com/seanb4t/engram/internal/relevance"
	"github.com/seanb4t/engram/internal/store"
)

// rerankWireQuestion/rerankWireRequest mirror the request shapes
// internal/decide/jev's own (unexported) wireRequest/wireQuestion decode —
// duplicated here deliberately (package server has no access to jev's
// unexported types) so the httptest Decisions handler below can inspect
// exactly what relevance.Hook sent.
type rerankWireQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     struct {
		True  string `json:"true"`
		False string `json:"false"`
	} `json:"criteria"`
}

type rerankWireRequest struct {
	Model     string                        `json:"model"`
	State     map[string]any                `json:"state"`
	Questions map[string]rerankWireQuestion `json:"questions"`
}

type rerankWireAnswer struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"`
}

type rerankWireResponse struct {
	Model   string                      `json:"model"`
	Answers map[string]rerankWireAnswer `json:"answers"`
}

// TestSearchRerankJevTracer proves the Task 1 tracer slice end to end: a
// server-held rank hook (relevance.Hook over a real jev client aimed at an
// httptest Decisions endpoint) flows through deps.searchMemory into
// store.SearchReranked, which sends ONE Decisions request carrying the
// WHOLE candidate pool (never just k), stable-sorts by the provider's
// P(relevant), truncates to k, and stamps Memory.Relevance on the Connect
// wire — with a decision failure or a nil hook both falling back to
// today's lexical order (D-03).
func TestSearchRerankJevTracer(t *testing.T) {
	d := testDeps(t)
	api := &engramAPI{d: d}
	scope := "rerank-jev-tracer:project:test"
	ctx := context.Background()
	now := timeNow()

	// Five records, deliberately built so RerankHits' lexical-overlap step
	// ranks them A(9 overlap) > B(4) > C(2) > D(1) > Target(0) against
	// `query` below — Target shares ZERO terms with the query, so the
	// lexical step ranks it dead last (candidate key c04), yet the
	// httptest provider below scores it highest (0.97), proving the
	// reorder is genuinely Jev's, not an artifact of lexical promotion.
	recA := store.Memory{
		ID:      "e4444444-0000-0000-0000-000000000001",
		Content: "Run task lint before committing; golangci-lint config is set in the .golangci.yaml file.",
		Scope:   scope, Owner: "actor-A", Tags: []string{"lint", "task"}, CreatedAt: now,
	}
	recB := store.Memory{
		ID:      "e4444444-0000-0000-0000-000000000002",
		Content: "Run task fmt before every commit; SECONDARY marker for jev tracer test, config lives in dprint.json.",
		Scope:   scope, Owner: "actor-A", Tags: []string{"fmt", "task"}, CreatedAt: now,
	}
	recC := store.Memory{
		ID:      "e4444444-0000-0000-0000-000000000003",
		Content: "The bare task target runs lint then test; CI invokes it directly.",
		Scope:   scope, Owner: "actor-A", Tags: []string{"task", "ci"}, CreatedAt: now,
	}
	recD := store.Memory{
		ID:      "e4444444-0000-0000-0000-000000000004",
		Content: "Deploy notes: verify the staging environment before promoting the release.",
		Scope:   scope, Owner: "actor-A", Tags: []string{"deploy"}, CreatedAt: now,
	}
	recTarget := store.Memory{
		ID:      "e4444444-0000-0000-0000-000000000005",
		Content: "Rotate the S3 backup credentials every 90 days per the security runbook. TARGET marker for jev tracer test.",
		Scope:   scope, Owner: "actor-A", Tags: []string{"security", "ops"}, CreatedAt: now,
	}
	records := []store.Memory{recA, recB, recC, recD, recTarget}
	for _, m := range records {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	t.Cleanup(func() {
		for _, m := range records {
			cleanupErr(t, "Delete "+m.ID, d.st.Delete(ctx, m.ID, store.Authenticated("actor-A")))
		}
	})

	const query = "Run task lint before committing; golangci-lint config is .golangci.yaml"

	var (
		reqCount atomic.Int32
		fail     atomic.Bool
		mu       sync.Mutex
		lastReq  rerankWireRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		body, _ := io.ReadAll(r.Body)
		var wr rerankWireRequest
		_ = json.Unmarshal(body, &wr)
		mu.Lock()
		lastReq = wr
		mu.Unlock()

		if fail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		answers := make(map[string]rerankWireAnswer, len(wr.Questions))
		for key := range wr.Questions {
			state, _ := wr.State[key].(string)
			switch {
			case strings.Contains(state, "TARGET"):
				answers[key] = rerankWireAnswer{Type: "noul", Noul: 0.97}
			case strings.Contains(state, "SECONDARY"):
				answers[key] = rerankWireAnswer{Type: "noul", Noul: 0.40}
			default:
				answers[key] = rerankWireAnswer{Type: "noul", Noul: 0.02}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rerankWireResponse{
			Model:   "typesafe/jev-1.13-20260917",
			Answers: answers,
		})
	}))
	t.Cleanup(srv.Close)

	newHook := func() {
		d.rankHook = relevance.Hook(jev.New(srv.URL+"/api", "test-key", ""), relevance.DefaultBudget())
	}

	actx := withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-A"}})
	protoIDs := func(ms []*engramv1.Memory) []string {
		ids := make([]string, len(ms))
		for i, m := range ms {
			ids[i] = m.Id
		}
		return ids
	}

	t.Run("jev reorders and stamps relevance", func(t *testing.T) {
		fail.Store(false)
		reqCount.Store(0)
		newHook()

		resp, err := api.SearchMemories(actx, connect.NewRequest(&engramv1.SearchMemoriesRequest{Query: query, Scope: scope, K: 2}))
		if err != nil {
			t.Fatalf("Connect SearchMemories: %v", err)
		}
		if got := len(resp.Msg.Memories); got != 2 {
			t.Fatalf("got %d memories, want 2: %+v", got, resp.Msg.Memories)
		}
		if resp.Msg.Memories[0].Id != recTarget.ID {
			t.Errorf("memories[0].Id = %s, want TARGET record %s (order = %v)", resp.Msg.Memories[0].Id, recTarget.ID, protoIDs(resp.Msg.Memories))
		}
		if resp.Msg.Memories[0].Relevance == nil || resp.Msg.Memories[0].GetRelevance() != 0.97 {
			t.Errorf("memories[0].Relevance = %v, want 0.97", resp.Msg.Memories[0].Relevance)
		}
		if resp.Msg.Memories[1].Id != recB.ID {
			t.Errorf("memories[1].Id = %s, want secondary record %s (order = %v)", resp.Msg.Memories[1].Id, recB.ID, protoIDs(resp.Msg.Memories))
		}
		if resp.Msg.Memories[1].Relevance == nil || resp.Msg.Memories[1].GetRelevance() != 0.40 {
			t.Errorf("memories[1].Relevance = %v, want 0.40", resp.Msg.Memories[1].Relevance)
		}
		for _, m := range resp.Msg.Memories {
			if m.Relevance == nil {
				t.Errorf("memory %s has nil Relevance after a successful rerank", m.Id)
			}
		}

		if got := reqCount.Load(); got != 1 {
			t.Fatalf("Decisions request count = %d, want exactly 1", got)
		}
		mu.Lock()
		gotReq := lastReq
		mu.Unlock()
		if got := len(gotReq.Questions); got != 5 {
			t.Fatalf("request carried %d questions, want 5 (the whole candidate pool, not k=2)", got)
		}
		for _, key := range []string{"c00", "c01", "c02", "c03", "c04"} {
			q, ok := gotReq.Questions[key]
			if !ok {
				t.Errorf("request missing question %s", key)
				continue
			}
			if _, ok := gotReq.State[key]; !ok {
				t.Errorf("request missing state %s", key)
			}
			if q.Type != "noul" {
				t.Errorf("question %s type = %q, want noul", key, q.Type)
			}
			if q.Instructions != relevance.Instructions(key) {
				t.Errorf("question %s instructions = %q, want %q", key, q.Instructions, relevance.Instructions(key))
			}
			if q.Criteria.True != relevance.WhenTrue {
				t.Errorf("question %s criteria.true = %q, want %q", key, q.Criteria.True, relevance.WhenTrue)
			}
			if q.Criteria.False != relevance.WhenFalse {
				t.Errorf("question %s criteria.false = %q, want %q", key, q.Criteria.False, relevance.WhenFalse)
			}
		}
		if gotQuery, _ := gotReq.State["query"].(string); gotQuery != query {
			t.Errorf("request state[query] = %q, want %q", gotQuery, query)
		}
	})

	t.Run("decision failure falls back to lexical order", func(t *testing.T) {
		newHook()
		fail.Store(true)
		t.Cleanup(func() { fail.Store(false) })

		withHook, err := api.SearchMemories(actx, connect.NewRequest(&engramv1.SearchMemoriesRequest{Query: query, Scope: scope, K: 5}))
		if err != nil {
			t.Fatalf("Connect SearchMemories (failing hook): %v", err)
		}

		d.rankHook = nil
		withoutHook, err := api.SearchMemories(actx, connect.NewRequest(&engramv1.SearchMemoriesRequest{Query: query, Scope: scope, K: 5}))
		if err != nil {
			t.Fatalf("Connect SearchMemories (nil hook): %v", err)
		}

		gotIDs := protoIDs(withHook.Msg.Memories)
		wantIDs := protoIDs(withoutHook.Msg.Memories)
		if !slices.Equal(gotIDs, wantIDs) {
			t.Fatalf("fallback order = %v, want the nil-hook lexical order %v", gotIDs, wantIDs)
		}
		for _, m := range withHook.Msg.Memories {
			if m.Relevance != nil {
				t.Errorf("memory %s carries Relevance %v after a decision failure, want nil", m.Id, m.GetRelevance())
			}
		}
	})

	t.Run("nil hook makes no decisions call", func(t *testing.T) {
		d.rankHook = nil
		reqCount.Store(0)

		resp, err := api.SearchMemories(actx, connect.NewRequest(&engramv1.SearchMemoriesRequest{Query: query, Scope: scope, K: 5}))
		if err != nil {
			t.Fatalf("Connect SearchMemories: %v", err)
		}
		if got := reqCount.Load(); got != 0 {
			t.Fatalf("Decisions request count = %d, want 0 with a nil hook", got)
		}
		for _, m := range resp.Msg.Memories {
			if m.Relevance != nil {
				t.Errorf("memory %s carries Relevance with a nil hook", m.Id)
			}
		}
	})
}
