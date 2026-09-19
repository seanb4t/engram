// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package store

import "testing"

// SetNoQdrantHandler installs f as the package's no-Qdrant handler, called by
// dialTestClient when no Qdrant address is available. Package store cannot
// import internal/store/storetest directly (storetest imports store — an
// import cycle, D-07), so internal/store's external TestMain (main_test.go,
// package store_test) calls this exactly once, before any test runs, to hand
// down storetest.SkipOrFailNoQdrant — keeping storetest's parser the sole
// place ENGRAM_REQUIRE_QDRANT is read across this package's test suite.
func SetNoQdrantHandler(f func(testing.TB)) { noQdrantHandler = f }
