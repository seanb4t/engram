// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves ListScheduled's cross_spine + cursor widening (milestone
// 2026-09-25.01 Phase 3, D-11) end to end on Connect, and the D-19
// deferred-reveal, paging, and coverage proofs on both lanes.

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

// seedScheduledFixture upserts a windowed (pending or expired) record for
// owner in scope, with the given created_at, and registers its cleanup.
// pending selects a not_before an hour in the future (hidden until then);
// !pending selects a not_after an hour in the past (already lapsed).
func seedScheduledFixture(t *testing.T, st *store.Store, owner, scope string, createdAt time.Time, pending, shared bool) string {
	t.Helper()
	id := uuid.NewString()
	m := store.Memory{
		ID: id, Content: "scheduled fixture", Scope: scope,
		Category: "gotcha", Source: "user-said", Owner: owner,
		CreatedAt: createdAt,
	}
	if pending {
		future := time.Now().UTC().Add(time.Hour)
		m.NotBefore = &future
	} else {
		past := time.Now().UTC().Add(-time.Hour)
		m.NotAfter = &past
	}
	if shared {
		m.Visibility = "shared"
	}
	if err := st.Upsert(context.Background(), m, []float32{0.1, 0.2, 0.3}); err != nil {
		t.Fatalf("seed scheduled fixture %s: %v", id, err)
	}
	t.Cleanup(func() {
		cleanupErr(t, "Delete "+id, st.Delete(context.Background(), id, store.Authenticated(owner)))
	})
	return id
}

// TestListScheduledConnectCrossSpinePages proves D-11 end to end against a
// real Qdrant: owner A's five pending records across two scopes page
// correctly under cross_spine+limit, in created_at-descending order, with no
// duplicates and no gaps, while owner B's SHARED pending record in the same
// scope as some of A's records never appears (deferred reveal, D-11).
func TestListScheduledConnectCrossSpinePages(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	ownerA := "sub-listscheduled-a-" + uuid.NewString()
	ownerB := "sub-listscheduled-b-" + uuid.NewString()
	scope1 := "iso-test:project:listscheduled-s1-" + uuid.NewString()
	scope2 := "iso-test:project:listscheduled-s2-" + uuid.NewString()
	ctx := parityConnectCtx(ownerA)

	base := time.Now().UTC().Truncate(time.Second)
	var wantIDs []string
	// Three in scope1, two in scope2, distinct whole-second created_at —
	// descending offsets so the union is already in the expected order.
	for i := range 3 {
		wantIDs = append(wantIDs, seedScheduledFixture(t, st, ownerA, scope1, base.Add(time.Duration(-i)*time.Second), true, false))
	}
	for i := 3; i < 5; i++ {
		wantIDs = append(wantIDs, seedScheduledFixture(t, st, ownerA, scope2, base.Add(time.Duration(-i)*time.Second), true, false))
	}
	// Owner B's SHARED pending record in scope1 must never appear to owner A.
	bID := seedScheduledFixture(t, st, ownerB, scope1, base.Add(time.Second), true, true)

	var (
		gotIDs    []string
		pageSizes []int
		lastAt    time.Time
		haveLast  bool
		token     string
	)
	for {
		resp, err := api.ListScheduled(ctx, connect.NewRequest(&engramv1.ListScheduledRequest{
			CrossSpine: true, Limit: 2, PageToken: token,
		}))
		if err != nil {
			t.Fatalf("ListScheduled: %v", err)
		}
		mems := resp.Msg.GetMemories()
		pageSizes = append(pageSizes, len(mems))
		for _, m := range mems {
			gotIDs = append(gotIDs, m.GetId())
			at := m.GetCreatedAt().AsTime()
			if haveLast && at.After(lastAt) {
				t.Errorf("created_at increased mid-sequence: %v after %v (not non-increasing)", at, lastAt)
			}
			lastAt, haveLast = at, true
		}
		if resp.Msg.GetScopesUnknown() {
			t.Error("scopes_unknown = true, want false")
		}
		if len(resp.Msg.GetSearchedScopes()) == 0 {
			t.Error("searched_scopes empty on a cross_spine response, want non-empty")
		}
		token = resp.Msg.GetNextPageToken()
		if token == "" {
			break
		}
	}

	if len(pageSizes) != 3 || pageSizes[0] != 2 || pageSizes[1] != 2 || pageSizes[2] != 1 {
		t.Errorf("page sizes = %v, want [2 2 1]", pageSizes)
	}
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("got %d ids across all pages, want %d", len(gotIDs), len(wantIDs))
	}
	seen := make(map[string]bool, len(gotIDs))
	for _, id := range gotIDs {
		if seen[id] {
			t.Errorf("duplicate id %s across pages", id)
		}
		seen[id] = true
		if id == bID {
			t.Errorf("owner B's shared record %s appeared in owner A's cross_spine ListScheduled — deferred reveal violated (D-11)", bID)
		}
	}
	for _, id := range wantIDs {
		if !seen[id] {
			t.Errorf("expected id %s missing from the paged result", id)
		}
	}
}

