// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

func TestMemoryToProto(t *testing.T) {
	now := time.Now().UTC()
	m := store.Memory{
		ID: "id1", Content: "c", Scope: "s", Owner: "sub-A",
		Visibility: "shared", Tags: []string{"x", "y"}, CreatedAt: now,
	}
	pb := memoryToProto(m)
	if pb.Id != "id1" || pb.Owner != "sub-A" || pb.Visibility != "shared" {
		t.Errorf("scalar fields not mapped: %+v", pb)
	}
	if len(pb.Tags) != 2 || pb.CreatedAt.AsTime().Unix() != now.Unix() {
		t.Errorf("tags/created_at not mapped: %+v", pb)
	}
}

// seedCrossActorRecords inserts a shared and a private record owned by actor-A into the
// given store, returning cleanup functions. Uses a unique scope derived from the test name.
func seedCrossActorRecords(t *testing.T, d *deps, scope string) (shared, priv store.Memory) {
	t.Helper()
	shared = store.Memory{
		ID: "d2222222-0000-0000-0000-000000000001", Content: "shared",
		Scope: scope, Owner: "actor-A", Visibility: "shared", CreatedAt: timeNow(),
	}
	priv = store.Memory{
		ID: "d2222222-0000-0000-0000-000000000002", Content: "private",
		Scope: scope, Owner: "actor-A", Visibility: "", CreatedAt: timeNow(),
	}
	ctx := context.Background()
	for _, m := range []store.Memory{shared, priv} {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete shared", d.st.Delete(ctx, shared.ID, store.Authenticated("actor-A")))
		cleanupErr(t, "Delete priv", d.st.Delete(ctx, priv.ID, store.Authenticated("actor-A")))
	})
	return shared, priv
}

func TestConnectCrossActorIsolation(t *testing.T) {
	d := testDeps(t)
	api := &engramAPI{d: d}
	scope := "iso-test:project:connect-xactor"
	shared, priv := seedCrossActorRecords(t, d, scope)
	ctx := context.Background()

	// caller B (distinct authed owner-claim value) injected via the test seam.
	bctx := withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "actor-B"}})

	t.Run("ListMemories_negative_no_private_leak", func(t *testing.T) {
		resp, err := api.ListMemories(bctx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10}))
		if err != nil {
			t.Fatalf("ListMemories: %v", err)
		}
		for _, m := range resp.Msg.Memories {
			if m.Owner == "actor-A" && m.Visibility != "shared" {
				t.Errorf("B leaked A's private record %s via Connect", m.Id)
			}
		}
	})

	t.Run("ListMemories_positive_shared_returned", func(t *testing.T) {
		resp, err := api.ListMemories(bctx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10}))
		if err != nil {
			t.Fatalf("ListMemories: %v", err)
		}
		found := false
		for _, m := range resp.Msg.Memories {
			if m.Id == shared.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("B should see A's shared record %s via ListMemories, but it was absent", shared.ID)
		}
	})

	t.Run("GetMemory_private_denied", func(t *testing.T) {
		_, err := api.GetMemory(bctx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: priv.ID}))
		if err == nil {
			t.Fatal("GetMemory: expected error for A's private record, got nil")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("GetMemory: expected CodeNotFound, got %v", connect.CodeOf(err))
		}
	})

	t.Run("GetMemory_shared_allowed", func(t *testing.T) {
		resp, err := api.GetMemory(bctx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: shared.ID}))
		if err != nil {
			t.Fatalf("GetMemory: expected success for A's shared record, got %v", err)
		}
		if resp.Msg.Memory.Id != shared.ID {
			t.Errorf("GetMemory: returned id %q, want %q", resp.Msg.Memory.Id, shared.ID)
		}
	})

	t.Run("SearchMemories_no_private_leak", func(t *testing.T) {
		resp, err := api.SearchMemories(bctx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Scope: scope, Query: "any", K: 20,
		}))
		if err != nil {
			t.Fatalf("SearchMemories: %v", err)
		}
		for _, m := range resp.Msg.Memories {
			if m.Owner == "actor-A" && m.Visibility != "shared" {
				t.Errorf("SearchMemories: B leaked A's private record %s", m.Id)
			}
		}
	})

	t.Run("Anonymous_ListMemories_sees_nothing_of_actor_A", func(t *testing.T) {
		// Anonymous = interceptor ran with nil TokenInfo (auth disabled / no issuer).
		// Inject via the test seam the same way the interceptor would: key present, ti==nil.
		anonCtx := withConnectTokenInfo(context.Background(), nil)
		resp, err := api.ListMemories(anonCtx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10}))
		if err != nil {
			t.Fatalf("ListMemories(anon): %v", err)
		}
		for _, m := range resp.Msg.Memories {
			if m.Owner != "" {
				t.Errorf("Anonymous caller saw record owned by %q (id=%s); expected empty-owner bucket only", m.Owner, m.Id)
			}
		}
	})

	t.Run("NoInterceptor_fails_closed", func(t *testing.T) {
		// Bare context.Background() has no connectSubjectKey → interceptor was never installed.
		// subjectFromConnectContext must fail closed with an error, NOT silently grant anonymous access.
		_, err := api.ListMemories(context.Background(), connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10}))
		if err == nil {
			t.Fatal("ListMemories with absent interceptor key: expected error (fail-closed), got nil")
		}
	})
}

