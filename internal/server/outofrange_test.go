// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves D-10 (plan 04-06): every recall count knob named in
// 04-CONTEXT.md D-02 refuses a count above store.MaxRecallLimit, by name,
// through its REAL entry point — never the shared core directly, since the
// point is to prove the wire boundary, not just rejectOverMaximumCount
// itself — and that the refusal happens before any embed call or Qdrant RPC.
// A surface added later without the check fails
// TestOutOfRangeRejectedOnEveryRecallSurface's table, which is the point.
package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// newOutOfRangeMCPSession registers a real MCP server around d and returns a
// connected client session, mirroring TestMCPListMemoryResponseTooLarge's own
// in-memory-transport setup (responsetoolarge_test.go).
func newOutOfRangeMCPSession(t *testing.T, d *deps, owner string) (*mcp.ClientSession, context.Context) {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "engram-test", Version: "test"}, nil)
	if err := registerTools(s, d); err != nil {
		t.Fatalf("registerTools: %v", err)
	}
	tctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := s.Connect(authedContext(t, owner), serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	mc := mcp.NewClient(&mcp.Implementation{Name: "engram-test-client", Version: "test"}, nil)
	cs, err := mc.Connect(tctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs, tctx
}

// TestOutOfRangeRejectedOnEveryRecallSurface is D-10's surface-complete
// invariant: ONE table over all seven recall count knobs (the Connect list
// RPC's limit, the Connect memory-search and discovery-search RPCs' k, and
// the four MCP tools' limit or k), each driven through its real entry point,
// with a case at store.MaxRecallLimit (must succeed) and a case one above it
// (must be refused, naming the field and the out_of_range hint — and, on the
// Connect lane, mapping to CodeInvalidArgument). A surface added later
// without the check fails this table.
func TestOutOfRangeRejectedOnEveryRecallSurface(t *testing.T) {
	d := testDeps(t)
	owner := "oor-owner-" + uuid.NewString()
	scope := "iso-test:project:oor-" + uuid.NewString()
	discoveryScope := "discovery:repo:oor-" + uuid.NewString()

	cs, tctx := newOutOfRangeMCPSession(t, d, owner)
	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": owner}})

	const atMax = uint64(store.MaxRecallLimit)
	const overMax = uint64(store.MaxRecallLimit + 1)

	assertMCPSuccess := func(t *testing.T, tool string, args map[string]any) {
		t.Helper()
		res, err := cs.CallTool(tctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool(%s) at the maximum: %v", tool, err)
		}
		if res.IsError {
			t.Fatalf("CallTool(%s) at the maximum: IsError = true, want false (content: %+v)", tool, res.Content)
		}
	}
	assertMCPRejected := func(t *testing.T, tool, field string, args map[string]any) {
		t.Helper()
		res, err := cs.CallTool(tctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool(%s) one above the maximum: %v", tool, err)
		}
		if !res.IsError {
			t.Fatalf("CallTool(%s) one above the maximum: IsError = false, want true", tool)
		}
		if len(res.Content) != 1 {
			t.Fatalf("CallTool(%s): len(Content) = %d, want 1 (content: %+v)", tool, len(res.Content), res.Content)
		}
		tc, ok := res.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatalf("CallTool(%s): Content[0] is %T, want *mcp.TextContent", tool, res.Content[0])
		}
		want := "field=" + field + " hint=out_of_range"
		if !strings.HasPrefix(tc.Text, want) {
			t.Errorf("CallTool(%s): Text = %q, want prefix %q", tool, tc.Text, want)
		}
	}
	assertConnectSuccess := func(t *testing.T, name string, call func() error) {
		t.Helper()
		if err := call(); err != nil {
			t.Fatalf("%s at the maximum: %v", name, err)
		}
	}
	assertConnectRejected := func(t *testing.T, name, field string, call func() error) {
		t.Helper()
		err := call()
		if err == nil {
			t.Fatalf("%s one above the maximum: got nil error, want a rejection", name)
		}
		if code := connect.CodeOf(err); code != connect.CodeInvalidArgument {
			t.Errorf("%s one above the maximum: CodeOf(err) = %v, want CodeInvalidArgument", name, code)
		}
		want := "field=" + field + " hint=out_of_range"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s one above the maximum: err.Error() = %q, want substring %q", name, err.Error(), want)
		}
	}

	t.Run("mcp_list_memory", func(t *testing.T) {
		t.Run("at_maximum", func(t *testing.T) {
			assertMCPSuccess(t, "list_memory", map[string]any{"scope": scope, "limit": atMax})
		})
		t.Run("one_above_maximum", func(t *testing.T) {
			assertMCPRejected(t, "list_memory", "limit", map[string]any{"scope": scope, "limit": overMax})
		})
	})
	t.Run("mcp_list_scheduled", func(t *testing.T) {
		t.Run("at_maximum", func(t *testing.T) {
			assertMCPSuccess(t, "list_scheduled", map[string]any{"scope": scope, "limit": atMax})
		})
		t.Run("one_above_maximum", func(t *testing.T) {
			assertMCPRejected(t, "list_scheduled", "limit", map[string]any{"scope": scope, "limit": overMax})
		})
	})
	t.Run("mcp_search_memory", func(t *testing.T) {
		t.Run("at_maximum", func(t *testing.T) {
			assertMCPSuccess(t, "search_memory", map[string]any{"scope": scope, "query": "probe", "k": atMax})
		})
		t.Run("one_above_maximum", func(t *testing.T) {
			assertMCPRejected(t, "search_memory", "k", map[string]any{"scope": scope, "query": "probe", "k": overMax})
		})
	})
	t.Run("mcp_search_discovery", func(t *testing.T) {
		t.Run("at_maximum", func(t *testing.T) {
			assertMCPSuccess(t, "search_discovery", map[string]any{"scope": discoveryScope, "query": "probe", "k": atMax})
		})
		t.Run("one_above_maximum", func(t *testing.T) {
			assertMCPRejected(t, "search_discovery", "k", map[string]any{"scope": discoveryScope, "query": "probe", "k": overMax})
		})
	})
	t.Run("connect_list_memories", func(t *testing.T) {
		t.Run("at_maximum", func(t *testing.T) {
			assertConnectSuccess(t, "ListMemories", func() error {
				_, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: atMax}))
				return err
			})
		})
		t.Run("one_above_maximum", func(t *testing.T) {
			assertConnectRejected(t, "ListMemories", "limit", func() error {
				_, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: overMax}))
				return err
			})
		})
	})
	t.Run("connect_search_memories", func(t *testing.T) {
		t.Run("at_maximum", func(t *testing.T) {
			assertConnectSuccess(t, "SearchMemories", func() error {
				_, err := api.SearchMemories(actx, connect.NewRequest(&engramv1.SearchMemoriesRequest{Scope: scope, Query: "probe", K: atMax}))
				return err
			})
		})
		t.Run("one_above_maximum", func(t *testing.T) {
			assertConnectRejected(t, "SearchMemories", "k", func() error {
				_, err := api.SearchMemories(actx, connect.NewRequest(&engramv1.SearchMemoriesRequest{Scope: scope, Query: "probe", K: overMax}))
				return err
			})
		})
	})
	t.Run("connect_search_discoveries", func(t *testing.T) {
		t.Run("at_maximum", func(t *testing.T) {
			assertConnectSuccess(t, "SearchDiscoveries", func() error {
				_, err := api.SearchDiscoveries(actx, connect.NewRequest(&engramv1.SearchDiscoveriesRequest{Scope: discoveryScope, Query: "probe", K: atMax}))
				return err
			})
		})
		t.Run("one_above_maximum", func(t *testing.T) {
			assertConnectRejected(t, "SearchDiscoveries", "k", func() error {
				_, err := api.SearchDiscoveries(actx, connect.NewRequest(&engramv1.SearchDiscoveriesRequest{Scope: discoveryScope, Query: "probe", K: overMax}))
				return err
			})
		})
	})
}

