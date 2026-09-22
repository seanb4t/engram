// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package store_test is an EXTERNAL test package for internal/store: it is
// the ONLY place internal/store's TestMain can live, because
// internal/store/storetest imports internal/store (an import cycle would
// result if store's own in-package tests imported storetest, D-07), yet Go
// permits exactly one TestMain per test binary, and a directory's test
// binary compiles BOTH its in-package (`package store`) and external
// (`package store_test`) test files together. This file hosts that single
// TestMain, delegating container lifecycle to storetest.Run and handing
// down storetest's skip-or-fail decision through store.SetNoQdrantHandler
// (export_test.go) — the only channel across the package boundary, since a
// package-level var declared in store_test.go is invisible from here and
// vice versa. In-package tests receive the booted (or externally supplied)
// address through the pre-existing ENGRAM_QDRANT_TEST_ADDR contract, which
// storetest.Run exports via os.Setenv before m.Run().
//
// TestSharedQdrantAddressHonored also lives here rather than in
// store_test.go: it asserts on state (whether THIS package's TestMain
// booted its own testcontainer vs. took the shared-address fast path) that
// only the package holding TestMain can see (RESEARCH.md Pitfall 2).
package store_test

import (
	"os"
	"testing"

	"github.com/seanb4t/engram/internal/store"
	"github.com/seanb4t/engram/internal/store/storetest"
)

// TestMain provisions Qdrant for this directory's integration tests (both
// the in-package `store` tests and this external `store_test` package) by
// delegating entirely to storetest.Run. Behaviors preserved verbatim through
// storetest (D-10): fail-closed ENGRAM_REQUIRE_QDRANT parsing (invalid value
// is an error, never coerced to false), 3-minute bounded startup, 30-second
// bounded terminate, skip-with-message when no Qdrant.
func TestMain(m *testing.M) {
	store.SetNoQdrantHandler(storetest.SkipOrFailNoQdrant)
	os.Exit(storetest.Run(m))
}

// TestSharedQdrantAddressHonored proves this directory's suite took the CI
// shared-Qdrant fast path rather than booting its own testcontainer,
// whenever ENGRAM_QDRANT_TEST_ADDR is set (CI pins this test name; see
// .github/workflows/ci.yaml's shared-address step).
func TestSharedQdrantAddressHonored(t *testing.T) {
	storetest.AssertSharedAddressHonored(t)
}
