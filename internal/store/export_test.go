// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import (
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

// MaxListLimit exposes maxListLimit to package store_test, so an external
// test can pin storetest.ManySmallRecords against it without duplicating the
// value (D-05).
const MaxListLimit = maxListLimit
