// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"slices"
	"sort"
	"testing"

	"connectrpc.com/connect"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// crossSpineFixture seeds the shared two-owner, two-scope fixture used by all
// three tests in this file: owner A holds a private record in scopeShared and
// a private record in scopeAOnly; owner B holds a private record (must never
// leak) and a shared record (positive control) both in scopeShared. Every
// seeded record carries fixtureTag — mem_eval_test is shared package-wide, so
// the tag is what turns a containment check into a genuine equality/parity
// assertion rather than a hopeful one.
func crossSpineFixture(t *testing.T, d *deps) (scopeShared, scopeAOnly, ownerA, ownerB, fixtureTag string, ids struct{ aShared, aOnly, bPrivate, bShared string }) {
	t.Helper()
	scopeShared = "connect-xspine:project:shared"
	scopeAOnly = "connect-xspine:project:a-only"
	ownerA = "sub-connect-xspine-A"
	ownerB = "sub-connect-xspine-B"
	fixtureTag = "connect-xspine-fixture-7d1c"

	ids.aShared = "c5c50004-0000-0000-0000-000000000001"
	ids.aOnly = "c5c50004-0000-0000-0000-000000000002"
	ids.bPrivate = "c5c50004-0000-0000-0000-000000000003"
	ids.bShared = "c5c50004-0000-0000-0000-000000000004"

	ctx := context.Background()
	mk := func(id, owner, scope, vis string) {
		m := store.Memory{
			ID: id, Content: "x", Scope: scope, Owner: owner, Visibility: vis,
			Tags: []string{fixtureTag}, CreatedAt: timeNow(),
		}
		if err := d.st.Upsert(ctx, m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	mk(ids.aShared, ownerA, scopeShared, "")
	mk(ids.aOnly, ownerA, scopeAOnly, "")
	mk(ids.bPrivate, ownerB, scopeShared, "")
	mk(ids.bShared, ownerB, scopeShared, "shared")
	t.Cleanup(func() {
		cleanupErr(t, "DeleteAll A/shared", d.st.DeleteAll(context.Background(), scopeShared, store.Authenticated(ownerA)))
		cleanupErr(t, "DeleteAll A/aonly", d.st.DeleteAll(context.Background(), scopeAOnly, store.Authenticated(ownerA)))
		cleanupErr(t, "DeleteAll B/shared", d.st.DeleteAll(context.Background(), scopeShared, store.Authenticated(ownerB)))
	})
	return scopeShared, scopeAOnly, ownerA, ownerB, fixtureTag, ids
}

func connectCtxFor(owner string) context.Context {
	return withConnectTokenInfo(context.Background(), &mcpauth.TokenInfo{Extra: map[string]any{"owner_claim": owner}})
}

// TestConnectCrossSpineScopeRequired pins D-04/T-03-02: an empty scope
// without cross_spine on either Connect memory RPC is rejected with
// CodeInvalidArgument specifically — never CodeInternal, and never a result
// set. Asserts the CODE, not the message text (round-4 error-taxonomy
// discipline this package already follows).
func TestConnectCrossSpineScopeRequired(t *testing.T) {
	d := testDeps(t)
	api := &engramAPI{d: d}
	ctx := connectCtxFor("sub-connect-xspine-required")

	t.Run("SearchMemories", func(t *testing.T) {
		_, err := api.SearchMemories(ctx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Query: "x", Scope: "", CrossSpine: false, K: 4,
		}))
		if err == nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("SearchMemories empty scope, cross_spine=false: got %v, want CodeInvalidArgument", err)
		}
	})

	t.Run("ListMemories", func(t *testing.T) {
		_, err := api.ListMemories(ctx, connect.NewRequest(&engramv1.ListMemoriesRequest{
			Scope: "", CrossSpine: false, Limit: 10,
		}))
		if err == nil || connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("ListMemories empty scope, cross_spine=false: got %v, want CodeInvalidArgument", err)
		}
	})
}