func TestListMemoriesHandlerPagesAndIsolates(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "hdl-page:project:x"
	// owner-A: 3 convention; owner-B: 1 private convention.
	seed := []store.Memory{
		{ID: "a1000000-0000-0000-0000-000000000001", Content: "A1", Scope: scope, Owner: "owner-A", Visibility: "private", Category: "convention", Source: "agent-inferred", CreatedAt: timeNow()},
		{ID: "a1000000-0000-0000-0000-000000000002", Content: "A2", Scope: scope, Owner: "owner-A", Visibility: "shared", Category: "gotcha", Source: "agent-inferred", CreatedAt: timeNow()},
		{ID: "a1000000-0000-0000-0000-000000000003", Content: "B1", Scope: scope, Owner: "owner-B", Visibility: "private", Category: "convention", Source: "agent-inferred", CreatedAt: timeNow()},
	}
	for _, m := range seed {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	defer func() {
		_ = d.st.Delete(ctx, seed[0].ID, store.Authenticated("owner-A"))
		_ = d.st.Delete(ctx, seed[1].ID, store.Authenticated("owner-A"))
		_ = d.st.Delete(ctx, seed[2].ID, store.Authenticated("owner-B"))
	}()
	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "owner-A"}})

	// A with category=convention -> only A's convention record; total 1.
	resp, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: scope, Limit: 10, Categories: []string{"convention"},
	}))
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if resp.Msg.Total != 1 || len(resp.Msg.Memories) != 1 || resp.Msg.Memories[0].Owner != "owner-A" {
		t.Fatalf("got total=%d len=%d", resp.Msg.Total, len(resp.Msg.Memories))
	}
}

