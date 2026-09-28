// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves ListRules' all-scopes widening (milestone 2026-09-25.01
// Phase 3, D-10) end to end on Connect against a real Qdrant, and the D-19
// all-scopes/coverage proofs on both lanes.

package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// TestListRulesConnectAllScopes proves D-10/RPC-03 end to end against a real
// Qdrant: an empty scopes list on the Connect ListRules RPC is one
// cross-scope read of every readable rule:* scope, oldest-first, compact by
// default, with rule-only coverage — never the pre-D-10 rejection. An
// explicit scope keeps today's contract exactly (single scope, no coverage
// keys; full=true surfaces content).
//
// base is deliberately far in the past (year 2000): the shared test
// collection holds other tests' shared rules, seeded at real wall-clock
// time. Anchoring this test's two rules at the very start of the ascending
// order keeps them well inside store.MaxRecallLimit's 1000-record cap
// regardless of how many other rules the suite has accumulated.
func TestListRulesConnectAllScopes(t *testing.T) {
	d := testDeps(t)
	base := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	var tick int64
	d.now = func() time.Time { tick++; return base.Add(time.Duration(tick) * time.Second) }

	ownerA := "sub-listrules-allscopes-a-" + uuid.NewString()
	ownerB := "sub-listrules-allscopes-b-" + uuid.NewString()
	scopeRepo := "rule:repo:listrules-allscopes-" + uuid.NewString()
	scopeProject := "rule:project:listrules-allscopes-" + uuid.NewString()
	ctxA := authedContext(t, ownerA)
	t.Cleanup(func() {
		bg := context.Background()
		cleanupErr(t, "DeleteAll "+scopeRepo, d.st.DeleteAll(bg, scopeRepo, store.Authenticated(ownerA)))
		cleanupErr(t, "DeleteAll "+scopeProject, d.st.DeleteAll(bg, scopeProject, store.Authenticated(ownerA)))
	})

	idR1, _, err := d.storeRule(ctxA, callerFor(ctxA, t), storeRuleArgs{
		Content: "R1 content", Scope: scopeRepo, Summary: "R1 summary",
	})
	if err != nil {
		t.Fatalf("storeRule R1: %v", err)
	}
	idR2, _, err := d.storeRule(ctxA, callerFor(ctxA, t), storeRuleArgs{
		Content: "R2 content", Scope: scopeProject, Summary: "R2 summary",
	})
	if err != nil {
		t.Fatalf("storeRule R2: %v", err)
	}

	api := &engramAPI{d: d}
	// Rules are always shared: a DIFFERENT authenticated owner reads them via
	// the all-scopes read with no scopes named.
	ctxB := connectCtxFor(ownerB)

	resp, err := api.ListRules(ctxB, connect.NewRequest(&engramv1.ListRulesRequest{}))
	if err != nil {
		t.Fatalf("ListRules (empty scopes): %v", err)
	}
	rules := resp.Msg.GetRules()
	var gotR1, gotR2 bool
	var lastCreated time.Time
	var haveLast bool
	for _, r := range rules {
		at := r.GetCreatedAt().AsTime()
		if haveLast && at.Before(lastCreated) {
			t.Fatalf("ListRules (empty scopes): not oldest-first — %v before %v", at, lastCreated)
		}
		lastCreated, haveLast = at, true
		if r.GetContent() != "" {
			t.Errorf("ListRules (empty scopes): rule %s carries content in the compact shape", r.GetId())
		}
		if r.GetSummary() == "" {
			t.Errorf("ListRules (empty scopes): rule %s has no summary in the compact shape", r.GetId())
		}
		switch r.GetId() {
		case idR1:
			gotR1 = true
		case idR2:
			gotR2 = true
		}
	}
	if !gotR1 || !gotR2 {
		t.Fatalf("ListRules (empty scopes): got R1=%v R2=%v among %d rules, want both present", gotR1, gotR2, len(rules))
	}
	for _, sc := range resp.Msg.GetSearchedScopes() {
		if !validRuleScope(sc) {
			t.Errorf("ListRules (empty scopes): searched_scopes contains a non-rule scope %q", sc)
		}
	}
	if resp.Msg.GetScopesUnknown() {
		t.Error("ListRules (empty scopes): ScopesUnknown = true, want false")
	}

	// Explicit scope: R1's scope only, no coverage keys.
	respScoped, err := api.ListRules(ctxB, connect.NewRequest(&engramv1.ListRulesRequest{Scopes: []string{scopeRepo}}))
	if err != nil {
		t.Fatalf("ListRules (explicit scope): %v", err)
	}
	if got := respScoped.Msg.GetRules(); len(got) != 1 || got[0].GetId() != idR1 {
		t.Fatalf("ListRules (explicit scope): got %d rules, want exactly [R1]", len(got))
	}
	if len(respScoped.Msg.GetSearchedScopes()) != 0 {
		t.Errorf("ListRules (explicit scope): SearchedScopes = %v, want empty", respScoped.Msg.GetSearchedScopes())
	}

	// full=true on an explicit scope surfaces content.
	respFull, err := api.ListRules(ctxB, connect.NewRequest(&engramv1.ListRulesRequest{Scopes: []string{scopeRepo}, Full: true}))
	if err != nil {
		t.Fatalf("ListRules (full): %v", err)
	}
	if got := respFull.Msg.GetRules(); len(got) != 1 || got[0].GetContent() != "R1 content" {
		t.Fatalf("ListRules (full): got %d rules, content = %q, want 1 rule with content %q", len(got), rulesContent(got), "R1 content")
	}
}