// TestConnectCrossSpineNotInferred pins D-04, the phase's anti-pattern guard:
// SearchMemories and ListMemories must NEVER map an empty scope to
// cross-spine the way SearchDiscoveries deliberately does at
// connectapi.go:266. This is a behavioral pin rather than a grep, because
// SearchDiscoveries legitimately contains `req.Msg.Scope == ""` in the same
// file and a file-scoped negative grep would be wrong; a region-scoped one
// would be evadable. The SearchDiscoveries assertion in the same test pins
// that its inference is UNCHANGED — the asymmetry is deliberate, not an
// oversight to "fix" later.
func TestConnectCrossSpineNotInferred(t *testing.T) {
	d := testDeps(t)
	api := &engramAPI{d: d}
	scopeShared, _, ownerA, _, fixtureTag, _ := crossSpineFixture(t, d)
	ctx := connectCtxFor(ownerA)

	t.Run("SearchMemories_empty_scope_errors_not_cross_spine", func(t *testing.T) {
		resp, err := api.SearchMemories(ctx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Query: "x", Scope: "", CrossSpine: false, K: 10, Tags: []string{fixtureTag},
		}))
		if err == nil {
			t.Fatalf("SearchMemories with empty scope produced a result set instead of erroring: %+v", resp)
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("SearchMemories empty scope: got code %v, want CodeInvalidArgument", connect.CodeOf(err))
		}
	})

	t.Run("ListMemories_empty_scope_errors_not_cross_spine", func(t *testing.T) {
		resp, err := api.ListMemories(ctx, connect.NewRequest(&engramv1.ListMemoriesRequest{
			Scope: "", CrossSpine: false, Limit: 10, Tags: []string{fixtureTag},
		}))
		if err == nil {
			t.Fatalf("ListMemories with empty scope produced a result set instead of erroring: %+v", resp)
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("ListMemories empty scope: got code %v, want CodeInvalidArgument", connect.CodeOf(err))
		}
	})

	// SearchDiscoveries' pre-existing empty-scope inference is UNCHANGED: it
	// still succeeds on an empty scope and still spans discovery scopes. This
	// pins the asymmetry as deliberate rather than an inconsistency someone
	// should later "fix" to match the memory RPCs.
	t.Run("SearchDiscoveries_empty_scope_still_infers_cross_spine", func(t *testing.T) {
		discScope := "discovery:connect-xspine:project"
		id, _, err := d.storeDiscovery(context.Background(), callerFor(authedContext(t, ownerA), t), storeDiscoveryArgs{
			Content: "a discovery fact for connect cross-spine asymmetry pin",
			Kind:    "fact",
			Scope:   discScope,
			Citations: []citationArg{
				{Kind: "file", Ref: "internal/server/connectapi.go", Excerpt: "SearchDiscoveries"},
			},
		})
		if err != nil {
			t.Fatalf("seed discovery: %v", err)
		}
		t.Cleanup(func() {
			cleanupErr(t, "Delete discovery", d.st.Delete(context.Background(), id, store.Authenticated(ownerA)))
		})

		resp, err := api.SearchDiscoveries(ctx, connect.NewRequest(&engramv1.SearchDiscoveriesRequest{
			Query: "connect cross-spine asymmetry pin", Scope: "", K: 10,
		}))
		if err != nil {
			t.Fatalf("SearchDiscoveries with empty scope should still succeed (inferred cross-spine): %v", err)
		}
		found := false
		for _, m := range resp.Msg.Discoveries {
			if m.Id == id {
				found = true
			}
		}
		if !found {
			t.Errorf("SearchDiscoveries empty-scope inferred cross-spine did not return the seeded discovery %s", id)
		}
	})

	// Scope-confined call is unaffected by the guard.
	t.Run("scope_confined_search_still_works", func(t *testing.T) {
		resp, err := api.SearchMemories(ctx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Query: "x", Scope: scopeShared, CrossSpine: false, K: 10, Tags: []string{fixtureTag},
		}))
		if err != nil {
			t.Fatalf("scope-confined SearchMemories: %v", err)
		}
		if len(resp.Msg.Memories) == 0 {
			t.Errorf("scope-confined SearchMemories with a valid scope returned no records")
		}
		if len(resp.Msg.SearchedScopes) != 0 || resp.Msg.ScopesTruncated {
			t.Errorf("scope-confined response should carry neither searched_scopes nor scopes_truncated, got %v / %v", resp.Msg.SearchedScopes, resp.Msg.ScopesTruncated)
		}
	})
}

