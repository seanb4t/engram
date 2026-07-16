// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

// This file holds test-only escape hatches for the async worker-pool queue
// types (summaryQueue, usageQueue). Wait is test-only (WR-03): moved out of
// production reach because every one of its call sites in the repo is a
// _test.go file, so there is no production caller to preserve. Blocking on
// an in-flight WaitGroup is safe only in tests; a hypothetical production
// caller could deadlock the write path. The _test.go suffix alone is a
// compiler-level exclusion — go build never compiles this file, so Wait is
// structurally unreachable from serve.go or any other production code. No
// build tag is needed. The package's tests are in-package (package server,
// not package server_test), so every existing call site resolves unchanged.

// Wait blocks until every enqueued (non-dropped) id currently in flight has
// reached a terminal fill outcome (success, exhausted retries, or a
// recovered panic) — the deterministic drain seam tests use instead of
// time.Sleep polling. Nil-safe no-op on a disabled queue.
func (q *summaryQueue) Wait() {
	if q == nil {
		return
	}
	q.inFlight.Wait()
}

// Wait blocks until every enqueued (non-dropped) id currently in flight has
// reached a terminal fill outcome (success, error, or a recovered panic) —
// the deterministic drain seam tests use instead of time.Sleep polling.
// Nil-safe no-op on a disabled queue.
func (q *usageQueue) Wait() {
	if q == nil {
		return
	}
	q.inFlight.Wait()
}