// rulesContent renders the Content of every rule for a failure message.
func rulesContent(rs []*engramv1.Memory) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.GetContent()
	}
	return out
}

// ruleViewIDs extracts the ID of every ruleView in an MCP-lane
// deps.listRules result (compact shape, a.Full == false).
func ruleViewIDs(t *testing.T, rules []any) []string {
	t.Helper()
	out := make([]string, len(rules))
	for i, r := range rules {
		rv, ok := r.(ruleView)
		if !ok {
			t.Fatalf("rules[%d] is %T, want ruleView", i, r)
		}
		out[i] = rv.ID
	}
	return out
}

// TestListRulesAllScopesBothLanes proves D-10/D-19 on both lanes against a
// real Qdrant: an empty scopes list is the all-scopes read on the MCP tool
// and the Connect RPC alike, the tags filter composes with it, and an
// explicit-scopes call stays byte-identical (per-scope order, no coverage
// keys).
func TestListRulesAllScopesBothLanes(t *testing.T) {
	d := testDeps(t)
	base := time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)
	var tick int64
	d.now = func() time.Time { tick++; return base.Add(time.Duration(tick) * time.Second) }

	ownerA := "sub-listrules-bothlanes-a-" + uuid.NewString()
	ownerB := "sub-listrules-bothlanes-b-" + uuid.NewString()
	scope1 := "rule:repo:listrules-bothlanes-" + uuid.NewString()
	scope2 := "rule:project:listrules-bothlanes-" + uuid.NewString()
	tag := "t-" + uuid.NewString()
	ctxA := authedContext(t, ownerA)
	t.Cleanup(func() {
		bg := context.Background()
		cleanupErr(t, "DeleteAll "+scope1, d.st.DeleteAll(bg, scope1, store.Authenticated(ownerA)))
		cleanupErr(t, "DeleteAll "+scope2, d.st.DeleteAll(bg, scope2, store.Authenticated(ownerA)))
	})

	idA1, _, err := d.storeRule(ctxA, callerFor(ctxA, t), storeRuleArgs{Content: "a1", Scope: scope1, Summary: "a1"})
	if err != nil {
		t.Fatalf("seed a1: %v", err)
	}
	idA2, _, err := d.storeRule(ctxA, callerFor(ctxA, t), storeRuleArgs{Content: "a2", Scope: scope1, Summary: "a2", Tags: []string{tag}})
	if err != nil {
		t.Fatalf("seed a2: %v", err)
	}
	idB1, _, err := d.storeRule(ctxA, callerFor(ctxA, t), storeRuleArgs{Content: "b1", Scope: scope2, Summary: "b1"})
	if err != nil {
		t.Fatalf("seed b1: %v", err)
	}

	t.Run("MCP_all_scopes", func(t *testing.T) {
		mctx, cs := newMCPSession(t, d, ownerB)
		structured, _ := callToolTextJSON(mctx, t, cs, "list_rules", map[string]any{})
		ids := mcpMemoryIDs(t, structured["rules"])
		if !ids[idA1] || !ids[idA2] || !ids[idB1] {
			t.Fatalf("list_rules {} ids = %v, want to contain a1=%s a2=%s b1=%s", ids, idA1, idA2, idB1)
		}
		if _, ok := structured["searched_scopes"]; !ok {
			t.Error("list_rules {} missing searched_scopes")
		}
	})

	t.Run("Connect_all_scopes", func(t *testing.T) {
		api := &engramAPI{d: d}
		resp, err := api.ListRules(connectCtxFor(ownerB), connect.NewRequest(&engramv1.ListRulesRequest{}))
		if err != nil {
			t.Fatalf("ListRules {}: %v", err)
		}
		ids := connectMemoryIDs(resp.Msg.GetRules())
		got := map[string]bool{}
		for _, id := range ids {
			got[id] = true
		}
		if !got[idA1] || !got[idA2] || !got[idB1] {
			t.Fatalf("ListRules{} ids = %v, want to contain a1=%s a2=%s b1=%s", ids, idA1, idA2, idB1)
		}
		if len(resp.Msg.GetSearchedScopes()) == 0 {
			t.Error("ListRules{} SearchedScopes is empty, want present")
		}
	})

	t.Run("tags_compose_with_all_scopes", func(t *testing.T) {
		mctx, cs := newMCPSession(t, d, ownerB)
		structured, _ := callToolTextJSON(mctx, t, cs, "list_rules", map[string]any{"tags": []string{tag}})
		ids := mcpMemoryIDs(t, structured["rules"])
		if len(ids) != 1 || !ids[idA2] {
			t.Fatalf("list_rules {tags} ids = %v, want exactly [%s]", ids, idA2)
		}

		api := &engramAPI{d: d}
		resp, err := api.ListRules(connectCtxFor(ownerB), connect.NewRequest(&engramv1.ListRulesRequest{Tags: []string{tag}}))
		if err != nil {
			t.Fatalf("ListRules {tags}: %v", err)
		}
		connIDs := connectMemoryIDs(resp.Msg.GetRules())
		if len(connIDs) != 1 || connIDs[0] != idA2 {
			t.Fatalf("ListRules {tags} ids = %v, want exactly [%s]", connIDs, idA2)
		}
	})

	t.Run("explicit_scopes_no_coverage", func(t *testing.T) {
		api := &engramAPI{d: d}
		resp, err := api.ListRules(connectCtxFor(ownerB), connect.NewRequest(&engramv1.ListRulesRequest{Scopes: []string{scope1, scope2}}))
		if err != nil {
			t.Fatalf("ListRules explicit scopes: %v", err)
		}
		ids := connectMemoryIDs(resp.Msg.GetRules())
		if len(ids) != 3 || ids[0] != idA1 || ids[1] != idA2 || ids[2] != idB1 {
			t.Fatalf("ListRules explicit scopes ids = %v, want [%s %s %s]", ids, idA1, idA2, idB1)
		}
		if len(resp.Msg.GetSearchedScopes()) != 0 {
			t.Errorf("ListRules explicit scopes: SearchedScopes = %v, want empty", resp.Msg.GetSearchedScopes())
		}
	})
}