// TestSearchMemoriesConnectCrossSpine proves criterion 4 (MCP<->Connect
// parity) for both search_memory and list_memory: the id set returned by the
// Connect handler with CrossSpine=true equals (set equality, not length) the
// id set returned by the corresponding deps method with CrossSpine=true over
// the identical fixture. It also pins per-result scope attribution (D-11),
// searched_scopes containment (D-12 — never equality: the enumeration is the
// authorized span, not the set of scopes with hits), and that owner B's
// private record leaks into neither lane.
func TestSearchMemoriesConnectCrossSpine(t *testing.T) {
	d := testDeps(t)
	api := &engramAPI{d: d}
	scopeShared, scopeAOnly, ownerA, _, fixtureTag, ids := crossSpineFixture(t, d)

	mcpCtx := authedContext(t, ownerA)
	mcpCaller := callerFor(mcpCtx, t)
	connCtx := connectCtxFor(ownerA)

	sortedIDs := func(ss []string) []string {
		out := slices.Clone(ss)
		sort.Strings(out)
		return out
	}

	t.Run("search", func(t *testing.T) {
		// Connect's own k default (20) differs from MCP's (8) — set both
		// explicitly so the two lanes are compared over the same result
		// window, not two different defaults.
		const k = 10
		mcpHits, err := d.searchMemory(mcpCtx, mcpCaller, coreSearchRequest{
			Query: "x", Scope: "", CrossSpine: true, K: k, Tags: []string{fixtureTag},
		})
		if err != nil {
			t.Fatalf("MCP searchMemory cross-spine: %v", err)
		}
		mcpIDs := make([]string, len(mcpHits))
		mcpScopes := map[string]string{}
		for i, m := range mcpHits {
			mcpIDs[i] = m.ID
			mcpScopes[m.ID] = m.Scope
			if m.Scope == "" {
				t.Errorf("MCP cross-spine result %s carries an empty scope", m.ID)
			}
		}

		resp, err := api.SearchMemories(connCtx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Query: "x", Scope: "", CrossSpine: true, K: k, Tags: []string{fixtureTag},
		}))
		if err != nil {
			t.Fatalf("Connect SearchMemories cross-spine: %v", err)
		}
		connIDs := make([]string, len(resp.Msg.Memories))
		for i, m := range resp.Msg.Memories {
			connIDs[i] = m.Id
			if m.Scope == "" {
				t.Errorf("Connect cross-spine result %s carries an empty scope", m.Id)
			}
			if m.Id == ids.bPrivate {
				t.Fatalf("Connect cross-spine search leaked owner B's private record: %s", m.Id)
			}
		}
		if slices.Contains(mcpIDs, ids.bPrivate) {
			t.Fatalf("MCP cross-spine search leaked owner B's private record: %s", ids.bPrivate)
		}

		if !slices.Equal(sortedIDs(mcpIDs), sortedIDs(connIDs)) {
			t.Fatalf("MCP/Connect cross-spine search id set mismatch:\n MCP:     %v\n Connect: %v", sortedIDs(mcpIDs), sortedIDs(connIDs))
		}

		wantScopes := []string{scopeShared, scopeAOnly}
		for _, sc := range wantScopes {
			if !slices.Contains(resp.Msg.SearchedScopes, sc) {
				t.Errorf("Connect cross-spine search_scopes missing seeded scope %q: %v", sc, resp.Msg.SearchedScopes)
			}
		}
	})

	t.Run("list", func(t *testing.T) {
		mcpRes, err := d.listMemory(mcpCtx, mcpCaller, coreListRequest{
			Scope: "", CrossSpine: true, Limit: 0, Tags: []string{fixtureTag}, CursorMode: true,
		})
		if err != nil {
			t.Fatalf("MCP listMemory cross-spine: %v", err)
		}
		mcpIDs := make([]string, len(mcpRes.Memories))
		for i, m := range mcpRes.Memories {
			mcpIDs[i] = m.ID
			if m.Scope == "" {
				t.Errorf("MCP cross-spine list result %s carries an empty scope", m.ID)
			}
		}
		if slices.Contains(mcpIDs, ids.bPrivate) {
			t.Fatalf("MCP cross-spine list leaked owner B's private record: %s", ids.bPrivate)
		}

		resp, err := api.ListMemories(connCtx, connect.NewRequest(&engramv1.ListMemoriesRequest{
			Scope: "", CrossSpine: true, Limit: 0, Tags: []string{fixtureTag},
		}))
		if err != nil {
			t.Fatalf("Connect ListMemories cross-spine: %v", err)
		}
		connIDs := make([]string, len(resp.Msg.Memories))
		for i, m := range resp.Msg.Memories {
			connIDs[i] = m.Id
			if m.Scope == "" {
				t.Errorf("Connect cross-spine list result %s carries an empty scope", m.Id)
			}
			if m.Id == ids.bPrivate {
				t.Fatalf("Connect cross-spine list leaked owner B's private record: %s", m.Id)
			}
		}

		if !slices.Equal(sortedIDs(mcpIDs), sortedIDs(connIDs)) {
			t.Fatalf("MCP/Connect cross-spine list id set mismatch:\n MCP:     %v\n Connect: %v", sortedIDs(mcpIDs), sortedIDs(connIDs))
		}

		wantScopes := []string{scopeShared, scopeAOnly}
		for _, sc := range wantScopes {
			if !slices.Contains(resp.Msg.SearchedScopes, sc) {
				t.Errorf("Connect cross-spine list searched_scopes missing seeded scope %q: %v", sc, resp.Msg.SearchedScopes)
			}
		}
	})

	t.Run("scope_confined_carries_no_coverage_keys", func(t *testing.T) {
		resp, err := api.SearchMemories(connCtx, connect.NewRequest(&engramv1.SearchMemoriesRequest{
			Query: "x", Scope: scopeShared, CrossSpine: false, K: 10, Tags: []string{fixtureTag},
		}))
		if err != nil {
			t.Fatalf("scope-confined SearchMemories: %v", err)
		}
		if len(resp.Msg.SearchedScopes) != 0 || resp.Msg.ScopesTruncated {
			t.Errorf("scope-confined SearchMemories should carry neither key, got %v / %v", resp.Msg.SearchedScopes, resp.Msg.ScopesTruncated)
		}

		lresp, err := api.ListMemories(connCtx, connect.NewRequest(&engramv1.ListMemoriesRequest{
			Scope: scopeShared, CrossSpine: false, Limit: 10, Tags: []string{fixtureTag},
		}))
		if err != nil {
			t.Fatalf("scope-confined ListMemories: %v", err)
		}
		if len(lresp.Msg.SearchedScopes) != 0 || lresp.Msg.ScopesTruncated {
			t.Errorf("scope-confined ListMemories should carry neither key, got %v / %v", lresp.Msg.SearchedScopes, lresp.Msg.ScopesTruncated)
		}
	})
}