// TestListMemoriesCursorModeBootstrap pins engram-3hp9: a pure-Connect client
// opts into cursor paging via cursor_mode=true on a tokenless first page and
// receives a usable next_page_token, then resumes with it to drain the rest —
// with no overlap and full coverage. The UI's default (cursor_mode=false, no
// page_token) stays in offset mode and never emits a token (ADR engram-1frj).
func TestListMemoriesCursorModeBootstrap(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "hdl-cursor:project:x"
	// Three owner-A records with staggered CreatedAt so the created_at-desc
	// keyset boundary is deterministic (newest first: c3, c2, c1).
	base := timeNow()
	seed := []store.Memory{
		{ID: "a3000000-0000-0000-0000-000000000001", Content: "c1", Scope: scope, Owner: "owner-A", Category: "convention", Source: "agent-inferred", CreatedAt: base.Add(-2 * time.Minute)},
		{ID: "a3000000-0000-0000-0000-000000000002", Content: "c2", Scope: scope, Owner: "owner-A", Category: "convention", Source: "agent-inferred", CreatedAt: base.Add(-1 * time.Minute)},
		{ID: "a3000000-0000-0000-0000-000000000003", Content: "c3", Scope: scope, Owner: "owner-A", Category: "convention", Source: "agent-inferred", CreatedAt: base},
	}
	for _, m := range seed {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	defer func() {
		for _, m := range seed {
			_ = d.st.Delete(ctx, m.ID, store.Authenticated("owner-A"))
		}
	}()
	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "owner-A"}})

	// Page 1: cursor_mode=true, no page_token (bootstrap). Limit 2 of 3 ->
	// a full page that MUST yield a next_page_token.
	p1, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: scope, Limit: 2, CursorMode: true,
	}))
	if err != nil {
		t.Fatalf("ListMemories page1: %v", err)
	}
	if p1.Msg.Total != 3 {
		t.Errorf("page1 total: got %d want 3", p1.Msg.Total)
	}
	if len(p1.Msg.Memories) != 2 {
		t.Fatalf("page1 len: got %d want 2", len(p1.Msg.Memories))
	}
	if p1.Msg.NextPageToken == "" {
		t.Fatal("page1 cursor_mode bootstrap returned empty next_page_token (engram-3hp9 regression)")
	}

	// Page 2: resume with the token. Drains the final record, token empties.
	p2, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: scope, Limit: 2, PageToken: p1.Msg.NextPageToken,
	}))
	if err != nil {
		t.Fatalf("ListMemories page2: %v", err)
	}
	if len(p2.Msg.Memories) != 1 {
		t.Fatalf("page2 len: got %d want 1", len(p2.Msg.Memories))
	}
	if p2.Msg.NextPageToken != "" {
		t.Errorf("page2 should be the last page (empty token), got %q", p2.Msg.NextPageToken)
	}

	// Union of both pages = all 3 ids, no overlap.
	got := map[string]bool{}
	for _, m := range append(p1.Msg.Memories, p2.Msg.Memories...) {
		if got[m.Id] {
			t.Errorf("duplicate id across pages: %s", m.Id)
		}
		got[m.Id] = true
	}
	for _, m := range seed {
		if !got[m.ID] {
			t.Errorf("record %s missing across paged results", m.ID)
		}
	}

	// UI default: cursor_mode=false, no page_token -> offset mode, never a token.
	off, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: scope, Limit: 2,
	}))
	if err != nil {
		t.Fatalf("ListMemories offset: %v", err)
	}
	if off.Msg.NextPageToken != "" {
		t.Errorf("offset-mode default must not emit a cursor, got %q", off.Msg.NextPageToken)
	}
}

