// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package httpdrain provides the one bounded response-body drain both
// provider clients (internal/embed, internal/summarize) call after they have
// read what they need from a response body and want to return the
// connection to the pool. It is a normal (non-_test.go) file in an internal
// package rather than a private copy in each client because a single shared
// mechanism cannot drift the way two independent copies can (D-02).
//
// Unlike internal/testhttp, this package is NOT test-only: it is imported by
// both provider clients' production code, on every request, not just from
// tests.
package httpdrain

import (
	"io"
	"time"
)

// Drain abandons the rest of body, bounded on two independent axes: it stops
// reading after maxBytes, and it abandons the read after maxTime elapses,
// whichever comes first.
//
// The time bound is enforced by arming a timer that closes body on
// expiry (D-01). Closing a response body unblocks any in-flight Read on
// it — verified against Go 1.27.1's net/http source
// (bodyEOFSignal.Read/Close in transport.go), not assumed — so a slow or
// stalled provider cannot hold the calling goroutine past maxTime. The timer
// is always stopped before Drain returns, so no timer and no goroutine
// outlives this call: there is no background goroutine here at all, only a
// stdlib timer bound to the call's own lifetime.
//
// A non-positive maxBytes or a non-positive maxTime skips the drain
// entirely: body is closed immediately and nothing is read. This is a
// deliberate, safe setting (D-05) — the caller is choosing to give up the
// connection (one extra TCP handshake next time) rather than ever risk a
// stall — never a signal to drain without a bound. There is no value, on
// either axis, that means "unbounded".
//
// Errors from the read are discarded: when the timer fires mid-read, the
// unblocked Read returns errReadOnClosedResBody, which needs no special
// case — Drain's only job is to let the connection go one way or the other,
// not to report why.
func Drain(body io.ReadCloser, maxBytes int64, maxTime time.Duration) {
	if maxBytes <= 0 || maxTime <= 0 {
		_ = body.Close()
		return
	}
	t := time.AfterFunc(maxTime, func() { _ = body.Close() })
	defer t.Stop()
	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxBytes))
}
