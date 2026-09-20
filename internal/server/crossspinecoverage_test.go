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

	engramv1 "github.com/seanb4t/engram/gen/go/engram/v1"
	"github.com/seanb4t/engram/internal/store"
)

// failingListScopesStore embeds *spyStore and overrides ListScopes to return
// a scripted error, in the lostRaceStore shape (tools_test.go:4202-4216) —
// the precondition D-01's tests need is hits existing BEFORE the coverage
// query fails.
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

func (f *failingListScopesStore) ListScopes(context.Context, store.Subject) ([]store.ScopeCount, bool, error) {
	return nil, false, f.listErr
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
