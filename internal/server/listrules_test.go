// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves ListRules' all-scopes widening (milestone 2026-09-25.01
// Phase 3, D-10) end to end on Connect against a real Qdrant.

package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

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
