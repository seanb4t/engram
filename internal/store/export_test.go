// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
	"context"
	"testing"

	"github.com/qdrant/go-client/qdrant"
)

// SetNoQdrantHandler installs f as the package's no-Qdrant handler, called by
// dialTestClient when no Qdrant address is available. Package store cannot
// import internal/store/storetest directly (storetest imports store — an
// import cycle, D-07), so internal/store's external TestMain (main_test.go,
// package store_test) calls this exactly once, before any test runs, to hand
// down storetest.SkipOrFailNoQdrant — keeping storetest's parser the sole
// place ENGRAM_REQUIRE_QDRANT is read across this package's test suite.
func SetNoQdrantHandler(f func(testing.TB)) { noQdrantHandler = f }

// NewTestStore exposes the in-package prefix-enforcing construction seam
// (newTestStore) to package store_test, so an external oversized regression
// test builds its Store through the SAME collection-prefix assertion every
// in-package test does — never a qualified store.New(...) call, which the
// collection-prefix conformance gate treats as a bypass (D-07).
func NewTestStore(t testing.TB, c *qdrant.Client, name string, opts ...Option) *Store {
	return newTestStore(t, c, name, opts...)
}

// PrefixedTestCollection exposes testCollection to package store_test. Named
// without a "Test" prefix: go vet's "tests" analyzer rejects a _test.go
// function whose name starts with "Test" but is not itself a test.
func PrefixedTestCollection(name string) string { return testCollection(name) }

// ReadView exposes the internal readView type to package store_test — the
// D-07 cycle-breaker (storetest imports store, so store_test's own oversized
// tests cannot import a helper package for this).
type ReadView = readView

// FullView exposes (*Store).fullView to package store_test.
func (s *Store) FullView() ReadView { return s.fullView() }

// SummaryView exposes (*Store).summaryView to package store_test.
func (s *Store) SummaryView() ReadView { return s.summaryView() }

// ViewMaxRecordBytes exposes readView.maxRecordBytes to package store_test.
func ViewMaxRecordBytes(v ReadView) int { return v.maxRecordBytes }

// SweepLimit exposes sweepLimit to package store_test.
func SweepLimit(v ReadView) int { return int(sweepLimit(v)) }

// RPCByteBudget exposes rpcByteBudget to package store_test.
func RPCByteBudget() int { return rpcByteBudget }

// PageByteBudget exposes pageByteBudget to package store_test.
func PageByteBudget() int { return pageByteBudget }

// ScrollAllPoints exposes (*Store).scrollAllPoints to package store_test.
func (s *Store) ScrollAllPoints(ctx context.Context, filter *qdrant.Filter, v ReadView, fn func(*qdrant.RetrievedPoint) error) error {
	return s.scrollAllPoints(ctx, filter, v, fn)
}

// OrderedPage exposes the internal orderedPage type to package store_test.
type OrderedPage = orderedPage

// ListCursor exposes the internal listCursor type to package store_test.
type ListCursor = listCursor

// ScrollOrderedPage exposes (*Store).scrollOrderedPage to package
// store_test, building the caller's filter through the same listFilter every
// in-package caller uses.
func (s *Store) ScrollOrderedPage(ctx context.Context, scope string, subj Subject, v ReadView, dir qdrant.Direction, from ListCursor, limit uint64) (OrderedPage, error) {
	f := s.listFilter(ctx, scope, subj, ListOptions{})
	return s.scrollOrderedPage(ctx, f, v, dir, from, limit)
}

// PerRPCLimit exposes perRPCLimit to package store_test.
func PerRPCLimit(v ReadView) int { return perRPCLimit(v.maxRecordBytes) }

// UnbudgetedView exposes unbudgetedView to package store_test.
func UnbudgetedView(sel *qdrant.WithPayloadSelector) ReadView { return unbudgetedView(sel) }

// KeysView exposes keysView to package store_test.
func KeysView() ReadView { return keysView() }

// SetByteBudgets overrides rpcByteBudget/pageByteBudget for t's duration,
// restoring both via t.Cleanup.
func SetByteBudgets(t testing.TB, rpc, page int) {
	t.Helper()
	oldRPC, oldPage := rpcByteBudget, pageByteBudget
	rpcByteBudget, pageByteBudget = rpc, page
	t.Cleanup(func() { rpcByteBudget, pageByteBudget = oldRPC, oldPage })
}

// IncludeIDs exposes includeIDs to package store_test.
func IncludeIDs(f *qdrant.Filter, ids []string) *qdrant.Filter { return includeIDs(f, ids) }

// FetchPayloadsByID exposes (*Store).fetchPayloadsByID to package store_test.
func (s *Store) FetchPayloadsByID(ctx context.Context, f *qdrant.Filter, v ReadView, ids []string) (map[string]Memory, error) {
	return s.fetchPayloadsByID(ctx, f, v, ids)
}