// TestConnectTagsFilter pins tag-filter parity on the Connect read API: both
// ListMemories and SearchMemories honor the optional tags filter (AND — records
// must carry all listed tags), matching the MCP search_memory/list_memory tools.
func TestConnectTagsFilter(t *testing.T) {
	d := testDeps(t)
	ctx := context.Background()
	scope := "hdl-tags:project:x"
	seed := []store.Memory{
		{ID: "a2000000-0000-0000-0000-000000000001", Content: "x", Scope: scope, Owner: "owner-A", Category: "convention", Source: "agent-inferred", Tags: []string{"go", "perf"}, CreatedAt: timeNow()},
		{ID: "a2000000-0000-0000-0000-000000000002", Content: "x", Scope: scope, Owner: "owner-A", Category: "convention", Source: "agent-inferred", Tags: []string{"go"}, CreatedAt: timeNow()},
		{ID: "a2000000-0000-0000-0000-000000000003", Content: "x", Scope: scope, Owner: "owner-A", Category: "convention", Source: "agent-inferred", Tags: []string{"python"}, CreatedAt: timeNow()},
	}
	for _, m := range seed {
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	defer func() {
		for _, m := range seed {
			_ = d.st.Delete(ctx, m.ID, store.Authenticated("owner-A"))
		}
	}()
	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(ctx, &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "owner-A"}})

	ids := func(ms []*engramv1.Memory) map[string]bool {
		out := map[string]bool{}
		for _, m := range ms {
			out[m.Id] = true
		}
		return out
	}

	// ListMemories single tag -> the two go-tagged records (not python).
	lresp, err := api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10, Tags: []string{"go"}}))
	if err != nil {
		t.Fatalf("ListMemories go: %v", err)
	}
	if g := ids(lresp.Msg.Memories); !g[seed[0].ID] || !g[seed[1].ID] || g[seed[2].ID] || lresp.Msg.Total != 2 {
		t.Errorf("ListMemories go: total=%d ids=%v", lresp.Msg.Total, g)
	}

	// ListMemories AND of two tags -> only the record carrying both.
	lresp, err = api.ListMemories(actx, connect.NewRequest(&engramv1.ListMemoriesRequest{Scope: scope, Limit: 10, Tags: []string{"go", "perf"}}))
	if err != nil {
		t.Fatalf("ListMemories AND: %v", err)
	}
	if g := ids(lresp.Msg.Memories); !g[seed[0].ID] || g[seed[1].ID] || lresp.Msg.Total != 1 {
		t.Errorf("ListMemories AND: total=%d ids=%v", lresp.Msg.Total, g)
	}

	// SearchMemories single tag -> the two go-tagged records, never python.
	sresp, err := api.SearchMemories(actx, connect.NewRequest(&engramv1.SearchMemoriesRequest{Query: "x", Scope: scope, K: 10, Tags: []string{"go"}}))
	if err != nil {
		t.Fatalf("SearchMemories go: %v", err)
	}
	if g := ids(sresp.Msg.Memories); !g[seed[0].ID] || !g[seed[1].ID] || g[seed[2].ID] {
		t.Errorf("SearchMemories go: ids=%v", g)
	}
}

func TestMemoryToProtoMapsSummary(t *testing.T) {
	pb := memoryToProto(store.Memory{ID: "1", Summary: "terse", SummarySource: store.SummarySourceAuto})
	if pb.Summary != "terse" || pb.SummarySource != "auto" {
		t.Fatalf("summary fields not mapped: %+v", pb)
	}
}

func TestShapeProtoMemoriesFullFlag(t *testing.T) {
	ms := []store.Memory{{ID: "1", Content: "long body over the cap here", Summary: "kept", SummarySource: store.SummarySourceClient}}
	full := shapeProtoMemories(ms, true, 4)
	if full[0].Content == "" {
		t.Fatal("full=true must keep content")
	}
	compact := shapeProtoMemories(ms, false, 4)
	if compact[0].Content != "" || compact[0].Summary != "kept" {
		t.Fatalf("full=false must clear content and keep summary: %+v", compact[0])
	}
}

// Note: SearchDiscoveries shares the identical subjectFromConnectContext seam.
// Coverage is skipped here because discovery-scope seeding requires a separate
// collection setup and discovery-specific Upsert path not yet exposed from testDeps.

func TestListMemoriesRejectsBadCreatedAfter(t *testing.T) {
	api := &engramAPI{d: testDeps(t)}
	ctx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "sub-A"}})
	_, err := api.ListMemories(ctx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: "s:project:x", CreatedAfter: "not-a-date",
	}))
	if err == nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("bad created_after: got %v want InvalidArgument", err)
	}
}

// TestListMemoriesRejectsMalformedPageToken pins the error-code taxonomy
// (g0ne.2/.6/.10): a garbage page token is a CLIENT error (CodeInvalidArgument
// via store.ErrInvalidArgument), not a CodeInternal infra failure.
func TestListMemoriesRejectsMalformedPageToken(t *testing.T) {
	api := &engramAPI{d: testDeps(t)}
	ctx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "sub-A"}})
	_, err := api.ListMemories(ctx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: "s:project:x", PageToken: "!!!garbage!!!",
	}))
	if err == nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("malformed page_token: got %v want InvalidArgument", err)
	}
}