// TestListRulesCoverageThreeStates proves D-19's rule-only coverage
// three-state contract on both lanes via failingListScopesStore: explicit
// scopes carry no coverage keys; an all-scopes call with a working coverage
// enumeration reports searched_scopes filtered to rule:* scopes ONLY (a
// readable non-rule scope must never appear, T-03-18); a failing enumeration
// reports scopes_unknown with searched_scopes absent, rules still returned.
func TestListRulesCoverageThreeStates(t *testing.T) {
	type wantMCP struct {
		hasSearchedScopes  bool
		hasScopesTruncated bool
		hasScopesUnknown   bool
		unknownValue       bool
	}
	type wantConnect struct {
		searchedScopesLen int
		scopesTruncated   bool
		scopesUnknown     bool
	}

	cases := []struct {
		name      string
		allScopes bool
		listErr   error
		wantMCP   wantMCP
		wantConn  wantConnect
	}{
		{
			name: "explicit scopes",
		},
		{
			name:      "all scopes, coverage known",
			allScopes: true,
			wantMCP:   wantMCP{hasSearchedScopes: true, hasScopesTruncated: true},
			wantConn:  wantConnect{searchedScopesLen: 1},
		},
		{
			name:      "all scopes, coverage query failed",
			allScopes: true,
			listErr:   errors.New("listrules_test: sentinel three-states failure 6b1f"),
			wantMCP:   wantMCP{hasScopesUnknown: true, unknownValue: true},
			wantConn:  wantConnect{scopesUnknown: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ownerA := "sub-listrules-coverage-a-" + uuid.NewString()
			ownerB := "sub-listrules-coverage-b-" + uuid.NewString()
			ruleScope := "rule:repo:listrules-coverage-" + uuid.NewString()
			nonRuleScope := "listrules-coverage:project:" + uuid.NewString()

			sp := newSpyStore()
			if err := sp.Upsert(context.Background(), store.Memory{
				ID: uuid.NewString(), Content: "rule content", Summary: "rule summary",
				Scope: ruleScope, Owner: ownerA, Category: "rule", Visibility: "shared",
				CreatedAt: time.Now().UTC(),
			}, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("seed rule: %v", err)
			}
			// A scope holding only owner B's SHARED, non-rule record — readable
			// to A via ListScopes' owner-or-shared predicate, but must never be
			// reported among searched_scopes for a rules read (T-03-18).
			if err := sp.Upsert(context.Background(), store.Memory{
				ID: uuid.NewString(), Content: "non-rule", Scope: nonRuleScope, Owner: ownerB,
				Category: "gotcha", Visibility: "shared", CreatedAt: time.Now().UTC(),
			}, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("seed non-rule: %v", err)
			}

			wrapper := &failingListScopesStore{spyStore: sp, scope: ruleScope, listErr: tc.listErr}
			d := &deps{st: wrapper, em: fakeEmbedder{}, summaryMaxChars: 500}

			var connScopes []string
			var mcpArgs map[string]any
			if tc.allScopes {
				mcpArgs = map[string]any{}
			} else {
				connScopes = []string{ruleScope}
				mcpArgs = map[string]any{"scopes": []string{ruleScope}}
			}

			t.Run("MCP", func(t *testing.T) {
				ctx, cs := newMCPSession(t, d, ownerA)
				res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_rules", Arguments: mcpArgs})
				if err != nil {
					t.Fatalf("CallTool: %v", err)
				}
				if res.IsError {
					t.Fatalf("CallTool: IsError = true, want false (content: %+v)", res.Content)
				}
				m, ok := res.StructuredContent.(map[string]any)
				if !ok {
					t.Fatalf("StructuredContent is %T, want map[string]any", res.StructuredContent)
				}

				if _, present := m["rules"]; !present {
					t.Error("structured content missing \"rules\" — the seeded rule must survive even a coverage-unknown response")
				}

				searchedVal, hasSearched := m["searched_scopes"]
				if hasSearched != tc.wantMCP.hasSearchedScopes {
					t.Errorf("searched_scopes present = %v, want %v", hasSearched, tc.wantMCP.hasSearchedScopes)
				}
				truncatedVal, hasTruncated := m["scopes_truncated"]
				if hasTruncated != tc.wantMCP.hasScopesTruncated {
					t.Errorf("scopes_truncated present = %v, want %v", hasTruncated, tc.wantMCP.hasScopesTruncated)
				}
				if hasTruncated {
					if b, _ := truncatedVal.(bool); b {
						t.Errorf("scopes_truncated = %v, want false", truncatedVal)
					}
				}
				unknownVal, hasUnknown := m["scopes_unknown"]
				if hasUnknown != tc.wantMCP.hasScopesUnknown {
					t.Errorf("scopes_unknown present = %v, want %v", hasUnknown, tc.wantMCP.hasScopesUnknown)
				}
				if hasUnknown {
					if b, _ := unknownVal.(bool); b != tc.wantMCP.unknownValue {
						t.Errorf("scopes_unknown = %v, want %v", unknownVal, tc.wantMCP.unknownValue)
					}
				}
				if tc.wantMCP.hasScopesUnknown {
					if _, present := m["searched_scopes"]; present {
						t.Errorf("coverage-unknown state carries searched_scopes at all (even empty): %v", m["searched_scopes"])
					}
				}
				if hasSearched && tc.wantMCP.hasSearchedScopes {
					got, ok := searchedVal.([]any)
					if !ok {
						t.Fatalf("searched_scopes is %T, want []any", searchedVal)
					}
					want := []string{ruleScope}
					if len(got) != len(want) {
						t.Fatalf("searched_scopes = %v, want %v (the non-rule scope must be filtered out)", got, want)
					}
					for i, w := range want {
						if got[i] != w {
							t.Errorf("searched_scopes[%d] = %v, want %v", i, got[i], w)
						}
					}
				}
			})

			t.Run("Connect", func(t *testing.T) {
				api := &engramAPI{d: d}
				resp, err := api.ListRules(connectCtxFor(ownerA), connect.NewRequest(&engramv1.ListRulesRequest{Scopes: connScopes}))
				if err != nil {
					t.Fatalf("ListRules: %v", err)
				}
				if len(resp.Msg.GetRules()) == 0 {
					t.Error("ListRules: no rules returned, want the seeded rule to survive")
				}
				if got := len(resp.Msg.GetSearchedScopes()); got != tc.wantConn.searchedScopesLen {
					t.Errorf("len(SearchedScopes) = %d, want %d (%v)", got, tc.wantConn.searchedScopesLen, resp.Msg.GetSearchedScopes())
				}
				if resp.Msg.GetScopesTruncated() != tc.wantConn.scopesTruncated {
					t.Errorf("ScopesTruncated = %v, want %v", resp.Msg.GetScopesTruncated(), tc.wantConn.scopesTruncated)
				}
				if resp.Msg.GetScopesUnknown() != tc.wantConn.scopesUnknown {
					t.Errorf("ScopesUnknown = %v, want %v", resp.Msg.GetScopesUnknown(), tc.wantConn.scopesUnknown)
				}
				if tc.wantConn.scopesUnknown && len(resp.Msg.GetSearchedScopes()) != 0 {
					t.Errorf("coverage-unknown state carries a non-empty SearchedScopes: %v", resp.Msg.GetSearchedScopes())
				}
			})
		})
	}
}