// grpcCallCounter is a mutex-free (atomic) recording grpc.UnaryClientInterceptor
// counting EVERY unary RPC regardless of method — TestOutOfRangeRejectedBeforeAnyBackend
// needs to prove ZERO calls of any kind, not just Scroll, reach Qdrant.
type grpcCallCounter struct {
	n int32
}

func (c *grpcCallCounter) intercept(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	atomic.AddInt32(&c.n, 1)
	return invoker(ctx, method, req, reply, cc, opts...)
}

func (c *grpcCallCounter) reset()       { atomic.StoreInt32(&c.n, 0) }
func (c *grpcCallCounter) count() int32 { return atomic.LoadInt32(&c.n) }

// failEmbedder is an embedder stub whose methods fail the test if ever
// called (via t.Errorf, safe from any goroutine — unlike t.Fatal/FailNow,
// which the MCP in-memory transport's own server goroutine would violate).
// Used by TestOutOfRangeRejectedBeforeAnyBackend to prove a rejected
// over-maximum request never reaches the embedder.
type failEmbedder struct {
	t     *testing.T
	calls int32
}

func (f *failEmbedder) Embed(context.Context, string) ([]float32, error) {
	atomic.AddInt32(&f.calls, 1)
	f.t.Errorf("Embed must not be called before an over-maximum count is rejected")
	return nil, errors.New("failEmbedder: Embed must not be called")
}

