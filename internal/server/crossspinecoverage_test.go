// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// This file proves REQ-cross-spine-partial (#456, 06-CONTEXT.md D-01/D-02/
// D-03/D-04/D-06): a cross-spine recall whose follow-up ListScopes coverage
// query fails must still return the already-authorized hits it found, with
// the coverage-unknown state reported as a wire-visible third state rather
// than discarded as an error. failingListScopesStore is the shared
// failure-injection double every test in this file builds on.

package server

import (
	"context"
	"errors"
	"fmt"
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

// failingListScopesStore embeds *spyStore and overrides ListScopes to return
// a scripted error when listErr is non-nil, in the lostRaceStore shape
// (tools_test.go:4202-4216) — the precondition D-01's tests need is hits
// existing BEFORE the coverage query fails. When listErr is nil, ListScopes
// delegates to the embedded spy unchanged — this is what lets
// TestCrossSpineCoverageThreeStates exercise the "coverage known" state (b)
// through the SAME double as the "coverage unknown" state (c), rather than a
// second type that could silently diverge in scope-translation behavior.
//
// It also overrides List and SearchReranked to translate a cross-spine
// call's resolved empty scope to the seeded fixture scope before delegating
// to the embedded spy: spyStore's own List/SearchReranked filter on exact
// scope equality (m.Scope != scope), so an unmodified cross-spine call
// (which resolves to "" via effectiveSearchScope) would match nothing and
// prove nothing about D-01's "hits survive" contract. The embedded spy's own
// filtering is left untouched — other tests depend on it.
type failingListScopesStore struct {
	*spyStore
	scope   string
	listErr error
}

func (f *failingListScopesStore) ListScopes(ctx context.Context, subj store.Subject) ([]store.ScopeCount, bool, error) {
	if f.listErr != nil {
		return nil, false, f.listErr
	}
	return f.spyStore.ListScopes(ctx, subj)
}

func (f *failingListScopesStore) List(ctx context.Context, scope string, subj store.Subject, opts store.ListOptions) ([]store.Memory, uint64, string, error) {
	if scope == "" {
		scope = f.scope
	}
	return f.spyStore.List(ctx, scope, subj, opts)
}

func (f *failingListScopesStore) SearchReranked(ctx context.Context, scope string, subj store.Subject, query string, vec []float32, k uint64, opts store.SearchOptions) ([]store.Memory, error) {
	if scope == "" {
		scope = f.scope
	}
	return f.spyStore.SearchReranked(ctx, scope, subj, query, vec, k, opts)
}

// seedCoverageFixture upserts n readable records for owner in scope, each
// tagged with fixtureTag, directly on the embedded spy (bypassing any
// embedding/store-level validation this failure-injection double does not
// need to exercise).
func seedCoverageFixture(t *testing.T, sp *spyStore, owner, scope, fixtureTag string, n int) {
	t.Helper()
	for range n {
		m := store.Memory{
			ID:        uuid.NewString(),
			Content:   "x",
			Scope:     scope,
			Owner:     owner,
			Tags:      []string{fixtureTag},
			CreatedAt: time.Now().UTC(),
		}
		if err := sp.Upsert(context.Background(), m, []float32{0.1, 0.2, 0.3}); err != nil {
			t.Fatalf("seed %s: %v", m.ID, err)
		}
	}
}

// TestCrossSpineCoverageUnknownConnectSearch proves the behavior block for
// Connect SearchMemories (06-01-PLAN.md Task 1): a cross-spine search whose
// follow-up ListScopes fails after hits exist still succeeds, with the hits
// intact, ScopesUnknown true, SearchedScopes empty, and ScopesTruncated
// false — and the injected cause is logged exactly once at ERROR level and
// appears nowhere in the response (D-02).
func TestCrossSpineCoverageUnknownConnectSearch(t *testing.T) {
	rec := captureSlog(t)

	const (
		owner      = "sub-coverage-unknown-connect-search"
		scope      = "coverage-unknown:project:connect-search"
		fixtureTag = "coverage-unknown-fixture-3f9a"
	)
	sp := newSpyStore()
	seedCoverageFixture(t, sp, owner, scope, fixtureTag, 2)

	injectedErr := errors.New("crossspinecoverage: sentinel ListScopes failure 8f3c1a")
	wrapper := &failingListScopesStore{spyStore: sp, scope: scope, listErr: injectedErr}
	d := &deps{st: wrapper, em: fakeEmbedder{}, summaryMaxChars: 500}
	api := &engramAPI{d: d}

	resp, err := api.SearchMemories(connectCtxFor(owner), connect.NewRequest(&engramv1.SearchMemoriesRequest{
		Query: "x", CrossSpine: true, K: 10, Tags: []string{fixtureTag},
	}))
	if err != nil {
		t.Fatalf("SearchMemories: got error %v, want nil (D-01: the RPC succeeds)", err)
	}
	if len(resp.Msg.GetMemories()) != 2 {
		t.Fatalf("SearchMemories: got %d memories, want 2 (the already-authorized hits must survive)", len(resp.Msg.GetMemories()))
	}
	if !resp.Msg.GetScopesUnknown() {
		t.Errorf("SearchMemories: ScopesUnknown = false, want true")
	}
	if got := resp.Msg.GetSearchedScopes(); len(got) != 0 {
		t.Errorf("SearchMemories: SearchedScopes = %v, want empty (never a populated-but-wrong list)", got)
	}
	if resp.Msg.GetScopesTruncated() {
		t.Errorf("SearchMemories: ScopesTruncated = true, want false")
	}

	var errorLevelHits int
	for _, r := range rec.containing(injectedErr.Error()) {
		if r.level == slog.LevelError {
			errorLevelHits++
		}
	}
	if errorLevelHits != 1 {
		t.Fatalf("ERROR-level log records mentioning the injected cause: got %d, want exactly 1 (records: %+v)", errorLevelHits, rec.records)
	}

	// D-02: no substring of the injected cause reaches the wire.
	wire := fmt.Sprintf("%+v", resp.Msg)
	if strings.Contains(wire, injectedErr.Error()) {
		t.Errorf("response leaks the injected ListScopes error text: %s", wire)
	}
}

// TestCrossSpineCoverageUnknownConnectList mirrors
// TestCrossSpineCoverageUnknownConnectSearch against Connect ListMemories,
// additionally asserting Total still reports the store's count — the
// evidence that the already-computed result, not a rebuilt empty one,
// reaches the caller.
func TestCrossSpineCoverageUnknownConnectList(t *testing.T) {
	rec := captureSlog(t)

	const (
		owner      = "sub-coverage-unknown-connect-list"
		scope      = "coverage-unknown:project:connect-list"
		fixtureTag = "coverage-unknown-fixture-list-2b7e"
	)
	sp := newSpyStore()
	seedCoverageFixture(t, sp, owner, scope, fixtureTag, 3)

	injectedErr := errors.New("crossspinecoverage: sentinel ListScopes failure connect-list 4d2b")
	wrapper := &failingListScopesStore{spyStore: sp, scope: scope, listErr: injectedErr}
	d := &deps{st: wrapper, em: fakeEmbedder{}, summaryMaxChars: 500}
	api := &engramAPI{d: d}

	resp, err := api.ListMemories(connectCtxFor(owner), connect.NewRequest(&engramv1.ListMemoriesRequest{
		CrossSpine: true, Limit: 10, Tags: []string{fixtureTag},
	}))
	if err != nil {
		t.Fatalf("ListMemories: got error %v, want nil (D-01: the RPC succeeds)", err)
	}
	if len(resp.Msg.GetMemories()) != 3 {
		t.Fatalf("ListMemories: got %d memories, want 3 (the already-authorized hits must survive)", len(resp.Msg.GetMemories()))
	}
	if resp.Msg.GetTotal() != 3 {
		t.Errorf("ListMemories: Total = %d, want 3 (the already-computed store result, not a rebuilt empty one)", resp.Msg.GetTotal())
	}
	if !resp.Msg.GetScopesUnknown() {
		t.Errorf("ListMemories: ScopesUnknown = false, want true")
	}
	if got := resp.Msg.GetSearchedScopes(); len(got) != 0 {
		t.Errorf("ListMemories: SearchedScopes = %v, want empty", got)
	}
	if resp.Msg.GetScopesTruncated() {
		t.Errorf("ListMemories: ScopesTruncated = true, want false")
	}

	var errorLevelHits int
	for _, r := range rec.containing(injectedErr.Error()) {
		if r.level == slog.LevelError {
			errorLevelHits++
		}
	}
	if errorLevelHits != 1 {
		t.Fatalf("ERROR-level log records mentioning the injected cause: got %d, want exactly 1 (records: %+v)", errorLevelHits, rec.records)
	}

	wire := fmt.Sprintf("%+v", resp.Msg)
	if strings.Contains(wire, injectedErr.Error()) {
		t.Errorf("response leaks the injected ListScopes error text: %s", wire)
	}
}

// newMCPSession builds an in-process MCP server (registered against d) and
// client, connected over an in-memory transport under an authed context for
// owner, returning the client session used to drive real tool calls — the
// same harness responsetoolarge_test.go's TestMCPListMemoryResponseTooLarge
// uses. This is what makes the MCP closure bodies — the actual discard sites
// this phase fixes — directly provable rather than approximated by composing
// the helper and the result map by hand.
func newMCPSession(t *testing.T, d *deps, owner string) (context.Context, *mcp.ClientSession) {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "engram-test", Version: "test"}, nil)
	if err := registerTools(s, d); err != nil {
		t.Fatalf("registerTools: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := s.Connect(authedContext(t, owner), serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	c := mcp.NewClient(&mcp.Implementation{Name: "engram-test-client", Version: "test"}, nil)
	cs, err := c.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	return ctx, cs
}

// TestCrossSpineCoverageUnknownMCPSearch proves the MCP search_memory
// closure's discard site is gone, driven through a real in-process tool
// call rather than a helper-composition approximation: the tool call is not
// an error result, the memories entry is non-empty, scopes_unknown is
// present and true, and searched_scopes is ABSENT via a two-value map
// lookup (never a zero-value or empty-slice comparison, which an
// emitted-but-empty key would pass while still breaking the contract).
func TestCrossSpineCoverageUnknownMCPSearch(t *testing.T) {
	rec := captureSlog(t)

	const (
		owner      = "sub-coverage-unknown-mcp-search"
		scope      = "coverage-unknown:project:mcp-search"
		fixtureTag = "coverage-unknown-fixture-mcp-search-9a1c"
	)
	sp := newSpyStore()
	seedCoverageFixture(t, sp, owner, scope, fixtureTag, 2)

	injectedErr := errors.New("crossspinecoverage: sentinel ListScopes failure mcp-search 6e0f")
	wrapper := &failingListScopesStore{spyStore: sp, scope: scope, listErr: injectedErr}
	d := &deps{st: wrapper, em: fakeEmbedder{}, summaryMaxChars: 500}

	ctx, cs := newMCPSession(t, d, owner)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "search_memory",
		Arguments: map[string]any{"query": "x", "cross_spine": true, "tags": []string{fixtureTag}},
	})
	if err != nil {
		t.Fatalf("CallTool: got Go error %v, want nil", err)
	}
	if res.IsError {
		t.Fatalf("CallTool: IsError = true, want false (content: %+v)", res.Content)
	}
	m, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent is %T, want map[string]any", res.StructuredContent)
	}
	mems, ok := m["memories"].([]any)
	if !ok || len(mems) != 2 {
		t.Fatalf("StructuredContent[%q] = %v (%T), want a 2-element slice", "memories", m["memories"], m["memories"])
	}
	unknown, ok := m["scopes_unknown"].(bool)
	if !ok || !unknown {
		t.Errorf("StructuredContent[%q] = %v (%T), want true", "scopes_unknown", m["scopes_unknown"], m["scopes_unknown"])
	}
	if _, present := m["searched_scopes"]; present {
		t.Errorf("StructuredContent unexpectedly carries searched_scopes: %v", m["searched_scopes"])
	}

	var errorLevelHits int
	for _, r := range rec.containing(injectedErr.Error()) {
		if r.level == slog.LevelError {
			errorLevelHits++
		}
	}
	if errorLevelHits != 1 {
		t.Fatalf("ERROR-level log records mentioning the injected cause: got %d, want exactly 1 (records: %+v)", errorLevelHits, rec.records)
	}

	wire := fmt.Sprintf("%+v", res)
	if strings.Contains(wire, injectedErr.Error()) {
		t.Errorf("response leaks the injected ListScopes error text: %s", wire)
	}
}