// mcpMemoryIDs extracts the "id" field from an MCP list_scheduled result's
// "memories" entries (a []any of map[string]any after the JSON round trip
// through the client), skipping any entry that fails to decode as expected
// rather than panicking — a decode failure is a test bug worth a loud
// mismatch below, not a crash.
func mcpMemoryIDs(t *testing.T, memories any) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	mems, ok := memories.([]any)
	if !ok {
		t.Fatalf("memories is %T, want []any", memories)
	}
	for _, mm := range mems {
		mem, ok := mm.(map[string]any)
		if !ok {
			t.Fatalf("memory entry is %T, want map[string]any", mm)
		}
		id, _ := mem["id"].(string)
		out[id] = true
	}
	return out
}

// TestListScheduledCrossSpineDeferredReveal proves D-19's ListScheduled
// share on both lanes against a real Qdrant: owner B has a SHARED pending
// record and a SHARED expired record in the same scope S owner A also uses.
// For every state (scheduled/expired/all) and every lane, B's records are
// absent while A's matching records are present — the deferred-reveal
// guarantee holds even when cross_spine spans every scope.
func TestListScheduledCrossSpineDeferredReveal(t *testing.T) {
	d, st := testDepsWithStore(t)
	api := &engramAPI{d: d}

	ownerA := "sub-listscheduled-deferred-a-" + uuid.NewString()
	ownerB := "sub-listscheduled-deferred-b-" + uuid.NewString()
	scope := "iso-test:project:listscheduled-deferred-" + uuid.NewString()
	base := time.Now().UTC().Truncate(time.Second)

	aPending := seedScheduledFixture(t, st, ownerA, scope, base, true, false)
	aExpired := seedScheduledFixture(t, st, ownerA, scope, base.Add(-time.Second), false, false)
	bPending := seedScheduledFixture(t, st, ownerB, scope, base.Add(-2*time.Second), true, true)
	bExpired := seedScheduledFixture(t, st, ownerB, scope, base.Add(-3*time.Second), false, true)

	cases := []struct {
		state string
		wantA []string
	}{
		{"scheduled", []string{aPending}},
		{"expired", []string{aExpired}},
		{"all", []string{aPending, aExpired}},
	}

	assertDeferred := func(t *testing.T, state string, wantA []string, gotIDs map[string]bool) {
		t.Helper()
		for _, id := range []string{bPending, bExpired} {
			if gotIDs[id] {
				t.Errorf("state=%s: owner B's shared record %s appeared, want absent (deferred reveal)", state, id)
			}
		}
		for _, id := range wantA {
			if !gotIDs[id] {
				t.Errorf("state=%s: owner A's record %s absent, want present", state, id)
			}
		}
	}

	t.Run("Connect", func(t *testing.T) {
		ctx := parityConnectCtx(ownerA)
		for _, tc := range cases {
			t.Run(tc.state, func(t *testing.T) {
				resp, err := api.ListScheduled(ctx, connect.NewRequest(&engramv1.ListScheduledRequest{
					Scope: scope, State: tc.state, CrossSpine: true, Limit: store.MaxRecallLimit,
				}))
				if err != nil {
					t.Fatalf("ListScheduled: %v", err)
				}
				gotIDs := map[string]bool{}
				for _, m := range resp.Msg.GetMemories() {
					gotIDs[m.GetId()] = true
				}
				assertDeferred(t, tc.state, tc.wantA, gotIDs)
			})
		}
	})

	t.Run("MCP", func(t *testing.T) {
		ctx, cs := newMCPSession(t, d, ownerA)
		for _, tc := range cases {
			t.Run(tc.state, func(t *testing.T) {
				res, err := cs.CallTool(ctx, &mcp.CallToolParams{
					Name:      "list_scheduled",
					Arguments: map[string]any{"scope": scope, "state": tc.state, "cross_spine": true, "limit": float64(store.MaxRecallLimit)},
				})
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
				gotIDs := mcpMemoryIDs(t, m["memories"])
				assertDeferred(t, tc.state, tc.wantA, gotIDs)
			})
		}
	})
}