func (f *failEmbedder) EmbedQuery(context.Context, string) ([]float32, error) {
	atomic.AddInt32(&f.calls, 1)
	f.t.Errorf("EmbedQuery must not be called before an over-maximum count is rejected")
	return nil, errors.New("failEmbedder: EmbedQuery must not be called")
}

func (f *failEmbedder) callCount() int32 { return atomic.LoadInt32(&f.calls) }

// TestOutOfRangeRejectedBeforeAnyBackend is T-04-06-01's DoS-cost proof: each
// search surface driven one above store.MaxRecallLimit, against a store
// dialed with a recorder counting EVERY gRPC call and an embedder that fails
// the test if called, records zero embed calls and zero gRPC calls — the
// rejection costs nothing downstream.
func TestOutOfRangeRejectedBeforeAnyBackend(t *testing.T) {
	rpcCount := &grpcCallCounter{}
	c := storetest.Dial(t, storetest.RecvLimit, grpc.WithChainUnaryInterceptor(rpcCount.intercept))
	st := newTestStore(t, c, testCollection("oor_backend_"+uuid.NewString()))
	if err := st.EnsureCollection(context.Background(), 3); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	// EnsureCollection itself issues setup RPCs; reset the baseline so each
	// subtest below measures only its own call, not collection setup.
	rpcCount.reset()

	embed := &failEmbedder{t: t}
	d := &deps{st: st, em: embed}
	owner := "oor-backend-owner-" + uuid.NewString()
	scope := "iso-test:project:oor-backend-" + uuid.NewString()
	discoveryScope := "discovery:repo:oor-backend-" + uuid.NewString()
	const overMax = uint64(store.MaxRecallLimit + 1)

	cs, tctx := newOutOfRangeMCPSession(t, d, owner)
	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": owner}})

	check := func(t *testing.T, name string) {
		t.Helper()
		if got := embed.callCount(); got != 0 {
			t.Errorf("%s: embed calls = %d, want 0", name, got)
		}
		if got := rpcCount.count(); got != 0 {
			t.Errorf("%s: recorded gRPC calls = %d, want 0", name, got)
		}
	}

	t.Run("mcp_search_memory", func(t *testing.T) {
		rpcCount.reset()
		res, err := cs.CallTool(tctx, &mcp.CallToolParams{Name: "search_memory", Arguments: map[string]any{"scope": scope, "query": "probe", "k": overMax}})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if !res.IsError {
			t.Fatalf("CallTool: IsError = false, want true")
		}
		check(t, "search_memory")
	})
	t.Run("mcp_search_discovery", func(t *testing.T) {
		rpcCount.reset()
		res, err := cs.CallTool(tctx, &mcp.CallToolParams{Name: "search_discovery", Arguments: map[string]any{"scope": discoveryScope, "query": "probe", "k": overMax}})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if !res.IsError {
			t.Fatalf("CallTool: IsError = false, want true")
		}
		check(t, "search_discovery")
	})
	t.Run("connect_search_memories", func(t *testing.T) {
		rpcCount.reset()
		_, err := api.SearchMemories(actx, connect.NewRequest(&engramv1.SearchMemoriesRequest{Scope: scope, Query: "probe", K: overMax}))
		if err == nil {
			t.Fatalf("SearchMemories: got nil error, want a rejection")
		}
		check(t, "SearchMemories")
	})
	t.Run("connect_search_discoveries", func(t *testing.T) {
		rpcCount.reset()
		_, err := api.SearchDiscoveries(actx, connect.NewRequest(&engramv1.SearchDiscoveriesRequest{Scope: discoveryScope, Query: "probe", K: overMax}))
		if err == nil {
			t.Fatalf("SearchDiscoveries: got nil error, want a rejection")
		}
		check(t, "SearchDiscoveries")
	})
}