// TestCrossSpineCoverageUnknownMCPList mirrors
// TestCrossSpineCoverageUnknownMCPSearch against MCP list_memory, additionally
// asserting next_cursor is still present in the result map.
func TestCrossSpineCoverageUnknownMCPList(t *testing.T) {
	rec := captureSlog(t)

	const (
		owner      = "sub-coverage-unknown-mcp-list"
		scope      = "coverage-unknown:project:mcp-list"
		fixtureTag = "coverage-unknown-fixture-mcp-list-5c3d"
	)
	sp := newSpyStore()
	seedCoverageFixture(t, sp, owner, scope, fixtureTag, 2)

	injectedErr := errors.New("crossspinecoverage: sentinel ListScopes failure mcp-list 1b9e")
	wrapper := &failingListScopesStore{spyStore: sp, scope: scope, listErr: injectedErr}
	d := &deps{st: wrapper, em: fakeEmbedder{}, summaryMaxChars: 500}

	ctx, cs := newMCPSession(t, d, owner)

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_memory",
		Arguments: map[string]any{"cross_spine": true, "tags": []string{fixtureTag}},
	})
	if err != nil {
		t.Fatalf("CallTool: got Go error %v, want nil", err)
	}
	if res.IsError {
		t.Fatalf("CallTool: IsError = true, want false (content: %+v)", res.Content)
	}
	m, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent is %T, want map[string]any", res.StructuredContent)
	}
	mems, ok := m["memories"].([]any)
	if !ok || len(mems) != 2 {
		t.Fatalf("StructuredContent[%q] = %v (%T), want a 2-element slice", "memories", m["memories"], m["memories"])
	}
	if _, present := m["next_cursor"]; !present {
		t.Errorf("StructuredContent missing next_cursor")
	}
	unknown, ok := m["scopes_unknown"].(bool)
	if !ok || !unknown {
		t.Errorf("StructuredContent[%q] = %v (%T), want true", "scopes_unknown", m["scopes_unknown"], m["scopes_unknown"])
	}
	if _, present := m["searched_scopes"]; present {
		t.Errorf("StructuredContent unexpectedly carries searched_scopes: %v", m["searched_scopes"])
	}

	var errorLevelHits int
	for _, r := range rec.containing(injectedErr.Error()) {
		if r.level == slog.LevelError {
			errorLevelHits++
		}
	}
	if errorLevelHits != 1 {
		t.Fatalf("ERROR-level log records mentioning the injected cause: got %d, want exactly 1 (records: %+v)", errorLevelHits, rec.records)
	}

	wire := fmt.Sprintf("%+v", res)
	if strings.Contains(wire, injectedErr.Error()) {
		t.Errorf("response leaks the injected ListScopes error text: %s", wire)
	}
}