// TestListScheduledCursorPagesBothLanes proves D-19's paging claim on the
// MCP lane against a real Qdrant: the Task-1 fixture shape (owner A's five
// pending records across two scopes) pages exactly once each under
// cross_spine+cursor through the list_scheduled tool, and state=expired with
// cross_spine returns only A's expired records, never a pending one.
func TestListScheduledCursorPagesBothLanes(t *testing.T) {
	d, st := testDepsWithStore(t)

	ownerA := "sub-listscheduled-cursor-a-" + uuid.NewString()
	scope1 := "iso-test:project:listscheduled-cursor-s1-" + uuid.NewString()
	scope2 := "iso-test:project:listscheduled-cursor-s2-" + uuid.NewString()

	base := time.Now().UTC().Truncate(time.Second)
	var wantPending []string
	for i := range 3 {
		wantPending = append(wantPending, seedScheduledFixture(t, st, ownerA, scope1, base.Add(time.Duration(-i)*time.Second), true, false))
	}
	for i := 3; i < 5; i++ {
		wantPending = append(wantPending, seedScheduledFixture(t, st, ownerA, scope2, base.Add(time.Duration(-i)*time.Second), true, false))
	}
	expired1 := seedScheduledFixture(t, st, ownerA, scope1, base.Add(-10*time.Second), false, false)
	expired2 := seedScheduledFixture(t, st, ownerA, scope2, base.Add(-11*time.Second), false, false)
	wantExpired := []string{expired1, expired2}

	ctx, cs := newMCPSession(t, d, ownerA)

	var gotIDs []string
	cursor := ""
	for {
		args := map[string]any{"cross_spine": true, "state": "all", "limit": float64(2)}
		if cursor != "" {
			args["cursor"] = cursor
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_scheduled", Arguments: args})
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
		for id := range mcpMemoryIDs(t, m["memories"]) {
			gotIDs = append(gotIDs, id)
		}
		next, _ := m["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}

	seen := make(map[string]bool, len(gotIDs))
	for _, id := range gotIDs {
		if seen[id] {
			t.Errorf("duplicate id %s across pages", id)
		}
		seen[id] = true
	}
	wantAll := append(append([]string{}, wantPending...), wantExpired...)
	if len(gotIDs) != len(wantAll) {
		t.Fatalf("got %d ids across all pages, want %d", len(gotIDs), len(wantAll))
	}
	for _, id := range wantAll {
		if !seen[id] {
			t.Errorf("expected id %s missing from the paged result", id)
		}
	}

	// state=expired with cross_spine returns only A's expired records.
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_scheduled",
		Arguments: map[string]any{"cross_spine": true, "state": "expired", "limit": float64(store.MaxRecallLimit)},
	})
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
	gotExpired := mcpMemoryIDs(t, m["memories"])
	for _, id := range wantPending {
		if gotExpired[id] {
			t.Errorf("state=expired returned a pending id %s", id)
		}
	}
	for _, id := range wantExpired {
		if !gotExpired[id] {
			t.Errorf("state=expired missing expected id %s", id)
		}
	}
}

// TestListScheduledCoverageThreeStates is ListScheduled's instance of the
// D-19 coverage table (mirroring TestCrossSpineCoverageThreeStates's shape
// exactly): "not cross-spine" carries none of the three coverage keys;
// "cross-spine, coverage known" reports searched_scopes equal to the sorted
// readable scopes of the fixture — including a scope holding only another
// owner's SHARED, non-scheduled record, which ListScopes' owner-or-shared
// predicate makes readable even though ListScheduled itself (owner-only)
// never returns anything from it; "cross-spine, coverage query failed"
// reports scopes_unknown true with searched_scopes absent, memories intact.
func TestListScheduledCoverageThreeStates(t *testing.T) {
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
		name       string
		crossSpine bool
		listErr    error
		wantMCP    wantMCP
		wantConn   wantConnect
	}{
		{
			name:       "not cross-spine",
			crossSpine: false,
		},
		{
			name:       "cross-spine, coverage known",
			crossSpine: true,
			wantMCP:    wantMCP{hasSearchedScopes: true, hasScopesTruncated: true},
			wantConn:   wantConnect{searchedScopesLen: 2},
		},
		{
			name:       "cross-spine, coverage query failed",
			crossSpine: true,
			listErr:    errors.New("scheduled_test: sentinel three-states failure 9c4e"),
			wantMCP:    wantMCP{hasScopesUnknown: true, unknownValue: true},
			wantConn:   wantConnect{scopesUnknown: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ownerA := "sub-listscheduled-coverage-a-" + uuid.NewString()
			ownerB := "sub-listscheduled-coverage-b-" + uuid.NewString()
			// "s1-"/"s2-" prefixes keep scope1 < scope2 lexicographically
			// regardless of the per-run uuid suffix, so searched_scopes'
			// sorted order is deterministic across runs.
			scope1 := "listscheduled-coverage:project:s1-" + uuid.NewString()
			scope2 := "listscheduled-coverage:project:s2-" + uuid.NewString()

			sp := newSpyStore()
			future := time.Now().UTC().Add(time.Hour)
			if err := sp.Upsert(context.Background(), store.Memory{
				ID: uuid.NewString(), Content: "pending", Scope: scope1, Owner: ownerA,
				NotBefore: &future, CreatedAt: time.Now().UTC(),
			}, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("seed pending: %v", err)
			}
			// A scope holding only owner B's SHARED, non-scheduled record —
			// readable to A via ListScopes' owner-or-shared predicate, even
			// though ListScheduled itself (owner-only) never returns it.
			if err := sp.Upsert(context.Background(), store.Memory{
				ID: uuid.NewString(), Content: "shared", Scope: scope2, Owner: ownerB,
				Visibility: "shared", CreatedAt: time.Now().UTC(),
			}, []float32{0.1, 0.2, 0.3}); err != nil {
				t.Fatalf("seed shared: %v", err)
			}

			wrapper := &failingListScopesStore{spyStore: sp, scope: scope1, listErr: tc.listErr}
			d := &deps{st: wrapper, em: fakeEmbedder{}, summaryMaxChars: 500}

			t.Run("MCP", func(t *testing.T) {
				ctx, cs := newMCPSession(t, d, ownerA)
				res, err := cs.CallTool(ctx, &mcp.CallToolParams{
					Name:      "list_scheduled",
					Arguments: map[string]any{"scope": scope1, "cross_spine": tc.crossSpine},
				})
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
					want := []string{scope1, scope2}
					if len(got) != len(want) {
						t.Fatalf("searched_scopes = %v, want %v", got, want)
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
				resp, err := api.ListScheduled(connectCtxFor(ownerA), connect.NewRequest(&engramv1.ListScheduledRequest{
					Scope: scope1, CrossSpine: tc.crossSpine,
				}))
				if err != nil {
					t.Fatalf("ListScheduled: %v", err)
				}
				if got := len(resp.Msg.GetSearchedScopes()); got != tc.wantConn.searchedScopesLen {
					t.Errorf("len(SearchedScopes) = %d, want %d", got, tc.wantConn.searchedScopesLen)
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