// TestListMemoriesRejectsCursorModeWithOffset pins the contract documented on
// the cursor_mode field (engram-3hp9): cursor_mode and offset>0 are mutually
// exclusive. The store enforces this (store.go) and the handler maps
// store.ErrInvalidArgument to CodeInvalidArgument — this test pins that mapping
// at the Connect boundary for the new flag so a future error-mapping refactor
// can't silently regress it.
func TestListMemoriesRejectsCursorModeWithOffset(t *testing.T) {
	api := &engramAPI{d: testDeps(t)}
	ctx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "sub-A"}})
	_, err := api.ListMemories(ctx, connect.NewRequest(&engramv1.ListMemoriesRequest{
		Scope: "s:project:x", Limit: 10, Offset: 5, CursorMode: true,
	}))
	if err == nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("cursor_mode + offset>0: got %v want InvalidArgument", err)
	}
}

func TestMountConnectSkipsWhenResolverNil(t *testing.T) {
	d := &deps{} // no store needed; we never serve a request
	mux := http.NewServeMux()
	if err := d.mountConnect(mux, nil); err != nil {
		t.Fatalf("mountConnect(nil): %v", err)
	}
	// With no resolver, the EngramService path must NOT be registered: a request
	// to it falls through to the mux's 404.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/engram.v1.EngramService/ListScopes", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 (Connect must be unmounted when resolver is nil)", rec.Code)
	}
}

func TestMountConnectMountsWhenResolverPresent(t *testing.T) {
	d := &deps{}
	mux := http.NewServeMux()
	resolve := func(context.Context, connect.AnyRequest) (*mcpauth.TokenInfo, error) { return nil, nil }
	if err := d.mountConnect(mux, resolve); err != nil {
		t.Fatalf("mountConnect: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/engram.v1.EngramService/ListScopes", nil)
	mux.ServeHTTP(rec, req)
	// Mounted: connect-go responds (415/400 for a malformed body), NOT 404.
	if rec.Code == http.StatusNotFound {
		t.Fatal("Connect path should be mounted when a resolver is present")
	}
}

// TestConnectGetMemoryByShortIDAndProtoField pins that Connect's GetMemory
// resolves a short id to its point UUID (mirroring the MCP by-id tools) and
// that the resolved record's short id round-trips on the wire via the new
// Memory.short_id proto field.
func TestConnectGetMemoryByShortIDAndProtoField(t *testing.T) {
	d := testDeps(t)
	ctxA := authedContext(t, "owner-A")
	id, sid, err := d.storeMemory(ctxA, storeArgs{Content: "hello", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	api := &engramAPI{d: d}
	actx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "owner-A"}})
	resp, err := api.GetMemory(actx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: sid}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.Memory.Id != id {
		t.Fatalf("short id resolved to %q, want %q", resp.Msg.Memory.Id, id)
	}
	if resp.Msg.Memory.ShortId != sid {
		t.Fatalf("proto ShortId = %q, want %q", resp.Msg.Memory.ShortId, sid)
	}
}

// TestConnectGetMemoryCrossOwnerShortIDDoesNotLeakUUID pins that a cross-owner
// GetMemory-by-short-id failure never leaks the resolved point UUID: the gate
// resolves the short id to another owner's real UUID before the ownership
// check runs, so the CodeNotFound error must echo only the caller-supplied
// short id, never that resolved UUID (mirrors tools_test.go's
// TestShortIDCrossOwnerVisibility no-leak assertion).
func TestConnectGetMemoryCrossOwnerShortIDDoesNotLeakUUID(t *testing.T) {
	d := testDeps(t)
	ctxA := authedContext(t, "owner-A")
	id, sid, err := d.storeMemory(ctxA, storeArgs{Content: "secret", Scope: "s", Category: "gotcha", Source: "user-said"})
	if err != nil {
		t.Fatal(err)
	}
	api := &engramAPI{d: d}
	bctx := withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": "owner-B"}})
	_, err = api.GetMemory(bctx, connect.NewRequest(&engramv1.GetMemoryRequest{Id: sid}))
	if err == nil || connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("cross-owner short id: got %v, want CodeNotFound", err)
	}
	if strings.Contains(err.Error(), id) {
		t.Fatalf("error leaks resolved UUID: %v", err)
	}
	if !strings.Contains(err.Error(), sid) {
		t.Fatalf("error should echo caller-supplied short id only: %v", err)
	}
}