// TestCrossSpineCoverageThreeStates is the table test D-03 requires: a
// consumer must be able to tell "not cross-spine" (a), "cross-spine,
// coverage known" (b), and "cross-spine, coverage failed" (c) apart on BOTH
// transports, and state (c) must never be representable as an
// empty-but-present searched_scopes — the single property this phase exists
// to protect (a zero-value/length check alone would pass while that
// property was violated, which is why every absence assertion below uses a
// two-value map lookup on the MCP side and a length check on the generated
// getter on the Connect side).
//
// Each row uses the SAME failingListScopesStore double (with listErr nil for
// state (b)) over its own fresh spyStore/owner/scope, so a within-package
// cross-spine ListScopes enumeration from an unrelated test can never leak
// into this table's counts.
func TestCrossSpineCoverageThreeStates(t *testing.T) {
	const fixtureTag = "coverage-three-states-fixture-7c2d"

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
			// State (a): the MCP result map carries none of the three
			// coverage keys at all; both Connect responses carry all three
			// coverage fields at their proto3 zero values (D-14
			// byte-identical guarantee, unchanged by this phase).
			name:       "not cross-spine",
			crossSpine: false,
		},
		{
			// State (b): searched_scopes present and populated,
			// scopes_truncated present, scopes_unknown false/absent.
			name:       "cross-spine, coverage known",
			crossSpine: true,
			wantMCP:    wantMCP{hasSearchedScopes: true, hasScopesTruncated: true},
			wantConn:   wantConnect{searchedScopesLen: 1},
		},
		{
			// State (c): scopes_unknown true, searched_scopes ABSENT (never
			// an empty list), scopes_truncated absent/false.
			name:       "cross-spine, coverage query failed",
			crossSpine: true,
			listErr:    errors.New("crossspinecoverage: sentinel three-states failure 3e7a"),
			wantMCP:    wantMCP{hasScopesUnknown: true, unknownValue: true},
			wantConn:   wantConnect{scopesUnknown: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner := "sub-coverage-three-states-" + uuid.NewString()
			scope := "coverage-three-states:project:" + uuid.NewString()
			sp := newSpyStore()
			seedCoverageFixture(t, sp, owner, scope, fixtureTag, 1)
			wrapper := &failingListScopesStore{spyStore: sp, scope: scope, listErr: tc.listErr}
			d := &deps{st: wrapper, em: fakeEmbedder{}, summaryMaxChars: 500}

			t.Run("MCP", func(t *testing.T) {
				ctx, cs := newMCPSession(t, d, owner)
				res, err := cs.CallTool(ctx, &mcp.CallToolParams{
					Name:      "list_memory",
					Arguments: map[string]any{"scope": scope, "cross_spine": tc.crossSpine, "tags": []string{fixtureTag}},
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

				_, hasSearched := m["searched_scopes"]
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

				// The single property this phase exists to protect: state
				// (c) must never carry searched_scopes at all, not even an
				// empty list — a zero-value/length check alone would pass on
				// an emitted-but-empty key while this property was violated.
				if tc.wantMCP.hasScopesUnknown {
					if _, present := m["searched_scopes"]; present {
						t.Errorf("coverage-unknown state carries searched_scopes at all (even empty): %v", m["searched_scopes"])
					}
				}
			})

			t.Run("Connect", func(t *testing.T) {
				api := &engramAPI{d: d}
				resp, err := api.ListMemories(connectCtxFor(owner), connect.NewRequest(&engramv1.ListMemoriesRequest{
					Scope: scope, CrossSpine: tc.crossSpine, Limit: 10, Tags: []string{fixtureTag},
				}))
				if err != nil {
					t.Fatalf("ListMemories: %v", err)
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
				// Same single property, restated on the Connect wire: state
				// (c) must never carry a non-empty SearchedScopes alongside
				// ScopesUnknown true.
				if tc.wantConn.scopesUnknown && len(resp.Msg.GetSearchedScopes()) != 0 {
					t.Errorf("coverage-unknown state carries a non-empty SearchedScopes: %v", resp.Msg.GetSearchedScopes())
				}
			})
		})
	}
}