// mcpResultCount extracts the length of the named array field (e.g.
// "memories", "discoveries") from a successful CallTool's StructuredContent.
func mcpResultCount(ctx context.Context, t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any, key string) int {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", tool, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s): IsError = true, want false (content: %+v)", tool, res.Content)
	}
	m, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("CallTool(%s): StructuredContent is %T, want map[string]any", tool, res.StructuredContent)
	}
	arr, ok := m[key].([]any)
	if !ok {
		t.Fatalf("CallTool(%s): StructuredContent[%q] is %T, want []any", tool, key, m[key])
	}
	return len(arr)
}

// TestZeroCountKeepsSurfaceDefaults pins D-08: a caller-supplied zero count
// is NEVER treated as an over-maximum rejection and yields each surface's
// own documented default — unchanged by this plan's D-10 rejection — proven
// against a store seeded with more records than any of those defaults, so a
// changed default is visible as a changed result count. The rule-listing
// case (an eighth row alongside the seven count knobs) proves D-03's
// "complete set, up to the maximum" contract when the true count is smaller
// than the maximum.
func TestZeroCountKeepsSurfaceDefaults(t *testing.T) {
	d := testDeps(t)
	owner := "oor-zero-owner-" + uuid.NewString()
	scope := "iso-test:project:oor-zero-" + uuid.NewString()
	scheduledScope := "iso-test:project:oor-zero-sched-" + uuid.NewString()
	discoveryScope := "discovery:repo:oor-zero-" + uuid.NewString()
	ruleScope := "rule:repo:oor-zero-" + uuid.NewString()
	ctx := authedContext(t, owner)
	c := callerFor(ctx, t)
	t.Cleanup(func() {
		bg := context.Background()
		cleanupErr(t, "DeleteAll "+scope, d.st.DeleteAll(bg, scope, store.Authenticated(owner)))
		cleanupErr(t, "DeleteAll "+scheduledScope, d.st.DeleteAll(bg, scheduledScope, store.Authenticated(owner)))
		cleanupErr(t, "DeleteAll "+discoveryScope, d.st.DeleteAll(bg, discoveryScope, store.Authenticated(owner)))
		cleanupErr(t, "DeleteAll "+ruleScope, d.st.DeleteAll(bg, ruleScope, store.Anonymous()))
	})

	// Seed strictly more than the largest documented default (20) in every
	// scope, so an unintentionally widened OR narrowed default is visible as
	// a changed count rather than accidentally matching by coincidence.
	const seedCount = 25
	for i := 0; i < seedCount; i++ {
		if _, _, err := d.storeMemory(ctx, c, storeArgs{
			Content: fmt.Sprintf("oor-zero memory %d", i), Scope: scope, Source: "agent-inferred", Category: "decision",
		}); err != nil {
			t.Fatalf("seed memory %d: %v", i, err)
		}
	}
	future := time.Now().Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339)
	for i := 0; i < seedCount; i++ {
		if _, _, err := d.scheduleMemory(ctx, c, scheduleArgs{
			storeArgs: storeArgs{Content: fmt.Sprintf("oor-zero scheduled %d", i), Scope: scheduledScope, Source: "agent-inferred", Category: "decision"},
			NotBefore: future,
		}); err != nil {
			t.Fatalf("seed scheduled %d: %v", i, err)
		}
	}
	cite := []citationArg{{Kind: "file", Ref: "f"}}
	for i := 0; i < seedCount; i++ {
		if _, _, err := d.storeDiscovery(ctx, c, storeDiscoveryArgs{
			Content: fmt.Sprintf("oor-zero discovery %d", i), Kind: "fact", Scope: discoveryScope, Citations: cite,
		}); err != nil {
			t.Fatalf("seed discovery %d: %v", i, err)
		}
	}
	const ruleCount = 5 // deliberately SMALLER than store.MaxRecallLimit
	for i := 0; i < ruleCount; i++ {
		if _, _, err := d.storeRule(ctx, c, storeRuleArgs{
			Content: fmt.Sprintf("oor-zero rule %d", i), Scope: ruleScope, Summary: fmt.Sprintf("oor-zero rule %d", i),
		}); err != nil {
			t.Fatalf("seed rule %d: %v", i, err)
		}
	}

	cs, tctx := newOutOfRangeMCPSession(t, d, owner)
	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": owner}})

	// connect_list_memories: NOT 20. D-01 (04-CONTEXT.md, locked one-way,
	// already implemented pre-this-plan) resolves a Connect ListMemories
	// zero limit to store.MaxRecallLimit, never a small per-surface default —
	// this is the one surface among the seven whose "zero count" behavior is
	// governed by D-01, not D-08. Asserting 20 here would be asserting a
	// requirement this plan's own locked decision forbids; the correct
	// pinned behavior is "every seeded record comes back" (seedCount is far
	// below the maximum). See this plan's own action text vs. D-01 note in
	// the SUMMARY.
	t.Run("connect_list_memories", func(t *testing.T) {
		res, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 0}))
		if err != nil {
			t.Fatalf("ListMemories: %v", err)
		}
		if got := len(res.Msg.Memories); got != seedCount {
			t.Errorf("ListMemories limit=0: got %d memories, want %d (D-01: 0 resolves to store.MaxRecallLimit, not a small default)", got, seedCount)
		}
	})
	t.Run("connect_search_memories", func(t *testing.T) {
		res, err := api.SearchMemories(actx, connect.NewRequest(&engramv1.SearchMemoriesRequest{Scope: scope, Query: "probe", K: 0}))
		if err != nil {
			t.Fatalf("SearchMemories: %v", err)
		}
		if got := len(res.Msg.Memories); got != 20 {
			t.Errorf("SearchMemories k=0: got %d memories, want 20", got)
		}
	})
	t.Run("connect_search_discoveries", func(t *testing.T) {
		res, err := api.SearchDiscoveries(actx, connect.NewRequest(&engramv1.SearchDiscoveriesRequest{Scope: discoveryScope, Query: "probe", K: 0}))
		if err != nil {
			t.Fatalf("SearchDiscoveries: %v", err)
		}
		if got := len(res.Msg.Discoveries); got != 20 {
			t.Errorf("SearchDiscoveries k=0: got %d discoveries, want 20", got)
		}
	})
	t.Run("mcp_list_memory", func(t *testing.T) {
		if got := mcpResultCount(tctx, t, cs, "list_memory", map[string]any{"scope": scope}, "memories"); got != 20 {
			t.Errorf("list_memory (no limit): got %d memories, want 20", got)
		}
	})
	t.Run("mcp_list_scheduled", func(t *testing.T) {
		if got := mcpResultCount(tctx, t, cs, "list_scheduled", map[string]any{"scope": scheduledScope}, "memories"); got != 20 {
			t.Errorf("list_scheduled (no limit): got %d memories, want 20", got)
		}
	})
	t.Run("mcp_search_memory", func(t *testing.T) {
		if got := mcpResultCount(tctx, t, cs, "search_memory", map[string]any{"scope": scope, "query": "probe"}, "memories"); got != 8 {
			t.Errorf("search_memory (no k): got %d memories, want 8", got)
		}
	})
	t.Run("mcp_search_discovery", func(t *testing.T) {
		if got := mcpResultCount(tctx, t, cs, "search_discovery", map[string]any{"scope": discoveryScope, "query": "probe"}, "discoveries"); got != 8 {
			t.Errorf("search_discovery (no k): got %d discoveries, want 8", got)
		}
	})
	t.Run("list_rules", func(t *testing.T) {
		out, _, err := d.listRules(ctx, c, listRulesArgs{Scopes: []string{ruleScope}})
		if err != nil {
			t.Fatalf("listRules: %v", err)
		}
		if got := len(out); got != ruleCount {
			t.Errorf("listRules: got %d rules, want the complete set of %d (smaller than store.MaxRecallLimit)", got, ruleCount)
		}
	})
}
